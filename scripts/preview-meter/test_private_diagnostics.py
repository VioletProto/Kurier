"""Offline private CONNECT receipt limits and persistence; no network."""

import importlib.util
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

import private_diagnostics as private

CANARY = "local-private-host-canary"


class Tests(unittest.TestCase):
    def test_exact_parsed_hostname_is_preserved_privately_and_bounded(self):
        hosts = [
            CANARY + ".example.invalid",
            "MiXeD.example.invalid.",
            "2001:db8::1",
            ".".join(["a" * 63] * 3 + ["b" * 61]),
        ]
        for host in hosts:
            with tempfile.TemporaryDirectory() as tmp:
                private.capture(tmp, host, 443, 200)
                path = Path(tmp) / private.FILENAME
                raw = path.read_bytes()
                value = json.loads(raw)
                self.assertEqual(value["hostname"], host)
                self.assertEqual(value["hostnameStatus"], "captured")
                self.assertEqual(
                    set(value),
                    {"version", "phase", "count", "port", "hostnameStatus", "hostname"},
                )
                self.assertLessEqual(len(raw), private.MAX_RECORD)
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_malformed_or_overlong_inputs_do_not_persist_url_material(self):
        for host in [
            None,
            {},
            123,
            "",
            CANARY + "@other.invalid",
            "https://" + CANARY,
            CANARY + ".invalid/path",
            CANARY + ".invalid?token=secret",
            CANARY + ".invalid#fragment",
            CANARY + "\r\nHeader: body",
            "a" * 254,
            "é.invalid",
            "[2001:db8::1]",
            "fe80::1%secret",
        ]:
            with tempfile.TemporaryDirectory() as tmp:
                private.capture(tmp, host, 443, 200)
                raw = (Path(tmp) / private.FILENAME).read_text()
                self.assertNotIn(CANARY, raw)
                self.assertNotIn("secret", raw)
                self.assertIsNone(json.loads(raw)["hostname"])
                self.assertLessEqual(len(raw), private.MAX_RECORD)

    def test_existing_receipt_cannot_be_replaced_or_appended(self):
        with tempfile.TemporaryDirectory() as tmp:
            private.capture(tmp, CANARY + ".invalid", 443, 200)
            path = Path(tmp) / private.FILENAME
            original = path.read_bytes()
            with self.assertRaises(FileExistsError):
                private.capture(tmp, "second.invalid", 443, 200)
            self.assertEqual(path.read_bytes(), original)

    def test_insecure_directory_and_symlinks_are_held(self):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            directory = base / "private"
            directory.mkdir(mode=0o755)
            directory.chmod(0o755)
            with self.assertRaises(OSError):
                private.capture(directory, "fixture.invalid", 443, 200)
            directory.chmod(0o700)
            alias = base / "alias"
            alias.symlink_to(directory, target_is_directory=True)
            with self.assertRaises(OSError):
                private.capture(alias, "fixture.invalid", 443, 200)
            target = base / "untouched"
            target.write_text("original")
            (directory / private.FILENAME).symlink_to(target)
            with self.assertRaises(OSError):
                private.capture(directory, "fixture.invalid", 443, 200)
            self.assertEqual(target.read_text(), "original")

    def test_short_writes_finish_and_both_file_and_directory_are_fsynced(self):
        with tempfile.TemporaryDirectory() as tmp:
            original = os.write
            with (
                patch.object(
                    os, "write", side_effect=lambda fd, raw: original(fd, raw[:3])
                ),
                patch.object(os, "fsync", wraps=os.fsync) as sync,
            ):
                private.capture(tmp, "fixture.invalid", 443, 200)
                self.assertEqual(sync.call_count, 2)
            self.assertEqual(
                json.loads((Path(tmp) / private.FILENAME).read_text())["hostname"],
                "fixture.invalid",
            )

    def test_zero_write_or_sync_failure_is_terminal(self):
        for function in ["write", "fsync"]:
            with tempfile.TemporaryDirectory() as tmp:
                options = (
                    {"return_value": 0}
                    if function == "write"
                    else {"side_effect": OSError("fixture-only")}
                )
                with patch.object(os, function, **options), self.assertRaises(OSError):
                    private.capture(tmp, "fixture.invalid", 443, 200)
                self.assertTrue((Path(tmp) / private.FILENAME).exists())

    def test_only_bounded_numeric_metadata_is_accepted(self):
        for port, count in [
            (True, 200),
            (0, 200),
            (65536, 200),
            (443, True),
            (443, -1),
            (443, 501),
        ]:
            with tempfile.TemporaryDirectory() as tmp:
                with self.assertRaises(OSError):
                    private.capture(tmp, "fixture.invalid", port, count)
                self.assertFalse((Path(tmp) / private.FILENAME).exists())

    def test_unexpected_capture_exception_kills_proxy_without_raw_public_host(self):
        class Exited(BaseException):
            pass

        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            config = directory / "config.json"
            config.write_text(
                json.dumps(
                    {
                        "mode": "fixture",
                        "fixtureOrigin": 1,
                        "ledger": str(directory / "attempts.jsonl"),
                        "budget": {"prior": 200, "maximum": 500, "seconds": 3600},
                        "privateDeniedConnectCapture": True,
                    }
                )
            )
            http = SimpleNamespace(Response=SimpleNamespace(make=Mock()))
            spec = importlib.util.spec_from_file_location(
                "private_proxy_fixture", Path(__file__).with_name("proxy.py")
            )
            module = importlib.util.module_from_spec(spec)
            with (
                patch.dict(sys.modules, {"mitmproxy": SimpleNamespace(http=http)}),
                patch.dict(os.environ, {"KURIER_PREVIEW_METER_CONFIG": str(config)}),
            ):
                spec.loader.exec_module(module)
            gate = module.addons[0]
            self.addCleanup(gate.done)
            flow = SimpleNamespace(
                request=SimpleNamespace(
                    host=CANARY + ".example.invalid",
                    port=443,
                    scheme="",
                    http_version="HTTP/1.1",
                    method="CONNECT",
                ),
                kill=Mock(),
            )
            with (
                patch.object(module, "capture", side_effect=RuntimeError(CANARY)),
                patch.object(os, "_exit", side_effect=Exited) as exit_proxy,
                self.assertRaises(Exited),
            ):
                gate.http_connect(flow)
            flow.kill.assert_called_once()
            exit_proxy.assert_called_once_with(74)
            http.Response.make.assert_not_called()
            raw = (directory / "attempts.jsonl").read_text()
            self.assertNotIn(CANARY, raw)
            self.assertFalse(
                any(json.loads(x)["event"] == "attempt" for x in raw.splitlines())
            )


if __name__ == "__main__":
    unittest.main()
