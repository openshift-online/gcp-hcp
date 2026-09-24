# CNO as dual-cluster precedent for console-operator

**Purpose:** How `cluster-network-operator` (CNO) already does dual-cluster (management-cluster operands, guest config) in production HyperShift today. This is the proven precedent for `../operator-migration.md` — what to copy and what not to.

**Source:** Analyzed from the `hypershift` codebase (`control-plane-operator/controllers/hostedcontrolplane/v2/cno/`, `v2/assets/cluster-network-operator/`) and upstream `openshift/cluster-network-operator` (cloned separately for verification).

**Companion:** `../operator-migration.md` §B "dual-API operator refactor" — console-operator's client-threading work is the same problem CNO already solved, but CNO solved it with annotation-based routing (console has per-controller typed clients, so must thread manually).

---

## The CNO topology

CNO is an **upstream OpenShift operator** (`github.com/openshift/cluster-network-operator`) that runs **control-plane-side in HyperShift** with its operand split:

- **Management cluster (MC) operands** — ovnkube-control-plane, network-node-identity, multus-admission-controller, cloud-network-config-controller Deployments in the HCP namespace
- **Guest cluster operands** — ovnkube-node DaemonSet, multus DaemonSet, CNI config on guest worker nodes
- **Guest CRs/config** — `networks.operator.openshift.io/cluster`, `networks.config.openshift.io/cluster`, guest Infrastructure/Proxy/APIServer
- **Status writes** — guest `ClusterOperator/network`, guest `Network.operator` status

This is **identical** to the console topology: operator runs MC-side, holds simultaneous clients to MC and guest, places some objects on MC (the control-plane operand) and others on guest (config CRs, status).

**Why CNO does this (isolation/availability):** No comment or doc in this repo or upstream CNO explicitly states "we placed ovnkube-control-plane on the management cluster for SLO reasons." But the topology itself is suggestive: OVN-Kubernetes' control-plane pieces run in the HCP namespace, co-located with etcd/KAS, rather than in the guest cluster alongside ovnkube-node. This means:

- The network control-plane brain keeps running even if guest nodes are unhealthy, being drained, or the guest API is under load — it is not competing for scheduling/resources with guest workloads.
- Guest-cluster admins with only guest-KAS access cannot delete, scale down, or otherwise disrupt ovnkube-control-plane — it sits under management-cluster RBAC.
- Monitoring/alerting/lifecycle for the control-plane piece rides on the same management-cluster machinery (CPO rollout, mgmt Prometheus) used for KAS/etcd/OAuth, rather than depending on the guest cluster's own monitoring stack being up.

**This is the same argument for console:** if console availability is part of a hosted cluster's advertised guarantees, keeping the bridge on management insulates it from guest-cluster disruption the same way CNO insulates OVN's control-plane brain.

---

## How CNO is deployed by CPO

**CPO creates only the CNO Deployment itself** plus its RBAC in the HCP namespace — **not** the operands.

`control-plane-operator/controllers/hostedcontrolplane/v2/cno/component.go:45-77` — the CPO v2 `cno` component creates:

- `assets/cluster-network-operator/serviceaccount.yaml` — CNO's management-cluster ServiceAccount
- `assets/cluster-network-operator/role.yaml` — HCP-namespace Role (management-side RBAC so CNO can create operand Deployments in the HCP namespace)
- `assets/cluster-network-operator/rolebinding.yaml` — binds the Role to the ServiceAccount (subject namespace rewritten by framework)
- `assets/cluster-network-operator/deployment.yaml` — the CNO operator pod itself
- (ARO-HCP only: `azure-secretprovider.yaml`)

**Full asset directory** `v2/assets/cluster-network-operator/` has exactly these files — **no** manifest for ovnkube-control-plane, network-node-identity, multus-admission-controller, or cloud-network-config-controller anywhere in the hypershift repo.

**CPO does not create those operands** — it only **polls for them** after the fact via `WithCustomOperandsRolloutCheckFunc` (`component.go:75,103-170`), fetching them from `cpContext.HCP.Namespace` to confirm CNO rolled them out with the expected image. This is a **readiness gate**, not a manifest apply — proof CNO itself, not CPO, creates the MC-side operand Deployments.

