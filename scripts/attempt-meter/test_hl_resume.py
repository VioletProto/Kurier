from pathlib import Path
import tempfile
import unittest
from budget import Budget, MeterStop
from hl_resume import parent_proof

ROOT = Path(__file__).resolve().parents[2]


class ResumeTests(unittest.TestCase):
    def test_exact_read_only_parent_cannot_be_substituted(self):
        parent = parent_proof(
            ROOT / "docs/development/workflow-assertions-hl-attempts.jsonl"
        )
        self.assertEqual(
            parent["counts"], {"api": 0, "aws": 54, "poll": 0, "static": 0}
        )
        with tempfile.TemporaryDirectory() as d:
            bad = Path(d) / "bad"
            bad.write_text("{}\n")
            with self.assertRaises(ValueError):
                parent_proof(bad)

    def test_inherited_counts_window_and_ceiling_are_not_reset(self):
        now = [1500.0]
        parent = {
            "firstAttemptAt": 1000.0,
            "counts": {"api": 0, "aws": 54, "poll": 0, "static": 0},
            "sha256": "local-fixture",
        }
        with tempfile.TemporaryDirectory() as d:
            b = Budget(
                Path(d) / "ledger", clock=lambda: now[0], continuation_parent=parent
            )
            try:
                self.assertEqual(b.counts["aws"], 54)
                b.admit("aws", "Query", False)
                self.assertEqual(b.counts["aws"], 55)
                now[0] = 8200.0
                with self.assertRaises(MeterStop):
                    b.admit("aws", "Query", False)
            finally:
                b.close()
            with self.assertRaises(MeterStop):
                Budget(
                    Path(d) / "expired",
                    clock=lambda: now[0],
                    continuation_parent=parent,
                )
