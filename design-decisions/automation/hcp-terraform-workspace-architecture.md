# HCP Terraform Must Use Per-Environment Workspace Hierarchy with Environment-Scoped Service Accounts

***Scope***: GCP-HCP

**Date**: 2026-07-15

## Decision

HCP Terraform workspaces must be organized into per-environment TFC projects (`gcp-hcp-{env}`) with environment-scoped service accounts (`tfc-automation@gcp-hcp-{env}-global`), a shared tooling project, and a CI project using two-hop impersonation. WIF variables must be delivered via project-level variable sets, not per-workspace locals.

## Context

- **Problem Statement**: Migrating from Atlantis to HCP Terraform requires mapping the existing infrastructure workspace model — per-environment Atlantis deployments, GCP project hierarchy, and service account structure — into TFC's organizational model (organizations, projects, workspaces). The mapping must preserve environment isolation, support programmatic workspace creation, and enable a phased migration path.
- **Constraints**:
  - The existing GCP project hierarchy (`gcp-hcp-{env}-global`, `{env}-reg-*`, `{env}-mgt-*`) and Atlantis project structure (`atlantis-integration.yaml`) define the 1:1 workspace mapping
  - Service accounts in environment global projects are managed by Atlantis — TFC cannot create them (chicken-and-egg problem). They must be created manually and imported, or created by Atlantis before the cutover
  - The WIF pool and OIDC provider are centralized in `gcp-hcp-commons` (see [HCP Terraform WIF design decision](hcp-terraform-workload-identity-federation.md))
  - CI workspaces require both static (hypershift-ci, platform-ci) and ephemeral (per-pipeline-run) workspace types
  - `scripts/infra.py` handles template-based config creation and must be extended for TFC workspace provisioning
- **Assumptions**:
  - All environments (integration, stage, production) follow the same workspace hierarchy pattern
  - The `hp-platform-engineering/workspaces/tfe` module will be the standard for programmatic workspace creation
  - PagerDuty is the lowest-risk workspace for initial migration, followed by global modules

## Alternatives Considered

1. **Per-environment TFC projects with environment-scoped SAs**: One TFC project per environment, each with its own SA in the environment's global project. Tooling and CI share a commons SA, with CI using impersonation for per-project SAs. WIF variables delivered via project-level variable sets.
2. **Single TFC project with shared SA**: All workspaces in a single TFC project, using one SA in commons with WIF `attribute_condition` to restrict access per workspace. Simpler setup but no project-level isolation.
3. **Per-environment TFC projects with commons SAs**: One TFC project per environment, but all SAs remain in `gcp-hcp-commons` (e.g., `tfc-int@gcp-hcp-commons`). Cross-environment isolation depends on IAM bindings rather than SA placement.
4. **Per-workspace variables (locals)**: Each workspace defines its own WIF variables via `local.tfc_wif_variables`. No variable set inheritance.

## Decision Rationale

* **Justification**: Alternative 1 provides the strongest isolation model. Per-environment TFC projects mirror the existing GCP project hierarchy, and placing SAs in their respective global projects ensures a compromised SA only has access to its own environment. Project-level variable sets eliminate per-workspace variable duplication and prevent drift.
* **Evidence**: The existing Atlantis model uses per-environment deployments on separate GKE clusters for credential isolation. This maps directly to per-environment TFC projects. The playground validation (see [TFC WIF playground experiment](../../experiments/terraform-automation-tools/tfc-wif-playground.md)) confirmed that variable sets work correctly for WIF variable inheritance.
* **Comparison**:
  - **Alternative 2 (single project)** provides no project-level RBAC boundary — all team members with access to the project can see all environments. TFC's permission model is project-scoped, making this a significant security gap.
  - **Alternative 3 (commons SAs)** keeps all SAs in one project, increasing blast radius if commons is compromised. It also doesn't align with the existing pattern where integration SAs live in integration projects.
  - **Alternative 4 (per-workspace locals)** causes variable duplication across every workspace and risks silent WIF misconfiguration if a variable is omitted or mistyped.

