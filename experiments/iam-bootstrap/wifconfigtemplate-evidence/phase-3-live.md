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
