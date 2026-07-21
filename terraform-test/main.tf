terraform {
  required_version = ">= 1.13.4"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 6.0.0, < 8.0.0"
    }
    tfe = {
      source  = "hashicorp/tfe"
      version = "~> 0.76"
    }
  }
}

locals {
  role_group_projects = merge([
    for role_group, config in var.role_groups : {
      for project, project_config in config.projects :
      "${role_group}:${project}" => {
        role_group              = role_group
        project                 = project
        workspace_names         = project_config.workspace_names
        apply_to_all_workspaces = project_config.apply_to_all_workspaces
      }
    }
  ]...)

  all_projects = toset([for entry in local.role_group_projects : entry.project])

  plan_role_bindings = merge([
    for role_group, config in var.role_groups : {
      for idx, role in config.plan_roles :
      "${role_group}:${idx}" => {
        role_group = role_group
        role       = role
      }
    }
  ]...)

  apply_role_bindings = merge([
    for role_group, config in var.role_groups : {
      for idx, role in config.apply_roles :
      "${role_group}:${idx}" => {
        role_group = role_group
        role       = role
      }
    }
  ]...)

  attribute_condition_for = {
    for role_group, config in var.role_groups : role_group => join(" || ", flatten([
      for project, project_config in config.projects : [
        for workspace in project_config.workspace_names :
        workspace == "*" ?
        "assertion.sub.startsWith(\"organization:${var.organization}:project:${project}:workspace:\")" :
        "assertion.sub.startsWith(\"organization:${var.organization}:project:${project}:workspace:${workspace}:\")"
      ]
    ]))
  }
}

data "tfe_project" "this" {
  for_each     = local.all_projects
  name         = each.value
  organization = var.organization
}

resource "google_project_service" "this" {
  for_each           = var.project_services
  project            = var.gcp_project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_iam_workload_identity_pool" "this" {
  for_each                  = var.role_groups
  project                   = var.gcp_project_id
  workload_identity_pool_id = "${var.pool_id_prefix}-${each.key}"
  depends_on                = [google_project_service.this]
}

resource "google_iam_workload_identity_pool_provider" "this" {
  for_each                           = var.role_groups
  project                            = var.gcp_project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.this[each.key].workload_identity_pool_id
  workload_identity_pool_provider_id = var.pool_provider_id
  attribute_condition                = local.attribute_condition_for[each.key]

  attribute_mapping = {
    "google.subject"                        = "assertion.sub"
    "attribute.aud"                         = "assertion.aud"
    "attribute.terraform_run_phase"         = "assertion.terraform_run_phase"
    "attribute.terraform_project_id"        = "assertion.terraform_project_id"
    "attribute.terraform_project_name"      = "assertion.terraform_project_name"
    "attribute.terraform_workspace_id"      = "assertion.terraform_workspace_id"
    "attribute.terraform_workspace_name"    = "assertion.terraform_workspace_name"
    "attribute.terraform_organization_id"   = "assertion.terraform_organization_id"
    "attribute.terraform_organization_name" = "assertion.terraform_organization_name"
    "attribute.terraform_run_id"            = "assertion.terraform_run_id"
    "attribute.terraform_full_workspace"    = "assertion.terraform_full_workspace"
  }

  oidc {
    issuer_uri = "https://app.terraform.io"
  }
}

resource "google_service_account" "plan" {
  for_each   = var.role_groups
  project    = var.gcp_project_id
  account_id = "${var.service_account_id_prefix}-${each.key}-plan"
  depends_on = [google_project_service.this]
}

resource "google_service_account" "apply" {
  for_each   = var.role_groups
  project    = var.gcp_project_id
  account_id = "${var.service_account_id_prefix}-${each.key}-apply"
  depends_on = [google_project_service.this]
}

resource "google_service_account_iam_member" "plan" {
  for_each           = var.role_groups
  service_account_id = google_service_account.plan[each.key].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.this[each.key].name}/attribute.terraform_run_phase/plan"
}

resource "google_service_account_iam_member" "apply" {
  for_each           = var.role_groups
  service_account_id = google_service_account.apply[each.key].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.this[each.key].name}/attribute.terraform_run_phase/apply"
}

resource "google_project_iam_member" "plan" {
  for_each = local.plan_role_bindings
  project  = var.gcp_project_id
  role     = each.value.role
  member   = "serviceAccount:${google_service_account.plan[each.value.role_group].email}"
}

resource "google_project_iam_member" "apply" {
  for_each = local.apply_role_bindings
  project  = var.gcp_project_id
  role     = each.value.role
  member   = "serviceAccount:${google_service_account.apply[each.value.role_group].email}"
}

resource "tfe_variable_set" "this" {
  for_each          = local.role_group_projects
  name              = "${each.value.project}-${var.gcp_project_name}-${each.value.role_group}-${var.variable_set_name_suffix}"
  organization      = var.organization
  parent_project_id = data.tfe_project.this[each.value.project].id
}

resource "tfe_project_variable_set" "this" {
  for_each        = { for k, v in local.role_group_projects : k => v if v.apply_to_all_workspaces }
  project_id      = data.tfe_project.this[each.value.project].id
  variable_set_id = tfe_variable_set.this[each.key].id
}

resource "tfe_variable" "provider_auth" {
  for_each        = local.role_group_projects
  key             = "TFC_GCP_PROVIDER_AUTH"
  value           = "true"
  category        = "env"
  variable_set_id = tfe_variable_set.this[each.key].id
}

resource "tfe_variable" "workload_provider_name" {
  for_each        = local.role_group_projects
  key             = "TFC_GCP_WORKLOAD_PROVIDER_NAME"
  value           = google_iam_workload_identity_pool_provider.this[each.value.role_group].name
  category        = "env"
  variable_set_id = tfe_variable_set.this[each.key].id
}

resource "tfe_variable" "plan_sa_email" {
  for_each        = local.role_group_projects
  key             = "TFC_GCP_PLAN_SERVICE_ACCOUNT_EMAIL"
  value           = google_service_account.plan[each.value.role_group].email
  category        = "env"
  variable_set_id = tfe_variable_set.this[each.key].id
}

resource "tfe_variable" "apply_sa_email" {
  for_each        = local.role_group_projects
  key             = "TFC_GCP_APPLY_SERVICE_ACCOUNT_EMAIL"
  value           = google_service_account.apply[each.value.role_group].email
  category        = "env"
  variable_set_id = tfe_variable_set.this[each.key].id
}

output "workload_identity_pool_names" {
  value = { for role_group, pool in google_iam_workload_identity_pool.this : role_group => pool.name }
}

output "workload_identity_pool_provider_names" {
  value = { for role_group, provider in google_iam_workload_identity_pool_provider.this : role_group => provider.name }
}

output "plan_service_account_emails" {
  value = { for role_group, sa in google_service_account.plan : role_group => sa.email }
}

output "apply_service_account_emails" {
  value = { for role_group, sa in google_service_account.apply : role_group => sa.email }
}

output "variable_set_names" {
  value = { for key, vs in tfe_variable_set.this : key => vs.name }
}
