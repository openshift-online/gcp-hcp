# Public API Quota: Namespace-Scoped Resource Limits with Adaptive Scaling

***Scope***: GCP-HCP

**Date**: 2026-09-24

## Decision

We will implement a namespace-scoped quota system in the gecko public API that enforces per-project limits on HostedCluster and NodePool resources, rejects quota-exceeding creation requests with `HTTP 413`, and adapts quota ceilings upward under sustained utilization — returning to the base level when resources are released. A new `QuotaRequest` resource allows users to view their current quota usage and request increases with a stated justification.

## Context

The GCP HCP public API is a multi-tenant service where a single tenant's workload can exhaust shared infrastructure — PSC service attachments, GKE node capacity, control-plane compute — before GCP-level hard limits surface. Beyond infrastructure protection, unrestricted provisioning exposes customers to accidental large bills if automation or operator error creates far more clusters than intended.

- **Problem Statement**: Without application-level quota enforcement, a single project can:
  - Exhaust GCP PSC service attachment limits (50–500 per management cluster), impacting all tenants sharing that cluster.
  - Incur unexpectedly large cloud bills through accidental over-provisioning (runaway automation, misconfigured scripts).
  - Starve other projects of capacity without any prior notice or escalation path.
  - Bypass per-project fairness — a high-traffic project receives the same share of capacity as a low-traffic one with no mechanism to adjust dynamically.
- **Constraints**: Quota enforcement must be atomic with object creation — a request that would exceed quota must be rejected before the object is persisted. Quota state must be authoritative and consistent even under concurrent creation requests. Quota limits must be visible to users at any time and adjustable via a supported workflow. The mechanism must not introduce disproportionate latency on the common (non-quota-exceeding) path.
- **Assumptions**: Namespace equals project: the existing Cedar authorization model already treats namespace as the unit of tenant isolation, and quota inherits this boundary. The gecko API server's storage backend (PostgreSQL/Spanner) supports transactional reads under write isolation sufficient for atomic quota checks. Base limits (50 HostedClusters, 500 NodePools per namespace) are conservative enough to serve the majority of tenants without manual intervention while still protecting shared infrastructure. Adaptive scaling operates on a week-level cadence; sub-minute burst protection is handled separately at the ESPv2 layer via Cloud Endpoints rate limits.

## Alternatives Considered

1. **Application-level quota in gecko with adaptive scaling (chosen)**: Gecko enforces quota atomically inside the creation handler, persists quota state in its existing storage backend, and adapts limits based on utilization history. A `QuotaRequest` resource surfaces limits and provides an increase workflow within the existing API surface.
2. **ESPv2 / Cloud Endpoints quota only**: Enforce per-API-key or per-consumer quota purely at the ESPv2 sidecar layer via Service Infrastructure `Check` calls. Cloud Endpoints supports per-method quota on a per-consumer basis configured through the OpenAPI spec and `serviceusage.googleapis.com`.
3. **Static admin-configured limits with no self-service**: Operators set per-namespace quota via a Helm values file or a private-API-only resource. Users cannot view quota or request increases; they must contact support.
4. **No quota, rely on GCP infrastructure limits**: Accept that PSC attachment and compute quota will act as the natural ceiling. Rely on billing alerts to catch accidental over-provisioning.

## Decision Rationale

* **Justification**: Application-level enforcement in gecko is the only option that provides atomic consistency (the quota check and the object write happen in the same transaction), user visibility (quota is a first-class API resource), and an adaptive mechanism that removes toil for both operators and tenants as projects grow. ESPv2 quota (Alternative 2) operates on request counts, not object counts — it cannot distinguish "already has 48 clusters and wants 3 more" from "has 0 clusters". Static admin limits (Alternative 3) require operator involvement for every tenant growth event and provide no self-service path. Relying solely on GCP limits (Alternative 4) gives users no early warning, no actionable error, and no way to plan capacity.

