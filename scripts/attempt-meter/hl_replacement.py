"""Explicit reviewed replacement: retain counters, start only the newly authorized clock."""

import hashlib
import json
from pathlib import Path

LEDGER_SHA256 = "5672e5f765b66646d4812f50f2ade0c6abd06f2070cb6babb85ac65d261e2495"
RECEIPT_SHA256 = "ef9200cbbd36beee38008896a6d0986fd675af052d243dfe9e62bafa5c06ac5a"
COUNTS = {"api": 31, "aws": 173, "poll": 0, "static": 9}


def parent_proof(ledger_path, receipt_path):
    raw = Path(ledger_path).read_bytes()
    receipt_raw = Path(receipt_path).read_bytes()
    if (
        hashlib.sha256(raw).hexdigest() != LEDGER_SHA256
        or hashlib.sha256(receipt_raw).hexdigest() != RECEIPT_SHA256
    ):
        raise ValueError("Exact accepted stopped ledger and cleanup receipt required")
    rows = [json.loads(x) for x in raw.splitlines()]
    attempts = [x for x in rows if x["event"] == "attempt"]
    outcomes = {x["attempt"]: x for x in rows if x["event"] == "outcome"}
    if (
        len(attempts) != 159
        or len(outcomes) != 159
        or attempts[-1]["counts"] != COUNTS
        or any(x["event"] == "stop" for x in rows)
    ):
        raise ValueError("Parent accounting/stop mismatch")
    if any(
        x["sequence"] not in outcomes
        or outcomes[x["sequence"]].get("transportError")
        or outcomes[x["sequence"]].get("status")
        not in (200, 201, 202, 204, 400, 401, 404)
        for x in attempts
    ):
        raise ValueError("Unknown parent outcome")
    r = json.loads(receipt_raw)
    c = r["coordinator"]
    u = c["users"]
    p = c["projects"]
    if (
        not c.get("cleanupComplete")
        or c["admittedSteps"] != 18
        or c["expectedSends"] != 16
        or c["runs"]
        or len(u) != 1
        or len(p) != 1
        or not all(
            u[0].get(k) for k in ["globalSignedOut", "deleted", "providerAbsent"]
        )
        or not p[0].get("cleaned")
    ):
        raise ValueError("Parent must be normally cleaned with zero H-L admissions")
    return {
        "sha256": LEDGER_SHA256,
        "receiptSha256": RECEIPT_SHA256,
        "counts": COUNTS.copy(),
        "historicalFirstAttemptAt": r["meter"]["firstOriginalAttemptAt"],
        "historicalLastOutcomeAt": r["meter"]["lastOutcomeAt"],
        "authorization": "accepted4803bd9: one replacement owner/project, new120min clock, counters retained",
        "additionalProceduralUsd": 1,
        "historicalUnusedBalanceClaimed": False,
    }
