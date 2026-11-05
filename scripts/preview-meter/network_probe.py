"""One authorized DNS/TCP/TLS diagnostic. No HTTP, credentials or retries."""

import argparse
import contextlib
import hashlib
import ipaddress
import json
import os
import re
import signal
import socket
import ssl
import struct
import subprocess
import time
from pathlib import Path

from accounting import MAXIMUM, PRIOR, baseline
from auth_diagnostics import transport_diagnostic
from handoff import persist, sync_directory

ROOT = Path(__file__).resolve().parents[2]
HOST = "signin.us-east-2.amazonaws.com"
RESOLVER = "127.0.0.53"
HOST_NAMESPACE = "net:[4026531833]"
SOURCE = "44f1fb79d943501c95e06fcc1a6114111a7c1ea4"
SECONDS = 45
LIMITS = {"dns": 2, "tcp": 2, "tls": 1}
REASONS = {
    "scope-held",
    "dns-invalid",
    "dns-rcode",
    "dns-failed",
    "dns-empty",
    "tcp-failed",
    "tls-failed",
    "tls-verified",
    "deadline",
    "limit",
    "persistence-failed",
    "executor-failed",
}


class Held(Exception):
    def __init__(self, reason):
        self.reason = reason if reason in REASONS else "executor-failed"


class Expired(BaseException):
    pass


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def public_address(value, version):
    try:
        ip = ipaddress.ip_address(value)
    except ValueError:
        raise Held("scope-held") from None
    if (
        ip.version != version
        or not ip.is_global
        or ip.is_multicast
        or ip.is_reserved
        or ip.is_unspecified
        or (ip.version == 6 and (ip.ipv4_mapped or ip.sixtofour or ip.teredo))
    ):
        raise Held("scope-held")
    return str(ip)


def wire_name(name):
    return (
        b"".join(bytes([len(x)]) + x.encode("ascii") for x in name.split(".")) + b"\0"
    )


def read_name(data, start):
    pos, end, labels, seen = start, None, [], set()
    for _ in range(64):
        if pos >= len(data) or pos in seen:
            raise Held("dns-invalid")
        seen.add(pos)
        size = data[pos]
        if size == 0:
            return ".".join(labels), end if end is not None else pos + 1
        if size & 0xC0 == 0xC0:
            if pos + 1 >= len(data):
                raise Held("dns-invalid")
            pointer = ((size & 63) << 8) | data[pos + 1]
            if pointer >= pos:
                raise Held("dns-invalid")
            end = end if end is not None else pos + 2
            pos = pointer
            continue
        if size & 0xC0 or pos + 1 + size > len(data):
            raise Held("dns-invalid")
        label = data[pos + 1 : pos + 1 + size]
        if not re.fullmatch(rb"[A-Za-z0-9_-]{1,63}", label):
            raise Held("dns-invalid")
        labels.append(label.decode("ascii").lower())
        if sum(map(len, labels)) + len(labels) - 1 > 253:
            raise Held("dns-invalid")
        pos += size + 1
    raise Held("dns-invalid")


def parse_dns(data, transaction, qtype):
    if len(data) < 12 or len(data) > 4096:
        raise Held("dns-invalid")
    ident, flags, questions, answers, authority, additional = struct.unpack(
        "!6H", data[:12]
    )
    if (
        ident != transaction
        or not flags & 0x8000
        or flags & 0x7A00
        or questions != 1
        or answers + authority + additional > 64
    ):
        raise Held("dns-invalid")
    question, pos = read_name(data, 12)
    if (
        pos + 4 > len(data)
        or question != HOST
        or struct.unpack("!HH", data[pos : pos + 4]) != (qtype, 1)
    ):
        raise Held("dns-invalid")
    pos += 4
    rcode = flags & 15
    if rcode:
        return {"rcode": rcode, "addresses": []}
    aliases, addresses = {}, []
    for index in range(answers + authority + additional):
        owner, pos = read_name(data, pos)
        if pos + 10 > len(data):
            raise Held("dns-invalid")
        kind, cls, _, length = struct.unpack("!HHIH", data[pos : pos + 10])
        pos += 10
        end = pos + length
        if end > len(data):
            raise Held("dns-invalid")
        if index < answers and cls == 1:
            if kind == 5:
                target, consumed = read_name(data, pos)
                if consumed != end or owner in aliases:
                    raise Held("dns-invalid")
                aliases[owner] = target
            elif kind == qtype:
                if length != (4 if qtype == 1 else 16):
                    raise Held("dns-invalid")
                addresses.append((owner, str(ipaddress.ip_address(data[pos:end]))))
        pos = end
    if pos != len(data):
        raise Held("dns-invalid")
    owners, current = {HOST}, HOST
    for _ in range(8):
        if current not in aliases:
            break
        current = aliases[current]
        if current in owners:
            raise Held("dns-invalid")
        owners.add(current)
    else:
        raise Held("dns-invalid")
    result = list(dict.fromkeys(ip for owner, ip in addresses if owner in owners))
    if len(result) > 16:
        raise Held("dns-invalid")
    return {
        "rcode": 0,
        "addresses": [public_address(ip, 4 if qtype == 1 else 6) for ip in result],
    }


