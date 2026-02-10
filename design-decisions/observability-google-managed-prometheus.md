# Observability Platform: Google Managed Prometheus for HCP Monitoring

***Scope***: GCP-HCP

**Date**: 2026-02-05

## Decision

We will implement a hybrid Google Managed Prometheus (GMP) architecture where self-managed Prometheus instances collect metrics from HyperShift Hosted Control Plane (HCP) components and export to GMP for long-term storage, combined with Google Cloud Stackdriver for load balancer golden signal metrics, to provide unified observability with regional data isolation and cost control through aggressive metric filtering.

## Context

### Problem Statement

GCP HCP requires platform-wide observability for HyperShift Hosted Control Planes with four golden signals (latency, traffic, errors, saturation) to enable SRE teams to monitor control plane health, detect incidents, and maintain service level objectives across multiple regions and hundreds of clusters. The solution must support regional data isolation while enabling global cross-region queries, operate within cost constraints at scale, and provide visibility into both HCP control plane components and Google Cloud infrastructure.

### Constraints

- **Regional data isolation required**: Metrics must remain in their home region project for compliance and data locality requirements
- **Global querying capability needed**: SRE teams must query metrics across all regions from a central observability stack
- **Cost management critical**: GMP sample-based pricing at scale requires aggressive filtering (100 HCPs = ~$93K/year baseline)
- **Golden signals mandatory**: Must collect latency, traffic, errors, and saturation for all control plane endpoints
- **Zero migration effort preferred**: Existing ServiceMonitors, PodMonitors, and PrometheusRules should work without modification
- **GKE environment**: Using GKE clusters with existing Prometheus Operator patterns from ROSA/ARO deployments
- **Alerting costs significant**: Alert query evaluation costs can exceed ingestion costs at scale

### Assumptions

- ROSA (Red Hat OpenShift Service on AWS) filtering strategies are applicable to GCP HCP architecture
- Billing account consolidation across management cluster projects will provide tier pricing benefits
- Self-managed Prometheus operational overhead is acceptable vs fully-managed GMP collection
- Grafana dashboards will be managed as code via GitOps (ArgoCD)
- Recording rules can reduce alerting query costs significantly
- Google's Prometheus fork respects standard `metric_relabel_configs` and `remoteWrite` `writeRelabelConfigs` for GCM export filtering

## Alternatives Considered

1. **Hybrid GMP (self-managed Prometheus + GMP storage)** - Deploy Prometheus with Google's GMP-enabled image to collect metrics and export to GMP for long-term storage
2. **GMP PodMonitoring (fully managed collection)** - Use Google's native PodMonitoring CRDs with managed DaemonSet collectors for full GCP-native solution
3. **Self-managed Prometheus only (no GMP)** - Deploy cluster-scoped Prometheus without GMP export, manage all storage and retention locally

## Decision Rationale

### Justification

The hybrid approach using self-managed Prometheus with GMP export provides the optimal balance of operational consistency, cost visibility, and cross-cloud compatibility. This architecture enables zero-effort migration from existing ROSA/ARO monitoring configurations, provides local metric visibility before GMP export for cost optimization, maintains consistency across all cloud platforms (GCP, AWS, Azure), and delivers a two-tier collection model where all metrics are available locally for debugging while only filtered essential metrics are exported to GMP for long-term retention.

### Evidence

Testing in GCP environment validated all acceptance criteria: 3 HCP namespaces with 42 ServiceMonitors/PodMonitors auto-discovered and metrics exported to GMP successfully; Argo CD and Tekton dashboards operational using both GMP and Stackdriver data sources; cost analysis based on ROSA staging data (136 HCP namespaces across 5 management clusters) shows $7,748/month for 100 HCPs with ROSA-like filtering vs $20,103/month unfiltered (61% reduction); global scoping project configured to query metrics across all regional projects while maintaining data isolation; Prometheus resource usage at test scale: 40m CPU, 280Mi RAM for 3 HCPs (projected 1-4 CPU, 4-8Gi RAM for 100 HCPs).

### Comparison

