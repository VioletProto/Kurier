"""Local-only transport matrix. Run under the outer network namespace in test.sh."""

import collections
import hashlib
import http.server
import json
import socket
import subprocess
import tempfile
import threading
import time
from pathlib import Path
from unittest.mock import patch
from urllib.parse import parse_qs

import run as runner
from handoff import SharedBudget, begin, seal
from private_diagnostics import FILENAME
from run import PRIOR, ROOT, SST, launch, valid_parent

FAKE = {
    "AWS_ACCESS_KEY_ID": "localfixture",
    "AWS_SECRET_ACCESS_KEY": "local-private-canary",
    "AWS_SESSION_TOKEN": "localfixture",
}
S3 = "https://sst-state-moshrdrrbeba.s3.us-east-2.amazonaws.com"
COUNTS = collections.Counter()
AUDIT = []


class Origin(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def reply(self, status, body, ctype="application/json", headers=None):
        b = body.encode()
        self.send_response(status)
        self.send_header("Content-Length", str(len(b)))
        self.send_header("Content-Type", ctype)
        for k, v in (headers or {}).items():
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(b)

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        op = (
            self.headers.get("X-Amz-Target", "").split(".")[-1]
            or parse_qs(body.decode()).get("Action", [""])[0]
        )
        agent = self.headers.get("User-Agent", "")
        AUDIT.append(("POST", op, agent))
        if op == "GetCallerIdentity":
            return self.reply(
                200,
                '<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>arn:aws:iam::747336059622:user/fixture</Arn><UserId>fixture</UserId><Account>747336059622</Account></GetCallerIdentityResult><ResponseMetadata><RequestId>fixture</RequestId></ResponseMetadata></GetCallerIdentityResponse>',
                "text/xml",
            )
        if op == "GetParameter":
            name = json.loads(body)["Name"]
            value = (
                json.dumps(
                    {
                        "version": 5,
                        "asset": "sst-asset-moshrdrrbeba",
                        "state": "sst-state-moshrdrrbeba",
                        "assetEcrRegistryId": "747336059622",
                        "assetEcrUrl": "747336059622.dkr.ecr.us-east-2.amazonaws.com/sst-asset",
                        "appsyncHttp": "",
                        "appsyncRealtime": "",
                    }
                )
                if name == "/sst/bootstrap"
                else "local-fixture-passphrase"
            )
            return self.reply(
                200,
                json.dumps(
                    {
                        "Parameter": {
                            "Name": name,
                            "Type": "SecureString",
                            "Value": value,
                            "Version": 1,
                        }
                    }
                ),
            )
        if op == "ListTables":
            COUNTS["go"] += 1
            if COUNTS["go"] == 1:
                return self.reply(503, '{"__type":"InternalServerError"}')
            return self.reply(
                200,
                json.dumps(
                    {
                        "TableNames": [],
                        **(
                            {"LastEvaluatedTableName": "next"}
                            if "ExclusiveStartTableName" not in json.loads(body)
                            else {}
                        ),
                    }
                ),
            )
        return self.reply(400, '{"error":"unexpected fixture operation"}')

    def do_PUT(self):
        AUDIT.append(("WRITE", self.path, ""))
        COUNTS["mutation"] += 1
        self.reply(200, "{}")

    def do_DELETE(self):
        self.do_PUT()

    def do_HEAD(self):
        AUDIT.append(("HEAD", self.path, self.headers.get("User-Agent", "")))
        self.reply(200, "")

    def do_GET(self):
        AUDIT.append(
            ("GET", self.path.split("?")[0], self.headers.get("User-Agent", ""))
        )
        if self.path == "/redirect":
            COUNTS["redirect"] += 1
            return self.reply(302, "", headers={"Location": S3 + "/must-not-follow"})
        if self.path == "/must-not-follow":
            COUNTS["followed"] += 1
            return self.reply(200, "bad")
        if self.path == "/disconnect":
            COUNTS["disconnect"] += 1
            self.close_connection = True
            self.connection.shutdown(socket.SHUT_RDWR)
            return
        if "list-type=2" in self.path:
            COUNTS["node"] += 1
            if COUNTS["node"] == 1:
                return self.reply(
                    503, "<Error><Code>SlowDown</Code></Error>", "application/xml"
                )
            more = "continuation-token" not in self.path
            return self.reply(
                200,
                '<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>'
                + str(more).lower()
                + "</IsTruncated>"
                + (
                    "<NextContinuationToken>next</NextContinuationToken>"
                    if more
                    else ""
                )
                + "</ListBucketResult>",
                "application/xml",
            )
        if "/app/" in self.path or "/secret/" in self.path:
            return self.reply(
                404, "<Error><Code>NoSuchKey</Code></Error>", "application/xml"
            )
        COUNTS["get"] += 1
        return self.reply(200, "{}")


def source_at(path):
    path.mkdir()
    (path / ".sst").mkdir()
    (path / ".build").mkdir()
    (path / "apps/web/node_modules").mkdir(parents=True)
    (path / "apps/web/dist").mkdir()
    (path / "node_modules").symlink_to(ROOT / "node_modules")
    (path / "package.json").write_text(
        '{"type":"module","name":"local-meter-fixture","private":true}'
    )
    (
        path / "sst.config.ts"
    ).write_text("""/// <reference path="./.sst/platform/config.d.ts" />
export default $config({app(){return {name:"kurier",home:"aws",providers:{aws:{package:"@pulumi/aws",version:"7.40.0",region:"us-east-2"}}}},async run(){const identity=aws.getCallerIdentityOutput({});return {account:identity.accountId}}});
""")
    return path


def main():
    # Outer namespace has no external network route. A mistake in test-origin
    # rewriting still cannot contact AWS. No real credentials are read here.
    with socket.socket() as sock:
        sock.settimeout(0.2)
        assert sock.connect_ex(("1.1.1.1", 443)) != 0
    before = valid_parent()
    before_hashes = {p["name"]: p["zipSha256"] for p in before["packages"]}
    origin = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Origin)
    threading.Thread(target=origin.serve_forever, daemon=True).start()
    results = []
    try:
        with tempfile.TemporaryDirectory(prefix="kurier-preview-fixture-") as t:
            d = Path(t)
            source = source_at(d / "source")
            host_socket = socket.socket(socket.AF_UNIX)
            host_socket.bind(str(d / "host-only.sock"))
            host_socket.listen(1)
            gofile = d / "go-client.go"
            gofile.write_text("""package main
import("context";"github.com/aws/aws-sdk-go-v2/config";"github.com/aws/aws-sdk-go-v2/service/dynamodb")
func main(){ctx:=context.Background();c,e:=config.LoadDefaultConfig(ctx,config.WithRegion("us-east-2"),config.WithRetryMaxAttempts(3));if e!=nil{panic(e)};p:=dynamodb.NewListTablesPaginator(dynamodb.NewFromConfig(c),&dynamodb.ListTablesInput{});for p.HasMorePages(){if _,e=p.NextPage(ctx);e!=nil{panic(e)}}}
""")
            subprocess.run(
                ["go", "build", "-o", str(source / "go-client"), str(gofile)],
                cwd=ROOT / "services/api",
                check=True,
                capture_output=True,
            )
            node = source / "node-client.mjs"
            node.write_text(
                "const {S3Client,paginateListObjectsV2}=await import('"
                + str(
                    ROOT
                    / ".sst/platform/node_modules/@aws-sdk/client-s3/dist-cjs/index.js"
                )
                + "');const c=new S3Client({region:'us-east-2',maxAttempts:3});for await(const p of paginateListObjectsV2({client:c},{Bucket:'sst-state-moshrdrrbeba'})){}"
            )
            parent = source / "parent.py"
            parent.write_text("""import subprocess,sys,socket,os,errno,json
s=socket.socket();s.settimeout(.2);assert s.connect_ex(('1.1.1.1',443))==errno.ENETUNREACH
assert not os.path.exists('/run/docker.sock') and not os.path.exists(os.path.expanduser('~/.aws'))
assert not os.path.exists(sys.argv[1]+'/attempts.jsonl')
s6=socket.socket(socket.AF_INET6);s6.settimeout(.2);assert s6.connect_ex(('2606:4700:4700::1111',443))!=0
try:
 u=socket.socket(socket.AF_UNIX);u.connect(sys.argv[1]+'/../host-only.sock');raise AssertionError('host socket visible')
except (FileNotFoundError,ConnectionRefusedError):pass
assert subprocess.run(['unshare','-Urn','true'],capture_output=True).returncode!=0
for command in [[sys.argv[2]+'/go-client'],['node',sys.argv[2]+'/node-client.mjs'],[sys.argv[3],'diff','--stage','dev-api','--json']]:
 r=subprocess.run(command,capture_output=True)
 if r.returncode:sys.stdout.buffer.write(r.stdout);sys.stderr.buffer.write(r.stderr);sys.exit(r.returncode)
""")
            r = launch(
                source,
                d / "shared",
                ["python3", str(parent), str(d / "shared"), str(source), str(SST)],
                FAKE,
                fixture_origin=origin.server_port,
            )
            if r["exitCode"] != 0:
                print((d / "shared/private-output.txt").read_text()[-8000:])
                print(
                    "meterErrors", (d / "shared/meter-errors.txt").read_text()[-2000:]
                )
                print(
                    "SSTlog",
                    (d / "shared/work/sst/log/sst.log").read_text()[-6000:]
                    if (d / "shared/work/sst/log/sst.log").exists()
                    else "none",
                )
                print("rows", r["rows"])
                raise AssertionError("shared SST/Pulumi/SDK fixture failed")
            attempts = [x for x in r["rows"] if x["event"] == "attempt"]
            assert COUNTS["go"] == 3 and COUNTS["node"] == 3
            assert len(attempts) == len(AUDIT)
            assert any(x["protocol"] == "HTTP/2.0" for x in attempts)
            assert not any(x["event"] == "stop" for x in r["rows"])
            assert [x["count"] for x in attempts] == list(
                range(PRIOR + 1, PRIOR + 1 + len(attempts))
            )
            assert any("pulumi" in a[2].lower() for a in AUDIT), AUDIT
            assert any(x["operation"] == "GetParameter" for x in attempts)
            assert (
                "local-private-canary" not in (d / "shared/attempts.jsonl").read_text()
            )
            results.append(
                {
                    "case": "shared-real-SST-Pulumi-Go-Node",
                    "attempts": len(attempts),
                    "goRetryPages": 3,
                    "nodeRetryPages": 3,
                    "exitCode": r["exitCode"],
                    "namespaceBypassHeld": True,
                    "httpVersions": sorted({x["protocol"] for x in attempts}),
                    "realSST": True,
                    "realPulumiAWSProvider": True,
                    "attemptCounts": dict(
                        collections.Counter(
                            x["service"] + ":" + x["operation"] for x in attempts
                        )
                    ),
                }
            )
            for name, script, limit, seconds, failure in [
                (
                    "write",
                    "await fetch('https://ssm.us-east-2.amazonaws.com/',{method:'POST',headers:{'X-Amz-Target':'AmazonSSM.PutParameter','Content-Type':'application/x-amz-json-1.1'},body:'{\"Name\":\"/sst/passphrase/kurier/dev-api\",\"Value\":\"local-private-canary\"}'});",
                    500,
                    3600,
                    False,
                ),
                (
                    "state-write",
                    "await fetch('"
                    + S3
                    + "/app/kurier/dev-api.json',{method:'PUT',body:'{}'});",
                    500,
                    3600,
                    False,
                ),
                (
                    "transaction-write",
                    "await fetch('https://dynamodb.us-east-2.amazonaws.com/',{method:'POST',headers:{'X-Amz-Target':'DynamoDB_20120810.TransactWriteItems','Content-Type':'application/x-amz-json-1.0'},body:'{}'});",
                    500,
                    3600,
                    False,
                ),
                (
                    "limit",
                    "for(let i=0;i<336;i++)await fetch('" + S3 + "/read');",
                    500,
                    3600,
                    False,
                ),
                (
                    "deadline",
                    "const{spawn}=await import('node:child_process');spawn('node',['-e',\"const fs=require('node:fs');setInterval(()=>fs.appendFileSync('/work/heartbeat','x'),50)\"],{stdio:'ignore'});await new Promise(r=>setTimeout(r,3000));",
                    500,
                    1,
                    False,
                ),
                (
                    "concurrent",
                    "const{spawn}=await import('node:child_process');const code=\"for(let i=0;i<6;i++)try{await fetch('"
                    + S3
                    + "/read')}catch{}\";await Promise.all([0,1].map(()=>new Promise(r=>spawn('node',['--input-type=module','-e',code]).on('exit',r))));",
                    PRIOR + 4,
                    3600,
                    False,
                ),
                ("redirect", "await fetch('" + S3 + "/redirect');", 500, 3600, False),
                (
                    "disconnect",
                    "try{await fetch('" + S3 + "/disconnect')}catch{};",
                    500,
                    3600,
                    False,
                ),
                (
                    "persistence",
                    "try{await fetch('" + S3 + "/read')}catch{};",
                    500,
                    3600,
                    True,
                ),
            ]:
                old_audit = len(AUDIT)
                old_get = COUNTS["get"]
                rr = launch(
                    source,
                    d / name,
                    ["node", "--input-type=module", "-e", script],
                    FAKE,
                    fixture_origin=origin.server_port,
                    budget={"maximum": limit, "seconds": seconds},
                    fixture_failure=failure,
                )
                aa = [x for x in rr["rows"] if x["event"] == "attempt"]
                stops = [x for x in rr["rows"] if x["event"] == "stop"]
                if name in {"write", "state-write", "transaction-write"}:
                    assert not aa and len(AUDIT) == old_audit and stops
                elif name == "limit":
                    assert (
                        len(aa) == 500 - PRIOR
                        and aa[-1]["count"] == 500
                        and COUNTS["get"] - old_get == 500 - PRIOR
                        and stops
                    )
                elif name == "deadline":
                    assert not aa and stops
                    pulse = d / name / "work/heartbeat"
                    assert pulse.exists() and pulse.stat().st_size > 0
                    size = pulse.stat().st_size
                    time.sleep(0.2)
                    assert (pulse.stat().st_size if pulse.exists() else 0) == size
                elif name == "concurrent":
                    assert len(aa) == 4 and COUNTS["get"] - old_get == 4 and stops
                elif name == "redirect":
                    assert len(aa) == 1 and COUNTS["followed"] == 0 and stops
                elif name == "disconnect":
                    assert len(aa) == 1 and stops
                elif name == "persistence":
                    assert not aa and len(AUDIT) == old_audit and rr["exitCode"] != 0
                results.append(
                    {
                        "case": name,
                        "attempts": len(aa),
                        "stopped": bool(stops) or rr["exitCode"] != 0,
                        "descendantShutdownVerified": name == "deadline",
                    }
                )
        # Each new endpoint scenario uses the production sandbox, but all
        # supervisor/proxy/origin networking remains in the outer offline namespace.
        for name, url, expected_host, expected_port, expected_phase in [
            (
                "denied-host",
                "https://sts.us-east-1.amazonaws.com/private-local-private-canary?token=local-private-canary",
                "sts.us-east-1.amazonaws.com",
                443,
                "connect",
            ),
            (
                "secret-host-label",
                "https://local-private-canary.example.invalid/private?token=local-private-canary",
                "<redacted-host>",
                443,
                "connect",
            ),
            (
                "private-capture-exists",
                "https://local-private-canary.example.invalid/private?token=local-private-canary",
                "<redacted-host>",
                443,
                "connect",
            ),
            (
                "denied-port",
                "https://sts.us-east-2.amazonaws.com:8443/private?token=local-private-canary",
                "sts.us-east-2.amazonaws.com",
                8443,
                "connect",
            ),
            (
                "denied-http",
                "http://sts.us-east-2.amazonaws.com/private?token=local-private-canary",
                "sts.us-east-2.amazonaws.com",
                80,
                "request",
            ),
        ]:
            code = """const{spawn}=await import('node:child_process');const fs=await import('node:fs');
const grand=\"const fs=require('node:fs');setInterval(()=>fs.appendFileSync('/work/grand-pulse','x'),40)\";
const child=\"const{spawn}=require('node:child_process');const fs=require('node:fs');spawn('node',['-e',\"+JSON.stringify(grand)+\"],{stdio:'ignore'});setInterval(()=>fs.appendFileSync('/work/child-pulse','x'),40)\";
spawn('node',['-e',child],{stdio:'ignore'});setInterval(()=>fs.appendFileSync('/work/root-pulse','x'),40);
for(let i=0;i<100&&!['root','child','grand'].every(x=>fs.existsSync('/work/'+x+'-pulse'));i++)await new Promise(r=>setTimeout(r,20));
if(!['root','child','grand'].every(x=>fs.existsSync('/work/'+x+'-pulse')))throw Error('fixture descendants did not start');
try{await fetch(TARGET,{method:'POST',headers:{Authorization:'Bearer local-private-canary',Cookie:'session=local-private-canary','X-Private':'local-private-canary'},body:'local-private-canary'})}catch{}
await new Promise(r=>setTimeout(r,3000));""".replace("TARGET", json.dumps(url))
            old_audit = len(AUDIT)
            real_write = runner.write

            def configure(path, value, real_write=real_write, name=name):
                real_write(path, value)
                if name == "private-capture-exists" and path.name == "config.json":
                    real_write(path.parent / FILENAME, {"fixtureOriginal": True})

            with patch.object(runner, "write", configure):
                rr = launch(
                    source,
                    d / name,
                    ["node", "--input-type=module", "-e", code],
                    FAKE,
                    fixture_origin=origin.server_port,
                    private_denied_connect=name
                    in {"secret-host-label", "private-capture-exists"},
                )
            assert rr["exitCode"] != 0 and len(AUDIT) == old_audit
            assert not any(x["event"] == "attempt" for x in rr["rows"])
            denied = [x for x in rr["rows"] if x["event"] == "denied-endpoint"]
            assert len(denied) == 1
            record = denied[0]
            assert (
                record["hostname"] == expected_host
                and record["port"] == expected_port
                and record["phase"] == expected_phase
            )
            assert record["count"] == PRIOR
            if name == "private-capture-exists":
                assert rr["exitCode"] == 77
                assert json.loads((d / name / FILENAME).read_text()) == {
                    "fixtureOriginal": True
                }
            else:
                assert any(
                    x["event"] == "stop" and x["count"] == PRIOR for x in rr["rows"]
                )
            receipt = d / name / FILENAME
            if name == "secret-host-label":
                captured = json.loads(receipt.read_text())
                assert captured["hostname"] == "local-private-canary.example.invalid"
                assert captured["count"] == PRIOR and captured["port"] == 443
                assert receipt.stat().st_mode & 0o777 == 0o600
                assert receipt.stat().st_size <= 512
                assert set(captured) == {
                    "version",
                    "phase",
                    "count",
                    "port",
                    "hostnameStatus",
                    "hostname",
                }
                assert not (d / name / "work" / FILENAME).exists()
            elif name != "private-capture-exists":
                assert not receipt.exists()
            raw = (d / name / "attempts.jsonl").read_text()
            assert all(
                value not in raw
                for value in [
                    "local-private-canary",
                    "Authorization",
                    "Cookie",
                    "token=",
                    "/private",
                    "https://",
                    "http://",
                ]
            )
            pulses = [
                d / name / "work" / (x + "-pulse") for x in ["root", "child", "grand"]
            ]
            sizes = [p.stat().st_size for p in pulses]
            assert all(sizes)
            time.sleep(0.2)
            assert sizes == [p.stat().st_size for p in pulses]
            results.append(
                {
                    "case": name,
                    "attempts": 0,
                    "stopped": True,
                    "allDescendantsStopped": True,
                    "sanitizedDiagnostic": record,
                    "privateCaptureOptIn": name
                    in {"secret-host-label", "private-capture-exists"},
                    "privateCaptureFailureHeld": name == "private-capture-exists",
                    "privateHostnameOmittedFromPublicReceipt": True,
                }
            )
        # The real production proxy branch consumes a synthetic authentication
        # handoff. Outer --unshare-net and a denied request prohibit AWS traffic.
        shared = d / "shared-auth-window"
        window = begin(shared, ROOT)
        b = SharedBudget(shared / "authentication.jsonl", window, prior=PRIOR)
        for service, operation in [
            ("signin", "CreateOAuth2Token"),
            ("sts", "GetCallerIdentity"),
        ]:
            n = b.admit(service, operation, "HTTP/1.1")
            b.outcome(n, 200)
        b.record(event="authenticated", count=b.count, identityVerified=True)
        b.close()
        receipt = seal(shared, ROOT)
        old_audit = len(AUDIT)
        rr = launch(
            source,
            shared / "provider",
            ["node", "--input-type=module", "-e", code],
            FAKE,
            handoff=receipt,
        )
        assert rr["exitCode"] != 0 and len(AUDIT) == old_audit
        assert (shared / "provider-claim.json").is_file()
        assert rr["rows"][0]["prior"] == PRIOR + 2
        assert rr["rows"][1]["window"]["deadline"] == window["deadline"]
        assert not any(x["event"] == "attempt" for x in rr["rows"])
        assert any(x["event"] == "stop" and x["count"] == PRIOR + 2 for x in rr["rows"])
        pulses = [
            shared / "provider/work" / (x + "-pulse")
            for x in ["root", "child", "grand"]
        ]
        sizes = [p.stat().st_size for p in pulses]
        time.sleep(0.2)
        assert all(sizes) and sizes == [p.stat().st_size for p in pulses]
        results.append(
            {
                "case": "production-branch-shared-handoff-denial",
                "attempts": 0,
                "syntheticAuthenticationAttempts": 2,
                "providerPrior": PRIOR + 2,
                "originalDeadlinePreserved": True,
                "singleUseClaim": True,
                "allDescendantsStopped": True,
            }
        )
        assert COUNTS["mutation"] == 0
        assert before_hashes == {
            p["name"]: p["zipSha256"] for p in valid_parent()["packages"]
        }
        host_socket.close()
        output = {
            "tooling": {
                "sstSha256": hashlib.sha256(SST.read_bytes()).hexdigest(),
                "pulumiAwsProviderSha256": hashlib.sha256(
                    (
                        Path.home()
                        / ".config/sst/plugins/resource-aws-v7.40.0/pulumi-resource-aws"
                    ).read_bytes()
                ).hexdigest(),
                "node": subprocess.check_output(
                    ["node", "--version"], text=True
                ).strip(),
                "go": subprocess.check_output(["go", "version"], text=True).strip(),
                "bwrap": subprocess.check_output(
                    ["bwrap", "--version"], text=True
                ).strip(),
            },
            "codeSha256": {
                p.name: hashlib.sha256(p.read_bytes()).hexdigest()
                for p in Path(__file__).parent.glob("*.py")
            },
            "fixtureOnly": True,
            "realCumulativeUsagePreserved": PRIOR,
            "awsCalls": 0,
            "releaseAndRollbackVerifiedUnchanged": True,
            "results": results,
        }
        path = ROOT / ".build/project-environments-preview-meter-tests.json"
        path.write_text(json.dumps(output, indent=2) + "\n")
        print(json.dumps(output))
    finally:
        origin.shutdown()
        origin.server_close()


if __name__ == "__main__":
    main()
