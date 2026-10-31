"""Actual driver/main -> start -> real locked cache -> pinned SDK, entirely offline."""

import contextlib
import copy
import errno
import fcntl
import hashlib
import json
import os
import socket
import sqlite3
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import driver
from auth_diagnostics import DeadlineExpired
from botocore.awsrequest import AWSResponse
from common import CREDENTIAL_POLICY, PRIOR, Campaign, Held
from credential_diagnostics import diagnostic, projection
from test_credentials import CANARY, SCOPE, Cache, Raw


class StartupTests(unittest.TestCase):
    def case(self, expiry=900):
        root = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.home = root / "home"
        folder = self.home / ".aws/login/cache"
        folder.mkdir(parents=True)
        self.config = self.home / ".aws/config"
        self.config.write_text(
            "[default]\nlogin_session = " + SCOPE["identityArn"] + "\n"
        )
        self.cache = folder / (
            hashlib.sha256(SCOPE["identityArn"].encode()).hexdigest() + ".json"
        )
        with tempfile.TemporaryFile() as seed:
            value = Cache(seed.fileno())
            value.expire_in(expiry)
            self.token = copy.deepcopy(value.original)
        self.write_cache()
        stat = self.cache.stat()
        self.checkpoint = [stat.st_ino, stat.st_size, stat.st_mtime_ns]
        self.base = root / "campaigns"
        self.base.mkdir()
        self.path = self.base / "release-from-390"
        self.auth = self.base / "authorization-from-390.json"
        self.pins = {"manifest": "a" * 64, "scope": "b" * 64}
        self.auth.write_text(
            json.dumps(
                {
                    "authorized": True,
                    "prior": PRIOR,
                    "maximum": 1799,
                    "seconds": 3300,
                    "reviewCommit": "c" * 40,
                    "manifestSha256": self.pins["manifest"],
                    "scopeSha256": self.pins["scope"],
                    "proceduralAllowanceUsd": 1,
                    "credentialPolicy": CREDENTIAL_POLICY,
                    "pendingScopedReads": [],
                }
            )
        )
        self.sends = []
        self.child_reached = False
        self.status = 200
        self.sts_account = SCOPE["account"]
        self.sts_identity = SCOPE["identityArn"]

    def write_cache(self):
        self.cache.write_text(json.dumps(self.token))
        self.cache.chmod(0o600)

    def sender(self, _session, request):
        with contextlib.closing(sqlite3.connect(self.path / "campaign.sqlite")) as db:
            row = db.execute(
                "SELECT op,outcome FROM attempts ORDER BY n DESC LIMIT 1"
            ).fetchone()
        self.assertEqual(row[1], 0)
        self.sends.append(row[0])
        if request.url.endswith("/v1/token"):
            body = json.dumps(
                {
                    "accessToken": {
                        "accessKeyId": "AKIA0000000000000000",
                        "secretAccessKey": CANARY,
                        "sessionToken": CANARY,
                    },
                    "expiresIn": 900,
                    "refreshToken": CANARY + "-rotated",
                }
            ).encode()
            kind = "application/json"
        else:
            self.assertEqual(request.url, "https://sts.us-east-2.amazonaws.com/")
            body = (
                "<GetCallerIdentityResponse><GetCallerIdentityResult><Arn>"
                + self.sts_identity
                + "</Arn><Account>"
                + self.sts_account
                + "</Account></GetCallerIdentityResult></GetCallerIdentityResponse>"
            ).encode()
            kind = "text/xml"
        return AWSResponse(request.url, self.status, {"content-type": kind}, Raw(body))

    def child(self, c, phase, subphase, _context, values):
        self.child_reached = True
        self.assertEqual((phase, subphase), ("collect", "collect-readers"))
        self.assertEqual(values["AWS_SECRET_ACCESS_KEY"], CANARY)
        self.assertEqual(c.get("credentialPhase"), "collect")
        raise Held("synthetic-stop-before-collector")

    def run_driver(self, *extra):
        with contextlib.ExitStack() as stack:
            for item in [
                patch.object(driver, "BASE", self.base),
                patch.object(driver, "check", return_value=({}, SCOPE, self.pins)),
                patch.object(driver, "pending_reads", return_value=[]),
                patch.object(driver, "stopped_parent", return_value=self.checkpoint),
                patch.object(
                    driver,
                    "git",
                    side_effect=lambda *args, **kw: (
                        "c" * 40 if args[0] == "rev-parse" else ""
                    ),
                ),
                patch.object(driver, "child", side_effect=self.child),
                patch("authentication.Path.home", return_value=self.home),
                patch("phase_credentials.URLLib3Session.send", self.sender),
                patch.object(socket, "socket", side_effect=AssertionError(CANARY)),
                patch.object(socket, "getaddrinfo", side_effect=AssertionError(CANARY)),
                patch.object(
                    sys,
                    "argv",
                    [
                        "driver.py",
                        "start",
                        "--campaign",
                        str(self.path),
                        "--authorization",
                        str(self.auth),
                    ],
                ),
                *extra,
            ]:
                stack.enter_context(item)
            with self.assertRaises(Held):
                driver.main()
        with contextlib.closing(Campaign(self.path)) as c:
            report = driver.report(c)
            self.config_after = {
                k: json.loads(v) for k, v in c.db.execute("SELECT k,v FROM config")
            }
            event_data = json.dumps(list(c.db.execute("SELECT data FROM events")))
            self.assertNotIn(CANARY, event_data)
            self.assertNotIn(CANARY, json.dumps(report))
            self.assertEqual(c.get("deadline") - c.get("started"), 3300)
            self.assertLessEqual(c.get("phaseDeadline"), c.get("deadline"))
            self.assertIsNone(c.get("closedDeadline"))
            self.assertTrue(c.get("stopped"))
            self.assertEqual(report["unmatchedAttempts"], 0)
            self.assertTrue((self.base / "window-from-390.json").is_file())
            return report

    def held(self, report, code, stage):
        self.assertEqual(
            report["authenticationFailure"], {"code": code, "stage": stage}
        )
        self.assertEqual(report["cumulative"], PRIOR)
        self.assertFalse(self.sends)
        self.assertFalse(self.child_reached)
        self.assertEqual(self.config_after["cacheStat"], self.checkpoint)

    def test_changed_size_and_mtime_stop_through_actual_start_without_adoption(self):
        self.case()
        self.token["refreshToken"] += "-outside-change"
        self.write_cache()
        self.held(
            self.run_driver(),
            "session-changed-no-unmetered-refresh",
            "checkpoint-check",
        )

    def test_replaced_inode_is_not_adopted(self):
        self.case()
        old = self.cache.with_suffix(".retained")
        self.cache.rename(old)
        self.write_cache()
        self.held(
            self.run_driver(),
            "session-changed-no-unmetered-refresh",
            "checkpoint-check",
        )

    def test_missing_cache_is_specific_and_zero_call(self):
        self.case()
        self.cache.unlink()
        self.held(self.run_driver(), "cache-missing", "cache-open")

    def test_permission_failure_is_specific_and_zero_call(self):
        self.case()
        real = os.open

        def guarded(path, flags, *args, **kw):
            if Path(path) == self.cache:
                raise PermissionError(errno.EACCES, CANARY)
            return real(path, flags, *args, **kw)

        self.held(
            self.run_driver(patch("authentication.os.open", side_effect=guarded)),
            "cache-access-denied",
            "cache-open",
        )

    def test_real_competing_lock_is_specific_and_zero_call(self):
        self.case()
        with self.cache.open("rb") as locked:
            fcntl.flock(locked, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self.held(self.run_driver(), "cache-locked", "cache-open")

    def test_missing_session_configuration_is_specific(self):
        self.case()
        self.config.write_text("[default]\n")
        self.held(self.run_driver(), "session-config-missing", "cache-open")

    def test_malformed_cache_never_logs_private_json(self):
        self.case()
        self.cache.write_text(CANARY)
        self.held(self.run_driver(), "cache-json-invalid", "cache-open")

    def test_wrong_cache_account_is_not_accepted(self):
        self.case()
        self.token["accessToken"]["accountId"] = "000000000000"
        self.write_cache()
        self.held(self.run_driver(), "cache-account-or-binding-held", "cache-open")

    def test_wrong_configured_session_is_not_accepted(self):
        self.case()
        self.config.write_text(
            "[default]\nlogin_session = arn:aws:iam::000000000000:user/other\n"
        )
        self.held(self.run_driver(), "configured-identity-held", "cache-open")

    def test_matching_checkpoint_reaches_sts_then_collector_boundary(self):
        self.case()
        report = self.run_driver()
        self.assertTrue(self.child_reached)
        self.assertIsNone(report["authenticationFailure"])
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])
        self.assertEqual(report["cumulative"], 391)

    def test_real_renewal_path_is_metered_and_preserves_original_deadlines(self):
        self.case(expiry=-1)
        report = self.run_driver()
        self.assertTrue(self.child_reached)
        self.assertEqual(
            self.sends, ["signin/CreateOAuth2Token", "sts/GetCallerIdentity"]
        )
        self.assertEqual(report["cumulative"], 392)
        self.assertIsNone(report["authenticationFailure"])
        self.assertEqual(self.config_after["credentialPhase"], "collect")
        self.assertNotEqual(self.config_after["cacheStat"], self.checkpoint)

    def test_wrong_sts_identity_never_reaches_collector(self):
        self.case()
        self.sts_identity = "arn:aws:iam::747336059622:root"
        report = self.run_driver()
        self.assertFalse(self.child_reached)
        self.assertEqual(
            report["authenticationFailure"],
            {"code": "authentication-identity-held", "stage": "sts"},
        )
        self.assertEqual(report["cumulative"], 391)

    def test_failed_renewal_is_metered_once_without_cache_adoption(self):
        self.case(expiry=-1)
        self.status = 403
        report = self.run_driver()
        self.assertFalse(self.child_reached)
        self.assertEqual(self.sends, ["signin/CreateOAuth2Token"])
        self.assertEqual(report["cumulative"], 391)
        self.assertEqual(
            report["authenticationFailure"],
            {
                "code": "write-uncertain-or-http-transport-failure",
                "stage": "sdk-credentials",
            },
        )
        self.assertEqual(self.config_after["cacheStat"], self.checkpoint)
        with contextlib.closing(sqlite3.connect(self.path / "campaign.sqlite")) as db:
            reasons = [
                json.loads(v)["reason"]
                for (v,) in db.execute(
                    "SELECT data FROM events WHERE json_extract(data,'$.event')='stop'"
                )
            ]
        self.assertEqual(
            reasons,
            [
                "write-uncertain-or-http-transport-failure",
                "phase-failed-or-uncertain-no-restart",
            ],
        )

    def test_other_same_account_session_is_not_adopted(self):
        self.case()
        other = SCOPE["identityArn"] + "-other"
        self.config.write_text("[default]\nlogin_session = " + other + "\n")
        alternative = self.cache.parent / (
            hashlib.sha256(other.encode()).hexdigest() + ".json"
        )
        alternative.write_text(self.cache.read_text())
        alternative.chmod(0o600)
        self.held(self.run_driver(), "inventoried-session-required", "session-check")

    def test_cache_symlink_is_rejected_without_following_it(self):
        self.case()
        saved = self.cache.with_suffix(".retained")
        self.cache.rename(saved)
        self.cache.symlink_to(saved)
        self.held(self.run_driver(), "cache-path-held", "cache-open")

    def test_sdk_local_failure_records_only_a_closed_code(self):
        self.case()
        report = self.run_driver(
            patch(
                "phase_credentials.LoginCredentialFetcher.refresh_credentials",
                side_effect=RuntimeError(CANARY),
            )
        )
        self.held(report, "local-authentication-failed", "sdk-credentials")

    def test_sdk_processing_deadline_is_recorded_without_clock_reset(self):
        self.case()
        report = self.run_driver(
            patch(
                "phase_credentials.LoginCredentialFetcher.refresh_credentials",
                side_effect=DeadlineExpired(),
            )
        )
        self.held(report, "authentication-deadline", "sdk-credentials")

    def test_sdk_pins_failure_is_a_specific_safe_guard(self):
        with (
            patch.object(driver, "sdk_pins", side_effect=ValueError(CANARY)),
            self.assertRaisesRegex(Held, "^authentication-sdk-pins-held$"),
        ):
            driver.check()

    def test_diagnostic_codes_never_serialize_arbitrary_exception_values(self):
        for error in [
            Held(CANARY),
            RuntimeError(CANARY),
            ValueError(CANARY),
            KeyError(CANARY),
            PermissionError(errno.EACCES, CANARY),
        ]:
            self.assertNotIn(CANARY, json.dumps(diagnostic(error, "cache-open")))
        for value in [
            {"code": CANARY, "stage": "boundary"},
            {"code": [], "stage": []},
            None,
        ]:
            self.assertIsNone(projection(value))
        self.assertEqual(
            projection(
                {"code": "cache-locked", "stage": "cache-open", "private": CANARY}
            ),
            {"code": "cache-locked", "stage": "cache-open"},
        )


if __name__ == "__main__":
    unittest.main()
