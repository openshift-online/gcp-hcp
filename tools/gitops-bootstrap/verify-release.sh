#!/usr/bin/env bash
# GCP-1286 release verification entry point.
#
# Wraps an authenticated `skopeo copy` under an isolated, fail-closed
# containers/image trust policy, then checks the saved layout against the
# approved release matrix: the requested index digest is what the registry
# served, the saved manifest is the approved selected-platform child, and every
# config and layer blob that manifest references is present with intact bytes.
# A platform-selected copy saves the child manifest; a `--all` copy keeps the
# index as manifest.json. Credentials are read only from --authfile and are
# never printed, copied, or written into the work directory. The work directory
# must be new or empty and is created private (0700); an existing directory is
# never reused, so no unrelated file or symlink can be overwritten.
#
# This verifies artifact identity and signature policy. It does not establish
# GKE support, chart certification, or that a component runs (see
# release-matrix.yaml and chart-src/PROVENANCE.md).
#
# Examples:
#   ./verify-release.sh --matrix release-matrix.yaml --image manager \
#       --authfile /tmp/pull-secret --key-file /tmp/release-key2.txt \
#       --workdir /tmp/gcp-1286-verify --arch amd64
#
#   # Re-check an already copied layout without contacting a registry:
#   ./verify-release.sh --matrix release-matrix.yaml --image argocd_image \
#       --layout-dir /tmp/gcp-1286-verify/argocd_image \
#       --key-file /tmp/release-key2.txt --workdir /tmp/gcp-1286-check --arch amd64
set -euo pipefail
umask 077

TOOL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="$TOOL_DIR/verify-release.py"

usage() {
  cat >&2 <<'EOF'
Usage: verify-release.sh --matrix FILE --image NAME --workdir DIR --key-file FILE \
         (--authfile FILE | --layout-dir DIR) [--arch ARCH]

  --matrix FILE      release-matrix.yaml
  --image NAME       'bundle' or a component name from the matrix
                     (e.g. manager, argocd_image)
  --workdir DIR      New or empty directory for the trust policy and copy
                     (created private, 0700; never an existing non-empty or
                     symlinked directory)
  --key-file FILE    Red Hat release public key (ASCII-armored GPG)
  --authfile FILE    Registry credential file for skopeo (never printed)
  --layout-dir DIR   Existing skopeo `dir:` layout to check; skips the copy
  --arch ARCH        linux architecture from the matrix platforms map
                     (amd64 by default; arm64/v8 means arm64 + variant v8)

The approved digest and platform child come from the matrix. `bundle` is the
single-platform OLM metadata manifest (not a runnable image) and is checked
against `spec.bundle.reference`. The copy is rejected for an unknown image, an
excluded component, an unsupported architecture, a non-digest reference, or any
policy/signature/identity mismatch. There is no insecure-policy or TLS-bypass
option.
EOF
}

die() { echo "verify-release: $*" >&2; exit 1; }

matrix=""
image=""
workdir=""
key_file=""
authfile=""
layout_dir=""
arch="amd64"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --matrix) matrix="${2:-}"; shift 2 ;;
    --image) image="${2:-}"; shift 2 ;;
    --workdir) workdir="${2:-}"; shift 2 ;;
    --key-file) key_file="${2:-}"; shift 2 ;;
    --authfile) authfile="${2:-}"; shift 2 ;;
    --layout-dir) layout_dir="${2:-}"; shift 2 ;;
    --arch) arch="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ -n "$matrix" && -n "$image" && -n "$workdir" && -n "$key_file" ]] || {
  usage
  die "missing required argument"
}
[[ "$image" =~ ^[A-Za-z0-9_.-]+$ ]] || die "invalid image name: $image"
[[ "$image" != "." && "$image" != ".." && "$image" != -* ]] || die "invalid image name: $image"
[[ "$workdir" != -* ]] || die "invalid work directory: $workdir"
[[ -f "$matrix" ]] || die "matrix not found: $matrix"
[[ -f "$key_file" && -s "$key_file" ]] || die "key file missing or empty: $key_file"
command -v yq >/dev/null 2>&1 || die "yq is required"
command -v python3 >/dev/null 2>&1 || die "python3 is required"
if [[ -z "$layout_dir" ]]; then
  # Report a missing credential before a missing tool: the credential is the
  # operator's input, and requiring skopeo first would mask it.
  [[ -f "$authfile" && -s "$authfile" ]] || die "auth file missing or empty: ${authfile:-<unset>}"
  command -v skopeo >/dev/null 2>&1 || die "skopeo is required for a fresh copy"
fi

scope=""
if [[ "$image" == "bundle" ]]; then
  scope="bundle"
  reference="$(yq -r '.spec.bundle.reference // ""' "$matrix")"
  [[ -n "$reference" ]] || die "matrix has no bundle reference"
  index_digest="${reference##*@}"
  platform_digest=""
