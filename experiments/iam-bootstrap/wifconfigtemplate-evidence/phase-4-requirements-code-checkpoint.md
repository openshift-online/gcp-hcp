# Phase 4 exact-requirements API — code-only checkpoint, not deployed

**Recorded 2026-11-18.** This is a code-first checkpoint. It is **not** Phase 4
completion or authorization to roll out images, replay the API migration Job,
enable either new-use gate, change customer IAM or start an upgrade.

## Reviewed source

- Gecko checkout `/Users/asegundo/git/osd/gecko`, branch
  `fix/GCP-1452-cel-integer-decoding`, two clean local commits ahead of its
  selected upstream branch: `5afac669fbc3fe98e0e80926314c47dd337063c5`
  (additive private/public API, projection and schema) followed by
  `c7d7fdb809a54f21347ab3d3313911412f2e1ac1` (HC/NodePool publishers).
  Neither commit was pushed. Gecko API and Orlop module code remains local;
  published CLI module versions are still too old for a standalone release.
- Cluster and NodePool each expose read-only public `status.iamRequirements`:
  `state`, generation/UID binding, bounded `lastRefreshTime`, last observed
  release and a potentially delivered pending release. Each includes exact
  release/image/group and template name/UID/digest; private dispatch receipts
  and Version resolution remain private. Missing/Incomplete is never "no IAM".
- A Complete view requires exact receipt UID and reference shape, observed
  release matching a receipt, explicit Progressing=False when observation exists,
  and desired/spec target represented by observed or pending work. Unknown,
  absent or progressing feedback, malformed/mismatched receipts or unreceipted
  spec changes yield Incomplete. Reconciler refresh time is **not** cloud proof
  or transport-feedback age. Callers must re-read the complete related object
  inventory and reject stale/future timestamps and mismatched generation/UIDs.
  No IAM update consumer is wired in this slice.
- Both controllers publish from private receipts in the **same status write** as
  pending intent/promotion and backfill old installations on ordinary non-stale
  reconciliation. No new dispatch authorization, cloud IAM calls or source
  catalog mutations were introduced. Public PATCH/PUT forging of the new
  controller-owned status is tested and rejected/ignored.
- CLI branch `fix/wif-federation-create-parents` also has local commit `c58b57d`
  (ahead of its remote), reviewed by the architect reviewer. Cluster/NodePool
  get/list YAML now preserves the API's camelCase JSON field names instead of
  lowercasing Go identifiers. Credential-free get tests verify that
  `iamRequirements` remains visible while private `iamDispatch` is dropped;
  full CLI race tests and vet passed with the exact local Gecko API workspace.
  This is readback only; no IAM-update consumer or writer was added.

## Validation and architect review

- `make -C platform-api generate` ran twice unchanged on the second pass.
  Generated API/public types, schemas, conversions and deep copies were
  reviewed; the API field is additive/optional. `make -C platform-api test`,
  `make -C controllers test`, both module lints and focused API/controller
  race tests passed. CLI tests with the exact local Gecko API workspace passed.
  `git diff --check` passed.
- The required architect reviewer initially blocked an absent/Unknown-progress
  fail-open; fixed it and added missing/Unknown, mid-upgrade, spec-bump, UID,
  backfill and timestamp tests. Final code-only review approved the committed
  source and said a **staged rollout is safe to propose**, not to execute without
  its operational gates. Nil transport feedback was additionally prevented
  from refreshing the read-only observation before the final checks.

## Read-only deployed baseline and rollout decision

- Read-only readback still shows retained `wif3-first` UID
  `6b152684-797c-4f17-a258-43538f8013c8` at 5.0.2, one original IAM
  application of template `5.0.0`, no requested upgrade, and template
  verified=false. The old server has no public `iamRequirements` field yet;
  that is expected until a separately reviewed deployment. Its argument is
  `--enable-iam-application-writes=false`. No dev infra override or running
  image was changed by this source-only checkpoint.
- **API image/chart first, then compatible HC and NodePool controllers.** Verify
  API health, persisted Cluster/NodePool/Version pins, Spanner continuity and
  generated public readback before enabling new controller images. Keep the
  other five controller Deployments unchanged. Expect one requirements-status
  backfill write per retained object, then bounded periodic refresh writes.
- The existing API Helm chart reuses the API image for
  `platform-api-server-migrate`, with Argo `Force=true,Replace=true`. An image
  tag bump can therefore cause a migration Job **replacement/replay** even
  though the new status field needs no new table. The personal-dev override
  uses `ApplyOutOfSyncOnly=true` and prune=false, but that alone does not
  prevent replay if the Job image changes. Review the exact render diff and
  explicit migration/storage continuity plan **before** publishing an override
  or syncing. Preserve the current Job UID
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1` until a deliberate decision.
- Preserve both new-use gates false, Spanner, the protected primary ownership
  journal, worker-email CRD-only source pin/annotation freeze, all 19 catalog
  pins and the retained one-worker stack. No new target, cloud IAM grant,
  hosted resource, upgrade or teardown was authorized by this checkpoint.

## Next action

Decide the scoped dev rollout and API migration-Job treatment explicitly; if
approved, publish exact source refs and matching image digests, stage API before
HC/NodePool controllers, and independently observe public readback and retained
guest health. Then separately implement the CLI update consumer/journal. Actual
blocked/released upgrade testing still needs an approved real Cincinnati edge,
distinct immutable contract, timing and live IAM/upgrade approval.
