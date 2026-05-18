# Cloud CDN for Public OIDC Document Serving

***Scope***: GCP-HCP

**Date**: 2026-05-18

**Related**: [GCP-621](https://redhat.atlassian.net/browse/GCP-621), [GCP-588](https://redhat.atlassian.net/browse/GCP-588)

## Decision

We will serve OIDC discovery documents through **Cloud CDN with a Backend Service and Private Origin Authentication** rather than exposing GCS bucket URLs directly or relying on in-cluster proxies. The CDN fronts a private GCS bucket, keeping the bucket locked down while providing a publicly accessible HTTPS endpoint that GCP STS and other OIDC consumers can reach without authentication.

## Context

- **Problem Statement**: GCP STS requires a publicly accessible HTTPS URL to fetch OIDC discovery documents (`/.well-known/openid-configuration` and `/openid/v1/jwks`) for Workload Identity Federation. The HyperShift Operator uploads these documents to a private GCS bucket per region, but the bucket cannot be made public because the GCP org policy constraint `constraints/iam.allowedPolicyMemberDomains` prohibits granting access to `allUsers`. We need infrastructure that serves the private bucket's contents at a public HTTPS endpoint without requiring org policy exceptions.

- **Constraints**:
  - Org policy (`constraints/iam.allowedPolicyMemberDomains`) blocks `allUsers` and `allAuthenticatedUsers` IAM bindings on GCS buckets
  - GCP STS makes unauthenticated HTTPS GET requests — clients cannot use signed URLs or bearer tokens
  - Solution must work independently of GKE cluster lifecycle (clusters may be created/destroyed frequently)
  - Must serve documents for all Hosted Clusters in a region from a single endpoint
  - Infrastructure must be provisionable via Terraform without manual steps

- **Assumptions**:
  - OIDC documents are small (~1KB each) and change only on key rotation (infrequently)
  - The bucket contents (JWKS public keys and OpenID configuration) are not sensitive — they are designed to be public
  - One CDN endpoint per region is sufficient; multiple Management Clusters in the same region share the bucket

## Alternatives Considered

1. **Public GCS Bucket (direct URLs)**: Remove `public_access_prevention`, grant `allUsers:objectViewer`, and point the issuer URL directly at `https://storage.googleapis.com/{bucket}/{infraID}/`.

2. **Backend Bucket CDN**: Use `google_compute_backend_bucket` with Cloud CDN. Google's `cloud-cdn-fill` service agent authenticates cache-fill requests to the private bucket natively.

3. **OIDC Proxy Pod (nginx + GCS Fuse)**: Deploy an nginx pod on the region GKE cluster that mounts the bucket via GCS Fuse CSI driver and serves documents over HTTPS via GKE Ingress.

4. **Cloud Run Proxy**: Deploy a minimal Cloud Run service that reads from GCS using its attached service account and serves documents over HTTPS at a custom domain.

5. **Cloud CDN with Private Origin Authentication** (chosen): Backend Service + Internet NEG pointing to `storage.googleapis.com`, with a project-owned service account and HMAC key authenticating CDN cache-fill requests.

## Decision Rationale

* **Justification**: Cloud CDN with Private Origin Authentication is the only option that satisfies all constraints simultaneously: the bucket stays private (no org policy change), the endpoint is publicly accessible without client-side authentication, the infrastructure is cluster-independent (pure Terraform), and it provides global edge caching. The HMAC key risk is minimal — it grants read-only access to a single bucket whose contents are intentionally public data (JWKS public keys).

* **Evidence**: We deployed and tested all viable options in the dev environment:
  - **Backend Bucket CDN** was blocked: after resolving the org policy constraint for the `cloud-cdn-fill` service agent, we discovered that GCS only honours authenticated cache fills when clients use **signed URLs**. Unauthenticated requests (which is how GCP STS fetches OIDC documents) result in anonymous cache fills that return 403. This is a fundamental limitation of the Backend Bucket CDN model.
  - **Private Origin CDN** was deployed end-to-end in dev and confirmed working — unauthenticated clients receive cached OIDC documents with sub-millisecond latency.
  - **OIDC Proxy Pod** was deployed as an interim workaround and works, but introduced a cluster lifecycle dependency — OIDC availability is tied to the region GKE cluster.
  - See [oidc-public-issuer-options.md](../experiments/gcp-588-public-bucket/oidc-public-issuer-options.md) for detailed test results and comparison.

* **Comparison**:
  - **vs Public GCS Bucket**: Direct GCS URLs would be the simplest approach, but org policy (`constraints/iam.allowedPolicyMemberDomains`) blocks `allUsers` bindings. The workaround requires a tag-based conditional exception at the org or folder level — an org admin creates a tag key/value pair, adds a conditional `allowAll: true` rule to the policy, and each OIDC bucket must be explicitly tagged. This is fragile: any resource inadvertently tagged gets the same broad exception, the tag lifecycle must be managed across environments, and the setup depends on org admin availability. Even after all that, the bucket becomes fully public, weakening defence-in-depth for no functional gain over the CDN approach which keeps the bucket private.
  - **vs Backend Bucket CDN**: The native CDN-to-GCS integration is simpler (~12 vs ~14 Terraform resources), but it only authenticates cache fills for signed-URL requests. Since GCP STS makes plain HTTPS GETs, this option is non-functional for our use case.
  - **vs OIDC Proxy Pod**: The proxy works but couples OIDC availability to the region GKE cluster lifecycle. If the cluster is unhealthy or being upgraded, OIDC discovery could be unavailable — breaking WIF token exchange for all Hosted Clusters in that region. There is also no edge caching; every request hits the pod.
  - **vs Cloud Run Proxy**: Cloud Run is cluster-independent and serverless, but requires maintaining a custom container image (build pipeline, CVE patching). It also has cold-start latency (~200-500ms) after scaling to zero. For a critical-path endpoint like OIDC discovery, always-warm infrastructure is preferable.

## Consequences

### Positive

* Bucket remains private with `public_access_prevention = "enforced"` — no org policy changes required
* Entirely managed by Terraform — no pods, containers, or application code to operate
* Independent of any GKE cluster lifecycle — CDN continues serving even if all clusters in the region are down
* Global edge caching provides sub-millisecond latency from any GCP region or edge PoP
* Near-100% cache hit rate (OIDC documents are tiny and rarely change)
* Single CDN endpoint serves all Hosted Clusters in a region — scales to thousands of clusters without additional infrastructure
* Standard GCP resources with well-understood operational model (load balancer, CDN, DNS)

### Negative

* Introduces an HMAC key (access key + secret) per region, though stored as part of the already existing encrypted Terraform state backend
* HMAC key may need occasional rotation as a best practice, though not a security necessity — the key grants read-only access to data that is already served publicly via CDN
* Slightly more Terraform resources (~14) than a native Backend Bucket CDN (~12), though the latter is non-functional for our use case
* Google-managed SSL certificate takes 15-60 minutes to provision on initial deployment — non-blocking, activates automatically once DNS propagates

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Cloud CDN scales transparently with Google's global edge network. No capacity planning needed. A single CDN endpoint serves all Hosted Clusters in a region — adding clusters requires no CDN changes.
* **Observability**: CDN cache hit/miss metrics via Cloud Monitoring, GCS data access logs for cache-fill auditing, HTTP load balancer request logs for traffic visibility.
* **Resiliency**: CDN serves cached responses even if GCS is temporarily unavailable (24-hour serve-while-stale). No single pod or cluster dependency. GCP SLA covers CDN and load balancer availability.

### Security:

* Bucket stays private — defence-in-depth even though the contents are public keys
* HMAC key has minimal blast radius: read-only access to a single bucket containing non-sensitive data
* TLS is enforced end-to-end: Google-managed certificate on the load balancer, HTTPS to GCS origin
* HTTP-to-HTTPS redirect prevents accidental plaintext access
* No bucket listing possible — uniform bucket-level access with only `objectViewer` permission

### Performance:

* Sub-millisecond response times for cached OIDC documents from CDN edge PoPs worldwide
* Near-100% cache hit rate: documents are ~1KB, change only on key rotation, cached with 1-hour TTL
* Cache-fill requests to GCS are rare and have negligible latency impact
* No cold-start or pod-scheduling delays — infrastructure is always warm

### Cost:

* ~$18/month per region for the global forwarding rule (fixed cost regardless of traffic)
* CDN egress and cache-fill costs are negligible given the small document sizes and high cache hit rates
* Static IP, SSL certificate, and DNS are free
* No compute costs (no pods, no Cloud Run instances)
* Cost-equivalent to every alternative (~$18/month for the load balancer or Ingress)

### Operability:

* Entire stack is Terraform-managed — deploy, update, or destroy with standard `terraform apply`
* Feature-gated via `enable_oidc_cdn` variable for incremental rollout
* No container images to build, patch, or maintain
* No pod scheduling, health checks, or scaling to manage
* HMAC key rotation follows standard Terraform lifecycle (`taint` and `apply`)
* SSL certificate provisions automatically — no manual certificate management

---

## Template Validation Checklist

### Structure Completeness
- [x] Title is descriptive and action-oriented
- [x] Scope is GCP-HCP
- [x] Date is present and in ISO format (YYYY-MM-DD)
- [x] All core sections are present: Decision, Context, Alternatives Considered, Decision Rationale, Consequences
- [x] Both positive and negative consequences are listed

### Content Quality
- [x] Decision statement is clear and unambiguous
- [x] Problem statement articulates the "why"
- [x] Constraints and assumptions are explicitly documented
- [x] Rationale includes justification, evidence, and comparison
- [x] Consequences are specific and actionable
- [x] Trade-offs are honestly assessed

### Cross-Cutting Concerns
- [x] Each included concern has concrete details (not just placeholders)
- [x] Irrelevant sections have been removed
- [x] Security implications are considered where applicable
- [x] Cost impact is evaluated where applicable

### Best Practices
- [x] Document is written in clear, accessible language
- [x] Technical terms are used appropriately
- [x] Document provides sufficient detail for future reference
- [x] All placeholder text has been replaced
- [x] Links to related documentation are included where relevant
