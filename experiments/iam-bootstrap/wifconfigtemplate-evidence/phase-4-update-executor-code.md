# Phase 4 linked IAM update executor — code-first checkpoint, no live writes

**Recorded 2026-11-19T18:05:27Z. Phase 4 remains incomplete.** The user
approved code-first implementation. This checkpoint does not authorize customer
IAM mutation, application assertions, gate activation, an upgrade, a disposable
cloud test allocation or teardown of the retained stack.

## Source and artifact authority

- CLI `/Users/asegundo/git/osd/gcp-hcp-ctl`, branch
  `fix/wif-federation-create-parents`, clean local commit
  `a727640d70f9c1af506f9e922c729edc8f05d4c6`, five commits ahead of the
  selected remote. **Not pushed or installed globally.**
- Gecko `/Users/asegundo/git/osd/gecko`, branch
  `fix/GCP-1452-cel-integer-decoding`, local test-only commit
  `367f546f5a1d355bc651ca484088419495355c60` over published/deployed
  `d70d649...`. No Gecko runtime or generated API change was needed.
  This tests the deployed implementation's private Version generation bump
  for a payload spec edit and public JSONPatch UID/resource-version guards.
  The tests are in-memory API integration proof, **not** live admission proof.
- External workfile
  `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-p3.go.work`
  maps CLI and local Gecko `platform-api`/`orlop`. Published module versions
  are still too old for standalone/merge/release readiness. No machine-local
  replacement was committed.
- Two native Go 1.27.4 darwin/arm64 builds, CGO disabled, from these clean
  source commits matched SHA-256
  `bba7d96f0fef9a79581d7a49b7354114b0e0f5dcdb842e063b6541acbbbeb13e`.
  Isolated artifacts: `.../opencode/wif-p4-cli-a727640` and `-repro`.
  Build used `go build -mod=readonly -trimpath -buildvcs=true`, fixed
  version/commit/stamp ldflags (`2026-11-19T18:04:57Z`). No server image
  was built or published; unchanged deployed source is still `d70d649...`.

## Implemented behavior

- `iam update --dry-run` remains read-only. Confirmation requires explicit
  `--confirm --expect-plan`, exact `--version`, `--channel-group` and original
  `--installation-state`. Source tests exercise confirmation and resume
  through fake public API and cloud interfaces; **the new apply command was
  not run against real resources**.
- Original ownership journal stays byte-for-byte unchanged. State lives at
  fixed `<installation-state>.updates.json`, under the same local installation
  lock used by update-aware destroy. Initial publication uses a fully
  written/fsynced private temp inode plus exclusive hard link and directory
  fsync; no empty authoritative-sidecar crash window. Subsequent saves are
  atomic. Legacy create/resume refuses a primary with linked state.
- Each operation pins the original canonical journal digest, Cluster UID,
  exact target Version UID/generation/group/template reference, all retained
  contracts, Cluster/NodePool specs and identities, before-etags and after
  role readback. Snapshot resource versions are still re-read but no longer
  enter the stable confirmation fingerprint: ordinary status refresh does
  not invalidate reviewed unchanged specs/contracts.
- Version payload pullspecs are intentionally **private**. The code preserves
  that API boundary. `payloadCatalogBinding` hashes target Version UID and
  generation, which changes on a private spec/payload edit; it is an indirect
  catalog binding, **not** a registry digest or independent image attestation.
- Only a role-only writer is invoked. It independently GETs and checks owner,
  stage, exact etag and non-pruning permission set, then patches **only**
  `includedPermissions`. It cannot create resources, change WIF trust/bindings,
  alter the worker SA or request a release. Permissions and writer authority
  are checked before writing. Fresh Complete generation/UID-bound views,
  full related NodePool inventory, target eligibility and retained template
  identities/content are rechecked before role writes/assertion submission.
- Intent precedes every role RPC. On lost response, resume accepts exactly
  the before state with the original etag or the authorized after union;
  unknown/third-state drift or 404 blocks rather than recreates/adopts. Role
  readback is durable before success. After complete cloud verification, the
  assertion uses JSONPatch tests for fresh Cluster UID/resource version and
  changes only `templateApplications`. Cloud success/assertion failure stays
  incomplete; uncertain assertion outcome is independently read back before
  any retry. Same-contract targets get no duplicate application.
