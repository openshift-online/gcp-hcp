# Gecko Backend Datastore: PostgreSQL for Dev, AlloyDB for Persistent Environments

***Scope***: GCP-HCP

**Date**: 2026-07-31

**Study**: [`studies/gecko-backend-datastore.md`](../../studies/gecko-backend-datastore.md)

## Decision

Use PostgreSQL (in Docker) for local development and minikube environments, and AlloyDB for PostgreSQL in persistent environments (staging and production). Gecko's pluggable `ResourceStore` architecture enables environment-appropriate backend selection without coupling the codebase to a single datastore. If cost is not a concern, Cloud Spanner could replace AlloyDB in production for zero-downtime operations and the strongest consistency guarantees.

## Context

Gecko (the Platform API server) requires a persistent datastore satisfying Kubernetes API semantics — atomic creates, optimistic concurrency control, monotonically increasing resource versions, ordered watch with historical replay, and label-selector filtering. The datastore must support multi-replica API server instances reading and writing concurrently.

- **Problem Statement**: etcd has operational limitations for a managed service: resiliency issues with Raft quorum, an 8 GB size limit, limited point-in-time recovery, and significant operational overhead. A managed GCP datastore eliminates these constraints while meeting Kubernetes storage semantics.
- **Constraints**:
  - Regional single-region deployment, duplicated per region (data sovereignty)
  - Zero-touch operations: autonomous upgrades, minimal operational burden
  - Managed via Config Connector (fallback: Terraform)
  - Multi-replica safe: multiple API server instances reading/writing concurrently
  - Cost-conscious: datastore duplicated in every region
  - **No static credentials**: all authentication must flow through GKE Workload Identity and GCP IAM
- **Assumptions**:
  - Gecko's `rest.Storage` + `ResourceStore` architecture (Pattern C) is the established approach — the codebase already has a working PostgreSQL implementation
  - The platform API workload is low-to-moderate write volume (not a high-throughput OLTP system)
  - PostgreSQL compatibility between dev and production environments reduces integration risk
  - AlloyDB's PostgreSQL wire compatibility means the same Gecko storage code runs against both backends

## Alternatives Considered

1. **Cloud Spanner (regional, 2 nodes)**: Google's globally distributed relational database. Proven as a Kubernetes storage backend via spanner-etcd; Google runs GKE at 65,000+ nodes on Spanner. Zero-downtime operations, TrueTime-backed linearizable consistency, Change Streams for watch (~30ms latency), 7-day PITR. Cost: ~$1,314/month per region (99.99% SLA), ~$788/month with 3-year CUD.

