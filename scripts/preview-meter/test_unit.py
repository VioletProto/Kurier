"""Pure policy and cumulative budget regressions; no network."""

import json
import tempfile
import unittest
from pathlib import Path

from policy import Budget, Stop, classify


class Tests(unittest.TestCase):
    def test_metadata_and_app_mutations_closed(self):
        held = [
            (
                "POST",
                "ssm.us-east-2.amazonaws.com",
                "/",
                {"x-amz-target": "AmazonSSM.PutParameter"},
                b'{"Name":"/sst/passphrase/kurier/dev-api"}',
            ),
            (
                "PUT",
                "sst-state-moshrdrrbeba.s3.us-east-2.amazonaws.com",
                "/app/kurier/dev-api.json",
                {},
                b"{}",
            ),
            (
                "DELETE",
                "sst-state-moshrdrrbeba.s3.us-east-2.amazonaws.com",
                "/lock/kurier/dev-api.json",
                {},
                b"",
            ),
            ("POST", "iam.amazonaws.com", "/", {}, b"Action=CreateRole"),
            ("POST", "sts.us-east-2.amazonaws.com", "/", {}, b"Action=GetSessionToken"),
            (
                "POST",
                "lambda.us-east-2.amazonaws.com",
                "/2015-03-31/functions/f/invocations",
                {},
                b"{}",
            ),
            (
                "POST",
                "dynamodb.us-east-2.amazonaws.com",
                "/",
                {"x-amz-target": "DynamoDB_20120810.TransactWriteItems"},
                b"{}",
            ),
            (
                "GET",
                "8dnymkcoa0.execute-api.us-east-2.amazonaws.com",
                "/api/v1/projects",
                {},
                b"",
            ),
            (
                "POST",
                "cloudfront.amazonaws.com",
                "/2020-05-31/distribution/x/invalidation",
                {},
                b"{}",
            ),
            (
                "PUT",
                "abc.cloudfront-kvs.global.api.aws",
                "/key-value-stores/x/keys/y",
                {},
                b"x",
            ),
            ("POST", "signin.us-east-2.amazonaws.com", "/v1/token", {}, b"{}"),
        ]
        for request in held:
            with (
                self.subTest(host=request[1], method=request[0]),
                self.assertRaises(Stop),
            ):
                classify(*request)

    def test_reads_and_closed_protocols(self):
        self.assertEqual(
            classify(
                "POST",
                "sts.us-east-2.amazonaws.com",
                "/",
                {},
                b"Action=GetCallerIdentity&Version=2011-06-15",
            ),
            ("sts", "GetCallerIdentity"),
        )
        self.assertEqual(
            classify(
                "POST",
                "ssm.us-east-2.amazonaws.com",
                "/",
                {"x-amz-target": "AmazonSSM.GetParameter"},
                b'{"Name":"/sst/bootstrap"}',
            ),
            ("ssm", "GetParameter"),
        )
        for body in [
            b"Action=GetCallerIdentity&Action=AssumeRole",
            b"Action=AssumeRole",
            b"garbage",
        ]:
            with self.assertRaises(Stop):
                classify("POST", "sts.us-east-2.amazonaws.com", "/", {}, body)
        with self.assertRaises(Stop):
            classify(
                "POST",
                "ssm.us-east-2.amazonaws.com",
                "/",
                {"x-amz-target": "AmazonSSM.GetParameter"},
                b'{"Name":"/unrelated/secret"}',
            )

    def test_shared_count_and_nonresettable_stop(self):
        with tempfile.TemporaryDirectory() as d:
            b = Budget(Path(d) / "log", prior=135, maximum=138)
            for service in ["sts", "s3", "dynamodb"]:
                b.admit(service, "read", "HTTP/2")
            with self.assertRaises(Stop):
                b.admit("sts", "read", "HTTP/1.1")
            with self.assertRaises(Stop):
                b.admit("s3", "read", "HTTP/1.1")
            b.close()
            rows = [json.loads(x) for x in (Path(d) / "log").read_text().splitlines()]
            self.assertEqual(
                [r["count"] for r in rows if r["event"] == "attempt"], [136, 137, 138]
            )

    def test_clock_and_failures_latch(self):
        for mode in ["clock", "redirect", "transport"]:
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as d:
                now = [0]
                b = Budget(Path(d) / "log", clock=lambda now=now: now[0], seconds=3)
                n = b.admit("sts", "read", "HTTP/1.1")
                with self.assertRaises(Stop):
                    if mode == "clock":
                        now[0] = 3
                        b.check()
                    elif mode == "redirect":
                        b.outcome(n, 302)
                    else:
                        b.outcome(n, error=True)
                with self.assertRaises(Stop):
                    b.admit("s3", "read", "HTTP/2")
                b.close()


if __name__ == "__main__":
    unittest.main()
