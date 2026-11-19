#!/usr/bin/env python3
"""Two approved negative public-API patches; never appends an IAM assertion.

The payload after each failing test is the exact existing application list.
Each mutation is explicitly journaled and attempted at most once. No retries,
gate changes, cloud IAM writes, fixture allocation, or release requests.
"""

import hashlib
import fcntl
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
from datetime import datetime, timezone
import urllib.error
import urllib.request


API = "https://platform-api-northamerica-northeast2-apahim1110.dev.gcp-hcp.devshift.net"
PATH = "/apis/gcp.managed.openshift.io/v1/namespaces/apahim-dev-test-1/clusters/wif3-first"
CONTEXT = "gke_dev-reg-na-ne2-apahim1110_northamerica-northeast2_dev-reg-na-ne2-apahim1110-gke"
UID = "6b152684-797c-4f17-a258-43538f8013c8"
DIRECTORY = Path("/Users/asegundo/.local/state/gcp-hcp/wif-phase4-api-guards-20261119")
STATE = DIRECTORY / "guard-state.json"
EXPECTED_REF = {
    "name": "5.0.0",
    "uid": "fb623828-d923-4540-82f1-df5c01503823",
    "digest": "sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71",
}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def digest(value):
    encoded = json.dumps(value, sort_keys=True, separators=(",", ":")).encode()
    return "sha256:" + hashlib.sha256(encoded).hexdigest()


def timestamp():
    return datetime.now(timezone.utc).isoformat()


def private(path, directory=False):
    info = path.lstat()
    expected = stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)
    if not expected or info.st_uid != os.geteuid() or info.st_mode & 0o077:
        raise RuntimeError("state ownership, mode or type changed")


def command(arguments):
    result = subprocess.run(arguments, capture_output=True, text=True, timeout=45, check=False)
    if result.returncode:
        raise RuntimeError("authenticated read command failed; raw diagnostics suppressed")
    return result.stdout


def kubectl(*arguments):
    return json.loads(command(["kubectl", "--context", CONTEXT, "--request-timeout=30s", "get", *arguments]))


def gates_and_private():
    template = kubectl("--raw", "/apis/gcp.managed.openshift.io/v1/wifconfigtemplates/5.0.0")
    deployment = kubectl("deployment", "platform-api-server", "-n", "gecko-system", "-o", "json")
    arguments = next(c["args"] for c in deployment["spec"]["template"]["spec"]["containers"] if c["name"] == "platform-api-server")
    if template["spec"].get("verified") is not False or "--enable-iam-application-writes=false" not in arguments:
        raise RuntimeError("both new-use gates must remain explicitly false")
    cluster = kubectl("--raw", PATH)
    return {
        "templateVerified": False,
        "iamApplicationWrites": False,
        "receipt": cluster.get("status", {}).get("iamDispatch"),
        "upgrade": cluster.get("status", {}).get("controlPlaneUpgrade"),
    }


def identity_token():
    token = command(["gcloud", "auth", "print-identity-token"]).strip()
    if not token:
        raise RuntimeError("identity token is unavailable")
    return token


def request(method="GET", body=None, token=None):
    if token is None:
        token = identity_token()
    headers = {"Authorization": "Bearer " + token, "Accept": "application/json"}
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json-patch+json"
        data = json.dumps(body).encode()
    req = urllib.request.Request(API + PATH, headers=headers, method=method, data=data)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        with opener.open(req, timeout=45) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        return error.code, error.read()
    except Exception:
        raise RuntimeError("request outcome unknown; retain intent and do not repeat PATCH") from None


def read_public():
    code, body = request()
    if code != 200:
        raise RuntimeError("authenticated public read failed")
    cluster = json.loads(body)
    if cluster["metadata"]["uid"] != UID or cluster["metadata"].get("deletionTimestamp"):
        raise RuntimeError("retained Cluster identity/deletion state changed")
    if cluster["spec"]["platform"]["gcp"]["iam"]["templateApplications"] != [EXPECTED_REF]:
        raise RuntimeError("application history no longer matches the approved unchanged list")
    if cluster["spec"]["release"] != {"version": "5.0.2", "channelGroup": "candidate"}:
        raise RuntimeError("retained release changed")
    return {
        "uid": UID,
        "generation": cluster["metadata"]["generation"],
        "resourceVersion": cluster["metadata"]["resourceVersion"],
        "specDigest": digest(cluster["spec"]),
        "applications": [EXPECTED_REF],
    }


