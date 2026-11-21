# GCP-1288 disposable GitOps lifecycle rehearsal (2026-11-20)

**Status: historical rehearsal record, written 2026-11-20.** This page records
two bounded rehearsals of the OpenShift GitOps legacy-to-operator adoption on
disposable GKE dev fixtures: one on a region cluster and one on a management
cluster. Each run included one fresh operator-managed organizational-login
window on the live operator runtime, attested as passed by the owner, and then
the mandatory post-success rollback to the legacy baseline. This is not
operating guidance, a release or support decision, an
authorization to repeat the work, or evidence for any other target. The reusable
protocol and downgrade limits stay in `gcp-hcp-infra`
[`docs/ARGOCD_OPERATOR_CONFIGURATION.md`](https://github.com/openshift-online/gcp-hcp-infra/blob/main/docs/ARGOCD_OPERATOR_CONFIGURATION.md);
[GCP-1288](https://redhat.atlassian.net/browse/GCP-1288) retains the chronology,
and the sanitized machine-readable evidence sets are team-retained outside this
repository (manifest digests below).

## What was exercised

| Fixture | Run | Outcome |
| --- | --- | --- |
| Region | one legacy→operator adoption with the two corrected handoff phases, one inert sentinel, one fresh operator-managed login window, mandatory retain-and-converge rollback | PASS; final state is the healthy legacy baseline |
| Management cluster | warm forward-only adoption with whole-cluster gates, one inert self-heal probe, one fresh operator-managed login window, mandatory rollback | PASS; final state is the healthy legacy baseline |

Both fixtures were created for this rehearsal on 2026-11-19 and are separate
from the older disposable pair used by GCP-1283. Each run stopped with the
operator namespace and manager absent, every operator writer at zero, and the
legacy writer chain serving. This record claims bounded technical acceptance only; the open
gates are listed at the end.

## Topology, versions and pins

| Item | Value |
| --- | --- |
| Platform | GKE `1.36.4-gke.2377000`, `linux/amd64`; zero HostedClusters and zero NodePools throughout, so no tenant control-plane survival was exercised |
| Terraform | pinned `1.17.1` binary; one fresh saved plan per stage, `-parallelism=1`, exactly two in-place membership updates, no plan reuse |
| Public bootstrap fixture | `04d42a8e6a7435fe356cbc80caa9edecee44d1f9` (`gcp-1288/rehearsal-aio-fix`); final RootSync source `fc05a28311b77436058818d035eeeb3fb9056e8e`/`bootstrap` on both clusters |
| Region adoption content | private catalog `gcp-1288/fixture-handoff-fix` head `fa199c22bdb657ef629a7b3f39a651af33f225dc`, base `a6742de99f0b31389328fe7e3fae27f6d74b5f90`; linear C1 `0a237606…` → C2 `ee94d61f…` → C3 `fa199c22…` (the adoption pin; the fixture later moved to the login-window base below) |
| MC release content | private catalog `gcp-1288/mc-forward-catalog-20261120`, base `37bff2f67087fbf62f1d5a9b304ed5f4988d8c01`, linear C1 `ac76fc96542d30847e682c80af6535387dab6f25` → C2 `bdbc0484249ed8f7c341865c0f442542943547a7` → C3 `25277be8424d616b613c460c2fb4447b4158d111` (C3 tree equals the base tree) |
| Login-window content | on top of base `25277be8424d616b613c460c2fb4447b4158d111` (tree `70ecd4d96dfcb027428974949e06fb0da6d19455`): MC catalog C1 `596a059f…` → C2 `fca02136…` → C3 `65a498ecba3d7350a1e7a2cb931a6b2c73dd7118`; region catalog C1 `1bbb570f…` (empty bridge) → C2 `fc91511c…` (the two reviewed region blobs) → C3 `d3b9c4bc67ea67da71c3b94b3df40a10d0cb6e4a`. Each C3 tree equals the base tree; both catalogs are non-force fast-forwards published as literal OIDs |
| Operator manager | `gitops-rhel9-operator@sha256:f5773d35e496a55999590024d1d8290e916c05d77ebdadcdc3263c29ea1a434a` |
| Operand images | Argo CD components `argocd-rhel9@sha256:9aaeb618197e278fb95f812fed82dc9253fc4843b9e08d16288bb885859b0f1f`; Dex `dex-rhel9@sha256:1294cf5c2d8ba1dcfe199511f1fbf11e219a8c6eedb698e47d1970484056ef60`; cache `redis-7@sha256:8e0c52a2f304473c1f62d1112e116e0e8d541ed9cbeea86f6ece3318146be105` |

Image versions above are served digests observed in the rehearsal, not a
vendor support statement. The release-artifact contract remains GCP-1286, whose
retrospective acceptance, local-chart exception, `linux/arm64` runtime gap and
support scope are unchanged.

## Deterministic sequence

Every stage used a literal reviewed revision, an explicit guard context, and a
saved plan limited to the two fleet membership objects. No stage deleted the
ArgoCD custom resource, the NotificationsConfiguration, or their child CRDs.
The repeatable checks were the existing handoff, CR/NC spec, self-heal, strict
saved-plan and convergence helpers; this record adds no new test framework.

### Region

1. `region-P1`: stop all six legacy editors; inventory stays 109; CR/NC and the
   four ConfigMaps unchanged.
2. `region-P2a`: install the operator namespace and the five child CRDs with
   `deletion: detach`, and stage the source bootstrap Secret through the
   existing ExternalSecret; manager stays at zero; inventory 158.
3. `region-R`: release exactly the two bootstrap seeds (`ArgoCD` and
   `argocd-cm`); inventory 158→156; detach and mutation-ignore metadata retained.
4. Fast-forward the private live ref to the operator stage while every
   controller is stopped.
5. `region-P2b`: start exactly one operator manager, verify the generated CR/NC
   before any metadata retirement, retire tracking on exactly four ConfigMaps,
   accept the natural Synced+Healthy refresh, then resume exactly
   applicationset-controller, notifications-controller and Dex from 0→1.
6. The adoption operation at C2 `Succeeded` at `04:14:51Z`, pruned exactly four
   `PodMonitoring/argocd-*` objects and created their four `gitops-*`
   equivalents; `ConfigMap/argocd-notifications-cm` was reported
   `PruneSkipped` while still tracked.
7. One inert sentinel on a disposable ConfigMap was automatically reverted with
   the same UID; whole cluster 23/23.
8. `region-Q`: manager 0, every writer 0, five detach CRDs; then fast-forward
   the private live ref to C3.
9. Retain-and-converge rollback R0–R5 with the exact baseline ConfigMap hashes,
   then the reviewed plan restored `fc05a283`/`bootstrap`.

### Management cluster

1. `A1`: protection-only stage; the four ConfigMaps gain exactly
   `Prune=false,Delete=false` (plus `deletion: detach` on `argocd-cm`) with
   unchanged UIDs and data, and the inert self-heal probe exists at
   `data.state=declared`.
2. `A2 mc-P1`: both memberships move to the stage revision; six legacy editors
   at zero; inventory 109.
3. `A3 mc-P2a`: five child CRDs served with detach, operator namespace created,
   source bootstrap Secret Ready, manager at zero; inventory 152.
4. `A4 mc-R`: release exactly `ArgoCD/gitops` and `argocd-cm`; inventory
   152→150; UIDs, data and spec unchanged.
5. `A5`: advance the live ref to C2 while every writer is zero; the observed
   Application revision stays frozen at C1.
6. `A6 mc-P2b`: verify the copied pull Secret and all eight ServiceAccount
   references before the manager; start one manager; gate the CR
   exact-per-path and the NC operator-created 11/10 union; retire tracking on
   exactly four ConfigMaps; resume only applicationset-controller,
   notifications-controller and Dex from 0→1 with observed-zero and UID/RV
   preconditions; the adoption operation pruned exactly four
   `PodMonitoring/argocd-*` and created the four `gitops-*` equivalents.
7. `A7`: whole cluster 16/16 before the sentinel. Read-only authorization
   checks returned yes for the probe namespace and no for `default`; no RBAC was
   granted. One UID/RV-guarded patch set the sentinel, which self-healed with
   the same UID in exactly one automated root-application operation at the live
   C2 revision.
8. `A8 mc-Q` → `A9`: manager 0, `depends-on` removed, no RoleBinding depends-on,
   six operand writers scaled to 0, cache retained at 1/1; advance to C3 with
   all writers 0.
9. `A10`: while every writer was still 0 and before the baseline Terraform,
   reconstruct the baseline from live data minus non-baseline keys; convergence
   reported added-only keys 13/0/2/15 and then removed only those keys with
   UID/RV-preconditioned patches.
10. `A11`: the rollback plan returned both memberships to `fc05a283`/`bootstrap`;
    the legacy C3 sync declaratively restored tracking on all four ConfigMaps
    and removed the temporary protections, with UIDs and data unchanged.
11. `A12`: MC 16/16, inventory 109, legacy writers 1/1, operator namespace
    absent; the C3 operation pruned exactly the four `gitops-*` PodMonitorings
    and skipped the retained CR/NC; the root application separately pruned the
    probe. Region stayed 23/23 at inventory 109.

## Fresh organizational-login windows

Each fixture opened one bounded operator-managed window on its live operator
runtime, with the owner performing a normal browser login, a read-only
permission query, logout and re-login, and the mandatory rollback deferred until
the conductor recorded an explicit rollback decision. The owner reported "MC
passed" and "Region passed"; this record carries an **owner attestation only** —
no session payload, token, cookie, subject, group list or issuer header was
captured, and none is reconstructed here. The windows were sequential: MC, its
rollback, then region, then its rollback.

| Window | Operator runtime at handoff | Owner result | Rollback decision (UTC) | Final gate (UTC) | Measured elapsed |
| --- | --- | --- | --- | --- | --- |
| MC | live C2 `fca02136…`, 16/16 at inventory 150, operator writer chain only, catalog C3 `65a498ec…` not live | PASS (attested) | `2026-11-20T20:42:46.787730Z` | `2026-11-20T21:02:25.392167Z` | `1178.604437 s` (19m 38.6s) |
| Region | live C2 `fc91511c…`, 23/23 at inventory 156, operator writer chain only, catalog C3 `d3b9c4bc…` not live | PASS (attested) | `2026-11-20T22:41:55.660040Z` | `2026-11-20T23:00:37.990270Z` | `1122.330230 s` (18m 42.33s) |

The checks asked of the owner were the normal IAP/Dex browser flow against each
cluster's own Argo CD URL, a non-mutating `can-i` permission query (a named
administrator account is configured, and `policy.default` stays
`role:readonly`; that default does not make a named administrator read-only), an
Argo CD logout with the session gone, and a re-login as the same identity. No
real sync was run and no admin-denial result was sought. A default-reader
(non-admin) check was not part of these windows.

Scope limits of this proof: it is the same authenticated browser path a GCP HCP
SRE uses, on the current `gcp1288r1119` fixture, while the operator was serving
Argo CD. It is not a Google-session revocation proof, not a security review of
IAP header handling, and not a test of every role mapping. The historical
GCP-1283 flow on the older `rflores0922` pair stays a separate, older record and
is not substituted for these windows.

Each window's rollback repeated the same mandatory sequence: quiesce the
operator writer chain, advance the private live ref to the legacy-restoring C3,
converge the ConfigMaps back to the recorded baseline while every writer was
zero, restore both memberships to `fc05a28311b77436058818d035eeeb3fb9056e8e` /
`bootstrap` with a fresh two-in-place plan, and let the legacy sync return
ownership declaratively. Both ended at inventory 109 with one legacy writer
chain, the operator namespace absent and the peer healthy — MC 16/16, region
23/23. The region kept its baseline `Prune=false,Delete=false`; the MC's
temporary `C1` protections were removed declaratively by the legacy sync. Each
elapsed interval starts at the recorded rollback decision and therefore excludes
adoption setup and the human login window.

Final source state after both windows: `refs/heads/rehearsal/gcp1288` and the
region login catalog are both at `d3b9c4bc67ea67da71c3b94b3df40a10d0cb6e4a`
with tree `70ecd4d9…` equal to the private base tree; the MC login catalog stays
at its own C3 `65a498ec…`. The private checkout is `25277be8…`, the public
fixture `04d42a8e…` and the runtime fixture `64357203…`, all tracked clean. No
force push, rewind, raw live push, pull request or `main` merge occurred in
either window.

## Retained state, prune graph and ownership

- **Retained intentionally, no delete authorized:** the ArgoCD CR (region UID
  `b0afa7b9-7000-429a-8d29-3b88ae1fb75b`, spec
  `3ffe09fc32d9ee1a447fba16cacdbdb756f6b1e514368c33abd4342ddb798e7d`;
  MC UID `74444d0c-604e-4542-9cb9-be22fc5f2829`, generation 3, spec
  `6bd015b5f18cf1f785e0fcc0a109ca9f9e05efe8674de5b8e780c4dac9a6b182`) and the
  NotificationsConfiguration (region UID `dfffc012-d6d9-42af-ae80-962ebdd8ae24`,
  spec `f3d061cb2a8c12e8eaacfb5f1a0f9a21e5251bc3265b73fd14013be182d975c5`;
  MC UID `61da2b3c-4d3f-40de-97ef-467041ad3c18`, spec
  `1f0f68810a29e47d26a0d497b280cb39f4c35b3e117639046b453238d4bff912`, 11
  templates / 10 triggers). Both keep prune/delete protection; the ArgoCD CRs
  also keep `argoproj.io/finalizer`.
- **35 CR-owned references** on the region (7 ConfigMap, 5 Secret, 8
  ServiceAccount, 8 Service, 6 Deployment, 1 StatefulSet). The MC records eight
  ServiceAccount pull references and three ConfigMap operator ownerReferences
  as a residual.
- **Five detach child CRDs** stay served with `deletion: detach`:
  `argocds`, `notificationsconfigurations`, `namespacemanagements`,
  `rolloutmanagers` and `controllerconfigurations`.
- **Kept in place:** the CR-owned `Service/gitops-server`, the copied registry
  pull Secret, the operator TLS/CA Secrets, the `gitops-redis` cache at 1/1, and
  the operator-created workloads at zero replicas. No CR/NC delete, ownerRef or
  finalizer surgery, forced prune, or manual tracking edit was performed, and no
  "operator retired" or complete garbage-collection claim is made.

Backup prerequisites are the retained objects and source pins themselves: the
CR/NC with their protections and ownerReferences, the five detach child CRDs,
the copied pull Secret and TLS/CA material, and the reviewed public bootstrap
revision. Recovery used those plus the source plan; no cluster snapshot or
state-file restore was exercised or is claimed.

| Direction | Pruned exactly | Skipped | Unexpected |
| --- | --- | --- | --- |
| Region adoption | four `PodMonitoring/argocd-*` | `ConfigMap/argocd-notifications-cm` (still tracked) | none |
| Region rollback | four `PodMonitoring/gitops-*` | `ArgoCD/gitops`, `NotificationsConfiguration` | none |
| MC adoption | four `PodMonitoring/argocd-*` | none | none |
| MC rollback | four `PodMonitoring/gitops-*`, probe via root application | `ArgoCD/gitops`, `NotificationsConfiguration` | none |

Baseline ConfigMap data SHA-256 after rollback: region `argocd-cm`
`1dd939338c22a107041d24bc74573951ab6ca385e5bb62deca952d9b9adbf316`,
`argocd-rbac-cm`
`2120ea8db43e0c7cc7451fee1ac3f940891ee810dc05aff670bbf1ab2a3dd9b3`,
`argocd-cmd-params-cm`
`44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a`,
`argocd-notifications-cm`
`8a9bd41d6bd55f476b22594400f7993152f9c7004d9ffccd3df3a0ea0a4f256e`; MC
`0514f193…`, `2120ea8d…`, `44136fa3…` and `b942bd3e…` respectively. UIDs,
tracking metadata and protection state were re-read, not inferred from a Git
revert.

## Self-heal and recovery evidence

| Probe | Same UID | Restored data | Causal operations |
| --- | --- | --- | --- |
| Region `ConfigMap/default/region-config` | `f42846c3-b706-47aa-909f-571154db978d` | `62ac0cbdede860543330670ac96e96a5200bccbdc2e31fb95a8aa2f0ed89eb02` | 1 automated root-application operation (`04:19:02Z→04:19:03Z`), no manual sync |
| MC `ConfigMap/monitoring/mc-selfheal-probe` | `72de1d4c-ac47-4007-9f67-a527a83e9367` | `80539362fe59437814e1db2d9ffb13e0c71354461610ce211c89930542f31576` | 1 automated Succeeded operation at `bdbc0484…`, `18:53:05Z→18:53:06Z` |

The MC patch second was not persisted, so its window is honestly bounded at
**≤66 s** from the last declared-state read; the checker proves causality and
restore, not the clock. The region rollback and the MC rollback were observed in
runtime: exact baseline data, unchanged UIDs, restored tracking and protection,
and no object loss beyond the declared prune sets.

Recorded staged intervals, kept for completeness and **not** proof toward the
owner-accepted rehearsal objective below: region S1 `03:53:02Z` → adoption Succeeded `04:14:51Z`
(21 m 49 s), adoption Succeeded → Synced+Healthy `04:15:45Z` (54 s), region
post-success rollback `04:25:44Z` → settled `04:39:30Z` (13 m 46 s), region
end-to-end `03:41Z` → `04:39:30Z` (~58 m 30 s), MC final rollback sync
`19:11:35Z` → capture `19:18Z` (~6 m 25 s, capture-bounded). These are warm,
staged, plan-assisted intervals that exclude detection, decision and plan
authoring; the windows above are the prospective measurements and these staged
intervals are not substituted for them. No cold rebuild or full RTO was
measured, and none of these staged intervals is the measured rehearsal
objective.

## Prior failed adoption and recovery

The successful runs followed a recorded failure path, preserved rather than
rewritten:

- **Region attempt v2** reached a real failed adoption. The deliberate-failure
  stage produced an actual `registry.redhat.io` image-pull 401 rather than the
  intended two-grant RBAC fault, so that fault signature was not proven. The run
  then hit a hard stop before legacy restore: the operator had added
  ownerReferences from the CR onto protected ConfigMaps (and other objects) and
  a finalizer on the CR. The initial automatic-prune concern was unverified at
  that stop. Follow-up confirmed that the live `Prune=false,Delete=false`
  protection caused `PruneSkipped`; normal legacy reconciliation would not
  automatically delete the protected CR. Explicit CR deletion still carried a
  cascade risk, so recovery retained the CR/NC and restored configuration in
  place. The stopped region was quiescent, not healthy legacy.
- **Region legacy recovery** then restored the healthy legacy baseline with a
  bounded retain-and-converge sequence: protect the extras, converge the
  ConfigMaps in place while every writer was zero, restore the baseline source
  plan, and observe a new Synced+Healthy operation. One deviation is recorded:
  the CR-owned `Service/gitops-server` was not pruned, leaving a stale tracking
  annotation and 35 CR-owned references instead of 34; no force or surgery was
  used.
- **MC first warm attempt** stopped on publication handling; its report and
  errata are retained, and the accepted forward-only replay used the new catalog
  with a literal-OID live-advance helper. The blocked report's intended-vs-actual
  live-ref strings and approximate timestamps are not evidence.

## Terraform and source discipline

The region run performed six serial applies (recorded serials 44 through 49);
the MC run entered at serial 56 and performed six further applies. Every plan
was `0 add / 2 in-place / 0 delete / 0 replace` with no unknowns and only
`sync_rev`/`policy_dir` under the Config Sync membership. Both fleet specs were
read twice after apply and twice after settle at every stage. Lineage
`8b90b0de-c115-458a-d506-ca23c80fe026` was unchanged, the restored tfvars digest
is `6d3d001d80f2f32d3f0908910e110b978314338338a68dc42dccfa669e0b37f8`, and the
MC final serial was deliberately not re-read, so linear serial 62 is **not**
claimed. The two login-window rollbacks added two further applies each — the
quiesce revision and the baseline restore — again as fresh two-in-place plans
(MC `d5f872df…` then `f1be190f…`; region `544c23b2…` then `603d188e…`) with
2+2 membership reads; no new linear serial is claimed for them either.

During the accepted runs and login windows, Git refs advanced only by
non-force fast-forward to reviewed literal commits;
no force push, rewind, raw live push, pull request or `main` merge occurred. The
read-only fixture deploy key was neither used to push nor widened.

## Limits, owner decisions and separate work

Recorded limits and the remaining open items:

- **Organization login (owner-attested pass, no captured payload).** Both
  current fixtures ran the fresh operator-managed window described above —
  normal browser login, non-mutating permission query, logout and re-login —
  and the owner reported pass for each. Nothing about the session was captured,
  so this is an attestation rather than an artifact: no subject, issuer host,
  group list, token or cookie is recorded, and none should be re-created. The
  historical GCP-1283 flow on the older `rflores0922` pair is **not** a
  substitute and is not counted again here. These windows used the named
  administrator; the default `role:readonly` does not make that account
  read-only, no real sync or expected admin denial was run, and default-reader
  behavior is still unproven.
- **Recovery objective (owner-accepted rehearsal objective).** The owner
  accepted a 30-minute rehearsal target on 2026-11-20, after reviewing these
  measured runs: the clock starts at an explicit recorded rollback
  decision, before quiescence and any rollback plan/apply, and ends when legacy
  GitOps management is healthy — intended RootSync/inventory, all baseline
  applications Synced+Healthy, one legacy writer chain, baseline ConfigMap
  UIDs/data/tracking/protections, and a healthy peer. It includes quiescing,
  fresh plan/gate/apply, configuration restore, declarative ownership return and
  health validation; it excludes initial adoption setup and the human login
  window. Both login windows measured it prospectively from the recorded
  decision — MC `1178.604437 s` (19m 38.6s), region `1122.330230 s`
  (18m 42.33s) — so both finished inside the accepted target. Acceptance came
  after the runs and does not restate them as pre-agreed, and the target is a
  rehearsal objective only, not a production availability SLO or tenant RTO;
  two in-target rehearsal runs do not prove other runs, other clusters or
  a guarantee.
- **Lifecycle evidence pointer (AC5).** AC5 asks for repeatable tests and
  operational runbooks that record sanitized upgrade/recovery evidence, backup
  prerequisites, version limits and remaining risks, plus a durable published
  pointer for the rollout story. This record, with the reusable protocol in
  `gcp-hcp-infra`, is offered as that dated pointer and carries the tested pins,
  release-contract limits and support limitations honestly. It becomes durable
  only when merged, so it does not close AC5 by itself, and GCP-1286's Review
  status alone is not a precondition for it. Vendor support and certification
  remain GCP-1444.

Not proven, and not to be inferred:

- Anything beyond the owner's pass/fail attestation for the login windows: no
  session artifact, no default-reader (non-admin) behavior, no IAP header
  security conclusion.
- Any notification delivery during these windows. GCP-1284 (closed) proved real
  delivery on its own historical target; this rehearsal preserved configuration
  only.
- Tenant control-plane survival (zero HostedClusters/NodePools) and GKE 1.35.8
  parity.
- Operator retirement, complete garbage collection, cleanup completion or a
  persistent/global rollout. The fixtures were torn down under the recorded
  authorization, but a project or folder delete request is a scheduled,
  recoverable deletion, not a purge, and no complete-absence, permanent-erasure
  or billing-stop claim is made. See the cleanup status below.
- Any change to GCP-1289 rollout, GCP-1287 optional components (no distribution
  project before Preview), or the standalone GitOps Promoter v0.49 placement,
  APIs, gates, dashboards, TLS, credentials and single-writer arrangement; the
  operator Promoter and Hydrator remain off.

## Cleanup status and retained credentials

The authorized teardown ran on 2026-11-20–21 in the recorded order. The 14
exact recorded IAM bindings were removed before the fixture teardown; the three
fixture projects and their folder were then issued `DELETE_REQUESTED`, which is
a scheduled, recoverable deletion and not a purge; and exactly nine shared DNS
records were removed. The fixture Terraform state prefix was cleared afterwards:
178 live and non-current object generations were deleted by explicit name, every
call returned success, and a fresh listing showed none remaining. Those copies
sit in the provider's seven-day soft-delete window and stay recoverable until
2026-11-28; no retention or soft-delete setting was changed, and no
secure-erasure claim is made. Locally, the generated plan, state, credential,
kubeconfig and cache files were removed (84 files, plus the Terraform working
directory and lock file, with a captured exit code of 0), while diagnostic logs,
derived summaries and per-phase receipts were kept to preserve context.

Two absences are **not** independently proven: the Endpoints service answers
with an API-disabled 403 on the surviving quota project rather than a not-found,
and the GKE clusters can no longer be described once their projects are
deleting. The delete requests were acknowledged; an acknowledged request is not
a verified-absence receipt.

By explicit owner direction, the fixture's registered read-only deploy key and
its original local keypair are **retained** unchanged for possible follow-up and
were not revoked, deleted, re-registered, widened or re-probed. This is a
recorded exception to the standing advice that temporary credentials should be
removed after disposable work; it does not weaken that default for other runs.
No fixture permission, service API or resource was re-enabled or recreated for
the follow-up.

Sanitized cleanup manifests (SHA-256 over each file list); detailed ledgers
remain team-retained outside this repository:

| Cleanup stage | Manifest |
| --- | --- |
| Authorized teardown (grants, DNS, scheduled deletes) | `00fe07e96470dbd72035b74d3d0052d2b7e1a7c846bb670f5ba6602a25a1fcc5` (4 files) |
| Remaining state and local artifacts | `a497e5d74e7a120f2954a069ccb98f3a53507d8aed1a71189b0a5dcccaf6b90f` (7 files) |
| Bounded Endpoints absence re-check | `f2f797027e95207d967951951bfc46b8e3ef5b661b8e9edea6c066ff827139fd` (2 files) |

## Evidence pointers and hash semantics

This repository-held record is the durable sanitized summary. The digests
below identify detailed reports retained by the conductor outside the
disposable cloud projects. This change does not publish those reports or
attach them to Jira; request access from the GCP-1288 owner. Raw runtime
captures are not included. A digest identifies evidence; it is not a
download link or proof that the full report has been published.

| Evidence set | Manifest | Members |
| --- | --- | --- |
| Region adoption + rollback | `c11753472e8cc48dffb29b109177488f48731008a177eb61d0128999ec11f36b` | 6 files |
| MC forward retry (accepted) | `bb4c5f49464d31c3fb538bb62531d99d3ab707dab3360cc691c03cd70fba2c34` | 7 files, including its errata |
| MC login window (READY handoff) | `d18b389acf8458df5efa2b72127af3c1deb241195d3e66a75a7f39787819c187` | 5 files |
| MC login-window rollback | `8270fb9c94cde212c96d028027f06d081ff1dc433c83ebef90ef4d747db09c54` | 4 files |
| Region login window (READY handoff) | `6b38306ac0542e8122bcf565744ce5831f5abd9b3b3016fb5a3e13cc3cc585be` | 5 files |
| Region login-window rollback | `6fc0bf197ed128306fe0de348474c650fd0cace637798bdd07e121d120cf314f` | 4 files |
| Corrected closeout preparation | `5130b23a2dcd0fe0cc663c44d85cd81a3c046c31ed543c403e1021b371e9df1e` | 4 payload files |

The corrected closeout manifest supersedes the frozen `f6b26376…` closeout,
which contained an incorrect "same disposable targets" claim for the GCP-1283
comparison; the frozen copy is preserved but must not be used as this record's
source. Each manifest is a SHA-256-of-`SHA256SUMS` digest over the files it
lists; the member count is not a byte count.

Three values that look like digests are different objects and algorithms, and
their difference is not drift:

- `637583656c50750f406283356d36db13950bab12` is the Git **tree OID** of the
  public `bootstrap` subtree at `04d42a8e…` (17 files), not a file digest.
- `cc0817fb780eeb137bb325140d8fb8559da4346a76edaa2053e4a76d0ab8c57e` is a
  SHA-256 digest of a 17-file list, not a Git object id.
- `815dfb43f1123e737b573de6d3044d395a4edba672b93a6d3615d940f936fc38` is the
  rehearsal/region partner digest used by the staged trees.

All other 40-hex executable pins in this prose are Git commit or tree ids as
labelled; 64-hex values are SHA-256 over file bytes or over a defined manifest,
as stated in each table.
