"""Credential-free tests for the fresh v2 harness; never imports the old one."""

import contextlib
import copy
import fcntl
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import urllib.error


TMP = "/private/var/folders/f4/sf1v545n5r140_7b1swtprz40000gn/T/opencode"


class FreshGuardRehearsalTest(unittest.TestCase):
    def setUp(self):
        path = Path(__file__).with_name("api-guard-rehearsal-v2.py")
        spec = importlib.util.spec_from_file_location("fresh_guard_rehearsal", path)
        self.m = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.m)
        temp = tempfile.TemporaryDirectory(prefix="wif-api-guards-v2-test-", dir=TMP)
        self.addCleanup(temp.cleanup)
        self.m.DIRECTORY = Path(temp.name).resolve()
        self.m.STATE = self.m.DIRECTORY / "guard-state.json"
        self.public = {"uid": self.m.UID, "generation": 2, "resourceVersion": "501", "specDigest": self.m.SPEC_DIGEST, "applications": [self.m.EXPECTED_REF]}
        self.internal = {"templateVerified": False, "iamApplicationWrites": False, "receipt": {"dispatched": "offline"}}
        self.snapshot_patch = mock.patch.object(self.m, "snapshot", side_effect=lambda: (copy.deepcopy(self.public), copy.deepcopy(self.internal)))
        self.snapshot = self.snapshot_patch.start()
        self.addCleanup(self.snapshot_patch.stop)
        for name, replacement in (("identity_token", lambda: "offline-token"), ("command", mock.Mock(side_effect=AssertionError("no external command allowed")))):
            p = mock.patch.object(self.m, name, replacement)
            p.start()
            self.addCleanup(p.stop)

    def mode(self, mode, locked=True):
        with contextlib.redirect_stdout(io.StringIO()):
            (self.m.run_locked if locked else self.m.run)(mode)

    @staticmethod
    def failure():
        return json.dumps({"kind": "Status", "apiVersion": "v1", "metadata": {}, "status": "Failure", "message": "JSON patch test failed", "code": 409}).encode()

    def test_exact_two_requests_intent_first_and_unchanged_history(self):
        self.mode("baseline", locked=False)
        calls = []

        def request(method, patch, token=None):
            mode = ("wrong-uid", "stale-rv")[len(calls)]
            self.assertEqual((method, token), ("PATCH", "offline-token"))
            state = self.m.load()
            self.assertEqual(state["cases"][mode]["phase"], "Intent")
            self.assertEqual(patch, state["cases"][mode]["request"])
            self.assertEqual(len(patch), 3)
            self.assertEqual(patch[0], {"op": "test", "path": "/metadata/uid", "value": "00000000-0000-0000-0000-000000000000" if mode == "wrong-uid" else self.m.UID})
            self.assertEqual(patch[1], {"op": "test", "path": "/metadata/resourceVersion", "value": "501" if mode == "wrong-uid" else "0"})
            self.assertEqual(patch[2], {"op": "replace", "path": "/spec/platform/gcp/iam/templateApplications", "value": [self.m.EXPECTED_REF]})
            calls.append(mode)
            return 409, self.failure()

        with mock.patch.object(self.m, "request", request):
            self.mode("wrong-uid", locked=False)
            self.mode("stale-rv", locked=False)
            self.public["resourceVersion"] = "502"  # Normal status refresh is allowed.
            self.mode("readback", locked=False)
            with self.assertRaises(RuntimeError):
                self.mode("wrong-uid")
        self.assertEqual(calls, ["wrong-uid", "stale-rv"])
        state = self.m.load()
        self.assertEqual(state["phase"], "Validated")
        self.assertEqual(state["after"]["resourceVersion"], "502")
        self.assertEqual(os.stat(self.m.STATE).st_mode & 0o777, 0o600)
        self.assertEqual(os.stat(self.m.DIRECTORY).st_mode & 0o777, 0o700)

    def test_classifier_requires_exact_status_contract_not_generic_4xx(self):
        self.assertTrue(self.m.rejected_by_test(409, self.failure()))
        valid = json.loads(self.failure())
        bodies = [b"not JSON", b"null", b"[]", b'"JSON patch test failed"', b'{"error":"JSON patch test failed"}', b'{"request":{"message":"JSON patch test failed"}}']
        for field in ("code", "kind", "apiVersion", "status", "message"):
            body = copy.deepcopy(valid)
            body[field] = "wrong"
            bodies.append(json.dumps(body).encode())
        echoed = copy.deepcopy(valid)
        echoed["message"] += ": missing path"
        bodies.append(json.dumps(echoed).encode())
        for body in bodies:
            with self.subTest(body=body):
                self.assertFalse(self.m.rejected_by_test(409, body))
        for code in (200, 302, 400, 401, 403, 412, 422, 429, 500):
            self.assertFalse(self.m.rejected_by_test(code, self.failure()))

    def test_unexpected_response_stops_both_cases_without_retry(self):
        self.mode("baseline")
        request = mock.Mock(return_value=(403, b'{"message":"forbidden"}'))
        with mock.patch.object(self.m, "request", request):
            for mode in ("wrong-uid", "wrong-uid", "stale-rv", "readback"):
                with self.assertRaises(RuntimeError):
                    self.mode(mode)
        self.assertEqual(request.call_count, 1)
        self.assertEqual(self.m.load()["cases"]["wrong-uid"]["phase"], "UnexpectedResponse")

    def test_uncertain_transport_leaves_intent_and_never_repeats(self):
        self.mode("baseline")
        request = mock.Mock(side_effect=RuntimeError("offline lost response"))
        with mock.patch.object(self.m, "request", request):
            for mode in ("wrong-uid", "wrong-uid", "stale-rv"):
                with self.assertRaises(RuntimeError):
                    self.mode(mode)
        self.assertEqual(request.call_count, 1)
        self.assertEqual(self.m.load()["cases"]["wrong-uid"]["phase"], "Intent")

    def test_failed_postcheck_does_not_clear_the_next_case(self):
        self.mode("baseline")
        changed = copy.deepcopy(self.public)
        changed["specDigest"] = "changed"
        self.snapshot.side_effect = [(self.public, self.internal), (changed, self.internal)]
        with mock.patch.object(self.m, "request", return_value=(409, self.failure())) as request:
            with self.assertRaises(RuntimeError):
                self.mode("wrong-uid")
            with self.assertRaises(RuntimeError):
                self.mode("stale-rv")
        self.assertEqual(request.call_count, 1)
        self.assertEqual(self.m.load()["cases"]["wrong-uid"]["phase"], "RejectionObserved")

    def test_auth_or_intent_save_failure_never_sends_patch(self):
        self.mode("baseline")
        for name in ("identity_token", "save"):
            with self.subTest(name=name), mock.patch.object(self.m, name, side_effect=RuntimeError("offline failure")), mock.patch.object(self.m, "request") as request:
                with self.assertRaises(RuntimeError):
                    self.mode("wrong-uid")
                request.assert_not_called()
                self.assertEqual(self.m.load()["cases"], {})

    def test_fresh_identity_existing_state_and_source_checksum(self):
        self.mode("baseline")
        self.snapshot.reset_mock()
        with self.assertRaises(RuntimeError):
            self.mode("baseline")
        self.snapshot.assert_not_called()
        with mock.patch.object(self.m, "harness_digest", return_value="changed"), self.assertRaises(RuntimeError):
            self.m.load()
        with self.assertRaises(RuntimeError):
            self.mode("stale-rv")
        with self.assertRaises(RuntimeError):
            self.mode("readback")

    def test_nonblocking_lock_prevents_competing_runner(self):
        lock = self.m.DIRECTORY / "guard-state.lock"
        fd = os.open(lock, os.O_CREAT | os.O_RDWR, 0o600)
        with os.fdopen(fd, "rb") as stream:
            fcntl.flock(stream.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            with self.assertRaises(BlockingIOError):
                self.mode("baseline", locked=False)
        self.snapshot.assert_not_called()

    def test_transport_pins_host_content_type_no_proxy_redirect_retry(self):
        patch = self.m.make_patch("wrong-uid", self.public)
        opener = mock.Mock()
        opener.open.side_effect = urllib.error.HTTPError(self.m.API + self.m.PATH, 302, "redirect", {}, io.BytesIO(b"blocked"))
        with mock.patch.object(self.m.urllib.request, "build_opener", return_value=opener) as build:
            self.assertEqual(self.m.request("PATCH", patch, "offline-token"), (302, b"blocked"))
        self.assertEqual(opener.open.call_count, 1)
        req = opener.open.call_args.args[0]
        self.assertEqual(req.full_url, self.m.API + self.m.PATH)
        self.assertEqual(req.get_method(), "PATCH")
        self.assertEqual(req.get_header("Content-type"), "application/json-patch+json")
        self.assertEqual(json.loads(req.data), patch)
        self.assertTrue(any(isinstance(h, self.m.NoRedirect) for h in build.call_args.args))
        self.assertTrue(any(isinstance(h, self.m.urllib.request.ProxyHandler) and h.proxies == {} for h in build.call_args.args))
        self.assertIsNone(self.m.NoRedirect().redirect_request(None, None, 302, "redirect", {}, "https://unapproved.invalid"))

    def test_protected_files_are_read_only_and_sidecars_block(self):
        protected = {}
        for name in ("primary", "retired"):
            path = self.m.DIRECTORY / name
            fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
            with os.fdopen(fd, "wb") as stream:
                stream.write(name.encode())
            protected[path] = hashlib.sha256(name.encode()).hexdigest()
        primary = next(iter(protected))
        with mock.patch.object(self.m, "PROTECTED", protected), mock.patch.object(self.m, "PRIMARY", primary):
            self.m.protected_unchanged()
            sidecar = primary.with_name(primary.name + ".updates.json")
            sidecar.touch(mode=0o600)
            with self.assertRaises(RuntimeError):
                self.m.protected_unchanged()
        self.assertEqual(primary.read_bytes(), b"primary")

    def fixtures(self):
        m = self.m
        ready = lambda kind: {"conditions": [{"type": kind, "status": "True"}]}
        deployment = {"metadata": {"name": "platform-api-server", "uid": "api-deployment"}, "spec": {"replicas": 1, "template": {"spec": {"containers": [{"name": "platform-api-server", "image": m.IMAGE, "args": ["--enable-iam-application-writes=false"], "env": [{"name": "SPANNER_DATABASE", "value": "projects/dev-reg-na-ne2-apahim1110/instances/dev-reg-na-ne2-apahim1110-spanner/databases/gecko"}]}]}}}, "status": {"readyReplicas": 1}}
        deployment["spec"]["selector"] = {"matchLabels": {"app.kubernetes.io/name": "platform-api-server", "app.kubernetes.io/instance": "platform-api-server"}}
        deployments = [deployment] + [{"metadata": {"name": "controller-" + str(i), "uid": str(i)}, "spec": {"replicas": 1}, "status": {"readyReplicas": 1}} for i in range(7)]
        objs = {
            "template": {"metadata": {"uid": m.EXPECTED_REF["uid"]}, "spec": {"verified": False}},
            "deployments": {"items": deployments},
            "pods": {"items": [{"metadata": {"name": "platform-api-server-offline", "uid": "api-pod"}, "status": {"containerStatuses": [{"name": "platform-api-server", "ready": True, "imageID": m.IMAGE_ID}]}}]},
            "application": {"spec": {"source": {"targetRevision": m.SOURCE}}, "status": {"sync": {"revision": m.SOURCE, "status": "Synced"}, "health": {"status": "Healthy"}, "resources": [{"kind": "Job", "name": "platform-api-server-migrate", "status": "Synced"}]}},
            "job": {"metadata": {"uid": "daa14792-1593-4a92-abe8-2b8f32ba3fd1", "creationTimestamp": "2026-11-17T19:25:31Z"}, "spec": {"template": {"spec": {"containers": [{"image": "quay.io/apahim/gecko:wif-p2-cel-54709c4-api"}]}}}, "status": ready("Complete")},
            "spannerdatabase": {"metadata": {"uid": "99e4163c-79f9-4875-92b9-0e576b9a5927"}, "spec": {}, "status": ready("Ready")},
        }
        for name, uid, generation, condition in (("cluster", m.UID, 2, "HostedClusterAvailable"), ("pool", m.NODEPOOL_UID, 1, "NodePoolHealthy")):
            objs[name] = {"metadata": {"uid": uid, "generation": generation, "resourceVersion": "501"}, "spec": {"release": {"version": "5.0.2", "channelGroup": "candidate"}, "nodeCount": 1}, "status": {**ready(condition), "iamRequirements": {"state": "Complete"}, "iamDispatch": {"dispatched": {"objectUID": uid, "clusterUID": m.UID, "releaseVersion": "5.0.2", "releaseImage": m.RELEASE_IMAGE, "channelGroup": "candidate", "templateRef": m.EXPECTED_REF}}}}
        objs["pods"]["items"][0]["metadata"].update({"labels": copy.deepcopy(deployment["spec"]["selector"]["matchLabels"]), "ownerReferences": [{"kind": "ReplicaSet", "controller": True, "uid": "api-rs"}]})
        return objs

    def real_snapshot(self, objs):
        def kubectl(*args):
            key = args[0]
            if key == "--raw":
                key = "template" if "wifconfigtemplates" in args[1] else "pool" if "nodepools" in args[1] else "cluster"
            return copy.deepcopy(objs[key])
        with mock.patch.object(self.m, "kubectl", kubectl), mock.patch.object(self.m, "protected_unchanged"):
            return self.m.private_snapshot()

    def test_live_prerequisites_fail_closed_in_offline_fixtures(self):
        objs = self.fixtures()
        baseline = self.real_snapshot(objs)
        # A normal status revision/requirements refresh must not masquerade as spec drift.
        objs["cluster"]["metadata"]["resourceVersion"] = "502"
        objs["cluster"]["status"]["iamRequirements"]["lastRefreshTime"] = "later"
        self.assertEqual(baseline, self.real_snapshot(objs))
        for target, field, value in (("template", "spec", {"verified": True}), ("job", "metadata", {"uid": "replacement"}), ("spannerdatabase", "metadata", {"uid": "replacement"}), ("pool", "spec", {"nodeCount": 2}), ("application", "status", {"sync": {"status": "OutOfSync"}})):
            with self.subTest(target=target):
                changed = copy.deepcopy(objs)
                changed[target][field] = value
                with self.assertRaises((RuntimeError, KeyError)):
                    self.real_snapshot(changed)

    def test_pod_selector_excludes_job_but_rejects_second_api_pod(self):
        objs = self.fixtures()
        baseline = self.real_snapshot(objs)
        migration = {"metadata": {"name": "platform-api-server-migrate-offline", "uid": "migration-pod", "labels": copy.deepcopy(objs["pods"]["items"][0]["metadata"]["labels"]), "ownerReferences": [{"kind": "Job", "controller": True, "uid": "migration-job"}]}, "status": {"phase": "Succeeded"}}
        objs["pods"]["items"].append(migration)
        self.assertEqual(baseline, self.real_snapshot(objs))
        for terminating in (False, True):
            changed = copy.deepcopy(objs)
            extra = copy.deepcopy(changed["pods"]["items"][0])
            extra["metadata"].update({"name": "platform-api-server-second", "uid": "second-api"})
            if terminating:
                extra["metadata"]["deletionTimestamp"] = "now"
            changed["pods"]["items"].append(extra)
            with self.assertRaises(RuntimeError):
                self.real_snapshot(changed)


if __name__ == "__main__":
    unittest.main()
