# Allow Authenticated Version, Channel, and PlatformRole Reads Outside Cedar Authorization

***Scope***: GCP-HCP

**Date**: 2026-10-06

**Status**: Superseded

**Superseded by**: [Add Cluster-Scoped Cedar ABAC Authorization](platform-scoped-cedar-authorization.md)

## Decision

Version, Channel, and PlatformRole collection and item reads require a valid
public API identity but do not require a Cedar `RoleBinding`. This decision
explicitly exempts only their `get` and `list` operations from Cedar authorization;
writes, watches, and every other resource remain subject to the existing
authorization path. This is a shared-platform-metadata exception, not a general
authorization model for non-namespaced resources.

## Context

- **Problem Statement**: `gcphcpctl` validates a requested Version before
  creating a Cluster, and service admins need to discover platform-managed
  roles when creating or auditing RoleBindings. Version, Channel, and PlatformRole
  are non-namespaced, shared platform metadata resources, so their routes have no
  namespace. PlatformRole discovery is read-only; role definitions remain
  system-managed. The existing Cedar model is intentionally namespace-pinned: a
  `RoleBinding` grants a permission only in its namespace. Without an explicit
  authorization rule, the middleware fails closed for direct non-namespaced routes.
- **Constraints**: Authentication remains mandatory through ESPv2. The current
  Cedar entity and permission model contains only namespace-scoped grants;
  non-namespaced shared-metadata authorization would require a separate grant
  model that is not tied to a namespace.
- **Assumptions**: Release and channel metadata, plus PlatformRole names and
  permission sets, are appropriate for every authenticated customer to read.
- **Non-goal**: This decision does not define how Cedar authorizes
  non-namespaced resources beyond the explicitly named read-only resources. That
  requires a separate resource-scope and grant model decision.

## Alternatives Considered

1. **Bypass Cedar authorization for authenticated shared-platform-metadata
   reads**: Require an ESPv2-authenticated identity, then bypass Cedar
   authorization for a fixed allowlist of Version, Channel, and PlatformRole
   collection and item `get`/`list` operations. Every authenticated caller is
   allowed these reads without a Cedar RoleBinding. This is the chosen approach.
2. **Authorize shared-platform-metadata reads through Cedar**: Model Version,
   Channel, and PlatformRole as non-namespaced Cedar resources, add actions for
   their `get` and `list` operations, and evaluate every request through Cedar.
   This requires extending the current namespace-scoped authorization model with
   a non-namespaced resource and policy scope. It would retain Cedar's
   policy-controlled access decisions, but establishes authorization semantics
   outside the current namespace-isolated model. It must define who receives
   access and how that non-namespaced grant is administered.

| Alternative | Benefits | Trade-offs |
| --- | --- | --- |
| Bypass Cedar authorization for authenticated shared-platform-metadata reads (chosen) | Meets Version validation and PlatformRole discovery use cases without a namespace binding or non-namespaced Cedar model; keeps ESPv2 authentication mandatory; narrowly expresses that the resources are shared metadata. | Cedar cannot grant, revoke, or record a determining policy for these reads; changing access requires changing and deploying the application rule. |
| Authorize shared-platform-metadata reads through Cedar | Every read follows the same policy engine; access can be changed through Cedar policy and can support policy-controlled allow/deny decisions. | Introduces a non-namespaced authorization scope alongside the existing namespace-isolated model, and requires explicit grant and administration semantics. |

## Decision Rationale

* **Justification**: Version validation is a prerequisite to cluster creation,
  and Version and Channel describe shared platform capabilities. PlatformRole
  discovery lets callers select valid system-managed roles and inspect their
  permissions without relying on out-of-band documentation. These resources are
  shared platform metadata rather than customer-owned state, and their data is
  approved for every authenticated caller. Namespace-bound grants cannot
  authorize their direct non-namespaced routes in the current model.
* **Evidence**: The current Cedar decision defines only `User`,
  `NamespaceRole`, and `Namespace` entities and explicitly pins generated
  policies to a namespace. The [CLI calls the non-namespaced
  `/versions/{name}` route](https://github.com/openshift-online/gcp-hcp-ctl/blob/main/pkg/cluster/create.go)
  before it derives the namespace for Cluster create. [GCP-1273](https://redhat.atlassian.net/browse/GCP-1273)
  requires read-only public PlatformRole discovery on cluster-scoped routes.
* **Comparison**: Non-namespaced Cedar authorization would preserve
  Cedar-controlled access decisions, but introduces a non-namespaced policy
  scope alongside the existing namespace-isolated model, with corresponding
  policy and operational complexity. The narrow application rule keeps the
  explicitly approved shared metadata outside namespace authorization while
  retaining mandatory authentication.

## Consequences

### Positive

* Every authenticated caller can validate Versions, discover Channels, and
  discover PlatformRoles (including their complete permission sets) without a
  pre-existing namespace RoleBinding.
* Existing namespace-scoped Cedar policy generation, role bindings, and
  cross-namespace filtering remain unchanged.
* The exception is explicit, generated from resource metadata, and restricted
  to non-mutating operations.

### Negative

* PlatformRole names and complete permission sets are disclosed to every
  authenticated caller.
* Shared-metadata reads cannot be revoked for an individual caller through Cedar
  while the exemption exists.
* Removing shared-metadata access requires changing the exemption and deploying
  it, not editing a RoleBinding or Cedar policy.

## Cross-Cutting Concerns

### Security

* ESPv2 authentication and Gecko identity validation run before the Cedar
  exemption. Requests without a valid identity remain denied.
* The exemption is limited to public Version, Channel, and PlatformRole
  `get` and `list` operations. Generator validation rejects mutating exempt
  verbs; watch is not declared exempt for any of these resources. Public
  PlatformRole exposure is read-only: create, update, patch, and delete are not
  exposed. No customer-owned resource is included in this exception.
* Rate limiting and user bans are separate concerns. This decision does not
  depend on a particular edge or application control, and adding one does not
  require broadening the catalog exemption. If audit for exempt reads is
  required, it must come from request/authentication observability rather than
  a Cedar determining-policy record.
* Reassess this decision if shared metadata or PlatformRole definitions
  become sensitive, customer-specific, or require per-principal policy control.
  In that case, extend Cedar with an explicit non-namespaced-resource
  authorization model rather than adding further exemptions.

### Operability

* Public shared-platform-metadata access continues to depend on the ESPv2
  trust boundary: the application listener must not be directly reachable outside
  the proxy.
* The exemption remains visible in the resource definition and is covered by
  generator, middleware, and public API tests.
## Related

* [Cedar-based public API authorization](cedar-public-api-authorization.md)
* [Cedar authorization implementation plan](../../implementation-plans/gcp-cedar-public-api-authorization.md)
* [GCP-1266](https://redhat.atlassian.net/browse/GCP-1266)
* [GCP-1273](https://redhat.atlassian.net/browse/GCP-1273)
