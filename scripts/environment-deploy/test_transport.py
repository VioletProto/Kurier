"""Real namespace, sole relay and host gate; run ONLY under outer --unshare-net."""

import hashlib
import http.server
import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
sys.path.insert(0, str(HERE.parent / "preview-meter"))
from common import PRIOR, ROOT, Campaign
from driver import check, mounts
from isolation import launch
from uploads import payload

S = json.loads(
    (ROOT / "docs/development/project-environments-execution-scope.json").read_text()
)
R = json.loads(
    (ROOT / "docs/development/project-environments-adopted-release.json").read_text()
)
P = json.loads((HERE / "pins.json").read_text())
AUDIT = []
CLOSED = {
    "PK": {"S": "STAGE#dev-api"},
    "SK": {"S": "META"},
    "kind": {"S": "stage"},
    "schemaVersion": {"N": "1"},
    "version": {"N": "31"},
    "recoveryGeneration": {"S": "offline-generation"},
    "state": {"S": "workflow-migration"},
    "savedRequestsSchemaVersion": {"N": "8"},
}
for k in [
    "protectedSecrets",
    "localAgentExecutions",
    "workflowAssertions",
    "sequentialWorkflows",
    "cloudExecutions",
    "executionInputs",
    "evidencePinning",
    "mcpReadOnly",
]:
    CLOSED[k + "SchemaVersion"] = {"N": "1"}


