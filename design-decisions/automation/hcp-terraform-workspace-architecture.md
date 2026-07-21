# HCP Terraform Must Use Per-Environment Workspace Hierarchy with Module-Managed WIF

***Scope***: GCP-HCP

**Date**: 2026-07-21

## Decision

HCP Terraform workspaces must be organized into per-environment TFC projects (`gcp-hcp-{env}`) with WIF credentials managed by the `terraform-tfe-gcp-dynamic-creds` module from infra-platform. Each environment's module call creates a role group with plan/apply service accounts and a project-level variable set with `apply_to_all_workspaces = true`, so all workspaces in the project automatically inherit WIF credentials. Cross-project IAM bindings are managed locally in `gcp-hcp-infra`.

## Context

- **Problem Statement**: Migrating from Atlantis to HCP Terraform requires mapping the existing infrastructure workspace model — per-environment Atlantis deployments, GCP project hierarchy, and service account structure — into TFC's organizational model (organizations, projects, workspaces). The mapping must preserve environment isolation, support programmatic workspace creation, and enable a phased migration path.
- **Constraints**:
  - The existing GCP project hierarchy (`gcp-hcp-{env}-global`, `{env}-reg-*`, `{env}-mgt-*`) and Atlantis project structure (`atlantis-integration.yaml`) define the 1:1 workspace mapping
  - WIF infrastructure is managed by the `terraform-tfe-gcp-dynamic-creds` module (see [HCP Terraform WIF design decision](hcp-terraform-workload-identity-federation.md)), which creates module-managed SAs and variable sets
  - CI workspaces extend existing Prow WIF pools — they do not use the module
  - `scripts/infra.py` handles template-based config creation and must be extended for TFC workspace provisioning
- **Assumptions**:
  - All environments (integration, stage, production) follow the same workspace hierarchy pattern
  - The `hp-platform-engineering/workspaces/tfe` module will be the standard for programmatic workspace creation
  - PagerDuty is the lowest-risk workspace for initial migration, followed by global modules

## Alternatives Considered

1. **Per-environment TFC projects with module-managed WIF**: One TFC project per environment, each with WIF managed by the infra-platform module. Variable sets auto-attached to all workspaces via `apply_to_all_workspaces`. Cross-project bindings managed locally.
2. **Single TFC project with shared SA**: All workspaces in a single TFC project, using one SA with org-wide WIF `attribute_condition`. Simpler setup but no project-level isolation.
3. **Per-environment TFC projects with hand-managed WIF**: One TFC project per environment, with manually created variable sets and SAs. The pre-module approach.
4. **Per-workspace variable sets**: Each workspace defines its own WIF variables via `local.tfc_wif_variables`. No variable set inheritance.

## Decision Rationale

* **Justification**: Alternative 1 combines environment isolation (per-project TFC boundaries) with automated WIF lifecycle management (module-managed pools, SAs, variable sets). The `apply_to_all_workspaces` flag ensures new workspaces — including ephemeral CI-adjacent workspaces — automatically get WIF credentials without manual attachment.
* **Evidence**: The playground validation ([WIF playground experiment](../../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md)) confirmed that project-level variable sets work correctly for WIF variable inheritance. The `apply_to_all_workspaces` feature was implemented in [infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90) based on our feedback.
* **Comparison**:
  - **Alternative 2 (single project)** provides no project-level RBAC boundary — all team members see all environments.
  - **Alternative 3 (hand-managed)** works but doesn't scale and is error-prone for SA/variable set lifecycle.
  - **Alternative 4 (per-workspace)** causes variable duplication and risks silent WIF misconfiguration.

## Consequences

### Positive

* TFC project hierarchy mirrors the GCP project hierarchy
* Module-managed variable sets with `apply_to_all_workspaces` eliminate per-workspace WIF configuration
* New workspaces automatically inherit WIF credentials
* Atlantis-to-TFC workspace mapping is 1:1, simplifying migration planning
* Module versioning provides controlled, per-environment upgrades

### Negative

* Cross-project IAM bindings must be managed outside the module — additional Terraform resources in `gcp-hcp-infra`
* Module creates separate plan/apply SAs even though the team prefers unified identity — mitigated by `plan_roles = apply_roles`
* Migration requires a phased approach to avoid concurrent Atlantis/TFC conflicts

## Cross-Cutting Concerns

### Security:

* Per-environment WIF pools in per-env global projects limit blast radius
* Workspace-scoped `attribute_condition` (via the module) replaces the current org-wide condition
* Cross-project bindings are explicitly managed — no implicit permissions

### Operability:

* **Adding a new region**: `scripts/infra.py` generates config and adds a workspace entry. WIF variables are inherited from the project variable set — no per-workspace config needed. Cross-project bindings are part of the module template.
* **Adding a new environment**: Call the WIF module targeting the env's global project, create TFC project, add workspace entries. Variable sets auto-attach.
* **Migration ordering**: PagerDuty (lowest risk) -> global modules -> integration -> stage -> production

### Cost:

* No additional GCP costs — WIF token exchanges are free
* TFC workspace costs scale with the number of workspaces

## Implementation Reference

### TFC Hierarchy

