"""Actual driver phase entrypoint, pinned SDK and virtual time; no live cache/network."""

import contextlib
import io
import json
import sys
import time
from unittest.mock import patch

import driver
from botocore.credentials import LoginCredentialFetcher
from common import PHASES, PRIOR, Campaign, Held, canonical, digest, private_write
from test_authentication_startup import StartupFixture


class TimingTests(StartupFixture):
    def phase_case(
        self, expiry=900, *, phase="deploy", campaign_left=3300, closed_left=1800
    ):
        self.case(expiry)
        self.now = [time.monotonic()]
        self.original_threshold = LoginCredentialFetcher._REFRESH_THRESHOLD
        auth = json.loads(self.auth.read_bytes())
        with contextlib.closing(
            Campaign(
                self.path,
                create={
                    **{
                        k: auth[k]
                        for k in [
                            "prior",
                            "maximum",
                            "seconds",
                            "manifestSha256",
                            "scopeSha256",
                            "reviewCommit",
                        ]
                    },
                    "authorizationSha256": "d" * 64,
                },
                clock=lambda: self.now[0],
            )
        ) as c:
            c.put("nextPhase", PHASES.index(phase))
            c.put("deadline", self.now[0] + campaign_left)
            c.put(
                "closedDeadline",
                self.now[0] + closed_left
                if phase in {"deploy", "verify", "activate"}
                else None,
            )
            c.put("cacheStat", self.checkpoint)
            c.put("cacheMetadata", self.parent_metadata)
            c.put(
                "cacheIdentityArn", json.loads(driver.SCOPE.read_bytes())["identityArn"]
            )
            for name, proof in [
                ("collect", {"bundle": {"routing": {}}}),
                ("close", {"next": {"synthetic": "closed"}}),
            ]:
                data = canonical(proof)
                private_write(self.path / (name + "-proof.json"), data)
                c.put("proof:" + name, {"sha256": digest(data)})
            self.approved = self.path / "deploy-approval.json"
            if phase == "deploy":
                private_write(
                    self.approved,
                    canonical({**driver.prepare(c, "deploy"), "authorized": True}),
                )
            self.original_deadlines = (c.get("deadline"), c.get("closedDeadline"))
        self.issued_seconds = 900
        self.behavior = None

    @contextlib.contextmanager
    def phase_context(self):
        with (
            self.driver_context(),
            patch.object(
                driver,
                "Campaign",
                side_effect=lambda path: Campaign(path, clock=lambda: self.now[0]),
            ),
            patch.object(
                sys,
                "argv",
                [
                    "driver.py",
                    "phase",
                    "--campaign",
                    str(self.path),
                    "--phase",
                    "deploy",
                    "--approval",
                    str(self.approved),
                ],
            ),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            yield

    def read_config(self):
        with contextlib.closing(Campaign(self.path, clock=lambda: self.now[0])) as c:
            return {k: json.loads(v) for k, v in c.db.execute("select k,v from config")}

    def child(self, c, phase, subphase, _context, values):
        self.child_reached = True
        self.assertEqual((phase, subphase), ("deploy", "deploy"))
        self.assertEqual(c.get("phaseDeadline"), self.now[0] + 840)
        self.assertEqual(
            (c.get("deadline"), c.get("closedDeadline")), self.original_deadlines
        )
        if self.behavior:
            self.behavior(c)
        return self.path / "deploy", {"closedVerified": True, "artifactsMatched": True}

    def run_phase(self, held=False):
        with self.phase_context():
            if held:
                with self.assertRaises(Held):
                    driver.main()
            else:
                driver.main()
        self.assertEqual(
            LoginCredentialFetcher._REFRESH_THRESHOLD, self.original_threshold
        )
        c = self.read_config()
        self.assertEqual((c["deadline"], c["closedDeadline"]), self.original_deadlines)
        self.assertNotIn("synthetic-private-renewal-canary", json.dumps(c))
        return c

    def test_long_lease_renews_token_above_default_threshold_and_sts_before_child(self):
        self.phase_case(expiry=600)
        c = self.run_phase()
        self.assertEqual(
            self.sends, ["signin/CreateOAuth2Token", "sts/GetCallerIdentity"]
        )
        self.assertEqual(c["count"], PRIOR + 2)
        self.assertEqual(c["nextPhase"], 3)
        self.assertNotEqual(c["cacheMetadata"], self.parent_metadata)

    def test_adequate_token_reused_without_global_sdk_change(self):
        self.phase_case()
        self.run_phase()
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])

    def test_short_response_rejects_work_and_cannot_replay_phase(self):
        self.phase_case(expiry=600)
        self.issued_seconds = 600
        c = self.run_phase(held=True)
        self.assertFalse(self.child_reached)
        self.assertEqual(
            c["authenticationFailure"]["code"], "credential-phase-window-too-short"
        )
        self.run_phase(held=True)
        self.assertEqual(len(self.sends), 2)

    def test_failed_renewal_stops_before_sts_and_replay(self):
        self.phase_case(expiry=600)
        self.status = 403
        self.run_phase(held=True)
        self.run_phase(held=True)
        self.assertEqual(self.sends, ["signin/CreateOAuth2Token"])
        self.assertFalse(self.child_reached)

    def test_wrong_identity_and_external_cache_drift_reject_long_phase(self):
        for mode in ["identity", "cache"]:
            with self.subTest(mode=mode):
                self.phase_case(expiry=600)
                if mode == "identity":
                    self.sts_identity = "arn:aws:iam::000000000000:user/other"
                else:
                    self.token["refreshToken"] += "-external"
                    self.write_cache()
                self.run_phase(held=True)
                self.assertFalse(self.child_reached)
                self.assertEqual(len(self.sends), 2 if mode == "identity" else 0)

    def test_read_polling_after_240_seconds_and_840_deadline_hold(self):
        self.phase_case()

        def poll(c):
            self.now[0] += 300
            c.check()
            n = c.admit("cloudfront/GetInvalidation", False)
            c.outcome(n, 200)
            self.assertEqual(c.get("count"), PRIOR + 2)
            self.now[0] = c.get("phaseDeadline")
            c.admit("cloudfront/GetInvalidation", False)

        self.behavior = poll
        c = self.run_phase(held=True)
        self.assertTrue(c["stopped"])
        self.assertEqual(c["count"], PRIOR + 2)
        self.assertEqual(len(self.sends), 1)

    def test_uncertain_mutation_never_replayed(self):
        self.phase_case()

        def uncertain(c):
            n = c.admit("lambda/UpdateFunctionCode", True)
            c.outcome(n, None)

        self.behavior = uncertain
        c = self.run_phase(held=True)
        self.run_phase(held=True)
        self.assertEqual(c["count"], PRIOR + 2)
        with contextlib.closing(Campaign(self.path)) as c:
            self.assertEqual(
                c.db.execute(
                    "select count(*) from attempts where op='lambda/UpdateFunctionCode'"
                ).fetchone()[0],
                1,
            )

    def test_every_phase_limit_and_deadline_clipping(self):
        for phase in ["collect", "close", "deploy", "verify", "activate", "final"]:
            for campaign_left, closed_left in [(3300, 1800), (100, 1800), (3300, 90)]:
                with self.subTest(
                    phase=phase, campaign_left=campaign_left, closed_left=closed_left
                ):
                    self.phase_case(
                        phase=phase,
                        campaign_left=campaign_left,
                        closed_left=closed_left,
                    )
                    with contextlib.closing(
                        Campaign(self.path, clock=lambda: self.now[0])
                    ) as c:
                        # Close begins its one original closed window here.
                        if phase == "close":
                            c.put("closedDeadline", None)
                        c.begin(phase)
                        closed = c.get("closedDeadline")
                        expected = min(
                            self.now[0] + (840 if phase == "deploy" else 240),
                            c.get("deadline"),
                            closed or c.get("deadline"),
                        )
                        self.assertEqual(c.get("phaseDeadline"), expected)
                        if phase == "close":
                            self.assertEqual(
                                closed, min(c.get("deadline"), self.now[0] + 1800)
                            )
                        self.now[0] = expected
                        with self.assertRaises(Held):
                            c.check()

    def test_clipped_deploy_requires_only_remaining_validity(self):
        self.phase_case(expiry=600, campaign_left=200)
        with patch.object(
            self,
            "child",
            side_effect=lambda *_: (
                self.path / "deploy",
                {"closedVerified": True, "artifactsMatched": True},
            ),
        ):
            self.run_phase()
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])


if __name__ == "__main__":
    import unittest

    unittest.main()
