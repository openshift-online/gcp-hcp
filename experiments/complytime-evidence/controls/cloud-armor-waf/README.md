# Cloud Armor WAF example

This directory contains a learning prototype with example-only Gemara Layer 1 WAF guidance, a Layer 2 Cloud Armor configuration check, and a Layer 3 policy implemented in OPA/Rego and packaged by CompliPack. Synthetic fixture scans and explicitly invoked, read-only live scans use the same ComplyTime path and produce a native Layer 5 `EvaluationLog` locally. This is not a deployment artifact, approved compliance control, or delivered audit evidence.

## What the example checks

The input is a normalized envelope containing target metadata, one backend service, one Cloud Armor security policy, and frontend evidence. For a live run, the collector marks the target internet-facing only after finding this GCP configuration chain; Rego then checks that normalized boolean. The committed baseline sets the boolean directly, so it does not independently prove the chain; the normalizer test exercises the chain with synthetic input. The rule returns a passing result only when:

1. The target is marked internet-facing.
2. The backend service has a non-empty security-policy reference, and it matches the policy being evaluated.
3. At least one security-policy rule has a deny or rate-limit action (`deny(...)`, `throttle`, or `rate_based_ban`) and is not in preview.
4. Backend-service request logging is enabled, and its sample rate is greater than zero and no greater than one.

This deliberately follows the walkthrough's initial “active deny or rate-limit rule” test. It is a generic Cloud Armor configuration action check: a qualifying rule's existence does not prove the policy detects or blocks a real web attack, that the rule's match expression covers the application, or that rule ordering makes it effective for a particular request. For live data, the collector follows the backend service's URL-map references, finds matching global HTTP(S) target proxies, then finds forwarding rules targeting those proxies. It marks the backend internet-facing only when this chain ends in an external global forwarding rule (`EXTERNAL` or `EXTERNAL_MANAGED`). This verifies GCP configuration relationships, not DNS resolution, client reachability, or whether a network allowlist limits who can connect.

### What “policy enabled” means here

Cloud Armor does not use a single policy-level `enabled` flag for this check. A security policy must be attached to a backend service to take effect, and each candidate deny/rate-limit rule must not be in preview to enforce its action. The fixture test runner generates temporary detached-policy, empty-reference, no-enforcing-rule, and preview-only cases to exercise those conditions. A policy that exists but is unattached is not counted as protecting the backend. An active rule alone is not proof that it successfully blocks an attack.

The logging check is also deliberately limited: a positive sample rate means requests can be logged, but it does not prove that a particular WAF event was logged, that logs reach an approved central destination, or that an actionable alert is delivered. Those require a later end-to-end evidence/alert-delivery test. A zero sample rate produces no request logs; a rate of `1.0` logs all requests.

The committed fixture data is one readable, synthetic passing baseline: an internet-facing target with a matching attached policy, an enforcing preconfigured WAF rule, and request logging enabled. `tests/run-fixtures.sh` derives positive custom-deny and rate-limit cases plus negative cases from that baseline in a temporary directory, runs each through Conftest, and checks both the expected outcome and failure reason. It removes the generated cases on exit, so there are no checked-in failure snapshots to maintain.

The JSON envelope is a prototype contract, not a native Cloud Armor API export. All fixture project and resource names are synthetic. The live runner uses the user's existing gcloud authentication to read the two explicitly named global resources; it never changes them. Raw responses and the normalized input are kept in a temporary directory and deleted when the runner exits.

This is only a narrow check against one selected backend, not an assessment of every internet- and customer-facing application. It does not assess WAF rule freshness, delivery of logs or alerts to an approved central destination, whether an alert can be investigated, or whether a particular attack generated an alert. Those need separate checks before drawing a conclusion about a complete WAF control. Therefore a `pass` is not a pass of a full organizational control.

## How this maps to the walkthrough

The content uses the intended Gemara layers: `guidance-catalog.yaml` is non-normative example Layer 1 guidance, `control-catalog.yaml` is a narrow Layer 2 assessment requirement linked to that guideline, and `policy.yaml` is the Layer 3 evaluation plan naming OPA. `make scan-fixture` packages the Gemara layers and OPA policy as local OCI artifacts, runs `complyctl get`, `generate`, and `scan` with the OPA provider, then leaves the native Layer 5 `EvaluationLog` in the local workspace. The registry is temporary and bound only to loopback.

