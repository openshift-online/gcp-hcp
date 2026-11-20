#!/usr/bin/env bash
# Offline checks for the GCP-1286 release verification entry point
# (verify-release.sh / verify-release.py). No network, no registry, no
# credentials: fixtures are synthetic manifests and policies.
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOL_DIR="$(cd "$TESTS_DIR/.." && pwd)"
CHECK="$TOOL_DIR/verify-release.py"
WRAP="$TOOL_DIR/verify-release.sh"
MATRIX="$TOOL_DIR/release-matrix.yaml"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gcp-1286-verify-tests.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); echo "PASS: $1"; }
bad() { FAIL=$((FAIL + 1)); echo "FAIL: $1" >&2; }

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    ok "$desc"
  else
    bad "$desc (expected [$expected], got [$actual])"
  fi
}

expect_fail() {
  local desc="$1" expected="$2"; shift 2
  local log="$TMP_DIR/out.log"
  if "$@" >"$log" 2>&1; then
    bad "$desc (unexpected success)"; cat "$log" >&2
  elif grep -qF "$expected" "$log"; then
    ok "$desc"
  else
    bad "$desc (wrong error)"; cat "$log" >&2
  fi
}

expect_pass() {
  local desc="$1"; shift
  local log="$TMP_DIR/out.log"
  if "$@" >"$log" 2>&1; then ok "$desc"; else bad "$desc"; cat "$log" >&2; fi
}

KEY="$TMP_DIR/release-key.asc"
printf '%s\n' '-----BEGIN PGP PUBLIC KEY BLOCK-----' 'synthetic test key' '-----END PGP PUBLIC KEY BLOCK-----' >"$KEY"

write_policy() {
  local path="$1" key="$2" default="$3" identity="$4" transport="$5"
  python3 - "$path" "$key" "$default" "$identity" "$transport" <<'PY'
import json, sys
path, key, default, identity, transport = sys.argv[1:6]
default_entry = {"type": "reject"} if default == "reject" else {"type": "insecureAcceptAnything"}
identity_block = {} if identity == "none" else {"type": identity}
json.dump({
    "default": [default_entry],
    "transports": {"docker": {transport: [{
        "type": "signedBy", "keyType": "GPGKeys", "keyPath": key,
        "signedIdentity": identity_block,
    }]}},
}, open(path, "w"), indent=2)
PY
}

GOOD_POLICY="$TMP_DIR/policy.json"
write_policy "$GOOD_POLICY" "$KEY" reject matchRepoDigestOrExact registry.redhat.io
expect_pass "fail-closed policy is accepted" \
  python3 "$CHECK" policy --policy "$GOOD_POLICY" --key-file "$KEY"

BAD_POLICY="$TMP_DIR/policy-accept-anything.json"
write_policy "$BAD_POLICY" "$KEY" accept matchRepoDigestOrExact registry.redhat.io
expect_fail "insecureAcceptAnything default is rejected" "insecureAcceptAnything" \
  python3 "$CHECK" policy --policy "$BAD_POLICY" --key-file "$KEY"

BAD_POLICY="$TMP_DIR/policy-match-exact.json"
write_policy "$BAD_POLICY" "$KEY" reject matchExact registry.redhat.io
expect_fail "signedIdentity other than matchRepoDigestOrExact is rejected" "matchRepoDigestOrExact" \
  python3 "$CHECK" policy --policy "$BAD_POLICY" --key-file "$KEY"

BAD_POLICY="$TMP_DIR/policy-wrong-registry.json"
write_policy "$BAD_POLICY" "$KEY" reject matchRepoDigestOrExact quay.io
expect_fail "policy for a different registry is rejected" "registry.redhat.io" \
  python3 "$CHECK" policy --policy "$BAD_POLICY" --key-file "$KEY"

BAD_POLICY="$TMP_DIR/policy-wrong-key.json"
write_policy "$BAD_POLICY" "$TMP_DIR/other-key.asc" reject matchRepoDigestOrExact registry.redhat.io
printf 'synthetic\n' >"$TMP_DIR/other-key.asc"
expect_fail "policy bound to a different key file is rejected" "keyPath" \
  python3 "$CHECK" policy --policy "$BAD_POLICY" --key-file "$KEY"

LAYOUT_INDEX="$TMP_DIR/layout-index"
LAYOUT_PLATFORM="$TMP_DIR/layout-platform"
python3 - "$TMP_DIR" >"$TMP_DIR/layout.out" <<'PY'
import hashlib, json, pathlib, sys
base = pathlib.Path(sys.argv[1])
def blob(raw):
    return "sha256:" + hashlib.sha256(raw).hexdigest()
