# Cedar-Based Authorization for Gecko Public API

***Scope***: GCP-HCP

**Date**: 2026-08-25 (updated from 2026-08-11 based on PoC findings)

## Decision

We will use [Cedar](https://www.cedarpolicy.com/) as the authorization engine for Gecko's public (customer-facing) API. Cedar policies are generated from database-stored roles and evaluated in-process per request — no external authorization service on the request path. System roles (`PlatformRole`, cluster-scoped) are seeded via Helm chart templates and managed through the private API. User-defined roles (`Role`, namespace-scoped) are managed via the public API in the configured leader region. Role bindings (`RoleBinding`, namespace-scoped) bind a user to either a `PlatformRole` or a `Role` within a namespace and may carry an optional Cedar condition for attribute-based access control. Authorization decisions for conditioned bindings require object-state-aware evaluation — the authorization middleware must evaluate Cedar policies against the effective resource state (decoded request body for creates, stored object for reads/deletes, post-update state for updates). Each regional Gecko instance maintains its own local copy of roles and bindings through [cross-region resource replication](../infrastructure/cross-region-resource-replication.md) and makes authorization decisions based on that local state, which may lag during replication outages or watch failures.

## Context

- **Problem Statement**: Gecko's public API (customer-facing, chi.Router-based, port 8081) requires authorization to control which customers can perform which actions (create clusters, manage node pools, administer role bindings). The [gecko-api-aggregation](gecko-api-aggregation.md) decision delegates the *private/internal* API's authn/authz to the GKE kube-apiserver, but the public API remains a standalone HTTP server that must implement its own authorization. Google confirmed that GCP IAM cannot be used as the authorization engine for non-first-party services, so an independent system is required.
- **Constraints**: Must run in-process within `platform-api-server` (no external service dependency on the authorization hot path). Must support namespace-scoped roles. Must work with the existing `ResourceStore` interface and all three storage backends (Spanner, PostgreSQL, in-memory). Must support attribute-based access control (ABAC) via conditions on resource properties, which requires authorization middleware to evaluate policies against effective resource state. Must integrate with ESPv2 sidecar authentication (`X-Endpoint-API-UserInfo` header) with strict trust boundary enforcement — the public API backend listener must only be reachable through the ESPv2 sidecar proxy to prevent header spoofing.
- **Assumptions**: User identity is a Google account email extracted from the ESPv2-injected JWT claims header with `email_verified == true` when available. Email-based principal keys carry case-sensitivity and account reassignment risks that are acceptable for the target user base (Google Workspace accounts with stable email addresses). System role definitions change infrequently (deployed via Helm). User-defined roles and role bindings change occasionally (admin operations) and are written through the configured leader region. Authorization decisions are dominated by reads (every API request), not writes. Cross-region replication provides eventual consistency for authorization data with bounded lag (typically sub-second, unbounded during Pub/Sub outages). Policy and entity cache staleness across horizontally scaled API server replicas is bounded by watch delivery latency plus an optional periodic resync interval.

## Alternatives Considered

1. **Cedar (in-process policy evaluation)**: Embed the `cedar-go` library in `platform-api-server`. Generate Cedar policies from database-stored roles. Evaluate policies per request against a cached entity graph built from RoleBinding records in the database.
2. **SpiceDB (external authorization service)**: Deploy SpiceDB as a separate service. Gecko calls SpiceDB's `CheckPermission` RPC on each request. Relationship tuples stored in SpiceDB's own database. SpiceDB supports conditional relationships via its caveat mechanism (CEL expressions evaluated at check time), so ABAC-style conditions are achievable through caveated tuples.
3. **Custom RBAC engine**: Build a bespoke permission-checking system with role/binding tables and a Go function that evaluates access. No policy language or external dependency.
4. **Separate cluster-scoped PlatformRoleBinding type**: Introduce `PlatformRoleBinding` as a cluster-scoped resource (distinct from the namespace-scoped `RoleBinding`) for binding users to platform-level roles. This would add a `platform-admin` role with `platformrolebinding.*` permissions and a separate public API surface for platform-level access management.

## Decision Rationale

* **Justification**: Cedar provides a formal policy language with well-defined semantics (`permit`/`forbid`, transitive `in` operator, typed entities) that maps directly to the authorization model. In-process evaluation eliminates network round-trips and external service availability concerns. The `forbid`-overrides-`permit` semantics provide a safety mechanism for future platform-level restrictions (though platform forbid policies are not implemented in the initial release). The `cedar-go` library is maintained by AWS/Cedar and provides the full evaluation engine as a Go package. Object-state-aware authorization enables rich ABAC conditions (e.g., region-based access, label selectors) while requiring the authorization middleware to fetch or decode resource state before policy evaluation.
* **Evidence**: A proof-of-concept presented to the team (see team's demo recordings) demonstrated the full authorization flow: Helm-based system role seeding, Cedar policy generation from database-stored roles, per-binding policy isolation, entity graph construction, per-request evaluation, cross-namespace list filtering with per-item condition filtering, and hot-reload with atomic policy swap. The implementation adds ~2,000 lines of Go with 59 tests and 127 subtests. A critical security finding from the PoC: policies must be generated **per-binding** (not per-role) to prevent unconditional bindings from satisfying conditioned policies for the same role — this was validated with targeted security tests.
* **Comparison**: SpiceDB (Alternative 2) adds operational complexity (separate service, separate database, network dependency on every request). While SpiceDB supports ABAC via CEL-based caveats, it requires a different data model (caveated tuples) and its consistency/caching tradeoffs (ZedTokens) are more complex than in-process state. A custom RBAC engine (Alternative 3) lacks a formal policy language, making future ABAC support a retrofit rather than a natural extension. The separate PlatformRoleBinding type (Alternative 4) was rejected because cross-namespace operations (e.g., granting a user cluster-admin in all namespaces) are handled via the private API and kube RBAC — there is no customer-facing need for a cluster-scoped binding resource, and introducing one would add a CRD, a permissions set (`platformrolebinding.*`), and a `platform-admin` role without a corresponding use case.

## Consequences

### Positive

* Zero external service dependency on the authorization hot path — Cedar evaluation is a pure function call in the same process
* Cedar conditions on RoleBindings enable attribute-based access control (e.g., `when` clauses referencing resource attributes like region or labels) for single-resource operations and list item filtering when the implementation supplies the correct object context
* `forbid`-overrides-`permit` semantics provide architectural support for future platform-level restrictions that user-defined roles cannot circumvent (though forbid policies are not implemented in the initial release)
* Per-binding policy isolation (PoC-validated) prevents unconditional bindings from leaking into conditioned policies for the same role
* System role definitions are versioned in Git as Helm templates, reviewed via PR, and deployed via the existing ArgoCD workflow
* Entity cache with per-user dirty invalidation minimizes database queries (most requests served from cache)
* Cross-namespace list authorization is handled at the database level (`WHERE namespace IN UNNEST(...)` in Spanner, `WHERE namespace = ANY($N)` in PostgreSQL) — no over-fetching for namespace filtering, with per-item condition evaluation applied afterward

### Negative

* Cedar is a newer policy language with a smaller ecosystem than OPA/Rego or SpiceDB's Zanzibar model — fewer community resources and third-party integrations
* The `cedar-go` library is a dependency that must be tracked for security updates
* System role changes require a Helm deployment (via ArgoCD). User-defined roles are modified via the public API without redeployment.
* Role and RoleBinding writes depend on the configured leader region. Follower regions maintain local read-only mirrors through the separate [cross-region resource replication](../infrastructure/cross-region-resource-replication.md) mechanism with eventual consistency guarantees
* ABAC conditions require authorization middleware to fetch stored objects for `GET`, `DELETE`, and some update operations before evaluating Cedar policies, increasing implementation complexity and potentially adding latency (one extra database read per conditioned authorization check)
* Policy and entity cache staleness can occur during ResourceStore watch failures or cross-region replication lag, bounded by watch retry/recovery time and optional periodic resync intervals
* Email-based principal keys carry case-sensitivity risks (email normalization must match exactly between RoleBinding subjects and request principals) and account reassignment risks (if a Google account email is reassigned, the new account holder inherits all existing RoleBindings for that email)
* Cedar condition authoring is error-prone without schema validation or guardrails — authors must manually ensure `context.resourcePlural` guards are used when conditions reference resource-type-specific attributes, and missing attribute paths fail evaluation rather than gracefully defaulting

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Cedar evaluation is stateless and CPU-bound (microseconds per decision). The entity cache is per-instance (1000-entry LRU); horizontal scaling adds more cache capacity. Database load is proportional to cache misses, which occur only on first access per user or after binding changes, plus ABAC object reads for conditioned GET/DELETE/PATCH operations.
* **Observability**: Authorization decisions (allow/deny) are logged with the action, resource, and determining policy. Cedar evaluation latency is trackable via existing request metrics. Cache hit/miss rates are observable. Metrics track policy reload success/failure, last successful reload timestamp, active policy count, cache invalidations, and authorization denials by reason. Logs avoid leaking sensitive resource specs or user-provided condition contents except at debug levels with appropriate redaction.
* **Resiliency**: Authorization is fully local to each Gecko instance — no cross-region or cross-service dependency. If the database is temporarily unavailable, cached entity graphs continue serving authorization decisions for known users. New users who have never been seen will get 403 until the database is reachable (fail-closed). Policy reload failures after initial startup retain the last-known-good policy set and retry with backoff; if no valid policy set exists, the server fails closed. Optional periodic policy/cache resync provides a safety backstop against missed watch events.

### Security:

* Default-deny: a user with valid authentication but no role bindings gets 403 on every operation
* Header spoofing protection: the public API backend listener must only be reachable through the ESPv2 sidecar proxy. Direct external access to the application container/port must be blocked by network topology. The `X-Endpoint-API-UserInfo` header is trusted only when received from ESPv2.
* Email claim validation: authentication requires non-empty `email` claim and `email_verified == true` when available from ESPv2. Email normalization (NFC Unicode normalization, lowercase domain, preserved local-part case) must match exactly between RoleBinding subject validation and request principal extraction.
* Object-state ABAC: conditioned authorization evaluates Cedar policies against effective resource state (decoded request body for creates, stored object for reads/deletes, post-update state for updates). If required context cannot be built, authorization fails closed with 403.
* Per-binding policy isolation: Cedar policies are generated per-binding (not per-role), with entity keys including the binding name (`ns/roleName/bindingName`). This prevents an unconditional binding for User A from satisfying the conditioned policy of User B's binding to the same role.
* Cedar condition validation at admission: conditions on RoleBindings are parsed as Cedar policies at creation time; syntactically invalid conditions are rejected. References to `Namespace::` entities within conditions are rejected to prevent namespace traversal.
* Namespace pinning for user-defined roles: generated policies constrain both the principal (`principal in NamespaceRole::"ns/..."`) and the resource (`resource in Namespace::"ns"`) to the same namespace.
* PlatformRoles are not exposed via the public API — no `platformrole.*` permissions exist. PlatformRole CRUD is exclusively via the private API (kube-apiserver + kube RBAC).
* Cross-namespace operations are handled via the private API and kube RBAC, not through the public API.
* Service-admin role grants can intentionally escalate privileges within a namespace: a `service-admin` can bind broader infrastructure roles (e.g., `cluster-admin`) to themselves or other users. This is namespace-local privilege escalation by design to support bootstrap and Marketplace flows.

### Performance:

* Cedar policy evaluation: sub-millisecond per request (pure in-memory computation against cached entity graph). The `atomic.Pointer[cedar.PolicySet]` ensures lock-free reads during authorization while allowing hot-reload.
* Total authorization latency includes Cedar evaluation plus object reads for conditioned GET/DELETE/PATCH operations — add one database read per conditioned check for those operations.
* Entity cache: populated on first access per user, served from memory on subsequent requests, invalidated per-user on binding writes and broadly on role mutations
* Cross-namespace list queries: database-level namespace filtering — no over-fetching for namespace discovery, with per-item condition evaluation applied to filtered results. Pagination works natively at the database layer.

### Cost:

* No additional infrastructure cost — Cedar runs in-process within the existing `platform-api-server` deployment
* No per-request cost (unlike external authorization services that may charge per check)
* Database cost for role/binding storage is negligible (small records, infrequent writes)

### Operability:

* System roles are Helm chart templates deployed via ArgoCD, identical across regions. No extra controller or ConfigMap reconciliation.
* `--disable-auth` flag skips both authn and authz middleware for local development (public API only; private API auth-disable semantics are covered by the [gecko-api-aggregation](gecko-api-aggregation.md) decision). A separate development authentication mode using `X-Dev-User` header may be provided for local testing with authorization still active.
* PlatformRole CRUD, system-level RoleBinding management, and any cross-namespace operations are available through the private API via kubectl
* Runbooks cover policy reload failures (continue with last-known-good, alert, retry with backoff) and cross-region authorization lag (bounded by replication Pub/Sub latency plus watch delivery, typically sub-second)
