# GCP Marketplace SaaS Integration for Gecko Platform

***Scope***: GCP-HCP

**Date**: 2026-08-26

## Decision

We will integrate GCP HCP Platform API (Gecko) with Google Cloud Marketplace as a SaaS offering using an event-driven architecture. A marketplace controller (Go, subcommand of the existing `gecko-controllers` binary) manages the full entitlement lifecycle — from purchase through provisioning, plan changes, suspension, cancellation, and deletion. The controller subscribes to two Pub/Sub topics: one owned by Google (purchase/lifecycle events) and one owned by GCP HCP (sign-up events). A new `MarketplaceEntitlement` API in Gecko records each customer's Marketplace entitlement and drives condition-based reconciliation for sign-up, provisioning, access, cancellation, cleanup, and metering readiness. The sign-up page is a separate module owned by the Partner Enablement Team at Red Hat: Google sends the purchaser there with an `x-gcp-marketplace-token`, the sign-up page verifies that token and the user's email identity, then publishes a Gecko-defined `SIGNUP_COMPLETED` event to the GCP HCP Team's Pub/Sub topic. The controller is deployed to all regions but runs with `replicas > 0` in a single active region; disaster recovery is a Helm values change to scale up in another region. ESPv2 remains the API frontend and Service Control integration point (see [espv2-api-frontend](../networking/espv2-api-frontend.md)) for request-level operational telemetry. This decision records the entitlement lifecycle and the Procurement API `usageReportingId` handoff required for future billing attribution; the billing reporter, usage aggregation, Service Control metric definitions, and pricing behavior are out of scope and will be defined in a separate DD/IP.

## Context

- **Problem Statement**: GCP HCP API needs a commercial path to customers via GCP Marketplace. This requires: (1) handling Marketplace entitlement lifecycle events (purchase, plan change, suspension, cancellation, deletion), (2) provisioning tenant resources (namespace, RBAC), (3) a customer sign-up flow that verifies Google's `x-gcp-marketplace-token`, captures Marketplace account/user identity from the verified token, and captures verified user email identity, (4) Procurement API integration for entitlement and account approval, and (5) persisting Marketplace billing attribution context for a separate metering implementation. The [ESPv2 frontend decision](../networking/espv2-api-frontend.md) already established Service Control as the API frontend integration; this decision defines the Marketplace-specific entitlement and access integration.
- **Constraints**: Must use Google's Procurement API for entitlement approval (Google's required pattern). Must normalize Marketplace event IDs into full Procurement API resource names using the configured provider ID before API calls. Must use Pub/Sub as the event transport (Google publishes to a topic). Must integrate with existing GCP HCP API Authorization (Cedar-based in Gecko) — provisioning creates RoleBindings consumed by Cedar. Must support the team boundary with the Partner Enablement Team (sign-up page owned by a separate team). Must be deployable to all regions with a single-active controller pattern for disaster recovery.
- **Assumptions**: One entitlement per GCP project (one namespace per customer). Multiple orders are disabled for launch; if enabled later, the sign-up event contract must include entitlement/order identity instead of relying on account-only lookup. The `MarketplaceEntitlement` Kubernetes object name is a deterministic DNS-safe value derived from the Marketplace entitlement resource name; raw Google IDs and full Procurement resource names are stored separately. The first user to complete verified sign-up receives the `service-admin` PlatformRole binding. GCP project number is derived from the Procurement API entitlement consumers list and is suitable as the Kubernetes namespace name. The Partner Enablement Team will own the sign-up page and integrate via a Pub/Sub event contract after verifying Google's Marketplace token. Eventual consistency across regions for Authorization is acceptable, as Marketplace-created RoleBindings depend on the planned cross-region resource replication mechanism, which is tracked separately and must be available before marketplace-created access is considered multi-region complete.

## Alternatives Considered

1. **Event-driven controller with condition-based CRD reconciliation**: Pub/Sub handlers update a `MarketplaceEntitlement` CRD, a reconciler evaluates Kubernetes-style status conditions and provisions resources. Sign-up page is a separate module communicating via Pub/Sub. Controller deployed with `replicas: 0` in standby regions for DR failover.
2. **Webhook-based integration (synchronous)**: Google Marketplace calls a webhook endpoint on entitlement events. The endpoint provisions resources synchronously and returns a response within Google's timeout window.
3. **Cloud Functions / Cloud Run event handler**: Serverless functions triggered by Pub/Sub, calling the Platform API to create resources. No controller-runtime, no CRD — state tracked in database rows or Pub/Sub acknowledgment.
4. **Direct database writes without CRD**: Skip the CRD and write directly to the Platform API's storage backend. The controller manages entitlement state in database rows rather than Kubernetes resources.

## Decision Rationale

