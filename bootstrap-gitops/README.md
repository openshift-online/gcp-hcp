# bootstrap-gitops: inactive operator-managed Argo CD workload bundles

> **Status: inactive, unreferenced, offline-validated only.** Nothing in
> `bootstrap/`, any RootSync, any workflow or any Terraform references this
> directory. It is GCP-1280 Step 2 output: reviewable artifacts that a later,
> separately authorized step (Step 3 parent wiring, Dev AIO, Integration) can
> consume. It proves no runtime behavior: no Config Sync apply, ownership,
> prune, ESO/IAM, operator reconcile, memory or health claim is made or implied.
> Project IDs here are synthetic. This is not a support or certification claim
> (that belongs to GCP-1286); optional components stay out of scope (GCP-1287).

## Layout

```text
bootstrap-gitops/
  workloads/synthetic/mc/       child bundle for a management cluster
  workloads/synthetic/region/   child bundle for a region cluster
  parent-artifacts/shared-crds.yaml   the three shared CRDs, for the later parent
```

Each bundle directory is a self-contained kustomization:
`crds.yaml` (5 child CRDs), `operator.yaml` (operator namespace, ServiceAccount,
RBAC, Deployment), `registry.yaml` (bootstrap SecretStore + ExternalSecret),
`gitops-instance.yaml` (ArgoCD seed), `controller-rbac.yaml` (temporary additive parity
RBAC) and `kustomization.yaml` (lifecycle and ordering patches).

Only **MC** and **region** exist. A global bundle is out of scope: the
assembler rejects `global` (and every other unknown type) before doing any work,
and the private global environment keeps its independently owned Promoter
install untouched.

Regenerate (the test suite diffs the committed files against a fresh run):

```sh
tools/gitops-bootstrap/assemble.sh --type mc --global-project-id synthetic-global-project \
  --output-dir /new/dir/mc --shared-crds-output /new/file/shared-crds.yaml
tools/gitops-bootstrap/assemble.sh --type region --global-project-id synthetic-global-project \
  --output-dir /new/dir/region
tools/gitops-bootstrap/tests/run-step2-tests.sh
```

Chart provenance, the official image tuple and every digest are in
`tools/gitops-bootstrap/chart-src/PROVENANCE.md`.

## Ownership model

| Owner | Objects |
| --- | --- |
| **Parent** (Fleet root-sync, Step 3, retained as-is) | `argocd` Namespace; ESO installation and CRDs; `argocd-local-gcpsm` SecretStore; `argocd-secrets` ServiceAccount; `cluster-metadata-annotations-template`; `argocd-cluster` and git-credentials ExternalSecrets; `root-app` ApplicationSet; **`applications`, `applicationsets`, `appprojects` CRDs** (`parent-artifacts/shared-crds.yaml`) |
| **Child** (this bundle, 17 objects) | `gitops-operator` Namespace, ServiceAccount, leader-election Role/RoleBinding, controller ClusterRole/ClusterRoleBinding (official CSV rules), manager Deployment; CRDs `argocds` (v1beta1-only), `rolloutmanagers`, `notificationsconfigurations`, `namespacemanagements`, `controllerconfigurations.promoter.argoproj.io` (required dependency API, mutation-managed, `deletion: detach`); registry SecretStore + ExternalSecret; ArgoCD seed `argocd/gitops`; interim ClusterRole/ClusterRoleBinding |
| **Day-two** (private chart, later) | the full ArgoCD spec after handoff, `argocd-cm` family, notifications configuration, `argocd-global-secret-store` and `argocd-oauth-secret-accessor` |

The child declares none of the parent identities and the parent must not
declare the five child CRDs. Inventories must stay disjoint: the Fleet root uses
`PolicyAdoptAll`, so overlapping declarations are never safe. Do not claim that
an automatic `ManagementConflict` universally protects duplicate roots.

### Classification against the legacy bootstrap (`bootstrap/`, offline, source-based)

This is a design inventory from the repository source at the pinned base, not
live-cluster proof.

- **Retained** by the parent, unchanged: the parent column above.
- **Child-added:** the 17 identities above.
- **Released** (Config Sync stops owning; objects remain under the
  operator/day-two owner via the existing Protect → Release → Adopt → Retire
  handoff in `docs/openshift-gitops-configuration-handoff.md`): the
  `argocd-cm`, `argocd-cmd-params-cm`, `argocd-rbac-cm` and
  `argocd-notifications-cm` ConfigMaps, `argocd-notifications-secret` and the
  `argocd-server` Service patch target. The new child seed is protected from
  creation instead.
