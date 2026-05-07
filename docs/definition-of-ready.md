---
name: Definition of Ready
description: The Definition of Ready (DoR) is a checklist of criteria that must be met before a backlog item can be considered "ready" for development and implementation. This enables AI-assisted backlog refinement and ensures consistent quality across all issue types.
tools: Read, Grep, Glob, Task
model: sonnet
---

# Definition of Ready

The Definition of Ready (DoR) establishes clear, checkable criteria for when a backlog item is ready to be pulled into active development. Items meeting DoR criteria are well-understood, properly scoped, and ready for implementation.

## Why Definition of Ready Matters

- **Prevents incomplete work from starting** - Teams avoid beginning work that lacks clarity or dependencies
- **Enables AI-assisted validation** - Agents can programmatically check if items meet criteria
- **Improves flow efficiency** - Well-refined items move smoothly through the development pipeline
- **Reduces work-in-progress churn** - Clear requirements minimize surprises during implementation

---

## Hierarchy Guidance

**When looking at work from the bottom to the top of the hierarchy, only create issues as far up in the hierarchy as necessary to ensure that issues align with their intended purpose.**

### Hybrid Platforms Jira Hierarchy

The GCP HCP team operates within the broader Hybrid Platforms organization hierarchy:

```
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

**Important**: The GCP project operates at **Level 4** (Business Unit), where Features and Initiatives live.

### Linking Mechanisms

**Two different field types are used:**

1. **Epic Link field** - Links Stories/Tasks/Bugs → Epics (Level 2 → Level 3)
2. **Parent Link field** - Links everything else (Epics → Features → Outcomes → Strategic Goals)

### Valid Structures for GCP HCP Team

Work from the **bottom-up**. Start with the work item, then create parents only as far up as needed:

- ✅ **Story alone** - Small bug fix or minor improvement (Level 2 only)
- ✅ **Task alone** - Team process work, one-off documentation (Level 2 only)
- ✅ **Stories → Epic** - Related Stories grouped under an Epic (Levels 2-3)
- ✅ **Stories → Epic → Feature** - Capability spanning multiple Epics (Levels 2-4)
- ✅ **Stories → Epic → Initiative** - Large initiative grouping Features (Levels 2-4)
- ✅ **Epic alone** - Self-contained Epic without needing a Feature parent (Level 3 only)

**Rarely used by GCP team:**
- Stories → Epic → Feature → Outcome → Strategic Goal (full 6-level hierarchy)

### When to Create Parent Issues

**Create an Epic (Level 3) when:**
- Multiple Stories/Tasks share a common technical goal or component
- You need to track progress of related work as a cohesive unit
- Work benefits from grouping but doesn't need strategic visibility

**Create a Feature (Level 4) when:**
- Multiple Epics need coordination and represent a broader capability
- Work represents a major product capability visible to customers or stakeholders
- Portfolio-level tracking is needed for quarterly/milestone planning

**Create an Initiative (Level 4) when:**
- Work represents a major strategic effort grouping multiple Features
- Cross-team or cross-quarter coordination is required
- Executive-level visibility and alignment is needed

**Don't create Outcomes (Level 5) or Strategic Goals (Level 6)** - These are managed at the org-wide strategy level (HPSTRAT/HATSTRAT), not within GCP project.

### Bottom-Up Principle

**Don't create parent issues just to satisfy hierarchy.** If a Story stands alone and doesn't need an Epic, let it stand alone. If an Epic doesn't need a Feature parent, that's valid too.

Only create parents when they serve a real purpose: grouping related work, enabling coordination, or providing necessary visibility.

---

## Definition of Ready: Initiative

Initiatives represent large portfolio goals spanning multiple quarters and Features.

**Ready Criteria**:
- [ ] **Title** follows format: `[Action Verb] + [Capability]`
- [ ] **Context** clearly explains why this Initiative supports strategic goals
- [ ] **Scope** defines what's included and what's NOT included
- [ ] **Acceptance Criteria** are specific and measurable (3+ criteria)
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Dependencies** are identified (other Initiatives, external teams, approvals)
- [ ] **DRI** (Directly Responsible Individual) is assigned
- [ ] **Size Estimate** is set (Small/Medium/Large)
- [ ] At least one **Feature** has been identified for breakdown

**Template**: [docs/jira-feature-template.md](./jira-feature-template.md) (adapt for Initiative level)

---

## Definition of Ready: Feature

Features represent high-level capabilities that decompose into Epics.

**Ready Criteria**:
- [ ] **Title** follows format: `[Action Verb] + [Capability]`
- [ ] **Context** explains why this Feature is needed and how it supports goals
- [ ] **Scope** defines what's included AND what's NOT included
- [ ] **Acceptance Criteria** are specific and measurable (3+ criteria)
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Dependencies** are identified and documented
- [ ] **DRI** (Directly Responsible Individual) is assigned
- [ ] **Size Estimate** is set (Small/Medium/Large)
- [ ] **Demo Critical** flag is set (Yes/No)
- [ ] **Parent Link field** is set to link to parent Initiative **if this Feature is part of a larger Initiative**
- [ ] At least one **Epic** has been identified for breakdown

**Template**: [docs/jira-feature-template.md](./jira-feature-template.md)

---

## Definition of Ready: Epic

Epics represent cohesive chunks of work within a Feature (or standalone work).

**Ready Criteria**:
- [ ] **Title** follows format: `[Action Verb] + [Specific Capability or Component]`
- [ ] **Use Case / Context** clearly explains why this Epic is needed
- [ ] **Current State** describes existing limitations or gaps
- [ ] **Desired State / Goal** describes what will be true when complete
- [ ] **Scope** defines what's covered AND what's out of scope
- [ ] **Acceptance Criteria** are specific and measurable (3+ criteria)
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Dependencies** are identified and tracked
- [ ] **Epic Name** custom field is populated
- [ ] **Parent Link field** is set to link to parent Feature or Initiative **if this Epic is part of a broader Feature/Initiative**
- [ ] **DRI** (Directly Responsible Individual) is assigned
- [ ] **Size Estimate** is set (Small/Medium/Large)
- [ ] At least 2-3 **Stories** have been identified for breakdown
- [ ] **Story Breakdown Checklist** section is started

**Template**: [docs/jira-epic-template.md](./jira-epic-template.md)

---

## Definition of Ready: Story

Stories represent the smallest unit of user-facing work.

**Ready Criteria**:
- [ ] **User Story** follows format: "As a [user], I want [goal], so that [benefit]"
- [ ] **Context / Background** provides current state and problem being solved
- [ ] **Requirements** include functional and non-functional needs
- [ ] **Technical Approach** outlines proposed solution and major steps
- [ ] **Acceptance Criteria** are specific, testable outcomes (3+ criteria as checkboxes)
- [ ] **Story Points** are estimated using Fibonacci (1, 2, 3, 5, 8) - **Stories should be 1-5 points**
  - [ ] If 8+, Story must be split into smaller Stories
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Epic Link field** is set to link to parent Epic **if this Story is part of a larger Epic**
- [ ] **Dependencies** are identified (blocking items, other teams, access needs)
- [ ] **Assignee** is assigned (or team agrees on who will pick it up)
- [ ] Story is **right-sized** - can be completed in a reasonable timeframe (1-5 days typically)
- [ ] Story has clear value and is demo-able

**Splitting Guidance**: If Story has >5 acceptance criteria, touches >3 components, or includes both investigation AND implementation, it should be split. See [Story Sizing Guide](./jira-story-template.md#story-sizing-guide).

**Template**: [docs/jira-story-template.md](./jira-story-template.md)

---

## Definition of Ready: Task

Tasks represent finite pieces of internal work (process, tooling, documentation, tech debt).

**Ready Criteria**:
- [ ] **Title** is clear and specific
- [ ] **Context / Background** explains why this work is needed
- [ ] **Requirements** define what needs to be delivered
- [ ] **Technical Approach** outlines major steps or approach
- [ ] **Acceptance Criteria** are specific and testable (2+ criteria as checkboxes)
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Epic Link** is set **if this Task is part of a larger Epic**
- [ ] **Dependencies** are identified
- [ ] **Assignee** is assigned (or team agrees on who will pick it up)
- [ ] Task is **right-sized** and clearly scoped

**Note**: Tasks are NOT pointed. Only Stories are estimated with story points.

**Template**: [docs/jira-task-template.md](./jira-task-template.md)

---

## Definition of Ready: Spike

Spikes represent time-boxed research or investigation to reduce risk or uncertainty.

**Ready Criteria**:
- [ ] **Title** starts with "Spike:" followed by research question or area
- [ ] **Context / Background** explains what uncertainty needs to be resolved
- [ ] **Research Questions** are clearly defined (what needs to be answered)
- [ ] **Acceptance Criteria** define what outputs are expected:
  - [ ] Findings documented
  - [ ] Decision made or options presented
  - [ ] Resulting backlog items created
- [ ] **Time-box** is defined (Spikes should be time-boxed, typically 1-3 days)
- [ ] **Priority** is set
- [ ] **Epic Link** is set **if this Spike is part of a larger Epic**
- [ ] **Assignee** is assigned

**Note**: Spikes are NOT pointed. They are time-boxed research activities.

**Important**: Spike output should include follow-up Stories/Tasks for implementation.

---

## Definition of Ready: Bug

Bugs represent problems or errors in existing functionality.

**Ready Criteria**:
- [ ] **Title** clearly describes the problem
- [ ] **Description** includes:
  - [ ] **Steps to Reproduce** (clear, numbered steps)
  - [ ] **Expected Behavior** (what should happen)
  - [ ] **Actual Behavior** (what actually happens)
  - [ ] **Environment** (where the bug occurs: dev, stage, prod)
  - [ ] **Impact** (who is affected, severity of impact)
- [ ] **Priority** is set based on severity and impact
- [ ] **Epic Link** is set **if this Bug fix is part of a larger Epic**
- [ ] **Assignee** is assigned (for critical/blocker bugs)
- [ ] Bug is reproducible OR includes sufficient diagnostic information
- [ ] **Root Cause** is identified or hypothesized (if known)

**Note**: Bugs are NOT pointed. They are prioritized based on severity/impact.

**Severity Guidelines**:
- **Blocker**: Prevents core functionality, no workaround
- **Critical**: Major feature broken, workaround exists but painful
- **Major**: Important feature affected, reasonable workaround exists
- **Normal**: Minor feature affected
- **Minor**: Cosmetic or edge case

---

## AI Agent Usage

This Definition of Ready is designed for both human and AI-assisted validation.

### For AI Agents Performing Backlog Refinement

When validating or creating backlog items, AI agents should:

1. **Check issue type** - Identify if item is Initiative, Feature, Epic, Story, Task, Spike, or Bug
2. **Apply correct criteria** - Use the DoR section for that specific issue type
3. **Validate each checkbox** - Programmatically check if each criterion is met
4. **Report gaps** - Clearly list which criteria are missing or incomplete
5. **Suggest improvements** - Offer to fill in missing fields based on templates
6. **Reference templates** - Point to the appropriate template in `docs/` for that issue type
7. **Respect hierarchy flexibility** - Don't require parent links unless the item genuinely needs a parent for context

### Machine-Readable Validation

Each DoR criterion is a boolean check:
- ✅ **Pass**: Criterion is met (field populated, format correct, linked properly)
- ❌ **Fail**: Criterion is not met (field empty, format incorrect, missing link)
- ⚠️ **Conditional**: Criterion is optional based on context (e.g., Epic Link for standalone work)

AI agents should output validation results in this format:

```
Definition of Ready Check: [ISSUE-KEY]
Issue Type: [Story/Epic/Task/etc]
Status: [READY/NOT READY]

