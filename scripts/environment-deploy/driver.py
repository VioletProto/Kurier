"""Reviewed phase-by-phase execution tooling. Default check is local only."""

import argparse
import datetime
import json
import os
import subprocess
import sys
import time
import uuid
from pathlib import Path
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE.parent / "preview-meter"))
from artifacts import load_objects
from authentication import (
    ACCOUNT,
    STS,
    ExistingCache,
    Transport,
    deadline_alarm,
    sdk_pins,
)
from botocore import UNSIGNED
from botocore.config import Config
from botocore.credentials import LoginCredentialFetcher
from botocore.httpsession import URLLib3Session
from botocore.session import Session
from common import PRIOR, ROOT, Campaign, Held, canonical, digest, private_write
from isolation import launch
from run import PROVIDER, PROVIDER_SHA, SOURCE, SST, SST_SHA, git, sha, valid_parent

SOURCE_DIR = Path("/tmp/kurier-environments-release-source")
MANIFEST = ROOT / "docs/development/project-environments-adopted-release.json"
SCOPE = ROOT / "docs/development/project-environments-execution-scope.json"
BASE = ROOT / ".build/project-environments-execution"


def check():
    pins = json.loads((HERE / "pins.json").read_text())
    release = json.loads(MANIFEST.read_bytes())
    scope = json.loads(SCOPE.read_bytes())
    if (
        digest(MANIFEST.read_bytes()) != pins["manifest"]
        or digest(SCOPE.read_bytes()) != pins["scope"]
    ):
        raise Held("execution-pins-held")
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
        att["accounting"]["cumulativeCalls"] != PRIOR
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
    load_objects(scope, release)
    valid_parent()
    sdk_pins()
    import botocore

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


class AuthBudget:
    def __init__(self, c):
        self.c = c

    @property
    def count(self):
        return self.c.get("count")

    @property
    def stopped(self):
        return self.c.get("stopped")

    def check(self):
        return self.c.check()

    def stop(self, r):
        return self.c.stop(r)

    def admit(self, s, o, _protocol):
        return self.c.admit(s + "/" + o, s == "signin")

    def record(self, **v):
        return self.c.event(v.pop("event"), **v)

    def outcome(self, n, status=None, error=False):
        return self.c.outcome(n, None if error else status)


def credentials(c, scope, *, authenticate=False):
    cache = ExistingCache()
    clients = []
    try:
        if cache.identity != scope["identityArn"]:
            raise Held("inventoried-session-required")
        if authenticate:
            t = Transport(AuthBudget(c), cache.original, URLLib3Session.send)
            session = Session(
                session_vars={
                    "config_file": (None, None, "/dev/null", None),
                    "credentials_file": (None, None, "/dev/null", None),
                    "data_path": (None, None, None, None),
                    "csm_enabled": (None, None, False, None),
                }
            )

            def client(service, config=None, credentials=None):
                if service not in {"signin", "sts"}:
                    raise Held("authentication-service-held")
                v = credentials or {}
                instance = session.create_client(
                    service,
                    region_name="us-east-2",
                    endpoint_url=None if service == "signin" else STS,
                    verify=True,
                    aws_access_key_id=v.get("access_key", "unused"),
                    aws_secret_access_key=v.get("secret_key", "unused"),
                    aws_session_token=v.get("token"),
                    config=Config(
                        signature_version=UNSIGNED if service == "signin" else "v4",
                        proxies={},
                        retries={"total_max_attempts": 1},
                        connect_timeout=10,
                        read_timeout=30,
                    ),
                )
                clients.append(instance)
                return instance

            import logging

            old = logging.root.manager.disable
            logging.disable(logging.CRITICAL)
            try:
                with (
                    deadline_alarm(c.get("deadline")),
                    patch.object(
                        URLLib3Session,
                        "send",
                        lambda physical, request: t.forward(physical, request),
                    ),
                ):
                    creds = LoginCredentialFetcher(
                        cache.identity, cache, client
                    ).refresh_credentials()
                    identity = client("sts", credentials=creds).get_caller_identity()
                    if (
                        identity.get("Account") != ACCOUNT
                        or identity.get("Arn") != scope["identityArn"]
                    ):
                        raise Held("authentication-identity-held")
            finally:
                logging.disable(old)
            c.put(
                "cacheStat",
                [
                    os.fstat(cache.fd).st_ino,
                    os.fstat(cache.fd).st_size,
                    os.fstat(cache.fd).st_mtime_ns,
                ],
            )
        stat = os.fstat(cache.fd)
        if c.get("cacheStat") != [stat.st_ino, stat.st_size, stat.st_mtime_ns]:
            raise Held("session-changed-no-unmetered-refresh")
        token = cache.load_token(cache.identity)["accessToken"]
        if (
            token["accountId"] != ACCOUNT
            or (
                datetime.datetime.fromisoformat(token["expiresAt"])
                - datetime.datetime.now(datetime.UTC)
            ).total_seconds()
            < c.get("deadline") - time.monotonic() + 5
        ):
            raise Held("credential-window-too-short-no-refresh-restart")
        return {
            "AWS_ACCESS_KEY_ID": token["accessKeyId"],
            "AWS_SECRET_ACCESS_KEY": token["secretAccessKey"],
            "AWS_SESSION_TOKEN": token["sessionToken"],
        }
    finally:
        for client in clients:
            client.close()
        cache.close()


