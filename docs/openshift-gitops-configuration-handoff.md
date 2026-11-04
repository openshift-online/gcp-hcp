# GCP-1282: In-place replacement with operator-managed Argo CD

> **TL;DR:** Meet the new Argo requirements, take the old Argo controllers offline
> with declarative scale-to-zero, then enable operator-managed takeover of retained
> resources. Do not run both controller sets against the same resources. This is
> the agreed direction, not authorization to execute a cutover or delete anything.

**Opt-in guidance; no default rollout.** Configuration ownership was validated on
existing operator-managed MC/region instances on October 13, 2026; see the
[sanitized validation record](openshift-gitops-configuration-validation.md).
The companion implementation [PR #2315](https://github.com/openshift-online/gcp-hcp-infra/pull/2315)
has merged; chart/matrix/tests are documented in
`gcp-hcp-infra/docs/ARGOCD_OPERATOR_CONFIGURATION.md`. Pin reviewed bootstrap and
chart revisions before proposing a live transition. The existing
`bootstrap/kustomization.yaml` does not reference these opt-in components;
ordinary bootstrap output is unchanged.

**Authentication follow-up, October 21–23:** GCP-1283 subsequently validated the
existing HCP IAP/Dex flow on both disposable operator-managed targets, including
identity, representative RBAC and logout/re-login. See the
[dated authentication record](https://github.com/openshift-online/gcp-hcp-infra/blob/0a523f7fc9bb480e91ea608e3dfe358ef380be1f/docs/ARGOCD_IAP_VALIDATION.md)
in [gcp-hcp-infra PR #2439](https://github.com/openshift-online/gcp-hcp-infra/pull/2439).
These results support readiness for the tested existing instances; they do not
demonstrate the legacy scale-to-zero/takeover sequence below or actual GitHub
notification delivery, and do not certify a new target or release artifact.

## Installation method: no OLM

**OLM is not used at either layer.** The proposed installation path is:

```text
Pinned operator/CRD Helm charts
  → rendered Kubernetes manifests in Git
  → Fleet Config Sync installs the operator and CRDs
  → operator directly reconciles Argo CD from supported CR inputs
```

This does not install Operator Lifecycle Manager (OLM), use OLM Subscriptions,
CatalogSources or InstallPlans to manage the operator, or have the operator invoke
OLM to deploy Argo CD. Helm packages the operator; the operator manages Argo CD
directly. This is not a proposal for a standalone, non-operator Argo Helm chart.
Verify the selected release's chart availability, support and provenance
separately; the engineering trials are not release qualification. Publishing this
guide does not change the default bootstrap or install anything.

## Agreed replacement sequence

The final state has **one owner per setting**: the private chart supplies supported
CR inputs, the operator generates its runtime/ConfigMaps, and the operator-managed
Argo application controller reconciles Applications and their workloads. Scaling
down the old runtime does not by itself transfer those ownership relationships.

| Stage | Required result |
| --- | --- |
| 1. Readiness | Meet the target's required configuration, functionality, artifact and access prerequisites; preserve backwards compatibility with existing non-operator Argo until cutover |
| 2. Old runtime offline | Declaratively scale the old owning workloads to zero and observe their controllers inactive; preserve Applications, shared resources and tracking state |
| 3. Operator-managed takeover | Release competing configuration/seed writers, verify intended tracking identity and ownership, and only then let the new controller set reconcile the retained resources |
| 4. Acceptance | Verify required settings, resource tracking, root continuity, application behavior and applicable login/notification gates; observe persistence through reconciliation/restart |
| 5. Later cleanup | Keep legacy-only workloads inactive until acceptance; reused objects must already be released from old management; review removal separately |

### Readiness and identity

Inventory the old and new controllers, sources, resource identities, annotations,
owners and finalizers. Pin compatible operator and CRD versions and complete
per-target CR settings, repository access, external credential delivery and
bootstrap/recovery inputs. The private chart cannot bootstrap itself without a
running Argo instance; the replacement must not depend on the old runtime remaining
active.
Identify the independent bootstrap/recovery executor in the target plan. Complete
any protection or source changes that require the old Argo before stopping it;
remaining steps need an independent executor, not a now-inactive Argo Application.
Establish pruning safeguards before activation: retained Applications may already
request automatic pruning, so a check after the new controller starts is too late.

Keep forward configuration backwards compatible with existing/unmigrated
non-operator targets. Test both chart modes and the existing Argo e2e path; its disabled
IAP/Dex settings do not prove organizational login. Per-target IAP login and actual
GitHub notification delivery remain required readiness checks, not consequences
of a successful configuration render or Synced status. The dated GCP-1283 follow-up
above supplies authentication evidence for the existing tested instances only;
actual notification delivery remains a separate GCP-1284 acceptance gate.

The meeting requirement for the "same identifier" needs explicit preflight checks:
capture the tracking method, any installation identifier, application identities
and tracking annotations, and verify the intended equivalents in the new instance
before enabling pruning. Review runtime/service names and Secret collisions too;
the trial name `gitops` is not a production naming decision. A recreated Kubernetes
object cannot retain its old UID; distinguish Argo tracking identity from UID
continuity of resources retained in place. Adoption must be observed, not assumed.
Stop on unexpected tracking/owner mismatches rather than rewriting identifiers to
force apparent adoption.

### Scale down the old control loops

The intent of "remove/nuke the old Argo" here is **take it offline**, not delete
the namespace or managed workloads. Scale owning Deployments/StatefulSets rather
than a child ReplicaSet that its Deployment can restore. Inventory all old writers,
especially application/ApplicationSet controllers and notifications; stopping only
the UI server does not stop application reconciliation. Existing bootstrap source
uses a StatefulSet for the application controller and Deployments for other roles.

Config Sync reconciles continuously, not only during initial bootstrap. Use
reviewed target-specific source inputs so Config Sync, an HPA or another owner
does not restore the old replicas. Observe zero replicas/no old reconciliation
before the new application controllers act on the shared resources. Operator/CRD
preparation must not accidentally activate a second competing controller set.
If the operator will reuse a scaled-down Deployment/StatefulSet identity, release
the old declarative owner with the appropriate prune/detach safeguards before the
operator activates it. Do not leave Config Sync enforcing zero on that same
object. Distinguish these reused objects from legacy-only workloads kept at zero.

Isolating changes from HyperShift-backed development clusters and validating on a
development All-in-One were discussed, not executed or authorized by these notes.
Do not change shared bootstrap sources or disable
Config Sync broadly. If a temporary pause is chosen, separately approve the exact
targets/writers, scope and resume conditions, and verify convergence after resume.

### Transfer, verify, then clean up

Preserve Applications, namespace/root resources, shared CRDs, ESO stores/Secrets,
metadata/repo access and the region platform API route. Release old chart or
Config Sync configuration writers before the new CR inputs take over. The
operator owns its runtime/configuration; its application controller, not the
operator alone, takes over application workload reconciliation.

Use the supporting seed procedure below only where the target has those competing
owners. Verify ownership/tracking without unintended pruning, required settings,
root/private-repo continuity and target-specific acceptance. Keep the old runtime
inactive; any later deletion needs an ownerReferences/finalizer/garbage-collection
and prune review. Never strip finalizers as a shortcut. This guide provides no
blanket deletion command or already-validated execution script for the new cutover.

## Supporting seed ownership handoff

These optional components handle a specific configuration-ownership overlap; they
do not scale old Argo down, install a second runtime or automatically adopt all
resources. The October 13 operator-managed trials started with two seeds overlapping
day-two configuration: `ArgoCD/argocd/gitops` and the Dex seed
`ConfigMap/argocd/argocd-cm`. The private chart will own supported ArgoCD and
NotificationsConfiguration CR inputs, not directly generated ConfigMaps.
Simply adding a private CR writer without releasing the public seed is unsafe.

### Supported release mechanism

Google documents both [preventing deletion and stopping management without
deleting objects](https://cloud.google.com/kubernetes-engine/enterprise/config-sync/docs/how-to/managing-objects).
Use separate observed revisions; do not collapse protection, release and
removal into one source update.

| Stage | Opt-in component | Required observed result |
| --- | --- | --- |
| Protect | `bootstrap/components/argocd-config-handoff` | Both seeds have `client.lifecycle.config.k8s.io/deletion: detach` with original UIDs |
| Release | `bootstrap/components/argocd-config-release` | Config Sync stops managing both seeds, removes its management metadata/inventory references, and does not delete them |
| Adopt | Private chart's reviewed operator mode | ArgoCD/default NotificationsConfiguration spec has one declarative owner; operator-generated CMs retain required values |
| Retire | Remove released seed resource references/files and temporary components | No generated resource deletion; namespace/ESO/metadata/root ApplicationSet/shared CRDs remain managed |

The release annotation `configmanagement.gke.io/managed: disabled` is an explicit
handoff, **not** a final drift-suppression workaround. The new private CR inputs
and their operator controllers must continue reconciling required settings.
Removing a managed resource without release/detach would prune it. A missing
ArgoCD CR can garbage-collect the runtime; never use deletion to clear OutOfSync.

### Candidate assembly and checks

These components are specifically scoped to the trial instance `gitops` in
`argocd`. That is not a production naming decision. Customize and independently
review selectors if another instance/namespace is chosen. Verify each selector
matches exactly one expected seed before any source publication.

In a **local reviewed copy** of the existing operator trial bootstrap, place the
team component next to the root and add:

```yaml
components:
  - components/argocd-config-handoff
```

Offline render with `kubectl kustomize <candidate-directory>`. Compare parsed
objects/resource identities with the preceding bootstrap: protection must only
add detach annotations to the two seeds. Release must only add the disabled
management annotation, preserving previous annotations and specs. Keep namespace,
ESO stores/ExternalSecrets, metadata/repo resources, root ApplicationSet,
operator/CRDs, controller RBAC and pull access unchanged. Preserve RootSync sizing.

The two known operator trial roots are on separate public fork revisions/dirs.
Do not add the component to shared org `main/bootstrap` or switch both clusters
as an incidental effect. Publication to a branch followed by a live RootSync
can itself deploy changes and needs explicit approval.

After each separately approved source stage, verify fetched revision, RootSync
Sync Completed/no errors, live annotations and original seed UIDs. Config Sync
does not leave `managed: disabled` on the live object; confirm its **management
metadata and inventory entry are removed**, not the presence of that value.
Private-chart CM protection, including the mutation-ignore bridge below, must
already have been observed before public release. The order is **private CM
protection → public seed protection → observed public release → private CR
adoption → guarded ConfigMap tracking retirement**. After release, proceed with
adoption and tracking retirement in the private chart's procedure; protection is
not a post-release step. Argo prune protection and obsolete tracking cleanup are
separate from Config Sync release and must not be skipped.

An existing `client.lifecycle.config.k8s.io/mutation: ignore` on argocd-cm can
cause Config Sync to skip an ordinary detach-annotation update. The private
chart's preceding CM-protection stage supplies the matching deletion-detach
bridge. Observe the live annotation on both seeds before release; a Synced
source alone is not that proof. The release stage was verified to remove only
management metadata and the two inventory references, without changing data or
UIDs. Do not remove mutation-ignore to force a sparse seed over the full CM.

## Bootstrap continuity and recovery

Keep the operator and root bootstrap resources under Config Sync. This preserves
the Terraform → Fleet Config Sync → Argo CD chain, but releasing the instance
means Config Sync cannot recreate that instance on a new cluster from the
released stage. Review protected installation-seed or exact-spec reclaim inputs
before replacement/rebuild. If bootstrap creates another Config Sync-owned
instance seed, its ownership still needs release before private-chart adoption.
Do not blindly replay an old sparse seed over current required settings.
Productized bootstrap/rebuild orchestration is separate GCP-1280/GCP-1288 work.

Retaining old definitions at zero is not an automatic rollback. Do not scale old
application controllers back up while new controllers are reconciling the same
resources. Stop for a reviewed recovery plan that coordinates controller activity,
configuration, tracking and source ownership; no automatic reverse cutover is
implemented here.

Before ownership release, a reviewed reversal may restore the preceding protected
inputs. After release, keep the retained resources protected and stop for an
exact-spec Config Sync reclaim plan if needed; never blindly replay an old seed.
After adoption, fix/roll back the private CR inputs rather than reintroducing
competing ConfigMaps or the old managed seed. Setting `operator.enabled=false`
is not a rollback: protected CRs continue reconciling while the chart resumes
direct ConfigMap writes. Reverse ownership transfer needs
its own protection, Argo tracking release, Config Sync reclaim and exact-spec
review. No reset/teardown, Route CRD removal, or shared-state Terraform action is
authorized by this procedure.

## Evidence boundary

Offline render tests verify preservation of seed specs/UID-independent resource
identities and existing annotations. The October 13 disposable trials additionally
verified live release/prune transitions, retained settings through two subsequent
Argo Application reconciliations and a relevant manager restart on each cluster,
and representative health/watch behavior **for configuration ownership on existing
operator-managed instances**. They do not demonstrate the complete old non-operator
Argo scale-to-zero → operator-managed takeover sequence, deleting/recreating Argo,
or rebuilding the cluster. These are bounded, dated observations; the agreed runtime
cutover needs its own target-specific evidence. Existing engineering
artifacts are not a certified supported release. Configuration preservation alone
does not establish authenticated IAP login, real GitHub delivery or complete rebuild
recovery; the later GCP-1283 authentication evidence is separately attributed above.
Team PR review/merge and applicable DoD remain separate from trial proof.
