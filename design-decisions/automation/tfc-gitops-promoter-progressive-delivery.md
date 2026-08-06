# TFC Workspaces Must Track GitOps Promoter Environment Branches for Progressive Terraform Delivery

***Scope***: GCP-HCP

**Date**: 2026-08-06

## Decision

TFC workspaces that manage infrastructure must track GitOps Promoter environment branches (e.g., `environment/global-integration`, `environment/sector-stage-main`) instead of `main`. Workspaces retain `trigger_prefixes` (or equivalent `trigger_patterns`) for shared terraform directories outside `working_directory`. Speculative terraform plans on PRs to `main` are triggered via a Prow presubmit that runs `terraform plan -refresh=false` using CLI remote execution against VCS-connected workspaces.

## Context

- **Problem Statement**: The current TFC workspace setup (GCP-532) has workspaces triggered by pushes to `main`. This means terraform plan/apply runs as soon as code is merged, bypassing the GitOps Promoter progressive promotion flow (integration -> stage -> production). Terraform changes need the same promotion gating that ArgoCD changes already have. Additionally, PR authors and reviewers cannot see `terraform plan` output before merging because TFC speculative plans on `main` show the plan against the wrong environment state.

- **Constraints**:
  - TFC VCS-driven workspaces can only track one branch per workspace
  - The hydration pipeline copies terraform content (configs, modules, metadata, workflows, dashboards) to environment branches preserving the directory structure, so `working_directory` paths remain valid
  - Speculative plans via TFC Runs API require a valid TFC API token -- the existing `tfcloud-ci-secret` in Prow CI provides this
  - Environment branches are managed by the `gcp-hcp-gitops-promoter` GitHub App and branch-protected -- Prow CI jobs cannot push to them
  - PR plans must not interfere with in-flight promotions (state lock contention, stale state)

- **Assumptions**:
  - The upstream `workspaces/tfe` module supports (or will be extended to support) a `vcs_branch` parameter
  - The hydration pipeline will continue to copy all terraform directories needed for `terraform init` to succeed on environment branches (modules, metadata, workflows, dashboards, config)
  - Prow presubmit jobs can make outbound HTTPS calls to `app.terraform.io` from the `build06` cluster

## Alternatives Considered

1. **VCS-driven on environment branches**: Configure each TFC workspace with `vcs_repo.branch` set to the corresponding environment branch. TFC watches the branch and auto-triggers plan/apply on promotion merges. PR speculative plans are triggered separately via the TFC Runs API from a Prow job.

2. **API-driven runs on promotion merge**: Keep workspaces with no VCS trigger. Add a GitHub Actions step to the promotion workflow (or a new workflow) that calls the TFC Runs API after a promotion PR merges. This gives more control over when runs start but requires maintaining API-driven trigger logic and loses TFC's native VCS integration features (auto-queue, auto-cancel superseded runs).

3. **CLI-driven from hydration pipeline**: Keep CLI-driven execution mode. Add `terraform plan` and `terraform apply` commands to the hydration pipeline GitHub Actions workflows. This bypasses TFC entirely for the promotion path, reducing it to a state management tool. Loses TFC's run queue, locking, Sentinel policies, approval gates, and audit trail.

4. **Dual-branch VCS on `main` with path scoping**: Configure workspaces to track `main` with `trigger_prefixes` scoped to environment-specific paths. This does not work because environment-specific terraform configs for all environments exist under the same directory tree on `main` (e.g., `terraform/config/global/integration/` and `terraform/config/global/stage/` are both on `main`). A change to shared modules would trigger all environments simultaneously.

## Decision Rationale

* **Justification**: Alternative 1 (VCS-driven on environment branches) is the natural fit. The hydration pipeline already copies all terraform content to environment branches with the same directory structure. TFC's native VCS integration handles change detection, run queuing, auto-cancel of superseded runs, and GitHub check status integration. No custom trigger logic is needed on the apply path. The speculative plan via TFC Runs API is a well-documented TFC feature.

