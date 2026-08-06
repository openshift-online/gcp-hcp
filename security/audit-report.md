# Security Audit Report

**Repository:** gcp-hcp  
**Date:** 2026-06-25  
**Scan Mode:** Full Project Audit  
**Tech Stack:** Go, Python, Shell, Kubernetes, ArgoCD, Tekton, Konflux, Docker, GCP  
**Files Reviewed:** 187  
**Domains Analyzed:** Application (SAST), Containers, Kubernetes, CI/CD, Secrets, Supply Chain, Cloud Native, Auth, Infrastructure, Git

---

## Executive Summary

This report presents a full-spectrum static security audit of the gcp-hcp repository. The scan covered 69 Go files, 29 Python files, 28 shell scripts, 53 YAML manifests, 4 Dockerfiles, and associated configuration files across 10 security domains.

The repository's **production-grade infrastructure** (bootstrap/ArgoCD, ExternalSecrets with GCP Secret Manager via Workload Identity Federation) follows good security practices. However, the **experiments/** directory -- which contains prototype code, demo scripts, and pipeline automation -- has significant security gaps typical of rapidly iterated experimental code.

**No hardcoded real secrets were found anywhere in the repository.**

| Severity | Count |
|----------|-------|
| CRITICAL | 8 |
| HIGH | 20 |
| MEDIUM | 25 |
| LOW | 19 |
| **Total** | **72** |

**Top 3 priorities:**
1. Pin the Konflux release pipeline image and task revision (C-07, C-08) -- active supply chain risk
2. Restrict the kube-applier wildcard RBAC ClusterRole (C-05) -- cluster-admin equivalent
3. Add credential file patterns to root `.gitignore` (M-25) -- prevents accidental secret commits

---

## Findings

### CRITICAL

**[C-01] Pervasive subprocess shell=True with Unsanitized Config Values**
- **File:** `experiments/ho-platform-none/install-ho-platform-none/common.py:174`
- **Category:** Application - Command Injection
- **Issue:** The `CommandRunner.run()` method uses `shell=True` for all commands. Every installer step passes environment-sourced config values (PROJECT_ID, ZONE, WORKER_NODE_NAME, etc.) via f-strings into shell command strings without escaping.
- **Impact:** If any config value contains shell metacharacters, arbitrary commands execute as the running user.
- **Remediation:** Use `subprocess.run()` with list arguments. If shell features are needed, validate inputs with `shlex.quote()`. Add character-set validation in `InstallConfig.__post_init__()`.

**[C-02] JWT Decoded Without Signature Verification**
- **File:** `experiments/auth/phase2-poc/cloud-function/main.py:172-180`
- **Category:** Auth - Broken Authentication
- **Issue:** `decode_jwt_claims()` splits the JWT on "." and base64-decodes the payload without cryptographic verification. The extracted `email` claim (line 238) is used for IAM role lookups and Kubernetes group membership decisions.
- **Impact:** An attacker could craft a JWT with arbitrary claims (any email) to impersonate users and gain elevated Kubernetes access (e.g., cluster-admin).
- **Remediation:** Use `google.oauth2.id_token.verify_oauth2_token()` to verify JWT signatures before trusting claims.

**[C-03] eval with User-Influenced Variables**
- **File:** `experiments/auth/phase2-poc/cloud-function/deploy.sh:115`
- **Category:** Application - Command Injection
- **Issue:** A gcloud command string is built via variable interpolation (including interactively-entered `IDP_API_KEY` from `read` at line 90), then executed via `eval "$DEPLOY_CMD"`.
- **Impact:** Shell metacharacters in the API key input execute arbitrary commands.
- **Remediation:** Build arguments in an array and expand with `"${args[@]}"` instead of `eval`.

**[C-04] --insecure-skip-tls-verify with Bearer Token Authentication**
- **File:** `experiments/auth/phase2-poc/demo-oidc-flow.sh:59,69`
- **Category:** Auth - Insecure Transport
- **Issue:** kubectl commands use `--insecure-skip-tls-verify` while simultaneously sending bearer tokens via `--token=$IDP_TOKEN`. Line 73 confirms this grants cluster-admin access.
- **Impact:** MITM attacker can intercept the bearer token and gain cluster-admin access to the hosted cluster.
- **Remediation:** Use `--certificate-authority` with the cluster CA certificate.

