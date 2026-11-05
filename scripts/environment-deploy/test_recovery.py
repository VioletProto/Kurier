"""Actual inspection driver/authentication entrypoint; synthetic cache, sends and collector."""

import contextlib
import datetime
import io
import json
import sys
import unittest
from unittest.mock import patch

import recovery
from botocore.exceptions import ClientError
from common import Campaign, Held, canonical, private_write
from recovery_state import inspect
from test_authentication_startup import StartupFixture
from test_credentials import SCOPE


class RecoveryTests(StartupFixture):
    def recovery_case(self, expiry=900):
        self.case(expiry)
        self.path = self.base / recovery.ID
        self.auth = self.base / recovery.AUTH
        self.parent = {
            "originalLedgerSha256": "a" * 64,
            "closeProofSha256": "b" * 64,
            "expectedMarker": {},
            "accountingSha256": "c" * 64,
            "stateCutoffUtc": "2026-11-03T18:40:00+00:00",
        }
        value = recovery.authorization(SCOPE, self.pins, self.parent)
        value["authorized"] = True
        value["reviewCommit"] = "c" * 40
        private_write(self.auth, canonical(value))
        self.launched = False
        self.proposed = False

    def launch(self, source, child, command, values, **kw):
        self.launched = True
        c = kw["campaign"]
        self.assertTrue(c.get("inspectionOnly"))
        self.assertEqual(c.get("phaseDeadline"), c.get("deadline"))
        self.assertEqual(c.get("deadline"), c.get("started") + 600)
        self.assertEqual(command[1:3], [str(recovery.HERE / "recovery.mjs"), "inspect"])
        child.mkdir()
        private_write(
            child / "private-output.txt", canonical({"syntheticCollector": True})
        )
        return {"exitCode": 0}

    def plan(self, *args, **kw):
        import subprocess

        self.proposed = True
        return subprocess.CompletedProcess(
            args[0], 0, b'{"executionAuthorized":false}', b""
        )

    def run_recovery(self, held=False, *extra):
        with (
            self.driver_context(),
            patch.object(
                sys, "argv", ["recovery.py", "start", "--authorization", str(self.auth)]
            ),
            patch.object(recovery, "reconcile", return_value=self.parent),
            patch.object(recovery, "launch", side_effect=self.launch),
            patch.object(recovery.subprocess, "run", side_effect=self.plan),
            contextlib.redirect_stdout(io.StringIO()),
            contextlib.ExitStack() as stack,
        ):
            for x in extra:
                stack.enter_context(x)
            if held:
                with self.assertRaises((Held, FileExistsError)):
                    recovery.main()
            else:
                recovery.main()

    def test_actual_start_capture_sts_inspection_and_proposal_are_read_only(self):
        self.recovery_case()
        self.run_recovery()
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])
        self.assertTrue(self.launched and self.proposed)
        with contextlib.closing(Campaign(self.path)) as c:
            self.assertEqual(c.get("prior"), 868)
            self.assertEqual(c.get("count"), 869)
            self.assertEqual(c.get("maximum"), 1068)
            self.assertIsNone(c.get("closedDeadline"))
            self.assertEqual(c.get("nextPhase"), 1)
        self.assertTrue((self.path / "proposed-reopen.json").exists())
        self.run_recovery(True)
        self.assertEqual(len(self.sends), 1)

    def test_insufficient_cache_for_600_seconds_renews_once_then_sts(self):
        self.recovery_case(500)
        self.run_recovery()
        self.assertEqual(
            self.sends, ["signin/CreateOAuth2Token", "sts/GetCallerIdentity"]
        )

    def test_short_renewal_or_wrong_identity_never_launches(self):
        for mode in ["short", "identity", "denied"]:
            with self.subTest(mode=mode):
                self.recovery_case(-1)
                if mode == "short":
                    self.issued_seconds = 300
                if mode == "identity":
                    self.sts_identity = "arn:aws:iam::000000000000:user/other"
                if mode == "denied":
                    self.status = 403
                self.run_recovery(True)
                self.assertFalse(self.launched)

    def test_unapproved_recovery_never_claims_or_captures(self):
        self.recovery_case()
        v = json.loads(self.auth.read_bytes())
        v["authorized"] = False
        self.auth.write_text(json.dumps(v))
        self.run_recovery(True)
        self.assertFalse(self.path.exists())
        self.assertFalse((self.base / recovery.CLAIM).exists())
        self.assertFalse(self.sends)

    def test_inspection_meter_denies_stage_write_and_later_phase(self):
        self.recovery_case()
        original = self.launch

        def attempt(*args, **kw):
            c = kw["campaign"]
            c.admit("dynamodb/TransactWriteItems", True)
            return original(*args, **kw)

        self.run_recovery(True, patch.object(recovery, "launch", side_effect=attempt))
        with contextlib.closing(Campaign(self.path)) as c:
            self.assertEqual(c.get("count"), 869)
            self.assertEqual(
                c.db.execute(
                    "select count(*) from attempts where op='dynamodb/TransactWriteItems'"
                ).fetchone()[0],
                0,
            )
            with self.assertRaises(Held):
                c.begin("deploy")

    def test_active_cache_change_stops_without_recapture(self):
        self.recovery_case()
        original = self.sender

        def race(physical, request):
            out = original(physical, request)
            self.token["refreshToken"] += "-external"
            self.write_cache()
            return out

        self.run_recovery(True, patch("phase_credentials.URLLib3Session.send", race))
        self.assertFalse(self.launched)
        self.assertEqual(len(self.sends), 1)

    def test_count_cap_and_original_inspection_deadline_enforced(self):
        self.recovery_case()
        original = self.launch

        def cap(*args, **kw):
            c = kw["campaign"]
            deadline = c.get("deadline")
            for _ in range(199):
                n = c.admit("s3/HEAD", False)
                c.outcome(n, 200)
            self.assertEqual(c.get("count"), 1068)
            self.assertEqual(c.get("deadline"), deadline)
            c.admit("s3/HEAD", False)
            return original(*args, **kw)

        self.run_recovery(True, patch.object(recovery, "launch", side_effect=cap))
        with contextlib.closing(Campaign(self.path)) as c:
            self.assertEqual(c.get("count"), 1068)

    def test_expired_inspection_stops_before_another_forward(self):
        self.recovery_case()

        def expired(*args, **kw):
            c = kw["campaign"]
            deadline = c.get("deadline")
            c.clock = lambda: deadline
            c.admit("s3/HEAD", False)

        self.run_recovery(True, patch.object(recovery, "launch", side_effect=expired))
        with contextlib.closing(Campaign(self.path)) as c:
            self.assertEqual(c.get("count"), 869)

    def test_wrong_account_fails_fresh_capture_without_authentication(self):
        self.recovery_case()
        self.token["accessToken"]["accountId"] = "000000000000"
        self.write_cache()
        self.run_recovery(True)
        self.assertFalse(self.sends)
        self.assertFalse(self.launched)


