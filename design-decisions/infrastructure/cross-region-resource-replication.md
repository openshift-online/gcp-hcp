# Cross-Region Resource Replication

***Scope***: GCP-HCP

**Date**: 2026-08-25

## Decision

We will implement cross-region replication for selected Gecko resources using a single-leader, follower-mirror model over Google Cloud Pub/Sub. Exactly one configured leader region accepts writes for replicated resource types. All non-leader regions keep read-only local mirrors of the leader's data and reject direct write attempts. The CLI routes requests to the configured leader region, so normal user traffic does not write to follower regions. Leadership changes are manual and controlled through GitOps/Argo/Helm configuration.

Replication is unidirectional: the leader publishes create, update, delete, bootstrap, and inventory events; followers subscribe to the shared Pub/Sub topic and apply only events from the configured leader. Followers never publish replicated resource changes and never become authoritative unless operators manually promote them by changing the leadership configuration. The initial use case is replicating authorization Roles and RoleBindings across regional Gecko instances, but the mechanism is designed to support any configured API resource type.

The design intentionally trades independent regional write authority for a simpler consistency model. Reads and authorization decisions can be served from local follower mirrors, but writes are available only through the leader until a manual failover promotes another region.

## Context

- **Problem Statement**: Gecko operates multiple regional instances, each with its own database. Certain resource types, starting with authorization Roles and RoleBindings, must be consistent across regions so that access granted through Gecko is available everywhere. The previous multi-region authoritative design allowed any region to write and relied on conflict resolution. We now want one authoritative write region to avoid concurrent-write conflicts and make behavior deterministic.
- **Constraints**: Writes for replicated resource types must be accepted by only one leader region at a time. Non-leader regions must reject direct public API writes. The CLI must route to the leader. Replication must remain asynchronous so follower read availability does not depend on cross-region calls. Leadership changes must be manual and auditable through GitOps/Argo/Helm. The private API must restrict human/operator writes in follower regions while preserving replication controller write permission so leader data can be applied locally. The mechanism must support namespaced and cluster-scoped resources, work with the existing `ResourceStore` interface and controller-runtime framework, and support local development via the Pub/Sub emulator.
- **Assumptions**: Eventual consistency is acceptable for follower reads and authorization cache updates. Replicated resource writes are infrequent administrative operations, so manual failover with minute-scale recovery is acceptable. There should always be exactly one configured leader region. PlatformRoles are system-defined, cluster-scoped resources deployed identically to all regions via Helm and do not need replication.

## Alternatives Considered

1. **Single-leader Pub/Sub replication with read-only followers**: One configured leader region accepts writes and publishes all replicated resource changes. Followers reject writes, apply only leader-origin events, and maintain local mirrors for reads and authorization decisions. Leadership changes are manual through GitOps/Argo/Helm.
2. **Multi-region authoritative Pub/Sub replication**: Every region accepts writes, publishes locally-owned resources, and resolves concurrent changes with last-writer-wins timestamps. This maximizes regional write availability but risks silent conflict loss and requires ownership-transfer semantics.
3. **Shared global Spanner instance**: All regions use one global database for replicated resource types.
4. **No replication**: Each region manages its own resources independently. Cross-region consistency is handled manually or by external orchestration.

## Decision Rationale

* **Justification**: The single-leader model provides a clear source of truth for globally replicated Gecko control data. It removes normal-path concurrent writes, ownership transfer, and last-writer-wins conflict resolution. CLI leader routing gives users a consistent write endpoint, while follower write rejection protects against forced or stale requests. Manual leadership changes through GitOps/Argo/Helm are auditable and safer than automatic cross-region leader election for this low-write-volume use case. Pub/Sub keeps replication asynchronous and allows followers to continue serving local reads when the leader or transport is unavailable.
* **Evidence**: The existing proof-of-concept showed that Pub/Sub fan-out can replicate Roles and RoleBindings across regions and that namespace auto-creation works for non-primary regions. The new model keeps the proven transport and receiver mechanics but removes multi-writer ownership transfer and conflict-resolution complexity.
* **Comparison**: Multi-region authoritative replication provides better write locality but creates ambiguous behavior when the same object is changed in multiple regions. Shared global Spanner simplifies consistency at the storage layer but introduces a cross-region request-path dependency and conflicts with the broader regional architecture. No replication pushes consistency management to external systems and does not scale to user-defined resources that must be available across regions.

## Consequences

### Positive

