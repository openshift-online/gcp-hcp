# GCP GA Readiness Checklist

Tracking document for everything needed to promote GCP from `TechPreviewNoUpgrade` to GA.

Last updated: 2026-08-04.

---

## Jira Ticket Mapping

| Section | Checklist Items | Jira Ticket | Parent Epic |
|---------|----------------|-------------|-------------|
| 1. API & Feature Gate | 1.1–1.4 | GCP-957 | GCP-451 |
| 2. KMS / Secret Encryption (API) | 2.1, 2.2, 2.6 | GCP-962 | GCP-506 |
| 2. KMS / Secret Encryption (Provider) | 2.3–2.5, 2.7 | GCP-963 | GCP-506 |
| 3. Guest Credentials (Webhook) | 3.1 | GCP-967 | GCP-955 |
| 3. Guest Credentials (CSI) | 3.2, 3.4 | GCP-968 | GCP-955 |
| 3. Guest Credentials (Ingress SA) | 3.3 | GCP-961 | GCP-314 |
| 4. CSI / Storage | 4.1–4.5 | GCP-958 | GCP-322 |
| 5. Cluster Lifecycle | 5.1–5.4 | GCP-503, GCP-507 | GCP-502 |
| 6. Monitoring & Alerting | 6.1–6.3 | GCP-959 | GCP-451 |
| 7. Testing — Unit & CEL | 7.1, 7.3, 7.5 | GCP-960 | GCP-451 |
| 8. Testing — E2E (PlatformConfig) | 8.1, 8.2, 8.5, 8.7 | GCP-964 | GCP-509 |
| 8. Testing — E2E (Platform tests) | 8.4, 8.6 | GCP-965 | GCP-509 |
| 9. CI | 9.1, 9.2 | GCP-966 | GCP-509 |
| 10. Documentation (Operator guides) | 10.1–10.4 | GCP-969 | GCP-956 |
| 10. Documentation (Reference) | 10.5, 10.6 | GCP-970 | GCP-956 |
| 11. OCP Release Payload (CSI creds) | 11.1, 11.3 | GCP-968 | GCP-955 |
| 11. OCP Release Payload (Ingress) | 11.2 | GCP-961 | GCP-314 |
| 11. OCP Release Payload (CVO) | 11.4 | GCP-958 | GCP-322 |

Items without dedicated tickets: 5.1–5.4 (covered by existing GCP-503/GCP-507), 6.4 (deferred), 6.5 (nice-to-have), 7.2 (folds into existing work), 7.4 (incremental), 8.3 (stretch), 8.8–8.9 (nice-to-have), 9.3 (no changes needed), 5.3–5.4 (inline fixes).

---

## 1. API & Feature Gate

All four items form a single atomic change and should ship in one PR. Dependency
order: 1.1 + 1.2 → 1.3 → 1.4.

**Key finding:** `GCPPlatform` is purely a CRD schema gate — it is never checked
at runtime via `Gate().Enabled()`. All runtime GCP behavior is driven by the
`PlatformType` value on the CR, so no controller or webhook code changes are needed.
The `AutoNodeKarpenter` graduation (commit `df33c3502b`) is the best precedent.

- [ ] **1.1 Remove TechPreview feature gate**

    Delete the `GCPPlatform` constant, variable, and registration from
    `hypershift-operator/featuregate/feature.go`. Following the `AutoNodeKarpenter`
    precedent, the feature gate is removed entirely rather than moved to Default.

    Lines to remove:

    - Line 30: `GCPPlatform featuregate.Feature = "GCPPlatform"` (and comment block lines 25–29)
    - Line 57: `gcpHCPFeature = featuregates.NewFeature(GCPPlatform, featuregates.WithEnableForFeatureSets(configv1.TechPreviewNoUpgrade))`
    - Line 67: `allFeatures.AddFeature(gcpHCPFeature)`

    Risk: **Low.** Confirmed via grep that `GCPPlatform` is never checked via
    `Gate().Enabled()` anywhere in the codebase. The only consumers are the CRD
    generation markers (1.2) and featuregate manifest files (1.3).

    **Also update:** `hypershift-operator/featuregate/feature_test.go` — has
    direct references to `featuregate.GCPPlatform` that will break at compile
    time when the constant is deleted:

    - Delete `TestGCPPlatformFeatureGate` (lines 73–87) entirely — it only
      tests `GCPPlatform`
    - Remove `"GCPPlatform"` entries from all 3 expected maps in
      `TestAllHypershiftOperatorFeatureGates` (lines 102, 113, 124)
    - Delete the `GCPPlatform` assertion block (lines 148–152)

- [ ] **1.2 Remove FeatureGateAwareEnum markers**

    Two marker types across 3 files, 5 locations total.

    **Type 1 — `FeatureGateAwareEnum`** (controls which enum values appear in the CRD):

    `api/hypershift/v1beta1/hostedcluster_types.go:1416–1417` and
    `api/hypershift/v1beta1/nodepool_types.go:573–574` — current:

    ```go
    // +openshift:validation:FeatureGateAwareEnum:featureGate="",enum=AWS;Azure;IBMCloud;KubeVirt;Agent;PowerVS;None
    // +openshift:validation:FeatureGateAwareEnum:featureGate=OpenStack;GCPPlatform,enum=AWS;Azure;IBMCloud;KubeVirt;Agent;PowerVS;None;OpenStack;GCP
    ```

    Change to (move `GCP` to the ungated line, remove `GCPPlatform` from the gated
    line — OpenStack remains TechPreview):

    ```go
    // +openshift:validation:FeatureGateAwareEnum:featureGate="",enum=AWS;Azure;IBMCloud;KubeVirt;Agent;PowerVS;None;GCP
    // +openshift:validation:FeatureGateAwareEnum:featureGate=OpenStack,enum=AWS;Azure;IBMCloud;KubeVirt;Agent;PowerVS;None;GCP;OpenStack
    ```

    **Type 2 — `+openshift:enable:FeatureGate`** (controls whether the field/CRD appears at all):

    Remove the `// +openshift:enable:FeatureGate=GCPPlatform` marker from:

    - `api/hypershift/v1beta1/hostedcluster_types.go:1463` — `PlatformSpec.GCP` field
    - `api/hypershift/v1beta1/nodepool_types.go:615` — `NodePoolPlatform.GCP` field
    - `api/hypershift/v1beta1/gcpprivateserviceconnect_types.go:143` — top-level `GCPPrivateServiceConnect` type

    Impact: `GCP` becomes a valid PlatformType in Default CRDs, `spec.platform.gcp`
    appears in Default CRDs for HostedCluster/NodePool/HostedControlPlane, and
    `GCPPrivateServiceConnect` gains a Default CRD variant (currently TechPreview-only).

- [ ] **1.3 Regenerate CRD manifests**

    **Step 1:** Update 4 featuregate manifest files — remove `{"name": "GCPPlatform"}`
    from each:

    | File | Remove from |
    |------|-------------|
    | `api/hypershift/v1beta1/featuregates/featureGate-Hypershift-Default.yaml` | `disabled` list (line 33–35) |
    | `api/hypershift/v1beta1/featuregates/featureGate-Hypershift-TechPreviewNoUpgrade.yaml` | `enabled` list (line 43–45) |
    | `api/hypershift/v1beta1/featuregates/featureGate-SelfManagedHA-Default.yaml` | `disabled` list (line 33–35) |
    | `api/hypershift/v1beta1/featuregates/featureGate-SelfManagedHA-TechPreviewNoUpgrade.yaml` | `enabled` list (line 43–45) |

    **Note:** 4 corresponding copies exist under
    `cmd/install/assets/crds/hypershift-operator/payload-manifests/featuregates/`.
    These are regenerated automatically by `make api` — no manual edits needed.

    **Step 2:** Run `make api`. This invokes the 3-phase codegen pipeline
    (`empty-partial-schemas` → `schemapatch` → `crd-manifest-merge`) and produces:

    - `hostedclusters-Hypershift-Default.crd.yaml` with full GCP schema (~1,700 new lines)
    - `nodepools-Default.crd.yaml` with full GCP schema (~500 new lines)
    - `hostedcontrolplanes-Hypershift-Default.crd.yaml` with full GCP schema
    - New file: `gcpprivateserviceconnects-Hypershift-Default.crd.yaml`
      (and SelfManagedHA-Default variant)
    - `zz_generated.featuregated-crd-manifests/`: GCPPlatform partials disappear;
      content moves into `AAA_ungated.yaml` files
    - `zz_generated.featuregated-crd-manifests.yaml`: `GCPPlatform` removed from all entries

    The codegen pipeline is: `controller-gen` (deepcopy) + OpenShift `codegen`
    (`github.com/openshift/api/tools/codegen/cmd`) which processes
    `+openshift:enable:FeatureGate` and `+openshift:validation:FeatureGateAwareEnum`
    markers. The merge step uses the featuregate YAML files above to decide which
    partials go into which per-profile CRD.

    Current state of GCP in generated CRDs (Default column changes after this step):

    | CRD | Default | TechPreview |
    |-----|---------|-------------|
    | `hostedclusters` — `GCP` in type enum | **NO** | YES |
    | `hostedclusters` — `spec.platform.gcp` | **NO** | YES |
    | `nodepools` — `GCP` in type enum | **NO** | YES |
    | `nodepools` — `spec.platform.gcp` | **NO** | YES |
    | `hostedcontrolplanes` — GCP fields | **NO** | YES |
    | `gcpprivateserviceconnects` — CRD exists | **NO** | YES |

    Note: GCP CAPI CRDs (`cluster-api-provider-gcp/`) are already ungated and
    unaffected — they use plain `controller-gen` without feature gate machinery.

    Note: There is an existing ungated `gcp:` field under
    `spec.configuration.ingress[].endpointPublishingStrategy.loadBalancer.providerParameters.gcp`
    from upstream OpenShift `configv1.IngressController`. This is completely
    separate from `spec.platform.gcp` and is unaffected.

