// Live, bounded two-owner test. Secret material stays in memory except an
// explicitly private disposable MCP pairing file; no traces or screenshots.
import assert from "node:assert/strict";
import { randomUUID, randomBytes, createHash } from "node:crypto";
import {
  readFileSync,
  writeFileSync,
  mkdtempSync,
  mkdirSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { createInterface } from "node:readline";
import { Amplify } from "aws-amplify";
import { signIn, signOut, fetchAuthSession } from "aws-amplify/auth";
import { cognitoUserPoolsTokenProvider } from "aws-amplify/auth/cognito";
import { sharedInMemoryStorage } from "aws-amplify/utils";
import { chromium } from "@playwright/test";
import {
  aws,
  identity,
  marker,
  outputs as o,
} from "./verify-read-only-mcp-deployment.mjs";
const mode = process.argv[2],
  path = process.argv[3];
assert.ok(["--validate", "--cleanup"].includes(mode) && path);
const receipt =
  mode === "--cleanup"
    ? JSON.parse(readFileSync(path))
    : {
        kind: "kurier-mcp-scopes-live-v1",
        startedAt: new Date().toISOString(),
        users: [],
        projects: [],
        checks: [],
      };
assert.equal(receipt.kind, "kurier-mcp-scopes-live-v1");
const save = () =>
  writeFileSync(path, JSON.stringify(receipt, null, 2) + "\n", { mode: 0o600 });
if (mode === "--validate")
  writeFileSync(path, JSON.stringify(receipt), { mode: 0o600, flag: "wx" });
const ok = (name) => {
  receipt.checks.push(name);
  save();
  console.log(JSON.stringify({ passed: name }));
};
Amplify.configure({
  Auth: {
    Cognito: {
      userPoolId: o.cognitoUserPoolId,
      userPoolClientId: o.cognitoClientId,
    },
  },
});
cognitoUserPoolsTokenProvider.setKeyValueStorage(sharedInMemoryStorage);
const sessions = new Map();
const passwords = new Map();
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
async function login(u) {
  const password = "Kurier1!" + randomBytes(24).toString("base64url");
  passwords.set(u.name, password);
  aws(["cognito-idp", "admin-set-user-password"], {
    UserPoolId: o.cognitoUserPoolId,
    Username: u.name,
    Password: password,
    Permanent: true,
  });
  await signOut();
  assert.ok(
    (
      await signIn({
        username: u.name,
        password,
        options: { authFlowType: "USER_SRP_AUTH" },
      })
    ).isSignedIn,
  );
  const s = await fetchAuthSession();
  sessions.set(u.name, s.tokens.accessToken.toString());
  u.subject = s.tokens.accessToken.payload.sub;
  save();
}
async function call(method, route, token, body, status, headers = {}) {
  const r = await fetch(o.apiUrl + route, {
    method,
    headers: {
      ...(token ? { Authorization: "Bearer " + token } : {}),
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...headers,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    redirect: "error",
    signal: AbortSignal.timeout(15000),
  });
  let data;
  try {
    data = r.status === 204 ? null : await r.json();
  } catch {
    throw Error("Non-JSON live response");
  }
  if (r.status !== status)
    throw Error(
      "Live status " +
        method +
        " " +
        route.split("/").slice(0, 5).join("/") +
        " expected" +
        status +
        " got" +
        r.status,
    );
  return { data: data?.data, headers: r.headers };
}
async function project(u) {
  const r = await call(
    "POST",
    "/api/v1/projects",
    sessions.get(u.name),
    { name: "MCP scopes disposable" },
    201,
  );
  const p = { id: r.data.project.projectId, user: u.name };
  receipt.projects.push(p);
  save();
  return p;
}
async function remove(p) {
  if (p.deleting) return;
  const r = await call(
    "GET",
    "/api/v1/projects/" + p.id,
    sessions.get(p.user),
    undefined,
    200,
  );
  await call(
    "DELETE",
    "/api/v1/projects/" + p.id,
    sessions.get(p.user),
    undefined,
    202,
    { "If-Match": r.headers.get("etag") },
  );
  p.deleting = true;
  save();
}
function metadata(table, pk) {
  const r = aws(["dynamodb", "query"], {
    TableName: table,
    KeyConditionExpression: "PK = :p",
    ExpressionAttributeValues: { ":p": { S: pk } },
    ProjectionExpression: "PK,SK,#k,#s,ownerId,userId,mcpTokenId",
    ExpressionAttributeNames: { "#k": "kind", "#s": "state" },
    ConsistentRead: true,
  });
  assert.ok(!r.LastEvaluatedKey);
  return r.Items ?? [];
}
async function cleanup() {
  for (const u of receipt.users)
    if (u.created && !u.deleted && !sessions.has(u.name)) await login(u);
  for (const u of receipt.users) {
    if (u.deleted) continue;
    const token = sessions.get(u.name);
    if (token) {
      const r = await call(
        "GET",
        "/api/v1/mcp/connections/current",
        token,
        undefined,
        200,
      );
      if (r.data.connection && !r.data.connection.revokedAt)
        await call(
          "DELETE",
          "/api/v1/mcp/tokens/" + r.data.connection.tokenId,
          token,
          undefined,
          204,
        );
    }
  }
  for (const p of receipt.projects) if (!p.cleaned) await remove(p);
  for (let n = 0; n < 180; n++) {
    let done = true;
    for (const p of receipt.projects) {
      if (p.cleaned) continue;
      const rows = metadata(o.controlTable, "P#" + p.id);
      if (
        rows.length === 1 &&
        rows[0].kind?.S === "projectTombstone" &&
        rows[0].state?.S === "deleted"
      ) {
        assert.equal(metadata(o.protectedTable, "P#" + p.id).length, 0);
        p.cleaned = true;
        save();
      } else done = false;
    }
    if (done) break;
    await sleep(5000);
  }
  assert.ok(
    receipt.projects.every((p) => p.cleaned),
    "Scheduled fixture cleanup incomplete; use --cleanup receipt",
  );
  for (const u of receipt.users) {
    if (!u.created || u.deleted) continue;
    if (u.owner) {
      const rows = metadata(o.controlTable, "U#" + u.owner);
      for (const r of rows) {
        if (r.kind?.S === "executionQuota") continue;
        assert.ok(
          [
            "user",
            "mcpCredential",
            "mcpSlot",
            "mcpIssuance",
            "mcpRate",
            "mcpAudit",
          ].includes(r.kind?.S),
          "Unexpected fixture owner row",
        );
        if (r.kind?.S === "mcpCredential") {
          assert.equal(r.state?.S, "revoked");
          const lookup = aws(["dynamodb", "scan"], {
            TableName: o.controlTable,
            FilterExpression: "#k = :k AND ownerId = :u AND mcpTokenId = :t",
            ExpressionAttributeNames: { "#k": "kind" },
            ExpressionAttributeValues: {
              ":k": { S: "mcpLookup" },
              ":u": { S: u.owner },
              ":t": r.mcpTokenId,
            },
            ProjectionExpression: "PK,SK",
          });
          assert.ok(!lookup.LastEvaluatedKey);
          for (const l of lookup.Items ?? [])
            aws(["dynamodb", "delete-item"], {
              TableName: o.controlTable,
              Key: { PK: l.PK, SK: l.SK },
              ConditionExpression: "ownerId = :u",
              ExpressionAttributeValues: { ":u": { S: u.owner } },
            });
        }
        aws(["dynamodb", "delete-item"], {
          TableName: o.controlTable,
          Key: { PK: r.PK, SK: r.SK },
          ConditionExpression:
            r.kind?.S === "user" ? "userId = :u" : "ownerId = :u",
          ExpressionAttributeValues: { ":u": { S: u.owner } },
        });
      }
      const h = createHash("sha256");
      for (const value of [o.cognitoIssuer, u.subject]) {
        const b = Buffer.from(value);
        const n = Buffer.alloc(4);
        n.writeUInt32BE(b.length);
        h.update(n).update(b);
      }
      aws(["dynamodb", "delete-item"], {
        TableName: o.controlTable,
        Key: { PK: { S: "IDENTITY#" + h.digest("hex") }, SK: { S: "META" } },
        ConditionExpression: "userId = :u",
        ExpressionAttributeValues: { ":u": { S: u.owner } },
      });
      assert.ok(
        metadata(o.controlTable, "U#" + u.owner).every(
          (r) => r.kind?.S === "executionQuota",
        ),
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
  receipt.cleaned = true;
  save();
  ok("disposable fixture cleanup verified");
}
async function main() {
  identity();
  assert.equal(marker().savedRequestsSchemaVersion.N, "11");
  assert.equal(marker().mcpReadOnlySchemaVersion.N, "2");
  if (mode === "--cleanup") {
    await cleanup();
    return;
  }
  for (let n = 0; n < 2; n++) {
    const u = {
      name: "kurier-mcp-scopes-" + randomUUID() + "@example.com",
      created: false,
    };
    receipt.users.push(u);
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
    await login(u);
    const me = await call(
      "GET",
      "/api/v1/users/me",
      sessions.get(u.name),
      undefined,
      200,
    );
    u.owner = me.data.user.userId;
    save();
  }
  const [owner, other] = receipt.users;
  const jwt = sessions.get(owner.name);
  const foreign = await project(other);
  // Genuine hosted browser consent and SRP, with disposable credentials only.
  const browser = await chromium.launch({
    headless: true,
    ...(process.env.HTTPS_PROXY
      ? { proxy: { server: process.env.HTTPS_PROXY } }
      : {}),
  });
  let cap, connection;
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto(o.frontendHosting.siteUrl, { waitUntil: "networkidle" });
    await page.getByLabel("Email", { exact: true }).fill(owner.name);
    await page
      .getByLabel("Password", { exact: true })
      .fill(passwords.get(owner.name));
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    const panel = page.getByRole("region", { name: "Read-only MCP access" });
    await panel.getByRole("button", { name: "Refresh MCP status" }).click();
    await panel
      .getByLabel("All my projects, including future projects")
      .check();
    await panel.getByLabel(/^I authorize/).check();
    assert.ok(
      await panel
        .getByRole("button", { name: "Create MCP credential" })
        .isDisabled(),
    );
    await panel.getByLabel(/^I understand/).check();
    await panel.getByRole("button", { name: "Create MCP credential" }).click();
    cap = await panel.getByLabel("One-time MCP token").innerText();
    assert.equal(cap.length, 43);
    assert.equal(
      await page.evaluate(
        (t) =>
          [
            ...Object.values(localStorage),
            ...Object.values(sessionStorage),
          ].some((x) => x.includes(t)),
        cap,
      ),
      false,
    );
    await panel
      .getByRole("button", { name: "Close MCP token display" })
      .click();
    connection = (
      await call("GET", "/api/v1/mcp/connections/current", jwt, undefined, 200)
    ).data.connection;
    await context.close();
    ok("real hosted Chromium/Cognito automatic consent and one-time token");
  } finally {
    await browser.close();
  }
  const dir = mkdtempSync(join(tmpdir(), "kurier-mcp-scopes-live-"));
  mkdirSync(join(dir, "private"), { mode: 0o700 });
  const credential = join(dir, "private", "credential.json");
  writeFileSync(
    credential,
    JSON.stringify({ apiOrigin: o.apiUrl, token: cap }),
    { mode: 0o600 },
  );
  const binary = join(dir, "kurier-mcp");
  assert.equal(
    spawnSync("go", ["-C", "services/mcp", "build", "-o", binary, "."], {
      stdio: "pipe",
    }).status,
    0,
  );
  const config =
    "mcp_servers={kurier={command=" +
    JSON.stringify(binary) +
    ',args=["serve","--credential-file",' +
    JSON.stringify(credential) +
    "],startup_timeout_sec=15,tool_timeout_sec=20}}";
  const client = spawn(
    "codex",
    ["app-server", "--stdio", "-c", config, "-c", 'history.persistence="none"'],
    { stdio: ["pipe", "pipe", "pipe"] },
  );
  let sequence = 0;
  const pending = new Map();
  let diagnosticLeak = false;
  client.stderr.on("data", (b) => {
    if (b.toString().includes(cap)) diagnosticLeak = true;
  });
  createInterface({ input: client.stdout }).on("line", (line) => {
    try {
      const m = JSON.parse(line);
      const p = pending.get(m.id);
      if (p) {
        pending.delete(m.id);
        m.error
          ? p.reject(Error("Installed client rejected RPC"))
          : p.resolve(m.result);
      }
    } catch {}
  });
  const rpc = (method, params) =>
    new Promise((resolve, reject) => {
      const id = ++sequence;
      const timer = setTimeout(() => {
        pending.delete(id);
        reject(Error("Installed client timeout"));
      }, 30000);
      pending.set(id, {
        resolve: (r) => {
          clearTimeout(timer);
          resolve(r);
        },
        reject: (e) => {
          clearTimeout(timer);
          reject(e);
        },
      });
      client.stdin.write(JSON.stringify({ id, method, params }) + "\n");
    });
  try {
    await rpc("initialize", {
      clientInfo: { name: "kurier-live-scope-validation", version: "1" },
      capabilities: { experimentalApi: true },
    });
    client.stdin.write(
      JSON.stringify({ method: "initialized", params: {} }) + "\n",
    );
    const t = await rpc("thread/start", {
      cwd: dir,
      ephemeral: true,
      approvalPolicy: "never",
      sandbox: "read-only",
    });
    const threadId = t.thread.id;
    const status = await rpc("mcpServerStatus/list", {
      threadId,
      serverName: "kurier",
    });
    assert.equal(Object.keys(status.data[0].tools).length, 7);
    const tool = async (name, args = {}) => {
      await sleep(1100);
      const r = await rpc("mcpServer/tool/call", {
        threadId,
        server: "kurier",
        tool: name,
        arguments: args,
      });
      const env =
        typeof r.structuredContent === "string"
          ? JSON.parse(r.structuredContent)
          : r.structuredContent;
      assert.ok(env && !JSON.stringify(env).includes(cap));
      return env;
    };
    const empty = await tool("list_projects");
    assert.equal(empty.data.items.length, 0);
    ok("installed Codex empty automatic account");
    const owned = await project(owner);
    for (let n = 0; n < 12; n++) {
      const r = await tool("list_projects", { limit: 1 });
      if (r.data?.items.some((p) => p.projectId === owned.id)) break;
      assert.ok(n < 11, "New project not discovered");
      await sleep(2000);
    }
    ok("same installed connection discovers new project");
    const second = await project(owner);
    await sleep(2000);
    let list = await tool("list_projects", { limit: 1 });
    assert.equal(list.data.items.length, 1);
    assert.ok(list.data.nextCursor);
    const stale = list.data.nextCursor;
    await call(
      "POST",
      "/api/v1/mcp/invocations",
      cap,
      { tool: "list_saved_requests", input: { projectId: foreign.id } },
      404,
    );
    connection = (
      await call(
        "PATCH",
        "/api/v1/mcp/connections/" + connection.tokenId + "/scope",
        jwt,
        { scope: { mode: "selected_projects", projectIds: [owned.id] } },
        200,
        { "If-Match": '"' + connection.version + '"' },
      )
    ).data.connection;
    assert.equal(
      (await tool("list_projects", { cursor: stale })).error.code,
      "invalid_request",
    );
    list = await tool("list_projects");
    assert.deepEqual(
      list.data.items.map((p) => p.projectId),
      [owned.id],
    );
    assert.equal(
      (await tool("list_saved_requests", { projectId: second.id })).error.code,
      "not_found",
    );
    ok("same installed connection narrows scope and invalidates cursor");
    await call("GET", "/api/v1/mcp/tokens/current", jwt, undefined, 409);
    await call(
      "PATCH",
      "/api/v1/mcp/connections/" + connection.tokenId + "/scope",
      jwt,
      { scope: { mode: "all_owned_projects" } },
      412,
      { "If-Match": '"0"' },
    );
    const expiry = connection.expiresAt;
    connection = (
      await call(
        "PATCH",
        "/api/v1/mcp/connections/" + connection.tokenId + "/scope",
        jwt,
        { scope: { mode: "all_owned_projects" } },
        200,
        { "If-Match": '"' + connection.version + '"' },
      )
    ).data.connection;
    assert.equal(connection.expiresAt, expiry);
    await remove(owned);
    assert.equal(
      (await tool("list_saved_requests", { projectId: owned.id })).error.code,
      "not_found",
    );
    ok("automatic deletion denies subsequent reads and preserves expiry");
    await call(
      "DELETE",
      "/api/v1/mcp/tokens/" + connection.tokenId,
      jwt,
      undefined,
      204,
    );
    assert.equal((await tool("list_projects")).error.code, "unauthenticated");
    assert.equal(diagnosticLeak, false);
    ok("installed connection immediately denies revoked credential");
  } finally {
    client.stdin.end();
    client.kill();
    rmSync(dir, { recursive: true, force: true });
  }
  await cleanup();
}
main().catch(() => {
  console.error(
    "Live scope validation stopped; use identifier-only receipt to reconcile and clean up. Secret responses are intentionally not printed.",
  );
  process.exitCode = 1;
});
