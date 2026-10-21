"""Fail-closed operator transport budget. Never persists request material."""

import json
import os
import re
import time
from urllib.parse import parse_qs, urlsplit


class MeterStop(Exception):
    pass


class Budget:
    def __init__(
        self,
        ledger,
        *,
        clock=time.time,
        api_limit=500,
        aws_limit=1500,
        poll_limit=240,
        seconds=7200,
    ):
        self.clock = clock
        self.elapsed_clock = time.monotonic if clock is time.time else clock
        self.started = None
        self.ledger = ledger
        self.limits = dict(api=api_limit, aws=aws_limit, poll=poll_limit, static=200)
        self.seconds = seconds
        self.counts = dict(api=0, aws=0, poll=0, static=0)
        self.stopped = False
        self.poll_times = {}
        self.sequence = 0
        self.fd = os.open(ledger, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        self.record(
            dict(
                event="meter-created",
                priorSteps=4,
                priorSends=4,
                priorIdentitiesAndProjects=4,
                priorKnownApiCalls=48,
                priorKnownPolls=4,
                priorKnownSecondAttemptAwsCharges=574,
                priorAwsReads=None,
                priorBillingUsd=None,
            )
        )

    def record(self, event):
        self.sequence += 1
        raw = (
            json.dumps(
                dict(sequence=self.sequence, at=self.clock(), **event),
                separators=(",", ":"),
            ).encode()
            + b"\n"
        )
        os.write(self.fd, raw)
        os.fsync(self.fd)

    def stop(self, reason):
        if not self.stopped:
            self.stopped = True
            self.record(dict(event="stop", reason=reason, counts=self.counts))
        raise MeterStop(reason)

    def classify(self, method, host, path, headers, content):
        host = host.lower()
        if host == "8dnymkcoa0.execute-api.us-east-2.amazonaws.com":
            if not urlsplit(path).path.startswith("/api/v1/"):
                self.stop("non-application-route-or-direct-target")
            poll = method == "GET" and bool(
                re.search(
                    r"/(workflow-runs|deletion-operations)/[a-f0-9-]{36}$|/executions/[a-f0-9-]{36}/status$",
                    urlsplit(path).path,
                )
            )
            return "api", method, method not in ("GET", "HEAD", "OPTIONS"), poll
        if host == "d2kpq905qyh9zg.cloudfront.net":
            if method not in ("GET", "HEAD"):
                self.stop("static-write")
            return "static", method, False, False
        if not (
            host.endswith(".amazonaws.com")
            or host.endswith(".api.aws")
            or host == "us-east-2.signin.aws.amazon.com"
        ):
            self.stop("unclassified-origin")
        # Charge ALL operator AWS attempts against the read allowance. This is
        # conservative and avoids undercounting reads misclassified as writes.
        operation = headers.get("x-amz-target", "").split(".")[-1]
        if host in (
            "signin.us-east-2.amazonaws.com",
            "us-east-2.signin.aws.amazon.com",
        ):
            if method == "POST" and urlsplit(path).path == "/v1/token":
                operation = "CreateOAuth2Token"
        if not operation and method == "POST":
            if len(content) > 8 * 1024 * 1024:
                self.stop("aws-body-bound")
            values = parse_qs(content.decode("utf8", errors="replace"))
            operation = values.get("Action", [""])[0]
            if not operation:
                operation = parse_qs(urlsplit(path).query).get("Action", [""])[0]
        if not operation:
            operation = method
        if not re.fullmatch(r"[A-Za-z][A-Za-z0-9]{0,100}", operation):
            self.stop("unclassified-operation")
        read = operation.startswith(
            (
                "Get",
                "List",
                "Describe",
                "Head",
                "Lookup",
                "Scan",
                "Query",
                "BatchGet",
                "Filter",
                "AdminGet",
                "AdminList",
            )
        ) or method in ("GET", "HEAD")
        if operation == "GetTokensFromRefreshToken":
            read = False
        return "aws", operation, not read, False

    def admit(
        self,
        category,
        operation,
        write,
        poll=False,
        poll_key=None,
        *,
        write_allowed=False,
        protocol=None,
    ):
        if self.stopped:
            raise MeterStop("latched-stop")
        if self.started is None:
            self.started = self.elapsed_clock()
        if self.elapsed_clock() - self.started >= self.seconds:
            self.stop("time-limit")
        if write and not write_allowed:
            self.stop("write-held")
        charges = [category] + (["poll"] if poll else [])
        for key in charges:
            if key in self.limits and self.counts[key] >= self.limits[key]:
                self.stop(key + "-limit")
        if (
            poll
            and poll_key in self.poll_times
            and self.elapsed_clock() - self.poll_times[poll_key] < 2
        ):
            self.stop("poll-interval")
        if poll:
            self.poll_times[poll_key] = self.elapsed_clock()
        for key in charges:
            self.counts[key] += 1
        # Persist and fsync BEFORE the proxy is permitted to forward an attempt.
        self.record(
            dict(
                event="attempt",
                category=category,
                operation=operation,
                write=write,
                poll=poll,
                protocol=protocol,
                counts=self.counts.copy(),
            )
        )
        return self.sequence

    def outcome(self, attempt, status=None, *, write=False, transport_error=False):
        self.record(
            dict(
                event="outcome",
                attempt=attempt,
                status=status,
                transportError=transport_error,
            )
        )
        if write and (transport_error or status is None or status >= 500):
            self.stop("uncertain-write")

    def close(self):
        os.close(self.fd)
