# Atlantis to HCP Terraform Cutover

## Overview

Migrate infrastructure automation from self-hosted Atlantis to HCP Terraform Cloud (TFC) for the GCP HCP platform. TFC workspaces authenticate to GCP via the infra-platform [`gcp-dynamic-creds`](https://app.terraform.io/app/hp-platform-engineering/registry/modules/private/hp-platform-engineering/gcp-dynamic-creds/tfe) module, which creates a WIF pool, OIDC provider, and plan/apply service accounts in a dedicated access GCP project per environment. `apply_to_all_workspaces = true` delivers WIF credentials to all workspaces in a TFC project automatically.

Integration environment first, then stage. Production does not have Atlantis today, so TFC will be the first automation there.

**Status (2026-09-01)**: Stories 1–5 are complete for integration — `terraform/atlantis-integration.yaml` no longer lists `global`, `region`, or `management-cluster` projects, all three run on TFC with `auto_apply = true` (validated in [GCP-951](https://redhat.atlassian.net/browse/GCP-951), now Closed). Region/MC onboarding automation (Story 6 script work) shipped under [GCP-535](https://redhat.atlassian.net/browse/GCP-535). All post-epic Atlantis integration projects have also migrated: `hypershift-ci` and `platform-ci` to `gcp-hcp-ci` via [GCP-1093](https://redhat.atlassian.net/browse/GCP-1093) (2026-08-21); `pagerduty` to the new `gcp-hcp-tooling` TFC project via [GCP-1094](https://redhat.atlassian.net/browse/GCP-1094) (2026-08-24); and `service` via [GCP-1092](https://redhat.atlassian.net/browse/GCP-1092) (now Closed) — **not** into a dedicated `gcp-hcp-service` project as originally planned, but as a workspace inside the existing `gcp-hcp-integration` TFC project (reusing `gcp-hcp-int-tfc-access`; see [Atlantis Project Migrations](#atlantis-project-migrations-post-epic)). `terraform/atlantis-integration.yaml` now lists no projects — **no Atlantis integration projects remain**. Separately, `commons` and `commons-dev` — never on Atlantis, previously SRE-manual-applied — are now fully TFC-native in a dedicated `gcp-hcp-commons` TFC project with `auto_apply = true` via [GCP-1111](https://redhat.atlassian.net/browse/GCP-1111) (see [Commons Migration](#commons--commons-dev-migration-to-tfc-gcp-1111--complete)). Also remaining: Story 6 doc finalization ([GCP-952](https://redhat.atlassian.net/browse/GCP-952)), Story 7/8 stage and production rollout ([GCP-953](https://redhat.atlassian.net/browse/GCP-953)), Story 9 decommission ([GCP-954](https://redhat.atlassian.net/browse/GCP-954)), and migrating the org-level Terraform config to TFC ([GCP-1112](https://redhat.atlassian.net/browse/GCP-1112), New — gated on a security review; see [Remaining Scope](#remaining-scope-org-level-terraform-config-gcp-1112--not-started)).

**Epic**: [GCP-532](https://redhat.atlassian.net/browse/GCP-532) - Terraform Cloud Evaluation & Plan

**Spike**: [GCP-536](https://redhat.atlassian.net/browse/GCP-536) - Evaluate HCP Terraform for GCP-HCP Infrastructure

**Design decision**: [hcp-terraform-workload-identity-federation](../design-decisions/automation/hcp-terraform-workload-identity-federation.md)

**Experiment**: [hcp-terraform-wif-playground](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md) — validated module E2E, `apply_to_all_workspaces`, default audience behavior

**Bootstrap pattern**: [gcp-hcp-infra SETUP.md](https://github.com/openshift-online/gcp-hcp-infra/blob/main/terraform/config/global/SETUP.md) — same local-apply approach used for environment setup

**Architecture diagram**: [Miro board](https://miro.com/app/board/uXjVH9A0m0w=/?focusWidget=3458764677714315706)

### Authentication Flow

```text
TFC Workspace (e.g., gcp-hcp-global-integration)
    │
    ├─ 1. Inherits WIF variables from project-level variable set
    │      (module-managed, apply_to_all_workspaces=true)
    │
    ├─ 2. TFC generates OIDC token → GCP STS validates against
    │      WIF provider in gcp-hcp-{env_abbrev}-tfc-access
    │      attribute_condition scoped to TFC project
    │
    ├─ 3. STS returns federated token → exchanged for SA access token
    │      Plan phase: plan SA (unified roles, same as apply)
    │      Apply phase: apply SA (full write access on target projects)
    │
    └─ 4. SA token used for GCP API calls on target projects
```

---

## Spike Evaluation Summary

Findings from the [GCP-536](https://redhat.atlassian.net/browse/GCP-536) spike mapped to each evaluation area. The design decision and experiment docs cover authentication in depth; this section captures the remaining areas.

### 1. Workflow Fit

TFC uses a VCS-driven workflow: push to a branch triggers a speculative plan, merge to main triggers an apply. This replaces Atlantis's PR-comment-driven model (`atlantis plan`, `atlantis apply`). Key differences:

- **Plan triggers**: Automatic on PR push (no manual comment needed). Plans appear as GitHub check runs, not PR comments.
- **Apply triggers**: Configurable per workspace. Auto-apply on merge to main, or require manual confirmation in the TFC UI. Workspaces start with `auto_apply = false` and are switched to `auto_apply = true` after validation (Story 5 cutover step).
- **Parallel operations**: TFC serializes standard plan/apply runs per workspace, but speculative plans (triggered by PR pushes) run concurrently and do not block the run queue. Multiple PRs touching the same workspace can generate speculative plans simultaneously. This differs from Atlantis, which serializes all operations per project including plans.
- **Developer experience**: Plan output is in the TFC UI (linked from the GitHub check), not inline in the PR. This is a visibility tradeoff: richer UI with history and logs, but one click away from the PR.
- **Fork PRs**: TFC only runs speculative plans for PRs from branches on the upstream repo (`openshift-online/gcp-hcp-infra`), not from forks. Contributors working from forks will not see TFC plan output on their PRs. **Workaround**: push the branch directly to the upstream repo instead of opening a PR from a fork. This was discovered during [GCP-534](https://redhat.atlassian.net/browse/GCP-534) when the initial PR (#1069, from a fork) could not trigger TFC plans — it was closed and reopened as #1070 from an upstream branch.

### 2. Authorization (RBAC)

TFC supports team-based access control at the organization, project, and workspace level. Evaluation is deferred to production rollout (Story 8) where stricter approval gates matter. For integration and stage:

- All team members (`gcp-hcp-eng`) have admin access on TFC projects (configured in infra-platform bootstrap)
- Apply approval is manual in the TFC UI (any team member can confirm)
- Sentinel/OPA policies enforce deletion protection (already configured via bootstrap module)
- Production will require a more restrictive model (e.g., separate plan-only and apply-allowed teams)

### 3. State Management

TFC manages workspace state internally. New workspaces (like the access workspace) use the `cloud {}` backend from the start.

For existing infrastructure currently managed by Atlantis (global, region, MC), state must be **seeded into TFC via the State Versions API** before the first plan. TFC remote execution mode ignores `backend "gcs"` blocks entirely — if a workspace is created without state, TFC sees zero resources and attempts to create everything from scratch. The `terraform init -migrate-state` approach does not work with remote execution mode because `init` runs locally but plans run remotely (where the GCS backend is inaccessible).

**State seeding process** (per workspace, discovered during [GCP-534](https://redhat.atlassian.net/browse/GCP-534)):

1. Freeze Atlantis for the workspace (disable the autoplan entry in `atlantis-{env}.yaml` or drain in-flight operations) to prevent state changes during the migration window
2. Lock the TFC workspace (API or UI)
3. Download the current state from GCS (`gsutil cp gs://{bucket}/{workspace}.tfstate .`)
4. Upload the state to TFC via the [State Versions API](https://developer.hashicorp.com/terraform/cloud-docs/api-docs/state-versions): `POST /workspaces/{id}/state-versions` with `data.type = "state-versions"`, `data.attributes.serial` (matching the state file's serial), `data.attributes.md5`, and either the base64-encoded `data.attributes.state` or the hosted-upload-url flow
5. Poll until `status = "finalized"` and verify in the TFC UI that resource count matches expectations — do not proceed while still `pending`
6. Unlock the workspace
7. Securely delete the downloaded `.tfstate` file from the operator's workstation (it may contain sensitive resource attributes)

After seeding, TFC manages state natively and the GCS state bucket is no longer used for that workspace.

### API Activation and the `user_project_override` Pattern

TFC authenticates via Workload Identity Federation (WIF) through a dedicated access project (`gcp-hcp-{env_abbrev}-tfc-access`). WIF authentication causes GCP to check API activation against the WIF pool's project (the access project), which only has identity-related APIs enabled (IAM, STS, IAM Credentials). Without mitigation, any cross-project API call fails with `"<API> has not been used in project <access-project-id> before or it is disabled"`.

This was not a problem with Atlantis because Atlantis uses GKE Workload Identity (native SA auth), not WIF. With native SA auth, GCP checks API activation against the target project (the project in the resource URL), where the APIs are already enabled via `google_project_service` resources in each module.

**Do not** enable all required APIs on the access project -- that would create an operational burden since the access workspace requires PAM-gated manual applies, and every new API added to any module would require repeating that process.

**Solution**: Set `user_project_override = true` and `billing_project` pointing to the **global** project (e.g., `gcp-hcp-int-global`) in each workspace's `google` and `google-beta` provider blocks. The global project is used instead of the target project for two reasons:

1. **Bootstrap chicken-and-egg**: During fresh region/MC provisioning, the target project does not exist yet when the first `terraform plan` runs. Pointing `billing_project` at a non-existent project would fail immediately.
2. **IAM propagation delay**: Even after the target project is created, TFC SAs need `serviceUsageConsumer` on it to use it as a billing project. Granting this role and waiting for IAM propagation adds complexity and fragility to the bootstrap flow.

The global project avoids both issues: it always exists and TFC SAs already have `serviceUsageConsumer` on it. The trade-off is that the global project must have all APIs enabled that region/MC modules call for quota attribution. Ten APIs were added to the global module's `activated_apis` in [PR #1114](https://github.com/openshift-online/gcp-hcp-infra/pull/1114): `aiplatform`, `endpoints`, `eventarc`, `eventarcpublishing`, `gkeconnect`, `privilegedaccessmanager`, `pubsub`, `run`, `sqladmin`, and `workflows`. These API enablements do not create resources or incur cost.

Reference: [GCP Quota project overview](https://cloud.google.com/docs/quotas/quota-project) | [PR #1073](https://github.com/openshift-online/gcp-hcp-infra/pull/1073) | [PR #1114](https://github.com/openshift-online/gcp-hcp-infra/pull/1114) | [GCP-990](https://redhat.atlassian.net/browse/GCP-990)

### 4. Integration Points

- **GitHub App**: TFC uses the org-level GitHub App configured in infra-platform bootstrap. No per-environment GitHub App needed (unlike Atlantis which requires one per environment).
- **Pre-apply validation**: Atlantis uses `hack/check-pr-labels.sh` to validate PR labels before apply. TFC equivalent is run tasks or Sentinel policies. For the initial migration, this validation will be handled by the existing Prow CI checks which remain unchanged. A TFC-native replacement can be added later if needed.
- **Drift detection**: TFC supports scheduled health assessments that detect configuration drift. Enabled per workspace via `assessments_enabled`. Will be evaluated during Story 5 validation but is not a migration blocker.
- **OPA/Sentinel policies**: Deletion protection policy already enforced via the bootstrap module (`gcp-hcp-deletion-protection` policy set). Additional policies can be added incrementally.

### 5. Operational Considerations

- **Availability**: TFC is a managed SaaS with a [public status page](https://status.hashicorp.com) ([HCP SLA](https://portal.cloud.hashicorp.com/sla) explicitly excludes HCP Terraform). Removes the operational burden of running Atlantis on GKE (pod restarts, scaling, certificate renewal, Helm upgrades). If TFC is unavailable, no runs execute, but infrastructure is unaffected (same failure mode as Atlantis GKE downtime).
- **Auditability**: TFC provides organization-scoped audit trails (retained 14 days via the Audit Trails API), plus per-workspace run history and state versions (retained for the lifetime of the workspace). GCP Cloud Audit Logs record all API calls made by the plan/apply SAs. For long-term audit retention beyond 14 days, log forwarding to an external system (e.g., Splunk) would be needed. This is still an improvement over Atlantis, which relies on GKE pod logs subject to cluster-level retention limits.
- **Cost**: WIF and STS token exchanges are free. TFC workspace costs are governed by the organization plan (managed by infra-platform team). No additional GCP charges beyond the access project (which has no running resources).
- **Fallback**: During the migration (Stories 1-5), Atlantis remains fully operational. TFC runs speculative plans only until validation is complete. Atlantis is not decommissioned until Story 9, after TFC is proven in all environments. If TFC does not work out, Atlantis continues as-is with no rollback needed.

### 6. Migration Strategy

- **Per-workspace cutover**: Atlantis and TFC are never active on the same workspace simultaneously. Each workspace is cut over individually: Atlantis autoplan is disabled for that workspace, then TFC takes over. During the migration period, some workspaces may still be on Atlantis while others have moved to TFC, but there is no overlap per workspace.
- **Phased rollout**: Integration first (Stories 1-5), then stage (Story 7), then production (Story 8). Each environment is fully validated before the next begins.
- **Rollback**: At any point before Story 9 (decommission), a workspace can be reverted to Atlantis: disable TFC auto-apply, cancel any in-flight runs, discard runs waiting for confirmation, wait for the workspace lock to clear, migrate state back to GCS, then re-enable Atlantis autoplan for that workspace.
- **Order**: Lowest risk first. Integration has the fewest projects and the most tolerance for experimentation. Production is last and has no existing Atlantis to cut over from.

---

## Story 1: Bootstrap Access Project (Integration) — ✅ Complete

### Summary

Create the dedicated access GCP project for integration, containing the WIF pool, OIDC provider, plan/apply SAs, and TFC variable set. This is bootstrapped locally following the same pattern as environment setup.

**Repo**: `gcp-hcp-infra`
**Applied by**: Local `terraform apply` (operator with folder-admin permissions)

### Prerequisites

- Operator has folder-admin permissions on the integration environment folder
- Operator has a TFC API token (`TFE_TOKEN`) for variable set creation
- `gcp-hcp-integration` TFC project exists

### Tasks

- [x] Create `terraform/config/tfc-access/integration/main.tf`:
  - `google_project` resource for `gcp-hcp-int-tfc-access` in the integration folder
  - `gcp-dynamic-creds` module call sourced from `app.terraform.io/hp-platform-engineering/gcp-dynamic-creds/tfe`
  - Module inputs: `project_id`, `tfc_organization`, `tfc_project_name = "gcp-hcp-integration"`, `apply_to_all_workspaces = true`
  - `plan_roles` — `roles/viewer` (read access on the access project only)
  - `apply_roles` — self-management roles for the access project only: `roles/viewer`, `roles/iam.workloadIdentityPoolAdmin`, `roles/iam.serviceAccountAdmin`, `roles/resourcemanager.projectIamAdmin`, `roles/serviceusage.serviceUsageAdmin`
  - Note: these roles apply to the access project, not target projects. The full Atlantis role set is granted on target projects in Story 2 (cross-project IAM).
  - `cloud {}` backend block (commented out for initial local apply)
- [x] Create `terraform/config/tfc-access/integration/versions.tf` with provider version constraints
- [x] Run `terraform init && terraform apply` locally
- [x] Verify in TFC UI: variable set exists, `apply_to_all_workspaces` is enabled, 4 `TFC_GCP_*` variables present
- [x] Register access workspace in meta workspace (`hcp-terraform/meta/main.tf` or equivalent) — workspace must exist before state migration
- [x] Uncomment `cloud {}` backend block
- [x] Run `terraform init -migrate-state` to migrate state to TFC
  > Note: `terraform init -migrate-state` works here because the access workspace starts with **local** state (from the initial apply). Existing workspaces with GCS state (Story 4) require the State Versions API instead — see [State Management](#3-state-management).

### Acceptance Criteria

- [x] GCP project `gcp-hcp-int-tfc-access` exists in the integration folder
- [x] WIF pool and OIDC provider exist in the access project
- [x] Plan and apply SAs exist in the access project
- [x] TFC variable set `*-gcp-dynamic-creds` attached to `gcp-hcp-integration` project with `apply_to_all_workspaces = true`
- [x] Variable set contains the expected `TFC_GCP_*` variables (`TFC_GCP_PROVIDER_AUTH`, `TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL`, `TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL`, `TFC_GCP_WORKLOAD_PROVIDER_NAME`)
- [x] State managed by TFC (not local)

---

## Story 2: Cross-Project IAM for Plan and Apply SAs (Integration) — ✅ Complete

### Summary

Grant both the plan and apply SAs the full write role set on each target project (global, region, MC). Using unified roles means permission gaps surface during `terraform plan` rather than only at apply time. This mirrors the existing `atlantis.tf` IAM pattern but with two SAs instead of one.

**Repo**: `gcp-hcp-infra`
**Applied by**: Atlantis PR
**Depends on**: Story 1 (SAs must exist)

### Tasks

- [x] Create `terraform/modules/global/tfc.tf`:
  - Variables: `enable_tfc` (default `false`), `tfc_plan_sa_email`, `tfc_apply_sa_email`
  - Apply SA — 19 project-level roles (same as Atlantis):
    `container.admin`, `gkehub.admin`, `compute.networkAdmin`, `storage.admin`, `compute.instanceAdmin.v1`, `iam.serviceAccountAdmin`, `resourcemanager.projectIamAdmin`, `serviceusage.serviceUsageAdmin`, `compute.securityAdmin`, `dns.admin`, `secretmanager.admin`, `certificatemanager.editor`, `iap.admin`, `artifactregistry.admin`, `cloudbuild.builds.editor`, `logging.admin`, `iam.workloadIdentityPoolAdmin`, `monitoring.metricsScopesAdmin`, `monitoring.editor`
  - Apply SA — 3 folder-level roles (on `folders/{parent_folder_id}`):
    `resourcemanager.projectCreator`, `resourcemanager.folderAdmin`, `logging.configWriter`
  - Plan SA — same 19 project-level roles as apply SA (unified roles catch permission issues at plan time)
  - All resources gated by `var.enable_tfc`
- [x] Create `terraform/modules/region/tfc.tf`:
  - Both SAs — 24 cross-project roles (same as `atlantis.tf`):
    `resourcemanager.projectIamAdmin`, `viewer`, `container.admin`, `compute.networkAdmin`, `storage.admin`, `compute.instanceAdmin.v1`, `iam.serviceAccountAdmin`, `iam.serviceAccountUser`, `serviceusage.serviceUsageAdmin`, `compute.securityAdmin`, `dns.admin`, `gkehub.admin`, `secretmanager.admin`, `workflows.admin`, `run.admin`, `pubsub.admin`, `eventarc.admin`, `resourcemanager.tagAdmin`, `resourcemanager.tagUser`, `privilegedaccessmanager.admin`, `storage.hmacKeyAdmin`, `bigquery.admin`, `artifactregistry.admin`, `monitoring.metricsScopesAdmin`
  - Same bootstrap pattern as `atlantis.tf` (project-creator impersonation for initial `projectIamAdmin`)
- [x] Create `terraform/modules/management-cluster/tfc.tf`:
  - Both SAs — 20 cross-project roles (region minus: `gkehub.admin`, `storage.hmacKeyAdmin`, `bigquery.admin`, `artifactregistry.admin`)
- [x] Enable in integration configs:
  - `terraform/config/global/integration/main/us-central1/main.tf`: `enable_tfc = true`, `tfc_plan_sa_email = "..."`, `tfc_apply_sa_email = "..."`
  - `terraform/config/region/integration/main/us-central1/main.tf`: same
  - `terraform/config/management-cluster/integration/main/us-central1-yjiv/main.tf`: same
- [x] Open PR, apply via Atlantis

### Acceptance Criteria

- [x] `terraform plan` shows IAM bindings for both plan and apply SAs on all target projects
- [x] After apply, both plan and apply SAs can read and modify resources in global/region/MC projects

### Notes

Split IAM creation from workspace creation — IAM propagation delay (~60s) means workspaces should not be created until IAM has propagated.

---

## Story 3: Commons Cross-Project Grants (Integration) — ✅ Complete

> **Partly superseded by [Commons + commons-dev Migration to TFC (GCP-1111)](#commons--commons-dev-migration-to-tfc-gcp-1111--complete).** This story granted the *integration* TFC plan/apply SAs (`gcp-hcp-int-tfc-access`) access to commons resources so integration workspaces could read commons state — that is still in place (the `terraform/modules/commons/tfc-iam.tf` grants below). What has changed is how commons *itself* is managed: it is no longer an SRE-manual-applied module. As of GCP-1111, `commons` and `commons-dev` are fully TFC-native (dedicated `gcp-hcp-commons` TFC project, `auto_apply = true`), so these grants now apply via the `gcp-hcp-commons` TFC workspace rather than a manual SRE `terraform apply`.

### Summary

Grant the apply SA access to commons resources: Terraform state bucket, GAR repo, and project-creator SA impersonation.

**Repo**: `gcp-hcp-infra`
**Applied by**: Originally SRE manual apply (commons was not managed by Atlantis or TFC at the time); now applied by the `gcp-hcp-commons` TFC workspace, since commons is TFC-native as of [GCP-1111](https://redhat.atlassian.net/browse/GCP-1111)
**Depends on**: Story 1 (SAs must exist)

### Tasks

- [x] Create `terraform/modules/commons/tfc-iam.tf` (parallel to `atlantis-iam.tf`):
  - `roles/storage.objectViewer` on commons TF state bucket for plan SA — needed for `terraform_remote_state` reads during speculative plans
  - `roles/storage.objectViewer` on commons TF state bucket for apply SA — same reads during apply phase
  - `roles/artifactregistry.admin` on commons GAR repo for apply SA
  - `roles/iam.serviceAccountTokenCreator` on `project-creator` SA for apply SA
  - Iterate over `var.environment_dns_zones` (excluding dev)
  - Members: `serviceAccount:{plan_sa}@gcp-hcp-{env_abbrev}-tfc-access.iam.gserviceaccount.com` and `serviceAccount:{apply_sa}@gcp-hcp-{env_abbrev}-tfc-access.iam.gserviceaccount.com`
  - Note: `objectViewer` is correct — matches Atlantis. Workspace state lives in TFC (cloud backend), not GCS. The commons bucket is only accessed via `terraform_remote_state` data sources for commons outputs.
- [x] Coordinate with SRE for manual apply

### Acceptance Criteria

- [x] Plan SA can read commons Terraform state (for `terraform_remote_state` in speculative plans)
- [x] Apply SA can read commons Terraform state
- [x] Apply SA can push to commons GAR
- [x] Apply SA can impersonate `project-creator` SA

---

## Story 4: TFC Workspace Definitions (Integration) — ✅ Complete

### Summary

Create TFC workspaces mirroring the Atlantis projects in `atlantis-integration.yaml`. Workspaces inherit WIF credentials from the module-created variable set — no per-workspace WIF configuration needed.

**Repo**: `gcp-hcp-infra`
**Applied by**: TFC meta workspace (push to main triggers it)
**Depends on**: Story 1 (variable set), Story 2 (IAM), Story 3 (commons grants — state bucket access required for `terraform init`)

### Tasks

- [x] Create `hcp-terraform/gcp-hcp-integration/main.tf` with workspace definitions via `workspaces/tfe` module:

  | TFC Workspace | Working Directory | Trigger Prefixes |
  |---|---|---|
  | `gcp-hcp-global-integration` | `terraform/config/global/integration/main/us-central1` | `terraform/metadata/`, `terraform/dashboards/global/`, `terraform/modules/global/` |
  | `gcp-hcp-region-int-main-us-central1` | `terraform/config/region/integration/main/us-central1` | `terraform/workflows/`, `terraform/metadata/`, `terraform/modules/region/` |
  | `gcp-hcp-mc-int-main-us-central1-yjiv` | `terraform/config/management-cluster/integration/main/us-central1-yjiv` | `terraform/workflows/`, `terraform/metadata/`, `terraform/modules/management-cluster/` |

  > **Note**: Trigger prefixes must include shared module paths (`terraform/modules/{type}/`) so that changes to the module source trigger plans in dependent workspaces. Additional regions/MCs (e.g. us-west1, us-south1) have since been onboarded via [GCP-535](https://redhat.atlassian.net/browse/GCP-535) automation.

- [x] Create workspaces with `auto_apply = false` — auto-apply is enabled after validation (Story 5)
- [x] Create `hcp-terraform/gcp-hcp-integration/cloud.tf` pointing at meta workspace
- [x] No `tfe_variable_set` or `tfe_variable` resources needed — module handles variable sets via `apply_to_all_workspaces`
- [x] Open PR, merge to main (meta workspace applies)
- [x] **Provider configuration** for each migrated workspace config -- update `google` and `google-beta` provider blocks **before** state seeding and the first speculative plan:
  ```hcl
  locals {
    global_project_id = "gcp-hcp-${local.env_config.abbreviation}-global"
  }

  provider "google" {
    billing_project       = local.global_project_id
    user_project_override = true
    default_labels        = local.common_labels
  }
  ```
  This redirects GCP API activation checks to the global project, avoiding bootstrap issues with non-existent target projects (see [API Activation](#api-activation-and-the-user_project_override-pattern) above). The `global_project_id` local is derived from metadata because provider blocks cannot reference data sources. Both `google` and `google-beta` provider blocks must be updated, including any aliased providers. See [PR #1114](https://github.com/openshift-online/gcp-hcp-infra/pull/1114) for reference.
- [x] **State seeding** (per workspace, for workspaces with existing Atlantis-managed infrastructure): follow the [State Management](#3-state-management) procedure (freeze Atlantis → lock workspace → download GCS state → upload via State Versions API preserving serial/MD5 → verify resource count → unlock → securely delete the local `.tfstate`), then run a speculative plan and confirm it shows no changes (or only expected drift).
  > **Important**: Atlantis must remain frozen from the initial freeze through the end of plan verification. An Atlantis apply between state download and TFC upload would leave TFC with a stale snapshot.

### Acceptance Criteria

- [x] Workspaces appear in `gcp-hcp-integration` TFC project
- [x] Each workspace has WIF variables inherited from the project-level variable set
- [x] Provider blocks include `user_project_override = true` and `billing_project` (merged before state seeding)
- [x] Each workspace with existing infrastructure has state seeded from GCS (resource count matches)
- [x] Speculative plans run on PR pushes (from upstream branches — not forks, see [Workflow Fit](#1-workflow-fit))

---

## Story 5: Validation and Cutover (Integration) — ✅ Complete ([GCP-951](https://redhat.atlassian.net/browse/GCP-951), Closed 2026-08-20)

### Summary

Validate that TFC can manage the same infrastructure as Atlantis with the same outcomes, then complete the cutover for each workspace.

**Depends on**: Stories 2, 3, 4

### Tasks

- [x] **Plan comparison**: Run TFC plan on each workspace, compare with latest Atlantis plan — outputs should be identical (no-op or same diff)
- [x] **Small change test**: Apply a minor change (e.g., add a resource label) via TFC on one workspace
- [x] **State consistency**: Verify TFC-seeded state matches the GCS state (resource count, serial number)
- [x] **Cross-project operations**: Verify region workspace can create IAM bindings on the global project (via `modules/region/global-iam.tf`) — this is the key operation that validates the cross-project IAM grants
- [x] **Plan SA isolation**: Verify speculative plans succeed using the plan SA's unified (write-capable) roles from Story 2 — `terraform plan` does not call write APIs even though the SA has permission to, so no destructive actions occur during a plan run
- [x] **API activation**: Verify `user_project_override = true` resolves quota project errors — plans should not fail with "API has not been used in project" errors
- [x] **Cutover per workspace** (after validation passes):
  1. Disable Atlantis autoplan for the workspace (remove entry from `atlantis-integration.yaml`)
  2. Set `auto_apply = true` in the workspace definition (`hcp-terraform/gcp-hcp-integration/main.tf`) so TFC auto-applies on merge to main, matching Atlantis behavior
  3. Merge the `auto_apply` change — TFC applies it via the meta workspace
  4. Verify the next merge-to-main triggers an automatic apply (no manual confirmation needed in TFC UI)
- [x] **Update Prow required status checks**: Remove Atlantis status checks (`atlantis-int/plan`, `atlantis-int/apply`) from the required checks on the `main` branch in [`openshift/release`](https://github.com/openshift/release/blob/main/core-services/prow/02_config/openshift-online/gcp-hcp-infra/_prowconfig.yaml). Without this, PRs to `gcp-hcp-infra` will be blocked because the removed Atlantis integration projects no longer report these statuses. Open a PR against `openshift/release` to update the Prow config. See [GCP-951](https://redhat.atlassian.net/browse/GCP-951).

### Acceptance Criteria

- [x] TFC plan output matches Atlantis for all integration workspaces
- [x] At least one apply completes successfully via TFC
- [x] Cross-project IAM operations work from region and MC workspaces
- [x] No "API has not been used in project" errors during plans or applies
- [x] Each validated workspace has `auto_apply = true` and its Atlantis autoplan entry removed
- [x] Prow required status checks updated to remove Atlantis checks for the cutover environment

---

## Story 6: Scripts and Tooling — 🟡 Scripts done, docs pending ([GCP-952](https://redhat.atlassian.net/browse/GCP-952))

### Summary

Extend automation tooling for TFC workspace generation and update design docs.

**Repo**: `gcp-hcp-infra` (scripts), `gcp-hcp` (docs)

### Tasks

- [x] Extend `scripts/infra.py` to generate TFC workspace entries (delivered under [GCP-535](https://redhat.atlassian.net/browse/GCP-535), [PR #1270](https://github.com/openshift-online/gcp-hcp-infra/pull/1270)):
  - Add workspace entries to `hcp-terraform/gcp-hcp-{env}/main.tf` when creating new regions/MCs
  - Add plan/apply SA cross-project IAM to `tfc.tf` files in region/MC modules
  - Same gate: `environment in ['integration', 'stage', 'production'] and sector != 'e2e'`
- [ ] Update design docs in `gcp-hcp`:
  - `design-decisions/automation/hcp-terraform-workload-identity-federation.md` — mark as implemented
  - Close experiment cleanup checklist items

### Acceptance Criteria

- [x] `scripts/infra.py new region {env} {sector} {region}` generates TFC workspace entry alongside Atlantis project entry (GCP-535)
- [x] Generated IAM includes both plan and apply SA bindings (GCP-535)

---

## Story 7: Repeat for Stage — 🔲 Not started ([GCP-953](https://redhat.atlassian.net/browse/GCP-953))

Same as Stories 1–5 for the stage environment:

- Target access project: `gcp-hcp-stg-tfc-access`
- Target projects: `gcp-hcp-stg-global`, `stg-reg-*`, `stg-mgt-*`
- New workspace definitions: `hcp-terraform/gcp-hcp-stg/`
- TFC project: `gcp-hcp-stage`

---

## Story 8: Production Rollout — 🔲 Not started ([GCP-953](https://redhat.atlassian.net/browse/GCP-953))

### Summary

Deploy TFC to production. Production does not have Atlantis today, so TFC will be the first automation. Same Stories 1–5 pattern, but no Atlantis cutover or parallel-running concern.

**Depends on**: Stories 5 and 7 (integration and stage validated)

- Target access project: `gcp-hcp-prd-tfc-access`
- Target projects: `gcp-hcp-prd-global`, `prd-reg-*`, `prd-mgt-*`
- New workspace definitions: `hcp-terraform/gcp-hcp-prd/`
- TFC project: `gcp-hcp-production`

### Notes

Production introduces additional considerations not present in integration/stage:
- Change approval gates may be stricter (RBAC, policy enforcement)
- First apply is also the first automated apply in production — manual validation of the initial plan is critical
- No Atlantis to compare against — validation relies on manual spot-checks and expected no-op plans

---

## Story 9: Cutover and Decommission Atlantis — 🔲 Not started ([GCP-954](https://redhat.atlassian.net/browse/GCP-954))

### Summary

Disable Atlantis and remove its infrastructure after TFC is validated in integration, stage, and production.

### Tasks

- [ ] Disable Atlantis autoplan (ArgoCD sync disabled or webhook removed) for workspaces that TFC manages
- [ ] Validate TFC handles all PRs for ~1 week with no Atlantis fallback
- [ ] Remove Atlantis ArgoCD application (`argocd/config/global/atlantis/`)
- [ ] Remove Atlantis Helm chart (`helm/charts/atlantis-stack/`)
- [ ] Remove Atlantis SA and IAM bindings (`atlantis.tf` files in global/region/MC/commons modules)
- [ ] Remove `atlantis-{env}.yaml` files
- [ ] Remove all Atlantis required status checks from Prow config in [`openshift/release`](https://github.com/openshift/release/blob/main/core-services/prow/02_config/openshift-online/gcp-hcp-infra/_prowconfig.yaml) (per-environment checks should already be removed during Story 5/7/8 cutover; verify none remain)
- [ ] Update mintmaker agent (`agent/mintmaker/tools/atlantis.py` → TFC equivalent)
- [ ] Remove `enable_tfc` variable gates — TFC IAM becomes the only IAM

### Acceptance Criteria

- [ ] No Atlantis pods running in any environment
- [ ] All `atlantis.tf` and `atlantis-iam.tf` files removed
- [ ] No Atlantis-related Prow required status checks remain in `openshift/release`
- [ ] All infrastructure changes flow through TFC

---

## PR Sequence (Integration)

| # | Story | PR | Applied By | Depends On | Status |
|---|---|---|---|---|---|
| 1 | Access project bootstrap | Local apply (no PR) | Operator | — | ✅ Done |
| 2 | Cross-project IAM | `gcp-hcp-infra` PR | Atlantis | Story 1 | ✅ Done |
| 3 | Commons grants | `gcp-hcp-infra` PR | SRE manual | Story 1 | ✅ Done |
| 4 | Workspace definitions | `gcp-hcp-infra` PR | TFC meta workspace | Stories 1, 2, 3 | ✅ Done |
| 5 | Validation | Manual testing | — | Stories 2, 3, 4 | ✅ Done (GCP-951) |
| 6 | Scripts & tooling | `gcp-hcp-infra` + `gcp-hcp` PRs | N/A (tooling) | Story 4 | 🟡 Scripts done, docs pending (GCP-952) |
| 7-8 | Stage & production rollout | — | — | Story 5 | 🔲 Not started (GCP-953) |
| 9 | Atlantis decommission | — | — | Stories 5, 7, 8 | 🔲 Not started (GCP-954) |

Stories 2 and 3 can run in parallel (both depend only on Story 1).

## Key Risks and Mitigations

| Risk | Mitigation |
|---|---|
| IAM propagation delay on new SA roles | Split SA creation (Story 1) from workspace creation (Story 4); allow ~60s between apply and first workspace run |
| State file locking during parallel Atlantis + TFC | Freeze Atlantis for the workspace before downloading GCS state for seeding; keep frozen through upload, verification, and cutover. Only one system should apply at a time |
| ~~Commons module requires SRE manual apply~~ (resolved) | No longer applicable — `commons` and `commons-dev` are TFC-native with `auto_apply = true` as of [GCP-1111](https://redhat.atlassian.net/browse/GCP-1111). Cross-project grants that were previously SRE-applied now apply via the `gcp-hcp-commons` TFC workspace. See [Commons Migration](#commons--commons-dev-migration-to-tfc-gcp-1111--complete) |
| Atlantis and TFC both triggering on same PR | Disable Atlantis autoplan for workspaces that TFC manages before enabling TFC |
| Module upstream breakage | Pin to specific module version in TFC private registry; test upgrades in integration first |
| Plan SA needs more than viewer for certain plans | If `terraform plan` fails with viewer-only, add specific read roles to plan SA — or use `use_apply_role_for_plan` ([infra-platform#119](https://github.com/openshift-online/infra-platform/pull/119)) to fall back to unified roles |
| GCP API activation fails on access project (quota project mismatch) | Set `user_project_override = true` and `billing_project` in provider blocks to redirect activation checks to the global project. Global is used instead of the target project to avoid bootstrap chicken-and-egg issues. See [API Activation](#api-activation-and-the-user_project_override-pattern) |
| TFC workspace created without state sees zero resources and tries to create everything | Seed state from GCS via the State Versions API **before** the first plan. See [State Management](#3-state-management) |
| Fork PRs do not trigger TFC speculative plans | Contributors must push branches to the upstream repo, not open PRs from forks. Document in team onboarding |
| Incomplete trigger prefixes miss shared module changes | Include shared module paths (`terraform/modules/{type}/`) in workspace trigger prefixes alongside config-specific paths |
| Atlantis required status checks block PRs after cutover | Prow config in `openshift/release` defines `atlantis-{env}/plan` and `atlantis-{env}/apply` as required checks on `main`. After removing Atlantis projects, these checks never report, blocking all merges. Update Prow config per environment as part of the cutover (Story 5). PR against [`openshift/release`](https://github.com/openshift/release/blob/main/core-services/prow/02_config/openshift-online/gcp-hcp-infra/_prowconfig.yaml) |

## Atlantis Project Migrations (Post-Epic)

All Atlantis projects beyond the original epic scope (`global`/`region`/`management-cluster`) have now migrated to TFC. `terraform/atlantis-integration.yaml` no longer lists any of them — **no Atlantis integration projects remain**. None of these were in the epic's original scope: `hypershift-ci` was listed but explicitly deferred ("planned separately after environment workspaces are validated," a milestone completed via GCP-951); `platform-ci`, `service`, and `pagerduty` were added to Atlantis after the epic was written and were never scoped at all.

`service` is the notable correction here. It was **not** migrated into a new, dedicated `gcp-hcp-service` TFC project as this section previously described. The dedicated-project approach shipped first ([PR #1450](https://github.com/openshift-online/gcp-hcp-infra/pull/1450), with follow-ups [#1459](https://github.com/openshift-online/gcp-hcp-infra/pull/1459)/[#1461](https://github.com/openshift-online/gcp-hcp-infra/pull/1461)) but was then reworked ([PR #1474](https://github.com/openshift-online/gcp-hcp-infra/pull/1474)) into a **workspace inside the existing `gcp-hcp-integration` TFC project** (`gcp-hcp-service-integration`), reusing the `gcp-hcp-int-tfc-access` identity — the same pattern as `global`/`region`/`management-cluster`. As part of the rework, `hcp-terraform/gcp-hcp-service/` and `terraform/config/tfc-access/service/` were deleted, the `service` entry was removed from `tfc_service_accounts` in `terraform/config/commons/main.tf` (integration already had project-creator impersonation), and the dedicated `gcp-hcp-service-tfc-access` and `gcp-hcp-service` projects were decommissioned ([infra-platform#178](https://github.com/openshift-online/infra-platform/pull/178)). `terraform/config/service/integration/main.tf` now points `tfc_plan_sa_email`/`tfc_apply_sa_email` at `hcp-tf-default-{plan,apply}@gcp-hcp-int-tfc-access`. [GCP-1092](https://redhat.atlassian.net/browse/GCP-1092) is **Closed**.

`service` was also the last live `backend = "gcs"` consumer of `global`'s state flagged by [Migration Finding #9](#migration-findings); its migration closed that out. Verified on `main`: `terraform/config/service/integration/main.tf`'s `data.terraform_remote_state.global` now reads `gcp-hcp-global-integration` via `backend = "remote"` (no `backend "gcs"` remains), and `gcp-hcp-service-integration` is registered in the `gcp-hcp-global-integration` workspace's `remote_state_consumer_workspaces` (`hcp-terraform/gcp-hcp-integration/main.tf`). No `backend = "gcs"` consumers of `global` remain.

`gcp-hcp-tooling` now exists (created for `pagerduty` under [GCP-1094](https://redhat.atlassian.net/browse/GCP-1094)). The `gcp-hcp-service` TFC project and its `gcp-hcp-service-tfc-access` project no longer exist (created, then decommissioned per the rework above).

### Migrated

| Project | TFC Project / Workspace | Migrated | PRs |
|---|---|---|---|
| `hypershift-ci` | `gcp-hcp-ci` | 2026-08-21 ([GCP-1093](https://redhat.atlassian.net/browse/GCP-1093)) | [#1391](https://github.com/openshift-online/gcp-hcp-infra/pull/1391), [#1411](https://github.com/openshift-online/gcp-hcp-infra/pull/1411) |
| `platform-ci` | `gcp-hcp-ci` | 2026-08-21 ([GCP-1093](https://redhat.atlassian.net/browse/GCP-1093)) | [#1391](https://github.com/openshift-online/gcp-hcp-infra/pull/1391), [#1411](https://github.com/openshift-online/gcp-hcp-infra/pull/1411) |
| `pagerduty` | `gcp-hcp-tooling` | 2026-08-24 ([GCP-1094](https://redhat.atlassian.net/browse/GCP-1094)) | [#1434](https://github.com/openshift-online/gcp-hcp-infra/pull/1434), [#1442](https://github.com/openshift-online/gcp-hcp-infra/pull/1442) |
| `service` | `gcp-hcp-integration` (workspace `gcp-hcp-service-integration`) | [GCP-1092](https://redhat.atlassian.net/browse/GCP-1092) (Closed) | [#1450](https://github.com/openshift-online/gcp-hcp-infra/pull/1450), [#1459](https://github.com/openshift-online/gcp-hcp-infra/pull/1459), [#1461](https://github.com/openshift-online/gcp-hcp-infra/pull/1461), [#1474](https://github.com/openshift-online/gcp-hcp-infra/pull/1474) |

`gcp-hcp-ci` already existed as a TFC project (access project `gcp-hcp-ci-tfc-access` bootstrapped). It now hosts `hypershift-ci` and `platform-ci` as persistent workspaces alongside Jim's ephemeral e2e workspaces. Both use the `gcp-dynamic-creds` access-project pattern (plan/apply SAs shared across the project via `apply_to_all_workspaces`), read `global` via `backend = "remote"` (added as remote-state consumers on `gcp-hcp-global-integration`, resolving the Finding #9 stale-snapshot risk), and were removed from Atlantis in the same change that switched the backend. State was seeded from GCS, validated as a no-op refresh plan, and `auto_apply` enabled. See Migration Finding #11 for the brownfield first-run detail.

> **Follow-up ([PR #1495](https://github.com/openshift-online/gcp-hcp-infra/pull/1495))**: two pre-existing `platform-ci` bugs in `commons` (introduced by [PR #1462](https://github.com/openshift-online/gcp-hcp-infra/pull/1462)'s onboarding) surfaced while reconciling commons state after this migration and were fixed: (1) the `platform-ci.gcp-hcp.devshift.net` zone delegation had guessed, wrong name servers (`ns-cloud-d1..d4`) — corrected to the actual `ns-cloud-e1..e4` from the workspace outputs; (2) `platform-ci` was wrongly included in six commons `for_each` loops granting `atlantis@{env}`/`e2e-deployer@{env}` roles, causing perpetual plan drift for non-existent SAs (platform-ci only ever referenced `atlantis@global`, never a local `atlantis@platform-ci`) — fixed with `&& k != "platform-ci"` exclusions mirroring the existing `dev` exclusion. Collateral to GCP-1093; no bearing on the CI migration itself, but required before commons state could be considered reconciled.

`gcp-hcp-tooling` is a **new** TFC project created for `pagerduty` as the home for non-environment-scoped tooling configs. Unlike the CI workspaces, `pagerduty` manages no `google` resources and reads no remote state, so there is no `backend "gcs"` to `backend "remote"` switch and no consumer grant. It still required a **full** access-project bootstrap: a dedicated `gcp-hcp-tooling-tfc-access` project with its own `gcp-dynamic-creds` WIF pool, provider, and plan/apply SAs. The existing CI/integration identities could not be reused because each access project's WIF provider is attribute-scoped to a single TFC project and its variable set attaches only to that project's workspaces. `terraform/config/pagerduty/providers.tf` reads the PagerDuty API token from the `pagerduty-apikey-gcp-hcp-eng` secret in `gcp-hcp-int-global` inside the provider block, so the read runs at plan time; both the plan and apply SAs were granted cross-project access to that one secret with `roles/secretmanager.secretAccessor` **and** `roles/secretmanager.viewer` (see Finding #12). State was seeded from GCS and validated as a no-op `refresh=true` plan before `auto_apply` was enabled, and Atlantis stayed the sole owner until the cutover PR ([#1442](https://github.com/openshift-online/gcp-hcp-infra/pull/1442)) removed it.

---

## Commons + commons-dev Migration to TFC (GCP-1111) — ✅ Complete

### Summary

`commons` and `commons-dev` were the last "never-automated" Terraform configs in the epic — never on Atlantis, always applied by hand by an SRE. [GCP-1111](https://redhat.atlassian.net/browse/GCP-1111) migrated both onto TFC with a dedicated `gcp-hcp-commons` TFC project and its own WIF-based access-project bootstrap, ending the manual-apply workflow. Both workspaces now run with `auto_apply = true` like every other TFC-managed config.

This supersedes the "commons is SRE-manual-applied" framing in [Story 3](#story-3-commons-cross-project-grants-integration--complete) and the (now-resolved) "Commons module requires SRE manual apply" risk.

**Repo**: `gcp-hcp-infra`
**TFC project**: `gcp-hcp-commons` (new)
**Access project**: `gcp-hcp-commons-tfc-access` (new, `terraform/config/tfc-access/commons/`)
**Scope**: `commons` (main: DNS delegation, GAR, mintmaker agent, Konflux WIF) and `commons-dev` (developer-shared DNS/state bucket). Migrating `stage` (`global-stage`, `region-stage`) off GCS was explicitly out of scope.

### Access-project pattern

Same `gcp-dynamic-creds` WIF pattern as `ci`/`tooling`/`integration`: `terraform/config/tfc-access/commons/` creates a dedicated `gcp-hcp-commons-tfc-access` GCP project and calls the `gcp-dynamic-creds` module (WIF pool, OIDC provider, plan/apply SAs, variable set with `apply_to_all_workspaces = true`). The old `terraform/modules/commons/tfc.tf` (a direct-WIF `tfc-pool`/`tfc-oidc`/`tfc-automation` setup) was **deleted** and replaced by this access-project pattern; its outputs (`tfc_service_account_email`, `tfc_wif_provider_name`) and the `tfc_organization_name` variable were removed. `terraform/modules/commons/tfc-iam.tf` (the [Story 3](#story-3-commons-cross-project-grants-integration--complete) grants for the existing int/ci consumer SAs) was kept — it is unrelated and unaffected.

> Note: the earlier "commons provisions a WIF trust root that every other TFC workspace authenticates through, so migrating it creates bootstrap circularity" concern (raised during GCP-1094) was **incorrect** — no such resources exist. Commons migrated normally with the standard access-project pattern.

### Derived IAM role set (`tfc-access/commons/commons-iam.tf`)

Granted to both `hcp-tf-default-plan` and `hcp-tf-default-apply` (unified plan=apply convention):

- **Project-level** on `gcp-hcp-commons` + `gcp-hcp-commons-dev`: `roles/viewer`, `serviceusage.serviceUsageAdmin`, `resourcemanager.projectIamAdmin`, `iam.serviceAccountAdmin`, `iam.workloadIdentityPoolAdmin`, `artifactregistry.admin`, `storage.admin`, `dns.admin`, `secretmanager.admin`, `aiplatform.admin`, `cloudscheduler.admin`, `compute.networkAdmin`, `networksecurity.admin`, `networkservices.admin`
- **Folder-level**: `resourcemanager.folderIamAdmin` on the top-level GCP HCP folder (`405445313657`) — needed for cloud-custodian's folder bindings
- **Cross-project, resource-scoped**: `artifactregistry.admin` on `gcp-hcp-int-global`'s `diagnose-agent` repo; `storage.objectViewer` on `gcp-hcp-stg-global-terraform-state` (commons reads `global_stage` remote state via a direct cross-project GCS read — stage isn't on TFC); `iam.serviceAccountUser` on the `mintmaker-agent` SA specifically (needed to `actAs` it when deploying the Vertex AI Reasoning Engine that runs as it)

The role set was derived **iteratively** from real first-runs — the stage-state GCS grant (#1542) and the mintmaker `actAs` grant (#1549) only surfaced when the WIF identity first touched real infrastructure, after the "complete" list was already written. This matches the service-integration (GCP-1092) experience: expect 2–3 rounds of narrowly-scoped follow-up grants after the initial role list.

### PR breakdown (8 PRs)

| PR | Purpose |
|---|---|
| [#1498](https://github.com/openshift-online/gcp-hcp-infra/pull/1498) | Foundational: `tfc-access/commons/` access-project bootstrap; `hcp-terraform/gcp-hcp-commons/` meta workspace (both `gcp-hcp-commons` and `gcp-hcp-commons-dev`); removed `backend "gcs"` and added `cloud.tf` + `user_project_override`/`billing_project` to both configs; flipped commons's own `global_integration` read to `backend = "remote"` (left `global_stage` on GCS); deleted `modules/commons/tfc.tf`. Consumer flips were reverted out of this PR (chicken-and-egg — see gotchas) |
| [#1532](https://github.com/openshift-online/gcp-hcp-infra/pull/1532) | Re-added `gcp-hcp-commons` as a `remote_state_consumer_workspaces` entry on `gcp-hcp-global-integration` (authorizes commons's own `global_integration` read; reverted out of #1498 for the same chicken-and-egg reason) |
| [#1535](https://github.com/openshift-online/gcp-hcp-infra/pull/1535) | Attempted to flip both workspaces to remote execution by removing `execution_mode = "local"` — did **not** actually change execution mode (see #1541) |
| [#1538](https://github.com/openshift-online/gcp-hcp-infra/pull/1538) | Redid the reverted consumer flips: `global-integration`, `region-int-main-us-central1`, `region-int-main-us-south1` switched `data.terraform_remote_state.commons` from `backend = "gcs"` to `backend = "remote"`. Deliberately sequenced to merge **before** #1535 (CodeRabbit feedback) |
| [#1541](https://github.com/openshift-online/gcp-hcp-infra/pull/1541) | Root-caused #1535: omitting `execution_mode` does not default to `"remote"`; it falls back to the TFC project's own default, which was also `"local"`. Set `execution_mode = "remote"` **explicitly** on both workspaces |
| [#1542](https://github.com/openshift-online/gcp-hcp-infra/pull/1542) | First real remote run 403'd reading `gs://gcp-hcp-stg-global-terraform-state`; added `roles/storage.objectViewer` on that bucket to both SAs (SRE local apply against `tfc-access/commons`, which is `execution_mode="local"` by design) |
| [#1549](https://github.com/openshift-online/gcp-hcp-infra/pull/1549) | Next real-run failure: no permission to `actAs` `mintmaker-agent@gcp-hcp-commons`; added `roles/iam.serviceAccountUser` scoped to that SA |
| [#1556](https://github.com/openshift-online/gcp-hcp-infra/pull/1556) | Flipped `auto_apply = true` on both `gcp-hcp-commons` and `gcp-hcp-commons-dev` after clean first real runs — completes the migration |

### Operational work (not PRs)

- **State seeding** via `terraform state push` directly into both new workspaces (simpler than the raw State Versions API used elsewhere, since both were `execution_mode="local"` at seed time).
- **`terraform import`** of a pre-existing `google_artifact_registry_repository.gcp_hcp_images` in `commons-dev` that existed in GCP but not in the seeded state (`409: the repository already exists`).
- **Reconciled `commons-dev` brownfield drift** (Konflux WIF pool/provider, `konflux-push` SA, AR repo, label updates) in one apply — pre-existing drift, surfaced by finally running a real plan against it.

### Gotchas (recurring — carry into future brownfield migrations)

- **`execution_mode` must be set explicitly, never by omission** ([Migration Finding #13](#migration-findings) covers the mechanism and the "local blocks first-run auto-apply" rule). It bit twice on commons specifically: (1) `tfc-access/commons` auto-created as `execution_mode="remote"` (org default) instead of the `"local"` its siblings use, which broke the `file("${path.module}/../../metadata/...")` reads all `tfc-access/*` configs rely on (remote execution uploads only the working directory, not the whole repo); fixed by pinning the live workspace to `"local"`. (2) #1535 removed the explicit `"local"` expecting a `"remote"` default and instead inherited the project default (`"local"`) — nothing changed until #1541 set it explicitly.
- **Cross-workspace consumer authorization must be sequenced around producer creation.** Bundling "create `gcp-hcp-commons`" and "flip consumers to read it via `backend = "remote"`" in one PR (#1498) made every speculative plan fail (the workspace doesn't exist yet at review time). The producer must exist before consumers can be authorized against it (#1532) and before consumers can flip their reads (#1538). This is the [Finding #7](#migration-findings) consumer-grant requirement applied to a producer that is itself being created in the same change.

### Known follow-ups (not blocking)

- **Mintmaker Vertex AI Reasoning Engine update timeouts**: `google_vertex_ai_reasoning_engine.agent` shows drift on nearly every plan (non-deterministic `source_code_spec`) and its update operation appears to hit a timeout under TFC remote execution. Pre-existing, unrelated to the migration; owned by Jim. Expect periodic "Apply errored" notifications on `gcp-hcp-commons` until fixed. Worth its own ticket.
- **Temporary personal IAM grants** used to bootstrap `tfc-access/commons` and reconcile `commons-dev` drift are now redundant (the WIF identity handles everything end-to-end) but revocation was deferred.
- **`commons-dev` drift discipline**: its brownfield drift reconciliation was a one-time catch-up, not a structural fix. Consider whether it needs the same `auto_apply` cadence discipline as `commons` or a periodic drift-detection job so it doesn't silently drift again.

---

## Remaining Scope: Org-Level Terraform Config (GCP-1112) — 🔲 Not started

[GCP-1112](https://redhat.atlassian.net/browse/GCP-1112) — "Migrate org-level Terraform config to TFC" — **Status: New. Nothing has been implemented.**

`terraform/config/org` (org `428383927003`) manages the `org-admin-jit` PAM entitlement and the `custom.allowedPolicyMembers` org policy constraint. It is currently GCS-backed with manual-only applies (see its README).

This is scoped **separately** from commons (GCP-1111) and the other migrations on purpose: a TFC identity applying this config would need org-admin-adjacent IAM (PAM entitlement admin, org policy admin), a materially higher blast radius than any per-project access identity used so far. Because of that, **an explicit security review is a blocking prerequisite** before any implementation. The migration approach (access-project shape, IAM scoping, guardrails) is deliberately not designed here and should not be until that review is complete.

---

## Migration Findings

Workspace migrations uncovered several undocumented requirements. The first batch came from `gcp-hcp-global-integration` ([GCP-534](https://redhat.atlassian.net/browse/GCP-534), [PR #1070](https://github.com/openshift-online/gcp-hcp-infra/pull/1070)). Finding #6 was discovered during the region/MC integration cutover ([GCP-951](https://redhat.atlassian.net/browse/GCP-951)). Findings 7-10 surfaced during the [GCP-535](https://redhat.atlassian.net/browse/GCP-535) region/MC onboarding automation work (us-west1 and us-south1 bootstrap, Aug 2026). Finding #11 surfaced during the [GCP-1093](https://redhat.atlassian.net/browse/GCP-1093) CI migration (hypershift-ci/platform-ci, brownfield projects with pre-existing state). Finding #12 surfaced during the [GCP-1094](https://redhat.atlassian.net/browse/GCP-1094) `pagerduty` migration (a workspace that reads a Secret Manager secret at plan time). Finding #13 surfaced during the [GCP-1111](https://redhat.atlassian.net/browse/GCP-1111) commons migration (`execution_mode` must be set explicitly). These findings have been integrated into the stories above and are summarized here for reference.

| # | Finding | Impact | Resolution | Reference |
|---|---------|--------|------------|-----------|
| 1 | TFC remote execution ignores `backend "gcs"` blocks — workspace starts with empty state | TFC sees zero resources, attempts to create all infrastructure from scratch | Seed state from GCS via State Versions API before first plan | [State Management](#3-state-management), Story 4 |
| 2 | WIF authentication checks API activation against the access project (WIF pool's project), which lacks target-project APIs | Plans fail with "API has not been used in project" errors | Set `user_project_override = true` + `billing_project` pointing to global project (not target, to avoid bootstrap chicken-and-egg). 10 APIs added to global module in [PR #1114](https://github.com/openshift-online/gcp-hcp-infra/pull/1114) | [API Activation](#api-activation-and-the-user_project_override-pattern), [GCP-990](https://redhat.atlassian.net/browse/GCP-990), [PR #1073](https://github.com/openshift-online/gcp-hcp-infra/pull/1073), [PR #1114](https://github.com/openshift-online/gcp-hcp-infra/pull/1114) |
| 3 | TFC does not run speculative plans on fork PRs | Contributors from forks get no plan feedback | Push branches to upstream repo | [Workflow Fit](#1-workflow-fit) |
| 4 | Workspaces created with `auto_apply = false` need explicit enablement after cutover | Merges to main do not auto-apply until flag is set | Enable `auto_apply = true` per workspace after validation | Story 5, [GCP-951](https://redhat.atlassian.net/browse/GCP-951) |
| 5 | Trigger prefixes missing shared module paths | Changes to shared modules do not trigger plans in dependent workspaces | Add `terraform/modules/{type}/` to trigger prefixes | Story 4 |
| 6 | Prow required status checks reference Atlantis projects that no longer exist after cutover | PRs blocked from merging because `atlantis-{env}/plan` and `atlantis-{env}/apply` never report | Update Prow config in `openshift/release` to remove Atlantis checks per environment | Story 5, [GCP-951](https://redhat.atlassian.net/browse/GCP-951) |
| 7 | HCP Terraform denies cross-workspace `terraform_remote_state` reads by default (no `remote_state_consumer_ids` grant) | A management-cluster workspace reading its region's TFC-native state fails until explicitly granted as a consumer | Added a `remote_state_consumers` input to the `workspaces/tfe` module, wired to `remote_state_consumer_ids` on `tfe_workspace_settings` | [infra-platform#164](https://github.com/openshift-online/infra-platform/pull/164)/[#166](https://github.com/openshift-online/infra-platform/pull/166), [GCP-1069](https://redhat.atlassian.net/browse/GCP-1069) |
| 8 | A region bootstrapped **TFC-only from creation** (no prior Atlantis) never writes state to GCS at all — its `backend "gcs"` block is dead code from day one, not just stale | Dependent configs reading `data.terraform_remote_state.region` via `backend = "gcs"` get `"object with no attributes"` — a harder failure than finding #1, since there's no snapshot to seed from | Switch the consumer's remote-state read to `backend = "remote"` (TFC-native) plus the consumer grant from finding #7 | [GCP-1069](https://redhat.atlassian.net/browse/GCP-1069), [PR #1287](https://github.com/openshift-online/gcp-hcp-infra/pull/1287) |
| 9 | Once a producer workspace's state migrates to TFC, its GCS object is frozen at migration day — any remaining `backend = "gcs"` consumer reads go stale silently, no error | Confirmed live drift on real workspaces: global's GCS snapshot was missing an entire region's `monitored_projects` entries; region's GCS snapshot was 12 applies behind TFC | Retroactively switched every remaining `backend = "gcs"` consumer read to `backend = "remote"` + consumer grant. **Sequencing rule for stage/prod**: never leave a consumer on `backend = "gcs"` after its producer cuts over to TFC — the backend cutover for consumers must happen as part of the same change that migrates the producer's state, not after | [PR #1362](https://github.com/openshift-online/gcp-hcp-infra/pull/1362)/[#1363](https://github.com/openshift-online/gcp-hcp-infra/pull/1363) |
| 10 | OPA deletion-approval lists are hand-maintained per teardown and silently go stale across `workspaces/tfe` module version bumps (e.g. `tfe_workspace_settings` added in v0.0.12) | A teardown's meta-workspace apply **stalls indefinitely** at `post_plan_awaiting_decision` instead of erroring — easy to miss, no alert | Manually add the missing deletion-approval entry per resource added by the module upgrade; no automated detection yet | [PR #1247](https://github.com/openshift-online/gcp-hcp-infra/pull/1247) |
| 11 | Brownfield migration: on a pre-existing project the TFC plan/apply SA has no project permissions until `tfc.tf`'s grants apply, so the first plan's refresh calls `getIamPolicy` on the existing `google_project_iam_member.*` resources and 403s | The first plan/apply cannot complete — it errors during refresh before producing a plan | Run the first plan/apply with `refresh=false` (plans only the new `tfc.tf` grants, 0 changes/destroys to existing resources). Before applying, confirm the plan contains only those expected grants and reject any other add/change/destroy, since `refresh=false` suppresses state refresh and does not by itself prove the absence of drift. Apply once to grant the SAs, after which a normal `refresh=true` plan must be a clean no-op (the real validation gate before enabling `auto_apply`) | [GCP-1093](https://redhat.atlassian.net/browse/GCP-1093), [PR #1391](https://github.com/openshift-online/gcp-hcp-infra/pull/1391)/[#1411](https://github.com/openshift-online/gcp-hcp-infra/pull/1411) |
| 12 | A provider that reads a Secret Manager secret via `data.google_secret_manager_secret_version` **inside the provider block** runs that read at plan time, and with no pinned `version` it resolves `"latest"`, which needs `secretmanager.versions.get` + `.list` (`roles/secretmanager.viewer`) on top of `.access` (`roles/secretmanager.secretAccessor`) | `secretAccessor` alone 403s at plan time on `secretmanager.versions.get`, so every plan fails before producing output even though the workspace manages no GCP resources | Grant both `roles/secretmanager.secretAccessor` and `roles/secretmanager.viewer` to the plan and apply SAs, scoped to the single secret (mirrors the working `e2e-gcp-deployer` accessor+viewer pattern) | [GCP-1094](https://redhat.atlassian.net/browse/GCP-1094), [PR #1434](https://github.com/openshift-online/gcp-hcp-infra/pull/1434)/[#1442](https://github.com/openshift-online/gcp-hcp-infra/pull/1442) |
| 13 | Omitting `execution_mode` on a `workspaces/tfe` workspace does **not** default it to `"remote"` — it removes the override and falls back to the TFC project's own default execution mode (often `"local"`). Separately, `execution_mode="local"` is what actually blocks a brownfield workspace's first VCS run from auto-confirming its apply — the module hardcodes `tfe_workspace_run`'s `apply.manual_confirm = false` for any non-local workspace, so `auto_apply=false` alone is insufficient | A workspace silently stays `"local"` when `"remote"` was intended (no error, just no change); or a brownfield workspace auto-applies its very first run before state is seeded/validated | Set `execution_mode` **explicitly** on all new and transitioning workspaces; keep it `"local"` until state is seeded and a clean plan is validated, then set `"remote"` explicitly | [GCP-1111](https://redhat.atlassian.net/browse/GCP-1111), [PR #1535](https://github.com/openshift-online/gcp-hcp-infra/pull/1535)/[#1541](https://github.com/openshift-online/gcp-hcp-infra/pull/1541) |

> **Resolved, no longer a concern**: an earlier IAM self-grant propagation race during greenfield bootstrap (`tfc_iam_ready` barrier not wired into all resources, [PR #1205](https://github.com/openshift-online/gcp-hcp-infra/pull/1205)/[#1216](https://github.com/openshift-online/gcp-hcp-infra/pull/1216)) was fixed upstream in the module. Sequential region-then-MC bootstrap via `infra.py` is validated working; concurrent bootstrap of multiple regions has not been tested.

---

## Migrating a New Config to TFC (Checklist)

The generalized runbook for migrating any Terraform config to HCP Terraform, distilled from Stories 1–5 and every [Migration Finding](#migration-findings) above. Use it for stage/production rollout (Stories 7–8) and any new config. Each step notes the finding that produced it. Copy this list into the migration ticket and check items off there.

> **Automation shortcut**: for **region/MC** configs, `scripts/infra.py new region|mc ...` ([GCP-535](https://redhat.atlassian.net/browse/GCP-535)) already generates the workspace entry in `hcp-terraform/gcp-hcp-{env}/main.tf` and the plan/apply SA cross-project IAM in the module's `tfc.tf`. The steps below are the full manual path for configs the generator does not cover (CI, tooling, service, commons, and future one-offs).

### 1. Decide the topology

- [ ] **Reuse an existing TFC project, or bootstrap a new dedicated one?**
  - **Reuse** (workspace inside an existing project — e.g. `service` → `gcp-hcp-integration`, `hypershift-ci`/`platform-ci` → `gcp-hcp-ci`) when an existing access project's plan/apply SAs already have — or can be granted — the IAM the config needs, and the config belongs in that project's scope.
  - **New dedicated project** (e.g. `pagerduty` → `gcp-hcp-tooling`, `commons` → `gcp-hcp-commons`) when the config needs a distinct identity or home. This requires a new access-project bootstrap. A dedicated identity is *required* whenever the config must be attribute-isolated: each access project's WIF provider is attribute-scoped to a single TFC project and its variable set attaches only to that project's workspaces, so identities cannot be shared across TFC projects.
- [ ] **If new dedicated project**: bootstrap the access project (`terraform/config/tfc-access/<name>/`) following [Story 1](#story-1-bootstrap-access-project-integration--complete) — `gcp-dynamic-creds` module (WIF pool, OIDC provider, plan/apply SAs, variable set with `apply_to_all_workspaces = true`), applied locally (sanctioned local-apply exception for access projects). Pin the access workspace to `execution_mode = "local"` explicitly (Finding #13; also required for its `file("${path.module}/../../metadata/...")` reads to work — remote execution uploads only the working directory).

### 2. IAM grants for the plan/apply SAs

- [ ] Add a `tfc.tf` (in-module) or `<config>-iam.tf` (in the access-project config) granting **both** plan and apply SAs the role set the config needs (unified plan=apply, so permission gaps surface at plan time — [Story 2](#story-2-cross-project-iam-for-plan-and-apply-sas-integration--complete)).
- [ ] Expect to derive the role set **iteratively** — plan for 2–3 rounds of narrowly-scoped follow-up grants after the first real runs (every migration so far needed them: service GCP-1092, commons GCP-1111). Prefer narrow, resource-scoped grants over broad catch-all roles.
- [ ] If the provider reads a **Secret Manager secret at plan time** (inside a provider block, unpinned version → `"latest"`): grant **both** `roles/secretmanager.secretAccessor` **and** `roles/secretmanager.viewer`, scoped to the single secret (**Finding #12**).

### 3. Config file changes

- [ ] Create a **`cloud.tf`** with the standard block (**GCP-1141**, [PR #1490](https://github.com/openshift-online/gcp-hcp-infra/pull/1490)); the workspace `name` must exactly match the meta-workspace registration:
  ```hcl
  terraform {
    cloud {
      organization = "hp-platform-engineering"
      workspaces {
        name = "gcp-hcp-{workspace-name}"
      }
    }
  }
  ```
- [ ] **Remove any stale `backend "gcs"` block** from `main.tf`. TFC ignores it when a `cloud` block is present, but leaving it invites accidental state migration and operator confusion (**GCP-1141**).
- [ ] Add `user_project_override = true` and `billing_project = <global project>` to **both** `google` and `google-beta` provider blocks, including aliased providers (**Finding #2** / [API Activation](#api-activation-and-the-user_project_override-pattern)). Derive the global project ID from metadata (provider blocks cannot reference data sources).
  - Caveat: **do not** add provider `default_labels` if the existing (Atlantis-era) resources were created without them — the label diff will break the `refresh=true` no-op validation gate (see `config/service/integration/main.tf`).
- [ ] Switch every `data.terraform_remote_state.*` read of a **TFC-managed producer** from `backend = "gcs"` to `backend = "remote"` (**Findings #8/#9**). Never leave a consumer on `backend = "gcs"` after its producer is on TFC — its GCS snapshot is frozen and goes stale silently.

### 4. Register the workspace (meta workspace)

- [ ] Add the workspace to `hcp-terraform/<project>/main.tf` via the `workspaces/tfe` module.
- [ ] Set **`execution_mode` explicitly** — never by omission (**Finding #13**). Use `"local"` for the seeding/validation phase, then flip to `"remote"` explicitly at cutover.
- [ ] Set `auto_apply = false` initially (**Finding #4**). Note: for a brownfield workspace, `auto_apply = false` alone does **not** stop the first VCS run from auto-confirming — `execution_mode = "local"` is what actually blocks it (**Finding #13**).
- [ ] Include shared module paths (`terraform/modules/{type}/`) in the workspace's trigger prefixes, not just the config path (**Finding #5**).
- [ ] **Remote-state consumer grants** (**Finding #7**): if this workspace reads another workspace's state, add it to that producer's `remote_state_consumer_workspaces`. If this workspace produces state others read, authorize those consumers. **Sequence around producer creation**: a producer must exist before consumers can be authorized against it or flip their reads — do not bundle "create producer" and "flip consumers" in one PR (commons chicken-and-egg, PRs #1498 → #1532/#1538).

### 5. State seeding (brownfield configs only)

- [ ] **Freeze the source of truth** first: if Atlantis-managed, disable/remove the config's autoplan entry in `atlantis-{env}.yaml` and keep it frozen through validation ([State Management](#3-state-management)).
- [ ] Seed state from GCS — either the [State Versions API](#3-state-management) (preserve serial + MD5 exactly), or `terraform state push` when the workspace is `execution_mode = "local"` (simpler; used for commons). Securely delete the downloaded `.tfstate` afterward.
- [ ] `terraform import` any resources that exist in GCP but not in the seeded state (e.g. an out-of-band AR repo — commons-dev).

### 6. Validation gate

- [ ] **Brownfield first run**: run with `refresh=false` (the SAs have no project IAM until `tfc.tf` applies — **Finding #11**). Confirm the plan contains **only** the expected new `tfc.tf` grants; reject any other add/change/destroy (`refresh=false` suppresses drift detection, so this manual check is the safeguard). Apply once to grant the SAs.
- [ ] Then a **`refresh=true` plan must be a clean no-op** — this is the real gate before enabling `auto_apply`.

### 7. Cutover

- [ ] Set `execution_mode = "remote"` **explicitly** (**Finding #13**).
- [ ] Set `auto_apply = true`.
- [ ] Remove the config from `atlantis-{env}.yaml` **in the same change as the backend switch** — leaving both active split-brains the state (Atlantis cannot read TFC remote state). For configs with remote-state consumers, this must also coincide with the consumers' `backend = "remote"` flip (**Finding #9**).
- [ ] Update Prow **required status checks** in [`openshift/release`](https://github.com/openshift/release/blob/main/core-services/prow/02_config/openshift-online/gcp-hcp-infra/_prowconfig.yaml) to drop `atlantis-{env}/plan` and `atlantis-{env}/apply` for the environment, or PRs will be blocked (**Finding #6**).

### 8. Cleanup & traceability

- [ ] Verify `terraform init` against the real workspace reports **no state changes** (confirms `cloud.tf` workspace name and state parity — GCP-1141).
- [ ] Remove transient migration/seeding comments; keep only the durable "why" (repo comment style).
- [ ] Link the cleanup PR/commit to [GCP-1141](https://redhat.atlassian.net/browse/GCP-1141) (or its successor) for audit traceability.
