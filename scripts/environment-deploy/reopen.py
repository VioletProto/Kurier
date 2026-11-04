"""Bounded one-shot old-release recovery. Local check/template never contact AWS."""

import argparse
import json
import sqlite3
import subprocess
import sys
from contextlib import closing
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import driver
import recovery
from checkpoints import private_json
from common import Campaign, Held, canonical, digest, private_write
from isolation import launch
from phase_credentials import credentials
from reopen_bindings import (
    AFTER_SHA,
    BEFORE_SHA,
    CLAIM,
    ID,
    INSPECTION_TOOL,
    PROPOSAL_SHA,
    SOURCE,
    TRANSACTION_SHA,
)
from reopen_guard import verify_arm

PINS = HERE / "reopen-pins.json"


def bindings():
    parent = recovery.reconcile()
    for relative, expected in json.loads(PINS.read_bytes()).items():
        if driver.sha(driver.ROOT / relative) != expected:
            raise Held("accepted-reopen-public-record-held")
    p = driver.BASE / recovery.ID
    if p.resolve() != p or (p / "campaign.sqlite").is_symlink():
        raise Held("reopen-inspection-path-held")
    wal = p / "campaign.sqlite-wal"
    if wal.exists() and wal.stat().st_size:
        raise Held("reopen-pending-inspection-held")
    with closing(
        sqlite3.connect(
            "file:" + str(p / "campaign.sqlite") + "?mode=ro&immutable=1", uri=True
        )
    ) as db:
        c = {k: json.loads(v) for k, v in db.execute("select k,v from config")}
        rows = db.execute(
            "select n,phase,op,write,status,outcome from attempts order by n"
        ).fetchall()
    report = json.loads(
        (
            driver.ROOT
            / "docs/development/project-environments-recovery-inspect-868-r1.json"
        ).read_bytes()
    )
    projected = [
        json.loads(line)
        for line in (driver.ROOT / report["attemptProjection"]["path"])
        .read_text()
        .splitlines()
    ]
    expected = [
        (
            r["ordinal"],
            r["phase"],
            r["operation"],
            int(r["write"]),
            r["status"],
            int(r["outcomeRecorded"]),
        )
        for r in projected
    ]
    if (
        rows != expected
        or len(rows) != 87
        or c["count"] != 955
        or c["prior"] != 868
        or c["maximum"] != 1068
        or c["seconds"] != 600
        or c["deadline"] != c["started"] + 600
        or c["activePhase"] is not None
        or c["nextPhase"] != 1
        or c["stopped"]
        or not c["inspectionOnly"]
        or c["closedDeadline"] is not None
        or c["reviewCommit"] != INSPECTION_TOOL
    ):
        raise Held("reopen-inspection-accounting-held")
    proof_path = p / "inspection-proof.json"
    proof, proof_sha = private_json(proof_path)
    if (
        proof_sha != report["inspectionProofSha256"]
        or proof_sha != c["proof:collect"]["sha256"]
        or proof["originalParent"] != parent
    ):
        raise Held("reopen-inspection-proof-held")
    proposal, proposal_sha = private_json(p / "proposed-reopen.json")
    transaction, transaction_sha = private_json(p / "proposed-reopen-transaction.json")
    if (
        proposal_sha != PROPOSAL_SHA
        or transaction_sha != TRANSACTION_SHA
        or digest(canonical(transaction)) != TRANSACTION_SHA
        or proposal["transaction"] != transaction
        or digest(canonical(proposal["before"])) != BEFORE_SHA
        or digest(canonical(proposal["after"])) != AFTER_SHA
        or proposal["accountingSha256"] != parent["accountingSha256"]
        or proposal["executionAuthorized"] is not False
    ):
        raise Held("accepted-reopen-bytes-held")
    auth, auth_sha = private_json(driver.BASE / recovery.AUTH)
    claim, _ = private_json(driver.BASE / recovery.CLAIM)
    cp, cp_sha = private_json(p / "initial-checkpoint.json")
    accepted, _ = private_json(p / "initial-checkpoint.json.accepted")
    if (
        auth_sha != c["authorizationSha256"]
        or auth_sha != claim["authorizationSha256"]
        or auth_sha != cp["authorizationSha256"]
        or cp_sha != c["checkpointSha256"]
        or cp_sha != accepted["checkpointSha256"]
        or claim["campaignId"] != recovery.ID
        or claim["directory"] != str(p)
        or c["credentialPhase"] != "collect"
        or c["cacheStat"]
        != [c["cacheMetadata"][k] for k in ["inode", "size", "mtimeNs"]]
    ):
        raise Held("reopen-inspection-claim-checkpoint-held")
    scope = json.loads(driver.SCOPE.read_bytes())
    pins = json.loads((driver.HERE / "pins.json").read_bytes())
    if (
        auth
        != {
            **recovery.authorization(scope, pins, parent),
            "authorized": True,
            "reviewCommit": INSPECTION_TOOL,
        }
        or claim
        != {
            "authorizationSha256": auth_sha,
            "campaignId": recovery.ID,
            "directory": str(p),
            "inspectionOnly": True,
        }
        or any(
            c[k] != pins[pin]
            for k, pin in [("manifestSha256", "manifest"), ("scopeSha256", "scope")]
        )
    ):
        raise Held("reopen-inspection-scope-held")
    protected = [
        driver.BASE / "release-608-t1/campaign.sqlite",
        p / "campaign.sqlite",
        p / "inspection-proof.json",
        p / "proposed-reopen.json",
        p / "proposed-reopen-transaction.json",
        p / "initial-checkpoint.json",
        p / "initial-checkpoint.json.accepted",
        driver.BASE / recovery.AUTH,
        driver.BASE / recovery.CLAIM,
    ]
    return {
        **parent,
        "inspectionLedgerSha256": driver.sha(p / "campaign.sqlite"),
        "inspectionProofSha256": proof_sha,
        "fixedProposal": proposal,
        "cacheStat": c["cacheStat"],
        "cacheMetadata": c["cacheMetadata"],
        "protectedEvidence": {
            str(f.relative_to(driver.ROOT)): driver.sha(f) for f in protected
        },
    }