else
  for candidate in required conditional excluded; do
    count="$(IMAGE="$image" yq -r "[.spec.components.$candidate[] | select(.name == strenv(IMAGE))] | length" "$matrix")"
    if [[ "$count" != "0" ]]; then
      scope="$candidate"
      break
    fi
  done
  [[ -n "$scope" ]] || die "image is not in the release matrix: $image"
  if [[ "$scope" == "excluded" ]]; then
    reason="$(IMAGE="$image" yq -r \
      '.spec.components.excluded[] | select(.name == strenv(IMAGE)) | .exclusionReason' "$matrix")"
    die "image is excluded from core validation: $image ($reason)"
  fi

  reference="$(IMAGE="$image" yq -r \
    ".spec.components.$scope[] | select(.name == strenv(IMAGE)) | .reference" "$matrix")"
  index_digest="$(IMAGE="$image" yq -r \
    ".spec.components.$scope[] | select(.name == strenv(IMAGE)) | .indexDigest" "$matrix")"
  platform_digest="$(IMAGE="$image" ARCH="$arch" yq -r \
    ".spec.components.$scope[] | select(.name == strenv(IMAGE)) | .platforms[strenv(ARCH)] // \"\"" "$matrix")"
fi

[[ -n "$reference" && -n "$index_digest" ]] || die "matrix entry is incomplete: $image"
[[ "$reference" == *"@$index_digest" ]] || die "matrix reference and index digest disagree: $image"
[[ "$index_digest" == sha256:* ]] || die "matrix digests must be sha256"
if [[ "$reference" == *"@"*"@"* ]]; then
  die "reference must contain exactly one digest pin: $reference"
fi
if [[ ! "$reference" =~ ^[A-Za-z0-9._-]+(:[0-9]+)?(/[A-Za-z0-9._-]+)+@sha256:[0-9a-f]{64}$ ]]; then
  die "reference is malformed: $reference"
fi
if [[ "$scope" != "bundle" ]]; then
  [[ -n "$platform_digest" ]] || die "architecture linux/$arch is not advertised by $image; refusing to guess"
  [[ "$platform_digest" == sha256:* ]] || die "matrix digests must be sha256"
fi

if [[ -e "$workdir" || -L "$workdir" ]]; then
  [[ ! -L "$workdir" ]] || die "--workdir must not be a symlink: $workdir"
  [[ -d "$workdir" ]] || die "--workdir is not a directory: $workdir"
  [[ -z "$(ls -A -- "$workdir")" ]] || die "--workdir must be new or empty: $workdir"
else
  mkdir -p -- "$workdir"
fi
chmod 0700 "$workdir"
workdir="$(cd -- "$workdir" && pwd -P)"
policy="$workdir/policy.json"
registries_dir="$workdir/registries.d"
mkdir -m 0700 -- "$registries_dir"

python3 - "$policy" "$key_file" <<'PY'
import json, os, sys
policy, key = sys.argv[1], os.path.abspath(sys.argv[2])
json.dump({
    "default": [{"type": "reject"}],
    "transports": {"docker": {"registry.redhat.io": [{
        "type": "signedBy",
        "keyType": "GPGKeys",
        "keyPath": key,
        "signedIdentity": {"type": "matchRepoDigestOrExact"},
    }]}},
}, open(policy, "x"), indent=2)
open(policy, "a").write("\n")
PY
chmod 0600 "$policy"
cat >"$registries_dir/redhat.yaml" <<'EOF'
docker:
  registry.redhat.io:
    sigstore: https://registry.redhat.io/containers/sigstore
EOF

python3 "$CHECKER" policy --policy "$policy" --key-file "$key_file" >/dev/null

if [[ -n "$layout_dir" ]]; then
  [[ -f "$layout_dir/manifest.json" ]] || die "layout has no manifest.json: $layout_dir"
  dest="$layout_dir"
else
  dest="$workdir/$image"
  [[ ! -e "$dest" ]] || die "copy destination already exists: $dest"
  arch_base="${arch%%/*}"
  variant=""
  if [[ "$arch" == */* ]]; then variant="${arch#*/}"; fi
  echo "verify-release: copying $image ($index_digest) for linux/$arch under the trust policy" >&2
  skopeo --override-os linux --override-arch "$arch_base" \
    ${variant:+--override-variant "$variant"} \
    --policy "$policy" --registries.d "$registries_dir" \
    copy --src-authfile "$authfile" --preserve-digests \
    "docker://$reference" "dir:$dest"
fi

python3 "$CHECKER" layout --dir "$dest" --reference "$reference" \
  --arch "$arch" --platform-digest "$platform_digest"

if [[ "$scope" == "bundle" ]]; then
  echo "verify-release: ok image=bundle digest=$index_digest (single OLM metadata manifest; not deployable)"
else
  echo "verify-release: ok image=$image digest=$index_digest arch=linux/$arch platform=$platform_digest"
fi
echo "verify-release: layout retained at $dest (remove it explicitly when evidence is no longer needed)"
