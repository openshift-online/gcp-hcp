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

**Two different field types are used:**

### Epic Link Field
- **Purpose**: Links Stories, Tasks, and Bugs to their parent Epic
- **Direction**: Level 2 → Level 3
- **Field ID**: `customfield_12310140`
- **Example**: Story GCP-100 links to Epic GCP-50

### Parent Link Field
- **Purpose**: Links all other hierarchical relationships
- **Direction**: Epics → Features/Initiatives, Features/Initiatives → Outcomes → Strategic Goals
- **Field ID**: `customfield_12313140`
- **Example**: Epic GCP-50 links to Feature GCP-10, or Epic GCP-60 links to Initiative GCP-20

**Why Two Fields?**
- Epic Link field predates the Parent Link field in Jira's evolution
- Maintains backward compatibility with existing workflows
- Both fields serve the same conceptual purpose (establishing parent-child relationships)

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
2. **Use correct field for link type**:
   - Stories/Tasks/Bugs → Epic: Use **Epic Link field**
   - Epics → Feature or Initiative: Use **Parent Link field** (choose Feature OR Initiative, not both)
3. **Create Epic first, then link to Feature** - Defensive two-step approach prevents creation failures
4. **Don't force parent creation** - Only create parent issues when explicitly requested or contextually necessary for grouping/coordination

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
- [Jira Initiative Template](./jira-initiative-template.md) - Initiative structure
- [Jira Task Template](./jira-task-template.md) - Task structure
- [AGENTS.md](../AGENTS.md) - AI agent guidance including Jira plugin details
