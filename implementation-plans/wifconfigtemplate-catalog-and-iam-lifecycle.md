# Explicit Component Permissions, WIFConfigTemplate Catalog and IAM Lifecycle

## Overview

Deliver a platform-managed IAM contract catalog, customer-operated IAM setup and
updates, and non-bypassable installation/upgrade prerequisites for GCP Hosted
Control Planes. Gecko publishes required contracts and controls dispatch; the CLI
applies and verifies IAM in the customer project. Neither Gecko nor a management
cluster receives authority to administer customer IAM.

**Status:** production integration plan informed by a completed POC, not a claim
that the implementation is merged, generally available or production-hardened.
The POC proved a fresh installation, a specific missing-target-assertion block,
real control-plane and worker upgrades, coexistence and owned cleanup. Production
authorization integration, module alignment, observability and broader validation
remain work below.

**Delivery stage:** Plan 1 of 2. This plan limits component operations using
explicit custom-role permissions and exact identity bindings. Project-level
component bindings remain where required; it does **not** condition their access
on resource-name prefixes. It can ship independently of component naming changes.

**Dependent follow-up:** [Resource-name-prefix IAM conditions](wifconfigtemplate-resource-name-prefix-conditions.md).
That plan adds per-resource conditions only after this lifecycle foundation and
the necessary component naming/capability work are available. Do not delay Plan 1
for those dependencies or interpret "explicit permissions" as resource isolation.

**Repositories:** Gecko, gcp-hcp-ctl, gcp-hcp-infra; HyperShift for worker identity
compatibility and any runtime fixes explicitly identified during integration.
Tracking epic/child issues must be assigned during refinement; this document does
not invent issue keys or assert backlog items have been created.

### Design references

- [Workload Identity](../design-decisions/identity/workload-identity-implementation.md).
- [Platform-scoped Cedar authorization](../design-decisions/identity/platform-scoped-cedar-authorization.md),
  which supersedes authenticated catalog-read exemptions in older documentation.
- [OIDC issuer discovery](gcp-oidc-discovery-for-wif.md) and
  [OIDC CDN infrastructure](gcp-621-oidc-cdn-infrastructure.md).
- [Upgrade controllers](hosted-cluster-upgrade-controllers.md) and
  [release catalog/resolution](adopt-cincinnati-for-version-resolution-selection-and-upgrades.md).
- [Repository placement](../design-decisions/governance/repository-organization-policy.md)
  and [deployment ownership](../design-decisions/automation/deployment-tooling-swim-lanes.md).
- [Definition of Done](../docs/definition-of-done.md).

Older plans describe
historical inline-JWKS, embedded-role and upgrade-request proposals; reconcile them
with current owning-repository APIs rather than reproducing superseded mechanisms.
Platform Terraform/GitOps ownership remains unchanged. The CLI workflow described
here is explicitly customer-owned installation IAM/networking, not another platform
infrastructure reconciler or authority to edit platform Terraform resources.

## POC source and evidence boundary

The implementation experiment is preserved in dedicated source branches, not in
the session notes formerly collected under `experiments/iam-bootstrap/`:

