"""Real local transport proof: verification profile cannot enable writes."""

import json
import test_transport as harness


def scenario(run, directory, _env):
    (directory / "control.json").write_text(
        json.dumps(
            {
                "writeCategories": ["aws", "api"],
                "awsWriteOperations": ["TransactWriteItems"],
            }
        )
    )
    run(
        [
            "aws",
            "dynamodb",
            "scan",
            "--table-name",
            "fixture",
            "--page-size",
            "1",
            "--region",
            "us-east-2",
            "--output",
            "json",
        ]
    )
    actual = "const{awsProcess}=await import('./scripts/verify-read-only-mcp-deployment.mjs');const r=awsProcess(['dynamodb','query','--no-paginate'],{TableName:'fixture',KeyConditionExpression:'PK = :p',ExpressionAttributeValues:{':p':{S:'local'}}},{deadlineMs:Date.now()+10000.816});if(r.status!==0)throw Error('actual collector transport failed');"
    run(["node", "--input-type=module", "-e", actual])
    events = [
        json.loads(line)
        for line in (directory / "ledger.jsonl").read_text().splitlines()
    ]
    assert events[0]["limits"]["aws"] == 100 and events[0]["seconds"] == 600
    assert events[0]["priorSteps"] == 18 and events[0]["priorSends"] == 16
    before = [e for e in events if e["event"] == "attempt"]
    assert len(before) == 4
    denied = "const r=await fetch('https://dynamodb.us-east-2.amazonaws.com/',{method:'POST',headers:{'X-Amz-Target':'DynamoDB_20120810.TransactWriteItems','Content-Type':'application/x-amz-json-1.0'},body:'{}'});if(r.status!==599)throw Error('verification allowed a write');"
    run(["node", "--input-type=module", "-e", denied])
    events = [
        json.loads(line)
        for line in (directory / "ledger.jsonl").read_text().splitlines()
    ]
    assert len([e for e in events if e["event"] == "attempt"]) == 4
    held = [e for e in events if e["event"] == "held-request"][-1]
    assert (
        held["method"] == "POST"
        and held["category"] == "aws"
        and held["operation"] == "TransactWriteItems"
    )
    assert harness.counts["cli"] == 4 and harness.counts["api-write"] == 0
    print(
        json.dumps(
            {
                "verificationTransport": True,
                "actualCollectorAdapterAndIntegerTimeout": True,
                "readAttempts": 4,
                "retryAndPaginationMetered": True,
                "awsCap": 100,
                "seconds": 600,
                "resourceWriteDeniedDespitePhaseFile": True,
                "liveAwsCalls": 0,
            }
        )
    )


if __name__ == "__main__":
    harness.main(mode="verify", scenario=scenario)
