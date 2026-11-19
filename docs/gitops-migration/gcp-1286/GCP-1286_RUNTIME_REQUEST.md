# GCP-1286 runtime authorization request (B5/B6)

> **Historical appendix.** This authorization request is retained for the
> record. Its grant was bounded to the named disposable pair and was consumed by
> the 2026-11-13 run; it does not authorize another run.

> **Executed 2026-11-13 under this grant and torn down the same day.** Kept as
> the authorization record; the values below are as proposed, not necessarily
> as run. Run record: [`gitops-release-validation/2026-11-13-dev-pair.md`](gitops-release-validation/2026-11-13-dev-pair.md).

Request for the only remaining infrastructure step: provision one disposable pair, install reviewed 1.22.2-7 packaging, run amd64 checks, delete. Bounded to that pair. Does not authorize integration/stage/production changes, merge, or support statement.

Two technical gates precede apply — B2 packaging acceptance and B4 scoped identity — both resolved. B2 was accepted against the 1.35.x plan baseline; the executed run was 1.36.4, accepted separately on 2026-11-17. This does not replace B5/B6 authorization.

- **B2** — cleared 2026-11-13. A GitOps engineer confirmed the packaging method
  on Slack (relayed by Roger), republished both charts to match, and then
  accepted this bundle's chart set: bundle-built charts from 1.22.2-7, trimmed
  to the 8 CRDs this deployment owns, `argocds` at `v1beta1` only with no
  `spec.conversion`, `ENABLE_CONVERSION_WEBHOOK=false`, and Helm outside OLM on
  GKE 1.35.x. The acceptance covers this pre-certification validation only. It
  is not a support or certification statement. #166 stays draft.
