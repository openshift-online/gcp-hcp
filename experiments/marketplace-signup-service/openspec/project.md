# Marketplace Sign-Up Service (Interim)

Interim self-service bootstrap for GCP-HCP: lets a Red Hat user sign in with
their Google Workspace identity, prove owner-class control of a GCP project, and
receive the initial `service-admin` RoleBinding in their namespace — standing in
for the future Google Cloud Marketplace controller until the marketplace
integration (CIAM identity federation, EGW entitlement correlation) is available.

Only identities from Red Hat's Google Workspace domain (`hd == redhat.com`) are
accepted; external customers are explicitly out of scope for the interim.

## Source Material

- Study: [gcp-marketplace-pubsub-and-customer-identity](../../../studies/marketplace/gcp-marketplace-pubsub-and-customer-identity.md) — why the customer email is not available in the Pub/Sub/EGW pipeline, and the documented Google frontend path
- Cedar authorization model: `implementation-plans/gcp-cedar-public-api-authorization.md` in the gcp-hcp repo — `RoleBinding.Spec.Subject` is a user email; principals are Google identities
- Deployment lives in the [gcp-hcp-infra](https://github.com/openshift-online/gcp-hcp-infra) repo (Terraform + Helm/ArgoCD: GCS+CDN static hosting, backend on the region cluster's shared Gateway, Cloud Workflows)

## Spec Organization

Capabilities are organized by cohesive system behavior:

- `signup-frontend` — the SPA and its hosting
- `signup-authentication` — Google identity sign-in, token validation, and the Red Hat hosted-domain restriction
- `project-eligibility` — the signer must be a verified Red Hat identity; ownership verification is deferred (design D3)
- `rolebinding-provisioning` — creating the initial service-admin RoleBinding

When the marketplace integration lands, its change proposal is expected to
`MODIFY` `signup-authentication` and `project-eligibility`
(replacing GIS + project-id with the `x-gcp-marketplace-token` JWT + EGW
correlation) while leaving `rolebinding-provisioning` largely intact.
