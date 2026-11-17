# Phase 1: API/catalog contracts — in progress

## Status and exact next action

**Not complete. No deployment, live API test or Phase 2 work has occurred.**

Live preflight failed because Google credentials require interactive reauthentication. The user was asked to run `gcloud auth login`. Do not attempt to bypass authentication, copy credentials, or deploy without a fresh live baseline.

Next: observe the in-progress API image build; smoke-test and publish it if successful, then resume the read-only live preflight after authentication is restored. Source validation/review and source publication are complete. Only after source/build and live rollout safeguards pass may the dev deployment proceed. Do not report Phase 1 complete until the required deployed tests and committed handoff exist.

## Refreshed source baseline

| Checkout | Branch / starting commit | Disposition |
| --- | --- | --- |
| `gcp-hcp` | `iam-bootstrap` / `425b5f1ae9156f4a72a35b24d29bc0de05192311` | User's newer `WifConfigTemplates` commit retained. Existing untracked historical plans/inventories and `.claude/settings.local.json` remain untouched. |
| `gecko` | `main` / `492bff6092b81b9c57417531ccfdd073ac1097f3` | Initially clean; implementation branch `feat/wifconfigtemplate-api-phase1` created without resetting work. Reviewed source is now committed/published at `6cef781f18552dde10413c31c331ea0ecbaaf019`. |
| `gcp-hcp-infra-dev-main` | `chore/dev-apahim-main-overlay` / `831338572d075d84747e9bc8ca5e00fe4d74dca9` | Clean; remote `origin/dev-apahim` readback matches this SHA. No deployment edits/publication yet. |
| `gcp-hcp-ctl` | Previously inspected `main` / `626da6d607c2529aee1ab87a9ccea212efe838c0` | No changes or artifacts in this phase. |

Gecko remotes: `origin` is `apahim/gecko`, `upstream` is `openshift-online/gecko`. Infrastructure origin is `apahim/gcp-hcp-infra`. Publish only task-owned branches or the approved dev deployment ref; never force-push/reset unrelated histories.

## Phase 1 implementation contract

- API resource: `gcp.managed.openshift.io/v1`, cluster-scoped `WIFConfigTemplate`, ordinary aggregated registration/persistence.
- Spec: `verified` (optional boolean; omitted means false), `schemaVersion` (1), typed `iam` contract. Remove obsolete completeness/validation/pendingComponents from the new catalog, not from historical input files.
- Template names: canonical numeric X.Y.Z, at most 32 characters. Immutable executable spec; metadata and verified may change. Delete always rejected initially.
- Canonical digest: SHA-256 over `encoding/json` of schemaVersion and typed IAM, excluding verified and all resource metadata. Set/map lists are sorted by complete keys; permissions, subjects and member sets are sorted; calculation does not mutate input.
- Version: optional public `spec.wifConfigTemplateRef` containing name/UID/digest, set once; public `status.conditions` with an actual status subresource. Only `Available` is exposed by Orlop's condition allowlist. No availability controller is activated until Phase 2.
- Cluster: optional `spec.platform.gcp.iam` beside immutable workloadIdentity, containing immutable exact default-worker-SA email and 1–128 immutable append-only template applications keyed by template name. New entries must match a currently verified stored template's UID/digest. Historical identical entries do not require current verification or a new catalog read during unrelated updates.
- New records are bound to existing immutable project/infra/issuer/WIF/component identities. Schema 1 requires supported deterministic names and the project's default Compute SA email matching its WIF project number. This is an authenticated assertion, not a cloud attestation.
- Storage validation callback uses the Platform API's own storage only. A missing/unavailable lookup fails closed for new references/applications. No cloud/Cincinnati calls in admission.
- New IAM assertions/appends are behind a default-off server rollout gate, `--enable-iam-application-writes` / Helm `iamApplications.enabled`. Phase 1 keeps it false; only Phase 2 may activate after every API replica and typed controller is compatible and dispatch prerequisites are deployed. Identical retained history remains updatable while the gate is off.
- Catalog opt-in chart values: `wifConfigTemplates.enabled`, `entries` with name/verified. A single typed baseline YAML file supplies permission data. Retention annotations disable GitOps prune/delete. Default chart deployment does not publish a template.

## Baseline equivalence

