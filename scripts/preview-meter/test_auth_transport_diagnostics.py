"""Offline typed failures through the pinned physical SDK transport, no sockets."""

import errno
import json
import socket
import ssl
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import authentication as auth
import handoff as h
from accounting import AUTH_SHA, CLAIM_SHA, FROZEN_FILES, PRIOR, baseline
from auth_diagnostics import DeadlineExpired, transport_diagnostic
from botocore import exceptions as sdk
from botocore.awsrequest import AWSRequest
from botocore.httpsession import URLLib3Session
from policy import Stop
from run import ROOT
from urllib3 import exceptions as pool

CANARY = "private-canary-header-body-credential-url"
TOKEN = {"clientId": CANARY, "refreshToken": CANARY}
FIELDS = {"exceptionCategory", "failurePhase", "errorCode", "errorCodeDomain"}


def failures():
    dns = socket.gaierror(socket.EAI_AGAIN, CANARY)
    refused = pool.NewConnectionError(None, CANARY)
    refused.__cause__ = ConnectionRefusedError(errno.ECONNREFUSED, CANARY)
    unreachable = pool.NewConnectionError(None, CANARY)
    unreachable.__cause__ = OSError(errno.ENETUNREACH, CANARY)
    certificate = ssl.SSLCertVerificationError(ssl.SSL_ERROR_SSL, CANARY)
    certificate.verify_code = 20
    tls = pool.SSLError(certificate)
    return [
        (dns, "dns", "dns", socket.EAI_AGAIN, "gai"),
        (refused, "connection", "connect", errno.ECONNREFUSED, "errno"),
        (unreachable, "connection", "connect", errno.ENETUNREACH, "errno"),
        (tls, "tls-certificate", "tls", 20, "x509"),
        (pool.SSLError(CANARY), "tls", "tls", None, None),
        (pool.ConnectTimeoutError(CANARY), "connect-timeout", "connect", None, None),
        (
            pool.ReadTimeoutError(None, CANARY, CANARY),
            "read-timeout",
            "read",
            None,
            None,
        ),
        (
            TimeoutError(errno.ETIMEDOUT, CANARY),
            "timeout",
            "unknown",
            errno.ETIMEDOUT,
            "errno",
        ),
        (RuntimeError(CANARY), "transport", "unknown", None, None),
        (DeadlineExpired(), "deadline", "unknown", None, None),
    ]


