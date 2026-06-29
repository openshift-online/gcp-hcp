---
name: ai-sdlc-pilot-update
description: Post a biweekly Agentic SDLC pilot update comment to GCP-579, gathering Jira and GitHub activity for the lookback period.
arguments: days
argument-hint: "[days]"
disable-model-invocation: true
model: sonnet
effort: high
---

# Agentic SDLC Pilot Update Skill

Gather data from Jira and GitHub, draft a biweekly update comment, get user input on subjective sections, and post the final comment to GCP-579.

**Tracking issue:** GCP-579 ("Establish AI-Native Developer Experience — Agentic SDLC Pilot")
**Deadline:** June 30, 2026

---

## Phase 1: Gather Jira Activity (BFS traversal)

Determine the lookback window. Default is 14 days. If `$days` was provided, use that value instead. If `$days` is missing, non-numeric, or less than 1, fall back to 14.

Compute:
- `end_date` = today's date
- `start_date` = today minus lookback days

**BFS traversal starting at GCP-579:**

Use a queue-based BFS. Start with `["GCP-579"]`.
Before traversal, fetch and store GCP-579's own fields (summary, description, status, issuetype, assignee, created, updated).
For each key in the queue:
1. Search Jira for direct children using JQL: `parent = KEY ORDER BY updated DESC`
   - Fetch up to 50 results per query; paginate if more exist (pagination applies per parent-key search, not once globally)
2. Fetch each child's key, summary, description, status, assignee, issuetype, created, updated fields
3. Add each child's key to the queue
4. Continue until the queue is empty

**After collecting all keys in the hierarchy (including GCP-579 itself, with fields stored for each):**

For each discovered key (in groups of 20), fetch the issue's changelog and extract status transitions. Filter transitions to those whose timestamp falls within the lookback window.

**Classify findings:**

- **Completed issues**: status transitioned to "Done", "Closed", or "Resolved" within the window
- **Created issues**: `created` field is within the window
- **Status transitions**: any status change within the window (excluding completions)
- **Blocked issues**: status is "Blocked" or summary/description contains "blocked" (case-insensitive)

---

## Phase 2: Gather GitHub Activity

**Repos to search:**
- `openshift-online/gcp-hcp`
- `openshift-online/gcp-hcp-infra`
- `openshift-eng/ai-helpers`
- `openshift-online/gcp-hcp-priv` (may have restricted access; skip gracefully if unavailable)

For each repo, use `mcp__plugin_github_github__search_pull_requests` with these queries:

**Merged PRs (last N days):**
```text
repo:<owner>/<repo> is:pr is:merged merged:>={start_date}
```

**Open PRs (in progress):**
```text
repo:<owner>/<repo> is:pr is:open updated:>={start_date}
```

**Filter to pilot-relevant PRs** by checking if any of these apply:
- Has label `agentic-sdlc-pilot`
- Touches paths: `claude-plugin/`, `.claude/`, `CLAUDE.md`, `AGENTS.md`, skills, agents
- Title or body mentions: "SDLC", "pilot", "agentic", "skill", "claude", "AI"

**For PRs in shared repos** (`openshift-eng/ai-helpers` and any repo not fully owned by the team), additionally require that the PR author is a known GCP HCP team member. Known GitHub handles:
`apahim`, `cblecker`, `ckandag`, `cristianoveiga`, `floresroger`, `jimdaga`, `patjlm`, `kkeane`, `billmvt`

Skip PRs from other authors in shared repos even if they match the pilot-relevance filters.

If the repo `openshift-online/gcp-hcp` is the current working directory, optionally supplement with:
```bash
git log --since="{start_date}" --oneline --no-merges
```

Collect for each pilot-relevant PR: number, title, state, merged_at or created_at, repo name.

---

## Phase 3: Draft the Update

Assemble gathered data into the 5-section template below.

**Auto-populate "What We Tried"** from:
- New Jira issues created in the window (list as: `[KEY] Summary — issuetype`)
- PRs opened (not yet merged)
- Tools, workflows, or skills referenced in PR titles/descriptions/Jira summaries

**Auto-populate "What Happened"** from SDLC process changes only:
- Jira issues completed (status → Done/Closed/Resolved) where the issue is about process, tooling, or documentation — not feature work
- Status transitions on process-related issues (DoD, DoR, templates, pilot tracking)
- Org directives or decisions that change how the team works
- PRs whose *primary purpose* is SDLC tooling (e.g., new skills, agents, pilot infrastructure)

Do NOT list PRs that are ordinary feature or bug work produced using the AI-assisted workflow — the fact that they were AI-generated is not itself a "What Happened" event.

**Auto-populate all five sections** from gathered context:

- **What We Learned**: synthesize from meeting notes, cross-team sessions, and recurring themes in Jira/PR activity (e.g., bottlenecks, surprises, process gaps)
- **What's Blocked**: extract from Jira issues with "Blocked" status, open dependencies, or explicit blocker language in issue descriptions or PR comments
- **What We're Trying Next**: derive from open Jira issues in "To Do" or "Refinement" state, F2F action items, and unmerged pilot PRs

Present the complete draft — all five sections populated — and invite the user to correct, add, or remove content using the same review loop.

Remove the AskUserQuestion prompt for these sections. If context is insufficient to populate a section, use "Nothing to report this period." and note what data was missing.

**Deduplication rule:** If a PR or Jira issue is already mentioned in "What We Tried," only carry it into "What Happened" if there is a distinct new event to report (e.g., it merged, it was completed, it transitioned to a new status). Do not list it in "What Happened" solely because it exists and is open.

---

## Phase 4: Review and Confirm

Assemble the complete comment using the template:

```markdown
## Agentic SDLC Pilot Update — {start_date} to {end_date}

### What We Tried
{auto-populated bullet list: tools, workflows, issue types worked on, PRs opened}

### What Happened
{auto-populated bullet list: PRs merged, issues completed, status changes}

### What We Learned
{auto-populated from Jira/PR activity and meeting notes}

### What's Blocked
{auto-populated from blocked issues and open dependencies}

### What We're Trying Next
{auto-populated from open issues and action items}
```

Display the full formatted Markdown in the terminal. Ask the user to confirm or request edits:

```text
Here is the full draft comment for GCP-579. Reply with:
- "post" to post as-is
- Specific edits to apply (e.g., "change the third bullet in What Happened to...")
- "cancel" to abort
```

Apply any requested edits and re-display until the user confirms with "post".

---

## Phase 5: Post the Comment

Post the confirmed comment to Jira issue GCP-579.

Confirm success with: "Comment posted to GCP-579."

If posting fails, display the full markdown so the user can copy-paste it manually.

---

## Error Handling

- **Repo not accessible** (e.g., `gcp-hcp-priv`): skip that repo, note it was skipped in the output.
- **No pilot-relevant PRs found**: include a note "No pilot-relevant PRs found in this period" in "What Happened".
- **BFS finds no children under GCP-579**: report only GCP-579 itself; this is valid.
- **Jira batch changelog limit**: if >20 keys, batch in groups of 20.
- **User provides empty input for a subjective section**: use "Nothing to report this period." as the placeholder.