manifests = []
children = {}
configs = {}
layers = {}
for arch in ("amd64", "arm64"):
    cfg = json.dumps({"architecture": arch, "os": "linux",
                      "rootfs": {"type": "layers", "diff_ids": []},
                      "config": {}}, separators=(",", ":")).encode()
    cfg_digest = blob(cfg)
    configs[arch] = (cfg_digest, cfg)
    layer_raw = f"layer-{arch}\n".encode()
    layer_digest = blob(layer_raw)
    layers[arch] = (layer_digest, layer_raw)
    child = json.dumps({
        "schemaVersion": 2,
        "mediaType": "application/vnd.oci.image.manifest.v1+json",
        "config": {"mediaType": "application/vnd.oci.image.config.v1+json",
                   "digest": cfg_digest, "size": len(cfg)},
        "layers": [{"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
                    "digest": layer_digest, "size": len(layer_raw)}],
    }, separators=(",", ":")).encode()
    child_digest = blob(child)
    children[arch] = (child_digest, child)
    platform = {"os": "linux", "architecture": arch}
    if arch == "arm64":
        platform["variant"] = "v8"
    manifests.append({
        "mediaType": "application/vnd.oci.image.manifest.v1+json",
        "digest": child_digest, "size": len(child), "platform": platform,
    })
index = {"schemaVersion": 2,
         "mediaType": "application/vnd.oci.image.index.v1+json",
         "manifests": manifests}
index_bytes = json.dumps(index, separators=(",", ":")).encode()
index_digest = blob(index_bytes)

# `--all` copy: manifest.json is the index; each platform child is
# <hex>.manifest.json; config and layer blobs are <hex>. This is the real
# `dir:` layout.
all_dir = base / "layout-index"
all_dir.mkdir(parents=True)
(all_dir / "manifest.json").write_bytes(index_bytes)
for arch, (child_digest, child) in children.items():
    (all_dir / (child_digest.split(":", 1)[1] + ".manifest.json")).write_bytes(child)
    cfg_digest, cfg = configs[arch]
    (all_dir / cfg_digest.split(":", 1)[1]).write_bytes(cfg)
    layer_digest, layer_raw = layers[arch]
    (all_dir / layer_digest.split(":", 1)[1]).write_bytes(layer_raw)

# Default platform-selected copy: manifest.json is the selected child.
one_dir = base / "layout-platform"
one_dir.mkdir(parents=True)
amd_digest, amd_child = children["amd64"]
(one_dir / "manifest.json").write_bytes(amd_child)
cfg_digest, cfg = configs["amd64"]
(one_dir / cfg_digest.split(":", 1)[1]).write_bytes(cfg)
layer_digest, layer_raw = layers["amd64"]
(one_dir / layer_digest.split(":", 1)[1]).write_bytes(layer_raw)

# Single-platform manifest (the OLM bundle shape).
single_layer = b"single-layer\n"
single_layer_digest = blob(single_layer)
single = json.dumps({
    "schemaVersion": 2,
    "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
    "config": {"mediaType": "application/vnd.docker.container.image.v1+json",
               "digest": cfg_digest, "size": len(cfg)},
    "layers": [{"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
                "digest": single_layer_digest, "size": len(single_layer)}],
}, separators=(",", ":")).encode()
single_dir = base / "layout-single"
single_dir.mkdir(parents=True)
(single_dir / "manifest.json").write_bytes(single)
(single_dir / cfg_digest.split(":", 1)[1]).write_bytes(cfg)
(single_dir / single_layer_digest.split(":", 1)[1]).write_bytes(single_layer)

print(index_digest, amd_digest, children["arm64"][0], blob(single),
      configs["amd64"][0], layers["amd64"][0], single_layer_digest)
PY
read -r INDEX_DIGEST AMD_DIGEST ARM_DIGEST SINGLE_DIGEST AMD_CFG_DIGEST AMD_LAYER_DIGEST SINGLE_LAYER_DIGEST <"$TMP_DIR/layout.out"
REF="example.com/team/app@$INDEX_DIGEST"
LAYOUT_SINGLE="$TMP_DIR/layout-single"

expect_pass "all-platform copy preserves the approved index digest" \
  python3 "$CHECK" layout --dir "$LAYOUT_INDEX" --reference "$REF" --arch amd64 --platform-digest "$AMD_DIGEST"