| Source | Published branch | Reviewed reference tip |
|---|---|---|
| [Gecko](https://github.com/apahim/gecko/tree/poc/wifconfigtemplate) | `origin/poc/wifconfigtemplate` | `6c28cf4348a8ab4f524b91ece1cdbe29f1389c44` |
| [gcp-hcp-ctl](https://github.com/apahim/gcp-hcp-ctl/tree/poc/wifconfigtemplate) | `origin/poc/wifconfigtemplate` | `ba211636d48d6cedf11e6090536ef7c5f985800f` |

Use immutable source links for implementation review:

- [Gecko contract and dispatch protocol](https://github.com/apahim/gecko/blob/6c28cf4348a8ab4f524b91ece1cdbe29f1389c44/platform-api/WIFCONFIGTEMPLATE.md).
- [Executable baseline](https://github.com/apahim/gecko/blob/6c28cf4348a8ab4f524b91ece1cdbe29f1389c44/helm/charts/platform-api-server/files/wifconfigtemplate-project.yaml).
- [CLI protocol and recovery](https://github.com/apahim/gcp-hcp-ctl/blob/ba211636d48d6cedf11e6090536ef7c5f985800f/docs/WIFCONFIGTEMPLATE.md).
- [Owned firewall-aware cleanup](https://github.com/apahim/gcp-hcp-ctl/blob/ba211636d48d6cedf11e6090536ef7c5f985800f/pkg/infra/network/cleanup.go).

POC source comments sometimes describe the earlier phase in which a file landed.
They are not the final implementation status. The published source is reusable
implementation input; integrate it in reviewable changes against current main,
not by assuming the entire historical branch is production-ready.
The CLI tip retains both `a727640` (linked updater) and `c3daed0` (same-project/
zero-delta tests) as verified ancestors, followed by the final cleanup fix.

### What was actually established

| Area | Observed result | Limit of that evidence |
|---|---|---|
| API/catalog | Persisted template/Version references, immutable content, verification toggles, authenticated discovery, restart/pin persistence and fail-closed dispatch | Personal dev using Spanner; not all database/backends or production authorization paths |
| Fresh setup | Template-driven 5.0.2 install and separate 4.22.21 install, cloud readback, TLS/OIDC login, healthy guest/control plane and one Ready worker | Not a complete six-component functional/optional-feature matrix |
| Role update | Separate unbound-role additive update, real stale-etag rejection, field-mask preservation and separate-process readback after deliberately discarded success | Simulated response loss, not an actual network outage/power-loss test |
| Target gating | 4.22.21 → exact 5.0.0 remained unaccepted with `WIFConfigUpdateRequired` while the target Version was Available and the guest stayed on source | Customer path; automatic and other race cases primarily have offline/integration coverage |
| IAM application | Zero role-permission delta, unchanged six role etags, exactly one distinct target assertion despite equal executable digest | Not a live permission-expanding release transition |
| Upgrade | Guest CVO completed exact 5.0.0, then one worker completed 5.0.0; RHCOS 9 → 10 replacement stayed within one-worker surge | Does not authorize unsupported edges, aliases or OS overrides |
| Coexistence/cleanup | Original 5.0.2 installation remained healthy; secondary hosted resources/VMs removed normally; reviewed CLI cleanup removed secondary IAM/network and restored policy memberships | Same-project lifecycle separation, not hard cross-cluster GCP authorization isolation |

The POC ended on 2026-11-20 with both template verification and assertion writes
closed. The secondary installation was removed; expected role/pool tombstones
were retained. One original validation installation and platform infrastructure
were intentionally retained. These are historical observations, not fresh health
or authorization to delete retained resources. Operational identities/journals
belong in restricted local handoff material, not as reusable production inputs.

## Scope and non-goals

In scope: typed API contracts; authenticated/authorized catalog discovery; Version
pinning and availability; exact Cluster application coverage; durable dispatch;
CLI preview/create/update/verification/resume; running-permission retention;
ownership-safe teardown; GitOps delivery; observability; integration and E2E tests.

Out of scope:

- Migration/adoption of existing broad-role installations or caller-controlled
  legacy exemptions. Initially support fresh installations only.
- Prefix-scoped runtime naming, hard same-project isolation, Resource Manager tags
  and automatic post-upgrade permission pruning. Naming work is independently
  tracked under [GCP-1446](https://redhat.atlassian.net/browse/GCP-1446).
- A privileged customer IAM controller, management issuer/updater identity,
  continuous cloud attestation, broad-role fallback or automatic permission repair.
- New general upgrade APIs/CLI commands merely to demonstrate IAM gating; reuse
  the supported upgrade surfaces in the target integration revision.
- Automatic verification, an embedded runtime fallback catalog or per-release
  CredentialsRequest extraction/merge. Future compliant declarations may generate
  reviewed catalog inputs without becoming a second runtime source of truth.

## Design constraints

| Concern | Contract |
|---|---|
| Authority | GitOps owns shared contracts/verification; customer-authorized CLI owns installation IAM; Gecko owns metadata and dispatch decisions |
| Resource | Platform-scoped `gcp.managed.openshift.io/v1/WIFConfigTemplate` in normal Gecko/Orlop storage; not a new standalone CRD or ConfigMap |
| Eligibility | One mutable `spec.verified`; omitted/false is unusable for new use; no completeness/validation lifecycle or second readiness flag |
| Immutability | `schemaVersion` and executable IAM immutable; initially reject all template deletion; retain historical contracts |
| Boundary selection | Canonical numeric template name `X.Y.Z`; greatest floor in the same raw major/minor; no cross-stream fallback |
| Integrity | References contain name, UID and canonical executable digest; Version references are set once |
| Per-installation coverage | Append-only exact-reference assertions in Cluster spec, independently of platform Version availability |
| Worker identity | Explicit exact default Compute SA for the customer project; verify its real unique ID in CLI state; never create/delete it or grant it broad project roles |
| Updates | Add permissions only to existing same-identity/same-grant-topology roles; retain all required historical/running/pending contracts; no pruning |
| Dispatch | Persist UID/RV-guarded exact-target intent before external Apply; committed same-target redelivery is distinct from authorizing new work |
| Cleanup | Normal hosted teardown and dependency proof precede journal-driven removal of owned network/IAM deltas; no prefix-based deletion/adoption |

Structural completeness is admission validation, not eligibility. A platform's
verification assertion is separate from CLI cloud readback. A Cluster application
is an authenticated customer assertion, not Gecko's proof that cloud permissions
exist or remain in place. Normal customer API permissions can assert it without
using this particular CLI; the CLI is not a server-side trust root.

## Architecture and implementation details

### 1. Typed catalog and immutable identity

Use private API types as the generation source for public types, schemas,
conversions and deep copies. Validate bounded lists/strings, unique component and
role names, valid internal references and supported scopes. No arbitrary
catalog-supplied expressions or cloud/Cincinnati calls during admission.

Minimal resource envelope:

```yaml
apiVersion: gcp.managed.openshift.io/v1
kind: WIFConfigTemplate
metadata:
  name: "5.0.0"
spec:
  verified: false
  schemaVersion: 1
  iam:
    # Complete typed naming, component identities, roles and exact grant topology.
```

`schemaVersion` describes the executable protocol independently of the Kubernetes
API envelope. New schema semantics need explicit versioning/immutability decisions;
do not tighten version 1 in ways that invalidate stored accepted contracts.

**Digest compatibility is part of the API.** The POC uses SHA-256 over UTF-8 typed
Go JSON with verification omitted; metadata and boundary name are excluded.
The fixed field order starts with `schemaVersion`, then `iam`. Canonicalize on a
copy: sort required components, account/role names, subjects, permissions, WIF refs,
actAs members, and role bindings by account/role key. This is **not RFC 8785**.
Retain golden vectors and prove list reordering/verification changes do not alter
the digest. Adding a spec field cannot silently change its digest participation.

Reviewed executable baseline digest:
`sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.

### 2. Component IAM baseline and its assurance limits

Preserve the executable baseline: six component SAs, six custom roles, six
unconditional project bindings, seven exact guest-subject WIF memberships and two
exact-worker-SA actAs memberships: **14 IAM resources / 15 scoped grants**.
There are 142 per-role permission entries representing 114 distinct permissions.

| Component | Guest service account subject(s) | Role capability |
|---|---|---|
| `cloud-controller` | `kube-system:cloud-controller-manager` | Compute service/LB/address/firewall/instance-group reconciliation and operation reads |
| `cloud-network` | `openshift-cloud-network-config-controller:cloud-network-config-controller` | Instance network-interface and subnet operations |
| `ctrlplane-op` | `kube-system:control-plane-operator` | Firewall/PSC, DNS, network and Service Directory prerequisites |
| `gcp-pd-csi` | `openshift-cluster-csi-drivers:gcp-pd-csi-driver-controller-sa` | Disk/attach/detach/resize/snapshot/image operations; exact worker-SA actAs |
| `image-registry` | `openshift-image-registry:cluster-image-registry-operator` and `openshift-image-registry:registry` | Reviewed bucket/object lifecycle permissions |
| `nodepool-mgmt` | `kube-system:capi-gcp-controller-manager` | Worker VM/disk/network/tag/service-account attachment; exact worker-SA actAs |

Subjects above expand to `system:serviceaccount:<namespace>:<name>` and exact
installation-specific pool principals, not wildcard subjects. Keep issuer URL
discovery, audience `openshift` and mapping `google.subject=assertion.sub` explicit.
Do not restore inline JWKS or export the management signing key.

Project-wide custom roles reduce broad predefined-role use but **do not provide
resource authorization isolation between installations in one project**. Separate
names, WIF pools, networks and journals do not change unconditional project grants.
Workers sharing a default SA also share that identity. Do not claim inherited/org
privileges have been exhaustively audited merely because no direct worker project
binding was added.

Earlier permission research informs review, not universal sufficiency/minimality:

- CCM's reviewed baseline follows 48 declared permissions plus three operation
  reads. A matched internal-LB request succeeded with regional backend-service use
  without generic backend/network use; that result is request-shape-specific.
  An empty instance group was an invalid LB fixture, not an IAM denial. Do not add
  permissions to repair fixture errors or call the incomplete matrix a full pass.
- PSC requests tested generic **and** PSC-specific create/delete/setLabels rights;
  Get used generic get. Internal address use required `compute.addresses.use` in
  that request, not an inference that `useInternal` is universally interchangeable.
- Compute's Service Directory side effects required namespace-create,
  service-create and service-delete for the tested path. Empty/shared namespace
  cleanup ownership is separate; do not add namespace-delete because an error
  lists a group of candidate permissions.
- A cross-project PSC endpoint can be authorized yet have REJECTED connection
  policy. Distinguish producer acceptance from IAM; do not infer a need for
  producer-project IAM grants from a rejected connection alone.
- The DNS ten-permission candidate covered selected public/private record workflows.
  Existing-zone Get, absent-zone lookup and List have different checks; an absent
  404 alone is not positive proof of Get authorization. Those API probes did not
  prove guest WIF, resolver/delegation behavior or universal role minimality.
- CredentialsRequest lists were incomplete or described different controllers;
  CSI broad-role unions, CAPG versus Machine API, registry bucket-update/tags and
  configuration-dependent discovery must be assessed against the actual payload.
  Keep Resource Manager tags and prefix-scoped alternatives separate from this
  supported baseline until explicitly implemented and validated.

Keep exact permissions in the owning chart's version-controlled contract, not a
second copied permission catalog in this document or the CLI.

### 3. Version resolution and availability

Versionsync owns `Version.spec.wifConfigTemplateRef` and current `Available`.
It preserves pins when updating payload/channel metadata and does not hide a
discovered release because IAM cannot yet be resolved.

- Select the greatest canonical numeric boundary at or below raw `X.Y.Z` in the
  same major/minor. A boundary need not itself be an advertised release.
- Prereleases inherit their numeric base's contract, preserving full release/image
  identity everywhere else. Never use lexical ordering (`5.0.10` versus `5.0.6`).
- Include unverified templates during selection. An unverified highest match blocks
  availability; never fall back to a lower verified boundary.
- A successfully unresolved Version may acquire its first reference later. Once
  set, never repoint it. A new higher applicable boundary produces a review-required
  unavailable condition, not silent reassignment or delete/recreate.
- Require complete release metadata and a verified, intact name/UID/digest pin for
  `Available=True`, with current `observedGeneration`. Missing/False/Unknown/stale
  status never authorizes new selection/dispatch. Keep transition times stable
  across no-op reconciliation.
- Read a complete template snapshot. Read errors/invalid snapshots preserve pins
  and block destructive catalog aging; upstream discovery failure preserves the
  last complete source snapshot. The independent eligibility loop must still
  observe verification changes. POC intervals were 30-second eligibility and
  15-minute upstream discovery; expose/document production settings deliberately.
- Channel default availability is tied to its exact install-default Version and
  membership. No implicit alternative default. Discovery minimum, install default
  and automatic fleet authorization are separate controls.

Lowering a discovery floor can add many Versions and irrevocable references. In
the POC, adding the 4.22 floor produced 39 Versions (36 4.22 and three 4.23), not
one source Version. The 4.23 entries correctly lacked a matching template. Restoring
the old floor would age/delete Versions even though retained templates remain.
Preview the complete source snapshot and consumer impact before such changes.

### 4. Cluster application history and read-only requirements

Public API location:

```yaml
spec:
  platform:
    gcp:
      workloadIdentity: {} # Existing immutable identity/trust configuration.
      iam:
        workerServiceAccount: "<project-number>-compute@developer.gserviceaccount.com"
        templateApplications:
          - name: "5.0.0"
            uid: "<template-uid>"
            digest: "sha256:<canonical-executable-contract-digest>"
```

Application history is alongside, not inside, immutable workloadIdentity.
Bind it to immutable project/infra/WIF/component/worker identity. Entries are
append-only, immutable and keyed by boundary name; the POC limit is 1–128.
Reject duplicate/conflicting refs and any edit/removal/parent clearing across
PUT/PATCH/apply/conversion paths. Unchanged retained entries remain valid after
verification revocation or Version aging, without fresh eligibility reads.
New appends require the referenced intact verified contract. A later numeric name
or equal executable digest alone does not cover a different name/UID reference.

Keep storage fields additive/optional for compatibility, but **do not make omission
a fresh-install dispatch exemption**. No legacy-migration or opt-out field is part
of the design. Existing non-POC fleets need a separately agreed migration strategy;
do not activate new-use gates over such a fleet and silently add exemptions.

Cluster and NodePool public `status.iamRequirements` project committed dispatched
and potentially delivered pending work. They are observations, not cloud attestation
or client-writable dispatch receipts. Include exact release/image/group/template
reference, `state`, object/parent UIDs, observed generation and refresh time.

Complete requires proven receipt/observation identities and explicit non-progressing
feedback; missing/Unknown/progressing observations or desired work absent from
receipts yield Incomplete. Refresh time is controller reconciliation time, **not**
cloud verification time or transport-feedback freshness. The CLI requires Complete,
matching UID/generation, no future timestamp, bounded age (POC: 15 minutes), a
complete related NodePool inventory and a fresh re-read before writing.

POC history entries are also conservatively retained even when their original
Version has aged out. History is not itself proof of a running component, but
retaining its permissions avoids unsafe narrowing.

### 5. New-dispatch protocol and upgrade interaction

Use one shared predicate at installation resolution, automatic/customer approval,
persisted preparation and final HC/NodePool Apply boundaries. Require current Version
availability plus exact verified pin plus exact Cluster coverage for **new work**.
Cached resolution success cannot bypass a fresh final check.

```text
Available catalog target + exact Cluster coverage + existing rollout prerequisites
  → persist Pending receipt with optimistic concurrency
  → external transport Apply of that exact intent
  → non-stale observation promotes Pending to Dispatched
```

Receipts are private controller state and bind object/parent UID, full release,
payload, channel group and full template ref. They exclude incidental whole-object
generation and derived Cincinnati channel. Persist the existing submission-unknown
marker in the same write where applicable. Failed/conflicting persistence means
zero Apply. Status-only/irrelevant generation changes must not invalidate an
already committed exact target.

Keep Pending distinct from last Dispatched. After restart, revocation or upstream
aging, the same committed intent may be redelivered because transport may have
partially succeeded. This is not permission for a different target or rollback.
Unobservable pending work remains locked: absence of feedback does not prove zero
external writes. Diagnose/recover the same target or use normal deletion; no
force-unlock or clear-status escape.

HC/NodePool dispatchers own installation-target `IAMPermissionsReady`. Upgrade
coordinators report future-target prerequisites through upgrade conditions; running
IAM readiness can remain True while a future upgrade is correctly blocked.
Missing target coverage remains discoverable as `WIFConfigUpdateRequired`, not
`NoUpgradeAvailable` or a silent alternative selection.

Preserve supported paths, strict forward ordering, Channel membership/fleet policy,
health, control-plane ceiling, worker step/EUS/skew and applicable maintenance rules.
IAM is an additional prerequisite, not a bypass. Automatic and customer requests
use the same exact-target safety boundary; worker upgrades remain customer-triggered.
Do not convert catalog membership into a per-cluster upgrade-edge API: actual
HC/CVO advertisement is required for the chosen live control-plane target.

**Maintenance is not an execution hold for customer requests.** Existing assertions
can release retained clusters when shared verification opens. The assertion-write
gate does not freeze already recorded coverage, and setting verified false does
not recall a committed rollout. Protect other consumers before shared eligibility
changes; check accepted/preparing/active intents and critical overrides. An
operational single-writer window plus automatic exclusions was an accepted POC
limitation, not a hard production authorization boundary.

### 6. Public PATCH, integer fidelity and persistence safety

Port the generic Orlop fixes as shared correctness work, not CLI-only workarounds:

- Preserve integer representation through decoding/conversion/pruning, CEL
  `oldSelf`, defaulting and patch paths. Include exact values beyond 2^53 and
  unchanged integer-bearing fields in regression tests (GCP-1452).
- Support RFC 6902 JSONPatch on the stored **public projection**, in addition to
  existing merge patch. Authorize the stored object before evaluating tests and
  the resulting object before storage. Preserve hidden metadata and complete
  controller/private status through conversion, including nested arrays.
- Use explicit content-type parsing; do not accidentally introduce strategic merge
  or server-side apply. Preserve deliberate legacy media-type compatibility until
  an explicitly reviewed deprecation changes it.
- CLI application writes test object UID and a freshly fetched resourceVersion.
  Reject attempts to mutate/remove server-managed identity/revision. Failed `test`
  is a fixed sanitized 409; malformed/non-object results use bounded sanitized
  400 categories, without compared values or raw decoder/provider errors.
- Snapshot tests are not enough: storage Update must atomically enforce the fetched
  revision. POC memory/Spanner support this; **Postgres lacked revision CAS**.
  Fix and transaction-test Postgres or explicitly keep assertion-writing unsupported
  on that backend. Do not claim cross-backend concurrent-write safety from the live
  negative test-op checks, which were not a real concurrent Spanner race.
- POC bounded JSONPatch copy amplification to 3 MiB and rejected negative indices.
  Define request-body/operation-count limits and regression-test existing PUT/merge
  private-field/identity preservation during integration; the POC did not repair
  every legacy update edge case.

The early Spanner inventory also recorded **HTTP 500 label-selector LIST failures
from invalid JSONPath handling**. Those paths were not shown repaired or retested
by the POC. Integer preservation, plain LIST success and revision CAS do not repair
or prove selector correctness. Reproduce selector LIST and selector-filtered watch
on the target integration revision for each supported backend. Here, a supported
backend includes the list/selector/watch paths actually used by controllers, the
CLI and the catalog, not just point CRUD. If the issue persists, track an owning
storage follow-up and gate affected production use on its resolution.

### 7. CLI planning, creation and owned IAM mutation

Reuse the catalog client and injectable pure planner/executor; do not implement
boundary selection or embed a second permission catalog in the CLI.

1. Resolve an exact current Available Version and fetch its pinned contract.
2. Verify schema/digest/eligibility, customer project number, stable infra ID,
   exact existing default worker SA/unique ID, issuer and supported grant topology.
3. Preview exact resources, permissions, scope/member/condition changes and a stable
   plan fingerprint. Dry-run creates no ownership journal or cloud resource.
4. Confirm the reviewed fingerprint and establish durable intent before RPCs.
5. Create owned custom roles/SAs/pool/provider and minimal bindings. Verify exact
   cloud readback, create/verify owned networking where requested, then reverify IAM
   before Cluster POST with the same pinned source assertion and maintenance settings.

Keep standalone `iam create`, integrated `cluster create --setup-infra` and config-
file flows on the same engine. Config files are identity/provenance hints, not fresh
verification. Explicit external networks are read-only input, not adopted cleanup
targets. Initial payloads must not submit success assertions for partial setup.

Resource IDs derive from protocol naming templates; role IDs hex-encode infra-ID
bytes rather than truncate lossy names. POC maxInfraIDLength=12 and related limits
are this protocol's bounds, not a universal runtime naming requirement. Refuse
foreign/deleted collisions; no automatic undelete, trust repointing, enabled APIs,
extra permissions after denial, SA keys or broad predefined-role fallback.

Preserve the final CLI's ownership protocol, not an earlier prototype's different
markers: `Journal.Owner()` binds the random installation operation ID and a hash
of original project number, infra ID and **original contract digest**. An appended
target contract must not retarget that original ownership identity. Runtime and
bootstrap resources have their own recorded owner markers; compare the appropriate
kind/name/UID and marker rather than treating names or a new target digest as
authority. Include custom-role quota/headroom and retired-name checks in preflight;
soft-delete tombstones accumulate, and quota/purge behavior needs current service
validation rather than assumptions about automatic name reuse.

Use policy v3, fresh etags and exact role/member/condition matching. Preserve
unrelated bindings, members, conditions and audit configuration. Recheck object
ownership before writes, then observe actual state before reporting success.
Keep caller IAM-admin authority separate from runtime component grants; never add
platform management IAM-admin authority to perform this workflow.

Carry these concrete SDK fixes forward:

- Federation create RPC parents are the containing location/pool, not a collection
  path that the SDK would append a second time.
- `queryTestablePermissions` omission of custom-role support means default SUPPORTED;
  it is not unsupported. Preserve pagination (up to 1000/page), bounded deadlines,
  and rejection of absent/unknown/NOT_SUPPORTED, API-disabled or predefined-only
  permissions. TESTING support needs a deliberate documented production policy,
  not confusion with a permission's GA launch stage. The observed example was
  `compute.addresses.useInternal`: GA launch stage did not imply SUPPORTED
  custom-role status.
- Preserve API camelCase in CLI JSON/YAML, including `iamRequirements`; do not
  serialize lowercased Go field names. Keep private receipts private.
- Version payload is private catalog data. Target UID/generation provides the CLI's
  indirect payloadCatalogBinding; it is not registry digest attestation. Public
  observed requirements expose their defined observed payload fields separately.

### 8. Target-aware updates and retention

`iam update <cluster> --version <exact-release> --channel-group <group>
--installation-state <original-journal> --dry-run` reads all related resources,
fresh running/pending requirements and retained history. Retention permits only
permission unions within identical project/worker/component identities, WIF trust,
role identities and exact grant topology. Reject unrepresentable scope changes and
permission/grant cross-products. The union is not a synthetic executable contract
and cannot be fed to the create-only engine as one.

Confirmation uses `--confirm --expect-plan`. Only additive role includedPermissions
updates and the exact application append are allowed; title, ownership description,
stage, bindings, trust, worker SA and release requests remain unchanged. Fresh
ownership/etag checks precede role writes; field masks change only permissions.
Keep original and new role content/etag lineage for exact before/after recovery.

Same exact reference needs no duplicate application. Different name/UID with equal
digest can require a new assertion while making **zero role-permission RPCs**.
Independent complete cloud verification precedes that append. Cloud success and
recording success are distinct outcomes; failed/uncertain recording must not be
reported as completed or bypassed with a record-only command.

Stable fingerprints bind UIDs, specs, target generation/contracts, complete NodePool
inventory and required permissions, not status-only resourceVersions/refresh times.
Still fetch current resourceVersions immediately before guarded writes.

**Pending-intent integration gap:** the POC refuses IAM-update preview when an
unaccepted requested release makes requirements Incomplete. The live proof withdrew
only the unaccepted intent, verified/appended, then resubmitted the exact target.
Retain that documented safe workflow until a focused change separates proven
running/pending requirements from a blocked requested candidate. Do not weaken the
Complete predicate or invent an empty running set. If unchanged-intent recovery is
required for the product workflow, implement it with tests proving conservative
retention and exact-target reevaluation. A target assertion can immediately release
an already pending customer upgrade; there is no implicit approval pause.

### 9. Journals, failure handling and cleanup

Use one authoritative installation primary and fixed linked `<primary>.updates.json`.
Updates/cleanup preserve original primary bytes; progress and updated role lineage
live in the sidecar. Private files/directories, ownership/type/symlink checks, local
locking, fsynced intent before RPC, atomic replacement and durable directory updates
are required. Initial sidecar publication must not leave an empty authoritative file
on a crash; POC uses exclusive hard-link publication of a fully written temp inode.
Validate filesystem support; do not mistake a local lock for a distributed lock.

Unknown RPC outcomes are reconciled against exact before/authorized-after state.
Do not retry a mutation merely because the response was lost, use a new random
installation ID, accept an arbitrary superset or hand-edit a journal. Unknown
Cluster POST followed by absence is not proof of no submission; do not auto-rePOST
or destroy its IAM on that alone. Abandon-unstarted, where supported, applies only
to wholly unattempted Prepared work and preserves a tombstone.

Cleanup sequence:

1. Complete ordinary hosted Cluster/NodePool deletion while their IAM still works.
2. Prove repeated API absence in the original realm, complete namespace/reference
   inventory and no cloud VM/address/forwarding dependencies on owned identities/network.
3. Preview exact journal-owned cleanup; check `cleanupEligible` and block, not only
   exit code. Read-only CLI refusals can intentionally return structured exit-0 output.
4. Confirm separately with `--include-owned-network --yes` when appropriate.
   Network cleanup uses exact numeric IDs/owner/shape and reverse dependency order;
   IAM removes only owned membership deltas/resources, preserving shared worker SA.
5. Verify resource absence/soft-delete tombstones and unrelated policy restoration.
   Retire IDs and keep journals; never automatically undelete/reuse them.

The final CLI fix recognizes only a definitive 404 for an already Verified exact
bootstrap Firewall with recorded UID/no outstanding delete operation after teardown
proof. Pre-5.0 setup creates that marked rule; later CPO can adopt/remove it. Persist
the resolved absence via the normal journal without another delete. Missing VPC,
subnet/router, unknown creates, changed present firewall and unreadable state remain
strict refusals. Present-but-CPO-modified firewall handling is not covered by this
fix. A 404 does not identify its deleting actor; do not label it cloud attestation.

The last worker's guest PodDisruptionBudget can block standalone NodePool drain.
Normal whole-cluster deletion resolved the POC's migrator-PDB stall. Keep a separate
product decision/test for standalone last-pool deletion; do not generalize that
observation into a force-drain/finalizer-removal recovery policy.

**Catalog aging limitation:** CLI verify/update/destroy currently bind API realm
through the original Version UID and can fail after that Version ages out, despite
retained templates. Before production, decide whether to retain referenced original
Versions for the supported journal lifetime or introduce a stable authenticated
realm identity binding. Do not simply omit the realm check or accept arbitrary 404s.

### 10. Release comparison bridge

Port comparison-only support if still needed on the target baseline. The POC follows
pinned HyperShift's 5.x → 4.(23+x) logical comparison mapping for order/skew/next minor,
preserving patch/prerelease/build details and failing closed on unsupported majors
or overflow. **Never normalize stored release names, payloads, graph membership,
WIF raw-boundary selection, pins or assertions.**

Keep strict forward admission, exact advertised identities, sequential/EUS steps,
worker-not-ahead-of-control-plane checks and active observed locks. Equal logical
aliases cannot authorize an upgrade. If aliases are advertised, deterministic raw
selection must not substitute an unadvertised alias. Advancing a 4.22 installation
with fleet 5.0 must query actual candidate-5.0, not fabricate a 4.23 channel identity.
Emergency critical-pin maintenance bypass remains same **raw** major/minor only.

Verify actual payload/CPO support through normal configured pulls and HyperShift;
do not make a standalone credential-inspection task a prerequisite. Check source
and target boot images/architecture and the live OSStreams configuration. RHCOS 9
to 10 worker replacement is expected for this tested edge; no silent stream override.

### 11. Packaging, authorization and staged rollout

App-specific catalog/chart code belongs in Gecko; environment Argo/Endpoints and
monitoring wiring belongs in gcp-hcp-infra. GitOps is the catalog authority, with
opt-in entries unverified and `iamApplications.enabled:false` by default.

The POC chart renders all boundaries from one baseline file. **Do not edit that
file to introduce a genuinely different future contract at an existing boundary.**
Extend packaging to retain per-boundary immutable contract assets before permission
sets diverge; test old manifests/digests remain identical. API deletion is initially
denied; manifests use deliberate retention rather than prune/delete/recreate.
That published chart file is now the baseline authority. Its historical header
still mentions the removed experiment-path source; correct that provenance comment
during WP4 integration without changing the immutable published contract contents
or rewriting the archived POC commit.

Production authorization must follow the newer platform-scoped Cedar ADR. The POC
template source still contains an authenticated-read exemption. Replace/reconcile
it with explicit generated get/list permissions, platform role/binding grants,
trusted authentication, scope separation and per-item conditions/list filtering.
Keep all public template writes unavailable; delegated private RBAC restricts writes
to platform/GitOps. Do not extend namespace-role scope by accident. Update ESPv2/
Endpoints collection **and** item GET routes; server-only support initially returned
frontend 404 until routing was added. Authentication, Cedar and Kubernetes RBAC
are separate boundaries and each needs tests.

Safe delivery order:

1. Integrate/release matching Orlop/API types and supported storage CAS, with writes
   disabled; wait until **all API replicas** understand the contracts.
2. Deploy compatible typed producers and all dispatch consumers before first pins/
   assertions. Old typed writers can drop newly added fields. Deploy API support
   before requirements-status producers and external CLI consumers.
3. Seed retained unverified contracts before expanding discovery. Confirm complete
   expected catalog/pin changes, routing, authz, persistence and controller readiness.
4. Protect retained consumers before shared verification, then explicitly enable
   selected templates and assertion writes for the supported rollout window.
5. Observe actual running digests, guest/worker completion and recovery; restore
   intended gate state via GitOps rather than patches self-heal would undo.

There is no safe old-binary rollback after first unsupported pins/assertions. Keep
compatible artifacts or forward-fix; never erase history or switch Spanner to
memory. No transaction spans catalog revocation and external transport.

The API chart's migration Job uses Force/Replace semantics and formerly followed
the API image. Pin its image independently; compare the complete rendered Job
before every nominally catalog/API-only change. `ApplyOutOfSyncOnly=true` prevents
reapply of an unchanged Job, **not replay of a changed Job**. Preserve storage and
pruning safeguards; execute real schema migrations only through an explicit plan.

The POC enabled a ninth NodePool-upgrade Deployment only in personal dev. Check
actual rollout inventory rather than assuming that controller exists or deploying
every monolithic-image consumer. Before broader promotion remove its unnecessary
NodePool main-resource patch verb and replace broad DNS egress with reviewed cluster-
specific access; retain only required status writes and API/DNS connectivity.

### 12. Observability and operational support

POC condition/log samples are not production alerting. Add bounded-label metrics
for availability/missing status, transitions, refresh/source failures, last successful
reconciliation and blocked prerequisites. Remove series when catalog objects age out;
do not use customer IDs, UIDs or contract digests as unbounded labels.

Use the existing Google Managed Prometheus collection and Cloud Monitoring alert
packaging. Do not assume a PrometheusRule delivery stack. Define alert scope and
hold periods for offered/support-required releases and Channel defaults, so intentionally
unverified future discovery candidates and customer-action IAM blocks do not page
indiscriminately. Alert on stale reconciliation even if an old Available=True remains.
Document supply-contract, restore-catalog/reconciler, explicit customer-IAM remediation
and safe partial-outcome recovery. No alert remediation may automatically widen IAM.

## Delivery work packages

Each work package needs owning-repository review, focused tests and acceptance;
production completion follows the team DoD, not the fact a POC branch exists.
None of these packages requires the prefix/naming work in Plan 2. Any prerequisite
HyperShift change here concerns existing worker-account acceptance or supported
runtime behavior, not new resource-prefix inputs or component renaming.

| WP | Repository / source anchors | Deliverable and acceptance |
|---|---|---|
| 1 — Rebase/integration inventory | Gecko/CLI current main versus published tips; Gecko `orlop`, `platform-api`, `controllers`; CLI public API dependencies | Map reusable commits and changed current APIs, especially authorization and upgrade request entrypoints. Split generic fixes from feature changes. Document baseline failures; no blind branch merge or history rewriting. |
| 2 — API/storage correctness | Gecko `orlop/pkg/apiserver/{conversion,handlers,schema}`, storage backends | Integer/CEL fidelity, public JSONPatch stored/result authorization, complete hidden-state preservation and real revision CAS. Reproduce selector LIST/watch on every supported backend and track any remaining Spanner JSONPath failure in owning storage work. Backend matrix explicitly states supported assertion writers and query paths. |
| 3 — Catalog and application API | Gecko `platform-api/api/private/v1/wifconfigtemplate*`, `version_types.go`, Cluster IAM types/validation, generated public files | Typed v1 schema, compatible canonical digest, immutable retained contracts/pins, one verified flag and bounded append-only records. Generate twice, stable second pass; no omission bypass. |
| 4 — Authorization/routing/catalog packaging | Gecko type markers/role grants/chart templates; infra Endpoints and Argo | Reconcile platform Cedar ADR, authenticated GET/list route support and no public writes. Opt-in unverified per-boundary immutable manifests, minimal RBAC and pruning protection. |
| 5 — Discovery and dispatch | Gecko `controllers/versionsync`, `util/catalog`, resolution, `hc`, `nodepool`, `controlplaneupgrade`, `nodepoolupgrade` | Single availability owner/predicate, correct floor/prerelease behavior, independent refresh, preserved pins, receipt-before-Apply and target-safe restart/redelivery. All actual upgrade entrypoints audited. |
| 6 — CLI creation and cloud adapter | CLI `pkg/platformapi`, `wifconfigtemplate`, `infra/iam/application`, `gcp/iam`, `gcp/networking`, `cluster/provision.go` | One planner/executor, no embedded fallback, SDK fixes, stable pinned intent, read-only preview and cloud-before-record behavior across standalone/integrated/config-file paths. |
| 7 — Requirements/update/recovery | Gecko requirements types/projection and publishers; CLI `update_*`, `retention.go`, journal/teardown code | Conservative UID/gen/freshness-bound retention, role-only additive CAS writes, linked crash-safe state and explicit record failure. Resolve or document pending-intent and Version-aging product constraints without weaker gates. |
| 8 — Worker/runtime/cleanup compatibility | Gecko worker manifest; HyperShift NodePool worker-SA schema; CLI `infra/network/cleanup.go`, update-aware destroy | Exact default-SA propagation and actual schema acceptance; preserve component custom-SA validation. Upstream the narrowly fixed firewall lifecycle handling, strict owned teardown and tombstone behavior. Specify last-pool drain support. |
| 9 — Comparison/upgrade semantics | Gecko `platform-api/pkg/ocpversion`, validation, resolution/coordinator consumers | Isolate comparison-only bridge from raw catalog/WIF identity; real advertised edge and CP-then-worker tests. Omit duplicate work if current main already supplies equivalent tested behavior. |
| 10 — Dependency/release alignment | Gecko public API/Orlop modules and CLI `go.mod`/CI | Publish compatible modules and pin reproducible public dependencies. Clean standalone build/test with no machine-local workfile/replacements; source publication is not this acceptance gate. |
| 11 — Operations and representative E2E | Gecko metrics/conditions; infra GMP/alerting/Argo; owning runtime tests | Least-privilege coordinator deployment, sustained alert firing/clear/freshness, complete functional/mixed-version/failure/restart/owned-cleanup validation and rollout/forward-recovery runbooks. Verify the actual controller/CLI/catalog selector LIST/watch paths on supported backends; require owning-storage remediation if the historical JSONPath failure persists. |

Dependencies: WP2/3 precede typed dispatch/requirements producers; WP4/5 must be
compatible before first enabled assertions; WP6/7 depend on published API shape;
WP8/9 are required where the selected release/runtime needs them; WP10 is required
for a supported standalone CLI; WP11 closes production readiness. Packaging and
observability can be developed in parallel without enabling live gates.

The worker-email fix was CRD/schema-only in the POC. Integration must deliberately
carry that behavior in the normal HyperShift source/CRD distribution, preserve
conversion CA information and avoid an accidental operator rollout just to update
schema. Temporary personal-dev source/annotation freezes are operational safeguards,
not product configuration to copy into shared defaults.

## Validation and acceptance

### API and reconciliation automation

- Exact/floor/numeric ordering, shuffled input, malformed names, raw prerelease base,
  no cross-minor fallback, highest-unverified and newly added-boundary behavior.
- Missing/False/Unknown/stale availability, failed/partial snapshots, first-reference
  late binding, immutable pins, source aging, no-op status and producer restart.
- Template mutation/deletion denial; schema/list limits; all assertion edit/clear/
  duplicate/unknown ref cases; retained-history updates after revocation/aging.
- Stored/result authorization, namespace/platform scope separation, public/private
  filtering, integer fidelity and real concurrent storage CAS for every supported
  backend. Distinguish UID/RV test-op rejection from a concurrent-write race.
- Initial Cluster/NodePool, automatic/customer/critical-override, cached resolution,
  preparing/pending/submitted delivery and wrong-parent/target paths; persist before
  Apply and zero Apply on failed persistence. Restart/same-target partial delivery,
  no force-unlock, unrelated generation changes and template revocation races.

### CLI and failure automation

- Zero-write previews (including no journal), exact resources/grants/subjects,
  collision/soft-delete refusal, schema/digest mismatch and worker unique-ID drift.
- Permission-support defaults/TESTING policy/pagination; minimal SDK field masks,
  etags, policy-v3/condition/audit preservation and no unsafe role/grant unions.
- Complete related inventories, stale/future requirements, mixed CP/worker contracts,
  retained unverified history, unchanged plan fingerprints under status churn.
- IAM denial/partial success, actual before/after response loss, process/fsync/save
  failures, successful IAM with failed/uncertain assertion, idempotent readback and
  guarded resume. No false success, duplicate assertion or record-only bypass.
- Independent teardown/realm proof, foreign consumers, unknown POST, partial cleanup,
  IDs/owner drift, exact missing-bootstrap-firewall exception, other missing-resource
  refusals, linked-only progress and unrelated-state/default-SA preservation.

### Representative runtime matrix

| Workflow | Required observation beyond IAM configuration |
|---|---|
| Worker creation/replacement | Actual VM/node Ready and attached exact worker SA; skew, surge and old/new OS behavior |
| Storage | Supported provision/attach/mount/read-write/detach/delete, resize and snapshot paths with appropriate test volumes |
| Registry | Supported authenticated image push/pull and owned bucket/object lifecycle; explicitly select optional features |
| Cloud network | Actual supported network-controller operation under guest WIF, not just a policy GET |
| Service/LB | Selected internal/external service reconciliation and cleanup using valid fixtures and declared request shape |
| CPO/PSC/DNS | Endpoint lifecycle/connectivity policy, real DNS/delegation where supported, issuer discovery and full settled teardown |
| IAM-gated upgrade | Eligible advertised exact target blocked only on missing coverage; verified application releases only intended target; CP and worker actual completion |
| Recovery/coexistence | Failures/restarts and original-installation survival during same-project setup/update/cleanup; no hard-isolation overclaim |

Use the existing owning-repository test/CI harnesses; do not ship retired one-off
mutation scripts as product tests or rerun historical resource names. Isolated
API permission characterization needs valid full-set controls and actual completed
operations; 403 text, missing-resource 404s and accepted asynchronous requests are
not substitutes for causal authorization/settlement proof.

Generate affected APIs twice, run focused tests/race and module lint/Helm first,
then the owning-repository standard suites/coverage/CI per DoD. Record skips and
backend/runtime limits explicitly. Do not perform cloud mutation simply to edit or
review this plan: live testing requires a selected footprint, time/cost limits,
permissions and ownership-safe cleanup scope.

## Risks, decisions and completion criteria

| Item | Required disposition before corresponding production use |
|---|---|
| Postgres revision CAS | Implement and transaction-test, or explicitly exclude assertion writes on that backend |
| Spanner selector JSONPath failure | Freshly reproduce selector LIST/watch per supported backend; plain CRUD, integer fixes and CAS are not proof this historical HTTP 500 is resolved. Track/resolve owning-storage follow-up before relying on affected query paths |
| Catalog authorization drift | Integrate platform-scope Cedar permissions/bindings rather than retain POC exemptions |
| Single-baseline chart | Support immutable distinct boundary content before publishing different permission requirements |
| Pending-intent requirements | Document safe withdraw/apply/resubmit or implement conservative unchanged-intent recovery; do not claim POC proved it |
| Original Version aging | Stable realm identity or supported reference retention; no weakening of teardown realm proof |
| Shared eligibility | Define fleet rollout/protection policy; closed write gate and maintenance exclusions are not universal holds |
| Worker/account semantics | Upstream schema acceptance, exact SA propagation and bounded replacement supported; no default-SA creation/deletion |
| Cleanup handoffs | Narrow absent-firewall recovery delivered in POC; present adopted-rule mismatch and standalone last-pool PDB behavior remain explicit |
| Role support/security boundaries | Review TESTING permissions, optional feature coverage and broad project-role/shared-SA exposure; naming isolation separate |
| Controller promotion | Remove excess main NodePool patch and broad DNS egress; verify endpoint/DNS configuration and readiness |
| Validation depth | Finish functional/failure/restart/observability coverage rather than equate one upgrade with full runtime authorization proof |

Completion means reviewed and merged owning-repository changes, reproducible normal
module/CLI builds, compatible GitOps rollout with all consumers gated, supported
backend concurrency guarantees, real required functional/upgrade/cleanup proof,
monitored operations and user-facing recovery documentation. Jira/story status,
stage deployment and human approvals follow the team's Definition of Done.
Published POC branches and AI-review approvals alone do not satisfy those gates.

## Relationship to resource-name-prefix conditions

Plan 2 owns resource-name conditions, component naming inputs/capabilities,
authoritative prefix reservations and any new condition-aware contract semantics.
The Plan 1 v1 contract must continue rejecting unsupported scopes rather than
partially rendering or ignoring them. Its role-only update protocol deliberately
cannot move arbitrary project grants to conditioned grant topologies.

Delivering Plan 1 does not authorize removal of unconditional grants from existing
installations. A future Plan 1 → Plan 2 transition needs explicit policy/role
retirement, mixed-runtime compatibility and separate acceptance. Keeping both
unconditional and conditioned grants does not enforce the condition because
independent IAM grants are additive. See Plan 2 for that dependency and transition
design; there is no hidden auto-pruning step in this plan.
