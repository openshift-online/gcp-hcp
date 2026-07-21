# terraform-tfe-gcp-dynamic-creds

Sets up GCP dynamic provider credentials (Workload Identity Federation) for HCP Terraform. Creates one Workload Identity Pool per named "role group" -- each with its own plan/apply service account pair and its own project-owned variable sets -- so different workspaces can use different service accounts and roles within the same GCP project. Workspaces opt into dynamic credentials via `variable_set_names` by default, or a project's variable set can be auto-attached to every workspace in it via `apply_to_all_workspaces`.

Each role group gets its own Workload Identity Pool, letting that pool's provider scope its `attribute_condition` to only that group's workspaces -- so a token issued for one role group can't be used to impersonate another role group's service accounts.

This module has no `provider` blocks of its own -- it inherits the default `google` and `tfe` provider configurations from whichever root module calls it, so the caller must configure both (project, region, `tfe` organization, etc.) itself.

## Usage

```hcl
provider "google" {
  project = "my-gcp-project-123"
  region  = "us-central1"
}

provider "tfe" {
  organization = "hp-platform-engineering"
}

module "gcp_prod_dynamic_creds" {
  source  = "app.terraform.io/hp-platform-engineering/gcp-dynamic-creds/tfe"
  version = "x.y.z"

  organization     = "hp-platform-engineering"
  gcp_project_id   = "my-gcp-project-123"
  gcp_project_name = "gcp-prod"

  role_groups = {
    # plan_roles / apply_roles default to roles/viewer and roles/owner if omitted.
    default = {
      projects = {
        gcp-resources = {}
      }
    }
    restricted = {
      apply_roles = ["roles/storage.admin"]
      projects = {
        gcp-resources = {
          workspace_names = ["app-sre-gcp-storage"]
        }
      }
    }
  }
}
```

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.13.4 |
| hashicorp/google | >= 6.0.0, < 8.0.0 |
| hashicorp/tfe | ~> 0.76 |

The `google` constraint spans both major versions rather than pinning to `~> 6.0`: v7 has no breaking changes to any resource this module uses (the only v7 change worth noting, `google_project_service.disable_on_destroy` losing its default, doesn't affect this module since it's set explicitly), and pinning to v6 would force callers already on the provider's current major version -- e.g. `app-sre`'s `gcp-resources` workspace, on 7.39.0 -- to coexist with a stale ceiling.

## Inputs

| Name | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `organization` | `string` | — | yes | HCP Terraform organization name |
| `gcp_project_id` | `string` | — | yes | GCP project ID to target. All resources are created in this project |
| `gcp_project_name` | `string` | — | yes | Short display name for the GCP project (e.g. `"gcp-prod"`). Used to derive variable set names |
| `role_groups` | `map(object)` | — | yes | Named GCP identity groups (see object schema below) |
| `service_account_id_prefix` | `string` | `"hcp-tf"` | no | Prefix for generated service account IDs |
| `pool_id_prefix` | `string` | `"hcp-tf"` | no | Prefix for generated Workload Identity Pool/Provider IDs |
| `pool_provider_id` | `string` | `"oidc"` | no | Workload Identity Pool Provider ID, created within each role group's pool |
| `variable_set_name_suffix` | `string` | `"gcp-dynamic-creds"` | no | Suffix for generated variable set names |
| `project_services` | `set(string)` | 4 Workload Identity Federation APIs | no | GCP APIs to enable on `gcp_project_id` |

### `role_groups` object

```hcl
map(object({
  plan_roles  = optional(list(string), ["roles/viewer"])
  apply_roles = optional(list(string), ["roles/owner"])
  projects = map(object({
    workspace_names         = optional(set(string), ["*"])
    apply_to_all_workspaces = optional(bool, false)
  }))
}))
```

The map key names a role group (e.g. `"default"`, `"restricted"`). Each group gets its own Workload Identity Pool + Provider, named `{pool_id_prefix}-{role_group}`, and its own plan/apply service account pair, named `{service_account_id_prefix}-{role_group}-plan`/`-apply`. GCP limits pool/provider IDs to 32 characters and service account IDs to 30 characters, so with the default 6-character prefixes, role group names can be up to 25 (pool) or 17 (service account, the tighter constraint due to the `-plan`/`-apply` suffix) characters before hitting those limits -- enforced at plan time by `validation` blocks computed from whatever `service_account_id_prefix`/`pool_id_prefix` is actually passed in, not just the defaults. A `validation` block also rejects a role group with an empty `projects` map, since that would produce an empty (invalid) `attribute_condition`.

