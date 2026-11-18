# Phase 4 read-only requirements — staged dev rollout

**Status: preparation only; live observations not yet recorded.** The user
approved the scoped personal-dev rollout after the code-only checkpoint.
This is not approval for new IAM grants, assertion writes, template verification,
an upgrade, customer resource creation or broad teardown.

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
