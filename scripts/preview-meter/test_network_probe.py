"""Offline diagnostic gate: fake DNS, TCP and TLS; never access a network."""

import hashlib
import ipaddress
import json
import socket
import ssl
import struct
import tempfile
import time
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import network_probe as n
from handoff import persist

V4 = "8.8.8.8"
V6 = "2001:4860:4860::8888"
CANARY = "private-canary-message-url-header-token"


def response(qtype=1, transaction=123, addresses=None, flags=0x8180):
    addresses = [V4 if qtype == 1 else V6] if addresses is None else addresses
    question = n.wire_name(n.HOST) + struct.pack("!HH", qtype, 1)
    rr = b""
    for address in addresses:
        raw = ipaddress.ip_address(address).packed
        rr += b"\xc0\x0c" + struct.pack("!HHIH", qtype, 1, 60, len(raw)) + raw
    return (
        struct.pack("!6H", transaction, flags, 1, len(addresses), 0, 0) + question + rr
    )


class FakeStream:
    closed = False

    def close(self):
        self.closed = True


class Fake:
    def __init__(
        self, ledger, *, v4=None, v6=None, tcp_fail=(), tls_error=False, dns_error=False
    ):
        self.ledger = ledger
        self.answers = {1: [V4] if v4 is None else v4, 28: [V6] if v6 is None else v6}
        self.tcp_fail, self.tls_error, self.dns_error = tcp_fail, tls_error, dns_error
        self.calls, self.streams = [], []

    def before(self, kind):
        rows = [
            json.loads(x)
            for x in (self.ledger.directory / "attempts.jsonl").read_text().splitlines()
        ]
        assert rows[-1]["event"] == "attempt" and rows[-1]["kind"] == kind
        self.calls.append(kind)

    def dns(self, qtype, timeout):
        self.before("dns")
        assert 0 < timeout <= 2
        if self.dns_error:
            raise TimeoutError(CANARY)
        return {"rcode": 0, "addresses": self.answers[qtype]}

    def route(self, address, timeout):
        assert address in [V4, V6] and 0 < timeout <= 2

    def connect(self, address, timeout):
        self.before("tcp")
        assert 0 < timeout <= 10
        if address in self.tcp_fail:
            raise TimeoutError(CANARY)
        stream = FakeStream()
        self.streams.append(stream)
        return stream

    def tls(self, stream, timeout):
        self.before("tls")
        assert 0 < timeout <= 5
        if self.tls_error:
            raise ssl.SSLError(CANARY)
        return "TLSv1.3"