class Ledger:
    def __init__(self, directory, *, clock=time.monotonic, started=None):
        self.directory, self.clock = Path(directory), clock
        self.started = clock() if started is None else started
        self.deadline = self.started + SECONDS
        self.counts = dict.fromkeys(LIMITS, 0)
        self.finished = False
        self.result = None
        self.fd = os.open(
            self.directory / "attempts.jsonl",
            os.O_WRONLY | os.O_CREAT | os.O_EXCL,
            0o600,
        )
        self.record(
            event="created",
            prior=PRIOR,
            maximum=MAXIMUM,
            limits=LIMITS,
            started=self.started,
            deadline=self.deadline,
            seconds=SECONDS,
        )
        sync_directory(self.directory)

    def record(self, **value):
        raw = (
            json.dumps(
                {"at": time.time(), "monotonic": self.clock(), **value},
                separators=(",", ":"),
            )
            + "\n"
        ).encode()
        while raw:
            n = os.write(self.fd, raw)
            if n <= 0:
                raise OSError("ledger-write")
            raw = raw[n:]
        os.fsync(self.fd)

    def remaining(self, maximum):
        if self.finished or self.clock() >= self.deadline:
            raise Held("deadline")
        return min(maximum, self.deadline - self.clock())

    def admit(self, kind, family=None):
        self.remaining(SECONDS)
        if kind not in LIMITS or self.counts[kind] >= LIMITS[kind]:
            raise Held("limit")
        if kind == "tcp" and PRIOR + self.counts["tcp"] >= MAXIMUM:
            raise Held("limit")
        self.counts[kind] += 1
        self.record(
            event="attempt",
            kind=kind,
            ordinal=self.counts[kind],
            family=family,
            cumulative=PRIOR + self.counts["tcp"],
        )

    def finish(self, reason):
        if self.result is not None:
            return self.result
        if self.finished:
            raise OSError("terminal-receipt-incomplete")
        self.finished = True
        self.record(
            event="stop",
            reason=reason,
            counters=self.counts,
            cumulative=PRIOR + self.counts["tcp"],
        )
        result = {
            "reason": reason,
            "counters": dict(self.counts),
            "prior": PRIOR,
            "cumulative": PRIOR + self.counts["tcp"],
            "maximum": MAXIMUM,
            "secondsAllowed": SECONDS,
            "elapsedSeconds": self.clock() - self.started,
            "httpRequests": 0,
            "credentialsUsed": False,
            "automaticContinuation": False,
            "proceduralAllowanceUsd": 1,
            "billingMeasured": False,
        }
        persist(self.directory / "result.json", result)
        self.result = result
        return result

    def close(self):
        os.close(self.fd)


def diagnostic(error):
    # Diagnostic categories never contain arbitrary exception text or URLs.
    if isinstance(error, Expired):
        return {
            "exceptionCategory": "deadline",
            "failurePhase": "unknown",
            "errorCode": None,
            "errorCodeDomain": None,
        }
    return transport_diagnostic(error)


