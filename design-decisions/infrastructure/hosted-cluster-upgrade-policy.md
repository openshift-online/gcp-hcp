# Mandatory Platform-Managed Upgrades for Hosted Cluster Control Planes

***Scope***: GCP-HCP

**Date**: 2026-06-29

## Decision

Adopt mandatory platform-managed upgrades for hosted cluster control planes following the GKE model: control plane upgrades are automatic and cannot be disabled. Customers control timing of y-stream upgrades through maintenance windows, maintenance exclusions, and channel selection. Z-stream upgrades are fully automatic and do not respect delay controls.

Node Pool upgrades are entirely customer-triggered — manual or scheduled — and always target the current control plane version. The platform does not automatically upgrade Node Pools.

## Context

- **Problem Statement**: GCP HCP needs a defined upgrade lifecycle policy for hosted OCP clusters. The policy must balance platform safety (keeping control planes on supported, secure versions) with customer operational needs (avoiding surprise disruptions during business-critical periods). Without a clear model, clusters risk running unsupported versions, accumulating security debt, or surprising customers with poorly timed upgrades.

- **Constraints**:
  - Must be compatible with HyperShift's upgrade mechanics (CVO, Cincinnati upgrade graph, sequential minor version steps)
  - Must enforce version skew constraints between control plane and Node Pools (N-3 minor versions)
  - Must support EUS-to-EUS upgrade paths (workers skipping odd minor versions)
  - Must provide customer controls that map to familiar GKE concepts where possible
  - Cincinnati is the canonical source for OCP release versions and upgrade paths (see [Adopt Cincinnati for Version Resolution](../governance/adopt-cincinnati-for-version-resolution.md))
  - Control plane upgrades are the platform's responsibility as a managed service — customers should not be exposed to internal upgrade plumbing

- **Assumptions**:
  - HyperShift handles the actual control plane rollout mechanics (etcd, kube-apiserver, kube-controller-manager, openshift-apiserver, CVO)
  - Cincinnati upgrade graph is authoritative for determining valid upgrade edges
  - Progressive fleet rollout will be used — not all clusters upgrade simultaneously
  - GKE's upgrade model is the target customer experience for control planes

## Alternatives Considered

1. **GKE model — mandatory platform-managed control plane upgrades with customer timing controls (chosen)**: Control plane upgrades are automatic and cannot be disabled. Customers control *when* y-stream upgrades happen via maintenance windows and exclusions, and *which channel* they track. Z-stream upgrades are fully automatic. Node Pool upgrades are customer-triggered.

2. **ROSA model — customer-triggered upgrades with EOL enforcement**: Upgrades are not enforced until end-of-life. Customers schedule and trigger all upgrades manually. If a cluster reaches EOL, the platform force-upgrades it.

3. **Fully customer-managed upgrades**: The platform publishes available versions but never initiates upgrades. Customers are responsible for all upgrade decisions and execution.

## Decision Rationale

* **Justification**: The GKE model (Alternative 1) provides the best balance between platform safety and customer control. Mandatory control plane upgrades ensure clusters stay on supported, secure versions — eliminating the class of support issues where customers run EOL versions for months. Customer timing controls (maintenance windows, exclusions, channel selection) give customers operational flexibility for y-stream upgrades without allowing indefinite deferral. Z-stream patches are low-risk and security-critical, justifying fully automatic application. Keeping Node Pool upgrades customer-triggered gives customers full control over workload-impacting changes.

* **Evidence**: GKE's upgrade model is proven at scale and well-understood by the target customer base (GCP-native users). ROSA's experience shows that optional upgrades lead to long-tail version sprawl — some customers stay on unsupported versions for extended periods, creating security exposure and support complexity.

* **Comparison**:
  - Alternative 2 (ROSA model) was rejected because it allows customers to defer upgrades indefinitely until EOL, creating security exposure and version sprawl. The EOL force-upgrade mechanism is a blunt instrument — customers on EOL already have degraded support, and force upgrades at that point are disruptive.
  - Alternative 3 (fully customer-managed) was rejected because it transfers all upgrade responsibility to customers, creates a long tail of unsupported versions, and increases support burden. No major managed Kubernetes platform uses this model.

## Upgrade Model

### Available Channels

