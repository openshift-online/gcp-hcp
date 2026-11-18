# Phase 3 live execution — in progress

## Authorization and scope

On 2026-11-18 the user explicitly waived further cost sizing, said the environment
would be destroyed when the work is finished, and instructed execution to continue.
The prior cost-sizing approval blocker is superseded. This is not permission for
Owner/IAM-admin grants, policy broadening, insecure bypasses or shared-infra deletion.

Retain the earlier explicit decision to keep the first validated Cluster for Phase 4;
the general eventual-cleanup statement is not treated as cancellation of that decision.
No automatic Phase 4 execution or background/unattended teardown is introduced.

Approved personal-dev test footprint:

- Customer project `apahim-dev-test-1` / `35809355916`, region
  `northamerica-northeast2`; existing dev regional/management platform only.
- Primary Cluster `wif3-first`, infra ID `wif3-7d9e`, exact release `5.0.2`,
  channel group `candidate`; retained `5.0.0` contract/UID/digest unchanged.
- Template-derived six component SAs, six custom roles, one pool/provider,
  exact scoped project/WIF grants and worker-SA-only actAs. Default Compute SA
  `35809355916-compute@developer.gserviceaccount.com` remains the worker identity.
- One owned VPC/subnet/router/NAT, subnet `10.0.0.0/24`; existing CPO firewall
  ownership/version behavior, no resource-prefix/runtime naming change.
- One NodePool `workers`, explicit replicas=1, `n2-standard-4`, 100 GB pd-balanced,
  zone `northamerica-northeast2-a`. Existing autoRepair/maxSurge=1 may temporarily
  replace a worker; no second steady-state pool or autoscaling is requested.
- Temporary isolated cleanup rehearsal infra ID `wif3-smk1`: at most one verified,
  unbound custom role, no SA/pool/provider/grant/VM/Cluster/NodePool. Delete only that
  proven-owned unused role through the journal-based cleanup path.
- Personal-dev GitOps booleans may be enabled in stages, then restored false/false;
  no server image change, migration replay, Terraform apply or service enablement.
- Initial maintenance exclusion through `2026-11-19T00:00:00Z`; no critical upgrade
  override. A later retention/upgrade test must review its maintenance timing again.

## Operational safeguards

Protected fresh journal directory:
`/Users/asegundo/.local/state/gcp-hcp/wif-phase3-20261118` (0700).
No journal or credential is committed. Exact plan/fingerprint and ownership facts
are persisted before mutation; unknown outcomes are not blind-retried or deleted.

Use only reviewed, clean-commit native CLI artifacts. Complete the bounded-role
source/tests/review/build before its live rehearsal. First enable only template
verification while API application writes remain false; enable assertion writes
only for the subsequent initial Cluster/NodePool use. Restore both at exit.

Primary success requires actual hosted control-plane readiness, exact persisted IAM
application/worker identity, one ready NodePool and a real guest Node Ready. A successful
CLI response, saved config or public Cluster object alone is not installation success.

This log will be updated with actual commits, artifact checksums, cloud/object IDs,
test outcomes, cleanup proof and retained resources. No completed live result is
claimed by this initial entry.

## Bounded cleanup rehearsal — passed

- Reviewed CLI `37b05328e4c23a21af00b2c6455a2c08ffb11783`, branch
  `feat/wif-bounded-cleanup-smoke`, committed/pushed after full race/vet and focused
  repeated-pause tests passed. It adds only a narrower `--stop-after-first-role`
  mode, never a verification bypass or partial IAM success output.
- Reproducible native artifact SHA-256
  `0f2d3378d5ce8f955eee7e1c7a80dc82f0cf26bb43584d4c61620677071a1252`, at
  `.../opencode/wif-p3-cli-37b05328e4c2/gcphcpctl`; two clean-source builds matched.
- Dev infra `ed450099` enabled template verification only. At 11:33:24Z Argo API
  Application was Synced/Healthy, verified=true with unchanged template UID,
  `5.0.2` Available=True, and application writes still false. Migration Job UID
  remained `daa14792-1593-4a92-abe8-2b8f32ba3fd1` (no replay).
- Fresh standalone IAM preview produced fingerprint
  `sha256:5a74b4fc1017bd7c93000862471a9ade9f368eac2b345e3f404a846b9431fe8d`.
  Confirm used the same fingerprint, `--stop-after-first-role`, and protected
  `smoke-state.json`. No interrupt race or legacy IAM path was used.
- At 11:34:21Z operation `0377c5a2df0f9e2b9559ddcd216c5078` paused durably at
  `PausedAfterFirstRole`, exactly one Verified step, no VerifiedAt. CLI JSON said
  paused=true/iamVerified=false. The explicitly requested success-config file was
  independently confirmed absent; exit code 0 was not treated as coverage.
- Role:
  `projects/apahim-dev-test-1/roles/hcp_776966332d736d6b31_hcpCloudControllerProject`,
  original etag `BwZgi2RhiXk=`. At 11:34:46Z independent ADC reads verified its exact
  permissions/operation marker, no project binding, and all other 13 planned IAM
  resource names absent (404): no further role, SA, pool or provider was created.
- `iam destroy --dry-run` required and returned cleanupEligible=true. Only then
  `iam destroy --yes` ran through normal realm/identity/dependency/ownership checks.
  At 11:35:11Z journal phase=Cleaned with exactly one Deleted step.
- Independent API read at 11:35:21Z confirmed role `deleted=true`. **Retire
  `wif3-smk1`; never reuse it.** Its soft-deleted role is an expected tombstone,
  not a residual active grant. Default worker SA was not deleted or granted roles.
