# GCP-1285 archive review notes

The YAML matrices and runtime record are immutable historical evidence. The
following correction was verified offline on 2026-11-18; it does not change
the original collection or establish additional runtime permission results.

## Probe build-to-record hash join

In [`runtime.yaml`](argocd-controller-validation/runtime.yaml), match each
permission row's `probeSha256` to the corresponding
`source.probeBuilds.<build>.binarySha256`. The build metadata's `recordScope`
uses the name `binarySha256`, but that field is not present on the individual
permission rows. There are 16 hashed rows in each before/after collection;
the remaining metadata-credential rows and separate identity records retain the
provenance limits stated in the YAML. Missing hashes have not been backfilled.

## MC and regional cert-manager hook inventory gaps

The [MC matrix](argocd-controller-validation/mgt-matrix.yaml) records
cert-manager `v1.19.1`; the
[regional matrix](argocd-controller-validation/reg-matrix.yaml) records
`v1.21.1`. Both omit startup-check hooks. The baseline Applications
([MC](https://github.com/openshift-online/gcp-hcp-infra/blob/d916de0d1ba274254c96a6b2338ea680f607a1b5/argocd/config/management-cluster/cert-manager/template.yaml),
[region](https://github.com/openshift-online/gcp-hcp-infra/blob/d916de0d1ba274254c96a6b2338ea680f607a1b5/argocd/config/region/cert-manager/template.yaml))
set `installCRDs=true` and `global.leaderElection.namespace=cert-manager`;
neither disables `startupapicheck`. Both rendered Applications at recorded
source revision `0efdd1b016f6768f23777d48c799854de703954f` have those same inputs.

Rendering the publisher's charts with those inputs produces the same four
`post-install` hooks in `cert-manager` for each version:

- [MC chart v1.19.1](https://charts.jetstack.io/charts/cert-manager-v1.19.1.tgz),
  SHA-256 `f72a727b1749df3521e7a65af5f18505f93b572d758841890270b150344b2b41`.
- [Regional chart v1.21.1](https://charts.jetstack.io/charts/cert-manager-v1.21.1.tgz),
  SHA-256 `c27101f3f3e2349fb4a9e704316105bf7b52ad73b8c8257d3498ef7f2f6a4adc`.

| API resource | Name |
| --- | --- |
| `batch/v1` Job | `cert-manager-startupapicheck` |
| `v1` ServiceAccount | `cert-manager-startupapicheck` |
| `rbac.authorization.k8s.io/v1` Role | `cert-manager-startupapicheck:create-cert` |
| `rbac.authorization.k8s.io/v1` RoleBinding | `cert-manager-startupapicheck:create-cert` |

```sh
# Substitute cert-manager-v1.19.1.tgz for the MC reproduction.
helm template cert-manager /path/to/cert-manager-v1.21.1.tgz \
  --namespace cert-manager --include-crds \
  --set installCRDs=true --set global.leaderElection.namespace=cert-manager
```

These objects are missing from both matrices' named resource rows. A complete
replacement inventory must include their lifecycle operations and hook cleanup
(`before-hook-creation,hook-succeeded`) in the target's permission review.
The retained evidence does not establish whether the original collector skipped
hooks or whether the scoped controller exercised their lifecycle successfully.
Do not infer that the hooks were disabled or that the archived matrix is an
exhaustive permission specification. The original YAML and its source hash are
preserved rather than retroactively adding unobserved grants or test results.
