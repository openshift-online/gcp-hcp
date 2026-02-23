# Jira Story Template for GCP HCP

This template provides a standardized structure for creating Jira stories for infrastructure projects. It ensures developers have consistent and clear work requirements, making backlog refinement more efficient and reducing ambiguity during implementation.

---

## User Story

**As a** [platform user/developer/operations team/end user]
**I want** [goal/desire],
**so that** [benefit/reason].

_[Optional placeholder for another user story if this deliverable serves multiple users]_

## Context / Background

[Current state, problem being solved, relevant history, links to related tickets/incidents]

## Requirements

[Functional and non-functional requirements (performance, scalability, reliability, SLOs, compliance)]

## Technical Approach

[Proposed solution, technologies/tools, major steps, alternatives considered]

## Dependencies

[Blocking items: other teams/stories, external vendors, infrastructure/access needs, required approvals]

## Additional Context

[Any relevant background, links, screenshots, or technical notes]

## Story Sizing Guide

Use this guide to estimate the size of a story during refinement. Story sizes reflect complexity, risk, and effort combined. Story points use the Fibonacci sequence (0, 1, 2, 3, 5, 8, 13) to reflect increasing uncertainty as size grows. Stories should typically be **1-5 points**. Stories sized at 8+ should be split into smaller stories.

### Pointing Criteria

| Points | Description |
|--------|-------------|
| **0** | Rarely used. Trivial task with stakeholder value but less risk/complexity than a 1-pointer. Example: Update a README link. |
| **1** | The smallest issue possible, everything scales from here. Can be a one-line change in code, a tedious but extremely simple task, etc. Basically, no risk, very low effort, very low complexity. |
| **2** | Simple, well-understood change. Low risk, low complexity but slightly more effort than a 1. Some investigation into how to accomplish the task may be necessary. |
| **3** | Doesn't have to be complex, but it is usually time consuming. The work should be fairly straightforward. There can be some minor risks. |
| **5** | Requires investigation, design, discussions, collaboration. Can be time consuming or complex. Risks involved. |
| **8** | Big task. Requires investigation, design, discussions, collaboration. Solution is challenging. Risks expected. Design doc required. **Consider splitting into smaller stories.** |
| **13** | Ideally, this shouldn't be used. If you ever see an issue that is this big, **it must be split into smaller stories**. |

### Story Point Examples (GCP HCP Context)

**1 Point Examples**:
- Add a new environment variable to an existing operator deployment
- Update GKE node pool version in terraform config
- Fix a typo in HyperShift API field documentation
- Add a simple validation check to an existing function

**2 Point Examples**:
- Implement a new Prometheus metric for hosted control plane CPU usage
- Add retry logic to GCP API client with exponential backoff
- Create a simple e2e test for GKE cluster creation
- Update RBAC rules to allow service account access to a new resource type

**3 Point Examples**:
- Implement health checks for all management cluster components
- Add automated cleanup of orphaned GCP resources after cluster deletion
- Refactor logging configuration to use structured logging library
- Implement GCP service account impersonation for a specific workload

**5 Point Examples**:
- Design and implement a new controller to manage GCP firewall rules for hosted clusters
- Add support for customer-managed encryption keys (CMEK) to a storage component
- Implement automated backup and restore for management cluster state
- Migrate an existing operator from in-cluster to out-of-cluster deployment pattern

**8 Point Examples** (Should be split):
- Implement full observability stack (metrics, logging, tracing) for hosted control planes
- Add support for VPC-native GKE clusters with IP aliasing and network policies
- Migrate entire CI/CD pipeline from one platform to another

### When to Split Stories

Split a story if any of these apply:

**By scope**:
- More than 5 acceptance criteria
- Touches more than 3 components or repositories
- Contains both investigation (spike) AND implementation work
- Has internal sequencing (step 1 must complete before step 2 begins)

**By layer**:
- API changes + controller implementation + CLI updates → 3 stories
- Backend logic + frontend UI + documentation → 3 stories

**By workflow step**:
- Create + Read + Update + Delete operations → 4 stories (or start with Create + Read)

**By component**:
- Changes needed in multiple operators → 1 story per operator
- Changes needed across multiple GCP services → 1 story per service

**By risk**:
- Separate proof-of-concept spike from production implementation
- Separate migration work (risky) from new feature work (lower risk)

### Splitting Strategies

When splitting a large story, consider these approaches:

1. **Vertical slices**: Each story delivers end-to-end value for a subset of functionality
   - "As a user, I can create a cluster with default settings" → Story 1
   - "As a user, I can create a cluster with custom network settings" → Story 2

2. **Technical layers**: Split by component or layer (use sparingly, prefer vertical slices)
   - API design and implementation → Story 1
   - Controller implementation → Story 2
   - CLI integration → Story 3

3. **Spike + Implementation**: Separate research from execution
   - "Investigate options for GCP Workload Identity Federation" → Spike
   - "Implement WIF for HyperShift operator" → Story

4. **Incremental delivery**: Build complexity over multiple stories
   - "Implement basic health check endpoint" → Story 1
   - "Add detailed health metrics to endpoint" → Story 2
   - "Add automated alerting based on health metrics" → Story 3

## Acceptance Criteria

- [ ] [Specific testable outcome 1]
- [ ] [Specific testable outcome 2]
- [ ] [Specific testable outcome 3]
