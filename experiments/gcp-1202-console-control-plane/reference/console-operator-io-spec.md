# console-operator I/O specification — dual-cluster refactor input

**Purpose:** Exhaustive inventory of what the OpenShift `console-operator` reads and writes, with target cluster (management vs guest) classification for the dual-cluster refactor. This is the raw input to `../operator-migration.md`.

**Source:** Reverse-engineered from the `openshift/console-operator` codebase (`pkg/console/starter/starter.go:RunOperator` as entry point). Line numbers are approximate and must be re-verified against the current upstream branch before implementation.

**Key constants** (`pkg/api/api.go`):
- `TargetNamespace = "openshift-console"` (`:41`) — operand namespace, must become parameterizable
- `OpenShiftConsoleNamespace` (`:64`) — alias for `TargetNamespace`
- `openshift-config` (`:26`), `openshift-config-managed` (`:25`), `openshift-console-operator` (`:30`)
- Cluster-scoped CR name: `cluster` (`:8`)
- ClusterOperator name: `console` (`:7`)

**Client construction** (today = single kubeconfig):
- All clients derive from `controllerContext.KubeConfig` or `.ProtoKubeConfig` (`starter.go:94-129`)
- `kubeClient` (core/apps), `configClient` (config.openshift.io), `operatorConfigClient` (operator.openshift.io), `consoleClient` (console.openshift.io), `routesClient`, `oauthClient`, `dynamicClient`, `policyClient`, `operatorClient`

---

## Table 1 — Resources READ by the operator

