# Phase 4 live POC — upgrade passed; owned cleanup blocked

**Continuation:** the [separately approved cleanup fix and recovery](phase-4-cleanup-recovery.md)
subsequently completed at 13:39:48 UTC with independent retained-state acceptance
at 13:41:09 UTC. The blocked cleanup/residual inventory below is historical;
follow that handoff for final state, CLI artifact and sidecar checksum.

**2026-11-20. Core live 4.22.21 → exact 5.0.0 proof PASSED. Phase 4 remains
INCOMPLETE because second-installation IAM/network cleanup is blocked.** Final
runtime/guest readback **12:39:32 UTC**: retained `wif3-first` / `workers` healthy
at 5.0.2, nine Deployments Ready, both new-use gates closed, 60 Versions/55 pins
unchanged, original Job/Spanner/CRD source and protected journals intact.

## Approval and preserved artifacts

The user approved one bounded live window covering retained protection, one fresh
same-project installation, missing-target coverage proof, IAM verification/assertion,
control-plane then worker upgrade, gate restoration and owned secondary cleanup.
No new code fix, journal repair, forced deletion or broader cleanup was authorized.
The approved plan is `/Users/asegundo/.opencode/plan/iam-wifconfigtemplate-phase-4-live-poc.md`.

- Window began **10:58:01 UTC**, active limit **16:58:01 UTC**, cleanup deadline
  **15:58:01 UTC**. Both initial exclusions end **18:58:01 UTC**. No unattended
  teardown was scheduled. Cleanup stopped conservatively well inside the window.
- Gecko stayed clean at `6c28cf4348a8ab4f524b91ece1cdbe29f1389c44`; existing
  mixed controller/chart/image pins, including enabled worker coordinator, were
  preserved. CLI stayed clean/local at `c3daed09e64f85a6d7025a1e0ba5d9bbca4ef2a0`.
  All earlier commits remain; no application build, publication or source fix.
- Executed native CLI was the retained `.../opencode/wif-p4-cli-a727640`, SHA-256
  `bba7d96f0fef9a79581d7a49b7354114b0e0f5dcdb842e063b6541acbbbeb13e`.
- Infra `/Users/asegundo/git/osd/gcp-hcp-infra-dev-main` advanced by explicit
  fast-forward pushes to `origin/dev-apahim`, with exact remote readbacks:

  | Commit | Gate state |
  | --- | --- |
  | `6d55bde53b99728e8c9cc8bee5d25963a6124c9a` | Source 4.22.0 verified, assertion writes enabled; target 5.0.0 unverified |
  | `b69ee7dbbb58b2886876731d9a981848c13b5aaa` | Target verification enabled after source acceptance and protection readback |
  | `d6e24053836c3f92ca0cdd9a89ee212ac12102cf` | Both templates unverified and assertion writes false; final state |

Each change touched only personal-dev API override/render. One combined architect
review covered all planned stages and guarded public maintenance/release PATCHes.
Actual root Helm values showed all other 22 child specs equal; API child renders
changed only template verification and the assertion-write argument. All gate
states preserved the migration Job byte-for-byte, Spanner, ESP, image/source refs,
`ApplyOutOfSyncOnly=true` and API pruning safeguards. Only regional root cache
refreshes were requested; normal Argo reconciliation applied the changes. No
manual sync/Force/Replace/Job replay. API-only gate rollouts used the same digest.
Existing tests, builds and guard proof were reused; full suites and guard
harnesses were deliberately not repeated. Narrow renders and whitespace checks
passed. No source code changed, so no new source-test/build claim is made.

## Protection and fresh source installation

- Fresh baseline confirmed nine Ready Deployments, both gates false, real retained
  guest 5.0.2 Completed/Available and original Ready worker. Cloud absence checks
  included soft-deleted role/pool names; no `wif4-iso1` collision was found.
- Public JSONPatch tested retained Cluster UID, fresh resourceVersion and existing
  exclusions, then appended `wif4-poc-protection`, **10:58:01–18:58:01 UTC**.
  Retained generation became **3**; this approved maintenance change is its only
  spec difference. No release, assertion or worker change. Operational single-writer
  coordination, not a new hard authorization hold, protected customer-write paths.
  Maintenance exclusions do not block customer requests or applicable overrides.
- New Cluster `apahim-dev-test-1/wif4-isolation`, infra **`wif4-iso1`**, used
  exact **4.22.21/candidate** and only the `4.22.0` assertion. Its initial exclusion
  also ended 18:58:01 UTC (CLI starts initial exclusions at UTC midnight).
- Reviewed setup plan fingerprint
  `sha256:ac573d1f01ccf971b8052932fec56e1729fb89c0e8752688c025e5ee57e16563`.
  Confirmed create completed in **209.5s**, exit 0, Cluster accepted **11:07:01 UTC**.
  Installation operation **`f926f9a9c67874e2592fda1fe3198b7f`**, Cluster UID
  **`8745559d-c7ee-4a30-9c2b-2c3424bcbad0`**. All **33** IAM/network steps were
  Verified and IAM readback at **11:06:59 UTC** preceded the POST.
