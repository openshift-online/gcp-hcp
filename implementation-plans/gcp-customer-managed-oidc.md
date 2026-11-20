# Customer-Managed OIDC Authentication

> **Status**: Draft

**Jira**: TBD

| Field | Value |
|-------|-------|
| **Identifier** | `customer-managed-oidc` |
| **Components** | gecko `platform-api`, `controllers/hc`, transport feedback; `gcp-hcp-ctl`; management-cluster egress controls |
| **Depends on** | Supported HyperShift External OIDC configuration and day-two reconciliation; [customer bootstrap access](gcp-customer-bootstrap-access.md); public issuer discovery/JWKS reachability safeguards |
| **Related** | [Customer authentication background](gcp-customer-authentication.md), [Cedar public API authorization](../design-decisions/identity/cedar-public-api-authorization.md), [no direct cross-cluster connectivity](../design-decisions/networking/no-direct-cross-cluster-connectivity.md) |

## Overview

Allow customers to configure an external OIDC identity provider for **guest Kubernetes
API access**, both when creating a cluster and by replacing the provider afterward.
Examples include a compatible tenant-specific Okta, Entra ID, or Keycloak issuer.
Compatibility depends on actual discovery, token, claims, and public-client behavior;
this plan does not assert that every configuration of those products is supported.

The existing Google provider remains the default. Customer-managed OIDC is opt-in and
replaces that guest provider; it does not replace platform authentication or workload
identity. Customers use bootstrap credentials to create their initial guest RBAC grants
and an external exec plugin for subsequent OIDC login. Gecko makes no guest API calls
to configure RBAC or validate customer login.

### Agreed scope

- Support cluster creation, Google-to-customer migration, customer-to-customer provider
  replacement, mapping/client changes, and an explicit return to default Google.
- Use existing `cluster.create` and `cluster.update` authorization with object-aware
  Cedar conditions. Do **not** introduce a separate authentication-update permission.
- Initially support only public HTTPS issuers using publicly trusted certificates.
- Allow claim-name selection, default usernames to `sub`, and enforce safe issuer-scoped
  identity prefixes. Do not expose arbitrary customer mapping/validation expressions.
- Use a documented external OIDC exec plugin initially. No native CLI browser login,
  token refresh/cache implementation, or automatic plugin installation/configuration.
- Recovery is an explicit customer action through the platform API and bootstrap
  credential path. There is no automatic rollback or guaranteed dual-provider window.

### Identity boundaries

| Identity system | Authority and purpose | Effect of this feature |
|-----------------|-----------------------|------------------------|
| Platform authentication | Existing ESPv2/platform identity and Cedar roles for lifecycle/configuration operations | Unchanged; guest IdP tokens do not become platform credentials |
| Guest user authentication | Guest API server trusts the selected OIDC issuer/audiences and maps claims to users/groups | Customer-configurable, with one provider in the deployed schema |
| Guest authorization | Customer-owned RoleBindings/ClusterRoleBindings | Not created or migrated automatically by Gecko |
| Service-account issuer and GCP WIF | Workload tokens and GCP infrastructure access | Unchanged; Cluster `spec.issuerURL` is not the guest login IdP |

## Current State and Evidence

Source inspection and read-only validation in a development environment established:

- Gecko's `controllers/hc/manifest/manifests.go` hardcodes issuer
  `https://accounts.google.com`, audience `32555940559.apps.googleusercontent.com`,
  username claim `email`, and workspace-domain group claim `hd`.
- The platform Cluster API has no public guest OIDC configuration. Its existing
  `spec.issuerURL` is for service accounts and must not be repurposed.
- `gcp-hcp-ctl/pkg/kubeconfig/kubeconfig.go` installs a Google-specific
  `gcphcpctl auth token --audience ...` exec entry. It cannot log into an arbitrary IdP.
- The HostedCluster CRD validated in development sets
  `spec.configuration.authentication.oidcProviders.maxItems=1`. A development hosted
  cluster used Google OIDC. The generated `auth-config` maps `email`,
  `hd`, and `sub` to username, groups, and UID respectively.
- HyperShift exposes guest authentication through
  `HostedCluster.spec.configuration.authentication`. Its source API includes issuer,
  audiences, claim mappings, prefix policies, and validation rules, some gated by release
  or feature gates. Source presence does not establish support on every release.

