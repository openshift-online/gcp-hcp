# Phase 3 handoff — complete

**Completed 2026-11-18. Final post-revocation readback: 12:40:07Z.**
The first real installation using the new CLI and deployed Gecko changes passed:
5.0.2 installation Completed, available control plane, one Healthy NodePool, exactly
one actual Ready guest Node, and the exact default Compute worker service account.
Both new-use gates were restored false. **Stop before Phase 4.**

Read [the detailed live evidence](phase-3-live.md) and the progress ledger first.
Earlier Stage A/proposal/ADC checkpoints are historical, not the current live state.

## Retained installation

| Item | Current identifier |
| --- | --- |
| Customer project | `apahim-dev-test-1`, number `35809355916` |
| Region / zone | `northamerica-northeast2` / `northamerica-northeast2-a` |
| Cluster | `apahim-dev-test-1/wif3-first`, UID `6b152684-797c-4f17-a258-43538f8013c8` |
| Primary infra ID | `wif3-7d9e` — already used; do not allocate a replacement with this ID |
| Public NodePool | `workers`, UID `a8768aef-c352-47bb-9f32-3638275f1e15`, nodeCount=1 |
| Management NodePool | UID `8f5326fe-f59f-4560-8c00-a8809da878f2`, replicas=1 |
| HostedCluster | `clusters-6b152684-797c-4f17-a258-43538f8013c8/wif3-first`, UID `d4ba8048-befb-4158-ab78-fb2e07efe873` |
| HostedControlPlane | `clusters-6b152684-797c-4f17-a258-43538f8013c8-wif3-first/wif3-first`, UID `a2bf3c7b-d61b-4f89-969e-7d02194b9835` |
| Worker VM | `workers-7zbj4-94pth`, instance ID `8160493395057321576`, n2-standard-4, 100 GB pd-balanced |
| Guest Node | Same name, UID `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`, Ready=True at 12:26:34Z |
| Worker SA | `35809355916-compute@developer.gserviceaccount.com`, unique ID `115529340171961812045` |
| Guest API | `https://api.wif3-first-user.northamerica-northeast2-apahim1110-1.dev.gcp-hcp.devshift.net` |

The actual guest provider ID matches that VM/project/zone exactly. The guest reports
amd64, Kubernetes v1.36.6, RHCOS 10.2.20261108-0 and CRI-O 1.36.8. TLS/OIDC login
through the public API succeeded as `asegundo@redhat.com`; no insecure TLS or
Workflows shortcut. The Cloud instance read never requested metadata/user-data.

Release: **5.0.2 / candidate**, pinned payload
`quay.io/openshift-release-dev/ocp-release@sha256:7c739d899b33a56060422580779b22bf859d59925ec130d6660d6fc4dc30a2e8`.
Guest ClusterVersion history completed at **12:33:27Z**; Available=True,
Failing=False, Progressing=False. HCP Available=True/Degraded=False and the public
Cluster/NodePool feedback agree. Both IAM dispatch receipts are **Dispatched**, with
no Pending receipt, bound to the exact parent/object UIDs, release/image/group and pin.

## IAM and final gate state

- Template `5.0.0`, UID `fb623828-d923-4540-82f1-df5c01503823`, digest
  `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71`.
  Contract content/UID unchanged; Cluster history has exactly one application.
- Six component SAs, six custom roles, one pool/provider, 15 scoped grants and
  owned VPC/subnet/router/NAT were verified before the initial POST. No legacy
  adoption, undelete, broad-role fallback, SA key or project-wide managed actAs.
- Independent direct policy reads confirmed six intended custom project bindings;
  the default worker has no direct project-policy role binding. Worker-SA actAs is
  granted to only the two intended managed members (CSI and NodePool management).
  This is not a claim about inherited/organization privileges.
- **verified=false; IAM application writes=false.** Twenty-one Versions and 19
  retained pins remain. Version Available=False after revocation is expected for
  new use; the installed guest remained Available with one Ready Node afterward.
- Post-revocation standalone CLI verification still returned cloudReadbackMatches=true
  and eligibleForNewUse=false, without appending records or modifying cloud state.
