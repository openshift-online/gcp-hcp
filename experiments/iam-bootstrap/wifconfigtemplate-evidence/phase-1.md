# Phase 1: API/catalog contracts — in progress

## Status and exact next action

**Not complete. API/migration deployment and initial live checks passed; public catalog routing is blocked pending a separately approved Endpoints apply. Phase 2 has not started.**

Initial live preflight failed because Google credentials required interactive reauthentication. The user subsequently ran `gcloud auth login`; authentication and the live baseline now pass. Do not bypass authentication or copy credentials.

Next: await the user's decision on the completed read-only Terraform plan below. **The user explicitly selected PLAN ONLY for this Endpoints change. No Terraform apply is authorized.** API/migration rollout passed, but the public route gate failed, so no catalog entry or verification toggle has been published. Do not advance to Phase 2.

## Refreshed source baseline

| Checkout | Branch / starting commit | Disposition |
| --- | --- | --- |
| `gcp-hcp` | `iam-bootstrap` / `425b5f1ae9156f4a72a35b24d29bc0de05192311` | User's newer `WifConfigTemplates` commit retained. Existing untracked historical plans/inventories and `.claude/settings.local.json` remain untouched. |
| `gecko` | `main` / `492bff6092b81b9c57417531ccfdd073ac1097f3` | Initially clean; API source commit `6cef781...` followed by native-build commit `cc26254c92b64d71a0b8cf00faf955fbc8e5b925`, published on `feat/wifconfigtemplate-api-phase1`. The latter artifact is deployed. |
| `gcp-hcp-infra-dev-main` | `chore/dev-apahim-main-overlay` / `831338572d075d84747e9bc8ca5e00fe4d74dca9` | Initially clean with matching remote. API rollout `469a838f...` is published/deployed; read-route fix `20491381...` is locally committed but not pushed/applied. |
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

- Remaining deployed gates: public Endpoints routing, catalog publication/verification/reapply tests, API-restart persistence and final handoff. The API-only deployment itself has now passed health/storage/image checks below.

## Source commit and artifact build

- Reviewed Gecko source commit: `6cef781f18552dde10413c31c331ea0ecbaaf019`, `feat(api): add retained WIF IAM catalog contracts`.
- Published normally, without force, to `apahim/gecko:feat/wifconfigtemplate-api-phase1`; source checkout clean at commit time.
- API artifact tag being built: `quay.io/apahim/gecko:wif-p1-6cef781f1855-api`, target `linux/amd64` based on historical dev nodes; freshly verify live node architecture before rollout.
- Build context is a clean `git archive` of the exact source commit, extracted under the approved temporary directory. This deliberately excludes ignored local API-test certificates/private keys and local binaries, which the repository's existing containerignore does not exclude recursively.
- Build command: `podman build --platform linux/amd64 --label org.opencontainers.image.revision=6cef781f18552dde10413c31c331ea0ecbaaf019 --file <archive>/deploy/platform-api/Containerfile --tag quay.io/apahim/gecko:wif-p1-6cef781f1855-api <archive>`.
- This original emulated build was subsequently canceled; use the clean native-build artifact recorded below, not this old pending tag.
- User created `quay.io/apahim/gecko` as public and confirmed local Podman login. Anonymous Quay repository API readback independently returned `is_public:true`, `state:NORMAL`, with no tags yet. No image-pull secret or new cluster credentials are needed for this public repository; verify the actual tagged artifact's anonymous pull after publication.
- No controller image is required for this Phase 1 rollout while assertions remain disabled and no Version pin is written. Do not change controller/chart refs globally; Phase 2 must deploy compatible controller builds before assertion activation.

### Mac-friendly build adjustment (requested by user)

The original amd64 compiler running inside the arm64 Podman VM was too slow. The user requested native local cross-compilation plus binary-only image packaging. The owned Podman build client PID 90614 was deliberately terminated; its background job exited 143 while compiling. It did not finish or publish the original `wif-p1-6cef781f1855-api` artifact. Do not reuse that pending tag as proof of a built image.

The native-build change added root Makefile local binary/image targets, README instructions and `deploy/Containerfile.local`, subsequently committed as `cc26254...`. Host Go cross-compiles with CGO disabled and explicit Linux GOARCH; separate ignored per-architecture/per-component binary contexts are packaged without any Containerfile RUN instructions. The pinned runtime base, CA trust bundle and non-root UID match the ordinary image. Existing CI/source Containerfiles are unchanged. Runtime help smoke may execute under emulation, but compilation and image assembly do not.

