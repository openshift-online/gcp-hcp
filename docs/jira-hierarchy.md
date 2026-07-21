# Jira Hierarchy for GCP HCP Team

This document describes the Jira issue hierarchy used by the GCP HCP team within the broader Hybrid Platforms organization structure.

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

Most work happens at Levels 2-4. Outcomes and Strategic Goals (Levels 5-6) are managed at the org-wide strategy level (HPSTRAT/HATSTRAT projects), not within the GCP project.

---

## Linking Mechanisms

Two different Jira field types establish parent-child relationships:

### Epic Link Field
- **Purpose**: Links Stories, Tasks, and Bugs to their parent Epic
- **Direction**: Level 2 → Level 3
- **Field ID**: `customfield_10014`

### Parent Link Field
- **Purpose**: Links all other hierarchical relationships
- **Direction**: Epics → Features/Initiatives, Features/Initiatives → Outcomes → Strategic Goals

---

## Feature vs Initiative (Level 4)

Feature and Initiative are **mutually exclusive** issue types at Level 4:

- **Feature** — Tangible value delivered to **customers** as part of the product roadmap. Example: "Customers can now configure custom encryption keys for their clusters"
- **Initiative** — Internal/architectural work that does **not directly contribute to the product roadmap**. Example: "Underlying architecture changed to improve reliability"

If customers will directly use the new capability → Feature. If it's internal improvement → Initiative.

---

## Valid Structures

- ✅ **Story alone** — Small bug fix or minor improvement (Level 2 only)
- ✅ **Task alone** — Team process work, one-off documentation (Level 2 only)
- ✅ **Stories → Epic** — Related Stories grouped under an Epic (Levels 2-3)
- ✅ **Stories → Epic → Feature** — Capability spanning multiple Epics (Levels 2-4)
- ✅ **Stories → Epic → Initiative** — Large strategic effort spanning multiple Epics (Levels 2-4)
- ✅ **Epic alone** — Self-contained Epic without needing a parent (Level 3 only)
- Stories → Epic → Feature → Outcome → Strategic Goal (full 6-level hierarchy, rarely used)

---

## When to Create Parent Issues

**Only create parent issues when they serve a real purpose for grouping, coordination, or visibility.** Standalone Stories, Tasks, and Epics are valid when work doesn't require grouping.

### Create an Epic (Level 3) when:
- Multiple Stories/Tasks share a common technical goal or component
- You need to track progress of related work as a cohesive unit
- Work will span multiple iterations but doesn't need Feature-level coordination

### Create a Feature (Level 4) when:
- Work delivers **customer-facing** product capabilities
- Multiple Epics need coordination to deliver a cohesive customer capability
- Portfolio-level tracking is needed for quarterly/milestone planning

### Create an Initiative (Level 4) when:
- Work is **internal/architectural** improvement
- Multiple Epics need coordination for architectural or process changes

### Don't create parents just to satisfy hierarchy

- ❌ Creating an Epic just because "Stories need Epics"
- ❌ Creating a Feature just because "Epics need Features"
- ❌ Auto-generating parent issues to fill hierarchy gaps

---

## For AI Agents and Automation

When creating or updating Jira issues programmatically:

1. **Check for existing parents first** — Don't auto-create; link to existing issues when appropriate
2. **Use correct field for link type**:
   - Stories/Tasks/Bugs → Epic: Use **Epic Link field**
   - Epics → Feature or Initiative: Use **Parent Link field** (choose Feature OR Initiative, not both)
3. **Create Epic first, then link to Feature** — Defensive two-step approach prevents creation failures
4. **Don't force parent creation** — Only create parent issues when explicitly requested or contextually necessary

---

## Project Locations

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
- [Jira Initiative Template](./jira-initiative-template.md) - Initiative structure
- [Jira Task Template](./jira-task-template.md) - Task structure
- [AGENTS.md](../AGENTS.md) - AI agent guidance including Jira plugin details
