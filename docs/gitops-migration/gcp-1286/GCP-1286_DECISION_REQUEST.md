# GCP-1286 decision request (B2 accepted; B3 internal; B4 decided; B1b alongside)

> **Historical record (source snapshot).** This decision request is an appendix
> to the GCP-1286 record. Its answers were scoped to the named disposable
> validation; they are not current approvals or authorization to repeat a run.

Both blocking questions answered via GitOps engineering channel; B2 has written acceptance from chart owner (confirmed method, accepted chart set for validation). Each answer records what it settles and does not. `support.approval` stays `pending`, `certified: []` empty — no support statement.

What changed since the first draft of this document:

- **B2 is accepted for this validation.** A GitOps engineer confirmed on
  2026-11-13 that the packaging approach matches the xKS path, republished both
  charts with the four adaptations, and then said the chart set itself is
  acceptable. That closes B2 for this run only.
- **B3 is routed, not decided.** GitOps engineering has nothing further to ask.
  The remaining question is what our own compliance bar is, which belongs to
  Product Security and ultimately to us. No contact was supplied.
- **B1a is settled.** The same guidance says 1.22.2-7 is viable for our own
  validation on xKS, with certification targeted at 1.23. We proceed and claim
  nothing certified. The reply also retracted its own version and ticket
  claims; the corrections are in the B1 section.
- **B1b is split out and does not block.** Three of its four parts are answered;
  a named escalation contact and the support tier wording are still ours to
  close.
- **B4 is internal and now decided as a scoped identity.** The operator CSV
  permission set is ours to own. We use the operator's supported custom-role
  mode so the manager no longer holds unqualified `bind` or `escalate`. The
  review and design are recorded in
  [`GCP-1286_CSV_RBAC_REVIEW.md`](GCP-1286_CSV_RBAC_REVIEW.md) §8.1; nothing is
  being asked of anyone outside the team.

Scope: **bounded validation on GKE**, amd64. No support claim; nothing deployed/merged/activated by answers here.

Context: candidate is OpenShift GitOps **1.22.2-7** (bundle
`sha256:2ba58c50f771…`, CSV `openshift-gitops-operator.v1.22.2`), selected by
Roger. Installation would use the locally maintained Helm charts in
gcp-hcp#166, not a vendor-published chart.

## B1 — platform scope (B1a settled, B1b mostly answered)

This gate turned out to be two questions, and only one of them is a
prerequisite.

### B1a — may we validate on this platform (answered, proceed)

**Basis:** guidance received 2026-11-11 in the GitOps engineering channel, from
the team's AI assistant (self-labelled "AI generated, check for accuracy"). It
states that 1.22.2-7 is technically viable for validation on xKS, that no formal
xKS certification exists for 1.22.x, that certification and a support model are
targeted at 1.23, that there is no Autopilot-specific testing, and that the
correct framing is "validated internally by GCP-HCP" rather than certified.

**Effect:** we proceed. This is permission to run our own bounded validation
with our own risk, not a support statement, and nothing in the matrix changes:
`spec.support.approval` stays `pending`, `spec.platforms.certified` stays `[]`.

**Three specifics from that reply were withdrawn by its author on 2026-11-11**
after being checked against the artifacts:

- The certification tracker is **GITOPS-11013** (In Progress, Blocker,
  "Shipping in OpenShift GitOps 1.23"), with delivery in **GITOPS-10648**.
  **GITOPS-9841** is Closed, fixVersion `1.22.0`, resolved 2026-10-07, and covers
  the functional gaps that made the operator deployable off-OpenShift.
- The "OCP 4.15–4.24" range is withdrawn: the 1.22 release notes do not list an
  OCP range the way 1.21 does.
- "Argo CD 3.7.1" is withdrawn. Its first replacement, release notes "CLI 3.5.3
  GA", also did not match: the bundle CSV we install describes the operand as
  **Argo CD v3.5.5**, and `3.5.3` appears nowhere in that CSV. On 2026-11-11 the
  reply accepted the CSV as authoritative and will cite v3.5.5 going forward,
  which closes the point. We pin neither string; the installed operand is the
  approved digest.

