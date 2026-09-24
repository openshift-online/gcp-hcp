# Upstream Changes Tracker

This document tracks every code change the proof-of-concept needed, classified by delivery state and production readiness.

## Summary table

| Repo | Change | Jira | PR | State | Classification |
|------|--------|------|----|-------|----------------|
| openshift/console | Honor `-ca-file` and `-service-ca-file` for off-cluster TLS trust (KAS resource proxy, anonymous transport, service proxies) + request OIDC offline access for refresh tokens + `http.ProxyFromEnvironment` on the plugin asset transport | GCP-1219 | [#17185](https://github.com/openshift/console/pull/17185) | Open | Production-shaped, unit-tested, all gated to off-cluster mode and/or external OIDC so the in-cluster OpenShift OAuth path is unchanged. **Merge blocker:** needs an upstream CONSOLE-xxxx or OCPBUGS-xxxx Jira prefix to replace the downstream GCP-1219. |
| openshift/hypershift | CPO owns console/downloads exposure Routes + `-private` variants + PSC ExternalName services for external-dns + generic labeled-Route router backends at port 8443 | GCP-1202 | [#9622](https://github.com/openshift/hypershift/pull/9622) (draft/RFC) | Open | Production-shaped, mirrors the existing KAS/OAuth route patterns. |
| openshift/hypershift | CPO strips the guest console-operator Deployment AND the `ClusterOperator/console` manifest from the CVO payload (GCP-gated). Both are required: leave the ClusterOperator in and the guest CVO blocks on ClusterOperatorNotAvailable forever. | GCP-1202 | In #9622 | Open | Spike hack: GCP-gated with hardcoded payload manifest filenames. |
| openshift/hypershift API | GCP-gated CEL relax so a GCP HostedCluster can have Console enabled while Ingress is disabled (upstream CEL forbids it). Converges with OCPBUGS-58422; console-operator#1182 merged; hypershift PR 8933 removes the rule for all platforms and will replace the GCP-only relax. | (OCPBUGS-58422) | [#8933](https://github.com/openshift/hypershift/pull/8933) | Open | Production-shaped (removes the rule for all platforms). |
| openshift/hypershift | GCP worker firewall rule moved from the one-shot infra CLI (`<infra-id>-allow-kubelet`) to continuous CPO reconciliation of a single grouped rule (`<infra-id>-internal-cluster`), including UDP/6081 geneve for OVN-Kubernetes cross-node pod networking | GCP-1221 | [#9640](https://github.com/openshift/hypershift/pull/9640) | Open | Production-shaped: ownership markers in the rule description, idempotent reconcile, teardown on delete, a `GCPFirewallRulesReady` condition mirrored to the HostedCluster, and a "degrade, don't wedge" contract (missing credentials/permissions/VPC set the condition False with an actionable message rather than failing HCP reconcile). Note: originally hit as a console-blocking bug (geneve drops broke konnectivity and all cross-node pod networking) but is independently valuable. |
| openshift/cluster-network-operator | Set container-level `allowPrivilegeEscalation: false` + `capabilities.drop: [ALL]` (and multus pod-level `runAsNonRoot: true`) on self-managed network operands when on an SCC-less/restricted-PSA management cluster | Not yet filed | Not yet filed | Local workaround | Currently using `hypershift.openshift.io/pod-security-admission-label-override: baseline` annotation on the HostedCluster. |

## What would a real implementation ship

The hand-applied `manifests/console/` kustomize tree used in this spike is a **study artifact, NOT a proposed deliverable**. The production shape would be:

1. **A CPOv2 console component** following the standard control-plane-operator deployment pattern (mirroring the existing ingress-operator and cluster-network-operator components). This component would:
   - Run the console-operator as a deployment in the HCP namespace
   - Inject konnectivity sidecar for guest cluster access (HTTPS mode for KAS-only, Socks5 for bridge reaching guest services)
   - Inject ServiceAccountKubeConfig for guest authentication
   - Include framework-provided availability probing and secret rotation
   - Wire the console image from the release payload via `ReleaseImageProvider`

2. **The ported dual-API console-operator** refactored to support split targets:
   - Operand (bridge Deployment/Service/ConfigMap/Secret) on the management cluster
   - Config CRs (Console, ConsolePlugin, Routes, OAuthClient) on the guest cluster
   - Dual kubeconfig inputs (`--kubeconfig` for management, `--guest-kubeconfig` for guest)
   - Configurable operand namespace parameter
   - Upstream-acceptable framing as "manage console on a remote cluster" (not HyperShift-specific)

3. **HyperShift exposure wiring** for console and downloads routes:
   - Router backend configuration for console/downloads and their `-private` variants
   - CPO-owned Route reconciliation following the KAS/OAuth model
   - Private-mode ExternalName services for external-dns PSC publishing
   - Wildcard certificate reuse from the api-server cert-manager Secret

What PR #9622 deliberately leaves out (deferred to later phases or separate work):

- **Monitoring plugin and terminal backend guest-network path**: The current implementation has the framework but needs validation at scale
- **Capability gate enforcement**: The spike runs console regardless of the Console capability setting; production needs proper gating
- **The bridge `-ca-file` fix**: Lives in the console repo (PR #17185), not HyperShift

## Custom console image

The proof-of-concept ran a custom-built console image carrying the PR #17185 patches (off-cluster CA file trust, service-ca-file support, plugin proxy fix, OIDC offline access), and a custom CPO image carrying the exposure route changes from PR #9622. Both are development affordances that disappear once the PRs merge and ship in release payloads.

The custom images were specified via the `hypershift.openshift.io/image-overrides` annotation on the HostedCluster, which flows through `ReleaseImageProvider.GetImage()` in the CPO codebase. This mechanism is already wired and required no new code.

Once the console and hypershift PRs merge and appear in consumed release payloads, deployments will use the stock release images with no overrides needed.
