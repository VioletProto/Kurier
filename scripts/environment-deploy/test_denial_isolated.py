"""Real preview entrypoint/SST/provider against synthetic loopback responses."""

import http.server
import importlib.util
import json
import os
import tempfile
import threading
from pathlib import Path
from types import SimpleNamespace
from urllib.parse import parse_qs, unquote, urlsplit

from common import PRIOR, ROOT, canonical, private_write
from driver import CHILD_CONTEXT, HERE, SST, mounts, phase_command
from fixture_bucket_reads import fixture_response, validate_model
from isolation import launch
from wire import BUCKET_CONFIGURATION_READS

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


class ReadOrigin(Origin):
    def do_GET(self):
        parsed = urlsplit(self.path)
        if parsed.path == "/" and parse_qs(parsed.query, keep_blank_values=True) == {
            "policy": [""]
        }:
            AUDIT.append(("GET", "GetBucketPolicy"))
            return self.reply(
                404, "<Error><Code>NoSuchBucketPolicy</Code></Error>", "application/xml"
            )
        if parsed.path == "/" and parse_qs(parsed.query, keep_blank_values=True) == {
            "acl": [""]
        }:
            AUDIT.append(("GET", "GetBucketAcl"))
            return self.reply(
                200,
                '<AccessControlPolicy xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Owner><ID>'
                + "0" * 64
                + "</ID><DisplayName>fixture</DisplayName></Owner><AccessControlList/></AccessControlPolicy>",
                "application/xml",
            )
        response = fixture_response(
            SimpleNamespace(
                method="GET",
                host=S["webBucket"] + ".s3.us-east-2.amazonaws.com",
                path=self.path,
                raw_content=b"",
            ),
            S,
        )
        if response is not None:
            operation, _selector, status, body = response
            AUDIT.append(("GET", operation))
            return self.reply(status, body.decode(), "application/xml")
        from s3control_tags import PATH

        if parsed.path == PATH:
            AUDIT.append(("GET", "ListTagsForResource"))
            return self.reply(
                200,
                '<ListTagsForResourceResult xmlns="http://awss3control.amazonaws.com/doc/2018-08-20/"><Tags/></ListTagsForResourceResult>',
                "application/xml",
            )
        old = json.loads(
            (ROOT / "docs/development/project-environments-inventory.json").read_bytes()
        )["frontendRollback"]["manifest"]
        prepared = json.loads(
            (ROOT / "docs/development/project-environments-release.json").read_bytes()
        )
        if parsed.path == "/artifact.json":
            AUDIT.append(("GET", "frontend-marker"))
            return self.reply(200, json.dumps(old))
        key = unquote(parsed.path.lstrip("/"))
        if key in {x["path"] for x in old["files"]}:
            data = (
                ROOT / prepared["artifactDirectory"] / "rollback/frontend/app" / key
            ).read_bytes()
            AUDIT.append(("GET", "frontend-asset"))
            self.send_response(200)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        decoded = unquote(parsed.path)
        if "/keys/" in decoded:
            key = decoded.rsplit("/keys/", 1)[1]
            assert key in {
                S["namespace"] + ":/" + x["path"]
                for x in old["files"]
                if x["path"].startswith("assets/")
            }
            AUDIT.append(("GET", "baseline-kvs"))
            return self.reply(
                200,
                json.dumps({"Key": key, "Value": "s3", "ContentType": "text/plain"}),
            )
        if parsed.path.endswith("/distribution/" + S["distribution"]):
            AUDIT.append(("GET", "distribution"))
            return self.reply(
                200,
                '<Distribution xmlns="http://cloudfront.amazonaws.com/doc/2020-05-31/"><Status>Deployed</Status></Distribution>',
                "application/xml",
            )
        return super().do_GET()