class Tests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.now = [100.0]
        self.ledger = n.Ledger(self.directory, clock=lambda: self.now[0])
        self.addCleanup(self.ledger.close)

    def run_case(self, **kwargs):
        backend = Fake(self.ledger, **kwargs)
        with (
            patch("socket.socket", side_effect=AssertionError("offline-only")),
            patch("socket.getaddrinfo", side_effect=AssertionError("no-dns-fallback")),
        ):
            result = n.execute(self.ledger, backend)
        self.assertFalse(result["credentialsUsed"])
        self.assertEqual(result["httpRequests"], 0)
        self.assertFalse(result["automaticContinuation"])
        self.assertEqual(result["cumulative"], 167 + result["counters"]["tcp"])
        self.assertNotIn(CANARY, (self.directory / "attempts.jsonl").read_text())
        self.assertNotIn(V4, (self.directory / "attempts.jsonl").read_text())
        self.assertTrue(all(s.closed for s in backend.streams))
        return result, backend

    def test_ipv4_success_stops_after_one_tls_and_never_restarts(self):
        result, backend = self.run_case()
        self.assertEqual(result["reason"], "tls-verified")
        self.assertEqual(result["counters"], {"dns": 2, "tcp": 1, "tls": 1})
        self.assertEqual(backend.calls, ["dns", "dns", "tcp", "tls"])
        self.assertEqual(n.execute(self.ledger, backend), result)
        self.assertEqual(len(backend.calls), 4)

    def test_ipv4_failure_allows_only_one_ipv6_and_one_tls(self):
        result, backend = self.run_case(tcp_fail=[V4])
        self.assertEqual(result["reason"], "tls-verified")
        self.assertEqual(result["counters"], {"dns": 2, "tcp": 2, "tls": 1})
        self.assertEqual(result["cumulative"], 169)
        self.assertEqual(backend.calls, ["dns", "dns", "tcp", "tcp", "tls"])

    def test_both_tcp_failures_stop_without_tls_or_retry(self):
        result, backend = self.run_case(tcp_fail=[V4, V6])
        self.assertEqual(result["reason"], "tcp-failed")
        self.assertEqual(result["counters"], {"dns": 2, "tcp": 2, "tls": 0})
        self.assertEqual(len(backend.calls), 4)

    def test_ipv6_only_and_empty_dns_are_bounded(self):
        result, _ = self.run_case(v4=[])
        self.assertEqual(result["counters"], {"dns": 2, "tcp": 1, "tls": 1})

    def test_empty_answers_do_not_connect(self):
        result, _ = self.run_case(v4=[], v6=[])
        self.assertEqual(result["reason"], "dns-empty")
        self.assertEqual(result["cumulative"], 167)
        self.assertEqual(result["counters"], {"dns": 2, "tcp": 0, "tls": 0})

    def test_tls_failure_never_tries_another_family(self):
        result, backend = self.run_case(tls_error=True)
        self.assertEqual(result["reason"], "tls-failed")
        self.assertEqual(result["counters"], {"dns": 2, "tcp": 1, "tls": 1})
        self.assertEqual(len(backend.calls), 4)

    def test_dns_failure_does_not_retry_or_connect(self):
        result, backend = self.run_case(dns_error=True)
        self.assertEqual(result["reason"], "dns-failed")
        self.assertEqual(result["counters"], {"dns": 1, "tcp": 0, "tls": 0})
        self.assertEqual(backend.calls, ["dns"])

    def test_private_or_wrong_family_dns_answer_stops(self):
        result, backend = self.run_case(v4=["127.0.0.1"])
        self.assertEqual(result["reason"], "scope-held")
        self.assertEqual(backend.calls, ["dns"])
        self.assertEqual(result["cumulative"], 167)

    def test_absolute_deadline_cannot_reset_between_phases(self):
        backend = Fake(self.ledger)
        original = backend.dns

        def slow(*args):
            value = original(*args)
            self.now[0] = 145.0
            return value

        with patch.object(backend, "dns", slow):
            result = n.execute(self.ledger, backend)
        self.assertEqual(result["reason"], "deadline")
        self.assertEqual(backend.calls, ["dns"])
        self.assertEqual(result["cumulative"], 167)

    def test_direct_counter_and_cumulative_limits(self):
        for kind, limit in n.LIMITS.items():
            for _ in range(limit):
                self.ledger.admit(kind)
            with self.assertRaises(n.Held):
                self.ledger.admit(kind)
        self.assertEqual(self.ledger.counts, {"dns": 2, "tcp": 2, "tls": 1})

    def test_negative_dns_stops_before_second_query(self):
        backend = Fake(self.ledger)

        def negative(qtype, timeout):
            backend.before("dns")
            return {"rcode": 3, "addresses": []}

        with patch.object(backend, "dns", negative):
            result = n.execute(self.ledger, backend)
        self.assertEqual(result["reason"], "dns-rcode")
        self.assertEqual(backend.calls, ["dns"])
        self.assertEqual(result["counters"], {"dns": 1, "tcp": 0, "tls": 0})

    def test_deadline_after_connect_prevents_handshake_and_closes_socket(self):
        backend = Fake(self.ledger)
        original = backend.connect

        def late(*args):
            stream = original(*args)
            self.now[0] = 145.0
            return stream

        with patch.object(backend, "connect", late):
            result = n.execute(self.ledger, backend)
        self.assertEqual(result["reason"], "deadline")
        self.assertEqual(backend.calls, ["dns", "dns", "tcp"])
        self.assertEqual(result["cumulative"], 168)
        self.assertTrue(backend.streams[0].closed)

    def test_global_ceiling_stops_second_connection(self):
        backend = Fake(self.ledger, tcp_fail=[V4])
        with patch.object(n, "MAXIMUM", 168):
            result = n.execute(self.ledger, backend)
        self.assertEqual(result["reason"], "limit")
        self.assertEqual(result["cumulative"], 168)
        self.assertEqual(backend.calls, ["dns", "dns", "tcp"])

    def test_persistence_failure_prevents_forwarding(self):
        backend = Fake(self.ledger)
        original = self.ledger.record

        def failed(**value):
            if value["event"] == "attempt":
                raise OSError("disk-full")
            original(**value)

        with patch.object(self.ledger, "record", failed):
            result = n.execute(self.ledger, backend)
        self.assertEqual(result["reason"], "persistence-failed")
        self.assertEqual(backend.calls, [])
        self.assertEqual(result["cumulative"], 167)


