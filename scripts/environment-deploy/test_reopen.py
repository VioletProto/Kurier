"""One-shot recovery: actual driver/auth path and actual SDK serialization; no network."""

import contextlib
import copy
import io
import json
import sys
import unittest
from unittest.mock import patch
from urllib.parse import urlsplit

import driver
import reopen
import reopen_guard
from common import Campaign, Held, canonical, private_write
from reopen_bindings import AFTER_SHA, AUTH, CLAIM, ID
from test_authentication_startup import StartupFixture
from test_credentials import SCOPE
from test_policy_review import fixture
from wire import Policy

BASE = driver.BASE / "recovery-inspect-868-r1"
PROPOSAL = json.loads((BASE / "proposed-reopen.json").read_bytes())
PROOF = json.loads((BASE / "inspection-proof.json").read_bytes())


class ReopenTests(StartupFixture):
    def setup_case(self, expiry=900):
        self.case(expiry)
        self.path = self.base / ID
        self.auth = self.base / AUTH
        self.bound = {
            **PROOF["originalParent"],
            "inspectionLedgerSha256": "e" * 64,
            "inspectionProofSha256": "f" * 64,
            "fixedProposal": copy.deepcopy(PROPOSAL),
            "cacheStat": self.checkpoint,
            "cacheMetadata": self.parent_metadata,
            "protectedEvidence": {},
        }
        with self.driver_context():
            value = reopen.authorization(SCOPE, self.pins, self.bound)
        value["authorized"] = True
        private_write(self.auth, canonical(value))
        self.children = []
        self.mode = "ok"

    def child(self, c, name, action, context, values):
        self.children.append(action)
        self.assertEqual(c.get("credentialPhase"), "recover-old")
        self.assertTrue(values["AWS_SESSION_TOKEN"])
        if action == "before":
            (c.directory / "before").mkdir()
            bundle = copy.deepcopy(PROOF["bundle"])
            if self.mode == "stale":
                bundle["marker"]["version"]["N"] = "18"
            return bundle, {"identityVerified": True}
        reopen_guard.verify_arm(c)
        if self.mode == "drift-proof":
            (c.directory / "before-write-proof.json").write_bytes(b"changed")
            reopen_guard.verify_arm(c)
        n = c.admit("dynamodb/TransactWriteItems", True)
        if self.mode == "uncertain":
            raise Held("synthetic-uncertain")
        c.outcome(n, 200)
        return {
            "stageWrites": 1,
            "marker": PROPOSAL["after"]
            if self.mode != "readback"
            else PROPOSAL["before"],
            "accountingSha256": self.bound["accountingSha256"],
        }, {"identityVerified": True, "lastMarkerSha256": AFTER_SHA}

    def run_entry(self, held=False, *extra):
        with (
            self.driver_context(),
            patch.object(
                sys, "argv", ["reopen.py", "start", "--authorization", str(self.auth)]
            ),
            patch.object(reopen, "bindings", return_value=self.bound),
            patch.object(reopen, "child", side_effect=self.child),
            patch.object(reopen_guard, "verify_source"),
            contextlib.redirect_stdout(io.StringIO()),
            contextlib.ExitStack() as stack,
        ):
            for item in extra:
                stack.enter_context(item)
            if held:
                with self.assertRaises((Held, FileExistsError)):
                    reopen.main()
            else:
                reopen.main()

    def test_actual_start_sts_real_offline_assessment_one_write_and_replay_denial(self):
        self.setup_case()
        self.run_entry()
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])
        self.assertEqual(self.children, ["before", "execute"])
        with contextlib.closing(Campaign(self.path)) as c:
            self.assertEqual(c.get("prior"), 955)
            self.assertEqual(c.get("count"), 957)
            self.assertEqual(c.get("maximum"), 1205)
            self.assertEqual(c.get("deadline"), c.get("started") + 600)
            self.assertIsNone(c.get("closedDeadline"))
            self.assertEqual(c.get("nextPhase"), 1)
        self.run_entry(True)
        self.assertEqual(len(self.sends), 1)
        self.assertTrue((self.base / CLAIM).exists())

    def test_renewal_is_metered_once_before_children(self):
        self.setup_case(500)
        self.run_entry()
        self.assertEqual(
            self.sends, ["signin/CreateOAuth2Token", "sts/GetCallerIdentity"]
        )

    def test_pre_campaign_cache_drift_stops_without_capture_or_sts(self):
        self.setup_case()
        self.token["refreshToken"] += "-outside"
        self.write_cache()
        self.run_entry(True)
        self.assertEqual(self.sends, [])
        self.assertEqual(self.children, [])

    def test_wrong_identity_failed_or_short_renewal_stops_before_children(self):
        for mode in ["identity", "denied", "short"]:
            with self.subTest(mode=mode):
                self.setup_case(-1)
                if mode == "identity":
                    self.sts_account = "000000000000"
                elif mode == "denied":
                    self.status = 403
                else:
                    self.issued_seconds = 300
                self.run_entry(True)
                self.assertEqual(self.children, [])

    def test_stale_evidence_uncertainty_and_readback_do_not_replay(self):
        for mode in ["stale", "drift-proof", "uncertain", "readback"]:
            with self.subTest(mode=mode):
                self.setup_case()
                self.mode = mode
                self.run_entry(True)
                with contextlib.closing(Campaign(self.path)) as c:
                    count = c.db.execute(
                        "select count(*) from attempts where op='dynamodb/TransactWriteItems'"
                    ).fetchone()[0]
                    self.assertEqual(count, int(mode in ["uncertain", "readback"]))
                    self.assertTrue(c.get("stopped"))
                self.run_entry(True)
                self.assertEqual(len(self.sends), 1)

    def test_authorization_changes_rejected_before_claim(self):
        self.setup_case()
        value = json.loads(self.auth.read_bytes())
        value["transactionSha256"] = "0" * 64
        self.auth.write_bytes(canonical(value))
        self.run_entry(True)
        self.assertFalse((self.base / CLAIM).exists())
        self.assertEqual(self.sends, [])

    def test_active_cache_change_after_sts_stops_before_children(self):
        self.setup_case()
        original = self.sender

        def changed(session, request):
            response = original(session, request)
            self.token["refreshToken"] += "-external"
            self.write_cache()
            return response

        self.run_entry(True, patch("phase_credentials.URLLib3Session.send", changed))
        self.assertEqual(self.children, [])
        self.assertEqual(self.sends, ["sts/GetCallerIdentity"])

    def test_expired_arm_or_changed_snapshot_stops_before_write(self):
        for mode in ["expired", "snapshot", "protected", "source"]:
            with self.subTest(mode=mode):
                self.setup_case()
                original = self.child

                def change(
                    c, name, action, context, values, mode=mode, original=original
                ):
                    if action == "execute":
                        if mode == "expired":
                            c.clock = lambda: c.get("reopenArmDeadline")
                        elif mode == "snapshot":
                            (c.directory / "before/readers-output.json").write_bytes(
                                b"changed"
                            )
                        elif mode == "protected":
                            c.put(
                                "protectedEvidence",
                                {"missing-recovery-evidence": "0" * 64},
                            )
                        else:
                            with patch.object(
                                reopen_guard,
                                "verify_source",
                                side_effect=Held("source-changed"),
                            ):
                                reopen_guard.verify_arm(c)
                        reopen_guard.verify_arm(c)
                    return original(c, name, action, context, values)

                with self.assertRaises((Held, FileNotFoundError)):
                    self.run_entry(
                        False, patch.object(reopen, "child", side_effect=change)
                    )
                with contextlib.closing(Campaign(self.path)) as c:
                    self.assertEqual(
                        c.db.execute(
                            "select count(*) from attempts where write=1"
                        ).fetchone()[0],
                        0,
                    )

    def test_durable_gate_blocks_unarmed_stale_low_reserve_other_writes_and_replay(
        self,
    ):
        for mode in [
            "unarmed",
            "stale",
            "time",
            "calls",
            "other",
            "pending",
            "complete",
        ]:
            with self.subTest(mode=mode):
                self.setup_case()
                with contextlib.closing(
                    Campaign(
                        self.path,
                        recovery_write=True,
                        create={
                            "prior": 955,
                            "maximum": 1205,
                            "seconds": 600,
                            "manifestSha256": "a" * 64,
                            "scopeSha256": "b" * 64,
                            "reviewCommit": "c" * 40,
                            "authorizationSha256": "d" * 64,
                        },
                    )
                ) as c:
                    c.begin("recover-old")
                    c.put("reopenWriteArmed", mode != "unarmed")
                    c.put(
                        "reopenArmDeadline", c.clock() + (0 if mode == "stale" else 240)
                    )
                    if mode == "time":
                        c.clock = lambda: c.get("deadline") - 59
                    if mode == "calls":
                        c.put("count", 1174)
                    if mode in ["pending", "complete"]:
                        n = c.admit("dynamodb/TransactWriteItems", True)
                        if mode == "complete":
                            c.outcome(n, 200)
                    with self.assertRaises(Held):
                        c.admit(
                            "s3/PutObject"
                            if mode == "other"
                            else "dynamodb/TransactWriteItems",
                            True,
                        )