- Original baseline bytes SHA-256: `7dbe2d4d3a4a802d8e545ab1a4e5f20707132f754dc2ba6d6a5cbe0dda070bc7`.
- New executable contract digest: `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
- Semantic comparison passed: schema version and every IAM identity, naming rule, permission, subject and grant are identical after removing only obsolete lifecycle fields.
- Six component SAs, six custom roles/project bindings, 142 per-role permission entries: CCM 51, cloud-network 5, control-plane 36, CSI 30, registry 9, NodePool 11.
- Baseline remains unverified; no publication has asserted runtime permission completeness.

## Source validation observed so far

Passed:

- Focused race-enabled private API/server tests covering template validation, immutability, digest stability, public authorization, application append-only rejection and Version pins.
- Private template HTTP create/no-op PUT, verified false/true/false, immutable-edit/JSON-patch/parent-removal/delete rejection.
- Public Cluster HTTP update positive control plus application removal/edit/parent clearing/stale resource-version rejection; public Version condition filtering.
- Full controller module `go test ./...` against the changed local API module.
- Full Orlop module `go test ./...`; focused race test for conversion.
- Full platform-api module `go test ./...`.
- Platform API lint: zero issues after focused protocol-specific exceptions for the explicitly requested binary flag and existing symbolic `reference` wire field. No global lint exclusions were added.
- Repository formatting, patch whitespace checks, and two-pass API generation with identical second-pass hashes across 30 generated files.
- Helm 3.19.2 lint/render of the opt-in template catalog with verified false.
- Helm-to-API integration test renders false and true through the real chart, validates both resources, verifies retention annotations and pins the exact executable digest. Default rendering publishes no template and disables application writes.
- Final root `make lint-fmt` / `make lint` passed across Orlop, platform-api and controllers, with zero lint issues.
- Root `make test` passed all three modules with `HELM_BIN` supplied, including the real Helm render integration check. No live database/cloud tests are implied by the unit/integration suites.
- Focused race tests passed after the delete-validation and application-write-gate fixes, including local Helm rendering.

Initial failures and disposition:

- Initial test fixture path had one excess parent directory; corrected and rerun successfully.
- New catalog authorization-exemption registration was missing from the existing test expectation; updated to include authenticated-only WIF catalog reads.
- Initial API lint exposed bounds/format/complexity issues; added bounds, split validation helpers, formatted and reran successfully.
- Catalog test cleanup intermittently exceeded its five-second shutdown deadline. The modified test now owns its HTTP transport and closes idle connections before server shutdown; the focused race-enabled server suite then passed. Do not weaken assertions/timeouts to hide a recurrence.

Still pending at this checkpoint:

- API container build, source/image publication, resolved digest and runtime smoke.
- Live baseline, dev deployment, observed migration/API health, authenticated public/private behavior and restart persistence.

## Source commit and artifact build

- Reviewed Gecko source commit: `6cef781f18552dde10413c31c331ea0ecbaaf019`, `feat(api): add retained WIF IAM catalog contracts`.
- Published normally, without force, to `apahim/gecko:feat/wifconfigtemplate-api-phase1`; source checkout clean at commit time.
- API artifact tag being built: `quay.io/apahim/gecko:wif-p1-6cef781f1855-api`, target `linux/amd64` based on historical dev nodes; freshly verify live node architecture before rollout.
- Build context is a clean `git archive` of the exact source commit, extracted under the approved temporary directory. This deliberately excludes ignored local API-test certificates/private keys and local binaries, which the repository's existing containerignore does not exclude recursively.
- Build command: `podman build --platform linux/amd64 --label org.opencontainers.image.revision=6cef781f18552dde10413c31c331ea0ecbaaf019 --file <archive>/deploy/platform-api/Containerfile --tag quay.io/apahim/gecko:wif-p1-6cef781f1855-api <archive>`.
- Build result, image ID, registry digest, smoke result and image publication: pending. Do not infer success from the tag or running build command.
- No controller image is required for this Phase 1 rollout while assertions remain disabled and no Version pin is written. Do not change controller/chart refs globally; Phase 2 must deploy compatible controller builds before assertion activation.

## Architect review and disposition

Read-only architect review identified two blockers, both addressed in source and regression tests. The follow-up review confirmed both resolved and reported **no remaining blockers**:

1. Unstructured storage objects could bypass custom delete validation (notably PostgreSQL). Convert to the registered private type before delete validation in aggregated storage, direct/converting handlers and the patch finalization path. Regression tests seed an actual unstructured WIF template and exercise aggregated/private/converting delete routes, verify rejection/preservation and assert DeleteCollection remains unsupported.
2. Mixed API/controller versions could erase or block newly written history. Add the default-off assertion-write gate and document two-step activation/unsupported rollback after the first pin/history. Phase 1 does not activate the gate or write live Cluster history. Existing immutable entries remain protected even if writes are disabled later.

Other findings addressed: hard-coded baseline digest; spec-field compatibility test; canonical digest independent of current validation; advertised string-item bounds; explicit GitOps ownership of verification; required chart contract file; public authorization and parent-removal HTTP tests; Version status-subresource isolation.

Nonblocking framework limitation retained explicitly: storage not-found versus temporary-error messages are distinct and credential-free, but Orlop's custom validation hooks normalize errors to validation responses rather than preserving a separate transient 5xx category. New writes fail closed. Do not interpret a transient catalog failure as authorization success; error-category plumbing is a follow-up, not a hidden bypass.

Phase 2 prerequisites remain explicit: versionsync must preserve pinned references instead of whole-spec replacement, own Available, receive minimal versions/status permissions, and coordinate initial/upgrade/NodePool dispatch gates before assertion-write activation. Version catalog deletion/recreation creates a new Version resource and does not preserve the old Version pin; retained templates remain immutable and undeletable.

Nonblocking follow-up observations: Phase 1 private Version pins are intentionally not activated operationally, although the API permits a platform writer to set one; do not do so until compatible versionsync is deployed. All Orlop tests pass with registered-type delete validation; unregistered/un-decodable kinds now fail deletion closed. The Helm integration test requires explicit `HELM_BIN` and was executed locally; wiring that tool into CI is not claimed. Default chart publication/write-gate rendering was additionally checked directly.

## Tooling and live preflight

- Go: `go1.27.4 darwin/arm64`.
- Podman engine is reachable and arm64; Quay login lookup returned `apahim`. No credential contents were read or copied.
- Historical target nodes are amd64; confirm live architecture before rollout. Build linux/amd64 explicitly if confirmed; do not deploy an arm64-only image to amd64 nodes.
- Helm was absent on PATH. A temporary official Helm 3.19.2 darwin/arm64 binary was downloaded and archive SHA-256 verified: `f0847f899479b66a6dd8d9fcd452e8db2562e4cf3f7de28103f9fcf2b824f1d5`.
- Temporary Helm path: `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/helm-v3.19.2`. This is local-only, not a durable artifact; recover from the official checked release if absent.
- Existing kubeconfig lists the management dev context. Google credential refresh failed with reauthentication-required/non-interactive error. No live counts, UIDs, readiness or image architecture were obtained.
- `uv` is also absent on PATH; resolve the infrastructure renderer's pinned toolchain before modifying/rendering deployment artifacts. Do not hand-edit rendered output.

## Future dev rollout boundary

1. Read fresh regional/management API state and confirm no hosted customer objects; record current eight Gecko Application/workload revisions, API image digest, DB/migration Job UIDs, Spanner target and Argo health.
2. Confirm a new template is the only intended catalog allocation. No customer IAM or hosted resources.
3. Publish reviewed Gecko source to a reachable fork ref; build from the exact source commit using repository-root context and `deploy/platform-api/Containerfile`. Record unique Quay tag and resolved digest. The API migration Job must use the same API image.
4. Override only the owner/region-specific platform-api-server Application's chart repo/ref and image registry/repository/tag. The inherited `image.tag` Helm parameter takes precedence over valuesObject: replace it explicitly. Do not change global gecko_ref and unintentionally roll all controllers.
5. First deploy API/schema support with catalog publication disabled and `iamApplications.enabled:false`. Observe migration and API health against the same Spanner DB. Then enable a real release-boundary catalog entry, initially verified false. Preserve `prune:false` and persistent resources. Do not pin live Versions while old versionsync replaces their entire spec.
6. Test discovery, authenticated get/list, anonymous/customer write denial, immutable content/deletion rejection, no-op apply and verified toggling, finishing false. UID-guard the planned API pod restart and prove template persistence. Do not claim customer/cloud verification.
7. Retain the explicitly published unverified catalog entry under the no-delete policy; it is not a disposable fixture. Record its UID/digest and ownership in the final handoff.
8. Read back all health/storage/image invariants and commit evidence/handoff before Phase 2.

No infrastructure publication commit, image digest, cloud operation, resource allocation, deployment or restoration is asserted yet. Gecko source publication alone does not deploy anything. Do not roll back to an older API that can erase newly introduced fields; never drop stored data or switch to memory as restoration.
