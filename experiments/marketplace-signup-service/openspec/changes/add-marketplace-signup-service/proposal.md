# Proposal

## Why

The GCP-HCP Cedar authorization model defines the initial `service-admin` RoleBinding (subject = user email) as the bootstrap grant for a customer namespace, created by the future Marketplace controller. The marketplace integration is blocked on external dependencies (CIAM identity federation, Partner Integration listing configuration), but the platform needs a working end-to-end bootstrap path now — for integration testing of the authorization model and for pre-marketplace pilots.

The customer's Google email cannot come from the marketplace event pipeline (Google anonymizes customers in Pub/Sub and the Procurement API — see the marketplace study). The only Google-native source today is Google Identity Services sign-in, which yields the same verified email the Cedar model uses as its principal.

## What Changes

Introduce a new interim sign-up service, self-contained under `experiments/marketplace-signup-service/`:

1. A single-page app (served by an unprivileged nginx container in the region cluster, on the same hostname as the API) where a user signs in with their Red Hat Google Workspace identity and submits a GCP project ID.
2. A backend (in the region cluster) that validates the Google ID token and rejects anything outside Red Hat's hosted domain, and triggers activation. Project ownership is deliberately not verified (design D3); eligibility is the verified Red Hat identity plus project-ID syntax validation.
3. Invocation of the existing `activate-project` Cloud Workflow — the platform's documented pre-Marketplace activation path — with the verified email and project ID; the workflow creates the `service-admin` RoleBinding through gecko's CRD API.

**Scope constraint (deliberate):** only Red Hat identities are accepted (`hd == redhat.com`, enforced server-side against the Google-issued hosted-domain claim). The service therefore cannot onboard external marketplace customers — it serves Red Hat staff for integration testing and pre-marketplace pilots until the marketplace/CIAM path exists. Relaxing this policy later means: external OAuth consent (verification, test-user cap), and re-examining whether Google identity alone is sufficient proof of entitlement once marketplace data is available.

Deployment changes (Helm chart/ArgoCD app, backend identity and workflow IAM) are made in the gcp-hcp-infra repo. No changes to gecko, EGW, or any Partner Integration component.

## Capabilities

### New Capabilities

- `signup-frontend`: the sign-up single-page app, its form, and its same-origin serving next to the API
- `signup-authentication`: Google Identity Services sign-in and server-side validation of the resulting Google ID token
- `project-eligibility`: the signing user must be a verified Red Hat identity; project ownership is explicitly not verified in the interim (design D3)
- `rolebinding-provisioning`: creation of the initial `service-admin` RoleBinding in the user's namespace, via a Cloud Workflow

### Modified Capabilities

(none)

## Impact

- **New code**: `experiments/marketplace-signup-service/` in the gcp-hcp repo (SPA, Go backend, OpenSpec artifacts; a design decision will be added to `design-decisions/` in the same branch when the design stabilizes)
- **Infrastructure**: gcp-hcp-infra repo — a Helm chart + ArgoCD config running the SPA (nginx) and backend in the region cluster behind the shared Gateway on one hostname (external-dns and Certificate Manager provide DNS/TLS), plus Terraform for the backend GSA with a PAM-conditioned invoker grant (the existing `activate-project` workflow is reused as-is). Neither Cloud Run (the org constraint `custom.allowedPolicyMembers` forbids the `allUsers` grant an external LB needs) nor a CDN-hosted SPA (Cloud CDN cannot forward the Cookie header its GCS signature covers) is usable in this org
- **Identity**: a dedicated OAuth client in the platform service project (the current platform API audience is Google's gcloud CLI client, which cannot host a browser flow), requesting **sign-in scopes only** (`openid email profile`) and configured with the SPA origin
- **Scope**: Red Hat identities only (`hd == redhat.com`); external customers are explicitly out of scope for the interim
- **No impact**: gecko, EGW, Partner Integration systems, CIAM/RH SSO, Google Marketplace/Producer Portal configuration
