# Interim Sign-Up Service to Bootstrap Customer Project Activation

***Scope***: GCP-HCP

**Date**: 2026-11-17

## Decision

Provide an interim, minimal sign-up service where a subscribing customer signs in with a Red Hat Google Workspace identity and submits their GCP project ID to trigger the existing `activate-project` Cloud Workflow. The service performs identity validation only — it does not verify project ownership. It exists until the GCP Marketplace Pub/Sub integration can supply the subscribing customer's identity, at which point it is retired.

## Context

- **Problem Statement**: Customers who purchase the offering through GCP Marketplace (Private Preview) need the initial `service-admin` RoleBinding in their project's namespace before anyone can act on their behalf. The marketplace integration that would deliver the subscribing customer's Google email is not yet available, and the Pub/Sub events exposed by the entitlement/EGW flow do not carry it. Without an identity source, there is no way to establish the first administrator.
- **Constraints**:
  - Only Red Hat Google Workspace identities (`@redhat.com`, verified via the signed `hd` claim together with email-domain agreement) may pass the gate — not arbitrary Google credentials
  - The service must be reachable from the public internet (customers are external to the platform network) but is not advertised; the URL is shared directly with Private Preview participants
  - No new workflow or platform code may be introduced — provisioning must reuse `activate-project` unchanged
  - The service is stateless: the sign-in ID token is sent with the single sign-up request and never persisted — there is no session store and no database
- **Assumptions**:
  - Private Preview participants are internal Red Hat users acting on behalf of their customer, or customers with Red Hat Google identities; the population is small and known
  - GCP Marketplace will eventually provide the subscribing customer's identity (see the marketplace identity study), making this service redundant
  - The OAuth consent screen stays in External mode with sign-in scopes only (`openid email profile`); the platform's GCP org and the identity org are separate Google organizations

## Alternatives Considered

1. **Wait for the marketplace integration (status quo)**: No bootstrap path until CIAM/entitlement data is available. Blocks all Private Preview onboarding.
2. **Manual RoleBinding by an operator**: An operator creates the first `service-admin` binding on request. Works, but reintroduces manual access operations, contradicts the Zero Operator Access direction, and leaves no self-service audit trail tied to a verified identity.
3. **Scope-based ownership verification**: Require a read-only scope (`cloud-platform.read-only`) and verify the customer's IAM access to the submitted project before activation. Restores ownership proof, but Workspace application-access control blocks the scope for Red Hat identities, and it adds a dependency the interim service does not need at Private Preview scale.
4. **Interim sign-up service (chosen)**: A small SPA + API that authenticates a Red Hat Google identity and invokes `activate-project` with the submitted project ID and the authenticated email.

## Decision Rationale

* **Justification**: The chosen alternative is the only one that unblocks Private Preview onboarding today while preserving the platform's security posture: it reuses the existing activation workflow unchanged, introduces no new data stores, and gates every activation behind a cryptographically verified Red Hat identity. First-subject-wins in `activate-project` already prevents a second, different identity from claiming the same project namespace.

* **Evidence**: The service is implemented and live in the `cveiga` dev environment, with successful end-to-end activations (new activations and already-active outcomes) performed with real Red Hat identities. Backend behavior — ID token validation (JWKS signature, issuer, audience, expiry, `email_verified`), `hd` policy enforcement, NFC email normalization, workflow invocation with outcome mapping, and structured audit logging — is covered by tests. See the experiment and OpenSpec change linked under References.

* **Comparison**: The status quo was rejected because it blocks all Private Preview onboarding on an integration with no delivery date. Manual RoleBinding was rejected because it reintroduces operator access for a routine, verifiable action and leaves no audit trail tied to a verified identity. Scope-based ownership verification was rejected because Workspace application-access control blocks the required scope for Red Hat identities, and the ownership gap is tolerable at Private Preview scale — but it remains the primary exit option if that changes.

## Consequences

### Positive

* Private Preview customers can self-serve the initial `service-admin` RoleBinding with no operator involvement
* The team is notified of every activation (who, which project, outcome) with zero added credentials or services — closing the visibility gap raised at the November office hours
* The provisioning path is identical to the future automated one (`activate-project`), so no migration of activation semantics is needed when marketplace lands
* The identity gate is enforced by Google's signed claims, not by an application-side email allowlist
* Zero new persistent state: removing the service removes the service's own attack surface (the RoleBindings it created, the OAuth client, and the DNS hostname are enumerated as sunset cleanup below)

### Negative

