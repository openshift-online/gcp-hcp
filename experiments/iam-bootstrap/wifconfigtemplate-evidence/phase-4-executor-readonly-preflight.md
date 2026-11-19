# Phase 4 executor — renewed-auth read-only preflight

**2026-11-19. Read-only refresh and both previews passed; stable fingerprint
confirmed across a watched normal status refresh.** The user renewed authentication and
authorized proceeding. This session conservatively limited execution to the
read-only prerequisites; an isolated cloud-role write test still needs explicit
approval of its exact footprint. No primary update, gate activation, assertion,
upgrade or cloud-resource allocation is authorized by this preflight.

## Exact artifact and footprint

- Isolated native CLI `a727640d70f9c1af506f9e922c729edc8f05d4c6`, binary
  `.../opencode/wif-p4-cli-a727640`, checksum
  `bba7d96f0fef9a79581d7a49b7354114b0e0f5dcdb842e063b6541acbbbeb13e`,
  independently rechecked before execution. Source checkout remains clean
  with local-only executor commits; no source/image/GitOps publication here.
- Original protected journal
  `/Users/asegundo/.local/state/gcp-hcp/wif-phase3-20261118/primary-state.json`,
  operation `30882e34568dbf073aedbcf769ebd103`, checksum unchanged:
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`.
  The primary was not opened for mutation or lock acquisition; preview/verify
  only loaded it. No linked update journal was requested.

## Fresh runtime and IAM observations

- Regional and guest credentials work again. At **18:32:26Z**, public/private
  readback confirmed original Cluster `wif3-first` UID
  `6b152684-797c-4f17-a258-43538f8013c8`, generation 2, one original
  `5.0.0` IAM application, and `workers` UID
  `a8768aef-c352-47bb-9f32-3638275f1e15`, generation 1, nodeCount=1.
  Both requirements views were Complete with current generation/UID binding;
  both receipts still Dispatched on exact original 5.0.2 candidate payload and
  template UID/digest, no Pending work. No upgrade was requested or available.
- Template verified=false; deployed API application-write argument=false.
  Twenty-one Versions/19 retained pins remain. Eight API/controller Deployments
  Ready, personal API/HC/NodePool Applications Synced/Healthy. Completed
  migration Job UID `daa14792-1593-4a92-abe8-2b8f32ba3fd1` and creation
  timestamp `2026-11-17T19:25:31Z` unchanged. SpannerDatabase/gecko UID
  `99e4163c-79f9-4875-92b9-0e576b9a5927` Ready=True/UpToDate.
- Actual guest ClusterVersion still 5.0.2 **Completed**, Available=True,
  Failing=False, Progressing=False. Original guest Node
  `workers-7zbj4-94pth`, UID `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`,
  Ready=True with unchanged provider ID. Independent masked Compute GET
  confirmed original VM ID `8160493395057321576`, RUNNING on owned network/
  subnet with exact default worker SA; no VM metadata/user-data was requested.
- HyperShift Application remained Synced/Healthy, source `914334ae...` and
  frozen annotation `3d8a55cb...` intact. Actual HC UID/generation and completed
  5.0.2 history remained unchanged.
- Standalone read-only `iam verify` returned `cloudReadbackMatches=true`,
  `eligibleForNewUse=false`, `updateComplete=true`, `updatePhase=NoUpdates`,
  same operation and original template pin. This **does not** claim any update
  was executed. Direct project and worker-SA policy reads at **18:34:32Z**
  confirmed six expected managed custom-role bindings, no direct default-worker
  project-policy roles, and worker-SA actAs only for CSI and NodePool managed
  members. Inherited/organization privileges were not assessed.
- The original automatic-upgrade exclusion expired **2026-11-19T00:00:00Z**.
  It was not extended or removed; gates remain closed. A new maintenance/
  execution window must be explicitly reviewed before activation.
- Public upstream Cincinnati graph reads at **18:33:48Z** still found no
  forward or conditional target from 5.0.2 in `candidate-5.0` or
  `candidate-5.1`. No target or contract was fabricated.

## Read-only preview observations

1. **18:33:17Z:** exact same-contract 5.0.2/candidate preview passed with
   Cluster RV `368`, NodePool/list RV `325`, original template ref,
   zero permission additions, `newApplication=false`,
   `targetEligibleForNewUse=false`,
   `targetBlock=WIFConfigTemplateUnverified`. Fingerprint:
   `sha256:0b33062c5309643c5fe829307fef12cfac85dd238c03dd847a84cdebce78d806`.
   Payload catalog binding:
   `sha256:d9eb9a675e4d1907b2b79b69d8c44eaa2620be0c569876f952a88080c74e5905`.
   No confirmation/write flag was used. The journal checksum remained unchanged.
2. A real regional Kubernetes watch observed a `MODIFIED` event at
   **18:34:39Z**: Cluster RV advanced to `369`, requirements remained Complete,
   and `lastRefreshTime` advanced from `18:29:39Z` to `18:34:39Z`.
   No spec update or controller restart was requested to cause this event.
3. **18:35:40Z:** second same-contract preview passed with Cluster RV `369`,
   same NodePool/list RV `325`, same unchanged spec/contract identities, zero
   additions and no new application. Fingerprint was exactly
   `sha256:0b33062c5309643c5fe829307fef12cfac85dd238c03dd847a84cdebce78d806`,
   matching the first despite the changed Cluster resource version. New-use
   eligibility remained false with the same unverified-template block.
4. Final independent reads still showed verified=false, the deployed API
   write gate=false, original primary journal checksum unchanged, and **no**
   `primary-state.json.updates.json`. No live apply/confirm, assertion, role
   change, migration replay, Terraform apply or resource allocation was run.

## Next boundary

The isolated real-GCP test requires separately approved scope: at most **one
unbound custom role** in `apahim-dev-test-1`, proposed role ID
`wif4_etag_20261119a` (must be fresh/unused; do not adopt or undelete any
collision), no IAM-policy binding, service
account, WIF, network, VM, Cluster or NodePool. Proposed read-only permissions:
initial `compute.instances.get`, then add `compute.zones.list`. Test fresh and
stale etags, field-mask preservation and a lost-response/readback recovery,
then remove only that proven-owned role, accepting its normal soft-delete
tombstone. None of this was executed by this read-only preflight. Do not use
the retained installation's role names or journal to run that rehearsal.
