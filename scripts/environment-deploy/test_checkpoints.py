"""Synthetic locked captures and actual driver entrypoint; no live cache or sends."""

import contextlib
import fcntl
import io
import json
import os
import socket
import sys
from pathlib import Path
from unittest.mock import patch

import checkpoints
import driver
from checkpoints import CAMPAIGN_ID, CLAIM_NAME, capture, load, stable_metadata
from common import Held, canonical, private_write
from test_authentication_startup import StartupFixture
from test_credentials import CANARY, SCOPE


class CheckpointTests(StartupFixture):
    def capture_new(self, *extra):
        target = self.checkpoint_path.with_name("additional.json")
        approved = checkpoints.approval(SCOPE, self.pins, "c" * 40, target)
        # This helper represents a separately bound synthetic capture approval.
        existing = json.loads(self.approval_path.read_bytes())
        approved.update(
            {key: existing[key] for key in existing if key != "checkpointPath"}
        )
        self.approval_path.write_text(json.dumps(approved))
        with (
            patch("checkpoints.Path.home", return_value=self.home),
            patch.object(
                socket, "socket", side_effect=AssertionError("network-disabled")
            ),
            patch.object(
                socket, "getaddrinfo", side_effect=AssertionError("DNS-disabled")
            ),
            contextlib.ExitStack() as stack,
        ):
            for item in extra:
                stack.enter_context(item)
            capture(SCOPE, self.pins, "c" * 40, self.approval_path, target)
        return target

    def reject_before_campaign(self, code):
        with self.driver_context(), self.assertRaisesRegex(Held, code):
            driver.main()
        self.assertFalse(self.path.exists())
        self.assertFalse((self.base / CLAIM_NAME).exists())
        self.assertFalse(self.sends)

    def test_private_checkpoint_contains_metadata_and_approval_only(self):
        self.case()
        record, _ = load(self.checkpoint_path, SCOPE, self.pins, "c" * 40)
        for path in [
            self.checkpoint_path,
            self.approval_path,
            Path(str(self.checkpoint_path) + ".accepted"),
        ]:
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertNotIn(CANARY, path.read_text())
            self.assertNotIn("secretAccessKey", path.read_text())
        self.assertFalse(record["approval"]["historicalGrantContinuityEstablished"])
        self.assertFalse(self.path.exists())
        self.assertFalse((self.base / CLAIM_NAME).exists())

    def test_capture_leaves_cache_bytes_and_metadata_unchanged(self):
        self.case()
        before = self.cache.read_bytes(), checkpoints.metadata(self.cache.stat())
        self.capture_new()
        self.assertEqual(
            (self.cache.read_bytes(), checkpoints.metadata(self.cache.stat())), before
        )

    def test_changed_cache_during_capture_rejects_and_preserves_failed_record(self):
        self.case()
        actual = checkpoints.private_write
        target = self.checkpoint_path.with_name("additional.json")

        def racing(path, data):
            actual(path, data)
            if path == target:
                self.token["refreshToken"] += "-outside-change"
                self.write_cache()

        with self.assertRaises(Held):
            self.capture_new(patch("checkpoints.private_write", side_effect=racing))
        self.assertTrue(target.is_file())
        self.assertTrue(Path(str(target) + ".rejected").is_file())
        with self.assertRaisesRegex(Held, "rejected-checkpoint-held"):
            load(target, SCOPE, self.pins, "c" * 40)

    def test_replaced_cache_during_capture_is_rejected(self):
        self.case()
        actual = checkpoints.private_write
        target = self.checkpoint_path.with_name("additional.json")

        def replacing(path, data):
            actual(path, data)
            if path == target:
                self.cache.rename(self.cache.with_suffix(".retained"))
                self.write_cache()

        with self.assertRaises(Held):
            self.capture_new(patch("checkpoints.private_write", side_effect=replacing))
        self.assertTrue(Path(str(target) + ".rejected").is_file())

    def test_capture_requires_exclusive_lock(self):
        self.case()
        with self.cache.open("rb") as fd:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            with self.assertRaises(BlockingIOError):
                self.capture_new()

    def test_capture_rejects_wrong_account_and_malformed_structure(self):
        for field, value in [
            ("accountId", "000000000000"),
            ("sessionToken", ""),
            ("expiresAt", "invalid"),
        ]:
            with self.subTest(field=field):
                self.case()
                self.token["accessToken"][field] = value
                self.write_cache()
                with self.assertRaises(Held):
                    self.capture_new()

    def test_capture_rejects_wrong_identity_and_key(self):
        self.case()
        self.config.write_text(
            "[default]\nlogin_session = arn:aws:iam::000000000000:user/other\n"
        )
        with self.assertRaisesRegex(Held, "inventoried-session-required"):
            self.capture_new()
        self.config.write_text(
            "[default]\nlogin_session = " + SCOPE["identityArn"] + "\n"
        )
        self.token["dpopKey"] = CANARY
        self.write_cache()
        with self.assertRaisesRegex(Held, "checkpoint-cache-key-held"):
            self.capture_new()

    def test_capture_rejects_cache_permissions_and_ownership(self):
        self.case()
        self.cache.chmod(0o644)
        with self.assertRaisesRegex(Held, "campaign-cache-metadata-changed"):
            self.capture_new()
        self.cache.chmod(0o600)
        with (
            self.cache.open("rb") as fd,
            patch("checkpoints.os.getuid", return_value=os.getuid() + 1),
            self.assertRaisesRegex(Held, "campaign-cache-metadata-changed"),
        ):
            stable_metadata(fd.fileno(), self.cache)

    def test_approval_must_be_explicit_and_bound_to_source_scope_and_usage(self):
        for field, value in [
            ("authorized", False),
            ("authorized", 1),
            ("prior", 0),
            ("campaignId", "release-from-390"),
            ("toolingCommit", "f" * 40),
            ("scopeSha256", "f" * 64),
        ]:
            with self.subTest(field=field, value=value):
                self.case()
                approved = json.loads(self.approval_path.read_bytes())
                approved[field] = value
                self.approval_path.write_text(json.dumps(approved))
                with self.assertRaisesRegex(
                    Held, "explicit-checkpoint-capture-approval-required"
                ):
                    self.capture_new()

    def test_overwrite_and_missing_acceptance_are_held(self):
        self.case()
        with (
            patch("checkpoints.Path.home", return_value=self.home),
            self.assertRaisesRegex(Held, "checkpoint-output-already-exists"),
        ):
            capture(
                SCOPE, self.pins, "c" * 40, self.approval_path, self.checkpoint_path
            )
        Path(str(self.checkpoint_path) + ".accepted").rename(
            self.checkpoint_path.with_name("accepted.retained")
        )
        with self.assertRaises(FileNotFoundError):
            load(self.checkpoint_path, SCOPE, self.pins, "c" * 40)

    def test_old_consumed_claim_survives_new_campaign_and_usage_starts_at_390(self):
        self.case()
        old = self.base / "window-from-390.json"
        private_write(old, b'{"historicalConsumed":true}')
        report = self.run_driver()
        self.assertEqual(old.read_bytes(), b'{"historicalConsumed":true}')
        self.assertTrue((self.base / CLAIM_NAME).is_file())
        self.assertEqual(report["prior"], 390)
        self.assertEqual(report["cumulative"], 391)
        self.assertEqual(report["maximum"], 1799)
        self.assertEqual(report["attempts"][0]["count"], 391)

    def test_new_identity_and_claim_cannot_be_reused_or_renamed(self):
        self.case()
        self.run_driver()
        sends = list(self.sends)
        with (
            self.driver_context(),
            self.assertRaisesRegex(Held, "single-use-campaign-claim-already-consumed"),
        ):
            driver.main()
        self.assertEqual(sends, self.sends)
        self.case()
        self.path = self.base / "release-from-390"
        self.reject_before_campaign("fresh-bound-campaign-identity-required")

    def test_authorization_cannot_reset_budget_or_change_checkpoint(self):
        for field, value in [
            ("prior", 0),
            ("maximum", 500),
            ("seconds", 3600),
            ("checkpointSha256", "f" * 64),
            ("campaignId", "another"),
        ]:
            with self.subTest(field=field):
                self.case()
                value_auth = json.loads(self.auth.read_bytes())
                value_auth[field] = value
                self.auth.write_text(json.dumps(value_auth))
                self.reject_before_campaign(
                    "explicit-expanded-execution-authorization-required"
                )

    def test_bound_checkpoint_and_approval_tampering_are_held(self):
        self.case()
        record = json.loads(self.checkpoint_path.read_bytes())
        record["cacheMetadata"]["size"] += 1
        self.checkpoint_path.write_bytes(canonical(record))
        with self.assertRaisesRegex(Held, "bound-checkpoint-held"):
            load(self.checkpoint_path, SCOPE, self.pins, "c" * 40)
        self.case()
        self.approval_path.write_text("{}")
        with self.assertRaisesRegex(Held, "bound-checkpoint-held"):
            load(self.checkpoint_path, SCOPE, self.pins, "c" * 40)

    def test_capture_approval_cannot_be_reused_for_another_output(self):
        self.case()
        with (
            patch("checkpoints.Path.home", return_value=self.home),
            self.assertRaisesRegex(
                Held, "explicit-checkpoint-capture-approval-required"
            ),
        ):
            capture(
                SCOPE,
                self.pins,
                "c" * 40,
                self.approval_path,
                self.checkpoint_path.with_name("unapproved-output.json"),
            )
        self.assertFalse(
            self.checkpoint_path.with_name("unapproved-output.json").exists()
        )

    def test_actual_capture_cli_is_local_only_and_separately_bound(self):
        self.case()
        target = self.checkpoint_path.with_name("cli.json")
        self.approval_path.write_text(
            json.dumps(checkpoints.approval(SCOPE, self.pins, "c" * 40, target))
        )
        output = io.StringIO()
        with (
            self.driver_context(),
            patch.object(
                sys,
                "argv",
                [
                    "driver.py",
                    "capture-checkpoint",
                    "--approval",
                    str(self.approval_path),
                    "--checkpoint",
                    str(target),
                ],
            ),
            contextlib.redirect_stdout(output),
        ):
            driver.main()
        self.assertEqual(
            json.loads(output.getvalue()),
            {
                "checkpointCaptured": True,
                "historicalGrantContinuityEstablished": False,
                "awsCalls": 0,
            },
        )
        record, _ = load(target, SCOPE, self.pins, "c" * 40)
        self.assertEqual(record["cacheStat"], self.checkpoint)
        self.assertFalse(self.sends)
        self.assertFalse(self.path.exists())

    def test_actual_template_binds_checkpoint_but_does_not_authorize_collection(self):
        self.case()
        output = io.StringIO()
        with (
            self.driver_context(),
            patch.object(
                sys,
                "argv",
                ["driver.py", "template", "--checkpoint", str(self.checkpoint_path)],
            ),
            contextlib.redirect_stdout(output),
        ):
            driver.main()
        result = json.loads(output.getvalue())
        self.assertIs(result["authorized"], False)
        self.assertEqual(result["prior"], 390)
        self.assertEqual(result["campaignId"], CAMPAIGN_ID)
        self.assertEqual(
            result["checkpointSha256"],
            load(self.checkpoint_path, SCOPE, self.pins, "c" * 40)[1],
        )
        self.assertFalse(self.sends)
        self.assertFalse(self.path.exists())

    def test_identity_configuration_change_during_capture_is_rejected(self):
        self.case()
        actual = checkpoints.private_write
        target = self.checkpoint_path.with_name("additional.json")

        def changing(path, data):
            actual(path, data)
            if path == target:
                self.config.write_text("[default]\nlogin_session = changed\n")

        with self.assertRaisesRegex(Held, "checkpoint-changed-during-capture"):
            self.capture_new(patch("checkpoints.private_write", side_effect=changing))
        self.assertTrue(Path(str(target) + ".rejected").is_file())

    def test_same_size_mtime_but_changed_ctime_is_not_silently_adopted(self):
        self.case()
        before = self.cache.stat()
        self.cache.chmod(0o400)
        self.cache.chmod(0o600)
        self.assertEqual(before.st_size, self.cache.stat().st_size)
        self.assertEqual(before.st_mtime_ns, self.cache.stat().st_mtime_ns)
        self.held(
            self.run_driver(), "campaign-cache-metadata-changed", "checkpoint-check"
        )


if __name__ == "__main__":
    import unittest

    unittest.main()