- Independent cloud reads subsequently confirmed six SAs, six custom roles,
  pool/provider, all 15 scoped grants, exact issuer/audience/mapping and worker-SA
  actAs. Existing project/SA bindings and audit configuration were preserved;
  retained roles' identities/etags stayed unchanged. No worker-SA project grant,
  broad-role fallback, key, trust repointing or resource adoption.
- Public NodePool **`wif4-workers`**, UID
  **`999fddf5-e123-4986-8bae-320bb34763ee`**, had explicit nodeCount=1,
  n2-standard-4, 100 GB pd-balanced, northamerica-northeast2-a, owned subnet
  `wif4-iso1-subnet`. No autoscaling or additional steady-state pool.
- Guest CVO completed **4.22.21 at 11:24:32 UTC**. Source acceptance at 11:29:25
  independently confirmed Available=True/Failing=False/Progressing=False, healthy
  HC/HCP/NodePool and current source Dispatched receipts. Source worker UID
  `c34ab4b5-0373-481f-87b7-55aec326fe1a`, VM `wif4-workers-l7rb9-68g5h`,
  Kubernetes **v1.35.9**, **RHCOS 9.8.20261026-0**, exact default worker SA
  `35809355916-compute@developer.gserviceaccount.com`.
  Real public TLS/OIDC login passed. No CPO credentials, Secret payloads, direct
  CPO image override or OS-stream override were needed.

## Missing assertion block, zero-delta IAM update and actual upgrades

The selected payloads/immutable references are unchanged from the
[catalog handoff](phase-4-42221-unverified-catalog-pinning.md). Actual guest CVO
and stored HC feedback both advertised exact **5.0.0** from the fresh source.
Target Version Available=True was independently confirmed before the request.

1. A guarded public release PATCH requested **5.0.0**, HTTP 200, new generation 3.
   From **11:30:23 UTC**, conditions explicitly showed
   `ControlPlaneUpgradeAccepted=False/WIFConfigUpdateRequired`,
   Progressing=False and Degraded=False. Source HC payload, guest 4.22.21,
   worker and source Dispatched receipt remained unchanged; no target Pending
   receipt or accepted target dispatch. The block persisted over five minutes
   (15-second controller requeue), with readback through 11:36:02 UTC.
   Stable RV was expected for idempotent unchanged status; an initial observer's
   unnecessary advancing-RV requirement was corrected, not counted as a server
   failure or an invitation to rerun guards.
2. **Sequencing limitation discovered:** pending release intent makes control-plane
   IAM requirements `Incomplete`; the supported IAM-update preview correctly
   refused it. A guarded PATCH withdrew only that **unaccepted** request back to
   the already-running 4.22.21. No running downgrade or cloud rollout occurred.
   After requirements recovered Complete, target preview succeeded. This proves
   block then apply/resubmit; it does **not** prove assertion-only recovery of
   an unchanged pending desired release. Do not conceal this workflow limitation.
3. Target update fingerprint
   `sha256:372a7830e10bae7a1c47f00fc9c5c97861c7680cf616b619a82c4e6ff31d93ed`:
   zero role additions, one new application, target eligible. Supported confirmed
   update succeeded at **11:39:21 UTC**, operation
   **`1f596b003b677d6beed8ab790b8e91f8`**, sidecar phase **Recorded**.
   Cloud readback and all six unchanged role etags independently proved zero
   permission writes. History appended exactly the distinct `5.0.0` name/UID
   reference despite identical executable digest. Primary journal stayed unchanged.
4. Resubmitted exact **5.0.0**, guarded UID/RV PATCH. Gecko accepted the customer
   upgrade at **11:39:38 UTC**; guest CVO applied the exact target digest and
   completed at **11:55:42 UTC**. Gecko recorded completedTime **11:59:44 UTC**;
   HCP, stored feedback and guest agreed, healthy, before the worker request.
5. Guarded public worker request followed at **12:00:22 UTC**. Enabled coordinator
   submitted it normally; no management NodePool patch or skew/force override.
   Normal maxSurge observed two workers temporarily, then one final Ready worker.
   NodePool **5.0.0**, `NodePoolUpgradeProgressing=False/UpgradeCompleted`,
   healthy at **12:08:02 UTC**, independently accepted **12:08:52 UTC**.
   Final worker UID **`faa288b7-0de7-45ee-92fa-cefd65ef77b5`**, VM
   **`wif4-workers-v972z-52m58`**, Kubernetes **v1.36.5**, **RHCOS 10.2.20261027-0**,
   same exact default worker SA. Control plane remained 5.0.0 Completed/Available.

This establishes exact-contract gating, zero-permission-delta assertion semantics,
real control-plane/worker upgrade and same-project coexistence. It does not claim
hard GCP resource authorization isolation, universal functional coverage, new live
permission additions or production monitoring/crash-recovery acceptance.

## Gate restoration, normal hosted deletion and cleanup defect