* Clear write authority: exactly one leader region owns replicated resource writes.
* Simpler consistency model: no normal-path multi-writer conflict resolution or ownership transfer.
* Followers provide local mirrored reads and authorization decisions.
* Forced writes to read-only regions are rejected deterministically.
* CLI can route all reads and writes to the leader for read-after-write consistency.
* Replication remains storage-backend agnostic through the `ResourceStore` interface.
* Namespace auto-creation in follower regions ensures replicated namespaced resources have valid target namespaces.
* Manual leadership changes are auditable through GitOps/Argo/Helm.
* Leader inventory reconciliation can repair dropped delete events and follower drift without lease expiration.

### Negative

* A leader outage makes writes unavailable until operators manually promote another region.
* Cross-region write latency increases for users far from the leader.
* Follower reads can lag behind the leader because replication is asynchronous.
* Failover has an RPO bounded by replication lag: writes accepted by the failed leader but not yet replicated to the promoted follower may be lost.
* Failover has an RTO bounded by detection, fencing, GitOps/Argo/Helm rollout, and validation time.
* GitOps rollout skew can create split-brain risk if more than one region accepts writes; fencing and alerts are required.
* Pub/Sub remains at-least-once and unordered, so receivers must be idempotent and reject stale or non-leader events.
* This is an intentional exception to the regional independence architecture for globally consistent Gecko control data.

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Each replicated resource type adds one leader-side controller-runtime reconciler. Followers process all replicated resource types from their regional Pub/Sub subscription. Inventory events add traffic proportional to the number of replicated objects, but the initial authorization use case is expected to be small.
* **Observability**: Regions expose their configured `region`, `leaderRegion`, and derived mode (`leader` or `follower`). Metrics track leader events published, follower events applied, rejected events, read-only write rejections, replication lag, inventory sync outcomes, inventory pruning, and split-brain detection. Alerts fire when no leader is configured, more than one region reports leader mode, a follower publishes events, a follower receives non-leader events, replication lag exceeds threshold, or inventory pruning fails.
* **Resiliency**: Pub/Sub unavailability does not affect leader-local writes or follower-local reads, but followers become stale until delivery resumes. The leader periodically publishes a full inventory for each replicated resource type so followers can repair drift and prune mirrored objects that no longer exist in leader-authoritative state. Manual failover promotes a follower by updating GitOps/Argo/Helm leadership config after fencing or confirming the old leader cannot accept writes.

### Security:

* Public API write authorization is leader-aware. Non-leader regions reject mutating requests for replicated resource types even when the caller is otherwise authorized.
* The CLI routes normal traffic to the leader. A caller who forces a request to a follower receives a structured read-only error rather than a redirect.
* Private API/RBAC restricts human and operator writes to replicated resource types in follower regions.
* The replication controller ServiceAccount keeps the local write permissions required to create, update, and delete mirrored objects in follower regions.
* Followers apply only replication events whose `OriginRegion` matches the configured leader. Events from any other origin are rejected, logged, and counted.
* The `replication.gcp.managed.openshift.io/replicated-from` annotation marks objects mirrored from the leader. It is not an ownership-transfer mechanism.
* Pub/Sub authentication uses GCP Workload Identity in production or the emulator for local development.

### Performance:

* Replication is asynchronous and not on the follower read path.
* Leader writes incur normal local write latency plus asynchronous Pub/Sub publish latency.
* Users far from the leader may see higher write latency because the CLI routes reads and writes to the leader.
* Followers serve local reads from their mirrored database state, subject to replication lag.

### Cost:

* Pub/Sub cost scales with leader event volume plus periodic inventory events.
* No additional database cost: mirrored resources are stored in the same regional database as other resources.
* One replication controller deployment runs per region, with publisher behavior enabled only in the leader.

### Operability:

* Leadership is configured through GitOps/Argo/Helm values. Mode should be derived from `region == leaderRegion` where possible.
* Manual failover requires fencing or confirming the old leader cannot accept writes, updating leadership config, syncing deployments, and validating that only the new leader accepts writes.
* Failback is a planned promotion operation. A recovered old leader rejoins as a follower, catches up from the current leader, and is promoted back only through the same manual process.
* Local development uses the Google Cloud Pub/Sub emulator with one leader Kind cluster and one or more follower Kind clusters.
* The E2E test suite validates leader-to-follower replication, follower read-only enforcement, CLI leader routing, inventory drift repair, manual failover, and split-brain detection.
