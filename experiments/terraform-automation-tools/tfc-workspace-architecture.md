# HCP Terraform Workspace Architecture — Wireframe

**Status:** Draft / For Review (Variable sets validated)
**Date:** July 2026

## TFC Hierarchy

```text
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
    ├── Workspace: gcp-hcp-platform-ci               → terraform/config/platform-ci
    └── Workspace (ephemeral): gcp-hcp-platform-{sha}
                                                      → terraform/config/platform-ci/{ephemeral_folder}
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
| *(new)* | `gcp-hcp-platform-ci` | `gcp-hcp-ci` | `terraform/config/platform-ci` |

Stage workspaces follow the same pattern (already have configs under `terraform/config/*/stage/`).

## WIF Authentication — Per-Environment Service Accounts

Each TFC project authenticates to GCP via WIF using the shared commons pool (`tfc-pool`). Environment service accounts live in each environment's own global project for blast-radius isolation, with cross-project `workloadIdentityUser` bindings on the commons pool. Tooling and CI use the commons SA directly.

```text
gcp-hcp-commons (WIF Pool: tfc-pool, Provider: tfc-oidc)
│
│   Cross-project workloadIdentityUser bindings:
│
├── SA: tfc-automation@gcp-hcp-{env}-global.iam.gserviceaccount.com
│   └── Used by: gcp-hcp-{env} project workspaces (integration, stage, production)
│       └── IAM roles granted on: gcp-hcp-{env}-global, {env}-reg-*, {env}-mgt-* projects
│       └── SA created manually and imported, or created by Atlantis if that's an option
│
└── SA: tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com
    ├── Used by: gcp-hcp-tooling project (pagerduty, etc.) — direct use
    └── Used by: gcp-hcp-ci project workspaces — impersonates per-CI-project SAs:
        ├── tfc-hypershift-ci@gcp-hcp-hypershift-ci.iam.gserviceaccount.com
        └── tfc-platform-ci@gcp-hcp-platform-ci.iam.gserviceaccount.com
```

**Decision:** Per-environment SAs in the environment's own global project. This isolates blast radius per environment — a compromised SA only has access to its own environment's projects. The WIF pool and OIDC provider remain centralized in commons since the OIDC configuration is identical across environments.

**CI two-hop impersonation:** CI workspaces authenticate via WIF as `tfc-automation@gcp-hcp-commons`, which then impersonates environment-specific CI SAs. This allows fine-grained permission scoping per CI project (hypershift-ci vs platform-ci) while sharing a single WIF entry point.

### WIF Variable Sets

TFC variable sets avoid duplicating WIF variables across every workspace in a project:

| Variable Set | Scope | Variables |
|---|---|---|
| `wif-integration` | All workspaces in `gcp-hcp-integration` project | `TFC_GCP_PROVIDER_AUTH`, `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` (tfc-automation@gcp-hcp-integration-global), `TFC_GCP_WORKLOAD_PROVIDER_NAME`, `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` |
| `wif-stage` | All workspaces in `gcp-hcp-stage` project | Same keys, tfc-automation@gcp-hcp-stage-global SA |
| `wif-production` | All workspaces in `gcp-hcp-production` project | Same keys, tfc-automation@gcp-hcp-production-global SA |
| `wif-tooling` | All workspaces in `gcp-hcp-tooling` project | Same keys, tfc-automation@gcp-hcp-commons SA |
| `wif-ci` | All workspaces in `gcp-hcp-ci` project | Same keys, tfc-automation@gcp-hcp-commons SA (base SA for impersonation) |

Individual workspaces don't set WIF variables — they inherit from the project-level variable set. CI workspaces additionally configure impersonation to their per-project SA via the Google provider's `impersonate_service_account` argument.

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
2. Create SA `tfc-automation@gcp-hcp-production-global.iam.gserviceaccount.com` in the environment's global project (manually or via Atlantis)
3. Grant cross-project `roles/iam.workloadIdentityUser` on the commons WIF pool (`tfc-pool`) to the new SA
4. Create variable set `wif-production` scoped to the new project, referencing the env SA
5. Add workspace entries for global, region, and MC configs

## Programmatic Workspace Management

### Code layout

```text
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

Each file uses the `hp-platform-engineering/workspaces/tfe` module alongside `tfe_variable_set` and `tfe_project_variable_set` resources to create project-level variable sets. Example for integration:

```hcl
locals {
  tfc_organization          = "hp-platform-engineering"
  tfc_service_account_email = "tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com"
  tfc_wif_provider_name     = "projects/573522191771/locations/global/workloadIdentityPools/tfc-pool/providers/tfc-oidc"
  tfc_wif_audience          = "https://app.terraform.io"

  tfc_wif_variables = [
    { key = "TFC_GCP_PROVIDER_AUTH",              value = "true",                         category = "env" },
    { key = "TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL",  value = local.tfc_service_account_email, category = "env" },
    { key = "TFC_GCP_WORKLOAD_PROVIDER_NAME",     value = local.tfc_wif_provider_name,     category = "env" },
    { key = "TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE", value = local.tfc_wif_audience,          category = "env" },
  ]
}

data "tfe_project" "this" {
  organization = local.tfc_organization
  name         = "gcp-hcp-integration"
}

resource "tfe_variable_set" "wif" {
  name              = "wif-integration"
  description       = "GCP Workload Identity Federation credentials"
  organization      = local.tfc_organization
  parent_project_id = data.tfe_project.this.id
}

resource "tfe_project_variable_set" "wif" {
  project_id      = data.tfe_project.this.id
  variable_set_id = tfe_variable_set.wif.id
}

resource "tfe_variable" "wif" {
  for_each        = { for v in local.tfc_wif_variables : v.key => v }
  variable_set_id = tfe_variable_set.wif.id
  key             = each.value.key
  value           = each.value.value
  category        = each.value.category
}

module "gcp-hcp-integration" {
  source            = "app.terraform.io/hp-platform-engineering/workspaces/tfe"
  organization      = local.tfc_organization
  project_name      = "gcp-hcp-integration"
  meta_project_name = "meta-gcp-hcp"
  notification_url  = var.notification_url

  workspaces = {
    gcp-hcp-global-integration = {
      terraform_version = "1.14.3"
      variables         = []
      working_directory = "terraform/config/global/integration/main/us-central1"
      github_repo_org   = "openshift-online"
      github_repo_name  = "gcp-hcp-infra"
    }
    gcp-hcp-region-integration-us-central1 = {
      terraform_version = "1.14.3"
      variables         = []
      working_directory = "terraform/config/region/integration/main/us-central1"
      github_repo_org   = "openshift-online"
      github_repo_name  = "gcp-hcp-infra"
    }
    gcp-hcp-mc-integration-main-us-central1-yjiv = {
      terraform_version = "1.14.3"
      variables         = []
      working_directory = "terraform/config/management-cluster/integration/main/us-central1-yjiv"
      github_repo_org   = "openshift-online"
      github_repo_name  = "gcp-hcp-infra"
    }
  }
}
```

> **Note:** This is the current "Option A" implementation using raw `tfe_*` resources for variable sets. Once the `workspaces/tfe` module is updated with native `variable_sets` support (PR pending in `infra-platform`), the `tfe_variable_set`, `tfe_variable`, and `tfe_project_variable_set` resources will be replaced by a `variable_sets` input on the module.

### Ephemeral platform-ci workspaces (gcp-hcp-ci)

Ephemeral workspaces (`gcp-hcp-platform-{sha}`) are created and destroyed per pipeline run, targeting `terraform/config/platform-ci/{ephemeral_folder}`. Options:

- **TFC Ephemeral Workspaces (evaluate)**: HashiCorp has released [Ephemeral Workspaces](https://www.hashicorp.com/en/blog/terraform-ephemeral-workspaces-public-beta-now-available) in public beta — purpose-built for short-lived infrastructure that is automatically destroyed after a configurable TTL. This may be the best fit for CI workspaces and should be evaluated first.
- **TFC API**: Tekton pipeline creates workspace via TFC API, runs plan/apply, destroys workspace on completion
- **tfe provider with dynamic workspaces**: A dedicated Terraform config creates/destroys workspaces based on input variables

These workspaces authenticate via the CI two-hop model: WIF as `tfc-automation@gcp-hcp-commons`, then impersonation to `tfc-platform-ci@gcp-hcp-platform-ci`. This is the most different from current Atlantis flow and needs further investigation.

## State Management

**Recommendation: Migrate state to TFC first, then cut over from Atlantis.**

Atlantis supports using [TFC/TFE as a remote backend](https://www.runatlantis.io/docs/terraform-cloud), which enables a phased migration strategy:

1. **Step 1 — Migrate state to TFC**: Move state from GCS (`gcp-hcp-{env}-global-terraform-state`) to TFC-managed state. Atlantis continues to run plan/apply but reads and writes state via TFC. This requires injecting TFC tokens into Atlantis.
2. **Step 2 — Cut over to TFC**: Disable Atlantis apply for a given path, enable TFC for that path. Atlantis and TFC never run concurrently on the same workspace since state is already in TFC.

This approach is safer than migrating state and tooling simultaneously — state migration is validated under Atlantis before TFC takes over execution. It also unlocks TFC-managed state features (versioning, locking, drift detection) before the full Atlantis cutover.

The `cloud {}` backend block and `backend "gcs" {}` are mutually exclusive. During Step 1, configs switch to `cloud {}` pointing at TFC workspaces while Atlantis still manages execution.

## What This Doesn't Cover Yet

- **RBAC model**: Who can approve applies per project/workspace (section 2 outstanding item)
- **OPA/Sentinel policies**: How to replicate or improve on the current `gcp-hcp-deletion-protection` policy
- **Pre-apply hooks**: Replacing `hack/check-pr-labels.sh` with TFC run tasks or policy checks
- **Drift detection**: TFC native capability, needs evaluation
- **Migration ordering**: Which workspaces to migrate first (likely tooling → integration → stage → production)
- **Atlantis TFC token injection**: How to provision and inject TFC API tokens into Atlantis for the state migration phase
- **Ephemeral workspace evaluation**: Assess TFC Ephemeral Workspaces (public beta) for CI use cases
