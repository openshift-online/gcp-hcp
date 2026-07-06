# Jira Initiative Template for GCP HCP

This template provides a standardized structure for creating Jira Initiatives. Initiatives represent internal/architectural work at Level 4 in the Jira hierarchy — the same level as Features, but for work that does **not** directly contribute to the product roadmap.

**Feature vs Initiative**: If customers will directly use the new capability → Feature. If it's internal improvement, architectural, or process work → Initiative. See [Jira Hierarchy](./jira-hierarchy.md#feature-vs-initiative-level-4) for details.

---

## Title

[Action Verb] + [Capability]

**Example**: "Establish Performance Testing Infrastructure for GCP HCP Services"

---

## Context

[Why this Initiative is needed. What internal gap, architectural limitation, or process improvement it addresses. How it supports strategic goals (e.g., reliability, scalability, team efficiency, technical debt reduction)]

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

## Technical Approach

[High-level architectural approach, key technologies/patterns to use, integration points, or note "TBD during Epic breakdown"]

---

## Internal Impact

[How this Initiative improves internal capabilities. Consider:]
- [Team efficiency gains (e.g., reduced manual effort, faster feedback loops)]
- [Reliability/stability improvements (e.g., reduced incident frequency, faster recovery)]
- [Scalability enablement (e.g., supports growth without proportional effort increase)]
- [Developer experience improvements (e.g., faster onboarding, better tooling)]

---

## Dependencies

- [Other Initiatives or Features that must complete first]
- [External teams or services (e.g., App-SRE, Hypershift upstream, CLM team)]
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
**Assignee**: [Directly Responsible Individual]

---

## Example Initiative

### Title
Establish Performance Testing Infrastructure for GCP HCP Services

### Context
The GCP HCP team currently lacks automated performance testing infrastructure. Load and stress tests are run manually and inconsistently, making it difficult to detect performance regressions before they reach production. This blocks our ability to confidently meet SLOs as the platform scales.

### Scope

#### What's Included
- Automated performance test framework integrated with CI/CD
- Load test scenarios for control plane provisioning and day-2 operations
- Performance baseline metrics and regression detection
- Results dashboard for tracking trends over time

#### What's NOT Included
- Customer-facing performance reporting (separate Feature)
- Chaos engineering / fault injection (separate Initiative)

### Technical Approach
Use k6 for load testing with custom GCP HCP scenarios. Integrate with Prow for automated execution on PR merges and nightly runs. Store results in Prometheus and visualize via Grafana dashboards. Define SLO-based thresholds for automated pass/fail gating.

### Internal Impact
- Catch performance regressions before production (currently detected via customer incidents)
- Reduce time spent on manual performance validation by ~80%
- Enable data-driven capacity planning for management cluster scaling
- Provide confidence for architectural changes (e.g., operator migrations)

### Dependencies
- Observability stack (for metrics storage and dashboards)
- CI/CD pipeline access (Prow integration)
- Dedicated test environment with representative cluster scale

### Acceptance Criteria
- [ ] Performance test suite runs automatically on every PR merge to main
- [ ] Baseline metrics established for control plane provisioning latency (p50, p95, p99)
- [ ] Regression detection alerts when latency exceeds baseline by >20%
- [ ] Results dashboard accessible to all team members with 90-day history

### Metadata
**Epic(s)**: TBD
**Priority**: Major
**Assignee**: TBD
