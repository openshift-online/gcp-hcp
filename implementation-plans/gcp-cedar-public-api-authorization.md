# Cedar-Based Authorization for Gecko Public API

## Overview

This document specifies the Cedar-based authorization system for Gecko's public (customer-facing) API. It defines the permission model, API resource types, Cedar entity hierarchy, policy generation strategy, authorization flow, and caching/hot-reload mechanisms.

This implements the architecture decided in [cedar-public-api-authorization](../design-decisions/identity/cedar-public-api-authorization.md). Work is tracked under [GCP-339](https://redhat.atlassian.net/browse/GCP-339) (Epic).

**Repository**: [gecko](https://github.com/openshift-online/gecko)

---

## Design Decisions

| Decision | Choice |
|---|---|
| Authorization engine | Cedar (`cedar-go`), in-process evaluation |
| User identity source | `X-Endpoint-API-UserInfo` header (base64 JWT claims injected by ESPv2 sidecar) |
| Trusted proxy boundary | Public API backend listener reachable only through ESPv2 sidecar; direct access blocked by network topology |
| Principal key | Email claim (e.g., `User::"alice@example.com"`) |
| Principal validation | Non-empty `email` required; `email_verified == true` required when available; NFC Unicode normalization + lowercase domain + preserved local-part case |
| API types | `PlatformRole` (cluster-scoped), `Role` (namespaced), `RoleBinding` (namespaced) |
| System role seeding | Helm chart templates for PlatformRole CRDs, deployed via ArgoCD |
| PlatformRole public API | None — no `platformrole.*` permissions exist; CRUD is via private API only |
| Cedar conditions | On `RoleBinding.spec.condition` (not on Role), enabling per-user ABAC |
| Object-state ABAC | Authorization evaluates Cedar against effective resource state: decoded body for creates, stored object for reads/deletes, post-update state for updates |
| Policy generation | Per-binding with explicit namespace pins |
| Grant constraints | Relaxed: no self-grant prevention. `service-admin` can bind infrastructure roles within the namespace (intentional namespace-local privilege escalation). |
| Validation | Referenced role existence, Cedar condition syntax, `Namespace::` traversal rejection, subject normalization |
| Entity model | 3 types: User, NamespaceRole, Namespace |
| Entity cache | 1000-entry LRU, per-user invalidation on binding writes, full invalidation on role changes |
| Cross-namespace list | Namespace-filter via `ListOptions.Namespaces` in storage layer (database-level filtering) + per-item `ItemFilter` for Cedar conditions |
| Multi-resource conditions | `context.resourcePlural` guard required when condition targets a specific resource type (documented authoring requirement) |
| Policy reload failure | Continue with last-known-good policy set, retry with backoff, fail closed if no valid policy exists |
| Auth disable flag | `--disable-auth` skips authn/authz middleware for public API local development; separate dev-header mode may exist for testing with authz active |
| Cross-region consistency | Via separate [cross-region resource replication](gcp-cross-region-resource-replication.md); authorization decisions based on local regional state, may lag during replication outages |

---

## Granular Permissions

Every API operation maps to a single granular permission. Permissions follow the pattern `{resource}.{verb}` and each maps to a PascalCase Cedar action. All permissions are namespace-scoped.

| Permission | Cedar Action |
|---|---|
| `cluster.create` | `CreateCluster` |
| `cluster.list` | `ListClusters` |
| `cluster.get` | `GetCluster` |
| `cluster.update` | `UpdateCluster` |
| `cluster.delete` | `DeleteCluster` |
| `nodepool.create` | `CreateNodepool` |
| `nodepool.list` | `ListNodepools` |
| `nodepool.get` | `GetNodepool` |
| `nodepool.update` | `UpdateNodepool` |
| `nodepool.delete` | `DeleteNodepool` |
| `rolebinding.create` | `CreateRoleBinding` |
| `rolebinding.list` | `ListRoleBindings` |
| `rolebinding.get` | `GetRoleBinding` |
| `rolebinding.update` | `UpdateRoleBinding` |
| `rolebinding.delete` | `DeleteRoleBinding` |
| `role.create` | `CreateRole` |
| `role.list` | `ListRoles` |
| `role.get` | `GetRole` |
| `role.update` | `UpdateRole` |
| `role.delete` | `DeleteRole` |

Unknown permission names are rejected at validation time (Role and PlatformRole creation). Unknown HTTP methods or URL patterns are rejected before Cedar evaluation (fail-closed with 403).

---

## System Roles

System roles are `PlatformRole` resources (cluster-scoped, `+kubebuilder:resource:scope=Cluster`). They are deployed as Helm chart templates via ArgoCD, identical across regions. PlatformRoles have no public API endpoint — there are no `platformrole.*` permissions, so they are naturally immutable from the customer's perspective.

| Role | Permissions |
|---|---|
| `cluster-admin` | `cluster.create`, `cluster.list`, `cluster.get`, `cluster.update`, `cluster.delete`, `nodepool.create`, `nodepool.list`, `nodepool.get`, `nodepool.update`, `nodepool.delete` |
| `cluster-viewer` | `cluster.list`, `cluster.get`, `nodepool.list`, `nodepool.get` |
| `service-admin` | `rolebinding.create`, `rolebinding.list`, `rolebinding.get`, `rolebinding.update`, `rolebinding.delete`, `role.create`, `role.list`, `role.get`, `role.update`, `role.delete` |

**Separation of concerns**: The default system roles separate access-management permissions (`service-admin`) from infrastructure permissions (`cluster-admin`, `cluster-viewer`). No single role conflates both capabilities. However, a `service-admin` can intentionally grant infrastructure roles to themselves or other users within the namespace because self-grant prevention is out of scope. This is namespace-local privilege escalation by design — `service-admin` is a highly privileged role that should be granted only to trusted namespace owners. Bootstrap and Marketplace flows rely on this trust model.

### Helm Template Format

Each system PlatformRole is a Helm template:

```yaml
{{- if .Values.platformRoles.enabled }}
apiVersion: gcp.managed.openshift.io/v1
kind: PlatformRole
metadata:
  name: cluster-viewer
spec:
  permissions:
    - cluster.list
    - cluster.get
    - nodepool.list
    - nodepool.get
{{- end }}
```

PlatformRoles are gated by `.Values.platformRoles.enabled` so they can be disabled in test environments.

---

## API Resources

### PlatformRole

Cluster-scoped. Defines a set of permissions. Managed exclusively via the private API (kube-apiserver + kube RBAC) and Helm.

```go
type PlatformRole struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec PlatformRoleSpec `json:"spec"`
}

type PlatformRoleSpec struct {
    Permissions []string `json:"permissions"`
}
```

### Role

Namespace-scoped. User-defined roles created via the public API by principals with `role.*` permissions. Defines a set of permissions drawn from the valid permission set.

```go
type Role struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec RoleSpec `json:"spec"`
}

type RoleSpec struct {
    Permissions []string `json:"permissions"`
}
```

### RoleBinding

Namespace-scoped. Binds a user email to a PlatformRole or a Role within a namespace. Optionally carries a Cedar condition for ABAC. All RoleBindings of configured resource types are globally replicated across regions (see [cross-region replication plan](gcp-cross-region-resource-replication.md)).

```go
type RoleBinding struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec RoleBindingSpec `json:"spec"`
}

type RoleBindingSpec struct {
    Subject   string  `json:"subject"`             // User email
    RoleRef   RoleRef `json:"roleRef"`              // References PlatformRole or Role
    Condition string  `json:"condition,omitempty"`   // Cedar expression body (optional)
}

type RoleRef struct {
    Kind     string `json:"kind"`     // "PlatformRole" or "Role"
    Name     string `json:"name"`
    APIGroup string `json:"apiGroup"` // "gcp.managed.openshift.io"
}
```

**RoleRef.Kind** distinguishes the binding target:
- `"PlatformRole"` — references a cluster-scoped PlatformRole (e.g., `cluster-admin`, `service-admin`)
- `"Role"` — references a namespace-scoped Role (user-defined)

---

## Authentication Middleware

### Trust Boundary

**Critical security requirement**: The public API backend listener must only be reachable through the ESPv2 sidecar proxy. Direct external access to the application container/port must be blocked by network topology (sidecar-to-app communication over localhost, or equivalent network isolation).

The `X-Endpoint-API-UserInfo` header is trusted **only** when the request arrives through ESPv2. Any user-supplied `X-Endpoint-API-UserInfo` header must be stripped by ESPv2 configuration or rendered impossible by network topology. Failure to enforce this trust boundary allows trivial authentication bypass via header spoofing.

### Production Mode

1. Read the `X-Endpoint-API-UserInfo` header (set by ESPv2 sidecar after JWT validation)
2. Base64-decode (try raw URL encoding first, fall back to padded)
3. Parse JSON and extract claims
4. **Email claim validation**:
   - Require non-empty `email` claim
   - Require `email_verified == true` when the claim is present (ESPv2 forwards this claim from Google ID tokens)
   - If `email_verified` is false or missing when expected, return `401 Unauthorized`
5. **Email normalization** (must match RoleBinding subject normalization exactly):
   - NFC Unicode normalization
   - Lowercase the domain part (after `@`)
   - Preserve local-part case
6. Store normalized email in request context via `authn.WithUser(ctx, email)`
7. **Failure cases return `401 Unauthorized`**:
   - Missing `X-Endpoint-API-UserInfo` header
   - Malformed base64 or JSON
   - Empty or missing `email` claim
   - `email_verified == false` (when claim is present)

### Development Mode (Dev Header)

1. Read the `X-Dev-User` header directly (no JWT validation, no ESPv2)
2. Missing header returns `401 Unauthorized`
3. Normalize email using the same rules as production mode
4. Store in context
5. Authorization middleware remains active (conditions are evaluated)

### Disabled Auth Mode

When `--disable-auth` flag is set:
- Both authentication and authorization middleware are skipped entirely for the public API
- No user identity extraction or policy evaluation occurs
- **Security**: This mode is for local development only. Production deployments must never set this flag.

**Deployment verification**: Startup tests and deployment checks verify that the public API backend listener is not directly reachable externally when `--disable-auth` is not set.

---

## Cedar Entity Model

The entity model uses three types. The Cedar schema below is documentation only — it is not enforced at runtime. The Go code constructs correctly-typed entities.

```cedarschema
entity User;
entity Namespace;
entity NamespaceRole in Namespace;
```

### Entity Construction

For a given user, the `EntityGetter` builds:

1. **`User::"alice@example.com"`** — parents: list of all `NamespaceRole` entities from the user's bindings
2. **`NamespaceRole::"project-a/cluster-admin/marketplace-service-admin"`** — parent: `Namespace::"project-a"`
3. **`Namespace::"project-a"`** — leaf entity, no parents

**Entity key format**: `NamespaceRole` entities use a three-part key: `{namespace}/{roleName}/{bindingName}`. This is critical for per-binding policy isolation.

### Per-Binding Isolation Rationale

Entity keys include the binding name (not just namespace/role) to prevent unconditional bindings from leaking into conditioned policies.

**Example**: User A has an unconditional binding to `cluster-viewer` in `project-a`. User B has a conditioned binding to `cluster-viewer` in `project-a` (only US regions). If the entity key were `project-a/cluster-viewer` (without binding name), User A's entity would match User B's conditioned policy — both would resolve to the same `NamespaceRole` entity. With the binding-name suffix, each binding produces a distinct `NamespaceRole` entity, and policies are generated per-binding with distinct entity references.

---

## Cedar Policy Generation

Each RoleBinding in the database produces a Cedar `permit` policy. The policy set is built at startup from all PlatformRoles, Roles, and RoleBindings, and rebuilt on changes (hot-reload).

### PlatformRole Bindings

For a RoleBinding referencing a PlatformRole:

```cedar
// platformrole:cluster-viewer:binding:project-a/viewer-binding
permit (
    principal,
    action in [Action::"ListClusters", Action::"GetCluster",
               Action::"ListNodepools", Action::"GetNodepool"],
    resource
)
when {
    principal in NamespaceRole::"project-a/cluster-viewer/viewer-binding" &&
    resource in Namespace::"project-a"
};
```

### Namespace-Scoped Role Bindings

For a RoleBinding referencing a user-defined Role:

```cedar
// role:us-east-viewer:binding:project-a/region-binding
permit (
    principal,
    action in [Action::"ListClusters", Action::"GetCluster"],
    resource
)
when {
    principal in NamespaceRole::"project-a/us-east-viewer/region-binding" &&
    resource in Namespace::"project-a" &&
    (!(context has resourceName) || context.spec.platform.gcp.region == "us-east1")
};
```

### Condition Wrapping

When a RoleBinding has a `condition` field, the policy generator wraps it to handle both namespace-level authorization (for list namespace discovery) and object-level authorization (for single-resource operations and per-item filtering):

```
<base_condition> && <user_condition>
```

**Important**: The condition is evaluated in two contexts:
1. **Namespace-level authorization** (list namespace discovery): Cedar context contains only `resourcePlural` and `method`. The user condition must not reference `context.spec` or other object-specific fields in this phase, or use guards like `context has spec` to avoid evaluation errors.
2. **Object-level authorization** (single-resource operations and per-item list filtering): Cedar context contains `resourceName`, `resourcePlural`, `method`, and `spec`. The user condition evaluates against the effective resource state.

The implementation separates these phases:
- List operations first perform namespace-level authorization to determine candidate namespaces (the condition should permit if object context is absent, or use `context has spec` guard).
- Per-item filtering then evaluates the condition against each item's full object context via the `ItemFilter` mechanism.

**Recommended condition pattern for ABAC on resource attributes**:
```cedar
!(context has spec) || context.spec.platform.gcp.region == "us-east1"
```

This permits namespace-level authorization (no `spec` available) and enforces the condition only when object state is available.

### Multi-Resource Condition Guard

When a RoleBinding's condition targets a specific resource type (e.g., cluster attributes) but the referenced role grants permissions on multiple resource types (e.g., `cluster.get` and `nodepool.get`), the condition **must** use `context.resourcePlural` to scope itself:

```cedar
context.resourcePlural != "clusters" || (
    !(context has spec) ||
    (context.spec has platform &&
     context.spec.platform has gcp &&
     context.spec.platform.gcp.region == "us-east1")
)
```

This ensures:
1. The condition is only evaluated when accessing clusters (`context.resourcePlural == "clusters"`)
2. Namespace-level authorization is permitted (`!(context has spec)`)
3. Object-level authorization checks the region attribute with safe path traversal (explicit `has` checks)

Without the `context.resourcePlural` guard, accessing nodepools would spuriously deny because the `spec.platform.gcp.region` path does not exist on nodepool resources.

**Validation and Enforcement**: The `context.resourcePlural` guard requirement is a **documented authoring requirement**. Validation does not structurally enforce it, but tests demonstrate that unsafe conditions (referencing resource-type-specific attributes without the guard) fail closed for unrelated resource types. Condition authors are responsible for following the recommended patterns.

### Policy IDs

Policy IDs are deterministic:
- PlatformRole bindings: `platformrole:<roleName>:binding:<namespace>/<bindingName>`
- Role bindings: `role:<roleName>:binding:<namespace>/<bindingName>`

### Forbid Policies (Future Work)

Cedar's `forbid`-overrides-`permit` semantics provide architectural support for platform-level policies that universally deny certain actions regardless of role bindings. A single matching `forbid` policy produces Deny even if multiple `permit` policies match.

**Status**: Forbid policies are **not implemented in the initial release**. The policy generator emits only `permit` policies derived from PlatformRole and Role bindings. Future platform forbid policies would require:

- Source of truth (Helm-managed config, private API resource, or hardcoded policies)
- Policy ID format and naming convention
- Supported actions and resource patterns
- Hot-reload trigger and deployment path
- Operational ownership and emergency rollout/rollback process
- Precedence and interaction tests with permit policies

Until forbid policies are implemented, the authorization model provides default-deny security (users with no matching permit policies are denied) but does not support platform-enforced prohibitions that override user-defined roles.

### Hot-Reload

`platform-api-server` watches PlatformRole, Role, and RoleBinding resources via the `ResourceStore` watch mechanism. Each API server replica starts its own watcher to ensure all replicas observe policy and binding changes.

#### Watch Setup

- Each replica creates a watch for PlatformRole, Role, and RoleBinding using the shared `ResourceStore` instances
- Watches must deliver events to all replicas, not just the replica that wrote the resource
- Storage backend implementations (Spanner, PostgreSQL) must support global watch delivery across all clients

#### Policy Reload on Resource Change

On create, update, or delete of any watched resource:

1. Load all PlatformRoles, Roles, and RoleBindings from the stores
2. Generate the new `cedar.PolicySet` via `GeneratePolicies()`
3. Swap atomically via `atomic.Pointer[cedar.PolicySet]` — the old set continues serving requests until the swap completes
4. Invalidate the entity cache:
   - PlatformRole or Role change → invalidate all cached entries
   - RoleBinding change → invalidate only the affected user's cache entry (and the previous subject's entry if the subject changed during an update)

#### Watch Failure and Recovery

- **On watch error**: log error, increment metrics, retry with exponential backoff
- **During watch failure**: continue using last-known-good policy set and entity cache
- **Recovery**: when watch reconnects, trigger full policy reload to catch any missed events

#### Periodic Resync Backstop

Optional periodic policy and cache resync provides a safety backstop against missed watch events:

- Configurable via `--authz-resync-interval` (e.g., `5m`, default disabled or `0`)
- On each interval:
  1. Reload all PlatformRoles, Roles, and RoleBindings from stores
  2. Generate new `cedar.PolicySet`
  3. Compare policy set version/hash with current; swap only if changed
  4. Optionally invalidate entity cache (or use cache TTL)
- Resync ensures bounded staleness even if watch delivery fails

#### Reload Failure Policy

- **Initial load at startup**: if policy generation fails, server fails to start (fail-fast)
- **Subsequent reload failure**: 
  - Retain last-known-good policy set
  - Log error at ERROR level
  - Increment `authz_policy_reload_errors_total` metric
  - Retry on next watch event or resync interval
  - If no last-known-good policy set exists (impossible after successful startup), fail closed

#### Cross-Replica Consistency

- ResourceStore watches deliver changes written through any replica and changes written by the cross-region replication receiver
- All replicas converge to the same policy set (eventually consistent, bounded by watch delivery latency)
- Cache invalidation is per-replica — each replica invalidates its own local cache on watch events
- No shared state between replicas; each maintains independent policy set pointer and entity cache

---

## Authorization Middleware

### URL Parsing

The middleware parses Kubernetes API-style URL paths to extract:
- **Plural resource name** (e.g., `clusters`, `nodepools`, `rolebindings`, `roles`)
- **Namespace** (if present)
- **Resource name** (if present)

Path canonicalization is applied to prevent traversal attacks (e.g., `/../` sequences).

### ABAC Context Source by Operation

Cedar policies with conditions require evaluating against effective resource state. The table below defines how the Cedar context is built for each operation type:

| Operation | Source Of Cedar Context | Required Fields | Failure Behavior |
|---|---|---|---|
| `POST /namespaces/{ns}/{resource}` | Decoded request object from body | `resourceName` (from `metadata.name`), `resourcePlural`, `method`, `spec` | Malformed object → `400`; missing metadata.name → fail closed `403` |
| `GET /namespaces/{ns}/{resource}/{name}` | Stored object fetched before authz | `resourceName`, `resourcePlural`, `method`, `spec` | Object not found → `404`; fetch error → fail closed `403` |
| `PUT /namespaces/{ns}/{resource}/{name}` | Decoded replacement object from body | `resourceName`, `resourcePlural`, `method`, `spec` | Malformed object → `400`; context build failure → fail closed `403` |
| `PATCH /namespaces/{ns}/{resource}/{name}` | Stored object after applying patch | `resourceName`, `resourcePlural`, `method`, `spec` | Fetch or patch merge error → fail closed `403` |
| `DELETE /namespaces/{ns}/{resource}/{name}` | Stored object fetched before authz | `resourceName`, `resourcePlural`, `method`, `spec` | Object not found → `404`; fetch error → fail closed `403` |
| Namespaced list (`GET /namespaces/{ns}/{resource}`) | Namespace-level authz (no object context) + per-item stored object context via `ItemFilter` | Per-item: `resourceName`, `resourcePlural`, `spec` | Namespace authz deny → `403`; per-item condition deny → filter item out |
| Cross-namespace list (`GET /{resource}`) | Authorized namespace prefilter + per-item stored object context via `ItemFilter` | Per-item: `namespace`, `resourceName`, `resourcePlural`, `spec` | Empty authorized namespace set → return empty list (not `403`) |

**Notes**:
- `resourceName` for `POST` comes from `metadata.name` in the decoded request body. If `metadata.generateName` is used without `metadata.name`, the generated name must be determined before Cedar evaluation or conditions depending on resource name cannot be supported.
- Conditions that reference `context.spec` attributes must not be bypassed by collection-level operations — list operations use a two-phase check (namespace-level authorization followed by per-item filtering).
- For `PATCH`, the effective object state is the stored object after applying the patch using the same patch semantics as the handler (JSON Patch, JSON Merge Patch, or Strategic Merge Patch). Authorization is evaluated against the post-patch state, not the patch fragment.
- Fail-closed behavior (`403`) applies when object context cannot be built for a conditioned authorization check. Unconditioned checks (namespace-level only) do not require object reads.

### Action Derivation Map

| HTTP Method | URL Pattern | Cedar Action | Strategy |
|---|---|---|---|
| GET | `/namespaces/{ns}/clusters` | `ListClusters` | Namespace authz |
| POST | `/namespaces/{ns}/clusters` | `CreateCluster` | Namespace authz |
| GET | `/namespaces/{ns}/clusters/{name}` | `GetCluster` | Single-resource authz |
| PUT/PATCH | `/namespaces/{ns}/clusters/{name}` | `UpdateCluster` | Single-resource authz |
| DELETE | `/namespaces/{ns}/clusters/{name}` | `DeleteCluster` | Single-resource authz |
| GET | `/clusters` | `ListClusters` | **Cross-namespace** |
| GET | `/namespaces/{ns}/nodepools` | `ListNodepools` | Namespace authz |
| POST | `/namespaces/{ns}/nodepools` | `CreateNodepool` | Namespace authz |
| GET | `/namespaces/{ns}/nodepools/{name}` | `GetNodepool` | Single-resource authz |
| PUT/PATCH | `/namespaces/{ns}/nodepools/{name}` | `UpdateNodepool` | Single-resource authz |
| DELETE | `/namespaces/{ns}/nodepools/{name}` | `DeleteNodepool` | Single-resource authz |
| GET | `/nodepools` | `ListNodepools` | **Cross-namespace** |
| GET | `/namespaces/{ns}/rolebindings` | `ListRoleBindings` | Namespace authz |
| POST | `/namespaces/{ns}/rolebindings` | `CreateRoleBinding` | Namespace authz |
| GET | `/namespaces/{ns}/rolebindings/{name}` | `GetRoleBinding` | Single-resource authz |
| PUT/PATCH | `/namespaces/{ns}/rolebindings/{name}` | `UpdateRoleBinding` | Single-resource authz |
| DELETE | `/namespaces/{ns}/rolebindings/{name}` | `DeleteRoleBinding` | Single-resource authz |
| GET | `/rolebindings` | `ListRoleBindings` | **Cross-namespace** |
| GET | `/namespaces/{ns}/roles` | `ListRoles` | Namespace authz |
| POST | `/namespaces/{ns}/roles` | `CreateRole` | Namespace authz |
| GET | `/namespaces/{ns}/roles/{name}` | `GetRole` | Single-resource authz |
| PUT/PATCH | `/namespaces/{ns}/roles/{name}` | `UpdateRole` | Single-resource authz |
| DELETE | `/namespaces/{ns}/roles/{name}` | `DeleteRole` | Single-resource authz |
| GET | `/roles` | `ListRoles` | **Cross-namespace** |

Health probes (`/healthz`, `/readyz`) are registered outside the middleware chain and bypass both authentication and authorization.

### Authorization Strategies

Authorization strategies differ by operation type to correctly handle object-state-aware ABAC.

#### Create Operations (`POST /namespaces/{ns}/{resource}`)

1. Extract user from `authn.UserFromContext()`
2. Derive Cedar action from HTTP method + URL structure
3. Decode request body to extract resource object
4. Build Cedar context: `resourceName` from `metadata.name`, `resourcePlural`, `method`, and `spec` (full resource spec converted to Cedar Record)
5. Preserve request body for handler (buffer or re-encode after decoding)
6. Call `authorizer.AuthorizeWithContext(ctx, user, action, namespace, cedarCtx)`
7. Deny → `403 Forbidden`
8. Missing `metadata.name` when condition evaluation requires it → fail closed with `403`

#### Read/Delete Existing Object (`GET`/`DELETE /namespaces/{ns}/{resource}/{name}`)

1. Extract user from `authn.UserFromContext()`
2. Derive Cedar action from HTTP method + URL structure
3. **Fetch stored object** from the database using the resource name and namespace
4. If object not found → `404 Not Found` (existence check happens before authz)
5. Build Cedar context: `resourceName`, `resourcePlural`, `method`, and `spec` from the stored object
6. Call `authorizer.AuthorizeWithContext(ctx, user, action, namespace, cedarCtx)`
7. Deny → `403 Forbidden`
8. Fetch error → fail closed with `403`

**Note**: For unconditioned authorization (no RoleBinding conditions), the object fetch can be skipped. The middleware should optimize by checking whether any applicable binding has a condition before fetching.

#### Full Replacement Update (`PUT /namespaces/{ns}/{resource}/{name}`)

1. Extract user from `authn.UserFromContext()`
2. Derive Cedar action from HTTP method + URL structure
3. Decode request body to extract replacement object
4. Build Cedar context: `resourceName`, `resourcePlural`, `method`, and `spec` from the decoded object
5. Preserve request body for handler
6. Call `authorizer.AuthorizeWithContext(ctx, user, action, namespace, cedarCtx)`
7. Deny → `403 Forbidden`
8. Malformed object → `400 Bad Request`

#### Partial Update (`PATCH /namespaces/{ns}/{resource}/{name}`)

1. Extract user from `authn.UserFromContext()`
2. Derive Cedar action from HTTP method + URL structure
3. **Fetch stored object** from the database
4. **Apply patch** using the same patch semantics as the handler (JSON Patch, JSON Merge Patch, or Strategic Merge Patch depending on `Content-Type`)
5. Build Cedar context from the **post-patch object**: `resourceName`, `resourcePlural`, `method`, and `spec`
6. Preserve original patch body for handler
7. Call `authorizer.AuthorizeWithContext(ctx, user, action, namespace, cedarCtx)`
8. Deny → `403 Forbidden`
9. Fetch or patch merge error → fail closed with `403`

**Note**: Authorization evaluates the effective post-patch state, not the patch fragment. This prevents partial updates from bypassing conditioned authorization.

#### Namespaced List (`GET /namespaces/{ns}/{resource}`)

1. Extract user from `authn.UserFromContext()`
2. Derive Cedar action from HTTP method + URL structure
3. **Namespace-level authorization**: call `authorizer.Authorize(ctx, user, action, namespace)` without Cedar context (no `resourceName` or `spec`)
4. Deny → `403 Forbidden`
5. **Inject `ItemFilter` function** into the context for per-item condition evaluation
6. Handler fetches list from database
7. Handler calls `ItemFilter(ctx, item)` for each item:
   - Build Cedar context from item: `resourceName`, `resourcePlural`, `spec`
   - Evaluate Cedar policies with object context
   - Filter out denied items
8. Return filtered list

#### Cross-Namespace List (`GET /{resource}`)

1. Extract user from `authn.UserFromContext()`
2. Derive Cedar action from HTTP method + URL structure
3. Detect cross-namespace list (namespaced resource, no namespace param)
4. **Pre-compute authorized namespace set**:
   - Query user's RoleBindings
   - For each binding, check if the referenced role grants the requested action's permission
   - Collect namespaces from matching bindings (ignore conditions at this stage)
5. **Inject authorized namespaces** into the request context
6. **Inject `ItemFilter` function** for per-item condition evaluation
7. Handler reads authorized namespaces from context and sets `ListOptions.Namespaces`
8. Storage layer filters at the database level (`WHERE namespace IN (...)`)
9. Handler calls `ItemFilter(ctx, item)` for each item to apply conditions
10. Return filtered list
11. **Never returns `403`** — empty authorized namespace set produces empty list

**Note**: Cross-namespace lists use a two-phase approach: namespace prefiltering (database-level) followed by per-item condition filtering (application-level).

### Cedar Context

The Cedar context (`cedar.Record`) passed to evaluation contains:

| Key | Type | Description |
|---|---|---|
| `resourceName` | String | Name of the resource being accessed (absent for list operations) |
| `resourcePlural` | String | Plural resource type (e.g., `"clusters"`, `"nodepools"`) |
| `method` | String | HTTP method |
| `spec` | Record | Full `spec` object from the request body (for write operations) or from the stored object (for per-item filtering), recursively converted to Cedar types |

The `spec` record is built by recursively converting Go `map[string]interface{}` values to Cedar types: maps → `Record`, slices → `Set`, strings → `String`, booleans → `Boolean`, numbers → `Long`.

---

## Entity Cache

The entity cache is a thread-safe LRU cache storing `cedar.EntityMap` objects keyed by user email.

| Property | Value |
|---|---|
| Max size | 1000 entries |
| Cache key | User email (normalized) |
| Eviction | LRU (least recently used evicted on overflow) |
| Thread safety | `sync.Mutex` + `container/list` |

### Population

On the first authorization check for a user, the `EntityGetter`:

1. Queries RoleBindings where `spec.subject` matches the user email (via `FieldFilters` in `ListOptions`)
2. For each binding, resolves the referenced PlatformRole or Role
3. Builds the entity graph (User → NamespaceRole → Namespace)
4. Caches the entity map

### Invalidation

| Trigger | Scope |
|---|---|
| RoleBinding created/deleted | Affected user's cache entry evicted |
| RoleBinding updated (same subject) | Affected user's cache entry evicted |
| RoleBinding updated (subject changed) | Both old and new subjects' cache entries evicted |
| PlatformRole or Role created/updated/deleted | **All** cache entries evicted (policy set also rebuilt) |

Subject change detection uses the `PreviousObject` field on `ResourceEvent` (populated for `MODIFIED` events by the storage layer). If `PreviousObject` is unavailable (storage backend limitation), fallback to invalidating all cache entries on any RoleBinding update.

### Cache TTL (Optional)

An optional cache entry TTL can be configured via `--authz-entity-cache-ttl` (e.g., `5m`, default disabled or `0`). When enabled:
- Cache entries expire after the TTL regardless of invalidation events
- Expired entries are lazily evicted on access
- Provides bounded staleness even if watch-based invalidation fails
- Trade-off: increases database load for entity graph queries

---

## Validation

### RoleBinding Validation

1. **`subject`** is required (non-empty)
2. **Subject normalization and canonicalization**:
   - Apply the same email normalization as authentication middleware: NFC Unicode normalization, lowercase domain, preserve local-part case
   - Store the canonicalized email in the RoleBinding
   - Optionally reject subjects that are not already in canonical form (strict mode) or auto-canonicalize on write
3. **`roleRef.name`** is required; `roleRef.kind` must be `"PlatformRole"` or `"Role"`; `roleRef.apiGroup` must be `"gcp.managed.openshift.io"`
4. **Referenced role existence**: the validator verifies that the referenced role exists in the database (PlatformRole or Role, depending on `roleRef.kind`). Uses injected `ValidatorDeps` functions to avoid circular imports between the `v1` types package and the authz/storage packages.
5. **Cedar condition validation** (if `condition` is non-empty):
   - Wrap the condition in a Cedar policy template and parse via `cedar.Policy.UnmarshalCedar()`
   - Syntactically invalid Cedar expressions are rejected with descriptive error message
   - References to `Namespace::` entities are rejected to prevent namespace traversal
   - Optional: enforce maximum condition length (e.g., 4096 characters) to prevent pathological policies
   - Optional: validate that conditions are expressions only, not full Cedar policies (no `permit`/`forbid` keywords)
   - Recommended but not enforced: warn if condition references `context.spec` without `!(context has spec)` guard when the role grants list permissions

No self-grant prevention — a `service-admin` can grant any role (including `cluster-admin`) to themselves or other users. This is intentional namespace-local privilege escalation: it simplifies the bootstrap flow and trusts `service-admin` role holders to manage their own namespace. Customers should grant `service-admin` only to namespace owners.

### Role Validation

1. At least one permission is required
2. All permissions must be in the valid permission set (20 permissions listed in the Granular Permissions section)
3. Duplicate permissions are either rejected or deduplicated (implementation choice)
4. Unknown permission names are rejected with a descriptive error listing valid permissions

No infrastructure permission restriction — user-defined Roles may include any valid permission. Note that Role changes trigger full policy set rebuild and cache invalidation across all API server replicas.

### PlatformRole Validation

Same as Role validation. PlatformRoles are only created via the private API (Helm/kubectl), so the validator enforces the same schema constraints.

**Operational note**: PlatformRole changes are operationally sensitive because they trigger full policy set rebuild and cache invalidation across all API server replicas. Changes should be deployed via Helm/ArgoCD with appropriate review and staging.

### Leader-Only Writes

Role and RoleBinding writes are accepted only in the configured leader region. Follower regions keep read-only mirrors of leader state and reject direct public API write attempts, even when the caller is otherwise authorized. The CLI routes Role and RoleBinding operations to the leader by default. During a leader outage, Cedar authorization in follower regions continues to evaluate from local mirror data — existing authorization grants remain in effect, but no new Roles or RoleBindings can be created, updated, or deleted until a new leader is promoted. See the [cross-region replication plan](gcp-cross-region-resource-replication.md#public-api-read-only-enforcement) for details.

### ValidatorDeps

A global singleton injected at server startup to break the circular import between `api/private/v1` (types package) and `pkg/authz` (storage/stores). Contains function pointers:

```go
type ValidatorDeps struct {
    RoleExists         func(ctx context.Context, namespace, name string) (bool, error)
    PlatformRoleExists func(ctx context.Context, name string) (bool, error)
}
```

**Consistency note**: Role existence validation is eventually consistent with the storage backend. If a referenced role is deleted after a RoleBinding is created but before the next policy reload, the binding's policy will fail to generate. The policy reload mechanism handles this gracefully by logging an error and retaining the last-known-good policy set. The next RoleBinding update or role recreation will trigger a successful reload.

---

## User-Defined Roles

Service-admins can create namespace-scoped Roles via the public API. These roles define custom permission sets and can be bound to users with optional Cedar conditions on the RoleBinding.

### Creating a User-Defined Role

```yaml
apiVersion: gcp.managed.openshift.io/v1
kind: Role
metadata:
  name: us-east-cluster-viewer
  namespace: project-a
spec:
  permissions:
    - cluster.list
    - cluster.get
```

### Binding with a Condition

The `condition` field on the RoleBinding is a **Cedar expression body** (not a full `when` clause). The policy generator wraps it inside a `when { ... }` block alongside the mandatory namespace constraints.

```yaml
apiVersion: gcp.managed.openshift.io/v1
kind: RoleBinding
metadata:
  name: alice-us-east-viewer
  namespace: project-a
spec:
  subject: alice@example.com
  roleRef:
    kind: Role
    name: us-east-cluster-viewer
    apiGroup: gcp.managed.openshift.io
  condition: 'context.resourcePlural != "clusters" || (!(context has spec) || (context.spec has platform && context.spec.platform has gcp && context.spec.platform.gcp.region == "us-east1"))'
```

The `context.resourcePlural` guard ensures the region condition is only evaluated when accessing clusters. The nested guards (`!(context has spec)` and `has` checks) ensure safe evaluation during namespace-level authorization and handle missing attributes gracefully.

### Condition Validation

Conditions are validated at RoleBinding creation/update time:
1. The condition is wrapped in a Cedar policy and parsed via `cedar.Policy.UnmarshalCedar()`
2. Syntactically invalid Cedar expressions are rejected
3. References to `Namespace::` entities are rejected (prevents namespace traversal)

---

## Server Wiring

### Startup Sequence

1. **Storage factory selection**: Spanner (if `SPANNER_DATABASE` set) > PostgreSQL (if `DB_HOST` set) > in-memory (default)
2. **Shared memoized factory**: A `sharedFactory` wraps the storage factory with a `sync.Mutex`-protected map, ensuring the same store instance is returned for the same resource type. This is critical — both the Cedar authorizer and the API handlers must share the same stores.
3. **Authz store construction**: Creates `ResourceStore` instances for PlatformRole, Role, and RoleBinding using the shared factory
4. **Authorizer creation**: `authz.NewAuthorizer(ctx, authzStores)` loads the initial PolicySet from all roles/bindings in the stores at startup
5. **Validator deps injection**: Wires the role/binding existence check functions using the authz stores
6. **Middleware chain**: `authnMW → authzMW` is set as the public API middleware. The private API (kube-apiserver aggregated API) does **not** run Cedar middleware.
7. **Hot-reload start**: `authorizer.StartWatching(ctx)` is called synchronously before serving. If it fails, the server crashes (fail-fast rather than serving with stale policies).

### Server Ports

| Port | API | Auth |
|---|---|---|
| 8080 | Private (aggregated K8s API) | kube-apiserver authn/authz delegation |
| 8081 | Public (customer-facing) | ESPv2 + Cedar authn/authz middleware |

---

## File Structure

```text
platform-api/
  api/private/v1/
    platformrole_types.go              # PlatformRole (cluster-scoped)
    platformrole_validator.go
    role_types.go                      # Role (namespaced)
    role_validator.go
    rolebinding_types.go               # RoleBinding (namespaced)
    rolebinding_validator.go
    rolebinding_validator_test.go
    validation.go                      # ValidatorDeps + valid permissions
  api/public/v1/
    zz_generated.platformrole_types.go # (generated by orlop-gen)
    zz_generated.role_types.go
    zz_generated.rolebinding_types.go
    zz_generated.conversion.go
    zz_generated.schemas.go
  pkg/
    authn/
      context.go                       # WithUser / UserFromContext
      middleware.go                    # ESPv2 header extraction + dev mode
      middleware_test.go
    authz/
      authorizer.go                    # Cedar PolicySet + Authorize() + AuthorizedNamespaces()
      authorizer_test.go
      cache.go                         # LRU entity cache (1000 entries)
      cache_test.go
      entities.go                      # EntityGetter + AuthzStores
      entities_test.go
      middleware.go                    # HTTP authz middleware (URL parsing, action derivation)
      middleware_test.go
      permissions.go                   # Permission-to-Action mapping (20 permissions)
      policygen.go                     # Cedar policy generation from roles/bindings
      policygen_test.go
      reload.go                        # Watch-based hot-reload
      reload_test.go
  cmd/platform-api-server/
    main.go                            # Wire authn/authz, load roles, start watching
    resources.go                       # Resource config + authz store construction
helm/charts/platform-api-server/
  templates/platformroles/
    platformrole-cluster-admin.yaml
    platformrole-cluster-viewer.yaml
    platformrole-service-admin.yaml
  values.yaml                         # platformRoles.enabled flag
orlop/pkg/apiserver/
  handlers/context.go                  # AuthorizedNamespaces + ItemFilter context keys
  storage/interface.go                 # ListOptions.Namespaces, FieldFilters
  storage/types.go                     # ResourceEvent.PreviousObject
```

---

## Verification Checklist

### Authentication & Trust Boundary
- [ ] Missing `X-Endpoint-API-UserInfo` header → `401 Unauthorized`
- [ ] Malformed base64 or JSON in header → `401 Unauthorized`
- [ ] Empty or missing `email` claim → `401 Unauthorized`
- [ ] `email_verified == false` → `401 Unauthorized` (when claim is present)
- [ ] Direct request to app port cannot spoof `X-Endpoint-API-UserInfo` (deployment test)
- [ ] ESPv2-proxied request produces trusted identity (integration test)
- [ ] RoleBinding subject normalization matches request principal normalization exactly
- [ ] Dev mode (`X-Dev-User`) works with authorization still active
- [ ] Disabled auth mode (`--disable-auth`) skips both authn and authz

### Basic Authorization
- [ ] Authenticated user with no bindings → `403 Forbidden`
- [ ] cluster-admin can create/list/get/update/delete clusters within their namespace
- [ ] cluster-admin can create/list/get/update/delete nodepools within their namespace
- [ ] cluster-admin cannot manage rolebindings or roles
- [ ] cluster-viewer can list/get clusters and nodepools
- [ ] cluster-viewer cannot create/update/delete clusters or nodepools
- [ ] service-admin can create/list/get/update/delete rolebindings within their namespace
- [ ] service-admin can create/list/get/update/delete roles within their namespace
- [ ] service-admin cannot create/update/delete clusters or nodepools
- [ ] service-admin can bind cluster-admin to themselves (intentional self-grant)
- [ ] service-admin binding cluster-admin to themselves grants cluster permissions
- [ ] service-admin cannot affect other namespaces

### Conditioned Authorization - Create
- [ ] Conditioned `cluster.create` with allowed spec → succeeds
- [ ] Conditioned `cluster.create` with disallowed spec → `403 Forbidden`
- [ ] Missing `metadata.name` with conditioned create → fail closed `403`

### Conditioned Authorization - Read
- [ ] Conditioned `cluster.get` with allowed stored object → succeeds
- [ ] Conditioned `cluster.get` with disallowed stored object → `403 Forbidden`
- [ ] Conditioned `cluster.get` on non-existent object → `404 Not Found`

### Conditioned Authorization - Update
- [ ] Conditioned `cluster.update` (PUT) into allowed state → succeeds
- [ ] Conditioned `cluster.update` (PUT) into disallowed state → `403 Forbidden`
- [ ] Conditioned `cluster.update` (PATCH) with final state allowed → succeeds
- [ ] Conditioned `cluster.update` (PATCH) with final state disallowed → `403 Forbidden`
- [ ] PATCH evaluation uses post-patch object state, not patch fragment

### Conditioned Authorization - Delete
- [ ] Conditioned `cluster.delete` with allowed stored object → succeeds
- [ ] Conditioned `cluster.delete` with disallowed stored object → `403 Forbidden`

### Conditioned Authorization - List
- [ ] Namespaced list: namespace-level authz allows query
- [ ] Namespaced list: per-item filter removes disallowed objects
- [ ] Namespaced list: allowed items appear, disallowed items filtered out
- [ ] Cross-namespace list: namespace DB filter limits candidate namespaces
- [ ] Cross-namespace list: per-item condition filter still applies
- [ ] Cross-namespace list with no authorized namespaces → empty list (not `403`)

### Multi-Resource Conditions
- [ ] Role grants `cluster.get` + `nodepool.get`, condition targets clusters only
- [ ] Accessing cluster applies region condition correctly
- [ ] Accessing nodepool does not spuriously deny due to cluster condition
- [ ] Condition without `context.resourcePlural` guard fails for unrelated resources

### Cross-Namespace Operations
- [ ] Cross-namespace list returns only resources from authorized namespaces
- [ ] User with bindings in namespace A and B sees resources from both
- [ ] User with binding in namespace A sees only namespace A resources

### Role & Binding Management
- [ ] User-defined Role creation via public API succeeds
- [ ] User-defined Role with valid permissions → accepted
- [ ] User-defined Role with invalid permission → rejected with descriptive error
- [ ] PlatformRole mutations via public API → no endpoint exists
- [ ] RoleBinding referencing non-existent role → rejected at creation
- [ ] RoleBinding with valid Cedar condition → accepted
- [ ] Cedar condition with invalid syntax → `400 Bad Request` at RoleBinding creation
- [ ] Cedar condition containing `Namespace::` → rejected at RoleBinding creation
- [ ] RoleBinding subject is normalized and stored in canonical form

### Hot-Reload & Multi-Replica
- [ ] Creating a new RoleBinding takes effect without server restart
- [ ] Deleting a RoleBinding removes access without server restart
- [ ] Updating a Role rebuilds policies and invalidates cache
- [ ] Deleting a Role invalidates policies and cache immediately
- [ ] RoleBinding created through replica A is enforced by replica B (cross-replica consistency)
- [ ] Role change invalidates all replicas (multi-replica cache invalidation)
- [ ] Watch interruption recovers through retry/resync
- [ ] Periodic resync (if enabled) catches missed watch events

### Reload Failure Handling
- [ ] Policy rebuild failure retains last-known-good policy set
- [ ] Policy rebuild failure increments error metric
- [ ] Initial startup with bad policy → server fails to start
- [ ] Subsequent reload failure → retry on next watch event

### Per-Binding Policy Isolation
- [ ] User A: unconditional binding to `cluster-viewer`
- [ ] User B: conditioned binding to `cluster-viewer` (same role, same namespace)
- [ ] User A's unconditional binding does not satisfy User B's conditioned policy
- [ ] User B's access is correctly filtered by condition

### Health Probes
- [ ] Health probes (`/healthz`, `/readyz`) bypass authn/authz
- [ ] Health probes bypass authn/authz
