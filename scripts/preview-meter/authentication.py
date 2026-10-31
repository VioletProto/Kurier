"""Existing CLI login refresh only; direct, synchronous metered SDK transport.

No default credential chain, subprocess, interactive login, or provider-policy
changes. Credentials and AWS response material stay in memory or the original
CLI cache. This module performs no network work at import time.
"""

import configparser
import contextlib
import datetime
import fcntl
import hashlib
import json
import logging
import os
import signal
import time
from pathlib import Path
from unittest.mock import patch
from urllib.parse import parse_qs, urlsplit

from accounting import CURRENT_PRIOR as PRIOR
from auth_diagnostics import DeadlineExpired, transport_diagnostic
from handoff import SharedBudget, seal
from policy import Stop

ACCOUNT = "747336059622"
SIGNIN = "https://us-east-2.signin.aws.amazon.com"
STS = "https://sts.us-east-2.amazonaws.com"


def sdk_pins():
    import botocore
    import urllib3

    base = Path(botocore.__file__).parent
    pins = json.loads(Path(__file__).with_name("authentication-sdk.json").read_text())
    if botocore.__version__ != pins["version"] or any(
        hashlib.sha256((base / name).read_bytes()).hexdigest() != sha
        for name, sha in pins["files"].items()
    ):
        raise ValueError("authentication-sdk-pin")
    base = Path(urllib3.__file__).parent
    if urllib3.__version__ != pins["urllib3"]["version"] or any(
        hashlib.sha256((base / name).read_bytes()).hexdigest() != sha
        for name, sha in pins["urllib3"]["files"].items()
    ):
        raise ValueError("authentication-pool-pin")


class ExistingCache:
    def __init__(self):
        cfg = configparser.RawConfigParser()
        cfg.read(Path.home() / ".aws/config")
        self.identity = cfg.get("default", "login_session")
        if not self.identity.startswith(f"arn:aws:iam::{ACCOUNT}:user/"):
            raise ValueError("authentication-existing-identity")
        self.path = (
            Path.home()
            / ".aws/login/cache"
            / (hashlib.sha256(self.identity.encode()).hexdigest() + ".json")
        )
        # Never create a session or cache file. Lock the original across the run.
        self.fd = os.open(self.path, os.O_RDWR | os.O_NOFOLLOW)
        try:
            fcntl.flock(self.fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            self.original = json.loads(self.path.read_text())
            self.verify(self.original)
        except BaseException:
            self.close()
            raise

    def verify(self, token):
        if (
            token["accessToken"]["accountId"] != ACCOUNT
            or not all(token.get(k) for k in ("refreshToken", "clientId", "dpopKey"))
            or token["clientId"] != self.original["clientId"]
            or token["dpopKey"] != self.original["dpopKey"]
        ):
            raise ValueError("authentication-cache-identity")

    def load_token(self, session):
        if session != self.identity:
            raise ValueError("authentication-session-change")
        token = json.loads(self.path.read_text())
        self.verify(token)
        return token

    def save_token(self, session, token):
        if session != self.identity:
            raise ValueError("authentication-session-change")
        self.verify(token)
        # Same file/inode as the locked CLI cache, never a copy in run artifacts.
        # A partial write is a terminal failure, never repaired or retried here.
        raw = json.dumps(token).encode()
        os.fchmod(self.fd, 0o600)
        os.lseek(self.fd, 0, os.SEEK_SET)
        while raw:
            n = os.write(self.fd, raw)
            if n <= 0:
                raise OSError("authentication-cache-write")
            raw = raw[n:]
        os.ftruncate(self.fd, os.lseek(self.fd, 0, os.SEEK_CUR))
        os.fsync(self.fd)

    def close(self):
        os.close(self.fd)


def classify(request, original_token):
    """Separate auth policy; never used by the preview gate."""
    url = urlsplit(request.url)
    if request.method != "POST" or url.query or url.fragment or url.username:
        raise Stop("authentication-endpoint-held")
    body = request.body
    if isinstance(body, str):
        body = body.encode()
    if request.url == SIGNIN + "/v1/token":
        data = json.loads(body)
        if data != {
            "clientId": original_token["clientId"],
            "refreshToken": original_token["refreshToken"],
            "grantType": "refresh_token",
        }:
            raise Stop("authentication-grant-held")
        return "signin", "CreateOAuth2Token"
    if request.url == STS + "/" and parse_qs(body.decode(), strict_parsing=True) == {
        "Action": ["GetCallerIdentity"],
        "Version": ["2011-06-15"],
    }:
        return "sts", "GetCallerIdentity"
    raise Stop("authentication-endpoint-held")


class Transport:
    def __init__(self, budget, original_token, send):
        self.budget, self.original_token, self.send = budget, original_token, send
        self.operations = []

    def forward(self, session, request):
        try:
            service, operation = classify(request, self.original_token)
            # No retries, pagination, extra identity calls or repeat token exchange.
            candidate = self.operations + [service]
            if candidate not in [["signin"], ["sts"], ["signin", "sts"]]:
                self.budget.stop("authentication-repeat-held")
            n = self.budget.admit(service, operation, "HTTP/1.1")
            self.operations = candidate
            try:
                response = self.send(session, request)
            except BaseException as error:
                self.budget.record(
                    event="authentication-transport-error",
                    attempt=n,
                    **transport_diagnostic(error),
                )
                self.budget.outcome(n, error=True)
                raise
            self.budget.outcome(n, response.status_code)
            if not 200 <= response.status_code < 300:
                self.budget.stop("authentication-failed")
            self.budget.check()
            return response
        except Stop as error:
            if not self.budget.stopped:
                self.budget.stop(str(error))
            raise
        except Exception:  # noqa: BLE001 - never expose a raw transport/policy error
            self.budget.stop("authentication-transport-or-policy-failure")


@contextlib.contextmanager
def deadline_alarm(deadline):
    # Interrupt DNS, connect, read or SDK processing at the original deadline.
    # Main thread only; no SDK background workers or child processes are used.
    remaining = deadline - time.monotonic()
    if remaining <= 0 or signal.getitimer(signal.ITIMER_REAL) != (0.0, 0.0):
        raise ValueError("authentication-deadline-or-existing-alarm")

    def expired(*_):
        raise DeadlineExpired()

    previous = signal.signal(signal.SIGALRM, expired)
    signal.setitimer(signal.ITIMER_REAL, remaining)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous)


