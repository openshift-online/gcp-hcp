# Cloud Network Config Controller: GCP Workload Identity Federation Support

## Overview
Add GCP Workload Identity Federation (WIF) support to cloud-network-config-controller (CNCC). This covers both standalone OCP (where WIF is optional and auto-detected from credentials) and HyperShift HCP (where WIF is mandatory). No feature gate is required — CNCC auto-detects the credential type from what is available.

## Key Design Decisions

### 1. Auto-Detection (No Feature Gate)
CNCC detects the credential type automatically from what is available. No feature gate is required — WIF only activates if WIF credentials are provided. This eliminates the need for an upstream `openshift/api` PR and simplifies the implementation.

**Standalone OCP:** User creates a secret with `workload_identity_config.json`; if absent, falls back to `service_account.json` (existing behavior).

**HCP (HyperShift):** WIF is mandatory. The credential is mounted as `application_default_credentials.json` via the `GOOGLE_APPLICATION_CREDENTIALS` environment variable. The HyperShift control-plane-operator provisions the credential secret, and CNO mounts it into the CNCC deployment with a token-minter sidecar.

### 2. Credential Priority
Single priority chain (no feature gate split):
1. **WIF config from secret** (`workload_identity_config.json`) — for standalone OCP
2. **Service account JSON from secret** (`service_account.json`) — existing behavior
3. **`GOOGLE_APPLICATION_CREDENTIALS` env var** — for HCP and explicit file configuration
4. **Fail with clear error message** listing all sources attempted

**Rationale:** Follows Azure's pattern of Secret -> Environment Variable fallback for each credential type.

### 3. Use `google.CredentialsFromJSON()` for Authentication
- Handles both service account and WIF credentials automatically
- Simpler than manual external account construction
- Supports universe domain configuration for both credential types

## Implementation Phases

### Phase 1: Core GCP Provider Changes
**File:** `pkg/cloudprovider/gcp.go`

#### 1.1 Add Credential Reading Helper
New function after `initCredentials()`:
```go
// readGCPCredentialsConfig reads GCP credentials from various sources.
// Auto-detects credential type: WIF config -> Service Account -> env var.
func (g *GCP) readGCPCredentialsConfig() ([]byte, error) {
    // Priority 1: WIF config from secret
    wifConfig, err := g.readSecretData("workload_identity_config.json")
    if err == nil {
        klog.Infof("Using GCP Workload Identity Federation from secret")
        return []byte(wifConfig), nil
    }
    klog.Infof("workload_identity_config.json not found in secret: %v, trying service account", err)

    // Priority 2: Service account JSON from secret (always try)
    saConfig, err := g.readSecretData("service_account.json")
    if err == nil {
        klog.Infof("Using GCP service account JSON from secret")
        return []byte(saConfig), nil
    }
    klog.Infof("service_account.json not found in secret: %v", err)

    // Priority 3: GOOGLE_APPLICATION_CREDENTIALS env var (for HCP deployments)
    if credFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credFile != "" {
        klog.Infof("Using GOOGLE_APPLICATION_CREDENTIALS from environment: %s", credFile)
        data, err := os.ReadFile(credFile)
        if err == nil {
            return data, nil
        }
        klog.Warningf("Failed to read GOOGLE_APPLICATION_CREDENTIALS file: %v", err)
    }

    return nil, fmt.Errorf("no valid GCP credentials found (tried: workload_identity_config.json, service_account.json, GOOGLE_APPLICATION_CREDENTIALS)")
}
```