**Key takeaway for console:** CPO will create the console-operator Deployment + MC-side RBAC; the **operator** creates the bridge Deployment (the operand). CPO does not hand-roll the operand — the operator's reconcile loop does.

---

## How CNO reaches both clusters

### Dual kubeconfig args

`v2/assets/cluster-network-operator/deployment.yaml:22-27`:

```yaml
args:
- start
- --listen=0.0.0.0:9104
- --kubeconfig=/configs/hosted            # primary client = GUEST cluster
- --namespace=openshift-network-operator
- --extra-clusters=management=/configs/management   # secondary client = MGMT cluster / HCP ns
```

Two kubeconfigs are assembled by an **init container** (`deployment.yaml:145-193`, `rewrite-config` style):

- `/configs/hosted` → **guest cluster**, via the in-namespace `kube-apiserver` Service (same in-cluster KAS trick used by KCM/scheduler/ingress-operator, since KAS runs in the same HCP namespace)
- `/configs/management` → the pod's own local KAS (**management cluster**), using its own projected SA token — no proxy needed, it is the management cluster CNO's own pod is already running in

### Mode/env markers

- `HYPERSHIFT=true` (`deployment.yaml:31`) — flips CNO into hosted-control-plane behavior
- `HOSTED_CLUSTER_NAMESPACE` (`deployment.yaml:52-56`, fieldRef `metadata.namespace`) — tells CNO which namespace IS its management-side target (its own HCP namespace)

### Image split

CNO receives **two separate images** from CPO for the same OVN-Kubernetes component:

- `OVN_CONTROL_PLANE_IMAGE` (`cno/deployment.go:96`) — from CPO's own control-plane release payload (`ReleaseImageProvider`)
- `OVN_IMAGE` (`deployment.go:110`) — from `UserReleaseImageProvider` (the guest/data-plane release)

This matches the control-plane/guest operand split — CPO hands CNO two images so CNO knows which to use for MC-side vs guest-side operands.

**Takeaway for console:** The console-operator will receive `CONSOLE_IMAGE` from `ReleaseImageProvider` (control-plane release) for the bridge Deployment, and the operator's own image comes from the same provider. The downloads image also comes from control-plane release (both run MC-side).

---

## CNO's dual-cluster client is native, not bolted on

**This is the key architectural difference** between CNO and console-operator.

Verified against upstream `openshift/cluster-network-operator` (paths do not exist in the `hypershift` repo — this is an upstream repo):

- `cmd/cluster-network-operator/main.go:78-80` — registers `--extra-clusters` as a `StringToString` flag (`name=kubeconfigPath`)
- `pkg/client/client.go:98-125` — `NewClient(...)` builds a `map[string]*OperatorClusterClient`, one entry per cluster: the in-cluster/guest default, plus one entry per `--extra-clusters` pair
- `pkg/client/client.go:135-144` — `ClientFor(name)` returns the client for a given cluster name
- `pkg/names/names.go:230-234` — defines `ManagementClusterName = "management"`, `DefaultClusterName = "default"`
- **Object-to-cluster routing is data, not code**: every rendered object carries an annotation `network.operator.openshift.io/cluster-name` (`pkg/names/names.go:103-104`). A **single generic apply path** (`pkg/apply/apply.go:38`) does `client.ClientFor(GetClusterName(obj)).…Apply(obj)`. Sending an object to management vs guest is an **annotation on the rendered manifest**, not a separate code path per controller.
- Hypershift-mode detection: `pkg/hypershift/hypershift.go:117` reads `HYPERSHIFT=true`

**CNO's multi-cluster support was a design decision baked into its client layer from early on** — every controller renders objects and tags them with a cluster-name annotation; one generic apply function routes them. There is **no per-controller "if management then X, if guest then Y" branching** to replicate.

**Implication for console-operator:** Console-operator was **not** designed this way. It has **per-controller typed clients** and **no central apply function**, no annotation routing, no `ClientFor(clusterName)` abstraction (verified: these symbols are absent from the console-operator codebase). So the dual-client split must be **manual** — thread the correct client (mgmt vs guest) through each controller constructor by hand.

**This is why the console refactor is larger than CNO's:** CNO had the abstraction from the start; console must retrofit it. But the **topology** is identical, so CNO proves the topology is viable in production.

---

## What console-operator should copy from CNO

### 1. Dual kubeconfig idiom

