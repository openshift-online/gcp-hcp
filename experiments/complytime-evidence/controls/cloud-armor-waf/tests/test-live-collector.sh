#!/usr/bin/env bash
set -euo pipefail

tests_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$tests_dir/.." && pwd)"
tmp_root="$(mktemp -d)"
trap 'rm -rf "$tmp_root"' EXIT

test_control="$tmp_root/control"
fake_bin="$tmp_root/bin"
responses="$tmp_root/responses"
capture="$tmp_root/capture"
mkdir -p "$test_control/scripts" "$fake_bin" "$responses" "$capture"
cp "$control_dir/scripts/run-live-check.sh" "$test_control/scripts/"
cp "$control_dir/scripts/normalize-gcloud-snapshot.jq" "$test_control/scripts/"
cp "$control_dir/Makefile" "$test_control/Makefile"
touch "$capture/gcloud-commands.log"

project_marker="project-name-secret"
backend_marker="backend-name-secret"
policy_marker="policy-name-secret"
map_marker="map-name-secret"

printf '%s\n' \
  "{\"name\":\"$backend_marker\",\"selfLink\":\"https://compute.example/projects/$project_marker/global/backendServices/$backend_marker\",\"loadBalancingScheme\":\"EXTERNAL_MANAGED\",\"securityPolicy\":\"https://compute.example/projects/$project_marker/global/securityPolicies/$policy_marker\",\"usedBy\":[{\"reference\":\"https://compute.example/projects/$project_marker/global/urlMaps/$map_marker\"}],\"logConfig\":{\"enable\":true,\"sampleRate\":1.0}}" \
  >"$responses/backend-service.json"
printf '%s\n' \
  "{\"name\":\"$policy_marker\",\"selfLink\":\"https://compute.example/projects/$project_marker/global/securityPolicies/$policy_marker\",\"rules\":[{\"priority\":1000,\"action\":\"deny(403)\",\"preview\":false}]}" \
  >"$responses/security-policy.json"
printf '%s\n' \
  "{\"name\":\"$map_marker\",\"selfLink\":\"https://compute.example/projects/$project_marker/global/urlMaps/$map_marker\",\"defaultService\":\"https://compute.example/projects/$project_marker/global/backendServices/$backend_marker\"}" \
  >"$responses/url-map.json"
printf '%s\n' \
  "[{\"name\":\"synthetic-https-proxy\",\"selfLink\":\"https://compute.example/projects/$project_marker/global/targetHttpsProxies/synthetic-https-proxy\",\"urlMap\":\"https://compute.example/projects/$project_marker/global/urlMaps/$map_marker\"}]" \
  >"$responses/https-proxies.json"
printf '%s\n' \
  "[{\"name\":\"synthetic-forwarding-rule\",\"selfLink\":\"https://compute.example/projects/$project_marker/global/forwardingRules/synthetic-forwarding-rule\",\"target\":\"https://compute.example/projects/$project_marker/global/targetHttpsProxies/synthetic-https-proxy\",\"loadBalancingScheme\":\"EXTERNAL_MANAGED\"}]" \
  >"$responses/forwarding-rules.json"

cat >"$fake_bin/gcloud" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$TEST_GCLOUD_LOG"
if [[ -n "${TEST_GCLOUD_FAIL_MATCH:-}" && "$*" == *"$TEST_GCLOUD_FAIL_MATCH"* ]]; then
  printf 'permission denied for %s in %s\n' "$TEST_BACKEND_MARKER" "$TEST_PROJECT_MARKER" >&2
  exit 42
fi
case "$*" in
  *"backend-services describe"*) cat "$TEST_RESPONSE_DIR/backend-service.json" ;;
  *"security-policies describe"*) cat "$TEST_RESPONSE_DIR/security-policy.json" ;;
  *"url-maps describe"*) cat "$TEST_RESPONSE_DIR/url-map.json" ;;
  *"target-http-proxies list"*) printf '[]\n' ;;
  *"target-https-proxies list"*) cat "$TEST_RESPONSE_DIR/https-proxies.json" ;;
  *"forwarding-rules list"*) cat "$TEST_RESPONSE_DIR/forwarding-rules.json" ;;
  *) printf 'unexpected synthetic gcloud command\n' >&2; exit 9 ;;
esac
EOF
chmod 0700 "$fake_bin/gcloud"

cat >"$test_control/scripts/run-complytime-scan.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
input_file=""
target_id=""
expected_result=""
while (($#)); do
  case "$1" in
    --input-file) input_file="$2"; shift 2 ;;
    --target-id) target_id="$2"; shift 2 ;;
    --expected-result) expected_result="$2"; shift 2 ;;
    *) exit 10 ;;
  esac
done
[[ -f "$input_file" ]] || exit 11
jq -e --arg target "$TEST_EXPECTED_TARGET" '
  .target.id == $target
  and .target.internetFacing == true
  and .backendService.logConfig.enable == true
  and any(.securityPolicy.rules[]; .action == "deny(403)" and .preview == false)
' "$input_file" >/dev/null || exit 12
printf '%s\n' "$input_file" >"$TEST_CAPTURE_DIR/input-path"
printf '%s\n' "$target_id" "$expected_result" >"$TEST_CAPTURE_DIR/scan-args"
if [[ "${TEST_SCAN_STATUS:-0}" != 0 ]]; then
  printf 'synthetic provider failure\n' >&2
  exit "$TEST_SCAN_STATUS"
