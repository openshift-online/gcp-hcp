# Cedar-Based Authorization for Gecko Public API

## Overview

This document specifies the Cedar-based authorization system for Gecko's public (customer-facing) API. It defines the permission model, Cedar entity hierarchy, role definitions, authorization flow, and regionalization strategy.

This implements the architecture decided in [cedar-public-api-authorization](../design-decisions/identity/cedar-public-api-authorization.md). Work is tracked under [GCP-339](https://redhat.atlassian.net/browse/GCP-339) (Epic).

**Repository**: [gecko](https://github.com/openshift-online/gecko)

---

## Design Decisions

| Decision | Choice |
|---|---|
| User identity source | `X-Endpoint-API-UserInfo` header (base64 JWT claims injected by ESPv2 sidecar) |
| Principal key | Email claim (e.g., `User::"alice@example.com"`) |
| Role definition source | Database (system roles seeded by role-seeder controller via private API; user-defined roles created via public API) |
| Cedar dependency | `platform-api` module only (via `github.com/cedar-policy/cedar-go`) |
| Namespace lifecycle | Implicit (no Namespace resource needed) |
| Storage | Same Spanner database as Clusters/NodePools |
| Entity cache | Cache until dirty — invalidate on RoleBinding/PlatformRoleBinding writes and on Role mutations |
| Auth disable flag | Existing `--disable-auth` covers both private and public APIs; startup rejects `--disable-auth` combined with a non-loopback bind address |
| Cross-namespace list | Namespace-filter via `ListOptions.Namespaces` in storage layer (database-level filtering) |

---

## Service Enablement

When a customer enables or disables the GCP HCP service via Google Cloud Marketplace:

**Enable event**: The Marketplace Handler fans out a namespace-level `ServiceAdmin` binding to all regions:

1. Marketplace sends a Pub/Sub `enable` event to a known topic
2. A Marketplace Handler reacts to the event
3. The handler creates the customer's Google identity as `ServiceAdmin` in the corresponding Gecko project namespace — **for each region**

```json
POST /api/v1/namespaces/<project_id>/rolebindings
{
    "subject": "customer@example.com",
    "roleRef": "service-admin"
}
```

**Disable event**: The Marketplace Handler revokes the `ServiceAdmin` binding in all regions:

1. Marketplace sends a Pub/Sub `disable` event
2. The handler deletes the customer's `service-admin` RoleBinding from every active region
3. The per-user entity cache is invalidated in each region so subsequent requests immediately receive 403

Each region hosts an independent Gecko instance with its own Spanner database. Namespace-level bindings (cluster-admin, cluster-viewer) are regional — created in the region where the clusters live. Async replication via Pub/Sub + Spanner Change Streams is a future upgrade path.

---

## Granular Permissions

Every API operation maps to a single granular permission. Permissions follow the pattern `{resource}.{verb}` and each maps to a PascalCase Cedar action.

| Permission | Cedar Action | Scope |
|---|---|---|
| `cluster.create` | `CreateCluster` | Namespace |
| `cluster.list` | `ListClusters` | Namespace |
| `cluster.get` | `GetCluster` | Namespace |
| `cluster.update` | `UpdateCluster` | Namespace |
| `cluster.delete` | `DeleteCluster` | Namespace |
| `nodepool.create` | `CreateNodepool` | Namespace |
| `nodepool.list` | `ListNodepools` | Namespace |
| `nodepool.get` | `GetNodepool` | Namespace |
| `nodepool.update` | `UpdateNodepool` | Namespace |
| `nodepool.delete` | `DeleteNodepool` | Namespace |
| `rolebinding.create` | `CreateRoleBinding` | Namespace |
| `rolebinding.list` | `ListRoleBindings` | Namespace |
| `rolebinding.get` | `GetRoleBinding` | Namespace |
| `rolebinding.update` | `UpdateRoleBinding` | Namespace |
| `rolebinding.delete` | `DeleteRoleBinding` | Namespace |
| `platformrolebinding.create` | `CreatePlatformRoleBinding` | Platform |
| `platformrolebinding.list` | `ListPlatformRoleBindings` | Platform |
| `platformrolebinding.get` | `GetPlatformRoleBinding` | Platform |
| `platformrolebinding.update` | `UpdatePlatformRoleBinding` | Platform |
| `platformrolebinding.delete` | `DeletePlatformRoleBinding` | Platform |
| `role.create` | `CreateRole` | Namespace |
| `role.list` | `ListRoles` | Namespace |
| `role.get` | `GetRole` | Namespace |
| `role.update` | `UpdateRole` | Namespace |
| `role.delete` | `DeleteRole` | Namespace |

---

## Roles

Roles are API resources stored in the database. Each role has a `system` flag:

- **System roles** (`system: true`): Seeded by the role-seeder controller from a ConfigMap via the private API. Immutable via the public API — mutations are rejected with 403. The ConfigMap is versioned in Git alongside the Helm chart and reviewed via PR.
- **User-defined roles** (`system: false`): Created by service-admins via the public API within a namespace. Support Cedar conditions for attribute-based access control (e.g., region-scoped read access).

| Role | Scope | Permissions | System |
|---|---|---|---|
| `platform-admin` | Platform | `platformrolebinding.*` | `true` |
| `service-admin` | Namespace | `rolebinding.*`, `role.*` | `true` |
| `cluster-admin` | Namespace | `cluster.*`, `nodepool.*` | `true` |
| `cluster-viewer` | Namespace | `cluster.list`, `cluster.get`, `nodepool.list`, `nodepool.get` | `true` |

**Separation of concerns**: Access management (platform-admin, service-admin) is fully separated from infrastructure management (cluster-admin, cluster-viewer). No single role conflates both. A full operator needs multiple bindings.

**Grant constraints for any principal with `rolebinding.*`**: These restrictions apply to every principal whose effective permissions include `rolebinding.*` — whether they hold `service-admin` directly or hold a user-defined role (a `Role` with `system: false`) that grants `rolebinding.*`. The RoleBinding validator enforces:

- `roleRef` must reference a namespace-scoped access-management role. Infrastructure roles (`cluster-admin`, `cluster-viewer`) are explicitly rejected as valid `roleRef` values in a `RoleBinding`, regardless of the caller's role. The validator implements this by categorizing roles at policy generation time into **access-management roles** (those whose permissions are drawn exclusively from `{rolebinding,role}.*`) and **infrastructure roles** (those with `{cluster,nodepool}.*` permissions), then rejecting any `roleRef` that resolves to an infrastructure role.
- Self-grant is rejected: a principal may not create or update a `RoleBinding` whose `subject` matches the caller's own identity.

Both constraints apply whether the caller is `service-admin` or a user-defined-role-bearing principal with `rolebinding.*` permissions.

Similarly, the Role validator enforces an infrastructure permission allow-list: **read-only** infrastructure permissions (`cluster.list`, `cluster.get`, `nodepool.list`, `nodepool.get`) are allowed in a user-defined role; **write/delete** infrastructure permissions (`cluster.create`, `cluster.update`, `cluster.delete`, `nodepool.create`, `nodepool.update`, `nodepool.delete`) are rejected. This prevents a service-admin from creating a user-defined role that grants `cluster-admin`-equivalent write access while still enabling the primary ABAC use case: attribute-scoped read access (e.g., view clusters in a specific region).

### Seed Config Format

The role-seeder controller reads its desired state from a ConfigMap. This ConfigMap is the **seed** — the database is the authoritative source of truth after reconciliation.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gecko-authz-config
  namespace: gecko-system
data:
  roles.yaml: |
    roles:
      - name: cluster-viewer
        scope: namespace
        system: true
        permissions:
          - cluster.list
          - cluster.get
          - nodepool.list
          - nodepool.get
      - name: cluster-admin
        scope: namespace
        system: true
        permissions:
          - cluster.create
          - cluster.list
          - cluster.get
          - cluster.update
          - cluster.delete
          - nodepool.create
          - nodepool.list
          - nodepool.get
          - nodepool.update
          - nodepool.delete
      - name: service-admin
        scope: namespace
        system: true
        permissions:
          - rolebinding.create
          - rolebinding.list
          - rolebinding.get
          - rolebinding.update
          - rolebinding.delete
          - role.create
          - role.list
          - role.get
          - role.update
          - role.delete
      - name: platform-admin
        scope: platform
        system: true
        permissions:
          - platformrolebinding.create
          - platformrolebinding.list
          - platformrolebinding.get
          - platformrolebinding.update
          - platformrolebinding.delete

  bootstrap.yaml: |
    platformRoleBindings:
      - name: bootstrap-admin
        subject: operator@example.com
        roleRef: platform-admin
```

### Role-Seeder Controller

The role-seeder is a Go controller in the gecko controller image, using `controller-runtime`, that reconciles the `gecko-authz-config` ConfigMap into Role and PlatformRoleBinding resources via the private API.

**Authentication**: Kubernetes ServiceAccount with projected SA tokens (auto-rotated, short-lived, audience-bound). No manual credential management.

**Authorization**: A `ClusterRole` granting CRUD on `roles` and `platformrolebindings` in the gecko API group, bound to the controller's ServiceAccount.

**Reconciliation behavior**:

1. Watches the `gecko-authz-config` ConfigMap in its namespace
2. On change (or on startup), parses `roles.yaml` and `bootstrap.yaml`
3. For each role in `roles.yaml`: creates the Role if missing, updates if the spec differs, deletes if removed from the ConfigMap. Only touches roles with `system: true`.
4. For each binding in `bootstrap.yaml`: creates the PlatformRoleBinding if missing (idempotent). Does not delete bootstrap bindings on removal from the ConfigMap (safety measure — explicit deletion via kubectl is required).
5. User-defined roles (`system: false`) are never touched by the controller.

**Startup sequence**: No ordering dependency with `platform-api-server`. The controller reconciles asynchronously. Until reconciliation completes on a fresh deployment, all public API requests receive 403 — this is correct default-deny behavior. The reconciliation window is seconds, not minutes.

**Failure mode**: If the private API is unavailable (kube-apiserver down), the controller retries via its standard reconcile loop with exponential backoff. Existing roles in the database continue to function — the policy set is already loaded in `platform-api-server` from the previous startup.

---

## Cedar Entity Model

```cedarschema
namespace Gecko {
  entity User;
  entity Namespace;
  entity Platform;
  entity NamespaceRole in Namespace;
  entity PlatformRole  in Platform;
  entity Cluster       in Namespace;
  entity NodePool      in [Namespace, Cluster];

  action CreateCluster  appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action ListClusters   appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action GetCluster     appliesTo { principal: [User, NamespaceRole], resource: [Namespace, Cluster] };
  action UpdateCluster  appliesTo { principal: [User, NamespaceRole], resource: Cluster };
  action DeleteCluster  appliesTo { principal: [User, NamespaceRole], resource: Cluster };

  action CreateNodepool appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action ListNodepools  appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action GetNodepool    appliesTo { principal: [User, NamespaceRole], resource: [Namespace, NodePool] };
  action UpdateNodepool appliesTo { principal: [User, NamespaceRole], resource: NodePool };
  action DeleteNodepool appliesTo { principal: [User, NamespaceRole], resource: NodePool };

  action CreateRoleBinding appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action ListRoleBindings  appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action GetRoleBinding    appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action UpdateRoleBinding appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action DeleteRoleBinding appliesTo { principal: [User, NamespaceRole], resource: Namespace };

  action CreatePlatformRoleBinding appliesTo { principal: [User, PlatformRole], resource: Platform };
  action ListPlatformRoleBindings  appliesTo { principal: [User, PlatformRole], resource: Platform };
  action GetPlatformRoleBinding    appliesTo { principal: [User, PlatformRole], resource: Platform };
  action UpdatePlatformRoleBinding appliesTo { principal: [User, PlatformRole], resource: Platform };
  action DeletePlatformRoleBinding appliesTo { principal: [User, PlatformRole], resource: Platform };

  action CreateRole appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action ListRoles  appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action GetRole    appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action UpdateRole appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action DeleteRole appliesTo { principal: [User, NamespaceRole], resource: Namespace };
}
```

The schema is documentation only — it is not enforced at runtime. The Go code is responsible for constructing correctly-typed entities.

---

## Cedar Policy Generation

Each role in the database is translated into a Cedar `permit` policy. The policy set is built at startup from all roles in the database and rebuilt on Role resource changes (hot-reload). Permission names map to Cedar actions via the explicit permission-to-action table in the [Granular Permissions](#granular-permissions) section above (e.g., `cluster.create` -> `CreateCluster`). Unknown permission names are rejected at policy generation time.

**Hot-reload**: `platform-api-server` watches its own Role resources via the ResourceStore (or Spanner Change Stream) and regenerates the Cedar policy set on create, update, or delete. This is necessary for both system role updates (deployed via the role-seeder controller) and user-defined role changes (created via the public API). The policy set swap is atomic — the old set continues serving requests until the new one is fully built.

For a namespace-scoped role:

```cedar
// System role: cluster-viewer
permit (
    principal,
    action in [Action::"ListClusters", Action::"GetCluster",
               Action::"ListNodepools", Action::"GetNodepool"],
    resource
)
when { principal in resource };
```

For a platform-scoped role:

```cedar
// System role: platform-admin
permit (
    principal,
    action in [Action::"CreatePlatformRoleBinding", Action::"ListPlatformRoleBindings",
               Action::"GetPlatformRoleBinding", Action::"UpdatePlatformRoleBinding",
               Action::"DeletePlatformRoleBinding"],
    resource
)
when { principal in resource };
```

The `when { principal in resource }` condition is what enforces scoping: the entity graph places users `in` their bound roles, roles `in` their namespace/platform, and resources `in` their namespace. Cedar's transitive `in` operator handles the rest.

In addition to `permit` policies, the policy set includes platform-level `forbid` policies for any actions that must be universally denied regardless of role bindings. A matching `forbid` overrides any matching `permit` — Cedar evaluates all applicable policies and a single matching `forbid` produces a Deny regardless of how many `permit` policies also match.

---

## API Resources

| Resource | Scope | Purpose |
|---|---|---|
| `Role` | Namespace or Platform | Defines a set of permissions. Scope is per-role (most system roles are namespace-scoped; `platform-admin` is platform-scoped). System roles (`system: true`) are seeded by the role-seeder controller and immutable via the public API. User-defined roles (`system: false`) are created by service-admins within a namespace. |
| `RoleBinding` | Namespaced | Binds a user email to a role within a namespace |
| `PlatformRoleBinding` | Cluster-scoped | Binds a user email to a platform-scoped role |

**Dual API surface for Roles**:

- **Private API** (kube-apiserver, kube RBAC): Full CRUD on all roles — used by the role-seeder controller and SRE tooling (`kubectl`).
- **Public API** (Cedar-authorized): Read-only for system roles (`system: true`). Full CRUD for user-defined roles (`system: false`) within the caller's authorized namespaces, requiring `role.*` permissions. Mutations to system roles return 403.

---

## Authorization Flow

Health probes (`/healthz`, `/readyz`) are registered outside the middleware chain and bypass both authentication and authorization.

**ESPv2 trust boundary**: `platform-api-server` binds its public API listener to loopback (`127.0.0.1`) so that only the ESPv2 sidecar can reach it. ESPv2 validates the JWT (issuer, audience, expiry, signature) before injecting `X-Endpoint-API-UserInfo` and strips any pre-existing value from inbound requests, preventing header forgery by direct callers.

```text
HTTP Request
  |
  +- /healthz, /readyz -> bypass (no authn/authz)
  |
  +- 1. AuthN Middleware
  |     - Read X-Endpoint-API-UserInfo header (base64 JWT claims from ESPv2)
  |     - Decode claims, extract email field
  |     - Inject email into request context
  |     - Missing/malformed header -> 401 Unauthenticated
  |
  +- 2. AuthZ Middleware (Cedar)
  |     a. Read user email from context
  |     b. Derive Cedar Action from HTTP method + chi route pattern
  |     c. Derive Cedar Resource from URL path parameters
  |     d. Build Entity Slice (from cache, or from DB if dirty):
  |        - User entity + parent NamespaceRoles (from RoleBindings in DB)
  |        - User entity + parent PlatformRoles (from PlatformRoleBindings in DB)
  |        - NamespaceRole entities + parent Namespace entities
  |        - Resource entity + parent hierarchy
  |     e. cedar.Authorize(policySet, entitySlice, request)
  |     f. Deny -> 403 Forbidden
  |
  +- 3. Handler: normal CRUD processing
```

---

## Action Derivation Map

| HTTP Method | URL Pattern | Cedar Action | AuthZ Strategy |
|---|---|---|---|
| GET | `/namespaces/{ns}/clusters` | `ListClusters` | Pre-filter |
| POST | `/namespaces/{ns}/clusters` | `CreateCluster` | Pre-filter |
| GET | `/namespaces/{ns}/clusters/{name}` | `GetCluster` | Pre-filter |
| PUT/PATCH | `/namespaces/{ns}/clusters/{name}` | `UpdateCluster` | Pre-filter |
| DELETE | `/namespaces/{ns}/clusters/{name}` | `DeleteCluster` | Pre-filter |
| GET | `/clusters` | `ListClusters` | **Namespace-filter** |
| GET | `/namespaces/{ns}/nodepools` | `ListNodepools` | Pre-filter |
| POST | `/namespaces/{ns}/nodepools` | `CreateNodepool` | Pre-filter |
| GET | `/namespaces/{ns}/nodepools/{name}` | `GetNodepool` | Pre-filter |
| PUT/PATCH | `/namespaces/{ns}/nodepools/{name}` | `UpdateNodepool` | Pre-filter |
| DELETE | `/namespaces/{ns}/nodepools/{name}` | `DeleteNodepool` | Pre-filter |
| GET | `/nodepools` | `ListNodepools` | **Namespace-filter** |
| GET | `/namespaces/{ns}/rolebindings` | `ListRoleBindings` | Pre-filter |
| POST | `/namespaces/{ns}/rolebindings` | `CreateRoleBinding` | Pre-filter |
| GET | `/namespaces/{ns}/rolebindings/{name}` | `GetRoleBinding` | Pre-filter |
| PUT/PATCH | `/namespaces/{ns}/rolebindings/{name}` | `UpdateRoleBinding` | Pre-filter |
| DELETE | `/namespaces/{ns}/rolebindings/{name}` | `DeleteRoleBinding` | Pre-filter |
| GET | `/rolebindings` | `ListRoleBindings` | **Namespace-filter** |
| GET | `/platformrolebindings` | `ListPlatformRoleBindings` | Pre-filter |
| POST | `/platformrolebindings` | `CreatePlatformRoleBinding` | Pre-filter |
| GET | `/platformrolebindings/{name}` | `GetPlatformRoleBinding` | Pre-filter |
| PUT/PATCH | `/platformrolebindings/{name}` | `UpdatePlatformRoleBinding` | Pre-filter |
| DELETE | `/platformrolebindings/{name}` | `DeletePlatformRoleBinding` | Pre-filter |
| GET | `/namespaces/{ns}/roles` | `ListRoles` | Pre-filter |
| POST | `/namespaces/{ns}/roles` | `CreateRole` | Pre-filter |
| GET | `/namespaces/{ns}/roles/{name}` | `GetRole` | Pre-filter |
| PUT/PATCH | `/namespaces/{ns}/roles/{name}` | `UpdateRole` | Pre-filter |
| DELETE | `/namespaces/{ns}/roles/{name}` | `DeleteRole` | Pre-filter |
| GET | `/roles` | `ListRoles` | **Namespace-filter** |

---

## Cross-Namespace List Authorization

When a list request has no namespace in the URL (e.g., `GET /clusters`), the authz middleware:

1. Detects the cross-namespace list (namespaced resource, no namespace param, GET method)
2. Pre-computes the authorized namespace set from the user's RoleBindings **whose role grants the requested action's permission** — namespaces where the user only holds unrelated roles (e.g., service-admin for a cluster list request) are excluded
3. Injects the set into the request context
4. The handler reads authorized namespaces from context and sets `ListOptions.Namespaces`
5. The storage layer filters at the database level: `WHERE namespace IN UNNEST(...)` (Spanner) or `WHERE namespace = ANY($N)` (Postgres)

A cross-namespace list **never returns 403**. If the user has no bindings, the namespace set is empty and the query returns an empty list.

---

## Entity Cache Strategy

The entity graph (user -> roles -> namespaces) is cached in memory and invalidated on writes:

- **Cache key**: user email
- **Population**: on first authorization check for a user, query RoleBindings and PlatformRoleBindings, build the entity graph, cache it
- **Invalidation**: per-subject. When a binding is written, only the affected user's cache entry is evicted. If the subject changes during an update, both old and new subjects are invalidated.
- **Role changes**: When a Role is created, updated, or deleted, regenerate the Cedar policy set. On update/delete, invalidate all users bound to the affected role (query RoleBindings by roleRef). Role changes are infrequent, so the broader invalidation is acceptable.
- **Multi-instance**: Spanner Change Streams provide cross-instance invalidation — the notification payload includes the affected subject.

---

## User-Defined Roles

Service-admins can create namespace-scoped user-defined roles with Cedar conditions:

```yaml
apiVersion: gcp.managed.openshift.io/v1
kind: Role
metadata:
  name: us-east-cluster-viewer
  namespace: project-a
spec:
  system: false
  permissions:
    - cluster.list
    - cluster.get
  condition: 'resource.region == "us-east1"'
```

The `condition` field is a **Cedar expression body** (not a full `when` clause). The policy generator wraps it inside a `when { ... }` block alongside the mandatory namespace constraints. Storing a bare expression body avoids invalid nested `when` syntax during generation.

Generated Cedar policy is **namespace-pinned**, restricting both the principal and the resource to the namespace:

```cedar
permit (
    principal,
    action in [Action::"ListClusters", Action::"GetCluster"],
    resource
)
when {
    principal in Namespace::"project-a" &&
    resource in Namespace::"project-a" &&
    resource.region == "us-east1"
};
```

Condition validation uses Cedar AST inspection (not string matching) to reject cross-namespace escalation and enforce an attribute allow-list.

**Grant constraint propagation through user-defined roles**: A user-defined role may include `rolebinding.*` permissions, making a principal bound to that role a de-facto grant manager. The same RoleBinding grant constraints that apply to `service-admin` apply to any such principal:

- The RoleBinding validator rejects `roleRef` values that resolve to infrastructure roles (`cluster-admin`, `cluster-viewer`), regardless of whether the caller's grant authority comes from the built-in `service-admin` or from a user-defined role.
- Self-grant rejection applies equally.

Required test coverage:
- A principal bound to a user-defined role with `rolebinding.*` is rejected when trying to bind an infrastructure roleRef (e.g., `cluster-admin`)
- A principal bound to a user-defined role with `rolebinding.*` is rejected when self-granting any role
- A user-defined role that includes infrastructure write/delete permissions (`cluster.create`, `cluster.update`, `cluster.delete`, `nodepool.create`, `nodepool.update`, `nodepool.delete`) is rejected at creation time by the Role validator
- A user-defined role with infrastructure read-only permissions (`cluster.list`, `cluster.get`, `nodepool.list`, `nodepool.get`) is accepted (primary ABAC use case)
- Mutations to system roles (`system: true`) via the public API return 403

---

## Regionalization Strategy

Each region runs an independent Gecko instance with its own Spanner database. The authorization data strategy has two tiers:

**Tier 1 (Baseline)**: The Marketplace Handler fans out namespace-level `ServiceAdmin` bindings to all regions at service enablement time (and revokes them on disable). The role-seeder controller runs in each region and reconciles system roles independently. Namespace-level bindings for infrastructure roles (cluster-admin, cluster-viewer) are regional — created in the region where the clusters live.

**Tier 2 (Future, when needed)**: Async replication of bindings across regions via Pub/Sub. Local Gecko instances publish binding mutations to a global Pub/Sub topic. Regional instances subscribe and replay mutations into their local Spanner. The existing Spanner Change Stream broadcaster can serve as the source of replication events.

Triggers for Tier 2: ServiceAdmins needing bindings to apply globally, or user-defined roles requiring cross-region consistency.

---

## File Structure

```text
platform-api/
  api/private/v1/
    role_types.go                            # Role type (both scopes, system flag)
    rolebinding_types.go                     # RoleBinding type (namespaced)
    platformrolebinding_types.go             # PlatformRoleBinding type (cluster-scoped)
  api/public/v1/
    zz_generated.role_types.go               # (generated by orlop-gen)
    zz_generated.rolebinding_types.go        # (generated by orlop-gen)
    zz_generated.platformrolebinding_types.go
    zz_generated.conversion.go               # (regenerated)
    zz_generated.schemas.go                  # (regenerated)
  pkg/
    authn/
      middleware.go                          # X-Endpoint-API-UserInfo extraction
      middleware_test.go
    authz/
      authorizer.go                          # Cedar PolicySet + Authorize()
      entities.go                            # EntityGetter backed by RoleBinding stores
      cache.go                               # Entity cache with dirty invalidation
      middleware.go                           # HTTP authz middleware
      policygen.go                           # Cedar policy generation from role definitions
      reload.go                              # Policy set hot-reload on Role changes
      validator.go                           # RoleRef + Role validation
      authorizer_test.go
      middleware_test.go
      entities_test.go
      cache_test.go
      policygen_test.go
      reload_test.go
      validator_test.go
  cmd/platform-api-server/
    main.go                                  # Wire authn/authz middleware, load roles from DB
controllers/
  role-seeder/
    controller.go                            # Reconciles ConfigMap -> Role + PlatformRoleBinding
    controller_test.go
deploy/
  gecko-authz-config.yaml                    # Seed config for role-seeder controller
```
