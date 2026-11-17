# Phase 2 — implementation and initial acceptance history

**Current status: complete after the GCP-1452 fix and approved resumed acceptance.**
Read the [completion handoff](phase-2-completion-handoff.md) and
[fix/live acceptance evidence](gcp-1452-fix.md) for current source, deployment and
final state. The following records the original implementation and blocked attempt.

**Historical stop point:** Reviewed source and clean native images were deployed in personal dev.
Catalog checks passed, but Cluster update validation prevented the required live IAM
gate test. Acceptance stopped; the fixture was deleted and verified=false restored.
Initial readback: 2026-11-17T18:55:39Z. Phase 3 remained unstarted.

## Scope and baseline

- User approved the detailed Phase 2 plan and switched to execution.
- Personal dev only; no customer IAM or hosted allocation. Assertion writes stay
  disabled. At most one temporary blocked Cluster API object after all compatible
  gates are deployed; no live successful applications or NodePool object.
- Gecko started clean at `cc26254c92b64d71a0b8cf00faf955fbc8e5b925`; new branch
  `feat/wifconfigtemplate-phase2`. Dev infrastructure worktree started clean at
  `d7231a6ab273f2a2fd0899d714e824c349b2f9a8`.
- Fresh live readback passed: zero regional Clusters/NodePools, eight Deployments
  Ready, Phase 1 API tag and seven f90f6df controller tags, no Version pins, same
  database UID and same template UID with verified=false.
- Preserve original infrastructure checkout's user Spanner edits and the reviewed
  Endpoints mirror; do not replay Phase 1 Terraform. All Python uses a venv.

## Agreed implementation direction

- Versionsync selects/pins immutable contract identity even while unverified;
  readiness stays false. No reassignment or verified-boundary fallback.
- Independent 30-second eligibility refresh, separate from 15-minute upstream
  discovery; numeric-core prerelease floor selection, numeric overflow safety.
- Single current Available condition plus shared fresh pin/verification consistency
  checks before new dispatch; exact name/UID/digest application coverage.
- HC and NodePool dispatchers own their IAMPermissionsReady feedback; upgrade
  coordinators own candidate/approval feedback, not those same conditions.
- Preserve observed/committed work without new unauthorized Apply. Bind durable
  receipt intent to actual release/image/channel and exact contract, not whole
  Cluster generation. Do not permit revocation/status lag at new submission.

## Reviewed source and artifacts

- Gecko branch `feat/wifconfigtemplate-phase2`, source
  `9f7161a0f333305bad789e7283196a621ab88e14`, pushed to `apahim/gecko`.
- Native `make local-images LOCAL_ARCH=amd64
  LOCAL_IMAGE_REPOSITORY=quay.io/apahim/gecko LOCAL_IMAGE_TAG=wif-p2-9f7161a`.
  Host Go 1.27.4; both binaries report matching revision and `vcs.modified=false`.
  Both images run as 65532:65532. Explicit Podman pushes completed; anonymous
  registry manifest requests returned 200 with matching digests.
- API `quay.io/apahim/gecko:wif-p2-9f7161a-api`, digest
  `sha256:4354295256376556b7ef74cbc52325f021983efd3a297e9386b4ca08534f6422`.
- Controllers `quay.io/apahim/gecko:wif-p2-9f7161a-controllers`, digest
  `sha256:0c69927dc6976887987e20e6c7c1ce2c123fef8e3a4ed9fb6ce6ed55ff48b355`.

- Pure private API helpers/tests for numeric floor selection, verification/pin
  integrity, current Version readiness and exact application coverage.
- Versionsync reference preservation and independent availability refresh loop,
  status transitions and Channel default readiness, with an uncached reader.
- Shared catalog Available/Prerequisites helpers; discovery separated from use,
  fresh template verification checks, retryable redacted errors preserving causes.
- Test-only catalog fixtures load the single reviewed chart baseline; no production
  embedded permission catalog or implicit fallback was introduced.

Initial focused pure-helper tests passed. Initial controller tests exposed expected
fixture gaps: old mocks do not implement WIF template lists/gets or the Version
status subresource, and old positive release fixtures have no Available condition.
These were not waived. Explicit positive fixtures were updated while retaining and
adding negative tests. Source validation completed:
root `make test` with `HELM_BIN`, full `make lint`, formatting, whitespace, focused
API/controller race suites, stable two-pass generation (30 artifacts), and all seven
controller Helm lint/render/read-only-template-RBAC checks passed. API Helm rendering
also passed with explicit local-only gateway/ESP/public-API disables; ordinary default
rendering correctly requires those deployment settings. No validation image was pushed.

