# GitOps release validation evidence

This directory holds sanitized results from **authorized** disposable GKE
validation runs (plan section B) and from teardown validation.

| File | What it records | Status |
| --- | --- | --- |
| `2026-11-13-dev-pair.md` | GCP-1286 core release validation on pair `rfl1286m1111`, us-central1 | Current. Pair destroyed after the run. |
| `2026-11-16-teardown-validation.md` | One end-to-end run of `teardown.sh` from PR #2855 on pair `rfl1440m1116` | Historical. That script was retired in GCP-1445 and `nuke.sh` is now the only teardown path. |
| `UPSTREAM-ISSUE-GCP-1441-route-late-CRD.md` | Upstream report for the Route-after-startup defect | Current. |

The offline baseline and procedure are in
[`../GITOPS_RELEASE_VALIDATION.md`](../GITOPS_RELEASE_VALIDATION.md). Artifact
review evidence is in [`../gcp-1286-evidence/`](../gcp-1286-evidence/).

> **Archive teardown-evidence correction:** The teardown-only record is for
> `rfl1440m1116` and collected no image identities. The release-validation
> record is for the different pair `rfl1286m1111`; its image evidence cannot
> establish the teardown pair's image set. The historical exemption rule below
> must not be used to transfer image proof across pairs: this teardown record
> makes no image-identity claim. Retain each run's own scope and evidence limits.

Rules for anything added here:

- Record the exact target identity, state/source revision, approved image
  digests, selected platform and observed `imageID` mapping. Redact project
  numbers only where the ticket already treats them as sensitive; never include
  credentials or Secret values.
- Distinguish core startup (manager + Argo CD reconciliation) from full workload
  adoption. A running Pod or a synced root does not prove every component.
- Record failures and upstream reports rather than deleting them; unresolved
  limits stay visible.
- Do not claim GKE support, certification, SLA or a Tech Preview tier. Those
  remain B1/B2/B3/B4 decisions.
- Keep runtime fixture paths and cleanup records only after B5/B6 authorization.
- Teardown-only records document cleanup of an already-validated pair. Name the
  pair, source revision, authorization basis and teardown procedure; the
  approved image set is carried by the validation record, not repeated here.
