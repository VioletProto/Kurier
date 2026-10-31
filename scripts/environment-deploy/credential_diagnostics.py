"""Closed authentication diagnostic vocabulary; never persist exception text/data."""

import configparser
import errno
import json

from auth_diagnostics import DeadlineExpired
from botocore import exceptions as sdk
from common import Held
from policy import Stop

STAGES = frozenset(
    {
        "boundary",
        "cache-open",
        "session-check",
        "checkpoint-check",
        "sdk-session",
        "sdk-credentials",
        "sts",
        "validity-check",
        "checkpoint-save",
    }
)
GUARDS = frozenset(
    {
        "authentication-boundary-required-no-replay",
        "inventoried-session-required",
        "session-changed-no-unmetered-refresh",
        "campaign-cache-metadata-changed",
        "credential-cache-response-mismatch",
        "authentication-service-held",
        "authentication-identity-held",
        "credential-phase-window-too-short",
        "latched-stop",
        "original-window-exhausted-or-boot-changed",
        "closed-window-exhausted",
        "phase-window-exhausted",
        "phase-or-campaign-renewal-limit",
        "phase-or-call-limit",
        "authentication-failed",
        "authentication-repeat-held",
        "authentication-endpoint-held",
        "authentication-grant-held",
        "authentication-transport-or-policy-failure",
        "write-uncertain-or-http-transport-failure",
        "duplicate-or-missing-outcome",
    }
)
CODES = GUARDS | frozenset(
    {
        "session-config-missing",
        "session-config-invalid",
        "cache-missing",
        "cache-access-denied",
        "cache-readonly",
        "cache-locked",
        "cache-path-held",
        "cache-io-failed",
        "cache-json-invalid",
        "cache-fields-invalid",
        "cache-account-or-binding-held",
        "configured-identity-held",
        "sdk-login-failed",
        "sdk-call-failed",
        "authentication-deadline",
        "local-authentication-failed",
    }
)


def diagnostic(error, stage):
    """Inspect only types, numeric errno and allowlisted literal guard arguments."""
    code = "local-authentication-failed"
    if isinstance(error, (Held, Stop)) and len(error.args) == 1:
        value = error.args[0]
        if type(value) is str and value in GUARDS:
            code = value
    elif isinstance(error, (configparser.NoSectionError, configparser.NoOptionError)):
        code = "session-config-missing"
    elif isinstance(error, configparser.Error):
        code = "session-config-invalid"
    elif isinstance(error, OSError) and stage == "cache-open":
        code = {
            errno.ENOENT: "cache-missing",
            errno.EACCES: "cache-access-denied",
            errno.EPERM: "cache-access-denied",
            errno.EROFS: "cache-readonly",
            errno.EAGAIN: "cache-locked",
            errno.ELOOP: "cache-path-held",
        }.get(error.errno, "cache-io-failed")
    elif isinstance(error, json.JSONDecodeError):
        code = "cache-json-invalid" if stage == "cache-open" else "cache-fields-invalid"
    elif isinstance(error, ValueError) and stage == "cache-open":
        value = error.args[0] if len(error.args) == 1 else None
        if type(value) is str:
            code = {
                "authentication-existing-identity": "configured-identity-held",
                "authentication-cache-identity": "cache-account-or-binding-held",
            }.get(value, "cache-fields-invalid")
    elif isinstance(error, (KeyError, TypeError, ValueError)) and stage in {
        "cache-open",
        "validity-check",
    }:
        code = "cache-fields-invalid"
    elif isinstance(error, DeadlineExpired):
        code = "authentication-deadline"
    elif isinstance(error, (sdk.LoginError, sdk.LoginTokenLoadError)):
        code = "sdk-login-failed"
    elif isinstance(error, sdk.BotoCoreError):
        code = "sdk-call-failed"
    return {"code": code, "stage": stage if stage in STAGES else "boundary"}


def projection(value):
    """Keep reports closed even if a stored diagnostic has unexpected fields."""
    if (
        type(value) is dict
        and type(value.get("code")) is str
        and type(value.get("stage")) is str
        and value.get("code") in CODES
        and value.get("stage") in STAGES
    ):
        return {"code": value["code"], "stage": value["stage"]}
    return None