def save(state, first=False):
    private(DIRECTORY, True)
    data = (json.dumps(state, indent=2, sort_keys=True) + "\n").encode()
    temporary = DIRECTORY / (".guards-" + os.urandom(8).hex())
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        if first:
            os.link(temporary, STATE)
        else:
            private(STATE)
            os.replace(temporary, STATE)
        directory = os.open(DIRECTORY, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        temporary.unlink(missing_ok=True)


def load():
    private(STATE)
    state = json.loads(STATE.read_text())
    if state.get("kind") != "WIFAPIGuardRehearsal/v1" or state.get("api") != API or state.get("path") != PATH:
        raise RuntimeError("unexpected guard rehearsal identity")
    return state


def compare(original, current):
    for key in ("uid", "generation", "specDigest", "applications"):
        if original[key] != current[key]:
            raise RuntimeError("retained Cluster changed; stop without rollback or another PATCH")


def run(mode):
    for path in DIRECTORY.parents:
        if path.is_symlink():
            raise RuntimeError("state parent is a symlink")
    DIRECTORY.mkdir(mode=0o700, parents=True, exist_ok=True)
    private(DIRECTORY, True)
    lock_path = DIRECTORY / "guard-state.lock"
    lock_fd = os.open(lock_path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    with os.fdopen(lock_fd, "rb") as lock:
        private(lock_path)
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        run_locked(mode)


def run_locked(mode):
    if mode == "baseline":
        if STATE.exists() or STATE.is_symlink():
            raise RuntimeError("guard journal already exists; do not repeat writes")
        public, internal = read_public(), gates_and_private()
        state = {"kind": "WIFAPIGuardRehearsal/v1", "api": API, "path": PATH, "baseline": public, "privateBaseline": internal, "cases": {}, "observedAt": timestamp()}
        save(state, True)
        print(json.dumps(state))
        return
    state = load()
    current = read_public()
    compare(state["baseline"], current)
    internal = gates_and_private()
    if internal != state["privateBaseline"]:
        raise RuntimeError("private receipt/upgrade/gate state changed")
    if mode == "readback":
        if set(state["cases"]) != {"wrong-uid", "stale-rv"} or not all(c.get("rejectedByTest") for c in state["cases"].values()):
            raise RuntimeError("both precise negative guards have not passed")
        state["after"] = current
        state["privateAfter"] = internal
        state["phase"] = "Validated"
        state["observedAt"] = timestamp()
        save(state)
        print(json.dumps(state))
        return
    if mode not in ("wrong-uid", "stale-rv") or mode in state["cases"]:
        raise RuntimeError("unsupported or already-attempted PATCH; no retries permitted")
    if mode == "stale-rv" and not state["cases"].get("wrong-uid", {}).get("rejectedByTest"):
        raise RuntimeError("wrong-UID guard must pass first")
    wrong_uid = "00000000-0000-0000-0000-000000000000"
    if current["uid"] == wrong_uid or current["resourceVersion"] == "0":
        raise RuntimeError("negative test value unexpectedly matches real identity")
    patch = [{"op": "test", "path": "/metadata/uid", "value": wrong_uid if mode == "wrong-uid" else UID}]
    guard = "/metadata/uid"
    if mode == "stale-rv":
        guard = "/metadata/resourceVersion"
        patch.append({"op": "test", "path": guard, "value": "0"})
    patch.append({"op": "replace", "path": "/spec/platform/gcp/iam/templateApplications", "value": current["applications"]})
    case = {"phase": "Intent", "request": patch, "before": current, "observedAt": timestamp()}
    # Resolve authentication before durable write intent; an auth failure must
    # not burn a single-shot PATCH when no HTTP request could have been sent.
    token = identity_token()
    state["cases"][mode] = case
    save(state)
    code, body = request("PATCH", patch, token=token)
    # Never print raw provider/server prose, tokens or headers. Require evidence
    # of the JSONPatch test failure, not a 401/403 or generic admission block.
    text = body.decode("utf-8", errors="replace")
    case["httpStatus"] = code
    case["responseDigest"] = "sha256:" + hashlib.sha256(body).hexdigest()
    case["guardPathMentioned"] = guard in text
    marker = "testing value " + guard + " failed"
    case["testValueMismatchMarker"] = marker in text and "test failed" in text
    case["missingPathMarker"] = "missing" in text.lower() or "not found" in text.lower()
    case["matchedErrorMarker"] = marker + ": test failed" if case["testValueMismatchMarker"] else None
    case["rejectedByTest"] = code in (400, 409, 412, 422) and case["testValueMismatchMarker"] and not case["missingPathMarker"]
    case["phase"] = "Rejected" if case["rejectedByTest"] else "UnexpectedResponse"
    save(state)
    if not case["rejectedByTest"]:
        print(json.dumps({"case": mode, "httpStatus": code, "guardPathMentioned": case["guardPathMentioned"], "rejectedByTest": False}))
        raise RuntimeError("guard rejection not specifically established; stop")
    after = read_public()
    compare(current, after)
    case["after"] = after
    case["observedAt"] = timestamp()
    save(state)
    print(json.dumps({"case": mode, **case}))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("one explicit mode required: baseline | wrong-uid | stale-rv | readback")
    try:
        run(sys.argv[1])
    except Exception:
        raise SystemExit("guard rehearsal stopped; retain protected journal and inspect approved diagnostics; raw errors suppressed") from None
