# Open Questions and Risks

Ordered by severity, most blocking first.

## 1. GCP OIDC console client model

**Status:** Downgraded from blocker to a design with a shipped precedent. One
product decision remains, and full zero-touch provisioning is still out of
reach. See [The day-2 client model](#the-day-2-client-model) below, which
supersedes the "leading candidates" further down this section.

**And it may be moot.** The whole section presumes external OIDC. If GCP HCP
adopts the internal OpenShift OAuth server for multiple/custom identity
providers, the console delegates to `openshift-oauth` and needs no Google client
at all — see
[the integrated OAuth option](#the-option-that-removes-the-question-entirely-integrated-openshift-oauth).

**The problem:** The console bridge is a confidential web application needing its own Google OAuth "Web application" client with a registered redirect URI. Google web-client creation is **not automatable** (no API, no gcloud, no Terraform) and Google **forbids wildcard redirect URIs**, so provisioning a per-hosted-cluster console client without a manual Google step is unresolved. Cluster creation must not require a manual Google step.

**Critical detail:** This is **topology-independent**. It would equally affect a data-plane console under external OIDC, so it is not an argument against moving the console. The control-plane-side port surfaced a pre-existing gap that would block any GCP HCP console under external OIDC.

**What works today:** OIDC base mechanics are proven live. HCCO owns the guest `Authentication` CR and already delivers any `oidcClients[]` entry plus client secret to the guest cluster. Bridge login is spec-driven. The shared Google client works as a **KAS audience** (token verification), but never as the bridge's own client.

**Why the bridge needs its own client:** The console bridge performs a confidential OAuth code-to-token exchange with a real `ClientSecret` and needs a registered web redirect URI (`console.<domain>/auth/callback`). The shared Google client is public/installed type with loopback/OOB redirects only, structurally incompatible with a web application flow.

**Hard Google constraints (verified September 2026):**

- No API/gcloud/Terraform to create a Web OAuth client (Cloud Console UI only)
- No wildcard redirect URIs (exact HTTPS match required, Console UI only to add/edit)
- Client secret shown once at creation

**Design constraint:** Any solution requiring Google to be changed *during* cluster creation is eliminated. Note the scope — this rules out the creation flow, not Google ever holding a per-cluster redirect URI. See below.

### The day-2 client model

The framing above treats "Google must not see a per-cluster URL" as an absolute
design constraint. It is not. It only holds if the Google client must be fully
provisioned *before* the cluster exists.

**A pre-existing Google client dissolves the circularity.** Redirect URIs are a
mutable property of a Google OAuth client, and the client can be created before
the cluster. So the client ID is known at cluster-creation time, and the only
thing that depends on the cluster hostname — the redirect URI — is added later,
directly on the client, never touching the HostedCluster.

**Day 0 — set once by the platform at creation, in `HostedCluster.spec`:**

| Field | Value |
|---|---|
| `oidcProviders[].issuer.audiences` | += console client ID |
| `oidcProviders[].oidcClients[]` | console entry: same client ID, `clientSecret` name reference |
| the referenced Secret | exists, empty, annotated `hypershift.openshift.io/hosted-cluster-sourced` |

**Day 2 — customer, entirely outside the HostedCluster:**

1. Add `https://console.<domain>/auth/callback` to the existing Google client.
2. Create the real secret in the guest cluster's `openshift-config` namespace.

No spec mutation after creation, and therefore no new day-2 platform API.

#### Why both fields are required

`issuer.audiences` and `oidcClients[].clientID` are independent. Nothing derives
one from the other — confirmed in HyperShift
(`control-plane-operator/controllers/hostedcontrolplane/kas/auth.go`, and the v2
equivalent) and in standalone OpenShift
(cluster-authentication-operator, `pkg/controllers/externaloidc/generation/kubeapiserver/generate.go`),
which contain the identical loop over `issuer.Audiences` and never read client
IDs. There is no shared library-go helper; two independent implementations that
agree. The upstream API doc is explicit: *"at least one of the entries must
match the 'aud' claim in the JWT token."*

The official OpenShift external-auth documentation shows the same, listing each
client ID in both places:

```yaml
issuer:
  audiences: [console-test, oc-cli-test]
oidcClients:
- clientID: oc-cli-test
  componentName: cli
- clientID: console-test
  clientSecret: { name: console-secret }
  componentName: console
```

Note `clientSecret` appears only on the console entry — the CLI client is
public, the console client confidential.

#### Precedent: ARO HCP does exactly this

ARO HCP's `externalAuths` resource carries the same client ID in both
positions:

```bicep
issuer: { url: issuerURL, audiences: [ clientID ] }
clients: [
  { clientId: clientID, component: { name: 'console', authClientNamespace: 'openshift-console' }, type: 'Confidential' }
  { clientId: clientID, component: { name: 'cli',     authClientNamespace: 'openshift-console' }, type: 'Public' }
]
```

ARO also registers an **exact per-cluster redirect URI**, read off the created
cluster, rather than a wildcard — even though Entra permits wildcards for
org-only tenants. So the precedent transfers to Google despite Google's absolute
prohibition: ARO never relied on wildcards either.

The customer supplies the console client secret by hand, guest-side, into
`openshift-config` under the name `<external_auth_name>-console-openshift-console`.

#### Code changes required

1. **Widen the `hosted-cluster-sourced` gate.** Today it is
   `azureutil.IsAroHCPByHCP(hcp)` — Azure with managed identities — in
   `control-plane-operator/hostedclusterconfigoperator/controllers/resources/resources.go`.
   Still gated the same way on current upstream `main`. Pitch the widening as
   wanting the **guest-sourced** property, not the **isolation** property: the
   annotation's doc comment justifies itself with "sensitive data that can't
   live on the control-plane", and in our topology the secret *must* reach the
   control plane for the bridge to mount it. Our reason is access — a customer
   can write to their own guest cluster but has no path to write a Secret into
   the HostedCluster namespace on the management cluster. This may warrant a
   distinct mode rather than reusing the ARO annotation verbatim.

2. **Sync guest `openshift-config` → HCP namespace.** Does not exist; the
   existing flow runs the other way (HCP namespace → guest). This is work for
   the ported console-operator — see [operator-migration.md](operator-migration.md).

3. **Stop the guest `Authentication` mirror from failing.** HCCO blindly copies
   `hcp.Spec.Configuration.Authentication` into the guest `Authentication`
   (`support/globalconfig/authentication.go`), and that write is rejected by the
   CEL rule while nothing writes guest `status.oidcClients`. Either the ported
   operator writes that status, or the console `oidcClients` entry is pruned
   from the guest mirror when console placement is control-plane-side — the
   guest has no console, so the entry is meaningless there. This pairs naturally
   with the CVO payload strip.

#### What this does not solve

**Google still has no API for creating OAuth web clients.** Day 2 changes
*when* the registration happens, not *who* does it. It converts an impossible
ordering constraint into a documented customer runbook step.

This demotes the **state-based redirect broker** from "the way to unblock this"
to "the way to make it zero-touch". The broker is still the only option that
removes the manual Google step entirely — one fleet-fixed redirect URI with the
target cluster encoded in the OAuth `state` parameter, working for private
clusters because Google never dials the redirect URI, it 302s the private-side
browser. It still needs upstream bridge support and new fleet auth
infrastructure. It is now an optimisation, not a prerequisite.

An **intermediate IdP with dynamic client registration** fronting Google remains
the other zero-touch option.

#### The option that removes the question entirely: integrated OpenShift OAuth

Everything above assumes the console authenticates the user directly against an
external OIDC provider, so it needs its own confidential client at that
provider. That assumption is not fixed.

There are parallel discussions about adopting the **internal OpenShift OAuth
server** to support multiple and custom identity providers. Under that model the
console does not talk to Google at all — it delegates to `openshift-oauth`,
which is the console's classic authentication path, and the Google client
problem simply does not arise. No per-cluster Google client, no Google
redirect-URI registration, no Google client secret to deliver, and none of the
three code changes listed above are needed for auth purposes. The integrated
path has its own client — the cluster-local `OAuthClient` named `console` and
its operator-generated secret — but both are created and rotated automatically
in-cluster, with no external provider involved.

This pairs unusually well with the control-plane-side console, because in
HyperShift the OAuth server **already runs control-plane-side** and is already
exposed at `oauth.<domain>` on the same router and the same wildcard
certificate. Console and OAuth server would then both sit on our side of the
boundary, with no guest dependency in the login path at all.

This is why the spike's standing recommendation is to keep the bridge's auth
**pluggable** rather than hard-wiring the OIDC flow. Which model wins is a
broader identity decision for GCP HCP, well outside this spike — but it is the
single largest fork in the road for this section, and the day-2 design above
should be understood as the answer *conditional on external OIDC remaining the
model*.

#### Open product decision

If external OIDC does remain the model, the day-2 design means the customer
brings their own Google OAuth client — their Google project, their consent
screen, their redirect URI, their secret. That is a product positioning
decision, not a technical one, and it should be made explicitly rather than
inherited from the implementation.

This is a question specifically about the *console* client, and it does not
necessarily follow what GCP HCP does for kube API access.
[implementation-plans/gcp-customer-authentication.md](../../implementation-plans/gcp-customer-authentication.md)
is largely concerned with kube API and CLI access, where the CLI provisions an
OAuth client during infrastructure setup and passes `oauthClientId` into the
cluster spec. The console can legitimately differ — and for the console client
it has to, because Google will not let us mint a web-application client
programmatically.

#### How this lands in GCP HCP specifically

- **Day-0 is the only safe option anyway.** gecko controllers continuously
  reconcile the HostedCluster spec, so an out-of-band edit to
  `spec.configuration` would be reverted. A day-2 spec change would have needed
  new public-API surface plus a field-mutability decision, neither of which is
  documented today. Setting both fields at creation avoids the question.
- **Routing the secret through the guest is a feature, not a workaround.**
  [design-decisions/identity/secret-management-strategy.md](../../design-decisions/identity/secret-management-strategy.md)
  explicitly defers customer-facing secrets to a future decision, and there is
  no documented path for a customer to supply a secret through the platform API.
  Guest-side delivery means the platform API never holds the value.

**Detailed analysis:** See `reference/console-auth-options.md` section 7.

**Open work:**

- Confirm the product decision that the customer brings their own Google OAuth client.
- Widen the `hosted-cluster-sourced` gate to GCP, or add a distinct guest-sourced mode.
- Build the guest → HCP namespace secret sync in the ported console-operator.
- Resolve the guest `Authentication` mirror rejection (operator writes status, or prune the entry).
- Optionally, pursue the redirect broker for zero-touch provisioning.
- Confirm `ConsolePublicURL` → KAS (`kas/params.go:77`, `kas/config.go:151`) for the `oc-oidc` login command when the operator runs control-plane-side.

## 2. Data-plane-driven redeployment of a control-plane workload

**Status:** Open design concern raised by Cesar Wong during PR review.

**The concern:** If guest-side ConfigMaps or custom resources drive a control-plane console redeployment (trigger a new deployment rollout), a customer can cheaply DoS control-plane capacity by churning data-plane objects. This violates the control-plane isolation boundary.

**Cesar's alternative:** Have the console process read what it needs from the data plane in-process rather than triggering redeployments. This would reduce the blast radius of customer-driven changes.

**Patrick's response:**

- Several console inputs are custom resources (ConsolePlugin, ConsoleNotification, ConsoleCLIDownload, operator.openshift.io/Console), not just ConfigMaps, so some operator handling of guest-side resources remains necessary.
- A trimmed-down set of customer-configurable console settings that trigger redeployment would be acceptable. The operator could distinguish between changes that require a restart (e.g., authentication config, operand namespace) versus those that can be read dynamically (e.g., branding assets, plugin list).
- Monitoring and rate-limiting guest-driven reconciliations is standard practice in other HyperShift operators (CNO, ingress-operator).

**Cross-reference:** See `operator-migration.md` for the full operator design.

**Open work:**

- Enumerate which guest-side inputs should trigger redeployment versus be read in-process.
- Design rate-limiting or change-buffering for guest-driven reconciliations if needed.
- Document the threat model and mitigations in the operator design.

## 3. Custom domain and certificate for the console

**Status:** Raised by Claudio Busse; since flagged as a near-term product
requirement. **Not tested in this spike** — everything below is desk analysis
from source.

### How OpenShift does it today

`Ingress.spec.componentRoutes[]` carries a `hostname` and an optional
`servingCertKeyPairSecret`. console-operator matches `openshift-console/console`
and `openshift-console/downloads` (`route.go:60-67`); the certificate is a
`kubernetes.io/tls` Secret the admin creates in `openshift-config`.
`Console.spec.route` is the deprecated predecessor — componentRoutes wins.

**The certificate never reaches the console pod.** console-operator inlines it
into `Route.spec.tls` (`route.go:347-352`) on a Route declaring
`termination: reencrypt`, so the OpenShift ingress router terminates it and the
pod keeps serving its own service-ca certificate.

Changing the hostname also updates the `console` OAuthClient, the
`console-config` ConfigMap, `Console.status.consoleURL` and the `console-public`
ConfigMap. The old hostname becomes a 301 redirector, so upstream a custom
domain *replaces* the console URL rather than adding one.

### Two possible traffic paths

DNS and certificate both follow from the traffic path, so that is the decision.

| | Direct to the control-plane router | Via a guest-side ingress |
|---|---|---|
| **Traffic** | Browser → public LB or PSC endpoint → HCP router → console pod, as `console.<domain>` does today | Browser → customer's IngressController → console over PSC |
| **DNS** | Customer CNAMEs the custom name to `console.<domain>` | Customer points the custom name at their own ingress |
| **Certificate** | Must reach the console pod — the gap | Terminated guest-side, never leaves the guest cluster |

From the browser both are one TLS connection on port 443. Neither needs changes
to the public LB, the ILB, the PSC service attachment or the firewall.

**Direct path — three gaps.**

1. **A Route in the HCP namespace for the custom host.** The browser sends SNI
   equal to that hostname, so the router needs a matching `req_ssl_sni` ACL,
   which it derives from Routes CPO already reconciles.
2. **The certificate on the pod.** It has to cross guest → management — the
   second consumer of a sync direction that does not exist today, after the OIDC
   client secret in §1 — and the pod must present it *only* for the custom
   hostname, since it also serves `console.<domain>` under the platform
   wildcard. That needs either SNI certificate selection in the bridge (an
   upstream change: `GetCertificate` is already the right shape but ignores its
   argument, `main.go:816-826`) or a second console Deployment per custom
   domain, which needs no upstream change.
3. **A CNAME from the customer.** The platform publishes nothing new.

**Guest-side path.** The customer runs their own IngressController and a
reencrypt Route with their own certificate, targeting the existing
`console.<domain>` — so nothing new is needed control-plane-side and the private
key never leaves the guest cluster. The obvious implementation does not work:
`ExternalName` Services rely on FQDN-typed EndpointSlices, which the router
rejects since the fix for
[CVE-2026-42965](https://access.redhat.com/security/cve/cve-2026-42965), and the
reencrypt backend sends no SNI (no `sni` keyword on the `server` line in
`openshift/router`'s `haproxy-config.template`), so the HCP router would fall
through to `default_backend kube_api`. It reduces to a purpose-built guest-side
proxy that dials `console.<domain>` with SNI set explicitly.

**Common to both.** `-additional-base-addresses` must cover the custom host, and
under external OIDC `https://<custom>/auth/callback` must be registered on the
Google client — another manual Google step, interacting with §1. The bridge
already rewrites the OAuth `redirect_uri` from the request `Host` for allowed
hosts (`auth.go:284-297`).

### Open questions

- **Which traffic path.** Whether the customer's private key crosses into the
  management cluster is as much a trust question as a technical one.
- **Configuration surface.** Guest `Ingress.spec.componentRoutes`, keeping the
  upstream API but needing guest API access, or a HostedCluster field?
- **Replace or add?** Upstream replaces the console URL. Keeping
  `console.<domain>` serving alongside is a deliberate divergence.
- **Certificate expiry** becomes a platform-visible outage on the direct path.
  The bridge reloads the key pair per handshake (`main.go:818-825`), so a secret
  refresh needs no pod restart, but monitoring and a degraded condition would
  still be needed.
- **Private clusters.** The custom name must resolve to the PSC endpoint inside
  the customer VPC, so custom DNS and private exposure have to be designed
  together.

## 4. Multi-tenant console

**Status:** Out of scope for the initial implementation; recorded for future consideration.

**Question raised by:** Bill Montgomery during design review.

**The question:** Given the work to move the console to the management cluster, should it also be made multi-tenant (one console instance serving multiple hosted clusters)?

**Argument against conflating the efforts:**

- **Scope difference:** The single-tenant control-plane-side console is an update to one existing component following existing HyperShift deployment patterns. The main review burden falls on the console team. It is a straightforward port.
- **Multi-tenant console is a new component:** It would require ground-up redesign of authentication, authorization, and RBAC. How do you decide which clusters a user may log into? How do you present the cluster-selection UI? How do you handle per-cluster guest-side plugins and configurations?
- **Shared auth assumption does not hold:** A multi-tenant console assumes every hosted cluster shares an auth backend. This is already unsafe to assume, since each HCP may configure its own IdP (different Google org, different Entra ID tenant, different Keycloak instance, or even IntegratedOAuth with distinct identity providers).
- **Multi-team, multi-month effort:** A production multi-tenant console would require collaboration across the console team, security, HyperShift, and managed OpenShift. Estimated as several months of work with significant design review overhead.

**Recommendation:** Treat the single-tenant control-plane-side port and multi-tenant redesign as separate initiatives. Ship the single-tenant version first to unblock zero-node clusters and control-plane isolation. Evaluate multi-tenancy separately if demand and shared-auth feasibility are proven.

**Cross-reference:** See `findings.md` for the single-tenant design.

## 5. Certificate model — resolved, recorded for clarity

**Status:** Not a conflict. Recorded because the surrounding documentation can
read as one.

The repository design decision
[networking/customer-dns-zone-management.md](../../design-decisions/networking/customer-dns-zone-management.md)
records the hosted-cluster API certificate as self-signed and uses that to
conclude a DNS zone is not required for certificate issuance. The PoC ran
against a cert-manager wildcard. Both are correct and describe different things:

- **Self-signed is the HyperShift default.** That is what the design decision
  describes.
- **GCP HCP overrides it.** The deployment supplies a Let's Encrypt-signed
  wildcard `*.<domain>` via a cert-manager `ClusterIssuer` (`public-issuer`),
  wired into the hosted cluster through
  `spec.configuration.apiServer.servingCerts.namedCertificates`. See
  [manifests/hostedcluster/certificate.yaml](manifests/hostedcluster/certificate.yaml).

That wildcard already covers `api.<domain>` and `oauth.<domain>`, and now covers
`console.<domain>` and `downloads.<domain>` at no additional cost, purely because
the console exposure reuses the same hostname pattern rather than inventing one
under the guest ingress domain.

So the "simpler certificate management" benefit claimed in
[findings.md](findings.md) holds, and holds for a concrete reason: the console
inherits a publicly trusted certificate whose lifecycle is already owned by the
management cluster, with no guest-side certificate configuration, no ACME
challenge delegation, and no additional DNS zone.

Worth noting for a browser-facing component specifically: a self-signed API
certificate is workable for `oc` with a supplied CA bundle, but would not be
acceptable for a console a customer opens in a browser. The Let's Encrypt
wildcard is what makes the control-plane-side console viable as a user-facing
endpoint, so it is a dependency of this design rather than an incidental detail.

## 6. CVO console removal specifics

**Status:** Mechanism verified; specifics open.

**Context:** The two-switch model (Console capability plus a new GCP-keyed placement flag) is decided. The mechanism (`preparePayloadScript` in `cvo/deployment.go`) is verified and has precedent. What remains open:

1. **Exact payload filenames to strip versus keep.** STRIP the workload (operator Deployment `07-operator*.yaml`, `05-service.yaml`, `05-config.yaml`, servicemonitor, prometheusrbac). KEEP the guest RBAC/SA/CR/namespace (`03-rbac-*`, `04-rbac-*`, `06-sa.yaml`, `02-namespace.yaml`, `01-operator-config.yaml`) so CVO applies them as-is. Real release-image filenames need confirmation (dev-repo names are re-prefixed `0000_50_console-operator_*`).

2. **Migration cleanup on an off→on flip.** A cluster changing from guest console to control-plane console must delete the already-applied guest console-operator via `resourcesToRemove` / `0000_01_cleanup.yaml`. Enumerate the exact objects to delete.

3. **Who publishes the guest ClusterOperator status.** The Console capability stays enabled, so something must still write `clusteroperator/console` status and `console.config.openshift.io/cluster .status.consoleURL`. Confirm the control-plane-side operator owns these writes when placement=on, and that they target the **guest** API.

4. **Where the placement flag lives.** Should this be a HostedCluster spec field, a platform-derived value, or an annotation? How does it reach `preparePayloadScript`?

**Cross-reference:** See `findings.md` section on CVO payload manipulation.

## 7. Hostname migration

**Status:** User-visible change; impact needs confirmation.

**The change:** Moving from `console-openshift-console.apps.<basedomain>` (guest ingress domain) to `console.<domain>` (control-plane domain, alongside `api.<domain>`).

**Impact:**

- User bookmarks break and need updating
- Documentation and tooling that hardcode the `.apps.` console hostname need updates
- The `console.config.openshift.io/cluster .status.consoleURL` field changes
- Any scripts or automation that parse the console URL need adjustment

**Mitigating factors:**

- The `consoleURL` is a status field, not spec, so clients should treat it as dynamic
- Login redirects are driven by the `Console` CR, which the operator would update

**Open work:**

- Confirm acceptability to managed OpenShift / GCP HCP customers
- Document the migration in release notes
- Identify and update any first-party tooling that embeds the console hostname

## 8. Plugin and monitoring traffic at scale

**Status:** Framework validated; scale testing needed.

**The concern:** Every plugin asset request (JavaScript bundles, i18n files) and every Thanos/Alertmanager query traverses the konnectivity socks5 tunnel. The spike validated that the path works functionally (monitoring plugin loaded successfully, queries returned data), but it has not been load-tested.

**Questions:**

- What is the latency impact of tunneling high-frequency asset requests?
- What is the throughput limit of the konnectivity tunnel for plugin traffic?
- Do we need request caching or a CDN-like layer for plugin assets?
- Should plugin assets be served from the control-plane side instead of proxied?

**Precedent:** The oauth-openshift server already uses konnectivity socks5 for reaching guest services, and OLM uses it for catalogd. However, console plugin assets may have different traffic patterns (larger payloads, more frequent requests).

**Cross-reference:** See `cost-analysis.md` for traffic volume estimates.

## 9. Resolved during the spike

The following items looked like risks during initial planning but are now **closed** with proven solutions:

| Concern | Resolution |
|---------|-----------|
| **Guest KAS authentication for the bridge** | Token-minter sidecar (standard HCP pattern), framework-rotated. Verified live. |
| **Guest KAS authentication for the operator** | `InjectServiceAccountKubeConfig` (standard HCP pattern), framework-rotated. Verified live. |
| **Konnectivity mode selection** | HTTPS for the operator (KAS-only, precedent: ingress-operator). Socks5 for the bridge (arbitrary guest services, precedent: oauth-openshift). `--resolve-from-guest-cluster-dns` turned out not to be needed because plain Services resolve via ClusterIP lookup against the guest KAS. |
| **RBAC (guest side)** | Guest RBAC arrives via CVO upstream manifests unmodified (ServiceAccount, Role, RoleBinding, ClusterRole, ClusterRoleBinding for the console-operator and console ServiceAccounts). No HyperShift-specific changes needed. |
| **RBAC (management side)** | Management-cluster RBAC ships in CPO assets following the CNO pattern (assets/console-operator/role.yaml, rolebinding.yaml). Framework rewrites the namespace. |
| **Restricted PSA for the console pod** | Upstream console Deployment manifest already complies with restricted PSA (runAsNonRoot, seccompProfile, capabilities.drop). No changes needed. |

## 10. Not console-specific: CNO restricted-PSA gap

The cluster-network-operator's self-managed network operands (`network-node-identity`, `ovnkube-control-plane`, `multus-admission-controller`, `cloud-network-config-controller`) violate restricted PSA on SCC-less management clusters (GKE), blocking node bring-up entirely.

**Summary:** CNO templates set pod-level `runAsUser`/`runAsNonRoot`/`seccompProfile` but never set container-level `allowPrivilegeEscalation: false` and `capabilities.drop: [ALL]` that restricted PSA also requires. Four CNO Deployments fail admission in the HCP namespace, `network-node-identity` never runs, node CSRs stay Pending forever, nodes never reach Ready.

**Workaround:** `hypershift.openshift.io/pod-security-admission-label-override: baseline` annotation on the HostedCluster. Relaxes the entire HCP namespace to baseline PSA, so CNO operands admit. This is a stopgap; it relaxes more than just CNO.

**Real fix:** In CNO bindata templates, set container-level `allowPrivilegeEscalation: false` and `capabilities.drop: [ALL]` (not just pod-level) when the HyperShift-managed-service code path is active. This is the CNO equivalent of the restricted-PSS work CPO already did for its own workloads (GCP-205).

**Not console-specific:** This blocks any zero-SCC HyperShift guest from bringing up nodes, regardless of the console spike. It surfaced during the spike because the PoC scaled a NodePool from 0 to 4 to observe real workloads.

**Tracking:** Needs a Jira and an upstream CNO PR. Neither has been filed.

**Provenance caveat:** the symptom chain above was observed directly — the
admission failures, the Pending CSRs, and the fact that the baseline PSA
override resolves them. The stated root cause, that the specific missing fields
are the container-level ones in CNO's bindata templates, is the diagnosis that
explains those symptoms and should be confirmed against CNO source before it is
quoted in a bug report.
