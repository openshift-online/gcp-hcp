#!/bin/bash
# 2-run-keycloak.sh - Run a local Keycloak with the two POC clients
#
#   gecko-poc          jwks_uri -> Google's per-SA JWKS (used by script 3)
#   gecko-poc-refetch  jwks_uri -> the local refetch proxy (used by script 4)
#
# The realm import file is generated from realm-template.json. The first
# image pull can take a few minutes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

PROJECT="${POC_PROJECT_ID:?Set POC_PROJECT_ID (see env.example)}"
SA_NAME="${POC_SA_NAME:-egw-poc-signer}"
SA_EMAIL="${SA_NAME}@${PROJECT}.iam.gserviceaccount.com"
PORT="${POC_KEYCLOAK_PORT:-8080}"
PROXY_PORT="${POC_PROXY_PORT:-9099}"
IMAGE="${POC_KEYCLOAK_IMAGE:-quay.io/keycloak/keycloak:26.8}"
CONTAINER="egw-poc-keycloak"

ENGINE="${POC_CONTAINER_ENGINE:-}"
if [ -z "${ENGINE}" ]; then
    if command -v podman >/dev/null 2>&1; then ENGINE="podman"; else ENGINE="docker"; fi
fi

JWKS_URL="https://www.googleapis.com/service_accounts/v1/jwk/${SA_EMAIL}"

mkdir -p "${SCRIPT_DIR}/generated/import"
sed -e "s|__JWKS_URL__|${JWKS_URL}|g" \
    -e "s|__PROXY_JWKS_URL__|http://localhost:${PROXY_PORT}/jwk|g" \
    "${SCRIPT_DIR}/realm-template.json" > "${SCRIPT_DIR}/generated/import/poc-realm.json"

echo "== Realm clients and their jwks.url:"
jq -r '.clients[] | "   \(.clientId): \(.attributes["jwks.url"])"' \
    "${SCRIPT_DIR}/generated/import/poc-realm.json"

"${ENGINE}" rm -f "${CONTAINER}" >/dev/null 2>&1 || true

echo "== Starting Keycloak (${IMAGE}, host network, port ${PORT})"
"${ENGINE}" run -d --name "${CONTAINER}" \
    --network host \
    -e KC_BOOTSTRAP_ADMIN_USERNAME=admin \
    -e KC_BOOTSTRAP_ADMIN_PASSWORD=admin \
    -e KC_HTTP_HOST=127.0.0.1 \
    -e KC_HTTP_PORT="${PORT}" \
    -v "${SCRIPT_DIR}/generated/import:/opt/keycloak/data/import:Z" \
    "${IMAGE}" start-dev --import-realm

echo "== Waiting for Keycloak (first pull may take a while)"
for i in $(seq 1 90); do
    if curl -fsS "http://localhost:${PORT}/realms/poc" >/dev/null 2>&1; then
        echo "✓ Keycloak is up: http://localhost:${PORT} (admin/admin, realm 'poc')"
        echo "  Token endpoint: http://localhost:${PORT}/realms/poc/protocol/openid-connect/token"
        echo "  Next: ./3-sign-and-exchange.sh"
        exit 0
    fi
    if "${ENGINE}" logs "${CONTAINER}" 2>&1 | grep -q "Failed to run import\|Failed to start server"; then
        echo "✗ Keycloak failed to start (realm import error?); logs:"
        "${ENGINE}" logs --tail 30 "${CONTAINER}"
        exit 1
    fi
    if ! "${ENGINE}" ps --format '{{.Names}}' 2>/dev/null | grep -q "^${CONTAINER}$"; then
        echo "✗ Container exited; logs:"
        "${ENGINE}" logs "${CONTAINER}"
        exit 1
    fi
    sleep 5
done

echo "✗ Keycloak did not come up in time; logs:"
"${ENGINE}" logs --tail 50 "${CONTAINER}"
exit 1
