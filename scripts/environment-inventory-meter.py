"""Read-only inventory transport; never logs arguments, bodies or credentials.

This meters botocore clients in this process only. It is NOT an SST/provider meter.
"""

import json
import os
import threading
import time
from pathlib import Path

from botocore.client import BaseClient
from botocore.httpsession import URLLib3Session

ALLOWED = {
    "sts": {"GetCallerIdentity"},
    "ssm": {"GetParameter", "DescribeParameters"},
    "lambda": {
        "GetFunction",
        "GetFunctionConfiguration",
        "GetFunctionConcurrency",
        "ListProvisionedConcurrencyConfigs",
        "GetAccountSettings",
        "ListEventSourceMappings",
        "GetPolicy",
    },
    "iam": {
        "GetRole",
        "ListRolePolicies",
        "GetRolePolicy",
        "ListAttachedRolePolicies",
        "GetPolicy",
        "GetPolicyVersion",
    },
    "dynamodb": {
        "GetItem",
        "Scan",
        "DescribeTable",
        "DescribeTimeToLive",
        "DescribeContinuousBackups",
    },
    "s3": {
        "GetObject",
        "HeadObject",
        "HeadBucket",
        "ListObjectsV2",
        "GetBucketLocation",
        "GetBucketPolicy",
        "GetPublicAccessBlock",
        "GetBucketVersioning",
        "GetBucketEncryption",
        "GetBucketLifecycleConfiguration",
    },
    "sqs": {"GetQueueAttributes"},
    "events": {"DescribeRule", "ListTargetsByRule"},
    "apigatewayv2": {"GetApi", "GetStage", "GetRoutes", "GetIntegrations"},
    "cognito-idp": {"DescribeUserPool", "DescribeUserPoolClient"},
    "cloudfront": {
        "GetDistribution",
        "GetFunction",
        "DescribeFunction",
        "GetCachePolicy",
        "GetResponseHeadersPolicy",
        "GetOriginAccessControl",
    },
    "cloudfront-keyvaluestore": {"DescribeKeyValueStore", "ListKeys"},
    "kms": {"DescribeKey", "GetKeyPolicy", "GetKeyRotationStatus"},
    "logs": {"DescribeLogGroups"},
    "ecr": {"DescribeRepositories"},
    "freetier": {"GetAccountPlanState"},
}


class Meter:
    def __init__(
        self, ledger, *, limit=500, seconds=3600, clock=time.monotonic, resume=False
    ):
        previous = (
            [json.loads(x) for x in Path(ledger).read_text().splitlines()]
            if resume
            else []
        )
        self.fd = os.open(
            ledger,
            os.O_WRONLY | (os.O_APPEND if resume else os.O_CREAT | os.O_EXCL),
            0o600,
        )
        self.clock, self.limit, self.seconds = clock, limit, seconds
        self.start = None
        self.count = 0
        self.stopped = False
        self.lock = threading.Lock()
        self.pending = threading.local()
        if previous:
            if (
                any(r["event"] == "stop" for r in previous)
                or previous[0].get("limit") != limit
                or previous[0].get("seconds") != seconds
            ):
                os.close(self.fd)
                raise RuntimeError("inventory-resume-held")
            attempts = [r for r in previous if r["event"] == "attempt"]
            self.count = len(attempts)
            if attempts:
                elapsed = time.time() - attempts[0]["at"]
                if elapsed < 0 or elapsed >= seconds:
                    os.close(self.fd)
                    raise RuntimeError("inventory-window-expired")
                self.start = clock() - elapsed
            self.record(event="resume", count=self.count)
        else:
            self.record(event="created", limit=limit, seconds=seconds)

    def record(self, **data):
        os.write(self.fd, (json.dumps(dict(at=time.time(), **data)) + "\n").encode())
        os.fsync(self.fd)

    def admit(self, service, operation):
        with self.lock:
            if self.stopped:
                raise RuntimeError("inventory-meter-latched")
            if self.start is None:
                self.start = self.clock()
            if self.count >= self.limit or self.clock() - self.start >= self.seconds:
                self.stopped = True
                self.record(event="stop", reason="inventory-limit", count=self.count)
                raise RuntimeError("inventory-limit")
            self.count += 1
            self.record(
                event="attempt", count=self.count, service=service, operation=operation
            )
            return self.count

    def install(self, *, allow_refresh=False):
        original = BaseClient._make_api_call
        original_send = URLLib3Session.send
        meter = self

        def send(session, request):
            attempt = getattr(meter.pending, "attempt", None)
            if attempt is None:
                raise RuntimeError("unmetered-transport-held")
            try:
                response = original_send(session, request)
                meter.record(
                    event="outcome", attempt=attempt, status=response.status_code
                )
                return response
            except Exception:
                meter.record(event="outcome", attempt=attempt, transportError=True)
                raise
            finally:
                meter.pending.attempt = None

        URLLib3Session.send = send

        def guarded(client, operation, kwargs):
            service = client.meta.service_model.service_name
            if operation not in ALLOWED.get(service, set()) and not (
                allow_refresh
                and service == "signin"
                and operation == "CreateOAuth2Token"
            ):
                raise RuntimeError("inventory-operation-held")
            event = (
                "before-send."
                + client.meta.service_model.service_id.hyphenize()
                + "."
                + operation
            )
            uid = "environment-inventory-meter"

            def before_send(**_):
                meter.pending.attempt = meter.admit(service, operation)

            client.meta.events.register(event, before_send, unique_id=uid)
            try:
                return original(client, operation, kwargs)
            finally:
                client.meta.events.unregister(event, unique_id=uid)

        BaseClient._make_api_call = guarded
        return original

    def finish(self):
        self.record(
            event="finished",
            count=self.count,
            elapsedSeconds=0 if self.start is None else self.clock() - self.start,
        )
        os.close(self.fd)


def summary(path):
    rows = [json.loads(x) for x in Path(path).read_text().splitlines()]
    return {
        "physicalAttempts": sum(r["event"] == "attempt" for r in rows),
        "events": rows,
    }
