# Design

## Context

The Cedar authorization model (`gcp-cedar-public-api-authorization` implementation plan) makes a user email the `RoleBinding` subject and expects a Marketplace controller to create the initial `service-admin` binding. The marketplace path is blocked on external dependencies (CIAM identity federation; Partner Integration listing configuration), and the customer's Google email is not available from the Pub/Sub/EGW event pipeline (see `studies/marketplace/gcp-marketplace-pubsub-and-customer-identity.md`). The platform has established infra patterns this design reuses: the shared Gateway deployment pattern (`helm/charts/shared-gateway`, `helm/charts/grafana`) and Cloud Workflows with Kubernetes API access (`terraform/modules/workflows/`).

## Goals / Non-Goals

**Goals:**

- A working end-to-end bootstrap: Google sign-in → ownership proof → `service-admin` RoleBinding, with zero external dependencies (no CIAM, no EGW, no marketplace configuration)
- Identity identical to the production Cedar principal model (verified Google email, same normalization rules)
- Every component reuses a proven infra pattern; deployable to dev/integration environments
- Evolvable: when marketplace lands, replace sign-in and verification sources while keeping provisioning intact

**Non-Goals:**

- Marketplace/EGW/CIAM integration of any kind
- Production customer onboarding (interim scope: dev/integration and pilots; the consent screen is limited to test users until Google app verification)
- Entitlement or billing checks beyond project association
- Serving or administering any platform API other than creating the initial binding
- A user database — the backend is stateless

## Decisions

### D1 — Architecture: SPA + backend in the region cluster behind the shared Gateway, one hostname

Chosen "Option A" from the design discussion. The backend's checks (identity) are a clean seam where the marketplace JWT + EGW correlation later plugs in; the workflow is the stand-in for the Marketplace controller.

Both the SPA and the backend run in the **region GKE cluster behind the shared Gateway** on a single hostname, `signup.<qualifier>.<env>.gcp-hcp.devshift.net`: the HTTPRoute sends `/api/*` to the backend and everything else to the SPA, an unprivileged nginx container that renders its public configuration into a CSP-safe meta tag at container start. The backend runs in the cluster because the org-level custom constraint `custom.allowedPolicyMembers` (terraform/config/org/constraints.tf) forbids IAM members outside the two Red Hat orgs, which blocks the `allUsers` invoker grant that any publicly reachable serverless endpoint (Cloud Run, Cloud Functions, App Engine) requires — the same reason the platform API runs on GKE. Deployment uses the platform's established pattern (Certificate Map entry + HTTPRoute + HealthCheckPolicy + Workload Identity), via a Helm chart and ArgoCD config in the infra repo.

Consequences:

- **One origin.** SPA and API share the hostname, so the browser makes same-origin calls and no CORS is needed in practice. The backend still reflects exactly one configurable origin for a split-origin deployment — never a wildcard, and no credentials mode.
- **No public serverless surface to protect.** The API is reachable only through the Gateway (GKE pods are not directly exposed), which is the same posture as the platform API.
- **Backend identity** is a GSA in the service project, impersonated by the chart's Kubernetes service account through Workload Identity; it holds a project-level `roles/workflows.invoker` on the region project (blast radius accepted and recorded in the security review).

*Alternatives rejected:* Cloud Run behind the external LB (blocked by the org constraint); Cloud Run + IAP (IAP's service agent is also outside the allowed member set); an org-policy exemption (weakens a deliberate, documented control for a throwaway experiment); GCS + Cloud CDN static hosting for the SPA (adds a bucket, extra LB and DNS for a temporary page, and would split the SPA onto a second origin requiring CORS); a gecko signup endpoint (couples the interim to gecko's release cadence and drags in ESPv2 fronting decisions).

### D2 — Identity: Google Identity Services with a dedicated OAuth client