| Kind | Group/Version | Name | Namespace | Fields consumed | Read by (controller) | File:line (approx) | Target cluster |
|---|---|---|---|---|---|---|---|
| **Console** (operator CR) | operator.openshift.io/v1 | cluster | — | `.spec` (ManagementState, Ingress.ConsoleURL/ClientDownloadsURL, Route.Hostname/Secret, Customization.*, Plugins, Providers.Statuspage, LogLevel, ObservedConfig, UnsupportedConfigOverrides, telemetry annotations); `.status.generations` | ConsoleOperator + all sync controllers | operator.go:316; sync_v400.go:72,124,461-513; deployment.go:401 | **Guest** |
| **Console** (config CR) | config.openshift.io/v1 | cluster | — | `.spec.authentication.logoutRedirect`; `.status.consoleURL` | ConsoleOperator | operator.go:329; sync_v400.go:297; configmap.go:87 | **Guest** |
| **Infrastructure** | config.openshift.io/v1 | cluster | — | `.status.apiServerURL`, `.status.controlPlaneTopology`, `.status.infrastructureTopology`, `.status.platformStatus.type` | ConsoleOperator, Route, Service, Downloads*, HealthCheck, OAuthClients, CLIDownloads, ServiceAccounts | operator.go:336; infrastructure/cluster.go:5-8; deployment.go:142-147,428; starter.go:242 | **Guest** |
| **ClusterVersion** | config.openshift.io/v1 | version | — | `.status.capabilities.enabledCapabilities`; `.spec.clusterID`; `.status.history[]/desired.version`; `.status.conditions` | starter, ConsoleOperator, Route, Service, OAuthClients, CLIDownloads, UpgradeNotification, telemetry | starter.go:246; util.go:90-100; telemetry.go:55-61 | **Guest** |
| **Proxy** | config.openshift.io/v1 | cluster | — | `.status.httpsProxy`, `.status.httpProxy`, `.status.noProxy`; `resourceVersion` | ConsoleOperator (deployment env) | operator.go:342; deployment.go:502-525,255 | **Guest** (see note 1) |
| **OAuth** | config.openshift.io/v1 | cluster | — | `.spec.tokenConfig.accessTokenInactivityTimeout` | ConsoleOperator (configmap) | operator.go:348; sync_v400.go:455-457 | **Guest** |
| **Ingress** | config.openshift.io/v1 | cluster | — | `.spec.domain`; `.spec.componentRoutes[]`; `.status.componentRoutes[]`; `.spec.loadBalancer.platform.*` | ConsoleOperator, Route, Service, OAuthClients, CLIDownloads, HealthCheck | operator.go:354; route/route.go:60-124,365-407; healthcheck controller.go:266 | **Guest** |
| **Authentication** | config.openshift.io/v1 | cluster | — | `.spec.type`; `.spec.oidcProviders[]` (Issuer.URL, CA.Name, Name, OIDCClients[]: ClientID/ClientSecret.Name/ExtraScopes/ComponentName/ComponentNamespace) | ConsoleOperator, OAuthClients, OAuthClientSecret, OIDCSetup, CLIOIDCClientStatus, SwitchedInformer | sync_v400.go:110,119-150; oauthclientsecret.go:104-149; oidcsetup.go:130-273; authentication/cluster.go:26-45 | **Guest** |
| **FeatureGate** | config.openshift.io/v1 | cluster | — | `.spec.featureSet`; `.status.featureGates[].enabled` | ConsoleOperator (TechPreview/OLM), featureGateAccessor (ExternalOIDC) | sync_v400.go:683-717; starter.go:218-237,327 | **Guest** |
| **APIServer** | config.openshift.io/v1 | cluster | — | `.spec.tlsSecurityProfile` | ConfigObserver | observe_config_controller.go:31,40,49 | **Guest** |
| **ClusterOperator** | config.openshift.io/v1 | console | — | `.status` (read-modify-write) | ClusterOperatorStatusController | starter.go:512-538 | **Guest** |
| **ConsolePlugin** | console.openshift.io/v1 | (enabled plugins) | — | `.spec.backend.*`, `.spec.proxy[]`, `.spec.i18n.loadType`, `.spec.contentSecurityPolicy` | ConsoleOperator (GetAvailablePlugins) | sync_v400.go:461,886-901; configmap.go:149-262 | **Guest** |
| **ConsoleCLIDownload** | console.openshift.io/v1 | oc-cli-downloads | — | `.spec` (compare for drift) | CLIDownloads | clidownloads controller.go:245-289 | **Guest** |
| **Route** | route.openshift.io/v1 | console, console-custom, downloads, downloads-custom, additional | openshift-console | `.spec.host`; `.spec.tls.certificate`; `.status.ingress[]` | ConsoleOperator (read-only), Route, HealthCheck, OAuthClients, CLIDownloads | sync_v400.go:88; route/route.go:274-284; healthcheck controller.go:139-173 | **Dropped** (see note 2) |
| **Secret** | v1 | console-oauth-config | openshift-console | `.data.clientSecret`; `resourceVersion` | ConsoleOperator, OAuthClients, OAuthClientSecret, OIDCSetup | sync_v400.go:223; oauthclientsecret.go:99,162 | **MC** (operand ns) |
| **Secret** | v1 | console-serving-cert | openshift-console | `resourceVersion` (mounted) | ConsoleOperator | sync_v400.go:229; deployment.go:259 | **MC** (operand ns) |
| **Secret** | v1 | session-secret | openshift-console | `.data` (session keys) | ConsoleOperator | sync_v400.go:969 | **MC** (operand ns) |
| **Secret** | v1 | custom/componentRoute TLS | openshift-config | `tls.crt`, `tls.key` | Route (custom route certs) | route controller.go:280-341,375 | **Guest** (source), **MC** (dest) — see §C |
| **Secret** | v1 | OIDC clientSecret ref | openshift-config | `.data.clientSecret` | OAuthClientSecret | oauthclientsecret.go:133-139 | **Guest** |
| **Secret** | v1 | pull-secret | openshift-config | `.dockerconfigjson` (cloud.openshift.com auth for telemetry) | ConsoleOperator (telemetry) | telemetry.go:72-92 | **Guest** |
| **ConfigMap** | v1 | console-config | openshift-config-managed | first `.data` value (managed overlay) | ConsoleOperator | sync_v400.go:431-437; configmap.go:297-304 | **Guest** |
| **ConfigMap** | v1 | monitoring-shared-config | openshift-config-managed | alertmanager hosts | ConsoleOperator | sync_v400.go:463-469; config_builder.go:245-249 | **Guest** |
| **ConfigMap** | v1 | service-ca | openshift-console | injected CA (empty on create, service-ca populates) | ConsoleOperator | sync_v400.go:592-635 | **MC** (operand ns) |
| **ConfigMap** | v1 | trusted-ca-bundle | openshift-console | `ca-bundle.crt` (egress trust) | ConsoleOperator, HealthCheck | sync_v400.go:637-679; healthcheck controller.go:237 | **MC** (operand ns) |
| **ConfigMap** | v1 | oauth-serving-cert | openshift-console | `ca-bundle.crt` (auth server CA) | ConsoleOperator, HealthCheck | sync_v400.go:764-775; healthcheck controller.go:237 | **MC** (operand ns, see note 3) |
| **ConfigMap** | v1 | default-ingress-cert | openshift-console | `ca-bundle.crt` (ingress CA) | HealthCheck | healthcheck controller.go:237-243 | **Dropped** (note 3) |
| **ConfigMap** | v1 | oauth-serving-cert, default-ingress-cert | openshift-config-managed | `.data` (sync source) | ResourceSyncController | starter.go:743-757 | **Guest** (note 3) |
| **ConfigMap** | v1 | OIDC provider CA | openshift-config / openshift-console | `ca-bundle.crt` | OIDCSetup, ConsoleOperator | oidcsetup.go:210-224; sync_v400.go:125 | **Guest** (source), **MC** (dest) |
| **ConfigMap** | v1 | custom logo | openshift-config | `.data` / `.binaryData[key]` | ConsoleOperator (logo sync) | sync_v400.go:844-856 | **Guest** |
| **ConfigMap** | v1 | telemetry-config | openshift-console-operator | `.data` | ConsoleOperator | sync_v400.go:550-559 | **MC** (operator ns) |
| **Node** | v1 | (all) | — | `.metadata.labels` (arch/os) | ConsoleOperator (node compute environments) | sync_v400.go:438-442,929-947 | **Guest** (note 4) |
| **Deployment** | apps/v1 | console | openshift-console | `.status.replicas`, `.status.conditions`; `.metadata.annotations` | ConsoleOperator, OIDCSetup | sync_v400.go:255-272; oidcsetup.go:245-273 | **MC** (operand ns) |
| **Deployment** | apps/v1 | downloads | openshift-console | `.metadata.generation` | DownloadsDeployment | downloadsdeployment controller.go:117-133 | **MC** (operand ns) |
| **Deployment** | apps/v1 | telemeter-client | openshift-monitoring | `.status.availableReplicas` | ConsoleOperator (telemetry check) | telemetry.go:41-52 | **Guest** |
| **Service** | v1 | console, downloads, console-redirect | openshift-console | applied/compared (Service controller) | ServiceController | service controller.go:132-195 | **MC** (operand ns) |
| **ServiceAccount** | v1 | console, downloads | openshift-console | applied (ServiceAccount controller) | ServiceAccounts | serviceaccounts controller.go:138-179 | **MC** (operand ns, see note 5) |
| **PodDisruptionBudget** | policy/v1 | console, downloads | openshift-console | applied (PDB controller) | PDB | poddisruptionbudget controller.go:99-121 | **MC** (operand ns) |
| **IngressController** | operator.openshift.io/v1 | default | openshift-ingress-operator | `.spec.defaultCertificate` | Route (custom cert lookup) | route controller.go:160,307-312 | **Dropped** (note 6) |
| **OAuthClient** | oauth.openshift.io/v1 | console | — | `.secret`; `.redirectURIs`; `.accessTokenInactivityTimeoutSeconds` | ConsoleOperator, OAuthClients | sync_v400.go:448-453; oauthclients.go:232-262 | **Guest** |
| **OLMConfig** | operators.coreos.com/v1 | cluster | — | `.spec.features.disableCopiedCSVs` | ConsoleOperator (dynamic lookup) | operator.go:210-230; sync_v400.go:950-961 | **Guest** |
| **StorageVersionMigration** | migration.k8s.io/v1alpha1 | console-plugin-storage-version-migration | — | `.status.conditions` | StorageVersionMigration | storageversionmigration controller.go:92-204 | **Guest** |
| **CustomResourceDefinition** | apiextensions.k8s.io/v1 | consoleplugins.console.openshift.io | — | `.status.storedVersions` | StorageVersionMigration | storageversionmigration controller.go:127-156 | **Guest** |

