"""Private, local-only cache checkpoint capture; no credential provider or network."""

import configparser
import datetime
import fcntl
import hashlib
import os
import stat
from contextlib import contextmanager
from pathlib import Path

from common import PRIOR, Held, canonical, digest, private_write, strict_json
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

CAMPAIGN_ID = "release-608-t1"
CLAIM_NAME = "window-from-608-timing-1.json"
AUTHORIZATION_NAME = "authorization-from-608-timing-1.json"
SOURCE = "44f1fb79d943501c95e06fcc1a6114111a7c1ea4"


def metadata(value):
    return {
        "device": value.st_dev,
        "inode": value.st_ino,
        "size": value.st_size,
        "mtimeNs": value.st_mtime_ns,
        "ctimeNs": value.st_ctime_ns,
        "uid": value.st_uid,
        "mode": stat.S_IMODE(value.st_mode),
    }


def stable_metadata(fd, path, expected=None):
    opened, named = os.fstat(fd), path.lstat()
    result = metadata(opened)
    if (
        not stat.S_ISREG(opened.st_mode)
        or not stat.S_ISREG(named.st_mode)
        or opened.st_uid != os.getuid()
        or stat.S_IMODE(opened.st_mode) != 0o600
        or result != metadata(named)
        or (expected is not None and result != expected)
    ):
        raise Held("campaign-cache-metadata-changed")
    return result


