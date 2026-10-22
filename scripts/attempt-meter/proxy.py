"""mitmproxy addon. No flow recording, body/header logs or request-value receipts."""

import hashlib
import json
import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
from budget import Budget, MeterStop
from cleanup import parent_proof, cleanup_allowed, validate_scope
from cognito import held_diagnostics
from hl_resume import parent_proof as hl_parent_proof
from mitmproxy import http


class AttemptMeter:
    def __init__(self):
        self.hl_only = os.environ.get("KURIER_METER_HL_ONLY") == "1"
        self.hl_creations = set()
        self.verify_only = os.environ.get("KURIER_METER_VERIFY_ONLY") == "1"
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
            continuation_parent=hl_parent_proof(os.environ["KURIER_METER_HL_PARENT"])
            if os.environ.get("KURIER_METER_HL_PARENT")
            else None,
            **(
                {
                    "aws_limit": 100,
                    "api_limit": 0,
                    "poll_limit": 0,
                    "seconds": 600,
                    "history": {
                        "purpose": "focused-read-only-cleanup-verification",
                        "priorSteps": 18,
                        "priorSends": 16,
                        "priorIdentitiesAndProjects": 6,
                        "priorKnownApiCalls": 116,
                        "priorKnownPolls": 11,
                        "priorKnownFinalAttemptAwsCharges": 917,
                        "priorKnownSecondAttemptAwsCharges": 574,
                        "priorAwsReads": None,
                        "priorBillingUsd": None,
                    },
                }
                if self.verify_only
                else {
                    "history": {
                        "purpose": "H-L-only-continuation",
                        "priorSteps": 18,
                        "priorSends": 16,
                        "priorIdentitiesAndProjects": 6,
                        "priorKnownApiCalls": 116,
                        "priorKnownPolls": 11,
                        "priorKnownFinalAttemptAwsCharges": 917,
                        "priorKnownSecondAttemptAwsCharges": 574,
                        "priorCleanupVerificationAwsCharges": 20,
                        "priorAwsReads": None,
                        "priorBillingUsd": None,
                    }
                }
                if self.hl_only
                else {}
            ),
        )

    def request(self, flow):
        request = flow.request
        diagnostic = held_diagnostics(request.method, request.host)
        try:
            if request.scheme != "https" or request.port != 443:
                self.budget.stop("insecure-origin")
            category, operation, write, poll = self.budget.classify(
                request.method,
                request.host,
                request.path,
                request.headers,
                request.raw_content or b"",
            )
            diagnostic = held_diagnostics(
                request.method, request.host, category, operation
            )
            if self.verify_only and (
                category != "aws"
                or operation == "CognitoAuthPreflight"
                or (write and operation != "CreateOAuth2Token")
            ):
                self.budget.stop("verification-read-only")
            # Default read-only. The explicit phase file is non-secret local
            # operator state; AWS stage writes still use the reviewed activator.
            control = json.load(
                open(os.environ["KURIER_METER_CONTROL"], encoding="utf8")
            )
            allowed = control.get("writeCategories", [])
            if self.verify_only:
                allowed = ["aws"] if operation == "CreateOAuth2Token" else []
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
            if self.hl_only and category == "aws" and write:
                if operation not in (
                    "CreateOAuth2Token",
                    "InitiateAuth",
                    "RespondToAuthChallenge",
                    "RevokeToken",
                    "GetTokensFromRefreshToken",
                    "AdminCreateUser",
                    "AdminSetUserPassword",
                    "AdminUserGlobalSignOut",
                    "AdminDeleteUser",
                ):
                    self.budget.stop("H-L-resource-write-forbidden")
                if operation == "AdminCreateUser":
                    # Count before forwarding; even an uncertain outcome cannot replay.
                    if self.hl_creations:
                        self.budget.stop("H-L-single-disposable-identity")
                    self.hl_creations.add("creation-attempted")
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
        except (MeterStop, OSError, ValueError, KeyError) as error:
            self.budget.record(
                {
                    "event": "held-request",
                    **diagnostic,
                    "reason": str(error)
                    if isinstance(error, MeterStop)
                    else "meter-input-or-persistence-failure",
                }
            )
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
