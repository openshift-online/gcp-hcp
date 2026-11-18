# Operator CSV cluster role: comparison and risk record (GCP-1286 B4)

> **Historical decision record.** Retained as the GCP-1286 B4 source snapshot.
> The reusable, trimmed permission contract remains in the `gcp-hcp-infra`
> repository; this document is not current operating guidance.

Status: **accepted as a recorded decision on 2026-11-11**. The platform register
is the Confluence page [Privileged cluster controllers on GCP
HCP](https://redhat.atlassian.net/wiki/spaces/GCP/pages/521209731); team
agreement and any change to the decision are tracked in
[GCP-1429](https://redhat.atlassian.net/browse/GCP-1429). Nothing here is a Red
Hat support statement, a security approval, or a runtime observation.

## 1. Why this document exists

`docs/ARGOCD_CONTROLLER_PERMISSIONS.md` ends with:

> The operator's own CSV permissions remain a separate owner/risk-review concern.

`tools/gitops-bootstrap/chart-src/PROVENANCE.md` already records a partial
statement about the CSV role, including that `bind`/`escalate` make the operator
cluster-admin-equivalent. What was missing is
the piece that turns "this looks broad" into a decision: an exact comparison
against the cluster roles we run **today**, a decomposition of the 46 rules by
what can actually act on a GKE cluster, the escalatory primitives named one by
one, and an assessment of whether a narrower role is available.

This document supplies them.

## 2. Scope and non-claims

In scope: the official CSV's `clusterPermissions` cluster role and its
namespaced `permissions`, as rendered by the local chart, and the cluster RBAC
the current Argo CD bootstrap installs.

Out of scope, and not implied anywhere below:

- Operator-generated per-instance roles (`<instance>-argocd-*`). Those belong to
  the controller-policy work in `docs/ARGOCD_CONTROLLER_PERMISSIONS.md` and to
  the interim parity grant in `bundle-templates/controller-rbac.yaml.tmpl`.
- Any admission-time or runtime enforcement on GKE, which does not exist.
- Any claim that GKE support is certified, or that Red Hat reviewed this
  exact rule set for GKE.

## 3. The two baselines

### 3.1 What runs today

`bootstrap/argocd/install.yaml` in `openshift-online/gcp-hcp` at revision
`2640be8e24b667d50003ad6a420af168e7742304` is applied by
`bootstrap/argocd/kustomization.yaml` and is the upstream Argo CD install
manifest pinned for Config Sync. Pin the revision when re-running; the file
drifts on `main`. It creates
**three cluster-privileged identities**, each with a Role and a ClusterRole
bound to the same ServiceAccount in `argocd`:

| Identity (ServiceAccount `argocd/...`) | ClusterRole rules |
| --- | --- |
| `argocd-application-controller` | `*/*/*` plus `nonResourceURLs: ["*"]` |
| `argocd-applicationset-controller` | 7 enumerated rules |
| `argocd-server` | `*/*` for `delete`, `get`, `patch`, plus 5 narrower rules |

### 3.2 What the candidate adds

The chart renders the CSV's cluster role unchanged (asserted by
`tests/run-step2-tests.sh:171`, "ClusterRole rules equal the official CSV
clusterPermissions"). It creates **one** cluster-privileged identity:

| Identity | Source | Rules |
| --- | --- | --- |
| `openshift-gitops-operator-controller-manager` in the operator namespace | CSV `clusterPermissions[0]` | 46 |
| same ServiceAccount | CSV `permissions[0]` as a namespaced Role | 3 (leader election) |

So the delta in identity count is 3 to 4. The new identity is the operator
manager itself, which is the component that reconciles `ArgoCD` and
`RolloutManager` resources.

## 4. Rule-shape comparison

| | Today | Candidate |
| --- | --- | --- |
| Wildcard `*/*/*` rule | yes (application controller) | no |
| `nonResourceURLs: *` | yes (application controller) | only `/metrics` `get` |
| Distinct (group, resource, verb) tuples | 49 across 3 roles | 567 across 46 rules |
| Writes to CRDs | yes (via wildcard) | no, `get/list/watch` only |
| Reads Nodes, writes PVs | yes (via wildcard) | no |

Literally the candidate is narrower than the wildcard we run; section 6 shows why that is misleading: a few verbs restore the wildcard.

## 5. Where the 46 rules can act on GKE

The CSV targets 102 distinct `(apiGroup, resource)` pairs plus one non-resource
URL. Classified by whether the API can exist on a GKE cluster:

| Class | Targets | Meaning |
| --- | --- | --- |
| Live on GKE | 50 | Standard Kubernetes APIs, plus `argoproj.io` and `promoter.argoproj.io` CRDs this chart installs |
| OpenShift-only | 22 | No such API on GKE, ever |
| Third-party CRD | 16 | Inert unless the CRD is installed later |
| Argo Rollouts CRDs | 10 | `rollouts`/`analysisruns`/`experiments` family; not installed by this chart |
| Unserved group | 1 | `extensions`, removed from Kubernetes in 1.22 |
| No-op entries | 3 | Resource name does not exist in that group |
| Non-resource | 1 | `/metrics` `get` |

Sum is 103.

### 5.1 OpenShift-only (22)

`apps.openshift.io/*`, `route.openshift.io/*`, `routes`, `routes/custom-host`,
`oauth.openshift.io/oauthclients`, `config.openshift.io/{apiservers,authentications,clusterversions,ingresses}`,
`console.openshift.io/{consoleclidownloads,consolelinks,consoleplugins}`,
`template.openshift.io/{templates,templateinstances,templateconfigs}`,
`pipelines.openshift.io/*` and the `gitopsservices` family,
`operators.coreos.com/{subscriptions,operatorgroups,clusterserviceversions}`.

These correspond to GitopsService, Console, OAuth, Routes, Templates and OLM.
A plain GKE region serves none of them. A management cluster that installs
HyperShift does serve `routes.route.openshift.io`, so the operator's Route
rules are live there; they grant control of Route objects only and do not add
to the escalation path in section 6. Everywhere else these rules are dead
weight, not risk.

### 5.2 Third-party CRDs (16)

`appmesh.k8s.aws`, `eks.amazonaws.com`, `elbv2.k8s.aws`, `getambassador.io`,
`x.getambassador.io`, `networking.istio.io`, `split.smi-spec.io`,
`traefik.containo.us`, `monitoring.coreos.com`.

**Inert is not the same as safe.** Each of these activates the moment someone
installs the corresponding CRDs. `monitoring.coreos.com` is the realistic one:
anyone deploying kube-prometheus-stack to a cluster where this operator runs
would grant it write access to Prometheus and ServiceMonitor objects. Those are
not passive: a ServiceMonitor selects what gets scraped, and a Prometheus object
can name the ServiceAccount the Prometheus pod runs as.

On the disposable pair this is not hypothetical. The GCP-1280 dev
baseline that the pair follows installs `prometheus-operator-crds` 31.0.0 on the
management cluster, so `prometheuses`, `prometheusrules` and `servicemonitors`
exist and the operator's write on all three is usable. This does not raise the
ceiling, and the pair carries no production data. It does mean the Prometheus
path is part of the pair's residual rather than an inert rule. The re-review
trigger in section 9 is scoped accordingly.

### 5.3 Argo Rollouts CRDs (10)

`rollouts`, `rollouts/finalizers`, `rollouts/scale`, `rollouts/status`,
`analysisruns`, `analysisruns/finalizers`, `analysistemplates`,
`clusteranalysistemplates`, `experiments`, `experiments/finalizers`.

This chart does not install those CRDs. Nothing in this bootstrap creates a
`RolloutManager` resource either, so the family stays inert. Note that the CSV
does grant create on `rolloutmanagers`, so the route to turning them on exists
without any further permission change.

### 5.4 No-op entries (3)

- `apiGroups: [""] resources: ["deployments"]`: Deployments are in `apps`, not
  core. The real permission is the separate `apps` rule.
- `apiGroups: ["apps"] resources: ["podtemplates"]`: pod templates are core
  only. The real permission is the core rule.
- `apiGroups: [""] resources: ["services/finalizers"]`: services have no
  `finalizers` subresource in current Kubernetes.

These are harmless, and they are a useful signal about the rule set: it was
assembled by hand over many releases and is not tightly validated against a
live API surface.

### 5.5 What remains live (50)

Core group: `configmaps`, `endpoints`, `events`, `namespaces`,
`persistentvolumeclaims`, `pods`, `pods/eviction`, `pods/log`, `podtemplates`,
`resourcequotas`, `secrets`, `serviceaccounts`, `serviceaccounts/token`,
`services` (full write on most of those).

`apps`: `daemonsets`, `deployments`, `deployments/finalizers`, `replicasets`,
`statefulsets`, full write.

`batch`: `cronjobs`, `jobs`, full write. `autoscaling/horizontalpodautoscalers`,
full write. `coordination.k8s.io/leases` create/get/update.
`networking.k8s.io/{ingresses,networkpolicies}`, full write.
`apiextensions.k8s.io/customresourcedefinitions`, read only.
`apiregistration.k8s.io/apiservices`, full write.
`authentication.k8s.io/tokenreviews` and `authorization.k8s.io/subjectaccessreviews`,
create.

`argoproj.io`: `applications`, `appprojects`, `argocds` (+ finalizers, status),
`rolloutmanagers` (+ finalizers, status), `namespacemanagements` (+ status),
`notificationsconfigurations` with `*`.

`promoter.argoproj.io`: `*` read, and `controllerconfigurations` (+ finalizers,
status) with `*`.

`rbac.authorization.k8s.io`: the rule discussed next.

## 6. The verbs that decide the risk

Kubernetes normally refuses to let a subject create a role that grants more than
it holds. `escalate` and `bind` are the two verbs that waive that check:
<https://kubernetes.io/docs/reference/access-authn-authz/rbac/#privilege-escalation-prevention-and-bootstrapping>

The CSV grants both, on `resources: ["*", "clusterrolebindings", "clusterroles"]`
with verbs `bind, create, delete, deletecollection, escalate, get, list, patch,
update, watch`.

Everything else in the 46 rules is advisory: with `escalate` on ClusterRoles a subject can author `*/*/*`; with `create`/`bind` on ClusterRoleBindings it can attach it to itself.

Comparison of the primitives that matter:

| Primitive | Operator CSV | application-controller (today) | applicationset-controller | server |
| --- | --- | --- | --- | --- |
| Mint an arbitrary ClusterRole | yes | yes | no | no |
| Bind any role, including `cluster-admin` | yes | yes | no | no |
| Create ClusterRoleBindings | yes | yes | no | no |
| Mint a token for any ServiceAccount | yes | yes | no | no |
| Read every Secret cluster-wide | yes | yes | yes | yes |
| Write Secrets cluster-wide | yes | yes | no | no |
| Delete any Namespace | yes | yes | no | yes |
| Create/delete APIServices (aggregation layer) | yes | yes | no | no |
| Evict any Pod | yes | yes | no | no |
| Write CRDs | no | yes | no | no |
| Read Nodes | no | yes | no | yes |
| Create PersistentVolumes | no | yes | no | no |

The honest conclusion: the candidate does not raise the privilege ceiling on
these clusters. The ceiling is already `cluster-admin`-equivalent, held by the
application controller we have been running. The candidate adds a **second**
identity at that ceiling, and makes its rule set **reviewable** because the
dangerous parts are five named privileges instead of an unbounded wildcard.

Two entries deserve separate mention because they are easy to miss:

- `serviceaccounts/token` `create`: minting a token for any ServiceAccount, in
  any namespace, is identity theft of every workload identity in the cluster.
  On GKE this does not directly reach the node's Google service account unless
  that account is itself projected into a pod, but it does reach every
  Kubernetes credential, including any admin ServiceAccount.
- `apiregistration.k8s.io/apiservices` full write: creating an APIService can
  redirect an existing API group to a backend the operator controls. It is a
  cluster-wide interception primitive, not a configuration convenience.

## 7. What the operator plausibly needs

From the pinned-source reading already recorded in `PROVENANCE.md`:

- `cmd/main.go` always starts the ArgoCD, RolloutManager and
  NotificationsConfiguration controllers.
- GitopsService registers only when `config.openshift.io` is available, which on
  GKE it is not.
- The conversion webhook is disabled in this packaging (`ENABLE_CONVERSION_WEBHOOK`
  is forced to `"false"` and tests reject anything else).

So on GKE the OpenShift-only rule set maps to controllers that never run. That
is consistent with the classification in section 5.1, and it is the strongest
argument that the CSV is carrying a platform's worth of permissions it cannot
use here.

What this document does **not** claim is a rule-by-rule justification. The
manager is a compiled binary and the CSV is generated from controller markers
plus hand edits. Establishing which of the 50 live targets each reconcile path
actually touches would require either upstream confirmation or runtime API
server audit logs. Both are outside offline scope.

## 8. Is a narrower supported role available?

No, on the evidence available:

- The CSV declares only `AllNamespaces` install mode, and it is the only
  published RBAC for this operator. There is no xKS or non-OpenShift role in the
  bundle.
- Vendor chart delivery for xKS is still in progress
  ([GITOPS-10648](https://redhat.atlassian.net/browse/GITOPS-10648), In Progress).
  A developer build of the 1.22.3 chart exists in a quay namespace and its
  manager ClusterRole still carries `bind` and `escalate`, so it offers no
  narrower baseline to adopt; the supported `registry.redhat.io` publication is
  timed with 1.23.
- Jira text search was unavailable during this review: every JQL, including
  `project = GCP ORDER BY updated DESC`, returned zero results. Absence of an
  upstream RBAC story is therefore **not** evidence of absence.

Pruning inert targets would remove 52 of 102 API targets (22 OpenShift-only, 16 third-party CRD, 10 Rollouts, 1 unserved group, 3 no-op). Not recommended:

- It breaks rule-for-rule parity with the CSV, so every future release becomes a
  manual diff of a hand-edited role.
- It reduces nothing that matters. The escalatory primitives in section 6 all
  remain; the risk is `escalate` and `bind`, not `traefik.containo.us`.
- Pruned APIs that appear later fail at reconcile time rather than degrade.

Only removing `escalate`/`bind`/`serviceaccounts/token`/cluster-wide `secrets`/`namespaces` delete would change risk, and that needs reconcile-path knowledge.

### 8.1 The scoped-identity variant we adopt

Decided on 2026-11-11 for GCP-1286: the bundle we would run uses a scoped
operator identity. The vendor CSV rule set stays in the release matrix as the
shipped baseline and stays the default chart mode; the scoped variant is an
explicit bundle option, tested offline, with section B as its runtime proof.

This supersedes the earlier note that we had not adopted it. Two claims in that
note were wrong, and checking the pinned source (`argoproj-labs/argocd-operator`
at `a213c171036a`) settled the mechanics:

- The switches are not inert here. `render.sh` sets
  `ARGOCD_CLUSTER_CONFIG_NAMESPACES` to the instance namespace, so
  `IsNamespaceClusterConfigNamespace(cr.Namespace)` is true.
- `spec.defaultClusterScopedRoleDisabled` is what gates the cluster-scoped
  ClusterRoles and ClusterRoleBindings (`controllers/argocd/role.go`,
  `checkCustomClusterRoleMode`; `controllers/argocd/rolebinding.go`).
  `CONTROLLER_CLUSTER_ROLE` and `SERVER_CLUSTER_ROLE` are what replace the
  default namespaced Role and repoint the namespaced RoleBinding at a named
  ClusterRole (`getCustomRoleName`, `reconcileRole`, `reconcileRoleBinding`).

With both switches set, the operator creates no controller or server ClusterRole
and no ClusterRoleBinding, skips the default namespaced controller and server
Roles, and deletes any default Role or ClusterRole it created earlier. It still
creates namespaced Roles and RoleBindings for dex, redis, redis-ha and the
commit server (empty policy) in the instance namespace.

What the bundle supplies instead:

- Controller: the existing `<instance>-application-controller-interim-parity`
  ClusterRole and its ClusterRoleBinding, unchanged from the migration-parity
  exception already reviewed. Scoped mode currently requires that name and
  cannot bind a caller-supplied role; [GCP-1433](https://redhat.atlassian.net/browse/GCP-1433)
  tracks letting it accept a reviewed controller role and removing the wildcard.
- Server: a new `<instance>-argocd-server` ClusterRole and its
  ClusterRoleBinding. The rule set is the union of the operator's two server
  policies for the reviewed seed with the web terminal off
  (`policyRuleForServer` and `policyRuleForServerClusterRole`). The union is
  required because the operator applies the namespaced policy through a
  RoleBinding and the cluster policy through a ClusterRoleBinding; replacing both
  with one ClusterRole must cover both.
- `<instance>` namespace `Role`/`RoleBinding`
  `openshift-gitops-operator-namespaced-bind`: `bind` on `roles` only. The
  operator needs it to bind the dex, redis and redis-ha Roles it still creates.
- `<instance>` namespace `Role`/`RoleBinding`
  `openshift-gitops-operator-applicationset-grant`: the operator's own
  ApplicationSet controller policy (`policyRuleForApplicationSetController` at
  the same pinned commit), bound to the operator ServiceAccount. Added after the
  first runtime apply; see the finding below.
- `<instance>-<namespace>-argocd-applicationset-controller` `ClusterRole` and
  `ClusterRoleBinding`: the cluster-scoped pair the operator creates for that
  controller under `GenerateUniqueResourceName`, declared by the bundle with the
  operator's cluster rule list so the operator's `Get` matches and it creates
  nothing.

The manager ClusterRole delta, verified by rendering both modes:

- The unqualified `bind` and `escalate` verbs are removed. The only remaining
  `bind` is `clusterroles` limited by `resourceNames` to the two roles above.
- Every other rule is unchanged. The rule count goes from 46 to 47 because the
  qualified bind is a separate rule.

`escalate` can be dropped entirely only because the target is GKE. The operator
appends an OpenShift restricted-SCC rule to its dex/redis Policies when the
OpenShift Config API is present, and the CSV rule set has no
`security.openshift.io` grant. On GKE that branch does not run. Scoped mode must
not be reused on OpenShift without adding an `escalate` path for those Roles.

#### Finding from the first runtime apply, 2026-11-13

Both clusters came up, Config Sync converged, the operator manager ran, and
`ArgoCD/gitops` on both targets stayed `phase=Failed` with `Reconciled=False`.
Abridged, the reconciliation error listed:

```
roles.rbac.authorization.k8s.io "gitops-applicationset-controller" is forbidden:
user "system:serviceaccount:gitops-operator:openshift-gitops-operator-controller-manager"
is attempting to grant RBAC permissions not currently held:
applicationsets, applicationsets/finalizers (all verbs)
applicationsets/status (get, patch, update)
coordination.k8s.io leases (delete, list, patch, watch)
extensions deployments (get, list, watch)
```

The design above missed this case. The ApplicationSet controller is
the one component outside both switches: `reconcileApplicationSetController`
creates `<instance>-applicationset-controller` itself, and
`reconcileApplicationSetClusterRole` plus its RoleBinding create the
cluster-scoped pair under `GenerateUniqueResourceName`, gated only by
`IsNamespaceClusterConfigNamespace`. `defaultClusterScopedRoleDisabled` reaches
neither. With `escalate` removed, the namespaced Role create fails and the
reconcile stops before the controller Deployment exists.

Pre-creating the namespaced Role does not fix it. When the Role exists the
operator always calls `Update`, and the RBAC storage layer checks an update
against the full rule set with no old-versus-new diff
(`pkg/registry/rbac/role/policybased/storage.go`, Kubernetes 1.36). A Role the
operator cannot create, it also cannot update, even to identical rules. The
operator has to hold those rules in that namespace.

The bundle now declares, all in the child bundle:

- `Role`/`RoleBinding` `openshift-gitops-operator-applicationset-grant` in the
  instance namespace, bound to the operator ServiceAccount, carrying the
  operator's own `policyRuleForApplicationSetController` copy. This is what makes
  the operator's create and later updates pass the escalation check.
- `ClusterRole`/`ClusterRoleBinding`
  `<instance>-<namespace>-argocd-applicationset-controller`, with the operator's
  cluster rule list, so the operator's `Get` matches on rules, subjects and
  roleRef and it creates nothing. Declaring the binding also keeps this
  ClusterRole out of the manager's `bind` grant, which still names exactly the
  controller and server roles.
- No `escalate` verb anywhere.

Measured on both targets: with those objects present, `gitops` reached
`phase=Available`, `Reconciled=True` and the ApplicationSet controller Deployment
came up. The objects were first applied by hand as a probe, then declared in the
bundle. The pair bundles were republished as `04f6e7d` (child content) and
`7982574` (parent pointer), the parent pin moved by apply on 2026-11-13, and all
four objects are now Config Sync-owned on both clusters with source token
`04f6e7d`. `ArgoCD/gitops` stayed `Available` after ownership moved.

Residual risk, stated plainly:

- The controller keeps the legacy wildcard policy. That is not an operator switch
  and the scoped bundle does not narrow it. GCP-1285 closed with a reviewed
  controller policy for the real clusters; the disposable pair keeps
  interim parity because scoped mode cannot bind a supplied role yet. Removing it
  is [GCP-1433](https://redhat.atlassian.net/browse/GCP-1433).
- The server ClusterRole and the ApplicationSet grant are copies. A future
  operator release that changes either server policy or
  `policyRuleForApplicationSetController`, or the cluster-scoped ApplicationSet
  rule list, must be diffed against the templates before scoped mode is used
  again. The cluster-scoped pair fails closed if it drifts; a changed namespaced
  rule set fails at reconcile with the same escalation error seen on 2026-11-13.
- The operator now holds `applications`/`applicationsets` write and
  `leases` delete in the instance namespace, which is what it needs to grant the
  ApplicationSet controller. That is a namespace-scoped addition. The manager
  already creates and updates the Deployments that run in that namespace, so it
  is not a meaningful change in authority over the namespace it owns.
- The manager can still bind the two named ClusterRoles anywhere, and still
  chooses which namespaces it manages. The narrowing removes cluster-wide
  escalation and arbitrary role binding, not the manager's authority over the
  instance.
- The scoped identity itself is proven at runtime on the disposable pair
  (2026-11-13): `ArgoCD/gitops` `Available` with the scoped manager, the
  ApplicationSet grant declared in the bundle, and the objects owned by Config
  Sync. The MC later failed on the Route scheme ordering, unrelated to RBAC; that
  is recorded in
  [`gitops-release-validation/2026-11-13-dev-pair.md`](gitops-release-validation/2026-11-13-dev-pair.md)
  §5.3.

The offline contract is enforced, not just documented: `render.sh` fails closed
if scoped mode renders an `escalate`, more than one `bind`, or a `bind` whose
`resourceNames` are not exactly the two roles; `assemble.sh` pins the identity
set, the seed switch, the namespaced bind grant, the server role and the three
ApplicationSet declarations.

Position: adopted for the GCP-1286 validation bundles, with the runtime proof in
section B and the follow-up tracked in
[GCP-1429](https://redhat.atlassian.net/browse/GCP-1429).

## 9. Risk position

Position: **the scoped identity in 8.1 is the position for the GKE bundle**, with
the residual stated. The vendor ceiling is not accepted as-is for GKE. The
register is the Confluence page
[Privileged cluster controllers on GCP HCP](https://redhat.atlassian.net/wiki/spaces/GCP/pages/521209731),
which is the platform home for this entry and for the other
cluster-admin-equivalent controllers. Team review is tracked in
[GCP-1429](https://redhat.atlassian.net/browse/GCP-1429); the decision may be
changed or reversed there. Jim raised the register location and the scoped-role
question in Slack; the record is the GCP-1429 comment that mentions him, and his
review is pending. Nothing there is agreement.

- Owner: GCP HCP GitOps workstream, assignee Roger Flores. Owned for the
  validation work from 2026-11-11. GCP-1429 is the team refinement record. This
  is the team's decision for identities it ships and operates; it is not a vendor
  certification, signature or approval, and no such claim is made.
- Ticket: GCP-1286 (this validation) and GCP-1429 (platform register and
  guardrails), with the controller-role follow-up in
  [GCP-1433](https://redhat.atlassian.net/browse/GCP-1433). No separate Jira Risk
  issue is raised. Per
  `gcp-hcp/docs/risk-tracking-process.md`, a Risk issue needs an uncertain future
  event with a trigger outside team control. "The operator ships 46 cluster
  rules" is a known state, so it is a decision recorded on the work item, not a
  Risk.
- Residual: the scoped bundle removes the manager's cluster-wide `escalate` and
  unqualified `bind`. The manager keeps `bind` on the two named ClusterRoles and
  keeps authority over the instance namespace. The application controller keeps
  the legacy wildcard policy, so a compromise of the controller is a cluster
  compromise, as it is today. The manager keeps `serviceaccounts/token` create and can bind the two named
  ClusterRoles anywhere, including the interim-parity controller role, so a
  compromise of the manager can still reach cluster-admin-equivalent in one
  step. The reduction is in standing authority, not in the ceiling.
  The wildcard stays on the disposable pair because scoped mode cannot bind a
  supplied controller role; [GCP-1433](https://redhat.atlassian.net/browse/GCP-1433)
  tracks that and the removal.
- Disposable pair: it carries only disposable credentials and
  no production data. Its management cluster keeps the Prometheus Operator CRDs
  from the GCP-1280 baseline, so the `monitoring.coreos.com` write in section 5.2
  is live there. That path stays inside the same ceiling and is accepted as
  part of the pair's baseline, not as a separate grant.
- Why accepted now: it is required to run the product at all, its delegation
  authority is now bounded to the instance plus two named ClusterRoles, and it is bounded in
  time by vendor delivery of a supported xKS packaging.
- Guardrails: no `RolloutManager` or `NamespaceManagement` resources; optional
  components stay off (GCP-1287); no third-party CRDs on clusters where this
  operator runs, other than the Prometheus Operator CRDs already in the pair's
  baseline. The register's check fails when a privileged identity is
  unregistered.
- Re-review triggers: a new operator release changes the CSV (diff the 46 rules)
  or either server policy behind the scoped ClusterRole, or the
  ApplicationSet rules the bundle now carries; a supported narrower
  role appears upstream; Prometheus Operator CRDs are
  introduced on any cluster other than the disposable pair where
  this operator runs; or the disposable validation shows a reconcile path
  failing in a way that suggests the rule set was already insufficient. The
  register also sunsets at the 1.23 xKS vendor chart or 2027-01-31, whichever is
  first.
- Mitigations already in place: the manager runs non-root, with
  `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`. Image identity is
  digest-pinned. There is no admission-time signature enforcement on GKE, which
  is B3.

## 10. Limits, and what a cluster would settle

- Which optional API groups exist on our clusters (`monitoring.coreos.com`,
  service mesh, SMI) needs a read-only `kubectl api-resources` per cluster. Not
  done here; it is a section B item.
- Whether the no-op entries in section 5.4 are truly unserved should be confirmed
  the same way.
- Operator-generated per-instance roles are not analyzed here; they are the
  subject of `docs/ARGOCD_CONTROLLER_PERMISSIONS.md`.
- No enforcement was tested. GKE has no CRI-O `ClusterImagePolicy`.

## 11. Reproduction

> **Archive reproduction note:** The historical revision below pins the
> compared bootstrap manifest, not the tooling checkout. Use development
> revision [`0ed658f60c16f19053d5c75556c5e614f57f288e`](https://github.com/floresroger/gcp-hcp/tree/0ed658f60c16f19053d5c75556c5e614f57f288e)
> for the operator chart and both test scripts referenced by these commands.
> Its `bootstrap/argocd/install.yaml` is byte-identical to the compared
> `2640be8e24b667d50003ad6a420af168e7742304` baseline (SHA-256
> `7777e7ede0b948d3338e94c0f603b441abb4b8ccee1653ae923e6eaa65fa0652`).
> The original commands are retained below; execute them from the development
> checkout, not from the historical baseline or this archive's main branch.

Run from the `gcp-hcp` repository root at revision
`2640be8e24b667d50003ad6a420af168e7742304`:

```sh
# 1. rendered operator role equals the CSV, rule for rule
helm template op tools/gitops-bootstrap/chart-src/gitops-operator \
  --namespace gitops-operator > /tmp/op.yaml

# 2. the two rule sets, side by side
yq -o=json 'select(.kind=="ClusterRole") | .rules' /tmp/op.yaml
yq -o=json 'select(.kind=="ClusterRole") | [.metadata.name, (.rules|length)]' \
  bootstrap/argocd/install.yaml

# 3. the test that asserts (1) permanently
#    (BOOTSTRAP_BASE_REF is needed because the script diffs against a PR base)
BOOTSTRAP_BASE_REF=90dfd92b5d35d8ff0a6ac3d114eabd59009ab3f4 \
  bash tools/gitops-bootstrap/tests/run-step2-tests.sh

# 4. the privileged controller register and its guardrail check
bash tools/gitops-bootstrap/tests/run-release-verification-tests.sh
```

The classification in section 5, the primitive matrix in section 6 and the
target counts were computed from those two files; the method is a set difference
over `(apiGroup, resource, verb)` tuples with wildcard rules evaluated
separately, so a wildcard is reported as a wildcard rather than expanded into
resource names.
