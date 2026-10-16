#!/usr/bin/env bash
set -euo pipefail

tests_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
control_dir="$(cd -- "$tests_dir/.." && pwd -P)"
clean_command="$(make -s -n -C "$control_dir" clean CONTROL_DIR=/ BIN_DIR=/ GO_PATH=/ GO_CACHE=/)"
rm_command="$(sed -n '/^rm -rf /p' <<<"$clean_command")"
expected_command="rm -rf \"$control_dir/bin\" \"$control_dir/.complytime\""

if [[ "$rm_command" != "$expected_command" ]]; then
  printf 'clean target can be redirected outside the WAF control directory\n' >&2
  printf 'expected: %s\n' "$expected_command" >&2
  printf 'actual:   %s\n' "$rm_command" >&2
  exit 1
fi

printf 'clean target remains scoped to ignored WAF control outputs\n'
