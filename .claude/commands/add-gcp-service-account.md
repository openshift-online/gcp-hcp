# Add GCP Service Account for WIF

This command documents the steps required to add a new GCP service account for Workload Identity Federation (WIF) in HyperShift and related projects.

## Overview

When adding a new GCP service account (e.g., for a new controller like Cloud Controller Manager), changes are required across multiple repositories to ensure the service account is:
1. Created by the `hypershift create iam gcp` command
2. Stored in the HostedCluster API
3. Propagated through the CLS stack
4. Used by the component at runtime

## Repositories Affected

1. **hypershift** - Core HyperShift project
2. **gcp-hcp-infra** - HyperShift operator manifests on management clusters
3. **cls-backend** - CLS Backend API
4. **cls-controller** - CLS Controller (HostedCluster template)
5. **gcp-hcp-cli** - gcphcp CLI tool

---

## Step 1: HyperShift API Types

**File:** `api/hypershift/v1beta1/gcp.go`

Add the new field to `GCPServiceAccountsEmails` struct:

```go
type GCPServiceAccountsEmails struct {
    // Existing fields...
    NodePool     string `json:"nodePool,omitempty"`
    ControlPlane string `json:"controlPlane,omitempty"`

    // NEW: Add your service account field
    CloudController string `json:"cloudController,omitempty"`
}
```

**After editing, run:**
```bash
make api
```

### Step 1b: HyperShift CLI Flag

**File:** `cmd/cluster/gcp/create.go`
**Test:** `cmd/cluster/gcp/create_test.go`

Add a CLI flag so users can pass the new service account email when creating a cluster via `hypershift create cluster gcp`. Without this, the `+required` API field cannot be set through the CLI, producing invalid HostedCluster resources.

Follow the pattern of the existing `--cloud-controller-service-account` flag:

1. Add a flag constant:
```go
const flagCloudControllerServiceAccount = "cloud-controller-service-account"
// NEW:
const flag<ServiceAccount>ServiceAccount = "<service-account>-service-account"
```

2. Add the field to `RawCreateOptions` and `ValidatedCreateOptions`:
```go
type RawCreateOptions struct {
    // ...
    <ServiceAccount>ServiceAccount string
}
```

3. Register the flag:
```go
cmd.Flags().StringVar(&opts.<ServiceAccount>ServiceAccount, flag<ServiceAccount>ServiceAccount, "", "GCP service account email for <Component>")
```

4. Add required validation in `ValidateCreateOptions()`:
```go
if len(opts.<ServiceAccount>ServiceAccount) == 0 {
    return nil, fmt.Errorf("--%s is required", flag<ServiceAccount>ServiceAccount)
}
```

5. Map to the HostedCluster spec in `CompleteCreateOptions()`:
```go
hostedCluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsEmails.<ServiceAccount> = opts.<ServiceAccount>ServiceAccount
```

6. Add test cases in `cmd/cluster/gcp/create_test.go`:
   - Test that the flag value is correctly set in the HostedCluster spec
   - Test that a missing flag produces the expected error

### Step 1c: HyperShift Operator Credential Reconciliation

**File:** `hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go`
**Test:** `hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp_test.go`

Add credential secret creation and validation for the new service account. Follow the pattern of `CloudControllerCredsSecret`:

1. Add a secret constructor function:
```go
func <ServiceAccount>CredsSecret(controlPlaneNamespace string) *corev1.Secret {
    return &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Namespace: controlPlaneNamespace,
            Name:      "<service-account>-creds",
        },
    }
}
```

2. Add the entry to the `credentialSecrets` map in `ReconcileCredentials()`:
```go
credentialSecrets := map[string]*corev1.Secret{
    // existing entries...
    hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsEmails.<ServiceAccount>: <ServiceAccount>CredsSecret(controlPlaneNamespace),
}
```

3. Add validation in `validateWorkloadIdentityConfiguration()`:
```go
if wif.ServiceAccountsEmails.<ServiceAccount> == "" {
    return fmt.Errorf("<service account> service account email is required")
}
```

4. Update tests:
   - Add a test constant: `test<ServiceAccount>GSA`
   - Add the field to `validHostedCluster()` helper
   - Add the field to all test fixtures that build `GCPServiceAccountsEmails`
   - Add a test case for missing service account validation

### Step 1d: HyperShift E2E Tests

**Files:**
- `test/e2e/create_cluster_test.go`
- `test/e2e/v2/tests/api_ux_validation_test.go`

