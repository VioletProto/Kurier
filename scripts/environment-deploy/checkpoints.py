"""Private, local-only cache checkpoint capture; no credential provider or network."""

import configparser
import datetime
import fcntl
import hashlib
import os
import stat
from pathlib import Path

from common import PRIOR, Held, canonical, digest, private_write, strict_json
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

CAMPAIGN_ID = "release-390-cp1"
CLAIM_NAME = "window-from-390-checkpoint-1.json"
AUTHORIZATION_NAME = "authorization-from-390-checkpoint-1.json"
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
        "action": "capture-fresh-current-cache-checkpoint",
        "toolingCommit": commit,
        "releaseSourceCommit": SOURCE,
        "identityArn": scope["identityArn"],
        "account": scope["account"],
        "manifestSha256": pins["manifest"],
        "scopeSha256": pins["scope"],
        "zeroCallParentSha256": pins["stoppedZeroCall390"],
        "campaignId": CAMPAIGN_ID,
        "checkpointPath": str(checkpoint_path),
        "prior": PRIOR,
        "maximum": 1799,
        "proceduralAllowanceUsd": 1,
        "historicalInspectionTupleAvailable": False,
        "historicalGrantContinuityEstablished": False,
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


def capture(scope, pins, commit, approval_path, destination):
    authorized, approval_sha = private_json(approval_path)
    if authorized.get("authorized") is not True or authorized != approval(
        scope, pins, commit, destination
    ):
        raise Held("explicit-checkpoint-capture-approval-required")
    parent = destination.parent.lstat()
    if (
        not stat.S_ISDIR(parent.st_mode)
        or parent.st_uid != os.getuid()
        or stat.S_IMODE(parent.st_mode) != 0o700
    ):
        raise Held("private-checkpoint-directory-held")
    # Metadata and approval live outside the cache and outside all child mounts.
    if (
        destination.exists()
        or Path(str(destination) + ".accepted").exists()
        or Path(str(destination) + ".rejected").exists()
    ):
        raise Held("checkpoint-output-already-exists")
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
    written = False
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        locked_config = configparser.RawConfigParser()
        locked_config.read(config_path)
        if (
            locked_config.get("default", "login_session") != identity
            or metadata(config_path.stat()) != config_before
        ):
            raise Held("checkpoint-changed-during-capture")
        before = stable_metadata(fd, path)
        raw = os.pread(fd, before["size"] + 1, 0)
        validate_cache(raw, scope)
        stable_metadata(fd, path, before)
        if (
            raw != os.pread(fd, before["size"] + 1, 0)
            or metadata(config_path.stat()) != config_before
        ):
            raise Held("checkpoint-changed-during-capture")
        record = {
            "schema": 1,
            "approval": authorized,
            "approvalPath": str(approval_path),
            "approvalSha256": approval_sha,
            "cacheMetadata": before,
            "cacheStat": [before["inode"], before["size"], before["mtimeNs"]],
        }
        private_write(destination, canonical(record))
        written = True
        stable_metadata(fd, path, before)
        if (
            raw != os.pread(fd, before["size"] + 1, 0)
            or metadata(config_path.stat()) != config_before
        ):
            raise Held("checkpoint-changed-during-capture")
        private_write(
            Path(str(destination) + ".accepted"),
            canonical({"checkpointSha256": digest(canonical(record))}),
        )
        stable_metadata(fd, path, before)
        if metadata(config_path.stat()) != config_before:
            raise Held("checkpoint-changed-during-capture")
    except BaseException:
        if written:
            private_write(Path(str(destination) + ".rejected"), b'{"rejected":true}')
        raise
    finally:
        os.close(fd)


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