**[C-05] Wildcard RBAC ClusterRole for kube-applier**
- **File:** `experiments/kube-applier-gcp/hack/manifests/rbac.yaml:6-8`
- **Category:** Kubernetes - Excessive Privileges
- **Issue:** ClusterRole grants `get, list, watch, create, update, patch, delete` on `apiGroups: ["*"]` and `resources: ["*"]` -- equivalent to cluster-admin.
- **Impact:** Any compromise of the kube-applier service account or bound user/group gives complete cluster control including reading all secrets and modifying RBAC.
- **Remediation:** Enumerate the specific resource types kube-applier manages. Remove wildcard apiGroups/resources/verbs.

**[C-06] Cluster-wide Secret Read Access for Prometheus/GMP**
- **Files:** `experiments/google-managed-prometheus/prometheus-cluster-wide.yaml:25-28`, `gmp-podmonitoring-example.yaml:24-26`
- **Category:** Kubernetes - Excessive Privileges
- **Issue:** Two ClusterRoles grant `get, list, watch` on all secrets in all namespaces. Required for TLS-based scraping but extremely broad.
- **Impact:** Compromise of Prometheus or GMP collector allows reading every secret cluster-wide: credentials, tokens, TLS keys, pull secrets.
- **Remediation:** Use namespace-scoped Roles where possible. Add `resourceNames` to restrict which secrets are accessible.

**[C-07] Unpinned :latest Image in Konflux Release Pipeline**
- **File:** `konflux/release-pipelines/tasks/push-snapshot-to-gar.yaml:40-41`
- **Category:** Supply Chain - Image Integrity
- **Issue:** The release task uses `image: quay.io/jimd_openshift/release-service-utils-gcr:latest`. A TODO acknowledges this. This is a **production release pipeline** that pushes images to Google Artifact Registry.
- **Impact:** Full supply chain compromise. A poisoned image from this personal Quay account could exfiltrate secrets, tamper with release artifacts, or inject backdoors.
- **Remediation:** Pin to a specific digest from an organizational registry. Complete the TODO to move to a Konflux-built image.

**[C-08] Konflux Pipeline Task Resolution Uses Mutable main Branch**
- **File:** `konflux/release-pipelines/push-snapshot-to-gar.yaml:30-31`
- **Category:** Supply Chain - Pipeline Integrity
- **Issue:** The pipeline resolves its task definition via git resolver with `default: main`. Task definitions change between runs without any pipeline change.
- **Impact:** A commit to `main` that modifies the task definition is automatically picked up by subsequent release runs.
- **Remediation:** Pin `taskGitRevision` to a specific commit SHA or release tag.

---

### HIGH

**[H-01] API Key Exposed in URL Query Parameter**
- **File:** `experiments/auth/phase2-poc/cloud-function/main.py:282,312`
- **Category:** Secrets - Credential Exposure
- **Issue:** Identity Platform API key passed as URL query parameter to `identitytoolkit.googleapis.com`. URLs are logged by web servers, proxies, and monitoring systems.
- **Impact:** API key leaked to Cloud Function logs and any intermediate proxies.
- **Remediation:** Restrict the API key with application restrictions in GCP console. Ensure Cloud Function logs don't capture full URLs.

**[H-02] TLS Private Keys Written to /tmp Without Secure Permissions**
- **Files:** `experiments/ho-platform-none/install-ho-platform-none/deploy_webhook.py:125`, `build_hypershift.py:88`, `experiments/ho-platform-none/webhook/deploy-webhook.sh:17`, `setup-webhook.sh:17`
- **Category:** Secrets - Key Material Exposure
- **Issue:** TLS private keys generated into `/tmp/` or current directory with default permissions (typically 0644). Cleanup exists but no `trap` on EXIT, creating a race window.
- **Impact:** Private key readable by other users, enabling TLS impersonation of the webhook service.
- **Remediation:** Use `mktemp -d` with `trap` cleanup on EXIT. Set `chmod 600` immediately after generation.

**[H-03] Kubeconfig Files Written to Predictable /tmp Paths**
- **Files:** `experiments/ho-platform-none/install-ho-platform-none/common.py:33-34`, `configure_kubelet.py:244-245`
- **Category:** Secrets - Credential Exposure
- **Issue:** Kubeconfig files with cluster-admin credentials written to `/tmp/kubeconfig-gke` and `/tmp/kubeconfig-hosted` with default permissions.
- **Impact:** Other users on the system gain cluster-admin access to both management and hosted clusters.
- **Remediation:** Use `~/.kube/` with 0600 permissions. Use `tempfile.mkstemp()` for temporary credential files.

