# GCP HCP

Documentation, design decisions, and experiments for Hypershift on GCP managed service

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

## Usage

- Browse `design-decisions/` for Architecture Decision Records
- Review `implementation-plans/` for step-by-step delivery plans
- Run experiments from the `experiments/` directory

To use the Claude Code plugin, see the [Installation section](#claude-code-plugin) above.

## Development

Contributions follow the standard GitHub fork-and-PR workflow:

1. Fork this repository
2. Create a feature branch
3. Add your design decision, experiment, or documentation
4. Submit a pull request for review
