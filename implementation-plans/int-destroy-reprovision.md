# Destroy and Reprovision Integration Region + Management Cluster

## Overview

Tear down the integration region (`int-reg-us-c1-nkcw`) and management cluster (`int-mgt-us-c1-yjiv`), then reprovision them with new infra IDs. The global cluster (`gcp-hcp-int-global`) stays intact. Atlantis is frozen during the destroy/cleanup phases to prevent automated applies from interfering; cleanup and destroy run locally, then Atlantis is re-enabled for provisioning.

**Repo**: `gcp-hcp-infra`

**Related Design Decisions:**
- [Terraform Automation Tooling](../design-decisions/automation/terraform-automation-tooling.md) — Atlantis PR-based workflow
- [Terraform Code Structure](../design-decisions/automation/terraform-code-structure.md) — region state separation rationale
- [GCP Folder and Project Hierarchy](../design-decisions/infrastructure/gcp-folder-project-hierarchy.md) — project naming conventions (`{env}-{type}-{region}-{id}`)

---

## Phase 1: Prepare for Destroy (Atlantis PR)

### Summary

Disable deletion protection on the region and MC GKE clusters, GCP folders, and GCP projects so Terraform can destroy them.

### Tasks

- [ ] Edit `terraform/config/region/integration/main/us-central1/main.tf` line 93: change `deletion_protection = true` to `false` on the GKE cluster
- [ ] In the same region `main.tf`, set `deletion_protection` to `false` on the `google_folder` resource and `deletion_policy = "DELETE"` on the `google_project` resource
- [ ] Edit `terraform/config/management-cluster/integration/main/us-central1-yjiv/main.tf` line 89: change `deletion_protection = true` to `false` on the GKE cluster
- [ ] In the same MC `main.tf`, set `deletion_protection` to `false` on the `google_folder` resource and `deletion_policy = "DELETE"` on the `google_project` resource
- [ ] Push PR, let Atlantis plan, then `atlantis apply` both projects

### Acceptance Criteria

- [ ] Atlantis apply succeeds for both region and MC projects
- [ ] GKE clusters, GCP folders, and GCP projects have deletion protection disabled

---

## Phase 2: Pre-Destroy Cleanup (Manual kubectl/gcloud)

### Summary

Freeze Atlantis to prevent automated applies from interfering with the destroy sequence. Delete all hosted clusters to free PrivateServiceConnect endpoints and subnets. Then clean up remaining in-cluster and GCP resources that must be removed before `terraform destroy`. These mirror the e2e cleanup pipeline in `gcp-hcp-infra` (`pipelines/tasks/cleanup-infrastructure.yaml`). Requires `kubectl` access to both clusters and `gcloud` access to the region project.

### Tasks

- [ ] Freeze Atlantis automated applies for the integration environment. Lock both projects via the Atlantis UI (`/locks` page) or by commenting `atlantis unlock` on any open PRs that target these projects:
  - `region-int-main-us-central1`
  - `mc-int-main-us-central1-yjiv`
  - Verify in the Atlantis UI that no plans or applies are queued for either project before proceeding
- [ ] Get cluster credentials:
  ```bash
  gcloud container clusters get-credentials int-mgt-us-c1-yjiv-gke \
    --project int-mgt-us-c1-yjiv --region us-central1
  gcloud container clusters get-credentials int-reg-us-c1-nkcw-gke \
    --project int-reg-us-c1-nkcw --region us-central1
  ```
- [ ] Disable Fleet Config Management on the **region** project (prevents Config Sync from reverting resource deletions during teardown):
  ```bash
  gcloud beta container fleet config-management disable \
    --project int-reg-us-c1-nkcw --force
  ```
- [ ] Disable Fleet Config Management on the **MC** project:
  ```bash
  gcloud beta container fleet config-management disable \
    --project int-mgt-us-c1-yjiv --force
  ```
- [ ] Scale down `config-management-system` on **both** clusters:
  ```bash
  # Region cluster
  kubectl --context gke_int-reg-us-c1-nkcw_us-central1_int-reg-us-c1-nkcw-gke \
    scale deployment -n config-management-system --all --replicas=0
  # MC cluster
  kubectl --context gke_int-mgt-us-c1-yjiv_us-central1_int-mgt-us-c1-yjiv-gke \
    scale deployment -n config-management-system --all --replicas=0
  ```