## Consequences

### Positive

* TFC project hierarchy mirrors the GCP project hierarchy, making the mapping intuitive for operators
* Environment-scoped SAs isolate blast radius per environment
* Project-level variable sets eliminate WIF variable duplication and prevent per-workspace drift
* CI two-hop impersonation allows fine-grained permission scoping per CI project while sharing a single WIF entry point
* Atlantis-to-TFC workspace mapping is 1:1, simplifying migration planning

### Negative

* Service accounts in environment global projects require manual creation or import — they cannot be bootstrapped by TFC
* Two-hop impersonation for CI adds operational complexity (two IAM bindings to maintain per CI project)
* Ephemeral CI workspace lifecycle management differs significantly from current Atlantis flow and needs further investigation
* Migration requires a phased approach (state migration first, then tooling cutover) to avoid concurrent Atlantis/TFC conflicts

## Cross-Cutting Concerns

### Security:

* Per-environment SAs in per-env global projects limit blast radius — a compromised SA cannot access other environments
* CI two-hop impersonation ensures the commons SA has no direct permissions on CI resources; only the impersonated per-project SA does
* WIF variables are not duplicated per workspace, reducing the risk of misconfigured credentials

### Operability:

* **Adding a new region**: `scripts/infra.py` generates config and adds a workspace entry. WIF variables are inherited from the project variable set — no per-workspace config needed
* **Adding a new environment**: Create TFC project, create SA in env global project, grant cross-project `workloadIdentityUser` on commons WIF pool, create variable set, add workspace entries
* **Migration ordering**: PagerDuty (lowest risk) → global modules → integration → stage → production

### Cost:

* No additional GCP costs — WIF token exchanges are free
* TFC workspace costs scale with the number of workspaces, governed by the organization's TFC plan

## Implementation Reference

### TFC Hierarchy

