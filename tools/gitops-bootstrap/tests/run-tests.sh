#!/usr/bin/env bash
# Focused offline checks for tools/gitops-bootstrap/render.sh.
#
# Builds synthetic Helm chart fixtures from testdata/, packages them with
# `helm package`, and exercises render.sh (which in turn calls real helm and
# kubectl kustomize) against both passing and deliberately broken inputs.
# Nothing here touches a network or a cluster.
set -euo pipefail

TESTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOL_DIR="$(cd "$TESTS_DIR/.." && pwd)"
RENDER="$TOOL_DIR/render.sh"

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gcp-1280-render-tests.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

PASS=0
FAIL=0

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

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

expect_failure() {
  local desc="$1" expected_substr="$2" log="$3"; shift 3
  if run_render "$@" >"$log" 2>&1; then
    bad "$desc"
  else
    grep -qF "$expected_substr" "$log" && ok "$desc" \
      || { bad "$desc (wrong error)"; cat "$log" >&2; }
  fi
}

run_render() {
  "$RENDER" "$@"
}

# Package testdata/operator-chart-image-override-src with its __IMAGE_PLACEHOLDER__
# substituted for the given image, without adding a new fixture source tree
# per image value under test. Echoes the resulting .tgz path.
package_chart_with_image() {
  local image="$1" work
  work="$(mktemp -d "$TMP_DIR/image-override.XXXXXX")"
  cp -R "$TOOL_DIR/testdata/operator-chart-image-override-src" "$work/chart"
  sed -i.bak "s|__IMAGE_PLACEHOLDER__|$image|" "$work/chart/values.yaml" && rm -f "$work/chart/values.yaml.bak"
  # Package into this call's own unique directory: helm package always names
  # the archive from Chart.yaml (name+version), so a shared output directory
  # would let a later call overwrite an earlier call's still-needed archive.
  helm package "$work/chart" -d "$work" >/dev/null
  echo "$work/synthetic-operator-chart-image-override-0.1.0.tgz"
}

# --- Build fixtures ---

mkdir -p "$TMP_DIR/charts"
helm package "$TOOL_DIR/testdata/crds-chart-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/crds-chart-contaminated-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-bad-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-missing-sa-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-unsupported-kind-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-bad-operand-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-valuefrom-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-decoy-sidecar-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-no-manager-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-ambiguous-manager-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-bare-digest-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-registry-port-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-decoy-deployment-pull-secret-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-manager-pull-secret-with-decoy-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-uppercase-path-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-double-dot-path-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-triple-underscore-path-image-src" -d "$TMP_DIR/charts" >/dev/null
helm package "$TOOL_DIR/testdata/operator-chart-separator-grammar-image-src" -d "$TMP_DIR/charts" >/dev/null

CRDS_TGZ="$TMP_DIR/charts/synthetic-crds-chart-0.1.0.tgz"
CRDS_CONTAMINATED_TGZ="$TMP_DIR/charts/synthetic-crds-chart-contaminated-0.1.0.tgz"
OPERATOR_TGZ="$TMP_DIR/charts/synthetic-operator-chart-0.1.0.tgz"
BAD_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-bad-image-0.1.0.tgz"
MISSING_SA_TGZ="$TMP_DIR/charts/synthetic-operator-chart-missing-sa-0.1.0.tgz"
UNSUPPORTED_KIND_TGZ="$TMP_DIR/charts/synthetic-operator-chart-unsupported-kind-0.1.0.tgz"
BAD_OPERAND_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-bad-operand-image-0.1.0.tgz"
VALUEFROM_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-valuefrom-image-0.1.0.tgz"
DECOY_SIDECAR_TGZ="$TMP_DIR/charts/synthetic-operator-chart-decoy-sidecar-0.1.0.tgz"
NO_MANAGER_TGZ="$TMP_DIR/charts/synthetic-operator-chart-no-manager-0.1.0.tgz"
AMBIGUOUS_MANAGER_TGZ="$TMP_DIR/charts/synthetic-operator-chart-ambiguous-manager-0.1.0.tgz"
BARE_DIGEST_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-bare-digest-image-0.1.0.tgz"
REGISTRY_PORT_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-registry-port-image-0.1.0.tgz"
DECOY_DEPLOYMENT_PULL_SECRET_TGZ="$TMP_DIR/charts/synthetic-operator-chart-decoy-deployment-pull-secret-0.1.0.tgz"
MANAGER_PULL_SECRET_WITH_DECOY_TGZ="$TMP_DIR/charts/synthetic-operator-chart-manager-pull-secret-with-decoy-0.1.0.tgz"
UPPERCASE_PATH_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-uppercase-path-image-0.1.0.tgz"
DOUBLE_DOT_PATH_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-double-dot-path-image-0.1.0.tgz"
TRIPLE_UNDERSCORE_PATH_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-triple-underscore-path-image-0.1.0.tgz"
SEPARATOR_GRAMMAR_IMAGE_TGZ="$TMP_DIR/charts/synthetic-operator-chart-separator-grammar-image-0.1.0.tgz"

