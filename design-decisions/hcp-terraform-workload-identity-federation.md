# HCP Terraform Must Use Workload Identity Federation with Dynamic Provider Credentials

***Scope***: GCP-HCP

**Date**: 2026-07-15

## Decision

HCP Terraform (Terraform Cloud) workspaces that manage GCP infrastructure must authenticate via Workload Identity Federation (WIF) using HCP Terraform's Dynamic Provider Credentials feature. The WIF OIDC provider must set `allowed_audiences = ["https://app.terraform.io"]` and workspaces must include the `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` environment variable to ensure the OIDC token audience matches.

## Context

- **Problem Statement**: HCP Terraform workspaces need to authenticate to GCP APIs to manage infrastructure (GKE clusters, GCS buckets, IAM bindings, etc.) without static service account keys. HCP Terraform supports two WIF integration modes — legacy environment variables and Dynamic Provider Credentials — which use different OIDC token audience conventions that must be aligned with the GCP-side WIF provider configuration.
- **Constraints**:
  - No static GCP service account JSON keys (consistent with platform-wide WIF-first policy)
  - The GCP WIF pool and provider live in the commons project (`gcp-hcp-commons`), managed by the commons Terraform module (applied manually by SRE, not by Atlantis)
  - HCP Terraform workspace configuration is managed via the `hp-platform-engineering/workspaces/tfe` module in `hcp-terraform/test-gcp-hcp-terraform/main.tf`
  - The HCP Terraform organization uses Dynamic Provider Credentials (credential sets configured per workspace), not the legacy `TFC_GCP_PROVIDER_AUTH` environment variable flow
- **Assumptions**:
  - All future HCP Terraform workspaces managing GCP resources will use the same commons WIF pool (`tfc-pool`) and OIDC provider (`tfc-oidc`)
  - The audience value `https://app.terraform.io` is stable and will remain the default HCP Terraform issuer URI

## Alternatives Considered

1. **Dynamic Provider Credentials with explicit audience**: Configure the GCP WIF provider with `allowed_audiences = ["https://app.terraform.io"]` and set `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE = "https://app.terraform.io"` on each workspace. The OIDC token audience matches the allowed audience on the GCP side.
2. **Dynamic Provider Credentials with default audience**: Remove `allowed_audiences` from the GCP WIF provider so GCP accepts the WIF provider resource name as the audience. Do not set any audience variable on workspaces — HCP Terraform defaults to the provider resource name.
3. **Legacy environment variable authentication**: Set `TFC_GCP_PROVIDER_AUTH`, `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL`, `TFC_GCP_WORKLOAD_PROVIDER_NAME`, and `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` as workspace environment variables without using Dynamic Provider Credential sets.
4. **Static service account keys**: Export a JSON key for the `tfc-automation` service account and store it as a sensitive workspace variable.

## Decision Rationale

