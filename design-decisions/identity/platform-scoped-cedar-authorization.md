# Authorize Platform-Scoped Resources Through Role Bindings

***Scope***: GCP-HCP

**Date**: 2026-10-09

**Supersedes**: [Allow Authenticated Version, Channel, and PlatformRole Reads
Outside Cedar Authorization](authenticated-catalog-reads.md)

## Decision

Gecko extends its existing Cedar-backed role-binding mechanism with private,
platform-scoped `PlatformRoleBinding` resources. Each binding grants permissions
from a `PlatformRole` to an authenticated subject for platform-scoped resources
and supports the same optional Cedar condition as a namespace-scoped
`RoleBinding`.

Argo CD manages platform bindings through the private Kubernetes API. Version
and Channel `get`/`list` are the first platform-scope operations authorized
through these bindings. An `authenticated-user` PlatformRole and an
all-authenticated binding preserve their current read access, with the initial
binding's condition left unset.

## Context

The existing authorization mechanism generates Cedar policies from roles and
bindings. A PlatformRole supplies system-managed permissions; a RoleBinding
grants those permissions within its own namespace. Version and Channel are
platform-scoped catalog resources whose reads were exempted from Cedar by the
[authenticated catalog-reads decision](authenticated-catalog-reads.md).

[Object-state-aware RoleBinding conditions](https://github.com/openshift-online/gecko/pull/484)
already provide optional Cedar expressions, effective-object evaluation, and
per-item list filtering before pagination. Platform-scope authorization can
reuse these capabilities while moving catalog reads into the same grant model.

The [Cedar authorization decision](cedar-public-api-authorization.md) considered
a public PlatformRoleBinding model for cross-namespace administration. This
decision introduces private grants for platform-scoped resources; Cluster,
NodePool, and other namespace-scoped resources retain their existing
namespace-bound authorization.

## Authorization Model

### Bindings and Scope

* A PlatformRoleBinding preserves the RoleBinding spec shape: `subject`,
  `roleRef`, and optional `condition`. Its role reference identifies a
  PlatformRole.
* A PlatformRoleBinding grants access only to platform-scoped resources. A
  RoleBinding grants access only to namespace-scoped resources in its own
  namespace, including when it references a PlatformRole.
* Existing registered resource-scope metadata and explicit role permissions
  determine the applicable grants. A namespace-less collection route for a
  namespace-scoped resource retains the existing cross-namespace behavior.
* A role may contain permissions for both scopes. Only permissions applicable
  to the binding's scope affect authorization. Validation continues to check
  known permissions, subjects, role references, and Cedar condition syntax;
  runtime scope enforcement supplies the access boundary.
* Each binding generates its own Cedar policy through the existing
  authorization flow. Policy generation, watches, reloads, and entity-cache
  invalidation extend the established RoleBinding mechanism.

### Subjects and Conditions

Exact subjects use the existing normalized authenticated identity. In a
PlatformRoleBinding, `subject: "*"` selects every authenticated principal.
Namespace-scoped RoleBinding subject semantics remain unchanged.

Cedar conditions are supported from introduction and constrain the binding's
grant using the existing object-aware evaluation mechanism. Subject matching,
role permissions, resource scope, and the optional condition must all permit
the operation. Grants remain additive across independently authorizing
bindings.

### Initial Grant

The initial configuration grants Version and Channel reads through one shared
role and binding:

```yaml
apiVersion: gcp.managed.openshift.io/v1
kind: PlatformRole
metadata:
  name: authenticated-user
spec:
  permissions:
    - version.get
    - version.list
    - channel.get
    - channel.list
---
apiVersion: gcp.managed.openshift.io/v1
kind: PlatformRoleBinding
metadata:
  name: authenticated-users
spec:
  subject: "*"
  roleRef:
    apiGroup: gcp.managed.openshift.io
    kind: PlatformRole
    name: authenticated-user
```

The optional condition is omitted from this binding. The role name is ordinary
configuration; its explicit permissions and binding establish access.

## Alternatives Considered

1. **Platform-scope role bindings (chosen)**: Extend the existing binding
   primitive with a private PlatformRoleBinding referencing a PlatformRole.
   Argo CD manages grants, and optional Cedar conditions provide attribute-based
   restrictions. One all-authenticated binding covers the initial catalog use
   case. This adds a private resource kind and integrates it with authorization
   storage, policy generation, and reload processing.
2. **Embedded, named Cedar policies**: Associate resource/action operations with
   named platform-managed Cedar policies through generated Orlop authorization
   metadata, as explored in [Gecko #447](https://github.com/openshift-online/gecko/pull/447).
   This keeps the persisted API surface smaller and supports direct authoring
   of Cedar `permit` and `forbid` policies. It introduces a policy-selection
   mechanism alongside binding-derived authorization, and changes to embedded
   rules or associations follow a Gecko release.

## Decision Rationale

* **Authorization consistency**: PlatformRoleBindings extend the existing
  subject, role, binding, and condition model. Existing resource-scope metadata
  and permissions drive evaluation while the binding determines the grant's
  scope.
* **Operational flexibility**: Reviewed GitOps changes can update bindings and
  their conditions using supported permissions and attributes. This separates
  grant changes from authorizer releases and fits the established Argo CD
  workflow. New permissions or context attributes can still require Gecko
  changes.
* **Shared authenticated access**: A single all-authenticated binding preserves
  current catalog access and avoids per-caller onboarding. Both considered
  approaches can express this baseline.
* **Existing condition support**: Both approaches use Cedar and can express
  attribute-based restrictions. Binding conditions already support effective
  object state and per-item list filtering, making that capability part of the
  shared authorization mechanism.
* **Trade-off**: The additional private binding kind and its authorization
  integration are justified by consistent grant management and operational
  flexibility. Embedded policies remain a viable option for fixed rules and
  direct policy authoring; the existing binding primitive better matches this
  use case and its management workflow.

## Consequences

### Positive

* Version and Channel reads use binding-derived Cedar authorization while
  preserving their public API paths and read-only access.
* The catalog authorization exemption is retired; catalog reads obtain a
  determining Cedar policy through the shared authorization mechanism.
* Platform grants are reviewable GitOps resources and support the existing
  Cedar condition and filtering capabilities.
* Namespace-scoped authorization retains its established permissions and scope
  boundaries.

### Negative

* A new private resource kind requires storage registration and integration
  with the existing authorization machinery.
* Catalog access depends on the configured role and binding. Missing or invalid
  grants deny access, so GitOps configuration becomes part of catalog
  availability.
* The binding API exposes additive grants and grant-local conditions. Global
  Cedar `forbid` policy management remains a separate capability.

### Follow-on Work

* PlatformRole public discovery follows GCP-1273. Its public endpoint,
  permissions, and corresponding grants are delivered together.
* Additional trusted principal attributes are introduced when configured Cedar
  conditions require them.

## Cross-Cutting Concerns

### Security

* Authentication and normalized identity extraction precede authorization.
  The `*` subject selects authenticated principals only.
* The management surface for PlatformRoleBindings is the private Kubernetes
  API. Private Kubernetes RBAC reserves binding mutation for the Argo CD
  identity, with changes controlled through infrastructure pull-request review.
* Each binding's policy enforces the resource-scope boundary independently of
  role contents. Missing or invalid bindings grant no access through that
  binding, while independent valid grants retain their existing behavior.
* Conditions use trusted identity and public object context. The initial grant
  uses the existing authenticated identity; additional principal attributes
  follow the needs of configured conditions.
* Collection permission checks retain their existing coarse semantics, with
  conditions evaluated per item before pagination. A condition excluding a
  caller can yield an empty authorized list rather than a request-level denial.

### Operability

* PlatformRole definitions follow the existing Helm and Argo CD workflow.
  PlatformRoleBindings are applied through Argo CD using the configuration
  managed in [gcp-hcp-infra](https://github.com/openshift-online/gcp-hcp-infra).
* Platform bindings participate in the established watch/reload and
  cache-invalidation lifecycle. Failed refreshes retain the last successfully
  loaded policy set, preserving existing RoleBinding behavior and revocation
  semantics.

## Related

* [Authenticated catalog reads](authenticated-catalog-reads.md) (superseded)
* [Cedar-based public API authorization](cedar-public-api-authorization.md)
* [PlatformRoleBinding POC (Gecko #596)](https://github.com/openshift-online/gecko/pull/596)
* [Object-state-aware RoleBinding conditions (Gecko #484)](https://github.com/openshift-online/gecko/pull/484)
* [GCP-1266](https://redhat.atlassian.net/browse/GCP-1266)
* [GCP-1273](https://redhat.atlassian.net/browse/GCP-1273)
