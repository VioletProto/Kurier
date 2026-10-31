"""Offline archive verification. Never emits keys, plaintext bindings or exceptions."""

import argparse
import base64
import hashlib
import json
import stat
import subprocess
import zipfile
from pathlib import Path

SOURCE = "44f1fb79d943501c95e06fcc1a6114111a7c1ea4"
ORIGINAL_SHA = "94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5"
ROOT = Path(__file__).resolve().parents[1]
MANIFEST_SHA = "25ef3cd0c8f0c09037607c564bfb8964c1ef8e8e14a3aa0ae8757894918895a9"
NAMES = {
    "UsersProjectsApi",
    "EmptyProjectCleanup",
    "CloudExecutionWorker",
    "ExecutionControlledEndpoint",
}


class Held(ValueError):
    pass


def sha(value):
    return hashlib.sha256(value).hexdigest()


def inspect_archive(path, package, key=None):
    raw = path.read_bytes()
    if sha(raw) != package["zipSha256"] or len(raw) != package["zipBytes"]:
        raise Held("archive-hash-held")
    with zipfile.ZipFile(path) as archive:
        entries = archive.infolist()
        if len(entries) != 3 or {p.filename for p in entries} != {
            "./",
            "bootstrap",
            "resource.enc",
        }:
            raise Held("archive-members-held")
        for p in entries:
            mode = p.external_attr >> 16
            if p.flag_bits & 1 or stat.S_ISLNK(mode) or p.file_size > 40 * 1024 * 1024:
                raise Held("archive-entry-held")
            if p.filename == "./" and (not p.is_dir() or p.file_size != 0):
                raise Held("archive-directory-held")
            if p.filename != "./" and p.is_dir():
                raise Held("archive-file-held")
        bootstrap = archive.read("bootstrap")
        if (
            len(bootstrap) != package["bootstrapBytes"]
            or sha(bootstrap) != package["bootstrapSha256"]
        ):
            raise Held("bootstrap-held")
        encrypted = archive.read("resource.enc")
        if len(encrypted) != 18:
            raise Held("unexpected-binding-length-held")
        authenticated = False
        if key is not None:
            from cryptography.hazmat.primitives.ciphers.aead import AESGCM

            if len(key) != 32:
                raise Held("binding-key-held")
            try:
                plain = AESGCM(key).decrypt(bytes(12), encrypted, None)
            except Exception as error:
                raise Held("binding-authentication-held") from error
            if plain != b"{}":
                raise Held("unexpected-binding-content-held")
            authenticated = True
    return {
        "name": package["name"],
        "zipSha256": package["zipSha256"],
        "exactMembers": True,
        "bootstrapMatches": True,
        "bindingBytes": 18,
        "bindingAuthenticated": authenticated,
        "bindingContent": "empty-object" if authenticated else "unverified-encrypted",
        "credentialOrUnrelatedBindingAbsenceConfirmed": authenticated,
    }


def check(manifest, key=None):
    original = ROOT / "docs/development/project-environments-release.json"
    if (
        sha(original.read_bytes()) != ORIGINAL_SHA
        or manifest["releaseSourceCommit"] != SOURCE
    ):
        raise Held("release-parent-held")
    packages = manifest["packages"]
    if len(packages) != 4 or {p["name"] for p in packages} != NAMES:
        raise Held("package-set-held")
    result = []
    for package in packages:
        path = ROOT / manifest["artifactDirectory"] / package["name"] / "code.zip"
        if not path.resolve().is_relative_to(
            (ROOT / ".build/project-environments-sst-release").resolve()
        ):
            raise Held("archive-location-held")
        result.append(inspect_archive(path, package, key))
    return {
        "localOnly": True,
        "awsCalls": 0,
        "releaseSource": SOURCE,
        "packages": result,
        "bindingAuthenticated": all(p["bindingAuthenticated"] for p in result),
        "adoptionHeld": not all(p["bindingAuthenticated"] for p in result),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--manifest",
        type=Path,
        default=ROOT / "docs/development/project-environments-sst-release.json",
    )
    parser.add_argument("--private-key-file", type=Path)
    parser.add_argument("--regenerated-directory", type=Path)
    args = parser.parse_args()
    key = None
    if args.private_key_file:
        p = args.private_key_file
        if (
            p.is_symlink()
            or not p.is_file()
            or p.stat().st_mode & 0o077
            or p.stat().st_size > 128
        ):
            raise Held("private-key-file-held")
        key = base64.b64decode(p.read_bytes().strip(), validate=True)
    raw_manifest = args.manifest.read_bytes()
    if sha(raw_manifest) != MANIFEST_SHA:
        raise Held("reviewed-manifest-held")
    manifest = json.loads(raw_manifest)
    review_path = (
        ROOT / "docs/development/project-environments-refresh-preview-review-8.json"
    )
    if sha(review_path.read_bytes()) != manifest["review8ResultSha256"]:
        raise Held("review-8-result-held")
    review = json.loads(review_path.read_bytes())
    for name, expected in review["preservation"]["originalProofSha256"].items():
        if sha((ROOT / name).read_bytes()) != expected:
            raise Held("original-proof-held")
    for name, expected in review["preservation"][
        "privateEvidenceExcludingCaptures"
    ].items():
        if sha((ROOT / name).read_bytes()) != expected["sha256"]:
            raise Held("review-8-evidence-held")
    result = check(manifest, key)
    result["cumulativeUsage"] = 296
    result["remainingPreparationHeadroom"] = 204
    result["regenerationCompared"] = False
    if args.regenerated_directory:
        for package in manifest["packages"]:
            inspect_archive(
                args.regenerated_directory / package["name"] / "code.zip", package, key
            )
        result["regenerationCompared"] = True
    # The checked bootstrap bytes are the original clean Go builds; Go's module
    # build information is read without executing the arm64 Lambda binaries.
    for package in result["packages"]:
        original = json.loads(
            (ROOT / "docs/development/project-environments-release.json").read_text()
        )
        bootstrap = ROOT / original["artifactDirectory"] / package["name"] / "bootstrap"
        info = subprocess.check_output(
            ["go", "version", "-m", str(bootstrap)], text=True
        )
        if "github.com/sst/sst" in info:
            raise Held("unexpected-sst-loader-dependency-held")
    result["goSstSdkLoaderDependency"] = False
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception:  # noqa: BLE001 - never expose sensitive diagnostics
        raise SystemExit(
            "Offline archive validation held; inspect private inputs. No key/plaintext/error publication."
        ) from None
