# GCP-1505 — WIFConfigTemplate integration inventory

This is an integration inventory, **not** a request to merge either POC branch.
It compares the published POC tips with repository mains on 2026-11-24 and
records baseline checks before the GCP-1505 implementation. POC evidence and
the intended API contract are described in the [implementation plan](../../implementation-plans/wifconfigtemplate-catalog-and-iam-lifecycle.md).

## Baselines and drift

| Repository | Current main at review | POC base → tip | Drift since POC base | Integration disposition |
| --- | --- | --- | --- | --- |
| Gecko | `4edfcf3` | `492bff6` → `6c28cf4` | 72 main commits, 10 POC commits | Reapply selected changes with tests; preserve main's ingress and release-catalog admission checks. |
| gcp-hcp-ctl | `cdf928e` | `626da6d` → `ba21163` | 30 main commits, 11 POC commits | Inventory only in GCP-1505; CLI implementation belongs to GCP-1507 and dependency alignment to GCP-1506. |

Main's Gecko #668 (`315b866`, `70b85e4`) already normalizes integer-typed
fields for CEL. POC `54709c4` complements it by preserving integer values
before they become rounded `float64` during conversion and patch processing;
the two changes must coexist. Main's release-catalog admission validation
(`ee275e4`) and ingress changes overlap the catalog API's Cluster validation
and generated types. CLI main has since changed IAM teardown (`cdd245a`,
`3993820`), platform API dependencies (`74c47b1`, `0692a44`) and direct
`PlatformRole` usage (`819b8b3`); porting its POC branch wholesale would
overwrite current interfaces and recovery logic.

## Gecko POC commit inventory

| Commit | Change | Disposition |
| --- | --- | --- |
| `6cef781` | Typed catalog, Cluster applications, Version pins, public generation, chart baseline | GCP-1505 API + Helm integration; reconcile current admission/generator output. |
| `cc26254` | Local native container images | Drop; not required by this story. |
| `9f7161a` | Version eligibility, resolution and dispatch gating | GCP-1506; do not activate new-work gates in this API-only stage. |
| `54709c4` | Integer fidelity across conversion, CEL, handlers and patches | Generic, reviewable GCP-1505 correctness change; reconcile #668. |
| `5afac66` | Public read-only IAM requirements and projection | GCP-1505 API shape/projection only; status publishers remain later work. Private receipt **types**, not dispatch behavior, are required for projection. |
| `c7d7fdb` | Controller requirements publishers | GCP-1507 (WP7). |
| `d70d649` | Separate Spanner migration Job image pin | GCP-1506 packaging (WP4); check complete rendered Job before deployment. |
| `367f546` | Assertion concurrency and catalog tests | GCP-1505 tests, backed by actual storage CAS. |
| `7319d8e` | Guarded JSONPatch on converting routes | Generic GCP-1505 correctness change; retain stored/result authorization and private state. |
| `6c28cf4` | 4.x/5.x comparison-only upgrade bridge | Later upgrade integration; not part of WP1–WP3. |

The POC does **not** fix Postgres revision compare-and-swap or prove Spanner
selector queries. GCP-1505 adds a separate Postgres conditional update and
tests for selector LIST and watch. New assertion writes remain disabled by
default until compatible consumers are deployed. Public catalog authorization
is not production-ready until the platform-scoped Cedar work in GCP-1506.

## gcp-hcp-ctl POC commit inventory

No CLI code is transplanted by GCP-1505. These are review inputs for the
subsequent stories, in source order:

| Commit | Reuse guidance |
| --- | --- |
| `bf158ab` | Planner/create, pinned catalog and owned operation state: rebase against current CLI and Gecko public API. |
| `53f39da` | Default custom-role permission-support handling: generic SDK fix, separate from catalog feature. |
| `37b0532` | Bounded first-role cleanup rehearsal: retain only with ownership-safe recovery. |
| `97f314d` | Federation create parent path: generic SDK fix, separately reviewable. |
| `8976dfc` | Read-only catalog commands: requires authenticated/authorized routing from GCP-1506. |
| `23e2f3d` | Retained permission planner: review topology/identity constraints against current CLI. |
| `c58b57d` | Public API field-name preservation: generic serialization fix. |
| `4e7e419` | Existing-Cluster preview: requires fresh complete status from compatible publishers. |
| `a727640` | Additive update/resume/linked journal: review current teardown and unknown-outcome behavior. |
| `c3daed0` | Zero-delta and same-project tests: retain when rebasing update. |
| `ba21163` | Narrow absent-bootstrap-firewall cleanup: reconcile current GCP-1517 teardown fixes. |

CLI generic candidates (`53f39da`, `97f314d`, `c58b57d`) should be
separately reviewable from creation/update feature work. No CLI history is
rewritten or merged by this inventory.

## Untouched-main test baseline

On clean Gecko main `4edfcf3`, `make test` passed in all three modules
(`orlop`, `platform-api`, `controllers`) on macOS on 2026-11-24. The initial
attempt run in the GCP-1505 worktree overlapped code edits and was **not** a
valid untouched-main baseline; this result is from the separate clean main
worktree. There were no baseline test failures in the clean run. Postgres
integration tests skip without a reachable `POSTGRES_TEST_URL` (or local
`postgres://localhost/orlop_test`); Spanner `TestMain` exits without
`SPANNER_EMULATOR_HOST`; neither env var is set in CI. At baseline this
environment had no Docker, `psql` or `pg_ctl`, so the default suite alone was
not evidence of backend behavior. The new regression suites were subsequently
executed here against `postgres:15-alpine` (podman) and the Spanner emulator
container on the GCP-1505 integration branches and pass in full (see the
matrix below); production backend acceptance under real load is still
outstanding.

### Storage acceptance matrix

| Backend | Assertion writes | Selector LIST | Selector-filtered watch | Evidence in this worktree |
| --- | --- | --- | --- | --- |
| Memory | Guarded by stored resourceVersion | Supported | Supported | Focused tests run locally. Development only; not durable production storage. |
| Postgres | Conditional bigint-typed `UPDATE` against the expected revision; the concurrent stale writer conflicts | Verified | Verified | Full `storage/postgres` suite run against `postgres:15-alpine` via podman on 2026-11-24, after review hardening: CAS, selector LIST/watch and all other tests pass. CI still skips without `POSTGRES_TEST_URL`. |
| Spanner | Existing transactional revision check | Verified via the quoted-key JSONPath fix | Verified (client-side filtering of subscribed events) | Full `storage/spanner` suite run against the Spanner emulator container on 2026-11-24: selector LIST/watch and all other tests pass. CI still skips without `SPANNER_EMULATOR_HOST`. |

Local emulator/container runs prove the regression tests themselves, not
production readiness: deployed Postgres/Spanner still need acceptance under
real load, including the catalog and controller query paths, not just point
CRUD, before assertion writes are enabled.

## Review boundaries

1. Generic Orlop/API correctness: integer fidelity, guarded JSONPatch, Postgres
   revision CAS and selector LIST/watch. Keep separate from feature API changes.
2. Typed catalog, immutable application history, read-only requirements API and
   opt-in baseline Helm asset. No controller publisher or new dispatch gating.
3. This cross-repository inventory in `gcp-hcp/experiments/`.

Required follow-ups: GCP-1506 owns platform-scoped catalog authorization,
per-boundary immutable packaging and versionsync/dispatch; GCP-1507 owns CLI
integration and requirements publishers. Pending-intent and original-Version
aging decisions remain open per the implementation plan. Do not enable new
assertions on the strength of the default (env-gated) CI suite alone; the
recorded local backend runs above are the current best evidence.
