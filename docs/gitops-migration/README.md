# GitOps migration records (GCP-1282–GCP-1288)

**Status: historical project records, exported 2026-11-18, with one
repository-authored record added 2026-11-20.** This directory indexes and
preserves the GCP-1282–GCP-1288 records and their source evidence. It is not
current operating guidance and does not authorize a deployment, cutover,
rollout, support claim, or repeat run. Each linked record carries the details
and its own limits.

Project history and validation evidence live in this archive. Reusable
permission contracts and operating guidance belong in `gcp-hcp-infra`. The
[companion infra change](https://github.com/openshift-online/gcp-hcp-infra/pull/2904)
records that separation.

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
| GCP-1288 | [`gcp-1288/GCP-1288_LIFECYCLE_REHEARSAL.md`](gcp-1288/GCP-1288_LIFECYCLE_REHEARSAL.md) | Two bounded disposable dev rehearsals, 2026-11-20: region and MC legacy→operator adoption, one inert sentinel or self-heal probe each, one fresh operator-managed organizational-login window per target that the owner attested as passed, and the mandatory retain-and-converge rollback after each window. Both rollbacks were measured prospectively from the recorded decision (MC 19m 38.6s, region 18m 42.33s); final state is the healthy legacy baseline on GKE 1.36.4. | Login is an owner attestation only — no session payload, subject or issuer was captured — and default-reader behavior was not exercised; historical GCP-1283 on `rflores0922` is not a substitute. The owner accepted a 30-minute rehearsal objective on 2026-11-20, after these measured runs; it is a rehearsal objective only, not a production SLO or tenant RTO. No notification-delivery, tenant-survival, support, cleanup-completion, or rollout claim; the authorized 2026-11-20 teardown ran to its recorded scope but leaves recoverable deletions, unproven absence checks, a soft-delete window and one owner-retained read-only deploy key. |
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
- **GCP-1288 is authored in this repository, not relocated.** It was added
  2026-11-20 for a rehearsal that ran after the 2026-11-18 export. It is
  therefore not one of the 29 source-mapped files, and
  [`source-map.json`](source-map.json) is intentionally unchanged: that map
  describes only the 2026-11-18 relocation of the GCP-1282–GCP-1286 records.
  Its reusable operating protocol stays in `gcp-hcp-infra`
  [`docs/ARGOCD_OPERATOR_CONFIGURATION.md`](https://github.com/openshift-online/gcp-hcp-infra/blob/main/docs/ARGOCD_OPERATOR_CONFIGURATION.md).
- **GCP-1285 matrix coverage correction.** The byte-preserved MC and regional
  matrices omit rendered cert-manager startup-check hooks. See the
  [archive review notes](gcp-1285/README.md) before using it as an inventory or
  permission specification; this archive does not backfill runtime evidence.
- **Signature-tag correction.** The tag explanation in the byte-preserved
  Sigstore README applies to the GitOps bundle, manager, Argo CD and Dex.
  Redis instead records tags `1`, `1-1791836411`, `1791836439`, `9.8`,
  `9.8-1791836411` and `latest` in
  [`sigstore/results.json`](gcp-1286/gcp-1286-evidence/sigstore/results.json).
  Its six registry signatures bind the same approved Redis digest; the other
  four artifacts each have four. The historical counts and bytes are unchanged.

### Signing-key identity correction

The suffix-based explanation in the byte-preserved [Sigstore README](gcp-1286/gcp-1286-evidence/sigstore/README.md)
and historical [decision request](gcp-1286/GCP-1286_DECISION_REQUEST.md) is incorrect:
an SPKI SHA-256 suffix does not establish an OpenPGP key ID or vendor identity.
Red Hat publishes release key 3 as a PEM RSA public key, with the **full SHA-256
fingerprint** on its [product signing keys page](https://access.redhat.com/security/team/key).

On 2026-11-19, the [official key download](https://security.access.redhat.com/data/63405576.txt)
was parsed as a 4096-bit RSA key with public exponent 65537. Re-encoding it as
DER SubjectPublicKeyInfo and hashing with SHA-256 produced
`ba9d26350d78893a20f337e2153a113108bfcf064085afaee60d446e63405576`,
matching the published full fingerprint and **all 22** signer fingerprints in
[the retained results](gcp-1286/gcp-1286-evidence/sigstore/results.json).
The downloaded file's SHA-256 was
`2d43fc0ec7f773a75b5d4ae70c1c0b93c6ddc4a8f2ef9fd81b2589076032e80c`.
This full-fingerprint match, rather than a suffix comparison, associates the
archived signer fingerprint with Red Hat's published release key 3.

The archive retains signer hashes, not the verification public-key bytes or raw
Rekor bodies. A direct RSA modulus/exponent comparison with the historical
verification key is therefore not available from these records. This check
confirms the fingerprint association; it does not independently reverify the
historical signatures or authenticate their original extraction. The original
evidence and checksum manifest remain unchanged.

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
