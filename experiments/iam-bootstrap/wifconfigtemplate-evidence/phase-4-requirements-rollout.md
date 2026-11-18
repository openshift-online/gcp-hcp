# Phase 4 read-only requirements — staged dev rollout

**Staged personal-dev rollout passed, final independent readback
2026-11-18T15:21:02Z. Phase 4 remains incomplete.** The user approved this
scoped rollout after the code-only checkpoint. This was not approval for new
IAM grants, assertion writes, template verification, an upgrade, customer
resource creation or broad teardown.

## Source and reviewed rollout footprint

- Gecko API/controller source `c7d7fdb809a54f21347ab3d3313911412f2e1ac1`;
  chart-only migration-image pin source
  `d70d649978bd4372b3ee1201d88b0538e1d59e87`, pushed to
  `apahim/gecko` branch `fix/GCP-1452-cel-integer-decoding`. The API and
  controller images rebuilt from this clean source must reach Quay and have
  registry-resolved digests before either GitOps pin is published.
- Selected infra checkout `/Users/asegundo/git/osd/gcp-hcp-infra-dev-main`,
  branch `chore/dev-apahim-main-overlay`. Stage **only** the personal
  `platform-api-server` Application first. Keep replicaCount=1,
  maxSurge=1/maxUnavailable=0, Spanner on, prune=false,
  `ApplyOutOfSyncOnly=true`, ESP image unchanged, template `5.0.0`
  verified=false and application writes=false.
- The `spanner.migration.image` override pins the existing migration Job to
  `quay.io/apahim/gecko:wif-p2-cel-54709c4-api` while the Deployment moves to
  its new API image. Chart defaults retain prior behavior. Helm integration
  tests and architect review passed, including rejection of partial/blank
  overrides. This change adds no DDL or new table.
- Independent local Helm render from the **live Argo Application values**:
  26 objects; completed `platform-api-server-migrate` Job exactly unchanged;
  20 changed objects contain only the
  `metadata.labels.app.kubernetes.io/version` update, except for the intended
  API Deployment image change. No other resource spec difference. Version
  labels are not selectors. The current Job is Synced/Complete, UID
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, created
  2026-11-17T19:25:31Z. Recheck before publishing.
- After independent API/Job/storage/public-readback checks, stage only the
  personal `gecko-hc` and `gecko-nodepool` Applications with the compatible
  controller image, preserving the other five controller deployments and the
  HyperShift CRD source pin/frozen annotation.
- CLI `c58b57d` retains JSON/YAML public `iamRequirements` readback but does
  not consume the view to authorize cloud writes. Two native darwin/arm64
  builds using the matching external Gecko workspace matched SHA-256
  `d6d08f993c5605fdd3e42e452119ea66ef04f77e55c6462205bd945e963c7269`.
  This isolated artifact does not replace the user's global CLI.

## Stop and restore boundaries

- Never manually Force/Replace-sync the migration Job or publish an API pin
  before its image is registry-readable. Confirm that Job remains Synced before
  root sync. If it is OutOfSync for any reason, **stop**, do not sync the app.
- If API rollout fails, preserve the existing pod and Spanner; only restore to
  a reviewed IAM-history-compatible API/controller pair by forward fix or
  independently evaluated image reversal. Do not replay migration, delete
  persistent data, or revert to a pre-IAM binary.
- If staged controller rollout fails, stop with gates closed; do not force a
  new dispatch. Record actual images, failures, preserved receipts and
  recovery plan. This preparation log does not claim deployment success.
- The primary owned IAM/network/worker, 19 pins, protected journal and
  HyperShift worker-email CRD fix/source pin must remain. The migration Job
  UID and Spanner database require independent post-sync readback.

## Actual image and GitOps results

- Clean Gecko `d70d649...` built component-specific linux/amd64 images from
  repository-root Containerfile/Dockerfile. Both were pushed to
  `quay.io/apahim/gecko` and independently pulled by immutable registry digest:
  - API `wif-p4-d70d649-api`:
    `sha256:02c7334ca0bd24f827072c18c337885f43aa9fb1144cd81a2a170be84a3033ed`.
  - Controllers `wif-p4-d70d649-controllers`:
    `sha256:4a5ec73161a03a9609ab8ceaa35a16ad8b58892f4a083dbb200f56b58d45226b`.
  These are **registry** digests, not the earlier local manifest digests.