- **Fast**: GA releases appear immediately after Red Hat declares them GA. Fully supported. If a regression is found in a fast release, it gets the same fix priority as stable. The only difference is timing, not support level.
- **Stable**: Same GA releases as fast, but after a soak period on the fast channel. This delay gives Red Hat more time to discover regressions before the release reaches stable customers. Same support level as fast — just delayed for extra confidence. Default channel.
- **EUS**: Available on even-numbered minor versions only (4.14, 4.16, 4.18, etc.). Extends Full and Maintenance support phases to 18 months. Enables EUS-to-EUS updates where workers skip the intermediate odd version.

### Version Skew Policy

The platform enforces version skew constraints between the control plane and Node Pools. These constraints are what make EUS-to-EUS updates possible (N-3 allows workers to remain on the even version while the control plane transits through the odd version).

| Constraint | Rule | Enforcement |
|---|---|---|
| **Worker-to-control-plane skew** | Workers must not be more than **N-3 minor versions** behind the control plane | Platform blocks control plane upgrades that would exceed this skew; auto-upgrades workers approaching the limit |
| **Worker version ceiling** | Workers cannot run a version **newer** than the control plane | Platform blocks worker upgrades beyond the control plane version |
| **Upgrade order** | Control plane upgrades **before** workers | Platform enforces this ordering — worker upgrades are scheduled only after control plane upgrade completes successfully |
| **Minor version steps** | Minor version upgrades are sequential (e.g., 4.22 → 4.23 → 4.24, not 4.22 → 4.24) | Platform enforces single-step minor upgrades via Cincinnati upgrade graph |

### Control Plane Upgrades

Control plane upgrades are **mandatory and platform-managed**. The platform keeps hosted control planes on supported, secure versions. Customers cannot disable automatic upgrades but can manually upgrade ahead of the automatic schedule.

**Upgrade triggers:**
- Automated (mandatory): new default version set on the channel
- Manual (optional): customer initiates upgrade ahead of the automatic schedule

**Delay controls:**
- Y-stream (minor) upgrades respect maintenance windows and maintenance exclusions
- Z-stream (patch) upgrades are fully automatic and do not respect delay controls

**How it works:**
1. HyperShift's CVO queries Cincinnati via the HostedCluster's `spec.channel` and populates `status.version.availableUpdates` on the HostedCluster CR
2. The platform reads available updates from the HostedCluster CR status and evaluates upgrade eligibility (channel target version, maintenance window, exclusion window)
3. When an upgrade is elected, the platform updates `spec.release.version` on the cluster spec, triggering the version-resolution adapter to resolve the new release image via Cincinnati
4. HyperShift orchestrates the actual control plane component rollout (etcd, kube-apiserver, kube-controller-manager, openshift-apiserver, CVO)
5. New target versions are rolled out progressively across the fleet — not all clusters upgrade simultaneously

### Node Pool Upgrades

Node Pool upgrades are **entirely customer-triggered**. The platform does not automatically upgrade Node Pools. Customers initiate upgrades manually or schedule them, and the target version is always the current control plane version — there is no version selection for Node Pools.

**Upgrade triggers:**
- Manual: customer initiates upgrade to match the current control plane version
- Scheduled: one-off scheduled upgrade to match the control plane version

**Upgrade strategy — Replace (default):** Creates new machine instances with the target version and removes old ones in a rolling fashion. HyperShift uses CAPI MachineDeployments — when the NodePool version changes, CAPG creates new GCPMachine resources (new GCE instances), drains pods from old nodes, then deletes them.

HyperShift Operator knobs:
- **maxSurge**: how many extra nodes to create during the rollout (more = faster but uses more quota)
- **maxUnavailable**: how many nodes can be down simultaneously
- **nodeDrainTimeout**: force-removes stuck nodes after a timeout
- **nodeVolumeDetachTimeout**: force-detaches volumes after a timeout

**Future direction — Karpenter:** With Karpenter, the upgrade execution model shifts from explicit CAPI MachineDeployment rollouts to drift-based node replacement. When the customer updates the NodePool version, Karpenter detects drift and replaces nodes through its own disruption controls (consolidation policies, disruption budgets). The upgrade policy (customer-triggered, always targets control plane version) remains the same — only the execution mechanism changes. Karpenter's natural node rotation also provides a more graceful upgrade experience, as nodes are replaced based on workload demand rather than a rigid rollout sequence.

## Customer Controls for Control Plane Upgrades

### Release Channel Selection

Customers select a release channel that determines which versions are offered for upgrade. Channels map to Cincinnati channel groups.

- Channel is set at cluster creation (default: `stable`) and can be changed at any time
- Changing channel might trigger an upgrade if the **default** version in the new channel is newer than the current version

### Maintenance Windows