1. Add the new field to the `ServiceAccountsEmails` fixture in `test/e2e/create_cluster_test.go`:
```go
ServiceAccountsEmails: hyperv1.GCPServiceAccountsEmails{
    NodePool:        "nodepool@my-project-123.iam.gserviceaccount.com",
    ControlPlane:    "controlplane@my-project-123.iam.gserviceaccount.com",
    CloudController: "cloudcontroller@my-project-123.iam.gserviceaccount.com",
    Storage:         "storage@my-project-123.iam.gserviceaccount.com",
    <ServiceAccount>: "<serviceaccount>@my-project-123.iam.gserviceaccount.com",  // NEW
}
```

2. Add a validation test entry in `test/e2e/v2/tests/api_ux_validation_test.go` to the `DescribeTable` block where other GSA fields are validated:
```go
Entry("it should reject invalid <ServiceAccount> service account email",
    func(spec *hyperv1.GCPPlatformSpec) {
        spec.WorkloadIdentity.ServiceAccountsEmails.<ServiceAccount> = "invalid-<service-account>-email"
    },
    "<serviceAccount> in body"),
```

This follows the pattern used by `CloudController`, `Storage`, `ControlPlane`, and `NodePool`.

---

## Step 2: HyperShift IAM Bindings

**File:** `cmd/infra/gcp/iam-bindings.json`

Add the new service account definition with required IAM roles:

```json
{
  "name": "cloud-controller",
  "displayName": "Cloud Controller Manager Service Account",
  "description": "Service account for GCP Cloud Controller Manager",
  "roles": [
    "roles/compute.loadBalancerAdmin",
    "roles/compute.securityAdmin",
    "roles/compute.viewer"
  ],
  "k8sServiceAccount": {
    "namespace": "kube-system",
    "name": "cloud-controller-manager"
  }
}
```

**Key fields:**
- `name`: Short identifier used in WIF bindings (becomes `{infraId}-{name}`)
- `roles`: GCP IAM roles required by the component
- `k8sServiceAccount`: K8s SA that will impersonate this GCP SA via WIF

**Note:** The `create_iam.go` and `destroy_iam.go` use `//go:embed` to load this file, so no code changes needed there.

---

## Step 3: Regenerate HyperShift Manifests in gcp-hcp-infra

After Steps 1 and 2 are merged to HyperShift main, regenerate `hypershift.yaml`
in the infra repo so the management cluster CRDs reflect the new field.

**File generated:** `kustomize/hypershift/hypershift.yaml`

```bash
cd /path/to/gcp-hcp-infra/kustomize/hypershift

./update.sh
```

This script:
1. Builds the HyperShift CLI from source using `podman`
2. Runs `hypershift install render` with GCP-specific flags
3. Overwrites `hypershift.yaml` with the updated manifests + CRDs

**Then commit and push** the updated `hypershift.yaml`:

```bash
git add kustomize/hypershift/hypershift.yaml
git commit -m "chore: regenerate hypershift.yaml with <field> API addition"
git push
```

> **Do not skip this step.** If `hypershift.yaml` is not updated, the management
> cluster will run with stale CRDs that don't include the new SA field, causing
> HostedCluster validation to reject the new field silently.

---

## Step 4: CLS Backend Model

**File:** `cls-backend/internal/models/cluster.go`

Add the field to `WIFServiceAccountsRef` struct:

```go
type WIFServiceAccountsRef struct {
    NodePoolEmail        string `json:"nodePoolEmail"`
    ControlPlaneEmail    string `json:"controlPlaneEmail"`
    CloudControllerEmail string `json:"cloudControllerEmail"`  // NEW
}
```

---

## Step 5: CLS Controller Template

**File:** `cls-controller/deployments/helm-cls-hypershift-client/templates/controllerconfig.yaml`

Add the field to the HostedCluster template's `serviceAccountsEmails` section:

```yaml
workloadIdentity:
  projectNumber: "{{`{{ .cluster.spec.platform.gcp.workloadIdentity.projectNumber }}`}}"
  poolID: {{`{{ .cluster.spec.platform.gcp.workloadIdentity.poolID }}`}}
  providerID: {{`{{ .cluster.spec.platform.gcp.workloadIdentity.providerID }}`}}
  serviceAccountsEmails:
    nodePool: {{`{{ .cluster.spec.platform.gcp.workloadIdentity.serviceAccountsRef.nodePoolEmail }}`}}
    controlPlane: {{`{{ .cluster.spec.platform.gcp.workloadIdentity.serviceAccountsRef.controlPlaneEmail }}`}}
    cloudController: {{`{{ .cluster.spec.platform.gcp.workloadIdentity.serviceAccountsRef.cloudControllerEmail }}`}}  # NEW
```

---

## Step 6: gcphcp CLI

**File:** `gcp-hcp-cli/src/gcphcp/utils/hypershift.py`

### 6a. Add to SERVICE_ACCOUNTS constant:

```python
SERVICE_ACCOUNTS = {
    "ctrlplane-op": "Control Plane Operator",
    "nodepool-mgmt": "Node Pool Management",
    "cloud-controller": "Cloud Controller Manager",  # NEW
}
```

### 6b. Add to iam_config_to_wif_spec function:

```python
def iam_config_to_wif_spec(iam_config: Dict[str, Any]) -> Dict[str, Any]:
    # ...
    return {
        "projectNumber": iam_config.get("projectNumber"),
        "poolID": pool.get("poolId"),
        "providerID": pool.get("providerId"),
        "serviceAccountsRef": {
            "controlPlaneEmail": service_accounts.get("ctrlplane-op"),
            "nodePoolEmail": service_accounts.get("nodepool-mgmt"),
            "cloudControllerEmail": service_accounts.get("cloud-controller"),  # NEW
        },
    }
```

---

## Verification Checklist

After making all changes:

- [ ] `make api` runs successfully in hypershift
- [ ] `make verify` passes in hypershift
- [ ] `hypershift create cluster gcp --help` shows the new `--<service-account>-service-account` flag
- [ ] E2E test fixtures in `test/e2e/create_cluster_test.go` include the new field
- [ ] API UX validation test in `test/e2e/v2/tests/api_ux_validation_test.go` includes the new field
- [ ] `kustomize/hypershift/update.sh` runs successfully in gcp-hcp-infra
- [ ] `hypershift.yaml` in gcp-hcp-infra contains the new SA field in the CRD schema
- [ ] `go build ./...` works in cls-backend
- [ ] Helm template renders correctly in cls-controller
- [ ] gcphcp CLI tests pass

## Testing

1. Run `hypershift create iam gcp` - should create the new service account
2. Check GCP console for new service account with correct IAM bindings
3. Run `hypershift create cluster gcp` with the new flag - should set the field in the HostedCluster spec
4. Create a cluster via CLS - should include new SA email in HostedCluster spec
5. Verify the component can authenticate using WIF

---

## Opening Pull Requests

> **Important:** All code changes should be pushed to **your fork** of each repository.
> Then create a Pull Request from your fork to the upstream repository.
>
> Example workflow:
> 1. Fork the upstream repo (if not already done)
> 2. Clone your fork locally
> 3. Add upstream as a remote: `git remote add upstream <upstream-url>`
> 4. Create a branch, make changes, push to your fork
> 5. Open PR from your fork to upstream

### PR 1: HyperShift (openshift/hypershift)

**Upstream:** `https://github.com/openshift/hypershift`
**Your fork:** `https://github.com/<your-username>/hypershift`

```bash
cd /path/to/hypershift

# Create branch
git checkout -b add-<service-account>-sa

# Stage changes
git add api/hypershift/v1beta1/gcp.go
git add cmd/infra/gcp/iam-bindings.json
git add cmd/cluster/gcp/create.go
git add cmd/cluster/gcp/create_test.go
git add hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go
git add hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp_test.go
git add test/e2e/create_cluster_test.go
git add test/e2e/v2/tests/api_ux_validation_test.go
git add client/  # Generated client code
git add vendor/  # Vendored API changes

# Commit
git commit -m "feat(gcp): add <ServiceAccount> service account for <Component>

Add <ServiceAccount> field to GCPServiceAccountsEmails and IAM bindings
for <Component> authentication via Workload Identity Federation.

IAM roles:
- roles/...
- roles/...

🤖 Generated with [Claude Code](https://claude.ai/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push and create PR
git push -u origin add-<service-account>-sa
gh pr create --title "feat(gcp): add <ServiceAccount> service account for <Component>" \
  --body "## Summary
- Add \`<ServiceAccount>\` field to \`GCPServiceAccountsEmails\` struct
- Add IAM bindings for <Component> with required roles
- Add \`--<service-account>-service-account\` CLI flag to \`hypershift create cluster gcp\`
- Add credential secret and WIF validation in hypershift-operator
- Update generated client code and tests

## Test plan
- [ ] \`make verify\` passes
- [ ] \`hypershift create cluster gcp\` accepts the new flag
- [ ] \`hypershift create iam gcp\` creates the new service account
- [ ] New service account has correct IAM role bindings

🤖 Generated with [Claude Code](https://claude.ai/claude-code)"
```

### PR 2: gcp-hcp-infra