CNO uses `--extra-clusters=management=<path>`. Console should use the same pattern or an equivalent:

- Option A: `--guest-kubeconfig=<path>` (simpler, aligns with console-operator's role — guest is the "remote" cluster)
- Option B: `--extra-clusters=guest=<path>` (matches CNO idiom exactly)

**Recommendation:** Option A. Console-operator's **in-cluster** client is the management cluster (where the operator pod runs, leader election, events, operand namespace), so the **injected** client is the guest. This is the **inverse** of CNO (CNO's primary client is guest, extra is management) because CNO is upstream OpenShift (guest-focused by default) while console-operator in HyperShift runs MC-side.

**CPO wiring:** Inject the guest kubeconfig via `InjectServiceAccountKubeConfig` (per-SA client-cert kubeconfig for the CVO-created guest SA `console-operator`/`openshift-console-operator`). This is the same mechanism ingress-operator uses (`ingressoperator/component.go:68-73`).

### 2. Management-side RBAC for operand creation

CNO ships `role.yaml`/`rolebinding.yaml` in `v2/assets/cluster-network-operator/` — the **management-side** RBAC that lets the operator manage its operands in the HCP namespace.

**Console must do the same:** The ported console-operator needs an HCP-namespace Role with CRUD on:

- Deployment (`console`, `downloads`)
- ConfigMap (`console-config`, `service-ca`, `trusted-ca-bundle`, OIDC CA)
- Secret (`console-oauth-config`, `session-secret`)
- Service (`console`, `downloads`, `console-redirect`)
- ServiceAccount (`console`, `downloads`)
- PodDisruptionBudget (`console`, `downloads`)

Ship these as `v2/assets/console-operator/{role,rolebinding,serviceaccount}.yaml`. The CPO framework auto-applies them to the HCP namespace and rewrites the RoleBinding subject namespace.

**Guest-side RBAC** (the operator's permissions to read/write guest CRs) is **not** shipped here — it comes from the upstream console-operator payload via CVO (ClusterRole + ClusterRoleBindings applied to the guest cluster).

### 3. HCP namespace awareness

CNO uses `HOSTED_CLUSTER_NAMESPACE` env (downward API `metadata.namespace`) to know which namespace is its operand target.

**Console should use the same pattern:** `--operand-namespace` flag (default: `os.Getenv("NAMESPACE")` from downward API, overridable by flag). Thread this into the operand informer factory, `NewConsoleOperator`, and all operand-writing controllers. Class-1 `api.TargetNamespace` references → the param; class-2 OIDC identity references → literal `"openshift-console"` const.

### 4. Operand image from ReleaseImageProvider

CNO receives `OVN_CONTROL_PLANE_IMAGE` from CPO's control-plane `ReleaseImageProvider`.

**Console:** CPO's `adaptDeployment` sets env `CONSOLE_IMAGE: cpContext.ReleaseImageProvider.GetImage("console")`. The operator reads this env and uses it for the bridge Deployment image. Same for `DOWNLOADS_IMAGE`.

### 5. Two separate mode flags, not one multi-valued flag

CNO uses `HYPERSHIFT=true` + `--extra-clusters` as independent switches (HyperShift mode + which clusters).

**Console could use:** `--guest-kubeconfig` presence as the implicit dual-cluster mode flag (if set → dual mode; if unset → classic single-cluster). Or explicit `--dual-cluster-mode` + `--guest-kubeconfig`. The former is simpler (one flag, auto-detected mode).

---

## What console-operator should NOT copy from CNO

### 1. Annotation-based object routing

CNO's `cluster-name` annotation + `ClientFor(name)` + generic `ApplyObject` works because CNO was designed that way from the start. **Console-operator cannot adopt this without rewriting every controller** — it has typed `resourceapply.ApplyDeployment`, `ApplyService`, etc., per controller, no central apply.

**Console's path:** Manual client threading (mgmt vs guest client per constructor). **Do not** try to bolt on annotation routing — it is a larger change than threading clients through 14 constructors.

### 2. Primary client = guest, extra = management

CNO's `--kubeconfig` (primary) is the guest cluster; `--extra-clusters` is management. This makes sense for CNO (upstream operator, guest-focused by default).

**Console is different:** The ported console-operator's pod runs **in-cluster on the management side** (leader election, events, pod identity all MC). So the **in-cluster client should be MC** (or MC-guest-split for core/apps resources), and the **injected client should be guest** (all typed CR clients). This is the **inverse** of CNO's choice.

**Recommendation:** `controllerContext.KubeConfig` (in-cluster) → split into `mgmtKubeClient` (operand objects) + second config from `--guest-kubeconfig` → `guestKubeClient` + all guest-scoped typed clients. Leader election stays in-cluster (MC). This aligns with "the operator's own pod identity is MC-side."

### 3. CNO's init container kubeconfig assembly

CNO's `deployment.yaml` init container rewrites kubeconfigs from projected volumes into `/configs/hosted` and `/configs/management`.

**Console does not need this:** CPO's `InjectServiceAccountKubeConfig` already generates a **ready-to-use kubeconfig** in a secret, mounted directly. No init container rewrite needed. The operator reads `KUBECONFIG=/etc/kubernetes/kubeconfig` (the CPO-injected guest kubeconfig) and builds its in-cluster client from the pod's own SA.

### 4. CNO's `WithCustomOperandsRolloutCheckFunc`

CNO polls for its MC-side operands via a custom readiness check (`component.go:103-170`).

**Console should use standard operand tracking:** The CPO component can rely on the operator's own status (the console-operator CR's `Available` condition) to gate readiness. If an explicit MC operand check is needed, add a standard `WithDependencies(...)` on the console Deployment (verify it exists/ready in HCP namespace). Do not copy CNO's bespoke polling — use framework primitives.