This is implementation/schema evidence, not a customer-IdP or replacement PoC. Day-two
support, rollout semantics, claim validation, plugin compatibility, and recovery must be
validated before enablement. Do not switch an existing live customer cluster to test it.
Keep public validation notes abstract; do not include environment cluster/project names,
resource UIDs, account details, internal endpoints, or deployment-specific metadata.

## Target Design

### Configuration path

```text
Customer authenticates to the platform using the existing platform identity
  -> creates/updates Cluster authentication configuration
  -> platform validates and authorizes the effective Cluster object
  -> hc-controller builds the selected HostedCluster OIDC configuration
  -> Firestore desires -> management-cluster kube-applier -> HostedCluster
  -> HyperShift reconciles guest API authentication

Customer uses platform authentication -> fetches bootstrap credential
  -> creates guest RBAC bindings for the effective customer IdP identities
Customer uses external OIDC exec plugin -> customer IdP token -> guest API -> guest RBAC
```

The customer never needs management-cluster Kubernetes access. A bootstrap certificate
authorizes guest operations only; it cannot configure Gecko or the HostedCluster.
Manual edits to managed HostedCluster authentication are unsupported and may be
overwritten. No direct Regional-to-Management Cluster or Gecko-to-guest API connection
is introduced. Public issuer discovery/JWKS requests require approved egress, not an
exception permitting direct cross-cluster Kubernetes coordination.

### 1. Platform Cluster API

Add typed, public guest authentication fields in `platform-api/api/private/v1/` and
generate public types, conversions, schemas, and deep copies. Do not hand-edit generated
files. The following is a **proposed API fragment**, not an existing supported field:

```yaml
spec:
  authentication:
    type: CustomerOIDC
    oidc:
      name: acme
      issuerURL: https://login.example.com/realms/acme
      clientID: hcp-kubectl
      audiences:
      - hcp-kubectl
      usernameClaim: sub
      groupsClaim: groups
```

Proposed contract:

| Field | Behavior |
|-------|----------|
| `authentication.type` | `Google` or `CustomerOIDC`; omitted configuration on creation/legacy objects retains current Google behavior |
| `authentication.oidc` | Required only for `CustomerOIDC`; forbidden for explicit `Google` |
| `name` | Bounded display/provider name, not an identity isolation boundary |
| `issuerURL` | Exact HTTPS OIDC issuer, including tenant/realm path; no query, fragment, userinfo, or unsupported private destination |
| `clientID` | Customer-registered public CLI application's client ID; not a secret |
| `audiences` | Explicit accepted ID-token audiences, defaulting to the client ID when omitted; validate bounds and client/plugin compatibility before permitting broader values |
| `usernameClaim` | Defaults to `sub`; must resolve to a non-empty supported string claim |
| `groupsClaim` | Optional supported claim name; omission means no customer group mapping, not an implicit `groups` or `hd` fallback |

Finalize exact Go types, bounds, list semantics, audience defaulting, and API names in
Phase 0 following `platform-api/API-CONVENTIONS.md`. No arbitrary provider configuration,
CA bundle/reference, discovery URL override, CEL expression, client secret, access token,
refresh token, or customer-supplied exec command belongs in this initial contract.

#### Defaulting, patching, and replacement

- Existing objects without authentication configuration retain the exact Google issuer,
  audience, and username/group mapping behavior; do not retroactively prefix Google users.
- A PATCH that does not mention authentication preserves it. For an existing
  `CustomerOIDC` configuration, removal/null or omission in a full-object replacement
  must not silently restore Google. Require an explicit transition to `type: Google`.
  Test validation before/after defaulting so defaults cannot bypass this requirement.
- Require a complete valid target configuration for a provider replacement. Do not
  expose a transient zero-provider state or accept partially invalid desired state.
- Use resource-version preconditions for replacements; concurrent updates must not
  silently discard another administrator's configuration. Reject conflicting writes.
- Changes to issuer, client/audience, or claim mappings are trust/identity changes and
  must produce a new observed authentication revision. They may require RBAC migration.
- No automatic guest binding edits, migration, or deletion accompany these transitions.

#### Authorization and audit

