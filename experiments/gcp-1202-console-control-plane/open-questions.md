# Open Questions and Risks

Ordered by severity, most blocking first.

## 1. GCP OIDC console client model

**Status:** ⛔ **PRIMARY BLOCKER** for going beyond a spike.

**The problem:** The console bridge is a confidential web application needing its own Google OAuth "Web application" client with a registered redirect URI. Google web-client creation is **not automatable** (no API, no gcloud, no Terraform) and Google **forbids wildcard redirect URIs**, so provisioning a per-hosted-cluster console client without a manual Google step is unresolved. Cluster creation must not require a manual Google step.

**Critical detail:** This is **topology-independent**. It would equally affect a data-plane console under external OIDC, so it is not an argument against moving the console. The control-plane-side port surfaced a pre-existing gap that would block any GCP HCP console under external OIDC.

**What works today:** OIDC base mechanics are proven live. HCCO owns the guest `Authentication` CR and already delivers any `oidcClients[]` entry plus client secret to the guest cluster. Bridge login is spec-driven. The shared Google client works as a **KAS audience** (token verification), but never as the bridge's own client.

**Why the bridge needs its own client:** The console bridge performs a confidential OAuth code-to-token exchange with a real `ClientSecret` and needs a registered web redirect URI (`console.<domain>/auth/callback`). The shared Google client is public/installed type with loopback/OOB redirects only, structurally incompatible with a web application flow.

**Hard Google constraints (verified September 2026):**

- No API/gcloud/Terraform to create a Web OAuth client (Cloud Console UI only)
- No wildcard redirect URIs (exact HTTPS match required, Console UI only to add/edit)
- Client secret shown once at creation

**Design constraint:** Any solution where Google sees a per-cluster URL is eliminated.

**Leading candidates:**

1. **State-based redirect broker** (recommended): One fleet-fixed Google redirect URI with the target cluster encoded in the OAuth `state` parameter. Adding clusters requires zero Google changes. Works for **private clusters** too, since Google never dials the redirect URI; it 302s the private-side browser. A private broker with a private redirect URI is reachable end-to-end. Needs **upstream bridge support** (bridge cannot target a separate broker today) plus new fleet auth infrastructure. Secret isolation ties to OCPSTRAT-2173 (the day-2 `hosted-cluster-sourced` annotation pattern).

2. **Intermediate IdP with dynamic client registration** fronting Google. Larger architectural change but avoids the broker complexity.

**Detailed analysis:** See `reference/console-auth-options.md` section 7.

**Open work:**

- Decide the client model (realistically option 1 or 2). Requires product, platform, and security input.
- If option 1: design and build the broker (state signing/allowlist, per-connectivity-domain reachability for private clusters) and the upstream bridge change to target it.
- Extend day-2 `hosted-cluster-sourced` secret-isolation honoring to GCP (currently ARO-HCP only).
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

## 3. Custom DNS for the console

**Status:** Open question raised by Claudio Busse.

**Context:** ROSA recently shipped custom domains for the console. GCP HCP has no custom domain support for the API today, and the current proposal puts console and downloads records next to the API on the same wildcard certificate (`console.<domain>`, `downloads.<domain>` alongside `api.<domain>` and `oauth.<domain>`).

**The question:** Should GCP HCP support custom console domains? If so, how does it interact with the existing wildcard certificate model?

**Mitigating detail:** The console already supports multiple hostnames (the `--additional-base-addresses` flag exists for exactly this), so it may be easier than the API case. However, it still requires:

- Customer-side DNS configuration (CNAME or A records pointing to the console route)
- Customer-provided TLS certificate for the custom domain
- Wiring into the `Console.spec.route` CR field

**Open question:** Would customers still configure the console's custom domain via the `Console` object **from within the HCP kube-apiserver** (requiring HCP API access), or would this be a HyperShift-level configuration (annotation/spec on the HostedCluster)?

**Cross-reference:** See `architecture.md` for the DNS and certificate model.

**Open work:**

- Decide if custom console DNS is in scope for the initial implementation.
- If yes: design the configuration surface (HostedCluster spec vs guest Console CR).
- If yes: determine whether a separate certificate or an extended SAN list is used.

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

## 5. Certificate model conflicts with an existing design decision

**Status:** Unresolved contradiction. Must be settled before any of this informs
a design decision.

**The conflict.** This spike's entire certificate story rests on the console
reusing a cert-manager-issued wildcard `*.<domain>` — the same `external-api-cert`
secret the API server uses for its named certificate. That is what the PoC
deployed and what [architecture.md](architecture.md) §5 documents.

The repository's existing design decision says something different.
[design-decisions/networking/customer-dns-zone-management.md](../../design-decisions/networking/customer-dns-zone-management.md)
records the hosted-cluster API certificate as **self-signed, explicitly not
ACME-based**, and uses that fact to conclude that a DNS zone is not required for
certificate issuance.

Both statements cannot describe the same target state.

**Why this matters more than it looks.** "Certificate management no longer
depends on guest-side configuration" is one of the headline benefits claimed for
moving the console — see [findings.md](findings.md). That benefit is real only
if the API-adjacent certificate is publicly trusted, because a browser is the
client. A self-signed API certificate is fine for `oc` with a supplied CA
bundle; it is not fine for a console a customer administrator opens in Chrome.

So this is not a documentation tidy-up. It determines:

- whether the platform needs ACME infrastructure and therefore a public DNS zone
  per hosted cluster, which is precisely what the existing decision concluded it
  could avoid;
- who owns certificate lifecycle and renewal;
- whether the wildcard SAN list can be extended for custom console domains
  (see [§3](#3-custom-dns-for-the-console));
- whether the "simpler certificate management" benefit survives at all.

**Possible resolutions.** Either the existing design decision is stale and the
fleet already issues cert-manager wildcards for `api.<domain>` (in which case the
decision record needs updating and the benefit stands), or the PoC used a
non-production certificate arrangement (in which case the console needs its own
publicly-trusted certificate story, and the claimed simplification shrinks).

**Next step.** Confirm against the deployed fleet what actually serves
`api.<domain>` today, then correct whichever document is wrong. This is a
question of fact, not of design, and should be cheap to answer.

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

- The console already supports multiple hostnames via `--additional-base-addresses`
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
