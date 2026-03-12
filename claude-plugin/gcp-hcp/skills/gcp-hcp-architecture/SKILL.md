---
name: GCP-HCP-Architecture
description: >
  Use when working on GCP HCP-related code or design in any of the GCP HCP repositories:
  hypershift (platform/gcp/, GCP provider, GCP-related tests), gcp-hcp-infra (Terraform,
  ArgoCD, infrastructure), gcp-hcp-cli, cls-backend, cls-controller, or when creating
  or reviewing design decisions, or needing architectural context for GCP Hosted Control
  Planes on GKE. Accepts optional topic filter: networking, identity, observability,
  infrastructure, ingress, storage, operators, testing, automation, naming, dns, fleet, incidents, slo.
---

# GCP HCP Architecture Context

Provides architectural context, design decisions, and implementation plans for GCP Hosted Control Planes (HCP) on GKE. Use this skill to understand constraints and rationale before writing or reviewing code.

**Source repository:** [openshift-online/gcp-hcp](https://github.com/openshift-online/gcp-hcp)

## When to Use

**Auto-invoke when:**
- Working on files in `platform/gcp/`, GCP provider code, or GCP-related tests in hypershift
- Creating or modifying design decisions in `design-decisions/`
- Reviewing PRs that touch GCP infrastructure or control plane components
- Writing implementation plans for GCP HCP features

**Manual invocation:** `/gcp-hcp-architecture [topic]`

## Topic Index

Specify a topic to narrow results. Read the linked files from the gcp-hcp repository for full context.

| Topic | Design Decisions | Architecture | Implementation Plans |
|-------|-----------------|--------------|---------------------|
| **networking** | [private-service-connect-networking](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/private-service-connect-networking.md), [gcp-private-service-connect](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/gcp-private-service-connect.md), [gke-dataplane-v2-networking](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/gke-dataplane-v2-networking.md), [no-direct-cross-cluster-connectivity](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/no-direct-cross-cluster-connectivity.md), [rc-mc-transport-layer](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/rc-mc-transport-layer.md) | — | [gcp-private-service-connect-implementation](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/gcp-private-service-connect-implementation.md) |
| **identity** | [workload-identity-implementation](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/workload-identity-implementation.md) | — | [gcp-wif-integration](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/gcp-wif-integration.md) |
| **observability** | [observability-google-managed-prometheus](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/observability-google-managed-prometheus.md) | — | — |
| **infrastructure** | [gke-container-platform](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/gke-container-platform.md), [gcp-folder-project-hierarchy](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/gcp-folder-project-hierarchy.md), [regional-independence-architecture](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/regional-independence-architecture.md), [terraform-infrastructure-as-code](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/terraform-infrastructure-as-code.md), [terraform-automation-tooling](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/terraform-automation-tooling.md), [terraform-code-structure](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/terraform-code-structure.md) | [L1 Context](https://github.com/openshift-online/gcp-hcp/tree/main/experiments/arch/L1%20Context), [L2 Container](https://github.com/openshift-online/gcp-hcp/tree/main/experiments/arch/L2%20Container) | — |
| **ingress** | [gke-ingress-controller](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/gke-ingress-controller.md), [google-managed-certificates](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/google-managed-certificates.md), [iap-authentication](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/iap-authentication.md), [gcp-api-gateway-frontend](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/gcp-api-gateway-frontend.md) | — | [gcp-customer-authentication](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/gcp-customer-authentication.md) |
| **storage** | — | — | [gcp-pd-csi-operator-migration](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/gcp-pd-csi-operator-migration.md), [gcp-persistent-disk-csi-driver](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/gcp-persistent-disk-csi-driver.md) |
| **operators** | — | [L3 Component/Hypershift](https://github.com/openshift-online/gcp-hcp/tree/main/experiments/arch/L3%20Component/Hypershift) | [gcp-321-image-registry-operator-integration](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/gcp-321-image-registry-operator-integration.md) |
| **testing** | — | — | [hypershift-repo-gcp-hcp-e2e-tests-implementation](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/hypershift-repo-gcp-hcp-e2e-tests-implementation.md) |
| **automation** | [automation-first-philosophy](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/automation-first-philosophy.md), [cloud-workflows-automation-platform](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/cloud-workflows-automation-platform.md), [pipeline-automation-tooling](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/pipeline-automation-tooling.md), [ai-centric-sdlc](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/ai-centric-sdlc.md) | — | [gcp-320-automated-remediation-platform](https://github.com/openshift-online/gcp-hcp/blob/main/implementation-plans/gcp-320-automated-remediation-platform.md) |
| **naming** | [naming-conventions](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/naming-conventions.md), [argocd-sync-wave-standardization](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/argocd-sync-wave-standardization.md) | — | — |
| **dns** | [customer-dns-zone-management](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/customer-dns-zone-management.md) | — | — |
| **fleet** | [gke-fleet-management](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/gke-fleet-management.md), [shared-node-pools](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/shared-node-pools.md) | — | — |
| **incidents** | — | — | [incidents/](https://github.com/openshift-online/gcp-hcp/tree/main/incidents) — read all post-mortems for past failure patterns and lessons learned |
| **slo** | — | — | [slo/](https://github.com/openshift-online/gcp-hcp/tree/main/slo) — read all SLO definitions for performance and quality targets |

## Architectural Invariants

These constraints apply across all topics. Violations should be flagged in code review.

1. **No direct cross-cluster connectivity** — All cross-cluster coordination uses asynchronous, indirect mechanisms or GCP-level APIs. Direct TCP/UDP between clusters (Global, Regional, Management) is forbidden. PSC for worker-to-control-plane data-plane is the exception.
2. **Regional independence** — Each region operates independently with minimal cross-region dependencies. No cross-region network costs or data residency violations.
3. **Workload Identity for all authentication** — No long-lived service account keys. All operators use WIF to bind Kubernetes SAs to Google SAs.
4. **Google Managed Prometheus for observability** — Hybrid GMP architecture (self-managed Prometheus + GMP storage). No custom Prometheus deployments.
5. **ArgoCD for all deployments** — No direct helm install or kubectl apply for production workloads.
6. **Terraform for infrastructure** — All GCP infrastructure managed via Terraform.

## Cross-Repo Map

| Repository | What Lives There |
|-----------|-----------------|
| **[gcp-hcp](https://github.com/openshift-online/gcp-hcp)** | Design decisions, architecture docs, implementation plans, studies, Jira templates |
| **hypershift** | GCP platform implementation code, CI enforcement (Tekton/Konflux, golangci-lint, gitlint, conventional commits) |
| **gcp-hcp-infra** | Terraform modules, ArgoCD configs, infrastructure automation |
| **gcp-hcp-cli** | CLI tooling |
| **cls-backend** | Cluster Lifecycle Service backend |
| **cls-controller** | Cluster Lifecycle Service controller |

## Quality Standards

- **Definition of Done**: [definition-of-done.md](https://github.com/openshift-online/gcp-hcp/blob/main/docs/definition-of-done.md) — test coverage >= 85%, PR merge criteria
- **Story Template**: [jira-story-template.md](https://github.com/openshift-online/gcp-hcp/blob/main/docs/jira-story-template.md) — acceptance criteria format
- **Design Decision Template**: [TEMPLATE.md](https://github.com/openshift-online/gcp-hcp/blob/main/design-decisions/TEMPLATE.md) — required sections and validation checklist

## How to Use

1. Identify the relevant topic(s) from the user's task
2. **Locate the gcp-hcp repo.** If you are already working inside gcp-hcp, use the current directory. Otherwise, ask the user: *"Where is your local clone of openshift-online/gcp-hcp? (e.g., ~/go/src/github.com/openshift-online/gcp-hcp)"* — use the provided path for all subsequent file reads in this session. If the repo is not cloned locally, use the GitHub links in the topic index above.
3. Read the mapped design decisions and architecture docs using the topic index above
4. Apply constraints and rationale when writing or reviewing code
5. Reference specific design decisions in PR descriptions and commit messages
6. Flag any violations of architectural invariants during code review
