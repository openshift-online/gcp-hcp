#!/bin/bash
# 9-cleanup.sh - Remove the POC's local resources and, with confirmation, the
# throwaway signer service account.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

PROJECT="${POC_PROJECT_ID:?Set POC_PROJECT_ID (see env.example)}"
SA_NAME="${POC_SA_NAME:-egw-poc-signer}"
SA_EMAIL="${SA_NAME}@${PROJECT}.iam.gserviceaccount.com"

ENGINE="${POC_CONTAINER_ENGINE:-}"
if [ -z "${ENGINE}" ]; then
    if command -v podman >/dev/null 2>&1; then ENGINE="podman"; else ENGINE="docker"; fi
fi

"${ENGINE}" rm -f egw-poc-keycloak >/dev/null 2>&1 || true
echo "✓ Keycloak container removed"

pkill -f "python3 .*jwks-refetch-proxy\.py" >/dev/null 2>&1 || true
echo "✓ Proxy stopped (if running)"

rm -rf "${SCRIPT_DIR}/generated"
echo "✓ Local artifacts removed"

read -r -p "Delete the throwaway signer ${SA_EMAIL}? [y/N] " REPLY || REPLY="n"
if [[ "${REPLY}" =~ ^[Yy]$ ]]; then
    gcloud iam service-accounts delete "${SA_EMAIL}" --project "${PROJECT}" --quiet
    echo "✓ Service account deleted"
else
    echo "Kept ${SA_EMAIL}. Delete later with:"
    echo "  gcloud iam service-accounts delete ${SA_EMAIL} --project ${PROJECT}"
fi
