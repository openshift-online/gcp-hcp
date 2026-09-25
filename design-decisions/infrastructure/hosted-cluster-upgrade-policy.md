# Mandatory Platform-Managed Upgrades for Hosted Cluster Control Planes

***Scope***: GCP-HCP

**Date**: 2026-06-29

## Decision

Adopt mandatory platform-managed upgrades for hosted cluster control planes following the GKE model: control plane upgrades are automatic and cannot be disabled. Customers control timing of both y-stream and z-stream upgrades through maintenance windows, maintenance exclusions, and channel selection.

Node Pool upgrades are entirely customer-triggered — manual or scheduled. Customers can specify a target version for the Node Pool; the default is the current control plane version. The target version must be within the supported version skew (N-3 minor versions) and cannot exceed the current control plane version. The platform does not automatically upgrade Node Pools.

## Context

- **Problem Statement**: GCP HCP needs a defined upgrade lifecycle policy that balances platform safety (supported, secure versions) with customer operational needs (predictable upgrade timing).

- **Constraints**:
  - Must be compatible with HyperShift's upgrade mechanics (CVO, Cincinnati upgrade graph, sequential minor version steps)
  - Must communicate version skew status between control plane and Node Pools (N-3 minor versions) and enforce out-of-support policy when the limit is exceeded
  - Must support EUS-to-EUS upgrade paths (workers skipping odd minor versions)
  - Must provide customer controls that map to familiar GKE concepts where possible
  - Cincinnati is the canonical source for OCP release versions and upgrade paths, evaluated per cluster via the HostedCluster CR (see [Adopt Cincinnati for Version Resolution](../governance/adopt-cincinnati-for-version-resolution.md))
  - Control plane upgrades are the platform's responsibility as a managed service — customers should not be exposed to internal upgrade plumbing

- **Assumptions**:
  - HyperShift handles the actual control plane rollout mechanics (etcd, kube-apiserver, kube-controller-manager, openshift-apiserver, CVO)
  - Cincinnati upgrade graph is authoritative for determining valid upgrade edges
  - Progressive fleet rollout will be used — not all clusters upgrade simultaneously
  - GKE's upgrade model is the target customer experience for control planes

## Alternatives Considered

1. **GKE model (chosen)**: As described in the Decision section above.

2. **ROSA model — customer-triggered upgrades with EOL enforcement**: Upgrades are not enforced until end-of-life. Customers schedule and trigger all upgrades manually. If a cluster reaches EOL, the platform force-upgrades it.

3. **Fully customer-managed upgrades**: The platform publishes available versions but never initiates upgrades. Customers are responsible for all upgrade decisions and execution.

## Decision Rationale

* **Justification**: Mandatory control plane upgrades eliminate the class of support issues where customers run EOL versions for months. Customer timing controls provide operational flexibility without allowing indefinite deferral.

* **Evidence**: GKE's upgrade model is proven at scale and well-understood by the target customer base (GCP-native users). ROSA's experience shows that optional upgrades lead to long-tail version sprawl — some customers stay on unsupported versions for extended periods, creating security exposure and support complexity.

* **Comparison**:
  - Alternative 2 (ROSA): allows indefinite deferral until EOL, creating version sprawl. EOL force-upgrades are disruptive and come too late.
  - Alternative 3 (fully customer-managed): transfers all upgrade responsibility to customers. No major managed Kubernetes platform uses this model.

## Upgrade Model

### Available Channels

- **Fast**: GA releases appear immediately. Fully supported.
- **Stable**: Same releases after a soak period on fast. Default channel.
- **EUS**: Even-numbered minor versions only (4.14, 4.16, …). 18-month support. Enables EUS-to-EUS updates. The platform rejects EUS selection for clusters on odd minor versions.

### Version Skew Policy

The platform tracks version skew between control plane and Node Pools:

| Constraint | Rule | Enforcement |
|---|---|---|
| **Worker-to-control-plane skew** | Workers should not be more than **N-3 minor versions** behind the control plane | Platform notifies the customer as skew approaches the limit. If N-3 is exceeded, the Node Pool enters out-of-support status (no alerting, no Red Hat support until the Node Pool is upgraded) |
| **Worker version ceiling** | Workers cannot run a version **newer** than the control plane | Platform blocks worker upgrades beyond the control plane version |
| **Upgrade order** | Control plane upgrades **before** workers | Platform enforces this ordering — worker upgrades can target any version up to and including the current control plane version |
| **Minor version steps** | Minor version upgrades are sequential (e.g., 4.22 → 4.23 → 4.24, not 4.22 → 4.24) | Platform enforces single-step minor upgrades via Cincinnati upgrade graph |

The specific out-of-support policy for Node Pools exceeding version skew will be documented separately.

### Control Plane Upgrades

Control plane upgrades are **mandatory and platform-managed**. Version downgrades are not supported — Cincinnati only publishes forward upgrade edges.

