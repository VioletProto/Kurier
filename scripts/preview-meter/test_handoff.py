"""Offline shared-window, physical authentication transport and handoff tests."""

import base64
import copy
import hashlib
import io
import json
import shutil
import tempfile
import time
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import authentication as auth
import handoff as h
from accounting import FROZEN_FILES
from policy import Stop
from run import ROOT, launch

TOKEN = {
    "clientId": "local-client",
    "refreshToken": "local-private-canary",
    "dpopKey": "unused-until-sdk-test",
    "accessToken": {"accountId": auth.ACCOUNT},
}


def request(service="signin"):
    return SimpleNamespace(
        method="POST",
        url=auth.SIGNIN + "/v1/token" if service == "signin" else auth.STS + "/",
        body=json.dumps(
            {
                "clientId": TOKEN["clientId"],
                "refreshToken": TOKEN["refreshToken"],
                "grantType": "refresh_token",
            }
        ).encode()
        if service == "signin"
        else b"Action=GetCallerIdentity&Version=2011-06-15",
    )


class Tests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.directory = Path(self.tmp.name) / "window"
        self.now = [100.0]
        self.clock = lambda: self.now[0]
        self.window = h.begin(self.directory, ROOT, clock=self.clock)

    def budget(self, name="authentication.jsonl", prior=167):
        b = h.SharedBudget(
            self.directory / name, self.window, prior=prior, clock=self.clock
        )
        self.addCleanup(b.close)
        return b

    def complete(self):
        b = self.budget()
        for service, operation in [
            ("signin", "CreateOAuth2Token"),
            ("sts", "GetCallerIdentity"),
        ]:
            n = b.admit(service, operation, "HTTP/1.1")
            b.outcome(n, 200)
        b.record(event="authenticated", identityVerified=True, count=b.count)
        return h.seal(self.directory, ROOT, clock=self.clock)

    def rewrite_ledger(self, change):
        p = self.directory / "authentication.jsonl"
        rows = [json.loads(x) for x in p.read_text().splitlines()]
        change(rows)
        p.write_text("".join(json.dumps(x) + "\n" for x in rows))

    def test_cross_phase_exhaustion_and_first_provider_ordinal(self):
        receipt = self.complete()
        self.now[0] += 10
        w, prior = h.consume(receipt, ROOT, clock=self.clock)
        self.assertEqual(prior, 169)
        b = h.SharedBudget(
            self.directory / "provider.jsonl", w, prior=prior, clock=self.clock
        )
        self.addCleanup(b.close)
        self.assertEqual(b.admit("sts", "GetCallerIdentity", "HTTP/2"), 170)
        b.outcome(170, 200)
        for _ in range(330):
            n = b.admit("s3", "GET", "HTTP/2")
            if n == 500:
                with self.assertRaisesRegex(Stop, "call-limit"):
                    b.outcome(n, 200)
            else:
                b.outcome(n, 200)
        self.assertEqual(b.count, 500)
        with self.assertRaises(Stop):
            b.admit("s3", "GET", "HTTP/2")
        rows = [
            json.loads(x)
            for x in (self.directory / "provider.jsonl").read_text().splitlines()
        ]
        self.assertEqual(sum(r["event"] == "attempt" for r in rows), 331)

    def test_deadline_is_original_not_new_provider_window(self):
        receipt = self.complete()
        self.now[0] = 3699
        w, prior = h.consume(receipt, ROOT, clock=self.clock)
        b = h.SharedBudget(
            self.directory / "provider.jsonl", w, prior=prior, clock=self.clock
        )
        self.addCleanup(b.close)
        self.assertEqual(b.started, 100)
        self.now[0] = 3700
        with self.assertRaisesRegex(Stop, "time-limit"):
            b.admit("s3", "GET", "HTTP/1.1")

    def test_reused_handoff_claim_and_window_cannot_restart(self):
        receipt = self.complete()
        h.consume(receipt, ROOT, clock=self.clock)
        with self.assertRaisesRegex(ValueError, "reused"):
            h.consume(receipt, ROOT, clock=self.clock)
        with self.assertRaises(FileExistsError):
            h.begin(self.directory, ROOT, clock=self.clock)
        with self.assertRaises(FileExistsError):
            self.budget()

    def test_new_output_cannot_reset_same_campaign_baseline(self):
        root = self.directory.parent / "fixture-root"
        docs = root / "docs/development"
        docs.mkdir(parents=True)
        (root / ".build/project-environments-preview").mkdir(parents=True)
        for name in FROZEN_FILES:
            dest = root / name
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, dest)
        h.claim_campaign(root, root / "output-1")
        with self.assertRaises(FileExistsError):
            h.claim_campaign(root, root / "output-2")

    def test_copied_handoff_cannot_move_or_reset_window(self):
        receipt = self.complete()
        dest = self.directory.parent / "copy"
        shutil.copytree(self.directory, dest)
        with self.assertRaises(ValueError):
            h.validate(dest / receipt.name, ROOT, clock=self.clock)

    def test_window_field_resets_rejected(self):
        receipt = self.complete()
        p = self.directory / "window.json"
        original = p.read_text()
        for key, value in [
            ("prior", 0),
            ("maximum", 501),
            ("seconds", 3601),
            ("deadline", 3800),
            ("bootId", "different-boot"),
            ("started", float("nan")),
            ("source", "wrong"),
        ]:
            with self.subTest(key=key):
                data = json.loads(original)
                data[key] = value
                p.write_text(json.dumps(data))
                with self.assertRaises(ValueError):
                    h.validate(receipt, ROOT, clock=self.clock)
        p.write_text(original)
        self.now[0] = 3700
        with self.assertRaises(ValueError):
            h.validate(receipt, ROOT, clock=self.clock)

    def test_handoff_count_or_hash_reset_rejected(self):
        receipt = self.complete()
        original = receipt.read_text()
        for key, value in [
            ("prior", 167),
            ("authenticationSha256", "0" * 64),
            ("windowSha256", "0" * 64),
        ]:
            with self.subTest(key=key):
                data = json.loads(original)
                data[key] = value
                receipt.write_text(json.dumps(data))
                with self.assertRaises(ValueError):
                    h.validate(receipt, ROOT, clock=self.clock)
        receipt.write_text(original)

    def test_ledger_missing_duplicate_sequence_failure_and_unknown_rows(self):
        self.complete()
        p = self.directory / "authentication.jsonl"
        original = p.read_text()
        changes = [
            lambda r: r.pop(3),
            lambda r: r.insert(4, r[3]),
            lambda r: r[2].update(count=170),
            lambda r: r[3].update(status=403),
            lambda r: r[3].update(transportError=True),
            lambda r: r[2].update(operation="CreateUser"),
            lambda r: r.insert(-1, {"event": "stop", "monotonic": 100}),
            lambda r: r[-1].update(identityVerified=False),
            lambda r: r[-1].update(count=167),
            lambda r: r[2].update(monotonic=99),
            lambda r: r.pop(),
        ]
        for i, change in enumerate(changes):
            with self.subTest(case=i):
                p.write_text(original)
                self.rewrite_ledger(change)
                with self.assertRaises(ValueError):
                    h.authentication_count(self.directory, self.window)

    def test_pending_authentication_never_seals(self):
        b = self.budget()
        b.admit("signin", "CreateOAuth2Token", "HTTP/1.1")
        with self.assertRaises(ValueError):
            h.seal(self.directory, ROOT, clock=self.clock)

    def test_durable_write_failure_prevents_authentication_forwarding(self):
        b = self.budget()
        with (
            patch.object(b, "record", side_effect=OSError("disk-full")),
            patch.object(auth.Transport, "send", create=True) as send,
        ):
            t = auth.Transport(b, TOKEN, send)
            with self.assertRaises((Stop, OSError)):
                t.forward(None, request())
            send.assert_not_called()

    def test_terminal_usage_preserves_failed_attempt_and_flags_truncation(self):
        b = self.budget()
        b.admit("signin", "CreateOAuth2Token", "HTTP/1.1")
        self.assertEqual(h.usage(self.directory)["cumulativeCalls"], 168)
        with open(self.directory / "authentication.jsonl", "a") as f:
            f.write('{"event":')
        self.assertFalse(h.usage(self.directory)["ledgerIntegrity"])
        self.assertIsNone(h.usage(self.directory)["cumulativeCalls"])

    def test_deadline_alarm_interrupts_synchronous_authentication(self):
        with (
            self.assertRaises(auth.DeadlineExpired),
            auth.deadline_alarm(time.monotonic() + 0.01),
        ):
            time.sleep(0.1)

    def test_authentication_expired_window_forwards_nothing(self):
        b = self.budget()
        self.now[0] = self.window["deadline"]
        with patch.object(auth.Transport, "send", create=True) as send:
            with self.assertRaises(Stop):
                auth.Transport(b, TOKEN, send).forward(None, request())
            send.assert_not_called()

    def test_provider_claim_race_has_only_one_winner(self):
        receipt = self.complete()
        original = h.persist

        def race(path, value):
            original(path, value)
            raise FileExistsError("simulated-racing-claim")

        with patch.object(h, "persist", race), self.assertRaises(FileExistsError):
            h.consume(receipt, ROOT, clock=self.clock)
        with self.assertRaises(ValueError):
            h.consume(receipt, ROOT, clock=self.clock)

    def test_transport_records_before_forward_and_failure_latches(self):
        b = self.budget()

        def failed(*_):
            rows = [
                json.loads(x)
                for x in (self.directory / "authentication.jsonl")
                .read_text()
                .splitlines()
            ]
            self.assertEqual(rows[-1]["event"], "attempt")
            self.assertEqual(rows[-1]["count"], 168)
            raise OSError("local-private-canary")

        t = auth.Transport(b, TOKEN, failed)
        with self.assertRaises(Stop):
            t.forward(None, request())
        with self.assertRaises(Stop):
            t.forward(None, request("sts"))
        self.assertEqual(b.count, 168)
        self.assertNotIn(
            "local-private-canary",
            (self.directory / "authentication.jsonl").read_text(),
        )
        with self.assertRaises(ValueError):
            h.seal(self.directory, ROOT, clock=self.clock)

    def test_auth_redirect_error_and_retry_do_not_forward_again(self):
        for status in [302, 401, 403, 429, 500]:
            with self.subTest(status=status):
                b = self.budget(f"failure-{status}.jsonl")
                calls = []

                def send(*_, calls=calls, status=status):
                    calls.append(1)
                    return SimpleNamespace(status_code=status)

                t = auth.Transport(b, TOKEN, send)
                with self.assertRaises(Stop):
                    t.forward(None, request())
                with self.assertRaises(Stop):
                    t.forward(None, request())
                self.assertEqual(len(calls), 1)
                self.assertEqual(b.count, 168)

    def test_auth_unexpected_endpoint_grant_and_pagination_are_not_forwarded(self):
        for change in [
            lambda r: setattr(r, "url", "https://iam.amazonaws.com/"),
            lambda r: setattr(r, "url", auth.SIGNIN + "/v1/token?secret=canary"),
            lambda r: setattr(
                r, "url", "http://signin.us-east-2.amazonaws.com/v1/token"
            ),
            lambda r: setattr(r, "body", b'{"grantType":"authorization_code"}'),
            lambda r: setattr(r, "body", b"Action=GetCallerIdentity&NextToken=canary"),
        ]:
            b = self.budget(
                "held-" + str(len(list(self.directory.iterdir()))) + ".jsonl"
            )
            r = request()
            change(r)
            with patch.object(auth.Transport, "send", create=True) as unused:
                t = auth.Transport(b, TOKEN, unused)
                with self.assertRaises(Stop):
                    t.forward(None, r)
                unused.assert_not_called()
            self.assertEqual(b.count, 167)

    def test_auth_second_refresh_or_extra_identity_read_blocked(self):
        for service in ["signin", "sts"]:
            b = self.budget(service + ".jsonl")
            calls = []

            def send(*_, calls=calls):
                calls.append(1)
                return SimpleNamespace(status_code=200)

            t = auth.Transport(b, TOKEN, send)
            t.forward(None, request(service))
            with self.assertRaises(Stop):
                t.forward(None, request(service))
            self.assertEqual(len(calls), 1)

    def test_launcher_rejects_unvalidated_or_reset_budget_before_startup(self):
        with patch("run.subprocess.Popen") as child:
            with self.assertRaises(ValueError):
                launch(Path("unused"), self.directory / "provider", [], {})
            with self.assertRaises(ValueError):
                launch(
                    Path("unused"),
                    self.directory / "provider",
                    [],
                    {},
                    handoff=self.directory / "handoff.json",
                    budget={"prior": 167},
                )
            child.assert_not_called()


