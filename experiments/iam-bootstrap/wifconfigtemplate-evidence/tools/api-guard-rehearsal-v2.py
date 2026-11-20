#!/usr/bin/env python3
"""Fresh, bounded negative JSONPatch rehearsal for deployed Gecko 7319d8e.

Never imports/runs the retired harness. Exactly one attempt per negative case,
durable intent before HTTP, no retries, no assertion append or IAM operation.
"""

import fcntl
import hashlib
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
BASE = "/apis/gcp.managed.openshift.io/v1"
CONTEXT = "gke_dev-reg-na-ne2-apahim1110_northamerica-northeast2_dev-reg-na-ne2-apahim1110-gke"
UID = "6b152684-797c-4f17-a258-43538f8013c8"
NODEPOOL_UID = "a8768aef-c352-47bb-9f32-3638275f1e15"
SOURCE = "7319d8ea790a18163279ac407e4f08c676609e88"
IMAGE = "quay.io/apahim/gecko:wif-p4-jsonpatch-7319d8e-api"
IMAGE_ID = "quay.io/apahim/gecko@sha256:5f4a2ad3201aff2d0a6dd01591a67480000cc7609a31d660cde2b7831c328a2c"
RELEASE_IMAGE = "quay.io/openshift-release-dev/ocp-release@sha256:7c739d899b33a56060422580779b22bf859d59925ec130d6660d6fc4dc30a2e8"
SPEC_DIGEST = "sha256:d8d739a85da263b872e1d20c8189aa7a0a1440e213adebb63a9f9f7468669a26"
EXPECTED_REF = {
    "name": "5.0.0",
    "uid": "fb623828-d923-4540-82f1-df5c01503823",
    "digest": "sha256:c0b4c3dbdd48d1925bf7c172ac0385639d00092f083b5cac11865e83c3acea71",
}
DIRECTORY = Path("/Users/asegundo/.local/state/gcp-hcp/wif-phase4-api-guards-jsonpatch-20261120")
STATE = DIRECTORY / "guard-state.json"
PROTECTED = {
    Path("/Users/asegundo/.local/state/gcp-hcp/wif-phase3-20261118/primary-state.json"):
        "6516c625a9d55b8a9748a2385c8b822085f5fc778a78e353a77f6ef2f754c995",
    Path("/Users/asegundo/.local/state/gcp-hcp/wif-phase4-api-guards-20261119/guard-state.json"):
        "af1f4abf8e822266f8a089bf03773afb8eb908083ca31f28f19e1a025d178588",
}
PRIMARY = next(iter(PROTECTED))
MAX_BODY = 1024 * 1024


def timestamp():
    return datetime.now(timezone.utc).isoformat()


def digest(value):
    return "sha256:" + hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def private(path, directory=False):
    info = path.lstat()
    expected = stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode)
    if not expected or info.st_uid != os.geteuid() or info.st_mode & 0o077:
        raise RuntimeError("unexpected state ownership, mode or type")


def protected_unchanged():
    for path, expected in PROTECTED.items():
        private(path)
        if hashlib.sha256(path.read_bytes()).hexdigest() != expected:
            raise RuntimeError("protected journal changed")
    sidecar = PRIMARY.with_name(PRIMARY.name + ".updates.json")
    if sidecar.exists() or sidecar.is_symlink():
        raise RuntimeError("unexpected primary update sidecar")


def command(arguments):
    result = subprocess.run(arguments, capture_output=True, text=True, timeout=45, check=False)
    if result.returncode:
        raise RuntimeError("authenticated read failed; raw diagnostics suppressed")
    return result.stdout


def kubectl(*arguments):
    return json.loads(command(["kubectl", "--context", CONTEXT, "--request-timeout=30s", "get", *arguments]))


def condition(obj, name):
    return next(c["status"] for c in obj["status"]["conditions"] if c["type"] == name)


