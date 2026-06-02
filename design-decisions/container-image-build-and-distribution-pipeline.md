# Container Image Build and Distribution Pipeline

***Scope***: GCP-HCP

**Date**: 2026-05-27

## Decision

Adopt Konflux as the standard build system for GCP HCP container images, publish to Google Artifact Registry (GAR) using Workload Identity Federation (WIF) for keyless authentication, distribute images to regional GKE clusters via pull-through caches, and transparently rewrite image URLs at pod admission using MutatingAdmissionPolicy.

## Context

GCP HCP requires a secure, automated pipeline to build container images, publish them to a registry accessible by GKE clusters across multiple regions and projects, and distribute those images efficiently. The solution must integrate with Red Hat's standard build infrastructure (Konflux), avoid long-lived credentials, and work across the multi-project GCP infrastructure where region and management-cluster projects are provisioned dynamically.

- **Problem Statement**: Container images need to flow from source code to running pods across a multi-project, multi-region GKE infrastructure. The pipeline must handle cross-project authentication without service account keys, provide regional resilience for image pulls, and require zero per-application configuration for new images.

- **Constraints**:
  - Konflux is Red Hat's mandated build system for container images — builds must run there, not in GCP
  - GCP HCP infrastructure spans multiple projects (commons, global, region, management-cluster) with strict IAM boundaries
  - No service account keys — all cross-project authentication must use Workload Identity Federation or native GKE identity
  - Customer worker nodes run in separate customer-owned GCP projects
  - GKE Autopilot clusters require beta API enablement for MutatingAdmissionPolicy

- **Assumptions**:
  - The `gcp-hcp-commons` project is the single shared project for cross-environment resources (GAR, WIF, DNS)
  - Each region project has its own GKE cluster(s) where images are consumed
  - Management cluster GKE nodes pull from their parent region's pull-through cache

## Architecture Overview

The pipeline has four layers:

```text
Layer 1: Build (Konflux)
  Code commit → Konflux build → OCI image in Quay

Layer 2: Publish (Konflux → GAR via WIF)
  Quay image → cosign copy → GAR commons repo (us-docker.pkg.dev/gcp-hcp-commons/gcp-hcp-images)

Layer 3: Distribute (Regional GAR pull-through caches)
  Commons GAR → regional cache (gcp-hcp-images)     — our images
  quay.io     → regional cache (quay-cache)          — upstream OCP images

Layer 4: Consume (MutatingAdmissionPolicy)
  Pod references source URL → MAP rewrites to regional cache → kubelet pulls from cache
```

### Image Distribution Strategy

Images are categorized into three tiers based on ownership, each with a different path to the regional cache:

| Tier | Source | Path to GAR | Regional Cache |
|------|--------|-------------|----------------|
| **Tier 1: Team-owned** | Konflux build → GAR commons | Konflux → WIF → `us-docker.pkg.dev/gcp-hcp-commons/gcp-hcp-images` | Pull-through from commons GAR |
| **Tier 2: Partner-owned** | Konflux build → GAR commons | Same as Tier 1 (partner teams publish direct to our GAR) | Pull-through from commons GAR |
| **Tier 3: Upstream** | All external registries (quay.io, ghcr.io, registry.redhat.io, gcr.io, etc.) | Remain in source registries (not republished) | Pull-through cache per upstream registry |

**Tier 1 and 2** images are published directly to our GAR commons repository. They flow through the same `gcp-hcp-images` pull-through cache. Partner teams (HyperShift, HyperFleet) will update their Konflux release pipelines to publish to our GAR alongside their existing Quay targets.

**Tier 3** covers all images we consume but don't build — OCP components, operators, controllers, GKE system components, Config Connector, and any other upstream dependency. These stay in their source registries and are not republished. Instead, a pull-through cache is created in each region project for each upstream registry. GAR remote repositories are scoped to a single upstream, so each distinct registry gets its own cache. Registries that require authentication store their pull secrets in Secret Manager in the global project. Every upstream registry is cached, including Google-managed registries — regions outside the US (e.g., EU) must not depend on cross-region pulls for any image.

All cache types are consumed identically: MutatingAdmissionPolicy rewrites source URLs to the regional cache at pod admission time. A CEL mutation is configured per source registry prefix. Adding a new upstream registry requires creating a new cache repo and adding a `sourceRepos` entry to the image rewriter chart.

### Regional Cache Architecture

Each region project hosts pull-through cache repositories — one for our own images and one per upstream registry that needs caching:

```text
Region Project ({region}-docker.pkg.dev/{region-project}/)
├── gcp-hcp-images/       ← pull-through from commons GAR (Tier 1 + 2, no upstream auth)
├── {registry}-cache/     ← pull-through from each upstream registry (Tier 3)
│                            One cache per upstream registry, some require auth
└── ...                      Additional caches added as new upstream registries are needed
```

