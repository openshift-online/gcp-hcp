# Phase 4 public JSONPatch fix — API-only dev rollout

**2026-11-20. API-only dev rollout passed; final independent readback
07:22:30 UTC. Phase 4 remains incomplete: neither live JSONPatch guard is proven.**
The user renewed authentication and explicitly approved the API rollout after
the [local source/build checkpoint](phase-4-public-jsonpatch-fix.md). This does
not authorize live guard PATCHes, IAM updates, assertion appends, gate activation,
an upgrade, a new maintenance window, or cleanup of the retained installation.

## Fresh preflight and exact footprint

- At **07:15:02 UTC**, authenticated regional and management reads succeeded.
  Cluster `wif3-first` retained UID `6b152684-797c-4f17-a258-43538f8013c8`,
  generation 2, RV 520; NodePool `workers` retained UID
  `a8768aef-c352-47bb-9f32-3638275f1e15`, generation 1, RV 477, one worker.
  Original Dispatched receipts, exact 5.0.2/candidate payload, and single `5.0.0`
  application remained. Requirements Complete; no Pending receipt or upgrade.
- Template verified=false and deployed API application writes=false. All eight
  API/controller Deployments Ready; 21 Versions and 19 pins. API/HC/NodePool and
  HyperShift Applications Synced/Healthy.
- Independently read guest at **07:15:47 UTC**: 5.0.2 history Completed,
  Available=True, Failing=False, Progressing=False; original Node
  `workers-7zbj4-94pth`, UID `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`, Ready=True
  with original provider ID. The first guest read lacked the documented isolated
  CLI exec-plugin PATH and failed; using the checksum-verified retained
  `wif-p3-cli-97f314dac0bb/gcphcpctl` fixed it. No credential extraction, guest
  login, global CLI replacement, kubeconfig or context modification was made.
- Authenticated public HTTPS GET succeeded with normal TLS, redirects/proxies
  disabled and token only in memory. Original UID/generation/release/history;
  public spec digest still
  `sha256:d8d739a85da263b872e1d20c8189aa7a0a1440e213adebb63a9f9f7468669a26`.
- Actual API Deployment selects `SPANNER_DATABASE=projects/dev-reg-na-ne2-apahim1110/instances/dev-reg-na-ne2-apahim1110-spanner/databases/gecko`.
  SpannerDatabase/gecko UID `99e4163c-79f9-4875-92b9-0e576b9a5927` is Ready.
  The pre-existing Postgres CAS limitation does not change; no backend fix or
  backend switch is part of this rollout.
- Completed migration Job remains Argo Synced, UID
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, creation
  `2026-11-17T19:25:31Z`, image `quay.io/apahim/gecko:wif-p2-cel-54709c4-api`.
  Its Force/Replace annotation is retained, not invoked. Stop if OutOfSync;
  never manually sync/replay/delete this Job.
- HyperShift source `914334ae61352fa5641e3f1d50250e87299e9eb3` and frozen
  `spec.source.kustomize.commonAnnotations` stamp
  `3d8a55cb8c633329d2a64939e7e0ab20520107ee` freshly confirmed intact.

Protected local read snapshots and render comparisons are in mode-0700 temp
directory `.../opencode/wif-p4-jsonpatch-rollout-s59oajjl`, individual snapshot
files mode 0600. No Secret objects or tokens were stored. These are local-only
supporting evidence, not durable backups; this document records the essentials.

## Reviewed publication and deployment plan

- Source `7319d8ea790a18163279ac407e4f08c676609e88`, preserving `367f546...`,
  fast-forward published only to `apahim/gecko` fork branch
  `fix/GCP-1452-cel-integer-decoding`. Explicit `origin` refspec; no upstream push
  or history rewrite. Remote readback confirms the exact SHA. No new code/chart
  change was needed; chart diff from deployed `d70d649...` is empty.
  Existing `origin.fetch` maps into `refs/remotes/upstream/*`, so local tracking
  status may appear up-to-date after this fork push; that is not evidence of an
  upstream publication. No remote configuration was changed. Continue using
  explicit remote/refspecs, never a bare push from this checkout.
