#!/usr/bin/env bash
set -euo pipefail
umask 077

script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$script_dir/.." && pwd)"
input_file=""
target_id=""
expected_result="Any"
runner_log=""

usage() {
  cat <<'EOF'
Run the WAF Gemara policy through complyctl and the OPA provider.

Usage:
  bash scripts/run-complytime-scan.sh \
    --input-file PATH --target-id ID [--expected-result Passed|Failed|Any]

Any accepts either completed policy result. Collection or scan errors remain
operational failures and return a non-zero status.
EOF
}

fail() {
  local message="$1"
  local status=2
  if (($# > 1)); then status="$2"; fi
  printf '%s\n' "$message" >&2
  if [[ -n "$runner_log" && -f "$runner_log" ]]; then
    printf 'Local diagnostics are in the ignored run directory under .complytime/runs/.\n' >&2
  fi
  exit "$status"
}

while (($#)); do
  case "$1" in
    --input-file)
      (($# >= 2)) || { usage; exit 2; }
      input_file="$2"
      shift 2
      ;;
    --target-id)
      (($# >= 2)) || { usage; exit 2; }
      target_id="$2"
      shift 2
      ;;
    --expected-result)
      (($# >= 2)) || { usage; exit 2; }
      expected_result="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      printf 'unknown argument\n' >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "$input_file" || -z "$target_id" ]]; then
  usage
  exit 2
fi
case "$expected_result" in
  Passed|Failed|Any) ;;
  *)
    printf 'expected result must be Passed, Failed, or Any\n' >&2
    exit 2
    ;;
esac
[[ -f "$input_file" ]] || fail 'assessment input file is missing'

for tool in podman curl jq yq oras conftest git; do
  command -v "$tool" >/dev/null 2>&1 || fail "required command not found: $tool"
done
complyctl_bin="$control_dir/bin/complyctl"
provider_bin="$control_dir/bin/complyctl-provider-opa"
complypack_bin="$control_dir/bin/complypack"
[[ -x "$complyctl_bin" ]] || fail 'complyctl is missing; run make install-tools from the WAF control directory'
[[ -x "$provider_bin" ]] || fail 'the OPA provider is missing; run make install-tools from the WAF control directory'
[[ -x "$complypack_bin" ]] || fail 'CompliPack is missing; run make install-tools from the WAF control directory'

if ! podman info >/dev/null 2>&1; then
  fail 'Podman is installed but its configured machine is unavailable. Start the existing machine with podman machine start, then retry; this runner will not initialize or reconfigure Podman.'
fi

mkdir -p "$control_dir/.complytime/runs"
run_dir="$(mktemp -d "$control_dir/.complytime/runs/scan.XXXXXXXX")"
runner_log="$run_dir/runner.log"
touch "$runner_log"
registry_name="complytime-waf-$$-$RANDOM"
registry_started=0

cleanup() {
  if ((registry_started)); then
    podman stop --time 1 "$registry_name" >>"$runner_log" 2>&1 || true
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if ! podman run --detach --rm --name "$registry_name" \
  --publish 127.0.0.1::5000 docker.io/library/registry:2.8.3 \
  >>"$runner_log" 2>&1; then
  fail 'could not start the temporary loopback OCI registry'
fi
registry_started=1
run_home="$run_dir/home"
mkdir -p "$run_home/.local/share/complytime/providers" "$run_home/.cache"

port_mapping="$(podman port "$registry_name" 5000/tcp 2>>"$runner_log" | tail -n 1 || true)"
registry_port="$(printf '%s\n' "$port_mapping" | awk -F: '{print $NF}')"
[[ "$registry_port" =~ ^[0-9]+$ ]] || fail 'could not determine the local OCI registry port'
registry_ref="127.0.0.1:$registry_port"
registry_url="http://$registry_ref"

registry_ready=0
for _ in {1..30}; do
  if curl --silent --show-error --fail "$registry_url/v2/" >/dev/null 2>>"$runner_log"; then
    registry_ready=1
    break
  fi
  sleep 1
done
((registry_ready)) || fail 'the temporary loopback OCI registry did not become ready'

printf 'Packing Gemara and CompliPack artifacts into the temporary local registry.\n'
if ! (cd "$control_dir" && HOME="$run_home" XDG_CACHE_HOME="$run_home/.cache" \
	oras push --plain-http --no-tty \
  "$registry_ref/policies/cloud-armor-waf:v0.1.0" \
  "gemara/guidance-catalog.yaml:application/vnd.gemara.guidance.v1+yaml" \
  "gemara/control-catalog.yaml:application/vnd.gemara.catalog.v1+yaml" \
  "gemara/policy.yaml:application/vnd.gemara.policy.v1+yaml") \
  >>"$runner_log" 2>&1; then
  fail 'could not push the Gemara policy layers to the temporary registry'
fi
if ! (cd "$control_dir" && HOME="$run_home" XDG_CACHE_HOME="$run_home/.cache" \
	"$complypack_bin" pack policy/ \
  "$registry_ref/complypacks/cloud-armor-waf:0.1.0" \
  --config complypack.yaml --plain-http --skip-tests) \
  >>"$runner_log" 2>&1; then
  fail 'could not package the WAF Rego and mapping as a CompliPack'
fi

cp "$provider_bin" "$run_home/.local/share/complytime/providers/complyctl-provider-opa"
chmod 0700 "$run_home/.local/share/complytime/providers/complyctl-provider-opa"
mkdir -p "$run_dir/.complytime"
target_id_yaml="$(jq -Rn --arg value "$target_id" '$value')"
input_file_abs="$(cd -- "$(dirname -- "$input_file")" && pwd)/$(basename -- "$input_file")"
input_file_yaml="$(jq -Rn --arg value "$input_file_abs" '$value')"
cat >"$run_dir/.complytime/complytime.yaml" <<EOF
policies:
  - url: $registry_url/policies/cloud-armor-waf:v0.1.0
    id: cloud-armor-waf
complypacks:
  - url: $registry_url/complypacks/cloud-armor-waf:0.1.0
    id: cloud-armor-waf-opa
targets:
  - id: $target_id_yaml
    policies:
      - cloud-armor-waf
    variables:
      input_path: $input_file_yaml
EOF
chmod 0600 "$run_dir/.complytime/complytime.yaml"

printf 'Fetching the local artifacts, generating the OPA assessment, and scanning.\n'
if ! (cd "$run_dir" && HOME="$run_home" XDG_DATA_HOME="$run_home/.local/share" COMPLYTIME_WORKSPACE="$run_dir" \
	"$complyctl_bin" get) >>"$runner_log" 2>&1; then
  fail 'complyctl could not fetch the temporary local artifacts'
fi
if ! (cd "$run_dir" && HOME="$run_home" XDG_DATA_HOME="$run_home/.local/share" COMPLYTIME_WORKSPACE="$run_dir" \
	"$complyctl_bin" generate --policy-id cloud-armor-waf) \
  >>"$runner_log" 2>&1; then
  fail 'complyctl could not generate the OPA assessment'
fi
if ! (cd "$run_dir" && HOME="$run_home" XDG_DATA_HOME="$run_home/.local/share" COMPLYTIME_WORKSPACE="$run_dir" \
	"$complyctl_bin" scan --policy-id cloud-armor-waf) \
  >>"$runner_log" 2>&1; then
  fail 'complyctl scan encountered an operational error'
fi

log_path="$(find "$run_dir/.complytime/scan" -maxdepth 1 -type f \
  -name 'evaluation-log-*.yaml' -print -quit 2>/dev/null || true)"
[[ -n "$log_path" ]] || fail 'complyctl completed without producing an EvaluationLog'
actual_result="$(yq -r '.result // ""' "$log_path" 2>/dev/null || true)"
case "$actual_result" in
  Passed|Failed) ;;
  *) fail 'EvaluationLog contains no completed WAF assessment result' ;;
esac
"$script_dir/assert-evaluation-log.sh" "$log_path" "$actual_result" >/dev/null || \
  fail 'EvaluationLog is missing the expected WAF requirement result'

if [[ "$expected_result" != Any && "$actual_result" != "$expected_result" ]]; then
  "$script_dir/assert-evaluation-log.sh" "$log_path" "$expected_result"
  fail 'fixture result did not match the expected result' 1
fi

printf 'EvaluationLog requirement result: %s\n' "$actual_result"
printf 'Local EvaluationLog: %s\n' "$log_path"
