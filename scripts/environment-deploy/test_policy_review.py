"""Compare gates locally using real SDK wire serialization; no sockets."""

import importlib.util
import json
import sys
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
from urllib.parse import urlsplit

import driver
from common import Held
from denial_diagnostics import denial_projection
from permissions import pending_reads
from policy import Stop
from policy import classify as old_classify
from s3control_tags import ARN
from wire import Policy

spec = importlib.util.spec_from_file_location(
    "environment_unit_fixture", Path(__file__).with_name("test_unit.py")
)
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)

S, R = fixture.S, fixture.R


class PolicyReviewTests(unittest.TestCase):
    def test_owned_bucket_policy_read_passed_old_preview_but_stays_held(self):
        request = fixture.capture("s3", "get_bucket_policy", {"Bucket": S["webBucket"]})
        u = urlsplit(request.url)
        target = u.path + "?" + u.query
        self.assertEqual(
            old_classify(
                request.method, u.hostname, target, request.headers, request.body or b""
            ),
            ("s3", "GET"),
        )
        with self.assertRaisesRegex(Held, "s3-read-scope-held"):
            Policy(S, R, {}, phase="collect").request(
                request.method, u.hostname, target, request.headers, request.body or b""
            )

    def test_same_exact_s3control_read_is_accepted_by_both_gates(self):
        request = fixture.capture(
            "s3control",
            "list_tags_for_resource",
            {"AccountId": S["account"], "ResourceArn": ARN},
        )
        u = urlsplit(request.url)
        body = request.body or b""
        self.assertEqual(
            old_classify(request.method, u.hostname, u.path, request.headers, body),
            ("s3control", "ListTagsForResource"),
        )
        result = Policy(S, R, {}, phase="collect").request(
            request.method, u.hostname, u.path, request.headers, body
        )
        self.assertEqual(
            result, {"operation": "s3control/ListTagsForResource", "write": False}
        )

    def test_wrong_s3control_resource_query_and_body_still_hold(self):
        request = fixture.capture(
            "s3control",
            "list_tags_for_resource",
            {"AccountId": S["account"], "ResourceArn": ARN},
        )
        u = urlsplit(request.url)
        for target, body in [
            (u.path + "?extra=1", b""),
            (u.path + "other", b""),
            (u.path, b"private-canary"),
        ]:
            with self.assertRaises(Stop):
                old_classify("GET", u.hostname, target, request.headers, body)
            with self.assertRaises(Held):
                Policy(S, R, {}, phase="collect").request(
                    "GET", u.hostname, target, request.headers, body
                )

    def test_known_missing_permissions_are_proposals_not_grants(self):
        missing = pending_reads(S, R)
        self.assertEqual(
            [(x["service"], x["operation"]) for x in missing],
            [
                ("s3", "GetBucketPolicy"),
                ("s3", "GetObject"),
                ("cloudfront-keyvaluestore", "GetKey"),
            ],
        )
        self.assertEqual(missing[1]["key"], "assets/index-CD_8s4hz.js")
        self.assertEqual(missing[2]["key"], "76ef:/assets/index-CD_8s4hz.js")

    def test_start_refuses_known_missing_permissions_before_claim_or_authentication(
        self,
    ):
        with (
            patch.object(
                sys,
                "argv",
                [
                    "driver.py",
                    "start",
                    "--campaign",
                    ".build/project-environments-execution/release-from-390",
                ],
            ),
            patch.object(driver, "check", return_value=(R, S, {})),
            patch.object(driver, "private_write") as write,
            patch.object(driver, "credentials") as auth,
            self.assertRaisesRegex(Held, "scoped-read-approval-required"),
        ):
            driver.main()
        write.assert_not_called()
        auth.assert_not_called()

    def test_private_diagnostic_never_records_headers_bodies_or_unknown_values(self):
        canary = "secret-private-canary"
        request = SimpleNamespace(
            host=S["webBucket"] + ".s3.us-east-2.amazonaws.com",
            method="GET",
            path="/?policy=" + canary,
            headers={"authorization": canary},
            raw_content=canary.encode(),
        )
        result = denial_projection(request, S, "s3-read-scope-held")
        self.assertEqual(
            result,
            {
                "operation": "s3/GetBucketPolicy",
                "condition": "s3-read-scope-held",
                "admitted": False,
                "beforeForward": True,
            },
        )
        self.assertNotIn(canary, json.dumps(result))
        self.assertEqual(
            denial_projection(request, S, canary)["condition"], "unclassified-held"
        )
        request.host = canary
        self.assertEqual(
            denial_projection(request, S, canary)["operation"], "unclassified"
        )


if __name__ == "__main__":
    unittest.main()