**Upgrade triggers:**
- Automated (mandatory): new default version set on the channel
- Manual (optional): customer initiates upgrade ahead of the automatic schedule

**Delay controls:**
- All control plane upgrades (both y-stream and z-stream) respect maintenance windows and maintenance exclusions

#### New Cluster Default Version and Fleet Minimum Y-Version

The platform maintains two separate version controls:

- **New cluster default version**: The platform-defined default y-version for the channel. New clusters are created at the latest z-stream of this version. Updated when the platform promotes a new y-stream version.
- **Fleet minimum y-version**: A separate floor for existing clusters, decoupled from the new cluster default. Only bumped when the current minimum approaches EOL, providing stability for existing clusters. The bump process includes customer communications (defined separately).

This decoupling means promoting a new default version affects new cluster creation but does not automatically force existing clusters to that y-version. Existing clusters upgrade to a new y-version when the applicable fleet minimum y-version advances — either because the platform bumps it (approaching EOL) or because the customer switches to a channel with a higher fleet minimum y-version — or when a customer manually requests a y-stream upgrade. These y-stream upgrades follow the same delay and override rules as all other control plane upgrades.

#### Version Promotion to Channel Default

The promotion flow applies to both y-stream and z-stream versions, with different effects on the fleet:

- **Z-stream** default promotion triggers progressive upgrades to existing clusters (respecting maintenance windows and exclusions)
- **Y-stream** default promotion updates the new cluster creation version; existing clusters only upgrade when the fleet minimum y-version is bumped or when a customer requests a y-stream control plane upgrade

The promotion flow:

1. Red Hat publishes a new GA version to Cincinnati (the platform can block versions from Cincinnati if needed)
2. The version becomes available in the corresponding channel (fast, stable, or EUS) per Red Hat's channel policies
3. The platform evaluates the version against its promotion criteria (defined separately) to determine readiness as the channel default
4. Once the criteria are met, the platform promotes the version to the channel's **default** — an internal platform operation, not a Cincinnati concept
5. New clusters are created at the new default version; existing clusters receive z-stream upgrades progressively (progressive delivery policy documented separately). Automatic y-stream upgrades to existing clusters are governed by the fleet minimum y-version (see above), not by the channel default promotion; customers can also manually request a y-stream upgrade

### Node Pool Upgrades

Node Pool upgrades are **entirely customer-triggered**. Customers can specify a target version; the default is the current control plane version. The target version must not exceed the control plane version and must be within the supported version skew (N-3).

**Upgrade triggers:**
- Manual: customer initiates upgrade to a chosen version (default: current control plane version)
- Scheduled: one-off scheduled upgrade to a chosen version (default: current control plane version)

## Customer Controls for Control Plane Upgrades

### Release Channel Selection

Customers select a release channel that determines which versions are offered for upgrade. Channels map to Cincinnati channel groups.

- Channel is set at cluster creation (default: `stable`) and can be changed at any time
- Switching from EUS to stable or fast may trigger an upgrade if the fleet minimum y-version in the new channel is newer than the cluster's current version

### Maintenance Windows

