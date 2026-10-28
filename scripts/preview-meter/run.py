"""Pinned preview launcher. No deploy mode. Do not run preview before review approval.

The fixture tests call the same isolation/supervision function with fake credentials.
"""

import argparse
import configparser
import datetime
import hashlib
import json
import os
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
PYTHON = ROOT / ".build/attempt-meter-venv/bin/python"
MITM = ROOT / ".build/attempt-meter-venv/bin/mitmdump"
sys.path.insert(0, str(HERE))
from accounting import MAXIMUM, PRIOR, baseline
from policy import PARENT_SHA, SOURCE

SST = ROOT / "node_modules/sst-linux-x64/bin/sst"
SST_SHA = "539659fa60f00e8b9610b7dfd967928d382b4800ca25452d9697e31a4eb521fd"
PROVIDER = Path.home() / ".config/sst/plugins/resource-aws-v7.40.0/pulumi-resource-aws"
PROVIDER_SHA = "741bfe58a4f6100dfa839beb98d53e0f27c9b63998d53a293cbbfb7a6b23a797"


def sha(p):
    return hashlib.sha256(Path(p).read_bytes()).hexdigest()


def git(*args, cwd=ROOT):
    return subprocess.check_output(["git", *args], cwd=cwd, text=True).strip()


def write(path, value):
    with open(path, "x", encoding="utf8") as f:
        os.chmod(path, 0o600)
        json.dump(value, f)
        f.flush()
        os.fsync(f.fileno())


def valid_parent():
    baseline(ROOT)
    p = ROOT / "docs/development/project-environments-inventory-attempts.jsonl"
    if sha(p) != PARENT_SHA:
        raise ValueError("parent-ledger-hash")
    rows = [json.loads(x) for x in p.read_text().splitlines()]
    attempts = [r for r in rows if r["event"] == "attempt"]
    if (
        len(attempts) != 135
        or attempts[-1]["count"] != 135
        or rows[-1]["event"] != "stop"
    ):
        raise ValueError("parent-ledger-shape")
    # Preserve and verify the exact existing local release and rollback bytes.
    m = json.loads(
        (ROOT / "docs/development/project-environments-release.json").read_text()
    )
    directory = ROOT / m["artifactDirectory"]
    for p in m["packages"]:
        if sha(directory / p["name"] / "code.zip") != p["zipSha256"]:
            raise ValueError("release-package-hash")
        if sha(directory / p["name"] / "bootstrap") != p["bootstrapSha256"]:
            raise ValueError("release-bootstrap-hash")
    for p in m["rollbackPackages"]:
        if sha(directory / "rollback" / p["name"] / "code.zip") != p["zipSha256"]:
            raise ValueError("rollback-package-hash")
        if (
            sha(directory / "rollback" / p["name"] / "bootstrap")
            != p["bootstrapSha256"]
        ):
            raise ValueError("rollback-bootstrap-hash")
    for p in m["clients"]:
        if sha(directory / "clients" / p["name"]) != p["sha256"]:
            raise ValueError("client-hash")
    inventory = json.loads(
        (ROOT / "docs/development/project-environments-inventory.json").read_text()
    )
    if (
        sha(directory / "rollback/frontend/cloudfront-function.js")
        != inventory["cloudfrontFunction"]["codeSha256"]
    ):
        raise ValueError("rollback-function-hash")
    for folder in ["frontend", "rollback/frontend"]:
        artifact = json.loads((directory / folder / "artifact.json").read_text())
        for f in artifact["files"]:
            p = directory / folder / "app" / f["path"]
            if sha(p) != f["sha256"] or p.stat().st_size != f["bytes"]:
                raise ValueError("frontend-file-hash")
    return m


def cached_credentials():
    # Local cache read only: no provider chain, refresh, SSO, process or IMDS calls.
    cfg = configparser.RawConfigParser()
    cfg.read(Path.home() / ".aws/config")
    session = cfg.get("default", "login_session")
    if not session.startswith("arn:aws:iam::747336059622:user/"):
        raise ValueError("credential-identity")
    p = (
        Path.home()
        / ".aws/login/cache"
        / (hashlib.sha256(session.encode()).hexdigest() + ".json")
    )
    access = json.loads(p.read_text())["accessToken"]
    expires = datetime.datetime.fromisoformat(access["expiresAt"])
    if (expires - datetime.datetime.now(datetime.UTC)).total_seconds() < 120:
        raise ValueError("credential-window-too-short-refresh-held")
    if access["accountId"] != "747336059622":
        raise ValueError("credential-account")
    return {
        "AWS_ACCESS_KEY_ID": access["accessKeyId"],
        "AWS_SECRET_ACCESS_KEY": access["secretAccessKey"],
        "AWS_SESSION_TOKEN": access["sessionToken"],
    }