#### 1.2 Rewrite initCredentials()
Replace existing implementation:
```go
func (g *GCP) initCredentials() (err error) {
    // Read credentials from configured sources
    credentialsJSON, err := g.readGCPCredentialsConfig()
    if err != nil {
        return err
    }

    // Parse credentials to check format
    var jsonMap map[string]interface{}
    if err := json.Unmarshal(credentialsJSON, &jsonMap); err != nil {
        return fmt.Errorf("error: cannot decode google credentials, err: %v", err)
    }

    // Ensure universe_domain is set to prevent metadata server calls
    // This is critical because OpenShift blocks metadata server access
    if _, hasUniverseDomain := jsonMap["universe_domain"]; !hasUniverseDomain {
        klog.Infof("universe_domain not found in credentials, setting to default: %s", defaultUniverseDomain)
        jsonMap["universe_domain"] = defaultUniverseDomain
        credentialsJSON, err = json.Marshal(&jsonMap)
        if err != nil {
            return fmt.Errorf("error: cannot encode google credentials with universe domain, err: %v", err)
        }
    }

    // Create credentials from JSON (handles both service account and WIF)
    creds, err := google.CredentialsFromJSON(g.ctx, credentialsJSON, compute.ComputeScope)
    if err != nil {
        return fmt.Errorf("error: cannot create credentials from JSON, err: %v", err)
    }

    // Build client options
    opts := []option.ClientOption{
        option.WithTokenSource(creds.TokenSource),
        option.WithUserAgent(UserAgent),
    }
    if g.cfg.APIOverride != "" {
        opts = append(opts, option.WithEndpoint(g.cfg.APIOverride))
    }

    // Initialize the compute service client
    g.client, err = google.NewService(g.ctx, opts...)
    if err != nil {
        return fmt.Errorf("error: cannot initialize google client, err: %v", err)
    }

    return nil
}
```

#### 1.3 Update Imports
Add missing imports at top of file:
```go
import (
    // ... existing imports ...
    "os"  // NEW - for os.Getenv and os.ReadFile
    "k8s.io/klog/v2"  // NEW - for logging (if not already present)
)
```

### Phase 2: Unit Tests
**File:** `pkg/cloudprovider/gcp_test.go`

Add comprehensive test coverage for auto-detection behavior:

```go
// Test auto-detection: WIF config present -> uses WIF
func TestReadGCPCredentialsConfig_WIF_Secret(t *testing.T) {}

// Test auto-detection: only SA JSON present -> uses SA
func TestReadGCPCredentialsConfig_ServiceAccount_Only(t *testing.T) {}

// Test auto-detection: only GOOGLE_APPLICATION_CREDENTIALS -> uses file
func TestReadGCPCredentialsConfig_EnvVar(t *testing.T) {}

// Test auto-detection: both WIF and SA present -> WIF takes priority
func TestReadGCPCredentialsConfig_Priority_WIFFirst(t *testing.T) {}

// Test auto-detection: nothing present -> clear error
func TestReadGCPCredentialsConfig_NoCredentials(t *testing.T) {}

// Test WIF with invalid JSON
func TestReadGCPCredentialsConfig_WIF_InvalidJSON(t *testing.T) {}

// Test env var with missing file
func TestReadGCPCredentialsConfig_EnvVar_FileNotFound(t *testing.T) {}

// Test universe domain handling
func TestInitCredentials_UniverseDomain_AlreadySet(t *testing.T) {}
func TestInitCredentials_UniverseDomain_Injection(t *testing.T) {}
```

**Testing Strategy:**
- Mock file system operations using test helpers
- Use test fixtures for JSON credential files
- Verify correct error messages for each failure case
- Verify credential priority order via auto-detection

### Phase 3: Documentation
**File:** `README.md`

Add new section after existing GCP credentials section:

```markdown
### GCP Workload Identity Federation

To use Workload Identity Federation instead of service account keys:

1. Create WIF configuration secret:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: cloud-credentials
     namespace: openshift-cloud-network-config-controller
   type: Opaque
   stringData:
     workload_identity_config.json: |
       {
         "type": "external_account",
         "audience": "//iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/providers/PROVIDER_ID",
         "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
         "token_url": "https://sts.googleapis.com/v1/token",
         "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SA_EMAIL:generateAccessToken",
         "credential_source": {
           "file": "/var/run/secrets/openshift/serviceaccount/token"
         }
       }
   ```

2. CNCC auto-detects the WIF credential and uses it. No feature gate or configuration change needed.

3. Configure GCP Workload Identity Pool and bindings (see GCP documentation)

**Migration from Service Account Keys:**
- Add `workload_identity_config.json` to existing secret
- Both credentials can coexist during migration
- WIF takes precedence when both are present
- Remove `service_account.json` after verification

