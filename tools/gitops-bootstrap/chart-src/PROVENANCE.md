# Provenance of the locally maintained OpenShift GitOps 1.22.2 charts

Scope: GCP-1280 engineering pin resolution. **Nothing here is a Red Hat
support, signature or certification claim** (GCP-1286), and neither archive is
a vendor-published or vendor-supported chart. The chart *packaging* is local;
the *content* (CRDs, RBAC rules, Deployment shape, env, image digests) comes
from the official operator bundle named below.

## Official source (primary)

| Item | Value |
| --- | --- |
| Bundle reference | `registry.redhat.io/openshift-gitops-1/gitops-operator-bundle@sha256:2ba58c50f771c08bbc5a1d8572c7b058df55bc71667cc66026d42840eb4bee6d` |
| Bundle manifest digest (= SHA256 of `manifest.json`, re-verified) | `sha256:2ba58c50f771c08bbc5a1d8572c7b058df55bc71667cc66026d42840eb4bee6d` |
| Config digest (file bytes re-verified) | `sha256:166400f2ea7afe0516ce065206c7cf22080e9310f9d0cb3d489c05ddb19e1bcf` |
| Layer digest (file bytes re-verified, extracted for these files) | `sha256:c552e9b88b48a87485170a215a1c52c3bc527e30ae43e660a605d0ac12c9fd3c` |
| CSV / version / release | `openshift-gitops-operator.v1.22.2`, label `version=v1.22.2`, `release=7`, channels `latest,gitops-1.22`, `build-date=2026-10-13T06:55:24Z` |
| Upstream source (config label) | `https://github.com/redhat-developer/gitops-operator.git` @ `cf32771491be2ed7f03d43d279171d6c265f2d2b` |
| Downstream source (config label) | `https://github.com/rh-gitops-midstream/release` @ `219dafaaf0e9431f8b468b4932f337f3ce397058` |
| How obtained | anonymous `skopeo copy` of the identical digest from the public Konflux registry (`quay.io/redhat-user-workloads/rh-openshift-gitops-tenant/gitops-operator-bundle@sha256:2ba58c50…`; no credentials), cross-checked against the anonymous Red Hat container catalog, which lists the same `registry.redhat.io` bundle digest for every 1.22.2 record |
| License | Apache-2.0. `LICENSE-gitops-operator.Apache-2.0` is a verbatim anonymous fetch of the LICENSE at the upstream commit above; SHA256 `43070e2d4e532684de521b885f385d0841030efa2b1a20bafb76133a5e1379c1` |

The upstream/downstream refs are the bundle's own declared source linkage. They
are not byte-for-byte proof of the downstream build internals. The licenses of
individual upstream dependencies that the CRDs originate from were not
separately verified.

Bundle files consumed (SHA256, recorded in `upstream.sha256`, checked by
`build-charts.sh` before use):

```text
7b812b76b4e89b58df34cb34a47e4d97bc91ea13a1c7340148905a44a71843ef  manifests/argoproj.io_argocds.yaml
12386babf7f862f3572a0ab084305d638ffd7e80f5d04fb8c8fac805b98ef289  manifests/argoproj.io_applications.yaml
32c1583267fbc87ac8f19072089b16ff3b9e1c0d36aa526960e83921894145bb  manifests/argoproj.io_applicationsets.yaml
8e624d1fd1ddecd27066f6ba0ce9aa0b211a4fe50507d7566b940b399765c5ae  manifests/argoproj.io_appprojects.yaml
808cd79f068ba3441d140105158f83d8973f96cd43c98976913401636c4dd4d4  manifests/argoproj.io_rolloutmanagers.yaml
f30d969069497a084e29f9728003e911a034d1b1d0a9b4256f3a742935cf6004  manifests/argoproj.io_namespacemanagements.yaml
3e23eae80d347ddfcd13be6a3f134df891d278fd4d0ca14fc15252f61c995293  manifests/argoproj.io_notificationsconfigurations.yaml
0da97d019103312748bf2feeaf9d375421ae4d42b47c888c79b9951111b614bb  manifests/promoter.argoproj.io_controllerconfigurations.yaml
8a7d3dc82a8a8b285e66cb3ece49f17db0315fd717b78dee666172c812035ea1  manifests/gitops-operator.clusterserviceversion.yaml
```

