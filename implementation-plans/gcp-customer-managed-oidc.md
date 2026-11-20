# Customer-Managed OIDC Authentication

> **Status:** Draft — direct customer-issuer trust proposed; compatibility and safe
> day-two replacement are implementation gates, not completed product capabilities.

**Identifier:** `customer-managed-oidc` · **Jira:** TBD

**Components:** gecko platform API/HC controller/transport; `gcp-hcp-ctl`; MC egress controls

**Depends on:** supported HyperShift External OIDC and
[customer bootstrap access](gcp-customer-bootstrap-access.md)

## Summary and decisions

Allow customers to configure a guest Kubernetes API OIDC provider at creation and
replace it afterward. Gecko and its CLI currently hardcode Google guest authentication;
this plan adds an opt-in configuration path, not a new platform login system.

| Agreed scope | Boundary |
|--------------|----------|
| Google remains the default | Existing clusters and omitted create configuration retain current issuer/audience/mappings |
| Create and replace | Google-to-custom, custom-to-custom, client/mapping changes and explicit return-to-Google |
| Existing `cluster.create` / `cluster.update` | Object-aware Cedar/project boundaries; no new authentication-update permission |
| Public HTTPS, publicly trusted issuer TLS | No custom CAs or private issuer connectivity initially |
| Configurable claim names, username default `sub` | Enforced issuer-scoped user/group prefixes; no arbitrary customer expressions |
| External exec-plugin login | No native CLI browser/token cache, automatic plugin installation/configuration or client-secret custody |
| Explicit recovery, not automatic rollback | Independent platform access and separately authorized bootstrap RBAC repair |

The development-validated HostedCluster schema permits **one OIDC provider**. This
baseline replaces the guest Google provider, not platform authentication, service-account
issuer or GCP WIF. Cluster `spec.issuerURL` remains the service-account issuer and must
not be repurposed. Guest tokens do not become platform credentials.

