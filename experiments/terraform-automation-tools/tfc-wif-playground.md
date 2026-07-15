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

These roles are needed because the playground Terraform config creates a GCS bucket (`roles/storage.admin`) and an IAM binding (`roles/resourcemanager.projectIamAdmin`).

### 2. Configure the HCP Terraform workspace

The workspace is defined in code at `gcp-hcp-infra/hcp-terraform/test-gcp-hcp-terraform/main.tf`:

```hcl
workspaces = {
  gcp-hcp-dev-playground = {
    terraform_version = "1.13.4"
    variables         = local.tfc_wif_variables
    working_directory = "terraform/config/playground"
    github_repo_org   = "openshift-online"
    github_repo_name  = "gcp-hcp-infra"
    branch            = "gcp-hcp-dev-playground"
  }
}
```

The workspace tracks the `gcp-hcp-dev-playground` branch and runs Terraform from `terraform/config/playground/`.

### 3. Configure Dynamic Provider Credentials

In the HCP Terraform workspace UI, configure a **GCP Dynamic Provider Credential set** ("Default provider instance") with:

| Setting | Value |
|---------|-------|
| Project ID | (the GCP project number for `gcp-hcp-commons`: `573522191771`) |
| Pool ID | `tfc-pool` |
| Provider ID | `tfc-oidc` |
| Service Account Email | `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com` |

The following environment variables are also set on the workspace (managed in code via `local.tfc_wif_variables`):

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
```
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

The HCP Terraform organization has a `gcp-hcp-deletion-protection` OPA policy that blocks any run containing resource destruction. This includes destroying `tfe_variable` resources when cleaning up workspace variables. Requires a policy override or exclusion for dev/playground workspaces.

### Terraform version must match

The `required_version` in the Terraform config must exactly match the version configured on the HCP Terraform workspace. A mismatch (e.g., config says `1.15.8` but workspace runs `1.15.7`) causes init failure before any plan runs.

## Authentication Flow

```
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
           ✓ google_storage_bucket (roles/storage.admin)
           ✓ google_project_iam_member (roles/resourcemanager.projectIamAdmin)
```

## Related PRs

| PR | Description |
|----|-------------|
| [#851](https://github.com/openshift-online/gcp-hcp-infra/pull/851) | Initial WIF audience env var addition |
| [#855](https://github.com/openshift-online/gcp-hcp-infra/pull/855) | Remove `allowed_audiences` (later reverted by #859) |
| [#859](https://github.com/openshift-online/gcp-hcp-infra/pull/859) | Restore `allowed_audiences` + add `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` |
| [#866](https://github.com/openshift-online/gcp-hcp-infra/pull/866) | Fix `required_version` mismatch |

## Related Design Decision

See [hcp-terraform-workload-identity-federation.md](../../design-decisions/hcp-terraform-workload-identity-federation.md) for the formal design decision documenting this authentication pattern.
