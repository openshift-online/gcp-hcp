# Customer Bootstrap Access

> **Status**: Draft

**Jira**: TBD

| Field | Value |
|-------|-------|
| **Identifier** | `customer-bootstrap-access` |
| **Components** | gecko `controllers/hc`, gecko `platform-api`, `gcp-hcp-ctl` |
| **Depends on** | HyperShift `control-plane-pki-operator` (OCP 4.15+; prerequisites validated in a development environment); kube-applier-gcp Secret-delivery and cleanup acceptance tests |
| **Replaces** | RBAC setup Job in gecko `controllers/hc/manifest/manifests.go` (`buildRBACJob`) |
| **Related** | [gcp-customer-authentication.md](gcp-customer-authentication.md) — resolves its "No Fallback Access" risk item |
| **Complementary plan** | [Customer-managed OIDC authentication](gcp-customer-managed-oidc.md) — optional guest IdP configuration and replacement; platform/bootstrap authentication remains independent |

## Overview

Today a customer's first access to their hosted cluster is bootstrapped by a platform-side
Job that runs in the HostedControlPlane namespace, mounts HyperShift's
`service-network-admin-kubeconfig` (a `system:masters` admin kubeconfig), waits for the
guest API to become healthy, and applies a `ClusterRoleBinding` granting `cluster-admin`
to the whole `redhat.com` group (the Google `hd` claim) plus the creating user.

This plan removes that Job. Instead, the platform hands the customer HyperShift's
**pre-generated customer admin client certificate** (a short-lived,
`system:masters` client cert that the `control-plane-pki-operator` continuously rotates
in the HostedControlPlane namespace). The customer initially uses it to create whatever
ClusterRoleBindings they want for their own OIDC identities, then continues accessing the
cluster through normal Google OIDC exec-auth.

The feature is **customer bootstrap access**, with repeatable credential retrieval for
recovery. It reuses HyperShift's upstream "break-glass" credential mechanism; that
upstream term is not the feature name. Upstream signer names, certificate subjects,
and documentation paths retain their actual names below.

After cutover, **this bootstrap/recovery flow makes no guest API calls from gecko**.
The customer, using their CLI, makes guest API calls. Gecko still handles guest
superuser credential material; removing the Job does not remove that trust boundary.

**Key Design Decisions:**