✅ Title follows format
✅ Context provided
❌ Story points not estimated
⚠️ Epic Link not set (optional for standalone Stories)
✅ Acceptance criteria defined (3)

MISSING: 1 required criterion
RECOMMENDATION: Set story points (suggest 3 based on complexity)
NOTE: Epic Link is optional - only set if this Story is part of a larger Epic
```

### Integration with Jira Plugin

The `jira:gcp-hcp` skill (from openshift-eng/ai-helpers) should reference this DoR when:
- Creating new issues
- Refining existing issues
- Validating backlog health
- Preparing items for development

**Important**: The skill should link to existing parent issues when appropriate (Stories under Epics, Epics under Features) but avoid automatically creating new parent issues just to satisfy hierarchy. Link when it adds value for grouping and tracking; omit when the work is truly standalone.

---

## Using This Definition of Ready

### Before Starting Work
1. Check that the item meets all DoR criteria for its type
2. Work with DRIs to complete any missing criteria
3. Only begin work on items that meet DoR

### During Backlog Refinement
1. Review upcoming work against DoR
2. Fill in gaps collaboratively
3. Split oversized items (Stories >5 points, Epics that are too large)
4. Update priorities based on team goals
5. Create parent issues (Epic/Feature/Initiative) only when needed for grouping or tracking

### With AI Assistance
1. Ask Claude to validate an issue: "Does GCP-XXX meet the Definition of Ready?"
2. Ask Claude to complete gaps: "Help me refine GCP-XXX to meet DoR"
3. Ask Claude to create compliant issues: "Create a Story for [feature] that meets DoR"
4. Let Claude know if work is standalone: "Create a standalone Story for [work] - no Epic needed"

### Workflow Integration
- **Kanban**: Items move from "Backlog" → "Ready" only when DoR is met
- **Pull-based**: Team members pull from "Ready" when capacity allows
- **Flow efficiency**: DoR ensures items entering active development are well-defined, reducing cycle time variability

---

## Related Documentation

- [Definition of Done](./definition-of-done.md) - Completion criteria for implemented work
- [Jira Story Template](./jira-story-template.md) - Story structure and sizing guide
- [Jira Epic Template](./jira-epic-template.md) - Epic structure and breakdown guide
- [Jira Feature Template](./jira-feature-template.md) - Feature structure
- [Jira Task Template](./jira-task-template.md) - Task structure
