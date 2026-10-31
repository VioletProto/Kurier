"""One-shot existing-session/STS/exact Lambda observation. No preview or writes."""

import argparse
import base64
import datetime
import hashlib
import importlib.util
import json
import logging
import os
import resource
import sys
import time
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts/preview-meter"))
from auth_diagnostics import transport_diagnostic
from authentication import (
    ACCOUNT,
    STS,
    ExistingCache,
    classify,
    deadline_alarm,
    sdk_pins,
)
from botocore import UNSIGNED
from botocore.config import Config
from botocore.credentials import LoginCredentialFetcher
from botocore.httpsession import URLLib3Session
from botocore.session import Session
from policy import Stop

PRIOR = 296
MAXIMUM = 299
CEILING = 500
SECONDS = 600
FUNCTION = "kurier-dev-api-UsersProjectsApiFunction-werhekow"
LAMBDA = "https://lambda.us-east-2.amazonaws.com"
TARGET = LAMBDA + "/2015-03-31/functions/" + FUNCTION + "/configuration"
REVIEW8_SHA = "ef67859ce50e8028671c5ad8309c3761f216868615b54c734eb7d5f3ec263aca"
MANIFEST_SHA = "25ef3cd0c8f0c09037607c564bfb8964c1ef8e8e14a3aa0ae8757894918895a9"
IDENTITY_SHA = "6acd0597730959b78dd9d871a748f91c293164a5bea770c0354c633763dd25ac"
MODEL_SHA = "affa29b7b2819e2db3fd727df95409293634dc275ab938e3b9abbf78dc24e5df"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def persist(path, value):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        raw = json.dumps(value, sort_keys=True).encode()
        while raw:
            n = os.write(fd, raw)
            if n <= 0:
                raise OSError()
            raw = raw[n:]
        os.fsync(fd)
    finally:
        os.close(fd)
    directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(directory)
    finally:
        os.close(directory)


class ObservationHeld(Exception):
    def __init__(self, category):
        self.category = category


class Budget:
    def __init__(self, path, *, clock=time.monotonic):
        self.clock = clock
        self.started = clock()
        self.deadline = self.started + SECONDS
        self.count = PRIOR
        self.stopped = False
        self.fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        self.record(
            event="created",
            prior=PRIOR,
            maximum=MAXIMUM,
            cumulativeCeiling=CEILING,
            seconds=SECONDS,
            started=self.started,
            deadline=self.deadline,
        )
        directory = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)

    def record(self, **fields):
        data = (
            json.dumps(
                {"at": time.time(), "monotonic": self.clock(), **fields},
                separators=(",", ":"),
            )
            + "\n"
        ).encode()
        while data:
            n = os.write(self.fd, data)
            if n <= 0:
                raise OSError()
            data = data[n:]
        os.fsync(self.fd)

    def stop(self, reason):
        if not self.stopped:
            self.stopped = True
            self.record(event="stop", count=self.count, reason=reason)
        raise Stop(reason)

    def check(self):
        if self.stopped:
            raise Stop("latched-stop")
        if self.clock() >= self.deadline:
            self.stop("time-limit")

    def admit(self, service, operation, protocol):
        self.check()
        if self.count >= MAXIMUM:
            self.stop("call-limit")
        self.count += 1
        self.record(
            event="attempt",
            count=self.count,
            service=service,
            operation=operation,
            protocol=protocol,
        )
        return self.count

    def outcome(self, attempt, status=None, error=False):
        self.record(
            event="outcome", attempt=attempt, status=status, transportError=error
        )
        if error:
            self.stop("transport-failure")
        if status is None or not 200 <= status < 300:
            self.stop("http-failure-no-retry")
        self.check()
        # Final successful response is allowed to be parsed at ordinal299.
        # A fourth attempt is denied before forwarding; validation is offline.

    def close(self):
        os.close(self.fd)


class Transport:
    def __init__(self, budget, token, send):
        self.budget, self.token, self.send = budget, token, send
        self.operations = []
        self.identity_verified = False

    def forward(self, session, request):
        self.budget.check()
        if (
            request.url == TARGET
            and request.method == "GET"
            and request.body in (None, b"", "")
        ):
            service, operation = "lambda", "GetFunctionConfiguration"
            if not self.identity_verified:
                self.budget.stop("identity-gate-held")
        else:
            try:
                service, operation = classify(request, self.token)
            except Exception:  # noqa: BLE001 - never publish raw policy errors
                self.budget.stop("unexpected-operation-held")
        candidate = self.operations + [service]
        if candidate not in [
            ["signin"],
            ["sts"],
            ["signin", "sts"],
            ["sts", "lambda"],
            ["signin", "sts", "lambda"],
        ]:
            self.budget.stop("repeat-or-order-held")
        n = self.budget.admit(service, operation, "HTTP/1.1")
        self.operations = candidate
        try:
            response = self.send(session, request)
        except BaseException as error:
            self.budget.record(
                event="transport-diagnostic", attempt=n, **transport_diagnostic(error)
            )
            self.budget.outcome(n, error=True)
            raise
        self.budget.outcome(n, response.status_code)
        return response