* **Evidence**: The L2 frontend architecture sketch (`experiments/arch/L2 Container/Frontend Service/README.md`) already proposed a `CLUSTER_QUOTA_EXCEEDED` error type with `current_usage` and `quota_limit` response fields, confirming this was a known requirement. Google Cloud APIs (Compute Engine, GKE, Cloud Run) consistently use `HTTP 429` for rate limits and `HTTP 429`/`HTTP 403` for quota; GCP's own CLM surface uses `HTTP 413` (content too large / quota exceeded) when a resource limit is breached — this decision aligns with that convention. PSC service attachment research (`psc_research_findings.md`) identifies 50–500 attachments per management cluster as the hard infrastructure ceiling; a per-namespace limit well below this ensures no single tenant can saturate a management cluster.

* **Comparison**: ESPv2 quota (Alternative 2) has no object-count semantic — it counts API calls, not persisted objects. A client that creates one cluster and lists it 10,000 times would exhaust an ESPv2 quota long before a client that creates 100 clusters with no further calls. ESPv2 quota is retained as a complementary request-rate layer but cannot replace object-count enforcement. Static limits (Alternative 3) scale poorly with tenant count; adaptive limits eliminate most operator intervention. GCP-only limits (Alternative 4) surface too late, produce unhelpful errors, and carry no per-tenant attribution.

## Quota Model

### Base Limits

| Resource | Base Limit | Maximum (3×) |
|---|---|---|
| HostedClusters per namespace | 50 | 150 |
| NodePools per namespace | 500 | 1500 |

NodePool limits are enforced independently of cluster count — a namespace with 10 clusters can allocate its 500 NodePools freely across those clusters, subject only to the aggregate namespace ceiling.

The maximum column reflects the ceiling the adaptive scaling mechanism will reach autonomously without any manual intervention. It is not an absolute system limit — higher values can be set by operators directly via the private API, or granted in response to a `QuotaRequest` submitted by the user. Both paths are uncapped and subject only to available shared infrastructure capacity.

### Adaptive Scaling

Quota limits adapt automatically based on observed utilization over a rolling window:

- **Scale-up**: When utilization reaches ≥ 50% of the current limit for a sustained period of 7 days, the limit increases by one step (25% of base, rounded up). Scale-up continues at weekly intervals until the limit reaches 3× the base value.
- **Scale-down**: When utilization falls below the scale-up threshold for a sustained period of 30 days, the limit decreases by one step toward the base value. Scale-down never drops below the base limit.
- **Manual increase**: A `QuotaRequest` resource (see API below) allows users to request an increase beyond 3× base, supplying a justification. Operators approve or deny via the private API. Approved manual limits override the adaptive ceiling.

The intent is that a project growing organically never needs to interact with the quota system at all — limits follow usage automatically. Quota requests are an escape hatch for planned bursts (migrations, large onboarding events) that would exceed the adaptive ceiling before the 7-day window matures.

Knowing quota ahead of actual usage — both through adaptive projections and explicit `QuotaRequest` increase requests — gives the platform advance notice to pre-provision shared infrastructure (PSC service attachments, management cluster node pools, IP ranges) before tenants reach their new ceiling. This avoids a cold-provisioning delay at the moment of peak demand and improves service quality for customers during planned growth events.

### Error Response

When a creation request would exceed quota, the API returns:

```
HTTP 413 Content Too Large
```

```json
{
  "apiVersion": "platform.gcp-hcp.openshift.io/v1alpha1",
  "kind": "Status",
  "status": "Failure",
  "reason": "QuotaExceeded",
  "message": "quota exceeded: namespace \"acme-prod\" has 50 HostedClusters (limit: 50)",
  "details": {
    "kind": "HostedCluster",
    "quota": {
      "resource": "hostedclusters",
      "namespace": "acme-prod",
      "current": 50,
      "limit": 50
    }
  },
  "code": 413
}
```

The object is not created. No partial state is written. The response body gives the user sufficient information to understand why the request was rejected and where to look for relief (current usage, current limit, resource kind).

## Quota API

### Quota (read-only, public API)

