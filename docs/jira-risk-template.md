# Jira Risk Template

Use this template when creating a Risk issue in the [GCP project](https://redhat.atlassian.net/jira/software/projects/GCP/boards). See [risk-tracking-process.md](risk-tracking-process.md) for qualifying criteria, probability/impact scales, field definitions, and lifecycle workflow.

## Is This a Risk?

Before creating, confirm it qualifies — see the [Qualifying a Risk](risk-tracking-process.md#qualifying-a-risk) section of the process doc.

If it doesn't qualify, create a Story, Task, or Epic instead.

---

## Summary (Risk Statement)

Use this one-line pattern for the Jira summary field:

```
<event> could <consequence> due to <root cause>
```

**Examples:**
- "Cincinnati API outage could block new cluster creation due to version resolution dependency"
- "Vendor contract renewal delay could pause GKE upgrade path due to dependency on partner SLA"

---

## Description

```
_What could go wrong:_ <Describe the risk event in detail>

_What triggers it:_ <Conditions or events that would cause the risk to materialize>

_What would be affected:_ <Teams, services, milestones, or customers impacted>

_Mitigation/contingency plan:_ <Actions to reduce probability or impact; how to respond if it materializes. Leave blank if not yet assessed — fill in during the Assess phase.>

_Originally raised: YYYY-MM-DD. Raised by: <your name>._
```

---

## Required Fields

Set these in the right-hand panel. See [risk-tracking-process.md](risk-tracking-process.md) for probability and impact level criteria.

| Field | Notes |
|---|---|
| Risk Probability | Rare / Unlikely / Moderate / Likely / Very Likely |
| Risk Impact | Annoyance / Low / Moderate / Medium / High |
| Component | GCP component area this risk relates to |

**Risk Score** and **Risk Score Assessment** are auto-calculated by ScriptRunner when Probability and Impact are saved — do not fill these manually.

## Optional Fields

| Field | Notes |
|---|---|
| Risk Proximity | When could it materialize? |
| Risk Response | Avoid / Mitigate / Transfer / Accept |
| Risk Category | Technical / Schedule / Resource / External / etc. |

---

## Workflow

New risks start in **New** status. See [risk-tracking-process.md](risk-tracking-process.md) for the full workflow, escalation criteria, and closure protocol.