- Protected plan/result/readback/cleanup-proof files and journal are in the stated
  durable state directory, not committed. The rehearsal created no Cluster,
  NodePool, network, hosted object or VM.

## Primary installation preparation — request-path fix validated

After rehearsal acceptance, dev infra `f490d1a5` temporarily enabled application
writes as well. API rollout completed and the old pod
`platform-api-server-58465dc676-65tkw` terminated before any primary command.

**Before primary IAM allocation**, inspection found an SDK request-path bug:
StartCreate passed a collection as the parent, so the Google SDK would append the
collection twice (`workloadIdentityPools/workloadIdentityPools` and
`providers/providers`). Credential-free HTTP regressions reproduced both bad paths.
The bounded smoke used only a role and is unaffected by this unused federation path.

Both new-use gates were closed through committed/pushed personal-dev GitOps
`13fe70cb` while the fix was tested/reviewed. At 11:41:00Z independent readback
confirmed false/false, zero Cluster/NodePool objects and unchanged migration Job UID.
No primary allocation was attempted with the faulty federation path.

Fix `97f314dac0bb7e7cc86320372c8e92757f56fa89`, clean/pushed branch
`fix/wif-federation-create-parents`, passes the containing location/pool resource
rather than its collection to the unchanged SDK methods. It rejects malformed
names before RPC. Credential-free HTTP regressions verify exact paths/query IDs,
operation identity, ownership markers and provider issuer/audiences/mapping.
Both pool/provider tests failed before the patch; full race/vet and focused race
after the strengthened audience test passed. Required architect approved it.

Two clean native builds matched SHA-256
`cb868c731b5f2fcdaccad0939850c3a9d3718269b22025c609c3036f920b29b0`, binary
`.../opencode/wif-p3-cli-97f314dac0bb/gcphcpctl`, fixed build stamp
`2026-11-18T11:41:30Z`. The adjacent build.json retains the exact workspace,
toolchain and full build command. It supersedes the prior role-rehearsal binary
for primary installation; no server image was changed.

Dev infra `35257dc7` re-enabled the two approved booleans after the fix. At
11:44:29Z Argo was Synced/Healthy, API Ready 1/1 with the real application-write
argument true, verified=true with unchanged UID, migration Job unchanged, and
the previous false-gated pod was fully deleted. Use a **new** primary state file;
never reuse or resume the cleaned rehearsal journal.

## First real Cluster submission — accepted; readiness in progress

The fixed CLI made a fresh executable preview at 11:45:18Z and applied the exact
full fingerprint `sha256:7ce50384ab946de9303efc56654fb305390726a184a27cb5efa24b129041cc89`
with `--confirm` and a new protected `primary-state.json`. It completed in 223.4s
at **11:49:01Z**, exit 0, with no stderr.

- Operation `30882e34568dbf073aedbcf769ebd103`.
- All 14 IAM resources, all 15 scoped grants and the owned VPC/subnet/router/NAT
  steps read back Verified before submission. No predefined broad-role fallback,
  project-wide actAs, SA key, undelete or unowned-resource adoption occurred.
- Fresh post-network IAM readback timestamp `2026-11-18T11:48:58.622836Z` preceded
  the initial API POST. Journal Cluster state=Confirmed, phase=ClusterSubmitted.
- Persisted Cluster `apahim-dev-test-1/wif3-first`, UID
  **`6b152684-797c-4f17-a258-43538f8013c8`**. Public API returned that same UID.
- Independent `iam verify` after submission returned cloudReadbackMatches=true,
  exact retained template reference/digest and the same operation ID.
- Persisted IAM history has exactly the intended `5.0.0` UID/digest reference and
  worker `35809355916-compute@developer.gserviceaccount.com`.
- Gecko placed the installation on `dev-mgt-na-ne2-apahim1110`, resolved exact
  release `5.0.2`/candidate to payload
  `quay.io/openshift-release-dev/ocp-release@sha256:7c739d899b33a56060422580779b22bf859d59925ec130d6660d6fc4dc30a2e8`,
  reported IAMPermissionsReady=True/IAMContractCovered and persisted its exact
  Pending dispatch receipt. Promotion/availability are not yet asserted here.
- Actual HostedCluster `wif3-first` exists in
  `clusters-6b152684-797c-4f17-a258-43538f8013c8`, UID
  `d4ba8048-befb-4158-ab78-fb2e07efe873`; actual HostedControlPlane exists in
  `clusters-6b152684-797c-4f17-a258-43538f8013c8-wif3-first`.
- Initial named-certificate validation waited for `external-api-cert`; certificate
  issuance completed normally. HostedCluster ValidConfiguration then became True,
  ValidOIDCConfiguration=True and reconciliation succeeded. No insecure TLS flag
  or certificate-validation bypass was used.
- At 11:53:06Z independent unauthenticated HTTPS GETs confirmed the exact
  installation issuer's discovery document and same-host JWKS both return 200,
  discovery issuer matches the plan and one public signing key is published.
  No private signing key or token was requested or printed.
- HCP InfrastructureReady=True and GCPResourceLabelsApplied=True were observed.
  Control-plane/etcd/API readiness and guest Node Ready are still being validated.

This is the first actual Cluster using both new CLI and deployed Gecko behavior,
not just the earlier blocked Phase 2 object or local fake-cloud tests. It is **not
yet a successful working installation claim**. The one-worker NodePool has not yet
been created at this checkpoint; wait for control-plane bootstrap before proceeding.
