# Shared Workflow Tools Image for Cloud Workflows

***Scope***: GCP-HCP

**Date**: 2026-06-12
**Updated**: 2026-06-17

## Decision

Adopt a single shared workflow tools image (`gcp-hcp-workflow-tools`) for all Cloud Workflows operations. The image layers domain-specific binaries and scripts on top of `gcp-hcp-common-tools`, using a domain-prefixed `MODE` dispatcher (`etcd-health`, `etcd-compact`, etc.) that auto-routes to scripts organized in per-domain subdirectories. Adding a new workflow domain is a code change (add scripts), not an infrastructure change (no new Konflux pipeline).

## Context

- **Problem Statement**: Cloud Workflows execute complex operations via Kubernetes Jobs. The current approach uses three ad-hoc patterns: (1) inline shell one-liners in Job specs constrained by Cloud Workflows' 400-character expression limit, (2) runtime discovery of container images from StatefulSets, and (3) hardcoded references to images on personal registries. None of these scale as we add more workflows with custom logic. There is no standard pattern for how a workflow should package and execute its scripts.

- **Constraints**:
  - Cloud Workflows has a 400-character limit on `${}` expression interpolation, making multi-line shell scripts impractical to maintain inline
  - Some images were hosted on personal registries, not org-owned, creating fragile dependencies for production SRE tooling
  - Images must be built via Konflux per Red Hat policy
  - GKE clusters pull images via Workload Identity (no pull secrets)
  - Onboarding a new Konflux component requires coordinated changes across three repositories with multiple gotchas (GAR SA secrets, `.tekton/` fixes, dual publishing)

- **Assumptions**:
  - The team will add workflow tool domains to a single shared image as needed — not one image per domain
  - `gcp-hcp-common-tools` is maintained as a stable base image with infrequent changes
  - The image distribution pipeline (Konflux → quay.io → GAR) is operational, as documented in the [Container Image Organization](container-image-build-and-distribution.md) design decision

## Alternatives Considered

1. **Inline shell in workflow Job specs**: Continue embedding shell commands directly in Cloud Workflows YAML. No container image dependency — workflows construct commands using string interpolation and pass them as Job `args`. Simple for short commands (health checks, status queries) but breaks down for multi-step operations.

2. **Per-domain images**: Each workflow domain gets its own Konflux-built image (e.g., `gcp-hcp-etcd-tools`, `gcp-hcp-kubectl-tools`). Each image layers on `gcp-hcp-common-tools` with its specific binary and scripts. Clean separation of concerns, independent build/release cycles.

3. **Single shared workflow-tools image** (chosen): One image bundles all workflow tool binaries and scripts across all domains. A thin entrypoint parses `MODE=<domain>-<operation>` and dispatches to `scripts/<domain>/<operation>.sh`. New domains are added by creating a script subdirectory — no Konflux, Containerfile, or pipeline changes needed.

## Decision Rationale

* **Justification**: The per-domain image model (alternative 2) was initially implemented with `gcp-hcp-etcd-tools`. During that onboarding, we discovered that each new Konflux component requires coordinated changes across three repositories (image source, Konflux release-data, Tekton pipeline fixes) with multiple gotchas: GAR service account secrets, `{{source_url}}` → `{{repo_url}}` fork access fixes, `pathChanged()` CEL filter additions, and dual-publishing setup. This operational overhead is disproportionate to the value of per-domain isolation. A single shared image eliminates this entirely — adding a new domain is a code change (scripts + entrypoint entry), not an infrastructure change.

* **Evidence**: The etcd-tools onboarding required 6 PRs across 2 repos and 3 GitLab MRs to get a single image building and releasing. The shared model reduces future domains to a single PR in `gcp-hcp-infra`.

