# HyperShift GMP-Native Monitoring Resource Support

***Scope***: GCP-HCP

**Date**: 2026-09-23

**Status**: Proposed — pending HyperShift team review and sign-off

**Jira**: [GCP-1227](https://redhat.atlassian.net/browse/GCP-1227) (Story) under [GCP-423](https://redhat.atlassian.net/browse/GCP-423) (Epic)

## Decision

We will add `MONITORING_API_GROUP` selection so the control-plane-operator and `hypershift install` can emit GMP resources as an alternative to CoreOS. The values are `monitoring.coreos.com` (default) and `monitoring.googleapis.com`; GMP plus `RHOBS_MONITORING=1` is rejected. The foundation reads the environment at install/CPO validation and threads it through Deployment/context plumbing. GMP resources remain separate, hand-maintained assets/constructors; no HostedCluster API field is added. Runtime reconciliation is still proposed.

## Context

This decision **evolves** the earlier [Observability Platform: Google Managed Prometheus](observability-google-managed-prometheus.md) decision. That decision deliberately chose a *hybrid* model — a self-managed Prometheus per management cluster that scrapes the CoreOS monitors and remote-writes filtered metrics to GMP — precisely because native GMP `PodMonitoring` "requires rewrite of HyperShift monitoring templates." GCP-423/GCP-1227 is the decision to now do that rewrite so the dedicated Prometheus (added in [gcp-hcp-infra #337](https://github.com/openshift-online/gcp-hcp-infra/pull/337)) can be removed, eliminating per-cluster Prometheus overhead and aligning with GKE-native observability.

### Problem Statement

GKE management clusters already run GMP collectors in `gke-gmp-system` that natively scrape `monitoring.googleapis.com` CRDs but cannot read `monitoring.coreos.com` CRDs. HyperShift emits only CoreOS monitors, so GCP HCP must deploy a dedicated self-managed Prometheus in every management cluster as a translation shim. To decommission that Prometheus, HyperShift must be able to emit GMP-native monitoring resources for the same targets — without regressing the existing CoreOS behavior that self-managed HyperShift, ROSA HCP, and ARO HCP depend on.

### Constraints

- **No regression for existing consumers**: The default (MONITORING_API_GROUP unset) and the existing `RHOBS_MONITORING=1` path must be byte-for-byte unchanged. ROSA HCP and ARO HCP must be unaffected.
- **This is an upstream HyperShift change**: Code lands in `github.com/openshift/hypershift` and must be acceptable to its maintainers. It cannot depend on GCP-only assumptions leaking into shared code paths.
- **Scheme registration**: the foundation registers GMP PodMonitoring/Rules and their lists additively in both runtime schemes, regardless of MONITORING_API_GROUP. Existing RHOBS-dependent registration remains unchanged; AllMonitoringScheme includes both CoreOS and RHOBS when RHOBS is selected. Registration only supports serialization and starts no resources, clients or watches. It does not enforce runtime selection.
- **Existing-platform isolation (ROSA/ARO/self-managed)**: retain the CoreOS default and existing RHOBS behavior. Runtime selection must exclude inactive assets before decoding and before update/delete/status/watch operations. Per-manifest predicates run after manifest loading and false predicates initiate deletion; they alone cannot guarantee isolation, contain monitoring failures or implement safe backend transitions.
- **No `HostedCluster` API change**: Selection must be a binary/install concern, not a customer-facing spec field, to avoid an API review cycle and to keep the choice an operational property of the management cluster.
- **GMP build boundary**: a maintained `support/gmpmonitoring` API-only snapshot supplies PodMonitoring, Rules, lists and required nested data. The source version is selected in HyperShift's download-only `hack/gmp-api-source/go.mod`; generated `upstream.sha256` records the resolved version, commit, module checksums and input file checksums. Reproducible extraction preserves upstream license/comments/tags, generated deepcopies and pinned CRD schema fixtures. Operator runtime, webhooks and Secret-resolution code are excluded. No full Google operator dependency, Prometheus fork replacement or existing module-version upgrade is needed. Cluster/global rule types remain deferred pending a recorded rule-scope requirement.
  - *Why the RHOBS "just change registration" trick does not transfer*: `RHOBS_MONITORING` needs no new vendoring because `monitoring.rhobs` is **schema-identical** to `monitoring.coreos.com` — it reuses the *same* `prometheus-operator` Go structs (`ServiceMonitor`/`PodMonitor`/`PrometheusRule`) under a different GroupVersion, so re-registering the existing types produces valid output. GMP is **not** a renamed CoreOS schema: it has different Kinds (`PodMonitoring`/`Rules`), pod-based `spec.endpoints[]` instead of Service-based selection, `tls` vs `tlsConfig`, Secret-only CA refs, etc. Re-registering the CoreOS structs under the `monitoring.googleapis.com` group would serialize CoreOS fields under a GMP `apiVersion`, which the `gke-gmp-system` collectors cannot consume. Emitting *valid* GMP therefore requires compatible GMP data definitions (now supplied by the API-only package) or building the resources as `unstructured` — the scheme-registration approach alone does not avoid this (see the API dependency alternatives and maintenance contract below). The CoreOS monitors reference their CA as a **ConfigMap** (`root-ca`/`etcd-ca`), which GMP cannot consume, so the GMP path must materialize CA-only **Secrets** from those public ConfigMaps. Phase 0 confirmed `gke-gmp-system` collectors can read HCP-local Secrets via a namespaced Role/RoleBinding — the earlier assumption that they couldn't was disproved.
- **Auth/discovery differences**: GMP `PodMonitoring` discovers pods directly (not via Services) and models TLS/auth differently from CoreOS monitors. Sensitive targets use mTLS client certificates — etcd (`etcd-metrics-client-tls` + `etcd-ca`) and kube-apiserver (`metrics-client` + `root-ca`); neither uses a bearer token. Both were validated end-to-end in Phase 0 (native GMP scrape, `up=1`).
- **The CoreOS→GMP mapping is not 1:1**: GMP has **no Service-based monitor** — only `PodMonitoring` (namespace-scoped) and `ClusterPodMonitoring` (cluster-wide), both pod-based. Both CoreOS kinds (`ServiceMonitor` *and* `PodMonitor`) therefore collapse onto `PodMonitoring`, and a `ServiceMonitor`'s Service→endpoints discovery must be re-expressed as a pod label selector that reproduces the same target set (Google's own docs note the ServiceMonitor→PodMonitoring field mapping is *incomplete*). Because `PodMonitoring.spec.endpoints` is a list, multiple CoreOS monitors that target the same pods may be **consolidated** into a single `PodMonitoring` with several endpoints. Consequently parity cannot be defined as a per-file or per-kind "twin"; it must be defined as **scrape-target coverage** — every (component, port, metrics path, MetricSet) a CoreOS monitor scrapes must be covered by some GMP endpoint (see the implementation plan's parity test, Phase 7).

### Assumptions

- The metric keep/drop lists encoded in `support/metrics/sets.go` are portable to GMP (standard PromQL regex on `__name__`); only the surrounding relabel *type* differs.



## Alternatives Considered

The environment interface and reproducible API-only snapshot approach are retained. Runtime reconciliation, the supported GKE/GMP compatibility matrix and how candidate API updates are detected remain open. The configuration/authoring alternatives below are separate from the API dependency choice.

1. **Environment variable MONITORING_API_GROUP**: retained as the management deployment configuration interface. Install and CPO validation default to CoreOS, reject unsupported values and reject the GMP/RHOBS conflict. Both Deployment hops preserve this setting. Registration is additive, so init-time scheme selection is not the reason this interface is retained.
2. **CLI flag --monitoring-api-group**: outside the agreed foundation interface. Additive GMP registration removes the former scheme-init ordering objection; a future interface change would require explicit agreement and equivalent validation/propagation. No new flag is implemented.
3. **`HostedCluster` API field**: A spec field selecting the monitoring backend per hosted cluster. Most explicit and per-cluster granular, but requires an upstream API review, adds a customer-facing surface for what is really a management-cluster property, and is rejected by the ticket scope.
4. **Runtime conversion of CoreOS manifests to GMP**: Keep single-source CoreOS manifests and generate GMP resources on the fly via a conversion function. Avoids duplicated manifests, but creates fragile coupling where an innocuous CoreOS change silently produces invalid GMP output, and hides the real schema differences (Service- vs pod-based discovery, `tlsConfig`→`tls`, ConfigMap-CA→Secret-CA). Rejected for the *authoring* dimension regardless of the activation interface chosen.

### GMP API dependency alternatives

| Option | Assessment |
|--------|------------|
| Reproducibly extracted API-only snapshot | Selected. Provides typed objects and scheme/deepcopy support using existing HyperShift dependencies. The owning team maintains the extractor and validates source updates. |
| Import the upstream Google API package directly | The evaluated package couples data declarations to operator/configuration code. Its parent module's Prometheus replacement is not inherited by consumers, so importing it would require broader dependency changes to build. Reconsider if upstream provides a suitably independent API module. |
| Author YAML or unstructured objects without copied types | Avoids a Go API snapshot, but moves field/type checking to fixtures and runtime/schema validation. It still requires schema tracking and deployed-version compatibility checks; typed constructors and nested object handling would lose compiler checks. |

## Decision Rationale

* **Justification (activation interface)**: retain the agreed environment interface and existing CoreOS/RHOBS behavior. Share validation between install and CPO startup, and propagate the value through the two Deployment hops. Backend isolation belongs to the forthcoming runtime design, not to scheme registration or unused predicates.
* **Justification (separate manifests)**: separate assets/constructors keep discovery, TLS/auth and relabel differences independently reviewable. The per-workload versus centralized reconciliation choice is unresolved. Resolve pre-decode selection, monitoring error reporting/retry, HCP availability/upgrade policy and ownership-checked cleanup/recovery in both backend directions before expanding resources.
* **Justification (CA/RBAC lifecycle)**: the future GMP path must own CA-only Secrets and collector RBAC in the HCP namespace, track source CA/client-credential rotation, and clean up only owned resources. Decide which CPOv2 reconciliation path manages these objects together with monitors; registration and owner references alone do not implement rotation triggers or backend-switch cleanup.
* **Evidence**: the foundation provides reproducible upstream-derived types, deepcopies, additive registration, shared validation and Deployment/context propagation without changing dependency versions. Existing WithManifestAdapter/WithPredicate facilities are useful building blocks, but manifest loading precedes per-manifest predicates and ordinary component status aggregation includes all components. A centralized GMP component would still need an explicit monitoring-health policy to avoid blocking HCP availability/upgrades. Phase 0 proved manual authentication feasibility for etcd/KAS only; operator reconciliation is pending.
* **Comparison**: environment versus CLI configuration is separate from inert type registration. A HostedCluster field remains outside scope; runtime conversion of CoreOS manifests remains rejected in favor of explicit GMP assets. Reconciliation design must be selected and tested independently.

## Consequences

These are expected outcomes of full delivery. The foundation layer (type definitions, scheme registration, validation and environment/context plumbing) does not itself emit GMP resources, establish target/MetricSet/rule parity or remove dedicated Prometheus.

### Positive

* GKE management clusters can scrape hosted control plane and management-level metrics natively via GMP collectors — no per-cluster self-managed Prometheus.
* Default and `RHOBS_MONITORING=1` behavior is untouched; ROSA HCP and ARO HCP are unaffected.
* GMP and CoreOS manifests are independently reviewable; schema/auth differences are explicit, not hidden in a converter.
* A repository-level parity test makes it impossible to add a CoreOS monitor without a GMP equivalent (or an explicit waiver), preventing silent coverage gaps.
* Reuses existing MetricSet relabel logic, preserving Telemetry/SRE/All semantics with no duplicated allow/drop lists.
* Leaves the `RHOBS_MONITORING` path fully untouched — it stays in use for those who rely on it; this design neither depends on nor changes it.

### Negative

* Adds a second, hand-maintained set of monitoring manifests covering all current scrape targets (11 ServiceMonitors + 8 PodMonitors + 1 PrometheusRule at CPO level, plus 2 management-level monitors — 1 ServiceMonitor for the HyperShift operator and 1 PodMonitor for external-dns — and 2 management-level PrometheusRules). Because the mapping is not 1:1 — `ServiceMonitor`/`PodMonitor` both collapse to `PodMonitoring` and same-pod targets may be consolidated into one `PodMonitoring` with multiple endpoints — the GMP side is likely *fewer* files than the CoreOS side, but it remains a parallel surface to keep in sync. Mitigated by the coverage-based parity test and shared relabel logic.
* Adds ownership of an API/schema snapshot and its extraction tooling to the GCP HCP team. Reproducible generation, CI verification and compatibility review mitigate drift; they do not detect upstream releases or prove live compatibility. It adds no Google operator-module runtime dependency or shared dependency-version change.
* ServiceMonitor→PodMonitoring is not a mechanical swap: Service-based discovery, port naming, and (for etcd/kube-apiserver) mTLS client-certificate auth must be re-expressed and validated per target. The risk that GMP collectors could not present etcd client certificates — originally the primary implementation risk — was resolved in Phase 0: with a NetworkPolicy allowance for etcd TCP/2381 and CA-only Secrets, `gke-gmp-system` collectors scraped both etcd and kube-apiserver natively (`up=1`). Phase 0 validated only these two sensitive targets; the remaining components still require per-target auth/discovery translation.
* NetworkPolicy must be widened, only on the GMP path, to admit `gke-gmp-system` collectors; getting the selector wrong either blocks scraping or over-exposes control plane pods. Phase 0 validated a concrete collector selector (namespace `gke-gmp-system` + pod label `app.kubernetes.io/name=collector`).
* GMP evaluates `Rules` via a managed in-cluster `rule-evaluator` (deployed in `gke-gmp-system`) that **queries the Monarch backend** for the underlying time series, rather than a self-managed in-cluster Prometheus that both stores and evaluates. Evaluating against a remote backend (rather than local TSDB) may change alert timing/semantics versus locally-evaluated `PrometheusRule`s.

## Cross-Cutting Concerns

The following describes intended migration behavior and later validation requirements, not capabilities delivered by the foundation layer.

### Reliability

* **Scalability**: Native GMP collection removes the self-managed Prometheus as a per-management-cluster scaling bottleneck and shifts time-series *storage* to Google-managed Monarch (rule evaluation stays in-cluster via the managed `rule-evaluator`, which queries that backend). Metric volume moves directly into Cloud Monitoring ingestion quotas — sizing and cost are owned by the follow-up ops story.
* **Observability**: The change is to the monitoring plumbing itself; success is measured by GMP scrape coverage/parity against the current Prometheus target set (validated in the follow-up story). A parity unit/repo test guards code-level coverage.
* **Resiliency**: Evaluating rules against the Monarch backend (via the managed `rule-evaluator`) means recording/alerting rules survive management-cluster-local Prometheus failures (there is no such Prometheus on the GMP path), but introduces dependence on GMP backend availability and query/evaluation latency.

### Security

* Preserves TLS and authentication semantics for sensitive targets. Both etcd and kube-apiserver require mTLS with client certificates (`etcd-metrics-client-tls` and `metrics-client` respectively); neither uses a bearer token. GMP models TLS differently (`tls` vs `tlsConfig`, `key`/`cert`/`ca` as Secret references, no ConfigMap-CA support — so the CoreOS ConfigMap CAs must be mirrored into Secrets), so each sensitive target's auth must be re-expressed and verified rather than assumed equivalent. Phase 0 confirmed this path works for both targets.
* NetworkPolicy is widened **only** when GMP is selected, using stable `gke-gmp-system` namespace/pod selectors; the default, ROSA HCP, and ARO HCP policies are unchanged (`reconcileOpenshiftMonitoringNetworkPolicy`, `network_policies.go:576-593`). This reconciler runs in the **hypershift-operator** — a third binary distinct from the two that *emit* monitors — so it must read the same `MONITORING_API_GROUP` selection independently to know when to widen the policy (plumbing detailed in the implementation plan).
* Validation fails fast if `MONITORING_API_GROUP=monitoring.googleapis.com` and `RHOBS_MONITORING=1` are combined, preventing an ambiguous/insecure mixed configuration. This is enforced at `hypershift install`/`render` time and at control-plane-operator startup today. The hypershift-operator is a *third* binary that reads the selection (to gate the NetworkPolicy, Phase 6); when that read lands it must reuse the same parser so every binary consuming the selection also validates it.

### Cost

* Native GMP ingestion is billed per sample in Cloud Monitoring; removing the self-managed Prometheus removes its compute/storage footprint but exposes raw ingestion volume to GMP quotas. Aggressive MetricSet filtering (Telemetry/SRE) remains the primary cost lever and is preserved by reusing the existing relabel functions. Quantified cost/quota analysis is explicitly deferred to the follow-up ops story (see the [hybrid GMP cost analysis](observability-google-managed-prometheus.md#cost) for the current baseline).

### GMP API maintenance and compatibility

The GCP HCP team owns the API snapshot, extraction tooling and compatibility validation. Package and tooling changes are reviewed by `gcp-reviewers` through the normal HyperShift review process. Maintainers accepted the API-only approach on the condition that updates remain reproducible; this does not approve the remaining runtime design.

HyperShift's `support/gmpmonitoring/README.md` is the canonical maintenance runbook. Its source pin is `hack/gmp-api-source/go.mod`; `make gmp-sync` regenerates types, deepcopies, license, schema fixtures and provenance. `make verify-gmp-sync` checks reproducibility in temporary storage without rewriting tracked files, and CI runs it for relevant GMP or dependency/tool changes. Ordinary `make generate` continues to regenerate deepcopies offline. Do not edit generated types or checksums by hand. Unexpected runtime dependencies or extraction failures require review by the owning team.

Choose source updates by reviewing relevant API/schema changes for the supported GKE/GMP environments, then regenerate, run verification and focused tests, and include compatibility evidence in the update PR. The copied version does not need to numerically match a deployed GMP image. Compatibility depends on the fields HyperShift emits and the schemas and behavior supported by the deployed components. Record the GKE version, GMP operator image and validation performed. Offline schema checks do not establish live admission, scraping or rule-evaluation compatibility.

How candidate updates are detected remains an open maintenance decision. No review cadence or release-monitoring automation is established here, and a new operator or collector release alone does not require a snapshot update. The maintenance workflow is implemented and validated locally on the GCP-1292 branch; it is not merged or deployed. The runbook lives in that local HyperShift change until it is published.

### Operability

* Selection is visible in the rendered Deployment (the env var appears in the container spec), aiding debugging and drift detection. In the GCP delivery path the value is set on the `hypershift install render` invocation (so render-time validation runs and the value is reproducible from source), **not** injected as a kustomize overlay on the rendered Deployment — an overlay bypasses that validation.
* Cutover is staged: this story ships the *capability* (dark by default); a follow-up ops story enables GMP on GKE management clusters, validates live scrape/parity, and removes the dedicated Prometheus from gcp-hcp-infra.
* **Activation triggers a one-time fleet-wide CPO restart**: when GMP is selected, the hypershift-operator stamps `MONITORING_API_GROUP` onto every managed control-plane-operator Deployment, so each CPO pod restarts once. This is a management-side reconcile pause only — it does not restart hosted control-plane components (KAS/etcd) or worker nodes, and the selection does **not** feed the NodePool config hash (so no NodePool rollout). The default/CoreOS path omits the env var and is a no-op (no restart). Sequence this with the cutover ops story so on-call expects the reconcile spike.
* **Mixed-CPO behavior during transition**: the CPO ships in the OCP release payload, so a management cluster runs a mix of CPO versions. An older, pre-GMP CPO ignores `MONITORING_API_GROUP` and keeps emitting CoreOS monitors; a GMP-aware CPO emits GMP `PodMonitoring` (and drops its CoreOS monitors). During the transition the dedicated Prometheus shim continues to cover the CoreOS-emitting (older) clusters while GMP collectors cover the GMP-emitting (newer) clusters, so there is no coverage gap — **provided the dedicated Prometheus is not removed until every CPO on the management cluster is GMP-aware.** Removing the shim while any pre-GMP CPO remains would leave those clusters unmonitored. This is a hard sequencing constraint on the cutover ops story.
* HyperShift repository documentation must be updated to cover installing the GMP CRDs and using the new `MONITORING_API_GROUP` env var (tracked as documentation tasks under GCP-1227).


## Related Documentation

- Canonical maintenance runbook: `support/gmpmonitoring/README.md` in [HyperShift](https://github.com/openshift/hypershift), currently in the local GCP-1292 foundation change; use the published implementation revision when available.
- Implementation plan: [gcp-1227-hypershift-gmp-monitoring](../../implementation-plans/gcp-1227-hypershift-gmp-monitoring.md) *(runtime design and migration remain proposed)*
- Supersedes/evolves: [observability-google-managed-prometheus](observability-google-managed-prometheus.md) (hybrid self-managed Prometheus + GMP export)
- Related: [integrated-alerting-framework](integrated-alerting-framework.md)
- Jira: [GCP-1227](https://redhat.atlassian.net/browse/GCP-1227), [GCP-423](https://redhat.atlassian.net/browse/GCP-423)
- Reference: [GMP setup](https://cloud.google.com/stackdriver/docs/managed-prometheus/setup-managed), [GMP config (PodMonitoring/Rules schema)](https://cloud.google.com/stackdriver/docs/managed-prometheus/config)
