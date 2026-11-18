# Phase 4 IAM update preview — partial code-first checkpoint

**Recorded 2026-11-18T16:01:50Z. Phase 4 remains incomplete.** This checkpoint
adds a read-only CLI preview, **not** a role update, application assertion,
customer IAM change, new target contract, upgrade or cleanup path.

## Source and validation

- CLI checkout `/Users/asegundo/git/osd/gcp-hcp-ctl`, branch
  `fix/wif-federation-create-parents`, clean local commit
  `4e7e4194d1010df4705a027ac699671ea791dc19`, four commits ahead of its
  selected remote branch. Not pushed or installed as a global CLI. The exact
  external workspace binds this CLI to local Gecko API/Orlop source
  `d70d649978bd4372b3ee1201d88b0538e1d59e87`; published Gecko modules
  remain too old for standalone release readiness.
- `gcphcpctl iam update <cluster-name> --version <exact-target>
  --channel-group <group> --installation-state <protected-original-journal>
  --dry-run` requires every flag. There is **no** `--confirm`, `--resume` or
  other write flag. The original journal is loaded read-only. The code takes
  only the `CloudReader` interface and public authenticated API Get/List
  methods; it has no cloud or API write method.
- It validates the confirmed original installation/realm/Cluster UID and
  worker SA, complete unpaginated NodePool inventory, fresh Complete
  generation/UID-bound requirements, exact recorded history coverage of every
  dispatched or pending receipt, every retained template UID/content digest
  (including currently unverified history), safe scope-preserving role unions,
  independent owned cloud IAM readback and custom-role permission support.
  It re-reads both resource inventory and target eligibility before returning
  a fingerprint. No old permission is removed; incompatible identity, trust,
  binding topology or a >512-permission role is rejected. The preview is **not
  an apply-ready Plan or cloud attestation**.
- Full CLI `go test -race ./...`, `go vet ./...`, focused read-only and fail-closed
  tests and `git diff --check` passed. The required architect reviewer
  approved a **read-only** commit and live **read-only** smoke; no write path
  was approved. Two clean native Go 1.27.4 darwin/arm64 builds with CGO off,
  fixed stamp and `-mod=readonly -trimpath -buildvcs=true` matched SHA-256
  `c390c7662d550abc6a438d6d08e1e3f88914fbe840d03b00454e29e68bd1e54a`.
  Isolated binary: `.../opencode/wif-p4-cli-4e7e419`.

## Live read-only acceptance and retained state

- The isolated artifact was run only with `--dry-run` against existing
  `apahim-dev-test-1/wif3-first`, exact 5.0.2/candidate and the protected
  primary journal. The first attempt **failed closed** with `Cluster or
  NodePool inventory changed during IAM preview` concurrent with a
  requirements/status refresh around 15:59:46Z; the exact writer was not
  established. No mutation occurred. A fresh read-only attempt
  passed with Cluster UID `6b152684-797c-4f17-a258-43538f8013c8`,
  Cluster RV `61`, NodePool-list RV `18`, one `workers` UID
  `a8768aef-c352-47bb-9f32-3638275f1e15`, original template `5.0.0`
  UID `fb623828-d923-4540-82f1-df5c01503823` and digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
  It returned **zero role additions**, `newApplication=false`,
  `targetEligibleForNewUse=false`,
  `targetBlock=WIFConfigTemplateUnverified`, fingerprint
  `sha256:23a282eec85d4b227891a74cd581dfe875cbcbbc9e48b5acc519fdb6fb810f83`.
  A zero exit from this inspectable blocked preview **does not authorize**
  cloud writes or an upgrade.
- Independent post-command readback still had exactly one Cluster IAM
  application and the same Dispatched 5.0.2 receipt, no Pending receipt;
  public requirements remained Complete. Deployed API application-write arg
  stayed false and the catalog template stayed verified=false. Protected
  primary journal SHA-256 stayed
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`.
  No source/image/GitOps rollout, Terraform apply, cloud IAM update or hosted
  upgrade was performed for this preview slice.

## Required next code slice — **not implemented**

Before exposing any apply flag, add a **separate linked update journal** bound
to the original journal/owner marker, Cluster UID, exact target and all retained
requirements, with intent-before-RPC and unknown-result recovery. Implement
fresh-etag, owner-checked additive custom-role updates without changing
identity/trust/grants; verify the entire retained union and then append the
exact application assertion under Cluster resource-version/UID guards.
Cloud-success/record-failure must remain incomplete. Same-contract targets
need no record. Crucially, update `iam verify` and owned teardown to understand
linked multi-contract roles **before** enabling writes; the original create-only
engine correctly rejects changed permissions/role etags and must not be
weakened. Unit/race/fault-injection and cleanup tests, architect review,
clean commits/reproducible builds and a separate live approval are required.
The read-only view's refresh time is not independent transport-feedback age;
the all-history permission union avoids pruning even if feedback is stale,
but any future consumer must continue to fail closed on missing/stale views.
