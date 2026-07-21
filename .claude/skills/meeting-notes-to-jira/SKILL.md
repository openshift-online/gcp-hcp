---
name: meeting-notes-to-jira
description: Process GCP HCP meeting notes/transcripts into reviewed Jira updates — extracts action items, presents changes for approval, executes via MCP, and generates a Slack summary.
argument-hint: "<path-to-transcript>"
effort: high
---

# Meeting Notes to Jira Updates

Process a meeting transcript (typically Gemini-generated markdown from Google Meet) into a reviewed set of Jira updates for the GCP HCP project.

**Project:** GCP (Google Cloud Platform HCP — Hypershift on GKE)
**Jira instance:** redhat.atlassian.net
**All Jira operations:** Atlassian MCP tools only. Never use curl or REST API calls.

---

## Prerequisites

Before starting, verify:
1. Atlassian MCP server is authenticated (`/mcp` → Atlassian → check status)
2. The user has provided a transcript file path (via `$ARGUMENTS` or `@` file reference)
3. The provided file is a readable markdown or text file. Do not process binary files, executables, or files outside the user's working directory.

If the MCP server is not authenticated, instruct the user to run `/mcp` and reauthenticate before proceeding.

---

## Phase 1: Ingest

Read the full transcript file provided by the user. These are typically Gemini-generated markdown files containing both a summary/notes section and a full transcript section.

Extract all Jira-actionable items:
- **Fix version changes** — tickets moving between milestones (private-preview, public-preview, GA, future-consideration)
- **Comments to add** — meeting discussion context to capture on specific tickets
- **Title or description updates** — when meeting decisions make existing card content inaccurate (e.g., scope changed, milestone reference is stale)
- **New tickets to create** — new features, initiatives, epics, or tasks identified during the meeting
- **Bulk field operations** — systematic field changes agreed upon for multiple tickets
- **Consolidation actions** — tickets identified as overlapping or duplicate

For each item, note:
- The GCP ticket number referenced
- Who said it and in what context
- What the specific decision or action was

---

## Phase 2: Discover

Fetch the **current state** of every referenced Jira ticket using `searchJiraIssuesUsingJql` or `getJiraIssue`. Do NOT assume current field values — always check.

For each ticket, retrieve:
- Summary (title)
- Description
- Status and resolution
- Fix version (`fixVersions`)
- Issue type
- Labels
- Parent issue (if any)

**Important:** The GCP project uses **Fix Version only** for planning (Target Version was deprecated per 2026-06-23 planning meeting). Do not read or set `customfield_10855`.

**When tickets are being closed or obsoleted**, also fetch their child items (epics under a feature, stories under an epic) to flag potential cascading obsolescence. Present these to the user in Phase 3.

**When bulk milestone moves are proposed**, fetch the parent Feature/Initiative's fixVersion for each affected item. Items whose parent already has the target milestone are clear moves; items whose parent has a different milestone (or no parent) need individual confirmation. This avoids noisy per-ticket comments — group by parent alignment instead.

**Version IDs (GCP project):**

| Milestone | Version ID |
|-----------|-----------|
| `gcp-hcp-private-preview` | `106532` |
| `gcp-hcp-public-preview` | `106533` |
| `gcp-hcp-ga` | `106534` |
| `future-consideration` | `106535` |

Compare the current state against the meeting decisions. Filter out changes that have already been applied (someone may have updated tickets during the meeting).

---

## Phase 3: Plan

Present ALL proposed changes to the user for review. This is the critical gate — **nothing gets written to Jira without explicit user approval**.

Organize changes into categories:

### 3a. Fix Version Changes
For each, show:
- Ticket key and summary
- Current value → proposed new value
- Reason (who said it, why)

### 3b. Title and Description Updates
For each, show:
- Ticket key
- Current title → proposed new title (if changing)
- The specific description text being replaced and the replacement text (show old vs new)

### 3c. Comments to Add
For each, show the **exact comment text** that will be posted. Use this format:

