# Phase 4 — bootstrap-firewall cleanup fix and owned recovery

**2026-11-20. Approved Phase 4 live POC and owned secondary cleanup COMPLETE.**
The [real 4.22.21 → exact 5.0.0 upgrade](phase-4-live-upgrade-poc.md) did not need
repeating. Its cleanup blocker is resolved: all 33 secondary cleanup steps are
Deleted and the linked journal phase is Cleaned. Independent cloud readback
**13:40:27 UTC** confirmed the recorded secondary network/IAM footprint removed
or soft-deleted and project/worker policy memberships restored to baseline.
Final retained runtime/guest readback **13:41:09 UTC** passed.

## Scope and reviewed source

The user specifically approved a narrow CLI compatibility fix, focused tests,
architect review, new local artifact and live journal-driven cleanup of only
`wif4-iso1`. No retained-resource cleanup, new installation/upgrade, forced delete,
manual journal edit, broad missing-resource acceptance or Terraform operation.

- CLI `/Users/asegundo/git/osd/gcp-hcp-ctl`, branch
  `fix/wif-federation-create-parents`, new clean local commit
  **`ba211636d48d6cedf11e6090536ef7c5f985800f`**, direct parent retained
  `c3daed09e64f85a6d7025a1e0ba5d9bbca4ef2a0`. **Not pushed**. All earlier
  local commits and native artifacts remain intact.
- Only `pkg/infra/network/cleanup.go` and `cleanup_test.go` changed. After the
  existing independent hosted-teardown/authority checks, a definitive Google
  API 404 for the exact planned bootstrap Firewall can be durably recorded
  Deleted without sending a delete RPC, only if its journal step was Verified,
  has a nonempty recorded UID and matching resource name/kind, and has no
  outstanding DeleteOperation. This works if absence is seen in either cleanup
  loop. Already-uncertain Intent/Unknown creation in that run does not qualify.
- Missing Network/Subnet/Router still refuse. Present firewall ID/owner/shape
  changes, 403, transport errors and outstanding delete operations still refuse.
  File-save/proof failures stop before other deletes. No journal-schema change.
  Absence is not an attestation of which actor deleted the rule; normal teardown
  and recorded identity bound this non-destructive recognition of absence.
- New happy regressions failed against old code with `cloud resource not found`.
  Final focused **race** tests and **vet** passed under the retained external
  `wif-p3.go.work` for `./pkg/infra/network ./pkg/infra/iam/...`. Tests cover
  first-/second-loop disappearance, linked-only persistence and unchanged primary,
  proof/save interruption and resume, strict other-resource/identity/error refusals
  and no missing-firewall delete RPC. Existing 5.0/no-firewall and external-network
  tests also passed. `gofmt` and `git diff --check` passed.
- Architect review gave go for final source/commit/build and conditional live
  recovery after fresh exact-ID/hash/health checks. Reviewer focused tests also
  passed with the explicit workspace; its first default-module invocation lacked
  unpublished API types, consistent with the pre-existing standalone dependency
  limitation. Full suites/guards were deliberately not rerun for this narrow fix.
- Known limits remain: already-present but CPO-modified firewall shape still fails
  closed; no generic adoption or repair was added. The code does not introduce
  historical provenance tracking for a prior resolved uncertain creation. The
  actual primary/sidecar hashes established this installation's Firewall was
  originally Verified with its recorded UID, not an uncertain creation.

## Reproducible native artifact

Two clean-source **Go 1.27.4 darwin/arm64, CGO_ENABLED=0** builds matched:

- Binary: `.../opencode/wif-p4-cli-ba21163`, independent `-repro` output.
- SHA-256: **`f0e36b61c500a16742197fe88cfa84d4e6f45654fc9e4b8f126a2a69f8af4886`**.
- Flags: `go build -mod=readonly -trimpath -buildvcs=true`, fixed version/commit/
  timestamp ldflags and the existing external workspace. Build metadata confirms
  exact `ba211636...`, `vcs.modified=false`; full command/stamp in private build.json.
- Original operational `wif-p4-cli-a727640` still hashes to
  `bba7d96f0fef9a79581d7a49b7354114b0e0f5dcdb842e063b6541acbbbeb13e`.
  No global CLI replacement, image build/publication or API/controller rollout.
  Gecko remains `6c28cf4`, infra remains closed-gate **`d6e24053`**.

## Live recovery and independent acceptance

Fresh checks reconfirmed retained health and both closed gates, original protected
journal hashes, secondary hosted/VM absence and exact original secondary IDs:
VPC `783207505884714870`, subnet `4980866386443999049`, router/NAT
`62623804499531613`, and definitive 404 for `wif4-iso1-internal-cluster`.
Operation ownership descriptions matched the owner derived from the journal.

