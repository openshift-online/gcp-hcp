# GCP Persistent Disk CSI Driver for GCP Hosted Control Planes

## Overview

This document describes the implementation approach for enabling the GCP Persistent Disk CSI driver on GCP Hosted Control Planes. The GCP PD CSI driver (`pd.csi.storage.gke.io`) provides persistent block storage for workloads running on GCP HCP clusters.

## Component Flow and Dependencies

Understanding the chain of operators and CRs is critical for this implementation.

### Startup Sequence

#### Management Cluster

1. **HyperShift Operator (HO)**
   - Watches: `HostedCluster` CR
   - Creates: `HostedControlPlane` CR, Control Plane Namespace
   - Creates (in CP namespace): `gcp-pd-cloud-credentials` secret — ❌ MISSING

2. **Control Plane Operator (CPO)**
   - Watches: `HostedControlPlane` CR
   - Deploys (in CP namespace): `cluster-storage-operator`

3. **Hosted Cluster Config Operator (HCCO)**
   - Watches: `HostedControlPlane` CR
   - Creates (in GUEST cluster):
     - `Infrastructure` CR (platform: GCP)
     - `Storage` CR (tells CSO to manage storage)
     - Reconciles `ClusterCSIDriver` CR  — ❌ MISSING

4. **Cluster Storage Operator (CSO)**
   - Watches (in GUEST cluster): `ClusterCSIDriver` CR
   - Deploys (in CP namespace): `gcp-pd-csi-driver-operator` — ❌ MISSING (CSO config)

5. **GCP PD CSI Driver Operator** — ❌ MISSING (HyperShift assets)
   - Watches (in GUEST cluster): `ClusterCSIDriver` CR (for config)
   - Deploys (in CP namespace): `gcp-pd-csi-driver-controller`
   - Deploys (in GUEST cluster): `gcp-pd-csi-driver-node` DaemonSet

6. **GCP PD CSI Driver Controller**
   - Uses: `gcp-pd-cloud-credentials` from CP namespace
   - Watches PVCs, calls GCP API to create/attach disks

#### Guest Cluster

7. **GCP PD CSI Driver Node DaemonSet** (deployed by CSI Driver Operator)
   - Mounts disks to pods on each worker node

8. **User creates PVC** → CSI Driver Controller provisions GCP Persistent Disk

---

### CR Dependency Chain

```
HostedCluster
    │
    ▼
HostedControlPlane
    │
    ├──► Infrastructure CR (platform info)
    │
    ├──► Storage CR (enable CSO)
    │         │
    │         ▼
    │    ClusterCSIDriver CR ❌ MISSING (HyperShift)
    │         │
    │         ▼
    │    StorageClass ❌ MISSING (CSO)
    │
    └──► gcp-pd-cloud-credentials ❌ MISSING (HyperShift)
              │
              ▼
         CSI Driver authenticates with GCP
```

### Implementation Work Areas

Enabling GCP PD block storage in HCP requires changes across **3 repositories**:

| # | Repository | Scope | Description |
|---|------------|-------|-------------|
| 1 | `openshift/csi-operator` | CSI Driver | ⚠️ **PREREQUISITE**: Migrate gcp-pd-csi-driver-operator to csi-operator |
| 2 | `openshift/cluster-storage-operator` | CSO | Add HyperShift starter config + GCP PD HyperShift assets |
| 3 | `openshift/hypershift` | HyperShift | API, IAM bindings, credential secrets, ClusterCSIDriver CR, CPO env vars |


#### 1. CSI-OPERATOR MIGRATION (`openshift/csi-operator`) — ⚠️ PREREQUISITE

- Migrate `gcp-pd-csi-driver-operator` to `csi-operator` monorepo
- Create driver overlay assets (base, patches, generated manifests)
- Implement driver generator and operator runtime configuration
- Add HyperShift patches (token-minter, guest kubeconfig)
- See: [gcp-pd-csi-operator-migration.md](./gcp-pd-csi-operator-migration.md) for full details

#### 2. CLUSTER STORAGE OPERATOR (`openshift/cluster-storage-operator`)

- Add HyperShift mode support to GCP PD driver config function
- Register GCP PD driver in HyperShift starter configuration
- Refactor existing assets into base/standalone/hypershift structure
- Create HyperShift-specific guest and management cluster assets

#### 3. HYPERSHIFT (`openshift/hypershift`)

- **IAM**: Define GCP PD CSI service account with required roles
- **API**: Add Storage service account field for GCP WIF configuration
- **HO**: Create cloud credential secret in control plane namespace
- **HCCO**: Create `ClusterCSIDriver` CR in guest cluster
- **CPO**: Add GCP PD image config and operand health status monitoring

---

## Implementation Details

### 1. CSI-Operator Migration

