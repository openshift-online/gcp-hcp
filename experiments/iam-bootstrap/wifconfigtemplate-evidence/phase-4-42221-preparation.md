# Phase 4 selected pair — 4.22.21 to exact 5.0.0, same-project coexistence

**Later continuation:** [reviewed code-only preparation](phase-4-42221-code-checkpoint.md)
now implements/tests the comparison bridge and inactive examples. The user
explicitly removed a standalone CPO inspection/credential task from the critical
path. Anonymous-read limitations below are historical observations, not an extra
blocking prerequisite. Use normal configured pulls/HyperShift validation later.

**2026-11-20. The user explicitly selected 4.22.21 as the fresh installation's
starting version, retaining exact 5.0.0 as target and identical IAM content.**
This supersedes the source-version decision in the
[4.23 feasibility checkpoint](phase-4-423-isolation-feasibility.md), not its
isolation findings or safety boundaries. It does not authorize provisioning,
IAM/assertion/gate changes, an upgrade, catalog publication or a runtime rollout.
No implementation was changed in this continuation.

## Exact selected release identities

| Role | Version | Payload |
| --- | --- | --- |
| Fresh source | **4.22.21** | `quay.io/openshift-release-dev/ocp-release@sha256:828fa71fe839d280cd731271d04917c5a5ca7f7ce49dd68e3eb1a5d37f9f7f02` |
| Upgrade target | **5.0.0** | `quay.io/openshift-release-dev/ocp-release@sha256:5d7ed8b3e98bc3a10c50f36f07262a962d59ad2d3ea5fb4935ff10c408c02e55` |

- At **07:55:22 UTC**, fresh candidate-5.0 and fast-5.0 graph reads reconfirmed
  the direct edge and both exact payloads. Candidate is the proposed channel
  group for continuity; no Channel was changed. Actual HC/CVO advertisement must
  still be checked before an eventual upgrade, not inferred from today's graph.
- Keep actual release identities unchanged; neither a floating `4.22` source
  nor floating `5.0` target is substituted.
- Source Version is not in the current minimum-5.0 synchronized catalog, so no
  source Version UID/generation/pin can be claimed yet. Target Version remains
  UID `9bbbd3d4-d2c6-442c-942d-4c65c3c873a5`, generation 2, original immutable
  `5.0.0` template reference. No existing pin is repointed or recreated.

## Read-only payload/boot-image findings

Payload inspection used explicit digest pullspecs, `linux/amd64`, normal registry
verification/TLS, `/dev/null` kubeconfig and an empty registry auth config. No
cluster interrogation, secret extraction, container execution or image rebuild.

- Public `oc adm release info -o json` reports **4.22.21** and **5.0.0**,
  respectively, with 191 and 197 component references.
- Static `0000_50_installer_coreos-bootimages.yaml` extraction succeeded for
  both payloads. Both contain `rhel-9` and `rhel-10` streams with x86_64 GCP
  project/image names, plus the legacy rhel-9 stream.
- Read-only Compute GETs at **07:56:16 UTC** confirmed both referenced images
  in `rhcos-cloud` are **READY**, `X86_64`, 16 GB base disks, no deprecation
  field in the returned metadata:
  - rhel-9: `rhcos-9-8-20261026-0-gcp-x86-64`, image ID `3174356214913046979`.
  - rhel-10: `rhcos-10-2-20261026-0-gcp-x86-64`, image ID `8474940032542711153`.
  Availability is not an assertion that a VM was booted or that quotas, machine
  compatibility, required IAM, CPO behavior or full guest functionality passed.
- Pinned HO `GetRHELStreamForBootImage` derives rhel-9 for <5.0 and rhel-10 for
  >=5.0 when OSStreams is enabled and no explicit stream is chosen. The live
  feature configuration and intended stream/worker replacement behavior must be
  reviewed before provisioning; do not assume the upgrade changes only kubelet
  version or silently add an OS-stream override.

### CPO capability check remains open

Payload `hypershift` component references are:

- Source: `quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:c3f62c92f9cc447961d0eba707c07b77794a97614d4f516a903e3787e69fc15b`.
  Payload source annotation: HyperShift `db8d2278410f6ce7b5ec85971b80aea598d27222`.
- Target: `quay.io/openshift-release-dev/ocp-v5.0-art-dev@sha256:29485d371b5c140f8140299501c6f165896092a3c8901efa73095a9faf81e10f`.
  Payload source annotation: HyperShift `4ae18b48e9170b891d4b5124bffe74e4e5dec3a8`.

`oc image info` for these component images returned **unauthorized using the
explicit empty registry config**. Public release metadata is readable; these
anonymous component-image reads are not. No fallback to Kubernetes Secret
payloads, global registry credentials, TLS/verification bypass or CPO image
override was attempted. Source annotations are descriptive metadata, not an
independent build attestation or proof of the deployed component's capability.
Use an explicitly approved existing registry-auth mechanism for the remaining
label/capability read; do not paste credentials into chat or commit them.

The pinned HO ordinarily uses the release's `hypershift` component for CPO;
its embedded platform override structure covers AWS/Azure, not a GCP override
that would justify assuming the management operator image replaces this check.

Local-only supporting files (mode 0600) are in mode-0700
`.../opencode/wif-p4-42221-preparation-c04_877f`. No actual registry credentials
were stored; the auth file contains only `{"auths":{}}`.

## Proposed code/catalog preparation — not implemented or deployed

1. **Bridge comparison semantics deliberately.** Use pinned HyperShift's
   supported 5.x ↔ 4.(23+x) relationship for OCP release ordering/stream/skew
   comparisons, preserving patch/prerelease distinctions. For this pair, 4.22
   to 5.0 is the logical next minor (Kubernetes 1.35 → 1.36). Audit worker
   admission/coordinator, automatic fleet selection and any version-resolution
   comparisons; reject normalized downgrades, unsupported majors and skipped
   minors. Do not simply remove major guards or force a dispatch.
2. **Keep WIF identities raw and immutable.** Comparison normalization must not
   change release names, payloads, template numeric-floor selection, existing
   Version pins or assertion references. Proposed source boundary `4.22.0`
   copies the existing schemaVersion/IAM content exactly; target remains the
   existing `5.0.0` reference. Equal executable digest, distinct name/UID refs;
   expected upgrade preview: zero role additions, new target application.
3. **Prepare an opt-in catalog change.** Propose candidate minimum 4.22 without
   changing install default/fleet authorization or existing pins. Source template
   starts verified=false. Publication/sync/pinning and later verification are
   separately approved live data changes, not implied by selecting the pair.
4. **Test and review before publication.** Focused boundary/skew/prerelease,
   graph/admission, raw WIF identity, no-op-permission/append and isolation/owned
   cleanup regressions; mandatory architect review, cohesive commits and
   reproducible changed-component artifacts. No broad controller bump by default.
5. **Resolve the live safety gates.** Close CPO capability/auth checks, choose
   the worker OS-stream behavior, approve the new footprint/cost/time bounds,
   and explicitly protect retained `wif3-first` when shared `5.0.0` eligibility
   opens. Customer upgrades bypass maintenance exclusions: a target assertion
   can release a previously IAM-blocked customer request immediately. Any new
   retained-cluster hold/spec change needs explicit approval, not an assumption.

Tentative second-installation names remain `wif4-isolation`, `wif4-workers`,
infra ID `wif4-iso1`, in `apahim-dev-test-1`, using separate owned IAM/network
resources/journals. Previous absence checks are not reservations and must be
refreshed. Same-project unconditional roles and the shared default worker SA
still limit the security-isolation claim; use the earlier matrix honestly.

At 07:55:22 UTC, fresh regional reads confirmed original Cluster UID/generation/
5.0.2 release and no requested upgrade, all eight Deployments Ready, both gates
false and all three protected journals byte-identical with no update sidecar.
No guard rehearsal, IAM update, assertion append, gate activation, catalog write,
new cluster/VM, rollout, maintenance modification, Terraform action or teardown
was executed. Selecting this pair is not a claim that live validation is ready.
