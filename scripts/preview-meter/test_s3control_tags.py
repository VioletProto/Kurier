"""Offline reviewed-resource, canonical wire and header-boundary regressions."""

import json
import socket
import unittest
from types import SimpleNamespace
from unittest.mock import patch

import policy
import s3control_tags as tags
from run import ROOT


def fixture_headers():
    signed = (
        "host;x-amz-account-id;x-amz-content-sha256;x-amz-date;x-amz-security-token"
    )
    return {
        "host": tags.HOST,
        "x-amz-account-id": tags.ACCOUNT,
        "x-amz-date": "20261029T120000Z",
        "x-amz-security-token": "offline-only-token",
        "x-amz-content-sha256": tags.EMPTY_SHA256,
        "authorization": "AWS4-HMAC-SHA256 Credential=AKIA0000000000000000/20261029/us-east-2/s3/aws4_request, "
        + "SignedHeaders="
        + signed
        + ", Signature="
        + "0" * 64,
        "amz-sdk-invocation-id": "00000000-0000-4000-8000-000000000000",
        "amz-sdk-request": "attempt=1; max=3",
        "user-agent": "offline-fixture",
    }


class Tests(unittest.TestCase):
    def allowed(
        self, *, method="GET", host=tags.HOST, path=tags.PATH, headers=None, body=b""
    ):
        return policy.classify(
            method, host, path, fixture_headers() if headers is None else headers, body
        )

    def test_exact_read_is_named_and_h2_host_header_is_optional(self):
        self.assertEqual(self.allowed(), ("s3control", "ListTagsForResource"))
        headers = fixture_headers()
        headers.pop("host")
        self.assertEqual(
            self.allowed(headers=headers), ("s3control", "ListTagsForResource")
        )
        self.assertNotIn(
            "s3control", policy.REST | set(policy.JSON_READS) | set(policy.QUERY_READS)
        )

    def test_only_observed_inventory_bucket_is_scoped(self):
        inventory = json.loads(
            (ROOT / "docs/development/project-environments-inventory.json").read_text()
        )
        self.assertIn(tags.BUCKET, inventory["buckets"])
        self.assertIn(tags.ARN, json.dumps(inventory))
        for bucket in inventory["buckets"]:
            if bucket == tags.BUCKET:
                continue
            path = tags.PATH.replace(tags.BUCKET, bucket)
            with self.assertRaisesRegex(policy.Stop, "s3control-operation-held"):
                self.allowed(path=path)

    def test_mutations_other_operations_and_nonempty_bodies_are_held(self):
        for method in ["HEAD", "PUT", "POST", "PATCH", "DELETE", "OPTIONS", "CONNECT"]:
            with self.assertRaises(policy.Stop):
                self.allowed(method=method)
        for body in [b"x", "", None, b"{}"]:
            with self.assertRaises(policy.Stop):
                self.allowed(body=body)
        with self.assertRaises(policy.Stop):
            self.allowed(path="/v20180820/configuration/publicAccessBlock")

    def test_noncanonical_and_ambiguous_encodings_and_parameters_are_held(self):
        paths = [
            tags.PATH.replace("%3A", ":"),
            tags.PATH.replace("%3A", "%3a"),
            tags.PATH.replace("%3A", "%253A"),
            tags.PATH.replace("arn", "%61rn"),
            tags.PATH + "/",
            tags.PATH + "?",
            tags.PATH + "?nextToken=x",
            tags.PATH + "?ResourceArn=" + tags.ARN,
            tags.PATH + "#fragment",
            tags.PATH.replace("/tags/", "/tags/../tags/"),
            tags.PATH.replace("/tags/", "/tags//"),
            tags.PATH + "%2Fobject",
            tags.PATH + "%00",
            "https://" + tags.HOST + tags.PATH,
            tags.PATH.replace(
                "s3%3A%3A%3A", "s3%3Aus-east-2%3A747336059622%3Abucket%2F"
            ),
        ]
        for path in paths:
            with self.assertRaises(policy.Stop):
                self.allowed(path=path)

    def test_wrong_account_region_or_host_is_held(self):
        for host in [
            tags.HOST.replace(tags.ACCOUNT, "000000000000"),
            tags.HOST.replace(tags.REGION, "us-east-1"),
            tags.HOST + ".",
            "s3-control.us-east-2.amazonaws.com",
        ]:
            with self.assertRaises(policy.Stop):
                self.allowed(host=host)
        headers = fixture_headers()
        headers["x-amz-account-id"] = "000000000000"
        with self.assertRaises(policy.Stop):
            self.allowed(headers=headers)

    def test_missing_duplicate_conflicting_or_unexpected_headers_are_held(self):
        for name in tags.REQUIRED:
            headers = fixture_headers()
            headers.pop(name)
            with self.assertRaises(policy.Stop):
                self.allowed(headers=headers)
        for name in [
            "x-amz-account-id",
            "authorization",
            "x-amz-date",
            "host",
            "content-length",
        ]:
            fields = list(fixture_headers().items()) + [
                (name.upper(), "0" if name == "content-length" else "unexpected")
            ]
            if name == "content-length":
                fields.insert(0, (name, "0"))
            with self.assertRaises(policy.Stop):
                self.allowed(headers=SimpleNamespace(fields=fields))
        for name, value in [
            ("x-amz-target", "DeleteBucket"),
            ("x-amz-expected-bucket-owner", tags.ACCOUNT),
            ("transfer-encoding", "chunked"),
            ("proxy-authorization", "private-canary"),
        ]:
            headers = fixture_headers()
            headers[name] = value
            with self.assertRaises(policy.Stop):
                self.allowed(headers=headers)

    def test_sigv4_region_service_date_and_signed_account_are_bound(self):
        for old, new in [
            ("/us-east-2/", "/us-east-1/"),
            ("/s3/", "/s3control/"),
            ("/20261029/", "/20261028/"),
            ("host;x-amz-account-id;", "host;"),
            ("host;x-amz-account-id;", "x-amz-account-id;host;"),
        ]:
            headers = fixture_headers()
            headers["authorization"] = headers["authorization"].replace(old, new)
            with self.assertRaises(policy.Stop):
                self.allowed(headers=headers)

    def test_optional_header_structures_are_checked_and_retry_metadata_is_not_budget(
        self,
    ):
        for name, value in [
            ("host", tags.HOST + ":443"),
            ("content-length", "00"),
            ("content-length", "1"),
            ("content-type", "application/json"),
            ("x-amz-api-version", "other"),
            ("x-amz-content-sha256", "bad"),
            ("amz-sdk-invocation-id", "bad"),
            ("amz-sdk-request", "attempt=1; attempt=2"),
            ("amz-sdk-request", "action=DeleteBucket"),
            ("accept-encoding", "gzip, deflate"),
            ("x-amz-security-token", ""),
        ]:
            headers = fixture_headers()
            headers[name] = value
            with self.assertRaises(policy.Stop):
                self.allowed(headers=headers)
        headers = fixture_headers()
        headers["amz-sdk-request"] = "attempt=2; max=3"
        headers["content-length"] = "0"
        self.assertEqual(
            self.allowed(headers=headers), ("s3control", "ListTagsForResource")
        )

    def test_header_types_controls_and_bounds_fail_without_value_disclosure(self):
        for value in ["bad\r\nsecret", "é", "x" * 16385, object()]:
            headers = fixture_headers()
            headers["user-agent"] = value
            with self.assertRaises(policy.Stop):
                self.allowed(headers=headers)
        with self.assertRaises(policy.Stop):
            self.allowed(headers=object())

    def test_actual_local_sdk_serialization_has_the_canonical_contract_offline(self):
        from botocore.config import Config
        from botocore.httpsession import URLLib3Session
        from botocore.loaders import Loader
        from botocore.session import Session

        model = Loader().load_service_model("s3control", "service-2")
        operation = model["operations"]["ListTagsForResource"]
        self.assertEqual(
            operation["http"],
            {"method": "GET", "requestUri": "/v20180820/tags/{resourceArn+}"},
        )
        shape = model["shapes"][operation["input"]["shape"]]
        self.assertEqual(set(shape["members"]), {"AccountId", "ResourceArn"})

        class Captured(BaseException):
            pass

        def blocked(*_, **__):
            raise AssertionError("offline-only")

        captures = []
        with (
            patch.object(socket.socket, "connect", blocked),
            patch.object(socket, "getaddrinfo", blocked),
            patch.object(URLLib3Session, "send", blocked),
            patch.object(Session, "get_credentials", blocked),
        ):
            session = Session(
                session_vars={
                    "config_file": (None, None, "/dev/null", None),
                    "credentials_file": (None, None, "/dev/null", None),
                }
            )
            client = session.create_client(
                "s3control",
                region_name=tags.REGION,
                aws_access_key_id="AKIA0000000000000000",
                aws_secret_access_key="offline-only",
                aws_session_token="offline-only-token",
                config=Config(proxies={}, retries={"total_max_attempts": 1}),
            )

            def capture(request, **_):
                self.assertEqual(request.url, "https://" + tags.HOST + tags.PATH)
                captures.append(
                    self.allowed(headers=request.headers, body=request.body or b"")
                )
                raise Captured()

            client.meta.events.register(
                "before-send.s3-control.ListTagsForResource", capture
            )
            try:
                with self.assertRaises(Captured):
                    client.list_tags_for_resource(
                        AccountId=tags.ACCOUNT, ResourceArn=tags.ARN
                    )
            finally:
                client.close()
        self.assertEqual(captures, [("s3control", "ListTagsForResource")])


if __name__ == "__main__":
    unittest.main()
