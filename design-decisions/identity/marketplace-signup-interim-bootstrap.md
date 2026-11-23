# Interim Sign-Up Service to Bootstrap Customer Project Activation

***Scope***: GCP-HCP

**Date**: 2026-11-17

## Decision

Provide an interim, minimal sign-up service intended for selected internal Red Hat teams participating in the Private Preview. A participant signs in with a Red Hat Google Workspace identity and submits a GCP project ID to trigger the existing `activate-project` Cloud Workflow. The service validates identity but does not verify project ownership. It is retired when the GCP Marketplace Integration via the Entitlement Gateway provides the customer's identity and target GCP project ID.

## Context

- **Problem Statement**: Private Preview participants need the initial `service-admin` RoleBinding in their project's namespace. The GCP Marketplace Integration via the Entitlement Gateway does not yet provide the customer's Google email or target GCP project ID to the activation flow. Without both values, the platform cannot establish the first administrator.
- **Constraints**:
  - Only Red Hat Google Workspace identities (`@redhat.com`, verified via the signed `hd` claim together with email-domain agreement) may pass the gate — not arbitrary Google credentials
  - The service must be reachable from the public internet but is not advertised; the URL is shared directly with Private Preview participants
  - No new workflow or platform code may be introduced — provisioning must reuse `activate-project` unchanged
  - The service is stateless: the sign-in ID token is sent with the single sign-up request and never persisted — there is no session store and no database
  - Every successful activation must attempt a team notification containing the signer, project, and outcome, without adding mail credentials or an external mail provider
- **Assumptions**:
  - Private Preview participants are selected internal Red Hat teams
  - The GCP Marketplace Integration via the Entitlement Gateway will eventually provide the customer's identity and target GCP project ID, making this service redundant

## Alternatives Considered

1. **Wait for the GCP Marketplace Integration (status quo)**: No bootstrap path until entitlement data is available. Blocks all Private Preview onboarding.
2. **Operator-assisted activation**: A GCP HCP operator invokes `activate-project` on behalf of each participant. This works but adds onboarding toil and makes the team the queue for every activation.
3. **Interim sign-up service (chosen)**: A small SPA + API that authenticates a Red Hat Google identity and invokes `activate-project` with the submitted project ID and authenticated email.

## Decision Rationale

* **Justification**: The service removes per-participant operator toil while reusing the existing activation workflow unchanged, adding no data store, and gating activation on a cryptographically verified Red Hat identity. First-subject-wins in `activate-project` prevents a second identity from claiming the same namespace.

* **Evidence**: The service is implemented and live in the `cveiga` dev environment, with successful end-to-end activations (new activations and already-active outcomes) performed with real Red Hat identities. Backend behavior — ID token validation (JWKS signature, issuer, audience, expiry, `email_verified`), `hd` policy enforcement, NFC email normalization, workflow invocation with outcome mapping, and structured audit logging — is covered by tests. See the experiment and OpenSpec change linked under References.

* **Comparison**: Waiting blocks Private Preview onboarding. Operator-assisted activation works but makes every onboarding dependent on the GCP HCP team. The interim service keeps the same activation semantics while allowing participants to initiate activation directly.

## Consequences

### Positive

* Participating teams can initiate the initial `service-admin` grant without GCP HCP operator toil.
* The submitting team member becomes the initial `service-admin`; subsequent delegation uses normal role management.
* The service reuses `activate-project`, is stateless, and holds no application credentials.
* Each successful activation attempts a best-effort team notification.

### Negative

* Project ownership is not verified; first-subject-wins means a mistaken claim requires the operator-run revocation path tracked by [GCP-1503](https://redhat.atlassian.net/browse/GCP-1503).
* Selected-team access is not technically enforced: any verified `@redhat.com` identity that knows the URL can submit a project ID. This broader internal audience is an explicitly accepted risk for the interim Private Preview.
* The authenticated email and project ID appear in IAM-scoped Workflow execution records and backend audit logs; raw tokens are never logged.

## Cross-Cutting Concerns

### Security

* Google ID tokens are fully validated; the signed `hd` claim and email domain must both be `redhat.com`.
* The public surface is the SPA plus `POST /api/v1/signup`. The backend has no secrets; Workload Identity provides access.
* The backend can invoke all non-PAM-gated regional workflows through the platform's shared conditional `roles/workflows.invoker` binding, although it only needs `activate-project`. It also has `roles/integrations.integrationInvoker` for notifications.
* Cloud Armor and application rate limiting are omitted for the small internal preview; audit logs are monitored for anomalous per-subject activity. Both controls are required before expanding the audience.

### Operability

* ArgoCD deploys the service behind the shared regional Gateway.
* `signup-notify` sends best-effort email after activation. Failure is logged, not retried, and does not affect sign-up.

## Sunset Condition

Retire the service when the GCP Marketplace Integration via the Entitlement Gateway provides the customer's identity and target GCP project ID. Remove the ArgoCD application, OAuth client, Terraform identity resources, DNS, and `signup-notify` integration together.

Track and reassess this interim decision through [GCP-1509](https://redhat.atlassian.net/browse/GCP-1509) under [GCP-1032](https://redhat.atlassian.net/browse/GCP-1032) at each Private Preview milestone.

## References

* Experiment: [`experiments/marketplace-signup-service/`](../../experiments/marketplace-signup-service/) (README, backend, frontend)
* OpenSpec change: `add-marketplace-signup-service` (spec deltas and design notes D1–D8, including the ownership-verification exit options)
* Marketplace identity study: `studies/marketplace/gcp-marketplace-pubsub-and-customer-identity.md` (branch `GCP-1377-marketplace-identity-study`)
* Related decisions: [pam-workflow-gating](pam-workflow-gating.md), [zero-operator-access](zero-operator-access.md), [iap-authentication](iap-authentication.md)
