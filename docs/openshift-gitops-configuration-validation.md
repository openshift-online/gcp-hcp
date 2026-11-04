# GCP-1282: Disposable MC and region configuration validation

**Evidence date: October 13, 2026.** This records bounded configuration ownership
validation, not a production rollout or story closure. The team implementation
defaults to legacy mode; no environment is enabled by publishing these changes.
See the [agreed drop-in replacement plan and supporting seed handoff](openshift-gitops-configuration-handoff.md) and
[proposed ownership decision](../design-decisions/automation/operator-managed-argocd-configuration.md).
The companion settings matrix/tests live in
`gcp-hcp-infra/docs/ARGOCD_OPERATOR_CONFIGURATION.md` and
`gcp-hcp-infra/argocd/scripts/test_argocd_config.py`.

## Scope and provenance

Reviewed backports were delivered through separate observed revisions on existing
personal trial branches. This records the **configuration-ownership handoff on
existing operator-managed instances**, not the complete newly agreed old-runtime
scale-down and drop-in takeover. It also does not validate a delete/rebuild path.
The default team bootstrap does
not install the operator as a result of these changes. The companion implementation
[PR #2315](https://github.com/openshift-online/gcp-hcp-infra/pull/2315) subsequently
merged on October 14 as `fba968fac27d7dcd62aea60beabecf9969d3f3ce`; that does not
change the exact trial revisions and evidence below.

| Source | MC final revision | Region final revision |
| --- | --- | --- |
| Public `floresroger/gcp-hcp` | `9f76f8b1ce90b6f71bcd5a92b7f062450d36b278` (`gcp-1226-mc-spike`, `bootstrap-gitops-spike`) | `80ee4e3e1c0f154ad603e07b40c99439b6129743` (`gcp-1226-region-spike`, `bootstrap-gitops-region-spike`) |
| Private `floresroger/gcp-hcp-infra` | `b56594f10dc03cdb0b6915612a9105745c29d56f` (`gcp-1226-config-spike`) | `cf660fd0961a75c79e8eca8543e4705902e4a5fc` (`gcp-1226-region-config-spike`) |

The MC manager used the unsigned engineering Route/Scheme-fix image
`quay.io/rfloresj/gitops-operator@sha256:3063cf8d315d431700f3c084dae4f7b2c512a32d2438a7c2dbab9d959030a8a7`.
Region retained its original trial image
`quay.io/redhat-user-workloads/rh-openshift-gitops-tenant/gitops-rhel9-operator@sha256:8afeee5e1c0e8c0684c119d4106de6c5bf97a8bc2e585d103b049d16ebb6c07f`.
Both used the Argo CD image
`quay.io/redhat-user-workloads/rh-openshift-gitops-tenant/argocd-rhel9@sha256:ec4e9bd577efa235da258f1ea0ab95b7207ac52cca92be6b5437f87dc243eeb2`.
Served ArgoCD/NotificationsConfiguration CRD specs were compared and matched.
Schema compatibility does **not** imply identical operator startup behavior or
signed/supported release validation. No image upgrade occurred.

## Ownership transition results

Each cluster followed protection → Config Sync release → existing-CR adoption →
guarded tracking/seed retirement. Gates were observed before proceeding. These
configuration checks support the agreed replacement plan where seed ownership
must transfer; the trials did not demonstrate quiescing a legacy non-operator
runtime and adopting its resources with a new controller set. Do not treat the
recorded CR/CM UID continuity as proof of matching Argo tracking identity across
that runtime replacement.

| Gate | Observed on both clusters |
| --- | --- |
| Protect | Four existing Helm CMs retained their UIDs/owners with `Prune=false,Delete=false`; both seeds had deletion-detach protection |
| Release | Config Sync removed only the ArgoCD and Dex-CM seed management/inventory references, 98→96; both resources stayed live |
| Adopt | The chart changed 17→15 declarations: four competing CMs retired, existing ArgoCD and default NotificationsConfiguration adopted; unrelated chart/root objects unchanged |
| Required configuration | 26 required argocd-cm settings, eight notification settings, two RBAC settings and command defaults reproduced; admin/anonymous disabled; per-cluster URL/project/auth inputs preserved |
| Retire | Fresh UID/RV/owner/value-guarded metadata patches removed only obsolete CM tracking/temporary protection; data/owners/operator labels/lifecycle and adopted-CR protections retained |
| Continuity | Original CR/CM/root/namespace identities retained; region Argo/platform API HTTPRoutes and Gateways unchanged; no recurring ownership/tracking fights observed |

The required baseline is the **desired private configuration**, not conflicting
live values before adoption. Correcting an operator-generated internal URL or
enabled-admin default is intentional. Complete post-adoption CM data fingerprints
were subsequently stable; this is not a claim that adoption left conflicting
pre-transition data unchanged.

## Behavior and persistence results

| Check | MC | Region |
| --- | --- | --- |
| Offline suite with served CRDs and handoff components | 25 tests passed | 25 tests passed |
| Deployed-binary health fixtures | 20 passed | 20 passed |
| Actual representative CM watch rule | Positive controls before/after; no resource-update refresh in 45-second ignored window | Positive controls before/after; no resource-update refresh in 180-second ignored window; pinned controller and non-silent log activity |
| Two explicit Argo Application normal-refresh cycles | `16:04:43→16:04:47`, `16:04:47→16:05:12` UTC | `18:27:58→18:29:59`, `18:29:59→18:30:15` UTC |
| One intentional manager restart | `16:11:13` UTC; same image/ReplicaSet, Ready, zero container restarts | `18:30:53` UTC; same image/ReplicaSet, Ready, zero container restarts |
| Final dated check | `18:35:44` UTC: 14/14 Healthy/Synced, inventory 96 | `18:45:35` UTC: 20/20 Healthy/Synced, inventory 96 |
| Operand continuity | Seven pod UIDs/images/restart counts unchanged from the MC runtime-test baseline | Seven pod UIDs/images/restart counts unchanged from before the region transition |

Health fixtures exercised Application propagation/missing status, representative
IAM/SQL/PubSub Ready conditions, PromotionStrategy open-PR/error/stale-generation
states and ChangeTransferPolicy open-PR/missing-status/deletion states. They ran
inside the existing pinned repo-server with captured non-secret live CM settings,
`--load-cluster-settings=false` and temporary files removed afterward. They did
not create Kubernetes/cloud resources or exercise a real promotion workflow.

The actual watch test used an existing tracked, non-secret monitoring ConfigMap.
Guarded metadata-only nonignored controls surrounded an ignored autoscaler
annotation update. Both annotation states were restored. The MC positive-after
window also contained a comparison-expiry refresh; only the resource-specific
update request is attributed to the control. Neither probe certifies every rule
or indefinite refresh suppression. The two cycles are Application reconciliations,
not operator-generation changes.

**CLI limitation retained:** five MC and three region admin filter cases were
unsupported, **not passed**. Matching Argo CD v3.5.2 source moves update rules to
IgnoreDifferences and clears IgnoreResourceUpdates; the admin command then checks
that cleared field. See [settings conversion](https://github.com/argoproj/argo-cd/blob/v3.5.2/util/settings/settings.go)
and [admin command](https://github.com/argoproj/argo-cd/blob/v3.5.2/cmd/argocd/commands/admin/settings.go).
The live watch probes, not these CLI cases, supply representative filter evidence.

The server pods had **24 MC / 48 region pre-existing restarts**; these were not
caused by the validation and did not increase through the recorded checks.
Do not interpret Ready status as absence of historic workload issues.

## Separate region autoscaler event

An extended check rejected an unexpected manager UID change after the initial
restart proof. The event identified `cluster-autoscaler`, reason `ScaleDown`, at
18:42:01 UTC: node scale-down removed the intentional replacement pod. The same
ReplicaSet created another same-image pod, Ready at 18:42:16 UTC after a brief
scheduling delay. Fresh scoped ArgoCD/NotificationsConfiguration reconciliation
and stable settings/identities were verified at 18:45:35 UTC.

This was **one intentional restart plus one separate autoscaler replacement**,
not a second requested restart or a container crash. No remedial sizing/image
change was made. Zero container restarts does not mean zero pod churn or
uninterrupted manager availability. The MC remained unchanged during region work.

## Repeating the checks and retaining evidence

1. Independently pin the target's sources/artifacts/CRDs and record required
   settings, original resource identities, owners, tracking, inventory and runtime
   status. Never copy MC identities or credentials to a region.
2. To repeat these configuration-ownership checks, use the supporting seed
   handoff's separate render/prune/ownership gates. The agreed drop-in runtime
   replacement separately requires old-controller scale-down, intended tracking
   identity verification and takeover evidence. Run the offline suite with both
   served CRDs and companion components; the earlier results do not substitute
   for the runtime cutover test.
3. For runtime health, execute `argocd admin settings resource-overrides health`
   against representative file fixtures and a captured live non-secret CM;
   explicitly disable cluster-settings reads. Ready pods/key presence alone do
   not prove health scripts or update behavior.
4. Record actual tracked-resource update logs with positive controls around a
   bounded ignored-field change. Journal mutations before issuing patches,
   guard UID/RV/current values, reconcile uncertain responses with fresh reads,
   restore original metadata and reject incomplete/ambiguous observations.
5. Request two normal Application refreshes with guarded metadata updates;
   require consumed requests, advancing reconciledAt, matching completed logs and
   stable required settings/fingerprints after each.
6. Obtain separate restart approval. Recheck fresh image access and API discovery
   before one UID/RV-guarded normal-grace manager-pod restart; await the actual
   replacement's expected owner/image/Ready state and fresh scoped reconciliation.
   The original image's Route-present/Config-absent startup hazard must stop a
   restart, not trigger an unapproved image change. Recheck settings and operands;
   preserve and explain unexpected replacements rather than silently accepting them.

Full baseline/source-diff/log archives remain retained by the trial author; they
are deliberately not committed wholesale because they contain personal topology
and auth-policy references. This team document retains sanitized outcomes,
method, source/image provenance and limitations. Arrange approved team evidence
retention before retiring trial branches/images; never publish Secret values,
tokens, browser cookies or raw credential-bearing configuration.

## Subsequent authentication validation — October 21–23

Separate GCP-1283 checks subsequently validated the established HCP IAP/Dex flow
on the existing disposable operator-managed region and MC: organizational login
and identity, existing admin/read-only permissions, actual permitted Application
reads and denied creates, logout/re-login, and fresh login after separately
approved same-image Dex replacements. The
[dated authentication record](https://github.com/openshift-online/gcp-hcp-infra/blob/0a523f7fc9bb480e91ea608e3dfe358ef380be1f/docs/ARGOCD_IAP_VALIDATION.md)
in [gcp-hcp-infra PR #2439](https://github.com/openshift-online/gcp-hcp-infra/pull/2439)
retains source/image pins, user-report/screenshot distinctions and security limits.

These later results are not part of the October 13 configuration-ownership trials
or proof of the newly proposed legacy-controller scale-to-zero/takeover sequence.
They preserve the existing flow rather than selecting a new authentication design.
The record identifies the pre-existing C4 hardening concern without claiming its
remediation or a passing signed-IAP/direct-backend negative. Actual GitHub
notification delivery remains a separate GCP-1284 acceptance gate.

## Explicit boundaries

The October 13 configuration-ownership trials did not prove organizational IAP
browser login, actual GitHub status delivery, workload-cloud identity, official
artifact/trust certification, optional-component workflow, production rollout or
whole-cluster rebuild. The later GCP-1283 authentication results are separately
attributed above. The linked GCP-1280 and GCP-1283–1289 stories govern their
respective adoption gates; this record does not mark them all complete.

The retired trial seeds cannot independently cold-start Argo CD after a cluster
rebuild. A separately reviewed protected-seed/reclaim and exact-spec recovery
plan is necessary. Successful manager replacement is not full-rebuild proof.
Trial success alone does not satisfy remaining design-review/merge or team DoD
requirements, and this report does not authorize another deployment.
