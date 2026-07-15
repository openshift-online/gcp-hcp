# HCP Terraform Workspace Architecture — Wireframe

**Status:** Draft / For Review
**Date:** July 2026

## TFC Hierarchy

```
ORG: hp-platform-engineering
│
├── Project: gcp-hcp-{env}                          (one per environment: integration, stage, production)
│   ├── Workspace: gcp-hcp-global-{env}             → terraform/config/global/{env}/main/{region}
│   ├── Workspace: gcp-hcp-region-{env}-{region}    → terraform/config/region/{env}/main/{region}
│   └── Workspace(s): gcp-hcp-mc-{env}-{sector}-{region}-{infra_id}
│                                                    → terraform/config/management-cluster/{env}/{sector}/{region}-{infra_id}
│
├── Project: gcp-hcp-tooling
│   └── Workspace: gcp-hcp-pagerduty                → terraform/config/pagerduty
│
└── Project: gcp-hcp-ci
    ├── Workspace: gcp-hcp-hypershift-ci             → terraform/config/hypershift-ci
    └── Workspace (ephemeral): gcp-hcp-e2e-{sha}    → terraform/config/ci/{ephemeral_folder}
```

## Atlantis → TFC Workspace Mapping

Current Atlantis projects (from `atlantis-integration.yaml`) map 1:1 to TFC workspaces:

| Atlantis Project | TFC Workspace | TFC Project | Working Directory |
|------------------|---------------|-------------|-------------------|
| `global-int-main-us-central1` | `gcp-hcp-global-integration` | `gcp-hcp-integration` | `terraform/config/global/integration/main/us-central1` |
| `region-int-main-us-central1` | `gcp-hcp-region-integration-us-central1` | `gcp-hcp-integration` | `terraform/config/region/integration/main/us-central1` |
| `mc-int-main-us-central1-yjiv` | `gcp-hcp-mc-integration-main-us-central1-yjiv` | `gcp-hcp-integration` | `terraform/config/management-cluster/integration/main/us-central1-yjiv` |
| `pagerduty` | `gcp-hcp-pagerduty` | `gcp-hcp-tooling` | `terraform/config/pagerduty` |
| `hypershift-ci` | `gcp-hcp-hypershift-ci` | `gcp-hcp-ci` | `terraform/config/hypershift-ci` |

Stage workspaces follow the same pattern (already have configs under `terraform/config/*/stage/`).

## WIF Authentication — Per-Environment Service Accounts

Each TFC project authenticates to its corresponding GCP environment via WIF. The commons WIF pool (`tfc-pool`) is shared, but each environment gets its own service account for blast-radius isolation.

```
gcp-hcp-commons (WIF Pool: tfc-pool, Provider: tfc-oidc)
│
├── SA: tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com
│   └── Used by: gcp-hcp-tooling project (pagerduty, etc.)
│
├── SA: tfc-int@gcp-hcp-commons.iam.gserviceaccount.com
│   └── Used by: gcp-hcp-integration project workspaces
│       └── IAM roles granted on: gcp-hcp-int-global, int-reg-*, int-mgt-* projects
│
├── SA: tfc-stg@gcp-hcp-commons.iam.gserviceaccount.com
│   └── Used by: gcp-hcp-stage project workspaces
│       └── IAM roles granted on: gcp-hcp-stg-global, stg-reg-*, stg-mgt-* projects
│
├── SA: tfc-prd@gcp-hcp-commons.iam.gserviceaccount.com
│   └── Used by: gcp-hcp-production project workspaces
│       └── IAM roles granted on: gcp-hcp-prd-global, prd-reg-*, prd-mgt-* projects
│
└── SA: tfc-ci@gcp-hcp-commons.iam.gserviceaccount.com
    └── Used by: gcp-hcp-ci project workspaces
        └── IAM roles granted on: CI-specific projects
```

**Open question:** Do we use one SA per environment or one shared SA with attribute conditions restricting which TFC project can impersonate it? Per-environment SAs are simpler to reason about and audit but require more IAM bindings to maintain.

### WIF Variable Sets

TFC variable sets avoid duplicating WIF variables across every workspace in a project:

| Variable Set | Scope | Variables |
|---|---|---|
| `wif-integration` | All workspaces in `gcp-hcp-integration` project | `TFC_GCP_PROVIDER_AUTH`, `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` (tfc-int@...), `TFC_GCP_WORKLOAD_PROVIDER_NAME`, `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` |
| `wif-stage` | All workspaces in `gcp-hcp-stage` project | Same keys, stage SA |
| `wif-production` | All workspaces in `gcp-hcp-production` project | Same keys, production SA |
| `wif-tooling` | All workspaces in `gcp-hcp-tooling` project | Same keys, tfc-automation SA |
| `wif-ci` | All workspaces in `gcp-hcp-ci` project | Same keys, tfc-ci SA |

