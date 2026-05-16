# Org Policy Change Request: Custom Constraint for Cloud CDN Service Agent Access

## Request Summary

We need a **custom organization constraint** that allows Cloud CDN fill service agents to
be granted IAM access on GCS buckets within the GCP HCP folder. This replaces the legacy
`iam.allowedPolicyMemberDomains` constraint with a more precise policy that preserves the
existing domain restriction while adding a targeted suffix-based exception for CDN service
agents.

The buckets themselves remain **private** — only the CDN fill service agent gets read access.
Cloud CDN then serves the documents publicly.

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

## Why a Custom Constraint

The legacy `iam.allowedPolicyMemberDomains` constraint and the managed
`iam.managed.allowedPolicyMembers` constraint both lack the ability to allow IAM bindings
by service account **suffix pattern**. Since the CDN fill service agent is
`service-PROJECT_NUMBER@cloud-cdn-fill.iam.gserviceaccount.com` and the project number
varies per region, we cannot enumerate every CDN fill SA in an allowlist.

A [custom organization constraint](https://cloud.google.com/resource-manager/docs/organization-policy/creating-managing-custom-constraints)
solves this by using the `MemberSubjectEndsWith` function to match all CDN fill service
agents regardless of project number, while preserving the existing org-member domain
restriction via `MemberInPrincipalSet`.

---

## Proposed Change

Create a custom constraint at the organization level and enforce it at the GCP HCP folder
level. The constraint allows IAM bindings where the member either:

1. **Belongs to our organization's principal set** (existing domain restriction behavior), or
2. **Is a Cloud CDN fill service agent** from any project (suffix match on
   `@cloud-cdn-fill.iam.gserviceaccount.com`)

No tags are required. No buckets are made public. No `allUsers` access is granted.

---

## Requested Action

### Step 1: Create the Custom Constraint (Org Admin)

```yaml
name: organizations/ORG_ID/customConstraints/custom.allowedPolicyMembers
resourceTypes:
  - iam.googleapis.com/AllowPolicy
methodTypes:
  - CREATE
  - UPDATE
condition: >-
  resource.bindings.all(binding,
    binding.members.all(member,
      MemberInPrincipalSet(member,
        ['//cloudresourcemanager.googleapis.com/organizations/NUMERIC_ORG_ID_1',
         '//cloudresourcemanager.googleapis.com/organizations/NUMERIC_ORG_ID_2'])
      || MemberSubjectEndsWith(member,
        ['@cloud-cdn-fill.iam.gserviceaccount.com'])
    )
  )
actionType: ALLOW
displayName: Domain-restricted sharing with Cloud CDN exception
description: >-
  Restricts IAM bindings to members within our two organizations, plus
  Cloud CDN fill service agents from any project. Required for
  GCP HCP OIDC document delivery via Cloud CDN backend buckets.
```

```bash
gcloud org-policies set-custom-constraint /path/to/constraint.yaml
```

### Step 2: Enforce the Custom Constraint at GCP HCP Folder (Org Admin)

```yaml
name: folders/GCP_HCP_FOLDER_ID/policies/custom.allowedPolicyMembers
spec:
  rules:
    - enforce: true
```

```bash
gcloud org-policies set-policy /path/to/policy.yaml
```

### Step 3: Disable Legacy Constraint at GCP HCP Folder (Org Admin)

If `iam.allowedPolicyMemberDomains` is currently enforced at the folder level, reset it
to inherit from the parent so the custom constraint takes over.

```bash
gcloud org-policies reset iam.allowedPolicyMemberDomains \
  --folder=GCP_HCP_FOLDER_ID
```

---

## GCP HCP Team Follow-Up

Once steps 1-3 are complete, the GCP HCP team will grant the Cloud CDN fill service agent
read access to OIDC buckets via Terraform. No tags, no ongoing org admin involvement required.

```bash
gcloud storage buckets add-iam-policy-binding gs://BUCKET_NAME \
  --member="serviceAccount:service-PROJECT_NUMBER@cloud-cdn-fill.iam.gserviceaccount.com" \
  --role=roles/storage.objectViewer
```

---

## Security

- **Buckets remain private** — no `allUsers` or `allAuthenticatedUsers` access is granted
- **Only CDN fill agents are exempted** — suffix match is specific to `@cloud-cdn-fill.iam.gserviceaccount.com`
- **CDN fill agents are Google-managed** — they cannot be impersonated or created by users
- **No tags required** — the constraint applies uniformly; no risk of tag misapplication
- **Existing domain restriction preserved** — `MemberInPrincipalSet` maintains the same org-member check
- **Data is inherently public** — OIDC docs contain public keys only, no secrets or PII
- **Scoped to GCP HCP folder** — other org projects are completely unaffected

---

## Limitations

- **CDN exception applies folder-wide** — the suffix-based exception allows the CDN fill SA on any resource in the folder, not just GCS buckets; mitigated by the fact that CDN fill agents only operate on backend buckets
- **Custom constraint limits** — GCP limits organizations to ~200 custom constraints; this uses one
- **Larger change** — custom constraints are a newer feature and represent a bigger change compared to the well-established tag-based approach in Plan B; requires more validation and testing

---

## Plan B: Tag-Based Alternative

If creating a custom constraint or migrating away from the legacy `iam.allowedPolicyMemberDomains`
constraint is not viable, we have a tag-based fallback approach described in
[org-policy-planB-tag-based.md](org-policy-planB-tag-based.md). The main trade-off is that the
tag-based `allowAll` exception opens tagged resources to any IAM member, not just the CDN fill
service agent — the restriction to CDN-only access is enforced operationally rather than by the
constraint itself.

---

## References

- [Custom organization policy constraints](https://cloud.google.com/resource-manager/docs/organization-policy/creating-managing-custom-constraints)
- [Custom constraint for IAM](https://cloud.google.com/iam/docs/custom-org-policy-constraints)
- [Restrict allowed policy member domains](https://cloud.google.com/resource-manager/docs/organization-policy/restricting-domains)
- [Cloud CDN backend buckets](https://cloud.google.com/cdn/docs/setting-up-cdn-with-bucket)
- [GCP-588: Public Access for GCS-hosted OIDC Issuer Documents](https://redhat.atlassian.net/browse/GCP-588)