- [ ] Delete ArgoCD Applications on **MC cluster** in order (root first to prevent re-sync of deleted resources): root Application, all ApplicationSets, then remaining Applications (14 apps: alerting-policies, argocd-config, cert-manager, config-connector, hypershift, maestro-agent, prometheus-*, etc.)
- [ ] Delete ArgoCD Applications on **Region cluster**: same process (28 apps: cls-*, hyperfleet-*, maestro-server, shared-gateway, etc.)
- [ ] Clean up Config Connector resources on both clusters: delete all CNRM custom resources and wait for finalizers (up to 20 min)
- [ ] Clean up Gateway API / Ingress / NEGs / DNS:
  - Delete Gateway API resources (GCPBackendPolicy, HealthCheckPolicy, HTTPRoute, Gateway)
  - Delete all Ingresses and LoadBalancer Services
  - Wait for NEG cleanup, then manually delete remaining NEGs
  - Clean up DNS records from the region's DNS zones
- [ ] Delete all hosted clusters (HostedClusters / NodePools) on the MC cluster — this frees PrivateServiceConnect endpoints and subnets:
  ```bash
  kubectl --context gke_int-mgt-us-c1-yjiv_us-central1_int-mgt-us-c1-yjiv-gke \
    delete hostedclusters --all -A
  kubectl --context gke_int-mgt-us-c1-yjiv_us-central1_int-mgt-us-c1-yjiv-gke \
    delete nodepools --all -A
  # Wait for hosted cluster resources and PSC endpoints to be fully cleaned up
  ```
- [ ] Clean up PSC Service Attachments in the MC project (should be mostly gone after hosted cluster deletion)

### Acceptance Criteria

- [ ] Atlantis projects are locked for integration environment
- [ ] All hosted clusters and node pools are fully deleted
- [ ] ArgoCD applications are fully deleted on both clusters
- [ ] Config Connector resources are gone (no pending finalizers)
- [ ] No orphaned NEGs, Service Attachments, PSC endpoints, or LoadBalancer Services remain
- [ ] DNS records are cleaned from region DNS zones

---

## Phase 3: Terraform Destroy (Manual CLI)

### Summary

Run `terraform destroy` manually in dependency order. MC must be destroyed first because it creates cross-project resources (Maestro SA, Pub/Sub subscriptions, CLS registration secret, fleet membership, cross-project IAM bindings).

### Prerequisites

