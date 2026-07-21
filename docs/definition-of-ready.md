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

## Jira Hierarchy Guidance

**Don't create unnecessary parent issues**: Link work items to existing parents when appropriate, but standalone Stories/Tasks/Bugs are valid when they don't need grouping. The team typically decomposes top-down from Features → Epics → Stories.

The GCP HCP team uses a 6-level Jira hierarchy within the broader Hybrid Platforms organization. For complete details on hierarchy structure, linking mechanisms, valid structures, and when to create parent issues, see:

**[Jira Hierarchy Documentation](./jira-hierarchy.md)**

Key points:
- ✅ Standalone Stories/Tasks are valid (no Epic needed for small, isolated work)
- ✅ Use **Epic Link field** for Stories/Tasks/Bugs → Epics
- ✅ Use **Parent Link field** for Epics → Feature or Initiative (choose one)
- ❌ Don't create parent issues just to satisfy hierarchy

---

## Definition of Ready: Initiative

Initiatives represent internal/architectural work at Level 4. See [Jira Hierarchy](./jira-hierarchy.md#feature-vs-initiative-level-4) for Feature vs Initiative distinction.

**Ready Criteria**:
- [ ] **Title** follows format: `[Action Verb] + [Capability]`
- [ ] **Context** clearly explains why this Initiative supports strategic goals
- [ ] **Scope** defines what's included and what's NOT included
- [ ] **Acceptance Criteria** are specific and measurable (3+ criteria)
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Dependencies** are identified (other Initiatives, external teams, approvals)
- [ ] **Assignee** is assigned
- [ ] At least one **Epic** has been identified for breakdown

**Template**: [docs/jira-initiative-template.md](./jira-initiative-template.md)

---

## Definition of Ready: Feature

Features represent customer-facing capabilities at Level 4. See [Jira Hierarchy](./jira-hierarchy.md#feature-vs-initiative-level-4) for Feature vs Initiative distinction.

**Ready Criteria**:
- [ ] **Title** follows format: `[Action Verb] + [Capability]`
- [ ] **Context** explains why this Feature is needed and how it supports goals
- [ ] **Scope** defines what's included AND what's NOT included
- [ ] **Acceptance Criteria** are specific and measurable (3+ criteria)
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Dependencies** are identified and documented
- [ ] **Assignee** is assigned
- [ ] **Demo Critical** flag is set (Yes/No)
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
- [ ] **Assignee** is assigned
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
- [ ] **Story Points** are estimated using Fibonacci (1, 2, 3, 5, 8, 13)
  - [ ] Stories sized at 13 points must be split into smaller Stories
  - [ ] Stories sized at 8 points should be considered for splitting
- [ ] **Priority** is set (Blocker/Critical/Major/Normal/Minor)
- [ ] **Epic Link field** is set to link to parent Epic **if this Story is part of a larger Epic**
- [ ] **Dependencies** are identified (blocking items, other teams, access needs)
- [ ] **Assignee** is assigned (or team agrees on who will pick it up)
- [ ] Story is **right-sized** - can be completed in a reasonable timeframe (1-5 days typically)
- [ ] Story has clear value and is demo-able

**Note**: Stories are the **only** issue type that uses story points. Tasks, Spikes, and Bugs are NOT pointed.

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
- [ ] **Epic Link field** is set **if this Task is part of a larger Epic**
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

```text
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
3. Split oversized items (Stories at 13 points must split, consider splitting 8-point Stories; split Epics that are too large)
4. Update priorities based on team goals
5. Create parent issues (Epic/Feature/Initiative) only when needed for grouping or tracking

### With AI Assistance
1. Ask Claude to validate an issue: "Does GCP-XXX meet the Definition of Ready?"
2. Ask Claude to complete gaps: "Help me refine GCP-XXX to meet DoR"
3. Ask Claude to create compliant issues: "Create a Story for [feature] that meets DoR"
4. Let Claude know if work is standalone: "Create a standalone Story for [work] - no Epic needed"

---

## Related Documentation

- [Definition of Done](./definition-of-done.md) - Completion criteria for implemented work
- [Jira Story Template](./jira-story-template.md) - Story structure and sizing guide
- [Jira Epic Template](./jira-epic-template.md) - Epic structure and breakdown guide
- [Jira Feature Template](./jira-feature-template.md) - Feature structure
- [Jira Initiative Template](./jira-initiative-template.md) - Initiative structure
- [Jira Task Template](./jira-task-template.md) - Task structure
