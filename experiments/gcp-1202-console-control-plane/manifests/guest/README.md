# In-guest-cluster API resources

Resources applied to the **guest** (hosted) cluster's kube-API, not the
management cluster. Everything else under `console/` targets the management
cluster's HCP namespace; these target the guest.

| File | What |
|------|------|
| `oc-cli-downloads.yaml` | The `oc-cli-downloads` ConsoleCLIDownload CR the console UI's "Command Line Tools" page reads. Hand-applied equivalent of what console-operator would generate, pointing at our control-plane-side downloads server. Still hand-applied because the console-operator (which normally creates it) is stripped; other CLI-download CRs (helm, netobserv) come from their own operators/CVO. |
| `console-admin.yaml` | ClusterRoleBinding granting cluster-admin to **one named** Google OIDC identity. Required because an externally-OIDC-authenticated guest cluster has no default RBAC for those identities. Bound to an explicit User rather than a domain group on purpose — see the comment in the file. |
| `allow-geneve-firewall.sh` | GCP firewall workaround allowing OVN-Kubernetes geneve (UDP/6081) between guest worker nodes. GCP's implied-deny broke all cross-node pod networking; this script creates the missing firewall rule. Productized in openshift/hypershift#9640 (GCP-1221). |

The `consoleclidownloads.console.openshift.io` CRD is no longer carried here: with the
`Console` capability enabled, CVO installs it automatically (it carries
`capability.openshift.io/name: Console`).

Apply with the guest kubeconfig (supply via the KUBECONFIG environment variable):

```bash
KUBECONFIG=/path/to/guest-kubeconfig ./apply.sh
KUBECONFIG=/path/to/guest-kubeconfig kubectl apply -f console-admin.yaml
# Run firewall workaround if cross-node pod networking is broken:
./allow-geneve-firewall.sh
```