- Image uniquely published as
  `quay.io/apahim/gecko:wif-p4-jsonpatch-7319d8e-api` after confirming the tag
  did not exist. Independently unauthenticated HTTPS-read registry manifest:
  **`sha256:5f4a2ad3201aff2d0a6dd01591a67480000cc7609a31d660cde2b7831c328a2c`**.
  Its content hash matches the registry header and push digest file; config
  digest matches local image ID
  `sha256:fa42bedd483ffbecdc79648aedc6ca7a2a40d4459ed5730d8e9b852bbdee6947`.
  Registry digest deliberately differs from the earlier local-only manifest.
  Independent `podman pull --platform linux/amd64` by that immutable registry
  digest succeeded and returned the same image/config ID.
- Only two infra files change in `/Users/asegundo/git/osd/gcp-hcp-infra-dev-main`:
  personal-dev `platform-api-server` override and generated Application. Only
  source SHA and image tag change (plus a comment). No shared values, controller,
  CRD, Spanner, IAM-policy spec or other environment change.
- Regeneration twice is stable. Root Helm render uses **live root Application
  values**: only the API child's source SHA/tag change. API Helm renders use
  **live API Application values/parameters**: 26 objects; 20 differ only in
  metadata version labels except intended API Deployment image. Entire migration
  Job render unchanged; canonical Job render SHA-256
  `b5383f2303787cc70ac320e4feb5186d7123140069c0a35eedc2bc42a84597f6`.
- Both Helm lints pass. Explicit `HELM_BIN` migration-image pin and template
  packaging integration tests pass with `-count=1`; whitespace check clean.
- Architect review: **approved API-only rollout, no blockers**, conditional on
  registry readability, exact fork/infra refspecs, last-minute Job/gate/readiness
  checks and independent postchecks. Keep replicaCount=1,
  maxSurge=1/maxUnavailable=0, ESP 2.57.1, Spanner enabled, prune=false,
  ApplyOutOfSyncOnly=true, both gates false. No controller rebuild/rollout.

## Rollout result and retained-state readback

- **07:18:30 UTC:** immediately before publication, Job still Synced/Complete
  with identical UID/timestamp/spec; all eight Deployments Ready; both gates
  false; Cluster/NodePool identities, generations, specs and receipts unchanged.
  Primary and retired guard-journal checksums still match; no update sidecar.
- Infra commit **`84cb52ffe9374d572214720df2f7378484e2c4d8`** fast-forward
  published explicitly to fork `origin/dev-apahim`, preserving prior
  `da44e854...`. Remote readback confirms the exact commit. No force push.
- Refreshed only the regional root Application's Argo cache annotation;
  ordinary automated reconciliation selected API source `7319d8e...`.
  At **07:19:03 UTC**, API rollout was Progressing/operation Running, while the
  migration Job remained Argo **Synced**. No manual sync/Force/Replace was sent.

- Bounded Kubernetes watches confirmed the desired Deployment image, successful
  rolling update, exact Argo source revision, and Application **Synced/Healthy**
  with operation Succeeded. New API pod `platform-api-server-7fb95859f7-pgd2l`,
  UID `af485c0b-bb1f-4de6-92dd-af6de22aef7a`, has API and ESP Ready with zero
  restarts. Running API imageID matches the registry digest above; ESP retains
  its original digest/2.57.1 image.
- The initial postcheck correctly stopped short of claiming old-pod removal:
  the old API pod still existed with deletionTimestamp during its ordinary
  graceful-drain interval (unchanged chart preStop delay 120 seconds,
  terminationGracePeriodSeconds=210). A bounded deletion watch was started; no manual
  deletion, force termination, re-sync or rollout retry was attempted.
