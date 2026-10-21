"""Explicit cleanup continuation; never unlocks or modifies a stopped meter."""

import hashlib
import json
from pathlib import Path
import re
from urllib.parse import urlsplit


def parent_proof(path):
    raw = Path(path).read_bytes()
    rows = [json.loads(line) for line in raw.splitlines()]
    stops = [r for r in rows if r["event"] == "stop"]
    attempts = [r for r in rows if r["event"] == "attempt"]
    outcomes = {r["attempt"]: r for r in rows if r["event"] == "outcome"}
    if len(stops) != 1 or stops[0]["reason"] != "write-held" or not attempts:
        raise ValueError("Only a known blocked, unforwarded write allows cleanup")
    for row in attempts:
        result = outcomes.get(row["sequence"])
        if not result or (
            row["write"]
            and (
                result.get("transportError")
                or result.get("status") is None
                or result["status"] >= 500
            )
        ):
            raise ValueError("Outstanding or uncertain writes prohibit continuation")
    return {
        "sha256": hashlib.sha256(raw).hexdigest(),
        "firstAttemptAt": attempts[0]["at"],
        "counts": attempts[-1]["counts"],
    }


def cleanup_allowed(category, method, host, path, operation, content, scope):
    if category == "api":
        return (
            method in ("GET", "DELETE")
            and urlsplit(path).path
            in ("/api/v1/projects/" + project for project in scope["projects"])
            and not urlsplit(path).query
        )
    if category != "aws":
        return False
    if operation in ("AdminDeleteUser", "AdminUserGlobalSignOut"):
        body = json.loads(content)
        return (
            method == "POST"
            and host == "cognito-idp.us-east-2.amazonaws.com"
            and body.get("UserPoolId") == "us-east-2_qTDZQT1FE"
            and body.get("Username") in scope["users"]
        )
    if operation in (
        "InitiateAuth",
        "RespondToAuthChallenge",
        "RevokeToken",
        "GetTokensFromRefreshToken",
    ):
        return host == "cognito-idp.us-east-2.amazonaws.com"
    return operation == "CreateOAuth2Token" and host in (
        "signin.us-east-2.amazonaws.com",
        "us-east-2.signin.aws.amazon.com",
    )


def validate_scope(scope):
    if (
        set(scope) != {"projects", "users"}
        or len(scope["projects"]) != 2
        or len(scope["users"]) != 2
    ):
        raise ValueError("Exactly the existing two fixture projects/users required")
    if any(not re.fullmatch(r"[a-f0-9-]{36}", p) for p in scope["projects"]):
        raise ValueError("Invalid project scope")
    if any(
        not re.fullmatch(r"kurier-assertions-[a-f0-9-]{36}-[01]@example.invalid", u)
        for u in scope["users"]
    ):
        raise ValueError("Invalid disposable identity scope")
    return scope