def port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def sandbox(source, directory, command, env, extra_ro=()):
    # Empty root, private pid/net/user/ipc/uts/cgroup namespaces, no host services,
    # no host /proc, /run, /tmp, /home, SSH agent, Docker or credential caches.
    args = [
        "bwrap",
        "--unshare-all",
        "--unshare-user",
        "--die-with-parent",
        "--new-session",
        "--disable-userns",
        "--assert-userns-disabled",
        "--cap-drop",
        "ALL",
        "--ro-bind",
        "/usr",
        "/usr",
    ]
    for name, target in [
        ("bin", "usr/bin"),
        ("sbin", "usr/bin"),
        ("lib", "usr/lib"),
        ("lib64", "usr/lib"),
    ]:
        args += ["--symlink", target, "/" + name]
    args += [
        "--proc",
        "/proc",
        "--dev",
        "/dev",
        "--tmpfs",
        "/tmp",
        "--tmpfs",
        "/run",
        "--dir",
        "/var",
        "--dir",
        "/etc",
        "--dir",
        "/meter",
        "--dir",
        "/work",
    ]
    for name in [
        "ssl",
        "ca-certificates",
        "resolv.conf",
        "nsswitch.conf",
        "passwd",
        "group",
        "hosts",
        "ld.so.cache",
    ]:
        p = Path("/etc") / name
        if p.exists():
            args += ["--ro-bind", str(p), str(p)]
    mounts = [
        (source, source),
        (ROOT / "node_modules", ROOT / "node_modules"),
        (ROOT / ".sst/platform/node_modules", ROOT / ".sst/platform/node_modules"),
        # Resolve the workspace dependency alias to pinned source, never the
        # developer checkout's ignored dotenv/config files.
        (source / "apps/web", ROOT / "apps/web"),
        (ROOT / "scripts/attempt-meter", ROOT / "scripts/attempt-meter"),
        (HERE, HERE),
        (ROOT / ".git", ROOT / ".git"),
        (Path.home() / "go/pkg/mod", Path.home() / "go/pkg/mod"),
        (Path.home() / ".config/sst/bin", Path.home() / ".config/sst/bin"),
        (Path.home() / ".config/sst/plugins", Path.home() / ".config/sst/plugins"),
    ]
    for src, dest in [*mounts, *extra_ro]:
        if Path(src).exists():
            args += ["--ro-bind", str(src), str(dest)]
    args += [
        "--bind",
        str(directory / "work"),
        "/work",
        "--bind",
        str(directory / "work/sst"),
        str(source / ".sst"),
        "--bind",
        str(directory / "work/build"),
        str(source / ".build"),
        "--bind",
        str(directory / "work/dist"),
        str(source / "apps/web/dist"),
        "--bind",
        str(directory / "work/web-modules"),
        str(source / "apps/web/node_modules"),
        "--ro-bind",
        str(directory / "relay.sock"),
        "/meter/relay.sock",
        "--ro-bind",
        str(directory / "ca/mitmproxy-ca-cert.pem"),
        "/meter/ca.pem",
        "--chdir",
        str(source),
    ]
    return [
        *args,
        "/usr/bin/python3",
        str(ROOT / "scripts/attempt-meter/relay.py"),
        "inside",
        "/meter/relay.sock",
        "18080",
        *command,
    ]


