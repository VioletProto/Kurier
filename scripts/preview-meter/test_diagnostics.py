"""Local endpoint disclosure and frozen-accounting regressions."""

import hashlib
import json
import shutil
import tempfile
import unittest
from pathlib import Path

from accounting import FROZEN_FILES, PRIOR, baseline
from diagnostics import endpoint_metadata
from policy import Stop, service
from run import ROOT

CANARY = "local-private-canary"


class DiagnosticTests(unittest.TestCase):
    def test_useful_names_do_not_authorize_endpoints(self):
        for host in [
            "sts.us-east-1.amazonaws.com",
            "lambda.amazonaws.com",
            "registry.npmjs.org",
            "cloudfront-keyvaluestore.global.api.aws",
        ]:
            with self.subTest(host=host):
                with self.assertRaises(Stop):
                    service(host)
                value = endpoint_metadata(
                    host.upper(), 443, "https", "HTTP/1.1", "CONNECT", "connect"
                )
                self.assertEqual(value["hostname"], host)
                self.assertEqual(value["phase"], "connect")
                self.assertEqual(value["port"], 443)
                self.assertLessEqual(len(json.dumps(value)), 512)
        # Authorization policy bytes, not just selected outcomes, are unchanged.
        self.assertEqual(
            hashlib.sha256(
                (ROOT / "scripts/preview-meter/policy.py").read_bytes()
            ).hexdigest(),
            "ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560",
        )

    def test_secret_labels_url_material_and_malformed_hosts_are_redacted(self):
        hosts = [
            None,
            {},
            123,
            "",
            CANARY + ".example.invalid",
            CANARY + ".s3.amazonaws.com",
            CANARY + "@sts.us-east-1.amazonaws.com",
            "https://user:"
            + CANARY
            + "@sts.us-east-1.amazonaws.com/path?token="
            + CANARY,
            "sts.us-east-1.amazonaws.com/path",
            "sts.us-east-1.amazonaws.com?token=" + CANARY,
            "sts.us-east-1.amazonaws.com\r\nAuthorization: " + CANARY,
            "é.amazonaws.com",
            "a" * 10000,
            "[2001:db8::abcd]",
        ]
        for host in hosts:
            with self.subTest(kind=type(host).__name__):
                result = endpoint_metadata(
                    host, CANARY, [CANARY], {CANARY: 1}, "GET " + CANARY, None
                )
                self.assertEqual(result["hostname"], "<redacted-host>")
                self.assertNotIn(CANARY, json.dumps(result))
                self.assertIsNone(result["port"])
                self.assertLessEqual(len(json.dumps(result)), 512)
                self.assertEqual(
                    set(result),
                    {
                        "hostname",
                        "hostnameStatus",
                        "trailingDot",
                        "port",
                        "scheme",
                        "httpVersion",
                        "method",
                        "phase",
                    },
                )

    def test_protocol_ports_and_normalization_are_bounded(self):
        v = endpoint_metadata(
            "STS.US-EAST-1.AMAZONAWS.COM.", 8443, "http", "HTTP/2.0", "POST", "request"
        )
        self.assertEqual(v["hostname"], "sts.us-east-1.amazonaws.com")
        self.assertTrue(v["trailingDot"])
        self.assertEqual(
            (v["port"], v["scheme"], v["httpVersion"]), (8443, "http", "HTTP/2.0")
        )
        for port in [True, False, -1, 0, 65536, {}, "443"]:
            self.assertIsNone(
                endpoint_metadata(
                    "sts.us-east-1.amazonaws.com",
                    port,
                    "https",
                    "HTTP/1.1",
                    "GET",
                    "request",
                )["port"]
            )

    def test_accounting_preserves_both_original_ledgers(self):
        self.assertEqual(PRIOR, 167)
        b = baseline(ROOT)
        self.assertEqual((b["prior"], b["maximum"]), (167, 500))
        with tempfile.TemporaryDirectory() as t:
            root = Path(t)
            dest = root / "docs/development"
            dest.mkdir(parents=True)
            for name in FROZEN_FILES:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / name, target)
            self.assertEqual(baseline(root), b)
            with (dest / "project-environments-preview-attempts.jsonl").open("a") as f:
                f.write("\n")
            with self.assertRaisesRegex(ValueError, "preview-ledger-hash"):
                baseline(root)
        self.assertEqual(baseline(ROOT), b)


if __name__ == "__main__":
    unittest.main()