* **Justification**: A condition-based CRD is consistent with GCP HCP API's controller-runtime architecture and existing Kubernetes API patterns. Pub/Sub decouples all three parties (Google, marketplace controller, sign-up page). The CRD provides observability via `kubectl`, audit logging via Kubernetes events, and crash recovery via the reconciler pattern. The single-active controller with `replicas: 0` standby avoids leader election complexity while providing sub-minute DR failover.
* **Evidence**: A proof-of-concept presented to the team (see team's demo recordings) validated the full purchase → sign-up → RBAC provisioning → Cedar authorization flow end-to-end in a two-cluster Kind environment. A 1,335-line E2E test covers 13 steps including marketplace-created RoleBinding behavior across two clusters, which informs the planned cross-region replication integration.
* **Comparison**: Webhook integration (Alternative 2) couples provisioning to the request path — Google's callback has strict timeout requirements, and a slow provision (namespace + RBAC + Procurement API) would exceed them. Cloud Functions (Alternative 3) introduces a new runtime and deployment model inconsistent with the existing Go controller pattern and complicates local development. Direct database writes (Alternative 4) lose CRD observability (`kubectl get marketplaceentitlements`), Kubernetes event history, and the reconciler retry model.

## Consequences

### Positive

* Consistent with existing Gecko controller-runtime architecture — same patterns as `hc/`, `nodepool/`, and `placement/` controllers
* Kubernetes-style conditions provide an inspectable readiness and lifecycle contract via Kubernetes events and `kubectl` inspection
* Reconciler pattern handles crash recovery and retries automatically — no event is lost if the controller restarts mid-provision
* Pub/Sub event contract enables clean team boundary with Partner Enablement Team — sign-up page has zero Gecko code dependency
* RoleBindings created by the controller can use the planned cross-region resource replication mechanism once it lands — no marketplace-specific replication code
* Single-active controller with `replicas: 0` standby in all other regions — DR failover is a Helm values change, no leader election
* ESPv2 Service Control integration provides the API frontend telemetry foundation; the Marketplace controller supplies the `usageReportingId` handoff needed by a separate billing implementation

### Negative

* Single-active region means purchase events queue in Pub/Sub during failover window (no data loss due to Pub/Sub retention, but provisioning latency increases)
* Entitlement condition complexity grows with lifecycle events (Google account, entitlement, sign-up, access, cancellation, cleanup, and metering conditions)
* Sign-up page team boundary requires a well-defined Pub/Sub event contract — schema evolution must be coordinated across teams
* Marketplace billing attribution requires a separate billing/metering DD/IP beyond this entitlement lifecycle decision
* Namespace cleanup after final cancellation requires a 30-day grace period and garbage collection — operational complexity for data retention compliance
* Marketplace suspension may need to be inferred from supported Google signals such as Procurement API state or Service Control check errors if SaaS Pub/Sub suspension events are unavailable

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Entitlement events are low-volume (one purchase per customer, infrequent lifecycle changes). A single-replica controller is sufficient. Pub/Sub handles backpressure and message retention during failover.
* **Observability**: Prometheus metrics for entitlement condition counts, provisioning duration, Procurement API errors, and Pub/Sub message rates. Structured audit logs for all condition changes. Kubernetes events on the CRD for operator visibility.
* **Resiliency**: Pub/Sub provides at-least-once delivery with configurable retry and dead-letter topics for poison messages. Idempotent handlers prevent duplicate provisioning. DR failover by scaling up a standby controller in another region — Pub/Sub subscriptions retain unacknowledged messages.

### Security:

* Production sign-up requires two layers of verification: the sign-up page verifies Google's `x-gcp-marketplace-token` for Marketplace account/user binding, then verifies the user's email identity before publishing `SIGNUP_COMPLETED`
* GCP HCP Marketplace Controller accepts Marketplace account binding only from the verified sign-up page's Pub/Sub event; it does not trust user-editable account IDs from URL parameters or form fields
* User email and obfuscated Marketplace user identity are excluded from public API output via `+orlop:public` marker omission
* Procurement API authenticated via Workload Identity Federation (no static keys)
* Pub/Sub subscriptions authenticated via GCP IAM with least-privilege service accounts
* Final cancellation includes a 30-day grace period before namespace cleanup to prevent accidental data loss; Google deletion/account deletion events trigger immediate private identity/billing cleanup, while minimal non-PII cleanup state is retained until namespace termination is confirmed

### Performance:

* Entitlement provisioning is asynchronous — no latency requirements on the customer's purchase path
* Target: < 5 minutes from purchase event receipt to entitlement approval, namespace readiness, and the customer being ready for sign-up; this excludes customer-controlled sign-up time
* Target: < 5 minutes from receipt of `SIGNUP_COMPLETED` to `AccessReady=True` when Marketplace lifecycle gates are eligible
* GCP HCP API policy hot-reload ensures the new RoleBinding takes effect within seconds of creation

### Cost:

* Pub/Sub costs are negligible at expected message volumes (< 1000 messages/day at scale)
* No new infrastructure beyond the controller deployment (reuses existing `gecko-controllers` binary and image)
* The controller adds no billing reporter infrastructure; it only persists the `usageReportingId` required by a future billing implementation

### Operability:

* Controller deployed via Helm/ArgoCD with `replicas` configurable per region
* DR failover procedure: set `replicas: 1` in the DR region, `replicas: 0` in the failed region — single Helm values change
* `kubectl get marketplaceentitlements` provides immediate visibility into entitlement conditions
* Dead-letter topic captures poison messages for debugging without blocking the subscription
