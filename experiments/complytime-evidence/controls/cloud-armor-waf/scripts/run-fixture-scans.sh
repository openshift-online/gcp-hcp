#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$script_dir/.." && pwd)"
baseline="$control_dir/tests/fixtures/active-preconfigured-waf.json"
tmp_dir="$(mktemp -d)"
trap 'rm -f "$tmp_dir/active-preconfigured-waf-fail.json"; rmdir "$tmp_dir"' EXIT

failed_fixture="$tmp_dir/active-preconfigured-waf-fail.json"
jq '.backendService.logConfig.enable = false' "$baseline" >"$failed_fixture"

bash "$script_dir/run-complytime-scan.sh" \
	--input-file "$baseline" \
	--target-id "synthetic-public-backend" \
	--expected-result Passed

bash "$script_dir/run-complytime-scan.sh" \
	--input-file "$failed_fixture" \
	--target-id "synthetic-backend-logging-disabled" \
	--expected-result Failed
