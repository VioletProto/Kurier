"""Host-only, one-use authentication to provider budget handoff. No network."""

import hashlib
import json
import math
import os
import time
import uuid
from pathlib import Path

from accounting import MAXIMUM, PRIOR, baseline
from policy import SOURCE, Budget, Stop

SECONDS = 3600


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def sync_directory(directory):
    fd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def persist(path, value):
    # Exclusive creation consumes receipts even if the process crashes. No resume.
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        raw = (json.dumps(value, sort_keys=True) + "\n").encode()
        while raw:
            n = os.write(fd, raw)
            if n <= 0:
                raise OSError("handoff-write")
            raw = raw[n:]
        os.fsync(fd)
    finally:
        os.close(fd)
    sync_directory(Path(path).parent)


def boot_id():
    return Path("/proc/sys/kernel/random/boot_id").read_text().strip()


def claim_campaign(root, directory):
    # A different output path must not replay the same cumulative165 baseline.
    # A later authorized run needs reviewed accounting for any calls spent here.
    persist(
        root / ".build/project-environments-preview/window-from-165.json",
        {"directory": str(Path(directory).resolve()), **baseline(root)},
    )


def begin(directory, root, *, clock=time.monotonic):
    directory = Path(directory).resolve()
    directory.mkdir(mode=0o700, exist_ok=False)
    sync_directory(directory.parent)
    start = clock()
    window = dict(
        version=1,
        id=str(uuid.uuid4()),
        directory=str(directory),
        source=SOURCE,
        bootId=boot_id(),
        started=start,
        startedAt=time.time(),
        deadline=start + SECONDS,
        seconds=SECONDS,
        **baseline(root),
    )
    persist(directory / "window.json", window)
    return window


def window_at(directory, root, *, clock=time.monotonic):
    directory = Path(directory).resolve()
    w = json.loads((directory / "window.json").read_text())
    if (
        w.get("version") != 1
        or w.get("source") != SOURCE
        or w.get("directory") != str(directory)
        or w.get("bootId") != boot_id()
        or w.get("seconds") != SECONDS
        or any(w.get(k) != v for k, v in baseline(root).items())
        or not all(
            type(w.get(k)) in (float, int) and math.isfinite(w[k])
            for k in ("started", "deadline")
        )
        or w["deadline"] - w["started"] != SECONDS
        or not w["started"] <= clock() < w["deadline"]
    ):
        raise ValueError("handoff-window-invalid-or-expired")
    return w


class SharedBudget(Budget):
    """Reuse the reviewed physical-attempt ledger without changing its policy."""

    def __init__(self, ledger, window, *, prior, clock=time.monotonic):
        if not window["started"] <= clock() < window["deadline"]:
            raise Stop("time-limit")
        if not PRIOR <= prior < MAXIMUM:
            raise Stop("call-limit")
        self.window = window
        super().__init__(
            ledger, prior=prior, maximum=MAXIMUM, seconds=SECONDS, clock=clock
        )
        self.started = window["started"]
        self.record(event="shared-window", window=window)
        # File fsync alone does not persist a newly created ledger's directory
        # entry. Finish that before the first attempt can ever be forwarded.
        sync_directory(Path(ledger).parent)

    def record(self, **data):
        super().record(monotonic=self.clock(), **data)


