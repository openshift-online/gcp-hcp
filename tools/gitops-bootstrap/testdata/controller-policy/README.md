# Controller policy fixtures (FIXTURE ONLY)

These files are **not** reviewed policy for any persistent environment. They
exist so `assemble.sh --controller-policy` and the offline suites exercise a
complete, realistic policy without shipping a disposable-pair artifact as if it
were a production decision.

## Provenance

- `mc-fixture.yaml`: `openshift-online/gcp-hcp-infra`,
  `c9ca69510ee90e0fb74092bd45ade0c7e7990004`,
  `argocd/config/management-cluster/argocd-config/dev/rflores-gcp1285/northamerica-northeast2/override.yaml`,
  `.spec.source.helm.valuesObject.operator.controllerPermissions`.
- `region-fixture.yaml`: same commit, region path
  `argocd/config/region/argocd-config/dev/rflores-gcp1285/northamerica-northeast2/override.yaml`.
- Extracted with `yq -o=json '.spec.source.helm.valuesObject.operator.controllerPermissions'`,
  then re-emitted as YAML. Rule order and content are unchanged except the
  documented delta below.

### Delta applied on top of the reviewed pair policy

Verified on 2026-11-18 against the pinned chart sources (cert-manager `v1.19.1`
on global/MC, `v1.21.1` on region):

- cert-manager `startupapicheck` is a Helm `post-install` hook. Argo CD maps it
  to a `PostSync` Job `cert-manager-startupapicheck` and deletes it on success,
  so a steady-state object inventory never sees it.
- added `batch` `jobs` `get,list,watch,create,patch,delete` to the
  `cert-manager` namespace prerequisites;
- added `cert-manager-startupapicheck:create-cert` to the cert-manager `roles`
  `bind` and `escalate` `resourceNames` (the controller creates that Role and
  RoleBinding itself);
- region additionally adds `cert-manager-tokenrequest`, which the region pair
  policy omitted while MC listed it.

This delta is a source-derived gap fix, not a runtime-proven grant. It has to be
re-inventoried per real target, together with every other workload hook.

## Use

`tools/gitops-bootstrap/tests/run-step2-tests.sh` and
`run-release-verification-tests.sh` consume these files. Nothing else reads
them. Do not point a real assembly at them outside the test suite.

`namespacePrerequisites` entries are rendered into the child bundle, not into a
side file:

- `lifecycle: ensure` (default) declares the Namespace with
  `deletion: detach`, and the Role depends on `/Namespace/<ns>`.
- `lifecycle: existing` declares nothing for the namespace; use it only for
  namespaces that are always present or owned by the platform. These fixtures
  mark `default` and `kube-system` that way.

The ArgoCD seed depends on every prerequisite RoleBinding, so the order is
namespaces -> finite grants -> seed -> first workload sync.

To derive a candidate artifact for a real target, extract that target's own
reviewed policy, repeat the hook inventory, and review each namespace you would
declare with `ensure` for an existing owner. Do not copy these files.