Read-only previews initially hit transient API/cloud errors; direct independent
API/cloud reads and the old binary's read-only preview established readable state.
The new artifact's subsequent preview returned **cleanupEligible=true** with the
exact original resource/grant plan and operation
**`f926f9a9c67874e2592fda1fe3198b7f`**. No cleanup write occurred during those
refused previews. Immediately before confirmation, gate/guest/absence/ID/owner/
journal checks passed again. The original primary and pre-recovery sidecar raw
hashes matched the prior handoff, including all 33 original Verified cleanup steps.

The new artifact executed only:
`iam destroy wif4-iso1 --state-file <same-original-installation-state.json>
--include-owned-network --yes`, with original project/region/API/OIDC settings.
It completed in **342.0 seconds**, exit 0 at **13:39:48 UTC**, within the original
six-hour window. All progress was persisted through normal linked sidecar writes:

- Cleanup phase **Cleaned**, **33 Deleted** steps. Missing firewall has no
  DeleteOperation; its absence was recorded, not recreated or force-deleted.
- Prior target update operation remains **Recorded**. Both journal files retained.
- Original secondary `installation-state.json` raw SHA-256 stays
  `5ab17ebb16f8f4d1c5e45348dbb712078c5391026729421de56122148198e7ac`.
- Final `installation-state.json.updates.json` raw SHA-256:
  **`78c2fe1a7725e0aa0820241a5c0ac1d7c303d80c9695465e439960d89915a25c`**.

At **13:40:27 UTC**, independent direct cloud GET/policy reads found:

- VPC, subnet, router/NAT and firewall: **404**. No secondary worker VMs.
- Six component SAs: **404**; six custom roles: **deleted tombstones**;
  pool: **DELETED tombstone**; provider: **404**. Retire `wif4-iso1` permanently;
  do not adopt/undelete/reuse these names or treat tombstones as active grants.
- Project and default-worker-SA policy binding/member/condition sets and audit
  configuration exactly restored to the pre-second-installation baseline.
- All six retained custom-role full definitions/etags unchanged. Retained worker
  VM ID `8160493395057321576`, worker SA unique ID `115529340171961812045` and
  retained network numeric identity preserved. No default-SA deletion or new grant.

At **13:41:09 UTC**, independent runtime/actual guest readback found:

- Only retained `wif3-first` / `workers`; original UIDs/specs/receipts, except the
  already approved retained maintenance append (generation 3). Complete IAM views,
  no requested upgrade, original guest **5.0.2 Completed**, Available=True,
  Failing=False, Progressing=False; original single Ready worker UID
  `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`.
- **Nine Deployments Ready**, both templates unverified and assertion writes false.
  **60 Version UIDs/full specs and 55 pins unchanged**. API still selects Spanner;
  original completed migration Job UID/spec and Ready Spanner UID/spec preserved.
  HyperShift CRD-only source/stamp and Argo expected source/health intact.
- All three original protected journals unchanged; no retained primary update
  sidecar. New CLI retained `iam verify`: cloudReadbackMatches=true, NoUpdates,
  eligibleForNewUse=false. Only secondary linked cleanup state changed.

Private mode-0600 evidence/build metadata is in mode-0700 directory
`/Users/asegundo/.local/state/gcp-hcp/wif-phase4-live-20261120/cleanup-recovery-20261120`:
`build.json`, `preflight.json`, `cleanup-preview.json`, `cleanup-confirm-intent.json`,
`cleanup-command-result.json`, `independent-cloud-cleanup-proof.json` and
`final-runtime-proof.json`. No tokens, Secret payloads or journals are committed;
these local supporting files are not backed up by Git.

## Handoff

The **approved Phase 4 POC is complete**, including owned secondary cleanup.
Keep both gates closed; the retained automatic exclusion expires **18:58:01 UTC**,
which does not authorize reopening eligibility or deleting the retained stack.
No active recorded secondary VPC/subnet/router-NAT or IAM grant remains; soft-delete
tombstones and ownership journals are intentional provenance.

Follow-ups, not hidden live acceptance claims: pending desired release causes
Incomplete IAM requirements, so this proof used withdraw-unaccepted-intent →
verify/append → exact resubmit; unchanged-pending-intent recovery is unproven.
Standalone last-worker deletion stalled on a guest PDB until normal whole-cluster
deletion. Same-project roles/shared worker SA are not hard authorization isolation.
Production monitoring, broader failure/restart/functional matrix, module publication
and coordinator RBAC/DNS hardening remain separate work. Do not rerun the upgrade
or proceed into phases 5/6 without a new scoped request.
