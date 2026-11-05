"""Offline fixtures: real pinned SDK serialization, no sockets or real cache."""

import base64
import datetime
import hashlib
import importlib.util
import io
import json
import socket
import tempfile
import time
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

from botocore.awsrequest import AWSRequest, AWSResponse
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

spec = importlib.util.spec_from_file_location(
    "observer", Path(__file__).with_name("observe-environment-bindings.py")
)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
CANARY = "offline-private-key-credential-environment-canary"
KEY = bytes(range(32))


class Cache:
    def __init__(self, refresh=False):
        self.identity = "arn:aws:iam::747336059622:user/offline-observer"
        self.original = {
            "clientId": CANARY,
            "refreshToken": CANARY,
            "dpopKey": ec.generate_private_key(ec.SECP256R1())
            .private_bytes(
                serialization.Encoding.PEM,
                serialization.PrivateFormat.PKCS8,
                serialization.NoEncryption(),
            )
            .decode(),
            "accessToken": {
                "accessKeyId": "AKIA0000000000000000",
                "secretAccessKey": CANARY,
                "sessionToken": CANARY,
                "accountId": m.ACCOUNT,
                "expiresAt": (
                    datetime.datetime.now(datetime.UTC)
                    + datetime.timedelta(hours=-1 if refresh else 2)
                ).isoformat(),
            },
        }
        self.saves = 0
        self.closed = False

    def load_token(self, _):
        return self.original

    def save_token(self, _, token):
        self.original = token
        self.saves += 1

    def close(self):
        self.closed = True


class Raw(io.BytesIO):
    def stream(self, *_args, **_kwargs):
        yield self.getvalue()


