# OpenShift GitOps release validation (GCP-1286)

> **Historical record (source snapshot, 2026-11-17).** The runbook and commands
> below document the single authorized 2026-11-13 validation and are not current
> operating instructions. Nothing here authorizes a new run, deployment, support
> claim, or cutover. The offline tooling cited below is on historical development
> branches based on `gcp-hcp` PR #166, not on `gcp-hcp` main at this snapshot.

Status: **offline slice plus one run on authorized disposable pair** `rfl1286m1111`/`us-central1` on GKE Autopilot (2026-11-13); results in [`gitops-release-validation/2026-11-13-dev-pair.md`](gitops-release-validation/2026-11-13-dev-pair.md). No GKE support/certification claimed, no approval pre-filled. Candidate fixed; do not replace it with a local image or newer tag during validation.

Acceptance focus **linux/amd64**. ARM64 runtime validation deferred (GCP-1286 comment 19502521, 2026-11-11) and must eventually be done; `linux/arm64/v8` artifact identity verified but not approved/certified, no runtime behaviour claimed. B1 owns supported scope/versions/architectures.

## Selected release

| Item | Value |
| --- | --- |
| Product | OpenShift GitOps 1.22.2, release 7 |
| Advisory | [RHBA-2026:83602](https://access.redhat.com/errata/RHBA-2026:83602) (2026-10-15) |
| Bundle | `registry.redhat.io/openshift-gitops-1/gitops-operator-bundle@sha256:2ba58c50f771c08bbc5a1d8572c7b058df55bc71667cc66026d42840eb4bee6d` |
| CSV | `openshift-gitops-operator.v1.22.2` |
| Operator source | `redhat-developer/gitops-operator` @ `cf32771491be2ed7f03d43d279171d6c265f2d2b` |
| Downstream source | `rh-gitops-midstream/release` @ `219dafaaf0e9431f8b468b4932f337f3ce397058` |
| Route/Scheme fix | GITOPS-11466; original PR #1321, release backport PR #1354 (`139db716ae65182624429cc1d78cd5a1c625f15c`) |

The released operator source `cf327714…` is one commit ahead of the backport.
That linkage is declared source provenance, not an independently reproduced
build attestation.

## Artifact trust (review evidence)

The review verified **16 manifests** — the bundle plus all 15 distinct related
image index manifests in the CSV — with Red Hat release key 2
`567E347AD0044ADE55BA8A5F199E2F91FD431D51`, enforced repository identity and
exact digest. The bundle also passed a full `skopeo copy` under a
`reject`-default policy with `matchRepoDigestOrExact`, and a tampered manifest
was rejected. Evidence (the bundle manifest and signature, the key, one
`SHA256SUMS`, and the aggregate verification summaries) is retained in
[`docs/gcp-1286-evidence/`](gcp-1286-evidence/); per-image manifests and
signatures are re-fetched when re-verifying.

Limits:

- Platform child manifests outside bundle and four required images not independently fetched. Fresh-copy check 2026-11-10 verified config/layer blobs for those five on amd64 and arm64/v8 (see "Contract checks"); limit is remaining child manifests only.
- Release-key-3 Sigstore **verified** 2026-11-11 for bundle and four required images: every `registry.redhat.io` signature in `sha256-<digest>.sig` verifies and binds approved digest, signed by key 3 (`E60D446E63405576`). Whether required is Product Security question (B3); see `gcp-1286-evidence/sigstore/`.
- Helm charts in `gcp-hcp` are local, no vendor signature; exception in `trust.chartVerification`.
- Allowlisted image/CRD/env does **not** mean component runs.

### Chart exception, trust bar and integration review

Charts are local, no vendor signature. Exception in `release-matrix.yaml` `trust.chartVerification`: `local_chart_exception`, approved by Anand Singh 2026-11-13 ("Yes, sounds good to me."), scoped to pre-certification GKE validation, covering two 0.1.0 checksums; not a vendor signature.

B3 bar in `trust.policyDecision`: keys 2 and 3 verified for bundle/four images, artifacts digest-pinned, unsigned charts under exception. Product Security confirmation via OCPSTRAT-3214, no sign-off claimed.

Integration review in `installation.integrationReview`: Roger Flores Jr, 2026-11-16 — pin-to-bundle match, 8-CRD set, `v1beta1`-only `argocds` no `conversion`, webhook disabled, ESO credential path. Team review, not a second-party security review.

## Support position

OpenShift GitOps 1.22.2-7 has no vendor-published GKE support or certification
statement. The GitOps engineering channel's assistant replied on 2026-11-11,
self-flagged for accuracy and not a human decision record, that 1.22.2-7 is
viable for our own bounded xKS validation, that there is no 1.22.x xKS
certification, and that certification and the support model target 1.23.

GCP-HCP records this run as internal validation on GKE `1.36.4-gke.2377000`,
amd64, with no support, SLA, Tech Preview tier, patch floor, or escalation
commitment. `support.approval` stays `pending` and `platforms.certified` stays
empty. The 1.35.8 value in the matrix still describes the target environments;
it is not what this run tested.

The vendor scope is tracked as [GCP-1444](https://redhat.atlassian.net/browse/GCP-1444)
under GCP-1186, with a trigger tied to adopting the 1.23 vendor chart or the
GCP-1289 rollout. AC1 of the story is split accordingly: the linked matrix for
1.22.2-7 closes here, and the vendor statement arrives with 1.23.

## Component scope

Required: `manager`, `argocd_image`, `argocd_dex_image`, `argocd_redis_image`. Conditional: `argocd_redis_ha_proxy_image` (only with Redis HA). Excluded from core validation: `backend_image`, console plugins, Rollouts extension/controller, must-gather, Argo CD agent/principal, image updater, Promoter extension/Promoter.

`backend_image` excluded: GitopsService controller registers only when `config.openshift.io` served (`gitops-operator` `cmd/main.go` at release commit). Neither GKE topology serves it, so no backend Deployment expected (CSV declares one Deployment `openshift-gitops-operator-controller-manager`, no backend).

## Where the implementation lives

The renderer and assembler are on `gcp-hcp` PR #166 at
`90dfd92b5d35d8ff0a6ac3d114eabd59009ab3f4`. The release matrix and verifier
were added in the later GCP-1286 work, retained on `gcp-1286/runtime-bundles`
at [source revision `0ed658f`](https://github.com/floresroger/gcp-hcp/tree/0ed658f60c16f19053d5c75556c5e614f57f288e/tools/gitops-bootstrap).
Neither is on `gcp-hcp` main at `fc05a28311b77436058818d035eeeb3fb9056e8e`.
The commands below retain historical paths: `tools/gitops-bootstrap/` refers to
that development checkout; `terraform/`, `helm/`, and teardown scripts refer to
the infra checkout. Retained evidence now lives beside this record in
`gcp-1286-evidence/`. Adjust evidence paths when replaying an offline check;
the original live-run commands are not current operating instructions.

- Release matrix, verification entry point and tests: `gcp-hcp`
  `tools/gitops-bootstrap/` — `release-matrix.yaml`, `verify-release.sh`,
  `verify-release.py`, `tests/run-release-verification-tests.sh`.
- Renderer/role contract: `tools/gitops-bootstrap/render.sh`,
  `tools/gitops-bootstrap/assemble.sh`.
- Controller permission policy: `helm/charts/argocd-config/templates/controller-permissions.yaml`
  and [`docs/ARGOCD_CONTROLLER_PERMISSIONS.md`](../gcp-1285/ARGOCD_CONTROLLER_PERMISSIONS.md).
- Runtime results belong in
  [`docs/gitops-release-validation/`](gitops-release-validation/); the 2026-11-13
  pair record, including its teardown, is there.

## Offline procedure

From the `gcp-hcp` checkout that contains the matrix (dependent on #166):

```sh
tools/gitops-bootstrap/tests/run-tests.sh
```

This branch is stacked on the #166 head `90dfd92b5d35d8ff0a6ac3d114eabd59009ab3f4`,
so pass that commit as the assembly suite's PR base. The suite defaults to
`origin/main`, which is older than #166 and therefore reports #166's own
committed changes as guard failures:

```sh
BOOTSTRAP_BASE_REF=90dfd92b5d35d8ff0a6ac3d114eabd59009ab3f4 \
  tools/gitops-bootstrap/tests/run-step2-tests.sh
```

From `docs/gcp-1286-evidence`:

```sh
shasum -a 256 -c SHA256SUMS
skopeo standalone-verify --public-key-file release-key2.txt \
  candidate-raw.json \
  registry.redhat.io/openshift-gitops-1/gitops-operator-bundle:1.22 \
  567E347AD0044ADE55BA8A5F199E2F91FD431D51 bundle-signature-1
```

With an approved registry credential, verify the bundle and each required image
(`bundle`, `manager`, `argocd_image`, `argocd_dex_image`, `argocd_redis_image`)
through the single entry point, one invocation each, selecting the architecture
with `--arch amd64` or `--arch arm64/v8`:

```sh
tools/gitops-bootstrap/verify-release.sh \
  --matrix tools/gitops-bootstrap/release-matrix.yaml \
  --image manager \
  --authfile "$AUTH_FILE" \
  --key-file docs/gcp-1286-evidence/release-key2.txt \
  --workdir "$NEW_TEMP_DIR" --arch amd64
```

`--layout-dir` re-checks an existing copy without a registry, including every
config and layer blob the saved manifest references, and `--image bundle`
checks the single-platform OLM metadata manifest (not deployable) against
`spec.bundle.reference`. The entry point has no insecure-policy or TLS-bypass
option, never prints credentials, and refuses excluded components, unknown
images, unadvertised architectures, non-digest references, a reused or
symlinked work directory, and any digest/platform/blob mismatch.

## Contract checks recorded for 1.22.2-7

Offline result (original 2026-11-10; the retained 15-file inventory was
re-checksummed on 2026-11-17 in the task worktree):
`shasum -a 256 -c SHA256SUMS` passes for every retained file and
`skopeo standalone-verify` confirms the bundle against key 2
`567E347AD0044ADE55BA8A5F199E2F91FD431D51` and digest `sha256:2ba58c50…`.

The retained `candidate-raw.json` is the bundle manifest only; the layout
re-check now requires every config and layer blob that manifest references, so
that manifest-only directory fails closed with `required blob missing`. Use
`standalone-verify` above for the retained manifest. A `--layout-dir` re-check
needs a complete `dir:` copy and still requires `--workdir` for the trust
policy:

```sh
tools/gitops-bootstrap/verify-release.sh \
  --matrix tools/gitops-bootstrap/release-matrix.yaml \
  --image bundle \
  --layout-dir "$EXISTING_BUNDLE_LAYOUT" \
  --workdir "$NEW_TEMP_DIR" \
  --key-file docs/gcp-1286-evidence/release-key2.txt
```

Suites on 2026-11-12: 92 renderer checks and 47 nested release-verification
checks pass, and the assembly suite reports 193 passed with zero failures against
the #166 base above without the extracted bundle. On 2026-11-13 the
release-verification suite grew to 49 checks when the installation-approval
assertions were added; 49 pass with zero failures. The bundle parity check only
runs when `GITOPS_BUNDLE_DIR` is set; the same suite reported 179 with the
bundle and 178 without it on 2026-11-10, before the scoped-identity checks were
added. Its `origin/main` default (which predates #166) reports 177 passed
and 1 failed, the active `bootstrap/` guard. With the extracted bundle it also
confirms that 7 of 8 CRDs are byte-identical to the official files and that the
adapted ArgoCD `v1beta1` block and the ControllerConfiguration schema match.
The unchanged infra suites `make test-argocd-config`,
`make test-argocd-notifications` and `make test-argocd-controller-permissions`
also pass, so this slice adds no configuration, notification or controller
permission regression. The fresh-copy path is now exercised: on 2026-11-10 the
bundle and the four required images (`manager`, `argocd_image`,
`argocd_dex_image`, `argocd_redis_image`) were copied from `registry.redhat.io`
for linux/amd64 and linux/arm64/v8 under the generated policy and passed digest,
selected-platform and config/layer blob checks. Sanitized results are in
[`gcp-1286-evidence/fresh-copies/`](gcp-1286-evidence/fresh-copies/) under
`amd64/` and `arm64/`; the copied layouts were deleted. The bundle layer was
also extracted to run the assembly suite's byte-level CRD parity checks (179
passing). Unexercised: ppc64le/s390x, the conditional Redis HA proxy image, and
platform child signatures (B3).

- Eight CRDs are packaged; `argocds.argoproj.io` is served/stored as `v1beta1`
  only with no conversion webhook. The v1alpha1 block and `spec.conversion` are
  removed by the documented local adaptation.
- The CSV's related-image env values plus the manager `.image` are exactly the
  15 matrix references; the offline suite fails if they drift.
- `--controller-cluster-role gitops-hcp-controller-local` is forwarded to the
  unique manager container's `CONTROLLER_CLUSTER_ROLE`. The infra chart derives
  the role name from `instanceName: gitops`, requires `operator.enabled` and
  `operator.ownershipTransferred`, rejects wildcards, limits `bind`/`escalate`
  to named RBAC roles, and sets `spec.aggregatedClusterRoles=true`. The renderer
  input selects a separately provisioned role; it creates no grant and does not
  remove default or interim bindings. The scoped mode added in `dd0b1e0`
  (`--scoped-rbac`, chosen for the GCP-1286 pair) instead has the bundle own both
  ClusterRoles and fixes the controller role to
  `<instance>-application-controller-interim-parity`; it rejects any other
  controller name. Accepting a reviewed controller role there is
  [GCP-1433](https://redhat.atlassian.net/browse/GCP-1433).

## Runtime validation on the disposable Dev pair

One pair, `rfl1286m1111` in `us-central1`, both projects under
`folders/1029606351992`, amd64 only, `argocd_controller_project_roles = []`.
Both clusters ran `1.36.4-gke.2377000` on the REGULAR channel at apply time; the
saved plan was built against the then-current `1.35.8` default, so the run
continued on 1.36.4. The runtime request's 1.35.x stop condition was written
against the 2026-11-12 plan baseline; Autopilot clusters upgrade automatically,
so the apply-time version is the channel default. The apply-time version was not 1.35.x, so the stop-and-ask condition applied;
the run continued without that decision. Roger's acceptance on 2026-11-17 was
retrospective, after the run and teardown. **No 1.35.8 parity is claimed.**

Converged state: both Fleet memberships `READY` and `SYNCED` on Config Sync
parent `7982574` with child `04f6e7d`; both child RootSyncs `SYNCED` with no
errors; `ArgoCD/gitops` `Available` and `Reconciled=True` on both clusters with
every operand Running; the `argocds.argoproj.io` CRD served and stored as
`v1beta1` only with `conversion: None`; Route present only on the MC and Config
API on neither; all four deployed release images matching the matrix index
digests with amd64 children on amd64 nodes. Release components converged.
Application health was MC 15/16 and region 15/24 `Synced`/`Healthy`, with
`argocd-config` a known `OutOfSync` diff and the remaining platform and
`gecko-*` applications pre-existing failures outside this scope (pair record
section 10).

Runtime findings recorded in the pair evidence:

- **ApplicationSet RBAC.** The operator could not create the ApplicationSet
  controller Role or the cluster-scoped pair under the scoped identity. The child
  bundle now declares a namespaced grant and the cluster-scoped pair, with no
  `escalate` and no change to the two-role `bind` list. Proven live on both
  clusters after republish.
- **dex pull secret race.** The region dex pod missed the operator-propagated
  pull secret by one second at first reconcile. Deleting the pod recovered it;
  recorded as a startup race.
- **MC Route/Scheme ordering.** HyperShift's Route CRD arrived on the MC after
  the manager started, so the manager had no Route scheme and stayed `Failed`
  until it was restarted. The 1.22.2 backport covers Route-without-Config at
  startup, not a Route API that appears later. Tracked as GCP-1441 for the
  upstream issue or a documented bootstrap order.
- **Fleet revision lag.** The first Terraform apply of the pin change reported
  the MC membership modified while the live spec and state kept the old revision;
  a second apply moved it. Verification has to read the membership spec.

The private chain was exercised end to end: the generated root Application
matched the approved tuple, the fixture synced from the private repository at
`73790466`, a deleted ConfigMap was self-healed within seconds, and revision
`67bff31e`, which drops the extra manifest, pruned that object.

## Runbook: apply, verify, tear down

This is the reviewed procedure, distilled from the 2026-11-13 run into the clean
path. The run itself needed the deviations the pair record keeps: a second apply
of the same pin change when a membership lagged, a hand-applied RBAC probe that
was later declared, a manual dex pod delete, and manual teardown repairs. Those
are evidence, not procedure. It assumes the B5/B6 grant already names the pair,
region, architecture and cleanup scope. Nothing here authorizes a run by itself.

### 1. Preflight

Confirm the tool versions the suites need, and record the approved ESO
pull-secret path without reading the secret:

```sh
terraform version
helm version --short
kubectl version --client
skopeo --version
yq --version
```

Each pair gets its own `terraform.tfvars` under
`terraform/config/dev-all-in-one/example/`. Do not reuse another pair's state
prefix, folder, or registry credentials.

### 2. Plan and apply

```sh
cd terraform/config/dev-all-in-one/example
terraform init
terraform plan -out=pair.tfplan
chmod 600 pair.tfplan
terraform show -no-color pair.tfplan
```

Read the plan for anything outside the pair: shared projects, DNS records, IAM
members, folder changes. Stop if any appear. Confirm the apply-time GKE
version; the runtime request's stop condition is 1.35.x, and a different
version needs an explicit decision before apply. The saved plan embeds the
fixture's deploy key, so keep it mode 600. `auto-apply-with-log.sh` exists but
replans instead of applying a saved file, so the reviewed-plan apply is the
recorded method. The 2026-11-13 pair planned `215 to add, 0 to change, 0 to
destroy`.

After review:

```sh
terraform apply pair.tfplan
rm -f pair.tfplan
```

### 3. Verify the platform and the release

Run these per cluster context. Set `CTX` first; `OPERATOR_NS` was
`gitops-operator` and the instance namespace was `argocd`.

```sh
kubectl --context "$CTX" get nodes -o json
gcloud container fleet memberships list --project "$FLEET_PROJECT" \
  --format='value(name,state.code)'
kubectl --context "$CTX" -n config-management-system get rootsyncs -o json
kubectl --context "$CTX" -n config-management-system get rootsyncs \
  -o jsonpath='{range .items[*]}{.metadata.name}{" src="}{.status.source.commit}{" sync="}{.status.sync.commit}{"\n"}{end}'
kubectl --context "$CTX" -n "$OPERATOR_NS" get deployment \
  openshift-gitops-operator-controller-manager -o json
kubectl --context "$CTX" get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{range .status.containerStatuses[*]}{.name}{"="}{.imageID}{" "}{end}{"\n"}{end}'
kubectl --context "$CTX" -n "$OPERATOR_NS" get externalsecret \
  redhat-registry-pull-secret -o json
kubectl --context "$CTX" -n argocd get argocd gitops -o json
kubectl --context "$CTX" -n argocd get applicationsets,applications -o json
kubectl --context "$CTX" get crd argocds.argoproj.io \
  -o jsonpath='{range .spec.versions[*]}{.name}{" served="}{.served}{" storage="}{.storage}{"\n"}{end}'
kubectl --context "$CTX" get crd argocds.argoproj.io -o jsonpath='{.spec.conversion}'
kubectl --context "$CTX" api-resources --api-group=route.openshift.io
kubectl --context "$CTX" api-resources --api-group=config.openshift.io
```

What the output has to show:

- Both Fleet memberships `READY`, and every RootSync `SYNCED` with the expected
  source and sync commits. A successful apply does not guarantee this. The
  2026-11-13 run needed a second apply of the same pin change because the MC
  membership reported success while its live spec and state kept the old
  revision.
- The registry pull secret ExternalSecret reports `SecretSynced` and `Ready`, and
  the propagated pull secret is present in the instance namespace. A dex pod
  that misses it is the first-reconcile race in the pair record.
- `ArgoCD/gitops` at `phase=Available` with `Reconciled=True`, and every operand
  Running.
- The CRD serving and storing `v1beta1` only, with `conversion` empty or `None`.
  The packaged file sets no `conversion` key at all.
- Route API on the MC and on neither target for the Config API.
- Container `imageID` values equal the matrix `indexDigest` values, resolved
  index to amd64 child. Compare digests, not tag strings.

### 3a. Management cluster Route CRD ordering (GCP-1441)

On a management cluster that installs HyperShift through Argo CD, the manager
normally starts before HyperShift creates `routes.route.openshift.io`. The
manager registers the Route type only at startup: `cmd/main.go` at
`cf32771491be2ed7f03d43d279171d6c265f2d2b` calls `routev1.AddToScheme` only when
`util.IsRouteAPIFound()` and `InspectCluster` runs once. A Route API that
appears later is never added to the scheme, so every Route reconciliation fails
with `no kind is registered for the type v1.Route in scheme "pkg/runtime/scheme.go:111"`.
The 1.22.2 backport GITOPS-11466 (PR #1354, `139db716ae65182624429cc1d78cd5a1c625f15c`)
makes Route detection independent of the Config API at startup but does not
cover a Route API that appears after startup. Observed on 2026-11-13: manager
at 16:00:03Z, Route CRD at 16:47:00Z, `ArgoCD/gitops` `phase=Failed` from
16:47:11Z until the manager pod was deleted at 17:08Z, then `phase=Available`
and `Reconciled=True` on the replacement.

Workaround after the first application sync on a new MC: either install the
Route CRD before the manager starts, or restart the manager once the CRD
exists. Check first, then restart only when present.

Check:

```sh
kubectl --context "$CTX" get crd routes.route.openshift.io
```

Expected when present: one row for `routes.route.openshift.io`. The `CREATED AT`
value is the CRD's own creation time, so it differs per cluster; it was
`2026-11-13T16:47:00Z` on the validated pair.

```
NAME                        CREATED AT
routes.route.openshift.io   <time the CRD was created>
```

Expected when absent:

```
Error from server (NotFound): customresourcedefinitions.apiextensions.k8s.io "routes.route.openshift.io" not found
```

Alternative:

```sh
kubectl --context "$CTX" api-resources --api-group=route.openshift.io
```

Expected when present:

```
NAME   SHORTNAMES   APIVERSION              NAMESPACED   KIND
routes   route      route.openshift.io/v1   true         Route
```

Expected when absent: no resources found.

> **Archive sequencing correction:** CRD existence alone is insufficient:
> discovery may lag. Before the historical restart command below, wait until
> discovery lists `routes` in `route.openshift.io/v1`, as required by the
> [GCP-1441 report](gitops-release-validation/UPSTREAM-ISSUE-GCP-1441-route-late-CRD.md).
>
> ```sh
> until kubectl --context "$CTX" get --raw /apis/route.openshift.io/v1 | \
>   jq -e -s 'length == 1 and any(.[0].resources[]?; .name == "routes" and .kind == "Route")' >/dev/null; do
>   sleep 5
> done
> ```
>
> Confirm discovery succeeds; do not restart while it is absent or failing.
> The following original wording is retained as history, with this added gate.

Restart once the CRD is present:

```sh
kubectl --context "$CTX" -n "$OPERATOR_NS" rollout restart deployment/openshift-gitops-operator-controller-manager
```

Expected:

```
deployment.apps/openshift-gitops-operator-controller-manager restarted
```

Wait for rollout:

```sh
kubectl --context "$CTX" -n "$OPERATOR_NS" rollout status deployment/openshift-gitops-operator-controller-manager
```

Expected:

```
deployment "openshift-gitops-operator-controller-manager" successfully rolled out
```

On success `ArgoCD/gitops` returns to `phase=Available` with `Reconciled=True`,
as observed after the manual delete at 17:08Z on 2026-11-13.

Full context: `docs/gitops-release-validation/2026-11-13-dev-pair.md` section
5.3. Upstream issue: `docs/gitops-release-validation/UPSTREAM-ISSUE-GCP-1441-route-late-CRD.md`,
filed as [redhat-developer/gitops-operator#1566](https://github.com/redhat-developer/gitops-operator/issues/1566).

### 4. Behavior

Confirm the root ApplicationSet generates the expected root Application, a
fixture syncs from the private repository, one harmless change self-heals, and
one reviewed prune removes the extra object. Record what was observed.

### 5. Teardown

GCP-1445 retired `teardown.sh`. `nuke.sh` is the only supported teardown path
and deletes the projects. This section is the historical 2026-11-13 record,
kept because GCP-1440 documents why the old drain failed; do not follow it as a
procedure. The old defects changed the order of operations: scale the operator
to zero before deleting operand resources.

```sh
kubectl --context "$CTX" -n gitops-operator scale deployment \
  openshift-gitops-operator-controller-manager --replicas=0
```

Scale the GMP StatefulSet to zero before its PVC as well, because it recreates
the PVC otherwise. The historical run destroyed with reviewed plans instead of the
script's internal replan:

```sh
cd terraform/config/dev-all-in-one/example
terraform plan -destroy -out=teardown.tfplan
terraform show -no-color teardown.tfplan
terraform apply teardown.tfplan
```

After a partial apply, plan the remainder and review it again before applying.
The 2026-11-13 pair took three reviewed passes: 215, then 78, then 11.

Post-destroy checks:

```sh
terraform state list
gcloud projects describe dev-reg-us-c1-rfl1286m1111 --format='value(lifecycleState)'
gcloud projects describe dev-mgt-us-c1-rfl1286m1111 --format='value(lifecycleState)'
gcloud projects describe gcp-hcp-dev-svc-rfl1286m1111 --format='value(lifecycleState)'
gcloud resource-manager folders describe 1029606351992 --format='value(lifecycleState)'
```

On the historical `terraform destroy` path, `terraform state list` must be
empty. Projects and the folder report `DELETE_REQUESTED` for a while, which is
not deletion. For a new teardown use `nuke.sh` (GCP-1445), which leaves
Terraform state listing the deleted resources: confirm the projects with
`gcloud projects describe`, then run `./clean.sh` and `terraform init` before the next apply. Do not
remove `prevent_destroy` protections. On this workstation `gcloud storage` and
`gsutil` fail with `ssl has no attribute PROTOCOL_TLSv1`, so bucket inspection
goes through the GCS JSON API.

### 6. Known traps and where they are recorded

- **MC Route/Scheme ordering (GCP-1441).** See section 3a for the check and
  restart commands. The manager at `cf327714` registers the Route type only at
  startup, so a Route CRD that appears after startup is never added to the
  scheme until the manager restarts. After the first MC sync either install the
  CRD before the manager starts or restart the manager once the CRD exists.
- **Operator restores deleted operands (historical).** The retired `teardown.sh`
  scaled namespaces but not the operator that recreates their contents. GCP-1440.
- **dex pull secret race.** The region dex pod can miss the propagated pull
  secret at first reconcile. Delete the pod and record it.
- **Fleet revision lag.** A successful apply can leave a membership on the old
  revision. Re-apply the same change and read the membership spec.

## Blocking inputs

| Gate | Decision needed |
| --- | --- |
| B1a | Cleared 2026-11-11: engineering channel assistant (self-flagged, not human decision) says 1.22.2-7 viable for own xKS validation, no 1.22.x cert, cert targeted at 1.23. Proceed, claim nothing certified |
| B1b | No formal escalation contacts (`#forum-openshift-gitops` plus direct GITOPS-11013 leads); arm64 not in xKS scope; no patch floor. Support-tier/patch floor moved to [GCP-1444](https://redhat.atlassian.net/browse/GCP-1444) with AC1 split; named contact remains ours. Non-blocking for section B |
| B2 | Accepted 2026-11-13: GitOps engineer confirmed method on Slack (relayed by Roger), republished both charts via GITOPS-10648 recipe and accepted bundle chart set for pre-cert validation ("Yes, sounds good to me.", 2026-11-13). `installation.approval` `approved` with approver/scope; not support/cert. See `GCP-1286_PLAN.md` B2 and Appendix A for chart manifests/digests (27 CRDs, `85ab6bee…baa59e4` etc.); quay remains a developer namespace and not a pin source |
| B3 | Routed to Product Security 2026-11-11: the engineering channel's assistant says nothing further is needed from them; key 2 plus key 3 verified is described as comprehensive. The compliance bar is recorded 2026-11-16: both keys verified, required artifacts digest-pinned, unsigned local charts under the recorded exception. Product Security confirmation is sought via OCPSTRAT-3214 and is not a gate |
| B4 | Decided 2026-11-11 as a scoped operator identity: the bundle pre-creates the controller and server ClusterRoles and narrows the manager to `bind` on those two roles only, with no `escalate`. Design and residual in [`GCP-1286_CSV_RBAC_REVIEW.md`](GCP-1286_CSV_RBAC_REVIEW.md) §8.1. Runtime proof completed 2026-11-13 on the disposable pair: `ArgoCD/gitops` `Available` with the scoped manager, and the ApplicationSet grant added after the runtime finding |
| B5 | Granted 2026-11-13 for `rfl1286m1111`, `us-central1`, `argocd_controller_project_roles = []`, amd64 only. Applied, validated and destroyed the same day. Request: [`GCP-1286_RUNTIME_REQUEST.md`](GCP-1286_RUNTIME_REQUEST.md) |
| B6 | Granted 2026-11-13 by Roger in chat, cleanup authorized and completed the same day: `terraform state list` empty, all three projects and the deployment folder `DELETE_REQUESTED`, pair delegation records removed. Runtime results: [`gitops-release-validation/2026-11-13-dev-pair.md`](gitops-release-validation/2026-11-13-dev-pair.md), teardown in section 11 |

`release-matrix.yaml` records `support.approval` as `pending` and
`installation.approval` as `approved` for this validation, with the approver and
scope recorded; B3 is an internal compliance record and B4 is decided as the
scoped identity. Support approval remains blocked by B1b, and
`platforms.certified` stays `[]`. The offline suite asserts that support approval
is not pre-filled and that the installation approval names an approver. Section B
of [`GCP-1286_PLAN.md`](GCP-1286_PLAN.md) was exercised only under the separate
2026-11-13 pair grant, and that grant's teardown is complete; nothing here
authorizes a second run, another teardown, or any other environment.

## Boundaries

- Optional components, Promoter/Source Hydrator workflows: GCP-1287.
- Upgrade, failure injection and recovery: GCP-1288.
- Environment selection, cutover and promotion: GCP-1289.
- Preserve GCP-1280/1282–1285 behavior; do not redesign auth, notifications or
  controller least privilege here.
- A successful pull, a rendered manifest, or a merged #166 is not runtime,
  support or cutover permission.