- **Removed** later, only after acceptance and a separate review (first taken
  offline, never finalizer-stripped): the legacy `bootstrap/argocd/install.yaml`
  workload set (6 Deployments, 1 StatefulSet, 8 Services, 7 ServiceAccounts,
  6 Roles and RoleBindings, 3 ClusterRoles and ClusterRoleBindings,
  7 NetworkPolicies, the gpg/ssh/tls ConfigMaps and `argocd-secret`; 59 objects
  plus the 3 CRDs that the parent retains). Whether the operator reuses any
  fixed-name object (`argocd-cm` family, `argocd-secret`) must be confirmed at
  runtime; this offline inventory does not prove it.
- Preserved: the HyperShift Route CRD and the region platform API (not touched).

## Required dependency API: ControllerConfiguration

`controllerconfigurations.promoter.argoproj.io` is a **required dependency API,
not an optional Promoter installation**. The pinned operator's ArgoCD reconcile
reads a `ControllerConfiguration` before it checks whether Promoter is enabled,
and its deletion path lists them, so a missing API is a hard error. The child
therefore declares just that CRD from the official 1.22.2 schema (no Promoter
workloads, CRs or other Promoter CRDs). It is owned by the sole MC/region child
RootSync like the other child CRDs: **normally mutation-managed, with only
`client.lifecycle.config.k8s.io/deletion: detach`** so that dropping the
declaration never deletes the schema.

In the reviewed private snapshot (`264d4910…`) the independent v0.48.0 Promoter
owner exists for the global environment only, which this tooling rejects.
**This bundle must never be applied to, or stamp, the global CRD** (that schema
is a superset and must not be downgraded), and the parent must not declare it.
Any later schema upgrade of this CRD needs explicit review. Source inventory is
not live proof: a later runtime preflight must confirm that the
ControllerConfiguration CRD has no competing owner on the target cluster.

## Ordering and lifecycle annotations

Ordering uses `config.kubernetes.io/depends-on`, in the syntax read from Config
Sync v1.23.3's vendored cli-utils (`<group>/<Kind>/<name>` cluster-scoped,
`<group>/namespaces/<ns>/<Kind>/<name>` namespaced, comma separated):
ExternalSecret → SecretStore; manager Deployment → the five child CRDs and the
registry ExternalSecret; ArgoCD seed → the `argocds` and ControllerConfiguration
CRDs and the manager. `check-depends-on.py` (run by `assemble.sh` before any
output is published, and by the tests) fails unless at least one edge exists,
every token resolves inside this bundle, no target is `mutation: ignore` and
the graph has no directed cycle. Cross-RootSync dependencies are not
expressible and are Step 3's concern: the parent's `argocd` namespace and ESO
CRDs must exist, and an early apply simply retries.

**Skip/dependent interaction (source-derived, cli-utils `5895ad6c17dd` pinned
by Config Sync v1.23.3; Config Sync was not run):** an incoming
`mutation: ignore` object that already exists is a *skipped apply*, and a
skipped or failed dependency makes the dependency filter skip its dependents.
So a `mutation: ignore` object must never be a `depends-on` target, otherwise
its dependents would be skipped on every sync after the first. That is why only
the ArgoCD seed (which nothing depends on) is `mutation: ignore`. Valid
annotation syntax alone does not prove safe second-sync behavior, and this
repository makes no runtime-closure claim.

Exact lifecycle annotation sets (asserted by the assembler and the tests):

| Object | `mutation` | `deletion` |
| --- | --- | --- |
| ArgoCD seed `argocd/gitops` (create-if-absent, nothing depends on it) | `ignore` | `detach` |
| `controllerconfigurations.promoter.argoproj.io` CRD | unset (managed) | `detach` |
| every other child object | unset | unset |

Also source-derived (Config Sync v1.23.3): the initial applier's
`PreventUpdateFilter` reads the **incoming** `mutation` annotation; for an
object that already exists the apply is skipped and Config Sync's
ownership/lifecycle metadata is then merge-patched on. The remediator diff reads
the **live** annotation. So `mutation: ignore` avoids an initial
content/schema overwrite of an existing object but **still stamps ownership**;
it is not a generic adoption escape hatch and does not make duplicate
declarations safe.

## Retirement boundary (destructive; needs its own reviewed sequence)