CRDS_SUM="$(sha256_of "$CRDS_TGZ")"
CRDS_CONTAMINATED_SUM="$(sha256_of "$CRDS_CONTAMINATED_TGZ")"
OPERATOR_SUM="$(sha256_of "$OPERATOR_TGZ")"
BAD_IMAGE_SUM="$(sha256_of "$BAD_IMAGE_TGZ")"
MISSING_SA_SUM="$(sha256_of "$MISSING_SA_TGZ")"
UNSUPPORTED_KIND_SUM="$(sha256_of "$UNSUPPORTED_KIND_TGZ")"
BAD_OPERAND_IMAGE_SUM="$(sha256_of "$BAD_OPERAND_IMAGE_TGZ")"
VALUEFROM_IMAGE_SUM="$(sha256_of "$VALUEFROM_IMAGE_TGZ")"
DECOY_SIDECAR_SUM="$(sha256_of "$DECOY_SIDECAR_TGZ")"
NO_MANAGER_SUM="$(sha256_of "$NO_MANAGER_TGZ")"
AMBIGUOUS_MANAGER_SUM="$(sha256_of "$AMBIGUOUS_MANAGER_TGZ")"
BARE_DIGEST_IMAGE_SUM="$(sha256_of "$BARE_DIGEST_IMAGE_TGZ")"
REGISTRY_PORT_IMAGE_SUM="$(sha256_of "$REGISTRY_PORT_IMAGE_TGZ")"
DECOY_DEPLOYMENT_PULL_SECRET_SUM="$(sha256_of "$DECOY_DEPLOYMENT_PULL_SECRET_TGZ")"
MANAGER_PULL_SECRET_WITH_DECOY_SUM="$(sha256_of "$MANAGER_PULL_SECRET_WITH_DECOY_TGZ")"
UPPERCASE_PATH_IMAGE_SUM="$(sha256_of "$UPPERCASE_PATH_IMAGE_TGZ")"
DOUBLE_DOT_PATH_IMAGE_SUM="$(sha256_of "$DOUBLE_DOT_PATH_IMAGE_TGZ")"
TRIPLE_UNDERSCORE_PATH_IMAGE_SUM="$(sha256_of "$TRIPLE_UNDERSCORE_PATH_IMAGE_TGZ")"
SEPARATOR_GRAMMAR_IMAGE_SUM="$(sha256_of "$SEPARATOR_GRAMMAR_IMAGE_TGZ")"
# Flip the last hex digit to produce a checksum that is well-formed but wrong.
BAD_SUM="${OPERATOR_SUM%?}$([ "${OPERATOR_SUM: -1}" = "0" ] && echo 1 || echo 0)"

write_pins() {
  local file="$1" crds_tgz="$2" crds_sum="$3" op_tgz="$4" op_sum="$5" crds_version="${6:-0.1.0}" op_version="${7:-0.1.0}"
  cat > "$file" <<EOF
crdsChart:
  path: $crds_tgz
  version: "$crds_version"
  origin: null
  digest: null
  checksum: "sha256:$crds_sum"
operatorChart:
  path: $op_tgz
  version: "$op_version"
  origin: null
  digest: null
  checksum: "sha256:$op_sum"
EOF
}