Retraction is kept visible. Every claim from this channel is checked against artifacts before recording, and answers are guidance not decisions.

**Platform observed (2026-11-11, `gcloud container clusters list`):**

| Cluster | Project | Location | Version | Channel | Mode |
| --- | --- | --- | --- | --- | --- |
| `int-mgt-us-c1-yjiv-gke` | `int-mgt-us-c1-yjiv` | us-central1 | `1.35.8-gke.1796000` | REGULAR | Autopilot |
| `int-reg-us-c1-nkcw-gke` | `int-reg-us-c1-nkcw` | us-central1 | `1.35.8-gke.1796000` | REGULAR | Autopilot |
| `stg-mgt-us-w1-zkpf-gke` | `stg-mgt-us-w1-zkpf` | us-west1 | `1.35.8-gke.1796000` | REGULAR | Autopilot |
| `stg-reg-us-w1-cvxu-gke` | `stg-reg-us-w1-cvxu` | us-west1 | `1.35.8-gke.1796000` | REGULAR | Autopilot |
| `gcp-hcp-int-global-gke` | `gcp-hcp-int-global` | us-central1 | `1.35.8-gke.1796000` | REGULAR | Autopilot |
| `gcp-hcp-stg-global-gke` | `gcp-hcp-stg-global` | us-west1 | `1.35.8-gke.1796000` | REGULAR | Autopilot |

Master and node versions are identical on all six. The target is the platform
HCP already runs, not a separately chosen version.

### B1b — support tier, escalation and scope (three of four answered)

**Received 2026-11-11, from the same channel:**

- **Escalation:** the GitOps team has no formal contacts configured.
  `#forum-openshift-gitops` is the public channel; the suggestion is to establish
  a direct line with the GITOPS-11013 leads before stage, so there is a known
  person rather than a cold channel post.
- **arm64:** not in xKS certification scope in GITOPS-11013, which lists QE on
  EKS/AKS/GKE without calling out architecture; existing GKE CI is amd64 only.
  Parking arm64 runtime (comment 19502521) is consistent with that.
- **Patch floor:** none specified for xKS. Treat the upstream Argo CD
  compatibility matrix as the closest reference and record the gap as
  "no floor specified by Red Hat".

**Still open, ours to close:** named escalation contact (channel is not a person) and support-tier wording. Neither blocks section B nor can the channel supply it.

**Why non-blocking:** we own/fund this validation. B1b matters for downstream messaging, not for running it; ask in parallel with disposable pair.

## B2 — installation and packaging (accepted 2026-11-13)

**Channel answer received 2026-11-11:** the local chart is acceptable for validation.
No official vendor chart exists for 1.22.x, and the channel said none would ship
before 1.23. A developer build is published in `quay.io/anjoseph` (tags through
1.22.3). It is unsigned and is not a pin source; the supported
`registry.redhat.io` publication
is timed with 1.23. Adoption is queued as Appendix A in
[`GCP-1286_PLAN.md`](GCP-1286_PLAN.md). The guidance says our approach matches
the xKS delivery path in
GITOPS-10648: strip `v1alpha1` and `spec.conversion` from `argocds.argoproj.io`,
set `ENABLE_CONVERSION_WEBHOOK=false`, install with Helm. With the webhook
pinned off and `v1alpha1` gone there is nothing to convert, no webhook to serve,
and no certificate to manage; cert-manager is not needed unless the webhook is
re-enabled.

**The one actionable item is now verified.** The reply asked us to confirm
`spec.conversion` is fully absent from the installed CRD, not merely that the
webhook config inside it is disabled. Checked against the packaged archive
rather than the chart source, because the CRD file is generated at package time
and the archive is what section B would install:

```
$ CRD=gitops-operator-crds/crds/argocds.argoproj.io.yaml
$ tar xOf charts/gitops-operator-crds-0.1.0.tgz "$CRD" | yq '.spec | keys'
- group
- names
- scope
- versions
$ tar xOf charts/gitops-operator-crds-0.1.0.tgz "$CRD" | yq '.spec | has("conversion")'
false
$ tar xOf charts/gitops-operator-crds-0.1.0.tgz "$CRD" \
    | yq '.spec.versions[] | [.name, .served, .storage] | @csv'
v1beta1,true,true
```

`conversion` is not a key at all, and `versions` is exactly one entry,
`v1beta1`, served and storage. That matters beyond hygiene: the upstream CRD
still carries a conversion webhook whose service namespace is hardcoded to
`openshift-gitops-operator` (GITOPS-11550, Backlog, Major), which breaks
validation when the operator lives elsewhere. Disabling the webhook does not
remove that stanza, so stripping it is the fix. Three existing assertions fail
if either the version list or the absent `conversion` key changes:
`run-step2-tests.sh:134` (source chart), `run-step2-tests.sh:333` (rendered
output for every child CRD) and `run-release-verification-tests.sh:395`
(packaged CRD). The post-install `jsonpath='{.spec.conversion}'` check from the
guidance is added to the section B runbook as a runtime confirmation.

**What was answered on 2026-11-13:** Anand Singh replied on Slack (relayed by
Roger), confirmed he had fixed both charts, and gave the exact generation
recipe: `kubectl-catalog` from
`quay.io/redhat-user-workloads/rh-openshift-gitops-tenant/catalog:v4.22`, `yq`
deletion of `spec.conversion` and `v1alpha1` from `argocds`,
`ENABLE_CONVERSION_WEBHOOK: "false"`, removal of
`gitopsservices.pipelines.openshift.io`, then package and push. That is the same
four adaptations this bundle makes, described from the author's side. It also
answers the open question about CRD backfill by republishing `1.22.2` with CRDs,
and it answers the CRD-set question in practice: he trims after generation
rather than exposing a component switch.

Verified against the republished tags, not the reply:

| Tag | OCI manifest | 27 CRDs | `argocds` | `gitopsservices` | Webhook | Manager image |
| --- | --- | --- | --- | --- | --- | --- |
| `1.22.2` | `sha256:85ab6bee…baa59e4` | yes | `v1beta1` only, no `conversion` | absent | `"false"` | `sha256:f5773d35…` = 1.22.2-7 |
| `1.22.3` | `sha256:457391a3…185ed2e` | yes | `v1beta1` only, no `conversion` | absent | `"false"` | `sha256:31114e72…` |

Both were created 2026-11-13, so the digests recorded on 2026-11-12 are stale
and must not be quoted.

**What that closes:** a GitOps engineer confirmed the method in writing and then
accepted this bundle's chart set for the pre-certification validation. B2 is
closed for this run. The acceptance is scoped to validation: it does not cover
support, certification, a vendor signature, or a later production install. The
trim to 8 CRDs, the local `0.1.0` label, and running Helm outside OLM on GKE
1.35.x are now recorded as accepted for this test rather than as our unilateral
choice. The executed run came up on 1.36.4; that platform was accepted
separately on 2026-11-17, because Autopilot upgrades automatically. `installation.approval` in `release-matrix.yaml` is set to `approved`
with the approver and scope recorded; `support.approval` stays `pending` and
`certified` stays `[]`. No other outcome changes the candidate or the plan.

**Independent review, still not sign-off (2026-11-11):** the channel's AI
assistant read the public PR and reviewed the build script, values, deployment
and RBAC. It found no packaging issue: bundle files are checked against
`upstream.sha256` before use, the vendored CSV is `cmp`-checked, the CRD change
is the single documented `yq` deletion, all images are digest-pinned, and the
render-time `fail` guard on `ENABLE_CONVERSION_WEBHOOK` is called out as the
strongest part. Two cosmetic items were raised and are deliberately kept:
declared port 9443 and the `optional: true` metrics-certs mount both come from
the CSV's deployment spec, and removing them would add a second declared
deviation to track. The reply states plainly that it is a bot and its review is
not the confirmation above.

