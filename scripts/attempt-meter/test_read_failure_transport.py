"""Real proxy/coordinator failure handoff, with only a controlled LOCAL origin."""

import json
import socket
import test_transport as harness


class FailingReadOrigin(harness.Origin):
    def do_GET(self):
        if self.path == "/api/v1/fail":
            harness.counts["failed-read"] += 1
            self.close_connection = True
            self.connection.shutdown(socket.SHUT_RDWR)
            return
        return super().do_GET()


def scenario(run, directory, env):
    # Even a write-enabled phase must not override a latched read failure.
    (directory / "control.json").write_text(
        json.dumps({"writeCategories": ["api", "aws"], "awsWriteOperations": ["PUT"]})
    )
    env["KURIER_LOCAL_COORDINATOR_TEST"] = str(directory)
    run(["node", str(harness.ROOT / "scripts/fixtures/workflow-read-failure.mjs")])
    rows = [
        json.loads(line)
        for line in (directory / "ledger.jsonl").read_text().splitlines()
    ]
    attempts = [r for r in rows if r["event"] == "attempt"]
    assert len(attempts) == 1
    assert attempts[0]["category"] == "api" and not attempts[0]["write"]
    assert any(r.get("reason") == "read-transport-failure" for r in rows)
    assert harness.counts["failed-read"] == 1
    assert harness.counts["api-write"] == harness.counts["api-delete"] == 0
    assert "local-private-canary" not in (directory / "ledger.jsonl").read_text()
    print(
        json.dumps(
            {
                "readFailureStopsProxyAndCoordinator": True,
                "chargedAttempts": 1,
                "forwardedWrites": 0,
                "replays": 0,
                "liveAwsCalls": 0,
                "targetSends": 0,
            }
        )
    )


if __name__ == "__main__":
    harness.Origin = FailingReadOrigin
    harness.main(scenario=scenario)