- [ ] **1.4 Migrate CEL envtest suite to stable**

    **Prerequisite:** Items 1.1–1.3 must be completed first. If `featureGates` is
    removed before GCP fields exist in the Default CRD, the tests fail.

    **Current file:**
    `cmd/install/assets/crds/hypershift-operator/tests/hostedclusters.hypershift.openshift.io/techpreview.hostedclusters.gcp.testsuite.yaml`

    Contains 20 test cases (14 onCreate, 6 onUpdate):

    - onCreate: project/region format, network name (format, length, empty), PSC
      subnet name, endpointAccess enum, Private/PublicAndPrivate valid, hyphens,
      single-char names, network SA (email format, wrong project, valid, omitted),
      imageRegistry SA wrong project
    - onUpdate: network SA immutability, release image update with unchanged SA,
      network SA ratcheting (2 cases), imageRegistry SA immutability,
      imageRegistry SA ratcheting (2 cases)

    **Changes:**

    1. Rename file: `techpreview.hostedclusters.gcp.testsuite.yaml` →
       `stable.hostedclusters.gcp.testsuite.yaml` (convention only — the test
       runner ignores file prefixes)
    2. Remove lines 4–5 (`featureGates: [GCPPlatform]`) from the YAML header

    How it works: `test/envtest/crd_filter.go:perTestRuntimeInfo()` line 54 — if
    `featureGates` is empty, include ALL CRD files for the matching `crdName` (both
    Default and TechPreview). If non-empty, only include CRDs whose feature set
    has all listed gates enabled. Removing the field makes the suite run against
    both Default and TechPreview CRDs, matching all other stable suites (e.g.,
    `stable.hostedclusters.azure.testsuite.yaml`).

    **Note:** The GCP NodePool envtest suite gap (AWS has 24 test cases, GCP has
    zero) is tracked separately in section 7.

    **Verification:**

    ```bash
    # After all source changes + make api:
    # 1. Verify GCP in Default CRDs
    grep -c 'gcp' cmd/install/assets/crds/hypershift-operator/zz_generated.crd-manifests/hostedclusters-Hypershift-Default.crd.yaml
    # Should show ~27+ (currently ~3)

    # 2. Verify GCPPrivateServiceConnect Default CRD exists
    ls cmd/install/assets/crds/hypershift-operator/zz_generated.crd-manifests/gcpprivateserviceconnects-*
    # Should include a -Default variant

    # 3. Verify GCPPlatform removed from metadata
    grep -r 'GCPPlatform' api/hypershift/v1beta1/zz_generated.featuregated-crd-manifests.yaml
    # Should return nothing

    # 4. Run envtests
    make test-envtest-ocp

    # 5. Full verification
    make verify
    ```

---

## 2. KMS / Secret Encryption

Secret encryption uses a layered architecture:

1. **API types** (`api/hypershift/v1beta1/`) define the provider spec and key status
2. **HO platform layer** (`hypershift-operator/controllers/hostedcluster/internal/platform/`)
   syncs KMS credential secrets from the HC namespace into the control plane namespace
3. **CPO KMS provider** (`control-plane-operator/controllers/hostedcontrolplane/v2/kas/kms/`)
   implements the `KMSProvider` interface to generate the Kubernetes
   `EncryptionConfiguration` and the KAS pod sidecar config (KMS plugin binary +
   volumes + mounts)
4. **Re-encryption controller** (`control-plane-operator/hostedclusterconfigoperator/controllers/reencryption/`)
   drives the key rotation lifecycle (ReadOnlyDeploy → WritePromote → Migrating → Completed)
5. **Support helpers** (`support/secretencryption/`) provide fingerprinting, key
   status derivation, and convergence checks

AWS, Azure, and IBMCloud all have complete implementations across these layers.
GCP has **nothing** — the entire stack must be built.

- [ ] **2.1 Define `GCPKMSSpec` and `GCPKMSKeyEntry` API types**

    Create in `api/hypershift/v1beta1/gcp.go`, following the AWS pattern
    (`AWSKMSSpec` at `aws.go:1011`, `AWSKMSKeyEntry` at `aws.go:1080`):

    ```go
    type GCPKMSSpec struct {
        // keyName is the Cloud KMS crypto key resource name used for etcd encryption.
        // Format: projects/{project}/locations/{location}/keyRings/{keyRing}/cryptoKeys/{key}
        ActiveKey GCPKMSKeyEntry `json:"activeKey"`
        // backupKey is deprecated — same pattern as AWSKMSSpec.BackupKey
        BackupKey *GCPKMSKeyEntry `json:"backupKey,omitempty"`
    }

    type GCPKMSKeyEntry struct {
        // resourceName is the Cloud KMS crypto key resource name.
        // Format: projects/{project}/locations/{location}/keyRings/{keyRing}/cryptoKeys/{key}
        ResourceName string `json:"resourceName,omitempty"`
    }
    ```

    The existing `GCPDiskEncryptionKey` type (`gcp.go:534`) already uses the same
    Cloud KMS resource name format for CMEK node boot disk encryption — reuse the
    same validation regex pattern. The two types are distinct: `GCPDiskEncryptionKey`
    is for node disks, `GCPKMSKeyEntry` is for etcd secret encryption.

    No separate auth struct is needed — GCP uses WIF (Workload Identity Federation),
    and the KMS sidecar authenticates via the same token-minter + SA pattern already
    used by other GCP components. The SA email could be a new field in
    `GCPServiceAccountsEmails` (e.g., `kms`) or reuse an existing SA with
    `roles/cloudkms.cryptoKeyEncrypterDecrypter`.

- [ ] **2.2 Add `GCP` to `KMSProvider` enum and `KMSSpec` struct**

    In `api/hypershift/v1beta1/hostedcluster_types.go`:

    - Add `GCP KMSProvider = "GCP"` constant (after line 2421)
    - Update kubebuilder validation: `+kubebuilder:validation:Enum=IBMCloud;AWS;Azure;GCP`
      (line 2415)
    - Add `GCP *GCPKMSSpec` field to `KMSSpec` struct (after line 2438)
    - Add `SecretEncryptionProviderGCP SecretEncryptionProvider = "GCP"` constant
      (after line 2463)
    - Add `GCP GCPKMSKeyEntry` field to `SecretEncryptionKeyStatus` (after line 2514)
      with CEL validation rule:
      `self.provider == 'GCP' ? has(self.gcp) : !has(self.gcp)`
    - Update `EncryptionKeyReference.Provider` enum to include `GCP` (line 2539)
    - Update `SecretEncryptionKeyStatus.Provider` enum to include `GCP` (line 2501)

- [ ] **2.3 Implement GCP KMS provider (CPO side)**

    This is the core implementation — the KMS plugin that runs alongside kube-apiserver.

    **New file:** `control-plane-operator/controllers/hostedcontrolplane/v2/kas/kms/gcp.go`

    Implement the `KMSProvider` interface (`kms/kms.go:17`):

    ```go
    type KMSProvider interface {
        GenerateKMSEncryptionConfig(apiVersion string) (*v1.EncryptionConfiguration, error)
        GenerateKMSPodConfig() (*KMSPodConfig, error)
    }
    ```

    `GenerateKMSEncryptionConfig()` builds the `EncryptionConfiguration` resource
    that tells KAS how to encrypt etcd data. Following the AWS pattern (`kms/aws.go:89`):
    - Compute a provider name from a hash of the key resource name (like `AWSKMSProviderName`)
    - Create KMS provider entries pointing to Unix sockets (`unix:///var/run/gcpkmsactive.sock`)
    - Append `identity` as fallback provider
    - Target resources from `config.KMSEncryptedObjects()` (secrets, configmaps, routes, oauth tokens)

    `GenerateKMSPodConfig()` generates the sidecar containers and volumes added to the
    KAS pod. Following AWS (`kms/aws.go:145`):
    - Active KMS sidecar container running the GCP KMS encryption provider binary
    - Optional backup sidecar for key rotation
    - Token-minter sidecar for WIF authentication
    - Volumes: KMS socket (emptyDir), GCP credentials secret, cloud provider token

    **Wire into dispatch** in `control-plane-operator/controllers/hostedcontrolplane/v2/kas/kms.go`:

    - Add `gcpWrite`/`gcpRead` fields to `kmsWriteReadKeys` struct (line 67)
    - Add `case hyperv1.GCP` to `deriveKMSKeys()` (line 86) — same two-stage rollout
      pattern as AWS (ReadOnlyDeploy/WritePromote)
    - Add `case hyperv1.GCP` to `getKMSProvider()` (line 179) — instantiate the new
      GCP provider
    - Add `GCPKMS` field to `kmsImages` struct and wire in `newKMSImages()`

    **Prerequisite — GCP KMS provider binary/image:** The Kubernetes KMS v2 protocol
    requires a sidecar binary that listens on a Unix socket and proxies
    encrypt/decrypt calls to GCP Cloud KMS. This is equivalent to:
    - AWS: `aws-encryption-provider` image
    - Azure: `azure-kms-encryption-provider` image

    This image is **external to the HyperShift repo** — it needs to be built or
    sourced separately and added to the OCP release payload.

- [ ] **2.4 Implement `ReconcileSecretEncryption` (HO platform layer)**

    **File:** `hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go:440`

    Currently a no-op with TODO comment. The implementation syncs KMS credential
    secrets from the HostedCluster namespace into the control plane namespace.

    Two patterns exist in the codebase:

    - **AWS pattern** (`aws/aws.go:341`): `ReconcileSecretEncryption` is also a
      no-op — AWS syncs the `kms-creds` secret during `ReconcileCredentials`
      instead (line 330). The secret contains a `credentials` INI file with a
      `role_arn` and `web_identity_token_file`.
    - **Azure pattern** (`azure/azure.go:343`): `ReconcileSecretEncryption`
      actively reconciles the Azure KMS config Secret into the CP namespace.

    For GCP, since authentication uses WIF (same as the AWS IRSA pattern), the
    implementation should either:
    - (a) Sync KMS credentials in `ReconcileCredentials` (AWS pattern — already
      handles other GCP WIF credentials), or
    - (b) Create a dedicated GCP KMS credentials Secret in `ReconcileSecretEncryption`

    The secret must live in the control plane namespace (e.g., `clusters-<name>`)
    where the KAS pod can mount it.

    The `ReconcileSecretEncryption` caller is in
    `hostedcluster_controller.go:2007` — it dispatches based on
    `hcluster.Spec.SecretEncryption.Type == hyperv1.KMS`, then calls
    `p.ReconcileSecretEncryption()` on the platform interface.

- [ ] **2.5 Add `ValidGCPKMSConfig` condition and validation**

    **Step 1: Define the condition**

    In `api/hypershift/v1beta1/hostedcluster_conditions.go` (after line 165):

    ```go
    // ValidGCPKMSConfig indicates whether the GCP KMS key is valid and operational
    ValidGCPKMSConfig ConditionType = "ValidGCPKMSConfig"
    ```

    **Step 2: Implement validation in CPO**

    In `control-plane-operator/controllers/hostedcontrolplane/hostedcontrolplane_controller.go`,
    add a `validateGCPKMSConfig()` method following the AWS pattern (`validateAWSKMSConfig`
    at line 2984):
    - Obtain GCP credentials via WIF
    - Create a Cloud KMS client
    - Attempt a `kms.Encrypt()` call to verify the key is accessible
    - Set `ValidGCPKMSConfig` condition to True/False based on result

    Add `case hyperv1.GCPPlatform:` to the validation dispatch switch (after
    line 645):
    ```go
    case hyperv1.GCPPlatform:
        r.validateGCPKMSConfig(ctx, hostedControlPlane)
    ```

    **Step 3: Bubble condition from HCP to HostedCluster**

    In `hypershift-operator/controllers/hostedcluster/hostedcluster_controller.go`
    (after line 893), add:
    ```go
    if hcluster.Spec.Platform.Type == hyperv1.GCPPlatform {
        validKMSConfig := meta.FindStatusCondition(hcp.Status.Conditions, string(hyperv1.ValidGCPKMSConfig))
        if validKMSConfig != nil {
            validKMSConfig.ObservedGeneration = hcluster.Generation
            meta.SetStatusCondition(&hcluster.Status.Conditions, *validKMSConfig)
        }
    }
    ```

    Same pattern in `reconcile_legacy.go` (after line 574).

