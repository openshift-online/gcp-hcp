# Platform Secret Management: Store Directly in Target System, Bitwarden for TOTP Only

***Scope***: GCP-HCP

**Date**: 2026-09-25

## Decision

We store each platform secret directly in its target system — primarily Google Cloud Secret Manager, delivered to GKE clusters via External Secrets Operator. As part of this decision, we maintain a central secret inventory in git (`gcp-hcp-infra/secrets/inventory.yaml`) recording each secret's type, target location, provisioning method, consumers, producers, and runbook — metadata only, never the secret value itself. Bitwarden is used only for secrets that require TOTP (shared accounts needing 2FA, e.g. GitHub bot accounts). Human access to secrets stored in target systems is granted through IAM, using time-bound Privileged Access Manager (PAM) grants where just-in-time elevated access is warranted.

## Context

The GCP HCP platform requires a consistent, secure approach to managing secrets across multiple environments and GKE clusters. While Workload Identity eliminates most credential needs, some secrets remain unavoidable — GitHub app keys, external API tokens, SSH keys, and certificates for services that do not support federated identity.

- **Problem Statement**: No unified strategy exists for storing, distributing, and rotating mandatory secrets across the platform. Current practices are ad-hoc, with secrets scattered across Google Secret Manager projects and no single authoritative source. This decision covers platform-level secrets only — secrets used to operate the GCP HCP infrastructure itself. Customer-facing secrets (e.g., OIDC signing keys, customer-provided credentials) belong to a separate security domain and will be addressed in a dedicated decision document.
- **Constraints**: Must comply with Red Hat security policies; must support team-wide shared access with auditing; must integrate with GKE workloads without granting overly broad permissions; must support 2FA token management for shared service accounts.
- **Assumptions**: Workload Identity covers the majority of authentication needs (see [workload-identity-implementation](workload-identity-implementation.md)). The remaining mandatory secrets are relatively few and change infrequently.
- **Note**: Prow/CI-managed secrets used by OpenShift CI jobs also live in Google Secret Manager, but are accessed via the CI team's [`sm` secret-manager tool](https://docs.ci.openshift.org/architecture/cli-secret-manager/) rather than IAM+PAM — a different access mechanism for the same storage backend, which is fine given the CI team's tool already provides its own controlled access.

## Alternatives Considered

1. **Store directly in target system; Bitwarden for TOTP only (chosen)**: Each secret lives in the system that consumes it (mostly Secret Manager, via ESO into GKE). Bitwarden is reserved for secrets that inherently require a shared, human-usable TOTP (e.g. shared GitHub/HashiCorp accounts). Human access to target-system secrets is via IAM, elevated through PAM grants when needed.
2. **Bitwarden as root of trust for all secrets**: Store every mandatory secret's authoritative copy in Bitwarden, duplicated into Secret Manager for workload consumption. Rejected: duplicates every secret across two stores and doubles the attack surface.
3. **HashiCorp Vault (ACCE Vault)**: Enterprise-grade secret management already available at Red Hat. Rejected — introduces operational overhead (dedicated infrastructure, expertise, maintenance) disproportionate to our secret volume.
4. **Sealed Secrets in Git**: Encrypt secrets into Git using Bitnami Sealed Secrets. Rejected — creates rotation friction and couples secret lifecycle to Git workflows.

## Decision Rationale

* **Justification**: Maintaining a central inventory gives the discoverability a Bitwarden-first workflow would otherwise be needed for — anyone can look up where a given secret is stored, how it's provisioned, and who consumes it without it also needing to exist in Bitwarden. Storing secrets directly in their target system avoids a redundant copy in Bitwarden. PAM is established on this platform for time-bound, approval-gated access (see [pam-workflow-gating](pam-workflow-gating.md)); extending the same pattern to secret access is consistent with existing practice rather than a new mechanism.
* **Evidence**: Requiring two copies of every secret would increase the number of places a leak could originate, with no added benefit once a central inventory provides discoverability on its own.
* **Comparison**: Vault adds operational complexity disproportionate to our secret volume. Sealed Secrets create rotation friction and couple secrets to Git workflows. A full Bitwarden duplicate is unnecessary once a central inventory provides discoverability directly.

## Consequences

### Positive