---

## The key difference: CNO had annotation routing from day one; console must thread clients manually

| | CNO | Console-operator (ported) |
|---|---|---|
| **Multi-cluster support** | Native — designed in from the start | Absent — must be retrofitted |
| **Apply path** | One generic `apply.Apply(obj)`, routed by annotation | ~14 controllers, each calling typed `resourceapply.Apply*` directly |
| **Client construction** | `NewClient` builds `map[string]*OperatorClusterClient` from `--extra-clusters` | Must build second typed client set and thread through ~14 controller constructors |
| **Object routing** | Annotation on rendered object (`cluster-name`) | Per-controller client selection (manual threading) |
| **Effort shape** | N/A (already built) | Moderate mechanical refactor: build guest client, thread through constructors, sweep operand-namespace constants |

**Conclusion:** CNO proves the **topology** (control-plane operator, MC operands, guest CRs/status) works in production. Console-operator's refactor is **larger** than CNO's own port because console lacks the routing abstraction CNO had from the start. But the refactor is **mechanical** (threading clients, not rewriting logic), and the **deployment/framework side** (CPO component, sidecars, image resolution, guest kubeconfig injection) is fully paved by CNO + ingress-operator.

---

## Summary

**Copy from CNO:**

- Dual kubeconfig idiom (flag + injected guest SA kubeconfig)
- Management-side RBAC (HCP-namespace Role/RoleBinding for operand creation)
- HCP namespace awareness (downward API `NAMESPACE` env → `--operand-namespace`)
- Operand image from ReleaseImageProvider
- Two independent switches (dual-cluster mode + which kubeconfig)

**Do NOT copy from CNO:**

- Annotation-based object routing (console has typed clients; threading is manual)
- Primary client = guest (console's in-cluster client is MC; injected client is guest)
- Init container kubeconfig assembly (CPO `InjectServiceAccountKubeConfig` already does this)
- Custom operand rollout polling (use standard framework readiness checks)

**The takeaway:** CNO is the **topology precedent** (dual-cluster works in production) and the **CPO component template** (management-side RBAC, guest kubeconfig injection, image handling). Console-operator's **client-layer work** is unique to console (manual threading because console has typed clients, not annotation routing) — but the **deployment framework** is entirely CNO/ingress-operator patterns.

**Cross-references:**

- Decision + why: `../operator-migration.md`
- Operator I/O inventory: `console-operator-io-spec.md` (this directory)
- CNO component in HyperShift: `control-plane-operator/controllers/hostedcontrolplane/v2/cno/`
- CNO assets: `control-plane-operator/controllers/hostedcontrolplane/v2/assets/cluster-network-operator/`
- Upstream CNO (not in this repo): `github.com/openshift/cluster-network-operator`
