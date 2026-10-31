"""Reviewed phase-by-phase execution tooling. Default check is local only."""

import argparse
import json
import sqlite3
import subprocess
import sys
import uuid
from contextlib import closing
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE.parent / "preview-meter"))
from artifacts import load_objects
from authentication import sdk_pins
from checkpoints import CAMPAIGN_ID, CLAIM_NAME, private_json
from checkpoints import capture as capture_checkpoint
from checkpoints import load as load_checkpoint
from common import (
    CREDENTIAL_POLICY,
    PRIOR,
    ROOT,
    Campaign,
    Held,
    canonical,
    digest,
    private_write,
)
from credential_diagnostics import projection as authentication_projection
from isolation import launch
from permissions import pending_reads
from phase_credentials import credentials
from run import PROVIDER, PROVIDER_SHA, SOURCE, SST, SST_SHA, git, sha, valid_parent

SOURCE_DIR = Path("/tmp/kurier-environments-release-source")
MANIFEST = ROOT / "docs/development/project-environments-adopted-release.json"
SCOPE = ROOT / "docs/development/project-environments-execution-scope.json"
BASE = ROOT / ".build/project-environments-execution"


def campaign_path(path):
    requested = path if path.is_absolute() else ROOT / path
    resolved = requested.resolve()
    if resolved.parent != BASE.resolve() or requested.absolute() != resolved:
        raise Held("campaign-location-or-alias-held")
    return resolved


def required_path(path, *, file=False):
    path = Path(path)
    if not path.is_absolute() or path.absolute() != path.resolve(strict=True):
        raise Held("required-path-or-alias-held")
    if file and not path.is_file():
        raise Held("required-context-file-held")
    return path


def stopped_parent():
    """Read retained stopped ledgers without writing or replaying them."""
    pins = json.loads((HERE / "pins.json").read_bytes())
    checkpoint = None
    for filename, pin, prior, total, auth_name in [
        (
            "project-environments-collection-stop.json",
            "stoppedParent",
            299,
            301,
            "authorization.json",
        ),
        (
            "project-environments-collection-from-301.json",
            "stoppedParent303",
            301,
            303,
            "authorization-from-301.json",
        ),
        (
            "project-environments-collection-from-303.json",
            "stoppedParent390",
            303,
            390,
            "authorization-from-303.json",
        ),
        (
            "project-environments-collection-from-390.json",
            "stoppedZeroCall390",
            390,
            390,
            "authorization-from-390.json",
        ),
        (
            "project-environments-collection-390-cp1.json",
            "stoppedParent499",
            390,
            499,
            "authorization-from-390-checkpoint-1.json",
        ),
    ]:
        path = ROOT / "docs/development" / filename
        if sha(path) != pins[pin]:
            raise Held("stopped-parent-record-held")
        record = json.loads(path.read_bytes())
        directory = campaign_path(Path(record["campaign"]))
        auth = directory.parent / auth_name
        claim = json.loads(
            (
                directory.parent
                / (
                    "window-from-390-checkpoint-1.json"
                    if total == 499
                    else f"window-from-{prior}.json"
                )
            ).read_bytes()
        )
        wal = directory / "campaign.sqlite-wal"
        if wal.exists() and wal.stat().st_size:
            raise Held("stopped-parent-pending-wal-held")
        with closing(
            sqlite3.connect(
                "file:" + str(directory / "campaign.sqlite") + "?mode=ro&immutable=1",
                uri=True,
            )
        ) as db:
            config = {k: json.loads(v) for k, v in db.execute("SELECT k,v FROM config")}
            rows = db.execute(
                "SELECT n,phase,op,write,status,outcome FROM attempts ORDER BY n"
            ).fetchall()
        if (
            not config["stopped"]
            or config["prior"] != prior
            or config["count"] != total
            or config["nextPhase"] != 0
            or config["closedDeadline"] is not None
            or config["reviewCommit"] != record["toolingCommit"]
            or config["manifestSha256"] != record["manifestSha256"]
            or config["scopeSha256"] != record["scopeSha256"]
            or (
                prior in {299, 301}
                and rows
                != [
                    (prior + 1, "collect", "signin/CreateOAuth2Token", 1, 200, 1),
                    (prior + 2, "collect", "sts/GetCallerIdentity", 0, 200, 1),
                ]
            )
            or (
                prior in {303, 390}
                and total != 390
                and rows
                != [
                    (
                        x["ordinal"],
                        x["phase"],
                        x["operation"],
                        int(x["write"]),
                        x["status"],
                        int(x["outcomeRecorded"]),
                    )
                    for x in (
                        json.loads(line)
                        for line in (ROOT / record["attemptProjection"]["path"])
                        .read_text()
                        .splitlines()
                    )
                ]
            )
            or (prior == 390 and total == 390 and rows != [])
            or sha(auth) != record["authorizationSha256"]
            or config["authorizationSha256"] != record["authorizationSha256"]
            or claim["authorizationSha256"] != record["authorizationSha256"]
            or claim["directory"] != str(directory.resolve())
            or not config.get("cacheStat")
        ):
            raise Held("stopped-parent-accounting-held")
        if (
            prior in {303, 390}
            and total != 390
            and sha(ROOT / record["attemptProjection"]["path"])
            != record["attemptProjection"]["sha256"]
        ):
            raise Held("stopped-parent-projection-held")
        checkpoint = {
            "cacheStat": config["cacheStat"],
            "cacheMetadata": config.get("cacheMetadata"),
        }
        if total == 499:
            checkpoint_path = BASE / "checkpoints/adopted-current-1.json"
            _old_checkpoint, old_sha = private_json(checkpoint_path)
            old_auth = json.loads(auth.read_bytes())
            if (
                old_sha != config.get("checkpointSha256")
                or old_auth.get("checkpointSha256") != old_sha
                or claim.get("checkpointSha256") != old_sha
                or not checkpoint["cacheMetadata"]
                or config.get("credentialPhase") != "collect"
                or config.get("authenticationFailure") is not None
                or checkpoint["cacheStat"]
                != [
                    checkpoint["cacheMetadata"][k] for k in ("inode", "size", "mtimeNs")
                ]
            ):
                raise Held("stopped-parent-checkpoint-held")
    return checkpoint