write_pins "$TMP_DIR/good-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$OPERATOR_TGZ" "$OPERATOR_SUM"
write_pins "$TMP_DIR/bad-checksum-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$OPERATOR_TGZ" "$BAD_SUM"
write_pins "$TMP_DIR/bad-version-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$OPERATOR_TGZ" "$OPERATOR_SUM" "9.9.9"
write_pins "$TMP_DIR/bad-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$BAD_IMAGE_TGZ" "$BAD_IMAGE_SUM"
write_pins "$TMP_DIR/crds-contaminated-pins.yaml" "$CRDS_CONTAMINATED_TGZ" "$CRDS_CONTAMINATED_SUM" "$OPERATOR_TGZ" "$OPERATOR_SUM"
write_pins "$TMP_DIR/missing-sa-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$MISSING_SA_TGZ" "$MISSING_SA_SUM"
write_pins "$TMP_DIR/unsupported-kind-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$UNSUPPORTED_KIND_TGZ" "$UNSUPPORTED_KIND_SUM"
write_pins "$TMP_DIR/bad-operand-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$BAD_OPERAND_IMAGE_TGZ" "$BAD_OPERAND_IMAGE_SUM"
write_pins "$TMP_DIR/valuefrom-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$VALUEFROM_IMAGE_TGZ" "$VALUEFROM_IMAGE_SUM"
write_pins "$TMP_DIR/decoy-sidecar-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$DECOY_SIDECAR_TGZ" "$DECOY_SIDECAR_SUM"
write_pins "$TMP_DIR/no-manager-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$NO_MANAGER_TGZ" "$NO_MANAGER_SUM"
write_pins "$TMP_DIR/ambiguous-manager-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$AMBIGUOUS_MANAGER_TGZ" "$AMBIGUOUS_MANAGER_SUM"
write_pins "$TMP_DIR/bare-digest-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$BARE_DIGEST_IMAGE_TGZ" "$BARE_DIGEST_IMAGE_SUM"
write_pins "$TMP_DIR/registry-port-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$REGISTRY_PORT_IMAGE_TGZ" "$REGISTRY_PORT_IMAGE_SUM"
write_pins "$TMP_DIR/decoy-deployment-pull-secret-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$DECOY_DEPLOYMENT_PULL_SECRET_TGZ" "$DECOY_DEPLOYMENT_PULL_SECRET_SUM"
write_pins "$TMP_DIR/manager-pull-secret-with-decoy-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$MANAGER_PULL_SECRET_WITH_DECOY_TGZ" "$MANAGER_PULL_SECRET_WITH_DECOY_SUM"
write_pins "$TMP_DIR/uppercase-path-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$UPPERCASE_PATH_IMAGE_TGZ" "$UPPERCASE_PATH_IMAGE_SUM"
write_pins "$TMP_DIR/double-dot-path-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$DOUBLE_DOT_PATH_IMAGE_TGZ" "$DOUBLE_DOT_PATH_IMAGE_SUM"
write_pins "$TMP_DIR/triple-underscore-path-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$TRIPLE_UNDERSCORE_PATH_IMAGE_TGZ" "$TRIPLE_UNDERSCORE_PATH_IMAGE_SUM"
write_pins "$TMP_DIR/separator-grammar-image-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$SEPARATOR_GRAMMAR_IMAGE_TGZ" "$SEPARATOR_GRAMMAR_IMAGE_SUM"

GOOD_MANAGER_IMAGE="quay.io/example/synthetic-operator@sha256:0000000000000000000000000000000000000000000000000000000000000000"
GOOD_OPERAND_IMAGE="quay.io/example/synthetic-argocd@sha256:1111111111111111111111111111111111111111111111111111111111111111"
OTHER_IMAGE="quay.io/example/synthetic-operator@sha256:2222222222222222222222222222222222222222222222222222222222222222"
REGISTRY_PORT_IMAGE="registry.example.com:5000/ns/synthetic-operator@sha256:3333333333333333333333333333333333333333333333333333333333333333"
BARE_DIGEST_IMAGE="@sha256:0000000000000000000000000000000000000000000000000000000000000000"
UPPERCASE_PATH_IMAGE="quay.io/EXAMPLE/operator@sha256:0000000000000000000000000000000000000000000000000000000000000000"
DOUBLE_DOT_PATH_IMAGE="quay.io/ex..ample/operator@sha256:0000000000000000000000000000000000000000000000000000000000000000"
TRIPLE_UNDERSCORE_PATH_IMAGE="quay.io/ex___ample/operator@sha256:0000000000000000000000000000000000000000000000000000000000000000"
SEPARATOR_GRAMMAR_IMAGE="quay.io/my-example_repo__sub/operator-two@sha256:4444444444444444444444444444444444444444444444444444444444444444"

COMMON_ARGS=(--namespace gitops-test-ns --instance-namespace argocd-test
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret
  --allowed-image "$GOOD_MANAGER_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE")

# --- Happy path: render succeeds and output is semantically correct ---

OUT1="$TMP_DIR/out1"
if run_render --pins "$TMP_DIR/good-pins.yaml" --output-dir "$OUT1" "${COMMON_ARGS[@]}" >"$TMP_DIR/out1.log" 2>&1; then
  ok "happy path render succeeds"
else
  bad "happy path render succeeds"; cat "$TMP_DIR/out1.log" >&2
fi

if [[ -f "$OUT1/crds.yaml" && -f "$OUT1/operator.yaml" ]]; then
  ok "render produces crds.yaml and operator.yaml"
else
  bad "render produces crds.yaml and operator.yaml"
fi

crd_count="$(yq eval -o=json -I=0 '.' "$OUT1/crds.yaml" | jq -s '[.[] | select(.kind == "CustomResourceDefinition")] | length')"
assert_eq "both synthetic CRDs rendered" "2" "$crd_count"

