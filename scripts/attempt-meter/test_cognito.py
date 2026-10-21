import tempfile
import unittest
from pathlib import Path
from budget import Budget, MeterStop
from cognito import HOST, ORIGIN, held_diagnostics


class CognitoTests(unittest.TestCase):
    def test_only_exact_browser_preflight_is_read_and_each_forward_is_charged(self):
        headers = {
            "origin": ORIGIN,
            "access-control-request-method": "POST",
            "access-control-request-headers": "cache-control,content-type,x-amz-target,x-amz-user-agent",
        }
        with tempfile.TemporaryDirectory() as d:
            b = Budget(Path(d) / "events")
            try:
                self.assertEqual(
                    b.classify("OPTIONS", HOST, "/", headers, b""),
                    ("aws", "CognitoAuthPreflight", False, False),
                )
                for _ in range(4):
                    b.admit("aws", "CognitoAuthPreflight", False)
                self.assertEqual(b.counts["aws"], 4)
                with self.assertRaises(MeterStop):
                    b.admit("aws", "InitiateAuth", True)
            finally:
                b.close()
        cases = [
            ("other.amazonaws.com", "/", headers, b""),
            (HOST, "/?signed=unknown", headers, b""),
            (HOST, "/other", headers, b""),
            (HOST, "/", {**headers, "origin": "https://other.example"}, b""),
            (HOST, "/", {**headers, "access-control-request-method": "DELETE"}, b""),
            (
                HOST,
                "/",
                {
                    **headers,
                    "access-control-request-headers": "content-type,x-amz-target,authorization",
                },
                b"",
            ),
            (
                HOST,
                "/",
                {**headers, "access-control-request-headers": "content-type"},
                b"",
            ),
            (HOST, "/", {**headers, "authorization": "private-canary"}, b""),
            (HOST, "/", headers, b"body"),
        ]
        for i, (host, path, h, body) in enumerate(cases):
            with tempfile.TemporaryDirectory() as d:
                b = Budget(Path(d) / str(i))
                try:
                    if host == HOST:
                        with self.assertRaises(MeterStop):
                            b.classify("OPTIONS", host, path, h, body)
                    else:
                        self.assertTrue(b.classify("OPTIONS", host, path, h, body)[2])
                finally:
                    b.close()

    def test_safe_diagnostics_have_no_arbitrary_operation_or_values(self):
        self.assertEqual(
            held_diagnostics(
                "POST", "dynamodb.us-east-2.amazonaws.com", "aws", "privateCanary"
            ),
            {"method": "POST", "category": "aws", "operation": "unknown"},
        )
        self.assertEqual(
            held_diagnostics("OPTIONS", HOST),
            {
                "method": "OPTIONS",
                "category": "aws",
                "operation": "CognitoAuthPreflight",
            },
        )

    def test_verification_budget_is_exactly_one_hundred_and_ten_minutes(self):
        now = [1000]
        with tempfile.TemporaryDirectory() as d:
            b = Budget(
                Path(d) / "calls",
                clock=lambda: now[0],
                aws_limit=100,
                api_limit=0,
                poll_limit=0,
                seconds=600,
            )
            try:
                for _ in range(100):
                    b.admit("aws", "Query", False)
                with self.assertRaises(MeterStop):
                    b.admit("aws", "Query", False)
                self.assertEqual(b.counts["aws"], 100)
            finally:
                b.close()
            b = Budget(Path(d) / "time", clock=lambda: now[0], seconds=600)
            try:
                b.admit("aws", "Query", False)
                now[0] += 600
                with self.assertRaises(MeterStop):
                    b.admit("aws", "Query", False)
            finally:
                b.close()