2. **Cloud SQL for PostgreSQL (HA)**: Managed PostgreSQL with familiar tooling and the lowest cost. Gecko already has a working implementation. LISTEN/NOTIFY for watch, PITR up to 35 days (Enterprise Plus). Cost: ~$80-120/month per region (99.95% SLA). Trade-offs: 5-10 minute downtime during automatic maintenance, no horizontal scaling. [Major version upgrades](https://cloud.google.com/sql/docs/postgres/upgrade-major-db-version-inplace) are significantly more involved than AlloyDB — requiring manual pre-checks (character set verification, extension compatibility, flag review, memory tuning), a 6-hour timeout ceiling with no guaranteed downtime window, mandatory post-upgrade `ANALYZE` runs, and recreation of logical replication slots that are deleted during the upgrade. Instances with high object counts (>10,000 tables) or large objects (>10 million) risk timeout or outright failure.

3. **AlloyDB for PostgreSQL (Primary)**: Google's PostgreSQL-compatible database with separated compute/storage layers. Near-zero downtime for minor version maintenance (<1 second), strong ACID consistency, microsecond-granularity PITR for 35 days. Reuses existing Gecko PostgreSQL code. Cost: ~$200-300/month per region (99.99% SLA). Trade-off: [major version upgrades](https://cloud.google.com/alloydb/docs/cluster-upgrade) require ~20+ minutes of downtime (in-place) or longer with migration-based approaches.

4. **Cloud Firestore (Native Mode)**: Truly serverless with real-time listeners and a free tier. Critical limitations: ~1 write/sec/document counter bottleneck caps practical throughput at ~10 writes/sec, document model mismatch with Kubernetes typed objects, client-side label filtering only, no transactional isolation for List.

5. **Cloud Bigtable**: Massive scale ceiling but fundamentally unsuitable — no cross-row transactions, counter hotspot with ReadModifyWriteRow, expensive minimum (~$1,400/month HA), all filtering client-side.

## Decision Rationale

* **Justification**: AlloyDB provides the best balance of production-grade reliability, PostgreSQL compatibility, and cost for persistent environments. Its near-zero downtime maintenance (<1 second) eliminates the 5-10 minute maintenance windows of Cloud SQL — critical for a platform API server that backs cluster lifecycle operations. PostgreSQL compatibility means the same Gecko `ResourceStore` code runs unchanged against both the local dev PostgreSQL and production AlloyDB, eliminating an entire class of integration bugs. For local development, plain PostgreSQL in Docker provides zero-cost, instant-start iteration with full code compatibility. If cost becomes unconstrained, Cloud Spanner could replace AlloyDB in production — it offers true zero-downtime operations (not just near-zero), TrueTime-backed linearizable consistency (the strongest available), and is proven at Google's own scale running GKE for 65,000+ nodes.

* **Evidence**: The upstream datastore study evaluated all five alternatives against Kubernetes storage requirements. AlloyDB satisfies all functional requirements (atomic create, optimistic concurrency, monotonic resource versions, ordered watch, label filtering) and all non-functional requirements (strong consistency, multi-replica safety, regional deployment, zero-touch ops, PITR). Gecko's existing PostgreSQL implementation (using `pg_current_xact_id()` for monotonic resource versioning, LISTEN/NOTIFY doorbell pattern for watch) runs unmodified on AlloyDB. The postgres-controller-backend project validates the dual-versioning pattern in production.

* **Comparison**:
  - **vs. Cloud Spanner**: Spanner offers the strongest guarantees (linearizable via TrueTime, true zero-downtime, Change Streams) but at 4-6x the cost of AlloyDB. It also uses a distinct SQL dialect — requiring a separate `ResourceStore` implementation rather than reusing the PostgreSQL code path. Notably, Spanner handles all upgrades — including major versions — with zero downtime, whereas AlloyDB major version upgrades require ~20+ minutes of planned downtime. This makes Spanner more desirable for workloads that cannot tolerate any maintenance windows. Spanner remains a viable upgrade path if cost constraints relax.
  - **vs. Cloud SQL**: Cloud SQL is cheapest but its 5-10 minute automatic maintenance downtime is unacceptable for a production platform API backing cluster lifecycle operations. AlloyDB's <1 second maintenance provides a 99.99% SLA versus Cloud SQL's 99.95%. Major version upgrades are also substantially harder on Cloud SQL — requiring [manual pre-checks, extension cleanup, flag review, memory tuning, and post-upgrade maintenance](https://cloud.google.com/sql/docs/postgres/upgrade-major-db-version-inplace) with no guaranteed downtime window and a 6-hour timeout ceiling. AlloyDB's in-place major upgrade is a single operation with ~20 minutes of downtime.
  - **vs. Firestore**: Counter bottleneck (~10 writes/sec practical limit) and document model mismatch make it unsuitable for Kubernetes API semantics. Already chosen as the [transport layer](../networking/datastore-transport.md) — using it for both would conflate two distinct architectural roles.
  - **vs. Bigtable**: No cross-row transactions, client-side filtering, and ~$1,400/month minimum make it over-provisioned and under-featured for this workload.

## Consequences

### Positive

* Single PostgreSQL-compatible codebase serves all environments — no separate storage backend implementations to maintain
* Zero-cost local development with PostgreSQL in Docker; instant startup, no cloud dependency
* AlloyDB's near-zero downtime maintenance (<1 second) provides production-grade availability without maintenance windows
* 99.99% SLA in production with AlloyDB Primary instances
* Microsecond-granularity PITR for 35 days provides strong disaster recovery
* Separated compute/storage architecture in AlloyDB allows independent scaling
* Proven implementation patterns: `pg_current_xact_id()` for monotonic resource versioning, LISTEN/NOTIFY doorbell for watch, atomic CTE for compaction
* Clear upgrade path to Spanner if cost is not a concern and stronger guarantees are needed

### Negative

* AlloyDB is 2-3x the cost of Cloud SQL (~$200-300/month vs. ~$80-120/month per region)
* LISTEN/NOTIFY has performance limits at scale — may require fallback to polling at very high watch counts
* AlloyDB is not serverless — requires provisioned compute capacity even during idle periods
* [Major version upgrades](https://cloud.google.com/alloydb/docs/cluster-upgrade) require ~20+ minutes of downtime (in-place method) — Spanner has no such limitation
* Not horizontally scalable for writes (read replicas available, but write scaling is vertical only)
* AlloyDB requires Private Service Connect or VPC peering for connectivity — more complex networking than Cloud SQL's Auth Proxy

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: AlloyDB's separated compute/storage allows independent storage scaling. Read replicas can offload read-heavy watch traffic. For the expected low-to-moderate platform API write volume, a single 2 vCPU primary instance is sufficient. PostgreSQL in dev matches the same SQL semantics at any scale.
* **Observability**: AlloyDB provides built-in Cloud Monitoring integration with query insights, per-query latency, and resource utilization metrics. PostgreSQL in dev can use standard `pg_stat_statements` for query analysis.
* **Resiliency**: AlloyDB Primary provides 99.99% SLA with automatic failover across zones within a region. Near-zero downtime maintenance (<1 second) eliminates planned downtime. PITR with microsecond granularity for 35 days. In dev/minikube, PostgreSQL data is ephemeral by design — fast iteration over durability.

### Security:

* AlloyDB authentication via IAM Database Authentication — GKE Workload Identity maps directly to database access, no static credentials
* AlloyDB Cluster and instances are VPC-native with private IP only — no public endpoint exposure
* Data encrypted at rest (Google-managed or CMEK) and in transit (TLS enforced)
* In dev, PostgreSQL runs locally with no external network exposure

### Performance:

* AlloyDB: single-digit millisecond read/write latency within a region, optimized for PostgreSQL workloads with Google's custom storage layer
* LISTEN/NOTIFY watch latency: sub-100ms with the doorbell pattern (notification triggers immediate read from event log)
* `pg_current_xact_id()` provides monotonic resource versions without a shared counter — no write serialization bottleneck

### Cost:

| Environment | Backend | Estimated Cost | SLA |
|-------------|---------|---------------|-----|
| Local dev / minikube | PostgreSQL (Docker) | $0 | N/A |
| CI | PostgreSQL (Docker in test) | $0 | N/A |
| Staging | AlloyDB Primary (2 vCPU) | ~$200-300/month | 99.99% |
| Production | AlloyDB Primary (2 vCPU) | ~$200-300/month | 99.99% |
| Production (if cost unconstrained) | Cloud Spanner (2 nodes) | ~$1,314/month | 99.99% |

### Operability:

* Gecko's pluggable `ResourceStore` interface selects the backend at startup — environment-tier configuration, not code changes
* AlloyDB managed via Config Connector or Terraform — standard IaC provisioning
* PostgreSQL in dev requires only `docker run postgres` — no cloud provisioning for local iteration
* Database schema migrations are shared across all PostgreSQL-compatible backends
* AlloyDB minor version upgrades are automatic with near-zero downtime; [major version upgrades](https://cloud.google.com/alloydb/docs/cluster-upgrade) require ~20+ minutes of planned downtime (in-place) and must be scheduled during low-traffic windows
* Dev PostgreSQL Docker image version must track AlloyDB's current major PostgreSQL version to maintain compatibility

---

## Template Validation Checklist

### Structure Completeness
- [x] Title is descriptive and action-oriented
- [x] Scope is GCP-HCP
- [x] Date is present and in ISO format (YYYY-MM-DD)
- [x] All core sections are present: Decision, Context, Alternatives Considered, Decision Rationale, Consequences
- [x] Both positive and negative consequences are listed

### Content Quality
- [x] Decision statement is clear and unambiguous
- [x] Problem statement articulates the "why"
- [x] Constraints and assumptions are explicitly documented
- [x] Rationale includes justification, evidence, and comparison
- [x] Consequences are specific and actionable
- [x] Trade-offs are honestly assessed

### Cross-Cutting Concerns
- [x] Each included concern has concrete details (not just placeholders)
- [x] Irrelevant sections have been removed
- [x] Security implications are considered where applicable
- [x] Cost impact is evaluated where applicable

### Best Practices
- [x] Document is written in clear, accessible language
- [x] Technical terms are used appropriately
- [x] Document provides sufficient detail for future reference
- [x] All placeholder text has been replaced
- [x] Links to related documentation are included where relevant
