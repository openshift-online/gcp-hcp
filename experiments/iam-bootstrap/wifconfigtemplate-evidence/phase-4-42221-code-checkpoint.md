# Phase 4 4.22.21 → 5.0.0 preparation — code-only checkpoint

**2026-11-20. Code-only preparation authorized. No publication, rollout, catalog
activation, IAM change, assertion write, gate change, cluster allocation or
upgrade is authorized by this slice.** Phase 4 remains incomplete.

The user selected the exact release pair and approved implementation of the
comparison bridge, inactive dev catalog examples and offline acceptance tests.
The standalone CPO metadata/credential inspection was explicitly removed from
the critical path: use normal configured image pulls and HyperShift validation
in a later approved live test, diagnosing a concrete failure if one occurs.

## Implementation scope

- Gecko started clean at `7319d8ea790a18163279ac407e4f08c676609e88`; prior API
  JSONPatch support remains. CLI started clean at `a727640d70f9c1af506f9e922c729edc8f05d4c6`
  with its five local-only commits preserved. No history rewrite or upstream push.
- New shared `platform-api/pkg/ocpversion` provides **comparison-only** 5.x →
  4.(23+x) normalization matching pinned HyperShift. Inputs are copied; patch,
  prerelease and build metadata remain. Unsupported majors and integer overflow
  fail closed with sanitized errors.
- Worker upgrade admission/skew/step validation, observed control-plane locks,
  automatic control-plane selection, version-resolution channel discovery and
  worker superseded-target observation use the supported comparison semantics.
  Logical next-minor discovery from 4.22 with fleet 5.0 queries the actual **5.0**
  channel; explicitly 4.x fleet naming retains its existing channel naming.
- Exact graph membership, strict forward admission, sequential/EUS/skew rules,
  readiness/health and automatic maintenance requirements remain. Equal logical
  aliases cannot authorize a new upgrade; already-observed differing aliases
  conservatively retain locks and exact dispatched identities. Alias candidate
  ties choose a deterministic raw identity. Emergency critical-pin maintenance
  bypass still requires the **same raw major/minor**, not just normalized equality.
- Raw release strings, payloads, WIF floor selection, template references and
  existing Version pins are not normalized or rewritten. No API type/marker,
  generated schema, CRD, database/storage or IAM permission content change.
- `helm/examples/wif-42221/` contains **inactive, opt-in** overlays only:
  candidate catalog minimum 4.22 with install default/fleet 5.0 unchanged;
  raw `4.22.0` and `5.0.0` templates render from the identical executable content,
  both unverified, assertion writes=false. No Argo Application references them;
  shared chart defaults and templates are unchanged. Merge over reviewed live
  values; never install the examples alone or lose Spanner/migration safeguards.
- CLI changes are **tests only**, not a new writer or production behavior change.
  Same-project tests use two isolated ownership journals and a mock cloud with
  fresh-etag enforcement, shared default worker SA and project policy documents.

## Offline acceptance

New tests first reproduced rejection of the requested worker/automatic/channel
bridge and acceptance of a normalized downgrade in the old control-plane check.
After the change:

- 4.22.21 → exact 5.0.0 passes only with adequate observed control-plane version;
  workers ahead of control plane, normalized downgrades/equal-version aliases,
  skipped minors and unknown majors are rejected. Existing patches, prereleases
  and EUS step behavior are exercised across the naming boundary.
- An unadvertised raw `4.23.0` alias cannot stand in for advertised `5.0.0`.
  WIF selection still chooses `4.22.0`, `4.23.0` or `5.0.0` by the **raw**
  release numeric base, including prereleases, not by comparison normalization.
- Controller reconciliation for customer and automatic paths stays blocked on
  the distinct target assertion despite an identical executable digest. Adding
  that assertion in the local fixture allows only exact 5.0.0 approval; automatic
  maintenance exclusion remains effective. Source manifests remain on 4.22.21
  while target coverage is absent. Customer maintenance bypass is not changed.
- CLI fake-HTTP/fake-cloud integration shows zero role-permission RPCs, no cloud
  writes, exactly one distinct assertion, raw release still 4.22.21, preserved
  primary journal, linked verification and idempotent resume. An unverified
  zero-delta target still cannot prepare an update.
- Two same-project mock installations coexist and verify with different owned
  resources and changing shared policy etags. Cleanup of the second preserves
  all original resources/journal/grants, default worker SA and unrelated policy
  bindings/conditions/audit configuration. Wrong-journal adoption and wrong-owner
  cleanup fail before cloud writes. These are ownership tests, not live proof of
  hard resource authorization isolation under project-wide role bindings.
- Explicit local Helm tests prove the inactive examples add only two unverified
  templates, identical contract digests, a closed write gate, deep-equal
  existing API objects including the completed migration Job, unchanged other
  Channels, and unchanged install/fleet defaults. No values were applied.

Validation so far: full Gecko `make test` with explicit `HELM_BIN`, full Gecko
`make lint-fmt`/`make lint` (zero issues after fixing an EOF comparison in a test),
focused API/comparison/controller race tests, full CLI `go test -race ./...` and
`go vet ./...`, and whitespace checks passed. Gecko uses `GOWORK=off`; CLI uses
the existing external `wif-p3.go.work`. No machine-specific replacement was
committed; published module alignment/standalone CLI release readiness is not
claimed. No API generation needed: no source type or marker changed.

Architect review: **approved local commits/builds, no blockers**, not live
acceptance. The reviewer independently passed API/helper/controller/Helm tests,
focused CLI race tests, vet/format checks, raw graph/WIF identity and lock/hold
inspection. A mapping-source permalink was added to the example README. Full
Gecko tests (explicit Helm), lint and relevant race checks passed; both example
chart lints passed. No local test is live installation/upgrade acceptance, and
optional database integration tests follow existing skip rules.

