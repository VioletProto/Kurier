// Read-only inventory and activation plan. No deploy/activate/mutation mode.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
if (process.argv.length !== 2)
  throw Error("Read-only preview accepts no arguments.");
const out = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args, region = "us-east-2") {
  const r = spawnSync(
    "aws",
    [...args, "--region", region, "--output", "json"],
    { encoding: "utf8", maxBuffer: 16 * 1024 * 1024 },
  );
  if (r.status !== 0)
    throw Error(`Read-only lookup failed: ${args.slice(0, 2).join(" ")}`);
  return r.stdout.trim() ? JSON.parse(r.stdout) : {};
}
const identity = aws(["sts", "get-caller-identity"]);
if (
  identity.Account !== "747336059622" ||
  identity.Arn.endsWith(":root") ||
  out.stage !== "dev-api" ||
  out.region !== "us-east-2"
)
  throw Error("Intended non-root account/stage required.");
const stage = aws([
  "dynamodb",
  "get-item",
  "--table-name",
  out.controlTable,
  "--key",
  JSON.stringify({ PK: { S: "STAGE#dev-api" }, SK: { S: "META" } }),
  "--consistent-read",
]).Item;
if (
  stage?.state?.S !== "active" ||
  stage.savedRequestsSchemaVersion?.N !== "6" ||
  stage.sequentialWorkflowsSchemaVersion?.N !== "1" ||
  stage.localAgentExecutionsSchemaVersion?.N !== "1" ||
  stage.evidencePinningSchemaVersion?.N !== "1" ||
  stage.mcpReadOnlySchemaVersion ||
  !stage.recoveryGeneration?.S
)
  throw Error("Unexpected pre-MCP marker; review required.");
const bootstrap = aws(["ssm", "get-parameter", "--name", "/sst/bootstrap"]);
if (!bootstrap.Parameter?.Value)
  throw Error("Existing bootstrap not verified; refuse implicit bootstrap.");
const counts = {
  projects: 0,
  queued: 0,
  running: 0,
  unsettledUploads: 0,
  mcpRecords: 0,
};
for (const table of [out.controlTable, out.protectedTable]) {
  let cursor;
  let complete = false;
  for (let page = 0; page < 1000; page++) {
    const args = [
      "dynamodb",
      "scan",
      "--table-name",
      table,
      "--consistent-read",
      "--limit",
      "100",
      "--no-paginate",
      "--projection-expression",
      "PK, SK, #k, #s, pinnedCount, pinnedBytes, writeSettled",
      "--expression-attribute-names",
      JSON.stringify({ "#k": "kind", "#s": "state" }),
    ];
    if (cursor) args.push("--exclusive-start-key", JSON.stringify(cursor));
    const r = aws(args);
    for (const x of r.Items ?? []) {
      const kind = x.kind?.S ?? "";
      if (kind === "project") {
        counts.projects++;
        if (!x.pinnedCount?.N || !x.pinnedBytes?.N)
          throw Error("Missing preserved pin counters");
      }
      if (kind === "executionJob") {
        if (x.state?.S === "queued") counts.queued++;
        if (["running", "claimed"].includes(x.state?.S)) counts.running++;
      }
      if (kind === "executionUpload" && x.state?.S !== "published")
        counts.unsettledUploads++;
      if (kind.startsWith("mcp") || kind.includes("MCP")) counts.mcpRecords++;
    }
    cursor = r.LastEvaluatedKey;
    if (!cursor) {
      complete = true;
      break;
    }
  }
  if (!complete) throw Error("Incomplete bounded inventory");
}
const consumers = [
  out.apiFunction,
  out.cleanupFunction,
  out.workerFunction,
].map((name) => {
  const f = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    name,
  ]);
  return {
    name,
    state: f.State,
    update: f.LastUpdateStatus,
    codeSha256: f.CodeSha256,
    memory: f.MemorySize,
    timeout: f.Timeout,
    architecture: f.Architectures,
    mcpCapable:
      f.Environment?.Variables?.KURIER_MCP_READ_ONLY_SCHEMA_VERSION === "1",
  };
});
const account = aws(["lambda", "get-account-settings"]);
const plan = aws(["freetier", "get-account-plan-state"]);
const maps = aws([
  "lambda",
  "list-event-source-mappings",
  "--function-name",
  out.workerFunction,
]);
if (maps.NextMarker) throw Error("Incomplete mapping inventory");
const queue = maps.EventSourceMappings.map((m) => ({
  state: m.State,
  batch: m.BatchSize,
  max: m.ScalingConfig?.MaximumConcurrency,
  provisionedPollers: m.ProvisionedPollerConfig ?? null,
}));
const reserved = consumers.map((f) => ({
  name: f.name,
  reserved:
    aws(["lambda", "get-function-concurrency", "--function-name", f.name])
      .ReservedConcurrentExecutions ?? null,
}));
if (
  plan.accountPlanType !== "FREE" ||
  plan.accountPlanStatus !== "ACTIVE" ||
  account.AccountLimit?.ConcurrentExecutions !== 10 ||
  queue.some((m) => m.max !== 2 || m.batch !== 1 || m.provisionedPollers)
)
  throw Error("Plan/quota/concurrency baseline changed; do not mutate.");
console.log(
  JSON.stringify(
    {
      checkedAt: new Date().toISOString(),
      preview: true,
      account: identity.Account,
      nonRoot: true,
      stageVersion: stage.version.N,
      generation: stage.recoveryGeneration.S,
      counts,
      consumers,
      plan: { type: plan.accountPlanType, status: plan.accountPlanStatus },
      accountLimit: account.AccountLimit.ConcurrentExecutions,
      unreserved: account.AccountLimit.UnreservedConcurrentExecutions,
      queue,
      reserved,
      plannedMarkerWrites: [
        {
          from: "active",
          to: "mcp-migration",
          changes: ["state", "version"],
          conditions:
            "exact version/generation, savedRequests6/workflow1/local1/pinning1/cloud1; no active jobs or unsettled writers",
        },
        {
          from: "mcp-migration",
          to: "active",
          changes: [
            "state",
            "version",
            "savedRequestsSchemaVersion=7",
            "mcpReadOnlySchemaVersion=1",
          ],
          conditions:
            "all compatible consumers/routes/IAM/code verified; closed-stage re-inventory; same generation and all accounting preserved",
        },
      ],
      counterResets: false,
      deploymentOrActivationPerformed: false,
    },
    null,
    2,
  ),
);