`plan_roles` defaults to `roles/viewer` and `apply_roles` defaults to `roles/owner`, which gives a role group full control over the project, including the IAM permissions needed to manage/re-apply this same module using its own dynamic credentials once bootstrapped.

`*_roles` are lists (not sets) so bindings can be keyed by position -- a role that's only known after apply (e.g. a custom role created alongside this module) doesn't break `for_each`. Reordering an existing list (with no roles added or removed) still replaces every binding at a shifted position, briefly detaching and reattaching those roles -- append/remove at the end instead of reordering where possible.

Each group's `projects` map key is an HCP Terraform project name, and one project-owned variable set is created per entry, named `{project}-{gcp_project_name}-{role_group}-{variable_set_name_suffix}`. `workspace_names` scopes which workspace names within that project may impersonate that group's service accounts, defaulting to `["*"]` (all workspaces). This scoping is enforced by each role group's Workload Identity Pool Provider `attribute_condition`, which only accepts OIDC tokens whose `sub` claim matches one of that group's `organization:project:workspace` patterns. GCP caps `attribute_condition` at 4096 characters -- since each role group's condition only covers that group's own projects/workspaces (not the union across every role group), this stays comfortably bounded for typical usage, but a role group listing many projects each with many explicit `workspace_names` could approach it.

`apply_to_all_workspaces` (default `false`) auto-attaches that project's variable set to every current and future workspace in it, instead of requiring each workspace to opt in via `variable_set_names` -- useful for setups with ephemeral/CI workspaces created and destroyed per pipeline run, where attaching variable sets per workspace doesn't scale. It's independent of `workspace_names`: `apply_to_all_workspaces` decides who *gets* the `TFC_GCP_*` env vars, `workspace_names` decides who can actually *authenticate* with them. Setting `apply_to_all_workspaces = true` alongside a `workspace_names` narrower than the default `["*"]` means every workspace in the project receives the credentials, but only the listed workspaces can complete authentication -- pair `apply_to_all_workspaces = true` with the default `workspace_names` unless that split is intentional.

The same HCP Terraform project can appear under multiple role groups -- each gets its own variable set, so different workspaces within the same project can opt into different service accounts/roles by attaching different variable sets. All role groups target the same GCP project (one GCP project is targeted per module call; a tenant needing dynamic creds for multiple GCP projects calls this module once per project).

## Resources

| Resource | Description |
|----------|--------------|
| `google_project_service.this` | Enables the 4 GCP APIs required for Workload Identity Federation |
| `google_iam_workload_identity_pool.this` | Workload Identity Pool, one per role group |
| `google_iam_workload_identity_pool_provider.this` | OIDC provider for app.terraform.io within each pool, scoped via `attribute_condition` to that role group's workspaces |
| `google_service_account.plan` | Plan-phase service account, one per role group |
| `google_service_account.apply` | Apply-phase service account, one per role group |
| `google_service_account_iam_member.plan` | Grants `roles/iam.workloadIdentityUser` on the plan SA, scoped to plan-phase tokens from that role group's pool |
| `google_service_account_iam_member.apply` | Grants `roles/iam.workloadIdentityUser` on the apply SA, scoped to apply-phase tokens from that role group's pool |
| `google_project_iam_member.plan` | Attaches project-level roles to each plan service account |
| `google_project_iam_member.apply` | Attaches project-level roles to each apply service account |
| `tfe_variable_set.this` | Project-owned variable sets, one per (role group, project) pair |
| `tfe_project_variable_set.this` | Auto-attaches a variable set to every workspace in its project, one per (role group, project) pair with `apply_to_all_workspaces = true` |
| `tfe_variable.provider_auth` | `TFC_GCP_PROVIDER_AUTH=true` on each variable set |
| `tfe_variable.workload_provider_name` | `TFC_GCP_WORKLOAD_PROVIDER_NAME` on each variable set |
| `tfe_variable.plan_sa_email` | `TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL` on each variable set |
| `tfe_variable.apply_sa_email` | `TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL` on each variable set |
| `data.tfe_project.this` | Looks up each target project by name |
