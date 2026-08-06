# HCP Terraform Must Use Workload Identity Federation with Dedicated Access Projects

***Scope***: GCP-HCP

**Date**: 2026-07-27

## Decision

HCP Terraform workspaces that manage GCP infrastructure must authenticate via Workload Identity Federation (WIF) with dedicated access GCP projects per environment. Each access project (e.g., `gcp-hcp-{env_abbrev}-tfc-access`) hosts the WIF pool, OIDC provider, and plan/apply service accounts. The implementation uses the infra-platform [`terraform-tfe-gcp-dynamic-creds`](https://app.terraform.io/app/hp-platform-engineering/registry/modules/private/hp-platform-engineering/gcp-dynamic-creds/tfe) module for alignment with organizational tooling, though the architecture does not depend on it — the same setup could be achieved with ~30 lines of direct Terraform. The module's `apply_to_all_workspaces` flag delivers WIF credentials to all workspaces in a TFC project automatically via a project-scoped variable set.

## Context

- **Problem Statement**: HCP Terraform is replacing Atlantis for infrastructure automation ([GCP-536](https://redhat.atlassian.net/browse/GCP-536)). TFC workspaces need to authenticate to GCP APIs to manage resources across multiple GCP projects per environment (global, region, management-cluster) without static service account keys. The architecture follows the same central-identity model as Atlantis — a single set of SAs in a dedicated project receives cross-project IAM grants on each target project. The infra-platform team published a reusable module ([infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90)) that automates WIF pool, service account, and variable set lifecycle within a single GCP project per call. Cross-project IAM grants are managed separately in each target module, identical to the existing `atlantis.tf` pattern.
- **Constraints**:
  - No static GCP service account JSON keys (platform-wide WIF-first policy)
  - The module creates a WIF pool, OIDC provider, plan/apply SAs, and a TFC variable set — all scoped to a single GCP project per call
  - Cross-project IAM grants (SA in project A managing resources in projects B, C, D) are outside the module's scope
  - ~~The module call must not live in the same TFC project as the workspaces it configures~~ — resolved upstream in [infra-platform#119](https://github.com/openshift-online/infra-platform/pull/119) via `depends_on`, which ensures variable sets are not created until all cloud resources (WIF pool, provider, SAs) exist
  - CI requires a TFC API token (`TFE_TOKEN`) for private registry module sourcing ([release#82376](https://github.com/openshift/release/pull/82376))
  - Atlantis currently manages 34 unique IAM role types across modules. Per-module binding counts are higher due to overlap: global (19 project-level + 3 folder-level), region (24), and management-cluster (20)
- **Assumptions**:
  - The module will continue to be published to the TFC private registry at `app.terraform.io/hp-platform-engineering/gcp-dynamic-creds/tfe`
  - Each long-lived environment (integration, stage, production) follows the same access project pattern. Dynamic environments (dev, e2e) may require a different approach and are out of scope for this decision
  - The `apply_to_all_workspaces` module feature remains stable

## Alternatives Considered

1. **Module with dedicated access projects**: One `gcp-dynamic-creds` module call per environment, targeting a purpose-built GCP project that hosts only WIF resources (pool, provider, SAs). `apply_to_all_workspaces = true` delivers credentials to all workspaces in the TFC project. Cross-project IAM grants on target projects (global, region, MC) are managed separately via `tfc.tf` files in each infrastructure module.

2. **Direct WIF (no module)**: Architecturally identical to Alternative 1 — same central-identity model with cross-project grants. The difference is where the WIF resources live (environment's global project vs. dedicated access project) and whether the WIF plumbing is hand-written (~30 lines of Terraform) or module-managed. No module dependency, no additional GCP projects, but more resources bootstrapped in the global project.

3. **Module targeting environment global projects directly**: One module call per target project (global, region, MC) within the same environment. Module creates per-project SAs with roles only on that project. Each new region or MC would require its own module call and bootstrap.

4. **Direct Workload Identity (no SAs)**: Use `TFC_GCP_PRINCIPAL_TYPE = workload_pool` with direct `principal://` IAM bindings. Eliminates service accounts entirely — the WIF federated identity accesses GCP resources directly.

5. **Static service account keys**: Export JSON keys for TFC service accounts and store as sensitive workspace variables.

## Decision Rationale

* **Justification**: Alternative 1 (module + dedicated access projects) resolves the module's single-project constraint while aligning with the infra-platform team's WIF tooling. The dedicated access project gives the module a clean target — one project, one module call, one variable set — while the SAs it creates receive cross-project IAM grants on global, region, and MC projects. This preserves Atlantis's proven cross-project access pattern. `apply_to_all_workspaces` eliminates per-workspace credential configuration, so adding new workspaces requires zero WIF setup.

* **Evidence**:
  - The module was validated end-to-end in the [Phase 2 experiment](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md): WIF pool, plan/apply SAs, and variable sets created successfully; a validation workspace authenticated with zero explicit WIF variables via `apply_to_all_workspaces` inheritance.
  - Default audience behavior works — no `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` variable needed. The module does not set `allowed_audiences` on the OIDC provider, and HCP Terraform defaults to the provider resource name as the audience.
  - Workspace-scoped attribute conditions (`assertion.sub.startsWith(...)`) restrict authentication to the correct TFC project.
  - The circular dependency issue (module-created variable set poisoning the calling workspace on partial apply) is resolved upstream in [infra-platform#119](https://github.com/openshift-online/infra-platform/pull/119) — the module now uses `depends_on` to ensure variable sets are not created until all cloud resources exist. The access workspace can safely live in the same TFC project as the workspaces it configures.

* **Comparison**:
  - **Alternative 2 (direct WIF)** is architecturally the same — central identity with cross-project grants. The difference is operational: WIF resources live in the global project (more resources to bootstrap there) vs. a dedicated access project (clean separation). The module encapsulates WIF plumbing details that were error-prone during manual setup (audience mismatch between `TFC_GCP_WORKLOAD_PROVIDER_AUDIENCE` and `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` caused debugging overhead in Phase 1). No alignment with infra-platform team tooling.
  - **Alternative 3 (module per target project)** was explored and abandoned. Calling the module once per target project (global, region, MC) creates 6 SAs per environment (plan + apply x 3 projects), produces 3 variable sets for one TFC project with undefined precedence behavior, and requires supplemental cross-project IAM grants outside the module for every target project. The region module creates resources on both the region project AND the global project (`modules/region/global-iam.tf`) — a per-region-project SA cannot do this without additional grants. See [experiment issue #7](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md) for variable set precedence findings.
  - **Alternative 4 (direct WID)** eliminates SAs entirely but requires `principal://` IAM binding support across every GCP resource type the team manages (GKE, Compute, DNS, Secret Manager, Workflows, Pub/Sub, Eventarc, Cloud Run, Tags, PAM, BigQuery, Artifact Registry). This was never validated, and discovering an unsupported resource type during production rollout would require a mid-migration architecture change.
  - **Alternative 5 (static keys)** is prohibited by platform security policy and introduces key management burden.

## Consequences

### Positive

* Aligns with infra-platform team's WIF module — shared maintenance, upstream improvements, and organizational consistency across teams
* `apply_to_all_workspaces` eliminates per-workspace credential configuration — new workspaces inherit WIF credentials automatically
* Dedicated access project isolates WIF resources (pool, SAs) from infrastructure resources — clean separation of concerns
* Module handles the error-prone WIF plumbing (audience configuration, variable set wiring, attribute conditions) that caused debugging overhead during manual setup
* Workspace-scoped attribute conditions provide fine-grained authentication scoping per TFC project
* Default audience behavior simplifies configuration — no explicit `TFC_GCP_WORKLOAD_IDENTITY_AUDIENCE` or `allowed_audiences` needed

### Negative

* Introduces a new GCP project per environment (`gcp-hcp-{env_abbrev}-tfc-access`) solely for WIF resources — additional project to create and manage
* Access workspace requires bootstrap with local or static credentials for initial apply — a manual step per environment
* Module dependency on infra-platform releases — upstream changes or breakages affect WIF infrastructure
* Cross-project IAM grants (the bulk of the implementation) are still managed separately outside the module — the module only handles WIF plumbing within the access project
* Plan/apply SA split creates two SA objects per environment regardless of whether roles differ
* IAM propagation delay (~60s) after module apply means dependent workspaces may fail on first run
* SAs in the access project trigger GCP API activation checks against the access project (quota project), not the target project — every workspace must set `user_project_override = true` and `billing_project` in its provider blocks to redirect these checks ([GCP-990](https://redhat.atlassian.net/browse/GCP-990))

## Cross-Cutting Concerns

### Security:

* Federated OIDC tokens are short-lived (~1h) and scoped to the specific HCP Terraform run phase (plan or apply)
* Workspace-scoped `attribute_condition` on the WIF provider restricts federation to workspaces within the configured TFC project — tighter than the org-wide condition used in Phase 1
* No secrets stored in HCP Terraform — the token exchange uses the workspace's OIDC identity
* Plan and apply SAs start with unified roles on target projects so permission gaps surface at plan time. The plan SA can be restricted to read-only later if tighter speculative plan isolation is needed
* Access project contains only WIF resources — compromising it does not expose infrastructure state or resources

### Operability:

* **Adding a new workspace**: Zero WIF configuration needed — workspace inherits credentials from the project-level variable set via `apply_to_all_workspaces`. Cross-project IAM for the apply SA must already be in place on the target project. The workspace config must include `user_project_override = true` and `billing_project` in provider blocks (see below).
* **Adding a new environment**: Create access GCP project, create access workspace with bootstrap credentials, apply module, then create infrastructure workspaces. Grant cross-project IAM on each target project via `tfc.tf` files.
* **Adding a new region**: `scripts/infra.py` generates config and adds a workspace entry. WIF credentials are inherited automatically via the project-level variable set — no per-workspace WIF configuration needed. However, the workspace's `google` and `google-beta` provider blocks must still include `user_project_override = true` and `billing_project` (see provider configuration below). Cross-project IAM grants for the new region project must be added to `modules/region/tfc.tf`.
* **Provider configuration (`user_project_override`)**: Because TFC SAs live in a dedicated access project (not the target project), GCP checks API activation against the access project by default. This fails because the access project only has WIF-related APIs enabled. Each workspace's `google` and `google-beta` provider blocks must set `user_project_override = true` and `billing_project = "<target-project-id>"` to redirect API activation checks to the target project. See [GCP Quota project overview](https://cloud.google.com/docs/quotas/quota-project). This is a direct consequence of the dedicated access project architecture. Reference: [GCP-990](https://redhat.atlassian.net/browse/GCP-990).
* **State seeding for existing workspaces**: TFC remote execution mode ignores `backend "gcs"` blocks. Migrating an existing workspace requires uploading the current GCS state to TFC via the [State Versions API](https://developer.hashicorp.com/terraform/cloud-docs/api-docs/state-versions) before the first plan. Without this, TFC sees zero resources and attempts to create everything.
* **Debugging authentication failures**: Check the WIF provider attribute condition matches the TFC project name. Verify the module-created SAs have the required cross-project IAM bindings on target projects. Check for IAM propagation delay (~60s) on newly created bindings.
* **Module updates**: Pin to a specific module version in the TFC private registry. Test version upgrades in integration before promoting to stage/production.

### Reliability:

* **Observability**: WIF authentication failures surface as `invalid_grant` or `iam.serviceAccounts.getAccessToken` errors in the TFC run log. GCP Cloud Audit Logs record STS token exchanges and SA impersonation attempts, providing an audit trail. No additional monitoring infrastructure is needed — failures are visible in the TFC UI and GCP logs.
* **Resiliency**: WIF depends on GCP STS and the HCP Terraform OIDC token issuer. Both are managed services with high availability. If TFC is unavailable, no runs execute (same as Atlantis depending on its GKE cluster). If GCP STS is unavailable, all GCP API calls fail regardless of authentication method.

### Performance:

* Not materially impacted. The STS token exchange adds milliseconds to the `terraform init` phase — negligible compared to plan/apply execution time. No difference from Atlantis's GKE Workload Identity token exchange.

### Cost:

* WIF and STS token exchanges are free — no additional GCP charges
* Dedicated access projects have no running resources (no compute, no storage) — project-level costs are negligible
* HCP Terraform workspace costs are governed by the organization's plan, not by the authentication method

## Implementation Reference

### Access Project Architecture

```text
Per Environment:

┌──────────────────────────────────────────────────────────────┐
│ TFC Project: gcp-hcp-{env}                                   │
│                                                               │
│  ┌───────────────────────────────────┐                        │
│  │ Access Workspace                  │                        │
│  │ (bootstrap: local/static creds)   │                        │
│  │                                   │                        │
│  │  gcp-dynamic-creds module call    │                        │
│  │  → target: gcp-hcp-{env_abbrev}-tfc-access GCP project           │
│  │  → creates: WIF pool, OIDC provider, plan/apply SAs       │
│  │  → creates: variable set (apply_to_all_workspaces=true)   │
│  └───────────────────────────────────┘                        │
│                                                               │
│  ┌─────────────────────────┐  ┌─────────────────────────┐    │
│  │ gcp-hcp-global-{env}    │  │ gcp-hcp-region-{env}-*  │    │
│  │ (inherits WIF creds)    │  │ (inherits WIF creds)    │    │
│  └─────────────────────────┘  └─────────────────────────┘    │
│                                                               │
│  ┌─────────────────────────┐                                  │
│  │ gcp-hcp-mc-{env}-*      │                                  │
│  │ (inherits WIF creds)    │                                  │
│  └─────────────────────────┘                                  │
└──────────────────────────────────────────────────────────────┘

GCP Projects:

┌─────────────────────────────────┐  ┌──────────────────────┐
│ gcp-hcp-{env_abbrev}-tfc-access │  │ gcp-hcp-{env}-global │
│                              │     │                      │
│  • WIF pool                   │  │  • Infrastructure    │
│  • OIDC provider              │  │  • Cross-project IAM │
│  • Plan SA ──── write ────────▶  │    for plan SA       │
│  • Apply SA ── write ─────────▶  │    for apply SA      │
│                               │  │                      │
└─────────────────────────────────┘  └──────────────────────┘
         │          │                          │
         │          │                 ┌────────┴────────┐
         │          │                 ▼                 ▼
         │          │     ┌──────────────────┐ ┌──────────────────┐
         │          └────▶│ {env}-reg-*      │ │ {env}-mgt-*      │
         └──── write ────▶│  • Cross-project │ │  • Cross-project │
                          │    IAM grants    │ │    IAM grants    │
                          └──────────────────┘ └──────────────────┘
```

### Authentication Flow

```text
HCP Terraform Workspace (e.g., gcp-hcp-global-integration)
    │
    ├─ 1. Workspace inherits WIF variables from project-level variable set
    │      (delivered by gcp-dynamic-creds module via apply_to_all_workspaces)
    │
    ├─ 2. TFC generates OIDC token:
    │      issuer:   https://app.terraform.io
    │      audience: (default — WIF provider resource name, auto-matched)
    │      subject:  organization:hp-platform-engineering:project:gcp-hcp-{env}:
    │                workspace:gcp-hcp-global-{env}:run_phase:apply
    │
    ├─ 3. Token sent to GCP STS
    │      → validated against WIF provider in gcp-hcp-{env_abbrev}-tfc-access
    │      → attribute_condition checks sub starts with
    │        "...project:gcp-hcp-{env}:..."
    │
    ├─ 4. STS returns federated token
    │      → exchanged for apply SA access token
    │      SA: {prefix}-apply@gcp-hcp-{env_abbrev}-tfc-access.iam.gserviceaccount.com
    │
    └─ 5. SA token used for GCP API calls on target projects
           (requires cross-project IAM grants on global/region/MC projects)
```

### Cross-Project IAM

The module does not manage cross-project IAM. The plan and apply SAs in the access project need IAM roles on each target project. These are managed via `tfc.tf` files in each infrastructure module, mirroring the existing `atlantis.tf` pattern:

| Module | File | Role Count | Pattern |
|---|---|---|---|
| Global | `modules/global/tfc.tf` | 19 project-level + 3 folder-level | Copy from `atlantis.tf`, change member to access project SA |
| Region | `modules/region/tfc.tf` | 24 cross-project | Copy from `atlantis.tf`, same bootstrap pattern |
| Management-Cluster | `modules/management-cluster/tfc.tf` | 20 cross-project | Copy from `atlantis.tf` |
| Commons | `modules/commons/tfc-iam.tf` | 3 cross-project | `storage.objectViewer` on state bucket, `artifactregistry.admin` on GAR, `iam.serviceAccountTokenCreator` on `project-creator` SA |

Member format: `serviceAccount:{sa_name}@gcp-hcp-{env_abbrev}-tfc-access.iam.gserviceaccount.com`

Commons grants require SRE manual apply (commons module is not managed by Atlantis).

### CI Workspaces

`hypershift-ci` targets a single GCP project. `platform-ci` creates region and management-cluster projects, so it has the same cross-project IAM requirements as environment workspaces — the same access project pattern applies.

The `gcp-dynamic-creds` module is a natural fit for both CI workspaces. Alternatively, existing Prow WIF pools could be extended with a TFC OIDC provider. CI handling will be finalized during implementation.

### PagerDuty

The PagerDuty workspace uses a PagerDuty API key — no GCP IAM needed. It does not participate in the WIF topology.

## Open Items

### Resolved

- **Access project creation and bootstrap credentials**: The access project and WIF resources are bootstrapped locally following the same pattern as environment setup ([gcp-hcp-infra SETUP.md](https://github.com/openshift-online/gcp-hcp-infra/blob/main/terraform/config/global/SETUP.md)). An operator runs `terraform apply` locally with their own `gcloud` credentials and a `TFE_TOKEN` to create the access GCP project, WIF pool, SAs, and TFC variable set in a single apply. State is then migrated to TFC by uncommenting the `cloud {}` backend block and running `terraform init` (which prompts interactively for migration; the `-migrate-state` flag is not supported with the `cloud` backend). No static service account keys are needed. The operator needs `roles/resourcemanager.projectCreator` on the environment folder (to create the project; as project creator they become owner, so WIF and SA permissions are implicit) and a TFC API token (for variable set creation).
- **TFC project must exist before bootstrap**: The `gcp-dynamic-creds` module uses `data "tfe_project"` to look up the target TFC project for variable set attachment. The TFC project (e.g., `gcp-hcp-integration`) must be created first via the infra-platform bootstrap module (`hcp-terraform/tenants/gcp-hcp/main.tf`) before running the access project apply.
- **Circular dependency**: The local bootstrap sidesteps the circular dependency entirely — the first apply runs locally, not in a TFC workspace, so there is no workspace for `apply_to_all_workspaces` to poison. After state is migrated to TFC, the access workspace inherits its own variable set, but at that point the SAs already exist and the credentials are valid. If a future `terraform apply` on the access workspace partially fails (e.g., SA recreation), the same manual detach-and-reapply fix from the [experiment](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md) applies.

- **Plan vs. apply role assignment**: The module creates separate plan and apply SAs. On the access project itself, roles are scoped to what the workspace needs to manage its own WIF resources: `roles/viewer`, `roles/iam.workloadIdentityPoolAdmin`, `roles/iam.serviceAccountAdmin`, `roles/resourcemanager.projectIamAdmin`, and `roles/serviceusage.serviceUsageAdmin`. Note that `projectIamAdmin` grants the access workspace authority to modify the access project's IAM policy. This is intentional: the `gcp-dynamic-creds` module creates `google_project_iam_member` bindings for the plan/apply SAs, and the workspace must be able to manage these after state migration to TFC. On target projects (global, region, MC), both the plan and apply SAs will start with the same role set (matching Atlantis). Using unified roles means permission gaps surface during `terraform plan` rather than only at apply time, catching issues earlier in the PR cycle. The module supports this via unified cross-project IAM grants in each module's `tfc.tf`. If we later want tighter plan isolation (e.g., restricting speculative plans to read-only), the module supports splitting roles via separate `plan_roles` and `apply_roles` in the cross-project IAM bindings.

### Future Considerations

- **CI workspace WIF approach**: Whether to use the `gcp-dynamic-creds` module or extend existing Prow WIF pools with a TFC OIDC provider for CI workspaces.
- **RBAC model**: Who can approve applies per TFC project/workspace — not yet evaluated.

## References

- [Atlantis to HCP Terraform Cutover Implementation Plan](../../implementation-plans/gcp-532-atlantis-to-tfc-cutover.md) — Story-by-story cutover process
- [HCP Terraform WIF Playground Experiment](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md) — Phase 1 (SA impersonation) and Phase 2 (module) validation results
- [GCP-536](https://redhat.atlassian.net/browse/GCP-536) — Spike: Evaluate HCP Terraform for GCP-HCP Infrastructure
- [gcp-dynamic-creds module](https://app.terraform.io/app/hp-platform-engineering/registry/modules/private/hp-platform-engineering/gcp-dynamic-creds/tfe) — TFC private registry
- [infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90) — Module implementation
- [Miro: TFC Workspace Architecture](https://miro.com/app/board/uXjVH9A0m0w=/?focusWidget=3458764677714315706) — Architecture diagram from Sr Eng call
- [Global Environment Setup Guide](https://github.com/openshift-online/gcp-hcp-infra/blob/main/terraform/config/global/SETUP.md) — Bootstrap pattern for new environments (same local-apply approach used for access projects)