def check():
    pins = json.loads((HERE / "pins.json").read_text())
    release = json.loads(MANIFEST.read_bytes())
    scope = json.loads(SCOPE.read_bytes())
    if (
        digest(MANIFEST.read_bytes()) != pins["manifest"]
        or digest(SCOPE.read_bytes()) != pins["scope"]
    ):
        raise Held("execution-pins-held")
    read_review = pins["additionalReadReview"]
    approved = ROOT / read_review["path"]
    acl_review = pins["aclReadReview"]
    acl_approved = ROOT / acl_review["path"]
    configuration_review = pins["bucketConfigurationReadReview"]
    configuration_approved = ROOT / configuration_review["path"]
    if (
        sha(approved) != read_review["sha256"]
        or sha(acl_approved) != acl_review["sha256"]
        or sha(configuration_approved) != configuration_review["sha256"]
        or scope.get("approvedAdditionalReads")
        != json.loads(approved.read_bytes())["pendingScopedReads"]
        + json.loads(acl_approved.read_bytes())["pendingScopedReads"]
        + json.loads(configuration_approved.read_bytes())["pendingScopedReads"]
    ):
        raise Held("additional-read-approval-pins-held")
    a = ROOT / release["adoption"]["attestationPath"]
    if (
        digest(a.read_bytes()) != pins["attestation"]
        or release["adoption"]["attestationSha256"] != pins["attestation"]
    ):
        raise Held("attestation-held")
    att = json.loads(a.read_bytes())
    if (
        not att["validation"]["allFourRetainedReview8BlobsAuthenticated"]
        or not att["validation"]["allMatchedRequiredEmptyBindings"]
        or release["status"] != "artifacts-adopted-deployment-held"
        or not release["adoption"]["accepted"]
        or not release["bindingValidation"]["authenticated"]
        or release["releaseSourceCommit"] != SOURCE
    ):
        raise Held("adoption-held")
    candidate = ROOT / release["supersedesCandidateManifestPath"]
    if (
        digest(candidate.read_bytes()) != release["supersedesCandidateManifestSha256"]
        or att["preservation"]["candidateManifestSha256"]
        != release["supersedesCandidateManifestSha256"]
        or json.loads(candidate.read_bytes())["packages"] != release["packages"]
    ):
        raise Held("historical-candidate-held")
    for name, proof in att["preservation"]["nonSensitivePrivateEvidence"].items():
        if sha(ROOT / name) != proof["sha256"]:
            raise Held("key-observation-proof-held")
    if (
        att["accounting"]["cumulativeCalls"] != 299
        or att["accounting"]["newCalls"] != 3
    ):
        raise Held("accounting-held")
    for p in release["packages"]:
        data = (
            ROOT / release["artifactDirectory"] / p["name"] / "code.zip"
        ).read_bytes()
        if (
            digest(data) != p["zipSha256"]
            or len(data) != p["zipBytes"]
            or next(x for x in att["validation"]["packages"] if x["name"] == p["name"])[
                "zipSha256"
            ]
            != p["zipSha256"]
        ):
            raise Held("adopted-bytes-held")
    stopped_parent()
    load_objects(scope, release)
    valid_parent()
    try:
        sdk_pins()
    except Exception:  # noqa: BLE001 - fixed code; no SDK paths/messages
        raise Held("authentication-sdk-pins-held") from None
    import botocore

    for rel, expected in pins["packager"].items():
        if sha(ROOT / rel) != expected:
            raise Held("pinned-packager-held")
    for rel, h in pins["models"].items():
        if sha(Path(botocore.__file__).parent / rel) != h:
            raise Held("execution-model-held")
    if (
        sha(SST) != SST_SHA
        or sha(PROVIDER) != PROVIDER_SHA
        or git("rev-parse", "HEAD", cwd=SOURCE_DIR) != SOURCE
        or git("status", "--porcelain", "--untracked-files=all", cwd=SOURCE_DIR)
    ):
        raise Held("pinned-source-toolchain-held")
    return release, scope, pins


