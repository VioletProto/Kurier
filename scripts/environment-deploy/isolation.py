"""All provider/Go/Node/CLI descendants use the reviewed namespace/sole relay."""

import os
import shutil
import subprocess
import sys
import sysconfig
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "preview-meter"))
from common import Held
from run import MITM, ROOT, port, sandbox, write


def launch(
    source,
    directory,
    command,
    credentials,
    *,
    campaign,
    phase,
    policy_context,
    extra_ro=(),
    fixture_origin=None,
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
    if (
        fixture_origin
        and policy_context.get("fixtureOnly")
        and policy_context.get("seedArtifacts")
    ):
        import json

        release = json.loads(
            (
                ROOT / "docs/development/project-environments-adopted-release.json"
            ).read_text()
        )
        for p in release["packages"]:
            target = work / "sst/artifacts" / p["name"]
            target.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(
                ROOT / release["artifactDirectory"] / p["name"] / "code.zip",
                target / "code.zip",
            )
        (work / "sst/frontend-hosting-artifact.json").write_text(
            json.dumps(release["frontend"])
        )
    # No inherited proxy/auth/process env. Host meter configuration/ledger/private CA
    # are outside every child mount. Only the relay socket and public CA are exposed.
    config = {
        "campaign": str(campaign.directory.resolve()),
        "phase": phase,
        "context": policy_context,
        "fixtureOrigin": fixture_origin,
    }
    write(directory / "config.json", config)
    proxy_env = {
        "PATH": os.environ["PATH"],
        "HOME": str(Path.home()),
        "KURIER_ENV_EXECUTION_CONFIG": str(directory / "config.json"),
        "PYTHONDONTWRITEBYTECODE": "1",
        # The pinned SDK is installed in the system interpreter's purelib;
        # mitmproxy's dedicated venv does not otherwise expose it.
        "PYTHONPATH": sysconfig.get_path("purelib"),
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
                str(Path(__file__).with_name("proxy.py")),
            ],
            env=proxy_env,
            stdout=errors,
            stderr=errors,
            close_fds=True,
        )
        errors.close()
        processes.append(proxy)
        for _ in range(100):
            if proxy.poll() is not None:
                private = Path(os.environ.get("KURIER_TEST_PRIVATE_DIR", "/tmp"))
                if policy_context.get("fixtureOnly"):
                    private.mkdir(mode=0o700, parents=True, exist_ok=True)
                    source_log = directory / "meter-errors.txt"
                    if source_log.exists():
                        destination = private / (directory.name + "-meter-errors.txt")
                        shutil.copyfile(source_log, destination)
                        os.chmod(destination, 0o600)
                raise RuntimeError("proxy-start-failed")
            if (directory / "ca/mitmproxy-ca-cert.pem").exists() and (
                directory / "proxy-ready"
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
            "GOTOOLCHAIN": "local",
            "GOSUMDB": "off",
            "GOCACHE": "/work/go-cache",
            "KURIER_ATTEMPT_METER_ACTIVE": "isolated-preview",
            "NODE_OPTIONS": "--use-env-proxy --import="
            + str(ROOT / "scripts/attempt-meter/node-proxy.mjs"),
            "AWS_CA_BUNDLE": "/meter/ca.pem",
            "SSL_CERT_FILE": "/meter/ca.pem",
            "NODE_EXTRA_CA_CERTS": "/meter/ca.pem",
            "AWS_MAX_ATTEMPTS": "1",
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
        deadline = campaign.get("deadline")
        while child.poll() is None:
            try:
                campaign.check()
            except Held:
                break
            if (
                proxy.poll() is not None
                or relay.poll() is not None
                or time.monotonic() >= deadline
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
        for _ in range(40):
            if not campaign.db.execute(
                "SELECT COUNT(*) FROM attempts WHERE outcome=0"
            ).fetchone()[0]:
                break
            if proxy.poll() is not None:
                result = 77
                break
            time.sleep(0.05)
        else:
            result = 78
        return {"exitCode": result, "directory": str(directory)}
    finally:
        if policy_context.get("fixtureOnly") and os.environ.get(
            "KURIER_TEST_PRIVATE_DIR"
        ):
            private = Path(os.environ["KURIER_TEST_PRIVATE_DIR"])
            for name in ("meter-errors.txt", "private-output.txt"):
                log = directory / name
                if log.exists():
                    dest = private / (directory.name + "-" + name)
                    shutil.copyfile(log, dest)
                    os.chmod(dest, 0o600)
        if child and child.poll() is None:
            child.kill()
            child.wait()
        for p in reversed(processes):
            if p.poll() is None:
                p.terminate()
            p.wait(timeout=5)
        shutil.rmtree(directory / "ca", ignore_errors=True)
