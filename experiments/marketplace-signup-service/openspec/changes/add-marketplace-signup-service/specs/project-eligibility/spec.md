# Spec Delta

## Purpose

Eligibility to activate a project namespace: the signing user must be a verified
Red Hat identity. Project ownership is deliberately **not** verified in this
interim — an accepted limitation forced by Google Workspace app-access control,
recorded in design D3.

## ADDED Requirements

### Requirement: Verified Red Hat identity is the eligibility gate

THE SYSTEM SHALL gate namespace activation on the validated Google ID token and the Red Hat hosted-domain policy, and SHALL NOT require a user OAuth access token or call Google APIs with the user's credentials.

#### Scenario: Verified Red Hat identity

- **WHEN** the ID token validates and its `hd` claim is in the allowlist
- **THEN** the sign-up proceeds to activate the submitted project namespace

#### Scenario: Non-Red-Hat identity

- **WHEN** the ID token's `hd` is missing or outside the allowlist
- **THEN** the request is rejected before any activation

#### Scenario: No user credentials are used

- **WHEN** a sign-up is processed
- **THEN** the backend makes no call to Resource Manager with the user's token, because no such token is collected

### Requirement: Unverified ownership is an explicit, recorded limitation

THE SYSTEM SHALL document that any accepted user may activate any syntactically valid project ID, that the first activation wins for that namespace, and that reversing a mistaken or malicious claim requires an operator.

#### Scenario: User submits a project they do not own

- **WHEN** an accepted Red Hat user submits a project ID they hold no role on
- **THEN** activation is attempted and succeeds if the namespace is unclaimed — this is a known, accepted behaviour of the interim, not a defect

#### Scenario: Namespace already claimed by another user

- **WHEN** the namespace's `service-admin-rb` already exists for a different subject
- **THEN** the workflow rejects the request (409) and the existing binding is left untouched
