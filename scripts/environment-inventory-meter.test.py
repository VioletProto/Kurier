"""Local transport regressions; no AWS endpoints or credentials are used."""

import importlib.util
import json
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

import boto3
from botocore.client import BaseClient
from botocore.config import Config
from botocore.httpsession import URLLib3Session

spec = importlib.util.spec_from_file_location(
    "inventory_meter", Path(__file__).with_name("environment-inventory-meter.py")
)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class MeterTests(unittest.TestCase):
    def test_resume_preserves_count_and_stopped_window_cannot_resume(self):
        with tempfile.TemporaryDirectory() as tmp:
            ledger = Path(tmp) / "ledger"
            meter = module.Meter(ledger, limit=2)
            meter.admit("sts", "GetCallerIdentity")
            meter.finish()
            resumed = module.Meter(ledger, limit=2, resume=True)
            self.assertEqual(resumed.count, 1)
            resumed.admit("sts", "GetCallerIdentity")
            with self.assertRaisesRegex(RuntimeError, "inventory-limit"):
                resumed.admit("sts", "GetCallerIdentity")
            resumed.finish()
            with self.assertRaisesRegex(RuntimeError, "resume-held"):
                module.Meter(ledger, limit=2, resume=True)

    def test_cumulative_limit_and_deadline_latch(self):
        with tempfile.TemporaryDirectory() as tmp:
            now = [0]
            meter = module.Meter(
                Path(tmp) / "ledger", limit=2, seconds=10, clock=lambda: now[0]
            )
            meter.admit("sts", "GetCallerIdentity")
            now[0] = 10
            with self.assertRaisesRegex(RuntimeError, "inventory-limit"):
                meter.admit("sts", "GetCallerIdentity")
            with self.assertRaisesRegex(RuntimeError, "latched"):
                meter.admit("sts", "GetCallerIdentity")
            meter.finish()
            self.assertEqual(
                module.summary(Path(tmp) / "ledger")["physicalAttempts"], 1
            )

    def test_actual_sdk_retry_and_held_write(self):
        calls = []

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                self.rfile.read(int(self.headers["Content-Length"]))
                calls.append(1)
                self.send_response(503)
                self.end_headers()
                self.wfile.write(
                    b"<ErrorResponse><Error><Code>ServiceUnavailable</Code></Error></ErrorResponse>"
                )

        server = HTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with tempfile.TemporaryDirectory() as tmp:
                meter = module.Meter(Path(tmp) / "ledger", limit=2)
                original_send = URLLib3Session.send
                original = meter.install()
                try:
                    client = boto3.client(
                        "sts",
                        region_name="us-east-2",
                        aws_access_key_id="fixture",
                        aws_secret_access_key="fixture",
                        endpoint_url=f"http://127.0.0.1:{server.server_port}",
                        config=Config(
                            retries={"total_max_attempts": 3, "mode": "standard"}
                        ),
                    )
                    with self.assertRaisesRegex(RuntimeError, "inventory-limit"):
                        client.get_caller_identity()
                    self.assertEqual(len(calls), 2)
                    self.assertEqual(meter.count, 2)
                    with self.assertRaisesRegex(RuntimeError, "operation-held"):
                        client.assume_role(
                            RoleArn="arn:aws:iam::123456789012:role/fixture",
                            RoleSessionName="fixture",
                        )
                    self.assertEqual(len(calls), 2)
                finally:
                    BaseClient._make_api_call = original
                    URLLib3Session.send = original_send
                    meter.finish()
                raw = (Path(tmp) / "ledger").read_text()
                self.assertNotIn("fixture", raw)
                rows = [json.loads(x) for x in raw.splitlines()]
                self.assertEqual(len([r for r in rows if r["event"] == "attempt"]), 2)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == "__main__":
    unittest.main()
