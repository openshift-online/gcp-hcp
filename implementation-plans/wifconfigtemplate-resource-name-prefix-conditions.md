# Resource-Name-Prefix Conditions for Component IAM

## Overview

Constrain supported component operations to installation-owned GCP resource names,
building on [Plan 1: explicit component permissions and IAM lifecycle](wifconfigtemplate-catalog-and-iam-lifecycle.md).
Plan 1 replaces broad predefined roles with explicit reviewed permissions and
establishes contract publication, cloud verification, exact assertions, dispatch
gates and safe cleanup. **This dependent plan changes resource scope**, not merely
the list of allowed operations.

**Status:** proposed follow-up implementation. Prefix-conditioned contracts and
the required component naming integrations were **not** proved by the final
project-wide-role installation/upgrade POC. Earlier source-only prototypes and
selected API probes are research inputs, not shipped or release-payload-matched
capabilities. Do not present the two published Plan 1 POC branches as containing
a completed implementation of every component dependency below.

**Prerequisites:** Plan 1's supported catalog/lifecycle/API/dependency release;
reviewed per-resource naming and condition capability matrices; and compatible
component artifacts for each scoped operation before enabling its contract.
Naming development can proceed in parallel, but conditioned-contract activation
depends on both the IAM lifecycle and those runtime deliveries. Plan 1 must not
depend on this plan.

### Related work and ownership