`upstream/gitops-operator.v1.22.2.clusterserviceversion.yaml` is a verbatim copy
of the bundle CSV (offline parity tests compare the rendered operator to it).

## Official image tuple (complete, 15 distinct references)

Every reference is the CSV `spec.relatedImages` value. In the anonymous Red Hat
container catalog all 15 had an exact published repository + manifest-list
digest match; the operator and its Red Hat operands are tagged `v1.22.2` /
`v1.22.2-7`, redis and haproxy carry their own tags. This is engineering pin
resolution, not support or signature verification, and registry.redhat.io was
not contacted (it needs credentials).

| Reference | CSV name(s) | Catalog tag |
| --- | --- | --- |
| `registry.redhat.io/openshift-gitops-1/argo-rollouts-rhel9@sha256:061217a286fbf1296a5f121097ef50351f1e5381dac43e7c9345c81ae853ad78` | `argo_rollouts_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/argocd-agent-rhel9@sha256:040edd45c6d6153f7940063d3d5c6306d4f489a5c7db02ad3916ab40e7884511` | `argocd_principal_image, argocd_agent_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/argocd-extensions-rhel9@sha256:877d3f4fb8141729a9861aba8ab5c60a1fcb1b354ff040af2b978148da545af4` | `rollouts_extension_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/argocd-image-updater-rhel9@sha256:36c9f6ecef9a791ce91affee82e0c1f285d37235239bcbb5834b157bf2270e9e` | `argocd_image_updater_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/argocd-promoter-extension-rhel9@sha256:cf9c1d9af3bd038692eb3de1e788819c9326155b6edcb8b8ef9c94815ec65b2a` | `promoter_extension_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/argocd-rhel9@sha256:9aaeb618197e278fb95f812fed82dc9253fc4843b9e08d16288bb885859b0f1f` | `argocd_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/console-plugin-4-18-rhel9@sha256:70ecb0886c99f9427b7a464c956a38b46a0c1e1f07cb447f1106a224a07a8640` | `gitops_console_plugin_image_pf5` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/console-plugin-rhel9@sha256:e814cc4ad19806e71380bd6e6d3f6419fb6df2cfdac2b1fc781bc818678c1bb4` | `gitops_console_plugin_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/dex-rhel9@sha256:1294cf5c2d8ba1dcfe199511f1fbf11e219a8c6eedb698e47d1970484056ef60` | `argocd_dex_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/gitops-promoter-rhel9@sha256:24871a04cad45b8fc3e82d2c1c26b7c6695315809c4f786cb254f219ca9ccda1` | `gitops_promoter_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/gitops-rhel9-operator@sha256:f5773d35e496a55999590024d1d8290e916c05d77ebdadcdc3263c29ea1a434a` | `manager` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/gitops-rhel9@sha256:6530b754dfe2d9b5182e32a62d115c5613c064da59c900476570d6be82a4b16b` | `backend_image` | v1.22.2-7 |
| `registry.redhat.io/openshift-gitops-1/must-gather-rhel9@sha256:94438482f584c5b1c6acbdb37783352021fb89d747e238d0ee20d929c79080bb` | `must_gather_image` | v1.22.2-7 |
| `registry.redhat.io/openshift4/ose-haproxy-router-rhel9@sha256:c1a1301fa4175237d863c9e97057c786acb1453e487c2afa189345e76832cde8` | `argocd_redis_ha_proxy_image` | v4.22.0-202609291750.p2.gb70b28c.assembly.stream.el9 |
| `registry.redhat.io/rhel9/redis-7@sha256:8e0c52a2f304473c1f62d1112e116e0e8d541ed9cbeea86f6ece3318146be105` | `argocd_redis_image` | 9.8-1791836411 |

The same list is `../allowed-images.txt`, the reviewed allowlist `render.sh`
and `assemble.sh` enforce for the manager image and every `*IMAGE*` env value
(31 env names; the packaged defaults are exactly these references; there is no
floating tag and no fallback registry).

## The charts

| Archive | Version / appVersion | SHA256 (pinned in `../pins.yaml`) |
| --- | --- | --- |
| `../charts/gitops-operator-crds-0.1.0.tgz` | `0.1.0` / `1.22.2` | `32bbf0742652160b3aaa953036f6bf933a45b79c55aaed9e9fdf13b9fdd9ec9b` |
| `../charts/gitops-operator-0.1.0.tgz` | `0.1.0` / `1.22.2` | `694b209eddc8b16b31e1d0bed2fbee2a8457774d88b8e76ace851c7f0bcd1010` |