Both gates were independently restored closed at **12:11:50 UTC**, source/image
pins unchanged and original migration Job preserved. Normal NodePool deletion at
12:12 stalled draining the last worker on the guest `migrator` PDB. Normal
whole-Cluster deletion at 12:27 resolved that dependency through the ordinary
controllers; **no PDB edit, force drain or finalizer removal**. By **12:35:45 UTC**
the second public Cluster/NodePool, management HC/HCP/NodePool/Machines and worker
VMs were absent. The standalone NodePool-delete stall remains a recorded issue,
not a claim of successful independent last-pool deletion.

Owned `iam destroy --include-owned-network --dry-run` returned eligible. The
confirmed command stopped at **12:37:58 UTC**, exit 1, `cloud resource not found`.
No blind retry, manual resource deletion or journal repair followed.

**Diagnosed lifecycle seam:** pre-5.0 CLI creates and journals bootstrap firewall
`wif4-iso1-internal-cluster`; 5.0 CPO adopts its managed-by marker and removes it
during normal teardown. `pkg/infra/network/cleanup.go` refuses a definitive missing
Firewall while its step is still Verified. Network preflight fails before any
delete RPC; IAM cleanup comes afterwards and was not reached. Architect read-only
follow-up confirmed the diagnosis. All **33 cleanup steps remain Verified**,
sidecar cleanup phase **Cleaning**; no resource is Deleting/Deleted in that state.

### Remaining resources — not cleaned up

| Resource | Exact retained identity |
| --- | --- |
| VPC `wif4-iso1-network` | ID `783207505884714870` |
| Subnet `wif4-iso1-subnet` | ID `4980866386443999049` |
| Router `wif4-iso1-router`, including NAT | ID `62623804499531613` |
| Missing bootstrap firewall | Previous ID `973338552663119706`; absence blocks tool |
| IAM | Six `wif4-iso1-*` component SAs, six `hcp_776966342d69736f31_*` custom roles, WIF pool/provider, 15 scoped grants still present |

The three network identities and operation ownership descriptions match the
journal. No secondary VMs or hosted control plane remain. **Router/NAT may still
incur cost; IAM grants/trust remain an exposure until owned cleanup finishes.**
Retire these installation IDs; do not reuse them, recreate the firewall or use
shared Terraform destroy.

## Final preservation and resume authority

Final cloud proof **12:38:43 UTC** confirmed all secondary IAM resources still
present, baseline project/worker policy members/audit configs preserved and full
retained role definitions/etags unchanged. Final runtime/guest proof **12:39:32 UTC**
confirmed original retained guest 5.0.2 Completed/Available, original worker UID
`79b85ccd-768f-44f9-bc5b-f40a2bdd2cab` Ready, only retained Cluster/NodePool present,
both original receipts/Complete requirements and no requested upgrade. Only the
approved maintenance append differs from its original spec. Nine Deployments
Ready; all 60 catalog Version UIDs/full specs and 55 pins preserved. Job/Spanner
UID/spec/health and HyperShift CRD source/stamp intact; API still uses Spanner.
Retained `iam verify` returned cloudReadbackMatches=true, NoUpdates, eligible=false.

Private local evidence/state, mode-0700 directory and mode-0600 files:
`/Users/asegundo/.local/state/gcp-hcp/wif-phase4-live-20261120`.
No credentials or journals are committed. Preserve:

- `installation-state.json`, original operation above, raw SHA-256
  `5ab17ebb16f8f4d1c5e45348dbb712078c5391026729421de56122148198e7ac`.
- `installation-state.json.updates.json`, Recorded update / Cleaning cleanup,
  raw SHA-256 `31bb4bfcb5cfba8e6d6be37fd49b73d378faca9052d4c1d5b3dea3b653069b23`.
- `live-upgrade-acceptance.json`, `missing-target-assertion-block.json`,
  `target-update-*`, `normal-hosted-teardown-complete.json`, `owned-cleanup-*`,
  `cleanup-failure-network-readback.json`, `final-cloud-residual-readback.json`
  and `final-runtime-readback.json`. These local files are not backed up by Git.
- All three pre-existing protected journal hashes match their recorded baselines;
  no retained primary update sidecar. Neither guard harness was rerun.

**Next decision:** approve only a narrow CLI cleanup compatibility fix and owned
resume, not another installation or upgrade. After independent hosted teardown,
allow an exact-name absent **Verified bootstrap Firewall with journaled UID** to
be recorded as controller-removed through normal sidecar persistence. Keep missing
Network/Subnet/Router, present mismatched firewall identity/ownership and unknown
intent fail-closed. Add focused regression tests, architect review, a new local
commit/artifact while preserving all previous commits/artifacts; then fresh
dry-run/readback and journal-driven cleanup. A present-but-CPO-modified firewall
is a distinct untested case, not silently covered. No source fix was made here.
Keep both gates closed regardless of exclusion expiry; do not extend protection,
reopen eligibility, repair journal state or bypass the cleanup tool automatically.