class StateTests(unittest.TestCase):
    def case(self):
        self.date = datetime.datetime(2026, 10, 21, tzinfo=datetime.UTC)
        self.header = {"LastModified": self.date, "ETag": '"old"', "ContentLength": 10}
        self.calls = []
        self.changed = False
        self.lock = False

        class Client:
            def head_object(_, **kw):
                self.calls.append(("head", kw))
                if kw["Key"].startswith("lock/"):
                    if self.lock:
                        return self.header
                    raise ClientError({"Error": {"Code": "404"}}, "HeadObject")
                return self.header

            def get_object(_, **kw):
                self.calls.append(("get", kw))
                if not self.lock:
                    raise ClientError({"Error": {"Code": "404"}}, "GetObject")
                return {
                    **self.header,
                    "Body": io.BytesIO(
                        b'{"created":"0001-01-01T00:00:00Z"}'
                        if self.lock == "empty"
                        else b'{"created":"2026-11-03T19:00:00Z"}'
                    ),
                }

            def list_objects_v2(_, **kw):
                self.calls.append(("list", kw))
                prefix = kw["Prefix"]
                objs = (
                    [
                        {
                            "Key": prefix,
                            "ETag": '"old"',
                            "Size": 10,
                            "LastModified": self.date,
                        }
                    ]
                    if prefix.startswith("app/")
                    or (prefix.startswith("lock/") and self.lock)
                    else []
                )
                if self.changed and prefix.startswith("update/"):
                    objs = [
                        {
                            "Key": prefix + "new.json",
                            "LastModified": datetime.datetime(
                                2026, 11, 4, tzinfo=datetime.UTC
                            ),
                        }
                    ]
                return {"Contents": objs, "IsTruncated": False}

        self.client = Client()

    def test_scoped_metadata_reads_and_absent_lock(self):
        self.case()
        r = inspect(self.client, "owned", "2026-11-03T18:40:00+00:00")
        self.assertTrue(r["noObservedStateChangesSinceOriginalCollection"])
        self.assertEqual(len(self.calls), 8)

    def test_existing_empty_lock_uses_get_and_passes_response_gate(self):
        from test_policy_review import fixture
        from wire import Policy

        self.case()
        self.lock = "empty"
        result = inspect(self.client, "owned", "2026-11-03T18:40:00+00:00")
        self.assertTrue(result["noCurrentDeployLock"])
        self.assertNotIn(
            ("head", {"Bucket": "owned", "Key": "lock/kurier/dev-api.json"}), self.calls
        )
        policy = Policy(fixture.S, fixture.R, {}, phase="collect")
        policy.response(
            {"lockRead": True}, 200, {}, b'{"created":"0001-01-01T00:00:00Z"}'
        )
        self.assertTrue(policy.lock_empty)

    def test_new_state_metadata_or_live_lock_stops(self):
        for mode in ["mutation", "lock", "app"]:
            with self.subTest(mode=mode):
                self.case()
                if mode == "mutation":
                    self.changed = True
                if mode == "lock":
                    self.lock = True
                if mode == "app":
                    self.header["LastModified"] = datetime.datetime(
                        2026, 11, 4, tzinfo=datetime.UTC
                    )
                with self.assertRaises(ValueError):
                    inspect(self.client, "owned", "2026-11-03T18:40:00+00:00")


