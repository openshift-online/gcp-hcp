Frontend API Interface: GCP API Gateway

> **Status**: Superseded by [ESPv2 Sidecar as API Frontend](espv2-api-frontend.md) (2026-07-15)

***Scope***: GCP-HCP

**Date**: 2025-09-30

## Decision

We will use GCP API Gateway for the customer-facing API interface, providing enterprise-grade authentication, authorization, rate limiting, and monitoring capabilities.

## Context

The project needs a customer-facing API interface that provides secure, scalable access to OpenShift cluster lifecycle management capabilities.

- **Problem Statement**: How to provide enterprise customers with a secure, scalable API to manage OpenShift clusters programmatically while ensuring proper authentication, billing, monitoring, and compliance.
- **Constraints**: Must integrate with GCP billing, support OAuth 2.0 authentication, handle enterprise-scale traffic, and provide comprehensive monitoring.
- **Assumptions**: Customers expect GCP-native authentication and billing integration, and the service should leverage existing GCP infrastructure capabilities.

## Alternatives Considered

1. **GCP API Gateway**: Fully managed API gateway service with native GCP integration for authentication, billing, monitoring, and scaling.
2. **Custom Frontend Service**: Build a custom Go/Python service with authentication middleware, rate limiting, and monitoring stack.
3. **Third-party API Gateway**: Use solutions like Kong, Ambassador, or Istio Gateway with custom integration work.

## Decision Rationale

* **Justification**: GCP API Gateway maximally leverages GCP facilities, reducing development work for authentication (OAuth 2.0, Cloud IAM), authorization (service accounts), rate limiting (quotas), billing integration (automatic usage metering), and monitoring (Cloud Monitoring). This aligns with the strategy of making GCP HCP as "Google-like as possible."
* **Evidence**: Our implementation achieved production-ready deployment with OAuth 2.0 authentication, automatic SSL/TLS, usage metering, and monitoring within days rather than months of custom development.
* **Comparison**: Custom frontend service would require significant development effort for enterprise features that API Gateway provides out-of-the-box. Third-party solutions would require extensive integration work and reduce GCP-native benefits.

## Consequences

### Positive

* Native GCP integration with OAuth 2.0, Cloud IAM, and automatic billing attribution
* Zero operational overhead for API gateway infrastructure (fully managed)
* Enterprise-grade security with built-in DDoS protection and SSL/TLS management
* Automatic scaling and SLA guarantees from Google
* Faster time-to-market leveraging existing GCP platform capabilities

### Negative

* Vendor lock-in to GCP ecosystem
* Limited customization compared to custom-built solutions
* Dependency on GCP API Gateway feature roadmap
* OpenAPI 2.0 limitations (path parameter handling complexities)

## Cross-Cutting Concerns

### Reliability:

* **Scalability**: Automatic scaling handles traffic spikes without manual intervention; supports 1000+ concurrent requests with <200ms P95 latency
* **Observability**: Native Cloud Monitoring integration provides API metrics (request rate, error rate, latency) with SLA dashboards and alerting
* **Resiliency**: GCP's global infrastructure provides high availability; can deploy across multiple regions for geographic redundancy

### Security:
- OAuth 2.0 and Google Cloud IAM integration for customer authentication
- Service account authentication for secure backend communication
- Built-in DDoS protection and automatic certificate management
- API key validation and quota enforcement

### Performance:
- <200ms P95 response latency for API operations
- Automatic request/response caching capabilities
- Global load balancing and edge caching

### Cost:
- Pay-per-request model eliminates fixed infrastructure costs
- Automatic usage metering for accurate customer billing attribution
- Reduced development and operational costs compared to custom solutions

### Operability:
- Zero infrastructure management required
- Automatic SSL certificate renewal and security updates
- Native integration with existing GCP monitoring and alerting systems
- Configuration via OpenAPI specifications enables infrastructure-as-code