expect_pass "platform-selected copy matches the approved platform digest" \
  python3 "$CHECK" layout --dir "$LAYOUT_PLATFORM" --reference "$REF" --arch amd64 --platform-digest "$AMD_DIGEST"

expect_fail "wrong platform digest is rejected" "platform digest mismatch" \
  python3 "$CHECK" layout --dir "$LAYOUT_INDEX" --reference "$REF" --arch amd64 --platform-digest "$ARM_DIGEST"

expect_fail "platform-selected copy with the wrong approved digest is rejected" "manifest digest mismatch" \
  python3 "$CHECK" layout --dir "$LAYOUT_PLATFORM" --reference "$REF" --arch amd64 --platform-digest "$ARM_DIGEST"

expect_fail "unadvertised architecture is rejected" "does not advertise linux/s390x" \
  python3 "$CHECK" layout --dir "$LAYOUT_INDEX" --reference "$REF" --arch s390x

expect_fail "non-digest reference is rejected" "not digest-pinned" \
  python3 "$CHECK" layout --dir "$LAYOUT_INDEX" --reference "example.com/team/app:1.0" --arch amd64

TAMPERED="$TMP_DIR/tampered"
cp -R "$LAYOUT_INDEX" "$TAMPERED"
printf 'x' >>"$TAMPERED/manifest.json"
expect_fail "tampered index content is rejected" "manifest digest mismatch" \
  python3 "$CHECK" layout --dir "$TAMPERED" --reference "$REF" --arch amd64

MISSING="$TMP_DIR/missing"
cp -R "$LAYOUT_INDEX" "$MISSING"
rm "$MISSING/${AMD_DIGEST#sha256:}.manifest.json"
expect_fail "missing platform child manifest is rejected" "platform child manifest missing" \
  python3 "$CHECK" layout --dir "$MISSING" --reference "$REF" --arch amd64

CORRUPT="$TMP_DIR/corrupt"
cp -R "$LAYOUT_INDEX" "$CORRUPT"
printf 'x' >>"$CORRUPT/${AMD_DIGEST#sha256:}.manifest.json"
expect_fail "corrupt platform child manifest is rejected" "does not hash to" \
  python3 "$CHECK" layout --dir "$CORRUPT" --reference "$REF" --arch amd64

MISSING_CFG="$TMP_DIR/missing-config"
cp -R "$LAYOUT_PLATFORM" "$MISSING_CFG"
rm "$MISSING_CFG/${AMD_CFG_DIGEST#sha256:}"
expect_fail "layout with a missing config blob is rejected" "required blob missing" \
  python3 "$CHECK" layout --dir "$MISSING_CFG" --reference "$REF" --arch amd64 --platform-digest "$AMD_DIGEST"

CORRUPT_CFG="$TMP_DIR/corrupt-config"
cp -R "$LAYOUT_PLATFORM" "$CORRUPT_CFG"
printf 'x' >>"$CORRUPT_CFG/${AMD_CFG_DIGEST#sha256:}"
expect_fail "layout with a corrupt config blob is rejected" "does not hash to" \
  python3 "$CHECK" layout --dir "$CORRUPT_CFG" --reference "$REF" --arch amd64 --platform-digest "$AMD_DIGEST"

MISSING_LAYER="$TMP_DIR/missing-layer"
cp -R "$LAYOUT_INDEX" "$MISSING_LAYER"
rm "$MISSING_LAYER/${AMD_LAYER_DIGEST#sha256:}"
expect_fail "indexed layout with a missing selected-platform layer is rejected" "required blob missing" \
  python3 "$CHECK" layout --dir "$MISSING_LAYER" --reference "$REF" --arch amd64 --platform-digest "$AMD_DIGEST"

MISSING_SINGLE_LAYER="$TMP_DIR/missing-single-layer"
cp -R "$LAYOUT_SINGLE" "$MISSING_SINGLE_LAYER"
rm "$MISSING_SINGLE_LAYER/${SINGLE_LAYER_DIGEST#sha256:}"
expect_fail "single-manifest layout with a missing layer is rejected" "required blob missing" \
  python3 "$CHECK" layout --dir "$MISSING_SINGLE_LAYER" --reference "example.com/team/app@$SINGLE_DIGEST" --arch amd64

expect_fail "duplicate digest pin is rejected in layout mode" "exactly one digest pin" \
  python3 "$CHECK" layout --dir "$LAYOUT_PLATFORM" --reference "$REF@$INDEX_DIGEST" --arch amd64