class WireAndParentTests(unittest.TestCase):
    def test_actual_sdk_state_reads_pass_read_gate_and_transaction_is_blocked(self):
        from test_policy_review import fixture

        capture, S, R = fixture.capture, fixture.S, fixture.R
        import subprocess
        from urllib.parse import urlsplit

        from wire import Policy

        policy = Policy(S, R, {}, phase="collect")
        calls = [
            (
                "head_object",
                {"Bucket": S["stateBucket"], "Key": "app/kurier/dev-api.json"},
            ),
            (
                "head_object",
                {"Bucket": S["stateBucket"], "Key": "lock/kurier/dev-api.json"},
            ),
            (
                "get_object",
                {"Bucket": S["stateBucket"], "Key": "lock/kurier/dev-api.json"},
            ),
        ]
        calls += [
            (
                "list_objects_v2",
                {"Bucket": S["stateBucket"], "Prefix": prefix, "MaxKeys": 100},
            )
            for prefix in [
                "app/kurier/dev-api.json",
                "lock/kurier/dev-api.json",
                *[
                    k + "/kurier/dev-api/"
                    for k in ["snapshot", "eventlog", "summary", "update"]
                ],
            ]
        ]
        for op, params in calls:
            q = capture("s3", op, params)
            url = urlsplit(q.url)
            result = policy.request(
                q.method,
                url.hostname,
                url.path + ("?" + url.query if url.query else ""),
                q.headers,
                q.body or b"",
            )
            self.assertFalse(result["write"])
        code = "import {fixture} from './scripts/environment-deploy/test-recovery-fixture.mjs';import {proposal} from './scripts/environment-deploy/recovery.mjs';const {bundle,context}=fixture();console.log(JSON.stringify(proposal(bundle,context).transaction));"
        transaction = json.loads(
            subprocess.check_output(["node", "--input-type=module", "-e", code])
        )
        q = capture("dynamodb", "transact_write_items", transaction)
        url = urlsplit(q.url)
        self.assertEqual(json.loads(q.body), transaction)
        with self.assertRaises(Held):
            policy.request(q.method, url.hostname, url.path, q.headers, q.body)

    def test_original_parent_is_read_only_and_pending_outcomes_rejected(self):
        import hashlib
        import shutil
        import sqlite3
        import tempfile
        from pathlib import Path

        original = recovery.driver.BASE
        source = original / "release-608-t1"
        before = hashlib.sha256((source / "campaign.sqlite").read_bytes()).hexdigest()
        recovery.reconcile()
        self.assertEqual(
            before,
            hashlib.sha256((source / "campaign.sqlite").read_bytes()).hexdigest(),
        )
        with tempfile.TemporaryDirectory() as d:
            base = Path(d)
            parent = base / "release-608-t1"
            parent.mkdir()
            for name in [
                "campaign.sqlite",
                "collect-proof.json",
                "close-proof.json",
                "close-minimum-time-check.json",
                "initial-checkpoint.json",
                "initial-checkpoint.json.accepted",
            ]:
                shutil.copyfile(source / name, parent / name)
                (parent / name).chmod(0o600)
            for name in [
                "authorization-from-608-timing-1.json",
                "window-from-608-timing-1.json",
            ]:
                shutil.copyfile(original / name, base / name)
                (base / name).chmod(0o600)
            with (
                contextlib.closing(sqlite3.connect(parent / "campaign.sqlite")) as db,
                db,
            ):
                db.execute("update attempts set outcome=0 where n=868")
            with patch.object(recovery.driver, "BASE", base), self.assertRaises(Held):
                recovery.reconcile()


if __name__ == "__main__":
    unittest.main()