**Where the confirmation comes from:** Jira shows Anand Singh (Anand Francis
Joseph on comments) as assignee and reporter of GITOPS-10648, and assignee of
GITOPS-11013 with Sho Weimer as reporter. His own comment on GITOPS-10648
describes generating the charts with `kubectl-catalog`, which is the approach we
followed, and the 2026-11-13 Slack reply is the written confirmation of that
method and of the chart set. Contact is through the GITOPS-10648 thread;
`#forum-openshift-gitops` remains the fallback if the thread does not respond.
The visible 404 was ours, not the PR's: #166 is an open draft PR in
`openshift-online/gcp-hcp` and is publicly readable, so the chart stays
reviewable there.

**What the charts are:** local packaging, authored from the official `1.22.2-7`
bundle, not a vendor-published or vendor-signed chart. Provenance is recorded in
`tools/gitops-bootstrap/chart-src/PROVENANCE.md`: exact bundle digest, per-file
SHA-256 of every bundle file consumed, and catalog cross-checks for all 15
related images.

**Adaptation already verified:** 7 of 8 CRDs are byte-identical to the official
bundle files; the ArgoCD CRD `v1beta1` block matches after canonicalization; the
`ControllerConfiguration` spec matches with only annotations added.

## B3 — trust requirement (routed to Product Security; the bar is ours)

> **Archive verification-scope clarification:** The preserved reply below
> overstates the combined coverage. Release key 2 was verified for the bundle
> and all 15 related-image index manifests (16 entries in
> [`all-image-verification.json`](gcp-1286-evidence/all-image-verification.json)).
> Release-key-3 Sigstore verification covers only the bundle and four required
> images: manager, Argo CD, Dex and Redis (five entries in
> [`sigstore/results.json`](gcp-1286-evidence/sigstore/results.json)). It does
> not establish key-3 coverage of the optional images or conditional HA proxy.

**Answer received 2026-11-11:** GitOps engineering has nothing further to ask.
Verifying both signing mechanisms across the bundle and all 15 images is
described as comprehensive; the images are signed and we have confirmed the
signatures verify. What a non-OpenShift deployment needs in order to be
considered compliant is a Product Security and/or governance question, not a
GitOps one. On OpenShift 4.21+ release-key-3 enforcement is automatic through
`ClusterImagePolicy` in CRI-O; on GKE there is no such mechanism, so the
enforcement is whatever policy we implement.

**No contact supplied.** The signing strategy for layered products is tracked
under **OCPSTRAT-3214**; the suggested entry points are the Red Hat Product
Security contact page or a TAM. No named owner exists for this question in the
channel.

**What that makes B3:** an internal decision with an optional external
confirmation, not an external gate. We can record our own bar now — key 2 and
key 3 both verified, digests pinned, unsigned local charts under a recorded
exception — and treat a Product Security opinion as confirmation of that bar
rather than as permission to hold it. That is the recommended path, because the
evidence does not change either way and the disposable validation is bounded and
teardown-scoped.

**Verified:** the bundle plus all 15 related image index manifests match their
requested digests and pass `skopeo standalone-verify` with release key 2
`567E347AD0044ADE55BA8A5F199E2F91FD431D51` and repository-bound identity. On
2026-11-10 the bundle and the four required images (`manager`, `argocd`, `dex`,
`redis`) were copied from `registry.redhat.io` for **linux/amd64 and
linux/arm64/v8** under a fail-closed policy (reject default, key 2,
`matchRepoDigestOrExact`, sigstore endpoint), and every config and layer blob
in each selected manifest was hash-verified.

Release-key-3 Sigstore signatures are now verified as well: each artifact's
`sha256-<digest>.sig` carries four signatures (six for redis), every
`registry.redhat.io` one verifies locally and binds the approved digest, and the
signing key's SPKI hash ends in `E60D446E63405576`, matching the published
release key 3. The evidence exists whichever way the policy question goes.

