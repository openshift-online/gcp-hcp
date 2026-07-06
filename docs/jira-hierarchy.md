# Jira Hierarchy for GCP HCP Team

This document describes the Jira issue hierarchy used by the GCP HCP team within the broader Hybrid Platforms organization structure.

## Overview

**Only create parent issues when they serve a real purpose for grouping, coordination, or visibility.**

The GCP team typically works **top-down** — decomposing Features into Epics and Epics into Stories. However, standalone Stories, Tasks, or Bugs are valid when work doesn't require grouping. Link to existing parents when appropriate; don't create unnecessary hierarchy.

---

## Hybrid Platforms Jira Hierarchy

The GCP HCP team operates within the broader Hybrid Platforms organization hierarchy:

```text
Level 6: Strategic Goal (HATSTRAT)         [Roadmap/Strategy]
   ↓ Parent Link field
Level 5: Outcome (HPSTRAT)                 [Org-wide Strategy]
   ↓ Parent Link field
Level 4: Feature / Initiative (GCP)        [Business Unit] ← GCP team works here
   ↓ Parent Link field
Level 3: Epic (GCP or Eng Team Projects)   [Execution - Team]
   ↓ Epic Link field
Level 2: Story / Task / Bug (GCP)          [Execution - Individual Work]
```

**Important**: The GCP project operates at **Level 4** (Business Unit), where Features and Initiatives live. Most work happens at Levels 2-4.

---

## Linking Mechanisms

Jira uses the **`parent`** field to establish all parent-child relationships. In the UI and in API responses, these relationships also appear in legacy custom fields for backward compatibility:

### Epic Link Field (UI / read-only reference)
- **Purpose**: Links Stories, Tasks, and Bugs to their parent Epic
- **Direction**: Level 2 → Level 3
- **Field ID**: `customfield_10014`
- **Example**: Story GCP-100 links to Epic GCP-50

### Parent Link Field (UI / read-only reference)
- **Purpose**: Links all other hierarchical relationships
- **Direction**: Epics → Features/Initiatives, Features/Initiatives → Outcomes → Strategic Goals
- **Example**: Epic GCP-50 links to Feature GCP-10, or Epic GCP-60 links to Initiative GCP-20