**Notes:**

1. **Proxy** — the operator reads the **guest** `Proxy.status.*` today and applies it as the operand pod's egress env. In dual-cluster mode the operand runs on the **management cluster**, so it should arguably use the **MC** proxy config (for the pod's own egress to the internet), not the guest proxy. The guest proxy config is irrelevant to the management-side pod's egress. **Recommendation:** dual-cluster mode sets pod proxy env **explicitly** (konnectivity socks5 for guest-service traffic; MC proxy for internet egress), not by copying `Proxy.status.*`. See `../operator-migration.md` "Blast-radius concern."

2. **Route** — CPO owns Routes in the dual-cluster topology (public/private variants, hostname from APIServer, ExternalName Service for Private), **including any custom-hostname Route** (§C). The operator **drops** Route controllers (`console`/`downloads` Route reconcile is gated off) and creates Service/Deployment/PDB only. See §B.

3. **oauth-serving-cert / default-ingress-cert** — only needed on the **integrated OAuth** path (non-OIDC). OIDC uses the OIDC issuer's CA, not the oauth-serving-cert. **Deferrable** for OIDC-first deployments (core console + plugins do not need it). Cross-cluster sync (guest `openshift-config-managed` → MC operand ns) is **not** implemented initially; stub or omit.

4. **Node** — must read **guest** nodes (the worker fleet), not management nodes. The node list feeds `console-config` `nodeArchitectures`/`nodeOperatingSystems` (UI metadata for CLI download page arch/OS options and Lightspeed gate). Reading MC nodes would advertise the wrong architecture. Tolerate empty node list at early reconcile (warns, disables Lightspeed, default downloads).

