"""Frozen inventory135 + stopped preview30 = cumulative165, never a reset."""

import hashlib
import json

INVENTORY_SHA = "1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171"
PREVIEW_SHA = "e65635f9b3e21f406cb70dcd0ba7ae71f952e203e54d30f6474da591abe525eb"
PRIOR = 165
MAXIMUM = 500


def baseline(root):
    inventory = root / "docs/development/project-environments-inventory-attempts.jsonl"
    preview = root / "docs/development/project-environments-preview-attempts.jsonl"
    if hashlib.sha256(inventory.read_bytes()).hexdigest() != INVENTORY_SHA:
        raise ValueError("inventory-ledger-hash")
    if hashlib.sha256(preview.read_bytes()).hexdigest() != PREVIEW_SHA:
        raise ValueError("preview-ledger-hash")
    old = [json.loads(x) for x in inventory.read_text().splitlines()]
    new = [json.loads(x) for x in preview.read_text().splitlines()]
    old_calls = [r["count"] for r in old if r["event"] == "attempt"]
    new_calls = [r["count"] for r in new if r["event"] == "attempt"]
    if old_calls != list(range(1, 136)) or new_calls != list(range(136, 166)):
        raise ValueError("ledger-sequence")
    if (
        old[-1]["event"] != "stop"
        or new[-1]["event"] != "stop"
        or new[-1]["count"] != PRIOR
    ):
        raise ValueError("stopped-ledger-required")
    return {
        "prior": PRIOR,
        "maximum": MAXIMUM,
        "inventoryLedgerSha256": INVENTORY_SHA,
        "stoppedPreviewLedgerSha256": PREVIEW_SHA,
    }