**Troubleshooting:**
- Check logs for "Using GCP Workload Identity Federation" message
- Ensure workload identity pool is properly configured in GCP
- Verify service account has required IAM permissions
```

### Phase 4: HyperShift / HCP Integration

This phase covers all HyperShift-side changes required to provision CNCC WIF credentials in the hosted control plane. In HCP mode, WIF is mandatory and credentials are provisioned by the hypershift-operator and consumed by CNO.

#### 4.1 API Type Change
**File:** `api/hypershift/v1beta1/gcp.go`

Add `CloudNetworkConfigController` field to the existing `GCPServiceAccountsEmails` struct (after `CloudController` at line 288):

```go
type GCPServiceAccountsEmails struct {
    // ... existing fields: NodePool, ControlPlane, CloudController ...

    // cloudNetworkConfigController is the Google Service Account email for the
    // Cloud Network Config Controller that manages egress IP and cloud networking
    // configuration in the hosted cluster.
    // This GSA requires the following IAM roles:
    // - roles/compute.networkAdmin (Compute Network Admin - for managing network configuration)
    // - roles/compute.viewer (Compute Viewer - for reading compute metadata)
    // See cmd/infra/gcp/iam-bindings.json for the authoritative role definitions.
    // Format: service-account-name@project-id.iam.gserviceaccount.com
    //
    // This is a user-provided value referencing a pre-created Google Service Account.
    // Typically obtained from the output of `hypershift infra create gcp` which creates
    // the required service accounts with appropriate IAM roles and WIF bindings.
    //
    // +required
    // +immutable
    // +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]{4,28}[a-z0-9]@[a-z][a-z0-9-]{4,28}[a-z0-9]\.iam\.gserviceaccount\.com$`
    // +kubebuilder:validation:MinLength=38
    // +kubebuilder:validation:MaxLength=100
    // +kubebuilder:validation:XValidation:rule="self == oldSelf",message="CloudNetworkConfigController is immutable"
    CloudNetworkConfigController string `json:"cloudNetworkConfigController,omitempty"`
}
```

Also add a cross-field validation rule to `GCPPlatformSpec`:
```go
// +kubebuilder:validation:XValidation:rule="self.workloadIdentity.serviceAccountsEmails.cloudNetworkConfigController.contains('@') && self.workloadIdentity.serviceAccountsEmails.cloudNetworkConfigController.endsWith('@' + self.project + '.iam.gserviceaccount.com')",message="cloudNetworkConfigController service account must belong to the same project"
```

After making this change, run `make api` to regenerate CRDs.

#### 4.2 Credential Secret
**File:** `hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go`

Add a fourth entry to the `ReconcileCredentials` map (currently at line 343) alongside the existing three:

```go
for email, secret := range map[string]*corev1.Secret{
    hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsEmails.NodePool:                      NodePoolManagementCredsSecret(controlPlaneNamespace),
    hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsEmails.ControlPlane:                  ControlPlaneOperatorCredsSecret(controlPlaneNamespace),
    hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsEmails.CloudController:               CloudControllerCredsSecret(controlPlaneNamespace),
    hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsEmails.CloudNetworkConfigController:  CloudNetworkConfigControllerCredsSecret(controlPlaneNamespace),
} {
```

Add the secret constructor function (following the `CloudControllerCredsSecret` pattern at line 396):

```go
// CloudNetworkConfigControllerCredsSecret returns the secret containing Workload Identity Federation credentials
// for the Cloud Network Config Controller.
func CloudNetworkConfigControllerCredsSecret(controlPlaneNamespace string) *corev1.Secret {
    return &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: controlPlaneNamespace,
            Name:      "cloud-network-config-controller-creds",
        },
    }
}
```

#### 4.3 Validation
**File:** `hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go`

Add to `validateWorkloadIdentityConfiguration()` (after the CloudController check at line 543):

```go
if wif.ServiceAccountsEmails.CloudNetworkConfigController == "" {
    return fmt.Errorf("cloud network config controller service account email is required")
}
```

#### 4.4 CNO Deployment Adapter — GCP CNCC Credential Env Vars
**File:** `control-plane-operator/controllers/hostedcontrolplane/v2/cno/deployment.go`

In the `buildCNOEnvVars()` function, add GCP credential env vars for CNCC following the Azure ARO HCP pattern (currently at lines 149-163). Add after the Azure block:

```go
// For GCP HCP deployments, pass the CNCC WIF credential secret name so CNO
// can mount it into the cloud-network-config-controller deployment.
// This follows the Azure ARO HCP pattern above.
if hcp.Spec.Platform.Type == hyperv1.GCPPlatform && hcp.Spec.Platform.GCP != nil {
    cnoEnv = append(cnoEnv,
        corev1.EnvVar{
            Name:  "GCP_CNCC_CREDENTIALS_SECRET",
            Value: "cloud-network-config-controller-creds",
        },
        corev1.EnvVar{
            Name:  "GCP_CNCC_CREDENTIALS_KEY",
            Value: "application_default_credentials.json",
        },
    )
}
```

**Cross-team interface:** CNO must consume these env vars to:
1. Mount the credential secret into the CNCC deployment
2. Set `GOOGLE_APPLICATION_CREDENTIALS` env var pointing to the mounted file
3. Add a token-minter sidecar container to the CNCC deployment (CNO already has `TOKEN_MINTER_IMAGE` available — see `deployment.go:99`)

#### 4.5 IAM Bindings
**File:** `cmd/infra/gcp/iam-bindings.json`

Add a fourth service account entry for CNCC (after the existing `cloud-controller` entry):

```json
{
  "name": "cncc",
  "displayName": "Cloud Network Config Controller Service Account",
  "description": "Service account for cloud-network-config-controller (egress IP and cloud networking)",
  "roles": [
    "roles/compute.networkAdmin",
    "roles/compute.viewer"
  ],
  "k8sServiceAccount": {
    "namespace": "openshift-cloud-network-config-controller",
    "name": "cloud-network-config-controller"
  }
}
```

#### 4.6 CLI
**File:** `cmd/cluster/gcp/create.go`

Add the `--cloud-network-config-controller-service-account` flag:

```go
const (
    // ... existing flags ...
    flagCloudNetworkConfigControllerServiceAccount = "cloud-network-config-controller-service-account"
)

