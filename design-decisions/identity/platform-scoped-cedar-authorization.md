# Add Cluster-Scoped Cedar ABAC Authorization

***Scope***: GCP-HCP

**Status**: Draft (for discussion)

**Date**: 2026-10-09

**Supersedes**: [Allow Authenticated Version, Channel, and PlatformRole Reads
Outside Cedar Authorization](authenticated-catalog-reads.md)

## Decision

Gecko adds a cluster-scoped ABAC authorization path for explicitly configured
public resources. Platform-managed Cedar policies evaluate trusted principal,
resource, and request attributes for resources in a default platform scope.
This path complements the current namespaced, RoleBinding-derived RBAC path.

Version and Channel `get`/`list` are the first platform-scope operations. Their
public routes and `+orlop:public-verbs` remain unchanged. Each operation selects
the shared platform-managed `authenticated-catalog-read` policy through
`+orlop:authorization-policy`.

## Context

The current Cedar authorization path maps namespace-scoped requests into a
RoleBinding-derived RBAC entity graph. Version and Channel are cluster-scoped
catalog resources whose reads were exempted from Cedar evaluation by the
[authenticated catalog-reads decision](authenticated-catalog-reads.md). That
decision identified Cedar-based authorization as a viable alternative and
included a reassessment clause directing this extension when per-principal
policy control is needed.

This decision replaces the exemption with a Cedar ABAC path for explicitly
configured cluster-scoped resources, beginning with Version and Channel. No
request reaches the handler without an explicit Cedar `permit`.

## Alternatives Considered

1. **Direct platform Cedar ABAC policies (chosen)**: Add a default platform
   scope, explicit resource/action policy associations, and platform-managed
   Cedar policies for cluster-scoped routes.
2. **Cluster-scoped role bindings**: Add a declarative cluster role and binding
   model, plus an onboarding and grant-management workflow for platform-scoped
   assignments.

| Alternative | Benefits | Trade-offs |
| --- | --- | --- |
| Direct platform Cedar ABAC policies (chosen) | Access derived from trusted request facts; no pre-created per-caller bindings required; policies can incorporate principal, resource, and request attributes for audience-gated visibility; Cedar `forbid` policies can revoke access for specific principals. | Introduces a platform-scoped policy path alongside the namespace-scoped path; authorization decisions depend on the accuracy of server-provided attributes. |
| Cluster-scoped role bindings | Explicit grants to individual users or groups; consistent model with the existing namespaced RBAC path. | Requires Gecko to create or infer an artificial cluster binding for each entitled caller; cannot express attribute-based conditions (e.g. channel type, release stage) without layering ABAC on top. |

## Decision Rationale

* **ABAC choice**: Direct platform policies express access from trusted request
  facts instead of pre-created user bindings. For the catalog use case, every
  authenticated caller has the same access today, making per-caller bindings
  unnecessary overhead.
* **Role-binding trade-off**: A cluster-scoped role-binding model represents
  explicit grants to individual users or groups. It would require Gecko to
  create or infer an artificial cluster binding for each entitled caller, with
  no gain until per-principal differentiation is needed — and when it is, ABAC
  handles it directly through attributes.
* **Attribute expansion**: Platform policies can tailor the catalog visible to a
  caller from trusted caller and resource attributes. For example, internal
  users can view candidate or nightly Channel resources while other callers see
  the generally available channels. PlatformRole visibility can be filtered by
  a resource attribute once its public endpoint is delivered.
* **Bypass elimination**: The authorization-exemption class is entirely removed.
  Every catalog read has a determining Cedar policy, providing an audit trail
  that the exemption model could not.

### RBAC and ABAC in Practice

The existing namespaced path and the platform path answer different
authorization questions. Namespaced RBAC evaluates whether a principal has a
RoleBinding-derived role for an action in a namespace. Platform ABAC evaluates
whether trusted properties of the principal, resource, and action satisfy a
platform policy.

The namespaced path generates Cedar policies from RoleBindings:

```cedar
permit (
    principal,
    action in [Action::"GetCluster"],
    resource
)
when {
    principal in NamespaceRole::"team-a/cluster-viewer/alice-binding" &&
    resource in Namespace::"team-a"
};
```

The platform path evaluates a policy for a resource in the default platform
scope. A Channel policy can use trusted resource and principal attributes:

```cedar
permit (
    principal,
    action == Action::"GetChannel",
    resource in PlatformScope::"default"
)
when {
    resource.resourceType == "channels" &&
    (resource.visibility == "public" || principal.internal == true)
};
```

RBAC derives access from a role relationship and namespace. ABAC derives access
from trusted properties of the principal, resource, and action. Both run in
Cedar, and a future model can combine role bindings with attributes when a use
case requires both.

## Consequences

### Positive

* Version and Channel reads use Cedar while preserving their public API paths
  and cluster-scoped API model.
* Explicit resource/action policy associations define the cluster-scoped
  operations that participate in platform Cedar evaluation.
* The authorization-exemption class is removed; every catalog request has a
  determining Cedar policy and audit trail.
* The same policy path supports trusted attributes, Cedar `forbid` policies,
  and audience-gated visibility as product requirements emerge.

### Follow-on Work

* PlatformRole joins the platform-scope path in a later migration after
  GCP-1273 delivers its public API work.
* The initial policy source is platform-managed configuration. Dynamic grants,
  denies, entitlements, and per-item list filtering can build on this path as
  their product requirements are defined.

## Cross-Cutting Concerns

### Security

* Authentication and normalized identity extraction occur before Cedar
  evaluation. The server exclusively manages the policy source.
* Platform-resource registration declares each participating resource/action
  policy association. Unregistered resources and unconfigured verbs are denied.
* Attribute-based policies use trusted server-provided attributes. A policy that
  filters a collection evaluates items before pagination and
  continuation-token construction.
* The existing public authorization feature flag remains the rollout control.
  When enabled, requests for resources in the default platform scope use their
  Cedar policy path with no per-resource authorization exemptions.

### Operability

* The platform policy is included in every Cedar policy-set rebuild. Policy
  parse errors at startup prevent deployment of malformed policies.
* Migration deploys and validates the default platform-scope path for Version
  and Channel. PlatformRole joins a later migration after GCP-1273.

## Related

* [Authenticated catalog reads](authenticated-catalog-reads.md) (superseded)
* [Cedar-based public API authorization](cedar-public-api-authorization.md)
* [Cedar authorization implementation
  plan](../../implementation-plans/gcp-cedar-public-api-authorization.md)
* [GCP-1266](https://redhat.atlassian.net/browse/GCP-1266)
* [GCP-1273](https://redhat.atlassian.net/browse/GCP-1273)
