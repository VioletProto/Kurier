"""Read-only verification of the completed 608 parent; never resume its ledger."""

import json
import sqlite3
from contextlib import closing

from checkpoints import SOURCE, fresh_policy, private_json
from common import Held, digest


def verify(root, base, pins, scope):
    def sha(path):
        return digest(path.read_bytes())

    record_path = root / "docs/development/project-environments-collection-499-f1.json"
    if sha(record_path) != pins["completedParent608"]:
        raise Held("completed-parent-record-held")
    record = json.loads(record_path.read_bytes())
    directory = base / "release-499-f1"
    if directory.resolve() != directory.absolute():
        raise Held("completed-parent-location-held")
    wal = directory / "campaign.sqlite-wal"
    db_path = directory / "campaign.sqlite"
    if db_path.is_symlink() or (wal.exists() and wal.stat().st_size):
        raise Held("completed-parent-pending-or-aliased-ledger-held")
    with closing(
        sqlite3.connect("file:" + str(db_path) + "?mode=ro&immutable=1", uri=True)
    ) as db:
        c = {k: json.loads(v) for k, v in db.execute("SELECT k,v FROM config")}
        rows = db.execute(
            "SELECT n,phase,op,write,status,outcome FROM attempts ORDER BY n"
        ).fetchall()
    attempts_path = root / record["attemptProjection"]["path"]
    projected = [json.loads(line) for line in attempts_path.read_text().splitlines()]
    expected_rows = [
        (
            x["ordinal"],
            x["phase"],
            x["operation"],
            int(x["write"]),
            x["status"],
            int(x["outcomeRecorded"]),
        )
        for x in projected
    ]
    auth, auth_sha = private_json(base / "authorization-from-499-fresh-1.json")
    claim, _ = private_json(base / "window-from-499-fresh-1.json")
    if claim != {
        "authorizationSha256": auth_sha,
        "directory": str(directory),
        "checkpointSha256": None,
        "campaignId": "release-499-f1",
        "checkpointPolicy": fresh_policy(scope),
    }:
        raise Held("completed-parent-claim-held")
    checkpoint_path = directory / "initial-checkpoint.json"
    checkpoint, checkpoint_sha = private_json(checkpoint_path)
    accepted, _ = private_json(directory / "initial-checkpoint.json.accepted")
    proof_path = directory / "collect-proof.json"
    proof = json.loads(proof_path.read_bytes())
    if (
        record["accounting"]["cumulative"] != 608
        or record["releaseSourceCommit"] != SOURCE
        or c["prior"] != 499
        or c["count"] != 608
        or c["maximum"] != 1799
        or c["seconds"] != 3300
        or c["deadline"] != c["started"] + 3300
        or c["stopped"]
        or c["activePhase"] is not None
        or c["nextPhase"] != 1
        or c["closedDeadline"] is not None
        or c["phaseDeadline"] is not None
        or c["reviewCommit"] != "bf1effb2835ae6be00bdbd753a802dc0b9bf6741"
        or c["reviewCommit"] != record["toolingCommit"]
        or any(
            c[key] != pins[pin] or record[key] != pins[pin]
            for key, pin in [("manifestSha256", "manifest"), ("scopeSha256", "scope")]
        )
        or sha(attempts_path) != record["attemptProjection"]["sha256"]
        or rows != expected_rows
        or [r[0] for r in rows] != list(range(500, 609))
        or any(
            r[1] != "collect"
            or not r[5]
            or r[4] not in {200, 404}
            or (r[3] and r[2] != "signin/CreateOAuth2Token")
            for r in rows
        )
        or rows[:2]
        != [
            (500, "collect", "signin/CreateOAuth2Token", 1, 200, 1),
            (501, "collect", "sts/GetCallerIdentity", 0, 200, 1),
        ]
        or sum(r[2] == "signin/CreateOAuth2Token" for r in rows) != 1
        or auth_sha != c["authorizationSha256"]
        or auth_sha != record["authorizationSha256"]
        or auth_sha != claim["authorizationSha256"]
        or auth_sha != checkpoint["authorizationSha256"]
        or auth.get("authorized") is not True
        or auth["prior"] != 499
        or auth["maximum"] != 1799
        or auth["seconds"] != 3300
        or auth["proceduralAllowanceUsd"] != 1
        or auth["reviewCommit"] != c["reviewCommit"]
        or auth["manifestSha256"] != pins["manifest"]
        or auth["scopeSha256"] != pins["scope"]
        or any(
            x["campaignId"] != "release-499-f1" for x in [auth, claim, checkpoint, c]
        )
        or any(
            x["checkpointPolicy"] != fresh_policy(scope)
            for x in [auth, claim, checkpoint, c]
        )
        or claim["directory"] != str(directory)
        or auth["checkpointSha256"] is not None
        or claim["checkpointSha256"] is not None
        or checkpoint_sha != c["checkpointSha256"]
        or checkpoint_sha != accepted["checkpointSha256"]
        or (directory / "initial-checkpoint.json.rejected").exists()
        or not c.get("freshCaptureCompleted")
        or not c.get("freshCaptureStarted")
        or c.get("credentialPhase") != "collect"
        or c.get("authenticationFailure") is not None
        or c.get("checkpointCaptureFailure") is not None
        or checkpoint["toolingCommit"] != c["reviewCommit"]
        or checkpoint["manifestSha256"] != pins["manifest"]
        or checkpoint["scopeSha256"] != pins["scope"]
        or c["cacheStat"]
        != [c["cacheMetadata"][k] for k in ("inode", "size", "mtimeNs")]
        or sha(proof_path) != c["proof:collect"]["sha256"]
        or proof.get("allFourFreshArchivesMatched") is not True
        or proof["bundle"]
        != json.loads((directory / "collect-readers/private-output.txt").read_bytes())
    ):
        raise Held("completed-parent-bindings-or-accounting-held")
    for section in ["readerSnapshot", "preview"]:
        if sha(root / record[section]["record"]) != record[section]["recordSha256"]:
            raise Held("completed-parent-projection-held")
    return {"cacheStat": c["cacheStat"], "cacheMetadata": c["cacheMetadata"]}
