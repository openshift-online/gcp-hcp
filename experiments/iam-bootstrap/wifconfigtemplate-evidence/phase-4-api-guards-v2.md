# Phase 4 fresh public JSONPatch guard rehearsal — passed

**2026-11-20. Both approved negative JSONPatch test-op guards passed against the
deployed public converting API. Final independent readback: 07:32:31 UTC.**
This resolves the parsing/precondition blocker for the exact deployed
`7319d8e...` artifact. **Phase 4 remains incomplete:** no primary IAM update,
new assertion, gate activation or actual blocked/released upgrade was attempted.

## Approval, reviewed harness and immutable provenance

- The user explicitly approved the described fresh rehearsal: read-only refresh,
  one wrong-UID negative PATCH, one revision-mismatch negative PATCH, unchanged-
  state readback and evidence. The replacement in both requests was the identical
  existing one-element application history, never an append or release request.
- New standalone [`api-guard-rehearsal-v2.py`](tools/api-guard-rehearsal-v2.py)
  and its [12 offline tests](tools/test_api_guard_rehearsal_v2.py) were reviewed
  and committed as **`0848b89` before live execution**. The old harness was not
  imported, run or modified. Working-file bytes were checked against the committed
  blob immediately before the independent preflight.
- Exact executed harness SHA-256:
  `5b86c5aa8648628f889fac1776c430c313cb56ac449e869ad702876e4123870e`.
  This digest is bound into the new journal; the harness was not edited after
  baseline or between cases.
- Architect review initially caught the possibility of counting a retained
  migration Job pod as an API pod by name prefix. Before commit/execution the
  filter was corrected to use the actual Deployment selector plus controlling
  ReplicaSet ownership. Regression covers a retained completed Job and refusal
  of a second running/terminating API pod. Final disposition: **approved** for
  baseline → wrong-uid → stale-rv → readback, at most two PATCHes.
- All 12 credential-free tests passed in the existing dedicated Python venv.
  Coverage includes exact error classification, durable intent before HTTP,
  unchanged request history, auth/save/transport failures, uncertain-outcome
  refusal, failed postcheck blocking the next case, source binding, exclusive
  nonblocking locking, protected-file checks, content type/host pinning, no
  proxy/redirect/retry, and fail-closed runtime prerequisites.

## Fresh state and transport boundaries

- Fresh mode-0700 directory:
  `/Users/asegundo/.local/state/gcp-hcp/wif-phase4-api-guards-jsonpatch-20261120`.
  Mode-0600 journal: `guard-state.json`; mode-0600 local lock. The directory/state
  were distinct from the retired rehearsal, and the journal was absent at preflight.
- Final journal phase **Validated**, exactly two cases, both Validated and
  `rejectedByTest=true`. Final SHA-256:
  **`54ec587d465eed4e5749dd23f8becd42e86018a61a06d9217412bb1866b8cc36`**.
  Retain it as completed provenance; **do not reset it or repeat either case**.
- Every PATCH has durable, fsynced intent before the HTTP call; each mode is
  attempted at most once. An uncertain response or failed postcheck stops the
  sequence, not a retry. Authentication acquisition precedes intent.
- Public HTTPS uses normal certificate validation, fixed hostname/path,
  `application/json-patch+json`, in-memory bearer tokens, proxies and redirects
  disabled, no automatic retries, and a bounded response read. Raw error bodies,
  tokens and headers are not logged or stored; only fixed outcomes/body digests.
- Before and after each case the harness checks explicit closed gates, exact
  API source/running digest and quiescent API pod, all eight Deployments Ready,
  selected Spanner backend, original Complete/Synced migration Job, Ready Spanner,
  retained Cluster/NodePool UID/generation/spec/receipt/upgrade state and Complete
  requirements, unchanged primary/retired journal hashes and no update sidecar.
  Normal status resource-version/refresh-time changes are not mistaken for spec
  drift; neither can turn an arbitrary response into guard success.

## Actual sequence — no retries

Independent broad preflight passed at **07:30:46 UTC**, including actual guest
5.0.2 Completed/Available=True/Failing=False/Progressing=False, original Ready
worker, original HC/HCP/NodePool, HyperShift pin/stamp, 19 retained pins, original
migration Job and Spanner, and unchanged protected journals.

