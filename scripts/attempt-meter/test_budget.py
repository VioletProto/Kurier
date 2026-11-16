import json
import os
import tempfile
import unittest
from budget import Budget, MeterStop


class BudgetTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.now = [1000.0]
        self.path = os.path.join(self.directory.name, "attempts.jsonl")
        self.b = Budget(self.path, clock=lambda: self.now[0], api_limit=3, aws_limit=3)

    def tearDown(self):
        self.b.close()
        self.directory.cleanup()

    def test_each_retry_and_page_charged_before_outcome(self):
        for status in [503, 200, 200]:
            n = self.b.admit("aws", "Scan", False)
            with open(self.path, encoding="utf8") as file:
                self.assertEqual(json.loads(file.readlines()[-1])["event"], "attempt")
            self.b.outcome(n, status)
        self.assertEqual(self.b.counts["aws"], 3)
        with self.assertRaises(MeterStop):
            self.b.admit("aws", "Scan", False)

    def test_read_only_default_and_latched_stop(self):
        with self.assertRaises(MeterStop):
            self.b.admit("aws", "TransactWriteItems", True)
        with self.assertRaises(MeterStop):
            self.b.admit("aws", "Scan", False)
        self.assertEqual(self.b.counts["aws"], 0)

    def test_uncertain_write_cannot_retry(self):
        n = self.b.admit("api", "POST", True, write_allowed=True)
        with self.assertRaises(MeterStop):
            self.b.outcome(n, transport_error=True, write=True)
        with self.assertRaises(MeterStop):
            self.b.admit("api", "POST", True, write_allowed=True)

    def test_failed_read_transport_latches_before_any_new_request(self):
        first = self.b.admit("aws", "Scan", False)
        in_flight = self.b.admit("api", "OPTIONS", False)
        with self.assertRaisesRegex(MeterStop, "read-transport-failure"):
            self.b.outcome(first, transport_error=True)
        # Requests admitted before the failure still receive their outcomes.
        self.b.outcome(in_flight, 200)
        for category, operation, write in [
            ("aws", "Scan", False),
            ("api", "OPTIONS", False),
            ("api", "POST", True),
            ("api", "DELETE", True),
        ]:
            with self.assertRaisesRegex(MeterStop, "latched-stop"):
                self.b.admit(category, operation, write, write_allowed=True)
        with open(self.path, encoding="utf8") as file:
            rows = [json.loads(line) for line in file]
        self.assertEqual(sum(r["event"] == "stop" for r in rows), 1)
        self.assertEqual(sum(r["event"] == "attempt" for r in rows), 2)
        self.assertEqual(sum(r["event"] == "outcome" for r in rows), 2)

    def test_options_error_is_not_silently_treated_as_cancellation(self):
        n = self.b.admit("api", "OPTIONS", False)
        with self.assertRaisesRegex(MeterStop, "read-transport-failure"):
            self.b.outcome(n, transport_error=True)

    def test_unknown_read_status_stops(self):
        n = self.b.admit("api", "GET", False)
        with self.assertRaisesRegex(MeterStop, "read-transport-failure"):
            self.b.outcome(n)

    def test_poll_interval_and_time(self):
        self.b.admit("api", "GET", False, True, b"opaque-in-memory-key")
        self.now[0] += 1.99
        with self.assertRaises(MeterStop):
            self.b.admit("api", "GET", False, True, b"opaque-in-memory-key")

    def test_deadline_stops_before_forward(self):
        self.b.admit("api", "GET", False)
        self.now[0] += 7200
        with self.assertRaises(MeterStop):
            self.b.admit("api", "GET", False)

    def test_classification_retains_no_material(self):
        canary = "private-canary-never-record"
        category, operation, write, poll = self.b.classify(
            "POST",
            "dynamodb.us-east-2.amazonaws.com",
            "/?secret=" + canary,
            {"x-amz-target": "DynamoDB_20120810.Scan", "authorization": canary},
            json.dumps({"TableName": canary}).encode(),
        )
        self.b.admit(category, operation, write, poll)
        with open(self.path, encoding="utf8") as file:
            raw = file.read()
        self.assertNotIn(canary, raw)
        self.assertNotIn("authorization", raw)
        self.assertNotIn("TableName", raw)

    def test_direct_target_and_unknown_origin_rejected(self):
        with self.assertRaises(MeterStop):
            self.b.classify(
                "GET",
                "8dnymkcoa0.execute-api.us-east-2.amazonaws.com",
                "/execution-fixture/assertions-values?fixture=opaque",
                {},
                b"",
            )
        self.assertEqual(self.b.counts["api"], 0)

    def test_actual_api_and_poll_ceilings_include_failed_attempts(self):
        with tempfile.TemporaryDirectory() as directory:
            b = Budget(os.path.join(directory, "api.jsonl"))
            try:
                for _ in range(500):
                    n = b.admit("api", "GET", False)
                    b.outcome(n, 404)
                with self.assertRaises(MeterStop):
                    b.admit("api", "GET", False)
                self.assertEqual(b.counts["api"], 500)
            finally:
                b.close()
            b = Budget(os.path.join(directory, "poll.jsonl"), clock=lambda: self.now[0])
            try:
                for _ in range(240):
                    b.admit("api", "GET", False, True, b"same-resource")
                    self.now[0] += 2
                with self.assertRaises(MeterStop):
                    b.admit("api", "GET", False, True, b"same-resource")
                self.assertEqual(b.counts["poll"], 240)
            finally:
                b.close()

    def test_actual_aws_ceiling_includes_writes_and_authentication(self):
        with tempfile.TemporaryDirectory() as directory:
            b = Budget(os.path.join(directory, "aws.jsonl"))
            try:
                for n in range(1500):
                    operation = ("Scan", "CreateOAuth2Token", "PUT")[n % 3]
                    b.admit("aws", operation, operation != "Scan", write_allowed=True)
                with self.assertRaises(MeterStop):
                    b.admit("aws", "Scan", False)
                self.assertEqual(b.counts["aws"], 1500)
            finally:
                b.close()


if __name__ == "__main__":
    unittest.main()