CHILD_CONTEXT = Path("/meter/phase-context.json")


def phase_command(action, context):
    if action not in {
        "collect",
        "diff",
        "close",
        "deploy",
        "verify",
        "activate",
        "final",
        "plan",
    }:
        raise Held("fixed-child-action-held")
    if action == "plan":
        required_path(context, file=True)
    elif context != CHILD_CONTEXT:
        raise Held("fixed-context-destination-held")
    return [
        "node",
        str(required_path(HERE / "phase.mjs", file=True)),
        action,
        str(context),
    ]


def mounts(context=None):
    fixed = [
        HERE,
        ROOT / "scripts/lib/project-environments-activation.mjs",
        MANIFEST,
        SCOPE,
        ROOT / "docs/development/project-environments-inventory.json",
    ]
    original = json.loads(
        (ROOT / "docs/development/project-environments-release.json").read_text()
    )
    fixed.append(ROOT / original["artifactDirectory"] / "frontend")
    result = [(required_path(p), p) for p in fixed]
    if context is not None:
        result.append((required_path(context, file=True), CHILD_CONTEXT))
    return result


def child(c, phase, subphase, context, creds):
    if (phase == "collect" and subphase not in {"collect-readers", "collect-diff"}) or (
        phase != "collect" and subphase != phase
    ):
        raise Held("fixed-child-subphase-held")
    directory = c.directory / subphase
    ctx = c.directory / (subphase + "-context.json")
    required_path(c.directory)
    private_write(ctx, canonical(context))
    context_mounts = mounts(ctx)
    command = phase_command(
        "diff" if subphase == "collect-diff" else phase, CHILD_CONTEXT
    )
    result = launch(
        required_path(SOURCE_DIR),
        directory,
        command,
        creds,
        campaign=c,
        phase=phase,
        policy_context=context,
        extra_ro=context_mounts,
    )
    if result["exitCode"] != 0 or c.get("stopped"):
        raise Held("child-phase-held-no-retry")
    summary = json.loads((directory / "proxy-summary.json").read_bytes())
    if not summary["identityVerified"]:
        raise Held("child-identity-proof-missing")
    return directory, summary


def read_proof(c, phase):
    p = c.directory / (phase + "-proof.json")
    if not p.is_file() or sha(p) != c.get("proof:" + phase)["sha256"]:
        raise Held("phase-proof-held")
    return json.loads(p.read_bytes())


def plan_projection(raw):
    by = {}
    for x in raw:
        k = (x["urn"].split("::")[-1], x["type"])
        v = by.setdefault(k, [set(), set()])
        v[0].add(x["op"])
        v[1].update(x.get("diffs") or [])
    return sorted([list(k) + [sorted(v[0]), sorted(v[1])] for k, v in by.items()])