```text
ORG: hp-platform-engineering
|
+-- Project: gcp-hcp-{env}                          (one per environment: integration, stage, production)
|   +-- Variable Set: gcp-hcp-{env}-default-gcp-dynamic-creds  (auto-attached, module-managed)
|   +-- Workspace: gcp-hcp-global-{env}             -> terraform/config/global/{env}/main/{region}
|   +-- Workspace: gcp-hcp-region-{env}-{sector}-{region}
|   |                                                -> terraform/config/region/{env}/{sector}/{region}
|   +-- Workspace(s): gcp-hcp-mc-{env}-{sector}-{region}-{infra_id}
|                                                    -> terraform/config/management-cluster/{env}/{sector}/{region}-{infra_id}
|
+-- Project: gcp-hcp-tooling
|   +-- Workspace: gcp-hcp-pagerduty                -> terraform/config/pagerduty
|
+-- Project: gcp-hcp-ci
    +-- Workspace: gcp-hcp-hypershift-ci             -> terraform/config/hypershift-ci
    +-- Workspace: gcp-hcp-platform-ci               -> terraform/config/platform-ci
    +-- Workspace (ephemeral): gcp-hcp-platform-{sha}
                                                      -> terraform/config/platform-ci/{ephemeral_folder}
```

### Variable Set Naming

The module generates variable set names as `{tfc_project}-{gcp_project_name}-{role_group}-gcp-dynamic-creds`. For a `default` role group in the integration environment:

`gcp-hcp-integration-gcp-hcp-integration-default-gcp-dynamic-creds`

This name is auto-generated by the module — callers don't choose it. The `variable_set_name_suffix` can be customized if the default is too long.

### Atlantis -> TFC Workspace Mapping

Current Atlantis projects (from `atlantis-integration.yaml`) map 1:1 to TFC workspaces:

| Atlantis Project | TFC Workspace | TFC Project | Working Directory |
|---|---|---|---|
| `global-int-main-us-central1` | `gcp-hcp-global-integration` | `gcp-hcp-integration` | `terraform/config/global/integration/main/us-central1` |
| `region-int-main-us-central1` | `gcp-hcp-region-integration-main-us-central1` | `gcp-hcp-integration` | `terraform/config/region/integration/main/us-central1` |
| `mc-int-main-us-central1-yjiv` | `gcp-hcp-mc-integration-main-us-central1-yjiv` | `gcp-hcp-integration` | `terraform/config/management-cluster/integration/main/us-central1-yjiv` |
| `pagerduty` | `gcp-hcp-pagerduty` | `gcp-hcp-tooling` | `terraform/config/pagerduty` |
| `hypershift-ci` | `gcp-hcp-hypershift-ci` | `gcp-hcp-ci` | `terraform/config/hypershift-ci` |
| *(new)* | `gcp-hcp-platform-ci` | `gcp-hcp-ci` | `terraform/config/platform-ci` |

### WIF Module Calls per Environment

```text
hcp-terraform/
+-- gcp-hcp-integration/
|   +-- wif.tf             # module "tfc_wif" targeting gcp-hcp-int-global
|   +-- workspaces.tf      # module "workspaces" using hp-platform-engineering/workspaces/tfe
+-- gcp-hcp-stage/
|   +-- wif.tf             # module "tfc_wif" targeting gcp-hcp-stg-global
|   +-- workspaces.tf
+-- gcp-hcp-production/
|   +-- wif.tf             # module "tfc_wif" targeting gcp-hcp-prd-global
|   +-- workspaces.tf
+-- gcp-hcp-tooling/
|   +-- wif.tf             # module "tfc_wif" targeting gcp-hcp-commons (if needed)
|   +-- workspaces.tf
+-- gcp-hcp-ci/
    +-- wif.tf             # Manual OIDC provider on existing Prow pools (no module)
    +-- workspaces.tf
```

### State Management

**Recommendation: Migrate state to TFC first, then cut over from Atlantis.**

1. **Step 1 — Migrate state to TFC**: Move state from GCS to TFC-managed state. Atlantis continues to run plan/apply but reads/writes state via TFC.
2. **Step 2 — Cut over to TFC**: Disable Atlantis, enable TFC for execution. State is already in TFC from step 1.

### Ephemeral Platform-CI Workspaces

Ephemeral workspaces (`gcp-hcp-platform-{sha}`) are created and destroyed per pipeline run. Options to evaluate:

- **TFC Ephemeral Workspaces (public beta)**: Purpose-built for short-lived infrastructure with auto-destroy TTL. Best fit for CI workspaces.
- **TFC API**: Tekton pipeline creates/destroys workspaces programmatically.

These workspaces authenticate via CI-specific WIF (extended Prow pools), not the module.

## Open Items

- **RBAC model**: Who can approve applies per project/workspace
- **OPA/Sentinel policies**: How to replicate the current `gcp-hcp-deletion-protection` policy
- **Pre-apply hooks**: Replacing `hack/check-pr-labels.sh` with TFC run tasks or policy checks
- **Drift detection**: TFC native capability, needs evaluation
- **Ephemeral workspace evaluation**: Assess TFC Ephemeral Workspaces for CI use cases
- **Variable set naming**: Evaluate whether the auto-generated names are acceptable or need `variable_set_name_suffix` customization

## Related Decisions

- [HCP Terraform WIF](hcp-terraform-workload-identity-federation.md) — WIF authentication model using infra-platform module
- [Terraform Automation Tooling](terraform-automation-tooling.md) — original Atlantis selection decision
