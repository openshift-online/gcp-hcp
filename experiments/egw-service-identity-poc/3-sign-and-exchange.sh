#!/bin/bash
# 3-sign-and-exchange.sh - Sign a private_key_jwt assertion with a Google-managed
# key and exchange it for a token at the local Keycloak.
#
# Proves:
#   - gcloud signJwt preserves our claims (iss/sub = client_id, aud = token endpoint)
#   - Keycloak fetches Google's per-SA JWKS and issues a token

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

PROJECT="${POC_PROJECT_ID:?Set POC_PROJECT_ID (see env.example)}"
SA_NAME="${POC_SA_NAME:-egw-poc-signer}"
SA_EMAIL="${SA_NAME}@${PROJECT}.iam.gserviceaccount.com"
PORT="${POC_KEYCLOAK_PORT:-8080}"

CLIENT_ID="gecko-poc"
TOKEN_URL="http://localhost:${PORT}/realms/poc/protocol/openid-connect/token"

b64url_decode() {
    local s="$1"
    case $((${#s} % 4)) in
        2) s="${s}==" ;;
        3) s="${s}=" ;;
    esac
    printf '%s' "$s" | tr '_-' '/+' | base64 -d
}

mkdir -p "${SCRIPT_DIR}/generated"

NOW=$(date +%s)
cat > "${SCRIPT_DIR}/generated/payload.json" <<EOF
{
  "iss": "${CLIENT_ID}",
  "sub": "${CLIENT_ID}",
  "aud": "${TOKEN_URL}",
  "jti": "$(uuidgen)",
  "iat": ${NOW},
  "exp": $((NOW + 60))
}
EOF

echo "== Signing as ${SA_EMAIL}"
gcloud iam service-accounts sign-jwt \
    --iam-account="${SA_EMAIL}" \
    "${SCRIPT_DIR}/generated/payload.json" \
    "${SCRIPT_DIR}/generated/assertion.jwt"

echo "== Assertion header"
b64url_decode "$(cut -d. -f1 "${SCRIPT_DIR}/generated/assertion.jwt")" | jq .
echo "== Assertion claims"
b64url_decode "$(cut -d. -f2 "${SCRIPT_DIR}/generated/assertion.jwt")" | jq .

KID=$(b64url_decode "$(cut -d. -f1 "${SCRIPT_DIR}/generated/assertion.jwt")" | jq -r .kid)
echo "== Pre-flight: is kid ${KID} published in Google's JWKS?"
if curl -fsS "https://www.googleapis.com/service_accounts/v1/jwk/${SA_EMAIL}" \
        | jq -e --arg kid "${KID}" '.keys[]? | select(.kid == $kid)' >/dev/null; then
    echo "✓ kid is published"
else
    echo "✗ kid is NOT in Google's JWKS - Google has not published the signing key yet"
    exit 1
fi

echo "== Checking claims are preserved exactly"
if diff <(jq -S . "${SCRIPT_DIR}/generated/payload.json") \
        <(b64url_decode "$(cut -d. -f2 "${SCRIPT_DIR}/generated/assertion.jwt")" | jq -S .); then
    echo "✓ Claims preserved"
else
    echo "✗ Claims differ from the payload - Google modified them"
    exit 1
fi

echo "== Exchanging at ${TOKEN_URL}"
HTTP_CODE=$(curl -s --max-time 30 -o "${SCRIPT_DIR}/generated/token.json" -w '%{http_code}' "${TOKEN_URL}" \
    -d grant_type=client_credentials \
    -d client_id="${CLIENT_ID}" \
    -d client_assertion_type="urn:ietf:params:oauth:client-assertion-type:jwt-bearer" \
    -d client_assertion="$(cat "${SCRIPT_DIR}/generated/assertion.jwt")")

if [ "${HTTP_CODE}" != "200" ]; then
    echo "✗ FAIL (HTTP ${HTTP_CODE}):"
    jq . "${SCRIPT_DIR}/generated/token.json" 2>/dev/null || \
        cat "${SCRIPT_DIR}/generated/token.json"
    exit 1
fi

echo "✓ PASS: token issued"
echo "== Access token claims"
b64url_decode "$(jq -r .access_token "${SCRIPT_DIR}/generated/token.json" | cut -d. -f2)" | jq .