**GMP PodMonitoring requires rewrite of HyperShift monitoring templates**: Each HCP namespace contains 9 ServiceMonitors, 4 PodMonitors, and 1 PrometheusRule automatically deployed by HyperShift (13 monitors per cluster, ~1,300 total for 100 HCPs); migration requires updating HyperShift's template codebase to generate PodMonitoring and Rules CRD formats with different syntax and capabilities instead of ServiceMonitor/PodMonitor CRDs, creates configuration divergence between GCP and ROSA/ARO platforms where HyperShift must maintain separate monitoring implementations, requires network policy modifications in HyperShift's namespace templates to allow `gke-gmp-system` access, and provides no local metric visibility for cost optimization (metrics go directly to GMP with no filtering preview).

**Self-managed Prometheus without GMP lacks long-term storage**: no cross-region query capability, no 24-month retention for compliance, requires significant local storage infrastructure, and does not leverage GCP's native monitoring integration for Stackdriver metrics. The hybrid approach combines the benefits of both: zero migration effort, cross-cloud consistency, local cost visibility, and GCP-native long-term storage.

## Consequences

### Positive

* Zero migration effort - existing ServiceMonitors, PodMonitors, and PrometheusRules from ROSA work without modification
* Cost visibility before export - Prometheus UI shows metric cardinality and sample rates before export to GMP, enabling iterative filter optimization
* Cross-cloud consistency - identical monitoring configurations across GCP, ROSA (AWS), and ARO (Azure) platforms
* Two-tier collection architecture - all metrics available locally for debugging (6-24h retention), filtered subset in GMP for long-term analysis (24-month retention)
* Regional data isolation with global scoping - metrics stored in home region projects, global scoping project can query across all regions
* Recording rules reduce alerting costs - PrometheusRules evaluated locally, pre-aggregated metrics reduce GMP alert query costs by estimated 60%
* Stackdriver integration for load balancer metrics - Google-native golden signals (latency, traffic, errors) from GCE load balancers

### Negative

* Prometheus infrastructure management overhead - must deploy and maintain Prometheus Operator and Prometheus instances (not Google-managed)
* Uptime responsibility falls on SRE team - if Prometheus pods crash, team must diagnose and restart (vs Google managing collector pods)
* Aggressive metric filtering mandatory - ROSA-like filtering (drop histograms, container metrics, allowlist) required for cost viability (83% cost reduction vs unfiltered)
* Local storage requirements - 50Gi PVC recommended per management cluster for recording rule evaluation and short-term retention
* Filter maintenance burden - allowlist configuration must evolve as monitoring requirements change, requires monthly cost review
* GMP costs significant at scale - $70,856/month for 1,000 HCPs with ROSA filtering and standard alerting ($850K/year), estimated $47K/month with recording rules optimization
* Export lag during Prometheus failures - short metric delivery gap if Prometheus restarts (mitigated by local retention and quick recovery)

## Cross-Cutting Concerns

### Reliability

* **Scalability**: GMP handles metric scale natively with no capacity concerns; self-managed Prometheus resource requirements scale linearly with HCP count (estimated 1-4 CPU cores, 4-8Gi RAM per management cluster at 100 HCPs); billing account tier aggregation provides cost efficiency at scale (98.8% of samples at Tier 2 pricing for 1,000 HCPs)
* **Observability**: Prometheus UI accessible via port-forward for metric cardinality analysis and filter testing; GCP Console Metrics Management shows GMP sample counts and unused metrics; Grafana dashboards combine GMP datasource (long-term queries) and Stackdriver datasource (load balancer golden signals); Cloud Monitoring alerts for Prometheus pod health and GMP export lag
* **Resiliency**: Metrics exported to GMP survive Prometheus pod failures (short gap only); 24-month GMP retention for historical analysis; automatic GMP export retry with backoff; Prometheus local storage (6-24h retention) provides fallback for recent data queries; recording rules continue evaluation during GMP outages

### Security

