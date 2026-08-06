# Mandatory Platform-Managed Upgrades for Hosted Cluster Control Planes

***Scope***: GCP-HCP

**Date**: 2026-06-29

## Decision

Adopt mandatory platform-managed upgrades for hosted cluster control planes following the GKE model: control plane upgrades are automatic and cannot be disabled. Customers control timing of y-stream upgrades through maintenance windows, maintenance exclusions, and channel selection. Z-stream upgrades are fully automatic and do not respect delay controls.

Node Pool upgrades are entirely customer-triggered — manual or scheduled — and always target the current control plane version. The platform does not automatically upgrade Node Pools.

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
| **Upgrade order** | Control plane upgrades **before** workers | Platform enforces this ordering — worker upgrades can only target the current control plane version |
| **Minor version steps** | Minor version upgrades are sequential (e.g., 4.22 → 4.23 → 4.24, not 4.22 → 4.24) | Platform enforces single-step minor upgrades via Cincinnati upgrade graph |

The specific out-of-support policy for Node Pools exceeding version skew will be documented separately.

### Control Plane Upgrades

Control plane upgrades are **mandatory and platform-managed**. Version downgrades are not supported — Cincinnati only publishes forward upgrade edges.

**Upgrade triggers:**
- Automated (mandatory): new default version set on the channel
- Manual (optional): customer initiates upgrade ahead of the automatic schedule

**Delay controls:**
- Y-stream (minor) upgrades respect maintenance windows and maintenance exclusions
- Z-stream (patch) upgrades are fully automatic and do not respect delay controls

#### Version Promotion to Channel Default

The promotion flow:

1. Red Hat publishes a new GA version to Cincinnati
2. The version becomes available in the corresponding channel (fast, stable, or EUS) per Red Hat's channel policies
3. The platform runs internal validation against the new version (e2e tests, upgrade tests)
4. Once validation passes, the platform promotes the version to the channel's **default** — an internal platform operation, not a Cincinnati concept
5. The new default triggers progressive fleet-wide upgrades (progressive delivery policy documented separately)

**How it works:**
1. CVO queries Cincinnati via `spec.channel` and populates `status.version.availableUpdates`
2. Platform evaluates upgrade eligibility (channel target, maintenance window, exclusions)
3. Platform updates `spec.release.version`, triggering version-resolution via Cincinnati
4. HyperShift orchestrates the control plane rollout
5. Rollout proceeds progressively across the fleet

### Node Pool Upgrades

Node Pool upgrades are **entirely customer-triggered**. The target version is always the current control plane version.

**Upgrade triggers:**
- Manual: customer initiates upgrade to match the current control plane version
- Scheduled: one-off scheduled upgrade to match the control plane version

**Upgrade strategy — Replace (default):** Creates new machine instances with the target version and removes old ones in a rolling fashion. HyperShift uses CAPI MachineDeployments — when the NodePool version changes, CAPG creates new GCPMachine resources (new GCE instances), drains pods from old nodes, then deletes them.

HyperShift Operator knobs:
- **maxSurge**: how many extra nodes to create during the rollout (more = faster but uses more quota)
- **maxUnavailable**: how many nodes can be down simultaneously
- **nodeDrainTimeout**: force-removes stuck nodes after a timeout
- **nodeVolumeDetachTimeout**: force-detaches volumes after a timeout

**Future direction — Karpenter:** With Karpenter, upgrades shift to drift-based node replacement through Karpenter's disruption controls. The upgrade policy remains the same — only the execution mechanism changes.

## Customer Controls for Control Plane Upgrades

### Release Channel Selection

Customers select a release channel that determines which versions are offered for upgrade. Channels map to Cincinnati channel groups.

- Channel is set at cluster creation (default: `stable`) and can be changed at any time
- Changing channel might trigger an upgrade if the **default** version in the new channel is newer than the current version

### Maintenance Windows

**Properties:**
- **Recurrence**: [RFC 5545](https://datatracker.ietf.org/doc/html/rfc5545) recurrence rules so that a single window can express complex schedules like "weekdays 2-6 AM UTC" or "Saturdays and Sundays only"
- **Duration**: Minimum 4 hours (TBD from Perf/Scale tests) to allow upgrades to complete
- **Default**: If no maintenance window is set, the platform may upgrade at any time
- **Scope**: Applies to control plane y-stream upgrades only (z-stream upgrades are fully automatic)

### Maintenance Exclusions

Customer-defined blackout periods during which no automatic y-stream upgrades are applied, even if a maintenance window is open.

- Up to 3 maintenance exclusions (aligned with GKE)
- Maximum duration 30 days
- Must leave at least 48 hours of maintenance availability in any rolling 32-day window (aligned with GKE). The platform rejects maintenance exclusion configurations that would violate this constraint at the API level

**Override conditions** — maintenance windows and exclusions are respected *except for*:
- Z-stream upgrades (always automatic)
- Version is within **30 days of EOL** (aligned with GKE)
- Workers are approaching the **version skew limit** (N-3) — the platform notifies the customer but does not block the control plane upgrade
- The cluster version is **incompatible with a required platform component** update

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
| Delay override pending | Advance notification at least **7 days** before override, with reason and override timeline |

## Customer Controls for Node Pool Upgrades

- **Manual upgrade**: initiate upgrade to match current control plane version
- **Scheduled upgrade**: one-off scheduled upgrade to match control plane version
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
* Customer timing controls provide operational flexibility for y-stream upgrades
* Z-stream auto-upgrades ensure security patches are applied promptly
* Node Pool upgrades remain fully customer-controlled
* Override mechanism ensures critical situations are not blocked by customer-configured delays
* EUS channel support enables extended stability between minor versions
* Progressive fleet rollout minimizes blast radius of problematic releases

### Negative

* Customers cannot defer y-stream upgrades indefinitely — less flexible than ROSA
* Z-stream upgrades bypass all delay controls, even for patches with behavioral changes
* Override conditions can force upgrades outside customer-preferred windows
* Node Pool version divergence is possible — exceeding N-3 skew puts the Node Pool out of support
* Dependency on Red Hat for edge resolution in EOL-with-no-edge scenarios

## Cross-Cutting Concerns

### Reliability:

* **Resiliency**: Multi-replica control planes minimize API availability impact during upgrades. Failed control plane upgrades trigger an SRE alert — no automatic rollback. Failed worker node replacements leave the old node in place.
* **Observability**: Customer notifications for upgrade lifecycle events. Platform-side metrics for upgrade success rates, duration, and remediation time.

### Security:

* Z-stream auto-upgrades ensure critical security patches are applied without delay
* Override mechanism ensures critical security situations are resolved regardless of customer-configured delays
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