def archive_checker():
    spec = importlib.util.spec_from_file_location(
        "environment_archive_check", ROOT / "scripts/environment-release-check.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def preflight():
    sdk_pins()
    from run import SOURCE, git, valid_parent

    valid_parent()
    source = Path("/tmp/kurier-environments-release-source")
    if git("rev-parse", "HEAD", cwd=source) != SOURCE or git(
        "status", "--porcelain", "--untracked-files=all", cwd=source
    ):
        raise ValueError("release-source-held")
    import botocore

    model = Path(botocore.__file__).parent / "data/lambda/2015-03-31/service-2.json"
    if sha(model.read_bytes()) != MODEL_SHA:
        raise ValueError("lambda-model-pin")
    manifest_bytes = (
        ROOT / "docs/development/project-environments-sst-release.json"
    ).read_bytes()
    if sha(manifest_bytes) != MANIFEST_SHA:
        raise ValueError("candidate-manifest-pin")
    manifest = json.loads(manifest_bytes)
    review = (
        ROOT / "docs/development/project-environments-refresh-preview-review-8.json"
    )
    if sha(review.read_bytes()) != REVIEW8_SHA:
        raise ValueError("review8-pin")
    review = json.loads(review.read_bytes())
    for name, value in review["preservation"]["originalProofSha256"].items():
        if sha((ROOT / name).read_bytes()) != value:
            raise ValueError("historical-proof-pin")
    for name, value in review["preservation"][
        "privateEvidenceExcludingCaptures"
    ].items():
        if sha((ROOT / name).read_bytes()) != value["sha256"]:
            raise ValueError("review8-evidence-pin")
    result = archive_checker().check(manifest)
    if len(result["packages"]) != 4 or review["accounting"]["cumulativeCalls"] != PRIOR:
        raise ValueError("baseline-held")
    inventory = json.loads(
        (ROOT / "docs/development/project-environments-inventory.json").read_bytes()
    )
    if (
        next(f for f in inventory["functions"] if f["logical"] == "UsersProjectsApi")[
            "configuration"
        ]["FunctionName"]
        != FUNCTION
    ):
        raise ValueError("function-scope-held")
    return manifest


def validate_response(response, manifest, checker):
    # The SDK necessarily parses the configuration response in transient memory.
    # Immediately discard every field except the one existing SST key; no dump,
    # response/body logging, file, subprocess argument or key fingerprint exists.
    key = None
    raw = None
    try:
        if (
            response.get("FunctionName") != FUNCTION
            or response.get("FunctionArn")
            != "arn:aws:lambda:us-east-2:" + ACCOUNT + ":function:" + FUNCTION
        ):
            raise ObservationHeld("configuration-identity-held")
        environment = response.get("Environment")
        if not isinstance(environment, dict) or "Error" in environment:
            raise ObservationHeld("configuration-environment-held")
        variables = environment.get("Variables")
        if not isinstance(variables, dict):
            raise ObservationHeld("key-absent-held")
        raw = variables.pop("SST_KEY", None)
        variables.clear()
        environment.clear()
        response.clear()
        if not isinstance(raw, str) or not raw or len(raw) > 128:
            raise ObservationHeld("key-absent-held")
        key = bytearray(base64.b64decode(raw, validate=True))
        raw = None
        if len(key) != 32:
            raise ObservationHeld("key-shape-held")
        try:
            result = checker.check(manifest, key)
        except Exception:  # noqa: BLE001 - no ciphertext/plaintext/crypto exception escapes
            raise ObservationHeld("binding-authentication-or-content-held") from None
        if len(result["packages"]) != 4 or not result["bindingAuthenticated"]:
            raise ObservationHeld("binding-validation-held")
        # A boolean conclusion only: no retained key, plaintext or environment.
        return {
            "allFourAuthenticatedEmptyBindings": True,
            "keyPresent": True,
            "candidateAdoptionPerformed": False,
        }
    finally:
        response.clear()
        raw = None
        if key is not None:
            key[:] = bytes(len(key))


def observe(
    budget,
    manifest,
    *,
    cache_factory=ExistingCache,
    session_factory=Session,
    sender=URLLib3Session.send,
    checker=None,
):
    cache = cache_factory()
    if sha(cache.identity.encode()) != IDENTITY_SHA:
        cache.close()
        budget.stop("existing-identity-scope-held")
    old_logging = logging.root.manager.disable
    logging.disable(logging.CRITICAL)
    clients = []
    try:
        transport = Transport(budget, cache.original, sender)
        session = session_factory(
            session_vars={
                "config_file": (None, None, "/dev/null", None),
                "credentials_file": (None, None, "/dev/null", None),
                "data_path": (None, None, None, None),
                "csm_enabled": (None, None, False, None),
            }
        )

        def client(service, config=None, credentials=None):
            budget.check()
            if service not in {"signin", "sts", "lambda"}:
                budget.stop("client-held")
            credentials = credentials or {}
            instance = session.create_client(
                service,
                region_name="us-east-2",
                endpoint_url=None
                if service == "signin"
                else STS
                if service == "sts"
                else LAMBDA,
                verify=True,
                aws_access_key_id=credentials.get("access_key", "unused"),
                aws_secret_access_key=credentials.get("secret_key", "unused"),
                aws_session_token=credentials.get("token"),
                config=Config(
                    signature_version=UNSIGNED if service == "signin" else "v4",
                    proxies={},
                    retries={"total_max_attempts": 1},
                    connect_timeout=min(10, budget.deadline - time.monotonic()),
                    read_timeout=min(30, budget.deadline - time.monotonic()),
                ),
            )
            clients.append(instance)
            return instance

        with (
            deadline_alarm(budget.deadline),
            patch.object(
                URLLib3Session,
                "send",
                lambda physical, request: transport.forward(physical, request),
            ),
        ):
            creds = LoginCredentialFetcher(
                cache.identity, cache, client
            ).refresh_credentials()
            if (
                creds["account_id"] != ACCOUNT
                or (
                    datetime.datetime.fromisoformat(creds["expiry_time"])
                    - datetime.datetime.now(datetime.UTC)
                ).total_seconds()
                < 120
            ):
                budget.stop("credential-expiry-or-account-held")
            identity = client("sts", credentials=creds).get_caller_identity()
            if (
                identity.get("Account") != ACCOUNT
                or identity.get("Arn") != cache.identity
            ):
                budget.stop("identity-mismatch-held")
            identity.clear()
            transport.identity_verified = True
            budget.record(event="identity-verified", count=budget.count)
            configuration = client(
                "lambda", credentials=creds
            ).get_function_configuration(FunctionName=FUNCTION)
            try:
                return validate_response(
                    configuration, manifest, checker or archive_checker()
                )
            finally:
                configuration.clear()
                creds.clear()
    finally:
        for instance in clients:
            instance.close()
        cache.close()
        logging.disable(old_logging)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["check", "observe"])
    parser.add_argument("--output", type=Path)
    parser.add_argument("--review-commit")
    args = parser.parse_args()
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    manifest = preflight()
    if args.action == "check":
        print(
            json.dumps(
                {
                    "localChecksPassed": True,
                    "prior": PRIOR,
                    "additionalLimit": 3,
                    "seconds": SECONDS,
                    "awsCalls": 0,
                }
            )
        )
        return
    git = lambda *a: (
        __import__("subprocess").check_output(["git", *a], cwd=ROOT, text=True).strip()
    )
    if (
        not args.review_commit
        or git("rev-parse", "HEAD") != args.review_commit
        or git("status", "--porcelain", "--untracked-files=all")
    ):
        raise ValueError("clean-reviewed-tooling-held")
    base = ROOT / ".build/project-environments-key-observation"
    if (
        args.output is None
        or args.output.resolve().parent != base.resolve()
        or args.output.exists()
    ):
        raise ValueError("fresh-output-held")
    base.mkdir(mode=0o700, exist_ok=True)
    persist(
        base / "window-from-296.json",
        {
            "prior": PRIOR,
            "additionalLimit": 3,
            "seconds": SECONDS,
            "reviewCommit": args.review_commit,
            "review8Sha256": REVIEW8_SHA,
            "manifestSha256": MANIFEST_SHA,
        },
    )
    args.output.mkdir(mode=0o700)
    budget = Budget(args.output / "attempts.jsonl")
    result = {
        "allFourAuthenticatedEmptyBindings": False,
        "candidateAdoptionPerformed": False,
    }
    try:
        result.update(observe(budget, manifest))
        budget.record(
            event="validated",
            count=budget.count,
            allFourAuthenticatedEmptyBindings=True,
        )
    except BaseException as error:  # noqa: BLE001 - sanitize every uncertain outcome
        if not budget.stopped:
            try:
                budget.stop(
                    error.category
                    if isinstance(error, ObservationHeld)
                    else "observation-or-validation-held-no-restart"
                )
            except Stop:
                pass
    finally:
        rows = [
            json.loads(line)
            for line in (args.output / "attempts.jsonl").read_text().splitlines()
        ]
        attempts = [x["count"] for x in rows if x["event"] == "attempt"]
        outcomes = [x["attempt"] for x in rows if x["event"] == "outcome"]
        result.update(
            priorCalls=PRIOR,
            newCalls=len(attempts),
            cumulativeCalls=budget.count,
            cumulativeCeiling=CEILING,
            campaignMaximum=MAXIMUM,
            ledgerIntegrity=attempts == list(range(PRIOR + 1, budget.count + 1))
            and sorted(outcomes) == attempts,
            seconds=SECONDS,
            started=budget.started,
            deadline=budget.deadline,
            proceduralAllowanceUsd=1,
            billingMeasured=False,
            automaticContinuation=False,
        )
        persist(args.output / "result.json", result)
        budget.close()
    print(json.dumps(result))
    raise SystemExit(0 if result["allFourAuthenticatedEmptyBindings"] else 1)


if __name__ == "__main__":
    try:
        main()
    except Exception:  # noqa: BLE001 - never publish raw SDK/key material
        raise SystemExit(
            "Key observation held. No retry or continuation; inspect sanitized durable records."
        ) from None