The GCP PD CSI driver operator must be migrated to `openshift/csi-operator` to gain HyperShift support. The csi-operator monorepo provides built-in patterns for asset generation, token-minter injection, and split deployment (controller in mgmt, node in guest).

> **Full migration plan**: [gcp-pd-csi-operator-migration.md](./gcp-pd-csi-operator-migration.md)

#### Stage 1: Repository Migration (Copy Only)

| Phase | Task | Description |
|-------|------|-------------|
| 1.1 | Git subtree import | Import existing operator into `legacy/` (no code changes) |
| 1.2 | Dockerfile | Create `Dockerfile.gcp-pd` pointing to `legacy/` |
| 1.3 | openshift/release | Update build location, test manifest paths, rehearse jobs |
| 1.4 | ocp-build-data | Update image source, cachito config for `legacy/` vendor |
| 1.5 | Merge coordination | Both PRs must merge together (robot sync dependency) |

#### Stage 2: Code Refactoring (New Structure)

| Phase | Task | Description |
|-------|------|-------------|
| 2.1-2.3 | Asset structure | Create overlay assets for standalone and HyperShift deployments |
| 2.4-2.5 | Driver config | Implement driver generator and runtime configuration |
| 2.6-2.7 | Integration | Register driver in shared generator and port registry |
| 2.8 | Dockerfile | Update `Dockerfile.gcp-pd` to build from refactored code |
| 2.9 | openshift/release | Update path from `legacy/` to `/`, update test manifest paths |
| 2.10 | ocp-build-data | Update `dockerfile` field from `legacy/Dockerfile.gcp-pd` to `Dockerfile.gcp-pd`, update cachito config |
| 2.11 | Merge coordination | Both openshift/release and ocp-build-data PRs must merge together |
| 2.12 | E2E tests | Move existing test manifests from source to `test/e2e/gcp-pd/` |
| 2.13 | CI setup | Set up CI infrastructure and presubmit/periodic test jobs |
| 2.14 | Cleanup | Remove legacy code and update build configuration |

#### Key Deliverables

- **Asset overlays**: `base/`, `patches/`, `generated/standalone/`, `generated/hypershift/`
- **HyperShift patches**: Token-minter sidecar, guest kubeconfig volume injection
- **Generator config**: Defines sidecars, volumes, env vars for controller and node
- **E2E test manifests**: Move existing manifests from source to `test/e2e/gcp-pd/`

---

### 2. Cluster Storage Operator Changes

#### Add HyperShift support to config function (`pkg/operator/csidriveroperator/csioperatorclient/gcp-pd.go`)

Enable CSO to deploy GCP PD operator in HyperShift mode by adding an `isHypershift` parameter that switches between standalone and HyperShift asset paths. When `isHypershift=true`, CSO deploys the operator to the management cluster with guest kubeconfig access.

| Change | Current | Required |
|--------|---------|----------|
| Function signature | `GetGCPPDCSIOperatorConfig()` | `GetGCPPDCSIOperatorConfig(isHypershift bool)` |
| Control plane image env | Not present | Add `GCP_PD_DRIVER_CONTROL_PLANE_IMAGE` |
| Asset branching | Single set of assets | Branch on `isHypershift` for standalone vs hypershift assets |
| Management assets | Not present | Add `MgmtStaticAssets` for HyperShift mode |


```go
func GetGCPPDCSIOperatorConfig(isHypershift bool) CSIOperatorConfig {
    // Add: "${DRIVER_CONTROL_PLANE_IMAGE}" env var
    // Add: if isHypershift { use hypershift/guest + hypershift/mgmt assets }
}
```

#### Register in HyperShift starter (`pkg/operator/operator_starter.go`)

```go
func (hsr *HyperShiftStarter) populateConfigs(...) []csioperatorclient.CSIOperatorConfig {
    return []csioperatorclient.CSIOperatorConfig{
        // ... existing drivers ...
        csioperatorclient.GetGCPPDCSIOperatorConfig(true),  // ADD THIS
    }
}
```

#### Refactor assets into base/standalone/hypershift structure

Create directory structure (`assets/csidriveroperators/gcp-pd/`):

```
gcp-pd/
├── base/                    # Refactor existing assets here
├── standalone/              # Standalone overlay
└── hypershift/
    ├── guest/               # Guest cluster assets
    └── mgmt/                # Management cluster assets
```

#### Create HyperShift-specific guest and management assets

| Location | Assets |
|----------|--------|
| `hypershift/guest/` | ServiceAccount, Roles, RoleBindings, ClusterCSIDriver CR |
| `hypershift/mgmt/` | Deployment (with guest-kubeconfig, affinity, tolerations), ServiceAccount, Roles |

---

### 3. HyperShift Changes

#### API: Add Storage GSA field

