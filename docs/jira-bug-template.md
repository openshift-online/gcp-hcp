# Jira Bug Template for GCP HCP

Use this template to report broken or regressed functionality in the GCP HCP platform. Bugs describe something that worked (or was expected to work) but does not.

---

## When to Use This Template

**Use a Bug for:**
- Broken or regressed functionality
- Security vulnerabilities or data correctness issues
- Production incidents caused by a defect in the system
- Behavior that contradicts documented or agreed-upon design

**Use a Task instead for:**
- Planned improvements or new capabilities
- Process work, follow-ups, or action items
- Anything that was never expected to work differently

---

## Description of Problem

[Clear, detailed description of what is broken. Include what you were trying to do, which component or feature is affected, and the observed impact.]

## Steps to Reproduce

1. [First step — be specific, include exact commands or configuration]
2. [Second step]
3. [Third step]

## Expected Behavior

[What should happen based on design, documentation, or prior behavior]

## Actual Behavior

[What actually happens — include exact error messages, symptoms, or unexpected output]

## Environment

| Field | Value |
|-------|-------|
| **Cluster name** | [e.g., `hcp-dev-us-east1-01`] |
| **HyperShift version** | [e.g., `4.17.3`] |
| **GKE version** | [e.g., `1.30.5-gke.1699000`] |
| **Cloud provider / region** | [e.g., `GCP / us-east1`] |
| **Config details** | [e.g., node pool size, network topology, relevant flags] |

## Error Logs / Screenshots

```
[Paste relevant log output here — use code blocks for readability]
```

[Add screenshots or links to dashboards if applicable]

## Impact Assessment

| Field | Value |
|-------|-------|
| **Severity** | [Blocker / Critical / Major / Normal / Minor] |
| **Affected users / clusters** | [e.g., "All clusters using CMEK", "2 dev clusters"] |
| **Workaround available?** | [Yes / No — if Yes, describe it briefly] |

---

## Acceptance Criteria

> Definition of Done: see [definition-of-done.md](definition-of-done.md) — Bugs section.

- [ ] Automated test added that verifies the fix (or justification documented in PR if not feasible)
- [ ] Root cause documented in PR description
- [ ] All tests pass — new and existing — no regressions introduced
- [ ] Code review approved (at least one approval received)
- [ ] Merged PR link added to this ticket before closing

---

## Worked Example

**Summary**: `NodePool reconciler panics when GCP service account is missing IAM binding`

**Description of Problem**: The NodePool reconciler crashes with a nil pointer panic when the GCP service account referenced in the HostedCluster spec is missing its required IAM binding. This prevents the node pool from being created and leaves the cluster in a broken state with no clear error in the HCP status.

**Steps to Reproduce**:
1. Create a HostedCluster referencing a GCP service account (`my-sa@project.iam.gserviceaccount.com`) that exists but does not have the `roles/iam.workloadIdentityUser` binding
2. Create a NodePool targeting that HostedCluster
3. Observe the hypershift-operator pod logs

**Expected Behavior**: The reconciler returns a descriptive error and sets a `Degraded` condition on the NodePool with the message "GCP service account is missing required IAM binding".

**Actual Behavior**: The reconciler panics with `runtime error: invalid memory address or nil pointer dereference` at `nodepool_controller.go:342`, crashing the operator pod.

**Environment**:
| Field | Value |
|-------|-------|
| **Cluster name** | `hcp-dev-us-east1-01` |
| **HyperShift version** | `4.17.3` |
| **GKE version** | `1.30.5-gke.1699000` |
| **Cloud provider / region** | `GCP / us-east1` |
| **Config details** | Workload Identity enabled, VPC-native networking |

**Error Logs**:
```
E0811 14:32:01.123456 1 nodepool_controller.go:342] panic: runtime error: invalid memory address or nil pointer dereference
goroutine 47 [running]:
hypershift/hypershift/hypershift-operator/controllers/nodepool.(*NodePoolReconciler).reconcileGCPNodePool(...)
```

**Impact Assessment**:
| Field | Value |
|-------|-------|
| **Severity** | Critical |
| **Affected users / clusters** | Any cluster with misconfigured GCP service account IAM bindings |
| **Workaround available?** | Yes — manually add the `roles/iam.workloadIdentityUser` binding to the service account |

**Acceptance Criteria**:
- [ ] Unit test covers the nil IAM binding case and asserts a `Degraded` condition is set
- [ ] Root cause (missing nil check before IAM binding fetch) documented in PR
- [ ] All existing NodePool tests pass
- [ ] Code review approved
- [ ] PR link added to GCP-NNN before closing
