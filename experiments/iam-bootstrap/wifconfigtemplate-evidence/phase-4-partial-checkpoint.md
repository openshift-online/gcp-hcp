# Phase 4 partial checkpoint — code-first, not complete

**Recorded 2026-11-18T14:14:33Z.** This entry does not authorize or claim live IAM
updates, a new template, gate activation, an upgrade, or Phase 4 completion.

The user approved execution after the separately reviewed Phase 4 plan. This
session treated that as **code-first approval only**; live changes and an actual
upgrade retain separate approval and a supported graph edge as prerequisites.

## Source progress

- CLI checkout `/Users/asegundo/git/osd/gcp-hcp-ctl`, branch
  `fix/wif-federation-create-parents`, is two local commits ahead of its remote:
  `8976dfc` adds reviewed read-only template `list`/`get` and documentation;
  `23e2f3da39153564845a16c3a68943999478cfa2` adds an unwired pure
  `RetainPermissions` planner and focused tests. Both were reviewed by the
  architect reviewer; the latter was **approved only as a pure planner**.
- The planner checks independently built/pinned plans for identical installation
  identity, grant topology, resource names, trust and non-permission fields.
  It unions custom-role permissions without deleting old requirements, rejects
  incompatible topology and a role over 512 permissions, and returns a distinct
  `RetentionPreview`, **not** an apply-compatible `Plan`. The caller still must
  prove every running/pending requirement and target eligibility. No caller
  invokes the planner yet; no IAM writes were introduced.
- Tests: full CLI `go test -race ./...`, `go vet ./...`, focused retention race
  tests and `git diff --check` passed using the external workspace mapping CLI
  to Gecko `platform-api`/`orlop` at `54709c4`. Published Gecko module versions
  still need alignment before a standalone/release CLI build. No source push,
  new build artifact for this planner, image, GitOps deployment or migration ran.

## Read-only runtime refresh and stop conditions

- At 2026-11-18T13:57:00Z, Cluster UID
  `6b152684-797c-4f17-a258-43538f8013c8` remained on 5.0.2 with one
  unchanged application of template `5.0.0`; `workers` UID
  `a8768aef-c352-47bb-9f32-3638275f1e15` remained healthy, nodeCount=1.
  No upgrade was requested. Template verified=false and the deployed API
  argument `--enable-iam-application-writes=false` remained unchanged.
- The upstream Cincinnati `candidate-5.0` and `candidate-5.1` graphs had **no
  forward or conditional edge from 5.0.2**. No real supported upgrade target
  or distinct immutable contract was selected. The existing maintenance
  exclusion ends 2026-11-19T00:00:00Z; revisit it before any future activation.
- Preserve the Phase 3 protected `primary-state.json` ownership journal,
  retained installation/IAM/network/worker, 19 catalog pins, original Spanner,
  and HyperShift worker-email CRD source pin and annotation freeze. No journal
  or customer cloud resource was changed by this code-first checkpoint.

## Required next work (Phase 4 remains incomplete)

1. Design and expose an authenticated **read-only public** exact-requirements
   view for running control-plane and NodePools and any pending dispatches.
   Current exact dispatch/Version resolution receipts are private. Never infer
   safe retention solely from the current release name or from an unverified
   template; distinguish active from historical and pending requirements.
2. Build an existing-Cluster update plan/journal/verification path bound to the
   original journal ownership and Cluster UID/resource version. Recompute the
   retained union from pinned, independently validated contracts; preserve
   unrelated policy-v3 entries and fresh etags. Existing create-only `Engine`
   and journal reject changed fingerprints and submitted Clusters by design;
   do **not** bypass these checks to call the new pure planner as an executor.
   Add cloud-success/record-failure, unknown-outcome and multi-contract cleanup
   tests before any live IAM update. Same-contract targets need no new record.
3. Obtain architect review, commit/reproducibly build changes, render/publish
   only approved dev artifacts, observe deployment, and document tests and
   storage/migration continuity. Do not mark the phase complete on source tests.
4. For the live blocked/released test, separately approve a real forward edge,
   exact release/payload, a reviewed distinct POC-only template/authority change,
   maintenance timing, gate toggles, customer IAM changes, and an upgrade window.
   Never fabricate edges, force-unlock receipts, mutate existing immutable
   contracts or start an upgrade without approval. Restore both gates false.
