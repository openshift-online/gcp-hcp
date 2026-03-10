# Jira Feature Template for GCP HCP

This template provides a standardized structure for creating Jira Features during milestone and quarterly planning. Features represent high-level capabilities that span multiple sprints and typically decompose into multiple Epics and Stories.

---

## Title

[Action Verb] + [Capability]

**Example**: "Implement Distributed Tracing for GCP HCP Services"

---

## Context

[Why this Feature is needed and how it supports overall goals (e.g., Q1 milestone, Zero Operator principles, customer requirements)]

---

## Scope

### What's Included
- [Main capability/component 1]
- [Main capability/component 2]
- [Main capability/component 3]

### What's NOT Included
- [Out of scope item 1 to avoid confusion]
- [Out of scope item 2]

---

## Technical Approach (Optional)

[High-level approach if decided, key technologies/patterns to use, or note "TBD during Epic breakdown"]

---

## Dependencies

- [Other Features that must complete first]
- [External teams or services (e.g., CLM team, Hypershift upstream, App-SRE)]
- [Infrastructure or access requirements]
- [Required approvals or decisions]

---

## Acceptance Criteria

- [ ] [Specific, measurable outcome 1]
- [ ] [Specific, measurable outcome 2]
- [ ] [Specific, measurable outcome 3]

---

## Metadata

**Epic(s)**: [To be created during breakdown]
**Priority**: [Set during prioritization]
**Demo Critical**: Yes/No
**Size Estimate**: Small / Medium / Large
**DRI**: [Directly Responsible Individual]

---

## Example Feature

### Title
Implement Automated Remediation for Node Failures

### Context
Support Zero Operator principle by enabling automated healing of common failure scenarios without manual intervention. This is critical for the Q1 demo where we need to demonstrate the complete Break → Detect → Alert → Remediate flow.

### Scope

#### What's Included
- Auto-detection of unhealthy nodes
- Automated remediation workflow (drain, terminate, replace)
- Remediation action logging for audit trail
- Integration with alerting system

#### What's NOT Included
- Manual remediation tools/dashboards
- Remediation for database failures (separate Feature)

### Technical Approach
Use Kubernetes operators and custom controllers to watch node health metrics, trigger remediation workflows via GCP APIs, and emit structured logs for traceability.

### Dependencies
- Observability Feature (for node health detection)
- Alerting Feature (for notification integration)
- GCP IAM policies for node management APIs

### Acceptance Criteria
- [ ] System detects node failure within 2 minutes of occurrence
- [ ] Automated workflow replaces failed node without manual intervention
- [ ] Full remediation trace captured in structured logs with correlation IDs

### Metadata
**Epic(s)**: TBD
**Priority**: Blocker
**Demo Critical**: Yes
**Size Estimate**: Large
**DRI**: TBD