`0.1.0` is a truthful local packaging version, distinct from the official
appVersion. The archives have no registry/OCI manifest digest of their own; the
`digest` field in `pins.yaml` records the official **bundle** digest they were
derived from and must not be read as an archive digest. Archives are produced
by `build-charts.sh` + `package-chart.py` (fixed member order, mtime and
ownership, zero gzip timestamp) and rebuilt byte-identically in the same
environment; reproducibility across different zlib builds is not claimed, the
pinned checksum is the authority.

### CRD chart

Eight CRDs, seven byte-identical to the bundle files (checked against
`upstream.sha256`). The generated `crds/` sources are intentionally not
committed (about 7 MB of YAML that only duplicates the pinned archive);
`vendored-crds.sha256` records the digest of every packaged CRD file.

**The one adaptation** (also stated as a changed-file notice, Apache-2.0 section 4(b), in `NOTICE`, which is packaged in both charts): `argocds.argoproj.io` drops the `v1alpha1` version
(served, deprecated, not the storage version) and `spec.conversion`. This
packaging never enables the conversion webhook (no serving certificates exist),
so a Webhook conversion block would point at a service nothing serves. The
`v1beta1` version block is unchanged (asserted by tests; byte-compared to the
bundle when `GITOPS_BUNDLE_DIR` is given). Consequence: v1alpha1 `ArgoCD`
resources cannot be served; the seed and the private chart use v1beta1.

Selected CRDs: `applications`, `applicationsets`, `appprojects` (shared with the
parent; extracted to `bootstrap-gitops/parent-artifacts/shared-crds.yaml`),
`argocds`, `rolloutmanagers`, `notificationsconfigurations`,
`namespacemanagements`, and `controllerconfigurations.promoter.argoproj.io` (a
required dependency API, see `bootstrap-gitops/README.md`). Excluded: ImageUpdater,
Rollouts, AnalysisRun/Template/ClusterAnalysisTemplate, Experiment, GitopsService
and every other Promoter CRD.

### Operator chart

Written independently around the CSV; template code from other charts was not
copied. `values.yaml` and `templates/rbac.yaml` are generated from the CSV. The
manager container's command, args, probes, ports, security context,
volume and volume mount, the pod security context, grace period and both RBAC
rule sets equal the CSV (offline tests diff them); manager `resources` is the one
exempted field (delta 5). Documented deltas:

1. `ENABLE_CONVERSION_WEBHOOK` defaults to `"false"` (CSV `"true"`); the chart
   renders only the literal `"false"` (a native Helm boolean false normalizes to
   it) and fails the render for any other value, with no case-folding or boolean
   parsing.
2. `ARGOCD_CLUSTER_CONFIG_NAMESPACES` is set by the renderer to the instance
   namespace (`argocd`) instead of `openshift-gitops`.
3. `WATCH_NAMESPACE=""` is set explicitly (OLM injects it for AllNamespaces);
   `LABEL_SELECTOR` is the empty string the CSV leaves unset.
4. Not shipped: ServiceMonitor, metrics/webhook Services, the Prometheus
   Role/RoleBinding, the metrics token Secret, the manager-config ConfigMap and
   any default `openshift-gitops` instance.
