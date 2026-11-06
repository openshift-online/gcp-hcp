# EGW Service Identity — POC Plan

**Ticket:** [GCP-1377](https://redhat.atlassian.net/browse/GCP-1377) · Validates the primary design (Google-managed signing). Needs no CIAM client,
no EGW access, and no Marketplace listing. Implementation:
[experiments/egw-service-identity-poc/](../../experiments/egw-service-identity-poc/).

## Questions answered

| # | Question | Why it matters |
|---|---|---|
| 1 | Does `signJwt` preserve our claims (`iss`/`sub` = client_id, `aud` = token endpoint)? | `private_key_jwt` compatibility |
| 2 | Does Keycloak fetch Google's JWKS as `jwks_uri`? | TLS, content type, no redirects |
| 3 | Does Keycloak refetch the JWKS on unknown `kid`? | Load-bearing: Google rotates keys unannounced |
| 4 | Does the full exchange return a token? | End-to-end proof |

## Setup

1. Scratch GSA in a dev project (the real signer will live in `<env>-global`; any GSA works here).
   Enable the IAM Credentials API in the project and grant yourself
   `roles/iam.serviceAccountTokenCreator` on the SA.
2. Local Keycloak (container). Realm `poc`, client `gecko-poc`:
   - Client authentication: on
   - Authentication flow: **Signed JWT** (`private_key_jwt`)
   - `jwks_uri`: `https://www.googleapis.com/service_accounts/v1/jwk/<scratch-sa>`
3. Confirm Keycloak fetches the JWKS (realm logs).

## Steps

1. **Sign an assertion:**

   ```bash
   NOW=$(date +%s)
   cat > payload.json <<EOF
   {"iss": "gecko-poc", "sub": "gecko-poc",
    "aud": "http://localhost:8080/realms/poc/protocol/openid-connect/token",
    "jti": "$(uuidgen)", "iat": $NOW, "exp": $((NOW + 60))}
   EOF
   gcloud iam service-accounts sign-jwt \
     --iam-account=<scratch-sa> --payload-file=payload.json > assertion.jwt
   ```

   Decode the JWT and check: header has `alg: RS256` and a `kid`; claims are exactly as supplied —
   Google adds and overrides nothing (`exp` ≤ 12 h is the only constraint).

2. **Exchange it for a token:**

   ```bash
   curl -s http://localhost:8080/realms/poc/protocol/openid-connect/token \
     -d grant_type=client_credentials \
     -d client_id=gecko-poc \
     -d client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer \
     -d "client_assertion=$(cat assertion.jwt)"
   ```

   Pass: an `access_token`. A `401 invalid_client` with a signature/`kid` error means the JWKS
   fetch failed — recheck Setup step 3.

3. **Prove unknown-`kid` refetch — deterministic, no waiting for a real rotation:**

   Point the client's `jwks_uri` at a tiny local HTTP proxy that serves `{"keys":[]}` on the first
   request and Google's real JWKS on every later request, logging each hit. Run the exchange again:
   success with **two** logged fetches proves Keycloak refetches on unknown `kid` instead of failing
   until a cache TTL expires.

4. **Watch rotation cadence (background, a few days):**

   ```bash
   watch -d 'curl -s https://www.googleapis.com/service_accounts/v1/jwk/<scratch-sa> \
     | jq -r ".keys[].kid"'
   ```

   How often Google actually rotates sets how frequently the unknown-`kid` path is exercised in
   production. Data, not pass/fail.

## Pass criteria

All of: claims preserved; JWKS fetch works; exchange returns a token; refetch-on-unknown-`kid`
demonstrated.

## Results (2026-11-04)

Run against a local Keycloak 26.8 with a throwaway signer in a scratch GCP project. **All criteria
met**, with one caveat worth designing around:

| # | Question | Result |
|---|---|---|
| 1 | Claims preserved by `signJwt` | **Pass** — claims byte-identical to the payload; header is RS256 + `kid` |
| 2 | Keycloak fetches Google's JWKS | **Pass** — fetched over TLS, no redirects, token issued |
| 3 | Refetch on unknown `kid` | **Pass, with a caveat** — see findings |
| 4 | Full exchange | **Pass** — HTTP 200, Bearer token |

Findings:

- **Google publishes a signing key for a brand-new SA immediately** — no first-use warm-up; the
  JWKS URL is usable from the moment the SA exists.
- **Keycloak's unknown-`kid` refetch is rate-limited.** On a cold cache, the first fetch and the
  refetch land inside the rate-limit window (~10 s), so the first exchange after a key rotation
  fails once with `invalid_client: Unable to load public key`; the next attempt after the interval
  succeeds. With a warm cache the refetch happens within the same request. Consequence for gecko:
  the token client must **retry once on `invalid_client` with a short backoff** — the same
  retry-once pattern already used for EGW 401s.
- Implementation note: the exchange uses
  `client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer` (the
  `grant-type:jwt-bearer` URN is for the grant, not the client assertion).

Remaining: these results are for a local Keycloak 26.8 — CIAM still needs to confirm their SSO's
JWKS refetch behaviour and accept the googleapis.com URL.

## If it passes

Create the real signer GSAs in `<env>-global` (Terraform, gcp-hcp-infra) and file the CIAM
enablement with the Google JWKS URLs while the remaining
questions (URL acceptance, validation rules) are answered.

## If it fails

Fallback is the self-managed keypair design. The same Keycloak harness is
reusable: it then verifies our own JWKS URL instead of Google's.
