"""Pinned browser author/reopen/unrelated-edit transport against LOCAL API/provider fixtures."""

import json
from urllib.parse import urlsplit
import test_transport as harness
import test_browser_auth as auth

PROJECT = {
    "projectId": "44444444-4444-4444-8444-444444444444",
    "name": "Local authoring project",
    "version": 0,
    "createdAt": "2026-10-21T00:00:00Z",
    "updatedAt": "2026-10-21T00:00:00Z",
}
REQUEST = {
    "requestId": "55555555-5555-4555-8555-555555555555",
    "revision": 0,
    "name": "Fixed local values",
    "method": "GET",
    "url": "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/assertions-values?fixture=77777777-7777-4777-8777-777777777777",
    "queryParameters": [],
    "headers": [],
    "body": None,
    "operationRef": None,
}
WORKFLOW = {
    "workflowId": "66666666-6666-4666-8666-666666666666",
    "revision": 0,
    "name": "Assertions H",
    "steps": [
        {
            "stepId": f"88888888-8888-4888-8888-88888888888{i}",
            "requestId": REQUEST["requestId"],
            "requestRevision": 0,
            "extract": [],
            "bindings": [],
            "assertions": [],
        }
        for i in range(2)
    ],
}
patches = []


class AuthorOrigin(auth.AuthOrigin):
    def reply(self, code, body, content_type="application/json"):
        raw = body if isinstance(body, bytes) else body.encode()
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(raw)))
        self.send_header("ETag", '"0"')
        self.send_header("Access-Control-Allow-Origin", auth.FRONT)
        self.send_header("Access-Control-Allow-Methods", "POST,GET,OPTIONS,PATCH")
        self.send_header(
            "Access-Control-Allow-Headers",
            "amz-sdk-invocation-id,amz-sdk-request,cache-control,content-type,x-amz-target,x-amz-user-agent,authorization,if-match",
        )
        self.send_header("Access-Control-Expose-Headers", "ETag")
        self.send_header("Access-Control-Max-Age", "0")
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        path = urlsplit(self.path).path
        root = "/api/v1/projects/" + PROJECT["projectId"]
        if path == "/api/v1/projects":
            return self.reply(
                200, json.dumps({"data": {"items": [PROJECT], "nextCursor": None}})
            )
        if path == root:
            return self.reply(200, json.dumps({"data": {"project": PROJECT}}))
        collections = {
            root + "/requests": [REQUEST],
            root + "/workflows": [WORKFLOW],
            root + "/workflow-runs": [],
            root + "/secrets": [],
            root + "/executions": [],
        }
        if path in collections:
            return self.reply(
                200,
                json.dumps({"data": {"items": collections[path], "nextCursor": None}}),
            )
        return super().do_GET()

    def do_PATCH(self):
        assert (
            self.path
            == "/api/v1/projects/"
            + PROJECT["projectId"]
            + "/workflows/"
            + WORKFLOW["workflowId"]
        )
        data = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        assert self.headers["If-Match"] == '"' + str(WORKFLOW["revision"]) + '"'
        patches.append(data)
        WORKFLOW.update(data)
        WORKFLOW["revision"] += 1
        self.reply(200, json.dumps({"data": {"workflow": WORKFLOW}}))


def scenario(run, d, env):
    (d / "control.json").write_text(
        json.dumps(
            {
                "writeCategories": ["aws", "api"],
                "awsWriteOperations": [
                    "InitiateAuth",
                    "RespondToAuthChallenge",
                    "GetTokensFromRefreshToken",
                    "RevokeToken",
                ],
            }
        )
    )
    try:
        run(
            [
                "node",
                str(harness.ROOT / "scripts/fixtures/workflow-browser-authoring.mjs"),
            ]
        )
    except RuntimeError:
        print(
            json.dumps(
                {
                    "localWorkflowName": WORKFLOW["name"],
                    "localWorkflowRevision": WORKFLOW["revision"],
                    "localPatches": len(patches),
                }
            )
        )
        raise
    rows = [json.loads(x) for x in (d / "ledger.jsonl").read_text().splitlines()]
    assert not any(r["event"] == "stop" for r in rows)
    assert len(patches) == 3
    assert (
        len(WORKFLOW["steps"][0]["assertions"]) == 8
        and len(WORKFLOW["steps"][1]["assertions"]) == 8
    )
    tokens = [
        a["expected"]["decimal"]
        for s in WORKFLOW["steps"]
        for a in s["assertions"]
        if a["kind"] == "jsonEquals"
    ]
    assert tokens == [
        "0.10000000000000001",
        "0.1",
        "1e-1",
        "1e-308",
        "9007199254740991",
        "-0.0",
        "0.10000000000000001",
        "0.1",
        "1E-01",
        "1e-308",
        "9007199254740991",
        "-0.0",
    ]
    print(
        json.dumps(
            {
                "pinnedBrowserAuthorReopenAndEdit": True,
                "localPatches": 3,
                "exactDecimalTokens": True,
                "liveAwsCalls": 0,
                "targetSends": 0,
            }
        )
    )


if __name__ == "__main__":
    harness.Origin = AuthorOrigin
    harness.main(scenario=scenario)