- Eight API/controller Deployments Ready; migration Job UID still
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1`. No migration replay or Terraform apply.

## Source and artifact authority

| Component | Committed/pushed authority |
| --- | --- |
| CLI | `97f314dac0bb7e7cc86320372c8e92757f56fa89`, `apahim/gcp-hcp-ctl`, branch `fix/wif-federation-create-parents` |
| CLI native artifact | `.../opencode/wif-p3-cli-97f314dac0bb/gcphcpctl`, SHA-256 `cb868c731b5f2fcdaccad0939850c3a9d3718269b22025c609c3036f920b29b0` |
| API / controllers | Unchanged Gecko source `54709c4` / `9f7161a`, and unchanged Phase 2 images/digests below |
| HyperShift API/schema source | `dfe1946a378547bd80e8c1a96a6115c2d8fa521f`, `apahim/hypershift`, branch `fix/gcp-default-compute-worker-email` |
| CRD-only deployment artifact | `914334ae61352fa5641e3f1d50250e87299e9eb3`, retained infra fork branch `fix/dev-default-worker-sa-crd`, parent exactly `3d8a55cb...` |
| Personal dev root | `d405901c2d90375da294d90f5a86d26d05cbad68`, pushed to `origin/dev-apahim`; both new-use gates false |

API image digest remains
`sha256:55df0ecd04b7b7eaebb6d80eb9021d431d803f71ab0cfb1be18fc3ef2b0daa35`;
controller digest remains
`sha256:0c69927dc6976887987e20e6c7c1ce2c123fef8e3a4ed9fb6ce6ed55ff48b355`.
No new server/runtime image was needed. CLI builds were native Go 1.27.4 darwin/arm64,
CGO off, clean source, fixed metadata and two outputs with equal checksums.

The HyperShift fix allows the default Compute email only in the **worker** field;
controller identities retain their custom-only validation. The deployed change is
only the NodePool CRD rule/message: 61 of 62 Argo-rendered objects were unchanged,
all Deployment templates identical, conversion CA bundle preserved. Operator,
external-DNS and cert-manager generations, pod UIDs and restart counts did not change.
The operator image is still `quay.io/acm-d/rhtap-hypershift-operator:d3173a012a...`.

**Keep the CRD source branch/pin.** The MC's `hypershift` Application is pinned to
`914334ae...`; its `gcp-hcp/argocd-app-revision` annotation is intentionally frozen at
`3d8a55cb8c633329d2a64939e7e0ab20520107ee` to prevent an annotation-only rollout.
Do not revert to the old rejecting schema. A future upstream bump must deliberately
carry the fix and replace these temporary pins/stamp controls.

## Recovery, access and eventual cleanup

Protected directory (0700):
`/Users/asegundo/.local/state/gcp-hcp/wif-phase3-20261118`.

- Keep **`primary-state.json`**, operation `30882e34568dbf073aedbcf769ebd103`.
  It contains the ownership proof needed for eventual cleanup; no credentials.
- `guest-kubeconfig.yaml` uses the CLI exec plugin. Put the isolated artifact's
  directory on PATH for these subprocesses; do not replace the user's global CLI.
- The primary is retained per the user's earlier Phase 4 preference. The user waived
  further cost sizing and intends eventual environment cleanup; no unattended
  deletion or shared-infra destruction was authorized or installed.
- Initial automatic-upgrade exclusion expires **2026-11-19T00:00:00Z**. Review that
  timing before Phase 4. With verification false, new dispatch stays blocked anyway.
- Normal Cluster/NodePool deletion does not need a new IAM assertion. Once the test
  work is finished, delete normally, wait for hosted/cloud teardown, then use the
  same journal's `iam destroy --dry-run` and explicit owned-network cleanup. No
  force-finalizer, prefix deletion, journal-less legacy destroy or shared Terraform.
- The rehearsal `wif3-smk1`, operation `0377c5a2df0f9e2b9559ddcd216c5078`, is Cleaned.
  Its one unbound custom role is independently confirmed soft-deleted. Never reuse
  that retired ID. No rehearsal SA, pool, provider, grant or VM was created.
- Unknown submission outcomes must remain journaled; never infer exactly-once from
  a 404, force-unlock or allocate a second installation. Compute network deletion
  still has no atomic numeric-ID CAS; client readbacks narrow rather than close races.

## Boundaries and next phase

- **Do not start Phase 4 automatically.** Next work is a reviewed plan for target-aware
  IAM updates/permission retention and a real blocked/released upgrade using this stack.
- Formal published Gecko module alignment is still needed before treating the CLI
  branch as standalone/merge/release ready. The recorded external workspace maps
  CLI source to Gecko API/Orlop `54709c4`; keep machine paths out of committed modules.
- HyperShift's optional runtime GCP credential-validation conditions report Unknown
  for this control-plane version. Do not mislabel that feature as validated; the CLI
  cloud readback, actual provisioning, VM identity and guest readiness are the proof here.
- ImageRegistry, Console and Ingress remain disabled by the existing HC manifest.
  No full six-component cloud functional matrix, PVC/registry test, actual upgrade,
  sustained-failure alerting or retained-primary teardown is claimed by Phase 3.
- Preserve the original infra checkout's unrelated Spanner/variables/Endpoints edits
  and PDF, the main HyperShift worktree, and the user's untracked experiment notes.