fi
printf 'EvaluationLog requirement result: %s\n' "$TEST_SCAN_RESULT"
EOF
chmod 0700 "$test_control/scripts/run-complytime-scan.sh"

run_case() {
  local result="$1" expected_status="$2" output="$tmp_root/output-$1-$2.txt" status input_path
  set +e
  PATH="$fake_bin:$PATH" \
    TEST_CAPTURE_DIR="$capture" \
    TEST_BACKEND_MARKER="$backend_marker" \
    TEST_EXPECTED_TARGET="$backend_marker" \
    TEST_GCLOUD_LOG="$capture/gcloud-commands.log" \
    TEST_PROJECT_MARKER="$project_marker" \
    TEST_RESPONSE_DIR="$responses" \
    TEST_SCAN_RESULT="$result" \
    TEST_SCAN_STATUS="$expected_status" \
      make -s -C "$test_control" scan-live \
        PROJECT="$project_marker" \
        BACKEND_SERVICE="$backend_marker" \
        SECURITY_POLICY="$policy_marker" >"$output" 2>&1
  status=$?
  set -e

  if [[ "$expected_status" == 0 ]]; then
    [[ "$status" == 0 ]] || { cat "$output" >&2; return 1; }
    jq -e --arg result "$result" '.source == "live-gcloud" and .result == $result and .observations.internet_facing == true' "$output" >/dev/null || {
      printf 'live collector did not report the completed %s EvaluationLog result\n' "$result" >&2
      cat "$output" >&2
      return 1
    }
  else
    [[ "$status" == 2 ]] || {
      printf 'provider failure should be an operational exit 2, got %s\n' "$status" >&2
      cat "$output" >&2
      return 1
    }
  fi

  for marker in "$project_marker" "$backend_marker" "$policy_marker" "$map_marker"; do
    if grep -Fq "$marker" "$output"; then
      printf 'live collector leaked a GCP identifier: %s\n' "$marker" >&2
      cat "$output" >&2
      return 1
    fi
  done

  input_path="$(<"$capture/input-path")"
  [[ -n "$input_path" && ! -e "$input_path" ]] || {
    printf 'temporary normalized input was not removed after the scan\n' >&2
    return 1
  }
  [[ "$(sed -n '1p' "$capture/scan-args")" == "$backend_marker" ]] || {
    printf 'shared scan runner did not receive the backend target id\n' >&2
    return 1
  }
  [[ "$(sed -n '2p' "$capture/scan-args")" == Any ]] || {
    printf 'shared scan runner did not accept either completed policy result\n' >&2
    return 1
  }
}

run_collection_error_case() {
  local failure_match="$1" expected_message="$2"
  local output="$tmp_root/output-gcloud-error.txt" status
  set +e
  PATH="$fake_bin:$PATH" \
    TEST_BACKEND_MARKER="$backend_marker" \
    TEST_CAPTURE_DIR="$capture" \
    TEST_EXPECTED_TARGET="$backend_marker" \
    TEST_GCLOUD_LOG="$capture/gcloud-commands.log" \
    TEST_GCLOUD_FAIL_MATCH="$failure_match" \
    TEST_PROJECT_MARKER="$project_marker" \
    TEST_RESPONSE_DIR="$responses" \
      make -s -C "$test_control" scan-live \
        PROJECT="$project_marker" \
        BACKEND_SERVICE="$backend_marker" \
        SECURITY_POLICY="$policy_marker" >"$output" 2>&1
  status=$?
  set -e

  [[ "$status" == 2 ]] || {
    printf 'gcloud collection failure should be an operational exit 2, got %s\n' "$status" >&2
    cat "$output" >&2
    return 1
  }
  for marker in "$project_marker" "$backend_marker" "$policy_marker" "$map_marker"; do
    if grep -Fq "$marker" "$output"; then
      printf 'live collector leaked a GCP identifier from gcloud stderr: %s\n' "$marker" >&2
      cat "$output" >&2
      return 1
    fi
  done
  grep -Fq "$expected_message" "$output" || {
    printf 'live collector did not keep the expected generic gcloud error message: %s\n' "$expected_message" >&2
    cat "$output" >&2
    return 1
  }
}

run_case Passed 0
run_case Failed 0
run_case Failed 2
run_collection_error_case \
  'backend-services describe' \
  'could not read the named global backend service'
run_collection_error_case \
  'security-policies describe' \
  'could not read the named global Cloud Armor policy'
run_collection_error_case \
  'url-maps describe' \
  'could not read a URL map associated with the named backend service'
run_collection_error_case \
  'target-http-proxies list' \
  'could not list global HTTP(S) proxies for an associated URL map'
run_collection_error_case \
  'target-https-proxies list' \
  'could not list global HTTP(S) proxies for an associated URL map'
run_collection_error_case \
  'forwarding-rules list' \
  'could not list global forwarding rules for an associated HTTP(S) proxy'

for command in \
  'compute backend-services describe' \
  'compute security-policies describe' \
  'compute url-maps describe' \
  'compute target-https-proxies list' \
  'compute forwarding-rules list'; do
  grep -Fq "$command" "$capture/gcloud-commands.log" || {
    printf 'expected read-only gcloud call was not observed: %s\n' "$command" >&2
    exit 1
  }
done
if grep -E '(^| )(create|delete|update|set|add|remove)( |$)' "$capture/gcloud-commands.log"; then
  printf 'fake-gcloud test observed a mutating gcloud command\n' >&2
  exit 1
fi

printf 'live collector tests passed (read-only calls, native scan routing, redaction, and cleanup)\n'
