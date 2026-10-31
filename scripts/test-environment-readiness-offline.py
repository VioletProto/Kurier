"""Run cached DynamoDB Local and readiness tests with external networking disabled."""

import argparse
import os
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--dynamodb-directory", type=Path, required=True)
parser.add_argument(
    "--inside-network-namespace", action="store_true", help=argparse.SUPPRESS
)
a = parser.parse_args()
jar = a.dynamodb_directory.resolve() / "DynamoDBLocal.jar"
if not jar.is_file():
    raise SystemExit("Cached DynamoDB Local JAR required; no download attempted.")
if not a.inside_network_namespace:
    raise SystemExit(
        subprocess.run(
            [
                "bwrap",
                "--unshare-net",
                "--ro-bind",
                "/",
                "/",
                "--bind",
                str(Path.home()),
                str(Path.home()),
                "--bind",
                "/tmp",
                "/tmp",
                "--proc",
                "/proc",
                "--dev",
                "/dev",
                sys.executable,
                "-B",
                str(Path(__file__).resolve()),
                "--dynamodb-directory",
                str(jar.parent),
                "--inside-network-namespace",
            ],
            check=False,
        ).returncode
    )
env = {k: v for k, v in os.environ.items() if not k.startswith(("AWS_", "KURIER_"))}
env.update(
    GOPROXY="off",
    GOSUMDB="off",
    GOTOOLCHAIN="local",
    AWS_CONFIG_FILE="/dev/null",
    AWS_SHARED_CREDENTIALS_FILE="/dev/null",
    AWS_EC2_METADATA_DISABLED="true",
    KURIER_DYNAMODB_TEST_ENDPOINT="http://127.0.0.1:8000",
    KURIER_ACTIVATOR_DDB_ENDPOINT="http://127.0.0.1:8000",
)
with tempfile.TemporaryFile() as log:
    server = subprocess.Popen(
        [
            "java",
            "-Djava.library.path=" + str(jar.parent / "DynamoDBLocal_lib"),
            "-jar",
            str(jar),
            "-inMemory",
            "-sharedDb",
            "-port",
            "8000",
            "-disableTelemetry",
        ],
        stdout=log,
        stderr=log,
        env=env,
    )
    try:
        for _ in range(100):
            if server.poll() is not None:
                raise SystemExit("Offline fixture startup failed.")
            try:
                with socket.create_connection(("127.0.0.1", 8000), timeout=0.1):
                    break
            except OSError:
                time.sleep(0.1)
        else:
            raise SystemExit("Offline fixture startup deadline.")
        for command, directory in [
            (
                [
                    "go",
                    "test",
                    "-race",
                    "-tags=integration",
                    "-count=1",
                    "-run",
                    "TestIntegration(Environment|AgentHTTPAuthenticationSeparation|AllSevenRoutesAndOwnership|WorkflowHTTPRevisionsOwnershipAndCleanup)",
                    "./internal/ownership",
                    "./execution",
                ],
                ROOT / "services/api",
            ),
            (["node", "scripts/project-environments-activation.test.mjs"], ROOT),
        ]:
            subprocess.run(command, cwd=directory, env=env, check=True)
    finally:
        server.terminate()
        server.wait(timeout=15)
