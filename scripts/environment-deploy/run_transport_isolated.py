"""Run local transport fixtures in a network namespace with explicit cache mounts."""

import argparse
import os
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RELEASE = Path("/tmp/kurier-environments-release-source")
PRIVATE = ROOT / ".build/environment-deploy-test-private"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--context-only", action="store_true")
    parser.add_argument("--denial-only", action="store_true")
    parser.add_argument("--packaging-only", action="store_true")
    options = parser.parse_args()
    if not (RELEASE / ".git").exists():
        raise SystemExit("pinned-release-worktree-missing")
    PRIVATE.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(PRIVATE, 0o700)
    private_run = Path(tempfile.mkdtemp(prefix="context-review-", dir=PRIVATE))
    args = [
        "bwrap",
        "--unshare-all",
        "--unshare-user",
        "--die-with-parent",
        "--ro-bind",
        "/usr",
        "/usr",
        "--symlink",
        "usr/bin",
        "/bin",
        "--symlink",
        "usr/bin",
        "/sbin",
        "--symlink",
        "usr/lib",
        "/lib",
        "--symlink",
        "usr/lib",
        "/lib64",
        "--ro-bind",
        "/etc",
        "/etc",
        "--dev",
        "/dev",
        "--proc",
        "/proc",
        "--tmpfs",
        "/tmp",
        "--ro-bind",
        str(RELEASE),
        str(RELEASE),
        "--ro-bind",
        str(ROOT),
        str(ROOT),
        "--bind",
        str(private_run),
        str(private_run),
    ]
    for path in [
        Path.home() / "go/pkg/mod",
        Path.home() / ".config/sst/bin",
        Path.home() / ".config/sst/plugins",
    ]:
        if path.exists():
            args += ["--ro-bind", str(path), str(path)]
    env = {k: v for k, v in os.environ.items() if not k.startswith(("AWS_", "KURIER_"))}
    env.update(
        PYTHONDONTWRITEBYTECODE="1",
        GOCACHE="/tmp/kurier-fixture-go-cache",
        GOPROXY="off",
        GOTOOLCHAIN="local",
        KURIER_TEST_PRIVATE_DIR=str(private_run),
        AWS_CONFIG_FILE="/dev/null",
        AWS_SHARED_CREDENTIALS_FILE="/dev/null",
        AWS_EC2_METADATA_DISABLED="true",
    )
    if options.packaging_only or not (options.context_only or options.denial_only):
        packaging = subprocess.run(
            [
                *args,
                "--",
                sys.executable,
                "-B",
                str(ROOT / "scripts/environment-deploy/test_packaging_isolated.py"),
            ],
            check=False,
            cwd=ROOT,
            env=env,
        )
        if packaging.returncode or options.packaging_only:
            return packaging.returncode
    if not options.context_only and not options.denial_only:
        first = subprocess.run(
            [
                *args,
                "--",
                sys.executable,
                "-B",
                str(ROOT / "scripts/environment-deploy/test_transport.py"),
            ],
            check=False,
            cwd=ROOT,
            env=env,
        )
        if first.returncode:
            return first.returncode
    if not options.context_only:
        denial = subprocess.run(
            [
                *args,
                "--",
                sys.executable,
                "-B",
                str(ROOT / "scripts/environment-deploy/test_denial_isolated.py"),
            ],
            check=False,
            cwd=ROOT,
            env=env,
        )
        if denial.returncode or options.denial_only:
            return denial.returncode
    fixture_campaigns = private_run / "synthetic-campaigns"
    fixture_campaigns.mkdir(mode=0o700)
    (fixture_campaigns / "fixture-only-overlay").write_text("synthetic")
    return subprocess.run(
        [
            *args,
            "--bind",
            str(fixture_campaigns),
            str(ROOT / ".build/project-environments-execution"),
            "--",
            sys.executable,
            "-B",
            str(ROOT / "scripts/environment-deploy/test_context_isolated.py"),
        ],
        check=False,
        cwd=ROOT,
        env=env,
    ).returncode


if __name__ == "__main__":
    raise SystemExit(main())
