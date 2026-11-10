---
name: gcp-hcp-feature-support-assessment
description: >
  Use when assessing GCP HCP feature support across HyperShift, Gecko,
  gcp-hcp-ctl, kube-applier-gcp, infrastructure, and external dependencies.
  Identify remaining implementation scope and effort from verified evidence.
---

# GCP HCP Feature Support Assessment

Assess the path relevant to a requested customer or operations feature.
Identify what already works at a stated source revision, release, or deployed
environment, then report the smallest remaining scope, effort, risk, and
confidence. An assessment is the deliverable; do not implement the feature
unless the user also requests implementation.

## Repositories and evidence

| Layer | Repository | Local clone hint |
| --- | --- | --- |
| HyperShift API, operator, and GCP behavior | [openshift/hypershift](https://github.com/openshift/hypershift) | `../hypershift` |
| Customer API and controllers | [openshift-online/gecko](https://github.com/openshift-online/gecko) | `../gecko` |
| Cluster, node pool, and operations CLI (`gcphcpctl`) | [openshift-online/gcp-hcp-ctl](https://github.com/openshift-online/gcp-hcp-ctl) | `../gcp-hcp-ctl` |
| Management-cluster resource applier and status feedback | [openshift-online/kube-applier-gcp](https://github.com/openshift-online/kube-applier-gcp) | `../kube-applier-gcp` |
| Internal deployment and platform automation | [openshift-online/gcp-hcp-infra](https://github.com/openshift-online/gcp-hcp-infra) | `../gcp-hcp-infra` |
| Designs and implementation plans | [openshift-online/gcp-hcp](https://github.com/openshift-online/gcp-hcp) | current repository |

Resolve clone hints relative to the parent of the GCP HCP repository. A clone
may be elsewhere or unavailable; locate it rather than assuming a path. Use
authenticated repository access when available for private sources. Never
infer support from an inaccessible repository.

Set the assessment target before searching: `SOURCE` (branch or commit),
`RELEASE` (version or artifact), or `DEPLOYMENT` (environment and running
version). If the user gives no target, inspect each relevant repository's
remote default branch as a provisional `SOURCE` baseline. Ask whether they
mean source, release, or deployment availability when that choice could change
the conclusion. Label source-only findings as such until the target is
confirmed. For a release or deployment, identify the source revisions it
contains and verify the artifact or running version where possible. Source
code alone does not prove release or deployment availability.

Verify each local clone's remote and commit against the target. Inspect pinned
refs or remote files without switching, resetting, or otherwise disturbing a
sibling checkout. Fetch only when needed and safe for that clone; otherwise use
remote access. Record the actual revisions inspected in `EVIDENCE`. If a
requested release cannot be mapped to source revisions, report that layer as
unverified. Code on a newer branch does not establish support on an older
release line.

## Investigation workflow

1. Restate the requested behavior, user or operator interface, target, and
   affected resources. Ask a concise question to confirm an assumption when
   plausible answers would change the target, required repositories,
   `SUPPORTED`, `SCOPE`, or `LEVEL OF EFFORT`. Continue investigation that
   does not depend on the answer. State low-impact assumptions whose
   alternatives would not change the conclusion.
2. Start at the feature's entry point and trace the required path. A customer
   API request may lead through Gecko, kube-applier-gcp, and HyperShift; an
   operations CLI request may instead lead through `gcp-hcp-ctl` and Cloud
   Workflows. Do not require unrelated layers to establish support.
3. Inspect Gecko API types, validation, generated schemas or clients,
   controller logic, status propagation, and Firestore desires or manifests
   when the feature uses Gecko. Inspect `ManifestWork` only when the requested
   revision uses that path.
4. Inspect HyperShift API fields, GCP-specific validation and controllers,
   rendered resources, feature gates, tests, and docs when HyperShift owns the
   behavior. Support on another cloud platform does not establish GCP support.
5. For management-cluster resources delivered through Gecko and Firestore,
   inspect `kube-applier-gcp` desire types, Firestore consumption,
   apply/read/delete behavior, status feedback, and tests. Verify the
   Gecko-to-applier contract in both repositories; a Gecko desire alone does
   not prove that the resource reaches the cluster.
6. Inspect `openshift-online/gcp-hcp-ctl` commands, flags, API payloads,
   output, and tests when the feature has a CLI path. Include `ops`, `iam`,
   or `network` commands when relevant, as well as cluster and node pool flows.
7. Inspect `gcp-hcp-infra` when the feature requires service-owned deployment,
   platform resources, or operations. Check the actual rollout or validation
   path, not merely whether a resource exists.
8. When HyperShift behavior depends on another component, follow its imports,
   `go.mod`, API types, and controllers to discover the provider, CAPI, cloud
   operator, or other code dependency. Check that dependency's implementation,
   release, and version consumed by HyperShift. Do not assume CAPG is the only
   dependency.
9. Trace external readiness when evidence points to release services,
   registries, image or operator catalogs, GCP product behavior, organization
   policy, or manual gates. Identify what is verifiable and what is not.
10. Read GCP HCP design decisions and implementation plans for constraints,
    then report positive and negative findings with repository paths and line
    numbers where possible. Name the files, paths, and terms searched for an
    absence.

Use available search, file, and repository tools; the workflow does not depend
on a particular agent's tool names. Parallelize independent repository reads
when practical, but follow dependency evidence before expanding the search.
If a required source cannot be inspected, report the limitation explicitly.

### Customer automation versus internal infrastructure

Customer-owned Terraform or CI consuming the Gecko public API belongs to the
customer-facing API path. Words such as “Terraform workflow” do not by
themselves imply `Infra Configuration`. For example, exposing an endpoint ID to
customer Terraform may require a Gecko API status field and controller status
propagation without any internal infrastructure change.

Use `Infra Configuration` only for GCP HCP-owned Terraform, ArgoCD, Helm,
pipelines, GKE/GAR/IAM/networking/Secret Manager resources, bootstrap, image
distribution, environment rollout, or operational validation needed to deliver
the feature. A GAR repository or pull-through cache alone does not prove that
required images are published, mirrored, pre-warmed, or pullable in the target
environment.

## Support and scope

Set `SUPPORTED: TRUE` only when evidence establishes the complete requested
path at the stated `TARGET`. For `SOURCE`, this means implementation at the
inspected commits; it makes no claim about a release or running environment.
For `RELEASE` or `DEPLOYMENT`, also verify that the relevant artifact or
environment contains the behavior, including required rollout and external
readiness. Set `SUPPORTED: FALSE` when evidence proves at least one required
layer is missing at the target. Use `SUPPORTED: UNVERIFIED` when a required
layer could not be inspected and there is no proven missing layer. Never turn
missing evidence into a positive or negative support claim.

If a material assumption remains unanswered, report conditional findings for
the plausible interpretations. Use `SUPPORTED: UNVERIFIED` when those
interpretations could change the result; do not present one as confirmed. A
proven gap shared by all interpretations may still support `FALSE`. Name the
open question and lower confidence until it is resolved.

List only proven remaining work, in dependency order, using these scope values.
Record inaccessible required layers separately so a partial list cannot appear
complete:

| Scope | Use for |
| --- | --- |
| `NONE` | Complete support with no implementation remaining. |
| `External Dependencies` | Hosted services, release metadata or pipelines, registries, image/catalog availability, GCP behavior, organization policy, or manual readiness outside the inspected code repositories. |
| `Hypershift Dependency Change` | Provider, CAPI, cloud operator, or other code dependency implementation, release, downstream sync, or bump needed by HyperShift. |
| `Hypershift Change` | HyperShift API, GCP validation, operator or CPO behavior, rendering, feature gates, tests, or integration. |
| `kube-applier-gcp` | Applier-owned desire parsing or schema, Kubernetes apply/read/delete behavior, or status feedback. Include for shared contract changes only when applier code must change. |
| `Infra Configuration` | GCP HCP-owned deployment, platform resources, image distribution, or operational automation in `gcp-hcp-infra`. |
| `Gecko API` | Customer-facing API types, validation, schema, generated clients, or API tests. |
| `Gecko Controllers Logic` | Reconciliation, durable state, status, retries, service calls, policy, or cross-resource coordination. |
| `Gecko Resource Delivery` | Gecko construction of management-cluster desires or HyperShift-facing resources, including field passthrough, without new durable controller behavior. |
| `gcphcpctl` | CLI commands, flags, payloads, output, docs, or tests in `openshift-online/gcp-hcp-ctl`. |

An existing HyperShift field copied through Gecko is usually `Gecko API`,
`Gecko Resource Delivery`, and possibly `gcphcpctl`; do not add controller
logic or `kube-applier-gcp` work for a Gecko-only desire field change. A simple
client-side transform also does not require durable controller logic. Add
`Gecko Controllers Logic` when it needs external calls, credentials, retries,
status, or asynchronous reconciliation.
Add `Hypershift Change` only after checking whether existing HyperShift GCP
fields and behavior can express the feature.

Track external readiness separately from repository code changes, while
listing every affected area in `SCOPE`. If an external service needs new
readiness work and platform code must consume it, include both
`External Dependencies` and the applicable implementation scopes.

For `SUPPORTED: UNVERIFIED`, use `SCOPE: UNVERIFIED` and
`LEVEL OF EFFORT: UNVERIFIED`. For `SUPPORTED: FALSE` with other uninspected
required layers, list confirmed missing scopes, name the unverified layers,
and use `LEVEL OF EFFORT: UNVERIFIED` unless the total effort can be bounded
from inspected evidence. Do not invent scopes or effort for inaccessible work.

## Effort, risk, and confidence

Effort reflects the hardest required behavior, not the number of repositories.
Missing evidence lowers confidence; it does not automatically raise effort.

| Level | Guideline |
| --- | --- |
| `NONE` | No implementation remains. |
| `XS` | Trivial change: single config value, documentation-only, or mechanical field copy requiring no validation or new tests. |
| `S` | Mechanical API/CLI exposure, field passthrough, docs, or local tests. |
| `M` | Moderate validation, generated API/client changes, simple transforms, or localized Terraform/Helm changes. |
| `L` | New HyperShift, Gecko, or kube-applier behavior; durable reconciliation; lifecycle semantics; significant infrastructure rollout; image pre-warming; or integration tests. |
| `XL` | Provider or other upstream dependency release chain, broad release-service or image supply-chain work, or multiple externally blocked stages. |

For example, zero-egress support that only needs a bounded platform-owned
mirroring workflow may be `L`; making the complete OCP payload and operator
catalog image supply chain available through external release or registry
changes is `XL`. Do not label all external dependencies `XL`: describe the
specific work and readiness gate first.

Use `RISK: LOW` for mechanical changes, `MEDIUM` for API compatibility or
coordinated rollout, and `HIGH` for lifecycle changes, credentials, provider
limits, production migration, external readiness, or critical unknowns. Use
`CONFIDENCE: HIGH` only when all required sources and revisions were inspected;
`MEDIUM` for indirect or incomplete detail; `LOW` when required repositories,
dependencies, revisions, release artifacts, deployments, or external readiness
cannot be verified.

## Output

Return this concise structure:

```text
TARGET: SOURCE (<ref or remote defaults; repository SHAs in EVIDENCE>)|RELEASE (<version or artifact>)|DEPLOYMENT (<environment and running version>)|UNVERIFIED (<unconfirmed target; inspected baseline>)
SUPPORTED: TRUE|FALSE|UNVERIFIED
SCOPE: NONE|<comma-separated scope values>|UNVERIFIED
UNVERIFIED LAYERS: NONE|<required repository, artifact, environment, or dependency and reason>
OPEN QUESTIONS: NONE|<material assumption awaiting user confirmation>
LEVEL OF EFFORT: NONE|XS|S|M|L|XL|UNVERIFIED
RISK: LOW|MEDIUM|HIGH
CONFIDENCE: HIGH|MEDIUM|LOW
SUMMARY: One or two sentences on current support, remaining work, and any partial assessment.
EVIDENCE:
- repository/path:line (revision) - concise positive or negative finding
- repository or dependency (revision unavailable) - what could not be inspected and why
```

Use `SCOPE: NONE` and `LEVEL OF EFFORT: NONE` only with complete support.
For a partial assessment with a proven gap, report `SUPPORTED: FALSE`, the
known scopes, `CONFIDENCE: LOW`, and every uninspected required layer in
`UNVERIFIED LAYERS`, `SUMMARY`, and `EVIDENCE`. Cite a relevant inspected file
and the searched path or terms when claiming a field or behavior is absent;
“searched the repo” is not evidence. Do not fabricate line numbers or claim
external readiness from configuration alone.

For an unanswered question that could change scope or effort, use
`SCOPE: UNVERIFIED` or `LEVEL OF EFFORT: UNVERIFIED` respectively unless the
same known work or bound applies under every plausible answer. Keep missing
evidence in `UNVERIFIED LAYERS` and unanswered user questions in
`OPEN QUESTIONS`.

## Creating Jira tickets from assessments

When creating a Feature or Initiative in Jira to track work identified by this assessment, use LEVEL OF EFFORT as a **starting point** for the Size field (customfield_10795), then adjust based on the playbook's Epic-count criteria.

**Playbook sizing criteria** (from `docs/feature-initiative-playbook.md`):
- **XS**: 1 Epic, well-understood, minimal dependencies
- **S**: 1-2 Epics, low complexity, few dependencies
- **M**: 2-3 Epics, moderate complexity or cross-team coordination
- **L**: 3-5 Epics, significant complexity or multiple dependencies
- **XL**: 5+ Epics, high complexity, architectural changes, or extensive unknowns

**Initial mapping from LEVEL OF EFFORT:**
- `NONE` → Do not create a ticket (work is complete)
- `XS` → Consider Size: XS (if truly single Epic, trivial change)
- `S` → Consider Size: S or M (depends on Epic count and complexity)
- `M` → Consider Size: M or L (refine based on Epic breakdown)
- `L` → Consider Size: L or XL (significant work, validate Epic count)
- `XL` → Size: XL (upstream dependencies, multi-stage work)
- `UNVERIFIED` → Architect estimates Size based on Epic breakdown and scope evidence when available; leave unset only if sizing evidence is insufficient. Record uncertainty in ticket description.

**Rationale:** LEVEL OF EFFORT reflects implementation complexity; Size reflects total scope (Epic count + complexity + dependencies). Use LEVEL OF EFFORT as initial guidance, then refine based on expected Epic breakdown and playbook criteria during Feature/Initiative refinement. This ensures Size accurately represents the full scope and satisfies the Definition of Ready requirement.
