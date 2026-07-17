# API Frontend: ESPv2 Sidecar for Cloud Endpoints

***Scope***: GCP-HCP

**Date**: 2026-07-15

**Supersedes**: [GCP API Gateway Frontend](gcp-api-gateway-frontend.md)

## Decision

We will replace GCP API Gateway with Extensible Service Proxy V2 (ESPv2) deployed as a sidecar container alongside the Platform API, integrated with Google Cloud Service Infrastructure for authentication, billing, and monitoring.

## Context

The project initially adopted GCP API Gateway as the customer-facing API frontend. During production readiness work, two blocking limitations were discovered.

- **Problem Statement**: GCP API Gateway supports only 8 regions (6 US, 2 EU, no APAC) — insufficient for all target deployment regions. Additionally, API Gateway's fully-managed model does not expose direct access to Service Control Check/Report APIs, which is required for Google Cloud Marketplace usage metering and billing attribution.
- **Constraints**: Must support all target deployment regions. Must integrate with Google Cloud Marketplace for usage-based billing and metering via Service Control APIs. Must provide authentication (Google ID tokens, API keys) and quota enforcement.
- **Assumptions**: ESPv2 provides equivalent API management functionality to API Gateway (authentication, quota, monitoring) with direct Service Control integration. Marketplace usage-based billing requires reporting consumption metrics to the Service Control API; ESPv2's native Service Control integration (Check/Report) provides the foundation for this, complemented by application-level metering for product-specific usage metrics.

## Alternatives Considered

1. **GCP API Gateway (current)**: Fully managed API gateway with native GCP integration. Blocked by limited regional availability (8 regions, no APAC) and lack of direct Service Control API access for Marketplace usage metering.
2. **ESPv2 Sidecar with Cloud Endpoints**: Envoy-based proxy running as a sidecar container, integrated with Google Cloud Service Infrastructure for authentication, quota enforcement, and billing metering.
3. **Custom Envoy with filters**: Self-managed Envoy proxy with custom Lua or Wasm filters for authentication, rate limiting, and metering.
4. **Istio Ingress Gateway**: Envoy-based ingress gateway deployed as part of the Istio service mesh. Would require the full Istio control plane (istiod) or a standalone gateway installation, plus custom EnvoyFilter or Wasm plugins to integrate with Service Control APIs for Marketplace metering — no native Service Infrastructure integration exists.

## Decision Rationale

* **Justification**: ESPv2 is the Google-recommended proxy for services requiring Cloud Service Infrastructure integration. It runs as a sidecar alongside the application, providing authentication (Google ID tokens, API keys), quota enforcement, billing metering, and logging — all through Service Infrastructure APIs. This is the same pattern used by other Google Cloud Marketplace-integrated services.
* **Evidence**: ESPv2 is the successor to ESP and is actively maintained by Google. It provides Cloud Endpoints integration with direct Service Control Check/Report access, which is a prerequisite for Marketplace billing attribution and the commercial launch. Unlike API Gateway, ESPv2 has no regional availability restrictions since it runs as a container within the application's own deployment.
* **Comparison**: API Gateway is blocked on two hard requirements. Custom Envoy would require reimplementing the Service Infrastructure integration that ESPv2 provides out-of-the-box. Istio Ingress Gateway would require deploying the Istio control plane and building custom Service Control integration via EnvoyFilter or Wasm — significant operational overhead for a capability that only requires an ingress proxy, not a full service mesh.

## Consequences

### Positive

* Marketplace billing/metering integration via Cloud Service Infrastructure
* No regional availability restrictions — runs wherever the application runs
* Sidecar deployment removes the API gateway as a separate infrastructure component — proxy availability is tied to pod lifecycle rather than an independent service
* OpenAPI 2.0 specification drives both proxy configuration and API documentation
* Native support for Google ID token validation and API key authentication
* Request/response logging integrated with Cloud Logging

### Negative

* Operational ownership shifts from fully managed (API Gateway) to team-managed (sidecar container lifecycle)
* ESPv2 container must be versioned and updated alongside application deployments
* Debugging requires understanding of Envoy internals when issues arise at the proxy layer
* Application-level usage metering required for product-specific Marketplace billing metrics — ESPv2 provides Service Control infrastructure, but product usage metrics must be instrumented separately
* OpenAPI 2.0 specification limitations remain (same as API Gateway)

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: ESPv2 scales with the application pods — no separate scaling configuration. Per-pod sidecar eliminates the gateway as a bottleneck.
* **Observability**: Native integration with Cloud Monitoring and Cloud Logging via Service Infrastructure. Provides request-level metrics (latency, error rate, request count) and structured access logs.
* **Resiliency**: Sidecar pattern means proxy availability is tied to pod availability. No separate infrastructure to fail. Health checks propagate through the pod lifecycle.

### Security:
- Google ID token validation via Cloud Service Infrastructure
- API key validation and quota enforcement
- DDoS protection provided by the external Cloud Load Balancer and Cloud Armor policies — ESPv2 itself does not provide DDoS mitigation
- TLS termination at the load balancer; sidecar communicates with the application over localhost

### Cost:
- Eliminates per-request API Gateway billing; compute, load-balancer, and Service Control tracked-operation costs remain
- Minimal compute overhead for the sidecar container (Envoy is lightweight)
- Service Infrastructure usage metering enables accurate Marketplace billing attribution

### Operability:
- ESPv2 container image managed as part of the application deployment
- Configuration driven by OpenAPI specification deployed to Cloud Endpoints
- Service activation via `gcloud endpoints services deploy`
- Startup probe ensures ESPv2 is ready before accepting traffic
