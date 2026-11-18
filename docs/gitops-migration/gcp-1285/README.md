# GCP-1285 archive review notes

The YAML matrices and runtime record are immutable historical evidence. The
following correction was verified offline on 2026-11-18; it does not change
the original collection or establish additional runtime permission results.

## Regional cert-manager hook inventory gap

The [regional matrix](argocd-controller-validation/reg-matrix.yaml) records
cert-manager `v1.21.1` but omits its startup-check hooks. The
[baseline Application](https://github.com/openshift-online/gcp-hcp-infra/blob/d916de0d1ba274254c96a6b2338ea680f607a1b5/argocd/config/region/cert-manager/template.yaml)
sets `installCRDs=true` and `global.leaderElection.namespace=cert-manager`;
it does not disable `startupapicheck`. The rendered Application at recorded
source revision `0efdd1b016f6768f23777d48c799854de703954f` has those same inputs.

Rendering the [publisher's chart archive](https://charts.jetstack.io/charts/cert-manager-v1.21.1.tgz)
(SHA-256 `c27101f3f3e2349fb4a9e704316105bf7b52ad73b8c8257d3498ef7f2f6a4adc`)
with those inputs produces these `post-install` hooks in `cert-manager`:

| API resource | Name |
| --- | --- |
| `batch/v1` Job | `cert-manager-startupapicheck` |
| `v1` ServiceAccount | `cert-manager-startupapicheck` |
| `rbac.authorization.k8s.io/v1` Role | `cert-manager-startupapicheck:create-cert` |
| `rbac.authorization.k8s.io/v1` RoleBinding | `cert-manager-startupapicheck:create-cert` |

```sh
helm template cert-manager /path/to/cert-manager-v1.21.1.tgz \
  --namespace cert-manager --include-crds \
  --set installCRDs=true --set global.leaderElection.namespace=cert-manager
```

These objects are missing from the matrix's named resource rows. A complete
replacement inventory must include their lifecycle operations and hook cleanup
(`before-hook-creation,hook-succeeded`) in the target's permission review.
The retained evidence does not establish whether the original collector skipped
hooks or whether the scoped controller exercised their lifecycle successfully.
Do not infer that the hooks were disabled or that the archived matrix is an
exhaustive permission specification. The original YAML and its source hash are
preserved rather than retroactively adding unobserved grants or test results.
