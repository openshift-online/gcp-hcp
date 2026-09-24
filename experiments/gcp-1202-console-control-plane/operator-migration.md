# Porting console-operator to dual-cluster operation

**Context:** Deliverable for spike GCP-1202. This document scopes the upstream `console-operator` split-cluster refactor required to run console control-plane-side in HyperShift GCP HCP. It is the primary cost item reviewers will scrutinize.

**Companions:** `architecture.md` (topology/network diagrams), `findings.md` (feasibility verdict), `open-questions.md` (risks), `reference/console-operator-io-spec.md` (operator I/O inventory), `reference/cno-dual-cluster-precedent.md` (CNO as prior art).

---

## The problem

Everything the console-operator normally owns is currently hand-rolled in `manifests/console/`:

- **Plugin enablement** — watching guest `ConsolePlugin` CRs, regenerating `console-config` with enabled plugin endpoints, rolling the bridge Deployment
- **Bridge pod identity** — guest ServiceAccount token minting, `POD_NAME` env injection
- **Console ServiceAccount + RBAC** for the guest SA the bridge authenticates as (user-settings ConfigMaps, Dashboards RBAC)
- **OAuthClient/OIDC status writes** — `Authentication.status.oidcClients[]` entries for console/cli components
- **User-settings RBAC** — Roles/RoleBindings for ConfigMap-based user preferences
- **Secret injection** — oauth-config secret, session keys, OIDC CA ConfigMaps cross-cluster sync
- **ConsoleCLIDownload CRs** — oc/kubectl/odo download links derived from ClusterVersion + Route host
- **ClusterOperator status** — `Available`/`Progressing`/`Degraded` conditions, version reporting, related-objects list
- **Custom branding/logo** — copying logos from `openshift-config` to the operand namespace
- **Upgrade notifications** — the cluster-upgrade banner ConsoleNotification

Without the operator there is **no reconciliation** of guest-side configuration changes. The hand-wiring is static kustomize overlays with `REPLACE_*` placeholders substituted at CPO build time — it does not react to plugins being added, logos changed, or CLI downloads configuration updated.

The console-operator exists to turn those CRs and config objects into runtime operand state.

---

## Why the operator is hard

The console-operator is **single-cluster by construction**. Every controller holds typed clients against **one** kube-apiserver:

- `~14 controllers` (deployment sync, OAuthClient, OIDC setup, ServiceAccounts, Downloads, CLI downloads, Services, Routes, PDB, upgrade notifications, status, etc.)
- Each calls `resourceapply.Apply*` directly with a typed client
- No central apply function
- No cluster-routing layer (no annotation-based object-to-cluster dispatch, no `ClientFor(clusterName)` abstraction)
- Namespace constants (`api.TargetNamespace = "openshift-console"`) are embedded throughout ~48 references across 19 files

The dual-cluster requirement is:

| Resource type | Current location (single-cluster) | Target cluster (split-cluster) |
|---|---|---|
| **Operand objects** (Deployment, Service, ConfigMap, Secret, SA, PDB) | guest `openshift-console` | **Management cluster HCP namespace** |
| **Config CRs** (`operator.openshift.io/Console`, `ConsolePlugin`, `ConsoleCLIDownload`, `ConsoleNotification`) | guest | **Guest** (source of truth) |
| **Config observation** (Infrastructure, Ingress, Proxy, Authentication, FeatureGate, ClusterVersion, OAuth) | guest | **Guest** (cluster config) |
| **Status writes** (`console.config.openshift.io/cluster` status, `Authentication.status.oidcClients`, `ClusterOperator/console`) | guest | **Guest** (status publish) |
| **Routes** (console/downloads exposure) | guest `openshift-console` | **CPO-owned** (special case) |

Routes are a **special case**: the HyperShift router owns exposure (public/private variants, hostname derived from APIServer, ExternalName Service for Private under GCP Private Service Connect), so the operator **must not** manage Route objects. CPO creates the Routes; the operator manages Service/Deployment/PDB only. See `reference/console-operator-io-spec.md` §B for the exposure vs content classification.

