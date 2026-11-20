# Phase 4 fresh-cluster upgrade and same-project isolation feasibility

**Later decision:** the user selected **4.22.21** as the source instead. Follow
the [selected-pair preparation record](phase-4-42221-preparation.md) for current
scope. The 4.23 NO-GO and isolation findings below remain historical evidence.

**2026-11-20. Read-only checkpoint complete. NO-GO for provisioning the requested
4.23 → exact 5.0.0 scenario: no supported path was found in the queried graphs.**
No source code, catalog, IAM, gate, deployment, journal or customer resource was
changed. This records the user-approved feasibility/isolation-design slice, not
approval to allocate a second cluster or repair version validation implicitly.

## Current retained baseline

- At **07:44:49 UTC**, fresh regional/management reads confirmed the API at
  `7319d8e...` and running digest
  `sha256:5f4a2ad3201aff2d0a6dd01591a67480000cc7609a31d660cde2b7831c328a2c`,
  all eight Deployments Ready, both gates false, original Cluster/NodePool
  UID/generation/spec/release/Dispatched receipts and no requested upgrade.
  Cluster RV 525, NodePool RV 483 are normal status observations, not new intent.
- Catalog: **21 Versions, 19 retained pins, one template `5.0.0` verified=false**.
  No 4.23 Version or 4.23 template exists. Candidate/fast/stable/eus Channels all
  have `minimumSupportedVersion: "5.0"`, `fleetMinorVersion: "5.0"`, and pinned
  install default `5.0.0`. Lowering the catalog floor is a platform-wide config
  change, not a per-cluster installation flag; none was made.
- Original migration Job UID `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, original
  creation time and Complete/Synced state; Spanner UID
  `99e4163c-79f9-4875-92b9-0e576b9a5927` Ready; exact Spanner backend retained.
- HyperShift Application Synced/Healthy at CRD-only source
  `914334ae61352fa5641e3f1d50250e87299e9eb3`, frozen stamp
  `3d8a55cb8c633329d2a64939e7e0ab20520107ee`. Actual operator image and
  supported-versions ConfigMap report running source `d3173a012ac0a2b188095aaa6722438b33ff7602`.
- At **07:46:16 UTC**, checksum-verified native CLI `a727640...` read-only
  `iam verify wif3-7d9e --state-file <primary>` returned
  `cloudReadbackMatches=true`, `updatePhase=NoUpdates`, `updateComplete=true`,
  `eligibleForNewUse=false`, original operation and template reference.
  "NoUpdates" does not mean an IAM update was executed.
- Independent guest reads at **07:48:24 UTC** confirmed 5.0.2 history Completed,
  Available=True, Failing=False, Progressing=False; original worker UID
  `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab` Ready with unchanged provider ID.
  Primary and both guard journals retain their recorded hashes, with no update
  sidecar. Neither guard harness was run again.
- Closing regional readback at **07:51:45 UTC** again confirmed unchanged
  original UID/generation/spec/receipts, all eight Deployments Ready, both gates
  false, all three journal hashes unchanged and no linked update sidecar.

Supporting GET/policy/graph snapshots are local-only, mode-0600 files under
mode-0700 `.../opencode/wif-p4-423-isolation-feasibility-pmiamtvy`. No Secret
objects, bearer tokens, component impersonation credentials or private keys
were stored. This document contains the durable credential-free findings.

## Release discovery — do not confuse graph nodes with supported edges

Public HTTPS graph reads used the deployed controller's configured endpoint
`https://api.openshift.com/api/upgrades_info/v1/graph`, `arch=amd64`, normal TLS,
no proxy/redirect, and no mutations. Snapshots were refreshed through
**07:47:28 UTC**. Direct and transitive unconditional paths and relevant
conditional edges were inspected.

| Graph | Observation relevant to the requested path |
| --- | --- |
| `candidate-4.23` | 39 nodes, 258 edges. Only `4.23.0-ec.0`, `ec.1`, `ec.2` in 4.23. Edges connect those ECs; no `5.0.0` target node. |
| `candidate-5.0` | 55 nodes, 241 edges. Exact `5.0.0` exists; no 4.23 source nodes. |
| `candidate-5.1` | 24 nodes, 30 edges. Both 4.23 EC nodes and `5.0.0` exist, but no path between them. |
| `fast-4.23`, `stable-4.23` | Empty graphs. |
| `fast-5.0`, `stable-5.0` | Exact `5.0.0` exists; no 4.23 source nodes. |

No relevant conditional 4.23-to-5.0.0 edge was found either. This finding is
bounded to these seven fresh graph queries; it is not a claim about every
possible channel or a future graph. **Do not fabricate an edge or relabel an
image/version to make the scenario run.**

The observed 4.23 candidate `4.23.0-ec.2` has payload
`quay.io/openshift-release-dev/ocp-release@sha256:03ad1a36aab2a223721f18098a38cd0393b4062dd130429e735d2e480583920b`.
It is an EC, not 4.23 GA; graph presence is not proof of GCP installability.
No starting release was selected or published.