# Shell entry point against synthetic and real matrices (layout mode only).
SYNTH_MATRIX="$TMP_DIR/synthetic-matrix.yaml"
cat >"$SYNTH_MATRIX" <<EOF
apiVersion: gcp-hcp.io/v1alpha1
kind: GitOpsReleaseMatrix
spec:
  bundle:
    reference: example.com/team/app@$SINGLE_DIGEST
    mediaType: application/vnd.docker.distribution.manifest.v2+json
  components:
    required:
      - name: fake
        reference: $REF
        indexDigest: $INDEX_DIGEST
        platforms:
          amd64: $AMD_DIGEST
    conditional: []
    excluded:
      - name: backend_image
        exclusionReason: "no Config API on GKE"
EOF

expect_pass "entry point checks the bundle metadata manifest" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image bundle --layout-dir "$LAYOUT_SINGLE" \
  --key-file "$KEY" --workdir "$TMP_DIR/work-bundle" --arch amd64

expect_fail "entry point rejects a bundle whose bytes changed" "manifest digest mismatch" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image bundle --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/work-bundle-bad" --arch amd64

expect_pass "entry point checks an existing layout" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image fake --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/work1" --arch amd64

mkdir -p "$TMP_DIR/workdir-nonempty"
printf 'keep\n' >"$TMP_DIR/workdir-nonempty/keep.txt"
expect_fail "entry point refuses an existing non-empty work directory" "must be new or empty" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image fake --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/workdir-nonempty" --arch amd64
assert_eq "existing non-empty work directory is preserved" "keep" \
  "$(cat "$TMP_DIR/workdir-nonempty/keep.txt")"

ln -s "$TMP_DIR/workdir-nonempty" "$TMP_DIR/workdir-link"
expect_fail "entry point refuses a symlinked work directory" "must not be a symlink" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image fake --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/workdir-link" --arch amd64

printf 'x\n' >"$TMP_DIR/workdir-is-file"
expect_fail "entry point refuses a work directory path that is a file" "is not a directory" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image fake --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/workdir-is-file" --arch amd64

