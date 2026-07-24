# GCP-536: Atlantis to TFC Cutover

**Jira**: [GCP-536](https://redhat.atlassian.net/browse/GCP-536)
**Date**: 2026-07-24
**Authors**: Roger Flores, Jim DAgostino, Pat Martin

## Context

We're migrating from Atlantis to HCP Terraform Cloud (TFC) for infrastructure automation. An E2E audit of Atlantis IAM identified **34 unique roles** across the global, region, management-cluster, and commons Terraform modules. The `gcp-dynamic-creds` module (v0.0.14, from [infra-platform#90](https://github.com/openshift-online/infra-platform/pull/90)) has been validated E2E in a [playground experiment](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md), proving per-project WIF pools with plan/apply SA pairs work. The team agreed on a 3-project TFC architecture (`gcp-hcp-{env}`, `gcp-hcp-tooling`, `gcp-hcp-ci`).

This plan implements the integration environment first, using Atlantis to bootstrap its own replacement. Stage follows once integration is validated. Production does not have Atlantis today, so TFC will be the first automation there.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Rollout order | Integration → Stage → Production | Validate on least-risky env first |
| Bootstrap location | `terraform/config/tfc-bootstrap/{env}/` | Dedicated config, Atlantis applies initially, migrates to TFC later |
| Plan IAM roles | `roles/viewer` only | Least-privilege; plan only needs read access |
| Apply IAM roles | Full Atlantis-equivalent role set per module type | Exact parity with current Atlantis permissions |
| Module | `app.terraform.io/hp-platform-engineering/gcp-dynamic-creds/tfe` v0.0.14 | Validated in experiment, creates per-project WIF pools + plan/apply SA pairs |
| SA architecture | Per-project plan/apply SAs (module-managed) | One WIF pool per role group per GCP project, scoped to specific workspaces |

## Atlantis IAM Audit Summary

The audit identified every IAM role Atlantis has across all modules. These are the roles the TFC apply SAs must replicate:

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

## What the Module Handles vs. What Falls Outside

| Concern | Handled by `gcp-dynamic-creds`? | Where to implement |
|---|---|---|
| Per-project WIF pool + OIDC provider | Yes | Module call |
| Plan/apply service accounts | Yes | Module call |
| Project-level IAM role grants | Yes (via `plan_roles` / `apply_roles`) | Module call |
| TFC variable sets | Yes (auto-attached) | Module call |
| **Folder-level roles** | **No** | Separate `google_folder_iam_member` resources |
| **Commons state bucket access** | **No** | Separate grant in commons module |
| **Commons GAR repo access** | **No** | Separate grant in commons module |
| **Project-creator SA impersonation** | **No** | Separate grant in commons module |
| **Bootstrap IAM for module caller** | **No** | Must pre-grant before module runs |

## Implementation Phases

### Phase 0: Bootstrap IAM for `tfc-automation` SA

**Goal**: Grant the commons `tfc-automation@gcp-hcp-commons.iam.gserviceaccount.com` SA the 5 roles it needs on each integration GCP project to create WIF infrastructure.

**Why Atlantis**: The `tfc-automation` SA currently has zero permissions on environment projects. Atlantis already has `projectIamAdmin` on all of them.

**Changes** (all in `gcp-hcp-infra`):

| File (new) | Scope | Roles |
|---|---|---|
| `terraform/modules/global/tfc-bootstrap-iam.tf` | Global project | 5 bootstrap roles (see below) |
| `terraform/modules/region/tfc-bootstrap-iam.tf` | Region project | Same 5 roles |
| `terraform/modules/management-cluster/tfc-bootstrap-iam.tf` | MC project | Same 5 roles |

Bootstrap roles: `serviceusage.serviceUsageAdmin`, `iam.workloadIdentityPoolAdmin`, `iam.serviceAccountAdmin`, `resourcemanager.projectIamAdmin`, `iam.serviceAccountUser`

Gated by `var.enable_tfc_bootstrap` (default `false`). Enable in integration configs only.

**Target projects**: `gcp-hcp-int-global`, `int-reg-us-c1-nkcw`, `int-mgt-us-c1-yjiv`

**Apply via**: Atlantis PR (IAM-only, split from resource creation per IAM propagation delay guidance)

### Phase 1: TFC Credential Bootstrap

**Goal**: Call `gcp-dynamic-creds` for each integration GCP project, creating per-project WIF pools + plan/apply SAs + TFC variable sets.

**Changes**:

1. **`terraform/config/tfc-bootstrap/integration/main.tf`** (new config)
   - Backend: GCS (`gcp-hcp-int-global-terraform-state`, prefix `tfc-bootstrap`)
   - Providers: `google` (via commons WIF) + `tfe`
   - Three module calls:

| Module Call | GCP Project | Plan Roles | Apply Roles | Workspace Scope |
|---|---|---|---|---|
| `creds_global` | `gcp-hcp-int-global` | `roles/viewer` | 19 global roles | `gcp-hcp-global-*` |
| `creds_region` | `int-reg-us-c1-nkcw` | `roles/viewer` | 24 region roles | `gcp-hcp-region-*` |
| `creds_mc` | `int-mgt-us-c1-yjiv` | `roles/viewer` | 20 MC roles | `gcp-hcp-mc-*` |

2. **`terraform/atlantis-integration.yaml`** — add `tfc-bootstrap-int` project entry

**Prerequisites**:
- Phase 0 IAM must have propagated (~60s)
- TFC project `gcp-hcp-int` must exist in TFC
- `TFE_TOKEN` must be available (workspace variable or variable set)

**Apply via**: Atlantis PR

### Phase 2: Supplemental IAM (Outside Module Scope)

**Goal**: Grant module-created plan/apply SAs the permissions the module can't handle.

#### 2a: Folder-Level Roles

Add to `terraform/modules/global/tfc-iam.tf` (new file):

- `resourcemanager.projectCreator` on `folders/{parent_folder_id}`
- `resourcemanager.folderAdmin` on `folders/{parent_folder_id}`
- `logging.configWriter` on `folders/{parent_folder_id}`

Gated by `var.tfc_apply_sa_email` (default `null`). Set to the apply SA email output from Phase 1.

**Apply via**: Atlantis PR

#### 2b: Commons Cross-Project Grants

Add to `terraform/modules/commons/tfc-iam.tf` (new file, parallel to `atlantis-iam.tf`):

- `storage.objectViewer` on commons TF state bucket
- `artifactregistry.admin` on commons GAR repo
- `iam.serviceAccountTokenCreator` on `project-creator` SA

Uses convention-based SA email: `hcp-tf-default-apply@gcp-hcp-{env}-global.iam.gserviceaccount.com`

**Apply via**: SRE manual apply (commons module is NOT managed by Atlantis)

### Phase 3: TFC Workspace Definitions

**Goal**: Create TFC workspaces mirroring the Atlantis projects in `atlantis-integration.yaml`.

**Changes**:

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

Tooling and CI workspaces deferred until infra workspaces are validated.

**Apply via**: TFC meta workspace (push to main triggers it)

### Phase 4: Validation

1. **Plan comparison**: TFC plan on each workspace; compare with latest Atlantis plan
2. **Small change test**: Apply a minor change (e.g., add a label) via TFC
3. **State consistency**: Verify TFC reads the same GCS remote state
4. **Credential scoping**: Verify workspace attribute conditions prevent cross-workspace impersonation

### Phase 5: Scripts & Tooling

1. **`scripts/infra.py`** — extend for TFC workspace generation:
   - Add workspace entries to `hcp-terraform/gcp-hcp-{env}/main.tf`
   - Add `gcp-dynamic-creds` module calls to bootstrap config for new projects
   - Same gate: `environment in ['integration', 'stage', 'production'] and sector != 'e2e'`

2. **Design docs** — update in this repo:
   - `design-decisions/automation/hcp-terraform-workload-identity-federation.md`
   - `design-decisions/automation/hcp-terraform-workspace-architecture.md`

### Phase 6: Repeat for Stage

Same Phases 0–4 for stage:
- Target projects: `gcp-hcp-stg-global`, `stg-reg-us-w1-{id}`, `stg-mgt-us-w1-zkpf`
- New bootstrap config: `terraform/config/tfc-bootstrap/stage/`
- New workspace definitions: `hcp-terraform/gcp-hcp-stg/`

### Phase 7: Cutover & Decommission Atlantis

1. Disable Atlantis autoplan (ArgoCD sync disabled or webhook removed)
2. Validate TFC handles all PRs for ~1 week
3. Remove Atlantis ArgoCD application (`argocd/config/global/atlantis/`)
4. Remove Atlantis Helm chart (`helm/charts/atlantis-stack/`)
5. Remove Atlantis SA and IAM bindings from all modules
6. Remove `atlantis-{env}.yaml` files
7. Migrate bootstrap config to its own TFC workspace (in `gcp-hcp-bootstrap` TFC project)
8. Update mintmaker agent (`agent/mintmaker/tools/atlantis.py` → TFC equivalent)

## PR Sequence (Integration)

| # | PR | Applied By | Depends On |
|---|---|---|---|
| 1 | Bootstrap IAM: 5 roles for `tfc-automation` on int projects | Atlantis | — |
| 2 | TFC bootstrap config: `gcp-dynamic-creds` module calls | Atlantis | PR 1 |
| 3 | Supplemental IAM: folder roles + commons grants | Atlantis + SRE | PR 2 |
| 4 | TFC workspace definitions | TFC meta workspace | PR 2 |
| 5 | Validation + small change test | TFC | PR 3 + PR 4 |
| 6 | `scripts/infra.py` extension | N/A (tooling) | PR 4 |

## Key Risks and Mitigations

| Risk | Mitigation |
|---|---|
| IAM propagation delay on new bootstrap roles | Split IAM PRs from resource PRs; re-apply after ~60s if first apply fails |
| Circular dependency (module poisons its own workspace) | Bootstrap config lives in Atlantis (not TFC) during bootstrap phase |
| Variable set precedence conflict | Ensure per-project variable sets don't conflict with workspace-level variables; validate after attachment |
| State file locking during parallel Atlantis+TFC | Only one system should apply at a time during validation; use TFC speculative plans |
| Commons module requires SRE manual apply | Coordinate with SRE team; include in phase sequencing |

## References

- [Atlantis IAM Audit](https://redhat.atlassian.net/browse/GCP-536) — full role inventory across all modules
- [HCP Terraform WIF Playground](../experiments/terraform-automation-tools/hcp-terraform-wif-playground.md) — experiment validation
- [HCP Terraform WIF Design Decision](../design-decisions/automation/hcp-terraform-workload-identity-federation.md)
- [HCP Terraform Workspace Architecture](../design-decisions/automation/hcp-terraform-workspace-architecture.md)
- [gcp-dynamic-creds module](https://app.terraform.io/app/hp-platform-engineering/registry/modules/private/hp-platform-engineering/gcp-dynamic-creds/tfe/0.0.14) — TFC private registry
