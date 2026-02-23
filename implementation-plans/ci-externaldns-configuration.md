# ExternalDNS Configuration for CI E2E Tests

## Overview

Configure ExternalDNS for HyperShift GCP e2e tests to resolve hosted cluster API endpoint DNS names. This requires infrastructure changes (DNS zone, SA) in `gcp-hcp-infra` and CI configuration changes in `openshift/release`.

**Design Decision**: [ci-externaldns-configuration.md](../design-decisions/ci-externaldns-configuration.md)

---

## Story 1: Create CI DNS Zone and ExternalDNS SA

**Summary**: Create a public Cloud DNS zone for CI and a dedicated ExternalDNS GCP service account

**Repository**: `gcp-hcp-infra`

**PR**: Must be merged and applied before Story 2

**Tasks**:

1. **Create Cloud DNS zone** (new file: `terraform/modules/hypershift-ci/cloud-dns.tf`):
   - [ ] Create public zone `hypershift-ci.gcp-hcp.openshiftapps.com` in the `gcp-hcp-hypershift-ci` project
   - [ ] Follow pattern from `terraform/modules/global/cloud-dns.tf` (`cloud_dns_zone_env_gcp_hcp`)

2. **Create ExternalDNS GCP service account** (modify: `terraform/modules/hypershift-ci/main.tf`):
   - [ ] Create SA: `external-dns@gcp-hcp-hypershift-ci.iam.gserviceaccount.com`
   - [ ] Grant `roles/dns.admin` on the `gcp-hcp-hypershift-ci` project
   - [ ] Grant the `hypershift-ci` SA `roles/iam.serviceAccountAdmin` on the ExternalDNS SA (required so CI jobs can add/remove WIF bindings on it during setup and deprovision)
   - [ ] Note: CI jobs create two WIF bindings per test run: `roles/iam.workloadIdentityUser` and `roles/iam.serviceAccountTokenCreator` (both required for cross-project WIF token generation)

3. **Export zone outputs** (modify: `terraform/modules/hypershift-ci/outputs.tf`):
   - [ ] Export `ci_dns_zone_name_servers`
   - [ ] Export `ci_dns_zone_domain`

**Acceptance Criteria**:
- [ ] Cloud DNS zone `hypershift-ci.gcp-hcp.openshiftapps.com` exists in `gcp-hcp-hypershift-ci` project
- [ ] ExternalDNS GCP SA exists with DNS Admin on CI project
- [ ] `hypershift-ci` SA can manage IAM bindings on the ExternalDNS SA

---

## Story 2: Add NS Delegation from Parent Zone

**Summary**: Delegate `hypershift-ci.gcp-hcp.openshiftapps.com` from the parent `gcp-hcp.openshiftapps.com` zone

**Repository**: `gcp-hcp-infra`

**Prerequisites**: Story 1 applied (zone nameservers available in Terraform state)

**Tasks**:

1. **Add NS delegation** (modify: `terraform/config/commons/main.tf`):
   - [ ] Add `hypershift-ci` entry to `hc_environment_dns_zones` variable
   - [ ] Reference CI zone nameservers (from hypershift-ci remote state or hardcoded after Story 1 apply)

**Acceptance Criteria**:
- [ ] `dig hypershift-ci.gcp-hcp.openshiftapps.com NS` returns the CI zone's nameservers
- [ ] DNS resolution works end-to-end for records in the CI zone

---

## Story 3: Configure ExternalDNS in CI with Workload Identity (GCP-401)

**Summary**: Enable ExternalDNS in the HyperShift install step with WIF authentication

**Jira**: [GCP-401](https://issues.redhat.com/browse/GCP-401)

> **Note**: This story extends the scope of GCP-401 (originally focused on CI step-registry components) to include ExternalDNS configuration with Workload Identity. The GCP-401 ticket should be updated to reflect this additional work.

**Repository**: `openshift/release`

**Prerequisites**: Stories 1 and 2 applied (DNS zone and delegation functional)

**Tasks**:

1. **Update hypershift-install** (modify: `ci-operator/step-registry/hypershift/install/hypershift-install-commands.sh`):
   - [ ] Add `--external-dns-domain-filter=hypershift-ci.gcp-hcp.openshiftapps.com` and `--external-dns-google-project=gcp-hcp-hypershift-ci` to `hypershift install`
   - [ ] After install, create WIF bindings on ExternalDNS GCP SA for the ephemeral cluster (both `roles/iam.workloadIdentityUser` and `roles/iam.serviceAccountTokenCreator` — both required for cross-project WIF)
   - [ ] Annotate K8s SA `external-dns` in namespace `hypershift` with GCP SA email
   - [ ] Restart ExternalDNS to pick up WIF credentials

2. **Update base domain** (modify: `ci-operator/step-registry/hypershift/gcp/run-e2e/hypershift-gcp-run-e2e-commands.sh`):
   - [ ] Change `BASE_DOMAIN="in.${CLUSTER_NAME}.int.gcp-hcp.devshift.net"` to `BASE_DOMAIN="in.${CLUSTER_NAME}.hypershift-ci.gcp-hcp.openshiftapps.com"`
   - [ ] Restore `--e2e.external-dns-domain` flag (required for Route hostname generation on non-OpenShift clusters)

3. **Clean up DNS records and WIF bindings** (modify: `ci-operator/step-registry/gke/deprovision/gke-deprovision-commands.sh`):
   - [ ] Delete all DNS records matching `*.in.${CLUSTER_NAME}.hypershift-ci.gcp-hcp.openshiftapps.com` from the CI zone via `gcloud dns record-sets` (catches orphans if ExternalDNS didn't reconcile before shutdown)
   - [ ] Remove both WIF bindings during cleanup (`|| true` to avoid blocking)

4. **Run `make update`** to regenerate Prow job configs

**Acceptance Criteria**:
- [ ] ExternalDNS creates DNS records for hosted cluster API endpoints
- [ ] `e2e-gke` rehearsal job passes (hosted cluster API is reachable)
- [ ] WIF bindings are cleaned up after test completion
- [ ] Parallel CI jobs do not interfere with each other's DNS records

---

## Verification

1. **Infrastructure**: `dig hypershift-ci.gcp-hcp.openshiftapps.com NS` returns valid nameservers
2. **CI test**: `e2e-gke` rehearsal job passes end-to-end
3. **WIF cleanup**: `gcloud iam service-accounts get-iam-policy external-dns@gcp-hcp-hypershift-ci.iam.gserviceaccount.com` shows no stale bindings after tests complete
