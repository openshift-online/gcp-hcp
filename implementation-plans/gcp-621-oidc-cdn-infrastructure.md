# GCP-621: OIDC CDN Infrastructure

**Epic**: [GCP-336 — Internal Secrets Management](https://redhat.atlassian.net/browse/GCP-336)
**Story**: [GCP-621 — Public OIDC endpoint for WIF discovery](https://redhat.atlassian.net/browse/GCP-621)
**Related**: [GCP-588 — Signing key strategy for GCP WIF](https://redhat.atlassian.net/browse/GCP-588)
**Last Updated**: 2026-05-18

---

## Problem

GCP STS needs to fetch OIDC discovery documents (`/.well-known/openid-configuration` and `/openid/v1/jwks`) via unauthenticated HTTPS GET requests to validate service account tokens for Workload Identity Federation. The HyperShift operator uploads these documents to a private GCS bucket, but GCP org policy constraints (`constraints/iam.allowedPolicyMemberDomains`) prevent making the bucket publicly readable. We need infrastructure that serves the private bucket's contents at a public HTTPS endpoint.

See [gcp-oidc-discovery-for-wif.md](gcp-oidc-discovery-for-wif.md) for the full OIDC upload design. This plan covers only the public serving infrastructure.

## Proposed Solution: Cloud CDN with Private Origin Authentication

Front the private GCS bucket with a **Global External Application Load Balancer** and **Cloud CDN**, using a private **Backend Service** with **Private Origin Authentication** to authenticate CDN cache-fill requests to GCS.

This was chosen over three alternatives — see [Decision Rationale](#decision-rationale) below.

## Architecture

```
GCP STS / Any Client
        │
        ▼  HTTPS (port 443)
Global External Application Load Balancer
  ├── Static IP (global)
  ├── Google-managed SSL Certificate
  ├── Target HTTPS Proxy
  └── URL Map
        │
        ▼
Cloud CDN (FORCE_CACHE_ALL, 1h TTL, serve-while-stale 24h)
        │
        ▼
Backend Service (EXTERNAL_MANAGED, HTTPS)
  ├── Private Origin Authentication
  └── Custom request header: Host: {bucket}.storage.googleapis.com
        │
        ▼
Internet NEG (FQDN: {bucket}.storage.googleapis.com:443)
        │
        ▼  Signed request
Private GCS Bucket
  ├── {infraID}/.well-known/openid-configuration
  └── {infraID}/openid/v1/jwks
```

### Request Flow

1. GCP STS (or any client) sends `GET https://oidc.{regional_domain}/{infraID}/.well-known/openid-configuration`
2. The Global LB terminates TLS with the Google-managed certificate
3. Cloud CDN checks its edge cache; on a **cache miss**, forwards the request to the Backend Service
4. The Backend Service signs the request using the configured credentials and sets the `Host` header to `{bucket}.storage.googleapis.com`
5. The signed request reaches GCS via the Internet NEG over HTTPS
6. GCS validates the signature against the service account's key, confirms `objectViewer` permission, and returns the object
7. CDN caches the response at edge PoPs and returns it to the client

---

## Infrastructure Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Regional Project                                                │
│                                                                  │
│  ┌──────────────────────────────┐    ┌────────────────────────┐ │
│  │ Cloud CDN + Global LB        │    │ GCS Bucket              │ │
│  │ oidc.{regional_domain}       │───▶│ {project_id}-oidc       │ │
│  │                              │    │ (private, uniform ACL)  │ │
│  │  ┌─────────────────────┐    │    └────────────────────────┘ │
│  │  │ Backend Service      │    │         ▲          ▲          │
│  │  │ + private origin auth │    │         │          │          │
│  │  │ + Internet NEG       │    │    write access     │          │
│  │  └─────────────────────┘    │         │          │          │
│  └──────────────────────────────┘         │          │          │
│                                            │          │          │
│  ┌────────────────────────┐               │          │          │
│  │ SA: oidc-cdn-reader    │───────────────┘          │          │
│  │ + HMAC key             │  objectViewer             │          │
│  └────────────────────────┘                           │          │
│                                                       │          │
│  ┌────────────────────────┐                          │          │
│  │ DNS Zone (tools)       │                          │          │
│  │ oidc.{domain} → LB IP │                          │          │
│  └────────────────────────┘                          │          │
└───────────────────────────────────────────────────────┼──────────┘
                                                        │
                                        uploads OIDC docs │
                                                        │
┌────────────────────────────┐    ┌─────────────────────┴──────┐
│  MC 1                      │    │  MC 2                      │
│  Hypershift Op (GSA)       │    │  Hypershift Op (GSA)       │
│  HostedClusters            │    │  HostedClusters            │
└────────────────────────────┘    └────────────────────────────┘
```

The CDN infrastructure lives in the **regional project**, co-located with the GCS bucket. Multiple MCs in the same region write to the shared bucket; the CDN serves all clusters' OIDC documents from one endpoint.

---

## Design Details

### Issuer URL

```
https://oidc.{regional_domain}/{infraID}/
```

> **Note**: `{infraID}` will be replaced by `{wifConfigID}` once that spec field is available on the HostedCluster API.

Examples:
- Dev: `https://oidc.dev-reg-us-c1-ckandagb3fc.dev.gcp-hcp.devshift.net/hctest42/`
- Integration: `https://oidc.int-reg-us-c1-nkcw.int.gcp-hcp.devshift.net/a1b2/`
- Production: `https://oidc.prd-reg-us-c1-x9k2.prd.gcp-hcp.devshift.net/c3d4/`

The issuer URL is set during `hypershift create iam gcp` — before cluster creation and before placement decides which MC hosts the cluster — so it must be MC-agnostic.

### Caching

OIDC documents are ~1KB each and change only on key rotation. CDN uses `FORCE_CACHE_ALL` with a 1-hour TTL and 24-hour serve-while-stale. This means near-100% cache hit rates and sub-millisecond latency for GCP STS token exchange from any region. Cache-fill requests to GCS are rare.

### HMAC Key

A project-owned service account with an HMAC key — a GCS-native access key + secret pair — authenticates CDN cache-fill requests to GCS. The key grants read-only access to the OIDC bucket only. Risk is low since the bucket contains only public keys; rotation follows standard Terraform lifecycle.

### SSL Certificate

The Google-managed SSL certificate provisions asynchronously (typically 15–60 minutes) after the DNS A record and forwarding rules are created. No blocking or polling is needed — the cert activates automatically once DNS is resolvable.

### Why This Approach

- **No org policy dependency**: Project-owned SA is always in the permitted domain — deployable immediately without org admin approval
- **Pure infrastructure**: Entire setup is Terraform — no pods, containers, or application code to maintain
- **Cluster-independent**: CDN operates independently of any GKE cluster lifecycle
- **Global edge caching**: OIDC documents cached at CDN edge PoPs worldwide

---

## Implementation Tasks

### 1. Terraform CDN module

Add CDN infrastructure to the region module (`terraform/modules/region/`):
- Service account + HMAC key for private origin auth
- Internet NEG pointing to GCS, backend service with CDN and HMAC auth
- Global LB: URL map, HTTPS/HTTP proxies, forwarding rules, static IP
- Google-managed SSL certificate
- DNS A record in the region tools zone
- Feature-gated via `enable_oidc_cdn` variable

### 2. Integration validation

- Enable CDN in integration environment
- Verify OIDC documents are served via CDN endpoint
- End-to-end: create a HostedCluster, confirm WIF token exchange works via CDN issuer URL

---

## Decision Rationale

See [oidc-public-issuer-options.md](../experiments/gcp-588-public-bucket/oidc-public-issuer-options.md) for the full options analysis and test results.

### Alternatives Evaluated

| # | Option | Summary | Verdict |
|---|--------|---------|---------|
| 1 | **Backend Bucket CDN** | Native CDN-to-GCS integration via `google_compute_backend_bucket`. Simpler, fewer resources, no secrets. The `cloud-cdn-fill` SA authenticates cache fills to the private bucket. | **Blocked** — after updating the org policy to allow the `cloud-cdn-fill` SA access to oidc buckets, we confirmed that CDN only authenticates cache fills when clients use **signed URLs**. Unauthenticated requests (which is how GCP STS fetches OIDC documents) result in anonymous cache fills that return 403 from GCS. Since we cannot require STS to use signed URLs, this approach does not work for our use case. |
| 2 | **Private Origin CDN** (chosen) | Backend Service + Internet NEG with private origin auth. Project-owned SA bypasses org policy. | **Viable** — tested end-to-end, deployed to dev. |
| 3 | **OIDC Proxy Pod** | nginx + GCS Fuse on GKE, served via Ingress. No secrets, Workload Identity auth. | **Viable but inferior** — depends on cluster lifecycle, no edge caching, pod availability risk. Currently deployed as interim workaround. |
| 4 | **Cloud Run proxy** | Cloud Run reading from GCS. Serverless, scales to zero. | **Viable but heavier** — requires maintaining a custom container image, build pipeline, and patching for CVEs.|

---

## Cost

| Resource | Cost (per region) |
|----------|-------------------|
| Global forwarding rule | ~$18/month |
| Static IP (in use) | Free |
| Google-managed SSL cert | Free |
| CDN egress + cache fill | Negligible (~1KB docs, high cache hit rate) |
| GCS storage | Negligible |
| **Total** | **~$18/month per region** |

---

## Security

- **Bucket stays private**: `public_access_prevention = "enforced"`, no `allUsers` or `allAuthenticatedUsers` bindings
- **HMAC key scope**: Read-only access to one bucket containing only JWKS public keys — inherently non-sensitive data
- **HMAC key storage**: Encrypted Terraform state in versioned GCS bucket
- **TLS everywhere**: Google-managed certificate on LB, HTTPS to GCS origin
- **No bucket listing**: Uniform bucket-level access; only `objectViewer` granted (not `objectAdmin` or `objectList`)
- **HTTP-to-HTTPS redirect**: All HTTP requests redirected to HTTPS via dedicated forwarding rule

---

## References

- [Cloud CDN Private Origin Authentication](https://cloud.google.com/cdn/docs/private-origin-authentication)
- [GCS HMAC Keys](https://cloud.google.com/storage/docs/authentication/hmackeys)
- [GCS XML API](https://cloud.google.com/storage/docs/xml-api/overview)
- [Experiment docs](../experiments/gcp-588-public-bucket/) — options analysis, design docs, test scripts
- [OIDC discovery implementation plan](gcp-oidc-discovery-for-wif.md) — operator-side OIDC upload design