- [ ] **2.6 Wire support helpers for key status and fingerprinting**

    **File:** `support/secretencryption/keystatus.go`

    - Add `KeyStatusFromGCPSpec()` (following `KeyStatusFromAWSSpec` at line 16)
    - Add `case hyperv1.GCP:` to `KeyStatusFromSpec()` switch (after line 72)

    **File:** `support/secretencryption/fingerprint.go`

    - Add `FingerprintGCPKMSKey()` (following `FingerprintAWSKMSKey` at line 23):
      hash the resource name
    - Add `case hyperv1.SecretEncryptionProviderGCP:` to `FingerprintFromKeyStatus()`
      (after line 63)

    **File:** `control-plane-operator/hostedclusterconfigoperator/controllers/reencryption/reencryption.go`

    - Add `case hyperv1.SecretEncryptionProviderGCP:` to
      `computeTargetKeyProviderName()` (after line 356)

- [ ] **2.7 Add GCP KMS provider image reference**

    The KMS plugin sidecar image must be added to:
    - `kmsImages` struct in `control-plane-operator/controllers/hostedcontrolplane/v2/kas/kms.go`
    - Image resolution in `newKMSImages()` (or equivalent)
    - Optionally, an annotation constant for image override (like
      `AWSKMSProviderImage` in the hypershift API annotations)

---

## 3. Guest Cluster Operands (HCCO)

HCCO (Hosted Cluster Config Operator) runs in the control plane namespace and
reconciles resources into the guest cluster. Three subsystems are relevant:

1. **Cloud credential secrets** — HCCO creates per-operand credential Secrets in
   guest namespaces via `reconcileCloudCredentialSecrets()` in `resources.go:2140`.
   The platform switch dispatches to `gcpresources.SetupOperandCredentials()`.
2. **Identity webhooks** — MutatingWebhookConfigurations deployed into the guest
   cluster that inject cloud tokens into pods at admission time.
3. **Operand CRs** — `ClusterCSIDriver`, `StorageClass`, etc. registered in the
   guest cluster by `reconcileStorage()` in `resources.go:3498`.

### What's already working

- **CNCC** is fully wired: GCP is in `platformHasCloudNetworkConfigController()`
  (`v2/cno/component.go:89`), `GCP_CNCC_CREDENTIALS_FILE` env var is set
  (`v2/cno/deployment.go:164`), and the `Network` SA provides the credential
  via token-minter. CNCC runs on the management cluster — no guest-side
  credential needed.
- **Image Registry credential** is provisioned: `SetupOperandCredentials()` in
  `gcp/gcp.go` creates `installer-cloud-credentials` in
  `openshift-image-registry` using the `ImageRegistry` SA email.
- **Token-minter cloud tokens** work for GCP: the condition at
  `support/controlplane-component/token-minter-container.go:60` includes
  `hyperv1.GCPPlatform`.

### Credential flow: two-layer (static + dynamic)

GCP uses a different mechanism than AWS/Azure:

- **Static layer**: `service_account.json` Secret (External Account Credential
  JSON) created by HCCO's `SetupOperandCredentials`. Contains WIF audience,
  STS token URL, GSA impersonation URL, and
  `credential_source.file: "/var/run/secrets/openshift/serviceaccount/token"`.
  Built by `gcputil.BuildWorkloadIdentityCredentials()` at
  `support/gcputil/gcputil.go:34`.
- **Dynamic layer**: token-minter sidecar continuously refreshes the projected
  service account token at the file path referenced by the credential JSON.

| Operand | Guest namespace | AWS | Azure | GCP |
|---------|----------------|-----|-------|-----|
| Image Registry | `openshift-image-registry` | IRSA | WI | WIF |
| Storage/CSI | `openshift-cluster-csi-drivers` | IRSA | WI | **missing** |
| Ingress | `openshift-ingress-operator` | IRSA | WI | **missing** |
| CNCC | CP-side (token-minter) | CP token-minter | CP token-minter | CP token-minter |

- [ ] **3.1 Implement GCP workload identity webhook**

    **Decision (2026-08-04): Build a GCP workload identity webhook for GA,
    following the AWS/Azure pattern (Option B).**

    **Rationale:** Although GCP's WIF credential JSON (`service_account.json`)
    is self-contained (it embeds the token file path in `credential_source.file`),
    guest-side pods still need a **projected ServiceAccountToken volume** mounted
    at that path (`/var/run/secrets/openshift/serviceaccount/token`). Without a
    webhook to inject this volume, guest-side workloads (e.g., CSI node plugin
    DaemonSet) cannot authenticate to GCP.

    Today, CP-side pods get tokens via the token-minter sidecar, but guest-side
    pods have no equivalent mechanism. Relying on the GCE instance metadata
    server was considered and rejected because:
    - **Security:** Any pod on the node can obtain the VM's SA token — a
      privilege escalation vector (the exact problem GKE Workload Identity
      was created to solve).
    - **Scope limitations:** Default NodePool SA scopes (`devstorage.read_only`,
      `logging.write`, `monitoring.write`) lack compute-related scopes the CSI
      node plugin may need.
    - **Proxy environments:** `no_proxy` at `support/proxy/no_proxy.go:36` does
      not include `169.254.169.254` for GCP (only AWS/Azure), so metadata
      requests would be proxied and fail.
    - **No parity:** Azure explicitly uses WIF for both controller and node CSI
      SAs (`cmd/infra/azure/identities.go:188-209`), not VM-level identity.

    AWS deploys `aws-pod-identity-webhook` (`reconcileAWSIdentityWebhook` at
    `resources.go:2606`) as a KAS sidecar (`kas/deployment.go:341`,
    `applyAWSPodIdentityWebhookContainer`) that intercepts Pod creates and
    injects projected SA token volumes + `AWS_WEB_IDENTITY_TOKEN_FILE` env var.

    Azure deploys `azure-workload-identity-webhook` (`reconcileAzureIdentityWebhook`
    at `resources.go:2669`) as a KAS sidecar (`kas/deployment.go:395`,
    `applyAzureWorkloadIdentityWebhookContainer`) with ObjectSelector on
    `azure.workload.identity/use: "true"`.

    Both are registered via `reconcilePlatformSpecificResources()` at
    `resources.go:545`:
    ```go
    case hyperv1.AWSPlatform:
        errs = append(errs, r.reconcileAWSIdentityWebhook(ctx)...)
    case hyperv1.AzurePlatform:
        errs = append(errs, r.reconcileAzureIdentityWebhook(ctx)...)
    ```
    There is no `case hyperv1.GCPPlatform`.

    **Implementation (following the AWS/Azure pattern):**

    1. **KAS sidecar** — Add `gcp-workload-identity-webhook` container to the
       KAS deployment (`kas/deployment.go`). The webhook intercepts Pod creates
       in the guest cluster and injects:
       - A projected ServiceAccountToken volume with the WIF audience
       - `GOOGLE_APPLICATION_CREDENTIALS` env var pointing to the credential
         file path

    2. **Guest-side webhook registration** — Create
       `reconcileGCPIdentityWebhook()` in HCCO (`resources.go`) to register a
       `MutatingWebhookConfiguration` + RBAC in the guest cluster (following
       `reconcileAWSIdentityWebhook` at `resources.go:2606` and
       `reconcileAzureIdentityWebhook` at `resources.go:2669`)

    3. **Add `case hyperv1.GCPPlatform:`** to
       `reconcilePlatformSpecificResources()` at `resources.go:545`

    4. **Add WIF bindings for node-side SAs** in
       `cmd/infra/gcp/iam-bindings.json` — currently only
       `gcp-pd-csi-driver-controller-sa` is bound. Add bindings for node SAs
       (e.g., `gcp-pd-csi-driver-node-sa`) following the Azure pattern in
       `cmd/infra/azure/identities.go:188-209`

    5. **Webhook binary/image** — Source or build a GCP workload identity
       webhook binary. Evaluate whether an existing open-source implementation
       can be reused or whether a new one must be written. Add the image to
       the OCP release payload.

    **Parity matrix after implementation:**

    | Piece | AWS | Azure | GCP |
    |-------|-----|-------|-----|
    | KAS sidecar webhook | `aws-pod-identity-webhook` | `azure-workload-identity-webhook` | `gcp-workload-identity-webhook` |
    | Token injection | Projected SA token + `AWS_WEB_IDENTITY_TOKEN_FILE` | Projected SA token + `AZURE_FEDERATED_TOKEN_FILE` | Projected SA token + `GOOGLE_APPLICATION_CREDENTIALS` |
    | Guest credential secrets | HCCO per-operand secrets | HCCO per-operand secrets | HCCO per-operand secrets |
    | Node SA WIF bindings | IRSA for controller + node | Federated creds for controller + node | WIF for controller + node |

    **Cross-references:** Items 3.2 (CSI guest credential), 3.3 (ingress SA),
    11.1 (storage credential), 11.2 (ingress credential) all depend on this
    webhook being in place for guest-side pods to authenticate.

- [ ] **3.2 Add storage/CSI guest-side credential**

    AWS creates `ebs-cloud-credentials` in `openshift-cluster-csi-drivers`
    (resources.go:2150). Azure creates `azure-disk-credentials` and
    `azure-file-credentials` (azure.go SetupOperandCredentials). GCP creates
    **nothing** for CSI.

    The `Storage` SA email already exists in `GCPServiceAccountsEmails`
    (`gcp.go:344`). Add a `gcpCredentialConfig` entry to `SetupOperandCredentials()`
    targeting `openshift-cluster-csi-drivers/gcp-pd-cloud-credentials` using the
    `Storage` SA.

    **Cross-reference:** This also needs the `ClusterCSIDriver` registration in
    item 3.4.

