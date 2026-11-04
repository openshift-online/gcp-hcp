# Pod Team Field Automation

This document describes the Jira automation rules that manage the Team custom field for pod-based organization of work in the GCP project.

## Overview

The GCP HCP team is organized into three pods (A, B, C). Work is automatically organized by pod using the Team custom field, which determines which items appear on each pod's board when using Quick Filters.

**Pod membership:** See the [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835/Team+Ceremonies+Calendar) in Confluence for current pod assignments.

> **Note:** Pod membership is maintained in Confluence as the single source of truth. This ensures accuracy as team members join, leave, or change pods.

## Automation Rules

The GCP project has 8 automation rules that manage Team field population:

### Rules 1-3: Auto-assign Team to Features/Initiatives

**Purpose:** Automatically populate the Team field when a Feature or Initiative is assigned to a pod member.

**Trigger:** Issue updated

**Conditions:**
1. Issue Type is one of: Feature, Initiative
2. Assignee is one of: [pod members]
3. Team is EMPTY

**Action:** Set Team field to the appropriate pod (Pod A, Pod B, or Pod C)

**Rule details:**
- **Rule 1:** Pod A members → Team = Pod A
- **Rule 2:** Pod B members → Team = Pod B
- **Rule 3:** Pod C members → Team = Pod C

### Rules 4-5: Cascade Team from Parent

**Purpose:** Automatically inherit Team field from parent issue to child issues when the child is created or updated.

**Trigger:** Work item created or updated (the child issue)

**Conditions:**
- For Rule 4: Issue Type equals Epic AND Parent matches JQL "Team is not EMPTY" AND Team is EMPTY
- For Rule 5: Issue Type is one of (Story, Task, Bug, Spike) AND Parent matches JQL "Team is not EMPTY" AND Team is EMPTY

**Action:** Copy Team from parent work item

