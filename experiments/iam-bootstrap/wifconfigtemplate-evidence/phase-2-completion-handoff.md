# Phase 2 handoff — complete

**Completed:** 2026-11-17; final safety readback at **19:33:15Z**.
Phase 3 is unstarted and needs separate scope agreement.

Read [numeric-fix and acceptance evidence](gcp-1452-fix.md), the original
[Phase 2 implementation/rollout evidence](phase-2.md), and the progress ledger.
The earlier blocked handoff is historical, superseded by this document.

## Current source and artifacts

| Checkout | Authoritative state |
| --- | --- |
| `../gecko` | Clean branch `fix/GCP-1452-cel-integer-decoding`, API fix `54709c471cdce451119eea69b69334224131ea48`, pushed to `apahim/gecko`; based on Phase 2 `9f7161a0f333305bad789e7283196a621ab88e14` |
| `../gcp-hcp-infra-dev-main` | Clean branch `chore/dev-apahim-main-overlay`; published `origin/dev-apahim` tip `327ebd9f10a761683d5b3fb5b4526bc4a7033bd3` |
| `../gcp-hcp-ctl` | Untouched. Do not execute the legacy broad-role IAM path. |

- API `quay.io/apahim/gecko:wif-p2-cel-54709c4-api`, digest
  `sha256:55df0ecd04b7b7eaebb6d80eb9021d431d803f71ab0cfb1be18fc3ef2b0daa35`.
- Seven controllers `quay.io/apahim/gecko:wif-p2-9f7161a-controllers`, digest
  `sha256:0c69927dc6976887987e20e6c7c1ce2c123fef8e3a4ed9fb6ce6ed55ff48b355`.
- API Application chart SHA is the full fixed API commit; controller chart SHAs
  remain the full compatible Phase 2 commit. No old f90 writer survives.
- API-only fix had no schema/storage/controller change. Both artifact sets were
  reviewed, committed, natively built from clean source and explicitly pushed.

## Contracts to retain

- Versionsync owns selection/Available and preserves once-set name/UID/digest pins.
  Same-major/minor numeric floor; prereleases retain full identity and inherit their
  numeric base's IAM contract. Unverified highest boundaries never fall back.
- Independent 30-second eligibility refresh and bounded consumer retries. Fresh
  final new-dispatch checks require current availability, intact verified pin and
  exact Cluster application coverage. Missing IAM is never an exemption.
- HC/NodePool dispatchers own IAMPermissionsReady for installation targets; upgrade
  coordinators report future-target prerequisites separately.
- Private Pending/Dispatched receipts bind object/parent UID, exact release/image/
  channel group/template identity. Persist before Apply; same-target redelivery is
  permitted after revocation/aging. No force-unlock or legacy receipt exemption.
- Default Compute worker-SA identity is propagated explicitly; no project-wide actAs.
- **New IAM assertion writes remain disabled**, including at Phase 2 completion.
  Later activation needs separate agreement and real CLI cloud verification.

## Live proof and final state

- GCP-1452 no longer blocks unchanged integer-networking writes. The one replacement
  Cluster acquired its finalizer/status, accepted a public metadata PATCH and reached
  actionable public/private WIFConfigUpdateRequired while its Version was available.
  It stayed generation 1; no applications/dispatch receipt/hosted result were written.
- Template toggles changed catalog/fixture feedback without a Cluster spec edit.
  Nineteen real 5.0 releases, including prereleases, retain exact pins. Two 5.1
  prereleases remain visible/unpinned/unavailable. True recovery met the 60-second
  bound; measured final false convergence was 34.355 seconds.
- Normal blocked deletion completed with its controller finalizer. No forced removal.
  Active-fixture and final Firestore checks found no Apply/Read/Delete documents in
  either database; independent management hosted-object counts stayed zero.
- Controlled versionsync restart retained pins/current false readiness. All eight
  Deployments and Applications are Ready/Synced/Healthy; no replica overrides.
- Template `5.0.0`: UID `fb623828-d923-4540-82f1-df5c01503823`, digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`,
  **verified=false**, generation 7/resourceVersion 8. Do not delete it.
- Regional Cluster/NodePool and management HC/HCP/NodePool counts all zero. The
  replacement `wif-p2-cel-20261117` UID `90abaf1f-d4e8-40c0-abe6-09f414ed4933`
  was deleted/GET 404 at 19:32:11Z. No IAM/customer hosted allocation occurred.
- Database UID `99e4163c-79f9-4875-92b9-0e576b9a5927` unchanged. Final chart migration
  Job UID `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, Complete 1/1; do not replay it.
- ESP 2.57.1, pruning/ApplyOutOfSyncOnly protections, disabled NodePool-upgrade app
  and disabled Workflows state preserved. No Terraform operation ran.

## Next action — discussion, not execution

Refresh repository/runtime state read-only, then agree Phase 3 CLI preview, verified
application/readback and owned cleanup scope. Start by auditing the CLI's old broad
IAM path against the retained template and exact default worker-SA contract. Do not
enable assertion writes, claim real cloud coverage or allocate hosted resources
until the separately agreed Phase 3 activation prerequisites are met.

Keep the original infra checkout's unrelated Spanner edits/PDF and reviewed Endpoints
mirror intact. Use the recorded GKE DNS contexts and a Python venv for every Python
command. Native build, Helm paths and local logs are in the detailed evidence.

Jira GCP-1452 has source/dev validation evidence; formal PR merge/closure is separate.
No existing backend data was migrated. Integer precision watch-fallback follow-ups
and unchanged public JSON Patch limitations are not proof of broader runtime support.

**Recovery:** pins exist. Keep compatible API/controllers or use a reviewed forward
fix; never restore f90 typed writers, erase pins/history or switch storage to memory.
