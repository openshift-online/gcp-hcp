# Phase 4 public JSONPatch compatibility — local source checkpoint

**2026-11-20. Local source/build execution; Phase 4 remains blocked pending separately
approved rollout and fresh live guard proof.** This continues checkpoint
`e5b2f24` and the [API guard blocker](phase-4-api-guard-blocker.md). The user
approved implementation of the reviewed compatibility plan, not an API rollout,
IAM update, assertion append, gate activation, upgrade, or resource allocation.

## Preserved boundaries and fresh runtime limitations

- Gecko started clean at `367f546f5a1d355bc651ca484088419495355c60`, branch
  `fix/GCP-1452-cel-integer-decoding`; its prior local test commit is retained.
  CLI remains clean at `a727640d70f9c1af506f9e922c729edc8f05d4c6`, five commits
  ahead of its selected remote. No CLI source or global binary was changed.
- Read-only refreshes at **06:40–06:41 UTC** failed on regional credentials,
  guest credentials, and public identity-token acquisition. A subsequent
  regional Cluster GET at **07:07:39 UTC** still reported reauthentication
  required. No authentication renewal or kubeconfig/context switch was made.
  **Current health, gates, receipts, migration Job and live source/image pins
  are unverified. Historical healthy observations are not current proof.**
- Preserve the retained `wif3-first` / `workers` installation and require both
  gates false. No live mutation was attempted, including no diagnostic PATCH.
  The old single-shot guard harness was neither imported nor executed.
