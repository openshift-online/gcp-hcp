# GCP-1227: GMP-Native Monitoring Support in HyperShift — Implementation Plan

**Jira**: [GCP-1227](https://redhat.atlassian.net/browse/GCP-1227) (Story) · Epic [GCP-423](https://redhat.atlassian.net/browse/GCP-423)

**Design decision**: [hypershift-gmp-native-monitoring](../design-decisions/observability/hypershift-gmp-native-monitoring.md)

**Target repository**: `github.com/openshift/hypershift` (upstream — requires HyperShift maintainer review/sign-off)

> **Status**: Draft plan pending design sign-off. Do **not** begin coding before the [design decision open questions](../design-decisions/observability/hypershift-gmp-native-monitoring.md#open-questions-for-hypershift-team-review) are resolved and Phase 0 (spike) confirms feasibility. All line numbers below were verified against the HyperShift `main` checkout on 2026-09-23 and will drift; treat them as anchors, not literals.

## Overview

Add a monitoring API-group selection to HyperShift so the control-plane-operator (CPOv2) and `hypershift install` can emit GMP resources (`monitoring.googleapis.com`) instead of CoreOS resources (`monitoring.coreos.com`), gated by a `MONITORING_API_GROUP` environment variable. The CoreOS default and the existing `RHOBS_MONITORING=1` path stay byte-for-byte unchanged. GMP resources are authored as separate, hand-maintained manifests/constructors and selected via the existing CPOv2 predicate system. A coverage-based parity test prevents a CoreOS scrape target from being added without matching GMP coverage (the CoreOS→GMP mapping is not 1:1 — see Phase 7).

### Goals

- `MONITORING_API_GROUP` environment variable read by **both** the control-plane-operator and `hypershift install`, values `monitoring.coreos.com` (default) and `monitoring.googleapis.com`. An env var (not a CLI flag) is used because `hypershift install` emits monitoring resources from a runtime scheme initialized at `init()`, before flags are parsed — see the design decision.
- GMP coverage for all current monitors — **11 ServiceMonitors + 8 PodMonitors + 1 PrometheusRule** (CPO level) and **4 management-level monitors** (install level). Note the mapping is **not 1:1**: GMP has no ServiceMonitor kind, so both `ServiceMonitor` and `PodMonitor` collapse onto `PodMonitoring`, and monitors targeting the same pods may be consolidated into a single `PodMonitoring` with multiple `endpoints`. The goal is that every CoreOS scrape *target* has GMP coverage, not that there are 20 matching GMP files.
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

The full plan covers all 24 current CoreOS monitors (20 CPO + 4 install) — the GMP side will be *at most* 24 manifests and likely fewer, since `ServiceMonitor`/`PodMonitor` both collapse to `PodMonitoring` and same-pod targets can be consolidated. The POC exists to **prove the two real unknowns cheaply** — (a) can GMP collectors actually scrape the authenticated targets, and (b) does the env-var→context→predicate→manifest slice work end-to-end — before investing in all the GMP equivalents, the parity test, and docs. Everything below is deliberately narrowed to **etcd + kube-apiserver only** (the two hardest targets: mTLS and bearer token).

The POC runs as **two decoupled tracks in parallel**:

### Track A — Auth/scrape spike (infra, BLOCKING, no repo changes)

This is **Phase 0** and gates everything. It does not touch the HyperShift repo — it needs a live GKE management cluster with `gke-gmp-system` collectors.

1. Hand-write plain-YAML `PodMonitoring` for **etcd** (mTLS: `tls.cert`/`tls.key`/`tls.ca` as Secret refs) and **kube-apiserver** (bearer token via `authorization`) — no Go, no typed structs.
2. Apply against a real GKE cluster and confirm the collectors scrape both targets.
3. Determine the `gke-gmp-system` collector namespace/pod-label selector for the NetworkPolicy (needed by Track B step 5 and later Phase 6).
4. Confirm any ConfigMap-sourced CA bundle has a Secret-based equivalent (GMP TLS supports Secret refs only).

**Exit criteria**: etcd and kube-apiserver both scraped, or a documented alternate path for anything that can't be. Record findings on GCP-1227. **Do not start Track B's manifest step (4) until this passes** — a failure here changes the design.

### Track B — Vertical code slice (repo, parallel with Track A)

A thin end-to-end slice through Phases 1–4, **etcd + kube-apiserver only**, deferring vendoring:

1. **Env var + options** (Phase 1.1–1.3): read `MONITORING_API_GROUP` on the CPO (and optionally `hypershift install`); default `monitoring.coreos.com`; enum validation.
2. **Context threading** (Phase 1.2): add `MonitoringAPIGroup` to `ControlPlaneContext` + `WorkloadContext` and copy it in `workloadContext()`, mirroring `MetricsSet` exactly (see Phase 1 step 2).
3. **Startup validation** (Phase 1.4): reject `MONITORING_API_GROUP=monitoring.googleapis.com` + `RHOBS_MONITORING=1`.
4. **Predicates** (Phase 1.6): `isGMP` / `isNotGMP` as `func(component.WorkloadContext) bool`.
5. **Emit `unstructured`, skip vendoring** (defer Phase 2): for the POC, build the GMP `PodMonitoring` objects as `unstructured.Unstructured` rather than vendoring `github.com/GoogleCloudPlatform/prometheus-engine`. This sidesteps design **open question #4** (maintainer sign-off on the new module dependency) and gets to a scraped metric faster. Swap to typed structs only when hardening for the real PR.
6. **Two manifests** (Phase 4, etcd + KAS only): add `gmp-podmonitoring.yaml` to the `etcd` and `kube-apiserver` components behind `isGMP`, keeping the existing `servicemonitor.yaml` behind `isNotGMP`. Reuse the YAML validated in Track A step 1.
7. **Relabel translator** (Phase 3, minimal): translate `EtcdRelabelConfigs`/`KASRelabelConfigs` output into the GMP relabel shape for these two adapt functions only.

### Explicitly NOT in the POC (deferred to full delivery)

- The other 18 CPO monitors and all 4 install-level monitors (Phase 5).
- Vendoring the GMP CRD Go types / typed structs (Phase 2) — POC uses `unstructured`.
- The parity test + CI guard (Phase 7) — meaningless until most GMP equivalents exist.
- Documentation (Phase 8).
- Production NetworkPolicy wiring (Phase 6) beyond whatever the Track A spike needs to get a scrape.

### POC exit criteria

- etcd and kube-apiserver GMP `PodMonitoring` are scraped by GMP collectors on a live GKE cluster (Track A evidence on GCP-1227).
- With `MONITORING_API_GROUP=monitoring.googleapis.com`, the CPO emits GMP `PodMonitoring` for etcd/KAS and **omits** the CoreOS `ServiceMonitor`; with the default (unset) env var, output is byte-for-byte the CoreOS behavior.
- RHOBS + env-var conflict fails at startup.
- MetricSet filtering (Telemetry/SRE/All) is preserved for etcd/KAS through the translator.

Passing these means the design is validated and the remaining work is mechanical replication across the other monitors — at which point resume the full phased delivery below (add vendoring, remaining GMP equivalents, parity test, docs).

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
| GMP CRD Go types | — | **Not vendored** (no `go.mod` dep, no `vendor/github.com/GoogleCloudPlatform`, no `.go` refs). CRDs *do* exist at runtime on GKE (`gke-gmp-system`) and gcp-hcp applies `PodMonitoring` YAML for ArgoCD — but HyperShift builds monitors as typed Go objects, so it needs the structs vendored (or must emit unstructured). |
| `AllMonitoringScheme` | `support/api/scheme.go:74,106-107` | Backs `AllMonitoringYamlSerializer` (decode helper). Holds coreos + rhobs today. A *decode helper*, not the isolation mechanism. |

## Architecture

Selection is threaded at three levels, matching the epic's model:

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
               env var → APIGroup on ControlPlaneContext
               → predicate isGMP / isNotGMP selects manifest per component
```

### Key design choices (from the design decision)

1. **Activation**: `MONITORING_API_GROUP` environment variable read by both binaries (chosen over a CLI flag because `hypershift install` emits monitoring resources from a scheme initialized at `init()`, before flags are parsed; the specific variable name/values are pending final maintainer confirmation).
2. **Scheme**: register GMP types **additively or on-demand** for the GMP path only, driven by the env var at `init()` (available before scheme registration, like `RHOBS_MONITORING`); the env var selects what to *emit*, not what other platforms *register*. Registration is inert (it only teaches the client to (de)serialize a Kind), and emission is gated by the env-var default + `isGMP` predicate — so ROSA/ARO/self-managed are unaffected regardless of scheme contents. **Do not modify the coreos/rhobs registration those platforms build.**
3. **Manifests**: separate GMP assets/constructors, never runtime conversion.
4. **Selection**: existing CPOv2 predicate system (`isGMP` / `isNotGMP`), same pattern as `isAroHCP`.
5. **MetricSet reuse**: GMP adapt functions call existing `metrics.*RelabelConfigs()` and translate the returned `[]prometheusoperatorv1.RelabelConfig` into GMP relabel types — no duplicated allow/drop lists.

## Phased Delivery

### Phase 0 — Feasibility Spike (BLOCKING, do first)

The highest-risk unknown is authenticated scraping. Build a throwaway GMP `PodMonitoring` for **etcd** (mTLS, client certs) and **kube-apiserver** (bearer token) against a real GKE management cluster with GMP collectors and confirm the collectors actually scrape them.

- Confirm GMP collectors can present etcd client certificates (`tls.cert`/`tls.key` as Secret refs) — if not, etcd scraping needs an alternate path (proxy/sidecar) and the design must be revisited before proceeding.
- Confirm bearer-token auth for kube-apiserver via GMP `authorization`.
- Confirm ConfigMap-sourced CA bundles used by any CoreOS monitor have a Secret-based equivalent for GMP (GMP TLS supports Secret refs only).
- Determine `gke-gmp-system` collector namespace/pod labels for the NetworkPolicy selector, and whether they are stable/documented.

**Exit criteria**: etcd and kube-apiserver both scraped by GMP collectors, or a documented alternate approach for any target that cannot be. Record findings on GCP-1227.

### Phase 1 — Env Var, Options, Scheme, Validation

Foundational plumbing, no manifests yet. CoreOS output must be unchanged after this phase. This phase wires the two *emitting* binaries (control-plane-operator and `hypershift install`); the **hypershift-operator** — a third binary that reads the same env var only to gate the GMP NetworkPolicy, not to emit monitors — is plumbed separately in Phase 6.

1. **Constants**: add a shared `MonitoringAPIGroup` type/consts (`monitoring.coreos.com`, `monitoring.googleapis.com`) in a support package (e.g. alongside `support/metrics` or `support/api`), plus a validation helper.
2. **control-plane-operator** (`control-plane-operator/main.go:321-361`): read `MONITORING_API_GROUP` from the environment at startup into the local options struct; default `monitoring.coreos.com` when unset; validate the enum; thread the value into the reconciler and down to components by mirroring how `MetricsSet` already flows — add a `MonitoringAPIGroup` field to **`ControlPlaneContext`** (`support/controlplane-component/controlplane-component.go:61`, next to `MetricsSet`) **and** to **`WorkloadContext`** (`:97`), and copy it in **`workloadContext()`** (`:114`). This is the proven, verified path: predicates and adapt functions receive `WorkloadContext`, so once the field is on it, `isGMP`/`isNotGMP` and the GMP adapt functions can read it with no framework change.
3. **hypershift install** (`cmd/install/install.go:87-149`): add `MonitoringAPIGroup` to `Options`, populated from the `MONITORING_API_GROUP` env var (mirroring how `RHOBSMonitoring` reads `RHOBS_MONITORING` rather than binding a flag); validate the enum. Unlike the RHOBS precedent — which also exposes a `--rhobs-monitoring` flag purely to validate consistency against the env var — no validation flag is proposed here (it can be added later if maintainers want the cross-check).
4. **Startup validation** (both binaries): if `MONITORING_API_GROUP` selects GMP (`monitoring.googleapis.com`) **and** `RHOBS_MONITORING=1`, exit non-zero with the actionable error from the epic:
   ```
   error: MONITORING_API_GROUP=monitoring.googleapis.com cannot be used when
   RHOBS_MONITORING=1 is set. RHOBS_MONITORING selects the RHOBS monitoring
   backend. To use a different monitoring API group, unset RHOBS_MONITORING first.
   ```
5. **Scheme** (`support/api/scheme.go`): register the GMP Go types **additively or on-demand** for the GMP code path only. Registration is inert — it teaches the (de)serializer/client to recognize `PodMonitoring`/`Rules`; it does not create, list, or watch anything. **Do not modify the coreos/rhobs registration** that self-managed HyperShift, ROSA HCP, and ARO HCP depend on. Because `MONITORING_API_GROUP` is available at `init()` (like `RHOBS_MONITORING`), registration can key off it directly — either additively at `init()`, or on-demand based on the env-var value — leaving the scheme those platforms build byte-identical. No restructuring of the scheme init path is required. Keep the RHOBS `init()` env-var path unchanged (folding it into this `MONITORING_API_GROUP` selection is possible future work, out of scope here). Avoid any generic "list/watch every registered monitoring type" logic — it would fail on clusters lacking the GMP CRD.
6. **Predicates**: add `isGMP` and `isNotGMP` reading `MonitoringAPIGroup` from context, in `support/controlplane-component` or the component package. Match the per-manifest predicate type exactly — `type Predicate func(cpContext WorkloadContext) bool` (`support/controlplane-component/generic-adapter.go:15`), i.e. `func isGMP(cpContext component.WorkloadContext) bool` (takes `WorkloadContext`, returns a plain `bool`, **no** error), same as the `isAroHCP` precedent (`.../azure/component.go:92`). Note: the *builder-level* `WithPredicate` (`builder.go:74`) takes a `(bool, error)` predicate, but the per-manifest `component.WithPredicate` you use for `gmp-podmonitoring.yaml` uses the plain-`bool` `Predicate` type.

7. **Deployment env injection** (so the var actually reaches the operator pods): a process's environment is **not** inherited by the pods it renders, so `MONITORING_API_GROUP` must be injected into each operator's container `env`, mirroring how `RHOBS_MONITORING` is propagated today. Two hops: (a) `hypershift install` must add `MONITORING_API_GROUP` to the **hypershift-operator** Deployment spec it renders, and (b) the **hypershift-operator** must in turn propagate it onto the per-HCP **control-plane-operator** Deployment it creates. Without both hops an operator pod silently falls back to the `monitoring.coreos.com` default while install-time resources emit GMP — a split-brain selection. (This is the deploy-time counterpart to the startup read in steps 2–3; the Phase 6 operator plumbing depends on hop (a) landing the var on the operator pod.)

**Tests**: env-var parsing/enum validation; the RHOBS + env-var conflict error on both binaries; default selection resolves to coreos; predicates return expected booleans; the **rendered Deployment specs** for the hypershift-operator (from `hypershift install`) and the control-plane-operator (from the operator) carry `MONITORING_API_GROUP` in their container env when GMP is selected, and are unchanged on the default path.

### Phase 2 — GMP CRD Types Dependency

1. Vendor the GMP CRD Go types (candidate: `github.com/GoogleCloudPlatform/prometheus-engine/pkg/operator/apis/monitoring/v1`, providing `PodMonitoring`, `ClusterPodMonitoring`, `Rules`). Confirm module/version with maintainers (design open question #4).
2. Register `PodMonitoring` and `Rules` in **every scheme used to encode and apply** emitted resources — not only `AllMonitoringScheme`, which backs the `AllMonitoringYamlSerializer` *decode* helper. Identify the scheme(s) the control-plane-operator and `hypershift install` actually use to serialize and apply the monitors they emit (the main `Scheme` their clients build, alongside `AllMonitoringScheme` if the emit path also round-trips through the serializer), and register the GMP types there, following the additive-or-on-demand approach from Phase 1 step 5 (keyed off `MONITORING_API_GROUP` at `init()`, leaving the coreos/rhobs registration untouched).
3. `go mod tidy` / `make vendor` / regenerate deepcopy if required; ensure the HyperShift build and image are unaffected on the default path.

### Phase 3 — MetricSet → GMP Relabel Translation Layer

A small, well-tested translator is the reusable core the manifests depend on.

1. New function, e.g. `metrics.ToGMPRelabeling([]prometheusoperatorv1.RelabelConfig) []<gmpv1.RelabelingRule>` mapping each field (`sourceLabels`, `targetLabel`, `regex`, `action`, `replacement`, `separator`, `modulus`).
2. Explicitly handle/verify each `action` used by the existing configs (`keep`, `drop`, `replace`, `labeldrop`, `labelkeep`, `hashmod`, `labelmap`); fail loudly on any unsupported action rather than silently dropping it.
3. GMP adapt functions call the existing per-component `metrics.*RelabelConfigs(set)` (e.g. `EtcdRelabelConfigs`, `KASRelabelConfigs`) and pass the result through the translator — no reimplementation of allow/drop lists.

**Tests**: table-driven translation tests per action; round-trip check that Telemetry/SRE/All produce equivalent keep/drop semantics against GMP types for etcd and kube-apiserver.

### Phase 4 — CPO-Level GMP Manifests (per component)

For each of the 20 CPO monitors, add GMP coverage using the predicate pattern (mirroring `isAroHCP`). This is **not necessarily one GMP file per CoreOS file**: both `ServiceMonitor` and `PodMonitor` map to `PodMonitoring`, and where a component has more than one CoreOS monitor over the same pods they may be folded into a single `PodMonitoring` with multiple `endpoints`. Default to one `gmp-podmonitoring.yaml` per component for reviewability, and consolidate only when the targets genuinely share a pod set:

```go
// e.g. v2/etcd/component.go
component.NewStatefulSetComponent(ComponentName, &etcd{}).
    WithAdaptFunction(adaptStatefulSet).
    WithManifestAdapter(
        "servicemonitor.yaml",
        component.WithAdaptFunction(adaptServiceMonitor),
        component.WithPredicate(isNotGMP),     // skip when GMP selected
    ).
    WithManifestAdapter(
        "gmp-podmonitoring.yaml",
        component.WithAdaptFunction(adaptGMPPodMonitoring),
        component.WithPredicate(isGMP),        // only when GMP selected
    ).
    Build()
```

- Add `gmp-podmonitoring.yaml` (translating a `ServiceMonitor` **or** `PodMonitor` — both become `PodMonitoring`; a ServiceMonitor's Service→endpoints discovery must be re-expressed as a pod label selector that reproduces the same target set) and `gmp-rules.yaml` (from the recording-rules PrometheusRule) alongside existing assets; `//go:embed */*.yaml` picks them up with no framework change.
- **Order of implementation** (highest risk first, per Phase 0): etcd → kube-apiserver → remaining ServiceMonitor components → PodMonitor components → recording rules.
- Each adapt function: translate discovery (Service→pod label selector — audit that pod labels reproduce the same target set), ports, `tlsConfig`→`tls` (Secret refs), auth, cluster-identity labels, and MetricSet relabelings (via Phase 3 translator).
- Per-target audit checklist recorded on the ticket: discovery mapping, port naming, TLS/auth, relabelings, cluster-identity labels.

**Tests**: for each component, golden-file test of the rendered GMP resource per MetricSet; presence/shape of TLS and auth for etcd and kube-apiserver; assertion that with `isNotGMP` the CoreOS asset renders unchanged and the GMP asset is absent (and vice versa).

### Phase 5 — Install-Level GMP Monitors

Add GMP equivalents for the 4 management-level monitors in `cmd/install/assets/hypershift_operator.go` (Go constructors, matching today; could move to YAML assets — decide during implementation, raise with maintainers if it affects their conventions):

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

- **hypershift-operator plumbing (prerequisite)**: `reconcileOpenshiftMonitoringNetworkPolicy` runs in the **hypershift-operator** — a *third* binary, distinct from the control-plane-operator and `hypershift install` wired up in Phase 1. It therefore needs its own copy of the selection. Read `MONITORING_API_GROUP` from the environment at operator startup (same `init()`-time read as the other binaries; reuse the Phase 1 constants + validation helper), store it on the operator's options/reconciler, and thread it into the `hostedcluster` controller so `reconcileOpenshiftMonitoringNetworkPolicy` can branch on it. Without this step the GMP ingress rule below has no selection signal to gate on. (The operator does not *emit* monitors — it only needs the selection to gate the NetworkPolicy — so no scheme/predicate changes are required here.)
- In `hypershift-operator/controllers/hostedcluster/network_policies.go`, extend `reconcileOpenshiftMonitoringNetworkPolicy` (`:576-593`) to add an ingress rule admitting GMP collectors from `gke-gmp-system` **only when GMP is selected**. Use the selector confirmed in Phase 0 (namespace selector vs. collector pod-label selector — decide during implementation based on Phase 0 findings).
- Default, ROSA HCP, and ARO HCP policies must be byte-for-byte unchanged (the new rule is additive and gated).

**Tests**: operator correctly reads `MONITORING_API_GROUP` and threads it to the reconciler; policy with GMP selected contains the `gke-gmp-system` ingress rule; without GMP the policy is unchanged; RHOBS path unaffected.

### Phase 7 — Parity Test & CI Guard

The mapping is many-to-one (no ServiceMonitor kind; `ServiceMonitor`/`PodMonitor` both collapse to `PodMonitoring`; monitors may be consolidated into one `PodMonitoring` with multiple endpoints), so a per-file "twin" check would throw false failures the moment anyone consolidates. Define parity as **scrape-target coverage** instead:

- Repo-level Go test that extracts the set of scrape targets from all **endpoint-bearing** CoreOS monitors (the `ServiceMonitor`/`PodMonitor` CPO YAML assets + the two endpoint-bearing install constructors) — keyed by `(component, pod/target selector, port, metrics path, MetricSet)` — and the equivalent set from all GMP `PodMonitoring.endpoints`, then asserts the GMP set **covers** the CoreOS set. This passes whether one `PodMonitoring` covers one old monitor or three. `PrometheusRule`/GMP `Rules` resources expose **no** scrape endpoints and are deliberately excluded from this coverage set.
- **Rules are checked separately** (not as scrape targets): assert each CoreOS `PrometheusRule` group (the CPO recording rule + the management-level recording/alerting rules) has a corresponding GMP `Rules` group. This is a distinct assertion from endpoint coverage above, because rule resources carry no `(selector, port, path)` scrape target.
- For `ServiceMonitor`s, normalize Service-based discovery to the backing pod target set before comparing, so a Service→pod selector translation that silently narrows or widens coverage is caught. Normalization is **static** (no live cluster): resolve the ServiceMonitor's Service selector against the Service definitions in the same assets, map each Service `spec.selector` to the pods it fronts, and resolve named ports through the Service `port.name → targetPort → container port` chain so the key compares effective pod label selectors and concrete ports, not Service-level names. Confirm this static resolution is sufficient during Phase 0 (it is the one place where a hand-written GMP pod selector — e.g. `{app: etcd}` — could legitimately differ from the Service's effective selector — e.g. `{app: etcd, tier: control-plane}` — and widen the target set).
- **Scope**: this test guards *target-level* coverage (same pods/ports/paths scraped per MetricSet), **not** metric-level equivalence (identical keep/drop after relabeling). Relabeling correctness is covered separately by the Phase 3 translator tests and the Phase 4 per-component × MetricSet golden files. A target can be "covered" here while its relabeling drifts; the golden tests are the guard for that, and the two together constitute full parity. Keep the MetricSet dimension in the target key so a target scraped under the wrong MetricSet still fails coverage.
- Support an explicit allowlist/waiver for intentional exclusions (keyed by target, not filename) so the test is actionable, not a blanket block.
- Fails CI when a new CoreOS monitor (or an added endpoint on an existing one) introduces a scrape target with no GMP coverage.

### Phase 8 — Documentation

- Update HyperShift repo docs: installing the GMP CRDs on the management cluster and setting the `MONITORING_API_GROUP` env var on **all three binaries** (control-plane-operator and `hypershift install` to emit GMP monitors; the hypershift-operator to gate the GMP NetworkPolicy — see Phase 6) — the epic's "Update Documentation" task.
- Note in gcp-hcp: mark the [hybrid GMP decision](../design-decisions/observability/hypershift-gmp-native-monitoring.md) linkage and that the dedicated Prometheus removal is a follow-up.

## Testing Strategy Summary

| Layer | Coverage |
|-------|----------|
| Env var/validation | enum validation; RHOBS + env-var conflict on both binaries; default→coreos |
| Scheme | `AllMonitoringScheme` registers coreos + GMP; RHOBS path unchanged |
| Translation | per-action relabel translation; unsupported-action failure; MetricSet parity for etcd/KAS |
| CPO manifests | golden files per component × MetricSet; predicate selection (GMP xor CoreOS); TLS/auth for etcd & KAS |
| Install monitors | both selections render; default coreos; alerts in openshift-monitoring |
| NetworkPolicy | GMP adds gke-gmp-system ingress; non-GMP unchanged |
| Parity | repo-level: every CoreOS scrape target is covered by a GMP endpoint (coverage-based, not per-file twin; handles SM/PM→PodMonitoring collapse and consolidation) |
| Regression | default and `RHOBS_MONITORING=1` outputs unchanged (golden) |

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| GMP collectors can't present etcd client certs | etcd unscrapeable — blocks epic | Phase 0 spike before any manifest work; alternate proxy path if needed |
| Service→pod discovery mismatch | missed/duplicated targets | Per-ServiceMonitor discovery audit; golden tests assert selector target set |
| ConfigMap-based CA refs unsupported in GMP | TLS config fails | Phase 0 audit; migrate CA to Secret where needed |
| Remote rule evaluation changes alert timing | alert semantics drift | Review affected rules for semantics drift before enabling GMP rules |
| Vendoring GMP CRD types into upstream module | build/dependency concerns | Confirm module/version with maintainers (open question #4) before vendoring |
| Manifest duplication drift (CoreOS target added without GMP coverage) | Target silently unscraped under GMP | Coverage-based parity test (Phase 7) blocks CI |
| Non-1:1 mapping mis-modeled (SM→pod selector narrows/widens target set; over-consolidation) | GMP scrapes a different target set than CoreOS | Per-target discovery audit (Phase 4); coverage test normalizes Service discovery to pod targets before comparing |
| Scheme `init()` ordering | wrong types registered | Env var is read at `init()` (before scheme build), so there is no flag-vs-init ordering gap; isolation still comes from the default + `isGMP` at emit time |

## Dependencies & Sequencing

- **Blocks on**: design sign-off (HyperShift maintainers).
- **Depends on**: GMP CRDs installed on GKE management clusters (already present); GMP collectors running in `gke-gmp-system` (already present).
- **Followed by** (separate ops/infra story): configure/validate GMP collectors, verify live scrape coverage & metric parity, assess ingestion volume/quota, cut over, and remove the dedicated Prometheus from gcp-hcp-infra.
- **Possible future work** (out of scope here, not planned by this work): if desired, `monitoring.rhobs` could be folded in as a third `MONITORING_API_GROUP` value, and/or a `--rhobs-monitoring`-style validation flag could be added to cross-check the env var.

## Definition of Done (this story)

- [ ] `MONITORING_API_GROUP` honored by both binaries; default and `RHOBS_MONITORING=1` outputs unchanged (golden tests green).
- [ ] RHOBS + GMP env-var combination fails at startup with the specified error on both binaries.
- [ ] Every CoreOS scrape target (all 11 SM + 8 PM at CPO level + the 2 endpoint-bearing management-level monitors) is covered by a GMP `PodMonitoring` endpoint and renders correctly — noting GMP resources may be fewer than CoreOS files due to SM/PM→PodMonitoring collapse and endpoint consolidation.
- [ ] Rule resources (the CPO recording-rule `PrometheusRule` + the 2 management-level recording/alerting rules) have matching GMP `Rules` and are verified **separately** from scrape-target coverage.
- [ ] etcd and kube-apiserver GMP monitors preserve TLS/auth and are validated (Phase 0 evidence attached).
- [ ] MetricSet (Telemetry/SRE/All) semantics preserved via the translation layer; no duplicated allow/drop lists.
- [ ] GMP-gated NetworkPolicy admits `gke-gmp-system`; non-GMP/ROSA/ARO policies unchanged.
- [ ] Coverage-based parity test fails CI when a CoreOS scrape target has no GMP coverage.
- [ ] HyperShift docs updated (CRD install + `MONITORING_API_GROUP` usage).
- [ ] ROSA HCP and ARO HCP verified unaffected.
