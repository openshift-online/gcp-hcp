# Publish Installation IAM Contracts as a Digest-Pinned WIFConfigTemplate Catalog

***Scope***: GCP-HCP

**Date**: 2026-11-25

## Decision

Gecko publishes each release boundary's installation IAM contract as a
platform-scoped, typed `WIFConfigTemplate` resource whose executable content
(`spec.schemaVersion` and `spec.iam`) is immutable and identified by a canonical
SHA-256 digest. A `Version` pins exactly one contract reference (name, UID,
digest) once, and Clusters record bounded append-only customer assertions of
applied contracts. A customer-operated CLI — never Gecko or a management
cluster — applies and verifies IAM in the customer project.

Contract invariants enforced by this decision:

- The executable content and schema version are immutable after creation;
  template deletion is initially rejected, retaining historical contracts.
- `spec.verified` is the single mutable eligibility assertion; observing it false
  blocks new authorization without revoking running access or cancelling committed
  work. The accepted read-to-commit revocation race is specified in the
  [dispatch protocol](../../implementation-plans/wifconfigtemplate-catalog-and-iam-lifecycle.md#5-new-dispatch-protocol-and-upgrade-interaction).
- Cluster `templateApplications` are append-only, immutable and bounded (1–128);
  new appends require an intact verified contract.
- Custom roles may not grant service-account impersonation or credential-minting
  permissions; actAs authority exists only on the exact worker service-account
  grant in `spec.iam.serviceAccountRoleBindings`.
- Cluster and NodePool `status.iamRequirements` are read-only projections, not
  dispatch receipts, cloud attestation or client-writable fields.
- Public catalog get/list uses the same authenticated-read exemption as
  Version/Channel ([GCP-1266](https://redhat.atlassian.net/browse/GCP-1266)) until
  [GCP-1406](https://redhat.atlassian.net/browse/GCP-1406) replaces catalog exemptions
  with platform-scoped Cedar grants. Authentication remains mandatory; public
  writes are unavailable and writes use the delegated private API and platform RBAC.

## Context

- **Problem Statement**: GCP Hosted Control Planes need exact, per-release IAM
  in customer projects. The prior model rendered IAM client-side from
  permission knowledge embedded in the CLI, relied on broad predefined project
  roles, could not pin which contract an installation actually received, and
  therefore could not gate upgrades on recorded IAM coverage.
  Nothing tied a running installation to the reviewed permission set of its
  release boundary.
- **Constraints**:
  - The customer owns project IAM; the platform holds no IAM-admin authority in
    customer projects (see [zero-operator-access](zero-operator-access.md)).
  - Contracts must be identical for every installation of a release boundary
    and reproducible across regions (see
    [cross-region resource replication](../infrastructure/cross-region-resource-replication.md)).
  - GitOps is the publication authority for shared contracts and verification.
  - The catalog must be readable by the customer CLI before a Cluster exists,
    through the same platform-scoped authorization as other catalog resources.
  - Contract changes after first publication cannot be allowed to silently
    redefine what installed clusters are entitled to retain.
- **Assumptions**:
  - Release boundaries are canonical numeric `X.Y.Z` names; Cincinnati remains
    the release-metadata source, with contracts pinned onto Gecko `Version`
    resources rather than carried by Cincinnati.
  - Contract schema evolution is rare and must be explicitly versioned.
  - Fresh installations are the initial supported population; migration of
    existing broad-role installations is out of scope.

## Alternatives Considered

1. **CLI-embedded permission catalog**: each CLI release embeds the permission
   sets it knows and renders IAM per cluster (the pre-POC practice). No
   platform-side pinning, audit trail or revocation; CLI version drift produces
   divergent installations; upgrades cannot verify coverage against the running
   release.
2. **Untyped contract store**: keep contracts as opaque YAML in ConfigMaps or
   Secrets in the platform cluster. No admission validation of contract shape,
   no generated public API or schema guarantees, weak immutability enforcement,
   and a digest that would be trusted without typed canonicalization.
3. **Contracts in Cincinnati channel metadata**: extend release metadata
   distribution to carry IAM contracts. Cincinnati is advisory release
   metadata, not an authoritative platform execution store; it has no
   write-side authorization or immutability enforcement, and it couples
   contract lifecycle to a system the platform does not control.
4. **Broad predefined project roles**: retain `roles/editor`-level grants.
   Violates least privilege, cannot express exact per-component permissions,
   provides no upgrade-gating granularity and leaves shared worker service
   account exposure unbounded.

## Decision Rationale

* **Justification**: A typed, digest-pinned catalog makes per-release contracts
  reviewable platform data. The canonical digest lets the CLI, controllers and
  auditors verify executable content against its pinned reference.
  Append-only assertions provide exact coverage gating for upgrades without
  narrowing running permissions. Keeping cloud verification in the
  customer-operated CLI preserves the authority boundary: a Cluster application
  is an authenticated customer assertion, never proof that cloud IAM exists.
* **Evidence**: The POC demonstrated fresh installs, IAM-gated control-plane and
  worker upgrades, additive role updates, coexistence and owned cleanup. The
  [inventory](../../experiments/wifconfigtemplate-integration-inventory/README.md#poc-results-and-limits)
  records the tested paths and their limits; it is not production-readiness proof.
  Delivery status is maintained in the
  [implementation plan](../../implementation-plans/wifconfigtemplate-catalog-and-iam-lifecycle.md#delivery-status-and-tracking).
* **Comparison**: The alternatives fail respectively on pinning and audit
  (1), validation and canonicalization (2), authority and lifecycle control
  (3), and least privilege (4). The typed catalog combines admission validation,
  immutable retention, exact per-release pinning and customer-owned application
  within the existing platform API.

## Consequences

### Positive

* Reviewed per-component baselines per release boundary, independently
  verifiable via the digest by any party holding the contract.
* Upgrades can require exact target coverage (`WIFConfigUpdateRequired`)
  without revoking permissions that running installations already depend on.
* The customer retains full authority over project IAM; the platform never
  needs IAM-admin authority to operate the lifecycle.
* Immutable retained history supports audit and permission-retention planning
  even after verification revocation or upstream Version aging.
* Catalog reads follow the same access model as Version/Channel: authenticated
  reads initially, followed by explicit Cedar grants without a read-access regression.

### Negative

* Pre-publication discipline is mandatory: after first publication the
  baseline is digest-frozen; a genuinely different contract requires a new
  boundary, not an edit.
* Digest protocol governance: the canonical encoding is deliberately not
  RFC 8785; every new spec field needs an explicit protocol and immutability
  decision, and golden re-pins require security review.
* Sequenced delivery burden: enabling application writes requires compatible
  API replicas, typed consumers, authenticated catalog access and the CLI lifecycle.
  Cedar migration is a separate GCP-1406 rollout, not a GCP-1506 prerequisite.
* Interim read access bypasses Cedar checks and conditions for authenticated
  callers; GCP-1406 must deliver verified seed grants before removing exemptions.
* Storage correctness is a hard dependency: revision CAS and selector
  LIST/watch must hold on every supported backend before assertion writes are
  enabled.
* No migration path for existing broad-role installations in the initial
  design; fresh installations only.

## Cross-Cutting Concerns

### Security:

* Least privilege: exact per-component permission sets; custom roles are
  project-bound and may not grant `actAs`, token or key minting, or signing
  permissions — the exact worker service-account grant is the only impersonation
  path.
* Access boundaries: authentication is mandatory. The interim catalog get/list
  exemption bypasses Cedar authorization, not authentication; GCP-1406 replaces it
  with grants under the platform-scoped Cedar design. Template writes remain
  private-RBAC/GitOps only throughout the transition.
* Open design: the digest algorithm, canonical encoding and participation
  rules are documented in Gecko's `WIFCONFIGTEMPLATE.md` and pinned by golden
  vectors.

### Reliability:

* **Resiliency**: append-only history keeps running installations valid through
  verification revocation and catalog aging; dispatch persists exact-target
  receipts before external Apply and redelivers only the same committed intent
  after restart. Multi-region consistency follows the platform's replicated
  storage rather than a second catalog channel.
* **Observability**: `status.iamRequirements` projections carry observed
  generation and refresh time; consumers must require complete, fresh views
  and never treat missing status as proof that no permissions are needed.

### Operability:

* GitOps (Argo) is the publication authority; verification changes go through
  owner/region values, and direct private-API patches are temporary and may be
  self-healed away.
* Activation is staged and gated (`iamApplications.enabled=false` by default);
  there is no safe old-binary rollback after first unsupported pins or
  assertions.

## References

* Implementation: [WIFConfigTemplate catalog and IAM lifecycle](../../implementation-plans/wifconfigtemplate-catalog-and-iam-lifecycle.md)
  and the dependent
  [resource-name-prefix conditions](../../implementation-plans/wifconfigtemplate-resource-name-prefix-conditions.md).
* Related decisions: [workload identity implementation](workload-identity-implementation.md),
  [platform-scoped Cedar authorization](platform-scoped-cedar-authorization.md),
  [gecko API aggregation](gecko-api-aggregation.md),
  [cross-region resource replication](../infrastructure/cross-region-resource-replication.md).
* Tracking: [GCP-944](https://redhat.atlassian.net/browse/GCP-944) (epic),
  [GCP-1505](https://redhat.atlassian.net/browse/GCP-1505) (WP1–3),
  [GCP-1506](https://redhat.atlassian.net/browse/GCP-1506) (WP4/5/9/10),
  [GCP-1507](https://redhat.atlassian.net/browse/GCP-1507) (WP6/7/8),
  [GCP-1508](https://redhat.atlassian.net/browse/GCP-1508) (WP11),
  [GCP-1406](https://redhat.atlassian.net/browse/GCP-1406) (catalog Cedar migration).