The SPA uses GIS sign-in for an ID token only. It requests no cloud scopes. The backend validates the ID token itself: signature via Google's JWKS, `aud` equal to the configured OAuth client ID, unexpired, `email_verified == true`. It then enforces the **Red Hat-only identity policy**: the Google-issued `hd` claim must equal an allowed domain (initially `redhat.com`) and the normalized email's domain must agree with it — a consumer Google account, a different Workspace domain, or a missing `hd` is rejected before any project lookup. Using `hd` rather than an email-suffix check matters: `hd` is set by Google for Workspace accounts and cannot be spoofed by the client, so it distinguishes "belongs to Red Hat's directory" from "has an address that ends in the right string". The allowlist is configuration. The SPA passes `hd` as an account-chooser hint only — never as a control. Email normalization (NFC, lowercase domain, preserved local-part case) matches the Cedar RoleBinding subject rules exactly.

*Decision:* a **dedicated OAuth client** in the platform service project. The shared client (`32555940559.apps.googleusercontent.com`) is **Google's gcloud CLI client, not a team asset** — its project is Google's, which is why `projects.describe` on it is denied; the value is used in `terraform/modules/service/templates/esp-openapi.yaml.tftpl` purely as `x-google-audiences` so that ESPv2 accepts `gcloud auth print-identity-token` tokens, and the repo itself identifies it as the gcloud CLI client ID (`experiments/auth/phase1-pilot/google-oidc.md`). It therefore cannot host a browser sign-in flow at all: no consent screen, no authorized JavaScript origins, no configurable scopes. (Secondary benefits, which the dedicated client also delivers: verification-risk isolation — a sensitive scope on the platform API's audience could drag the platform's authentication into Google's verification process; least common mechanism — the sign-up backend's `aud` trust decouples from the platform API's.)

The identity does not split: the Cedar principal is the verified email, not the audience, and Google guarantees the email belongs to the authenticated account either way.

*Consent-screen user type:* **External**, with sign-in scopes only. Because `openid email profile` are non-sensitive, no Google verification and no test-user list are required, and the app can be published. Two org constraints shape this: Red Hat's Workspace app-access control blocks third-party apps requesting Google data (the cloud scope returned `access_not_configured` even for test users), and the project lives in `gcp.redhat.com` while identities live in `redhat.com`, so Internal is rejected (`org_internal`). If the Workspace still blocks unconfigured apps that request only sign-in scopes, the app must be added to the Workspace allowlist — an admin action, not a code change.

The backend's `aud` requirement also means platform-API tokens (different client) are rejected at signup, and vice versa.

Note for the eventual design decision (out of scope here): the platform API's current `aud` trust grants acceptance to *any* Google identity with gcloud access — the audience carries no client-identity discrimination; all authorization rests on the Cedar layer.

### D3 — Project ownership is not verified (interim, accepted)

The original design proved owner-class control by calling `projects.testIamPermissions` with the user's OAuth access token for `cloud-platform.read-only`. That is not usable in this org: Red Hat's Workspace app-access control blocks unconfigured third-party apps from touching Google data, so the consent flow fails with `access_not_configured` even for test users, and clearing it requires a Workspace allowlisting review (the Buy@Red Hat path). Google verification would not help either.

*Decision:* the interim verifies only the **identity** (validated ID token plus the Red Hat hosted-domain policy) and the **project ID syntax**. It does not inspect the project at all. Consequences, accepted explicitly for a pilot under 50 internal users:

- Any accepted Red Hat user can activate **any** syntactically valid project ID, including one they hold no role on. A typo or a curious click can therefore claim a namespace that belongs to someone else.
- Because activation is first-subject-wins, a wrong claim needs an operator to delete the binding and re-run — the same recovery path as any mistaken activation.
- The `service-admin` grant is the bootstrap for the Cedar trust model, so this weakens the bootstrap's assurance: the namespace's first administrator is whoever registered the project ID, not necessarily its owner.

Mitigations that remain in place: Red Hat-only identity, project-ID syntax validation, reserved-namespace rejection in the workflow, 409 on a different subject, and audit logging of `(subject, project, namespace, outcome)`.

*Exit options, in order of preference:* (1) prove control by having the user grant the backend's service account read access to the project, then verify their owner binding with the backend's own credentials (no user scope, one manual step); (2) host the client in a project under the `redhat.com` organization so Internal consent works; (3) Workspace allowlisting plus Google verification for the original scope-based check.

### D4 — Provisioning: invoke the existing `activate-project` workflow

The platform already provisions `terraform/workflows/activate-project.yaml` — explicitly documented as "the temporary activation path used before Marketplace activation events are available". It takes `project_namespace` (GCP project ID / Gecko logical namespace) and `subject` (verified email), validates both, rejects reserved namespaces, waits for the `service-admin` PlatformRole to be readable, and creates the fixed `service-admin-rb` binding through gecko's CRD API (`/apis/gcp.managed.openshift.io/v1/.../rolebindings`) with the GKE connector.

Its semantics shape this design:

- **First-subject-wins**: one service-admin per project namespace. A repeat activation by the same subject is idempotent (already-active); a different subject gets a 409. Additional users are managed by the service-admin through the public API after bootstrap — matching the Cedar plan's trust model.
- **Not PAM-gated** (`pam_gated: false` in `terraform/metadata/workflows.yaml`): the backend service account invokes it by being added to the region's `workflows_invoker_members`.
- **`LOG_NONE` call logs**: per-step call logs are suppressed, so the email does not reach Cloud Logging. Note precisely: the execution *resource* still stores its `argument` (including the subject email) and is readable by principals holding `workflows.executions.get` on the region project — IAM-scoped, not log-exposed. The audit trail is therefore backend structured logs (subject, project, namespace, status) correlated by time with execution records.

The backend maps outcomes to sign-up results: `activated` → success, already-active → success, 409 → "project already administered by another user", 403 (reserved namespace) → "reserved project name" (a deliberate, distinct category — not a generic input error), input-validation raises → invalid input, execution failure or timeout → transient. Authentication-stage rejections — bad token, non-Red-Hat `hd`/email domain — are reported as authorization failures and never reach the workflow. Note the workflow's `subject` regex is looser than the backend's email normalization — the backend validates and normalizes first, and its validation must not be relaxed to match the workflow's.

### D5 — Namespace derivation: the project ID itself

Namespace = GCP project ID, matching the workflow's own validation (`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`). Per the `activate-project` header: the project ID is Gecko's *logical* namespace, served by the platform CRD API — it is not a Kubernetes namespace and must not be created as one. Project IDs are globally unique in GCP, giving collision-free, memorable namespaces that match the Cedar plan's examples.

*Alternative rejected:* a prefix like `mp-<project-id>` (isolates the namespace space but adds an unmotivated mapping layer; revisit if namespace squatting becomes a concern).

### D6 — Endpoint hardening and discoverability

The API has **no origin reachable outside the Gateway**: it is a ClusterIP Service with no Ingress of its own, so the shared Gateway's HTTPRoute is the only path in. The hostname itself is public by necessity (the SPA calls it), so rate limiting at the Gateway is a required follow-up before wider exposure. Identity is validated in-app (D2/D3); the only mutating action is the single workflow invocation. The SPA and API communicate over HTTPS across origins, and the backend reflects exactly one configured `Origin` (D1) — preflight is answered only for that origin, and unknown origins get no CORS headers at all.

**Invoker-grant blast radius (explicit):** `workflows_invoker_members` grants project-level `roles/workflows.invoker`, and the PAM-on conditional binding excludes only PAM-*gated* workflows — so the backend SA would also be able to invoke the region's other non-gated workflows (`get`, `describe`, `logs`: cluster reads and raw pod logs). Preferred: scope the backend SA's invoker grant to `activate-project` only, via an IAM condition on the binding (e.g., `resource.name` match or a tag-based condition mirroring the `pam-gated` mechanism). If a scoped condition proves impractical, the broader grant must be explicitly accepted in the security review (task 8.2) as a known read-exposure of a compromised backend — not left implicit.

The SPA origin is exposed as a Terraform output (`signup_url`), computed from the same locals that produce the DNS record and managed certificate, so downstream configuration (OAuth consent screen origins, smoke tests, docs) consumes `terraform output signup_url` rather than transcribing hostnames from plans. The OAuth client's authorized JavaScript origins remain a one-time manual entry per environment — Google's OAuth client configuration for console-created web clients is not Terraform-manageable.

### D7 — Deployment: iterate in the personal dev-all-in-one env, graduate later

Until the design stabilizes and tests pass, everything deploys to the personal dev environment at `terraform/config/dev-all-in-one/cveiga` (local Terraform state, `infra_id = "cveiga"`, region `northamerica-northeast2`, ArgoCD tracking the fork's `dev-cveiga` branch). That config already passes `enable_workflows`, `enable_pam`, and `workflows_invoker_members` through to the region module, so the backend service account's invoker grant is a tfvars change; `activate-project` is not PAM-gated, so the conditional invoker binding covers it even with PAM enabled. New Terraform resources (SPA hosting, DNS, backend identity) are added to the shared `service` module; the API is a Helm chart plus ArgoCD config rendered by the repo's render script. Images are pushed to the dev PR repository and pulled directly in dev (no per-region cache). Graduation to shared environments (integration/stage) moves to the HCP Terraform + `terraform/metadata/` image-pin pattern (per the `diagnose-agent` precedent) once the design is stable.

### D8 — Statelessness

No database: idempotency comes from the workflow's read-first-create with conflict reconciliation (same-subject → already-active; different-subject → 409; create races re-read and apply the same rule), audit from backend structured logs plus workflow execution records, and any needed dedup from the cluster state itself. Nothing about a sign-up needs to survive beyond the binding and the audit trail.

## Risks / Trade-offs

- **Red Hat-only interim** — restricting identities to Red Hat's Workspace domain means this service cannot onboard actual marketplace customers; it is usable only by Red Hat staff for integration testing and pre-marketplace pilots until the marketplace/CIAM path lands. This is a deliberate reversal of the "any Google identity" trust the platform API currently accepts, and it should be the first assumption to revisit if the interim is ever proposed for external use
- **Sensitive-scope consent cap** — the consent screen must be External + Testing because the projects live in `gcp.redhat.com` while user identities live in `redhat.com`, so Internal rejects them (`org_internal`). That caps the interim at 100 test users — accepted, since the pilot is under 50 — shows an unverified-app warning, and makes every user a manual console addition; Google verification (or a client in the `redhat.com` org, or dropping the scope) is required before any wider use
- **Public write endpoint** — the API is public by necessity (the SPA calls it), mediated by in-app token/hosted-domain checks and the single narrow mutation; rate limiting at the Gateway is a follow-up before any wider exposure, and the security review covers it
- **Org policy blocks serverless public endpoints** — `custom.allowedPolicyMembers` forbids `allUsers`, so Cloud Run/Functions/App Engine cannot serve public traffic in this org. The backend therefore lives on GKE; this also means the eventual marketplace controller must not assume a serverless frontend
- **One service-admin per project (first-subject-wins)** — inherited from `activate-project`, and consistent with the Cedar trust model (the service-admin grants further roles via the public API); a typo'd or wrong first activation requires operator intervention to undo (delete the binding and re-run)
- **Platform-project namespaces** — platform GCP project IDs are valid namespace names and not on the workflow's reserved list, and with ownership unverified (D3) nothing closes this vector: any Red Hat identity could claim a platform project's namespace — accepted residual risk for the interim, recorded in the design decision
- **Email in backend logs** — deliberate audit trade-off (the workflow suppresses call arguments via `LOG_NONE`; the binding subject is public via the CRD API anyway); the design decision records the justification, and logs are restricted to the normalized subject with stated retention
- **Interim becoming permanent** — the marketplace transition has a designed landing (swap sign-in/verification sources; keep provisioning — the `activate-project` workflow is already the platform's designated pre-Marketplace path), but this must actually happen; the design decision in `design-decisions/` should record the sunset condition
- **No RH-side identity link** — bindings are Google emails only; if the platform later shifts to RH SSO principals, these bindings need migration (same open question the Cedar plan already carries)