Initial configuration is authorized by `cluster.create`; all day-two authentication
changes use existing `cluster.update`, with the established effective-object Cedar
evaluation and project boundary. No creator-only bypass or separate permission is added.

**Security consequence:** anyone granted applicable `cluster.update` can change guest
authentication trust, even if they lack `cluster.credential.get`. Document and review
existing grants accordingly. Customer IdP administration can also control identities
that match guest RBAC grants. Do not present configuration updates as harmless metadata.

Record actor, project, Cluster UID, request ID, old/new safe configuration revisions,
outcome, and explicit return-to-Google operations. Treat URLs/claim names as customer
data: fixed failure categories, no raw error prose, tokens, or full request-body logging.
Preserve normal platform authorization deployment requirements; no new exemption or
allowlist is introduced by this feature.

### 2. Identity Mapping and Validation

For customer-managed providers, generate explicit issuer-scoped prefixes for usernames
and groups, including when the username claim is `email`. Expose the exact resulting
prefixes as safe effective metadata so customers can prepare RBAC before replacement.
The prefix must be deterministic from the exact issuer and a documented mapping domain
version, independent of display name. Use distinct user/group namespaces; disallow empty
prefixes and reserved `system:` identities. Finalize encoding/length/collision handling
in Phase 0; do not depend on unspecified upstream prefix defaults.

An illustrative effective identity is:

```text
issuer: https://login.example.com/realms/acme
username claim: sub = alice-123
effective username: https://login.example.com/realms/acme#user:alice-123
group claim: groups = [engineering-admins]
effective group: https://login.example.com/realms/acme#group:engineering-admins
```

The example prefix encoding is illustrative until the contract is finalized. Changing
the issuer changes its identity namespace. Returning to an earlier issuer can reactivate
bindings left behind for it; explicitly warn about this during repair/reversion. Changing
claim names or client applications can also change subjects (including pairwise `sub`)
and group membership even if the prefix remains the same. Do not assume audience/client
changes are identity-neutral or automatically migrate existing grants.

Validation requirements:

- Match signed tokens to the exact issuer and configured audiences; require the expected
  token lifetime/signature checks. Do not accept arbitrary OAuth access tokens or generic
  JWTs as if they were compatible OIDC ID tokens.
- Validate tenant-specific issuer semantics; a generic multi-tenant login endpoint or
  tenant claim alone is not equivalent to accepting the correct issuer.
- Default to `sub`, not mutable email. For `email` usernames, require enforcement of a
  boolean verified-email condition in addition to prefixing. Validate upstream behavior
  and feature gates; if it cannot be safely enforced, reject that mapping on the release.
- Bound claim-name lengths and identity sizes. Define missing/empty/malformed username
  and group behavior. Support only explicitly tested group claim shapes, such as string
  arrays; reject incompatible tokens rather than treating object values as identities.
- Omitted group mapping must not synthesize admin groups. Group values in tokens do not
  grant roles without guest RBAC. Do not fetch group-overage/UserInfo/directory data in
  this release; customers must issue the required supported claims in their tokens.
- Reject arbitrary mapping/validation expressions and reserved identity mappings.
  Platform-generated safety rules are allowed only where supported and tested; this is
  not a public arbitrary-expression feature.

### 3. Public Issuer Reachability and SSRF Safeguards

The hosted API server must retrieve discovery/JWKS data, while the customer's plugin/
browser must reach the authorization/token endpoints. Neither worker reachability nor
a successful request from a developer laptop proves management-cluster reachability.

- Require HTTPS with public trust and strict hostname verification. No insecure TLS
  fallback or custom issuer CA delivery in this release.
- Reject private, loopback, link-local, metadata-service, management/internal, and other
  prohibited destinations, including IPv6 forms, DNS aliases, and redirect targets.
- Validate discovery issuer equality and discovery-provided JWKS URLs, not only the
  submitted URL. Bound response sizes, timeouts, redirects, and retries for any probe.
- Account for DNS rebinding and later discovery/JWKS changes. A one-time URL or DNS check
  is insufficient: define enforceable egress/runtime destination controls for the actual
  fetching components. If the required isolation cannot be enforced in the shared MC,
  do not enable arbitrary customer issuers until the design is resolved.