**[H-04] Firewall Rules Open to 0.0.0.0/0**
- **Files:** `experiments/ho-platform-none/install-ho-platform-none/create_ignition_worker.py:296-298`, `experiments/psc-research/bash/01-setup-hypershift-redhat-vpc.sh:96-104`, `02-setup-hypershift-customer-vpc.sh:64-73`, `experiments/psc-research/golang/pkg/vpc/vpc.go:216-222,275-282`
- **Category:** Infrastructure - Network Exposure
- **Issue:** Multiple firewall rules allow SSH (tcp:22) and ignition (tcp:30080) from all IPs. The ignition server serves bootstrap configuration including authentication tokens.
- **Impact:** SSH brute-force surface on all VPCs. Ignition endpoint exposed to the internet.
- **Remediation:** Restrict `--source-ranges` to VPC CIDR ranges or use IAP tunneling (`35.235.240.0/20`).

**[H-05] Binary Downloads Without Checksum Verification**
- **File:** `experiments/ho-platform-none/install-ho-platform-none/templates/worker-setup.sh:72,102`
- **Category:** Supply Chain - Integrity
- **Issue:** kubelet and CNI plugins downloaded from the internet without SHA-256 verification. CNI plugins piped directly from `curl` to `tar`.
- **Impact:** DNS poisoning or CDN compromise leads to malicious binaries executed as root.
- **Remediation:** Download checksum files and verify with `sha256sum --check` before installation.

**[H-06] Tokens Visible in Process Table**
- **Files:** `experiments/auth/phase2-poc/demo-oidc-flow.sh:17,32,35,57,68`, `setup-identity-platform.sh:77`
- **Category:** Secrets - Credential Exposure
- **Issue:** Bearer tokens and access tokens passed as command-line arguments to `curl` and `kubectl --token=`, visible via `ps aux` and `/proc/<pid>/cmdline`.
- **Impact:** Any local user can observe tokens granting GCP and Kubernetes access.
- **Remediation:** Use `--password-stdin`, config files, or environment variables instead of CLI arguments.

**[H-07] GCP Service Account Key File With Default Permissions**
- **File:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/setup/setup-local-gcp-auth.sh:160`
- **Category:** Secrets - Key Material Exposure
- **Issue:** `gcloud iam service-accounts keys create` writes JSON key with default umask (0644). The SA has `compute.admin`, `iam.serviceAccountUser`, `viewer`, `storage.admin`.
- **Impact:** Any local user reads the key and authenticates with near-admin project access.
- **Remediation:** Set `umask 077` before creation or `chmod 600` immediately after.

**[H-08] API Key Written to Unprotected Config File**
- **File:** `experiments/auth/phase2-poc/setup-identity-platform.sh:185-194`
- **Category:** Secrets - Credential Exposure
- **Issue:** Identity Platform API key written to `idp-config.json` in plaintext with default permissions.
- **Impact:** API key accessible to all local users, risk of accidental git commit.
- **Remediation:** `chmod 600` on config file. Use Secret Manager for key storage.

**[H-09] Overly Broad IAM Roles at Project Level**
- **Files:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/setup/setup-local-gcp-auth.sh:111-141`, `setup-workload-identity.sh:141-163`
- **Category:** Cloud - Excessive Privileges
- **Issue:** `roles/compute.admin`, `roles/iam.serviceAccountUser`, `roles/viewer`, `roles/storage.admin` granted at project level. `iam.serviceAccountUser` allows impersonation of any SA in the project.
- **Impact:** Compromised SA key gives near-admin access to the entire GCP project.
- **Remediation:** Use custom roles or scoped predefined roles with IAM conditions.

**[H-10] Unpinned :latest Images in Tekton Pipeline Steps**
- **Files:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/pipeline.yaml:36,98`, `gcp-region-e2e/cronjob.yaml:18`, `gcp-region-e2e/pipeline.yaml:117`
- **Category:** Supply Chain - Image Integrity
- **Issue:** Pipeline steps use `alpine:latest`, `alpine/git:latest`, and `tkn:latest`. The CronJob's `tkn:latest` runs `kubectl apply`.
- **Impact:** Compromised images execute with GCP credentials and cluster access.
- **Remediation:** Pin all images to specific digests.

**[H-11] Missing Resource Limits on All Tekton Task Steps**
- **Files:** All pipeline files in `experiments/pipeline-automation/tekton/`, `konflux/release-pipelines/tasks/`
- **Category:** Kubernetes - Denial of Service
- **Issue:** No `resources.requests` or `resources.limits` defined on any task step.
- **Impact:** Runaway or malicious steps consume unbounded cluster resources.
- **Remediation:** Add CPU/memory requests and limits to all task steps.

**[H-12] Command Injection Risk in Terraform Task**
- **File:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/k8s/terraform-gcp-task.yaml:74-78`
- **Category:** CI/CD - Command Injection
- **Issue:** `$(params.command)` and `$(params.args)` interpolated directly into shell script without validation.
- **Impact:** Arbitrary command execution in the Terraform container, which has GCP credentials.
- **Remediation:** Allowlist `command` parameter (init, validate, plan, apply, destroy). Quote arguments.

