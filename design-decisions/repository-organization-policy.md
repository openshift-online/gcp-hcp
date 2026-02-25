# Repository Organization Policy: Three-Tier Structure with Graduation Criteria

***Scope***: GCP-HCP

**Date**: 2026-02-25

## Decision

We will adopt a three-tier repository structure: `gcp-hcp` as the project hub for documentation, architecture, and time-bounded experiments; `gcp-hcp-infra` for all infrastructure-as-code and deployment artifacts; and dedicated repositories only when explicit graduation criteria are met.

## Context

The team lacks formal criteria for when to create new repositories, leading to repeated debates when new work begins. Two valid perspectives emerged from team discussion: minimize repositories by defaulting everything into `gcp-hcp` subfolders unless technical or policy reasons justify separation, versus separating concerns so that `gcp-hcp` stays focused on documentation and architecture while code projects with distinct pipelines get their own repositories.

- **Problem Statement**: Without clear repository creation criteria, the team wastes time debating where new work should live and risks either repository sprawl (too many small repos with overhead) or concern dilution (a single repo trying to serve too many purposes).
- **Constraints**: Small team size means per-repo overhead (OWNERS files, CI/CD pipelines, branch protections, onboarding) has a proportionally higher cost. The `gcp-hcp-infra` repository is already established for deployment artifacts including Terraform, Helm charts, ArgoCD manifests, and bootstrap manifests.
- **Assumptions**: CI/CD pipelines differ meaningfully per deliverable type (doc linting vs. container builds vs. Terraform validation). Upstream contributions to external projects (e.g., HyperShift) are out of scope for this policy. The team will continue to grow and onboard new members who benefit from clear navigation.

## Alternatives Considered

1. **Monorepo**: All code, infrastructure, and documentation in `gcp-hcp` subfolders. Simple navigation but mixes concerns, creates CI/CD complexity, and makes OWNERS granularity difficult.
2. **Liberal repo creation**: Every new deliverable gets its own repository from day one. Clear separation but creates administrative overhead, fragments team attention, and makes cross-cutting changes harder.
3. **Three-tier with graduation criteria**: Experiments start in `gcp-hcp`, infrastructure lives in `gcp-hcp-infra`, and work graduates to a dedicated repository only when explicit criteria are met. Balances simplicity with separation.

## Decision Rationale

* **Justification**: The three-tier approach honors both perspectives from the team discussion. Defaulting new work to `experiments/` satisfies the "minimize repos" principle by keeping everything consolidated until there is a clear reason to split. Graduation criteria satisfy the "separate concerns" principle by providing an objective, repeatable mechanism for determining when separation is warranted. Deployment artifacts (Helm charts, ArgoCD manifests, Terraform modules, bootstrap manifests) belong in `gcp-hcp-infra` because mixing deployable infrastructure with documentation creates confusion about repository purpose and ownership.
* **Evidence**: Industry practice demonstrates that mixed-concern repositories create confusion about purpose, complicate CI/CD pipelines, and make OWNERS/CODEOWNERS files unwieldy. The team has already naturally adopted this pattern by establishing `gcp-hcp-infra` as a separate repository for infrastructure-as-code.
* **Comparison**: The monorepo approach would force all CI/CD pipelines into a single repository, creating complexity and blast radius concerns. Liberal repo creation would impose administrative overhead disproportionate to team size and fragment attention across many repositories with minimal content.

## Graduation Criteria

All four criteria must be true for work to graduate from `gcp-hcp/experiments/` to a dedicated repository:

1. **Independent release lifecycle** — The deliverable needs its own versioned releases (tags, container images, or binaries) separate from the project documentation.
2. **Distinct CI/CD pipeline** — The deliverable requires build, test, and deploy processes fundamentally different from documentation linting (e.g., Go builds, container image publishing, Terraform validation).
3. **Expected longevity > 6 months** — The work is not a time-bounded spike or proof-of-concept. It is expected to be maintained and evolved over the medium to long term.
4. **Clear single owner** — An identified maintainer is willing to be listed in an OWNERS file and take responsibility for the repository's health.

