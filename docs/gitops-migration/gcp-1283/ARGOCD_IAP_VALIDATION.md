# Existing IAP/Dex login validation — GCP-1283

> **Historical validation record (source snapshot).** Retained for provenance.
> Its engineering trial images and the unresolved pre-existing C4 criterion are
> not current-state claims or authorization to repeat the checks.

**Existing-flow validation completed on disposable dev region and MC,
2026-10-21–23.** This is not release certification, a security exception or rollout
approval. The Oct 26 source-history clarification below is not a new runtime test
or a change to Jira acceptance criteria.

## Approach and outcome

The operator-managed dev region and MC preserve the HCP team's established FOSS
Argo CD IAP/Dex flow for the tested contract: organizational login and identity,
representative permitted reads and forbidden creates, existing admin/read-only
permissions, and logout/re-login and Dex-replacement recovery. No migration-specific
authentication regression was identified by these checks.

This change records that existing-flow validation and adds two parity regressions;
it does not redesign authentication or implement additional security hardening.
No Envoy, new identity provider, independent Google-login connector or new OAuth
client was needed. Earlier alternative investigations are not selected
implementations or prerequisites for accepting the demonstrated flow parity.

The unchanged flow is:

```text
Browser -> IAP/GKE Gateway -> Argo CD server -> existing Dex authproxy connector
        -> Dex-issued identity -> Argo CD session and RBAC
```

Dex already exists in the legacy installation; `authproxy` is its connector,
not another proxy Deployment. Preserve both IAP admission and application roles:
the shared-gateway default admits `group:gcp-hcp-eng@redhat.com`, while Argo CD
uses `role:readonly` plus the existing ten named admin bindings. This is not a new
grant to every `redhat.com` account or a claim to have enumerated all inherited
IAM grants/group memberships.

## Established HCP design and inherited hardening concern

The flow above is an intentionally maintained HCP implementation, not a new
operator-specific authentication design:

* The [official bootstrap configuration](https://github.com/openshift-online/gcp-hcp/blob/5d8ef7b9e9ed840e5acc34c968d8da25d2a08139/bootstrap/argocd/argocd-cm-patch.yaml#L19-L27)
  explicitly seeds the same `authproxy`/`iap` connector and Google email header.
* [Merged infrastructure PR #324](https://github.com/openshift-online/gcp-hcp-infra/pull/324)
  introduced the Gateway API/IAP/Dex SSO deployment pattern in February.
* [Merged HCP PR #64](https://github.com/openshift-online/gcp-hcp/pull/64) calls
  authproxy "our IAP setup," fixes its Dex compatibility issue, and records
  successful IAP/Dex login testing in May. The [bootstrap Dex pin](https://github.com/openshift-online/gcp-hcp/blob/5d8ef7b9e9ed840e5acc34c968d8da25d2a08139/bootstrap/argocd/kustomization.yaml#L18-L22)
  preserves that fix.

The header-trust/in-cluster bypass concern also predates this validation. The
infrastructure repository's **June 25 security audit, [finding C4](https://github.com/openshift-online/gcp-hcp-infra/blob/e55f2a4e13907045d73907da715c058579fc72f3/security/audit-report.md#L100-L115)**,
already records the concern and proposes assertion validation and network
restrictions. That static finding is an existing hardening reference, not proof
that this experiment reproduced a bypass or that an operator migration introduced
one. Its presence does not establish that every team member reviewed it or that
the risk was formally accepted.

This record therefore separates **completed existing-flow validation** from
**pre-existing C4 hardening**. It does not declare C4 resolved, create a follow-up
ticket, or approve deferring GCP-1283's stronger signed-IAP/direct-backend criterion.
That criterion still needs an explicit reviewed disposition before story closure;
it must not be marked passed merely because behavior matches the baseline.

## Baseline and versions

| Target | Normal public URL |
| --- | --- |
| Existing integration FOSS baseline | `https://argocd.int.gcp-hcp.devshift.net/` |
| Operator region | `https://argocd-dev-reg-us-c1-rflores0922.dev.gcp-hcp.devshift.net/` |
| Operator MC | `https://argocd-dev-mgt-us-c1-rflores0922.dev-reg-us-c1-rflores0922.dev.gcp-hcp.devshift.net/` |

At the Oct 21 comparison, integration ran upstream Argo CD v3.4.3 and Dex v2.45.1.
The disposable operator trials used engineering images, not release-certified
artifacts. Their inspected image references were:

* Argo CD: `quay.io/redhat-user-workloads/rh-openshift-gitops-tenant/argocd-rhel9@sha256:ec4e9bd577efa235da258f1ea0ab95b7207ac52cca92be6b5437f87dc243eeb2`, reporting upstream source `e258ee23c3e52266d407572f4bcdfe7d9ed36cb5` (v3.5.2).
* Dex: `quay.io/redhat-user-workloads/rh-openshift-gitops-tenant/dex-rhel9@sha256:a8bb0f4ad645b21a21f78e03bbd7d53b4c6abf4e7062d246c0bfbf741d42fa9a`, reporting source `03f0d045357764788673462e4a4a2396416b50a2`.

Image labels are source references, not independently verified build provenance
or confirmation of the intended xKS product-support contract.

## Results and evidence level

| Check | Result | Evidence / limit |
| --- | --- | --- |
| Organizational browser login | Passed on region and MC for Roger | User executed the existing Google Cloud IAP flow, matching integration |
| Displayed identity | Passed on both | User Info screenshots show `accounts.google.com:rfloresj@redhat.com`, not local `admin`, and each correct external Dex issuer |
| Admin sync-permission smoke check | Passed on all three | User reports `{"value":"yes"}` for `applications/sync/default/argocd-config`; no application was synced |
| Default-reader policy semantics | 72/72 local decisions matched expectations | Captured policies evaluated with two Argo CD CLI versions; not a real read-only-user login |
| Public unauthenticated edge | 18/18 requests challenged by IAP | Six paths per target; no cookies/tokens or followed redirects |
| Public-edge synthetic identity headers | 12/12 requests challenged by IAP | Anonymous root requests only; not direct-backend or signed-assertion verification |
| Offline configuration suite | 34/34 passed with each target's served schemas | Bootstrap handoff/schema checks enabled; no optional skips |
| Serving health | Passed at observation | One HEALTHY backend endpoint per target; 21/21 workload pods Ready before restart tests |
| Normal reconciliation stability | Passed to sampled scope | Auth data, UIDs and revisions unchanged while Application reconciliation timestamps advanced |
| Region and MC Dex replacement | Passed for tested positive path | Separately approved same-image replacements; infrastructure checks and fresh user login/permission queries passed |
| Logged-out denial, region | Passed for browser smoke check on Oct 22 | After Argo CD UI logout, screenshots show User Info `{}` and sync-permission query rejected with code `16`, `no session information` |
| Logged-out denial, MC | Passed for browser smoke check on Oct 22 | Roger reports the same User Info `{}` and code `16`, `no session information`, after MC Argo CD UI logout |
| Re-login after Oct 22 logout checks | Passed on region and MC | User-supplied JSON shows `loggedIn:true`, the expected organizational username and each correct Dex issuer; sync permission returns `yes` on both |
| Real read-only user, region | Passed for tested smoke checks on Oct 22 | Roger relays that Kat completed normal login, read query returned `yes` and sync query returned `no` |
| Real read-only user, MC | Passed for tested smoke checks on Oct 22 | Roger relays that Kat completed normal login, read query returned `yes` and sync query returned `no`, matching region |
| Actual Application read, region | Passed for Roger and Bill's reported sessions on Oct 22 | Roger supplied his Application JSON and a screenshot attributed to Bill showing `argocd-config` in namespace `argocd`, project `default`; this is not a `can-i` query |
| Actual Application read, MC | Passed for Bill's reported session on Oct 22 | Roger confirms Bill's same actual Application-read check succeeded on dev MC; no new screenshot or User Info capture was supplied |
| Authenticated Application-create denial, region | Passed for representative forbidden operation on Oct 23 | User-supplied UI screenshot shows `permission denied: applications, create, default/gcp-1283-rbac-probe` and Kat's organizational subject; target attribution is Roger's report |
| Authenticated Application-create denial, MC | Passed for representative forbidden operation on Oct 23 | A second user-supplied UI screenshot shows the same `applications, create` denial for `default/gcp-1283-rbac-probe` and Kat's organizational subject; target attribution is Roger's report |
| Organizational-account denial, region | Observed in user-supplied screenshot on Oct 22 | IAP shows "You don't have access" for `jbranham@redhat.com` at the region URL; exact rule/group cause and HTTP status were not captured |
| Organizational-account denial, MC | Observed by user report on Oct 22 | Roger relays that Jim's same colleague encountered the same IAP access denial as region; no MC screenshot or HTTP status was captured |
| Signed-IAP/backend-spoof negatives | Pre-existing C4 hardening concern; not validated as passing | Already documented in the June 25 audit; successful login and public-edge challenges do not establish these controls |

### Deployed configuration comparison

All three `dex.config` values were byte-identical: one `authproxy` connector, ID
`iap`, consuming `X-Goog-Authenticated-User-Email`. Admin bindings and the default
role matched. Integration leaves RBAC scopes unset (default `[groups]`); the
operator makes `[groups]` explicit. Password admin is disabled; no `accounts.*`
local-account configuration was found. Anonymous login is disabled explicitly
on the operator targets and by default on the inspected baseline.

IAP policies were attached to the appropriate server Services, OAuth
ExternalSecrets reported Ready, and external URLs/internal Dex references were
consistent. Expected environment-specific client IDs, hostnames, Service names
and image distributions were not copied between integration and dev. No Secret
values were read. The existing configuration/secret ownership remains as described
in [the operator configuration guide](https://github.com/openshift-online/gcp-hcp-infra/blob/d2fc1a1b35a84193d654d25fd47407b68c1bacd2/docs/ARGOCD_OPERATOR_CONFIGURATION.md).

### Non-mutating permission checks

Use the normal public hostname in the user's authenticated browser:

```text
GET /api/v1/account/can-i/applications/sync/default/argocd-config
```

This asks whether the current session may sync; it does not execute sync. The
actual `default/argocd-config` Application was verified on all three targets.
Initial queries incorrectly used a lone `*` object and returned `no` everywhere;
application policies require `<project>/<application>` (`*/*` in built-in policy).
Do not count that test-input mistake as an RBAC regression or a denied-user pass.

For an IAP-admitted non-admin user chosen by Roger, the intended smoke matrix is
`applications/get/default/argocd-config -> yes` and
`applications/sync/default/argocd-config -> no`. Use that user's own browser
session, not Roger's admin session. No token/cookie extraction, account creation
or temporary role change is needed. These checks do not exercise every real
mutation endpoint.

**Oct 22 follow-up:** Roger reports Kat successfully authenticated to the normal
operator-region and MC URLs and both expected permission-query values were returned
on each: application read `yes`, application sync `no`, for `default/argocd-config`.
This is relayed user-executed evidence, not an agent-observed session or an
actual attempted sync. This work did not create/change an account, role binding
or IAP grant for this test. Her tokens/cookies and User Info details were not captured.

This completes the real-user read-only smoke matrix on **both region and MC**.
It confirms effective IAP admission and the tested permission pattern, not which
group/inherited grant admitted her; the earlier directory lookup was inconclusive.
It does not establish denial at actual mutation endpoints, logged-out behavior
or unauthorized-account rejection.

**Actual read follow-up, Oct 22:** Roger supplied the result of
`GET /api/v1/applications/argocd-config?project=default` from his own dev-region
browser and subsequently a screenshot attributed to Bill's browser at the same
URL. Both show the actual `argocd-config` Application in namespace `argocd`,
project `default`. This confirms successful Application reads, not just positive
permission queries. Bill's earlier screenshot returned code `16`, `no session
information`, after IAP admission; the successful read followed instructions to
complete Argo CD's separate SSO login. The screenshots do not independently
identify Bill's session subject; attribution is Roger's report. No complete
Application payload, tokens or cookies are included in this record. Roger then
confirmed Bill's same actual-read check succeeded on dev MC, completing Bill's
two-target actual-read matrix. MC is relayed user-executed evidence, not another
screenshot or independently captured session subject. Both targets' subsequent
authenticated create denials are recorded below.
Disabled UI controls alone are not counted as server-side write denial, and the
user declined asking volunteers to run browser scripts.

### Actual Application-create denial — normal UI, Oct 23

Roger reports Kat submitted the proposed New App form on **dev region**. The
supplied screenshot shows application name `gcp-1283-rbac-probe`, project
`default`, Manual sync policy, no deletion finalizer, and this server error:

```text
Unable to create application:
permission denied: applications, create, default/gcp-1283-rbac-probe,
sub: accounts.google.com:kkeane@redhat.com
```

This is an authenticated server-side create-authorization rejection, not an IAP
refusal, absent-session response, disabled button or invalid-source/destination
validation error. The subject is visible in the error. The cropped screenshot
does not show the hostname, source/destination fields or HTTP/gRPC status code;
dev-region attribution is Roger's report. The displayed issue timestamp is not
retained, and no token/cookie contents are included.

The proposed test uses the normal UI, not browser scripts: Manual sync; the
reserved Git URL `https://gcp-1283.invalid/rbac-probe.git`, revision `HEAD`, path
`not-a-real-application`; destination **NAME** `gcp-1283-not-a-cluster` (must not
be registered), namespace `gcp-1283-rbac-test`; and no automated sync, namespace
creation or deletion finalizer. No real destination or deployable source is
needed. In the inspected trial source, create authorization precedes destination/
repository validation and Application creation. If authorization unexpectedly
allows the request, the unregistered destination fails validation before create.
Only the explicit `applications, create` RBAC denial counts as passing; source/
cluster validation errors do not. Cancel the form after recording the result;
do not sync or create a real fixture to turn the test into a deployment.

**MC result, Oct 23:** Roger then supplied a second cropped UI screenshot,
attributed to **dev MC**, with the same `permission denied: applications, create,
default/gcp-1283-rbac-probe` notification and Kat's same organizational subject.
It also does not show the hostname, complete form fields or HTTP/gRPC status code;
the target attribution is Roger's report. No issue timestamp or token/cookie
contents are retained in this record.

Both actual Application-read checks (Bill) and authenticated create-denial checks
(Kat) now pass on both dev targets at this representative scope. The rejected
create requests do not create the requested Application. This completes the
selected permitted-read/forbidden-create operation matrix; no live sync/delete/
override denial or every-write-endpoint coverage is claimed. Integration was not
the target, and no additional restart, role/IAM change, real deployment or
volunteer script execution was needed.

**Separate Oct 22 denial evidence:** Roger supplied Jim's colleague's screenshot
showing Google IAP's "You don't have access" page for `jbranham@redhat.com`, with
the resource URL matching the normal operator-region Argo CD hostname. This is
screenshot-backed observation of an identified organizational account refused
at the public admission gate, before reaching Argo CD. It is not a DNS failure
or an observed Argo CD/Dex login regression; the hostname was reached and the
user identified. No access, group or role change was made by this work in response.

The screenshot does not establish the exact IAM/group/context rule responsible,
a directory-membership result, an HTTP status code, MC behavior, signed-assertion
validation or direct-backend rejection. If this account is not intended to be
admitted, the observed denial is the expected negative case, not a reason to
broaden access. An admitted reader and a rejected organizational account are
different test personas; neither result substitutes for the other.

**Subsequent MC report, Oct 22:** Roger relays that Jim's same colleague also
encountered the same IAP access denial on the disposable dev MC. This completes
observed organizational-account admission-denial coverage on both dev targets:
region is screenshot-backed and MC is relayed user-executed evidence. The MC
report is not a new screenshot, HTTP-status capture or directory-membership
lookup, and neither denial proves signed-IAP validation or direct-backend rejection.

### Logout checks — disposable dev targets only

**Region result, Oct 22:** Roger reports using the dev region Argo CD UI logout
button, then navigating to both supplied region API URLs before logging in again.
User-supplied screenshots show `/api/v1/session/userinfo` returned `{}` (no
authenticated identity displayed) and the sync-permission query returned:

```json
{"error":"no session information","code":16,"message":"no session information"}
```

This passes the region browser logout-denial smoke check: the protected
permission query rejected the request as unauthenticated rather than returning
the earlier admin permission. Code `16` is an application/gRPC code; the
screenshots do not capture an HTTP status. Neither replay of a previously issued
token nor Google/IAP session logout/revocation was tested.

**MC result, Oct 22:** Roger reports using the MC Argo CD UI logout and receiving
the same results as region: User Info `{}` and sync-permission query rejected
with code `16`, `no session information`. This is user-reported evidence, not a
new MC screenshot or HTTP-status capture.

**Re-login on both, Oct 22:** Roger then supplied the returned JSON after normal
login on each target. Both report `loggedIn:true` and username
`accounts.google.com:rfloresj@redhat.com`. Region's issuer is its normal public
URL plus `/api/dex`; MC's issuer is its own normal public URL plus `/api/dex`.
Both sync-permission queries return `{"value":"yes"}`. This completes the
two-target browser logout-denial/re-login smoke matrix, separate from the Oct 21
restart recovery evidence. No application sync was executed.

Repeatable procedure:

Use the **dev operator-region and dev operator-MC URLs** in the target table,
one target at a time. Integration is only the comparison baseline; it is not a
requested logout-test target.

1. In Roger's own authenticated browser profile, freshly open
   `/api/v1/session/userinfo` and the sync-permission URL above to establish the
   current logged-in identity and admin permission.
2. Use that target's Argo CD UI logout, not Google logout or a separate incognito
   session. Stay in the same browser profile.
3. Before clicking login again, freshly navigate to both URLs and record the
   returned JSON or redirect behavior, not a cached page reached with Back.
   Record whether the application session is logged out and whether the prior
   admin permission remains. Report any automatic reauthentication separately;
   it is not logged-out-denial proof. Do not collect tokens, cookies or headers.
4. Log in normally again and confirm identity/permission restoration. Repeat on
   the other dev target. These GET queries do not execute an application sync.

### Local RBAC and public-edge tests

Official upstream CLIs v3.4.3+1801122 and v3.5.2+e258ee2 evaluated the same three
captured RBAC policies: six syntax validations and **72/72 expected decisions**.
Configured admin read/sync/delete checks succeeded; a synthetic unbound policy
subject could read applications/projects/logs but not sync/update/delete/override,
exec or create ApplicationSets. The synthetic subject was an input string, not
a provisioned user. These were local policy-file checks with an isolated HOME
and synthetic localhost-only kubeconfig, not logins or application operations.
Release-asset checksums were verified; this is not downstream artifact certification.

Public GETs checked `/`, `/login`, `/auth/login`, Dex discovery/callback paths and
session user-info (six per target). Four additional root-request profiles per
target used no headers, synthetic unsigned Google identity, synthetic `X-Remote-*`
identity/group headers, or a malformed synthetic assertion plus unsigned identity.
All **18 + 12** requests received IAP-generated Google-login challenges. Only
reserved `.invalid` fixture identities were used; no Authorization/Cookie
credentials, followed redirects, completed OAuth exchanges or direct-backend
requests were involved. Full redirect queries/cookies were not retained.

Those negatives show the tested anonymous headers cannot bypass the **public
entrance**. They do not show that Dex/Argo CD verifies IAP assertions or that a
request reaching the backend around IAP would be denied.

### Reconciliation and separately approved Dex replacements

Passive snapshots approximately 16:45–16:52 UTC showed unchanged captured Dex/RBAC
data, disabled-admin/anonymous settings, ConfigMap/Application UIDs and source
revisions. Each `argocd-config` remained Healthy/Synced; both operator instances
were Available. Application reconciliation timestamps advanced on all three.
This observes Argo **Application** reconciliation, not two operator-generation
changes or uninterrupted behavior between samples.

Roger then explicitly approved one region Dex pod replacement and, after region
login succeeded, separately approved one MC replacement. Each preflight required
the expected UID/image, a Ready single replica, correct owner chain, Available
Argo CD and Healthy/Synced configuration. DELETE used UID/resourceVersion
preconditions and normal termination. No forced deletion or template edit was used.

| Target | Old pod | Replacement pod | Ready transition (UTC) |
| --- | --- | --- | --- |
| Region | `gitops-dex-server-7f6b968cc8-jlwq4` | `gitops-dex-server-7f6b968cc8-9k586` | `16:59:21Z` |
| MC | `gitops-dex-server-8b8f97f57-th7g9` | `gitops-dex-server-8b8f97f57-chp2f` | `17:17:00Z` |

Each replacement retained its ReplicaSet and pinned image. Recovery checks
preserved ArgoCD UID/spec/generation, server/Dex Deployment UIDs/specs, four
ConfigMap UIDs/data, configuration Application UID/spec, routing/IAP policy and
OAuth references/readiness. **20 region and 14 MC Applications remained
Healthy/Synced**. Backend health, discovery and the public IAP challenge recovered;
the region's separate Gecko route remained Accepted/ResolvedRefs on its own backend.
Roger then reported fresh logout/login and positive permission queries on both.
Existing sessions/discovery alone were not counted as that login proof.

Integration was never restarted. Ready timing is not a measurement of end-user
SSO downtime. The only agent-initiated Kubernetes mutations were the two individually
approved pod deletions: no images, auth inputs, permissions, Secrets, Gateway/
Service, CR/Deployment/Application configuration or Terraform were changed. No
further restart or production rollout is authorized by this record.

## Regression coverage and reproduction

Two focused tests in `argocd/scripts/test_argocd_config.py` preserve the default
single-connector login contract and IAP/OAuth ExternalSecret/SecretStore delivery
between legacy and operator modes. No production values/templates changed.
Both target schemas passed **34/34** tests with bootstrap components from a clean
`gcp-hcp` checkout at `4a59fb3`. Unlike earlier 30-pass/two-skip runs, optional
schema/bootstrap checks were enabled. Their fixture work is local, not a cluster apply.

> **Archive checkout note:** Run both test examples below from the root of the
> appropriate `gcp-hcp-infra` checkout. `GCP_HCP_REPO` selects the bootstrap
> checkout for optional checks; it does not change the working directory.

```sh
uv run argocd/scripts/test_argocd_config.py
```

To enable the optional checks, provide a bootstrap checkout and a directory
containing read-only-fetched `argocds.argoproj.io.json` and
`notificationsconfigurations.argoproj.io.json`:

```sh
GCP_HCP_REPO=/path/to/gcp-hcp \
ARGOCD_CONFIG_CRD_DIR=/path/to/served-crds \
uv run argocd/scripts/test_argocd_config.py
```

Sanitized detailed reports/snapshots remain in the tester's local ignored
`.work/gcp-1283-baseline/` directory; they are not distributed with this change.
The evidence above distinguishes observed runtime results, user reports and local
policy/model checks. No credentials, browser-session tokens or raw cookies are included.

## Oct 23 read-only security boundary review

After Roger refreshed local Google authentication, fresh Kubernetes/GCP reads on
both dev targets inspected the existing trust boundary; no active bypass test was
run. The Oct 26 source-history review above ties these observations to the
previously documented C4 concern, not a newly discovered operator regression.

| Boundary | Read-back / limit |
| --- | --- |
| Deployed authentication | Same pinned server/Dex images and connector configuration; selected running Pods have no verifier sidecar. The inspected Argo CD `/api/dex/` proxy and Dex `authproxy` callback do not validate IAP assertions |
| Public entrance | Accepted HTTPS HTTPRoutes directly target `gitops-server:443`; attached policies and Compute backend configuration enable IAP. Project-local accessor grant remains the existing engineering group |
| Alternative exposure | All Services in each `argocd` namespace are ClusterIP without external IPs/NodePorts/LB addresses; no Ingress objects were returned. These observations are not an exhaustive reachability test |
| Network isolation | Both `gitops-server-network-policy` objects select the actual server label and allow `ingress: [{}]`. Dex restricts its auth ports to same-namespace server-labelled Pods, but the server proxies Dex requests without adding an IAP assertion verifier |
| GKE/firewall context | Both clusters have private nodes and Dataplane V2 (`ADVANCED_DATAPATH`); returned firewall rules permit cluster-internal traffic and Google LB/health-check ranges. Actual caller-to-backend reachability was not tested |
| IAM evidence limit | Project grant was read, but enabled-service listings did not include the IAP management API. It was not enabled and backend-specific/inherited IAM or group membership was not fully enumerated; this does not mean backend IAP is disabled |
| Separate region route | Gecko's separate Gateway/Service route remains Accepted/ResolvedRefs with its own attached backend policy |

Private Services/nodes and successful IAP admission are **not** cryptographic
assertion verification. Conversely, the permissive server NetworkPolicy and
header-trusting source are **not proof of an internet-accessible bypass or a
completed unauthorized session**. No such session or spoof request was attempted.
The conventional GKE `networkPolicyConfig.disabled` field was not mistaken for
disabled Dataplane V2 enforcement.

These read-backs are consistent with the established HCP implementation and the
pre-existing C4 concern. They do not identify a new authentication defect caused
by the operator migration. Additional hardening is not implemented or selected
by this validation change; a new verifier, proxy or login flow is not proposed as
its outcome. NetworkPolicy-only changes would not implement cryptographic
signature/issuer/exact-audience/expiry validation.

No new proxy/IDP/OAuth flow, API enablement, policy change, port-forward, Pod
exec/run, Secret-value read, token collection or restart was performed or selected
by the read-only review.

## Completed validation and closeout qualifications

The browser logout-denial and subsequent re-login checks are complete on both
dev targets. They establish the tested browser-session behavior, not replay or
revocation of an old token or termination of the Google/IAP session.

The representative role-operation matrix is also complete: actual Application
reads pass on both dev targets, and Kat's authenticated create requests are
explicitly denied on both. These checks do not establish denial at every mutation
endpoint or satisfy the original signed-IAP/backend-protection requirement.

* A separate organizational volunteer's IAP denial is observed on
  both dev targets (region screenshot; MC user report). These observations do
  not enumerate group membership or establish application-side signed-IAP or
  direct-backend protection.
* The inherited signed-IAP/header-spoof/direct-bypass concern is already recorded
  in C4. The inspected Dex connector trusts headers, does not validate IAP
  assertions and does not implement its `allowedDomains` setting. This validation
  neither changes those facts nor demonstrates those negatives as passing.
* **Jira scope disposition remains explicit:** GCP-1283 still contains the stronger
  security criterion. Any decision to scope this ticket to existing-flow parity
  and track C4 hardening separately must be reviewed and recorded, including any
  follow-up tracking. This document does not change the criterion, accept the
  risk or assert that a separate hardening ticket exists.
* Complete review/merge of the validation record and regression tests and the
  applicable Definition of Done. No additional authentication redesign is proposed
  as part of the completed existing-flow validation. This is not a declaration
  that the story is already closed or that the remaining security criterion passed.

Official release/support verification and production rollout remain separate
adoption work. Earlier Envoy/native-Google experiments are not selected deployment
decisions or included in this validation change.

## Source references

* [Baseline API](https://github.com/argoproj/argo-cd/blob/v3.4.3/server/account/account.proto) and [trial CanI implementation](https://github.com/argoproj/argo-cd/blob/e258ee23c3e52266d407572f4bcdfe7d9ed36cb5/server/account/account.go): permission evaluation without application mutation.
* [Baseline built-in policy](https://github.com/argoproj/argo-cd/blob/v3.4.3/assets/builtin-policy.csv) and [trial policy](https://github.com/argoproj/argo-cd/blob/e258ee23c3e52266d407572f4bcdfe7d9ed36cb5/assets/builtin-policy.csv): project/application-scoped permissions.
* [Inspected Dex connector](https://github.com/rh-gitops-midstream/dex/blob/03f0d045357764788673462e4a4a2396416b50a2/connector/authproxy/authproxy.go): existing header-trust limitations.
* [Trial Application Create handler](https://github.com/argoproj/argo-cd/blob/e258ee23c3e52266d407572f4bcdfe7d9ed36cb5/server/application/application.go#L348-L395) and [destination validation](https://github.com/argoproj/argo-cd/blob/e258ee23c3e52266d407572f4bcdfe7d9ed36cb5/server/application/application.go#L1365-L1367): create RBAC is checked before validation and persistence.
* [Trial Dex route registration](https://github.com/argoproj/argo-cd/blob/e258ee23c3e52266d407572f4bcdfe7d9ed36cb5/server/server.go#L1366-L1375) and [Dex reverse proxy](https://github.com/argoproj/argo-cd/blob/e258ee23c3e52266d407572f4bcdfe7d9ed36cb5/util/dex/dex.go#L50-L98): inspected backend path, not an IAP assertion verifier.
* [HCP IAP decision](https://github.com/openshift-online/gcp-hcp/blob/3ff054a6384cffab43d3216b7d8dc128675f64ef/design-decisions/identity/iap-authentication.md): existing signed-assertion and IAM requirements, not waived here.