crds_only="$(yq eval -o=json -I=0 '.' "$OUT1/crds.yaml" | jq -s 'all(.[]; .kind == "CustomResourceDefinition")')"
assert_eq "crds.yaml contains only CustomResourceDefinitions" "true" "$crds_only"

top_level_status_count="$(
  { yq eval -o=json -I=0 '.' "$OUT1/crds.yaml"; yq eval -o=json -I=0 '.' "$OUT1/operator.yaml"; } |
    jq -s '[.[] | select(has("status"))] | length'
)"
assert_eq "no top-level status survives in either output file" "0" "$top_level_status_count"

nested_status_present="$(
  yq eval -o=json -I=0 '.' "$OUT1/crds.yaml" |
    jq -s '[.[] | select(.metadata.name == "widgets.example.com")][0]
      | .spec.versions[0].schema.openAPIV3Schema.properties.status.properties | has("phase")'
)"
assert_eq "nested CRD schema status property preserved" "true" "$nested_status_present"

ns_ok="$(
  yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" |
    jq -s --arg ns gitops-test-ns \
      'all(.[] | select(.kind == "Deployment" or .kind == "ServiceAccount" or .kind == "Service" or .kind == "Secret"); .metadata.namespace == $ns)'
)"
assert_eq "all namespaced operator objects use --namespace" "true" "$ns_ok"

sa_declared="$(
  yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" |
    jq -s 'any(.[] | select(.kind == "ServiceAccount"); .metadata.name == "test-operator-sa")'
)"
assert_eq "requested ServiceAccount is declared" "true" "$sa_declared"

secret_kinds="$(yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" | jq -sc '[.[] | select(.kind == "Secret") | .type]')"
assert_eq "only the service-account-token placeholder Secret is rendered (no dockerconfigjson)" '["kubernetes.io/service-account-token"]' "$secret_kinds"

secret_data_empty="$(
  yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" |
    jq -s 'all(.[] | select(.kind == "Secret"); ((.data // {}) | length == 0) and ((.stringData // {}) | length == 0))'
)"
assert_eq "service-account-token placeholder Secret carries no data" "true" "$secret_data_empty"

env_webhook="$(
  yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" |
    jq -sr '[.[] | select(.kind == "Deployment")][0].spec.template.spec.containers[0].env[] | select(.name == "ENABLE_CONVERSION_WEBHOOK") | .value'
)"
assert_eq "ENABLE_CONVERSION_WEBHOOK forced false" "false" "$env_webhook"

env_instance_ns="$(
  yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" |
    jq -sr '[.[] | select(.kind == "Deployment")][0].spec.template.spec.containers[0].env[] | select(.name == "ARGOCD_CLUSTER_CONFIG_NAMESPACES") | .value'
)"
assert_eq "ARGOCD_CLUSTER_CONFIG_NAMESPACES set to --instance-namespace" "argocd-test" "$env_instance_ns"

env_operand_image="$(
  yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" |
    jq -sr '[.[] | select(.kind == "Deployment")][0].spec.template.spec.containers[0].env[] | select(.name == "ARGOCD_IMAGE") | .value'
)"
assert_eq "approved digest-pinned operand image (ARGOCD_IMAGE) passes through" "$GOOD_OPERAND_IMAGE" "$env_operand_image"

pull_secret_name="$(
  yq eval -o=json -I=0 '.' "$OUT1/operator.yaml" |
    jq -sr '[.[] | select(.kind == "Deployment")][0].spec.template.spec.imagePullSecrets[0].name'
)"
assert_eq "imagePullSecrets[0].name set to --image-pull-secret-name" "test-pull-secret" "$pull_secret_name"

# --- Determinism: re-rendering the same inputs is byte-identical ---

OUT1B="$TMP_DIR/out1-repeat"
run_render --pins "$TMP_DIR/good-pins.yaml" --output-dir "$OUT1B" "${COMMON_ARGS[@]}" >/dev/null 2>&1 || true
if diff -rq "$OUT1" "$OUT1B" >/dev/null 2>&1; then
  ok "render is deterministic across repeated invocations"
else
  bad "render is deterministic across repeated invocations"
fi

# --- Fail-closed cases ---

expect_failure "refuses to overwrite an existing output directory" "already exists" \
  "$TMP_DIR/no-overwrite.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$OUT1" "${COMMON_ARGS[@]}"

expect_failure "rejects checksum mismatch" "checksum mismatch" \
  "$TMP_DIR/bad-checksum.log" --pins "$TMP_DIR/bad-checksum-pins.yaml" --output-dir "$TMP_DIR/out-bad-checksum" "${COMMON_ARGS[@]}"

