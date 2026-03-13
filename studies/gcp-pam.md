# GCP Privileged Access Manager (PAM)

## Overview

GCP [Privileged Access Manager (PAM)](https://cloud.google.com/iam/docs/pam-overview) replaces standing (always-on) elevated IAM privileges with just-in-time (JIT), time-bounded, approval-gated access. Instead of granting persistent elevated roles to users or service accounts, administrators configure **entitlements** that define who can request what roles, for how long, and with what approval workflow. Principals then request temporary elevation (**grants**) against these entitlements when they need elevated access.

PAM reached General Availability (GA) in 2024 and is available for projects, folders, and organizations.

## Key Concepts

| Concept | Description |
|---|---|
| Entitlement | Admin-created configuration defining who can request what roles, for how long, and with what approval workflow. Scoped to a project, folder, or organization. |
| Grant | An instance of access elevation. Requested by an eligible user against an entitlement. Time-bounded and audited. |
| Eligible Users | Principals listed in the entitlement who are allowed to request grants. Can be users, groups, service accounts, or Workload Identity Federation principals. |
| Approvers | Principals who approve or deny grant requests. Up to 20 per approval step; use groups to exceed this limit. |
| Max Request Duration | Maximum time a grant can remain active, configured per entitlement. Range: 30 minutes to 7 days. Requesters can ask for less than the maximum. |
| Justification | Optional free-text explanation provided by the requester. Appears in audit logs and approval notifications. |

## Grant Lifecycle

A grant transitions through the following states:

```
Requested --> Approved --> Active --> Expired
                |                      |
                |                      +---> Revoked
                |
                +---> Denied
                +---> Withdrawn (by requester)
```

- **Requested**: Grant has been submitted and is awaiting approval (or auto-activates if no approval is required).
- **Active**: Grant has been approved and the IAM role binding is in effect.
- **Expired**: Grant duration has elapsed and the role binding has been removed.
- **Revoked**: An administrator or approver has revoked the grant before expiry.
- **Denied**: An approver has denied the request.
- **Withdrawn**: The requester has withdrawn their own request before approval.

Important timing:
- Unapproved requests auto-expire after **24 hours**.
- Grant records are deleted from PAM **30 days** after reaching a terminal state (expired, revoked, denied, withdrawn).
- Audit log records persist according to the Cloud Audit Log retention policy (default 400 days for Admin Activity logs).

## Approval Workflow

See [Approve or deny grants](https://cloud.google.com/iam/docs/pam-approve-deny-grants) for full details.

- **Single-level approval**: Standard, always available. One approval step with configurable number of required approvals.
- **Two-level approval**: Requires Security Command Center (SCC) Premium or Enterprise. Sequential approval -- the second level is only triggered after the first level approves.
- Maximum **20 principals** per approvers block. Use groups to exceed this limit.
- Maximum **5 approvals needed** per level (with SCC Premium).
- A principal **cannot self-approve**, even if they are a member of an approver group.
- **Service accounts as approvers** (preview): Enables programmatic/automated approval workflows.

## Notifications

- Approvers receive **email notifications** when a grant is requested against an entitlement they can approve.
- Additional notification targets are configurable: admin and requester email recipients can be added to the entitlement configuration.
- **Pub/Sub integration**: Entitlements can publish grant state changes to a Pub/Sub topic for custom alerting, SIEM integration, or event-driven automation.
- **Cloud Asset Inventory feeds** can trigger on grant state changes for broader organizational visibility.

## How to Request a Grant

See [Request a grant](https://cloud.google.com/iam/docs/pam-request-grant) for full details.

### CLI

```bash
# Request access
gcloud beta pam grants create \
  --entitlement=workflows-invoker \
  --location=global \
  --project=PROJECT_ID \
  --requested-duration=3600s \
  --justification="Investigating pod crash in namespace clusters-abc123"

# Check grant status
gcloud beta pam grants search \
  --entitlement=workflows-invoker \
  --location=global \
  --project=PROJECT_ID \
  --caller-relationship=had-created
```

### Other Methods

- **Console**: Navigate to IAM & Admin > PAM in the Google Cloud Console. Select the entitlement and click "Request Access".
- **REST API**: `POST https://privilegedaccessmanager.googleapis.com/v1/projects/PROJECT_ID/locations/global/entitlements/ENTITLEMENT_ID/grants`

## How to Approve a Grant

### CLI

```bash
# List pending requests awaiting your approval
gcloud beta pam grants search \
  --entitlement=workflows-invoker \
  --location=global \
  --project=PROJECT_ID \
  --caller-relationship=can-approve

# Approve
gcloud beta pam grants approve GRANT_ID \
  --entitlement=workflows-invoker \
  --location=global \
  --project=PROJECT_ID \
  --reason="Approved for incident INC-12345"

# Deny
gcloud beta pam grants deny GRANT_ID \
  --entitlement=workflows-invoker \
  --location=global \
  --project=PROJECT_ID \
  --reason="Not justified"
```

### Other Methods

- **Console**: Navigate to IAM & Admin > PAM > Approve/Deny in the Google Cloud Console.
- **REST API**: `POST .../grants/GRANT_ID:approve` or `POST .../grants/GRANT_ID:deny`

## How to Revoke a Grant

An active grant can be revoked before its expiry:

```bash
gcloud beta pam grants revoke GRANT_ID \
  --entitlement=workflows-invoker \
  --location=global \
  --project=PROJECT_ID \
  --reason="No longer needed"
```

Revocation removes the temporary IAM binding immediately.

## Audit Logging

All PAM operations are logged to Cloud Audit Logs under the service name `privilegedaccessmanager.googleapis.com`.

Logged information includes:
- Requester identity
- Approver identity and justification
- Grant state transitions (requested, approved, denied, activated, expired, revoked)
- Timestamps for all transitions
- Requester-provided justification text

### Query Example

```bash
gcloud logging read \
  'protoPayload.serviceName="privilegedaccessmanager.googleapis.com"' \
  --project=PROJECT_ID --limit=20 \
  --format='table(timestamp,protoPayload.methodName,protoPayload.authenticationInfo.principalEmail)'
```

## Constraints and Limitations

See [PAM quotas and limits](https://cloud.google.com/iam/docs/pam-quotas) for the full list.

| Constraint | Detail |
|---|---|
| No legacy basic roles | Owner, Editor, and Viewer cannot be used in entitlements. Use predefined or custom roles instead. |
| Max 30 roles per entitlement | Use multiple entitlements if more roles are needed. |
| Max 20 principals per eligible\_users/approvers | Use groups to exceed this limit. |
| Max 2 approval levels | Second level requires SCC Premium or Enterprise. |
| Max grant duration: 7 days | Minimum: 30 minutes. |
| 24-hour request expiry | Unapproved requests auto-expire after 24 hours. |
| Grant records kept 30 days | After reaching a terminal state (expired, revoked, denied, withdrawn). |
| Entitlement ID immutable | Cannot rename an entitlement without destroy and recreate. |
| Authoritative IAM conflict | `google_project_iam_policy` and `google_project_iam_binding` will wipe PAM-injected bindings on next apply. **MUST** use `google_project_iam_member` instead. |
| No native CI/CD integration | No built-in hooks for Atlantis, GitHub Actions, or Tekton. Custom automation required. |
| Service accounts via CLI/API only | Service accounts cannot request grants via the Console. Must use `gcloud` or the REST API. |

## Terraform Resources

### google\_privileged\_access\_manager\_entitlement

See [Terraform Registry documentation](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/privileged_access_manager_entitlement) for the full resource reference. See also the [Create entitlements](https://cloud.google.com/iam/docs/pam-create-entitlements) guide.

```hcl
resource "google_privileged_access_manager_entitlement" "example" {
  entitlement_id       = "workflows-invoker"
  location             = "global"
  max_request_duration = "3600s"
  parent               = "projects/my-project"

  requester_justification_config {
    unstructured {}
  }

  eligible_users {
    principals = [
      "group:sre-team@example.com",
      "serviceAccount:automation@my-project.iam.gserviceaccount.com",
    ]
  }

  privileged_access {
    gcp_iam_access {
      resource      = "//cloudresourcemanager.googleapis.com/projects/my-project"
      resource_type = "cloudresourcemanager.googleapis.com/Project"
      role_bindings {
        role = "roles/workflows.invoker"
      }
    }
  }

  approval_workflow {
    manual_approvals {
      require_approver_justification = false
      steps {
        approvals_needed = 1
        approvers {
          principals = ["group:approvers@example.com"]
        }
      }
    }
  }
}
```

### Official Terraform Module

Google provides an official Terraform module for PAM: [`GoogleCloudPlatform/pam/google`](https://registry.terraform.io/modules/GoogleCloudPlatform/pam/google). It wraps the entitlement resource with opinionated defaults and validation. Evaluate it for use but be aware that the raw resource provides full flexibility.

## Integration with Cloud Workflows

### Current Implementation: Tag-Based Per-Workflow Gating

The gcp-hcp project uses GCP Resource Tags combined with IAM conditions to gate specific workflows behind PAM approval, while leaving routine workflows accessible with permanent permissions.

**How it works:**

1. A [tag key](https://cloud.google.com/resource-manager/docs/tags/tags-creating-and-managing) `pam-gated` with value `true` is created at the project level.
2. Workflows that require PAM gating are tagged via [`google_tags_location_tag_binding`](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/tags_location_tag_binding) with `pam-gated=true`.
3. The permanent `roles/workflows.invoker` binding carries an [IAM condition](https://cloud.google.com/iam/docs/conditions-overview) that excludes tagged workflows:
   ```
   !resource.matchTag('PROJECT/pam-gated', 'true')
   ```
4. A PAM entitlement grants **unconditional** `roles/workflows.invoker` temporarily upon approval.

This means:
- Untagged workflows (routine operations) remain permanently invocable.
- Tagged workflows (sensitive operations) require a PAM grant to invoke.

```
Operator/Automation
  |
  +-- invoke "get" workflow ------- tag: none ---------- IAM condition passes --- OK (permanent)
  +-- invoke "describe" ----------- tag: none ---------- IAM condition passes --- OK (permanent)
  +-- invoke "logs" --------------- tag: pam-gated=true -- IAM condition fails -- DENIED
                                      |
                                      +-- Request PAM grant -> Approval -> Temporary invoker -> OK
```

### Marking Workflows as PAM-Gated

Workflows are marked for PAM gating in `terraform/metadata/workflows.yaml`:

```yaml
workflows:
  get:
    targets: [region, management-cluster]
    pam_gated: false

  logs:
    targets: [region, management-cluster]
    pam_gated: true   # This workflow requires PAM approval
```

> **Naming convention**: The YAML metadata key uses underscores (`pam_gated`) to follow YAML/Terraform conventions, while the GCP Resource Tag key uses hyphens (`pam-gated`) to follow GCP naming conventions. The Terraform module handles the mapping between the two.

The Terraform workflows module reads this metadata and automatically:

- Tags the workflow resource with `pam-gated=true`
- Excludes it from the permanent invoker binding via IAM condition

### Future: In-Workflow PAM Pattern

In this advanced pattern, the workflow itself requests PAM access as part of its execution, enabling fully automated elevated-access workflows with human approval gates.

**Flow:**

1. Workflow starts with limited permissions.
2. Workflow calls the PAM REST API to create a grant request.
3. Workflow creates a [callback endpoint](https://cloud.google.com/workflows/docs/creating-callback-endpoints) (`events.create_callback_endpoint`).
4. Workflow waits for approval (`events.await_callback`, configurable timeout up to [365 days](https://cloud.google.com/workflows/quotas)).
5. Approver approves the grant (via Console, CLI, or API). A Pub/Sub event fires, and a Cloud Run service posts to the callback URL.
6. Workflow resumes with elevated permissions (PAM grant is now active).
7. Workflow executes the sensitive operation.
8. Grant auto-expires after the configured duration.

**Pseudo-YAML:**

```yaml
main:
  steps:
    - request_pam_grant:
        call: http.post
        args:
          url: https://privilegedaccessmanager.googleapis.com/v1/projects/PROJECT/locations/global/entitlements/ENTITLEMENT/grants
          auth:
            type: OAuth2
          body:
            requestedDuration: "3600s"
            justification:
              unstructuredJustification: ${args.justification}
        result: grant_response

    - create_callback:
        call: events.create_callback_endpoint
        args:
          http_callback_method: POST
        result: callback

    - notify_approver:
        # Send callback URL to approver via Slack/email/ticket system
        # Implementation depends on notification channel

    - await_approval:
        call: events.await_callback
        args:
          callback: ${callback}
          timeout: 86400  # 24 hours
        result: approval_response

    - execute_sensitive_operation:
        # Now has elevated permissions via PAM grant
        call: gke.request
        args:
          # ... sensitive operation parameters
```

This pattern is documented for future implementation. The current PoC uses the pre-invocation PAM pattern (tag-based gating).

### Automation Callers

- Service accounts (Cloud Run, other workflows, AI agents) can be listed as `eligible_users` in entitlements.
- They request grants via the PAM REST API (not the Console).
- The same approval workflow applies -- approvers still review and approve/deny.
- This is useful for automated pipelines that need temporary elevated access with a human approval gate.

## Security Considerations

- **IAM is additive — overlapping roles bypass PAM gating**: If a principal has another role that includes `workflows.run` (e.g., `roles/owner`, `roles/editor`, or a custom role), the tag-based IAM condition on `roles/workflows.invoker` is irrelevant because the other binding grants unconditional access. PAM-gated workflows only work when the caller's invoke access comes exclusively from the conditioned binding. Ensure no overlapping roles grant workflow invoke permissions to PAM-eligible principals.
- **Tag management is privileged**: Anyone with `roles/resourcemanager.tagUser` can attach tags that satisfy IAM conditions. Treat tag management as a security-sensitive operation and restrict this role.
- **IAM condition evaluation timing**: IAM condition evaluation may not be instantaneous after tag changes due to token caching and expiry. Plan for propagation delay.
- **Avoid self-escalation roles**: Do not include roles that allow self-escalation (`setIamPolicy`, `iam.roles.update`) in entitlements. A principal with these roles could grant themselves permanent access, defeating PAM.
- **Avoid service agent roles**: Do not include service agent roles in entitlements. These are managed by GCP and not intended for user assignment.
- **Keep grant durations short**: Use the minimum duration practical for the operation. Shorter durations reduce the blast radius of compromised credentials.
- **Use non-authoritative Terraform IAM resources**: Always use `google_project_iam_member` (not `_binding` or `_policy`) to avoid overwriting PAM-injected IAM bindings. This repository already follows this pattern.

## Future Work

The following items are out of scope for the initial scaffolding but are planned:

- **In-workflow PAM pattern implementation**: Workflow requests its own PAM grant and waits for approval via callback endpoints.
- **Approval request management integrations**: Slack notifications when grants are requested, ticket system integration for approval tracking.
- **Config-level PAM enablement across environments**: Rolling out PAM entitlements to integration, stage, and production via Atlantis PRs.
- **Automated approval bots**: Service account approvers with policy logic (e.g., auto-approve during incident windows, require ticket references).
- **Additional PAM-gated workflows**: Delete pod, expand PV, restart deployment, cordon/uncordon nodes, and other sensitive operations.
- **ArgoCD/Helm integration for PAM-aware deployments**: Entitlement definitions managed as Helm chart values, deployed via ArgoCD.
- **Workflow YAML changes to surface PAM status to callers**: Return grant status, approval state, and remaining duration in workflow outputs.

## References

### PAM

- [PAM Overview](https://cloud.google.com/iam/docs/pam-overview)
- [Create Entitlements](https://cloud.google.com/iam/docs/pam-create-entitlements)
- [Request a Grant](https://cloud.google.com/iam/docs/pam-request-grant)
- [Approve/Deny Grants](https://cloud.google.com/iam/docs/pam-approve-deny-grants)
- [PAM Quotas and Limits](https://cloud.google.com/iam/docs/pam-quotas)
- [PAM REST API Reference](https://cloud.google.com/iam/docs/reference/pam/rest)
- [PAM Audit Logging](https://cloud.google.com/iam/docs/pam-audit-logging)

### Resource Tags and IAM Conditions

- [Tags Overview](https://cloud.google.com/resource-manager/docs/tags/tags-overview)
- [Creating and Managing Tags](https://cloud.google.com/resource-manager/docs/tags/tags-creating-and-managing)
- [Tags and Conditional Access](https://cloud.google.com/iam/docs/tags-access-control)
- [Services that Support Tags](https://cloud.google.com/resource-manager/docs/tags/tags-supported-services)
- [IAM Conditions Overview](https://cloud.google.com/iam/docs/conditions-overview)
- [Cloud Workflows Tag Management](https://cloud.google.com/workflows/docs/create-manage-tags)

### Cloud Workflows

- [Workflows Callbacks](https://cloud.google.com/workflows/docs/creating-callback-endpoints)
- [Workflows Quotas and Limits](https://cloud.google.com/workflows/quotas)
- [Use IAM to Control Access to Workflows](https://cloud.google.com/workflows/docs/use-iam-for-access)

### Terraform

- [google\_privileged\_access\_manager\_entitlement](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/privileged_access_manager_entitlement)
- [google\_tags\_location\_tag\_binding](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/tags_location_tag_binding)
- [Terraform Module: GoogleCloudPlatform/pam/google](https://registry.terraform.io/modules/GoogleCloudPlatform/pam/google)
