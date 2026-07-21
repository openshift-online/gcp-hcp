# HCP Terraform Must Authenticate via the infra-platform GCP Dynamic Credentials Module

***Scope***: GCP-HCP

**Date**: 2026-07-21

## Decision

HCP Terraform workspaces that manage GCP infrastructure must authenticate via Workload Identity Federation (WIF) using the `terraform-tfe-gcp-dynamic-creds` module from [infra-platform](https://github.com/openshift-online/infra-platform). Each environment gets its own module call targeting the environment's global GCP project, creating per-role-group WIF pools, plan/apply service accounts, and TFC variable sets. Cross-project IAM bindings for region and management-cluster projects are managed outside the module via supplementary `google_project_iam_member` resources in `gcp-hcp-infra`.

This decision supersedes the earlier proposal (July 2026) to use direct Workload Identity (`principal://` bindings without service accounts).

## Context

- **Problem Statement**: HCP Terraform workspaces need keyless authentication to GCP APIs across multiple GCP projects per environment (global, region, management-cluster). The initial WIF setup used a single pool in `gcp-hcp-commons` with a hand-managed service account and manual variable sets. The app-sre team has since published a reusable module ([infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90)) that automates WIF pool, service account, and variable set lifecycle — adopting it reduces maintenance burden and aligns with platform-wide tooling.
- **Constraints**:
  - The module creates service accounts (plan/apply pair per role group) — there is no direct WID mode. The team's earlier proposal to use `principal://` bindings without SAs is not compatible with this module.
  - The module only creates IAM bindings on a single GCP project per module call. GCP-HCP's multi-project environment model (global + N region + N management-cluster projects) requires supplementary cross-project bindings.
  - The module does not set `allowed_audiences` on the OIDC provider. When `allowed_audiences` is unset, GCP accepts the provider's resource name as the default audience, and HCP Terraform sends that resource name automatically. This is safe as long as `allowed_audiences` is never explicitly set on the GCP side.
  - CI workspaces extend existing Prow WIF pools with a TFC OIDC provider — they do not use this module.
  - PagerDuty workspaces do not need GCP IAM (PagerDuty API key only).
- **Assumptions**:
  - The `terraform-tfe-gcp-dynamic-creds` module will be published to the `hp-platform-engineering` TFC private registry and versioned
  - The module's SA-based model provides sufficient security isolation for production environments when combined with scoped `apply_roles`
  - GCP's 100 service accounts per project default quota is sufficient (each role group adds 2 SAs)

## Alternatives Considered

1. **Adopt infra-platform module with SA-based authentication**: Use the `terraform-tfe-gcp-dynamic-creds` module as-is. One module call per environment targeting the environment's global project. Plan/apply service accounts per role group. Cross-project bindings managed locally in `gcp-hcp-infra`.
2. **Direct Workload Identity (no SAs)**: Use `TFC_GCP_PRINCIPAL_TYPE = workload_pool` to authenticate as a federated principal directly. Grant IAM via `principal://` or `principalSet://` bindings. Eliminates service accounts entirely.
3. **Hand-managed WIF in commons**: Continue with the current single pool in `gcp-hcp-commons`, single `tfc-automation` SA, and manually created variable sets.
4. **Fork the module for direct WID support**: Fork `terraform-tfe-gcp-dynamic-creds`, add a `principal_type` toggle that skips SA creation and outputs `principalSet://` members instead. Maintain the fork in `gcp-hcp-infra`.

## Decision Rationale

* **Justification**: Alternative 1 (adopt the module) balances standardization with pragmatism. The module automates the error-prone parts of WIF setup (pool/provider creation, attribute conditions, variable set wiring, SA lifecycle) while leaving cross-project bindings — which are specific to GCP-HCP's multi-project model — to local management. Adopting a shared module also means benefiting from upstream fixes and features (e.g., the `apply_to_all_workspaces` flag added in response to our feedback).
* **Evidence**: The team validated WIF + Dynamic Provider Credentials end-to-end in a playground environment (PRs [#851](https://github.com/openshift-online/gcp-hcp-infra/pull/851), [#859](https://github.com/openshift-online/gcp-hcp-infra/pull/859), [#866](https://github.com/openshift-online/gcp-hcp-infra/pull/866)). The module was reviewed in [infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90) where the `apply_to_all_workspaces` feature was implemented based on our request. The module's approach (one pool per role group, workspace-scoped attribute conditions) provides tighter isolation than our current org-wide attribute condition.
* **Comparison**:
  - **Alternative 2 (direct WID)** eliminates SAs but requires validating `principal://` binding support across all GCP resource types we manage (GKE, Compute, DNS, Secret Manager, Workflows, Pub/Sub, Eventarc, Cloud Run, Tags, PAM). This validation was never completed. Additionally, no shared module supports this mode, so we'd be maintaining custom WIF infrastructure. The SA model is proven and well-understood.
  - **Alternative 3 (hand-managed)** continues the current pattern but doesn't scale — each new environment requires manual pool/SA/variable set creation and has no programmatic lifecycle management.
  - **Alternative 4 (fork)** gives the most control but creates maintenance burden. A fork diverges from upstream, and the direct WID validation gap remains.

## Consequences

### Positive

* Automated WIF lifecycle — pools, providers, SAs, and variable sets are created and maintained by the module
* Per-role-group WIF pools provide workspace-scoped isolation (vs. current org-wide attribute condition)
* `apply_to_all_workspaces` eliminates manual variable set attachment for ephemeral/CI-adjacent workspaces
* Aligns with platform-wide infra-platform tooling — benefits from upstream improvements
* Proven SA-based authentication model — no `principal://` compatibility gaps to validate

### Negative

* Reverses the team's July 17 decision to pursue direct WID — Phase 2 of the experiment doc is abandoned
* Cross-project IAM bindings must be managed outside the module — supplementary `google_project_iam_member` resources in each region and management-cluster module
* The module always creates separate plan/apply SAs even though the team prefers a single identity — mitigated by setting `plan_roles = apply_roles` for identical permissions
* `roles/owner` is the default `apply_roles` — production environments must override with scoped roles (see Implementation Reference)
* No `allowed_audiences` on the OIDC provider — relies on GCP default audience behavior. Safe with new pools but requires removing `allowed_audiences` from the existing commons pool if it's ever reused
* Audience configurability may need an upstream PR if the team later wants explicit audiences for defense-in-depth

## Cross-Cutting Concerns

### Security:

* Federated tokens are short-lived (~1h) and scoped to the specific HCP Terraform run phase (plan or apply)
* Per-role-group WIF pools scope `attribute_condition` to only that group's workspaces — a token from one role group cannot impersonate another group's SAs
* Production `apply_roles` must be scoped to specific roles (see Implementation Reference) — never use the `roles/owner` default in production
* Cross-project bindings are explicitly managed — no implicit permissions leak across environments

### Operability:

* **Adding a new environment**: One module call targeting the env's global project, plus cross-project bindings in region/MC modules. Variable sets auto-attach if `apply_to_all_workspaces = true`.
* **Adding a new workspace**: Inherits WIF variables from the project-level variable set — no per-workspace config needed.
* **Debugging authentication failures**: With default audience behavior, the token audience is the WIF provider resource name. If authentication fails, verify `TFC_GCP_WORKLOAD_PROVIDER_NAME` matches the actual provider resource name. The `attribute_condition` is logged in GCP audit logs.
* **Module versioning**: Pin the module version in each environment's config. Upgrades are explicit per-environment.

### Cost:

* WIF and STS token exchanges are free — no additional GCP charges
* Each role group adds 2 SAs to the GCP project (within the default 100/project quota)

## Implementation Reference

### Module Usage Pattern

Each environment calls the module once, targeting the environment's global GCP project:

```hcl
module "tfc_wif" {
  source  = "app.terraform.io/hp-platform-engineering/gcp-dynamic-creds/tfe"
  version = "x.y.z"

  organization     = "hp-platform-engineering"
  gcp_project_id   = "gcp-hcp-int-global"
  gcp_project_name = "gcp-hcp-integration"

  role_groups = {
    default = {
      plan_roles  = local.tfc_plan_roles
      apply_roles = local.tfc_apply_roles
      projects = {
        gcp-hcp-integration = {
          apply_to_all_workspaces = true
        }
      }
    }
  }
}
```

### Role Group Configuration

The module defaults to `roles/viewer` (plan) and `roles/owner` (apply). For production, override with scoped roles matching the Atlantis permission model:

```hcl
locals {
  tfc_plan_roles = ["roles/viewer"]
  tfc_apply_roles = [
    "roles/container.admin",
    "roles/compute.networkAdmin",
    "roles/compute.instanceAdmin.v1",
    "roles/iam.serviceAccountAdmin",
    "roles/iam.serviceAccountUser",
    "roles/resourcemanager.projectIamAdmin",
    "roles/dns.admin",
    "roles/secretmanager.admin",
    "roles/serviceusage.serviceUsageAdmin",
    "roles/workflows.admin",
    "roles/run.admin",
    "roles/pubsub.admin",
    "roles/eventarc.admin",
    "roles/resourcemanager.tagAdmin",
    "roles/resourcemanager.tagUser",
    "roles/privilegedaccessmanager.admin",
    "roles/monitoring.metricsScopesAdmin",
  ]
}
```

Since the team prefers a single identity for plan and apply, set `plan_roles = apply_roles` to give both SAs identical permissions.

### Cross-Project IAM Bindings

The module only binds roles on `gcp_project_id` (the global project). Region and management-cluster projects need supplementary bindings. These are managed in `gcp-hcp-infra` via the existing Atlantis IAM pattern in each module's `atlantis.tf`, adapted for TFC:

```hcl
# In terraform/modules/region/tfc.tf (new file)
resource "google_project_iam_member" "tfc_apply" {
  for_each = toset(var.tfc_apply_roles)
  project  = module.project.project_id
  role     = each.value
  member   = "serviceAccount:${var.tfc_apply_sa_email}"
}

resource "google_project_iam_member" "tfc_plan" {
  for_each = toset(var.tfc_plan_roles)
  project  = module.project.project_id
  role     = each.value
  member   = "serviceAccount:${var.tfc_plan_sa_email}"
}
```

The SA emails come from the module's outputs (`apply_service_account_emails`, `plan_service_account_emails`) and are passed as variables to each region/MC config.

### Per-Environment WIF Topology

| Environment | GCP Project (pool location) | TFC Project | Role Group | `apply_to_all_workspaces` |
|---|---|---|---|---|
| Integration | `gcp-hcp-int-global` | `gcp-hcp-integration` | `default` | `true` |
| Stage | `gcp-hcp-stg-global` | `gcp-hcp-stage` | `default` | `true` |
| Production | `gcp-hcp-prd-global` | `gcp-hcp-production` | `default` | `true` |
| Tooling | `gcp-hcp-commons` | `gcp-hcp-tooling` | `default` | `true` |
| CI | *(extends Prow pools)* | `gcp-hcp-ci` | *(not using module)* | N/A |

### CI Authentication (Outside Module)

CI workspaces (`gcp-hcp-ci` TFC project) do not use this module. Instead:

1. Existing Prow WIF pools in CI projects (`gcp-hcp-hypershift-ci`, `gcp-hcp-platform-ci`) are extended with a TFC OIDC provider
2. The OIDC provider's `attribute_condition` scopes to the `gcp-hcp-ci` TFC project
3. CI workspaces may need workspace-specific env vars (not all via variable sets) since each targets a different IAM case

### Authentication Flow

```text
HCP Terraform Run (e.g., gcp-hcp-region-integration-us-central1)
    |
    +-- 1. TFC generates OIDC token with:
    |       issuer:   https://app.terraform.io
    |       audience: (WIF provider resource name, auto-matched)
    |       subject:  organization:hp-platform-engineering:project:gcp-hcp-integration:workspace:...:run_phase:apply
    |
    +-- 2. Token sent to GCP STS -> validated against WIF provider in gcp-hcp-int-global
    |       - issuer matches
    |       - audience matches (default = provider resource name)
    |       - attribute_condition: sub starts with "...project:gcp-hcp-integration:"
    |
    +-- 3. STS returns federated token -> exchanged for apply SA access token
    |       SA: hcp-tf-default-apply@gcp-hcp-int-global.iam.gserviceaccount.com
    |
    +-- 4. SA token used for GCP API calls
            - Global project: IAM granted by module
            - Region/MC projects: IAM granted by supplementary bindings in gcp-hcp-infra
```

### Variable Sets (Module-Managed)

The module creates one variable set per (role group, TFC project) pair:

| Variable | Value | Purpose |
|----------|-------|---------|
| `TFC_GCP_PROVIDER_AUTH` | `true` | Enables GCP provider authentication via WIF |
| `TFC_GCP_WORKLOAD_PROVIDER_NAME` | `projects/{number}/locations/global/workloadIdentityPools/hcp-tf-default/providers/oidc` | Full resource name of the WIF OIDC provider |
| `TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL` | `hcp-tf-default-plan@gcp-hcp-{env}-global.iam.gserviceaccount.com` | Plan-phase SA |
| `TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL` | `hcp-tf-default-apply@gcp-hcp-{env}-global.iam.gserviceaccount.com` | Apply-phase SA |

The module does **not** set `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` — it relies on default audience matching. It also does **not** set `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` — it uses the separate plan/apply SA variables instead.

### Migration from Commons Pool

The existing WIF infrastructure in `gcp-hcp-commons` (`tfc-pool`, `tfc-oidc`, `tfc-automation` SA) will be decommissioned after all environments are migrated to module-managed pools:

1. Deploy module-managed pools per environment (new pools in each env global project)
2. Migrate workspaces environment-by-environment (update variable sets to point at new pools)
3. Remove commons WIF resources from `terraform/modules/commons/tfc.tf`

### Open Items

- **Upstream PR for cross-project bindings**: Consider contributing an `additional_project_bindings` input to the infra-platform module to reduce local workaround maintenance
- **Upstream PR for audience configurability**: Consider contributing an optional `allowed_audiences` input for defense-in-depth if the team wants explicit audience matching
- **Tooling project**: Confirm whether `gcp-hcp-tooling` workspaces (PagerDuty) need GCP WIF at all — PagerDuty uses only a PagerDuty API key

## Related Decisions

- [Terraform Automation Tooling](terraform-automation-tooling.md) — original Atlantis selection decision
- [HCP Terraform Workspace Architecture](hcp-terraform-workspace-architecture.md) — TFC project/workspace hierarchy
- [Prow CI Workload Identity Federation](../identity/prow-ci-workload-identity-federation.md) — CI WIF model (extended for TFC)
