# OIDC Public Issuer URL: Options Analysis

## Problem

GCP STS requires a **publicly accessible HTTPS URL** to fetch OIDC discovery documents for
Workload Identity Federation. The HyperShift Operator uploads these documents to a **private
GCS bucket**. We need a way to serve them publicly.

GCP org policy constraints prevent making the bucket public, so we need an alternative
approach to serve these documents.

## Options

### Option 1: Backend Bucket CDN

Uses `google_compute_backend_bucket` to wrap the GCS bucket with Cloud CDN. A Google-managed
`cloud-cdn-fill` service account would authenticate cache fill requests to the private bucket.

```
GCP STS / Any Client
        │
        ▼  HTTPS
Global External Application Load Balancer
  ├── Static IP (global)
  ├── Google-managed SSL Certificate
  ├── Target HTTPS Proxy
  └── URL Map
        │
        ▼
Cloud CDN (edge caching, FORCE_CACHE_ALL)
        │
        ▼
Backend Bucket (CDN-enabled wrapper around GCS)
        │
        ▼  Authenticated via cloud-cdn-fill SA
Private GCS Bucket
  ├── {infraID}/.well-known/openid-configuration
  └── {infraID}/openid/v1/jwks
```

We deployed this end-to-end and hit two blockers:

1. **Org policy constraint**: CDN returned 403, so we proceeded to grant the `cloud-cdn-fill`
   SA `objectViewer` on the bucket. However, this was blocked
   by org policy (`HTTPError 412: ... do not belong to a permitted customer`). We resolved
   this    by implementing a custom constraint that allows the `cloud-cdn-fill` SA domain.

2. **Signed URLs required**: After resolving the org policy and successfully granting bucket
   access, CDN requests still returned 403. GCS data access audit logs showed CDN was making
   **anonymous** requests (`PRINCIPAL_EMAIL` empty). The `cloud-cdn-fill` SA only authenticates
   cache fills when clients use **signed URLs** -- not for unauthenticated requests. This is
   poorly documented by GCP but confirmed by multiple sources. Since OIDC consumers (GCP STS,
   K8s API servers) make plain HTTPS GET requests, signed URLs are not viable.

**To make this work**, the bucket would need to be publicly readable (`allUsers:objectViewer`),
which requires an org policy change. With a public
bucket, a Backend Bucket CDN works -- but the CDN layer becomes optional since direct GCS
URLs would also work.


---

### Option 2: HMAC CDN (Internet NEG + Backend Service)

Uses an Internet NEG pointing to `{bucket}.storage.googleapis.com` with HMAC Private Origin
Authentication (AWS Signature V4). A project-owned service account with HMAC keys
authenticates CDN cache fills. Clients access CDN normally -- no signed URLs needed.

```
GCP STS / Any Client
        │
        ▼  HTTPS
Global External Application Load Balancer
  ├── Static IP (global)
  ├── Google-managed SSL Certificate
  ├── Target HTTPS Proxy
  └── URL Map
        │
        ▼
Cloud CDN (edge caching, FORCE_CACHE_ALL)
        │
        ▼
Backend Service (EXTERNAL_MANAGED, HTTPS protocol)
  ├── HMAC Private Origin Auth (awsV4Authentication)
  └── Custom Host header: {bucket}.storage.googleapis.com
        │
        ▼
Internet NEG (FQDN: {bucket}.storage.googleapis.com:443)
        │
        ▼  HMAC-signed requests (S3 Signature V4)
Private GCS Bucket (via XML API / S3-compatible endpoint)
  ├── {infraID}/.well-known/openid-configuration
  └── {infraID}/openid/v1/jwks
```

OIDC documents served successfully via CDN in testing.

**Issues:**
1. **HMAC key management** -- access key + secret stored in Terraform state (encrypted
   GCS backend). Rotation is a hygiene best practice but not a security necessity since
   the key only grants read-only access to one bucket containing only JWKS public keys
   (inherently non-sensitive data)

**Advantages:**
- **No org policy change needed** -- project-owned SA is in allowed domain
- **Can deploy immediately** -- no dependency on org admin approval
- **Bucket stays private** -- no `allUsers` binding
- **CDN edge caching** -- global, sub-ms latency from any region


---

### Option 3: OIDC Proxy Pod (GCS Fuse + nginx)

Uses an nginx pod on the region cluster that mounts the private GCS bucket via GCS Fuse
CSI driver (read-only) and serves OIDC documents over HTTPS. A GKE Ingress with a
Google-managed certificate provides the external endpoint. The pod authenticates to GCS
via Workload Identity -- no keys or secrets needed.

```
GCP STS / Any Client
        │
        ▼  HTTPS
Static IP (pre-created in Terraform for stable DNS)
        │
        ▼
GKE Ingress (auto-creates remaining LB resources)
  ├── Global Forwarding Rule (binds static IP)
  ├── Target HTTPS Proxy
  ├── URL Map
  ├── Google-managed SSL Certificate
  └── Backend Service + Health Check
        │
        ▼  HTTP
K8s Service → oidc-proxy pods (nginx + GCS Fuse CSI sidecar)
        │
        ▼  Workload Identity
Private GCS Bucket
  ├── {infraID}/.well-known/openid-configuration
  └── {infraID}/openid/v1/jwks
```

