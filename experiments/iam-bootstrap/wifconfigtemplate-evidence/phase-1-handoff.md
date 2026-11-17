# Phase 1 handoff — complete

**Completed:** 2026-11-17, live acceptance finished at `15:04:31Z`.
**Scope:** APIs/catalog and personal dev validation only. **Phase 2 has not started.**

## Fresh-session reading order

1. Applicable `AGENTS.md` instructions in each selected checkout.
2. [Progress and decisions](../wifconfigtemplate-progress.md), then this handoff.
3. [Detailed Phase 1 evidence](phase-1.md), [catalog validation results](phase-1-catalog-validation.json), and [dev Endpoints rollout](phase-1-dev-service-rollout.json).
4. Gecko `platform-api/WIFCONFIGTEMPLATE.md` for API/digest/activation contracts.
5. The phased plan at `/Users/asegundo/.opencode/plan/iam-wifconfigtemplate-version-catalog.md`, using the committed progress/handoff as the actual-state authority.

Do not rebuild, reapply the saved Terraform plan, replay the migration Job, repeat
the verification toggles or restart the API merely to reconstruct this session.
Refresh state read-only before any new phase or operational action.

## Exact source and artifacts

| Repository / location | Revision and state |
| --- | --- |
| `../gecko`, branch `feat/wifconfigtemplate-api-phase1` | API contracts `6cef781f18552dde10413c31c331ea0ecbaaf019`; native local builds `cc26254c92b64d71a0b8cf00faf955fbc8e5b925`. Both published to `apahim/gecko`; the latter is deployed. |
| `../gcp-hcp-infra-dev-main`, branch `chore/dev-apahim-main-overlay` | Published `dev-apahim` tip `d7231a6ab273f2a2fd0899d714e824c349b2f9a8`. API rollout `469a838f...`; Endpoints source `20491381...`; initial catalog `f8b25d31...`; true toggle `716a3561...`; final false restoration `d7231a6a...`. |
| `../gcp-hcp-ctl` | Untouched; previously inspected `626da6d607c2529aee1ab87a9ccea212efe838c0`, still using the older API dependency. Do not run its legacy broad-role IAM path as a test of this feature. |
| `gcp-hcp`, branch `iam-bootstrap` | This committed handoff/progress/evidence records completion; find its exact commit with `git log -1 -- experiments/iam-bootstrap/wifconfigtemplate-evidence/phase-1-handoff.md`. Pre-existing untracked experiment notes/settings were not staged or discarded. |

