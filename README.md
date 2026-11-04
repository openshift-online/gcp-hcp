# GCP HCP

Documentation, design decisions, and experiments for Hypershift on GCP managed service

## Team Structure

The GCP HCP team is organized into three pods (A, B, C) to improve focus and execution. Pod structure is dynamic through private preview delivery.

**Resources:**
- [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835) — Pod membership, ceremonies, and communication guidelines
- [New Hire Onboarding](https://redhat.atlassian.net/wiki/spaces/GCP/pages/418190496) — Comprehensive onboarding checklist and team resources
- [Using Pod Boards](https://redhat.atlassian.net/wiki/spaces/GCP/pages/506364992) — How to use Quick Filters and Team field automation

## Repository Structure

- `design-decisions/` -- Architecture Decision Records (ADRs) covering all aspects of system design.
- `docs/` -- Jira templates and team processes (definition of done, story/epic/feature templates).
- `experiments/` -- Proofs of concept and research spikes (auth, networking, Terraform tooling, etc.).
- `implementation-plans/` -- Step-by-step plans for delivering approved designs.
- `incidents/` -- Incident reports and postmortems.
- `slo/` -- Service Level Objective definitions and related artifacts.
- `studies/` -- In-depth technical studies and analyses.

## Claude Code Plugin

This repository is also a Claude Code plugin that provides skills for agents working on GCP HCP code across multiple repositories.

### Skills

- **gcp-hcp-architecture** — topic-filtered access to design decisions, implementation plans, and architectural invariants
- **add-gcp-service-account** — step-by-step playbook for adding new GCP service accounts for WIF across all repos
- **ai-sdlc-pilot-update** — biweekly Agentic SDLC pilot reporting for GCP-579
- **fix-implementation-plan-drifts** — reconcile implementation plans with the codebase and repository state
- **bot-pr-review** — first-pass code review plus repository-specific merge-readiness checks for GCP HCP team repositories
- **gcp-hcp-feature-support-assessment** — evidence-based support, scope, and effort assessment across GCP HCP product layers and dependencies

### Installation

**Option 1: Permanent install (recommended)**

Replace `<path-to-local-clone>` with the path to your local clone of this repo (e.g., `~/go/src/github.com/openshift-online/gcp-hcp`):

```bash
claude plugin marketplace add <path-to-local-clone>
claude plugin install gcp-hcp@gcp-hcp
```

Skills are then available in every session, from any repository.

**Updating** (after pulling new changes):

```bash
claude plugin marketplace update gcp-hcp
claude plugin update gcp-hcp@gcp-hcp
```

**Option 2: Per-session**

```bash
claude --plugin-dir <path-to-local-clone>
```

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details.