def authorization(scope, pins, bound):
    checkpoint = digest(
        canonical(
            {
                "cacheStat": bound["cacheStat"],
                "cacheMetadata": bound["cacheMetadata"],
                "identityArn": scope["identityArn"],
                "account": scope["account"],
            }
        )
    )
    return {
        "authorized": False,
        "action": "reopen-unchanged-old-release-once",
        "campaignId": ID,
        "reviewCommit": driver.git("rev-parse", "HEAD"),
        "releaseSourceCommit": SOURCE,
        "manifestSha256": pins["manifest"],
        "scopeSha256": pins["scope"],
        "proposalSha256": PROPOSAL_SHA,
        "transactionSha256": TRANSACTION_SHA,
        "beforeMarkerSha256": BEFORE_SHA,
        "afterMarkerSha256": AFTER_SHA,
        "inspectionToolingCommit": INSPECTION_TOOL,
        "inspectionLedgerSha256": bound["inspectionLedgerSha256"],
        "inspectionProofSha256": bound["inspectionProofSha256"],
        "originalLedgerSha256": bound["originalLedgerSha256"],
        "checkpointPolicy": {
            "mode": "strict-carry-from-consumed-inspection",
            "checkpointSha256": checkpoint,
            "identityArn": scope["identityArn"],
            "account": scope["account"],
            "freshCaptureAllowed": False,
        },
        "prior": 955,
        "maximum": 1205,
        "maximumAdditionalAttempts": 250,
        "cumulativeCeiling": 1799,
        "seconds": 600,
        "maximumRenewals": 1,
        "mandatoryFreshSts": True,
        "stageWriteLimit": 1,
        "beforeProofMaximumAgeSeconds": 240,
        "minimumWriteTimeReserveSeconds": 60,
        "postWriteAttemptReserve": 32,
        "proceduralAllowanceUsd": 1,
    }


def child(c, name, action, context, values):
    path = c.directory / (name + "-context.json")
    private_write(path, canonical(context))
    directory = c.directory / name
    outcome = launch(
        driver.SOURCE_DIR,
        directory,
        ["node", str(HERE / "reopen.mjs"), action, str(driver.CHILD_CONTEXT)],
        values,
        campaign=c,
        phase="recover-old",
        policy_context=(
            {
                "closedMarker": context["expectedMarker"],
                "transaction": context["fixedProposal"]["transaction"],
            }
            if action == "execute"
            else {"closedMarker": context["expectedMarker"]}
        ),
        extra_ro=driver.mounts(path),
    )
    if outcome["exitCode"] or c.get("stopped"):
        raise Held("reopen-child-held-no-retry")
    summary = json.loads((directory / "proxy-summary.json").read_bytes())
    if not summary["identityVerified"]:
        raise Held("reopen-child-identity-held")
    return json.loads((directory / "private-output.txt").read_bytes()), summary


def arm(c, bound, context, bundle):
    c.check()
    driver.check()
    if bindings() != bound:
        raise Held("reopen-source-evidence-changed")
    if driver.git("status", "--porcelain", "--untracked-files=all"):
        raise Held("reopen-tooling-drift")
    snapshot = c.directory / "before/readers-output.json"
    private_write(snapshot, canonical(bundle))
    file = c.directory / "assess-input.json"
    private_write(file, canonical({**context, "bundle": bundle}))
    checked = subprocess.run(
        ["node", str(HERE / "reopen.mjs"), "assess", str(file)],
        capture_output=True,
        check=False,
        timeout=max(0.1, min(30, c.get("deadline") - c.clock())),
    )
    if checked.returncode:
        raise Held("reopen-fresh-readiness-held")
    verified = json.loads(checked.stdout)
    if verified != {
        "markerSha256": BEFORE_SHA,
        "accountingSha256": bound["accountingSha256"],
        "oldReleaseVerified": True,
    }:
        raise Held("reopen-fresh-attestation-held")
    c.check()
    now = c.clock()
    deadline = min(c.get("deadline"), now + 240)
    proof = {
        "authorizationSha256": c.get("authorizationSha256"),
        "transactionSha256": TRANSACTION_SHA,
        "beforeMarkerSha256": BEFORE_SHA,
        "accountingSha256": bound["accountingSha256"],
        "snapshotSha256": driver.sha(snapshot),
        "verifiedAt": now,
        "deadline": deadline,
        "meterCount": c.get("count"),
    }
    path = c.directory / "before-write-proof.json"
    private_write(path, canonical(proof))
    c.put("reopenBeforeProofSha256", driver.sha(path))
    c.put("reopenArmDeadline", deadline)
    c.put("reopenWriteArmed", True)
    verify_arm(c)