5. **ServiceAccount** — the operand pods' **management-cluster** ServiceAccount (for pod identity on the MC). **Distinct** from the **guest** SA the bridge authenticates **as** when proxying to guest KAS (that guest SA is CVO-created, name=`console`, ns=`openshift-console`; token is minted by CPO token-minter sidecar, not by the operator). See `../operator-migration.md` "Guest kube-apiserver authentication."

6. **IngressController** — read only to resolve the default ingress certificate when a custom route reuses the default hostname. There is no guest `IngressController` in this topology, so the lookup has no meaning; the custom certificate comes from the `openshift-config` Secret directly (§C).

---

## Table 2 — Resources WRITTEN / ACTED-ON by the operator

| Kind | Name | Namespace | Action / fields | Purpose | Written by | File:line (approx) | Target cluster |
|---|---|---|---|---|---|---|---|
| **ConfigMap** | console-config | openshift-console | apply `.data["console-config.yaml"]` | bridge config (auth, clusterInfo, customization, plugins, proxy, monitoring, telemetry, servingInfo) | ConsoleOperator | sync_v400.go:492-530; configmap.go:36-147 | **MC** (operand ns) |
| **ConfigMap** | service-ca | openshift-console | create; inject-cabundle annotation | bridge→Prometheus/plugin proxy CA | ConsoleOperator | sync_v400.go:592-635 | **MC** (operand ns) |
| **ConfigMap** | trusted-ca-bundle | openshift-console | create; inject-trusted-cabundle label | egress/proxy trust | ConsoleOperator | sync_v400.go:637-679 | **MC** (operand ns) |
| **ConfigMap** | console-public | openshift-config-managed | apply `.data["consoleURL"]` | publish console URL | ConsoleOperator | sync_v400.go:316-326 | **Guest** |
| **ConfigMap** | oauth-serving-cert, default-ingress-cert | openshift-console | resourceSyncer copy (from guest -config-managed) | OAuth/ingress CAs into operand ns | ResourceSyncController | starter.go:739-758 | **MC** (dest, note 1) |
| **ConfigMap** | OIDC provider CA | openshift-console | create `.data.ca-bundle.crt` | OIDC issuer CA for bridge mount | OIDCSetup | oidcsetup.go:215-224 | **MC** (operand ns) |
| **Secret** | console-oauth-config | openshift-console | apply `.data.clientSecret` | OAuth/OIDC client secret for bridge | OAuthClientSecret | oauthclientsecret.go:156-169 | **MC** (operand ns) |
| **Secret** | session-secret | openshift-console | apply session keys | bridge cookie session keys | ConsoleOperator | sync_v400.go:963-992 | **MC** (operand ns) |
| **Deployment** | console | openshift-console | apply full pod spec (image, flags, env, volumes, replicas, annotations) | run the bridge | ConsoleOperator | sync_v400.go:328-392; deployment.go:71-117 | **MC** (operand ns) |
| **Deployment** | downloads | openshift-console | apply image/replicas/affinity | run downloads server | DownloadsDeployment | downloadsdeployment controller.go:117-133 | **MC** (operand ns) |
| **Service** | console, downloads | openshift-console | apply (ClusterIP; **NodePort when ingressDisabled**) | expose pods (port 8443 for HCP router) | ServiceController | service controller.go:132-195 | **MC** (operand ns) |
| **Service** | console-redirect | openshift-console | apply/delete (custom hostname only) | redirect svc for custom route | ServiceController | service controller.go:143-170 | **MC** (operand ns) — only under upstream replace semantics, §C |
| **ServiceAccount** | console, downloads | openshift-console | apply | pod identity on MC | ServiceAccounts | serviceaccounts controller.go:129-179 | **MC** (operand ns) |
| **PodDisruptionBudget** | console, downloads | openshift-console | apply/delete | availability | PDB | poddisruptionbudget controller.go:99-121 | **MC** (operand ns) |
| **Route** | console, console-custom, downloads, downloads-custom, additional | openshift-console | apply/delete `.spec.host`, `.spec.tls` | expose via ingress | RouteController | route controller.go:208-406 | **Dropped** (note 2) |
| **OAuthClient** | console | — | update `.secret`+`.redirectURIs`; deregister on Removed | register redirect URIs with internal OAuth | OAuthClients | oauthclients.go:226-273 | **Guest** |
| **ConsoleCLIDownload** | oc-cli-downloads | — | create/update `.spec.links[]`; delete on Removed | oc download links | CLIDownloads | clidownloads controller.go:168-289 | **Guest** |
| **ConsoleNotification** | cluster-upgrade | — | create/delete | upgrade banner | UpgradeNotification | upgradenotification controller.go:101-160 | **Guest** |
| **Console** (config CR) | cluster | — | UpdateStatus `.status.consoleURL` | publish console URL | ConsoleOperator | sync_v400.go:296-314 | **Guest** |
| **Console** (operator CR) | cluster | — | patch `.status.conditions`, `.status.generations`, `.status.observedConfig` | operator status + observed TLS | all sync controllers, ConfigObserver | status/status.go:18-59; observe_config_controller.go:35-50 | **Guest** |
| **Authentication** | cluster | — | ApplyStatus `.status.oidcClients[console/cli]` | report OIDC client usage | OIDCSetup, CLIOIDCClientStatus | oidcsetup.go:151-174; clioidcclientstatus.go:114-136 | **Guest** |
| **ClusterOperator** | console | — | `.status.conditions`, `.status.versions`, `.status.relatedObjects` | CVO status | ClusterOperatorStatusController | starter.go:512-588 | **Guest** |
| **CustomResourceDefinition** | consoleplugins.console.openshift.io | — | patch `.status.storedVersions` | finalize migration | StorageVersionMigration | storageversionmigration controller.go:207-256 | **Guest** |
| **Deployment/Service/Secret** | console-conversion-webhook, webhook, webhook-serving-cert | openshift-console-operator | delete (one-time migration cleanup) | remove 4.16 webhook | MigrationCleanup | migration/cleanup_controller.go:75-122 | **MC** (operator ns) |
| **ConfigMap/Secret/Deployment** | console-config, service-ca, console-oauth-config, console | openshift-console | delete on Removed (management-state) | tear down console | ConsoleOperator.removeConsole | operator.go:397-419 | **MC** (operand ns) |

