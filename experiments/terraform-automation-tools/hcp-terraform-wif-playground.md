# HCP Terraform + WIF Playground: Dynamic Provider Credentials Validation

**Status:** Validated (Phase 1 and Phase 2)
**Date:** July 2026

> **Note:** For the prior implementation of this document (Phase 1 step-by-step + Phase 2 direct WIF proposal), see commit `cd414e2`.

## Phase 1: SA Impersonation via Commons Pool

**Status:** Validated (2026-07-15)

### Objective

Validate that HCP Terraform can authenticate to GCP via Workload Identity Federation (WIF) using Dynamic Provider Credentials and provision real GCP resources in a developer project (`rflores-dev`), using the WIF pool and service account already established in the commons project (`gcp-hcp-commons`).

### What We Proved

* Dynamic Provider Credentials with OIDC -> GCP WIF works end-to-end
* Cross-project operations: a service account in `gcp-hcp-commons` can manage resources in `rflores-dev` after granting the necessary IAM roles
* Project-level variable sets deliver WIF credentials automatically to new workspaces (verified with a second playground workspace with `variables = []`)

### Key Finding: Audience Mismatch

HCP Terraform Dynamic Provider Credentials use `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` — **not** `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` (legacy env var flow). Using the wrong variable causes the token to be sent with the WIF provider resource name as the audience, which does not match `allowed_audiences` on the GCP side.

**Error:** `invalid_grant: The audience in ID Token [...] does not match the expected audience.`

**Fix:** Add `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE = "https://app.terraform.io"` to the workspace.

### Other Findings

* **Cross-project IAM is manual**: The WIF SA lives in commons but needs roles on whatever target project it manages. These must be granted before the first apply.
* **OPA deletion protection**: Blocks `tfe_variable` destruction. Bypass via `_deletion_approvals` in `terraform.auto.tfvars`.
* **Variable set requires both `organization` and `parent_project_id`**: Omitting either causes different errors.
* **Terraform version must match**: `required_version` must match the TFC workspace version exactly.

### Related PRs

