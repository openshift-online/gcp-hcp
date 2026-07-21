# HCP Terraform + WIF Playground: Provisioning GCP Resources via Dynamic Provider Credentials

**Status:** Validated
**Date:** July 2026

## Objective

Validate that HCP Terraform can authenticate to GCP via Workload Identity Federation (WIF) using Dynamic Provider Credentials and provision real GCP resources in a developer project (`rflores-dev`), using the WIF pool and service account already established in the commons project (`gcp-hcp-commons`).

## Prerequisites

- An HCP Terraform organization (`hp-platform-engineering`) with a workspace (`gcp-hcp-dev-playground`)
- The commons WIF infrastructure already deployed:
  - Workload Identity Pool: `tfc-pool` in `gcp-hcp-commons`
  - OIDC Provider: `tfc-oidc` with `issuer_uri = "https://app.terraform.io"` and `allowed_audiences = ["https://app.terraform.io"]`
  - Service Account: `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com`
- A target GCP project (`rflores-dev`) where resources will be created
- `gcloud` CLI authenticated with sufficient permissions on the target project

## Setup Steps

### 1. Grant the TFC service account IAM roles on the target project

The `tfc-automation` service account lives in `gcp-hcp-commons` but needs permissions on the target project where resources will be created. These must be granted manually:

```bash
# Grant Storage Admin (for GCS buckets)
gcloud projects add-iam-policy-binding rflores-dev \
  --member="serviceAccount:tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com" \
  --role="roles/storage.admin"

# Grant Project IAM Admin (for managing project IAM bindings)
gcloud projects add-iam-policy-binding rflores-dev \
  --member="serviceAccount:tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com" \
  --role="roles/resourcemanager.projectIamAdmin"
```

`roles/storage.admin` is needed by the GCS bucket and IAM binding resources. `roles/resourcemanager.projectIamAdmin` is a prerequisite permission that allows the service account to create `google_project_iam_member` bindings; the Terraform config grants `roles/storage.admin`, not `projectIamAdmin`.

### 2. Configure the HCP Terraform workspace

The workspace is defined in code at `gcp-hcp-infra/hcp-terraform/test-gcp-hcp-terraform/main.tf`:

```hcl
workspaces = {
  gcp-hcp-dev-playground = {
    terraform_version = "1.13.4"
    variables         = []
    working_directory = "terraform/config/playground"
    github_repo_org   = "openshift-online"
    github_repo_name  = "gcp-hcp-infra"
    branch            = "gcp-hcp-dev-playground"
  }
}
```

The workspace tracks the `gcp-hcp-dev-playground` branch and runs Terraform from `terraform/config/playground/`. WIF variables are not set per-workspace — they are inherited from a project-level variable set (see below).

### 3. Configure Dynamic Provider Credentials

In the HCP Terraform workspace UI, configure a **GCP Dynamic Provider Credential set** ("Default provider instance") with:

| Setting | Value |
|---------|-------|
| Project ID | (the GCP project number for `gcp-hcp-commons`: `573522191771`) |
| Pool ID | `tfc-pool` |
| Provider ID | `tfc-oidc` |
| Service Account Email | `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com` |

The following environment variables are inherited by the workspace from a project-level variable set (`wif-test`), managed in code via `tfe_variable_set` and `tfe_project_variable_set` resources:

| Variable | Value | Purpose |
|----------|-------|---------|
| `TFC_GCP_PROVIDER_AUTH` | `true` | Enables WIF authentication |
| `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` | `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com` | SA to impersonate |
| `TFC_GCP_WORKLOAD_PROVIDER_NAME` | `projects/573522191771/locations/global/workloadIdentityPools/tfc-pool/providers/tfc-oidc` | Full WIF provider resource name |
| `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` | `https://app.terraform.io` | OIDC token audience for Dynamic Provider Credentials |

### 4. Write the Terraform config

The playground config (`terraform/config/playground/main.tf`) on the `gcp-hcp-dev-playground` branch:

```hcl
terraform {
  required_version = "1.13.4"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "6.37.0"
    }
  }
}

provider "google" {
  project = var.project
}

resource "google_project_iam_member" "tfc_storage_admin" {
  project = var.project
  role    = "roles/storage.admin"
  member  = "serviceAccount:${var.service_account_email}"
}

module "gcs_bucket" {
  source = "../../modules/gcs-bucket"

  project       = var.project
  name          = var.bucket_name
  location      = var.location
  force_destroy = true
}
```

The `cloud` backend block (`terraform/config/playground/cloud.tf`) connects to HCP Terraform:

```hcl
terraform {
  cloud {
    organization = "hp-platform-engineering"
    workspaces {
      name = "gcp-hcp-dev-playground"
    }
  }
}
```

### 5. Trigger a run

Push a commit to the `gcp-hcp-dev-playground` branch. HCP Terraform automatically triggers a plan. After review, apply to create the resources.

## Findings

### Audience mismatch is the primary pitfall

The most significant issue encountered was an OIDC token audience mismatch. HCP Terraform's Dynamic Provider Credentials use `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` to set the OIDC token audience — **not** `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` (which is the legacy env var flow). Using the wrong variable name causes the token to be sent with the WIF provider resource name as the audience, which does not match `allowed_audiences` on the GCP side.

**Error observed:**
```text
oauth2/google: status code 400: {"error":"invalid_grant",
  "error_description":"The audience in ID Token
  [//iam.googleapis.com/projects/573522191771/locations/global/workloadIdentityPools/tfc-pool/providers/tfc-oidc]
  does not match the expected audience."}
```

**Root cause:** `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` was set but Dynamic Provider Credentials ignore it. The credential set was sending the default audience (provider resource name) instead of `https://app.terraform.io`.

**Fix:** Add `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE = "https://app.terraform.io"` to the workspace.

### Cross-project IAM is a manual step

The WIF service account (`tfc-automation`) lives in `gcp-hcp-commons` but needs IAM roles on whatever target project it manages. These cross-project bindings must be granted manually via `gcloud` before the first Terraform run. Without them, `terraform plan` succeeds (read-only) but `terraform apply` fails with permission denied.

### OPA deletion protection policy blocks variable cleanup

The HCP Terraform organization has a `gcp-hcp-deletion-protection` OPA policy that blocks any run containing resource destruction. This includes destroying `tfe_variable` resources when migrating from per-workspace variables to a variable set. The policy checks a `_deletion_approvals` variable in `terraform.auto.tfvars` for explicitly approved resource addresses with ISO 8601 expiration timestamps (e.g., `expires_at = "2026-08-14T23:59:59Z"`).

### Project-level variable sets require both `organization` and `parent_project_id`

When creating a `tfe_variable_set` scoped to a TFC project, the resource requires both `organization` (for API auth) and `parent_project_id` (for ownership scoping). Omitting `organization` produces `no organization was specified on the resource or provider`. Omitting `parent_project_id` and using only `organization` produces `resource not found` if the workspace token lacks org-level permissions.

### Variable set inheritance verified

A second test workspace (`gcp-hcp-dev-playground-2`) was created with `variables = []` to confirm that new workspaces automatically inherit WIF credentials from the project-level variable set. The workspace authenticated to GCP successfully without any per-workspace WIF variables, validating the pattern for scaling across environments.

### Terraform version must match

The `required_version` in the Terraform config must exactly match the version configured on the HCP Terraform workspace. A mismatch (e.g., config says `1.15.8` but workspace runs `1.15.7`) causes init failure before any plan runs.

## Authentication Flow

```text
HCP Terraform Workspace (gcp-hcp-dev-playground)
    |
    +-- 1. Run triggered by push to gcp-hcp-dev-playground branch
    |
    +-- 2. Dynamic Provider Credentials generate OIDC token:
    |       issuer:   https://app.terraform.io
    |       audience: https://app.terraform.io  (from TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE)
    |       subject:  organization:hp-platform-engineering:project:...:workspace:gcp-hcp-dev-playground:...
    |
    +-- 3. Token sent to GCP STS -> validated against tfc-oidc provider in gcp-hcp-commons
    |       - issuer matches
    |       - audience matches allowed_audiences
    |       - attribute_condition passes (organization name)
    |
    +-- 4. STS returns federated token -> exchanged for tfc-automation SA access token
    |
    +-- 5. Terraform uses SA token to manage resources in rflores-dev project
            - google_storage_bucket (requires roles/storage.admin)
            - google_project_iam_member (requires roles/resourcemanager.projectIamAdmin to manage IAM)
```