Nonblocking review follow-ups remain documented: unknown future majors stay
fail-closed (including observation locks); contradictory raw 5.x current/4.x
fleet configurations are not a supported rollout configuration; advertised alias
ties select a deterministic raw identity; `approveUpgrade` relies on its preceding
successful target validation before parsing the same local object. None grants
authority to use unsupported majors or alter the selected raw release pair.

## Reviewed local commits and artifact build

- Gecko **`6c28cf4348a8ab4f524b91ece1cdbe29f1389c44`**, parent retained `7319d8e...`.
  Seventeen files: comparison logic/consumers, regressions and inactive examples.
  Clean source; no shared chart/default/type/generated/module change; **not pushed**.
- CLI **`c3daed09e64f85a6d7025a1e0ba5d9bbca4ef2a0`**, parent retained `a727640...`.
  Exactly one new test file; production code unchanged; all previous local-only
  commits preserved; **not pushed**. The existing operational native `a727640`
  artifact/checksum remains retained rather than replacing the user's global CLI.
- Changed API and monolithic controller binaries/images were built locally for
  **linux/amd64**, CGO disabled, Go 1.27.4, from the clean Gecko commit.
  Independent `go build -mod=readonly -trimpath -buildvcs=true` builds match the
  Makefile packaging-input binaries exactly:

  | Component | Binary SHA-256 |
  | --- | --- |
  | API | `07d2919031a2743dcdb6c77209775491567d17900d9c76c1e1323968c4d76ca0` |
  | Controllers | `625bc7b6f37b3199abe80928a497bd35e0f98fd80ce1b86e97c1ba1bf8bbff23` |

  Both binaries' build metadata reports exact source `6c28cf4...` and
  `vcs.modified=false`. Build root:
  `.../opencode/wif-p4-bridge-6c28cf4-build-_cdeqs_2/{api,controllers}/app/gecko`;
  independent outputs `.../opencode/wif-p4-bridge-6c28cf4-{api,controllers}-repro`.
  The dedicated build root avoided overwriting prior ignored artifacts.
- Packaging used existing `deploy/Containerfile.local` via
  `make local-images LOCAL_BUILD_ROOT=<dedicated-root> LOCAL_IMAGE_REPOSITORY=localhost/gecko LOCAL_IMAGE_TAG=wif-p4-bridge-6c28cf4`.
  Both images inspect as linux/amd64, user `65532:65532`, exact source label:

  | Local image | Image ID | Local manifest digest |
  | --- | --- | --- |
  | `localhost/gecko:wif-p4-bridge-6c28cf4-api` | `d30c0d77de4246481c115763ca8473271f0b2af4ccb63b6c782bc26c8c3ba5be` | `sha256:79fbdfa6172793ba34a00f8c25ebbe654845936b4f959310360e48801c081d1d` |
  | `localhost/gecko:wif-p4-bridge-6c28cf4-controllers` | `187edd1d81e915e3eca8d99d94a6ee583140c8dc308ecbf3cc5fc90a2d9b18c5` | `sha256:e3eaca8c963c39eb8263a00013d046a3e6fa6c4eed00d8464d8b8b84ff5848f8` |

  These are **local-only** identities, not registry-resolved/pullability or
  running-image proof. No image/container was executed. No registry/image/source
  publication or dev deployment occurred; no global CLI artifact was replaced.

## Fresh preserved runtime — read-only, not a deployment

At **08:37:28 UTC**, independent reads confirmed actual retained guest 5.0.2
Completed/Available=True/Failing=False/Progressing=False and original worker UID
`79b85ccd-768f-44f9-bc5b-f40a2bdd2cab` Ready. Eight API/controller Deployments
Ready; original Cluster/NodePool UID/generation/spec/receipts and no requested
upgrade, both gates false, original completed migration Job and Ready Spanner.

API is **still deployed at 7319d8e**, not the uncommitted comparison bridge.
There are still 21 Versions/19 pins. HyperShift CRD-only source
`914334ae61352fa5641e3f1d50250e87299e9eb3` and frozen stamp
`3d8a55cb8c633329d2a64939e7e0ab20520107ee` are preserved. All three protected
journal hashes remain unchanged; no primary update sidecar. Neither guard
harness was run, no new operational installation journal was created, and no IAM or
upgrade apply command was executed.

Supporting read snapshots are local-only, mode-0600 files in mode-0700
`.../opencode/wif-p4-42221-code-readback-hovnp_m5`; no Secret objects or tokens
were saved. No credential or CPO-inspection task was introduced.

## Next live boundary

After reviewed local source/artifact completion, separately propose the exact
dev-only rollout. Preserve migration image/Job identity, storage and CRD pin;
do not bump all controllers by default. Catalogue publication/pinning, verified
eligibility, assertion writes, the second installation's cost/footprint and
actual upgrade are separate live approvals. Protect `wif3-first` before shared
5.0.0 eligibility opens: its existing assertion already covers that template.
Customer requests bypass maintenance exclusions, so a missing assertion append
may release a requested upgrade immediately. No implied hold or timing approval.

The current personal-dev infra explicitly defaults `gecko-nodepool-upgrade` to
`_disabled: true`, and that coordinator is absent from the observed eight
Deployments. An eventual worker-upgrade validation needs its separately reviewed
activation/RBAC/image footprint as well as the changed API/control-plane-upgrade/
version-resolution comparison consumers. No controller was enabled here. Do not
mistake a local monolithic-controller image build for authorization to deploy
every controller or silently enable a previously disabled one.