# Exercise the fresh-copy branch without a registry: a fake skopeo writes the
# synthetic layout and fails if the source reference carries more than one
# digest (which the real copy command must not do).
mkdir -p "$TMP_DIR/bin"
cat >"$TMP_DIR/bin/skopeo" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
args=("$@"); n=${#args[@]}
src="${args[$((n - 2))]}"; dest="${args[$((n - 1))]}"
[[ "$src" == docker://* ]] || { echo "unexpected source: $src" >&2; exit 1; }
[[ "$dest" == dir:* ]] || { echo "unexpected destination: $dest" >&2; exit 1; }
count="$(printf '%s' "$src" | grep -o '@sha256:' | wc -l | tr -d ' ')"
[[ "$count" == "1" ]] || { echo "source must carry exactly one digest: $src" >&2; exit 1; }
cp -R "$FAKE_LAYOUT" "${dest#dir:}"
FAKE
chmod +x "$TMP_DIR/bin/skopeo"
printf '{}\n' >"$TMP_DIR/auth.json"
cat >"$TMP_DIR/duplicate-matrix.yaml" <<EOF
apiVersion: gcp-hcp.io/v1alpha1
kind: GitOpsReleaseMatrix
spec:
  components:
    required:
      - name: fake
        reference: $REF@$INDEX_DIGEST
        indexDigest: $INDEX_DIGEST
        platforms:
          amd64: $AMD_DIGEST
    conditional: []
    excluded: []
EOF
expect_pass "entry point copies and re-checks under the generated policy" \
  env PATH="$TMP_DIR/bin:$PATH" FAKE_LAYOUT="$LAYOUT_PLATFORM" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image fake --authfile "$TMP_DIR/auth.json" \
  --key-file "$KEY" --workdir "$TMP_DIR/work-copy" --arch amd64

expect_fail "entry point refuses a duplicate-digest source" "exactly one digest" \
  env PATH="$TMP_DIR/bin:$PATH" FAKE_LAYOUT="$LAYOUT_PLATFORM" \
  "$WRAP" --matrix "$TMP_DIR/duplicate-matrix.yaml" --image fake \
  --authfile "$TMP_DIR/auth.json" --key-file "$KEY" --workdir "$TMP_DIR/work-dup" --arch amd64

expect_fail "entry point rejects a tampered layout" "manifest digest mismatch" \
  "$WRAP" --matrix "$SYNTH_MATRIX" --image fake --layout-dir "$TAMPERED" \
  --key-file "$KEY" --workdir "$TMP_DIR/work2" --arch amd64

expect_fail "entry point rejects an excluded component" "excluded from core validation" \
  "$WRAP" --matrix "$MATRIX" --image backend_image --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/work3"

expect_fail "entry point rejects an unknown component" "not in the release matrix" \
  "$WRAP" --matrix "$MATRIX" --image not-a-component --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/work4"

expect_fail "entry point refuses an unadvertised architecture" "not advertised" \
  "$WRAP" --matrix "$MATRIX" --image manager --layout-dir "$LAYOUT_PLATFORM" \
  --key-file "$KEY" --workdir "$TMP_DIR/work5" --arch riscv64

expect_fail "entry point requires an auth file for a fresh copy" "auth file missing" \
  "$WRAP" --matrix "$MATRIX" --image manager --key-file "$KEY" --workdir "$TMP_DIR/work6"

# Matrix integrity: the 15 references must match the reviewed allowlist, every
# entry must state why it is in or out, support approval must not be pre-filled,
# and any installation acceptance must name its approver.
matrix_refs="$(yq -r '[.spec.components.required[], .spec.components.conditional[], .spec.components.excluded[]] | .[].reference' "$MATRIX" | sort)"
allow_refs="$(grep -v '^#' "$TOOL_DIR/allowed-images.txt" | grep -v '^[[:space:]]*$' | sort)"
if [[ "$matrix_refs" == "$allow_refs" ]]; then
  ok "matrix and allowed-images.txt carry the same references"
else
  bad "matrix and allowed-images.txt references differ"
fi

missing_reason="$(yq -r '[.spec.components.excluded[] | select((.exclusionReason // "") == "")] | length' "$MATRIX")"
assert_eq "every excluded component states a reason" 0 "$missing_reason"
missing_inclusion="$(yq -r '[.spec.components.required[] | select((.inclusionReason // "") == "")] | length' "$MATRIX")"
assert_eq "every required component states an inclusion reason" 0 "$missing_inclusion"
assert_eq "support approval is not pre-filled" pending "$(yq -r '.spec.support.approval' "$MATRIX")"
assert_eq "installation approval is recorded" approved "$(yq -r '.spec.installation.approval' "$MATRIX")"
installation_approver="$(yq -r '.spec.installation.approvedBy // ""' "$MATRIX")"
if [[ -n "$installation_approver" ]]; then
  ok "installation approval names an approver"
else
  bad "installation approval has no approver"
fi
installation_scope="$(yq -r '.spec.installation.approvalScope // ""' "$MATRIX")"
if [[ -n "$installation_scope" ]]; then
  ok "installation approval states its scope"
else
  bad "installation approval has no scope"
fi

# Contract checks: the matrix must cover exactly the CSV's image references
# (manager image plus RELATED_IMAGE/* env) and the packaged CRDs must match the
# documented adaptation.
CSV="$TOOL_DIR/chart-src/upstream/gitops-operator.v1.22.2.clusterserviceversion.yaml"
csv_refs="$(yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[] | select(.name=="manager") | [.image, ((.env // [])[] | select(.name | test("IMAGE")) | .value)][]' "$CSV" | sort -u)"
assert_eq "matrix covers exactly the CSV image references" "$csv_refs" "$matrix_refs"
assert_eq "CSV defines only the manager Deployment (no backend)" \
  openshift-gitops-operator-controller-manager \
  "$(yq -r '[.spec.install.spec.deployments[].name] | join(",")' "$CSV")"

CRDS_TGZ="$TOOL_DIR/charts/gitops-operator-crds-0.1.0.tgz"
assert_eq "exactly eight CRDs are packaged" 8 \
  "$(tar tzf "$CRDS_TGZ" | grep -c '/crds/.*\.yaml$')"
tar xzf "$CRDS_TGZ" -C "$TMP_DIR"
ARGOCD_CRD="$(find "$TMP_DIR" -name 'argocds.argoproj.io.yaml' -print -quit)"
assert_eq "ArgoCD CRD is v1beta1-only" v1beta1 \
  "$(yq -r '[.spec.versions[].name] | join(",")' "$ARGOCD_CRD")"
assert_eq "ArgoCD CRD stores v1beta1" v1beta1 \
  "$(yq -r '.spec.versions[] | select(.storage==true) | .name' "$ARGOCD_CRD")"
assert_eq "ArgoCD CRD has no conversion webhook" none \
  "$(yq -r '.spec.conversion.strategy // "none"' "$ARGOCD_CRD")"

# Privileged controller allowlist: every identity bound to a rendered
# ClusterRole with escalate, bind, */*/* or the built-in cluster-admin role
# must be listed in docs/privileged-controller-allowlist.yaml. The allowlist
# is deliberately identity-only. The register that explains why each identity
# is accepted lives in Confluence, not in this repository.
PRIVILEGED_ALLOWLIST="${PRIVILEGED_ALLOWLIST:-$TOOL_DIR/../../docs/privileged-controller-allowlist.yaml}"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-gitops-operator}"
helm template gitops-operator "$TOOL_DIR/chart-src/gitops-operator" \
  --namespace "$OPERATOR_NAMESPACE" >"$TMP_DIR/operator-rbac.yaml"
yq -o=json -I=0 '.' "$PRIVILEGED_ALLOWLIST" >"$TMP_DIR/allowlist.json"
yq -o=json -I=0 '.' "$TOOL_DIR/../../bootstrap/argocd/install.yaml" >"$TMP_DIR/install.jsonl"
yq -o=json -I=0 '.' "$TMP_DIR/operator-rbac.yaml" >"$TMP_DIR/operator.jsonl"

# GCP-1433: the scoped bundle owns the application controller's ClusterRoles, so
# its binding must be observed here too. Rendering the fixture assembly keeps
# this check honest for roles that never pass through the bootstrap install or
# the operator chart.
POLICY_BUNDLE="$TMP_DIR/policy-bundle"
"$TOOL_DIR/assemble.sh" --type mc --global-project-id synthetic-global-project \
  --output-dir "$POLICY_BUNDLE" --scoped-rbac \
  --controller-policy "$TOOL_DIR/testdata/controller-policy/mc-fixture.yaml" >/dev/null
kubectl kustomize "$POLICY_BUNDLE" | yq -o=json -I=0 '.' - >"$TMP_DIR/policy-bundle.jsonl"

python3 - "$TMP_DIR/allowlist.json" "$TMP_DIR/install.jsonl" "$TMP_DIR/operator.jsonl" \
  "$TMP_DIR/policy-bundle.jsonl" >"$TMP_DIR/privileged.out" <<'PY'
import json, sys

allowlist_path, *manifest_paths = sys.argv[1:]
allowlist = json.load(open(allowlist_path))
entries = allowlist.get("entries") or []
required = ("id", "identity", "kind")
missing = sorted(
    str(e.get("id") or e.get("identity") or "?")
    for e in entries
    if any(not e.get(field) for field in required) or "inRepo" not in e
)
registered = {e.get("identity") for e in entries if e.get("identity")}


def privileged(rule):
    verbs = rule.get("verbs") or []
    if "escalate" in verbs or "bind" in verbs:
        return True
    if "*" in (rule.get("nonResourceURLs") or []) and "*" in verbs:
        return True
    groups = rule.get("apiGroups") or []
    resources = rule.get("resources") or []
    return "*" in groups and "*" in resources and "*" in verbs


observed = {}
for path in manifest_paths:
    docs = [json.loads(line) for line in open(path)
            if line.strip() and line.strip() != "null"]
    roles = {
        d["metadata"]["name"]
        for d in docs
        if d.get("kind") == "ClusterRole"
        and any(privileged(r) for r in (d.get("rules") or []))
    }
    for d in docs:
        if d.get("kind") != "ClusterRoleBinding":
            continue
        ref = (d.get("roleRef") or {}).get("name")
        if ref not in roles and ref != "cluster-admin":
            continue
        for subject in d.get("subjects") or []:
            if subject.get("kind") == "ServiceAccount":
                identity = f"{subject.get('namespace', 'default')}/{subject['name']}"
            else:
                identity = f"{subject.get('kind')}:{subject['name']}"
            observed[identity] = ref

unregistered = sorted(set(observed) - registered)
stale = sorted(
    e["identity"]
    for e in entries
    if e.get("inRepo") and e.get("identity") not in observed
)
print("|".join(unregistered))
print("|".join(stale))
print("|".join(missing))
PY

{ IFS= read -r unregistered; IFS= read -r stale; IFS= read -r missing_fields; } \
  <"$TMP_DIR/privileged.out"
assert_eq "every privileged rendered controller is allowlisted" "" "$unregistered"
assert_eq "every in-repo allowlist entry is still observed" "" "$stale"
assert_eq "every allowlist entry has the required fields" "" "$missing_fields"

echo "gitops-bootstrap release verification checks: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