def private_json(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        value = os.fstat(fd)
        if (
            not stat.S_ISREG(value.st_mode)
            or value.st_uid != os.getuid()
            or stat.S_IMODE(value.st_mode) != 0o600
        ):
            raise Held("private-checkpoint-record-held")
        raw = os.pread(fd, value.st_size + 1, 0)
        if metadata(os.fstat(fd)) != metadata(value) or metadata(
            path.lstat()
        ) != metadata(value):
            raise Held("private-checkpoint-record-changed")
        return strict_json(raw), digest(raw)
    finally:
        os.close(fd)


def approval(scope, pins, commit, checkpoint_path):
    return {
        "authorized": True,
        "action": "carry-forward-metered-cache-checkpoint",
        "toolingCommit": commit,
        "releaseSourceCommit": SOURCE,
        "identityArn": scope["identityArn"],
        "account": scope["account"],
        "manifestSha256": pins["manifest"],
        "scopeSha256": pins["scope"],
        "completedParentSha256": pins["completedParent608"],
        "campaignId": CAMPAIGN_ID,
        "checkpointPath": str(checkpoint_path),
        "prior": PRIOR,
        "maximum": 1799,
        "proceduralAllowanceUsd": 1,
        "historicalInspectionTupleAvailable": False,
        "historicalGrantContinuityEstablished": False,
        "parentCheckpointMatchRequired": True,
        "cacheDriftAdoptionAuthorized": False,
    }


def validate_cache(raw, scope):
    try:
        value = strict_json(raw)
        token = value["accessToken"]
        if (
            token["accountId"] != scope["account"]
            or not all(
                type(value.get(k)) is str and value[k]
                for k in ("clientId", "refreshToken", "dpopKey")
            )
            or not all(
                type(token.get(k)) is str and token[k]
                for k in ("accessKeyId", "secretAccessKey", "sessionToken", "expiresAt")
            )
            or datetime.datetime.fromisoformat(token["expiresAt"]).tzinfo is None
        ):
            raise Held("checkpoint-cache-structure-held")
    except (KeyError, TypeError, ValueError):
        raise Held("checkpoint-cache-structure-held") from None
    try:
        key = serialization.load_pem_private_key(
            value["dpopKey"].encode(), password=None
        )
    except (TypeError, ValueError):
        raise Held("checkpoint-cache-key-held") from None
    if not isinstance(key, ec.EllipticCurvePrivateKey) or not isinstance(
        key.curve, ec.SECP256R1
    ):
        raise Held("checkpoint-cache-key-held")


@contextmanager
def locked_snapshot(scope, expected_metadata=None, *, fresh=False):
    """Read one locked cache; return only metadata and an in-memory race check."""
    cfg = configparser.RawConfigParser()
    config_path = Path.home() / ".aws/config"
    config_before = metadata(config_path.stat())
    cfg.read(config_path)
    identity = cfg.get("default", "login_session")
    if identity != scope["identityArn"] or not identity.startswith(
        "arn:aws:iam::" + scope["account"] + ":user/"
    ):
        raise Held("inventoried-session-required")
    path = (
        Path.home()
        / ".aws/login/cache"
        / (hashlib.sha256(identity.encode()).hexdigest() + ".json")
    )
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        locked_config = configparser.RawConfigParser()
        locked_config.read(config_path)
        if (
            locked_config.get("default", "login_session") != identity
            or metadata(config_path.stat()) != config_before
        ):
            raise Held("checkpoint-changed-during-capture")
        if expected_metadata is None and not fresh:
            raise Held("verified-parent-checkpoint-required")
        before = stable_metadata(fd, path, expected_metadata)
        raw = os.pread(fd, before["size"] + 1, 0)
        validate_cache(raw, scope)

        def unchanged():
            stable_metadata(fd, path, before)
            if (
                raw != os.pread(fd, before["size"] + 1, 0)
                or metadata(config_path.stat()) != config_before
            ):
                raise Held("checkpoint-changed-during-capture")

        unchanged()
        yield before, unchanged
        unchanged()
    finally:
        os.close(fd)


def write_checkpoint(destination, record, unchanged):
    parent = destination.parent.lstat()
    if (
        not stat.S_ISDIR(parent.st_mode)
        or parent.st_uid != os.getuid()
        or stat.S_IMODE(parent.st_mode) != 0o700
    ):
        raise Held("private-checkpoint-directory-held")
    if any(
        os.path.lexists(str(destination) + suffix)
        for suffix in ["", ".accepted", ".rejected"]
    ):
        raise Held("checkpoint-output-already-exists")
    written = False
    try:
        private_write(destination, canonical(record))
        written = True
        unchanged()
        private_write(
            Path(str(destination) + ".accepted"),
            canonical({"checkpointSha256": digest(canonical(record))}),
        )
        unchanged()
    except BaseException:
        if written:
            private_write(Path(str(destination) + ".rejected"), b'{"rejected":true}')
        raise


def capture(scope, pins, commit, approval_path, destination, expected_metadata):
    authorized, approval_sha = private_json(approval_path)
    if authorized.get("authorized") is not True or authorized != approval(
        scope, pins, commit, destination
    ):
        raise Held("explicit-checkpoint-capture-approval-required")
    # Refuse existing evidence before opening the cache, including dangling links.
    if any(
        os.path.lexists(str(destination) + suffix)
        for suffix in ["", ".accepted", ".rejected"]
    ):
        raise Held("checkpoint-output-already-exists")
    try:
        with locked_snapshot(scope, expected_metadata) as (before, unchanged):
            record = {
                "schema": 1,
                "approval": authorized,
                "approvalPath": str(approval_path),
                "approvalSha256": approval_sha,
                "cacheMetadata": before,
                "cacheStat": [before[k] for k in ("inode", "size", "mtimeNs")],
            }
            write_checkpoint(destination, record, unchanged)
    except BaseException:
        if destination.exists() and not Path(str(destination) + ".rejected").exists():
            private_write(Path(str(destination) + ".rejected"), b'{"rejected":true}')
        raise


def fresh_policy(scope):
    return {
        "mode": "fresh-at-start",
        "allowFreshCapture": True,
        "identityArn": scope["identityArn"],
        "account": scope["account"],
        "historicalGrantContinuityEstablished": False,
        "record": "initial-checkpoint.json",
    }


def capture_at_start(c, scope):
    # A single explicit authorization, claimed campaign and original deadlines
    # precede capture. Nothing can reenter this path within an active campaign.
    c.check()
    inspection = c.get("inspectionOnly") is True
    campaign_id = "recovery-inspect-868-r1" if inspection else CAMPAIGN_ID
    prior = 868 if inspection else PRIOR
    if (
        c.get("checkpointPolicy") != fresh_policy(scope)
        or c.get("campaignId") != campaign_id
        or c.get("activePhase") != "collect"
        or c.get("nextPhase") != 0
        or c.get("credentialPhase") is not None
        or c.get("freshCaptureStarted")
        or c.get("count") != prior
        or c.db.execute("SELECT COUNT(*) FROM attempts").fetchone()[0]
    ):
        raise Held("fresh-checkpoint-approval-or-replay-held")
    c.put("freshCaptureStarted", True)
    destination = c.directory / "initial-checkpoint.json"
    try:
        with locked_snapshot(scope, fresh=True) as (before, unchanged):
            record = {
                "schema": 2,
                "campaignId": campaign_id,
                "authorizationSha256": c.get("authorizationSha256"),
                "toolingCommit": c.get("reviewCommit"),
                "manifestSha256": c.get("manifestSha256"),
                "scopeSha256": c.get("scopeSha256"),
                "checkpointPolicy": fresh_policy(scope),
                "cacheMetadata": before,
                "cacheStat": [before[k] for k in ("inode", "size", "mtimeNs")],
            }
            destination = c.directory / "initial-checkpoint.json"
            write_checkpoint(destination, record, unchanged)
            c.check()
        # The final race check and descriptor close finish before capture is
        # marked complete. Authentication immediately rechecks this snapshot.
        c.put("checkpointSha256", digest(canonical(record)))
        c.put("freshCaptureCompleted", True)
        c.event("fresh-checkpoint-captured", historicalGrantContinuityEstablished=False)
        return record
    except BaseException:
        if destination.exists() and not Path(str(destination) + ".rejected").exists():
            private_write(Path(str(destination) + ".rejected"), b'{"rejected":true}')
        raise


def load(path, scope, pins, commit):
    if Path(str(path) + ".rejected").exists():
        raise Held("rejected-checkpoint-held")
    value, sha = private_json(path)
    accepted, _ = private_json(Path(str(path) + ".accepted"))
    approved, approved_sha = private_json(Path(value["approvalPath"]))
    if (
        set(value)
        != {
            "schema",
            "approval",
            "approvalPath",
            "approvalSha256",
            "cacheMetadata",
            "cacheStat",
        }
        or type(value["schema"]) is not int
        or value["schema"] != 1
        or set(value["cacheMetadata"])
        != {"device", "inode", "size", "mtimeNs", "ctimeNs", "uid", "mode"}
        or any(type(x) is not int or x < 0 for x in value["cacheMetadata"].values())
        or value["cacheMetadata"]["uid"] != os.getuid()
        or value["cacheMetadata"]["mode"] != 0o600
        or approved.get("authorized") is not True
        or approved != approval(scope, pins, commit, path)
        or value["approval"] != approved
        or approved_sha != value["approvalSha256"]
        or accepted != {"checkpointSha256": sha}
        or value["cacheStat"]
        != [value["cacheMetadata"][k] for k in ("inode", "size", "mtimeNs")]
    ):
        raise Held("bound-checkpoint-held")
    return value, sha


def require_active_continuity(c):
    """Host-only metadata check during child work; never load or renew tokens."""
    expected = c.get("cacheMetadata")
    if expected is None:
        if c.get("credentialPhase") is not None:
            c.stop("active-cache-checkpoint-changed")
        return  # Transport-only offline fixtures have no authentication boundary.
    try:
        identity = c.get("cacheIdentityArn")
        if not identity or c.get("credentialPhase") != c.get("activePhase"):
            raise Held("active-cache-checkpoint-changed")
        cfg = configparser.RawConfigParser()
        cfg.read(Path.home() / ".aws/config")
        if cfg.get("default", "login_session") != identity:
            raise Held("active-cache-checkpoint-changed")
        path = (
            Path.home()
            / ".aws/login/cache"
            / (hashlib.sha256(identity.encode()).hexdigest() + ".json")
        )
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        try:
            stable_metadata(fd, path, expected)
        finally:
            os.close(fd)
    except Exception:  # noqa: BLE001 - never publish config/cache exception text
        c.stop("active-cache-checkpoint-changed")
