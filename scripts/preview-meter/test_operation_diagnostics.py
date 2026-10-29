"""Offline exact-host, private operation-name and unconditional-hold regressions."""

import importlib.util
import json
import os
import re
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import operation_diagnostics as op
import policy
from private_diagnostics import OPERATION_FILENAME

HOST = "747336059622.s3-control.us-east-2.amazonaws.com"
CANARY = "private-resource-query-authorization-body-canary"


class Tests(unittest.TestCase):
    def test_exact_account_and_region_only(self):
        self.assertEqual(policy.service(HOST), "s3control")
        self.assertEqual(policy.service(HOST.upper()), "s3control")
        for wrong in [
            "000000000000.s3-control.us-east-2.amazonaws.com",
            "747336059622.s3-control.us-east-1.amazonaws.com",
            "s3-control.us-east-2.amazonaws.com",
            "747336059622.s3-control.us-east-2.api.aws",
            "747336059622.s3-control-fips.us-east-2.amazonaws.com",
            HOST + ".",
            HOST + ".invalid",
            "prefix." + HOST,
            HOST + ":443",
        ]:
            with self.assertRaisesRegex(policy.Stop, "endpoint-held"):
                policy.service(wrong)

    def test_all_operations_hold_before_headers_or_body_are_needed(self):
        self.assertNotIn(
            "s3control", policy.REST | set(policy.JSON_READS) | set(policy.QUERY_READS)
        )
        for method in [
            "GET",
            "HEAD",
            "POST",
            "PUT",
            "PATCH",
            "DELETE",
            "OPTIONS",
            "CONNECT",
            CANARY,
        ]:
            with self.assertRaisesRegex(policy.Stop, "^s3control-operation-held$"):
                policy.classify(method, HOST, object(), object(), object())

    def test_model_read_and_mutation_routes_identify_but_never_authorize(self):
        for method, target, name in [
            (
                "GET",
                "/v20180820/configuration/publicAccessBlock",
                "GetPublicAccessBlock",
            ),
            (
                "DELETE",
                "/v20180820/configuration/publicAccessBlock",
                "DeletePublicAccessBlock",
            ),
            (
                "GET",
                "/v20180820/tags/arn%3Aaws%3As3%3A%3A%3A"
                + CANARY
                + "?secret="
                + CANARY,
                "ListTagsForResource",
            ),
        ]:
            value = op.identify(method, target)
            self.assertEqual(value["matchStatus"], "unique")
            self.assertEqual(value["candidateOperations"], [name])
            self.assertNotIn(CANARY, json.dumps(value))
            with self.assertRaises(policy.Stop):
                policy.classify(method, HOST, target, {}, b"")

    def test_all_pinned_model_routes_have_useful_synthetic_matches(self):
        from botocore.loaders import Loader

        model = Loader().load_service_model("s3control", "service-2")
        catalogue = json.loads(
            Path(op.__file__).with_name("s3control-operation-routes.json").read_text()
        )
        self.assertEqual(catalogue["sourceModelSha256"], op.MODEL_SHA)
        self.assertEqual(
            catalogue["operations"],
            [
                {"name": name, **data["http"]}
                for name, data in model["operations"].items()
            ],
        )
        for name, data in model["operations"].items():
            target = re.sub(r"\{[^{}]+\}", CANARY, data["http"]["requestUri"])
            result = op.identify(data["http"]["method"], target)
            self.assertIn(name, result["candidateOperations"], name)
            self.assertNotIn(CANARY, json.dumps(result))

    def test_receipt_is_private_bounded_exclusive_and_contains_only_names(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = "/v20180820/tags/" + CANARY + "?X-Amz-Credential=" + CANARY
            op.capture_operation(tmp, "GET", target, 231)
            path = Path(tmp) / OPERATION_FILENAME
            original = path.read_bytes()
            value = json.loads(original)
            self.assertEqual(value["candidateOperations"], ["ListTagsForResource"])
            self.assertEqual(value["modelSha256"], op.MODEL_SHA)
            self.assertEqual(
                set(value),
                {
                    "version",
                    "phase",
                    "count",
                    "modelSha256",
                    "method",
                    "candidateOperations",
                    "matchStatus",
                },
            )
            for forbidden in [
                CANARY,
                "X-Amz-Credential",
                "/v20180820",
                "Authorization",
                "Bearer",
            ]:
                self.assertNotIn(forbidden, original.decode())
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertLessEqual(len(original), 1024)
            with self.assertRaises(FileExistsError):
                op.capture_operation(tmp, "DELETE", target, 231)
            self.assertEqual(path.read_bytes(), original)

    def test_unknown_invalid_overlong_or_ambiguous_metadata_stays_bounded(self):
        for target in [
            None,
            {},
            "",
            "https://user:secret@invalid/path",
            "//[invalid",
            "/" + "a" * 8192,
            "/" + CANARY + "\r\nsecret",
            "/é",
            "/unknown?secret=" + CANARY,
        ]:
            value = op.identify("GET", target)
            self.assertFalse(value["candidateOperations"])
            self.assertNotIn(CANARY, json.dumps(value))
        value = op.identify(CANARY, "/")
        self.assertEqual(value["method"], "OTHER")
        many = [(f"Operation{i}", "GET", re.compile("/"), ()) for i in range(5)]
        with patch.object(op, "rules", return_value=many):
            value = op.identify("GET", "/")
            self.assertEqual(value["matchStatus"], "ambiguous-overflow")
            self.assertEqual(value["candidateOperations"], [])

    def test_model_pin_failure_prevents_receipt_and_does_not_fall_back(self):
        op.rules.cache_clear()
        self.addCleanup(op.rules.cache_clear)
        with tempfile.TemporaryDirectory() as tmp:
            with (
                patch.object(Path, "read_bytes", return_value=b"wrong-model"),
                self.assertRaisesRegex(OSError, "operation-model-pin-held"),
            ):
                op.capture_operation(
                    tmp, "GET", "/v20180820/configuration/publicAccessBlock", 231
                )
            self.assertFalse((Path(tmp) / OPERATION_FILENAME).exists())

    def test_gate_holds_before_touching_authorization_or_body_and_never_admits(self):
        class Request:
            host, port, scheme = HOST, 443, "https"
            http_version, method = "HTTP/1.1", "GET"
            path = "/v20180820/tags/" + CANARY + "?secret=" + CANARY

            @property
            def headers(self):
                raise AssertionError("headers-must-not-be-read")

            @property
            def raw_content(self):
                raise AssertionError("body-must-not-be-read")

        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            config = directory / "config.json"
            config.write_text(
                json.dumps(
                    {
                        "mode": "fixture",
                        "fixtureOrigin": 1,
                        "ledger": str(directory / "attempts.jsonl"),
                        "budget": {"prior": 231, "maximum": 500, "seconds": 3600},
                        "privateS3ControlOperationCapture": True,
                    }
                )
            )
            http = SimpleNamespace(
                Response=SimpleNamespace(make=Mock(return_value=object()))
            )
            spec = importlib.util.spec_from_file_location(
                "operation_proxy_fixture", Path(__file__).with_name("proxy.py")
            )
            module = importlib.util.module_from_spec(spec)
            with (
                patch.dict(sys.modules, {"mitmproxy": SimpleNamespace(http=http)}),
                patch.dict(os.environ, {"KURIER_PREVIEW_METER_CONFIG": str(config)}),
            ):
                spec.loader.exec_module(module)
            gate = module.addons[0]
            self.addCleanup(gate.done)
            flow = SimpleNamespace(request=Request(), kill=Mock())
            with patch.object(gate.budget, "admit", wraps=gate.budget.admit) as admit:
                gate.request(flow)
                gate.request(flow)
                admit.assert_not_called()
            self.assertTrue(gate.budget.stopped)
            self.assertEqual(gate.budget.count, 231)
            raw = (directory / "attempts.jsonl").read_text()
            self.assertNotIn(CANARY, raw)
            self.assertNotIn("ListTagsForResource", raw)
            receipt = json.loads((directory / OPERATION_FILENAME).read_text())
            self.assertEqual(receipt["candidateOperations"], ["ListTagsForResource"])


if __name__ == "__main__":
    unittest.main()