```text
ORG: hp-platform-engineering
│
├── Project: gcp-hcp-{env}                          (one per environment: integration, stage, production)
│   ├── Workspace: gcp-hcp-global-{env}             → terraform/config/global/{env}/main/{region}
│   ├── Workspace: gcp-hcp-region-{env}-{sector}-{region}
│   │                                                → terraform/config/region/{env}/{sector}/{region}
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

### Atlantis → TFC Workspace Mapping

Current Atlantis projects (from `atlantis-integration.yaml`) map 1:1 to TFC workspaces:

| Atlantis Project | TFC Workspace | TFC Project | Working Directory |
|------------------|---------------|-------------|-------------------|
| `global-int-main-us-central1` | `gcp-hcp-global-integration` | `gcp-hcp-integration` | `terraform/config/global/integration/main/us-central1` |
| `region-int-main-us-central1` | `gcp-hcp-region-integration-main-us-central1` | `gcp-hcp-integration` | `terraform/config/region/integration/main/us-central1` |
| `mc-int-main-us-central1-yjiv` | `gcp-hcp-mc-integration-main-us-central1-yjiv` | `gcp-hcp-integration` | `terraform/config/management-cluster/integration/main/us-central1-yjiv` |
| `pagerduty` | `gcp-hcp-pagerduty` | `gcp-hcp-tooling` | `terraform/config/pagerduty` |
| `hypershift-ci` | `gcp-hcp-hypershift-ci` | `gcp-hcp-ci` | `terraform/config/hypershift-ci` |
| *(new)* | `gcp-hcp-platform-ci` | `gcp-hcp-ci` | `terraform/config/platform-ci` |

Stage workspaces follow the same pattern (already have configs under `terraform/config/*/stage/`).

### WIF Service Account Architecture

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

### WIF Variable Sets

| Variable Set | Scope | Variables |
|---|---|---|
| `wif-integration` | All workspaces in `gcp-hcp-integration` project | `TFC_GCP_PROVIDER_AUTH`, `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` (tfc-automation@gcp-hcp-integration-global), `TFC_GCP_WORKLOAD_PROVIDER_NAME`, `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` |
| `wif-stage` | All workspaces in `gcp-hcp-stage` project | Same keys, tfc-automation@gcp-hcp-stage-global SA |
| `wif-production` | All workspaces in `gcp-hcp-production` project | Same keys, tfc-automation@gcp-hcp-production-global SA |
| `wif-tooling` | All workspaces in `gcp-hcp-tooling` project | Same keys, tfc-automation@gcp-hcp-commons SA |
| `wif-ci` | All workspaces in `gcp-hcp-ci` project | Same keys, tfc-automation@gcp-hcp-commons SA (base SA for impersonation) |

Individual workspaces don't set WIF variables — they inherit from the project-level variable set. CI workspaces additionally configure impersonation to their per-project SA via the Google provider's `impersonate_service_account` argument.

### Code Layout

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
    gcp-hcp-region-integration-main-us-central1 = {
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

### Ephemeral Platform-CI Workspaces

Ephemeral workspaces (`gcp-hcp-platform-{sha}`) are created and destroyed per pipeline run, targeting `terraform/config/platform-ci/{ephemeral_folder}`. Options:

- **TFC Ephemeral Workspaces (evaluate)**: HashiCorp has released [Ephemeral Workspaces](https://www.hashicorp.com/en/blog/terraform-ephemeral-workspaces-public-beta-now-available) in public beta — purpose-built for short-lived infrastructure that is automatically destroyed after a configurable TTL. This may be the best fit for CI workspaces and should be evaluated first.
- **TFC API**: Tekton pipeline creates workspace via TFC API, runs plan/apply, destroys workspace on completion
- **tfe provider with dynamic workspaces**: A dedicated Terraform config creates/destroys workspaces based on input variables

These workspaces authenticate via the CI two-hop model: WIF as `tfc-automation@gcp-hcp-commons`, then impersonation to `tfc-platform-ci@gcp-hcp-platform-ci`. This is the most different from current Atlantis flow and needs further investigation.

### State Management

**Recommendation: Migrate state to TFC first, then cut over from Atlantis.**

Atlantis supports using [TFC/TFE as a remote backend](https://www.runatlantis.io/docs/terraform-cloud), which enables a phased migration strategy:

1. **Step 1 — Migrate state to TFC**: Move state from GCS (`gcp-hcp-{env}-global-terraform-state`) to TFC-managed state. Atlantis continues to run plan/apply but reads and writes state via TFC. This requires injecting TFC tokens into Atlantis.
2. **Step 2 — Cut over to TFC**: Disable Atlantis apply for a given path, enable TFC for that path. Atlantis and TFC never run concurrently on the same workspace since state is already in TFC.

This approach is safer than migrating state and tooling simultaneously — state migration is validated under Atlantis before TFC takes over execution. It also unlocks TFC-managed state features (versioning, locking, drift detection) before the full Atlantis cutover.

The `cloud {}` backend block and `backend "gcs" {}` are mutually exclusive. During Step 1, configs switch to `cloud {}` pointing at TFC workspaces while Atlantis still manages execution.

## Open Items

- **RBAC model**: Who can approve applies per project/workspace (section 2 outstanding item)
- **OPA/Sentinel policies**: How to replicate or improve on the current `gcp-hcp-deletion-protection` policy
- **Pre-apply hooks**: Replacing `hack/check-pr-labels.sh` with TFC run tasks or policy checks
- **Drift detection**: TFC native capability, needs evaluation
- **Atlantis TFC token injection**: How to provision and inject TFC API tokens into Atlantis for the state migration phase
- **Ephemeral workspace evaluation**: Assess TFC Ephemeral Workspaces (public beta) for CI use cases