Recording-transport tests cover initial persist-before-Apply, optimistic conflict,
exact identity/parent UID, missing/empty/wrong application history, missing/false/
unknown/stale eligibility, replaced pin, payload mismatch, restart redelivery after
revocation/aging, and pending-only zero observation. HC Apply/observe and NodePool
observe tests prove stale feedback retains Pending and non-stale feedback promotes
both slots atomically. Cached resolutions recover after revocation without spec edits.
Positive NodePool/upgrade behavior is integration-only, not live cloud verification.

## Review and next action

Architect review closed its foundation/dispatch blockers. Findings addressed include
per-Version error isolation, serialized bounded eligibility refresh, uncached reads/
minimal RBAC, bounded consumer retries, preserving upstream failure diagnostics,
no higher-boundary fallback/rebinding, Pending-only observation exclusion and
promotion on observation. Final test-credibility findings were addressed with real
transport spies and controller promotion tests before the source commit.

Uncertain committed targets deliberately remain locked: absence of feedback cannot
prove absence of a partial BulkWriter delivery. Recovery is the same committed
target or ordinary deletion/confirmed cleanup, never a force-unlock or legacy
exemption. Read-only HyperShift CRD inspection confirmed the deployed NodePool
v1beta1 schema accepts `platform.gcp.serviceAccount.email`. Local HyperShift source
`hypershift-operator/controllers/nodepool/gcp.go` forwards email to CAPG and supplies
the existing standard scopes when scopes are omitted; no runtime/source/grant changes.

Architect review also approved the owner/region-only staged infrastructure changes.
Actual old controller pod termination, full API rollout/old pod termination, exact
image parameters, live read-only RBAC and six-consumer readiness were checked before
versionsync resumed. No shared `gecko_ref` or other owner/environment changed.

## Dev rollout commits

All commits below are in `gcp-hcp-infra-dev-main` and published to `origin/dev-apahim`.

| Commit | Change |
| --- | --- |
| `9bcec133e9306922512745f096be39049236f849` | Seven old typed controllers quiesced through GitOps; actual pods, including terminating pods, gone |
| `1aad28b7605e416fa014ee2207cf1e12f9c2857e` | Matched API/schema/public-feedback image rolled first |
| `38b64fac31b99e7b9dbc3d81d3af1a00817055ad` | Seven compatible controller chart refs/image parameters and minimal RBAC staged at zero replicas |
| `c52509d7b66d8922be350902a8a5eb9deb89862e` | Six consumers resumed and observed Ready on published digest |
| `803e2f49c9b780d1c84e4dc218de2ffa4c004d56` | Compatible versionsync resumed last; final temporary replica override removed |
| `e478c1077d84e1f696dde844bd277f18b8dc7110` | Existing retained template temporarily verified for catalog acceptance |
| `eaf6c885f5e31c223bd9b3257d210fb591ed6cd7` | Final GitOps restoration to verified=false after acceptance blocker |

Each Application is pinned to the full Gecko source SHA. Image parameters contain
exactly one tag and the explicit Quay registry/repository. All eight Applications
ended Synced/Healthy and Deployments 1/1 Ready. NodePool-upgrade stays disabled;
Cloud Workflows was not enabled. ESP 2.57.1, `prune:false` and
`ApplyOutOfSyncOnly=true` remain intact. No temporary replica overrides remain.

The API image change caused the chart's expected migration Job replacement. Old UID
`e11cbeec-b922-465b-9bc2-73d997f21b93` was replaced by
`a1f86a15-f5a5-4aa7-a22a-9dde9e8b5828` (created 16:57:44Z; Complete 1/1).
There was no migration-source change or independent manual migration replay.
The Spanner database UID remained `99e4163c-79f9-4875-92b9-0e576b9a5927`.
No Terraform operation, database reset, IAM write or customer resource allocation ran.

## Live results and blocker

**Passed catalog checks:**

- All 21 upstream Versions remained visible. Nineteen real 5.0 releases, including
  `5.0.0-ec.*` and `5.0.0-rc.*`, pinned name `5.0.0`, UID
  `fb623828-d923-4540-82f1-df5c01503823`, digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
- With verified=false, those 19 reported current-generation Available=False /
  `WIFConfigTemplateUnverified`. Two 5.1 prereleases remained visible/unpinned with
  `WIFConfigTemplateNotFound`; no cross-minor fallback.