## Related PRs

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

**Status:** Planned
**Date:** July 2026

### Background

~~During architecture review (2026-07-17), Pat proposed eliminating service accounts entirely by using HCP Terraform's `TFC_GCP_PRINCIPAL_TYPE = workload_pool` mode.~~ The team initially planned to pursue direct Workload Identity (no SAs) as Phase 2. After reviewing the `terraform-tfe-gcp-dynamic-creds` module published by the app-sre team ([infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90)), the team decided to adopt the module's SA-based model instead. The module automates the error-prone parts of WIF setup and aligns with platform-wide tooling.

See [HCP Terraform WIF design decision](../../design-decisions/automation/hcp-terraform-workload-identity-federation.md) for the full rationale.

### What Changes from Phase 1

| Aspect | Phase 1 (Playground) | Phase 2 (Module Adoption) |
|---|---|---|
| WIF pool location | `gcp-hcp-commons` (single pool) | Per-environment global project (one pool per role group per env) |
| Service accounts | Single `tfc-automation` SA in commons | Per-role-group plan/apply SAs in each env global project |
| Attribute condition | Org-wide (`organization_name == ...`) | Workspace-scoped (`sub.startsWith(...)` per role group) |
| Variable sets | Hand-managed `tfe_variable_set` | Module-managed, auto-attached via `apply_to_all_workspaces` |
| Audience | Explicit `allowed_audiences` + `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` | Default audience behavior (neither side sets it) |
| Cross-project IAM | Manual `gcloud` commands | Supplementary `google_project_iam_member` in region/MC modules |
| CI | Commons SA with two-hop impersonation | Extend Prow WIF pools with TFC OIDC provider (no module) |

### Abandoned: Direct Workload Identity (No SA)

The original Phase 2 proposal to use `TFC_GCP_PRINCIPAL_TYPE = workload_pool` with `principal://` bindings is abandoned. Reasons:

1. **No module support**: The infra-platform module has no direct WID mode — it always creates SAs. Forking or maintaining custom WIF infrastructure increases operational burden.
2. **Unvalidated compatibility**: `principal://` binding support was never validated across all GCP resource types we manage (GKE, Compute, DNS, Secret Manager, Workflows, Pub/Sub, Eventarc, Cloud Run, Tags, PAM).
3. **SA model is proven**: The SA-based authentication model is well-understood, well-documented by HashiCorp, and used by every other team in the organization.

### Validation Plan

#### 1. Deploy module in dev environment

1. Add a module call in `hcp-terraform/gcp-hcp-dev/wif.tf` targeting `rflores-dev` project
2. Configure a `default` role group with `apply_to_all_workspaces = true`
3. Set `plan_roles = apply_roles` with scoped roles (not `roles/owner`)
4. Verify the module creates: WIF pool, OIDC provider, plan/apply SAs, variable set
5. Verify a workspace in the dev project can authenticate and provision resources

#### 2. Validate cross-project IAM workaround

1. Add supplementary `google_project_iam_member` resources granting the module-created SA roles on a second GCP project
2. Verify the workspace can provision resources in the second project
3. Document the pattern for region/MC module integration

#### 3. Validate `apply_to_all_workspaces`

1. Create a second workspace in the dev TFC project without any per-workspace variables
2. Verify it inherits WIF credentials from the auto-attached variable set
3. Verify authentication succeeds

#### 4. Validate audience behavior

1. Confirm the module's OIDC provider does not set `allowed_audiences`
2. Verify authentication works with default audience (provider resource name)
3. Compare with Phase 1's explicit audience approach — document any behavioral differences

### Open Questions

- Should dev environments use `roles/owner` for faster iteration, with scoped roles only for integration/stage/production?
- What is the module upgrade cadence — pin to exact version or use pessimistic constraint?
- How does the OPA deletion protection policy interact with module-managed resources?