- Independent readback at **07:20:31 UTC** confirmed original migration Job
  UID/timestamp/spec and Complete/Synced state; original Spanner UID/spec and
  Ready; both gates false; all eight Deployments Ready. API Deployment spec
  differs only at the intended API container image. All seven controller
  Deployment specs and pod UIDs/container image IDs/restart counts unchanged.
- Original Cluster/NodePool UID/generation/spec/receipt/upgrade state unchanged;
  21 Versions/19 original pins; both requirements Complete. Public Cluster and
  NodePool GETs succeed. Public Cluster spec digest remains exactly
  `sha256:d8d739a85da263b872e1d20c8189aa7a0a1440e213adebb63a9f9f7468669a26`.
- Original HC/management NodePool identities/generations/specs and completed
  release history unchanged; original HCP UID Available=True. Fresh actual guest
  read confirms 5.0.2 Completed/Available=True/Failing=False/Progressing=False
  and original worker UID/providerID Ready=True. HyperShift Application spec,
  CRD source pin/frozen stamp and Synced/Healthy state unchanged.
- Primary and retired guard-journal checksums remain the recorded originals;
  no primary update sidecar. A bounded startup log sample (23 API lines) had
  zero error-severity, panic, permission-denied or Spanner-failure matches.
  This is not sustained monitoring or an IAM functional acceptance test.

- Bounded deletion watch confirmed old API pod removal. **Final independent
  readback at 07:22:30 UTC passed after removal:** exactly one new API pod Ready
  with the expected running registry digest and zero restarts; Application
  Synced/Healthy at `7319d8e...`; all eight Deployments Ready; original Job and
  Spanner preserved; both gates false. Public Cluster/NodePool and actual guest
  reads still healthy. Cluster RV 521 / NodePool RV 478 reflect status refresh;
  generations, specs, receipts, history and exact template identities unchanged.
- **Observed infrastructure churn, not a controller rollout:** during final
  checking, version-sync pod `gecko-version-sync-5fdd8dd7cd-69sqw` (UID
  `2c91218e-4e6a-4dcd-a21f-a74717dae310`) was replaced by
  `gecko-version-sync-5fdd8dd7cd-kdn7w` (UID
  `7dfe0cfd-8c60-4304-93ca-45552e3831ea`). The old pod's event explicitly
  reports `cluster-autoscaler`, reason `ScaleDown`, message
  `deleting pod for node scale down` at 07:21:31Z. The replacement has the
  **same ReplicaSet UID and original p2 image digest**, Ready with zero restarts.
  Its Deployment spec/UID and Argo pin did not change. Six other controller
  pod UIDs stayed unchanged; all seven controller Deployment specs/images are
  unchanged. The first strict final pod-identity check stopped at this mismatch;
  it was investigated read-only and explicitly accounted for, not hidden or
  "repaired" with another rollout. The autoscaler was not modified.
- Final primary/retired guard-journal hashes still match, with no update sidecar.
  CLI source/artifact and HyperShift/CRD source checkouts remain untouched.

**Neither live JSONPatch guard has been tested after this rollout.** No guard PATCH or old harness run occurred;
no IAM update, new assertion, gate activation, upgrade, maintenance extension,
Terraform action, new customer resource, or teardown was executed.

## Next boundary

The deployed artifact now contains the reviewed public patch fix, but GET/health
checks are **not** live JSONPatch protocol/precondition proof. Separately review
and approve a fresh bounded guard harness/state, with classifier matching the new
409 `JSON patch test failed` contract; do not reuse the retired harness/journal.
Keep both gates false and refresh runtime again before any such test. Real IAM
update and distinct-contract blocked/released upgrade acceptance remain outside
this rollout, including explicit target/permission/maintenance approvals. The
pre-existing Postgres CAS limitation remains documented; this rollout retains
the independently observed Spanner backend.
