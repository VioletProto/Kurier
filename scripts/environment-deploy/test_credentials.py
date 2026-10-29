"""Pinned SDK, synthetic cache and mocked physical sends; sockets are forbidden."""

import datetime
import hashlib
import io
import json
import os
import socket
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

from botocore.awsrequest import AWSResponse
from botocore.credentials import LoginCredentialFetcher
from botocore.session import Session
from common import PRIOR, ROOT, Campaign, Held
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec
from driver import credentials, execute_phase
from phase_credentials import AuthBudget, cache_stat

SCOPE = json.loads(
    (ROOT / "docs/development/project-environments-execution-scope.json").read_bytes()
)
CANARY = "synthetic-private-renewal-canary"


class Raw(io.BytesIO):
    def stream(self, *_args, **_kwargs):
        yield self.getvalue()


class Cache:
    def __init__(self, fd):
        self.fd = fd
        self.identity = SCOPE["identityArn"]
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
                "accountId": SCOPE["account"],
            },
        }
        self.expire_in(900)
        self.saves = 0

    def expire_in(self, seconds):
        self.original["accessToken"]["expiresAt"] = (
            datetime.datetime.now(datetime.UTC) + datetime.timedelta(seconds=seconds)
        ).isoformat()

    def load_token(self, session):
        assert session == self.identity
        return self.original

    def save_token(self, session, token):
        assert session == self.identity
        self.original = token
        self.saves += 1
        os.write(self.fd, b"synthetic-rotation")

    def close(self):
        pass  # The fixture owns this synthetic descriptor.


