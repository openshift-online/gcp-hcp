# teardown.sh validation run, 2026-11-16

> **Historical.** This documents `teardown.sh`, which GCP-1445 retired. `nuke.sh`
> is the only teardown path and deletes the projects rather than draining them.
> Keep this record as evidence for GCP-1440. Do not follow its procedure.

Sanitized result of one end-to-end run of hardened `teardown.sh` from PR #2855 (GCP-1440) on a disposable pair. Purpose was teardown path, not feature test: apply pair, converge, run script unattended, record script and Cloud output.

## 1. Target

| Item | Value |
| --- | --- |
| Pair | `rfl1440m1116`, `us-central1` |
| Sector | `rflores-gcp1286` |
| Projects | `dev-reg-us-c1-rfl1440m1116`, `dev-mgt-us-c1-rfl1440m1116` |
| Service project | `gcp-hcp-dev-svc-rfl1440m1116` |
| Folder | `folders/601530101020` (`dev-us-c1-rflores-gcp1286`, parent `folders/880244462064`) |
| State | `gs://gcp-hcp-commons-dev-terraform-state`, prefix `dev-all-in-one/rflores-gcp1286` |
| Clusters | `dev-reg-us-c1-rfl1440m1116-gke`, `dev-mgt-us-c1-rfl1440m1116-gke` |
| `deletion_protection` | `false` |

The apply ran from the PR worktree at `4098feac` and reported `215 added, 0
changed, 0 destroyed`, exit 0.

Authorization basis: GCP-1440 live teardown validation, recorded by Roger on
2026-11-16 (comment 19588211), against PR #2855 head `4098feac`; disposable pair,
torn down in the same run. This is a GCP-1440 teardown run, not a GCP-1286
B5/B6 run; B5/B6 covers the validation pair `rfl1286m1111`.

Image digests and `imageID` mappings were not collected: this run tested
teardown, not release identity. The image set is in
`2026-11-13-dev-pair.md`, and the evidence README records the teardown
record exemption.

## 2. Convergence before teardown

| Check | Result |
| --- | --- |
| GKE versions | `1.36.4-gke.2377000` master and nodes on both clusters, `RUNNING` |
| Fleet memberships | `dev-reg-...-gke` and `dev-mgt-...-gke` both `READY` |
| Config Sync parent | `root-sync` at `7982574b93110101dd29f4260fe5306a28efc9c6` on both clusters, `Syncing=False` with "Sync Completed" |
| Config Sync child | `gitops-workload` at `04f6e7d249352ca7813adccd3fda9e74ae8da113` on both clusters, `Syncing=False` |
| `ArgoCD/gitops` | `phase=Available`, `Reconciled=True` on both clusters |
| Route API | `routes.route.openshift.io` present on the MC only, created 19:42:35Z |
| GKE support scope | none claimed |

The MC GitOps manager started at 19:38:16Z, before the Route CRD existed. It
restarted at 19:57:47Z and logged `route.openshift.io/v1 API verified`, then
started the Route controller. The trigger for that restart is not recorded in
this run's logs, so do not read it as automatic recovery, and do not read it as
proof that no manual restart was needed. The GCP-1441 scheme error did not
recur.

Application baseline at the same point:

- Region: 24 Applications. Beyond `argocd-config` `OutOfSync`/`Healthy`, the
  outstanding entries are `platform-api-server` `OutOfSync`/`Healthy`,
  `gecko-version-sync` `OutOfSync`/`Missing`, and six `gecko-*` Applications
  `Synced`/`Progressing`. This matches the baseline in
  `2026-11-13-dev-pair.md` section 10.
- MC: 16 Applications, all `Synced`/`Healthy` except `argocd-config`
  `OutOfSync`/`Healthy`.

### dex pull race

