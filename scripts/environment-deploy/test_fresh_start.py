"""Explicit fresh authorization through real driver/main; synthetic cache, no network."""

import contextlib
import hashlib
import io
import json
import sys
import unittest
from unittest.mock import patch

import checkpoints
import driver
from botocore.credentials import LoginCredentialFetcher
from checkpoints import (
    CLAIM_NAME,
    capture_at_start,
    fresh_policy,
    metadata,
)
from common import PRIOR, Held
from test_authentication_startup import StartupFixture
from test_credentials import SCOPE


class FreshStartTests(StartupFixture):
    def fresh_case(self, expiry=900):
        self.case(expiry)
        value = json.loads(self.auth.read_bytes())
        value["checkpointSha256"] = None
        value["checkpointPolicy"] = fresh_policy(SCOPE)
        self.auth.write_text(json.dumps(value))

    @contextlib.contextmanager
    def driver_context(self, *extra):
        with (
            super().driver_context(*extra),
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
                    "--fresh-checkpoint",
                ],
            ),
        ):
            yield

    def snapshot(self):
        return json.loads((self.path / "initial-checkpoint.json").read_bytes())

    def test_changed_pre_campaign_cache_is_captured_only_with_explicit_fresh_approval(
        self,
    ):
        self.fresh_case()
        original = self.checkpoint_path.read_bytes()
        self.token["refreshToken"] += "-approved-current-cache"
        self.write_cache()
        current = metadata(self.cache.stat())
        result = self.run_driver()
        self.assertTrue(self.child_reached)
        self.assertEqual(result["cumulative"], PRIOR + 1)
        self.assertTrue(result["freshCheckpointCaptured"])
        self.assertEqual(self.snapshot()["cacheMetadata"], current)
        self.assertNotEqual(current, self.parent_metadata)
        self.assertEqual(self.checkpoint_path.read_bytes(), original)
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])
        self.assertFalse(
            self.snapshot()["checkpointPolicy"]["historicalGrantContinuityEstablished"]
        )
        self.assertEqual(
            self.snapshot()["authorizationSha256"],
            hashlib.sha256(self.auth.read_bytes()).hexdigest(),
        )

    def test_fresh_template_does_not_capture_or_authorize(self):
        self.fresh_case()
        out = io.StringIO()
        with (
            self.driver_context(),
            patch.object(sys, "argv", ["driver.py", "template", "--fresh-checkpoint"]),
            patch.object(driver, "capture_at_start") as capture,
            contextlib.redirect_stdout(out),
        ):
            driver.main()
        value = json.loads(out.getvalue())
        self.assertIs(value["authorized"], False)
        self.assertIsNone(value["checkpointSha256"])
        self.assertEqual(value["checkpointPolicy"], fresh_policy(SCOPE))
        capture.assert_not_called()
        self.assertFalse(self.path.exists())

    def test_unapproved_or_mismatched_fresh_scope_never_claims_or_captures(self):
        for field, value in [
            ("allowFreshCapture", False),
            ("identityArn", "arn:aws:iam::000000000000:user/other"),
            ("account", "000000000000"),
            ("historicalGrantContinuityEstablished", True),
        ]:
            with self.subTest(field=field):
                self.fresh_case()
                auth = json.loads(self.auth.read_bytes())
                auth["checkpointPolicy"][field] = value
                self.auth.write_text(json.dumps(auth))
                with self.driver_context(), self.assertRaises(Held):
                    driver.main()
                self.assertFalse(self.path.exists())
                self.assertFalse((self.base / CLAIM_NAME).exists())
                self.assertFalse(self.sends)

    def test_unauthorized_execution_never_captures(self):
        self.fresh_case()
        auth = json.loads(self.auth.read_bytes())
        auth["authorized"] = False
        self.auth.write_text(json.dumps(auth))
        with (
            self.driver_context(),
            patch.object(driver, "capture_at_start") as capture,
            self.assertRaises(Held),
        ):
            driver.main()
        capture.assert_not_called()
        self.assertFalse(self.path.exists())
        self.assertFalse((self.base / CLAIM_NAME).exists())

    def test_wrong_configured_identity_and_local_account_stop_before_sts(self):
        for mode in ["identity", "account"]:
            with self.subTest(mode=mode):
                self.fresh_case()
                if mode == "identity":
                    self.config.write_text(
                        "[default]\nlogin_session = arn:aws:iam::000000000000:user/other\n"
                    )
                else:
                    self.token["accessToken"]["accountId"] = "000000000000"
                    self.write_cache()
                result = self.run_driver()
                self.assertEqual(result["cumulative"], PRIOR)
                self.assertFalse(self.child_reached)
                self.assertFalse(self.sends)
                self.assertIsNotNone(result["checkpointCaptureFailure"])
                self.assertFalse((self.path / "initial-checkpoint.json").exists())

    def test_cache_permissions_are_validated_in_fresh_capture(self):
        self.fresh_case()
        self.cache.chmod(0o644)
        result = self.run_driver()
        self.assertEqual(
            result["checkpointCaptureFailure"]["code"],
            "campaign-cache-metadata-changed",
        )
        self.assertFalse(self.sends)

    def test_fresh_capture_rejects_wrong_owner(self):
        self.fresh_case()
        actual = checkpoints.stable_metadata

        def wrong_owner(fd, path, expected=None):
            with patch(
                "checkpoints.os.getuid", return_value=self.cache.stat().st_uid + 1
            ):
                return actual(fd, path, expected)

        result = self.run_driver(
            patch("checkpoints.stable_metadata", side_effect=wrong_owner)
        )
        self.assertEqual(
            result["checkpointCaptureFailure"]["code"],
            "campaign-cache-metadata-changed",
        )
        self.assertFalse(self.sends)

    def test_capture_mutation_and_replacement_races_reject_before_authentication(self):
        for replace in [False, True]:
            with self.subTest(replace=replace):
                self.fresh_case()
                real = checkpoints.private_write

                def race(path, data, real=real, replace=replace):
                    real(path, data)
                    if path.name == "initial-checkpoint.json":
                        if replace:
                            self.cache.rename(self.cache.with_suffix(".retained"))
                        self.token["refreshToken"] += "-race"
                        self.write_cache()

                result = self.run_driver(
                    patch("checkpoints.private_write", side_effect=race)
                )
                self.assertFalse(result["freshCheckpointCaptured"])
                self.assertTrue(
                    (self.path / "initial-checkpoint.json.rejected").is_file()
                )
                self.assertFalse(self.sends)
                self.assertFalse(self.child_reached)

    def test_change_between_capture_and_authentication_is_not_recaptured(self):
        self.fresh_case()
        actual = driver.capture_at_start

        def race(c, scope):
            record = actual(c, scope)
            self.token["refreshToken"] += "-after-capture"
            self.write_cache()
            return record

        result = self.run_driver(
            patch.object(driver, "capture_at_start", side_effect=race)
        )
        self.assertTrue(result["freshCheckpointCaptured"])
        self.assertEqual(
            result["authenticationFailure"]["code"],
            "session-changed-no-unmetered-refresh",
        )
        self.assertFalse(self.sends)
        self.assertNotEqual(
            self.snapshot()["cacheMetadata"], metadata(self.cache.stat())
        )

    def test_failed_sts_identity_cannot_reach_collector(self):
        self.fresh_case()
        self.sts_identity = "arn:aws:iam::747336059622:root"
        result = self.run_driver()
        self.assertTrue(result["freshCheckpointCaptured"])
        self.assertFalse(self.child_reached)
        self.assertEqual(
            result["authenticationFailure"]["code"], "authentication-identity-held"
        )
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])

    def test_only_metered_renewal_advances_active_checkpoint(self):
        self.fresh_case(expiry=-1)
        result = self.run_driver()
        self.assertTrue(self.child_reached)
        self.assertEqual(
            self.sends, ["signin/CreateOAuth2Token", "sts/GetCallerIdentity"]
        )
        self.assertEqual(result["cumulative"], PRIOR + 2)
        self.assertNotEqual(
            self.snapshot()["cacheMetadata"], self.config_after["cacheMetadata"]
        )
        self.assertEqual(
            self.config_after["cacheMetadata"], metadata(self.cache.stat())
        )

    def test_sdk_load_rejects_external_change_before_any_send(self):
        self.fresh_case()
        original = LoginCredentialFetcher.refresh_credentials

        def race(fetcher):
            self.token["refreshToken"] += "-external-during-sdk"
            self.write_cache()
            return original(fetcher)

        result = self.run_driver(
            patch.object(
                LoginCredentialFetcher,
                "refresh_credentials",
                autospec=True,
                side_effect=race,
            )
        )
        self.assertEqual(
            result["authenticationFailure"]["code"], "campaign-cache-metadata-changed"
        )
        self.assertFalse(self.child_reached)
        self.assertFalse(self.sends)

    def test_inflight_authentication_drift_never_advances_checkpoint(self):
        for expiry in [900, -1]:
            with self.subTest(expiry=expiry):
                self.fresh_case(expiry=expiry)
                original = self.sender

                def race(physical, request, original=original):
                    response = original(physical, request)
                    self.token["refreshToken"] += "-external-inflight"
                    self.write_cache()
                    return response

                result = self.run_driver(
                    patch("phase_credentials.URLLib3Session.send", race)
                )
                self.assertFalse(self.child_reached)
                self.assertEqual(len(self.sends), 1)
                self.assertEqual(result["cumulative"], PRIOR + 1)
                self.assertEqual(
                    result["authenticationFailure"]["code"],
                    "campaign-cache-metadata-changed",
                )
                self.assertEqual(
                    self.config_after["cacheMetadata"], self.snapshot()["cacheMetadata"]
                )

    def test_sdk_cannot_save_without_successful_metered_renewal(self):
        self.fresh_case()
        before = self.cache.read_bytes()

        def unmetered(fetcher):
            # The real fetcher exposes its SDK cache through _token_loader.
            fetcher._token_loader.save_token(SCOPE["identityArn"], self.token)

        result = self.run_driver(
            patch.object(
                LoginCredentialFetcher,
                "refresh_credentials",
                autospec=True,
                side_effect=unmetered,
            )
        )
        self.assertEqual(
            result["authenticationFailure"]["code"], "unmetered-cache-save-held"
        )
        self.assertFalse(self.child_reached)
        self.assertFalse(self.sends)
        self.assertEqual(self.cache.read_bytes(), before)

    def test_mid_campaign_drift_cannot_reenter_fresh_capture_or_send(self):
        self.fresh_case()

        def during_work(c, *_args):
            deadlines = (
                c.get("deadline"),
                c.get("closedDeadline"),
                c.get("phaseDeadline"),
            )
            prior_meta = c.get("cacheMetadata")
            checkpoints.require_active_continuity(c)
            self.token["refreshToken"] += "-external-mid-campaign"
            self.write_cache()
            with self.assertRaisesRegex(
                Held, "fresh-checkpoint-approval-or-replay-held"
            ):
                capture_at_start(c, SCOPE)
            with self.assertRaisesRegex(Held, "active-cache-checkpoint-changed"):
                checkpoints.require_active_continuity(c)
            self.assertEqual(c.get("cacheMetadata"), prior_meta)
            self.assertEqual(
                (c.get("deadline"), c.get("closedDeadline"), c.get("phaseDeadline")),
                deadlines,
            )
            raise Held("synthetic-stop")

        result = self.run_driver(patch.object(driver, "child", side_effect=during_work))
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])
        self.assertEqual(result["cumulative"], PRIOR + 1)
        self.assertTrue(result["stopped"])

    def test_capture_deadline_failure_cannot_reach_authentication(self):
        from auth_diagnostics import DeadlineExpired

        self.fresh_case()
        result = self.run_driver(
            patch.object(driver, "capture_at_start", side_effect=DeadlineExpired())
        )
        self.assertEqual(
            result["checkpointCaptureFailure"]["code"], "authentication-deadline"
        )
        self.assertFalse(self.sends)
        self.assertFalse(self.child_reached)
        self.assertFalse(result["freshCheckpointCaptured"])

    def test_claim_and_snapshot_replay_rejected_without_another_capture(self):
        self.fresh_case()
        self.run_driver()
        before = (self.path / "initial-checkpoint.json").read_bytes()
        with (
            self.driver_context(),
            patch.object(driver, "capture_at_start") as capture,
            self.assertRaisesRegex(Held, "single-use-campaign-claim-already-consumed"),
        ):
            driver.main()
        capture.assert_not_called()
        self.assertEqual((self.path / "initial-checkpoint.json").read_bytes(), before)
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])

    def test_no_fresh_mode_fallback_or_existing_checkpoint_substitution(self):
        self.fresh_case()
        for argv in [
            [
                "driver.py",
                "start",
                "--campaign",
                str(self.path),
                "--authorization",
                str(self.auth),
            ],
            [
                "driver.py",
                "start",
                "--campaign",
                str(self.path),
                "--authorization",
                str(self.auth),
                "--fresh-checkpoint",
                "--checkpoint",
                str(self.checkpoint_path),
            ],
        ]:
            with (
                self.driver_context(),
                patch.object(sys, "argv", argv),
                self.assertRaises(Held),
            ):
                driver.main()
            self.assertFalse(self.path.exists())
            self.assertFalse(self.sends)


if __name__ == "__main__":
    unittest.main()
