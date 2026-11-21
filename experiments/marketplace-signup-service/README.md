# Marketplace Sign-Up Service (Interim)

Interim self-service bootstrap for GCP-HCP: a user signs in with their **Red Hat
Google Workspace identity**, submits a GCP project ID, and receives the initial
`service-admin` RoleBinding in that project's namespace — standing in
for the future Google Cloud Marketplace controller until the marketplace
integration (CIAM identity federation, EGW entitlement correlation) is available.

## Status

**Experiment — not production.** This lives under `experiments/` to validate the
authorization-model bootstrap end to end and to support pre-marketplace pilots.
Nothing here is wired into the platform API's authentication path.

| Area | State |
| --- | --- |
| Spec (OpenSpec change `add-marketplace-signup-service`) | Complete, validated `--strict` |
| Frontend SPA | Implemented; served by nginx from the region cluster on the same hostname as the API |
| Go backend | Implemented and unit-tested (ID token + issuer, hosted-domain policy, workflow invocation, CORS); ownership verification removed by constraint (design D3) |
| Infrastructure (gcp-hcp-infra) | Helm chart + ArgoCD app; backend GSA + Workload Identity applied in dev |
| Dedicated OAuth client | Manual console step, per environment; External user type, sign-in scopes only (no verification, no test-user list) |
| Deployment target | `terraform/config/dev-all-in-one/cveiga` (Terraform) + region cluster (Helm) |

## Scope constraint: Red Hat identities only

The backend accepts only identities whose Google signed hosted-domain claim
(`hd`) is `redhat.com`, and whose email domain agrees with it. Consequences:

- The service **cannot onboard actual marketplace customers.** It serves Red Hat
  staff for integration testing and pilots.
- The OAuth consent screen uses the **External** user type with sign-in scopes
  only (`openid email profile`): no app-verification and no test-user cap are
  needed at that scope. The Internal user type is **not** usable here — the
  consent screen's project lives in the `gcp.redhat.com` organization while the
  identities belong to the separate `redhat.com` organization, so Google rejects
  internal-only audiences with `org_internal`.
- Relaxing the policy later is a deliberate change: external OAuth consent
  (verification, test-user cap) plus a re-think of whether Google identity alone
  is sufficient proof of entitlement once marketplace data exists.

## Why this exists

The Cedar authorization model defines the initial `service-admin` RoleBinding
(subject = verified user email) as the bootstrap grant for a customer namespace.
That grant is supposed to be created by the Marketplace controller when Google
sends an activation event — but the marketplace event pipeline (Pub/Sub via EGW,
Procurement API) **never carries the customer's email**; Google anonymizes
customers there. The only Google-native source of a verified email today is a
frontend sign-in, which the marketplace flow blocks on CIAM federation. Details:
[`studies/marketplace/gcp-marketplace-pubsub-and-customer-identity.md`](../../../studies/marketplace/gcp-marketplace-pubsub-and-customer-identity.md)
(on the `GCP-1377-marketplace-identity-study` branch until merged).

## Architecture

Both the SPA and the API run in the **region GKE cluster**, behind the shared
Gateway, on **one hostname**. The SPA is an unprivileged nginx container; the
API is a Go deployment. The HTTPRoute sends `/api/*` to the backend and
everything else to the SPA.

```
  Browser (Red Hat Workspace identity)
      │
      ├── 1. GET /            ─┐
      └── 2. POST /api/v1/signup ─┐
                                   ▼
                 Shared Gateway (GKE)  signup.<qualifier>.<env>.gcp-hcp.devshift.net
                                   │
                     ┌─────────────┴──────────────┐
                     ▼                            ▼
              signup-spa (nginx)          signup-backend (Go)
              index.html, app.js          validate ID token + issuer + aud
              config in a meta tag        enforce hd=redhat.com
                                          invoke activate-project
                                   │
                                   ▼
                 Cloud Workflow activate-project (region project)
                                   │
                                   ▼
                 gecko CRD API: service-admin-rb in namespace = project ID
```

### Request flow

1. The user loads the SPA; config (client ID) is injected at container start.
2. The SPA obtains a Google **ID token** via GIS sign-in.
3. It `POST`s `{project_id, id_token}` to the same origin.
4. The backend validates the ID token (JWKS signature, issuer, `aud`, expiry,
   `email_verified`) and enforces the Red Hat hosted-domain policy.
5. The backend invokes the existing `activate-project` workflow with
   `{project_namespace: <project id>, subject: <normalized email>}`.
6. The workflow creates `service-admin-rb` via gecko (idempotent; rejects a
   different subject with 409, reserved namespaces with 403).

> **Interim limitation — ownership is not verified.** Google Workspace
> app-access control blocks the `cloud-platform.read-only` scope in this org
> (`access_not_configured`), so the backend cannot read a project's IAM policy as
> the user. Any accepted Red Hat user can therefore activate any syntactically
> valid project ID, and first-subject-wins means a mistaken claim needs an
> operator to undo. Accepted for a sub-50 internal pilot; see design D3.

