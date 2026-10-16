// Genuine Cognito SRP -> deployed HTTP API/Lambda -> DDB/S3/worker/schedule.
// All passwords/JWTs/capabilities/evidence stay in child memory. No browser/TUI claim.
import { readFileSync, writeFileSync } from "node:fs";
import { randomUUID, randomBytes, createHash } from "node:crypto";
import { Amplify } from "aws-amplify";
import { signIn, signOut, fetchAuthSession } from "aws-amplify/auth";
import { cognitoUserPoolsTokenProvider } from "aws-amplify/auth/cognito";
import { sharedInMemoryStorage } from "aws-amplify/utils";
import {
  aws,
  awsProcess,
  outputs as o,
  identity,
  marker,
  hash,
} from "./verify-read-only-mcp-deployment.mjs";
const [mode, path, ...rest] = process.argv.slice(2);
if (
  !["--validate", "--resume", "--cleanup"].includes(mode) ||
  !path ||
  rest.length
)
  throw Error(
    "Choose --validate NEW_RECEIPT, --resume EXISTING_RECEIPT or --cleanup EXISTING_RECEIPT; receipts contain only fixture identifiers.",
  );
class ValidationFailure extends Error {}
const check = (ok, message) => {
  if (!ok) throw new ValidationFailure(message);
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const receipts =
  mode !== "--validate"
    ? JSON.parse(readFileSync(path, "utf8"))
    : {
        kind: "kurier-mcp-aws-disposable-v1",
        users: [],
        projects: [],
        startedAt: new Date().toISOString(),
      };
const save = () =>
  writeFileSync(path, JSON.stringify(receipts, null, 2) + "\n", {
    mode: 0o600,
  });
if (mode === "--validate")
  writeFileSync(path, JSON.stringify(receipts), { mode: 0o600, flag: "wx" });
check(
  receipts.kind === "kurier-mcp-aws-disposable-v1",
  "Explicit disposable MCP receipt required",
);
Amplify.configure({
  Auth: {
    Cognito: {
      userPoolId: o.cognitoUserPoolId,
      userPoolClientId: o.cognitoClientId,
    },
  },
});
cognitoUserPoolsTokenProvider.setKeyValueStorage(sharedInMemoryStorage);
const live = {};
async function login(u) {
  const password = "Kurier1!" + randomBytes(24).toString("base64url");
  aws(["cognito-idp", "admin-set-user-password"], {
    UserPoolId: o.cognitoUserPoolId,
    Username: u.name,
    Password: password,
    Permanent: true,
  });
  await signOut();
  const signed = await signIn({
    username: u.name,
    password,
    options: { authFlowType: "USER_SRP_AUTH" },
  });
  check(signed.isSignedIn, "Real Cognito SRP did not sign in");
  const s = await fetchAuthSession();
  check(s.tokens?.accessToken, "Cognito access session missing");
  const token = s.tokens.accessToken.toString();
  u.subject = s.tokens.accessToken.payload.sub;
  return token;
}
async function call(method, route, token, body, headers = {}) {
  const r = await fetch(o.apiUrl + route, {
    method,
    headers: {
      ...(token ? { Authorization: "Bearer " + token } : {}),
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    redirect: "error",
    signal: AbortSignal.timeout(15000),
  });
  const raw = await r.text();
  let data;
  try {
    data = raw ? JSON.parse(raw) : null;
  } catch {
    throw Error("Deployed response was not JSON");
  }
  return { status: r.status, data, headers: r.headers, raw };
}
async function expected(method, route, token, body, status, headers) {
  let r = await call(method, route, token, body, headers);
  // Hydration may contend with a live writer. Retry only bounded read failures,
  // never creation/admission/revocation or any ambiguous mutation.
  if (method === "GET")
    for (let n = 0; r.status === 503 && n < 4; n++) {
      await sleep(1500);
      r = await call(method, route, token, body, headers);
    }
  check(
    r.status === status,
    `Unexpected deployed status: ${method} ${route} expected${status} got${r.status} code${/^[a-z_]+$/.test(r.data?.error?.code ?? "") ? r.data.error.code : "unavailable"}`,
  );
  return r;
}
function query(table, pk, prefix = "") {
  const r = aws(["dynamodb", "query"], {
    TableName: table,
    KeyConditionExpression:
      "PK = :pk" + (prefix ? " AND begins_with(SK,:prefix)" : ""),
    ExpressionAttributeValues: {
      ":pk": { S: pk },
      ...(prefix ? { ":prefix": { S: prefix } } : {}),
    },
    ConsistentRead: true,
  });
  check(!r.LastEvaluatedKey, "Fixture query exceeded one bounded page");
  return r.Items ?? [];
}
async function cleanup() {
  for (const u of receipts.users)
    if (u.created && !u.deleted && !live[u.name]) live[u.name] = await login(u);
  for (const u of receipts.users)
    if (u.created && !u.deleted && live[u.name]) {
      const a = await expected(
        "GET",
        "/api/v1/local-agents/current",
        live[u.name],
        undefined,
        200,
      );
      if (a.data.data.agent && !a.data.data.agent.revokedAt)
        await expected(
          "DELETE",
          "/api/v1/local-agents/" + a.data.data.agent.localAgentId,
          live[u.name],
          undefined,
          204,
        );
    }
  for (const p of receipts.projects) {
    if (p.cleaned) continue;
    const bearer = live[p.user];
    check(bearer, "Fixture owner session required");
    let r = await call("GET", "/api/v1/projects/" + p.id, bearer);
    if (r.status === 200) {
      r = await expected(
        "DELETE",
        "/api/v1/projects/" + p.id,
        bearer,
        undefined,
        202,
        { "If-Match": r.headers.get("etag") },
      );
      p.operation = r.data.data.deletionOperation.operationId;
      save();
    } else check(r.status === 404, "Fixture cleanup requires inspection");
    // Scheduled consumer only. Never hide a schedule failure by direct draining.
    let finished = false;
    for (let n = 0; n < 180; n++) {
      const rows = query(o.controlTable, "P#" + p.id);
      if (
        rows.length === 1 &&
        rows[0].kind?.S === "projectTombstone" &&
        rows[0].state?.S === "deleted"
      ) {
        finished = true;
        break;
      }
      await sleep(5000);
    }
    check(
      finished,
      "Scheduled fixture deletion incomplete; resume cleanup using receipt",
    );
    check(
      query(o.protectedTable, "P#" + p.id).length === 0,
      "Protected fixture cleanup incomplete",
    );
    const list = aws([
      "s3api",
      "list-objects-v2",
      "--bucket",
      o.evidenceBucket,
      "--prefix",
      "dev-api/projects/" + p.id + "/",
      "--max-keys",
      "1",
    ]);
    check(
      !list.IsTruncated && (list.KeyCount ?? 0) === 0,
      "S3 fixture cleanup incomplete",
    );
    p.cleaned = true;
    save();
  }
  for (const u of receipts.users) {
    if (!u.created || u.deleted) continue;
    if (u.owner) {
      const rows = query(o.controlTable, "U#" + u.owner);
      check(
        rows.every(
          (r) =>
            r.kind?.S !== "localAgentCredential" || r.state?.S === "revoked",
        ),
        "Agent fixture obligations remain",
      );
      // Explicit disposable fixture teardown; this is not a 90-day retention claim.
      for (const r of rows) {
        if (r.kind?.S === "executionQuota") continue; // Never refund/reset usage.
        if (r.kind?.S === "mcpCredential" && r.tokenHash?.S)
          aws(["dynamodb", "delete-item"], {
            TableName: o.controlTable,
            Key: { PK: { S: "MCPTOKEN#" + r.tokenHash.S }, SK: { S: "META" } },
          });
        if (r.kind?.S === "localAgentCredential" && r.tokenHash?.S)
          aws(["dynamodb", "delete-item"], {
            TableName: o.controlTable,
            Key: {
              PK: { S: "AGENTTOKEN#" + r.tokenHash.S },
              SK: { S: "META" },
            },
          });
        aws(["dynamodb", "delete-item"], {
          TableName: o.controlTable,
          Key: { PK: r.PK, SK: r.SK },
        });
      }
      const h = createHash("sha256");
      for (const value of [o.cognitoIssuer, u.subject]) {
        const b = Buffer.from(value);
        const size = Buffer.alloc(4);
        size.writeUInt32BE(b.length);
        h.update(size).update(b);
      }
      aws(["dynamodb", "delete-item"], {
        TableName: o.controlTable,
        Key: { PK: { S: "IDENTITY#" + h.digest("hex") }, SK: { S: "META" } },
        ConditionExpression: "userId = :owner",
        ExpressionAttributeValues: { ":owner": { S: u.owner } },
      });
      check(
        query(o.controlTable, "U#" + u.owner).every(
          (r) => r.kind?.S === "executionQuota",
        ),
        "Disposable owner cleanup incomplete",
      );
    }
    aws(["cognito-idp", "admin-delete-user"], {
      UserPoolId: o.cognitoUserPoolId,
      Username: u.name,
    });
    u.deleted = true;
    save();
  }
  await signOut();
  receipts.cleaned = true;
  save();
}
async function main() {
  identity();
  const m = marker();
  check(
    m.state?.S === "active" &&
      m.savedRequestsSchemaVersion?.N === "7" &&
      m.mcpReadOnlySchemaVersion?.N === "1",
    "Activated MCP development stage required",
  );
  if (mode === "--cleanup") {
    await cleanup();
    console.log(
      JSON.stringify({
        cleanup: true,
        projects: receipts.projects.length,
        credentialsRemoved: true,
      }),
    );
    return;
  }
  if (mode === "--resume") {
    for (const u of receipts.users) {
      check(u.created && !u.deleted, "Existing disposable identities required");
      live[u.name] = await login(u);
      const me = await expected(
        "GET",
        "/api/v1/users/me",
        live[u.name],
        undefined,
        200,
      );
      check(me.data.data.user.userId === u.owner, "Fixture owner mismatch");
    }
    check(
      receipts.projects.length === 2 &&
        receipts.projects.every((p) => !p.cleaned),
      "Existing active fixture projects required",
    );
  } else {
    for (let i = 0; i < 2; i++) {
      const u = {
        name: "kurier-mcp-" + randomUUID() + "@example.com",
        created: false,
      };
      receipts.users.push(u);
      save();
      aws(["cognito-idp", "admin-create-user"], {
        UserPoolId: o.cognitoUserPoolId,
        Username: u.name,
        MessageAction: "SUPPRESS",
        UserAttributes: [
          { Name: "email", Value: u.name },
          { Name: "email_verified", Value: "true" },
        ],
      });
      u.created = true;
      save();
      live[u.name] = await login(u);
      const me = await expected(
        "GET",
        "/api/v1/users/me",
        live[u.name],
        undefined,
        200,
      );
      u.owner = me.data.data.user.userId;
      save();
    }
  }
  const [owner, other] = receipts.users,
    jwt = live[owner.name],
    foreign = live[other.name];
  if (mode === "--validate") {
    for (let i = 0; i < 2; i++) {
      const r = await expected(
        "POST",
        "/api/v1/projects",
        jwt,
        { name: "Disposable MCP AWS validation " + i },
        201,
      );
      receipts.projects.push({
        id: r.data.data.project.projectId,
        user: owner.name,
      });
      save();
    }
  }
  const [project, excluded] = receipts.projects.map((p) => p.id),
    root = "/api/v1/projects/" + project;
  const protectedCanary = "Bearer " + randomUUID();
  async function request(name, large = false) {
    const body = {
      name,
      method: "POST",
      url: o.controlledEndpointUrl,
      queryParameters: [],
      headers: [
        {
          name: "Authorization",
          bindingId: randomUUID(),
          enabled: true,
          sensitive: true,
          secretWrite: { action: "set", value: protectedCanary },
        },
      ],
      body: large
        ? { type: "text", text: "x".repeat(60000), sensitive: false }
        : null,
      operationRef: null,
    };
    const r = await expected("POST", root + "/requests", jwt, body, 201);
    return r.data.data.request;
  }
  let small, large, workflow, runID, largeID;
  if (mode === "--resume") {
    const r = await expected("GET", root + "/requests", jwt, undefined, 200);
    small = r.data.data.items.find((r) => r.name === "MCP small");
    large = r.data.data.items.find((r) => r.name === "MCP large");
    const w = await expected("GET", root + "/workflows", jwt, undefined, 200);
    workflow = w.data.data.items.find((w) => w.name === "MCP AWS run");
    check(
      small && large && workflow,
      "Fixture definitions incomplete; do not replay setup",
    );
    const runs = await expected(
      "GET",
      root + "/workflow-runs",
      jwt,
      undefined,
      200,
    );
    check(
      runs.data.data.items.length === 1,
      "Exactly one existing fixture run required",
    );
    runID = runs.data.data.items[0].workflowRunId;
    const history = await expected(
      "GET",
      root + "/executions",
      jwt,
      undefined,
      200,
    );
    const standalone = history.data.data.items.filter(
      (e) => e.source.requestId === large.requestId && !e.runId,
    );
    check(
      standalone.length === 1,
      "Exactly one existing large execution required",
    );
    largeID = standalone[0].executionId;
  } else {
    ((small = await request("MCP small")),
      (large = await request("MCP large", true)));
    const wf = await expected(
      "POST",
      root + "/workflows",
      jwt,
      {
        name: "MCP AWS run",
        steps: [small, small].map((r) => ({
          stepId: randomUUID(),
          requestId: r.requestId,
          requestRevision: r.revision,
          extract: [],
          bindings: [],
        })),
      },
      201,
    );
    workflow = wf.data.data.workflow;
    const run = await expected(
      "POST",
      root + "/workflows/" + workflow.workflowId + "/runs",
      jwt,
      { target: "cloud" },
      202,
      {
        "If-Match": '"' + workflow.revision + '"',
        "Idempotency-Key": randomUUID(),
      },
    );
    runID = run.data.data.run.workflowRunId;
    const admission = await expected(
      "POST",
      root + "/requests/" + large.requestId + "/executions",
      jwt,
      {},
      202,
      {
        "If-Match": '"' + large.revision + '"',
        "Idempotency-Key": randomUUID(),
      },
    );
    largeID = admission.data.data.execution.executionId;
  }
  let terminalRun, terminalLarge;
  for (let n = 0; n < 36; n++) {
    const r = await expected(
      "GET",
      root + "/workflow-runs/" + runID,
      jwt,
      undefined,
      200,
    );
    terminalRun = r.data.data.run;
    const e = await expected(
      "GET",
      root + "/executions/" + largeID + "/status",
      jwt,
      undefined,
      200,
    );
    terminalLarge = e.data.data.status;
    if (
      ["completed", "failed"].includes(terminalRun.state) &&
      ["completed", "failed"].includes(terminalLarge.status)
    )
      break;
    await sleep(5000);
  }
  check(
    terminalRun.state === "completed" && terminalLarge.status === "completed",
    "Deployed worker evidence publication not completed",
  );
  const executionID = terminalRun.steps[0].executionId;
  if (mode === "--resume") {
    const current = await expected(
      "GET",
      "/api/v1/mcp/tokens/current",
      jwt,
      undefined,
      200,
    );
    if (current.data.data.credential && !current.data.data.credential.revokedAt)
      await expected(
        "DELETE",
        "/api/v1/mcp/tokens/" + current.data.data.credential.tokenId,
        jwt,
        undefined,
        204,
      );
  }
  const pairKey = randomUUID();
  const pair = await expected(
    "POST",
    "/api/v1/mcp/tokens",
    jwt,
    { projectIds: [project] },
    201,
    { "Idempotency-Key": pairKey },
  );
  const token = pair.data.data.token,
    credential = pair.data.data.credential;
  check(
    typeof token === "string" && token.length === 43,
    "One-time MCP credential missing",
  );
  const retry = await expected(
    "POST",
    "/api/v1/mcp/tokens",
    jwt,
    { projectIds: [project] },
    200,
    { "Idempotency-Key": pairKey },
  );
  check(retry.data.data.token === null, "Issuance retry exposed capability");
  const current = await expected(
    "GET",
    "/api/v1/mcp/tokens/current",
    jwt,
    undefined,
    200,
  );
  check(
    current.data.data.credential.tokenId === credential.tokenId &&
      !current.raw.includes(token),
    "Current metadata mismatch or token disclosure",
  );
  await expected("GET", "/api/v1/users/me", token, undefined, 401);
  await expected("GET", "/api/v1/local-agent/session", token, undefined, 401);
  await expected(
    "POST",
    "/api/v1/mcp/invocations",
    jwt,
    { tool: "list_projects", input: {} },
    401,
  );
  const agentPair = await expected(
    "POST",
    "/api/v1/local-agents",
    jwt,
    {
      name: "Disposable MCP authority separation",
      scopes: [
        {
          projectId: project,
          destinations: [
            { origin: "http://127.0.0.1:59999", addressClass: "loopback" },
          ],
        },
      ],
    },
    201,
    { "Idempotency-Key": randomUUID() },
  );
  check(
    typeof agentPair.data.data.token === "string",
    "Distinct local-agent credential missing",
  );
  await expected(
    "GET",
    "/api/v1/local-agent/session",
    agentPair.data.data.token,
    undefined,
    200,
  );
  await expected(
    "POST",
    "/api/v1/mcp/invocations",
    agentPair.data.data.token,
    { tool: "list_projects", input: {} },
    401,
  );
  await expected("GET", "/api/v1/mcp/tokens/current", token, undefined, 401);
  await expected(
    "DELETE",
    "/api/v1/local-agents/" + agentPair.data.data.agent.localAgentId,
    jwt,
    undefined,
    204,
  );
  await expected("GET", root, foreign, undefined, 404);
  await expected(
    "DELETE",
    "/api/v1/mcp/tokens/" + credential.tokenId,
    foreign,
    undefined,
    404,
  );
  const mcp = async (tool, input = {}, status = 200, capability = token) => {
    await sleep(1300);
    const r = await expected(
      "POST",
      "/api/v1/mcp/invocations",
      capability,
      { tool, input },
      status,
    );
    check(
      !r.raw.includes(protectedCanary) &&
        !r.raw.includes("Bearer ") &&
        !/"(configuration|ciphertext|wrappedKey|secretRef|objectKey|runtimeVariables)"/.test(
          r.raw,
        ),
      "MCP response containment failure",
    );
    return r;
  };
  const projects = await mcp("list_projects");
  check(
    projects.data.data.items.length === 1 &&
      projects.data.data.items[0].projectId === project,
    "Project scope listing failed",
  );
  await mcp("list_saved_requests", { projectId: excluded }, 404);
  const list = await mcp("list_saved_requests", {
    projectId: project,
    limit: 1,
  });
  check(
    list.data.data.items.length === 1 && list.data.data.nextCursor,
    "Bounded pagination missing",
  );
  const page = await mcp("list_saved_requests", {
    projectId: project,
    limit: 1,
    cursor: list.data.data.nextCursor,
  });
  check(
    page.data.data.items.length === 1 &&
      page.data.data.items[0].requestId !== list.data.data.items[0].requestId,
    "Continuation repeated item",
  );
  await mcp("list_executions", { projectId: project });
  await mcp("get_execution", { projectId: project, executionId: executionID });
  await mcp("list_workflow_runs", { projectId: project });
  await mcp("get_workflow_run", { projectId: project, workflowRunId: runID });
  const ordinary = await expected(
    "GET",
    root + "/executions/" + executionID + "/evidence",
    jwt,
    undefined,
    200,
  );
  const before = query(
    o.controlTable,
    "P#" + project,
    "EXEC#" + executionID + "#SNAP",
  );
  check(before.length === 1, "Published snapshot missing");
  const complete = await mcp("get_execution_evidence", {
    projectId: project,
    executionId: executionID,
    view: "complete",
  });
  check(
    hash(ordinary.data.data.evidence) ===
      hash(complete.data.data.evidence.capture),
    "MCP capture differs from immutable published evidence",
  );
  check(
    Buffer.byteLength(complete.raw) <= 262144,
    "Hosted evidence response too large",
  );
  const tooLarge = await mcp(
    "get_execution_evidence",
    { projectId: project, executionId: largeID, view: "complete" },
    413,
  );
  check(
    tooLarge.data.error.code === "evidence_too_large",
    "Missing explicit evidence size error",
  );
  const summary = await mcp("get_execution_evidence", {
    projectId: project,
    executionId: largeID,
  });
  check(
    !summary.data.data.evidence.complete &&
      summary.data.data.evidence.omissionReason &&
      summary.data.data.evidence.omittedFields.length,
    "Summary omission not explicit",
  );
  check(
    hash(before) ===
      hash(
        query(o.controlTable, "P#" + project, "EXEC#" + executionID + "#SNAP"),
      ),
    "Evidence manifest changed during MCP reads",
  );
  const snap = before[0];
  const object = awsProcess([
    "s3api",
    "get-object",
    "--bucket",
    o.evidenceBucket,
    "--key",
    snap.objectKey.S,
    "/dev/stdout",
    "--query",
    "to_array(ContentLength)[?@ < `0`]",
  ]);
  const objectBytes = object.stdout?.subarray(0, Number(snap.bytes.N));
  check(
    object.status === 0 &&
      objectBytes.length === Number(snap.bytes.N) &&
      createHash("sha256").update(objectBytes).digest("hex") ===
        snap.checksum.S,
    "S3 evidence checksum validation failed",
  );
  const audit = query(o.controlTable, "P#" + project, "AUDIT#"),
    accountAudit = query(o.controlTable, "U#" + owner.owner, "AUDIT#");
  check(
    audit.length >= 7 &&
      audit.every((r) => r.kind.S === "executionMCPAudit") &&
      accountAudit.some((r) => r.mcpCode?.S === "not_found") &&
      accountAudit.some((r) => r.mcpCode?.S === "evidence_too_large"),
    "Persistent read/denial audit missing",
  );
  check(
    !JSON.stringify([...audit, ...accountAudit]).includes(token) &&
      !JSON.stringify([...audit, ...accountAudit]).includes(protectedCanary),
    "Audit contained credential values",
  );
  await expected(
    "DELETE",
    "/api/v1/mcp/tokens/" + credential.tokenId,
    jwt,
    undefined,
    204,
  );
  await mcp("list_projects", {}, 401);
  const next = await expected(
    "POST",
    "/api/v1/mcp/tokens",
    jwt,
    { projectIds: [project] },
    201,
    { "Idempotency-Key": randomUUID() },
  );
  const nextToken = next.data.data.token;
  const detail = await expected("GET", root, jwt, undefined, 200);
  const deletion = await expected("DELETE", root, jwt, undefined, 202, {
    "If-Match": detail.headers.get("etag"),
  });
  receipts.projects[0].operation =
    deletion.data.data.deletionOperation.operationId;
  save();
  await mcp(
    "get_execution_evidence",
    { projectId: project, executionId: executionID },
    404,
    nextToken,
  );
  const results = {
    genuineCognitoSRP: true,
    distinctUsers: true,
    oneTimeIssuance: true,
    authenticationSeparated: true,
    localAgentCredentialRejected: true,
    sevenReadTools: true,
    scopedPagination: true,
    auditRecords: audit.length + accountAudit.length,
    evidenceIntegrity: true,
    s3ChecksumVerified: true,
    complete96KiBLimit: true,
    explicitSummary: true,
    revokedTokenDenied: true,
    deletingProjectDenied: true,
    operatorBrowserOrTUI: false,
  };
  receipts.validation = results;
  save();
  await cleanup();
  console.log(
    JSON.stringify({
      ...results,
      scheduledProjectCleanup: true,
      protectedAndS3Empty: true,
      disposableCognitoUsersDeleted: true,
      receipt: path,
    }),
  );
}
main().catch((e) => {
  const safe =
    e instanceof ValidationFailure ||
    (e instanceof Error && e.message.startsWith("Scoped AWS operation failed"));
  console.error(
    "MCP AWS validation/cleanup incomplete: " +
      (safe
        ? e.message
        : "inspect fixture receipt and bounded checks before resuming"),
  );
  process.exitCode = 1;
});