Native host builds of both linux/amd64 binaries passed. `file` independently identified both as statically linked x86-64 Linux ELF executables; Go build metadata confirmed Go 1.27.4, CGO_ENABLED=0, GOOS=linux and GOARCH=amd64. These initial test binaries correctly showed dirty source and were not deployment artifacts. Invalid target architecture was rejected before building; arm64 dry-run showed matching Go target and container platform for both components. Packaging-only validation images built under localhost; the generic targets default to localhost and never push. The approved clean-source API artifact was subsequently built/published to Quay as recorded below.

The initial dirty validation images were not used for deployment. The clean rebuild, completed architect review and source-specific publication are recorded below.

### Completed native build and first deployment publication

- Native-build source commit: `cc26254c92b64d71a0b8cf00faf955fbc8e5b925`, `build: add native cross-compiled local container images`, published normally on the same Gecko feature branch. This changes only Makefile, README and the new local Containerfile; chart/API/controller/Orlop source is unchanged from reviewed `6cef781`.
- Final local layout is `bin/local/linux-<arch>/{api,controllers}/app/gecko`; the app directory is copied with UID/GID 65532 ownership and the executable is mode 0555. Builds disable Go workspaces and use readonly module files. Default tags include architecture and a dirty suffix where applicable; generic default repository is `localhost/gecko`; nothing pushes automatically.
- Both validation images built successfully with no target-platform RUN instruction. Image export verified writable `/app` ownership, read-only binary and CA bundle. API and controller `--help` smoke tests passed under non-root, read-only filesystem and no-network settings. Go metadata/ELF checks confirmed Linux/amd64 static binaries. Rebuilding over existing mode-0555 outputs also passed.
- Architect follow-up confirmed all native-build review findings addressed and **no blockers**. Existing CI Dockerfiles are unchanged; these are development artifacts, not a claim of Red Hat pipeline promotion.
- Clean Phase 1 API build command: `make local-image-api LOCAL_ARCH=amd64 LOCAL_IMAGE_REPOSITORY=quay.io/apahim/gecko LOCAL_IMAGE_TAG=wif-p1-cc26254c92b6`, followed by explicit Podman push.
- Published tag: `quay.io/apahim/gecko:wif-p1-cc26254c92b6-api`.
- Published immutable manifest digest: `sha256:b1a477398519f3561aa080bd7deb3777c69fac32c1f74e2f524bec3fd6bb05cc`.
- Local image/config ID: `sha256:6b471b7f5c8f07705aa1657c9510a6625b1982407d799c8d801ff26639c80072`.
- Anonymous registry manifest/config reads passed; content SHA matched the push digest. Registry config confirmed Linux/amd64 and exact clean source label. Actual API image help smoke passed, and Go build metadata reported `vcs.modified=false` at source `cc26254...`, host Go 1.27.4. No controller image was published/deployed for Phase 1.
- Infrastructure commit `469a838f41945807da1e92b39777aea0c9a7846f`, `chore(dev): deploy Phase 1 WIF catalog API support`, was pushed normally to `apahim/gcp-hcp-infra:dev-apahim`. Exactly two owner/region API files changed. Source pin/tag updated to the clean native-build artifact; the reviewed API chart content is identical between `6cef781` and `cc26254`.
- Refreshed the existing regional root Application through its Argo refresh annotation. A bounded wait observed the exact child chart revision, successful Argo operation and Deployment rollout; independent digest/health checks are recorded below.
- No catalog entry has been intentionally published yet. IAM application writes remain disabled. ESP 2.57.1, seven old controller pins, Spanner and prune safeguards remain selected.
- One staging command initially used root-relative git-add paths while working under `argocd/`; it failed before commit/publication. Corrected the working directory and committed/pushed once. No mutation was replayed.

### API rollout observed; public routing gate failed

- The exact chart revision, Argo operation success and API Deployment rollout wait all completed successfully. Re-ran root `make test` with Helm enabled on clean Gecko commit `cc26254`; all three modules passed and the checkout remained unchanged.
- Active API Pod: `platform-api-server-6669fdd78f-p2h7x`, UID `0e9e65cb-8d95-4d2b-ba1e-78f15998985a`, Ready, running the published Quay digest `sha256:b1a477398519f3561aa080bd7deb3777c69fac32c1f74e2f524bec3fd6bb05cc`.
- Matching migration Job replacement UID `e11cbeec-b922-465b-9bc2-73d997f21b93`, Complete with one successful Pod. Its image is the same new API tag. This was the anticipated image-driven Job replacement, not a database reset.
- Spanner target and KCC DB UID are unchanged. New API arguments keep IAM application writes false. Private aggregated `wifconfigtemplates` get/list works and returns an empty catalog. Clusters/NodePools remain zero and old controller image pins are retained.
- Public routing check first stopped on anonymous 404 before reaching its authenticated branch. After the user's identity-token reminder, explicitly compared both paths using the **same fresh gcloud identity token in memory**: Versions 200/21 items; WIFConfigTemplates 404 with ESP's fixed `The current request is not defined by this API.` message. This is not an authentication failure or an API persistence failure.
- Inspected the Terraform-owned ESP OpenAPI template: it explicitly enumerates public paths and lacks the new WIF collection/item GET routes. No authentication bypass or broad catch-all route was attempted.

