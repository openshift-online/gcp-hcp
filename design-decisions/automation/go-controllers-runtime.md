# Controllers Runtime: Go Controllers for Cluster Lifecycle Management

***Scope***: GCP-HCP

**Date**: 2026-07-15

**Supersedes**: Configuration-based adapter framework ([GCP-333 Hyperfleet Adapters](../../implementation-plans/gcp-333-hyperfleet-adapters.md))

## Decision

We will replace the configuration-based adapter framework and Sentinel event publisher with Go-based Kube-like controllers developed, lifecycled, and released directly by the GCP HCP team. Controllers reconcile against the Platform API (via its private API surface) using watcher and time-based reconciliation patterns.

## Context

The GCP HCP team is transitioning to full-stack ownership of the Cluster Lifecycle layer. The existing adapter framework was developed by the Fleet Engineering team as part of the Hyperfleet project, using a configuration-driven approach with CEL-based preconditions and CloudEvent-driven reconciliation via Sentinel.

- **Problem Statement**: Ongoing development and troubleshooting challenges with the configuration-based adapter framework have slowed iteration on cluster lifecycle features. The framework introduces complexity through external components (Sentinel for event publishing, CEL for precondition evaluation, CloudEvents over Pub/Sub for reconciliation triggers) that increase the incident surface area and make debugging difficult.
- **Constraints**: Must preserve the existing cluster lifecycle business logic (placement, version resolution, signing key provisioning, hosted cluster creation). Must reconcile against the Platform API rather than directly against the database.
- **Assumptions**: Kube-like controller patterns (watch + reconcile, time-based requeue) are well-understood by the team and provide equivalent or superior reliability to the event-driven adapter model. The transition shifts framework ownership from Fleet Engineering to the GCP HCP team.

## Alternatives Considered

1. **Go controllers with watcher + time-based reconciliation**: Standard Kubernetes controller patterns applied to the Platform API. Each lifecycle concern is an independent controller binary.
2. **Continue with configuration-based adapters**: Maintain the CEL/CloudEvent/Sentinel framework. Extend with additional adapters as needed.
3. **Go rewrite of adapters with Pub/Sub reconciliation**: Rewrite the adapter business logic in Go (eliminating CEL) but retain the Sentinel/Pub/Sub event-driven reconciliation model. Validated end-to-end in a [prototype](https://github.com/openshift-online/gcp-hcp/pull/80).

## Decision Rationale

* **Justification**: Go controllers provide a familiar, well-understood pattern for the team. Eliminating Sentinel and the CEL-based precondition framework removes external components that are difficult to debug and contribute to incident surface area. The controller pattern — watch for changes, reconcile desired vs. actual state, requeue on failure — is the industry standard for Kubernetes-native lifecycle management. Direct team ownership enables rapid iteration on feature requests.
* **Evidence**: An early experiment validated that Go controllers work as a drop-in replacement for the adapter framework. The controller pattern has been proven at scale in Kubernetes operators, HyperShift itself, and numerous other control-plane systems. The existing adapter business logic (placement algorithm, version resolution via Cincinnati, signing key provisioning, HostedCluster creation via transport) is directly portable to controller reconciliation loops.
* **Comparison**:
  - **Configuration-based adapters**: Introduced CEL evaluation, CloudEvent parsing, and Sentinel event publishing as additional layers between the business logic and the reconciliation trigger — layers that add complexity without proportional benefit.
  - **Go adapter rewrite**: Eliminates CEL but retains the Sentinel/Pub/Sub event-delivery infrastructure that contributes to incident surface area — if the ownership transition already justifies the migration cost, it should target the ideal architecture rather than carry forward the event-coupling. It also stays coupled to the HyperFleet API rather than the Platform API's private surface, which does not align with the Platform API architecture.


## Consequences

### Positive

* Elimination of Sentinel reduces the component count and incident surface area
* Standard Go controller patterns are well-understood, debuggable, and testable
* Watcher + time-based reconciliation provides both responsiveness and self-healing
* Watching events from the transport layer (e.g., Maestro, kube-applier) is out of scope and requires additional investigation
* Full team ownership enables direct iteration without cross-team coordination
* Business logic from adapter implementation plans carries forward unchanged
* All controllers are compiled into a single image; the deployment is parameterized to select which controller to run
* Familiar development tooling: standard Go testing, profiling, and debugging

### Negative

* Increased scope for the GCP HCP team (framework ownership + business logic)
* Loss of the adapter framework's declarative configuration model (CEL preconditions)
* Dependency on the Platform API's private API surface for controller clients

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Each controller runs as a Kubernetes Deployment with configurable replicas. Leader election ensures single-writer semantics where needed.
* **Observability**: Standard controller-runtime metrics (reconcile duration, queue depth, error rate) exposed via Prometheus. Structured logging with reconciliation context.
* **Resiliency**: Time-based reconciliation provides self-healing — missed events are recovered on the next reconciliation cycle. Exponential backoff on transient failures.

### Security:
- Controllers access the Platform API via its private API surface (port 8080, cluster-internal only)
- Workload Identity Federation for all GCP API access — no static credentials
- Controller RBAC scoped to minimum required permissions

### Operability:
- Standard Kubernetes deployment (Deployment, ServiceAccount, RBAC)
- Single image with parameterized controller selection per deployment
- ArgoCD-managed deployment following existing sync wave conventions
- Health and readiness probes for each controller

