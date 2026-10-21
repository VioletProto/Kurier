"""mitmproxy addon. No flow recording, body/header logs or request-value receipts."""

import hashlib
import json
import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from budget import Budget, MeterStop
from cleanup import parent_proof, cleanup_allowed, validate_scope
from mitmproxy import http


class AttemptMeter:
    def __init__(self):
        parent = os.environ.get("KURIER_METER_CLEANUP_PARENT")
        self.cleanup_scope = (
            validate_scope(
                json.load(
                    open(os.environ["KURIER_METER_CLEANUP_SCOPE"], encoding="utf8")
                )
            )
            if parent
            else None
        )
        self.budget = Budget(
            os.environ["KURIER_METER_LEDGER"],
            cleanup_parent=parent_proof(parent) if parent else None,
        )

    def request(self, flow):
        try:
            request = flow.request
            if request.scheme != "https" or request.port != 443:
                self.budget.stop("insecure-origin")
            category, operation, write, poll = self.budget.classify(
                request.method,
                request.host,
                request.path,
                request.headers,
                request.raw_content or b"",
            )
            # Default read-only. The explicit phase file is non-secret local
            # operator state; AWS stage writes still use the reviewed activator.
            control = json.load(
                open(os.environ["KURIER_METER_CONTROL"], encoding="utf8")
            )
            allowed = control.get("writeCategories", [])
            if self.cleanup_scope:
                if category == "static" or (
                    category == "api"
                    and not cleanup_allowed(
                        category,
                        request.method,
                        request.host,
                        request.path,
                        operation,
                        request.raw_content or b"",
                        self.cleanup_scope,
                    )
                ):
                    self.budget.stop("cleanup-only-scope")
                if write:
                    allowed = (
                        [category]
                        if cleanup_allowed(
                            category,
                            request.method,
                            request.host,
                            request.path,
                            operation,
                            request.raw_content or b"",
                            self.cleanup_scope,
                        )
                        else []
                    )
            if category == "aws" and write and not self.cleanup_scope:
                if operation not in control.get("awsWriteOperations", []):
                    allowed = []
                # IAM/bootstrap/account/plan changes are never part of rollout.
                if request.host.startswith(
                    ("iam.", "organizations.", "servicequotas.", "freetier.")
                ):
                    allowed = []
            key = hashlib.sha256(
                (request.method + request.host + request.path).encode()
            ).digest()
            attempt = self.budget.admit(
                category,
                operation,
                write,
                poll,
                key,
                write_allowed=category in allowed,
                protocol=request.http_version,
            )
            flow.metadata["kurier_attempt"] = (attempt, write)
            # Local tests route classified AWS/API hosts to a controlled origin.
            # Live mode has no override and always verifies upstream TLS.
            test_origin = os.environ.get("KURIER_METER_LOCAL_ORIGIN")
            if test_origin:
                request.scheme, request.host, request.port = (
                    "http",
                    "127.0.0.1",
                    int(test_origin),
                )
        except (MeterStop, OSError, ValueError, KeyError):
            self.budget.stopped = True
            flow.response = http.Response.make(
                599, b'{"meterStopped":true}', {"Content-Type": "application/json"}
            )

    def response(self, flow):
        if "kurier_attempt" in flow.metadata:
            attempt, write = flow.metadata["kurier_attempt"]
            try:
                self.budget.outcome(attempt, flow.response.status_code, write=write)
            except MeterStop:
                pass

    def error(self, flow):
        if "kurier_attempt" in flow.metadata:
            attempt, write = flow.metadata["kurier_attempt"]
            try:
                self.budget.outcome(attempt, write=write, transport_error=True)
            except MeterStop:
                pass

    def done(self):
        self.budget.close()


addons = [AttemptMeter()]