### Endpoints source fix and user-authorized plan only

- Added exactly two GET-only paths to `terraform/modules/service/templates/esp-openapi.yaml.tftpl`, inheriting the existing global OAuth2 requirement. No write routes, anonymous security override, issuer/audience changes or changes to existing routes.
- Semantic YAML comparison passed: removing the two new routes produces the exact original configuration. Architect review approved with **no blockers**, including unique operation IDs and item path parameter validation.
- Source fix is locally committed in the selected deployment worktree at `2049138124954c5c0e3928053f813d4f725a0e46`, `fix(api): route authenticated WIF catalog reads through ESP`. It has not been pushed/applied at this checkpoint; the watched dev ref remains the successful API rollout `469a838f`.
- The user selected **Plan only** when asked about planning/applying the existing personal dev Endpoints service update. **Do not run apply, manually deploy an Endpoints config, change IAM, enable APIs or broaden the plan.**
- Use the authoritative original Terraform root `/Users/asegundo/git/osd/gcp-hcp-infra/terraform/config/dev-all-in-one/apahim`; no state, tfvars, keys or other credentials were moved/copied. Its module source references the original checkout's service template, not the deployment worktree.
- Before mirroring, verified that original service template exactly matched the selected worktree's pre-change baseline (SHA-256 `ed245f2cd232d040c5be278e8c8d69414d8e8a400a5ff90a924fcefcc7ca73ad`). Mirrored only the reviewed 23-line route addition into that previously clean source file so the existing Terraform root can plan the intended config. This is a task-owned uncommitted change in the original checkout; preserve it separately from the user's existing `terraform/modules/region/spanner.tf`, `variables.tf` edits and untracked PDF. Nothing else there was modified by this preparation.
- Terraform 1.17.1 matches the original checkout's `.tool-versions`; a temporary official darwin/arm64 binary was downloaded and archive SHA-256 verified: `b1ecb3fe0d0a0290c1ab6f83d967d43f25da05d0b671ee0fb355252268dce95a`.
- Terraform binary path: `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/terraform-1.17.1`. Python remains in the dedicated venv.
- Full non-targeted refreshed plan was started with `-input=false -no-color -detailed-exitcode -out=<restricted>/endpoints.tfplan`, from the existing root. A short-lived gcloud access token is passed only in that process environment, never logged or persisted as a credential file. Raw plan/log files are restricted under a mode-0700 temporary directory, file mode 0600, not Git; they may contain sensitive Terraform data and must never be committed or printed wholesale.
- Original state SHA-256 before and after plan: `f8fdb0f30354bfc153d96ed5e09075bf0ccfaeeadcfca12221fb25b4608c8262`. The authoritative state bytes are unchanged.

### Completed plan result — not applied

- Full refreshed, non-targeted Terraform plan completed successfully, detailed exit code 2.
- **Exactly one managed resource update:** `module.service.google_endpoints_service.hcp`, actions `["update"]`. No creates, deletes or replacements; no other managed resource changes, including no IAM or Spanner changes.
- Target project is unchanged: `gcp-hcp-dev-svc-apahim1110`.
- Managed service identity is unchanged: `hcp-api.endpoints.gcp-hcp-dev-svc-apahim1110.cloud.goog`.
- Parsed the actual saved plan's before/after OpenAPI configs and independently verified that the only semantic difference is the two GET collection/item paths. All existing routes, global OAuth2, issuer, audience and other configuration are identical. No replacement paths are present.
- Sensitive saved plan/log directory (local-only, mode 0700): `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-endpoints-plan-j_ulxnda/`. `endpoints.tfplan` and `plan.log` are mode 0600; a sanitized `summary.json` is also kept there. Never commit or print the raw plan/log.
- Saved plan SHA-256: `f463520a8a9e4df2a6af403e37acb68c26b906b3115e2c8f1223a1492fa8e469`.
- **No apply, manual Endpoints deployment, new service config rollout, API enablement or IAM change was executed.** The published healthy API remains in place with catalog publication and application writes disabled. Await explicit user approval; revalidate state/source and plan scope before any later apply.

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

### Resumed live baseline after user login