def mounts(*files):
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
    return [(p, p) for p in [*fixed, *files]]


def child(c, phase, subphase, context, creds, *, command=None):
    directory = c.directory / subphase
    ctx = c.directory / (subphase + "-context.json")
    private_write(ctx, canonical(context))
    result = launch(
        SOURCE_DIR,
        directory,
        command or ["node", str(HERE / "phase.mjs"), phase, str(ctx)],
        creds,
        campaign=c,
        phase=phase,
        policy_context=context,
        extra_ro=mounts(ctx),
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


def execute_phase(c, phase, approval, scope, creds):
    expected = prepare(c, phase)
    if approval != {**expected, "authorized": True}:
        raise Held("explicit-fresh-phase-approval-required")
    c.begin(phase)
    context = {"sst": str(SST)}
    closed = None
    if phase != "close":
        closed = read_proof(c, "close")["next"]
    if phase in {"close", "activate"}:
        context = stage_options(c, phase)
        planfile = c.directory / (phase + "-plan-input.json")
        private_write(planfile, canonical(context))
        r = subprocess.run(
            ["node", str(HERE / "phase.mjs"), "plan", str(planfile)],
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


def start(c, scope):
    c.begin("collect")
    creds = credentials(c, scope, authenticate=True)
    directory, _summary = child(c, "collect", "collect-readers", {}, creds)
    bundle = json.loads((directory / "private-output.txt").read_bytes())
    directory, _summary = child(
        c,
        "collect",
        "collect-diff",
        {"sst": str(SST)},
        creds,
        command=[
            "node",
            str(HERE / "phase.mjs"),
            "diff",
            str(c.directory / "collect-diff-context.json"),
        ],
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
        "prior": PRIOR,
        "newAttempts": len(rows),
        "cumulative": c.get("count"),
        "maximum": c.get("maximum"),
        "deadline": c.get("deadline"),
        "closedDeadline": c.get("closedDeadline"),
        "nextPhase": c.get("nextPhase"),
        "stopped": c.get("stopped"),
        "unmatchedAttempts": sum(not r[5] for r in rows),
        "proceduralAllowanceUsd": 1,
        "billingMeasured": False,
        "automaticContinuation": False,
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
        choices=["check", "template", "start", "prepare", "phase", "report"],
        nargs="?",
        default="check",
    )
    parser.add_argument("--campaign", type=Path)
    parser.add_argument("--authorization", type=Path)
    parser.add_argument("--approval", type=Path)
    parser.add_argument(
        "--phase", choices=["close", "deploy", "verify", "activate", "final"]
    )
    a = parser.parse_args()
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
    if a.action == "check":
        print(
            json.dumps(
                {
                    "localChecksPassed": True,
                    "artifactsAdopted": True,
                    "awsCalls": 0,
                    "prior": 299,
                    "currentPreparationCeiling": 500,
                    "executionRequiresSeparateAuthorization": True,
                }
            )
        )
        return
    if a.action == "template":
        print(
            json.dumps(
                {
                    "authorized": False,
                    "prior": PRIOR,
                    "maximum": 1799,
                    "seconds": 3300,
                    "reviewCommit": git("rev-parse", "HEAD"),
                    "manifestSha256": pins["manifest"],
                    "scopeSha256": pins["scope"],
                    "proceduralAllowanceUsd": 1,
                    "allowExistingSessionRefresh": True,
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
            auth = json.loads(a.authorization.read_bytes())
            expected = {
                "authorized": True,
                "prior": PRIOR,
                "maximum": 1799,
                "seconds": 3300,
                "reviewCommit": git("rev-parse", "HEAD"),
                "manifestSha256": pins["manifest"],
                "scopeSha256": pins["scope"],
                "proceduralAllowanceUsd": 1,
                "allowExistingSessionRefresh": True,
            }
            if auth != expected:
                raise Held("explicit-expanded-execution-authorization-required")
            BASE.mkdir(mode=0o700, exist_ok=True)
            private_write(
                BASE / "window-from-299.json",
                canonical(
                    {
                        "authorizationSha256": sha(a.authorization),
                        "directory": str(a.campaign.resolve()),
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
            start(c, scope)
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
                creds = credentials(c, scope)
                execute_phase(
                    c, a.phase, json.loads(a.approval.read_bytes()), scope, creds
                )
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
    except Exception:  # noqa: BLE001 - redact private request/response material
        raise SystemExit(
            "Execution held; no retry, repair, reopen or automatic continuation. Inspect sanitized campaign report."
        ) from None