**[H-13] Unsanitized Webhook Body Data in Trigger Binding**
- **File:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/triggerbinding.yaml:7-15`
- **Category:** CI/CD - Input Validation
- **Issue:** `environment`, `region`, `sector` extracted from HTTP webhook body and passed into pipeline parameters. Region only checks emptiness, not format.
- **Impact:** Path traversal via crafted region or sector values (e.g., `../../etc`).
- **Remediation:** Add strict regex validation. Region should match `^[a-z]+-[a-z]+[0-9]+$`.

**[H-14] No CODEOWNERS File**
- **File:** Repository root
- **Category:** Git - Access Control
- **Issue:** No `CODEOWNERS` file exists. Critical infrastructure files can be modified without mandatory review.
- **Impact:** Changes to release pipelines, ArgoCD bootstrap, and Tekton configs bypass required reviewers.
- **Remediation:** Create CODEOWNERS requiring review for `konflux/`, `bootstrap/`, and `experiments/pipeline-automation/`.

**[H-15] ArgoCD Auto-Sync with Prune Enabled and Default Project**
- **File:** `bootstrap/argocd/root-applicationset.yaml:47-53`
- **Category:** Kubernetes - Configuration
- **Issue:** Root ApplicationSet enables `prune: true`, `selfHeal: true`, uses `default` project (no restrictions), and derives source from cluster secret annotations.
- **Impact:** Modified annotations automatically sync and delete resources. Compromised git repo deploys arbitrary resources.
- **Remediation:** Create dedicated AppProject with explicit source/destination restrictions. Consider `prune: false` or sync windows.

**[H-16] Dockerfile Running as Root**
- **File:** `tools/etcd-benchmark-tool/Dockerfile:8-17`
- **Category:** Containers - Privilege
- **Issue:** No `USER` directive in final stage. Container runs as root (UID 0).
- **Impact:** Container escape or vulnerability exploitation has root-level blast radius.
- **Remediation:** Add `USER 65534` (nobody) after COPY steps, before ENTRYPOINT.

**[H-17] hostPath Volume in PersistentVolume**
- **File:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/pvc.yaml:14-16`
- **Category:** Kubernetes - Container Escape
- **Issue:** PV uses `hostPath: /tmp/tekton-workspace`. Pods get direct host filesystem access.
- **Impact:** Containers read/write host filesystem, bypassing container isolation. Other pods/processes access pipeline data.
- **Remediation:** Use a proper StorageClass with dynamic provisioning.

**[H-18] WIF Example Deployment Missing securityContext**
- **File:** `experiments/wif-example/app/deployment.yaml:50-169`
- **Category:** Kubernetes - Configuration
- **Issue:** No pod-level or container-level securityContext. No `runAsNonRoot`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation`, or `capabilities` drop.
- **Impact:** Container runs with default (potentially root) privileges and writable filesystem.
- **Remediation:** Add full securityContext with `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `capabilities: {drop: [ALL]}`.

**[H-19] PSC Service Attachment with ACCEPT_AUTOMATIC**
- **File:** `experiments/psc-research/bash/04-setup-private-service-connect.sh:122-128`
- **Category:** Cloud - Access Control
- **Issue:** Private Service Connect service attachment allows any GCP project to connect automatically.
- **Impact:** Unauthorized projects connect to internal services without approval.
- **Remediation:** Use `ACCEPT_MANUAL` with `--consumer-accept-list`.

**[H-20] SA Signing Key Written to Unprotected YAML File**
- **File:** `experiments/wif-example/hosted-cluster-setup/2-create-secret.sh:54-63`
- **Category:** Secrets - Key Material Exposure
- **Issue:** SA signing private key base64-encoded into `sa-signing-key-secret.yaml` with default file permissions.
- **Impact:** Anyone with read access extracts the key and forges service account tokens.
- **Remediation:** Pipe directly to `kubectl apply -f -` without writing to disk, or set `chmod 600` immediately.

---

### MEDIUM

**[M-01] Command Injection via gcloud SSH --command**
- **File:** `experiments/psc-research/golang/pkg/testing/testing.go:256-258,329-336,413-419`
- **Category:** Application - Command Injection
- **Issue:** IP addresses interpolated via `fmt.Sprintf` into `--command` arguments for `gcloud compute ssh`. The `--command` value is executed by a remote shell.
- **Impact:** Crafted IP values (from VM metadata) could execute arbitrary commands on target VMs.
- **Remediation:** Validate IP addresses against a strict IPv4/IPv6 regex before interpolation.

