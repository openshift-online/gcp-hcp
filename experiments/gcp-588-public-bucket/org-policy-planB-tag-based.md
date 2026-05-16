# Org Policy Change Request: Allow Cloud CDN Service Agent Access to GCS Buckets


## Request Summary

We need a **tag-based conditional exception** to `constraints/iam.allowedPolicyMemberDomains`
so that the Cloud CDN fill service agent can be granted read access on GCS buckets hosting
OIDC discovery documents. This is a hard requirement for Workload Identity Federation (WIF)
on our hosted control planes.

The buckets themselves remain **private** — only the CDN fill service agent gets read access.
Cloud CDN then serves the documents publicly.

This is the approach
[documented by Google](https://cloud.google.com/resource-manager/docs/organization-policy/restricting-domains#share-other-data-publicly)
for exempting specific resources from domain-restricted sharing policies.

---

## Background

GCP HCP (Hypershift on GKE) provisions hosted OpenShift control planes on GKE. Each cluster
uses **Workload Identity Federation (WIF)** so workloads can authenticate to GCP services.
WIF requires two publicly accessible OIDC documents per cluster:

- `/.well-known/openid-configuration` -- OIDC discovery document
- `/openid/v1/jwks` -- JSON Web Key Set (public keys only)

GCP Security Token Service (STS) fetches these documents via **unauthenticated HTTPS GET**.
There is no way to grant STS private access — the documents must be publicly reachable.

We store these documents in **private GCS buckets** (one per region, shared across all clusters
in that region) and serve them through **Cloud CDN with a backend bucket**. Cloud CDN uses a
Google-managed service agent (`service-PROJECT_NUMBER@cloud-cdn-fill.iam.gserviceaccount.com`)
to read objects from the bucket and populate its cache. The current
`constraints/iam.allowedPolicyMemberDomains` org policy blocks us from granting this service
agent `roles/storage.objectViewer` on the bucket:

```
ERROR: HTTPError 412: One or more users named in the policy do not belong to a permitted customer.
```

The documents are inherently public data — they contain no secrets, credentials, or PII.

---

## Proposed Change

GCP supports [tag-based conditional enforcement](https://cloud.google.com/resource-manager/docs/organization-policy/tags-organization-policy)
of organization policies. We propose adding a conditional `allowAll: true` rule to the
existing `iam.allowedPolicyMemberDomains` policy at the **GCP HCP folder level** (not
org-wide). This rule would only activate on GCS buckets explicitly tagged with
`oidc-public-access: enabled`, allowing the CDN fill service agent to be granted read
access on those specific buckets. All other resources in the folder remain subject to the
existing domain restriction.

While the tag-based exception technically allows any member on the tagged resource, we will
only grant IAM to the Cloud CDN fill service agent — no `allUsers` or `allAuthenticatedUsers`
bindings are created.

---

## Requested Action

### Step 1: Create the Tag (Org Admin)

Create an organization-level tag key/value pair

```bash
gcloud resource-manager tags keys create oidc-public-access \
  --parent=organizations/ORGANIZATION_ID

gcloud resource-manager tags values create enabled \
  --parent=ORGANIZATION_ID/oidc-public-access \
  --description="Allow CDN service agent IAM bindings for OIDC buckets"
```

### Step 2: Add Conditional Rule to Org Policy (Org Admin)

Add a conditional `allowAll: true` rule to the existing policy at the GCP HCP folder level

```yaml
name: folders/GCP_HCP_FOLDER_ID/policies/iam.allowedPolicyMemberDomains
spec:
  rules:
  - allowAll: true
    condition:
      expression: "resource.matchTag('ORGANIZATION_ID/oidc-public-access', 'enabled')"
      title: allowOidcCdnAccess
      description: >-
        Allow IAM bindings on resources tagged with oidc-public-access: enabled.
        Used for OIDC discovery document buckets that require Cloud CDN service
        agent read access for WIF token validation.
  - values:
      allowedValues:
      - "is:C04j7mbwl"  # Existing: Red Hat Google Workspace customer ID
```

```bash
gcloud org-policies set-policy /path/to/policy.yaml
```

### Step 3: Grant Tag User Role (Org Admin)

Grant `Tag User` role on the `oidc-public-access` tag key to the GCP HCP Atlantis service
account and infra team, scoped to GCS buckets only via an IAM condition. This ensures the
tag can only be applied to storage buckets, not to other resource types.

```bash
gcloud resource-manager tags keys add-iam-policy-binding \
  ORGANIZATION_ID/oidc-public-access \
  --member="serviceAccount:atlantis@GCP_HCP_GLOBAL_PROJECT.iam.gserviceaccount.com" \
  --role="roles/resourcemanager.tagUser" \
  --condition="expression=resource.type == 'storage.googleapis.com/Bucket',title=buckets_only,description=Allow tagging only for GCS buckets"
```

---

## GCP HCP Team Follow-Up

Once steps 1-3 are complete, the GCP HCP team will tag the OIDC buckets and grant the
Cloud CDN fill service agent read access via Terraform. No ongoing org admin involvement
is required.

**Tag the OIDC bucket:**

```bash
gcloud resource-manager tags bindings create \
  --tag-value=ORGANIZATION_ID/oidc-public-access/enabled \
  --parent=//storage.googleapis.com/projects/_/buckets/BUCKET_NAME \
  --location=REGION
```

**Grant CDN fill service agent read access on the tagged bucket:**

```bash
gcloud storage buckets add-iam-policy-binding gs://BUCKET_NAME \
  --member="serviceAccount:service-PROJECT_NUMBER@cloud-cdn-fill.iam.gserviceaccount.com" \
  --role=roles/storage.objectViewer
```

---

## Security

- **Buckets remain private** — no `allUsers` or `allAuthenticatedUsers` access is granted
- **Only CDN fill agents get access** — the IAM binding is scoped to `@cloud-cdn-fill.iam.gserviceaccount.com`
- **CDN fill agents are Google-managed** — they cannot be impersonated or created by users
- **Only explicitly tagged buckets are affected** — untagged resources remain fully restricted
- **Tag application is IAM-controlled** — only principals with `Tag User` role can apply the tag
- **Data is inherently public** — OIDC docs contain public keys only, no secrets or PII
- **Scoped to GCP HCP folder** — other org projects are completely unaffected

---

## Limitations

- **`allowAll` is broad** — the tag-based exception allows any member (not just CDN agents) on tagged resources; we mitigate this operationally by only granting the CDN fill SA, but the constraint itself does not enforce this
- **Per-bucket tagging required** — every new OIDC bucket must be explicitly tagged before the CDN fill SA can be granted access
- **Tags cannot be scoped to resource types** — the `oidc-public-access` tag key can be applied to any resource that supports tags, not just GCS buckets; mitigated by scoping the `Tag User` role with an IAM condition (`resource.type == 'storage.googleapis.com/Bucket'`) to restrict tagging to GCS buckets only

---

## References

- [Share data publicly with domain restricted sharing](https://cloud.google.com/resource-manager/docs/organization-policy/restricting-domains#share-other-data-publicly)
- [Scope organization policies with tags](https://cloud.google.com/resource-manager/docs/organization-policy/tags-organization-policy)
- [Restrict allowed policy member domains](https://cloud.google.com/resource-manager/docs/organization-policy/restricting-domains)
- [Cloud CDN backend buckets](https://cloud.google.com/cdn/docs/setting-up-cdn-with-bucket)
- [GCP-588: Public Access for GCS-hosted OIDC Issuer Documents](https://redhat.atlassian.net/browse/GCP-588)