class Origin(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def reply(self, status, body, kind="application/json"):
        data = body.encode()
        self.send_response(status)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        op = self.headers.get("X-Amz-Target", "").split(".")[-1]
        if b"GetCallerIdentity" in body:
            AUDIT.append(("POST", "GetCallerIdentity"))
            return self.reply(
                200,
                '<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>'
                + S["identityArn"]
                + "</Arn><Account>"
                + S["account"]
                + "</Account><UserId>fixture</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>",
                "text/xml",
            )
        AUDIT.append(("POST", op))
        if op == "GetParameter":
            name = json.loads(body)["Name"]
            value = (
                json.dumps(
                    {
                        "version": 5,
                        "asset": S["assetBucket"],
                        "state": S["stateBucket"],
                        "assetEcrRegistryId": S["account"],
                        "assetEcrUrl": S["account"]
                        + ".dkr.ecr.us-east-2.amazonaws.com/sst-asset",
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
        if op == "GetItem":
            return self.reply(200, json.dumps({"Item": CLOSED}))
        return self.reply(400, "{}")

    def do_GET(self):
        AUDIT.append(("GET", self.path))
        self.reply(404, "<Error><Code>NoSuchKey</Code></Error>", "application/xml")

    def do_HEAD(self):
        AUDIT.append(("HEAD", self.path))
        self.reply(200, "")

    def do_PUT(self):
        if self.headers.get("Transfer-Encoding", "").lower() == "chunked":
            chunks = []
            while True:
                size = int(self.rfile.readline().split(b";", 1)[0], 16)
                if size == 0:
                    while self.rfile.readline() != b"\r\n":
                        pass
                    break
                chunks.append(self.rfile.read(size))
                assert self.rfile.read(2) == b"\r\n"
            data = b"".join(chunks)
        else:
            data = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        decoded = payload(data, {k.lower(): v for k, v in self.headers.items()})
        AUDIT.append(("PUT", hashlib.sha256(decoded).hexdigest()))
        self.reply(200, "")


def source(path):
    path.mkdir()
    (path / ".sst").mkdir()
    (path / ".build").mkdir()
    (path / "apps/web/node_modules").mkdir(parents=True)
    (path / "apps/web/dist").mkdir()
    (path / "node_modules").symlink_to(ROOT / "node_modules")
    (path / "package.json").write_text('{"type":"module","private":true}')
    (path / "sst.config.ts").write_text(
        '/// <reference path="./.sst/platform/config.d.ts" />\nexport default $config({app(){return {name:"kurier",home:"aws",providers:{aws:{package:"@pulumi/aws",version:"7.40.0",region:"us-east-2"}}}},async run(){const identity=aws.getCallerIdentityOutput({});return {account:identity.accountId}}});'
    )
    return path


def campaign(path, phase, maximum=PRIOR + 26, seconds=3300):
    c = Campaign(
        path,
        create={
            "prior": PRIOR,
            "maximum": maximum,
            "seconds": seconds,
            "manifestSha256": P["manifest"],
            "scopeSha256": P["scope"],
            "reviewCommit": "a" * 40,
            "authorizationSha256": "b" * 64,
        },
    )
    c.put("nextPhase", {"collect": 0, "deploy": 2}[phase])
    c.begin(phase)
    return c


def main():
    # Namespace identity differs from the host and has only loopback routes.
    assert not any(
        line.split()[0] != "lo" and line.split()[1] == "00000000"
        for line in Path("/proc/net/route").read_text().splitlines()[1:]
    )
    check()
    origin = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Origin)
    threading.Thread(target=origin.serve_forever, daemon=True).start()
    results = []
    try:
        with tempfile.TemporaryDirectory(prefix="kurier-execution-fixtures-") as tmp:
            d = Path(tmp)
            src = source(d / "source")
            # Actual Go child: HTTPS proxy + system CA path are honored by Go.
            go = d / "wire.go"
            go.write_text(
                'package main\nimport("net/http";"strings";"io")\nfunc main(){q,_:=http.NewRequest("POST","https://ssm.us-east-2.amazonaws.com/",strings.NewReader(`{"Name":"/sst/bootstrap"}`));q.Header.Set("X-Amz-Target","AmazonSSM.GetParameter");r,e:=http.DefaultClient.Do(q);if e!=nil{panic(e)};io.Copy(io.Discard,r.Body);r.Body.Close()}'
            )
            subprocess.run(
                ["go", "build", "-o", str(src / "go-child"), str(go)],
                check=True,
                capture_output=True,
            )
            py = src / "client.py"
            py.write_text(
                '''import boto3,sys
from botocore.config import Config
c=boto3.client('sts',region_name='us-east-2',config=Config(retries={'total_max_attempts':1}));c.get_caller_identity()
if len(sys.argv)>1:
 d=boto3.client('dynamodb',region_name='us-east-2',config=Config(retries={'total_max_attempts':1}));d.get_item(TableName="'''
                + S["controlTable"]
                + '''",Key={'PK':{'S':'STAGE#dev-api'},'SK':{'S':'META'}},ConsistentRead=True)
 s=boto3.client('s3',region_name='us-east-2',config=Config(retries={'total_max_attempts':1}));data=open(sys.argv[1],'rb').read()
 if len(sys.argv)>2:data=bytes([data[0]^1])+data[1:]
 s.put_object(Bucket="'''
                + S["assetBucket"]
                + """",Key="assets/UsersProjectsApi-code-"""
                + R["packages"][0]["zipSha256"]
                + """.zip",Body=data)
"""
            )
            (src / "accepted.zip").write_bytes(
                (
                    ROOT / R["artifactDirectory"] / "UsersProjectsApi/code.zip"
                ).read_bytes()
            )
            creds = {
                "AWS_ACCESS_KEY_ID": "localfixture",
                "AWS_SECRET_ACCESS_KEY": "offline-private-canary",
                "AWS_SESSION_TOKEN": "offline-session",
            }
            # Real SST and its pinned Pulumi AWS provider run beside Go/Node/Python.
            c = campaign(d / "shared", "collect")
            old = len(AUDIT)
            node = f"""const{{spawn}}=await import('node:child_process');for(const [cmd,args] of [['go-child',[]],['python3',['client.py']],['{ROOT}/node_modules/sst-linux-x64/bin/sst',['diff','--stage','dev-api','--json']]]){{const p=spawn(cmd==='go-child'?'./go-child':cmd,args,{{stdio:'inherit'}});await new Promise((r,j)=>p.on('exit',x=>x===0?r():j(Error('fixture child failed'))));}}await fetch('https://ssm.us-east-2.amazonaws.com/',{{method:'POST',headers:{{'X-Amz-Target':'AmazonSSM.GetParameter'}},body:'{{"Name":"/sst/bootstrap"}}'}});"""
            r = launch(
                src,
                d / "shared-child",
                ["node", "--input-type=module", "-e", node],
                creds,
                campaign=c,
                phase="collect",
                policy_context={"fixtureOnly": True},
                fixture_origin=origin.server_port,
                extra_ro=mounts(),
            )
            aa = c.db.execute("SELECT n,op FROM attempts ORDER BY n").fetchall()
            assert r["exitCode"] == 0 and len(aa) == len(AUDIT) - old and len(aa) > 4, (
                r,
                aa,
            )
            results.append(
                {
                    "case": "real-SST-Pulumi-Go-Node-Python-shared-budget",
                    "attempts": len(aa),
                    "allForwardedAttemptsReconciled": True,
                }
            )
            c.close()
            pulse = """const{spawn}=await import('node:child_process');const fs=await import('node:fs');const grand="const fs=require('node:fs');setInterval(()=>fs.appendFileSync('/work/grand','x'),30)";const child="const{spawn}=require('node:child_process');const fs=require('node:fs');spawn('node',['-e',"+JSON.stringify(grand)+"],{stdio:'ignore'});setInterval(()=>fs.appendFileSync('/work/child','x'),30)";spawn('node',['-e',child],{stdio:'ignore'});setInterval(()=>fs.appendFileSync('/work/root','x'),30);for(let i=0;i<100&&!['root','child','grand'].every(x=>fs.existsSync('/work/'+x));i++)await new Promise(r=>setTimeout(r,20));"""
            for name in [
                "call-limit",
                "deadline",
                "phase-deadline",
                "mutation",
                "endpoint",
                "upload-good",
                "upload-bad",
            ]:
                phase = "deploy" if name.startswith("upload-") else "collect"
                c = campaign(
                    d / name,
                    phase,
                    maximum=PRIOR + (4 if name == "call-limit" else 26),
                    seconds=5 if name == "deadline" else 3300,
                )
                if name == "phase-deadline":
                    c.put("phaseDeadline", time.monotonic() + 5)
                old = len(AUDIT)
                if name == "call-limit":
                    tail = (
                        "for(let i=0;i<8;i++)await fetch('https://"
                        + S["stateBucket"]
                        + ".s3.us-east-2.amazonaws.com/app/kurier/dev-api.json');"
                    )
                elif name in {"deadline", "phase-deadline"}:
                    tail = "await new Promise(r=>setTimeout(r,10000));"
                elif name == "mutation":
                    tail = (
                        "await fetch('https://lambda.us-east-2.amazonaws.com/2015-03-31/functions/"
                        + S["outputs"]["apiFunction"]
                        + "',{method:'DELETE'});"
                    )
                elif name == "endpoint":
                    tail = "await fetch('https://lambda.us-east-1.amazonaws.com/');"
                else:
                    tail = (
                        "await new Promise(r=>{const p=spawn('python3',['client.py','accepted.zip'"
                        + (",'bad'" if name == "upload-bad" else "")
                        + "],{stdio:'inherit'});p.on('exit',r)});"
                    )
                # Even after a success the fixture deliberately reaches its time
                # limit/denial so descendant shutdown is proven, never left alive.
                if name == "upload-good":
                    tail += "await fetch('https://lambda.us-east-1.amazonaws.com/');"
                code = pulse + tail + "await new Promise(r=>setTimeout(r,3000));"
                r = launch(
                    src,
                    d / (name + "-child"),
                    ["node", "--input-type=module", "-e", code],
                    creds,
                    campaign=c,
                    phase=phase,
                    policy_context={
                        "fixtureOnly": True,
                        "seedArtifacts": True,
                        "closedMarker": CLOSED,
                    },
                    fixture_origin=origin.server_port,
                    extra_ro=mounts(),
                )
                assert r["exitCode"] != 0, (name, r)
                writes = [a for a in AUDIT[old:] if a[0] == "PUT"]
                if name == "upload-good":
                    assert writes == [("PUT", R["packages"][0]["zipSha256"])], (
                        name,
                        writes,
                    )
                else:
                    assert not writes, (name, writes)
                if name in {"mutation", "endpoint", "deadline", "phase-deadline"}:
                    assert len(AUDIT) == old, (name, AUDIT[old:])
                if name == "call-limit":
                    assert c.get("count") == PRIOR + 4 and len(AUDIT) - old == 4
                pulses = [
                    d / (name + "-child") / "work" / x
                    for x in ["root", "child", "grand"]
                ]
                sizes = [p.stat().st_size for p in pulses]
                time.sleep(0.15)
                assert all(sizes) and sizes == [p.stat().st_size for p in pulses]
                results.append(
                    {
                        "case": name,
                        "attempts": c.get("count") - PRIOR,
                        "allDescendantsStopped": True,
                        "upstreamWrites": len(writes),
                        "actualUploadHashVerified": name == "upload-good",
                        "wrongBytesNeverForwarded": name == "upload-bad",
                    }
                )
                c.close()
            check()
        print(
            json.dumps(
                {
                    "fixtureOnly": True,
                    "awsCalls": 0,
                    "priorUsageUnchanged": PRIOR,
                    "cases": results,
                }
            )
        )
    except Exception:
        target = Path(
            os.environ.get(
                "KURIER_TEST_PRIVATE_DIR", "/tmp/kurier-environment-test-private"
            )
        )
        target.mkdir(mode=0o700, parents=True, exist_ok=True)
        os.chmod(target, 0o700)
        if "d" in locals():
            for log in d.glob("**/meter-errors.txt"):
                dest = target / (log.parent.name + "-meter-errors.txt")
                shutil.copyfile(log, dest)
                os.chmod(dest, 0o600)
        raise
    finally:
        origin.shutdown()
        origin.server_close()


if __name__ == "__main__":
    main()
