# GCP HCP Feature & Initiative Ownership Playbook

This document defines what it means to own a Feature or Initiative in the GCP HCP project — the
responsibilities at each workflow stage, what must be true to move forward, and how ownership
relates to Epic-level execution.

Related docs: [jira-hierarchy.md](jira-hierarchy.md) · [definition-of-ready.md](definition-of-ready.md) · [definition-of-done.md](definition-of-done.md) · [jira-feature-template.md](jira-feature-template.md) · [jira-initiative-template.md](jira-initiative-template.md)

---

## What Is Ownership?

Owning a Feature or Initiative means **coordination and accountability, not sole execution**. The
owner ensures the work is well-defined, unblocked, and moving — not that they personally write all
the code.

Ownership shifts across the workflow:

| Stage | Who owns it |
|-------|-------------|
| New, Refinement | **Reporter** — created the issue, drives socialization and information gathering |
| Refinement (if offline work needed) | **Temporarily assigned person** — gathers information, then unassigns when done |
| To Do | **Nobody** — item is ready and waiting to be picked up |
| In Progress → Closed | **Assignee** — takes formal ownership when work begins and drives delivery through to close |

The To Do state is intentionally unassigned. It means "sufficiently defined, ready for someone to
start." The Assignee is set when work actually begins.

---

## Roles

Three roles should be named on every Feature and Initiative, per HP-wide guidance:

| Role | Who | When named | Jira field |
|------|-----|-----------|-----------|
| **Reporter** | Person who raised the issue | At creation | Reporter (system field) |
| **Assignee** | Engineer who drives delivery | When work begins (In Progress) | Assignee |
| **Architect** | Technical lead responsible for the architectural approach | During Refinement | Architect field |
| **Product Manager** | Owns the product perspective — the "what and why" | During Refinement | Product Manager field |

> **Note:** If the GCP Jira project does not yet have the Architect or Product Manager fields,
> submit a request to the Jira Admin team to have them added. These fields are part of the
> HP-wide ownership model introduced in May 2026.

### Field Responsibilities

Setting Priority and Fix Version depends on whether the work is customer-facing or engineering-led:

| Field | Feature (customer-facing) | Initiative (internal/engineering) |
|-------|---------------------------|-----------------------------------|
| **Priority** | Product Manager | Architect |
| **Fix Version** | Product Manager | Architect |

> **Note:** Fix Versions set on Features and Initiatives automatically cascade to child Epics via
> Jira automation. Set the Fix Version at the Feature/Initiative level during Refinement.
>
> **Setting Fix Version requires conversation about capacity and throughput** — discuss in Feature/Initiative
> Refinement sessions or Slack. Don't set Fix Versions unilaterally without team awareness.
>
> **When Fix Version changes:** Always have a conversation before adding or removing items from a Fix Version.
> Ensure mutual awareness of scope changes (Slack for PM visibility, or raise in refinement sessions).
> Add a comment explaining the scope/timeline shift for the audit trail.

---

## Feature vs. Initiative

Both are Level 4 in the [hierarchy](jira-hierarchy.md) and follow the same workflow. The
distinction is audience and outcome:

| | Feature | Initiative |
|--|---------|-----------|
| **What it is** | Customer-facing capability on the product roadmap | Internal/architectural work — engineering enablement, reliability, process improvement |
| **Template** | [jira-feature-template.md](jira-feature-template.md) | [jira-initiative-template.md](jira-initiative-template.md) |
| **Demo Critical** | Yes — must be marked Yes or No | N/A — internal work, no demo requirement |
| **Internal Impact section** | Not required | Required — team efficiency, reliability, scalability, DX gains |

When in doubt: if a customer would notice or benefit from it, it's a Feature. If it's invisible
to customers but makes the team or system better, it's an Initiative.

---

## Workflow Stages

### New

**Definition:** Idea raised, not yet evaluated or scoped.

**Who leads:** Reporter

**Actions:**
- Create the issue using the [Feature](jira-feature-template.md) or
  [Initiative](jira-initiative-template.md) template — title, context, rough scope, and initial
  acceptance criteria
- Socialize with the team (Slack or in the Feature/Initiative Refinement meeting)
- If connected to a strategic Outcome, link to the parent HPSTRAT issue

**To move forward, ensure:**
- [ ] Summary follows [Action Verb] + [Capability] format
- [ ] Description provides enough context for the team to evaluate the work
- [ ] Reporter has been identified

---

### Refinement

**Definition:** Work is being scoped and discussed. The team is determining whether and how to
proceed.

**Who leads:** Reporter — or someone temporarily assigned to fill information gaps (see note below)

**Actions:**
- Present to the team in the Feature/Initiative Refinement meeting (held every three weeks — see [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835/Team+Ceremonies+Calendar))
- Drive scoping discussions: what's in scope, what's explicitly out of scope
- Clarify acceptance criteria — specific, testable outcomes
- Identify at least one Epic that represents a first chunk of engineering breakdown
- Confirm team buy-in and set Priority
- Name an **Architect** in the Jira Architect field (the technical lead for this work)
- Name a **Product Manager** in the Jira Product Manager field
- For Features: determine Demo Critical status (Yes/No)
- For Initiatives: fill in the Internal Impact section

