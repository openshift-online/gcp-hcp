# Tasks

## 1. Scaffolding and spec

- [x] 1.1 Create the experiment README (purpose, architecture diagram, how to run, link to the marketplace study)
- [x] 1.2 Validate the OpenSpec change (`openspec validate add-marketplace-signup-service --strict`) and commit the planning artifacts

## 2. OAuth client (dedicated, platform service project)

- [x] 2.1 Create a dedicated OAuth client in the platform service project for the target environment (dev experiment: `gcp-hcp-dev-svc-<infra_id>`); consent screen with **sign-in scopes only** (`openid email profile`), user type **External** (Internal is rejected: project in `gcp.redhat.com`, identities in `redhat.com`). Console-only by necessity: OAuth clients and consent screens have no Terraform resources, and brand listing via the IAP OAuth Admin API was shut down by Google (March 2026)
- [x] 2.2 Add authorized JavaScript origins for the SPA domain to the consent screen, sourced from the Terraform `signup_url` output (one-time per environment; the OAuth client config itself is not Terraform-manageable)
- [x] 2.3 Record the client ID in the Terraform variables (infra repo) — never a hardcoded fallback

## 3. Frontend SPA

- [x] 3.1 Scaffold the SPA (plain HTML/JS or a minimal framework; no build-time secrets)
- [x] 3.2 Integrate GIS sign-in button and token client (`google.accounts.oauth2.initTokenClient`, `cloud-platform.read-only`); pass `hd` as an account-chooser hint (UX only — the backend enforces the domain restriction)
- [x] 3.3 Implement the project ID form with client-side syntax validation (6–30 chars, `[a-z][a-z0-9-]`, leading letter)
- [x] 3.4 Implement submission to the backend endpoint with outcome feedback (success + namespace; failure categories: auth, non-Red-Hat identity, ownership, invalid input, transient)
- [x] 3.5 Verify no tokens persist in browser storage beyond the transaction; no access token leaves the sign-up request — confirmed: the ID token is held in a closure variable, cleared after submission, and no storage API (localStorage/sessionStorage/cookies) is used anywhere in the SPA

## 4. Backend service (Go)

- [x] 4.1 Scaffold the Go service with `/api/v1/signup`, `/healthz` (ESPv2-compatible liveness shape)
- [x] 4.2 Implement Google ID token validation: JWKS fetch + cache, `aud` check, expiry, `email_verified`
- [x] 4.3 Implement the Red Hat-only identity policy: require `hd` in the configured allowlist (initial `redhat.com`) and agree with the normalized email domain, rejecting before any project lookup; make the allowlist configuration; unit tests for consumer accounts, foreign Workspace domains, and missing/mismatched `hd`
- [x] 4.4 Implement email normalization (NFC, lowercase domain, preserve local-part) with unit tests mirroring the Cedar plan's rules
- [x] 4.5 (superseded) Ownership verification removed: Workspace app-access control blocks the only usable scope (`access_not_configured`). The pipeline now trusts the verified Red Hat identity plus project-ID syntax; the accepted risk and exit options are in design D3
- [x] 4.6 Implement `activate-project` workflow invocation with `{project_namespace, subject}`; map outcomes (`activated` → success, already-active → success, 409 → already-administered, 403 → reserved-name rejection, validation raises → invalid input, execution failure/timeout → transient); no tokens in logs
- [x] 4.7 Configure structured logging — metrics dropped (interim service, not justified for a temporary page; revisit if the service persists beyond Private Preview)
- [x] 4.8 Unit tests for validation, normalization, and error mapping; integration test with fake JWKS/workflow endpoints

## 5. Workflow integration (reuse `activate-project` as-is)