expect_failure "rejects chart version mismatch" "version mismatch" \
  "$TMP_DIR/bad-version.log" --pins "$TMP_DIR/bad-version-pins.yaml" --output-dir "$TMP_DIR/out-bad-version" "${COMMON_ARGS[@]}"

# Required pin fields that are absent or explicitly null must be reported as
# missing (not as a "null" path/checksum), and must publish nothing.
for pair in crdsChart.path:CRDS_CHART_PATH crdsChart.version:CRDS_CHART_VERSION crdsChart.checksum:CRDS_CHART_CHECKSUM \
  operatorChart.path:OPERATOR_CHART_PATH operatorChart.version:OPERATOR_CHART_VERSION operatorChart.checksum:OPERATOR_CHART_CHECKSUM; do
  field="${pair%%:*}"; var="${pair##*:}"
  for mode in absent null; do
    tag="${field//./-}-$mode"
    if [[ "$mode" == absent ]]; then
      yq eval "del(.$field)" "$TMP_DIR/good-pins.yaml" > "$TMP_DIR/pins-$tag.yaml"
    else
      yq eval ".$field = null" "$TMP_DIR/good-pins.yaml" > "$TMP_DIR/pins-$tag.yaml"
    fi
    expect_failure "rejects a pin with $field $mode as a missing required field ($var)" \
      "pins file missing required field (resolved empty): $var" \
      "$TMP_DIR/pins-$tag.log" --pins "$TMP_DIR/pins-$tag.yaml" --output-dir "$TMP_DIR/out-pins-$tag" "${COMMON_ARGS[@]}"
    if [[ -e "$TMP_DIR/out-pins-$tag" ]]; then
      bad "no output directory published for $field $mode"
    else
      ok "no output directory published for $field $mode"
    fi
  done
done

expect_failure "rejects floating/malformed manager image reference" "floating/malformed image" \
  "$TMP_DIR/bad-image.log" --pins "$TMP_DIR/bad-image-pins.yaml" --output-dir "$TMP_DIR/out-bad-image" "${COMMON_ARGS[@]}"

expect_failure "rejects malformed namespace" "not a valid Kubernetes namespace name" \
  "$TMP_DIR/bad-ns.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-bad-ns" \
  --namespace 'Bad_NS!' --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$GOOD_MANAGER_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

OVERLONG_NS="$(printf 'a%.0s' {1..64})"
expect_failure "rejects namespace over the 63-char DNS-1123 label limit" "max 63 chars" \
  "$TMP_DIR/long-ns.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-long-ns" \
  --namespace "$OVERLONG_NS" --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$GOOD_MANAGER_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

expect_failure "rejects CRDs chart that renders a non-CRD object" "not a CustomResourceDefinition" \
  "$TMP_DIR/crds-contaminated.log" --pins "$TMP_DIR/crds-contaminated-pins.yaml" --output-dir "$TMP_DIR/out-crds-contaminated" "${COMMON_ARGS[@]}"

expect_failure "rejects operator chart that omits the requested ServiceAccount" "is not declared as a ServiceAccount" \
  "$TMP_DIR/missing-sa.log" --pins "$TMP_DIR/missing-sa-pins.yaml" --output-dir "$TMP_DIR/out-missing-sa" "${COMMON_ARGS[@]}"

expect_failure "rejects operator chart that renders an unsupported object kind" "unsupported object kind" \
  "$TMP_DIR/unsupported-kind.log" --pins "$TMP_DIR/unsupported-kind-pins.yaml" --output-dir "$TMP_DIR/out-unsupported-kind" "${COMMON_ARGS[@]}"

expect_failure "rejects floating operand image (ARGOCD_IMAGE) even with manager approved" "floating/malformed image" \
  "$TMP_DIR/bad-operand-image.log" --pins "$TMP_DIR/bad-operand-image-pins.yaml" --output-dir "$TMP_DIR/out-bad-operand-image" "${COMMON_ARGS[@]}"

expect_failure "rejects operand image env set via valueFrom instead of a literal" "does not set a literal value" \
  "$TMP_DIR/valuefrom-image.log" --pins "$TMP_DIR/valuefrom-image-pins.yaml" --output-dir "$TMP_DIR/out-valuefrom-image" "${COMMON_ARGS[@]}"

# --- Mandatory, non-bypassable image allowlist ---

NO_ALLOWLIST_ARGS=(--namespace gitops-test-ns --instance-namespace argocd-test
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret)

expect_failure "rejects render with no approved image input at all" "no approved image input supplied" \
  "$TMP_DIR/no-allowlist.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-no-allowlist" "${NO_ALLOWLIST_ARGS[@]}"

