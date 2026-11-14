// Scoped MCP2 release. Never retries an uncertain marker or deployment write.
import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { pathToFileURL } from "node:url";
import {
  aws,
  identity,
  marker,
  snapshot,
  hash,
  outputs as o,
} from "./verify-read-only-mcp-deployment.mjs";
import {
  checkTuple,
  transitionInput,
  planSummary,
  checkPlan,
} from "./lib/mcp-scopes-release-guards.mjs";
const basePath = ".sst/mcp-scopes-before.json";
const planPath = ".sst/mcp-scopes-diff.txt";
function inventory() {
  const counts = {};
  const accounting = [];
  for (const table of [o.controlTable, o.protectedTable]) {
    let cursor;
    for (let page = 0; page < 1000; page++) {
      const fields = [
        "PK",
        "SK",
        "kind",
        "state",
        "inflightCount",
        "storageReservedBytes",
        "writeSettled",
      ];
      const names = Object.fromEntries(fields.map((k, i) => ["#p" + i, k]));
      const q = aws(["dynamodb", "scan"], {
        TableName: table,
        ConsistentRead: true,
        Limit: 100,
        ProjectionExpression: Object.keys(names).join(","),
        ExpressionAttributeNames: names,
        ...(cursor ? { ExclusiveStartKey: cursor } : {}),
      });
      for (const r of q.Items ?? []) {
        const k = r.kind?.S ?? "unknown";
        counts[k] = (counts[k] ?? 0) + 1;
        const state = r.state?.S;
        if (["project", "executionQuota"].includes(k)) accounting.push(r);
        if (
          Number(r.inflightCount?.N ?? 0) !== 0 ||
          (k === "project" && state !== "active") ||
          (["executionJob", "executionWorkflowRun"].includes(k) &&
            !["completed", "failed", "deleted"].includes(state)) ||
          [
            "executionLocalWait",
            "executionWorkflowInputs",
            "executionWorkflowOutput",
            "deletionWork",
          ].includes(k) ||
          (k === "executionOutbox" && !["published", "sent"].includes(state)) ||
          (k === "executionUpload" &&
            (!["published", "committed", "deleted"].includes(state) ||
              r.writeSettled?.BOOL !== true))
        )
          throw Error("Unsettled work blocks scoped migration: " + k);
      }
      cursor = q.LastEvaluatedKey;
      if (!cursor) break;
      if (page === 999) throw Error("Incomplete bounded inventory");
    }
  }
  return {
    counts,
    accountingHash: hash(
      accounting.sort((a, b) =>
        (a.PK.S + "/" + a.SK.S).localeCompare(b.PK.S + "/" + b.SK.S),
      ),
    ),
  };
}
function bootstrap() {
  const p = aws(["ssm", "get-parameter", "--name", "/sst/bootstrap"]).Parameter;
  const b = JSON.parse(p.Value);
  assert.equal(b.version, 5);
  assert.equal(b.state, "sst-state-moshrdrrbeba");
  assert.equal(b.asset, "sst-asset-moshrdrrbeba");
  assert.equal(b.assetEcrRegistryId, "747336059622");
  return { ...b, parameterVersion: p.Version };
}
function sst(operation) {
  const c = aws(["configure", "export-credentials", "--format", "process"]);
  const r = spawnSync(
    "npm",
    [
      "exec",
      "--",
      "sst",
      operation,
      "--stage",
      "dev-api",
      ...(operation === "diff" ? ["--json"] : []),
    ],
    {
      encoding: "utf8",
      maxBuffer: 64 * 1024 * 1024,
      env: {
        ...process.env,
        AWS_REGION: "us-east-2",
        AWS_ACCESS_KEY_ID: c.AccessKeyId,
        AWS_SECRET_ACCESS_KEY: c.SecretAccessKey,
        AWS_SESSION_TOKEN: c.SessionToken,
        KURIER_AWS_ACCOUNT_ID: "747336059622",
        KURIER_COGNITO_POOL_ID: o.cognitoUserPoolId,
        KURIER_COGNITO_CLIENT_ID: o.cognitoClientId,
        KURIER_FRONTEND_ORIGIN: o.frontendOrigin,
        SST_TELEMETRY_DISABLED: "1",
      },
    },
  );
  writeFileSync(
    operation === "diff" ? planPath : ".sst/mcp-scopes-deploy.txt",
    r.stdout + r.stderr,
  );
  {
    const fmt = spawnSync(
      "npm",
      [
        "exec",
        "--",
        "prettier",
        "--write",
        "sst-env.d.ts",
        "apps/web/sst-env.d.ts",
      ],
      { stdio: "pipe" },
    );
    assert.equal(fmt.status, 0);
  }
  if (r.status !== 0)
    throw Error(
      "Scoped SST " +
        operation +
        " failed; inspect ignored log, no automatic retry",
    );
}
function verifyDeployed(before) {
  const after = snapshot();
  assert.deepEqual(after.controls, before.snapshot.controls);
  for (const f of after.functions) {
    const old = before.snapshot.functions.find((x) => x.name === f.name);
    assert.equal(f.iamHash, old.iamHash);
    for (const k of [
      "runtime",
      "architecture",
      "memory",
      "timeout",
      "reserved",
      "configurationHash",
    ])
      assert.deepEqual(f[k], old[k]);
    assert.equal(f.state, "Active");
    assert.equal(f.update, "Successful");
    if (
      [
        "UsersProjectsApi",
        "EmptyProjectCleanup",
        "CloudExecutionWorker",
      ].includes(f.resource)
    ) {
      assert.equal(f.mcp, "2");
      const code = readFileSync(".sst/artifacts/" + f.resource + "/code.zip");
      assert.equal(
        f.codeSha256,
        createHash("sha256").update(code).digest("base64"),
      );
    }
  }
  for (const route of [
    "POST /api/v1/mcp/connections",
    "GET /api/v1/mcp/connections/current",
    "PATCH /api/v1/mcp/connections/{tokenId}/scope",
  ]) {
    const u = new URL(o.apiUrl);
    const routes = aws([
      "apigatewayv2",
      "get-routes",
      "--api-id",
      u.hostname.split(".")[0],
    ]);
    assert.ok(!routes.NextToken);
    assert.ok(routes.Items.some((r) => r.RouteKey === route));
  }
  const broker = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    o.browserSessionFunction,
  ]);
  assert.equal(
    hash(broker.Environment?.Variables ?? {}),
    before.broker.environmentHash,
  );
  assert.equal(broker.Role, before.broker.role);
  assert.equal(
    broker.CodeSha256,
    createHash("sha256")
      .update(readFileSync(".sst/artifacts/BrowserSessionBroker/code.zip"))
      .digest("base64"),
  );
  assert.deepEqual(bootstrap(), before.bootstrap);
  return after;
}
async function main() {
  const mode = process.argv[2];
  assert.ok(
    ["preview", "preview-closed", "close", "deploy", "activate"].includes(mode),
  );
  identity();
  assert.equal(
    spawnSync("git", ["status", "--porcelain"], {
      encoding: "utf8",
    }).stdout.trim(),
    "",
    "Clean pinned source required",
  );
  bootstrap();
  if (mode === "preview" || mode === "preview-closed") {
    const m = marker();
    checkTuple(
      m,
      mode === "preview" ? "active" : "mcp-scopes-migration",
      10,
      1,
    );
    const counts = inventory();
    const previous = JSON.parse(readFileSync(basePath));
    assert.equal(
      hash(m),
      hash(
        mode === "preview"
          ? previous.marker
          : JSON.parse(readFileSync(".sst/mcp-scopes-closed.json")),
      ),
    );
    sst("diff");
    const changes = planSummary(readFileSync(planPath, "utf8"));
    checkPlan(changes);
    writeFileSync(
      ".sst/mcp-scopes-plan.json",
      JSON.stringify(
        {
          changes,
          counts,
          markerHash: hash(m),
          createdAt: new Date().toISOString(),
          source: spawnSync("git", ["rev-parse", "HEAD"], {
            encoding: "utf8",
          }).stdout.trim(),
        },
        null,
        2,
      ),
    );
    console.log(JSON.stringify({ changes, counts }));
    return;
  }
  const before = JSON.parse(readFileSync(basePath));
  const plan = JSON.parse(readFileSync(".sst/mcp-scopes-plan.json"));
  assert.equal(
    plan.source,
    spawnSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).stdout.trim(),
    "Source changed since preview",
  );
  assert.ok(
    Date.now() - Date.parse(plan.createdAt) < 60 * 60 * 1000,
    "Fresh preview required",
  );
  assert.deepEqual(bootstrap(), before.bootstrap);
  checkPlan(plan.changes);
  if (mode === "close") {
    const m = marker();
    assert.equal(hash(m), hash(before.marker));
    assert.equal(inventory().accountingHash, plan.counts.accountingHash);
    aws(["dynamodb", "update-item"], transitionInput(m, false, o.controlTable));
    const closed = marker();
    checkTuple(closed, "mcp-scopes-migration", 10, 1);
    assert.equal(Number(closed.version.N), Number(m.version.N) + 1);
    writeFileSync(".sst/mcp-scopes-closed.json", JSON.stringify(closed));
    console.log(JSON.stringify({ closed: true, version: closed.version.N }));
    return;
  }
  const closed = JSON.parse(readFileSync(".sst/mcp-scopes-closed.json"));
  assert.equal(hash(marker()), hash(closed));
  assert.equal(inventory().accountingHash, plan.counts.accountingHash);
  if (mode === "deploy") {
    sst("deploy");
    const deployed = verifyDeployed(before);
    writeFileSync(
      ".sst/mcp-scopes-deployed.json",
      JSON.stringify(deployed, null, 2),
    );
    console.log(
      JSON.stringify({
        deployed: true,
        stageStillClosed: true,
        functions: deployed.functions.map((f) => ({
          name: f.name,
          codeSha256: f.codeSha256,
        })),
      }),
    );
    return;
  }
  verifyDeployed(before);
  aws(
    ["dynamodb", "update-item"],
    transitionInput(closed, true, o.controlTable),
  );
  const active = marker();
  checkTuple(active, "active", 11, 2);
  assert.equal(Number(active.version.N), Number(closed.version.N) + 1);
  writeFileSync(".sst/mcp-scopes-active.json", JSON.stringify(active, null, 2));
  console.log(
    JSON.stringify({
      active: true,
      version: active.version.N,
      generation: active.recoveryGeneration.S,
    }),
  );
}
if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href)
  main().catch((e) => {
    console.error(e.message);
    console.error(
      "MCP scopes release stopped; inspect ignored scoped artifacts before continuing. No automatic reopen or replay.",
    );
    process.exitCode = 1;
  });