def private_snapshot():
    protected_unchanged()
    template = kubectl("--raw", BASE + "/wifconfigtemplates/5.0.0")
    deployments = kubectl("deployments", "-n", "gecko-system", "-o", "json")["items"]
    if len(deployments) != 8 or any(d["spec"]["replicas"] != 1 or d["status"].get("readyReplicas") != 1 for d in deployments):
        raise RuntimeError("API/controller readiness changed")
    deployment = next(d for d in deployments if d["metadata"]["name"] == "platform-api-server")
    container = next(c for c in deployment["spec"]["template"]["spec"]["containers"] if c["name"] == "platform-api-server")
    if template["metadata"]["uid"] != EXPECTED_REF["uid"] or template["spec"].get("verified") is not False or "--enable-iam-application-writes=false" not in container["args"]:
        raise RuntimeError("both gates must remain explicitly false")
    if container["image"] != IMAGE or not any(e.get("name") == "SPANNER_DATABASE" and e.get("value") == "projects/dev-reg-na-ne2-apahim1110/instances/dev-reg-na-ne2-apahim1110-spanner/databases/gecko" for e in container.get("env", [])):
        raise RuntimeError("approved image/backend changed")
    pods = kubectl("pods", "-n", "gecko-system", "-o", "json")["items"]
    selector = deployment["spec"]["selector"]["matchLabels"]
    if not selector:
        raise RuntimeError("API Deployment selector unavailable")
    # The retained migration Job uses the same name prefix/common labels, but
    # has a Job owner. Only Deployment/ReplicaSet pods participate in rollout.
    api_pods = [p for p in pods if
                all(p["metadata"].get("labels", {}).get(k) == v for k, v in selector.items())
                and any(o.get("kind") == "ReplicaSet" and o.get("controller") is True
                        for o in p["metadata"].get("ownerReferences", []))]
    if len(api_pods) != 1 or api_pods[0]["metadata"].get("deletionTimestamp"):
        raise RuntimeError("API rollout is not quiescent")
    statuses = api_pods[0]["status"]["containerStatuses"]
    if not all(c["ready"] for c in statuses) or next(c["imageID"] for c in statuses if c["name"] == "platform-api-server") != IMAGE_ID:
        raise RuntimeError("running API artifact/readiness changed")
    app = kubectl("application", "platform-api-server", "-n", "argocd", "-o", "json")
    if app["spec"]["source"]["targetRevision"] != SOURCE or app["status"]["sync"]["revision"] != SOURCE or app["status"]["sync"]["status"] != "Synced" or app["status"]["health"]["status"] != "Healthy":
        raise RuntimeError("approved API source is not Synced/Healthy")
    if not any(r.get("kind") == "Job" and r.get("name") == "platform-api-server-migrate" and r.get("status") == "Synced" for r in app["status"].get("resources", [])):
        raise RuntimeError("migration Job is not Synced")
    job = kubectl("job", "platform-api-server-migrate", "-n", "gecko-system", "-o", "json")
    if job["metadata"]["uid"] != "daa14792-1593-4a92-abe8-2b8f32ba3fd1" or job["metadata"]["creationTimestamp"] != "2026-11-17T19:25:31Z" or condition(job, "Complete") != "True":
        raise RuntimeError("original migration Job changed")
    if [c["image"] for c in job["spec"]["template"]["spec"]["containers"]] != ["quay.io/apahim/gecko:wif-p2-cel-54709c4-api"]:
        raise RuntimeError("migration image changed")
    spanner = kubectl("spannerdatabase", "gecko", "-n", "gecko-system", "-o", "json")
    if spanner["metadata"]["uid"] != "99e4163c-79f9-4875-92b9-0e576b9a5927" or condition(spanner, "Ready") != "True":
        raise RuntimeError("original Spanner state changed")
    cluster = kubectl("--raw", PATH)
    pool = kubectl("--raw", BASE + "/namespaces/apahim-dev-test-1/nodepools/workers")
    stable = {}
    for name, obj, uid, generation in (("cluster", cluster, UID, 2), ("nodepool", pool, NODEPOOL_UID, 1)):
        status = obj.get("status", {})
        receipt = status.get("iamDispatch", {})
        if obj["metadata"]["uid"] != uid or obj["metadata"]["generation"] != generation or obj["metadata"].get("deletionTimestamp"):
            raise RuntimeError("retained identity/generation changed")
        expected_receipt = {"objectUID": uid, "clusterUID": UID, "releaseVersion": "5.0.2", "releaseImage": RELEASE_IMAGE, "channelGroup": "candidate", "templateRef": EXPECTED_REF}
        if receipt != {"dispatched": expected_receipt} or status.get("controlPlaneUpgrade"):
            raise RuntimeError("retained receipt/upgrade state changed")
        if obj["spec"].get("release") != {"version": "5.0.2", "channelGroup": "candidate"}:
            raise RuntimeError("retained release changed")
        if status.get("iamRequirements", {}).get("state") != "Complete":
            raise RuntimeError("requirements are not Complete")
        stable[name] = {"uid": uid, "generation": generation, "specDigest": digest(obj["spec"]), "receipt": receipt, "upgrade": status.get("controlPlaneUpgrade")}
    if pool["spec"]["nodeCount"] != 1 or condition(pool, "NodePoolHealthy") != "True" or condition(cluster, "HostedClusterAvailable") != "True":
        raise RuntimeError("retained installation health changed")
    deployment_specs = [{"name": d["metadata"]["name"], "uid": d["metadata"]["uid"], "spec": d["spec"]} for d in sorted(deployments, key=lambda d: d["metadata"]["name"])]
    stable.update({
        "templateUID": template["metadata"]["uid"],
        "templateSpecDigest": digest(template["spec"]),
        "templateVerified": False,
        "iamApplicationWrites": False,
        "deploymentSpecs": digest(deployment_specs),
        "apiPodUID": api_pods[0]["metadata"]["uid"],
        "apiImageID": IMAGE_ID,
        "applicationSpecDigest": digest(app["spec"]),
        "jobSpecDigest": digest(job["spec"]),
        "spannerSpecDigest": digest(spanner["spec"]),
    })
    return stable


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def identity_token():
    token = command(["gcloud", "auth", "print-identity-token", "--quiet"]).strip()
    if not token:
        raise RuntimeError("identity token unavailable")
    return token