- [ ] **3.3 Add dedicated ingress service account**

    **Decision (2026-08-04): Add a dedicated `Ingress` SA to
    `GCPServiceAccountsEmails`, following the AWS/Azure pattern.**

    **Rationale:** The ingress operator currently reuses the ControlPlane SA
    (`ctrlplane-op`), which has `roles/dns.admin`, `roles/compute.networkAdmin`,
    and `roles/compute.viewer`. The ingress operator only needs DNS management
    (`roles/dns.admin`) and possibly LB state reads (`roles/compute.viewer`).
    Granting `roles/compute.networkAdmin` to the ingress operator violates
    least-privilege. AWS and Azure both have dedicated ingress identities with
    scoped permissions (AWS: `route53:ChangeResourceRecordSets` +
    `elasticloadbalancing:DescribeLoadBalancers`; Azure: custom ingress role
    scoped to the DNS zone RG).

    Additionally, token-minter at `v2/ingressoperator/component.go:74` already
    mints a cloud token for `openshift-ingress-operator/ingress-operator`, but
    no GCP SA has a WIF binding for that K8s SA — so the STS exchange currently
    fails silently.

    No backward compatibility needed — GCP is not GA yet.

    **Implementation:**

    1. **API change** — Add `Ingress` field to `GCPServiceAccountsEmails` in
       `api/hypershift/v1beta1/gcp.go` (after the existing six SAs: NodePool,
       ControlPlane, CloudController, Storage, ImageRegistry, Network)

    2. **IAM roles** — Create an `ingress-op` GSA in
       `cmd/infra/gcp/iam-bindings.json` with:
       - `roles/dns.admin` (Cloud DNS record management for `*.apps.<domain>`)
       - `roles/compute.viewer` (read LB state to determine ingress IP)

    3. **WIF binding** — Bind the new GSA to K8s SA
       `openshift-ingress-operator/ingress-operator` in `iam-bindings.json`

    4. **CLI IAM creation** — Update `cmd/infra/gcp/create_iam.go` to
       provision the new Ingress GSA with the roles above

    5. **CP-side credential** — Update `ReconcileCredentials` in
       `hypershift-operator/controllers/hostedcluster/internal/platform/gcp/gcp.go`
       to create an ingress credential Secret in the CP namespace using the
       `Ingress` SA email (following the existing pattern for other SAs)

    6. **Guest-side credential** — Add `GCPIngressCloudCredsSecret()` to
       `manifests/creds.go` targeting
       `openshift-ingress-operator/cloud-credentials`, and add a
       `gcpCredentialConfig` entry to `SetupOperandCredentials()` in
       `gcp/gcp.go` using the `Ingress` SA email

    7. **Token-minter update** — Update `ingressoperator/component.go` to
       use the new `Ingress` SA for token minting instead of falling through
       to the ControlPlane SA

    **Cross-references:** Item 3.1 (workload identity webhook) provides the
    projected SA token volume for guest-side pods. Item 11.2 (ingress
    credential) is superseded by step 6 above.

- [ ] **3.4 Register `GCPPDCSIDriver` in HCCO**

    In `reconcileStorage()` (`resources.go:3517`), the platform switch creates
    `ClusterCSIDriver` CRs:

    ```go
    case hyperv1.AWSPlatform:
        driverNames = []operatorv1.CSIDriverName{operatorv1.AWSEBSCSIDriver}
    case hyperv1.AzurePlatform:
        driverNames = []operatorv1.CSIDriverName{operatorv1.AzureDiskCSIDriver, operatorv1.AzureFileCSIDriver}
    ```

    Add:
    ```go
    case hyperv1.GCPPlatform:
        driverNames = []operatorv1.CSIDriverName{operatorv1.GCPPDCSIDriver}
    ```

    The constant `operatorv1.GCPPDCSIDriver` (`"pd.csi.storage.gke.io"`) already
    exists in the vendor at
    `vendor/github.com/openshift/api/operator/v1/types_csi_cluster_driver.go:83`.

---

## 4. CSI / Storage

The CSI storage stack runs as a CPOv2 component (`v2/storage/`). The Cluster
Storage Operator (CSO) Deployment is the hub — it reads env vars to locate
CSI driver images, deploys the CSI driver operator and controller, and
manages the `ClusterCSIDriver` CR lifecycle.

GCP PD CSI driver images are already in the payload and partially wired:
`GCP_PD_DRIVER_OPERATOR_IMAGE` and `GCP_PD_DRIVER_IMAGE` map to
`gcp-pd-csi-driver-operator` and `gcp-pd-csi-driver` in `envreplace.go:17-18`.
The CSO knows how to deploy GCP PD CSI — the gap is that HyperShift doesn't
track its rollout or provide the control-plane-specific image variant.

- [ ] **4.1 Track CSI driver rollout status**

    **File:** `control-plane-operator/controllers/hostedcontrolplane/v2/storage/component.go:95`

    `checkOperandsRolloutStatus()` checks whether CSI driver Deployments have
    rolled out the expected image version. The switch (line 97) only handles
    AWS (lines 98-109, 2 deployments) and Azure (lines 111-133, 4 deployments).
    The `default` case (line 134) returns `true` unconditionally — meaning GCP
    CSI rollout is never verified.

    Add:
    ```go
    case hyperv1.GCPPlatform:
        operandsDeploymentsList = []operand{
            {
                DeploymentName:  "gcp-pd-csi-driver-operator",
                ContainerName:   "gcp-pd-csi-driver-operator",
                ReleaseImageKey: "gcp-pd-csi-driver-operator",
            },
            {
                DeploymentName:  "gcp-pd-csi-driver-controller",
                ContainerName:   "csi-driver",
                ReleaseImageKey: "gcp-pd-csi-driver",
            },
        }
    ```

    The deployment and container names must match what the CSO creates for GCP.
    Verify by checking the CSO repo's GCP PD deployment manifests.

- [ ] **4.2 Add `GCP_PD_DRIVER_CONTROL_PLANE_IMAGE` env var**

    **File:** `control-plane-operator/controllers/hostedcontrolplane/v2/storage/envreplace.go`

    The `operatorImageRefs` map (line 14) includes `_CONTROL_PLANE_IMAGE`
    entries for AWS (line 53), Azure disk/file (lines 54-55), and OpenStack
    (lines 56-57). These tell the CSO which image to use for CSI driver
    containers running on the management cluster (vs the data plane image
    running on worker nodes).

    **Missing:** `GCP_PD_DRIVER_CONTROL_PLANE_IMAGE`. Add:
    ```go
    "GCP_PD_DRIVER_CONTROL_PLANE_IMAGE": "gcp-pd-csi-driver",
    ```

    Without this, the CSO falls back to using the same image for both control
    plane and data plane, which may fail if the management cluster uses a
    different architecture or if image pull policies differ. The
    `setOperatorImageReferences()` method (line 80) uses
    `releaseImageProvider` (CP images) for non-data-plane refs and
    `userReleaseImageProvider` (guest images) for `_DRIVER_IMAGE` refs.
    The `_CONTROL_PLANE_IMAGE` env var must be resolved from the CP
    release image.

- [ ] **4.3 Evaluate GCP PD CSI PKI certificates**

    **Existing PKI for other platforms:**
    - AWS EBS CSI: `pki/aws_ebs_csi_driver_operator.go` (metrics serving cert),
      `pki/aws_ebs_csi_driver_controller.go` (metrics serving cert)
    - Azure Disk CSI: `pki/azure_disk_csi_driver_operator.go`,
      `pki/azure_disk_csi_driver_controller.go`
    - Azure File CSI: `pki/azure_file_csi_driver_operator.go`,
      `pki/azure_file_csi_driver_controller.go`

    These generate TLS certificates for **metrics serving** (HTTPS endpoints
    exposed by the CSI driver operator and controller for Prometheus scraping).
    Example pattern from `pki/aws_ebs_csi_driver_operator.go`:
    ```go
    func ReconcileAWSEBSCsiDriverOperatorMetricsServingCertSecret(secret, ca *corev1.Secret, ...) {
        dnsNames := []string{
            "aws-ebs-csi-driver-operator-metrics.<ns>.svc",
            "aws-ebs-csi-driver-operator-metrics.<ns>.svc.cluster.local",
            ...
        }
    }
    ```

    **Assessment:** GCP PD CSI will also need metrics serving certs if the
    CSI driver operator and controller expose metrics endpoints. Create:
    - `pki/gcp_pd_csi_driver_operator.go` — metrics serving cert for
      `gcp-pd-csi-driver-operator-metrics`
    - `pki/gcp_pd_csi_driver_controller.go` — metrics serving cert for
      `gcp-pd-csi-driver-controller-metrics`

    These certs are generated by CPO's PKI reconciler and mounted into the
    CSI pods. Without them, Prometheus scraping will fail or fall back to
    insecure HTTP.

    **Note:** This is only needed if the GCP PD CSI driver operator exposes
    a metrics endpoint. Check the CSO's GCP PD deployment to confirm.

    The PKI reconciler dispatches per platform in `reconcilePKI()` at
    `hostedcontrolplane_controller.go:1998`. Currently only AWS (line 1767,
    `reconcileAWSPlatformCerts`) and Azure (line 1820,
    `reconcileAzurePlatformCerts`) are handled — there is no
    `case hyperv1.GCPPlatform`.

- [ ] **4.4 Add GCP PD CSI to CVO exclusion list**

    **File:** `control-plane-operator/controllers/hostedcontrolplane/v2/cvo/deployment.go:277`

    `resourcesToRemove()` tells CVO which Deployments to delete from the guest
    cluster (because they run on the management cluster instead). The `default`
    case (which includes GCP) removes `cluster-storage-operator`,
    `csi-snapshot-controller-operator`, and AWS EBS CSI deployments — but does
    NOT include `gcp-pd-csi-driver-operator` or `gcp-pd-csi-driver-controller`.

    If the GCP PD CSI controller runs on the management cluster (which is the
    HyperShift pattern), these deployments must be excluded from the guest
    cluster to prevent CVO from deploying duplicate controllers. Add:
    ```
    {Namespace: "openshift-cluster-csi-drivers", Name: "gcp-pd-csi-driver-operator"},
    {Namespace: "openshift-cluster-csi-drivers", Name: "gcp-pd-csi-driver-controller"},
    ```

    **Note:** Verify against the CSO's GCP PD deployment naming before adding.

- [ ] **4.5 Add `GCP_PD_DRIVER_CONTROL_PLANE_IMAGE` to CSO deployment manifest**

    **File:** `control-plane-operator/controllers/hostedcontrolplane/v2/assets/cluster-storage-operator/deployment.yaml`

    The CSO deployment manifest lists `_CONTROL_PLANE_IMAGE` env vars for
    AWS, Azure, and OpenStack (lines 98-111) but not GCP PD. Add:
    ```yaml
    - name: GCP_PD_DRIVER_CONTROL_PLANE_IMAGE
    ```
    alongside the other `_CONTROL_PLANE_IMAGE` entries. The value will be
    populated by `envreplace.go` at runtime from the release image provider.

---

## 5. Cluster Lifecycle & Cleanup

Cluster deletion follows a multi-phase flow:

1. HC controller removes the CAPI Cluster CR, which triggers CAPG to delete
   GCP VMs and machine resources
2. HC controller calls `DeleteOrphanedMachines()` (if the platform implements
   `OrphanDeleter`) to clean up stuck machines
3. For GCP, `deleteGCPPrivateServiceConnect()` is called to remove PSC resources
   (`hostedcluster_controller.go:3842`)
