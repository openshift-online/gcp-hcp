# OpenShift GitOps on GKE: GCP HCP discovery and adoption handoff

**GCP-1226, Dev AIO trial, 2026-09-23–30.** Owner: GCP HCP. Parent: [GCP-1186](https://redhat.atlassian.net/browse/GCP-1186). Upstream Route issue: [GITOPS-11466](https://redhat.atlassian.net/browse/GITOPS-11466) / [operator PR #1321](https://github.com/redhat-developer/gitops-operator/pull/1321). This is a discovery record and implementation handoff, **not** a supported install guide or a claim that the production migration is complete. The trials used disposable GKE Autopilot clusters and developer-published OpenShift GitOps 1.22 artifacts. Product support, signatures, upgrade policy and final release images still need confirmation.

**Evidence boundary:** MC and region results below are **Sep 29–30 snapshots**, not current live checks as of this Oct 7 handoff. MC used a locally built `v1.22` + Route-fix image; region used the original developer 1.22 chart image. These trials did **not** validate one identical supported release across both clusters. Findings were recorded in [MC comment 18685991](https://redhat.atlassian.net/browse/GCP-1226?focusedCommentId=18685991) and [region comment 18686632](https://redhat.atlassian.net/browse/GCP-1226?focusedCommentId=18686632).

## Result at a glance

| Trial | Direct observation | Limit |
| --- | --- | --- |
| Clean, isolated GKE | Explicit `ArgoCD` instance became Available with ApplicationSet, Notifications and Dex enabled; seven operand pods Ready. | No existing applications or HCP integration migrated. |
| Dev AIO management cluster (MC) on GKE with HyperShift | Original operator crashed when `route.openshift.io/v1` was present without `config.openshift.io/v1`. A locally built `v1.22` operator image with the three PR #1321 commits started successfully: Route controller running, `ArgoCD/gitops` Available, seven workloads Ready, **14/14 Applications Healthy**. | Functional fix only. Local distroless test image is not a signed z-stream. `argocd-config` remains OutOfSync. |
| Dev AIO region on GKE | The original developer-chart 1.22 operator (no HyperShift Route API here) replaced the old OSS runtime successfully: RootSync Sync Completed, `ArgoCD/gitops` Available, seven workloads Ready, **20/20 Applications Healthy**. Gateway's new backend has a healthy endpoint. | **19/20 Synced**; `argocd-config` remains OutOfSync on three ConfigMaps. End-to-end SSO, GCP identity and notifications not proven. Old GKE backend/NEG cleanup completed automatically after transient warnings. |

This separates **operator compatibility** from **HCP migration**. GitOps-dev owns the Route/Scheme bug in GITOPS-11466; GCP HCP owns bootstrap sources, Config Sync, ESO/IAM, day-two chart configuration, Gateway, and rollout. A successful pod rollout is not equivalent to preserving the previous configuration or product support on xKS.

## Current bootstrap and replacement contract

```text
Dev AIO Terraform (shared region + MC state)
  -> Fleet Config Sync RootSync (cluster-specific public Git directory)
      -> ESO, argocd Namespace, metadata/repo ExternalSecrets, root ApplicationSet,
         shared Argo CD/ESO CRDs, GitOps operator and ArgoCD/gitops CR
          -> operator-managed Argo CD workloads in argocd
              -> root-generated Applications in private gcp-hcp-infra config
                  -> HCP day-two workloads and configuration
```

The HCP team chose an **OSS runtime replacement**, not side-by-side coexistence or deletion of the whole `argocd` bootstrap. On each disposable cluster the reviewed source preserved the namespace, ESO cluster/repo Secrets and SecretStore, root ApplicationSet, and three shared `Application`/`ApplicationSet`/`AppProject` CRDs. Config Sync intentionally pruned the old six Argo CD Deployments and one StatefulSet and applied the operator plus an explicit `argoproj.io/v1beta1 ArgoCD` instance. The instance is named **`gitops` in namespace `argocd`** for the PoC: the operator's admin Secret becomes `gitops-cluster`, avoiding collision with ESO's `argocd-cluster` **cluster metadata** Secret. This is a trial naming decision, not a general convention.

The region's old 109-object public bootstrap and the new 98-object source have **54 identities retained, 55 removed, 44 added**; four retained resources change YAML (Dex seed `argocd-cm` and three shared Argo CD CRDs). The MC trial had the same identity counts. Source identity and parsed-YAML diffs were checked before each switch; this does not prove a production in-place migration or safe rollback. The private `argocd-config` Helm app uses dev-only overrides so the generated Service, HTTPRoute, backend/health policies and PodMonitoring selectors target `gitops-*` names rather than `argocd-*`.

| Cluster | Public Config Sync source at snapshot | Private app-config branch at snapshot | Operator image |
| --- | --- | --- | --- |
| MC | [`bootstrap-gitops-spike` at `be89663`](https://github.com/floresroger/gcp-hcp/tree/be89663bf4a55ff44a43eb21d9a17f449c439bca/bootstrap-gitops-spike), branch `gcp-1226-mc-spike` | [`gcp-1226-config-spike` at `ec7ff580`](https://github.com/floresroger/gcp-hcp-infra/tree/ec7ff580b8985031e796871f20e3b6160567c660) (private) | Public **local test image** `quay.io/rfloresj/gitops-operator@sha256:3063cf8d315d431700f3c084dae4f7b2c512a32d2438a7c2dbab9d959030a8a7` |
| Region | [`bootstrap-gitops-region-spike` at `c59174e`](https://github.com/floresroger/gcp-hcp/tree/c59174e0c23a6e9732eb1e7b893fbd4fa085f720/bootstrap-gitops-region-spike), branch `gcp-1226-region-spike` | [`gcp-1226-region-config-spike` at `b8b569c7`](https://github.com/floresroger/gcp-hcp-infra/tree/b8b569c7a53b762a4898d2c866add29526360c64) (private) | Developer chart's `gitops-rhel9-operator@sha256:8afeee5e1c0e8c0684c119d4106de6c5bf97a8bc2e585d103b049d16ebb6c07f` |

**Evidence retention:** the four public/private trial branch heads were verified against these commits on Oct 7. Private links require repository access. Retain the fork repositories and trial branches through handoff; before retiring them, preserve the referenced manifests and provenance in a team-owned location and update these links. Commit-pinned links prevent branch movement from changing the cited content, but cannot protect against repository deletion. This verification is not a guarantee of future availability.

Org `openshift-online/gcp-hcp:main/bootstrap` and the previously active private `dev-rflores` branch were not modified by these trials. The separate clean test cluster remains provisioned pending cleanup.

## Specific blockers and tested local workarounds

| Finding | Evidence / owner |
| --- | --- |
| Partial OpenShift APIs on GKE MC | Route present / Config absent caused the operator's Route/Scheme startup failure. The local `v1.22` + PR #1321 fix resolved it. GitOps-dev owns [GITOPS-11466](https://redhat.atlassian.net/browse/GITOPS-11466); details below. |
| Config Sync chart render validation | Config Sync rejected top-level `status` on **27 CRDs and three Services** (`KNV1045`). The [generated-manifest correction](https://github.com/floresroger/gcp-hcp/commit/d50f6616cedbb0fbfa639377c62c6f2d667f884d) removes those fields; that commit contains manifest changes, **not the renderer script**. The local credential-free prototype stripped only top-level status and compared parsed YAML to protect nested CRD status schemas, as described below. The forked sources with that fix synced on both clusters. |
| RootSync reconciler memory | The MC's 512-MiB reconciler OOMKilled while applying the larger source (~50 CRDs with ESO). A documented `spec.override.resources` **2-GiB memory request** stabilized it. Region's RootSync received the same approved region-only override **before** cutover. These overrides are live but **outside Terraform**; proposed story #1 must capture declarative management and recovery. No follow-up ticket has been created yet. |
| Operator chart SA/token default | The developer chart's token Secret targets `openshift-gitops-operator-controller-manager`, while its default generated SA name differs; initial clean `helm install --wait` timed out. Explicit chart `serviceAccount.name=openshift-gitops-operator-controller-manager` worked and was included in pinned renders. This local workaround is not a confirmed packaging fix for the supported release. |
| Registry credentials | Reuse the existing `gcp-hcp-commons-dev/default-openshift-pull-secret` through a **bootstrap-owned ESO store/ExternalSecret**, never rendered Docker config data in public Git. MC already had ESO read permission; region required an approved secret-scoped IAM grant to its `external-secrets-system/external-secrets` KSA before switching RootSync. Operator/operand pods successfully pulled pinned images on both clusters. |

### MC Route/Scheme failure

HyperShift installs Route but not the full OpenShift Config API on the **GKE management API server**.
One operator detection path skipped Route Scheme registration while another watched Route;
the manager repeatedly failed with `no kind is registered for the type v1.Route in scheme`.
The old OSS runtime had already been pruned: this was not an OSS coexistence failure.

Cherry-picking PR #1321's three commits onto `v1.22`, passing the focused unit test,
and deploying a local amd64 build **resolved the specific MC crash**.
Confirm the final z-stream artifact separately; do not remove HyperShift's shared Route CRD.
Region lacks both APIs and did not reproduce this failure with the original image.

### Pinned trial artifacts

- Operator chart: `oci://quay.io/anjoseph/openshift-gitops-operator-chart:1.22.0`,
  digest `sha256:1cd5f02e563637a7401da55ec438a1688eddd76dfba3c7dbab99a68b9a257ac8`.
- CRD chart: `oci://quay.io/anjoseph/openshift-gitops-operator-crds:1.22.0`,
  digest `sha256:3280759d93311a8ab884ace6f0db4beea354ab1935830a36fd0944877235eaa3`.
- Core Argo CD operand: `argocd-rhel9@sha256:ec4e9bd577efa235da258f1ea0ab95b7207ac52cca92be6b5437f87dc243eeb2`
  (Argo CD v3.5.2). Redis and Dex had separate pinned images.

These are observed pull/image identities, **not signature or support verification**.

### Tested render settings and validation

The renderer remains a **local `prototype/update.sh`** in the trial author's retained
`gitops-operator` evidence workspace, not a script published by this PR.
There is **no published script branch or Jira attachment to link**.
The engineer taking proposed story #1 should obtain the prototype and its namespace-overlay
files from the trial author, then publish them in the team repository before productizing them.

Its relevant steps were:

1. Pull and unpack both charts; reject OCI digests differing from the pins above.
2. Render from local chart paths to avoid Helm OCI banners in YAML; use `--include-crds` for the CRD chart.
3. Remove only each document's top-level `status`. Compare parsed YAML before/after
   against the original documents with only `.status` deleted; reject any other semantic change,
   including changes to nested CRD status schemas.
4. Validate the expected 27 GitOps CRDs, explicit operator namespaces, and absence of
   embedded Docker credentials. The rendered operator token Secret is a data-free placeholder.

The successful operator renders used these **nondefault chart inputs**, not the chart defaults alone:

- `serviceAccount.name=openshift-gitops-operator-controller-manager` to match the chart's token Secret.
- `env.ENABLE_CONVERSION_WEBHOOK=false`, following the developer's trial example; this is a tested input, not a general supported-release recommendation.
- `env.ARGOCD_CLUSTER_CONFIG_NAMESPACES=argocd` for the existing instance namespace.
- `pullSecret.create=false` and `imagePullSecrets[0].name=redhat-registry-pull-secret` so ESO supplies credentials separately rather than Helm rendering Docker config data.

Helm's `--namespace gitops-operator` did **not** populate `metadata.namespace` on most chart objects. Before Config Sync consumed the render, a Kustomize overlay added `Namespace/gitops-operator` and set that namespace on namespaced operator resources, including checking the ClusterRoleBinding's ServiceAccount subject namespace. The separate `ArgoCD/gitops` instance stayed in `argocd` with ApplicationSet, Notifications and Dex explicitly enabled. These settings describe the tested prototype; they are not a complete installation or rollout procedure.

## HCP adoption gaps revealed by running the trials

### Configuration ownership

Both clusters' `argocd-config` Applications are Healthy but OutOfSync on three ConfigMaps.
The runtime swap did **not** preserve all desired configuration:

| ConfigMap | Observation | Generated owner |
| --- | --- | --- |
| `argocd-cm` | **22 desired keys absent**, including Application/Promoter/KCC health checks, update filters and links. | Argo CD operator |
| `argocd-cmd-params-cm` | Operator defaults conflict with the chart's empty data map. | Argo CD operator |
| `argocd-notifications-cm` | **Seven desired keys absent**, including `service.github`, context and status triggers/templates. | `NotificationsConfiguration` controller |

The `ArgoCD` CRD exposes `spec.extraConfig` and structured fields;
`NotificationsConfiguration` exposes service/context/template/trigger maps.
Move HCP day-two values into supported CR ownership and retire competing Helm ConfigMaps
only after validation. This is an **HCP implementation design**, not a tested resolution yet.

### Notifications

- MC's ESO `creationPolicy: Merge` target was initially empty because the operator created it
  after ESO's last hourly refresh. A later refresh became `SecretSynced` and populated the
  expected **key name** `github-app-private-key`; no value was read.
- At the Sep 30 snapshot, region's target was empty: ESO's last refresh at 01:23:14 UTC
  preceded operator target creation at 01:23:16 UTC. Recheck after a post-creation refresh
  before calling the region merge broken.
- Actual GitHub status delivery remains untested.

### Permissions and identity

PoC RBAC mirrors the previous controller's broad cross-namespace permissions for continuity,
not least privilege. Terraform changed the Argo GSA IAM member to the operator-generated
`argocd/gitops-argocd-application-controller` KSA, but that SA has **no**
`iam.gke.io/gcp-service-account` annotation.

Kubernetes `can-i` passed for cross-namespace Deployment creation;
intended GCP API identity/access was **not** proven.
This is separate from the operator SA and needs HCP design/test work.

### Gateway/IAP

- Both `argocd-route` HTTPRoutes point at `gitops-server:443`; unauthenticated HTTPS redirects to IAP.
- Region's new GKE backend has a **HEALTHY** endpoint; its separate Gecko platform API route
  stayed Accepted/ResolvedRefs. No authenticated browser login or Dex/IAP flow was completed.
- During the region Service swap, an old backend reference briefly prevented deletion of
  the old `argocd-server` NEG. A later check confirmed the old NEG/backend were
  **removed automatically**; no manual cleanup was needed.

### Product/lifecycle contract

Confirm supported/signed xKS artifacts and exact release version, Promoter and Source Hydrator
packaging and actual controller/image provenance, shared CRD schema migration/ownership,
and operator/operand upgrades and rollback.

CRDs and optional image settings alone do **not** prove Promoter/Source Hydrator were deployed.
A clean replacement on disposable clusters does not validate a seamless production upgrade
or rollback. Any claim that 1.22 GKE is supported is **unconfirmed** by this engineering trial.

## Operational notes and proposed HCP stories

**Team review feedback** highlighted OutOfSync `argocd-cm` configuration ownership and
authenticated IDP login as important but solvable adoption work. Prioritize those gaps
under GCP-1186; this feedback is not evidence that either is resolved or authorization to close Jira.

The Dev AIO region and MC share **one Terraform state** in the development shared-state
GCS backend under the trial author's personal AIO prefix. The actual root is a locally
git-ignored developer directory under `terraform/config/dev-all-in-one/`; Git status
does not reveal its trial input edits. Exact backend, prefix and root details remain in
the trial author's retained operational notes; obtain and verify them before any state operation.

MC-specific and region-specific Git directories and private config revisions were reviewed
separately. Before region cutover, the full refreshed plan included an **unrelated MC GKE
monitoring-component order** update (same components, still an API change).
The user explicitly approved a *new saved five-action targeted region plan* that excluded MC;
only that plan was applied.

A future complete plan or restore must account for both clusters, branch/dir overrides,
the out-of-Terraform RootSync 2-GiB overrides and old runtime pruning.
Whole-AIO teardown/rebuild is an acceptable fallback in principle, not an automatic action.

Suggested GCP HCP stories under [GCP-1186](https://redhat.atlassian.net/browse/GCP-1186), **not sub-tasks of the spike**:

These are proposed scopes, **not created tracking tickets**; add issue links when they are created.

1. **Productized render and bootstrap:** obtain and publish the local renderer and namespace-overlay
   files in the team repository; pin and validate chart-to-Config-Sync output. Include secure ESO
   credentials, explicit feature CR, shared CRD/prune contract and generalized MC/region identities.
   **Replace the manual RootSync memory overrides with a reviewed declarative ownership path**;
   validate reconciler sizing, persistence across reconciliation, and recovery/rebuild behavior.
2. **HCP configuration migration:** translate custom health/update config and GitHub notifications
   into supported CR inputs; resolve ConfigMap ownership. Validate ESO delivery, IAP/Dex, Gateway
   and GCP identity, and narrow workload RBAC.
3. **Signed artifacts and optional components:** confirm support/certification and adopt downstream
   Promoter/Source Hydrator with known compatible versions and running-image evidence.
4. **Lifecycle and rollout:** release pinning, CRD/operator/instance ordering, upgrade/rollback,
   verification of Gateway backend cleanup, region/MC sequencing and production-environment rollout.
   Preserve fork-based trial evidence in a team-owned location before retiring the trial repositories.

The spike can conclude with **verified technical results, explicit unknowns and these story scopes ready to size**; completing these implementation stories is separate work. See [GCP-1226 trial comments](https://redhat.atlassian.net/browse/GCP-1226) for dated MC/region evidence and the upstream issue for product-fix status.
