# Upstream issue: GitOps manager does not detect Route CRD that appears after startup (GCP-1441)

> Status: filed as [redhat-developer/gitops-operator#1566](https://github.com/redhat-developer/gitops-operator/issues/1566) on 2026-11-16. This file is kept as the filed copy; the issue is the source of truth for follow-up.

## Summary

On a GKE MC where HyperShift is installed via GitOps, the manager starts before `routes.route.openshift.io` exists. It registers Route type only at startup, so never sees the API and every Route reconcile fails until restart. This ordering is normal for MCs that install HyperShift via Argo CD, not an edge case.

## Observed versions

* Product: OpenShift GitOps 1.22.2, release 7
* Advisory: RHBA-2026:83602 (2026-10-15)
* Bundle: `registry.redhat.io/openshift-gitops-1/gitops-operator-bundle@sha256:2ba58c50f771c08bbc5a1d8572c7b058df55bc71667cc66026d42840eb4bee6d`
* CSV: `openshift-gitops-operator.v1.22.2`
* Operator source: `redhat-developer/gitops-operator` at `cf32771491be2ed7f03d43d279171d6c265f2d2b`
* Downstream source: `rh-gitops-midstream/release` at `219dafaaf0e9431f8b468b4932f337f3ce397058`
* Route/Scheme fix included in this release: GITOPS-11466, original PR #1321, release backport PR #1354 (`139db716ae65182624429cc1d78cd5a1c625f15c`)
* GKE: `1.36.4-gke.2377000` on Autopilot, `REGULAR` channel
* Config Sync: `1.23.3`
* Argo CD: installed by Fleet Config Sync from the public bootstrap repo, then driving `argocd/rendered/management-cluster/...`

Repository context: `docs/gitops-release-validation/2026-11-13-dev-pair.md` section 5.3 and `docs/GITOPS_RELEASE_VALIDATION.md`. The 2026-11-13 pair was `rfl1286m1111` in `us-central1` with parent Config Sync revision `7982574b93110101dd29f4260fe5306a28efc9c6` and child `04f6e7d249352ca7813adccd3fda9e74ae8da113`.

## Timeline observed on 2026-11-13 (GKE)

All times UTC:

* 16:00:03Z: GitOps manager pod started in namespace `gitops-operator` on the management cluster.
* 16:46:57Z: HyperShift CRDs landed on the MC API server after the root ApplicationSet synced management-cluster applications.
* 16:47:00Z: `routes.route.openshift.io` CRD created.
* 16:47:11Z to 17:08Z: `ArgoCD/gitops` reported `phase=Failed` with `Reconciled=False`. Operator logs showed the scheme error on every Route reconcile.
* 17:08Z: manager pod deleted manually. Replacement manager started with the Route API present and reported `phase=Available` and `Reconciled=True`. No Route object was created because every route switch in the instance spec is disabled.

This is documented in `docs/gitops-release-validation/2026-11-13-dev-pair.md` section 5.3.

## Error

```
no kind is registered for the type v1.Route in scheme "pkg/runtime/scheme.go:111"
```

The error repeated on every reconciliation attempt until the manager was restarted. After restart the instance reached Available and Reconciled=True.

## Source reference in this release

At the released commit `cf327714` the manager registers the Route type only at startup:

* `cmd/main.go` calls `routev1.AddToScheme` only when `util.IsRouteAPIFound()` returns true
* `InspectCluster` runs once at startup

GITOPS-11466 (PR #1354, commit `139db716ae65182624429cc1d78cd5a1c625f15c`) makes Route detection independent of the Config API. That change fixes the case where a manager starts after the Route CRD exists but the Config API is absent. It does not cover a Route API that appears after startup.

Because the MC installs HyperShift through GitOps, the manager always starts before HyperShift creates the Route CRD, so the current code hits this path on every fresh MC bring-up.

## Expected behavior

Manager should handle Route CRD appearing after startup. Possible approaches:

* re-run discovery periodically/on CRD watch, add Route to the scheme, and
  start the Route reconciliation path when `routes.route.openshift.io` appears,
  or
* watch the CRD instead of a one-time startup check

Adding the type to the scheme alone is not sufficient if the Route controller
setup is also gated on startup discovery.

No claim is made about which approach upstream prefers or has committed to.

## Workarounds in gcp-hcp-infra

Either workaround restores the manager without code change:

1. Install the Route CRD before the GitOps manager starts.
2. Restart the manager once `routes.route.openshift.io` exists.

For live verification on a management cluster, set the cluster context in `CTX` and the operator namespace in `OPERATOR_NS` (`gitops-operator` in the validated pair):

Check that the CRD is present:

```sh
kubectl --context "$CTX" get crd routes.route.openshift.io
```

Expected when present: one row for `routes.route.openshift.io`. The `CREATED AT`
value differs per cluster; it was `2026-11-13T16:47:00Z` on the observed pair.

```
NAME                        CREATED AT
routes.route.openshift.io   <time the CRD was created>
```

Expected when absent:

```
Error from server (NotFound): customresourcedefinitions.apiextensions.k8s.io "routes.route.openshift.io" not found
```

Alternative check via API discovery:

```sh
kubectl --context "$CTX" api-resources --api-group=route.openshift.io
```

Expected when present:

```
NAME   SHORTNAMES   APIVERSION              NAMESPACED   KIND
routes   route      route.openshift.io/v1   true         Route
```

Expected when absent: no resources listed.

Wait until discovery lists the Route API, not just the CRD. Discovery can lag
CRD creation:

```sh
until kubectl --context "$CTX" api-resources --api-group=route.openshift.io | grep -q '^routes '; do sleep 5; done
```

`rollout status` alone is not proof: the replacement manager checks discovery
at startup, so starting it during the lag can miss Route again. Then restart
the manager:

```sh
kubectl --context "$CTX" -n "$OPERATOR_NS" rollout restart deployment/openshift-gitops-operator-controller-manager
```

Expected:

```
deployment.apps/openshift-gitops-operator-controller-manager restarted
```

Wait for rollout:

```sh
kubectl --context "$CTX" -n "$OPERATOR_NS" rollout status deployment/openshift-gitops-operator-controller-manager
```

Expected:

```
deployment "openshift-gitops-operator-controller-manager" successfully rolled out
```

The same recovery was observed by deleting the manager pod at 17:08Z; the replacement came up with the Route API present and the instance became Available.

## References in this repository

> **Archive location note:** The original paths below are preserved from the
> filed report. Their current locations are the
> [pair record](2026-11-13-dev-pair.md) (section 5.3),
> [release-validation record](../GITOPS_RELEASE_VALIDATION.md) (section 3a), and
> [infra bootstrap flow](https://github.com/openshift-online/gcp-hcp-infra/blob/d2fc1a1b35a84193d654d25fd47407b68c1bacd2/docs/BOOTSTRAP_FLOW.md).
> The bootstrap flow stays in `gcp-hcp-infra`; the other records moved together
> under `gcp-hcp/docs/gitops-migration/gcp-1286/`.

* `docs/gitops-release-validation/2026-11-13-dev-pair.md` section 5.3: full timeline and operator log excerpt
* `docs/GITOPS_RELEASE_VALIDATION.md`: runbook with verification steps and known trap entry for GCP-1441
* `docs/BOOTSTRAP_FLOW.md`: overall bootstrap flow showing Config Sync installs Argo CD, then Argo CD deploys HyperShift in wave -5 on MC

## Filing status

Filed as [redhat-developer/gitops-operator#1566](https://github.com/redhat-developer/gitops-operator/issues/1566) on 2026-11-16. Update both this copy and the issue if the operator source moves past `cf327714` or if GITOPS-11466 is superseded.
