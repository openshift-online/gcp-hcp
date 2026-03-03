---
name: GCP-HCP-Architecture
description: >
  Use when working on GCP HCP-related code or design in any of the GCP HCP repositories:
  hypershift (platform/gcp/, GCP provider, GCP-related tests), gcp-hcp-infra (Terraform,
  ArgoCD, infrastructure), gcp-hcp-cli, cls-backend, cls-controller, or when creating
  or reviewing design decisions, or needing architectural context for GCP Hosted Control
  Planes on GKE. Accepts optional topic filter: networking, identity, observability,
  infrastructure, ingress, storage, operators, testing, automation, naming, incidents, slo.
---

# GCP HCP Architecture Context

Provides architectural context, design decisions, and implementation plans for GCP Hosted Control Planes (HCP) on GKE. Use this skill to understand constraints and rationale before writing or reviewing code.

## When to Use

**Auto-invoke when:**
- Working on files in `platform/gcp/`, GCP provider code, or GCP-related tests in hypershift
- Creating or modifying design decisions in `design-decisions/`
- Reviewing PRs that touch GCP infrastructure or control plane components
- Writing implementation plans for GCP HCP features

**Manual invocation:** `/gcp-hcp:architecture [topic]`

## Topic Index

Specify a topic to narrow results. Read the linked files from this repository for full context.

| Topic | Design Decisions | Architecture | Implementation Plans |
|-------|-----------------|--------------|---------------------|
| **networking** | `private-service-connect-networking.md`, `gcp-private-service-connect-implementation.md`, `gke-dataplane-v2-networking.md`, `no-direct-cross-cluster-connectivity.md`, `rc-mc-transport-layer.md` | — | `gcp-private-service-connect-implementation.md` |
| **identity** | `workload-identity-implementation.md` | — | `gcp-wif-integration.md` |
| **observability** | `observability-google-managed-prometheus.md` | — | — |
| **infrastructure** | `gke-container-platform.md`, `gcp-folder-project-hierarchy.md`, `regional-independence-architecture.md`, `terraform-infrastructure-as-code.md`, `terraform-automation-tooling.md`, `terraform-code-structure.md` | `experiments/arch/L1 Context/`, `experiments/arch/L2 Container/` | — |
| **ingress** | `gke-ingress-controller.md`, `google-managed-certificates.md`, `iap-authentication.md`, `gcp-api-gateway-frontend.md` | — | `gcp-customer-authentication.md` |
| **storage** | — | — | `gcp-pd-csi-operator-migration.md`, `gcp-persistent-disk-csi-driver.md` |
| **operators** | — | `experiments/arch/L3 Component/Hypershift/` | `gcp-321-image-registry-operator-integration.md` |
| **testing** | — | — | `hypershift-repo-gcp-hcp-e2e-tests-implementation.md`. Docs: `docs/ci-e2e-test-flow.md`, `docs/create-cluster-test-validations.md` |
| **automation** | `automation-first-philosophy.md`, `cloud-workflows-automation-platform.md`, `pipeline-automation-tooling.md`, `ai-centric-sdlc.md` | — | `gcp-320-automated-remediation-platform.md` |
| **naming** | `naming-conventions.md`, `argocd-sync-wave-standardization.md` | — | — |
| **dns** | `customer-dns-zone-management.md` | — | — |
| **fleet** | `gke-fleet-management.md`, `shared-node-pools.md` | — | — |
| **incidents** | — | — | `incidents/` — read all post-mortems for past failure patterns and lessons learned |
| **slo** | — | — | `slo/` — read all SLO definitions for performance and quality targets |

**File paths** (relative to gcp-hcp repo root): Design decisions are in `design-decisions/`. Implementation plans are in `implementation-plans/`. Architecture docs are in `experiments/arch/`. Studies are in `studies/`. Incidents are in `incidents/`. SLOs are in `slo/`. Team docs (templates, definition of done, test flows) are in `docs/`.

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
| **gcp-hcp** (this repo) | Design decisions, architecture docs, implementation plans, studies, Jira templates |
| **hypershift** | GCP platform implementation code, CI enforcement (Tekton/Konflux, golangci-lint, gitlint, conventional commits) |
| **gcp-hcp-infra** | Terraform modules, ArgoCD configs, infrastructure automation |
| **gcp-hcp-cli** | CLI tooling |
| **cls-backend** | Cluster Lifecycle Service backend |
| **cls-controller** | Cluster Lifecycle Service controller |

## Quality Standards

- **Definition of Done**: `docs/definition-of-done.md` — test coverage >= 85%, PR merge criteria
- **Story Template**: `docs/jira-story-template.md` — acceptance criteria format
- **Design Decision Template**: `design-decisions/TEMPLATE.md` — required sections and validation checklist

## How to Use

1. Identify the relevant topic(s) from the user's task
2. **Locate the gcp-hcp repo.** If you are already working inside gcp-hcp, use the current directory. Otherwise, ask the user: *"Where is your local clone of openshift-online/gcp-hcp? (e.g., ~/go/src/github.com/openshift-online/gcp-hcp)"* — use the provided path for all subsequent file reads in this session.
3. Read the mapped design decisions and architecture docs using the topic index above
4. Apply constraints and rationale when writing or reviewing code
5. Reference specific design decisions in PR descriptions and commit messages
6. Flag any violations of architectural invariants during code review