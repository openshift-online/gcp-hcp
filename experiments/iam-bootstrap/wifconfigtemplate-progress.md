# WIFConfigTemplate implementation progress

## Resume checkpoint

**Phases 1–3 are complete. Phase 4 code-first work has started but is incomplete. First real 5.0.2 Cluster/one-worker installation passed, including actual guest Node Ready and completed release feedback. Both new-use gates are false; the validated stack is retained. No upgrade execution or IAM update is authorized by the code-first checkpoint.**

Read the [Phase 3 completion handoff](wifconfigtemplate-evidence/phase-3-completion-handoff.md) and [live evidence](wifconfigtemplate-evidence/phase-3-live.md). Final readback at 2026-11-18T12:40:07Z confirmed guest 5.0.2 Available, one Ready Node, healthy NodePool, both receipts Dispatched, both gates false, eight API/controller Deployments Ready and 19 pins retained. The user waived further cost sizing; retain the validated primary for Phase 4, not a shared Terraform teardown. Keep the protected primary journal and the CRD-only source pin/annotation freeze.

## Agreed delivery contract

Each phase requires reviewed source commits, recorded build artifacts, a dev-only deployment, observed test results, and a committed handoff before advancing. A blocked required check keeps the phase incomplete. Preserve unrelated tracked, untracked and ignored work.

| Phase | Scope | Status |
| --- | --- | --- |
| 1 | Persisted WIFConfigTemplate/Version/Cluster APIs and opt-in GitOps catalog packaging | Complete: reviewed/committed source, native image, dev deployment, live negative/toggle/reapply tests, API-restart + independent Spanner persistence, clean Terraform post-plan and durable handoff |
| 2 | Template resolution, prerelease numeric-base matching, Version availability and exact-contract installation/upgrade dispatch gates | Complete: reviewed source/native builds/dev rollout; GCP-1452 repaired and redeployed; live missing-IAM/zero-Apply proof, unchanged-generation recovery, ordinary blocked deletion and producer restart/pin persistence passed; final safe state restored |
| 3 | CLI preview/application/readback, owned cleanup and first fresh Cluster | Complete: reviewed/pushed/reproducible CLI; live one-role owned cleanup; independently verified IAM/network before initial POST; HC/HCP available, real TLS login, 5.0.2 history Completed, Healthy one-worker NodePool, actual Node Ready with exact default SA. CRD-only compatibility fix applied without runtime rollout. Both gates restored false; post-revocation guest/IAM proofs pass; primary retained and handoff committed. |
| 4 | Target-aware IAM updates, safe running-permission retention and actual blocked/released upgrade | In progress, code-first only: read-only template CLI and pure retention preview committed. Gecko API and controller source for read-only public running/pending requirements is reviewed, tested and locally committed, **not deployed**. Update executor/journaling and actual blocked/released upgrade remain incomplete; see [Phase 4 partial checkpoint](wifconfigtemplate-evidence/phase-4-partial-checkpoint.md) and [requirements code checkpoint](wifconfigtemplate-evidence/phase-4-requirements-code-checkpoint.md). |
| 5 | Monitoring, sustained-failure alerts and operational recovery | Not started |
| 6 | Full runtime functional matrix, restart/upgrade regressions and independent owned teardown | Not started |

## Confirmed design decisions

- Fresh Clusters only; no migration or legacy exemption machinery.
- A single mutable `spec.verified` boolean controls new-use eligibility. Omitted/false is unusable. GitOps explicitly patches true; there is no automatic promotion or second readiness flag.
- Structural completeness is admission validation, not an eligibility field. Executable IAM and schema version are immutable. The canonical digest excludes verification and metadata.
- Template names are canonical numeric `X.Y.Z` release boundaries. Selection is a same-major/minor numeric floor. Prereleases explicitly inherit their numeric `X.Y.Z` base without changing the actual release identity.
- Initially use the project's exact default Compute SA, with exact-SA actAs only. Do not create/delete the default SA or add broad project grants to it.
- References pin template name, UID and contract digest. A Version reference is set once. Cluster applications are immutable append-only exact-contract assertions, not Gecko-verified cloud attestations.
- New assertion writes remain disabled behind a server rollout gate until Phase 2 has deployed all compatible APIs/controllers and dispatch prerequisites. This is not an additional template readiness field. Rollback to old binaries after the first assertion/pin is unsupported.
- Assertion writes remain disabled at Phase 2 completion; later activation requires separately agreed CLI/cloud verification prerequisites, not merely a verified template.
- Initially deny template deletion entirely; setting verified false disables new use without deleting retained contracts or disrupting already-running work.
- The typed baseline preserves all six component identities, permissions, guest subjects and exact-worker-SA actAs. GitOps is the catalog; no embedded fallback or prefix-runtime changes.

## Selected checkouts and operational boundaries

- Gecko implementation: `/Users/asegundo/git/osd/gecko`, branch `fix/GCP-1452-cel-integer-decoding` at `54709c471cdce451119eea69b69334224131ea48`; Phase 2 controller source remains its compatible parent `9f7161a`.
- CLI: `/Users/asegundo/git/osd/gcp-hcp-ctl`, clean/pushed `fix/wif-federation-create-parents` at `97f314dac0bb7e7cc86320372c8e92757f56fa89`; follow-ups include bounded cleanup rehearsal and corrected SDK parent paths. Use the newest native artifact/checksum in the live execution log. External workspace still required; do not use a legacy broad-role binary or claim standalone release readiness.
- Deployment artifacts: `/Users/asegundo/git/osd/gcp-hcp-infra-dev-main`; preserve Spanner and the pruning safeguards.
- HyperShift worker-email schema fix: `/Users/asegundo/git/osd/hypershift-wif-worker-sa`, clean/pushed `dfe1946a378547bd80e8c1a96a6115c2d8fa521f`, based on the exact running operator source. Its CRD-only infra artifact lives in `/Users/asegundo/git/osd/gcp-hcp-infra-wif-worker-sa` at `914334ae...`; keep the fork branch and personal-MC source pin/stamp until a deliberate upstream refresh carries the fix.
- Images: locally built API/controller artifacts in `quay.io/apahim/gecko`, using unique component-specific tags and recorded resolved digests.
- Do not use/repair stale `.worktree/dev-apahim` or `.worktree/dev-asegundo` Git links. Do not modify unrelated Terraform changes in the original infrastructure checkout.
- Phase 1 has no customer IAM changes, hosted Cluster/NodePool creation or upgrade execution. Cluster record behavior is tested using local API integration fixtures only.
- Always use a dedicated Python virtual environment for Python commands, package installation and infrastructure rendering. The phase evidence records the current venv/tool versions.
- Source publication, image publication and dev rollout are permitted only within this task's selected footprint. New authority, costs, customer resources, source-history rewriting or broad cleanup require explicit approval.

The detailed session planning document is `/Users/asegundo/.opencode/plan/iam-wifconfigtemplate-version-catalog.md`. This tracked checkpoint and the phase evidence are the durable resume authority; do not depend solely on that local planning file.