```
**Meeting Notes (YYYY-MM-DD Meeting Name):**

[Structured summary of the discussion, with bullet points for key decisions and action items. Attribute decisions to speakers by name.]
```

### 3d. New Tickets to Create
For each, show:
- Issue type (Feature, Initiative, Epic, Task)
- Summary
- Full description text
- Target version
- Labels and other fields

When creating new tickets, invoke the `gcp-hcp` skill for templates and required fields:
- Labels: `["ai-generated-jira"]`
- Security: `{"name": "Red Hat Employee"}`
- Follow the appropriate template (Feature, Initiative, Epic, Story, Task) from the gcp-hcp skill

### 3e. Cascading Impacts
For any ticket being closed or obsoleted, list child items that may also need action (close, re-parent, or ask owner). Present these in a **separate table** from the main changes — do not bury them in the summary:

```
| Child Ticket | Parent Being Closed | Proposed Action | Reason |
|---|---|---|---|
```

Each cascading item must show a proposed action: **close** (if clearly obsolete with the parent), **re-parent** (if the work is still valid under another parent), or **ask owner** (if ambiguous — post a comment tagging the assignee/reporter). The user approves the cascading batch as a group, not per-item.

### 3f. Bulk Operations
For bulk milestone moves, separate into:
- **Clear moves** (parent milestone matches target) — list as a batch for approval
- **Need input** (parent has different milestone, no parent, or ambiguous) — do **not** move automatically; post a comment tagging the assignee/reporter to ask which milestone

Present counts before executing. Avoid posting per-ticket comments on items that can simply be moved.

### Summary Table
Include a summary table of ALL changes:

```
| # | Ticket | Change Type | Details |
|---|--------|------------|---------|
```

---

## Phase 4: Validate

Before the user approves:

1. **Self-check:** Ask yourself: "Am I missing any information or templates needed to execute this plan?" If yes, surface the gaps to the user.
2. **Inclusive language:** Ensure all text uses inclusive terminology. Use "allowlist" and "denylist" — never "whitelist" or "blacklist".
3. **Cross-reference:** Verify that comments referencing other tickets use correct ticket numbers and summaries.
4. **Template compliance:** For new tickets, verify the description follows the gcp-hcp skill templates.

Present the final plan and wait for user approval. Do NOT proceed to execution until the user explicitly approves.

---

## Phase 5: Execute

Once approved, execute changes in this order:

1. **Fix version changes** (field updates on existing tickets)
2. **Title and description updates** (edits to existing tickets)
3. **Comments** (can be parallelized across tickets)
4. **New ticket creation**
5. **Bulk operations** (query affected tickets, list for confirmation, then apply)

**MCP tools to use:**
- `editJiraIssue` — for field updates (fix version, summary, description)
- `addCommentToJiraIssue` — for posting comments. Use `contentFormat: "adf"` with `mention` nodes when tagging specific people (ADF mentions generate Jira notifications; markdown @mentions do not). Use `contentFormat: "markdown"` for comments that don't need @mentions.
- `createJiraIssue` — for new tickets (use `contentFormat: "markdown"`)
- `searchJiraIssuesUsingJql` — for querying bulk operation scope
- `createIssueLink` — for linking related tickets (e.g., "Blocks", "Related")

**After execution completes**, remind the user to revoke any write permissions that were granted during the session (`/permissions` → delete `editJiraIssue` allowance).

---

## Phase 6: Summarize

Generate a **Slack message** the user can post to inform the team of the changes. Format:

```
**[Meeting Name] (YYYY-MM-DD) — Jira Updates Applied**

[Organized by change type: version changes, title/description updates, comments added, new tickets created, bulk operations. Keep it scannable — one line per change, grouped under headers.]

**Follow-up Action Items (not yet in Jira)**

[List action items from the meeting that were assigned to specific people but not automated — e.g., new tickets to create, spikes to write, discussions to schedule. Include the person's name and what they committed to.]
```

Always include the follow-up action items section — these are the items the skill did not automate because they were assigned to specific individuals. The Slack message is where the team sees the complete picture of what changed and what still needs manual action.