def stage_options(c, phase):
    first = read_proof(c, "collect")
    bundle = first["bundle"] if phase == "close" else read_proof(c, "verify")["bundle"]
    closure = None if phase == "close" else read_proof(c, "close")["result"]["closure"]
    engine = {
        "action": phase,
        "reviewCommit": c.get("reviewCommit"),
        "manifestSha256": c.get("manifestSha256"),
        "markerSha256": digest(canonical(bundle["marker"])),
        "accountingSha256": digest(
            canonical(
                sorted(
                    [
                        r
                        for r in bundle["rows"]
                        if r["item"].get("kind", {}).get("S")
                        in {"project", "executionQuota"}
                    ],
                    key=lambda r: (
                        r["table"] + r["item"]["PK"]["S"] + r["item"]["SK"]["S"]
                    ),
                )
            )
        ),
        "closureSha256": digest(canonical(closure)) if closure else None,
    }
    return {
        "action": phase,
        "bundle": bundle,
        "baseline": first["bundle"]["platform"],
        "reviewCommit": c.get("reviewCommit"),
        "manifestSha256": c.get("manifestSha256"),
        "approval": engine,
        "closure": closure,
        "clientRequestToken": str(
            uuid.uuid5(uuid.NAMESPACE_URL, str(c.directory.resolve()) + phase)
        ),
    }


def prepare(c, phase):
    c.check()
    index = c.get("nextPhase")
    if (
        index >= 6
        or ["collect", "close", "deploy", "verify", "activate", "final"][index] != phase
        or c.get("activePhase") is not None
    ):
        raise Held("phase-approval-order-held")
    previous = ["collect", "close", "deploy", "verify", "activate"][index - 1]
    approval = {
        "authorized": False,
        "action": phase,
        "reviewCommit": c.get("reviewCommit"),
        "manifestSha256": c.get("manifestSha256"),
        "inputProofSha256": c.get("proof:" + previous)["sha256"],
        "authorizationSha256": c.get("authorizationSha256"),
    }
    if phase in {"close", "activate"}:
        approval["engineApproval"] = stage_options(c, phase)["approval"]
    return approval


def execute_phase(c, phase, approval, scope):
    expected = prepare(c, phase)
    if approval != {**expected, "authorized": True}:
        raise Held("explicit-fresh-phase-approval-required")
    c.begin(phase)
    creds = credentials(c, scope)
    context = {"sst": str(SST)}
    closed = None
    if phase != "close":
        closed = read_proof(c, "close")["next"]
    if phase in {"close", "activate"}:
        context = stage_options(c, phase)
        planfile = c.directory / (phase + "-plan-input.json")
        private_write(planfile, canonical(context))
        r = subprocess.run(
            phase_command("plan", required_path(planfile, file=True)),
            capture_output=True,
            check=False,
        )
        if r.returncode:
            raise Held("offline-stage-plan-held")
        plan = json.loads(r.stdout)
        context["transaction"] = plan["transaction"]
        context["closedMarker"] = closed
    if phase in {"deploy", "verify", "activate", "final"}:
        context.update(
            closedMarker=closed,
            initialRouting=read_proof(c, "collect")["bundle"]["routing"],
        )
    if phase == "final":
        context.update(
            expectedActiveMarker=read_proof(c, "activate")["next"],
            verifiedPlatform=read_proof(c, "verify")["bundle"]["platform"],
            accountingSha256=read_proof(c, "close")["result"]["accountingSha256"],
        )
    directory, summary = child(c, phase, phase, context, creds)
    if phase == "deploy":
        if not summary["closedVerified"] or not summary["artifactsMatched"]:
            raise Held("deployment-closure-or-archive-proof-held")
        proof = {"completed": True, "summary": summary}
    else:
        output = json.loads((directory / "private-output.txt").read_bytes())
        if phase in {"close", "activate"}:
            if summary["lastMarkerSha256"] != digest(canonical(plan["next"])):
                raise Held("actual-stage-readback-held")
            proof = {"result": output, "next": plan["next"]}
        elif phase == "verify":
            if not summary["closedVerified"] or not output["frontendReady"]:
                raise Held("deployed-frontend-or-stage-held")
            proof = {"bundle": output}
        else:
            proof = {"result": output}
    file = c.directory / (phase + "-proof.json")
    private_write(file, canonical(proof))
    c.finish(phase, {"sha256": sha(file)})


