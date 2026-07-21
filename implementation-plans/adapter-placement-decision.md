# Placement Decision Adapter

> **Status**: Superseded — The adapter framework is replaced by Go controllers per [Go Controllers Runtime](../design-decisions/automation/go-controllers-runtime.md). The business logic documented here carries forward.

**Jira**: [GCP-333](https://redhat.atlassian.net/browse/GCP-333)

| Field | Value |
|-------|-------|
| **Identifier** | `placement-adapter` |
| **Transport** | Kubernetes (placement decision Job on Region) |
| **Runs on** | Region cluster |
| **Depends on** | None (first in current chain; validation-adapter will precede once implemented) |

## Overview

Selects a target management cluster and DNS zone for a new HostedCluster, and writes the placement decision to the HyperFleet API cluster status. All downstream adapters read the chosen MC and base domain from this decision.

## Behavior

### MC Selection

Eligible MCs are identified by listing Secret Manager secrets in the regional project, created by the management-cluster Terraform module when a new MC is provisioned. These are cross-checked against Maestro's registered consumers to confirm the agent is connected and healthy. This naturally excludes MCs that are still being provisioned, have disconnected agents, or are being decommissioned. No region filtering is needed — all candidates are inherently in the same region as the placement adapter.

Among eligible MCs, the Job picks the least-loaded one by counting existing cluster placements in the HyperFleet API against each MC's `gcp-hcp/max-clusters` capacity label (set by Terraform on the Secret Manager secret). If all MCs are at capacity, the adapter reports `Available: False` and downstream adapters skip processing until capacity frees up.

### DNS Zone Selection

Available DNS domains are identified by reading the regional ArgoCD cluster secret in Secret Manager (labeled `infra-type:region`), which contains a `meta_hc_dns_domains` field — a comma-separated list of base domains provisioned for hosted clusters. The Job selects the first available domain as the `baseDomain` for the cluster's API endpoint (e.g. `a1b2.gcp-hcp.devshift.net`). If no DNS domains are found, the Job reports `DNSPlacement: False` and the overall placement fails.

### Placement Decision Output

Written to the HyperFleet API cluster status `data` field so downstream adapters can read it:

```json
{
  "adapter": "placement-adapter",
  "observed_generation": 1,
  "conditions": [
    {"type": "Applied", "status": "True"},
    {"type": "Available", "status": "True"},
    {"type": "MCPlacement", "status": "True", "message": "dev-mgt-us-c1-a1b2"},
    {"type": "DNSPlacement", "status": "True", "message": "a1b2.gcp-hcp.devshift.net"},
    {"type": "Health", "status": "True"}
  ],
  "data": {
    "managementClusterName": "dev-mgt-us-c1-a1b2",
    "managementClusterNamespace": "clusters-{clusterId}",
    "baseDomain": "a1b2.gcp-hcp.devshift.net"
  }
}
```

The placement-decision Job patches its own `.status.conditions` with `MCPlacement` and `DNSPlacement` results. The adapter framework discovers these via label selectors and maps them into the status payload posted to the HyperFleet API.

## Preconditions & Gating

| Gate | CEL Expression (summary) |
|------|--------------------------|
| No existing placement | Cluster has no prior `placement-adapter` status with `Available: True` |

The adapter skips processing if a placement decision already exists for the current cluster. This prevents re-placement on every reconcile event.

## Status Reporting

Reports the three mandatory conditions plus two adapter-specific conditions:

| Condition | Meaning |
|-----------|---------|
| `Applied` | Job was successfully created on the region cluster |
| `Available` | Both MC and DNS placement succeeded |
| `MCPlacement` | Target management cluster selected (message contains MC name) |
| `DNSPlacement` | Base domain selected (message contains domain) |
| `Health` | Job completed without errors |

## Idempotency & Edge Cases

- **First run**: performs selection algorithm, writes placement decision
- **Subsequent runs (same generation)**: reads existing placement from prior status, reports same MC — skips re-selection
- **Generation change (spec update)**: placement is sticky — reports existing MC with bumped `observed_generation`. MC assignment does not change on spec update.
- **Re-placement**: not supported in MVP. If needed post-MVP, could be triggered via a dedicated annotation or admin API.

## Credentials

| Credential | Access | Source |
|-----------|--------|--------|
| GCP SA | Secret Manager secrets list + labels | Workload Identity on region cluster |
| Maestro API | Consumer list | In-cluster service URL (`http://maestro.hyperfleet.svc.cluster.local:8000`) |
| HyperFleet API | Cluster list + status POST | In-cluster service URL (`http://hyperfleet-api.hyperfleet.svc.cluster.local:8000`) |

## Design Alternatives Considered

None documented for MVP.

## Open Questions

- [ ] Should placement support weighted or priority-based MC selection beyond least-loaded?
- [ ] How should placement behave when an MC is being drained for decommissioning?

## Backlog

| Story | Jira | Status |
|-------|------|--------|
| Implement placement decision adapter for hosted cluster provisioning | [GCP-569](https://redhat.atlassian.net/browse/GCP-569) | In Progress |
