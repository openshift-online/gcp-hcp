#!/bin/bash
# 4-test-refetch.sh - Prove Keycloak recovers from an unknown kid by refetching
# the JWKS.
#
# A local proxy serves an empty JWKS on its first request and Google's real
# JWKS afterwards; the gecko-poc-refetch client points at the proxy.
#
# Observed Keycloak 26 behaviour (default config): on an unknown kid, Keycloak
# refetches the JWKS only if its last fetch was more than ~10 s ago. A cold
# cache therefore fails the first exchange ("Unable to load public key") and a
# retry after the interval succeeds. This script reproduces exactly that:
# attempt 1 may fail, attempt 2 must succeed. In production, attempt 2 is
# gecko's retry-once-on-invalid_client - the same pattern already used for 401s.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

PROJECT="${POC_PROJECT_ID:?Set POC_PROJECT_ID (see env.example)}"
SA_NAME="${POC_SA_NAME:-egw-poc-signer}"
SA_EMAIL="${SA_NAME}@${PROJECT}.iam.gserviceaccount.com"
PORT="${POC_KEYCLOAK_PORT:-8080}"
PROXY_PORT="${POC_PROXY_PORT:-9099}"

CLIENT_ID="gecko-poc-refetch"
TOKEN_URL="http://localhost:${PORT}/realms/poc/protocol/openid-connect/token"
JWKS_URL="https://www.googleapis.com/service_accounts/v1/jwk/${SA_EMAIL}"
REFETCH_WAIT="${POC_REFETCH_WAIT:-15}"

mkdir -p "${SCRIPT_DIR}/generated"
rm -f "${SCRIPT_DIR}/generated/proxy.log"

# A cold Keycloak cache is required: if the client's keys are already cached,
# no fetch happens at all and the test would pass vacuously.
echo "== Restarting Keycloak for a cold JWKS cache"
if ! "${SCRIPT_DIR}/2-run-keycloak.sh" >"${SCRIPT_DIR}/generated/keycloak-restart.log" 2>&1; then
    echo "✗ Keycloak restart failed:"
    tail -20 "${SCRIPT_DIR}/generated/keycloak-restart.log"
    exit 1
fi

echo "== Starting the JWKS refetch proxy on :${PROXY_PORT}"
GOOGLE_JWKS_URL="${JWKS_URL}" \
PROXY_LOG="${SCRIPT_DIR}/generated/proxy.log" \
    python3 "${SCRIPT_DIR}/jwks-refetch-proxy.py" "${PROXY_PORT}" &
PROXY_PID=$!
trap 'kill "${PROXY_PID}" 2>/dev/null || true' EXIT

for i in $(seq 1 20); do
    curl -fsS "http://localhost:${PROXY_PORT}/healthz" >/dev/null 2>&1 && break
    if [ "${i}" = "20" ]; then
        echo "✗ Proxy did not start"
        exit 1
    fi
    sleep 1
done
echo "✓ Proxy is up (health checks do not consume the empty-first request)"

exchange() {
    local now
    now=$(date +%s)
    printf '{"iss": "%s", "sub": "%s", "aud": "%s", "jti": "%s", "iat": %d, "exp": %d}' \
        "${CLIENT_ID}" "${CLIENT_ID}" "${TOKEN_URL}" "$(uuidgen)" "${now}" "$((now + 60))" \
        > "${SCRIPT_DIR}/generated/payload.json"
    gcloud iam service-accounts sign-jwt \
        --iam-account="${SA_EMAIL}" \
        "${SCRIPT_DIR}/generated/payload.json" \
        "${SCRIPT_DIR}/generated/assertion.jwt" >/dev/null
    HTTP_CODE=$(curl -s --max-time 30 -o "${SCRIPT_DIR}/generated/token.json" -w '%{http_code}' "${TOKEN_URL}" \
        -d grant_type=client_credentials \
        -d client_id="${CLIENT_ID}" \
        -d client_assertion_type="urn:ietf:params:oauth:client-assertion-type:jwt-bearer" \
        -d client_assertion="$(cat "${SCRIPT_DIR}/generated/assertion.jwt")")
}

echo "== Attempt 1 (the first JWKS fetch returns an empty key set)"
exchange
if [ "${HTTP_CODE}" = "200" ]; then
    echo "✓ Token issued on the first attempt - Keycloak refetched within the same request"
else
    echo "  Attempt 1 failed (HTTP ${HTTP_CODE}) - refetch is rate-limited on a cold cache:"
    jq -r '"  \(.error): \(.error_description)"' "${SCRIPT_DIR}/generated/token.json" 2>/dev/null || true
fi

echo "== Waiting ${REFETCH_WAIT}s (the JWKS refetch interval)"
sleep "${REFETCH_WAIT}"

echo "== Attempt 2 (the proxy now serves Google's real JWKS)"
exchange

sleep 1
echo "== Proxy fetch log"
cat "${SCRIPT_DIR}/generated/proxy.log"

if [ "${HTTP_CODE}" = "200" ] && \
   grep -q '^EMPTY' "${SCRIPT_DIR}/generated/proxy.log" && \
   grep -q '^REAL' "${SCRIPT_DIR}/generated/proxy.log"; then
    echo "✓ PASS: Keycloak recovered from the unknown kid by refetching the JWKS"
elif [ "${HTTP_CODE}" = "200" ]; then
    echo "⚠ Token issued, but inspect the fetch log - expected an EMPTY then a REAL fetch"
    exit 1
else
    echo "✗ FAIL (HTTP ${HTTP_CODE}): Keycloak did not recover from the unknown kid"
    jq . "${SCRIPT_DIR}/generated/token.json" 2>/dev/null || \
        cat "${SCRIPT_DIR}/generated/token.json"
    exit 1
fi
