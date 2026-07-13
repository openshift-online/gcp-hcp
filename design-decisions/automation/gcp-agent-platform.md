# Gemini Enterprise Agent Platform for Hosted Agent Workloads

***Scope***: GCP-HCP

**Date**: 2026-07-09

## Decision

Use Gemini Enterprise Agent Platform (Agent Runtime) as the standard hosting platform for production-deployed AI agents. Google's Agent Development Kit (ADK) is the preferred agent framework. Microsoft's Agent Governance Toolkit (AGT) is required for tool-level governance on any agent with write access to external systems.

Companion agents on Cloud Run are an approved exception when an agent requires filesystem write access (see A2A Coordination below). Cloud Run companions must still meet all governance, observability, and authentication requirements defined in this document.

This decision applies to hosted/production agents only. Local developer agents (Claude Code, OpenCode, Gemini CLI, etc.) are explicitly out of scope.

## Context

- **Problem Statement**: The team is building autonomous agents that interact with production systems — GitHub, Jira, GCP resources — on schedules or in response to events, without a human in the loop. As the team builds more agents, each would otherwise make ad-hoc infrastructure, framework, and security decisions. We need a standard platform and governance model before agent workloads proliferate.

- **Constraints**:
  - Must align with the team's existing GCP-native infrastructure (no new cluster operations)
  - Must satisfy the [5-layer agent security model](https://medium.com/@rbean_3467/what-even-is-the-harness-f21336768a80) adopted by the Global Engineering Agentic SDLC Working Group (Infrastructure → Sandbox → Harness → Runtime → Model)
  - Must support Workload Identity for model authentication — no static API keys. Each agent runs as a dedicated GCP service account with Workload Identity providing keyless access to Vertex AI. Agent Identity (`identity_type=AGENT_IDENTITY`) may be adopted as it matures for per-agent identity beyond the service account level
  - Must provide network-level egress control for agents with access to secrets
  - Team is small; operational overhead for agent infrastructure must be near zero

- **Assumptions**:
  - Agent workloads will grow as the team automates more operational and development workflows
  - The platform will continue to mature — Agent Gateway (currently in preview) may eventually consolidate enforcement and observability into a single layer
  - ADK's open-source, model-agnostic design will remain stable as the framework evolves through major versions

## Alternatives Considered

1. **Gemini Enterprise Agent Platform (chosen)**: Fully managed Agent Runtime with built-in compute isolation, auto-scaling, identity, observability, and scheduling. In the source-archive deployment mode (the required mode for this team), agent code is deployed into a managed container with no shell, no exec, and no writable filesystem. Agent Runtime also supports custom container deployments, which may have different capabilities — custom containers are not approved under this decision without separate review. Integrated with Cloud Trace, Cloud Logging, and Cloud Monitoring.

2. **Cloud Run**: Google's scalable managed container platform. Not purpose-built for agents — lacks native agent observability (no execution traces, session management, or memory bank), no built-in multi-agent orchestration, and no framework-aware deployment. Would require building agent lifecycle management, session state, scheduling triggers, and telemetry instrumentation from scratch. A general-purpose compute platform adapted for agents rather than an agent-first platform.

3. **OpenShell on GKE**: An agent sandbox providing full Linux environments with kernel-level security controls — Landlock LSM restricts filesystem paths, seccomp BPF filters system calls, and a credential proxy replaces real keys with opaque placeholders so the agent process never sees them. Powerful isolation, but requires operating a standard GKE cluster, managing sidecars, maintaining proxy infrastructure, and running the OpenShell control plane. Significant operational overhead for a small team that currently runs no standard GKE clusters — all existing infrastructure uses GKE Autopilot or managed services.

## Decision Rationale

* **Justification**: Agent Platform provides a fully managed, agent-first hosting environment with zero cluster overhead. In source-archive deployment mode, the sandbox properties that OpenShell achieves through kernel-level enforcement are achieved architecturally: the managed container exposes no shell, no exec capability, and no writable filesystem — there is nothing to subtract because dangerous capabilities are never provided. Workload Identity handles model authentication natively, eliminating API keys from the environment entirely. Cloud Logging + Trace deliver observability out of the box with OpenTelemetry support.

* **Evidence**: Production deployments on the platform have validated the architecture across all five security layers:
  - Agent Runtime manages compute, scaling, and execution lifecycle with no cluster operations
  - Workload Identity provides keyless authentication to Vertex AI models
  - Secret Manager with per-secret IAM bindings isolates tool credentials (GitHub, Jira tokens)
  - Private Service Connect + Secure Web Proxy enforces deny-all egress with an explicit domain allowlist
  - AGT allow/deny lists and content filters provide in-process tool governance
  - Cloud Trace captures full execution traces across multi-agent orchestration

* **Comparison**:
  - **Cloud Run** (alternative 2): Viable as general-purpose compute, but every agent-specific capability (session management, multi-agent orchestration, execution traces, memory) would need to be built and maintained by the team. For a small team, the build-vs-buy calculus strongly favors the managed platform. Cloud Run also lacks the inherent sandbox properties — a container image on Cloud Run can include a shell and writable filesystem unless explicitly hardened.
  - **OpenShell on GKE** (alternative 3): Provides the strongest isolation model (kernel-level enforcement, credential proxy injection), but requires operating a standard GKE cluster. The team runs GKE Autopilot exclusively and has no standard GKE clusters. Standing up and maintaining the OpenShell infrastructure (cluster, sidecars, proxy) would be a significant operational burden for a platform that Google already manages. Agent Platform achieves equivalent security outcomes through different mechanisms at each layer.

## Consequences

### Positive

* Zero operational overhead for agent infrastructure — Google manages compute, scaling, container lifecycle, and platform updates
* Built-in sandbox properties in source-archive mode: no shell, no exec, no writable filesystem, read-only deployment
* Workload Identity eliminates model API keys from the environment; tool credentials in Secret Manager with per-secret IAM
* Native observability: Cloud Trace (OpenTelemetry), Cloud Logging, Cloud Monitoring — no instrumentation required
* ADK is open-source and model-agnostic (Gemini, Anthropic, OpenAI) — agent code is portable if we ever need to change platforms
* Cloud Scheduler integration enables scheduled agent execution with no additional infrastructure
* Agent Platform Sessions and Memory Bank provide managed state and long-term context across agent invocations

### Negative

* Agent Runtime's read-only filesystem means agents cannot write to disk — use cases requiring filesystem access (code generation, build tooling) need a secondary compute target (see A2A pattern below)
* Platform is relatively new — rebranded to Gemini Enterprise Agent Platform at Google Cloud Next '26 (April 2026)
* Agent Gateway is still in preview; until it GAs, enforcement and observability remain split across AGT (in-process) and SWP (network-level)
* AGT runs in-process, not at the kernel level — a compromised Python runtime could theoretically bypass tool governance. SWP independently enforces destination-level egress control (the agent can only reach allowlisted domains), but SWP does not enforce which API operations are called on those domains. A bypassed AGT with valid credentials could perform unauthorized operations on allowed endpoints. This is partially mitigated by the platform providing no shell or exec capabilities, and by per-secret IAM scoping credentials to the minimum required
* Network egress enforcement requires VPC + Secure Web Proxy setup via Terraform — not zero-config, though the modules are reusable across agents
* Platform lock-in for infrastructure (deployment, scheduling, observability) — mitigated by ADK portability for the agent code itself

## Framework and Governance Guidance

### Agent Development Kit (ADK) — Preferred Framework

ADK is the preferred framework for all new agents deployed on Agent Platform. Key properties:

- **Open-source** ([adk.dev](https://adk.dev)) with active development by Google
- **Multi-language**: Python, Go, Java, TypeScript
- **Model-agnostic**: First-class support for Gemini, Anthropic Claude, OpenAI, and others via the `BaseLlm` interface
- **Multi-agent orchestration**: Built-in support for hierarchical agent teams, sub-agent delegation, sequential/parallel/loop workflows, and graph-based orchestration
- **First-class Agent Runtime support**: `adk deploy agent_engine` handles packaging, container build, and deployment
- **Portable**: ADK agents run anywhere Python (or Go/Java/TS) runs — local development, Cloud Run, GKE, or Agent Runtime

Teams may use alternative frameworks (LangGraph, custom containers) if ADK does not fit their use case, but must document the justification and ensure the alternative integrates with Agent Runtime's deployment and observability stack.

### Agent Governance Toolkit (AGT) — Required for Write Access

AGT is **required** for all agents that have write access to external systems (GitHub API operations, Jira task creation, PR approvals, infrastructure changes). Configuration must include:

- **Allow list**: Enumerate every tool the agent is permitted to invoke
- **Deny list**: Explicitly block dangerous operations (`execute_code`, `shell_exec`, `eval`, `delete_file`, `write_file`, `read_file`, `create_file`)
- **Content filters**: Block tool calls containing sensitive data patterns (API tokens, private keys, credentials)

Agents with strictly read-only access and no tools capable of data exfiltration may omit AGT, but this exception must be justified in the agent's design documentation.

AGT integrates with ADK via the plugin system and adds sub-millisecond latency per tool call evaluation.

### Filesystem Write Access via A2A Coordination

Agent Runtime's read-only deployment model means agents cannot write to the local filesystem. For use cases that require write access — such as code generation, build tooling, or coding exercises — a coordinator agent on Agent Platform can use the [Agent-to-Agent (A2A) protocol](https://adk.dev/a2a/intro/index.md) to delegate filesystem-bound work to a companion agent running on Cloud Run, where writable storage is available.

In this pattern, the coordinator agent on Agent Platform retains orchestration, governance (AGT), and observability, while the Cloud Run agent handles the filesystem-dependent execution in a separately scoped environment. This preserves the security properties of Agent Platform for the coordination layer while extending capability to workloads that need it.

The Cloud Run companion agent is an independent security boundary with its own requirements:

- **Identity**: Runs as a dedicated GCP service account with least-privilege IAM, separate from the coordinator agent's SA
- **Credential isolation**: Tool credentials scoped via per-secret Secret Manager IAM bindings, same as Agent Platform agents
- **Filesystem lifecycle**: Cloud Run's writable filesystem is ephemeral (in-memory, cleared on instance shutdown). Persistent storage, if needed, must be explicitly provisioned and its access controls documented
- **Network egress**: Must use the same VPC + Secure Web Proxy egress controls as Agent Platform agents — deny-all with explicit domain allowlist
- **Governance**: AGT is required if the companion has write access to external systems. The coordinator's AGT policy does not extend to the companion — each agent enforces its own governance
- **Observability**: Must emit traces to Cloud Trace and logs to Cloud Logging, ensuring end-to-end tracing across the A2A boundary

## Cross-Cutting Concerns

### Security:

The platform maps to the 5-layer agent security model as follows:

| Layer | Mechanism |
|-------|-----------|
| **Infrastructure** | Agent Runtime manages compute, scaling, execution. Dedicated GCP service account per agent with least-privilege IAM. Workload Identity for model access. |
| **Sandbox** | No shell, no exec, no writable filesystem in source-archive mode (platform-enforced). AGT allow/deny lists for tool governance. Content filters for exfiltration prevention. |
| **Harness** | Multi-agent orchestration via ADK. Least-privilege tool assignment per sub-agent (e.g., researcher can search the web but cannot approve PRs). |
| **Runtime** | ADK agent loop and tool dispatch. Session management. No custom agent loop implementation needed. |
| **Model** | Model-agnostic via ADK. Gemini models accessed via Workload Identity (no API keys). Vertex AI handles model serving. |

Network egress is enforced at the VPC boundary via Private Service Connect and Secure Web Proxy with an explicit domain allowlist. Firewall rules ensure nothing bypasses the proxy.

### Observability:

* Cloud Trace with OpenTelemetry captures full execution traces across multi-agent orchestration
* Cloud Logging receives agent output and framework logs. AGT content filters apply to tool calls; agents must also avoid logging raw credentials, API tokens, or PII. Log access is controlled via IAM roles on the logging project
* Cloud Monitoring provides runtime metrics (invocation count, latency, errors)
* Agent Gateway (evaluating, in preview) may consolidate enforcement and observability into a single layer when it GAs

### Cost:

* Agent Runtime: per-invocation pricing based on compute time and model calls
* Vertex AI: standard API pricing for Gemini model calls (Pro for orchestration/judgment, Flash for context gathering)
* Secret Manager: per-access pricing for tool credentials (negligible at current scale)
* Secure Web Proxy: per-GB egress pricing through the proxy
* No cluster hosting costs — fully managed

### Operability:

* **Deployment**: `adk deploy agent_engine` CLI packages code and deploys to Agent Runtime. Terraform modules provision infrastructure (VPC, SWP rules, IAM bindings, Cloud Scheduler triggers).
* **Scheduling**: Cloud Scheduler triggers agent invocations on configurable schedules
* **Monitoring**: Agent deployments visible in the Agent Runtime console. Execution traces in Cloud Trace. Alerts via Cloud Monitoring.
* **Updates**: Redeployment via `adk deploy` — no rolling update infrastructure needed. Source archive is replaced atomically.

---

**Related documents**:
- [Agent Autonomy Levels](agent-autonomy-levels.md)
- [AI-Centric SDLC](ai-centric-sdlc.md)
- [Zero Operator Access](../identity/zero-operator-access.md)
- [Blog: What Even Is the Harness? (5-Layer Agent Security Model)](https://medium.com/@rbean_3467/what-even-is-the-harness-f21336768a80)
- [ADK Documentation](https://adk.dev)
- [Microsoft Agent Governance Toolkit](https://github.com/microsoft/agent-governance-toolkit)
- [Gemini Enterprise Agent Platform Documentation](https://docs.cloud.google.com/gemini-enterprise-agent-platform/overview)