- **B4** — the operator CSV RBAC position is decided on 2026-11-11: the bundle
  uses the scoped identity, not the vendor ceiling as-is. The platform register
  is the Confluence page [Privileged cluster controllers on GCP
  HCP](https://redhat.atlassian.net/wiki/spaces/GCP/pages/521209731); team
  review and any change are tracked in GCP-1429. Sections 8.1 and 9 of
  [`GCP-1286_CSV_RBAC_REVIEW.md`](GCP-1286_CSV_RBAC_REVIEW.md) carry the design
  and the residual, and the runtime proof is section B step 4 of the plan.

B1a is settled: we may run this ourselves and claim nothing certified. B3 is
recorded internally.

## Target

Proposed values follow the GCP-1285 disposable pair (`rfl1285m1109`, sector
`rflores-gcp1285`, one AIO state). Confirm or change each one before apply.

| Input | Proposed | Notes |
| --- | --- | --- |
| Terraform root | `terraform/config/dev-all-in-one/example` | One state creates the region and management cluster pair |
| `parent_folder_id` | `<Dev folder used by the GCP-1285 pair>` | Read from that state, do not guess |
| `github_username` | `floresroger` | Only used to derive the default private repo URL; the two repo rows below are authoritative |
| `sector` | `rflores-gcp1286` | Pair-specific label |
| `infra_id` | `rfl1286m1111` | Fixed, no dashes; must not reuse a recently deleted id (7-day project soft delete) |
| `region` | `us-central1` | Matches the observed integration pair. `northamerica-northeast2` is the fallback if quota or capacity is better there |
| Cluster mode | Autopilot | Module default; matches all six observed clusters |
| GKE version | Current `REGULAR` default | Record the actual master and node version at apply time. If it is not 1.35.x, stop and ask before continuing |
| Architecture | linux/amd64 only | arm64 runtime stays deferred (comment 19502521) |
| State | prefix `dev-all-in-one/rflores-gcp1286` in the existing `gcp-hcp-commons-dev-terraform-state` bucket | Uncomment the backend block in the pair's copy of `main.tf`; single state, no TFC workspace, no shared environment changes |
| `configsync_git_repo` | `https://github.com/floresroger/gcp-hcp.git` | Public fork, `auth: none`, same shape as the GCP-1285 rehearsal |
| `configsync_git_revision` | publish commit SHA | Full SHA, not a branch name |
| `configsync_git_dir_mc` | `bundles/gcp1286/dev/rflores-gcp1286/<region>/parent-mc` | Parent bundle for the management cluster |
| `configsync_git_dir_region` | `bundles/gcp1286/dev/rflores-gcp1286/<region>/parent-region` | Parent bundle for the region cluster |
| `argocd_git_repo` | `git@github.com:floresroger/gcp-hcp-infra.git` | Private repo, clone-only credentials through the existing ESO path |
| `argocd_git_revision` | private fixture SHA | Full SHA |
| Namespaces | `gitops-operator`, `argocd` | Matches `assemble.sh --namespace` and the tested prototype; instance name `gitops` |
| Time window | 5 business days from apply, teardown immediately after evidence capture | Confirm dates |

The pair creates two projects, a folder, DNS records in
`dev.gcp-hcp.devshift.net`, and pair-specific entries in `commons-dev`. Those
additions are part of the grant and are removed by teardown.

## What gets published first

None of this is applied until the grant is given, but it has to exist at an
immutable revision before the apply so the runtime is reproducible.

1. **Operator bundles (public).** Generate the MC and region child bundles from
   the approved charts at `floresroger/gcp-hcp` commit `ffc9b86` (chart files
   identical to #166 head `90dfd92b`) and commit one target directory,
   `bundles/gcp1286/dev/rflores-gcp1286/<region>/`, containing `mc/`, `region/`,
   `parent-mc/`, `parent-region/` and `parent-artifacts/shared-crds.yaml`. Copy
   the parent layout from `bundles/gcp1285/parent-{mc,region}` on the
   `gcp-1285/rehearsal-bundles` branch at revision `89857b09`: the same ten
   numbered files, with `10-gitops-rootsync.yaml` pointing `git.repo` at
   `https://github.com/floresroger/gcp-hcp.git`, `git.revision` at the child-bundle
   commit, `git.dir` at the matching child directory, and `auth: none`. The
   shared CRDs are `03-shared-crds.yaml` in each parent, byte-identical to
   `parent-artifacts/shared-crds.yaml`. Commit the shared CRDs at the documented
   `bootstrap-gitops/parent-artifacts/shared-crds.yaml` path too, replacing the
   two handoff guards in `tests/run-step2-tests.sh` that currently refuse a
   committed bundle and adding an equality check. Publish the child bundle first and record its full SHA. Point each parent
   `10-gitops-rootsync.yaml` at that child SHA, then publish the parent and
   record its full SHA. A parent cannot pin the commit that contains itself:
   writing a SHA into the file changes the commit. Use the child SHA for both
   child RootSync revisions and the parent SHA for the Terraform
   `configsync_git_revision` input, and record both SHAs separately. This is a
   public repo; no secrets go in it.
2. **Private fixture and render (private).** On one task branch in
   `floresroger/gcp-hcp-infra`, add `gcp-1286/private-chain-test/` shaped like
   the GCP-1280 AC5 fixture (`Chart.yaml`, `values.yaml`, one ConfigMap in
   `argocd` that records the repository and credential path) and the `dev`
   render target described below. Pin that branch's full SHA as
   `argocd_git_revision`, and use clone-only credentials delivered through the
   existing ESO path. Nothing from this branch goes to `main`.
3. **Application wiring.** The parent's `root-app` ApplicationSet takes
   `repoURL`, `targetRevision` and `path` from the `meta_git_repo`,
   `meta_git_revision` and `meta_argocd_path` annotations on the cluster
   secret. Those default to
   `argocd/rendered/{region,management-cluster}/dev/rflores-gcp1286/<region>`.
   Follow the GCP-1280 disposable target (`5267dae9`): add
   `{environment: dev, sector: rflores-gcp1286, region: <region>}` to
   `argocd/config/{region,management-cluster}/config.yaml` and render both
   directories on the private task branch, with one Application whose source is
   the fixture chart path. Record the exact tuple (repo URL, revision, path,
   destination, sync policy) and include the render in the reviewed plan. No
   shared render target is changed for other environments.

## Ordered operations

1. **Preflight (read-only).** Confirm billing and quota for two Autopilot
   clusters in the chosen region, project-creation rights under the folder,
   registry entitlement, tool versions (`terraform`, `helm`, `kubectl`, `yq`,
   `skopeo`), and the ESO service account path the pair will use. Record the
   approved secret location without reading it.
2. **Plan.** Save the full plan. Review resource identity, project/folder
   additions in `commons-dev`, DNS records, IAM members, and unrelated drift.
   No targeting to hide effects. If the plan contains anything outside the pair,
   stop.
3. **Apply the pair.** One state. Then verify both Fleet memberships and both
   Config Sync revisions, not just the apply exit code. A prior pair had one
   membership stay on the old revision despite a successful apply.
4. **Bootstrap and operator.** Confirm Config Sync applies the parent bundle on
   each cluster and the child RootSync it creates reports the child bundle
   Synced/Healthy. Confirm the ArgoCD CRD is served/storage as `v1beta1` only
   with no `spec.conversion`, then verify the manager Deployment and the
   `ArgoCD/gitops` instance become Available with stable restarts. Record the
   runtime `jsonpath` output for the CRD.
5. **Topology and identity checks.** MC must have `route.openshift.io` and no
   `config.openshift.io`; region must have neither. Capture every container
   imageID with node architecture and join it to the matrix. Resolve index to
   child digests rather than comparing strings. Record actual requests/limits
   after scheduling, since Autopilot may adjust them.
6. **Private chain and behavior.** Confirm the root ApplicationSet generates the
   root Application from the approved tuple, the fixture syncs from the private
   repository, one harmless change self-heals, and one reviewed prune removes
   the extra object. Confirm required baseline applications stay Synced/Healthy.
7. **Evidence.** Write sanitized results into
   `docs/GITOPS_RELEASE_VALIDATION.md` and `docs/gitops-release-validation/`.
   No secret payloads, tokens, private keys, full env dumps or unsanitized logs.
8. **Teardown.** Remove the fixture through its declarative owner, verify prune,
   then run `nuke.sh`, the only supported teardown path (GCP-1445). The
   historical 2026-11-13 run reviewed the retired `teardown.sh` and a destroy
   plan; do not follow that sequence. Verify both projects, the folder, DNS
   records and the pair-specific `commons-dev` entries are gone. Do not force
   state operations or remove `prevent_destroy` protections.
   Leave anything unowned or uncertain untouched and record it.

## Explicitly not authorized by this request

- Any change to integration, stage, production, the global clusters, shared
  CRDs, or shared IAM.
- Merging #166, marking it ready, or any cutover or promotion work
  (GCP-1287/1288/1289).
- arm64 runtime validation, optional components, Promoter or Source Hydrator
  workflows.
- New Terraform advisories or shared-platform changes.
- Jira writes beyond the recorded arm64 deferral note.

## Authorization

Signing this confirms the target table, the publish items, the operations above,
and the teardown scope.

- Authorized by: Roger, in chat on 2026-11-13
- Date: 2026-11-13
- Target pair: `rfl1286m1111` / `us-central1`
- Publish commits: `b516e70` (gcp-hcp, Config Sync pin), `73790466`
  (gcp-hcp-infra, Argo CD pin). Superseded within the same grant by `7982574`
  (gcp-hcp parent, child `04f6e7d`) after the ApplicationSet finding, and by
  `67bff31e` (gcp-hcp-infra) for the prune step
- Window and teardown date: apply and validation from 2026-11-13; teardown date
  not yet set, and cleanup of this pair is authorized in this same grant
- Conditions or exclusions added: `argocd_controller_project_roles = []`;
  linux/amd64 only; B2 acceptance recorded 2026-11-13; #166 stays draft

**Disposition, 2026-11-17 (retrospective).** This request assumed 1.35.x and
said to stop and ask otherwise. The pair came up on 1.36.4, the Autopilot
channel default at apply time, so the stop-and-ask condition applied; the run
continued without that decision. Roger's acceptance on 2026-11-17 came after
the apply, validation and teardown; it does not retroactively authorize the
deviation. No 1.35.8 parity is claimed.

Until this was signed on 2026-11-13, preparation stayed offline: generating bundles, writing the
fixture, and rehearsing commands were fine; provisioning, publishing, IAM/secret
writes, applies and Kubernetes operations were not.
