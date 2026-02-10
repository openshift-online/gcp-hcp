# GKE Fleet Management for Cluster Bootstrap and SRE Access

***Scope***: GCP-HCP

**Date**: 2026-02-10

## Decision

We will use GKE Fleet management features — specifically Fleet membership, Config Sync, and Connect Gateway — to bootstrap cluster workloads without requiring external network access to the Kubernetes API, and to provide SRE kubectl access to private clusters without public DNS endpoints.

## Context

The GCP HCP infrastructure operates private GKE clusters (region and management clusters) that have no public API endpoints. Two operational challenges arise from this design:

- **Problem Statement**: (1) How to bootstrap initial workloads (ArgoCD, External Secrets Operator) onto newly provisioned private GKE clusters when Terraform cannot reach the Kubernetes API, and (2) how to provide SRE teams with kubectl access to these clusters for operational tasks without exposing public endpoints or maintaining bastion hosts.
- **Constraints**: No public DNS endpoints on cluster APIs. No cross-cluster network connectivity between region and management clusters. Terraform cannot use `kubernetes` or `helm` providers against private clusters. Bootstrap content must be cluster-agnostic (no per-cluster templating at sync time). Must align with the project's regional independence architecture.
- **Assumptions**: Bootstrap manifests are static and contain no secrets (secrets are injected at runtime via External Secrets Operator and GCP Secret Manager). SRE access is infrequent and does not require persistent connections. GKE Fleet features (Config Sync, Connect Gateway) are included with GKE at no additional cost.

## Alternatives Considered

1. **GKE Fleet (Config Sync + Connect Gateway)**: Register clusters in regional fleets via Terraform. Use Config Sync to sync bootstrap manifests from a public Git repository. Use Connect Gateway for SRE kubectl access through the Fleet control plane.
2. **Cloud Run v2 Job for bootstrap + VPN/bastion for access**: Run a container in the cluster's VPC during `terraform apply` to execute kubectl/helm commands. Maintain bastion hosts or VPN for SRE access.
3. **Authorized networks with public DNS endpoints**: Keep public API endpoints restricted by IP allowlists for both Terraform bootstrap and SRE access.

## Decision Rationale

* **Justification**: GKE Fleet provides a native, fully managed solution for both problems using a single mechanism (fleet membership). Config Sync bootstraps clusters without any network path from Terraform to the cluster API — Terraform only interacts with the GCP Fleet API. Connect Gateway provides authenticated kubectl access through the GKE Hub control plane without VPN, bastion, or public endpoints. Regional fleets align with the project's regional independence architecture, limiting blast radius and enabling independent Config Sync management per cluster.
* **Evidence**: Config Sync is configured entirely via Terraform (`google_gke_hub_feature_membership`) with no kubectl calls. The bootstrap repo is public and cluster-agnostic, so no authentication or per-cluster templating is needed at sync time. Connect Gateway supports full kubectl operations (including exec and port-forward on GKE 1.30+) with native IAM-based access control and Cloud Audit Logs. Config Sync and Connect Gateway are included with GKE at no additional cost.
* **Comparison**: Cloud Run v2 Jobs would require building and maintaining a bootstrap container image, VPC access configuration, and a separate mechanism for SRE access (bastion or VPN with ongoing maintenance costs). Authorized networks with public endpoints contradict the project's security posture of eliminating external API access entirely and introduce IP management overhead.

## Consequences

### Positive

* Zero kubectl access required during cluster provisioning — Terraform interacts only with GCP APIs
* No cross-cluster network connectivity needed — each cluster bootstraps itself via Fleet
* SRE access to private clusters without bastion hosts, VPN, or public endpoints
* Regional fleets limit blast radius and align with regional independence architecture
* Per-cluster Config Sync revision pinning enables progressive rollout of bootstrap changes (e.g., canary a new ArgoCD version in dev before rolling to production)
* Regional fleets enable progressive rollout of GKE version upgrades via fleet chaining
* No additional infrastructure to maintain for either bootstrap or access
* Native Cloud Audit Logs for all Connect Gateway access

### Negative

* Config Sync cannot template cluster-specific values — cluster-specific configuration must be handled separately (solved via GCP Secret Manager + External Secrets Operator)
* Private clusters require Cloud NAT for Config Sync to reach the public Git repository
* Bootstrap content in the public repository is visible (acceptable since it contains no secrets — only operator manifests and CRDs)
* Connect Gateway sessions have time limits (20-30 minutes) — not suitable for long-running operations
* Dependency on GKE Fleet service availability for both bootstrap and SRE access

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Regional fleets support up to 250 global memberships and 50 regional memberships per fleet (soft limits, can be increased). Each regional fleet contains 1 region cluster plus N management clusters. Config Sync manages only static bootstrap components, so changes are infrequent.
* **Observability**: Config Sync status is visible via `gcloud` and Cloud Console. Connect Gateway access is logged in Cloud Audit Logs with principal identity, method, and timestamp.
* **Resiliency**: Regional fleet isolation prevents a fleet misconfiguration from affecting other regions. Each cluster's Config Sync operates independently — a Git repository outage delays syncing but does not affect running workloads.

### Security:
* Connect Gateway access is controlled via IAM roles (`roles/gkehub.gatewayAdmin`, `roles/gkehub.viewer`) scoped to the regional project, which also governs management cluster access since MCs register in the region's fleet
* All Connect Gateway kubectl sessions produce Cloud Audit Log entries (who, what, when)
* No static credentials, VPN keys, or SSH keys needed for cluster access
* Can be combined with Google Privileged Access Manager (PAM) in the future for time-limited, justification-required access grants
* Config Sync uses `secret_type = "none"` (public repo) — no Git credentials stored in clusters
* Bootstrap manifests contain no secrets; runtime secrets are injected via Workload Identity and Secret Manager

### Cost:
* Fleet management, Config Sync, and Connect Gateway are included with GKE at no additional cost
* Eliminates costs of bastion host VMs, VPN infrastructure, and their ongoing maintenance
* Cloud NAT is required for private clusters to reach the public bootstrap repository (standard NAT pricing applies, but Cloud NAT is already required for other outbound traffic)

### Operability:
* Fleet membership is configured declaratively in Terraform — no manual registration steps
* Config Sync revision is a Terraform variable (`configsync_git_revision`), enabling version pinning per cluster and progressive rollout across environments
* The Config Management feature (`google_gke_hub_feature`) is created once in the region module; management clusters reference it via cross-project fleet membership, keeping the fleet topology flat within each region
* SRE access via Connect Gateway uses standard `gcloud container fleet memberships get-credentials` followed by regular kubectl — no special tooling required
* Teardown requires disabling Config Management before destroying clusters to prevent Config Sync from reverting deletions
