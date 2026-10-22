"""Inherited H-L clock/counts through the actual isolated proxy, local origin only."""

import json
from pathlib import Path
import tempfile
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
    attempts = [r for r in rows if r["event"] == "attempt"]
    assert rows[0]["continuationParent"]["counts"]["aws"] == 54
    assert (
        len(attempts) == 3
        and attempts[0]["counts"]["aws"] == 55
        and attempts[-1]["counts"]["aws"] == 57
    )
    assert rows[0]["seconds"] == 7200
    print(
        json.dumps(
            {
                "inherited54AwsCharges": True,
                "retryAndPagination": 3,
                "newCumulativeAwsCharges": 57,
                "liveAwsCalls": 0,
            }
        )
    )


if __name__ == "__main__":
    # Fixed published parent bytes/counts; only the LOCAL test epoch moves so
    # this transport regression remains runnable after the live window expires.
    # Production hl_resume.py and its deadline checks are never changed.
    with tempfile.TemporaryDirectory(prefix="kurier-local-resume-addon-") as d:
        addon = Path(d) / "local_addon.py"
        addon.write_text(
            "import sys,time,os\n"
            + "sys.path.insert(0,"
            + repr(str(harness.SCRIPTS))
            + ")\n"
            + "assert os.environ.get('KURIER_METER_LOCAL_ORIGIN')\nimport hl_resume\noriginal=hl_resume.parent_proof\ndef local_parent(path):\n    proof=original(path)\n    proof['firstAttemptAt']=time.time()-600\n    return proof\nhl_resume.parent_proof=local_parent\nfrom proxy import addons\n"
        )
        harness.main(mode="hl-resume", scenario=scenario, proxy_script=addon)
