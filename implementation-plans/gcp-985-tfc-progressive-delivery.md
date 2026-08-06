# TFC Progressive Delivery and Speculative Plans

## Overview

Two independent workstreams under [GCP-532](https://redhat.atlassian.net/browse/GCP-532):

1. **Speculative plans on PRs** ([GCP-1006](https://redhat.atlassian.net/browse/GCP-1006)) -- Prow presubmit that runs `terraform plan -refresh=false` on PRs, restoring the plan visibility Atlantis provided. **Can start immediately** against existing workspaces on `main`.
2. **Progressive delivery** ([GCP-985](https://redhat.atlassian.net/browse/GCP-985)) -- TFC workspaces track GitOps Promoter environment branches so terraform changes flow through the same promotion pipeline as ArgoCD. **Blocked** until all workspaces are ported to TFC.

| | |
|---|---|
| **Epic** | [GCP-532](https://redhat.atlassian.net/browse/GCP-532) -- Terraform Cloud Evaluation & Plan |
| **GCP-1006** | Prow presubmit for speculative Terraform plans on PRs |
| **GCP-985** | Wire up GitOps Promoter with TFC for progressive delivery |
| **Design Decision** | [tfc-gitops-promoter-progressive-delivery](../design-decisions/automation/tfc-gitops-promoter-progressive-delivery.md) |

### Current Architecture

```
Developer -> PR to main -> merge -> hydration pipeline -> environment/-next branches
                                                             |
                                                   gitops-promoter creates PR
                                                             |
                                                   environment/active branches
                                                             |
                                                   ArgoCD syncs (helm/kustomize)
                                                             |
                                              TODAY: Terraform NOT triggered here
                                              GOAL:  TFC auto plan/apply here
```

### Target Architecture

```
Developer -> PR to main -----> Prow: speculative terraform plan (refresh=false)
                |                    posts plan output to PR
                v
             merge to main
                |
                v
          hydration pipeline -> environment/-next branches
                                       |
                              gitops-promoter creates PR
                                       |
                              environment/active branches
                                       |
                         +-------------+-------------+
                         |                           |
                    ArgoCD syncs              TFC auto plan/apply
                  (helm/kustomize)           (VCS-driven on branch)
                         |                           |
                  argocd-health              tfc-apply status
                  commit status              commit status (future)
                         |                           |
                         +-------------+-------------+
                                       |
                              promoter gates on commit
                              statuses before promoting
                              to next environment
```

### Workspace-to-Branch Mapping (Integration)

| TFC Workspace | Environment Branch | Working Directory |
|---|---|---|
| `gcp-hcp-global-integration` | `environment/global-integration` | `terraform/config/global/integration/main/us-central1` |
| `gcp-hcp-region-int-main-us-central1` | `environment/sector-integration-main` | `terraform/config/region/integration/main/us-central1` |
| `gcp-hcp-mc-int-main-us-central1-yjiv` | `environment/sector-integration-main` | `terraform/config/management-cluster/integration/main/us-central1-yjiv` |

---

# Workstream A: Speculative Plans on PRs (GCP-1006)

This workstream can proceed immediately -- it does not depend on the progressive delivery branch migration.

## Phase 1: Add Prow Presubmit for Speculative Plans

**Summary**: Create a Prow presubmit job that triggers speculative `terraform plan` runs on PR commits using CLI remote execution against VCS-connected TFC workspaces.

**Repos**: `openshift/release` (ci-operator config), `gcp-hcp-infra` (plan script)
**Depends on**: None -- works against existing integration workspaces on `main`

### Execution Mode: CLI Remote Execution

TFC supports CLI-driven speculative plans on VCS-connected workspaces. When `terraform plan` is run from the CLI against a workspace with a `cloud` backend block, TFC creates a configuration version from the local files and executes a speculative (plan-only) run remotely. This is the simplest approach -- no configuration version tarball upload or TFC Runs API orchestration required.

Key characteristics:
- Local terraform files are uploaded to TFC automatically
- TFC executes the plan remotely using the workspace's WIF credentials
- The plan is speculative (read-only) -- it cannot be promoted to apply
- `-refresh=false` is passed through to the remote execution
- The plan output streams back to the CLI stdout

**Tasks**:

### Plan Script (`gcp-hcp-infra`)

- [ ] Create `hack/tfc-speculative-plan.sh`:
  1. Read TFC API token from `/etc/terraform-cloud/token`
  2. Determine affected integration workspaces from the PR's changed files:
     - `terraform/config/global/integration/` -> `gcp-hcp-global-integration`
     - `terraform/config/region/integration/` -> `gcp-hcp-region-int-main-us-central1`
     - `terraform/config/management-cluster/integration/` -> `gcp-hcp-mc-int-main-us-central1-yjiv`
     - `terraform/modules/global/` -> `gcp-hcp-global-integration`
     - `terraform/modules/region/` -> all region integration workspaces
     - `terraform/modules/management-cluster/` -> all MC integration workspaces
     - `terraform/metadata/`, `terraform/workflows/`, `terraform/dashboards/` -> all integration workspaces
     - Changes only to non-integration paths (e.g., `terraform/config/global/stage/`) -> skip with a message "No integration workspaces affected"
  3. For each affected workspace:
     ```bash
     cd terraform/config/<working_directory_path>
     TF_TOKEN_app_terraform_io=$(cat /etc/terraform-cloud/token) \
     terraform init -input=false
     terraform plan -refresh=false -input=false
     ```
  4. Exit with the worst exit code across all workspace plans (0 = clean, 1 = error, 2 = diff)
- [ ] Add a `Makefile` target `terraform-plan-speculative` that wraps the script
- [ ] Test locally with a sample PR diff

### Presubmit Result Contract

- **Reporter**: Prow ci-operator posts the result as a GitHub check run named `ci/prow/terraform-plan` on the PR's head SHA against `main`
- **Success (exit 0)**: Plan shows no changes -- check run is green
- **Changes detected (exit 2)**: Plan shows infrastructure diff -- check run is green (changes are expected on PRs). Plan output is available in the Prow job logs.
- **Error (exit 1)**: Plan failed (syntax error, init failure, TFC auth failure) -- check run is red
- **No workspace affected**: If the PR only changes non-integration terraform paths (stage, production) or non-terraform files that passed `run_if_changed`, the script exits 0 with a message "No integration workspaces affected by this PR"
- **PR and SHA association**: Prow automatically associates the check run with the PR's head commit SHA on `main`. No custom GitHub API calls needed.

### CI Operator Config (`openshift/release`)

- [ ] Add a new test step `terraform-plan` in `ci-operator/config/openshift-online/gcp-hcp-infra/openshift-online-gcp-hcp-infra-main.yaml`:
  ```yaml
  - as: terraform-plan
    steps:
      test:
      - as: plan
        commands: |
          git config --global url."https://github.com/".insteadOf "git@github.com:"
          git config --global credential.helper '!f() {
            if [ "$1" = "get" ]; then
              read -r line
              case "$line" in
                *host=github.com*) echo username=x-access-token; echo "password=$(cat /etc/github-private/oauth)" ;;
                *) exit 1 ;;
              esac
            fi
          }; f'
          cat > "$HOME/.terraformrc" <<TFRC
          credentials "app.terraform.io" {
            token = "$(cat /etc/terraform-cloud/token)"
          }
          TFRC
          make terraform-plan-speculative
        credentials:
        - mount_path: /etc/github-private
          name: github-credentials-openshift-ci-robot-private-git-cloner
          namespace: ci
        - mount_path: /etc/terraform-cloud
          name: tfcloud-ci-secret
          namespace: ci
        from: src
        resources:
          requests:
            cpu: "8"
            memory: 4Gi
  ```
- [ ] Set `run_if_changed` to trigger only on integration and shared terraform paths:
  ```yaml
  run_if_changed: '^terraform/(config/(global|region|management-cluster)/integration/|modules/|metadata/|workflows/|dashboards/)'
  ```

**Acceptance Criteria**:
- [ ] PR touching `terraform/config/global/integration/` triggers a speculative plan against `gcp-hcp-global-integration`
- [ ] PR touching `terraform/modules/region/` triggers plans for all region integration workspaces
- [ ] PR touching only `terraform/config/global/stage/` does not trigger the job (filtered by `run_if_changed`)
- [ ] Plan output is visible in the Prow job logs, accessible from the `ci/prow/terraform-plan` check run on the PR
- [ ] Plan does not perform state refresh (`-refresh=false`)
- [ ] Plan does not block or interfere with in-flight promotion applies (speculative plans are read-only)
- [ ] Job exits 0 with a message when no integration workspace is affected

**Design Notes**:
- **Workspace discovery**: The script uses a static mapping from changed file paths to workspace names. Dynamic discovery via TFC API (list workspaces, match by `working_directory`) is a future enhancement to avoid config drift.
- **Which workspaces**: Only integration workspaces -- stage/production plans require separate credentials and are a future enhancement.
- **Credential helper scoping**: The Git credential helper is scoped to `github.com` only. It rejects credential requests for other hosts to prevent a malicious Terraform module source from exfiltrating the GitHub token.
- **Why CLI over TFC Runs API**: CLI remote execution is simpler (no tarball upload, no polling for run completion, no plan output parsing). TFC handles configuration version creation and execution automatically. The API approach (`POST /api/v2/runs` with `plan_only: true`, `refresh: false`) is documented as a fallback if CLI limitations are hit.

## Phase 2: Update Branch Protection for Speculative Plans

**Summary**: Add `ci/prow/terraform-plan` to `main` branch status checks.

**Repo**: `openshift/release`
**File**: `core-services/prow/02_config/openshift-online/gcp-hcp-infra/_prowconfig.yaml`
**Depends on**: Phase 1

**Tasks**:
- [ ] Remove remaining Atlantis status checks from `main` branch required status checks (PR #83051 removes `atlantis-int/plan` and `atlantis-int/apply`; stage checks should also be removed when stage moves to TFC)
- [ ] Optionally add `ci/prow/terraform-plan` to `main` branch required status checks if the speculative plan should be required before merge

**Acceptance Criteria**:
- [ ] Speculative plan check runs appear on PRs to `main` (when terraform files are changed)

---

# Workstream B: Progressive Delivery (GCP-985)

This workstream is **blocked until all workspaces are ported to TFC**. It cannot proceed until the workspace migration from the GCP-532 cutover plan is complete.

## Phase 3: Extend Upstream Workspaces Module for Branch Support

**Summary**: Verify or add `vcs_branch` parameter to the upstream `workspaces/tfe` module.

**Repo**: `infra-platform` (upstream module at `app.terraform.io/hp-platform-engineering/workspaces/tfe`)

**Tasks**:
- [ ] Check if `workspaces/tfe` module (v0.0.11) already supports a `vcs_branch` parameter in its workspace object type
- [ ] If not: add `vcs_branch` (optional string, defaults to null) to the workspace object, passing it through to `tfe_workspace.vcs_repo.branch`
- [ ] Publish new module version
- [ ] Test: create a workspace with `vcs_branch = "test-branch"` and verify TFC tracks that branch

**Acceptance Criteria**:
- [ ] Workspace objects accept a `vcs_branch` parameter
- [ ] When set, TFC workspace is configured to track the specified branch
- [ ] When unset, TFC defaults to the repository's default branch (backwards compatible)

---

## Phase 4: Configure Integration Workspaces for Environment Branches

**Summary**: Update integration workspace definitions to track environment branches instead of `main`.

**Repo**: `gcp-hcp-infra`
**File**: `hcp-terraform/gcp-hcp-integration/main.tf`
**Applied by**: TFC meta workspace
**Depends on**: Phase 3, all workspaces ported to TFC

**Tasks**:
- [ ] Bump `workspaces/tfe` module version to the version with `vcs_branch` support
- [ ] Add `vcs_branch` to each workspace definition:

  ```hcl
  gcp-hcp-global-integration = {
    auto_apply        = true
    working_directory = "terraform/config/global/integration/main/us-central1"
    vcs_branch        = "environment/global-integration"
    trigger_prefixes  = ["terraform/metadata/", "terraform/dashboards/global/", "terraform/modules/global/"]
    github_repo_org   = "openshift-online"
    github_repo_name  = "gcp-hcp-infra"
    terraform_version = "1.14.9"
    variables         = []
  }
  ```

- [ ] **Retain `trigger_prefixes`** for shared directories outside `working_directory`. Without them, TFC only detects changes within `working_directory` and will miss changes to shared modules, metadata, workflows, or dashboards. The existing `trigger_prefixes` values are correct and should be kept as-is.
- [ ] Verify `auto_apply = true` so promotion merges automatically apply without manual confirmation
- [ ] Run `terraform plan` on the meta workspace to preview the changes

### Branch Cutover Procedure

The `vcs_branch` change must be applied safely to avoid dual-triggering (runs from both `main` and the environment branch targeting the same state):

1. **Disable auto-apply** on all three workspaces via TFC UI or API
2. **Drain queued runs**: wait for any in-progress `main`-triggered runs to complete; cancel any queued `main`-triggered runs that have not started
3. **Apply the `vcs_branch` change** via the meta workspace (this switches TFC to track the environment branch)
4. **Verify**: confirm TFC shows the environment branch as the tracked branch; confirm no `main`-triggered run can apply
5. **Re-enable auto-apply** on all three workspaces
6. **Trigger a test run**: push a trivial change through the promotion pipeline to verify end-to-end flow

**Acceptance Criteria**:
- [ ] TFC workspaces show the environment branch as the tracked branch in the TFC UI
- [ ] A promotion merge to `environment/global-integration` triggers a TFC plan+apply on `gcp-hcp-global-integration`
- [ ] A promotion merge to `environment/sector-integration-main` triggers TFC plan+apply on both region and MC workspaces
- [ ] Pushes to `main` no longer trigger TFC runs on these workspaces
- [ ] No dual-triggered runs occurred during the cutover

**Notes**:
- Region and MC workspaces both track `environment/sector-integration-main` because sector branches contain content for both cluster types. The `working_directory` filter ensures each workspace only triggers on changes to its own config path.
- `trigger_prefixes` remain necessary because shared terraform directories (`modules/`, `metadata/`, etc.) are outside each workspace's `working_directory`. Without them, a change to a shared module would not trigger a TFC run.

---

## Phase 5: Wire TFC Apply Status into Promoter Commit Status Gates (Deferred from v1, Required for Stage/Production)

**Summary**: Configure GitOps Promoter to gate promotion on TFC apply success.

**Repo**: `gcp-hcp-infra`
**Depends on**: Phase 4

**Tasks**:
- [ ] Determine the exact GitHub check run name that TFC posts on environment branches (e.g., `hashicorp-cloud/plan:gcp-hcp-global-integration` or similar)
- [ ] Add a `WebRequestCommitStatus` or native commit status key for TFC apply in the promoter configuration:
  - **Option A**: If TFC posts a native GitHub commit status, add the status context name to `activeCommitStatuses` in the PromotionStrategy
  - **Option B**: If TFC posts a GitHub check run (not a commit status), create a `WebRequestCommitStatus` that polls the GitHub Checks API for the check run status
- [ ] For global promoter: update `kustomize/gitops-promoter/config-global.yaml` to add the TFC status key
- [ ] For sector promoter: update `helm/charts/gitops-promoter-config-region/values.yaml` to add the TFC status key and update `templates/promotion-strategy.yaml`

**Acceptance Criteria**:
- [ ] Promoter gates promotion on TFC apply success
- [ ] Failed TFC applies block promotion to the next environment
- [ ] Successful TFC applies allow promotion to proceed (combined with existing `argocd-health` and `timer` gates)

**Notes**:
This phase is **deferred from the initial integration rollout** but **required before stage/production promotion**. The promoter already gates on `argocd-health` commit status, but `argocd-health` alone does not guarantee terraform ran. Without a TFC apply gate, terraform-bearing promotions could proceed silently without terraform execution (e.g., if hydration fails to copy terraform content or TFC does not trigger). For the initial integration rollout, manual verification of TFC applies is acceptable. For stage/production, this gate must be in place.

---

## Phase 6: Update Branch Protection for Environment Branches

**Summary**: Ensure Prow branch protection allows TFC check runs on environment branches.

**Repo**: `openshift/release`
**File**: `core-services/prow/02_config/openshift-online/gcp-hcp-infra/_prowconfig.yaml`
**Depends on**: Phase 4

**Tasks**:
- [ ] Verify TFC GitHub App can post check runs on environment branches without being blocked by branch protection (currently only `gcp-hcp-gitops-promoter` has push access; posting check runs requires the TFC GitHub App to be installed on the repo, not push access)

**Acceptance Criteria**:
- [ ] TFC check runs appear on promotion PRs to environment branches

---

## Phase 7: Update Documentation

**Summary**: Update promotions.md and prepare design docs for the gcp-hcp repo.

**Repos**: `gcp-hcp-infra`, `gcp-hcp`

**Tasks**:

### gcp-hcp-infra
- [ ] Update `docs/promotions.md`:
  - Remove "Terraform apply from promotion branches" from Future Considerations (line 269)
  - Add a "Terraform Cloud Integration" section describing:
    - TFC workspaces track environment branches
    - Promotion merges trigger TFC plan/apply
    - TFC apply status gates promotion to next environment (when Phase 5 is implemented)
    - Speculative plans on PRs via Prow
- [ ] Update `CLAUDE.md` if any workflow guidance changes

### gcp-hcp
- [ ] Update design docs and INDEX.md as needed

**Acceptance Criteria**:
- [ ] `docs/promotions.md` accurately describes the TFC integration
- [ ] Design docs in gcp-hcp are up to date

---

## PR Sequence

### Workstream A: Speculative Plans (GCP-1006) -- start now

| # | Phase | PR | Repo | Applied By | Depends On |
|---|-------|-----|------|------------|------------|
| 1a | Speculative plan script | gcp-hcp-infra PR | gcp-hcp-infra | Prow merge | -- |
| 1b | Speculative plan CI config | openshift/release PR | openshift/release | Prow config merge | Phase 1a |
| 2 | Branch protection (main) | openshift/release PR | openshift/release | Prow config merge | Phase 1 |

### Workstream B: Progressive Delivery (GCP-985) -- blocked until all workspaces ported

| # | Phase | PR | Repo | Applied By | Depends On |
|---|-------|-----|------|------------|------------|
| 3 | Module branch support | infra-platform PR | infra-platform | Module maintainer | -- |
| 4 | Workspace branch config | gcp-hcp-infra PR | gcp-hcp-infra | TFC meta workspace | Phase 3 |
| 5 | Promoter TFC gating | gcp-hcp-infra PR | gcp-hcp-infra | ArgoCD sync | Phase 4 |
| 6 | Branch protection (env) | openshift/release PR | openshift/release | Prow config merge | Phase 4 |
| 7 | Documentation updates | gcp-hcp-infra + gcp-hcp PRs | both | N/A | Phases 4, 5 |

Phases 5 and 6 can proceed in parallel once Phase 4 is complete.

---

## Key Risks and Mitigations

### Workstream A (Speculative Plans)

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Speculative plans show unexpected drift | Confusing PR feedback | Medium | Expected with `refresh: false` -- document that PR plans are previews, not exact state comparisons |
| Prow job cannot reach TFC API | Plan job fails | Low | `build06` has outbound internet -- verify with test curl to `app.terraform.io` |
| `tfcloud-ci-secret` token lacks speculative plan permissions | Plan job auth failure | Low | Token already used for `terraform validate` which requires TFC API access -- verify `Plan runs` permission (see Pending Validation in design doc) |

### Workstream B (Progressive Delivery)

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Upstream `workspaces/tfe` module does not support `vcs_branch` | Blocks Phase 4 | Medium | Simple passthrough to `tfe_workspace.vcs_repo.branch` -- small PR to add |
| Hydrated environment branches missing terraform content | TFC init fails | Low | Hydration already copies all required dirs -- verify with `terraform init -backend=false` on an env branch (see Pending Validation in design doc) |
| Dual-triggered runs during branch cutover | State corruption | Medium | Follow the cutover procedure in Phase 4: disable auto-apply, drain queued runs, switch branch, verify, re-enable |
| TFC and Atlantis both triggering during migration | Double applies | Low | Atlantis is scoped to `main` -- environment branches are not in its trigger scope |
| State backend mismatch (GCS vs TFC cloud) | Init failures | Low | GCP-532 cutover migrates backends; if done first, no issue. If not, WIF SA has GCS access |
| Terraform changes promoted without terraform running | Silent infrastructure drift | Medium | Phase 5 (TFC apply gate) mitigates this; required before stage/production. Integration uses manual verification initially |

---

## Deferred

* **Stage and production workspaces**: Follow the same pattern when TFC access projects are provisioned for those environments
* **Automatic workspace discovery from metadata**: Use `terraform/metadata/environments.yaml` and `terraform/metadata/infra_ids.yaml` to dynamically generate workspace-to-branch mappings
* **Plan output as PR comment**: Start with TFC's native GitHub check integration; add custom PR comment formatting later if needed