- API image: `quay.io/apahim/gecko:wif-p1-cc26254c92b6-api`.
- Immutable published/running manifest digest: `sha256:b1a477398519f3561aa080bd7deb3777c69fac32c1f74e2f524bec3fd6bb05cc`.
- Clean native build: host Go `1.27.4`, `GOWORK=off`, `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, readonly modules, trimpath; actual Go metadata reports source `cc26254...`, `vcs.modified=false`.
- Fast build command: `make local-image-api LOCAL_ARCH=amd64 LOCAL_IMAGE_REPOSITORY=quay.io/apahim/gecko LOCAL_IMAGE_TAG=wif-p1-cc26254c92b6`. The existing image is already published; future phases need new source-specific tags.
- `deploy/Containerfile.local` only copies the cross-compiled binary/app directory and trusted CA bundle into the normal non-root UBI micro runtime; it has no RUN instructions. Ordinary CI Dockerfiles are unchanged. Both API/controller local builds and runtime help smoke were tested; only the API artifact was published/deployed for Phase 1.
- The original emulated build was deliberately canceled and never published. Local dirty validation images are not deployment artifacts.

## Final personal dev state

- Region/project: `northamerica-northeast2` / `dev-reg-na-ne2-apahim1110`; management project `dev-mgt-na-ne2-apahim1110`. Use their configured **GKE DNS endpoints**; private IP access from the Mac timed out. No network configuration or new permissions were added.
- Regional context: `gke_dev-reg-na-ne2-apahim1110_northamerica-northeast2_dev-reg-na-ne2-apahim1110-gke`.
- Management context: `gke_dev-mgt-na-ne2-apahim1110_northamerica-northeast2_dev-mgt-na-ne2-apahim1110-gke`.
- All eight regional Gecko Deployments Ready. API is on the new digest; seven controllers remain on `f90f6dfeafaf56092debc81e61fd870454d5bc21`. ESP remains `2.57.1`.
- API Application is Synced/Healthy, pinned to fork chart source `cc26254...`. The regional root observed final dev commit `d7231a6a...`.
- `iamApplications.enabled:false` and `--enable-iam-application-writes=false` remain enforced. No live Version references were pinned; Version availability producers/consumer gates are Phase 2 work.
- Catalog: one intentionally retained `WIFConfigTemplate`, name `5.0.0`, UID `fb623828-d923-4540-82f1-df5c01503823`, resource version `4`, generation `3`, **verified=false**.
- Contract digest: `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`. Exact six component identities, six project roles/bindings and 142 permission entries match the starting baseline; no actual customer SAs/roles/grants were allocated.
- Template retention remains admission-enforced, with Argo `Prune=false,Delete=false`. Do not treat the retained template as a disposable fixture.
- API Pod after the deliberate graceful restart: `platform-api-server-6669fdd78f-xqdrs`, UID `9e2319cc-f5d8-4f68-acf4-de9a33a70a91`.
- Migration Job UID `e11cbeec-b922-465b-9bc2-73d997f21b93` stayed unchanged through catalog creation, toggles, validation and restart. `ApplyOutOfSyncOnly=true` prevents unnecessary replay of this unchanged Force/Replace Job.
- Existing Spanner database remains `projects/dev-reg-na-ne2-apahim1110/instances/dev-reg-na-ne2-apahim1110-spanner/databases/gecko`; KCC DB UID `99e4163c-79f9-4875-92b9-0e576b9a5927` is unchanged. No memory rollback, database reset or handwritten new resource table.
- Regional Clusters/NodePools: `0/0`; Versions/Channels: `21/4`. Final management HostedClusters/HostedControlPlanes/NodePools readback also found none. No new hosted/customer allocation or upgrade occurred.

## Validation and assurance limits

Passed: root module tests across Orlop/API/controllers; focused race suites; API lint,
format/whitespace checks; stable two-pass generation; Helm render/lint and pinned
contract integration; typed API validation/conversion/append-only record tests;
unstructured-backend deletion regression; native cross-compilation/image assembly;
public image access and clean-source provenance; live authenticated catalog reads,
anonymous denial, public write denial, malformed/immutable/delete rejection, no-op
PUT; GitOps false -> true -> false with unchanged IAM/UID; graceful API restart with
unchanged template UID/RV/generation/content and independent read-only Spanner row
confirmation.

The temporary true transition tested the API lifecycle, **not runtime permission
completeness**. The final flag is false. No live Cluster application assertions,
cloud IAM executor, initial hosted installation or upgrade gates are claimed.
Cluster assertion behavior was tested with local API integration fixtures, not live
hosted resources. Live PostgreSQL, Docker-engine and arm64 runtime tests were not
run; dev uses Spanner, Podman and amd64 nodes. The historical label-selector issue
was not retested or repaired as part of this phase.

## Dev-only Endpoints change and original Terraform work

- Added only authenticated GET collection/item WIF routes, retaining all existing
  routes and OAuth2 settings. Fresh identity-token requests proved the initial ESP
  404 was routing, not auth; after the fix, list/item reads return 200 authenticated
  and 401 anonymous, and public writes return 405.
- User first requested plan-only, then explicitly authorized apply provided only
  dev changed. The saved plan and live previous rollout were revalidated first.
- Exact applied address: `module.service.google_endpoints_service.hcp`, project
  `gcp-hcp-dev-svc-apahim1110`, service
  `hcp-api.endpoints.gcp-hcp-dev-svc-apahim1110.cloud.goog`.
- Apply result: **0 added / 1 changed / 0 destroyed**. Independent managed rollout
  readback: SUCCESS, 100% on config `2026-11-17r0`. Full refreshed non-targeted
  post-plan: exit 0, no changes. No non-dev apply or IAM/API-enablement change.
- State stayed at its authoritative original root:
  `/Users/asegundo/git/osd/gcp-hcp-infra/terraform/config/dev-all-in-one/apahim`.
  Post-apply state hash: `83d8674e0d9af29574eb4cd354a289c532e45917a9bebc7190b4db82bc3ebcb6`.
- **Original checkout dirty state:** the user's `terraform/modules/region/spanner.tf`,
  `variables.tf` edits and untracked PDF are preserved. A task-owned **uncommitted**
  mirror of the reviewed 23-line Endpoints template patch also remains there so
  that root describes the applied config. Its durable tracked version is commit
  `20491381...` in the selected deployment worktree. Do not reset it or stage the
  user's files; integrate the source deliberately when changing original branches.
- Raw Terraform plan/apply logs may contain sensitive state. They remain mode-0600
  under local-only mode-0700 directory
  `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-endpoints-plan-j_ulxnda/`.
  Never commit or print them wholesale, and do not replay the already-applied plan.
- All Python work must use a venv. Current local venv is
  `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-phase1-venv`,
  with `PyYAML==6.0.3`; tools may disappear from temporary storage, so reconstruct a
  venv rather than installing into system Python.

## Exact next action for Phase 2 — not execution approval

Refresh branches/dirty work and runtime state read-only; then agree the Phase 2
scope before code or deployment. Start with Gecko `controllers/versionsync`,
`controllers/util/catalog`, resolution/cached-success paths, control-plane upgrade
approval/dispatch and NodePool manifests/controllers. Preserve existing reference
pins during ordinary whole-spec release synchronization before ever setting a live
pin. Implement one current-Available predicate, numeric-base prerelease floor
matching, recovery on template/verified changes, minimal template-read/Version-status
RBAC, exact-contract Cluster prerequisites and explicit default-worker-SA propagation.

Keep the live assertion-write gate off until all compatible APIs/controllers and
dispatch prerequisites are deployed. Do not use missing IAM fields as an opt-out,
write live Version pins with old versionsync, use the old CLI IAM path, add migration
exemptions, broaden grants or create hosted test resources prematurely. Future phases
must again review -> commit -> build -> dev deploy -> test -> commit evidence/handoff.
