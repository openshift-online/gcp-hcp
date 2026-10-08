#!/usr/bin/env bash
set -euo pipefail

tests_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$tests_dir/.." && pwd)"
policy_dir="$control_dir/policy"
baseline="$tests_dir/fixtures/active-preconfigured-waf.json"
namespace="cloudarmor.waf"

for tool in conftest jq shasum; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf 'required command not found: %s\n' "$tool" >&2
		exit 2
	fi
done

if [ ! -f "$baseline" ]; then
	printf 'baseline fixture not found: %s\n' "$baseline" >&2
	exit 2
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

run_case() {
	local name="$1"
	local expected="$2"
	local expected_reason="$3"
	local transform="$4"
	local snapshot="$tmp_dir/$name.json"
	local test_output="$tmp_dir/$name.conftest.json"
	local actual reason target_id digest conftest_status

	jq -c "$transform" "$baseline" >"$snapshot"
	if conftest test "$snapshot" --policy "$policy_dir" --namespace "$namespace" --output json >"$test_output" 2>&1; then
		conftest_status=0
	else
		conftest_status=$?
	fi

	if ! jq -e --arg namespace "$namespace" 'type == "array" and length == 1 and .[0].namespace == $namespace and all(.[]; (.successes | type) == "number" and .successes > 0 and (if has("failures") then (.failures | type) == "array" else true end))' "$test_output" >/dev/null; then
		printf 'conftest did not produce a valid evaluation result for %s\n' "$name" >&2
		cat "$test_output" >&2
		exit 2
	fi

	reason="$(jq -er '[.[].failures[]?.msg] | unique | join("; ")' "$test_output")"
	case "$conftest_status" in
		0) actual=pass ;;
		1) actual=fail ;;
		*)
			printf 'conftest returned an execution error for %s (exit %s)\n' "$name" "$conftest_status" >&2
			cat "$test_output" >&2
			exit 2
			;;
	esac

	if { [ "$actual" = fail ] && [ -z "$reason" ]; } || { [ "$actual" = pass ] && [ -n "$reason" ]; }; then
		printf 'conftest exit status and result details disagree for %s\n' "$name" >&2
		cat "$test_output" >&2
		exit 2
	fi

	if [ "$actual" != "$expected" ]; then
		printf 'unexpected %s result for %s (expected %s)\n' "$actual" "$name" "$expected" >&2
		cat "$test_output" >&2
		exit 1
	fi

	if [ -n "$expected_reason" ] && [[ "$reason" != *"$expected_reason"* ]]; then
		printf 'unexpected failure reason for %s: %s\n' "$name" "$reason" >&2
		exit 1
	fi

	target_id="$(jq -er '.target.id' "$snapshot")"
	digest="$(shasum -a 256 "$snapshot" | awk '{print $1}')"
	jq -cn \
		--arg scenario "$name" \
		--arg target_id "$target_id" \
		--arg snapshot_sha256 "$digest" \
		--arg requirement_id "EXAMPLE-WAF-POC-1" \
		--arg result "$actual" \
		--arg reason "$reason" \
		'{scenario: $scenario, target_id: $target_id, requirement_id: $requirement_id, snapshot_sha256: $snapshot_sha256, result: $result, reason: (if $reason == "" then null else $reason end)}'
}

run_case "active-preconfigured-waf" pass "" '.'
run_case "active-cloud-armor-custom-deny-action" pass "" \
	'.securityPolicy.rules[0].match.expr.expression = "origin.region_code == '\''US'\''"'
run_case "active-cloud-armor-rate-limit-action" pass "" \
	'.securityPolicy.rules[0].action = "throttle" | .securityPolicy.rules[0].match.expr.expression = "origin.region_code == '\''US'\''" | .securityPolicy.rules[0].rateLimitOptions = {"rateLimitThreshold":{"count":100,"intervalSec":60},"conformAction":"allow","exceedAction":"deny(429)"}'
run_case "detached-policy" fail "backend service is not attached" \
	'.backendService.securityPolicy = "https://www.googleapis.com/compute/v1/projects/synthetic-project/global/securityPolicies/another-policy"'
run_case "empty-policy-references" fail "backend service is not attached" \
	'.backendService.securityPolicy = "" | .securityPolicy.selfLink = ""'
run_case "no-enforcing-rule" fail "no active, non-preview Cloud Armor deny or rate-limit action" \
	'.securityPolicy.rules[0].action = "allow"'
run_case "not-internet-facing" fail "target is not confirmed as internet-facing" \
	'.target.internetFacing = false | .target.id = "synthetic-private-backend"'
run_case "preview-only-waf" fail "no active, non-preview Cloud Armor deny or rate-limit action" \
	'.securityPolicy.rules[0].preview = true'
run_case "request-logging-disabled" fail "request logging is not enabled" \
	'.backendService.logConfig.enable = false'
run_case "request-logging-zero-sample" fail "sample rate is zero, missing, or invalid" \
	'.backendService.logConfig.sampleRate = 0'
