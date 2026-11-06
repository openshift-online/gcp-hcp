# EGW Service Identity POC

Validates the primary design from [GCP-1377](https://redhat.atlassian.net/browse/GCP-1377): gecko's
client assertions for Red Hat SSO are signed with a **Google-managed service account key** (IAM
Credentials `signJwt`), and the verifier (Keycloak) checks them against **Google's per-SA JWKS**.
Needs no CIAM client, no EGW access, and no Marketplace listing.

Design and pass criteria: [studies/marketplace/egw-service-identity-poc.md](../../studies/marketplace/egw-service-identity-poc.md).

## What it proves

| Script | Question |
|---|---|
| `1-setup-gcp.sh` | A keyless throwaway signer SA exists; `signJwt` needs only `tokenCreator` |
| `2-run-keycloak.sh` | Local Keycloak with the two POC clients (direct Google JWKS, refetch proxy) |
| `3-sign-and-exchange.sh` | `signJwt` preserves our claims; Keycloak fetches Google's JWKS and issues a token |
| `4-test-refetch.sh` | Keycloak refetches the JWKS on unknown `kid` — load-bearing, since Google rotates keys unannounced |

## Prerequisites

- Linux (uses `--network host` and GNU `base64 -d`)
- `gcloud` authenticated, with rights to create a service account in the POC project
- `podman` or `docker`, `python3`, `jq`, `uuidgen`

## Usage

```bash
cp env.example .env        # defaults target the throwaway project cveiga-gcp-hcp-1
source .env
./1-setup-gcp.sh           # create the signer SA
./2-run-keycloak.sh        # local Keycloak (first image pull is slow)
./3-sign-and-exchange.sh   # sign + exchange → token
./4-test-refetch.sh        # empty-JWKS-first proxy → proves unknown-kid recovery
```

## Pass criteria

1. Claims in the signed assertion match the payload exactly (script 3 checks with `diff`).
2. The exchange in script 3 returns an `access_token`.
3. Script 4: attempt 1 may fail — Keycloak rate-limits JWKS fetches (~1 per 10 s), so on a cold
   cache the refetch on unknown `kid` does not happen within the same request. Attempt 2, after
   the interval, must return an `access_token` with an `EMPTY` then a `REAL` fetch logged. This
   mirrors production: after a Google key rotation the first exchange can fail once, and gecko's
   retry recovers.

Failure meanings:

- `401 invalid_client` with a signature/`kid` error in script 3 → the JWKS fetch failed (URL,
  content type, TLS); the script's pre-flight check separates this from "Google has not
  published the key yet".
- `PERMISSION_DENIED` on `signJwt` right after script 1 → the IAM binding is still propagating;
  wait a minute and retry.
- Script 4 attempt 2 still fails with no `REAL` fetch logged → Keycloak is not refetching on
  unknown `kid` (or cached the empty key set): the primary design's main risk.

## Cleanup

`./9-cleanup.sh` removes the Keycloak container and local artifacts, and deletes the throwaway
service account after confirmation.