def start(auth_path, scope, pins, bound):
    auth, auth_sha = private_json(auth_path)
    if auth != {**authorization(scope, pins, bound), "authorized": True}:
        raise Held("explicit-one-shot-reopen-approval-required")
    directory = driver.BASE / ID
    if directory.exists() or (driver.BASE / CLAIM).exists():
        raise Held("one-shot-reopen-claim-consumed")
    if driver.git("status", "--porcelain", "--untracked-files=all"):
        raise Held("clean-reopen-tooling-required")
    private_write(
        driver.BASE / CLAIM,
        canonical(
            {
                "authorizationSha256": auth_sha,
                "campaignId": ID,
                "directory": str(directory),
                "transactionSha256": TRANSACTION_SHA,
            }
        ),
    )
    c = Campaign(
        directory,
        recovery_write=True,
        create={
            k: auth[k]
            for k in [
                "prior",
                "maximum",
                "seconds",
                "manifestSha256",
                "scopeSha256",
                "reviewCommit",
            ]
        }
        | {"authorizationSha256": auth_sha},
    )
    try:
        c.put("campaignId", ID)
        c.put("protectedEvidence", bound["protectedEvidence"])
        c.begin("recover-old")
        c.put("cacheStat", bound["cacheStat"])
        c.put("cacheMetadata", bound["cacheMetadata"])
        values = credentials(c, scope)
        context = {
            **{
                k: bound[k]
                for k in [
                    "expectedMarker",
                    "accountingSha256",
                    "stateCutoffUtc",
                    "fixedProposal",
                ]
            }
        }
        bundle, _ = child(c, "before", "before", context, values)
        arm(c, bound, context, bundle)
        result, summary = child(c, "write", "execute", context, values)
        writes = c.db.execute(
            "select status,outcome from attempts where op='dynamodb/TransactWriteItems'"
        ).fetchall()
        if (
            writes != [(200, 1)]
            or result.get("stageWrites") != 1
            or digest(canonical(result.get("marker"))) != AFTER_SHA
            or result.get("accountingSha256") != bound["accountingSha256"]
            or summary["lastMarkerSha256"] != AFTER_SHA
        ):
            raise Held("reopen-readback-proof-held-no-retry")
        if bindings() != bound:
            raise Held("reopen-parent-mutated-held")
        path = c.directory / "reopen-proof.json"
        private_write(
            path,
            canonical(
                {
                    "result": result,
                    "transactionSha256": TRANSACTION_SHA,
                    "beforeProofSha256": c.get("reopenBeforeProofSha256"),
                    "originalEvidence": bound["protectedEvidence"],
                }
            ),
        )
        c.finish("recover-old", {"sha256": driver.sha(path)})
        print(
            json.dumps(
                {
                    "reopenedOldRelease": True,
                    "state": "active",
                    "version": 17,
                    "stageTransactions": 1,
                    "cumulative": c.get("count"),
                    "automaticContinuation": False,
                }
            )
        )
    except BaseException:
        try:
            c.stop("reopen-failed-or-uncertain-no-retry")
        except Held:
            pass
        raise
    finally:
        c.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["check", "template", "start"])
    parser.add_argument("--authorization", type=Path)
    a = parser.parse_args()
    _release, scope, pins = driver.check()
    bound = bindings()
    if a.action == "check":
        print(
            json.dumps(
                {
                    "localChecksPassed": True,
                    "prior": 955,
                    "maximumAdditionalAttempts": 250,
                    "seconds": 600,
                    "stageWriteLimit": 1,
                    "freshCaptureAllowed": False,
                    "awsCalls": 0,
                }
            )
        )
        return
    if a.action == "template":
        print(json.dumps(authorization(scope, pins, bound), indent=2))
        return
    if a.authorization is None:
        raise Held("reopen-authorization-required")
    start(
        driver.required_path(a.authorization.absolute(), file=True), scope, pins, bound
    )


if __name__ == "__main__":
    try:
        main()
    except Exception:  # noqa: BLE001 - closed diagnostics only
        raise SystemExit(
            "One-shot recovery held/uncertain. No retry, substitution, reset or repair; inspect retained evidence."
        ) from None