class WireTests(unittest.TestCase):
    def policy(self):
        p = Policy(
            fixture.S,
            fixture.R,
            {},
            phase="recover-old",
            stage_transaction=PROPOSAL["transaction"],
            closed_marker=PROPOSAL["before"],
        )
        p.identity_verified = p.closed_verified = True
        return p

    def request(self, policy, params):
        q = fixture.capture("dynamodb", "transact_write_items", params)
        u = urlsplit(q.url)
        return policy.request(q.method, u.hostname, u.path, q.headers, q.body)

    def test_actual_sdk_exact_transaction_and_one_shot(self):
        p = self.policy()
        self.assertEqual(
            self.request(p, PROPOSAL["transaction"])["operation"],
            "dynamodb/TransactWriteItems",
        )
        with self.assertRaises(Held):
            self.request(p, PROPOSAL["transaction"])

    def test_changed_token_condition_action_and_table_blocked(self):
        for mode in ["token", "condition", "action", "table"]:
            with self.subTest(mode=mode):
                v = copy.deepcopy(PROPOSAL["transaction"])
                if mode == "token":
                    v["ClientRequestToken"] += "x"
                elif mode == "condition":
                    v["TransactItems"][0]["Update"]["ConditionExpression"] = (
                        "attribute_exists(PK)"
                    )
                elif mode == "table":
                    v["TransactItems"][0]["Update"]["TableName"] = "other"
                else:
                    v["TransactItems"].pop()
                p = self.policy()
                p.stage_transaction = v
                with self.assertRaises(Held):
                    self.request(p, v)

    def test_identity_closed_marker_and_postwrite_marker_binding(self):
        for guard in ["identity_verified", "closed_verified"]:
            p = self.policy()
            setattr(p, guard, False)
            with self.assertRaises(Held):
                self.request(p, PROPOSAL["transaction"])
        meta = {
            "service": "dynamodb",
            "api": "GetItem",
            "params": {"Key": {"SK": {"S": "META"}}},
        }
        p = self.policy()
        p.response(meta, 200, {}, canonical({"Item": PROPOSAL["before"]}))
        with self.assertRaises(Held):
            p.response(meta, 200, {}, canonical({"Item": PROPOSAL["after"]}))
        self.request(p, PROPOSAL["transaction"])
        p.response(meta, 200, {}, canonical({"Item": PROPOSAL["after"]}))
        with self.assertRaises(Held):
            p.response(meta, 200, {}, canonical({"Item": PROPOSAL["before"]}))


if __name__ == "__main__":
    unittest.main()
