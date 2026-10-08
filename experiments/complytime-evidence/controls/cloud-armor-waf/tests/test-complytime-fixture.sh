#!/usr/bin/env bash
set -euo pipefail

tests_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$tests_dir/.." && pwd)"
output_file="$(mktemp)"
trap 'rm -f "$output_file"' EXIT

if ! make -s -C "$control_dir" scan-fixture >"$output_file" 2>&1; then
	cat "$output_file" >&2
	exit 1
fi

for expected in Passed Failed; do
	if ! grep -Fq "EvaluationLog requirement result: $expected" "$output_file"; then
		printf 'fixture scan did not confirm a native %s result\n' "$expected" >&2
		cat "$output_file" >&2
		exit 1
	fi
done

printf 'ComplyTime fixture scans passed (native EvaluationLog Passed and Failed)\n'