def request(method="GET", body=None, token=None):
    if method not in ("GET", "PATCH") or (method == "GET" and body is not None):
        raise RuntimeError("unsupported request")
    token = identity_token() if token is None else token
    headers = {"Authorization": "Bearer " + token, "Accept": "application/json"}
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json-patch+json"
        data = json.dumps(body).encode()
    req = urllib.request.Request(API + PATH, headers=headers, method=method, data=data)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        try:
            response = opener.open(req, timeout=45)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            result = response.read(MAX_BODY + 1)
            if len(result) > MAX_BODY:
                raise RuntimeError("response exceeded bound")
            return response.code, result
    except Exception:
        raise RuntimeError("request outcome uncertain; retain intent and never repeat PATCH") from None


def read_public():
    code, body = request()
    if code != 200:
        raise RuntimeError("public GET failed")
    obj = json.loads(body)
    result = {"uid": obj["metadata"]["uid"], "generation": obj["metadata"]["generation"], "resourceVersion": obj["metadata"]["resourceVersion"], "specDigest": digest(obj["spec"]), "applications": obj["spec"]["platform"]["gcp"]["iam"]["templateApplications"]}
    if obj["metadata"].get("deletionTimestamp") or result["uid"] != UID or result["generation"] != 2 or result["specDigest"] != SPEC_DIGEST or result["applications"] != [EXPECTED_REF] or result["resourceVersion"] in ("", "0"):
        raise RuntimeError("public retained snapshot changed")
    return result


def compare(original, current):
    if any(original[k] != current[k] for k in ("uid", "generation", "specDigest", "applications")):
        raise RuntimeError("retained public state changed")


def make_patch(mode, current):
    if mode not in ("wrong-uid", "stale-rv") or current["uid"] != UID or current["resourceVersion"] in ("", "0") or current["applications"] != [EXPECTED_REF]:
        raise RuntimeError("unexpected negative request input")
    return [
        {"op": "test", "path": "/metadata/uid", "value": "00000000-0000-0000-0000-000000000000" if mode == "wrong-uid" else UID},
        {"op": "test", "path": "/metadata/resourceVersion", "value": current["resourceVersion"] if mode == "wrong-uid" else "0"},
        {"op": "replace", "path": "/spec/platform/gcp/iam/templateApplications", "value": current["applications"]},
    ]


def rejected_by_test(code, body):
    try:
        result = json.loads(body)
    except (ValueError, UnicodeDecodeError, RecursionError):
        return False
    return isinstance(result, dict) and code == 409 and result.get("code") == 409 and result.get("apiVersion") == "v1" and result.get("kind") == "Status" and result.get("status") == "Failure" and result.get("message") == "JSON patch test failed"


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


