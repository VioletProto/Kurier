"""Scoped operator launcher. All child TCP/UDP is isolated from host networking."""

import argparse
import configparser
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import socket
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
PYTHON = ROOT / ".build/attempt-meter-venv/bin/python"
AUTH = [
    "CreateOAuth2Token",
    "InitiateAuth",
    "RespondToAuthChallenge",
    "RevokeToken",
    "GetTokensFromRefreshToken",
]
PHASES = {
    "inspect": {
        "writeCategories": ["aws"],
        "awsWriteOperations": ["CreateOAuth2Token"],
    },
    "seed": {
        "writeCategories": ["aws", "api"],
        "awsWriteOperations": AUTH + ["AdminCreateUser", "AdminSetUserPassword"],
    },
    "close": {
        "writeCategories": ["aws"],
        "awsWriteOperations": ["CreateOAuth2Token", "TransactWriteItems"],
    },
    "deploy": {
        "writeCategories": ["aws"],
        "awsWriteOperations": ["CreateOAuth2Token", "PUT", "POST", "DELETE"],
    },
    "activate": {
        "writeCategories": ["aws"],
        "awsWriteOperations": ["CreateOAuth2Token", "TransactWriteItems"],
    },
    "runtime": {"writeCategories": ["aws", "api"], "awsWriteOperations": AUTH},
    "cleanup": {
        "writeCategories": ["aws", "api"],
        "awsWriteOperations": AUTH + ["AdminUserGlobalSignOut", "AdminDeleteUser"],
    },
}


def write_json(path, data):
    temp = path.with_suffix(".tmp")
    with open(temp, "x", encoding="utf8") as file:
        os.chmod(temp, 0o600)
        json.dump(data, file)
        file.flush()
        os.fsync(file.fileno())
    os.replace(temp, path)


def events(directory):
    return [
        json.loads(line)
        for line in (directory / "attempts.jsonl").read_text().splitlines()
    ]


