"""One metered authentication boundary per approved, bounded phase.

No background refresh, default provider chain, credential broker, or new session.
SDK renewal uses the locked original cache and the pinned five-minute threshold.
"""

import datetime
import logging
import os
from unittest.mock import patch

from auth_diagnostics import DeadlineExpired
from authentication import ACCOUNT, STS, ExistingCache, Transport, deadline_alarm
from botocore import UNSIGNED
from botocore.config import Config
from botocore.credentials import LoginCredentialFetcher
from botocore.httpsession import URLLib3Session
from botocore.session import Session
from checkpoints import stable_metadata
from common import CREDENTIAL_GUARD_SECONDS, MAX_REFRESHES, Held
from credential_diagnostics import diagnostic
from policy import Stop


def cache_stat(cache):
    stat = os.fstat(cache.fd)
    return [stat.st_ino, stat.st_size, stat.st_mtime_ns]


class AuthBudget:
    def __init__(self, campaign):
        self.c = campaign

    @property
    def count(self):
        return self.c.get("count")

    @property
    def stopped(self):
        return self.c.get("stopped")

    def campaign_call(self, function, *args):
        # The pinned authentication transport expects policy.Stop; campaign
        # guards use common.Held. Preserve the first stop instead of having the
        # transport recategorize it as an unexpected exception and stop again.
        try:
            return function(*args)
        except Held as error:
            raise Stop(diagnostic(error, "boundary")["code"]) from None

    def check(self):
        return self.campaign_call(self.c.check)

    def stop(self, reason):
        code = diagnostic(Held(reason), "boundary")["code"]
        return self.campaign_call(self.c.stop, code)

    def admit(self, service, operation, _protocol):
        if service == "signin":
            total, this_phase = self.c.db.execute(
                "SELECT COUNT(*),COALESCE(SUM(phase=?),0) FROM attempts WHERE op=?",
                (self.c.get("activePhase"), "signin/CreateOAuth2Token"),
            ).fetchone()
            if total >= MAX_REFRESHES or this_phase:
                self.stop("phase-or-campaign-renewal-limit")
        return self.campaign_call(
            self.c.admit, service + "/" + operation, service == "signin"
        )

    def record(self, **values):
        return self.c.event(values.pop("event"), **values)

    def outcome(self, n, status=None, error=False):
        return self.campaign_call(self.c.outcome, n, None if error else status)


def credentials(c, scope):
    """Called only after approval validation and single-use phase begin."""
    cache = None
    clients = []
    stage = "boundary"
    try:
        c.check()
        phase = c.get("activePhase")
        if (
            phase is None
            or c.get("credentialPhase") == phase
            or c.get("phaseDeadline") is None
        ):
            raise Held("authentication-boundary-required-no-replay")
        stage = "cache-open"
        cache = ExistingCache()
        stage = "session-check"
        if cache.identity != scope["identityArn"]:
            raise Held("inventoried-session-required")
        # The first boundary is seeded from the stopped parent; subsequent ones
        # must match our last successful renewal. External replacement stops.
        stage = "checkpoint-check"
        if c.get("cacheStat") != cache_stat(cache):
            raise Held("session-changed-no-unmetered-refresh")
        if c.get("cacheMetadata") is not None:
            stable_metadata(cache.fd, cache.path, c.get("cacheMetadata"))
        stage = "sdk-session"
        transport = Transport(AuthBudget(c), cache.original, URLLib3Session.send)
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
            values = credentials or {}
            instance = session.create_client(
                service,
                region_name="us-east-2",
                endpoint_url=None if service == "signin" else STS,
                verify=True,
                aws_access_key_id=values.get("access_key", "unused"),
                aws_secret_access_key=values.get("secret_key", "unused"),
                aws_session_token=values.get("token"),
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

        old = logging.root.manager.disable
        logging.disable(logging.CRITICAL)
        try:
            with (
                deadline_alarm(c.get("phaseDeadline")),
                patch.object(
                    URLLib3Session,
                    "send",
                    lambda physical, request: transport.forward(physical, request),
                ),
            ):
                # The pinned SDK reuses credentials above 300 seconds; otherwise
                # it makes at most one real renewal and rotates the same cache.
                stage = "sdk-credentials"
                values = LoginCredentialFetcher(
                    cache.identity, cache, client
                ).refresh_credentials()
                stage = "sts"
                identity = client("sts", credentials=values).get_caller_identity()
                if (
                    identity.get("Account") != ACCOUNT
                    or identity.get("Arn") != scope["identityArn"]
                ):
                    raise Held("authentication-identity-held")
        finally:
            logging.disable(old)
        stage = "validity-check"
        c.check()
        token = cache.load_token(cache.identity)["accessToken"]
        if c.get("cacheMetadata") is not None and any(
            token[key] != values[name]
            for key, name in [
                ("accessKeyId", "access_key"),
                ("secretAccessKey", "secret_key"),
                ("sessionToken", "token"),
            ]
        ):
            raise Held("credential-cache-response-mismatch")
        validity = (
            datetime.datetime.fromisoformat(token["expiresAt"])
            - datetime.datetime.now(datetime.UTC)
        ).total_seconds()
        if (
            token["accountId"] != ACCOUNT
            or validity < c.get("phaseDeadline") - c.clock() + CREDENTIAL_GUARD_SECONDS
        ):
            raise Held("credential-phase-window-too-short")
        stage = "checkpoint-save"
        if c.get("cacheMetadata") is not None:
            c.put("cacheMetadata", stable_metadata(cache.fd, cache.path))
        c.put("cacheStat", cache_stat(cache))
        c.put("credentialPhase", phase)
        c.event("phase-authenticated", phase=phase, identityVerified=True)
        return {
            "AWS_ACCESS_KEY_ID": token["accessKeyId"],
            "AWS_SECRET_ACCESS_KEY": token["secretAccessKey"],
            "AWS_SESSION_TOKEN": token["sessionToken"],
        }
    except (Exception, DeadlineExpired) as error:  # noqa: BLE001 - closed codes only
        if c.get("authenticationFailure") is None:
            failure = diagnostic(error, stage)
            c.put("authenticationFailure", failure)
            c.event("authentication-held", **failure)
        if not c.get("stopped"):
            c.stop("phase-authentication-held-no-retry")
        raise Held("phase-authentication-held-no-retry") from None
    finally:
        for client in clients:
            client.close()
        if cache is not None:
            cache.close()
