# Hosted Cluster API Availability via Batched Synthetic Monitors

***Scope***: GCP-HCP

**Date**: 2026-11-13

**Jira**: [GCP-346](https://redhat.atlassian.net/browse/GCP-346) (Spike) under [GCP-335](https://redhat.atlassian.net/browse/GCP-335) (Epic)

## Decision

Monitor every hosted cluster's kube-apiserver `/livez` with Cloud Monitoring synthetic monitors. Each management cluster runs two of them:

- **PSC prober (every hosted cluster):** probes each cluster through a platform-owned Private Service Connect endpoint.
- **Public prober (PublicAndPrivate clusters only):** probes each cluster's public API endpoint.

Both probers discover their targets from Service Directory, which Gecko populates with Config Connector resources. Each writes a per-cluster availability metric, `custom.googleapis.com/hcp/api_up{cluster_id, path}`, and one alerting policy per management cluster raises a separate alert for each failing cluster and path.

## Context

- **Problem Statement**: We have no external, Red Hat-owned signal that a hosted cluster's API server is reachable. We need to learn that a cluster's API is unavailable before the customer does, and to report availability per cluster over time ("cluster X had Y% API availability this month"). Private clusters must be covered over the private network, using Google Cloud services.
- **Constraints**:
  - **Google Cloud native.** Cloud Monitoring synthetic monitoring and alerting. No in-cluster probing (blackbox exporter / Prometheus `probe_success`).
  - **Responsibility boundary.** The customer project is Red Hat's zero-responsibility line. Failures anywhere on Red Hat's side (management cluster, private router, internal load balancer, PSC service attachment and NAT, kube-apiserver) must be detected. Failures inside the customer project (their PSC endpoint, DNS) are out of scope.
  - **Every region.** Management clusters will exist in every GCP region, so the design cannot depend on region-specific behaviour.
  - **No globally reachable internal load balancers.** Enabling internal load balancer global access on every hosted cluster's private router is expected to meet security pushback.
  - **Per-cluster signal.** Availability must be queryable per cluster ID, and alerts must be per cluster, never merged across clusters.
  - **Cost.** Monitoring cost must stay small relative to the revenue of small clusters.
  - **Scale.** Up to 100 hosted clusters per management cluster (one GCP project and one GKE cluster per management cluster).
- **Assumptions**:
  - Every hosted cluster, in both endpoint access modes, has a PSC service attachment named `psc-<clusterID>` in the management cluster project. HyperShift creates it today.
  - `/livez` on the hosted cluster's kube-apiserver answers anonymously with `200 ok`. Verified on OCP 5.0.0-ec.6.
  - **Endpoint choice: `/livez`.** It reports whether the API server process is healthy, and it doesn't fail on purpose during graceful shutdown the way `/readyz` does. That avoids false outages when a probe hits a terminating replica during a rolling restart. It is also the conventional external liveness endpoint for hosted control planes.
  - The public API hostname is `api.<clusterName>-<slug>.<baseDomain>`, which Gecko already builds when creating the cluster.
  - Config Connector runs in cluster mode on each management cluster, with an identity that can manage compute, Service Directory and monitoring resources in the management cluster project.

## Alternatives Considered

1. **Per-cluster uptime check on the public endpoint**: one Cloud Monitoring uptime check per PublicAndPrivate cluster, from three Google regions. Works, and alerts in about 3.5 minutes. Does not cover Private clusters. Costs about $39 per cluster per month.
2. **Per-cluster private uptime check on the private-router internal load balancer**: an uptime check with `checkerType: VPC_CHECKERS` targeting each cluster's internal load balancer through Service Directory.
   - Works only when internal load balancer global access is enabled, and HyperShift does not enable it. That would be needed on every hosted cluster in every region outside the six regions where Google runs private checkers.
   - Blind to failures in Red Hat's PSC data path.
   - About $39 per cluster per month.
   - Config Connector cannot express private uptime checks.
3. **Per-cluster private uptime check through a platform-owned PSC endpoint**: not viable. Cloud Monitoring private checkers cannot reach PSC consumer endpoints (they time out), even with PSC global access, although VMs in the same VPC reach the endpoint from any region.
4. **Per-cluster synthetic monitors**: one Cloud Run function and monitor per cluster. Reaches PSC endpoints, but costs about $52 per cluster per month and needs one deployed function per cluster.
5. **Batched synthetic monitors per management cluster (chosen)**: one PSC prober and one public prober per management cluster, each probing all of that management cluster's hosted clusters every minute and writing a per-cluster metric.
6. **In-cluster probing (e.g. blackbox exporter)**: excluded by the Google-native constraint, and it would not exercise Red Hat's PSC path.

## Architecture

```
Management cluster project (one per MC, every region)
┌───────────────────────────────────────────────────────────────────────────────┐
│  Cloud Monitoring                                                             │
│   ├─ synthetic monitor "hc-api-psc"     (60s) ──► Cloud Run fn prober-psc     │
│   └─ synthetic monitor "hc-api-public"  (60s) ──► Cloud Run fn prober-public  │
│                                                                               │
│  prober-psc ── Direct VPC egress ──► monitoring VPC (MC region)               │
│      │  1. list Service Directory services hc-<clusterID>                     │
│      │  2. GET https://<PSC endpoint IP>/livez  ──► psc-<clusterID>          │
│      │       (service attachment → private-router ILB → kube-apiserver)       │
│      └─ 3. write api_up{cluster_id, path="psc", management_cluster}           │
│                                                                               │
│  prober-public                                                                │
│      │  1. list Service Directory services hcpub-<clusterID>                  │
│      │  2. GET https://api.<name>-<slug>.<baseDomain>/livez (TLS verified)   │
│      └─ 3. write api_up{cluster_id, path="public", management_cluster}        │
│                                                                               │
│  Alerting policy (one per MC): api_up < 1 for 60s, grouped by                 │
│  cluster_id + path ──► PagerDuty + diagnose agent                             │
└───────────────────────────────────────────────────────────────────────────────┘
```

### Components

| Component | Owner / mechanism | Per |
|---|---|---|
| Monitoring VPC + subnet (MC region), probers' service accounts, Cloud Run functions, synthetic monitors, Service Directory namespace `hcp-uptime` | Terraform, `terraform/modules/management-cluster` (gcp-hcp-infra) | MC |
| Prober source (one codebase, `MODE=psc\|public`) | Own repository, versioned and released through Konflux; Terraform deploys a pinned version | — |
| PSC endpoint (`ComputeAddress` + `ComputeForwardingRule` → `psc-<clusterID>`), Service Directory service `hc-<clusterID>` + endpoint (`addressRef` → the address) | Config Connector resources shipped by Gecko with the hosted cluster manifests; removed with the cluster | HC |
| Service Directory service `hcpub-<clusterID>` + address-less endpoint named `<clusterName>-<slug>` | Config Connector, shipped by Gecko for PublicAndPrivate clusters only | HC |
| Service attachment accept list includes the MC project | HyperShift change (upstream PR) | HC |
| Alerting policy on `api_up` | `MonitoringAlertPolicy` via Config Connector/ArgoCD, per the [integrated alerting framework](integrated-alerting-framework.md) | MC |

### Lifecycle

1. Gecko creates a hosted cluster and ships its manifests, now including the PSC endpoint and Service Directory resources above.
2. HyperShift creates the service attachment `psc-<clusterID>` and accepts the platform project. Config Connector retries until the attachment exists, then the PSC connection is accepted and Service Directory holds the endpoint IP.
3. The next prober run (≤ 60s) discovers the cluster and starts writing `api_up` for it.
4. On deletion, Gecko's existing cleanup removes the Config Connector resources, so the cluster disappears from the probers' target lists. No per-cluster Monitoring objects need to be created or deleted.

### Signal and reporting

* **Metric**: `custom.googleapis.com/hcp/api_up` (gauge, 1/0) with labels `cluster_id` (HyperShift `spec.clusterID`), `path` (`psc` | `public`) and `management_cluster`, on monitored resource `global`. Up means HTTP 200 with body `ok`; the probe timeout is 5s. Endpoint IPs are deliberately not labels, so a cluster's history never splits.
* **Per-cluster availability** (PromQL, usable from Grafana):
  ```promql
  avg by (cluster_id, path) (
    avg_over_time({"__name__"="custom.googleapis.com/hcp/api_up","monitored_resource"="global"}[30d]))
  ```
* **Prober health**: a run fails only when discovery or the metric write fails, never because a cluster is down. Each monitor's own `uptime_check/check_passed` drives a separate "prober unhealthy" alert, so a broken prober is never mistaken for healthy clusters.

### Alerting

* One `MonitoringAlertPolicy` per MC project: `api_up` minimum < 1 for 60s, grouped by `cluster_id` and `path`. Cloud Monitoring opens one alert per (cluster, path). A PublicAndPrivate cluster whose API is fully down produces two alerts.
* Alerts carry `cluster_id` and `path` as labels, and the policy documentation templates `${metric.label.cluster_id}`.
* **Routing**: both paths page (PagerDuty critical) and trigger the diagnose agent, following the [integrated alerting framework](integrated-alerting-framework.md).
* The "prober unhealthy" policy routes to the same channels.
* **Alert latency (~4 min)** = up to 60s until the next probe run, plus a 60s policy duration (effectively two consecutive failed probes), plus about 2 min of Cloud Monitoring ingestion and alert evaluation, which we can't tune. Measured: outage at 15:07:04, first failed sample ~15:07, alert at 15:11:00. Setting the duration to 0s saves about 1 min, but a single failed probe would page, which is too noisy given frequent cluster state changes. A longer duration trades latency for fewer false pages. The floor is about 3 min.
* SLO targets and burn-rate alerting are out of scope; they belong in a separate SLO decision that consumes `api_up`.

## Decision Rationale

* **Justification**:
  * This is the only option that meets every constraint at once:
    * detects Red Hat-side failures, including the PSC data path;
    * works in every region without internal load balancer or PSC global access;
    * gives per-cluster metrics and alerts;
    * costs about $1 per hosted cluster per month.
  * Discovery through Service Directory and Config Connector means no per-cluster Monitoring objects, so the 100 uptime-checks-per-project quota does not apply and there is no create/delete lifecycle to get wrong.
* **Evidence** (GCP-346 spike, dev environment in northamerica-northeast2, 2026-11-12/13):
  * `/livez` and `/readyz` both answered anonymously `200 ok` on both access modes. The spike's probers measured `/readyz`. Every injected failure took the API fully down or cut the network path, so `/livez` would have failed identically. Probers will use `/livez`.
  * Detection by the batched probers, measured from the start of each injected failure to the alert opening:

    | Injected failure | Alerts | Time to alert |
    |---|---|---|
    | kube-apiserver scaled to 0 (Private cluster) | `{cluster, psc}` | 3m56s |
    | Red Hat-side PSC NAT path blocked | `{cluster, psc}`, only the affected cluster | 5m33s (delayed by a prior open alert) |
    | Public load balancer blocked | `{cluster, public}` only; the PSC path stayed healthy | 4m36s |
    | kube-apiserver down (PublicAndPrivate cluster) | separate `{cluster, psc}` and `{cluster, public}` | 4m09s / 4m38s |

  * The private uptime check on the internal load balancer (alternative 2) **did not detect** the PSC NAT failure. It also failed (`VPC_REQUEST_TIMEOUT`) until global access was enabled by hand.
  * Private uptime checks against a PSC endpoint (alternative 3) timed out from every checker region, with and without PSC global access. Diagnostic VMs in the same VPC reached the endpoint from the same region and from us-central1.
  * Config Connector created the PSC endpoint and Service Directory entry end to end. `ComputeForwardingRule` needs `ipAddress.addressRef.external` set to the address URL; a name reference resolves to the raw IP and the API rejects it with a 400. The PSC connection was accepted once the MC project was on the attachment's accept list.
  * Per-cluster availability PromQL returned the expected values for both test clusters. Alerts carried `cluster_id` and `path`.
* **Comparison**:

  | | Private clusters | RH-side PSC failures | Every region w/o global access | Per-cluster objects | Cost / HC / month |
  |---|---|---|---|---|---|
  | 1. Public uptime check | no | no | yes | uptime check | ~$39 |
  | 2. Private uptime check → ILB | yes | **missed** | **no** (needs ILB global access) | uptime check + SD (not expressible in KCC) | ~$39 |
  | 3. Private uptime check → PSC | — | — | — | not viable | — |
  | 4. Per-cluster synthetic monitor | yes | yes | yes | function + monitor | ~$52 |
  | **5. Batched synthetic monitors** | **yes** | **yes** | **yes** | **KCC PSC + SD entries only** | **~$1** |
  | 6. In-cluster probing | — | no | yes | — | excluded |

## Consequences

### Positive

* Red Hat learns about API outages on either path within about 4–5 minutes, per cluster, before customers report them. This includes Private clusters, probed over the private network.
* Covers Red Hat-side PSC failures (attachment, NAT, firewall). For PublicAndPrivate clusters it also covers the node-to-control-plane PSC path, which a public-only check would miss.
* No internal load balancer global access, and no per-cluster Monitoring objects, quota or lifecycle.
* About $104 per MC per month for both probers, roughly $1 per cluster at 100 clusters, against roughly $3,900 per MC with per-cluster uptime checks.
* One metric gives per-cluster availability reporting, dashboards and alerting.
* Everything per-cluster is created by Config Connector through Gecko's existing manifest transport.

### Negative

* We own prober code (discovery, probing, metric writes), with its tests, releases and on-call runbook, where uptime checks would have been configuration only.
* Probes come from a single location, the MC's region. A regional Google Cloud problem could look like an outage. This is accepted for simplicity, and the prober-health alert helps distinguish the two.
* Alert latency is about 4–5 minutes (60s probes plus Cloud Monitoring alert evaluation), roughly a minute slower than uptime checks.
* Requires an upstream HyperShift change: the MC project on each service attachment's accept list.
* Adds one PSC endpoint, and one IP in the monitoring subnet, per hosted cluster.
* Config Connector lacks Service Directory annotations, so the public prober depends on a naming convention (endpoint name `<clusterName>-<slug>`) plus a per-MC `BASE_DOMAIN`.
* If a region has more than one base domain, discovery must learn each cluster's domain (open question below).
* The availability metric is self-written. Missing data (prober down) must be treated explicitly in reporting rather than read as up or down.

## Cross-Cutting Concerns

### Reliability:

* **Scalability**:
  * Work per run grows linearly with hosted clusters per MC (≤ 100). Probes run in parallel with a 5s timeout; a 2-cluster run took about 1s.
  * The monitoring subnet needs one IP per hosted cluster (a /24 suffices for 100).
  * Limits: 1,000 simultaneously open alerts per policy, which is why there is one policy per MC; 30 labels per custom metric; 200k active series per custom metric on `global`.
* **Observability**:
  * Per-cluster `api_up` series.
  * Structured prober logs per target (cluster, target, status, latency).
  * The synthetic monitors' own pass/fail and run logs.
  * A "prober unhealthy" alert.
* **Resiliency**:
  * Probers are stateless and re-discover targets on every run.
  * A failed run does not hide cluster health: it fires the prober-health alert.
  * Cloud Run retries and scheduling are managed by Cloud Monitoring.

### Security:

* Prober service accounts get only `roles/servicedirectory.viewer` and `roles/monitoring.metricWriter`.
* Probers are invoked only by the Monitoring service agent (`roles/run.invoker`); there is no unauthenticated access.
* Function builds use a dedicated build service account, not the default compute account, which stays disabled.
* The monitoring VPC is platform-owned and contains only PSC endpoints and the probers' egress. Nothing in the customer project is touched.
* The accept-list change only lets the MC project connect to its own attachments. It stays `ACCEPT_MANUAL`, and the customer's connection is unaffected.
* Probes are anonymous `GET /livez`; no credentials are needed or stored.
* The public prober verifies TLS, so certificate failures count as down.

### Cost:

* Synthetic monitors: $1.20 per 1,000 executions, billed per execution regardless of how many clusters are probed. Two monitors × 43,200 runs/month ≈ **$104 per MC/month**.
* Cloud Run compute (a few seconds per run): estimated a few dollars per MC per month.
* Custom metric ingestion: about 8.6M points per MC per month at 100 clusters with both paths. Likely within or near the free ingestion tier; to be confirmed on billing.
* Per-cluster PSC endpoints: billed per endpoint-hour. To be priced during implementation.

### Operability:

* Adding a hosted cluster requires no monitoring-specific action. Monitoring follows Gecko's manifests.
* The prober is released through Konflux; Terraform pins the version per environment and promotes it through the normal pipeline.
* Runbook: per-(cluster, path) alerts link to prober logs; the `path` label tells PSC (Red Hat side) from public (public LB/DNS/certificate) failures.

## Open Questions

1. **Multiple base domains per region/MC**: can one MC host clusters under more than one base domain? If so, public discovery must carry the domain per cluster instead of using a per-MC `BASE_DOMAIN`.
2. **Konflux artifact vs. Cloud Run functions**: synthetic monitors target a Cloud Run function (gen2), which Google builds from source. Confirm whether a monitor can target a function deployed from a pre-built, Konflux-attested artifact, or define how a pinned, released source bundle is deployed.
3. **HyperShift accept-list API shape**: an operator-level setting (e.g. "accept the attachment's own project") rather than a per-cluster field.

## References

* [Integrated Alerting Framework](integrated-alerting-framework.md)
* [Observability: Google Managed Prometheus](observability-google-managed-prometheus.md)
* [GCP Private Service Connect](../networking/gcp-private-service-connect.md)
* [Datastore transport (Gecko → MC manifests)](../networking/datastore-transport.md)
* Jira: GCP-346 (spike), GCP-335 (epic)
* Cloud Monitoring: [synthetic monitors](https://cloud.google.com/monitoring/synthetic-monitors), [private uptime checks](https://cloud.google.com/monitoring/uptime-checks/private-checks), [quotas](https://cloud.google.com/monitoring/quotas)
