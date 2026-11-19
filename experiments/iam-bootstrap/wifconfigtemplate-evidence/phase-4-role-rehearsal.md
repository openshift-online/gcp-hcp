# Phase 4 isolated custom-role rehearsal — passed and cleaned

**Executed 2026-11-19; independent cleanup proof at 21:05:59Z. Phase 4
remains incomplete.** The user explicitly approved one fresh unbound role
`wif4_etag_20261119a` in `apahim-dev-test-1`, initial permission
`compute.instances.get`, addition `compute.zones.list`, stale-etag and
lost-response/readback tests, and deletion of only that proven-owned role.
No primary IAM update, API assertion, gate activation or upgrade was authorized.

## Source, review and protected state

- The exact bounded harness is archived at
  [`tools/role-rehearsal/main.go`](tools/role-rehearsal/main.go), source SHA-256
  `3e7fedf2dcee3192b22b1ca91d82dd585d3099adef32d2ee4b5323a48fd4716d`.
  It was built/reviewed/executed from the approved opencode temporary directory;
  the archived source is byte-identical. Harness source authority at execution
  was the pre-recorded hash plus architect review, **not a prior harness Git
  commit**; the shipping adapter itself came from clean committed source.
  This evidence commit archives the harness afterward and does not retroactively
  claim a committed-harness build or full phase delivery. **The role ID is now retired: do not
  rerun writes or change the constants without a fresh scoped approval.**
- The harness imports the reviewed CLI's `application.GCP` and Google IAM
  adapter from clean `a727640d70f9c1af506f9e922c729edc8f05d4c6` using the
  existing external workspace. Native `CGO_ENABLED=0 go build -mod=readonly
  -trimpath` and `go vet` passed. Executed binary SHA-256
  `3ebc5c178e59b194fd1896afdcb43f7c2536f618dce499402101919bd098b918`,
  local path `.../opencode/wif-p4-role-rehearsal/rehearsal` (local-only,
  not a released CLI or registry image).
- Required architect review approved **only** this isolated one-role rehearsal
  before live writes, including scope, owner/etag validation, failure cleanup,
  journal intent and simulated response-loss limitations. No change to the
  shipping CLI/core executor or runtime source was needed.
- Protected state directory (0700):
  `/Users/asegundo/.local/state/gcp-hcp/wif-phase4-role-20261119`.
  Journal `role-state.json` (0600), operation
  `70b7a50c3921997d2da1513e82e10966`, final phase **Cleaned**. Final
  journal SHA-256
  `dbd6910cdd1d3bd3589c7102f2f5bca28e220861f22a1f3b4564ce241ba293f6`.
  The journal has no credentials and is retained locally, not committed.
- Authentication used authorized-user ADC with no impersonation. Independent
  Cloud Audit Logs identify actual mutation principal **asegundo@redhat.com**.
  No credentials, tokens or raw provider error bodies were printed/committed.

## Actual proof chain

| Stage | Actual result |
| --- | --- |
| Read-only preflight | GET returned 404 for the exact role ID; no adoption/undelete. Customer project number independently matched `35809355916`. Existing caller create/get/update/delete and project-policy-read authority and custom-permission support checked, without new grants. No project-policy binding to this name. |
| Create, recorded 21:04:59Z | One role at `projects/apahim-dev-test-1/roles/wif4_etag_20261119a`, GA, initial permission only. Description owner marker `gcphcpctl/wif4-role-rehearsal/v1/70b7a50c3921997d2da1513e82e10966`; exact title and owner independently read back. Initial etag `BwZgp3rvIXE=`. CreateIntent was saved before the RPC. |
| Real additive update | The shipping `application.GCP.UpdateRolePermissions` path re-read owner/stage/etag and used only the `includedPermissions` field mask. It added `compute.zones.list`, retained `compute.instances.get`, and preserved title, owner description and GA stage. UpdateIntent was durable first. |
| Deliberately lost result | After that real successful API call, the harness **discarded its success** and exited 1 while the journal remained UpdateIntent. The expected failure was not treated as an actual failed/no-effect RPC. This is simulated post-success response loss, not a real network outage. |
| Separate-process recovery, 21:05:16Z | A new process GET-only read the exact role/owner and both permissions; etag advanced to `BwZgp3uKJ9w=`. Recorded Recovered without a second successful permission update. |
| Stale etag, 21:05:24Z | The underlying reviewed SDK method intentionally sent the old etag with the **same approved permission union**. Real GCP rejected it with **HTTP 409**. Independent GET showed the role's updated etag, title, description, stage and permissions unchanged. No broader permission was even requested. |
| Owned cleanup, 21:05:33Z | Independent exact-name/owner/title/stage/permission/etag reads and project-policy unbound check preceded DeleteIntent. One delete used `BwZgp3uKJ9w=`. GET confirmed `deleted=true`; journal became Cleaned. |
| Independent post-cleanup proof, 21:05:59Z | Separate Python/HTTP IAM GET, not the harness's cached state, confirmed the same owned role soft-deleted and zero matching project-policy bindings. The normal soft-deleted tombstone is expected; do not undelete/reuse this ID. |