- Keep customer discovery off the synchronous API admission hot path. Decide whether an
  asynchronous MC-side validator is required, and how its safe observations return via
  transport. Never add direct RC-to-MC Kubernetes calls or expose a general URL-fetch API.
- Deploy infrastructure safeguards through Terraform/Argo CD. Assess allowed external
  egress and shared-control-plane effects before enabling the feature broadly.

### 4. Controller Reconciliation and Observability

Extend manifest inputs to carry the validated guest authentication selection. Preserve
the default Google rendering and change only guest authentication fields. Do not modify
the service-account issuer, WIF pool/provider, infrastructure service accounts, or guest
RBAC. Deliver the desired HostedCluster via the existing asynchronous transport.

Map customer fields into the supported HyperShift issuer/claim-prefix structures;
verify the actual release schema and feature gates for username prefixes, group omission,
verified-email validation, and day-two provider changes. Determine update ordering and
supported rollout behavior on all supported releases before permitting replacements.

Expose desired/effective mapping metadata and an observed authentication revision
through public status. Exact field/condition names are Phase 0 decisions. Distinguish:

| State | What it establishes | What it does not establish |
|-------|---------------------|----------------------------|
| API accepted | Configuration passed local validation/authorization | Issuer reachable or guest updated |
| Desired config delivered | Corresponding HostedCluster apply/read observation matches the target revision | All API replicas use it |
| Authentication reconciled | Supported upstream observations establish the target rollout state | A customer can obtain a usable token or has guest RBAC |
| Issuer health observed | A defined bounded check or upstream condition establishes discovery/trust health | End-user login, MFA completion, or authorization |
| Customer verification | Customer uses normal IdP login and checks an intended guest operation | Platform-generated proof unless an explicit verification mechanism is later designed |

Never equate `HostedCluster Available=True` or successful ApplyDesire status with a
completed authentication replacement. Identify the exact upstream status/rollout signals
and any additional applier feedback needed. Mark observations stale/unknown when they
cannot confirm the target revision. Do not invent a ready condition unsupported by data.
Do not call the guest API from Gecko to perform login or authorization tests.

### 5. CLI and External Login

Use **int128/kubelogin**, invoked as `kubectl oidc-login`, as the candidate external
exec plugin, not the different Azure tool also named kubelogin. Pin and test a supported
version, public-client authorization-code/PKCE flow, redirect URIs, scopes, token refresh,
cache permissions, and credential API version before publishing executable instructions.
The customer registers the client in their IdP; Gecko does not provision it or store a
client secret. Confidential-client-only configurations are out of initial scope.

An illustrative, manually installed exec-auth entry is:

```yaml
users:
- name: acme-oidc
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: kubectl
      args:
      - oidc-login
      - get-token
      - --oidc-issuer-url=https://login.example.com/realms/acme
      - --oidc-client-id=hcp-kubectl
      interactiveMode: IfAvailable
```

This is a proposed kubeconfig fragment, not a complete file or validated provider
recipe. Bind a separate normal OIDC context to this user and the observed guest API
endpoint, preserving TLS verification and unrelated entries. Additional scopes and
redirect registration are provider-specific. Do not copy a whole generated config over
the user's existing kubeconfig or place tokens in command arguments/shell history.

Initial CLI changes:

- `cluster create` must accept the supported typed configuration through a documented
  input path; finalize file/flag UX in Phase 0. Permit authenticated day-two updates
  through the existing Cluster API, without requiring a new dedicated CLI update command.
- `cluster get`/JSON output makes safe desired/effective authentication metadata and
  rollout state available for manual plugin configuration and RBAC preparation.
- `cluster login` detects effective/desired customer-managed mode before any kubeconfig
  write. Give a fixed, actionable unsupported-native-login message and documentation
  pointer; do not generate Google credentials, auto-install a plugin, execute server-
  supplied commands, or overwrite customer configuration. Handle pending transitions
  conservatively, including return-to-Google until the observed configuration permits it.
- Preserve Google login behavior for default clusters and preserve the independent
  `cluster credential` bootstrap path. Define safe behavior for old clients that cannot
  recognize the new mode, and document supported client versions before rollout.

### 6. Customer Workflows

#### Create with a customer IdP