5. **Manager `resources` (intentional delta from the CSV's `resources: {}`):**
   values-driven, defaulting to an **untuned initial budget** with no
   performance or sizing guarantee and no verified source: requests cpu `100m` /
   memory `256Mi`, limits cpu `1` / memory `1Gi`. The generated `values.yaml`
   carries it (from `build-charts.sh`), and the same statement is in the packaged
   `NOTICE`. Offline parity tests exempt exactly this block and nothing else. The
   budget must be validated before activation (see `bootstrap-gitops/README.md`).

## Interim controller RBAC (temporary additive migration-parity exception)

`bootstrap-gitops/.../controller-rbac.yaml` grants the application controller
ServiceAccount the legacy bootstrap controller's wildcard rules
(`*/*/*` and `nonResourceURLs *`). It is a **temporary, additive
migration-parity exception, not least privilege**, with the exit criterion
**GCP-1285**: scoped permissions covering the managed workloads must be supplied
and validated before the grant can be removed or narrowed. It is not narrowed or
redesigned here.

Why it is not simply dropped (pinned-source evidence, not GKE runtime proof):

- argocd-operator `a213c171036aaae3b90a44754420d14e6752287b` generates a
  ClusterRole and ClusterRoleBinding for the seed (`argocd/gitops`:
  `gitops-argocd-argocd-application-controller`, subject ServiceAccount
  `argocd/gitops-argocd-application-controller`, gated by
  `ARGOCD_CLUSTER_CONFIG_NAMESPACES=argocd`).
- The GitOps operator `cf32771491be2ed7f03d43d279171d6c265f2d2b` registers
  `openshift.ReconcilerHook` unconditionally, outside `IsOpenShiftCluster()`
  (`cmd/main.go` line 365). argocd-operator's `role.go` calls the hook (line 376),
  and `controllers/argocd/openshift/openshift.go` (lines 35-42 and 318-473)
  replaces the stock wildcard rules with narrower ones: cluster-wide read, writes
  limited to rbac/storage-style cluster resources and namespaces, PVC/PV and
  ConfigMaps, and ServiceAccount impersonation. It also rewrites the namespaced
  Role, generated only in instance/managed-by-labelled namespaces.
- So the operator-generated RBAC is not migration-parity-equivalent: without this
  grant the controller would lose writes to Deployment, Service, Secret and
  similar kinds in unmanaged namespaces and cluster writes such as CRDs and
  webhooks that the legacy controller performs.

| File | SHA256 |
| --- | --- |
| gitops-operator `cmd/main.go` @ `cf327714…` | `69d0821362e591c6042818627149e98d45f3efe76d41d90493667a8b90f0722a` |
| gitops-operator `controllers/argocd/openshift/openshift.go` @ `cf327714…` | `85301c8239de18d2f88d0edb40cdd6080583fd60f801d512e6b2f10c3a1e4f96` |
| argocd-operator `controllers/argocd/role.go` @ `a213c171…` | `20c9c379fab26d996923cc7afb929ed5fe85f70899fe658d58a0e5e25388bba3` |
| argocd-operator `controllers/argocd/rolebinding.go` @ `a213c171…` | `de59446b2b63e3e3f5b58c8027aba156add28c65e17479973fb5e54b1c9f9c34` |

The offline tests keep proving that the interim role equals the legacy bootstrap
role's rules and that its binding subject matches the operator's service-account
naming; the manager's `ARGOCD_CLUSTER_CONFIG_NAMESPACES` is checked as the
instance namespace. No runtime closure is claimed.

## Official operator RBAC exception (separate from the controller grant above)

The generated `gitops-operator/templates/rbac.yaml` carries the official CSV's
operator ClusterRole unchanged (rule-for-rule parity, asserted by the offline
tests). Among those rules (about lines 408-424) it grants
`rbac.authorization.k8s.io` resources `*`, `clusterroles` and `clusterrolebindings`
the verbs `bind`, `create`, `delete`, `deletecollection`, `escalate`, `get`,
`list`, `patch`, `update` and `watch`, and the Role/RoleBinding rule that follows
adds create/update/delete. It is bound to the **operator** ServiceAccount
(`openshift-gitops-operator-controller-manager`, in the operator namespace, with
the default names).

`bind` and `escalate` on wildcard RBAC resources let that ServiceAccount create
arbitrary privileged roles and bind them, or bind `cluster-admin`, so the
operator is **cluster-admin-equivalent in practice**. Official CSV parity is **not
least privilege**. Kubernetes' privilege-escalation prevention normally stops a
subject from granting permissions it does not hold, and `bind`/`escalate` are
exactly the verbs that waive that check:
<https://kubernetes.io/docs/reference/access-authn-authz/rbac/#privilege-escalation-prevention-and-bootstrapping>.

Status and boundaries:

- It is retained unchanged **for upstream CSV parity in inactive engineering
  artifacts**. No rule or binding was redesigned or narrowed here.
- It is a **different exception** from the additive Argo CD application
  controller grant in the previous section. GCP-1285 is the exit criterion for
  that application-controller grant **only**; nothing here claims GCP-1285 owns,
  scopes or removes the operator grant.
- Owner and ticket for this exception: **unassigned**, pending a pre-activation
  review.
- Before any activation, a risk and permission review of the operator grant is
  required (including whether upstream supports a narrower role for this
  deployment). No production risk acceptance or approval is claimed or implied.

## Controller-dependency evidence (selected-release assumptions)

Read anonymously at the commits the bundle and its `go.mod` pin
(`go.mod` SHA256 `bd6c0a9f47bc7c606d96f39f1624e931356f553f16685e251ebe38ebe985d382`
pins argocd-operator `v0.20.0-rc1.0.20261011153353-a213c171036a` and
argo-rollouts-manager `v0.0.10-0.20260826124203-67002e569d7f`). This is source
reading of the declared linkage, not a runtime observation and not proof of the
downstream build.

| File | SHA256 | Finding |
| --- | --- | --- |
| gitops-operator `cmd/main.go` @ `cf327714…` | `69d0821362e591c6042818627149e98d45f3efe76d41d90493667a8b90f0722a` | ArgoCD, RolloutManager and NotificationsConfiguration controllers always start; GitopsService only if `IsOpenShiftCluster()`; webhook only if `ENABLE_CONVERSION_WEBHOOK=true` |
| argocd-operator `controllers/argocd/argocd_controller.go` @ `a213c171…` | `af71f8f0af182dda6821506934c06812e2ab9c109424ee0925c2b232afcf0f18` | `NamespaceManagement` handling |
| argocd-operator `controllers/argocd/util.go` @ `a213c171…` | `194c7b8b23c1540d02e9ced62f919fb391501b4705ab396d5b4641971a65f06b` | watches `ArgoCD`, owns `NotificationsConfiguration`, watches `NamespaceManagement`; `reconcileGitOpsPromoter` runs on every reconcile; ImageUpdater detected via API discovery and disabled when absent; deletion path lists `ControllerConfiguration` |
| argocd-operator `controllers/gitopspromoter/controllerconfiguration.go` @ `a213c171…` | `4e5185f554dce35ef852aa60e5bc485df5a41939dba52953de15183d6754e565` | `Get` of the ControllerConfiguration before checking whether Promoter is enabled; any non-NotFound error (a missing API) aborts the reconcile |
| argo-rollouts-manager `controllers/argorollouts_controller.go` @ `67002e56…` | `fa405fbd475ab30ef136f4e5fe387b2b572a24f1321e0603a5de0a9870f6a6c7` | `For(&RolloutManager{})` |

## Config Sync lifecycle/dependency evidence

Config Sync `v1.23.3` pins cli-utils `v0.37.3-0.20251021150641-5895ad6c17dd`
(its `go.mod`, line 60). Read anonymously at that commit (source reading only;
Config Sync was not run):

| File (kubernetes-sigs/cli-utils @ `5895ad6c17dd`) | SHA256 | Finding |
| --- | --- | --- |
| `pkg/apply/filter/prevent-update-filter.go` | `5c515eed8cbdb048c727d2af482030188b92bef6caaa5eac4c09339335a91f48` | an incoming `mutation: ignore` object that already exists returns `AnnotationPreventedUpdateError` ("Object exists - skip apply") |
| `pkg/apply/task/apply_task.go` | `8811f189f83ad4ac1e92f2b5a20ef323d381b2e5e43f6919915617ac0fa9f9c1` | a non-fatal filter error becomes `AddSkippedApply` |
| `pkg/apply/filter/dependency-filter.go` | `fbf9acef10e8563175b463cc3d77a9c1bdc19164bf360aa21f182671ac80e2ff` | a Skipped or Failed dependency yields `DependencyPreventedActuationError` for its dependents |

Consequence used by the assembler: no `depends-on` target may be
`mutation: ignore`, otherwise its dependents are skipped on every sync after
the first. This is a source-derived constraint, not a runtime observation, and
valid annotation syntax alone does not prove safe second-sync behavior. The
`depends-on` token syntax comes from the Config Sync v1.23.3 vendored
`pkg/object/dependson` (`strings.go`).

## Rebuild

```sh
# extract the verified bundle layer into a fresh directory first
chart-src/build-charts.sh --bundle-dir <dir containing manifests/>
```

The script verifies the recorded digests, regenerates the sources, repackages the
archives, rewrites `allowed-images.txt` and `vendored-crds.sha256`, and prints the
new checksums, which must then be copied into `pins.yaml` and reviewed.