- Primary journal SHA-256 remains
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`;
  retired guard-journal SHA-256 remains
  `af1f4abf8e822266f8a089bf03773afb8eb908083ca31f28f19e1a025d178588`.
  Both files were read-only checksummed again at **07:12:00 UTC** and have mode `0600`; no primary
  `.updates.json` exists. Neither journal was edited or locked for mutation.
- Selected infra checkout remains clean and unchanged. Local manifests retain
  API/HC/NodePool source `d70d649...`, the original migration image
  `wif-p2-cel-54709c4-api`, and HyperShift CRD-only source
  `914334ae61352fa5641e3f1d50250e87299e9eb3` with frozen annotation
  `3d8a55cb8c633329d2a64939e7e0ab20520107ee`. These are local-source checks,
  not fresh deployed-state evidence. Other five controllers, Spanner, and the
  CRD source checkout were not modified.

## Implemented public contract

The change belongs to Gecko Orlop's public `ConvertingResourceHandler`, not to
the CLI executor or the private handler:

- Parse media types, including parameters. JSONPatch arrays are applied to the
  existing **public** projection using the existing RFC 6902 dependency.
  Ordinary merge patch remains supported. Every other content type, including
  absent, unknown and malformed legacy headers, retains ordinary merge semantics.
  Strategic merge and SSA are not newly implemented. No new media-type rejection
  policy is introduced; merge error text is sanitized rather than exposing raw
  decoder errors.
- Authorize the stored object **before** reading/evaluating operations, then
  authorize the resulting object before conversion/storage.
- Preserve complete controller-owned private status, not merely its public
  projection. Restore status despite root replacement, `move`, `copy`, removal
  and multiple case variants. Keep private metadata, schema pruning/defaulting,
  CEL `oldSelf`, custom validators and generation computation.
- Reject JSONPatch changes/removal of server-managed identity/metadata,
  especially resourceVersion. Empty/zero stored revisions fail closed. Bind the
  storage precondition explicitly to the fetched snapshot, without server-side
  replay. Memory and Spanner enforce this precondition; **the existing Postgres
  Update lacks revision CAS**. Snapshot tests do not make assertion writes safe
  against concurrent overwrites on Postgres; fixing/testing that backend remains
  separately scoped. No backend was changed. Freshly verify actual deployed
  Spanner selection before any rollout/live acceptance.
- Failed JSONPatch `test`: **409**, fixed message `JSON patch test failed`.
  Decode/operation/non-object-result failures: **400**, sanitized fixed error
  categories, without raw dependency errors or compared values. Copy
  amplification is bounded to 3 MiB; negative array indices are rejected.
- No generated API/schema, chart, controller, IAM executor or CRD change.

## Credential-free evidence

The new public-handler happy-path test first reproduced the original object-
decoding HTTP 400, then passed after implementation. Regression coverage now
includes:

- Successful persisted JSONPatch and the exact UID-test/RV-test/unchanged-history
  CLI sequence with both in-memory gates closed; unchanged spec/generation,
  new storage revision and retained private dispatch receipt.
- Specific wrong-UID and stale-RV test failures, failed-test atomicity and
  unchanged complete persisted state, not generic 4xx acceptance.
- A real intervening in-memory store update after successful test evaluation:
  HTTP 409, no overwrite and only one attempted storage update.
- Stored/result authorization; protected metadata and CAS-bypass attempts;
  public/private status preservation including hidden array fields; escaped
  JSON pointer paths; exact integers beyond 2^53; unchanged no-op generation;
  legacy media-type fallbacks; malformed patches; bounded copies; captured-log
  and response non-disclosure on new patch-error paths.
- Real Cluster schema/CEL immutable-network checks now run for public JSONPatch.
  Their earlier explicit "JSONPatch unsupported" branch was replaced by success
  and actual immutable-field rejection coverage, not removed without replacement.
- Append-only IAM removal/edit rejection and a specific closed-write-gate
  rejection for an appended application.

Validation passed with Go **1.27.4 darwin/arm64**:

- `GOWORK=off make lint-fmt`, `GOWORK=off make test` (all three Gecko modules),
  `GOWORK=off make lint` (all three modules, zero issues), and
  `git diff HEAD --check`.
- Focused `go test -race` for Orlop handlers/conversion/memory storage and
  Platform API private API/authn/authz; latest handler and private API race
  runs used `-count=1`.
- Full Platform API `GOWORK=off go test -race ./...` and full retained CLI
  `go test -race ./...` using the unchanged external `wif-p3.go.work` both
  passed. The CLI checkout and artifact remain unchanged.
- An initial overlapping pair of full Orlop test invocations collided on the
  local test server's fixed port `127.0.0.1:8765`; neither process was killed.
  The independent invocation and subsequent serial full repository runs passed.
- Initial lint-format invocation with the external CLI workfile was rejected
  because `-modfile` cannot run in workspace mode. Repository-wide validation
  then used `GOWORK=off` and Gecko's existing relative module replacements;
  no module/workfile change was necessary.
- The extended IAM test initially exceeded the existing complexity threshold;
  common readback/no-op assertions were extracted and full lint passed, without
  new exclusions or weakened checks. No API generation was needed or run:
  no API type/marker/generated-schema source changed.

Architect review identified two blockers: an overbroad cross-backend CAS claim,
and a new unknown/malformed-media-type 415 policy that would break the old merge
fallback. The implementation/docs now retain that fallback and explicitly scope
CAS to memory/Spanner, recording Postgres as a pre-existing residual risk rather
than silently modifying its storage backend. Added fail-closed empty/zero stored
revision checks, combined UID/RV cases, whole-metadata and move/copy protection,
private metadata injection, authorizer-error and empty-array tests. Architect
re-review: **no blockers for local source commit/build only**. The reviewer
independently passed the handler tests, handler `go vet`, and whitespace check.
Final full `make test`, full `make lint` (zero issues), and handler `-race
-count=1` passed after these corrections, before the source commit.

Optional hardening remains: operation-count/body-size limits and safe operation
indices in errors. Pre-existing merge/PUT identity overlay and private spec-array
preservation are not repaired by this JSONPatch change. These are not live
acceptance claims or permission to widen the task.

No publication or dev rollout has occurred. None of these
offline tests proves the deployed public API or finishes Phase 4. Real Spanner
transaction/live admission behavior is not established by the in-memory race
test; database integration tests follow their existing environment skip rules.

## Reviewed source and reproducible local artifact

- Gecko source commit **`7319d8ea790a18163279ac407e4f08c676609e88`**, atop
  preserved `367f546...`. Clean checkout, two commits ahead of its selected
  remote; **not pushed**. Six changed files: public converting handler/helper,
  handler regression tests, Cluster IAM and integer API tests, and Orlop patch
  contract documentation. No controller/chart/generated/module file changed.
- Two independent host cross-compiles of the API for **linux/amd64** from that
  clean commit produced the same SHA-256:
  `c1f27be7eba0cba95405785ede80809a14c28e902c633df78344ee5a9c392f16`.
  Artifacts in the approved temp directory are
  `wif-p4-api-7319d8e-linux-amd64` and
  `wif-p4-api-7319d8e-linux-amd64-repro`.

  ```sh
  # From /Users/asegundo/git/osd/gecko/platform-api; distinct -o paths per build.
  GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=readonly -trimpath -buildvcs=true \
    -o <approved-temp-artifact-path> ./cmd/platform-api-server
  ```

  `go version -m` confirms Go 1.27.4, linux/amd64, CGO disabled,
  `vcs.revision=7319d8ea790a18163279ac407e4f08c676609e88`, and
  `vcs.modified=false`. Gecko's existing `../orlop` module replacement was
  used; this does not resolve standalone CLI module-publication readiness.
- Local packaging command from Gecko root:
  `GOWORK=off make local-image-api LOCAL_IMAGE_REPOSITORY=localhost/gecko LOCAL_IMAGE_TAG=wif-p4-jsonpatch-7319d8e`.
  The packaging input `bin/local/linux-amd64/api/app/gecko` independently hashes
  to the same value above. The existing `deploy/Containerfile.local` uses the
  existing UBI micro base/CA bundle; no image/container was executed.
- Local Podman image **`localhost/gecko:wif-p4-jsonpatch-7319d8e-api`**:
  image ID `fa42bedd483ffbecdc79648aedc6ca7a2a40d4459ed5730d8e9b852bbdee6947`;
  **local manifest** digest
  `sha256:1590cf502aed8e09f60e483edfb5b9be0f09253f5a0dad3e9296acf6f3852f22`.
  Inspection confirms linux/amd64, user `65532:65532`, entrypoint `/app/gecko`,
  and the exact source revision label. This is **not a registry-resolved digest**
  and is not evidence of registry readability, deployment or runtime behavior.
  No image was pushed and no controller artifact was rebuilt.
- CLI, dev infra, HyperShift source and CRD artifact checkouts are still clean
  and unchanged. Unrelated untracked files in `gcp-hcp` remain untouched.

## Next authorization boundary

1. Renew operator authentication separately and freshly read public/private
   objects, both gates, runtime health, dispatch/upgrade state, pins, completed
   migration Job identity/spec, and Spanner before proposing any rollout.
2. Review an API-only dev rollout using an approved registry-readable artifact.
   Preserve migration Job UID/image (no replay, Force or Replace), storage,
   controller images, both closed gates and the HyperShift source pin/stamp.
3. Only after deployment checks, separately approve a new bounded live guard
   rehearsal with a new reviewed harness/state. Its error classifier must use
   the new fixed test-failure contract, not the retired dependency-text marker.
   Do not reset/reuse the old guard journal or rerun its harness.
4. IAM update, distinct target/permission approval, maintenance timing and real
   blocked/released upgrade acceptance remain outstanding. The initial exclusion
   expired; do not infer a new maintenance window or fabricate an upgrade edge.
