# Phase 3 Stage B — proposed live manifest, NOT approved

**November 18 update:** [ADC resume and narrow CLI fix](phase-3-adc-resume.md) supersede
the earlier missing-ADC blocker. Actual read-only preview now completes in 23.9s
using reviewed CLI `53f39da`, with no authority block and expected unverified-template
blocking. Management cost/footprint and safe optional cleanup-smoke preparation still
need completion before this proposal is ready for live approval. No allocation or
live gate mutation has occurred.

The user approved code-first Stage A only and prefers retaining the first validated
Cluster for Phase 4. **No allocation, activation, cost or retention deadline is
authorized by this document.** Phase 3 live acceptance is unstarted.

## Candidate scope and read-only observations

Read-only operator discovery at 2026-11-17T21:33:48Z:

- Candidate customer project `apahim-dev-test-1`, number `35809355916`, ACTIVE.
- Region `northamerica-northeast2`, UP. Compute, IAM, Cloud Resource Manager, DNS
  and Service Directory APIs were already enabled; none was enabled by this task.
- Actual default Compute account
  `35809355916-compute@developer.gserviceaccount.com`, unique ID
  `115529340171961812045`, not reported disabled. No direct project-policy role
  bindings for that member were observed. This is not an effective/inherited-rights
  assessment; do not add broad worker grants to make acceptance pass.
- Observed headroom: total CPUs 2400, N2 CPUs 750, standard disk GB 204800, SSD GB
  81920, in-use addresses 575. These are readbacks, not reservations. Router/NAT
  headroom and actual installation/management capacity still require confirmation.
- Reads used the operator's current gcloud access token, not the candidate CLI's ADC.
  CLI ADC identity/authority and its alignment with Platform API identity remain gates.

Proposed IDs were checked read-only at 21:37:39Z; all listed IAM/network names returned
404. **They are not reserved, and a 404 does not exclude a hidden tombstone.** Recheck
before approval/apply and never silently generate replacement IDs on a failed run.

| Resource | Proposal |
| --- | --- |
| Installation ID | `wif3-7d9e` (fresh proposal only) |
| Cluster / namespace | `wif3-first` / `apahim-dev-test-1` |
| Release | Propose exact `5.0.2`, **candidate** (not stable); authenticated public read at 21:46:19Z confirmed Version UID `a85eeff8-46e2-4876-b04e-87355a132a3d`, generation 2, candidate membership and retained template pin; Available=False/WIFConfigTemplateUnverified is expected while the gate stays closed |
| Contract | Retained template `5.0.0`, UID `fb623828-d923-4540-82f1-df5c01503823`, digest `sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71` |
| Component SAs | Exactly the six template-derived `wif3-7d9e-<component>` accounts in the customer project |
| Custom roles | Exactly six `hcp_776966332d37643965_<logical-role>` project roles; role definitions/permissions come only from the pinned contract |
| Federation | One `wif3-7d9e-wi-pool` and one `wif3-7d9e-k8s-provider`; exact discovered installation issuer, audience `openshift`, mapping `google.subject=assertion.sub`, exact guest subjects |
| actAs | Only on the actual default worker SA, only for template-specified CSI and NodePool management component accounts |
| CLI-owned network | One `wif3-7d9e-network`, `wif3-7d9e-subnet`, `wif3-7d9e-router`, `wif3-7d9e-nat`; candidate subnet `10.0.0.0/24`, subject to routing/CIDR review |
| Firewall | Preserve existing version semantics: for 5.0.0 GA+ the CPO, not CLI, creates its managed worker firewall. The proposed worker-firewall name was absent at discovery |
| NodePool | One `workers`, exactly one steady-state worker, no autoscaling; propose supported n2-standard-4 and 100 GB disk; exact zone and repair/surge ceilings remain approval items |
| Other installation resources | Only the declared HC/HCP/operator workloads and their expected disks, addresses, PSC/LB/DNS/storage resources; enumerate expected quantities/costs before approval |

No new GKE cluster, service enablement, worker account/key, project-wide actAs,
predefined broad component role, Workflows deployment, prefix-runtime change or
non-dev/other-owner modification is proposed.

## Required decisions before any live action

Read-only API compatibility check at 21:46:24Z: the proposed missing Cluster GET
returned HTTP 404 with explicit `kind: Status`, `apiVersion: v1`, `status: Failure`,
`code: 404` (no reason). This matches the candidate's strict resource-absence parser;
bare/proxy 404s do not. Cleanup is additionally bound to this exact API endpoint and
the retained Version UID, never another environment's absence responses.

1. Approve exact customer project, IDs, release/channel, CIDRs, zone, VM/disk size,
   transient worker/repair ceiling and management control-plane resource footprint.
2. Verify platform/CAPG/CPO/applier, OIDC bootstrap/publication sequence, pull secret,
   delegated project access and guest login/readiness-check path. No Workflows shortcut.
3. Validate installer/cleanup authority using the actual CLI ADC principal; validate
   Platform API namespace visibility/create access using its separate identity source.
   Read-only discovery above is not this proof. No automatic IAM-admin/Owner grant.
4. Check custom-role support for every permission, remaining quotas, network routing,
   service-agent/org policy constraints and existing default-worker prerequisites.
5. Review a current priced estimate across customer and management projects.
   **Proposed, not approved:** a USD 100 total incremental spend ceiling; stop
   provisioning after 2 hours or validation after 1 further hour; retain for at
   most 24 hours after validation and never later than **2026-11-19T00:00:00Z**
   without another explicit approval. Proposed responsible operator: apahim.
   These are a planning allowance and manual stop/cleanup terms, not a validated
   price estimate or an automatically enforced billing cap. Management footprint,
   current service rates and cleanup-overrun allowance still need review before
   approval. If approval comes too late for that window, propose a fresh deadline
   rather than silently extending it.
6. Confirm retention for Phase 4 under those explicit terms and the expiry/extension
   runbook. Budget alerts are not hard caps; no background auto-teardown is installed.
7. Approve the initial bounded maintenance exclusion so no unrequested automatic
   upgrade starts during acceptance/retention; no critical override.
8. Approve any separate partial-cleanup smoke. The implementation creates roles
   before SAs, so the narrow proposal is now **one isolated unbound custom role**,
   interrupted before SAs/pools/grants, then deleted normally. This replaces the
   planning example of a first-SA smoke; neither has been allocated or approved.
9. Approve only the personal-dev GitOps verification/assertion-write activation
   window after artifact and server compatibility checks. Restore false/false after
   new use, preserving already-committed installation and its immutable history.

## Recovery constraints to accept explicitly

- An unknown Cluster POST followed by absence is not automatically re-POSTed or
  cleaned up. It needs operator reconciliation; ordinary name uniqueness is not a
  permanent exactly-once receipt across an accepted-then-deleted object.
- Deletion requires normal hosted teardown, namespace/cloud dependency inspection
  and the protected operation journal. No force-finalizer or legacy name-only destroy.
- Compute network deletion is not an atomic numeric-ID CAS; concurrent authorized
  actors must not repoint/recreate the stack while cleanup runs. Extra/refused/unknown
  state stops work and remains explicitly inventoried, never silently deleted.
- The code-first CLI currently uses an explicit pinned development workspace. It is
  not a standalone/release-CI-ready dependency update. Resolve formal module
  publication/release requirements before treating it as a generally shippable CLI.

**Approval gate stays closed** until the missing decisions and fresh prerequisite
readbacks are complete. The next stage is a review conversation, not a live command.
