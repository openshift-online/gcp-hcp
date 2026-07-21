variable "organization" {
  type = string
}

variable "gcp_project_id" {
  type        = string
  description = "GCP project ID to target. All resources (workload identity pools, service accounts, IAM bindings) are created in this project."
}

variable "gcp_project_name" {
  type        = string
  description = <<-EOT
    Short display name for the GCP project (e.g. "gcp-prod"). Used to derive
    variable set names. Deliberately separate from gcp_project_id, since
    project IDs can be auto-generated and unreadable.
  EOT
}

variable "project_services" {
  type = set(string)
  default = [
    "iam.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "sts.googleapis.com",
    "iamcredentials.googleapis.com",
  ]
  description = "GCP APIs to enable on gcp_project_id. Defaults to the 4 APIs required for Workload Identity Federation."
}

variable "service_account_id_prefix" {
  type        = string
  default     = "hcp-tf"
  description = "Prefix for generated service account IDs: \"{service_account_id_prefix}-{role_group}-plan\"/\"-apply\". GCP limits service account IDs to 30 characters, so with the default 6-character prefix, role group names can be up to 17 characters before hitting the limit."
}

variable "pool_id_prefix" {
  type        = string
  default     = "hcp-tf"
  description = "Prefix for generated Workload Identity Pool and Provider IDs: \"{pool_id_prefix}-{role_group}\". GCP limits pool/provider IDs to 32 characters, so with the default 6-character prefix, role group names can be up to 25 characters before hitting the limit."
}

variable "pool_provider_id" {
  type        = string
  default     = "oidc"
  description = "Workload Identity Pool Provider ID, created within each role group's pool. Only needs to be unique within a pool, and each pool has exactly one provider, so the default is safe to leave as-is."
}

variable "variable_set_name_suffix" {
  type        = string
  default     = "gcp-dynamic-creds"
  description = "Suffix for generated variable set names: \"{project}-{gcp_project_name}-{role_group}-{variable_set_name_suffix}\"."
}

variable "role_groups" {
  type = map(object({
    plan_roles  = optional(list(string), ["roles/viewer"])
    apply_roles = optional(list(string), ["roles/owner"])
    projects = map(object({
      workspace_names         = optional(set(string), ["*"])
      apply_to_all_workspaces = optional(bool, false)
    }))
  }))
  description = <<-EOT
    Named groups of GCP identities for this project. Each group gets its own
    Workload Identity Pool, its own plan/apply service account pair --
    plan_roles defaults to roles/viewer, and apply_roles defaults to
    roles/owner, so a role group can manage/re-apply this same module using
    its own dynamic credentials once bootstrapped -- and its own
    project-owned variable set per project listed in its projects map
    (see service_account_id_prefix/variable_set_name_suffix for the naming
    pattern). workspace_names scopes which workspace names within that
    project may impersonate that group's service accounts, defaulting to
    ["*"] (all workspaces). The same HCP Terraform project can appear under
    multiple role groups, so different workspaces within it can use
    different service accounts/roles -- each workspace opts into the
    matching variable set via variable_set_names, unless
    apply_to_all_workspaces is set, which auto-attaches that project's
    variable set to every current and future workspace in it instead.
    apply_to_all_workspaces and workspace_names are independent controls:
    the former decides who gets the TFC_GCP_* env vars, the latter decides
    who can actually authenticate with them -- pairing
    apply_to_all_workspaces = true with a workspace_names narrower than the
    default ["*"] means every workspace in the project receives credentials
    but only the listed ones can complete authentication.

    Each role group gets its own Workload Identity Pool, letting that
    pool's provider scope its attribute_condition to only that group's
    workspaces -- so tokens issued for one role group can't be used to
    impersonate another role group's service accounts.
  EOT

  validation {
    condition = alltrue([
      for k in keys(var.role_groups) :
      length(k) <= 30 - length(var.service_account_id_prefix) - length("-apply") - 1
    ])
    error_message = "Role group name too long: with the given service_account_id_prefix, \"{prefix}-{name}-apply\" must fit GCP's 30-character service account ID limit."
  }

  validation {
    condition = alltrue([
      for k in keys(var.role_groups) :
      length(k) <= 32 - length(var.pool_id_prefix) - 1
    ])
    error_message = "Role group name too long: with the given pool_id_prefix, \"{prefix}-{name}\" must fit GCP's 32-character Workload Identity Pool ID limit."
  }

  validation {
    condition     = alltrue([for config in values(var.role_groups) : length(config.projects) > 0])
    error_message = "Each role group must list at least one project; an empty projects map produces an invalid (empty) attribute_condition on that role group's Workload Identity Pool Provider."
  }
}