: > "$TMP_DIR/empty-allowlist.txt"
expect_failure "rejects render with an empty --image-allowlist file (no bypass)" "no approved image input supplied" \
  "$TMP_DIR/empty-allowlist-file.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-empty-allowlist" \
  "${NO_ALLOWLIST_ARGS[@]}" --image-allowlist "$TMP_DIR/empty-allowlist.txt"

expect_failure "rejects image not present in allowlist" "unreviewed image tuple" \
  "$TMP_DIR/allowlist-reject.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-allowlist-reject" \
  --namespace gitops-test-ns --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$OTHER_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

if run_render --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-allowlist-accept" "${COMMON_ARGS[@]}" \
    >"$TMP_DIR/allowlist-accept.log" 2>&1; then
  ok "accepts manager and operand images present in allowlist"
else
  bad "accepts manager and operand images present in allowlist"; cat "$TMP_DIR/allowlist-accept.log" >&2
fi

# --- Manager env checks are bound to the manager container, not any container ---

expect_failure "rejects manager missing required env even when a decoy sidecar carries it" \
  "must set exactly one env entry named ENABLE_CONVERSION_WEBHOOK (found 0" \
  "$TMP_DIR/decoy-sidecar.log" --pins "$TMP_DIR/decoy-sidecar-pins.yaml" --output-dir "$TMP_DIR/out-decoy-sidecar" "${COMMON_ARGS[@]}"

expect_failure "rejects a chart with no container named 'manager'" \
  "expected exactly one Deployment owning a container named 'manager'" \
  "$TMP_DIR/no-manager.log" --pins "$TMP_DIR/no-manager-pins.yaml" --output-dir "$TMP_DIR/out-no-manager" "${COMMON_ARGS[@]}"

expect_failure "rejects a chart with two containers ambiguously named 'manager' in one Deployment" \
  "expected exactly one container named 'manager' in its Deployment" \
  "$TMP_DIR/ambiguous-manager.log" --pins "$TMP_DIR/ambiguous-manager-pins.yaml" --output-dir "$TMP_DIR/out-ambiguous-manager" "${COMMON_ARGS[@]}"

# --- Image-reference grammar: repository required, shared check for input and output ---

expect_failure "rejects a bare @sha256 allowlist entry with no repository" \
  "approved image is not a valid fully-qualified, digest-pinned reference" \
  "$TMP_DIR/bare-allowlist.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-bare-allowlist" \
  --namespace gitops-test-ns --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$BARE_DIGEST_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

expect_failure "rejects a rendered bare @sha256 manager image with no repository" \
  "floating/malformed image" \
  "$TMP_DIR/bare-rendered.log" --pins "$TMP_DIR/bare-digest-image-pins.yaml" --output-dir "$TMP_DIR/out-bare-rendered" "${COMMON_ARGS[@]}"

OUT_REGISTRY_PORT="$TMP_DIR/out-registry-port"
if run_render --pins "$TMP_DIR/registry-port-image-pins.yaml" --output-dir "$OUT_REGISTRY_PORT" \
    --namespace gitops-test-ns --instance-namespace argocd-test \
    --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
    --allowed-image "$REGISTRY_PORT_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE" \
    >"$TMP_DIR/registry-port.log" 2>&1; then
  ok "accepts a fully-qualified registry:port/multi-segment image reference"
else
  bad "accepts a fully-qualified registry:port/multi-segment image reference"; cat "$TMP_DIR/registry-port.log" >&2
fi
rendered_registry_port_image="$(
  yq eval -o=json -I=0 '.' "$OUT_REGISTRY_PORT/operator.yaml" 2>/dev/null |
    jq -sr '[.[] | select(.kind == "Deployment")][0].spec.template.spec.containers[] | select(.name == "manager") | .image' 2>/dev/null
)"
assert_eq "registry:port image reference round-trips exactly" "$REGISTRY_PORT_IMAGE" "$rendered_registry_port_image"

# --- DNS-1123 subdomain grammar: dots are valid, malformed dot placement is not ---

DOTTED_ARGS=(--namespace gitops-test-ns --instance-namespace argocd-test
  --service-account "test.operator.sa" --image-pull-secret-name test-pull-secret
  --allowed-image "$GOOD_MANAGER_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE")
if run_render --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-dotted-sa" "${DOTTED_ARGS[@]}" \
    >"$TMP_DIR/dotted-sa.log" 2>&1; then
  ok "accepts a dotted DNS-1123 subdomain ServiceAccount name"
else
  bad "accepts a dotted DNS-1123 subdomain ServiceAccount name"; cat "$TMP_DIR/dotted-sa.log" >&2
fi

