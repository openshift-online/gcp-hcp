# GCP-1284: sanitized disposable validation evidence

**Historical snapshots: October 27–29, 2026.** These are not current health
checks, production/release certification or authorization to repeat a test.
[GCP-1284](https://redhat.atlassian.net/browse/GCP-1284) retains the detailed
chronology. This source-controlled summary preserves non-secret facts outside
temporary directories; it is not a raw log archive or a claim that Jira is closed.

## Scope and identity

| Target | Disposable project | Fixture / GitHub context |
| --- | --- | --- |
| Region | `dev-reg-us-c1-rflores0922` | `gcp-1284-notifications-region` / `ArgoCD/gcp-1284-notifications-region` |
| MC | `dev-mgt-us-c1-rflores0922` | `gcp-1284-notifications-mc` / `ArgoCD/gcp-1284-notifications-mc` |

Repository: **`openshift-online/gcp-hcp-infra`**. Sender for the records below:
**`gcp-hcp-argocd[bot]`**, existing App ID `4096266`, installation `141370274`.
The private App's repository scope was not broadened to personal forks.
Target URLs were verified individually:

- Region: `https://argocd-dev-reg-us-c1-rflores0922.dev.gcp-hcp.devshift.net/applications`
- MC: `https://argocd-dev-mgt-us-c1-rflores0922.dev-reg-us-c1-rflores0922.dev.gcp-hcp.devshift.net/applications`

The operator-managed engineering runtime was Argo CD **v3.5.2**, with core image
`quay.io/redhat-user-workloads/rh-openshift-gitops-tenant/argocd-rhel9@sha256:ec4e9bd577efa235da258f1ea0ab95b7207ac52cca92be6b5437f87dc243eeb2`.
The region used the developer operator image; MC used a locally built Route-fix
operator image. These are trial identities, not signed-release/support claims.
Offline regressions retain the separate fleet **v3.4.3** dependency baseline.

Usable notification credentials followed the existing Secret Manager replication
and ESO Merge path into the ArgoCD-owned target. The initial dev placeholder was
intentional provisioning, not a demonstrated ESO merge defect. Git repository
reads used a separate fixture credential. No credential values are included here.

## Immutable fixture revisions

| Label | Full revision | Purpose |
| --- | --- | --- |
| A | `75305b9de691c80c10c705c70ec6a56b0a2c91a1` | Corrected initial success |
| B | `1f6138819c9365e8dec9789e673bf2bf6287586a` | Deliberate application failure |
| C | `c83bb64152474c543a5b4f529c34b6a36b9409f1` | Application recovery |
| D | `7e4dcb3080a1d4c9b7272f1850b5fab1af936628` | Missing-key-reference recovery |
| E | `6fcb07a8219e91bfba11b8a7f5bde5e46f20685b` | Delivery after target recreation |
| F | `52a39eb2d9d45c8526e6f7761b5153daa4d6686b` | Delivery after notification-Pod restart |

Each event used its target-specific context and fresh individual records, not a
preexisting combined green status. Probe containers exited **1** for B and **0**
for the successful phases, with zero probe container restarts. Terminal
source/requested/resolved/comparison SHAs and marker contents were correlated;
previous-revision status-ID sets were checked for unintended later writes.

| Phase | Region pending / result IDs | MC pending / result IDs |
| --- | --- | --- |
| A: pending → success | `57470441864` / `57470744138` | `57471765882` / `57472075395` |
| B: pending → failure | `57472894270` / `57473059002` | `57473517191` / `57473724717` |
| C: pending → success | `57473171527` / `57473478981` | `57473802897` / `57474135975` |
| D: success only after reference restoration | — / `57477008191` | — / `57478891130` |
| E: pending → success after target recreation | `57495793278` / `57496130605` | `57576818456` / `57577147495` |
| F: pending → success after Pod restart | `57505772999` / `57506079626` | `57577351045` / `57577700466` |

Region F records were created October 28 at **19:14:57 / 19:18:16 UTC**;
MC F records October 29 at **14:00:48 / 14:04:20 UTC**. Individual records can be
read with `GET /repos/openshift-online/gcp-hcp-infra/commits/<full-SHA>/statuses`;
filter the exact context, paginate, and compare ID/timestamp sets.

Primary evidence: [initial delivery](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19159296),
[B/C failure/recovery](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19159840),
[D credential-reference test](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19161114),
[region E/restart](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19168243),
[region F](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19168355),
[MC E/F and final guards](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19202377).

## Missing-target and restart observations

The supported notification toggle removed the operator target while a
disposable, explicitly reviewed source retained the **same** ESO Merge resource.
The successful attempts gated transitions on exact completed source SHAs, actual
absence, new target UID/same ArgoCD owner, post-creation refresh and fresh delivery.
They did not directly delete/fill Secret data or pause a shared operator.

| Observation (UTC) | Region, October 28 | MC, October 29 |
| --- | --- | --- |
| ESO refresh against absent target | 17:24:31 | 13:46:47 |
| Operator replacement target created | 17:27:55 | 13:48:42 |
| Post-creation ESO refresh | 17:27:58 | 13:48:49 |
| Enabled recovery observed within outage budget | 219 seconds | 144 seconds |

ESO v2.2.0 reported **Ready=True / SecretMissing** while the target was absent.
Actual absence and timestamps—not Ready alone—established that phase. The same
ArgoCD owner and ESO UID/spec were retained; new target UIDs were expected for
recreation and stayed stable across the post-creation refresh.

Each subsequent explicit restart deleted **one notification-controller Pod with
an API UID precondition**. Its replacement retained the ReplicaSet/image and
NC/ConfigMap/Secret identities/content/owners; fresh F delivery then proved use
of the persisted configuration. No additional restart was needed for region's
later delivery-only continuation. This is not all-operator restart certification.

## Preserved failures and scope exceptions

- The first October 27 region A request omitted operation syncOptions, so namespace
  creation failed before Pod creation. Its pending/failure IDs `57421850475` and
  `57421851289` were **excluded** from the corrected success proof. It was not the
  planned negative case; the negative Sync wave was not the namespace cause.
- The original region credential observer timed out on an unnecessarily strict
  freshness assertion. Raw failure evidence was retained; separate read-only
  Pod/log/controller-order/GitHub timestamp correlation qualified the observation.
  It was not re-labelled an automated pass. MC used the corrected observer.
- Earlier shared MC branch fault/restore commits induced HyperShift operator and
  external-dns rollouts through `${ARGOCD_APP_REVISION}` Pod annotations, despite
  unchanged images. No explicit restart command was issued, but the commits did
  induce those out-of-scope rollouts. Earlier unchanged-Pod statements covered
  Argo CD operands, **not all MC workloads**. This was disclosed, not hidden.
- A region lifecycle attempt timed out before the disable completed. Its rollback
  observer prematurely captured the original Secret; the later UID assertion
  correctly stopped it. One replacement—not two—was established. Exact completed
  restore-SHA gating and independent recovery budgets corrected the procedure.
- The first MC missing-target observer hit a bookkeeping-file race after actual
  absence. Enabled state and normal routing were restored; no E/F sync or explicit
  Pod restart followed that failed attempt. The fix persisted the clock before
  gate completion/refresh. Expired gcloud login then blocked the corrective retry
  until reauthentication; it did not authorize credential/permission workarounds.
- Region's first F request failed a stale resource-version test before mutation.
  A broader Pod guard then stopped its refreshed request after GKE balloon-Pod
  replacements. F ran only after a separately approved delivery-only continuation
  with fresh guards. The old guard failure remains a failure.
- Provider-system Pod churn was recorded separately; no claim is made that every
  GKE Pod UID stayed unchanged. Corrected isolation retained all HCP/Argo CD/ESO/
  Config Sync/operator protection and every unrelated workload-template guard.

See [region stopped attempt](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19164632)
and [MC stopped/auth-blocked attempt](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19199347).
Original raw stop records remain in the engineer-retained trial evidence; this
summary does not replace or claim to archive those raw records. Preserve them and
referenced repositories/branches through handoff or approve a separate archive.

## Caveat dispositions

**Historical defaults.** Recreation left out 8 operator-seeded templates and
7 triggers that the chart did not declare. Required GitHub inputs remained exact;
full equality to the *older* historical snapshot was not claimed. The October 29
read-only audit covered 36 Applications and two AppProjects: no consumers of those
removed triggers, no team-template dependency, no global/default subscriptions or
source-namespace self-service. Team triggers call only `github-status-pending`,
`github-status-success`, `github-status-failure`. The preserved team `on-deployed`
does not call the seed `app-deployed` template.

The recommended scoped disposition is to retain declared inputs and leave unused
seed entries absent, not introduce another writer. This is not an all-fleet
consumer audit. [Caveat audit](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19204310).

**Deploy-key permissions.** Christoph confirmed through Roger that the dedicated
fixture deploy key is **read-only**. This is repository-admin attestation, not an
independent API result: the agent's key-list request returned 404 and no key
ID/fingerprint/screenshot was supplied. Successful Git reads alone were not used
as permission proof. No write probe or permission expansion occurred.
[Confirmation](https://redhat.atlassian.net/browse/GCP-1284?focusedCommentId=19205902).

## Final recorded state and limits

At October 29 **14:07:23 UTC**, all **34 original Applications plus both Manual
fixtures were Healthy/Synced**, both fixtures pinned F, and normal source routing
and original ESO refresh annotations were restored. Shared private heads stayed
frozen at region `d402d1a0fe15897369e41cc6d0e7155d1bf41c3b` and MC
`12de7ecc711067d30beb2e5149320e87ce31a4bd`; HyperShift stayed on the latter without
another induced rollout. Public restore trees matched their pre-attempt trees
(region `70e0d50590a96ee0f12c4ef4a4fa1b514f4e13a4`, MC
`534fabe3098e3a297b4743047000490f0eb20a78`). Region received no MC-test mutations;
earlier status histories were preserved by later phases.

Positive evidence covers the scoped pending/success/failure, missing-reference
recovery, absent-target/repopulation and notification-controller restart-delivery
paths on both disposable targets. It does **not** prove virgin bootstrap,
Promoter/hydration rollout, every HTTP/credential failure, all-controller recovery
or production readiness. No CI gate/monitoring, fleet pin bump, broader rollout,
shared-key revocation or automatic cleanup is implied. Acceptance review and any
fixture/credential/branch cleanup remain explicit decisions, not test side effects.