Tested on GKE Autopilot -- OIDC documents served successfully via HTTP and HTTPS.

**Issues:**
1. **No edge caching** -- every request hits the pod. GCP STS may call from any region;
   latency depends on where the region cluster is
2. **Pod availability** -- depends on pod health, node scheduling, and GKE Autopilot
   capacity. Not inherently HA (though 2 replicas help)
3. **GCS Fuse latency** -- first reads go to GCS; subsequent reads use the Fuse cache
   layer. Nginx memory caching can help but adds config complexity

**Advantages:**
- **No org policy change needed** -- Workload Identity uses project-owned SA
- **No secrets to manage** -- keyless auth via Workload Identity
- **Simple concept** -- nginx serving files from a GCS mount
- **GCS Fuse built into GKE Autopilot** -- no addon needed


---

## Comparison

| Aspect | Backend Bucket CDN | HMAC CDN | OIDC Proxy Pod |
|--------|-------------------|----------|----------------|
| **Org policy change** | Required | Not needed | Not needed |
| **GCS auth** | Bucket must be public or use signed URLs | Private bucket, HMAC signature per request | Private bucket, Workload Identity (keyless) |
| **Bridge to GCS** | Backend Bucket (native) | Internet NEG + HMAC keys | nginx pod + GCS Fuse mount |
| **GCP resources** | ~12 Terraform | ~14 Terraform | ~4 Terraform + ~6 auto-created by Ingress |
| **Compute required** | None | None | Pod (nginx + GCS Fuse sidecar) |
| **Secret management** | None | HMAC key in Terraform state | None |
| **Cost per region** | ~$18/mo | ~$18/mo | ~$18/mo (Ingress LB) |
| **Edge caching** | Yes (if bucket is public) | Yes (global) | No |
| **Latency (global)** | Sub-ms (CDN edge) | Sub-ms (CDN edge) | Pod-level (~ms, single region) |
| **Scaling** | 1 per region | 1 per region | 1 per region |
| **Cluster dependency** | None | None | Region GKE cluster |
| **Key rotation** | None | Optional (HMAC key has read-only access to one bucket containing only public keys) | None |

## Recommendation

**Option 2 (HMAC CDN)** and **Option 3 (OIDC Proxy Pod)** are both viable:

- **Option 2** is pure infrastructure (Terraform only), no pod to manage, cluster-independent,
  and provides edge caching. HMAC key risk is low (read-only access to public docs).
- **Option 3** is keyless (Workload Identity), no secrets to manage, but requires a running
  pod on the region cluster and is tied to the cluster lifecycle.

Both have the same cost (~$18/mo) and scale at 1 per region. OIDC documents are tiny
and fetched infrequently, so edge caching provides negligible benefit for this use case.

**Option 1 (Backend Bucket)** requires making the bucket public via org policy change,
at which point the CDN layer is optional.

---

## Option 4: Cloud Run Proxy (to consider)

Uses a Cloud Run service that reads OIDC documents from the private GCS bucket via its
attached service account and serves them over HTTPS. Cloud Run provides a managed HTTPS
endpoint with a custom domain, eliminating the need for a separate load balancer.

```
GCP STS / Any Client
        │
        ▼  HTTPS
Cloud Run (custom domain + Google-managed cert, CNAME DNS)
        │
        ▼  Attached SA (objectViewer)
Private GCS Bucket
  ├── {infraID}/.well-known/openid-configuration
  └── {infraID}/openid/v1/jwks
```

**The Cloud Run service** would be a minimal Go app (~30 lines) that proxies GCS
objects. It receives a request path like `/{infraID}/.well-known/openid-configuration`,
reads the corresponding object from GCS using the Go SDK, and returns it with the
correct `Content-Type: application/json` header. No GCS Fuse, no nginx, no sidecar --
just a single binary.

**Issues:**
1. **Custom container required** -- unlike Options 2 and 3 (pure infrastructure / stock
   nginx), this requires a custom Go app (~30 lines), a Dockerfile, a build pipeline,
   and an Artifact Registry repo. The image needs periodic patching for Go/SDK CVEs.
   Small overhead, but it's code and a pipeline to own
2. **Cold start latency** -- Cloud Run scales to zero after inactivity. First request
   after idle triggers a cold start (~200-500ms for a small Go binary)

**Advantages:**
- **No org policy change needed** -- project-owned SA
- **No secrets** -- keyless auth via attached SA (Application Default Credentials)
- **Cluster-independent** -- not tied to any GKE cluster lifecycle
- **Serverless** -- no pods, no nodes, no scheduling concerns (but requires a custom container)
- **Scales to zero** -- no cost when idle
- **No LB needed** -- Cloud Run + GCS + CNAME DNS, no static IP or forwarding rule
- If edge caching is ever needed, a CDN layer can be added in front via a
  Serverless NEG + Backend Service
