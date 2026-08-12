# Cedar-Based Authorization for Gecko Public API

***Scope***: GCP-HCP

**Date**: 2026-08-11

## Decision

We will use [Cedar](https://www.cedarpolicy.com/) as the authorization engine for Gecko's public (customer-facing) API. Cedar policies are generated from ConfigMap-defined roles at startup and evaluated in-process per request — no external authorization service on the request path. Role bindings are stored in the same Spanner database as other Gecko resources. Each regional Gecko instance is independently authoritative for its own bindings.

## Context

- **Problem Statement**: Gecko's public API (customer-facing, chi.Router-based, port 8081) requires authorization to control which customers can perform which actions (create clusters, manage node pools, administer role bindings). The [gecko-api-aggregation](gecko-api-aggregation.md) decision delegates the *private/internal* API's authn/authz to the GKE kube-apiserver, but the public API remains a standalone HTTP server that must implement its own authorization. Google confirmed that GCP IAM cannot be used as the authorization engine for non-first-party services, so an independent system is required.
- **Constraints**: Must run in-process within `platform-api-server` (no external service dependency on the authorization hot path). Must support namespace-scoped and platform-scoped roles. Must work with the existing `ResourceStore` interface and all three storage backends (Spanner, PostgreSQL, in-memory). Must support future attribute-based access control (ABAC) via conditions on resource properties. Must integrate with ESPv2 sidecar authentication (`X-Endpoint-API-UserInfo` header).
- **Assumptions**: User identity is a Google account email extracted from the ESPv2-injected JWT claims header. Role definitions change infrequently (deployment-time ConfigMap). Role bindings change occasionally (admin operations). Authorization decisions are dominated by reads (every API request), not writes.

## Alternatives Considered

1. **Cedar (in-process policy evaluation)**: Embed the `cedar-go` library in `platform-api-server`. Generate Cedar policies from ConfigMap-defined roles. Evaluate policies per request against a cached entity graph built from RoleBinding/PlatformRoleBinding records in the database.
2. **SpiceDB (external authorization service)**: Deploy SpiceDB as a separate service. Gecko calls SpiceDB's `CheckPermission` RPC on each request. Relationship tuples stored in SpiceDB's own database.
3. **Custom RBAC engine**: Build a bespoke permission-checking system with role/binding tables and a Go function that evaluates access. No policy language or external dependency.

## Decision Rationale

* **Justification**: Cedar provides a formal policy language with well-defined semantics (`permit`/`forbid`, transitive `in` operator, typed entities) that maps directly to the authorization model. In-process evaluation eliminates network round-trips and external service availability concerns. The `forbid`-overrides-`permit` semantics enable safe composition of user-defined custom roles — a platform-level `forbid` policy cannot be overridden by any number of `permit` policies. The `cedar-go` library is maintained by AWS/Cedar and provides the full evaluation engine as a Go package.
* **Evidence**: The proof-of-concept implementation (gecko `authz` branch) demonstrates the full authorization flow: ConfigMap parsing, Cedar policy generation, entity graph construction from database-backed stores, per-request evaluation, cross-namespace list filtering, and bootstrap. The implementation adds ~2,000 lines of Go with 59 tests and 127 subtests covering all role-action combinations.
* **Comparison**: SpiceDB (Alternative 2) adds operational complexity (separate service, separate database, network dependency on every request). SpiceDB does support conditional relationships via its caveat mechanism (CEL expressions evaluated at check time), so ABAC-style conditions are achievable. However, they require a different data model (caveated tuples) and the consistency/caching tradeoffs of an external service — freshness is controlled by ZedTokens, not in-process state. Cedar's native `when` conditions in the policy language, combined with in-process evaluation, provide a simpler path to the ABAC requirements here. A custom RBAC engine (Alternative 3) lacks a formal policy language, making future ABAC support a retrofit rather than a natural extension.

## Consequences

### Positive

* Zero external service dependency on the authorization hot path — Cedar evaluation is a pure function call in the same process
* Cedar conditions enable attribute-based access control (Phase 6: custom roles with `when` clauses referencing resource attributes like region, labels)
* `forbid`-overrides-`permit` semantics provide a safety net for platform-level restrictions that custom roles cannot circumvent
* ConfigMap-defined roles are versioned in Git alongside the Helm chart, reviewed via PR, and deployed as a unit — immutable infrastructure for authorization policy
* Entity cache with per-user dirty invalidation minimizes database queries (most requests served from cache)
* Cross-namespace list authorization is handled at the database level (`WHERE namespace IN UNNEST(...)` in Spanner, `WHERE namespace = ANY($N)` in PostgreSQL) — no over-fetching or post-filtering

### Negative

* Cedar is a newer policy language with a smaller ecosystem than OPA/Rego or SpiceDB's Zanzibar model — fewer community resources and third-party integrations
* The `cedar-go` library is a dependency that must be tracked for security updates
* ConfigMap-based role definitions require a redeploy to change — acceptable for built-in roles but mitigated by Phase 6 custom roles for user-defined policies
* Each regional Gecko instance maintains its own role bindings independently — cross-region consistency requires explicit fan-out (Marketplace handler) or async replication (future)

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Cedar evaluation is stateless and CPU-bound (microseconds per decision). The entity cache is per-instance; horizontal scaling adds more cache capacity. Database load is proportional to cache misses, which occur only on first access per user or after binding changes.
* **Observability**: Authorization decisions (allow/deny) are logged with a hashed principal identifier, action, resource, and determining policy — raw customer emails are not written to routine logs. Audit-level logging of raw principals (for compliance purposes) is stored separately with restricted access and defined retention. Cedar evaluation latency is trackable via existing request metrics. Cache hit/miss rates are observable.
* **Resiliency**: Authorization is fully local to each Gecko instance — no cross-region or cross-service dependency. If the database is temporarily unavailable, cached entity graphs continue serving authorization decisions. New users who have never been seen will get 403 until the database is reachable (fail-closed). Cache staleness is bounded by a maximum TTL (entries expire even without a write-triggered invalidation) and by Spanner Change Streams, which broadcast binding mutations to all regional instances so their per-user cache entries are evicted on writes from any instance.

### Security:

* Default-deny: a user with valid authentication but no role bindings gets 403 on every operation
* Separation of concerns: access management roles (platform-admin, service-admin) are fully separated from infrastructure management roles (cluster-admin, cluster-viewer) — no single role conflates both
* Custom role namespace-pinning (Phase 6): user-defined roles are always scoped to their namespace via Cedar AST inspection — cross-namespace escalation is rejected at creation time. Generated policies constrain both the principal (`principal in Namespace::"<ns>"`) and the resource (`resource in Namespace::"<ns>"`), ensuring a principal bound in namespace A cannot access resources in namespace B even if a Cedar condition on resource attributes would otherwise match.
* RoleRef validation: binding creation validates that the referenced role exists and has the correct scope (namespace vs. platform)

### Performance:

* Cedar policy evaluation: sub-millisecond per request (pure in-memory computation against cached entity graph)
* Entity cache: populated on first access per user, served from memory on subsequent requests, invalidated only on binding writes for the affected user
* Cross-namespace list queries: database-level filtering via `WHERE namespace IN UNNEST(...)` (Spanner) or `WHERE namespace = ANY($N)` (PostgreSQL) — no post-query filtering, pagination works natively

### Cost:

* No additional infrastructure cost — Cedar runs in-process within the existing `platform-api-server` deployment
* No per-request cost (unlike external authorization services that may charge per check)
* Spanner cost for binding storage is negligible (small records, infrequent writes)

### Operability:

* Built-in roles are defined in a ConfigMap mounted at `/etc/gecko/authz/` — changes require a Helm chart update and redeploy
* Bootstrap mechanism creates the initial platform-admin from the ConfigMap on first startup (idempotent)
* `--disable-auth` flag disables both authn and authz for local development (binds to localhost only)
* `--authz-config` flag specifies the ConfigMap mount path (default: `/etc/gecko/authz/`)
