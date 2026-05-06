---
name: Risk Tracking Process
description: How the GCP HCP team identifies, assesses, tracks, and mitigates project risks using Jira Risk issue types.
---

# Risk Tracking Process

***Scope***: GCP-HCP

**Date**: 2026-04-17

This document defines how the GCP HCP team identifies, assesses, tracks, and mitigates project risks.

## Tooling

Use the **Risk issue type** in the GCP Jira project.

### Fields

**Required fields:**

| Field | Jira Field ID | Type | Purpose |
|-------|---------------|------|---------|
| Risk Probability | customfield_10642 | Dropdown | How likely is this risk to occur (1-5) |
| Risk Impact | customfield_10842 | Dropdown | Severity if the risk materializes (1-5) |
| Risk Score | customfield_10976 | Number | Calculated severity (Probability x Impact) |

**Optional fields** (available on the screen, use when they add value):

| Field | Jira Field ID | Type | Purpose |
|-------|---------------|------|---------|
| Risk Proximity | customfield_10645 | Dropdown | How soon the risk could materialize |
| Risk Response | customfield_10846 | Dropdown | Response strategy (Avoid, Mitigate, Transfer, Accept) |
| Risk Category | customfield_10679 | Dropdown | Classification (Technical, Schedule, Resource, etc.) |
| Risk Score Assessment | customfield_10974 | Paragraph | Qualitative risk assessment narrative |

Standard Jira fields are also used:

- **Summary** -- one-line risk statement
- **Description** -- detailed risk context, background, and mitigation/contingency plan
- **Assignee** -- risk owner responsible for monitoring and response
- **Reporter** -- person who identified the risk
- **Components** -- GCP component area the risk relates to

### Workflow

The Risk issue type uses the following workflow statuses:

```text
New  -->  Refinement  -->  To Do  -->  In Progress  -->  Review  -->  Closed
```

- **New** -- risk has been raised but not yet assessed
- **Refinement** -- risk is being evaluated for probability, impact, and response strategy
- **To Do** -- risk has been assessed and is ready for mitigation work to begin
- **In Progress** -- active mitigation or response plan is underway
- **Review** -- mitigation actions are complete; risk is being validated as resolved
- **Closed** -- risk has been resolved, accepted, or is no longer relevant

### Board

**TODO:** Create a dedicated **Risk Board** (Kanban) filtered to `issuetype = Risk AND project = GCP` to provide a view of all team risks and their current status.

Useful JQL queries for risk management:

- **All open risks**: `issuetype = Risk AND project = GCP AND status != Closed`
- **High-severity risks**: `issuetype = Risk AND project = GCP AND "Risk Score" >= 10`
- **Risks needing owners**: `issuetype = Risk AND project = GCP AND assignee = EMPTY AND status != Closed`

## Qualifying a Risk

Before creating a Risk issue, confirm the concern actually qualifies as a risk. A risk is an **uncertain future event** that could negatively affect the project if it occurs. Many concerns that feel like risks are actually planned work, known tech debt, or open design questions — these belong as stories, epics, or tasks instead.

**A concern qualifies as a risk when:**

- There is **genuine uncertainty** about whether or how it will happen. If you already know the problem exists and what the fix is, it's a backlog item — not a risk.
- It describes something that **might happen**, not something that has already happened or is a known state. "Config Connector uses `roles/editor`" is a fact (file a story). "A compromised credential could grant cross-region access before cluster separation is complete" is a risk.
- The **trigger is outside the team's full control** — external dependencies, upstream projects, vendor decisions, or timing uncertainties.
- There is a **credible scenario** where the concern materializes before the team can address it. Proximity to a milestone matters.

**A concern does NOT qualify as a risk when:**