def start(c, scope, checkpoint):
    c.put("cacheStat", checkpoint["cacheStat"])
    c.put("cacheMetadata", checkpoint["cacheMetadata"])
    c.begin("collect")
    creds = credentials(c, scope)
    directory, _summary = child(c, "collect", "collect-readers", {}, creds)
    bundle = json.loads((directory / "private-output.txt").read_bytes())
    directory, _summary = child(
        c,
        "collect",
        "collect-diff",
        {"sst": str(SST)},
        creds,
    )
    raw = json.loads((directory / "private-output.txt").read_bytes())
    old = json.loads(
        (
            ROOT
            / ".build/project-environments-preview/review-8/provider/private-output.txt"
        ).read_bytes()
    )
    if len(raw) != len(old) or plan_projection(raw) != plan_projection(old):
        raise Held("fresh-plan-drift-held")
    release = json.loads(MANIFEST.read_bytes())
    for p in release["packages"]:
        if (
            sha(directory / "work/sst/artifacts" / p["name"] / "code.zip")
            != p["zipSha256"]
        ):
            raise Held("fresh-package-drift-held")
    if (
        json.loads((directory / "work/sst/frontend-hosting-artifact.json").read_bytes())
        != release["frontend"]
    ):
        raise Held("fresh-frontend-drift-held")
    proof = {
        "bundle": bundle,
        "freshPlanSha256": digest(canonical(plan_projection(raw))),
        "allFourFreshArchivesMatched": True,
    }
    path = c.directory / "collect-proof.json"
    private_write(path, canonical(proof))
    c.finish("collect", {"sha256": sha(path)})


