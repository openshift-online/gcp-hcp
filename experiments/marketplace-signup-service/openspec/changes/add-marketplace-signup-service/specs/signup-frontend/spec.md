# Spec Delta

## Purpose

The sign-up frontend: a single-page app where users sign in with Google and submit a GCP project ID, served over HTTPS from the region cluster behind the shared platform Gateway.

## ADDED Requirements

### Requirement: SPA served same-origin behind the shared Gateway

THE SYSTEM SHALL serve the sign-up single-page app from an unprivileged container in the region cluster, at the deployment's `signup` hostname behind the shared external Gateway, on the same origin as the sign-up API — with TLS provided by the Gateway's Certificate Map (Google-managed certificate).

#### Scenario: User loads the sign-up page

- **WHEN** a user navigates to the sign-up hostname
- **THEN** the SPA is served successfully over HTTPS with a Google-managed certificate
- **AND** the serving container runs unprivileged with a read-only root filesystem; its public configuration is injected at container start into a CSP-safe meta tag (no inline script)

#### Scenario: Plain HTTP request

- **WHEN** a request arrives over plain HTTP
- **THEN** it is not served (HTTPS-only; the Gateway exposes no port 80 listener for this hostname)

#### Scenario: Bare hostname request

- **WHEN** a browser navigates to the SPA hostname root
- **THEN** the SPA's `index.html` is served

#### Scenario: Same-origin API call

- **WHEN** the SPA calls `/api/v1/signup`
- **THEN** the request stays on the page's origin and needs no CORS headers; if the deployment is later split across origins, the API still answers only the configured SPA origin

### Requirement: Sign-in with Google and project ID submission

THE SPA SHALL offer Google Identity Services sign-in (returning a Google ID token to the page) and a form for submitting a GCP project ID. The sign-in request SHALL pass an `hd` hint for Red Hat's domain — presentation only; the backend enforces the restriction regardless. The SPA's API base SHALL be deployment configuration, defaulting to the page's own origin (same-origin deployment); a non-empty value is only for a future split-origin deployment.

#### Scenario: Completing the form

- **WHEN** the user is signed in and submits a non-empty project ID
- **THEN** the SPA sends the Google ID token and the project ID to the backend sign-up endpoint over HTTPS
- **AND** no Google access token is persisted in browser storage beyond the sign-up transaction

#### Scenario: Non-Red Hat account signed in

- **WHEN** the backend rejects the submission because the identity is not from an allowed Red Hat domain
- **THEN** the SPA explains that the interim sign-up accepts only Red Hat accounts and does not offer a retry loop

#### Scenario: Client-side input validation

- **WHEN** the user submits a project ID that does not match the GCP project ID syntax (6–30 characters, lowercase letters, digits, hyphens, starting with a letter)
- **THEN** the SPA rejects the input locally with an explanatory message without calling the backend

### Requirement: Submission outcome feedback

THE SPA SHALL present the outcome of a sign-up submission to the user, distinguishing success, validation failure, and transient failure.

#### Scenario: Successful sign-up

- **WHEN** the backend reports the RoleBinding was created
- **THEN** the SPA shows a success message identifying the namespace that was granted

#### Scenario: Repeat sign-up by the same user

- **WHEN** the backend reports the binding already existed for the same subject (`already_active`)
- **THEN** the SPA says the namespace already had access for that account and that nothing was changed

#### Scenario: Already signed in

- **WHEN** the user completes Google sign-in
- **THEN** the sign-in button is replaced by their identity and a way to switch accounts

#### Scenario: Failed sign-up

- **WHEN** the backend reports an authentication failure, invalid input, a reserved project name, a transient failure, or that the project is already administered by a different user
- **THEN** the SPA shows a failure message with the reason category and does not retry automatically

#### Scenario: Outcomes include next steps

- **WHEN** an outcome is shown (success or failure)
- **THEN** it states what happened, whether anything was changed, whether the user can close the window, and what they can do next — for success, that they may now create RoleBindings for other users through the platform API, with an example `gcphcpctl rolebinding create` command referencing the project's namespace and a link to the gcp-hcp-ctl CLI; for failures, when to retry and when to contact the platform team