for bad_sa in ".leadingdot" "trailingdot." "double..dot" "UPPER.case"; do
  expect_failure "rejects malformed dotted ServiceAccount name: $bad_sa" "not a valid Kubernetes object name" \
    "$TMP_DIR/bad-sa-$(echo "$bad_sa" | tr -c 'a-zA-Z0-9' '_').log" \
    --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-bad-sa-$(echo "$bad_sa" | tr -c 'a-zA-Z0-9' '_')" \
    --namespace gitops-test-ns --instance-namespace argocd-test \
    --service-account "$bad_sa" --image-pull-secret-name test-pull-secret \
    --allowed-image "$GOOD_MANAGER_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"
done

# --- Pull-secret postcondition is bound to the manager's own Deployment ---

expect_failure "rejects a decoy Deployment satisfying the manager's pull-secret postcondition" \
  "the manager Deployment does not reference --image-pull-secret-name" \
  "$TMP_DIR/decoy-deployment-pull-secret.log" --pins "$TMP_DIR/decoy-deployment-pull-secret-pins.yaml" \
  --output-dir "$TMP_DIR/out-decoy-deployment-pull-secret" "${COMMON_ARGS[@]}"

if run_render --pins "$TMP_DIR/manager-pull-secret-with-decoy-pins.yaml" --output-dir "$TMP_DIR/out-manager-pull-secret-with-decoy" "${COMMON_ARGS[@]}" \
    >"$TMP_DIR/manager-pull-secret-with-decoy.log" 2>&1; then
  ok "accepts the manager's own Deployment referencing the pull secret alongside an unrelated decoy Deployment"
else
  bad "accepts the manager's own Deployment referencing the pull secret alongside an unrelated decoy Deployment"
  cat "$TMP_DIR/manager-pull-secret-with-decoy.log" >&2
fi

# --- Image repository-path grammar: lowercase components, restricted separators ---

expect_failure "rejects an uppercase repository path in an --allowed-image entry" \
  "approved image is not a valid fully-qualified, digest-pinned reference" \
  "$TMP_DIR/uppercase-allowlist.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-uppercase-allowlist" \
  --namespace gitops-test-ns --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$UPPERCASE_PATH_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

expect_failure "rejects a rendered image with an uppercase repository path" \
  "floating/malformed image" \
  "$TMP_DIR/uppercase-rendered.log" --pins "$TMP_DIR/uppercase-path-image-pins.yaml" --output-dir "$TMP_DIR/out-uppercase-rendered" "${COMMON_ARGS[@]}"

expect_failure "rejects a double-dot repository path in an --allowed-image entry" \
  "approved image is not a valid fully-qualified, digest-pinned reference" \
  "$TMP_DIR/double-dot-allowlist.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-double-dot-allowlist" \
  --namespace gitops-test-ns --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$DOUBLE_DOT_PATH_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

expect_failure "rejects a rendered image with a double-dot repository path" \
  "floating/malformed image" \
  "$TMP_DIR/double-dot-rendered.log" --pins "$TMP_DIR/double-dot-path-image-pins.yaml" --output-dir "$TMP_DIR/out-double-dot-rendered" "${COMMON_ARGS[@]}"

expect_failure "rejects a triple-underscore repository path in an --allowed-image entry" \
  "approved image is not a valid fully-qualified, digest-pinned reference" \
  "$TMP_DIR/triple-underscore-allowlist.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-triple-underscore-allowlist" \
  --namespace gitops-test-ns --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$TRIPLE_UNDERSCORE_PATH_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

expect_failure "rejects a rendered image with a triple-underscore repository path" \
  "floating/malformed image" \
  "$TMP_DIR/triple-underscore-rendered.log" --pins "$TMP_DIR/triple-underscore-path-image-pins.yaml" --output-dir "$TMP_DIR/out-triple-underscore-rendered" "${COMMON_ARGS[@]}"

OUT_SEPARATOR_GRAMMAR="$TMP_DIR/out-separator-grammar"
if run_render --pins "$TMP_DIR/separator-grammar-image-pins.yaml" --output-dir "$OUT_SEPARATOR_GRAMMAR" \
    --namespace gitops-test-ns --instance-namespace argocd-test \
    --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
    --allowed-image "$SEPARATOR_GRAMMAR_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE" \
    >"$TMP_DIR/separator-grammar.log" 2>&1; then
  ok "accepts a repository path using dash-run, single underscore, and double underscore separators"
else
  bad "accepts a repository path using dash-run, single underscore, and double underscore separators"
  cat "$TMP_DIR/separator-grammar.log" >&2
fi
rendered_separator_grammar_image="$(
  yq eval -o=json -I=0 '.' "$OUT_SEPARATOR_GRAMMAR/operator.yaml" 2>/dev/null |
    jq -sr '[.[] | select(.kind == "Deployment")][0].spec.template.spec.containers[] | select(.name == "manager") | .image' 2>/dev/null
)"
assert_eq "separator-grammar image reference round-trips exactly" "$SEPARATOR_GRAMMAR_IMAGE" "$rendered_separator_grammar_image"