def authenticate(directory, root, window):
    """Only called by the explicitly reviewed live action, never local checks."""
    from botocore import UNSIGNED
    from botocore.config import Config
    from botocore.credentials import LoginCredentialFetcher
    from botocore.httpsession import URLLib3Session
    from botocore.session import Session

    sdk_pins()
    cache = ExistingCache()
    budget = None
    old_logging = logging.root.manager.disable
    logging.disable(logging.CRITICAL)
    try:
        budget = SharedBudget(directory / "authentication.jsonl", window, prior=PRIOR)
        transport = Transport(budget, cache.original, URLLib3Session.send)
        # Unsigned/static credentials avoid every default credential provider.
        # Resolve Sign-In via the pinned operation rules, then enforce the actual
        # prepared URL in Transport before admitting or forwarding an attempt.
        session = Session(
            session_vars={
                "config_file": (None, None, "/dev/null", None),
                "credentials_file": (None, None, "/dev/null", None),
                "data_path": (None, None, None, None),
            }
        )

        def client(service, config=None, credentials=None):
            budget.check()
            if service not in {"signin", "sts"}:
                budget.stop("authentication-client-held")
            credentials = credentials or {}
            return session.create_client(
                service,
                region_name="us-east-2",
                endpoint_url=None if service == "signin" else STS,
                verify=True,
                aws_access_key_id=credentials.get("access_key", "unused"),
                aws_secret_access_key=credentials.get("secret_key", "unused"),
                aws_session_token=credentials.get("token"),
                config=Config(
                    signature_version=UNSIGNED if service == "signin" else "v4",
                    proxies={},
                    retries={"total_max_attempts": 1},
                    connect_timeout=min(10, window["deadline"] - time.monotonic()),
                    read_timeout=min(30, window["deadline"] - time.monotonic()),
                ),
            )

        def send(session, request):
            return transport.forward(session, request)

        with (
            deadline_alarm(window["deadline"]),
            patch.object(URLLib3Session, "send", send),
        ):
            fetcher = LoginCredentialFetcher(cache.identity, cache, client)
            creds = fetcher.refresh_credentials()
            if (
                creds["account_id"] != ACCOUNT
                or (
                    datetime.datetime.fromisoformat(creds["expiry_time"])
                    - datetime.datetime.now(datetime.UTC)
                ).total_seconds()
                < 120
            ):
                budget.stop("authentication-expired-or-account")
            identity = client("sts", credentials=creds).get_caller_identity()
            if (
                identity.get("Account") != ACCOUNT
                or identity.get("Arn") != cache.identity
            ):
                budget.stop("authentication-identity-mismatch")
            budget.check()
            budget.record(
                event="authenticated", identityVerified=True, count=budget.count
            )
        budget.close()
        budget = None
        handoff = seal(directory, root)
        return {
            "AWS_ACCESS_KEY_ID": creds["access_key"],
            "AWS_SECRET_ACCESS_KEY": creds["secret_key"],
            "AWS_SESSION_TOKEN": creds["token"],
        }, handoff
    except BaseException:  # noqa: BLE001 - include deadline interrupts and never expose tokens
        if budget is not None:
            try:
                budget.stop("authentication-failed-or-uncertain")
            except Stop:
                pass
        # No raw SDK exception, response, token, URL or body escapes this function.
        raise RuntimeError("authentication-failed-or-uncertain-no-restart") from None
    finally:
        if budget is not None:
            budget.close()
        cache.close()
        logging.disable(old_logging)
