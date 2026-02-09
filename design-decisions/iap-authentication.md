# Identity-Aware Proxy (IAP) for Internal Tooling Authentication

***Scope***: GCP-HCP

**Date**: 2026-01-09

**Related**: [GCP-138](https://issues.redhat.com/browse/GCP-138)

## Decision

We will use Google Cloud Identity-Aware Proxy (IAP) with external OAuth brands to provide centralized authentication and authorization for internal tooling UIs (Grafana, Atlantis, ArgoCD, and future applications). IAP is integrated via Gateway API with GCPBackendPolicy resources, using shared certificate maps for TLS management, project-level IAM policies for access control, and JWT-based authentication with issuer validation.

**OAuth Configuration**: OAuth brands and clients are created once per environment in a global project, then credentials are distributed to downstream region/management projects. This eliminates the need for manual OAuth setup in each downstream project.

## Context

### Problem Statement

Our internal tooling (Grafana, Atlantis, ArgoCD) requires secure access control that:
- Authenticates users via their @redhat.com Google accounts
- Authorizes access based on organizational identity
- Provides SSO experience without managing separate credentials
- Centralizes authentication policy across multiple applications
- Integrates with GKE ingress infrastructure

### Constraints

- **API Deprecation**: Google deprecated the IAP OAuth Admin API (March 19, 2026 shutdown), eliminating programmatic OAuth client management. See the [Google Cloud deprecation notice](https://cloud.google.com/docs/deprecations) for details.
- **External Brand Requirement**: Must support @redhat.com users (not just GCP organization members)
- **GKE Autopilot**: Must work with GKE Autopilot clusters and Gateway API infrastructure
- **No Custom Authentication System**: Cannot deploy/maintain custom identity providers
- **Cost Sensitivity**: Must minimize additional infrastructure costs
- **GitOps-First**: Prefer infrastructure-as-code for reproducibility

### Assumptions

- Users have @redhat.com Google Workspace accounts
- Applications are exposed via Gateway API (GKE Gateway controller)
- IAP will be available and supported by Google for the foreseeable future
- Backend services are managed by Gateway API and remain stable

## Alternatives Considered

### 1. **Cloud Armor + IP Allowlisting**
Restrict access based on source IP addresses using Cloud Armor policies.

**Pros**:
- Simple configuration
- No authentication complexity
- Already used for some services (Tekton)
- No additional infrastructure

**Cons**:
- Requires users to proxy through Red Hat squid proxies to get allowlisted IPs
- Complicated proxy configuration for users (VPN + proxy setup)
- Not identity-based (anyone on corporate network can access)
- Doesn't support remote work scenarios well
- No user-level audit trails (cannot differentiate between users)
- Fails security principle of least privilege
- Poor user experience with proxy requirements

### 2. **Red Hat SSO (sso.redhat.com)**
Use Red Hat's centralized SSO infrastructure for authentication.

**Pros**:
- Centralized authentication across Red Hat systems
- Existing infrastructure already managed
- Integrated with Red Hat identity management
- Familiar to Red Hat users

**Cons**:
- Requires coordination with external teams to configure
- Approval and setup process adds delays to deployment
- Doesn't help lock down applications without built-in authentication (e.g., Tekton Dashboard)
- Cannot act as man-in-the-middle proxy for apps without auth support
- Limited to Red Hat SSO capabilities and roadmap
- External dependency on other team's availability

### 3. **OAuth2 Proxy**
Self-hosted reverse proxy that implements OAuth2 authentication flow.

**Pros**:
- Full control over authentication logic
- Works with any OAuth2 provider (including Google Workspace)
- Can run in Kubernetes
- Can protect applications without built-in auth

**Cons**:
- Additional infrastructure to maintain (deployment, updates, monitoring)
- Requires Redis for session storage (more components)
- Manual certificate management
- Need to implement JWT validation ourselves
- Higher operational burden
- More moving parts to troubleshoot

## Decision Rationale

### Justification

Google Cloud IAP provides the optimal balance of security, operational simplicity, and integration with our GKE infrastructure. It offloads authentication to Google's OAuth2 infrastructure while providing transparent integration with Gateway API resources.

**Key advantages**:
1. **Zero infrastructure overhead**: No additional deployments to maintain
2. **Google-managed security**: Google handles JWT signature verification, token validation, and OAuth flows
3. **Gateway API integration**: Modern declarative routing with HTTPRoutes and GCPBackendPolicy
4. **Simplified access control**: Project-level IAM policies via Config Connector enable GitOps
5. **Certificate automation**: Google-managed certificates auto-renew; shared certificate maps centralize per-app certificate tracking
6. **Identity-based**: Each access is tied to a specific @redhat.com user
7. **Audit trail**: All authentication attempts logged to Cloud Logging
8. **SSO experience**: Users authenticate once via Google Workspace

### Evidence

From implementation experience with Grafana:
- **Setup time**: IAP configured in <1 hour vs. days for self-hosted solutions
- **Maintenance burden**: Minimal ongoing maintenance (periodic OAuth secret rotation) vs. regular updates/patches for OAuth2 Proxy
- **Security posture**: JWT signature validation against Google's public keys, no credential storage
- **User experience**: Seamless SSO, no password management
- **Integration**: GCPBackendPolicy directly targets Services, HTTPRoute handles routing
- **GitOps**: Config Connector IAMPolicyMember resources managed via ArgoCD

### Comparison

**vs Cloud Armor + IP Allowlisting**: IAP eliminates the need for users to configure squid proxies and provides identity-based access control with user-level audit trails. Better user experience and supports remote work scenarios without proxy configuration.

**vs Red Hat SSO**: IAP provides immediate deployment without external team dependencies. Can protect applications without built-in authentication (like Tekton Dashboard) by acting as authentication proxy. Faster time-to-deployment and full control over configuration.

**vs OAuth2 Proxy**: IAP eliminates operational overhead (no pods to run, no Redis to maintain, no manual certificate rotation needed). Google manages the OAuth flow, JWT validation, and certificate renewal, reducing maintenance burden and complexity.

## Implementation Architecture

### OAuth Brand Strategy

**Brand Type**: External (`orgInternalOnly: false`)
- Required to authenticate @redhat.com users (who are not part of the GCP organization)
- Created once per environment in the global project
- Shared across all IAP-protected applications in all projects within the environment
- Requires manual Google verification of OAuth consent screen (one-time setup per environment)

**Multi-Project Architecture**:
- OAuth brand is created in the global project for each environment
- Downstream region/management projects reference the same brand via shared OAuth client credentials
- Eliminates the need to create and verify brands in each downstream project

**Note**: While external brands can technically authenticate any Google account, access is restricted to @redhat.com users via IAM policies (see Access Control section)

### OAuth Client Creation

**Approach**: Single shared OAuth client per environment

**Key Benefits**:
- **Environment-level setup**: One OAuth client created per environment in the global project
- **Cross-project reuse**: Same OAuth client used by all downstream region/management projects
- **Minimal manual overhead**: Manual setup required only once per environment, not per project
- **Simplified management**: No per-application or per-project OAuth client tracking
- **Automated distribution**: Credentials distributed to downstream projects via Secret Manager

**Why Manual**:
- Google deprecated the IAP OAuth Admin API (March 2026)
- External brands cannot use Terraform's `google_iap_client` resource
- Manual creation via Cloud Console is Google's recommended approach

**Process**:

**Global Project (One-Time per Environment)**:
1. Create OAuth client via Cloud Console in global project (Web application type)
2. Configure redirect URI: `https://iap.googleapis.com/v1/oauth/clientIds/CLIENT_ID:handleRedirect`
3. Store credentials in global project's Secret Manager

**Downstream Projects (Automated)**:
1. Credentials are populated to each downstream project's Secret Manager
2. External Secrets Operator syncs to Kubernetes namespaces as needed
3. No manual OAuth configuration required in downstream projects

**OAuth Client Secret Rotation**:

OAuth client secrets require periodic rotation for security best practices:

- **Rotation Cadence**: A regular rotation schedule will be defined based on security risk assessment and adhered to (e.g., quarterly, semi-annually, or annually)
- **Rotation Process**:
  1. Generate new OAuth client secret in global project's Cloud Console
  2. Update secret in global project's Secret Manager
  3. Distribute updated secret to all downstream projects' Secret Managers
  4. External Secrets Operator automatically syncs new credentials to Kubernetes
  5. Applications pick up new credentials on pod restart or secret refresh
  6. Verify authentication works with new credentials
  7. Remove old OAuth client secret
- **Impact**: Brief service interruption may occur during credential transition
- **Automation Opportunity**: While initial creation is manual, rotation distribution can be scripted to update all downstream Secret Managers
- **Multi-Environment Coordination**: Each environment (dev, stage, prod) rotates independently on its own schedule

### Certificate Management

**Shared Certificate Map Architecture**:
- Single certificate map managed in `gateway-system` namespace
- Per-application certificate map entries reference individual certificates
- Centralized TLS management reduces operational overhead

**Benefits**:
- **Centralized management**: All TLS certificates tracked in one map
- **GitOps-friendly**: Config Connector manages certificates declaratively
- **Automation-ready**: Certificate map entries created alongside applications
- **Scalable**: Adding new applications requires only creating new map entries

**Implementation**:
- Certificate map created via Config Connector in shared Gateway chart
- Each application creates its own certificate map entry pointing to its certificate
- Gateway references the certificate map for TLS termination

### Gateway API Architecture

**Shared Gateway Pattern**:
- Single Gateway resource in `gateway-system` namespace
- Serves all IAP-protected applications
- Deployed with ArgoCD sync-wave `-5` (before applications)

**Per-Application HTTPRoutes**:
- Each application creates its own HTTPRoute
- HTTPRoute references the shared Gateway
- Routes traffic from hostname to Service
- Deployed in application namespace

**Benefits vs Traditional Ingress**:
- **Cleaner separation**: Gateway infrastructure separate from application routing
- **Modern Kubernetes pattern**: Gateway API is the successor to Ingress
- **Declarative**: HTTPRoutes provide explicit routing configuration
- **Role-based**: Gateway admins vs. application developers

### IAP Configuration

**GCPBackendPolicy Approach**:
- Each application creates a GCPBackendPolicy resource
- Policy targets the Service directly (no BackendConfig annotations)
- Enables IAP and configures OAuth client

**Benefits**:
- **Gateway API native**: No BackendConfig annotations required
- **Declarative**: Config Connector manages GCPBackendPolicy as CRD
- **GitOps-friendly**: Policy defined alongside application manifests

### JWT Authentication

**Validation Strategy**: Issuer-only validation

Applications validate JWT tokens from IAP using:
- Header: `X-Goog-IAP-JWT-Assertion`
- Issuer claim: `https://cloud.google.com/iap`
- JWKS URL: `https://www.gstatic.com/iap/verify/public_key-jwk`

**Security Layers**:
1. JWT signature validation using Google's public keys
2. Issuer claim verification
3. Token expiration validation
4. IAM policy enforcement (who can access)

### Access Control

**Project-Level IAM Approach**:
- IAM policies granted at GCP project level (not per-backend-service)
- Single Google Group for all IAP-protected applications
- Managed via Config Connector IAMPolicyMember resources

**Benefits**:
- **Simplified management**: No backend service name tracking required
- **GitOps-enabled**: Config Connector resources managed via ArgoCD
- **Scalable**: New applications automatically inherit access policies
- **Centralized**: Single source of truth for IAP access

**Implementation**:
```yaml
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: iap-web-accessor
spec:
  member: group:hcm-gcp-hcp@redhat.com
  role: roles/iap.httpsResourceAccessor
  resourceRef:
    kind: Project
    external: projects/PROJECT_ID
```

**Trade-offs**:
- **No per-app granularity by default**: All group members can access all IAP apps
- **Simpler operations**: Eliminates backend service discovery complexity
- **Group-based control**: Access managed through Google Group membership

### Deployment Pattern

**ArgoCD Sync Wave Architecture**:

**Wave -5: Shared Gateway Infrastructure**
- Gateway resource in `gateway-system` namespace
- Certificate map (shared across all applications)
- Project-level IAM policy (Config Connector)

**Wave 0: Per-Application Resources**
- Application namespace and Service
- HTTPRoute (references shared Gateway)
- Certificate map entry (for application hostname)
- GCPBackendPolicy (enables IAP for Service)
- ExternalSecret (syncs OAuth credentials from project's Secret Manager)

**One-Time Setup (Manual - Global Project Only)**:
- Create OAuth brand (one-time per environment)
- Complete Google OAuth consent screen verification
- Create shared OAuth client (one-time per environment)
- Distribute OAuth credentials to downstream projects' Secret Managers
- Add Google Group members for access control

**Downstream Project Setup (Fully Automated)**:
- OAuth credentials already populated in Secret Manager
- All resources deployed via GitOps/ArgoCD
- No manual OAuth configuration required
- Zero manual intervention needed for new region/management projects

**Benefits**:
- **Ordered deployment**: Gateway ready before applications
- **Declarative**: All resources managed via GitOps
- **Reusable**: Pattern easily replicated for new applications
- **Minimal manual overhead**: OAuth setup only needed once per environment, not per project
- **Scalability**: New downstream projects require zero manual OAuth configuration

## Consequences

### Positive

* **Minimal Operational Overhead**: No authentication infrastructure to deploy, patch, or maintain; only periodic OAuth client secret rotation required
* **Minimal Manual Overhead**: OAuth setup required only once per environment, not per project
* **Zero-Touch Downstream Projects**: New region/management projects require no manual OAuth configuration
* **Environment-Level OAuth Management**: Single OAuth brand and client per environment shared across all projects
* **Automated Credential Distribution**: OAuth credentials populated to downstream projects' Secret Managers
* **Gateway API Integration**: Modern declarative routing with HTTPRoutes and GCPBackendPolicy
* **Certificate Management Automation**: Each app still requires its own TLS certificate and certificate map entry, but management is automated via Config Connector with shared certificate map infrastructure
* **Project-Level IAM**: Eliminates backend service tracking complexity
* **GitOps-Enabled**: Config Connector manages IAM policies declaratively via ArgoCD
* **Centralized Authentication**: Consistent OAuth configuration and JWT validation across all projects
* **Strong Security**: Google-managed JWT validation, no credential storage, identity-based access
* **Audit Trail**: All authentication attempts logged to Cloud Logging with user identity
* **User Experience**: Seamless SSO via @redhat.com accounts, no separate passwords
* **Cost Effective**: No additional infrastructure costs, IAP is free for GCP resources
* **Highly Scalable**: Pattern easily replicated for new applications and new projects without manual intervention

### Negative

* **Manual OAuth Setup for Global Projects**: OAuth brand verification and client creation are manual tasks in global projects (Google API deprecation), though only required once per environment
* **OAuth Client Secret Rotation**: OAuth client secrets require periodic rotation on a defined cadence (~1 hour per environment per rotation cycle), requiring manual secret generation and distribution to all downstream projects
* **Credential Distribution**: OAuth credentials must be manually populated to each downstream project's Secret Manager (one-time setup per project, plus updates during rotation)
* **No Per-Application Access Control by Default**: Project-level IAM grants access to all IAP apps, not individual applications
* **Limited Group Integration**: IAP JWT doesn't include Google Groups membership by default (requires Google Workspace admin config or separate automation)
* **API Deprecation Risk**: Google already deprecated IAP OAuth Admin API; future changes possible
* **No Offline Access**: Requires connectivity to Google's OAuth and JWT validation services
* **Application-Level Roles**: Applications must implement their own role management (e.g., Grafana Viewer/Editor/Admin)

## Cross-Cutting Concerns

### Security

* **Authentication**: Google OAuth2 flow with @redhat.com Workspace accounts
* **Authorization**: IAM policies at domain/user/group level, application-specific RBAC for roles
* **JWT Validation**: Signature verification via Google's public keys (JWKS), issuer claim verification, token expiration
* **Threat Mitigation**:
  - Prevents credential theft (no passwords stored)
  - Prevents token spoofing (JWT signature validation)
  - Prevents unauthorized access (IAM policy enforcement)
  - Audit trail for all access attempts
* **Compliance**: Identity-based access control supports least privilege and audit requirements
* **Secret Management**: OAuth credentials stored in Secret Manager, synced via External Secrets Operator (no secrets in Git)

### Performance

* **Latency**: IAP adds ~10-50ms per request for JWT validation (negligible for UI applications)
* **Throughput**: Google-managed infrastructure scales transparently
* **Caching**: JWT tokens cached by applications, validated once per session
* **Resource Utilization**: Zero additional compute resources (no proxies to run)

### Cost

* **IAP for GCP Resources**: Free (applications on GKE qualify as GCP resources)
* **Certificates**: Google-managed certificates are free; each application requires its own certificate resource
* **Operational Cost Savings**: Eliminates infrastructure for OAuth2 Proxy, Redis, and manual certificate management
* **Minimal Overhead**: Cloud Logging and Secret Manager usage is negligible (<$1/month total)

### Operability

* **Deployment Complexity**:
  - One-time: OAuth brand creation and verification, shared OAuth client creation
  - Per App: Deploy via ArgoCD (Gateway, HTTPRoute, GCPBackendPolicy, certificates)
  - Ongoing: Periodic OAuth client secret rotation per defined cadence
* **Monitoring**:
  - IAP authentication failures → Cloud Logging
  - JWT validation errors → Application logs
  - Gateway health → GKE monitoring
* **Troubleshooting**:
  - Runbooks available in gcp-hcp-infra repository
  - Common issues: OAuth client misconfiguration, IAM policy errors
  - Debugging: Cloud Logging for IAP events, application logs for JWT issues
* **Tooling Requirements**:
  - ArgoCD for GitOps deployment
  - Config Connector for IAM and certificates
  - External Secrets Operator for secret syncing

### Reliability

* **Scalability**: Google-managed infrastructure scales transparently, no capacity planning needed
* **Observability**:
  - Cloud Logging: All IAP authentication events
  - Application logs: JWT validation results
  - Metrics: IAP latency, success/failure rates via Cloud Monitoring
* **Resiliency**:
  - **SLA**: Google Cloud IAP is part of GCP's 99.95% SLA
  - **Failover**: Multi-region Google OAuth infrastructure
  - **Degradation**: Applications can fall back to admin password if IAP fails
  - **Recovery**: IAP issues resolved by Google, no customer action needed

## Deployment Approach

### Global Project Setup (One-Time per Environment)

Performed once for each environment in the global project:

1. **OAuth Brand Configuration**:
   - Create external OAuth brand in global project
   - Complete Google OAuth consent screen verification

2. **Shared OAuth Client**:
   - Create single OAuth client for the environment
   - Store credentials in global project's Secret Manager

3. **Credential Distribution**:
   - Populate OAuth credentials to each downstream project's Secret Manager
   - Credentials are identical across all projects in the environment

### Downstream Project Setup (Fully Automated)

All steps performed via GitOps/ArgoCD with zero manual OAuth configuration:

1. **Shared Gateway Infrastructure** (Wave -5):
   - Deploy Gateway in `gateway-system` namespace
   - Create certificate map
   - Configure project-level IAM policy (Config Connector)

2. **Per-Application Deployment** (Wave 0):
   - HTTPRoute targeting shared Gateway
   - Certificate map entry for application hostname
   - GCPBackendPolicy enabling IAP
   - ExternalSecret syncing OAuth credentials from project's Secret Manager

3. **Application Configuration**:
   - Configure JWT validation (issuer, JWKS URL)
   - Set application-specific roles/permissions

### Example Applications

- **Grafana**: Monitoring dashboards with IAP authentication
- **Atlantis**: Terraform automation UI with IAP protection
- **ArgoCD**: GitOps management console with IAP authentication
- **Future Tools**: Pattern reusable for any HTTP/HTTPS application on GKE

## Future Considerations

### Google Groups Integration
IAP JWT doesn't include Google Groups membership by default. Options for role management:
1. **Custom Claims**: Configure Google Workspace to add groups to JWT (requires admin access)
2. **Application API Automation**: Periodic sync from Google Groups API to application roles
3. **Manual Team Management**: Use application's built-in team/role features

Recommendation: Start with manual team management, implement automation if needed.

### Pattern Reusability
The IAP pattern is reusable across any HTTP/HTTPS application on GKE and across multiple projects:
- **Environment-level setup**: OAuth brand and client created once per environment in global project
- **Cross-project reuse**: Same OAuth credentials distributed to all downstream projects
- **Shared Gateway infrastructure**: Common certificate map and Gateway resources
- **Consistent configuration**: Standard HTTPRoute and GCPBackendPolicy approach
- **Standard JWT validation**: Uniform authentication across all applications and projects
- **Scalable deployment**: New projects and applications can be added with zero manual OAuth overhead

### OAuth Client Secret Rotation Automation
While OAuth client secret rotation is currently a manual process, future automation opportunities include:
- **Scripted Distribution**: Automate copying rotated secrets from global project to all downstream projects' Secret Managers
- **Rotation Tooling**: Build tooling to orchestrate the full rotation process (generate, distribute, verify, cleanup)
- **Monitoring**: Track rotation cadence and alert when rotation is due
- **Zero-Downtime Rotation**: Implement dual-secret support (old + new credentials active simultaneously) to eliminate service interruption during rotation

Recommendation: Start with manual rotation on a defined cadence, automate distribution scripting once the process is well-understood.

### API Evolution
Google deprecated the IAP OAuth Admin API. Mitigation:
- Manual OAuth setup is Google's recommended approach
- External brands continue to be supported
- Monitor Google Cloud release notes for IAP changes

## Related Documentation

**In gcp-hcp-infra repository**:
- **Gateway API Implementation**: `argocd/config/global/shared-gateway/template.yaml`
- **Grafana IAP Setup**: `helm/charts/grafana/templates/gcpbackendpolicy.yaml`
- **Certificate Management**: `helm/charts/grafana/templates/certificate-map-entry.yaml`
- **IAM Configuration**: `helm/charts/shared-gateway/templates/iap-iam.yaml`
- **Operational Runbook**: `docs/GRAFANA_IAP_RUNBOOK.md`

## Success Metrics

- **User Experience**: <5 seconds from URL access to authenticated UI
- **Global Project Setup**: <2 hours per environment for complete OAuth configuration (one-time)
- **Downstream Project Setup**: <30 minutes per project for IAP deployment (fully automated via GitOps)
- **Operational Overhead**: Zero manual OAuth configuration for downstream projects; periodic secret rotation per defined cadence
- **Rotation Time**: <1 hour per environment for OAuth client secret rotation when performed
- **Security**: 100% of access attempts logged with user identity
- **Cost**: <$1/month per application in IAP-related costs
- **Reliability**: 99.9% authentication success rate (matches GCP SLA)

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