- Dev infra first committed/pushed API-only
  `60d60bc9cba5e1e742af2bdbf5fcaf700d17214b` to `origin/dev-apahim`.
  Root Argo refresh reconciled the API Application to the exact Gecko source
  `d70d649...`; after its rolling update, the old API pod terminated and the
  new API pod reported the registry-resolved digest above with zero restarts.
  The Application was Synced/Healthy and the API Deployment Ready 1/1. The
  migration Job was Synced/Complete, UID
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, original creation timestamp
  `2026-11-17T19:25:31Z` and original p2 image: **no replay**.
- Only after API and Job checks, dev infra committed/pushed HC/NodePool pins as
  `da44e85474c8857d12c2ad6322efbdc66f59d100`. Their Argo Applications
  converged Synced/Healthy; both Deployments Ready 1/1, new pods running the
  registry-resolved controller digest above, zero restarts. Old HC/NodePool
  pods terminated. The remaining five controller Deployments kept their
  prior p2 images/digests and are Ready. Eight API/controller Deployments
  remained Ready.

## Independently observed retained installation

- Authenticated public CLI readback shows Cluster `wif3-first` UID
  `6b152684-797c-4f17-a258-43538f8013c8`, generation=2, and NodePool
  `workers` UID `a8768aef-c352-47bb-9f32-3638275f1e15`, generation=1.
  Each new `status.iamRequirements` view is **Complete**, binds its object and
  parent Cluster UIDs exactly, and reports a fresh `lastRefreshTime`
  `2026-11-18T15:18:18Z`. Each contains only the original Dispatched 5.0.2
  candidate payload and template `5.0.0` UID
  `fb623828-d923-4540-82f1-df5c01503823`, digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
  No Pending receipt or new application was created. The private receipts
  still match the same exact parent/object UIDs, payload and reference.
- Template verified=false; application writes remained disabled. Twenty-one
  Versions and 19 pins remain. Public Cluster history still has **one**
  assertion; public NodePool remains one worker and observed 5.0.2. Guest
  ClusterVersion remained 5.0.2 Completed/Available=True/Failing=False/
  Progressing=False. The actual original guest Node
  `workers-7zbj4-94pth` remained Ready=True with unchanged UID/provider ID.
- KCC `SpannerDatabase/gecko` in `gecko-system` reported Ready=True/UpToDate;
  the migration Job was not recreated. Both API and HyperShift Argo
  Applications remain Synced/Healthy. The HyperShift CRD-only fix source pin
  `914334ae...` and frozen annotation `3d8a55cb...` were preserved.
  A bounded 10-minute startup log sample contained 10 HC and nine NodePool
  lines, with zero lines matching errors, permission denials or status-update
  conflict patterns; this is not a sustained-failure alert test.
- After controller rollout, the isolated reviewed CLI ran standalone
  `iam verify` against the original protected primary journal (read-only).
  Independent cloud readback returned `cloudReadbackMatches=true`,
  `eligibleForNewUse=false`, the same operation ID
  `30882e34568dbf073aedbcf769ebd103` and unchanged template
  name/UID/digest. The journal was not opened for an update; its original
  SHA-256 `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`
  is unchanged.
- A subsequent public read showed **both** requirements still Complete with
  the same observed generations and refreshed `lastRefreshTime`
  `2026-11-18T15:23:19Z` (previously `15:18:18Z`), consistent with the
  bounded five-minute refresh. This is a bounded observation, not a claim
  that older controllers can never clobber the field later.

## Remaining Phase 4 work and caveats

- The CLI pure `RetentionPreview` is **not** an apply-ready plan; target-aware
  update execution/journaling, complete running/pending inventory CAS/recheck,
  multi-contract cleanup, and an actual blocked/released upgrade remain.
  A separate exact supported Cincinnati edge and live IAM/upgrade approval are
  still required. Do not interpret Complete as cloud IAM verification.
- Older compatible p2 controllers remain in service and can erase the new
  status field on whole-status writes. Missing status means Incomplete; watch
  for flapping before using this view as an IAM-update input. `lastRefreshTime`
  measures reconciliation, **not** transport-feedback age. The pinned p2
  migration image is intentional until a deliberate future migration refresh.
  No Terraform apply, Spanner replay, IAM mutation or upgrade was performed.
