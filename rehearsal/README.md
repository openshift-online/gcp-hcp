# Disposable GitOps operator migration fixture

GCP-1288/GCP-1186 fixture only. Never merge these generated revisions to main.
Pin the exact reviewed commit SHA; this catalog branch head is not a runtime default.
Each revision has two independent Kustomize roots: rehearsal/region/bootstrap and rehearsal/mc/bootstrap.
Only the active target differs from legacy bootstrap; the partner remains legacy.
Stages: P1 legacy scale0; P2a manager0; P2b manager1; bad missing two grants; P3a seed absent/manager0; P3b seed absent/manager1 for cleanup.
All five child CRDs have fixture-only deletion:detach. Parent shared APIs and bootstrap/ are unchanged.
No environment target is changed by publishing these files. Runtime provisioning/adoption/recovery requires a separate fixture grant.
No standalone Promoter installation or migration is included. Operator-managed Promoter and Source Hydrator stay disabled.