**Notes:**

1. **resourceSyncer cross-cluster** — copies ConfigMaps from **guest** `openshift-config-managed` to **MC** operand namespace. library-go's `ResourceSyncController` is single-cluster, so dual-cluster needs a hand-rolled cross-cluster copy or direct fetch. **Deferrable** for OIDC-first (oauth-serving-cert only needed on integrated OAuth path; default-ingress-cert superseded). See `../operator-migration.md` "The thorniest pieces."

2. **Route dropped** — CPO owns Routes (see Table 1 note 2). Operator Route controllers are gated off via `--unmanaged-resources` flag (default `console/Route,downloads/Route` in dual mode). Operator creates Service on port 8443 (HCP router backend); CPO creates the passthrough Route with HCP label + APIServer-derived hostname, and likewise for any custom hostname (§C).

---

## §B. Traffic-routing / exposure classification

Each resource is classified as **EXPOSURE** (Routes/Services/hostnames/ingress/TLS-termination/health-probing), **CONTENT** (plugins/branding/auth/telemetry/the bridge itself), or **STATUS/LIFECYCLE**.

**EXPOSURE resources** (candidates to drop/gate when HyperShift router owns exposure):

- RouteController (console/downloads) — **gate off** (CPO-owned Routes)
- console-redirect Service — **conditional** (only if a custom hostname *replaces* `console.<domain>` rather than being added alongside it, §C)
- HealthCheckController — **drop** (probes external route URL; meaningless when router owns URL)
- Ingress `spec.componentRoutes` — **keep reading, stop acting on**. It is the configuration surface for the custom hostname (§C); the operator consumes the hostname and the certificate reference but no longer builds a Route from them. `.spec.domain` and the guest ingress wildcard — **gate off**
- operator `spec.route.hostname`/`spec.route.secret` — **drop** (legacy custom console route + TLS, deprecated)
- Custom-route cert handling — **re-target, not drop**. The `openshift-config` TLS Secret is still the input; its destination changes from `Route.spec.tls` to the console pod, because the HyperShift router terminates nothing (§C)
- `default-ingress-cert` sync + read — **drop** (only feeds health-check client CA pool; HealthCheck dropped)

