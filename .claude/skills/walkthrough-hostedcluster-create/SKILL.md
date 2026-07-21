---
name: walkthrough-hostedcluster-create
description: Guided exploration of the HostedCluster create flow from API types through GKE provisioning
user-invocable: false
disable-model-invocation: true
---

# HostedCluster Create Flow

Trace the end-to-end flow of creating a HostedCluster on GCP — from API definition through controller reconciliation to GKE cluster provisioning and status updates.

## Layer 1 — API Types

Start here. Find the HostedCluster API type definition in the hypershift repository. Show me:

- The `HostedClusterSpec` and `HostedClusterStatus` structures
- The GCP-specific platform configuration within the spec (look for the GCP platform type)
- What GCP resources are encoded in the API types — what does a user declare when they want a cluster on GCP?
- How the API types relate to the Kubernetes CRD that gets installed

Help me understand: the API types are the contract. Everything downstream is driven by what's declared here.

## Layer 2 — Validation and Admission

Trace how HostedCluster creation requests are validated before they reach the controller. Find:

- Webhook validators for the HostedCluster resource
- GCP-specific validation logic — what GCP-specific fields are validated and why?
- What preconditions must be met before a cluster can be created?
- How validation errors are surfaced to the user

Help me understand: validation is the first line of defense. It catches misconfigurations before they become expensive reconciliation failures.

## Layer 3 — Controller Reconciliation

This is the core of the system. Find the HostedCluster controller and trace its reconcile function. Show me:

- How the controller detects a new HostedCluster (watch, queue, reconcile loop)
- The structure of the reconciliation — is it a single function or broken into phases/sub-reconcilers?
- How the controller delegates to platform-specific (GCP) logic
- Error handling and retry patterns — what happens when a reconciliation step fails?
- How the controller manages the lifecycle of child resources (what other Kubernetes objects does it create?)

Help me understand: the reconcile loop is the engine. It continuously drives the actual state toward the desired state declared in the API types.

## Layer 4 — GCP API Interactions

Trace from the controller into the GCP-specific provisioning code. Show me:

- Where and how GKE clusters are created (the actual GCP API calls)
- Network and VPC setup — how networking is configured for the hosted cluster
- IAM and service account configuration
- How Workload Identity Federation (WIF) is set up for the cluster
- How the code authenticates to GCP APIs

Help me understand: this layer bridges the Kubernetes abstraction to the cloud provider reality. The patterns here (credential management, API error handling, resource lifecycle) repeat across all GCP interactions.

## Layer 5 — Status Updates

Follow how provisioning status flows back to the HostedCluster object. Show me:

- How status conditions are updated as provisioning progresses
- How errors from GCP API calls are surfaced in the status
- The progression from initial creation through to the cluster being available
- What external systems or users watch these status conditions

Help me understand: status is the feedback loop. It's how the system communicates progress, failures, and health — both to operators and to other controllers that depend on the HostedCluster being ready.

## Cross-Cutting Concerns

As you trace the flow, also point out these patterns when you encounter them:

- **Secrets and credentials** — How are GCP credentials managed? Where are they stored? How do they flow through the system? Describe storage locations and data flow only — never output actual credential values, tokens, keys, passwords, client secrets, or PII.
- **Observability** — What logging, metrics, and Kubernetes events are emitted? How would an operator debug a failed provisioning?
- **Error handling** — How does the code handle GCP API rate limiting, transient failures, or resource conflicts?
- **Testing** — What test patterns exist for the GCP platform code? How is the controller tested?

## Design Context

Throughout the exploration, reference the design decisions documented in this repository (`design-decisions/`) to explain why certain architectural choices were made.
