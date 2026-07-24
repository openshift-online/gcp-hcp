# GCP-536: Atlantis to TFC Cutover

**Jira**: [GCP-536](https://redhat.atlassian.net/browse/GCP-536)
**Date**: 2026-07-24
**Authors**: Roger Flores, Jim DAgostino, Pat Martin

## Context

We're migrating from Atlantis to HCP Terraform Cloud (TFC) for infrastructure automation. An E2E audit of Atlantis IAM identified **34 unique roles** across the global, region, management-cluster, and commons Terraform modules. The team agreed on a 3-project TFC architecture (`gcp-hcp-{env}`, `gcp-hcp-tooling`, `gcp-hcp-ci`).

This plan implements the integration environment first, using Atlantis to bootstrap its own replacement. Stage follows once integration is validated. Production does not have Atlantis today, so TFC will be the first automation there.

## Why Not `gcp-dynamic-creds` for Infra Workspaces

The `gcp-dynamic-creds` module (v0.0.14, from [infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90)) was validated E2E in the [playground experiment](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md). It works well for its intended use case — workspaces that manage resources in a **single** GCP project. However, it introduces significant architectural constraints when applied to our infra workspaces, which manage resources **across multiple GCP projects**.

### The Problem

Atlantis is simple: one SA per environment, lives in the global project, gets IAM on every project it touches. A single region workspace manages resources in the region project AND creates IAM bindings on the global project (via `modules/region/global-iam.tf`). One identity, cross-project access, no friction.

The `gcp-dynamic-creds` module doesn't fit this pattern:

| Constraint | Impact |
|---|---|
| **SA created in target project, not global** | Region SA lives in the region project instead of the global project. Doesn't match the architecture diagram's per-env SA model. |
| **One module call = one GCP project** | Can't create an SA in the global project and grant it roles on a different project. Cross-project grants are entirely outside the module's scope. |
| **Cross-project IAM gap** | Region/MC modules create resources on the global project (`global-iam.tf`). A per-project SA with roles only on the region project gets `Permission denied` on the global project. This breaks most applies. |
| **Variable set conflicts** | Three module calls for the same TFC project create three variable sets, each setting different `TFC_GCP_*_SERVICE_ACCOUNT_EMAIL` values. Precedence between same-priority variable sets is undefined. |
| **6 SAs instead of 1** | Module creates plan + apply SA per project — 6 SAs for integration alone, vs. 1 SA with Atlantis. More complexity for no isolation benefit (all workspaces in an env need the same access). |

Forcing the module means: supplemental cross-project IAM grants outside the module for every region/MC project, careful variable set scoping to avoid precedence conflicts, and a fundamentally different SA architecture from what Atlantis uses. The complexity isn't worth it.

### Where the Module Still Fits

The `gcp-dynamic-creds` module is a good fit for the **`gcp-hcp-ci`** TFC project (hypershift-ci, platform-ci), where each workspace genuinely manages resources in a single GCP project with no cross-project IAM. The impersonation pattern in the architecture diagram (commons SA → per-CI-project SA) aligns with how the module works.

### What We Use Instead

Direct WIF setup: one SA per environment, one WIF pool per environment, same cross-project IAM pattern as Atlantis. ~30 lines of Terraform per environment — the WIF plumbing is straightforward and doesn't need a module abstraction.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Rollout order | Integration → Stage → Production | Validate on least-risky env first |
| Infra WIF approach | Direct WIF setup (no module) | Module creates per-project SAs that can't handle cross-project IAM; direct setup matches Atlantis's proven single-SA-per-env model |
| CI WIF approach | `gcp-dynamic-creds` module | CI workspaces target single GCP projects — the module's intended use case |
| SA architecture | One SA per env in the global project | Mirrors Atlantis. One identity manages global + region + MC in that env. |
| WIF pool location | Per-env global project | SA and pool co-located. Attribute condition scopes to that env's TFC project. |
| Plan IAM | SA gets full role set (no plan/apply split) | Single SA simplifies; `terraform plan` sometimes needs write-adjacent permissions (e.g., generating plans for IAM changes). Can split later if needed. |

## Atlantis IAM Audit Summary

The audit identified every IAM role Atlantis has across all modules. The TFC SA must replicate all of these.

### Global Project (19 project-level + 3 folder-level roles)

**Project-level** (via workload-identity module `roles` list in `modules/global/atlantis.tf`):

`container.admin`, `gkehub.admin`, `compute.networkAdmin`, `storage.admin`, `compute.instanceAdmin.v1`, `iam.serviceAccountAdmin`, `resourcemanager.projectIamAdmin`, `serviceusage.serviceUsageAdmin`, `compute.securityAdmin`, `dns.admin`, `secretmanager.admin`, `certificatemanager.editor`, `iap.admin`, `artifactregistry.admin`, `cloudbuild.builds.editor`, `logging.admin`, `iam.workloadIdentityPoolAdmin`, `monitoring.metricsScopesAdmin`, `monitoring.editor`

**Folder-level** (on `folders/{parent_folder_id}`):

`resourcemanager.projectCreator`, `resourcemanager.folderAdmin`, `logging.configWriter`

### Region Projects (24 cross-project roles in `modules/region/atlantis.tf`)

`resourcemanager.projectIamAdmin` (bootstrap), `viewer`, `container.admin`, `compute.networkAdmin`, `storage.admin`, `compute.instanceAdmin.v1`, `iam.serviceAccountAdmin`, `iam.serviceAccountUser`, `serviceusage.serviceUsageAdmin`, `compute.securityAdmin`, `dns.admin`, `gkehub.admin`, `secretmanager.admin`, `workflows.admin`, `run.admin`, `pubsub.admin`, `eventarc.admin`, `resourcemanager.tagAdmin`, `resourcemanager.tagUser`, `privilegedaccessmanager.admin`, `storage.hmacKeyAdmin`, `bigquery.admin`, `artifactregistry.admin`, `monitoring.metricsScopesAdmin`

### Management-Cluster Projects (20 cross-project roles in `modules/management-cluster/atlantis.tf`)

Same as region minus: `gkehub.admin`, `storage.hmacKeyAdmin`, `bigquery.admin`, `artifactregistry.admin`

### Commons Grants (in `modules/commons/atlantis-iam.tf`)

- `storage.objectViewer` on commons TF state bucket
- `artifactregistry.admin` on commons GAR repo
- `iam.serviceAccountTokenCreator` on `project-creator` SA

## Implementation Phases

### Phase 0: TFC WIF + SA Setup in Global Module (via Atlantis)

**Goal**: Create one TFC service account and WIF pool per environment in the global project, with the same cross-project IAM pattern as Atlantis.

**Changes** (all in `gcp-hcp-infra`):

1. **`terraform/modules/global/tfc.tf`** (new file) — WIF pool, OIDC provider, SA, and WIF impersonation binding:

   ```hcl
   resource "google_iam_workload_identity_pool" "tfc" { ... }
   resource "google_iam_workload_identity_pool_provider" "tfc_oidc" { ... }
   resource "google_service_account" "tfc" {
     account_id = "tfc-automation"
   }
   resource "google_service_account_iam_member" "tfc_wif" { ... }
   ```

   Plus project-level IAM — the same 19 roles Atlantis has, granted to the TFC SA. Pattern: copy `atlantis.tf` project-level roles, change the member to `google_service_account.tfc.email`.

   Plus folder-level IAM — the same 3 folder roles. Pattern: copy `google_folder_iam_member.atlantis_*` resources.

   Gated by `var.enable_tfc` (default `false`).

2. **`terraform/modules/region/tfc.tf`** (new file) — cross-project IAM for the TFC SA on each region project. Same 24 roles as `atlantis.tf`, same bootstrap pattern (project-creator impersonation for the first `projectIamAdmin` grant, then self-managed). Member: `serviceAccount:tfc-automation@${var.global_project_id}.iam.gserviceaccount.com`.

3. **`terraform/modules/management-cluster/tfc.tf`** (new file) — same pattern, 20 roles on each MC project.

4. **Enable in integration configs**:
   - `terraform/config/global/integration/main/us-central1/main.tf`: `enable_tfc = true`
   - `terraform/config/region/integration/main/us-central1/main.tf`: `enable_tfc = true`
   - `terraform/config/management-cluster/integration/main/us-central1-yjiv/main.tf`: `enable_tfc = true`

**Why this works**: The TFC SA gets exactly the same IAM as Atlantis — same project roles, same folder roles, same cross-project grants. The SA lives in the global project and can manage resources in all projects, just like Atlantis. No gaps, no supplemental grants needed.

**Apply via**: Atlantis PR (split IAM from resource creation per propagation delay guidance — first PR creates WIF pool + SA + IAM, second PR adds workspaces)

**Target projects**: `gcp-hcp-int-global` (SA + WIF home), `int-reg-us-c1-nkcw` (cross-project IAM), `int-mgt-us-c1-yjiv` (cross-project IAM)

### Phase 1: Commons Cross-Project Grants (SRE-applied)

**Goal**: Grant the TFC SA the same commons-level access as Atlantis.

Add to `terraform/modules/commons/tfc-iam.tf` (new file, parallel to `atlantis-iam.tf`):

- `roles/storage.objectViewer` on commons TF state bucket
- `roles/artifactregistry.admin` on commons GAR repo
- `roles/iam.serviceAccountTokenCreator` on `project-creator` SA

Iterate over `var.environment_dns_zones` (excluding dev). Member: `serviceAccount:tfc-automation@${each.value.project_id}.iam.gserviceaccount.com`

**Apply via**: SRE manual apply (commons module is NOT managed by Atlantis)

### Phase 2: TFC Variable Set

**Goal**: Create a TFC variable set so workspaces can authenticate via WIF.

This can be a simple manual step in the TFC UI or a Terraform resource in the workspace definitions:

| Variable | Value |
|---|---|
| `TFC_GCP_PROVIDER_AUTH` | `true` |
| `TFC_GCP_RUN_SERVICE_ACCOUNT_EMAIL` | `tfc-automation@gcp-hcp-int-global.iam.gserviceaccount.com` |
| `TFC_GCP_WORKLOAD_PROVIDER_NAME` | `projects/{project_number}/locations/global/workloadIdentityPools/tfc-pool/providers/tfc-oidc` |

One variable set per environment, attached to all workspaces in that env's TFC project. No `PLAN`/`APPLY` SA split — single `RUN` SA. No precedence conflicts.

### Phase 3: TFC Workspace Definitions

**Goal**: Create TFC workspaces mirroring the Atlantis projects in `atlantis-integration.yaml`.

| New File | Purpose |
|---|---|
| `hcp-terraform/gcp-hcp-int/cloud.tf` | Cloud backend pointing to meta workspace |
| `hcp-terraform/gcp-hcp-int/main.tf` | Workspace definitions via `workspaces/tfe` module |

Workspaces:

| TFC Workspace | Working Directory | Trigger Prefixes |
|---|---|---|
| `gcp-hcp-global-int-main` | `terraform/config/global/integration/main/us-central1` | `terraform/metadata/`, `terraform/dashboards/global/` |
| `gcp-hcp-region-int-main-us-central1` | `terraform/config/region/integration/main/us-central1` | `terraform/workflows/`, `terraform/metadata/` |
| `gcp-hcp-mc-int-main-us-central1-yjiv` | `terraform/config/management-cluster/integration/main/us-central1-yjiv` | `terraform/workflows/`, `terraform/metadata/` |

Each workspace references the environment's variable set via `variable_set_names`. All workspaces use the same SA — no variable set conflicts.

Tooling and CI workspaces deferred until infra workspaces are validated.

**Apply via**: TFC meta workspace (push to main triggers it)

### Phase 4: Validation

1. **Plan comparison**: TFC plan on each workspace; compare with latest Atlantis plan
2. **Small change test**: Apply a minor change (e.g., add a label) via TFC
3. **State consistency**: Verify TFC reads the same GCS remote state
4. **Cross-project operations**: Verify region workspace can create IAM bindings on the global project (the key thing the module approach couldn't do)

### Phase 5: Scripts & Tooling

1. **`scripts/infra.py`** — extend for TFC workspace generation:
   - Add workspace entries to `hcp-terraform/gcp-hcp-{env}/main.tf`
   - Add TFC SA cross-project IAM to the new region/MC `tfc.tf` file
   - Same gate: `environment in ['integration', 'stage', 'production'] and sector != 'e2e'`

2. **Design docs** — update in this repo:
   - `design-decisions/automation/hcp-terraform-workload-identity-federation.md`
   - `design-decisions/automation/hcp-terraform-workspace-architecture.md`

### Phase 6: Repeat for Stage

Same Phases 0–4 for stage:
- Target projects: `gcp-hcp-stg-global`, `stg-reg-us-w1-{id}`, `stg-mgt-us-w1-zkpf`
- New `tfc.tf` files enabled with `enable_tfc = true` in stage configs
- New workspace definitions: `hcp-terraform/gcp-hcp-stg/`

### Phase 7: Cutover & Decommission Atlantis

1. Disable Atlantis autoplan (ArgoCD sync disabled or webhook removed)
2. Validate TFC handles all PRs for ~1 week
3. Remove Atlantis ArgoCD application (`argocd/config/global/atlantis/`)
4. Remove Atlantis Helm chart (`helm/charts/atlantis-stack/`)
5. Remove Atlantis SA and IAM bindings (`atlantis.tf` files)
6. Remove `atlantis-{env}.yaml` files
7. Update mintmaker agent (`agent/mintmaker/tools/atlantis.py` → TFC equivalent)

## PR Sequence (Integration)

| # | PR | Applied By | Depends On |
|---|---|---|---|
| 1 | WIF pool + SA + IAM in global/region/MC modules (`tfc.tf` files) | Atlantis | — |
| 2 | Commons cross-project grants (`tfc-iam.tf`) | SRE manual | PR 1 |
| 3 | TFC workspace definitions (`hcp-terraform/gcp-hcp-int/`) | TFC meta workspace | PR 1 + variable set created |
| 4 | Validation + small change test | TFC | PR 2 + PR 3 |
| 5 | `scripts/infra.py` extension | N/A (tooling) | PR 3 |

Note: fewer PRs than the module-based approach because there's no separate bootstrap phase — the WIF + SA + IAM are all created in the same Atlantis-managed modules.

## Comparison: Direct WIF vs. `gcp-dynamic-creds` Module

| Aspect | Direct WIF (this plan) | Module approach |
|---|---|---|
| **SAs per env** | 1 (matches Atlantis) | 6 (plan + apply × 3 projects) |
| **WIF pools per env** | 1 (in global project) | 3 (one per target project) |
| **Cross-project IAM** | Works identically to Atlantis | Requires supplemental grants outside module |
| **Variable sets per env** | 1 (no conflicts) | 3 (precedence conflicts possible) |
| **Folder-level roles** | Same `tfc.tf` file | Separate supplemental file |
| **Lines of Terraform** | ~30 per module + role list | ~30 per module call + role list + supplemental IAM |
| **Phases/PRs** | 5 PRs | 6 PRs |
| **Circular dependency risk** | None (no module creating TFC resources) | Must keep bootstrap in separate TFC project |
| **Module dependency** | None (standard GCP resources only) | Depends on infra-platform module versioning |

## Key Risks and Mitigations

| Risk | Mitigation |
|---|---|
| IAM propagation delay on new TFC SA roles | Split SA creation from workspace creation; re-apply after ~60s if first apply fails |
| State file locking during parallel Atlantis+TFC | Only one system should apply at a time during validation; use TFC speculative plans |
| Commons module requires SRE manual apply | Coordinate with SRE team; include in phase sequencing |
| Atlantis and TFC both triggering on same PR | Disable Atlantis autoplan for workspaces that TFC manages before enabling TFC |

## References

- [Atlantis IAM Audit](https://redhat.atlassian.net/browse/GCP-536) — full role inventory across all modules
- [HCP Terraform WIF Playground](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md) — experiment validation (WIF E2E proven, module constraints discovered)
- [HCP Terraform WIF Design Decision](../design-decisions/automation/hcp-terraform-workload-identity-federation.md)
- [HCP Terraform Workspace Architecture](../design-decisions/automation/hcp-terraform-workspace-architecture.md)
- [gcp-dynamic-creds module](https://app.terraform.io/app/hp-platform-engineering/registry/modules/private/hp-platform-engineering/gcp-dynamic-creds/tfe/0.0.14) — used for CI workspaces only
