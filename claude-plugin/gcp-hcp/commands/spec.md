---
description: "Decompose a Jira Feature into Epics, or an Epic into Stories"
arguments: "ISSUE_KEY - The Jira issue key (e.g., GCP-123) to decompose"
user_invocable: true
---

Invoke the `spec` agent using the Task tool with `subagent_type="spec"`.

Pass the following context to the agent:
- Issue key: Parse from $ARGUMENTS (required)
- Execution mode: Interactive (show decomposition plan, confirm before creating issues)

The agent will:

1. Read the Jira Feature/Epic description
2. Load appropriate templates (epic template for Features, story template for Epics), design decisions, and architecture context
3. Decompose following the hierarchy: Feature into Epics, Epic into Stories
4. Show the decomposition plan and ask for confirmation
5. Create child issues in Jira with proper labels, links, and fields
6. Post a summary comment on the parent issue

All child issues are created with the `ai-generated-jira` label for easy identification.
Issues are NOT assigned and NOT moved to a sprint.

## Examples

```text
/gcp-hcp:spec GCP-456     # Decompose Feature GCP-456 into Epics
/gcp-hcp:spec GCP-789     # Decompose Epic GCP-789 into Stories
```

$ARGUMENTS
