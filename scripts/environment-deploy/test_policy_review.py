"""Compare gates locally using real SDK wire serialization; no sockets."""

import copy
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
from permissions import BUCKET_CONFIGURATION_READS, pending_reads
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
    def test_owned_bucket_policy_read_is_now_exactly_approved(self):
        request = fixture.capture("s3", "get_bucket_policy", {"Bucket": S["webBucket"]})
        u = urlsplit(request.url)
        target = u.path + "?" + u.query
        self.assertEqual(
            old_classify(
                request.method, u.hostname, target, request.headers, request.body or b""
            ),
            ("s3", "GET"),
        )
        result = Policy(S, R, {}, phase="collect").request(
            request.method, u.hostname, target, request.headers, request.body or b""
        )
        self.assertEqual(result, {"operation": "s3/GetBucketPolicy", "write": False})

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

    def test_all_fourteen_approved_reads_clear_preflight(self):
        self.assertEqual(pending_reads(S, R), [])
        stripped = copy.deepcopy(S)
        stripped.pop("approvedAdditionalReads")
        self.assertEqual(len(pending_reads(stripped, R)), 14)
        for operation, _ in BUCKET_CONFIGURATION_READS:
            scoped = copy.deepcopy(S)
            scoped["approvedAdditionalReads"] = [
                x
                for x in scoped["approvedAdditionalReads"]
                if x["operation"] != operation
            ]
            with self.subTest(operation=operation):
                self.assertEqual(
                    [x["operation"] for x in pending_reads(scoped, R)], [operation]
                )

    def test_start_still_refuses_missing_permissions_before_claim_or_authentication(
        self,
    ):
        stripped = copy.deepcopy(S)
        stripped.pop("approvedAdditionalReads")
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
            patch.object(driver, "check", return_value=(R, stripped, {})),
            patch.object(driver, "private_write") as write,
            patch.object(driver, "credentials") as auth,
            self.assertRaisesRegex(Held, "scoped-read-approval-required"),
        ):
            driver.main()
        write.assert_not_called()
        auth.assert_not_called()

    def request(
        self,
        service,
        op,
        params,
        *,
        target_suffix="",
        body=None,
        method=None,
        headers=None,
        phase="collect",
    ):
        request = fixture.capture(service, op, params)
        parsed = urlsplit(request.url)
        target = (
            parsed.path + ("?" + parsed.query if parsed.query else "") + target_suffix
        )
        h = dict(request.headers)
        h.update(headers or {})
        policy = Policy(S, R, {}, phase=phase)
        if phase == "deploy":
            policy.identity_verified = True
            policy.closed_verified = True
            policy.artifacts_ready = True
        return policy.request(
            method or request.method,
            parsed.hostname,
            target,
            h,
            request.body or b"" if body is None else body,
        )

    def test_actual_sdk_serialization_all_four_reads(self):
        cases = [
            ("s3", "get_bucket_acl", {"Bucket": S["webBucket"]}, "s3/GetBucketAcl"),
            (
                "s3",
                "get_bucket_policy",
                {"Bucket": S["webBucket"]},
                "s3/GetBucketPolicy",
            ),
            (
                "s3",
                "get_object",
                {"Bucket": S["webBucket"], "Key": "assets/index-CD_8s4hz.js"},
                "s3/GetObject",
            ),
            (
                "cloudfront-keyvaluestore",
                "get_key",
                {"KvsARN": S["kvsArn"], "Key": "76ef:/assets/index-CD_8s4hz.js"},
                "cloudfront-keyvaluestore/GetKey",
            ),
        ]
        for service, op, params, name in cases:
            with self.subTest(operation=op):
                result = self.request(service, op, params)
                self.assertEqual(result["operation"], name)
                self.assertFalse(result["write"])

    def test_s3_policy_denies_other_bucket_selector_body_and_mutation(self):
        for params, extras in [
            ({"Bucket": S["assetBucket"]}, {}),
            ({"Bucket": S["webBucket"]}, {"target_suffix": "&versionId=other"}),
            ({"Bucket": S["webBucket"]}, {"target_suffix": "&policy="}),
            ({"Bucket": S["webBucket"]}, {"body": b"{}"}),
            ({"Bucket": S["webBucket"]}, {"method": "HEAD"}),
            ({"Bucket": S["webBucket"]}, {"method": "PUT"}),
            (
                {"Bucket": S["webBucket"]},
                {"headers": {"x-amz-expected-bucket-owner": "000000000000"}},
            ),
        ]:
            with self.subTest(params=params, extras=extras), self.assertRaises(Held):
                self.request("s3", "get_bucket_policy", params, **extras)
        for operation in ["get_bucket_ownership_controls", "get_bucket_tagging"]:
            with self.subTest(operation=operation), self.assertRaises(Held):
                self.request("s3", operation, {"Bucket": S["webBucket"]})

    def test_s3_old_asset_denies_other_key_version_body_range_head_and_writes(self):
        base = {"Bucket": S["webBucket"], "Key": "assets/index-CD_8s4hz.js"}
        for params, extras in [
            ({**base, "Key": "assets/other.js"}, {}),
            ({**base, "Bucket": S["assetBucket"]}, {}),
            ({**base, "VersionId": "other"}, {}),
            (base, {"body": b"private-canary"}),
            (base, {"headers": {"Range": "bytes=0-1"}}),
            (base, {"method": "HEAD"}),
            (base, {"target_suffix": "?other=value"}),
        ]:
            with self.subTest(params=params, extras=extras), self.assertRaises(Held):
                self.request("s3", "get_object", params, **extras)
        for operation, params in [
            ("put_object", {**base, "Body": b"synthetic"}),
            ("delete_object", base),
        ]:
            with self.subTest(operation=operation), self.assertRaises(Held):
                self.request("s3", operation, params, phase="deploy")

    def test_kvs_exact_read_denies_other_store_key_selectors_body_and_mutations(self):
        base = {"KvsARN": S["kvsArn"], "Key": "76ef:/assets/index-CD_8s4hz.js"}
        for params, extras in [
            ({**base, "Key": "76ef:/assets/other.js"}, {}),
            ({**base, "Key": "other:/assets/index-CD_8s4hz.js"}, {}),
            (
                {**base, "KvsARN": S["kvsArn"].replace("747336059622", "000000000000")},
                {},
            ),
            (base, {"target_suffix": "?NextToken=other"}),
            (base, {"body": b"{}"}),
        ]:
            with self.subTest(params=params, extras=extras), self.assertRaises(Held):
                self.request("cloudfront-keyvaluestore", "get_key", params, **extras)
        with self.assertRaises(Held):
            self.request(
                "cloudfront-keyvaluestore",
                "put_key",
                {**base, "Value": "s3", "IfMatch": "synthetic"},
                phase="deploy",
            )

    def test_empty_read_framing_is_canonical_for_all_added_reads(self):
        cases = [
            ("s3", "get_bucket_policy", {"Bucket": S["webBucket"]}),
            ("s3", "get_bucket_acl", {"Bucket": S["webBucket"]}),
            (
                "s3",
                "get_object",
                {"Bucket": S["webBucket"], "Key": "assets/index-CD_8s4hz.js"},
            ),
            (
                "cloudfront-keyvaluestore",
                "get_key",
                {"KvsARN": S["kvsArn"], "Key": "76ef:/assets/index-CD_8s4hz.js"},
            ),
        ]
        for service, op, params in cases:
            for header in [
                {"Content-Length": "2"},
                {"Transfer-Encoding": "chunked"},
                {"Content-Encoding": "gzip"},
            ]:
                with self.subTest(operation=op, header=header), self.assertRaises(Held):
                    self.request(service, op, params, headers=header)

    def test_grants_are_not_uploads_wildcards_or_general_kvs_keys(self):
        scoped = copy.deepcopy(S)
        scoped["approvedAdditionalReads"][1]["key"] = "*"
        p = Policy(scoped, R, {}, phase="collect")
        with self.assertRaises(Held):
            p.request(
                "GET",
                S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                "/assets/index-CD_8s4hz.js",
                {},
                b"",
            )
        kv = {"KvsARN": S["kvsArn"], "Key": "76ef:/assets/index-CD_8s4hz.js"}
        self.assertFalse(
            Policy(S, R, {}, phase="collect").read(
                "cloudfront-keyvaluestore", "ListKeys", kv
            )
        )
        self.assertFalse(
            Policy(S, R, {}, phase="collect").read(
                "cloudfront-keyvaluestore", "GetKey", {**kv, "NextToken": "other"}
            )
        )

    def test_acl_grant_is_exact_bucket_root_empty_read_only(self):
        base = {"Bucket": S["webBucket"]}
        self.assertEqual(
            self.request(
                "s3", "get_bucket_acl", {**base, "ExpectedBucketOwner": S["account"]}
            )["operation"],
            "s3/GetBucketAcl",
        )
        for params, extras in [
            ({"Bucket": S["assetBucket"]}, {}),
            ({**base, "ExpectedBucketOwner": "000000000000"}, {}),
            (base, {"body": b"{}"}),
            (base, {"method": "HEAD"}),
            (base, {"method": "PUT", "phase": "deploy"}),
            (base, {"target_suffix": "&acl="}),
            (base, {"target_suffix": "&policy"}),
            (base, {"target_suffix": "&versionId=other"}),
            (base, {"headers": {"Range": "bytes=0-1"}}),
        ]:
            with self.subTest(params=params, extras=extras), self.assertRaises(Held):
                self.request("s3", "get_bucket_acl", params, **extras)
        for target in [
            "/other?acl",
            "//?acl",
            "/?%61cl",
            "/?acl=other",
            "/?acl&other=1",
        ]:
            with self.subTest(target=target), self.assertRaises(Held):
                Policy(S, R, {}, phase="collect").request(
                    "GET",
                    S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                    target,
                    {},
                    b"",
                )
        for operation, params in [
            ("put_bucket_acl", {**base, "ACL": "private"}),
            ("delete_bucket_policy", base),
        ]:
            with self.subTest(operation=operation), self.assertRaises(Held):
                self.request("s3", operation, params, phase="deploy")

    @staticmethod
    def sdk_name(operation):
        return "".join(
            ("_" + c.lower() if c.isupper() else c) for c in operation
        ).lstrip("_")

    def test_ten_configuration_reads_use_actual_sdk_serialization(self):
        for operation, selector in BUCKET_CONFIGURATION_READS:
            for params in [
                {"Bucket": S["webBucket"]},
                {"Bucket": S["webBucket"], "ExpectedBucketOwner": S["account"]},
            ]:
                with self.subTest(
                    operation=operation, owner="ExpectedBucketOwner" in params
                ):
                    self.assertEqual(
                        self.request("s3", self.sdk_name(operation), params),
                        {"operation": "s3/" + operation, "write": False},
                    )
            for target in ["/?" + selector, "/?" + selector + "="]:
                self.assertEqual(
                    Policy(S, R, {}, phase="collect").request(
                        "GET",
                        S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                        target,
                        {},
                        b"",
                    )["operation"],
                    "s3/" + operation,
                )

    def test_ten_configuration_reads_reject_changed_sdk_requests(self):
        for operation, selector in BUCKET_CONFIGURATION_READS:
            base = {"Bucket": S["webBucket"]}
            for params, extras in [
                ({"Bucket": S["assetBucket"]}, {}),
                ({"Bucket": "unapproved-bucket"}, {}),
                ({**base, "ExpectedBucketOwner": "000000000000"}, {}),
                (base, {"body": b"{}"}),
                *[
                    (base, {"method": method, "phase": "deploy"})
                    for method in ["PUT", "DELETE", "POST", "HEAD"]
                ],
                (base, {"target_suffix": "&" + selector + "="}),
                (base, {"target_suffix": "&other="}),
                (base, {"target_suffix": "&versionId=other"}),
                (base, {"target_suffix": "&x-id=" + operation}),
                *[
                    (base, {"headers": h})
                    for h in [
                        {"Content-Length": "1"},
                        {"Transfer-Encoding": "chunked"},
                        {"Content-Encoding": "gzip"},
                        {"Range": "bytes=0-1"},
                        {"If-Match": "fixture"},
                        {"x-amz-request-payer": "requester"},
                    ]
                ],
            ]:
                with (
                    self.subTest(operation=operation, params=params, extras=extras),
                    self.assertRaises(Held),
                ):
                    self.request("s3", self.sdk_name(operation), params, **extras)

    def test_ten_configuration_reads_reject_noncanonical_targets_and_scope(self):
        for operation, selector in BUCKET_CONFIGURATION_READS:
            for target in [
                "/other?" + selector,
                "//?" + selector,
                "/%2F?" + selector,
                "/?" + selector + "=value",
                "/?" + selector + "=&",
                "/?&" + selector,
                "/?" + selector + "==",
                "/?%" + format(ord(selector[0]), "02x") + selector[1:],
                "/?" + selector.upper(),
                " /?" + selector,
                "/?" + selector[:1] + "\n" + selector[1:],
                "/?" + selector + "\t",
            ]:
                with (
                    self.subTest(operation=operation, target=target),
                    self.assertRaises(Held),
                ):
                    Policy(S, R, {}, phase="collect").request(
                        "GET",
                        S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                        target,
                        {},
                        b"",
                    )
            for field, value in [("bucket", "*"), ("key", "*"), ("operation", "*")]:
                scoped = copy.deepcopy(S)
                next(
                    x
                    for x in scoped["approvedAdditionalReads"]
                    if x["operation"] == operation
                )[field] = value
                with (
                    self.subTest(operation=operation, field=field),
                    self.assertRaises(Held),
                ):
                    Policy(scoped, R, {}, phase="collect").request(
                        "GET",
                        S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                        "/?" + selector,
                        {},
                        b"",
                    )
            for host in [
                S["webBucket"] + ".s3.us-east-1.amazonaws.com",
                S["webBucket"] + ".s3.amazonaws.com",
            ]:
                with (
                    self.subTest(operation=operation, host=host),
                    self.assertRaises(Held),
                ):
                    Policy(S, R, {}, phase="collect").request(
                        "GET", host, "/?" + selector, {}, b""
                    )

    def test_unreviewed_bucket_operations_still_block_actual_sdk(self):
        for operation, extra in [
            ("get_bucket_tagging", {}),
            ("get_bucket_ownership_controls", {}),
            ("get_public_access_block", {}),
            ("get_bucket_notification_configuration", {}),
            (
                "put_bucket_versioning",
                {"VersioningConfiguration": {"Status": "Enabled"}},
            ),
            ("delete_bucket_cors", {}),
        ]:
            with self.subTest(operation=operation), self.assertRaises(Held):
                self.request(
                    "s3", operation, {"Bucket": S["webBucket"], **extra}, phase="deploy"
                )

    def test_offline_response_oracle_is_model_exact_and_never_handles_writes(self):
        from fixture_bucket_reads import fixture_response, validate_model

        validate_model()
        for operation, selector in BUCKET_CONFIGURATION_READS:
            request = SimpleNamespace(
                method="GET",
                host=S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                path="/?" + selector,
                raw_content=b"",
            )
            self.assertEqual(fixture_response(request, S)[0], operation)
            for field, value in [
                ("method", "PUT"),
                ("method", "POST"),
                ("host", S["assetBucket"] + ".s3.us-east-2.amazonaws.com"),
                ("path", "/other?" + selector),
                ("path", "/?" + selector + "&extra=1"),
                ("path", "/?session"),
                ("raw_content", b"private"),
            ]:
                changed = copy.copy(request)
                setattr(changed, field, value)
                self.assertIsNone(fixture_response(changed, S))

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
