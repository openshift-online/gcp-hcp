# gcphcpctl: Graduation to Dedicated Repository

**Scope**: GCP-HCP

**Date**: 2026-05-19

## Decision

Graduate the GCP HCP CLI (currently at `github.com/ckandag/gcp-hcp-cli`) to a dedicated repository at `github.com/openshift-online/gcp-hcp-ctl`, following the graduation process defined in the [Repository Organization Policy](repository-organization-policy.md).

## Context

`gcp-hcp-ctl` is the designated CLI tool for the GCP HCP platform, currently providing the `ops` subcommand. It is being graduated from a personal GitHub account (`github.com/ckandag/gcp-hcp-cli`) to establish a proper organizational home. As the platform transitions from the legacy CLS system to CLM (Hyperfleet), `gcp-hcp-ctl` will replace both the existing Python CLI (a separate tool at `github.com/apahim/gcp-hcp-cli`) and the ad-hoc scripts currently used for Hyperfleet API interactions.

- **Problem Statement**: Without a dedicated repository, `gcp-hcp-ctl` lacks proper access controls and a team-managed CI/CD pipeline to grow sustainably.

## Alternatives Considered

1. **Co-locate in `gcp-hcp`**: Host the CLI source inside the project hub repository. Avoids a new repository, but mixes Go source code with documentation, imposes documentation-oriented CI/CD on a Go binary project, and contradicts the repository placement guide which directs graduated tooling to dedicated repositories.

2. **Dedicated repository under `openshift-online`**: A standalone `openshift-online/gcp-hcp-ctl` repository with its own CI/CD, OWNERS file, and branch protections. Provides clear organizational ownership, appropriate pipeline configuration, and satisfies all four graduation criteria.

## Decision Rationale

* **Justification**: A dedicated repository satisfies all four graduation criteria and aligns with the organization policy's placement guide for "Graduated tooling/services." The CLI has a distinct release lifecycle (versioned Go binaries) and a fundamentally different CI/CD pipeline (Go builds, binary publishing) than documentation linting. Co-locating in `gcp-hcp` would impose documentation-oriented CI/CD on a Go binary project and make independent releases impractical.

* **Comparison**: Co-locating in `gcp-hcp` (alternative 1) would make the independent release lifecycle criterion impossible to satisfy in practice — `gcp-hcp` has no versioned release mechanism — and mixes Go source code with a documentation-oriented repository.

## Graduation Criteria Assessment

This work meets all four graduation criteria defined in the [Repository Organization Policy](repository-organization-policy.md):

| Criterion | Assessment |
|---|---|
| **Independent release lifecycle** | The CLI produces versioned Go binaries released independently from documentation or infrastructure changes. |
| **Distinct CI/CD pipeline** | Go builds, cross-platform binary publishing, and Go test runs are fundamentally different from documentation linting or Terraform validation. |
| **Expected longevity > 6 months** | The CLI is built for the long term — serving as the platform's interface to CLM as CLS is deprecated and Hyperfleet becomes the primary system. |
| **Clear single owner** | The GCP HCP team is the identified owner and will be listed in the OWNERS file. |

## Consequences

### Positive

* Organizational ownership, shared access controls, and branch protections from day one
* CI/CD pipeline is tailored to Go builds and binary publishing rather than documentation linting
* Clear OWNERS file establishes accountability as Hyperfleet API integration expands

### Negative

* Adds a new repository to the team's portfolio with ongoing maintenance overhead (OWNERS, branch protections, CI/CD)
