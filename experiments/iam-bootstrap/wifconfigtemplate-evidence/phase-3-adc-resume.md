# Phase 3 preflight resume — ADC works, permission-query fix validated

**2026-11-18, 11:20–11:25 UTC.** The user configured ADC and requested continuation.
Only read-only live checks and a reviewed CLI correctness fix were performed.
No allocation, gate activation, cleanup smoke or Cluster/NodePool POST occurred.

## Credential and cloud readback

- ADC is `authorized_user`, quota project `apahim-dev-test-1`; fresh token/user-info
  read identified verified user `asegundo@redhat.com`. No credential contents were
  printed, committed or included in the operation evidence.
- Cloud Resource Manager returned project number `35809355916`.
- Default Compute SA still exists in that project, unique ID
  `115529340171961812045`, not reported disabled. No direct project-policy bindings
  for that SA were observed; this does not assess inherited/organization grants.
- Read-only `testIamPermissions` returned all **32** tested installer/cleanup/network
  permissions. This proves those named tests, not unlimited permissions or future
  runtime success under every organization policy.
- Region `northamerica-northeast2` is UP; proposed zone `northamerica-northeast2-a`
  offers `n2-standard-4` (4 vCPUs, 16 GiB). N2 CPU headroom 750, total CPU 2400,
  disk headroom 204800 GB, in-use address headroom 575 at read time.
- Region router list is empty; only the customer project's auto-mode `default`
  network exists. No new network was created. These are not reservations or a
  complete validation of every router/NAT quota and routing dependency.

## Actual CLI bug found, reproduced and fixed

The original binary's first 100-second bounded dry-run timed out. A subsequent
bounded read-only run finished in 134.3 seconds with an unsupported-permission
failure, before any mutation. Independent ADC queries found all 114 distinct
contract permissions over 15 pages using page size 1000. The API normally omitted
`customRolesSupportLevel` for its default enum value, SUPPORTED.

The old CLI incorrectly required the literal strings SUPPORTED or TESTING and
used the API's default 100-item pages for approximately 15,000 permissions.
An omitted default is not evidence that the permissions are unsupported.

Official protocol: [permissions.queryTestablePermissions](https://cloud.google.com/iam/docs/reference/rest/v1/permissions/queryTestablePermissions):
SUPPORTED is the default state; page size defaults to 100, maximum 1000.
The installed Google Go SDK's Permission documentation states the same default.

Narrow CLI fix **`53f39da01a84dde59480093020a16b9395ff133a`**, clean/pushed branch
`fix/wif-custom-role-default-support`, based on `bf158ab`:

- Recognize omitted/default SUPPORTED, explicit SUPPORTED and TESTING.
- Still reject missing permissions, NOT_SUPPORTED, unknown future enum strings,
  predefined-only permissions and now explicitly API-disabled permissions.
- Read 1000-item pages, preserve continuation, and bound the query to 90 seconds
  without extending a caller's shorter deadline/cancellation.
- No role definitions, scopes, grants, template fields or server behavior changed.

HTTP regression tests failed against the old source before the fix. They cover
omitted/explicit support, testing/unsupported/unknown values, API-disabled and
predefined-only permissions, missing results, pagination and cancellation, with no
real credentials. Required architect approved the narrow change; gofmt was then
applied. Full `make test` (`go test -race ./...`), `make lint` (`go vet ./...`) and
whitespace checks passed before commit.

## Replacement isolated artifact and real read-only preview

- Go 1.27.4, darwin/arm64, CGO off; same pinned external workspace and Gecko
  API/Orlop `54709c471cdce451119eea69b69334224131ea48` as Stage A.
- Two clean-source builds matched SHA-256:
  `d9587bc2f68bbd7b2fb47ff38d8ad3d2fb2ab9f0eaf1ac760651ccceb7411d02`.
- Binary:
  `/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode/wif-p3-cli-53f39da01a84/gcphcpctl`.
- Adjacent `build.json` records the complete build command. It uses the Stage A
  `-mod=readonly -trimpath -buildvcs=true -s -w` recipe with fixed version
  `wif-phase3-53f39da01a84`, full commit above and date `2026-11-18T11:22:51Z`.
- Version smoke and checksum verification passed; no global CLI was overwritten.
- Use this replacement artifact, not the superseded `bf158ab` binary. Published
  module alignment remains a separate merge/release dependency.

At **11:23:41Z** the replacement binary completed the real integrated read-only
preview in **23.9 seconds**, exit 0, no stderr:

```sh
gcphcpctl cluster create wif3-first \
  --setup-infra --infra-id wif3-7d9e \
  --version 5.0.2 --channel-group candidate \
  --project apahim-dev-test-1 --region northamerica-northeast2 \
  --environment apahim1110.dev --vpc-cidr 10.0.0.0/24 \
  --maintenance-exclusion-until 2026-11-19T00:00:00Z --dry-run --v=0
```

- `executable=false`, expected `WIFConfigTemplateUnverified` target block.
- Empty authority block; 14 IAM resources (six roles, six SAs, one pool/provider),
  15 scoped grants. Exact worker identity, federation and network preflight passed
  this CLI's read-only checks. No journal was created.
- Full review fingerprint:
  `sha256:7ce50384ab946de9303efc56654fb305390726a184a27cb5efa24b129041cc89`.
- IAM fingerprint:
  `sha256:ce7a06985c4bfd53a6ea617c5abf67f1c3f84dd2ff58ed91f81d6d54877b364b`.
- The complete local plan is in `.../opencode/wif-p3-stage-b-preview-fixed.json`.
  This preview is not approval to apply it; re-resolve before any authorized use.

## Additional readiness observations and remaining gates

- Management HyperShift operator Ready 2/2, image
  `quay.io/acm-d/rhtap-hypershift-operator:d3173a012ac0a2b188095aaa6722438b33ff7602`.
- `gcp-secret-manager` ClusterSecretStore and `public-issuer` ClusterIssuer are Ready.
- HC controller selects `default-openshift-pull-secret-ci`. The secret store points
  to **`gcp-hcp-commons-dev`**, not the management project. Metadata-only query there
  confirmed an enabled version; secret payload was never requested. A first lookup
  in the wrong project was discarded rather than treated as a missing prerequisite.
- Actual guest login, OIDC publication/bootstrap and future CAPG/CPO behavior remain
  installation acceptance items; Ready prerequisite services alone do not prove them.
- The NodePool CLI defaults to TWO workers; any approved live invocation must pass
  `--replicas 1` explicitly, plus explicit zone/machine/disk flags. The current
  dispatcher enables autoRepair and maxSurge=1: one steady worker is not a guarantee
  of zero temporary second worker. No controller setting was changed to hide that.
- Cloud Billing catalog read returned 403; no API was enabled and no billing access
  was broadened. Public pricing pages were read, but default-region tables are not
  a validated Toronto/project quote. Management footprint and a priced all-in
  estimate still need completion/review; the proposed USD 100 remains an allowance,
  not an approved or automatically enforced cap.
- Safe deterministic stop for the optional one-unbound-role cleanup smoke remains
  a prerequisite before allocating that fixture. Do not race an interrupt.

Final read-only state at **11:24:54Z**: eight Deployments Ready; verified=false;
assertion writes disabled; zero Cluster, NodePool, HC and HCP objects. No IAM, network
or hosted resources were allocated. No live template mutation, rollout, Terraform,
service enablement or permission grant was performed.

**Resume:** ADC is no longer the blocker. Finish the outstanding scope/cost/smoke
preparation and obtain separate approval of the [live manifest](phase-3-live-proposal.md)
before any live mutation. Do not treat the preview's exit 0 as installation success.