### What the backend never does

- Never persists tokens (no database; the ID token lives for one request).
- Never writes tokens to logs. The RoleBinding subject email **is** logged
  deliberately (audit), justified in the design.
- Never trusts client-supplied identity: email comes from the signed ID token.
- Never inspects the customer's project — that check was removed (see above).

## Repository layout

```
experiments/marketplace-signup-service/
├── README.md                  # this file
├── openspec/                  # spec-driven change (proposal, design, specs, tasks)
│   ├── project.md
│   ├── changes/add-marketplace-signup-service/
│   └── specs/                 # archived/current specs (empty until archive)
├── backend/                   # Go service (container image)
│   ├── go.mod
│   ├── cmd/server/
│   └── internal/
└── frontend/                  # static SPA (served by an unprivileged nginx container)
    ├── index.html
    └── app.js
```

The backend's cloud identity (GSA, Workload Identity, workflow invoker grant)
and the Kubernetes chart live in the
[gcp-hcp-infra](https://github.com/openshift-online/gcp-hcp-infra) repo.

## Local development

The backend can serve the SPA from disk, keeping the same-origin shape as
production (no CORS):

```bash
cd backend
go test ./...
OAUTH_CLIENT_ID=<client-id>.apps.googleusercontent.com \
WORKFLOW_PROJECT_ID=<platform-project> \
WORKFLOW_REGION=northamerica-northeast2 \
FRONTEND_DIR=../frontend \
  go run ./cmd/server            # http://localhost:8080
```

For local sign-in, set `window.SIGNUP_CONFIG` in the browser console (client
ID, `apiBase: ""`), and add the local origin to the OAuth client's authorized
JavaScript origins (e.g. `http://localhost:8080`).

### Backend configuration

| Variable | Purpose |
| --- | --- |
| `OAUTH_CLIENT_ID` | Dedicated OAuth client ID; ID tokens must carry it as `aud` (required) |
| `ALLOWED_EMAIL_DOMAINS` | Hosted-domain allowlist (default `redhat.com`) |
| `ALLOWED_ORIGIN` | Kept for split-origin deployments; empty means same-origin only (the current deployment) |
| `WORKFLOW_PROJECT_ID` / `WORKFLOW_REGION` | Where `activate-project` runs (required) |
| `WORKFLOW_NAME` | Workflow to invoke (default `activate-project`) |
| `FRONTEND_DIR` | Serve the SPA from disk at the same origin (local dev only) |
| `PORT` | Listen port (default `8080`) |

No secrets are needed: token validation uses Google's public JWKS, and workflow
invocation uses the pod's Workload Identity service account.

## Deployment

1. Create the dedicated OAuth client in the environment's service project
   (console-only; OAuth clients and consent screens have no Terraform resources).
   Consent screen: user type **External**, scopes `openid`, `email`, `profile`
   only, and the SPA origin as an authorized JavaScript origin. No Google
   verification or test-user list is needed for sign-in scopes — but the Red Hat
   Workspace must allow the app (see design D3/D2).
2. Apply Terraform in gcp-hcp-infra (dev: `terraform/config/dev-all-in-one/cveiga`)
   — backend GSA + Workload Identity and the workflow invoker grant.
3. Build and push the backend image. Deployment is ArgoCD-managed everywhere:
   the chart (`helm/charts/marketplace-signup`) and app config
   (`argocd/config/region/marketplace-signup`) render per target, with the OAuth
   client ID, SPA origin, and workflow coordinates in the values.
4. Build and push the SPA image (`frontend/Dockerfile`), which the chart deploys
   next to the backend. Public configuration is injected at container start
   into the page's `signup-config` meta tag; the SPA and API share one
   hostname, so there is no CORS.
5. Smoke-test: SPA loads, `/healthz` answers through the Gateway, and a
   non-Red-Hat account is rejected.

## Specs

Spec-driven development uses [OpenSpec](https://github.com/Fission-AI/OpenSpec).
The active change is `add-marketplace-signup-service`:

```bash
npx -y @fission-ai/openspec@latest validate add-marketplace-signup-service --strict
npx -y @fission-ai/openspec@latest show add-marketplace-signup-service
```

Capabilities: `signup-frontend`, `signup-authentication`,
`project-eligibility`, `rolebinding-provisioning`. When the marketplace
integration lands, its change is expected to modify the first two and the third,
while leaving `rolebinding-provisioning` largely intact.

## Out of scope

- External customer onboarding (see the Red Hat-only constraint above)
- Anything in the marketplace/EGW/Pub/Sub pipeline
- CIAM/RH SSO federation
- Replacing or modifying the `activate-project` workflow
- Multi-user namespace administration (first-subject-wins; further grants go
  through the platform API afterward)
