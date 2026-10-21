"""Real pinned browser/Amplify SRP, refresh and sign-out via the real TLS meter.
Cognito responses and GET-only API are LOCAL fixtures, never AWS validation.
"""

import base64
import collections
import json
import re
import time
import test_transport as harness

ROOT = harness.ROOT
APP = (
    ROOT
    / ".build/workflow-assertions-release/68d9f218225c7092a1e4fca0b66a21421d862a22/frontend/app"
)
CLIENT = "5jg9sb38i6ae9c0adm5rtdfkno"
FRONT = "https://d2kpq905qyh9zg.cloudfront.net"
USER = "11111111-1111-4111-8111-111111111111"
EMAIL = "fixture@example.invalid"
BLOCK = base64.b64encode(b"local-private-secret-block").decode()
counts = collections.Counter()
operations = []
canaries = [
    "Local-only-password1!",
    "local-session-private",
    BLOCK,
    "local-refresh-initial-private",
    "local-refresh-rotated-private",
]
issued = []


def token(kind, expiry):
    def b64(v):
        return (
            base64.urlsafe_b64encode(json.dumps(v, separators=(",", ":")).encode())
            .decode()
            .rstrip("=")
        )

    now = int(time.time())
    value = (
        b64({"alg": "HS256", "typ": "JWT"})
        + "."
        + b64(
            {
                "sub": USER,
                "iss": "https://cognito-idp.us-east-2.amazonaws.com/us-east-2_qTDZQT1FE",
                "token_use": kind,
                "origin_jti": USER,
                "client_id": CLIENT,
                "aud": CLIENT,
                "username": EMAIL,
                "cognito:username": EMAIL,
                "iat": now,
                "auth_time": now,
                "exp": now + expiry,
            }
        )
        + ".bG9jYWwtc2lnbmF0dXJl"
    )
    canaries.append(value)
    return value


class AuthOrigin(harness.Origin):
    def reply(self, code, body, content_type="application/json"):
        raw = body if isinstance(body, bytes) else body.encode()
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(raw)))
        self.send_header("Access-Control-Allow-Origin", FRONT)
        self.send_header("Access-Control-Allow-Methods", "POST,GET,OPTIONS")
        self.send_header(
            "Access-Control-Allow-Headers",
            "amz-sdk-invocation-id,amz-sdk-request,cache-control,content-type,x-amz-target,x-amz-user-agent,authorization",
        )
        self.send_header("Access-Control-Max-Age", "0")
        self.end_headers()
        self.wfile.write(raw)

    def do_OPTIONS(self):
        if self.path == "/":
            counts["aws-options"] += 1
        elif self.path.startswith("/api/v1/"):
            counts["api-options"] += 1
        else:
            raise AssertionError("Unexpected preflight scope")
        self.reply(204, b"")

    def do_GET(self):
        if self.path.startswith("/api/v1/"):
            counts["api-get"] += 1
            assert self.headers.get("Authorization") in [
                "Bearer " + value for value in issued
            ]
            if self.path == "/api/v1/users/me":
                return self.reply(
                    200,
                    json.dumps(
                        {
                            "data": {
                                "user": {
                                    "userId": USER,
                                    "displayName": "Local fixture",
                                    "createdAt": "2026-10-21T00:00:00Z",
                                }
                            }
                        }
                    ),
                )
            assert self.path.split("?")[0] == "/api/v1/projects"
            return self.reply(200, '{"data":{"items":[],"nextCursor":null}}')
        path = self.path.split("?")[0]
        path = "index.html" if path == "/" else path.lstrip("/")
        assert path in ["index.html", "startup.js", "favicon.ico"] or re.fullmatch(
            r"assets/[A-Za-z0-9_-]+\.(js|css)", path
        )
        if path == "favicon.ico":
            return self.reply(204, b"")
        content_type = (
            "text/html"
            if path.endswith(".html")
            else "application/javascript"
            if path.endswith(".js")
            else "text/css"
        )
        self.reply(200, (APP / path).read_bytes(), content_type)

    def do_POST(self):
        data = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        operation = self.headers.get("x-amz-target", "").split(".")[-1]
        assert operation in [
            "InitiateAuth",
            "RespondToAuthChallenge",
            "GetTokensFromRefreshToken",
            "RevokeToken",
        ]
        operations.append(operation)
        counts["aws-post"] += 1
        assert data["ClientId"] == CLIENT
        if operation == "InitiateAuth":
            assert (
                data["AuthFlow"] == "USER_SRP_AUTH"
                and data["AuthParameters"]["USERNAME"] == EMAIL
            )
            assert re.fullmatch(r"[A-Fa-f0-9]{128,}", data["AuthParameters"]["SRP_A"])
            return self.reply(
                200,
                json.dumps(
                    {
                        "ChallengeName": "PASSWORD_VERIFIER",
                        "Session": "local-session-private",
                        "ChallengeParameters": {
                            "SALT": "deadbeef",
                            "SRP_B": "2",
                            "SECRET_BLOCK": BLOCK,
                            "USER_ID_FOR_SRP": EMAIL,
                            "USERNAME": EMAIL,
                        },
                    }
                ),
            )
        if operation == "RespondToAuthChallenge":
            assert data["ChallengeName"] == "PASSWORD_VERIFIER"
            response = data["ChallengeResponses"]
            assert response["USERNAME"] == EMAIL
            assert response["PASSWORD_CLAIM_SECRET_BLOCK"] == BLOCK
            assert (
                re.fullmatch(
                    r"[A-Za-z0-9+/]{43}=", response["PASSWORD_CLAIM_SIGNATURE"]
                )
                and response["TIMESTAMP"]
            )
            expiry = 2
            refresh = "local-refresh-initial-private"
        elif operation == "GetTokensFromRefreshToken":
            assert data["RefreshToken"] == "local-refresh-initial-private"
            expiry = 900
            refresh = "local-refresh-rotated-private"
        else:
            assert data["Token"] == "local-refresh-rotated-private"
            return self.reply(200, "{}")
        access = token("access", expiry)
        issued.append(access)
        self.reply(
            200,
            json.dumps(
                {
                    "AuthenticationResult": {
                        "AccessToken": access,
                        "IdToken": token("id", expiry),
                        "RefreshToken": refresh,
                        "ExpiresIn": expiry,
                        "TokenType": "Bearer",
                    }
                }
            ),
        )


