#!/bin/sh
# Runs at container start (nginx image entrypoint). Renders the SPA into a
# writable directory (the root filesystem is read-only) and injects the public
# configuration into the page's meta tag — no inline script, so the page's CSP
# needs no 'unsafe-inline'.
set -eu

: "${SIGNUP_CLIENT_ID:?SIGNUP_CLIENT_ID must be set (public OAuth client ID)}"
: "${SIGNUP_API_BASE:=}"

target=/usr/share/nginx/html
mkdir -p "$target"

sed   -e "s|data-client-id=\"\"|data-client-id=\"${SIGNUP_CLIENT_ID}\"|"   -e "s|data-api-base=\"\"|data-api-base=\"${SIGNUP_API_BASE}\"|"   /etc/signup/index.html > "$target/index.html"
cp /etc/signup/app.js /etc/signup/styles.css "$target/"

echo "signup SPA configured (api base: ${SIGNUP_API_BASE:-same-origin})"
exec nginx -g 'daemon off;'
