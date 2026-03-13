# Use GCP PAM with Resource Tags to Gate Sensitive Cloud Workflows

***Scope***: GCP-HCP

**Date**: 2026-03-13

## Decision

Use GCP Privileged Access Manager (PAM) combined with GCP Resource Tags and IAM conditions to gate sensitive Cloud Workflows behind approval-based, time-bounded access. Workflows are marked as PAM-gated via metadata, automatically tagged at the resource level, and excluded from permanent `roles/workflows.invoker` bindings through IAM conditions. Operators and automation must request a temporary PAM grant (with approval) before invoking gated workflows.

## Context

- **Problem Statement**: The GCP HCP platform uses Cloud Workflows for Zero Operator Access operations (get, describe, logs, and future write operations). Currently, all operators with the `roles/workflows.invoker` role can invoke any workflow at any time without oversight. As the platform adds sensitive workflows (pod deletion, PV expansion, configuration changes), there is no mechanism to require approval before execution or to limit access duration. Both human operators and AI agents/automation need to be subject to the same access controls.

- **Constraints**:
  - Cloud Workflows does not support IAM conditions on `resource.name` — project-level `roles/workflows.invoker` grants invoke access to all workflows in the project
  - PAM operates at the IAM binding level — it adds/removes project-level role bindings, not resource-level ones
  - The solution must work for both human operators (Console/CLI) and automation callers (service accounts via API)
  - Must use non-authoritative IAM resources (`google_project_iam_member`) to avoid conflicts between Terraform and PAM-injected bindings — the codebase already follows this pattern
  - Per-workflow granularity is required: routine read-only workflows must remain permanently accessible

- **Assumptions**:
  - Cloud Workflows will continue to support GCP Resource Tags and `resource.matchTag()` in IAM conditions
  - The Google Terraform provider (>= 7.17) supports all required resources (`google_privileged_access_manager_entitlement`, `google_tags_tag_key`, `google_tags_location_tag_binding`)
  - PAM approval latency (email notification + human review) is acceptable for the target use cases — these are not time-critical automated pipelines but operational actions requiring human judgment

## Alternatives Considered

1. **Resource Tags + IAM conditions (chosen)**: Tag PAM-gated workflows with `pam-gated=true`. Grant permanent `roles/workflows.invoker` with an IAM condition `!resource.matchTag('PROJECT/pam-gated', 'true')` to exclude tagged workflows. Create a PAM entitlement that grants unconditional `roles/workflows.invoker` temporarily. Non-gated workflows remain permanently accessible; gated workflows require a PAM grant.

2. **Resource-level IAM per workflow**: Remove the project-level `roles/workflows.invoker` binding entirely. Grant `roles/workflows.invoker` on each individual workflow resource using `google_workflows_workflow_iam_member` for non-gated workflows. PAM grants project-level access temporarily for gated ones. Achieves per-workflow control without tags.

3. **Gate all workflows behind PAM (project-level)**: Remove the permanent `roles/workflows.invoker` binding. All workflow invocations require a PAM grant. Simplest implementation but gates read-only operations unnecessarily.

4. **In-workflow PAM request**: Keep permanent invoke access for all workflows, but have sensitive workflows request their own PAM grant internally (call PAM API, wait for approval via callback, then proceed). The workflow itself becomes the approval gate.

## Decision Rationale

* **Justification**: Alternative 1 (Resource Tags) provides clean per-workflow granularity using native GCP capabilities. Tags are a first-class GCP concept supported by Cloud Workflows, and `resource.matchTag()` IAM conditions are evaluated per-resource. This enables the simplest operational model: mark a workflow as `pam_gated: true` in metadata, and the Terraform module automatically handles tagging and IAM configuration.

