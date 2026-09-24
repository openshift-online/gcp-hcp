# GCP Marketplace SaaS Integration for Gecko Platform

## Overview

This document specifies the GCP Marketplace SaaS integration for the Gecko Platform. It defines the entitlement CRD, condition-based reconciliation, event handling, Procurement API client, sign-up integration contract, billing handoff, controller deployment, and observability.

This implements the architecture decided in [gcp-marketplace-integration](../design-decisions/governance/gcp-marketplace-integration.md).

**Epic**: TBD

**Repository**: [gecko](https://github.com/openshift-online/gecko)

**Related decisions**:
- [espv2-api-frontend](../design-decisions/networking/espv2-api-frontend.md) — ESPv2 API frontend and Service Control integration point
- Cedar authorization consuming marketplace-created RoleBindings
- Planned cross-region resource replication — RoleBinding replication to non-primary regions; tracked separately and required before marketplace-created access is considered multi-region complete

**Evidence**: The proof-of-concept presented to the team (see team's demo recordings) validated the purchase → sign-up → RBAC provisioning → Cedar authorization flow end-to-end in a two-cluster Kind environment.

---

## Design Decisions

| Decision | Choice |
|---|---|
| Entitlement storage | `MarketplaceEntitlement` CRD (cluster-scoped) with Kubernetes-style status conditions |
| Event transport | Google Cloud Pub/Sub (v2 client library) |
| Purchase topic ownership | Google-owned — Google publishes entitlement lifecycle events |
| Sign-up topic ownership | Gecko-owned — sign-up page publishes completion events |
| Sign-up page ownership | Partner Enablement Team (separate Go module, Pub/Sub event contract) |
| Sign-up identity | Verified identity required (contract specifies requirements) |
| Namespace naming | GCP project number (immutable, unique, aligns with billing context) |
| Entitlement identity | Normalize Marketplace event IDs into full Procurement API resource names using configured `providerId`; use deterministic DNS-safe Kubernetes object names |
| Multiple orders | Disabled for launch; if enabled later, sign-up events must carry entitlement/order identity, not just account identity |
| Initial access | First sign-up user gets `service-admin` PlatformRole binding |
| Controller deployment | Single-active region, `replicas: 0` in standby regions, DR via scaling |
| Procurement approval | Real Procurement API via `ProcurementClient` with WIF auth |
| Idempotency | Check-before-write; reject conflicting email on duplicate sign-up |
| Lifecycle events | Full: creation, plan change, pending cancellation, cancellation revert, final cancellation, deletion |
| Namespace cleanup | 30-day grace period after final cancellation before deletion; immediate cleanup for Google deletion/account deletion events |
| Billing handoff | Persist Procurement API `usageReportingId` and expose `MeteringReady`; billing reporter and Service Control metric implementation are separate DD/IP scope |
| Topic naming | `mkt_` prefix (Pub/Sub rejects `goog` prefix — Google reserved) |
| Poison message handling | Dead-letter topic after configurable max delivery attempts |
| Cross-region consistency | Via planned cross-region resource replication; tracked separately and required before marketplace-created access is considered multi-region complete |

---

## MarketplaceEntitlement CRD

Cluster-scoped. Tracks the full lifecycle of a customer's Marketplace entitlement. The Kubernetes object name is a deterministic DNS-safe value derived from the full Procurement entitlement resource name, not the raw Google entitlement ID. Store the raw ID and full resource name in spec for API calls and auditability.

```go
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type MarketplaceEntitlement struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              MarketplaceEntitlementSpec   `json:"spec"`
    Status            MarketplaceEntitlementStatus `json:"status,omitempty"`
}

type MarketplaceEntitlementSpec struct {
    ProviderID           string `json:"providerId,omitempty"`
    EntitlementID        string `json:"entitlementId,omitempty"`
    EntitlementName      string `json:"entitlementName,omitempty"`
    ProcurementAccountID string `json:"procurementAccountId,omitempty"`
    ProjectNumber        string `json:"projectNumber,omitempty"`
    ProjectID            string `json:"projectId,omitempty"`
    Product              string `json:"product,omitempty"`
    Plan                 string `json:"plan,omitempty"`
}

type MarketplaceEntitlementStatus struct {
    UserEmail          string       `json:"userEmail,omitempty"`          // not +orlop:public
    MarketplaceUserIdentity string  `json:"marketplaceUserIdentity,omitempty"` // not +orlop:public
    Namespace          string       `json:"namespace,omitempty"`          // +orlop:public
    CleanupNamespace   string       `json:"cleanupNamespace,omitempty"`   // not +orlop:public
    PendingPlan        string       `json:"pendingPlan,omitempty"`        // +orlop:public
    UsageReportingID   string       `json:"usageReportingId,omitempty"`   // not +orlop:public
    LastMarketplaceEventID string    `json:"lastMarketplaceEventId,omitempty"` // not +orlop:public
    LastMarketplaceEventTime *metav1.Time `json:"lastMarketplaceEventTime,omitempty"` // not +orlop:public
    LastSuspensionSignalKey string `json:"lastSuspensionSignalKey,omitempty"` // not +orlop:public
    LastSuspensionSignalObservedAt *metav1.Time `json:"lastSuspensionSignalObservedAt,omitempty"` // not +orlop:public
    CreatedAt          *metav1.Time `json:"createdAt,omitempty"`          // +orlop:public
    ActivatedAt        *metav1.Time `json:"activatedAt,omitempty"`        // +orlop:public
    SuspendedAt        *metav1.Time `json:"suspendedAt,omitempty"`        // +orlop:public
    PendingCancellationAt *metav1.Time `json:"pendingCancellationAt,omitempty"` // +orlop:public
    CancelledAt        *metav1.Time `json:"cancelledAt,omitempty"`        // +orlop:public
    DeletionScheduledAt *metav1.Time `json:"deletionScheduledAt,omitempty"` // not +orlop:public
    Conditions         []metav1.Condition `json:"conditions,omitempty"`  // +orlop:public
}
```

### Labels

| Label | Value | Purpose |
|---|---|---|
| `marketplace.gcp.io/account` | Procurement account ID | Lookup by account ID (sign-up handler) |
| `marketplace.gcp.io/project` | GCP project number | Lookup by project (observability) |
| `marketplace.gcp.io/entitlement-key` | Label-safe truncated hash of full Procurement entitlement resource name | Reverse lookup without storing non-DNS-safe IDs in object names |

### Naming

The controller normalizes Marketplace identifiers before creating or looking up Kubernetes resources:

1. Read `providerId` from controller configuration and validate that Pub/Sub events carry the expected provider.
2. Treat `event.entitlement.id` as a raw entitlement ID unless it is already a full `providers/<providerId>/entitlements/<id>` resource name.
3. Build `spec.entitlementName` as `providers/<providerId>/entitlements/<entitlementId>`.
4. Set `metadata.name` to a deterministic DNS-safe value, for example `mkt-<first-32-hex-sha256(entitlementName)>`, and label the object with `marketplace.gcp.io/entitlement-key=<first-32-hex-sha256(entitlementName)>`. The full digest can be stored in a private annotation if collision investigation is needed.
5. Use `spec.entitlementName` for all Procurement API calls, never `metadata.name`.

### Privacy

`UserEmail`, `MarketplaceUserIdentity`, `CleanupNamespace`, `UsageReportingID`, `LastMarketplaceEventID`, `LastMarketplaceEventTime`, `LastSuspensionSignalKey`, `LastSuspensionSignalObservedAt`, and `DeletionScheduledAt` are deliberately excluded from the public API via `+orlop:public` marker omission. All other status fields are customer-visible.

---

## Entitlement Lifecycle and Conditions

The CRD uses Kubernetes-style status conditions as the primary reconciliation contract. Events set or clear conditions, and the reconciler derives the next required action from the condition set. This avoids overloading a single local state with independent facts such as entitlement approval, sign-up completion, access suspension, pending cancellation, cleanup, and metering readiness.

### Conditions

| Condition | Meaning When True |
|---|---|
| `EntitlementApproved` | Procurement API entitlement approval succeeded |
| `EntitlementActive` | Procurement API reports the upstream Marketplace entitlement is active |
| `SignupCompleted` | Verified sign-up event received with Marketplace account/user identity and verified email |
| `AccountApproved` | Procurement API account `signup` approval succeeded |
| `NamespaceReady` | Customer namespace exists |
| `AccessReady` | Marketplace `service-admin` RoleBinding exists and access is active |
| `Suspended` | A supported Google signal indicates access must be revoked, such as Procurement API inactive state or Service Control check errors |
| `PendingCancellation` | Customer requested cancellation, but Marketplace has not finalized it; existing access remains active |
| `Cancelled` | Marketplace cancellation is final; access is revoked and cleanup timer is running |
| `Deleted` | Google deletion/account deletion was received or cleanup grace period expired; resources are cleaned up |
| `MeteringReady` | Procurement API `usageReportingId` is available for Service Control reporting |

### Condition Contract

Every condition update uses `metav1.Condition` and must set `type`, `status`, `observedGeneration`, `lastTransitionTime`, `reason`, and `message`.

Condition writers follow these rules:

- Set `observedGeneration` to `metadata.generation` for the `MarketplaceEntitlement` being reconciled.
- Update `lastTransitionTime` only when the condition `status` changes.
- Use stable CamelCase `reason` values suitable for automation and alerting.
- Use `message` for human-readable operational detail; do not include secrets or raw credentials.
- Maintain exactly one entry for each `Condition.Type`; writers update existing entries by type instead of appending duplicates.
- Treat duplicate condition types as invalid and not ready until the reconciler normalizes the set.
- Allow only `Status=True` to satisfy positive readiness gates. `Status=False`, `Status=Unknown`, and absent conditions are not ready.
- Treat spec-dependent readiness conditions with stale `observedGeneration` as not ready. For example, `AccessReady=True` with `observedGeneration < metadata.generation` must not be used as proof that the current spec is provisioned.

Example:

```yaml
conditions:
  - type: EntitlementApproved
    status: "True"
    observedGeneration: 3
    lastTransitionTime: "2026-08-26T10:05:00Z"
    reason: ProcurementApproved
    message: "Entitlement approved through Cloud Commerce Procurement API"
```

### Event Handling Rules

Handlers apply event freshness before mutating conditions:

- Persist `lastMarketplaceEventID` and `lastMarketplaceEventTime` for the latest accepted Google Marketplace event.
- Ack duplicate `eventId` values without reapplying side effects.
- Ignore stale lifecycle events whose `updateTime` is older than `lastMarketplaceEventTime` unless the operation is an idempotent cleanup event.
- When event ordering is ambiguous, refresh the entitlement with `GetEntitlement(spec.entitlementName)` and derive conditions from the Procurement API source of truth before granting access.

| Current Conditions | Event / Trigger | Condition Updates | Actions |
|---|---|---|---|
| *(new)* | `ENTITLEMENT_CREATION_REQUESTED` | `NamespaceReady=True`, `MeteringReady=True` when corresponding actions succeed; `EntitlementApproved=False`, `EntitlementActive=False` | Create or refresh CRD, create namespace, store `usageReportingId`, set `createdAt`, and wait for verified sign-up; do not approve the entitlement during purchase handling |
| `NamespaceReady=True` | `SIGNUP_COMPLETED` | `SignupCompleted=True`, then `AccountApproved=True`, then `EntitlementApproved=True` after Procurement approvals succeed; `EntitlementActive=False` until Google confirms active state | Set `userEmail` and Marketplace identity, approve account signup, approve entitlement, then wait for `ENTITLEMENT_ACTIVE` or active `GetEntitlement` before provisioning access |
| `EntitlementApproved=True`, `EntitlementActive=True`, `SignupCompleted=True`, `AccountApproved=True`, `NamespaceReady=True`, `Suspended=False`, `PendingCancellation=False`, `Cancelled=False`, `Deleted=False` | *(reconciler)* | `AccessReady=True` | Create `RoleBinding/marketplace-service-admin`, set `activatedAt`; usage-based plans also require `MeteringReady=True` |
| `Deleted=False`, `Cancelled=False` | `ENTITLEMENT_PLAN_CHANGE_REQUESTED` | Record pending plan field | Approve plan change via Procurement API |
| `Deleted=False`, `Cancelled=False` | `ENTITLEMENT_PLAN_CHANGED` | Clear pending plan field | Update `spec.plan` to the effective plan from `GetEntitlement` |
| `Deleted=False`, `Cancelled=False` | Service Control check error or Procurement API inactive entitlement | `EntitlementActive=False`, `Suspended=True`, `AccessReady=False` | Set `suspendedAt`, delete RoleBinding if present, and block future sign-up provisioning until supported recovery signals confirm access can resume |
| `Suspended=True`, `Deleted=False`, `Cancelled=False` | `ENTITLEMENT_ACTIVE`, Procurement API active entitlement, or sustained successful Service Control checks | `EntitlementActive=True`, `Suspended=False` | Clear `suspendedAt`; reconciler restores access if sign-up and account approval are complete |
| `Deleted=False`, `Cancelled=False` | `ENTITLEMENT_PENDING_CANCELLATION` | `PendingCancellation=True` | Set `pendingCancellationAt`; keep existing access, but block new sign-up provisioning until cancellation is reverted or finalized |
| `PendingCancellation=True` | `ENTITLEMENT_CANCELLATION_REVERTED` | `PendingCancellation=False` | Clear `pendingCancellationAt`; reconciler restores or retains access when prerequisites are satisfied |
| `Deleted=False` | `ENTITLEMENT_CANCELLED` | `EntitlementActive=False`, `Cancelled=True`, `AccessReady=False`, `PendingCancellation=False` | Set `cancelledAt`, set `deletionScheduledAt` (now + 30 days), delete RoleBinding |
| `Cancelled=True` | *(30-day expiry via reconciler)* | `Deleted=True`, `EntitlementActive=False`, `MeteringReady=False`, `AccessReady=False`, `PendingCancellation=False`, `Cancelled=False`, `NamespaceReady=False` | Delete namespace, clean up resources, and remove private identity/billing data |
| Any | `ENTITLEMENT_DELETED` / `ACCOUNT_DELETED` | `Deleted=True`, `EntitlementActive=False`, `MeteringReady=False`, `AccessReady=False`, `PendingCancellation=False`, `Cancelled=False`, `NamespaceReady=False` | Immediate privacy cleanup: clear private identity/billing data and account labels, delete RoleBinding, request namespace deletion, retain minimal cleanup state until namespace termination, then delete the CRD |

### Condition Flow

```
ENTITLEMENT_CREATION_REQUESTED
  └─ NamespaceReady=True, MeteringReady=True when available, waiting for verified sign-up; entitlement is not approved yet
SIGNUP_COMPLETED
  └─ SignupCompleted=True, AccountApproved=True after ApproveAccount, EntitlementApproved=True after ApproveEntitlement
ENTITLEMENT_ACTIVE or active GetEntitlement refresh
  └─ EntitlementActive=True
Reconciler
  └─ AccessReady=True when entitlement approval, active entitlement, sign-up, account approval, namespace, and metering conditions allow access
Service Control check error or Procurement API inactive entitlement
  └─ Suspended=True, AccessReady=False
ENTITLEMENT_ACTIVE, Procurement API active entitlement, or sustained successful Service Control checks
  └─ EntitlementActive=True, Suspended=False, reconciler may restore AccessReady=True
ENTITLEMENT_PENDING_CANCELLATION
  └─ PendingCancellation=True, existing access retained
ENTITLEMENT_CANCELLATION_REVERTED
  └─ PendingCancellation=False
ENTITLEMENT_CANCELLED
  └─ EntitlementActive=False, Cancelled=True, AccessReady=False, cleanup scheduled
ENTITLEMENT_DELETED / ACCOUNT_DELETED
  └─ Deleted=True, AccessReady=False, private identity/billing data removed immediately, CRD retained until namespace termination
Cleanup expiry or namespace termination confirmed
  └─ NamespaceReady=False, CRD deleted
```

---

## Event Schemas

### Google Marketplace Events (Purchase Topic)

Events published by Google to the `mkt_purchase_events` topic. The schema follows [Google's Pub/Sub notification format](https://cloud.google.com/marketplace/docs/partners/integrated-saas/manage-entitlements).

```json
{
  "eventId": "evt-abc-123",
  "eventType": "ENTITLEMENT_CREATION_REQUESTED",
  "providerId": "gecko-platform",
  "entitlement": {
    "id": "E-67890",
    "updateTime": "2026-08-26T10:00:00Z"
  }
}
```

The Pub/Sub event is treated as a trigger, not the source of truth. Handlers call `GetEntitlement` before mutating state that depends on account ID, product, plan, consumer/project identity, or `usageReportingId`.

| Condition-Changing Event Type | Description |
|---|---|
| `ENTITLEMENT_CREATION_REQUESTED` | Customer purchased the product — create entitlement and namespace |
| `ENTITLEMENT_PLAN_CHANGE_REQUESTED` | Customer requested a plan change — approve with the pending plan name |
| `ENTITLEMENT_PLAN_CHANGED` | Plan change took effect — update the effective plan |
| `ENTITLEMENT_ACTIVE` | Entitlement became or returned to active; restores access after suspension |
| `ENTITLEMENT_PENDING_CANCELLATION` | Customer requested cancellation effective later — keep access until final cancellation |
| `ENTITLEMENT_CANCELLATION_REVERTED` | Customer reverted pending cancellation — return to active |
| `ENTITLEMENT_CANCELLED` | Cancellation finalized — revoke access and start 30-day cleanup grace period |
| `ENTITLEMENT_DELETED` | Entitlement deleted by Google — immediately clean up resources |
| `ACCOUNT_DELETED` | Account deleted by Google — immediately clean up account-associated resources |

| Acknowledged Event Type | Handling |
|---|---|
| `ACCOUNT_ACTIVE` | Ack and log; sign-up completion remains driven by the Gecko-owned sign-up event |
| `ENTITLEMENT_OFFER_ACCEPTED` | Ack and log; no local state change until an entitlement lifecycle event follows |
| `ENTITLEMENT_PLAN_CHANGE_CANCELLED` | Ack and log; keep the current effective plan |
| `ENTITLEMENT_CANCELLING` | Ack and log; wait for `ENTITLEMENT_PENDING_CANCELLATION` or `ENTITLEMENT_CANCELLED` |
| `ENTITLEMENT_RENEWED` | Ack and log; no local access change required |
| `ENTITLEMENT_OFFER_ENDED` | Ack and log; no local access change unless followed by a plan or cancellation event |

Google SaaS Marketplace may not emit an `ENTITLEMENT_SUSPENDED` Pub/Sub event for this product. The controller therefore does not depend on that event for access revocation. It drives `Suspended=True`, `AccessReady=False` from supported signals such as Procurement API polling or Service Control `services.check` errors (`SERVICE_NOT_ACTIVATED`, `BILLING_DISABLED`, `PROJECT_DELETED`). Recovery clears `Suspended=False` only after supported signals confirm the entitlement/consumer is usable again, such as Procurement API active state or sustained successful Service Control checks.

### Sign-Up Events (Sign-Up Topic)

Events published by the sign-up page to the `mkt_signup_events` topic. Schema defined by the Gecko team, implemented by the Partner Enablement Team. The sign-up page must publish this event only after validating Google's Marketplace frontend token and verifying the user's identity.

```json
{
  "eventType": "SIGNUP_COMPLETED",
  "accountId": "A-12345",
  "marketplaceUserIdentity": "accounts.google.com:1234567890",
  "email": "alice@example.com"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `eventType` | string | yes | Must be `"SIGNUP_COMPLETED"` |
| `accountId` | string | yes | Procurement account ID from the verified Marketplace token |
| `marketplaceUserIdentity` | string | yes | Obfuscated Marketplace user identity from the verified Marketplace token |
| `email` | string | yes | Verified user email (identity verified by sign-up page) |

---

## Pub/Sub Handler

The `PubSubHandler` processes messages from both topics. Each handler follows the same error classification pattern: permanent errors are acknowledged (preventing poison pill redelivery), transient errors are negatively acknowledged (Pub/Sub retries with backoff).

### Event Type Definitions

```go
type MarketplaceEvent struct {
    EventID     string                      `json:"eventId"`
    EventType   string                      `json:"eventType"`
    ProviderID  string                      `json:"providerId"`
    Account     MarketplaceAccount         `json:"account"`
    Entitlement MarketplaceEntitlementEvent `json:"entitlement"`
}

type MarketplaceAccount struct {
    ID string `json:"id"`
}

type MarketplaceEntitlementEvent struct {
    ID             string `json:"id"`
    UpdateTime     string `json:"updateTime"`
    Plan           string `json:"plan"`
    NewPendingPlan string `json:"newPendingPlan"`
}

type ProcurementEntitlement struct {
    Name             string `json:"name"`
    Account          string `json:"account"`
    Product          string `json:"product"`
    Plan             string `json:"plan"`
    NewPendingPlan   string `json:"newPendingPlan"`
    State            string `json:"state"`
    UsageReportingID string `json:"usageReportingId"`
    Consumers        []ProcurementConsumer `json:"consumers"`
}

type ProcurementConsumer struct {
    Project string `json:"project"` // format: "projects/<project-number>"
}

type SignupEvent struct {
    EventType               string `json:"eventType"`
    AccountID               string `json:"accountId"`
    MarketplaceUserIdentity string `json:"marketplaceUserIdentity"`
    Email                   string `json:"email"`
}

type ServiceControlSuspensionSignal struct {
    SignalKey        string    `json:"signalKey"`
    Namespace        string    `json:"namespace"`
    ProjectNumber    string    `json:"projectNumber"`
    UsageReportingID string    `json:"usageReportingId"`
    CheckErrorCode   string    `json:"checkErrorCode"`
    ObservedAt       time.Time `json:"observedAt"`
}
```

### HandleMarketplaceMessage

Routes by `eventType`:

```
Message received
  ├─ Deserialize MarketplaceEvent from JSON
  │   └─ Malformed → Ack (permanent), log ERROR
  ├─ Route by eventType:
  │   ├─ ENTITLEMENT_CREATION_REQUESTED → handleCreationRequested()
  │   ├─ ENTITLEMENT_PLAN_CHANGE_REQUESTED → handlePlanChangeRequested()
  │   ├─ ENTITLEMENT_PLAN_CHANGED → handlePlanChanged()
  │   ├─ ENTITLEMENT_ACTIVE → handleActive()
  │   ├─ Procurement inactive → handleSuspended()
  │   ├─ ENTITLEMENT_PENDING_CANCELLATION → handlePendingCancellation()
  │   ├─ ENTITLEMENT_CANCELLATION_REVERTED → handleCancellationReverted()
  │   ├─ ENTITLEMENT_CANCELLED → handleCancelled()
  │   ├─ ENTITLEMENT_DELETED / ACCOUNT_DELETED → handleDeleted()
  │   ├─ Known no-op events → Ack, log INFO
  │   └─ Unknown → Ack (permanent), log WARN
  └─ Error handling:
      ├─ Permanent (Invalid, Forbidden) → Ack, log ERROR, increment metric
      └─ Transient (network, API unavailable) → Nack (Pub/Sub retries)
```

#### handleCreationRequested

1. Normalize the event entitlement ID into `entitlementName=providers/<providerId>/entitlements/<entitlementId>` and compute the DNS-safe Kubernetes object name.
2. Call `procurementClient.GetEntitlement(ctx, entitlementName)` and treat the response as the source of truth for account, product, effective plan, pending plan, consumers, and `usageReportingId`. If the result is typed not-found and no local CRD exists, Ack without provisioning and emit an operator-visible event; if a local CRD exists, enter the deletion cleanup path.
3. Extract project number from exactly one Procurement API consumer project (parse `projects/<project-number>`); reject multiple consumers for launch because multiple orders are disabled.
4. Extract account ID from the Procurement API `account` field (parse after last `/`)
5. **Idempotency check**: `Get` entitlement by the DNS-safe Kubernetes object name derived from `entitlementName` or by `marketplace.gcp.io/entitlement-key`. If exists:
   - If `Deleted=True` or `status.cleanupNamespace` is set, Ack without creating namespaces or mutating provisioning status; the retained object is terminal cleanup state and must not be recreated by a late purchase event
   - Refresh `MeteringReady`, `usageReportingId`, effective plan, account, and project from the Procurement API response
   - If `status.usageReportingId` is empty, refresh it from `GetEntitlement`
   - If `NamespaceReady=False`, create or verify the namespace and set `NamespaceReady=True`, `status.namespace=<projectNumber>`
   - If `createdAt` is empty, set `createdAt=now`
   - Do not call `ApproveEntitlement` from the purchase idempotency path; entitlement approval happens only after verified sign-up and account approval
   - Ack only after required idempotent follow-up is complete or a permanent error is recorded
6. Create `MarketplaceEntitlement` CRD:
   - Name: deterministic DNS-safe object name derived from `entitlementName`
   - Labels: `marketplace.gcp.io/account: <accountID>`, `marketplace.gcp.io/project: <projectNumber>`
   - Spec: `providerId`, `entitlementId`, `entitlementName`, `procurementAccountId`, `projectNumber`, `plan`
7. Create `Namespace` named by project number. Handle `AlreadyExists` (idempotent).
8. Persist `status.usageReportingId` from the Procurement API response when present.
9. Update status: `NamespaceReady=True`, `MeteringReady=True` when `usageReportingId` is present, `EntitlementApproved=False`, `EntitlementActive=False`, `namespace=<projectNumber>`, `createdAt=now`. Purchase handling must not call `ApproveEntitlement`; it prepares local state and waits for verified sign-up.

Creation failures follow the error classification in [HandleMarketplaceMessage](#handlemarketplacemessage): transient failures cause `Nack` so Pub/Sub retries; permanent failures are acknowledged only after recording status and emitting an operator-visible event.

#### handlePlanChangeRequested

1. Normalize the event entitlement ID to the full entitlement resource name and find the CR by entitlement key label
2. **Guard**: Reject if `Deleted=True` or `Cancelled=True`
3. Validate `entitlement.newPendingPlan` is non-empty
4. Record the pending plan in `status.pendingPlan`; keep `spec.plan` as the effective plan until Google confirms the change
5. Call `procurementClient.ApprovePlanChange(ctx, spec.entitlementName, entitlement.newPendingPlan)`, sending `{ "pendingPlanName": "<newPendingPlan>" }`

#### handlePlanChanged

1. Normalize the event entitlement ID to the full entitlement resource name and find the CR by entitlement key label
2. Call `procurementClient.GetEntitlement(ctx, spec.entitlementName)` and use the returned effective `plan` as the source of truth. If the result is typed not-found, stop plan handling and enter the deletion cleanup path because the upstream entitlement no longer exists.
3. Update `spec.plan` to the effective plan from the Procurement API response
4. Clear `status.pendingPlan`
5. Refresh `usageReportingId` if the Procurement API response changed it

#### handleActive (Reactivation)

1. Normalize the event entitlement ID to the full entitlement resource name and find the CR by entitlement key label
2. **Guard**: Ignore only if `Deleted=True` or `Cancelled=True`
3. Call `procurementClient.GetEntitlement(ctx, spec.entitlementName)` immediately before changing local access state. If the result is typed not-found, enter the deletion cleanup path. If the upstream state is not active, leave `Suspended=True` or `EntitlementActive=False` and do not restore access.
4. Set `EntitlementActive=True`, `Suspended=False`, and clear `suspendedAt` only after Procurement confirms the entitlement is active.
5. If `SignupCompleted=True` and `AccountApproved=True`, let the reconciler verify Procurement state again and restore `AccessReady=True`

#### handleSuspended

1. Find the CR from the Procurement polling result, Service Control check error signal, or normalized entitlement resource name when available
2. **Guard**: Ignore only if `Deleted=True` or `Cancelled=True`
3. Set `EntitlementActive=False`, `Suspended=True`, `AccessReady=False`, `suspendedAt=now`
4. Delete `RoleBinding/marketplace-service-admin` in the entitlement's namespace (revoke access). Handle `NotFound` (idempotent).

#### handleServiceControlSuspensionSignal

Service Control suspension signals use a non-Marketplace dispatch path that decodes `ServiceControlSuspensionSignal` and invokes `handleServiceControlSuspensionSignal` directly; they must not be decoded by `HandleMarketplaceMessage` or routed through the unknown Marketplace event path. The dispatch mechanism may be Pub/Sub, a controller work queue, or another internal mechanism selected during implementation, but it must deliver a typed signal with a stable `signalKey`, `observedAt`, `checkErrorCode`, and at least one entitlement lookup key such as `usageReportingId`, namespace, or project number.

The controller looks up `MarketplaceEntitlement` by `status.usageReportingId` first, then by `marketplace.gcp.io/project=<projectNumber>` as a fallback. If no entitlement is found, retry with bounded backoff because metering and entitlement creation can race; after retry exhaustion, Ack and emit an operator-visible event. Before calling `handleSuspended`, ignore duplicate `signalKey` values and signals whose `observedAt` is older than or equal to `status.lastSuspensionSignalObservedAt`. Persist `lastSuspensionSignalKey` and `lastSuspensionSignalObservedAt` only after suspension handling succeeds so stale signals cannot re-suspend access after a later active recovery.

#### reconcileSuspensionRecovery

1. Periodically refresh active and suspended entitlements with `GetEntitlement(spec.entitlementName)` and, for usage-based plans, Service Control `services.check`.
2. If `GetEntitlement` returns typed not-found, enter the deletion cleanup path and keep `AccessReady=False`.
3. Keep `Suspended=True` while Procurement reports a non-active entitlement or Service Control returns `SERVICE_NOT_ACTIVATED`, `BILLING_DISABLED`, or `PROJECT_DELETED`.
4. Clear `Suspended=False` and set `EntitlementActive=True` only after supported signals confirm the entitlement/consumer is usable again.
5. Let the reconciler restore `AccessReady=True` when sign-up, account approval, namespace, and metering conditions are satisfied.

#### handlePendingCancellation

1. Normalize the event entitlement ID to the full entitlement resource name and find the CR by entitlement key label
2. **Guard**: Ignore only if `Deleted=True` or `Cancelled=True`
3. Set `PendingCancellation=True`, `pendingCancellationAt=now`
4. Keep an existing `RoleBinding/marketplace-service-admin` in place; Marketplace has not finalized the cancellation
5. If sign-up has not completed yet, do not create access while the entitlement is pending cancellation

#### handleCancellationReverted

1. Normalize the event entitlement ID to the full entitlement resource name and find the CR by entitlement key label
2. **Guard**: Ignore if `Deleted=True` or `Cancelled=True`
3. **Guard**: Only process if `PendingCancellation=True`
4. Clear `pendingCancellationAt`
5. Set `PendingCancellation=False`
6. Refresh entitlement with `GetEntitlement(spec.entitlementName)`; set `EntitlementActive=True` only if the upstream entitlement is active
7. If `SignupCompleted=True` and `AccountApproved=True`, let the reconciler verify or restore `AccessReady=True`

#### handleCancelled

1. Normalize the event entitlement ID to the full entitlement resource name and find the CR by entitlement key label
2. **Guard**: Reject if `Deleted=True`
3. **Guard**: If `Cancelled=True`, return without changing `cancelledAt` or `deletionScheduledAt` so duplicate cancellation events preserve the original cleanup deadline.
4. Set `EntitlementActive=False`, `Cancelled=True`, `AccessReady=False`, `PendingCancellation=False`, `cancelledAt=now`, `deletionScheduledAt=now+30days`, clear `pendingCancellationAt`
5. Delete `RoleBinding/marketplace-service-admin` (revoke access). Handle `NotFound`.

#### handleDeleted

1. Normalize the event entitlement ID to the full entitlement resource name and find the CR by entitlement key label, or find by account label for `ACCOUNT_DELETED`. If `NotFound` → Ack (already cleaned up).
2. Immediately copy `status.namespace` to `status.cleanupNamespace`, clear `status.namespace`, clear private identity/billing fields (`userEmail`, `marketplaceUserIdentity`, `usageReportingId`, `deletionScheduledAt`, `lastMarketplaceEventID`, `lastMarketplaceEventTime`, `lastSuspensionSignalKey`, `lastSuspensionSignalObservedAt`), clear account/entitlement identifiers from spec, and remove marketplace account/project/entitlement labels.
3. Set `Deleted=True`, `EntitlementActive=False`, `MeteringReady=False`, `AccessReady=False`, `PendingCancellation=False`, `Cancelled=False`.
4. Delete `RoleBinding/marketplace-service-admin`. Handle `NotFound`.
5. Delete the namespace. Handle `NotFound`.
6. Retain only `status.cleanupNamespace` and non-identifying condition timestamps needed to observe namespace termination, requeue until namespace termination is observed complete, then set `NamespaceReady=False`, clear `cleanupNamespace`, and delete the `MarketplaceEntitlement` CRD.

### HandleSignupMessage

```
Message received
  ├─ Deserialize SignupEvent from JSON
  │   └─ Malformed → Ack (permanent), log ERROR
  ├─ Route by eventType:
  │   ├─ SIGNUP_COMPLETED → handleSignupCompleted()
  │   └─ Unknown → Ack, log WARN
  └─ Error handling (same classification as marketplace handler)
```

#### handleSignupCompleted

1. Validate `accountId`, `marketplaceUserIdentity`, and `email` are non-empty
2. Find entitlement by label `marketplace.gcp.io/account=<accountId>`. If not found, retry with bounded backoff because `mkt_purchase_events` and `mkt_signup_events` are separate Pub/Sub topics and sign-up can arrive before the purchase handler creates the CRD and account label. Ack as a permanent unknown account only after the configured retry window or dead-letter policy is exhausted. This assumes multiple orders are disabled for launch; if multiple orders are enabled, the event schema must include entitlement/order identity and this lookup must use that identity instead.
3. Build the full Procurement account resource name from `spec.providerId` and the event account ID, for example `providers/<providerId>/accounts/<accountId>`; do not use a user-supplied full resource name from the sign-up event.
4. **Idempotency and conflict checks**:
   - If `userEmail` matches `email` and `marketplaceUserIdentity` matches the event value, but `AccountApproved=False` → retry `ApproveAccount`, then continue to `ApproveEntitlement` before Ack if account approval succeeds
   - If `userEmail` matches `email`, `marketplaceUserIdentity` matches the event value, `AccountApproved=True`, and `EntitlementApproved=False` → retry `ApproveEntitlement` before Ack
   - If `userEmail` matches `email`, `marketplaceUserIdentity` matches the event value, `AccountApproved=True`, and `EntitlementApproved=True` → Ack (duplicate, already processed)
   - If `userEmail` is set and differs → Ack (permanent error — email conflict), log ERROR
   - If `marketplaceUserIdentity` is set and differs → Ack (permanent error — Marketplace user identity conflict), log ERROR
   - If `Cancelled=True` or `Deleted=True` → Ack, log WARN, and do not approve account or provision access
   - If `AccessReady=True` → Ack (already provisioned)
5. Set `status.userEmail=email` and `status.marketplaceUserIdentity=marketplaceUserIdentity`; set `SignupCompleted=True`; keep `AccessReady=False` until account and entitlement approval succeed
6. Call `procurementClient.ApproveAccount(ctx, accountName, "signup")`, sending `{ "approvalName": "signup" }`.
7. Update status: `AccountApproved=True`
8. Call `procurementClient.ApproveEntitlement(ctx, spec.entitlementName)` only after account approval succeeds, then set `EntitlementApproved=True`.
9. Keep `EntitlementActive=False` and `AccessReady=False` until Google sends `ENTITLEMENT_ACTIVE` or a fresh `GetEntitlement` confirms the entitlement is active. If `Suspended=True` or `PendingCancellation=True`, the reconciler will not provision access until Google sends a lifecycle event that returns the entitlement to active.

Account and entitlement approval failures follow the same error classification as marketplace approvals: transient failures cause `Nack` so Pub/Sub retries; permanent failures are acknowledged only after recording status and emitting an operator-visible event. Duplicate sign-up events retry account approval when `AccountApproved=False`, then continue to entitlement approval if account approval succeeds; they retry entitlement approval when `AccountApproved=True` but `EntitlementApproved=False`.

---

## Reconciler

The reconciler watches `MarketplaceEntitlement` resources and acts when condition changes require Kubernetes resource creation or cleanup.

### Reconcile Logic

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    entitlement := &privatev1.MarketplaceEntitlement{}
    if err := r.client.Get(ctx, req.NamespacedName, entitlement); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    switch {
    case shouldProvisionAccess(entitlement):
        return r.reconcileProvisioning(ctx, entitlement)
    case isConditionTrue(entitlement, "Cancelled") || isConditionTrue(entitlement, "Deleted") || entitlement.Status.CleanupNamespace != "":
        return r.reconcileCancellation(ctx, entitlement)
    default:
        return ctrl.Result{}, nil
    }
}
```

### reconcileProvisioning

1. If `EntitlementApproved=False` or `EntitlementActive=False` → return (wait for Google entitlement approval and active upstream entitlement)
2. If `SignupCompleted=False` or `userEmail` is empty → return (wait for sign-up)
3. If `AccountApproved=False` → return (wait for Procurement account approval; never create access before Google account approval succeeds)
4. If `Suspended=True`, `PendingCancellation=True`, `Cancelled=True`, or `Deleted=True` → return (access is blocked by Marketplace lifecycle)
5. If the plan is usage-based and `MeteringReady=False` → return (wait for `usageReportingId` so access cannot generate unattributed billable usage)
6. Verify the upstream entitlement is still active with `GetEntitlement(spec.entitlementName)` before creating, retaining, or treating access as ready. If the result is typed not-found, enter the deletion cleanup path and keep `AccessReady=False`.
7. If `AccessReady=True`, `observedGeneration == metadata.generation`, the upstream entitlement is active, and the observed RoleBinding matches the expected subject, roleRef, namespace, and API group → set `activatedAt` if empty and return
8. Verify `RoleBinding/marketplace-service-admin` is absent or matches the expected subject, roleRef, namespace, and API group before creating or treating access as ready
9. If `namespace` is empty → return (wait for namespace creation)
10. Create `RoleBinding/marketplace-service-admin` in the namespace:
   ```yaml
   apiVersion: gcp.managed.openshift.io/v1
   kind: RoleBinding
   metadata:
     name: marketplace-service-admin
     namespace: "<projectNumber>"
   spec:
     subject: "<userEmail>"
     roleRef:
       kind: PlatformRole
       name: service-admin
       apiGroup: gcp.managed.openshift.io
   ```
11. Handle `AlreadyExists` only if the existing RoleBinding matches the expected subject, roleRef, namespace, and API group; conflicting RoleBindings are permanent errors that leave `AccessReady=False`
12. Update status: `AccessReady=True`, `activatedAt=now` only after observing the expected RoleBinding state

### reconcileCancellation

1. If `Deleted=True` or `cleanupNamespace` is set, skip the grace-period wait and continue observing cleanup.
2. If `deletionScheduledAt` is nil or in the future → requeue after remaining duration
3. If `deletionScheduledAt` is in the past or deletion cleanup is already in progress:
    - Delete `RoleBinding/marketplace-service-admin`. Handle `NotFound`.
    - Delete the namespace. Handle `NotFound`.
    - Immediately copy `status.namespace` to `status.cleanupNamespace`, clear `status.namespace`, clear private identity/billing fields (`userEmail`, `marketplaceUserIdentity`, `usageReportingId`, `deletionScheduledAt`, `lastMarketplaceEventID`, `lastMarketplaceEventTime`, `lastSuspensionSignalKey`, `lastSuspensionSignalObservedAt`), clear account/entitlement identifiers from spec, and remove marketplace account/project/entitlement labels.
    - Set `Deleted=True`, `EntitlementActive=False`, `MeteringReady=False`, `PendingCancellation=False`, `AccessReady=False`, `Cancelled=False`.
    - Requeue until namespace termination is observed complete; namespace deletion is asynchronous and can be blocked by finalizers.
    - After namespace termination is confirmed, set `NamespaceReady=False`, clear `cleanupNamespace`, and delete the `MarketplaceEntitlement` CRD so account binding does not remain in spec or labels

---

## Procurement API Client

The `ProcurementClient` wraps the Google Cloud Commerce Procurement API with Workload Identity authentication.

```go
type ProcurementClient struct {
    baseURL        string // "https://cloudcommerceprocurement.googleapis.com"
    httpClient     *http.Client // OAuth2-authenticated via WIF
    perCallTimeout time.Duration
}
```

`baseURL` is host-only; method paths include the `/v1` API prefix. URL construction must produce exactly one `/v1` segment. Each public method wraps the caller context with `context.WithTimeout(ctx, perCallTimeout)` before issuing the HTTP request. Stalled Procurement API calls return transient timeout errors so Pub/Sub can retry instead of blocking a handler indefinitely.

### Methods

| Method | API Endpoint | Purpose |
|---|---|---|
| `ApproveEntitlement(ctx, entitlementName)` | `POST /v1/{name}:approve` | Approve entitlement after verified sign-up and account approval |
| `ApproveAccount(ctx, accountName, approvalName)` | `POST /v1/{name}:approve` | Approve account after sign-up with body `{ "approvalName": "signup" }` |
| `ApprovePlanChange(ctx, entitlementName, pendingPlanName)` | `POST /v1/{name}:approvePlanChange` | Approve plan change with body `{ "pendingPlanName": "<pendingPlanName>" }` |
| `GetEntitlement(ctx, entitlementName)` | `GET /v1/{name}` | Query entitlement state for reconciliation |

### Authentication

The client uses Workload Identity credentials and an OAuth2-authenticated HTTP client. Construct the underlying transport with bounded timeouts before wrapping it with OAuth2, for example dial timeout, TLS handshake timeout, idle connection timeout, and response-header timeout. The service account must be linked to the product's Producer Portal Billing integration as an account allowed to call the Partner Procurement API. It also requires the `roles/commerceproducer.admin` IAM role on the Procurement API project when that role is part of the product deployment model.

Verify the final identity can call authenticated `GetEntitlement`, `ApproveEntitlement`, and `ApproveAccount` against the product provider before enabling the controller in production.

### Error Handling

- 404 Not Found → log warning and return a typed `ErrEntitlementNotFound`; callers must stop provisioning and enter the deletion cleanup path because the upstream entitlement may have been cleaned up by Google
- 409 Conflict → log warning, return nil (already approved)
- 4xx → permanent error (do not retry)
- 5xx / network → transient error (caller retries)

---

## Sign-Up Page Integration Contract

The sign-up page is owned and built by the Partner Enablement Team at Red Hat. The Gecko team provides the following integration contract.

### Pub/Sub Contract

| Item | Value |
|---|---|
| Topic | `mkt_signup_events` (Gecko-owned) |
| Project | Provided by Gecko team per environment |
| Authentication | WIF-authenticated service account with `roles/pubsub.publisher` on the topic |
| Message format | JSON (`SignupEvent` schema, see [Sign-Up Events](#sign-up-events-sign-up-topic)) |

### Marketplace Token and Identity Verification Requirement

Google Marketplace redirects the customer to the sign-up page with an `x-gcp-marketplace-token` POST parameter. The sign-up page **must** verify this Marketplace token before publishing the `SIGNUP_COMPLETED` event. The procurement account ID and obfuscated Marketplace user identity in the event must come from the verified token, not from a user-editable query parameter or form field.

The sign-up page **must** also verify the user's identity before publishing the event. The `email` field must contain a verified email address — not a free-text form input. Acceptable verification methods include:

- Google OAuth consent screen with `email` scope → verified email from ID token
- Identity-Aware Proxy (IAP) → verified email from `X-Goog-Authenticated-User-Email` header
- ESPv2 with Cloud Endpoints → verified email from `X-Endpoint-API-UserInfo` header

The Gecko team does not prescribe the implementation — the Partner Enablement Team chooses the method that fits their architecture.

### Sign-Up URL Format

Google Marketplace redirects the customer to the sign-up URL after purchase and posts `x-gcp-marketplace-token` to that URL:

```
https://signup.gecko-platform.example.com/
```

The sign-up page verifies the token, extracts the procurement account ID and obfuscated Marketplace user identity from the verified token payload, and uses that account ID to associate the sign-up with the correct entitlement. Multiple orders are disabled for launch; if they are enabled later, this contract must include entitlement/order identity from the token and the controller must use that identity instead of account-only lookup.

### Configuration Provided by Gecko Team

| Item | Description |
|---|---|
| Pub/Sub project ID | GCP project where the `mkt_signup_events` topic resides |
| Pub/Sub topic name | `mkt_signup_events` |
| WIF service account email | `gecko-signup-publisher@<project>.iam.gserviceaccount.com` |
| Sign-up URL | `https://signup.gecko-platform.example.com/` |
| Pub/Sub emulator instructions | For local development (see [Local Development](#local-development)) |

### Testing with Pub/Sub Emulator

The Partner Enablement Team can test against the Pub/Sub emulator:

```bash
# Start emulator
docker run -p 8085:8085 gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085

# Set environment (auto-detected by pubsub v2 client library)
export PUBSUB_EMULATOR_HOST=localhost:8085

# Create topic via REST API
curl -s -X PUT "http://localhost:8085/v1/projects/gecko-local/topics/mkt_signup_events"
```

---

## Billing Handoff

### Architecture

This plan defines only the Marketplace entitlement state needed by future billing work:

- The marketplace controller retrieves `usageReportingId` from the Procurement API and stores it privately on `MarketplaceEntitlement.status.usageReportingId`.
- `MeteringReady=True` means the entitlement has the attribution key a later billing implementation needs.
- Usage-based plans must not grant access while `MeteringReady=False`, so billable usage cannot start before attribution context exists.
- ESPv2 request metrics remain operational telemetry unless a separate billing DD/IP explicitly maps them to Marketplace attribution.

The billing reporter, usage aggregation, metric dimensions, pricing behavior, Service Control `services.check`/`services.report` calls, retry behavior, IAM scope, and failover semantics are out of scope for this plan and will be defined in a separate DD/IP.

### Billing Attribution Context

The GCP project number remains the namespace name and internal correlation key, but it is not the Service Control consumer ID for Marketplace usage reporting. The future billing implementation must use the entitlement `usageReportingId` for Marketplace attribution if it reports usage through Service Control.

### Suspension Signal Boundary

If the separate billing implementation emits suspension signals from Service Control check results, those signals must use the non-Marketplace `ServiceControlSuspensionSignal` contract defined in this plan so entitlement access is revoked safely and stale signals cannot override later recovery.

---

## Controller Deployment

### Command Registration

The marketplace controller is registered as a subcommand of `gecko-controllers`:

```go
rootCmd.AddCommand(cmdmarketplace.NewCommand(rf))
```

### Startup Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--procurement-url` | `PROCUREMENT_URL` | Production Procurement API host | Host-only Procurement API base URL; method paths add `/v1` |
| `--provider-id` | `PROVIDER_ID` | *(required)* | Google Marketplace provider ID used to construct Procurement API resource names |
| `--pubsub-project` | `PUBSUB_PROJECT` | `gecko-local` | GCP project for Pub/Sub |
| `--purchase-sub` | `MKT_PURCHASE_SUB` | `mkt_purchase_sub` | Google purchase events subscription |
| `--signup-sub` | `MKT_SIGNUP_SUB` | `mkt_signup_sub` | Sign-up events subscription |

The `PUBSUB_EMULATOR_HOST` environment variable is supported for local development.

### Helm Chart Values

```yaml
marketplaceController:
  enabled: true
  replicas: 1  # Set to 0 in standby regions
  image:
    repository: <artifact-registry>/gecko-controllers
    tag: latest
  procurement:
    url: "https://cloudcommerceprocurement.googleapis.com"
    providerId: "<marketplace-provider-id>"
  pubsub:
    project: "<gcp-project>"
    purchaseSubscription: "mkt_purchase_sub"
    signupSubscription: "mkt_signup_sub"
  serviceAccount:
    name: gecko-marketplace
    annotations:
      iam.gke.io/gcp-service-account: gecko-marketplace@<project>.iam.gserviceaccount.com
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi
```

### RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: marketplace-controller
rules:
  - apiGroups: ["gcp.managed.openshift.io"]
    resources: ["marketplaceentitlements"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["gcp.managed.openshift.io"]
    resources: ["marketplaceentitlements/status"]
    verbs: ["get", "update", "patch"]
  - apiGroups: ["gcp.managed.openshift.io"]
    resources: ["rolebindings"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
```

### GCP Service Accounts

```bash
# 1. Marketplace controller — Procurement API access + Pub/Sub subscriber
gcloud iam service-accounts create gecko-marketplace \
  --project=$PROJECT_ID \
  --display-name="Gecko Marketplace Controller"

# Grant Procurement API access
gcloud projects add-iam-policy-binding $PROCUREMENT_PROJECT \
  --member="serviceAccount:gecko-marketplace@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/commerceproducer.admin"

# Link this service account in the product's Producer Portal Billing integration
# before enabling production traffic; IAM alone does not authorize Partner
# Procurement API calls for the product.

# Grant Pub/Sub subscriber access (Google purchase topic)
gcloud pubsub subscriptions add-iam-policy-binding $PURCHASE_SUB \
  --member="serviceAccount:gecko-marketplace@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.subscriber"

# Grant Pub/Sub subscriber access (sign-up topic)
gcloud pubsub subscriptions add-iam-policy-binding $SIGNUP_SUB \
  --member="serviceAccount:gecko-marketplace@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.subscriber"

# 2. Sign-up page — Pub/Sub publisher (provided to Partner Enablement Team)
gcloud iam service-accounts create gecko-signup-publisher \
  --project=$PROJECT_ID \
  --display-name="Gecko Sign-Up Page Publisher"

gcloud pubsub topics add-iam-policy-binding mkt_signup_events \
  --member="serviceAccount:gecko-signup-publisher@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/pubsub.publisher"

# 3. Billing reporter IAM is out of scope for this plan and will be defined
# by the separate billing/metering DD/IP.
```

Before production enablement, verify the controller service account can call `GetEntitlement`, `ApproveEntitlement`, and `ApproveAccount` for the configured `providers/<providerId>` resource. Verification must use the same Workload Identity principal and Producer Portal product linkage as the deployed controller.

### Disaster Recovery

The marketplace controller is deployed to all regions but runs with `replicas: 0` in standby regions.

**Normal operation**: One region has `replicas: 1`. All Pub/Sub subscriptions pull from this single consumer.

**Failover procedure**:
1. Detect primary region failure (automated alert or manual observation)
2. Scale up in the DR region: `helm upgrade --set marketplaceController.replicas=1` in the DR region
3. Scale down in the failed region (when recovered): `helm upgrade --set marketplaceController.replicas=0`
4. Pub/Sub subscriptions automatically resume delivery to the new consumer — unacknowledged messages are redelivered

**Recovery time**: Sub-minute (Helm upgrade + pod scheduling). Pub/Sub retains unacknowledged messages for the configured retention period (default 7 days).

---

## Metrics and Alerting

### Prometheus Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `gecko_marketplace_entitlements_total` | Gauge | `condition`, `status` | Current count of entitlements by condition status |
| `gecko_marketplace_provisioning_duration_seconds` | Histogram | | Time from all provisioning gates becoming eligible to `AccessReady=True`; excludes suspension, pending cancellation, and other Marketplace lifecycle wait time |
| `gecko_marketplace_events_total` | Counter | `topic`, `event_type`, `result` | Pub/Sub messages processed (`ack`, `nack`, `error`) |
| `gecko_marketplace_procurement_requests_total` | Counter | `operation`, `status` | Procurement API calls by operation and HTTP status |
| `gecko_marketplace_procurement_duration_seconds` | Histogram | `operation` | Procurement API call latency |
| `gecko_marketplace_signup_conflicts_total` | Counter | | Sign-up events rejected due to email conflict |
| `gecko_marketplace_namespace_cleanups_total` | Counter | `result` | Namespace deletions after cancellation grace period |
| `gecko_marketplace_metering_reports_total` | Counter | `status` | Service Control Report API calls |

### ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: marketplace-controller
  namespace: gecko-system
spec:
  selector:
    matchLabels:
      app: marketplace-controller
  endpoints:
    - port: metrics
      interval: 30s
```

### Alerts

| Alert | Condition | Severity |
|---|---|---|
| MarketplaceProvisioningSlow | `gecko_marketplace_provisioning_duration_seconds > 300` (5 min) | Warning |
| MarketplaceProcurementErrors | `rate(gecko_marketplace_procurement_requests_total{status=~"5.."}[5m]) > 0` for 10m | Warning |
| MarketplaceDeadLetterMessages | Dead-letter topic has messages (`pubsub.googleapis.com/subscription/dead_letter_message_count > 0`) | Critical |
| MarketplaceControllerDown | `up{job="marketplace-controller"} == 0` for 5m | Critical |
| MarketplaceMeteringFailures | `rate(gecko_marketplace_metering_reports_total{status!="200"}[5m]) > 0` for 10m | Warning |

---

## Local Development

### Pub/Sub Emulator

Local development uses the Google Cloud Pub/Sub emulator. The v2 client library auto-detects `PUBSUB_EMULATOR_HOST`.

```bash
# Start emulator
docker run -d --name pubsub-emulator -p 8085:8085 \
  gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators \
  gcloud beta emulators pubsub start --host-port=0.0.0.0:8085

export PUBSUB_EMULATOR_HOST=localhost:8085

# Create topics and subscriptions
curl -s -X PUT "http://localhost:8085/v1/projects/gecko-local/topics/mkt_purchase_events"
curl -s -X PUT "http://localhost:8085/v1/projects/gecko-local/topics/mkt_signup_events"
curl -s -X PUT "http://localhost:8085/v1/projects/gecko-local/subscriptions/mkt_purchase_sub" \
  -H "Content-Type: application/json" \
  -d '{"topic":"projects/gecko-local/topics/mkt_purchase_events"}'
curl -s -X PUT "http://localhost:8085/v1/projects/gecko-local/subscriptions/mkt_signup_sub" \
  -H "Content-Type: application/json" \
  -d '{"topic":"projects/gecko-local/topics/mkt_signup_events"}'
```

### Mock Procurement API

For local development, the following `http-echo` container only validates that the controller can reach a configured Procurement URL. It does not return schema-compatible account, plan, consumer, state, or `usageReportingId` fields and cannot exercise purchase, sign-up, or reconciliation flows:

```bash
docker run -d --name mock-procurement -p 8080:5678 \
  hashicorp/http-echo -text='{"status":"approved"}'
```

Set `--procurement-url=http://localhost:8080` when running the controller.

Use handler unit tests with a mock `ProcurementClient`, or an endpoint-specific fixture server, for lifecycle testing. Fixture responses for `GetEntitlement` must include `account`, `product`, `plan`, `state`, `consumers[].project`, and `usageReportingId`.

---

## File Structure

```text
controllers/
  marketplace/
    marketplace_controller.go           # Reconciler: provisions RoleBindings, handles cancellation cleanup
    marketplace_controller_test.go      # Reconciler unit tests
    pubsub_handler.go                   # PubSubHandler: handles all Pub/Sub message types
    pubsub_handler_test.go             # Handler unit tests (with mock Procurement client)
    procurement.go                      # ProcurementClient: real Procurement API wrapper with WIF auth
    procurement_test.go                # Procurement client unit tests (HTTP test server)
    types.go                            # Event schemas, state constants, label keys
  cmd/marketplace/
    cmd.go                              # Cobra subcommand, Pub/Sub + controller wiring
platform-api/
  api/private/v1/
    marketplaceentitlement_types.go     # CRD type definition with kubebuilder markers
  api/public/v1/
    zz_generated.marketplaceentitlement_types.go  # Generated public type (no UserEmail)
    zz_generated.conversion.go                    # Generated conversion (strips private fields)
    zz_generated.schemas.go                       # Generated OpenAPI schema
  cmd/platform-api-server/
    main.go                             # API server entrypoint; billing reporter tracked in separate DD/IP
helm/charts/
  marketplace-controller/
    Chart.yaml
    values.yaml                         # replicas, image, pubsub, procurement config
    templates/
      deployment.yaml
      serviceaccount.yaml
      clusterrole.yaml
      clusterrolebinding.yaml
      servicemonitor.yaml
signup-page/                            # Owned by Partner Enablement Team
  main.go                               # HTTP server with sign-up form
  go.mod                                # Standalone module (no gecko dependency)
```

---

## Verification Checklist

### Entitlement Lifecycle

- [ ] Purchase event → CRD created, namespace created, `NamespaceReady=True`
- [ ] Purchase handling does not call `ApproveEntitlement`; it prepares local state and waits for verified sign-up
- [ ] `handleSignupCompleted` calls `ApproveAccount` before `ApproveEntitlement`; transient approval failures are retried
- [ ] Creation handler keeps `EntitlementActive=False` until `ENTITLEMENT_ACTIVE` or a fresh `GetEntitlement` confirms active state
- [ ] Creation handler derives account, project, plan, and `usageReportingId` from `GetEntitlement`, not only from the Pub/Sub event payload
- [ ] Creation handler treats `GetEntitlement` typed not-found as deletion cleanup and does not provision access
- [ ] Duplicate purchase event during deferred deletion is Ack'd without recreating namespace or mutating provisioning status
- [ ] Event entitlement IDs are normalized with configured `providerId`; Kubernetes object names are deterministic and DNS-safe
- [ ] Generated `MarketplaceEntitlement` CRD has `scope: Cluster` and `subresources.status`
- [ ] Access provisioning requires `EntitlementApproved=True`, `EntitlementActive=True`, and `AccountApproved=True`
- [ ] Access provisioning verifies current Procurement state before creating or retaining `AccessReady=True`
- [ ] Duplicate purchase event → idempotent (CRD already exists, Ack)
- [ ] Sign-up event → `userEmail` set, `SignupCompleted=True`, `AccountApproved=True` after account approval, then `EntitlementApproved=True` after entitlement approval
- [ ] Sign-up page verifies `x-gcp-marketplace-token` before publishing `SIGNUP_COMPLETED`
- [ ] Procurement API called on sign-up with `approvalName=signup` before entitlement approval
- [ ] Sign-up received before purchase handler creates the CRD/account label is retried or dead-lettered by bounded policy rather than lost
- [ ] Sign-up received during suspension or pending cancellation is persisted, but access remains blocked until Marketplace returns the entitlement to active
- [ ] Duplicate sign-up (same email) → idempotent (Ack, no state change)
- [ ] Conflicting sign-up (different email) → rejected (Ack, logged ERROR)
- [ ] Sign-up for unknown account → retried with bounded policy, then Ack'd or dead-lettered only after retry exhaustion proves the account is genuinely unknown
- [ ] Reconciler → RoleBinding created, `AccessReady=True`, `activatedAt` set
- [ ] Plan change request event → pending plan approved with `pendingPlanName`
- [ ] Plan changed event → effective plan updated
- [ ] Plan changed handler treats `GetEntitlement` typed not-found as deletion cleanup
- [ ] Suspension signal → RoleBinding deleted, `Suspended=True`, `AccessReady=False`
- [ ] Service Control `SERVICE_NOT_ACTIVATED`, `BILLING_DISABLED`, or `PROJECT_DELETED` check errors trigger reconciliation to `Suspended=True`, `AccessReady=False` when no SaaS suspension event is available
- [ ] Service Control check error signal includes enough context to look up the entitlement and delete the affected `RoleBinding/marketplace-service-admin`
- [ ] Service Control suspension signals use a non-Marketplace dispatch path and cannot be swallowed by unknown Marketplace event handling
- [ ] Duplicate suspension signals and signals older than the persisted observation are ignored
- [ ] An older suspension signal arriving after active recovery does not re-suspend access
- [ ] Suspension signal freshness metadata advances only after successful suspension handling
- [ ] Suspension before sign-up blocks later access provisioning until supported recovery signals confirm the entitlement/consumer is usable again
- [ ] Reactivation event refreshes Procurement state before clearing `Suspended`; non-active or not-found upstream entitlement does not restore access
- [ ] Pending cancellation event → `PendingCancellation=True`, existing access retained
- [ ] Cancellation reverted event → `PendingCancellation=False`, access retained/restored when appropriate
- [ ] Final cancellation event → `Cancelled=True`, `AccessReady=False`, access revoked, `deletionScheduledAt` set
- [ ] Duplicate final cancellation event preserves original `cancelledAt` and `deletionScheduledAt`
- [ ] 30-day grace period expiry from `Cancelled=True` → namespace deleted, `Deleted=True`
- [ ] Immediate entitlement/account deletion event → namespace deletion requested, `Deleted=True`, and CRD retained until namespace termination completes
- [ ] Entitlement/account deletion clears private identity/billing fields, removes account/project labels, and deletes the CRD after namespace termination so account binding is not retained in spec or labels
- [ ] Entitlement/account deletion clears account/entitlement identifiers from spec before retaining minimal cleanup state
- [ ] Known no-op Marketplace events → Ack'd and logged without state changes
- [ ] Every condition update sets `observedGeneration=metadata.generation`
- [ ] Reconciler treats spec-dependent readiness conditions with stale `observedGeneration` as not ready
- [ ] Reconciler treats absent, duplicate, and `Unknown` conditions as not ready; repeated events update conditions by type
- [ ] `lastTransitionTime` changes only when condition `status` changes
- [ ] Duplicate and stale Marketplace events are ignored using `eventId` and `updateTime`, with Procurement API refresh before access-granting decisions when ordering is ambiguous

### Authorization Integration

- [ ] Marketplace-created RoleBinding grants `service-admin` permissions via Cedar
- [ ] Cross-namespace list returns resources only from authorized namespaces
- [ ] RoleBinding replicates to all regions once the planned cross-region replication prerequisite lands
- [ ] Suspended entitlement → user gets 403 on all API calls
- [ ] Reactivated entitlement → user regains access

### Billing Handoff

- [ ] ESPv2 request-level telemetry is treated as operational telemetry unless a separate billing DD/IP maps it to Marketplace attribution
- [ ] Marketplace controller retrieves and persists `usageReportingId`
- [ ] `MeteringReady=True` is set only when `usageReportingId` exists
- [ ] Public API output does not expose `usageReportingId`
- [ ] Cleanup clears `usageReportingId` with other private billing attribution state
- [ ] Billing reporter, usage aggregation, metric dimensions, Service Control reporting, pricing behavior, IAM scope, and failover behavior are tracked in a separate DD/IP
- [ ] `gecko_marketplace_provisioning_duration_seconds` starts only after all provisioning gates are eligible and excludes suspension or pending-cancellation wait time

### Procurement Integration

- [ ] Controller service account is linked in the product's Producer Portal Billing integration for Partner Procurement API calls
- [ ] Controller service account has required IAM for the deployment model, including Procurement API access
- [ ] Authenticated `GetEntitlement`, `ApproveEntitlement`, and `ApproveAccount` calls succeed before production enablement
- [ ] Procurement client applies per-call context deadlines and HTTP transport timeouts
- [ ] Procurement timeout tests prove stalled responses return transient errors and do not block Pub/Sub handlers indefinitely
- [ ] Procurement URL construction tests verify every method request has exactly one `/v1` segment
- [ ] `GetEntitlement` 404 returns typed not-found; other 4xx are permanent and 5xx/network errors are transient

### Operational

- [ ] Controller starts and processes events in active region
- [ ] Standby region has `replicas: 0` (no processing)
- [ ] DR failover: scale up in DR region → events processed within 1 minute
- [ ] Poison message → dead-letter topic after max retries
- [ ] Prometheus metrics scraped by ServiceMonitor
- [ ] Alerts fire for provisioning slow, procurement errors, dead-letter messages
- [ ] `kubectl get marketplaceentitlements` shows all entitlements with condition summaries
- [ ] Unit test coverage > 80% on controller and handler code
- [ ] E2E test covers full purchase → sign-up → provision → suspension → reactivation → cancellation → deletion lifecycle