Management clusters pull from their parent region's caches via project-level `artifactregistry.reader` IAM — the same pattern used for `gcp-hcp-images`.

## Shared Pipeline Evaluation

The `app-sre/shared-pipelines` pattern was evaluated as a potential model. In that pattern, a dedicated repository holds pipeline definitions and components reference them via `pipelineRef`. We adopted this approach: our GAR push pipeline (`push-snapshot-to-gar`) is contributed to the [Konflux community catalog](https://github.com/konflux-ci/community-catalog) and referenced via Tekton's git resolver from the Konflux release plan. This provides the same benefits — shared pipeline definitions, centralized updates, community visibility — while following Konflux's standard contribution model. The standard Quay push path uses Konflux's managed `rh-push-to-external-registry` pipeline from `release-service-catalog`, which is itself a shared pipeline maintained by the releng team.

## Alternatives Considered

1. **Build in GCP (Cloud Build + GAR)**: Build images using Cloud Build in the commons project and push directly to GAR. Avoids the Konflux-to-GAR bridge but doesn't meet Red Hat's requirement for Konflux as the standard build system.

2. **Konflux → Quay only (no GAR)**: Build in Konflux, publish to Quay.io, and have GKE nodes pull from Quay. Simpler pipeline but requires GKE nodes to have Quay credentials (a file-on-disk credential management problem) and offers no regional caching.

3. **Konflux → GAR with service account keys**: Publish to GAR using a downloaded service account key stored as a Konflux secret. Avoids WIF complexity but violates the no-keys policy and creates a key rotation burden.

4. **Konflux → GAR with WIF + direct pulls (no cache)**: Publish to the commons GAR repo and have all clusters pull directly from it. Simpler infrastructure but creates a single point of failure and cross-region bandwidth costs.

5. **Konflux → GAR with WIF + pull-through cache + ArgoCD image overrides**: Use ArgoCD Helm value overrides to point each app at the regional cache. Requires per-app configuration in every ArgoCD template for every image — N charts x M images = significant wiring.

## Decision Rationale

* **Justification**: The chosen approach (option 5 variant with MAP instead of ArgoCD overrides) provides the best combination of security, resilience, and operational simplicity. Konflux builds satisfy Red Hat's standard. WIF eliminates credential management entirely. Pull-through caches provide regional resilience and reduce cross-region bandwidth. MutatingAdmissionPolicy provides transparent image rewriting at a single chokepoint instead of N configuration points.

* **Evidence**: 
  - The AWS ECR credential incident ([ITN-2026-00006](https://issues.redhat.com/browse/OHSS-49424)) demonstrated that file-on-disk credential management for image pulls is brittle. WIF eliminates this failure mode entirely — there are no credential files to overwrite.
  - GCP's pull-through cache mode for Artifact Registry serves cached images even when the upstream is unavailable, providing genuine regional resilience.
  - MutatingAdmissionPolicy (v1beta1) is a production-ready Kubernetes API that evaluates CEL expressions in-process — no webhook server to maintain or monitor.

* **Comparison**:
  - **Cloud Build** (alternative 1): Rejected — does not meet Konflux mandate.
  - **Quay only** (alternative 2): Rejected — requires credential files on GKE nodes, creating the same failure mode as the AWS ECR incident. No regional caching.
  - **Service account keys** (alternative 3): Rejected — violates security policy.
  - **Direct pulls** (alternative 4): Rejected — no regional resilience, cross-region bandwidth costs scale with cluster count.
  - **ArgoCD overrides** (alternative 5 original): Rejected — requires per-app configuration. MAP is zero-touch for new images.

## Implementation Details

### Konflux Tenant Configuration

A dedicated Konflux tenant (`gcp-hcp-tenant`) manages all GCP HCP container images. Each image is registered as a Konflux application with a component that points to its source repository and Containerfile. Examples of current applications:

| Application | Source | Dockerfile | Visibility |
|---|---|---|---|
| `gcp-hcp-common-tools` | `openshift-online/gcp-hcp-infra` | `images/tools/Containerfile` | Private |
| `gcp-release-utils` | `openshift-online/gcp-hcp-infra` | `images/release-utils/Containerfile` | Public |

Additional images are onboarded following the same pattern. All applications use the `docker-build-oci-ta` build pipeline with builds triggering on commits to the `main` branch.

### Dual Release Path

Each application has two release paths:

1. **GAR Push** (tenant pipeline): Copies images from Quay to `us-docker.pkg.dev/gcp-hcp-commons/gcp-hcp-images/{image}` using `cosign copy` (preserving signatures and attestations) with WIF authentication. Runs as `gar-release-sa` in the `gcp-hcp-tenant` namespace.

2. **Quay Push** (managed pipeline): Pushes images to `quay.io/redhat-services-prod/gcp-hcp-tenant/{image}` via the standard `rh-push-to-external-registry` release pipeline. This provides a public distribution endpoint.

### Workload Identity Federation

The GAR push pipeline authenticates using WIF — no service account keys exist anywhere in the system:

1. Konflux pod mounts a projected Kubernetes service account OIDC token (audience: `gcp-hcp-konflux`)
2. Token is exchanged at Google Security Token Service (STS) against the `konflux-pool` WIF pool in `gcp-hcp-commons`
3. STS returns a short-lived credential that impersonates `konflux-push@gcp-hcp-commons.iam.gserviceaccount.com`
4. `docker-credential-gcr` uses this credential transparently to push images to GAR

The WIF provider is configured with an attribute condition restricting access to namespace `gcp-hcp-tenant` and service account `gar-release-sa` only.

### GAR Commons Repository

A single multi-region (`us`) Artifact Registry repository in the `gcp-hcp-commons` project serves as the source of truth:

- **Repository**: `us-docker.pkg.dev/gcp-hcp-commons/gcp-hcp-images`
- **IAM**: `konflux-push` SA has `artifactregistry.writer`; Atlantis and e2e-deployer SAs have `artifactregistry.admin` (for managing pull-through cache IAM bindings)
- **Lifecycle**: `prevent_destroy = true` on the repository, WIF pool, and `konflux-push` SA

### Regional Pull-Through Cache

Each region project creates a pull-through cache repository that transparently proxies from the commons GAR repository:

- **Module**: `terraform/modules/gar-pull-through-cache`
- **Repository**: `{region}-docker.pkg.dev/{region-project}/gcp-hcp-images` (mode: `REMOTE_REPOSITORY`)
- **IAM**: The AR service agent in the cache project (`service-{project_number}@gcp-sa-artifactregistry.iam.gserviceaccount.com`) is granted `artifactregistry.reader` on the commons repo
- **Cleanup**: 365-day retention policy (configurable via `cache_retention_days`). Based on upload time since GCP does not support last-pull conditions. Deleted images are re-fetched from the source on next pull.
- **Resilience**: Once an image is cached locally, it serves from the cache even if the upstream repository is unavailable.

IAM propagation is handled with explicit `time_sleep` resources (30s for AR service agent provisioning, 60s for IAM propagation before cache validation).

### Management Cluster Access

Management cluster GKE nodes access the region's pull-through cache via project-level IAM:

- **Binding**: `roles/artifactregistry.reader` on the region project, granted to the MC's GKE node service account (`local.gke_cluster.service_account`)
- **Pattern**: Project-level binding (not repository-level) to avoid race conditions with the region's GAR cache creation during parallel Atlantis/E2E applies

### MutatingAdmissionPolicy Image Rewriter

A Helm chart (`gar-image-rewriter`) deploys a MutatingAdmissionPolicy to both region and management-cluster GKE clusters that transparently rewrites image URLs:

- **Input**: `us-docker.pkg.dev/gcp-hcp-commons/gcp-hcp-images/tool:v1`
- **Output**: `us-central1-docker.pkg.dev/int-reg-us-c1-nkcw/gcp-hcp-images/tool:v1`
- **API**: `admissionregistration.k8s.io/v1beta1` with CEL-based `ApplyConfiguration` mutations
- **Scope**: All pods, all namespaces (opt-out via annotation `gcp-hcp/skip-image-rewrite: "true"`)
- **Target**: Pod CREATE operations only — ArgoCD sees no drift because Deployments/StatefulSets are not mutated
- **Configuration**: Extensible via `sourceRepos[]` list in `values.yaml`. Adding a new source repo requires only a values change and a matching pull-through cache in the region module.
- **Deployment**: ArgoCD sync-wave `-5` (hard dependency, must exist before wave-0 workloads create pods)

### Onboarding Checklist for New Images

To onboard a new container image to this pipeline:

1. **Image source**: Add `images/{name}/Containerfile` to `gcp-hcp-infra` (or the appropriate source repo)
2. **Konflux application**: Create application and component resources in `konflux-release-data` under `tenants-config/cluster/kflux-prd-rh02/tenants/gcp-hcp-tenant/applications/{name}/`
   - `application.yaml` — Application resource
   - `components/{name}.yaml` — Component with git source, Dockerfile path, and build pipeline (`docker-build-oci-ta`)
   - `components/image-repository.yaml` — Image repository for build artifacts
   - `release-plan.yaml` — ReleasePlan pointing to `push-snapshot-to-gar` pipeline with destination `us-docker.pkg.dev/gcp-hcp-commons/gcp-hcp-images/{name}`
   - `integration-test-enterprise-contract.yaml` — Enterprise contract test scenario
   - `kustomization.yaml` — Kustomize resource list
3. **Quay release** (if needed): Add `ReleasePlanAdmission` in `config/kflux-prd-rh02.0fk9.p1/service/ReleasePlanAdmission/gcp-hcp/{name}.yaml`
4. **Run kustomize build**: Regenerate auto-generated manifests in `tenants-config/auto-generated/`
5. **Verify**: Merge, wait for build trigger, confirm image appears in both GAR and Quay

No infrastructure changes are needed — the pull-through cache, IAM bindings, and MutatingAdmissionPolicy handle distribution automatically for any image in the `gcp-hcp-images` repository.

### Repositories Involved

| Repository | Role |
|---|---|
| `openshift-online/gcp-hcp-infra` | Image source (Containerfiles), Terraform (GAR, WIF, cache, IAM), Helm chart (image rewriter), ArgoCD configs |
| `konflux-ci/community-catalog` | GAR push release pipeline (`push-snapshot-to-gar` task) |
| `releng/konflux-release-data` | Konflux tenant config (applications, components, release plans, WIF ConfigMap, service accounts) |

## Consequences

### Positive

* No service account keys in the entire pipeline — WIF and GKE node identity handle all authentication
* Regional resilience — cached images serve even during commons GAR outages
* Zero-touch for new images — any image pushed to commons GAR is automatically available via pull-through cache and MAP rewriting
* Dual distribution — images available in both GAR (for GKE) and Quay (for external consumers)
* Attestation preservation — `cosign copy` preserves signatures and attestations from build to GAR
* Single cleanup policy — 365-day retention on caches keeps storage costs bounded

### Negative

* Pipeline complexity — four layers (build, publish, distribute, consume) with cross-system dependencies (Konflux, GCP, Kubernetes)
* IAM propagation delays — fresh project deployments require `time_sleep` resources (30s + 60s) for IAM to propagate, adding ~90s to E2E provisioning
* Beta API dependency — MutatingAdmissionPolicy requires `admissionregistration.k8s.io/v1beta1`, which cannot be disabled once enabled on a GKE cluster
* Cleanup policy limitation — GCP Artifact Registry only supports upload-time-based cleanup, not last-pull-based. Long-lived images that are still actively used could be cleaned up if not re-cached within the retention window.
* Customer worker nodes — the current design covers Red Hat-managed clusters. Customer worker nodes in separate GCP projects will require a WIF model with OIDC token exchange for image pulls, similar to the Konflux-to-GAR pattern.

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Adding new regions requires only a Terraform config that instantiates the region module — the pull-through cache, IAM bindings, and ArgoCD image rewriter are provisioned automatically. Adding new images requires only pushing them to the commons GAR repo.
* **Observability**: Image pull failures surface as standard Kubernetes `ErrImagePull` / `ImagePullBackOff` events. The pull-through cache's health can be verified with `gcloud artifacts docker images list`. WIF authentication failures appear in the Konflux release pipeline logs.
* **Resiliency**: Pull-through caches serve cached images during upstream outages. If the MutatingAdmissionPolicy is deleted or unavailable, pods fall back to pulling directly from the commons repo (cross-region, slower, but functional). If the commons repo is unavailable and the image is not cached, the pull fails — this is the only hard failure mode.

### Security:

* No long-lived credentials anywhere in the system — WIF tokens are short-lived and automatically refreshed
* WIF provider restricted to specific Kubernetes namespace and service account via attribute conditions
* GAR repository has `prevent_destroy` lifecycle protection
* Image rewriter opt-out is a convenience feature, not a security control — the cache is an optimization, not a policy gate
* The `konflux-push` SA has only `artifactregistry.writer` on the single GAR repository (least privilege)
* Cosign attestations are preserved through the pipeline, supporting future Binary Authorization enforcement

### Performance:

* Pull-through cache eliminates cross-region GAR pulls after first access — subsequent pulls serve from regional cache
* Cache is co-located with the GKE cluster (same region), minimizing network latency
* MutatingAdmissionPolicy evaluates CEL expressions in-process (no webhook round-trip)

### Cost:

* Regional cache storage cost is bounded by the 365-day cleanup policy
* Cross-region egress from commons GAR is reduced to first-pull-only per region per image tag
* The `gcp-hcp-commons` project uses multi-region (`us`) GAR, which has higher storage cost but lower egress to US-based caches

### Operability:

* Adding a new image to the pipeline: push Containerfile to `gcp-hcp-infra/images/`, register as a Konflux component, add a release plan — no infrastructure changes needed
* Adding a new region: Terraform `infra.py new region` provisions the pull-through cache, IAM, and ArgoCD image rewriter automatically
* MintMaker/Renovate dependency updates are automatically labeled with `/ok-to-test` for CI
* Manual commons module apply required for changes to the GAR repo, WIF pool, or cross-project IAM (commons is not managed by Atlantis)
