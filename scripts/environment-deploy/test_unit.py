"""Offline execution policy, ledger and actual-byte regressions; no AWS transports."""

import base64
import concurrent.futures
import json
import os
import socket
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch
from urllib.parse import urlsplit

from botocore.config import Config
from botocore.session import Session
from common import ROOT, Campaign, Held, strict_json
from driver import check
from uploads import Uploads, payload
from wire import Policy

S = json.loads(
    (ROOT / "docs/development/project-environments-execution-scope.json").read_bytes()
)
R = json.loads(
    (ROOT / "docs/development/project-environments-adopted-release.json").read_bytes()
)
PINS = json.loads(Path(__file__).with_name("pins.json").read_bytes())


class Captured(BaseException):
    pass


def capture(service, operation, params):
    c = Session().create_client(
        service,
        region_name="us-east-2",
        aws_access_key_id="AKIA0000000000000000",
        aws_secret_access_key="offline",
        aws_session_token="offline",
        config=Config(retries={"total_max_attempts": 1}, proxies={}),
    )
    data = []

    def take(request, **_):
        data.append(request)
        raise Captured()

    c.meta.events.register("before-send", take)
    try:
        with (
            patch.object(socket, "socket", side_effect=AssertionError("no sockets")),
            patch.object(socket, "getaddrinfo", side_effect=AssertionError("no DNS")),
        ):
            getattr(c, operation)(**params)
    except Captured:
        pass
    finally:
        c.close()
    return data[0]


def classify(policy, service, op, params):
    q = capture(service, op, params)
    u = urlsplit(q.url)
    body = q.body or b""
    if isinstance(body, str):
        body = body.encode()
    return policy.request(
        q.method,
        u.hostname,
        u.path + ("?" + u.query if u.query else ""),
        q.headers,
        body,
    )


def new_campaign(d, maximum=1799, clock=time.monotonic, boot=lambda: "fixture-boot"):
    return Campaign(
        Path(d) / "campaign",
        create={
            "prior": 301,
            "maximum": maximum,
            "seconds": 3300,
            "manifestSha256": PINS["manifest"],
            "scopeSha256": PINS["scope"],
            "reviewCommit": "a" * 40,
            "authorizationSha256": "b" * 64,
        },
        clock=clock,
        boot_id=boot,
    )


