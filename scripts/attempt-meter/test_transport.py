"""Controlled-origin integration: CLI/Go/Node/browser through isolated TLS proxy."""

import collections
import http.server
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import time
import sys

ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = Path(__file__).resolve().parent
PYTHON = ROOT / ".build/attempt-meter-venv/bin/python"
MITM = ROOT / ".build/attempt-meter-venv/bin/mitmdump"
counts = collections.Counter()


class Origin(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_args):
        pass

    def reply(self, code, body, content_type="application/json"):
        raw = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(raw)))
        self.send_header("Access-Control-Allow-Origin", "*")

        if code == 302:
            self.send_header(
                "Location",
                "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/api/v1/must-not-follow",
            )
        if code == 503 and self.path.startswith("/api/v1/"):
            self.send_header("Retry-After", "2")
        self.end_headers()
        self.wfile.write(raw)

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        if self.path.startswith("/api/v1/"):
            counts["api-write"] += 1
            # The upstream may have accepted the write; lose the response.
            self.close_connection = True
            self.connection.shutdown(socket.SHUT_RDWR)
            return
        label = "go" if "aws-sdk-go" in self.headers.get("User-Agent", "") else "cli"
        counts[label] += 1
        if counts[label] == 1:
            return self.reply(
                503, '{"__type":"InternalServerError","message":"controlled retry"}'
            )
        data = json.loads(body)
        result = {"Items": [], "Count": 0, "ScannedCount": 0}
        if not data.get("ExclusiveStartKey"):
            result["LastEvaluatedKey"] = {"PK": {"S": "next"}, "SK": {"S": "next"}}
        self.reply(200, json.dumps(result))

    def do_DELETE(self):
        counts["api-delete"] += 1
        self.reply(202, '{"data":{"deletionOperation":{"operationId":"local"}}}')

    def do_GET(self):
        if "list-type=2" in self.path:
            counts["node-sdk"] += 1
            if counts["node-sdk"] == 1:
                return self.reply(
                    503, "<Error><Code>SlowDown</Code></Error>", "application/xml"
                )
            last = "continuation-token=" in self.path
            return self.reply(
                200,
                '<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">'
                "<IsTruncated>"
                + ("false" if last else "true")
                + "</IsTruncated>"
                + (
                    ""
                    if last
                    else "<NextContinuationToken>next</NextContinuationToken>"
                )
                + "</ListBucketResult>",
                "application/xml",
            )
        if "/workflow-runs/" in self.path:
            counts["coordinator-poll"] += 1
            if counts["coordinator-poll"] == 1:
                return self.reply(503, '{"error":{"code":"service_unavailable"}}')
            return self.reply(
                200,
                '{"data":{"run":{"workflowRunId":"22222222-2222-4222-8222-222222222222","state":"completed"}}}',
            )
        if self.path == "/api/v1/redirect":
            counts["redirect"] += 1
            return self.reply(302, "{}")
        if self.path == "/api/v1/must-not-follow":
            counts["followed"] += 1
            return self.reply(500, "{}")
        counts["api"] += 1
        self.reply(200, '{"data":{"items":[]}}')