1. Register a public OIDC CLI application, enable the tested flow, configure redirect
   URIs/scopes, and issue the desired username/group claims in ID tokens.
2. Authenticate to the platform as usual and create the Cluster with customer OIDC
   configuration. Wait for the target authentication revision to reconcile.
3. Retrieve bootstrap credentials using platform authentication. Learn the exact
   effective/predicted issuer-scoped identities from the supported metadata contract.
4. Create guest RBAC for the intended user or group using the separate bootstrap context.
   A user grant would look like this for the illustrative mapping above:

   ```shell
   kubectl --context my-project-my-cluster-bootstrap create clusterrolebinding alice-admin \
     --clusterrole=cluster-admin \
     --user='https://login.example.com/realms/acme#user:alice-123'
   ```

5. Manually configure the supported plugin and a distinct normal OIDC context, then verify
   authentication and authorization using that context, not the bootstrap certificate:

   ```shell
   kubectl --context my-cluster-acme auth whoami
   kubectl --context my-cluster-acme auth can-i create clusterrolebindings
   # Expected after the correct binding is observed: yes
   ```

6. Follow the bootstrap plan's local credential cleanup guidance. Mapping alone creates
   no RBAC grant; the platform does not verify or create it on the customer's behalf.

#### Replace Google or another customer provider

1. Obtain the current configuration/resource version and save its non-secret values for
   an explicit recovery operation. Establish the new IdP/client and determine the exact
   future prefixed identities before switching.
2. Confirm independent platform access and bootstrap credential retrieval. The actor
   performing recovery needs `cluster.update`; guest RBAC repair also needs a bootstrap
   retrieval grant (`cluster.credential.get`) or another functioning guest administrator.
   `cluster.update` alone does not guarantee authority to fetch bootstrap credentials.
3. Using existing guest admin access or bootstrap credentials, pre-create the intended
   new identity bindings. Prepare a separate normal kubeconfig context/plugin entry.
4. Submit the complete replacement through the platform Cluster API with a resource-
   version precondition. Only one provider is configured. Do not promise Google and the
   customer IdP can coexist or that updates across API replicas are instantaneous.
5. Observe the target authentication revision and defined rollout signals. Test new
   login and an intended authorized guest operation from the customer client. Mixed
   replica state or failed issuer access can temporarily prevent normal access.
6. After verification, the customer removes obsolete bindings, IdP clients, and local
   credential caches as appropriate. Existing legacy domain bindings are not removed by
   Gecko; retaining them can matter if Google is restored later.

#### Explicit recovery and return to Google

If new login fails, retain platform authentication as the independent repair path.
Explicitly correct the customer configuration, restore the saved compatible provider,
or submit `authentication.type: Google` with the customer OIDC block removed. Observe
reconciliation again; there is no automatic rollback to the previous trust configuration.

Use bootstrap access to repair guest RBAC where needed. Returning to Google does not
automatically grant any Google identity guest access. Conversely, retained Google or
previous-provider bindings may become effective again. Warn customers to assess both
lockout and unintended restored grants before reverting. Do not automatically recreate
the old domain-wide admin binding or resurrect the legacy RBAC Job.

Replacing/removing a provider is not an instantaneous global revocation promise. Define
propagation, in-flight request, connection, and cache behavior in supported releases;
IdP account removal or group changes may also leave issued tokens valid until expiry.
Plugin cache cleanup alone is not server-side revocation. An outage of the platform or
bootstrap delivery path can still delay recovery; this is not an offline fallback.

## Implementation Phases

### Phase 0 — contracts and support gates

1. Finalize API defaulting/null/PUT/PATCH semantics, audience/client relationship, safe
   prefix encoding, claim shapes, public effective metadata, and revision tracking.
2. Prove supported HyperShift releases can reconcile provider/mapping/client replacement
   and explicit return-to-Google. Verify relevant feature gates, verified-email behavior,
   group omission, and exact rollout/issuer-health observations. If day-two support is
   unavailable, resolve the upstream dependency before claiming this scope is supported.
3. Design enforceable public discovery/JWKS egress/SSRF controls, asynchronous validation
   placement, safe transport observations, and customer-facing error states.
4. Review existing `cluster.update` grants and audit coverage. Validate role conditions
   against the proposed effective configuration without adding a new permission.