`gitops-dex-server` was `Init:ImagePullBackOff` on both clusters. The event was
`401 Unauthorized` from `registry.redhat.io` for the `argocd-rhel9` init image,
and the pods carried no `imagePullSecrets` while the service account already had
`redhat-registry-pull-secret`. This is the race described in
`2026-11-13-dev-pair.md` section 5.2: the pod was created before the service
account admission injected the secret.

Deleting the two dex pods produced replacements that pulled the image and ran
`1/1`. Both `ArgoCD` instances then reported `Available`. Nothing else was
changed.

## 3. Teardown run

Command, from `terraform/config/dev-all-in-one/example/` in the PR worktree:

```
./teardown.sh 2>&1 | tee <log>
```

Started 15:08:15 EST and printed `Teardown Complete` at 15:27:07 EST, about 19
minutes. No manual steps were needed once the script started.

The state held 223 addresses including data sources at start. Phases, as
reported by the script:

1. Fleet `config-management` disable succeeded. The three Fleet feature and
   membership addresses were removed from state.
2. Config Sync, the GitOps operator and `argocd` were stopped on both clusters.
   The operator logged "No pods remain" in `gitops-operator` on both.
3. `gecko-system` was scaled on the region cluster, and NodePools and
   HostedClusters were deleted on the MC. The 600 second wait passed.
4. Blocking resources were deleted per cluster: the five resource types, the
   NEG annotated Services, `monitoring`, `gmp-public`, PVCs and PVs, then Config
   Connector webhooks and resources. The 600 second Config Connector wait
   passed.
5. Resource verification passed on both clusters.
6. NEG cleanup: the MC project was clean immediately. The region project still
   had one NEG at the 300 second advisory limit and logged the warning. VPC
   deletion then succeeded, so the NEG did not block anything.
7. `google_storage_hmac_key.oidc_cdn[0]` was removed from state.
8. Bucket versions were emptied: one version in
   `gcp-hcp-dev-svc-rfl1440m1116-discovery`, zero in
   `dev-reg-us-c1-rfl1440m1116-oidc`.
9. `terraform destroy -parallelism=20 -auto-approve` planned and destroyed 209
   resources, including both clusters, the three projects and the folder.

The run logged zero `ERROR` lines. The only warnings were the two `gmp-system`
namespace absences (the bundle runs `gke-gmp-system` and `gmp-public`) and the
advisory NEG timeout above.

## 4. Post-teardown state

| Check | Result |
| --- | --- |
| `terraform state list` | empty |
| `dev-reg-us-c1-rfl1440m1116` | `DELETE_REQUESTED` |
| `dev-mgt-us-c1-rfl1440m1116` | `DELETE_REQUESTED` |
| `gcp-hcp-dev-svc-rfl1440m1116` | `DELETE_REQUESTED` |
| `folders/601530101020` | `DELETE_REQUESTED` |
| Parent zone records for the pair | none |
| GKE clusters | removed by destroy |

The parent zone check listed 295 record sets in
`gcp-hcp-commons-dev:dev-gcp-hcp-devshift-net` and found zero whose name
contains `rfl1440m1116`. No shared project, zone or folder outside the
deployment was touched.

## 5. What this run supports

- The six defects in `2026-11-13-dev-pair.md` section 11 did not recur: the
  operator and Argo CD stayed down, PVCs stayed deleted, every wait ended in
  wall-clock time, the Fleet addresses left state before destroy, and the
  versioned discovery bucket emptied before destroy.
- The script ran unattended from the first line to `Teardown Complete`, with no
  plan edit and no `modules/` change. The 2026-11-13 run needed a reviewed plan
  and a temporary edit to `modules/region/cdn.tf`; neither was needed here.
- `teardown-tests.sh` passed 29 cases at the same commit before the run.

Limits of the record: one pair, one region, one run. It does not claim GKE
support, certification, an SLA or approval, and it does not prove the script on
a pair with `deletion_protection = true` or with `prevent_destroy` left on the
OIDC HMAC key. The projects and folder are scheduled for deletion, not yet
purged.
