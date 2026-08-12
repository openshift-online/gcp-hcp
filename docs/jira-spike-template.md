# Jira Spike Template for GCP HCP

Use this template to define a time-boxed investigation that resolves a specific uncertainty before committing to implementation. Spikes answer a research question so the team can make an informed decision.

**GCP convention:** Spikes are not story-pointed. Effort is bounded by the time box instead.

---

## When to Use This Template

**Use a Spike for:**
- Resolving technical uncertainty before implementation can begin
- Evaluating competing approaches or tools before committing
- Proof-of-concept work to confirm feasibility
- Investigation where the outcome is unknown (and unknown how long it will take)

**Use a Story instead for:**
- User-facing features or capabilities where the implementation approach is known
- Work that can be fully scoped into acceptance criteria upfront

**Use a Task instead for:**
- Planned work with a known approach that doesn't need a user story format
- Post-meeting follow-ups, process work, or discrete deliverables

---

## Research Question / Goal

[What specific question or uncertainty does this spike address? What decision will the findings inform?]

## Context

[Why is this investigation needed now? What is blocked or uncertain without it? Link any relevant tickets, incidents, or design discussions.]

## Investigation Approach

[How will you investigate? Examples: proof-of-concept implementation, benchmark comparison, API exploration, reading upstream docs, vendor evaluation.]

## Time Box

**Maximum time allocated:** [e.g., 1 day / 2 days]

> Time boxes for GCP HCP spikes are typically 1-2 days. If the research question cannot be answered in that time, narrow the scope or split into multiple spikes.

## Success Criteria

[What findings or decisions mark this spike as complete? Success criteria for spikes are outcome-oriented, not implementation-oriented.]

Examples:
- Decision documented: approach A vs B selected, with rationale recorded in design doc
- Proof-of-concept confirms feasibility; follow-up stories created
- Benchmark results show metric X meets the team's SLO threshold

## Findings Documentation Format

[Where will findings be recorded upon completion? e.g., PR comment, design doc, ADR, Confluence page, follow-up ticket]

## Follow-up Actions

- [ ] [Backlog item or decision record to create upon completion]
- [ ] [Additional follow-up if applicable]

---

## Acceptance Criteria

> Definition of Done: see [definition-of-done.md](definition-of-done.md) — Spike DoD section.

- [ ] Spike findings documented in the agreed location (design doc, ADR, or ticket comment)
- [ ] Decision made and documented (which approach to take, or whether to proceed)
- [ ] Resulting backlog items created (implementation stories, follow-up spikes, or explicit decision to defer)

---

## Worked Example

**Summary**: `Spike: evaluate Workload Identity Federation vs. service account key rotation for HCP node authentication`

**Research Question / Goal**: Determine whether GCP Workload Identity Federation (WIF) can replace service account key rotation for authenticating HCP node pools to GCP APIs, and whether WIF is feasible within the current cluster architecture.

**Context**: The team is planning GCP-NNN (implement secure node authentication), but the approach is unclear. Service account key rotation is the current path, but WIF would eliminate long-lived credentials. A spike is needed before scoping implementation stories.

**Investigation Approach**:
1. Review GCP WIF documentation for GKE-hosted workloads
2. Prototype WIF configuration on a dev cluster
3. Test whether HyperShift NodePool controller can acquire GCP credentials via WIF
4. Compare operational complexity of WIF vs. key rotation

**Time Box**: 2 days

**Success Criteria**: Decision documented — WIF feasible (and follow-up implementation story created) or infeasible (and key rotation approach confirmed with rationale).

**Findings Documentation Format**: Comment on this ticket + entry in `docs/architecture-decisions/` documenting the decision and rationale for the chosen approach.

**Follow-up Actions**:
- [ ] Create implementation story for chosen authentication approach
- [ ] Add decision record to `docs/architecture-decisions/`

**Acceptance Criteria**:
- [ ] Findings documented in this ticket (comment) and in `docs/architecture-decisions/`
- [ ] Decision recorded: WIF vs. key rotation, with rationale
- [ ] Follow-up story created for implementation approach
