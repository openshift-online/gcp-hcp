---
name: Definition of Done
description: The definition of “done” (DoD) is a checklist of activities that the team can realistically commit to completing for each story/bug as a means of asserting that the work is completed.
tools: Read, Grep, Glob, Task
model: sonnet
---

# Definitions of Done

## Definition of Done: Story

In addition to meeting the requirements and any acceptance criteria from the Jira ticket, the developer must be able to check off the following activities for the story to be considered “done”:

1. Story satisfies all acceptance criteria
2. Test automation complete, where applicable:
   1. Unit test coverage at >= 85% and passing
   2. Integration tests added and passing
   3. e2e test added and passing
3. PR for code changes has been merged
4. AI-Assisted Development: Human-in-the-Loop Guidelines are followed (e.g. commit message conventions)  
5. PR for relevant architecture and design doc changes has been merged.  
6. Deployment to stage (once we have a stage platform\!)  
7. Story is demo-able for end of sprint

## Definition of Done: Spike

1. Spike findings are documented 
2. Decision is made and documented in the relevant design decision/architecture docs.
3. Resulting backlog items are created

## Definition of Done: Bugs

1. Test Added
    - Automated test included that verifies the fix
    - If not feasible, document why in the PR
2. Root Cause Documented
    - PR description explains what caused the bug
3. All Tests Pass
    – New and existing tests pass
    - No regressions introduced
4. Code Review Approved
    - At least one approval received
5. Ticket Closed
    - Link to merged PR added to bug ticket

## Definition of Done: Task

1. All acceptance criteria in the ticket are satisfied
2. Deliverables (documents, configs, decisions, scripts) are in their expected final state
3. If code changes: PR merged, all tests pass, no regressions
4. If documentation changes: PR merged to main
5. Ticket closed with a link to the relevant PR, doc, or output

## Definition of Done: Epic

1. All child Stories are Closed
2. All acceptance criteria in the epic ticket are satisfied
3. No open blocking issues remain linked to the epic
4. Any documentation updates referenced in the epic scope are merged
5. Parent Feature or Initiative ticket reflects the epic's completion

## Definition of Done: Feature

1. All child Epics are Closed
2. All acceptance criteria in the feature ticket are satisfied
3. End-to-end validation confirms the feature works as intended in a representative environment
4. Stakeholder demo completed if Demo Critical = Yes (or explicitly waived with documented rationale)
5. Product documentation updated (if Product Documentation Required = Yes), or field explicitly set to No
6. Resolution field is set; final Jira comment summarizing the outcome is added

## Definition of Done: Initiative

1. All child Epics are Closed (or any deferred scope documented with rationale)
2. Strategic objectives and success criteria stated in the initiative are met — or any unmet targets have documented variance with approved follow-up
3. Internal impact delivered — efficiency, reliability, scalability, or developer experience improvements are demonstrable
4. Architecture and design documentation updated to reflect the initiative's outcomes
5. Resolution field is set; final Jira comment summarizing the outcome is added

## Definition of Done: Risk

A Risk is Closed when one of the following is true:

1. **Mitigated:** Mitigation plan fully executed; linked mitigation stories/tasks are Closed
2. **Accepted:** Team has explicitly accepted the risk with rationale documented in the ticket
3. **No longer applicable:** Risk is moot; reason documented in the ticket
4. **Materialized and handled:** Risk occurred and was addressed via incident response or equivalent

In all cases, a comment is added explaining the closure rationale.

