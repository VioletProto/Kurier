"""Host-only recovery write arm. No request data or credentials in diagnostics."""

import json
import subprocess
from pathlib import Path

from common import ROOT, Held, digest
from reopen_bindings import BEFORE_SHA, SOURCE, TRANSACTION_SHA


def verify_source(c):
    for directory, commit in [
        (ROOT, c.get("reviewCommit")),
        (Path("/tmp/kurier-environments-release-source"), SOURCE),
    ]:
        if subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=directory
        ).decode().strip() != commit or subprocess.check_output(
            ["git", "status", "--porcelain", "--untracked-files=all"], cwd=directory
        ):
            raise Held("reopen-source-or-tooling-changed")


def verify_arm(c):
    c.check()
    verify_source(c)
    path = c.directory / "before-write-proof.json"
    raw = path.read_bytes()
    if digest(raw) != c.get("reopenBeforeProofSha256"):
        raise Held("reopen-before-proof-changed")
    proof = json.loads(raw)
    if (
        not c.get("recoveryWriteOnly")
        or c.get("activePhase") != "recover-old"
        or proof["authorizationSha256"] != c.get("authorizationSha256")
        or proof["transactionSha256"] != TRANSACTION_SHA
        or proof["beforeMarkerSha256"] != BEFORE_SHA
        or proof["verifiedAt"] > c.clock()
        or c.clock() >= proof["deadline"]
        or proof["deadline"] != c.get("reopenArmDeadline")
        or not c.get("reopenWriteArmed")
    ):
        raise Held("reopen-arm-stale-or-unbound")
    snapshot = c.directory / "before/readers-output.json"
    if digest(snapshot.read_bytes()) != proof["snapshotSha256"]:
        raise Held("reopen-snapshot-changed")
    for relative, expected in c.get("protectedEvidence").items():
        path = ROOT / relative
        wal = Path(str(path) + "-wal")
        if path.name == "campaign.sqlite" and wal.exists() and wal.stat().st_size:
            raise Held("reopen-protected-ledger-pending")
        if path.is_symlink() or digest(path.read_bytes()) != expected:
            raise Held("reopen-protected-evidence-changed")
