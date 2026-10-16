#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
Usage: assert-evaluation-log.sh EVALUATION_LOG.yaml Passed|Failed

Checks the overall EvaluationLog result and the WAF example requirement result.
EOF
}

if [[ $# -ne 2 ]]; then
	usage
	exit 2
fi

log_path="$1"
expected_result="$2"
requirement_id="EXAMPLE-WAF-POC-1"

case "$expected_result" in
	Passed|Failed) ;;
	*)
		printf 'expected result must be Passed or Failed\n' >&2
		exit 2
		;;
esac

if [[ ! -f "$log_path" ]]; then
	printf 'EvaluationLog file is missing\n' >&2
	exit 2
fi

for tool in yq jq; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf 'required command not found: %s\n' "$tool" >&2
		exit 2
	fi
done

yq_version="$(yq --version 2>/dev/null || true)"
case "$yq_version" in
	*github.com/mikefarah/yq*) yq_flavor=mikefarah ;;
	'yq '*) yq_flavor=jq-wrapper ;;
	*)
		printf 'unsupported yq implementation; use Mike Farah yq v4 or the Python yq jq wrapper\n' >&2
		exit 2
		;;
esac

yq_to_json() {
	case "$yq_flavor" in
		mikefarah) yq -o=json '.' "$log_path" 2>/dev/null ;;
		jq-wrapper) yq '.' "$log_path" 2>/dev/null ;;
	esac
}

if ! yq_to_json | jq -e \
	--arg expected "$expected_result" \
	--arg requirement_id "$requirement_id" \
	'(.result == $expected)
	and ([.evaluations[]?
		| .["assessment-logs"][]?
		| select(.requirement["entry-id"] == $requirement_id)
		| .result] == [$expected])' \
	>/dev/null 2>&1; then
	printf 'EvaluationLog does not contain the expected %s result for %s\n' \
		"$expected_result" "$requirement_id" >&2
	exit 1
fi

printf 'EvaluationLog requirement result: %s\n' "$expected_result"