* No duplicated secrets between Bitwarden and Secret Manager — one authoritative copy per secret, in its target system
* Smaller attack surface: fewer places a given secret's value can leak from
* `secrets/inventory.yaml` is the single, always-current source for where a secret lives, how it's provisioned, and who consumes/produces it
* Access to target-system secrets follows the same auditable, time-bound PAM pattern already used elsewhere on the platform
* TOTP-dependent shared accounts keep the workflow they actually need (Bitwarden's built-in TOTP generation), without over-applying it to secrets that don't require it

### Negative

* No single human-browsable UI across all secrets — engineers use `secrets/inventory.yaml` plus each target system's own console/CLI
* Emergency/break-glass read access depends on PAM and target-system access rather than a Bitwarden fallback
* Secrets not yet reflected in the inventory need to be reconciled as they come up for rotation

## Cross-Cutting Concerns

### Security:

* Each secret's authoritative location is recorded in `secrets/inventory.yaml` (`secret_manager` or `bitwarden` block); this is the reference for where a given secret should be created, read, and rotated
* Bitwarden is used only for secrets whose `type` requires a shared TOTP-protected account (e.g. `github-account`, `hashicorp-account`); Bitwarden access is governed by Rover group `gcp-hcp-eng` (Note `@BW@`), collection "Gcp Hcp Eng"
* Google Cloud Secret Manager provides encryption at rest, IAM-based access control, and Cloud Audit Logs for all secrets stored there
* Human access to secrets in Secret Manager is granted via IAM, elevated through PAM entitlements for just-in-time access rather than standing bindings, consistent with [pam-workflow-gating](pam-workflow-gating.md). This grants direct IAM access to the target system (Console/`gcloud`) for provisioning and rotation, distinct from the Cloud Workflows-mediated PAM pattern in [zero-operator-access](zero-operator-access.md), which governs production operational actions
* ESO SecretStores authenticate to Secret Manager via GKE Workload Identity Federation — `roles/secretmanager.secretAccessor` is granted directly to the Kubernetes service account principal, with no intermediate Google service account. Static service account JSON keys (`secretAccessKeySecretRef`) are prohibited
* ClusterSecretStores are avoided to prevent cross-namespace secret leakage. Creation of `ExternalSecret` resources should be restricted via RBAC to prevent unauthorized namespaces from referencing a SecretStore's identity
* Independently authorized values must be stored as separate Secret Manager secrets, since IAM bindings apply at the secret level, not at individual payload fields within a secret

### Reliability:

* **Scalability**: Secret Manager scales independently per project; ESO handles reconciliation per namespace
* **Observability**: Cloud Audit Logs for Secret Manager access and PAM grant activity; ESO metrics and events for sync status; Bitwarden access logs via Red Hat IT for TOTP-only secrets
* **Resiliency**: Secret Manager secrets use automatic replication (multi-region) by default. Secrets with data residency requirements (if any) must use user-managed replication with explicitly approved locations, consistent with the [regional-independence architecture](../infrastructure/regional-independence-architecture.md). ESO reconciles on failure. Recovery from a Secret Manager outage or lost secret is to rotate and re-store the value at its source (GitHub App, OAuth provider, etc.) rather than restore a cached copy — these secrets are cheap to rotate, and the platform is already fully committed to GCP availability for everything else

### Cost:

* Google Cloud Secret Manager: per-secret and per-access-operation pricing, negligible at our scale
* External Secrets Operator: open-source, runs as a lightweight controller on existing GKE clusters
* Bitwarden: managed by Red Hat IT, no direct cost to team, scoped to TOTP-dependent secrets only

### Operability:

* New secrets follow the inventory: add an entry to `secrets/inventory.yaml` describing type, target system, provisioning method, and consumers, then create the secret directly in that target system (Secret Manager via Terraform/manual, or Bitwarden only if TOTP is required)
* ESO SecretStore per namespace with dedicated service account reduces blast radius of misconfigurations
* Team onboarding requires IAM/PAM access for target systems, and Rover group membership for Bitwarden only where TOTP-dependent secrets are involved

---

## Template Validation Checklist

### Structure Completeness
- [x] Title is descriptive and action-oriented
- [x] Scope is GCP-HCP
- [x] Date is present and in ISO format (YYYY-MM-DD)
- [x] All core sections are present: Decision, Context, Alternatives Considered, Decision Rationale, Consequences
- [x] Both positive and negative consequences are listed

### Content Quality
- [x] Decision statement is clear and unambiguous
- [x] Problem statement articulates the "why"
- [x] Constraints and assumptions are explicitly documented
- [x] Rationale includes justification, evidence, and comparison
- [x] Consequences are specific and actionable
- [x] Trade-offs are honestly assessed

### Cross-Cutting Concerns
- [x] Each included concern has concrete details (not just placeholders)
- [x] Irrelevant sections have been removed
- [x] Security implications are considered where applicable
- [x] Cost impact is evaluated where applicable

### Best Practices
- [x] Document is written in clear, accessible language
- [x] Technical terms are used appropriately
- [x] Document provides sufficient detail for future reference
- [x] All placeholder text has been replaced
- [x] Links to related documentation are included where relevant
