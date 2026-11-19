# Release-key-3 Sigstore signatures (2026-11-11)

Verification of the cosign/sigstore signatures Red Hat attaches to the release
artifacts, complementing the release-key-2 GPG checks in
`../fresh-copies/`. This was added after GitOps engineering indicated that
key 3 is the signing mechanism for layered product operators on
`registry.redhat.io`, and that key 2 alone is not sufficient evidence.

## Method

Sigstore signatures are stored by convention as an OCI artifact in the same
repository, tagged `sha256-<manifest-digest>.sig`. For each artifact we copied
that artifact, then for every signature layer:

1. decoded the simple-signing payload and read
   `critical.image.docker-manifest-digest`;
2. parsed the Rekor bundle, confirmed the logged hash equals the SHA-256 of the
   payload, and took the signing key from the transparency-log entry;
3. verified the signature over the payload bytes locally (RSA PKCS#1 v1.5 with
   SHA-256 for every layer that verified).

Nothing was executed and no image layer was pulled. The signature artifact copy
is small — a few hundred bytes per signature.

## Results

`results.json` holds the per-artifact detail. Summary:

| Artifact | Registry signatures | Verified | Binds approved digest |
| --- | --- | --- | --- |
| bundle | 4 | 4 | 4 |
| manager | 4 | 4 | 4 |
| argocd_image | 4 | 4 | 4 |
| argocd_dex_image | 4 | 4 | 4 |
| argocd_redis_image | 6 | 6 | 6 |

The extra signatures per artifact correspond to the release's other tags
(`1.22`, `v1.22.2`, `v1.22.2-7`, and a numeric tag); every one of them binds the
same manifest digest we approved, which is why they all count.

Each artifact also carries one build-time signature against the Konflux
repository reference (`quay.io/redhat-user-workloads/…`) with empty
certificate annotations; those are recorded but excluded from the counts above
because they do not assert the `registry.redhat.io` identity.

## Signing key

Every verified layer is signed by the same RSA key. Its SubjectPublicKeyInfo
SHA-256 is `ba9d2635…e60d446e63405576`; the trailing 16 hex digits are the
release key 3 key ID, `E60D446E63405576`, matching the published key at
<https://access.redhat.com/security/data/63405576.txt>
("Red Hat, Inc. (release key 3)"). The signatures are key-based rather than
keyless: no Fulcio certificate is attached, so the identity evidence is the
Rekor entry plus the key match above.

## Limits

- Verification is at the manifest-digest level. Individual layer blobs of the
  operator and operand images were hash-checked in `../fresh-copies/` under the
  GPG policy, not re-checked here.
- No admission-time enforcement was tested. GKE has no CRI-O
  `ClusterImagePolicy`; enforcing this at pull/admission time on a cluster is a
  separate design question, not part of artifact validation.
- Whether key 3 verification is *required* for this deployment remains a
  Product Security policy question (B3). This directory records that the
  signatures exist and verify; it does not decide the policy.
