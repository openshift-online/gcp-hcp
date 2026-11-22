# Customer Bootstrap Access

> **Status:** Draft — certificate baseline proposed; claim-mapping mechanism verified
> in development, but Gecko entitlement integration and architecture selection are pending.

**Identifier:** `customer-bootstrap-access` · **Jira:** TBD

**Components:** gecko HC controller, transport, platform API; `gcp-hcp-ctl`

**Related:** [Customer-managed OIDC](gcp-customer-managed-oidc.md)

## Summary and decisions

Replace Gecko's guest RBAC setup Job with customer-visible bootstrap/recovery access.
The baseline exports HyperShift's pre-generated customer admin certificate; the customer
creates their chosen guest bindings, then uses normal OIDC login. This flow makes no
Gecko-to-guest API calls, but Gecko still handles guest superuser credentials.

Today, `buildRBACJob` mounts `service-network-admin-kubeconfig`, polls guest health and
creates `redhat-domain-admins`, granting `cluster-admin` to the `redhat.com` domain group
and creator. Removing that domain-wide platform grant motivates the change. Transport
`Apply` does not prune omitted resources, so deleting the builder alone cannot retire it.

**Creating a new binding is a baseline choice, not a technical necessity.** OpenShift
already binds `system:cluster-admins` to `cluster-admin`. Mapping an eligible identity
into that group works in the tested development environment. The
[alternatives](#alternatives-and-prerequisites) compare ways to derive access from Gecko
authorization without assuming that integration already exists.

| Agreed baseline decision | Consequence |
|-------------------------|-------------|
| Pre-generated customer certificate, not the management-cluster CSR flow | Avoids the privileged-subject admission restriction described below |
| Dedicated `cluster.credential.get`, granted to platform `cluster-admin`, not viewers | Ordinary cluster reads, creator identity, or project membership alone do not authorize export |
| Repeatable, audited retrieval | Supports recovery; not one-time access or an offline fallback |
| Continuous mirroring into the MC status Firestore DB | Requires explicit reader, indexing, retention, audit, and cleanup safeguards |
| Separate CLI bootstrap context; preserve current context | Normal login stays separate; local key cleanup remains the customer's responsibility |
| Additive rollout before retiring Jobs/desires | Supported API/CLI and recovery must exist before automatic admin setup stops |
| Preserve existing guest grants | Customers migrate/remove the legacy domain binding themselves |

Guest Google OIDC is unchanged in this baseline. Customer-selected issuers follow the
complementary OIDC plan; platform authentication and workload WIF remain independent.
SRE credentials and a CLI `--bind-admin` convenience are deferred. Upstream uses the
term "break-glass"; it appears here only where needed for actual upstream names.

## Design and API contract

### Credential source and CSR placement

HyperShift's `control-plane-pki-operator` manages these resources in the HCP namespace:

| Resource | Default rotation expectation |
|----------|-----------------------------|
| `customer-system-admin-signer` Secret | CA validity 7d, refresh 2d |
| `customer-system-admin-signer-ca` ConfigMap | Customer signer trust bundle |
| `customer-system-admin-client-cert-key` TLS Secret | Client validity 36h, refresh 6h |

The certificate has organization `system:masters` and a
`system:customer-break-glass:<id>` common name, trusted through `total-client-ca`.
`system:masters` bypasses Kubernetes authorization; it is stronger than an ordinary
`cluster-admin` RBAC grant. Require the PKI operator on supported releases (OCP 4.15+
floor) and verify the full support matrix. HyperShift owns rotation; Gecko does not tune
`CERT_ROTATION_SCALE`.

**Why not the existing CSR flow?** Its Kubernetes CSR is cluster-scoped on the
**management cluster**, with a matching HyperShift `CertificateSigningRequestApproval`
in the HCP namespace. The PKI operator signs the management-cluster CSR; the resulting
certificate is used against the **guest**. Upstream's test explicitly uses
`mgmt.KubeClient` and `mgmt.HyperShiftClient`, and documents GKE Warden rejection of
the requested `system:` CNs / `system:masters` organization. This is upstream evidence,
not a fresh admission test or a blanket prohibition on all GKE/guest CSRs. Moving the
object to the guest would require a different signer/approval path and guest permissions;
the existing controller does not watch that CSR API. The pre-generated path avoids CSR
objects and has been validated in development.

Rotation is not a serving guarantee or revocation. A leaf nearly 6h old when read and
served 6h later can have roughly 24h remaining, not 30h; outages invalidate even that
estimate. Validate each export using the [serving rules](#serving-validation).

### Delivery and controller ownership

```text
HC controller -> specs DB: standard manifests + standalone credential ReadDesire
MC kube-applier -> status DB: observed HostedCluster state + customer Secret
CLI -> platform-api: authorize parent Cluster, read and validate credential
Customer CLI -> guest API: create intended RBAC, then use normal OIDC
```

Once placement is known and HC desires are applied, maintain a dedicated read group:

```text
GVR:       /v1/secrets
namespace: clusters-{clusterUID}-{safeName}
name:      customer-system-admin-client-cert-key
groupKey:  {clusterUID}-credential
```

Use `cluster.UID` and existing `spec.safeName`/defaulting, not the customer-facing name.
Keep this group separate from `projects/{namespace}/clusters/{name}`. Derive the target
and MC placement from trusted parent state, never caller-provided resource coordinates.

| Component | Required change |
|-----------|-----------------|
| Transport | Add idempotent `EnsureRead` without ApplyDesires; reuse the read-desire builder (`Secret` resource mapping already exists). Do not rewrite unchanged specs or churn acknowledgement timestamps. |
| Typed read results | Existing status extraction handles HC/NodePool/Certificate, not Secret data. Add explicit credential extraction and safe observed metadata; never use generic string feedback or Cluster status to carry keys. |
| Read lifecycle | Add `RemoveRead`: existing `Delete` walks ApplyDesires and cannot remove standalone read groups. Removing a subscription must not delete the HyperShift-owned Secret. |
| HC status | Publish advisory `BootstrapCredentialReady` plus observed validity, not certificate/key data. The API independently validates at fetch time. |
| Deletion | Clean subscriptions and mirrored status/key material after partial or full provisioning. Reject deleting Clusters and prevent reuse across deletion/recreation under the same name. |

Pinned applier source supports standalone per-object list/watch with 60-second **cached**
reconciliation. Source corresponding to the inspected deployment also has orphan-status
cleanup/startup reconciliation. Neither a cache resync, Firestore update time, nor
`Status.Stale` (spec acknowledgement) proves a fresh MC observation. Runtime Secret
delivery/rotation/purge tests remain open; keep status-DB writes owned by the applier.

### Credential endpoint

```text
GET /apis/gcp.managed.openshift.io/v1/namespaces/{project}/clusters/{name}/credential
```

This is a **computed, non-persisted singleton subresource**, not an independently stored
child such as NodePool. Add an explicit Orlop extension, with no generic credential
CRUD, collection, list, or watch exposure. Define types in the private API source and
follow public generation rules; discovery/OpenAPI and any private routes need an
explicit contract, not an accidental alternate access path.

Authorize before reading the Secret, using `cluster.credential.get` and the parent
Cluster's project/identity/spec for Cedar conditions. Add permission validation, action
mapping, role grants, and tests. Reuse the shared authorizer, and explicitly execute its
object check: installing a middleware callback alone does not authorize a custom handler.
Enforce authentication and credential authorization even when
`gecko.public-api.authorization.enabled` is false; fail closed on evaluation/context errors.

Preferred read path: a read-only MC status client in platform-api. An authenticated,
constrained internal service is an alternative, not a bypass or permission to introduce
direct cross-cluster connectivity. Finalize shared-package/deployment boundaries before
build-out; do not import the controllers application wholesale.

| JSON response field | Contract |
|---------------------|----------|
| `kind`, `apiVersion` | `ClusterCredential`, `gcp.managed.openshift.io/v1` |
| `metadata.name`, `metadata.namespace` | Parent Cluster name and project |
| `apiEndpoint` | Validated HTTPS form of `status.hostedClusterResult.apiEndpoint`, not reconstructed DNS |
| `clientCertificate`, `clientKey` | Validated PEM material; CLI assembles the kubeconfig |
| `username`, `validUntil` | Server-derived certificate subject and expiry; CLI may independently validate |

Return `Cache-Control: no-store`; exclude shared caching and conditional credential
responses. Preserve existing auth/parent-not-found conventions without disclosing
readiness to unauthorized callers. Define one typed not-ready reason and bounded retry
guidance; distinguish unavailable delivery (`503`), deletion, and invalid internal data.
Do not overload a normal parent `404` or retry every `409`. Verify applicable rate limits.

### Serving validation

Before every export, require:

1. Expected target/Cluster incarnation, successful read status, and agreed observation
   freshness evidence; readiness alone is insufficient.
2. Decodable Secret, parsed PEM, matching certificate/key, expected customer identity /
   `system:masters` organization, and client-auth usage.
3. Valid `NotBefore`/`NotAfter`, defined clock skew, and minimum remaining lifetime.
4. Rejection of known pre-revocation material, with explicit delivery blocking/resumption
   around signer trust propagation. Valid dates do not prove guest trust.

Thresholds and revocation/freshness evidence are prerequisite decisions. Do not probe
the guest from Gecko or promise fixed remaining lifetime based only on rotation cadence.

### CLI workflow

```text
gcphcpctl cluster credential my-cluster [--kubeconfig path] [-o json]
```

Use `pkg/platformapi` project/name resolution and centralized safe error normalization;
never display server prose or infer policy causes from a `403`. Wait only on the agreed
retryable response, with timeout, cancellation, and bounded retry guidance.

The default output merges a distinct `{project}-{name}-bootstrap` context/auth entry,
preserves unrelated and normal exec-auth entries, and **does not change current-context**.
Re-fetch replaces the same auth entry; define safe collision/overwrite rules and
owner-only files. Use the observed public endpoint with TLS verification on; do not
retain unrelated CA settings or silently fall back to insecure TLS. No CA data was
needed in the validated public-serving configuration; do not generalize that to all TLS.

Print the context, expiry, superuser warning, and a short customer-owned binding example:

```shell
kubectl --context my-project-my-cluster-bootstrap create clusterrolebinding my-admin \
  --clusterrole=cluster-admin --user=you@example.com
gcphcpctl cluster login my-cluster
kubectl --context my-cluster auth can-i create clusterrolebindings
```

Use actual context names/identities, preferring a user over a domain-wide grant. Verify
the intended operation through normal OIDC, not the bootstrap context; `auth whoami`
alone proves only authentication. Customer-managed issuers use the companion plan's
external-plugin workflow rather than the Google login example above.

`-o json` emits only the credential document, without kubeconfig mutation or stdout
guidance/progress. Document its sensitivity. Normal login does **not** erase the separate
bootstrap key: after verifying access, customers remove its context/auth entry and any
unreferenced dedicated cluster entry. Local cleanup is not revocation of copied keys.

## Alternatives and prerequisites

### Alternative: Gecko-derived guest administrator access

Gecko entitlement could grant guest admin through the existing `system:cluster-admins`
binding, removing routine manual binding creation for eligible users while retaining
independently authorized certificate recovery. This is an ongoing platform-derived
privilege policy, not merely a different first-login UX. No alternative is adopted here.

| Candidate | What changes | Main constraints |
|-----------|--------------|------------------|
| Reconciled claim mapping | Gecko computes eligible principals and publishes a platform-owned groups expression through HC transport. The guest derives groups from original verified issuer claims. | No token modification, new issuer, or guest RBAC writes; inspected 1,024-character limit and direct-OIDC API-server rollouts constrain size/churn. Mechanism verified; Gecko reconciler absent. |
| Token broker | Verify identity/Gecko entitlement, issue short-lived signed guest tokens, and configure guest issuer trust. | New login/exchange, signing-key custody/rotation/publication, availability, and token lifecycle; broker tokens need cluster-specific acceptance/isolation. |
| RBAC synchronization | Authorized component reconciles guest user bindings from applicable Gecko grants. | Guest-writing privileges, approved placement/connectivity, drift and removal handling; changes the baseline no-Gecko-guest-API-calls design. An OpenShift Group object alone does not bridge direct JWT membership. |

Standard Google tokens lack selected Google Groups membership (`hd` is a domain claim),
but Gecko-derived eligibility does not need it. Claim mapping cannot query live Cedar
or modify Google signatures; a reconciled eligible set can still drive local expressions.
It retains configured issuer trust and does not inherently require another provider slot.
Broker coexistence and customer identity association are covered by
[the OIDC plan](gcp-customer-managed-oidc.md#interaction-with-gecko-derived-guest-administrator-access).

Complete these gates before implementation/selection, rather than duplicating them in
a separate risks/backlog list:

| Gate | Decision or prerequisite still required |
|------|-----------------------------------------|
| Baseline API/data path | Computed-route/type/discovery/private-surface contract; read-only Firestore client versus authenticated internal service, including package/IAM/deployment boundaries |
| Serving/error policy | Minimum lifetime, clock skew, observation evidence, revocation fence/resumption, one typed retry contract, terminal/deletion errors and CLI timeout |
| Secret boundary | Approve/narrow all status readers and broader grants; payload index exclusions, effective/inherited Data Access audit, regional retention/backups/exports, and purge guarantees |
| Applier/runtime support | Secret delivery, rotation, watch interruption, removal/startup cleanup and purge latency; confirm deployed compatibility, not just source; assign dependency owners |
| Cutover/operations | Targeted all-generation Job retirement with restart-safe progress, supported API/CLI versions, safe rollback, authorized CRR owner/path and runbooks |
| Guest-admin policy | Exact entitlement/action and Cedar object/method context for a project and Cluster incarnation; role-name membership or an arbitrary permitted CRUD action is insufficient |
| Materialization/identity | Enumerate principals and use `AuthorizeWithContext`; `HasNamespacePermission` omits conditions. Preserve Gecko email Unicode/domain/local-part semantics or establish authoritative subject association; never join issuers by email alone. |
| Reconciler correctness | RoleBinding/Role/PlatformRole/relevant Cluster watches, concurrent publication, safely quoted generated values, size bounds and explicit oversized-policy behavior; no silent truncation or arbitrary customer CEL |
| Convergence/isolation | Measure rollout cost, batching/churn, policy replication/cache, stale snapshots, outages, authentication caches and active connections. Target-bound mapping sets/UID recreation must remain isolated. |
| Ownership/support | Release matrix, customer opt-out/default-binding reconciliation, guest audit identity, and customer-IdP compatibility. Keep customer-controlled reserved-group mappings disallowed under the existing proposals. |

The alternative gates are an evaluation track, not a delivery dependency for the
certificate baseline. A selection requires an explicit architecture/security decision.

## Security and lifecycle

- **Secret custody:** mirroring moves a guest superuser key outside the MC. Use Workload
  Identity and reviewed database-scoped IAM, but remember database scope does not isolate
  Secret documents from other status readers. Default encryption is insufficient; apply
  the indexing/retention/audit decisions above. Do not copy keys into the platform DB,
  Cluster status, conditions, events, or generic feedback. Keep status writes with applier.
- **MC access:** platform-api gains no direct MC Kubernetes access. Review actual applier
  RBAC and privileged readers; its chart is broad, and a field-selected watch is not a
  security boundary or proof that no other principal can read the Secret.
- **Audit/diagnostics:** public fetch audit is separate from private GKE and database
  Data Access audit. Record actor, project, Cluster name/UID, request ID, outcome,
  `credential.fetch`, and successful certificate fingerprint. Never capture response
  bodies, PEM, keys, raw validation errors, or tokens in API/proxy/CLI logs/traces.
  Restrict heap/pprof/debug access and monitor safe delivery/retirement failure metrics.
- **Shared credentials:** multiple callers may receive the same key and guest identity.
  Fetch fingerprints aid correlation, not unique attribution of guest actions. Revoking
  platform retrieval permission stops future exports, not existing downloaded access.
- **Certificate containment:** an authorized `CertificateRevocationRequest` with
  `signerClass: customer-break-glass` in the HCP namespace rotates the signer and removes
  old trust asynchronously. It affects all customer-class certificates; wait for verified
  completion and safe delivery resumption. CRR avoids CSR admission, but its GKE recovery
  procedure still needs testing. Investigate and remove attacker-created RBAC, credentials,
  and other persistence separately; certificate revocation is not full incident recovery.
- **Alternative revocation:** reconciled mapping can remove the group from the same
  unexpired token after convergence, but guests may retain a published mapping during
  Gecko/store outages. Do not promise live Cedar checks, instantaneous revocation, or a
  fixed bound without evidence. Broker tokens and RBAC synchronization have different
  expiry/removal delays. Brokers must re-evaluate entitlement on issuance/refresh;
  removal stops new tokens only after policy convergence. No method undoes changes
  already made as guest admin.
- **Alternative trust:** bind verified claims to the exact entitlement decision. For
  mapping, derive membership at each target from its own issuer-bound set; a shared
  Google audience does not itself share derived groups. Broker-issued admin claims need
  cluster-specific token acceptance and separation from platform API tokens. Specify
  fail-closed publication/stale-grant handling, not an assumed always-live policy lookup.
- **Recovery/ownership:** the baseline customer manages guest RBAC, while platform
  admins with export permission retain mediated recovery power. IdP, PKI, platform and
  transport outages can delay recovery. Alternative ongoing entitlement and durable
  customer opt-out require an explicit policy, including default-binding reconciliation.

## Implementation phases

1. **Contracts and gates — gecko/infrastructure:** resolve baseline gates above before
   build-out; track alternative selection separately. Design generation/discovery,
   transport read results, safe errors, IAM and operational ownership.
2. **Additive backend — gecko/applier:** retain the Job initially. Add `EnsureRead`, typed
   extraction, `RemoveRead`, mocks/emulator coverage, UID-bound subscriptions, advisory
   readiness and deletion cleanup. Add the computed endpoint, permission/action/role
   wiring, serving validation, audit/no-store and bounded retrieval controls. Resolve
   runtime lifecycle/freshness failures without adding Gecko status-write permissions.
3. **CLI and pre-cutover validation — gcp-hcp-ctl:** add the typed client, isolated writer,
   bounded waiting, JSON-only mode, creation guidance and local-key cleanup instructions.
   Publish supported versions and rehearse recovery. Use the test matrix below; distinguish
   access from the new grant from legacy domain/creator grants in disposable/canary tests.
4. **Staged cutover — gecko:** remove `buildRBACJob` and its obsolete tests only after
   API/CLI, security, recovery and cleanup gates pass. Remove unused manifest `Input.CreatedBy`,
   preserving creator audit metadata. Remaining manifest count varies with ingress.
   Idempotently retire all platform-owned `rbac-setup-gen-*` Jobs and Apply/ReadDesires
   for the exact Cluster/HCP namespace, with retries and restart-safe progress. Stop
   desired-state recreation before deleting Jobs; never delete the whole HC apply group.
   Preserve Namespace/HC/Certificates/other resources. Test TTL, restarts and rollback
   so older components cannot silently restore the domain-wide grant.
5. **Existing clusters and operations:** keep existing `redhat-domain-admins` bindings.
   Document customer-led migration: establish intended OIDC access, verify it, remove
   the legacy binding, verify again. Publish CRR, persistence-remediation, local cleanup
   and rollback runbooks; alert on sustained refresh or retirement failures without secrets.

## Acceptance and verification

### Claim-mapping verification in a development environment

Public evidence is intentionally abstract: no cluster/project/account identifiers,
resource UIDs, internal endpoints, deployment metadata, token subjects, or raw credentials.

| Evidence | Established | Not established |
|----------|-------------|-----------------|
| Certificate PoC/read-only inspection | Required PKI resources/default timing, `system:masters` identity/client usage, CA-bundle configuration, public endpoint TLS/system-root trust and certificate recovery; namespace/placement matched transport expectations | Full supported-release matrix; freshness/revocation guarantees |
| Applier source inspection | Pinned source supports standalone list/watch with cached reconciliation; inspected deployment's corresponding source adds orphan-status deletion and startup cleanup | Independent binary provenance or runtime standalone Secret rotation/purge |
| Local code/tests | Default schema/feature set accepts group expressions with 1,024-character bound; generated config and signed synthetic-token positive/negative tests passed, including wrong issuer/audience, expiry, verification, removal and preservation of normal groups; config changes alter pod-template hash; Gecko authn/authz tests passed | Live Google behavior from synthetic tests alone; Gecko entitlement reconciler |
| Controlled live mapping test | Existing development guest's admin binding and group-only authorization verified. An approved desired-state change selected one verified Google subject, preserving issuer/audience/username/normal groups; original token gained the group, and restoration removed it from the same valid token without guest RBAC writes. | Full loss of admin: the identity had another grant. No issuer replacement, automatic Gecko decisions, high-churn/scalability or comprehensive cache/connection proof. |
| Restoration | Original desired/generated auth, healthy API replicas, HC controller and GitOps self-healing independently rechecked; temporary recovery key files removed. Rollouts took several minutes in each direction. | An SLO, instantaneous updates, or approval to perform such tests on customer clusters |

Earlier read-only inspection and the separately approved live test are different
exercises. No fresh privileged-CSR admission or CRR test was performed. Credential
endpoint/CLI and standalone Secret-delivery acceptance remain open; neither code nor
mechanism-level verification means those features are implemented.

### Test matrix

| Area | Required acceptance |
|------|---------------------|
| Authorization | Dedicated-grant admin allowed; viewer/no-grant/non-member/unauthenticated/cross-project denied before secret reads. Parent conditions, flag-off, errors and every route covered. |
| API | Computed singleton, correct generated response/discovery/errors, no accidental store/CRUD/list/watch/alternate-route bypass or shared cache. |
| Serving | Missing/failed/wrong-target/old-incarnation reads; malformed/mismatched PEM, wrong identity/usage, future/expired/near-expiry, stale and known-revoked material never returned as usable. |
| Lifecycle | Standalone read without apply, rotation, watch/DB outage/restart, partial/full deletion, orphan purge latency and same-name recreation behave safely without deleting the source Secret. |
| CLI | Distinct context/current-context unchanged, exec/unrelated entries preserved, safe collisions, owner-only files, bounded retry/cancel/timeout, output-only JSON, safe diagnostics at supported verbosity. |
| Baseline e2e | No-Job cluster → fetch → SSR confirms certificate identity → customer binding → normal OIDC authenticates **and authorizes** the intended operation. Isolate legacy-grant effects. |
| Cutover | All prior Job generations/desires retired without affecting other resources or auto-removing guest grants; TTL/restarts/version skew/rollback do not resurrect the Job/grant. |
| Revocation/incident | GKE CRR completion rejects old certificate and permits new recovery; stale delivery blocked/resumed safely; persistent-access investigation/remediation documented. |
| Secret boundary | Effective reader grants, payload index exclusions, inherited Data Access audit, retention/backups/purge reviewed and demonstrated; captured API/proxy/CLI logs/traces contain no credential bodies. |
| Alternative selection | Support matrix, actual condition-aware entitlement/principal association/reconciliation, oversize/injection/concurrent writes, target isolation, churn/rollout and outage/cache/connection removal behavior verified before selection. |

Run focused and owning-module suites, Firestore emulator tests, API generation/lint
checks where applicable, and authorized development/canary e2e. Document skipped checks.

## References

- [OpenShift bootstrap policy](https://github.com/openshift/openshift-apiserver/blob/master/pkg/bootstrappolicy/policy.go)
  and [constants](https://github.com/openshift/openshift-apiserver/blob/master/pkg/bootstrappolicy/constants.go):
  `cluster-admins` / `system:cluster-admins`; verify supported versions independently.
- HyperShift: `control-plane-operator/controllers/hostedcontrolplane/v2/kas/auth.go`,
  `control-plane-operator/featuregates/featuregates.go`, generated default HostedCluster
  CRD under `cmd/install/assets/crds/hypershift-operator/zz_generated.crd-manifests/`,
  and `support/controlplane-component/defaults.go` (mapping, validation, limit/rollout).
- HyperShift PKI: `docs/content/how-to/pki/break-glass-credentials.md`,
  `control-plane-pki-operator/{operator.go,certrotationcontroller,certificatesigningcontroller,certificatesigningrequestapprovalcontroller}`;
  `test/integration/control_plane_pki_operator.go` (`validateCSRFlow` and GCP skip).
- Gecko: `controllers/hc/{hc_controller.go,manifest/manifests.go}`,
  `controllers/client/transport/firestore/{client,status,desires}.go`,
  `platform-api/pkg/authz/{middleware,permissions,authorizer,context,entities,reload,list_authorization}.go`,
  `platform-api/api/private/v1/authorization_validation.go`, `controllers/cmd/hc/cmd.go`,
  and `orlop/pkg/apiserver/{router,router_helper}.go`.
- Applier: Gecko's pinned `a0a17126d22a` source and reviewed public
  [read manager](https://github.com/openshift-online/kube-applier-gcp/blob/ad75b07/pkg/controllers/read_desire_manager/controller.go),
  [reflector](https://github.com/openshift-online/kube-applier-gcp/blob/ad75b07/pkg/controllers/read_desire_kubernetes/controller.go),
  [cleanup](https://github.com/openshift-online/kube-applier-gcp/blob/ad75b07/pkg/controllers/desirestatuscleanup/desirestatuscleanup.go).
- CLI: `pkg/platformapi/{client,errors}.go`, `pkg/cluster/login.go`,
  `pkg/kubeconfig/kubeconfig.go`, `pkg/httpdebug/{payload,redaction}.go`.
- [Customer-managed OIDC](gcp-customer-managed-oidc.md) owns issuer replacement/client
  behavior; [earlier authentication design](gcp-customer-authentication.md) is historical
  context, not evidence that its Identity Platform/IAM-role proposals are deployed.