**[M-02] Unvalidated Input in kubectl Arguments**
- **File:** `experiments/pipeline-automation/tekton/gcpctl/internal/client/kubectl.go:28-35,70-76`
- **Category:** Application - Input Validation
- **Issue:** `namespace` and `eventID` passed directly to `exec.CommandContext` for kubectl with only empty-string check.
- **Remediation:** Validate parameters match expected patterns (UUID, alphanumeric+dash).

**[M-03] Missing TLS MinVersion Configuration**
- **File:** `experiments/ho-platform-none/webhook/main.go:44-49`
- **Category:** Application - Insecure Transport
- **Issue:** TLS config has no explicit `MinVersion`, potentially allowing TLS 1.0/1.1 on older Go runtimes.
- **Remediation:** Set `MinVersion: tls.VersionTLS12`.

**[M-04] Insecure Temporary File for Ignition Data**
- **File:** `experiments/ho-platform-none/install-ho-platform-none/create_ignition_worker.py:212-214`
- **Category:** Application - File Handling
- **Issue:** `tempfile.NamedTemporaryFile(delete=False)` stores ignition data (including bootstrap tokens) without restrictive permissions.
- **Remediation:** Set `os.chmod(f.name, 0o600)` immediately after creation.

**[M-05] TLS Verification Bypassed with curl -k**
- **File:** `experiments/ho-platform-none/install-ho-platform-none/create_ignition_worker.py:131`
- **Category:** Application - Insecure Transport
- **Issue:** `curl -k` skips TLS verification for ignition server health check. Normalizes ignoring TLS errors.
- **Remediation:** Use the ignition server CA certificate for verification.

**[M-06] Sensitive Data Logged in Error Responses**
- **File:** `experiments/auth/phase2-poc/cloud-function/main.py:293,106-107`
- **Category:** Application - Information Disclosure
- **Issue:** Full IDP response bodies (potentially containing tokens) logged at error level.
- **Remediation:** Sanitize logs to exclude token values. Log only error codes.

**[M-07] User Email Disclosed in Error Responses**
- **File:** `experiments/auth/phase2-poc/cloud-function/main.py:252,257`
- **Category:** Application - Information Disclosure
- **Issue:** Error responses confirm whether specific emails have project access, enabling account enumeration.
- **Remediation:** Return generic "Access denied" messages. Log details server-side only.

**[M-08] Missing set -o pipefail Across Shell Scripts**
- **Files:** 15 scripts in `experiments/auth/`, `experiments/psc-research/`, `experiments/ho-platform-none/webhook/`
- **Category:** Application - Error Handling
- **Issue:** Without `pipefail`, failed commands in pipelines are masked by the exit code of the last command.
- **Remediation:** Use `set -euo pipefail` in all scripts.

**[M-09] Unquoted Variables in gcloud Commands**
- **Files:** All PSC research scripts, pipeline setup scripts (20+ locations)
- **Category:** Application - Command Injection
- **Issue:** Unquoted `$PROJECT_ID`, `$GSA_EMAIL`, etc. in gcloud commands. Word splitting and glob expansion possible.
- **Remediation:** Quote all variable expansions: `"$PROJECT_ID"`.

**[M-10] Missing set -u (nounset) Across Shell Scripts**
- **Files:** Most scripts outside kube-applier-gcp
- **Category:** Application - Error Handling
- **Issue:** Undefined variables silently expand to empty strings. Combined with unquoted variables, causes injection risk.
- **Remediation:** Add `set -euo pipefail` to all scripts.

**[M-11] Unpinned Dockerfile Base Images**
- **Files:** All 4 Dockerfiles (`golang:1.23`, `golang:1.26`, `golang:1.24-alpine`, `ubi-minimal:latest`, `distroless/static:nonroot`)
- **Category:** Supply Chain - Image Integrity
- **Issue:** All base images use version tags rather than digests. `ubi-minimal:latest` is especially risky.
- **Remediation:** Pin to digests: `golang:1.26@sha256:<digest>`.

**[M-12] Loosely Pinned Python Dependencies**
- **Files:** `experiments/ho-platform-none/install-ho-platform-none/requirements.txt`, `experiments/auth/phase2-poc/cloud-function/requirements.txt`
- **Category:** Supply Chain - Dependency Pinning
- **Issue:** All Python deps use `>=` or `*` version specs with no hash pinning.
- **Remediation:** Pin exact versions with hashes: `flask==3.1.1 --hash=sha256:<hash>`.