class SDKTests(unittest.TestCase):
    def test_existing_cache_is_only_mutable_file_and_rejects_changed_identity(self):
        with tempfile.TemporaryDirectory() as tmp:
            home = Path(tmp)
            folder = home / ".aws/login/cache"
            folder.mkdir(parents=True)
            identity = f"arn:aws:iam::{auth.ACCOUNT}:user/local-fixture"
            (home / ".aws/config").write_text(
                f"[default]\nlogin_session = {identity}\n"
            )
            path = folder / (hashlib.sha256(identity.encode()).hexdigest() + ".json")
            path.write_text(json.dumps(TOKEN))
            with patch.object(Path, "home", return_value=home):
                cache = auth.ExistingCache()
                try:
                    token = cache.load_token(identity)
                    token["refreshToken"] = "local-rotated"
                    cache.save_token(identity, token)
                    self.assertEqual(
                        json.loads(path.read_text())["refreshToken"], "local-rotated"
                    )
                    token["accessToken"]["accountId"] = "different"
                    with self.assertRaises(ValueError):
                        cache.save_token(identity, token)
                    with self.assertRaises(ValueError):
                        cache.load_token(identity + "-other")
                finally:
                    cache.close()
            self.assertEqual(list(folder.iterdir()), [path])
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_actual_sdk_refresh_and_identity_with_fake_physical_transport(self):
        self.sdk_scenario("success")

    def test_sdk_failure_or_uncertainty_never_produces_handoff(self):
        for mode in [
            "transport",
            "500",
            "redirect",
            "malformed",
            "expired",
            "identity-mismatch",
            "cache-write",
        ]:
            with self.subTest(mode=mode):
                self.sdk_scenario(mode)

    def sdk_scenario(self, mode):
        # Actual installed/pinned SDK serialization, DPoP, signing, parsing and
        # refresh persistence path; raw network transport replaced, no sockets.
        from awscrt import crypto
        from botocore.awsrequest import AWSResponse
        from botocore.httpsession import URLLib3Session

        auth.sdk_pins()
        identity = f"arn:aws:iam::{auth.ACCOUNT}:user/local-fixture"
        key = crypto.EC.new_generate(crypto.ECType.P_256)
        token = copy.deepcopy(TOKEN)
        token["dpopKey"] = base64.b64encode(
            key.export_key(crypto.ECExportFormat.SEC1)
        ).decode()
        token["accessToken"]["expiresAt"] = "2020-01-01T00:00:00Z"

        class Cache:
            original = token

            def __init__(self):
                self.identity = identity

            def load_token(self, _):
                return copy.deepcopy(token)

            def save_token(self, _, value):
                if mode == "cache-write":
                    raise OSError("local-private-canary")
                token.update(value)

            def close(self):
                pass

        def send(_, r):
            if mode == "transport":
                raise OSError("local-private-canary")
            status = 200
            if "signin" in r.url:
                data = json.dumps(
                    {
                        "accessToken": {
                            "accessKeyId": "local-access",
                            "secretAccessKey": "local-secret",
                            "sessionToken": "local-token",
                        },
                        "expiresIn": 0 if mode == "expired" else 3600,
                        "refreshToken": "local-rotated",
                    }
                ).encode()
                headers = {"content-type": "application/json"}
                if mode == "malformed":
                    data = b"{}"
                if mode in {"500", "redirect"}:
                    data = b"{}"
                    status = 500 if mode == "500" else 302
            else:
                arn = identity if mode != "identity-mismatch" else identity + "-other"
                data = (
                    f'<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">'
                    f"<GetCallerIdentityResult><Arn>{arn}</Arn><Account>{auth.ACCOUNT}</Account>"
                    "<UserId>local</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>"
                ).encode()
                headers = {"content-type": "text/xml"}

            class Raw(io.BytesIO):
                def stream(self, amt=None, decode_content=False):
                    yield self.read()

            return AWSResponse(r.url, status, headers, Raw(data))

        with tempfile.TemporaryDirectory() as tmp:
            d = Path(tmp) / "window"
            w = h.begin(d, ROOT)
            with (
                patch.object(auth, "ExistingCache", Cache),
                patch.object(URLLib3Session, "send", send),
                patch(
                    "socket.socket",
                    side_effect=AssertionError("offline-only-no-sockets"),
                ),
            ):
                if mode == "success":
                    creds, receipt = auth.authenticate(d, ROOT, w)
                    self.assertEqual(creds["AWS_ACCESS_KEY_ID"], "local-access")
                    self.assertEqual(h.validate(receipt, ROOT)[1], 169)
                else:
                    with self.assertRaisesRegex(
                        RuntimeError, "failed-or-uncertain-no-restart"
                    ):
                        auth.authenticate(d, ROOT, w)
                    self.assertFalse((d / "handoff.json").exists())
            ledger = (d / "authentication.jsonl").read_text()
            self.assertEqual(
                h.usage(d)["newCalls"],
                2 if mode in {"success", "identity-mismatch"} else 1,
            )
            for secret in [
                "local-secret",
                "local-private-canary",
                "local-rotated",
                "local-token",
            ]:
                self.assertNotIn(secret, ledger)


if __name__ == "__main__":
    unittest.main()