**Special cases:**

- **Proxy** (`status.*Proxy` → pod env `HTTP(S)_PROXY`/`NO_PROXY`) — **CONTENT** (pod egress config), but the guest `Proxy` CR is **wrong** for a management-cluster pod. **Recommendation:** dual-cluster mode sets pod proxy env **explicitly** (konnectivity socks5 for guest-service traffic; MC proxy for internet egress), not by copying guest `Proxy.status.*`. See `deployment.go:502` `setEnvironmentVariables`.
- **trusted-ca-bundle** inject+mount — **CONTENT** (egress trust). Keep; re-source for the MC pod.
- **oauth-serving-cert** sync/read — **CONTENT** (auth CA). On OIDC: **drop** (superseded by `authServerCA`). Keep only if integrated OAuth returns.
- **service-ca** ConfigMap — **CONTENT** (bridge→Prometheus/plugin proxy CA). Keep.

**CONTENT/STATUS resources (keep):**

- console-config, Deployment, session+oauth secrets — **keep** (core deliverable)
- OAuthClient `console` redirect URIs — **keep** (integrated OAuth path); feed correct external console URL
- plugins, CSP, proxy-services, i18n, catalog, quickstarts, perspectives, capabilities, telemetry, branding — **keep**
- ClusterOperator status, operator-CR conditions/versions — **keep** (CO semantics may need HyperShift adaptation)
- StorageVersionMigration, MigrationCleanup, staleConditions, logLevel, managementState, ConfigObserver — **keep**
- UpgradeNotification, CLIOIDCClientStatus, OIDCSetup status writes — **keep**

