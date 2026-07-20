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
    │
    ├─ 1. Run triggered by push to gcp-hcp-dev-playground branch
    │
    ├─ 2. Dynamic Provider Credentials generate OIDC token:
    │      issuer:   https://app.terraform.io
    │      audience: https://app.terraform.io  (from TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE)
    │      subject:  organization:hp-platform-engineering:project:...:workspace:gcp-hcp-dev-playground:...
    │
    ├─ 3. Token sent to GCP STS → validated against tfc-oidc provider in gcp-hcp-commons
    │      ✓ issuer matches
    │      ✓ audience matches allowed_audiences
    │      ✓ attribute_condition passes (organization name)
    │
    ├─ 4. STS returns federated token → exchanged for tfc-automation SA access token
    │
    └─ 5. Terraform uses SA token to manage resources in rflores-dev project
           ✓ google_storage_bucket (requires roles/storage.admin)
           ✓ google_project_iam_member (requires roles/resourcemanager.projectIamAdmin to manage IAM)
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

## Phase 2: Direct Workload Identity (No Service Account)

**Status:** Proposed — pending validation
**Date:** July 2026

### Background

During architecture review (2026-07-17), Pat proposed eliminating service accounts entirely by using HCP Terraform's `TFC_GCP_PRINCIPAL_TYPE = workload_pool` mode. Instead of authenticating through WIF and then impersonating a service account, TFC authenticates as a federated principal directly and gets IAM permissions via `principal://` or `principalSet://` bindings.

Jim confirmed the approach and agreed it expands the testing scope to validate `principal://` binding support across all GCP resource types we manage.