Exact target `5.0.0` already has Version UID
`9bbbd3d4-d2c6-442c-942d-4c65c3c873a5`, generation 2, payload
`quay.io/openshift-release-dev/ocp-release@sha256:5d7ed8b3e98bc3a10c50f36f07262a962d59ad2d3ea5fb4935ff10c408c02e55`,
and the original immutable `5.0.0` template reference. It remains unavailable
for new use because that template is unverified. No pin/payload was rewritten.

### Observed alternative — not silently substituted

Both **candidate-5.0 and fast-5.0 advertise a direct `4.22.21 → 5.0.0` edge**.
The source payload is
`quay.io/openshift-release-dev/ocp-release@sha256:828fa71fe839d280cd731271d04917c5a5ca7f7ce49dd68e3eb1a5d37f9f7f02`.
This is a candidate for a *revised* proposal, not authorization or proof of a
GCP installation, CPO/boot-image compatibility, or eventual HC advertisement.
It needs the user's agreement to change the source release, catalog/template
preparation and the compatibility work below before any allocation.

Starting at `5.0.0-rc.8` or `5.0.0` offers other advertised upgrade choices, but
those releases currently share the same `5.0.0` template reference. Such a test
can validate coexistence/upgrade, not missing-target-template assertion gating;
do not present it as equivalent WIF contract-transition acceptance.

## Compatibility findings from the exact selected sources

1. **4.23 and 5.0 are dual-versioned equivalents in pinned HyperShift.**
   `support/supportedversion/version.go:43-66,390-408` maps both to Kubernetes
   **1.36** and normalizes 5.x to 4.(23+x), retaining patch/prerelease identity
   for comparisons. The live supported list includes 4.23 and 5.0. A diff of
   this file against actual running `d3173a0...` is empty. This is not a generic
   major-version/Kubernetes-major upgrade. Supported-version listing alone does
   not prove the chosen GCP payload's CPO or boot-image functionality.
2. **Gecko worker comparisons do not implement that bridge.**
   `platform-api/api/private/v1/nodepool_upgrade_validation.go:133-153` requires
   the target major to match the observed control-plane major and forbids
   differing current/target majors in `ValidNodePoolUpgradeStep`. Any future
   repair needs normalized release/skew semantics, downgrade/unknown-major and
   prerelease tests, not simply deletion of the major checks. Keep actual
   version, payload and immutable WIF boundary identities unchanged.
3. **Automatic control-plane selection also lacks the bridge.**
   `controllers/controlplaneupgrade/target.go:71-106` requires matching fleet
   major and same-major next-minor candidates. Customer-requested exact targets
   instead use mirrored HC `AvailableUpdates` in
   `platform-api/api/private/v1/cluster_upgrade_validation.go:178-207`; semver
   "newer" alone cannot bypass the advertised-target check.
4. **Catalog discovery currently excludes 4.x.**
   `controllers/versionsync/versionsync_controller.go:230-276,448-472` starts
   at Channel minimum 5.0 and filters lower raw majors. A new 4.23 or 4.22
   template by itself cannot produce an installable Version. Any approved
   minimum change must preserve existing pins/defaults and scope its fleet impact.
5. **Maintenance is not a hold for a customer-requested upgrade.**
   `controllers/controlplaneupgrade/inline.go:100-125` consults maintenance only
   for automatic targets. Existing test
   `TestReconcile_InlineManualWaitsForHealthAndBypassesMaintenance` confirms it.
   After setting a desired target blocked on IAM, appending the target assertion
   may release that customer upgrade immediately. Treat assertion + release as
   one explicit execution/window approval, or separately design a real hold.
   Do not promise "append now, remove maintenance exclusion later" on this path.
6. **Shared verification affects the retained installation too.** Enabling the
   existing `5.0.0` template makes its other pinned Versions eligible. The retained
   cluster already has that assertion; the assertion-write gate is not a global
   dispatch freeze. Before opening shared eligibility, agree on protection against
   an automatically advertised new target for `wif3-first`. A temporary exclusion
   on it would itself be an explicitly approved spec change, not "untouched".

The inspected Gecko worker/upgrade source files have no diff from deployed
controller source `9f7161a...`. Existing focused tests for worker update rules,
different-major automatic selection, manual maintenance bypass and maintenance
evaluation passed. HyperShift's existing `TestGetKubeVersionForSupportedVersion`,
`TestNormalizeToV4`, `TestValidateVersionSkew` and `TestIsValidReleaseVersion`
also passed from the preserved fork checkout. All focused runs used `GOWORK=off`
and `-count=1`. No tests or implementation were added or altered. Full repository
lint/test/generation was not rerun for this source-read/docs-only assessment;
payload/CPO execution and GCP boot-image compatibility remain unproven.

## Same-project isolation: what can and cannot be claimed

At **07:46:57 UTC**, independent read-only project and SA `getIamPolicy` RPCs
requested policy v3. These methods use POST but are **policy reads**, not
`setIamPolicy`. Live readback found:

- Six installation-specific custom-role project bindings, each **unconditional**.
  They have no per-cluster resource-name/label condition.
- Six separate component SA policies bind only exact subjects under the original
  WIF pool: one per component except image-registry's two expected subjects.
  Original provider ACTIVE, exact issuer ending `/wif3-7d9e`, audience
  `openshift`, mapping `google.subject=assertion.sub`.
