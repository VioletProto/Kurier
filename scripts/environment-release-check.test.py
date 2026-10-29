"""Offline tests use synthetic ciphertext/keys only."""

import hashlib
import importlib.util
import os
import subprocess
import tempfile
import unittest
import zipfile
from pathlib import Path

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

spec = importlib.util.spec_from_file_location(
    "release_check", Path(__file__).with_name("environment-release-check.py")
)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


class Tests(unittest.TestCase):
    def fixture(self, files=None, key=bytes(32), plain=b"{}"):
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        path = Path(d.name) / "code.zip"
        with zipfile.ZipFile(path, "w") as z:
            for name, data in files or [
                ("./", b""),
                ("bootstrap", b"offline-bootstrap"),
                ("resource.enc", AESGCM(key).encrypt(bytes(12), plain, None)),
            ]:
                z.writestr(name, data)
        p = {
            "name": "fixture",
            "zipSha256": hashlib.sha256(path.read_bytes()).hexdigest(),
            "zipBytes": path.stat().st_size,
            "bootstrapSha256": hashlib.sha256(b"offline-bootstrap").hexdigest(),
            "bootstrapBytes": len(b"offline-bootstrap"),
        }
        return path, p

    def test_authenticated_empty_map_has_no_credentials_or_unrelated_bindings(self):
        path, p = self.fixture()
        r = m.inspect_archive(path, p, bytes(32))
        self.assertTrue(r["bindingAuthenticated"])
        self.assertTrue(r["credentialOrUnrelatedBindingAbsenceConfirmed"])
        self.assertNotIn("plaintext", r)

    def test_missing_key_never_claims_content_validation(self):
        path, p = self.fixture()
        r = m.inspect_archive(path, p)
        self.assertFalse(r["bindingAuthenticated"])
        self.assertFalse(r["credentialOrUnrelatedBindingAbsenceConfirmed"])

    def test_wrong_key_or_tampering_fails_without_plaintext(self):
        path, p = self.fixture()
        with self.assertRaisesRegex(m.Held, "binding-authentication-held"):
            m.inspect_archive(path, p, bytes([1]) * 32)
        self.assertEqual(len(path.read_bytes()), p["zipBytes"])

    def test_even_authenticated_nonempty_bindings_or_json_alias_are_rejected(self):
        for plain in [b"[]", b"42", b'""', b'{"secret":"private-canary"}']:
            path, p = self.fixture(plain=plain)
            with self.assertRaises(m.Held):
                m.inspect_archive(path, p, bytes(32))

    def test_unexpected_files_paths_and_duplicates_are_rejected(self):
        base = [
            ("./", b""),
            ("bootstrap", b"offline-bootstrap"),
            ("resource.enc", b"x" * 18),
        ]
        for extra in [
            ".aws/credentials",
            "../private",
            "/private",
            "bootstrap",
            "bootstrap/",
            "link",
        ]:
            path, p = self.fixture(files=base + [(extra, b"private-canary")])
            with self.assertRaisesRegex(m.Held, "archive-members-held"):
                m.inspect_archive(path, p)

    def test_archive_and_bootstrap_bytes_are_pinned(self):
        path, p = self.fixture()
        p["zipSha256"] = "0" * 64
        with self.assertRaisesRegex(m.Held, "archive-hash-held"):
            m.inspect_archive(path, p)
        path, p = self.fixture()
        p["bootstrapSha256"] = "0" * 64
        with self.assertRaisesRegex(m.Held, "bootstrap-held"):
            m.inspect_archive(path, p)

    def test_short_and_oversized_ciphertext_are_rejected(self):
        for size in [0, 15, 17, 19, 64]:
            path, p = self.fixture(
                files=[
                    ("./", b""),
                    ("bootstrap", b"offline-bootstrap"),
                    ("resource.enc", bytes(size)),
                ]
            )
            with self.assertRaisesRegex(m.Held, "binding-length-held"):
                m.inspect_archive(path, p)

    def test_symlink_and_bad_root_entry_are_rejected(self):
        path, p = self.fixture()
        with zipfile.ZipFile(path) as z:
            data = [(x.filename, z.read(x)) for x in z.infolist()]
        with zipfile.ZipFile(path, "w") as z:
            for name, value in data:
                info = zipfile.ZipInfo(name)
                info.create_system = 3
                info.external_attr = (
                    0o120777 if name == "bootstrap" else 0o100600
                ) << 16
                z.writestr(info, value)
        p["zipSha256"] = hashlib.sha256(path.read_bytes()).hexdigest()
        p["zipBytes"] = path.stat().st_size
        with self.assertRaises(m.Held):
            m.inspect_archive(path, p)

    def test_cli_diagnostics_do_not_disclose_private_key_content(self):
        with tempfile.TemporaryDirectory() as directory:
            key = Path(directory) / "key"
            key.write_text("private-credential-canary-not-base64")
            os.chmod(key, 0o600)
            r = subprocess.run(
                [
                    str(m.ROOT / ".build/attempt-meter-venv/bin/python"),
                    "-B",
                    str(Path(__file__).with_name("environment-release-check.py")),
                    "--private-key-file",
                    str(key),
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(r.returncode, 0)
            self.assertNotIn("private-credential-canary", r.stderr + r.stdout)
            self.assertNotIn(str(key), r.stderr + r.stdout)

    def test_regenerated_mismatched_archive_stops_offline_preflight(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "UsersProjectsApi"
            target.mkdir()
            (target / "code.zip").write_bytes(b"unreviewed")
            r = subprocess.run(
                [
                    str(m.ROOT / ".build/attempt-meter-venv/bin/python"),
                    "-B",
                    str(Path(__file__).with_name("environment-release-check.py")),
                    "--regenerated-directory",
                    directory,
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(r.returncode, 0)
            self.assertIn("validation held", r.stderr)
            self.assertEqual(r.stdout, "")


if __name__ == "__main__":
    unittest.main()
