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
| Role definition source | ConfigMap loaded at deployment time (not an API resource) |
| Cedar dependency | `platform-api` module only (via `github.com/cedar-policy/cedar-go`) |
| Namespace lifecycle | Implicit (no Namespace resource needed) |
| Storage | Same Spanner database as Clusters/NodePools |
| Entity cache | Cache until dirty — invalidate on RoleBinding/PlatformRoleBinding writes |
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
| `customrole.create` | `CreateCustomRole` | Namespace |
| `customrole.list` | `ListCustomRoles` | Namespace |
| `customrole.get` | `GetCustomRole` | Namespace |
| `customrole.update` | `UpdateCustomRole` | Namespace |
| `customrole.delete` | `DeleteCustomRole` | Namespace |

---

## Built-in Roles

Roles are defined in a ConfigMap loaded at deployment time. They are **not** API resources — they cannot be created, modified, or deleted via the API. The ConfigMap is versioned in Git alongside the Helm chart.

| Role | Scope | Permissions |
|---|---|---|
| `platform-admin` | Platform | `platformrolebinding.*` |
| `service-admin` | Namespace | `rolebinding.*`, `customrole.*` |
| `cluster-admin` | Namespace | `cluster.*`, `nodepool.*` |
| `cluster-viewer` | Namespace | `cluster.list`, `cluster.get`, `nodepool.list`, `nodepool.get` |

**Separation of concerns**: Access management (platform-admin, service-admin) is fully separated from infrastructure management (cluster-admin, cluster-viewer). No single role conflates both. A full operator needs multiple bindings.

**Grant constraints for `service-admin`**: Although `service-admin` holds `rolebinding.*` permissions, the RoleBinding validator enforces the following restrictions to prevent privilege escalation:

- A `service-admin` may only create RoleBindings whose `roleRef` is a namespace-scoped access-management role (i.e., `service-admin` or a user-defined `CustomRole`). Referencing infrastructure roles (`cluster-admin`, `cluster-viewer`) is rejected.
- A `service-admin` may not bind any role to their own subject via a RoleBinding they create (self-grant is rejected).

Similarly, the CustomRole validator (Story 3) rejects `CustomRole` definitions that include infrastructure permissions (`cluster.*`, `nodepool.*`) — custom roles are limited to access-management permissions within the namespace.

### ConfigMap Format

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
        permissions:
          - cluster.list
          - cluster.get
          - nodepool.list
          - nodepool.get
      - name: cluster-admin
        scope: namespace
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
        permissions:
          - rolebinding.create
          - rolebinding.list
          - rolebinding.get
          - rolebinding.update
          - rolebinding.delete
          - customrole.create
          - customrole.list
          - customrole.get
          - customrole.update
          - customrole.delete
      - name: platform-admin
        scope: platform
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

On startup, the server reads the ConfigMap, generates Cedar policies from the role definitions, and upserts bootstrap bindings into the store (idempotent).

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

  action CreateCustomRole appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action ListCustomRoles  appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action GetCustomRole    appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action UpdateCustomRole appliesTo { principal: [User, NamespaceRole], resource: Namespace };
  action DeleteCustomRole appliesTo { principal: [User, NamespaceRole], resource: Namespace };
}
```

The schema is documentation only — it is not enforced at runtime. The Go code is responsible for constructing correctly-typed entities.

---

## Cedar Policy Generation

Each role in the ConfigMap is translated into a Cedar `permit` policy at startup. Permission names map to Cedar actions via the explicit permission-to-action table in the [Granular Permissions](#granular-permissions) section above (e.g., `cluster.create` -> `CreateCluster`). Unknown permission names are rejected at startup.

For a namespace-scoped role:

```cedar
// Built-in role: cluster-viewer
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
// Built-in role: platform-admin
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
| `RoleBinding` | Namespaced | Binds a user email to a role within a namespace |
| `PlatformRoleBinding` | Cluster-scoped | Binds a user email to a platform-scoped role |
| `CustomRole` | Namespaced (future) | User-defined role with Cedar conditions |

There is no `Role` API resource. Built-in roles are defined in the ConfigMap and are not exposed as API objects.

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
- **Multi-instance**: Spanner Change Streams provide cross-instance invalidation — the notification payload includes the affected subject.

---

## Custom Roles (Future)

Service-admins will be able to create namespace-scoped custom roles with Cedar conditions:

```yaml
apiVersion: gcp.managed.openshift.io/v1
kind: CustomRole
metadata:
  name: us-east-cluster-viewer
  namespace: project-a
spec:
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

---

## Regionalization Strategy

Each region runs an independent Gecko instance with its own Spanner database. The authorization data strategy has two tiers:

**Tier 1 (Baseline)**: The Marketplace Handler fans out namespace-level `ServiceAdmin` bindings to all regions at service enablement time (and revokes them on disable). Namespace-level bindings for infrastructure roles (cluster-admin, cluster-viewer) are regional — created in the region where the clusters live.

**Tier 2 (Future, when needed)**: Async replication of bindings across regions via Pub/Sub. Local Gecko instances publish binding mutations to a global Pub/Sub topic. Regional instances subscribe and replay mutations into their local Spanner. The existing Spanner Change Stream broadcaster can serve as the source of replication events.

Triggers for Tier 2: ServiceAdmins needing bindings to apply globally, or custom roles requiring cross-region consistency.

---

## File Structure

```text
platform-api/
  api/private/v1/
    rolebinding_types.go                   # RoleBinding type (namespaced)
    platformrolebinding_types.go           # PlatformRoleBinding type (cluster-scoped)
    customrole_types.go                    # CustomRole type (namespaced, future)
  api/public/v1/
    zz_generated.rolebinding_types.go      # (generated by orlop-gen)
    zz_generated.platformrolebinding_types.go
    zz_generated.customrole_types.go       # (future)
    zz_generated.conversion.go             # (regenerated)
    zz_generated.schemas.go                # (regenerated)
  pkg/
    authn/
      middleware.go                        # X-Endpoint-API-UserInfo extraction
      middleware_test.go
    authz/
      authorizer.go                        # Cedar PolicySet + Authorize()
      config.go                            # ConfigMap parsing (roles, bootstrap)
      entities.go                          # EntityGetter backed by RoleBinding stores
      cache.go                             # Entity cache with dirty invalidation
      middleware.go                        # HTTP authz middleware
      policygen.go                         # Cedar policy generation from role definitions
      bootstrap.go                         # Bootstrap loader for initial bindings
      validator.go                         # RoleRef validation
      authorizer_test.go
      config_test.go
      middleware_test.go
      entities_test.go
      cache_test.go
      policygen_test.go
      bootstrap_test.go
  cmd/platform-api-server/
    main.go                                # Wire authn/authz middleware, load config
deploy/
  gecko-authz-config.yaml                  # ConfigMap with built-in roles and bootstrap
```