- [x] 5.1 Verify `activate-project` is deployed in the target region environment and confirm its input contract and outcome shapes against `terraform/workflows/activate-project.yaml`
- [x] 5.2 Grant the backend service account workflow-invoker access scoped to `activate-project` only (IAM condition on the binding, e.g. `resource.name` match or a tag-based condition mirroring the `pam-gated` mechanism); if a scoped condition proves impractical, explicitly accept the broader non-gated set (`get`, `describe`, `logs`) in the security review

## 6. Infrastructure (dev-all-in-one env: `terraform/config/dev-all-in-one/cveiga`, infra worktree `.worktree/signup-service`)

- [x] 6.1 Terraform (shared module): private GCS bucket + CDN backend for the SPA (follow `discovery.tf`), global LB with HTTPS-only serving, DNS record under the dev env's domain pattern, `signup_url`/`signup_spa_bucket` outputs, and a root rewrite so `/` serves `index.html` instead of a bucket listing
- [x] 6.2 Helm chart + ArgoCD config for the backend in the region cluster (shared Gateway pattern: ServiceAccount with Workload Identity, Deployment, Service, HTTPRoute, Certificate Map entry, HealthCheckPolicy) — the backend cannot run on Cloud Run: the org constraint `custom.allowedPolicyMembers` blocks the `allUsers` grant an external LB requires
- [x] 6.3 Backend identity in Terraform: GSA in the service project, Workload Identity binding for the chart's KSA, and the region-project `workflows.invoker` grant; OAuth client ID and workflow coordinates are chart values, never hardcoded
- [x] 6.4 Dev env config: `enable_signup`, cluster project, DNS project, host qualifier wired in the `cveiga` config; applied (SPA hosting + identity)
- [x] 6.5 Smoke-test the full flow end to end against the dev env (SPA loads, `/healthz` through the Gateway, non-Red-Hat account rejected, real sign-up with a dev project) — covered by real sign-ups (activated and already-active) plus live checks
- [x] 6.6 Deploy the chart through ArgoCD (chart + app config merged to `dev-cveiga`, rendered for the dev target only; ArgoCD owns the resources, Helm release removed)

## 7. End-to-end verification

- [x] 7.1 E2E: sign-in → `service-admin-rb` RoleBinding exists with the signer's email in the derived logical namespace; verify via the platform CRD API (ownership is not verified — D3) — confirmed via the platform CRD API (e.g. `bmontgom-dev-20261109` and `jimd-customer` namespaces carry `service-admin-rb` with `@redhat.com` subjects)
- [x] 7.2 E2E: repeated sign-up by the same email is idempotent (already-active success); a different email for the same project is rejected with the already-administered category
- [x] 7.3 E2E: wrong-audience token, unverified email, consumer Google account, foreign Workspace domain, reserved project name, and malformed project ID are each rejected with the right failure category — covered by backend unit tests plus live checks (junk token 401, missing token 400, malformed project ID 400); non-Red-Hat account verified live
- [x] 7.4 Verify audit: backend structured logs contain email, project, namespace, outcome; a workflow execution record exists per invocation, correlated by time; `LOG_NONE` keeps the email out of Cloud Logging (it remains on the execution resource, readable only with `workflows.executions.get`)

## 8. Review and documentation

- [x] 8.1 Architect review of code and spec changes (repo AGENTS.md requirement) — round 1 findings addressed; design decision reviewed separately
- [x] 8.2 Security review checklist for the public endpoint: trust boundary, Cloud Armor, rate limits, token handling, invoker-grant blast radius (scoped or explicitly accepted per task 5.2), and the email-in-logs justification (normalized subject only, stated retention) — folded into the design decision's Security section (Cloud Armor and rate limiting accepted as gaps: unadvertised URL, Red Hat identity gate, Private Preview-only audience)
- [x] 8.3 Draft the design decision in `design-decisions/` (interim bootstrap service; sunset condition = marketplace integration replaces sign-in/verification sources) and index it
- ~~8.4 Update the marketplace study with a pointer to this experiment~~ (out of scope for the sign-up flow; the study is tracked separately under GCP-1377)
