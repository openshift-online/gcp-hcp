# Zero Egress Worker Nodes

***Scope***: GCP-HCP

**Date**: 2026-10-29

## Decision

Deliver required OpenShift release images for workers through regional Google Artifact Registry (GAR) pull-through repositories, using worker VM identity and on-demand credential helpers. Deny public worker egress by default, with only explicitly approved exceptions for required platform services.

## Context

- **Problem Statement:** Workers need images from `quay.io` and `registry.redhat.io` to bootstrap, scale and upgrade. Downloading directly from these registries requires internet egress; blocking it can prevent required uncached pulls.
- **Constraints:** No worker external IPs, service-account keys or distributed GAR pull-secret tokens. Control-plane authentication is outside this decision.
- **Required connectivity:** Keep DNS resolution, Private Google Access to GAR and required Google APIs, and private connections to the hosted control-plane API and bootstrap services.
- **Customer credentials:** Retain customers' private-registry credentials. Network access to those registries remains a separate requirement.
- **Assumptions:** GAR can fetch uncached images upstream, and supported RHCOS releases can authenticate during bootstrap and normal operation.

## Alternatives Considered

1. **Centrally refreshed GAR pull secrets:** Require token distribution, synchronization and handling of renewal-driven disruption.
2. **Workers pull directly from Quay and Red Hat:** Download from these public registry endpoints instead of GAR, requiring worker internet access.

## Decision Rationale

- **Justification:** VM identity and native digest mirrors provide keyless image access without centrally distributing GAR tokens.
- **Evidence:** [Kubernetes credential providers](https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/) and [Compute Engine metadata authentication](https://docs.cloud.google.com/compute/docs/access/authenticate-workloads) provide the underlying mechanisms.
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

### Security

Grant worker identities read-only access to the required GAR repositories and scope mirror rules to required OpenShift repositories. Restricted upstream credentials / scopes must be in place to prevent arbiatary image pulls.

### Operability

Propose disabling Insights and telemetry reporting by default for zero-egress deployments, pending business and support agreement on reporting requirements. Define any approved egress explicitly. Helper packaging, permission provisioning and rollout details remain implementation choices.

### Cost

See [Google's GAR pricing breakdown](https://cloud.google.com/artifact-registry/pricing) for storage, data transfer, free allowances and optional vulnerability-scanning charges.

Related: [image-distribution decision](../automation/container-image-build-and-distribution-pipeline.md).