- Authenticated Google account: `asegundo@redhat.com`; regional GKE cluster RUNNING and Spanner `gecko` database READY.
- Private IP endpoint requests timed out because the control plane is private. The already-configured GKE DNS endpoint allows external IAM-authenticated access. Refreshed normal kubeconfig entries using `gcloud container clusters get-credentials ... --dns-endpoint` for regional and management clusters; Kubernetes reads then passed. No network configuration/permissions changed.
- Cloud Workflows API is disabled in the regional dev project. The CLI did not enable it; no workflows or authority were added.
- Regional Gecko: all eight Deployments Ready, all relevant Argo Applications Synced/Healthy, seven controllers and API on published `f90f6df` artifacts. Nodes independently report amd64.
- API running digest: `sha256:5713488f6a85e1faa8418267c6ddcb9dcc0e775282a64c891875102b7ae7a990`; ESP remains 2.57.1. API Deployment UID `4187325a-61e2-43c4-8c3b-7286030c6c55`, generation 4; Pod UID `cd74d08b-26a9-427a-97e0-2926cef4b136`.
- Migration Job `platform-api-server-migrate` UID `b0bb3371-fbd6-40b6-93da-1435f9c7956e`, Complete, one successful Pod. Database KCC UID `99e4163c-79f9-4875-92b9-0e576b9a5927` unchanged. Preserve this DB UID across rollout; replacing the completed migration Job for its new API image is expected and separately observed.
- Regional catalog: Clusters/NodePools 0/0, Versions/Channels 21/4. Management HC/HCP/NodePools: none.
- Public API baseline: Versions anonymous 401, authenticated 200 with 21 items; authenticated customer Cluster list 200 with zero items. Tokens stayed in memory and were not recorded.
- Existing API Application uses Spanner, `prune:false`, self-heal enabled. The chosen source ref and watched `origin/dev-apahim` were both freshly confirmed at `831338572d075d84747e9bc8ca5e00fe4d74dca9` before preparation.

### Python and prepared deployment artifacts

- User requires **all Python work to use a venv**. Created `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-phase1-venv`; all subsequent Python scripts/rendering use its `bin/python`. Pinned installed dependency: `PyYAML==6.0.3`. No package was installed into system Python.
- Initial deployment preparation changed exactly two files in `gcp-hcp-infra-dev-main`: owner/region `platform-api-server` override and its generated regional dev Application, subsequently committed/published as `469a838f...`. No other environment/controller/management rendered files changed.
- Owner override pins API chart source to `apahim/gecko` at `6cef781...`, supplies image registry/repository/tag via Helm parameters (overriding inherited tag), keeps IAM writes and catalog publication off, preserves Spanner/prune safeguards and explicitly retains running ESP 2.57.1.
- Chart diff against current deployed `f90f6df` was inspected: apart from the IAM additions, the only platform API chart default change is ESP 2.57.1 -> 2.57.2, intentionally neutralized by the dev override.
- Ran the repository renderer using the venv; repeated rendering produced no additional diff. Assertions passed for exact two-file scope, source pin, image parameter precedence, disabled writes/catalog, existing sidecar, persistent storage and pruning guards. Deployment architect review found **no blockers** and confirmed both API and migration Job receive the override. Publication was deferred until the native-built image passed clean-source and anonymous registry checks, then completed as recorded above.

## Remaining acceptance and safe resume

1. Obtain the user's separate Endpoints apply decision. The API image/migration already rolled out successfully: do not rebuild or replay that rollout merely to resume this session.
2. If apply is approved, revalidate the source/state/plan and require exactly the reviewed one-resource in-place service update. Stop on any additional changes. Preserve original user edits and state/credential locations.
3. Observe the managed ESP rollout loading the new service config, then require public catalog list/get with a fresh gcloud identity token. Verify anonymous requests and public writes are denied, rather than bypassing the gateway or weakening authentication.
4. Only after that gate, publish a real release-boundary template through dev-only GitOps, initially verified false. Keep IAM assertion writes false, the seven controller pins unchanged, Spanner/prune safeguards intact and no live Version pins. Preview sync behavior: the API chart's migration Job has Force/Replace annotations, so do not accidentally replay it during catalog-only syncs without a deliberate observed plan.
5. Complete template discovery, content equality, authenticated reads, invalid contract/edit/delete rejection, no-op apply and GitOps verified toggling ending false. UID-guard the agreed API Pod restart and independently prove template UID/digest/content persistence. No live Cluster/NodePool or customer IAM allocation.
6. Retain the unverified catalog entry under the no-delete policy; record its UID/digest and ownership. It is not a disposable fixture.
7. Read back all health/storage/image invariants and commit final evidence/handoff before marking Phase 1 complete. Do not advance to Phase 2 on a partial gate.

The API rollout and image digest are independently verified above. Public routing and catalog/persistence acceptance remain incomplete; the Endpoints change is plan-only until separately approved. No customer IAM allocation, hosted Cluster operation or restoration is asserted. Do not roll back to an older API that can erase newly introduced fields; never drop stored data or switch to memory as restoration.
