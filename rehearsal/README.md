# Disposable GitOps operator migration fixture (corrected release stages)

GCP-1288/GCP-1186 fixture only. Never merge these generated revisions to main.
Pin the exact reviewed commit SHA; this catalog branch head is not a runtime default.
Each revision has two independent Kustomize roots: rehearsal/region/bootstrap and rehearsal/mc/bootstrap.
Only the active target differs from legacy bootstrap; the partner remains legacy.

Corrected stage chain: P1 legacy scale0; P2a manager0 with the seed present and
Config Sync-managed; R source-declared release of ArgoCD/gitops and argocd-cm
(configmanagement.gke.io/managed: disabled) with manager0; bad manager1 with
exactly the two applicationset grants missing; P2b manager1 with full grants,
after the release; Q quarantine manager0 with depends-on removed from every object
the following legacy transition prunes.

The release annotation is declared in SOURCE. A manual annotation on the live
object is overridden by Config Sync and is not a release. Config Sync does not
leave managed: disabled on the released object; verify the inventory entry and
management metadata are gone, not the annotation value.

Stages P3a/P3b from the earlier publication are retained as historical
release-by-removal/cleanup evidence and are not part of this chain.
All five child CRDs have fixture-only deletion:detach. Parent shared APIs and
bootstrap/ are unchanged. No environment target is changed by publishing these
files. Runtime provisioning/adoption/recovery requires a separate fixture grant.
No standalone Promoter installation or migration is included. Operator-managed
Promoter and Source Hydrator stay disabled.