This means individual workspaces don't set WIF variables at all — they inherit from the project-level variable set.

## Scaling: New Regions and Environments

### Adding a new region (e.g., `us-west1` to integration)

`scripts/infra.py` already handles template-based config creation. The extension for TFC:

1. `infra.py new region integration main us-west1` — generates config (unchanged)
2. `infra.py` also adds a workspace entry to the `tfe` module config for `gcp-hcp-integration`:
   ```hcl
   gcp-hcp-region-integration-us-west1 = {
     terraform_version = "1.14.3"
     working_directory = "terraform/config/region/integration/main/us-west1"
   }
   ```
3. WIF variables are inherited from the `wif-integration` variable set (no per-workspace config)
4. Cross-project IAM for the TFC SA is handled by the region module's bootstrap flow (same as Atlantis today)

### Adding a new environment (e.g., production)

1. Create TFC project `gcp-hcp-production` via the `tfe` module
2. Create SA `tfc-prd@gcp-hcp-commons.iam.gserviceaccount.com` in the commons module
3. Grant `roles/iam.workloadIdentityUser` on the WIF pool to the new SA
4. Create variable set `wif-production` scoped to the new project
5. Add workspace entries for global, region, and MC configs

## Programmatic Workspace Management

### Code layout

```
hcp-terraform/
├── meta/                        # Meta workspace (manages all other workspaces)
│   └── main.tf                  # tfe provider config, manages projects
├── gcp-hcp-integration/
│   └── main.tf                  # Workspaces for integration env
├── gcp-hcp-stage/
│   └── main.tf                  # Workspaces for stage env
├── gcp-hcp-tooling/
│   └── main.tf                  # Workspaces for pagerduty, etc.
└── gcp-hcp-ci/
    └── main.tf                  # Workspaces for CI/e2e
```

Each file uses the `hp-platform-engineering/workspaces/tfe` module. Example for integration:

```hcl
module "gcp-hcp-integration" {
  source            = "app.terraform.io/hp-platform-engineering/workspaces/tfe"
  organization      = "hp-platform-engineering"
  project_name      = "gcp-hcp-integration"
  meta_project_name = "meta-gcp-hcp"
  notification_url  = var.notification_url

  workspaces = {
    gcp-hcp-global-integration = {
      working_directory = "terraform/config/global/integration/main/us-central1"
    }
    gcp-hcp-region-integration-us-central1 = {
      working_directory = "terraform/config/region/integration/main/us-central1"
    }
    gcp-hcp-mc-integration-main-us-central1-yjiv = {
      working_directory = "terraform/config/management-cluster/integration/main/us-central1-yjiv"
    }
  }
}
```

### E2E ephemeral workspaces (gcp-hcp-ci)

E2E workspaces are created and destroyed per pipeline run. Options:

- **TFC API**: Tekton pipeline creates workspace via TFC API, runs plan/apply, destroys workspace on completion
- **tfe provider with dynamic workspaces**: A dedicated Terraform config creates/destroys workspaces based on input variables

This is the most different from current Atlantis flow and needs further investigation.

## State Management

**Recommendation: Keep GCS backend initially.**

- Current state lives in GCS (`gcp-hcp-{env}-global-terraform-state`)
- TFC can use GCS as a remote backend — no migration needed
- Avoids the risk of state migration during the Atlantis → TFC transition
- Can evaluate TFC-managed state later once the workflow is stable

The `cloud {}` backend block (used in the playground) and `backend "gcs" {}` are mutually exclusive. During migration, workspaces can keep `backend "gcs"` and TFC runs the plan/apply remotely while state stays in GCS.

**Open question:** TFC-managed state gives features like state versioning, locking, and drift detection built-in. Worth evaluating post-migration.

## What This Doesn't Cover Yet

- **RBAC model**: Who can approve applies per project/workspace (section 2 outstanding item)
- **OPA/Sentinel policies**: How to replicate or improve on the current `gcp-hcp-deletion-protection` policy
- **Pre-apply hooks**: Replacing `hack/check-pr-labels.sh` with TFC run tasks or policy checks
- **Drift detection**: TFC native capability, needs evaluation
- **Migration ordering**: Which workspaces to migrate first (likely tooling → integration → stage → production)
- **Parallel Atlantis/TFC operation**: Can both systems target the same state during migration?
