#!/usr/bin/env python3
"""JWKS refetch-test proxy for the EGW service identity POC.

Serves an empty JWKS on the first request, then proxies Google's real JWKS
for the signer service account on every later request. Each request is
logged so the test can count how many times Keycloak fetched the key set.

Usage:
    GOOGLE_JWKS_URL=https://www.googleapis.com/service_accounts/v1/jwk/<sa> \
    PROXY_LOG=generated/proxy.log \
    python3 jwks-refetch-proxy.py 9099

/healthz answers 200 without consuming the empty-first state.
"""

import os
import sys
import threading
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

GOOGLE_JWKS_URL = os.environ.get("GOOGLE_JWKS_URL")
PROXY_LOG = os.environ.get("PROXY_LOG", "proxy.log")

state = {"first": True}
state_lock = threading.Lock()


def log(line):
    with open(PROXY_LOG, "a", encoding="utf-8") as f:
        f.write(line + "\n")
    print(line, flush=True)


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/healthz":
            self._send(b'{"ok": true}', note="HEALTH")
            return
        with state_lock:
            first = state["first"]
            state["first"] = False
        if first:
            self._send(b'{"keys": []}', note="EMPTY")
            return
        try:
            with urllib.request.urlopen(GOOGLE_JWKS_URL, timeout=10) as resp:
                body = resp.read()
        except Exception as exc:  # noqa: BLE001 - surface as 502, log the cause
            log(f"ERROR upstream fetch failed: {exc}")
            self.send_error(502, "upstream fetch failed")
            return
        self._send(body, note="REAL")

    def _send(self, body, note="-"):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
        log(f"{note} {self.path}")

    def log_message(self, fmt, *args):
        pass  # keep stdout to the meaningful lines only


if GOOGLE_JWKS_URL is None:
    sys.exit("Set GOOGLE_JWKS_URL (see usage in the module docstring)")

port = int(sys.argv[1]) if len(sys.argv) > 1 else 9099
print(f"jwks-refetch-proxy on 127.0.0.1:{port}, upstream {GOOGLE_JWKS_URL}, log {PROXY_LOG}",
      flush=True)
ThreadingHTTPServer(("127.0.0.1", port), Handler).serve_forever()
