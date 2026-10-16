#!/usr/bin/env bash
set -euo pipefail

tests_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$tests_dir/.." && pwd)"
assert_log="$control_dir/scripts/assert-evaluation-log.sh"

command -v yq >/dev/null 2>&1 || {
	printf 'required command not found: yq\n' >&2
	exit 2
}

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

write_log() {
	local overall="$1"
	local requirement="$2"
	local destination="$3"
	cat >"$destination" <<EOF
metadata:
  id: cloud-armor-waf-prototype-policy
result: $overall
evaluations:
  - name: cloud-armor-waf-control
    result: $overall
    assessment-logs:
      - requirement:
          reference-id: cloud-armor-waf-prototype-policy
          entry-id: EXAMPLE-WAF-POC-1
        result: $requirement
target:
  id: synthetic-backend
EOF
}

write_log Passed Passed "$tmp_dir/passed.yaml"
write_log Failed Failed "$tmp_dir/failed.yaml"
malformed_path="$tmp_dir/private-path-marker/malformed.yaml"
mkdir -p "${malformed_path%/*}"
printf 'result: [malformed\n' >"$malformed_path"

if [[ ! -x "$assert_log" ]]; then
	printf 'evaluation-log assertion helper is missing\n' >&2
	exit 1
fi

checked_yq=0
while IFS= read -r yq_path; do
	yq_dir="${yq_path%/*}"
	selected_yq="$(PATH="$yq_dir:$PATH" bash -c 'command -v yq')"
	[[ "$selected_yq" == "$yq_path" ]] || continue
	checked_yq=1

	PATH="$yq_dir:$PATH" "$assert_log" "$tmp_dir/passed.yaml" Passed
	PATH="$yq_dir:$PATH" "$assert_log" "$tmp_dir/failed.yaml" Failed

	if PATH="$yq_dir:$PATH" "$assert_log" "$tmp_dir/failed.yaml" Passed >/dev/null 2>&1; then
		printf 'assertion helper accepted a failed log as Passed with %s\n' "$yq_path" >&2
		exit 1
	fi

	if PATH="$yq_dir:$PATH" "$assert_log" "$tmp_dir/missing.yaml" Passed >/dev/null 2>&1; then
		printf 'assertion helper accepted a missing EvaluationLog with %s\n' "$yq_path" >&2
		exit 1
	fi

	malformed_output="$tmp_dir/malformed-output.txt"
	if PATH="$yq_dir:$PATH" "$assert_log" "$malformed_path" Passed >"$malformed_output" 2>&1; then
		printf 'assertion helper accepted malformed YAML with %s\n' "$yq_path" >&2
		exit 1
	fi
	if grep -Fq "$malformed_path" "$malformed_output"; then
		printf 'assertion helper leaked a malformed EvaluationLog path with %s\n' "$yq_path" >&2
		cat "$malformed_output" >&2
		exit 1
	fi
done < <(type -ap yq | LC_ALL=C sort -u)

if ((checked_yq == 0)); then
	printf 'could not find an executable yq implementation to test\n' >&2
	exit 2
fi

printf 'EvaluationLog assertion tests passed\n'