See [GCP Dynamic Provider Credentials — GCP Configuration](https://developer.hashicorp.com/terraform/cloud-docs/dynamic-provider-credentials/gcp-configuration) for the HashiCorp documentation.

### What Changes

**Current flow (Phase 1 — SA impersonation):**
```
TFC workspace → OIDC token → WIF Pool (org-level check) → impersonate tfc-automation SA → GCP resources
```

**Proposed flow (Phase 2 — direct WID):**
```
TFC workspace → OIDC token → WIF Pool (project-scoped check) → direct principal access → GCP resources
```

#### Attribute Condition

The current attribute condition is org-wide — any workspace in the org can authenticate:

```hcl
attribute_condition = "assertion.terraform_organization_name == \"hp-platform-engineering\""
```

The proposed condition scopes access to a specific TFC project using the `sub` claim:

```hcl
attribute_condition = "assertion.sub.startsWith(\"organization:hp-platform-engineering:project:gcp-hcp-integration:\")"
```

The TFC `sub` claim format is `organization:{org}:project:{project}:workspace:{workspace}:run_phase:{phase}`, so `startsWith` at the project level restricts authentication to only workspaces within that TFC project.

#### Variable Sets

Phase 1 variable sets carry SA-specific variables:

| Variable | Phase 1 (SA) | Phase 2 (Direct WID) |
|----------|-------------|---------------------|
| `TFC_GCP_PROVIDER_AUTH` | `true` | `true` |
| `TFC_GCP_PRINCIPAL_TYPE` | (omitted, defaults to `service_account`) | `workload_pool` |
| `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` | `tfc-automation@...` | (not needed) |
| `TFC_GCP_WORKLOAD_PROVIDER_NAME` | full provider resource name | full provider resource name |
| `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` | `https://app.terraform.io` | `https://app.terraform.io` |

#### IAM Bindings

Phase 1 grants IAM roles to the service account:

```hcl
member = "serviceAccount:tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com"
```

Phase 2 grants IAM roles directly to the federated principal:

```hcl
member = "principalSet://iam.googleapis.com/projects/{project_number}/locations/global/workloadIdentityPools/tfc-pool/attribute.terraform_project_name/gcp-hcp-integration"
```

Or scoped to a specific workspace:

```hcl
member = "principal://iam.googleapis.com/projects/{project_number}/locations/global/workloadIdentityPools/tfc-pool/subject/organization:hp-platform-engineering:project:gcp-hcp-integration:workspace:gcp-hcp-region-integration-us-central1:run_phase:apply"
```

#### Resources Eliminated

The following resources in `terraform/modules/commons/tfc.tf` would be removed:

- `google_service_account.tfc` — the `tfc-automation` SA
- `google_service_account_iam_member.tfc_wif_impersonation` — the WIF→SA impersonation binding

### Proposed Authentication Flow

```text
HCP Terraform Workspace (gcp-hcp-region-integration-us-central1)
    │
    ├─ 1. Run triggered by push
    │
    ├─ 2. Dynamic Provider Credentials generate OIDC token:
    │      issuer:   https://app.terraform.io
    │      audience: https://app.terraform.io
    │      subject:  organization:hp-platform-engineering:project:gcp-hcp-integration:workspace:...
    │
    ├─ 3. Token sent to GCP STS → validated against WIF provider in gcp-hcp-int-global
    │      ✓ issuer matches
    │      ✓ audience matches allowed_audiences
    │      ✓ attribute_condition: sub starts with "...project:gcp-hcp-integration:"
    │
    ├─ 4. STS returns federated access token (no SA impersonation)
    │
    └─ 5. Terraform uses federated token directly to manage resources
           ✓ IAM granted via principalSet:// bindings on target project
```

### Revised WIF Pool Topology

Each environment gets its own WIF pool in its global project (not a single pool in commons):

| WIF Pool Location | TFC Project Served | Attribute Condition |
|---|---|---|
| `gcp-hcp-int-global` | `gcp-hcp-integration` | `assertion.sub.startsWith("...project:gcp-hcp-integration:")` |
| `gcp-hcp-stg-global` | `gcp-hcp-stage` | `assertion.sub.startsWith("...project:gcp-hcp-stage:")` |
| `gcp-hcp-prd-global` | `gcp-hcp-production` | `assertion.sub.startsWith("...project:gcp-hcp-production:")` |
| CI projects (extend Prow pools) | `gcp-hcp-ci` | Extend existing pools with TFC OIDC provider |
| `gcp-hcp-commons` | `gcp-hcp-tooling` (agents only) | `assertion.sub.startsWith("...project:gcp-hcp-tooling:")` |

PagerDuty workspace does not need GCP IAM — it uses a PagerDuty API key only.

### Additional Decisions

- **Same identity for plan and apply** — no need for separate plan/apply principals.
- **CI workspaces reuse existing Prow WIF pools** — extend with a TFC OIDC provider instead of two-hop impersonation. CI workspaces may need workspace-specific env vars (not all via variable sets) since each targets a different IAM case.

### Validation Plan

#### 1. Validate `principal://` binding support for all managed resource types

Need to confirm that GCP accepts `principal://` or `principalSet://` members for IAM bindings on all resource types we manage. Some older GCP APIs may only accept `serviceAccount:` or `user:` member types.

**Resource types to test** (from the Atlantis IAM role table):

| Resource Type | Required Role | `principal://` Supported? |
|---|---|---|
| `google_project_service` | `roles/serviceusage.serviceUsageAdmin` | ? |
| `google_container_*` (GKE) | `roles/container.admin` | ? |
| `google_compute_*` | `roles/compute.networkAdmin`, `roles/compute.instanceAdmin.v1` | ? |
| `google_service_account` | `roles/iam.serviceAccountAdmin` | ? |
| `google_project_iam_*` | `roles/resourcemanager.projectIamAdmin` | ? |
| `google_dns_*` | `roles/dns.admin` | ? |
| `google_secret_manager_*` | `roles/secretmanager.admin` | ? |
| `google_workflows_*` | `roles/workflows.admin` | ? |
| `google_cloud_run_*` | `roles/run.admin` | ? |
| `google_pubsub_*` | `roles/pubsub.admin` | ? |
| `google_eventarc_*` | `roles/eventarc.admin` | ? |
| `google_tags_*` | `roles/resourcemanager.tagAdmin` | ? |
| PAM entitlements | `roles/privilegedaccessmanager.admin` | ? |
| `google_monitoring_*` | `roles/monitoring.metricsScopesAdmin` | ? |

**Test approach:** In the playground workspace, switch to `TFC_GCP_PRINCIPAL_TYPE = workload_pool`, grant `principalSet://` IAM bindings on `rflores-dev`, and attempt to provision each resource type.

#### 2. Workspace cleanup

The current playground workspaces (`gcp-hcp-dev-playground`, `gcp-hcp-dev-playground-2`) use SA impersonation. To properly test direct WID:

1. Remove or update the playground variable set to drop `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL`
2. Add `TFC_GCP_PRINCIPAL_TYPE = workload_pool` to the variable set
3. Grant `principalSet://` IAM bindings on `rflores-dev` for the pool principal
4. Trigger a run and verify authentication works without any SA

#### 3. Per-environment pool setup

Once direct WID is validated in the playground, test the per-environment pool pattern:

1. Create a WIF pool + OIDC provider in `rflores-dev` (simulating a per-env global project)
2. Set the attribute condition to scope by TFC project
3. Point the playground workspace at the new pool
4. Verify isolation — a workspace in a different TFC project should fail to authenticate

### Open Questions

- Does GCP enforce any rate limits or quotas on federated principal tokens differently than SA tokens?
- Are there any GCP Console UX differences when viewing resources created by a federated principal vs a service account (e.g., audit log display, IAM policy readability)?
- For CI: can we add a TFC OIDC provider to the existing Prow WIF pool, or do pool-level attribute conditions conflict?

## Related Design Decision

See [hcp-terraform-workload-identity-federation.md](../../design-decisions/automation/hcp-terraform-workload-identity-federation.md) for the formal design decision documenting this authentication pattern.