A new `Quota` resource, scoped to a namespace, is the authoritative place to read both the current quota settings and live consumption for a namespace. It is served as a read-only singleton per namespace (name: `default`) on the public API. The `list` and `get` verbs are exposed; `create`, `update`, `patch`, and `delete` are restricted (annotation: `+orlop:public-verbs: list,get`).

```yaml
apiVersion: platform.gcp-hcp.openshift.io/v1alpha1
kind: Quota
metadata:
  name: default
  namespace: acme-prod
spec:
  resources:
    - resource: hostedclusters
      limit: 50           # effective limit (max of adaptiveLimit and manualLimit)
      adaptiveLimit: 50   # current adaptive ceiling
      manualLimit: null   # operator-approved manual override, null if not set
    - resource: nodepools
      limit: 500
      adaptiveLimit: 500
      manualLimit: null
status:
  resources:
    - resource: hostedclusters
      current: 12
      scaleUpEligibleAt: null   # null = not yet at 50% threshold
      scaleDownEligibleAt: null
    - resource: nodepools
      current: 87
      scaleUpEligibleAt: "2026-10-01T00:00:00Z"  # reached 50% threshold
      scaleDownEligibleAt: null
```

### QuotaRequest (read/write, public API)

A `QuotaRequest` resource allows users to request a quota increase beyond the adaptive ceiling. Verbs: `create`, `get`, `list`, `delete` (no update or patch — requests are immutable once submitted; operators act on them via the private API). Annotated `+orlop:public-verbs: create,get,list,delete`.

Deletion is only permitted while the request is in `Pending` or `Denied` phase — this allows users to withdraw a request that has not yet been acted on, or to clear a denied request. Attempts to delete an `Approved` `QuotaRequest` are rejected with `HTTP 409 Conflict`, preserving approved requests as an immutable audit record of granted quota increases.

```yaml
apiVersion: platform.gcp-hcp.openshift.io/v1alpha1
kind: QuotaRequest
metadata:
  name: migration-wave-1
  namespace: acme-prod
spec:
  resource: hostedclusters
  requestedLimit: 200
  reason: "Migrating 150 on-premise clusters over Q4 2026. Batch 1 of 3."
  neededBy: "2026-10-15T00:00:00Z"
status:
  phase: Pending   # Pending | Approved | Denied
  approvedLimit: null
  decidedAt: null
  decisionNote: null
```

## Implementation

### Enforcement point

Quota is enforced inside gecko's creation handler, after schema validation and Cedar authorization have passed, and inside the same database transaction as the object write. The check is:

```
current_count = SELECT COUNT(*) FROM resources
                WHERE kind = $kind AND namespace = $namespace
                FOR SHARE NOWAIT;

IF current_count >= quota_limit THEN
  RETURN 413 QuotaExceeded
END IF

INSERT INTO resources ...
```

Using `FOR SHARE` (PostgreSQL) or equivalent read-lock semantics in Spanner prevents two concurrent creation requests from both reading `count=49` against a limit of 50 and both succeeding. The quota check does not add a round-trip; it is a single transaction that either commits the new object or rolls back with the 413 response.

### Quota state storage

Quota configuration (base limits, manual overrides, adaptive history) is stored in a dedicated `quota` table in the same PostgreSQL/Spanner backend. The adaptive scaling evaluation runs as a periodic reconciliation in the gecko quota controller (not in the request hot path). Quota state is read from this table on every creation request; the table is small (one row per resource kind per namespace) and benefits from the database's buffer cache at steady state.

### Adaptive scaling controller

A gecko-internal controller evaluates quota adaptation on a configurable schedule (default: hourly evaluation, 7-day sustained threshold for scale-up, 30-day for scale-down). It reads utilization history from the quota table, computes the new limit, and writes it back — triggering no external side effects. Operator-approved manual limits are not touched by the adaptive controller.

## Consequences

### Positive

* Tenants receive a self-service visibility surface (`Quota`) and an escalation path (`QuotaRequest`) without requiring support contact for routine growth.
* Shared infrastructure (PSC attachments, management cluster node capacity) is protected from single-tenant exhaustion.
* Customers are protected from accidental bill shock caused by runaway automation.
* `HTTP 413` with structured `QuotaExceeded` details aligns with GCP API conventions — familiar to GCP-native operators.
* Adaptive scaling eliminates operator toil for organic growth: most tenants never hit the ceiling.
* Quota enforcement is a single chokepoint in the gecko creation handler — no per-controller duplication.