class Tests(unittest.TestCase):
    def setup_case(self, refresh=False):
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        path = Path(d.name) / "attempts.jsonl"
        b = m.Budget(path)
        self.addCleanup(b.close)
        cache = Cache(refresh)
        sends = []
        response = {
            "FunctionName": m.FUNCTION,
            "FunctionArn": "arn:aws:lambda:us-east-2:"
            + m.ACCOUNT
            + ":function:"
            + m.FUNCTION,
            "Environment": {
                "Variables": {
                    "SST_KEY": base64.b64encode(KEY).decode(),
                    "UNRELATED": CANARY,
                }
            },
        }

        def sender(_session, request):
            rows = [json.loads(x) for x in path.read_text().splitlines()]
            self.assertEqual(rows[-1]["event"], "attempt")
            sends.append(request.url)
            if request.url == m.TARGET:
                body = json.dumps(response).encode()
                ctype = "application/json"
            elif request.url == m.STS + "/":
                body = (
                    '<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>'
                    + cache.identity
                    + "</Arn><Account>"
                    + m.ACCOUNT
                    + "</Account></GetCallerIdentityResult></GetCallerIdentityResponse>"
                ).encode()
                ctype = "text/xml"
            else:
                self.assertEqual(
                    request.url, "https://us-east-2.signin.aws.amazon.com/v1/token"
                )
                body = json.dumps(
                    {
                        "tokenOutput": {
                            "accessToken": {
                                "accessKeyId": "AKIA0000000000000000",
                                "secretAccessKey": CANARY,
                                "sessionToken": CANARY,
                            },
                            "expiresIn": 3600,
                            "refreshToken": CANARY,
                        }
                    }["tokenOutput"]
                ).encode()
                ctype = "application/json"
            return AWSResponse(request.url, 200, {"content-type": ctype}, Raw(body))

        return path, b, cache, sends, response, sender

    def run_case(self, b, cache, sender, checker=None):
        def validate(manifest, key):
            self.assertEqual(bytes(key), KEY)
            self.assertEqual(
                AESGCM(key).decrypt(
                    bytes(12), AESGCM(KEY).encrypt(bytes(12), b"{}", None), None
                ),
                b"{}",
            )
            return {"bindingAuthenticated": True, "packages": [{}] * 4}

        with (
            patch.object(
                m, "IDENTITY_SHA", hashlib.sha256(cache.identity.encode()).hexdigest()
            ),
            patch.object(socket, "socket", side_effect=AssertionError("no-network")),
            patch.object(socket, "getaddrinfo", side_effect=AssertionError("no-dns")),
        ):
            return m.observe(
                b,
                {},
                cache_factory=lambda: cache,
                sender=sender,
                checker=checker or SimpleNamespace(check=validate),
            )

    def assert_private(self, path):
        text = path.read_text()
        for value in [
            CANARY,
            base64.b64encode(KEY).decode(),
            "UNRELATED",
            "Variables",
            "Authorization",
            "https://",
            "FunctionArn",
        ]:
            self.assertNotIn(value, text)

    def test_cached_session_exact_sdk_destination_two_calls_and_validation(self):
        path, b, cache, sends, _response, sender = self.setup_case()
        r = self.run_case(b, cache, sender)
        self.assertTrue(r["allFourAuthenticatedEmptyBindings"])
        self.assertEqual(sends, [m.STS + "/", m.TARGET])
        self.assertEqual(b.count, 298)
        self.assertEqual(cache.saves, 0)
        self.assertTrue(cache.closed)
        self.assert_private(path)

    def test_refresh_sts_and_read_share_budget_third_response_can_be_validated(self):
        path, b, cache, sends, _response, sender = self.setup_case(True)
        r = self.run_case(b, cache, sender)
        self.assertTrue(r["allFourAuthenticatedEmptyBindings"])
        self.assertEqual(b.count, 299)
        self.assertEqual(len(sends), 3)
        self.assertEqual(cache.saves, 1)
        with self.assertRaises(m.Stop):
            b.admit("lambda", "GetFunctionConfiguration", "HTTP/1.1")
        self.assertEqual(len(sends), 3)
        self.assert_private(path)

    def test_model_pins_and_original_proofs_are_local(self):
        with patch.object(socket, "socket", side_effect=AssertionError("no-network")):
            manifest = m.preflight()
        self.assertEqual(len(manifest["packages"]), 4)

    def test_wrong_endpoint_resource_query_alias_method_and_body_never_forward(self):
        for url, method, body in [
            (m.TARGET + "?Qualifier=x", "GET", None),
            (m.TARGET.replace(m.FUNCTION, "other"), "GET", None),
            (m.TARGET.replace("us-east-2", "us-east-1"), "GET", None),
            (m.TARGET + "/", "GET", None),
            (m.TARGET, "PUT", None),
            (m.TARGET, "GET", b"x"),
            (m.TARGET.replace("/functions/", "/functions/%6b"), "GET", None),
        ]:
            path, b, cache, sends, _response, sender = self.setup_case()
            t = m.Transport(b, cache.original, sender)
            t.identity_verified = True
            t.operations = ["sts"]
            with self.assertRaises(m.Stop):
                t.forward(None, AWSRequest(method=method, url=url, data=body).prepare())
            self.assertEqual(sends, [])
            self.assertEqual(b.count, 296)
            self.assert_private(path)

    def test_read_before_identity_and_repeat_read_cannot_forward(self):
        _path, b, cache, sends, _response, sender = self.setup_case()
        t = m.Transport(b, cache.original, sender)
        with self.assertRaises(m.Stop):
            t.forward(None, AWSRequest(method="GET", url=m.TARGET).prepare())
        self.assertEqual(sends, [])
        _path, b, cache, sends, _response, sender = self.setup_case()
        t = m.Transport(b, cache.original, sender)
        t.identity_verified = True
        t.operations = ["sts", "lambda"]
        with self.assertRaises(m.Stop):
            t.forward(None, AWSRequest(method="GET", url=m.TARGET).prepare())
        self.assertEqual(sends, [])

    def test_missing_malformed_key_config_identity_or_env_error_stops_after_one_read(
        self,
    ):
        for mode in ["missing", "malformed", "wrong-function", "env-error"]:
            path, b, cache, sends, response, sender = self.setup_case()
            if mode == "missing":
                response["Environment"]["Variables"].pop("SST_KEY")
            if mode == "malformed":
                response["Environment"]["Variables"]["SST_KEY"] = "!"
            if mode == "wrong-function":
                response["FunctionName"] = "other"
            if mode == "env-error":
                response["Environment"]["Error"] = {"Message": CANARY}
            with self.assertRaises((m.ObservationHeld, ValueError)):
                self.run_case(b, cache, sender)
            self.assertEqual(len(sends), 2)
            self.assertEqual(b.count, 298)
            self.assert_private(path)

    def test_key_and_full_response_are_cleared_after_success_and_failed_authentication(
        self,
    ):
        for fail in [False, True]:
            response = {
                "FunctionName": m.FUNCTION,
                "FunctionArn": "arn:aws:lambda:us-east-2:"
                + m.ACCOUNT
                + ":function:"
                + m.FUNCTION,
                "Environment": {
                    "Variables": {
                        "SST_KEY": base64.b64encode(KEY).decode(),
                        "PRIVATE": CANARY,
                    }
                },
            }
            held = []

            def check(_, key, held=held, fail=fail):
                held.append(key)
                if fail:
                    raise ValueError(CANARY)
                return {"bindingAuthenticated": True, "packages": [{}] * 4}

            if fail:
                with self.assertRaises(m.ObservationHeld):
                    m.validate_response(response, {}, SimpleNamespace(check=check))
            else:
                m.validate_response(response, {}, SimpleNamespace(check=check))
            self.assertEqual(response, {})
            self.assertEqual(held[0], bytearray(32))

    def test_validation_failure_never_fetches_alternate_key_or_repeats(self):
        path, b, cache, sends, _response, sender = self.setup_case()
        with self.assertRaises(m.ObservationHeld):
            self.run_case(
                b,
                cache,
                sender,
                SimpleNamespace(check=Mock(side_effect=ValueError(CANARY))),
            )
        self.assertEqual(sends, [m.STS + "/", m.TARGET])
        self.assert_private(path)

    def test_failed_transports_count_once_and_no_retry_or_read_follows(self):
        for refresh in [True, False]:
            path, b, cache, _sends, _response, _sender = self.setup_case(refresh)
            failed = Mock(side_effect=socket.gaierror(socket.EAI_AGAIN, CANARY))
            with self.assertRaises(m.Stop):
                self.run_case(b, cache, failed)
            self.assertEqual(failed.call_count, 1)
            self.assertEqual(b.count, 297)
            self.assertTrue(b.stopped)
            self.assert_private(path)

    def test_http_redirect_and_failure_stop_before_retry_or_key_validation(self):
        for code in [301, 302, 403, 429, 500, 503]:
            path, b, cache, _sends, _response, _sender = self.setup_case()
            failed = Mock(
                return_value=AWSResponse(
                    m.STS + "/", code, {"location": m.TARGET}, Raw(b"")
                )
            )
            with self.assertRaises(m.Stop):
                self.run_case(b, cache, failed)
            self.assertEqual(failed.call_count, 1)
            self.assertEqual(b.count, 297)
            self.assert_private(path)

    def test_identity_mismatch_prohibits_lambda(self):
        _path, b, cache, sends, _response, sender = self.setup_case()

        def wrong(session, request):
            r = sender(session, request)
            return AWSResponse(
                request.url,
                200,
                {"content-type": "text/xml"},
                Raw(
                    r.content.replace(
                        cache.identity.encode(), b"arn:aws:iam::747336059622:user/other"
                    )
                ),
            )

        with self.assertRaises(m.Stop):
            self.run_case(b, cache, wrong)
        self.assertEqual(len(sends), 1)
        self.assertEqual(b.count, 297)

    def test_original_deadline_and_exhaustion_are_shared_across_phases(self):
        _path, b, cache, sends, _response, sender = self.setup_case()
        b.count = 299
        with self.assertRaises(m.Stop):
            self.run_case(b, cache, sender)
        self.assertEqual(sends, [])
        _path, b, cache, sends, _response, sender = self.setup_case()

        def expire(session, request):
            r = sender(session, request)
            b.deadline = time.monotonic() - 1
            b.started = b.deadline - 600
            return r

        with self.assertRaises(m.Stop):
            self.run_case(b, cache, expire)
        self.assertEqual(len(sends), 1)
        self.assertEqual(b.count, 297)

    def test_ledger_failure_holds_before_network(self):
        _path, b, cache, sends, _response, sender = self.setup_case()
        with (
            patch.object(b, "record", side_effect=OSError(CANARY)),
            self.assertRaises(OSError),
        ):
            self.run_case(b, cache, sender)
        self.assertEqual(sends, [])

    def test_hard_deadline_interrupts_a_blocking_transport_once(self):
        path, b, cache, _sends, _response, _sender = self.setup_case()
        b.deadline = time.monotonic() + 0.3
        blocked = Mock(side_effect=lambda *_: time.sleep(1.0))
        with self.assertRaises(m.Stop):
            self.run_case(b, cache, blocked)
        self.assertEqual(blocked.call_count, 1)
        self.assertEqual(b.count, 297)
        self.assertTrue(b.stopped)
        self.assert_private(path)

    def test_changed_existing_identity_is_rejected_before_any_attempt(self):
        _path, b, cache, _sends, _response, sender = self.setup_case()
        with patch.object(m, "IDENTITY_SHA", "0" * 64), self.assertRaises(m.Stop):
            m.observe(b, {}, cache_factory=lambda: cache, sender=sender)
        self.assertEqual(b.count, 296)
        self.assertTrue(cache.closed)

    def test_physical_transport_disables_pool_retries_and_redirects(self):
        conn = Mock()
        conn.urlopen.return_value = SimpleNamespace(
            status=200, headers={}, stream=lambda *a, **k: [b"{}"]
        )
        manager = Mock()
        manager.connection_from_url.return_value = conn
        physical = m.URLLib3Session(verify=True, proxies={})
        try:
            with patch.object(
                physical, "_get_connection_manager", return_value=manager
            ):
                physical.send(AWSRequest(method="GET", url=m.TARGET).prepare())
            self.assertEqual(conn.urlopen.call_count, 1)
            options = conn.urlopen.call_args.kwargs
            self.assertIs(options["retries"].total, False)
            self.assertEqual(options["retries"].redirect, 0)
            self.assertEqual(conn.cert_reqs, "CERT_REQUIRED")
        finally:
            physical.close()

    def test_exclusive_claim_and_output_records_cannot_be_reused_or_reset(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "window-from-296.json"
            m.persist(p, {"prior": 296})
            before = p.read_bytes()
            with self.assertRaises(FileExistsError):
                m.persist(p, {"prior": 0})
            self.assertEqual(before, p.read_bytes())
            m.Budget(Path(d) / "ledger").close()
            with self.assertRaises(FileExistsError):
                m.Budget(Path(d) / "ledger")


if __name__ == "__main__":
    unittest.main()
