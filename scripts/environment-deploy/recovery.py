"""One separately approved read-only recovery inspection; no reopen executor."""

import argparse
import datetime
import json
import sqlite3
import subprocess
import sys
import uuid
from contextlib import closing
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import driver
from authentication import deadline_alarm
from checkpoints import capture_at_start, fresh_policy, private_json
from common import Campaign, Held, canonical, digest, private_write
from isolation import launch
from phase_credentials import credentials

ID = "recovery-inspect-868-r1"
CLAIM = "recovery-window-from-868-r1.json"
AUTH = "recovery-authorization-from-868-r1.json"
PINS = HERE / "recovery-pins.json"


def reconcile():
    pins = json.loads(PINS.read_bytes())
    for name, sha in pins.items():
        if driver.sha(driver.ROOT / name) != sha:
            raise Held("recovery-parent-record-held")
    parent = driver.BASE / "release-608-t1"
    if parent.resolve() != parent or (parent / "campaign.sqlite").is_symlink():
        raise Held("recovery-parent-path-held")
    wal = parent / "campaign.sqlite-wal"
    if wal.exists() and wal.stat().st_size:
        raise Held("recovery-parent-pending-held")
    with closing(
        sqlite3.connect(
            "file:" + str(parent / "campaign.sqlite") + "?mode=ro&immutable=1", uri=True
        )
    ) as db:
        c = {k: json.loads(v) for k, v in db.execute("select k,v from config")}
        rows = db.execute(
            "select n,phase,op,write,status,outcome from attempts order by n"
        ).fetchall()
    expected = []
    for name in [
        "project-environments-collection-608-t1-attempts.jsonl",
        "project-environments-close-608-t1-attempts.jsonl",
    ]:
        for line in (driver.ROOT / "docs/development" / name).read_text().splitlines():
            r = json.loads(line)
            expected.append(
                (
                    r["ordinal"],
                    r["phase"],
                    r["operation"],
                    int(r["write"]),
                    r["status"],
                    int(r["outcomeRecorded"]),
                )
            )
    record = json.loads(
        (
            driver.ROOT / "docs/development/project-environments-close-608-t1.json"
        ).read_bytes()
    )
    auth, auth_sha = private_json(driver.BASE / "authorization-from-608-timing-1.json")
    claim, _ = private_json(driver.BASE / "window-from-608-timing-1.json")
    checkpoint, checkpoint_sha = private_json(parent / "initial-checkpoint.json")
    accepted, _ = private_json(parent / "initial-checkpoint.json.accepted")
    scope = json.loads(driver.SCOPE.read_bytes())
    if (
        c["reviewCommit"] != "4de31ab9a9c791a1fc224bbce8a7fe79220519ba"
        or c["deadline"] != c["started"] + 3300
        or c["authorizationSha256"] != record["authorizationSha256"]
        or auth_sha != c["authorizationSha256"]
        or auth_sha != checkpoint["authorizationSha256"]
        or auth["checkpointPolicy"] != fresh_policy(scope)
        or checkpoint["checkpointPolicy"] != fresh_policy(scope)
        or checkpoint_sha != c["checkpointSha256"]
        or checkpoint_sha != accepted["checkpointSha256"]
        or claim
        != {
            "authorizationSha256": auth_sha,
            "directory": str(
                driver.ROOT / ".build/project-environments-execution/release-608-t1"
            ),
            "checkpointSha256": None,
            "campaignId": "release-608-t1",
            "checkpointPolicy": fresh_policy(scope),
        }
        or any(
            c[k] != record[k] or auth[k] != record[k]
            for k in ["manifestSha256", "scopeSha256"]
        )
    ):
        raise Held("recovery-original-claim-binding-held")
    if (
        rows != expected
        or any(not x[5] for x in rows)
        or c["count"] != 868
        or c["nextPhase"] != 2
        or c["activePhase"] is not None
        or c["stopped"]
    ):
        raise Held("recovery-parent-ledger-held")
    if (
        c["deadline"] != record["clocks"]["originalCampaignDeadlineMonotonicSeconds"]
        or c["closedDeadline"]
        != record["clocks"]["fixedHostClosedDeadlineMonotonicSeconds"]
    ):
        raise Held("recovery-original-deadlines-held")
    for phase in ["collect", "close"]:
        if (
            driver.sha(parent / (phase + "-proof.json"))
            != c["proof:" + phase]["sha256"]
        ):
            raise Held("recovery-parent-proof-held")
    if driver.sha(parent / "close-proof.json") != record["closedProofSha256"]:
        raise Held("recovery-closure-receipt-held")
    proof = json.loads((parent / "close-proof.json").read_bytes())
    collect = json.loads((parent / "collect-proof.json").read_bytes())
    marker = proof["next"]
    if (
        marker["state"] != {"S": "workflow-migration"}
        or marker["version"] != {"N": "16"}
        or proof["result"]["closure"]["markerSha256"] != digest(canonical(marker))
        or not proof["result"]["accountingPreserved"]
    ):
        raise Held("recovery-closed-marker-held")
    if sorted(
        k
        for k in marker.keys() | collect["bundle"]["marker"].keys()
        if marker.get(k) != collect["bundle"]["marker"].get(k)
    ) != ["state", "version"]:
        raise Held("recovery-capability-drift-held")
    guard = json.loads((parent / "close-minimum-time-check.json").read_bytes())
    if guard != record["minimumTimeEntryGuard"]:
        raise Held("recovery-original-time-proof-held")
    at = datetime.datetime.fromisoformat(guard["checkedUtc"])
    cutoff = at - datetime.timedelta(
        seconds=c["deadline"] - guard["originalRemainingSeconds"] - c["started"]
    )
    return {
        "originalLedgerSha256": driver.sha(parent / "campaign.sqlite"),
        "closeProofSha256": driver.sha(parent / "close-proof.json"),
        "expectedMarker": marker,
        "accountingSha256": proof["result"]["accountingSha256"],
        "stateCutoffUtc": cutoff.isoformat(),
        "originalCampaignDeadline": c["deadline"],
        "originalClosedDeadline": c["closedDeadline"],
    }