### Negative

* Transactional quota checks add a `FOR SHARE` lock to every creation request, increasing write latency slightly and creating potential lock contention at very high creation concurrency.
* Adaptive scaling logic must be carefully tuned to avoid oscillation (rapid scale-up/scale-down cycles) under bursty but irregular usage patterns.
* The `QuotaRequest` approval workflow requires operator involvement for above-ceiling increases — this is intentional but creates a support queue.
* Deletion of `Approved` requests is blocked on the public API — operators must use the private API to remove stale approved records if ever necessary.
* Two new resource types (`Quota`, `QuotaRequest`) add API surface that must be versioned and maintained.

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Quota state is a small, infrequently written table; reads are hot-cached in the database buffer pool. Lock contention on `FOR SHARE` is negligible for expected creation rates (single-digit requests per second per namespace). At extreme concurrency (bulk migration tooling), contention increases linearly — mitigated by the `QuotaRequest` manual increase workflow, which raises limits before bulk operations begin.
* **Observability**: Emit a `quota_check_duration_seconds` histogram and `quota_exceeded_total` counter (labelled by namespace and resource kind) on every creation attempt. Alert when `quota_exceeded_total` spikes for a namespace — this is an early signal that a tenant is approaching a ceiling and may need a manual increase before they raise a support ticket. Log the quota decision (allowed/denied, current/limit) at INFO level on every creation request.
* **Resiliency**: If the quota table is unreadable due to a database outage, gecko returns `503 Service Unavailable` — it does not fail open and allow unlimited creation. This is a safe failure mode: brief unavailability is preferable to quota bypass. The adaptive controller is non-critical; if it fails to run for several cycles, limits remain at their last computed value until the next successful evaluation.

### Security:

* Quota state is writable only via the gecko quota controller (private API, RBAC-restricted). Users cannot manipulate their own limits via the public API.
* `QuotaRequest` objects are immutable after creation — users cannot modify the `requestedLimit` or `reason` after submission, preventing retroactive justification changes. Approved requests cannot be deleted via the public API, preserving them as an audit record.
* The `FOR SHARE` lock prevents quota bypass via concurrent creation — a common attack pattern against quota systems that read and write non-atomically.
* Operator approval of a `QuotaRequest` above 3× base requires justification and is logged in the kube audit trail (private API call via `GenericAPIServer`).

### Performance:

* The `FOR SHARE` lock on the quota check is expected to add < 5ms to creation latency at p99 under normal load. Object creation is already the most expensive path (schema validation, Cedar authorization, database write); the lock overhead is small relative to these existing costs.
* `Quota` reads are served from the quota table with no additional joins; expected latency < 10ms at p99.

### Cost:

* Quota state adds a small number of rows to the existing PostgreSQL/Spanner instance — negligible storage cost.
* The adaptive controller runs on an hourly schedule as an in-process goroutine — no additional compute cost.
* Preventing runaway cluster creation is expected to provide significant cost savings in avoided infrastructure spend, outweighing the implementation cost by a large margin.

### Operability:

* Operators manage quota via the private API (`kubectl get/patch quota -n <namespace>` via the aggregated `GenericAPIServer`). No bespoke tooling required.
* `QuotaRequest` objects surface in standard `kubectl` workflows: `kubectl get quotarequests -n acme-prod` shows all pending requests; `kubectl patch quotarequest migration-wave-1 -n acme-prod --subresource=status` approves or denies.
* Base limits and the adaptive scaling parameters (scale-up threshold, scale-up period, scale-down period, maximum multiplier) are configurable via gecko Helm values, allowing per-environment tuning without code changes.
* Runbooks for common operator actions (approve a QuotaRequest, override adaptive scaling for a namespace, reset a namespace to base limits) should be added to the SRE playbook at initial rollout.