def launch(
    source,
    directory,
    command,
    credentials,
    *,
    fixture_origin=None,
    budget=None,
    extra_ro=(),
    fixture_failure=False,
):
    directory.mkdir(mode=0o700, parents=True, exist_ok=False)
    work = directory / "work"
    work.mkdir()
    (work / "build").mkdir()
    (work / "dist").mkdir()
    (work / "web-modules").mkdir()
    shutil.copytree(
        ROOT / ".sst/platform",
        work / "sst/platform",
        ignore=shutil.ignore_patterns("node_modules", "sst.config.*.mjs"),
    )
    shutil.copyfile(ROOT / ".sst/provider-lock.json", work / "sst/provider-lock.json")
    (work / "sst/platform/node_modules").symlink_to(ROOT / ".sst/platform/node_modules")
    (work / "sst/platform/node_modules").is_dir() or (_ for _ in ()).throw(
        ValueError("missing-platform-cache")
    )
    # No inherited proxy/auth/process env. Host meter configuration/ledger/private CA
    # are outside every child mount. Only the relay socket and public CA are exposed.
    config = {
        "mode": "fixture" if fixture_origin else "preview",
        "ledger": str(directory / "attempts.jsonl"),
        "budget": {"prior": PRIOR, "maximum": MAXIMUM, "seconds": 3600},
    }
    if not fixture_origin:
        config["budget"] = {"prior": PRIOR, "maximum": MAXIMUM, "seconds": 3600}
        config["accounting"] = baseline(ROOT)
    if fixture_origin:
        config.update(fixtureOrigin=fixture_origin)
        config["budget"].update(budget or {})
    if fixture_failure:
        config["fixtureFailure"] = True
    write(directory / "config.json", config)
    proxy_env = {
        "PATH": os.environ["PATH"],
        "HOME": str(Path.home()),
        "KURIER_PREVIEW_METER_CONFIG": str(directory / "config.json"),
        "PYTHONDONTWRITEBYTECODE": "1",
    }
    listener = port()
    processes = []
    child = None
    result = None
    try:
        errors = open(directory / "meter-errors.txt", "wb")  # noqa: SIM115 - closed after child inherits descriptor
        proxy = subprocess.Popen(
            [
                str(MITM),
                "--quiet",
                "--listen-host",
                "127.0.0.1",
                "--listen-port",
                str(listener),
                "--set",
                "confdir=" + str(directory / "ca"),
                "--set",
                "connection_strategy=lazy",
                "--set",
                "upstream_cert=false",
                "--set",
                "http2=true",
                "--set",
                "body_size_limit=64m",
                "-s",
                str(HERE / "proxy.py"),
            ],
            env=proxy_env,
            stdout=subprocess.DEVNULL,
            stderr=errors,
            close_fds=True,
        )
        errors.close()
        processes.append(proxy)
        for _ in range(100):
            if proxy.poll() is not None:
                raise RuntimeError("proxy-start-failed")
            if (directory / "ca/mitmproxy-ca-cert.pem").exists() and (
                directory / "attempts.jsonl"
            ).exists():
                break
            time.sleep(0.05)
        else:
            raise RuntimeError("proxy-start-deadline")
        relay = subprocess.Popen(
            [
                "/usr/bin/python3",
                str(ROOT / "scripts/attempt-meter/relay.py"),
                "host",
                str(directory / "relay.sock"),
                str(listener),
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            close_fds=True,
        )
        processes.append(relay)
        for _ in range(100):
            if (directory / "relay.sock").exists():
                break
            time.sleep(0.05)
        else:
            raise RuntimeError("relay-start-deadline")
        env = {
            "PATH": "/usr/bin:/bin",
            "HOME": str(Path.home()),
            "XDG_CONFIG_HOME": str(Path.home() / ".config"),
            "AWS_REGION": "us-east-2",
            "AWS_DEFAULT_REGION": "us-east-2",
            "AWS_EC2_METADATA_DISABLED": "true",
            "AWS_CONFIG_FILE": "/dev/null",
            "AWS_SHARED_CREDENTIALS_FILE": "/dev/null",
            "SST_TELEMETRY_DISABLED": "1",
            "SST_SKIP_DEPENDENCY_CHECK": "1",
            "SST_NO_BUN": "1",
            "PULUMI_SKIP_UPDATE_CHECK": "true",
            "PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION": "true",
            "PULUMI_HOME": str(Path.home() / ".config/sst"),
            "NPM_CONFIG_OFFLINE": "true",
            "NPM_CONFIG_UPDATE_NOTIFIER": "false",
            "GOPROXY": "off",
            "GOSUMDB": "off",
            "GOCACHE": "/work/go-cache",
            "KURIER_ATTEMPT_METER_ACTIVE": "isolated-preview",
            "NODE_OPTIONS": "--use-env-proxy --import="
            + str(ROOT / "scripts/attempt-meter/node-proxy.mjs"),
            "AWS_CA_BUNDLE": "/meter/ca.pem",
            "SSL_CERT_FILE": "/meter/ca.pem",
            "NODE_EXTRA_CA_CERTS": "/meter/ca.pem",
            **credentials,
        }
        for key in [
            "HTTP_PROXY",
            "HTTPS_PROXY",
            "ALL_PROXY",
            "http_proxy",
            "https_proxy",
            "all_proxy",
        ]:
            env[key] = "http://127.0.0.1:18080"
        env["NO_PROXY"] = env["no_proxy"] = "127.0.0.1,localhost,::1"
        env.update(
            KURIER_AWS_ACCOUNT_ID="747336059622",
            KURIER_COGNITO_POOL_ID="us-east-2_qTDZQT1FE",
            KURIER_COGNITO_CLIENT_ID="5jg9sb38i6ae9c0adm5rtdfkno",
            KURIER_FRONTEND_ORIGIN="http://localhost:5173",
        )
        raw = directory / "private-output.txt"
        fd = os.open(raw, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        child = subprocess.Popen(
            sandbox(source, directory, command, env, extra_ro),
            env=env,
            stdout=fd,
            stderr=fd,
            close_fds=True,
        )
        os.close(fd)
        started = time.monotonic()
        seconds = (budget or {}).get("seconds", 3600)
        while child.poll() is None:
            rows = [
                json.loads(x)
                for x in (directory / "attempts.jsonl").read_text().splitlines()
            ]
            if (
                proxy.poll() is not None
                or relay.poll() is not None
                or any(x["event"] == "stop" for x in rows)
                or time.monotonic() - started >= seconds
            ):
                break
            time.sleep(0.05)
        if child.poll() is None:
            child.terminate()
        try:
            result = child.wait(timeout=5)
        except subprocess.TimeoutExpired:
            child.kill()
            result = child.wait()
        if proxy.poll() is not None or relay.poll() is not None:
            result = 77
        # Drain already charged reads; missing outcomes never become successful proof.
        for _ in range(40):
            rows = [
                json.loads(x)
                for x in (directory / "attempts.jsonl").read_text().splitlines()
            ]
            aa = {x["count"] for x in rows if x["event"] == "attempt"}
            oo = {x["attempt"] for x in rows if x["event"] == "outcome"}
            if aa == oo:
                break
            if proxy.poll() is not None:
                result = 77
                break
            time.sleep(0.05)
        else:
            result = 78
        return {
            "exitCode": result,
            "rows": [
                json.loads(x)
                for x in (directory / "attempts.jsonl").read_text().splitlines()
            ],
            "directory": str(directory),
        }
    finally:
        if child and child.poll() is None:
            child.kill()
            child.wait()
        for p in reversed(processes):
            if p.poll() is None:
                p.terminate()
            p.wait(timeout=5)
        shutil.rmtree(directory / "ca", ignore_errors=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["check", "preview"])
    parser.add_argument(
        "--source", type=Path, default=Path("/tmp/kurier-environments-release-source")
    )
    parser.add_argument("--output", type=Path)
    parser.add_argument("--review-commit")
    a = parser.parse_args()
    m = valid_parent()
    if sha(SST) != SST_SHA:
        raise ValueError("sst-binary-hash")
    if (
        sha(PROVIDER) != PROVIDER_SHA
        or (ROOT / ".sst/platform/version").read_text() != "4.17.1"
    ):
        raise ValueError("provider-platform-pin")
    if git("rev-parse", "HEAD", cwd=a.source) != SOURCE or git(
        "status", "--porcelain", "--untracked-files=all", cwd=a.source
    ):
        raise ValueError("clean-pinned-source-required")
    if a.action == "check":
        print(
            json.dumps(
                {
                    "localChecksPassed": True,
                    "priorCalls": PRIOR,
                    "remainingCalls": MAXIMUM - PRIOR,
                    "source": SOURCE,
                    "awsCalls": 0,
                }
            )
        )
        return
    if (
        a.review_commit != git("rev-parse", "HEAD")
        or not a.output
        or git("status", "--porcelain", "--untracked-files=all")
    ):
        raise ValueError("reviewed-head-and-new-output-required")
    base = (ROOT / ".build/project-environments-preview").resolve()
    if a.output.resolve().parent != base or a.source.resolve().is_relative_to(base):
        raise ValueError("private-output-outside-child-mounts-required")
    creds = cached_credentials()
    command = [str(SST), "diff", "--stage", "dev-api", "--json"]
    r = launch(a.source.resolve(), a.output.resolve(), command, creds)
    valid_parent()
    packaging = []
    for p in m["packages"]:
        z = a.output.resolve() / "work/sst/artifacts" / p["name"] / "code.zip"
        if z.exists():
            packaging.append(
                {
                    "name": p["name"],
                    "bytes": z.stat().st_size,
                    "sha256": sha(z),
                    "matchesPreparedZip": sha(z) == p["zipSha256"],
                }
            )
    frontend_path = a.output.resolve() / "work/sst/frontend-hosting-artifact.json"
    frontend = json.loads(frontend_path.read_text()) if frontend_path.exists() else None
    write(
        a.output.resolve() / "packaging-review.json",
        {
            "releaseSource": SOURCE,
            "packages": packaging,
            "frontend": frontend,
            "matchesPreparedFrontend": frontend == m["frontend"],
            "deploymentPerformed": False,
            "activationPerformed": False,
            "requiresSeparateReview": True,
        },
    )
    print(
        json.dumps(
            {
                "exitCode": r["exitCode"],
                "newCalls": sum(x["event"] == "attempt" for x in r["rows"]),
                "priorCalls": PRIOR,
                "stopped": any(x["event"] == "stop" for x in r["rows"]),
                "previewOnly": True,
            }
        )
    )
    sys.exit(
        0
        if r["exitCode"] == 0 and not any(x["event"] == "stop" for x in r["rows"])
        else 1
    )


if __name__ == "__main__":
    try:
        main()
    except (
        OSError,
        ValueError,
        RuntimeError,
        subprocess.SubprocessError,
        configparser.Error,
    ):
        print(
            "Preview launcher failed closed. No automatic retry or continuation.",
            file=sys.stderr,
        )
        sys.exit(1)