class Tests(unittest.TestCase):
    def test_physical_sdk_failures_are_sanitized_charged_once_and_never_handed_off(
        self,
    ):
        auth.sdk_pins()
        for error, category, phase, code, domain in failures():
            with (
                self.subTest(category=category, phase=phase),
                tempfile.TemporaryDirectory() as tmp,
            ):
                d = Path(tmp) / "window"
                w = h.begin(d, ROOT)
                b = h.SharedBudget(d / "authentication.jsonl", w, prior=PRIOR)
                sends = []

                def urlopen(*, sends=sends, d=d, error=error, **kwargs):
                    sends.append(1)
                    # Assert the durable receipt exists before even the fake send.
                    rows = [
                        json.loads(x)
                        for x in (d / "authentication.jsonl").read_text().splitlines()
                    ]
                    self.assertEqual(rows[-1]["event"], "attempt")
                    self.assertEqual(rows[-1]["count"], PRIOR + 1)
                    self.assertEqual(kwargs["retries"].total, False)
                    raise error

                conn = SimpleNamespace(urlopen=urlopen)
                manager = SimpleNamespace(connection_from_url=lambda _, conn=conn: conn)
                session = URLLib3Session(proxies={})
                req = AWSRequest(
                    method="POST",
                    url=auth.SIGNIN + "/v1/token",
                    data=json.dumps(
                        {
                            **TOKEN,
                            "grantType": "refresh_token",
                        }
                    ),
                ).prepare()
                t = auth.Transport(b, TOKEN, URLLib3Session.send)
                try:
                    with (
                        patch.object(
                            session, "_get_connection_manager", return_value=manager
                        ),
                        patch.object(session, "_setup_ssl_cert"),
                        patch(
                            "socket.socket", side_effect=AssertionError("no-network")
                        ),
                        patch(
                            "socket.getaddrinfo", side_effect=AssertionError("no-dns")
                        ),
                    ):
                        with self.assertRaises(Stop):
                            t.forward(session, req)
                        with self.assertRaises(Stop):
                            t.forward(session, req)
                    self.assertEqual(len(sends), 1)
                    self.assertEqual(b.count, PRIOR + 1)
                    self.assertEqual(h.usage(d)["newCalls"], 1)
                    self.assertEqual(h.usage(d)["cumulativeCalls"], PRIOR + 1)
                    rows = [
                        json.loads(x)
                        for x in (d / "authentication.jsonl").read_text().splitlines()
                    ]
                    diag = [
                        x
                        for x in rows
                        if x["event"] == "authentication-transport-error"
                    ]
                    self.assertEqual(len(diag), 1)
                    self.assertEqual(
                        set(diag[0]), FIELDS | {"event", "at", "monotonic", "attempt"}
                    )
                    self.assertEqual(
                        {k: diag[0][k] for k in FIELDS},
                        {
                            "exceptionCategory": category,
                            "failurePhase": phase,
                            "errorCode": code,
                            "errorCodeDomain": domain,
                        },
                    )
                    self.assertEqual(sum(x["event"] == "attempt" for x in rows), 1)
                    outcome = next(x for x in rows if x["event"] == "outcome")
                    self.assertTrue(outcome["transportError"])
                    self.assertIsNone(outcome["status"])
                    self.assertEqual(rows[-1]["event"], "stop")
                    raw = (d / "authentication.jsonl").read_text()
                    for forbidden in [
                        CANARY,
                        "https://",
                        "/v1/token",
                        "endpoint_url",
                        "verify_message",
                    ]:
                        self.assertNotIn(forbidden, raw)
                    self.assertFalse(any(x["event"] == "authenticated" for x in rows))
                    with self.assertRaises(ValueError):
                        h.seal(d, ROOT)
                    self.assertFalse((d / "handoff.json").exists())
                finally:
                    b.close()
                    session.close()

    def test_diagnostic_never_stringifies_unknown_errors_or_records_extra_fields(self):
        class Unprintable(Exception):
            def __str__(self):
                raise AssertionError("must-not-stringify")

            def __repr__(self):
                raise AssertionError("must-not-repr")

        error = Unprintable(CANARY)
        error.url = CANARY
        error.headers = {CANARY: CANARY}
        error.body = CANARY
        error.errno = 123456789
        error.kwargs = {"error": CANARY, "endpoint_url": CANARY}
        value = transport_diagnostic(error)
        self.assertEqual(set(value), FIELDS)
        self.assertEqual(
            value,
            {
                "exceptionCategory": "unknown",
                "failurePhase": "unknown",
                "errorCode": None,
                "errorCodeDomain": None,
            },
        )
        self.assertLess(len(json.dumps(value)), 180)

    def test_only_allowlisted_numeric_codes_are_retained(self):
        for bad in [True, 123456789, -123456789, CANARY, None]:
            error = ssl.SSLCertVerificationError(ssl.SSL_ERROR_SSL, CANARY)
            error.verify_code = bad
            value = transport_diagnostic(error)
            self.assertIsNone(value["errorCode"])
            self.assertIsNone(value["errorCodeDomain"])
            self.assertNotIn(CANARY, json.dumps(value))
        self.assertIsNone(
            transport_diagnostic(socket.gaierror(98765, CANARY))["errorCode"]
        )
        self.assertIsNone(transport_diagnostic(OSError(98765, CANARY))["errorCode"])

    def test_wrapped_dns_cause_and_cycles_are_bounded(self):
        dns = socket.gaierror(socket.EAI_NONAME, CANARY)
        reason = pool.NameResolutionError(CANARY, None, dns)
        outer = sdk.EndpointConnectionError(endpoint_url=CANARY, error=reason)
        reason.__cause__ = outer
        result = transport_diagnostic(outer)
        self.assertEqual(result["exceptionCategory"], "dns")
        self.assertEqual(result["errorCode"], socket.EAI_NONAME)
        self.assertNotIn(CANARY, json.dumps(result))

    def test_missing_numeric_tls_code_and_generic_timeout_do_not_invent_phase(self):
        tls = transport_diagnostic(
            ssl.SSLCertVerificationError(ssl.SSL_ERROR_SSL, CANARY)
        )
        self.assertEqual(tls["failurePhase"], "tls")
        self.assertIsNone(tls["errorCode"])
        self.assertEqual(
            transport_diagnostic(TimeoutError(CANARY))["failurePhase"], "unknown"
        )

    def test_diagnostic_persistence_failure_cannot_retry_or_handoff(self):
        with tempfile.TemporaryDirectory() as tmp:
            d = Path(tmp) / "window"
            w = h.begin(d, ROOT)
            b = h.SharedBudget(d / "authentication.jsonl", w, prior=PRIOR)
            original = b.record

            def record(**data):
                if data["event"] == "authentication-transport-error":
                    raise OSError("disk-full")
                original(**data)

            send = Mock(side_effect=socket.gaierror(socket.EAI_AGAIN, CANARY))
            t = auth.Transport(b, TOKEN, send)
            req = SimpleNamespace(
                method="POST",
                url=auth.SIGNIN + "/v1/token",
                body=json.dumps({**TOKEN, "grantType": "refresh_token"}),
            )
            try:
                with patch.object(b, "record", record):
                    with self.assertRaises(Stop):
                        t.forward(None, req)
                    with self.assertRaises(Stop):
                        t.forward(None, req)
                self.assertEqual(send.call_count, 1)
                self.assertEqual(h.usage(d)["cumulativeCalls"], PRIOR + 1)
                with self.assertRaises(ValueError):
                    h.seal(d, ROOT)
            finally:
                b.close()

    def test_frozen_failed_attempt_and_claim_are_required_and_unchanged(self):
        current = baseline(ROOT)
        self.assertEqual(current["prior"], 166)
        self.assertEqual(current["maximum"], 500)
        self.assertEqual(current["failedAuthenticationLedgerSha256"], AUTH_SHA)
        self.assertEqual(current["failedCampaignClaimSha256"], CLAIM_SHA)
        self.assertEqual(PRIOR, 166)
        import shutil

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for name in FROZEN_FILES:
                dest = root / name
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / name, dest)
            self.assertEqual(baseline(root), current)
            for name in FROZEN_FILES:
                with self.subTest(proof=name):
                    failed = root / name
                    original = failed.read_bytes()
                    failed.write_bytes(original + b"\n")
                    with self.assertRaises(ValueError):
                        baseline(root)
                    failed.write_bytes(original)
            self.assertEqual(baseline(root), current)
        self.assertEqual(baseline(ROOT), current)


if __name__ == "__main__":
    unittest.main()
