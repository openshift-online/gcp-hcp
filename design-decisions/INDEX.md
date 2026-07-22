# Design Decisions Index

Decision records organized by topic. Use [TEMPLATE.md](TEMPLATE.md) when adding a new one, and update this index and the [architecture skill](../claude-plugin/gcp-hcp/skills/gcp-hcp-architecture/SKILL.md) accordingly.

## Infrastructure

| Decision | Summary |
|----------|---------|
| [gke-container-platform](infrastructure/gke-container-platform.md) | GKE as the container platform (not OpenShift) |
| [gcp-folder-project-hierarchy](infrastructure/gcp-folder-project-hierarchy.md) | Environment-first GCP folder and project hierarchy |
| [regional-independence-architecture](infrastructure/regional-independence-architecture.md) | Regions operate independently with minimal cross-region dependencies |
| [gke-fleet-management](infrastructure/gke-fleet-management.md) | GKE Fleet for cluster bootstrap without external network access |
| [shared-node-pools](infrastructure/shared-node-pools.md) | Shared node pool architecture for management cluster control plane components |

## Networking

| Decision | Summary |
|----------|---------|
| [private-service-connect-networking](networking/private-service-connect-networking.md) | PSC for worker-to-control-plane connectivity |
| [gcp-private-service-connect](networking/gcp-private-service-connect.md) | PSC design decisions and architectural patterns for HyperShift integration |
| [no-direct-cross-cluster-connectivity](networking/no-direct-cross-cluster-connectivity.md) | No direct cross-cluster network connectivity |
| [gke-dataplane-v2-networking](networking/gke-dataplane-v2-networking.md) | GKE Dataplane V2 with eBPF/Cilium for enhanced security and observability |
| [gke-ingress-controller](networking/gke-ingress-controller.md) | GKE Ingress (GCE) as standard ingress for infrastructure tooling services |
| [google-managed-certificates](networking/google-managed-certificates.md) | Google-managed SSL/TLS certificates for automatic certificate lifecycle |
| [gcp-api-gateway-frontend](networking/gcp-api-gateway-frontend.md) | ~~GCP API Gateway for customer-facing API~~ *(superseded by espv2-api-frontend)* |
| [espv2-api-frontend](networking/espv2-api-frontend.md) | ESPv2 sidecar with Cloud Endpoints for API frontend and Marketplace integration |
| [customer-dns-zone-management](networking/customer-dns-zone-management.md) | Customer DNS zones created by control-plane-operator with WIF authentication |
| [ci-externaldns-configuration](networking/ci-externaldns-configuration.md) | Dedicated public Cloud DNS zone with WIF for CI E2E ExternalDNS |
| [oidc-cdn-public-serving](networking/oidc-cdn-public-serving.md) | Cloud CDN for public OIDC document serving |
| [datastore-transport](networking/datastore-transport.md) | Firestore as transport layer for regional-management cluster communication |
| [rc-mc-transport-layer](networking/rc-mc-transport-layer.md) | ~~Maestro for RC-MC communication~~ *(superseded by datastore-transport)* |

## Identity & Security

| Decision | Summary |
|----------|---------|
| [workload-identity-implementation](identity/workload-identity-implementation.md) | Workload Identity from day one for all operators in the control plane |
| [prow-ci-workload-identity-federation](identity/prow-ci-workload-identity-federation.md) | Prow CI must use WIF exclusively — no static JSON keys |
| [iap-authentication](identity/iap-authentication.md) | Identity-Aware Proxy for internal tooling authentication |
| [pam-workflow-gating](identity/pam-workflow-gating.md) | GCP PAM with resource tags to gate sensitive Cloud Workflows behind approval |
| [zero-operator-access](identity/zero-operator-access.md) | Layered model restricting human and AI agent access to production resources |

## Observability

| Decision | Summary |
|----------|---------|
| [observability-google-managed-prometheus](observability/observability-google-managed-prometheus.md) | Hybrid GMP architecture for HCP component metrics with regional data isolation |
| [integrated-alerting-framework](observability/integrated-alerting-framework.md) | Cloud Monitoring routing to PagerDuty and Cloud Run diagnosis agent |
| [data-lake-for-diagnostics-and-compliance](observability/data-lake-for-diagnostics-and-compliance.md) | Two-tier data lake: BigQuery for real-time data, Log Analytics for compliance |