- [GCP-1446 — consistent cluster-prefixed resource naming](https://redhat.atlassian.net/browse/GCP-1446)
  tracks the **naming** foundation independently of IAM. Its scope excludes IAM
  catalogs/grants, Resource Manager tags and migration of existing/imported resources.
  Reference its deliverables as dependencies; do not silently expand that epic to
  own all the IAM work in this plan. Assign separate IAM follow-up tracking in refinement.
- [Workload Identity](../design-decisions/identity/workload-identity-implementation.md),
  [naming conventions](../design-decisions/governance/naming-conventions.md),
  [upgrade controllers](hosted-cluster-upgrade-controllers.md),
  [CSI integration](gcp-persistent-disk-csi-driver.md),
  [registry integration](gcp-321-image-registry-operator-integration.md), and
  [Definition of Done](../docs/definition-of-done.md).
- Owners span Gecko, gcp-hcp-ctl, HyperShift HO/CPO, CAPI/CAPG, GCP CCM,
  cluster-storage-operator, csi-operator and, if the inventory proves necessary,
  the GCP PD CSI driver/sidecars or image-registry operator. Infra owns deployment,
  image configuration and platform integrations, not customer-IAM reconciliation.

## Scope and guarantees

In scope: authoritative prefix allocation; immutable opt-in runtime naming policy;
component naming/configuration changes; typed condition-aware contracts; compatible
CLI rendering and verification; ownership-safe lifecycle; supported fresh-install
activation; scoped positive/negative runtime tests; explicit transition design.

Not in scope by implication: universal same-project isolation, automatic migration
or permission pruning, runtime IAM-admin delegation, arbitrary user-supplied IAM
CEL, Resource Manager tag bindings, imported-resource renaming or a new authorization
hold. Compute network tags used by existing networking code are not Resource Manager
tags and must not be confused with that separate scope.

**The isolation claim is per tested permission/resource/request path.** List and
operation polling, shared subnet/network use, some create checks, DNS/Service
Directory and any remaining unconditional mutations need explicit exceptions or
different supported designs. An installation still sharing a default worker SA
retains that shared identity boundary. Do not label the result hard cluster isolation
while another effective grant or a shared identity authorizes the same access.

## Dependency and delivery model

```text
Plan 1: explicit permissions + catalog/assertions + owned lifecycle
                      │
Naming/resource inventory → component APIs/implementations → tested release artifacts
                      │                                  │
                      └──── condition-aware contract ────┘
                                      │
                      fresh-install capability admission
                                      │
                 matched IAM/runtime positive + negative proof
                                      │
                      scoped contract verification / rollout
```

Use component-sized delivery tracks. Do not condition a resource whose actual
producer still uses an unrelated legacy name. A partially delivered track may
remain explicitly project-scoped in a complete reviewed contract; it is not
eligible for a stronger isolation claim. Keep all six component identities and
the entire permission inventory explicit even when only some surfaces are scoped.

## 1. Naming, allocation and ownership contract

Define one versioned creation-time naming policy, agreed across CLI, Gecko, HC/HCP,
HO/CPO and actual resource producers. A candidate policy name such as `InfraIDV1`
is illustrative, not a currently supported public API promise.

- Persist an immutable opt-in before IAM rendering and installation submission.
  Decide which public input expresses customer intent and which platform-owned
  capability checks authorize it; do not treat a caller-provided capability as
  trusted proof of runtime support. The naming mode must equal the mode required
  by the resolved Version's single contract: reject both marked requests against
  project-contract Versions and unmarked requests against prefix-contract Versions.
  Neither CLI nor Gecko selects another profile or silently falls back. Propagate
  the admitted mode consistently through HC and HCP.
- Existing installations with no policy retain identical names, configuration and
  rollout/ignition hashes. Never add the policy to an already-running legacy fleet
  by default, strip it to roll back a marked cluster, or retrofit existing objects.
- Derive names from stable installation/resource UIDs, not friendly names alone or
  management-cluster metadata. Preserve complete uniqueness where possible; define
  explicit collision handling rather than silent truncation or randomized retry.
- Derive prefix lengths per resource/API and suffix budget. The earlier prototype's
  12-character infra-ID bound was IAM-influenced and is not automatically the
  production runtime limit. Account for full PVC/snapshot/Service UIDs, regional
  versus zonal resources, global bucket uniqueness and case/character constraints.
- Reserve prefixes atomically in the authoritative customer-project namespace.
  A check-then-create helper is not a concurrency guarantee. Prefixes `a-` and
  `a-b-` overlap under startsWith semantics; require a prefix-free allocation or
  another explicitly verified non-overlapping namespace. Cover all creation paths,
  crashes/resume, imported reservations and delayed/tombstoned installations.
  Assign the namespace to the customer-installation lifecycle, not to an incidental
  regional controller cache: the CLI has customer-project access, while Gecko does
  not inventory customer GCP resources. Candidate anchors are a customer-owned
  transactional/CAS reservation store keyed by project number and a stable
  installation ID, or a separately reviewed platform reservation authority covering
  every region/creation tool. Select the authority and failure/cleanup semantics
  before implementation. A local journal lock, regional Cluster LIST or the old
  pure `CheckPrefixReservations` proposal alone cannot prevent cross-region races;
  no customer IAM-admin controller is introduced by this reservation requirement.
- Resource names are not ownership proof. Continue storing/checking numeric IDs,
  project/location/network/subnet, immutable parent UID and owner metadata where
  available. Reject same-name foreign/recreated resources before reuse/deletion.
- Record lifecycle owner per resource and any supported handoff, e.g. bootstrap
  firewall to CPO. Keep the narrow Plan 1 absent-firewall cleanup rule; do not use
  this work to accept every missing/mutated resource or bypass finalizers.
  Failed/unverifiable PSC cleanup retains its finalizer, including when its HCP
  cannot be found; missing parent feedback is not proof of safe external deletion.

**Acceptance:** deterministic per-type naming examples and length/collision tests;
atomic concurrent reservation proof; no-policy compatibility; owned/foreign name
collision tests; complete create/reconcile/upgrade/delete inventory.

## 2. Component development dependencies

The table is a delivery/inventory requirement, not a claim that provisional names
or every upstream API already support the required behavior.

| Producer / consumer | Required change and integration chain | Validation before conditioned binding |
|---|---|---|
| Gecko and creation tooling | Immutable naming mode and explicit worker-network inputs in planned intent, Cluster/HC/HCP/NodePool manifests; no hidden default or CLI-only prefix | Public/private admission, propagation and canonical plan checks; reject unknown/incomplete mode before dispatch |
| HO + CAPI/CAPG workers | Propagate stable Machine naming into GCPMachine, VM and boot-disk names; preserve complete suffix identity | Trace actual deployed CAPI/CAPG versions; VM/boot-disk create, labels, replacement, repair and teardown all match selectors |
| CPO PSC/firewall | Replace UUID-only names where needed for endpoint/IP/firewall surfaces; preserve UID/project/network/subnet/owner checks and legacy behavior | Full PSC lifecycle, connection acceptance versus IAM denial, endpoint/address labels and settled cleanup; no broad same-name adoption |
| CPO → GCP CCM | Versioned prefix/config input and explicit customer region/subnetwork; shared and per-Service names use stable IDs without discovery from management metadata | Internal/external LB create/reconcile/update/delete with actual provider images and valid instance-group fixtures |
| Cloud network config controller | Ensure instance targets follow worker naming; inventory shared-subnet reads/use and operation polling | Network-interface mutation succeeds only on intended instances; do not assume subnet access becomes cluster-local |
| CPO → CSO → GCP CSI operator/sidecars | Carry opt-in only to hosted GCP; set supported provisioner volume-name and snapshotter snapshot-name prefixes idempotently; trace snapshot-derived image paths separately | Dynamic volume, attach/detach, resize, snapshot, restore/image and deletion with actual shipped sidecars; existing/imported resources unchanged |
| Registry / storage APIs | Inventory actual bucket allocation, bucket/object IAM attribute support and stable names; require operator changes only if that inventory demonstrates a gap | Global-name allocation, create/update/list/object lifecycle and cleanup; do not assume a bucket/object prefix condition scopes bucket creation or project listing |

### Prototype source availability and research anchors

The early prefix prototype is **not retained as a complete recoverable source
snapshot in the published POC branches or the locations checked during consolidation**.
The referenced HyperShift/CSO prototype worktrees were absent; the CCM/CSI checkouts
found were clean at their old bases, without the reported changes. A partial
HyperShift tracked-file diff was preserved in the private context archive, but
does not contain the originally untracked new files and cannot rebuild the prototype.
Do not treat narrative test claims or that diff as cherry-pick-ready implementation.
P2 starts from the design and current upstream source; recover/reimplement tests
and changes deliberately rather than assuming the earlier code is available.

Historical source anchors, not new release selections:

| Component/input | Earlier inspected base or identifier |
|---|---|
| HyperShift | `1d3c34c749` |
| CCM cloud-provider-gcp | `3e67bea588c9eac2978d14726db729ee420cd557` |
| cluster-storage-operator | `1b546af` |
| csi-operator | `f5d54d11ed1ba2ee3cefb75a4b2c18b30c6df378` |
| Traced CAPI / CAPG | v1.14.3 / v1.13.1 |
| Proposed CPO capability label | `io.openshift.hypershift.control-plane-operator-supports-gcp-resource-naming` |
| Proposed CSO → GCP CSI env input | `GCP_RESOURCE_NAME_PREFIX` |

These anchors preserve what was inspected, not evidence that those current images
support the proposed ABI. Research starting points must be revalidated against
selected release payloads:

- Earlier source traces found CAPI MachineNaming → infrastructure clone name →
  CAPG instance name; an unset Compute boot-disk name defaulted to the instance
  name. This is not proof that the runtime images in a new payload have the same
  behavior, especially if CAPG begins assigning an explicit disk name.
- The prototype distinguished **PSC endpoint subnet** from **worker/CCM subnet**.
  Do not reuse the current `gcp.subnet` field for both without an explicit API
  design. Both network shapes must be present in validation/cleanup identity checks.
- Proposed PSC names included `<infraID>-psc-endpoint` / `<infraID>-psc-ip`.
  The existing `<infraID>-internal-cluster` worker firewall did not require renaming;
  lifecycle adoption still required checks. No need to rename already-compatible
  resources merely for naming uniformity.
- Proposed CSI flags were `--volume-name-prefix=<infraID>-pvc` and
  `--snapshot-name-prefix=<infraID>-snapshot`; full UIDs were retained. They can
  affect newly allocated PV object names as well as cloud names. Validate sidecar
  versions and snapshot-to-image paths; do not assume flags rename existing disks.
  The earlier 12-character-prefix proposal budgeted 53-character disk names and
  58-character snapshot names with full UIDs. Recalculate against the final naming
  contract rather than truncate or treat those prototype bounds as universal.
- Registry naming support was not established as a component change. Establish
  whether existing naming suffices before adding a new operator input.

Earlier notes report unit/mock tests and one HyperShift envtest version (Kubernetes
1.34.1: 13 create and six update naming cases), with no HCP naming envtest suite.
Those results are not independently reproducible from a complete retained prototype
and were not live prefix-IAM, traffic, release-image or mixed-fleet upgrade proof.
Use the recorded designs to reconstruct coverage; do not imply test/source retention
or depend on unpublished local paths.

## 3. IAM condition capability matrix

For every permission and operation, record: effective principal; target API and
request shape; IAM authorization resource/attributes; supported resource type/name
form; create/get/list/update/delete/operation checks; condition expression; remaining
exceptions; positive and outside-boundary evidence. IAM membership is additive:
review all effective grants on test identities so an unconditional grant cannot
mask a nonfunctional conditional one.

### Candidate scope buckets

| Component | Candidate conditionable surface | Reads/support that require separate review |
|---|---|---|
| CCM | Named firewall/forwarding/regional-backend operations; worker instance get/use | Project list/polling, shared network/subnet use, and address/health-check/instance-group/target-pool operations whose names or authorization resources do not yet fit |
| Cloud network | Worker instance get/updateNetworkInterface | Subnet get/use and operation reads |
| CPO | Named firewall and generic/PSC forwarding operations, potentially addresses after producer/support validation | DNS/Service Directory, shared network/subnet and operation support; do not infer a condition for services that lack the same name attributes |
| CSI | Instance attach/detach/get and supported named disk/snapshot/image operations | Project discovery/list/polling, parent-resource create checks and restore/import paths |
| Registry | Supported bucket/object operations on the allocated bucket namespace | Bucket creation and project listing; object-list checks may be bucket-scoped rather than per-object-name |
| CAPG/nodepool | Worker instance/boot-disk lifecycle, metadata/labels/service-account/network-tag operations | Shared subnet use and zone operation polling |

Earlier research split these into 15 logical roles by scope/nature while preserving
the same six component permission unions as the six-role project-wide alternative.
That split is a starting point, not the final role count or verified condition set.
Every binding grants the whole role: permissions needing different scope semantics
must not remain in a shared role if the split creates unintended cross-products.
More specifically, the proposed role-per-(scope × nature) split was CCM four roles
(resource mutations, instance use, read-only, support), CPO three (DNS, resource
mutations, support), and two each for cloud-network, CSI, registry and nodepool.
The CCM support role held **30 project-wide exceptions**, not just harmless reads;
the registry isolated `storage.buckets.list` in a discovery role. Keep these facts
visible when refining exceptions instead of describing all support access as scoped.

Provisional selector inputs were instance `{infraID}-`, CSI disk `{infraID}-pvc-`,
snapshot and snapshot-derived image `{infraID}-snapshot-`, nodepool boot disk
`{infraID}-`, and bucket `{infraID}-`. They are input candidates requiring the
producer/IAM matrix, not verified conditions. The earlier prototype described
`ErrRoleSplitRequired` for simultaneous permission and grant changes on a shared
role; reconstruct that safety test rather than relax Plan 1's stricter topology
refusal or assume the unpublished helper is available.

### Condition construction rules

- Use allow-listed typed resource selectors and an audited renderer; keep raw CEL
  out of customer/catalog input. Project/region/zone/name fragments must be validated
  and escaped. Do not execute the old illustrative regex comments as policy.
- Verify the service's actual IAM Conditions expression support and attributes.
  Use canonical IAM resource-name forms, not a Compute HTTPS selfLink. Compute
  global/regional/zonal and GCS `projects/_/buckets/...` forms differ; attributes
  available during creation may not identify the future child resource at all.
- Bind resource type/service together with bounded name predicates where required.
  Do not use a prefix alone to authorize unintended resource kinds or substitute
  string matching for a real project/location boundary.
- List permission is often evaluated on the parent/project, not filtered by child
  names. Operation names can be generated without the installation prefix. Model
  unavoidable discovery/polling separately rather than pretending the list is scoped.
- Shared network use is not made installation-local by the name of the VM being
  created. Enumerate secondary authorization checks (backend use, subnet use,
  actAs, PSC-specific verbs and Service Directory side effects).
- Never solve a false-negative test by restoring broad predefined roles. If a
  required operation cannot be safely conditioned, keep an explicit reviewed
  exception or change the supported architecture and state the weaker guarantee.

Keep the request-specific findings about CCM regional backend use, DNS absent versus
existing Get/List, generic plus PSC-specific permissions and Directory side effects
from [Plan 1](wifconfigtemplate-catalog-and-iam-lifecycle.md#2-component-iam-baseline-and-its-assurance-limits).
An ACCEPTED/REJECTED PSC connection policy is not an IAM result. Error prose alone
does not isolate the missing permission; fixture/admin operations and asynchronous
settlement must be distinguished from component calls.

## 4. Contract/API and CLI extension

Plan 1 v1 intentionally supports its reviewed project and exact-worker-SA grant
semantics, not arbitrary prefix bindings. Introduce a reviewed protocol/schema
version or compatible explicit capability model before accepting new scope kinds.
Do not silently change v1's interpretation, change its digest or enable an old CLI
to ignore an unknown condition/naming requirement. Preserve all published contracts.

The extension must represent, as immutable executable content:

- Typed resource selector scope, condition-relevant project/location/name inputs
  and naming-policy version; installation-specific values remain installation input.
- Split logical role IDs and exact binding role/member/condition topology.
- Required runtime capabilities and the creation policy necessary for those scopes;
  a second mutable verification boolean is not needed. `verified` remains the sole
  catalog eligibility assertion, not a capability-negotiation shortcut.
- Explicit project/shared-resource exceptions, retained permission unions and exact
  worker-SA actAs. No new long-lived keys, updater identity or worker project grants.

Extend private-source types, generated public API/schemas, canonical digest vectors,
boundary manifests, CLI parser/planner/executor/readback and owned cleanup together.
Scope/naming changes are semantic contract changes, not a verification toggle.
The CLI must compare policy v3 bindings by role/member/**condition**, preserve
unrelated conditional grants/audit state, use etags and independently verify exact
rendered conditions. Journal ownership/intent remains authoritative on resume.

**One Version still has exactly one immutable template reference.** A project
contract and prefix contract are not installation-selectable alternatives for an
already-pinned Version. Do not alter existing pins, numeric-floor selection or
assertion keys to provide a hidden profile toggle. An installation-selectable
profile model would be a separate reviewed API redesign spanning Version references,
Cluster assertions, canonical digest semantics, CLI and rollout compatibility.

Publish new scope content only at a **new canonical numeric raw release boundary**
after complete pin-impact review, selecting it only for Versions still unpinned or newly acquiring
their first reference. Prefer `X.Y.0` minor boundaries to avoid changing scope in the
middle of a z-stream. A new boundary that would invalidate an existing association
must stop for explicit review, never silently repin/delete/recreate. No fabricated
installable Version or profile alias is a substitute. A template boundary itself
need not be an advertised release, as in Plan 1, but installed/upgraded Versions
must be real supported releases. Extend Plan 1's per-boundary immutable
packaging before publishing different scope content, and satisfy the supported-
upgrade-path gate in §6 before any reachable boundary becomes eligible.

## 5. Runtime capability and rollout ordering

A CPO capability label can prove it understands a naming input, **not** that all
selected CCM/CSO/CSI/CAPI/CAPG/registry binaries honor it. Maintain the supported
release-to-component artifact/capability matrix and test the actual deployment.

**Invariant:** a conditioned template cannot be verified, nor a Version referencing
it made Available, before the actual release payload/component matrix satisfies
the naming and condition requirements. `verified` remains the only mutable template
eligibility assertion. Enforce this through reviewed immutable release/capability
metadata and publication/admission checks; extend the shared availability predicate
for the new protocol as required. Do not rely solely on an operator remembering a
label, add a second mutable readiness flag, or call external registries during API
admission. Naming-capability checks must also fail closed in the NodePool path.

1. Deliver component API and implementation changes with absent-policy legacy
   behavior unchanged. Generate API/CRD artifacts twice; do not hand-edit outputs.
2. Deliver compatible HO/CPO and all required downstream component images through
   their normal release/build pipelines. Confirm what the actual release payload
   selects; a patched management operator alone may not select patched CPO/CCM/CSI.
3. Add creation/admission and capability checks; reject unknown/partial modes before
   provisioning a prefix-conditioned installation. Do not silently fall back to
   legacy names when a marked input cannot be honored.
4. Publish complete conditioned templates **unverified**. Validate rendered IAM
   conditions, runtime mapping and complete Version/pin impact before verification.
5. Verify only after scoped live functional/negative/teardown acceptance. Existing
   Plan 1 contracts remain available under their explicitly supported scope.

For unmarked existing fleets, prove byte/semantic stability of ignition, config,
rollout hashes and selected identity inputs; no automatic rollout merely because
the operator learned a new field. Marked clusters cannot be downgraded to consumers
that do not understand their immutable naming policy. Use Plan 1's compatible
API/consumer-before-write staging, migration-Job preservation, durable receipts and
GitOps verification controls; no new bypass is introduced here.

## 6. Existing-installation transition is a separate gate

The first supported delivery may be **fresh prefix-enabled installations only**.
Existing Plan 1 workloads retain legacy names and unconditional grants. Migration
or imported-resource renaming is not authorized by GCP-1446 or this initial path.

That fresh-only scope must not silently dead-end supported upgrade paths. A legacy
unmarked cluster cannot honor the prefix contract's naming requirement: Plan 1's
role-only updater rejects its grant-topology changes and old resource names would
not match the conditions. Before publishing a prefix boundary reachable by that
fleet, require either an approved P2.9 transition with acceptance, or a deliberate
documented support-policy decision that the boundary is not a supported target for
unmarked installations, enforced by actual Channel/fleet/upgrade admission policy.
A global fleet-minor number alone may not distinguish marked and unmarked clusters
sharing a channel; implement the necessary policy before activation if it is absent.
Do not express an unsupported transition as a permanent, unexplained IAM block.

Symmetrically, a marked installation cannot silently upgrade to a later minor's
project-scoped contract or strip its immutable naming mode. Every supported next
boundary/minor needs its own compatible conditioned contract (no cross-stream
fallback), or a separately reviewed scope-change transition covering any widening
and runtime compatibility. Neither direction is automatic migration. Tests must
show unmarked fleets keep every promised supported edge and marked fleets keep a
compatible forward path, with no implicit retrofit or fleet-wide dead end.

Plan 1's permission-only union writer correctly rejects arbitrary grant-topology
changes. Do not loosen it to reuse an existing role at a broader/new condition or
create a role-permission/grant cross-product. If an in-place transition is later
required, design a separate staged operation with explicit approval and tests:

1. Inventory all running/pending consumers, old resource names and ownership, all
   effective grants and shared dependencies; identify unrepresentable mixtures.
2. Add distinctly owned conditioned roles/bindings only through a supported new
   contract/journal protocol, retaining old access while any old consumer needs it.
3. Deliver and observe compatible component/resource transitions without renaming
   existing disks/imports or falsely asserting isolation during overlap.
4. Independently prove every required path works and every old consumer is gone.
5. Remove only the explicitly approved unconditional membership/role deltas through
   a separate scope-reduction/retirement policy; there is no automatic pruning.

**An old unconditional binding continues to authorize outside-prefix access even
when the new conditioned binding exists.** Retaining both may be a safe availability
transition but cannot satisfy a restricted-access/isolation acceptance claim.
Customer-owned external grants and inherited/org policy need explicit treatment.

## Work packages and acceptance

| WP | Owner/dependency | Deliverable / pass condition |
|---|---|---|
| P2.1 — Resource/permission inventory | Component owners + IAM/security; Plan 1 baseline | Full per-operation authorization-resource and name-producer matrix, exact exceptions and supported claim; no assumptions copied from provisional CEL |
| P2.2 — Naming/reservation ABI | GCP-1446, Gecko/CLI/HO/CPO | Creation-only versioned inputs, explicit worker/PSC network semantics, type-specific length/UID rules, authoritative concurrent prefix reservation and ownership contract |
| P2.3 — Worker/PSC/runtime foundation | HO/CPO/CAPI/CAPG; P2.2 | Actual names/lifecycle verified, immutable propagation and legacy/no-rollout behavior; foreign/recreated resources protected |
| P2.4 — Network/service producers | CCM + cloud-network + CPO; worker naming for instance selectors | Hosted-GCP config propagation and Service resource names; actual internal/external LB/network paths function within documented scopes |
| P2.5 — Storage/registry producers | CPO/CSO/CSI/sidecars and registry only as necessary | Validated new-volume/snapshot/image/bucket semantics, imports/legacy preserved and whole dependency chain shipped |
| P2.6 — Condition-aware contract | Gecko API/chart + CLI; Plan 1 + P2.1/2.2 | Explicit schema/capability compatibility, safe typed renderer, role splitting, immutable digest vectors, exact policy-v3 condition readback and journal lifecycle |
| P2.7 — Artifact capability admission | Release/component owners + Gecko; P2.3–2.6 for selected scopes | Actual release component matrix gates template verification, Version availability and new marked work; no inference from CPO label alone |
| P2.8 — Scoped runtime proof | Owning E2E/IAM suites; selected capabilities complete | Positive lifecycle plus controlled outside-prefix denial, async settlement, same-project coexistence, restart/upgrade and safe cleanup; exceptions accurately reported |
| P2.9 — Optional transition design | IAM/security + lifecycle owners; not implicit in fresh-install release | Reviewed grant retirement/mixed-name policy with no widening, outage or isolation overclaim; otherwise document fresh-only support |

Do not place P2 IAM work under the naming-only epic by default. Create linked,
reviewable work items during refinement and use normal owning-repository review,
generation, tests/race, CI and the team's Definition of Done. No issue creation or
source/runtime rollout is performed merely by approving this document.

## Validation matrix

- **Naming/compatibility:** absent/valid/invalid/unknown mode, create-only immutability,
  full name budgets and UIDs, overlapping prefixes/concurrent reservations,
  legacy byte/rollout/ignition stability and unknown-capability refusal.
- **Condition/unit tests:** canonical service/type/name formats and escaping,
  supported attribute/function subset, empty/parent create resource, mixed scopes,
  role-split cross-products and same project/different installation cases.
- **Cloud controls:** fresh narrowly authorized principal per comparison, valid full-
  set control with matching request shape, actual successful lifecycle and settled
  async operation. Audit effective policy so unconditional grants do not mask tests.
- **Negative tests:** known existing outside-prefix resources owned by the fixture,
  unused-name creates with matching positive controls, wrong type/location/project
  where applicable, and foreign/recreated resource deletion refusal. Do not treat
  a resource-not-found response as proof of denied or authorized access.
- **Runtime:** full supported worker/PSC/LB/network/storage/registry lifecycle using
  actual guest WIF, release-selected binaries and declared unconditional exceptions;
  include normal cleanup and controller adoption/handoff behavior.
- **Upgrade/recovery:** mixed old/new consumers, partial delivery/restart, failure
  before/after policy writes, exact assertions/receipts, conservative old permission
  retention, no-policy fleet upgrade and unsupported downgrade refusal. Prove both
  naming-mode/contract mismatch directions refuse, one pinned Version cannot select
  another profile, and every supported legacy/marked upgrade edge remains usable
  without implicit retrofit or an indefinite fleet-wide IAM block.
- **Isolation scope:** prove only the covered paths; report remaining project-wide
  mutations/discovery and shared worker identity. Broader hard isolation requires
  its own architecture/security acceptance, not stronger wording on this plan.

Live validation requires a separate explicit footprint/time/cost and cleanup scope.
Do not replay retired POC harnesses, names, permissions or mutation journals.
Keep implementation/release source in owning repositories, not a new AI-context
experiment tree. This plan plus Plan 1 and their immutable source references must
be sufficient to refine and implement the delivery without personal local notes.
