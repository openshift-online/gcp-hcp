# Findings

**Jira:** [GCP-1202](https://redhat.atlassian.net/browse/GCP-1202) — Spike: evaluate running the OpenShift console control-plane-side vs data-plane + ingress controller (GCP HCP)
**Feeds:** [GCP-926](https://redhat.atlassian.net/browse/GCP-926) — Define and Implement Customer Ingress Strategy for GCP HCP

Feasibility study and proof of concept for relocating the OpenShift web console
from the guest data plane to the HostedControlPlane namespace on the management
cluster. Everything below was validated live on real GCP HostedClusters.

Diagrams for all of it are in [architecture.md](architecture.md).

---

## Verdict

**Feasible.** The core console is a stateless Go reverse proxy that needs only
the guest kube-apiserver and an auth issuer, both already reachable from the HCP
namespace. It runs on a zero-node guest cluster, and it is exposed on the
existing shared HAProxy router and public LB / Private Service Connect rails —
no guest-side ingress infrastructure involved. Per-user Google OIDC login works
unchanged.

The PoC demonstrated, live in a browser: core console resource browsing, the pod
terminal, monitoring via Thanos and Alertmanager, and dynamic plugin loading.

### Proven

- Core console: browse guest Kubernetes and OpenShift resources, per-user Google
  OIDC login, CLI downloads, exposed via both public LB and PSC endpoint.
- Zero-node guest cluster: console reachable and functional with no workers.
- Cross-plane data paths: guest kube-apiserver via an in-namespace ClusterIP;
  the guest monitoring stack via the konnectivity tunnel.
- Pod terminal and logs; Observe → Metrics.
- Dynamic plugin loading: the `monitoring-plugin` ConsolePlugin, served from the
  guest data plane, rendering Observe → Alerting / Dashboards / Targets.
- Multi-replica availability: two bridge replicas with OIDC refresh-token
  session recovery, proven by deleting the pod holding the session and observing
  zero re-authentication.
- TLS: the console serving certificate reuses the API server's wildcard
  certificate; service-ca trust for guest monitoring services.

### Not proven

- **console-operator migration.** The operator was not ported. All
  configuration, plugin enablement and lifecycle management were hand-applied.
- **Guest-side config reconciliation.** The operator normally reconciles guest
  ConsolePlugin CRs, generates `console-config`, and writes status. All of that
  was bypassed with static bridge flags.
- **Zero-touch OIDC client provisioning.** The PoC used a pre-created Google
  OAuth client, applied by hand. A day-2 model where the customer supplies their
  own client now has a design and an ARO precedent, so this is no longer a
  blocker — but provisioning without *any* manual Google step remains unsolved,
  because Google has no API for creating OAuth web clients. It may also become
  moot: if GCP HCP moves to the internal OpenShift OAuth server for
  multiple/custom identity providers, the console delegates to `openshift-oauth`
  and needs no Google client. Keep the bridge's auth pluggable. See
  [open-questions.md](open-questions.md) §1.
- **Capability-aware payload stripping.** CVO payload removal of the guest
  console-operator works, but is GCP-gated with hardcoded manifest filenames
  rather than productized.

---

## Why move it

Five operational benefits, all borne out by the spike.

**Removes the default-IngressController dependency.** Today that ingress exists
*solely* to serve the console, and because the platform maintains it for that
purpose, customers cannot create their own ingress resources on day two. With a
control-plane-side console the guest Ingress capability can be disabled
outright, and the console becomes immune to guest-side ingress changes.

**Console available on day 0, and with zero worker nodes.** Administrators can
log in, browse resources and debug control-plane components before the data
plane is scaled — much as they can with the API today.

**Certificate management stops depending on guest-side configuration.** Console
TLS terminates at the pod using the same cert-manager-issued wildcard as the API
server. The lifecycle is owned entirely by the management cluster: no guest CVO
payload, no guest CertificateRequests, no service-ca operator.

**The component becomes SLO-able.** Running on the management cluster puts the
console under the same SLO ownership as the kube-apiserver and the OAuth server,
rather than leaving it a guest workload whose availability depends on customer
data-plane health.

**Reduced blast radius.** Guest-side NetworkPolicies, Route changes,
IngressController replacement or service-mesh experiments cannot take the
console down.

> The reverse of that last point is a real concern raised in review: a
> control-plane workload whose *inputs* live in the data plane opens a different
> attack surface. See [open-questions.md](open-questions.md).

---

## What was proven, by phase

### Phase 1 — core console control-plane-side

**Deployed.** Console bridge Deployment in the HCP namespace, exposed via a
Route on the shared HAProxy router (SNI passthrough), reachable over the public
LB for `PublicAndPrivate` clusters and over the PSC endpoint for `Private`
clusters. The bridge ran in off-cluster mode (`-k8s-mode=off-cluster`) pointed at
the in-namespace guest kube-apiserver ClusterIP Service. Per-user Google OIDC
with `email` and `profile` scopes. A CLI downloads server deployed alongside.

**Verified live.** Logged in with a real Google identity; browsed guest
Namespaces, Pods and Deployments; loaded the Command Line Tools page and
downloaded `oc`. Reachable on both the public LB and the PSC endpoint hostname.
Functioned identically on a zero-node cluster and later on a four-worker one.

**Changes required.**

- *CPO router backends for the console and downloads Routes.* The HCP router
  built backends only for hardcoded route names; this was generalized to
  labeled-Route discovery, and CPO took ownership of the console and downloads
  exposure Routes including the `-private` variants for PSC.
  [openshift/hypershift#9622](https://github.com/openshift/hypershift/pull/9622).
- *Bridge `-ca-file` trust for the guest kube-apiserver.* The off-cluster bridge
  could not verify the guest KAS private `root-ca`; skip-verify was the only
  workaround. Fixed in three places: the KAS resource proxy, the anonymous
  transport used by user-settings and login metrics, and later the service proxy
  transport.
  [openshift/console#17185](https://github.com/openshift/console/pull/17185),
  GCP-1219.
- *OIDC client registration workaround.* Proper
  `oidcProviders[].oidcClients[]` registration is blocked because the guest
  admission rule requires a matching `status.oidcClients`, which only a running
  console-operator writes. The PoC instead added the console client ID to the
  guest KAS OIDC `audiences` and passed the client configuration to the bridge as
  flags.

**Finding.** The guest kube-apiserver is an ordinary in-namespace ClusterIP
Service, so the entire core console path needs **no konnectivity tunnel**. This
removes a major assumed constraint and is the single most important structural
result of Phase 1.

### Phase 2 — pod terminal and monitoring

**Pod terminal** worked with zero new plumbing. The terminal opens a WebSocket
to the Kubernetes resource proxy, which proxies through the guest kube-apiserver
to the kubelet, governed by the logged-in user's guest RBAC.

**Monitoring** needed a tunnel: Thanos and Alertmanager are guest ClusterIP
Services, unreachable from a control-plane pod.

**Deployed.** A `konnectivity-socks5-proxy` sidecar on the console pod, with
`HTTP_PROXY`/`HTTPS_PROXY` pointing at `socks5://127.0.0.1:8090` and a `NO_PROXY`
that keeps the in-namespace guest kube-apiserver off the tunnel. Bridge flags
`-k8s-mode-off-cluster-thanos` and `-k8s-mode-off-cluster-alertmanager`. A
`service-serving-ca` ConfigMap mounted for `-service-ca-file`.

**Verified live.** Observe → Metrics rendered live graphs; PromQL queries
returned real data, having travelled from the browser through the console,
through the socks5 sidecar, through the konnectivity tunnel into the guest pod
network, to `thanos-querier`.

**Changes required.**

- *`-service-ca-file` for service proxies.* Guest services present
  service-ca-signed certificates — a different signer than the kube-apiserver
  CA. Folded into
  [openshift/console#17185](https://github.com/openshift/console/pull/17185).
- *Guest VPC geneve firewall rule.* The first live test returned 504s because
  the guest VPC dropped OVN-Kubernetes geneve UDP/6081 between nodes, breaking
  all cross-node pod networking and therefore konnectivity. Fixed with an
  ingress allow rule; productized as a CPO firewall reconciler in
  [openshift/hypershift#9640](https://github.com/openshift/hypershift/pull/9640)
  (GCP-1221).

**Finding.** Observe → Alerting / Dashboards / Targets did *not* render. Those
tabs are the `monitoring-plugin` dynamic ConsolePlugin, not core console — the
Alertmanager backend was reachable and alerts were firing, but the UI surface was
simply absent. This shaped Phase 3.

### Phase 3 — dynamic plugins and the Console capability

**Goal.** Prove a dynamic ConsolePlugin loads and renders in a control-plane-side
console with the guest Console capability enabled but the console-operator
running nowhere.

**Deployed.** The HostedCluster recreated with the Console capability enabled and
the Ingress capability disabled, which required relaxing a CEL admission rule.
CPO's CVO payload script modified to strip both the console-operator Deployment
manifest and the `ClusterOperator/console` manifest, GCP-gated. The
cluster-monitoring-operator then reconciled the `monitoring-plugin` Deployment,
Service and ConsolePlugin CR in the guest. The bridge was given
`-plugins=monitoring-plugin=https://...` directly, since there was no operator to
write enabled plugins into `console-config`. A `token-minter` native init
sidecar was added to give the bridge its own guest service-account identity.
The bridge was changed to request offline access so Google returns a refresh
token.

**Verified live.** Observe → Alerting rendered with live alert data, Dashboards
rendered real ConfigMap-driven dashboards, Targets displayed Prometheus scrape
targets — the plugin bundle fetched from the guest Service through the socks5
proxy and the service-ca trust chain. With `replicas: 2`, deleting the pod
holding the user session produced zero re-authentication: the other replica
rebuilt the session via a silent OIDC refresh.

**Changes required.**

- *GCP-gated CEL relaxation.* Upstream CEL forbids disabling Ingress unless
  Console is also disabled. Relaxed to a GCP-only rule, with regenerated CRDs and
  envtest coverage. Converges with
  [OCPBUGS-58422](https://redhat.atlassian.net/browse/OCPBUGS-58422);
  [openshift/console-operator#1182](https://github.com/openshift/console-operator/pull/1182)
  is merged and
  [openshift/hypershift#8933](https://github.com/openshift/hypershift/pull/8933)
  removes the rule for all platforms, which will replace the GCP-only relax.
- *GCP-gated console-operator strip.* Both the operator Deployment manifest and
  the `ClusterOperator/console` manifest must be stripped. Strip only the first
  and the guest CVO blocks forever on `ClusterOperatorNotAvailable`.
- *Plugin asset proxy fix.* The bridge's plugin asset transport hand-built an
  `http.Transport` with only `TLSClientConfig` and no `Proxy`, so it ignored
  `HTTP_PROXY` and could not reach guest services. Fixed with
  `Proxy: http.ProxyFromEnvironment`, folded into
  [openshift/console#17185](https://github.com/openshift/console/pull/17185).
- *Bridge service-account identity via token-minter.* Under `-user-auth=oidc`,
  user requests carry the user's token, but the bridge's own backend calls
  (Dashboards ConfigMaps, plugin metrics) need a service account. A token-minter
  native init sidecar creates the guest `console` SA, mints a KAS-audience token
  into a shared in-memory file and refreshes it; the bridge reads it via
  `-k8s-mode-off-cluster-service-account-bearer-token-file`. The Console
  capability's own RBAC already binds that SA.
- *Multi-replica OIDC refresh-token recovery.* The bridge names its session
  cookie per pod but can only recover a cross-pod session from a refresh-token
  cookie, and it never requested one — so a second replica forced a re-login
  loop. Fixed by requesting `access_type=offline` plus `prompt=consent`; Google
  does not honour the standard `offline_access` scope. No sticky routing is
  needed, which matters because the SNI-passthrough router cannot do cookie
  affinity.

**Finding.** Enabling the Console capability replaces all of the Phase 1
hand-rolled guest scaffolding. The guest CVO installs the Console CRDs, the
`openshift-console*` namespaces and all the RBAC. Only the `oc-cli-downloads`
CR remains hand-applied, because the console-operator that would normally create
it is stripped.

**Finding.** Cross-region OIDC works unchanged: a HostedCluster can be recreated
in a new region reusing the previous region's OIDC issuer and published JWKS.

### Phase 4 — the console-operator (design only)

Not implemented. The decision — port the upstream console-operator to
dual-kube-API mode and deploy it as a CPOv2 control-plane component, with zero
console business logic in CPO — is recorded with its rejected alternatives and
its sizing in [operator-migration.md](operator-migration.md).

---

## Key technical findings

**The guest kube-apiserver is an in-namespace ClusterIP.** The core console
proxy path reaches it directly over the service network. Only guest ClusterIP
services — the monitoring stack and plugin backends — need tunnelling.

**OIDC client registration is blocked by a status-writer dependency.** The
proper `oidcProviders[].oidcClients[]` entry is not admissible without a running
console-operator to write `status.oidcClients` first. Productization needs a
control-plane-side owner of that status.

**HCP namespaces enforce restricted PSA, and the upstream console already
complies.** Every control-plane-side component must ship a restricted-compliant
`securityContext` or it never schedules. The upstream console-operator's static
Deployment asset already ships exactly that, and the console image runs as
non-root. There is no PSA gap for the console itself.

**Observe → Alerting / Dashboards is the monitoring-plugin, not core console.**
Metrics is core console; Alerting, Dashboards and Targets come from a dynamic
ConsolePlugin shipped by the cluster-monitoring-operator. Enabling them needs the
Console capability, the `consoleplugins` CRD and plugin enablement.

**Enabling the Console capability replaces the hand-rolled guest scaffolding.**
CRDs, namespaces and RBAC all arrive via CVO.

**The Service port must be 8443, not the upstream 443,** because the CPO router
dials `ClusterIP:8443` directly. Consistent with how the kube-apiserver and
OAuth server are exposed.

---

## Zero-node validation

Demonstrated on a cluster with no worker nodes:

- Console UI reachable via both the public LB and the PSC endpoint.
- Google OIDC login completed with a real identity.
- Resource browsing functional — Namespaces, Pods, ClusterOperators.
- CLI downloads page loaded and `oc` binaries served.

Not demonstrated on zero nodes, because the backing workloads run on guest
workers:

- Pod terminal and logs.
- Monitoring — Thanos, Alertmanager and the monitoring-plugin all run guest-side.
- Dynamic plugin loading, for the same reason.

The core console works with zero nodes. Day-1 features need at least one worker,
which matches the design: the console *frontend* moves to the control plane, the
plugin and monitoring *workloads* stay guest-side.

---

## Comparison vs the status quo

| Dimension | Status quo: data-plane console | Control-plane-side console |
|---|---|---|
| Day-0 availability | Needs the default IngressController, Route admission, and a worker to schedule on | Available immediately, no guest ingress dependency |
| Zero-node support | Not supported — unreachable without workers | Core console supported |
| Ingress dependency | Requires a default IngressController that exists only for the console, which blocks customers owning day-2 ingress | Ingress capability can be disabled; customers own ingress from day 1 |
| Certificate management | Depends on guest CVO payload, service-ca, and guest Route TLS config | Reuses the API server wildcard; lifecycle entirely management-side |
| SLO ownership | Guest workload; availability tied to customer data-plane health | Management-cluster component, same tier as the kube-apiserver |
| Blast radius from customer changes | Guest NetworkPolicies, Route changes, ingress replacement or a service mesh can break it | Customer data-plane changes cannot break it — but see the inverse concern in [open-questions.md](open-questions.md) |
| Plugin / monitoring reach | Guest-side, direct Service access | Guest-side, reached over the konnectivity tunnel — one extra hop, untested at scale |
| Hostname | `console-openshift-console.apps.<basedomain>` | `console.<domain>`, next to `api.<domain>` — a user-visible migration |
| Custom domain | Works today: `Ingress.spec.componentRoutes`, cert terminated at the ingress router | Needs new work — the passthrough router cannot terminate a custom cert, so the cert must reach the pod and the bridge must do SNI selection. [open-questions.md](open-questions.md) §3 |
| Implementation cost today | Zero, already shipping | A CPOv2 component plus a broad mechanical console-operator refactor, plus the unresolved OIDC client-provisioning problem |

The status quo wins on implementation cost and on plugin latency. The
control-plane-side model wins on operational resilience, day-0 availability and
customer ingress freedom. Both work; the choice is operational strategy against
incremental cost.

---

## Cross-cutting issue found in passing

### cluster-network-operator restricted-PSA gap — not console-specific

**Problem.** cluster-network-operator renders its self-managed network operand
Deployments from its own bindata, setting only pod-level security fields. The
container-level fields that `restricted` PSA requires are missing. On an
SCC-less management cluster — which GKE is — Kubernetes PSA becomes the
admission authority, the Deployments fail admission, and guest nodes never come
up at all.

**Impact.** Blocks node bring-up entirely. Nothing to do with the console; it
simply surfaced here because the PoC needed workers.

**Workaround, applied live.** The
`hypershift.openshift.io/pod-security-admission-label-override: baseline`
annotation on the HostedCluster. Nodes then came up.

**Root cause.** CNO's bindata templates predate PSA and were written against
OpenShift SCC.

**Fix needed.** Container-level security fields in CNO's bindata. Not yet filed;
this needs its own ticket and is not tracked under any of the console Jiras.

---

## Next steps

1. **Confirm the GCP OIDC console client model.** The day-2 design — customer
   brings a pre-existing Google client, client ID set in `HostedCluster.spec` at
   creation, redirect URI and secret supplied day-2 outside the spec — follows
   ARO HCP's shipped pattern. What is needed now is the product decision that
   customers own the client, plus the three code changes listed in
   [open-questions.md](open-questions.md) §1. Background in
   [reference/console-auth-options.md](reference/console-auth-options.md).
2. **Upstream console-operator dual-client refactor**, framed as "manage console
   on a remote cluster" following CNO's merged precedent. Scoped in
   [operator-migration.md](operator-migration.md).
3. **CPOv2 console-operator component**, modelled on `ingress-operator`.
4. **Capability-aware payload stripping** to replace the GCP-gated
   `manifestsToOmit` strip, including migration when the flag flips.
5. **Converge on OCPBUGS-58422** — drop the GCP-only CEL relax once
   [openshift/hypershift#8933](https://github.com/openshift/hypershift/pull/8933)
   merges.
6. **Router config hot-reload**, to remove the manual router restart after a
   Route change.
7. **Nothing to reconcile on certificates.** Self-signed is the HyperShift
   default; GCP HCP already supplies a Let's Encrypt wildcard that covers
   `api.<domain>`, `oauth.<domain>` and now `console.<domain>`. Recorded in
   [open-questions.md](open-questions.md) §5 only because the two statements can
   look contradictory.

The runtime path is proven and the engineering cost is bounded. The OIDC client
model, previously the gating question, now has a day-2 design with an ARO
precedent; what is left there is a product decision rather than a technical
unknown. The remaining open question of substance is whether the console team
will accept a remote-cluster operating mode upstream.

See also [architecture.md](architecture.md),
[operator-migration.md](operator-migration.md),
[upstream-changes.md](upstream-changes.md) and
[cost-analysis.md](cost-analysis.md).