- The identity running `terraform destroy` has **DNS Admin** on `gcp-hcp-int-global` (the region module creates NS delegation records in the global project's DNS zone)

### Tasks

- [ ] Verify the identity running destroy has effective **DNS Admin** access on `gcp-hcp-int-global`:
  ```bash
  # Confirm which account will run terraform destroy
  gcloud config get-value account

  # Verify effective DNS access with a read-only operation against the target zone
  gcloud dns managed-zones list --project=gcp-hcp-int-global --format="table(name)"
  ```
  If the DNS command fails with a permission error, the active identity lacks DNS Admin — resolve before proceeding with destroy
- [ ] Destroy Management Cluster **first**:
  ```bash
  cd terraform/config/management-cluster/integration/main/us-central1-yjiv
  terraform init
  terraform destroy -auto-approve
  ```
- [ ] Destroy Region **second**:
  ```bash
  cd terraform/config/region/integration/main/us-central1
  terraform init
  terraform destroy -auto-approve
  ```
- [ ] Handle expected failures and retry (up to 8 times per the e2e pipeline):
  - Force-unlock stale state locks — **only after verifying no active Atlantis/Terraform runs**:
    1. Check for active runs: `atlantis status` and inspect GCS lock metadata
    2. Record the lock ID from the error message
    3. Confirm no `terraform apply/destroy` is in progress
    4. Run `terraform force-unlock <LOCK_ID>`
  - Manually delete orphaned NEGs and Service Attachments
  - Re-enable disabled APIs (`compute.googleapis.com`, `container.googleapis.com`)
  - Clear DNS zone records before zone deletion
  - If failures persist after these remediation steps, escalate to the infrastructure team to investigate orphaned resources or IAM issues before continuing

### Acceptance Criteria

- [ ] MC project `int-mgt-us-c1-yjiv` and all its resources are destroyed
- [ ] Region project `int-reg-us-c1-nkcw` and all its resources are destroyed
- [ ] NS delegation records in global DNS zone are removed
- [ ] No orphaned GCP resources remain in either project

---

## Phase 4: Clean Up Old Config (PR to main)

### Summary

Remove old configuration files, infra ID entries, Atlantis project entries, and Terraform state. Then apply global Terraform to drop stale monitoring scoped projects.

### Tasks

- [ ] Delete old region config directory: `terraform/config/region/integration/main/us-central1/` (path has no infra ID — old `main.tf` would be reused by `infra.py new region` if not deleted)
- [ ] Delete old MC config directory: `terraform/config/management-cluster/integration/main/us-central1-yjiv/`
- [ ] Remove old Atlantis entries from `terraform/atlantis-integration.yaml`:
  - `mc-int-main-us-central1-yjiv`
  - `region-int-main-us-central1`
- [ ] Remove old infra ID entries (`nkcw` and `yjiv`) from:
  - `terraform/metadata/infra_ids.yaml`
  - `helm/charts/gitops-promoter-config-region/metadata/infra_ids.yaml`
- [ ] Delete old state prefixes from GCS bucket `gcp-hcp-int-global-terraform-state`:
  ```bash
  gsutil -m rm -r gs://gcp-hcp-int-global-terraform-state/region/main/us-central1/
  gsutil -m rm -r gs://gcp-hcp-int-global-terraform-state/management-cluster/main/us-central1-yjiv/
  ```
  The region prefix **must** be deleted — it has no infra ID, so the new region would pick up stale state referencing deleted resources
- [ ] Merge PR (no Atlantis action needed — only removing config)
- [ ] Apply global Terraform to drop stale monitoring scoped projects:
  ```bash
  atlantis apply -p global-int-main-us-central1
  ```
  This removes `google_monitoring_monitored_project` entries for the deleted projects

**Note**: ArgoCD config does NOT need cleanup. Targets are keyed by `environment/sector/region`, not infra ID. The `infra.py new` step in Phase 5 handles any ArgoCD updates.

### Acceptance Criteria

- [ ] Old region config directory removed
- [ ] Old MC config directory removed
- [ ] Old Atlantis project entries removed
- [ ] Old infra IDs removed from both metadata files
- [ ] Old Terraform state deleted from GCS
- [ ] Global Terraform applied — stale monitoring scoped projects removed

---

## Phase 5: Generate New Infra IDs and Configs (Automated)

### Summary

Use the bootstrap scripts to generate new infra IDs and configuration for the region and management cluster.

### Tasks

- [ ] Generate new region infra ID and config:
  ```bash
  ./scripts/infra.py new region integration main us-central1
  ```
- [ ] Generate new MC infra ID and config:
  ```bash
  ./scripts/infra.py new management-cluster integration main us-central1
  ```
- [ ] Verify `infra.py` outputs:
  - New infra IDs registered in `terraform/metadata/infra_ids.yaml`
  - New config directories created from templates
  - `terraform init && terraform validate` passed
  - New Atlantis project entries added to `terraform/atlantis-integration.yaml`
  - `argocd/config/` config.yaml files updated with new render targets
  - ArgoCD render script executed
- [ ] Copy new infra ID entries to `helm/charts/gitops-promoter-config-region/metadata/infra_ids.yaml`

### Acceptance Criteria

- [ ] New infra IDs generated and registered
- [ ] New Terraform config directories exist and validate
- [ ] Atlantis config updated with new project entries
- [ ] ArgoCD config updated with new render targets
- [ ] Gitops-promoter metadata updated

---

## Phase 6: Provision via Atlantis (PR)

### Summary

Push new configs as a PR and provision via Atlantis. Region must be applied before MC because MC reads from region's state (`terraform_remote_state`).

### Tasks

- [ ] Push new configs as a PR to `main`
- [ ] Atlantis auto-plans both new projects
- [ ] Apply region first: `atlantis apply -p region-int-main-us-central1`
- [ ] Apply MC second: `atlantis apply -p mc-int-main-us-central1-<new-id>`
- [ ] Apply global to register new monitoring scoped projects: `atlantis apply -p global-int-main-us-central1`

### Acceptance Criteria

- [ ] Region and MC Terraform apply succeeded
- [ ] GKE clusters are provisioned and healthy
- [ ] Global monitoring scoped projects include new project IDs

---

## Phase 7: Post-Provision Setup and Verification

### Summary

Populate secrets, let ArgoCD sync, and verify the new infrastructure.

### Tasks

- [ ] Re-enable Fleet Config Management on the new **region** project:
  ```bash
  gcloud beta container fleet config-management enable \
    --project <new-region-project-id>
  ```
- [ ] Re-enable Fleet Config Management on the new **MC** project:
  ```bash
  gcloud beta container fleet config-management enable \
    --project <new-mc-project-id>
  ```
- [ ] Verify `config-management-system` pods are running on both new clusters
- [ ] Unlock Atlantis automated applies for the new integration projects via the Atlantis UI (`/locks` page) — verify both projects accept new plan/apply operations
- [ ] Manually populate secrets (documented in `terraform/config/global/integration/SETUP.md`):
  - `argocd-repo-creds` (SSH deploy key)
  - `argocd-notifications-github-key`
  - Any other secrets in the new region/MC projects
- [ ] Verify ArgoCD applications sync automatically via Fleet / Config Sync on the global cluster
- [ ] Verify GKE clusters are healthy
- [ ] Verify DNS delegation is working (global -> region zones)
- [ ] Verify Config Connector resources are reconciling
- [ ] Verify Maestro agent is connected (Pub/Sub subscriptions active)

### Acceptance Criteria

- [ ] Fleet Config Management re-enabled and `config-management-system` running
- [ ] Atlantis projects unlocked and operational
- [ ] All secrets populated
- [ ] ArgoCD applications are synced and healthy
- [ ] DNS resolution works end-to-end
- [ ] Config Connector resources reconciling without errors
- [ ] Maestro Pub/Sub subscriptions active

---

## PRs Summary

| PR | Purpose | Atlantis Action |
|----|---------|-----------------|
| PR 1 (Phase 1) | Disable `deletion_protection` on region + MC | `atlantis apply` both projects |
| PR 2 (Phase 4) | Remove old configs, infra IDs, Atlantis entries | Merge, then `atlantis apply` global to drop stale monitoring |
| PR 3 (Phase 5+6) | Add new configs from `infra.py` + new infra IDs | `atlantis apply` region, then MC, then global (monitoring) |

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| GCP project IDs held 30 days after deletion | New infra IDs generate new project IDs — no waiting needed |
| DNS zone NS records change | Region module auto-creates NS delegation in global zone; global DNS is preserved. **Identity running `terraform destroy` needs DNS Admin on `gcp-hcp-int-global`** |
| WIF pool IDs reused | New infra IDs = new pool IDs, no conflict |
| State bucket is in global project | Global stays intact, bucket is preserved |
| Cross-project resources (MC -> region) | Destroying MC first ensures clean removal |
| Region state prefix reuse | Region state prefix is `region/main/us-central1` (no infra ID). **Must be deleted** from GCS before reprovisioning |
| Region config directory unchanged | Region config dir path has no infra ID — `infra.py` recreates it from template. Old `main.tf` must be deleted first (Phase 4) |
| Atlantis automated applies during destroy | Lock Atlantis projects via UI in Phase 2 before any cleanup. Unlock via UI in Phase 7 after reprovision is complete |
| Hosted clusters hold PSC endpoints and subnets | Delete all HostedClusters/NodePools in Phase 2 before subnet/PSC cleanup — frees PrivateServiceConnect endpoints and allows clean subnet teardown |
| Monitoring scoping | Global project's `google_monitoring_monitored_project` references old project IDs. Global Terraform must be applied in **Phase 4** (to drop stale scoped projects after removing old IDs) and again in **Phase 6** (to register new scoped projects after adding new IDs) |