def authentication_count(directory, window):
    rows = [
        json.loads(line)
        for line in (Path(directory) / "authentication.jsonl").read_text().splitlines()
    ]
    if (
        len(rows) < 5
        or rows[0].get("event") != "created"
        or rows[0].get("prior") != PRIOR
        or rows[0].get("maximum") != MAXIMUM
        or rows[0].get("seconds") != SECONDS
        or rows[1].get("event") != "shared-window"
        or rows[1].get("window") != window
        or rows[-1].get("event") != "authenticated"
        or rows[-1].get("identityVerified") is not True
    ):
        raise ValueError("authentication-ledger-shape")
    count, pending, operations = PRIOR, None, []
    previous_time = window["started"]
    for row in rows:
        stamp = row.get("monotonic")
        if (
            type(stamp) not in (float, int)
            or not math.isfinite(stamp)
            or not previous_time <= stamp < window["deadline"]
        ):
            raise ValueError("authentication-ledger-time")
        previous_time = stamp
    for row in rows[2:-1]:
        if row.get("event") == "attempt":
            op = (row.get("service"), row.get("operation"))
            if pending is not None or op not in {
                ("signin", "CreateOAuth2Token"),
                ("sts", "GetCallerIdentity"),
            }:
                raise ValueError("authentication-operation-or-pending")
            count += 1
            if row.get("count") != count or count > MAXIMUM:
                raise ValueError("authentication-ledger-sequence")
            operations.append(op)
            pending = count
        elif row.get("event") == "outcome":
            if (
                pending is None
                or row.get("attempt") != pending
                or row.get("transportError") is not False
                or type(row.get("status")) is not int
                or not 200 <= row["status"] < 300
            ):
                raise ValueError("authentication-outcome-uncertain")
            pending = None
        else:
            raise ValueError("authentication-stopped-or-unexpected-event")
    # No authentication retry, pagination, redirect, second refresh or extra STS.
    if operations not in [
        [("sts", "GetCallerIdentity")],
        [("signin", "CreateOAuth2Token"), ("sts", "GetCallerIdentity")],
    ]:
        raise ValueError("authentication-phase-order")
    if pending is not None or rows[-1].get("count") != count or count >= MAXIMUM:
        raise ValueError("authentication-incomplete-or-exhausted")
    return count


def seal(directory, root, *, clock=time.monotonic):
    directory = Path(directory).resolve()
    w = window_at(directory, root, clock=clock)
    count = authentication_count(directory, w)
    receipt = {
        "windowSha256": digest(directory / "window.json"),
        "authenticationSha256": digest(directory / "authentication.jsonl"),
        "prior": count,
    }
    persist(directory / "handoff.json", receipt)
    return directory / "handoff.json"


def validate(path, root, *, clock=time.monotonic):
    path = Path(path).resolve()
    if path.name != "handoff.json" or (path.parent / "provider-claim.json").exists():
        raise ValueError("handoff-reused-or-invalid-path")
    w = window_at(path.parent, root, clock=clock)
    receipt = json.loads(path.read_text())
    count = authentication_count(path.parent, w)
    if receipt != {
        "windowSha256": digest(path.parent / "window.json"),
        "authenticationSha256": digest(path.parent / "authentication.jsonl"),
        "prior": count,
    }:
        raise ValueError("handoff-hash-or-count")
    return w, count


def consume(path, root, *, clock=time.monotonic):
    w, count = validate(path, root, clock=clock)
    persist(
        Path(path).parent / "provider-claim.json",
        {"handoffSha256": digest(path), "prior": count, "deadline": w["deadline"]},
    )
    return w, count


def usage(directory):
    """Sanitized conservative terminal accounting, including truncated receipts."""
    count = PRIOR
    valid = True
    for p in [
        Path(directory) / "authentication.jsonl",
        Path(directory) / "provider/attempts.jsonl",
    ]:
        if not p.exists():
            continue
        for line in p.read_text().splitlines():
            try:
                row = json.loads(line)
                if row["event"] == "attempt":
                    if row["count"] != count + 1 or row["count"] > MAXIMUM:
                        valid = False
                    count = max(count, row["count"])
            except (ValueError, KeyError, TypeError):
                valid = False
    return {
        "priorCalls": PRIOR,
        "newCalls": count - PRIOR if valid else None,
        "cumulativeCalls": count if valid else None,
        "ledgerIntegrity": valid,
        "maximum": MAXIMUM,
        "proceduralAllowanceUsd": 1,
        "billingMeasured": False,
        "automaticContinuation": False,
        "requiresInspection": not valid,
    }