| PR | Description |
|----|-------------|
| [#851](https://github.com/openshift-online/gcp-hcp-infra/pull/851) | Initial WIF audience env var addition |
| [#855](https://github.com/openshift-online/gcp-hcp-infra/pull/855) | Remove `allowed_audiences` (later reverted by #859) |
| [#859](https://github.com/openshift-online/gcp-hcp-infra/pull/859) | Restore `allowed_audiences` + add `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` |
| [#866](https://github.com/openshift-online/gcp-hcp-infra/pull/866) | Fix `required_version` mismatch |
| [#873](https://github.com/openshift-online/gcp-hcp-infra/pull/873) | Add project-level WIF variable set (initial attempt) |
| [#875](https://github.com/openshift-online/gcp-hcp-infra/pull/875) | Fix: scope variable set to project instead of org |
| [#877](https://github.com/openshift-online/gcp-hcp-infra/pull/877) | Fix: add `organization` to project-scoped variable set |
| [#878](https://github.com/openshift-online/gcp-hcp-infra/pull/878) | Test: second playground workspace to verify variable set inheritance |

---

## Phase 2: Adopt infra-platform `terraform-tfe-gcp-dynamic-creds` Module

**Status:** Validated (2026-07-21)

### Background

The app-sre team published a reusable `terraform-tfe-gcp-dynamic-creds` module in [infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90) that automates WIF pool, service account, and variable set lifecycle. After reviewing the module and discussing with Jim, the team decided to adopt it instead of pursuing direct Workload Identity (`principal://` bindings without SAs).

The earlier Phase 2 proposal (2026-07-17) to use `TFC_GCP_PRINCIPAL_TYPE = workload_pool` with direct `principal://` bindings was abandoned. See [HCP Terraform WIF design decision](../../design-decisions/automation/hcp-terraform-workload-identity-federation.md) for the full rationale.

### What Changed from Phase 1

| Aspect | Phase 1 (Playground) | Phase 2 (Module) |
|---|---|---|
| WIF pool location | `gcp-hcp-commons` (single pool) | Per-environment global project (one pool per role group) |
| Service accounts | Single `tfc-automation` SA in commons | Per-role-group plan/apply SAs in target project |
| Attribute condition | Org-wide (`organization_name == ...`) | Workspace-scoped (`sub.startsWith(...)` per role group) |
| Variable sets | Hand-managed `tfe_variable_set` | Module-managed, auto-attached via `apply_to_all_workspaces` |
| Audience | Explicit `allowed_audiences` + `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` | Default audience behavior (neither side sets it) |
| Variable set contents | `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` (single SA) | `TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL` + `TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL` (separate plan/apply) |

### Experiment Setup

Two workspaces in the `test-gcp-hcp-terraform` TFC project ([gcp-hcp-infra#907](https://github.com/openshift-online/gcp-hcp-infra/pull/907)):

| Workspace | Config | Purpose |
|---|---|---|
| `test-gcp-dynamic-creds` | `terraform/config/tfc-wif-experiment/` | Calls the module against `rflores-dev` — creates WIF pool, SAs, variable sets |
| `test-gcp-wif-validation` | `terraform/config/tfc-wif-validation/` | Creates a GCS bucket with zero WIF variables — proves `apply_to_all_workspaces` inheritance |

The experiment workspace authenticated via the existing commons WIF pool (Phase 1 credentials) to bootstrap the new per-project WIF infrastructure. The validation workspace had no explicit WIF variables — it relied entirely on the module-created variable set.

Module was sourced from a temporary copy on `openshift-online/gcp-hcp` branch `tfc-wif-module-copy` because CI couldn't source from `infra-platform` (both SSH and HTTPS failed due to host key verification and DNS resolution issues).

### E2E Walkthrough

This section captures the chronological sequence of the experiment — what we did, what broke, and how we resolved it. This is the operational narrative that the static findings below don't convey.

#### Step 1: Initial config and PR

Created the experiment config at `hcp-terraform/test-gcp-dynamic-creds/` with the module sourced via HTTPS from `github.com/openshift-online/infra-platform//...`, variables in a separate `variables.tf` + `terraform.tfvars`, a `cloud.tf` backend for TFC, and a workspace registered in the meta workspace at `hcp-terraform/test-gcp-hcp-terraform/main.tf` with `branch = "test-gcp-dynamic-creds"`. Opened as [gcp-hcp-infra#907](https://github.com/openshift-online/gcp-hcp-infra/pull/907).

#### Step 2: Code review and config restructuring

Jim reviewed the PR and asked why `working_directory` pointed at `hcp-terraform/` instead of `terraform/config/` — our repo convention is that deployment configs live under `terraform/config/`. Valid point. Moved the config to `terraform/config/tfc-wif-experiment/`. This also meant switching from `variables.tf` + `terraform.tfvars` to inlined locals, because `*.tfvars` is gitignored under `terraform/config/`.

CodeRabbit flagged three things: (1) module source not pinned — **fixed**, pinned to the infra-platform#90 merge commit (`58994fd`); (2) plan/apply role separation — **dismissed**, unified identity is intentional; (3) broad IAM defaults — **dismissed**, needed for validation on a dev project. Later flagged `public_access_prevention` on the validation bucket — **fixed**.

#### Step 3: TFE_TOKEN discovery

The module uses the `tfe` provider to create variable sets and look up TFC projects. The default TFC workspace token doesn't have enough permissions — even `data.tfe_project` lookups return 404 without a proper `TFE_TOKEN` (confirmed by infra-platform's own docs). Our `gcp-hcp-admin-creds` variable set has a `TFE_TOKEN` but it's scoped to `meta-gcp-hcp`, not accessible from `test-gcp-hcp-terraform`.

Consulted Jim — he approved creating a temporary `test-tfe-creds` variable set in the `test-gcp-hcp-terraform` TFC project via the UI, with the understanding it gets cleaned up after the experiment.

#### Step 4: Module sourcing failures

After merging PR #907, CI couldn't source the module from infra-platform:
- HTTPS source: `Could not resolve host: github.com` (DNS resolution failure on CI runner)
- Switched to SSH (`git@github.com:openshift-online/infra-platform.git//...`): `Host key verification failed`
- Workaround: Copied the module to `openshift-online/gcp-hcp` branch `tfc-wif-module-copy` at `hcp-terraform/modules/terraform-tfe-gcp-dynamic-creds/` and sourced from there via HTTPS. This worked because `gcp-hcp` is accessible to our CI.

#### Step 5: Version and branch issues

Two follow-up PRs to fix sequencing problems:
- [#908](https://github.com/openshift-online/gcp-hcp-infra/pull/908): Commented out `required_version` in configs and set `terraform_version = "1.15.7"` in workspace definitions. The configs had `1.15.8` but TFC workspaces needed `1.15.7`.
- [#911](https://github.com/openshift-online/gcp-hcp-infra/pull/911): Removed `branch = "test-gcp-dynamic-creds"` from workspace definitions. That branch only existed on the fork — after merging, TFC couldn't find it. Workspaces now track the default branch (main).

#### Step 6: First apply — IAM permission failure

The experiment workspace ran and failed: `Permission 'iam.workloadIdentityPools.create' denied on resource '//iam.googleapis.com/projects/rflores-dev/locations/global'`. The commons SA had roles from the Phase 1 playground (`roles/storage.admin`, `roles/resourcemanager.projectIamAdmin`) but the module also needs `roles/iam.workloadIdentityPoolAdmin` and `roles/iam.serviceAccountAdmin`. Granted both via `gcloud`.

#### Step 7: Second apply — partial apply poisoning (circular dependency)

Re-ran the apply. It failed again with `iam.serviceAccounts.getAccessToken` denied — a different error. Checked the TFC UI and found the root cause: the module had partially applied on the first attempt. It created the TFC resources (variable set with `apply_to_all_workspaces = true`) before the GCP resources (SAs). The variable set auto-attached to the experiment workspace itself, delivering `TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL = hcp-tf-default-plan@rflores-dev` and `TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL = hcp-tf-default-apply@rflores-dev` — SAs that didn't exist yet. On the re-run, TFC tried to impersonate those non-existent SAs instead of the commons SA.

**Fix:** Manually detached the `test-gcp-hcp-terraform-rflores-dev-default-gcp-dynamic-creds` variable set from the experiment workspace in the TFC UI. Re-ran — apply succeeded, all 22 resources created.

**Key lesson:** The module call must NOT live in the same TFC project as the workspaces it configures. For production, this is a non-issue because the module call lives in a separate bootstrap workspace.

#### Step 8: Validation workspace — IAM propagation delay

The validation workspace kicked off immediately after the experiment applied. It failed with `iam.serviceAccounts.getAccessToken` denied. Checked the TFC UI — the variable set was correctly delivered (confirmed `apply_to_all_workspaces` works), with all 4 `TFC_GCP_*` variables present and `TFC_GCP_WORKLOAD_PROVIDER_NAME` pointing at the new pool in `rflores-dev` (project `702934521445`), not the commons pool. The failure was IAM propagation delay — the `workloadIdentityUser` bindings on the newly created SAs hadn't taken effect yet.

Re-ran after ~60 seconds — the validation workspace authenticated through the module-created WIF pool and SAs and successfully created the GCS bucket. Experiment complete.

### What We Proved

1. **Module creates expected resources**: WIF pool (`hcp-tf-default`), OIDC provider, plan/apply SAs (`hcp-tf-default-plan`, `hcp-tf-default-apply`), project IAM bindings, variable set with 4 TFC env vars, project variable set for auto-attachment.

2. **Default audience works**: The module does not set `allowed_audiences` on the OIDC provider. HCP Terraform sends the provider resource name as the audience by default, and GCP accepts it when `allowed_audiences` is unset. No `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` variable needed. This is safe as long as neither side explicitly sets an audience.

3. **`apply_to_all_workspaces` works**: The validation workspace received the module-created variable set automatically — zero manual configuration. It authenticated through the module-created WIF pool and SAs, and successfully created a GCS bucket in `rflores-dev`.

4. **Workspace-scoped attribute conditions work**: The module's OIDC provider uses `assertion.sub.startsWith("organization:hp-platform-engineering:project:test-gcp-hcp-terraform:workspace:")`, restricting authentication to workspaces within the specified TFC project.

5. **`plan_roles = apply_roles` works**: Setting both to the same role list gives two SAs with identical permissions — functionally a unified identity with two SA objects.

### Issues Encountered

#### 1. Partial apply poisons the workspace (circular dependency)

**Severity:** High — blocks further applies until manually resolved.

The module creates both GCP resources (WIF pool, SAs) and TFC resources (variable sets). When GCP resources fail (e.g., missing IAM permissions), the TFC resources still get created. With `apply_to_all_workspaces = true`, the module-created variable set is auto-attached to ALL workspaces in the project — including the experiment workspace that's running the module.

The variable set contains `TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL` and `TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL` pointing at SAs that don't exist yet. On the next run, TFC tries to impersonate those non-existent SAs instead of the commons SA, causing `iam.serviceAccounts.getAccessToken` denied.

**Fix:** Manually detach the variable set from the experiment workspace in the TFC UI, then re-run.

**Production implication:** The module call must NOT live in the same TFC project as the workspaces it configures. For production, the module call lives in a separate workspace — either in the infra-platform tenant config or in a dedicated bootstrap workspace in `gcp-hcp-infra`. This eliminates the circular dependency.

#### 2. CI cannot source modules from infra-platform

**Severity:** Medium — workaround available.

Both HTTPS (`github.com/openshift-online/infra-platform//...`) and SSH (`git@github.com:openshift-online/infra-platform.git//...`) module sources failed in CI. HTTPS failed with DNS resolution errors; SSH failed with host key verification. 

**Workaround:** Copied the module to `openshift-online/gcp-hcp` branch `tfc-wif-module-copy` and sourced from there.

**Production resolution:** Use the TFC private registry (`app.terraform.io/hp-platform-engineering/gcp-dynamic-creds/tfe`) once the module is published via the infra-platform bootstrap workspace. The module is already registered in `registry_modules` in the bootstrap config.

#### 3. IAM permissions not pre-granted on rflores-dev

**Severity:** Low — one-time setup per project.

The commons SA had `roles/storage.admin` and `roles/resourcemanager.projectIamAdmin` from Phase 1, but the module also needs:
- `roles/iam.workloadIdentityPoolAdmin` — to create WIF pools
- `roles/iam.serviceAccountAdmin` — to create SAs
- `roles/serviceusage.serviceUsageAdmin` — to enable APIs

**Production implication:** When deploying per-environment, the SA (or whatever identity runs the module) needs these roles on the target project. For production, this is handled by the Atlantis/TFC bootstrap IAM flow.

#### 4. IAM propagation delay

**Severity:** Low — transient.

The validation workspace failed with `iam.serviceAccounts.getAccessToken` immediately after the experiment workspace applied. The `workloadIdentityUser` bindings on the newly created SAs hadn't propagated yet. Re-running after ~60 seconds succeeded.

**Production implication:** When deploying the module and immediately triggering dependent workspaces, expect a brief propagation delay. Not a blocking issue.

#### 5. Terraform version drift

**Severity:** Low — operational discipline.

`required_version` in configs must match `terraform_version` on the TFC workspace. Mismatches cause init failures. We hit this multiple times during the experiment. Commented out `required_version` as a workaround for the experiment.

**Production implication:** Pin versions consistently. The module supports `>= 1.13.4`, so any recent version works.

#### 6. Branch attribute in workspace definition

**Severity:** Low — sequencing mistake.

Workspace definitions referenced `branch = "test-gcp-dynamic-creds"` which only existed on the fork, not upstream. After merging the PR, the branch was deleted and workspace creation failed with "Branch doesn't exist".

**Fix:** Remove `branch` attribute — workspaces track the default branch (main).

### Module Variable Set Contents

The module creates one variable set per (role group, TFC project) pair. For our `default` role group:

| Variable | Value | Notes |
|---|---|---|
| `TFC_GCP_PROVIDER_AUTH` | `true` | Enables Dynamic Provider Credentials |
| `TFC_GCP_WORKLOAD_PROVIDER_NAME` | `projects/702934521445/locations/global/workloadIdentityPools/hcp-tf-default/providers/oidc` | Points at the module-created pool, not commons |
| `TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL` | `hcp-tf-default-plan@rflores-dev.iam.gserviceaccount.com` | Plan-phase SA |
| `TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL` | `hcp-tf-default-apply@rflores-dev.iam.gserviceaccount.com` | Apply-phase SA |

Notably absent: `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` (Phase 1 used this for a unified SA) and `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` (not needed with default audience behavior).

### Authentication Flow (Validated)

```text
HCP Terraform Workspace (test-gcp-wif-validation)
    |
    +-- 1. TFC generates OIDC token:
    |       issuer:   https://app.terraform.io
    |       audience: projects/702934521445/locations/global/workloadIdentityPools/hcp-tf-default/providers/oidc
    |                 (auto-matched, no explicit audience variable)
    |       subject:  organization:hp-platform-engineering:project:test-gcp-hcp-terraform:workspace:test-gcp-wif-validation:run_phase:apply
    |
    +-- 2. Token sent to GCP STS -> validated against module-created WIF provider in rflores-dev
    |       - issuer matches
    |       - audience matches (default = provider resource name)
    |       - attribute_condition: sub starts with "...project:test-gcp-hcp-terraform:workspace:"
    |
    +-- 3. STS returns federated token -> exchanged for apply SA access token
    |       SA: hcp-tf-default-apply@rflores-dev.iam.gserviceaccount.com
    |
    +-- 4. SA token used for GCP API calls
            - Created GCS bucket rflores-dev-tfc-wif-validation successfully
```

### Cleanup Checklist

- [ ] Destroy `test-gcp-dynamic-creds` workspace resources (`terraform destroy`)
- [ ] Destroy `test-gcp-wif-validation` workspace resources
- [ ] Remove both workspaces from `hcp-terraform/test-gcp-hcp-terraform/main.tf`
- [ ] Delete `test-tfe-creds` variable set from `test-gcp-hcp-terraform` TFC project
- [ ] Delete `terraform/config/tfc-wif-experiment/` directory
- [ ] Delete `terraform/config/tfc-wif-validation/` directory
- [ ] Delete `tfc-wif-module-copy` branch from `openshift-online/gcp-hcp`
- [ ] Remove additional IAM roles granted on `rflores-dev` for the commons SA (`workloadIdentityPoolAdmin`, `serviceAccountAdmin`)

### Related PRs

| PR | Description |
|----|-------------|
| [infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90) | Module merged — `terraform-tfe-gcp-dynamic-creds` |
| [gcp-hcp-infra#907](https://github.com/openshift-online/gcp-hcp-infra/pull/907) | Experiment: two workspaces calling the module |
| [gcp-hcp-infra#908](https://github.com/openshift-online/gcp-hcp-infra/pull/908) | Fix: pin terraform_version to 1.15.7 |
| [gcp-hcp-infra#911](https://github.com/openshift-online/gcp-hcp-infra/pull/911) | Fix: remove branch attribute from workspace definitions |

### Related Decisions

- [HCP Terraform WIF](../../design-decisions/automation/hcp-terraform-workload-identity-federation.md) — formal design decision
- [HCP Terraform Workspace Architecture](../../design-decisions/automation/hcp-terraform-workspace-architecture.md) — TFC project hierarchy
