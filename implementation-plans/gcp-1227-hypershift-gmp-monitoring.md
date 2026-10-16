# GCP-1227: GMP-Native Monitoring Support in HyperShift — Implementation Plan

**Jira**: [GCP-1227](https://redhat.atlassian.net/browse/GCP-1227) (Story) · Epic [GCP-423](https://redhat.atlassian.net/browse/GCP-423)

**Design decision**: [hypershift-gmp-native-monitoring](../design-decisions/observability/hypershift-gmp-native-monitoring.md)

**Target repository**: `github.com/openshift/hypershift` (upstream — requires HyperShift maintainer review/sign-off)

> **Status**: Proposed runtime design pending maintainer sign-off. GCP-1292 foundation, including the reproducible API maintenance workflow, is implemented and validated locally on `acwalczyk/GCP-1292-monitoring-api-group-plumbing`; validation evidence and limitations are recorded in the HyperShift working plan `gmp-native-monitoring-revised-plan.md`. Phase 0 remains historical two-target feasibility evidence. Runtime reconciliation and the operator etcd/KAS slice are pending. Older line-number anchors below are historical and can drift.

## Overview

Add a monitoring API-group selection to HyperShift so the control-plane-operator (CPOv2) and `hypershift install` can emit GMP resources (`monitoring.googleapis.com`) instead of CoreOS resources (`monitoring.coreos.com`), gated by a `MONITORING_API_GROUP` environment variable. The CoreOS default and the existing `RHOBS_MONITORING=1` path stay byte-for-byte unchanged. GMP resources are authored as separate, hand-maintained manifests/constructors with runtime resource selection still to be designed. A coverage-based parity test prevents a CoreOS scrape target from being added without matching GMP coverage (the CoreOS→GMP mapping is not 1:1 — see Phase 7).

### Goals

- Backend selection retains the agreed MONITORING_API_GROUP management-deployment environment interface; GMP registration is additive and independent of selection.
- GMP coverage for all current monitors — **11 ServiceMonitors + 8 PodMonitors + 1 PrometheusRule** (CPO level) and **2 management-level monitors + 2 PrometheusRules** (install level). Note the mapping is **not 1:1**: GMP has no ServiceMonitor kind, so both `ServiceMonitor` and `PodMonitor` collapse onto `PodMonitoring`, and monitors targeting the same pods may be consolidated into a single `PodMonitoring` with multiple `endpoints`. The goal is that every CoreOS scrape *target* has GMP coverage, not that there are 20 matching GMP files.
- Preserve MetricSet (Telemetry/SRE/All) filtering by reusing `support/metrics/sets.go`, translating output into GMP relabel types.
- GMP-gated NetworkPolicy allowing `gke-gmp-system` collector ingress.
- Fail-fast validation when `MONITORING_API_GROUP=monitoring.googleapis.com` is combined with `RHOBS_MONITORING=1`.
- Unit + repo-level parity tests covering both selections, backward compatibility, MetricSet translation, and authenticated targets.

### Non-Goals (this story)

- GKE/GMP collector deployment or tuning; live-cluster scrape validation; cost/quota analysis (follow-up ops story).
- Removing the dedicated Prometheus from gcp-hcp-infra (follow-up after cutover).
- Refactoring the RHOBS `init()` mechanism or adding `monitoring.rhobs` as a third `MONITORING_API_GROUP` value (possible future work, out of scope here).
- Any change to ROSA HCP / ARO HCP behavior, or to guest-cluster workload monitoring.

## POC Path (proof-of-concept, before committing to full delivery)

The full plan covers all 24 current CoreOS monitoring resources (20 CPO + 4 install), verified at HyperShift revision 0bb9d28005080a6fc14ce3314ad5b722193f2281 — the GMP side will be *at most* 24 manifests and likely fewer, since `ServiceMonitor`/`PodMonitor` both collapse to `PodMonitoring` and same-pod targets can be consolidated. The POC exists to **prove the two real unknowns cheaply** — (a) can GMP collectors actually scrape the authenticated targets, and (b) does the configuration propagation and the selected monitoring reconciliation path work end-to-end — before investing in all the GMP equivalents, the parity test, and docs. Everything below is deliberately narrowed to **etcd + kube-apiserver only** (the two hardest targets, both mTLS client-cert).

The POC runs as **two decoupled tracks in parallel**:

### Track A — Auth/scrape spike (infra, BLOCKING, no repo changes)

This is **Phase 0** and gates everything. It does not touch the HyperShift repo — it needs a live GKE management cluster with `gke-gmp-system` collectors.

> **Status: COMPLETE — passed 2026-10-08.** etcd and kube-apiserver were scraped natively by `gke-gmp-system` collectors (`up=1`) on `dev-mgt-us-e4-dw0928-gke`. Both use mTLS client certs (no bearer token). ConfigMap-sourced CAs were mirrored into CA-only Secrets and the collector ServiceAccount was granted read via a namespaced Role/RoleBinding. Track B (the code slice) was **not** run — the foundation in Phases 1–2 is now implemented locally; the runtime slice and Phases 3–8 remain pending.

1. Hand-write plain-YAML `PodMonitoring` for **etcd** (mTLS: `tls.cert`/`tls.key`/`tls.ca` as Secret refs) and **kube-apiserver** (mTLS: `metrics-client` cert/key + `root-ca` CA as Secret refs) — no Go, no typed structs.
2. Apply against a real GKE cluster and confirm the collectors scrape both targets.
3. Determine the `gke-gmp-system` collector namespace/pod-label selector for the NetworkPolicy (needed by Track B step 5 and later Phase 6).
4. Confirm any ConfigMap-sourced CA bundle has a Secret-based equivalent (GMP TLS supports Secret refs only).

**Exit criteria**: etcd and kube-apiserver both scraped, or a documented alternate path for anything that can't be. Record findings on GCP-1227. **Do not start Track B's resource step (3) until this passes** — a failure here changes the design.

### Track B — Vertical code slice (repo, parallel with Track A)

The next code slice uses the local API-only foundation and is limited to etcd and kube-apiserver. The GCP-1292 foundation supplies enum/conflict validation, both Deployment environment hops, reconciler/context threading, data types and serialization support. It emits no GMP monitoring resources yet.

1. Select and test the reconciliation architecture before adding assets. Backend-aware per-workload handling and centralized GMP reconciliation are candidates; neither is approved or implemented by the foundation.
2. Define inactive-backend exclusion before decoding and before update/delete/status/watch operations. Monitoring errors must be visible and retried without stopping core workloads; explicitly define HCP availability/upgrade policy, ownership-checked cleanup in both directions, and recovery after CRD/RBAC repair.
3. Implement the Phase 4 writer RBAC prerequisite for the CPO and the HyperShift operator's authority to grant it, then add typed etcd/KAS resources using `support/gmpmonitoring`, with the validated pod discovery, ports, TLS client credentials and CA-only Secrets.
4. Preserve MetricSet behavior through a tested relabel translator, rejecting unsupported actions and protected labels.
5. Implement GMP-only collector Secret access and NetworkPolicy, credential/CA rotation triggers, and tests of both backend transitions.
6. Validate the resulting operator path and live scrape evidence under GCP-1293/GCP-1295. Historical Phase 0 manual YAML does not prove these operator behaviors.

### Explicitly NOT in the POC (deferred to full delivery)

- The other 18 CPO resources and all 4 install-level resources — 2 monitors + 2 rules (Phase 5).
- Full Google operator-module vendoring or Prometheus replacement. The local API-only package supplies typed resources without either.
- The parity test + CI guard (Phase 7) — meaningless until most GMP equivalents exist.
- Documentation (Phase 8).
- Fleet-wide/all-target NetworkPolicy wiring (Phase 6) beyond the two-target Track B slice.

### POC exit criteria

- etcd and kube-apiserver GMP `PodMonitoring` are scraped by GMP collectors on a live GKE cluster (Track A evidence on GCP-1227).
- With `MONITORING_API_GROUP=monitoring.googleapis.com`, the CPO emits GMP `PodMonitoring` for etcd/KAS and **omits** the CoreOS `ServiceMonitor`; with the default (unset) env var, output is byte-for-byte the CoreOS behavior.
- RHOBS + env-var conflict fails at startup.
- MetricSet filtering (Telemetry/SRE/All) is preserved for etcd/KAS through the translator.

Passing the thin slice proves those two targets only. Audit the remaining discovery/auth/MetricSet and rule-scope differences before expanding; complete semantic parity, capacity, mixed-CPO compatibility and rollback gates before cutover.

## Current-State Reference (verified 2026-09-23)

| Area | Location | Notes |
|------|----------|-------|
| Scheme registration | `support/api/scheme.go:141-148` | `init()` reads `RHOBS_MONITORING`; picks rhobs vs coreos types. `AllMonitoringScheme` registers both (`:143-144`). |
| RHOBS scheme | `support/rhobsmonitoring/scheme.go` | group `monitoring.rhobs/v1`; `EnvironmentVariable = "RHOBS_MONITORING"`. |
| Install RHOBS flag | `cmd/install/install.go:345-346` | `--rhobs-monitoring` validated against the env var. |
| Management monitors | `cmd/install/assets/hypershift_operator.go` | `ExternalDNSPodMonitor.Build()` :1219, `HyperShiftServiceMonitor.Build()` :1840, `HypershiftRecordingRule.Build()` :1871, `HypershiftAlertingRule.Build()` :1891 (all Go-constructed). |
| CPO SM assets (11) | `v2/assets/{component}/servicemonitor.yaml` | etcd, kube-apiserver, kube-controller-manager, kube-scheduler, openshift-apiserver, openshift-controller-manager, openshift-route-controller-manager, cluster-version-operator, cluster-node-tuning-operator, catalog-operator, olm-operator. |
| CPO PM assets (8) | `v2/assets/{component}/podmonitor.yaml` | hosted-cluster-config-operator, cluster-autoscaler, karpenter-operator, karpenter, cluster-image-registry-operator, ingress-operator, ignition-server, control-plane-operator. |
| CPO recording rules (1) | `v2/assets/kube-apiserver/prometheus-recording-rules.yaml` | PrometheusRule. |
| Component framework | `support/controlplane-component/builder.go:69-90` | `WithAdaptFunction`, `WithPredicate`, `WithManifestAdapter`. |
| Predicate precedent | `v2/cloud_controller_manager/azure/component.go:55-57,92` | `isAroHCP` gating `config-secretprovider.yaml`. |
| Asset embed | `v2/assets/assets.go:20` | `//go:embed */*.yaml`. |
| MetricSet configs | `support/metrics/sets.go:22-26` (sets), `:139` (`EtcdRelabelConfigs`), `:157` (`KASRelabelConfigs`) | return `[]prometheusoperatorv1.RelabelConfig`. |
| NetworkPolicy | `hypershift-operator/controllers/hostedcluster/network_policies.go:95-99` (call), `:576-593` (`reconcileOpenshiftMonitoringNetworkPolicy`, monitoring label `:583`) | allows ns label `network.openshift.io/policy-group: monitoring`. |
| Monitoring label helper | `support/metrics/labels.go:8-10` | `HyperShiftMonitoringLabel`. |
| CPO CLI | `control-plane-operator/main.go:321-361` | `NewStartCommand`, flags block. |
| Install options | `cmd/install/install.go:87-149` | `Options` struct (`RHOBSMonitoring` :142, `PlatformMonitoring` :103, `MetricsSet` :139). |
| GMP CRD Go types | `support/gmpmonitoring` | Local foundation snapshot: v0.17.2 / `058752ec791edefba5c42486eb9237562c3f0d29`, 29 data types including PodMonitoring, Rules, their lists and required nested spec/status types. Current source pin is maintained in `hack/gmp-api-source/go.mod`; generated `upstream.sha256` records provenance. No full operator dependency or existing-version changes. |
| `AllMonitoringScheme` | `support/api/scheme.go` | Additive GMP types alongside CoreOS; RHOBS additionally registered when RHOBS is selected. A decode helper, not the isolation mechanism. |

## Architecture

The full-delivery topology below remains proposed. The foundation supplies install/CPO validation and Deployment/context propagation; resource emission and NetworkPolicy selection are pending:

```
MONITORING_API_GROUP=monitoring.googleapis.com
      │
      ├── hypershift install ──► management-level monitors (hypershift ns)
      │        Go constructors in cmd/install/assets/hypershift_operator.go
      │        + Options field (read from env) + startup validation
      │
      ├── hypershift-operator ──► NetworkPolicy (per-HCP ns)
      │        GMP-gated ingress from gke-gmp-system
      │
      └── control-plane-operator ──► per-HCP CPOv2 monitors
               env var → MonitoringAPIGroup on contexts
               → reconciliation architecture pending
```

### Key design choices (from the design decision)

1. **Activation**: retain the agreed MONITORING_API_GROUP management-deployment environment interface; GMP registration is additive and independent of selection.
2. **Scheme**: register GMP types unconditionally and additively in Scheme and AllMonitoringScheme. Retain existing CoreOS/RHOBS registration. Runtime resource selection and lifecycle isolation are separate design requirements.
3. **Manifests**: separate GMP assets/constructors, never runtime conversion.
4. **Selection**: runtime architecture is unresolved. Per-manifest predicates run after manifest loading and false predicates initiate deletion; they do not alone provide pre-decode selection, failure containment or safe transitions.
5. **MetricSet reuse**: GMP adapt functions call existing `metrics.*RelabelConfigs()` and translate the returned `[]prometheusoperatorv1.RelabelConfig` into GMP relabel types — no duplicated allow/drop lists.

## Phased Delivery

### Phase 0 — Feasibility Spike (COMPLETE — passed 2026-10-08)

The highest-risk unknown is authenticated scraping. Build a throwaway GMP `PodMonitoring` for **etcd** (mTLS, client certs) and **kube-apiserver** (mTLS, client certs) against a real GKE management cluster with GMP collectors and confirm the collectors actually scrape them.

> **Status: COMPLETE — passed 2026-10-08** on `dev-mgt-us-e4-dw0928-gke`. Both etcd and kube-apiserver were scraped natively by `gke-gmp-system` collectors (`up=1`). Findings below confirmed; see GCP-1227.

- **Confirmed**: GMP collectors can present etcd client certificates (`tls.cert`/`tls.key` as Secret refs) — no proxy/sidecar needed.
- **Confirmed**: kube-apiserver uses mTLS client-certificate auth (`metrics-client` cert/key, `root-ca` trust bundle) — not a bearer token.
- **Confirmed**: ConfigMap-sourced CA bundles (`root-ca`, `etcd-ca`) must be mirrored into CA-only Secrets for GMP (GMP TLS supports Secret refs only), and the collector ServiceAccount needs a namespaced `Role`/`RoleBinding` to read them (see Phase 4.5).
- **Confirmed**: the `gke-gmp-system` collector peer is namespace `gke-gmp-system` + pod label `app.kubernetes.io/name=collector`.

**Exit criteria (met)**: etcd and kube-apiserver both scraped by GMP collectors. Findings recorded on GCP-1227.

### Phase 1 — Env Var, Options, Scheme, Validation (local foundation implemented)

The existing environment/context plumbing is retained. `support/metrics` supplies the CoreOS/GMP constants, `ParseMonitoringAPIGroup` and `MonitoringAPIGroupFromEnv`. Install validates explicit option values and environment-derived values, rejects GMP with `RHOBS_MONITORING=1` even without the CLI flag, and also rejects the flag/GMP conflict. CPO startup uses the same parser and conflict check.

The value flows through `HostedControlPlaneReconciler`, `ControlPlaneContext` and `WorkloadContext`. Install forwards GMP to the HyperShift operator Deployment; its CPO deployment adapter forwards the environment value to CPO. Both hops omit an unset or explicit CoreOS value so default/RHOBS output remains unchanged. HyperShift-operator startup selection for NetworkPolicy remains Phase 6 work.

GMP type registration is unconditional and additive in both `Scheme` and `AllMonitoringScheme`. Existing CoreOS/RHOBS initialization is retained; registration starts no clients or watches. The unused `isGMP`/`isNotGMP` helpers were removed. Selection helpers belong with the eventual reconciliation design, where callers and lifecycle semantics exist.

Tests cover parser defaults/errors, both install rejection regressions, RHOBS compatibility, both Deployment hops and process initialization. Validation results and environmental limitations are in the HyperShift working plan; this is local work, not merged or deployed behavior.

### Phase 2 — Maintained GMP API-only package (local foundation implemented)

`support/gmpmonitoring` contains upstream-derived PodMonitoring, Rules, their lists and required nested spec/status data. The source version is selected in the download-only `hack/gmp-api-source/go.mod`; its `go.sum` verifies module inputs, and generated `support/gmpmonitoring/upstream.sha256` records the resolved version, immutable upstream commit, module checksums and input file checksums. The snapshot is a schema/serialization reference, not blanket GKE production compatibility.

`hack/update-gmp-api.sh` downloads the selected source in isolation. The Go AST extractor discovers upstream API source files, follows the supported type closure and rejects unexpected runtime imports. It preserves upstream headers, license, comments, JSON tags and generation markers, copies pinned CRD fixtures and regenerates deepcopies using the repository's pinned controller-gen. Operator methods, admission, Secret resolution, collector/exporter runtime and Prometheus config construction are excluded. HyperShift's root module, existing dependency versions, vendor directory and Prometheus replacement remain unchanged. ClusterPodMonitoring, ClusterRules and GlobalRules remain deferred pending an explicit discovery or rule-scope requirement.

#### Maintenance ownership and workflow

The GCP HCP team owns the snapshot, extraction tooling and compatibility validation. `gcp-reviewers` review package and tooling changes through the normal HyperShift review process. The canonical runbook is HyperShift's `support/gmpmonitoring/README.md`, currently part of the local GCP-1292 change; use the published implementation revision when available. Keep detailed operational instructions there rather than maintaining a second runbook here.

From the HyperShift repository root, with Go, Python 3 and Make available:

```sh
make gmp-sync
make verify-gmp-sync
```

An unchanged source pin must reproduce the snapshot without differences. Verification regenerates into temporary storage and fails on missing, different or unexpected generated files without rewriting tracked files. CI runs verification when GMP inputs, tooling, outputs or relevant dependency/tool versions change. Ordinary `make generate` continues to regenerate deepcopies offline.

For a source update, review relevant API/schema changes for the supported environments, change the prometheus-engine requirement in `hack/gmp-api-source/go.mod`, regenerate, verify and run the focused tests documented in the canonical runbook. Review the generated diff and include compatibility evidence in the update PR. Expected changes are limited to the source pin/checksums, provenance, API types, deepcopies, license and schema fixtures. Do not hand-edit generated files; unexpected runtime dependencies or extraction failures require owning-team review. How candidate updates are detected remains open; no fixed review cadence or release-monitoring automation is established. A new operator or collector release alone does not require a snapshot update.

#### Validation and compatibility

Local validation covers reproducible regeneration from existing and fresh module caches, verification rejecting altered/missing/unexpected generated files and missing input checksums, and an isolated source-pin update exercise. Focused race tests, the full unit suite and lint passed locally. This records local foundation evidence, not merged or deployed behavior.

Package tests exercise actual HyperShift serializers, objects and lists with spec/status, nested deepcopy isolation, fresh-process registration and emitted examples against pinned v1 OpenAPI schemas with negative controls. Fixture CRDs are test inputs; HyperShift does not install them.

The copied source version does not need to numerically match the deployed GMP image. Compatibility depends on the fields HyperShift emits and the schemas and behavior supported by deployed components. Record the GKE version, GMP operator image and validation performed for the intended environments. Offline schema checks do not execute CEL or Google admission/runtime checks, and do not establish live admission, scraping or rule-evaluation compatibility. The supported GKE/GMP matrix and live validation remain later gates.

### Runtime design gate (before Phases 3–8)

Resolve backend-aware per-workload handling versus centralized GMP reconciliation with executable tests. A centralized component is an option, not a confirmed architecture: ordinary ControlPlaneComponent status aggregation can affect HCP availability and rollout completion. Per-manifest predicates alone do not isolate inactive assets or contain errors. Define selection before decoding, all API/watch/status paths, monitoring retry/reporting, availability/upgrade policy, ownership-checked transitions in both directions, CA/client rotation and older-CPO support before expanding assets.

**Transition ordering (verify-before-drop)**: a backend switch must create (and, where feasible, confirm scraping of) the new backend's owned resources **before** removing the old backend's owned resources, so a cluster is never left without coverage mid-switch. This matters during cutover specifically: while the dedicated Prometheus shim is still running, it covers a cluster only through that cluster's CoreOS monitors, so removing those monitors the instant GMP is selected drops the cluster from shim coverage before GMP collection is proven. Make this ordering an explicit requirement of the chosen transition design, not an implicit side effect of reconcile order.

The remaining phases below describe desired outcomes, not implemented runtime behavior. Predicate references are contingent on this gate and may need framework changes.

### Phase 3 — MetricSet → GMP Relabel Translation Layer

A small, well-tested translator is the reusable core the manifests depend on.

1. New function, e.g. `metrics.ToGMPRelabeling([]prometheusoperatorv1.RelabelConfig) ([]gmpmonitoring.RelabelingRule, error)` mapping each field (`sourceLabels`, `targetLabel`, `regex`, `action`, `replacement`, `separator`, `modulus`).
2. Verify all actions actually used by support/metrics/sets.go (currently keep/drop). Test any additional supported GMP actions against the schema; reject labelmap, unsupported actions and attempts to overwrite protected labels with explicit errors.
3. GMP adapt functions call the existing per-component `metrics.*RelabelConfigs(set)` (e.g. `EtcdRelabelConfigs`, `KASRelabelConfigs`) and pass the result through the translator — no reimplementation of allow/drop lists.

**Tests**: table-driven translation tests per action; round-trip check that Telemetry/SRE/All produce equivalent keep/drop semantics against GMP types for etcd and kube-apiserver.

### Phase 4 — CPO-Level GMP Manifests (per component)

After the runtime design gate, add GMP coverage for each current CPO monitor. ServiceMonitor and PodMonitor both map to PodMonitoring, and same-pod endpoints may be consolidated only when discovery/ports/auth remain equivalent. Asset names such as gmp-podmonitoring.yaml are examples; the selected lifecycle design determines their reconciliation path.

- **Writer RBAC prerequisite**: grant the CPO namespaced access to `monitoring.googleapis.com` resources `podmonitorings` and `rules`, with `get`, `list`, `watch`, `create`, `update`, `patch` and `delete`. Update the CPO Role (`v2/assets/control-plane-operator/role.yaml` and its adapter) and the HyperShift operator ClusterRole (`cmd/install/assets/hypershift_operator.go`) so the operator has the corresponding authority to grant that Role without broad `escalate` permissions. The collector Secret reader Role in Phase 4.5 serves a different principal and does not authorize these writes. Extend permissions for cluster/global rule resources only if the Phase 5 scope audit requires them.
- **Permission lifecycle**: under the selected backend-transition design, establish writer permissions before creating GMP resources. When returning to CoreOS, retain the CPO permissions and the HyperShift operator's grant authority until owned GMP resources have been cleaned up, then restore the steady CoreOS policy. Default/RHOBS steady-state permissions remain unchanged; do not revoke access before cleanup completes.
- Add `gmp-podmonitoring.yaml` (translating a `ServiceMonitor` **or** `PodMonitor` — both become `PodMonitoring`; a ServiceMonitor's Service→endpoints discovery must be re-expressed as a pod label selector that reproduces the same target set) and `gmp-rules.yaml` (from the recording-rules PrometheusRule) alongside existing assets; `//go:embed */*.yaml` makes assets available, but inactive-backend loading and API/status/watch isolation still require the selected runtime design.
- **Order of implementation** (highest risk first, per Phase 0): etcd → kube-apiserver → remaining ServiceMonitor components → PodMonitor components → recording rules.
- Each adapt function: translate discovery (Service→pod label selector — audit that pod labels reproduce the same target set), ports, `tlsConfig`→`tls` (Secret refs), auth, cluster-identity labels, and MetricSet relabelings (via Phase 3 translator).
- Per-target audit checklist recorded on the ticket: discovery mapping, port naming, TLS/auth, relabelings, cluster-identity labels.

**Tests**: golden GMP resources per MetricSet, discovery/TLS/auth for etcd/KAS, unchanged default/RHOBS output, and no inactive-backend decoding or API/watch/status operations. Assert the rendered CPO writer permissions and the HyperShift operator's grant authority; use an API-server-backed test with the intended identities to verify Role granting and GMP create/update/delete authorization, since fake clients do not enforce RBAC. Cover grants before resource creation and cleanup before permission revocation. Add explicit monitoring error/retry and both-direction cleanup/recovery tests under the agreed availability/upgrade policy.

### Phase 4.5 — CA Secrets & Collector RBAC (GMP-gated)

GMP TLS supports only Secret-based CA refs, but the CoreOS monitors reference their CA as ConfigMaps (`root-ca`, `etcd-ca`). The GMP path must therefore mirror those public CA bundles into CA-only Secrets in the HCP namespace, and — because `gke-gmp-system` collectors run under their own ServiceAccount — grant that ServiceAccount read access via a namespaced `Role`/`RoleBinding`. Phase 0 confirmed collectors can read HCP-local Secrets once this RBAC exists; without it, scraping fails.

- Owned by CPOv2 in the HCP namespace and selected only for GMP through the agreed lifecycle design; the responsible per-workload or centralized path remains unresolved.
- **CA-only Secrets**: reconcile `ca.crt` from the source `root-ca`/`etcd-ca` ConfigMaps (and any other ConfigMap-sourced CA used by an enabled GMP endpoint). Re-sync every reconcile so CA rotation propagates; copy only the public `ca.crt`, never private signing keys.
- **`Role`**: `get`/`list`/`watch` on Secrets, restricted by `resourceNames` to the CA-only Secrets plus the existing client-credential Secrets the endpoints need (`metrics-client`, `etcd-metrics-client-tls`).
- **`RoleBinding`**: bind the Role to the `gke-gmp-system` collector ServiceAccount (subject confirmed in Phase 0).
- Owner-reference all objects to the HostedControlPlane so they are garbage-collected on teardown.

**Tests**: with GMP selected, the CA-only Secrets, Role, and RoleBinding render with the expected `resourceNames` and subject; the Secret `ca.crt` matches the source ConfigMap; none are emitted on the default/CoreOS path.

### Phase 5 — Install-Level GMP Monitors

Decide rule scope from the rendered expressions: GMP Rules are namespace-scoped. Management expressions aggregating across HCP namespaces may require ClusterRules or GlobalRules plus an API-package extension; do not assume every PrometheusRule maps to Rules.

Add GMP equivalents for the 4 management-level resources (2 monitors + 2 rules) in `cmd/install/assets/hypershift_operator.go` (Go constructors, matching today; could move to YAML assets — decide during implementation, raise with maintainers if it affects their conventions):

| CoreOS constructor | Line | GMP equivalent |
|--------------------|------|----------------|
| `HyperShiftServiceMonitor.Build()` (`operator`) | :1840 | `HyperShiftGMPPodMonitoring.Build()` |
| `ExternalDNSPodMonitor.Build()` (`external-dns`) | :1219 | `ExternalDNSGMPPodMonitoring.Build()` |
| `HypershiftRecordingRule.Build()` (`metrics`) | :1871 | `HypershiftGMPRecordingRules.Build()` |
| `HypershiftAlertingRule.Build()` (`alerts`, openshift-monitoring ns) | :1891 | `HypershiftGMPAlertingRules.Build()` |

- Selection logic in `cmd/install/install.go` where the monitor objects are appended (near the existing `PlatformMonitoring`/external-dns blocks) chooses GMP vs CoreOS constructors based on `Options.MonitoringAPIGroup`.
- These are simpler than CPO monitors (plain HTTP `/metrics`, no mTLS/bearer token).

**Tests**: rendered object assertions for both selections; default remains CoreOS; alerting rule lands in `openshift-monitoring`.

### Phase 6 — NetworkPolicy (GMP-gated)

- **hypershift-operator plumbing (prerequisite)**: `reconcileOpenshiftMonitoringNetworkPolicy` runs in the **hypershift-operator** — a *third* binary, distinct from the control-plane-operator and `hypershift install` wired up in Phase 1. It therefore needs its own copy of the selection. Read `MONITORING_API_GROUP` from the environment at operator startup (reuse the Phase 1 startup/validation parser; registration alone occurs at init), store it on the operator's options/reconciler, and thread it into the `hostedcluster` controller so `reconcileOpenshiftMonitoringNetworkPolicy` can branch on it. Without this step the GMP ingress rule below has no selection signal to gate on. (The operator does not *emit* monitors — it only needs the selection to gate the NetworkPolicy — so no scheme/predicate changes are required here.)
- In `hypershift-operator/controllers/hostedcluster/network_policies.go`, extend `reconcileOpenshiftMonitoringNetworkPolicy` (`:576-593`) to add an ingress rule admitting GMP collectors from `gke-gmp-system` **only when GMP is selected**. Reuse the existing broad monitoring policy shape (`podSelector: {}`, no port restrictions) with the Phase 0-confirmed collector peer (namespace `gke-gmp-system` + pod label `app.kubernetes.io/name=collector`). If the HyperShift team decides to descope collector access to specific metrics ports (the design decision's NetworkPolicy-scope open question), this plan will be updated to emit a per-endpoint destination set.
- Default, ROSA HCP, and ARO HCP policies must be byte-for-byte unchanged (the new rule is additive and gated).

**Tests**: operator correctly reads `MONITORING_API_GROUP` and threads it to the reconciler; the operator **validates** the value at startup via the shared parser (rejecting unsupported values and the GMP+RHOBS conflict), not just reading it; policy with GMP selected contains the `gke-gmp-system` ingress rule; without GMP the policy is unchanged; RHOBS path unaffected.

### Phase 7 — Parity Test & CI Guard

The mapping is many-to-one (no ServiceMonitor kind; `ServiceMonitor`/`PodMonitor` both collapse to `PodMonitoring`; monitors may be consolidated into one `PodMonitoring` with multiple endpoints), so a per-file "twin" check would throw false failures the moment anyone consolidates. Define parity as **scrape-target coverage** instead:

- Repo-level Go test that extracts the set of scrape targets from all **endpoint-bearing** CoreOS monitors (the `ServiceMonitor`/`PodMonitor` CPO YAML assets + the two endpoint-bearing install constructors) — keyed by `(component, pod/target selector, port, metrics path, MetricSet)` — and the equivalent set from all GMP `PodMonitoring.endpoints`, then asserts the GMP set **covers** the CoreOS set. This passes whether one `PodMonitoring` covers one old monitor or three. `PrometheusRule`/GMP `Rules` resources expose **no** scrape endpoints and are deliberately excluded from this coverage set.
- **Rules are checked separately** (not as scrape targets): assert each CoreOS `PrometheusRule` group (the CPO recording rule + the management-level recording/alerting rules) has a corresponding GMP rule group at the audited namespace/cluster/global scope. This is a distinct assertion from endpoint coverage above, because rule resources carry no `(selector, port, path)` scrape target.
- Build the discovery inventory **offline** from rendered and adapted monitors, Services and workload pod templates, including both YAML assets and Go constructors/reconcilers for the same HCP/platform/MetricSet configuration and component predicates. Include the kube-apiserver Service reconciled through `hostedcontrolplane/infra/infra.go` and `kas/service.go`; it is not a Service YAML asset.
- For `ServiceMonitor`s, resolve the monitor's Service selector and namespace selection against that inventory, then map each Service `spec.selector` to the pods it fronts. If an endpoint specifies `port`, select the named Service port and resolve its backing endpoint/container port; `port` takes precedence over `targetPort`. Otherwise, resolve endpoint `targetPort` directly against selected pods' container ports by name or number. Compare effective pod targets and concrete ports with GMP endpoints, rather than assuming every monitor follows a Service-port-name chain. Fail on unresolved discovery instead of silently excluding the target.
- Test normalization with the Go-reconciled kube-apiserver Service, named Service ports, named/numeric endpoint `targetPort`, and selector/port changes. Assert that narrowing or widening a GMP selector is detected — for example, `{app: etcd}` must not silently replace an effective selector of `{app: etcd, tier: control-plane}`. Include negative cases for missing Service or pod-port resolution.
- **Scope**: this test guards *target-level* coverage (same pods/ports/paths scraped per MetricSet), **not** metric-level equivalence (identical keep/drop after relabeling). Relabeling correctness is covered separately by the Phase 3 translator tests and the Phase 4 per-component × MetricSet golden files. A target can be "covered" here while its relabeling drifts; golden and translator tests establish code-level coverage/relabel checks. Live metric, label and rule parity remains a cutover gate. Keep the MetricSet dimension in the target key so a target scraped under the wrong MetricSet still fails coverage.
- Support an explicit allowlist/waiver for intentional exclusions (keyed by target, not filename) so the test is actionable, not a blanket block.
- Fails CI when a new CoreOS monitor (or an added endpoint on an existing one) introduces a scrape target with no GMP coverage.

### Phase 8 — Documentation

- Update HyperShift repo docs: installing the GMP CRDs on the management cluster and setting the `MONITORING_API_GROUP` env var on **all three binaries** (control-plane-operator and `hypershift install` to emit GMP monitors; the hypershift-operator to gate the GMP NetworkPolicy — see Phase 6) — the epic's "Update Documentation" task.
- Note in gcp-hcp: mark the [hybrid GMP decision](../design-decisions/observability/hypershift-gmp-native-monitoring.md) linkage and that the dedicated Prometheus removal is a follow-up.

## Testing Strategy Summary

| Layer | Coverage |
|-------|----------|
| Env var/validation | enum validation; RHOBS + env-var conflict across the selection-consuming binaries (install/render + CPO startup now; hypershift-operator at Phase 6); default→coreos |
| Scheme | `AllMonitoringScheme` registers coreos + GMP; RHOBS path unchanged |
| API snapshot reproducibility | Local foundation: source pin/checksums and generated provenance; unchanged-pin regeneration; non-writing verification rejects missing, changed or unexpected artifacts; isolated pin-update exercise; CI verification on relevant changes |
| API data/schema | Local foundation: serializer round trips, nested deepcopy isolation and pinned OpenAPI checks with negative controls; live admission/scraping/rule evaluation remain separate compatibility gates |
| Translation | per-action relabel translation; unsupported-action failure; MetricSet parity for etcd/KAS |
| CPO manifests | golden files per component × MetricSet; backend selection/isolation under the agreed runtime design; TLS/auth for etcd & KAS |
| Writer RBAC | CPO GMP resource permissions and HyperShift operator grant authority; API-server-backed authorization checks; grants before creation and cleanup before revocation; default/RHOBS steady state unchanged |
| Install monitors | both selections render; default coreos; alerts in openshift-monitoring |
| CA/RBAC | CA-only Secrets + collector Role/RoleBinding render GMP-gated; Secret `ca.crt` matches source ConfigMap; absent on default |
| NetworkPolicy | GMP adds gke-gmp-system ingress; non-GMP unchanged |
| Parity | repo-level: every CoreOS scrape target is covered by a GMP endpoint; rendered YAML/Go discovery inventory, distinct `port`/`targetPort` semantics and selector/port drift checks (coverage-based, not per-file twin; handles SM/PM→PodMonitoring collapse and consolidation) |
| Regression | default and `RHOBS_MONITORING=1` outputs unchanged (golden) |

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| GMP collectors can't present etcd client certs | etcd unscrapeable — blocks epic | **Resolved in Phase 0**: collectors scraped etcd via mTLS client certs (`up=1`); no proxy needed |
| Service→pod discovery mismatch | missed/duplicated targets | Per-ServiceMonitor discovery audit; golden tests assert selector target set |
| ConfigMap-based CA refs unsupported in GMP | TLS config fails | **Resolved in Phase 0**: CA bundles mirrored into CA-only Secrets + collector RBAC (Phase 4.5) |
| Remote rule evaluation changes alert timing | alert semantics drift | Review affected rules for semantics drift before enabling GMP rules |
| Maintaining GMP API definitions | copied API/schema drift | GCP HCP ownership and gcp-reviewers review; source pin in `hack/gmp-api-source/go.mod`; generated provenance and reproducible sync/CI verification; canonical HyperShift runbook and deliberate compatibility review. Candidate-update detection remains open. |
| Source snapshot differs from deployed GMP | API fields rejected or behave differently | Record GKE version, GMP operator image and deployed-schema/live compatibility evidence; do not infer support from matching version numbers or offline fixtures. |
| Manifest duplication drift (CoreOS target added without GMP coverage) | Target silently unscraped under GMP | Coverage-based parity test (Phase 7) blocks CI |
| Non-1:1 mapping mis-modeled (SM→pod selector narrows/widens target set; over-consolidation) | GMP scrapes a different target set than CoreOS | Per-target discovery audit (Phase 4); coverage test normalizes Service discovery to pod targets before comparing |
| Inactive backend isolation | unintended decoding/API/status/watch operations | Additive registration is inert; resolve and test lifecycle selection before expanding assets |

## Dependencies & Sequencing

- **Blocks on**: design sign-off (HyperShift maintainers).
- **Depends on**: GMP CRDs installed on GKE management clusters (already present); GMP collectors running in `gke-gmp-system` (already present).
- **Followed by** (separate ops/infra story): configure/validate GMP collectors, verify live scrape coverage & metric parity, assess ingestion volume/quota, cut over, and remove the dedicated Prometheus from gcp-hcp-infra.
- **Possible future work** (out of scope here, not planned by this work): if desired, `monitoring.rhobs` could be folded in as a third `MONITORING_API_GROUP` value, and/or a `--rhobs-monitoring`-style validation flag could be added to cross-check the env var.

## Definition of Done (this story)

- [ ] API snapshot has a documented GCP HCP owner and gcp-reviewers routing, a pinned source module, generated provenance, reproducible `make gmp-sync` and non-writing `make verify-gmp-sync` enforced in CI for relevant changes; canonical maintenance instructions and focused checks are documented. Implemented and validated locally; merge remains pending.
- [ ] Supported GKE/GMP environments and compatibility evidence are recorded, including GKE version, GMP operator image and the validation performed; offline schema checks are distinguished from live admission, scraping and rule evaluation.
- [ ] `MONITORING_API_GROUP` honored by both binaries; default and `RHOBS_MONITORING=1` outputs unchanged (golden tests green).
- [ ] RHOBS + GMP env-var combination is rejected with the specified error by every binary that consumes the selection — `hypershift install`/`render` and control-plane-operator startup today, and the hypershift-operator once it reads the selection for the NetworkPolicy (Phase 6).
- [ ] Every CoreOS scrape target (all 11 SM + 8 PM at CPO level + the 2 endpoint-bearing management-level monitors) is covered by a GMP `PodMonitoring` endpoint and renders correctly — noting GMP resources may be fewer than CoreOS files due to SM/PM→PodMonitoring collapse and endpoint consolidation.
- [ ] Rule resources (the CPO recording-rule `PrometheusRule` + the 2 management-level recording/alerting rules) have matching GMP rule resources at the required namespace/cluster/global scope and are verified **separately** from scrape-target coverage.
- [ ] CPO writer permissions and the HyperShift operator's authority to grant them cover the selected GMP resources; authorization and backend-transition tests prove grants precede creation and owned-resource cleanup precedes revocation, with unchanged default/RHOBS steady-state policies.
- [ ] etcd and kube-apiserver GMP monitors preserve TLS/auth and are validated (Phase 0 evidence attached).
- [ ] MetricSet (Telemetry/SRE/All) semantics preserved via the translation layer; no duplicated allow/drop lists.
- [ ] CA-only Secrets and the collector Role/RoleBinding are emitted (GMP-gated), owner-referenced to the HCP, and absent on the default path.
- [ ] GMP-gated NetworkPolicy admits `gke-gmp-system`; non-GMP/ROSA/ARO policies unchanged.
- [ ] Coverage-based parity test fails CI when a CoreOS scrape target has no GMP coverage.
- [ ] HyperShift docs updated (CRD install + `MONITORING_API_GROUP` usage).
- [ ] ROSA HCP and ARO HCP verified unaffected.