Supporting signals that strengthen the case for graduation (but are not required):

- External consumers depend on the deliverable
- Codebase exceeds 500 lines of code
- Deliverable has its own dependency management (go.mod, requirements.txt)
- Separate security scanning or compliance requirements apply

## Repository Placement Guide

| Content Type | Target Repository | Rationale |
|---|---|---|
| Design decisions | `gcp-hcp/design-decisions/` | Core project documentation |
| Architecture diagrams | `gcp-hcp/architecture/` | Core project documentation |
| Meeting notes, runbooks | `gcp-hcp/docs/` | Core project documentation |
| Jira templates, process docs | `gcp-hcp/docs/` | Core project documentation |
| Time-bounded spikes/PoCs | `gcp-hcp/experiments/` | Low-friction starting point; graduate when criteria met |
| Terraform modules | `gcp-hcp-infra` | Infrastructure-as-code |
| Helm charts | `gcp-hcp-infra` | Deployment artifacts |
| ArgoCD manifests | `gcp-hcp-infra` | GitOps deployment configuration |
| Bootstrap manifests | `gcp-hcp-infra` | Cluster bootstrap configuration |
| Graduated tooling/services | Dedicated repository | Met all four graduation criteria |

## Experiments Directory Policy

### Lifecycle

1. **Start**: Create a subdirectory under `experiments/` with a README explaining the experiment's purpose and link to a Jira ticket tracking the work.
2. **Execute**: Develop the spike or proof-of-concept within the experiment directory. Follow standard PR review processes.
3. **Conclude**: When the experiment is complete, either graduate the work to a dedicated repository (if graduation criteria are met) or archive it by adding an `ARCHIVED.md` file noting the outcome and date.

## Lightweight Governance

1. **Default location**: All new work starts in `gcp-hcp/experiments/` unless it is infrastructure-as-code (which goes to `gcp-hcp-infra`) or documentation (which goes to the appropriate `gcp-hcp` directory).
2. **Graduation process**: A PR is submitted to `gcp-hcp` documenting the graduation rationale (e.g., as a design decision or amendment to this document), explaining how the new repository satisfies each graduation criterion.
3. **Approval**: Two approvers listed in the OWNERS file must approve the graduation PR.
4. **New repository checklist**:
   - README with purpose, quick-start, and links back to `gcp-hcp` design decisions
   - OWNERS file with at least one identified maintainer
   - CLAUDE.md with repository-specific agent instructions
   - Branch protection enabled on the main branch
   - CI/CD pipeline configured and passing

## Consequences

### Positive

* Clear, repeatable criteria eliminate ad-hoc debates about repository placement
* Default-to-experiments minimizes repository sprawl and administrative overhead
* Graduation rationale documented via PR provides an auditable record of repository creation decisions
* Graduation criteria ensure new repositories have sufficient justification and ownership before creation
* Alignment with the team's existing pattern of using `gcp-hcp-infra` for infrastructure-as-code

### Negative

* Experiments may accumulate in `gcp-hcp` if the team does not regularly assess and archive completed spikes
* The graduation threshold may delay separating work that would benefit from its own repository
* Requires discipline to follow the governance process rather than creating repositories ad-hoc
* The experiments directory may grow large enough to create noise in the project hub repository

## Cross-Cutting Concerns

### Security:
- Branch protections are justified through the graduation process, ensuring new repositories meet minimum security standards before creation
- Experiments follow the same PR review process as all other changes in `gcp-hcp`, preventing unreviewed code from entering the repository
- The new repository checklist ensures OWNERS files and branch protections are configured from day one

### Cost:
- Fewer repositories reduce CI/CD minutes and GitHub administrative overhead
- Consolidated experiments avoid duplicating CI/CD pipeline configuration across many small repositories
- Graduation criteria prevent the creation of repositories that would incur ongoing maintenance cost without sufficient justification

### Operability:
- Fewer repositories reduce the surface area for maintenance, monitoring, and access management
- Standard new repository checklist ensures consistency across all team repositories
- Clear placement guide reduces cognitive overhead when deciding where new work belongs