Only the ArgoCD seed and the ControllerConfiguration CRD carry
`deletion: detach`. **Dropping this child bundle or its root remains
destructive:** the `argocds` CRD, the other child CRDs, the manager Deployment,
the RBAC and the `gitops-operator` namespace are all still prunable. Deleting
the `argocds` CRD cascades to every ArgoCD CR, including the "detached" seed,
and the operator finalizer cannot be assumed to run once the manager is gone.
Detach is therefore not a retirement mechanism. Decommissioning needs
separately reviewed sequencing (take the instance offline, release or
re-home ownership, confirm no ArgoCD finalizers are pending, only then remove
the operator) and is deliberately not designed here (Step 3 / decommission).

## Selected-controller assumptions and the runtime gate

Child schemas are chosen from source of the pinned release lineage (bundle
upstream ref `cf32771491be…`; argocd-operator `a213c171036a…`; see
PROVENANCE.md for the file digests): the ArgoCD controller watches `ArgoCD`,
owns `NotificationsConfiguration` and watches `NamespaceManagement`; the
RolloutManager controller starts unconditionally; the Promoter
`ControllerConfiguration` is read on every reconcile; ImageUpdater is detected
at runtime and disabled when its API is absent; the GitopsService controller
starts only on OpenShift. These are source-reading assumptions about the
selected release, not observed behavior. The runtime gate stays with later
authorized Dev AIO and Integration work: operator start with exactly these
CRDs, ArgoCD reconcile with Promoter disabled, ESO delivery of the pull secret
(the region ESO identity needs its IAM grant first, future private Step 4), the
`gitops-argocd-application-controller` service-account name, prune/ownership
behavior and health.

Additional later Dev AIO acceptance checks:

- Manager resources: before activation or promotion, observe the operator's
  startup and reconcile CPU and memory, CPU throttling, OOM kills and restarts
  under the `100m`/`256Mi` requests and `1`/`1Gi` limits, and confirm the sizing
  is appropriate for the target. The budget is untuned and has no sizing
  guarantee; adjust through the reviewed values only after observation.
- Interim controller RBAC: confirm the effective controller permissions on the
  target and track GCP-1285 for the scoped replacement.
- The `argocds` CRD is about 912 KB (911,547 bytes as compact JSON). Whether
  Config Sync's server-side-apply managedFields leave headroom under the etcd
  1.5 MiB object limit is **unproven**; confirm it on a real cluster before
  relying on this bundle. No offline claim is made and no unsupported
  workaround is applied preemptively.

## Seed and registry decisions

- ArgoCD `argocd/gitops`: create-if-absent; ApplicationSet, Notifications and
  Dex enabled; `disableAdmin: true`, `usersAnonymousEnabled: false`, annotation
  resource tracking; same authproxy connector as the existing bootstrap seed; no
  `argocd-cm` seed. The instance name must not be `argocd`: the operator would
  create an `argocd-cluster` admin Secret that collides with the ESO-owned
  cluster metadata Secret.
- Registry: a namespaced SecretStore in the operator namespace using the ESO
  controller's own identity against the explicit global project (no cluster
  IDs, no keys) and an ExternalSecret producing a `dockerconfigjson` Secret
  labelled for propagation. No Secret object or credential is declared in Git.
  The child never depends on a day-two Argo-managed store.
- **Interim controller RBAC: a temporary, additive migration-parity exception
  (not least privilege).** `controller-rbac.yaml` gives the application
  controller ServiceAccount the legacy controller's wildcard rules under a
  distinct name, on top of the ClusterRole/ClusterRoleBinding the operator
  generates (RBAC is additive). It is kept because, in the pinned source, the GitOps
  operator registers its OpenShift reconciler hook unconditionally and that hook
  replaces the stock wildcard rules with narrower ones, so the operator-generated
  role would lose writes to workload kinds in unmanaged namespaces and cluster
  writes such as CRDs and webhooks that the legacy controller performs. This is
  pinned-source evidence, not GKE runtime proof (details and file digests:
  `tools/gitops-bootstrap/chart-src/PROVENANCE.md`). **Exit criterion: GCP-1285**
  supplies and validates scoped permissions covering the managed workloads
  before the grant can be removed or narrowed; it is not redesigned here.
- **Manager resources are an untuned initial budget.** The operator Deployment
  requests cpu `100m` / memory `256Mi` and is limited to cpu `1` / memory `1Gi`
  (the official CSV sets none). The numbers have no verified source and no
  performance or sizing guarantee.