def main():
    validate_model()
    origin = http.server.ThreadingHTTPServer(("127.0.0.1", 0), ReadOrigin)
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
                assert result["exitCode"] == 0 and not c.get("stopped")
                assert not (output / "denial.json").exists()
                assert not list(output.glob("observed-read-*.json"))
                required = {
                    "GetBucketPolicy",
                    "GetBucketAcl",
                    *[op for op, _ in BUCKET_CONFIGURATION_READS],
                }
                assert all(
                    AUDIT.count(("GET", operation)) == 1 for operation in required
                )
                admitted = {op for (op,) in c.db.execute("SELECT op FROM attempts")}
                assert {"s3/" + op for op in required} <= admitted
                assert not c.db.execute(
                    "SELECT COUNT(*) FROM attempts WHERE write=1 OR outcome=0"
                ).fetchone()[0]
                assert c.db.execute("SELECT COUNT(*) FROM attempts").fetchone()[
                    0
                ] == len(AUDIT)
                record = {
                    "case": "actual-preview-production-policy-approved-bucket-reads",
                    "fixtureOnly": True,
                    "awsCalls": 0,
                    "priorUsageUnchanged": PRIOR,
                    "previewExitCode": result["exitCode"],
                    "productionProxy": True,
                    "approvedReadsForwarded": sorted(required),
                    "pendingObservedReads": [],
                    "allForwardedAttemptsReconciled": True,
                    "fixtureAttempts": len(AUDIT),
                }
                private_write(
                    Path(os.environ["KURIER_TEST_PRIVATE_DIR"])
                    / "production-preview-result.json",
                    canonical(record),
                )
                print(json.dumps(record))
            finally:
                c.close()
        AUDIT.clear()
        with tempfile.TemporaryDirectory(
            prefix="kurier-unapproved-bucket-read-"
        ) as directory:
            root = Path(directory)
            src = source(root / "source")
            c = campaign(root / "campaign", "collect", maximum=450)
            try:
                output = root / "unexpected-bucket-read"
                result = launch(
                    src,
                    output,
                    [
                        "python3",
                        "-c",
                        "import boto3;boto3.client('sts',region_name='us-east-2').get_caller_identity();boto3.client('s3',region_name='us-east-2').get_bucket_tagging(Bucket="
                        + repr(S["webBucket"])
                        + ")",
                    ],
                    {
                        "AWS_ACCESS_KEY_ID": "ASIA0000000000000000",
                        "AWS_SECRET_ACCESS_KEY": "synthetic",
                        "AWS_SESSION_TOKEN": "synthetic",
                    },
                    campaign=c,
                    phase="collect",
                    policy_context={"fixtureOnly": True},
                    fixture_origin=origin.server_port,
                    extra_ro=mounts(),
                )
                assert result["exitCode"] != 0 and c.get("stopped")
                diagnostic = json.loads((output / "denial.json").read_bytes())
                assert diagnostic["beforeForward"] and not diagnostic["admitted"]
                assert AUDIT == [("POST", "GetCallerIdentity")]
                assert not c.db.execute(
                    "SELECT COUNT(*) FROM attempts WHERE write=1 OR outcome=0"
                ).fetchone()[0]
                assert c.db.execute("SELECT COUNT(*) FROM attempts").fetchone()[
                    0
                ] == len(AUDIT)
                print(
                    json.dumps(
                        {
                            "case": "production-policy-unapproved-bucket-read-held",
                            "awsCalls": 0,
                            "unexpectedReadDeniedBeforeForward": True,
                            "fixtureAttempts": len(AUDIT),
                        }
                    )
                )
            finally:
                c.close()
        AUDIT.clear()
        with tempfile.TemporaryDirectory(prefix="kurier-baseline-reads-") as directory:
            root = Path(directory)
            src = source(root / "source")
            c = campaign(root / "campaign", "collect", maximum=450)
            try:
                code = (
                    """const fs=await import('node:fs');const{spawnSync}=await import('node:child_process');const{aws,frontendReady}=await import('"""
                    + str(HERE / "collector.mjs")
                    + """');aws('sts','get-caller-identity');if(!frontendReady('collect'))throw Error('Baseline fixture failed');fs.writeFileSync('/work/baseline-proof.json',JSON.stringify({ready:true}));const bad=spawnSync('python3',['-c',"import boto3;boto3.client('s3',region_name='us-east-2').get_object(Bucket='"""
                    + S["webBucket"]
                    + """',Key='assets/unapproved.js')"],{stdio:'inherit'});if(bad.status!==0)process.exit(1);throw Error('Unexpected read forwarded');"""
                )
                output = root / "frontend-child"
                result = launch(
                    src,
                    output,
                    ["node", "--input-type=module", "-e", code],
                    {
                        "AWS_ACCESS_KEY_ID": "ASIA0000000000000000",
                        "AWS_SECRET_ACCESS_KEY": "synthetic",
                        "AWS_SESSION_TOKEN": "synthetic",
                    },
                    campaign=c,
                    phase="collect",
                    policy_context={"fixtureOnly": True},
                    fixture_origin=origin.server_port,
                    extra_ro=mounts(),
                )
                assert result["exitCode"] != 0 and c.get("stopped")
                assert json.loads((output / "work/baseline-proof.json").read_bytes())[
                    "ready"
                ]
                assert AUDIT.count(("GET", "baseline-kvs")) == 2
                assert not any("unapproved" in op for _, op in AUDIT)
                assert not c.db.execute(
                    "SELECT COUNT(*) FROM attempts WHERE write=1 OR outcome=0"
                ).fetchone()[0]
                assert c.db.execute("SELECT COUNT(*) FROM attempts").fetchone()[
                    0
                ] == len(AUDIT)
                print(
                    json.dumps(
                        {
                            "case": "actual-old-frontend-check-through-sdk-and-gate",
                            "fixtureOnly": True,
                            "awsCalls": 0,
                            "priorUsageUnchanged": PRIOR,
                            "baselineReady": True,
                            "oldAssetAndKvsReadsPassed": True,
                            "unexpectedObjectDeniedBeforeForward": True,
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