## Automation & CI/CD

| Decision | Summary |
|----------|---------|
| [automation-first-philosophy](automation/automation-first-philosophy.md) | Automation-first "allergic to toil" approach |
| [terraform-infrastructure-as-code](automation/terraform-infrastructure-as-code.md) | Terraform as primary IaC scoped to region bootstrapping and foundational infra |
| [terraform-code-structure](automation/terraform-code-structure.md) | Hierarchical Terraform directory structure separating global and regional resources |
| [terraform-automation-tooling](automation/terraform-automation-tooling.md) | Atlantis for PR-based Terraform automation on global GKE clusters per environment |
| [hcp-terraform-workload-identity-federation](automation/hcp-terraform-workload-identity-federation.md) | HCP Terraform must use WIF with Dynamic Provider Credentials and explicit audience |
| [hcp-terraform-workspace-architecture](automation/hcp-terraform-workspace-architecture.md) | HCP Terraform workspace hierarchy, variable sets, and state migration strategy |
| [argocd-sync-wave-standardization](automation/argocd-sync-wave-standardization.md) | Standardized ArgoCD sync wave annotations using minimal 3-wave system |
| [deployment-tooling-swim-lanes](automation/deployment-tooling-swim-lanes.md) | Configuration management swim lanes defining tool-specific resource lifecycle |
| [pipeline-automation-tooling](automation/pipeline-automation-tooling.md) | Tekton as general-purpose pipeline automation for scheduled/event-driven workflows |
| [cloud-workflows-automation-platform](automation/cloud-workflows-automation-platform.md) | Google Cloud Workflows for Zero Operator remediation with Vertex AI and PAM gates |
| [cloud-workflows-common-tools-image](automation/cloud-workflows-common-tools-image.md) | Shared workflow tools image with domain-prefixed MODE dispatcher |
| [container-image-build-and-distribution](automation/container-image-build-and-distribution.md) | Dedicated repo for team-maintained utility container images |
| [container-image-build-and-distribution-pipeline](automation/container-image-build-and-distribution-pipeline.md) | Konflux for builds, Artifact Registry for publishing, regional pull-through caches |
| [ai-centric-sdlc](automation/ai-centric-sdlc.md) | AI-centric SDLC with multi-tool support and required human review |
| [agent-autonomy-levels](automation/agent-autonomy-levels.md) | Three-stage approach for agent-driven remediation with increasing autonomy levels |
| [go-controllers-runtime](automation/go-controllers-runtime.md) | Go controllers replacing config-based adapters for cluster lifecycle management |
| [hcp-terraform-workload-identity-federation](automation/hcp-terraform-workload-identity-federation.md) | HCP Terraform authenticates via infra-platform `terraform-tfe-gcp-dynamic-creds` module with per-environment WIF pools |
| [hcp-terraform-workspace-architecture](automation/hcp-terraform-workspace-architecture.md) | Per-environment TFC projects with module-managed WIF and `apply_to_all_workspaces` |

## Governance

| Decision | Summary |
|----------|---------|
| [naming-conventions](governance/naming-conventions.md) | Standardized naming pattern `{env}-{type}-{region_code}-{identifier}` |
| [repository-organization-policy](governance/repository-organization-policy.md) | Three-tier repository structure with graduation criteria |
| [gcphcpctl-graduation](governance/gcphcpctl-graduation.md) | Graduate GCP HCP CLI to dedicated repository |
| [adopt-cincinnati-for-version-resolution](governance/adopt-cincinnati-for-version-resolution.md) | Replace hardcoded release image with Cincinnati update service for version resolution |
| [platform-api](governance/platform-api.md) | Platform API server as single source of truth for CLM API definition |
| [gecko-graduation](governance/gecko-graduation.md) | Graduate Platform API experiment to dedicated gecko repository |