- Use the **pre-generated** customer admin certificate (`customer-system-admin-client-cert-key`
  Secret in the HCP namespace), not HyperShift's existing **management-cluster CSR flow**
  for these privileged certificates. Upstream records GKE Warden rejection of the
  requested `system:`-prefixed CNs and `system:masters` organization on that path. This
  does not mean all CSRs, or guest-cluster CSRs, are blocked by GKE. The pre-generated
  path avoids CSR admission and was validated in a development environment. See
  [Why not the CSR flow](#why-not-the-csr-flow).
- The OIDC authentication configuration on the HostedCluster is **unchanged**.
  `claimMappings` produce identity (`<email>` user, workspace-domain group); they carry no
  permissions themselves. In the baseline proposal, guest RBAC is customer-owned after
  first access. Mapping to an already-bound group is a distinct possibility; see
  [Why not OIDC claim mappings alone](#why-not-oidc-claim-mappings-alone).
  Customer-managed guest issuers are covered separately by
  [customer-managed OIDC authentication](gcp-customer-managed-oidc.md). That plan reuses
  this bootstrap/recovery path but replaces the normal guest Google login workflow;
  its independently configured platform authentication does not change.
- A computed, non-persisted Cluster subresource serves the credential:
  `GET /apis/gcp.managed.openshift.io/v1/namespaces/{project}/clusters/{name}/credential`.
  There is no credential CRUD resource, collection, list, or watch endpoint.
- Retrieval requires the dedicated `cluster.credential.get` permission, granted to the
  platform `cluster-admin` role, not `cluster-viewer`. Authentication and authorization
  remain mandatory even if the general public-authorization feature flag is disabled.
- Retrieval is **repeatable**, including for existing clusters and lockout recovery,
  subject to authorization and a valid credential being available. It is not a one-time
  capability, and availability depends on the PKI operator and delivery path.
- Continuously mirror the Secret into the MC status Firestore DB, subject to explicit
  IAM, retention, indexing, diagnostic-redaction, and cleanup safeguards.
- Roll out additively before retiring the RBAC Job and its existing desires. Preserve
  existing guest RBAC grants; customers remove them after establishing their own access.
- The CLI writes a distinct bootstrap context without changing the current context.
  Initial bootstrap uses a manual RBAC walkthrough; `--bind-admin` is deferred.
- [Gecko-derived guest administrator access](#alternative-gecko-derived-guest-administrator-access)
  is an alternative to evaluate, not an adopted replacement for this baseline. It could
  remove routine manual RBAC bootstrap for eligible administrators while retaining
  certificate-based recovery, but requires an identity bridge and a separate trust decision.
- SRE admin credentials (`sre-system-admin-client-cert-key`) are **out of scope** for this plan;
  the same mechanics can be reused later.

## Background

### HyperShift customer admin credentials

The `control-plane-pki-operator` runs once per HostedControlPlane (unconditionally, since
OCP 4.15) and maintains two upstream signer classes: `customer-break-glass` and
`sre-break-glass`. For the customer class it continuously rotates:

| Resource (HCP namespace) | Rotation |
|--------------------------|----------|
| `customer-system-admin-signer` (CA Secret) | 7d validity, refreshed every 2d |
| `customer-system-admin-signer-ca` (trust ConfigMap) | tracks the CA |
| `customer-system-admin-client-cert-key` (client cert Secret, `kubernetes.io/tls`) | **36h validity, refreshed every 6h** |

The client cert has subject `O=system:masters, CN=system:customer-break-glass:<random>`
and is trusted by the hosted kube-apiserver through the `total-client-ca` bundle, so it
authenticates against the **public** API route with full `system:masters` power. Rotation
timings scale with `CERT_ROTATION_SCALE` (default 24h) and are owned by HyperShift; gecko
does not tune them.

These timings are operational expectations, not a credential-serving guarantee. Under
healthy rotation, a leaf can already be almost 6h old when read; serving that snapshot
6h later can leave approximately **24h**, not 30h, of validity. Operator or transport
failures invalidate even that estimate. The API must parse and validate each fetched
certificate/key pair, enforce a minimum remaining lifetime, and handle revocation and
delivery freshness separately from certificate expiry.

`system:masters` bypasses Kubernetes authorization; it is stronger than an ordinary
`cluster-admin` RBAC grant. Multiple fetches can return the same certificate/key and
guest audit identity. Removing a caller's platform permission stops future retrievals,
but does not invalidate a downloaded certificate. Normal leaf rotation also does not
immediately revoke previously issued certificates.

### Why not the CSR flow

HyperShift's existing CSR flow for these customer admin certificates uses the
**management-cluster APIs**, even though the resulting certificate authenticates to
the **guest API**. The certificate's destination does not determine where its signing
request is stored:

| Resource or operation | Location |
|-----------------------|----------|
| Kubernetes `CertificateSigningRequest` | Management cluster, cluster-scoped |
| HyperShift `CertificateSigningRequestApproval` | Management cluster, HCP namespace |
| Approval/signing and signed CSR status | Management-cluster CSR, processed by `control-plane-pki-operator` |
| Authentication using the issued certificate | Guest API, through its configured client-CA trust |

```text
Management-cluster CSR + HCP-namespaced approval
  -> PKI operator approves/signs and updates the management-cluster CSR
  -> requester retrieves the signed certificate
  -> certificate authenticates to the guest API
```

This placement is explicit in upstream `test/integration/control_plane_pki_operator.go`:
`validateCSRFlow` creates and reads the CSR through `mgmt.KubeClient`, and creates its
approval through `mgmt.HyperShiftClient`. The signing controller watches CSRs through
the management-cluster informer and writes their certificate status; it does not
forward the request into the guest cluster.

Because the request goes through GKE management-cluster admission, it encounters GKE
Warden. Upstream's `RunTestControlPlanePKIOperatorBreakGlassCredentials` records a GCP
skip because Warden rejects the privileged certificate subjects (`system:`-prefixed
CNs and `system:masters` organization). Thus the limitation is **this existing
management-cluster CSR flow for privileged customer admin certificates**, not a blanket
GKE prohibition on CSRs. Rejection was not freshly tested during development validation;
the admission restriction remains based on this upstream evidence.

Creating a CSR against the guest API would be a **different signing/approval path**.
The existing HyperShift customer admin signer does not watch the guest CSR API, so moving
the object there would not cause this controller to approve or sign it. Submitting and
approving guest CSRs also requires guest permissions; that path does not independently
solve first access. Supporting it would require separate design and validation, not a
change of target endpoint alone.

The chosen pre-generated Secret path creates no CSR object, avoids this admission
restriction, and was validated in a development environment.

### Why not OIDC claim mappings alone

OIDC `claimMappings` supply **identity**, not new RoleBindings or ClusterRoleBindings.
The current configuration maps the Google `email` claim to a username and the `hd` claim
to a workspace-domain group. Those identities receive any permissions already granted
to them by guest RBAC. Thus claim mapping can provide administrator access **when it
targets an already-bound group**; it does not inherently require creating a new binding.

For ordinary OIDC identities, guest RBAC must already grant the required permissions.
Mapping a claim to a group named `cluster-admin` does **not** grant the `cluster-admin`
ClusterRole: a binding must explicitly connect that group to the role. OpenShift's
upstream bootstrap policy already defines a different, prefixed admin group:

| Identity | Upstream default behavior |
|----------|---------------------------|
| Group `cluster-admin` | No special name-based admin grant |
| Group `system:cluster-admins` | Bound to ClusterRole `cluster-admin` by ClusterRoleBinding `cluster-admins` |
| Group `system:masters` | Special Kubernetes authorization bypass, not equivalent to an ordinary RBAC grant |

Upstream `openshift-apiserver` bootstrap policy defines the `cluster-admins` binding
and its `system:cluster-admins` group constant. Verify that binding's presence on supported
guest releases and validate whether their OIDC configuration accepts the intended group
mapping. These are source findings, not a newly performed guest mapping/login test.
If accepted, a trusted mapping into that existing group would not need another binding.

The baseline and customer-managed OIDC proposal intentionally do not expose arbitrary
customer mappings into reserved privileged groups. In particular, `system:masters`
would bypass authorization and is not an acceptable ordinary OIDC admin mechanism.
Do not confuse this policy restriction with evidence that every `system:` group mapping
is rejected by upstream. A platform-owned bridge targeting `system:cluster-admins`
requires separate support/security validation and would use the existing RBAC binding,
not the `system:masters` bypass.

Customer bootstrap credentials provide the authorized identity needed to create the
initial customer-chosen binding. The customer then uses normal OIDC authentication
with that binding. OIDC configuration remains unchanged in the baseline. Mapping alone
neither invents Google Group membership nor evaluates Gecko authorization. Standard
Google ID tokens do not supply Google Groups membership, and `hd` is a domain claim,
not membership in a selected Google Group. The Gecko-derived alternative below would
use platform authorization instead of requiring that Google membership information.

#### Example 1: mapping claims supplies an identity, not an admin grant

Given these illustrative Google token claims:

```json
{
  "email": "alice@example.com",
  "hd": "example.com"
}
```

The existing HostedCluster OIDC provider uses this mapping fragment:

```yaml
claimMappings:
  username:
    claim: email
  groups:
    claim: hd
    prefix: ""
```

This supplies username `alice@example.com` and workspace group `example.com` (alongside
standard authenticated groups). Neither the email nor domain name inherently grants
permission to create a ClusterRoleBinding. Without a relevant guest RBAC grant,
authentication can succeed while this authorization check returns `no`:

```shell
kubectl --context my-cluster auth can-i create clusterrolebindings
```

Here `my-cluster` is the normal OIDC context. The example assumes no existing binding
grants that identity the operation; existing legacy domain/creator grants would mask
the bootstrap problem.

#### Example 2: a group named `cluster-admin` is still only a group

Suppose a different OIDC provider issued a custom claim `roles: ["cluster-admin"]`,
and its configured mapping produced a group named `cluster-admin`. This is hypothetical:
the current Google mapping uses `hd`, not such a custom roles claim.

That group would need an explicit binding such as:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: oidc-admins
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: cluster-admin
```

The matching strings do not create this relationship automatically. The binding must
already exist or be created by an authorized identity. This illustrates the dependency,
not a recommendation to change the current claim mapping or grant an entire domain.

#### Example 3: bootstrap access creates the initial user grant

After fetching the bootstrap credential, Alice explicitly uses its separate context
to create a binding for her OIDC username, then checks access using normal OIDC:

```shell
kubectl --context my-project-my-cluster-bootstrap create clusterrolebinding alice-admin \
  --clusterrole=cluster-admin --user=alice@example.com

kubectl --context my-cluster auth can-i create clusterrolebindings
# Expected after the binding is observed: yes
```

Only the first command uses the bootstrap certificate. The second uses the unchanged
OIDC identity mapping and the newly created guest RBAC grant. The grant is to Alice's
user, not the whole `example.com` group. Replace the example context names and email
with the actual values; do not test the second command with the bootstrap context.

### Alternative: Gecko-derived guest administrator access

**Status: evaluation only.** The certificate/manual-binding flow remains the baseline.
This alternative considers a deliberate policy that an applicable Gecko administrator
entitlement for a specific cluster also grants guest `cluster-admin` access. It could
remove the routine initial-binding step for those users, while retaining bootstrap
certificates for independently authorized recovery and customer RBAC administration.

Gecko would supply the entitlement decision, so this does **not** require Google Groups
membership. It does require a trusted mechanism to carry that decision into guest
authentication or guest RBAC. A Google-signed ID token cannot be modified to add Gecko
claims, and claim mapping cannot query Cedar by itself.

An illustrative broker path is:

```text
User authenticates through an explicitly supported identity flow
  -> Gecko evaluates guest-admin entitlement for the requested project/cluster
  -> trusted broker issues a short-lived, cluster-bound guest token
  -> supported guest authentication maps the entitlement to system:cluster-admins
  -> OpenShift's existing cluster-admins binding grants cluster-admin
```

The group mapping and binding must be verified on the support matrix; this is not a
claim that the path is implemented or accepted by current OIDC validation. If that group
cannot be used, a custom issuer-prefixed group would require its own initial binding,
so the design could no longer assume the same binding-free bootstrap benefit.

#### Candidate mechanisms

| Mechanism | Required work and architectural implications |
|-----------|----------------------------------------------|
| Token broker | Verify user identity, evaluate scoped Gecko entitlement, issue signed short-lived guest tokens, and configure guest trust/audiences/mappings. This adds a security-critical issuer and token lifecycle, not a claim-mapping-only change. |
| RBAC synchronization | Reconcile applicable Gecko grants into guest user bindings, including removal and drift handling. Requires an authorized guest-writing component and an explicit departure from the baseline's no-Gecko-guest-API-calls design, with approved placement/connectivity. |

For synchronization, creating an OpenShift Group object alone does not cause a direct
OIDC token authenticator to consult it for JWT group membership. Specify the actual
identity/binding mechanism, such as user bindings, instead of assuming that a group
object bridges the authentication boundary.

#### Entitlement and security decisions

- **Exact policy:** choose which Gecko grant/action means guest-admin entitlement, or
  define an intentional role-to-guest policy. No new permission is adopted here. A role
  called `cluster-admin` somewhere in Gecko is not sufficient: evaluate the requested
  project, exact Cluster incarnation, and relevant Cedar conditions. Define the action,
  method, and object context rather than substituting an arbitrary allowed CRUD action.
- **Token isolation:** broker tokens must be unusable at other guest clusters, using
  cluster-specific validated audiences or equivalent isolation. A common issuer with
  an admin group and shared audience across all clusters is insufficient. Separate guest
  token acceptance from platform API token acceptance to prevent confused-deputy use.
- **Revocation and consistency:** evaluate entitlement at issuance/refresh, with defined
  policy-cache/replication behavior, token lifetime, and failure handling. Grant removal
  stops new token issuance only after policy convergence; existing tokens may remain
  usable until expiry. Synchronization has its own reconciliation/removal delay. Neither
  approach automatically undoes RBAC or other changes a guest administrator already made.
- **Identity and signing:** bind the verified principal to the entitlement check and
  signed subject. Decide supported login/token-exchange flows, signing-key custody,
  rotation, publication, and emergency containment. A broker can assert guest identities
  and is therefore part of the guest administrative trust boundary.
- **Availability:** fail closed when identity/entitlement cannot be established. Document
  IdP, broker, policy-store/cache, and synchronization outages; preserve an independently
  authorized bootstrap recovery path without promising offline recovery.
- **Audit:** correlate verified principal, target Cluster, entitlement decision/revision,
  issuance or reconciliation outcome, and guest identity using safe metadata. Never log
  bearer tokens, signing keys, or raw identity claims. Define guest audit attribution.
- **Ownership:** decide explicitly whether platform administrators retain guest-admin
  access independently of customer-owned guest RBAC preferences, and whether customers
  can opt out. In the broker variant, access still depends on the guest binding, but
  platform-issued membership is an ongoing source of privilege while that binding exists.
- **Issuer compatibility:** account for the one-provider constraint and
  [customer-managed OIDC](gcp-customer-managed-oidc.md#interaction-with-gecko-derived-guest-administrator-access).
  A broker is not automatically an additional provider alongside Google/customer OIDC.

These are comparison gates, not implementation requirements silently added to the
baseline. Do not expose reserved-group mapping to arbitrary customer-controlled issuers
or equate `system:cluster-admins` with `system:masters`. Selection requires an explicit
architecture/security decision and revised implementation scope.

### PoC validation in a development environment

Historical PoC evidence recorded before the read-only validation below. These guest
authentication/authorization checks were not repeated during that later inspection:

- `control-plane-pki-operator` running; `customer-system-admin-client-cert-key` present
  with a valid `O=system:masters` cert and validity consistent with upstream defaults.
- A kubeconfig built from **only** the public API URL
  (`api.<clusterName>-<slug>.<baseDomain>`) plus `tls.crt`/`tls.key` authenticates with
  **no CA data** (the route serves the public cert-manager certificate, so system roots
  suffice).
- `SelfSubjectReview` returns `system:masters` + `system:authenticated`; `auth can-i
  create clusterrolebindings` → `yes`.
- The old flow's `redhat-domain-admins` CRB was visible in the guest, i.e. the
  customer-side replacement is a 1:1 mapping of what the Job created today.

### Development-environment validation (read-only)

Read-only inspection in a development environment validated prerequisites, not the
proposed feature end to end. No private key was exported, and no cluster/Firestore
resources, CSR, or CRR were created, modified, or deleted. This public plan records
verification facts only: omit deployment identifiers, project/account details, image
locations, certificate identifiers/timestamps, and other environment-specific evidence.

| Item | Observed state |
|------|----------------|
| Management cluster | Validated on GKE Autopilot in a development environment |
| Hosted cluster | Available on a release with the required PKI functionality |
| Platform identity and HCP namespace | UID/internal-name derivation and management-cluster placement matched the expected model |
| PKI operator | Ready with the default certificate rotation scale |
| Customer certificate | Expected Secret, organization, upstream username prefix, client-auth usage, and approximately 36h validity confirmed |
| Trust configuration | Certificate issuer represented in both `customer-system-admin-signer-ca` and `total-client-ca`; this is configuration evidence, not a new client-authentication test |
| Public API TLS | System-root and hostname verification succeeded without custom CA data |
| Effective OIDC config | Generated HCP `auth-config` maps Google `email` to username, `hd` to groups, and `sub` to UID; username/group prefixes are empty |
| Firestore placement | `specs`/`status` database placement and applier configuration matched the transport model; backup/retention review remains a separate gate |
| Applier deployment | Ready; corresponding source supports standalone reads and status cleanup, with runtime Secret lifecycle tests still pending |
| Existing desires | Observed reads were paired with applies; standalone customer credential delivery was not exercised |
| Legacy bootstrap | Multiple Job generations and their desires were observed, confirming all-generation retirement is needed; no recreation history was established |
| Platform roles | Deployed `cluster-admin` and `cluster-viewer` do not yet include the proposed `cluster.credential.get` permission |

**Applier evidence levels:**

1. **Pinned dependency source:** Gecko's `a0a17126d22a` dependency supports standalone
   ReadDesires with per-object list/watch and 60-second cached reconciliation.
2. **Source corresponding to the inspected deployment:** also implements read-status
   deletion when its spec disappears and startup orphan reconciliation. The reader is
   stopped before status deletion. This inspection establishes source support for the
   inspected deployment, not independently verified binary provenance or runtime cleanup.
3. **Runtime acceptance pending:** no standalone Secret subscription exists in this
   snapshot. Secret delivery, rotation propagation, watch interruption, subscription
   removal, mirrored-key purge, and restart behavior have not been exercised.

**Security conclusions from development validation:**

- Database-scoped IAM does not isolate Secret documents from other status readers.
  Inventory controller and broader grants, then explicitly approve or narrow the reader
  boundary before enabling mirroring. Do not publish environment IAM bindings here.
- Verify effective payload/descendant indexing and implement necessary secret-payload
  exclusions before mirroring; do not assume existing status storage is suitable for keys.
- Verify effective Firestore/Datastore Data Access audit policy, including inherited
  settings, required safe coverage, and retention. Project-policy inspection alone does
  not prove effective logging is enabled or disabled.

**Not revalidated by this inspection:** certificate/private-key matching and guest
authentication; guest RBAC changes or authorization checks; CRR revocation/recovery;
fresh admission rejection of privileged CSR subjects; and the planned credential
endpoint, dedicated authorization, and CLI.
The CSR rationale remains based on the cited upstream evidence. Retain all corresponding
acceptance tests and do not treat the historical PoC as a newly repeated test.

## Current State (what we remove)

`controllers/hc/manifest/manifests.go` builds a base set of five manifests: Namespace,
ExternalSecret (pull secret), Certificate (API TLS), HostedCluster, and the RBAC setup
Job. An ingress Certificate can be added conditionally. The Job
(`rbac-setup-gen-<generation>`, HCP namespace `clusters-{clusterUID}-{safeName}`):

- mounts `service-network-admin-kubeconfig` (HyperShift's admin kubeconfig for the guest API),
- polls the guest `/healthz` for up to 25 minutes,
- applies `ClusterRoleBinding redhat-domain-admins`: `cluster-admin` → Group `redhat.com`
  (Google `hd` claim) and User `{CreatedBy}`.

Problems: the platform uses `system:masters` credentials against the guest API; the
default grant is domain-wide; the Job is a point-in-time hack with retry/backoff logic;
nothing about it is customer-visible or customer-owned.

Transport `Apply` adds/updates desires but does not prune omitted manifests. Removing
the Job from the builder alone therefore leaves existing Job ApplyDesires behind;
targeted retirement of those desires and Jobs is required during cutover.

Development validation found multiple generations represented as Jobs and desires.
Cover all-generation targeted retirement in migration tests; a snapshot alone does not
establish how often a Job was recreated.

## Target Design

### End-to-end flow

```mermaid
sequenceDiagram
    participant User
    participant CLI as gcp-hcp-ctl
    participant API as platform-api
    participant HC as gecko hc-controller
    participant FS as Firestore (specs/status DBs)
    participant KA as kube-applier-gcp (on MC)
    participant HCP as HCP namespace (MC)
    participant KAS as Hosted kube-apiserver

    Note over User,HCP: Cluster creation (no RBAC Job anymore)
    User->>CLI: gcphcpctl cluster create ...
    CLI->>API: POST clusters
    HC->>FS: ApplyDesires (base resources plus optional ingress certificate)
    HC->>FS: ReadDesire: secret/customer-system-admin-client-cert-key
    KA->>HCP: applies manifests, list/watches customer admin Secret
    KA->>FS: status: HostedCluster conditions + secret content

    Note over User,KAS: First access (repeatable)
    User->>CLI: gcphcpctl cluster credential my-cluster
    CLI->>API: GET .../namespaces/{project}/clusters/{name}/credential
    API->>API: authorize cluster.credential.get against parent Cluster
    API->>FS: read latest ReadDesire status (secret content)
    API->>API: validate target, read status, certificate/key and serving policy
    API-->>CLI: 200 { apiEndpoint, clientCertificate, clientKey }
    CLI->>CLI: write distinct bootstrap context, preserve current context
    CLI->>KAS: (user) create ClusterRoleBinding for own identity
    User->>CLI: gcphcpctl cluster login my-cluster (existing exec-auth)

    Note over User,KAS: leaf rotation does not revoke downloaded certs<br/>Clean up the local context and key
```

### What changes, by component

#### 1. gecko hc-controller (`controllers/hc`)

**Retire the RBAC Job only after additive rollout and validation.** `buildRBACJob` and
the Job manifest are removed from `manifests.go`; the remaining count depends on the
optional ingress Certificate. The `CreatedBy` input field has
no remaining consumer in the manifest builder — keep it in `Input` only if another
consumer exists, otherwise remove it (it remains available in the Cluster CR for audit
and future use).

**Maintain a credential ReadDesire.** Once the Cluster is placed and the HostedCluster
Desires are applied, the controller ensures a standalone ReadDesire exists for:

```
group/version/resource:    /v1/secrets
namespace/name:            clusters-{clusterUID}-{safeName}/customer-system-admin-client-cert-key
groupKey:                  {clusterUID}-credential   (dedicated; not mixed into the HC apply group)
```

Use `cluster.UID` and the controller's existing `spec.safeName`/defaulting logic, not
the customer-facing Cluster name, to derive the HCP namespace. The normal HC apply
group remains `projects/{namespace}/clusters/{name}`; the credential read group is
separate and bound to this Cluster incarnation.

kube-applier-gcp list/watches the Secret on the MC and writes its content into the
status DB. The controller surfaces a `BootstrapCredentialReady` condition based on
successful read status and certificate validation, with observed validity metadata
only. Never put the certificate/key in Cluster status, conditions, events, or logs.
The condition is advisory: the API independently validates the credential at fetch
time rather than trusting an old readiness condition.

**Cleanup.** Remove the credential read subscription during Cluster deletion, including
partial provisioning, and verify eventual removal of its mirrored status/key material.
Removing a read subscription must not create a DeleteDesire for the HyperShift-owned
Secret. Reject retrieval for a deleting Cluster and prevent reuse of an old Cluster's
credential after deletion/recreation.

**No guest API access.** The controller never constructs a kubeconfig for the hosted
cluster and never touches the guest API.

#### 2. gecko transport (`controllers/client/transport`)

The Firestore transport currently creates ReadDesires only as a side effect of `Apply`
(for gecko-owned manifests). Add read-only subscription capabilities (illustrative
signatures; finalize typed results during Phase 0):

```go
// Client abstracts the transport layer for delivering resources to management clusters.
type Client interface {
    // ... existing methods ...

    // EnsureRead creates (idempotently) ReadDesires for the given manifests without
    // creating ApplyDesires, and returns the current status read back for them.
    // Used for resources owned by other actors (e.g. HyperShift-created Secrets).
    EnsureRead(ctx context.Context, targetCluster, groupKey string, manifests [][]byte) (*Status, error)

    // RemoveRead removes read subscriptions only, never their target resources.
    // Mirrored status cleanup is owned by kube-applier-gcp and must be verified.
    RemoveRead(ctx context.Context, targetCluster, groupKey string) error
}
```

Implementation notes:

- Reuses `buildReadDesireDoc`; no `applydesires` documents are written.
- Create subscriptions only when absent or materially changed. Reconciliation must not
  rewrite unchanged specs and churn Firestore update times or read acknowledgements.
- Needs `kindToResource` to already map `Secret` → `secrets` (it does).
- **Pinned dependency source verified:** Gecko pins kube-applier-gcp
  `v0.0.0-20260903184437-a0a17126d22a`. Its ReadDesire manager independently starts
  per-object list/watch controllers, with 60-second reconciliation of the informer
  cache. No ApplyDesire is required. Source corresponding to the inspected development
  deployment also supports this path and orphan-status cleanup/startup reconciliation.
  Runtime standalone Secret delivery and cleanup remain pending; see
  [development-environment validation](#development-environment-validation-read-only).
  Watch events, rather than a nominal periodic re-read, propagate Secret rotation.
- Existing `extractResourceStatuses` handles HostedCluster, NodePool, and Certificate
  fields, not Secret data. Add explicit typed credential extraction/validation and safe
  readiness metadata; do not route private keys through generic string feedback fields.
- Existing `Status.Stale` checks desired-spec acknowledgement, not the age of an actual
  MC observation. Define the observation/freshness evidence needed for serving; an
  informer resync or unchanged Firestore document is not proof of a fresh MC read.
- Existing `Delete` walks ApplyDesires and cannot remove a standalone read group.
  Implement `RemoveRead` and verify status/key cleanup, including applier restarts and
  outages. Cleanup exists in the source corresponding to the inspected deployment, but
  its runtime behavior and purge latency must be demonstrated. Address any failures
  before rollout rather than granting Gecko status-DB write access.
- Add targeted resource retirement for legacy Job desires. Never use whole-Cluster
  group deletion to retire an individual Job; other managed resources must survive.
- Firestore integration tests follow the existing emulator patterns
  (`client_integration_test.go`).

#### 3. platform-api

Computed, non-persisted endpoint:

```
GET /apis/gcp.managed.openshift.io/v1/namespaces/{project}/clusters/{name}/credential
```

Behavior:

- **Routing and types**: this is a singleton computed subresource, not an independently
  persisted child such as NodePool. Design an explicit Orlop handler/registration
  extension; ordinary child registration would create the wrong storage/CRUD contract.
  Define the response type from private API sources and follow public generation rules
  without registering a generic credential store, collection, list, or watch. Decide
  discovery/OpenAPI and private-surface exposure in Phase 0; no accidental alternate
  route may bypass the credential permission.
- **Authorization**: add `cluster.credential.get` throughout permission validation,
  Cedar action mapping/policy generation, role definitions, and tests. Grant it to the
  platform `cluster-admin` role, not `cluster-viewer`; `cluster.get`, project membership,
  or creator identity alone does not grant retrieval. Evaluate binding conditions using
  the parent Cluster's identity/spec and project namespace, never the secret payload.
  Fetch parent state and authorize before any credential read. The existing middleware
  installs object-authorization callbacks for CRUD handlers; the new handler must
  explicitly execute the authorization check, not merely add a path mapping.
- **Always enforced**: authentication and credential authorization must remain active
  when `gecko.public-api.authorization.enabled` is false. Reuse the shared Cedar
  authorizer rather than introducing a separate allowlist. Test flag-off behavior and
  fail closed when identity, parent context, or policy evaluation is unavailable.
- **Read path**: on demand, read the latest `{clusterUID}-credential` ReadDesire status
  from the target MC's status DB. Preferred shape: platform-api holds a read-only
  Firestore status client (same credential pattern as the controllers module). If
  platform-api must stay Firestore-free, fall back to a thin internal endpoint served by
  the controllers deployment — decided in Phase 0 before build-out. Authenticate and
  constrain any internal endpoint; it must not bypass public authorization or create
  forbidden direct cross-cluster connectivity. Derive MC placement, UID, and Secret
  identity from trusted parent state, never caller-supplied targeting parameters.
- **Response** (JSON; no kubeconfig assembly server-side — the CLI owns the file format):

```json
{
  "kind": "ClusterCredential",
  "apiVersion": "gcp.managed.openshift.io/v1",
  "metadata": { "name": "{name}", "namespace": "{project}" },
  "apiEndpoint": "https://api.example-cluster.example.com",
  "username": "system:customer-break-glass:example",
  "clientCertificate": "-----BEGIN CERTIFICATE-----...",
  "clientKey": "-----BEGIN RSA PRIVATE KEY-----...",
  "validUntil": "2030-01-02T12:00:00Z"
}
```

  `username` and `validUntil` are derived server-side from the validated certificate.
  The CLI may validate independently. Use `status.hostedClusterResult.apiEndpoint`,
  normalize it to HTTPS, and validate it rather than reconstructing a DNS name.

- **Serving validation**: require the expected read target and successful status;
  decode the Secret, parse PEM, verify the key matches the certificate, check the
  expected upstream customer admin identity/organization and client-auth usage, and check
  `NotBefore`/`NotAfter` with defined clock-skew and minimum-remaining-lifetime rules.
  Enforce the agreed observation/freshness policy and reject known pre-revocation
  material. Define how delivery resumes after signer trust propagation; a valid date
  alone does not establish that the guest trusts a certificate. Keep validation errors
  free of certificate/key content. Final thresholds/evidence are Phase 0 gates.

- **Errors**:
  - Preserve platform authentication/authorization and parent-not-found conventions;
    do not disclose credential readiness to unauthorized callers.
  - Define one typed not-ready response with a stable machine-readable reason and
    bounded retry guidance for provisioning/refresh. Finalize its status and reason in
    Phase 0; do not overload an ordinary parent `404` or retry every `409` blindly.
  - Distinguish credential/transport unavailability (`503`) from parent absence and
    invalid internal credential data. Define deleting-Cluster and validation-failure
    responses alongside the not-ready contract, using safe Kubernetes-style errors.
- **Audit and diagnostics**: implement/verify public-path audit coverage for requester
  identity, project, Cluster name/UID, outcome, request ID, action (`credential.fetch`),
  and a certificate fingerprint on successful retrieval. Do not assume private GKE
  audit automatically covers public requests. Never log/trace response bodies, PEM,
  keys, or raw validation errors. Cover proxies and CLI diagnostics, and assess heap/
  pprof/debug access because this process handles private keys in memory.
- **Caching**: return `Cache-Control: no-store`; do not expose credentials through
  shared response caches or conditional-response mechanisms.
- **Not one-time**: repeated fetches are allowed and expected (lockout recovery,
  re-fetch after cert expiry). Verify applicable rate limits and add bounded controls
  if missing; no per-cluster "already retrieved" state.

#### 4. gcp-hcp-ctl

New command, mirroring the endpoint:

```
gcphcpctl cluster credential my-cluster [--kubeconfig path] [-o json]
```

- Fetches through `pkg/platformapi` using the configured project and existing Cluster
  name resolution. Add a typed credential client method and normalize errors in that
  package; do not display server prose or infer policy causes from a `403`.
- Assembles a kubeconfig entry by adding a client-cert variant to `pkg/kubeconfig`.
  Use a distinct, project-qualified name such as `{project}-{name}-bootstrap` for the
  context and auth entry, with collision-safe cluster entry handling. Preserve other
  entries and **do not change current-context** or replace normal exec-auth credentials.
  Re-fetch replaces this context's auth entry rather than accumulating obsolete keys.
  Define safe overwrite/collision rules and owner-only file permissions.
- Cluster entry needs **no CA data** (public cert-manager certificate on the route) —
  matches the PoC validation for this serving configuration. Keep TLS verification on;
  do not retain unrelated CA settings from a colliding entry or silently use an
  insecure fallback if public trust is unavailable.
- Wait only for the agreed retryable response, honoring bounded retry guidance, a
  documented timeout, and cancellation. Do not retry authorization denials or parent
  absence as provisioning. Validate the response and warn based on `validUntil`.
- Prints the bootstrap guidance:

```
Credential written to ~/.kube/config (context "my-project-my-cluster-bootstrap", valid until 2030-01-02 12:00 UTC).
Your current context has not changed.

This certificate has system:masters power and bypasses Kubernetes authorization.
Use it to grant access to your own OIDC identity, then use normal exec-auth:

  kubectl --context my-project-my-cluster-bootstrap create -f - <<EOF
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  metadata:
    name: my-admins
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-admin
  subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: you@example.com
  EOF

  gcphcpctl cluster login my-cluster

Verify the intended operation using the normal login context, not the bootstrap context.
Normal login does not erase the separate bootstrap context or its private key.
```

- `-o json` emits only the raw credential document for scripting, performs no kubeconfig
  writes, and prints no guidance/progress on stdout. Document that this deliberately
  exports sensitive material; never include it in diagnostic output.
- Document local cleanup of the bootstrap context, auth entry/private key, and any
  unreferenced dedicated cluster entry after normal access is verified. Cleanup is
  local hygiene, not revocation; copied credentials can remain valid until expiry.
- Deferred follow-up (separate story, not blocking): `--bind-admin <email>` convenience
  that creates the CRB through the fetched kubeconfig and then invokes the exec-auth
  login flow, collapsing first access into one command.

### What the customer ends up with in the guest

A ClusterRoleBinding **they created**, scoped how they chose (their user, or explicitly
their workspace group if they accept the domain-wide blast radius). The bootstrap CRB's
audit actor is the upstream certificate identity (`system:customer-break-glass:<id>`), not the
customer's email — worth documenting, since it shows up in audit logs.

Platform audit can correlate a fetch with a certificate fingerprint, but cannot uniquely
attribute guest actions when multiple callers received the same certificate. This
limitation is accepted for this design and must not be presented as per-user guest audit.

## Implementation Plan

### Phase 0 — contracts and security gates (repos: gecko, infrastructure as needed)

Record these decisions here before build-out:

1. Computed-subresource routing, response type generation, discovery/OpenAPI, and any
   private API exposure, without credential persistence or generic CRUD/list/watch.
2. Dedicated permission, role grants, parent-object Cedar context, and always-enforced
   authn/authz independent of the general public-authorization feature flag.
3. Direct read-only Firestore access vs. an authenticated internal service interface;
   define shared package boundaries rather than importing the controllers application
   into platform-api. Confirm database-scoped IAM and deployment wiring.
4. Minimum remaining lifetime, allowed clock skew, observation/freshness evidence,
   revocation fencing/resumption, and one typed not-ready/error/retry contract.
5. Secret-bearing status DB reader boundary, including all controller readers and
   broader grants: explicitly accept or narrow it before mirroring. Review and
   implement needed payload index exclusions, retention/backup controls, effective audit
   policy (including inheritance), and verified removal of mirrored key material.
6. Runtime acceptance for the inspected applier version and any required freshness
   enhancements. Standalone reads are verified in pinned/deployed-tag source; cleanup
   and startup orphan reconciliation are verified in deployed-tag source, not yet by a
   live Secret lifecycle test. Assign cross-repository dependency and CRR owners.

Separate alternative-evaluation track: verify OpenShift's default admin binding and
release-specific OIDC mapping acceptance; define scoped Gecko guest-admin entitlement;
compare broker and synchronization feasibility, customer-IdP compatibility, revocation,
cross-cluster isolation, signing/audit, and recovery. Record an explicit decision before
changing the baseline or treating that alternative as a delivery dependency.

### Phase 1 — additive backend (repo: gecko; kube-applier/infrastructure if required)

Keep the existing RBAC Job behavior during this phase.

1. Add `EnsureRead`, typed credential extraction/read results, safe observed metadata,
   and `RemoveRead`; update mocks and add Firestore emulator integration tests. Test
   idempotency and separation from ApplyDesire/target-resource deletion.
2. hc-controller: ensure the UID-bound credential ReadDesire for placed clusters,
   including existing clusters; expose advisory `BootstrapCredentialReady` and observed
   validity without key material. Wire read cleanup into partial/full deletion.
3. Add the computed credential endpoint, dedicated permission validation/action and role
   grants, explicit parent authorization, serving validation, no-store headers, safe
   errors, audit coverage, and bounded retrieval controls.
4. Verify deployed applier standalone Secret delivery, watch/rotation behavior, status/
   key cleanup, and restart/orphan reconciliation. Source support exists for the deployed
   tag; address runtime failures and missing freshness support before broad enablement.

### Phase 2 — CLI and pre-cutover validation (repo: gcp-hcp-ctl; dev environment)

1. Add the typed client method and `cluster credential` command, bounded readiness
   waiting, and isolated kubeconfig context that preserves current-context/exec-auth.
2. Add output-only `-o json`, expiry guidance, README/bootstrap walkthrough, local-key
   cleanup instructions, and a pointer from cluster creation to the new access flow.
3. Run focused and owning-module suites, diagnostic-redaction tests, and the acceptance
   checks below. In the mixed-mode environment, do not mistake access provided by the
   old domain binding for access provided by the customer's new binding. Use a test
   identity without that grant or deliberately remove the legacy binding in a disposable
   guest and verify denial before the new binding, then success afterward.
4. Validate the no-Job target flow on a disposable/canary cluster before general cutover;
   publish supported backend/CLI versions and rehearse the recovery runbook.

### Phase 3 — staged cutover and legacy Job retirement (repo: gecko)

Only proceed after credential retrieval, supported CLI distribution, security gates,
and recovery/cleanup validation are complete.

1. Stop generating the RBAC Job and remove `buildRBACJob`/obsolete tests. Remove
   `Input.CreatedBy` if unused; preserve creator audit metadata on the Cluster.
2. Idempotently retire existing `rbac-setup-gen-*` Job ApplyDesires/ReadDesires and Jobs,
   with retries and restart-safe progress. Select only platform-owned legacy Job targets
   belonging to the exact Cluster/HCP namespace; include all prior generations.
   Cover multiple prior generations as migration cases, not just the latest Job.
3. Ensure retained ApplyDesires cannot recreate the Job after its TTL or after restart.
   Never delete the whole HC apply group to retire Jobs. Verify Namespace, HostedCluster,
   Certificates, and other desired resources remain intact.
4. Validate fresh cluster creation without the Job and successful first access/recovery.
   Document rollback/version-skew behavior; rollback must not silently reintroduce a
   domain-wide grant into clusters already using customer-owned RBAC.

### Phase 4 — existing-cluster guidance and operations

1. Preserve existing guest `redhat-domain-admins` bindings. Document customer-led
   migration: fetch credential → establish intended OIDC access → verify guest
   authorization → remove the legacy binding → verify intended access still works.
2. Publish CRR authority, operational path, completion conditions, delivery invalidation,
   and local credential cleanup guidance. CRR revokes the customer signer class, not an
   individual caller's downloaded copy.
3. Monitor safe readiness/delivery metrics and fetch outcomes; alert on sustained
   credential refresh failure or legacy Job retirement failure without logging secrets.

### Acceptance criteria

- **Authorization**: admin with the dedicated grant succeeds; viewer, caller lacking
  the permission, non-member, unauthenticated, and cross-project callers cannot retrieve
  credentials. Test parent-specific Cedar conditions, policy errors, flag-off behavior,
  and every registered route. Denials must not trigger a secret read.
- **API contract**: computed singleton only; no accidental credential persistence,
  CRUD/list/watch, alternate-route bypass, or shared caching. Response/error schema,
  no-store header, typed retry behavior, and public discovery/OpenAPI match the contract.
- **Validation**: missing Secret, failed read status, wrong target/Cluster incarnation,
  malformed PEM, mismatched key, unexpected identity/usage, not-yet-valid, expired,
  near-expiry, stale, and known-revoked credentials are never returned as usable.
- **Delivery/lifecycle**: standalone read works without an ApplyDesire; Secret rotation
  propagates; watch interruption/restart and DB outages behave safely; deletion during
  partial/full provisioning removes subscriptions and mirrored keys; recreation under
  the same public name cannot retrieve the previous Cluster's credential.
- **Revocation**: on GKE, prove CRR completion rejects the old credential and a newly
  fetched credential authenticates afterward. Exercise the stale-status window and
  define when retrieval is blocked/resumed; certificate date checks alone are not enough.
- **CLI**: distinct context, unchanged current-context, preserved normal exec-auth and
  unrelated entries, safe collision handling, owner-only files, bounded timeout/
  cancellation, and JSON-only output without kubeconfig writes. Diagnostics at supported
  verbosity levels never reveal credential material.
- **End to end**: create a no-Job cluster → fetch → `SelfSubjectReview` shows
  `system:masters` → create a user binding → normal exec-auth both authenticates and
  authorizes an intended privileged operation. `auth whoami` alone is insufficient.
- **Migration**: all legacy Job desires/Jobs are retired without affecting other managed
  resources or automatically deleting guest bindings. Controller/applier restart and Job
  TTL do not resurrect the old behavior; version-skew/rollback behavior is tested.
- **Secret handling**: captured API/proxy/CLI logs and traces contain no key or response
  bodies; audit records include safe correlation metadata. IAM, indexing, retention,
  backup handling, and eventual key-purge behavior are reviewed and demonstrated.
  Explicitly accept or narrow the controller/broader-grant reader boundary, verify
  effective secret-payload index exclusions, and verify inherited/effective Data Access
  audit policy rather than assuming the current status DB is already suitable for keys.

## Security Considerations

- **Key material at rest**: continuous mirroring stores `tls.key` outside the MC in a
  secret-bearing status DB. Inventory existing readers before approval; grant only
  necessary access to the applier writer and controller/API readers using Workload
  Identity and explicit database-scoped IAM. Firestore default encryption is necessary
  but insufficient. Review index exclusions for secret payloads, backups/PITR/exports,
  regional placement, retention, and removal after subscription/Cluster deletion. Do not
  describe this as an unchanged boundary or copy the key into the platform resource DB.
  Development validation reinforced that approving/narrowing all status-reader access
  and reviewing/applying payload index exclusions are prerequisites, not completed work.
- **Effective audit policy**: project-policy inspection alone does not establish
  effective Firestore/Datastore Data Access coverage. Verify inherited settings and any
  needed configuration before mirroring. Database-read audit
  and public credential-fetch audit are separate requirements; neither substitutes for
  the other, and neither should record credential payloads.
- **MC-side RBAC**: the new reader is the existing applier identity; do not grant
  platform-api direct MC Kubernetes access. Review actual namespace/resource access and
  existing privileged principals rather than claiming no other principal can read the
  Secret. The current applier chart grants broad resource access; narrower exposure
  must be assessed explicitly, not assumed from the field-selected watch.
- **Endpoint exposure**: require the dedicated permission and parent-object conditions,
  independent of the general authorization flag. Return no-store responses, audit
  successes/denials/failures, and prevent response-body logging/capture throughout the
  delivery path. Restrict debug/heap/profile access to processes handling private keys.
- **Repeatable recovery**: multiple callers may receive the same key/guest identity.
  Permission removal blocks future exports only; downloaded certificates remain usable
  until expiry or signer-class revocation. Recovery depends on the platform auth path,
  PKI operator, MC observation, and status transport; it is not an offline fallback.
- **Blast radius**: default guidance binds the *user*, not the workspace group; the
  domain-wide grant that exists today is no longer created by the platform.
- **Kill switch**: if a handoff is botched or a credential is suspected leaked, create a
  `CertificateRevocationRequest` (`signerClass: customer-break-glass`) in the HCP
  namespace — rotates the signer CA, re-signs leaf certs, prunes the old CA from KAS
  trust. It affects all credentials for the customer signer class and is asynchronous;
  wait for the documented completion conditions. Verify GKE behavior and delivery
  invalidation/resumption before cutover, with an authorized operational path and owner.
  CRR avoids the CSR API, but that alone does not validate the full runbook.
- **Lifetime**: enforce parsed certificate validity and the agreed serving policy; do
  not promise 30h remaining. Normal rotation is not revocation. Document local key
  cleanup, the residual lifetime of copied credentials, and recovery during outages.

## Risks & Considerations

1. **Computed-subresource integration**: current Orlop child-resource routing is
   persisted CRUD, not this contract. Routing/type generation/authz integration is
   prerequisite work, not a path-mapping-only change.
2. **Deployed applier lifecycle/freshness**: pinned and deployed-tag source support
   standalone reads; inspected deployment source also has orphan-status cleanup and startup
   reconciliation. Runtime Secret delivery/rotation/purge/restart behavior and revocation
   propagation still require proof; source inspection is not an end-to-end test.
3. **Secret-store blast radius**: status DB readers gain access to guest superuser keys.
   Controller/broader grants, payload indexing, and effective audit coverage require
   explicit review; IAM/retention/indexing/audit safeguards are pre-mirroring gates.
4. **platform-api transport coupling**: direct status DB access adds a dependency;
   an internal service adds a hop and its own authenticated contract. Decide in Phase 0.
5. **Loss of automatic admin**: new clusters require a supported CLI/manual bootstrap.
   Stage rollout, publish instructions, and keep retrieval repeatable for authorized
   admins. Outages or invalid credentials may temporarily block recovery.
6. **Shared guest audit identity and revocation scope**: fetch audit is correlatable,
   not unique attribution; revocation affects all credentials for the customer signer class.
7. **Legacy grants and rollback**: retiring the Job does not remove existing domain-wide
   guest grants. Customer cleanup is explicit; controller rollback must not silently
   restore those grants or legacy Job desires.
8. **HyperShift version floor**: verify the PKI operator/Secret behavior across supported
   releases; development validation is not a full support matrix. Retain the OCP 4.15+
   requirement in that matrix.
9. **Alternative-policy confusion**: OpenShift has a default `system:cluster-admins`
   binding, but Gecko grants do not currently become guest identities automatically.
   Broker/synchronization evaluation must not be described as an implemented path or
   weaken the baseline/customer-issuer identity restrictions without a reviewed decision.

## Open Questions

The product/security direction above is agreed; the following technical gates remain:

- [ ] Which explicit Orlop extension serves computed subresources, and how are response
      generation, discovery/OpenAPI, and any private routes handled without a store?
- [ ] Direct read-only Firestore access vs. an authenticated internal service interface:
      which fits platform-api boundaries, deployment topology, and least-privilege IAM?
- [ ] What minimum remaining lifetime, clock skew, observation freshness evidence, and
      revocation fence/resumption rules are required? What applier metadata is missing?
- [ ] What exact typed not-ready status/reason, retry guidance, timeout, and terminal
      validation/deletion errors will API and CLI implement?
- [ ] Does the inspected applier version pass standalone Secret delivery, rotation, cleanup,
      startup/restart, and outage tests? What freshness enhancements and owners are needed?
      Corresponding cleanup source was inspected; runtime tests remain.
- [ ] Is the controller/broader-grant status-reader boundary accepted or narrowed,
      and what are the reviewed payload index exclusions, effective/inherited audit policy,
      backup/export retention, and mirrored-key purge guarantees?
- [ ] What targeted retirement API/progress model handles all legacy Job generations
      safely, and what prevents unsafe grant reintroduction on rollback/version skew?
- [ ] Who may invoke CRR, through which authorized operational path, and how are its
      completion and subsequent credential delivery verified on GKE?
- [ ] Alternative only: is the default `cluster-admins` binding present on supported
      guests, and is the proposed `system:cluster-admins` OIDC mapping accepted? What
      exact scoped/condition-aware Gecko entitlement should produce guest-admin access?
- [ ] Alternative only: broker or RBAC synchronization, with what customer-issuer,
      audience isolation, revocation/cache, ownership, signing/audit, and recovery model?
      Selection and implementation are not yet agreed.

## Backlog

| Story | Jira | Status |
|-------|------|--------|
| Phase 0: computed-subresource, authorization, transport, freshness/error contracts | TBD | Not Started |
| Phase 0: approve/narrow status readers, apply index safeguards, verify effective audit/retention | TBD | Development review informed prerequisites; decisions and safeguards pending |
| Alternative evaluation: default admin binding/mapping support and scoped Gecko entitlement | TBD | Upstream default binding identified; guest validation and policy decision pending |
| Alternative evaluation: broker vs. synchronization, issuer compatibility, isolation/revocation/recovery tests | TBD | Evaluation only; neither mechanism selected |
| Runtime tests: applier standalone Secret delivery, rotation, freshness, purge, and restart | TBD | Pinned/deployed-tag read source and deployed-tag cleanup source inspected; runtime acceptance pending |
| Transport `EnsureRead`, typed extraction, `RemoveRead`, and emulator tests | TBD | Not Started |
| hc-controller credential subscription, advisory readiness, and deletion cleanup | TBD | Not Started |
| Computed credential endpoint, validation, audit/no-store, and always-enforced permission | TBD | Not Started |
| CLI typed client, `cluster credential`, isolated context, and output-only JSON | TBD | Not Started |
| Security/negative tests and dev e2e, including GKE CRR recovery | TBD | Not Started |
| Staged cutover: remove Job generation and target legacy Job desires/Jobs | TBD | Not Started |
| Docs: first access, customer-led grant migration, local cleanup, and CRR/rollback runbooks | TBD | Not Started |

## References

- OpenShift default administrator binding:
  [`openshift-apiserver/pkg/bootstrappolicy/policy.go`](https://github.com/openshift/openshift-apiserver/blob/master/pkg/bootstrappolicy/policy.go)
  (`GetOpenshiftBootstrapClusterRoleBindings`) and
  [`constants.go`](https://github.com/openshift/openshift-apiserver/blob/master/pkg/bootstrappolicy/constants.go)
  (`ClusterAdminGroup`, `ClusterAdminRoleBindingName`): `cluster-admins` binds
  `system:cluster-admins` to `cluster-admin`. Upstream source is not proof that the
  binding is unmodified or that a particular OIDC mapping is supported on every guest.
- HyperShift customer admin credential mechanism (upstream):
  `docs/content/how-to/pki/break-glass-credentials.md` in openshift/hypershift — signer
  classes, rotation, `CertificateSigningRequestApproval`/`CertificateRevocationRequest` APIs.
- HyperShift `control-plane-pki-operator`: `certrotationcontroller` (pre-generated certs,
  `O=system:masters`), `certificatesigningcontroller`, `certificatesigningrequestapprovalcontroller`.
- Management-cluster CSR placement and GKE Warden constraint:
  `test/integration/control_plane_pki_operator.go` in openshift/hypershift;
  `validateCSRFlow` uses `mgmt.KubeClient` for CSR creation/retrieval and
  `mgmt.HyperShiftClient` for the HCP-namespaced approval.
  `RunTestControlPlanePKIOperatorBreakGlassCredentials` documents the GCP admission
  restriction on privileged subjects; this is upstream evidence, not a fresh admission
  test or a claim about all guest-cluster CSRs.
- CSR controller wiring: `control-plane-pki-operator/operator.go`,
  `control-plane-pki-operator/certificatesigningcontroller/certificatesigningcontroller.go`,
  and `control-plane-pki-operator/certificatesigningrequestapprovalcontroller/certificatesigningrequestapprovalcontroller.go`;
  the controllers observe and update management-cluster CSRs, not guest CSRs.
- Development PoC: customer admin kubeconfig over the public route
  authenticates as `system:masters` with no CA data; `auth can-i create
  clusterrolebindings` = yes. Environment identifiers are intentionally omitted.
- gecko transport architecture: `controllers/client/transport` (Firestore apply/read/delete
  desires consumed by kube-applier-gcp on management clusters).
- Gecko implementation references: `controllers/hc/hc_controller.go` (UID/safeName and
  deletion), `controllers/client/transport/firestore/{client,status,desires}.go`,
  `platform-api/pkg/authz/{middleware,permissions,context}.go`,
  `platform-api/api/private/v1/authorization_validation.go`,
  `platform-api/cmd/platform-api-server/{main,resources}.go`, and
  `orlop/pkg/apiserver/{router,router_helper}.go`.
- Pinned kube-applier source: `v0.0.0-20260903184437-a0a17126d22a`,
  `pkg/controllers/read_desire_manager/controller.go` and
  `pkg/controllers/read_desire_kubernetes/controller.go` (independent list/watch and
  60-second cached reconciliation; distinguish this from deployed-tag inspection).
- Reviewed public kube-applier source for standalone reads and cleanup:
  [read manager](https://github.com/openshift-online/kube-applier-gcp/blob/ad75b07/pkg/controllers/read_desire_manager/controller.go),
  [Kubernetes reflector](https://github.com/openshift-online/kube-applier-gcp/blob/ad75b07/pkg/controllers/read_desire_kubernetes/controller.go),
  and [status cleanup](https://github.com/openshift-online/kube-applier-gcp/blob/ad75b07/pkg/controllers/desirestatuscleanup/desirestatuscleanup.go).
  These are source evidence, not independently verified binary provenance or runtime
  standalone Secret delivery/cleanup results.
- gcp-hcp-ctl references: `pkg/platformapi/{client,errors}.go`,
  `pkg/cluster/login.go`, `pkg/kubeconfig/kubeconfig.go`, and
  `pkg/httpdebug/{payload,redaction}.go`.
- Related plan: [gcp-customer-authentication.md](gcp-customer-authentication.md).
