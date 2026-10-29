"""Run local transport fixtures in a network namespace with explicit cache mounts."""

import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RELEASE = Path("/tmp/kurier-environments-release-source")
PRIVATE = ROOT / ".build/environment-deploy-test-private"


def main():
    if not (RELEASE / ".git").exists():
        raise SystemExit("pinned-release-worktree-missing")
    PRIVATE.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(PRIVATE, 0o700)
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
        str(PRIVATE),
        str(PRIVATE),
    ]
    for path in [
        Path.home() / "go/pkg/mod",
        Path.home() / ".config/sst/bin",
        Path.home() / ".config/sst/plugins",
    ]:
        if path.exists():
            args += ["--ro-bind", str(path), str(path)]
    args += [
        "--",
        sys.executable,
        "-B",
        str(ROOT / "scripts/environment-deploy/test_transport.py"),
    ]
    return subprocess.run(
        args,
        check=False,
        cwd=ROOT,
        env={
            **os.environ,
            "PYTHONDONTWRITEBYTECODE": "1",
            "GOCACHE": "/tmp/kurier-fixture-go-cache",
            "GOPROXY": "off",
            "GOTOOLCHAIN": "local",
            "KURIER_TEST_PRIVATE_DIR": str(PRIVATE),
        },
    ).returncode


if __name__ == "__main__":
    raise SystemExit(main())
