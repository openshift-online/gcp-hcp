"""Offline harness checks: no real authentication, HTTP or Kubernetes calls."""

import contextlib
import copy
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import urllib.error


class GuardRehearsalTest(unittest.TestCase):
    def setUp(self):
        path = Path(__file__).with_name("api-guard-rehearsal.py")
        spec = importlib.util.spec_from_file_location("guard_rehearsal", path)
        self.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.module)
        self.temp = tempfile.TemporaryDirectory(prefix="wif-api-guards-test-")
        self.addCleanup(self.temp.cleanup)
        self.module.DIRECTORY = Path(self.temp.name).resolve()
        self.module.STATE = self.module.DIRECTORY / "guard-state.json"
        self.snapshot = {
            "uid": self.module.UID,
            "generation": 2,
            "resourceVersion": "100",
            "specDigest": "sha256:unchanged-test-spec",
            "applications": [self.module.EXPECTED_REF],
        }
        self.private = {"templateVerified": False, "iamApplicationWrites": False, "receipt": {"dispatched": "test"}, "upgrade": None}
        for name, replacement in (
            ("read_public", lambda: copy.deepcopy(self.snapshot)),
            ("gates_and_private", lambda: copy.deepcopy(self.private)),
            ("identity_token", lambda: "offline-token"),
            ("command", mock.Mock(side_effect=AssertionError("tests must not access authentication or cloud"))),
        ):
            patch = mock.patch.object(self.module, name, replacement)
            patch.start()
            self.addCleanup(patch.stop)

    def run_mode(self, mode):
        with contextlib.redirect_stdout(io.StringIO()):
            self.module.run_locked(mode)

    def test_rejections_keep_exact_history_and_intent_before_patch(self):
        self.run_mode("baseline")
        calls = []

        def request(method, patch, token=None):
            self.assertEqual(method, "PATCH")
            self.assertEqual(token, "offline-token")
            mode = "wrong-uid" if len(patch) == 2 else "stale-rv"
            state = json.loads(self.module.STATE.read_text())
            self.assertEqual(state["cases"][mode]["phase"], "Intent")
            self.assertEqual(patch[-1], {"op": "replace", "path": "/spec/platform/gcp/iam/templateApplications", "value": [self.module.EXPECTED_REF]})
            self.assertEqual(patch[0]["path"], "/metadata/uid")
            if mode == "wrong-uid":
                self.assertNotEqual(patch[0]["value"], self.module.UID)
                guard = "/metadata/uid"
            else:
                self.assertEqual(patch[0]["value"], self.module.UID)
                self.assertEqual(patch[1], {"op": "test", "path": "/metadata/resourceVersion", "value": "0"})
                guard = "/metadata/resourceVersion"
            calls.append(mode)
            return 400, json.dumps({"error": "testing value " + guard + " failed: test failed"}).encode()

        with mock.patch.object(self.module, "request", request):
            self.run_mode("wrong-uid")
            self.run_mode("stale-rv")
            self.run_mode("readback")
            with self.assertRaises(RuntimeError):
                self.run_mode("stale-rv")
        self.assertEqual(calls, ["wrong-uid", "stale-rv"])
        state = json.loads(self.module.STATE.read_text())
        self.assertEqual(state["phase"], "Validated")
        self.assertEqual(state["baseline"]["specDigest"], state["after"]["specDigest"])

    def test_auth_rejection_is_not_guard_success_or_retried(self):
        self.run_mode("baseline")
        request = mock.Mock(return_value=(403, b'{"error":"forbidden"}'))
        with mock.patch.object(self.module, "request", request):
            with self.assertRaises(RuntimeError):
                self.run_mode("wrong-uid")
            with self.assertRaises(RuntimeError):
                self.run_mode("wrong-uid")
        self.assertEqual(request.call_count, 1)
        state = json.loads(self.module.STATE.read_text())
        self.assertFalse(state["cases"]["wrong-uid"]["rejectedByTest"])
        self.assertEqual(state["cases"]["wrong-uid"]["phase"], "UnexpectedResponse")

    def test_auth_acquisition_failure_does_not_record_patch_intent(self):
        self.run_mode("baseline")
        with mock.patch.object(self.module, "identity_token", side_effect=RuntimeError("auth unavailable")):
            with self.assertRaises(RuntimeError):
                self.run_mode("wrong-uid")
        self.assertEqual(self.module.load()["cases"], {})

    def test_generic_request_echo_and_missing_path_are_not_guard_success(self):
        for body in (
            b'{"error":"failed validation","request":[{"op":"test","path":"/metadata/uid"}]}',
            b'{"error":"testing value /metadata/uid failed: missing value"}',
        ):
            with self.subTest(body=body):
                self.module.STATE.unlink(missing_ok=True)
                self.run_mode("baseline")
                with mock.patch.object(self.module, "request", return_value=(400, body)):
                    with self.assertRaises(RuntimeError):
                        self.run_mode("wrong-uid")
                self.assertFalse(self.module.load()["cases"]["wrong-uid"]["rejectedByTest"])

    def test_existing_crash_intent_cannot_repeat_patch(self):
        self.run_mode("baseline")
        state = self.module.load()
        state["cases"]["wrong-uid"] = {"phase": "Intent"}
        self.module.save(state)
        request = mock.Mock(side_effect=AssertionError("PATCH must not be repeated"))
        with mock.patch.object(self.module, "request", request):
            with self.assertRaises(RuntimeError):
                self.run_mode("wrong-uid")
        request.assert_not_called()

    def test_transport_pins_host_body_no_proxy_redirect_or_retry(self):
        body = [{"op": "test", "path": "/metadata/uid", "value": "wrong"}]
        opener = mock.Mock()
        opener.open.side_effect = urllib.error.HTTPError(self.module.API + self.module.PATH, 302, "redirect", {}, io.BytesIO(b"redirect blocked"))
        with mock.patch.object(self.module.urllib.request, "build_opener", return_value=opener) as build:
            code, response = self.module.request("PATCH", body, token="offline-token")
        self.assertEqual((code, response), (302, b"redirect blocked"))
        self.assertEqual(opener.open.call_count, 1)
        request = opener.open.call_args.args[0]
        self.assertEqual(request.full_url, self.module.API + self.module.PATH)
        self.assertEqual(request.get_method(), "PATCH")
        self.assertEqual(request.get_header("Content-type"), "application/json-patch+json")
        self.assertEqual(request.get_header("Authorization"), "Bearer offline-token")
        self.assertEqual(json.loads(request.data), body)
        handlers = build.call_args.args
        self.assertTrue(any(isinstance(h, self.module.NoRedirect) for h in handlers))
        self.assertTrue(any(isinstance(h, self.module.urllib.request.ProxyHandler) and h.proxies == {} for h in handlers))
        self.assertIsNone(self.module.NoRedirect().redirect_request(None, None, 302, "redirect", {}, "https://unapproved.invalid"))


if __name__ == "__main__":
    unittest.main()
