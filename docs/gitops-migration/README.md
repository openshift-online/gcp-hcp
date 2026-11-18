# GitOps migration records (GCP-1282–GCP-1286)

**Status: historical project records, exported 2026-11-18.** This directory
indexes and preserves the GCP-1282–GCP-1286 records and their source evidence.
It is not current operating guidance and does not authorize a deployment,
cutover, rollout, support claim, or repeat run. Each linked record carries the
details and its own limits.

The archive and companion infra cleanup are separate changes. Merge this
destination first. The original records remain in infra until the cleanup
merges, and its retained permission guide becomes the shorter reusable contract.

Records preserve the full historical source text. Markdown adds relocation
links, historical framing and explicit archive-review clarifications; raw JSON,
YAML, text and signature evidence is byte-identical to the pinned source. See
[`source-map.json`](source-map.json) for per-file source revisions, SHA-256
hashes, and transformations.

## Story index

| Story | Record | Recorded scope | Open qualifiers |
| --- | --- | --- | --- |
| GCP-1282 | [Replacement guidance](../openshift-gitops-configuration-handoff.md) and [configuration validation record](../openshift-gitops-configuration-validation.md) (already in this repository) | Opt-in configuration-ownership validation on disposable MC/region, 2026-10-13. No default rollout; chart and Application inputs remain legacy. | The implementation contract and mixed operating doc remain in `gcp-hcp-infra` [`docs/ARGOCD_OPERATOR_CONFIGURATION.md`](https://github.com/openshift-online/gcp-hcp-infra/blob/d2fc1a1b35a84193d654d25fd47407b68c1bacd2/docs/ARGOCD_OPERATOR_CONFIGURATION.md). Healthy pods do not prove configuration preservation, login, notification delivery, or cloud identity. |
| GCP-1283 | [`gcp-1283/ARGOCD_IAP_VALIDATION.md`](gcp-1283/ARGOCD_IAP_VALIDATION.md) | Existing HCP IAP/Dex flow validated on disposable dev region and MC, 2026-10-21–23. Not release certification, a security exception, or rollout approval. | Engineering trial images, not release-certified artifacts. Pre-existing C4 header-trust/in-cluster-bypass criterion remains unresolved and was not waived. |
| GCP-1284 | [`gcp-1284/GCP-1284-EVIDENCE.md`](gcp-1284/GCP-1284-EVIDENCE.md) | Sanitized disposable notification validation, 2026-10-27–29. Jira retains the chronology and remains the detailed record. | Historical snapshots, not current health. Trial operator images; not a repeat authorization. Raw stop records remain engineer-retained outside Git. |
| GCP-1285 | [`gcp-1285/ARGOCD_CONTROLLER_PERMISSIONS.md`](gcp-1285/ARGOCD_CONTROLLER_PERMISSIONS.md), with [`mgt-matrix.yaml`](gcp-1285/argocd-controller-validation/mgt-matrix.yaml), [`reg-matrix.yaml`](gcp-1285/argocd-controller-validation/reg-matrix.yaml), and [`runtime.yaml`](gcp-1285/argocd-controller-validation/runtime.yaml) | Opt-in direct-permission reduction validated on disposable pair `rfl1285m1109`, `northamerica-northeast2`. Permission checks and identity attestations before and after controller recreation. | Not admission-enforced isolation. Named `bind`/`escalate`, wildcard cache reads including Secrets, writes to the controller's own CR/policy, and workload/admission/KCC-IAM paths retain indirect authority. The companion infra cleanup retains the reusable contract in `gcp-hcp-infra`. |
| GCP-1286 | [`gcp-1286/GITOPS_RELEASE_VALIDATION.md`](gcp-1286/GITOPS_RELEASE_VALIDATION.md), with [plan](gcp-1286/GCP-1286_PLAN.md), [decision request](gcp-1286/GCP-1286_DECISION_REQUEST.md), [runtime request](gcp-1286/GCP-1286_RUNTIME_REQUEST.md), [CSV RBAC review](gcp-1286/GCP-1286_CSV_RBAC_REVIEW.md), [run evidence](gcp-1286/gitops-release-validation/README.md), and [artifact evidence](gcp-1286/gcp-1286-evidence/) | One authorized amd64 run of OpenShift GitOps 1.22.2-7 on disposable pair `rfl1286m1111`, `us-central1`, 2026-11-13, then teardown; artifact and offline checks as recorded. | Actual GKE was 1.36.4, not the proposed 1.35.x; acceptance was retrospective. arm64 artifacts verified only, no arm64 runtime. Support approval pending under GCP-1444. Local-chart exception. Controller cluster-admin-equivalent residual under GCP-1433. No default rollout or cutover permission. |
| GCP-1226 | [GKE spike discovery record](../openshift-gitops-gke-spike.md) (unchanged) | Earlier September 2026 discovery/handoff, kept separate. | The MC and region used different trial builds; one identical supported release was not validated across both. |

## Critical qualifiers

These qualifiers must survive any summary of this directory.

1. **Tested platform was GKE 1.36.4, not 1.35.x.** The GCP-1286 plan and runtime
   request assumed a 1.35.x baseline; the pair actually ran
   `1.36.4-gke.2377000`. The run continued through the request's stop-and-ask
   condition, and acceptance on 2026-11-17 was retrospective, after run and
   teardown. No 1.35.8 parity is claimed.
2. **amd64 runtime only.** Artifact identity was verified for `linux/amd64` and
   `linux/arm64/v8`, but only amd64 ran. The arm64 runtime validation remains
   deferred and is still required.
3. **No support or certification.** `support.approval` is `pending` and
   `platforms.certified` is empty. The vendor scope is carried by
   [GCP-1444](https://redhat.atlassian.net/browse/GCP-1444); certification
   targets 1.23. A self-flagged engineering-channel assistant reply is not a
   human support decision.
4. **Local charts are an exception, not a signed release artifact.** The two
   local `0.1.0` charts have no vendor signature. The recorded
   `local_chart_exception` is scoped to pre-certification GKE validation only.
5. **Controller residual is cluster-admin-equivalent.** The scoped GCP-1286
   manager removes cluster-wide `escalate` and unqualified `bind`, but keeps
   `bind` on two named ClusterRoles and `serviceaccounts/token` create, so a
   manager compromise can still reach cluster-admin-equivalent in one step. The
   application controller keeps the legacy wildcard policy on the disposable
   pair. [GCP-1433](https://redhat.atlassian.net/browse/GCP-1433) tracks
   accepting a supplied reviewed controller role and removing interim parity.
   This pair is not controller least-privilege.
6. **C4 remains unresolved.** The pre-existing June 25 security-audit finding
   (IAP header trust / no assertion validation / direct-backend concern) is
   documented in the GCP-1283 record. Neither the migration nor these
   validations implement hardening, waive the criterion, or prove a
   signed-IAP/direct-backend negative.
7. **No default rollout or cutover.** GCP-1282 is opt-in and leaves defaults in
   legacy mode. GCP-1286 is a bounded validation whose pair was destroyed.
   Environment selection, cutover, and promotion remain under GCP-1289 and were
   not authorized.
8. **Older trials used engineering images.** GCP-1282, GCP-1283, and GCP-1284
   used developer/engineering images or local builds. They are not
   release-certified artifacts, and their results do not certify a new target.
9. **Historical source authorizations do not authorize a new run.** The
   GCP-1286 grant was bounded to `rfl1286m1111` and consumed by the 2026-11-13
   run and teardown. Nothing here authorizes another run, another teardown,
   merge of PR #166, arm64 runtime, or deployment.

## Evidence and classification notes

- **GCP-1286 evidence is byte-preserved.** `gcp-1286-evidence/SHA256SUMS`
  verifies all 13 listed members, including `.gitignore`,
  `fresh-copies/README.md`, and `sigstore/README.md`. The source map records
  the manifest's own hash separately. Those files were not edited.
  Their internal `docs/...` references describe the original
  `gcp-hcp-infra` layout; do not edit them without regenerating the checksum
  manifest.
- **The teardown record is auxiliary, retired evidence.**
  [`2026-11-16-teardown-validation.md`](gcp-1286/gitops-release-validation/2026-11-16-teardown-validation.md)
  documents the GCP-1440 `teardown.sh` run. GCP-1445 retired that script;
  `nuke.sh` is the current teardown path. It is retained for source completeness
  and is not GitOps release proof or a current procedure.
- **Unmerged implementation dependency.** The offline tooling and tests cited by
  GCP-1286 are on development branches. PR #166 at
  `90dfd92b5d35d8ff0a6ac3d114eabd59009ab3f4` contains the renderer and
  assembler; the later release matrix and verifier are retained at
  [`0ed658f` on the runtime-bundles branch](https://github.com/floresroger/gcp-hcp/tree/0ed658f60c16f19053d5c75556c5e614f57f288e/tools/gitops-bootstrap).
  Neither is on `gcp-hcp` main at `fc05a28311b77436058818d035eeeb3fb9056e8e`.
  The archived runbook distinguishes those tooling paths from infra-repository
  paths and the relocated evidence directory.
- **Sanitized records only.** This move copies the committed records. Raw logs,
  Terraform state, credentials and the engineer-retained GCP-1284 stop records
  are outside its scope.
- **GCP-1285 matrix coverage correction.** The byte-preserved MC and regional
  matrices omit rendered cert-manager startup-check hooks. See the
  [archive review notes](gcp-1285/README.md) before using it as an inventory or
  permission specification; this archive does not backfill runtime evidence.

## Source provenance

| Content | Source repository and revision |
| --- | --- |
| GCP-1283, GCP-1284, GCP-1285 records and controller matrices | `openshift-online/gcp-hcp-infra` at `d2fc1a1b35a84193d654d25fd47407b68c1bacd2` (`docs/ARGOCD_IAP_VALIDATION.md`, `tests/argocd-notifications/GCP-1284-EVIDENCE.md`, `docs/ARGOCD_CONTROLLER_PERMISSIONS.md`, `docs/argocd-controller-validation/*.yaml`) |
| GCP-1286 records, evidence, and run records | `openshift-online/gcp-hcp-infra` PR [#2887](https://github.com/openshift-online/gcp-hcp-infra/pull/2887), head `4fb403ea0346426943519c28a12881fb8d44105d`, base `5d655f04733c530ddb6e22e50d0c75e0ac6e9742`; **23 added docs files**. The two modified docs at PR head were not exported: `docs/BOOTSTRAP_FLOW.md` stays in infra, and this directory uses the merged `d2fc1a1` snapshot of `docs/ARGOCD_CONTROLLER_PERMISSIONS.md` for GCP-1285. |
| GCP-1282 guidance and validation | `openshift-online/gcp-hcp` at `fc05a28311b77436058818d035eeeb3fb9056e8e`; linked in place with navigation-only backlinks added; factual content unchanged |
| GCP-1226 spike | `openshift-online/gcp-hcp` at `fc05a28311b77436058818d035eeeb3fb9056e8e`; unchanged and linked in place |
| GCP-1282 implementation/mixed operating doc | `openshift-online/gcp-hcp-infra` at `d2fc1a1b35a84193d654d25fd47407b68c1bacd2`, `docs/ARGOCD_OPERATOR_CONFIGURATION.md`; retained in infra, not exported |

## Related records

- [GCP-1282 replacement guidance](../openshift-gitops-configuration-handoff.md)
- [GCP-1282 configuration validation](../openshift-gitops-configuration-validation.md)
- [GCP-1226 GKE spike](../openshift-gitops-gke-spike.md)
- [GCP-1286 upstream Route-ordering report](gcp-1286/gitops-release-validation/UPSTREAM-ISSUE-GCP-1441-route-late-CRD.md)
- [Per-file source map](source-map.json)
