# GCP-1505 — WIFConfigTemplate integration inventory

This inventory records dated POC evidence, research limits and integration checks.
The baseline comparison is against repository mains on 2026-11-24; integration
outcomes are dated below. Requirements, open decisions and delivery status belong
in the [implementation plan](../../implementation-plans/wifconfigtemplate-catalog-and-iam-lifecycle.md).

Use the commit maps and storage matrix for integration review; the historical
sources/results and prefix research at the end are evidence, not runtime instructions.

## Integration outcome

Recorded 2026-11-25; the tables below remain the 2026-11-24 point-in-time
evidence.

- **WP1 (this inventory):** retained here as the reusable-commit map and backend
  test-disposition record.
- **WP2 (generic API/storage correctness):** merged to Gecko `main` in
  [gecko#690](https://github.com/openshift-online/gecko/pull/690) — integer
  fidelity beyond 2^53, guarded RFC 6902 JSONPatch on the stored public
  projection, real Postgres revision CAS, and selector LIST/watch on every
  supported backend.
- **WP3 (catalog and application API):** [gecko#692](https://github.com/openshift-online/gecko/pull/692)
  (open, in review) — typed WIFConfigTemplate v1 API, Version pins, bounded
  append-only Cluster
  applications, read-only Cluster/NodePool requirements projection, private
  receipt types and the opt-in Helm baseline, with `iamApplications.enabled=false`
  and catalog publication disabled by default.

Integration changes relative to the POC:

- The executable baseline gained the seventh component (`ingress-op`): 147
  permission entries (114 distinct), 16 IAM resources / 17 scoped grants.
  The six-component POC had 142 entries and 14 resources / 15 grants. The five
  added ingress DNS permissions already existed in `hcpControlPlaneProject`, so
  the distinct-permission count did not change.
- The digest protocol canonicalizes the encoding (every object key sorted
  alphabetically) so the digest is independent of Go struct declaration order;
  the integrated golden digest is
  `sha256:71d9aaf90729b0365610a16bd46d7cb4d4582e1e69b79f114f968086017d1af4`.
  The prior six-component POC digest was
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
- Review hardening in the same integration: server-side apply now runs the
  type's CustomValidator before storage writes (closing the apply-path gap);
  delete-validation decode failures log resource identity; custom roles reject
  service-account impersonation and credential-minting permissions; guest
  subject namespace/name segments are bounded to the Kubernetes 63-character
  limit; naming bound fields use `maximum*` JSON keys; reference name/uid
  carry `minLength: 1` and list items are required and atomic.
- Remaining work is tracked in [GCP-1506](https://redhat.atlassian.net/browse/GCP-1506)
  (WP4/5/9/10: authorization, packaging, discovery/dispatch, dependency
  alignment), [GCP-1507](https://redhat.atlassian.net/browse/GCP-1507)
  (WP6/7/8: CLI lifecycle, requirements publishers, runtime compatibility), and
  [GCP-1508](https://redhat.atlassian.net/browse/GCP-1508) (WP11: operations and validation).

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
tests for selector LIST and watch (delivered in
[gecko#690](https://github.com/openshift-online/gecko/pull/690)). New assertion writes remain disabled by
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

The historical Spanner failure was **HTTP 500 on label-selector LIST from invalid
JSONPath handling**. The POC did not repair or retest it; gecko#690 supplied the
quoted-key JSONPath fix and the backend regression evidence recorded above.

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

## POC sources

The implementation experiment is preserved in dedicated source branches, not in
the session notes formerly collected under `experiments/iam-bootstrap/`:

| Source | Published branch | Reviewed reference tip |
|---|---|---|
| [Gecko](https://github.com/apahim/gecko/tree/poc/wifconfigtemplate) | `origin/poc/wifconfigtemplate` | `6c28cf4348a8ab4f524b91ece1cdbe29f1389c44` |
| [gcp-hcp-ctl](https://github.com/apahim/gcp-hcp-ctl/tree/poc/wifconfigtemplate) | `origin/poc/wifconfigtemplate` | `ba211636d48d6cedf11e6090536ef7c5f985800f` |

Immutable implementation references:

- [Gecko contract and dispatch protocol](https://github.com/apahim/gecko/blob/6c28cf4348a8ab4f524b91ece1cdbe29f1389c44/platform-api/WIFCONFIGTEMPLATE.md).
- [Executable POC baseline](https://github.com/apahim/gecko/blob/6c28cf4348a8ab4f524b91ece1cdbe29f1389c44/helm/charts/platform-api-server/files/wifconfigtemplate-project.yaml).
- [CLI protocol and recovery](https://github.com/apahim/gcp-hcp-ctl/blob/ba211636d48d6cedf11e6090536ef7c5f985800f/docs/WIFCONFIGTEMPLATE.md).
- [Owned firewall-aware cleanup](https://github.com/apahim/gcp-hcp-ctl/blob/ba211636d48d6cedf11e6090536ef7c5f985800f/pkg/infra/network/cleanup.go).

Source comments may describe an earlier phase, not final implementation status.
Integrate selected changes against current main rather than merge the POC branch
wholesale. The CLI tip retains both `a727640` (linked updater) and `c3daed0`
(same-project/zero-delta tests) as verified ancestors, followed by the cleanup fix.

## POC results and limits

These observations were recorded at the end of the POC on **2026-11-20**.

| Area | Observed result | Limit of that evidence |
|---|---|---|
| API/catalog | Persisted template/Version references, immutable content, verification toggles, authenticated discovery, restart/pin persistence and fail-closed dispatch | Personal dev using Spanner; not all database/backends or production authorization paths |
| Fresh setup | Template-driven 5.0.2 install and separate 4.22.21 install, cloud readback, TLS/OIDC login, healthy guest/control plane and one Ready worker | Not a complete six-component functional/optional-feature matrix |
| Role update | Separate unbound-role additive update, real stale-etag rejection, field-mask preservation and separate-process readback after deliberately discarded success | Simulated response loss, not an actual network outage/power-loss test |
| Target gating | 4.22.21 → exact 5.0.0 remained unaccepted with `WIFConfigUpdateRequired` while the target Version was Available and the guest stayed on source | Customer path; automatic and other race cases primarily have offline/integration coverage |
| IAM application | Zero role-permission delta, unchanged six role etags, exactly one distinct target assertion despite equal executable digest | Not a live permission-expanding release transition |
| Upgrade | Guest CVO completed exact 5.0.0, then one worker completed 5.0.0; RHCOS 9 → 10 replacement stayed within one-worker surge | Does not authorize unsupported edges, aliases or OS overrides |
| Coexistence/cleanup | Original 5.0.2 installation remained healthy; secondary hosted resources/VMs removed normally; reviewed CLI cleanup removed secondary IAM/network and restored policy memberships | Same-project lifecycle separation, not hard cross-cluster GCP authorization isolation |

The POC ended with template verification and assertion writes closed. The secondary
installation was removed; expected role/pool tombstones were retained. One original
validation installation and platform infrastructure were intentionally retained.
These observations are neither current health checks nor authorization to delete
retained resources. Operational identities/journals remain restricted handoff material.

### Discovery-floor observation

Adding the 4.22 discovery floor produced 39 Versions (36 4.22 and three 4.23), not
one source Version. The 4.23 entries correctly lacked a matching template. Restoring
the old floor would age/delete Versions even though retained templates remain.

## Permission research and assurance limits

The following request-specific findings inform review; they do not establish
universal sufficiency or minimality:

- CCM's reviewed baseline follows 48 declared permissions plus three operation
  reads. A matched internal-LB request succeeded with regional backend-service use
  without generic backend/network use; that result is request-shape-specific.
  An empty instance group was an invalid LB fixture, not an IAM denial. Do not add
  permissions to repair fixture errors or call the incomplete matrix a full pass.
- PSC requests tested generic **and** PSC-specific create/delete/setLabels rights;
  Get used generic get. Internal address use required `compute.addresses.use` in
  that request, not an inference that `useInternal` is universally interchangeable.
- Compute's Service Directory side effects required namespace-create,
  service-create and service-delete for the tested path. Empty/shared namespace
  cleanup ownership is separate; do not add namespace-delete because an error
  lists a group of candidate permissions.
- A cross-project PSC endpoint can be authorized yet have REJECTED connection
  policy. Distinguish producer acceptance from IAM; do not infer a need for
  producer-project IAM grants from a rejected connection alone.
- The DNS ten-permission candidate covered selected public/private record workflows.
  Existing-zone Get, absent-zone lookup and List have different checks; an absent
  404 alone is not positive proof of Get authorization. Those API probes did not
  prove guest WIF, resolver/delegation behavior or universal role minimality.
- CredentialsRequest lists were incomplete or described different controllers;
  CSI broad-role unions, CAPG versus Machine API, registry bucket-update/tags and
  configuration-dependent discovery must be assessed against the actual payload.
  Resource Manager tags and prefix-scoped alternatives are separate from the
  supported project-wide baseline until implemented and validated.

## Prefix prototype and research anchors

This is historical input to [Plan 2](../../implementation-plans/wifconfigtemplate-resource-name-prefix-conditions.md),
not evidence that prefix-conditioned IAM is implemented.

The early prefix prototype is **not retained as a complete recoverable source
snapshot in the published POC branches or the locations checked during consolidation**.
The referenced HyperShift/CSO prototype worktrees were absent; the CCM/CSI checkouts
found were clean at their old bases, without the reported changes. A partial
HyperShift tracked-file diff was preserved in the private context archive, but
does not contain the originally untracked new files and cannot rebuild the prototype.
Plan 2 therefore starts from the design and current upstream source, not a
cherry-pick-ready prototype.

| Component/input | Earlier inspected base or identifier |
|---|---|
| HyperShift | `1d3c34c749` |
| CCM cloud-provider-gcp | `3e67bea588c9eac2978d14726db729ee420cd557` |
| cluster-storage-operator | `1b546af` |
| csi-operator | `f5d54d11ed1ba2ee3cefb75a4b2c18b30c6df378` |
| Traced CAPI / CAPG | v1.14.3 / v1.13.1 |
| Proposed CPO capability label | `io.openshift.hypershift.control-plane-operator-supports-gcp-resource-naming` |
| Proposed CSO → GCP CSI env input | `GCP_RESOURCE_NAME_PREFIX` |

These are research anchors, not release selections or proof that current images
support the proposed ABI:

- Source traces found CAPI MachineNaming → infrastructure clone name → CAPG instance
  name; an unset Compute boot-disk name defaulted to the instance name. Revalidate
  against shipped images, including any explicit boot-disk naming.
- The prototype distinguished **PSC endpoint subnet** from **worker/CCM subnet**.
  Reusing `gcp.subnet` for both requires an API decision; both network shapes belong
  in validation/cleanup identity checks.
- Proposed PSC names were `<infraID>-psc-endpoint` / `<infraID>-psc-ip`. The existing
  `<infraID>-internal-cluster` firewall already matched; lifecycle adoption still
  required ownership checks.
- Proposed CSI flags were `--volume-name-prefix=<infraID>-pvc` and
  `--snapshot-name-prefix=<infraID>-snapshot`, retaining full UIDs. They may also
  affect PV object names. Validate sidecar and snapshot-to-image paths; they do not
  rename existing disks. The earlier 12-character-prefix budget yielded 53-character
  disk and 58-character snapshot names; recalculate for the final naming contract.
- Registry naming support was not established as a component change; inventory
  existing allocation behavior before adding an operator input.

Earlier notes report unit/mock tests and HyperShift envtest on Kubernetes 1.34.1
(13 create and six update naming cases), with no HCP naming envtest suite. The
results are not independently reproducible from a complete retained prototype and
were not live prefix-IAM, traffic, release-image or mixed-fleet upgrade proof.

### Provisional role split and selectors

Earlier research split six-component permission unions into 15 logical roles:
CCM four (resource mutations, instance use, read-only, support), CPO three (DNS,
resource mutations, support), and two each for cloud-network, CSI, registry and
nodepool. The CCM support role held **30 project-wide exceptions**, not just
reads; registry separated `storage.buckets.list` into a discovery role. This is
neither a final role count nor a verified condition set for the seven-component baseline.

Candidate selectors were instance/boot disk `{infraID}-`, CSI disk `{infraID}-pvc-`,
snapshot and snapshot-derived image `{infraID}-snapshot-`, and bucket `{infraID}-`.
They require the producer/IAM capability matrix. The prototype described
`ErrRoleSplitRequired` for simultaneous permission and grant changes on a shared
role; reconstruct that safety test rather than assume the unpublished helper exists
or relax Plan 1's topology refusal.
