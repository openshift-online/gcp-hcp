# Incremental control-plane cost

## Why this section exists

At the 2026-09-16 GCP HCP backlog refinement, Bill Montgomery added a
requirement to the spike: produce a rough-order-of-magnitude estimate of the
**incremental control-plane cost per hosted cluster per month**. The reasoning
was that this approach had previously been rejected by ROSA as too heavy on
cost, so the cost answer — not the technical answer — would determine whether
there was a path forward.

The technical benefits were never in dispute. This was the gating question.

## Where it landed

It was subsequently downgraded. Bill's follow-up position, after asking a model
about console resource utilisation:

> In a Hosted Control Plane (HCP) topology on an OpenShift Management Cluster
> (MC), the OpenShift Console consumes a negligible amount of resources compared
> to primary control plane components like kube-apiserver and etcd. It typically
> accounts for roughly 1% to 3% of total memory and 2% to 4% of total CPU
> requests within a hosted control plane namespace.

with the caveat that it was unverified, that it passes a sniff test, and that
the answer contained an obvious error — it implies the console already runs in
the management-cluster namespace today, which is exactly the thing this spike
is proposing.

So: cost is **most probably negligible**, and is no longer treated as gating.
The rest of this document records the measured inputs so the claim can be
checked rather than assumed.

## Measured resource requests

Taken directly from the manifests in [manifests/](manifests/) as deployed during
the PoC. These are requests, not observed usage.

| Workload | Container | CPU | Memory | Ephemeral storage |
|---|---|---|---|---|
| `console` (×2 replicas) | `console` bridge | 10m | 100Mi | — |
| | `konnectivity-socks5-proxy` sidecar | 10m | 30Mi | — |
| | `token-minter` init sidecar | 10m | 30Mi | — |
| | **per replica** | **30m** | **160Mi** | — |
| `downloads` (×2 replicas) | `download-server` | 10m | 50Mi | 6Gi |
| | `oauth-proxy` sidecar | 10m | 50Mi | — |
| | **per replica** | **20m** | **100Mi** | **6Gi** |
| | | | | |
| **Total per hosted cluster** | | **100m** | **520Mi** | **12Gi** |

Notes on those numbers:

- The bridge's 10m/100Mi is upstream's own request, carried over unmodified. It
  is a stateless Go reverse proxy; this is a realistic figure for an idle
  console and understates a console under active use.
- `replicas: 2` is a PoC choice made to prove cross-replica session recovery. A
  production deployment would make this a deliberate availability decision; at
  `replicas: 1` the totals halve.
- **The 6Gi ephemeral-storage request on the downloads pod is the one
  non-trivial line item**, and it is not a padding figure. On startup the
  download-server generates a `.tar` and `.zip` for every `oc` binary — four
  architectures across Linux, macOS and Windows, roughly 139 MB each — into an
  emptyDir, observed at about 3.2 GB. Upstream sets no ephemeral-storage
  request at all, so GKE Autopilot injects a 1Gi default limit and evicts the
  pod mid-generation, producing a continuous evict-and-replace loop. 6Gi is the
  observed footprint plus headroom. At two replicas that is 12Gi of ephemeral
  storage per hosted cluster.

## What this does *not* add

The architecturally important cost point, and the reason the figure stays small:

- **No new load balancer.** Console and downloads ride the existing shared
  HAProxy router and the existing public LB / PSC service attachment. They are
  two additional SNI backends on infrastructure that is already provisioned per
  hosted cluster.
- **No new certificate.** They reuse the wildcard certificate the API server
  already uses.
- **No new DNS zone.** Two records in a zone that already exists.
- **No new tunnel.** The konnectivity socks5 sidecar rides the konnectivity
  server that is already running.

## What it will add later

- The ported console-operator ([operator-migration.md](operator-migration.md))
  is an additional control-plane pod per hosted cluster. Upstream's
  console-operator is small, but its footprint is not included in the table
  above because it was never deployed.
- Plugin and monitoring traffic traverses the konnectivity tunnel. This is
  control-plane bandwidth and CPU that the data-plane topology does not consume,
  and it has not been load-tested. See
  [open-questions.md](open-questions.md).

## The offsetting side, and the product question

Moving the console off the data plane removes the reason the default
IngressController exists. That eliminates a guest-side load balancer and the
worker capacity the router consumes — real money, but it is *the customer's*
money, on the customer's GCP bill, while the console's new footprint is on the
management cluster and therefore on Red Hat's.

So the honest framing is not "does this cost more in total" but "does this move
cost from the customer to us, and is that worth what it buys". What it buys is
listed in [findings.md](findings.md): day-0 console availability, console on
zero-node clusters, customer freedom to own day-2 ingress, simpler certificate
management, and — most importantly — a component that can actually be given an
SLO because it runs on infrastructure we control.

A related product suggestion, making the console optional or opt-in with
customers paying more for it, was raised and met with scepticism on product
positioning grounds. It is recorded here only because it came up; it is not a
recommendation of this spike.

## To verify before this number is quoted anywhere

1. Observed usage rather than requests, for a console under real administrative
   load, including plugin traffic.
2. The console and downloads share of an actual HCP namespace's total requests,
   to confirm or refute the 1–3% memory / 2–4% CPU figure quoted above against a
   real management cluster.
3. Whether `replicas: 2` is the right production default for both operands.
4. Whether the downloads operand needs to exist per hosted cluster at all, or
   whether the 12Gi of ephemeral storage can be avoided — for example by serving
   CLI artifacts from a shared fleet-level endpoint rather than regenerating
   identical archives in every hosted control plane. This is the single largest
   saving available and it was not investigated.
5. GKE Autopilot pricing applied to the totals above, at realistic fleet scale.

Item 4 is the one worth pulling on. Everything else in the table is noise
against a kube-apiserver and an etcd.