- `iam verify` and `iam destroy` automatically validate the linked role
  permission/etag lineage. They do not weaken the original Plan/fingerprint
  checks or accept arbitrary cloud supersets. Verification distinguishes
  matching cloud readback from a Recorded update. Normal hosted teardown and
  cloud dependency proof still precede cleanup. IAM/network teardown progress
  is saved only to the sidecar; the default worker and unrelated policies
  remain protected.
- A Prepared operation with every role still Planned may be explicitly
  `--abandon-unstarted` with confirmation/resume/original fingerprint. This
  leaves an Abandoned tombstone; it cannot resume or erase prior intent.
  Attempted writes cannot be abandoned. Cross-machine journal copies are not
  protected by the local lock; role CAS fails closed, and divergent/third-state
  intent requires an explicit reviewed recovery, not force-unlock or manual
  journal edits. See CLI `docs/WIFCONFIGTEMPLATE.md` for operational rules.

## Checks and architect disposition

- Full CLI `go test -race ./...`, `go vet ./...`, changed-file gofmt and
  `git diff --check` passed. Full Gecko Platform API tests, focused API race
  tests and API lint passed. No generated file changed and no generation was
  needed for the test-only Gecko commit. Existing network/create/cleanup
  suites passed through the shared persistence/interface changes.
- Credential-free tests cover stable confirmation across periodic refresh;
  role-only HTTP field mask/etag/owner/removal checks; exact JSONPatch
  UID/revision op order and 409/412 with no transport retry; lost role and
  assertion responses; disk-save failure after a role RPC; record failure;
  unauthorized/ineligible targets; changing inventory; unknown third-state
  drift; original-journal protection and shared lock; sidecar permissions;
  two distinct sequential contracts retaining earlier additions; same-contract
  reuse; active-Cluster cleanup blocking; owned-only IAM/network teardown with
  sidecar-only progress; and abandon/new-operation tombstone preservation.
- Architect review initially blocked status-dependent confirmation, first-save
  orphan state, and old writers reopening the primary. All were corrected and
  tested. Final review: **no blockers for a local code-first commit**, no live
  writes cleared. Residual gaps include real IAM propagation/conflict behavior,
  deployed admission validation and process/filesystem crash injection between
  temp fsync and link. Source/fake-cloud tests are not those live proofs.

## Retained state and live prerequisites

- No live IAM update, assertion, gate change, upgrade, rollback or teardown was
  attempted. The protected primary journal still hashes to
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`;
  **no `primary-state.json.updates.json` was created** for the retained stack.
  Original infra/HyperShift pins and unrelated untracked material were untouched.
- Current live health/gates were **not freshly verified**: the last access
  attempt required interactive GCP reauthentication (`gcloud auth login`).
  Last validated live state on November 18 is in the requirements rollout
  checkpoint: healthy retained `wif3-first`/`workers`, both gates false,
  original migration Job UID and Spanner preserved. Initial automatic-upgrade
  exclusion ended November 19 at 00:00 UTC; review timing before activation.
- Public upstream graph GETs at **2026-11-19T18:05:27Z** still found **no
  forward or conditional target from 5.0.2** in either `candidate-5.0` or
  `candidate-5.1`. No supported upgrade edge or new immutable contract was
  selected, and no forced edge or pin rewrite is acceptable.

## Next required gate — separate approval

1. Renew operator authentication and refresh live health, both gates, pins,
   receipts and maintenance timing read-only. Run two read-only previews to
   verify stable fingerprints with real status refresh.
2. Approve an isolated/disposable cloud role/project test footprint (no broad
   grants) before checking actual GCP role etag conflicts, field-mask
   preservation, propagation and interrupted-run recovery. Verify the
   deployed API admission path's UID/RV preconditions before any assertion.
3. Obtain explicit approval for exact target/contract/permissions, staged
   gate activation and upgrade timing. Do not test a real target-aware IAM
   update or upgrade on the retained primary until these gates pass.
4. Only a real distinct-contract blocked/released completed upgrade plus
   required live recovery results and committed handoff can finish Phase 4.
