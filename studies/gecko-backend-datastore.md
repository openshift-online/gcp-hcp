# Datastore Study: Non-etcd Backend for Gecko Platform API

**Date:** 2026-07-31
**Branch:** `study-kube-api-backend-datastore-2`
**Status:** Draft

## Table of Contents

- [1. Executive Summary](#1-executive-summary)
- [2. Problem Statement](#2-problem-statement)
- [3. Requirements](#3-requirements)
- [4. Architecture Decision: rest.Storage vs storage.Interface](#4-architecture-decision-reststorage-vs-storageinterface)
- [5. Prior Art](#5-prior-art)
  - [5.1 Kine (k3s)](#51-kine-k3s)
  - [5.2 postgres-controller-backend](#52-postgres-controller-backend)
  - [5.3 sample-apiserver and apiserver-builder-alpha](#53-sample-apiserver-and-apiserver-builder-alpha)
- [6. GCP Datastore Alternatives](#6-gcp-datastore-alternatives)
  - [6.1 Option A: Cloud Spanner (Recommended)](#61-option-a-cloud-spanner-recommended)
  - [6.2 Option B: Cloud SQL for PostgreSQL](#62-option-b-cloud-sql-for-postgresql)
  - [6.3 Option C: AlloyDB for PostgreSQL](#63-option-c-alloydb-for-postgresql)
  - [6.4 Option D: Firestore (Native Mode)](#64-option-d-firestore-native-mode)
  - [6.5 Option E: Cloud Bigtable](#65-option-e-cloud-bigtable)
  - [6.6 Rejected Alternative](#66-rejected-alternative)
- [7. Comparison Matrix](#7-comparison-matrix)
- [8. Database Schema Proposals](#8-database-schema-proposals)
  - [8.1 SQL Schema (Cloud SQL / AlloyDB)](#81-sql-schema-cloud-sql--alloydb)
  - [8.2 Spanner Schema](#82-spanner-schema)
- [9. Key Implementation Patterns](#9-key-implementation-patterns)
  - [9.1 Monotonic Resource Versioning](#91-monotonic-resource-versioning)
  - [9.2 Watch / Change Notification](#92-watch--change-notification)
  - [9.3 Optimistic Concurrency Control](#93-optimistic-concurrency-control)
  - [9.4 Compaction and Garbage Collection](#94-compaction-and-garbage-collection)
  - [9.5 Multi-Replica Consistency](#95-multi-replica-consistency)
- [10. PoC Addendums](#10-poc-addendums)
  - [10.1 Bigtable PoC (PR #16)](#101-bigtable-poc-pr-16)
  - [10.2 Firestore PoC (PR #17)](#102-firestore-poc-pr-17)
- [11. Recommendation](#11-recommendation)
- [12. References](#12-references)

---

## 1. Executive Summary

This study evaluates non-etcd datastore options for the Gecko platform API server running on Google Cloud Platform. etcd poses challenges in resiliency, scalability (8GB limit), point-in-time recovery, and operability. We need a zero-touch, regionalized, cost-effective managed datastore.

**Key findings:**

- **Architecture:** Gecko should continue implementing `rest.Storage` interfaces directly (current approach), not `k8s.io/apiserver/pkg/storage.Interface`. No production non-etcd `storage.Interface` implementations exist, and the interface has no API stability guarantees.

- **Recommended option:** **Cloud Spanner** (regional, 2 nodes) — proven k8s storage replacement ([spanner-etcd](https://github.com/n0rm4l-me/spanner-etcd), [GKE 65k nodes](https://cloud.google.com/blog/products/containers-kubernetes/gke-65k-nodes-and-counting)), Change Streams for watch, zero-downtime operations, strong consistency, excellent Config Connector support. ~$1,314/month for production with SLA (reducible to ~$788/month with 3-year CUD).

- **Cost-effective alternative:** **Cloud SQL for PostgreSQL** — cheapest production option (~$50-120/month HA), PostgreSQL ecosystem, LISTEN/NOTIFY for watch, proven via kine and postgres-controller-backend. Trade-off: 5-10 min downtime during automatic updates, not serverless.

- **Premium alternative:** **AlloyDB for PostgreSQL** — best PITR (35 days, microsecond granularity), near-zero downtime maintenance, PostgreSQL compatible. Trade-off: expensive (~$200-400/month minimum HA).

- **Additional options:** **Firestore** (serverless, cheapest, real-time listeners, but counter bottleneck and document model mismatch) and **Bigtable** (massive scale, but no cross-row transactions, expensive minimum, counter hotspot). Both have working PoCs ([PR #16](https://github.com/openshift-online/gecko/pull/16), [PR #17](https://github.com/openshift-online/gecko/pull/17)) with known gaps.

- **Rejected:** Memorystore Redis (ephemeral, no durability).

---

## 2. Problem Statement

The Gecko platform API needs a persistent datastore backend that satisfies Kubernetes API server semantics while running on GCP. The reasons for avoiding etcd are:

| Concern | etcd Limitation |
|---------|----------------|
| **Resiliency** | Raft quorum requires 3-5 nodes; losing majority = cluster down |
| **Scalability** | Hard 8GB database size limit |
| **Point-in-time recovery** | Snapshot-only; no fine-grained PITR |
| **Operability** | Manual upgrades, quorum management, compaction tuning |
| **Cost** | Dedicated VM fleet per region for a small-scale API |

**Constraints:**

- **Regional:** All infra within a single region, duplicated per region (data sovereignty)
- **Zero-touch:** Seamless, autonomous upgrades; minimal operational burden
- **Managed via Config Connector** (fallback: Terraform if Config Connector support is poor)
- **Multi-replica API server:** Multiple replicas reading/writing concurrently
- **Cost-conscious:** Datastore duplicated in every region

---

## 3. Requirements

Requirements are derived from the [Kubernetes storage.Interface contract](https://pkg.go.dev/k8s.io/apiserver/pkg/storage), the [postgres-controller-backend design invariants](https://github.com/openshift-online/gecko), and Gecko's `ResourceStore` interface.

### Functional Requirements

| ID | Requirement | Source |
|----|-------------|--------|
| **F1** | Atomic create (fail if already exists) | k8s storage.Interface |
| **F2** | Optimistic concurrency (reject stale updates via resource version comparison) | k8s storage.Interface, postgres-controller-backend I6 |
| **F3** | Monotonically increasing resource versions (no gaps required, but monotonic) | k8s storage.Interface, postgres-controller-backend I4 |
| **F4** | Watch with ordered event delivery (events in resource version order) | k8s storage.Interface |
| **F5** | Watch historical replay from a given resource version | k8s storage.Interface |
| **F6** | List with pagination (stable cursor across pages) | k8s storage.Interface |
| **F7** | Label selector filtering | k8s storage.Interface |
| **F8** | Namespace scoping | k8s storage.Interface |
| **F9** | Soft delete with finalizer support (deletionTimestamp lifecycle) | Gecko aggregated adapter |
| **F10** | Status subresource (update status independently from spec) | Gecko aggregated adapter |
| **F11** | Compaction / tombstone garbage collection with 410 Gone on stale watchers | k8s storage.Interface, postgres-controller-backend I5 |

### Non-Functional Requirements

| ID | Requirement | Source |
|----|-------------|--------|
| **N1** | Strong consistency across all replicas (linearizable writes) | k8s storage.Interface |
| **N2** | Multi-replica safety (concurrent writers must not corrupt data) | postgres-controller-backend I1-I7 |
| **N3** | Regional deployment (single region, data sovereignty) | Project constraint |
| **N4** | Zero-touch operations (autonomous upgrades, no maintenance burden) | Project constraint |
| **N5** | Point-in-time recovery | Project constraint |
| **N6** | Manageable via Config Connector | Project constraint |
| **N7** | Cost-effective when duplicated per region | Project constraint |
| **N8** | Serverless preferred (not required) | Project constraint |
| **N9** | Watch event delivery within seconds of commit | Operational requirement |

---

## 4. Architecture Decision: rest.Storage vs storage.Interface

### Decision: Continue with rest.Storage + ResourceStore (Gecko's current approach)

Gecko's current architecture implements `rest.Storage` (and related `rest.*` interfaces) at the registry layer, with a simpler `ResourceStore` sitting below as the persistence abstraction. The adapter in `aggregated/storage.go` bridges these two layers.

**This is the correct approach.** Here is why:

| Factor | storage.Interface (low-level) | rest.Storage (registry-level) |
|--------|-------------------------------|-------------------------------|
| **Production non-etcd implementations** | Zero known | metrics-server (memory), Cozystack (PostgreSQL) |
| **API stability** | None — "No compatibility guarantees" per [pkg.go.dev](https://pkg.go.dev/k8s.io/apiserver) | More stable, simpler contract |
| **Semantic fit for SQL** | Poor — etcd MVCC assumptions (prefix keys, revision-based watches) | Natural — implement REST operations matching your storage model |
| **Maintenance burden** | Very high — must track k8s.io/apiserver changes | Medium — rest.Storage changes less |
| **Built-in features** | Free via genericregistry.Store (validation, defaulting, SSA) | Must implement in adapter layer |
| **Gecko's current state** | Would require rewrite | Already implemented and working |

**Industry evidence:**

- [k3s/kine](https://github.com/k3s-io/kine): Implements etcd gRPC protocol translation, not storage.Interface
- [metrics-server](https://github.com/kubernetes-sigs/metrics-server): Implements rest.Storage directly with in-memory storage
- [Cozystack](https://kubernetes.io/blog/2024/11/21/dynamic-kubernetes-api-server-for-cozystack/): PostgreSQL-backed rest.Storage
- [apiserver-builder-alpha](https://github.com/kubernetes-sigs/apiserver-builder-alpha): Uses rest.Storage pattern, supports non-etcd via `WithResourceAndHandler()`
- [sample-apiserver](https://github.com/kubernetes/sample-apiserver): Reference implementation uses genericregistry.Store + etcd, but documents all three patterns

**The three patterns for aggregated API server storage:**

```
Pattern A (default):  Types → Strategy → genericregistry.Store → RESTOptionsGetter → storage.Interface → etcd3
Pattern B (kine):     Types → Strategy → genericregistry.Store → etcd gRPC API → Kine → PostgreSQL/MySQL
Pattern C (gecko):    Types → Strategy → rest.Storage adapter → ResourceStore → PostgreSQL/Spanner/Memory
```

Gecko uses Pattern C — the adapter layer owns validation, defaulting, status subresources, and soft-delete semantics, while `ResourceStore` handles pure persistence. This gives full control without fighting etcd assumptions.

---

## 5. Prior Art

### 5.1 Kine (k3s)

[Kine](https://github.com/k3s-io/kine) ("Kine Is Not Etcd") translates the etcd v3 gRPC protocol to SQL databases. It enables Kubernetes to run without etcd by implementing a log-structured storage model.

**Schema (single table, PostgreSQL):**

```sql
CREATE TABLE IF NOT EXISTS kine (
    id               BIGSERIAL PRIMARY KEY,
    name             TEXT COLLATE "C",
    created          INTEGER,
    deleted          INTEGER,
    create_revision  BIGINT,
    prev_revision    BIGINT,
    lease            INTEGER,
    value            BYTEA,
    old_value        BYTEA
);

CREATE UNIQUE INDEX kine_name_prev_revision_uindex ON kine (name, prev_revision);
CREATE INDEX kine_name_index ON kine (name);
CREATE INDEX kine_name_id_index ON kine (name, id);
CREATE INDEX kine_id_deleted_index ON kine (id, deleted);
CREATE INDEX kine_prev_revision_index ON kine (prev_revision);
```

**Key design decisions:**

| Decision | Rationale |
|----------|-----------|
| **Log-structured (append-only)** | Matches etcd's MVCC semantics; every write appends a new row |
| **`BIGSERIAL` for global revision** | Auto-incrementing, sequential, gap-free (required) |
| **`(name, prev_revision)` unique constraint** | Optimistic concurrency control — prevents two concurrent updates with same `prev_revision` |
| **`COLLATE "C"` on name** | Ensures `LIKE` queries use index (byte-order comparison) |
| **Polling-based watch** | Cross-database portable (MySQL lacks LISTEN/NOTIFY) |
| **Periodic compaction** | Deletes superseded revisions and tombstones |

**Strengths:** Simple, proven at scale for k3s, supports PostgreSQL/MySQL/SQLite/NATS.

**Weaknesses:**
- Polling-based watch adds latency vs. native notifications
- Single-table design creates contention at high write rates
- Performance degrades after years of operation (table bloat, [k3s#9256](https://github.com/k3s-io/k3s/issues/9256))
- Sequential `BIGSERIAL` limits HA options (no multi-master with `auto_increment_increment > 1`)
- Higher RPO/RTO than etcd for failover scenarios

**Lesson for Gecko:** Kine's log-structured model is clever but adds complexity (compaction, tombstones, old_value storage). Since Gecko already owns its `rest.Storage` adapter, a simpler update-in-place model with a separate event log (like postgres-controller-backend) is more appropriate.

### 5.2 postgres-controller-backend

The [postgres-controller-backend](https://github.com/openshift-online/gecko) project implements a PostgreSQL-based replacement for etcd, designed for controller-runtime controllers. It provides the most mature reference for our requirements.

**Schema:**

```sql
CREATE TABLE IF NOT EXISTS kubernetes_resources (
    gvk                TEXT        NOT NULL,
    namespace          TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    uid                UUID        NOT NULL DEFAULT gen_random_uuid(),
    txid_stamp         xid8        NOT NULL,
    object_version     BIGINT      NOT NULL DEFAULT 1,
    spec               JSONB       NOT NULL,
    status             JSONB       NOT NULL,
    metadata           JSONB       NOT NULL,
    deletion_timestamp TIMESTAMPTZ NULL,
    created_at         TIMESTAMPTZ DEFAULT now(),
    updated_at         TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (gvk, namespace, name)
);

CREATE TABLE IF NOT EXISTS compaction_horizon (
    gvk           TEXT   NOT NULL PRIMARY KEY,
    compacted_xid BIGINT NOT NULL
);
```

**Key innovations:**

| Innovation | How It Works |
|-----------|--------------|
| **Dual versioning** | `txid_stamp` (PostgreSQL `pg_current_xact_id()`) for event stream ordering; `object_version` (per-object counter) for optimistic concurrency |
| **No shared counter** | Each writer gets its own `txid` without blocking — no write serialization |
| **Watermark-based watch** | `pg_snapshot_xmin()` guarantees all txids below watermark have committed — safe resume point |
| **Stored procedure** | `pgctl_write()` handles create/update/status-update/tombstone-revival in 181 lines of PL/pgSQL, eliminating 2 round-trips per write |
| **No-op suppression** | Content-equal writes consume no sequence number and emit no event |
| **Poll-primary with doorbell** | LISTEN/NOTIFY as latency optimization only; baseline timer ensures liveness even if doorbell lost |
| **Atomic compaction** | CTE atomically deletes tombstones and advances compaction horizon |

**Composite resourceVersion format:**

```
Collection-level: "12345678"          (watermark only)
Object-level:     "o5;12345678"       (ObjectVersion=5, Watermark=12345678)
```

The `o<version>;` prefix carries the per-object version so objects from the informer cache remain usable for optimistic-concurrency writes.

**Seven formal invariants (I1-I7):**

| ID | Invariant | Mechanism |
|----|-----------|-----------|
| I1 | Commit-ordered delivery | `pg_snapshot_xmin()` watermark |
| I2 | No watermark regression | RDS Multi-AZ synchronous replication |
| I3 | Exactly-once per state | Watermark + informer dedup |
| I4 | RV monotonicity | xmin never decreases |
| I5 | Compaction safety | Atomic CTE + 410 Gone |
| I6 | Optimistic concurrency | `object_version` WHERE clause |
| I7 | Partition completeness | Hash-based sharding |

**Performance:** 15,322 writes/sec at p99=4.3ms on RDS (db.m6g.8xlarge).

**Lesson for Gecko:** The dual-versioning approach (transaction ID for ordering + per-object version for OCC) is elegant and eliminates the single-counter bottleneck that plagues both kine and the Firestore/Bigtable PoCs. This pattern should be adapted for our chosen datastore.

### 5.3 sample-apiserver and apiserver-builder-alpha

**[sample-apiserver](https://github.com/kubernetes/sample-apiserver)** is the official reference implementation for aggregated API servers. It demonstrates the standard `genericregistry.Store` + etcd pattern:

```go
func NewREST(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (*registry.REST, error) {
    strategy := NewStrategy(scheme)
    store := &genericregistry.Store{
        NewFunc:        func() runtime.Object { return &wardle.Flunder{} },
        NewListFunc:    func() runtime.Object { return &wardle.FlunderList{} },
        CreateStrategy: strategy,
        UpdateStrategy: strategy,
        DeleteStrategy: strategy,
    }
    options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: GetAttrs}
    store.CompleteWithOptions(options)
    return &registry.REST{Store: store}, nil
}
```

The clean seam for swapping backends is at `RESTOptionsGetter`, but implementing `storage.Interface` for a non-etcd backend is complex and has no known production examples.

**[apiserver-builder-alpha](https://github.com/kubernetes-sigs/apiserver-builder-alpha)** is a code generation framework for aggregated API servers. Key findings:
- Uses `rest.Storage` pattern (not `storage.Interface` directly)
- Supports non-etcd via `WithResourceAndHandler()` + `WithoutEtcd()`
- **Stuck at Kubernetes 1.23** (3+ versions behind), limited maintenance
- Maintainers recommend [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) (CRDs) instead
- Multiple [open issues](https://github.com/kubernetes-sigs/apiserver-builder-alpha/issues/547) about etcd dependencies persisting even with custom storage

**Lesson for Gecko:** Gecko's existing approach (custom `ResourceStore` + `rest.Storage` adapter) aligns with how production non-etcd API servers work. The sample-apiserver pattern is useful as a reference but not as a template for our storage layer.

---

## 6. GCP Datastore Alternatives

### 6.1 Option A: Cloud Spanner (Recommended)

**What:** Fully managed, horizontally scalable relational database with strong consistency via TrueTime.

**Why it fits:**

- **Proven k8s storage replacement.** A production implementation exists that [replaced etcd with Cloud Spanner](https://github.com/n0rm4l-me/spanner-etcd) — 45 Watch streams migrated in ~10s during failover, zero-gap event delivery confirmed.
- **Change Streams** provide near real-time change data capture (~30ms same-region latency) — ideal for watch implementation.
- **Zero-downtime everything:** Upgrades, scaling, and schema changes require no maintenance windows.
- **Regional configuration** with synchronous replication across zones.
- **Strong consistency** (linearizability via TrueTime) across all operations.

**Config Connector support:**

```yaml
apiVersion: spanner.cnrm.cloud.google.com/v1beta1
kind: SpannerInstance
metadata:
  name: gecko-platform-api
spec:
  config: regional-us-central1
  displayName: Gecko Platform API Storage
  processingUnits: 100
---
apiVersion: spanner.cnrm.cloud.google.com/v1beta1
kind: SpannerDatabase
metadata:
  name: gecko-storage
spec:
  instanceRef:
    name: gecko-platform-api
```

Config Connector support is mature (`spanner.cnrm.cloud.google.com/v1beta1`) with full CRD coverage for instances, databases, and IAM policies.

**Cost estimate (per region):**

| Configuration | Monthly Cost | SLA |
|---------------|-------------|-----|
| 100 PU (dev/test only) | ~$66/month | **No SLA** |
| 1 node / 1000 PU | ~$657/month | **No SLA** |
| **2 nodes / 2000 PU (production minimum)** | **~$1,314/month** | **99.99%** |
| 2 nodes with 3-year CUD | ~$788/month | 99.99% |

Storage: $0.30/GiB/month additional. Backups: $0.30/GiB/month.

> **Important:** Spanner requires **≥2 nodes (2000 PU)** for SLA coverage. Instances below this threshold have no uptime guarantee. The 100 PU option is suitable for dev/test and PoC only.

Autoscaling available in Enterprise edition but does not change SLA minimum.

**Trade-offs:**

| Pro | Con |
|-----|-----|
| Proven k8s storage replacement exists | **~$1,314/month minimum for production** (with SLA) |
| Change Streams for watch (~30ms) | Significantly more expensive than Cloud SQL |
| Zero-downtime operations | Change Stream ordering requires application-level handling |
| Strong consistency (TrueTime) | Spanner SQL dialect differs from PostgreSQL |
| Horizontal auto-scaling | Learning curve for Spanner-specific patterns |
| 7-day PITR | |
| Excellent Config Connector support | |

### 6.2 Option B: Cloud SQL for PostgreSQL

**What:** Fully managed PostgreSQL with HA, automatic backups, and automated maintenance.

**Why it fits:**

- **Cheapest option** at ~$16-22/month for a small HA deployment.
- **PostgreSQL ecosystem** — well-understood, vast tooling, native LISTEN/NOTIFY.
- **Proven patterns** — both kine and postgres-controller-backend demonstrate PostgreSQL as a k8s storage backend.
- **Gecko already has a working PostgreSQL store** implementation with JSONB, GIN indexes, and LISTEN/NOTIFY broadcasting.

**Config Connector support:**

```yaml
apiVersion: sql.cnrm.cloud.google.com/v1beta1
kind: SQLInstance
metadata:
  name: gecko-platform-api
spec:
  region: us-central1
  databaseVersion: POSTGRES_16
  settings:
    tier: db-custom-2-4096
    availabilityType: REGIONAL
    backupConfiguration:
      enabled: true
      pointInTimeRecoveryEnabled: true
---
apiVersion: sql.cnrm.cloud.google.com/v1beta1
kind: SQLDatabase
metadata:
  name: gecko-storage
spec:
  instanceRef:
    name: gecko-platform-api
```

Config Connector support is the most mature of all options (`sql.cnrm.cloud.google.com/v1beta1`).

**Cost estimate (per region):**

| Configuration | Monthly Cost | SLA | Notes |
|---------------|-------------|-----|-------|
| db-f1-micro (shared CPU, HA) | ~$14-20/month | **No SLA** | Shared CPU, not eligible for CUD |
| db-custom-2-4096 (2 vCPU, HA) | ~$80-120/month | 99.95% | Minimum production with SLA |
| db-custom-4-8192 (4 vCPU, HA) | ~$160-200/month | 99.95% | Comfortable production |

Storage: $0.17/GB/month SSD additional. Enterprise Plus (~30% premium) adds up to 35-day PITR.

> **Note:** Shared-CPU instances (db-f1-micro, db-g1-small) have **no SLA coverage** and are not recommended for production. The minimum production configuration with SLA is a dedicated-CPU HA instance.

**Trade-offs:**

| Pro | Con |
|-----|-----|
| Cheapest option ($16-22/month) | 5-10 min downtime during automatic updates |
| PostgreSQL — familiar ecosystem | LISTEN/NOTIFY has [performance limits](https://www.recall.ai/blog/postgres-listen-notify-does-not-scale) (serialization bottleneck) |
| LISTEN/NOTIFY for low-latency watch | Not serverless |
| Gecko already has PostgreSQL store | Manual major version upgrades |
| Proven via kine + postgres-controller-backend | Not horizontally scalable |
| PITR up to 35 days (Enterprise Plus) | Shared-CPU instances lack SLA |
| Best Config Connector support | |

### 6.3 Option C: AlloyDB for PostgreSQL

**What:** Fully managed PostgreSQL-compatible database with Google's custom storage engine, optimized for demanding workloads.

**Why it fits:**

- **Best PITR** — microsecond granularity, up to 35 days retention, parallelized WAL replay.
- **Near-zero downtime maintenance** — <1 second for primary instances, 0 seconds for read pools.
- **PostgreSQL compatible** — can reuse Gecko's existing PostgreSQL store implementation.
- **Shared regional storage** — separates compute and storage layers for better resilience.

**Config Connector support:**

```yaml
apiVersion: alloydb.cnrm.cloud.google.com/v1beta1
kind: AlloyDBCluster
metadata:
  name: gecko-platform-api
spec:
  location: us-central1
  networkConfig:
    networkRef:
      name: gecko-vpc
---
apiVersion: alloydb.cnrm.cloud.google.com/v1beta1
kind: AlloyDBInstance
metadata:
  name: gecko-primary
spec:
  clusterRef:
    name: gecko-platform-api
  instanceType: PRIMARY
  machineConfig:
    cpuCount: 2
```

Config Connector support is comprehensive (`alloydb.cnrm.cloud.google.com/v1beta1`).

**Cost estimate (per region):**

| Configuration | Monthly Cost | SLA | Notes |
|---------------|-------------|-----|-------|
| 2 vCPU BASIC (no standby) | ~$100-150/month | **No SLA** | Dev/test only |
| 2 vCPU PRIMARY (auto standby) | ~$200-300/month | 99.99% | Minimum production |
| 4 vCPU PRIMARY | ~$350-500/month | 99.99% | Comfortable production |
| 2 vCPU PRIMARY + 1 read pool | ~$300-450/month | 99.99% | With read scaling |

Storage: $0.339/GB/month (~2x Cloud SQL). Backups: $0.113/GB/month.

> **AlloyDB HA model:** PRIMARY instances automatically include a standby replica in a different zone — HA is built-in, not opt-in. You pay for both primary and standby compute. BASIC instances have no standby, no automatic failover, and no production SLA.

**Trade-offs:**

| Pro | Con |
|-----|-----|
| Best PITR (microsecond, 35 days) | Expensive minimum ($200-400/month) |
| Near-zero downtime maintenance (<1s) | LISTEN/NOTIFY same limitations as Cloud SQL |
| PostgreSQL compatible (reuse Gecko code) | Not serverless |
| Separated compute/storage layers | Manual major version upgrades |
| Strong consistency | 2-3x cost of Cloud SQL for equivalent workload |
| Good Config Connector support | |

### 6.4 Option D: Firestore (Native Mode)

**What:** Serverless NoSQL document database with real-time listeners and strong consistency.

**Why consider it:**

- **Truly serverless.** Zero provisioning, zero ops, automatic scaling. The lowest operational burden of all options.
- **Real-time listeners.** Native watch mechanism — Firestore's core feature. No polling, no event log needed for live events.
- **Free tier.** Covers prototyping and very small deployments ($0/month).
- **Strong consistency.** Even with regional replication, reads are strongly consistent.
- **Config Connector:** Likely supported but not explicitly confirmed.

**Challenges:**

- **Document model mismatch.** Kubernetes resources are typed objects with well-defined schemas. Mapping to Firestore's document model requires a translation layer and loses SQL query capabilities (no joins, limited aggregation, no complex WHERE clauses).
- **Counter bottleneck.** The monotonic counter stored as a single document is limited by Firestore's ~1 sustained write/sec/document soft limit. As the [Firestore PoC (PR #17)](#102-firestore-poc-pr-17) acknowledges, practical throughput caps at ~10 writes/sec. The timestamp-as-RV alternative could eliminate this but relies on undocumented guarantees (see [PoC addendum](#102-firestore-poc-pr-17)).
- **Composite index limitations.** Firestore requires pre-defined composite indexes for multi-field queries. Adding new query patterns requires index deployment.
- **Client-side label/field filtering.** Cannot push label selector queries server-side efficiently — must read all documents and filter in application.
- **No transactional isolation for List.** Unlike SQL's `REPEATABLE READ`, Firestore queries provide no snapshot isolation across multiple documents.

**Cost estimate (per region):**

| Configuration | Monthly Cost | SLA | Notes |
|---------------|-------------|-----|-------|
| Free tier (50K reads/20K writes/day, 1GB) | $0/month | 99.99% (regional) | Sufficient for prototyping |
| Light production (~100 resources, 10 req/s) | ~$1-5/month | 99.99% | Pay-per-operation |
| Moderate production (~10K resources) | ~$10-50/month | 99.99% | Scales with usage |

> **Best for:** Prototyping, extremely cost-sensitive deployments, or use cases where the write throughput bottleneck (~10 writes/sec) is acceptable. The serverless model and real-time listeners are compelling if the document model impedance mismatch can be managed.

### 6.5 Option E: Cloud Bigtable

**What:** NoSQL wide-column database optimized for massive-scale, low-latency workloads.

**Why consider it:**

- **Massive scale ceiling.** Petabytes of storage, millions of ops/sec. If the platform API ever grows to extremely high read throughput, Bigtable handles it.
- **Atomic single-row operations.** `ReadModifyWriteRow` and `CheckAndMutateRow` (CondMutation) provide atomic primitives sufficient for create and OCC.
- **Change Streams.** Built-in CDC for watch implementation.
- **Low read latency.** Sub-10ms reads at scale.

**Challenges:**

- **No cross-row transactions.** Bigtable only guarantees strong consistency at the single-row level. List operations have no snapshot isolation — concurrent writes during a scan produce inconsistent results.
- **Counter hotspot.** The `ReadModifyWriteRow` atomic increment on a single counter row creates a write hotspot. Bigtable [explicitly warns](https://cloud.google.com/bigtable/docs/schema-design#types_of_row_keys) about single-row hot-write patterns.
- **Expensive minimum.** ~$470/month per node, minimum 1 node. For production HA, 3 nodes = ~$1,400/month — significantly over-provisioned for our workload.
- **All filtering is client-side.** Label selectors and field filters cannot be pushed to the server, requiring full scan and deserialization of every row.
- **Change Streams limitations.** No old-value capture, can emit duplicates, requires idempotent processing.
- **Workload mismatch.** Bigtable excels at time-series, analytics, and high-throughput scans — not at low-volume, transaction-heavy Kubernetes API patterns.

**Cost estimate (per region):**

| Configuration | Monthly Cost | SLA | Notes |
|---------------|-------------|-----|-------|
| 1 node (dev/test) | ~$470/month | **No SLA** (single node) | Over-provisioned for this workload |
| 3 nodes (production HA) | ~$1,400/month | 99.99% | Minimum for HA with replication |

> **Best for:** Use cases where read throughput at massive scale is the priority and the lack of cross-row transactions is acceptable. Not the natural fit for Kubernetes API storage patterns, but a viable option if the team is willing to manage the impedance mismatch and the cost is justified by other platform needs (e.g., shared Bigtable cluster for other workloads).

### 6.6 Rejected Alternative

#### Memorystore for Redis — Rejected

**Why rejected:** Ephemeral in-memory store with limited persistence. Cannot combine AOF + RDB snapshots, no PITR, version frozen at Redis 7.2, 3-5x cost of self-managed Redis. Fundamentally unsuitable for persistent Kubernetes resource storage.

---

## 7. Comparison Matrix

| Requirement | Cloud Spanner | Cloud SQL PostgreSQL | AlloyDB | Firestore | Bigtable |
|-------------|:---:|:---:|:---:|:---:|:---:|
| **F1** Atomic create | Yes (transactions) | Yes (INSERT + unique constraint) | Yes | Yes (docRef.Create) | Yes (CondMutation) |
| **F2** Optimistic concurrency | Yes (read-write txn) | Yes (UPDATE WHERE version=) | Yes | Possible (transactions) | Partial (CondMutation) |
| **F3** Monotonic RV | Yes (commit timestamps) | Yes (pg_current_xact_id / SEQUENCE) | Yes | Counter doc bottleneck | Counter row hotspot |
| **F4** Ordered watch | Yes (Change Streams) | Yes (LISTEN/NOTIFY + event log) | Yes | Snapshot listeners (unordered) | Change Streams (may duplicate) |
| **F5** Watch replay | Yes (Change Stream cursor) | Yes (event log query) | Yes | Query event log | Query event log |
| **F6** Pagination | Yes (keyset) | Yes (LIMIT/OFFSET, keyset) | Yes | Yes (StartAfter) | Yes (row key range) |
| **F7** Label filtering | Yes (SQL WHERE on JSONB/ARRAY) | Yes (JSONB + GIN index) | Yes | Client-side only | Client-side only |
| **F8** Namespace scoping | Yes (SQL WHERE) | Yes (SQL WHERE) | Yes | Yes (document field) | Yes (row key prefix) |
| **N1** Strong consistency | Linearizable (TrueTime) | ACID (single region) | ACID | Strong reads | Single-row only |
| **N2** Multi-replica safe | Yes | Yes | Yes | Yes (with transactions) | Partial |
| **N3** Regional | Yes | Yes | Yes | Yes | Yes |
| **N4** Zero-touch ops | Zero downtime | 5-10 min downtime | <1s downtime | Zero downtime | Managed |
| **N5** PITR | 7 days | 7-35 days | 35 days | 7 days | No |
| **N6** Config Connector | Excellent | Excellent | Good | Unconfirmed | Likely |
| **N7** Cost (production w/ SLA) | ~$1,314/mo (2 nodes) | ~$80-120/mo (2 vCPU HA) | ~$200-300/mo (2 vCPU HA) | ~$0-1/mo (serverless) | ~$470+/mo (1 node, no SLA) |
| **N8** Serverless | No (100 PU min) | No | No | Yes | No |

---

## 8. Database Schema Proposals

### 8.1 SQL Schema (Cloud SQL / AlloyDB)

This schema is based on patterns from both postgres-controller-backend and Gecko's existing PostgreSQL store, adapted for our requirements.

```sql
-- ============================================================
-- Resource table: one table per resource type
-- ============================================================
CREATE TABLE IF NOT EXISTS resources_{type} (
    -- Identity
    namespace          TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    uid                UUID        NOT NULL DEFAULT gen_random_uuid(),

    -- Versioning
    resource_version   BIGINT      NOT NULL,
    object_version     BIGINT      NOT NULL DEFAULT 1,

    -- Content (split for status subresource support)
    data               JSONB       NOT NULL,
    labels             JSONB       NOT NULL DEFAULT '{}'::jsonb,

    -- Lifecycle
    deletion_timestamp TIMESTAMPTZ NULL,

    -- Multi-tenancy / context filtering
    context_filter     TEXT        NOT NULL DEFAULT '',

    -- Audit
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Constraints
    PRIMARY KEY (namespace, name, context_filter)
);

-- Label filtering via GIN index
CREATE INDEX IF NOT EXISTS idx_{type}_labels
    ON resources_{type} USING GIN (labels);

-- Watch: scan by resource_version
CREATE INDEX IF NOT EXISTS idx_{type}_rv
    ON resources_{type} (resource_version);

-- List: live objects only
CREATE INDEX IF NOT EXISTS idx_{type}_list
    ON resources_{type} (namespace)
    WHERE deletion_timestamp IS NULL;

-- ============================================================
-- Event log table: append-only for watch replay
-- ============================================================
CREATE TABLE IF NOT EXISTS event_log_{type} (
    id                 BIGSERIAL   PRIMARY KEY,
    resource_version   BIGINT      NOT NULL,
    event_type         TEXT        NOT NULL,  -- ADDED, MODIFIED, DELETED
    namespace          TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    data               JSONB       NOT NULL,
    context_filter     TEXT        NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_{type}_eventlog_rv
    ON event_log_{type} (resource_version);

-- ============================================================
-- Compaction horizon: tracks what's been cleaned up
-- ============================================================
CREATE TABLE IF NOT EXISTS compaction_horizon (
    resource_type      TEXT        NOT NULL PRIMARY KEY,
    compacted_rv       BIGINT      NOT NULL DEFAULT 0
);
```

**Stored procedure for writes (PostgreSQL):**

```sql
CREATE OR REPLACE FUNCTION resource_write(
    p_resource_type    TEXT,
    p_namespace        TEXT,
    p_name             TEXT,
    p_expected_version BIGINT,     -- 0 = create, >0 = update
    p_data             JSONB,
    p_labels           JSONB,
    p_context_filter   TEXT DEFAULT '',
    p_deletion_ts      TIMESTAMPTZ DEFAULT NULL
) RETURNS TABLE(
    out_uid            UUID,
    out_version        BIGINT,
    out_rv             BIGINT,
    out_changed        BOOLEAN
) LANGUAGE plpgsql AS $$
DECLARE
    v_txid    BIGINT := pg_current_xact_id()::text::bigint;
    v_uid     UUID;
    v_version BIGINT;
BEGIN
    IF p_expected_version = 0 THEN
        -- CREATE
        INSERT INTO resources (namespace, name, uid, resource_version,
                              object_version, data, labels, context_filter,
                              deletion_timestamp)
        VALUES (p_namespace, p_name, gen_random_uuid(), v_txid,
                1, p_data, p_labels, p_context_filter, p_deletion_ts)
        RETURNING uid, object_version INTO v_uid, v_version;

        RETURN QUERY SELECT v_uid, v_version, v_txid, true;
    ELSE
        -- UPDATE with optimistic concurrency
        UPDATE resources
           SET resource_version = v_txid,
               object_version = object_version + 1,
               data = p_data,
               labels = p_labels,
               deletion_timestamp = p_deletion_ts,
               updated_at = now()
         WHERE namespace = p_namespace
           AND name = p_name
           AND context_filter = p_context_filter
           AND object_version = p_expected_version
        RETURNING uid, object_version INTO v_uid, v_version;

        IF NOT FOUND THEN
            RAISE EXCEPTION 'conflict' USING ERRCODE = 'P0002';
        END IF;

        RETURN QUERY SELECT v_uid, v_version, v_txid, true;
    END IF;
END;
$$;
```

**Resource version strategy:** Uses `pg_current_xact_id()` (PostgreSQL's native 64-bit transaction ID) as the resource version. This provides:
- Global monotonicity without a shared sequence
- No write serialization (each transaction gets its own ID without blocking)
- Safe watermark computation via `pg_snapshot_xmin()` for watch

**Watch mechanism:** LISTEN/NOTIFY with event log table for historical replay. The doorbell pattern from postgres-controller-backend (debounced `pg_notify`) provides sub-100ms notification latency while the event log ensures completeness.

### 8.2 Spanner Schema

```sql
-- ============================================================
-- Resource table
-- ============================================================
CREATE TABLE Resources (
    ResourceType       STRING(256)   NOT NULL,
    Namespace          STRING(256)   NOT NULL,
    Name               STRING(256)   NOT NULL,
    UID                STRING(36)    NOT NULL,
    ResourceVersion    INT64         NOT NULL,
    ObjectVersion      INT64         NOT NULL,
    Data               JSON          NOT NULL,
    Labels             JSON,
    DeletionTimestamp   TIMESTAMP,
    ContextFilter      STRING(256)   NOT NULL,
    CreatedAt          TIMESTAMP     NOT NULL OPTIONS (allow_commit_timestamp = true),
    UpdatedAt          TIMESTAMP     NOT NULL OPTIONS (allow_commit_timestamp = true),
) PRIMARY KEY (ResourceType, Namespace, Name, ContextFilter);

-- Secondary index for watch queries
CREATE INDEX ResourcesByRV
    ON Resources (ResourceType, ResourceVersion);

-- Secondary index for listing live objects
CREATE NULL_FILTERED INDEX ResourcesByNamespace
    ON Resources (ResourceType, Namespace)
    WHERE DeletionTimestamp IS NULL;

-- ============================================================
-- Event log for watch replay
-- ============================================================
CREATE TABLE EventLog (
    ResourceType       STRING(256)   NOT NULL,
    ResourceVersion    INT64         NOT NULL,
    EventType          STRING(16)    NOT NULL,
    Namespace          STRING(256)   NOT NULL,
    Name               STRING(256)   NOT NULL,
    Data               JSON          NOT NULL,
    ContextFilter      STRING(256)   NOT NULL,
    CreatedAt          TIMESTAMP     NOT NULL OPTIONS (allow_commit_timestamp = true),
) PRIMARY KEY (ResourceType, ResourceVersion),
  ROW DELETION POLICY (OLDER_THAN(CreatedAt, INTERVAL 7 DAY));

-- ============================================================
-- Counter table for monotonic resource versions
-- ============================================================
CREATE TABLE Counters (
    CounterID          STRING(256)   NOT NULL,
    Value              INT64         NOT NULL,
) PRIMARY KEY (CounterID);

-- ============================================================
-- Compaction horizon
-- ============================================================
CREATE TABLE CompactionHorizon (
    ResourceType       STRING(256)   NOT NULL,
    CompactedRV        INT64         NOT NULL,
) PRIMARY KEY (ResourceType);
```

**Resource version strategy for Spanner:** Two options:

**Option 1 — Counter-based (simpler, recommended for low write volumes):**
```go
func (s *SpannerStore) nextResourceVersion(ctx context.Context, tx *spanner.ReadWriteTransaction) (int64, error) {
    row, err := tx.ReadRow(ctx, "Counters",
        spanner.Key{"rv_" + s.resourceType}, []string{"Value"})
    var current int64
    row.Column(0, &current)
    next := current + 1
    tx.BufferWrite([]*spanner.Mutation{
        spanner.Update("Counters", []string{"CounterID", "Value"},
            []interface{}{"rv_" + s.resourceType, next}),
    })
    return next, nil
}
```

Within a Spanner read-write transaction, the counter read acquires a lock, ensuring sequential increment. The counter update and the resource write are committed atomically.

**Option 2 — Commit timestamp-based (no counter bottleneck, higher complexity):**
```go
// Use Spanner's commit timestamp as resource version
// CommitTimestamp is monotonically increasing and unique per transaction
mutation := spanner.InsertOrUpdate("Resources",
    []string{"ResourceType", "Namespace", "Name", "UpdatedAt", ...},
    []interface{}{resourceType, ns, name, spanner.CommitTimestamp, ...})
```

This eliminates the counter document entirely but requires converting timestamps to int64 resource versions (microseconds since epoch). The trade-off is that RVs are no longer sequential integers but are still monotonically increasing.

**Watch mechanism for Spanner:** [Change Streams](https://cloud.google.com/spanner/docs/change-streams) provide real-time CDC:

```sql
CREATE CHANGE STREAM ResourceChanges
    FOR Resources
    OPTIONS (retention_period = '7d', value_capture_type = 'NEW_VALUES');
```

Each API server replica reads the change stream independently. Cursor position persisted to Spanner for resume across failover. Measured latency: ~30ms same-region.

---

## 9. Key Implementation Patterns

### 9.1 Monotonic Resource Versioning

| Datastore | Strategy | Pros | Cons |
|-----------|----------|------|------|
| **PostgreSQL** | `pg_current_xact_id()` | No shared counter, no write serialization, globally monotonic | PostgreSQL-specific; xid wraparound (extremely rare with 64-bit xid8) |
| **Spanner** (counter) | Read-increment-write in transaction | Simple, sequential integers | Counter row contention under concurrent writes |
| **Spanner** (commit TS) | `PENDING_COMMIT_TIMESTAMP()` | No counter bottleneck, Spanner guarantees monotonicity | Not sequential integers; requires timestamp→int64 mapping |
| **Kine** | `BIGSERIAL` | Simple, sequential | Requires gap-free sequence; limits HA options |

**Recommendation:** For PostgreSQL, use `pg_current_xact_id()` (proven by postgres-controller-backend). For Spanner, start with counter-based (simpler) and migrate to commit-timestamp if counter becomes a bottleneck.

### 9.2 Watch / Change Notification

| Datastore | Primary Mechanism | Latency | Historical Replay |
|-----------|-------------------|---------|-------------------|
| **PostgreSQL** | LISTEN/NOTIFY + event log table | Sub-100ms (with doorbell debouncing) | Query event log WHERE rv > $since |
| **Spanner** | Change Streams API | ~30ms (measured) | Change Stream with start timestamp |

**PostgreSQL watch pattern (doorbell + poll):**

```
Writer goroutine:
  1. Write resource + event log row in transaction
  2. pg_notify('resource_type_channel', event_log_id)

Watcher goroutine:
  Loop:
    1. Wait for: doorbell notification OR 5-second timer
    2. BEGIN REPEATABLE READ, READ ONLY
    3. SELECT pg_snapshot_xmin(pg_current_snapshot()) → watermark
    4. SELECT * FROM event_log WHERE rv > $hwm ORDER BY rv
    5. COMMIT
    6. Fan out events to subscribers
    7. Advance hwm to watermark - 1
```

**Spanner watch pattern (Change Streams):**

```
Watcher goroutine:
  1. Open Change Stream reader from last persisted cursor
  2. For each change record:
     a. Parse data change
     b. Reconstruct resource object
     c. Fan out to subscribers
  3. Periodically persist cursor position
```

### 9.3 Optimistic Concurrency Control

All three options support OCC via different mechanisms:

**PostgreSQL / AlloyDB:**
```sql
UPDATE resources SET ... WHERE name = $1 AND object_version = $2
-- Returns 0 rows affected → Conflict (409)
```

**Spanner:**
```go
// Within read-write transaction:
row, _ := tx.ReadRow(ctx, "Resources", key, []string{"ObjectVersion"})
var stored int64
row.Column(0, &stored)
if stored != expected {
    return status.Errorf(codes.FailedPrecondition, "conflict")
}
// Proceed with buffered write
```

### 9.4 Compaction and Garbage Collection

**Tombstone compaction (PostgreSQL):**

```sql
WITH deleted AS (
    DELETE FROM resources_{type}
    WHERE deletion_timestamp IS NOT NULL
      AND GREATEST(deletion_timestamp, updated_at) < now() - $retention::interval
      AND (data->'metadata'->'finalizers' IS NULL
           OR data->'metadata'->'finalizers' = '[]'::jsonb)
    RETURNING resource_version
),
horizon AS (
    INSERT INTO compaction_horizon (resource_type, compacted_rv)
    VALUES ($type, (SELECT max(resource_version) FROM deleted))
    ON CONFLICT (resource_type)
    DO UPDATE SET compacted_rv = GREATEST(
        compaction_horizon.compacted_rv,
        EXCLUDED.compacted_rv
    )
)
SELECT count(*) FROM deleted;
```

**Event log compaction (PostgreSQL):**
```sql
DELETE FROM event_log_{type}
WHERE created_at < now() - $retention::interval;
```

**Event log compaction (Spanner):** Automatic via `ROW DELETION POLICY` — zero application code needed.

**410 Gone enforcement:** When a watcher's resume resource version falls below the compaction horizon, return `410 Gone` to force a relist.

### 9.5 Multi-Replica Consistency

With multiple API server replicas writing concurrently:

| Concern | PostgreSQL Solution | Spanner Solution |
|---------|-------------------|-----------------|
| **Concurrent creates** | UNIQUE constraint → `AlreadyExists` | Primary key constraint → `AlreadyExists` |
| **Concurrent updates** | `object_version` WHERE clause → Conflict | Read-write transaction + version check → Conflict |
| **Event ordering** | `pg_current_xact_id()` globally monotonic per PostgreSQL instance | Commit timestamps monotonic per Spanner |
| **Watch consistency** | `pg_snapshot_xmin()` ensures no in-flight txns below watermark | Change Stream guarantees linearizable delivery |
| **List consistency** | `REPEATABLE READ` snapshot isolation | Spanner strong reads (default) |

---

## 10. PoC Addendums

### 10.1 Bigtable PoC (PR #16)

**PR:** [openshift-online/gecko#16](https://github.com/openshift-online/gecko/pull/16) — "feat: add Bigtable storage backend with integration tests"
**Files:** +2139 / -153 across 11 files

**Data Model:**

| Table | Row Key | Column Family | Columns |
|-------|---------|---------------|---------|
| `resources_{type}` | `[contextFilter\x00]namespace\x00name` | `d` (data) | `json`, `rv`, `labels` |
| `counters` | `rv_{type}` | `c` (counter) | `v` (big-endian uint64) |
| `eventlog_{type}` | Zero-padded 20-digit RV | `e` (event) | `type`, `rv`, `data`, `cf`, `ts` |

**Monotonic counter:** Uses `ReadModifyWriteRow` atomic increment on a single counter row per resource type. This is the correct Bigtable pattern but creates a hotspot under concurrent writes.

**Watch:** Uses Bigtable Change Streams on the event log table. Change stream reconnection always starts from `timestamppb.Now()`, meaning events between disconnect and reconnect are lost.

**Critical gaps:**

| Gap | Impact |
|-----|--------|
| **No optimistic concurrency on Update** | Two concurrent updates both succeed; last-write-wins. Breaks Kubernetes API contract. |
| **TOCTOU race in Update/Delete** | `Get()` then `Apply()` is not atomic. Concurrent operations between these calls cause silent data corruption. |
| **Watch events silently dropped** | Non-blocking channel send drops events when subscriber buffer is full. Controllers miss state changes permanently. |
| **All filtering client-side** | Label and field filtering deserializes every row. 100K resources → 100K deserializations per list. |
| **No snapshot isolation for List** | Prefix scan has no transaction — concurrent writes produce inconsistent results. |

**Cost:** ~$470/month minimum (1 node) — dramatically over-provisioned for this workload.

**Verdict:** The impedance mismatch between Bigtable's NoSQL model and Kubernetes API semantics makes this the least natural fit. The PoC is a useful learning exercise but should not be pursued for production.

### 10.2 Firestore PoC (PR #17)

**PR:** [openshift-online/gecko#17](https://github.com/openshift-online/gecko/pull/17) — "feat: Add Firestore storage backend"
**Files:** +1866 / -113 across 12 files

**Data Model:**

| Collection | Document ID | Fields |
|------------|------------|--------|
| `resources_{type}` | `{contextFilter}_{namespace}_{name}` | `data`, `rv`, `labels`, `namespace`, `name`, `contextFilter`, `createdAt` |
| `eventlog_{type}` | Zero-padded 20-digit RV | `type`, `rv`, `data`, `contextFilter`, `timestamp` |
| `counters` | `rv_{type}` | `value` (int64) |

**Monotonic counter:** Read-increment-write in a Firestore transaction on a single counter document. Uses a classic integer counter (not timestamps). Firestore's ~1 write/sec/document soft limit makes this a hard bottleneck; the README honestly acknowledges practical throughput caps at ~10 writes/sec.

**Alternative: timestamp-as-resource-version.** Using `firestore.ServerTimestamp` on each document write would eliminate the counter bottleneck entirely — each write gets its own server-assigned timestamp independently. However, this approach has risks specific to Firestore:

| Risk | Detail |
|------|--------|
| **No documented uniqueness** | Two concurrent writes could receive the same microsecond timestamp, producing duplicate resource versions |
| **No documented monotonicity** | Firestore's API does not explicitly guarantee that server timestamps from sequential transactions are strictly ordered |
| **Watch ordering gaps** | `WHERE timestamp > $lastSeen` could miss or double-deliver events sharing the same timestamp |
| **List "current version" ambiguity** | No clear way to determine the current resource version without a counter; in-flight writes may have timestamps between query time and commit |

Firestore is built on Spanner internally, so these guarantees *likely* hold — but relying on undocumented behavior in production is risky. By contrast, Spanner's `PENDING_COMMIT_TIMESTAMP()` explicitly guarantees monotonic, unique timestamps per transaction via TrueTime (this is exactly how [spanner-etcd](https://github.com/n0rm4l-me/spanner-etcd) achieves 15x throughput over etcd without a global write lock). If the timestamp-as-RV pattern is desired, Spanner is the safe choice.

**Watch:** Uses Firestore snapshot listeners on the event log collection — a natural fit for Firestore's real-time capabilities.

**Critical gaps:**

| Gap | Impact |
|-----|--------|
| **No optimistic concurrency on Update** | Same as Bigtable PoC — last-write-wins semantics, violates Kubernetes API conventions. |
| **TOCTOU race in Update/Delete** | `Get()` then `Set()` not atomic. Should use Firestore transactions. |
| **Broadcast silently swallows errors** | Event log write failure is ignored; watchers miss events with no error surfaced. |
| **Document ID collision risk** | `_` separator means `ns_a` + name `b` collides with `ns` + name `a_b`. |

**Cost:** $0-1/month (free tier covers small deployments) — most cost-effective option by far.

**Verdict:** Firestore's document model and real-time listeners are appealing, but the 1 write/sec/document counter bottleneck is a fundamental limitation. The PoC is missing OCC on updates. Could work for very low-write-volume use cases but the counter bottleneck needs to be addressed (see timestamp-as-RV analysis above).

---

## 11. Recommendation

### Primary: Cloud Spanner (regional, 2 nodes)

Cloud Spanner is the recommended datastore for the following reasons:

1. **Proven.** [spanner-etcd](https://github.com/n0rm4l-me/spanner-etcd) is an open-source etcd v3 replacement using Spanner — 15x write throughput, ~30ms watch latency, 24-hour soak test with zero data loss. Google internally [uses Spanner for GKE at 65,000+ nodes](https://cloud.google.com/blog/products/containers-kubernetes/gke-65k-nodes-and-counting).
2. **Zero-touch.** No maintenance windows, zero-downtime upgrades, automatic zone-level failover.
3. **Strong consistency.** Linearizable reads and writes via TrueTime — matches etcd's consistency model.
4. **Change Streams.** ~30ms latency for watch, with built-in cursor management and 7-day retention.
5. **Config Connector.** Mature `spanner.cnrm.cloud.google.com/v1beta1` CRDs.
6. **Regional.** Single-region configuration with synchronous cross-zone replication.
7. **Scalability.** Horizontal auto-scaling if workload grows, without schema or application changes.

**Cost consideration:** Production Spanner with SLA requires ≥2 nodes = **~$1,314/month per region** (reducible to ~$788/month with 3-year CUD). This is significantly more than Cloud SQL but buys zero-downtime operations, horizontal scaling, and the strongest consistency guarantees. For dev/test, 100 PU at ~$66/month is viable (no SLA).

### Fallback: Cloud SQL for PostgreSQL

If Spanner cost is prohibitive across many regions, Cloud SQL for PostgreSQL at **~$80-120/month per region** (2 vCPU HA with SLA) is a cost-effective alternative. The trade-offs (5-10 min maintenance downtime, LISTEN/NOTIFY scalability limits, no horizontal scaling) are acceptable for a low-volume platform API.

Gecko already has a working PostgreSQL store implementation. The postgres-controller-backend patterns (dual versioning with `pg_current_xact_id()`, doorbell-based watch, atomic compaction) provide a proven template for hardening it.

### Cost Summary (Production with SLA, per region)

| Option | Monthly (on-demand) | Monthly (3yr CUD) | SLA |
|--------|--------------------|--------------------|-----|
| **Cloud Spanner** (2 nodes) | ~$1,314 | ~$788 | 99.99% |
| **AlloyDB** (2 vCPU HA) | ~$200-300 | ~$144-216 (52% off) | 99.99% |
| **Cloud SQL** (2 vCPU HA) | ~$80-120 | N/A (shared CPU only) | 99.95% |

### Pluggable ResourceStore: Dev/Stage/Prod Strategy

Gecko's `ResourceStore` interface already provides the abstraction needed to run different backends per environment. This avoids coupling to a single datastore while optimizing cost and operability per tier:

| Environment | Backend | Estimated Cost | Rationale |
|-------------|---------|---------------|-----------|
| **Local dev** | PostgreSQL (Docker) | $0 | Fast iteration, no cloud dependency |
| **Integration / CI** | Cloud SQL PostgreSQL (shared CPU) | ~$14/month | Cheapest cloud option, disposable |
| **Stage** | Cloud Spanner (2 nodes) or Cloud SQL (HA) | $80-1,314/month | Match prod topology or use cheaper HA |
| **Production** | Cloud Spanner (2 nodes) | ~$1,314/month | Zero-downtime, strongest consistency |

Each backend implements `ResourceStore` with its own storage-native patterns:

- **PostgreSQL:** `pg_current_xact_id()` for RV, LISTEN/NOTIFY + event log for watch, stored procedure for writes, GIN index for labels
- **Spanner:** `PENDING_COMMIT_TIMESTAMP()` for RV, Change Streams for watch, read-write transactions for OCC, Search Indexes or generated columns for labels

### Spanner PostgreSQL Interface: Compatibility Analysis

Spanner offers a [PostgreSQL-compatible interface](https://cloud.google.com/spanner/docs/postgresql-interface) via PGAdapter (wire-protocol proxy). Standard PostgreSQL drivers (pgx, lib/pq) connect through it. However, this does **not** enable a single SQL codebase across Cloud SQL and Spanner.

**What works well through Spanner's PG interface:**

| Feature | Status | Notes |
|---------|--------|-------|
| Basic CRUD (SELECT, INSERT, UPDATE, DELETE) | Works | Standard PostgreSQL syntax |
| JSONB data type | Works | Storage and querying supported |
| REPEATABLE READ isolation | Works | Snapshot isolation semantics |
| ON CONFLICT (upsert) | Works | No WHERE clause support on the DO UPDATE |
| Non-recursive CTEs | Works | WITH ... AS queries |
| pgx driver (Go) | Works | v4.15+ via PGAdapter, ~0.2ms overhead |
| Foreign keys, check constraints | Works | Always enforced |
| `SPANNER.PENDING_COMMIT_TIMESTAMP()` | Works | Monotonic, unique per transaction (TrueTime) |
| Change Streams | Works | Near real-time CDC with 7-day retention |

**What does NOT work through Spanner's PG interface:**

| Feature | Impact on Gecko | Alternative |
|---------|----------------|-------------|
| `pg_current_xact_id()` | Cannot use for resource versioning | Use `PENDING_COMMIT_TIMESTAMP()` or counter |
| LISTEN/NOTIFY | Cannot use for watch notifications | Use Change Streams |
| Stored procedures / PL/pgSQL | Cannot use `pgctl_write()` pattern | Move write logic to application layer |
| GIN indexes on JSONB | Cannot index labels for server-side filtering | Use generated columns + secondary index, or Search Indexes (read-only txn only) |
| Monotonic sequences (SERIAL) | Sequences are bit-reversed (non-monotonic) | Use `PENDING_COMMIT_TIMESTAMP()` or app-level counter |
| Recursive CTEs | Minor — unlikely needed | Rewrite as iterative queries |
| Triggers | Minor — not used in Gecko | N/A |

**Conclusion:** Writing a Spanner backend using the PG interface is possible for basic CRUD but loses access to the key PostgreSQL-specific features Gecko relies on (txid, LISTEN/NOTIFY, stored procedures, GIN indexes). The recommended approach is to write the Spanner backend using **native GoogleSQL**, which provides the best access to Spanner-specific features (Change Streams, commit timestamps, interleaved tables). The `ResourceStore` interface isolates this from the rest of the codebase.

### Implementation Priority

1. **Phase 1:** Harden PostgreSQL store with postgres-controller-backend patterns (stored procedure, OCC, compaction). This gives immediate value for all environments.
2. **Phase 2:** Implement Spanner `ResourceStore` backend using native GoogleSQL with commit-timestamp RV and Change Streams for watch.
3. **Phase 3:** Performance testing, migration tooling, and environment-tier configuration.

---

## 12. References

### Kubernetes API Server & Storage

- [Kubernetes storage.Interface](https://pkg.go.dev/k8s.io/apiserver/pkg/storage) — Go package documentation
- [Generic API Server Framework — DeepWiki](https://deepwiki.com/kubernetes/apiserver/2.1-generic-api-server-framework)
- [Storage Interface and etcd3 Backend — DeepWiki](https://deepwiki.com/kubernetes/apiserver/3.3-storage-interface-and-etcd3-backend)
- [Storage System Architecture — DeepWiki](https://deepwiki.com/kubernetes/apiserver/3-storage-system)
- [K8s ASA: The Storage Interface — Daniel Mangum](https://danielmangum.com/posts/k8s-asa-the-storage-interface/)
- [Consistent ResourceVersion Semantics KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/2523-consistent-resource-versions-semantics/README.md)
- [Watch-List KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/3157-watch-list/README.md)
- [Watch Bookmark KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/956-watch-bookmark/README.md)
- [Kubernetes API Concepts](https://kubernetes.io/docs/reference/using-api/api-concepts/)

### Reference Implementations

- [kubernetes/sample-apiserver](https://github.com/kubernetes/sample-apiserver) — Official reference aggregated API server
- [kubernetes-sigs/apiserver-builder-alpha](https://github.com/kubernetes-sigs/apiserver-builder-alpha) — Code generation framework for aggregated API servers
- [k3s-io/kine](https://github.com/k3s-io/kine) — etcd shim for SQL backends (PostgreSQL, MySQL, SQLite, NATS)
- [Goodbye etcd, Hello PostgreSQL — Martin Heinz](https://martinheinz.dev/blog/100)
- [Exploring Kine — 01Cloud](https://engineering.01cloud.com/2026/02/03/exploring-kine-the-etcd-shim-revolutionizing-kubernetes-storage/)
- [k3s Cluster Datastore Documentation](https://docs.k3s.io/datastore)
- [How we built a dynamic Kubernetes API Server for Cozystack](https://kubernetes.io/blog/2024/11/21/dynamic-kubernetes-api-server-for-cozystack/)
- [How to Create Kubernetes Aggregated API Servers](https://oneuptime.com/blog/post/2026-01-30-kubernetes-aggregated-api-servers/view)

### apiserver-builder-alpha Issues

- [Issue #98: Generate and register new storage backend](https://github.com/kubernetes-sigs/apiserver-builder-alpha/issues/98)
- [Issue #547: How to remove etcd dependency](https://github.com/kubernetes-sigs/apiserver-builder-alpha/issues/547)
- [Issue #583: Why is etcd needed when using different storage](https://github.com/kubernetes-sigs/apiserver-builder-alpha/issues/583)

### GCP Database Services

- [Cloud Spanner Pricing](https://cloud.google.com/spanner/pricing)
- [Spanner Change Streams Overview](https://cloud.google.com/spanner/docs/change-streams)
- [Spanner TrueTime and External Consistency](https://cloud.google.com/spanner/docs/true-time-external-consistency)
- [Spanner Point-in-Time Recovery](https://cloud.google.com/spanner/docs/pitr)
- [Spanner Managed Autoscaler](https://cloud.google.com/spanner/docs/managed-autoscaler)
- [spanner-etcd: Drop-in etcd replacement using Cloud Spanner](https://github.com/n0rm4l-me/spanner-etcd)
- [GKE supports 65,000-node clusters (Spanner-based storage)](https://cloud.google.com/blog/products/containers-kubernetes/gke-65k-nodes-and-counting)
- [How we built a 130,000-node GKE cluster](https://cloud.google.com/blog/products/containers-kubernetes/how-we-built-a-130000-node-gke-cluster/)
- [Cloud Spanner SLA](https://cloud.google.com/spanner/sla)
- [AlloyDB Pricing](https://cloud.google.com/alloydb/pricing)
- [AlloyDB Backup and Recovery Overview](https://cloud.google.com/alloydb/docs/backup/overview)
- [AlloyDB Maintenance Overview](https://cloud.google.com/alloydb/docs/maintenance)
- [Cloud SQL Pricing](https://cloud.google.com/sql/pricing)
- [Cloud SQL PostgreSQL PITR](https://cloud.google.com/sql/docs/postgres/backup-recovery/pitr)
- [Cloud SQL Maintenance Updates](https://cloud.google.com/sql/docs/postgres/maintenance)
- [Firestore Pricing](https://cloud.google.com/firestore/pricing)
- [Firestore Real-time Queries at Scale](https://firebase.google.com/docs/firestore/real-time_queries_at_scale)
- [Bigtable Pricing](https://cloud.google.com/bigtable/pricing)
- [Bigtable Change Streams Overview](https://cloud.google.com/bigtable/docs/change-streams-overview)

### Config Connector

- [Config Connector Resource Reference](https://docs.cloud.google.com/config-connector/docs/reference/overview)
- [SpannerInstance CRD](https://docs.cloud.google.com/config-connector/docs/reference/resource-docs/spanner/spannerinstance)
- [SpannerDatabase CRD](https://docs.cloud.google.com/config-connector/docs/reference/resource-docs/spanner/spannerdatabase)
- [AlloyDBCluster CRD](https://docs.cloud.google.com/config-connector/docs/reference/resource-docs/alloydb/alloydbcluster)
- [AlloyDBInstance CRD](https://docs.cloud.google.com/config-connector/docs/reference/resource-docs/alloydb/alloydbinstance)
- [SQLInstance CRD](https://docs.cloud.google.com/config-connector/docs/reference/resource-docs/sql/sqlinstance)

### Spanner PostgreSQL Interface

- [PostgreSQL Interface Overview](https://cloud.google.com/spanner/docs/postgresql-interface)
- [PGAdapter Overview](https://cloud.google.com/spanner/docs/pgadapter)
- [Connect pgx to PostgreSQL-dialect Database](https://docs.cloud.google.com/spanner/docs/pg-pgx-connect)
- [Supported PostgreSQL Functions](https://docs.cloud.google.com/spanner/docs/reference/postgresql/functions)
- [PostgreSQL Data Types in Spanner](https://docs.cloud.google.com/spanner/docs/reference/postgresql/data-types)
- [Commit Timestamps in PostgreSQL-dialect Databases](https://docs.cloud.google.com/spanner/docs/commit-timestamp-postgresql)
- [Work with JSONB Data](https://docs.cloud.google.com/spanner/docs/working-with-jsonb)
- [Create and Manage Sequences](https://docs.cloud.google.com/spanner/docs/sequence-tasks)

### PostgreSQL Specific

- [Postgres LISTEN/NOTIFY Actually Scales — DBOS](https://www.dbos.dev/blog/postgres-listen-notify-scalability)
- [Postgres LISTEN/NOTIFY Does Not Scale — Recall.ai](https://www.recall.ai/blog/postgres-listen-notify-does-not-scale)
- [Scaling Postgres LISTEN/NOTIFY — PgDog](https://pgdog.dev/blog/scaling-postgres-listen-notify)

### Gecko PoCs

- [PR #16: Bigtable storage backend](https://github.com/openshift-online/gecko/pull/16)
- [PR #17: Firestore storage backend](https://github.com/openshift-online/gecko/pull/17)

### Kine Known Issues

- [k3s#9256: PostgreSQL becomes very slow after 3+ years](https://github.com/k3s-io/k3s/issues/9256)
- [kine#36: Thoughts on scaling](https://github.com/k3s-io/kine/issues/36)
- [kine#112: Lessons learned using different storage backend](https://github.com/k3s-io/kine/issues/112)
- [Exploring NATS as a backend for k3s](https://nats.io/blog/exploring-nats-as-a-backend-for-k3s/)