```go
// api/hypershift/v1beta1/gcp.go
type GCPServiceAccountsEmails struct {
    NodePool     string `json:"nodePool,omitempty"`
    ControlPlane string `json:"controlPlane,omitempty"`
    Storage      string `json:"storage,omitempty"`  // NEW
}
```

#### IAM: Define service account (`cmd/infra/gcp/iam-bindings.json`)

```json
{
  "name": "gcp-pd-csi",
  "roles": [
    "roles/compute.storageAdmin",
    "roles/compute.instanceAdmin.v1",
    "roles/iam.serviceAccountUser",
    "roles/resourcemanager.tagUser"
  ],
  "k8sServiceAccount": {
    "namespace": "openshift-cluster-csi-drivers",
    "name": "gcp-pd-csi-driver-controller-sa"
  }
}
```

#### HO: Create credential secret in CP namespace

```go
// hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go
if storageGSA := hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsEmails.Storage; storageGSA != "" {
    credentialSecrets[storageGSA] = GCPPDCloudCredentialsSecret(controlPlaneNamespace)
}
```

#### HCCO: Create ClusterCSIDriver CR in guest cluster

Create `ClusterCSIDriver` CR (`reconcileStorage`):

```go
// control-plane-operator/hostedclusterconfigoperator/controllers/resources/resources.go
case hyperv1.GCPPlatform:
    driverNames = []operatorv1.CSIDriverName{operatorv1.GCPPDCSIDriver}
```

#### CPO: Image config and health monitoring

Add GCP PD control plane image mapping:

```go
// control-plane-operator/controllers/hostedcontrolplane/v2/storage/envreplace.go
"GCP_PD_DRIVER_CONTROL_PLANE_IMAGE": "gcp-pd-csi-driver",
```

Add GCP PD to operand health status check:

```go
// control-plane-operator/controllers/hostedcontrolplane/v2/storage/component.go
case hyperv1.GCPPlatform:
    operandsDeploymentsList = []operand{
        {DeploymentName: "gcp-pd-csi-driver-operator", ...},
        {DeploymentName: "gcp-pd-csi-driver-controller", ...},
    }
```

---

## Summary Tables

### Files Changed by Repository

| Repository | Key Files | Changes |
|------------|-----------|---------|
| `openshift/csi-operator` | `pkg/driver/gcp-pd/`, `assets/overlays/gcp-pd/`, `cmd/`, `Dockerfile.gcp-pd` | Full migration with HyperShift support |
| `openshift/cluster-storage-operator` | `operator_starter.go`, `gcp-pd.go`, `csidriveroperators/gcp-pd/hypershift/` | Register driver, add HyperShift config and assets |
| `openshift/hypershift` | `gcp.go`, `iam-bindings.json`, `resources.go`, `component.go`, `envreplace.go` | API, IAM, credentials, health check, image config |
| `openshift/release` | CI job configs | Build configuration for csi-operator |
| `openshift/ocp-build-data` | Image metadata | Release payload configuration |

### Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| csi-operator migration | ❌ Required | GCP PD must be migrated to csi-operator for HyperShift support |
| CSO HyperShift support | ❌ Required | `HyperShiftStarter` doesn't register GCP PD driver |
| HyperShift changes | ❌ Required | API, IAM bindings, credential secrets, image config |
| WIF Infrastructure | ✅ Available | Set up via `hypershift infra create gcp` |

---

## Open Questions

1. **KMS Encryption**: Expose `GCPCSIDriverConfigSpec.kmsKey` through HostedCluster API?
   - *Note*: Neither AWS nor Azure expose CSI driver PV encryption through the HostedCluster API. Cluster admins configure it via StorageClass parameters (`disk-encryption-kms-key`) or the `ClusterCSIDriver` CR (`spec.driverConfig.gcp.kmsKey`). No HostedCluster API changes needed for consistency.
2. **Filestore Support**: Enable GCP Filestore CSI driver for ReadWriteMany workloads?
   - *Note*: The Filestore CSI driver is installed via OLM and runs entirely in the guest cluster. The operator does not need any code changes to support HyperShift. Provide installation documentation covering GCP Workload Identity setup.
3. **Regional PD**: Support regional persistent disks for HA across zones?
   - *Note*: Regional PD is enabled via a StorageClass parameter (`replication-type: regional-pd`). No code changes required. Cluster admins can create this StorageClass if they need cross-zone HA.

---

## Testing Plan

| Type | Scope |
|------|-------|
| Unit Tests | GCP platform fixtures for all components |
| Integration | Credential flow, ClusterCSIDriver CR creation |
| E2E | PVC lifecycle: create, attach, mount, resize, delete |

---

## Related Documents

- **[gcp-pd-csi-operator-migration.md](./gcp-pd-csi-operator-migration.md)** - Detailed migration plan for csi-operator
- **[gcp-wif-integration.md](./gcp-wif-integration.md)** - GCP Workload Identity Federation integration