* **Evidence**: The hydration pipeline (`hydrate-global.yaml`, `hydrate-sectors.yaml`) already copies `terraform/config/{cluster-type}/{env}/{sector}/`, `terraform/modules/`, `terraform/metadata/`, `terraform/workflows/`, and `terraform/dashboards/` to environment branches. The `working_directory` values in `hcp-terraform/gcp-hcp-integration/main.tf` (e.g., `terraform/config/global/integration/main/us-central1`) are exactly the paths that exist on the hydrated branches.

* **Comparison**: Alternative 2 (API-driven) adds maintenance burden and custom orchestration without clear benefit over TFC's native VCS capabilities. Alternative 3 (CLI-driven) loses TFC's core value proposition (Sentinel policies, approval gates, run queue, audit trail). Alternative 4 (dual-branch on `main`) does not work architecturally because shared module changes would trigger all environments simultaneously.

## Consequences

### Positive

* Terraform changes flow through the same promotion pipeline as ArgoCD changes -- unified progressive delivery for all infrastructure changes
* TFC auto-triggers plan/apply when promoter merges to active branches -- no custom orchestration needed
* Speculative plans on PRs give reviewers terraform plan output before merge, replacing the visibility Atlantis provided
* `refresh: false` on PR plans eliminates state contention with running promotions
* TFC's native GitHub check integration provides plan/apply status directly on promotion PRs

### Negative

* Each workspace must specify a `vcs_branch`, making workspace definitions slightly more complex
* The upstream `workspaces/tfe` module may need a `vcs_branch` parameter if it does not already support one
* Speculative plans against `main` code (not yet promoted) may show drift from the environment's actual current state -- this is expected and acceptable since they serve as a "what would happen" preview, not an exact state comparison
* Adding a new environment branch requires both the promoter config update AND the TFC workspace branch update -- two-place coordination
* The branch cutover requires a controlled procedure: disable auto-apply, drain/cancel queued `main` runs, switch `vcs_branch`, verify no `main`-triggered run can apply, then re-enable auto-apply (see implementation plan Phase 2 for the detailed procedure)

## Cross-Cutting Concerns

### Security

Two separate trust boundaries are involved:

* **Prow -> TFC (API token)**: `tfcloud-ci-secret` is a TFC team or user API token that authenticates Prow CI to HCP Terraform for creating speculative plans. This token must have `Plan runs` and `Read workspace` permissions on the target workspaces. It cannot trigger applies on VCS-connected workspaces (VCS merge is the only apply path). Token ownership, rotation, and revocation follow the existing CI secret management procedures documented in `secrets/inventory.yaml`.
* **TFC -> GCP (WIF)**: During plan/apply execution, TFC uses Workload Identity Federation to authenticate to GCP. The WIF pool in the TFC access project issues short-lived credentials scoped to the workspace's plan or apply SA. No static GCP credentials are involved.
* Environment branches are push-protected to the `gcp-hcp-gitops-promoter` GitHub App, preventing unauthorized terraform applies. The TFC API token cannot trigger applies on VCS-connected workspaces -- only VCS merges can.

### Reliability

* **Resiliency**: If the hydration pipeline fails to copy terraform content to an environment branch, TFC will not trigger (no commit to detect). The promoter's existing `argocd-health` commit status gate blocks promotion if ArgoCD cannot sync, but does not guarantee terraform ran successfully. For promotions that include terraform content changes, a TFC apply commit status gate should be mandatory to prevent silent promotion without terraform execution. This gate is deferred to Phase 4 in the implementation plan but should be treated as required before promoting terraform-bearing changes to stage/production.
* **Observability**: TFC provides native run history and plan/apply logs per workspace. Speculative plan results are posted as GitHub check runs on the PR. Promotion merges trigger TFC runs visible in the TFC UI with full audit trail.

### Cost

* No additional TFC cost -- speculative plans are included in the TFC plan tier
* Prow job execution cost is negligible (lightweight API call per PR commit per affected workspace)

### Operability

* Adding a new workspace: specify `vcs_branch` in addition to existing parameters (`working_directory`, `github_repo_org`, `github_repo_name`)
* Adding a new environment: create environment branches (documented in `docs/promotions.md`), add workspace definitions with the branch, update Prow config for speculative plans if needed
* Debugging failed applies: TFC UI provides detailed plan/apply logs; run history shows the exact commit that triggered the run

