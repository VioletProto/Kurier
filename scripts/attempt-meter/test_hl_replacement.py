import tempfile
import unittest
from pathlib import Path
from budget import Budget, MeterStop
from hl_replacement import parent_proof, COUNTS

ROOT = Path(__file__).resolve().parents[2]


class ReplacementTests(unittest.TestCase):
    def proof(self):
        return parent_proof(
            ROOT
            / "docs/development/workflow-assertions-hl-continuation-attempts.jsonl",
            ROOT / "docs/development/workflow-assertions-hl-continuation.json",
        )

    def test_exact_accepted_cleanup_and_counters_required(self):
        proof = self.proof()
        self.assertEqual(proof["counts"], COUNTS)
        self.assertFalse(proof["historicalUnusedBalanceClaimed"])
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "changed"
            p.write_text("{}\n")
            with self.assertRaises(ValueError):
                parent_proof(
                    p,
                    ROOT / "docs/development/workflow-assertions-hl-continuation.json",
                )
            with self.assertRaises(ValueError):
                parent_proof(
                    ROOT
                    / "docs/development/workflow-assertions-hl-continuation-attempts.jsonl",
                    p,
                )

    def test_new_clock_preserves_all_counters_and_exact_limits(self):
        proof = self.proof()
        now = [proof["historicalLastOutcomeAt"] + 999999]
        with tempfile.TemporaryDirectory() as d:
            b = Budget(Path(d) / "new", clock=lambda: now[0], replacement_parent=proof)
            try:
                self.assertEqual(b.counts, COUNTS)
                self.assertIsNone(b.started)
                b.admit("aws", "Query", False)
                self.assertEqual(b.counts["aws"], 174)
                now[0] += 7200
                with self.assertRaises(MeterStop):
                    b.admit("aws", "Query", False)
            finally:
                b.close()
            for category, limit in [("aws", 1500), ("api", 500), ("static", 200)]:
                b = Budget(
                    Path(d) / category, clock=lambda: now[0], replacement_parent=proof
                )
                try:
                    for _ in range(limit - COUNTS[category]):
                        b.admit(category, "GET", False)
                    self.assertEqual(b.counts[category], limit)
                    with self.assertRaises(MeterStop):
                        b.admit(category, "GET", False)
                finally:
                    b.close()
