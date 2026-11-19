# Argo CD application-controller permissions

> **Historical source snapshot (GCP-1285).** A companion infra cleanup will
> retain the reusable permission contract at
> `gcp-hcp-infra/docs/ARGOCD_CONTROLLER_PERMISSIONS.md`; until that cleanup
> merges, infra still has the full record. This snapshot preserves the full
> reviewed matrices and runtime evidence from the disposable validation. It is
> not current operating guidance for another target.

[GCP-1285](https://redhat.atlassian.net/browse/GCP-1285) validates an opt-in,
direct-permission reduction on a disposable Dev AIO MC/region pair. The
controller reconciles Kubernetes resources; KCC, ESO, GKE and installed operators
perform their respective cloud operations. No direct controller GCP role or GSA
annotation was added. Default targets retain their existing configuration.

The owner explicitly preserved Argo's lifecycle ownership of privileged
components and selected direct-permission reduction with tracked retained
authority. This policy is not admission-enforced isolation: its named
`bind`/`escalate` exceptions and writes to its own ArgoCD CR/policy can restore
broader access indirectly.

## Reviewed requirements and evidence

- [MC resource/action matrix](argocd-controller-validation/mgt-matrix.yaml)
- [Region resource/action matrix](argocd-controller-validation/reg-matrix.yaml)
- [Sanitized runtime evidence](argocd-controller-validation/runtime.yaml)

The matrices expand the actual 14 baseline MC Applications and 22 baseline
region Applications, plus the configuration chart's replacement objects. They
include destinations, resolved source revisions, hooks, served API resource names
and scope, lifecycle verbs, and named RBAC exceptions. An Application destination
alone does not justify a RoleBinding: `gke-capacity`, for example, has no
namespaced desired resources in this pair. The region's initially unavailable
Channel/PlatformRole mapping comes from pinned Gecko private-API source
`b6237baf1c2aa11a9291a8de65ac475ad84c7177`; it is marked separately from live API
discovery. Matrices must be reviewed again for a different target or workload set.

Manifest collection used the task administrator and a local v3.5.6 CLI with only
the operator Redis Secret-name lookup corrected. That CLI is not the running
controller and supplies no runtime permission proof. Tests instead execute the
small [probe](https://github.com/openshift-online/gcp-hcp-infra/blob/d2fc1a1b35a84193d654d25fd47407b68c1bacd2/tests/argocd-controller-permissions/README.md) inside the actual
controller container and observe its real Application reconciliations.

## Identity and delegation

| Concern | MC | Region |
| --- | --- | --- |
| Kubernetes subject | `system:serviceaccount:argocd:gitops-argocd-application-controller` | Same KSA name in the independent region cluster |
| Google credential authenticated by the Kubernetes API | `serviceAccount:dev-mgt-na-ne2-rfl1285m1109.svc.id.goog[argocd/gitops-argocd-application-controller]` | `serviceAccount:dev-reg-na-ne2-rfl1285m1109.svc.id.goog[argocd/gitops-argocd-application-controller]` |
| Controller GSA annotation | Absent | Absent |
| Required direct controller GCP actions | None established for the reviewed Applications | None established for the reviewed Applications |
| Verified cloud delegate | `config-connector@dev-mgt-na-ne2-rfl1285m1109.iam.gserviceaccount.com` | `config-connector@dev-reg-na-ne2-rfl1285m1109.iam.gserviceaccount.com` |

Same-credential SelfSubjectReviews attest both the projected-token identity and
the Google metadata-credential identity. This is stronger than a metadata-email
response or an IAM binding inspection. Each cluster registration uses the local
Kubernetes endpoint with TLS settings only: no alternate bearer credential,
client key, exec provider, AWS authentication or Application plugin was found.

Actual GitOps reconciliation created a synthetic ConfigMap in `monitoring` and a
MonitoringDashboard CR in `gcp-dashboards`. KCC reported Ready, an independent
administrator confirmed the cloud dashboard existed, and Cloud Audit Logs
identify each target's Config Connector GSA on successful CreateDashboard calls.
The audit response names identify the synthetic dashboard explicitly.

Other cloud paths remain delegated: KCC actuates cloud/IAM CRs; ESO retrieves
registry/OAuth/GitHub secrets through its reviewed stores and accessor KSAs; GKE
actuates Gateway/backend resources; HyperShift and other installed workloads keep
their own identities. These responsibilities do not justify giving their IAM
roles to the application controller. The separate global controller is unchanged.

## Declarative policy and retained authority

The pinned operator provenance is GitOps source
`cf32771491be2ed7f03d43d279171d6c265f2d2b`, with argocd-operator dependency
`a213c171036a`. Its supported controller-role inputs were verified against source,
the served CRD and the running image:

1. Bootstrap owns the manager Deployment input
   `CONTROLLER_CLUSTER_ROLE=gitops-hcp-controller-local`. This replaces the
   controller's default namespace wildcard Role; aggregation alone does not.
2. The configuration chart owns the adopted ArgoCD CR and finite policy objects.
   `operator.controllerPermissions.enabled` requires operator mode and the
   reviewed ownership transition, and sets `spec.aggregatedClusterRoles=true`.
3. `localRules` supplies the ClusterRole the operator binds in `argocd`;
   `namespaceRules` supplies destination Roles/RoleBindings; `clusterRules`
   supplies the controller-only aggregation write role. Its selector labels are
   `argocd/aggregate-to-controller=true` and
   `app.kubernetes.io/managed-by=gitops`.
4. The operator owns generated roles/bindings and KSAs. Native aggregation combines
   its cache-read role with the reviewed writes. Generated resources are not
   manually patched. The server role is preserved; its hashes remained unchanged.

Chart validation rejects wildcard groups/resources/verbs/names and requires
explicit role names for RBAC `bind`/`escalate`. Cluster writes separate creation
from name-restricted update/patch/delete. Namespace writes are bounded by the
reviewed namespace and resource kind. Application status/finalizer updates and
Event recording are explicit controller control-loop requirements.

Retained permissions are deliberate exceptions, not a claim of irreducible access:

- The native cache role retains wildcard `get/list/watch`, including Secrets.
  Wildcard `get` also covers subresources authorized with that verb; it is not a
  guarantee against connect/proxy or credential access. Cache-read restriction
  needs a separate reviewed change to the cache strategy.
- Kubernetes RBAC cannot name-restrict collection `create` requests. Creation of
  the enumerated cluster resource kinds remains broader than named updates.
- Named role `bind`/`escalate`, workload/admission/KCC-IAM lifecycle writes, and
  writes to the controller's own ArgoCD CR and policy retain indirect
  administration. The matrices enumerate these exceptions.
- The operator's own CSV permissions remain a separate owner/risk-review concern.
  Its identity and permissions were not replaced with controller grants.

## IAM and state ownership

`argocd_controller_project_roles` is nullable in MC/region modules and their generic
and Dev AIO callers. Null preserves the existing workload-identity defaults;
explicit `[]` removes only the Argo GSA's project-role grants. This pair retired
both legacy `roles/container.admin` bindings. Other workload identities and
`additional_projects` are untouched; inspect those grants for every new target.

The legacy Argo GSAs and their old
`argocd/argocd-application-controller` impersonation memberships remain declared,
with no Argo project-role grants in this pair. They are not linked to the running
operator KSA. Deleting these legacy identity resources is not required to grant
the selected controller access and was not mixed into this change.

Cross-project IAM remains Terraform-owned; Kubernetes/SA policy remains with its
reviewed cluster owner. Config Connector does not compete for these Terraform
bindings. Existing ESO, KCC, operator and other KSA UIDs/GSA annotations were
unchanged across adoption and recreation; the runtime evidence lists additions
separately.

Dev AIO uses one state at prefix `dev-all-in-one/rflores-gcp1285` in the existing
Dev Terraform-state bucket. The full creation plan contained 217 new managed
resources, including 17 pair-specific additions in commons-dev. Transition plans
were reviewed in full: two Argo IAM-member deletions and the selected Config Sync
membership revisions, with no unrelated changes. A final full plan was empty.

Both memberships belong to the region fleet. One concurrent membership update
left MC on its old revision despite a successful apply; a fresh full plan exposed
and corrected that sole remaining change. Subsequent membership updates used
serial apply and were checked against both the Fleet API and live RootSync status.
An apply summary alone is not convergence evidence.

Generic MC and region callers use separate HCP Terraform states and live lookups,
not the AIO boundary. Review their workspace inputs and run dependencies before
rollout. The Integration metadata, for example, configures MC `source_workspaces`
from its region workspace. Integration/stage/production applies stay in HCP
Terraform; this proof changed only the disposable Dev pair.

## Transition and recovery

The new target is `rfl1285m1109`, sector `rflores-gcp1285`, region
`northamerica-northeast2`. GCP-1280's old pair was already destroyed and was not
reused. The owner authorized creation, validation and cleanup of this new pair.

1. Bootstrap from GCP-1280's immutable child `12673ed7208272954963cc5c029915b187e4ca63`
   through parent `89857b0971e2dc93d9a8b8a792bb06ff50bfc47d`, with the private
   configuration Application disabled. Record identity/binding baselines.
2. Release the seed through parent `6e3d9774f29188b82e98e36257c5115fe106c450` and
   child `9d695f1051f80c6d5d0d61648e13477b523e80aa`. The child selects the custom
   manager role and retains interim parity. Verify both seed inventory entries
   and Config Sync ownership markers are removed while ArgoCD UIDs are preserved.
3. Adopt the CR/policy through the private chart. Verify every replacement rule,
   namespace binding, operator RoleBinding, native aggregation and Available CR.
4. Retire the distinctly named interim ClusterRole/Binding through child
   `e4948e4` and parent `a561b3fdcfe905d6d1de8ab3485b14eface40847`. Verify live
   pruning and effective bindings, then run the actual-controller tests.

Runtime private source is pinned to
`c9ca69510ee90e0fb74092bd45ade0c7e7990004` in `floresroger/gcp-hcp-infra`; complete source
commits and URLs are in the evidence. Final bundles release the ArgoCD seed and therefore
are not a standalone cold-bootstrap entry point. Use the staged sequence above.

Recovery uses the independent test administrator, whose access is not controlled
by the controller's roles. If replacement policy prevents self-reconciliation,
repin the approved parent/child owner to the recorded parity stage, wait for
convergence, then repair/reapply the reviewed chart. Keep one active controller
set and application resources. Restore previous IAM only if the selected identity
actually requires it; a permanent blanket workaround is not the recovery result.

> **Archive recovery-completion clarification:** Repairing the chart after a
> parity-stage rollback is not completion. The independent administrator must
> restore the reviewed retirement parent
> `a561b3fdcfe905d6d1de8ab3485b14eface40847` and child
> `e4948e425d7b8fa40f971a1df99d40fa303d389f` from stage 4, verify that the
> bootstrap-owned `gitops-application-controller-interim-parity` ClusterRole
> and ClusterRoleBinding are pruned and their broad grant is absent from the
> controller's effective bindings, then rerun the scoped permission checks.
> Include the required positive operations and out-of-scope denials before and
> after controller/operator recreation. Reconciliation or readiness alone does
> not establish restored scope. These are the historical pair's retirement
> revisions; another target needs its own reviewed pins and authorization.

Bootstrap required two bounded recoveries before policy activation: stalled Dex
pods were recreated after their SAs referenced the existing registry pull Secret;
MC's same-image operator was restarted after HyperShift installed the Route API.
These did not patch permissions or identity settings. They are warm-start caveats,
not a claim of flawless cold startup or artifact certification.

## Runtime acceptance and limits

> **Archive terminology correction:** The historical wording "an unnamed
> ClusterRole patch" below means a patch to a ClusterRole outside the reviewed
> name set. The recorded denied request targets `gcp1285-outside-role`; the
> successful patch targets `gitops-hcp-controller-writes`. See `runtime.yaml`
> and the [archive review notes](README.md) for the matrix coverage limitation.

All 36 primary permission cases passed before and after operator/controller pod
recreation: required reads, cross-namespace ConfigMap creation and a named
ClusterRole patch succeeded; out-of-scope ConfigMap create/patch, an unnamed
ClusterRole patch, TokenRequest creation and unnecessary GCP GetCluster were
denied. The metadata-credential Kubernetes path also denied the out-of-scope
write. Kubernetes 403s additionally required same-credential authorizer denials;
GCP 403s required `IAM_PERMISSION_DENIED`. Authentication/service/network errors
do not count as permission denials.

The actual controller also recreated each synthetic ConfigMap after deletion
following recreation; object UIDs changed and both Applications returned to
Synced/Healthy. Before fixture pruning, all 24 region and 16 MC Applications
were Synced/Healthy, including each proof and configuration Application. GitOps
pruning then removed both synthetic workloads and cloud dashboards.
Images, KSAs, adopted ArgoCD UIDs, finite bindings and other
existing identities persisted. No static controller cloud credentials or token
values are emitted or retained in this evidence.

Organization `428383927003` denied `resourcemanager.organizations.getIamPolicy`
to the test administrator. This review covers available project/folder/GSA
policies and effective operations on named resources. It does not certify every
inherited organization grant or every GCP permission; Org Admin is unnecessary
for this disposable workflow. Production rollout, operator-RBAC redesign,
user-login authorization and artifact support/certification remain separate work.