---

## Open Items

### Resolved

* **TFC API token availability**: Confirmed -- `tfcloud-ci-secret` is mounted in Prow CI jobs `terraform-validate` and `terraform-test` (see `ci-operator/config/openshift-online/gcp-hcp-infra/openshift-online-gcp-hcp-infra-main.yaml` in `openshift/release`)
* **Environment branch naming**: Confirmed -- `environment/global-{env}` for global, `environment/sector-{env}-{sector}` for region/MC (see `core-services/prow/02_config/openshift-online/gcp-hcp-infra/_prowconfig.yaml` in `openshift/release` for branch protection config)

### Pending Validation

* **Hydration content completeness**: The hydration workflows (`.github/workflows/hydrate-global.yaml`, `hydrate-sectors.yaml`) copy `terraform/config/`, `terraform/modules/`, `terraform/metadata/`, `terraform/workflows/`, and `terraform/dashboards/` to environment branches. Requires executable verification before Phase 2: checkout an environment branch and run `terraform init -backend=false` in each `working_directory` path to confirm all module sources resolve. Command: `git checkout environment/global-integration && cd terraform/config/global/integration/main/us-central1 && terraform init -backend=false`
* **TFC API token permissions**: Verify that `tfcloud-ci-secret` has `Plan runs` permission on the `gcp-hcp-integration` project workspaces (required for CLI remote speculative plans on VCS-connected workspaces). Test: `curl -s -H "Authorization: Bearer $(cat /etc/terraform-cloud/token)" https://app.terraform.io/api/v2/organizations/hp-platform-engineering/workspaces`

### Future Considerations

* **Stage and production workspaces**: This design applies to integration first. Stage and production workspaces will follow the same pattern when TFC access projects are provisioned for those environments.
* **TFC apply status as promoter gate**: Deferred from initial integration rollout but required before stage/production. The promoter already gates on `argocd-health`, but this alone does not guarantee terraform ran. For terraform-bearing promotions, a `tfc-apply` commit status gate is necessary to prevent silent promotion without terraform execution. See Phase 4 in the implementation plan.
* **Terraform module git sources**: Migrating terraform module references from relative paths to `git::` sources pointing at `dry_sha` would eliminate file copying for modules, metadata, workflows, and dashboards. This is tracked separately in `docs/promotions.md`.

## JIRA Tracking

This design covers two independent workstreams, tracked as separate stories under the same epic:

| Story | Scope | Prerequisite |
|---|---|---|
| [GCP-1006](https://redhat.atlassian.net/browse/GCP-1006) | Prow presubmit for speculative plans on PRs | None -- works against existing workspaces on `main` |
| [GCP-985](https://redhat.atlassian.net/browse/GCP-985) | Progressive delivery -- TFC workspaces track environment branches | All workspaces ported to TFC |

GCP-1006 was split from GCP-985 because speculative plans are independent of which branch workspaces track. The Prow job can run immediately against the current integration workspaces while the progressive delivery branch migration is blocked until all workspaces are ported.

## References

* [GCP-985](https://redhat.atlassian.net/browse/GCP-985) -- Progressive delivery (workspace branch tracking)
* [GCP-1006](https://redhat.atlassian.net/browse/GCP-1006) -- Prow speculative plans on PRs
* [GCP-532](https://redhat.atlassian.net/browse/GCP-532) -- Parent epic (Terraform Cloud Evaluation & Plan)
* `gcp-532-atlantis-to-tfc-cutover.md` -- TFC cutover implementation plan
* `hcp-terraform-workload-identity-federation.md` -- TFC WIF design decision
* `docs/promotions.md` -- GitOps Promoter promotion model
* [TFC VCS-Driven Runs](https://developer.hashicorp.com/terraform/cloud-docs/run/ui) -- TFC VCS workflow documentation
* [TFC Runs API](https://developer.hashicorp.com/terraform/cloud-docs/api-docs/run) -- Speculative plan API
