# Zero Egress Worker Nodes

***Scope***: GCP-HCP

**Date**: 2026-10-29

## Decision

Deliver required images for workers through regional Google Artifact Registry (GAR) pull-through repositories, using worker VM identity and on-demand credential helpers. Deny public worker egress by default, with only explicitly approved exceptions for required platform services.

## Context

- **Problem Statement:** Workers need images from `quay.io` to bootstrap, scale and upgrade. Downloading directly from Quay requires internet egress; blocking it can prevent required uncached pulls.
- **Constraints:** No worker external IPs, service-account keys or distributed GAR pull-secret tokens. Control-plane authentication is outside this decision.
- **Required connectivity:** Keep DNS resolution, Private Google Access to GAR and required Google APIs, and private connections to the hosted control-plane API and bootstrap services.
- **Customer credentials:** Retain customers' private-registry credentials. Network access to those registries remains a separate requirement.
- **Assumptions:** GAR can fetch uncached images upstream, and supported RHCOS releases can authenticate during bootstrap and normal operation.

## Alternatives Considered

1. **Centrally refreshed GAR pull secrets:** Generate tokens with External Secrets Operator and propagate them to workers, adding token distribution, synchronization and handling of renewal-driven disruption.
2. **Workers pull directly from Quay and Red Hat:** Download from these public registry endpoints instead of GAR, requiring worker internet access.

## Decision Rationale

- **Justification:** VM identity and native digest mirrors provide keyless image access without centrally distributing GAR tokens.
- **Evidence:** Development validation is tracked in [GCP-1276](https://redhat.atlassian.net/browse/GCP-1276). [Kubernetes credential providers](https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/) and [Compute Engine metadata authentication](https://docs.cloud.google.com/compute/docs/access/authenticate-workloads) provide the underlying mechanisms.
- **Comparison:** A runtime helper complements kubelet's provider for bootstrap and mirror pulls, avoiding recurring token-file updates.

## Consequences

### Positive

- Workers can bootstrap and pull required images without external IPs or Cloud NAT.
- Limits general outbound internet access from workers.
- Avoids central GAR credential distribution and credential-renewal-driven kubelet restarts.
- Cached images remain available during upstream outages; uncached images still require upstream access.
- Regional caches can reduce pull latency and repeated upstream downloads.
- GAR IAM centralizes read-access management for worker identities.

### Negative

- A **temporary custom shell helper** must be maintained for bootstrap and mirror authentication until a supported replacement is available in RHCOS. Track [RHEL-82921](https://redhat.atlassian.net/browse/RHEL-82921) and [OCPSTRAT-3945](https://redhat.atlassian.net/browse/OCPSTRAT-3945).
- Image access depends on metadata, IAM and GAR; uncached content also depends on upstream availability. Recovery and supported-release compatibility need attention during implementation.

## Cross-Cutting Concerns

### Reliability

Regional caches limit cross-region dependencies and can serve retained images during Quay outages; uncached or evicted images still require upstream availability. Validate recovery from metadata, IAM and GAR failures, including credential-cache expiry and fresh-worker bootstrap.

### Security

Grant worker identities read-only access to the required GAR repositories and scope mirror rules to required OpenShift repositories. Use least-privilege upstream credentials; [public Quay images remain readable](https://docs.projectquay.io/use_quay.html#project-quay-repository-overview), so enforcing an image allowlist requires additional controls.

### Operability

Reporting behavior is a rollout dependency: business and support must agree whether Insights and telemetry are disabled or use an explicitly approved egress path. Helper packaging, permission provisioning and rollout details remain implementation choices.

### Cost

Plan for one Quay pull-through repository per enabled regional deployment/project. For example, dev, stage and production deployments in the same geographic region have three separate repositories; a single production deployment has one. Estimate storage and data transfer across deployments, including duplicated cached images.

See [Google's GAR pricing breakdown](https://cloud.google.com/artifact-registry/pricing) for storage, data transfer, free allowances and optional vulnerability-scanning charges.

Related: [image-distribution decision](../automation/container-image-build-and-distribution-pipeline.md).