- **The condition has already been resolved.** If the architecture or implementation has evolved past the concern, there is nothing to track.
- **It's too vague to assess.** A risk needs enough specificity to evaluate probability, impact, and a response plan. If it can't clear that bar, it isn't ready to be a Risk issue. Refine it first.
- **It's planned work with no time pressure.** "We haven't built quota monitoring yet" is not a risk when production is months away — it's a backlog item. It only becomes a risk if there's a credible scenario of reaching a milestone without the gap being closed.
- **It's an open design question with active engagement.** If there's a structured process underway to resolve the question (vendor engagement, spike, design review), track it there. A risk only emerges if the resolution process itself is at risk of failing or not completing in time.
- **It's work that is already underway with a known path.** If active work covers the concern (e.g., a feature in progress, a PAM entitlement being implemented), the work stream itself is the tracking mechanism.

## Process

### 1. Identify

Anyone on the team can raise a risk at any time by creating a Risk issue in the GCP project. Include:

- A clear, specific summary (e.g., "Cincinnati API outage could block new cluster creation due to version resolution dependency")
- A description covering: what could go wrong, what triggers it, and what would be affected
- Set status to **New**

Good times to identify risks:
- Sprint planning
- Design reviews and architecture discussions
- Retrospectives
- Incident postmortems
- External dependency changes

### 2. Assess

The risk owner (or the team during grooming) evaluates the risk:

1. Set **Risk Probability** and **Risk Impact** using the scoring criteria below
2. Calculate and set **Risk Score** (Probability x Impact)
3. Write a mitigation/contingency plan in the **Description** field
4. Optionally set **Risk Response**, **Risk Proximity**, and **Risk Category** if they add clarity
5. Link the risk to any related epics, stories, or features using issue links
6. Transition status to **Refinement** or directly to **In Progress** if mitigation is already underway

#### Probability (1-5)

Assign the score where any one of the criteria applies:

| Score | Level | Criteria |
|-------|-------|----------|
| 1 | Rare | Theoretical; no precedent in this or similar projects |
| 2 | Unlikely | Has happened elsewhere but conditions aren't present here |
| 3 | Moderate | Has happened before or some contributing factors exist today |
| 4 | Likely | Contributing factors are active; expected without changes |
| 5 | Very Likely | Already showing early signs; a matter of when, not if |

#### Impact (1-5)

Assign the score where any one of the criteria applies:

| Score | Level | Criteria |
|-------|-------|----------|
| 1 | Annoyance | Cosmetic or documentation issue, OR no effect on delivery, service, or customers, OR absorbed within normal workflow without re-planning |
| 2 | Low | Small delay or workaround required, OR limited to a single team or component, OR no customer-visible effect, OR minimal rework (days, not weeks) |
| 3 | Moderate | Noticeable delay to a milestone, OR partial service degradation, OR affects multiple components or teams, OR requires engineering intervention or re-planning, OR SLO breach possible |
| 4 | Medium | Significant schedule slip (weeks), OR service outage or data integrity issue, OR blocks dependent work streams, OR affects customers directly, OR reputational or compliance risk |
| 5 | High | Project delivery blocked, OR complete service unavailability or data loss, OR security or compliance breach, OR affects all customers or the entire project timeline, OR regulatory or contractual consequences |

### 3. Track

- **Grooming**: Triage newly identified risks (status = New). Assign owners. Assess probability and impact.
- **Sprint reviews**: Review the Risk Board. Update status and scores for any risks that have changed.
- **Monthly**: Review all open risks with the full team. Close risks that are no longer relevant. Identify new risks.

### 4. Respond

For risks in **In Progress** status:

- Create linked stories or tasks for specific mitigation actions
- Track mitigation progress through those linked issues
- Update the mitigation/contingency section in the **Description** with progress
- Reassess probability and impact as mitigation actions complete
- When mitigation is complete, transition to **Review**

### 5. Close

Transition a risk to **Closed** when:

- The mitigation plan has been fully executed and the risk is resolved
- The risk is accepted with documented rationale (note the decision in the description)
- The risk is no longer applicable (document why)
- The risk materialized and has been handled through incident response

Add a comment explaining the closure rationale.

## Escalation

Escalate a risk to leadership when:

- Risk Score is >= 10
- The risk affects a milestone commitment
- The risk requires cross-team coordination or external dependencies
- The risk has been open for more than two sprints without progress on mitigation

## Related Documents

- [Definition of Done](definition-of-done.md)
- [Jira Story Template](jira-story-template.md)