def scenario(run, d, env):
    (d / "control.json").write_text(
        json.dumps(
            {
                "writeCategories": ["aws"],
                "awsWriteOperations": [
                    "InitiateAuth",
                    "RespondToAuthChallenge",
                    "GetTokensFromRefreshToken",
                    "RevokeToken",
                ],
            }
        )
    )
    client = ROOT / "scripts/fixtures/workflow-browser-auth.mjs"
    try:
        run(["node", str(client)])
    except RuntimeError:
        local_events = [
            json.loads(line) for line in (d / "ledger.jsonl").read_text().splitlines()
        ]
        print(
            json.dumps(
                {
                    "localAuthOperations": operations,
                    "originCounts": dict(counts),
                    "held": [e for e in local_events if e["event"] == "held-request"],
                    "outcomes": [
                        {
                            "status": e.get("status"),
                            "transportError": e.get("transportError"),
                        }
                        for e in local_events
                        if e["event"] == "outcome"
                    ],
                }
            )
        )
        raise
    events = [
        json.loads(line) for line in (d / "ledger.jsonl").read_text().splitlines()
    ]
    assert not any(e["event"] == "stop" for e in events)
    attempts = [e for e in events if e["event"] == "attempt"]
    aws = [e for e in attempts if e["category"] == "aws"]
    api = [e for e in attempts if e["category"] == "api"]
    assert operations == [
        "InitiateAuth",
        "RespondToAuthChallenge",
        "GetTokensFromRefreshToken",
        "RevokeToken",
    ], operations
    assert (
        len(aws) == counts["aws-options"] + counts["aws-post"]
        and counts["aws-options"] >= 4
    )
    assert (
        len(api) == counts["api-get"] + counts["api-options"] and counts["api-get"] >= 3
    )
    assert (
        sum(e["operation"] == "CognitoAuthPreflight" for e in aws)
        == counts["aws-options"]
    )
    # A disallowed resource write must STILL be rejected before the local origin.
    denied = "const r=await fetch('https://cognito-idp.us-east-2.amazonaws.com/',{method:'POST',headers:{'Content-Type':'application/x-amz-json-1.1','X-Amz-Target':'AWSCognitoIdentityProviderService.AdminCreateUser'},body:JSON.stringify({Username:'private-held-body-canary'})});if(r.status!==599)throw Error('resource write was not held');"
    run(["node", "--input-type=module", "-e", denied])
    text = (d / "ledger.jsonl").read_text()
    final = [json.loads(line) for line in text.splitlines()]
    held = [e for e in final if e["event"] == "held-request"]
    assert (
        held[-1]["method"] == "POST"
        and held[-1]["category"] == "aws"
        and held[-1]["operation"] == "AdminCreateUser"
    )
    assert operations == [
        "InitiateAuth",
        "RespondToAuthChallenge",
        "GetTokensFromRefreshToken",
        "RevokeToken",
    ]
    assert all(value not in text for value in canaries + ["private-held-body-canary"])
    print(
        json.dumps(
            {
                "passed": True,
                "pinnedBrowserBundle": True,
                "srpPasswordVerifier": True,
                "rotatingRefresh": True,
                "revocation": True,
                "cognitoForwardedOptions": counts["aws-options"],
                "cognitoPostAttempts": counts["aws-post"],
                "meteredAwsAttempts": len(aws),
                "apiAttempts": len(api),
                "resourceWriteHeld": True,
                "safeHeldDiagnostics": True,
                "rawCredentialsPersisted": False,
                "realAwsCalls": 0,
                "realIdentitiesCreated": 0,
                "targetSends": 0,
            }
        )
    )


if __name__ == "__main__":
    harness.Origin = AuthOrigin
    harness.main(scenario=scenario)
