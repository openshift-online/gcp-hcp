# Console kustomize layers

Three layers, each building on the last:

| Layer | What it is | Builds standalone? |
|---|---|---|
| `origin/` | Verbatim upstream `openshift/console-operator` static assets for the `console` **and** `downloads` operands (deployment/service/route/pdb/serviceaccount each). Cited, unmodified, for diffing. Vendored from openshift/console-operator @ 7fa0a807acb45ccb18f3ce8b8f32d588ccdc469e. | No — has operator-only placeholders (`${IMAGE}`, no volumes, no `spec.host`). |
| `hypershift/` | Generic, cluster-agnostic patch of `origin` for running the core console control-plane-side with **no console-operator** (Phase 1). Every patch is commented with *why* (what the operator would otherwise inject). | No — still has `REPLACE_*` placeholders. |
| `example-overlay/` | Per-HostedCluster overlay: real namespace, hostnames, image digest, OIDC client ID, and session keys for the live `console-demo` cluster. | Yes — `./apply.sh` deploys it. |

Add a new per-cluster overlay by copying `example-overlay/` and swapping its values; `hypershift/` should rarely need to change.

## Route ownership

CPO creates the console/downloads exposure Routes: per `endpointAccess` it reconciles a public route, or a `-private` route (labeled `route-visibility=private`) plus a matching ExternalName service for external-dns under Private, deriving the host from the APIServer host (`api.<domain>` → `console.<domain>` / `downloads.<domain>`). This tree manages the Deployments/Services/PDBs/Secrets; the console Deployment's `-base-address` (set in the per-cluster overlay) must match CPO's derived host.

## Downloads operand (CLI download server)

The `downloads` operand serves the `oc`/CLI download page. Two Phase 1 notes:

- **TLS sidecar:** the HCP router is SNI passthrough only and can't do the upstream downloads Route's `edge` TLS termination, so the pod runs an `oauth-proxy` sidecar (auth bypassed via `-skip-auth-regex=^/`) that terminates TLS on 8443 and forwards to the verbatim upstream download-server on `127.0.0.1:8080`.
- **UI "Command Line Tools" link:** the page lists `ConsoleCLIDownloads` CRs read (browser-side) from the guest. The guest has the `Console` capability **enabled** (with Ingress disabled), so CVO installs the CRD automatically, but the console-operator (which normally creates the CR instances) is stripped from the CVO payload. We supply the `oc-cli-downloads` CR by hand in `../guest/` (pointing at the downloads host). Other CLI-download CRs (helm, netobserv) come from their own operators/CVO. Open question — who installs the CRD and owns the CR long-term (normally the Console capability + console-operator): see ../../findings.md and ../../open-questions.md.

## `origin/` → `hypershift/` deltas (what we changed vs stock, and why)

Diffing against `origin/` makes every departure from upstream an explicit, reviewable patch. The real deltas:

| Field | Upstream (`origin/`) | `hypershift/` | Why |
|---|---|---|---|
| `priorityClassName` | `system-cluster-critical` | `hypershift-control-plane` | Guest-cluster default; the pod now runs on the management cluster (matches every other HCP-ns operator Deployment). |
| `nodeSelector`/`tolerations` (`node-role.kubernetes.io/master`) | present | removed | Guest-master concept; meaningless on the management cluster. |
| `serviceAccountName`/`serviceAccount` | `console` | removed (`automountServiceAccountToken: false`) | Bridge uses a guest ServiceAccount token minted by the `token-minter` native init-sidecar (added in `hypershift/`), not a management-side SA identity. The token is auto-refreshed and mounted from a shared emptyDir at `/var/run/console-sa/token`, consumed via `-k8s-mode-off-cluster-service-account-bearer-token-file`. |
| Service `spec.ports[0].port` | `443` (→ `targetPort: 8443`) | `8443` | **Functional.** CPO router's console case hardcodes `DestinationPort: 8443` and dials `ClusterIP:8443` directly (`v2/router/config.go:136-138`); upstream `443` leaves nothing on 8443. |
| Service annotation `service.beta.openshift.io/serving-cert-secret-name` | present | removed | No service-ca operator on GKE. |
| Route `spec.host` | unset (guest admission fills default) | explicit | No route-admission for a hand-applied mgmt-cluster Route. |
| Route `spec.tls.termination` | `reencrypt`+`Redirect` | `passthrough`/`None` | Router is SNI-passthrough only; bridge terminates its own TLS. |
| `spec.replicas` | unset (operator computes) | `2` | **Multi-replica now works.** The patched bridge requests `access_type=offline` + `prompt=consent` (openshift/console#17185) so Google issues a refresh token. When a request is load-balanced to a replica with no in-memory session, the bridge silently rebuilds the session via a back-channel token refresh (no interactive re-auth loop). The HCP router is SNI-passthrough so it can't do cookie affinity, and Service sessionAffinity:ClientIP is useless (the router dials the Service fresh so it only sees the router pod IP); the refresh-token cookie is what makes multi-replica work without affinity. |
| Container `command`/`args`, `env`, `volumeMounts`, `volumes` | operator-generated (`--config=console-config.yaml` + injected volumes) | static off-cluster CLI flags + `serving-cert`/`guest-ca`/`oidc-client-secret`/`session-keys`/`service-ca`/`console-sa-token` volumes | No console-operator to generate `console-config.yaml`/inject auth volumes. Flags set: `-k8s-mode=off-cluster`, `-k8s-mode-off-cluster-endpoint=https://kube-apiserver.<hcp-namespace>.svc:6443`, `-ca-file=/var/run/guest-ca/ca.crt` (guest KAS root-ca trust), `-k8s-public-endpoint`, `-user-auth=oidc` + `-user-auth-oidc-issuer-url=https://accounts.google.com` + `-user-auth-oidc-client-id` + `-user-auth-oidc-client-secret-file` + `-user-auth-oidc-token-scopes=email,profile`, `-cookie-encryption-key-file` + `-cookie-authentication-key-file`, `-tls-cert-file` + `-tls-key-file`, `-branding=ocp`, `-k8s-mode-off-cluster-thanos` + `-k8s-mode-off-cluster-alertmanager` (Phase 2 monitoring, tunneled via `konnectivity-proxy-socks5` sidecar), `-service-ca-file=/var/run/service-ca/service-ca.crt` (Phase 2 service-serving cert trust for Thanos/Alertmanager/terminal/plugins), `-plugins=monitoring-plugin=...` (Phase 3 dynamic plugin), and `-k8s-mode-off-cluster-service-account-bearer-token-file=/var/run/console-sa/token` (Phase 3 bridge backend identity). |
| `POD_NAME` env (downward API `metadata.name`) | injected at runtime by the operator (`deployment.go`, *not* in the static bindata) — "console distinguishes cookie sessions by pod names in OIDC envs" | added in `hypershift/` | Multi-replica OIDC session correctness: the bridge names its session cookie `<cookie>-$POD_NAME` and expires other pods' cookies. Without it both replicas share one cookie name. This is why it's absent from `origin/` (that file is the *verbatim static bindata*; the operator appends this env in Go at apply time). |
| `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` env | absent | `socks5://127.0.0.1:8090` / `socks5://127.0.0.1:8090` / `kube-apiserver.<hcp-namespace>.svc,localhost,127.0.0.1` | Phase 2: routes Thanos/Alertmanager/plugin proxy dials through the `konnectivity-proxy-socks5` sidecar (added below) into the guest service network. NO_PROXY keeps the in-namespace KAS dial direct (not tunneled). |
| Sidecars | none | `konnectivity-proxy-socks5` (regular container) + `console-sa-token-minter` (native init-sidecar with `restartPolicy: Always`) | Phase 2/3: `konnectivity-proxy-socks5` tunnels the bridge's HTTP(S)_PROXY traffic into the guest service network via the reverse konnectivity tunnel, resolving guest Service → ClusterIP from the guest API (via KUBECONFIG). Shape mirrors the framework's InjectKonnectivityContainer({Mode: Socks5}). `console-sa-token-minter` mints and continuously refreshes a token for the guest `console` ServiceAccount (creating the SA if absent), giving the bridge's own backend calls (Dashboards ConfigMaps, plugin metrics) a real identity instead of system:anonymous. Shape mirrors the framework's InjectTokenMinterContainer. |

**Deliberately unchanged from `origin/`** (upstream choices that already fit): the restricted-PSA `securityContext` (pod `runAsNonRoot`/`seccompProfile`, container `allowPrivilegeEscalation: false`/`capabilities.drop: [ALL]`/`readOnlyRootFilesystem: true`), the `target.workload.openshift.io/management` annotation, probes, ports, resources, and the `{app: console, component: ui}` selector.

Two gotchas hit during the restructure:
- **Deployment `spec.selector` is immutable** — adopting upstream's real `{app: console, component: ui}` selector required delete+recreate of the live Deployment (Service/PDB selector changes apply in place).
- **kustomize `images:` can't match `${IMAGE}`** — upstream's `image: ${IMAGE}` is the operator's own Go placeholder; kustomize parses `images:` targets as docker refs and silently no-ops on `${IMAGE}`. Fix: JSON6902-`replace` the field to a valid-looking placeholder (`REPLACE_CONSOLE_IMAGE_REGISTRY`) in `hypershift/`, then let the per-cluster `images:` transform target that.

See ../../findings.md (what was deployed), ../../architecture.md (design), and ../../upstream-changes.md (upstream patch tracking) for what this implements.
