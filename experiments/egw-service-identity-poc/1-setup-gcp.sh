#!/bin/bash
# 1-setup-gcp.sh - Create the throwaway signer service account
#
# Creates a service account with no roles and no user-managed keys. Its
# Google-managed keys will sign the POC assertions via the IAM Credentials
# API. Grants the current gcloud account tokenCreator on it so we can call
# signJwt.
#
# Usage: ./1-setup-gcp.sh   (reads POC_PROJECT_ID / POC_SA_NAME from env or .env)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
[ -f "${SCRIPT_DIR}/.env" ] && source "${SCRIPT_DIR}/.env"

PROJECT="${POC_PROJECT_ID:?Set POC_PROJECT_ID (see env.example)}"
SA_NAME="${POC_SA_NAME:-egw-poc-signer}"
CALLER="$(gcloud config get-value account)"
if [ -z "${CALLER}" ] || [ "${CALLER}" = "(unset)" ]; then
    echo "✗ No active gcloud account; run: gcloud auth login"
    exit 1
fi
# IAM member type: serviceAccount: when impersonating, user: otherwise
case "${CALLER}" in
    *.iam.gserviceaccount.com) CALLER_MEMBER="serviceAccount:${CALLER}" ;;
    *) CALLER_MEMBER="user:${CALLER}" ;;
esac
SA_EMAIL="${SA_NAME}@${PROJECT}.iam.gserviceaccount.com"
JWKS_URL="https://www.googleapis.com/service_accounts/v1/jwk/${SA_EMAIL}"

echo "Project:  ${PROJECT}"
echo "Caller:   ${CALLER}"
echo "Signer:   ${SA_EMAIL}"
echo ""

# signJwt is served by the IAM Credentials API
gcloud services enable iamcredentials.googleapis.com --project "${PROJECT}" >/dev/null

# Create the signer SA if it does not exist yet
if ! gcloud iam service-accounts describe "${SA_EMAIL}" --project "${PROJECT}" >/dev/null 2>&1; then
    gcloud iam service-accounts create "${SA_NAME}" \
        --display-name="EGW POC throwaway signer" \
        --project "${PROJECT}"
else
    echo "Service account already exists: ${SA_EMAIL}"
fi

# Allow the caller to sign JWTs as this SA
gcloud iam service-accounts add-iam-policy-binding "${SA_EMAIL}" \
    --member="${CALLER_MEMBER}" \
    --role="roles/iam.serviceAccountTokenCreator" \
    --project "${PROJECT}" >/dev/null

echo ""
echo "✓ Setup complete"
echo "  JWKS URL: ${JWKS_URL}"
echo ""
echo "Keys currently published for this SA (may be empty until first use):"
curl -fsS "${JWKS_URL}" 2>/dev/null | jq -r '.keys[]? | "  kid: \(.kid)"' || \
    echo "  (no keys published yet - this is fine)"