> **For programmatic use (REST API):** Always use the standard `parent` field to set or update parent-child relationships. See [Jira API Technical Reference](#jira-api-technical-reference) for details.

---

## Valid Structures for GCP HCP Team

The team typically decomposes **top-down** from Features → Epics → Stories, but not all work requires the full hierarchy. Create parents only when needed:

### Common Structures

- ✅ **Story alone** - Small bug fix or minor improvement (Level 2 only)
- ✅ **Task alone** - Team process work, one-off documentation (Level 2 only)
- ✅ **Stories → Epic** - Related Stories grouped under an Epic (Levels 2-3)
- ✅ **Stories → Epic → Feature** - Capability spanning multiple Epics (Levels 2-4)
- ✅ **Stories → Epic → Initiative** - Large strategic effort spanning multiple Epics (Levels 2-4)
- ✅ **Epic alone** - Self-contained Epic without needing a parent (Level 3 only)

### Rarely Used by GCP Team

- Stories → Epic → Feature → Outcome → Strategic Goal (full 6-level hierarchy)

**Outcomes and Strategic Goals** (Levels 5-6) are managed at the org-wide strategy level (HPSTRAT/HATSTRAT projects), not within the GCP project.

---

## Feature vs Initiative (Level 4)

Feature and Initiative are **mutually exclusive** issue types at Level 4. Choose one based on the nature of the work:

### Feature
- **Purpose**: Tangible pieces of value delivered to **customers** as part of the product roadmap
- **Audience**: External - customer-facing capabilities
- **Delivery means**: Customers can use new functionality in the product
- **Example**: "Customers can now configure custom encryption keys for their clusters"

### Initiative
- **Purpose**: Larger goals that do **not directly contribute to the product roadmap**; typically **architectural or improvement-focused** work
- **Audience**: Internal - Red Hat/team capabilities
- **Delivery means**: Red Hat associates can do something more/better/differently
- **Example**: "Underlying architecture changed to improve reliability (while maintaining existing functionality)"

**Key distinction**: If customers will directly use the new capability → Feature. If it's internal improvement/architectural work → Initiative.

---

## When to Create Parent Issues

### Create an Epic (Level 3) when:
- Multiple Stories/Tasks share a common technical goal or component
- You need to track progress of related work as a cohesive unit
- Work benefits from grouping but doesn't need strategic visibility
- Work will span multiple iterations but doesn't need Feature-level coordination

### Create a Feature (Level 4) when:
- Work delivers **customer-facing** product capabilities
- Multiple Epics need coordination to deliver a cohesive customer capability
- Work will be visible on the product roadmap
- Portfolio-level tracking is needed for quarterly/milestone planning

### Create an Initiative (Level 4) when:
- Work is **internal/architectural** improvement that doesn't directly appear on product roadmap
- Multiple Epics need coordination for architectural or process changes
- Work improves internal capabilities but doesn't add customer-facing features
- Examples: infrastructure refactoring, process improvements, technical debt reduction

### Don't create Outcomes (Level 5) or Strategic Goals (Level 6)
These are managed at the org-wide strategy level by leadership teams, not within individual project backlogs.

---

## When Parent Issues Are Optional

**Don't create parent issues just to satisfy hierarchy.**

Valid approaches:
- ✅ A standalone Story for a bug fix (no Epic needed)
- ✅ A standalone Task for team process work (no Epic needed)
- ✅ An Epic with Stories but no Feature (work is self-contained)
- ✅ Linking to existing parent issues when work naturally fits

Invalid approaches:
- ❌ Creating an Epic just because "Stories need Epics"
- ❌ Creating a Feature just because "Epics need Features"
- ❌ Auto-generating parent issues to fill hierarchy gaps

**Only create parents when they serve a real purpose**: grouping related work, enabling coordination, or providing necessary visibility.

---

## Linking Best Practices

### For AI Agents and Automation

When creating or updating Jira issues programmatically:

1. **Check for existing parents first** - Don't auto-create; link to existing issues when appropriate
2. **Set the `parent` field** to establish parent-child links at any hierarchy level. Use the parent issue's key (e.g., `"parent": {"key": "GCP-627"}`). This works for:
   - Stories/Tasks/Bugs → Epic
   - Epics → Feature or Initiative (choose Feature OR Initiative, not both)
3. **Set `parent` at creation time** - Include the `parent` field in the create payload. No separate edit call is needed.
4. **Don't force parent creation** - Only create parent issues when explicitly requested or contextually necessary for grouping/coordination

### Jira API Technical Reference

> **This section documents the exact API behavior for the GCP project on our Jira Cloud instance (redhat.atlassian.net). AI agents should follow these patterns to avoid trial-and-error API calls.**
>
> Field IDs were verified against live API responses in July 2025 ([PR #79 review](https://github.com/openshift-online/gcp-hcp/pull/79#issuecomment-4868112074)).

#### Setting parent links (Story → Epic, Epic → Feature/Initiative)

**Use the standard `parent` field.** It handles all parent-child relationships regardless of hierarchy level and can be set at creation time:

```json
{
  "parent": { "key": "GCP-627" }
}
```

This works for both Story → Epic and Epic → Feature/Initiative links. The `parent` field auto-populates the Epic Link custom field (`customfield_10014`) — no need to set that field directly.

**Recommended pattern (single API call):**

```text
POST /rest/api/3/issue → create issue with "parent": {"key": "EPIC-KEY"} in the payload
```

The `parent` field is also settable via `PUT /rest/api/3/issue/{key}` if you need to change or add a parent after creation.

#### Custom fields settable via API

These fields can be set at creation time via `additional_fields` or updated via edit:

| Field | Field ID | Value Format |
|-------|----------|---|
| Story Points | `customfield_10028` | Number (e.g., `3`) |
| Epic Link (read) | `customfield_10014` | String — auto-populated by `parent`, do not set directly |
| Epic Name | `customfield_10011` | String |
| Risk Probability | `customfield_10642` | `{"value": "Very Likely"}` |
| Risk Impact | `customfield_10842` | `{"value": "Major"}` |
| Risk Score | `customfield_10976` | Number |

#### Creating issue links (Related, Blocks, etc.)

Issue links (non-hierarchical relationships like "Related" or "Blocks") use the issue link API, not fields:

```json
{
  "type": { "name": "Related" },
  "inwardIssue": { "key": "GCP-869" },
  "outwardIssue": { "key": "RFE-4099" }
}
```

For directional links (e.g., Blocks): `inwardIssue` is the blocker, `outwardIssue` is the blocked issue. Example: "GCP-627 blocks SDCICD-1843" → `inwardIssue: GCP-627`, `outwardIssue: SDCICD-1843`.

**Note:** Creating issue links requires "Link Issue" permission on both issues. Cross-project links may fail if the agent lacks permission on the target project.

#### Common pitfalls (avoid these)

| Pitfall | Fix |
|---------|-----|
| Using `customfield_10014` to set Epic Link | Use `parent` field instead — `customfield_10014` is auto-populated |
| Using `customfield_10028` for the wrong thing | Verify it's "Story Points" (type: float), not another field |
| Guessing link type names for `createIssueLink` | Call `getIssueLinkTypes` first if unsure |
| Linking to issues in projects without permission | Reference the issue key in the description instead |

### For Manual Jira Usage

When creating issues in the Jira UI:

1. Start with the work item (Story, Task, Bug, Epic)
2. Ask: "Does this need a parent for grouping, tracking, or visibility?"
3. If yes: Link to existing parent or create one
4. If no: Leave it standalone

---

## Project Locations

Understanding where issue types live across projects:

| Issue Type | Primary Project(s) | Notes |
|------------|-------------------|-------|
| **Story** | GCP | Individual work items |
| **Task** | GCP | Internal work, process, docs |
| **Bug** | GCP | Defects and fixes |
| **Spike** | GCP | Research and investigation |
| **Epic** | GCP or team projects | Team-level execution |
| **Feature** | GCP | Business unit capabilities |
| **Initiative** | GCP | Strategic business unit efforts |
| **Outcome** | HPSTRAT | Org-wide strategy (not GCP) |
| **Strategic Goal** | HATSTRAT | Top-level roadmap (not GCP) |

---

## Related Documentation

- [Definition of Ready](./definition-of-ready.md) - Readiness criteria for all issue types
- [Definition of Done](./definition-of-done.md) - Completion criteria
- [Jira Story Template](./jira-story-template.md) - Story structure and sizing
- [Jira Epic Template](./jira-epic-template.md) - Epic structure and breakdown
- [Jira Feature Template](./jira-feature-template.md) - Feature structure
- [Jira Task Template](./jira-task-template.md) - Task structure
- [AGENTS.md](../AGENTS.md) - AI agent guidance including Jira plugin details