- Shared default worker SA has no direct project-policy role binding. Its
  `roles/iam.serviceAccountUser` membership includes the original CSI/NodePool
  managed identities. The new installation would share the worker SA identity,
  not establish a new per-cluster worker authorization boundary.

Thus the proposed test can establish **owned lifecycle/coexistence and specific
WIF identity separation**, not hard isolation of all GCP resources. Some
cross-cluster resource reads/operations are authorized by the current project
roles. Do not expect universal 403s or treat different names/networks/journals as
an authorization boundary. Inherited/org privileges and active cross-access,
impersonation, STS or destructive probes were not assessed/executed here.

| Test area | Proposed acceptance after separate approval |
| --- | --- |
| Ownership | New journal cannot adopt or delete original owned identities/resources; mismatches stop before writes. Use non-destructive/offline negative cases, not deletion attempts on the retained installation. |
| Shared policies | Preserve unrelated and original binding members/conditions using fresh policy etags. Adding a second installation legitimately changes project/default-SA policy etags; compare ownership-specific content, not byte-identical shared policies. |
| WIF | Review negative issuer/audience/pool/subject cases and exact probe footprint before token minting/exchange. No implicit permission to impersonate retained component SAs. |
| Upgrade targeting | Only the new Cluster UID/application history and intended NodePool may advance; original release, receipts and actual guest must remain unchanged. |
| Network | Distinct VPC/subnet, no accidental peering/shared routes, and an explicit expected-reachability matrix. Public endpoints intentionally reachable are not an isolation failure. |
| Teardown | Normal hosted teardown first; remove only second-journal-owned resources/memberships. Preserve default worker SA, original grants/resources, storage and the retained guest. |

Immutable platform templates and Version pins are retained under the existing
policy, not deleted as part of customer-resource cleanup. Restore approved gate/
catalog configuration deliberately without erasing retained contracts/history.

## Conditional resource and IAM proposal — nothing reserved or created

Tentative names checked absent with authenticated reads:

- Project `apahim-dev-test-1` / `35809355916`; region `northamerica-northeast2`,
  proposed one-worker zone `northamerica-northeast2-a`.
- Cluster **`wif4-isolation`**, public NodePool **`wif4-workers`** (not the already
  occupied project-wide `workers` name), infra ID **`wif4-iso1`**.
- Six `{infraID}-{component}` SAs; six
  `hcp_776966342d69736f31_{logicalRole}` custom roles;
  pool `wif4-iso1-wi-pool`, provider `wif4-iso1-k8s-provider` beneath it.
- Network/subnet/router `wif4-iso1-network`, `wif4-iso1-subnet`, `wif4-iso1-router`,
  NAT `wif4-iso1-nat` within the absent router. CIDR/range validation remains part
  of a future concrete network proposal. Separate protected ownership/update
  journals; never reuse the original or either guard journal.
- All 16 exact cloud-name GET checks returned 404. Additional complete paginated
  role/pool lists with `showDeleted=true` found no matching tombstones. Both API
  names are absent. These are point-in-time checks, **not reservations** or proof
  that every deleted-SA/global name collision is impossible; recheck before apply,
  and fail rather than adopt/undelete any conflict.
- Proposed maximum initial customer worker shape: one `n2-standard-4`, 100 GB
  `pd-balanced`, matching the validated primary shape, plus one hosted control
  plane and its required network/LB/NAT/storage resources. **No dollar ceiling,
  retention deadline, quota clearance or new-cost approval has been established.**
  Do not extrapolate the earlier cost-sizing waiver for the retained cluster into
  approval for this additional installation. No optional PVC/LB canaries are
  included unless their extra resources/probes are separately approved.

Use the same executable IAM content (schemaVersion/IAM) at an approved source
boundary and the existing target boundary. Names/UIDs differ while canonical
digest can stay identical. CLI `update_preview.go:188` checks the complete
reference, so different boundaries can require `newApplication=true` with zero
role additions. A new installation still creates its own **14 IAM resources and
15 grants**; only its later same-permissions upgrade has zero role additions.
Never reuse the retained installation's roles or weaken its journal lineage.

## Required decision before further execution

1. **Keep 4.23:** remain blocked until a real supported 4.23 → 5.0.0 path is
   available and the catalog/compatibility gaps are separately reviewed.
2. **Change source to 4.22.21:** obtain explicit user agreement first, then do
   code-first bridge/catalog/template and GCP payload-compatibility preparation,
   with architect review. Do not allocate a cluster merely because a graph edge
   exists. Exact source/target must still be advertised by the actual HC at the
   eventual upgrade decision.
3. After that, obtain an exact live footprint/cost/time approval and a safe
   shared-eligibility/upgrade-release sequence, plus separately scoped identity
   probes and eventual owned teardown approval.

There was no code change, image build/push, catalog publication, gate toggle,
IAM/role/policy update, assertion append, upgrade, resource allocation, journal
mutation, guard rerun, migration replay, Terraform operation or teardown in
this read-only checkpoint. Original source pins and local CLI commits remain.
