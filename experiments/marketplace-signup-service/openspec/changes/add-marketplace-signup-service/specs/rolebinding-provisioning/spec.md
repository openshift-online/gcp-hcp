# Spec Delta

## Purpose

Granting the initial `service-admin` RoleBinding for the verified user in the Gecko logical namespace derived from their project — by invoking the platform's existing `activate-project` Cloud Workflow, which is the documented pre-Marketplace activation path.

## ADDED Requirements

### Requirement: Namespace derivation from project ID

THE SYSTEM SHALL derive the target Gecko logical namespace from the verified GCP project ID (the project ID itself), validated against the same syntax the `activate-project` workflow enforces (`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`), and SHALL NOT create it as a Kubernetes namespace — it is a Gecko logical namespace served by the platform CRD API.

#### Scenario: Valid project ID

- **WHEN** sign-up succeeds for project ID `acme-corp-platform`
- **THEN** activation is requested for logical namespace `acme-corp-platform`

#### Scenario: Malformed project ID

- **WHEN** the submitted project ID fails syntax validation (length, characters, leading/trailing characters)
- **THEN** the request is rejected before activation is requested

### Requirement: Activation via the existing activate-project workflow

THE SYSTEM SHALL grant the initial binding by invoking the existing `activate-project` Cloud Workflow with `project_namespace` (the derived namespace) and `subject` (the normalized verified email), reusing the workflow as provisioned — no new provisioning workflow is introduced by this change.

#### Scenario: First sign-up for a project

- **WHEN** identity validation passes for email `Alice@example.com` and project `acme-corp-platform`
- **THEN** the `activate-project` workflow creates the `service-admin-rb` RoleBinding with subject `Alice@example.com` and roleRef `PlatformRole/service-admin` in `acme-corp-platform`

#### Scenario: Project already activated by a different user

- **WHEN** a different verified email completes sign-up for a project whose `service-admin-rb` already exists with another subject
- **THEN** the workflow rejects the request (409) and the sign-up reports that the project is already administered by another user, without altering the existing binding

#### Scenario: Reserved project name

- **WHEN** the submitted project ID matches one of the workflow's reserved namespace names
- **THEN** the workflow rejects the request (403) and the sign-up reports a reserved-name failure, without attempting to alter any binding

#### Scenario: Transient workflow failure

- **WHEN** the workflow execution fails for infrastructure reasons (timeout, PlatformRole unreadable after retries, GKE API error)
- **THEN** the sign-up reports a transient failure, may be retried by the user, and no partial binding state is left behind

### Requirement: Idempotent activation

THE SYSTEM SHALL report success for repeated sign-up by the same email for the same project: the `activate-project` workflow returns an already-active result and the binding is unchanged.

#### Scenario: Repeated sign-up

- **WHEN** the same email completes sign-up for the same project a second time
- **THEN** the existing RoleBinding is unchanged and the sign-up reports success

### Requirement: Complete mediation of activation

THE SYSTEM SHALL request activation only after the identity validation and the project-ID syntax validation have passed, and SHALL NOT expose any activation path that bypasses the identity check.

#### Scenario: Verification bypass attempted

- **WHEN** a request reaches the activation step without a validated ID token
- **THEN** no activation is requested and the attempt is rejected

### Requirement: Auditable provisioning without PII in workflow logs

THE SYSTEM SHALL record each activation outcome (email, project ID, namespace, result) in the backend's structured logs, while relying on Cloud Workflows execution records (which do not log call arguments, per the workflow's `LOG_NONE` configuration) for execution history.

#### Scenario: Audit trail for an activation

- **WHEN** a RoleBinding is created for `Alice@example.com` in `acme-corp-platform`
- **THEN** the backend's structured logs contain the email, project ID, namespace, and outcome, and a workflow execution record exists for the invocation