* **Evidence**: GCP documentation confirms Cloud Workflows is a [supported service for Resource Tags](https://cloud.google.com/resource-manager/docs/tags/tags-supported-services) with dedicated [tag management documentation](https://cloud.google.com/workflows/docs/create-manage-tags). IAM conditions with `resource.matchTag()` are evaluated at the individual workflow resource level, as confirmed by the Workflows tag documentation stating that "changing or deleting the tag attached to a resource can remove user access to that resource."

* **Comparison**:
  - Alternative 2 (resource-level IAM) achieves similar granularity but requires managing N IAM bindings per workflow per member (cross-product), is harder to reason about, and doesn't leverage the existing project-level IAM pattern
  - Alternative 3 (gate all) is too coarse — forcing PAM approval for read-only `get` and `describe` operations adds friction without security benefit
  - Alternative 4 (in-workflow PAM) is architecturally elegant but significantly more complex (callback endpoints, Pub/Sub integration, long-running executions), better suited as a future enhancement once the foundational PAM infrastructure is proven

## Consequences

### Positive

* Per-workflow access control without changing the project-level IAM model — adding a new gated workflow requires only a metadata flag (`pam_gated: true` in YAML, which maps to the GCP Resource Tag key `pam-gated`)
* Time-bounded access with automatic expiry — no standing elevated privileges for sensitive operations
* Full audit trail — PAM logs requester, approver, justification, and grant lifecycle in Cloud Audit Logs
* Works for both human operators and automation/AI agents — service accounts can request grants via the PAM API
* Non-disruptive — read-only workflows remain permanently accessible; only sensitive workflows require approval
* Extensible — future workflows (delete pod, expand PV, restart deployment) simply set `pam_gated: true` in metadata

### Negative

* During a PAM grant window, the operator has `roles/workflows.invoker` on all workflows (not just the gated one) — acceptable since non-gated workflows were already permanently accessible
* IAM is additive — if a principal has another role that includes `workflows.run` (e.g., `roles/owner`, `roles/editor`, or a custom role), the tag-based IAM condition is bypassed because the unconditional binding grants access regardless. PAM-gated workflows only work when the caller's invoke access comes exclusively from the conditioned `roles/workflows.invoker` binding.
* Tag management is a privileged operation — anyone with `roles/resourcemanager.tagUser` could remove the `pam-gated` tag from a workflow, bypassing the gate. Tag management permissions must be restricted.
* IAM condition evaluation is not instantaneous after tag changes — token caching means there may be a propagation delay
* PAM approval adds latency to the operational workflow — operators must request and wait for approval before acting. This is by design for sensitive operations but could slow incident response if approval is not timely.
* One additional GCP API to enable per project (`privilegedaccessmanager.googleapis.com`)

## Cross-Cutting Concerns

### Reliability:

* **Observability**: PAM grant requests, approvals, denials, and expirations are all captured in Cloud Audit Logs under `privilegedaccessmanager.googleapis.com`. Workflow execution audit logs (under `workflowexecutions.googleapis.com`) already capture who invoked which workflow. Together, these provide end-to-end traceability: who requested access, who approved it, and what was executed.
* **Resiliency**: PAM is a managed GCP service with no self-hosted components. Tag bindings and IAM conditions are evaluated by the GCP IAM infrastructure. No additional failure modes are introduced beyond standard GCP API availability.

### Security:

* Eliminates standing elevated access for sensitive operations — access is temporary, approved, and justified
* Both execution and approval are logged with requester/approver identity
* Tag key/value management must be restricted to prevent unauthorized removal of `pam-gated` tags
* PAM does not allow self-approval — even members of the same approver group cannot approve their own requests
* The entitlement uses `requester_justification_config { unstructured {} }` to require free-text justification with every grant request

### Cost:

* PAM itself has no additional per-API-call cost — it is included in GCP IAM
* Resource Tags are free
* No additional infrastructure components (serverless, managed by GCP)

### Operability:

* Operators request grants via `gcloud beta pam grants create` or the Console — no new tooling required
* Approvers receive email notifications and can approve via Console or CLI
* Adding a new PAM-gated workflow requires only setting `pam_gated: true` in `terraform/metadata/workflows.yaml` — no manual IAM or tag configuration
* The `wf-cli.sh` wrapper may be enhanced in the future to detect PAM-gated workflows and prompt for grant requests automatically