type RawCreateOptions struct {
    // ... existing fields ...

    // CloudNetworkConfigControllerServiceAccount is the Google Service Account email for CNCC
    CloudNetworkConfigControllerServiceAccount string
}
```

In `BindOptions()`:
```go
flags.StringVar(&opts.CloudNetworkConfigControllerServiceAccount,
    flagCloudNetworkConfigControllerServiceAccount,
    opts.CloudNetworkConfigControllerServiceAccount,
    "Google Service Account email for Cloud Network Config Controller (from `hypershift infra create gcp` output)")
```

In `Validate()`:
```go
if err := util.ValidateRequiredOption(flagCloudNetworkConfigControllerServiceAccount, o.CloudNetworkConfigControllerServiceAccount); err != nil {
    return nil, err
}
```

In `ApplyPlatformSpecifics()` — update the `ServiceAccountsEmails` initialization (currently at line 193) to include `CloudNetworkConfigController`:
```go
ServiceAccountsEmails: hyperv1.GCPServiceAccountsEmails{
    NodePool:                      o.NodePoolServiceAccount,
    ControlPlane:                  o.ControlPlaneServiceAccount,
    CloudNetworkConfigController:  o.CloudNetworkConfigControllerServiceAccount,
},
```

Note: `CloudController` (CCM service account) is not included here yet because the CCM flag is being added in a separate PR (GCP-367). Both will be present when both PRs land.

#### 4.7 Token Minting (CNO-side dependency)
CNO must add a token-minter sidecar to the CNCC deployment when running in HCP mode on GCP. This is a cross-team dependency:

- CNO already has `TOKEN_MINTER_IMAGE` env var available (set in `deployment.go:99`)
- CNO should detect GCP HCP mode via `GCP_CNCC_CREDENTIALS_SECRET` env var presence
- The token-minter sidecar mints projected service account tokens for the CNCC pod
- Token audience should match the WIF provider configuration

This follows the same pattern used by the Azure cloud controller manager component, which uses `InjectTokenMinterContainer()` (see `control-plane-operator/controllers/hostedcontrolplane/v2/cloud_controller_manager/azure/component.go:59-63`).

## Files to Modify

| File | Changes | LOC |
|------|---------|-----|
| `pkg/cloudprovider/gcp.go` | Add WIF auto-detection, rewrite initCredentials() | ~100 |
| `pkg/cloudprovider/gcp_test.go` | Add auto-detection unit tests | ~200 |
| `README.md` | Document WIF setup and migration | ~40 |
| `api/hypershift/v1beta1/gcp.go` | Add `CloudNetworkConfigController` field + validation | ~20 |
| `hypershift-operator/.../gcp/gcp.go` | Credential secret constructor, map entry, validation | ~25 |
| `control-plane-operator/.../cno/deployment.go` | GCP CNCC credential env vars | ~12 |
| `cmd/infra/gcp/iam-bindings.json` | CNCC service account IAM definition | ~10 |
| `cmd/cluster/gcp/create.go` | CNCC service account CLI flag | ~15 |

## Backward Compatibility

- **Auto-detection is fully backward compatible:** Existing `service_account.json` deployments continue to work unchanged. WIF is only used when WIF credentials are provided.
- **No feature gate migration:** Since there is no feature gate, there is nothing to enable or disable.
- **HCP API field is additive:** The new `cloudNetworkConfigController` field in `GCPServiceAccountsEmails` is additive. Existing clusters will need an update to specify this field. Validation will require it on new clusters.

## Testing Checklist

### Unit Tests
- [ ] WIF config present in secret -> auto-detected and used
- [ ] Only SA JSON present in secret -> used (backward compatibility)
- [ ] Only `GOOGLE_APPLICATION_CREDENTIALS` env var -> file read and used
- [ ] Both WIF and SA present -> WIF takes priority
- [ ] Nothing present -> clear error listing all sources
- [ ] Universe domain injection when missing
- [ ] Universe domain preserved when already set
- [ ] Invalid JSON -> clear error

### HyperShift Unit Tests
- [ ] `CloudNetworkConfigControllerCredsSecret()` returns correct secret name
- [ ] `ReconcileCredentials` creates CNCC credential secret
- [ ] `validateWorkloadIdentityConfiguration` rejects empty CNCC service account email
- [ ] CNO deployment env vars set for GCP platform
- [ ] CNO deployment env vars NOT set for non-GCP platforms
- [ ] CLI flag `--cloud-network-config-controller-service-account` required and validated

### Integration Tests
- [ ] Deploy with WIF config, verify egress IP operations
- [ ] Deploy with service account (backward compatibility)
- [ ] Test with both credentials present (WIF priority)
- [ ] Verify no metadata server calls
- [ ] HyperShift: create cluster with CNCC service account, verify credential secret provisioned
- [ ] HyperShift: verify CNCC pod starts with WIF credentials in HCP namespace

### Manual Verification
- [ ] Create WIF config secret in test cluster
- [ ] Verify pod starts and logs show WIF usage
- [ ] Test egress IP assign/release operations
- [ ] Test migration path (add WIF, verify, remove SA key)
- [ ] HyperShift: verify end-to-end CNCC WIF flow

## Risk Mitigation

### High: Metadata Server Dependency
**Risk:** SDK might attempt blocked metadata server access
**Mitigation:**
- Use `google.CredentialsFromJSON()` instead of `FindDefaultCredentials()`
- Explicitly set universe domain
- Monitor logs for timeout errors

### Medium: CNO Coordination
**Risk:** CNO changes for CNCC credential delivery require cross-team work
**Mitigation:**
- Define clear env var interface (`GCP_CNCC_CREDENTIALS_SECRET`, `GCP_CNCC_CREDENTIALS_KEY`)
- Document expected CNO behavior in this plan
- CNO already implements this pattern for Azure ARO HCP (lines 149-163 in `deployment.go`)

### Low: API Field Addition
**Risk:** Existing clusters will not have `cloudNetworkConfigController` field set
**Mitigation:**
- Field is additive; existing clusters need update
- Validation only applies to new clusters (or updates that touch the WIF section)
- OpenAPI immutability validation (`self == oldSelf`) prevents accidental changes

### Low: Invalid WIF Configuration
**Risk:** Malformed JSON breaks authentication
**Mitigation:**
- Validate JSON structure before use
- Clear error messages
- Fallback to service account (in standalone OCP mode)
- Comprehensive unit tests

## Timeline

| Phase | Dependencies |
|-------|--------------|
| 1. Core CNCC implementation | None |
| 2. Unit tests | Phase 1 |
| 3. Documentation | Phases 1-2 |
| 4. HyperShift integration | Can start in parallel with 1-3 |
| 5. Code review & iteration | All phases |
| 6. CNO integration (cross-team) | Phases 1-5 |
| 7. Integration testing | All merged |

**Critical path:** CNO integration for CNCC credential delivery in HCP mode.

## Success Criteria

- [ ] WIF credentials successfully authenticate to GCP API
- [ ] Service account JSON continues to work (backward compatibility)
- [ ] Auto-detection correctly identifies credential type
- [ ] Credential fallback chain works as designed
- [ ] Universe domain prevents metadata server calls
- [ ] All unit tests pass with >80% coverage
- [ ] HyperShift API field accepted by `make api && make verify`
- [ ] HyperShift credential secret provisioned correctly
- [ ] CNO env vars delivered to CNCC deployment
- [ ] Integration tests pass in live cluster
- [ ] Documentation is complete and accurate
- [ ] Zero downtime migration path validated

## Example WIF Configuration

### Standalone OCP: Kubernetes Secret
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-credentials
  namespace: openshift-cloud-network-config-controller
type: Opaque
stringData:
  workload_identity_config.json: |
    {
      "type": "external_account",
      "audience": "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/my-pool/providers/my-provider",
      "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
      "token_url": "https://sts.googleapis.com/v1/token",
      "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/cncc@my-project.iam.gserviceaccount.com:generateAccessToken",
      "credential_source": {
        "file": "/var/run/secrets/openshift/serviceaccount/token"
      }
    }
```