This split-client topology is **not** how console-operator is architected today. There is no code path for "create the operand Deployment in one cluster, read config CRs from another, write status to a third (same as config but conceptually distinct)."

---

## The required split

A console-operator process running in the management cluster HCP namespace must hold **two simultaneous kube-apiserver clients**:

### Management cluster (MC) client
- **Operand objects** — bridge + downloads Deployment, ConfigMap (`console-config`, CA bundles), Secret (oauth-config, session keys), ServiceAccount, Service, PodDisruptionBudget
- **Leader election** — in-cluster lease in the HCP namespace
- **Events** — publish to management cluster EventRecorder
- **Operator pod identity** — its own in-cluster ServiceAccount (for RBAC to create/update operand objects)

### Guest cluster client
- **Operator CR** — `consoles.operator.openshift.io/cluster` spec and status
- **Config CRs** — all `config.openshift.io/v1` objects (Infrastructure, Ingress, Proxy, Console, Authentication, OAuth, APIServer, FeatureGate, ClusterVersion)
- **Console CRs** — `ConsolePlugin`, `ConsoleCLIDownload`, `ConsoleNotification` (`console.openshift.io/v1`)
- **OAuthClient** — `oauth.openshift.io/v1` `console` client registration (integrated OAuth path)
- **Status writes** — `console.config.openshift.io/cluster` status, `Authentication.status.oidcClients[]`, `ClusterOperator/console` conditions/version/relatedObjects
- **Config namespaces** — ConfigMap/Secret reads from `openshift-config`, `openshift-config-managed` (logo, oauth-serving-cert, monitoring-shared-config)
- **Guest SA identity** — the ServiceAccount the bridge pod authenticates **as** when proxying to the guest KAS (distinct from the operator's own MC SA)

Every controller constructor must be threaded with the correct client set — **no logic change**, only which existing client variable each receives. This is the mechanical bulk of the refactor.

---

## Decision

**Port the upstream console-operator to be dual-kube-API capable and deploy it as a HyperShift CPOv2 control-plane component** modelled on `ingress-operator` and `cluster-network-operator`.

Two separable problems:

| Problem | Owner | Nature |
|---|---|---|
| **A. How the operator is deployed** control-plane-side (image resolution, konnectivity + token-minter sidecars, guest kubeconfig injection, HCP field substitution, gating) | **CPO component** (framework wiring only) | Small–medium, precedented by ingress-operator/CNO |
| **B. How the operator talks to two kube-APIs** and splits reconcile (MC operand vs guest CRs/config/status) | **Upstream console-operator** (dual-kube refactor) | Larger but mechanical |

### Problem A — CPO component deployment (framework patterns)

Every `REPLACE_*` placeholder and hand-mirrored sidecar in the current `console/kustomize/` becomes a framework call:

| Today (hand-rolled kustomize) | CPO component (framework) |
|---|---|
| konnectivity socks5 sidecar (verbatim copy) | `.InjectKonnectivityContainer(...)` |
| token-minter init-sidecar (verbatim copy) | `.InjectTokenMinterContainer(...)` |
| `REPLACE_CPO_IMAGE`, `REPLACE_CONSOLE_IMAGE`, `REPLACE_DOWNLOADS_IMAGE` | `ReleaseImageProvider.GetImage(...)` in `adaptDeployment` |
| `REPLACE_ISSUER_URL` (token-minter audience) | `cpContext.HCP.Spec.IssuerURL` |
| `REPLACE_HCP_NAMESPACE` | `cpContext.HCP.Namespace` (auto-namespaced) |
| `REPLACE_API_DOMAIN`, public/private endpoint | `HCP.Status.ControlPlaneEndpoint` + `netutil.IsPublicHCP/IsPrivateHCP` |
| `service-network-admin-kubeconfig` volume | `.InjectServiceAccountKubeConfig(...)` |
| GCP + capability gating | `.WithPredicate(platform==GCP && Console capability)` |

This is **proof that CPO carries zero console business logic** — it is all standard component framework machinery. `ingress-operator` is the direct template (an upstream OpenShift operator run control-plane-side that reconciles guest resources). CNO is the precedent for management-side operand RBAC (the operator needs an HCP-namespace Role to create the bridge Deployment).

### Problem B — dual-client operator refactor (the upstream work)

Quantified from source analysis:

- **~92 client variable references** in `starter.go` to audit (which controller constructor gets which client)
- **~48 `TargetNamespace` constant references** across 19 files (operand namespace must be parameterized: `openshift-console` → HCP namespace)
- **~14 controllers** to thread (deployment sync, oauth, oidc, service accounts, downloads, CLI downloads, services, routes, PDB, health check, upgrade notifications, status, config observer, migration cleanup)
- Add `--guest-kubeconfig` flag (or `--guest-apiserver-url` + `--guest-token-file` for token-minted credentials)
- Build a second `*rest.Config` from the flag
- Split `kubeClient` into `mgmtKubeClient` (operand objects, migration cleanup, resourceSyncer) and `guestKubeClient` (config namespace reads)
- Route all typed CR clients (`configClient`, `operatorConfigClient`, `consoleClient`, `oauthClient`) to the guest config
- Operand namespace parameterization: `--operand-namespace` flag (default: pod's own namespace via downward API `NAMESPACE` env)
- Off-cluster bridge builder: teach `subresource/deployment` to emit the bridge's token-minter + konnectivity sidecars, off-cluster flags (`-k8s-mode=off-cluster`, `-ca-file`, `-service-ca-file`, OIDC issuer/client-id/secret-file, session keys, `-branding=ocp`, `-plugins`, Thanos/AM URLs, `POD_NAME`, proxy env)

This is a **medium-heavy mechanical refactor**, not a rewrite. No controller logic is rewritten — only which client each constructor receives.

The thorniest pieces (design risk, not line count):

- **Cross-cluster secret/config hand-off** — oauth-config, console-config CAs, `service-ca.crt` computed from guest data but mounted on MC bridge. library-go's `resourcesynccontroller` is single-cluster, so this needs a hand-rolled cross-cluster copy or direct fetch. **Deferrable for core console + plugins** (OIDC path does not need oauth-serving-cert; service-ca is guest-side only for plugin backends).
- **Configurable operand namespace** — wide but mechanical (~48 refs, 19 files).
- **Coherent cross-cluster status** — keep `Available`/`Progressing`/`version` gated on the MC Deployment while writing conditions to the guest CR.
- **Upstream appetite** is the main external risk — mitigated by dev image-overrides (HC `image-overrides` annotation) until it lands.

---

## Rejected alternatives

### (a) Reimplement console logic directly in CPO

Fork the operator's reconciliation logic into CPO controllers — re-derive `SyncConfigMap`, plugin-discovery, status publishing, custom logo sync, upgrade banner logic, etc.

**Rejected:** Violates the "zero console business logic in CPO" constraint. Also means maintaining a fork of behavior the upstream operator already has, and that fork does not automatically evolve with future OCP releases. This trades a one-time upstream refactor for perpetual drift maintenance.

### (b) Fold guest reconciliation into HCCO

Run console reconciliation in the Hosted Control Plane Config Operator (HCCO), which already manages guest resources from the control-plane side.

**Rejected:** HCCO is the right conceptual analogy but the wrong host. HCCO's internal controllers are bespoke, non-framework Go. Putting console there means writing new controllers rather than running the upstream operator image — again, console logic in HyperShift instead of in the console codebase.

### (c) Teach the console bridge to read required configs in-process (Cesar Wong's counter-proposal)

Eliminate the operator's config-reconcile-then-redeploy loop by having the bridge process itself read the required ConfigMaps/Secrets/CRs directly from the data plane and hot-reload them without a pod restart.

**Fair treatment:** This addresses a real concern (see "Blast-radius concern" below) and is plausible for simple configs (feature flags, branding). Several inputs are custom resources (`ConsolePlugin`, `ConsoleCLIDownload`), and the plugin-enablement set is non-trivial — watching CRs, discovering backends, validating endpoints, merging i18n, regenerating the plugins map, and pushing that to the frontend requires reconciliation logic somewhere. Moving it into the bridge means the bridge grows an in-process reconcile loop, duplicating operator patterns but without the separation-of-concerns that makes the operator testable/observable via its CR status. For **simple** configs it is viable; for **dynamic plugin discovery** it is effectively "fold reconciliation into the bridge" — architectural debt.

**Not mutually exclusive:** In-process hot-reload of simple configs (logos, feature flags) **reduces** the redeploy surface regardless of whether the operator exists. This is an orthogonal improvement, not a replacement for the operator port.

**Conclusion:** Defer as a later optimization. The operator port does not block exploring this for specific high-churn low-complexity configs.

---

## Effort

Split into the two problems:

### Problem A — CPO component deployment: **small/medium, precedented**

- New Go package `control-plane-operator/controllers/hostedcontrolplane/v2/console-operator/`
- `component.go` (options, predicate, builder registration)
- `deployment.go` (adaptDeployment: image env, volumes, args)
- Assets: `v2/assets/console-operator/{deployment,serviceaccount,role,rolebinding}.yaml`
- Management-side RBAC (HCP namespace Role) — CNO model, not ingress-operator (operator's operand lives MC-side, needs RBAC)
- Add `capabilities.IsConsoleCapabilityEnabled` to `support/capabilities/`
- Register in `hostedcontrolplane_controller.go`
- Konnectivity sidecar (HTTPS mode — operator talks only to guest KAS), token-minter for the bridge, guest kubeconfig via `InjectServiceAccountKubeConfig`

**Sizing:** small. Almost entirely pattern-matching against the existing
ingress-operator component plus CNO's management-side RBAC; no novel design.

### Problem B — dual-client operator refactor: **medium-heavy mechanical**

> **Provenance of the counts below.** They come from reading
> `openshift/console-operator` at commit
> `b8447d3ef1eb621046ebdce84efb8ceb736dabdb` (2026-09-15). They are approximate
> by construction — a grep-and-audit pass, not a compiler-verified figure — and
> they will drift as upstream moves. Re-derive them against current `main`
> before quoting them in an estimate or a design decision.

Quantified:

- **Client construction** — ~92 references to audit in `starter.go`, split `kubeClient` → `mgmtKubeClient`/`guestKubeClient`, add second config
- **Informer factories** — ~6 guest-namespaced factories (openshift-config, -config-managed, -console-operator, monitoring), operand factory stays MC
- **Controller threading** — ~14 constructors, each receives mgmt or guest client; **no logic change**, only which variable is passed
- **Operand namespace param** — ~48 refs across 19 files, mechanical const→param sweep
- **Off-cluster bridge builder** — small, contained to `deployment.go` (emit sidecars/flags from options struct)
- **resourceSyncer cross-cluster** — deferrable (core console + plugins do not need it under OIDC; only integrated OAuth/custom logo)

**Line items:**
- `starter.go` (~200 lines modified: client construction, informer factories, controller calls)
- `cmd/operator/cmd.go` (add flags, forward via closure to preserve library-go signature)
- `deployment.go` (~50 lines: off-cluster seam, sidecar append points)
- `api/api.go` + 18 consuming files (~48 sites, mechanical: class-1 operand ns → param, class-2 OIDC identity → literal const)
- `operator.go`, sync controllers (client param added to each constructor signature, ~14 files touched)
- `resourceSyncer` two-client variant (defer / stub initially)

**Sizing:** medium-heavy, but low design risk. The const sweep is wide and
grep-driven, client threading is mechanical, and CNO already validates the
dual-cluster topology in production. The work is broad rather than deep.

### On calendar estimates

This spike deliberately does **not** put a week count on either problem. The
counts above (≈92 client references, ≈48 namespace constant references across 19
files, ≈14 controllers) are the honest, checkable sizing inputs and they came
out of reading the code; any calendar figure derived from them here would be a
guess presented as an estimate. Sizing should be done by whoever will implement
it, ideally with the console team in the room.

What can be said about sequencing: problem A can start immediately and is
independent; problem B can proceed in a fork behind image overrides until
upstream accepts it. The long pole is almost certainly not the code — it is
getting the console team to agree in principle to a remote-cluster operating
mode, which is a review and design-consensus cost rather than an engineering
one.

---

## Upstream framing

**How to pitch it to the console team so it is not "HyperShift-specific":**

"Manage console on a remote cluster" — the same capability CNO already shipped with `--extra-clusters`.

**Precedent:** `cluster-network-operator` merged support for dual-cluster operation (`--extra-clusters=management=<kubeconfig>`) to run its control-plane operands (ovnkube-control-plane, network-node-identity) on a management cluster while reconciling guest CRs/config. This is production-deployed in HyperShift today. Console-operator's refactor follows the same motivations:

- **Isolation/availability** — console as a control-plane component, resilient to guest-cluster disruption
- **Zero-node support** — console accessible before worker nodes exist
- **Generic "off-cluster operand" capability** — useful for any hosted/remote-cluster topology, not HyperShift-exclusive

**Key difference from CNO:** CNO had annotation-based routing from day one (`network.operator.openshift.io/cluster-name` annotation on every object, one generic `ApplyObject` that routes by annotation). Console-operator has per-controller typed clients, so the client threading is **manual** rather than declarative. This is the **mechanical** work, not a rewrite.

**Upstream benefits:**

- Console survives guest-cluster disruption (same SLO argument as KAS/etcd)
- Enables console in zero-node test/dev clusters
- Paves the way for any "console-as-a-service" or remote-cluster-management scenario

**Commitment to upstream:** The dual-client refactor is intended to land in `openshift/console-operator` as a generic feature. Until it merges, HyperShift allows **image-overrides** (HC annotation) so dev/staging environments can run a patched operator. This is not a fork — it is upstream-first with a temporary dev affordance.

---

## Open design questions

### 1. Placement flag — where does it live?

Two options:

- **(a) HostedCluster spec field** — `spec.console.placement: ControlPlane | Guest`, structured API
- **(b) Annotation** — `hypershift.openshift.io/console-placement: control-plane`, lower API commitment

**Tradeoff:** (a) is discoverable and validated; (b) is lower-friction for experimentation. Lean toward (b) initially, promote to (a) when stable.

### 2. CVO payload strip — which filenames, strip vs keep?

The console-operator ships ~8 payload manifests:

- CRDs (`0000_50_console-operator_00-*.yaml`)
- Namespace, RBAC, ServiceAccount (`..._02-*.yaml`, `..._04-*.yaml`, `..._06-*.yaml`)
- Operator Deployment (`..._07-operator-ibm-cloud-managed.yaml` — GCP-specific stripped already; standard `..._07-operator.yaml`)
- ClusterOperator manifest (`..._95-clusteroperator.yaml`)

**When placement=on (console runs control-plane-side):**

- **Strip** the operator Deployment manifest (CVO must not create it in the guest)
- **Strip** the ClusterOperator manifest (guest CVO does not own console status when console is control-plane-side)
- **Keep** CRDs, Namespace, RBAC, ServiceAccounts (guest still needs the console CR schema, the guest SA the bridge authenticates as, and the config namespace)

**Mechanism:** extend the CVO `preparePayloadScript` conditional pruning (`v2/cvo/deployment.go`) — same pattern as the existing `!oauthEnabled` strip of `0000_50_console-operator_01-oauth.yaml`. Gate on the placement flag.

**Exact filenames:** verify against dev-repo → release-payload manifest renaming (dev `manifests/*.yaml` → payload `0000_50_console-operator_*.yaml`).

### 3. Migration cleanup — placement flip off→on

When a cluster's placement flag flips **off → on** (guest console → control-plane console), the already-applied guest console-operator Deployment + operand must be **actively deleted** (not just omitted from future CVO syncs).

**Mechanism:** the CVO `resourcesToRemove` / `0000_01_cleanup.yaml` path (`v2/cvo/deployment.go:243-273`) — add console-operator Deployment + console Deployment/Service/ConfigMap to the cleanup list when placement flips.

**Alternative:** HCCO guest-resource cleanup controller (more surgical; avoids CVO coupling). Needs design.

### 4. Guest ClusterOperator status — who publishes it?

When the console runs control-plane-side, the **guest `ClusterOperator/console`** still exists (its manifest is in the kept payload scaffolding or hand-created), but **who writes its status**?

Options:

- **(a) Control-plane operator writes guest status** — cross-cluster status publish (dual-client path, same as writing `Authentication.status.oidcClients`)
- **(b) Stub/static status** — guest CO is present but shows a static "console managed externally" message
- **(c) No guest CO** — strip it entirely; console status lives only control-plane-side (breaks CVO's expectation of a console CO)

**Tradeoff:** (a) is most transparent but requires the dual-client refactor to handle cross-cluster status writes. (c) may confuse CVO. Lean toward **(a)** — the operator writes both MC operand status (via its in-cluster OperatorClient) and guest CO status (via guest client).

---

## Blast-radius concern (raised in review)

**Concern (Cesar Wong):** A workload on the control plane whose inputs live in the data plane means a customer editing a data-plane ConfigMap (custom logo, feature flag, plugin CR) can trigger a console redeployment **on the control plane** — a cheap DoS vector against control-plane capacity. A malicious or careless customer could churn console restarts and consume management-cluster resources.

**Validity:** Real. Any data-plane-to-control-plane config feedback creates this surface.

**Mitigations to design for (not blockers, but constraints on the implementation):**

1. **Trim what is customer-configurable** — reduce the console config surface exposed to data-plane customers. A "reduced-configurability console object" in the HCP KAS (not the guest KAS) was floated and considered acceptable by the team. Not all `operator.openshift.io/Console` fields need to be guest-writeable.

2. **Rate-limit/debounce reconciliation** — standard operator practice (don't roll the bridge on every 1-second ConfigMap edit; debounce to 30s or detect no-op changes).

3. **Prefer in-process hot-reload over pod restarts** — where viable (simple configs), teach the bridge to reload without redeploying. Reduces the blast radius to "the bridge re-parses a ConfigMap" instead of "Kubernetes reschedules pods."

4. **Cap the operand's resource footprint** — console bridge pods have CPU/memory limits. A customer-triggered restart consumes a bounded amount of management-cluster capacity (unlike, say, triggering a KAS restart).

5. **Audit logging** — control-plane operators should log reconcile triggers. If a customer is churning console restarts, that is observable and addressable via support escalation (same as any other abuse vector).

**Conclusion:** This is a **design constraint** that must be resolved before implementation, **not a blocker to the spike verdict**. The ported operator must incorporate rate-limiting and cap resource consumption. The in-process hot-reload direction (Cesar's proposal) is **complementary** — it addresses the same concern for a subset of configs.

**Status:** Open design question, not a "reject the operator port" finding. CNO faces the same surface (guest `Network` CR changes can trigger control-plane OVN restarts) — validate how CNO mitigates this and apply the same patterns.

---

## Summary

**Decision:** Port console-operator to dual-cluster, deploy via CPO component. Console business logic stays in the console image; CPO contributes only platform wiring.

**Effort:** a small CPO component plus a broad-but-mechanical upstream operator refactor. Not sized in calendar terms here — see [On calendar estimates](#on-calendar-estimates).

**Risks:** Upstream acceptance (mitigated by generic "remote cluster" framing + CNO precedent), blast-radius controls (design for rate-limit/hot-reload/resource-cap).

**Next:** Validate CNO's dual-cluster client implementation (`reference/cno-dual-cluster-precedent.md`), audit console-operator I/O (`reference/console-operator-io-spec.md`), confirm upstream appetite with console team.