The local scan path now works through EvaluationLog generation, but evidence delivery is intentionally out of scope: there is no S3/Evidence Locker upload, Hyperproof API connection, Sumo Logic forwarding, WIF setup, or production pipeline automation. The provider is still marked for testing purposes by its maintainers. This remains a narrow prototype; a passing run does not prove alert delivery, rule freshness, detection of a particular attack, or full WAF-control compliance.

## Files

The Gemara artifacts are under `gemara/`. The live gcloud collector and its jq normalizer are under `scripts/`. The tests and the single passing synthetic baseline are under `tests/`; the normalizer test uses inline synthetic input.

- `complypack.yaml`: local ComplyPack configuration and custom schema registration.
- `schema/cloud-armor-assessment.cue`: schema for the normalized input envelope.
- `policy/cloud_armor_waf.rego`: illustrative OPA rule.
- `policy/complytime-mapping.json`: mapping to an example-only requirement ID.
- `tests/fixtures/active-preconfigured-waf.json`: the synthetic passing baseline.
- `tests/run-fixtures.sh`: derives and tests pass/fail scenarios in temporary files, then emits JSON Lines with each scenario's outcome, target ID, and input SHA-256.
- `tests/test-normalizer.sh`: tests the gcloud-response normalizer using synthetic input.
- `scripts/run-complytime-scan.sh`: shared local OCI → ComplyTime → EvaluationLog runner used by fixture and live scans.
- `scripts/run-fixture-scans.sh`: runs one synthetic pass and one derived synthetic failure through the shared runner.
- `tests/test-complytime-fixture.sh` and `tests/test-live-collector.sh`: exercise the native fixture path and the read-only live collector contract.
- `Makefile`: short entry points so the tool and scan commands are easy to repeat.

The Gemara artifacts are split by layer: `gemara/guidance-catalog.yaml` is the example Layer 1 guidance, `gemara/control-catalog.yaml` is the Layer 2 control, and `gemara/policy.yaml` is the Layer 3 evaluation policy. `complypack.yaml` includes all three documents.

## Install and run locally

Run the commands from this directory so the local Gemara and CUE file references resolve correctly.

### Prerequisites

- `make`, Bash, `git`, `curl`, `jq`, `yq`, `conftest`, `oras`, `shasum`, and Podman.
- There are two common, incompatible tools named `yq`: this prototype supports both Mike Farah `yq` v4 and the Python `yq` jq-wrapper.
- Go 1.26.7 or newer. `make install-tools` sets `GOTOOLCHAIN=auto`, so Go can download a compatible toolchain when the system Go is older (network access is needed the first time).
- ORAS 1.3.2 on `PATH` (`oras version` should report version `1.3.2`; build metadata such as `+Homebrew` may follow it). This prototype does not install ORAS for you.
- `gcloud` and an identity with read access to the selected resources are needed only for `make scan-live`.

Install the ComplyTime tools into this control's ignored `bin/` directory:

```sh
make install-tools
```

The installer pins `complyctl v1.0.0`, OPA provider `v0.2.1`, and CompliPack `v0.0.8`; these are installed locally and their Go build caches stay under ignored `.complytime/`. CompliPack `v0.0.8` is intentional: it is the API version required by `complyctl v1.0.0`; CompliPack `v0.1.0` emits an incompatible provenance shape. The OPA provider is still testing-only, not a production-readiness signal.

### Repeatable commands

```sh
# Run unit/normalizer tests plus local ComplyTime pass and fail scans.
make test

# Run just the two native ComplyTime fixture scans.
make scan-fixture

# Read-only scan of explicitly named global GCP resources.
make scan-live \
  PROJECT=YOUR_PROJECT_ID \
  BACKEND_SERVICE=YOUR_BACKEND_SERVICE \
  SECURITY_POLICY=YOUR_CLOUD_ARMOR_POLICY
```

Fixture inputs and fake-gcloud responses are synthetic. `make test` does not call the live GCP project; its live-collector test substitutes fake `gcloud` and a scan-runner stub. The actual `make scan-live` command uses only read-only `gcloud describe` and filtered `list` calls, follows the backend's URL-map → HTTP(S) proxy → forwarding-rule chain, and sends the normalized temporary snapshot through the same shared ComplyTime runner. Check the active gcloud account before a live run with `gcloud auth list --filter=status:ACTIVE`. If Podman is unavailable, the runner prints instructions to start the existing machine using `podman machine start`; it never initializes or reconfigures Podman.

