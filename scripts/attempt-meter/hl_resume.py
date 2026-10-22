"""Explicitly authorized continuation of the exact read-only H-L prerequisite stop."""

import hashlib
import json
from pathlib import Path

PARENT_SHA256 = "50318328c26aecc98f776e41768931d3d693c23a3b103c8bbf2f7b5282fdece1"


def parent_proof(path):
    raw = Path(path).read_bytes()
    if hashlib.sha256(raw).hexdigest() != PARENT_SHA256:
        raise ValueError("Exact published read-only H-L parent required")
    rows = [json.loads(x) for x in raw.splitlines()]
    attempts = [x for x in rows if x["event"] == "attempt"]
    outcomes = {x["attempt"]: x for x in rows if x["event"] == "outcome"}
    if (
        rows[0].get("purpose") != "H-L-only-continuation"
        or len(attempts) != 54
        or any(x["event"] == "stop" for x in rows)
    ):
        raise ValueError("Read-only preflight parent required")
    if any(
        x["write"]
        or x["sequence"] not in outcomes
        or outcomes[x["sequence"]].get("status") != 200
        or outcomes[x["sequence"]].get("transportError")
        for x in attempts
    ):
        raise ValueError("Unknown outcome or write prohibits this continuation")
    return {
        "sha256": PARENT_SHA256,
        "firstAttemptAt": attempts[0]["at"],
        "counts": attempts[-1]["counts"],
    }