def serve(directory):
    directory.mkdir(mode=0o700, parents=True, exist_ok=False)
    write_json(directory / "control.json", PHASES["inspect"])
    processes = []
    # Let the official CLI preserve rotated credentials in its one existing
    # protected login-cache file. Never copy them into task artifacts or logs.
    # Fixture passwords/access/refresh/MCP tokens remain process memory only.
    config = configparser.RawConfigParser()
    config.read(Path.home() / ".aws/config")
    session = config.get("default", "login_session", fallback="")
    if not session.startswith("arn:aws:iam::747336059622:user/"):
        raise RuntimeError("Intended existing non-root CLI login profile required")
    key = hashlib.sha256(session.encode()).hexdigest() + ".json"
    cache_file = Path.home() / ".aws/login/cache" / key
    if not cache_file.is_file():
        raise RuntimeError("Existing intended CLI cache required")
    port_socket = socket.socket()
    port_socket.bind(("127.0.0.1", 0))
    port = port_socket.getsockname()[1]
    port_socket.close()
    env = {
        k: v
        for k, v in os.environ.items()
        if not k.startswith(("AWS_", "KURIER_", "NODE_"))
        and k not in ("SSLKEYLOGFILE", "SSLKEYLOG", "QLOGDIR")
    }
    env.update(
        KURIER_METER_LEDGER=str(directory / "attempts.jsonl"),
        KURIER_METER_CONTROL=str(directory / "control.json"),
    )

    def terminate(_signum=None, _frame=None):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, terminate)
    try:
        proxy = subprocess.Popen(
            [
                str(ROOT / ".build/attempt-meter-venv/bin/mitmdump"),
                "--quiet",
                "--listen-host",
                "127.0.0.1",
                "--listen-port",
                str(port),
                "--set",
                "confdir=" + str(directory / "ca"),
                "--set",
                "connection_strategy=lazy",
                "--set",
                "upstream_cert=false",
                "--set",
                "http2=true",
                "-s",
                str(HERE / "proxy.py"),
            ],
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        processes.append(proxy)
        for _ in range(100):
            if proxy.poll() is not None:
                raise RuntimeError("Meter proxy failed to start")
            if (directory / "ca/mitmproxy-ca-cert.pem").exists() and (
                directory / "attempts.jsonl"
            ).exists():
                break
            time.sleep(0.1)
        else:
            raise RuntimeError("Meter startup deadline")
        relay = subprocess.Popen(
            [
                str(PYTHON),
                str(HERE / "relay.py"),
                "host",
                str(directory / "relay.sock"),
                str(port),
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        processes.append(relay)
        for _ in range(100):
            if (directory / "relay.sock").exists():
                break
            time.sleep(0.05)
        write_json(
            directory / "state.json",
            {
                "proxyPid": proxy.pid,
                "relayPid": relay.pid,
                "loginCacheFile": str(cache_file),
                "acceptedExecutionPacket": "f3a698d565fa29e99cd7f89517252994c32e44ef",
            },
        )
        print(
            json.dumps({"ready": True, "phase": "inspect", "liveCalls": 0}),
            flush=True,
        )
        while proxy.poll() is None and relay.poll() is None:
            time.sleep(0.2)
        raise RuntimeError("Meter process unexpectedly exited; no restart/resume")
    except KeyboardInterrupt:
        pass
    finally:
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
            process.wait(timeout=10)
        shutil.rmtree(directory / "ca", ignore_errors=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["serve", "exec", "phase", "stats"])
    parser.add_argument("directory", type=Path)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    directory = args.directory.resolve()
    if args.action == "serve":
        return serve(directory)
    state = json.loads((directory / "state.json").read_text())
    for pid in [state["proxyPid"], state["relayPid"]]:
        os.kill(pid, 0)
    log = events(directory)
    if args.action == "stats":
        attempts = [e for e in log if e["event"] == "attempt"]
        print(
            json.dumps(
                {
                    "counts": attempts[-1]["counts"] if attempts else {},
                    "stopped": any(e["event"] == "stop" for e in log),
                    "firstAttemptAt": attempts[0]["at"] if attempts else None,
                }
            )
        )
        return
    if any(e["event"] == "stop" for e in log):
        raise RuntimeError("Meter stop is latched; no automatic resume/reopen")
    if args.action == "phase":
        if len(args.command) != 1 or args.command[0] not in PHASES:
            raise RuntimeError("Explicit known phase required")
        write_json(directory / "control.json", PHASES[args.command[0]])
        print(json.dumps({"phase": args.command[0], "awsWrites": 0}))
        return
    if not args.command:
        raise RuntimeError("Explicit command required")
    env = dict(os.environ)
    for key in ("SSLKEYLOGFILE", "SSLKEYLOG", "QLOGDIR"):
        env.pop(key, None)
    for key in (
        "AWS_PROFILE",
        "AWS_DEFAULT_PROFILE",
        "AWS_ACCESS_KEY_ID",
        "AWS_SECRET_ACCESS_KEY",
        "AWS_SESSION_TOKEN",
        "AWS_WEB_IDENTITY_TOKEN_FILE",
        "AWS_ROLE_ARN",
        "AWS_CONTAINER_CREDENTIALS_FULL_URI",
        "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
    ):
        env.pop(key, None)
    for key in ("NO_PROXY", "no_proxy"):
        env[key] = "127.0.0.1,localhost,::1"
    for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy"):
        env[key] = "http://127.0.0.1:18080"
    env.update(
        AWS_CA_BUNDLE=str(directory / "ca/mitmproxy-ca-cert.pem"),
        SSL_CERT_FILE=str(directory / "ca/mitmproxy-ca-cert.pem"),
        NODE_EXTRA_CA_CERTS=str(directory / "ca/mitmproxy-ca-cert.pem"),
        NODE_OPTIONS="--use-env-proxy --import=" + str(HERE / "node-proxy.mjs"),
        KURIER_ATTEMPT_METER_ACTIVE="live-isolated",
        KURIER_ATTEMPT_METER_DIRECTORY=str(directory),
        AWS_MAX_ATTEMPTS="1",
        AWS_LOGIN_CACHE_DIRECTORY=str(Path(state["loginCacheFile"]).parent),
        AWS_CONFIG_FILE=str(Path.home() / ".aws/config"),
        AWS_SHARED_CREDENTIALS_FILE="/dev/null",
        AWS_REGION="us-east-2",
        AWS_EC2_METADATA_DISABLED="true",
        SST_TELEMETRY_DISABLED="1",
        PULUMI_SKIP_UPDATE_CHECK="true",
        NPM_CONFIG_UPDATE_NOTIFIER="false",
        NPM_CONFIG_OFFLINE="true",
        GOCACHE=str(ROOT / ".build/attempt-meter-go-cache"),
    )
    command = [
        "bwrap",
        "--unshare-net",
        "--ro-bind",
        "/",
        "/",
        "--bind",
        str(ROOT),
        str(ROOT),
        "--bind",
        "/tmp",
        "/tmp",
        "--proc",
        "/proc",
        "--dev",
        "/dev",
        "--bind",
        state["loginCacheFile"],
        state["loginCacheFile"],
        str(PYTHON),
        str(HERE / "relay.py"),
        "inside",
        str(directory / "relay.sock"),
        "18080",
        *args.command,
    ]
    return subprocess.run(command, env=env, check=False).returncode


if __name__ == "__main__":
    try:
        sys.exit(main() or 0)
    except (OSError, ValueError, RuntimeError, configparser.Error):
        print(
            "Meter launcher failed closed; no automatic retry/resume.", file=sys.stderr
        )
        sys.exit(1)
