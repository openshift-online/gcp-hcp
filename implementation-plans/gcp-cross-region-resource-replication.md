# Cross-Region Resource Replication

## Overview

This document specifies the single-leader cross-region resource replication mechanism for Gecko. It defines leader/follower behavior, CLI routing, read-only enforcement, Pub/Sub event processing, follower mirror reconciliation, manual failover, and deployment topology.

This implements the architecture decided in [cross-region-resource-replication](../design-decisions/infrastructure/cross-region-resource-replication.md).

**Repository**: [gecko](https://github.com/openshift-online/gecko)

---

## Design Decisions

| Decision | Choice |
|---|---|
| Transport | Google Cloud Pub/Sub (shared topic, per-region subscriptions) |
| Write authority | Exactly one configured leader region |
| Follower behavior | Read-only mirror for replicated resource types |
| CLI routing | CLI routes reads and writes to the configured leader |
| Forced follower writes | Reject with structured read-only error; no server-side redirect |
| Leadership config | GitOps/Argo/Helm values |
| Failover | Manual promotion by changing leadership config after fencing old leader |
| Replication direction | Leader to followers only |
| Resource type selection | Startup flags or Helm values (`--replicate`) |
| Event validation | Followers apply only events from the configured leader |
| Conflict handling | No normal multi-writer conflict resolution; stale-event checks remain defensive |
| Delete recovery | Leader-authoritative inventory reconciliation and follower pruning |
| Namespace handling | Receiver auto-creates target namespace if missing |
| Private API writes | Restricted by follower RBAC except for the replication controller ServiceAccount |
| Error handling | Permanent errors Ack'd, logged at ERROR, and counted; transient errors Nack'd for Pub/Sub retry |
| Observability | Structured audit logs + Prometheus metrics for mode, lag, events, rejections, inventory, and split-brain |
| Initial use case | Authorization Roles and RoleBindings |

---

## Terminology

| Term | Meaning |
|---|---|
| Leader region | The single region configured to accept writes for replicated resource types |
| Follower region | A non-leader region that mirrors leader state and rejects direct writes |
| Mirror object | A local follower copy of an object whose authoritative source is the leader |
| Leadership config | GitOps/Argo/Helm-delivered configuration naming the current leader region |
| RPO | Recovery Point Objective: possible accepted write loss during failover, bounded by replication lag |
| RTO | Recovery Time Objective: time to restore write availability, bounded by detection, fencing, config rollout, and validation |

---

## Leadership Configuration

Each regional deployment receives both its own region and the configured leader region.

```yaml
replication:
  region: us-east1
  leaderRegion: us-east1
```

Follower example:

```yaml
replication:
  region: europe-west1
  leaderRegion: us-east1
```

Mode should be derived from `region == leaderRegion` rather than independently configured where possible:

```go
mode := "follower"
if cfg.Region == cfg.LeaderRegion {
    mode = "leader"
}
```

### Startup Flags

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--region` | `REPL_REGION` | (required) | This region's identifier |
| `--leader-region` | `REPL_LEADER_REGION` | (required) | Current leader region identifier |
| `--pubsub-project` | `PUBSUB_PROJECT` | `gecko-local` | GCP project ID for Pub/Sub |
| `--pubsub-topic` | `REPL_PUBSUB_TOPIC` | `resource-replication` | Pub/Sub topic name |
| `--pubsub-subscription` | `REPL_PUBSUB_SUBSCRIPTION` | (required) | Region-specific subscription name |
| `--replicate` | `REPL_RESOURCE_TYPES` | (required) | Comma-separated resource types to replicate, for example `roles.gcp.managed.openshift.io,rolebindings.gcp.managed.openshift.io` |
| `--resync-interval` | `REPL_RESYNC_INTERVAL` | `30m` | Leader interval for periodic resource resync and inventory publishing |

The `PUBSUB_EMULATOR_HOST` environment variable is supported for local development with the Pub/Sub emulator.

---

## CLI Routing

The CLI routes requests to the configured leader region.

### Behavior

1. Discover the current leader region and endpoint from the metadata service. The metadata service provides the CLI with regional configuration including the current leader region and its API endpoint.
2. Send reads and writes to the leader endpoint.
3. If the user explicitly forces a follower endpoint, surface the follower's read-only rejection.
4. Do not rely on server-side redirects for writes.
5. Cache the metadata service response briefly, and invalidate it when a read-only rejection indicates stale routing information (the leader may have changed since the last discovery).

### Follower Rejection Response

Follower public APIs reject mutating requests for replicated resource types with a structured response:

```json
{
  "error": "region is read-only",
  "leaderRegion": "us-east1",
  "leaderEndpoint": "https://api.us-east1.example",
  "retryable": false
}
```

The exact HTTP status can be selected during implementation. The key requirement is deterministic rejection with machine-readable leader metadata and no automatic server-side redirect.

---

## Public API Read-Only Enforcement

The public API server enforces leader-only writes for replicated resource types.

| Request | Leader Region | Follower Region |
|---|---|---|
| `GET` | Allow | Allow if exposed locally; CLI normally uses leader |
| `LIST` | Allow | Allow if exposed locally; CLI normally uses leader |
| `POST` | Allow after normal authorization | Reject read-only |
| `PUT` | Allow after normal authorization | Reject read-only |
| `PATCH` | Allow after normal authorization | Reject read-only |
| `DELETE` | Allow after normal authorization | Reject read-only |

The read-only guard runs after authentication and before resource mutation. Authorization still applies normally in the leader. Follower rejection is not an authorization success; it is a regional mode constraint.

---

## Private API and RBAC

Follower regions restrict human/operator writes through private API RBAC while preserving replication controller write access.

### Goals

* Human and operator identities should not create, update, patch, or delete replicated resource types in follower regions.
* The replication controller ServiceAccount must be able to create, update, patch, and delete replicated resource types in follower regions so it can apply leader state.
* Namespace read/create permissions remain available to the replication controller when namespace auto-creation is enabled.
* Break-glass access, if required, must be explicit, audited, and outside the normal role bindings.

Example replication controller ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: replication-controller
rules:
  - apiGroups: ["gcp.managed.openshift.io"]
    resources: ["roles", "rolebindings"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch", "create"]
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
```

The RBAC rules for human/operator identities are environment-specific, but follower regions must not grant normal write verbs for replicated resource types to those identities.

---

## Annotations

The replication controller uses annotations to mark follower mirror objects. Annotations are not used for ownership transfer.

### Replicated-From Annotation

```text
replication.gcp.managed.openshift.io/replicated-from: <leader-region>
```

Set by the Receiver on objects it creates or updates from leader replication events. This indicates the object is a follower mirror of leader-authoritative state.

### Last-Applied Annotation

```text
replication.gcp.managed.openshift.io/last-applied: <RFC3339 timestamp>
```

Optional defensive metadata set by the Receiver after applying a leader event. It can be used to reject stale events and expose replication lag. The implementation may instead use persisted object timestamps if those are reliable through the `ResourceStore` interface.

---

## Replication Event Model

Events are serialized as JSON and published to Pub/Sub as message payloads.

```go
type ReplicationEvent struct {
    EventType    string          `json:"eventType"`    // CREATE_OR_UPDATE, DELETE, RESYNC_REQUEST, INVENTORY
    ResourceKind string          `json:"resourceKind"` // e.g. Role, RoleBinding
    OriginRegion string          `json:"originRegion"` // Must be the configured leader for data events
    Namespace    string          `json:"namespace"`    // Empty for cluster-scoped resources
    Name         string          `json:"name"`         // Resource name
    UpdatedAt    time.Time       `json:"updatedAt"`    // Defensive stale-event check
    SyncID       string          `json:"syncID"`       // Inventory/resync correlation ID
    Object       json.RawMessage `json:"object"`       // Full resource for CREATE_OR_UPDATE only
    Inventory    []ObjectRef     `json:"inventory"`    // Complete object list for INVENTORY only
}

type ObjectRef struct {
    Namespace string    `json:"namespace"`
    Name      string    `json:"name"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

| Event Type | Description |
|---|---|
| `CREATE_OR_UPDATE` | Leader upsert for one object |
| `DELETE` | Leader deletion for one object |
| `RESYNC_REQUEST` | Follower asks the leader to immediately republish current state |
| `INVENTORY` | Leader publishes the complete desired object set for one resource kind |

For large datasets, `INVENTORY` can be split into chunked events. Chunked inventories share the same `SyncID` value. Each chunk includes `ChunkIndex` (zero-based) and `ChunkCount` (total number of chunks), or alternatively a `Final` flag on the last chunk. Followers must assemble all chunks for a given `SyncID` and resource kind before pruning, and must discard incomplete chunk sets after a timeout. The full chunk envelope schema, assembly algorithm, and error handling are deferred to future work — the initial authorization use case is expected to fit within a single `INVENTORY` message.

---

## Publisher

The Publisher is active only in the leader region. It uses controller-runtime reconcilers, one per configured resource type, to watch leader-local resource changes and publish replication events to the shared Pub/Sub topic.

### Setup

```go
func (p *Publisher) SetupWithManager(mgr ctrl.Manager) error {
    if p.region != p.leaderRegion {
        return nil
    }

    for _, gvk := range p.replicatedTypes {
        ctrl.NewControllerManagedBy(mgr).
            For(resourceForGVK(gvk)).
            Complete(reconcilerFor(gvk, p))
    }
    return nil
}
```

### Publishing Logic

On leader reconcile:

1. **Object not found**: Publish a `DELETE` event with resource kind, namespace, name, origin region, and timestamp.
2. **Object found**: Serialize the object and publish a `CREATE_OR_UPDATE` event.

On follower reconcile:

1. The publisher is not registered.
2. No local watch event is published.
3. Any attempt by a follower to publish a data event (`CREATE_OR_UPDATE`, `DELETE`, `INVENTORY`) is an error and should increment `replication_events_rejected_total{reason="follower_publish"}`.

Failed publishes are requeued after a short delay, for example 5 seconds.

### Periodic Resync and Inventory

On `--resync-interval`, the leader re-lists all resources of each configured type and publishes:

1. `CREATE_OR_UPDATE` events for current leader objects.
2. An `INVENTORY` event containing the complete current object set for that resource kind.

The resync repairs missed create/update events. The inventory allows followers to prune mirror objects that no longer exist in leader-authoritative state, which repairs missed delete events.

### Resync Request Handling

When a follower starts or detects drift, it publishes a `RESYNC_REQUEST`. Only the configured leader responds by triggering an immediate resync and inventory publish. Followers do not respond to `RESYNC_REQUEST` events.

**Follower publish path**: `RESYNC_REQUEST` is a control-plane signal, not a data event. Followers publish it directly through a standalone Pub/Sub client, independent of the controller-runtime Publisher reconcilers (which are not registered in followers). The `replication_events_rejected_total{reason="follower_publish"}` metric applies only to data events (`CREATE_OR_UPDATE`, `DELETE`, `INVENTORY`), not to `RESYNC_REQUEST`.

---

## Receiver

The Receiver subscribes to the Pub/Sub topic via a region-specific subscription and processes incoming replication events.

### Message Processing

```text
Message received
  |-- Deserialize ReplicationEvent from JSON
  |-- If event.OriginRegion == receiver.region: Ack and skip
  |-- If data event origin != configured leader: Ack, log ERROR, increment rejected metric
  |-- Route by EventType:
  |     |-- CREATE_OR_UPDATE -> upsert()
  |     |-- DELETE -> delete()
  |     |-- RESYNC_REQUEST -> leader-only immediate resync
  |     |-- INVENTORY -> reconcileInventory()
  |-- Error handling:
        |-- Permanent error -> Ack, log ERROR, increment replication_events_dropped_total
        |-- Transient error -> Nack for Pub/Sub retry
```

### Upsert Flow

1. Reject the event unless `event.OriginRegion == leaderRegion`.
2. Deserialize the incoming resource from `event.Object`.
3. If the resource is namespaced, ensure the target namespace exists.
4. Set `replicated-from` to `event.OriginRegion`.
5. Clear `ResourceVersion`.
6. Get the existing object.
7. If not found, create it.
8. If found, update it only when the incoming event is newer than the local last-applied timestamp or stored update timestamp.
9. Ack stale duplicate events after counting them.

### Delete Flow

1. Reject the event unless `event.OriginRegion == leaderRegion`.
2. Delete the resource by kind, namespace, and name.
3. Treat NotFound as success.

### Namespace Auto-Creation

```go
func (r *Receiver) ensureNamespace(ctx context.Context, namespace string) error {
    ns := &corev1.Namespace{
        ObjectMeta: metav1.ObjectMeta{Name: namespace},
    }
    err := r.client.Create(ctx, ns)
    if err != nil && !apierrors.IsAlreadyExists(err) {
        return err
    }
    return nil
}
```

This requires the replication controller's RBAC to include `get`, `list`, `watch`, and `create` on `namespaces`.

---

## Leader Inventory Reconciliation

Follower mirrors are reconciled against leader-authoritative inventory to recover from dropped delete events and manual follower drift.

### Inventory Event

For each configured resource kind, the leader periodically publishes the complete set of current leader objects:

```json
{
  "eventType": "INVENTORY",
  "resourceKind": "RoleBinding",
  "originRegion": "us-east1",
  "syncID": "2026-09-02T12:00:00Z/us-east1/rolebindings",
  "inventory": [
    {"namespace": "customer-a", "name": "service-admin", "updatedAt": "2026-09-02T11:58:00Z"}
  ]
}
```

### Follower Pruning Rules

Followers prune only when all of these conditions are true:

* The inventory event is from the configured leader.
* The inventory is complete for one resource kind and one sync ID.
* The inventory is newer than the last completed inventory for that resource kind.
* The local object being considered has `replicated-from: <leader-region>`.
* The local object is absent from the completed leader inventory.

Followers must not prune when:

* The inventory is incomplete or chunk assembly failed.
* The inventory is stale.
* The inventory origin is not the configured leader.
* The object is not marked as a mirror from the leader.
* The resource type is not configured for replication.

This replaces the prior replicated-object lease and refresh-deadline garbage collection model.

---

## Pub/Sub Topology

```text
Leader Region (Publisher)  --publish-->  Pub/Sub Topic  <--RESYNC_REQUEST--  Followers
                                             |
                          +------------------+------------------+
                          v                  v                  v
                   Subscription L      Subscription A     Subscription B
                   Leader                Follower A         Follower B
                   Receiver handles      Receiver applies   Receiver applies
                   RESYNC_REQUEST only   leader events      leader events
```

Each region has:

* A **Receiver** that subscribes to the topic via a region-specific subscription. The leader's Receiver processes only `RESYNC_REQUEST` events (its own data events are skipped via the origin-region check). Follower Receivers process `CREATE_OR_UPDATE`, `DELETE`, and `INVENTORY` events from the leader.
* A **Publisher** that is active only when `region == leaderRegion`. Followers do not register Publisher reconcilers but can publish `RESYNC_REQUEST` control-plane signals directly through a standalone Pub/Sub client (see [Resync Request Handling](#resync-request-handling)).

The shared topic name and per-region subscription names are configured through Helm values and startup flags.

---

## Deployment

### Controller Binary

The replication controller is a subcommand of the existing `gecko-controllers` binary:

```text
gecko-controllers replication \
  --region=us-east1 \
  --leader-region=us-east1 \
  --pubsub-subscription=repl-us-east1 \
  --replicate=roles.gcp.managed.openshift.io,rolebindings.gcp.managed.openshift.io
```

### Containerfile

Reuses the `gecko-controllers` binary image. The entrypoint specifies the `replication` subcommand:

```dockerfile
FROM registry.access.redhat.com/ubi9/ubi-micro:latest
COPY gecko-controllers /app/gecko-controllers
USER 65532:65532
ENTRYPOINT ["/app/gecko-controllers", "replication"]
```

### Kubernetes Deployment

Single replica per region in the `gecko-system` namespace is sufficient for the initial implementation. Environment variables configure region, leader region, Pub/Sub project, topic, subscription, and emulator host for local development.

---

## Local Development

### Pub/Sub Emulator

Local development uses the Google Cloud Pub/Sub emulator (`gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators`). In a multi-cluster Kind setup, the emulator runs as a standalone container on the Docker/Podman network shared by all clusters.

### Kind Multi-Cluster Setup

The `deploy/kind/setup-multi-region.sh` script:

1. Creates at least two Kind clusters, for example `gecko-us-east1` and `gecko-eu-west1`.
2. Starts a shared Pub/Sub emulator container.
3. Creates the shared topic and per-region subscriptions.
4. Builds and loads controller images into all clusters.
5. Deploys via Kustomize or Helm with one leader overlay and one or more follower overlays.
6. Configures in-cluster Services pointing to the shared emulator.

### Kustomize or Helm Overlays

Each region patches:

* `REPL_REGION`
* `REPL_LEADER_REGION`
* `REPL_PUBSUB_SUBSCRIPTION`
* Pub/Sub emulator host for local development

---

## Manual Failover

Leadership changes are manual. Automatic cross-region leader election is intentionally out of scope for the initial implementation.

### Failover Procedure

1. Declare leader outage or planned failover.
2. Fence the old leader or confirm it cannot accept writes. **RPO note**: any writes accepted by the old leader but not yet published to Pub/Sub, or published but not yet delivered to follower subscriptions, are at risk of loss. The data loss window is bounded by the target follower's replication lag at the moment of failover. Check `replication_lag_seconds` and `replication_last_leader_event_timestamp` on the target follower before proceeding.
3. Select the target follower region.
4. Check the target follower's last applied leader event and replication lag.
5. Update GitOps/Argo/Helm config so `leaderRegion` points to the target region.
6. Sync the target region so it starts accepting writes and publishing events.
7. Sync remaining regions so they become followers of the new leader.
8. Verify CLI discovery points to the new leader.
9. Verify writes succeed only in the new leader.
10. Verify forced writes to all followers are rejected.
11. Monitor replication lag, rejected events, and split-brain alerts.

### Failback Procedure

1. The old leader comes back as a follower/read-only region.
2. It catches up from the current leader through normal replication.
3. Operators verify convergence.
4. Optional promotion back to the original region uses the same failover procedure.

### Split-Brain Guardrails

* Mode is derived from `region == leaderRegion`.
* Followers reject public writes.
* Follower RBAC restricts private writes by human/operator identities.
* Followers reject replication events from non-leader origins.
* Alerts fire if more than one region reports leader mode.
* Alerts fire if a follower publishes replication events.

---

## Observability

### Audit Logs

All replication operations emit structured log entries using the controller-runtime logger.

**Publisher:**

| Level | Event | Fields |
|---|---|---|
| INFO | Published event | `eventType`, `resourceKind`, `namespace`, `name`, `originRegion` |
| INFO | Periodic resync completed | `resourceKind`, `resourcesPublished`, `syncID` |
| INFO | Published inventory | `resourceKind`, `objectCount`, `syncID` |
| WARN | Publish failed, requeuing | `resourceKind`, `namespace`, `name`, `error` |
| ERROR | Follower attempted publish | `region`, `leaderRegion`, `eventType` |

**Receiver:**

| Level | Event | Fields |
|---|---|---|
| INFO | Upserted resource | `resourceKind`, `namespace`, `name`, `originRegion`, `outcome` |
| INFO | Deleted resource | `resourceKind`, `namespace`, `name`, `originRegion` |
| INFO | Created namespace | `namespace` |
| INFO | Received `RESYNC_REQUEST` | `originRegion` |
| INFO | Applied inventory | `resourceKind`, `originRegion`, `syncID`, `objectCount`, `prunedCount` |
| WARN | Rejected stale event | `resourceKind`, `namespace`, `name`, `originRegion`, `updatedAt` |
| ERROR | Rejected non-leader event | `eventType`, `resourceKind`, `namespace`, `name`, `originRegion`, `leaderRegion` |
| ERROR | Permanent error, Ack'd | `eventType`, `resourceKind`, `namespace`, `name`, `originRegion`, `error` |

**Public API:**

| Level | Event | Fields |
|---|---|---|
| INFO | Rejected follower write | `region`, `leaderRegion`, `resourceKind`, `namespace`, `name`, `verb` |

### Prometheus Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `replication_region_mode_info` | Gauge | `region`, `leader_region`, `mode` | Current regional replication mode |
| `replication_events_published_total` | Counter | `region`, `event_type`, `resource_kind` | Events published by the leader |
| `replication_events_applied_total` | Counter | `region`, `event_type`, `resource_kind`, `origin_region` | Events applied by receivers |
| `replication_events_rejected_total` | Counter | `region`, `event_type`, `resource_kind`, `reason`, `origin_region` | Events rejected before apply |
| `replication_events_dropped_total` | Counter | `region`, `event_type`, `resource_kind`, `reason` | Permanent errors Ack'd |
| `replication_readonly_write_rejections_total` | Counter | `region`, `resource_kind`, `verb` | Public API writes rejected in followers |
| `replication_publish_errors_total` | Counter | `region`, `resource_kind` | Publish failures that are requeued |
| `replication_publish_duration_seconds` | Histogram | `event_type` | Time to publish a single event |
| `replication_receive_duration_seconds` | Histogram | `event_type` | Time to process a received event |
| `replication_resync_duration_seconds` | Histogram | `resource_kind` | Time for leader resync and inventory |
| `replication_inventory_sync_total` | Counter | `region`, `resource_kind`, `outcome` | Inventory sync outcomes |
| `replication_inventory_pruned_objects_total` | Counter | `region`, `resource_kind` | Mirror objects pruned from completed leader inventory |
| `replication_lag_seconds` | Gauge | `region`, `leader_region` | Time since the last applied leader event |
| `replication_last_leader_event_timestamp` | Gauge | `region`, `leader_region` | Unix timestamp of last applied leader event |
| `replication_split_brain_detected_total` | Counter | `region` | Split-brain detection events |

### Recommended Alerts

| Alert | Condition | Severity |
|---|---|---|
| No leader configured | No region reports `mode="leader"` | Critical |
| Multiple leaders configured | More than one region reports `mode="leader"` | Critical |
| Follower publishing events | `rate(replication_events_rejected_total{reason="follower_publish"}[5m]) > 0` | Critical |
| Non-leader events received | `rate(replication_events_rejected_total{reason="non_leader_origin"}[5m]) > 0` | Critical |
| Follower replication lag high | `replication_lag_seconds > threshold` | Warning |
| Read-only write rejections spike | Unexpected increase in `replication_readonly_write_rejections_total` | Warning |
| Inventory pruning failed | `replication_inventory_sync_total{outcome="failed"}` increases | Warning |
| Sustained publish failures | `rate(replication_publish_errors_total[5m]) > 0.1` for 10m | Warning |

---

## Integration: Authorization Use Case

The initial use case for cross-region replication is authorization Roles and RoleBindings. The integration points with the Cedar authorization system are:

* **Replicated resources**: `roles.gcp.managed.openshift.io` and `rolebindings.gcp.managed.openshift.io`
* **Write authority**: Role and RoleBinding writes go to the leader region.
* **Follower reads**: Follower regions can evaluate authorization from local mirror data, subject to replication lag. The CLI uses the leader for reads by default.
* **PlatformRoles are NOT replicated**: They are system-defined and deployed identically to all regions via Helm.
* **Cedar hot-reload interaction**: When the receiver creates, updates, or deletes a mirrored Role or RoleBinding in the local database, the Cedar authorizer's watch mechanism detects the change and triggers policy rebuild and cache invalidation.
* **Marketplace integration**: The Marketplace controller creates the initial `service-admin` RoleBinding through the leader. The replication controller propagates it to follower regions.
* **Leader outage behavior**: During a leader outage, follower regions continue to evaluate Cedar authorization decisions from their local mirror data. Authorization remains available but may become stale — no new Roles or RoleBindings can be created, updated, or deleted until a new leader is promoted. Existing authorization grants remain in effect. The staleness window is bounded by the time between the leader outage and the next successful failover.

---

## E2E Test Coverage

The E2E test suite runs against one leader Kind cluster and at least one follower Kind cluster.

| # | Test | What It Validates |
|---|---|---|
| 1 | Leader create replication | Resource created in leader appears in follower with `replicated-from` annotation |
| 2 | Leader update replication | Resource updated in leader updates follower mirror |
| 3 | Leader delete replication | Resource deleted in leader is removed from follower |
| 4 | RoleBinding replication | Namespace-scoped binding replicates correctly |
| 5 | Namespace auto-creation | Receiver creates missing namespace in follower |
| 6 | CLI leader routing | CLI sends reads and writes to leader endpoint |
| 7 | Forced follower public write rejected | Direct mutating request to follower returns read-only error |
| 8 | Private follower write blocked | Non-replication private API identity cannot write replicated resource type in follower |
| 9 | Replication controller follower write allowed | Replication controller can apply leader data in follower |
| 10 | Follower does not publish | Local follower changes do not produce replication events |
| 11 | Non-leader event rejected | Follower rejects replication event whose origin is not configured leader |
| 12 | New region bootstrap | Follower publishes `RESYNC_REQUEST`; leader republishes current state |
| 13 | Periodic resync repairs drift | Follower object manually deleted reappears after leader resync |
| 14 | Inventory prunes stale mirror | Follower object absent from completed leader inventory is deleted |
| 15 | Incomplete inventory does not prune | Follower preserves mirrors when inventory is incomplete or invalid |
| 16 | Manual failover | Follower promoted through config accepts writes; old leader becomes follower |
| 17 | Old leader rejoins as follower | Recovered old leader catches up from current leader |
| 18 | Split-brain detection | Multiple leader reports trigger detection/alert metric |

Tests use polling with a 30-second timeout. A `--no-pause` flag supports CI execution.

---

## File Structure

```text
controllers/
  replication/
    publisher.go                        # Leader-only controller-runtime reconcilers for publishing
    publisher_test.go
    receiver.go                         # Pub/Sub message handler with validation, upsert, delete, inventory
    receiver_test.go
    inventory.go                        # Leader inventory publishing and follower pruning helpers
    inventory_test.go
    types.go                            # Replication event structs + constants
  cmd/replication/
    cmd.go                              # Cobra subcommand + wiring
deploy/kind/
  replication/
    Containerfile.controller            # Container image for replication controller
    controller-deployment.yaml          # Kubernetes deployment
    kustomization.yaml                  # Base kustomization
    rbac.yaml                           # ServiceAccount + ClusterRole + ClusterRoleBinding
    test/
      e2e-test.sh                       # Leader/follower E2E test suite
  setup-multi-region.sh                 # Multi-cluster Kind setup script
  teardown-multi-region.sh              # Cleanup script
deploy/multi-region/
  us-east1/kustomization.yaml           # Region overlay, leader in default local setup
  eu-west1/kustomization.yaml           # Region overlay, follower in default local setup
```