**Properties:**
- **Recurrence**: [RFC 5545](https://datatracker.ietf.org/doc/html/rfc5545) recurrence rules so that a single window can express complex schedules like "weekdays 2-6 AM UTC" or "Saturdays and Sundays only"
- **Duration**: Minimum 4 hours (TBD from Perf/Scale tests) to allow upgrades to complete
- **Default**: If no maintenance window is set, the platform may upgrade at any time
- **Scope**: Applies to all control plane upgrades (both y-stream and z-stream)

### Maintenance Exclusions

Customer-defined blackout periods during which no automatic upgrades are applied, even if a maintenance window is open.

- Up to 3 maintenance exclusions (aligned with GKE)
- Maximum duration 30 days
- Must leave at least 48 hours of maintenance availability in any rolling 32-day window (aligned with GKE). The platform rejects maintenance exclusion configurations that would violate this constraint at the API level

**Override conditions** — maintenance windows and exclusions are respected *except when*:
- The cluster's y-stream version is within **30 days of EOL** (aligned with GKE)
- A z-stream upgrade addressing a **critical security or platform issue** requires immediate patching

### Manual Upgrades

**Capabilities:**
- **List available upgrades**: Query the platform for versions the cluster can upgrade to (based on channel and Cincinnati upgrade graph, evaluated in-cluster)
- **Initiate upgrade**: Start an upgrade to a chosen version from the available list
- **View upgrade status**: Monitor progress of an in-progress upgrade

### Customer Notifications

| Event | Notification |
|---|---|
| New version available in channel | Control Plane event / notification |
| Automatic upgrade scheduled | Advance notification with target version and window |
| Upgrade started | Control Plane status update |
| Upgrade completed | Control Plane status update |
| Upgrade failed | Control Plane status update with error details |
| Upgrade remediation in progress | Control Plane status update indicating remediation is in progress |
| Channel change triggering upgrade | Control Plane event with target version and reason |
| Node Pool version skew approaching N-3 | Control Plane event with Node Pool identifier and recommended action |
| Fleet minimum y-version bumped | Advance notification with new minimum version, upgrade timeline, and required action |
| Delay override pending | Advance notification with reason and override timeline |

## Customer Controls for Node Pool Upgrades

- **Manual upgrade**: initiate upgrade to a chosen version (default: current control plane version; must be within N-3 skew and not exceed the control plane version)
- **Scheduled upgrade**: one-off scheduled upgrade to a chosen version (same constraints apply)
- **View upgrade status**: monitor progress of an in-progress upgrade

### Customer Notifications

| Event | Notification |
|---|---|
| New version available (control plane upgraded) | NodePool event / notification |
| Upgrade started | NodePool status update |
| Upgrade completed | NodePool status update |
| Upgrade failed | NodePool status update with error details |

## EOL with No Upgrade Edge

If a cluster approaches EOL but no valid Cincinnati upgrade edge exists from its current version (e.g., an edge was removed due to a regression, or the customer is on a z-stream with no edge to the next y-stream), the platform handles this as an operational issue:

1. **Detection**: The platform detects clusters approaching EOL with no available upgrade edge and raises an internal alert
2. **Escalation**: The operations team engages Red Hat to restore or add the missing upgrade edge, or to provide an alternative resolution path
3. **Resolution**: The platform applies the upgrade once a valid edge is available

## Consequences

### Positive

* Eliminates long-tail version sprawl seen in customer-triggered models
* Customer timing controls provide operational flexibility for all control plane upgrades (y-stream and z-stream)
* Node Pool upgrades remain fully customer-controlled
* Override mechanism ensures critical situations are not blocked by customer-configured delays
* EUS channel support enables extended stability between minor versions
* Progressive fleet rollout minimizes blast radius of problematic releases
* Decoupling fleet minimum y-version from new cluster default provides greater stability for existing clusters

### Negative

* Customers cannot defer y-stream upgrades indefinitely — less flexible than ROSA
* Override conditions can force upgrades outside customer-preferred windows
* Node Pool version divergence is possible — exceeding N-3 skew puts the Node Pool out of support
* Dependency on Red Hat for edge resolution in EOL-with-no-edge scenarios
* Decoupling fleet minimum y-version from new cluster default may result in a wider spread of y-versions across the fleet

## Cross-Cutting Concerns

### Reliability:

* **Resiliency**: Multi-replica control planes minimize API availability impact during upgrades. Failed control plane upgrades trigger an SRE alert — no automatic rollback. Failed worker node replacements leave the old node in place.
* **Observability**: Customer notifications for upgrade lifecycle events. Platform-side metrics for upgrade success rates, duration, and remediation time.

### Security:

* All control plane upgrades (including z-stream security patches) respect customer timing controls; override conditions (EOL, critical security or platform issues) ensure critical situations are still resolved
* Customer notifications at skew limit provide clear signal to act

### Performance:

* Control plane upgrades complete in minutes with seconds-level API unavailability
* Worker node upgrades use maxSurge/maxUnavailable to balance speed vs. resource consumption
* PDB-aware draining minimizes workload disruption during worker upgrades

### Cost:

* Replace strategy for worker upgrades temporarily increases resource consumption (maxSurge creates extra nodes during rollout)
* Progressive fleet rollout may require additional platform infrastructure for orchestration and monitoring

### Operability:

* Channel and version management leverages existing Cincinnati infrastructure — no new version management systems needed
* Maintenance window configuration uses standard RFC 5545 recurrence rules
* Platform must implement progressive rollout orchestration, override logic, and customer notification pipeline
* EOL-with-no-edge handling requires operational runbook for Red Hat engagement

## References

- [GKE Cluster Upgrades](https://cloud.google.com/kubernetes-engine/upgrades)
- [GKE Maintenance Windows and Exclusions](https://cloud.google.com/kubernetes-engine/docs/concepts/maintenance-windows-and-exclusions)
- [Kubernetes Version Skew Policy](https://kubernetes.io/releases/version-skew-policy/)
- [OCP Update Channels](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/updating_clusters/understanding-openshift-updates-1)
- [Adopt Cincinnati for Version Resolution](../governance/adopt-cincinnati-for-version-resolution.md)
- [GKE Fleet Management](../infrastructure/gke-fleet-management.md)
- [Cincinnati Update Service (OpenShift docs)](https://docs.openshift.com/container-platform/latest/updating/understanding_updates/intro-to-updates.html)
- [ROSA Upgrade Documentation](https://docs.openshift.com/rosa/upgrading/rosa-hcp-upgrading.html)

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
