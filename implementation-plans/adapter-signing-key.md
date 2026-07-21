# Signing Key Adapter

> **Status**: Superseded — The adapter framework is replaced by Go controllers per [Go Controllers Runtime](../design-decisions/automation/go-controllers-runtime.md). The business logic documented here carries forward.

**Jira**: [GCP-333](https://redhat.atlassian.net/browse/GCP-333)

| Field | Value |
|-------|-------|
| **Identifier** | `signing-key-adapter` |
| **Transport** | Maestro (ManifestWork with signing-key Job to MC) |
| **Runs on** | Region cluster → creates resources on MC |
| **Depends on** | `placement-adapter` Available |

## Overview

Provisions the RSA signing keypair for the HostedCluster's API server on the target MC. The private key is stored as a K8s Secret on the MC, and the public key is published to the OIDC issuer bucket (GCS) so customer WIF can validate service account tokens. Private key material never flows through HyperFleet API, Maestro payloads, or Pub/Sub — the adapter sends only resource definitions (Job, RBAC) via Maestro.

## Behavior

### MC-Side Key Generation

The keypair is generated directly on the MC via a Kubernetes Job delivered as a ManifestWork through Maestro. The private key never leaves the MC — no intermediate storage (GSM, ESO) is needed.

```
Region Cluster                          Management Cluster
──────────────                          ──────────────────

1. Create ManifestWork ────────────►    ManifestWork contains:
   via Maestro                            • Namespace (clusters-{clusterId})
   (contains resource definitions,        • ServiceAccount (keygen-sa, WIF-annotated)
    NOT key material)                     • Role + RoleBinding (get/create Secrets)
                                          • Job (keygen-{clusterId})

                                        2. Job runs on MC (as keygen-sa):
                                          • if signing key provided in spec:
                                            decode and validate PKCS#1 PEM
                                          • else: generate RSA 4096 keypair,
                                            format JWKS + OIDC discovery doc,
                                            upload to GCS issuer bucket
                                          • creates K8s Secret ({clusterName}-signing-key)
                                            in namespace (private key stays on MC)

3. Adapter monitors Job status ◄───     Maestro FeedbackRules report
   via Maestro feedback                   Job .status.conditions

4. POST /clusters/{id}/statuses
   signing key ready
```

With this approach, the private key never leaves the MC — there is no GSM involvement or ESO handoff. The `keygen-sa` K8s ServiceAccount is created as part of the ManifestWork on the MC, annotated with a GCP SA that has `roles/storage.objectCreator` on the OIDC issuer bucket. The GCP SA and its WIF binding are pre-provisioned by the management-cluster Terraform module.

The Job generates a new RSA 4096 keypair, creates the K8s Secret, builds JWKS + OIDC discovery documents, and uploads them to the issuer bucket at `https://storage.googleapis.com/{infraID}-oidc-issuer`. A temporary fallback mode accepts a pre-existing signing key via the cluster spec's `signingKeyPEM` field (validates PKCS#1, creates Secret, skips JWKS upload) — this will be removed once the GCS issuer bucket provisioning is fully operational. The Job is idempotent — if the Secret already exists, it exits successfully without regenerating.

On cluster deletion, ManifestWork removal cascades to the Job, RBAC, ServiceAccount, and signing key Secret on the MC. Public keys in the GCS issuer bucket are cleaned up separately.

## Preconditions & Gating

| Gate | CEL Expression (summary) |
|------|--------------------------|
| Placement available | `placement-adapter` reports `Available: True` |
| Target MC known | Placement decision contains `managementClusterName` |
| Cluster not Ready | Cluster aggregate status is not yet `Ready` |

The adapter only proceeds when the placement decision is available and the cluster is not yet fully ready. Once the signing key is provisioned, subsequent reconcile events skip key generation (idempotent — Secret already exists).

## Status Reporting

Reports the three mandatory conditions:

| Condition | Meaning |
|-----------|---------|
| `Applied` | ManifestWork successfully sent to Maestro |
| `Available` | Signing key Job completed, Secret created on MC |
| `Health` | Job completed without errors |

## Idempotency & Edge Cases

- **First run**: creates ManifestWork with Job and RBAC, Job generates keypair and creates Secret
- **Subsequent runs (same generation)**: Job is idempotent — if the Secret already exists, exits successfully
- **Signing key provided in spec**: Job decodes and validates the PKCS#1 PEM instead of generating a new keypair; skips JWKS/OIDC upload
- **Cluster deletion**: ManifestWork removal cascades to all MC-side resources (Job, RBAC, SA, Secret)
- **GCS upload failure**: Job fails and reports error; adapter surfaces failure in status conditions

## Credentials

| Credential | Access | Source |
|-----------|--------|--------|
| GCP SA (region) | None — adapter sends ManifestWork only | Workload Identity on region cluster |
| GCP SA (MC-side, keygen-sa) | `roles/storage.objectCreator` on OIDC issuer bucket | WIF binding pre-provisioned by management-cluster Terraform module |
| Maestro | gRPC CloudEvents client | In-cluster service (`maestro-grpc.hyperfleet.svc.cluster.local:8090`) |
| HyperFleet API | Cluster details + status POST | In-cluster service (`http://hyperfleet-api.hyperfleet.svc.cluster.local:8000`) |

## Design Alternatives Considered

**Region-side generation via GSM**: Generate the keypair on the region cluster, store in GCP Secret Manager, and use ESO to pull it onto the MC. This adds centralized key audit logging and version-based rotation, but introduces five additional components (GSM write, IAM grant, ESO SecretStore, ExternalSecret, cross-project IAM) and a wider blast radius for key material. Rejected for MVP in favor of the simpler MC-side approach. Can be revisited if centralized key management becomes a requirement.

## Open Questions

- [ ] Key rotation: What triggers rotation? Does the adapter support re-generation on spec change?
- [ ] Should the OIDC issuer bucket be per-MC or shared regionally?

## Backlog

| Story | Jira | Status |
|-------|------|--------|
| Implement signing-key adapter (MC-side keygen via Maestro) | TBD | Not Started |
