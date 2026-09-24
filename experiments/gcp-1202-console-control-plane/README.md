# GCP-1202: OpenShift console on the control plane

Spike evaluating whether the OpenShift web console can run **control-plane-side**
— inside the HostedControlPlane namespace on the GKE management cluster —
instead of on the data plane behind a customer-managed IngressController.

- Jira: [GCP-1202](https://redhat.atlassian.net/browse/GCP-1202)
- Feeds: [GCP-926](https://redhat.atlassian.net/browse/GCP-926) — Define and Implement Customer Ingress Strategy for GCP HCP

## Verdict

**Feasible, and demonstrated live.** A full console — including the pod
terminal, monitoring, and dynamic plugins such as alerting and dashboards — was
run from the HostedControlPlane namespace against a real GCP HostedCluster, in
both `PublicAndPrivate` and `Private` endpoint-access modes.

The core console turns out to be a stateless Go reverse proxy that needs only
the guest kube-apiserver and an auth issuer. Both are already reachable from the
HCP namespace, and the guest kube-apiserver is an ordinary in-namespace
ClusterIP Service — so the core console needs no tunnel at all. It is exposed on
the existing shared HAProxy router and public LB / PSC rails, on
`console.DOMAIN` alongside `api.DOMAIN`, under the same wildcard certificate.

**What is not proven:** reconciliation of guest-side configuration. The
console-operator was not ported, so console configs, additional ConsolePlugins,
ConsoleCLIDownloads and client-side OIDC credentials are all hand-applied YAML
in this spike. Closing that gap means making the upstream console-operator
dual-kube-API capable — a medium-heavy mechanical refactor, scoped in
[operator-migration.md](operator-migration.md).

**The GCP OIDC console client model** was the other thing holding this back: the
bridge needs its own Google OAuth web-application client with a registered
redirect URI, Google forbids wildcard redirects, and web-client creation is not
automatable. That problem is topology-independent — it would equally affect a
data-plane console under external OIDC.

It now has a design with a shipped precedent. Because a Google client's redirect
URI is mutable and the client can pre-exist the cluster, the client ID goes into
`HostedCluster.spec` at creation (in both `issuer.audiences` and
`oidcClients[]`), and the two things that depend on the cluster hostname — the
redirect URI and the client secret — are supplied day-2 by the customer, outside
the HostedCluster entirely. ARO HCP already ships exactly this shape, including
exact per-cluster redirect URIs rather than wildcards. What remains is a product
decision (the customer brings their own Google client) and three bounded code
changes. Full design in [open-questions.md](open-questions.md) §1.

## Why it is worth doing

- Removes the need for a default IngressController. Today that ingress exists
  *solely* to serve the console, and because the platform maintains it for that
  purpose, customers cannot own their own day-2 ingress.
- The console is available on day 0, and on a cluster with zero worker nodes —
  much like the API.
- Certificate management stops depending on guest-side configuration.
- The component can be given a real SLO, because it runs on infrastructure the
  platform controls rather than on customer-managed workers.

## Read in this order

| Document | What it covers |
|---|---|
| [findings.md](findings.md) | What was proven, phase by phase, and how it compares to the status quo |
| [architecture.md](architecture.md) | Diagrams: system topology, pod anatomy, exposure, guest-service access, certificates, auth flow, ownership |
| [operator-migration.md](operator-migration.md) | The console-operator dual-kube-API refactor: decision, rejected alternatives, effort |
| [open-questions.md](open-questions.md) | Blockers and unresolved design questions, most severe first |
| [upstream-changes.md](upstream-changes.md) | Every code change the PoC needed, with PR state and a production-vs-hack split |
| [cost-analysis.md](cost-analysis.md) | Measured resource footprint and the incremental-cost question |
| [manifests/](manifests/) | Scrubbed copy of the manifests actually deployed |
| [reference/](reference/) | Deep dives: auth options, konnectivity DNS resolution, private endpoint access, console-operator I/O spec, CNO dual-cluster precedent |

## Upstream changes

| Repo | Change | Jira | PR |
|---|---|---|---|
| openshift/console | Off-cluster TLS trust (`-ca-file`, `-service-ca-file`), OIDC offline access, proxy-aware plugin transport | GCP-1219 | [#17185](https://github.com/openshift/console/pull/17185) |
| openshift/hypershift | CPO owns console/downloads Routes, router backends, PSC ExternalName services; CVO payload strip | GCP-1202 | [#9622](https://github.com/openshift/hypershift/pull/9622) (draft/RFC) |
| openshift/hypershift | GCP worker firewall rules reconciled by CPO, including OVN-Kubernetes geneve | GCP-1221 | [#9640](https://github.com/openshift/hypershift/pull/9640) |
| openshift/hypershift | Allow Console enabled with Ingress disabled | OCPBUGS-58422 | [#8933](https://github.com/openshift/hypershift/pull/8933) |
| openshift/cluster-network-operator | Restricted-PSA compliance for network operands on SCC-less management clusters | not yet filed | — |

Full detail and classification in [upstream-changes.md](upstream-changes.md).

## Scope note

This experiment records a spike outcome. It is deliberately **not** a design
decision record — the go/no-go, and the resulting entry in
[design-decisions/](../../design-decisions/), should follow once
[GCP-926](https://redhat.atlassian.net/browse/GCP-926) settles the wider ingress
strategy this feeds into.

## Credentials

Nothing in this tree is a real credential, and nothing in it may ever become
one. The manifests are a scrubbed copy of a live PoC; every secret was excluded
and every environment-specific identifier replaced. See
[manifests/README.md](manifests/README.md) for what a reproducer must supply
themselves, and `manifests/.gitignore` for the guard rail.
