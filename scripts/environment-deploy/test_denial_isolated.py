"""Real preview entrypoint/SST/provider against synthetic loopback responses."""

import http.server
import importlib.util
import json
import tempfile
import threading
from pathlib import Path

from common import PRIOR, Held
from driver import CHILD_CONTEXT, SST, mounts, phase_command
from isolation import launch
from wire import Policy

spec = importlib.util.spec_from_file_location(
    "environment_transport_fixture", Path(__file__).with_name("test_transport.py")
)
fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture)
AUDIT, Origin, R, S, campaign, source = (
    fixture.AUDIT,
    fixture.Origin,
    fixture.R,
    fixture.S,
    fixture.campaign,
    fixture.source,
)


def main():
    origin = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Origin)
    threading.Thread(target=origin.serve_forever, daemon=True).start()
    try:
        with tempfile.TemporaryDirectory(prefix="kurier-policy-denial-") as directory:
            root = Path(directory)
            src = source(root / "source")
            # A minimal read of the same owned bucket drives the actual pinned
            # provider's bucket Read path. No current cloud state is imported.
            (src / "sst.config.ts").write_text(
                '/// <reference path="./.sst/platform/config.d.ts" />\n'
                'export default $config({app(){return {name:"kurier",home:"aws",providers:{aws:{package:"@pulumi/aws",version:"7.40.0",region:"us-east-2"}}}},async run(){'
                'const bucket=aws.s3.Bucket.get("DevelopmentFrontendBucket",'
                + json.dumps(S["webBucket"])
                + ");return {bucket:bucket.id}}});"
            )
            c = campaign(root / "campaign", "collect", maximum=450)
            try:
                context = c.directory / "collect-diff-context.json"
                context.write_text(json.dumps({"sst": str(SST)}))
                context.chmod(0o600)
                output = root / "preview-child"
                result = launch(
                    src,
                    output,
                    phase_command("diff", CHILD_CONTEXT),
                    {
                        "AWS_ACCESS_KEY_ID": "ASIA0000000000000000",
                        "AWS_SECRET_ACCESS_KEY": "synthetic",
                        "AWS_SESSION_TOKEN": "synthetic",
                    },
                    campaign=c,
                    phase="collect",
                    policy_context={"fixtureOnly": True},
                    fixture_origin=origin.server_port,
                    extra_ro=mounts(context),
                )
                assert result["exitCode"] != 0 and c.get("stopped")
                # Read privately; publish only the operation/category conclusion.
                raw = (output / "work/sst/log/pulumi.log").read_text()
                assert "operation error S3: GetBucketPolicy" in raw and "599" in raw
                assert S["webBucket"] in raw
                diagnostic = json.loads((output / "denial.json").read_bytes())
                assert diagnostic == {
                    "operation": "s3/GetBucketPolicy",
                    "condition": "s3-read-scope-held",
                    "admitted": False,
                    "beforeForward": True,
                }
                assert (output / "denial.json").stat().st_mode & 0o777 == 0o600
                assert not any(
                    method in {"PUT", "DELETE"} or "policy" in target
                    for method, target in AUDIT
                )
                policy = Policy(S, R, {}, phase="collect")
                try:
                    policy.request(
                        "GET",
                        S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                        "/?policy",
                        {},
                        b"",
                    )
                except Held as error:
                    assert str(error) == "s3-read-scope-held"
                else:
                    raise AssertionError("GetBucketPolicy must remain denied")
                assert not c.db.execute(
                    "SELECT COUNT(*) FROM attempts WHERE write=1 OR outcome=0"
                ).fetchone()[0]
                assert c.db.execute("SELECT COUNT(*) FROM attempts").fetchone()[
                    0
                ] == len(AUDIT)
                print(
                    json.dumps(
                        {
                            "case": "actual-preview-entrypoint-owned-bucket-policy-denial",
                            "fixtureOnly": True,
                            "awsCalls": 0,
                            "priorUsageUnchanged": PRIOR,
                            "operation": "s3/GetBucketPolicy",
                            "condition": "s3-read-scope-held",
                            "deniedBeforeForward": True,
                            "newReadPermissionGranted": False,
                            "fixtureAttempts": len(AUDIT),
                        }
                    )
                )
            finally:
                c.close()
    finally:
        origin.shutdown()


if __name__ == "__main__":
    main()
