# Phase 4 — personal-dev NodePool-upgrade coordinator activation

**2026-11-20. Dev-only coordinator activation and ten-minute idle acceptance
passed; final independent readback 10:06:00 UTC. Phase 4 remains incomplete.**
This slice does not authorize
catalog activation, an assertion, IAM changes, provisioning or upgrade
execution. The completed negative API guards were not rerun.

## Exact publication and intended footprint

- Personal-dev infra source `/Users/asegundo/git/osd/gcp-hcp-infra-dev-main`,
  commit **`0d47f9eecccc9db245705b689c4efd2f6a86c2a2`**, fast-forward pushed
  explicitly to fork `origin/dev-apahim`; exact ref read back. Only two files
  were added: the dev-only `gecko-nodepool-upgrade` override and its generated
  Application. The common template remains `_disabled: true`; integration
  overrides were not touched. No existing child Application spec changed.
- New Application pins already-reviewed/published Gecko source
  `6c28cf4348a8ab4f524b91ece1cdbe29f1389c44` and the previously published
  `quay.io/apahim/gecko:wif-p4-bridge-6c28cf4-controllers` image. A fresh
  linux/amd64 pull by resolved registry digest
  **`sha256:cf07c10b7b37ac443b7ddad98fc14b7b45976def45c275848a329255cf4a3876`**
  returned the known image ID
  `187edd1d81e915e3eca8d99d94a6ee583140c8dc308ecbf3cc5fc90a2d9b18c5`.
  Gecko/CLI source and native CLI artifact were unchanged.
- The reviewed child chart renders **five objects only**: one Deployment,
  ServiceAccount, ClusterRole, ClusterRoleBinding and NetworkPolicy. Replica
  count 1, rolling update maxSurge 1/maxUnavailable 0; resource requests
  50m CPU/64Mi memory, limits 500m/256Mi. Non-root UID 65532, read-only root
  filesystem, dropped capabilities and RuntimeDefault seccomp. No ServiceAccount
  cloud-identity annotation, Cloud IAM grant, Secret, Job, CRD or database object.
  Application has automated prune/selfHeal and a resource finalizer; deleting
  it later would remove these five Kubernetes objects and needs separate review.

## Network/RBAC review and bounded preparation

- The chart's default TCP/443 allow-list is `0.0.0.0/0, ::/0`. The dev override
  narrowed API egress to observed Kubernetes service `10.8.0.1/32` and the
  sole Ready `default/kubernetes` EndpointSlice backend `172.16.0.2/32` on
  port 443. The exact Service and EndpointSlice were refreshed before push;
  GKE readback showed private endpoint `172.16.0.2` and advanced datapath.
  The chart still allows UDP/TCP port 53 to any destination and has no ingress;
  the broad DNS rule is an acknowledged **dev-only residual risk**, not a
  least-privilege production claim. A rotated control-plane IP would require
  a reviewed CIDR update; this override fails closed if the endpoint changes.
- The existing chart ClusterRole grants get on `wifconfigtemplates`,
  get/list/watch/patch on NodePools, get/update/patch on NodePool status and
  get/list/watch on Clusters/Versions. Architect review identified the
  main-resource `nodepools` patch verb as unnecessary for the current
  coordinator (which uses optimistic-lock status patches). This pre-existing
  **dev RBAC over-grant must be removed before integration/production**; no
  source/chart change was authorized in this dev-only activation. A connected
  customer NodePool upgrade request would no longer be inert; this rollout
  must not be mistaken for a dispatch hold or an upgrade permission grant.
  Live authorization-review requests confirmed `nodepools/status` patch
  allowed, main-resource `nodepools` patch allowed (the documented over-grant),
  and Secrets get denied for this ServiceAccount. These authorization checks
  did not fetch Secrets or modify resources.
- Fresh inventory at 09:48 UTC found only `wif3-first` and `workers`; their
  raw releases were both 5.0.2. `workers` spec and status versions matched,
  no upgradeStartTime, pending dispatch or NodePoolUpgrade conditions. Its
  private UID `a8768aef-c352-47bb-9f32-3638275f1e15`, generation 1, spec
  digest `fabd8ab994a94a03c7b43bbb99a3d1008fe87b13ccb8f81cee0624eeb77f69f0`
  and resourceVersion **508** were recorded before activation. Both gates
  were false; eight existing Deployments Ready, guest/worker healthy,
  21 Versions/19 pins, migration Job/Spanner/CRD pin and all three journal
  hashes unchanged. Last-minute read at 09:53:26 UTC reconfirmed all of these
  plus the live Kubernetes service and endpoint.
- Architect review: **conditional go for dev** after validating endpoint,
  catalog/gates, empty idle-upgrade state, five-object render, source/image
  availability and protected state. Repo renderer ran twice from a dedicated
  Python venv, stable byte-for-byte (new Application SHA-256
  `16af245d3b6911ee09769d597f9508d89fa1eba63f44a288e51b2a89ad805164`).
  Renderer test (1), Argo configuration tests (45; two optional skips), Helm
  chart lint and focused Gecko NodePool-upgrade/controller-command tests,
  including a race run, passed. Full Gecko `GOWORK=off make test` with explicit
  `HELM_BIN` also passed from the unchanged source (standard optional database
  integration-test skip rules apply); full `GOWORK=off make lint` passed with
  zero issues and no tracked Gecko changes. Standard infra `make` wrapper uses
  unavailable `uv`; its Python test
  entry points were run directly in the dedicated venv. Actual live regional
  root Helm values rendered all eight existing child Application specs exactly
  equal to live and added only the one new Application; its child chart rendered
  precisely the five reviewed objects. No guard or IAM harness was run.