| Step | Observed result |
| --- | --- |
| Baseline, 07:31:02 UTC | Public Cluster UID `6b152684-797c-4f17-a258-43538f8013c8`, generation 2, RV **523**, original one-element history/spec; compatible deployed source/digest, both gates false. |
| Wrong UID, validated 07:31:25 UTC | Exactly one PATCH: `test /metadata/uid` = all-zero wrong UID; `test /metadata/resourceVersion` = fresh `523`; `replace` application history with identical existing list. **HTTP 409**, exact Kubernetes-style Status Failure/code 409/message `JSON patch test failed`. Immediate public/private postchecks passed. |
| Revision mismatch, validated 07:31:47 UTC | Exactly one PATCH: correct UID test; `test /metadata/resourceVersion` = deliberately non-current literal **`0`**; identical-history replacement. **HTTP 409**, same exact Status failure contract. Immediate public/private postchecks passed. |
| Harness readback, 07:31:59 UTC | Both cases Validated; retained spec digest, UID, generation, history, private receipts and gates unchanged. Cluster RV still **523**. |
| Independent final readback, 07:32:31 UTC | Public/private state and actual guest healthy; retained specs/identities, receipts, pins, storage, migration Job and both gates preserved; Cluster RV **523**, NodePool RV **480**. |

Both error bodies have the same SHA-256 (the fixed message intentionally does
not echo a path/value):
`sha256:6d2a225f67f4be6505644f4b37d58664a01f6ada8e53654b10d8169b6eb9c8cf`.
Success requires HTTP **409** and the complete expected Status classification,
not a generic 4xx, echoed request, missing-path error, authorization error,
decoding error or arbitrary storage conflict. Known request order distinguishes
the two deliberate mismatches; identity stability is independently checked.

## Independently verified retained state

- API source **`7319d8ea790a18163279ac407e4f08c676609e88`**; running API imageID
  `quay.io/apahim/gecko@sha256:5f4a2ad3201aff2d0a6dd01591a67480000cc7609a31d660cde2b7831c328a2c`.
  API Application remains Synced/Healthy. No source/image/GitOps rollout here.
- All eight Deployment specs/UIDs remain unchanged and Ready. Actual guest
  5.0.2 completed history and original Node UID
  `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`/provider ID remain unchanged and healthy.
- Original Cluster generation 2 and NodePool generation 1, specs, one template
  application, Dispatched receipts, exact release/payload/contract and no requested
  upgrade. Both requirements Complete; 21 Versions/19 identical retained pins.
- Template verified=false and application writes=false. Public spec digest stays
  `sha256:d8d739a85da263b872e1d20c8189aa7a0a1440e213adebb63a9f9f7468669a26`.
- Migration Job UID `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, creation timestamp
  `2026-11-17T19:25:31Z`, image/spec and Complete/Synced state unchanged. No replay.
  SpannerDatabase/gecko UID `99e4163c-79f9-4875-92b9-0e576b9a5927`, spec and Ready
  state unchanged. Actual API still selects the original Spanner database.
- HyperShift Application spec/Synced/Healthy state, CRD source
  `914334ae61352fa5641e3f1d50250e87299e9eb3` and frozen stamp
  `3d8a55cb8c633329d2a64939e7e0ab20520107ee` unchanged.
- Primary journal hash remains
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`;
  no primary `.updates.json` exists. Retired v1 guard journal hash remains
  `af1f4abf8e822266f8a089bf03773afb8eb908083ca31f28f19e1a025d178588` and its
  old UnexpectedResponse result remains historical evidence, not rewritten as a pass.
- Supporting read-only snapshots are local-only, mode-0600 files under
  mode-0700 `.../opencode/wif-p4-fresh-guards-readback-e7fslyof`. No Secret
  objects or tokens were stored. This document is the durable credential-free record.

## Scope of proof and next approval boundary

This is live proof that **JSONPatch `test` operations reject the UID and revision
literal mismatches on the deployed public route**. The `0` revision is deliberately
non-current, not a previously committed RV. This is **not** a live concurrent
storage-CAS race, a successful assertion append, cloud IAM verification/update,
propagation/recovery proof, or upgrade acceptance. Postgres CAS remains a separately
documented pre-existing limitation; no backend code changed.

The protocol/negative-guard prerequisite is now satisfied for this artifact.
Keep both gates false and preserve all journals/pins/retained resources. The next
scope needs a freshly discovered real supported target and exact contract,
permission-retention review, maintenance timing and explicit IAM/gate/upgrade
authorization. Do not infer any of those approvals from this rehearsal. No IAM
update, assertion append, gate activation, upgrade, exclusion extension, customer
resource allocation, migration replay, Terraform action or teardown was executed.