# --- Image host grammar: label-shaped DNS hostnames only ---

DOUBLE_DOT_HOST_IMAGE="quay..io/example/synthetic-operator@sha256:0000000000000000000000000000000000000000000000000000000000000000"
LEADING_DASH_LABEL_HOST_IMAGE="quay-.io/example/synthetic-operator@sha256:0000000000000000000000000000000000000000000000000000000000000000"
MIXED_CASE_HOST_IMAGE="Quay.io/example/synthetic-operator@sha256:0000000000000000000000000000000000000000000000000000000000000000"

expect_failure "rejects a double-dot host in an --allowed-image entry" \
  "approved image is not a valid fully-qualified, digest-pinned reference" \
  "$TMP_DIR/double-dot-host-allowlist.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-double-dot-host-allowlist" \
  --namespace gitops-test-ns --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$DOUBLE_DOT_HOST_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

expect_failure "rejects a host label starting with '-' in an --allowed-image entry" \
  "approved image is not a valid fully-qualified, digest-pinned reference" \
  "$TMP_DIR/leading-dash-host-allowlist.log" --pins "$TMP_DIR/good-pins.yaml" --output-dir "$TMP_DIR/out-leading-dash-host-allowlist" \
  --namespace gitops-test-ns --instance-namespace argocd-test \
  --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
  --allowed-image "$LEADING_DASH_LABEL_HOST_IMAGE" --allowed-image "$GOOD_OPERAND_IMAGE"

DOUBLE_DOT_HOST_TGZ="$(package_chart_with_image "$DOUBLE_DOT_HOST_IMAGE")"
write_pins "$TMP_DIR/double-dot-host-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$DOUBLE_DOT_HOST_TGZ" "$(sha256_of "$DOUBLE_DOT_HOST_TGZ")"
expect_failure "rejects a rendered manager image with a double-dot host" \
  "floating/malformed image" \
  "$TMP_DIR/double-dot-host-rendered.log" --pins "$TMP_DIR/double-dot-host-pins.yaml" --output-dir "$TMP_DIR/out-double-dot-host-rendered" "${COMMON_ARGS[@]}"

LEADING_DASH_LABEL_HOST_TGZ="$(package_chart_with_image "$LEADING_DASH_LABEL_HOST_IMAGE")"
write_pins "$TMP_DIR/leading-dash-host-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$LEADING_DASH_LABEL_HOST_TGZ" "$(sha256_of "$LEADING_DASH_LABEL_HOST_TGZ")"
expect_failure "rejects a rendered manager image with a host label starting with '-'" \
  "floating/malformed image" \
  "$TMP_DIR/leading-dash-host-rendered.log" --pins "$TMP_DIR/leading-dash-host-pins.yaml" --output-dir "$TMP_DIR/out-leading-dash-host-rendered" "${COMMON_ARGS[@]}"

MIXED_CASE_HOST_TGZ="$(package_chart_with_image "$MIXED_CASE_HOST_IMAGE")"
write_pins "$TMP_DIR/mixed-case-host-pins.yaml" "$CRDS_TGZ" "$CRDS_SUM" "$MIXED_CASE_HOST_TGZ" "$(sha256_of "$MIXED_CASE_HOST_TGZ")"
OUT_MIXED_CASE_HOST="$TMP_DIR/out-mixed-case-host"
if run_render --pins "$TMP_DIR/mixed-case-host-pins.yaml" --output-dir "$OUT_MIXED_CASE_HOST" \
    --namespace gitops-test-ns --instance-namespace argocd-test \
    --service-account test-operator-sa --image-pull-secret-name test-pull-secret \
    --allowed-image "$MIXED_CASE_HOST_IMAGE" \
    >"$TMP_DIR/mixed-case-host.log" 2>&1; then
  ok "accepts a mixed-case registry host (Quay.io), consistent with case-insensitive DNS hostnames"
else
  bad "accepts a mixed-case registry host (Quay.io), consistent with case-insensitive DNS hostnames"
  cat "$TMP_DIR/mixed-case-host.log" >&2
fi
rendered_mixed_case_host_image="$(
  yq eval -o=json -I=0 '.' "$OUT_MIXED_CASE_HOST/operator.yaml" 2>/dev/null |
    jq -sr '[.[] | select(.kind == "Deployment")][0].spec.template.spec.containers[] | select(.name == "manager") | .image' 2>/dev/null
)"
assert_eq "mixed-case host image reference round-trips exactly" "$MIXED_CASE_HOST_IMAGE" "$rendered_mixed_case_host_image"

echo
echo "gitops-bootstrap offline checks: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