* **Project ownership is not verified**: any Red Hat identity can claim any not-yet-claimed project ID. First-subject-wins bounds this to unclaimed namespaces, and the identity gate bounds it to Red Hat users, but a mistaken or curious claim can lock a namespace to the wrong subject. Recovery currently requires an operator — a known, accepted tension with [zero-operator-access](zero-operator-access.md) at interim scale; the durable fix is a gated correction workflow (PAM-approved), not standing operator access
* One `service-admin` per namespace: the first claimant is the bootstrap admin; delegation to the actual customer identity happens later through normal role management
* The sign-in identity is a Google identity, not Red Hat SSO — customer-facing equivalence for non-Red-Hat end users is out of scope for the interim
* The authenticated email appears in Workflow execution records (`workflows.executions.get` is IAM-scoped) and, as the normalized subject, in the backend's Cloud Logging audit entries (raw tokens are never logged)

## Cross-Cutting Concerns

### Security:

* **Authentication**: Google-issued ID tokens validated in full (JWKS signature, `iss`, `aud`, expiry, `email_verified`); the Red Hat-only policy is enforced via the signed `hd` claim, with the allowlist held as configuration
* **Trust boundary**: The public surface is static SPA files plus a single validated endpoint (`POST /api/v1/signup`); the backend holds no secrets — the OAuth client is public, and Workload Identity supplies workflow invocation
* **Invoker blast radius**: The backend's service account is limited to `roles/workflows.invoker` with the PAM-gated tag condition (per the [PAM workflow gating](pam-workflow-gating.md) decision) plus `roles/integrations.integrationInvoker` for the activation notification — the platform's narrowest integration role, allowing execution, list/get, and `executions.cancel`/`replay` on the region project's integrations (no creating or editing them); it cannot invoke non-gated workflows beyond that binding
* **Accepted gaps (interim scope)**: No Cloud Armor edge policy and no application rate limiting. Accepted because every submission requires a verified Red Hat identity and the audience is bounded to internal Private Preview participants — obscurity of the URL is a convenience, not a control. The residual risk is a single curious or compromised Red Hat identity invoking `activate-project` in a tight loop (execution cost, claims across unclaimed project IDs); the compensating control is monitoring the activation audit log for anomalous per-subject volume. Both Cloud Armor and rate limiting become prerequisites before any broader or customer-public exposure
* **Data handling**: The sign-in ID token is sent with the single sign-up request and never persisted by the service. The backend's structured audit log records the normalized subject (the verified email — the same value used as the RoleBinding subject), plus project, namespace, and outcome; raw tokens are never logged. Cloud Workflows suppresses call arguments in its logs (`LOG_NONE`), though the execution resource retains the argument (IAM-scoped)

### Operability:

* Deployed as a Helm chart via ArgoCD in the region cluster (dev target only), behind the shared external Gateway with a Certificate Map entry; no dedicated infrastructure beyond the backend's service account and Workload Identity binding
* **Activation notification**: each successful sign-up sends exactly one email (signer, project, status) by executing the `signup-notify` Application Integration in the region project — an API trigger and a single Send Email task delivering through Google's mailer, with no credentials anywhere in the chain and a custom subject rather than alert-template wording. The backend calls it fire-and-forget after activation succeeds, so a notification failure logs a warning and never affects the sign-up. The integration's definition is checked into the infra repo and published by Terraform; the backend's service account holds the platform's narrowest integration role on the region project (which still includes execution cancel/replay)
* Removal is a single ArgoCD application plus the Terraform identity resources and the `signup-notify` integration

## Sunset Condition

The service is retired when the GCP Marketplace Pub/Sub integration (or a successor CIAM source) supplies the subscribing customer's identity for activation events. At that point sign-in-based bootstrap and its ownership gap are both replaced by the marketplace's entitlement data; the service, its OAuth client, and its Terraform identity should be deleted in the same change.

The decision is tracked by GCP-1503 (operator correction path) under epic GCP-1032 and should be re-evaluated at each Private Preview milestone — an interim decision with no revisit trigger tends to become permanent.

If stronger ownership proof is needed before then, the recorded exit options are: (a) user-grant-based proof (customer grants the service's service account a role in their project), (b) an OAuth client owned by the `redhat.com` organization, or (c) Workspace allowlisting to unblock scope-based verification.

## References

* Experiment: [`experiments/marketplace-signup-service/`](../../experiments/marketplace-signup-service/) (README, backend, frontend)
* OpenSpec change: `add-marketplace-signup-service` (spec deltas and design notes D1–D8, including the ownership-verification exit options)
* Marketplace identity study: `studies/marketplace/gcp-marketplace-pubsub-and-customer-identity.md` (branch `GCP-1377-marketplace-identity-study`)
* Related decisions: [pam-workflow-gating](pam-workflow-gating.md), [zero-operator-access](zero-operator-access.md), [iap-authentication](iap-authentication.md)