4. HC controller calls `DeleteCredentials()` to clean up credentials
5. The finalizer is removed and the HC object is deleted

The CLI `destroy cluster gcp` command (`cmd/cluster/gcp/destroy.go`) is a
separate path — it deletes the HC CR and waits, but does NOT perform any
infra or IAM cleanup (passes `nil` for `DestroyPlatformSpecifics`).

Infra and IAM cleanup exist as standalone CLI commands:
- `cmd/infra/gcp/destroy_infra.go` — networking only (5 resource types)
- `cmd/infra/gcp/destroy_iam.go` — GSAs, WIF pool/provider

- [ ] **5.1 Implement `OrphanDeleter` interface**

    **Interface:** `hypershift-operator/controllers/hostedcluster/internal/platform/platform.go:82`
    ```go
    type OrphanDeleter interface {
        DeleteOrphanedMachines(ctx context.Context, c client.Client, hc *hyperv1.HostedCluster, controlPlaneNamespace string) error
    }
    ```

    **Caller:** `hostedcluster_controller.go:3791` — uses a type assertion:
    ```go
    if od, ok := p.(platform.OrphanDeleter); ok {
        if err = od.DeleteOrphanedMachines(ctx, r.Client, hc, controlPlaneNamespace); err != nil {
            return false, err
        }
    }
    ```
    Since GCP does not implement `OrphanDeleter`, this call is silently skipped.

    **AWS pattern** (`aws/aws.go:380`): Checks `GetCredentialStatus()` — if
    credentials are invalid, lists all `AWSMachine` objects in the CP namespace,
    and for any with a non-zero `DeletionTimestamp`, clears their finalizers so
    they can be garbage collected without calling the cloud.

    **Azure pattern** (`azure/azure.go:371`): More conservative — only runs for
    managed identity clusters, waits for a 10-minute `deletionFailedThreshold`,
    and checks for `Ready=False` with `Reason=DeletionFailed` before removing
    finalizers.

    **GCP implementation needed:** Add `DeleteOrphanedMachines()` to the `GCP`
    struct in `gcp/gcp.go`. The implementation should:
    1. Check if WIF credentials are valid (via `GetCredentialStatus()`)
    2. If invalid, list `GCPMachine` objects in the CP namespace
    3. For machines stuck in deletion, remove their finalizers

    Without this, if WIF credentials expire or are deleted during cluster
    teardown, `GCPMachine` CRs will be stuck indefinitely because CAPG cannot
    communicate with GCP to confirm VM deletion.

- [ ] **5.2 Wire `destroy cluster gcp` to infra/IAM cleanup**

    **File:** `cmd/cluster/gcp/destroy.go` (48 lines)

    Currently passes `nil` for `DestroyPlatformSpecifics`:
    ```go
    return core.DestroyCluster(ctx, hostedCluster, destroyOptions, nil)
    ```

    When `DestroyPlatformSpecifics` is `nil` (`cmd/cluster/core/destroy.go:121`):
    - No `openshift.io/destroy-cluster` finalizer is added to the HC
    - No platform-specific infrastructure cleanup runs
    - The command only deletes the HC CR and polls until it's gone

    **AWS comparison** (`cmd/cluster/aws/destroy.go:59`): Passes a real callback
    that calls `awsinfra.DestroyInfraOptions.Run()` (VPCs, subnets, ELBs, S3,
    instances, security groups, etc.) and `awsinfra.DestroyIAMOptions.Run()`
    (IAM roles/policies, OIDC provider).

    **Fix:** Create a `destroyPlatformSpecifics` function for GCP that calls:
    1. `gcpinfra.DestroyInfraOptions.Run()` — network cleanup
    2. `gcpinfra.DestroyIAMOptions.Run()` — WIF cleanup (unless `--preserve-iam`)

    Both destroy commands already exist as standalone CLI entry points — the gap
    is only the wiring.

- [ ] **5.3 Evaluate infra destroy completeness**

    **File:** `cmd/infra/gcp/destroy_infra.go` (108 lines)

    Cleans up exactly 5 networking resource types, deleted in reverse
    dependency order (`networking.go`):
    1. Cloud NAT (`DeleteNAT` at `networking.go:280`)
    2. Cloud Router (`DeleteRouter` at `networking.go:258`)
    3. Subnet (`DeleteSubnet` at `networking.go:236`)
    4. Firewall Rule (`DeleteFirewallRule` at `networking.go:446`)
    5. VPC Network (`DeleteNetwork` at `networking.go:214`)

    **Assessment:** This is **complete for CLI-created infrastructure**.
    `cmd/infra/gcp/create_infra.go` creates exactly these 5 resource types —
    so the destroy is symmetric.

    CAPG-managed resources (GCPMachine VMs, disks, load balancers) are cleaned
    up by CAPG during CAPI Cluster deletion, not by the CLI. PSC resources
    (endpoints, service attachments) are cleaned up by the operator via
    `deleteGCPPrivateServiceConnect()`.

    **Potential gaps for future evaluation:**
    - DNS records (if HyperShift creates Cloud DNS records outside of CAPG)
    - GCS buckets (if HyperShift creates storage for image registry)
    - Static external IPs (if allocated outside of CAPG)

    The severity is low because the operator handles most of these during HC
    deletion. The CLI destroy is primarily for when the operator cannot clean
    up (e.g., management cluster is gone).

- [ ] **5.4 Credential cleanup — `DeleteCredentials` is a no-op (by design)**

    **File:** `gcp/gcp.go:455`
    ```go
    func (p GCP) DeleteCredentials(...) error {
        // TODO: Implement GCP credential cleanup
        return nil
    }
    ```

    **This is the same as AWS and Azure** — `DeleteCredentials` returns `nil`
    on all three platforms:
    - AWS: `aws/aws.go:457` — returns `nil`
    - Azure: `azure/azure.go:358` — returns `nil`

    In-cluster credential Secrets are cleaned up automatically when the
    control plane namespace is deleted. WIF resources (GSAs, pool, provider,
    IAM bindings) are cloud-side resources cleaned up by the CLI command
    `cmd/infra/gcp/destroy_iam.go`, which handles:
    1. `DeleteServiceAccounts()` (`iam.go:1001`) — deletes GSAs and their
       project IAM role bindings
    2. `DeleteOIDCProvider()` (`iam.go:977`) — deletes the WIF pool provider
    3. `DeleteWorkloadIdentityPool()` (`iam.go:952`) — deletes the WIF pool

    **Action:** Remove the TODO comment and document that this is intentionally
    a no-op (matching AWS/Azure pattern). Cloud-side credential cleanup is
    handled by the CLI, not by the operator.

---

## 6. Monitoring & Alerting

HyperShift monitoring is centralized — not per-platform. The key pieces are:

1. **HostedCluster metrics collector** (`hypershift-operator/controllers/hostedcluster/metrics/metrics.go`)
   — Prometheus collector emitting ~20 per-cluster gauge metrics
2. **Expected conditions** (`support/conditions/conditions.go`) — per-platform
   condition list used by `hypershift_hostedclusters_failure_conditions` metric
3. **Transition duration histogram** (`metrics.go:358`) — tracks how long
   conditions take to become True after HC creation
4. **PrometheusRule resources** — recording rules and SLO alerts installed
   by `hypershift install`

No platform (AWS, Azure, or GCP) has controller-level Prometheus metrics in
the PrivateLink/PSC controllers themselves. The monitoring gap is primarily in
the condition tracking, not in raw metric instrumentation.

- [ ] **6.1 Add GCP conditions to `ExpectedHCConditions()`**

    **File:** `support/conditions/conditions.go`

    `ExpectedHCConditions()` returns a per-platform map of conditions that
    feed into the `hypershift_hostedclusters_failure_conditions` metric.

    AWS includes (lines 40-57): `ValidOIDCConfiguration`,
    `ValidAWSIdentityProvider`, `AWSDefaultSecurityGroupCreated`,
    `AWSEndpointAvailable`, `AWSEndpointServiceAvailable`, `ValidAWSKMSConfig`.

    GCP currently includes (lines 65-76): `ValidGCPWorkloadIdentity`,
    `ValidGCPCredentials` only.

    **Missing for GCP:** `GCPEndpointAvailable` and
    `GCPServiceAttachmentAvailable`. These conditions ARE set on the
    HostedCluster (via `computeGCPPSCCondition` at
    `hostedcluster_controller.go:970`) but are not tracked as expected
    conditions — so PSC failures won't show up in the failure metrics.

    Add these conditions to the GCP case in `ExpectedHCConditions()`, gated
    on `EndpointAccess != Public` (same pattern as AWS private endpoint
    conditions).

- [ ] **6.2 Add GCP conditions to transition duration tracking**

    **File:** `hypershift-operator/controllers/hostedcluster/metrics/metrics.go:358`

    `collectTransitionDurationMetrics()` tracks how long conditions take to
    become True. Current list:
    ```go
    []hyperv1.ConditionType{
        hyperv1.EtcdAvailable,
        hyperv1.InfrastructureReady,
        hyperv1.ExternalDNSReachable,
        hyperv1.AWSEndpointServiceAvailable,
        hyperv1.AWSEndpointAvailable,
    }
    ```

    **Missing:** `GCPEndpointAvailable` and `GCPServiceAttachmentAvailable`.
    Without these, GCP PSC setup latency is invisible to the SRE dashboard.

- [ ] **6.3 Add GCP-specific gauge metrics**

    AWS has `hypershift_cluster_invalid_aws_creds` (metrics.go:75) and Azure
    has `hosted_cluster_managed_azure_info` (metrics.go:92). GCP has none.

    Consider adding:
    - `hypershift_cluster_invalid_gcp_creds` — credential validity gauge
    - `hypershift_cluster_gcp_psc_status` — PSC endpoint availability gauge

    **Note:** The NodePool vCPU counting (`metrics.go:287`) only supports
    AWS today. Adding GCP vCPU counting from machine type specs would be
    a nice-to-have.

