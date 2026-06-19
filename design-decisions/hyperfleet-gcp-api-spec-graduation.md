# hyperfleet-gcp-api-spec: Graduation to Dedicated Repository

**Scope**: GCP-HCP

**Date**: 2026-06-19

## Decision

Create a dedicated GCP HCP provider repository at `github.com/openshift-online/hyperfleet-gcp-api-spec` based on the upstream HyperFleet API spec template (`github.com/openshift-hyperfleet/hyperfleet-api-spec-template`), following the graduation process defined in the [Repository Organization Policy](repository-organization-policy.md).

## Context

The HyperFleet API spec template (`github.com/openshift-hyperfleet/hyperfleet-api-spec-template`) is designed to be used as a starting point by each provider to define their own `ClusterSpec` and `NodePoolSpec` using TypeSpec. The CLM team owns the API surface — routes, pagination, status models — but leaves the `spec` types undefined as provider extension points. GCP HCP is responsible for defining what goes inside the `spec` field.

Running `tsp compile` in the provider repo merges the CLM-owned routes with the GCP-specific schema definitions into a single self-contained `openapi.yaml`, serving three purposes:

- **API Gateway config** — `gcp-hcp-infra` embeds it in the `APIGatewayAPIConfig` for routing and authentication at the user-facing gateway layer
- **API validation** — `gcp-hcp-infra` mounts it as a Kubernetes ConfigMap for `hyperfleet-api` to validate GCP `spec` payloads at the HTTP layer
- **Go type generation** — `gcphcpctl` uses `oapi-codegen` to generate typed Go structs from it

**Problem Statement**: Without a dedicated repository, the GCP-specific TypeSpec schema has no organizational home, no CI/CD pipeline to validate `tsp compile` on each change, and no versioning mechanism for the `openapi.yaml` artifact that consumers depend on.

## Alternatives Considered

1. **Co-locate TypeSpec sources in `gcp-hcp-infra`**: Keep the TypeSpec model alongside the Helm charts that consume the generated schema. Avoids a new repository but mixes schema authoring (npm/TypeSpec toolchain) with infrastructure deployment (Terraform/Helm), imposes incompatible CI/CD requirements on a single repository, and makes it harder for `gcphcpctl` to reference a clean schema artifact with its own version history.

2. **Dedicated repository under `openshift-online`**: A standalone `openshift-online/hyperfleet-gcp-api-spec` repository initialized from the upstream template, with its own npm toolchain, CI/CD for TypeSpec compilation, and versioned `openapi.yaml` artifacts. Provides clear organizational ownership, appropriate pipeline configuration, and satisfies all four graduation criteria.

## Decision Rationale

* **Justification**: A dedicated repository is the right home for this work because the TypeSpec compilation pipeline (npm, `tsp compile`) is fundamentally different from the Terraform/Helm pipelines in `gcp-hcp-infra`. The schema has two external consumers (`gcp-hcp-infra` and `gcphcpctl`) that benefit from a clearly versioned artifact with its own commit history.
* **Evidence**: The GCP-836 spike confirmed end-to-end that the generated `openapi.yaml` is sufficient for both API validation and Go type generation (`oapi-codegen` produces clean, compiling structs). See [studies/GCP-836-findings.md](../studies/GCP-836-findings.md).
* **Comparison**: Co-locating the TypeSpec sources in `gcp-hcp-infra` would create CI/CD complexity and make schema versioning impractical.

## Graduation Criteria Assessment

This work meets all four graduation criteria defined in the [Repository Organization Policy](repository-organization-policy.md):

| Criterion | Assessment |
|---|---|
| **Independent release lifecycle** | The repo produces versioned `openapi.yaml` artifacts consumed independently by `gcp-hcp-infra` (API validation) and `gcphcpctl` (Go type generation). Schema changes are released on their own cadence, triggered by upstream CLM spec updates or GCP-specific model changes. |
| **Distinct CI/CD pipeline** | TypeSpec compilation (`tsp compile`) and OpenAPI schema generation require the npm toolchain — fundamentally different from documentation linting, Terraform validation, or Go builds. |
| **Expected longevity > 6 months** | The schema evolves alongside the GCP HCP platform for as long as the platform operates, tracking upstream CLM spec releases and expanding to cover new GCP-specific capabilities indefinitely. |
| **Clear single owner** | The GCP HCP team owns and maintains the schema. The team will be listed in the OWNERS file. |

## Consequences

### Positive

* Schema has a clear organizational home with proper access controls and branch protections
* CI/CD is tailored to the npm/TypeSpec toolchain, not shared with incompatible pipelines
* Versioned schema artifacts give all consumers a stable, auditable source of truth
* Provides a path to consolidate the hand-maintained Swagger 2.0 spec in `api-config.yaml` (API Gateway) into the TypeSpec-generated `openapi.yaml`, eliminating schema drift across consumers

### Negative

* Adds a new repository to the team's portfolio with ongoing maintenance overhead (OWNERS, branch protections, CI/CD)
