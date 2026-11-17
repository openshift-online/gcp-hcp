# Phase 2 initial blocked handoff — historical

**Superseded by the [completed Phase 2 handoff](phase-2-completion-handoff.md).**
GCP-1452 was subsequently fixed and the user-approved replacement acceptance passed.
Keep the following as the record of the initial stop, not the current resume action.

**Stop point:** 2026-11-17, final safety readback at 18:55:39Z.

Reviewed Phase 2 source and clean native images are deployed to personal dev, but
the required live missing-IAM gate test could not run: a persisted Cluster update
fails numeric CEL validation before any finalizer/status can be recorded. Do not
advance to Phase 3 or interpret safe non-allocation as a passed IAM gate.

## Authoritative state

- Gecko `/Users/asegundo/git/osd/gecko`, clean branch
  `feat/wifconfigtemplate-phase2`, commit
  `9f7161a0f333305bad789e7283196a621ab88e14`, pushed to `origin` (`apahim/gecko`).
- Infra `/Users/asegundo/git/osd/gcp-hcp-infra-dev-main`, clean branch
  `chore/dev-apahim-main-overlay`, published `dev-apahim` tip
  `eaf6c885f5e31c223bd9b3257d210fb591ed6cd7`.
- Eight Deployments Ready and Applications Synced/Healthy; no temporary replica
  override. All chart refs use full Gecko SHA. API image
  `quay.io/apahim/gecko:wif-p2-9f7161a-api` / digest
  `sha256:4354295256376556b7ef74cbc52325f021983efd3a297e9386b4ca08534f6422`.
  Controllers image `quay.io/apahim/gecko:wif-p2-9f7161a-controllers` / digest
  `sha256:0c69927dc6976887987e20e6c7c1ce2c123fef8e3a4ed9fb6ce6ed55ff48b355`.
- All binaries were natively cross-compiled from the clean reviewed commit, with
  `vcs.modified=false`; both published images anonymously readable. Build command,
  tests and seven rollout/toggle commits are in [Phase 2 evidence](phase-2.md).
- Template `5.0.0`, unchanged UID `fb623828-d923-4540-82f1-df5c01503823`, digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`,
  **verified=false**, generation 5, resourceVersion 6. Retain it.
- Nineteen 5.0 Versions retain exact pins/current Available=False. Two 5.1 prereleases
  remain visible/unpinned/unavailable. Live true recovery took 25.333 seconds.
- Assertion writes remain disabled. NodePool-upgrade remains disabled. Cloud
  Workflows was not enabled. CLI untouched; never use its legacy IAM path here.
- Regional Cluster/NodePool and management HC/HCP/NodePool counts all zero. The one
  API fixture was deleted normally, exact client identity checked, GET 404 confirmed.
  No matching Apply/Read/Delete transport documents in either Firestore database.
- Persistent database UID `99e4163c-79f9-4875-92b9-0e576b9a5927` unchanged. API image
  rollout replaced its expected migration Job; final UID
  `a1f86a15-f5a5-4aa7-a22a-9dde9e8b5828`, Complete 1/1. Do not replay it manually.
- Preserve the original infrastructure checkout's unrelated Spanner edits/PDF and
  task-owned Endpoints mirror. No Terraform operation was run in Phase 2.

## Blocker and exact next action

The only admitted fixture was `apahim-dev-test-1/wif-p2-blocked-20261117`, UID
`c9ea960b-6f6d-4791-9748-0933e589db39`, generation 1. Its networking included
`clusterNetwork: [{cidr: 10.132.0.0/14, hostPrefix: 23}]`. Creation succeeded, but HC
finalizer and placement/resolution status writes failed with:

```text
spec.networking.clusterNetwork: invalid data, expected int, got float64
evaluating rule: clusterNetwork is immutable and cannot be modified once set.
```

It never acquired status/finalizers/dispatch receipts. **The live IAM gate was not
reached.** No application assertion, second live fixture, validation bypass, immutable
network edit, hosted allocation or automatic broad framework fix was attempted.

Read-only architect triage identifies a latent baseline defect, **not Spanner-specific
and not introduced by Phase 2**. `orlop/pkg/apiserver/aggregated/strategy.go` `toMap`
and the public/private handler JSON decoding paths produce float64 values. They feed
`orlop/pkg/apiserver/schema/processor.go` `Processor.Process`; upstream CEL integer
conversion rejects those values when evaluating the array's `self == oldSelf` rule
over integer hostPrefix. Create skips transition rules. These framework files are
unchanged in the Phase 2 diff. A real memory-backed handler/strategy test can reproduce
the path; controller-runtime fakes skip it. No regression or fix was written after
the stop point, and no workaround omitting hostPrefix was attempted.

Next: refresh source/live state read-only, then agree the narrow numeric-round-trip
prerequisite fix and resumed acceptance footprint. Reproduce the unchanged Cluster
finalizer/status update with schema/CEL plus an unstructured persisted old object,
not only controller-runtime fakes. Distinguish typed request numbers from JSON-backed
old-object numbers. Cover array-of-integer-field oldSelf equality in the processor,
typed strategy finalizer/status updates, and actual public/private PUT/PATCH/status
handlers. Preserve immutability rejection for genuinely changed hostPrefix; normalize
numeric representation centrally rather than removing transition validation. Require
a failing regression, architect review, passing source
checks, clean commit/native build and compatible dev rollout before resuming live
acceptance. Do not weaken immutability or remove validation to get the fixture through.

After agreement and repair, re-run the required missing-IAM condition/zero-Apply
proof, recovery without spec edits, normal blocked deletion and controlled versionsync
restart/pin persistence. A replacement fixture needs an agreed resumption; do not
silently extend the original one-object acceptance attempt. End verified=false and
assertion writes false, commit actual outcomes and only then consider Phase 2 complete.

**Recovery boundary:** pins now exist. Never resume old f90 typed controllers or
erase pins/history. Use a reviewed forward fix; quiesce compatible dispatchers if
uncertain. Pending receipts deliberately have no force-unlock escape.

## Local tools/evidence

- Use the existing GKE DNS-endpoint contexts recorded in the Phase 1 handoff.
- Every Python command uses venv
  `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-phase1-venv`
  (PyYAML 6.0.3); recreate a venv if temporary files disappeared.
- Helm 3.19.2 lives under the same approved temp directory. Full module tests used
  `HELM_BIN`; source lint, targeted API/controller race suites and two-pass generation
  passed before the clean source commit. Tests did not cover this live Spanner-backed
  numeric/CEL update path.
- Local-only logs/snapshots: `wif-phase2-{root-test,lint,controller-race,api-race}.log`,
  `wif-phase2-{live-baseline,live-true,final-state,fixture-identity,transport-absence}.json`
  under the approved temp directory. Durable facts are recorded here and in
  `phase-2.md`; do not depend on temporary files or commit credentials/raw state.