- Workload Identity authentication for keyless GCP access (no service account keys in cluster)
- Regional data isolation enforced by GMP project boundaries (metrics never leave home region)
- Network policies restrict metric scraping to `openshift-monitoring` namespace only
- TLS encryption for all metric endpoints (certificates managed via ServiceMonitor/PodMonitor specs)
- GCP IAM controls access to GMP metrics (read-only access via global scoping project)
- Secret management via External Secrets Operator for OAuth credentials (IAP-protected dashboards)

### Performance

- 30-second scrape interval balanced for metric resolution vs sample volume cost
- 6-hour local Prometheus retention minimizes storage overhead while supporting recording rules
- GMP query performance adequate for dashboard refresh (3-5 second typical response time)
- Recording rules pre-compute expensive aggregations (reduces alert query latency)
- Local Prometheus queries for real-time debugging (sub-second response)
- Stackdriver metrics delivered with 60-second granularity (GCP infrastructure metrics)

### Cost

**Per Management Cluster (55 HCPs):**
- ROSA filtering baseline: $1,530/month ingestion + $2,470/month alerting = $4,000/month
- With recording rules (estimated): $1,530/month ingestion + $1,000/month alerting = $2,530/month

**100 HCPs (2 Management Clusters):**
- Ingestion: $3,048/month (51B samples, all in Tier 1)
- Alerting (standard): $4,700/month (15 alerts/HCP, 1-min evaluation)
- **Total: $7,748/month ($93K/year)**
- With recording rules (estimated): $5,400/month ($65K/year, 30% reduction)

**1,000 HCPs (19 Management Clusters):**
- Ingestion: $23,856/month (484.5B samples, 98.8% at Tier 2 rate)
- Alerting (standard): $47,000/month
- **Total: $70,856/month ($850K/year)**
- With recording rules (estimated): $47,000/month ($564K/year, 34% reduction)

**Critical cost factors:**
- ROSA filtering mandatory (drop histogram buckets, container metrics, allowlist) - reduces costs 83% vs unfiltered baseline
- Alerting costs exceed ingestion costs at scale without recording rules ($47K alerting vs $24K ingestion for 1,000 HCPs)
- Billing account tier aggregation provides modest savings (Tier 2: $0.048/M samples vs Tier 1: $0.06/M samples beyond 50B/month)
- Self-managed Prometheus compute costs: ~$150/MC/month (negligible vs GMP costs)

**Cost control mechanisms:**
- Prometheus-level metric filtering via ConfigMap (centralized allowlist, easy to update)
- Monthly cost review using GCP Metrics Management dashboard to identify unused metrics
- GCP budget alerts configured at $5,000/MC threshold for early warning
- Recording rules to reduce alerting query costs (pre-aggregate high-cardinality metrics)
- Drop histogram buckets (saves ~50% samples due to GMP sparse encoding)
- Metric relabeling to eliminate high-cardinality labels before export

### Operability

- **Deployment model**: Prometheus Operator and Prometheus instances deployed via ArgoCD with GitOps workflow
- **Metric filtering**: Allowlist configuration managed in ConfigMap, changes applied via ArgoCD sync (no manual kubectl required)
- **Dashboard management**: All Grafana dashboards checked into repository at `gcp-hcp/dashboards/`, auto-deployed on merge to main
- **Recording rules**: Standard PrometheusRules CRDs evaluated locally, results exported to GMP as pre-aggregated metrics
- **Cost monitoring workflow**: Monthly review of GCP Metrics Management dashboard → identify unused metrics → update allowlist → measure cost reduction
- **Incident response**: Local Prometheus UI for real-time queries (port-forward to prometheus-cluster-wide:9090), GMP for historical analysis
- **Troubleshooting tools**: Prometheus UI for cardinality analysis (`topk(20, count by (__name__) ({__name__=~".+"}))`), GCP Console for GMP sample ingestion rates, Grafana Explore for ad-hoc queries
- **Maintenance burden**: Prometheus Operator upgrades (quarterly), Prometheus version updates (as needed), monthly cost optimization review, allowlist tuning as requirements evolve
- **Runbook procedures**: Prometheus pod restart, GMP export lag investigation, cost spike response (check for configuration changes or cardinality explosion)

