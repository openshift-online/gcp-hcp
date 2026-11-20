# Phase 4 4.22.21 → 5.0.0 comparison bridge — dev-only rollout

**2026-11-20, final independent readback 09:32:51 UTC. Bridge-only rollout
passed. Phase 4 remains incomplete.** This rollout did not activate the
catalog, either gate, the NodePool-upgrade coordinator, IAM updates, an
assertion, a new installation or any upgrade. The previous two negative API
guards remain completed evidence and were **not rerun**.

## Approved footprint and provenance

- Gecko source `6c28cf4348a8ab4f524b91ece1cdbe29f1389c44`, fast-forward
  published to the `apahim/gecko` fork's
  `fix/GCP-1452-cel-integer-decoding` branch only, with exact remote readback.
  All earlier commits, including deployed `7319d8e`, remain reachable. The CLI
  source `c3daed09e64f85a6d7025a1e0ba5d9bbca4ef2a0` remains clean, local,
  unpushed and untouched, with previous local commits preserved. No CLI binary
  was rebuilt or replaced. The operational `a727640` native artifact remains
  SHA-256 `bba7d96f0fef9a79581d7a49b7354114b0e0f5dcdb842e063b6541acbbbeb13e`.
- Both previously recorded local images were published under new component-
  specific tags. `podman pull --platform linux/amd64` by registry digest
  independently returned the original configuration IDs; both images report
  `65532:65532` and source `6c28cf4`.

  | Changed component | Tag in `quay.io/apahim/gecko` | Registry digest | Image/config ID |
  | --- | --- | --- | --- |
  | API | `wif-p4-bridge-6c28cf4-api` | `sha256:460bd083bf1f25e9bbad81b0704051b288a587adb6e423cf96e1aa4145172130` | `d30c0d77de4246481c115763ca8473271f0b2af4ccb63b6c782bc26c8c3ba5be` |
  | Controllers | `wif-p4-bridge-6c28cf4-controllers` | `sha256:cf07c10b7b37ac443b7ddad98fc14b7b45976def45c275848a329255cf4a3876` | `187edd1d81e915e3eca8d99d94a6ee583140c8dc308ecbf3cc5fc90a2d9b18c5` |

- Infra fork branch `dev-apahim` advanced by **fast-forward only**, with
  explicit push refspecs and remote readback. `5912daba034750a5aa03cda7f139620e577b138e`
  recorded the six reviewed override/render changes. To honor API-before-
  controller staging, `6f9f977883d3efa1f29113c0784f2c18882bbd5a`
  temporarily left the two controller overrides at their previous revisions;
  after API convergence, final `0fe723c29f896f26a22405f5e8fde6ca99f503b9`
  restored exactly those reviewed controller updates. No history was rewritten.
  The final infra tree contains only the intended three personal-dev source/
  image overrides and generated Applications, with no catalog example enabled.

## Preflight and review

- Fresh private reads found exactly one Cluster (`wif3-first`, raw 5.0.2) and
  one NodePool (`workers`), no requested upgrade, eight Ready Deployments,
  both gates false, 21 Versions/19 pins, unchanged completed migration Job,
  Ready Spanner and all three journal hashes unchanged. Guest/public and
  management reads at the prior 09:08 UTC refresh had independently confirmed
  healthy retained state; immediately before the first rollout the Job/gates/
  readiness, retained identities and Complete requirements were rechecked.
- Architect review approved the narrowly scoped six-file diff and conditional
  live rollout. No Gecko chart/default/schema change is in this source revision;
  inactive examples remain inert. Renderer was run twice with the dedicated
  Python venv; exactly six intended files changed. `git diff --check`, renderer
  test (1), Helm tool tests (8) and Argo config tests (45, two documented
  optional skips) passed. The standard `make` wrapper requires `uv`, which was
  unavailable; equivalent repository Python entry points ran in the dedicated
  venv (PyYAML 6.0.3, jsonschema 4.26.0, lupa 2.6). Gecko/CLI full offline
  tests, lint, race, build reproducibility and original architect source review
  are recorded in the [code checkpoint](phase-4-42221-code-checkpoint.md);
  unchanged source was not retested during this deployment-only step.
- Planned root Helm output using live root parameters was compared with all
  three current live child Application specs. Apart from source SHA and image
  tag, each entire child spec was equal. Independent child Helm renders using
  actual live values/parameters produced 26 API objects and four objects each
  for the two controllers. Object identity and all fields were unchanged except
  version labels and intended Deployment images. The API migration Job document
  was exactly equal between old/new renders; no delete/prune operation was
  introduced. The existing completed Job image stayed pinned at
  `quay.io/apahim/gecko:wif-p2-cel-54709c4-api`.

