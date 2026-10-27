"""Closed read-only AWS protocol policy and one durable shared preview budget."""

import json
import os
import re
import time
from urllib.parse import parse_qs, urlsplit

PARENT_SHA = "1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171"
SOURCE = "44f1fb79d943501c95e06fcc1a6114111a7c1ea4"
PRIOR = 135
MAXIMUM = 500
SECONDS = 3600
JSON_READS = {
    "ssm": {"GetParameter", "GetParameters", "DescribeParameters"},
    "dynamodb": {
        "ListTables",
        "DescribeTable",
        "DescribeTimeToLive",
        "DescribeContinuousBackups",
        "GetItem",
        "ListTagsOfResource",
    },
    "ecr": {"DescribeRepositories", "ListTagsForResource", "DescribeImages"},
    "logs": {"DescribeLogGroups", "ListTagsLogGroup", "ListTagsForResource"},
    "cognito-idp": {
        "DescribeUserPool",
        "DescribeUserPoolClient",
        "ListTagsForResource",
    },
    "events": {"DescribeRule", "ListTargetsByRule", "ListTagsForResource"},
    "kms": {
        "DescribeKey",
        "GetKeyPolicy",
        "GetKeyRotationStatus",
        "ListAliases",
        "ListResourceTags",
    },
    "tagging": {"GetResources"},
    "cloudcontrolapi": {"GetResource", "GetResourceRequestStatus", "ListResources"},
}
QUERY_READS = {
    "sts": {"GetCallerIdentity"},
    "iam": {
        "GetRole",
        "GetRolePolicy",
        "ListRolePolicies",
        "ListAttachedRolePolicies",
        "GetPolicy",
        "GetPolicyVersion",
        "ListPolicyVersions",
        "ListRoleTags",
        "ListInstanceProfilesForRole",
    },
    "ec2": {
        "DescribeRegions",
        "DescribeAvailabilityZones",
        "DescribeVpcEndpoints",
        "DescribeSecurityGroups",
        "DescribeSubnets",
        "DescribeVpcs",
        "DescribeNetworkInterfaces",
    },
    "sqs": {"GetQueueAttributes", "GetQueueUrl", "ListQueueTags"},
}
JSON_READS["sqs"] = QUERY_READS["sqs"]
REST = {"lambda", "apigateway", "cloudfront", "cloudfront-keyvaluestore"}
BUCKETS = {
    "sst-state-moshrdrrbeba",
    "sst-asset-moshrdrrbeba",
    "kurier-dev-api-web-fc2927a49a0b246240e5a17774",
    "executionevidence-71a0f7a",
}


class Stop(Exception):
    pass


def service(host):
    host = host.lower()
    if host in {"iam.amazonaws.com", "cloudfront.amazonaws.com", "sts.amazonaws.com"}:
        return host.split(".")[0]
    if re.fullmatch(r"(?:[a-z0-9-]+\.)?cloudfront-kvs\.global\.api\.aws", host):
        return "cloudfront-keyvaluestore"
    if host in {"s3.us-east-2.amazonaws.com", "s3.amazonaws.com"}:
        return "s3"
    if any(
        host in {b + ".s3.us-east-2.amazonaws.com", b + ".s3.amazonaws.com"}
        for b in BUCKETS
    ):
        return "s3"
    for name in set(JSON_READS) | set(QUERY_READS) | REST:
        if host in {name + ".us-east-2.amazonaws.com", name + ".us-east-2.api.aws"}:
            return name
    if host == "api.ecr.us-east-2.amazonaws.com":
        return "ecr"
    raise Stop("endpoint-held")


def classify(method, host, path, headers, body):
    svc = service(host)
    if len(body) > 1024 * 1024:
        raise Stop("request-size")
    if svc == "s3":
        if method not in {"GET", "HEAD"}:
            raise Stop("s3-mutation-held")
        if (
            host in {"s3.us-east-2.amazonaws.com", "s3.amazonaws.com"}
            and urlsplit(path).path.split("/")[1] not in BUCKETS
        ):
            raise Stop("bucket-held")
        return svc, method
    if svc in REST:
        if method not in {"GET", "HEAD"}:
            raise Stop("rest-mutation-held")
        return svc, method
    if method != "POST":
        raise Stop("protocol-held")
    targets = (
        headers.get_all("x-amz-target")
        if hasattr(headers, "get_all")
        else ([headers["x-amz-target"]] if "x-amz-target" in headers else [])
    )
    if len(targets) > 1:
        raise Stop("duplicate-target")
    if svc in JSON_READS and targets:
        target = targets[0] if targets else ""
        op = target.split(".")[-1]
        try:
            payload = json.loads(body)
        except (ValueError, UnicodeError):
            raise Stop("invalid-protocol") from None
        if not isinstance(payload, dict) or op not in JSON_READS[svc]:
            raise Stop("json-operation-held")
        if svc == "ssm" and op in {"GetParameter", "GetParameters"}:
            names = (
                [payload.get("Name")]
                if op == "GetParameter"
                else payload.get("Names", [])
            )
            if not names or any(
                n
                not in {
                    "/sst/bootstrap",
                    "/sst/passphrase/kurier/dev-api",
                    "/kurier/dev-api/cursor-key",
                }
                for n in names
            ):
                raise Stop("parameter-held")
        return svc, op
    if targets or svc not in QUERY_READS:
        raise Stop("protocol-conflict")
    try:
        args = parse_qs(body.decode("ascii"), strict_parsing=True)
    except (ValueError, UnicodeError):
        raise Stop("invalid-query") from None
    action = args.get("Action", [])
    if len(action) != 1 or action[0] not in QUERY_READS[svc]:
        raise Stop("query-operation-held")
    return svc, action[0]


class Budget:
    def __init__(
        self,
        ledger,
        *,
        prior=PRIOR,
        maximum=MAXIMUM,
        seconds=SECONDS,
        clock=time.monotonic,
    ):
        self.clock = clock
        self.started = clock()
        self.prior = prior
        self.count = prior
        self.maximum = maximum
        self.seconds = seconds
        self.stopped = False
        self.fd = os.open(ledger, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        self.record(
            event="created",
            prior=prior,
            maximum=maximum,
            seconds=seconds,
            parentSha256=PARENT_SHA,
        )

    def record(self, **v):
        raw = (
            json.dumps(dict(at=time.time(), **v), separators=(",", ":")) + "\n"
        ).encode()
        # Do not forward unless the entire receipt is durably recorded.
        while raw:
            n = os.write(self.fd, raw)
            if n <= 0:
                raise OSError("ledger-write")
            raw = raw[n:]
        os.fsync(self.fd)

    def stop(self, reason):
        if not self.stopped:
            self.stopped = True
            self.record(event="stop", reason=reason, count=self.count)
        raise Stop(reason)

    def check(self):
        if self.stopped:
            raise Stop("latched-stop")
        if self.clock() - self.started >= self.seconds:
            self.stop("time-limit")

    def admit(self, svc, op, protocol):
        self.check()
        if self.count >= self.maximum:
            self.stop("call-limit")
        self.count += 1
        self.record(
            event="attempt",
            count=self.count,
            service=svc,
            operation=op,
            protocol=protocol,
        )
        return self.count

    def outcome(self, attempt, status=None, error=False):
        self.record(
            event="outcome", attempt=attempt, status=status, transportError=error
        )
        if error:
            self.stop("transport-failure")
        if status in {401, 403}:
            self.stop("authorization-denied")
        if status is not None and 300 <= status < 400:
            self.stop("redirect-held")
        if self.count >= self.maximum:
            self.stop("call-limit")

    def close(self):
        os.close(self.fd)