5. Pin/test the external plugin and public-client flow; decide CLI create input and
   transition/version-skew handling. Publish tested provider recipes only after validation.
6. Make the bootstrap feature's availability, independent permissions, security gates,
   and recovery runbook prerequisites for enabling customer-managed OIDC.

### Phase 1 — API, controller, and infrastructure

1. Add private source types/validation and generate public artifacts twice, verifying
   the second pass has no diff. Cover create/update authorization and default compatibility.
2. Parameterize HC manifest rendering, enforce the selected one-provider configuration,
   and add revision-aware safe feedback/status. Preserve workload/WIF fields and Google
   rendering for existing clusters. Add idempotency/restart/concurrent-update tests.
3. Implement and validate required egress/probe controls and safe failure reporting.
   Keep feature enablement closed until the runtime security/support gates pass.

### Phase 2 — CLI and documentation

1. Add typed create input and safe authentication metadata display. Guard Google-only
   login before kubeconfig mutation; retain the external-plugin/manual setup workflow.
2. Publish creation, pre-binding, replacement, provider-specific plugin, explicit repair,
   return-to-Google, residual-token lifetime, and local cleanup instructions.
3. Add tests for unmodified Google login, custom/pending modes, preserved contexts,
   safe diagnostics, client version skew, and independent bootstrap access.

### Phase 3 — validation and staged enablement

1. Run focused and owning-module tests, generation/lint checks, and disposable-cluster
   e2e with a public test issuer. No broad IdP product-support claim from one test issuer.
2. Exercise creation, Google-to-custom, custom-to-custom, mapping/audience changes,
   explicit repair, and return-to-Google. Prove real OIDC login and guest RBAC, not
   success inherited from the bootstrap context or old domain bindings.
3. Exercise invalid/unreachable issuers, key rotation, refresh failures, admission/egress
   protections, interrupted reconciliation, mixed replicas, restart, and concurrent edits.
4. Enable progressively with supported releases/CLI versions and bootstrap recovery
   already available. Define disable/rollback behavior that does not silently restore
   Google or strand existing customer-managed clusters on an older renderer.

## Acceptance Criteria

- Omitted authentication preserves existing Google guest behavior and never modifies
  platform identity, service-account issuer, GCP WIF, or infrastructure identities.
- `cluster.create`/`cluster.update` and existing object conditions govern configuration;
  viewers, unauthorized/cross-project callers, and guest-only tokens cannot mutate it.
- The API accepts complete supported configurations and rejects unsupported/private
  issuers, unsafe identity mappings, secrets, arbitrary expressions, and accidental
  resets through null/defaulting/full-object updates. Conflicting updates fail safely.
- Discovery/JWKS and subsequent requests cannot reach prohibited internal destinations,
  including through DNS rebinding, redirects, or changed discovery metadata. Tests
  demonstrate runtime controls, not just admission-time string validation.
- Correct issuer/audience/signature/lifetime and supported claims authenticate; wrong
  issuers/audiences, unverified-email mappings, malformed claims, and reserved identities
  fail closed. Names are issuer-scoped and match documented effective RBAC identities.
- Creation and day-two replacement work on the support matrix. Reported readiness is
  tied to the observed target revision and does not claim customer authorization.
- Customer-created user/group bindings govern access. No platform guest RBAC call, new
  default admin grant, or automatic migration/removal of existing bindings occurs.
- Existing Google login is unchanged; custom or pending authentication does not cause
  `cluster login` to write Google auth or damage an existing kubeconfig. A tested manual
  plugin flow obtains/refreshes tokens without platform custody of client secrets/tokens.
- IdP outage, bad replacement, and interrupted rollout are recoverable through explicit
  platform updates and independently authorized bootstrap RBAC repair. Reversion warns
  about retained grants; the legacy bootstrap Job is never silently reintroduced.
- No secret, token, sensitive raw error, or full authentication request body appears in
  logs/traces. Safe audit events identify changes and outcomes; generation, docs, unit,
  integration, negative-security, and disposable-cluster e2e checks pass.

## Risks and Deferred Scope

- **Trust administration:** existing `cluster.update` includes guest trust changes; a
  narrower permission model would require a separate product decision, not this plan.
