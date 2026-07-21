# HostedCluster Adapter

> **Status**: Superseded — The adapter framework is replaced by Go controllers per [Go Controllers Runtime](../design-decisions/automation/go-controllers-runtime.md). The business logic documented here carries forward.

**Jira**: [GCP-333](https://redhat.atlassian.net/browse/GCP-333)

| Field | Value |
|-------|-------|
| **Identifier** | `hostedcluster-adapter` |
| **Transport** | Maestro (ManifestWork with HostedCluster CR to MC) |
| **Runs on** | Region cluster → creates resources on MC |
| **Depends on** | `signing-key-adapter` Available |

## Overview

Materializes the OpenShift control plane on the management cluster. This is the core adapter in the chain — it sends a ManifestWork via Maestro containing the HostedCluster CR and its supporting resources (pull secret, TLS certificate), and tracks the control plane's availability and health through Maestro status feedback.

## Behavior

### ManifestWork Contents

The adapter sends a single ManifestWork to the target MC containing:

- **Namespace** `clusters-{clusterId}` (already exists from signing-key adapter; applied idempotently via ServerSideApply)
- **HostedCluster CR** (`hypershift.openshift.io/v1beta1`) with full GCP platform spec including WIF configuration (pool, provider, SA emails), network configuration (VPC, subnet, PSC endpoint access), signing key Secret reference (`{clusterName}-signing-key`), OIDC issuer URL, and release image
- **Certificate** (cert-manager) for API server TLS — wildcard on `*.{clusterName}-user.{baseDomain}`, issued by a ClusterIssuer
- **ExternalSecret** for pull secret — references a ClusterSecretStore on the MC backed by GCP Secret Manager *(will be moved to a dedicated pull-secret-adapter in the future)*

## Preconditions & Gating

| Gate | CEL Expression (summary) |
|------|--------------------------|
| Placement available | `placement-adapter` reports `Available: True` |
| Base domain set | Placement decision contains `baseDomain` |
| Signing key available | `signing-key-adapter` reports `Available: True` |
| Cluster not Ready | Cluster aggregate status is not yet `Ready` |

The adapter only proceeds when both placement and signing key are available. This ensures the HostedCluster CR references a valid MC, base domain, and signing key Secret.

## Status Reporting

The adapter evaluates CEL expressions against Maestro status feedback to derive the three mandatory conditions:

| Condition | Meaning |
|-----------|---------|
| `Applied` | ManifestWork's `Applied` condition (Maestro agent accepted and applied resources) |
| `Available` | HostedCluster's `Available` condition (streamed back via Maestro `statusFeedback`) |
| `Health` | Inverse of the HostedCluster's `Degraded` condition |

The adapter also surfaces `data.hostedCluster.apiEndpoint` and `data.hostedCluster.version` from status feedback for downstream consumption.

## Idempotency & Edge Cases

- **First run**: creates ManifestWork with HostedCluster CR, Certificate, ExternalSecret
- **Subsequent runs (same generation)**: ManifestWork is applied idempotently via ServerSideApply; no duplicate resources
- **Spec update (generation change)**: adapter updates the ManifestWork with new spec values; Maestro agent applies the diff
- **Cluster deletion**: ManifestWork removal cascades to the HostedCluster CR and supporting resources on the MC; HyperShift handles control plane teardown

## Credentials

| Credential | Access | Source |
|-----------|--------|--------|
| GCP SA | None — adapter sends ManifestWork only | Workload Identity on region cluster |
| Maestro | gRPC CloudEvents client | In-cluster service (`maestro-grpc.hyperfleet.svc.cluster.local:8090`) |
| HyperFleet API | Cluster details + status POST | In-cluster service (`http://hyperfleet-api.hyperfleet.svc.cluster.local:8000`) |

## Design Alternatives Considered

None documented for MVP.

## Open Questions

- [ ] Should the pull secret be moved to a dedicated pull-secret-adapter before GA?
- [ ] How should the adapter handle HostedCluster upgrade rollbacks (e.g., version downgrade in spec)?

## Backlog

| Story | Jira | Status |
|-------|------|--------|
| Implement hostedcluster-adapter (HostedCluster lifecycle via Maestro) | TBD | Not Started |
