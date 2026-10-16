# Manage HCP Argo CD configuration through operator CR inputs

***Scope***: GCP-HCP

**Date**: 2026-10-13

**Status**: Proposed record of the agreed drop-in direction; runtime cutover validation and rollout approval pending.

## Decision

Use an **in-place/drop-in replacement**: satisfy the operator-managed Argo
requirements, take the old Argo runtime offline by scaling its owning workloads
to zero, then let operator-managed Argo reconcile the retained resources. Do not
run old and new application controllers against the same resources concurrently.
Scaling down is not namespace deletion or permission to prune managed workloads.

Declare HCP configuration through supported ArgoCD/NotificationsConfiguration
inputs, with no competing generated-ConfigMap writers. Keep forward configuration
backwards compatible with existing/unmigrated non-operator Argo targets; keep
operator mode opt-in per target. Quiescing the old runtime and releasing old
configuration writers are separate, required checks.

**No OLM at either layer.** Render pinned operator/CRD Helm charts into ordinary
Kubernetes manifests in Git; Fleet Config Sync delivers them to the cluster. The
operator then directly reconciles Argo CD through CR inputs. Neither installation
of the operator nor its management of Argo CD uses Operator Lifecycle Manager
(OLM), OLM Subscriptions, CatalogSources or InstallPlans. Operator-managed does
not mean OLM-managed. Availability/support and provenance of the selected release's
Helm artifacts still require separate verification; this is not a claim about
every GitOps release or a standalone non-operator Argo chart.

The October 13 trials validated configuration ownership on existing
operator-managed instances. They did **not** validate this complete old-runtime
scale-down and replacement sequence, organizational login, or actual notification
delivery. Historical seed-handoff tooling supports that ownership work where
needed; it is not an alternative rollout direction or a complete replacement tool.

## Context

- **Problem:** GCP-1226 found operator reconciliation overwrote required HCP
  ConfigMap settings. Ready runtimes and Healthy Applications did not establish
  equivalent custom health, update filters, auth or notification configuration.
- **Constraints:** Preserve the Terraform → Fleet Config Sync → Argo CD boundary,
  bootstrap/root chain, shared resources, disabled-admin baseline and external
  credential ownership. Keep the old runtime inactive before new reconciliation
  starts. Any later cleanup/deletion must be separately scoped and approved. Never
  use drift suppression, operator disablement or local users to conceal a
  conflicting owner. MC and region require independent inputs/review.
- **Assumptions:** The selected operator/CRDs expose the inventoried supported
  fields. Revalidate contract and behavior for another artifact; trial images are
  not official signed/support certification. An unsupported required setting is
  an explicit product/integration blocker, not permission for an unsafe bypass.

This specializes the existing
[deployment-tooling swim lanes](deployment-tooling-swim-lanes.md); it does not
move Kubernetes resources into Terraform or add a separate infrastructure lane.

## Alternatives Considered

1. **Supported CR inputs with one owner (selected):** Private day-two Argo inputs
   own the adopted or newly bootstrapped CRs; operator controllers generate the
   ConfigMaps. The selected cutover quiesces the old runtime first, then replaces
   its active control loops while retaining managed resources.
2. **Continue direct Helm/Config Sync CM writes:** Rejected; concurrent writers
   produce partial-sync fights and values that do not survive reconciliation.
3. **Ignore drift or stop operator reconciliation:** Rejected; green status would
    hide missing configuration or eliminate the ownership contract being adopted.
4. **Delete/rebuild or concurrent old/new Argo as the normal cutover:** Not the
   selected direction. Keep the old runtime scaled down rather than deleting
   shared resources or allowing both application controllers to reconcile them.

## Decision Rationale

- **Justification:** Structured fields express auth, RBAC, health and exclusions;
  supported extraConfig handles remaining required keys. Notification fields
  express service/context/templates/triggers without embedding credentials.
- **Evidence:** The [sanitized MC/region record](../../docs/openshift-gitops-configuration-validation.md)
  accounts for 26 CM/eight notification/two RBAC values and demonstrates staged
  configuration ownership/prune transfer, representative behavior, two Application
  cycles and one intentional manager restart on each disposable topology. This
  supports configuration readiness, not proof of the newly agreed runtime cutover.
- **Comparison:** CR ownership survives operator reconciliation without the
  competing CM declarations or drift-suppression workarounds in alternatives 2–3.

Final owners:

| Inputs/resources | Declarative owner | Runtime reconciler |
| --- | --- | --- |
| Operator/CRDs, namespace, bootstrap ESO/metadata/repository/root resources | Fleet Config Sync bootstrap | Existing bootstrap controllers |
| ArgoCD spec, adopted or newly bootstrapped | Private argocd-config chart after release of any competing Config Sync seed owner | Argo CD operator |
| Default NotificationsConfiguration spec | Private argocd-config chart | NotificationsConfiguration controller |
| Generated Argo CD ConfigMaps | No direct chart/bootstrap writer | Respective operator controllers |
| Registry/OAuth/GitHub credential values | Existing approved external secret path | ESO and existing Secret owner |

The [replacement guide](../../docs/openshift-gitops-configuration-handoff.md)
defines one sequence: readiness → old runtime offline → operator-managed takeover
→ acceptance → separately reviewed cleanup. Capture the existing Argo application
tracking/installation identifiers and annotations; verify the replacement uses
the intended identities before it can prune resources. A recreated Kubernetes
object gets a new UID: that is distinct from preserving Argo tracking identity.

Scale the owning Deployments/StatefulSets through reviewed declarative inputs,
not just a child ReplicaSet. Verify Config Sync, autoscalers or other controllers
cannot restore old replicas. Config Sync reconciles continuously, not only at
initial bootstrap. A temporary pause or targeted development sync needs its own
explicit scope and resume conditions; this decision does not authorize one.
For reused workload identities, release old declarative management before the
operator restores replicas; Config Sync must not keep enforcing zero on a new
operator-owned object. Pre-stage pruning safeguards and identify an independent
bootstrap executor before stopping the old Argo.

Scaling down also does not release a surviving Config Sync seed owner or stop
direct chart ConfigMap writes. Where the target has the trial seed overlap, the
optional components support protection → release → CR adoption → guarded tracking
retirement, including the mutation-ignore bridge. Preserve their observed gates;
they do not implement old-runtime scale-down or automatically adopt all resources.
Bootstrap/recovery must work without depending on the old Argo being active.

## Consequences

### Positive

- Required configuration has one declarative input owner and supported runtime
  generation, with mapping/precedence tests and explicit per-cluster inventory.
- Legacy/default deployment behavior stays unchanged until reviewed target opt-in.
- Retained applications/shared resources and a single active controller set avoid
  unnecessary teardown while preserving an explicit acceptance boundary.

### Negative

- Cutover has a control-plane interruption and needs coordinated declarative
  revisions, old-writer shutdown, identity/ownership checks and observed acceptance.
  Neither replica count nor Git convergence alone proves successful adoption.
- Scaled-down old definitions are not an automatic rollback. Restoring old
  controllers requires stopping new reconciliation and reviewing configuration,
  tracking and ownership; do not reactivate two competing controller sets.
- Releasing the instance removes automatic Config Sync recreation on a fresh
  cluster. Protected-seed rebuild/reclaim orchestration remains GCP-1280/GCP-1288
  work; do not reverse ownership by blindly replaying an old spec.
- Configuration contracts are version-sensitive. The trial name `gitops` and
  engineering artifact choices are not production naming/support decisions.

## Cross-Cutting Concerns

### Security

- Preserve HTTPS/Gateway/IAP, Dex/RBAC configuration and disabled admin/anonymous
  access; never add a local-user fallback. Actual organizational authentication
  and signed-identity trust-boundary verification remain GCP-1283 acceptance.
- Credential values stay outside Git/logs/evidence. Configuration preservation
  is not proof of GitHub delivery or workload cloud identity (GCP-1284/GCP-1285).
- Chart tuning cannot override chart-owned auth settings, bypass the approved
  transport path or disable essential runtime components.

### Reliability and operability

- Compare rendered identity/prune effects, served schemas and all required values;
  verify actual behavior and persistence, not only readiness/key presence.
- Guard one-time metadata retirement with fresh UID/RV/owner/value tests; preserve
  data, lifecycle/operator metadata and adopted-CR prune/delete protections.
  Inventory retained resources and ownerReferences/finalizers/garbage-collection
  effects before any later cleanup; scale-down is not a namespace/shared-CRD wipe.
- Record stop/recovery boundaries. After adoption, roll forward or restore reviewed
  CR inputs; reintroducing legacy CM writers is not a safe rollback.
- No Terraform/IAM, artifact upgrade or default bootstrap replacement is part of
  this decision. Full release, lifecycle and production rollout remain separate
  adoption gates with explicit target authorization and applicable team DoD.