**Rule details:**
- **Rule 4:** Epic inherits Team from parent Feature or Initiative (when Epic is created/updated and Epic's Team is empty)
- **Rule 5:** Story/Task/Bug/Spike inherits Team from parent Epic (when child is created/updated and child's Team is empty)

**Note:** These rules trigger when the **child** issue is created or updated, not when the parent's Team value changes. If you change a parent's Team value, existing children will not be updated automatically.

### Rules 6-8: Auto-assign Team to Orphaned Items

**Purpose:** Automatically populate Team field for orphaned Epics/Stories/Tasks (items without a parent) when assigned to a pod member.

**Trigger:** Issue updated

**Conditions:**
1. Issue Type is one of: Story, Task, Bug, Spike, Epic
2. Parent does not exist
3. Assignee is one of: [pod members]
4. Team is EMPTY

**Action:** Set Team field to the appropriate pod (Pod A, Pod B, or Pod C)

**Rule details:**
- **Rule 6:** Pod A members → Team = Pod A (if no parent and Team empty)
- **Rule 7:** Pod B members → Team = Pod B (if no parent and Team empty)
- **Rule 8:** Pod C members → Team = Pod C (if no parent and Team empty)

## Team is EMPTY Safeguard

All automation rules (Rules 1-8) include a "Team is EMPTY" condition. This means:

- Automation only runs when the Team field is not already set
- Manual Team assignments are never overwritten by automation
- You can safely override the Team field for edge cases

**Use cases for manual overrides:**
- Cross-pod collaboration (e.g., tech lead from Pod A working on Pod B feature)
- Temporarily moving work between pods
- Features assigned for refinement but owned by a different pod

## Coverage and Hierarchy

The automation ensures comprehensive Team field population across the entire issue hierarchy:

```
Feature/Initiative (Rules 1-3: assignee-based)
  ├── Epic (Rule 4: inherit from Feature/Initiative)
  │     ├── Story (Rule 5: inherit from Epic)
  │     ├── Task (Rule 5: inherit from Epic)
  │     ├── Bug (Rule 5: inherit from Epic)
  │     └── Spike (Rule 5: inherit from Epic)
  └── Orphaned Epic (Rules 6-8: assignee-based)

Orphaned Story/Task/Bug/Spike (Rules 6-8: assignee-based)
```

**Inheritance strategy:** Child issues inherit Team from their parent. This ensures that all work under a Feature shows on that pod's board, even if the child issues are unassigned or assigned to members of other pods.

**Orphaned items:** Items without a parent get Team assigned based on the assignee, using the same logic as Features/Initiatives.

## Using Pod Board Filters

On the Kanban board and Feature/Initiative board, Quick Filters allow you to view work by pod:

- **Pod A Quick Filter:** Shows items where Team = Pod A
- **Pod B Quick Filter:** Shows items where Team = Pod B
- **Pod C Quick Filter:** Shows items where Team = Pod C

**JQL for each filter:**
```
Team = "Pod A"
Team = "Pod B"
Team = "Pod C"
```

## Assignee Policy and Team Field

The GCP team's assignee policy affects when Team field gets populated:

**Workflow stages:**
- **New, Refinement, To Do:** Items may be unassigned (per team policy: "To Do must remain unassigned")
- **In Progress, Review, Closed:** Items should have an assignee

**Impact on Team field:**
- Unassigned items in New/Refinement/To Do will not have Team field populated (expected behavior)
- Team field populates when the item is assigned during transition to In Progress
- Items can still be manually assigned to a pod even when unassigned (for planning purposes)

## Manual Team Assignment

To manually set or change the Team field:

1. Edit the issue in Jira
2. Find the Team field
3. Select the desired pod (Pod A, Pod B, or Pod C)
4. Save

The automation will not overwrite your manual assignment thanks to the "Team is EMPTY" condition.

## Troubleshooting

### Issue not appearing on pod board

**Possible causes:**
1. Team field is empty (check issue details)
2. Item is unassigned and in New/Refinement/To Do status (expected per assignee policy)
3. Item is an orphan without a parent and without an assignee

**Solutions:**
- Manually set the Team field if needed for planning
- Assign the item to a pod member to trigger auto-population
- For unassigned items that need pod visibility, manually set Team

### Issue appearing on wrong pod's board

**Possible causes:**
1. Team field inherited from parent that belongs to different pod
2. Item was assigned to one pod member but work was transferred to another pod
3. Automation populated Team based on assignee but feature ownership differs

**Solutions:**
- Manually set the Team field to the correct pod (your change will stick)
- Consider whether the parent Feature/Initiative Team field needs updating
- Note: Changing a parent's Team does NOT automatically cascade to existing children; children only inherit when they are created or updated

### Item assigned but Team field still empty

**Possible causes:**
1. Assignee is not a pod member (e.g., assigned to Bill, Kat, or external contributor)
2. Automation rule is disabled or malfunctioning
3. Item is closed (automation may not fire on closed issues)

**Solutions:**
- Manually set the Team field
- Verify assignee is one of the 12 pod members
- Check Jira Automation audit log for rule execution

### Orphaned items (no parent) missing from board

**Possible causes:**
1. Item is unassigned (Rules 6-8 require an assignee to fire)
2. Item is in New/Refinement/To Do status without manual Team assignment

**Solutions:**
- Assign to a pod member to trigger Rules 6-8
- OR manually set Team field for planning purposes

## Common JQL Queries

### Find items missing Team field (active work only)

```
project = GCP AND Team is EMPTY AND resolution = Unresolved AND status IN ("In Progress", "Review")
```

This identifies items actively being worked on that are missing Team assignment.

### Find orphaned items for a specific pod

> **Maintenance note:** These queries include Jira account IDs for pod members. When pod membership changes, update the account IDs to match the current membership listed in the [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835/Team+Ceremonies+Calendar).

**Pod A:**
```
project = GCP AND type in (Story, Task, Bug, Spike, Epic) AND parent is EMPTY AND assignee IN (557058:f58d285f-ca16-4196-98ae-c8576387a7a8, 712020:05660a16-5e3f-4d3e-bd7e-b8f0bc0ab8ca, 712020:53e74065-a06d-48da-91ec-95dd01f59f18, 712020:b2c7df9f-43dd-4999-a8a7-d1bbb73c1b85) AND Team is EMPTY AND resolution = Unresolved
```

**Pod B:**
```
project = GCP AND type in (Story, Task, Bug, Spike, Epic) AND parent is EMPTY AND assignee IN (71e7cb8b-69d6-4f1a-bc99-c0dd159b44cd, 712020:cb8b470b-78d3-4bb3-87e7-e0303090dd22, 712020:e08ab655-a58a-44ba-a6f9-ce604be36de3, 712020:8fe92d81-e996-45c1-bfc8-c40af89ddd05) AND Team is EMPTY AND resolution = Unresolved
```

**Pod C:**
```
project = GCP AND type in (Story, Task, Bug, Spike, Epic) AND parent is EMPTY AND assignee IN (5c23e222b92be60d8040734f, 63d898f889bf35bcc8cf08ea, 712020:adf0b2e3-0ac6-40aa-a97a-a4c4f7cc2656, 712020:19a8c82b-ec27-404c-877e-3ef8f06cd756) AND Team is EMPTY AND resolution = Unresolved
```

### View all work for a specific pod

```
project = GCP AND Team = "Pod A" AND resolution = Unresolved
project = GCP AND Team = "Pod B" AND resolution = Unresolved
project = GCP AND Team = "Pod C" AND resolution = Unresolved
```

## Implementation History

**Automation rules created:** October 2026

**Initial backfill:**
- 21 Features/Initiatives
- 62 Epics
- 299 Stories/Tasks/Bugs/Spikes
- Additional orphaned items across all pods

**Tracking ticket:** GCP-1310 - Create Pod-Specific Jira Boards and Automation

## Related Documentation

- [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835/Team+Ceremonies+Calendar) - Pod structure, ceremonies, and board usage
- [Jira Hierarchy](jira-hierarchy.md) - Issue type hierarchy and parent-child relationships
- [Feature/Initiative Playbook](feature-initiative-playbook.md) - Process for Features and Initiatives
- [Definition of Ready](definition-of-ready.md) - Assignee policy and workflow stages
