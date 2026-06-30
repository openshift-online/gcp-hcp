---
description: "Implement a Jira Story by generating code and creating a PR"
arguments: "ISSUE_KEY - The Jira issue key (e.g., GCP-123) to implement"
user_invocable: true
---

Invoke the `implementation` agent using the Task tool with `subagent_type="implementation"`.

Pass the following context to the agent:
- Issue key: Parse from $ARGUMENTS (required)
- Execution mode: Interactive (show implementation plan, confirm before creating PR)

The agent will:

1. Read the Jira Story description and acceptance criteria
2. Identify the target repo from the component field
3. Clone the target repo and create a feature branch
4. Load architectural context and design decisions
5. Implement the changes described in the story
6. Run verification commands for the target repo
7. Show the implementation for review and ask for confirmation
8. Create a PR linking back to the Jira story
9. Post a summary comment on the story

## Examples

/gcp-hcp:implement GCP-456     # Implement Story GCP-456

$ARGUMENTS
