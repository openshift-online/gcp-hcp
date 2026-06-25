#!/usr/bin/env bash
set -euo pipefail

HOST_PORT="${FIRESTORE_EMULATOR_HOST:-localhost:8219}"

EMULATOR_PID=""

cleanup() {
  echo ""
  echo "Stopping Firestore emulator..."
  if [[ -n "${EMULATOR_PID}" ]]; then
    kill "${EMULATOR_PID}" 2>/dev/null || true
    wait "${EMULATOR_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "Starting Firestore emulator on ${HOST_PORT}..."
echo ""
echo "In other terminals, export:"
echo "  export FIRESTORE_EMULATOR_HOST=${HOST_PORT}"
echo ""

gcloud emulators firestore start --host-port="${HOST_PORT}" &
EMULATOR_PID=$!
wait "${EMULATOR_PID}"