def report(c):
    rows = c.db.execute(
        "SELECT n,phase,op,write,status,outcome FROM attempts ORDER BY n"
    ).fetchall()
    return {
        "prior": c.get("prior"),
        "newAttempts": len(rows),
        "cumulative": c.get("count"),
        "maximum": c.get("maximum"),
        "deadline": c.get("deadline"),
        "phaseDeadline": c.get("phaseDeadline"),
        "closedDeadline": c.get("closedDeadline"),
        "nextPhase": c.get("nextPhase"),
        "stopped": c.get("stopped"),
        "unmatchedAttempts": sum(not r[5] for r in rows),
        "proceduralAllowanceUsd": 1,
        "billingMeasured": False,
        "automaticContinuation": False,
        "authenticationFailure": authentication_projection(
            c.get("authenticationFailure")
        ),
        "attempts": [
            {
                "count": r[0],
                "phase": r[1],
                "operation": r[2],
                "write": bool(r[3]),
                "status": r[4],
            }
            for r in rows
        ],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "action",
        choices=[
            "check",
            "prepare-checkpoint",
            "template",
            "start",
            "prepare",
            "phase",
            "report",
        ],
        nargs="?",
        default="check",
    )
    parser.add_argument("--campaign", type=Path)
    parser.add_argument("--authorization", type=Path)
    parser.add_argument("--approval", type=Path)
    parser.add_argument("--checkpoint", type=Path)
    parser.add_argument(
        "--phase", choices=["close", "deploy", "verify", "activate", "final"]
    )
    a = parser.parse_args()
    if a.campaign is not None:
        a.campaign = campaign_path(a.campaign)
    for name in ("authorization", "approval"):
        path = getattr(a, name)
        if path is not None:
            path = path if path.is_absolute() else ROOT / path
            setattr(a, name, required_path(path, file=True))
    if a.checkpoint is not None:
        requested = a.checkpoint if a.checkpoint.is_absolute() else ROOT / a.checkpoint
        directory = BASE / "checkpoints"
        if (
            requested.absolute() != requested.resolve()
            or requested.parent.resolve() != directory.resolve()
        ):
            raise Held("checkpoint-location-or-alias-held")
        a.checkpoint = requested
    if a.action == "report":
        if a.campaign is None or a.campaign.resolve().parent != BASE.resolve():
            raise Held("campaign-location-held")
        c = Campaign(a.campaign)
        try:
            print(json.dumps(report(c), indent=2))
        finally:
            c.close()
        return
    _release, scope, pins = check()
    pending = pending_reads(scope, _release)
    if a.action == "start" and pending:
        raise Held("scoped-read-approval-required-before-new-campaign")
    if a.action == "check":
        print(
            json.dumps(
                {
                    "localChecksPassed": True,
                    "artifactsAdopted": True,
                    "awsCalls": 0,
                    "prior": PRIOR,
                    "authorizedCumulativeCeiling": 1799,
                    "credentialPolicy": CREDENTIAL_POLICY,
                    "pendingScopedReads": pending,
                    "executionRequiresSeparateAuthorization": True,
                }
            )
        )
        return
    if a.action == "prepare-checkpoint":
        if git("status", "--porcelain", "--untracked-files=all"):
            raise Held("clean-tooling-required")
        if a.checkpoint is None or a.approval is None:
            raise Held("checkpoint-and-explicit-approval-required")
        capture_checkpoint(
            scope,
            pins,
            git("rev-parse", "HEAD"),
            a.approval,
            a.checkpoint,
            stopped_parent()["cacheMetadata"],
        )
        print(
            json.dumps(
                {
                    "checkpointCaptured": True,
                    "historicalGrantContinuityEstablished": False,
                    "awsCalls": 0,
                }
            )
        )
        return
    checkpoint = None
    checkpoint_sha = None
    if a.action in {"template", "start"}:
        if a.checkpoint is None:
            raise Held("adopted-checkpoint-required")
        checkpoint, checkpoint_sha = load_checkpoint(
            a.checkpoint, scope, pins, git("rev-parse", "HEAD")
        )
    if a.action == "template":
        print(
            json.dumps(
                {
                    "authorized": False,
                    "campaignId": CAMPAIGN_ID,
                    "checkpointSha256": checkpoint_sha,
                    "prior": PRIOR,
                    "maximum": 1799,
                    "seconds": 3300,
                    "reviewCommit": git("rev-parse", "HEAD"),
                    "manifestSha256": pins["manifest"],
                    "scopeSha256": pins["scope"],
                    "proceduralAllowanceUsd": 1,
                    "credentialPolicy": CREDENTIAL_POLICY,
                    "pendingScopedReads": pending,
                },
                indent=2,
            )
        )
        return
    if a.campaign is None or a.campaign.resolve().parent != BASE.resolve():
        raise Held("campaign-location-held")
    if a.action in {"start", "phase"} and git(
        "status", "--porcelain", "--untracked-files=all"
    ):
        raise Held("clean-tooling-required")
    c = None
    try:
        if a.action == "start":
            if a.campaign.name != CAMPAIGN_ID:
                raise Held("fresh-bound-campaign-identity-required")
            auth = json.loads(a.authorization.read_bytes())
            expected = {
                "authorized": True,
                "campaignId": CAMPAIGN_ID,
                "checkpointSha256": checkpoint_sha,
                "prior": PRIOR,
                "maximum": 1799,
                "seconds": 3300,
                "reviewCommit": git("rev-parse", "HEAD"),
                "manifestSha256": pins["manifest"],
                "scopeSha256": pins["scope"],
                "proceduralAllowanceUsd": 1,
                "credentialPolicy": CREDENTIAL_POLICY,
                "pendingScopedReads": pending,
            }
            if auth != expected:
                raise Held("explicit-expanded-execution-authorization-required")
            if (BASE / CLAIM_NAME).exists() or a.campaign.exists():
                raise Held("single-use-campaign-claim-already-consumed")
            BASE.mkdir(mode=0o700, exist_ok=True)
            private_write(
                BASE / CLAIM_NAME,
                canonical(
                    {
                        "authorizationSha256": sha(a.authorization),
                        "directory": str(a.campaign.resolve()),
                        "checkpointSha256": checkpoint_sha,
                        "campaignId": CAMPAIGN_ID,
                    }
                ),
            )
            c = Campaign(
                a.campaign,
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
                | {"authorizationSha256": sha(a.authorization)},
            )
            c.put("campaignId", CAMPAIGN_ID)
            c.put("checkpointSha256", checkpoint_sha)
            start(c, scope, checkpoint)
        else:
            c = Campaign(a.campaign)
            if (
                c.get("reviewCommit") != git("rev-parse", "HEAD")
                or c.get("manifestSha256") != pins["manifest"]
                or c.get("scopeSha256") != pins["scope"]
            ):
                raise Held("exact-campaign-tooling-required")
            if a.action == "prepare":
                print(json.dumps(prepare(c, a.phase), indent=2))
                return
            if a.action == "phase":
                execute_phase(c, a.phase, json.loads(a.approval.read_bytes()), scope)
        print(json.dumps(report(c), indent=2))
    except BaseException:
        if c is not None:
            try:
                c.stop("phase-failed-or-uncertain-no-restart")
            except Held:
                pass
        raise
    finally:
        if c is not None:
            c.close()


if __name__ == "__main__":
    try:
        main()
    except Exception as error:  # noqa: BLE001 - redact private request/response material
        code = (
            " Authentication code: authentication-sdk-pins-held."
            if type(error) is Held and error.args == ("authentication-sdk-pins-held",)
            else ""
        )
        raise SystemExit(
            "Execution held; no retry, repair, reopen or automatic continuation. Inspect sanitized campaign report."
            + code
        ) from None