```bash
cd /path/to/gcp-hcp-infra/kustomize/hypershift

# Regenerate from main (see Step 3)
./update.sh

cd /path/to/gcp-hcp-infra

# Create branch
git checkout -b add-<service-account>-sa

# Stage changes
git add kustomize/hypershift/hypershift.yaml

# Commit
git commit -m "chore: regenerate hypershift.yaml with <field> API addition"

# Push and create PR
git push -u origin add-<service-account>-sa
gh pr create --title "chore: regenerate hypershift.yaml with <field> API addition" \
  --body "## Summary
Regenerate \`kustomize/hypershift/hypershift.yaml\` to pick up the updated
CRD schema from hypershift PR: <link>.

The new \`<field>\` SA field on \`GCPServiceAccountsEmails\` would be silently
rejected by the management cluster without this update.

## Dependencies
- Requires hypershift PR: <link>"
```

### PR 3: CLS Backend (apahim/cls-backend)

**Upstream:** `https://github.com/apahim/cls-backend`
**Your fork:** `https://github.com/<your-username>/cls-backend`

```bash
cd /path/to/cls-backend

# Create branch
git checkout -b add-<service-account>-sa

# Stage changes
git add internal/models/cluster.go

# Commit
git commit -m "feat: add <ServiceAccount>Email to WIFServiceAccountsRef

Support the new <ServiceAccount> service account for GCP clusters.

🤖 Generated with [Claude Code](https://claude.ai/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push and create PR
git push -u origin add-<service-account>-sa
gh pr create --title "feat: add <ServiceAccount>Email to WIFServiceAccountsRef" \
  --body "## Summary
Add \`<ServiceAccount>Email\` field to \`WIFServiceAccountsRef\` struct to support
the new <Component> service account for GCP HyperShift clusters.

## Dependencies
- Requires hypershift PR: <link>

🤖 Generated with [Claude Code](https://claude.ai/claude-code)"
```

### PR 4: CLS Controller (apahim/cls-controller)

**Upstream:** `https://github.com/apahim/cls-controller`
**Your fork:** `https://github.com/<your-username>/cls-controller`

```bash
cd /path/to/cls-controller

# Create branch
git checkout -b add-<service-account>-sa

# Stage changes
git add deployments/helm-cls-hypershift-client/templates/controllerconfig.yaml

# Commit
git commit -m "feat: add <serviceAccount> to HostedCluster template

Include the new <ServiceAccount> service account in the HostedCluster
workloadIdentity configuration.

🤖 Generated with [Claude Code](https://claude.ai/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push and create PR
git push -u origin add-<service-account>-sa
gh pr create --title "feat: add <serviceAccount> to HostedCluster template" \
  --body "## Summary
Add \`<serviceAccount>\` field to the HostedCluster template's
\`serviceAccountsEmails\` section.

## Dependencies
- Requires hypershift PR: <link>
- Requires cls-backend PR: <link>

🤖 Generated with [Claude Code](https://claude.ai/claude-code)"
```

### PR 5: gcphcp CLI (apahim/gcp-hcp-cli)

**Upstream:** `https://github.com/apahim/gcp-hcp-cli`
**Your fork:** `https://github.com/<your-username>/gcp-hcp-cli`

```bash
cd /path/to/gcp-hcp-cli

# Create branch
git checkout -b add-<service-account>-sa

# Stage changes
git add src/gcphcp/utils/hypershift.py

# Commit
git commit -m "feat: add <service-account> to SERVICE_ACCOUNTS mapping

Support the new <ServiceAccount> service account from hypershift IAM output.

🤖 Generated with [Claude Code](https://claude.ai/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push and create PR
git push -u origin add-<service-account>-sa
gh pr create --title "feat: add <service-account> to SERVICE_ACCOUNTS mapping" \
  --body "## Summary
- Add \`<service-account>\` to \`SERVICE_ACCOUNTS\` constant
- Add \`<serviceAccount>Email\` to \`iam_config_to_wif_spec\` output

## Dependencies
- Requires hypershift PR: <link>

🤖 Generated with [Claude Code](https://claude.ai/claude-code)"
```

---

## PR Merge Order

PRs should be merged in this order to avoid breaking changes:

1. **hypershift** - API and IAM bindings (foundation)
2. **gcp-hcp-infra** - Regenerate `hypershift.yaml` CRDs
3. **gcp-hcp-cli** - CLI support (uses hypershift output)
4. **cls-backend** - Backend model (API contract)
5. **cls-controller** - Template update (uses backend model)

**Note:** Steps 3-5 can be merged in parallel if coordinated, but hypershift and gcp-hcp-infra must be updated first so the management cluster CRDs are in sync.

---

## Example: CloudController Service Account

The CloudController service account was added with these IAM roles:
- `roles/compute.loadBalancerAdmin` - Manage load balancers
- `roles/compute.securityAdmin` - Manage firewall rules
- `roles/compute.viewer` - Read compute resources

Bound to K8s ServiceAccount: `kube-system/cloud-controller-manager`