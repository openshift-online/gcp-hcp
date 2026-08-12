# TFC Progressive Delivery

## Overview

TFC workspaces track GitOps Promoter environment branches (e.g., `environment/global-integration`, `environment/sector-stage-main`) instead of `main`, so terraform changes flow through the same promotion pipeline as ArgoCD. **Blocked** until all workspaces are ported to TFC under the GCP-532 cutover plan.

| | |
|---|---|
| **Epic** | [GCP-532](https://redhat.atlassian.net/browse/GCP-532) -- Terraform Cloud Evaluation & Plan |
| **Story** | [GCP-985](https://redhat.atlassian.net/browse/GCP-985) -- Wire up GitOps Promoter with TFC for progressive delivery |
| **Related** | [GCP-1006](https://redhat.atlassian.net/browse/GCP-1006) -- Speculative plans on PRs (tracked separately; no dependency between the two) |

### Current Architecture

```text
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

```text
Developer -> PR to main -----> (speculative terraform plan, tracked separately under GCP-1006)
                |
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

The `vcs_branch` change must be applied atomically to avoid dual-triggering (runs from both `main` and the environment branch targeting the same state). Locking a workspace is the hard gate: a locked workspace rejects all new runs (VCS-triggered or otherwise), so locking first eliminates the window where a merge to `main` could queue a run during cutover.

1. **Lock all three workspaces** via TFC UI or API -- this rejects any new VCS-triggered runs immediately, no race window
2. **Disable auto-apply** on all three workspaces -- prevents any existing queued run from auto-applying if it was accepted before the lock
3. **Wait for in-flight runs to finish**: let any currently `planning` or `applying` runs reach a terminal state naturally rather than cancelling mid-apply
4. **Discard any remaining queued/pending runs** that were accepted before the lock took effect
5. **Apply the `vcs_branch` change** via the meta workspace (this switches TFC to track the environment branch)
6. **Verify**: confirm TFC shows the environment branch as the tracked branch and the expected commit; confirm no `main`-triggered run exists in a runnable state
7. **Unlock workspaces and re-enable auto-apply** on all three workspaces
8. **Trigger a test run**: push a trivial change through the promotion pipeline to verify end-to-end flow
9. **Unfreeze merges to `main`** (if a merge freeze was communicated to the team)

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
- [ ] Do **not** rely on TFC's native GitHub check runs as the apply-success signal -- those checks report speculative plan results and may remain passing even after an apply fails
- [ ] Instead, create a `WebRequestCommitStatus` in the promoter configuration that queries the TFC Runs API (`GET /api/v2/workspaces/:id/runs`) for the exact VCS commit SHA, requiring run status `applied` and apply status `finished`
- [ ] Alternatively, publish a dedicated `tfc-apply` commit status from post-apply automation (e.g., a TFC run task or notification webhook) and wire that into `activeCommitStatuses`
- [ ] For global promoter: update `kustomize/gitops-promoter/config-global.yaml` to add the TFC apply status key
- [ ] For sector promoter: update `helm/charts/gitops-promoter-config-region/values.yaml` to add the TFC apply status key and update `templates/promotion-strategy.yaml`

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
- [ ] Update `CLAUDE.md` if any workflow guidance changes

### gcp-hcp
- [ ] Add a design decision documenting the environment-branch tracking approach and update `design-decisions/INDEX.md`

**Acceptance Criteria**:
- [ ] `docs/promotions.md` accurately describes the TFC integration
- [ ] Design docs in gcp-hcp are up to date

---

## PR Sequence

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

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Upstream `workspaces/tfe` module does not support `vcs_branch` | Blocks Phase 4 | Medium | Simple passthrough to `tfe_workspace.vcs_repo.branch` -- small PR to add |
| Hydrated environment branches missing terraform content | TFC init fails | Low | Hydration already copies all required dirs -- verify with `terraform init -backend=false` on an env branch before Phase 4 |
| Dual-triggered runs during branch cutover | State corruption | Medium | Follow the cutover procedure in Phase 4: disable auto-apply, drain queued runs, switch branch, verify, re-enable |
| TFC and Atlantis both triggering during migration | Double applies | Low | Atlantis is scoped to `main` -- environment branches are not in its trigger scope |
| State backend mismatch (GCS vs TFC cloud) | Init failures | Low | GCP-532 cutover migrates backends; if done first, no issue. If not, WIF SA has GCS access |
| Terraform changes promoted without terraform running | Silent infrastructure drift | Medium | Phase 5 (TFC apply gate) mitigates this; required before stage/production. Integration uses manual verification initially |

---

## Deferred

* **Stage and production workspaces**: Follow the same pattern when TFC access projects are provisioned for those environments
* **Automatic workspace discovery from metadata**: Use `terraform/metadata/environments.yaml` and `terraform/metadata/infra_ids.yaml` to dynamically generate workspace-to-branch mappings
* **Terraform module git sources**: Migrating terraform module references from relative paths to `git::` sources pointing at `dry_sha` would eliminate file copying for modules, metadata, workflows, and dashboards