## Observed deployment and independent final readback

- Only the regional **root** Application cache was refreshed for each stage;
  normal Argo automated reconciliation performed the deployment. An initial
  harmless cache refresh of the separate `gecko-cloud-resources` Application
  was observed and corrected by refreshing the actual
  `root-app-dev-reg-na-ne2-apahim1110-gke`; neither was manually synced. No
  Force/Replace, direct Deployment apply or Job replay was requested.
- API-only interim root revision `6f9f9778` converged first. API old pod
  removal was independently awaited after normal graceful drain. API App
  Synced/Healthy at `6c28cf4`; new API pod
  `platform-api-server-696c579b65-jg27c` UID
  `27d27f13-fcc0-4f7d-9227-da2016c5d4c5` is Ready with zero restarts,
  running exact registry digest above. ESP stayed at its prior digest. The
  migration Job stayed Argo Synced through this stage.
- Final root revision `0fe723c2` converged the two named controller Apps,
  both Synced/Healthy at `6c28cf4` with zero-restart Ready pods running the
  exact controller registry digest. The other five controller Deployment
  images/specs and observed pod identities remained unchanged. All eight
  Deployments Ready; `gecko-nodepool-upgrade` remains disabled and absent.
  The three new pods were the only intended controller/API rollouts.
- At **09:32:51 UTC**, independent private, public and actual guest readback
  passed. Original Cluster UID `6b152684-797c-4f17-a258-43538f8013c8` /
  generation 2 and `workers` UID `a8768aef-c352-47bb-9f32-3638275f1e15` /
  generation 1 retain the original exact spec digests, 5.0.2 Dispatched
  receipts, no requested upgrade and Complete IAM requirements. Public TLS GET
  returned HTTP 200 and the unchanged Cluster public spec digest
  `d8d739a85da263b872e1d20c8189aa7a0a1440e213adebb63a9f9f7468669a26`.
  The guest still reports 5.0.2 Completed, Available=True, Failing=False,
  Progressing=False, with the same original Ready worker UID
  `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab` and provider ID. HC/HCP and
  management NodePool remain healthy.
- Migration Job UID `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, creation
  `2026-11-17T19:25:31Z`, spec digest
  `273c6c47a0da1d9383178425ca07b88422a6c29bd3a8bd26cdd70d8e5fae93ae`
  remain unchanged and Complete/Synced. Spanner UID
  `99e4163c-79f9-4875-92b9-0e576b9a5927` and spec digest unchanged,
  Ready=True; actual API still selects Spanner. HyperShift Application remains
  Synced/Healthy at CRD-only source
  `914334ae61352fa5641e3f1d50250e87299e9eb3` with frozen annotation
  `3d8a55cb8c633329d2a64939e7e0ab20520107ee`.
- Both gates remain false; 21 Versions/19 existing pins unchanged. Primary
  journal SHA-256 `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`,
  retired guard SHA-256 `af1f4abf8e822266f8a089bf03773afb8eb908083ca31f28f19e1a025d178588`
  and completed v2 guard SHA-256
  `54ec587d465eed4e5749dd23f8becd42e86018a61a06d9217412bb1866b8cc36`
  are identical to their protected baselines. No primary update sidecar.
  Bounded post-start log samples (23 API, 6 control-plane-upgrade, 6 version-
  resolution lines) contained no error/panic/permission/Spanner matches; this
  is not sustained monitoring proof.

## Remaining boundaries

Phase 4 remains **INCOMPLETE**. The disabled NodePool-upgrade coordinator
requires a separately reviewed/approved Deployment/RBAC/NetworkPolicy
activation before worker-upgrade validation. Candidate-floor/catalog publication,
source pin, shared verified eligibility, assertion writes, a second installation
and its footprint/cost/time, IAM update and actual blocked/released upgrade
each require their own explicit live approvals. The retained cluster's prior
maintenance exclusion is expired; it already asserts the shared `5.0.0`
contract. Protect it before opening shared eligibility. Customer upgrade
requests bypass maintenance exclusions, so a target assertion can release
them immediately. Existing project-wide IAM roles and shared default worker SA
do not provide hard cross-cluster resource authorization isolation. No
standalone CPO credential-inspection prerequisite was introduced.

If an eventual forward fix or rollback is needed, preserve Spanner, the
completed Job and its migration-image pin, and never roll the API chart before
the compatible `d70d649` chart or infer that rollback reverses an upgrade.
