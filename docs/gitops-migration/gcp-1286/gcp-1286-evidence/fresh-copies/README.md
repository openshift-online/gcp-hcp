# Fresh registry copy verification (2026-11-10)

Sanitized results of copying the bundle and the four required images from
`registry.redhat.io`, for linux/amd64 and linux/arm64/v8, through
`tools/gitops-bootstrap/verify-release.sh`. This closes the fresh-copy gap
recorded in `docs/GITOPS_RELEASE_VALIDATION.md`; it is artifact identity and
signature evidence only, not GKE support, QE certification or runtime
acceptance.

## Method

- `skopeo copy` ran under the generated trust policy (reject default, Red Hat
  release key 2, `matchRepoDigestOrExact`, sigstore endpoint configured), so a
  missing or wrong signature on the selected platform manifest fails the copy.
  Index signature verification is the separate `standalone-verify` check
  recorded in `all-image-verification.json`.
- The checker confirmed the source index digest, the approved selected platform
  digest, and the hash of every config and layer blob that manifest references.
- Credentials came from the commons-dev pull secret as `--authfile`; no
  credential content is stored in this directory. The copied layouts were
  deleted after these JSON results were produced.
- The bundle is a single-platform OLM metadata manifest; the same bundle copy
  serves both architectures. Its verified content layer was extracted once and
  used as `GITOPS_BUNDLE_DIR` for the assembly suite, which then passed its
  byte-level CRD parity checks (179 checks total).

## Results

Per-architecture directories contain the aggregated `summary.json`. The
per-artifact checker JSON and nested checksums were not retained; re-run the
checker as described in `GCP-1286_PLAN.md` to reproduce them. `blobsVerified`
counts the config plus layer blobs hashed for that artifact.

| Artifact | Index digest | amd64 child | arm64/v8 child | blobs |
| --- | --- | --- | --- | --- |
| bundle | `sha256:2ba58c50f771…` | — | — | 2 |
| manager | `sha256:f5773d35e496…` | `sha256:7d4f3f467ec9…` | `sha256:b0dd0c85797a…` | 3 |
| argocd_image | `sha256:9aaeb618197e…` | `sha256:a82f8ec909e1…` | `sha256:742ab1d4784f…` | 3 |
| argocd_dex_image | `sha256:1294cf5c2d8b…` | `sha256:c26521bcd470…` | `sha256:f098f4696fb5…` | 3 |
| argocd_redis_image | `sha256:8e0c52a2f304…` | `sha256:84cc29e751a7…` | `sha256:638579db9c21…` | 4 |

## Limits

Not pulled: ppc64le/s390x children, the conditional Redis HA proxy image, and
any optional component. Separate platform-child signatures and release-key-3
Sigstore signatures were not tested (B3) as of 2026-11-10; key 3 was verified on
2026-11-11, see `../sigstore/`. Advertised arm64 availability is not
an approved or certified GKE architecture: that remains B1. Section B runtime
validation, including on arm64 nodes, was unstarted and unauthorized as of
2026-11-10. The amd64 run completed on 2026-11-13 and the pair was destroyed; arm64 runtime was not part of it.
