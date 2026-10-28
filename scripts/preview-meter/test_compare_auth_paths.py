"""Offline operation-endpoint comparison and metadata disclosure regressions."""

import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import compare_auth_paths as comparison


class Tests(unittest.TestCase):
    def test_operation_endpoint_difference_is_captured_before_any_send(self):
        result = subprocess.run(
            [sys.executable, "-B", comparison.__file__],
            capture_output=True,
            text=True,
            check=False,
            timeout=30,
        )
        self.assertEqual(
            result.returncode,
            0,
            "Offline comparison must complete without raw child diagnostics",
        )
        data = json.loads(result.stdout)
        modes = {x["mode"]: x for x in data["paths"]}
        self.assertEqual(
            modes["sdk-default"]["operationHostname"], "us-east-2.signin.aws.amazon.com"
        )
        self.assertEqual(
            modes["cli-default"]["operationHostname"], "us-east-2.signin.aws.amazon.com"
        )
        self.assertEqual(
            modes["custom"]["operationHostname"], "signin.us-east-2.amazonaws.com"
        )
        self.assertEqual(
            modes["sdk-default"]["clientBaseHostname"],
            modes["custom"]["clientBaseHostname"],
        )
        for item in modes.values():
            self.assertEqual(item["region"], "us-east-2")
            self.assertEqual(item["networkRequests"], 0)
            self.assertFalse(item["credentialProviderInvoked"])
            self.assertFalse(item["cacheWritten"])
            self.assertTrue(item["beforeCallCaptured"])
            self.assertFalse(item["effectiveProxyConfigured"])
            self.assertTrue(item["tlsVerifyRequested"])
            self.assertTrue(item["tlsCertRequired"])
            self.assertFalse(item["clientCertificateConfigured"])
            self.assertFalse(item["urllib3HostnameAssertionDisabled"])
            self.assertEqual(
                item["expectedTlsServerHostname"], item["operationHostname"]
            )
            self.assertEqual(
                item["caAnchorsSha256"], modes["custom"]["caAnchorsSha256"]
            )
            self.assertEqual(item["tlsContext"], modes["custom"]["tlsContext"])
        for forbidden in [
            "offline-fixture",
            "offline-only",
            "refreshToken",
            "secretAccessKey",
            "dpopKey",
            "https://",
            "arn:aws:iam",
        ]:
            self.assertNotIn(forbidden, result.stdout)

    def test_metadata_never_discloses_credential_or_session_values(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp)
            cache = home / ".aws/login/cache"
            cache.mkdir(parents=True)
            session = "arn:aws:iam::000000000000:user/offline-private-canary"
            (home / ".aws/config").write_text(
                f"[default]\nregion=us-east-2\nlogin_session={session}\n"
            )
            path = cache / (hashlib.sha256(session.encode()).hexdigest() + ".json")
            path.write_text(
                json.dumps(
                    {
                        "accessToken": {"secretAccessKey": "private-canary"},
                        "clientId": "private-canary",
                        "refreshToken": "private-canary",
                        "dpopKey": "private-canary",
                        "region": "private-canary",
                    }
                )
            )
            original = path.read_bytes()
            with patch.object(Path, "home", return_value=home):
                result = comparison.metadata()
            self.assertNotIn("private-canary", json.dumps(result))
            self.assertNotIn(session, json.dumps(result))
            self.assertEqual(result["cacheRegionMetadata"], [None])
            self.assertTrue(result["cacheUnchangedDuringMetadataRead"])
            self.assertEqual(path.read_bytes(), original)

    def test_unexpected_endpoint_material_is_not_published(self):
        for value in [
            "http://signin.us-east-2.amazonaws.com",
            "https://other.invalid",
            "https://private@signin.us-east-2.amazonaws.com",
            "https://signin.us-east-2.amazonaws.com?private=canary",
            "https://signin.us-east-2.amazonaws.com:444",
        ]:
            with self.assertRaises(comparison.OfflineViolation):
                comparison.hostname(value)


if __name__ == "__main__":
    unittest.main()