* **Justification**: Alternative 1 (Dynamic Provider Credentials with explicit audience) is the most secure and operationally sound option. Dynamic Provider Credentials are HCP Terraform's recommended authentication method — they manage the OIDC token exchange lifecycle automatically, link credentials to a named credential set visible in the workspace UI, and support credential rotation without workspace variable changes. Setting an explicit audience on both sides makes the authentication contract clear and debuggable.
* **Evidence**: During initial setup, the workspace was configured with `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` (the legacy variable). Dynamic Provider Credentials ignore this variable and instead use `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE`. The token was sent with the WIF provider resource name as the audience, which did not match the `allowed_audiences` on the GCP side, causing `invalid_grant` errors. Adding `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE = "https://app.terraform.io"` immediately resolved the authentication failure. See PRs [#851](https://github.com/openshift-online/gcp-hcp-infra/pull/851), [#855](https://github.com/openshift-online/gcp-hcp-infra/pull/855), [#859](https://github.com/openshift-online/gcp-hcp-infra/pull/859), and [#866](https://github.com/openshift-online/gcp-hcp-infra/pull/866).
* **Comparison**:
  - **Alternative 2 (default audience)** works but is fragile — the accepted audience becomes an opaque provider resource name (`//iam.googleapis.com/projects/573522191771/...`) that could change if the pool or provider is recreated. Explicit audiences are easier to reason about and debug.
  - **Alternative 3 (legacy env vars)** does not integrate with HCP Terraform's credential set UI, making it harder to audit which workspaces use which credentials. The key pitfall: `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` and `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` are distinct variables — the former is for the legacy flow, the latter for Dynamic Provider Credentials. Mixing them causes silent authentication failures.
  - **Alternative 4 (static keys)** is prohibited by platform security policy and introduces key management burden.

## Consequences

### Positive

* No static credentials — OIDC tokens are short-lived and automatically rotated by HCP Terraform
* Explicit audience alignment between GCP and HCP Terraform makes authentication failures easy to diagnose
* Dynamic Provider Credential sets are visible in the workspace UI, providing clear audit trail of which workspaces use which GCP identity
* Consistent with platform-wide WIF-first authentication policy
* Workspace variables are managed in code via the `tfe` module, preventing configuration drift

### Negative

* The distinction between `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` (legacy) and `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` (Dynamic Provider Credentials) is poorly documented by HashiCorp and caused debugging overhead during initial setup
* Changes to the WIF provider (`allowed_audiences`) require an SRE to manually apply the commons module — cannot be automated via Atlantis
* Adding a new workspace requires adding the `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` variable alongside the other WIF variables; omitting it will silently break authentication

## Cross-Cutting Concerns

### Security:

* Federated tokens are short-lived (~1h) and scoped to the specific HCP Terraform run phase (plan or apply)
* The `attribute_condition` on the WIF provider restricts federation to the configured HCP Terraform organization only
* Workload Identity User binding on the `tfc-automation` service account is scoped to `principalSet` matching the organization name
* No secrets stored in HCP Terraform — the token exchange uses the workspace's OIDC identity, not stored credentials

### Operability:

* **Adding a new workspace**: Add a workspace entry to `hcp-terraform/test-gcp-hcp-terraform/main.tf` with `variables = local.tfc_wif_variables`. The `tfc_wif_variables` local includes all required WIF environment variables including `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE`.
* **Debugging authentication failures**: Check for audience mismatch — the GCP error message includes the token's actual audience. Verify it matches `allowed_audiences` on the WIF provider. Common failure: using `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` instead of `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE`.
* **WIF provider changes**: Require manual `terraform apply` of the commons module by an SRE. Changes are not managed by Atlantis.

### Cost:

* WIF and STS token exchanges are free — no additional GCP charges
* HCP Terraform workspace costs are governed by the organization's HCP Terraform plan, not by the authentication method

## Implementation Reference

### GCP Side (commons module)

The WIF pool, OIDC provider, and service account are defined in `terraform/modules/commons/tfc.tf`:

- **Pool**: `tfc-pool` — Workload Identity Federation pool for HCP Terraform
- **Provider**: `tfc-oidc` — OIDC provider with issuer `https://app.terraform.io` and `allowed_audiences = ["https://app.terraform.io"]`
- **Service account**: `tfc-automation` — GCP service account that HCP Terraform impersonates via WIF
- **IAM binding**: `roles/iam.workloadIdentityUser` granted to all identities in the pool matching the organization name

### HCP Terraform Side (workspace config)

Workspace variables are defined in `hcp-terraform/test-gcp-hcp-terraform/main.tf`:

| Variable | Value | Purpose |
|----------|-------|---------|
| `TFC_GCP_PROVIDER_AUTH` | `true` | Enables GCP provider authentication via WIF |
| `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` | `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com` | GCP service account to impersonate |
| `TFC_GCP_WORKLOAD_PROVIDER_NAME` | `projects/573522191771/locations/global/workloadIdentityPools/tfc-pool/providers/tfc-oidc` | Full resource name of the WIF OIDC provider |
| `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` | `https://app.terraform.io` | OIDC token audience for Dynamic Provider Credentials |

### Authentication Flow

```
HCP Terraform Run
    │
    ├─ 1. TFC generates OIDC token with:
    │      - issuer: https://app.terraform.io
    │      - audience: https://app.terraform.io (from TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE)
    │      - subject: organization:hp-platform-engineering:project:...:workspace:...:run_phase:...
    │
    ├─ 2. Token sent to GCP Security Token Service (STS)
    │      - STS validates token against WIF provider tfc-oidc
    │      - Checks issuer matches (https://app.terraform.io)
    │      - Checks audience matches allowed_audiences (https://app.terraform.io)
    │      - Evaluates attribute_condition (organization name)
    │
    ├─ 3. STS returns federated access token
    │
    └─ 4. Federated token exchanged for service account access token
           - Impersonates tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com
           - Short-lived token (~1h) used for GCP API calls
```