- [ ] **6.4 ~~Evaluate~~ SLO support on GKE management clusters — deferred**

    **Decision (2026-08-04): Accept the current limitation and align with
    Azure. SLO alerting on GKE management clusters is out of scope for GA.**

    **Rationale:** This is a management cluster infrastructure issue, not a
    GCP guest platform issue. Azure has the same skip. GKE management
    clusters lack OpenShift monitoring (`prometheus-k8s` in
    `openshift-monitoring`), which `alertSLOs()` at `e2e_test.go:279`
    depends on. Deploying standalone Prometheus on GKE or integrating GCP
    Cloud Monitoring would be significant work with no parity precedent
    (Azure hasn't done it either).

    **Current state (no change needed):**
    ```go
    if globalOpts.Platform == hyperv1.GCPPlatform {
        return fmt.Errorf("Alerting SLOs is not supported on GCP")
    }
    ```

    The SLO PrometheusRule (`cmd/install/assets/slos/hypershift.yaml`) is
    platform-agnostic and will work if/when GCP clusters are managed from
    an OCP management cluster instead of GKE. No code changes required.

- [ ] **6.5 Evaluate platform-specific alerting rules**

    **No platform** (AWS, Azure, or GCP) has PrometheusRule resources for
    private connectivity health (PrivateLink / PSC). The existing alerting
    rules are:
    - `cmd/install/assets/slos/hypershift.yaml` — time-to-availability SLO
    - `cmd/install/assets/recordingrules/hypershift.yaml` — 16 recording rules
    - Guest-cluster alerts in HCCO (`alerts/assets/`) — API deprecation,
      pod security violations

    Creating GCP PSC alerting rules would be first-of-its-kind. Consider
    alerting on `hypershift_hostedclusters_failure_conditions` with
    `condition=GCPEndpointAvailable` once item 6.1 is implemented.

---

## 7. Testing — Unit & CEL Validation

### GCP test file inventory

| Area | File | Lines | Tests |
|------|------|-------|-------|
| HO NodePool | `controllers/nodepool/gcp_test.go` | 1,018 | 5 |
| HO HC | `controllers/hostedcluster/gcp_oidc_test.go` | 444 | 2 |
| HO platform | `internal/platform/gcp/gcp_test.go` | 418 | 10 |
| HO conditions | `internal/platform/gcp/gcp_conditions_test.go` | 346 | 4 |
| HO PSC | `controllers/platform/gcp/privateserviceconnect_controller_test.go` | 600 | 21 |
| CPO PSC | `controllers/gcpprivateserviceconnect/psc_endpoint_controller_test.go` | 905 | 18 |
| CPO DNS | `controllers/gcpprivateserviceconnect/dns_test.go` | 562 | — |
| CPO observer | `controllers/gcpprivateserviceconnect/observer_test.go` | 322 | — |
| CPO CCM | `v2/cloud_controller_manager/gcp/config_test.go` | 167 | — |
| HCCO | `controllers/resources/gcp/gcp_test.go` | 121 | 1 |
| Support | `gcputil/gcputil_test.go` | 100 | — |
| Envtest | `techpreview.hostedclusters.gcp.testsuite.yaml` | 1,959 | 24 |

Total: ~6,962 lines across 12 test files.

- [ ] **7.1 Create GCP NodePool envtest CEL suite**

    No GCP NodePool CEL test file exists. AWS has 24 test cases in
    `stable.nodepools.aws.testsuite.yaml` (668 lines), Azure has its own
    suite. GCP has zero.

    The GCP HostedCluster suite exists at
    `tests/hostedclusters.hypershift.openshift.io/techpreview.hostedclusters.gcp.testsuite.yaml`
    (1,959 lines, 24 cases) — but the NodePool CRD has its own CEL rules
    that need independent coverage.

    **CEL validations in GCP NodePool types** (from `gcp.go`):

    1. `GCPNodePoolPlatform` (line 392): cross-field rule —
       `onHostMaintenance must be TERMINATE when provisioningModel is Spot or Preemptible`
    2. `MachineType` (line 406): regex — must start/end with lowercase
       letter or digit, only lowercase letters, digits, hyphens
    3. `Zone` (line 417): format — must match `region-zone`
       (e.g., `us-central1-a`)
    4. `GCPResourceLabel.Key` (lines 33-34): regex + reserved prefix
       (`goog` prefix disallowed)
    5. `GCPResourceLabel.Value` (line 45): regex — empty or lowercase
       letter start
    6. `nodepool_types.go:106`: arch enum — `arm64` supported on GCP

    **Create:** `techpreview.nodepools.gcp.testsuite.yaml` with
    `featureGates: [GCPPlatform]` in the header (will become `stable.*`
    after feature gate graduation in Section 1).

    Target: ~12-15 test cases (positive + negative for each validation).

- [ ] **7.2 Generate GCS mock**

    **File:** `support/gcpapi/gcs.go:8`
    ```go
    //go:generate ../../hack/tools/bin/mockgen -source=gcs.go -package=gcpapi -destination=gcs_mock.go
    ```

    The `GCSAPI` interface (line 11) has 2 methods: `UploadObject` and
    `DeleteObject`. The expected output `gcs_mock.go` does **not exist**.
    The directory contains only `gcs.go` and `gcs_client.go`.

    **Consumer:** `hostedcluster_controller.go:193` has a `GCSClient gcpapi.GCSAPI`
    field. The OIDC test at `gcp_oidc_test.go:36` uses a hand-written
    `fakeGCSClient` instead of the generated mock.

    **Fix:** Run `go generate ./support/gcpapi/` to create the mock.
    Optionally update `gcp_oidc_test.go` to use the generated mock.

- [ ] **7.3 Add CPO PSC endpoint controller interface abstraction**

    The PSC controller at
    `control-plane-operator/controllers/gcpprivateserviceconnect/psc_endpoint_controller.go`
    (1,064 lines) uses concrete `*compute.Service` and `*dns.Service`
    clients throughout. Key methods accepting concrete types:
    - `ensureIPAddress(... customerGCPClient *compute.Service ...)` (line 567)
    - `reconcilePSCEndpoint(... customerGCPClient *compute.Service ...)` (line 669)
    - `reconcileDelete(... customerGCPClient *compute.Service ...)` (line 765)
    - `dns.go:45`: `newDNSClient` returns `*dns.Service`

    The test file (905 lines, 18 functions) can only test helper/utility
    functions — **core reconciliation cannot be unit tested** because it
    requires live GCP API calls.

    **AWS comparison:** The PrivateLink controller
    (`awsprivatelink/awsprivatelink_controller.go:237`) defines an
    `awsClientProvider` interface with `//go:generate mockgen`. Its test
    file is **2,115 lines** with 20 functions covering full reconciliation
    paths via mocks.

    **What to change:**
    1. Define `gcpComputeAPI` interface covering: `Addresses.Get/Insert/Delete`,
       `ForwardingRules.Get/Insert/Delete`
    2. Define `gcpDNSAPI` interface covering DNS operations
    3. Add `//go:generate mockgen` directives
    4. Refactor `gcpClientBuilder.getClient()` to return the interface
    5. Update method signatures from `*compute.Service` to the interface

    This is a significant refactoring effort — consider scoping to
    post-GA if timeline is tight.

- [ ] **7.4 Expand nodepool unit tests**

    GCP: 1,018 lines / 5 test functions. AWS: 2,020 lines / 12 test functions.

    **GCP test functions** (`controllers/nodepool/gcp_test.go`):
    1. `TestGcpMachineTemplateSpec` (line 21)
    2. `TestDefaultNodePoolGCPImage` (line 535)
    3. `TestConfigureGCPMaintenanceBehavior` (line 725)
    4. `TestConfigureGCPNetworkTags` (line 774)
    5. `TestGcpMachineTemplateSpecWithRHELStream` (line 836)

    **Missing scenarios** (covered by AWS but not GCP):
    - Platform config validation (`TestValidateAWSPlatformConfig`)
    - Condition setting (`TestSetAWSConditions`)
    - Boot disk configuration (`TestBuildAWSRootVolume`)
    - Subnet configuration (`TestBuildAWSSubnet`)
    - Service account configuration
    - Provisioning model / spot/preemptible (`TestIsSpotEnabled`)
    - Resource label propagation

- [ ] **7.5 Add DNS module interface for PSC controller**

    `dns.go` (609 lines) in the PSC controller package uses `*dns.Service`
    directly with no interface. `dns_test.go` (562 lines) only tests
    helper/formatting functions — `ReconcileDNS()` and zone management
    functions cannot be unit-tested.

    Same approach as item 7.3: define a `gcpDNSAPI` interface, generate
    mocks, refactor method signatures.

---

## 8. Testing — E2E

The v2 E2E framework (`test/e2e/v2/`) uses a `PlatformConfig` interface
(`lifecycle/platform.go:50`) with 11 methods. Each platform provides a
`gcp.go`/`aws.go`/`azure.go` that defines cluster variants and a test matrix.

**Existing GCP e2e coverage** (v2 tests with platform guards — all
require a GCP lifecycle PlatformConfig to actually run):
- PSC test: `v2/tests/hosted_cluster_psc_test.go` — label `gcp-psc`
- CCM test: `v2/tests/hosted_cluster_ccm_test.go` — label `hosted-cluster-ccm`
- Image Registry: `v2/tests/hosted_cluster_image_registry_test.go:138` — GCP-specific
- Workload registry: `gcp-cloud-controller-manager` registered at
  `v2/internal/workload_registry.go:399`

The v1 framework has GCP CLI flags (`e2e_test.go:169`, 20 flags) and
fixture support (`util/fixture.go:191`), but no GCP-specific test files.

**CI status:** Only 2 GKE periodic jobs exist in `.chai-bot/ci-status-jobs.yaml`
(lines 51, 54) — these test GKE-as-management-cluster, not
GCP-as-guest-platform. **No GCP platform CI jobs exist.** AWS has ~40,
Azure ~15.

- [ ] **8.1 Add v2 lifecycle PlatformConfig for GCP**

    **File to create:** `test/e2e/v2/lifecycle/gcp.go`

    **Interface:** `PlatformConfig` at `platform.go:50` — 11 methods:
    `Name()`, `DefaultBaseDomain()`, `ClusterSpecs()`, `CreateArgs()`,
    `PreCreate()`, `PostCreate()`, `PostAvailable()`,
    `PostVersionRollout()`, `TestMatrix()`, `SetupTestEnv()`,
    `DestroyArgs()`

    **Wire:** Add `case "gcp":` to `NewPlatformConfig()` at
    `platform.go:103`.

    **AWS** (`aws.go`, 122 lines): 1 cluster variant ("public"), 1 test
    matrix group.

    **Azure** (`azure.go`, 420 lines): 6 cluster variants (public,
    private, oauth-lb, upgrade, autoscaling, external-oidc), 5 parallel
    groups + 1 sequential group.

    **GCP recommended variants:**
    - `public` — basic lifecycle with `--endpoint-access=PublicAndPrivate`
    - `private` — PSC-based with `--endpoint-access=Private` (the default
      for GCP per `e2e_test.go:183`)
    - `upgrade` — N-1 control plane image for upgrade testing

    **GCP recommended test matrix groups:**
    - Parallel: `hosted-cluster-ccm`, `gcp-psc`,
      `hosted-cluster-image-registry`, `nodepool-autoscaling`,
      `control-plane-workloads`
    - Sequential: `control-plane-upgrade` (on the upgrade variant)

    This is the **keystone item** — without it, no GCP e2e test can run in
    the v2 framework.

- [ ] **8.2 Add control plane upgrade test for GCP**

    The v2 test already exists and is **platform-agnostic**:
    `v2/tests/control_plane_upgrade_test.go` — label `control-plane-upgrade`.
    No platform skip.

    **Fix:** Add an `"upgrade"` cluster variant to the GCP `PlatformConfig`
    (item 8.1) with N-1 release image and HA replicas, following the
    Azure pattern (`azure.go:138`). Wire `control-plane-upgrade` into
    the `TestMatrix()` as a sequential group.

    No new test code is needed — just CI wiring.

- [ ] **8.3 Add HyperShift operator upgrade test for GCP**

    The v1 test (`upgrade_hypershift_operator_test.go`) is **AWS-specific**
    — it uses AWS zones and IAM roles. No v2 equivalent exists.

    **Options:**
    - (a) Port the v1 test to be platform-agnostic (significant effort)
    - (b) Create a GCP-specific v2 HO upgrade test
    - (c) Defer to the existing `e2e-aws-upgrade-hypershift-operator` CI
      job since the HO upgrade is mostly platform-agnostic (the operator
      binary doesn't change per platform)

    **Assessment:** The HO upgrade test primarily validates that upgrading
    the operator binary doesn't break existing clusters. The platform-
    specific part is only the cluster creation. If the GCP `PlatformConfig`
    (item 8.1) is implemented, the test infrastructure is in place.

- [ ] **8.4 Enable NodePool rolling upgrade for GCP**

    **File:** `v2/tests/nodepool_lifecycle_test.go:512`
    ```go
    Skip("rolling upgrade test only supported on AWS and Azure platforms")
    ```

    The test changes the NodePool machine type (e.g., `m5.xlarge` →
    `m5.2xlarge`) and verifies machines are replaced. For GCP:
    - Change machine type: e.g., `n2-standard-4` → `n2-standard-8`
    - Verify machines: list `GCPMachine` objects post-upgrade

    **What to change:**
    1. Add `case hyperv1.GCPPlatform:` to the machine type mutation logic
    2. Add GCP machine verification (list `GCPMachine` objects from CAPG)
    3. Remove GCP from the skip guard

    The v2 test has a TODO at line 594 about verifying machine specs
    post-upgrade.

- [ ] **8.5 Add autoscaling test for GCP**

    The v2 test already exists and is **platform-agnostic**:
    `v2/tests/nodepool_autoscaling_test.go` — label `nodepool-autoscaling`.
    `AutoscalingScaleUpDownTest` and `AutoscalingBalancingTest` deep-copy
    from the default NodePool, preserving GCP platform config. No platform
    skip.

    **Fix:** Add an `"autoscaling"` cluster variant to the GCP
    `PlatformConfig` (item 8.1) and wire `nodepool-autoscaling` into the
    `TestMatrix()`, following the Azure pattern (`azure.go:146`, line 355).

    No new test code is needed — just CI wiring.

- [ ] **8.6 Add node auto-repair test for GCP**

    **File:** `v2/tests/nodepool_lifecycle_test.go:1185`

    The v2 test is **fully skipped** with two guards:
    1. Line 1187: `Skip("auto-repair instance termination not yet implemented for v2 framework")`
    2. Line 1194: guards to AWS and Azure only

    The v1 test (`nodepool_autorepair_test.go:40`) terminates EC2
    instances via the AWS SDK and waits for MHC-driven replacement.

    **What to implement:**
    - GCP instance termination via Compute Engine API
      (`instances.delete`) — the v2 test has a TODO at lines 1220-1225
    - Add `case hyperv1.GCPPlatform:` to the platform guard
    - Need GCP Compute client setup in the test framework

- [ ] **8.7 Add private cluster test for GCP**

    The v2 PSC test already exists:
    `v2/tests/hosted_cluster_psc_test.go` — validates the
    `GCPPrivateServiceConnect` CR, forwarding rule, NAT subnet, and
    `GCPServiceAttachmentAvailable` condition. Label: `gcp-psc`.

    **Fix:** Add a `"private"` cluster variant to the GCP `PlatformConfig`
    with `--endpoint-access=Private`. Wire the `gcp-psc` label into the
    `TestMatrix()`.

    Note: `Private` is actually the **default** for GCP endpoint access
    (per the CLI flag default at `e2e_test.go:183`).

- [ ] **8.8 Add preemptible/spot instance test for GCP**

    **No test exists.** The AWS spot test
    (`nodepool_spot_termination_handler_test.go`) validates SQS-based
    termination handling, which is AWS-specific.

    GCP preemptible/spot VMs use a different mechanism (metadata server
    maintenance event signal). A GCP-specific test should:
    1. Create a NodePool with `Preemptible: true` in the GCP platform spec
    2. Verify instances are correctly marked as preemptible via the
       Compute Engine API
    3. Optionally validate termination handler behavior

- [ ] **8.9 Add GCP resource tagging day-2 test for GCP**

    **No test exists.** The AWS day-2 tags test
    (`nodepool_day2_tags_test.go`) adds tags to the NodePool spec, then
    verifies via the EC2 API that tags appear on instances without
    triggering a rolling upgrade.

    A GCP equivalent should:
    1. Add `ResourceTags` to the GCP NodePool spec (via `GCPResourceTag`)
    2. Verify labels appear on GCE instances via the Compute Engine API
    3. Verify no rolling upgrade occurred (replicas unchanged)

---

## 9. CI

CI job definitions live in the external `openshift/release` repository, not in this repo.
The `.chai-bot/ci-status-jobs.yaml` file in this repo controls which periodic jobs are
tracked on the CI status dashboard.

**Current GCP CI footprint vs other platforms:**

| Category | AWS | Azure | GCP |
|----------|-----|-------|-----|
| Platform e2e jobs (ci-status-jobs) | ~38 across 3 OCP versions | ~24 across 3 OCP versions | **2** (GKE-as-management only) |
| Upgrade jobs | 5+ (`e2e-aws-upgrade` per version + HCM upgrade) | 4+ (`aks-upgrade-from-zero`, `aks-upgrade-minor`) | **0** |
| Conformance jobs | FIPS, serial, proxy, Cilium, Calico variants | conformance, serial | **0** |
| Private cluster jobs | `calico-private`, `cilium-private` | via AKS | **0** |

- [ ] **9.1 Add GCP-as-guest-platform e2e jobs to `openshift/release`**

    The 2 existing GCP jobs in `.chai-bot/ci-status-jobs.yaml` (lines 51, 54)
    are both GKE-as-management-cluster tests (`e2e-v2-gke`), categorized under
    "HCM & Other" (line 40). These test HyperShift running *on* GKE with AWS
    guest clusters — they do NOT test GCP as the hosted cluster platform.

    **No job exists that creates a HostedCluster with `platform.type: GCP`.**

    Jobs needed in `openshift/release` (follow AWS patterns):
    1. `e2e-gcp-ovn` — basic lifecycle (create, verify, destroy)
    2. `e2e-gcp-ovn-conformance` — OCP conformance suite on GCP guest
    3. `e2e-gcp-upgrade` — upgrade test (blocks any PR touching config
       hashes; see `CLAUDE.md` "Fleet-Wide Rollout Impact")
    4. `e2e-gcp-private` — Private/PSC topology test

    **Prerequisite:** Section 8.1 (v2 E2E `PlatformConfig` for GCP) must be
    implemented first — without it, no GCP lifecycle test can run.

- [ ] **9.2 Add GCP jobs to `.chai-bot/ci-status-jobs.yaml`**

    **File:** `.chai-bot/ci-status-jobs.yaml`

    Once jobs are created in `openshift/release`, add a dedicated GCP category
    block (following the AWS/Azure pattern):

    ```yaml
    - name: "GCP (5.0)"
      description: "GCP hosted control planes — OCP 5.0"
      jobs:
        - periodic-ci-openshift-hypershift-release-5.0-periodics-e2e-gcp-ovn
        - periodic-ci-openshift-hypershift-release-5.0-periodics-e2e-gcp-upgrade
    ```

    The existing GKE jobs (lines 51, 54) should remain in "HCM & Other" since
    they test a different configuration (GKE management, not GCP guest).

- [ ] **9.3 Verify no GCP-specific Makefile targets are needed**

    **File:** `Makefile`

    GCP references in the Makefile are limited to CRD generation (lines
    313-316, `cluster-api-provider-gcp` target). No platform has dedicated
    e2e Makefile targets — `e2e` (line 515) and `e2ev2-run-tests` (line 533)
    are generic binaries, with platform selection happening via CLI flags at
    runtime. **No Makefile changes are needed** — this matches the AWS/Azure
    pattern.

---

## 10. Documentation

GCP currently has 7 docs in `docs/content/how-to/gcp/` (index, setup, infra,
IAM, cluster creation, image registry, CI job). AWS has 14+ files and Azure
has 14+ files. GCP is the **only platform** in `docs/mkdocs.yml` nav that
lacks both a "Global Pull Secret" entry and a "Troubleshooting" section.

**Documentation gap analysis:**

| Document type | AWS | Azure | GCP |
|--------------|-----|-------|-----|
| Troubleshooting guide | `troubleshooting/` (3 files) | `troubleshooting/` (2 files) | **missing** |
| Disaster recovery | `disaster-recovery.md` (34KB) | `backup-and-restore-etcd-snapshot.md` | **missing** |
| Private cluster guide | `deploy-aws-private-clusters.md` | `deploy-azure-private-clusters.md` (20KB) | **missing** |
| Global pull secret | symlink → `common/global-pull-secret.md` | symlink → `common/global-pull-secret.md` | **missing** |
| Architecture reference | `reference/architecture/aws/privatelink.md` (6KB) | `reference/architecture/azure/privatelink.md` (9KB) | **missing** |

- [ ] **10.1 Create GCP troubleshooting guide**

    **Create:** `docs/content/how-to/gcp/troubleshooting/index.md` and
    `docs/content/how-to/gcp/troubleshooting/debug-nodes.md`

    Follow the AWS pattern (`how-to/aws/troubleshooting/`, 3 files) and Azure
    pattern (`how-to/azure/troubleshooting/`, 2 files). GCP-specific topics:
    1. PSC endpoint connectivity issues (forwarding rules, NAT subnet
       exhaustion)
    2. WIF credential validation (token-minter sidecar logs, audience
       mismatch)
    3. GCE instance boot failures (custom image, service account
       permissions)
    4. Node join failures (firewall rules, network tags)

    **Update `docs/mkdocs.yml`** (after line 203) to add:
    ```yaml
      - 'Troubleshooting':
        - how-to/gcp/troubleshooting/index.md
        - how-to/gcp/troubleshooting/debug-nodes.md
    ```

- [ ] **10.2 Create GCP disaster recovery guide**

    **Create:** `docs/content/how-to/gcp/disaster-recovery.md` or link to
    the shared platform-agnostic disaster recovery docs at
    `docs/content/how-to/disaster-recovery/` (9 files including OADP backup,
    etcd recovery, DR CLI).

    AWS has a full platform-specific DR guide (`disaster-recovery.md`, 34KB)
    plus a troubleshooting addendum. Azure has
    `backup-and-restore-etcd-snapshot.md`. GCP should at minimum document:
    1. How etcd backup/restore works with GCP-hosted control planes
    2. Any GCP-specific considerations (e.g., PD snapshot lifecycle, WIF
       credential restoration)
    3. Link to the shared OADP-based DR flow

- [ ] **10.3 Create GCP private cluster deployment guide**

    **Create:** `docs/content/how-to/gcp/deploy-gcp-private-clusters.md`

    GCP supports `Private` and `PublicAndPrivate` endpoint access (validated
    by CEL in the HostedCluster CRD). The PSC-based private topology is
    fundamentally different from AWS PrivateLink and Azure Private Link
    Service. Document:
    1. PSC forwarding rule creation and NAT subnet requirements
    2. DNS configuration for private API access
    3. `EndpointAccess` enum values and their network topology implications
    4. Cross-reference the PSC controller
       (`controllers/gcpprivateserviceconnect/`)

    **Update `docs/mkdocs.yml`** to add the page to GCP nav.

- [ ] **10.4 Add GCP global pull secret documentation**

    **Create:** `docs/content/how-to/gcp/global-pull-secret.md` as a symlink
    to `../common/global-pull-secret.md`

    The common doc already exists at `how-to/common/global-pull-secret.md` and
    is shared by 6 other platforms via symlinks (AWS line 170 in mkdocs.yml,
    Azure line 179, Agent line 158, KubeVirt line 208, None line 218,
    OpenStack line 225). GCP is the only platform without this entry.

    ```bash
    cd docs/content/how-to/gcp/
    ln -s ../common/global-pull-secret.md global-pull-secret.md
    ```

    **Update `docs/mkdocs.yml`** (after line 203) to add:
    ```yaml
      - 'Global Pull Secret': how-to/gcp/global-pull-secret.md
    ```

- [ ] **10.5 Create GCP PSC architecture reference**

    **Create:** `docs/content/reference/architecture/gcp/psc.md`

    AWS has `reference/architecture/aws/privatelink.md` (6KB) and Azure has
    `reference/architecture/azure/privatelink.md` (9KB). No GCP directory or
    file exists under `reference/architecture/`. Document:
    1. PSC forwarding rule → service attachment → KAS/OAuth flow
    2. NAT subnet allocation and PSC connection limits
    3. Comparison with AWS PrivateLink / Azure PLS topology
    4. How the PSC controller (`gcpprivateserviceconnect/
       psc_endpoint_controller.go`, 1,064 lines) manages the lifecycle

    **Update `docs/mkdocs.yml`** nav to add the architecture page.

- [ ] **10.6 Create GCP infrastructure reference**

    **Create:** `docs/content/reference/infrastructure/gcp.md`

    `docs/content/reference/infrastructure/` has entries for AWS (`aws.md`),
    Azure (`azure-aro-hcp.md`, `azure-self-managed.md`), and Agent
    (`agent.md`) but no GCP entry. Document GCP-specific infrastructure
    components: VPC/subnet layout, Cloud NAT, firewall rules, PSC
    networking, and WIF IAM resources.

    **Update `docs/mkdocs.yml`** nav to add the infrastructure page.

---

## 11. OCP Release Payload / CVO Integration

The following payload images are already wired and functional: `gcp-cloud-controller-manager`,
`cluster-api-provider-gcp`, `gcp-pd-csi-driver`, `gcp-pd-csi-driver-operator`,
`cloud-network-config-controller`. No MCO or installer artifact dependencies exist.

CCO is not needed for GCP. HyperShift does not use CCO to provision guest-side credentials on
any platform — HCCO handles this directly via `reconcileCloudCredentialSecrets()` in
`resources.go`, which calls each platform's `SetupOperandCredentials()`. CCO is only deployed
on AWS (gated by `isAWSPlatform` in `v2/cloud_credential_operator/component.go`) for
control-plane-side `CredentialsRequest` reconciliation. GCP bypasses CCO entirely with WIF.

HCCO guest-side credential status by platform:

| Operator | Guest namespace | AWS | Azure | GCP |
|----------|----------------|-----|-------|-----|
| Image Registry | `openshift-image-registry` | IRSA | WI | WIF |
| Storage/CSI | `openshift-cluster-csi-drivers` | IRSA | WI | **missing** |
| Ingress | `openshift-ingress-operator` | IRSA | WI | **missing** |

GCP's `SetupOperandCredentials()` in `gcp/gcp.go` uses an extensible `configs` slice pattern.
The `Storage` and `Network` SAs already exist in `GCPServiceAccountsEmails`; adding entries
for CSI and ingress is straightforward.

- [ ] **11.1 Add storage/CSI guest-side credential**

    **Files to modify:**

    1. `control-plane-operator/hostedclusterconfigoperator/controllers/resources/
       manifests/creds.go` — Add `GCPStorageCloudCredsSecret()` function:

        ```go
        func GCPStorageCloudCredsSecret() *corev1.Secret {
            return &corev1.Secret{
                ObjectMeta: metav1.ObjectMeta{
                    Namespace: "openshift-cluster-csi-drivers",
                    Name:      "gcp-pd-cloud-credentials",
                },
            }
        }
        ```

        This follows the exact pattern of `AWSStorageCloudCredsSecret()`
        (line 26, targeting `ebs-cloud-credentials`) and
        `AzureDiskCSICloudCredsSecret()` (line 55, targeting
        `azure-disk-credentials`). GCP currently has only
        `GCPImageRegistryCloudCredsSecret()` (line 75).

    2. `control-plane-operator/hostedclusterconfigoperator/controllers/resources/
       gcp/gcp.go` — Add entry to the `configs` slice in
       `SetupOperandCredentials()` (after line 42):

        ```go
        {
            manifestFunc:        manifests.GCPStorageCloudCredsSecret,
            serviceAccountEmail: string(hcp.Spec.Platform.GCP.WorkloadIdentity.
                                   ServiceAccountsEmails.Storage),
            errorContext:        "guest cluster storage/CSI credential",
        },
        ```

        The `Storage` SA already exists in `GCPServiceAccountsEmails`
        (`api/hypershift/v1beta1/gcp.go:344`) with roles
        `compute.storageAdmin`, `compute.instanceAdmin.v1`,
        `iam.serviceAccountUser`, and `resourcemanager.tagUser`. No
        `capabilityChecker` is needed — storage is always enabled (matches
        Azure's `azureDisk` pattern at `azure/azure.go:107`).

    **Credential flow:** `reconcileCloudCredentialSecrets()` at
    `resources.go:2317` dispatches to `gcpresources.SetupOperandCredentials()`,
    which iterates the `configs` slice and calls
    `gcputil.BuildWorkloadIdentityCredentials()` to generate a WIF External
    Account Credential JSON, then upserts it as `service_account.json` in the
    target Secret.

    **Cross-reference:** Section 4 (CSI/Storage) — the CSI driver operator
    reads this credential from the guest namespace.

- [ ] **11.2 Add ingress guest-side credential**

    **Decision (2026-08-04): Resolved — covered by item 3.3 (dedicated
    Ingress SA), step 6.** A new `Ingress` SA will be added to
    `GCPServiceAccountsEmails` with `roles/dns.admin` and
    `roles/compute.viewer`. The guest-side credential
    (`openshift-ingress-operator/cloud-credentials`) will be created by
    HCCO's `SetupOperandCredentials()` using the `Ingress` SA email.

    See item 3.3 for full implementation details.

- [ ] **11.3 Register `GCPPDCSIDriver` in HCCO `reconcileStorage`**

    **File:** `control-plane-operator/hostedclusterconfigoperator/controllers/
    resources/resources.go`

    The `reconcileStorage` function's platform switch at line 3517 creates
    `ClusterCSIDriver` CRs for each platform's CSI drivers. GCP is missing:

    ```go
    // Current (line 3517-3533):
    var driverNames []operatorv1.CSIDriverName
    switch hcp.Spec.Platform.Type {
    case hyperv1.AWSPlatform:
        driverNames = []operatorv1.CSIDriverName{operatorv1.AWSEBSCSIDriver}
    case hyperv1.OpenStackPlatform:
        driverNames = []operatorv1.CSIDriverName{
            operatorv1.CinderCSIDriver, operatorv1.ManilaCSIDriver,
        }
    case hyperv1.AzurePlatform:
        if !azureutil.IsAroHCPByHCP(hcp) {
            driverNames = []operatorv1.CSIDriverName{
                operatorv1.AzureDiskCSIDriver, operatorv1.AzureFileCSIDriver,
            }
        }
    }
    ```

    **Add** before the closing `}`:
    ```go
    case hyperv1.GCPPlatform:
        driverNames = []operatorv1.CSIDriverName{operatorv1.GCPPDCSIDriver}
    ```

    The constant `operatorv1.GCPPDCSIDriver` (`"pd.csi.storage.gke.io"`)
    exists in the vendor at `vendor/github.com/openshift/api/operator/v1/
    types_csi_cluster_driver.go:83`.

    Without this, the guest cluster has no `ClusterCSIDriver` CR for GCP PD,
    which means the CSI driver operator cannot reconcile storage in the guest.

    **Cross-reference:** Section 4.1 (CSI rollout status) — the management-
    side `checkOperandsRolloutStatus()` at `v2/storage/component.go:95` also
    needs a GCP case (currently falls through to `default: return true`).

- [ ] **11.4 Add GCP PD CSI deployments to CVO exclusion list**

    **File:** `control-plane-operator/controllers/hostedcontrolplane/v2/cvo/
    deployment.go`

    The `resourcesToRemove()` function (line 277) tells CVO which guest-side
    deployments to skip because HyperShift manages them on the management
    cluster. The `default` case (line 287-304) currently excludes AWS EBS
    CSI driver deployments (lines 298-299):

    ```go
    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
        Name: "aws-ebs-csi-driver-operator",
        Namespace: "openshift-cluster-csi-drivers"}},
    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
        Name: "aws-ebs-csi-driver-controller",
        Namespace: "openshift-cluster-csi-drivers"}},
    ```

    **Problem:** These AWS-specific exclusions are in the `default` case,
    which runs for ALL platforms (including GCP). But the GCP equivalents
    are NOT listed. Without exclusion, CVO will attempt to deploy
    `gcp-pd-csi-driver-operator` and `gcp-pd-csi-driver-controller` inside
    the guest cluster, conflicting with the management-side CSO deployment.

    **Two options:**
    1. **Add GCP entries to the `default` case** (alongside AWS):
       ```go
       &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
           Name: "gcp-pd-csi-driver-operator",
           Namespace: "openshift-cluster-csi-drivers"}},
       &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
           Name: "gcp-pd-csi-driver-controller",
           Namespace: "openshift-cluster-csi-drivers"}},
       ```
    2. **Refactor to per-platform cases** — move AWS entries to an
       `AWSPlatform` case and add a `GCPPlatform` case (cleaner but larger
       change)

    **Cross-reference:** Section 4.4 tracks this same item from the storage
    perspective.