A completed assessment returns exit status 0 whether its native result is `Passed` or `Failed`. A failed requirement is an assessment outcome, not a runner error. Collection, provider, workspace, or EvaluationLog errors return status 2. A fixture whose actual result differs from its expected result fails the test.

### Local outputs and privacy

- `bin/` contains the locally installed pinned tools.
- `.complytime/gopath/` and `.complytime/go-cache/` contain Go modules, toolchains, and build cache used by `make install-tools`.
- `.complytime/runs/scan.*` contains local runner diagnostics, generated ComplyTime workspace/configuration, and EvaluationLogs at `<run>/.complytime/scan/evaluation-log-*.yaml`.
- The temporary loopback registry is stopped and removed after each run. Live raw GCP responses and the normalized input are created in a temporary directory and removed on exit.

`bin/` and `.complytime/` are ignored by Git. EvaluationLogs from live scans can contain resource identifiers; keep them local and do not add them to a commit. To list local reports, run `find .complytime/runs -type f -path '*/.complytime/scan/evaluation-log-*.yaml' -print`. `make clean` removes only the control's ignored tool binaries and generated `.complytime/` data.

The fast policy-only suites remain directly runnable as `bash tests/run-fixtures.sh` and `bash tests/test-normalizer.sh`; they use synthetic fixtures and do not replace the native ComplyTime scan.

## Why this first version uses Bash

The Bash scripts are small learning glue around real `gcloud` API reads and the native ComplyTime CLI; they do not manufacture live assessment data. `jq` reshapes the API responses, and the same Gemara policy, CompliPack, and provider path is used for fixture and live input. The wrapper is not a decision that production collection should be Bash. This local experiment does not define a scheduled automation pipeline; the repository's [pipeline automation decision](../../../../design-decisions/automation/pipeline-automation-tooling.md) selects Tekton for platform workflows.

## Scope boundaries

The compact live summary reports the native ComplyTime result and configuration observations, without echoing project or resource names. Because cloud CLI error messages may include identifiers, keep the run local and do not publish captured stderr or local EvaluationLogs. The scan is read-only, but it uses the currently active gcloud identity rather than WIF.

The EvaluationLog is local-only: there is no S3/Evidence Locker upload, Hyperproof integration, Sumo Logic forwarding, WIF setup, or auditor access in this prototype. A live result is evidence of this one configuration snapshot only. It does not prove that Cloud Armor blocks a particular attack, that alert delivery works, that logs reach a central destination, or that rules are kept current.

## References

- [Cloud Armor security policy REST resource](https://docs.cloud.google.com/compute/docs/reference/rest/v1/securityPolicies)
- [Cloud Armor security policy overview](https://docs.cloud.google.com/armor/docs/security-policy-overview)
- [Cloud Armor per-request logging](https://docs.cloud.google.com/armor/docs/request-logging)
- [Global external Application Load Balancer logging and monitoring](https://docs.cloud.google.com/load-balancing/docs/https/https-logging-monitoring)
- [ComplyPack README and CLI examples](https://github.com/complytime/complypack)
- [ComplyPack v0.0.8 (pinned for complyctl v1.0.0)](https://github.com/complytime/complypack/tree/v0.0.8)
- [complyctl v1.0.0](https://github.com/complytime/complyctl/tree/v1.0.0)
- [OPA provider v0.2.1 README](https://github.com/complytime/complytime-providers/blob/v0.2.1/cmd/opa-provider/README.md)
- [ComplyPack example configuration](https://github.com/complytime/complypack/blob/main/complypack.example.yaml)
- [Gemara Control Catalog schema](https://gemara.openssf.org/schema/controlcatalog.html)
- [Gemara Guidance Catalog schema](https://gemara.openssf.org/schema/guidancecatalog.html)
- [Gemara Policy schema](https://gemara.openssf.org/schema/policy.html)
- [complyctl Quick Start](https://github.com/complytime/complyctl/blob/main/docs/QUICK_START.md)
- [OPA provider README](https://github.com/complytime/complytime-providers/blob/main/cmd/opa-provider/README.md)
- [Gemara ADR-0022: Evidence on Assessment Logs](https://gemara.openssf.org/adrs/0022-evidence-on-assessment-log.html)
