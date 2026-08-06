# Platform API Storage Backend Selection

***Scope***: GCP-HCP

**Date**: 2026-07-21

## Decision

Pending. This document compares four storage backend architectures for the Platform API (orlop) to inform a final decision.

## Context

- **Problem Statement**: The Platform API server implements a Kubernetes-style API (CRUD, watches, server-side apply, garbage collection, optimistic concurrency) and needs a production storage backend that supports real-time event streaming, horizontal scalability, and operational simplicity on GCP.
- **Constraints**: Must run on GCP. Must implement the `ResourceStore` interface (Create, Get, List, Update, Delete, Watch with event replay). Must support monotonically increasing resource versions, label-selector filtering, pagination, shard-based partitioning, and context-based multi-tenancy. Watch delivery must be cross-instance for HA deployments.
- **Assumptions**: Multiple API server instances will share the storage backend. The system must tolerate instance restarts without losing watch continuity. Write throughput is moderate (not millions of writes/second). The target SLO is aligned with managed Kubernetes control planes.

## Alternatives Considered

### 1. PostgreSQL with LISTEN/NOTIFY (postgres-controller-backend)

Uses [postgres-controller-backend](https://github.com/jmelis/postgres-controller-backend), which replaces etcd with PostgreSQL. Writes go through a `pgctl_write()` stored procedure stamped with `pg_current_xact_id()`. Watches use a poll-primary model where LISTEN/NOTIFY is a latency optimization, not a correctness mechanism. Resource versions map directly to PostgreSQL transaction IDs.

### 2. PostgreSQL + Message Broker

PostgreSQL for durable storage with a separate message broker (e.g., Pub/Sub, NATS, or Redis Streams) for event fan-out. Mutations are written to PostgreSQL, then published to the broker. Watch subscribers consume from the broker and fall back to database queries for replay.

### 3. Google Firebase (Firestore)

Google's managed NoSQL document database with native real-time snapshot listeners. Documents are organized in collections, and the Go SDK supports server-side snapshot listeners that push changes without polling.

### 4. Google Cloud Spanner

Google's globally distributed relational database with native Change Streams for real-time change data capture. Change records are written atomically within the same transaction as the data mutation and can be consumed via the Spanner API, Dataflow, or Kafka connectors.

## Comparison

### 1. PostgreSQL with LISTEN/NOTIFY (postgres-controller-backend)

**Pros**
- Purpose-built for Kubernetes controller patterns; implements controller-runtime `Manager` interface with minimal migration effort
- Resource versions use native PostgreSQL transaction IDs (`pg_current_xact_id()`), eliminating in-process counters and making multi-instance safe by design
- Poll-primary watch model is resilient to notification loss; LISTEN/NOTIFY only reduces latency, never affects correctness
- No-op write suppression avoids spurious events when data hasn't changed
- Single dependency (PostgreSQL 16+); no additional infrastructure to operate
- Well-understood operational model; Cloud SQL for PostgreSQL is a managed GCP service
- Proven performance: ~15k writes/s small payloads, ~6k writes/s at 15-20KB on Aurora; comparable performance expected on Cloud SQL
- Strong correctness guarantees with 7 formally stated invariants and deterministic race tests

**Cons**
- Requires all writers to use this library; incompatible with raw SQL writes to the same tables
- Single-primary PostgreSQL only; no multi-master or logical replication support
- Poll interval (default 5s) introduces baseline latency for watch delivery even with NOTIFY optimization
- Requires PostgreSQL 16+ for `pg_current_xact_id()` and snapshot functions
- Relatively new project; smaller community compared to established alternatives
- Vertical scaling ceiling on a single PostgreSQL primary; read replicas help reads but writes are bounded by one node

### 2. PostgreSQL + Message Broker

**Pros**
- Separates storage concerns (durability) from event delivery (fan-out), allowing each component to scale independently
- Message broker (Pub/Sub, NATS) provides push-based delivery with lower latency than polling
- Pub/Sub is fully managed on GCP with at-least-once delivery and no infrastructure to operate
- PostgreSQL handles all query patterns (label selectors, pagination, sharding) with mature SQL capabilities
- Flexible: broker can be swapped (Pub/Sub, NATS, Redis Streams) without changing the storage layer
- Existing orlop postgres store already uses a dual-write pattern (event log table + NOTIFY), making the architecture a natural evolution

**Cons**
- Dual-write problem: writing to both PostgreSQL and the broker is not atomic; crash between the two creates inconsistency unless carefully handled (outbox pattern, CDC)
- Additional operational complexity: two systems to monitor, tune, and keep available
- Message ordering guarantees vary by broker; Pub/Sub does not guarantee ordering without ordering keys, and even then only within a partition
- Event replay requires either broker retention configuration or fallback to database queries, adding complexity to the watch implementation
- Higher cost: managed broker fees on top of PostgreSQL costs
- Increased latency variance: end-to-end latency depends on both systems

### 3. Google Firebase (Firestore)

**Pros**
- Fully managed and serverless; zero operational overhead for the database layer
- Native real-time snapshot listeners with push-based change delivery; no polling needed
- Go SDK supports server-side real-time listeners via `Snapshots()` on documents and queries
- Automatic scaling with no capacity planning required
- Built-in multi-region replication and high availability
- Document model is a natural fit for storing Kubernetes-style JSON objects
- Strong consistency for reads within a single document

**Cons**
- NoSQL document model does not natively support complex queries: no SQL-style label selector filtering, no JOINs for owner reference lookups during GC
- No server-side transaction IDs or monotonic counters; resource version semantics must be implemented in application code
- Query limitations: composite indexes must be pre-defined; inequality filters on multiple fields require composite indexes; `OR` queries are limited
- 1 MiB maximum document size; large objects with many annotations/labels could approach this limit
- Firestore's consistency model is per-document; cross-document transactions have a 500-document limit and higher latency
- No native sharding by hash; shard-based partitioning would require application-level collection design
- Vendor lock-in: no portable migration path; Firestore's data model and query semantics are proprietary
- Limited server-side Go SDK ecosystem compared to PostgreSQL; no offline caching on server SDKs
- Pricing model (per read/write/delete operation + storage) can be unpredictable at scale and expensive for high-throughput list operations

### 4. Google Cloud Spanner

**Pros**
- Globally distributed with strong consistency (external consistency / linearizability); no stale reads across regions
- Native Change Streams provide real-time CDC written atomically within the same transaction as data mutations; no dual-write problem
- Horizontally scalable for both reads and writes; no single-primary bottleneck
- SQL support with a relational model: label selector queries, JOINs for GC owner lookups, and indexed columns all work naturally
- Fully managed on GCP with automatic sharding, replication, and failover
- Commit timestamps can serve as resource versions, providing globally ordered, monotonically increasing values without application-level counters
- Change Streams support heartbeat intervals for keep-alive, analogous to Kubernetes watch bookmarks
- Fine-grained IAM and encryption (CMEK) out of the box

**Cons**
- Highest cost of all options: minimum ~$0.90/node-hour for regional, ~$2.70/node-hour for multi-region; even a minimal single-node regional instance costs ~$650/month before I/O
- Change Streams are consumed via Spanner API TVF (table-valued function) or Dataflow; implementing a Kubernetes-style Watch with replay, filtering, and bookmark semantics requires significant custom integration code
- Change Stream reads consume Spanner compute resources; high fan-out to many watchers can impact transactional workload performance
- No LISTEN/NOTIFY equivalent; Change Streams are pull-based (poll with heartbeat), not push-based
- Spanner SQL is not fully PostgreSQL-compatible; migration of existing queries requires adaptation (e.g., different function names, no `JSONB` type, uses `JSON` type instead)
- Interleaved tables and primary key design require careful schema modeling to avoid hotspots; poor key design leads to split contention
- Overkill for moderate write throughput; the global consistency guarantees come with latency overhead that may not be needed for a single-region deployment
- Steeper learning curve for schema design, key selection, and Change Stream consumption patterns

---

## Template Validation Checklist

### Structure Completeness
- [x] Title is descriptive and action-oriented
- [x] Scope is GCP-HCP
- [x] Date is present and in ISO format (YYYY-MM-DD)
- [x] All core sections are present: Decision, Context, Alternatives Considered
- [ ] Decision Rationale (pending selection)
- [ ] Consequences (pending selection)

### Content Quality
- [x] Problem statement articulates the "why"
- [x] Constraints and assumptions are explicitly documented
- [x] Trade-offs are honestly assessed