## Observed deployment and immediate acceptance

- Refreshed only `root-app-dev-reg-na-ne2-apahim1110-gke` cache, allowing
  ordinary Argo automated reconciliation; no direct apply, manual sync,
  Force/Replace, Job replay or upgrade request. Root and new child Application
  were Synced/Healthy, child source exactly `6c28cf4`; all five objects were
  individually Argo Synced. New Deployment UID
  `54992224-59d7-4c17-9d1c-a916772f1ba1` reached 1/1 Ready. New pod
  `gecko-nodepool-upgrade-65f4fcd486-c2htw` UID
  `71f0d553-84fb-4f0e-b8ac-c737f7d52075` ran the expected registry digest
  with zero restarts. Live NetworkPolicy selectors and exact two /32 HTTPS
  destinations matched the reviewed render; the ServiceAccount had no GCP
  Workload Identity annotation. A bounded startup log sample had five lines,
  no error, panic, forbidden, timeout or upgrade-write matches.
- Immediately after activation, original `workers` UID/generation/spec,
  resourceVersion **508**, version 5.0.2, Dispatched receipt and Complete IAM
  requirements remained unchanged; no NodePoolUpgrade conditions or pending
  dispatch. Cluster UID/generation/spec/public digest and original Dispatched
  receipt unchanged. Nine Deployments Ready; existing eight Deployment specs
  and image IDs unchanged. Two pre-activation pods (control-plane-upgrade and
  version-sync) were independently rescheduled by node autoscaler at 09:51:10
  UTC (ScaleDown events), retaining their original ReplicaSets and image IDs;
  this was **not** a controller spec or image rollout.
- Independent reads at 09:55:05 UTC confirmed the retained guest 5.0.2
  Completed/Available=True/Failing=False/Progressing=False and same original
  Ready Node UID `79b85ccd-768f-44f9-bc5b-f40a2bdd2cab`; public TLS Cluster
  GET HTTP 200 and original public spec digest. Both gates explicitly false;
  21 Versions/19 existing pins. Migration Job UID
  `daa14792-1593-4a92-abe8-2b8f32ba3fd1`, timestamp, spec digest
  `273c6c47a0da1d9383178425ca07b88422a6c29bd3a8bd26cdd70d8e5fae93ae`
  and Complete/Synced state unchanged. Spanner UID
  `99e4163c-79f9-4875-92b9-0e576b9a5927`, spec and Ready state unchanged.
  HyperShift Application Synced/Healthy at CRD-only source
  `914334ae61352fa5641e3f1d50250e87299e9eb3`, stamp
  `3d8a55cb8c633329d2a64939e7e0ab20520107ee`. Primary journal checksum
  `6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995`,
  retired v1 guard
  `af1f4abf8e822266f8a089bf03773afb8eb908083ca31f28f19e1a025d178588`
  and completed v2 guard
  `54ec587d465eed4e5749dd23f8becd42e86018a61a06d9217412bb1866b8cc36`
  match originals; no primary update sidecar.

## Idle observation and next boundary

The first watch invocation failed locally because this kubectl version does
not accept `get --resource-version`; it made no mutation and the NodePool RV
remained 508. A replacement read-only watch ran **600 seconds**, from
09:55:35 to 10:05:36 UTC. Its bounded process was intentionally terminated
at the ten-minute limit (`kubectl` exit 1, no stderr); this is not a controller
failure. The watch received two NodePool events; both retained the original
UID, generation, spec, dispatch receipt, Complete requirements state,
status version 5.0.2 and empty upgrade-condition set. RV advanced from 508
to 510 while the requirements `lastRefreshTime` advanced (final
10:01:12 UTC). This is consistent with ordinary status refresh; the watched
upgrade-relevant fields and spec did not change. Final independent private,
public, management and guest reads at 10:06:00 UTC passed all retained-state
checks above: nine Deployments Ready, original `workers` UID/generation/spec/
receipt with **no NodePoolUpgrade conditions or requested upgrade**, guest
5.0.2 healthy, both gates false, 21 Versions/19 pins, Job/Spanner/CRD source
and all three journal hashes intact, no update sidecar. New coordinator pod
still Ready with zero restarts and the exact registry digest. Its 15-minute
startup log sample (five lines) had no error, forbidden, timeout, panic or
upgrade-patch match. **The approved idle-activation acceptance passed.**

Phase 4 remains incomplete. Separate approvals are required for candidate
floor/source template publication/pinning, shared verification and protection
of the retained installation, second-installation footprint/cost/time, real
IAM/assertion operations, and any control-plane or worker upgrade. Customer
upgrade requests bypass maintenance exclusions. Do not extend or request an
upgrade as part of this activation. Address dev-only chart RBAC and DNS
egress limitations before broader promotion.
