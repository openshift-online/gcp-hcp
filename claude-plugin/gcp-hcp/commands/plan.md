---
description: "Create a detailed implementation plan for a Jira Story"
arguments: "ISSUE_KEY - The Jira issue key (e.g., GCP-123) to plan"
user_invocable: true
---

Invoke the `plan` agent using the Task tool with `subagent_type="plan"`.

Pass the following context to the agent:
- Issue key: Parse from $ARGUMENTS (required)
- Execution mode: Interactive (show plan, confirm before creating PR)

The agent will:

1. Read the Jira Story description and acceptance criteria
2. Identify the target repo from the component field
3. Clone the target repo (read-only) and load architectural context
4. Analyze the story requirements and explore the target codebase
5. Write a detailed implementation plan at `implementation-plans/<story-key>-<desc>.md`
6. Show the plan and ask for confirmation
7. Create a PR in gcp-hcp with the plan document
8. Post a summary comment on the story

The plan document guides the Implementation Agent (`/gcp-hcp:implement`) when it later implements the story.

## Examples

```text
/gcp-hcp:plan GCP-456     # Create implementation plan for Story GCP-456
```

$ARGUMENTS
