// Read-only scoped inventory/activation plan. Deliberately has no mutation mode.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
if (process.argv.length !== 2)
  throw Error("Preview only; no arguments or mutation mode.");
const out = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
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
  throw Error("Intended non-root account/stage required");
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
  stage.savedRequestsSchemaVersion?.N !== "5" ||
  stage.localAgentExecutionsSchemaVersion?.N !== "1" ||
  stage.cloudExecutionsSchemaVersion?.N !== "1" ||
  stage.executionInputsSchemaVersion?.N !== "1" ||
  stage.protectedSecretsSchemaVersion?.N !== "1" ||
  stage.evidencePinningSchemaVersion?.N !== "1" ||
  stage.sequentialWorkflowsSchemaVersion ||
  !stage.recoveryGeneration?.S
)
  throw Error("Unexpected marker; review required");
const counts = {
  projects: 0,
  queued: 0,
  claimedOrRunning: 0,
  workflowRecords: 0,
  unsettledUploads: 0,
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
      "PK, SK, #k, #s, pinnedCount, pinnedBytes, inflightCount, writeSettled",
      "--expression-attribute-names",
      JSON.stringify({ "#k": "kind", "#s": "state" }),
    ];
    if (cursor) args.push("--exclusive-start-key", JSON.stringify(cursor));
    const r = aws(args);
    for (const x of r.Items ?? []) {
      const kind = x.kind?.S;
      if (kind === "project") {
        counts.projects++;
        if (!x.pinnedCount?.N || !x.pinnedBytes?.N)
          throw Error("Missing counters; never initialize/reset blindly");
      }
      if (kind === "executionJob") {
        if (x.state?.S === "queued") counts.queued++;
        if (["claimed", "running"].includes(x.state?.S))
          counts.claimedOrRunning++;
      }
      if (kind?.startsWith("executionWorkflow")) counts.workflowRecords++;
      if (kind === "executionUpload" && !["published"].includes(x.state?.S))
        counts.unsettledUploads++;
    }
    cursor = r.LastEvaluatedKey;
    if (!cursor) {
      complete = true;
      break;
    }
  }
  if (!complete) throw Error("Incomplete bounded table inventory");
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
    lastUpdate: f.LastUpdateStatus,
    memory: f.MemorySize,
    timeout: f.Timeout,
    architecture: f.Architectures,
    workflowCapable:
      f.Environment?.Variables?.KURIER_SEQUENTIAL_WORKFLOWS_SCHEMA_VERSION ===
      "1",
    codeSha256: f.CodeSha256,
  };
});
const account = aws(["lambda", "get-account-settings"]);
const mappings = aws([
  "lambda",
  "list-event-source-mappings",
  "--function-name",
  out.workerFunction,
]);
if (mappings.NextMarker) throw Error("Incomplete event mapping inventory");
const queue = mappings.EventSourceMappings.map((m) => ({
  state: m.State,
  batch: m.BatchSize,
  maximumConcurrency: m.ScalingConfig?.MaximumConcurrency,
  provisionedPollers: m.ProvisionedPollerConfig ?? null,
}));
const plan = aws(["freetier", "get-account-plan-state"]);
console.log(
  JSON.stringify(
    {
      preview: true,
      account: identity.Account,
      nonRoot: true,
      stageVersion: stage.version.N,
      generation: stage.recoveryGeneration.S,
      counts,
      consumers,
      accountLimit: account.AccountLimit?.ConcurrentExecutions,
      unreserved: account.AccountLimit?.UnreservedConcurrentExecutions,
      queue,
      plan: { type: plan.accountPlanType, status: plan.accountPlanStatus },
      plannedMarkerWrites: [
        {
          from: "active",
          to: "workflow-migration",
          changes: ["state", "version"],
          conditions:
            "expected marker version/generation, savedRequests5/local1/cloud1/pinning1; no unresolved jobs/uploads",
        },
        {
          from: "workflow-migration",
          to: "active",
          changes: [
            "state",
            "version",
            "savedRequestsSchemaVersion=6",
            "sequentialWorkflowsSchemaVersion=1",
          ],
          conditions:
            "all consumers/code/IAM/routes verified; rechecked closed-stage inventories; same generation",
        },
      ],
      counterResets: false,
      deploymentOrActivationPerformed: false,
    },
    null,
    2,
  ),
);