Customers define recurring time windows when automatic y-stream upgrades are allowed to proceed. This controls *when* upgrades happen, not *which* upgrades happen.

**Properties:**
- **Recurrence**: [RFC 5545](https://datatracker.ietf.org/doc/html/rfc5545) recurrence rules so that a single window can express complex schedules like "weekdays 2-6 AM UTC" or "Saturdays and Sundays only"
- **Duration**: Minimum 4 hours (TBD from Perf/Scale tests) to allow upgrades to complete
- **Default**: If no maintenance window is set, the platform may upgrade at any time
- **Scope**: Applies to control plane y-stream upgrades only (z-stream upgrades are fully automatic)

### Maintenance Exclusions

Customer-defined blackout periods during which no automatic y-stream upgrades are applied, even if a maintenance window is open.

- Up to 3 maintenance exclusions (aligned with GKE)
- Maximum duration 30 days
- Must leave at least 48 hours of maintenance availability in any rolling 32-day window (aligned with GKE)

**Override conditions** — maintenance windows and exclusions are respected *except for*:
- Z-stream upgrades (always automatic)
- Version is approaching **EOL**
- Workers are approaching the **version skew limit** (N-3)
- The cluster version is **incompatible with a required platform component** update

### Manual Upgrades

Customers can initiate a control plane upgrade to a specific target version without waiting for the automatic schedule.

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
| Delay override pending | Advance notification with reason and override timeline |

## Customer Controls for Node Pool Upgrades

### Manual Upgrades

Customers can initiate a Node Pool upgrade to match the current control plane version.

**Capabilities:**
- **Initiate upgrade**: Start an upgrade to match the current control plane version
- **View upgrade status**: Monitor progress of an in-progress upgrade

### Scheduled Upgrades

Customers can schedule a one-off Node Pool upgrade to match the control plane version at a specified time.

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

This is an internal platform concern — control plane upgrades are the platform's responsibility as a managed service. The customer is not exposed to the mechanics of edge resolution.

## Consequences

### Positive

* Control planes stay on supported, secure versions — eliminates long-tail version sprawl seen in customer-triggered models
* Familiar model for GCP-native customers who already use GKE's upgrade lifecycle
* Customer timing controls (maintenance windows, exclusions, channels) provide operational flexibility for y-stream upgrades
* Z-stream auto-upgrades ensure security patches are applied promptly without customer action
* Node Pool upgrades remain fully customer-controlled, avoiding unexpected workload disruption
* Override mechanism ensures critical situations (EOL, version skew, platform compatibility) are not blocked by customer-configured delays
* EUS channel support enables customers who need extended stability between minor versions
* Progressive fleet rollout minimizes blast radius of problematic releases
* EOL-with-no-edge handling is transparent to customers — the platform owns the resolution

### Negative

* Customers lose the ability to defer y-stream upgrades indefinitely — may be perceived as less flexible than ROSA
* Z-stream upgrades bypassing all delay controls removes customer control over patch timing, even for patches that could cause behavioral changes
* Override conditions can force upgrades outside customer-preferred windows — requires clear advance communication
* Node Pool version divergence from the control plane is possible if customers delay upgrades — approaching skew limits triggers platform intervention
* Dependency on Red Hat for edge resolution in EOL-with-no-edge scenarios — resolution timeline is outside platform control

## Cross-Cutting Concerns

### Reliability:

* **Resiliency**: Control plane upgrades use HyperShift's rolling update mechanism — multi-replica control planes minimize API availability impact (brief, seconds-level disruptions during pod replacement). Failed control plane upgrades trigger automatic rollback to the previous version. Failed worker node replacements leave the old node in place — no workload disruption from a failed worker upgrade.
* **Observability**: Customer notifications for upgrade lifecycle events (scheduled, started, completed, failed, delay override pending). Platform-side metrics for upgrade success rates, duration, and rollback frequency across the fleet.

### Security:

* Z-stream auto-upgrades ensure critical security patches are applied without waiting for customer action or maintenance windows
* Override mechanism ensures critical security situations are resolved regardless of customer-configured delays
* Mandatory control plane upgrades prevent clusters from accumulating security debt on unsupported versions
* Version skew enforcement prevents configurations where workers run incompatible versions

### Performance:

* Control plane upgrades complete in minutes with brief API unavailability (seconds)
* Worker node upgrades depend on node count and strategy — Replace strategy uses maxSurge/maxUnavailable knobs to balance speed vs. resource consumption
* PDB-aware draining ensures workload disruption is minimized during worker upgrades

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