class CredentialTests(unittest.TestCase):
    def setup_case(self, expiry=900, maximum=1799):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        cachefile = self.enterContext(tempfile.TemporaryFile())
        self.cache = Cache(cachefile.fileno())
        self.cache.expire_in(expiry)
        self.now = [time.monotonic()]
        self.c = Campaign(
            Path(directory.name) / "campaign",
            create={
                "prior": PRIOR,
                "maximum": maximum,
                "seconds": 3300,
                "manifestSha256": "a" * 64,
                "scopeSha256": "b" * 64,
                "reviewCommit": "c" * 40,
                "authorizationSha256": "d" * 64,
            },
            clock=lambda: self.now[0],
        )
        self.addCleanup(self.c.close)
        self.c.put("cacheStat", cache_stat(self.cache))
        self.c.begin("collect")
        self.sends = []
        self.status = 200
        self.identity = self.cache.identity
        self.account = SCOPE["account"]
        self.issued_seconds = 900
        self.send_error = None
        self.send_delay = 0

    def sender(self, _session, request):
        # Each mock transport observes its durable admission before forwarding.
        row = self.c.db.execute(
            "SELECT n,op,outcome FROM attempts ORDER BY n DESC LIMIT 1"
        ).fetchone()
        self.assertEqual(row[2], 0)
        self.sends.append(row)
        if self.send_delay:
            time.sleep(self.send_delay)
        if self.send_error:
            raise self.send_error
        if request.url.endswith("/v1/token"):
            body = json.dumps(
                {
                    "accessToken": {
                        "accessKeyId": "AKIA0000000000000000",
                        "secretAccessKey": CANARY,
                        "sessionToken": CANARY,
                    },
                    "expiresIn": self.issued_seconds,
                    "refreshToken": CANARY + "-rotated",
                }
            ).encode()
            kind = "application/json"
        else:
            self.assertEqual(request.url, "https://sts.us-east-2.amazonaws.com/")
            body = (
                '<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">'
                "<GetCallerIdentityResult><Arn>"
                + self.identity
                + "</Arn><Account>"
                + self.account
                + "</Account></GetCallerIdentityResult></GetCallerIdentityResponse>"
            ).encode()
            kind = "text/xml"
        return AWSResponse(request.url, self.status, {"content-type": kind}, Raw(body))

    def authenticate(self):
        with (
            patch("phase_credentials.ExistingCache", return_value=self.cache),
            patch("phase_credentials.URLLib3Session.send", self.sender),
            patch.object(socket, "socket", side_effect=AssertionError("no sockets")),
            patch.object(socket, "getaddrinfo", side_effect=AssertionError("no DNS")),
        ):
            return credentials(self.c, SCOPE)

    def test_valid_fifteen_minute_credentials_cover_bounded_phase_without_refresh(self):
        self.setup_case()
        result = self.authenticate()
        self.assertEqual(result["AWS_SECRET_ACCESS_KEY"], CANARY)
        self.assertEqual(self.cache.saves, 0)
        self.assertEqual(self.c.get("count"), 302)
        self.assertEqual(self.sends[0][1], "sts/GetCallerIdentity")

    def test_pinned_model_has_no_duration_request_and_caps_lifetime_at_900(self):
        operation = (
            Session().get_service_model("signin").operation_model("CreateOAuth2Token")
        )
        self.assertEqual(
            set(operation.input_shape.members["tokenInput"].members),
            {
                "clientId",
                "grantType",
                "code",
                "redirectUri",
                "codeVerifier",
                "refreshToken",
            },
        )
        self.assertIn(
            "maximum 900",
            operation.output_shape.members["tokenOutput"]
            .members["expiresIn"]
            .documentation,
        )
        self.assertEqual(LoginCredentialFetcher._REFRESH_THRESHOLD, 300)

    def test_real_locked_cache_rotates_in_place_with_same_session_binding(self):
        self.setup_case(expiry=-1)
        home = Path(self.enterContext(tempfile.TemporaryDirectory()))
        folder = home / ".aws/login/cache"
        folder.mkdir(parents=True)
        (home / ".aws/config").write_text(
            "[default]\nlogin_session = " + self.cache.identity + "\n"
        )
        path = folder / (
            hashlib.sha256(self.cache.identity.encode()).hexdigest() + ".json"
        )
        path.write_text(json.dumps(self.cache.original))
        os.chmod(path, 0o600)
        before = path.stat().st_ino
        client_id = self.cache.original["clientId"]
        key = self.cache.original["dpopKey"]
        from authentication import ExistingCache

        with patch("authentication.Path.home", return_value=home):
            self.cache = ExistingCache()
        self.c.put("cacheStat", cache_stat(self.cache))
        self.authenticate()
        after = json.loads(path.read_bytes())
        self.assertEqual(path.stat().st_ino, before)
        self.assertEqual(after["clientId"], client_id)
        self.assertEqual(after["dpopKey"], key)
        self.assertEqual(after["refreshToken"], CANARY + "-rotated")
        self.assertEqual(after["accessToken"]["accountId"], SCOPE["account"])

    def test_expired_and_five_minute_tokens_renew_once_using_pinned_sdk(self):
        self.setup_case(expiry=-1)
        self.authenticate()
        self.assertEqual(self.cache.saves, 1)
        self.assertEqual(self.c.get("count"), 303)
        self.assertEqual(
            [row[1] for row in self.sends],
            ["signin/CreateOAuth2Token", "sts/GetCallerIdentity"],
        )
        self.assertEqual(self.c.get("cacheStat"), cache_stat(self.cache))
        self.assertEqual(self.cache.original["refreshToken"], CANARY + "-rotated")

    def test_sdk_threshold_reuses_above_five_minutes_and_renews_at_threshold(self):
        self.setup_case(expiry=301)
        self.authenticate()
        self.assertEqual(self.cache.saves, 0)
        self.c.finish("collect", {})
        self.now[0] += 1
        self.c.begin("close")
        self.cache.expire_in(300)
        self.authenticate()
        self.assertEqual(self.cache.saves, 1)
        self.assertEqual(self.c.get("count"), 304)

    def test_short_renewal_is_rejected_before_any_phase_work(self):
        self.setup_case(expiry=-1)
        self.issued_seconds = 120
        with self.assertRaises(Held):
            self.authenticate()
        self.assertTrue(self.c.get("stopped"))
        self.assertIsNone(self.c.get("credentialPhase"))
        self.assertEqual(len(self.sends), 2)

    def test_renewal_failure_latches_and_cannot_retry(self):
        self.setup_case(expiry=-1)
        self.status = 403
        with self.assertRaises(Held):
            self.authenticate()
        self.assertEqual(self.c.get("count"), 302)
        self.assertEqual(len(self.sends), 1)
        self.assertEqual(self.cache.saves, 0)
        with self.assertRaises(Held):
            self.authenticate()
        self.assertEqual(len(self.sends), 1)

    def test_uncertain_refresh_consumes_attempt_and_never_replays(self):
        self.setup_case(expiry=-1)
        self.send_error = OSError(CANARY)
        with self.assertRaises(Held):
            self.authenticate()
        self.assertEqual(self.c.get("count"), 302)
        self.assertEqual(
            self.c.db.execute("SELECT status,outcome FROM attempts").fetchone(),
            (None, 1),
        )
        self.assertTrue(self.c.get("stopped"))

    def test_wrong_sts_identity_and_account_stop_after_metered_observation(self):
        self.setup_case()
        self.identity = "arn:aws:iam::747336059622:root"
        with self.assertRaises(Held):
            self.authenticate()
        self.assertEqual(len(self.sends), 1)
        self.assertTrue(self.c.get("stopped"))

    def test_wrong_sts_account_is_not_accepted(self):
        self.setup_case()
        self.account = "000000000000"
        with self.assertRaises(Held):
            self.authenticate()
        self.assertTrue(self.c.get("stopped"))

    def test_replaced_cache_or_changed_session_stops_before_authentication(self):
        self.setup_case()
        self.cache.identity = "arn:aws:iam::747336059622:user/other"
        with self.assertRaises(Held):
            self.authenticate()
        self.assertFalse(self.sends)

    def test_external_cache_change_is_not_silently_adopted(self):
        self.setup_case()
        os.write(self.cache.fd, b"outside-change")
        with self.assertRaises(Held):
            self.authenticate()
        self.assertFalse(self.sends)

    def test_refresh_and_sts_cannot_exceed_shared_budget(self):
        self.setup_case(expiry=-1, maximum=302)
        with self.assertRaises(Held):
            self.authenticate()
        self.assertEqual(self.c.get("count"), 302)
        self.assertEqual(len(self.sends), 1)
        self.assertTrue(self.c.get("stopped"))

    def test_six_boundaries_share_original_and_closed_deadlines(self):
        self.setup_case(expiry=-1)
        original = self.c.get("deadline")
        closed = None
        for phase in ["collect", "close", "deploy", "verify", "activate", "final"]:
            if phase != "collect":
                self.now[0] += 10
                self.c.begin(phase)
            if phase == "close":
                closed = self.c.get("closedDeadline")
            self.cache.expire_in(-1)
            phase_deadline = self.c.get("phaseDeadline")
            self.authenticate()
            self.assertEqual(self.c.get("deadline"), original)
            self.assertEqual(self.c.get("phaseDeadline"), phase_deadline)
            if phase in {"close", "deploy", "verify", "activate"}:
                self.assertEqual(self.c.get("closedDeadline"), closed)
            self.c.finish(phase, {})
        self.assertEqual(self.c.get("count"), 313)
        self.assertEqual(self.cache.saves, 6)

    def test_same_boundary_cannot_authenticate_twice(self):
        self.setup_case(expiry=-1)
        self.authenticate()
        with self.assertRaises(Held):
            self.authenticate()
        self.assertEqual(len(self.sends), 2)

    def test_transport_renewal_cap_cannot_be_bypassed_with_another_transport(self):
        self.setup_case(expiry=-1)
        self.authenticate()
        with self.assertRaises(Held):
            AuthBudget(self.c).admit("signin", "CreateOAuth2Token", "HTTP/1.1")
        self.assertEqual(self.c.get("count"), 303)

    def test_campaign_renewal_cap_denies_seventh_before_admission(self):
        self.setup_case()
        for i in range(6):
            self.c.db.execute(
                "INSERT INTO attempts VALUES(?,?,?,1,200,1)",
                (i + 1, "old-fixture-phase", "signin/CreateOAuth2Token"),
            )
        with self.assertRaises(Held):
            AuthBudget(self.c).admit("signin", "CreateOAuth2Token", "HTTP/1.1")
        self.assertEqual(self.c.get("count"), 301)

    def test_phase_expiry_stops_before_renewal_and_later_forwarding(self):
        self.setup_case(expiry=-1)
        self.now[0] = self.c.get("phaseDeadline")
        with self.assertRaises(Held):
            self.authenticate()
        self.assertFalse(self.sends)
        with self.assertRaises(Held):
            self.c.admit("fixture/write", True)

    def test_closed_deadline_cannot_be_extended_by_renewal(self):
        self.setup_case()
        self.c.finish("collect", {})
        self.c.begin("close")
        self.c.put("closedDeadline", self.now[0] + 2)
        self.c.put("phaseDeadline", self.now[0] + 2)
        self.now[0] += 2
        with self.assertRaises(Held):
            self.authenticate()
        self.assertFalse(self.sends)

    def test_deadline_alarm_interrupts_blocked_authentication_without_reset(self):
        self.setup_case(expiry=-1)
        self.c.put("phaseDeadline", time.monotonic() + 0.1)
        deadline = self.c.get("deadline")
        self.send_delay = 1
        with self.assertRaises(Held):
            self.authenticate()
        self.assertTrue(self.c.get("stopped"))
        self.assertEqual(self.c.get("deadline"), deadline)
        self.assertEqual(len(self.sends), 1)

    def test_phase_approval_is_checked_before_any_renewal(self):
        self.setup_case()
        self.c.finish("collect", {"sha256": "a" * 64})
        with (
            patch("driver.credentials") as authenticate,
            patch("driver.prepare", return_value={"authorized": False}),
            self.assertRaises(Held),
        ):
            execute_phase(self.c, "deploy", {"authorized": False}, SCOPE)
        authenticate.assert_not_called()

    def test_no_authentication_values_in_durable_events_or_attempts(self):
        self.setup_case(expiry=-1)
        self.authenticate()
        rows = list(self.c.db.execute("SELECT data FROM events"))
        self.assertNotIn(CANARY, json.dumps(rows))
        self.assertNotIn(
            CANARY, json.dumps(list(self.c.db.execute("SELECT * FROM attempts")))
        )


if __name__ == "__main__":
    unittest.main()