def execute(ledger, backend):
    reason = "executor-failed"
    try:
        addresses = []
        for qtype, version in [(1, 4), (28, 6)]:
            ledger.admit("dns", version)
            try:
                answer = backend.dns(qtype, ledger.remaining(2))
            except (OSError, Held) as error:
                ledger.record(
                    event="outcome", kind="dns", success=False, **diagnostic(error)
                )
                raise error if isinstance(error, Held) else Held("dns-failed") from None
            ledger.remaining(SECONDS)
            if answer["rcode"]:
                ledger.record(
                    event="outcome", kind="dns", success=False, rcode=answer["rcode"]
                )
                raise Held("dns-rcode")
            values = [public_address(x, version) for x in answer["addresses"]]
            if len(values) > 16:
                raise Held("dns-invalid")
            ledger.record(
                event="outcome",
                kind="dns",
                success=True,
                family=version,
                rcode=0,
                addressCount=len(values),
            )
            if values:
                addresses.append(values[0])
        persist(ledger.directory / "addresses.json", {"selected": addresses})
        if not addresses:
            raise Held("dns-empty")
        reason = "tcp-failed"
        for address in addresses:
            version = ipaddress.ip_address(address).version
            backend.route(address, ledger.remaining(2))
            ledger.admit("tcp", version)
            try:
                stream = backend.connect(address, ledger.remaining(10))
            except OSError as error:
                ledger.record(
                    event="outcome",
                    kind="tcp",
                    success=False,
                    family=version,
                    **diagnostic(error),
                )
                continue
            try:
                ledger.record(event="outcome", kind="tcp", success=True, family=version)
                ledger.admit("tls", version)
                try:
                    protocol = backend.tls(stream, ledger.remaining(5))
                except (OSError, Held) as error:
                    ledger.record(
                        event="outcome", kind="tls", success=False, **diagnostic(error)
                    )
                    reason = error.reason if isinstance(error, Held) else "tls-failed"
                else:
                    ledger.remaining(SECONDS)
                    ledger.record(
                        event="outcome",
                        kind="tls",
                        success=True,
                        verified=True,
                        protocol=protocol
                        if protocol in {"TLSv1.2", "TLSv1.3"}
                        else "other",
                    )
                    reason = "tls-verified"
            finally:
                stream.close()
            break  # Never try another family after a TCP success / TLS attempt.
    except Held as error:
        reason = error.reason
    except Expired:
        reason = "deadline"
    except OSError:
        reason = "persistence-failed"
    except Exception:  # noqa: BLE001 - no raw diagnostics escape
        reason = "executor-failed"
    return ledger.finish(reason)


class Backend:
    def __init__(self, directory, ledger, ca_file):
        self.directory, self.ledger = directory, ledger
        self.context = ssl.create_default_context(cafile=ca_file)
        if (
            self.context.verify_mode != ssl.CERT_REQUIRED
            or not self.context.check_hostname
        ):
            raise Held("scope-held")

    def dns(self, qtype, timeout):
        transaction = int.from_bytes(os.urandom(2), "big")
        query = (
            struct.pack("!6H", transaction, 0x0100, 1, 0, 0, 0)
            + wire_name(HOST)
            + struct.pack("!HH", qtype, 1)
        )
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
            sock.settimeout(timeout)
            if sock.sendto(query, (RESOLVER, 53)) != len(query):
                raise Held("dns-failed")
            data, peer = sock.recvfrom(4097)
        if peer != (RESOLVER, 53):
            raise Held("scope-held")
        return parse_dns(data, transaction, qtype)

    def route(self, address, timeout):
        version = ipaddress.ip_address(address).version
        result = subprocess.run(
            ["ip", "-j", f"-{version}", "route", "get", address],
            capture_output=True,
            timeout=timeout,
            text=True,
            check=False,
        )
        persist(
            self.directory / f"route-{version}.json",
            {
                "exitCode": result.returncode,
                "routes": json.loads(result.stdout) if result.returncode == 0 else [],
            },
        )
        # A missing route is observed by the one permitted connect, not repaired.

    def connect(self, address, timeout):
        ip = ipaddress.ip_address(address)
        sock = socket.socket(
            socket.AF_INET if ip.version == 4 else socket.AF_INET6, socket.SOCK_STREAM
        )
        try:
            sock.settimeout(timeout)
            sock.connect((address, 443) if ip.version == 4 else (address, 443, 0, 0))
            return sock
        except BaseException:
            sock.close()
            raise

    def tls(self, stream, timeout):
        stream.settimeout(timeout)
        with self.context.wrap_socket(
            stream, server_hostname=HOST, do_handshake_on_connect=False
        ) as secure:
            secure.do_handshake()
            return secure.version()


@contextlib.contextmanager
def alarm(deadline):
    if signal.getitimer(signal.ITIMER_REAL) != (0.0, 0.0):
        raise Held("scope-held")

    def expired(*_):
        raise Expired()

    previous = signal.signal(signal.SIGALRM, expired)
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        signal.signal(signal.SIGALRM, previous)
        raise Held("deadline")
    signal.setitimer(signal.ITIMER_REAL, remaining)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous)


