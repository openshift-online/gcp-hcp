# GCP-1452 — integer decoding fix and resumed Phase 2 acceptance

**Result:** fixed in the personal-dev API; Phase 2 acceptance completed.
Final safety readback: **2026-11-17T19:33:15Z**. No Phase 3 work started.
[Tracking bug](https://redhat.atlassian.net/browse/GCP-1452) remains subject to formal
PR/merge workflow; dev validation is not a claim of upstream merge or Jira closure.

## Source, review and validation

- Gecko branch `fix/GCP-1452-cel-integer-decoding`, clean source commit
  `54709c471cdce451119eea69b69334224131ea48`, published to `apahim/gecko`.
- Before the fix, real-schema/memory-backed tests reproduced the exact float64 CEL
  error for aggregated finalizer/status updates and private/public PUT/PATCH.
  Merge-patch tests also reproduced rounding above 2^53 and at int64 bounds.
- Preserve integer literals during generic JSON decoding: Kubernetes JSON utilities
  for byte slices; a shared UseNumber/ConvertMapNumbers helper for streaming maps.
  Current and old objects use the same canonical types. Typed struct decoding is
  unchanged. No float repair, integer truncation, schema relaxation or API-field change.
- The condition projection/preservation JSON-map round trips also preserve precision.
  No storage migration, controller logic change or generated schema change.
- Architect reviewed the decoder design, production diff, test coverage and API-only
  rollout; no blockers. All full-root tests with HELM_BIN, lint/format/whitespace,
  focused Orlop/API race tests and real Cluster integer regressions passed.
- Tests prove real immutable changes are still rejected, rejected writes do not
  mutate stored networking, adjacent large integers remain distinct, defaults work,
  invalid/fractional/out-of-range integer values are rejected, number fields retain
  fractions, and streaming null/trailing-value semantics are unchanged.
- Private JSON Patch and public merge patch are covered. The public converting
  handler's existing rejection of RFC 6902 arrays is explicitly tested, not expanded.
  Generation/schema regeneration was unnecessary: no type/marker/schema changed.

## Artifact and deployment

Clean native build:

```sh
make local-image-api LOCAL_ARCH=amd64 LOCAL_IMAGE_REPOSITORY=quay.io/apahim/gecko LOCAL_IMAGE_TAG=wif-p2-cel-54709c4
```

- API image: `quay.io/apahim/gecko:wif-p2-cel-54709c4-api`.
- Published/running digest:
  `sha256:55df0ecd04b7b7eaebb6d80eb9021d431d803f71ab0cfb1be18fc3ef2b0daa35`.
- Binary provenance: source `54709c4...`, `vcs.modified=false`, Go 1.27.4,
  linux/amd64, CGO disabled, readonly modules/trimpath. Non-root image, runtime help
  smoke and anonymous registry 200/digest checks passed. Explicit Podman push used.
- Only the personal-dev API Application chart SHA/image changed. Seven compatible
  controllers remain on source `9f7161a0f333305bad789e7283196a621ab88e14`, image
  `quay.io/apahim/gecko:wif-p2-9f7161a-controllers`, digest
  `sha256:0c69927dc6976887987e20e6c7c1ce2c123fef8e3a4ed9fb6ce6ed55ff48b355`.
- Waited for API rollout and actual old pod termination before fixture creation.
  No quiesce was required: no schema/wire incompatibility and zero customer objects.
- Infra commits, all pushed to `origin/dev-apahim`:
  - `0fbb44404d9b7ccbaef18db76681313cf705785a`: fixed API deployment.
  - `7d983fa1715a730bc0e143250baf5e6d6aa28787`: temporary template=true acceptance.
  - `327ebd9f10a761683d5b3fb5b4526bc4a7033bd3`: final template=false restoration.
- ESP 2.57.1, `prune:false`, `ApplyOutOfSyncOnly=true` and assertion writes=false
  retained. No shared Gecko ref, other owner/environment or Terraform changed.
- The image change replaced the chart's expected migration Job; final UID
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, Complete 1/1. No migration-source change or
  manual replay. Database UID `99e4163c-79f9-4875-92b9-0e576b9a5927` unchanged.

## Replacement live fixture and actual gate proof

The user explicitly approved the fix/redeploy/resumed acceptance. One replacement
API-only fixture was created, no concurrent/additional fixture:

- `apahim-dev-test-1/wif-p2-cel-20261117`.
- UID `90abaf1f-d4e8-40c0-abe6-09f414ed4933`, generation **1** throughout.
- Created 19:28:37.814682Z; ordinary deletion/GET 404 verified 19:32:11.807116Z.
- Explicit `clusterNetwork[].hostPrefix: 23`, stable `5.0.0`, IAM omitted. References
  described no newly allocated cloud infrastructure; no legacy CLI IAM command ran.

**Passed:**

1. HC finalizer and placement/controller status persisted with hostPrefix 23; the
   original CEL failure did not recur. Public metadata merge PATCH returned 200,
   retained hostPrefix and did not bump generation.
2. With template=false, IAMPermissionsReady=False / VersionUnavailable was recorded.
3. GitOps template=true recovered all 19 matching real Versions within the asserted
   60-second bound. The replacement run did not retain an exact true-transition
   elapsed value; the initial attempt separately measured 25.333 seconds.
4. Without a Cluster spec/generation edit, the fixture reached public/private
   IAMPermissionsReady=False / **WIFConfigUpdateRequired**, with required template
   name and full digest in the message. No IAM application or dispatch receipt,
   HostedClusterResult or successful hosted status was recorded.
5. While the Version was available and the fixture actively blocked, read-only
   Firestore structured queries found zero matching Apply/Read/Delete desires in
   both `specs` and `status`. Independent management HC/HCP/NodePool lists were empty.
6. GitOps template=false produced current false readiness on all matching Versions
   in **34.355 seconds** from the first false observation; the same-generation
   fixture again reported VersionUnavailable. Unmatched 5.1 releases stayed visible
   and unpinned throughout; EUS default correctly stayed unavailable for membership.
7. Normal public DELETE while blocked completed with the controller finalizer
   present. Client name/namespace/UID/generation guards matched. The controller
   removed its own finalizer; no forced removal or foreign cleanup. Public GET 404.
8. One controlled versionsync restart (ordinary exact Pod deletion, no GitOps drift)
   retained all pins and current false readiness, with no startup errors:
   old UID `90f5d9cb-2226-4767-a78e-72dab577c44e`, new UID
   `a05a39ab-0264-4c29-a047-7d473cf7a7ad`; verified at 19:32:48Z.

An initial observation command mistakenly used the ambiguous short resource name
`cluster` and received a Kubernetes namespace-not-found error. The fully qualified
`clusters.gcp.managed.openshift.io` watch/read succeeded; no namespace was created,
permission expanded, fixture retried or acceptance condition waived.

## Final safety state and limits

- All eight Deployments 1/1 Ready and their Applications Synced/Healthy; no temporary
  replica overrides. NodePool-upgrade remains disabled; Cloud Workflows not enabled.
- Retained template UID `fb623828-d923-4540-82f1-df5c01503823`, digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`,
  **verified=false**, generation 7/resourceVersion 8.
- 21 Versions: 19 unchanged exact pins/current Unverified false readiness; two 5.1
  prereleases unpinned/current NotFound false readiness.
- Assertion-write gate false. Regional Clusters/NodePools 0/0; management HC/HCP/
  NodePools 0/0. No fixture transport residue in all six collections. No customer
  IAM, SA/role, hosted resource or NodePool allocation occurred.
- Authenticated public template/Version/Channel reads 200; anonymous template read
  401. Database identity unchanged; no Phase 1 Terraform operation replayed.
- Positive IAM application/upgrade/NodePool cases remain local integration tests,
  not cloud attestations. No live PostgreSQL, Docker-engine or arm64 test.
- Float-formatted integer literals such as 23.0 are not coerced; existing CEL behavior
  is unchanged. Unregistered-type storage watch fallback decoding remains a separate
  follow-up; no migration or broader broadcaster redesign was included.
- Root tests/race/lint logs and scoped snapshots remain local under the approved
  temporary directory as `gcp-1452-*`. No credentials or raw state were committed.