def authorization(scope, pins, parent):
    return {
        "authorized": False,
        "action": "read-only-recovery-inspection",
        "campaignId": ID,
        "reviewCommit": driver.git("rev-parse", "HEAD"),
        "releaseSourceCommit": driver.SOURCE,
        "manifestSha256": pins["manifest"],
        "scopeSha256": pins["scope"],
        "originalLedgerSha256": parent["originalLedgerSha256"],
        "closeProofSha256": parent["closeProofSha256"],
        "checkpointPolicy": fresh_policy(scope),
        "prior": 868,
        "maximum": 1068,
        "cumulativeCeiling": 1799,
        "seconds": 600,
        "maximumRenewals": 1,
        "mandatorySts": True,
        "proceduralAllowanceUsd": 1,
        "stageWritesAuthorized": False,
    }


def start(directory, auth_path, scope, pins, parent):
    auth, auth_sha = private_json(auth_path)
    if auth != {**authorization(scope, pins, parent), "authorized": True}:
        raise Held("explicit-recovery-inspection-approval-required")
    if (
        directory != driver.BASE / ID
        or directory.exists()
        or (driver.BASE / CLAIM).exists()
    ):
        raise Held("recovery-claim-consumed-or-location-held")
    if driver.git("status", "--porcelain", "--untracked-files=all"):
        raise Held("clean-recovery-tooling-required")
    private_write(
        driver.BASE / CLAIM,
        canonical(
            {
                "authorizationSha256": auth_sha,
                "directory": str(directory),
                "campaignId": ID,
                "inspectionOnly": True,
            }
        ),
    )
    c = Campaign(
        directory,
        inspection=True,
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
        c.put("checkpointPolicy", auth["checkpointPolicy"])
        c.begin("collect")
        with deadline_alarm(c.get("phaseDeadline")):
            checkpoint = capture_at_start(c, scope)
        c.put("cacheStat", checkpoint["cacheStat"])
        c.put("cacheMetadata", checkpoint["cacheMetadata"])
        values = credentials(c, scope)
        if reconcile() != parent:
            raise Held("recovery-parent-changed-held")
        context = {
            **parent,
            "clientRequestToken": str(
                uuid.uuid5(
                    uuid.NAMESPACE_URL,
                    str(directory)
                    + ":reopen-old:"
                    + digest(canonical(parent["expectedMarker"])),
                )
            ),
        }
        path = directory / "inspection-context.json"
        private_write(path, canonical(context))
        child = directory / "readers"
        outcome = launch(
            driver.SOURCE_DIR,
            child,
            ["node", str(HERE / "recovery.mjs"), "inspect", str(driver.CHILD_CONTEXT)],
            values,
            campaign=c,
            phase="collect",
            policy_context={},
            extra_ro=driver.mounts(path),
        )
        if outcome["exitCode"] or c.get("stopped"):
            raise Held("recovery-reader-held")
        if reconcile() != parent:
            raise Held("recovery-parent-changed-held")
        bundle = json.loads((child / "private-output.txt").read_bytes())
        plan_input = directory / "proposal-input.json"
        private_write(plan_input, canonical({**context, "bundle": bundle}))
        result = subprocess.run(
            ["node", str(HERE / "recovery.mjs"), "plan", str(plan_input)],
            capture_output=True,
            check=False,
        )
        if result.returncode:
            raise Held("recovery-offline-plan-held")
        proposal = json.loads(result.stdout)
        private_write(directory / "proposed-reopen.json", canonical(proposal))
        private_write(
            directory / "inspection-proof.json",
            canonical(
                {
                    "bundle": bundle,
                    "proposalSha256": driver.sha(directory / "proposed-reopen.json"),
                    "originalParent": parent,
                    "stageWriteAuthorized": False,
                }
            ),
        )
        c.finish("collect", {"sha256": driver.sha(directory / "inspection-proof.json")})
        print(
            json.dumps(
                {
                    "inspectionCompleted": True,
                    "cumulative": c.get("count"),
                    "attempts": c.get("count") - 868,
                    "stageWrites": 0,
                    "proposedReopenOnly": True,
                }
            )
        )
    except BaseException:
        try:
            c.stop("recovery-inspection-failed-no-retry")
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
    parent = reconcile()
    if a.action == "check":
        print(
            json.dumps(
                {
                    "localChecksPassed": True,
                    "prior": 868,
                    "maximumAdditionalAttempts": 200,
                    "seconds": 600,
                    "awsCalls": 0,
                    "originalLedgerReconciled": True,
                }
            )
        )
        return
    if a.action == "template":
        print(json.dumps(authorization(scope, pins, parent), indent=2))
        return
    if a.authorization is None:
        raise Held("recovery-authorization-required")
    start(
        driver.BASE / ID,
        driver.required_path(a.authorization.absolute(), file=True),
        scope,
        pins,
        parent,
    )


if __name__ == "__main__":
    try:
        main()
    except Exception:  # noqa: BLE001 - never expose credentials or raw errors
        raise SystemExit(
            "Read-only recovery held; no retry or stage write. Inspect private evidence."
        ) from None
