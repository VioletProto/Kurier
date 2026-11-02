"""Synthetic completed-parent bindings through check entrypoint; no live cache/network."""

import contextlib
import io
import json
import socket
import sqlite3
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import driver
from checkpoints import SOURCE, fresh_policy
from common import Held, canonical, digest, private_write
from completed_parent import verify
from test_credentials import SCOPE

COMMIT = "bf1effb2835ae6be00bdbd753a802dc0b9bf6741"


class CompletedParentTests(unittest.TestCase):
    def case(self):
        self.root = Path(self.enterContext(tempfile.TemporaryDirectory()))
        self.base = self.root / ".build/project-environments-execution"
        self.parent = self.base / "release-499-f1"
        self.parent.mkdir(parents=True)
        self.docs = self.root / "docs/development"
        self.docs.mkdir(parents=True)
        self.pins = {"manifest": "a" * 64, "scope": "b" * 64}
        policy = fresh_policy(SCOPE)
        auth = {
            "authorized": True,
            "campaignId": "release-499-f1",
            "prior": 499,
            "maximum": 1799,
            "seconds": 3300,
            "proceduralAllowanceUsd": 1,
            "checkpointSha256": None,
            "reviewCommit": COMMIT,
            "manifestSha256": self.pins["manifest"],
            "scopeSha256": self.pins["scope"],
            "checkpointPolicy": policy,
        }
        auth_sha = self.write(self.base / "authorization-from-499-fresh-1.json", auth)
        self.write(
            self.base / "window-from-499-fresh-1.json",
            {
                "campaignId": "release-499-f1",
                "checkpointPolicy": policy,
                "checkpointSha256": None,
                "authorizationSha256": auth_sha,
                "directory": str(self.parent),
            },
        )
        cp = {
            "campaignId": "release-499-f1",
            "checkpointPolicy": policy,
            "authorizationSha256": auth_sha,
            "toolingCommit": COMMIT,
            "manifestSha256": self.pins["manifest"],
            "scopeSha256": self.pins["scope"],
        }
        cp_sha = self.write(self.parent / "initial-checkpoint.json", cp)
        self.write(
            self.parent / "initial-checkpoint.json.accepted",
            {"checkpointSha256": cp_sha},
        )
        bundle = {"marker": {"synthetic": True}}
        (self.parent / "collect-readers").mkdir()
        self.write(self.parent / "collect-readers/private-output.txt", bundle)
        proof_sha = self.write(
            self.parent / "collect-proof.json",
            {"bundle": bundle, "allFourFreshArchivesMatched": True},
        )
        c = {
            "prior": 499,
            "count": 608,
            "maximum": 1799,
            "seconds": 3300,
            "started": 1000,
            "deadline": 4300,
            "stopped": False,
            "activePhase": None,
            "nextPhase": 1,
            "closedDeadline": None,
            "phaseDeadline": None,
            "reviewCommit": COMMIT,
            "manifestSha256": self.pins["manifest"],
            "scopeSha256": self.pins["scope"],
            "authorizationSha256": auth_sha,
            "campaignId": "release-499-f1",
            "checkpointPolicy": policy,
            "checkpointSha256": cp_sha,
            "freshCaptureCompleted": True,
            "freshCaptureStarted": True,
            "credentialPhase": "collect",
            "cacheStat": [1, 2, 3],
            "cacheMetadata": {"inode": 1, "size": 2, "mtimeNs": 3},
            "proof:collect": {"sha256": proof_sha},
        }
        rows = [
            (
                n,
                "collect",
                "signin/CreateOAuth2Token" if n == 500 else "sts/GetCallerIdentity",
                int(n == 500),
                200,
                1,
            )
            for n in range(500, 609)
        ]
        with self.database(self.parent / "campaign.sqlite") as db:
            db.executescript(
                "create table config(k text primary key,v text); create table attempts(n integer primary key,phase text,op text,write integer,status integer,outcome integer);"
            )
            db.executemany(
                "insert into config values(?,?)",
                [(k, json.dumps(v)) for k, v in c.items()],
            )
            db.executemany("insert into attempts values(?,?,?,?,?,?)", rows)
        projection = self.docs / "attempts.jsonl"
        projection.write_text(
            "".join(
                json.dumps(
                    dict(
                        zip(
                            [
                                "ordinal",
                                "phase",
                                "operation",
                                "write",
                                "status",
                                "outcomeRecorded",
                            ],
                            row,
                        )
                    )
                )
                + "\n"
                for row in rows
            )
        )
        sections = {}
        for section in ["readerSnapshot", "preview"]:
            path = self.docs / (section + ".json")
            sha = self.write(path, {"synthetic": True})
            sections[section] = {
                "record": str(path.relative_to(self.root)),
                "recordSha256": sha,
            }
        record = {
            "accounting": {"cumulative": 608},
            "releaseSourceCommit": SOURCE,
            "toolingCommit": COMMIT,
            "manifestSha256": self.pins["manifest"],
            "scopeSha256": self.pins["scope"],
            "attemptProjection": {
                "path": str(projection.relative_to(self.root)),
                "sha256": digest(projection.read_bytes()),
            },
            "authorizationSha256": auth_sha,
            **sections,
        }
        self.record = self.docs / "project-environments-collection-499-f1.json"
        self.pins["completedParent608"] = self.write(self.record, record)

    @contextlib.contextmanager
    def database(self, path):
        with contextlib.closing(sqlite3.connect(path)) as db, db:
            yield db

    def write(self, path, value):
        raw = canonical(value)
        private_write(path, raw)
        return digest(raw)

    def check_entrypoint(self):
        def checked():
            verify(self.root, self.base, self.pins, SCOPE)
            return {}, SCOPE, self.pins

        with (
            patch.object(driver, "check", side_effect=checked),
            patch.object(driver, "pending_reads", return_value=[]),
            patch.object(sys, "argv", ["driver.py", "check"]),
            patch.object(
                socket, "socket", side_effect=AssertionError("network-disabled")
            ),
            patch.object(
                socket, "getaddrinfo", side_effect=AssertionError("network-disabled")
            ),
            contextlib.redirect_stdout(io.StringIO()),
        ):
            driver.main()

    def test_completed_parent_is_verified_read_only_even_after_old_window_expired(self):
        self.case()
        before = {
            p: digest(p.read_bytes()) for p in self.root.rglob("*") if p.is_file()
        }
        self.check_entrypoint()
        self.assertEqual(
            before,
            {p: digest(p.read_bytes()) for p in self.root.rglob("*") if p.is_file()},
        )
        self.assertFalse((self.parent / "campaign.sqlite-wal").exists())

    def test_config_count_active_phase_closed_window_or_tooling_drift_rejected(self):
        for key, value in [
            ("count", 607),
            ("nextPhase", 2),
            ("activePhase", "deploy"),
            ("closedDeadline", 1800),
            ("reviewCommit", "f" * 40),
            ("deadline", 5000),
        ]:
            with self.subTest(key=key):
                self.case()
                with self.database(self.parent / "campaign.sqlite") as db:
                    db.execute(
                        "update config set v=? where k=?", (json.dumps(value), key)
                    )
                with self.assertRaises(Held):
                    self.check_entrypoint()

    def test_claim_capture_proof_and_attempt_tampering_rejected(self):
        for rel in [
            "window-from-499-fresh-1.json",
            "release-499-f1/initial-checkpoint.json",
            "release-499-f1/collect-proof.json",
        ]:
            with self.subTest(rel=rel):
                self.case()
                path = self.base / rel
                v = json.loads(path.read_bytes())
                v["syntheticTamper"] = True
                path.write_bytes(canonical(v))
                with self.assertRaises(Held):
                    self.check_entrypoint()
        self.case()
        with self.database(self.parent / "campaign.sqlite") as db:
            db.execute("update attempts set outcome=0 where n=608")
        with self.assertRaises(Held):
            self.check_entrypoint()

    def test_record_pin_and_pending_wal_rejected(self):
        self.case()
        self.record.write_bytes(self.record.read_bytes() + b" ")
        with self.assertRaises(Held):
            self.check_entrypoint()
        self.case()
        (self.parent / "campaign.sqlite-wal").write_bytes(b"pending")
        with self.assertRaises(Held):
            self.check_entrypoint()


if __name__ == "__main__":
    unittest.main()