- **Single-provider replacement:** rollout can disrupt existing login, and per-replica
  convergence must not be confused with atomic revocation or a dual-provider window.
- **Network isolation:** arbitrary issuer metadata introduces SSRF/egress risks on a
  shared MC; public trusted HTTPS alone does not prevent access to internal destinations.
- **Identity drift:** issuer/client/claim changes and recycled email/group names can
  lock users out or revive old grants. Prefixes do not replace customer lifecycle policy.
- **Recovery dependencies:** customer IdP is independent of platform authentication, but
  platform/bootstrap outages and missing recovery permissions can still block repair.
- **Client skew:** older Google-only clients must not silently reset authentication or
  corrupt login entries; staged rollout and support guidance must cover them.
- **Deferred:** private issuers/custom CAs, SAML bridges operated by the platform,
  multiple guest providers, arbitrary expressions, directory/UserInfo enrichment,
  confidential-client secrets, console integration, native CLI token management, automatic
  plugin configuration, automatic rollback, and platform-authentication federation.

## Open Questions

- [ ] What exact API field bounds, audience/client rules, prefix encoding, and public
      desired/effective revision metadata fit supported upstream releases?
- [ ] Which upstream observations prove authentication rollout and issuer health, and
      what additional safe transport/MC-side validation feedback is needed?
- [ ] How will verified-email checks and absent group mappings be enforced on each
      supported release without exposing arbitrary customer expressions?
- [ ] Which egress/runtime controls enforce public-only discovery/JWKS across DNS and
      metadata changes, and who owns the infrastructure changes?
- [ ] Which int128/kubelogin version, flow, scopes, redirect registration, cache behavior,
      and IdP configurations pass the initial support matrix?
- [ ] What exact create-input UX, safe login transition behavior, version-skew policy,
      and concurrent replacement UX will the CLI/API publish?
- [ ] What are the measured replacement/reversion convergence and residual-access
      behaviors, and the operational procedure if the platform/bootstrap path is impaired?

## Backlog

| Story | Jira | Status |
|-------|------|--------|
| API/defaulting/prefix, authorization, and support-matrix contracts | TBD | Not Started |
| Public issuer egress/SSRF and asynchronous validation design | TBD | Not Started |
| Cluster private/public authentication types and validation/generation tests | TBD | Not Started |
| HC manifest rendering, revision-aware observations, and reconciliation tests | TBD | Not Started |
| Infrastructure safeguards and safe authentication-change audit coverage | TBD | Not Started |
| CLI create input, safe metadata, Google-login guard, and manual-plugin documentation | TBD | Not Started |
| Disposable-cluster create/replace/repair/revert and negative-security e2e | TBD | Not Started |
| Supported external-plugin recipes, staged rollout, and recovery/version-skew runbooks | TBD | Not Started |

## References

- [Customer bootstrap access](gcp-customer-bootstrap-access.md): independent credential
  retrieval, manual guest RBAC, and recovery prerequisites. Its Google exec-auth examples
  describe the default mode; customer-managed guest login follows this plan instead.
- [Customer authentication background](gcp-customer-authentication.md): earlier design
  context; not a claim that its Identity Platform/IAM-role proposals are deployed.
- Gecko: `platform-api/api/private/v1/cluster_types.go`,
  `platform-api/{AGENTS.md,API-CONVENTIONS.md,api/public/README.md}`,
  `platform-api/pkg/authz/{middleware,context,permissions}.go`,
  `controllers/hc/{hc_controller.go,manifest/manifests.go}`, and
  `controllers/client/transport/firestore/status.go`.
- CLI: `pkg/{cluster/login.go,kubeconfig/kubeconfig.go,platformapi/client.go}` and
  `pkg/auth/`; platform token acquisition is not a generic guest OIDC client.
- HyperShift: `api/hypershift/v1beta1/hostedcluster_types.go` (`ClusterConfiguration`);
  OpenShift API `config/v1/types_authentication.go` (`AuthenticationSpec`, `OIDCProvider`,
  `TokenIssuer`, and claim/prefix/validation types). Verify release-specific generated
  schemas and feature gates, not just the latest source types.
- [int128/kubelogin](https://github.com/int128/kubelogin): candidate external exec plugin;
  a supported version and executable recipe remain to be pinned and tested.