def harness_digest():
    return hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


def load():
    private(STATE)
    state = json.loads(STATE.read_text())
    if (state.get("kind"), state.get("api"), state.get("path"), state.get("source"), state.get("harnessSHA256")) != ("WIFAPIGuardRehearsal/v2", API, PATH, SOURCE, harness_digest()):
        raise RuntimeError("unexpected rehearsal/source/harness identity")
    return state


def snapshot():
    internal = private_snapshot()
    public = read_public()
    return public, internal


def run_locked(mode):
    if mode == "baseline":
        if STATE.exists() or STATE.is_symlink():
            raise RuntimeError("fresh state required; never reset a journal")
        public, internal = snapshot()
        state = {"kind": "WIFAPIGuardRehearsal/v2", "api": API, "path": PATH, "source": SOURCE, "harnessSHA256": harness_digest(), "baseline": public, "privateBaseline": internal, "cases": {}, "observedAt": timestamp()}
        save(state, True)
        print(json.dumps({"phase": "Baseline", "observedAt": state["observedAt"], "resourceVersion": public["resourceVersion"], "specDigest": public["specDigest"]}))
        return
    state = load()
    if mode not in ("wrong-uid", "stale-rv", "readback") or mode in state["cases"]:
        raise RuntimeError("unsupported/already-attempted case; no retries")
    if mode == "stale-rv" and state["cases"].get("wrong-uid", {}).get("phase") != "Validated":
        raise RuntimeError("wrong UID must be validated first")
    if mode == "readback" and (set(state["cases"]) != {"wrong-uid", "stale-rv"} or not all(c.get("phase") == "Validated" and c.get("rejectedByTest") for c in state["cases"].values())):
        raise RuntimeError("both guards must be validated")
    current, internal = snapshot()
    compare(state["baseline"], current)
    if internal != state["privateBaseline"]:
        raise RuntimeError("private/gate/artifact state changed")
    if mode == "readback":
        state.update({"phase": "Validated", "after": current, "privateAfter": internal, "observedAt": timestamp()})
        save(state)
        print(json.dumps({"phase": "Validated", "observedAt": state["observedAt"], "resourceVersion": current["resourceVersion"], "specDigest": current["specDigest"]}))
        return
    patch = make_patch(mode, current)
    token = identity_token()  # Auth failure must precede durable PATCH intent.
    case = {"phase": "Intent", "request": patch, "before": current, "observedAt": timestamp()}
    state["cases"][mode] = case
    save(state)
    code, body = request("PATCH", patch, token=token)
    accepted = rejected_by_test(code, body)
    case.update({"phase": "RejectionObserved" if accepted else "UnexpectedResponse", "httpStatus": code, "responseDigest": "sha256:" + hashlib.sha256(body).hexdigest(), "rejectedByTest": accepted})
    save(state)
    if not accepted:
        raise RuntimeError("specific guard failure not established; stop")
    after, after_internal = snapshot()
    compare(current, after)
    if after_internal != internal:
        raise RuntimeError("private/gate/artifact state changed after PATCH")
    case.update({"phase": "Validated", "after": after, "observedAt": timestamp()})
    save(state)
    print(json.dumps({"case": mode, "phase": "Validated", "httpStatus": code, "rejectedByTest": True, "responseDigest": case["responseDigest"], "observedAt": case["observedAt"]}))


def run(mode):
    if mode not in ("baseline", "wrong-uid", "stale-rv", "readback"):
        raise RuntimeError("explicit supported mode required")
    for path in DIRECTORY.parents:
        if path.is_symlink():
            raise RuntimeError("symlink state parent")
    DIRECTORY.mkdir(mode=0o700, parents=True, exist_ok=True)
    private(DIRECTORY, True)
    lock_path = DIRECTORY / "guard-state.lock"
    fd = os.open(lock_path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "rb") as lock:
        private(lock_path)
        fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        run_locked(mode)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("one mode required: baseline | wrong-uid | stale-rv | readback")
    try:
        run(sys.argv[1])
    except Exception:
        raise SystemExit("fresh guard rehearsal stopped; preserve intent, never retry PATCH; raw diagnostics suppressed") from None
