# Gecko: Graduation of Platform API to Dedicated Repository

**Scope**: GCP-HCP

**Date**: 2026-07-21

## Decision

Graduate the Platform API experiment (currently at `gcp-hcp/experiments/platform-api/`) to a dedicated repository at `github.com/openshift-online/gecko`, following the graduation process defined in the [Repository Organization Policy](repository-organization-policy.md).

## Context

The Platform API was validated through two experiments in `experiments/platform-api/`: the `orlop` framework (a kube-like REST API server with dual public/private surfaces, converter layer, and schema-driven code generation) and the `platform-api` application (GCP-specific types such as HostedCluster and NodePool with HyperShift private API types). The [Platform API design decision](platform-api.md) established the architectural direction — a centralized API server replacing the Hyperfleet API / CLM Backend as the single source of truth for the GCP HCP API definition.

- **Problem Statement**: The experiment has outgrown its home in `gcp-hcp/experiments/`. With 219 files and over 35,000 lines of Go code, its own `go.mod` dependency management, and a fundamentally different CI/CD pipeline (Go builds, container image publishing, integration tests), it needs a dedicated repository to support independent releases, proper CI/CD, and team-wide collaboration.
- **Constraints**: The repository must support both the `orlop` framework and the `platform-api` application types. Production deployment targets the `hyperkube` repository; `gecko` houses the API server source and code generation tooling.
- **Assumptions**: The GCP HCP team has Go expertise to maintain the API server. The validated experiments demonstrate readiness for production development.

## Alternatives Considered

1. **Dedicated repository under `openshift-online`**: A standalone `openshift-online/gecko` repository with its own CI/CD, OWNERS file, and branch protections. Provides clear organizational ownership, appropriate pipeline configuration, and satisfies all four graduation criteria.

2. **Co-locate in `gcp-hcp`**: Keep the Platform API source inside the project hub repository. Avoids a new repository, but mixes 40,000+ lines of Go source with documentation, imposes documentation-oriented CI/CD on a Go server project, and contradicts the repository placement guide which directs graduated tooling/services to dedicated repositories.

3. **Co-locate in `hyperkube`**: Place the API server source directly in the production repository. Couples the API definition with the deployment target, but `hyperkube` is an upstream fork with its own release cadence and review processes, making independent API iteration difficult.

## Decision Rationale

* **Justification**: A dedicated repository satisfies all four graduation criteria and aligns with the repository organization policy's placement guide for "Graduated tooling/services." The API server has a distinct release lifecycle (versioned container images, generated client libraries) and a fundamentally different CI/CD pipeline (Go builds, code generation, integration tests) than documentation linting. The codebase has exceeded 35,000 lines of Go with its own dependency management, well past the supporting signal thresholds.

* **Comparison**: Co-locating in `gcp-hcp` (alternative 2) would impose documentation-oriented CI/CD on a Go server project and make independent releases impractical. Co-locating in `hyperkube` (alternative 3) would couple API iteration to an upstream fork's release cadence.

## Graduation Criteria Assessment

This work meets all four graduation criteria defined in the [Repository Organization Policy](repository-organization-policy.md):

| Criterion | Assessment |
|---|---|
| **Independent release lifecycle** | The API server produces versioned container images and generated client libraries released independently from documentation or infrastructure changes. |
| **Distinct CI/CD pipeline** | Go builds, code generation (`orlop-gen`), container image publishing, and integration tests are fundamentally different from documentation linting. |
| **Expected longevity > 6 months** | The Platform API is the long-term replacement for the Hyperfleet API / CLM Backend — the single source of truth for the GCP HCP API definition. |
| **Clear single owner** | The GCP HCP team is the identified owner and will be listed in the OWNERS file. |

Supporting signals satisfied:

- Codebase exceeds 500 lines (35,000+ lines of Go)
- Own dependency management (`go.mod`)
- External consumers will depend on generated client libraries (CLI, controllers)

## Consequences

### Positive

* Organizational ownership, shared access controls, and branch protections from day one
* CI/CD pipeline tailored to Go builds, code generation, and container image publishing
* Independent release lifecycle enables faster API iteration without coupling to documentation or infrastructure changes
* Clear separation between API definition (gecko) and deployment (hyperkube)
* OWNERS file establishes accountability as the Platform API becomes the production API surface

### Negative

* Adds a new repository to the team's portfolio with ongoing maintenance overhead (OWNERS, branch protections, CI/CD)
* Developers must coordinate changes spanning gecko (API types) and hyperkube (deployment) across repositories