**Not verified, and named as such:** separate platform-child signatures,
ppc64le/s390x, the conditional Redis HA proxy image (only used with Argo CD
Redis HA, which this test does not enable), and any chart signature.

## B4 — operator CSV RBAC (internal, decided as a scoped identity)

**Status:** decided by the GCP HCP GitOps workstream on 2026-11-11. No external
answer is needed. Team review is tracked in
[GCP-1429](https://redhat.atlassian.net/browse/GCP-1429) and can change or
reverse the decision. This is not a vendor certification, signature or approval.

**What changed:** we do not accept the operator at the vendor ceiling as-is. The
GKE bundle uses the scoped identity the operator already supports. Checked in
the pinned source, `argoproj-labs/argocd-operator` at `a213c171036a`:

- `render.sh` sets `ARGOCD_CLUSTER_CONFIG_NAMESPACES` to the instance namespace,
  so `spec.defaultClusterScopedRoleDisabled: true` stops the operator creating
  the controller and server ClusterRoles and ClusterRoleBindings.
- `CONTROLLER_CLUSTER_ROLE` and `SERVER_CLUSTER_ROLE` replace the default
  namespaced Roles and point the RoleBindings at bundle-owned ClusterRoles.

What the bundle supplies:

- `<instance>-application-controller-interim-parity`, the existing
  migration-parity role.
- `<instance>-argocd-server`, new, the union of the operator's two server
  policies for the reviewed seed.
- A namespaced `Role`/`RoleBinding` granting `bind` on `roles` only, for the dex,
  redis, redis-ha and commit-server Roles the operator still creates.

**As of 2026-11-11.** The 2026-11-13 run proved an ApplicationSet grant that the
list above does not include: a namespaced grant plus the cluster-scoped pair,
with no `escalate`. Without it `ArgoCD/gitops` stayed `Failed`. See
`GCP-1286_CSV_RBAC_REVIEW.md` §8.1.

The manager ClusterRole then drops the unqualified `bind` and `escalate`. Its
only remaining `bind` is limited by `resourceNames` to the two roles above. The
vendor rule set stays the chart default and the rule-for-rule baseline.

**Limits:** the controller keeps the legacy wildcard policy. That is a deliberate
exception for this pair, not a missing switch: GCP-1285 closed with a reviewed
controller policy for the real clusters, and scoped mode cannot bind it yet.
[GCP-1433](https://redhat.atlassian.net/browse/GCP-1433) tracks that and the
removal. The server role is a copy and must be diffed on operator upgrade.
Dropping `escalate` is safe only on GKE, where the OpenShift Config API and its
SCC rule are absent. The runtime proof was completed on 2026-11-13; see the pair record.

**Record:** [`GCP-1286_CSV_RBAC_REVIEW.md`](GCP-1286_CSV_RBAC_REVIEW.md) §8.1 and
the platform register, the Confluence page [Privileged cluster controllers on GCP
HCP](https://redhat.atlassian.net/wiki/spaces/GCP/pages/521209731). Owner:
GCP HCP GitOps workstream. Tickets: GCP-1286, GCP-1429 and GCP-1433.

**What the team needs to do:** review GCP-1429 and the register entry. The
decision is ours to make because these are identities we ship and operate; the
runtime proof is recorded in the 2026-11-13 pair record.

## Not being asked

- Permission to deploy, provision, or run section B. That is B5/B6 and will be
  requested separately, after these answers.
- Any change to #166's draft status, any merge, or any cutover.
- Any statement that GKE, arm64, or the installation as a whole is certified.

## Evidence index

- `docs/GCP-1286_PLAN.md` — plan, gate table, AC mapping.
- `docs/GITOPS_RELEASE_VALIDATION.md` — runbook and recorded results.
- `docs/gcp-1286-evidence/` — the bundle manifest and signature, the key, one
  `SHA256SUMS`, the aggregate verification summaries, and per-architecture
  fresh-copy summaries.
- `tools/gitops-bootstrap/release-matrix.yaml` and `verify-release.sh` in
  gcp-hcp, branch `gcp-1286/release-matrix`.
