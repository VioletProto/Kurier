"""Frozen inventory135 + preview30 + two uncertain refreshes =167, never a reset."""

import hashlib
import json

INVENTORY_SHA = "1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171"
PREVIEW_SHA = "e65635f9b3e21f406cb70dcd0ba7ae71f952e203e54d30f6474da591abe525eb"
AUTH_SHA = "5316b2ec89fd7ff682f175a6cd14008a840fe2438814c368ab15a05023b997d7"
CLAIM_SHA = "87e39ae4b58c5942d951a7c12b5f5534e23c6f8fc64a8fc16db591921f5bd108"
PROJECTION_SHA = "a70542adbdab540682f958d826289ca70191b97f0e00cdf28bc0a71d07f42c99"
AUTH4_SHA = "0bb82bf9b14cd14a95861424bf3103ebb13bdf8218c08cf5444965837a72de69"
CLAIM4_SHA = "68f93672420d6ffff27c74417351516e339687e1ea4384e6a646bd63a619039a"
PROJECTION4_SHA = "e8d05b3663a499f8225bc3d68d1dd74d4ea4b4fed34d5418ccb1950a684b33b1"
PRIOR = 167
MAXIMUM = 500
FROZEN_FILES = {
    "docs/development/project-environments-inventory-attempts.jsonl": INVENTORY_SHA,
    "docs/development/project-environments-preview-attempts.jsonl": PREVIEW_SHA,
    "docs/development/project-environments-refresh-preview-auth-projection.jsonl": PROJECTION_SHA,
    ".build/project-environments-preview/review-3/authentication.jsonl": AUTH_SHA,
    ".build/project-environments-preview/review-3/window.json": "19c362f32d5ce9e0ea4c8ad67c9f76e8ee03130f58c3e257aadbcdf1aa4c8f39",
    ".build/project-environments-preview/review-3/cumulative-usage.json": "346fa66374bac7ac87fd48ef250e4eb438d8d3ad725ca23876f9a7cacf684671",
    ".build/project-environments-preview/window-from-165.json": CLAIM_SHA,
    "docs/development/project-environments-refresh-preview-review-4-auth-projection.jsonl": PROJECTION4_SHA,
    ".build/project-environments-preview/review-4/authentication.jsonl": AUTH4_SHA,
    ".build/project-environments-preview/review-4/window.json": "538921f5c442ae7b2f3492267c62c8fe55bf1cd3f060e34d486923cde60b911e",
    ".build/project-environments-preview/review-4/cumulative-usage.json": "1cca6aaeef18233ab7db538f4560684375167c731a8182b04756c7e2c5595765",
    ".build/project-environments-preview/window-from-166.json": CLAIM4_SHA,
}


def baseline(root):
    inventory = root / "docs/development/project-environments-inventory-attempts.jsonl"
    preview = root / "docs/development/project-environments-preview-attempts.jsonl"
    if hashlib.sha256(inventory.read_bytes()).hexdigest() != INVENTORY_SHA:
        raise ValueError("inventory-ledger-hash")
    if hashlib.sha256(preview.read_bytes()).hexdigest() != PREVIEW_SHA:
        raise ValueError("preview-ledger-hash")
    for name, expected in FROZEN_FILES.items():
        if hashlib.sha256((root / name).read_bytes()).hexdigest() != expected:
            raise ValueError("frozen-accounting-proof-hash")
    old = [json.loads(x) for x in inventory.read_text().splitlines()]
    new = [json.loads(x) for x in preview.read_text().splitlines()]
    old_calls = [r["count"] for r in old if r["event"] == "attempt"]
    new_calls = [r["count"] for r in new if r["event"] == "attempt"]
    if old_calls != list(range(1, 136)) or new_calls != list(range(136, 166)):
        raise ValueError("ledger-sequence")
    if (
        old[-1]["event"] != "stop"
        or new[-1]["event"] != "stop"
        or new[-1]["count"] != 165
    ):
        raise ValueError("stopped-ledger-required")
    failed = root / ".build/project-environments-preview/review-3"
    auth = [
        json.loads(x)
        for x in (failed / "authentication.jsonl").read_text().splitlines()
    ]
    if (
        [r["event"] for r in auth]
        != ["created", "shared-window", "attempt", "outcome", "stop"]
        or auth[0]["prior"] != 165
        or auth[2]["count"] != 166
        or (auth[2]["service"], auth[2]["operation"]) != ("signin", "CreateOAuth2Token")
        or auth[3]["attempt"] != 166
        or auth[3]["status"] is not None
        or auth[3]["transportError"] is not True
        or auth[4]["count"] != 166
        or auth[4]["reason"] != "transport-failure"
        or (failed / "handoff.json").exists()
        or (failed / "provider-claim.json").exists()
    ):
        raise ValueError("failed-authentication-proof")
    failed4 = root / ".build/project-environments-preview/review-4"
    auth4 = [
        json.loads(x)
        for x in (failed4 / "authentication.jsonl").read_text().splitlines()
    ]
    if (
        [r["event"] for r in auth4]
        != [
            "created",
            "shared-window",
            "attempt",
            "authentication-transport-error",
            "outcome",
            "stop",
        ]
        or auth4[0]["prior"] != 166
        or auth4[2]["count"] != PRIOR
        or (auth4[2]["service"], auth4[2]["operation"])
        != ("signin", "CreateOAuth2Token")
        or auth4[3]["attempt"] != PRIOR
        or auth4[3]["exceptionCategory"] != "connect-timeout"
        or auth4[3]["failurePhase"] != "connect"
        or auth4[3]["errorCode"] is not None
        or auth4[3]["errorCodeDomain"] is not None
        or auth4[4]["attempt"] != PRIOR
        or auth4[4]["status"] is not None
        or auth4[4]["transportError"] is not True
        or auth4[5]["count"] != PRIOR
        or auth4[5]["reason"] != "transport-failure"
        or (failed4 / "handoff.json").exists()
        or (failed4 / "provider-claim.json").exists()
    ):
        raise ValueError("second-failed-authentication-proof")
    return {
        "prior": PRIOR,
        "maximum": MAXIMUM,
        "inventoryLedgerSha256": INVENTORY_SHA,
        "stoppedPreviewLedgerSha256": PREVIEW_SHA,
        "failedAuthenticationLedgerSha256": AUTH_SHA,
        "failedCampaignClaimSha256": CLAIM_SHA,
        "failedAuthenticationProjectionSha256": PROJECTION_SHA,
        "secondFailedAuthenticationLedgerSha256": AUTH4_SHA,
        "secondFailedCampaignClaimSha256": CLAIM4_SHA,
        "secondFailedAuthenticationProjectionSha256": PROJECTION4_SHA,
    }