> **Note on assignees:** There is no formal assignee requirement during Refinement. If the item
> needs significant offline work (research, stakeholder outreach, architectural investigation),
> assign whoever will own that work — but unassign the item when it moves to To Do unless that
> person will also be doing the implementation. The goal is: To Do = unassigned and ready.

**To move forward (Definition of Ready — trial adoption), ensure:**
- [ ] Context and problem statement are clear
- [ ] Scope is defined: what's included, what's not
- [ ] Acceptance Criteria are defined and specific
- [ ] Priority set (not Undefined) — see [Priority Scheme](https://github.com/openshift-online/ai-helpers/blob/main/plugins/jira/reference/gcp-hcp.md)
- [ ] At least one Epic identified for breakdown
- [ ] Dependencies documented (blocking issues linked)
- [ ] Architect named in Jira
- [ ] Product Manager named in Jira
- [ ] Team agrees to proceed
- [ ] **Feature only:** Demo Critical set (Yes/No)
- [ ] **Initiative only:** Internal Impact section completed

---

### To Do

**Definition:** Sufficiently defined and ready to start. Not yet assigned or in progress.

**Who leads:** Nobody — item is intentionally **unassigned**

**Actions:**
- Confirm at least one Epic is in Refinement or To Do state
- Ensure no blocking dependencies remain unresolved
- Item sits here until someone picks it up to begin work

> Items in To Do should not be assigned. When an engineer or PM is ready to begin, they take
> ownership at that point and move the item to In Progress.

**To move forward, ensure:**
- [ ] Item is unassigned
- [ ] At least one child Epic is in Refinement or To Do
- [ ] No unresolved blocking dependencies

---

### In Progress

**Definition:** Active execution is underway.

**Who leads:** Assignee

**Actions:**
- Assignee takes formal ownership when picking up the work
- Coordinate with the Architect and Epic owners to align on approach, surface dependencies, and agree on first steps
- Facilitate regular delivery check-ins
- Clear blockers — escalate if needed
- Post a brief status **Jira comment** at least every two weeks
- Monitor child Epic progress; keep statuses current
- Drive demos where applicable

**To remain in good standing, ensure:**
- [ ] Assignee is set
- [ ] Jira comment posted within the last 14 days
- [ ] Child Epic statuses are current
- [ ] Target end date is set (recommended — use Epic target end dates as the source of truth if
  no Feature-level target applies)
- [ ] No unaddressed blockers

---

### Review

**Definition:** Engineering work is complete. Validating that all acceptance criteria are met.

**Who leads:** Assignee

**Actions:**
- Confirm all Acceptance Criteria have been satisfied
- Coordinate a final demo or review session with stakeholders
- Resolve any outstanding issues before closing
- For Features with Demo Critical = Yes: ensure the demo has been presented or recorded

**To move forward, ensure:**
- [ ] All child Epics are in Review or Closed
- [ ] All Acceptance Criteria validated
- [ ] Demo completed (Features with Demo Critical = Yes)
- [ ] No open questions or blockers

---

### Closed

**Definition:** Work is complete, validated, and the issue is formally closed.

**Who leads:** Assignee

**Actions:**
- Confirm all child Epics and Stories are Closed
- Set the Resolution field (e.g., Done, Obsolete, Duplicate, Won't Do)
- Write a final Jira comment summarizing the outcome
- Facilitate a closing retrospective if the work was significant or surfaced process learnings
- Announce the delivery to stakeholders as appropriate

**To close, ensure:**
- [ ] All child Epics are Closed
- [ ] Resolution field is set
- [ ] Final Jira comment added

---

## Relationship to Epic Ownership

Features and Initiatives decompose into Epics. Ownership works at two levels:

| | Feature/Initiative Assignee | Epic Assignee |
|--|----------------------------|--------------|
| **Owns** | The outcome — why this matters, whether the ACs are met | The execution — how the work gets done within their Epic |
| **Horizon** | Multi-milestone; coordinates across multiple Epics | Drives a focused slice of work to done |
| **Primary concern** | Is the work moving? Are blockers cleared? Are stakeholders informed? | Is the Epic broken down well? Are Stories in progress? |
| **Escalates to** | Engineering leadership, PM | Feature/Initiative Assignee |

The Feature/Initiative Assignee may or may not own any Epics directly. They coordinate across Epic
Assignees and are accountable for the outcome, not a substitute for them.

---

## Quick Reference

| Status | Assignee | Jira comment required | Key exit condition |
|--------|----------|----------------------|-------------------|
| New | Optional | No | Team agrees to scope it |
| Refinement | Optional (temporary) | No | DoR met; team agrees to proceed |
| To Do | **None** | No | ≥1 Epic ready; unassigned |
| In Progress | **Required** | Every 14 days | No unaddressed blockers |
| Review | Required | No | All ACs validated; demo done (if Demo Critical) |
| Closed | Required | Final comment | All children closed; Resolution set |

For field-level Jira conventions (priority values, custom field IDs, component names), see the
[GCP HCP Jira conventions reference](https://github.com/openshift-online/ai-helpers/blob/main/plugins/jira/reference/gcp-hcp.md).

For Story/Epic/Task/Bug criteria, see [definition-of-ready.md](definition-of-ready.md) and
[definition-of-done.md](definition-of-done.md).