class WireTests(unittest.TestCase):
    def test_alarm_interrupts_blocking_work(self):
        with self.assertRaises(n.Expired), n.alarm(time.monotonic() + 0.01):
            time.sleep(0.1)

    def test_dns_a_aaaa_compression_and_in_packet_alias(self):
        self.assertEqual(
            n.parse_dns(response(), 123, 1), {"rcode": 0, "addresses": [V4]}
        )
        self.assertEqual(n.parse_dns(response(28), 123, 28)["addresses"], [V6])
        alias = "alias.example.com"
        question = n.wire_name(n.HOST) + struct.pack("!HH", 1, 1)
        name = n.wire_name(alias)
        cname = b"\xc0\x0c" + struct.pack("!HHIH", 5, 1, 60, len(name)) + name
        address = (
            name + struct.pack("!HHIH", 1, 1, 60, 4) + ipaddress.ip_address(V4).packed
        )
        data = struct.pack("!6H", 123, 0x8180, 1, 2, 0, 0) + question + cname + address
        self.assertEqual(n.parse_dns(data, 123, 1)["addresses"], [V4])

    def test_dns_malformed_truncated_wrong_question_and_private_are_rejected(self):
        values = [
            b"",
            response(transaction=124),
            response(flags=0x8380),
            response() + b"x",
            response().replace(
                n.wire_name(n.HOST), n.wire_name("otherx.us-east-2.amazonaws.com")
            ),
            response(addresses=["10.0.0.1"]),
        ]
        for data in values:
            with self.subTest(size=len(data)), self.assertRaises(n.Held):
                n.parse_dns(data, 123, 1)
        with self.assertRaises(n.Held):
            n.read_name(b"\xc0\x00", 0)

    def test_negative_dns_code_is_returned_without_followup(self):
        self.assertEqual(
            n.parse_dns(response(addresses=[], flags=0x8183), 123, 1),
            {"rcode": 3, "addresses": []},
        )

    def test_public_filter_does_not_accept_private_mapped_or_multicast(self):
        for address, version in [
            ("::1", 6),
            ("::ffff:8.8.8.8", 6),
            ("224.0.0.1", 4),
            (V6, 4),
            ("169.254.169.254", 4),
        ]:
            with self.subTest(version=version), self.assertRaises(n.Held):
                n.public_address(address, version)

    def test_gate_and_exclusive_claim_cannot_be_reset(self):
        names = {
            "network_probe.py",
            "test_network_probe.py",
            "accounting.py",
            "policy.py",
            "auth_diagnostics.py",
        }
        with tempfile.TemporaryDirectory() as tmp:
            gate = Path(tmp) / "gate.json"
            value = {
                "offlinePassed": True,
                "acceptedScope": "cb4bfc922c149e5c8a814e583851f89d95608908",
                "sourceSha256": {
                    name: n.sha(Path(n.__file__).with_name(name)) for name in names
                },
            }
            gate.write_text(json.dumps(value))
            n.verified_gate(gate)
            value["sourceSha256"]["network_probe.py"] = "0" * 64
            gate.write_text(json.dumps(value))
            with self.assertRaises(n.Held):
                n.verified_gate(gate)
            claim = Path(tmp) / "claim.json"
            persist(claim, {"prior": 167})
            original = hashlib.sha256(claim.read_bytes()).hexdigest()
            with self.assertRaises(FileExistsError):
                persist(claim, {"prior": 0})
            self.assertEqual(hashlib.sha256(claim.read_bytes()).hexdigest(), original)

    def test_actual_backend_framing_has_no_http_or_dns_fallback(self):
        calls = []

        class Raw:
            def __init__(self, family, kind):
                self.kind = kind

            def settimeout(self, value):
                assert 0 < value <= 10

            def sendto(self, data, peer):
                assert self.kind == socket.SOCK_DGRAM and peer == (n.RESOLVER, 53)
                self.query = data
                calls.append("dns")
                return len(data)

            def recvfrom(self, size):
                ident = struct.unpack("!H", self.query[:2])[0]
                kind = struct.unpack("!HH", self.query[-4:])[0]
                return response(kind, ident), (n.RESOLVER, 53)

            def connect(self, peer):
                assert self.kind == socket.SOCK_STREAM and peer == (V4, 443)
                calls.append("tcp")

            def close(self):
                pass

            def __enter__(self):
                return self

            def __exit__(self, *_):
                self.close()

            def send(self, *_):
                raise AssertionError("no-http-send")

            def sendall(self, *_):
                raise AssertionError("no-http-sendall")

        class Secure(Raw):
            def __init__(self):
                pass

            def do_handshake(self):
                calls.append("tls")

            def version(self):
                return "TLSv1.3"

            def write(self, *_):
                raise AssertionError("no-http-write")

        class Context:
            verify_mode = ssl.CERT_REQUIRED
            check_hostname = True

            def wrap_socket(self, stream, *, server_hostname, do_handshake_on_connect):
                assert server_hostname == n.HOST and do_handshake_on_connect is False
                return Secure()

        with tempfile.TemporaryDirectory() as tmp:
            d = Path(tmp)
            ledger = n.Ledger(d)
            try:
                with (
                    patch.object(
                        n.ssl, "create_default_context", return_value=Context()
                    ),
                    patch.object(n.socket, "socket", Raw),
                    patch.object(
                        n.socket,
                        "getaddrinfo",
                        side_effect=AssertionError("no-fallback"),
                    ),
                    patch.object(
                        n.subprocess,
                        "run",
                        return_value=SimpleNamespace(returncode=0, stdout="[]"),
                    ),
                ):
                    backend = n.Backend(d, ledger, "local-ca")
                    result = n.execute(ledger, backend)
                self.assertEqual(result["reason"], "tls-verified")
                self.assertEqual(calls, ["dns", "dns", "tcp", "tls"])
            finally:
                ledger.close()

    def test_tls_verification_cannot_be_disabled(self):
        with (
            patch.object(
                n.ssl,
                "create_default_context",
                return_value=SimpleNamespace(
                    verify_mode=ssl.CERT_NONE, check_hostname=False
                ),
            ),
            self.assertRaises(n.Held),
        ):
            n.Backend(Path("unused"), None, "local-ca")


if __name__ == "__main__":
    unittest.main()