def verified_gate(path):
    gate = json.loads(Path(path).read_text())
    if (
        gate.get("offlinePassed") is not True
        or gate.get("acceptedScope") != "cb4bfc922c149e5c8a814e583851f89d95608908"
    ):
        raise Held("scope-held")
    expected = {
        "network_probe.py",
        "test_network_probe.py",
        "accounting.py",
        "policy.py",
        "auth_diagnostics.py",
    }
    if set(gate.get("sourceSha256", {})) != expected:
        raise Held("scope-held")
    if any(
        sha(Path(__file__).with_name(name)) != value
        for name, value in gate["sourceSha256"].items()
    ):
        raise Held("scope-held")


def preflight(ledger, commit, gate):
    from authentication import sdk_pins
    from botocore.httpsession import get_cert_path
    from run import git, valid_parent

    if (
        PRIOR != 167
        or MAXIMUM != 500
        or git("rev-parse", "HEAD") != commit
        or git("status", "--porcelain", "--untracked-files=all")
    ):
        raise Held("scope-held")
    verified_gate(gate)
    valid_parent()
    sdk_pins()
    source = Path("/tmp/kurier-environments-release-source")
    if git("rev-parse", "HEAD", cwd=source) != SOURCE or git(
        "status", "--porcelain", "--untracked-files=all", cwd=source
    ):
        raise Held("scope-held")
    if os.readlink("/proc/self/ns/net") != HOST_NAMESPACE or os.getuid() != 1000:
        raise Held("scope-held")
    resolv = Path("/etc/resolv.conf")
    servers = [
        x.split()[1]
        for x in resolv.read_text().splitlines()
        if x.startswith("nameserver ")
    ]
    if servers != [RESOLVER]:
        raise Held("scope-held")
    if any(
        k in os.environ
        for k in [
            "HTTP_PROXY",
            "HTTPS_PROXY",
            "ALL_PROXY",
            "http_proxy",
            "https_proxy",
            "all_proxy",
            "NO_PROXY",
            "no_proxy",
        ]
    ):
        raise Held("scope-held")
    ca_file = get_cert_path(True)
    context = {
        "netNamespace": os.readlink("/proc/self/ns/net"),
        "uid": os.getuid(),
        "resolver": RESOLVER,
        "resolverSha256": sha(resolv),
        "caSha256": sha(ca_file),
        "implementationCommit": commit,
        "source": SOURCE,
        "baseline": baseline(ROOT),
    }
    for name, args in [
        ("routes", ["ip", "-j", "route", "show", "table", "all"]),
        ("rules", ["ip", "-j", "rule", "show"]),
    ]:
        result = subprocess.run(
            args,
            capture_output=True,
            text=True,
            timeout=ledger.remaining(2),
            check=False,
        )
        if result.returncode:
            raise Held("scope-held")
        context[name] = json.loads(result.stdout)
    persist(ledger.directory / "context.json", context)
    persist(
        ROOT / ".build/project-environments-preview/window-from-167.json",
        {
            "purpose": "unauthenticated-network-diagnostic",
            "directory": str(ledger.directory),
            "started": ledger.started,
            "deadline": ledger.deadline,
            **baseline(ROOT),
        },
    )
    return Backend(ledger.directory, ledger, ca_file)


def main():
    started = time.monotonic()
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--implementation-commit", required=True)
    p.add_argument("--gate", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    args = p.parse_args()
    directory = args.output.resolve()
    if (
        directory.parent
        != (ROOT / ".build/project-environments-network-diagnostics").resolve()
    ):
        raise Held("scope-held")
    directory.parent.mkdir(mode=0o700, exist_ok=True)
    directory.mkdir(mode=0o700, exist_ok=False)
    sync_directory(directory.parent)
    ledger = Ledger(directory, started=started)
    try:
        try:
            with alarm(ledger.deadline):
                backend = preflight(ledger, args.implementation_commit, args.gate)
                result = execute(ledger, backend)
        except Held as error:
            result = ledger.finish(error.reason)
        except Expired:
            result = ledger.finish("deadline")
        except Exception:  # noqa: BLE001 - fail closed without raw SDK/config errors
            result = ledger.finish("executor-failed")
        print(json.dumps(result))
        return 0 if result["reason"] == "tls-verified" else 1
    finally:
        ledger.close()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, Held):
        print("Diagnostic executor stopped locally; no automatic restart.")
        raise SystemExit(1) from None