## Implementation Artifacts

The GCP-343 spike deliverables provide detailed implementation guidance:

- **Technical comparison and architecture**: [experiments/google-managed-prometheus/README.md](https://github.com/openshift-online/gcp-hcp/blob/main/experiments/google-managed-prometheus/README.md) - Detailed comparison of GMP PodMonitoring vs cluster-wide Prometheus, network policy requirements, recording rules comparison
- **Cost analysis and projections**: [experiments/google-managed-prometheus/COST-ANALYSIS.md](https://github.com/openshift-online/gcp-hcp/blob/main/experiments/google-managed-prometheus/COST-ANALYSIS.md) - Comprehensive cost breakdown at 10, 100, 500, 1,000 HCP scale, ROSA filtering baseline, alerting costs, recording rules impact estimates
- **Cost control strategy**: [experiments/google-managed-prometheus/GMP-COST-CONTROL-STRATEGY.md](https://github.com/openshift-online/gcp-hcp/blob/main/experiments/google-managed-prometheus/GMP-COST-CONTROL-STRATEGY.md) - Prometheus-level filtering implementation, allowlist examples, two-tier collection architecture, verification procedures

**Reference Dashboards:**
- Argo CD monitoring dashboard (operational)
- Tekton pipeline dashboard (operational)
- HCP control plane overview dashboard (in development)

**Global Scoping Project Configuration:**
- Project: `gcp-hcp-{env}-global` (one per environment)
- Scoping permissions: Read-only access to all regional management cluster projects
- Query pattern: PromQL queries with `project_id` label for cross-region aggregation

## Acceptance Criteria Coverage (Jira GCP-343)

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Monitoring backend selected and validated | ✓ Complete | Google Managed Prometheus (hybrid approach) validated in test environment |
| Four Golden Signals defined with collection | ✓ Complete | Latency/traffic/errors from Stackdriver (load balancers), saturation from Prometheus (HCP pods) |
| Global scoping project configured | ✓ Complete | Global project can query metrics across all regional projects |
| Regional data isolation validated | ✓ Complete | Metrics stored in home region project, cross-region queries via scoping only |
| Argo CD dashboard operational | ✓ Complete | Dashboard auto-deployed via GitOps, metrics from GMP datasource |
| Cost control knobs designed and documented | ✓ Complete | Prometheus-level allowlist filtering, GCP budget alerts, monthly cost review process |
| TCO measured for single MC | ✓ Complete | $4,000/month per MC with ROSA filtering (55 HCPs) |
| Cost projection model created | ✓ Complete | Projections at 100, 500, 1,000 HCPs with ingestion + alerting costs |
| Create implementation stories | ⏳ Future Work | Follow-on Jira tickets to be created post-design approval |
| Design doc written and merged | ✓ This Document | Design decision documented with rationale, trade-offs, and implementation references |

## Next Steps

1. **Approval and review**: Circulate design document for team review and stakeholder approval
2. **Create implementation stories**: Break down implementation into actionable Jira tickets (Prometheus deployment, metric filtering configuration, dashboard development, recording rules, runbooks)
3. **Pilot deployment**: Deploy to development environment with 5-10 HCPs, validate costs match projections
4. **Cost validation**: Run pilot for 30 days, analyze actual GMP costs vs estimates, tune allowlist filters
5. **Recording rules development**: Implement PrometheusRules for common aggregations to reduce alerting costs
6. **Dashboard expansion**: Develop additional dashboards for HCP components (kube-apiserver, etcd, controllers)
7. **Runbook development**: Create operational procedures for Prometheus troubleshooting, cost spike response, filter optimization
8. **Production rollout**: Deploy to integration environment, then stage, then production (one management cluster at a time)

---

**Related Documentation:**
- Jira Epic: https://issues.redhat.com/browse/GCP-343 - Observability Spike for HCP Monitoring
- Experiment Repository: [gcp-hcp/experiments/google-managed-prometheus/](https://github.com/openshift-online/gcp-hcp/tree/main/experiments/google-managed-prometheus)