**Answer:** Yes, drop/gate the guest-side objects that *perform* routing when the HyperShift router owns exposure. The existing `ingressDisabled` external-control-plane path (`starter.go:250-257`, `util.go:88-100`) already does this (drops Route+HealthCheck, switches Service to NodePort, makes URL-consumers fall back to `spec.ingress.consoleURL`). Extend that pattern for dual-cluster mode.

The distinction that matters for custom hostnames: guest-side objects that *declare intent* — the hostname and its certificate reference — must keep being read. Only the objects that act on that intent move to the control plane.

---

## §C. Custom console hostnames

A customer-supplied console hostname is a **near-term requirement**, not an
optional extra. This section records what it implies for the operator.

**Upstream mechanics.** The operator builds a separate Route per component-route
with its own `ServingCertKeyPairSecret`
(`GetAdditionalComponentRouteSpecs`/`GetAdditionalRouteHostnames`,
`route.go:365-407`) and inlines the certificate into `Route.spec.tls`
(`route.go:347-352`). `console-config` gets the hostname list
(`additionalConsoleBaseAddresses`, `config_builder.go:409-413`). The OpenShift
ingress router terminates the customer's TLS; the pod never sees the
certificate.

**Why that inverts here.** The HyperShift router is `mode tcp` with SNI
passthrough and terminates nothing, so there is no edge to attach a certificate
to. TLS for every accepted hostname is terminated **at the bridge pod on
:8443**. The certificate therefore has to travel in the opposite direction from
upstream — out of guest `openshift-config` and into the HCP namespace — and the
pod has to present it only for the custom hostname, since it is simultaneously
serving `console.<domain>` under the platform wildcard.

**What the operator owns.**

- **Read** the hostname and the certificate reference from guest
  `Ingress.spec.componentRoutes[]`, as upstream. This is the configuration
  surface; it does not change.
- **Do not build a Route.** CPO owns the control-plane Route that gives the
  router its `req_ssl_sni` ACL for the custom hostname (Table 1 note 2).
- **Carry the certificate guest → management.** No such sync direction exists
  today; the OIDC client secret needs the same one (`../open-questions.md` §1),
  so it is worth building once.
- **Feed the hostname to the bridge** via `-additional-base-addresses`, which
  covers CSRF origins and per-request OAuth `redirect_uri` rewriting. That part
  already works.

**The unresolved piece** is how the pod selects between two certificates. The
bridge takes a single `-tls-cert-file` and its `GetCertificate` hook discards
the `*tls.ClientHelloInfo`, so it cannot do SNI selection today. Either that
becomes an upstream change, or the operator runs a second console Deployment per
custom hostname with its own certificate and `-base-address` — which needs no
upstream change and is the cheaper option to evaluate first.

**Also affected.** `console-redirect` Service and the bridge's `-redirect-port`
implement upstream's *replace* semantics, where the default hostname 301s to the
custom one. Whether `console.<domain>` should instead keep serving alongside is
an open product question (`../open-questions.md` §3); the answer decides whether
those two stay. The additional hostnames also feed OAuthClient redirect URIs
(`oauthclients.go:190`), which matters only on the integrated-OAuth path.

**Requirement-level analysis, including the guest-side alternative:**
[`../open-questions.md`](../open-questions.md) §3.

---

## Cross-references

- Dual-cluster implementation tasks: `../CONSOLE_CONTROL_PLANE_PHASE4_OPERATOR_TASKS.md` (file-by-file edit map, not in this worktree)
- Decision + why: `../operator-migration.md`
- Dual-cluster precedent: `cno-dual-cluster-precedent.md` (this directory)
