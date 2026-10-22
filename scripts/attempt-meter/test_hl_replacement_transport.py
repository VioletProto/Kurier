"""Actual local isolated transport: authorized clock replacement, counters retained."""

import json
import test_transport as harness


def scenario(run, directory, _env):
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
    rows = [
        json.loads(x) for x in (directory / "ledger.jsonl").read_text().splitlines()
    ]
    attempts = [x for x in rows if x["event"] == "attempt"]
    assert rows[0]["replacementParent"]["counts"] == {
        "aws": 173,
        "api": 31,
        "poll": 0,
        "static": 9,
    }
    assert rows[0]["priorIdentitiesAndProjects"] == 7
    assert (
        len(attempts) == 3
        and attempts[0]["counts"]["aws"] == 174
        and attempts[-1]["counts"]["aws"] == 176
    )
    assert attempts[-1]["counts"]["api"] == 31 and attempts[-1]["counts"]["static"] == 9
    assert rows[0]["seconds"] == 7200
    print(
        json.dumps(
            {
                "replacementWindow": True,
                "priorAws173Api31Static9Retained": True,
                "retryAndPagination": 3,
                "newCumulativeAws176": True,
                "realAwsCalls": 0,
            }
        )
    )


if __name__ == "__main__":
    harness.main(mode="hl-replacement", scenario=scenario)