class Tests(unittest.TestCase):
    def policy(self, phase="collect"):
        p = Policy(S, R, {}, phase=phase)
        p.identity_verified = True
        p.closed_verified = True
        return p

    def test_adoption_and_frozen_parent_attestation_verify_locally(self):
        r, s, _p = check()
        self.assertTrue(r["adoption"]["accepted"])
        self.assertFalse(r["deployment"]["permitted"])
        self.assertEqual(s["account"], "747336059622")

    def test_real_sdk_read_serialization_matches_exact_scopes(self):
        p = self.policy()
        fn = S["functions"]["UsersProjectsApi"]["name"]
        role = S["functions"]["UsersProjectsApi"]["role"].split("/")[-1]
        requests = [
            ("sts", "get_caller_identity", {}),
            ("lambda", "get_function_configuration", {"FunctionName": fn}),
            ("lambda", "get_policy", {"FunctionName": fn}),
            ("lambda", "get_function_concurrency", {"FunctionName": fn}),
            ("lambda", "list_provisioned_concurrency_configs", {"FunctionName": fn}),
            ("lambda", "get_account_settings", {}),
            (
                "lambda",
                "list_event_source_mappings",
                {"FunctionName": S["outputs"]["workerFunction"]},
            ),
            ("iam", "get_role", {"RoleName": role}),
            ("ssm", "get_parameter", {"Name": "/sst/bootstrap"}),
            ("freetier", "get_account_plan_state", {}),
            (
                "apigatewayv2",
                "get_stage",
                {"ApiId": S["apiId"], "StageName": "$default"},
            ),
            ("apigatewayv2", "get_routes", {"ApiId": S["apiId"]}),
            (
                "sqs",
                "get_queue_attributes",
                {
                    "QueueUrl": S["outputs"]["executionQueueUrl"],
                    "AttributeNames": ["ApproximateNumberOfMessages"],
                },
            ),
            ("events", "describe_rule", {"Name": S["outputs"]["cleanupSchedule"]}),
            ("cloudfront", "get_distribution", {"Id": S["distribution"]}),
            (
                "cloudfront-keyvaluestore",
                "describe_key_value_store",
                {"KvsARN": S["kvsArn"]},
            ),
            (
                "dynamodb",
                "get_item",
                {
                    "TableName": S["controlTable"],
                    "Key": {"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "META"}},
                    "ConsistentRead": True,
                },
            ),
        ]
        for service, op, args in requests:
            with self.subTest(service=service, op=op):
                self.assertFalse(classify(p, service, op, args)["write"])

    def test_wrong_functions_accounts_regions_tables_parameters_and_mutations_hold(
        self,
    ):
        cases = [
            ("lambda", "get_function_configuration", {"FunctionName": "other"}),
            (
                "lambda",
                "delete_function",
                {"FunctionName": S["outputs"]["apiFunction"]},
            ),
            ("ssm", "get_parameter", {"Name": "/unrelated"}),
            (
                "dynamodb",
                "get_item",
                {
                    "TableName": "other",
                    "Key": {"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "META"}},
                    "ConsistentRead": True,
                },
            ),
            ("iam", "get_role", {"RoleName": "other"}),
            ("apigatewayv2", "get_api", {"ApiId": "other"}),
            ("cloudfront", "get_distribution", {"Id": "OTHER"}),
            (
                "cloudfront-keyvaluestore",
                "describe_key_value_store",
                {"KvsARN": S["kvsArn"].replace("747336059622", "000000000000")},
            ),
        ]
        for s, o, a in cases:
            with self.subTest(s=s, o=o), self.assertRaises(Held):
                classify(self.policy(), s, o, a)
        for h in [
            "lambda.us-east-1.amazonaws.com",
            "747336059622.cloudfront-kvs.global.api.aws.evil",
            "iam.amazonaws.com.",
            "127.0.0.1",
        ]:
            with self.assertRaises(Held):
                self.policy().host(h)

    def test_scan_is_metadata_only_consistent_and_bounded(self):
        names = {"#p" + str(i): v for i, v in enumerate(S["projectionFields"])}
        args = {
            "TableName": S["controlTable"],
            "Limit": 100,
            "ConsistentRead": True,
            "ProjectionExpression": ",".join(names),
            "ExpressionAttributeNames": names,
        }
        self.assertFalse(classify(self.policy(), "dynamodb", "scan", args)["write"])
        for change in [
            {"ConsistentRead": False},
            {"Limit": 1000},
            {"ProjectionExpression": "configurationJSON"},
            {"ExpressionAttributeNames": {"#secret": "ciphertext"}},
        ]:
            with self.assertRaises(Held):
                classify(self.policy(), "dynamodb", "scan", {**args, **change})

    def test_stage_transaction_requires_exact_approved_body_and_single_attempt(self):
        p = self.policy("close")
        p.stage_transaction = {
            "TransactItems": [
                {
                    "ConditionCheck": {
                        "TableName": S["controlTable"],
                        "Key": {"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "META"}},
                        "ConditionExpression": "attribute_exists(PK)",
                    }
                }
            ],
            "ClientRequestToken": "fixed-offline-token",
        }
        self.assertTrue(
            classify(p, "dynamodb", "transact_write_items", p.stage_transaction)[
                "write"
            ]
        )
        with self.assertRaises(Held):
            classify(p, "dynamodb", "transact_write_items", p.stage_transaction)
        original = p.stage_transaction
        for patching in [
            {"ClientRequestToken": "other"},
            {
                "TransactItems": [
                    {
                        "ConditionCheck": {
                            "TableName": S["protectedTable"],
                            "Key": {"PK": {"S": "STAGE#dev-api"}, "SK": {"S": "META"}},
                            "ConditionExpression": "attribute_exists(PK)",
                        }
                    }
                ]
            },
        ]:
            p = self.policy("activate")
            p.stage_transaction = original
            with self.assertRaises(Held):
                classify(
                    p, "dynamodb", "transact_write_items", {**original, **patching}
                )

    def test_writes_need_actual_identity_and_closed_witness(self):
        for identity, closed in [(False, True), (True, False)]:
            p = self.policy("deploy")
            p.identity_verified = identity
            p.closed_verified = closed
            with self.assertRaises(Held):
                classify(
                    p,
                    "lambda",
                    "add_permission",
                    {
                        "FunctionName": S["outputs"]["apiFunction"],
                        "Action": "lambda:InvokeFunction",
                        "Principal": "apigateway.amazonaws.com",
                        "SourceArn": S["permissions"][0]["sourceArn"],
                        "StatementId": S["permissions"][0]["statementId"],
                    },
                )

    def test_permission_exact_sid_principal_api_scope_and_target(self):
        wanted = S["permissions"][0]
        a = {
            "FunctionName": S["outputs"]["apiFunction"],
            "Action": wanted["action"],
            "Principal": wanted["principal"],
            "SourceArn": wanted["sourceArn"],
            "StatementId": wanted["statementId"],
        }
        self.assertTrue(
            classify(self.policy("deploy"), "lambda", "add_permission", a)["write"]
        )
        for k, v in [
            ("FunctionName", S["outputs"]["workerFunction"]),
            ("Principal", "*"),
            ("SourceArn", "arn:aws:execute-api:us-east-2:747336059622:other/*"),
            ("StatementId", "other"),
        ]:
            with self.assertRaises(Held):
                classify(self.policy("deploy"), "lambda", "add_permission", {**a, k: v})

    def test_dynamic_integration_ids_learned_only_from_scoped_upstream_response(self):
        p = self.policy("deploy")
        a = {
            "ApiId": S["apiId"],
            "ConnectionType": "INTERNET",
            "IntegrationType": "AWS_PROXY",
            "IntegrationUri": S["functions"]["UsersProjectsApi"]["arn"],
            "PassthroughBehavior": "WHEN_NO_MATCH",
            "PayloadFormatVersion": "2.0",
        }
        meta = classify(p, "apigatewayv2", "create_integration", a)
        route = {
            "ApiId": S["apiId"],
            "RouteKey": S["routes"][0],
            "AuthorizationType": "NONE",
            "Target": "integrations/new123",
        }
        with self.assertRaises(Held):
            classify(p, "apigatewayv2", "create_route", route)
        p.response(meta, 201, {}, b'{"integrationId":"new123"}')
        self.assertTrue(classify(p, "apigatewayv2", "create_route", route)["write"])
        for key, val in [
            ("RouteKey", "ANY /unreviewed"),
            ("Target", "integrations/other"),
            ("ApiId", "other"),
            ("AuthorizationType", "CUSTOM"),
        ]:
            with self.assertRaises(Held):
                classify(
                    self.policy("deploy"),
                    "apigatewayv2",
                    "create_route",
                    {**route, key: val},
                )

    def test_json_duplicates_and_body_shape_aliases_fail_closed(self):
        for data in [b'{"Action":1,"Action":2}', b'{"value":NaN}']:
            with self.assertRaises(Held):
                strict_json(data)
        p = self.policy()
        with self.assertRaises(Held):
            p.request(
                "POST",
                "dynamodb.us-east-2.amazonaws.com",
                "/",
                {"X-Amz-Target": "DynamoDB_20120810.GetItem"},
                b'{"TableName":"one","TableName":"two"}',
            )

    def test_complete_upload_hash_same_filename_wrong_actual_bytes_holds(self):
        u = Uploads({("bucket", "assets/code.zip"): b"accepted-bytes"})
        with self.assertRaises(Held):
            u.request("PUT", "bucket", "assets/code.zip", "", b"rejected-bytes", {})
        op, m = u.request("PUT", "bucket", "assets/code.zip", "", b"accepted-bytes", {})
        self.assertEqual(op, "PutObject")
        u.response(m, 200, {}, b"")
        self.assertIn(("bucket", "assets/code.zip"), u.finished)

    def test_parallel_multipart_parts_validate_actual_offsets_then_complete(self):
        a = os.urandom(5 * 1024 * 1024)
        b = os.urandom(65536)
        u = Uploads({("bucket", "code.zip"): a + b})
        _, m = u.request("POST", "bucket", "code.zip", "uploads=", b"", {})
        u.response(
            m,
            200,
            {},
            b"<InitiateMultipartUploadResult><UploadId>fixture</UploadId></InitiateMultipartUploadResult>",
        )
        for n, data in [(2, b), (1, a)]:
            _, m = u.request(
                "PUT",
                "bucket",
                "code.zip",
                f"partNumber={n}&uploadId=fixture",
                data,
                {},
            )
            u.response(m, 200, {"etag": f'"part{n}"'}, b"")
        complete = b'<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>"part1"</ETag></Part><Part><PartNumber>2</PartNumber><ETag>"part2"</ETag></Part></CompleteMultipartUpload>'
        _, m = u.request("POST", "bucket", "code.zip", "uploadId=fixture", complete, {})
        u.response(m, 200, {}, b"")
        self.assertIn(("bucket", "code.zip"), u.finished)

    def test_multipart_wrong_body_upload_id_repeat_and_incomplete_commit_hold(self):
        u = Uploads({("bucket", "code.zip"): b"x" * 100})
        with self.assertRaises(Held):
            u.request(
                "PUT", "bucket", "code.zip", "partNumber=1&uploadId=other", b"x", {}
            )
        _, m = u.request("POST", "bucket", "code.zip", "uploads=", b"", {})
        u.response(
            m,
            200,
            {},
            b"<InitiateMultipartUploadResult><UploadId>fixture</UploadId></InitiateMultipartUploadResult>",
        )
        with self.assertRaises(Held):
            u.request(
                "PUT", "bucket", "code.zip", "partNumber=1&uploadId=fixture", b"bad", {}
            )
        with self.assertRaises(Held):
            u.request(
                "POST",
                "bucket",
                "code.zip",
                "uploadId=fixture",
                b"<CompleteMultipartUpload/>",
                {},
            )

    def test_aws_chunked_payload_and_checksum_trailer_are_checked(self):
        body = b"known"
        crc = base64.b64encode(
            (__import__("binascii").crc32(body) & 0xFFFFFFFF).to_bytes(4, "big")
        )
        encoded = b"5\r\nknown\r\n0\r\nx-amz-checksum-crc32:" + crc + b"\r\n\r\n"
        h = {
            "content-encoding": "aws-chunked",
            "x-amz-content-sha256": "STREAMING-UNSIGNED-PAYLOAD-TRAILER",
            "x-amz-decoded-content-length": "5",
            "x-amz-trailer": "x-amz-checksum-crc32",
        }
        self.assertEqual(payload(encoded, h), body)
        for wrong in [
            encoded.replace(b"known", b"wrong"),
            encoded + b"private",
            b"5\r\nknown",
        ]:
            with self.assertRaises(Held):
                payload(wrong, h)

    def test_campaign_count_is_shared_and_atomic_under_concurrency(self):
        with tempfile.TemporaryDirectory() as d:
            c = new_campaign(d, maximum=311)
            c.begin("collect")

            def call(_):
                x = Campaign(c.directory, boot_id=lambda: "fixture-boot")
                try:
                    n = x.admit("fixture/read", False)
                    x.outcome(n, 200)
                    return n
                finally:
                    x.close()

            with concurrent.futures.ThreadPoolExecutor(max_workers=5) as pool:
                numbers = list(pool.map(call, range(10)))
            self.assertEqual(sorted(numbers), list(range(302, 312)))
            with self.assertRaises(Held):
                c.admit("fixture/read", False)
            self.assertEqual(c.get("count"), 311)
            c.close()

    def test_shared_deadline_and_closed_deadline_cannot_reset_between_phases(self):
        now = [100.0]
        with tempfile.TemporaryDirectory() as d:
            c = new_campaign(d, clock=lambda: now[0])
            c.begin("collect")
            c.finish("collect", {})
            deadline = c.get("deadline")
            c.close()
            now[0] = 101
            c = Campaign(
                Path(d) / "campaign",
                clock=lambda: now[0],
                boot_id=lambda: "fixture-boot",
            )
            self.assertEqual(c.get("deadline"), deadline)
            c.begin("close")
            closed = c.get("closedDeadline")
            c.finish("close", {})
            now[0] = closed
            with self.assertRaises(Held):
                c.begin("deploy")
            c.close()

    def test_phase_replay_wrong_order_failed_writes_and_missing_outcomes_latch(self):
        for mode in ["replay", "order", "failure", "missing", "boot"]:
            with tempfile.TemporaryDirectory() as d:
                c = new_campaign(d)
                c.begin("collect")
                if mode == "replay":
                    with self.assertRaises(Held):
                        c.begin("collect")
                elif mode == "order":
                    c.finish("collect", {})
                    with self.assertRaises(Held):
                        c.begin("deploy")
                elif mode == "failure":
                    n = c.admit("fixture/write", True)
                    with self.assertRaises(Held):
                        c.outcome(n, None)
                    with self.assertRaises(Held):
                        c.admit("fixture/write", True)
                elif mode == "missing":
                    c.admit("fixture/read", False)
                    with self.assertRaises(Held):
                        c.finish("collect", {})
                else:
                    c.boot_id = lambda: "different"
                    with self.assertRaises(Held):
                        c.check()
                self.assertTrue(c.get("stopped"))
                c.close()

    def test_outcomes_duplicates_and_redirects_hold_without_compensation(self):
        for status in [301, 403, 429, 500, 503, None]:
            with tempfile.TemporaryDirectory() as d:
                c = new_campaign(d)
                c.begin("collect")
                n = c.admit("fixture/write", True)
                with self.assertRaises(Held):
                    c.outcome(n, status)
                self.assertEqual(c.get("count"), 302)
                self.assertTrue(c.get("stopped"))
                c.close()


if __name__ == "__main__":
    unittest.main()