**[M-13] ServiceAccount Impersonation Without resourceNames**
- **File:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/sa.yaml:25-27`
- **Category:** Kubernetes - Privilege Escalation
- **Issue:** `impersonate` verb on `serviceaccounts` without `resourceNames` restriction.
- **Remediation:** Scope to specific SA names.

**[M-14] Tekton Deployer Can Read All Namespace Secrets**
- **File:** `experiments/pipeline-automation/tekton/tekton/gcp-region-provision/k8s/serviceaccount.yaml:26-28`
- **Category:** Kubernetes - Excessive Privileges
- **Issue:** `get` and `list` on all `secrets` in namespace, not scoped to specific names.
- **Remediation:** Add `resourceNames: ["gcp-credentials"]`.

**[M-15] ServiceAccount Token Automount Not Disabled**
- **Files:** 6 ServiceAccount definitions across `experiments/ho-platform-none/webhook/`, `experiments/wif-example/`, `experiments/pipeline-automation/`, `bootstrap/argocd/`, `experiments/google-managed-prometheus/`
- **Category:** Kubernetes - Least Privilege
- **Issue:** SA tokens automatically mounted into pods even when not needed.
- **Remediation:** Set `automountServiceAccountToken: false` where pod doesn't need K8s API access.

**[M-16] Webhook readOnlyRootFilesystem Explicitly Disabled**
- **File:** `experiments/ho-platform-none/webhook/simple-webhook.yaml:67`
- **Category:** Kubernetes - Container Hardening
- **Issue:** `readOnlyRootFilesystem: false` explicitly set.
- **Remediation:** Use volumes for writable paths and set `readOnlyRootFilesystem: true`.

**[M-17] Test Webhook Uses Full SDK Image with Runtime Compilation**
- **File:** `experiments/ho-platform-none/webhook/test-webhook-deployment.yaml:388`
- **Category:** Containers - Attack Surface
- **Issue:** Deployment uses `golang:1.21-alpine`, runs `apk add`, downloads Go modules, and `go run` at runtime.
- **Impact:** Massive attack surface from SDK image. Network-dependent startup vulnerable to module proxy compromise.
- **Remediation:** Use multi-stage build with minimal final image.

**[M-18] OAuthServer Bound to 0.0.0.0**
- **File:** `experiments/ho-platform-none/install-ho-platform-none/templates/hosted-cluster.yaml:33-34`
- **Category:** Infrastructure - Network Exposure
- **Issue:** OAuth server NodePort listens on all interfaces.
- **Remediation:** Bind to a specific interface IP.

**[M-19] Missing Resource Limits in ArgoCD Deployments**
- **File:** `bootstrap/argocd/install.yaml` (6 Deployment definitions)
- **Category:** Kubernetes - Denial of Service
- **Issue:** No resource limits on ArgoCD containers.
- **Remediation:** Add limits via Kustomize patches.

**[M-20] CronJob Container Missing securityContext**
- **File:** `experiments/pipeline-automation/tekton/tekton/gcp-region-e2e/cronjob.yaml:17-63`
- **Category:** Kubernetes - Container Hardening
- **Issue:** Container running `kubectl apply` has no securityContext.
- **Remediation:** Add `runAsNonRoot`, `readOnlyRootFilesystem`, `capabilities: {drop: [ALL]}`.

**[M-21] Hardcoded Placeholder Project IDs**
- **Files:** 7 scripts in `experiments/psc-research/bash/` using `"your-project-id"` as default
- **Category:** Application - Configuration
- **Issue:** Scripts proceed with placeholder values, causing confusing failures or operations against wrong projects.
- **Remediation:** Use `PROJECT_ID="${PROJECT_ID:?Error: PROJECT_ID must be set}"`.

**[M-22] SSH Key Generated with Empty Passphrase**
- **File:** `experiments/ho-platform-none/install-ho-platform-none/create_namespace_secrets.py:43`
- **Category:** Secrets - Key Material
- **Issue:** SSH key generated without passphrase, directory permissions not explicitly set.
- **Remediation:** Set restrictive directory permissions (0700) on the SSH key directory.

**[M-23] Credentials/Config Written with Default Permissions**
- **Files:** `experiments/auth/phase2-poc/cloud-function/deploy.sh:147-156` (deployment-config.json), `experiments/wif-example/infra/setup-wif-example-gcp.sh:247-258` (credentials.json)
- **Category:** Secrets - File Permissions
- **Issue:** Config files with infrastructure details and WIF configuration written as world-readable.
- **Remediation:** `chmod 600` immediately after creation.

**[M-24] Race Condition in Webhook Certificate Issuance**
- **Files:** `experiments/ho-platform-none/webhook/deploy-webhook.sh:69`, `setup-webhook.sh:68`
- **Category:** Application - Reliability
- **Issue:** Fixed `sleep 5` after CSR approval with no retry logic. Certificate may not be ready.
- **Remediation:** Use a polling loop with timeout.

**[M-25] Credential File Patterns Missing from Root .gitignore**
- **File:** `.gitignore`
- **Category:** Git - Secret Prevention
- **Issue:** Root `.gitignore` lacks patterns for `gcp-key.json`, `*-key.json`, `credentials.json`, `*.pem`, `sa-signing-key-secret.yaml`, `idp-config.json`.
- **Remediation:** Add these patterns to prevent accidental credential commits.

---

### LOW

**[L-01] Information Disclosure in Webhook Error Responses**
- **Files:** `experiments/ho-platform-none/webhook/main.go:83-84,620-621,640-641`
- **Issue:** Raw `err.Error()` returned to clients, revealing parsing/marshaling details.
- **Remediation:** Return generic error messages; log details server-side.

**[L-02] Missing Request Body Size Limit**
- **File:** `experiments/ho-platform-none/webhook/main.go:69-71`
- **Issue:** `io.ReadAll(r.Body)` with no size limit enables memory exhaustion.
- **Remediation:** Use `http.MaxBytesReader(w, r.Body, 10<<20)`.

**[L-03] Missing HTTP Server Timeouts**
- **File:** `experiments/ho-platform-none/webhook/main.go:44-49`
- **Issue:** No `ReadTimeout`, `WriteTimeout`, or `IdleTimeout`. Susceptible to slowloris attacks.
- **Remediation:** Configure all three timeouts.

**[L-04] Race Condition on Global Config Object**
- **File:** `experiments/pipeline-automation/tekton/gcpctl/internal/config/config.go:19,56-70`
- **Issue:** `globalConfig` read/written without synchronization. Data race in concurrent scenarios.
- **Remediation:** Protect with `sync.RWMutex` or use `sync.Once`.

**[L-05] Token Metadata Logged Including Subject Claim**
- **File:** `experiments/wif-example/app/main.go:164-167`
- **Issue:** JWT `sub` claim logged, potentially containing service account identity.
- **Remediation:** Redact or hash the `sub` claim in logs.

**[L-06] Unsafe Type Assertion in Cooldown Checker**
- **File:** `experiments/kube-applier-gcp/internal/controllerutils/cooldown.go:58`
- **Issue:** LRU cache value type-asserted to `time.Time` without checking second return value.
- **Remediation:** Use checked assertion: `if t, ok := nextExecTime.(time.Time); ok`.

**[L-07] Health Endpoint Exposes Project ID**
- **File:** `experiments/auth/phase2-poc/cloud-function/main.py:191`
- **Issue:** `/health` endpoint returns `{"project": PROJECT_ID}` without authentication.
- **Remediation:** Remove project ID from health response.

**[L-08] Hardcoded Certificate Metadata**
- **Files:** `experiments/ho-platform-none/install-ho-platform-none/fix_ovn_networking.py:99-100`, `fix_ovn_networking_final.py:146-149`, `test_ignition_worker.py:176-177`
- **Issue:** Certificate annotations with hardcoded issuer IDs and date ranges that will become stale.
- **Remediation:** Extract values dynamically from the actual certificate.

**[L-09] Partial Token/Key Display in Output**
- **Files:** `experiments/auth/phase2-poc/demo-oidc-flow.sh:18` (60 chars of token), `setup-identity-platform.sh:161` (10 chars of API key)
- **Issue:** Truncated but still reveals structure and narrows brute-force space.
- **Remediation:** Display only "[set]" or "[redacted]".

**[L-10] Using :latest Tag for Built Container Images**
- **Files:** `experiments/wif-example/app/build-and-push.sh:17`, `experiments/ho-platform-none/webhook/deploy-webhook.sh:164`
- **Issue:** Images tagged and deployed as `:latest` -- mutable and non-reproducible.
- **Remediation:** Use immutable tags (SHA digests or version tags).

**[L-11] Missing Cleanup Traps for Temporary Files**
- **Files:** `experiments/psc-research/bash/03-deploy-vms.sh:32-144`, `experiments/ho-platform-none/webhook/setup-webhook.sh:96`
- **Issue:** Temp files with infrastructure details or TLS data persist on early script exit.
- **Remediation:** Add `trap 'rm -f ...' EXIT` at script top.

**[L-12] Demo API Service Runs as Root**
- **File:** `experiments/psc-research/bash/03-deploy-vms.sh:104-105`
- **Issue:** Systemd service for demo Python HTTP server specifies `User=root`.
- **Remediation:** Create and use a dedicated non-root user.

**[L-13] Empty TLS Secret Placeholder in Manifest**
- **File:** `experiments/ho-platform-none/webhook/webhook-deployment.yaml:128-137`
- **Issue:** Empty TLS secret declared as placeholder. Applied without setup script = broken TLS.
- **Remediation:** Remove from manifest; generate exclusively via setup script or cert-manager.

**[L-14] Webhook failurePolicy: Ignore**
- **File:** `experiments/ho-platform-none/webhook/webhook-deployment.yaml:162`
- **Issue:** If webhook is down, admission requests pass without mutation.
- **Remediation:** Consider `failurePolicy: Fail` with high-availability webhook.

**[L-15] Default Namespace Used for Tekton Resources**
- **Files:** Multiple SA and CronJob definitions in `experiments/pipeline-automation/tekton/`
- **Issue:** No namespace isolation for pipeline resources.
- **Remediation:** Deploy to a dedicated namespace with RBAC and NetworkPolicies.

**[L-16] Missing HEALTHCHECK in All Dockerfiles**
- **Files:** All 4 Dockerfiles
- **Issue:** Orchestrators cannot detect unhealthy containers.
- **Remediation:** Add `HEALTHCHECK` instructions.

**[L-17] Password via Command-Line Argument (podman login)**
- **File:** `experiments/wif-example/app/build-and-push.sh:26`
- **Issue:** Access token passed as `-p` argument to `podman login`, visible in process table.
- **Remediation:** Use `gcloud auth print-access-token | podman login -u oauth2accesstoken --password-stdin gcr.io`.

**[L-18] Undefined Variable Reference Across Scripts**
- **File:** `experiments/psc-research/bash/04-setup-private-service-connect.sh:174`
- **Issue:** `$PSC_NAT_SUBNET_RANGE` never defined in this script. Firewall rule gets empty source-ranges.
- **Remediation:** Define the variable in this script's variable section.

**[L-19] Branch Protection Not Codified**
- **File:** Repository root
- **Issue:** No `.github/settings.yml` or rulesets. Branch protection may exist in GitHub UI but is not version-controlled.
- **Remediation:** Codify branch protection with required reviews, status checks, and signed commits.

---

## Positive Observations

1. **No hardcoded real secrets found** across the entire repository
2. **ExternalSecrets properly configured** -- using GCP Secret Manager via Workload Identity Federation (no static credentials)
3. **SecretStore uses WIF**, not JSON keys (`bootstrap/argocd/secret-store.yaml`)
4. **Konflux release task verifies WIF auth** before pushing images
5. **kube-applier-gcp sample deployment** follows security best practices: `automountServiceAccountToken: false`, `runAsNonRoot`, `seccompProfile: RuntimeDefault`, `readOnlyRootFilesystem`, `capabilities.drop: ALL`, and resource limits
6. **Most Dockerfiles use multi-stage builds** and run as non-root in final stage
7. **Go dependencies managed with Go modules** and verified `go.sum` lock files
8. **YAML loading uses `yaml.safe_load()`** correctly in Python code
9. **No use of `eval()`, `exec()`, or `pickle`** on untrusted data in Python
10. **No `InsecureSkipVerify`** in Go TLS configurations
11. **Secret volume mounts** in Tekton tasks are `readOnly: true`
12. **Tekton provision pipeline** has input validation step that allowlists environment values
13. **kube-applier-gcp scripts** use `set -euo pipefail` consistently
14. **ArgoCD NetworkPolicies** included in the install manifest

---

## Security Posture

**Overall Risk:** MODERATE

The production infrastructure (bootstrap/ArgoCD/ExternalSecrets) follows strong security patterns with Workload Identity Federation and Secret Manager integration. The risk concentration is in the experiments/ directory, which contains prototype code used for research and proof-of-concept work. While experiments are expected to have looser controls, several findings (especially the Konflux release pipeline issues C-07/C-08 and the wildcard RBAC C-05) pose real risk if these patterns propagate to production.

**Top Priority Fixes:**
1. Pin Konflux release pipeline image and task revision (C-07, C-08)
2. Scope kube-applier RBAC to specific resources (C-05)
3. Add credential file patterns to root .gitignore (M-25)
4. Add CODEOWNERS file (H-14)

**Quick Wins:**
- Add `set -euo pipefail` to all shell scripts (M-08, M-10)
- Quote all shell variables (M-09)
- Pin Dockerfile base images to digests (M-11)
- Pin Python dependencies to exact versions (M-12)
- Set `automountServiceAccountToken: false` on unused SAs (M-15)
- Add resource limits to Tekton task steps (H-11)
