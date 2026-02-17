# ExternalDNS Configuration for CI E2E Tests

***Scope***: GCP-HCP

**Date**: 2026-02-17

## Decision

Use a dedicated public Cloud DNS zone (`hypershift-ci.gcp-hcp.openshiftapps.com`) in the `gcp-hcp-hypershift-ci` project with GKE Workload Identity for ExternalDNS authentication, matching the production pattern of keyless authentication and per-project DNS zone isolation. The `openshiftapps.com` domain is used because these are hosted cluster API endpoint records, matching production's HC DNS hierarchy. An alternative would be using `devshift.net` (e.g., `hypershift-ci.gcp-hcp.devshift.net`), but that domain is reserved for internal tooling (ArgoCD, Grafana, etc.), not hosted cluster endpoints.
## Context

- **Problem Statement**: HyperShift GCP e2e tests fail because ExternalDNS is disabled in CI. The hosted cluster API endpoint DNS names are unresolvable, causing `ExternalDNSHostNotReachable` failures. AWS and Azure CI flows have properly configured ExternalDNS with real credentials and domain filters; GCP does not have ExternalDNS configured.
- **Constraints**:
  - E2e tests run on the Prow build farm (outside the GKE VPC), requiring publicly resolvable DNS
  - Multiple CI jobs run in parallel, each with its own ephemeral GKE cluster
  - CI must be fully isolated from integration/staging/production environments
  - The `gcp-hcp.openshiftapps.com` HC domain is managed in the gcp-hcp-infra commons module

## Alternatives Considered

### DNS Zone Placement

1. **Dedicated CI zone** (chosen): Create `hypershift-ci.gcp-hcp.openshiftapps.com` in the `gcp-hcp-hypershift-ci` project with NS delegation from the parent `gcp-hcp.openshiftapps.com` zone. All CI DNS permissions stay within the CI project.

2. **Reuse existing integration DNS zone**: Write CI records to an existing integration zone. This requires granting the CI ExternalDNS SA `roles/dns.admin` on either the integration global project (`gcp-hcp-int-global`, for the `int.gcp-hcp.openshiftapps.com` zone) or an integration region project (e.g., `int-reg-us-c1-nkcw`, for its regional HC zone). Both options mix CI test records with integration HC records and grant a CI service account write access to integration DNS — a cross-environment security boundary violation.

### ExternalDNS Authentication

1. **GKE Workload Identity** (chosen): ExternalDNS authenticates via WIF, matching the production pattern. Requires dynamic WIF binding management per test (create on setup, remove on teardown) because ephemeral GKE clusters have different project IDs each run.

2. **SA key file**: Pass `--external-dns-credentials=${CLUSTER_PROFILE_DIR}/credentials.json` to mount the `hypershift-ci` SA key into the ExternalDNS pod. Simpler (no WIF binding management), but uses static credentials instead of production's keyless authentication.

## Decision Rationale

* **Justification**: A dedicated CI DNS zone with WIF matches the production architecture (keyless auth, `--google-project` targeting, per-project zone isolation) while maintaining complete separation from other environments. The `hypershift-ci.gcp-hcp.openshiftapps.com` zone is scoped to the CI project with no cross-environment permissions.
* **Evidence**: Production ExternalDNS uses WIF with `--google-project=<region-project>` and `--txt-owner-id=<mc-id>` for multi-cluster isolation. The same pattern applies to CI where multiple parallel jobs share a zone.
* **Comparison**: For DNS zone placement, reusing integration zones violates the CI/integration security boundary. For authentication, SA key files would work but diverge from production's keyless Workload Identity pattern.

## Consequences

### Positive

* Matches production authentication pattern (Workload Identity, no static credentials)
* Complete isolation from integration/staging/production DNS
* Supports parallel CI jobs via `--txt-owner-id` per-job record ownership
* DNS zone lives in the existing `gcp-hcp-hypershift-ci` project — no new projects needed

### Negative

* Requires dynamic WIF binding management per test (create on setup, remove on teardown)
* Stale WIF bindings may accumulate if cleanup fails (harmless but untidy)

## Cross-Cutting Concerns

### Security:

* CI ExternalDNS SA (`external-dns@gcp-hcp-hypershift-ci`) only has `roles/dns.admin` on the CI project — cannot modify DNS in any other environment
* `hypershift-ci` SA has `roles/iam.serviceAccountAdmin` scoped to the CI ExternalDNS SA only — this allows CI jobs to create and remove WIF bindings per test run, without granting IAM permissions on any other service account
* WIF bindings are scoped to ephemeral GKE cluster identities — cannot impersonate production SAs
* NS delegation in the commons `gcp-hcp.openshiftapps.com` zone is a static, read-only record from CI's perspective

### Operability:

* WIF binding cleanup runs in the deprovision step with `|| true` to avoid blocking cleanup on failure
* Each CI job uses a unique `--txt-owner-id` to prevent parallel jobs from interfering with each other's DNS records
* ExternalDNS cleans up DNS records during normal operation when hosted clusters are deleted; orphaned records from abrupt terminations (e.g., test timeout) are mitigated by explicit cleanup in the deprovision step, which deletes all matching records via `gcloud` before destroying the GKE cluster