Independent Cloud Audit Logs (actual backend timestamps) corroborated exactly:

| Timestamp | Method | Status |
| --- | --- | --- |
| 21:04:58.079335169Z | `google.iam.admin.v1.CreateRole` | Success |
| 21:05:08.394976564Z | `google.iam.admin.v1.UpdateRole` | Success |
| 21:05:24.660057694Z | `google.iam.admin.v1.UpdateRole` | RPC code 10 (ABORTED), matching HTTP 409 |
| 21:05:32.293412273Z | `google.iam.admin.v1.DeleteRole` | Success |

The journal's `permissionUpdateCalls=1` counts the one intended successful
update; the rejected stale-etag test is tracked separately by
`staleEtagStatus=409`. There was one create and one delete. Neither the harness
nor this session wrote an IAM policy binding. "Unbound" was directly checked
against project policy; all resource-level/inherited policies were not exhaustively
enumerated. The role name was independently absent before allocation, and no
binding mutation was ever sent.

## Retained-primary safety readback

- Both gates stayed closed: `5.0.0` verified=false and actual deployed API
  argument `--enable-iam-application-writes=false`. Primary Cluster remained
  UID `6b152684-797c-4f17-a258-43538f8013c8`, generation 2, one original
  application and same Dispatched 5.0.2/candidate payload/ref, no Pending
  receipt and no requested upgrade. Requirements remained Complete.
- Post-cleanup standalone read-only CLI `iam verify` independently returned
  `cloudReadbackMatches=true`, `eligibleForNewUse=false`,
  `updateComplete=true`, `updatePhase=NoUpdates`, original operation and
  template pin. This does **not** mean a primary update was run.
- Guest 5.0.2 history remained Completed; Available=True, Failing=False,
  Progressing=False. Original guest worker `workers-7zbj4-94pth`, UID
  `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`, remained Ready=True.
- Original primary journal SHA-256 stayed
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`.
  No primary `.updates.json` sidecar exists. Migration Job UID
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1` and original creation timestamp
  `2026-11-17T19:25:31Z` remained unchanged and Complete. No Terraform apply,
  new SA/WIF/network/VM, Cluster/NodePool or migration replay occurred.

## Limits and remaining Phase 4 gates

This proves real GCP role authority, narrow field-mask preservation, updated
etag behavior, stale-etag rejection, direct readback recovery after a simulated
lost success, and owned cleanup **for one isolated role only**. It exercised
the shipping GCP adapter, **not** the complete `iam update --confirm` command
or its linked installation journal against live Cluster assertions. No live
abandon-unstarted test, actual process/power-loss crash or real network outage
is claimed; those remain local/fault-injection coverage where applicable.

Deployed API UID/resource-version admission checks still need an explicitly
approved safe test before the first assertion write. A real distinct-contract
blocked/released upgrade still needs a supported Cincinnati edge, exact target
and permissions, maintenance timing and separate approval. Do not enable gates,
mutate/repin existing contracts, force an edge or start an upgrade automatically.