- First observed verified=true at 18:18:26.655503Z. All 19 matching Versions became
  Available=True within **25.333 seconds** from that observation, without object spec
  edits or a controller restart to trigger recovery. Pins stayed unchanged. This is
  a measured readback bound, not an atomic catalog/transport transaction guarantee.
- Final verified=false, same UID/digest, generation 5/resourceVersion 6. All matching
  Versions again current Available=False; unmatched Versions still missing-template.
- Authenticated public template/Version/Channel reads returned 200; anonymous template
  catalog returned 401. Cross-namespace Cluster listing correctly returned 403 while
  the existing customer namespace list succeeded. EUS default remains unavailable
  because its pinned release is not in its synchronized channel, not a fallback.

**Required live IAM gate check: NOT PASSED / NOT REACHED.**

One API-only Cluster was admitted in the existing namespace `apahim-dev-test-1`:
`wif-p2-blocked-20261117`, UID `c9ea960b-6f6d-4791-9748-0933e589db39`, generation 1,
created at 18:20:18.710479Z. It requested stable `5.0.0`, explicitly omitted IAM,
and referenced no newly created cloud infrastructure. No application record or
NodePool was submitted. Two prior POSTs were rejected before creation because the
prepared fixture used `project` rather than the required JSON field `projectID`;
readback confirmed zero objects before correcting this input. These were not IAM
gate checks and did not create additional fixtures.

HC finalizer writes, placement status and Version-resolution status were rejected:

```text
[].spec.networking.clusterNetwork: Invalid value: "array": invalid data,
expected int, got float64 evaluating rule:
clusterNetwork is immutable and cannot be modified once set.
```

The admitted valid networking entry contained `hostPrefix: 23`. The error occurs on
normal persisted-object updates against the real Spanner-backed API. The fixture
stayed at generation 1 with empty status and no finalizers; **there is no live
IAMPermissionsReady or dispatcher-gate success claim**. Local typed/fake API tests
did not exercise this backend/numeric-CEL round trip. No numeric conversion or CEL
framework code was changed in the Phase 2 source; root-cause confirmation belongs
to the recorded follow-up, not an unreviewed validation bypass.

Read-only architect triage identified a **latent baseline, backend-independent**
cause: plain `encoding/json` maps contain float64 numbers, while the upstream CEL
integer converter expects int/int32/int64. The unchanged Orlop
`aggregated/strategy.go` `toMap` and public/private handler paths feed those maps
to `schema/processor.go` `Processor.Process`, which passes them to CEL without
integer normalization. `clusterNetwork`'s `self == oldSelf` evaluates its integer
`hostPrefix` field. Creation skips that transition rule; unchanged updates trigger
the conversion error. These paths predate Phase 2. A memory-backed real handler/
strategy test can reproduce it; a live Spanner instance is not required. Existing
tests covered string oldSelf rules and did not supply hostPrefix. This triage was
read-only, not a tested fix or permission to omit hostPrefix as a test workaround.

Acceptance stopped. No immutable-network patch, validation relaxation, second live
fixture, real assertion, extra cloud authority or out-of-scope framework fix was
attempted. After restoring template=false, ordinary public DELETE returned 200 and
GET returned 404 at 18:55:12.837297Z. Immediate client-side name/namespace/UID/
generation guards matched; no finalizer was removed. This was not a server-enforced
atomic UID-precondition test.

**Independent negative-footprint readback:**

- Regional Clusters/NodePools: 0/0; management HostedClusters/HostedControlPlanes/
  NodePools: 0/0.
- Exact transport group
  `projects/apahim-dev-test-1/clusters/wif-p2-blocked-20261117`: zero matching
  applydesires/readdesires/deletedesires in both management Firestore `specs` and
  `status` databases. Read-only structured queries also confirmed no group documents
  before creation. No foreign document was modified or cleaned up.
- Application-write server flag remains false; no real IAM history exists.

Controlled versionsync restart acceptance and further fixture verification-toggle
recovery were **not executed after the required check failed**. Positive application,
upgrade and NodePool cases remain integration-only. There was no live PostgreSQL,
Docker-engine or arm64 runtime test. A read-only final management schema check is not
proof of actual worker boot or cloud IAM completeness.

The full plan remains at
`/Users/asegundo/.opencode/plan/iam-wifconfigtemplate-phase-2.md`. Resume from the
[blocked Phase 2 handoff](phase-2-handoff.md), not from a completion assumption.
Phase 3 is not authorized. Do not roll back to f90 controllers after live pins exist.