Guest RBAC stays customer-managed; Gecko makes no guest API calls to create bindings or
test login. The [bootstrap plan](gcp-customer-bootstrap-access.md#alternatives-and-prerequisites)
owns the comparison with Gecko-derived administrator access. Manual binding creation is
the baseline workflow, not a claim that every administrator needs a new binding.

## Design and API contract

### Configuration and authority

Add typed fields under `platform-api/api/private/v1/` and generate public types,
conversions, schemas and deep copies; do not edit generated output. Proposed fragment
(**not yet an implemented API**):

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

| Field | Contract |
|-------|----------|
| `authentication.type` | `Google` or `CustomerOIDC`; absent create/legacy configuration retains Google |
| `authentication.oidc` | Required for `CustomerOIDC`, forbidden for explicit `Google` |
| `name` | Bounded display/provider name, not an identity boundary |
| `issuerURL` | Exact tenant/realm HTTPS issuer; no query, fragment, userinfo or prohibited private destination |
| `clientID`, `audiences` | Registered public CLI client and explicit accepted ID-token audiences; default audiences to client ID, gate broader values on validated compatibility |
| `usernameClaim` | Defaults to `sub`; supported, non-empty string identity |
| `groupsClaim` | Optional supported claim; omission means no customer group mapping, not implicit `groups`/`hd` fallback |

Use existing create/update authorization with effective-object Cedar conditions; no
creator-only bypass or authorization exemption. **Applicable `cluster.update` grants
authority to change guest authentication trust even without `cluster.credential.get`.**
Review existing grants and document the IdP administrator's power over identities that
match guest RBAC. This is a trust change, not harmless metadata.

API update semantics:

- Preserve the exact existing Google defaults, including unprefixed Google identities;
  do not retroactively change them when adding optional fields.
- PATCH omission preserves authentication. For existing custom configuration, removal /
  null or omission in a full-object replacement must not silently reset to Google.
  Require explicit `type: Google` with the customer block removed; test defaulting order.
- Accept a complete valid target configuration, not an intermediate zero-provider state.
  Use resource-version preconditions and reject conflicts rather than losing updates.
- Issuer, audience/client and claim changes produce a new observed authentication
  revision. They may change identities; do not automatically edit/delete guest grants.

### Reconciliation and status

```text
Customer platform request -> validated/authorized Cluster
  -> HC controller -> Firestore desires -> MC applier -> HostedCluster authentication
  -> HyperShift guest authentication reconciliation
Customer exec plugin -> configured IdP -> guest API -> existing guest RBAC
```

Parameterize the HC manifest input and render only guest authentication changes. Do not
alter service-account issuer, WIF pools/providers, infrastructure identities or guest
RBAC. Bootstrap certificates authorize guest operations, not platform/MC configuration;
manual HostedCluster edits may be overwritten. Keep RC–MC coordination asynchronous;
no new direct cross-cluster Kubernetes or Gecko-to-guest connection is introduced.

Publish safe desired/predicted and observed effective prefixes/revisions so customers
can pre-bind future identities. Base conditions on real supported observations:

| Observation | Does not establish |
|-------------|--------------------|
| API validation/authorization accepted | Issuer reachable or guest configured |
| Target HC apply/read revision observed | All guest API replicas use it |
| Authentication rollout reconciled | Customer can obtain a token or has RBAC |
| Issuer discovery/trust health observed | MFA/login/guest authorization succeeds |
| Customer checks normal login and intended operation | A platform-generated verification signal unless separately designed |

Do not treat `HostedCluster Available=True` or successful apply as completed replacement.
Define exact upstream rollout/health signals and any safe MC-side validator/transport
feedback; mark missing/stale observations unknown. Gecko does not probe guest login.

### CLI behavior

- Provide typed create input and safe authentication metadata in `cluster get`/JSON;
  day-two updates use the existing Cluster API, without requiring a new update command.
- Before **any kubeconfig write**, `cluster login` detects desired/effective custom or
  pending transition state. Return a fixed actionable external-login message/docs link,
  not a broken Google entry. Treat return-to-Google conservatively until observed state
  permits it. Do not execute server-supplied commands or overwrite customer entries.
- Preserve default Google login and independent `cluster credential`; define old-client
  version-skew behavior so unrecognized fields cannot reset authentication or corrupt login.
- Use [int128/kubelogin](https://github.com/int128/kubelogin), invoked as
  `kubectl oidc-login`, as the **candidate** manual plugin, not the unrelated Azure tool.
  Pin/test a version before publishing recipes: public-client authorization-code/PKCE,
  redirect registration, provider-specific scopes, ID-token audience, refresh/cache
  permissions and exec credential version. Customers register their own client.
- Instructions must merge a distinct normal context using the observed HTTPS endpoint,
  preserve unrelated entries/TLS verification, and keep tokens out of arguments/history.
  The plugin owns local tokens; Gecko neither provisions a client nor stores its secrets.

## Alternatives and prerequisites

### Interaction with Gecko-derived guest administrator access

The bootstrap plan owns the [three-way alternative comparison](gcp-customer-bootstrap-access.md#alternative-gecko-derived-guest-administrator-access)
and [verification evidence](gcp-customer-bootstrap-access.md#claim-mapping-verification-in-a-development-environment).
Reconciled mapping can use the configured issuer without consuming another provider
slot; a broker cannot simply coexist with Google/customer OIDC under the one-provider
constraint. Broker federation changes this plan's direct-trust/login assumptions, while
RBAC synchronization introduces a guest writer. None is selected or implemented here.

Extending Gecko entitlement to a customer issuer needs an authoritative principal
association and target-specific, condition-aware policy. Do not join issuers by matching
email alone or ignore Gecko's Unicode/domain/local-part normalization semantics. Keep
customer-supplied reserved-group mappings disallowed under this baseline; the verified
platform-owned mechanism is not a public arbitrary-expression feature.

| Gate before enablement | Unresolved decision or prerequisite |
|------------------------|-------------------------------------|
| API/identity | Final field/types/bounds, audience/client rules, list/defaulting semantics, prefix encoding/version/collision handling and desired/effective revision metadata |
| Release support | Actual schemas and feature gates, username prefixes, verified-email enforcement, group omission/shapes and day-two issuer/client/mapping changes; resolve upstream gaps before claiming support |
| Reconciliation | Exact rollout/health/staleness evidence, safe MC validation placement and return feedback; no invented ready condition |
| Public issuer safety | Enforce runtime egress/destination restrictions across DNS and discovery changes; assign infrastructure owners, do not substitute a one-time URL check |
| Client/support matrix | Tested plugin/version/flow/scopes/redirect/cache behavior, create UX, concurrent-update UX, old-client handling and explicit repair/version rollback |
| Recovery/operations | Available bootstrap feature/security gates, independent permissions, measured replacement/reversion and residual-access behavior, platform/bootstrap outage procedure |
| Alternative track only | Entitlement/action and identity association; mapping-size/churn, concurrency, stale policy, caches/connections, isolation and recovery; architecture selection remains separate |

Resolve these in Phase 0 rather than treating example types or untested plugin/IdP
configurations as product support. Public OIDC products such as Okta, Entra ID and
Keycloak are compatibility candidates, not a blanket supported-provider claim.

## Security and lifecycle

### Identity and token constraints

Generate explicit deterministic **issuer-scoped** prefixes for customer usernames and
groups, including email usernames. Separate user/group namespaces, disallow empty and
reserved `system:` identities, and do not derive isolation from the display name or
unspecified upstream defaults. For example, a finalized scheme might yield
`https://login.example.com/realms/acme#user:alice-123`; exact encoding is still a gate.

Require exact issuer/audience, signature and lifetime validation, and tenant-specific
issuer semantics—not arbitrary access tokens/generic JWTs or a generic multi-tenant
endpoint with an assumed tenant restriction. Bound identity/claim sizes and validate
missing/empty/malformed values. Prefer `sub` to mutable email; an email mapping must
enforce boolean verified-email validation or be rejected on that release. Platform-
generated safety rules must themselves be supported and tested.

Only tested group shapes are allowed; absent mapping must not synthesize admin groups.
Required membership must be present in the token: directory/UserInfo/group-overage
enrichment is deferred. Group identity alone creates no RBAC object, but existing
bindings can already grant permissions, as explained in the bootstrap plan.

An issuer change changes the identity namespace; client/claim changes can also alter
subjects (including pairwise `sub`) or groups. Returning to a previous issuer may revive
retained bindings. Prefixing does not replace customer identity/grant lifecycle policy.

### Network and data constraints

- Require publicly trusted HTTPS and hostname verification, with no insecure fallback.
  KAS must reach discovery/JWKS; the customer client/browser must reach login/token
  endpoints. Laptop or worker reachability does not prove MC reachability.
- Block private, loopback, link-local, metadata-service, management/internal and other
  prohibited destinations, including IPv6, aliases and redirects. Validate discovery
  issuer equality and its JWKS URLs, not only the submitted issuer URL.
- Enforce restrictions on the **actual runtime fetchers**, accounting for DNS rebinding
  and changed discovery/JWKS data. Bound probe responses, timeout, redirects and retries.
  If isolation cannot be enforced on the shared MC, do not enable arbitrary issuers.
- Keep discovery off synchronous API admission. Any MC-side validator is asynchronous,
  bounded and returns safe observations through transport—not a general fetch service.
  Deploy safeguards through Terraform/Argo CD and assess shared-MC/egress impact.
- No CA bundle/reference, discovery override, arbitrary mapping/validation CEL, client
  secret, access/refresh token or supplied exec command enters the initial Cluster API.
  Confidential-client-only configurations are out of scope.
- Audit actor, project, Cluster UID, request ID, safe old/new revisions, outcome and
  explicit returns to Google. URLs/claim names are customer data: use fixed error
  categories, no raw errors/full request bodies/tokens in logs or traces. Public trust
  changes get no new authentication/authorization exemption.

### Customer operations

1. **Create:** register the tested public client/redirects/scopes and token claims;
   authenticate to the platform, create with custom OIDC and observe target reconciliation.
   Use bootstrap access for intended effective user/group bindings, configure the manual
   plugin/context, and verify both authentication and an intended privileged operation
   through normal OIDC. Follow the bootstrap plan's local-key cleanup guidance.
2. **Replace:** save current non-secret configuration/resource version, establish the
   new provider and future identities, confirm independent recovery, pre-create intended
   guest bindings and prepare a separate context. Submit a complete conditional update,
   observe convergence, verify new login/authorization, then let the customer remove
   obsolete bindings, clients and caches. Do not rely on a dual-provider window or
   atomic cross-replica switch; mixed replicas/outages can temporarily prevent login.
3. **Repair/revert:** explicitly correct/restore configuration or choose `type: Google`
   with the custom block removed, then observe reconciliation again. Platform repair
   requires `cluster.update`; bootstrap RBAC repair also requires `cluster.credential.get`
   or another functioning guest admin. Neither permission implies the other.

Returning to Google creates no new grant; retained Google/previous-issuer bindings can
become effective again. Warn about both lockout and revived access. Do not automatically
create/delete/migrate guest bindings, restore the domain-wide grant or resurrect its Job.
Guest authority is separate from management-cluster access; no customer MC access is needed.

IdP account/group changes may leave issued tokens usable until expiry; replacement has
replica/in-flight/connection/cache propagation, not instantaneous global revocation.
Clearing a plugin cache is not server-side revocation. Recovery depends on available
platform/bootstrap paths and adequate grants; it is not offline recovery. There is no
automatic rollback to an earlier trust configuration.

**Deferred:** private issuers/custom CAs, platform-operated SAML bridging, multiple guest
providers, arbitrary expressions, claims enrichment, confidential-client secrets, console
integration, native CLI token management, automatic plugin configuration/rollback and
platform-authentication federation. A SAML-only IdP needs a separately supported OIDC bridge.

## Implementation phases

1. **Contracts/support gates:** resolve the table above, review existing update grants,
   prove upstream day-two support and identify observable status; design network isolation,
   pin/test the plugin and establish recovery prerequisites. Alternative selection is separate.
2. **API/controller/infrastructure:** add private-source types/validation, generate public
   artifacts twice and require an unchanged second pass. Parameterize rendering, revision
   feedback and status; preserve Google/WIF behavior. Implement egress/validator controls;
   keep feature enablement closed until runtime gates pass.
3. **CLI/documentation:** add typed create input, safe metadata and the pre-write login
   guard. Publish manual provider/plugin, pre-binding, replacement, repair, residual-access
   and cleanup recipes plus supported client versions. No native customer-IdP token client.
4. **Validation/staged rollout:** run the matrix below on authorized disposable/canary
   resources with a public test issuer, then progressively enable supported versions.
   Define feature-disable/old-renderer rollback so existing custom configurations are not
   silently reset to Google or stranded. Do not test switches on live customer clusters.

## Acceptance and verification

**Evidence boundary:** development inspection confirmed the single-provider schema,
current Google mappings and configuration path. The linked bootstrap verification proved
group-expression addition/removal under Google, not arbitrary customer IdPs, issuer
replacement, CLI plugin compatibility, the release matrix or automatic Gecko entitlement
propagation. Keep published facts abstract: no environment names/IDs, accounts, endpoints,
deployment-specific metadata or raw credentials. The following tests are still required.

| Area | Acceptance |
|------|------------|
| Compatibility | Omitted/legacy configuration preserves Google login exactly; platform identity, service-account issuer, WIF and infrastructure identities unchanged. |
| Authority | Existing create/update and object conditions enforced; viewers, unauthorized/cross-project callers and guest-only tokens denied. Trust-changing implications documented. |
| API/generation | Valid typed config accepted; unsupported/private input, secrets/commands/expressions and accidental null/PUT/PATCH/default resets rejected. Conflicts safe; second generation pass clean. |
| Identity | Exact issuer/audience/signature/lifetime, verified-email and supported claims/shapes; malformed, reserved or wrong-trust identities fail closed. Effective prefixes match guest RBAC. |
| Network | Discovery/JWKS/probe egress remains safe through aliases, redirects, rebinding and later metadata changes; demonstrate runtime controls, not just admission string checks. |
| Lifecycle | Create, Google-to-custom, custom-to-custom, client/audience/mapping changes, repair and explicit Google return work across supported versions with no automatic RBAC migration or Job resurrection. |
| Observability | Desired/effective revisions, delivery/rollout/health and stale observations accurate; apply/Available alone never asserts customer login or authorization. |
| Client | Default Google preserved; custom/pending/old-client cases do not corrupt/reset config. Manual pinned-plugin login/refresh works with secure caches, correct scopes/redirects and no platform token/secret custody. |
| Failure/recovery | Invalid/unreachable issuer, IdP/JWKS rotation/refresh failure, mixed replicas, restart/interrupted reconciliation and concurrent edits tested; explicit recovery works with its independent grants. |
| End to end | Real normal-IdP login and intended guest authorization verified, not success inherited from bootstrap credentials or old domain grants. No Gecko guest API probes/writes. |
| Data/rollout | Safe change audit and captured-log/trace redaction; supported API/client matrix and disable/rollback procedure preserve existing custom trust. |

Run focused/owning-module suites, generation/lint, integration/security checks and
authorized e2e; document skipped checks. One test issuer does not establish broad IdP
product support. The alternative track needs its own selection and integration acceptance.

## References

- [Customer bootstrap access](gcp-customer-bootstrap-access.md) owns credential export,
  default admin binding, alternative comparison, mechanism verification and recovery.
- Gecko: `platform-api/{AGENTS.md,API-CONVENTIONS.md,api/public/README.md}`,
  `platform-api/api/private/v1/cluster_types.go`, `platform-api/pkg/authz/`,
  `controllers/hc/{hc_controller.go,manifest/manifests.go}` and
  `controllers/client/transport/firestore/status.go`.
- HyperShift: `api/hypershift/v1beta1/hostedcluster_types.go` (`ClusterConfiguration`),
  OpenShift API `config/v1/types_authentication.go` (issuer/claims/prefix/validation),
  and release-specific generated schemas/feature gates; latest source is not a support matrix.
- CLI: `pkg/{cluster/login.go,kubeconfig/kubeconfig.go,platformapi/client.go,auth/}`;
  [int128/kubelogin](https://github.com/int128/kubelogin) remains a candidate to pin/test.
- [Cedar authorization](../design-decisions/identity/cedar-public-api-authorization.md),
  [network isolation](../design-decisions/networking/no-direct-cross-cluster-connectivity.md),
  and [earlier authentication design](gcp-customer-authentication.md) for context; the
  latter's Identity Platform/IAM-role proposals are not evidence of deployed behavior.