def unused_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def main(mode="normal"):
    origin = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Origin)
    threading.Thread(target=origin.serve_forever, daemon=True).start()
    processes = []
    with tempfile.TemporaryDirectory(prefix="kurier-meter-local-") as directory:
        d = Path(directory)
        os.chmod(d, 0o700)
        (d / "control.json").write_text('{"writeCategories":[]}')
        port = unused_port()
        env = dict(
            os.environ,
            KURIER_METER_LEDGER=str(d / "ledger.jsonl"),
            KURIER_METER_CONTROL=str(d / "control.json"),
            KURIER_METER_LOCAL_ORIGIN=str(origin.server_port),
        )
        if mode == "cleanup":
            parent = d / "parent.jsonl"
            prior = {"api": 62, "aws": 766, "poll": 7, "static": 9}
            parent.write_text(
                "\n".join(
                    json.dumps(e)
                    for e in [
                        {"event": "meter-created"},
                        {
                            "event": "attempt",
                            "sequence": 2,
                            "at": time.time() - 300,
                            "write": False,
                            "counts": prior,
                        },
                        {
                            "event": "outcome",
                            "attempt": 2,
                            "status": 200,
                            "transportError": False,
                        },
                        {"event": "stop", "reason": "write-held", "counts": prior},
                    ]
                )
                + "\n"
            )
            scope = d / "scope.json"
            scope.write_text(
                json.dumps(
                    {
                        "projects": [
                            "11111111-1111-4111-8111-111111111111",
                            "22222222-2222-4222-8222-222222222222",
                        ],
                        "users": [
                            "kurier-assertions-11111111-1111-4111-8111-111111111111-0@example.invalid",
                            "kurier-assertions-11111111-1111-4111-8111-111111111111-1@example.invalid",
                        ],
                    }
                )
            )
            env.update(
                KURIER_METER_CLEANUP_PARENT=str(parent),
                KURIER_METER_CLEANUP_SCOPE=str(scope),
            )
        proxy = subprocess.Popen(
            [
                str(MITM),
                "--quiet",
                "--listen-host",
                "127.0.0.1",
                "--listen-port",
                str(port),
                "--set",
                "confdir=" + str(d / "ca"),
                "--set",
                "connection_strategy=lazy",
                "--set",
                "upstream_cert=false",
                "--set",
                "http2=true",
                "-s",
                str(SCRIPTS / "proxy.py"),
            ],
            env=env,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        processes.append(proxy)
        for _ in range(100):
            if (d / "ca/mitmproxy-ca-cert.pem").exists() and (
                d / "ledger.jsonl"
            ).exists():
                break
            if proxy.poll() is not None:
                raise RuntimeError("Local meter failed to start")
            time.sleep(0.1)
        relay = subprocess.Popen(
            [
                str(PYTHON),
                str(SCRIPTS / "relay.py"),
                "host",
                str(d / "relay.sock"),
                str(port),
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        processes.append(relay)
        for _ in range(100):
            if (d / "relay.sock").exists():
                break
            time.sleep(0.05)
        child_env = dict(
            os.environ,
            HTTP_PROXY="http://127.0.0.1:18080",
            HTTPS_PROXY="http://127.0.0.1:18080",
            ALL_PROXY="http://127.0.0.1:18080",
            NO_PROXY="127.0.0.1,localhost,::1",
            no_proxy="127.0.0.1,localhost,::1",
            http_proxy="http://127.0.0.1:18080",
            https_proxy="http://127.0.0.1:18080",
            AWS_CA_BUNDLE=str(d / "ca/mitmproxy-ca-cert.pem"),
            SSL_CERT_FILE=str(d / "ca/mitmproxy-ca-cert.pem"),
            NODE_EXTRA_CA_CERTS=str(d / "ca/mitmproxy-ca-cert.pem"),
            NODE_OPTIONS="--use-env-proxy --import=" + str(SCRIPTS / "node-proxy.mjs"),
            KURIER_ATTEMPT_METER_ACTIVE="local-test",
            AWS_ACCESS_KEY_ID="localfixture",
            AWS_SECRET_ACCESS_KEY="localfixture",
            AWS_SESSION_TOKEN="",
            AWS_EC2_METADATA_DISABLED="true",
            AWS_MAX_ATTEMPTS="3",
            AWS_RETRY_MODE="standard",
            AWS_CONFIG_FILE="/dev/null",
            AWS_SHARED_CREDENTIALS_FILE="/dev/null",
            AWS_PROFILE="",
        )

        child_env.pop("AWS_PROFILE", None)
        child_env.pop("AWS_DEFAULT_PROFILE", None)

        def run(command, cwd=ROOT):
            isolated = [
                "bwrap",
                "--unshare-net",
                "--ro-bind",
                "/",
                "/",
                "--bind",
                str(ROOT),
                str(ROOT),
                "--bind",
                "/tmp",
                "/tmp",
                "--proc",
                "/proc",
                "--dev",
                "/dev",
                str(PYTHON),
                str(SCRIPTS / "relay.py"),
                "inside",
                str(d / "relay.sock"),
                "18080",
                *command,
            ]
            result = subprocess.run(
                isolated, cwd=cwd, env=child_env, capture_output=True, timeout=90
            )
            if result.returncode:
                # Only local fake credentials/fixture errors can occur here.
                raise RuntimeError(
                    "Controlled client failed: " + result.stderr.decode()[-1200:]
                )
            return result.stdout

        try:
            if mode == "preflight":
                browser = """const{chromium}=await import('@playwright/test');const b=await chromium.launch({headless:true,proxy:{server:'http://127.0.0.1:18080'}});try{const c=await b.newContext({ignoreHTTPSErrors:true});const p=await c.newPage();await p.goto('https://d2kpq905qyh9zg.cloudfront.net');const blocked=await p.evaluate(async()=>{try{await fetch('https://cognito-idp.us-east-2.amazonaws.com/',{method:'POST',headers:{'Content-Type':'application/x-amz-json-1.1','X-Amz-Target':'AWSCognitoIdentityProviderService.InitiateAuth'},body:'{}'});return false}catch{return true}});if(!blocked)throw Error('expected known preflight gap');await c.close()}finally{await b.close()}"""
                run(["node", "--input-type=module", "-e", browser])
                log = [
                    json.loads(line)
                    for line in (d / "ledger.jsonl").read_text().splitlines()
                ]
                assert any(e.get("reason") == "write-held" for e in log)
                assert not any(e.get("category") == "aws" for e in log)
                assert counts["cli"] == 0 and counts["api-write"] == 0
                print(
                    json.dumps(
                        {
                            "browserCognitoPreflightHeldLocally": True,
                            "awsCalls": 0,
                            "forwardedAuthWrites": 0,
                        }
                    )
                )
                return
            if mode == "cleanup":
                raw = parent.read_bytes()
                client = """const root='https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/api/v1/projects/11111111-1111-4111-8111-111111111111';for(const [method,status]of [['GET',200],['DELETE',202],['POST',599]]){const r=await fetch(root,{method});if(r.status!==status)throw Error('cleanup transport guard')}"""
                run(["node", "--input-type=module", "-e", client])
                log = [
                    json.loads(line)
                    for line in (d / "ledger.jsonl").read_text().splitlines()
                ]
                attempts = [e for e in log if e["event"] == "attempt"]
                assert len(attempts) == 2 and attempts[-1]["counts"] == {
                    **prior,
                    "api": 64,
                }
                assert counts["api-delete"] == 1 and counts["api-write"] == 0
                assert any(e.get("reason") == "cleanup-only-scope" for e in log)
                assert parent.read_bytes() == raw
                print(
                    json.dumps(
                        {
                            "cleanupOnlyTransport": True,
                            "inheritedApiCalls": 62,
                            "inheritedAwsAttempts": 766,
                            "campaignPostBlocked": True,
                            "parentUnchanged": True,
                            "awsCalls": 0,
                        }
                    )
                )
                return
            run(
                [
                    "node",
                    "--input-type=module",
                    "-e",
                    "const http=await import('node:http');const s=http.createServer((q,r)=>r.end('local-rpc'));await new Promise(r=>s.listen(0,'127.0.0.1',r));try{const result=await new Promise((resolve,reject)=>http.get('http://127.0.0.1:'+s.address().port,r=>{let v='';r.on('data',x=>v+=x);r.on('end',()=>resolve(v));}).on('error',reject));if(result!=='local-rpc')throw Error('Local RPC failed')}finally{s.close()}",
                ]
            )
            # Prove proxy environment bypass cannot reach an external network.
            run(
                [
                    "python3",
                    "-c",
                    "import errno,socket;s=socket.socket();s.settimeout(.2);assert s.connect_ex(('1.1.1.1',443))==errno.ENETUNREACH;print('blocked')",
                ]
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
            go = d / "go-client.go"
            go.write_text("""package main
import("context";"fmt";"github.com/aws/aws-sdk-go-v2/config";"github.com/aws/aws-sdk-go-v2/service/dynamodb")
func main(){c:=context.Background();cfg,e:=config.LoadDefaultConfig(c,config.WithRegion("us-east-2"));if e!=nil{panic(e)};d:=dynamodb.NewFromConfig(cfg);name:="fixture";p:=dynamodb.NewScanPaginator(d,&dynamodb.ScanInput{TableName:&name});for p.HasMorePages(){if _,e=p.NextPage(c);e!=nil{panic(e)}};fmt.Println("go-ok")}
""")
            # Go build cache is writable only in this task's temporary directory.
            child_env["GOCACHE"] = str(d / "go-cache")
            run(["go", "run", str(go)], ROOT / "services/api")
            node = """const {S3Client,paginateListObjectsV2}=await import('./.sst/platform/node_modules/@aws-sdk/client-s3/dist-cjs/index.js');const c=new S3Client({region:'us-east-2',maxAttempts:3,forcePathStyle:true});for await(const p of paginateListObjectsV2({client:c},{Bucket:'fixture'})){};const r=await fetch('https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/api/v1/projects',{headers:{Authorization:'Bearer local-private-canary'}});if(r.status!==200)throw Error('local fetch failed');console.log('node-ok');"""
            run(["node", "--input-type=module", "-e", node])
            browser = """const{chromium}=await import('@playwright/test');const b=await chromium.launch({headless:true,proxy:{server:'http://127.0.0.1:18080'}});try{const c=await b.newContext({ignoreHTTPSErrors:true});const p=await c.newPage();await p.goto('https://d2kpq905qyh9zg.cloudfront.net');const statuses=await p.evaluate(async()=>{let a=[];for(let i=0;i<2;i++){let r=await fetch('https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/api/v1/projects');a.push(r.status)}return a});if(statuses.join(',')!=='200,200')throw Error('browser meter failed');await c.close()}finally{await b.close()}console.log('browser-ok');"""
            run(["node", "--input-type=module", "-e", browser])
            events = [
                json.loads(line)
                for line in (d / "ledger.jsonl").read_text().splitlines()
            ]
            attempts = [e for e in events if e["event"] == "attempt"]
            assert not any(e["event"] == "stop" for e in events), (
                "Meter stopped in controlled suite"
            )
            assert (
                counts["cli"] == 3 and counts["go"] == 3 and counts["node-sdk"] == 3
            ), dict(counts)
            aws = [e for e in attempts if e["category"] == "aws"]
            api = [e for e in attempts if e["category"] == "api"]
            assert len(aws) == 9 and len(api) == 3, (len(aws), len(api), dict(counts))
            assert "local-private-canary" not in (d / "ledger.jsonl").read_text()
            assert any(e.get("protocol", "").startswith("HTTP/2") for e in api), (
                "HTTP/2 request counting not exercised"
            )
            child_env["KURIER_LOCAL_COORDINATOR_TEST"] = str(d)
            run(["node", "scripts/fixtures/workflow-coordinator-transport.mjs"])
            assert counts["coordinator-poll"] == 2 and counts["redirect"] == 1
            assert counts["followed"] == 0 and counts["api-write"] == 0
            updated = [
                json.loads(line)
                for line in (d / "ledger.jsonl").read_text().splitlines()
            ]
            api_updated = [
                e for e in updated if e["event"] == "attempt" and e["category"] == "api"
            ]
            assert len(api_updated) == 6 and sum(e["poll"] for e in api_updated) == 2
            assert "local-private-canary" not in (d / "ledger.jsonl").read_text()
            # The origin accepts one write, then loses its ACK. The proxy must
            # latch before a client can replay it, independently of client logic.
            (d / "control.json").write_text('{"writeCategories":["api"]}')
            lost = """for(let i=0;i<2;i++){const r=await fetch('https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/api/v1/projects',{method:'POST',body:'{}',headers:{'Content-Type':'application/json'}});if((i===0&&r.status!==502)||(i===1&&r.status!==599))throw Error('uncertain-write guard failed')}console.log('no-replay');"""
            run(["node", "--input-type=module", "-e", lost])
            assert counts["api-write"] == 1, (
                "Proxy or client replayed an uncertain write"
            )
            final = [
                json.loads(line)
                for line in (d / "ledger.jsonl").read_text().splitlines()
            ]
            assert any(e.get("reason") == "uncertain-write" for e in final)
            print(
                json.dumps(
                    dict(
                        passed=True,
                        sdkRetryAndPaginationAttempts=9,
                        apiAttempts=6,
                        coordinator503Then200=True,
                        redirectRejected=True,
                        admissionsAndTargetSends=0,
                        directNetworkBypassBlocked=True,
                        browserCovered=True,
                        http2Covered=True,
                        lostWriteAcknowledgmentNoReplay=True,
                        awsCalls=0,
                        credentialsOrBodiesPersisted=False,
                    )
                )
            )
        finally:
            for process in reversed(processes):
                process.terminate()
                process.wait(timeout=10)
            origin.shutdown()


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "normal")
