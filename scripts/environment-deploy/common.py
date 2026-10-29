"""Host-owned cumulative campaign ledger. No request values or credentials logged."""

import hashlib
import json
import os
import sqlite3
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PRIOR = 299
PHASES = ("collect", "close", "deploy", "verify", "activate", "final")


class Held(Exception):
    pass


def digest(value):
    return hashlib.sha256(value).hexdigest()


def canonical(value):
    return json.dumps(
        value, sort_keys=True, separators=(",", ":"), ensure_ascii=False
    ).encode()


def strict_json(data):
    def pairs(items):
        result = {}
        for k, v in items:
            if k in result:
                raise Held("duplicate-json")
            result[k] = v
        return result

    return json.loads(
        data,
        object_pairs_hook=pairs,
        parse_constant=lambda _: (_ for _ in ()).throw(Held("nonfinite-json")),
    )


def boot():
    return Path("/proc/sys/kernel/random/boot_id").read_text().strip()


def syncdir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def private_write(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        while data:
            n = os.write(fd, data)
            if n <= 0:
                raise OSError()
            data = data[n:]
        os.fsync(fd)
    finally:
        os.close(fd)
    syncdir(path.parent)


class Campaign:
    def __init__(self, directory, *, create=None, clock=time.monotonic, boot_id=boot):
        self.directory = Path(directory)
        self.clock, self.boot_id = clock, boot_id
        path = self.directory / "campaign.sqlite"
        if create is not None:
            self.directory.mkdir(mode=0o700, parents=False, exist_ok=False)
        elif not path.is_file() or path.is_symlink():
            raise Held("original-campaign-required")
        self.db = sqlite3.connect(path, timeout=5, isolation_level=None)
        os.chmod(path, 0o600)
        self.db.execute("PRAGMA journal_mode=WAL")
        self.db.execute("PRAGMA synchronous=FULL")
        if create is not None:
            self.db.executescript(
                "CREATE TABLE config(k TEXT PRIMARY KEY,v TEXT);CREATE TABLE attempts(n INTEGER PRIMARY KEY,phase TEXT,op TEXT,write INTEGER,status INTEGER,outcome INTEGER DEFAULT 0);CREATE TABLE events(id INTEGER PRIMARY KEY,at REAL,data TEXT);"
            )
            now = clock()
            if (
                set(create)
                != {
                    "prior",
                    "maximum",
                    "seconds",
                    "manifestSha256",
                    "scopeSha256",
                    "reviewCommit",
                    "authorizationSha256",
                }
                or create["prior"] != PRIOR
                or type(create["maximum"]) is not int
                or not PRIOR < create["maximum"] <= 1799
                or type(create["seconds"]) is not int
                or not 0 < create["seconds"] <= 3300
            ):
                raise Held("authorization-budget-held")
            for k, v in {
                **create,
                "count": PRIOR,
                "started": now,
                "deadline": now + create["seconds"],
                "boot": boot_id(),
                "stopped": False,
                "nextPhase": 0,
                "activePhase": None,
                "closedDeadline": None,
            }.items():
                self.put(k, v)
            self.event(
                "created",
                prior=PRIOR,
                maximum=create["maximum"],
                deadline=now + create["seconds"],
            )
            syncdir(self.directory)

    def get(self, key):
        row = self.db.execute("SELECT v FROM config WHERE k=?", (key,)).fetchone()
        return json.loads(row[0]) if row else None

    def put(self, key, value):
        self.db.execute(
            "INSERT OR REPLACE INTO config VALUES(?,?)", (key, json.dumps(value))
        )

    def event(self, event, **data):
        self.db.execute(
            "INSERT INTO events(at,data) VALUES(?,?)",
            (self.clock(), json.dumps({"event": event, **data}, separators=(",", ":"))),
        )

    def stop(self, reason):
        self.put("stopped", True)
        self.event("stop", reason=reason, count=self.get("count"))
        raise Held(reason)

    def check(self):
        if self.get("stopped"):
            raise Held("latched-stop")
        if (
            self.get("boot") != self.boot_id()
            or self.clock() < self.get("started")
            or self.clock() >= self.get("deadline")
        ):
            self.stop("original-window-exhausted-or-boot-changed")
        if self.get("closedDeadline") is not None and self.clock() >= self.get(
            "closedDeadline"
        ):
            self.stop("closed-window-exhausted")

    def begin(self, phase):
        self.db.execute("BEGIN IMMEDIATE")
        try:
            self.check()
            if (
                self.get("activePhase") is not None
                or self.get("nextPhase") >= len(PHASES)
                or PHASES[self.get("nextPhase")] != phase
            ):
                self.stop("phase-replay-or-order")
            self.put("activePhase", phase)
            if phase == "close":
                self.put(
                    "closedDeadline", min(self.get("deadline"), self.clock() + 1800)
                )
            self.event("phase-start", phase=phase)
        finally:
            self.db.execute("COMMIT")

    def admit(self, operation, write):
        self.db.execute("BEGIN IMMEDIATE")
        try:
            self.check()
            phase = self.get("activePhase")
            if phase is None or self.get("count") >= self.get("maximum"):
                self.stop("phase-or-call-limit")
            n = self.get("count") + 1
            self.put("count", n)
            self.db.execute(
                "INSERT INTO attempts(n,phase,op,write) VALUES(?,?,?,?)",
                (n, phase, operation, int(write)),
            )
            self.event(
                "attempt", count=n, phase=phase, operation=operation, write=write
            )
            return n
        finally:
            self.db.execute("COMMIT")

    def outcome(self, n, status=None):
        self.db.execute("BEGIN IMMEDIATE")
        try:
            row = self.db.execute(
                "SELECT write,outcome FROM attempts WHERE n=?", (n,)
            ).fetchone()
            if row is None or row[1]:
                self.stop("duplicate-or-missing-outcome")
            self.db.execute(
                "UPDATE attempts SET status=?,outcome=1 WHERE n=?", (status, n)
            )
            self.event(
                "outcome", attempt=n, status=status, transportError=status is None
            )
            if status is None or not (
                200 <= status < 300 or (status == 404 and not row[0])
            ):
                self.stop("write-uncertain-or-http-transport-failure")
        finally:
            self.db.execute("COMMIT")

    def finish(self, phase, proof):
        self.check()
        if (
            self.get("activePhase") != phase
            or self.db.execute(
                "SELECT COUNT(*) FROM attempts WHERE outcome=0"
            ).fetchone()[0]
        ):
            self.stop("incomplete-phase-or-outcomes")
        self.put("proof:" + phase, proof)
        self.put("activePhase", None)
        self.put("nextPhase", self.get("nextPhase") + 1)
        if phase == "activate":
            self.put("closedDeadline", None)
        self.event("phase-complete", phase=phase, count=self.get("count"))

    def close(self):
        self.db.close()
