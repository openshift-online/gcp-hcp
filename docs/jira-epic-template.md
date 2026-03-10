# Jira Epic Template for GCP HCP

This template provides a standardized structure for creating Jira Epics. Epics represent a cohesive chunk of work within a Feature that can typically be completed in 1-2 sprints and decomposes into multiple Stories.

**Hierarchy**: Feature → **Epic** → Story

---

## Title

[Action Verb] + [Specific Capability or Component]

**Examples**:
- "Establish e2e test suite for Hypershift on GKE in Prow"
- "Audit and set resource requests/limits for all Management Cluster components"
- "Cloud Network Config Controller: GCP Workload Identity Federation"

---

## Use Case / Context

[Brief description of why this Epic is needed, what problem it solves, or what capability it enables]

---

## Current State

[Describe the current state, limitations, or gaps that this Epic addresses]

**Optional**: Include technical details about current implementation, blockers, or constraints

---

## Desired State / Goal

[Describe what will be true when this Epic is complete]

---

## Scope

**This Epic covers**:
- [Component or capability 1]
- [Component or capability 2]
- [Component or capability 3]

**Out of Scope** (if applicable):
- [What's NOT included to avoid confusion]

---

## Technical Details (Optional)

[Include relevant technical information such as:
- Architecture changes needed
- Technologies or tools to use
- Integration points
- Configuration requirements
- Standards or patterns to follow]

---

## Dependencies

- [ ] [Blocking Epic or Story from another team]
- [ ] [External dependency or approval needed]
- [ ] [Infrastructure or access requirement]

---

## Story Breakdown Checklist

- [ ] Stories created for all work identified in scope
- [ ] Each Story follows the [Jira Story Template](./jira-story-template.md)
- [ ] Story sequencing/priorities established
- [ ] Dependencies between Stories identified

---

## Acceptance Criteria

- [ ] [Specific, measurable outcome 1]
- [ ] [Specific, measurable outcome 2]
- [ ] [Specific, measurable outcome 3]

---

## Metadata

**Feature**: [Parent Feature, if applicable]
**Assignee**: [DRI for this Epic]
**Priority**: [Blocker/Critical/Major/Normal/Minor]
**Sprint Target**: [Target sprint(s) or quarter]
**Size Estimate**: Small / Medium / Large

---

## Example Epic

### Title
Audit and set resource requests/limits for all Management Cluster components

### Use Case / Context
Ensure all workloads running on Management Clusters (GKE Autopilot) have explicit resource requests and limits set. GKE Autopilot requires explicit resource requests to size pods appropriately.

### Current State
Many Management Cluster components deployed by Hypershift and related operators do not have resource requests/limits configured. This causes pod scheduling issues on GKE Autopilot and can lead to resource contention.

### Desired State / Goal
All Management Cluster workloads have appropriate resource requests and limits configured, enabling proper scheduling on GKE Autopilot and preventing resource contention issues.

### Scope

**This Epic covers**:
1. Auditing all MC workloads to identify missing requests/limits
2. Remediation Stories (one per component/operator) to add the missing configurations
3. E2E test to verify all MC workloads have proper resource configurations

**Out of Scope**:
- Hosted Cluster workload resource settings (different Epic)
- Performance tuning or optimization of existing settings

### Technical Details

**Components to audit**:
- Hypershift operator deployments
- OCP component operators (Ingress, Storage, Network, etc.)
- Monitoring and observability stack components
- Custom controllers and operators

**Standards to follow**:
- Requests should reflect typical usage patterns
- Limits should allow for burst capacity without OOMKill
- Follow OCP resource management best practices

### Dependencies
- [ ] Access to representative Management Clusters for baseline measurements
- [ ] Coordination with upstream Hypershift team for operator changes

### Story Breakdown Checklist
- [ ] Story: Audit all MC workloads and document current state
- [ ] Story: Set requests/limits for Hypershift operator
- [ ] Story: Set requests/limits for OCP component operators
- [ ] Story: Set requests/limits for monitoring stack
- [ ] Story: E2E test to verify all MC workloads have resource requests/limits
- [ ] Dependencies between Stories identified

### Acceptance Criteria
- [ ] 100% of Management Cluster workloads have resource requests defined
- [ ] 100% of Management Cluster workloads have resource limits defined
- [ ] E2E test validates resource configurations on every CI run
- [ ] Documentation updated with resource sizing guidance

### Metadata
**Feature**: Hypershift GCP Support
**Assignee**: TBD
**Priority**: Critical
**Sprint Target**: Q1 2026
**Size Estimate**: Medium