* **Comparison**:
  - **Inline shell** (alternative 1): Rejected. The ~400-character compact command in `etcd-ops.yaml` demonstrates the maintainability ceiling. This approach cannot accommodate operations requiring error handling, conditional logic, or multi-step orchestration.
  - **Per-domain images** (alternative 2): Rejected after initial implementation. The Konflux onboarding overhead per image is too high for the team's size. Image isolation provides minimal practical benefit — PAM enforcement happens at the workflow layer (not the image), and the shared base means most of the image content is identical anyway. The lower maintenance burden of one pipeline outweighs the theoretical cleanliness of separate images.

## Implementation

### Image Structure

```
images/workflow-tools/
├── Containerfile           # FROM common-tools + domain binaries
├── README.md
└── scripts/
    ├── entrypoint.sh       # MODE dispatcher (parses <domain>-<operation>)
    ├── common.sh           # Shared TLS setup, ectl() wrapper
    └── etcd/               # etcd operations domain
        ├── health.sh
        ├── status.sh
        ├── member-list.sh
        ├── defrag.sh
        ├── compact.sh
        ├── benchmark.sh
        ├── cleanup.sh
        └── demo.sh
```

### MODE Format

Modes use a `<domain>-<operation>` format. The entrypoint auto-parses the domain prefix and routes to the correct script:

```
MODE=etcd-health      → scripts/etcd/health.sh
MODE=etcd-compact     → scripts/etcd/compact.sh
MODE=etcd-benchmark   → scripts/etcd/benchmark.sh
```

Invalid modes print available operations for the domain.

### Adding a New Domain

1. Create `scripts/<domain>/` with operation scripts
2. Each script sources `common.sh` for shared setup (or brings its own)
3. Add domain-specific binaries to the Containerfile if needed
4. Pin binary versions in `.tool-versions`
5. Update the README

No entrypoint changes needed — the dispatcher auto-discovers scripts from the directory structure.

## Consequences

### Positive

* **One Konflux pipeline for all workflow tools** — no per-domain onboarding overhead
* Adding a new domain is a single PR with scripts — no Konflux, GAR SA, or pipeline changes
* Scripts are testable, lintable (shellcheck), and version-controlled
* Eliminates personal registry dependencies — org-owned, Konflux-built, scanned
* Domain-prefixed MODEs prevent naming collisions (`etcd-health` vs future `gcs-health`)
* Version pinning via `.tool-versions` provides a single source of truth

### Negative

* All domains share one build/release cycle — a broken script in one domain blocks all domains
* Image size grows as domains are added — acceptable for occasional Job execution with AR pull-through cache
* Common-tools base includes utilities not needed by every domain (e.g., terraform in etcd operations)
* No independent release cadence per domain — all domains ship together

## Cross-Cutting Concerns

### Security:

* All images run as non-root user (UID 65532)
* TLS is enforced — tool scripts validate ENDPOINT starts with `https://` and verify TLS certificate files exist before execution
* PAM (Privileged Access Manager) gating is enforced at the workflow layer, not the image layer — the image contains both safe (health check) and destructive (defrag) capabilities, but destructive workflows require PAM approval before invocation
* Binary downloads are SHA256-verified against upstream checksums
* Images are scanned by Konflux's enterprise contract pipeline (Clair, Snyk, RPM signature scan, deprecated image check)

### Operability:

* **Adding a new domain** requires only a PR to `gcp-hcp-infra` — add scripts under `images/workflow-tools/scripts/<domain>/`, update README
* **Adding domain-specific binaries** requires updating the Containerfile and `.tool-versions` — still a single PR
* **Konflux pipeline fixes** were applied once during initial setup — `{{repo_url}}` for fork access and `pathChanged()` CEL filters. These persist across domain additions.
* **Dual publishing** (GAR + quay prod) is configured once for the `gcp-hcp-workflow-tools` component

### Cost:

* Image storage cost is marginal — GAR and quay storage for one image is negligible
* Build cost: Go compilation of benchmark binary adds ~30-60s to CI builds (Konflux-only cost, not affecting deployment)
* No additional GCP API costs — images are pulled via existing Workload Identity bindings

---
