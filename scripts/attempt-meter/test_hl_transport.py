"""Local real-meter proof of H-L resource-write and single-identity fences."""

import json
import test_transport as harness


class LocalCreation(harness.Origin):
    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", "0")))
        harness.counts["local-create"] += 1
        self.reply(200, "{}")


def scenario(run, directory, _env):
    (directory / "control.json").write_text(
        json.dumps(
            {
                "writeCategories": ["aws"],
                "awsWriteOperations": ["AdminCreateUser", "TransactWriteItems"],
            }
        )
    )
    script = "const r=await fetch('https://cognito-idp.us-east-2.amazonaws.com/',{method:'POST',headers:{'X-Amz-Target':'AWSCognitoIdentityProviderService.AdminCreateUser'},body:'{}'});if(r.status!==STATUS)throw Error('single identity fence');"
    run(["node", "--input-type=module", "-e", script.replace("STATUS", "200")])
    run(["node", "--input-type=module", "-e", script.replace("STATUS", "599")])
    events = [
        json.loads(x) for x in (directory / "ledger.jsonl").read_text().splitlines()
    ]
    assert (
        events[0]["priorSteps"] == 18
        and events[0]["priorSends"] == 16
        and events[0]["priorIdentitiesAndProjects"] == 6
    )
    assert len([e for e in events if e["event"] == "attempt"]) == 1
    assert events[-1]["reason"] == "H-L-single-disposable-identity"
    assert harness.counts["local-create"] == 1
    print(
        json.dumps(
            {
                "H-L-single-identity-fence": True,
                "prior18steps16sends": True,
                "liveAwsCalls": 0,
            }
        )
    )


def forbidden(run, directory, _env):
    (directory / "control.json").write_text(
        json.dumps(
            {"writeCategories": ["aws"], "awsWriteOperations": ["TransactWriteItems"]}
        )
    )
    run(
        [
            "node",
            "--input-type=module",
            "-e",
            "const r=await fetch('https://dynamodb.us-east-2.amazonaws.com/',{method:'POST',headers:{'X-Amz-Target':'DynamoDB_20120810.TransactWriteItems'},body:'{}'});if(r.status!==599)throw Error('stage write allowed')",
        ]
    )
    events = [
        json.loads(x) for x in (directory / "ledger.jsonl").read_text().splitlines()
    ]
    assert not any(e["event"] == "attempt" for e in events)
    assert events[-1]["reason"] == "H-L-resource-write-forbidden"
    print(
        json.dumps(
            {"H-L-stage-write-denied-despite-phase-file": True, "liveAwsCalls": 0}
        )
    )


if __name__ == "__main__":
    harness.Origin = LocalCreation
    harness.main(mode="hl", scenario=scenario)
    harness.main(mode="hl", scenario=forbidden)
