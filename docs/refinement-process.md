---
name: Refinement Process
description: How the GCP HCP team refines backlog items across all issue types — meetings, offline processes, and when refinement is optional.
---

# Refinement Process

***Scope***: GCP-HCP

**Date**: 2026-10-09

This document defines how the GCP HCP team refines backlog items to ensure they meet the Definition of Ready before moving to To Do status.

## What is Refinement?

Refinement is the collaborative process of taking a backlog item from initial idea to ready-for-work. A refined item has clear scope, acceptance criteria, dependencies identified, and enough detail that the team can confidently commit to delivering it.

**Goal**: Every item moving to To Do status must meet the [Definition of Ready](definition-of-ready.md) for its issue type.

**Key principle**: Refinement defines *how* items are reviewed and discussed. The *what* (Definition of Ready criteria) remains consistent across all refinement paths.

---

## Refinement Meetings

The GCP HCP team has two recurring refinement meetings:

### Weekly Backlog Refinement

**Cadence**: Weekly

**Scope**: Epics, Stories, Tasks, Spikes, and Bugs

**Process**:
- Team members move issues to **Refinement** status in Jira before the meeting
- View items in Refinement status using the [Refinement board filter](https://redhat.atlassian.net/jira/software/c/projects/GCP/boards/12273?quickFilter=13641)
- Meeting focuses on scoping, clarifying acceptance criteria, identifying dependencies, and confirming readiness
- Items meeting Definition of Ready move to **To Do** status
- Items needing more work stay in **Refinement** or move back to **New**

**Logistics**: See [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835/Team+Ceremonies+Calendar)

**Who attends**: Engineering team, PM, Architect

### Feature/Initiative Refinement

**Cadence**: Every 3 weeks

**Scope**: Features and Initiatives only

**Process**: See [Feature & Initiative Ownership Playbook](feature-initiative-playbook.md) for detailed workflow, ownership model, and refinement criteria.

**Logistics**: See [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835/Team+Ceremonies+Calendar)

---

## Refinement Paths by Issue Type

Different issue types have different refinement requirements. Some require the weekly meeting, some have alternatives, and some are optional.

| Issue Type | Refinement Path | Notes |
|------------|----------------|-------|
| **Feature** | Every-3-weeks meeting | Required. See [feature-initiative-playbook.md](feature-initiative-playbook.md) |
| **Initiative** | Every-3-weeks meeting | Required. See [feature-initiative-playbook.md](feature-initiative-playbook.md) |
| **Epic** | Weekly meeting | Required. Team reviews breakdown and scope |
| **Story** | Weekly meeting **OR** Offline (Slack) | Two paths available. See [Offline Story Refinement](#offline-story-refinement-slack-alternative) |
| **Task** | Weekly meeting | Required |
| **Spike** | Weekly meeting | Required. Team reviews research questions and time-box |
| **Bug** | **Optional** (author discretion) | Can skip refinement. See [Bug Refinement Policy](#bug-refinement-policy) |
| **Risk** | Dedicated process | See [risk-tracking-process.md](risk-tracking-process.md) |

---

## Offline Story Refinement (Slack Alternative)

**Effective**: 2026-10-09

Stories have **two refinement paths**. Authors can choose either:

1. **Weekly Backlog Refinement meeting** (traditional path), OR
2. **Offline refinement via Slack** (asynchronous alternative)

### When to Use Offline Refinement

Use the offline Slack path when:
- The Story is well-scoped and unlikely to need synchronous discussion
- Waiting for the next weekly meeting would delay progress unnecessarily
- The team can provide meaningful feedback asynchronously

### Offline Refinement Process

1. **Author posts the Story** to #team-gcp-hcp-eng using the **Request Ticket Review workflow** (available in the channel shortcuts menu)
   - Include a link to the Jira Story
   - Briefly explain what feedback is needed ("ready for +1s to move to To Do")

2. **Team reviews asynchronously** (24-hour window)
   - Any team member can provide a **+1** or ask questions
   - Blocking concerns or questions should be raised immediately

3. **After 24 hours + 2 +1s**, the Story can move to **To Do** status

4. **If blocking feedback is received**:
   - Address the feedback in Jira or Slack
   - Consider bringing the Story to the weekly meeting for synchronous discussion
   - Or revise and re-post for another 24-hour +1 window

### Requirements

- **Channel**: #team-gcp-hcp-eng
- **Workflow**: Use the **Request Ticket Review** Slack workflow automation
- **Time window**: 24 hours minimum
- **+1 count**: 2 +1s from any team members
- **Exceptions**: None — all Stories using the offline path must meet the 24-hour + 2 +1s requirement
- **Definition of Ready**: Story must still meet all [Definition of Ready criteria](definition-of-ready.md#definition-of-ready-story) before moving to To Do

### Examples

**Good candidate for offline refinement**:
- Story is clearly scoped with specific acceptance criteria
- Technical approach is straightforward
- Dependencies are identified and documented
- Estimated at 1-5 points

**Should use weekly meeting instead**:
- Story touches multiple components and needs design discussion
- Acceptance criteria are unclear or incomplete
- Team needs to align on technical approach
- Story is 8+ points and may need splitting

---

## Bug Refinement Policy

**Effective**: 2026-10-09

**Bug refinement is optional and at the author's discretion.**

### Rationale

Many bugs are straightforward — clear reproduction steps, known root cause, obvious fix. These bugs benefit from fast resolution without waiting for the weekly refinement meeting.

### When to Skip Refinement

Skip the weekly meeting when the bug:
- Has clear, reproducible steps
- Has a known or easily identified root cause
- Has an obvious fix approach
- Affects a single component with no cross-team dependencies
- Impact and scope are well-understood

### When to Bring Bugs to Refinement

Bring the bug to the weekly meeting when:
- Root cause is unclear or requires investigation
- Fix approach needs architectural input or design discussion
- Bug affects multiple components or has cross-team dependencies
- Impact or scope needs team assessment
- Severity/priority is unclear and needs team input

### Bug Visibility (Optional)

While bugs don't require refinement, authors may choose to share bugs in #team-gcp-hcp-eng for team visibility. This is **optional** — the decision is at the author's discretion.

### Definition of Ready Still Applies

Regardless of whether a bug goes through the weekly meeting or not, it must meet the [Definition of Ready criteria for Bugs](definition-of-ready.md#definition-of-ready-bug) before moving to To Do status.

**Bug DoR checklist** (from [definition-of-ready.md](definition-of-ready.md)):
- [ ] Title clearly describes the problem
- [ ] Description includes:
  - [ ] **Steps to Reproduce** (clear, numbered steps)
  - [ ] **Expected Behavior** (what should happen)
  - [ ] **Actual Behavior** (what actually happens)
  - [ ] **Environment** (where the bug occurs: dev, stage, prod)
  - [ ] **Impact** (who is affected, severity of impact)
- [ ] Priority is set based on severity and impact
- [ ] Epic Link is set **if this Bug fix is part of a larger Epic**
- [ ] Assignee is assigned (for critical/blocker bugs)
- [ ] Bug is reproducible OR includes sufficient diagnostic information
- [ ] Root Cause is identified or hypothesized (if known)

See [Definition of Ready - Bug](definition-of-ready.md#definition-of-ready-bug) for complete criteria.

---

## Relationship to Definition of Ready

**Critical principle**: All items must meet the [Definition of Ready](definition-of-ready.md) criteria for their issue type before moving to To Do, **regardless of which refinement path is used**.

| Refinement Path | Definition of Ready |
|-----------------|---------------------|
| Weekly Backlog Refinement meeting | Must meet DoR ✅ |
| Feature/Initiative Refinement meeting | Must meet DoR ✅ |
| Offline Story Refinement (Slack) | Must meet DoR ✅ |
| Bug (no refinement) | Must meet DoR ✅ |

The refinement process defines *how* items are reviewed (meeting vs. Slack, required vs. optional). The Definition of Ready defines *what* makes an item ready for development.

**If an item doesn't meet DoR, it cannot move to To Do** — even if it went through the weekly meeting or received 2 +1s in Slack.

---

## Quick Reference

| Issue Type | Meeting | Offline (Slack) | Optional | DoR Required |
|------------|---------|----------------|----------|--------------|
| Feature | Every 3 weeks | ❌ | ❌ | ✅ |
| Initiative | Every 3 weeks | ❌ | ❌ | ✅ |
| Epic | Weekly | ❌ | ❌ | ✅ |
| Story | Weekly | ✅ (24h + 2 +1s) | ❌ | ✅ |
| Task | Weekly | ❌ | ❌ | ✅ |
| Spike | Weekly | ❌ | ❌ | ✅ |
| Bug | Weekly (if needed) | ❌ | ✅ (author discretion) | ✅ |
| Risk | Dedicated process | ❌ | N/A | See [risk-tracking-process.md](risk-tracking-process.md) |

---

## Related Documentation

- [Definition of Ready](definition-of-ready.md) — Readiness criteria for all issue types
- [Definition of Done](definition-of-done.md) — Completion criteria for implemented work
- [Feature & Initiative Ownership Playbook](feature-initiative-playbook.md) — Feature/Initiative workflow and refinement
- [Team Ceremonies Calendar](https://redhat.atlassian.net/wiki/spaces/GCP/pages/424214835/Team+Ceremonies+Calendar) — Meeting schedules (Confluence)
- [Jira Story Template](jira-story-template.md) — Story structure and sizing guide
- [Jira Bug Template](jira-bug-template.md) — Bug reporting structure
