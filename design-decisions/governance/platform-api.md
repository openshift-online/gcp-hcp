# Platform API: Centralized API Server for Cluster Lifecycle Management

***Scope***: GCP-HCP

**Date**: 2026-07-15

**Study**: [Platform API Experiment (orlop)](../../experiments/platform-api/orlop/ARCHITECTURE.md)

## Decision

We will implement a Platform API server as the single source of truth for the GCP HCP API definition, replacing the Hyperfleet API / CLM Backend. The Platform API provides a dual API surface (public and private) with schema-driven code generation, authorization middleware, and a clear separation between API concerns and cluster lifecycle business logic.

## Context

The cluster lifecycle API surface has evolved through three phases. It started as CLS (Cluster Lifecycle Service), a proof-of-concept that ended up deployed in production environments. The team then adopted CLM (Cluster Lifecycle Management) as part of the Hyperfleet project, with the API and backend developed by the Fleet Engineering team. The GCP HCP team is now transitioning to full-stack ownership, building the Platform API using the `orlop` framework in the `gecko` repository. Several gaps in the current architecture drive this change.

- **Problem Statement**: The current architecture lacks multi-tenancy, private API support, and a centralized API definition for code generation (CLI, controller clients). Full-stack ownership requires the GCP HCP team to control the API surface directly rather than depending on an upstream project.
- **Authorization Gap**: GCP first-party IAM integration is unavailable to external services, necessitating an authorization middleware integrated into the API server.
- **Constraints**: Must support both public (customer-facing) and private (controller-facing) API surfaces from a single source of truth. Must enable code generation for CLI and controller clients. Must support an authorization middleware for tenant-scoped access control. Must maintain an API abstraction in front of the database.
- **Assumptions**: The validated `orlop` framework experiment demonstrates the viability of the dual API surface pattern. The production repository is `gecko`. The team has Go expertise to build and maintain the API server.

## Alternatives Considered

1. **Platform API (orlop-based)**: Kube-like REST API server with dual public/private surfaces, schema-driven code generation, converter layer for field filtering, and embedded authorization middleware.
2. **Continue with Hyperfleet API / CLM Backend**: Maintain the existing backend developed by Fleet Engineering. Add multi-tenancy, private APIs, and authorization as extensions.
3. **Controllers reconciling directly against database**: The ROSA Hyperfleet approach — a customer-facing API records requests in the database, but controllers reconcile directly against the datastore rather than through an API abstraction layer.

## Decision Rationale

* **Justification**: The Platform API provides a single source of truth for API definitions that drives code generation for both CLI and controller clients. The dual public/private surface (validated in the orlop experiment) elegantly solves multi-tenancy field isolation: internal fields and metadata are automatically filtered from the public API via schema-driven converters, while updates via the public API preserve internal state. The API abstraction layer is essential for enforcing authorization, validation, and audit logging at a single chokepoint.
* **Evidence**: The `orlop` experiment (`experiments/platform-api/orlop/`) has been validated as a working prototype with dual API surfaces, converter layer, schema generation, and Kubernetes-style resource semantics (GVK, resourceVersion, SSA). The `platform-api` experiment (`experiments/platform-api/platform-api/`) extends this with GCP-specific types (HostedCluster, NodePool) and HyperShift private API types.
* **Comparison**: Continuing with the Hyperfleet API would require extensive rework for multi-tenancy and authorization without full ownership of the codebase. Direct database reconciliation (ROSA approach) loses the single enforcement point for authorization and validation — each controller must independently enforce access control and input validation.

## Consequences

### Positive

* Single source of truth for API definition — drives CLI generation, controller clients, and documentation
* Dual public/private API surface provides clean multi-tenancy field isolation
* Schema-driven code generation (`orlop-gen`) eliminates duplication between private and public API types
* Authorization middleware enforced at a single chokepoint (the API server)
* Kubernetes-style semantics (resourceVersion, SSA, GVK) familiar to the team
* Cluster lifecycle business logic is decoupled from the API server — lives in independent controllers
* Full ownership by the GCP HCP team enables faster iteration

### Negative

* Increased scope and maintenance burden for the GCP HCP team
* API server remains a critical path component — high availability requirements carry over from the existing CLM Backend
* Dual API surface requires two Go type definitions per resource (private with all fields, public with customer-visible fields only) kept in sync via code generation
* Converter layer depends on a consistent private prefix convention across labels, annotations, and conditions for metadata isolation

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: API server is stateless when backed by an external database (Firestore or PostgreSQL) and scales horizontally behind a load balancer. The storage backend handles persistence scaling independently.
* **Observability**: Request logging middleware captures method, path, status, and duration for all API calls. Integration with Cloud Monitoring for API-level metrics.
* **Resiliency**: Standard Kubernetes deployment with multiple replicas, health checks, and pod disruption budgets. With external storage, restart recovery is immediate since no in-process state is lost.

### Security:
- Authorization middleware enforces access control on every API request — default deny
- Public API automatically filters private fields, labels, annotations, and conditions
- Private API enforces the same authentication and authorization as the public API; additionally restricted to cluster-internal access (port 8080, no external exposure)
- Google JWT validation for authentication; no static credentials
- Tenant isolation via project-scoped authorization policies

### Performance:
- Converter layer adds minimal overhead (JSON round-trip for field filtering)
- Authorization evaluated inline — no external service call per request

### Operability:
- OpenAPI specification generated from Go types — always in sync with implementation
- Schema changes validated at compile time via code generation
- CLI and controller client libraries auto-generated from API definition
- Standard Kubernetes deployment patterns (Deployment, Service, HPA)
