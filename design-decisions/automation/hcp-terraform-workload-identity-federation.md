# HCP Terraform Must Use Workload Identity Federation with Dynamic Provider Credentials

***Scope***: GCP-HCP

**Date**: 2026-07-15

## Decision

HCP Terraform (Terraform Cloud) workspaces that manage GCP infrastructure must authenticate via Workload Identity Federation (WIF) using HCP Terraform's Dynamic Provider Credentials feature. The WIF OIDC provider must set `allowed_audiences = ["https://app.terraform.io"]` and workspaces must include the `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` environment variable to ensure the OIDC token audience matches.

## Context

- **Problem Statement**: HCP Terraform workspaces need to authenticate to GCP APIs to manage infrastructure (GKE clusters, GCS buckets, IAM bindings, etc.) without static service account keys. HCP Terraform supports two WIF integration modes — legacy environment variables and Dynamic Provider Credentials — which use different OIDC token audience conventions that must be aligned with the GCP-side WIF provider configuration.
- **Constraints**:
  - No static GCP service account JSON keys (consistent with platform-wide WIF-first policy)
  - The GCP WIF pool and OIDC provider live in the commons project (`gcp-hcp-commons`), managed by the commons Terraform module (applied manually by SRE, not by Atlantis)
  - Environment service accounts live in each environment's own global project (`tfc-automation@gcp-hcp-{env}-global.iam.gserviceaccount.com`) with cross-project `workloadIdentityUser` bindings on the commons pool. Tooling and CI base SAs remain in `gcp-hcp-commons`
  - HCP Terraform workspace configuration is managed via the `hp-platform-engineering/workspaces/tfe` module in `hcp-terraform/{tfe_project}/main.tf`
  - The HCP Terraform organization uses Dynamic Provider Credentials (credential sets configured per workspace). `TFC_GCP_PROVIDER_AUTH=true` enables this feature. The legacy flow used `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` to set the token audience; Dynamic Provider Credentials use `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` instead
- **Assumptions**:
  - All future HCP Terraform workspaces managing GCP resources will use the same commons WIF pool (`tfc-pool`) and OIDC provider (`tfc-oidc`)
  - The audience value `https://app.terraform.io` is stable and will remain the default HCP Terraform issuer URI

## Alternatives Considered

1. **Dynamic Provider Credentials with explicit audience**: Configure the GCP WIF provider with `allowed_audiences = ["https://app.terraform.io"]` and set `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE = "https://app.terraform.io"` on each workspace. The OIDC token audience matches the allowed audience on the GCP side.
2. **Dynamic Provider Credentials with default audience**: Remove `allowed_audiences` from the GCP WIF provider so GCP accepts the WIF provider resource name as the audience. Do not set any audience variable on workspaces — HCP Terraform defaults to the provider resource name.
3. **Legacy environment variable authentication**: Set `TFC_GCP_PROVIDER_AUTH`, `TFC_GCP_WORKLOAD_PROVIDER_NAME`, and the legacy audience variable `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` as workspace environment variables without using Dynamic Provider Credential sets. In this flow, `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` is also set as a plain env var rather than being linked to a credential set.
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
* ~~Adding a new workspace requires adding the `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` variable alongside the other WIF variables; omitting it will silently break authentication~~ — **Resolved**: WIF variables are now delivered via project-level variable sets; new workspaces inherit them automatically (validated with `gcp-hcp-dev-playground-2`)
* Environment SAs in per-env global projects require cross-project `workloadIdentityUser` bindings on the commons WIF pool — an additional IAM step when onboarding a new environment

## Cross-Cutting Concerns

### Security:

* Federated tokens are short-lived (~1h) and scoped to the specific HCP Terraform run phase (plan or apply)
* The `attribute_condition` on the WIF provider restricts federation to the configured HCP Terraform organization only
* Workload Identity User binding on the `tfc-automation` service account is scoped to `principalSet` matching the organization name
* No secrets stored in HCP Terraform — the token exchange uses the workspace's OIDC identity, not stored credentials

### Operability:

* **Adding a new workspace**: Environment workspaces inherit WIF variables from project-level variable sets (e.g., `wif-integration`). The SA is in the environment's global project (`tfc-automation@gcp-hcp-{env}-global`). CI workspaces additionally configure impersonation to per-CI-project SAs.
* **Debugging authentication failures**: Check for audience mismatch — the GCP error message includes the token's actual audience. Verify it matches `allowed_audiences` on the WIF provider. Common failure: using `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` instead of `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE`. For CI two-hop failures, verify both the WIF authentication (commons SA) and the impersonation binding (CI SA).
* **WIF provider changes**: Require manual `terraform apply` of the commons module by an SRE. Changes are not managed by Atlantis.

### Cost:

* WIF and STS token exchanges are free — no additional GCP charges
* HCP Terraform workspace costs are governed by the organization's HCP Terraform plan, not by the authentication method

## Implementation Reference

### GCP Side (commons module)

The WIF pool and OIDC provider are defined in `terraform/modules/commons/tfc.tf`:

- **Pool**: `tfc-pool` — Workload Identity Federation pool for HCP Terraform
- **Provider**: `tfc-oidc` — OIDC provider with issuer `https://app.terraform.io` and `allowed_audiences = ["https://app.terraform.io"]`
- **IAM binding**: `roles/iam.workloadIdentityUser` granted to identities in the pool matching the organization name

Service accounts follow a per-environment model:

- **Environment SAs**: `tfc-automation@gcp-hcp-{env}-global.iam.gserviceaccount.com` — live in each environment's own global project, with cross-project `workloadIdentityUser` bindings on the commons pool. Created manually and imported, or created by Atlantis.
- **Tooling/CI base SA**: `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com` — used directly by `gcp-hcp-tooling` workspaces. For `gcp-hcp-ci` workspaces, this SA authenticates via WIF and then impersonates per-CI-project SAs:
  - `tfc-hypershift-ci@gcp-hcp-hypershift-ci.iam.gserviceaccount.com`
  - `tfc-platform-ci@gcp-hcp-platform-ci.iam.gserviceaccount.com`

### HCP Terraform Side (workspace config)

Workspace variables are delivered via project-scoped variable sets (e.g., `wif-integration`):

| Variable | Value (environment workspaces) | Purpose |
|----------|-------|---------|
| `TFC_GCP_PROVIDER_AUTH` | `true` | Enables GCP provider authentication via WIF |
| `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` | `tfc-automation@gcp-hcp-{env}-global.iam.gserviceaccount.com` | GCP service account to impersonate (env SA in env global project) |
| `TFC_GCP_WORKLOAD_PROVIDER_NAME` | `projects/573522191771/locations/global/workloadIdentityPools/tfc-pool/providers/tfc-oidc` | Full resource name of the WIF OIDC provider (commons) |
| `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` | `https://app.terraform.io` | OIDC token audience for Dynamic Provider Credentials |

For CI workspaces, `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` is `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com` (the base SA), and the Terraform provider configuration uses `impersonate_service_account` to assume the per-CI-project SA.

### Authentication Flow

```text
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
           - Environment workspaces: impersonates tfc-automation@gcp-hcp-{env}-global
           - Tooling workspaces: impersonates tfc-automation@gcp-hcp-commons
           - CI workspaces: impersonates tfc-automation@gcp-hcp-commons, then
             impersonates per-CI-project SA (e.g., tfc-hypershift-ci@gcp-hcp-hypershift-ci)
           - Short-lived token (~1h) used for GCP API calls
```
