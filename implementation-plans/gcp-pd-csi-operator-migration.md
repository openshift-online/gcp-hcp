# GCP PD CSI Driver Operator Migration to csi-operator

## Executive Summary

Migrating `gcp-pd-csi-driver-operator` to the consolidated `csi-operator` repository is a **prerequisite** for enabling GCP Persistent Disk storage in HyperShift. This document provides a comprehensive guide based on analysis of the existing csi-operator patterns (AWS EBS, Azure Disk, OpenStack Cinder).

## Why Migration is Required

See [csi-driver-operator-merge enhancement](https://github.com/openshift/enhancements/blob/master/enhancements/storage/csi-driver-operator-merge.md) for full details.

1. **Maintenance Simplification**: All CSI driver operators share the same library-go CSI controllers. When fixing CVEs or backporting bugfixes, changes currently require PRs to 15+ repositories. Migration to a single repo reduces this significantly.

2. **HyperShift Support**: The csi-operator has built-in HyperShift patterns including:
   - Asset generation for both `standalone/` and `hypershift/` modes
   - Token-minter sidecar injection for workload identity (STS/WIF)
   - HyperShift-specific hooks (replicas, node selector, labels, tolerations)
   - Automatic kubeconfig volume injection for sidecars
   - Two kubeconfig support (management cluster + guest cluster)

3. **Unified Asset Generation**: The generator automatically creates:
   - Proper RBAC bindings for both guest and control plane
   - Service/ServiceMonitor for metrics
   - Strategic merge patches for HyperShift modifications

4. **Shared Infrastructure**: Common code for:
   - Deployment hooks (CA bundles, proxy, feature gates)
   - Storage class hooks
   - Volume snapshot class hooks
   - Credential handling

## Migration Approach

The migration follows a **two-stage process** to reduce risk:

| Stage | Goal | Changes |
|-------|------|---------|
| **Stage 1** | Repository Migration | Copy code to csi-operator, no functional changes |
| **Stage 2** | Code Refactoring | Adopt csi-operator patterns, add HyperShift support |

---

## Stage 1: Repository Migration (Copy Only)

This stage copies the existing operator code into csi-operator **without any changes**. After this stage, the operator builds from csi-operator but behaves identically to before.

### 1.1 Git Subtree Import

```bash
# 1. Clone csi-operator (target repo)
cd /path/to/csi-operator

# 2. Check .gitignore in source repo (gcp-pd-csi-driver-operator) for conflicting entries
#    See PR #110: https://github.com/openshift/csi-operator/pull/110

# 3. Add gcp-pd-csi-driver-operator as subtree in legacy/
git subtree add --prefix legacy/gcp-pd-csi-driver-operator \
  https://github.com/openshift/gcp-pd-csi-driver-operator.git master --squash

# 4. Push changes back to source repo (maintains sync)
git subtree push --prefix legacy/gcp-pd-csi-driver-operator \
  https://github.com/openshift/gcp-pd-csi-driver-operator.git master
```

### 1.2 Create Dockerfile (pointing to legacy/)

Place `Dockerfile.gcp-pd` and `Dockerfile.gcp-pd.test` at top of csi-operator tree:

```dockerfile
# Dockerfile.gcp-pd - points to legacy/ code (Stage 1)
FROM ...
COPY legacy/gcp-pd-csi-driver-operator /go/src/...
# Build from legacy location
```

```dockerfile
# Dockerfile.gcp-pd.test
FROM src
```

Verify you can build an image from csi-operator repository.

### 1.3 Update openshift/release

Create PR to build the operator from csi-operator: [Example PR #46233](https://github.com/openshift/release/pull/46233)

- Update build location to point to csi-operator
- Update `storage-conf-csi-gcp-pd-commands.sh` - test manifest at different location
- **Rehearse jobs must pass for both old and new versions**

### 1.4 Update ocp-build-data

Create PR to change image source: [Example PR #4148](https://github.com/openshift-eng/ocp-build-data/pull/4148)

- Change image source location to csi-operator
- **Important**: Add cachito line to build with vendor from `legacy/` directory
- Request ART scratch build for testing

**Test the scratch build:**

```bash
oc adm release new \
  --from-release=registry.ci.openshift.org/ocp/release:4.x.0-0.nightly.XYZ \
  gcp-pd-csi-driver-operator=<scratch-build-image> \
  --to-image=quay.io/<user>/scratch:release1 \
  --name=4.x.0-0.nightly.test.1

oc adm release extract --command openshift-install quay.io/<user>/scratch:release1
# Install a cluster and verify it works
```

### 1.5 Merge Coordination

**⚠️ Critical**: The openshift/release and ocp-build-data PRs must be merged at approximately the same time. There is a robot that syncs data between these repos and will break things if they use different source repositories.

After both PRs merge, the operator builds from csi-operator with **no functional changes**.

---

## Stage 2: Code Refactoring (New Structure)

This stage refactors the operator to use csi-operator patterns and adds HyperShift support.

### 2.1 Create Directory Structure

```
assets/overlays/gcp-pd/
├── base/
│   ├── csidriver.yaml                    # Copy from gcp-pd-csi-driver-operator
│   ├── storageclass.yaml                 # pd-standard StorageClass
│   ├── storageclass_ssd.yaml             # pd-ssd StorageClass  
│   └── volumesnapshotclass.yaml          # VolumeSnapshotClass
├── generated/                            # Created by `make update` (cmd/generator)
│   ├── standalone/                       # manifests for standalone OCP
│   └── hypershift/                       # manifests for HyperShift mode
└── patches/
    ├── controller_add_driver.yaml        # Driver container patch
    ├── controller_add_hypershift.yaml    # HyperShift token-minter patch
    └── node_add_driver.yaml              # Node DaemonSet patch
```

### 2.2 Create Driver Patch Files

Derive patch files from source assets. Use existing AWS EBS patches as structural templates:

| Patch File | Source Reference | Template |
|------------|------------------|----------|
| `patches/controller_add_driver.yaml` | `legacy/.../assets/controller.yaml` | `assets/overlays/aws-ebs/patches/controller_add_driver.yaml` |
| `patches/controller_add_hypershift.yaml` | (new for HyperShift) | `assets/overlays/aws-ebs/patches/controller_add_hypershift_controller_minter.yaml` |
| `patches/node_add_driver.yaml` | `legacy/.../assets/node.yaml` | `assets/overlays/aws-ebs/patches/node_add_driver.yaml` |

#### Key Differences from AWS EBS

**Controller patch (`controller_add_driver.yaml`):**
- Credential mount: `service_account.json` in `/etc/cloud-sa/` (not AWS INI format)
- Env var: `GOOGLE_APPLICATION_CREDENTIALS` (not `AWS_CONFIG_FILE`)
- No init container for credential file conversion (GCP uses JSON directly)
- Driver args: `--enable-storage-pools=true` (GCP-specific)
- Secret name: `gcp-pd-cloud-credentials`

**Node patch (`node_add_driver.yaml`):**
- Additional udev mounts required for GCP:
  - `/etc/udev`, `/lib/udev`, `/run/udev`, `/sys`
- Plugin path: `/var/lib/kubelet/plugins/pd.csi.storage.gke.io/`
- Driver args: `--enable-storage-pools=true`

**HyperShift patch (`controller_add_hypershift.yaml`):**

Copy from `aws-ebs/patches/controller_add_hypershift_controller_minter.yaml` and change:
```diff
- --service-account-name=aws-ebs-csi-driver-controller-sa
+ --service-account-name=gcp-pd-csi-driver-controller-sa
```

The patch already includes the complete token-minter sidecar, volume mounts, and `bound-sa-token` emptyDir override.

### 2.3 Create Driver Go Implementation

Port logic from `legacy/gcp-pd-csi-driver-operator/pkg/operator/` to `pkg/driver/gcp-pd/gcp_pd.go`.

**Hooks to port from source:**

| Source Hook | Source File | Purpose |
|-------------|-------------|---------|
| `withCustomLabels` | `starter.go:283-316` | Add GCP resource labels to driver args |
| `withCustomResourceTags` | `starter.go:318-355` | Add GCP resource tags to driver args |
| `withVolumeAttributesClass` | `starter.go:388-411` | Enable VAC feature gate for provisioner/resizer |
| `getKMSKeyHook` | `storageclasshook.go:13-48` | Set KMS key in StorageClass parameters |

Create `pkg/driver/gcp-pd/gcp_pd.go` using `pkg/driver/aws-ebs/aws_ebs.go` as template.

**Key functions to implement:**

| Function | Purpose |
|----------|---------|
| `GetGCPPDGeneratorConfig()` | Asset generation config (sidecars, patches, base assets) |
| `GetGCPPDOperatorConfig()` | Runtime operator config (driver name, asset dir) |
| `GetGCPPDOperatorControllerConfig()` | Hook registration, HyperShift support |

**GCP-specific differences from AWS EBS:**

| Aspect | AWS EBS | GCP PD |
|--------|---------|--------|
| Driver name | `ebs.csi.aws.com` | `pd.csi.storage.gke.io` |
| Credential secret | `ebs-cloud-credentials` | `gcp-pd-cloud-credentials` |
| Service account | `aws-ebs-csi-driver-controller-sa` | `gcp-pd-csi-driver-controller-sa` |
| Custom hooks | Region, custom tags | Labels, resource tags, KMS key, VAC |

**HyperShift support:** Use `operator.WithTokenMinter("gcp-pd-csi-driver-controller-sa")` for WIF authentication.

### 2.4 Create Main Entry Point

Create `cmd/gcp-pd-csi-driver-operator/main.go` using `cmd/aws-ebs-csi-driver-operator/main.go` as template.

**GCP-specific differences from AWS EBS:**

| AWS EBS | GCP PD |
|---------|--------|
| `aws_ebs "...pkg/driver/aws-ebs"` | `gcp_pd "...pkg/driver/gcp-pd"` |
| `aws-ebs-csi-driver-operator` | `gcp-pd-csi-driver-operator` |
| `GetAWSEBSOperatorConfig()` | `GetGCPPDOperatorConfig()` |

**Key requirement:** The `--guest-kubeconfig` flag enables HyperShift integration, allowing the operator 
running in the management cluster to communicate with the guest cluster.

### 2.5 Update Generator

Update `cmd/generator/main.go` to include GCP PD:

```go
import (
	// ... existing imports ...
	gcp_pd "github.com/openshift/csi-operator/pkg/driver/gcp-pd"
)

func collectConfigs() []*generator.CSIDriverGeneratorConfig {
	return []*generator.CSIDriverGeneratorConfig{
		aws_ebs.GetAWSEBSGeneratorConfig(),
		aws_efs.GetAWSEFSGeneratorConfig(),
		azure_disk.GetAzureDiskGeneratorConfig(),
		azure_file.GetAzureFileGeneratorConfig(),
		gcp_pd.GetGCPPDGeneratorConfig(),  // ADD THIS
		openstack_cinder.GetOpenStackCinderGeneratorConfig(),
		openstack_manila.GetOpenStackManilaGeneratorConfig(),
		samba.GetSambaGeneratorConfig(),
	}
}
```

### 2.6 Add Port Registry Entry

Update `pkg/driver/common/generator/port_registry.go`:

```go
const (
	// ... existing entries ...

	// GCP PD ports - can reuse same ports as AWS EBS/Azure Disk since
	// different cloud providers won't run on the same cluster
	GCPPDLoopbackMetricsPortStart = 8201
	GCPPDExposedMetricsPortStart  = 9201
)
```

### 2.7 Update Dockerfile (Point to New Location)

Create `Dockerfile.gcp-pd` at root:

```dockerfile
FROM registry.ci.openshift.org/ocp/builder:rhel-9-golang-1.22-openshift-4.18 AS builder
WORKDIR /go/src/github.com/openshift/csi-operator
COPY . .
RUN make build-gcp-pd

FROM registry.ci.openshift.org/ocp/4.18:base-rhel9
COPY --from=builder /go/src/github.com/openshift/csi-operator/bin/gcp-pd-csi-driver-operator /usr/bin/
ENTRYPOINT ["/usr/bin/gcp-pd-csi-driver-operator"]
```

### 2.8 Update Makefile

No explicit Makefile changes needed. The csi-operator uses build-machinery-go which auto-discovers build targets. The Dockerfile uses:

```makefile
# In Dockerfile - the Makefile handles this automatically:
RUN make GO_BUILD_PACKAGES=./cmd/gcp-pd-csi-driver-operator
```

### 2.9 openshift/release Update

Update Dockerfile path from `legacy/` to `/`, update test manifest paths:

- Update Prow job configurations to reference new Dockerfile location
- Update test manifest paths to `test/e2e/gcp-pd/`
- Rehearse jobs before merging

### 2.10 ocp-build-data Update

Update `dockerfile` field from `legacy/Dockerfile.gcp-pd` to `Dockerfile.gcp-pd`, update cachito config:

- Remove `legacy/` from cachito configuration
- Example: [PR #4219](https://github.com/openshift-eng/ocp-build-data/pull/4219)

### 2.11 Merge Coordination

Both openshift/release and ocp-build-data PRs must merge together (robot sync dependency).

### 2.12 Post-Refactoring Cleanup

After the refactored code is merged and build config updated:

**Clean up legacy/ directory:**

Once the new structure is proven stable, remove the `legacy/gcp-pd-csi-driver-operator` directory.

## Files to Create/Modify Summary

| File | Action | Description |
|------|--------|-------------|
| `assets/overlays/gcp-pd/base/csidriver.yaml` | Create | Copy from gcp-pd-csi-driver-operator |
| `assets/overlays/gcp-pd/base/storageclass.yaml` | Create | pd-standard StorageClass |
| `assets/overlays/gcp-pd/base/storageclass_ssd.yaml` | Create | pd-ssd StorageClass |
| `assets/overlays/gcp-pd/base/volumesnapshotclass.yaml` | Create | VolumeSnapshotClass |
| `assets/overlays/gcp-pd/patches/controller_add_driver.yaml` | Create | Driver container patch |
| `assets/overlays/gcp-pd/patches/controller_add_hypershift.yaml` | Create | Token-minter patch |
| `assets/overlays/gcp-pd/patches/node_add_driver.yaml` | Create | Node DaemonSet patch |
| `pkg/driver/gcp-pd/gcp_pd.go` | Create | Driver configuration |
| `cmd/gcp-pd-csi-driver-operator/main.go` | Create | Entry point |
| `cmd/generator/main.go` | Modify | Add GCP PD to generator |
| `pkg/driver/common/generator/port_registry.go` | Modify | Add GCP PD ports |
| `Dockerfile.gcp-pd` | Create | Build Dockerfile |
| `Dockerfile.gcp-pd.test` | Create | Test Dockerfile (Stage 1) |
| `test/e2e/gcp-pd/manifest.yaml` | Move | E2E test manifest (from source) |
| `test/e2e/gcp-pd/ocp-manifest.yaml` | Move | OCP stress test manifest (from source) |
| `test/e2e/gcp-pd/hyperdisk-manifest.yaml` | Move | Hyperdisk test manifest (from source) |
| `test/e2e/gcp-pd/volumeattributesclass.yaml` | Move | VAC for hyperdisk tests (from source) |

## Dependency on cluster-storage-operator

Even after migrating to csi-operator, CSO must be updated to:

1. Add `GetGCPPDCSIOperatorConfig()` to `HyperShiftStarter.populateConfigs()`
2. Include proper HyperShift assets for GCP PD

This is documented in `gcp-persistent-disk-csi-driver.md`.

## Testing

1. Run `make update` to generate assets
2. Run `make test` to verify tests pass
3. Build image and test on standalone cluster
4. Build image and test on HyperShift cluster
5. Verify WIF authentication works in HyperShift mode

### E2E Test Manifests

Move existing test manifests from `legacy/gcp-pd-csi-driver-operator/test/e2e/` to `test/e2e/gcp-pd/`:

| Source File | Destination |
|-------------|-------------|
| `legacy/.../test/e2e/manifest.yaml` | `test/e2e/gcp-pd/manifest.yaml` |
| `legacy/.../test/e2e/ocp-manifest.yaml` | `test/e2e/gcp-pd/ocp-manifest.yaml` |
| `legacy/.../test/e2e/hyperdisk-manifest.yaml` | `test/e2e/gcp-pd/hyperdisk-manifest.yaml` |
| `legacy/.../test/e2e/volumeattributesclass.yaml` | `test/e2e/gcp-pd/volumeattributesclass.yaml` |

These files already exist in the source repository - no need to create from scratch.

### CI Setup in openshift/release

Set up or migrate existing CI jobs from `gcp-pd-csi-driver-operator` to the new `csi-operator` location:

#### Infrastructure Requirements

- GCP project with appropriate quotas for e2e testing
- Service account credentials for CI cluster provisioning
- Persistent disk quota for storage tests

#### Job Configuration

Create job configurations in `openshift/release`:

| Job Type | Purpose |
|----------|---------|
| **Presubmit** | Run on PRs to validate changes before merge |
| **Periodic** | Scheduled runs to catch regressions |
| **Informing** | Optional jobs that don't block PRs |

#### Test Manifest Location

Update `storage-conf-csi-gcp-pd-commands.sh` to reference the new test manifest location:

```bash
# Point to csi-operator test manifest location
TEST_MANIFEST="${REPO_ROOT}/test/e2e/gcp-pd/manifest.yaml"
```

#### Example Job Files

Reference existing GCP jobs in `openshift/release` for patterns:
- `ci-operator/config/openshift/csi-operator/` - csi-operator job configs
- `ci-operator/jobs/openshift/csi-operator/` - generated job definitions

## References

- [csi-operator migration docs](https://github.com/openshift/csi-operator/blob/main/doc/migrating-operators.md)
- [csi-operator README](https://github.com/openshift/csi-operator/blob/main/README.md)
- [AWS EBS implementation](https://github.com/openshift/csi-operator/tree/main/pkg/driver/aws-ebs)
- [Azure Disk implementation](https://github.com/openshift/csi-operator/tree/main/pkg/driver/azure-disk)