### HCP: Management Cluster Credential Secret (provisioned by HyperShift)
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-network-config-controller-creds
  namespace: clusters-my-cluster  # control plane namespace
type: Opaque
data:
  application_default_credentials.json: <base64-encoded WIF credential JSON>
```

This secret is auto-generated by the hypershift-operator's `ReconcileCredentials` function using the `CloudNetworkConfigController` service account email from the `HostedCluster` spec.

### GCP Setup Commands (Standalone OCP)
```bash
# Create workload identity pool
gcloud iam workload-identity-pools create openshift-pool \
  --location=global \
  --display-name="OpenShift Workload Identity Pool"

# Create provider
gcloud iam workload-identity-pools providers create-oidc openshift-provider \
  --location=global \
  --workload-identity-pool=openshift-pool \
  --issuer-uri="https://kubernetes.default.svc.cluster.local" \
  --allowed-audiences="openshift"

# Bind service account
gcloud iam service-accounts add-iam-policy-binding \
  cncc@PROJECT_ID.iam.gserviceaccount.com \
  --role=roles/iam.workloadIdentityUser \
  --member="principalSet://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/openshift-pool/attribute.namespace/openshift-cloud-network-config-controller"
```

### GCP Setup Commands (HCP — CNCC Service Account)
```bash
# Create CNCC service account
gcloud iam service-accounts create cncc \
  --display-name="Cloud Network Config Controller" \
  --project=PROJECT_ID

# Grant required IAM roles
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:cncc@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/compute.networkAdmin"

gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:cncc@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/compute.viewer"

# Bind WIF (allow Kubernetes SA to impersonate GCP SA)
gcloud iam service-accounts add-iam-policy-binding \
  cncc@PROJECT_ID.iam.gserviceaccount.com \
  --role=roles/iam.workloadIdentityUser \
  --member="principal://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/POOL_ID/subject/system:serviceaccount:openshift-cloud-network-config-controller:cloud-network-config-controller"
```

## Next Steps After Plan Approval

1. Implement core CNCC auto-detection on `gcp-wif` branch (Phases 1-3)
2. Add `CloudNetworkConfigController` API field in HyperShift (Phase 4.1), run `make api && make verify`
3. Implement CNCC credential provisioning in HyperShift (Phases 4.2-4.6)
4. Coordinate with CNO team on CNCC credential delivery (Phase 4.4, 4.7)
5. Set up test environment with WIF configuration
6. Run full integration tests after all components merged
