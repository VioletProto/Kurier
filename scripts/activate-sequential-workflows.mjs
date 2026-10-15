// Explicit guarded activation only. Failure never automatically reopens a stage.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
if (process.argv.length !== 3 || process.argv[2] !== "--activate")
  throw Error("Use --activate only after approval and deployed verification.");
await import("./verify-sequential-workflow-deployment.mjs");
const o = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (r.status !== 0)
    throw Error(
      "Activation operation failed; inspect durable stage before resuming.",
    );
  return r.stdout.trim() ? JSON.parse(r.stdout) : {};
}
const key = { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } };
function marker() {
  return aws([
    "dynamodb",
    "get-item",
    "--table-name",
    o.controlTable,
    "--key",
    JSON.stringify(key),
    "--consistent-read",
  ]).Item;
}
function inventory() {
  const rows = [];
  for (const table of [o.controlTable, o.protectedTable]) {
    let cursor;
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
      ];
      if (cursor) args.push("--exclusive-start-key", JSON.stringify(cursor));
      const r = aws(args);
      rows.push(...r.Items);
      cursor = r.LastEvaluatedKey;
      if (!cursor) break;
      if (page === 999) throw Error("Incomplete bounded inventory");
    }
  }
  const indexed = new Map(rows.map((r) => [r.PK.S + "/" + r.SK.S, r]));
  for (const r of rows) {
    const kind = r.kind?.S ?? "";
    if (
      kind.startsWith("executionWorkflow") ||
      (kind === "executionJob" &&
        !["completed", "failed", "deleted"].includes(r.state?.S)) ||
      (kind === "executionUpload" && r.state?.S !== "published")
    )
      throw Error(
        "Unsettled execution/workflow/writer requires reviewed reconciliation",
      );
    if (kind === "executionUpload") {
      const job = indexed.get(r.PK.S + "/JOB#" + r.jobId?.S);
      const snap = indexed.get(r.PK.S + "/EXEC#" + r.executionId?.S + "#SNAP");
      if (
        r.writeSettled?.BOOL !== true ||
        !["completed", "failed"].includes(job?.state?.S) ||
        snap?.objectKey?.S !== r.objectKey?.S ||
        snap?.checksum?.S !== r.checksum?.S ||
        snap?.bytes?.N !== r.bytes?.N
      )
        throw Error("Published-writer proof mismatch");
    }
    if (
      kind === "project" &&
      (!r.pinnedCount?.N ||
        !r.pinnedBytes?.N ||
        (r.inflightCount && r.inflightCount.N !== "0"))
    )
      throw Error("Missing accounting or active inflight slot");
  }
  const accounts = rows.filter(
    (r) => r.kind?.S === "project" || r.kind?.S === "executionQuota",
  );
  accounts.sort((a, b) =>
    (a.PK.S + "/" + a.SK.S).localeCompare(b.PK.S + "/" + b.SK.S),
  );
  const stable = (v) =>
    Array.isArray(v)
      ? v.map(stable)
      : v && typeof v === "object"
        ? Object.fromEntries(
            Object.keys(v)
              .sort()
              .map((k) => [k, stable(v[k])]),
          )
        : v;
  return createHash("sha256")
    .update(JSON.stringify(stable(accounts)))
    .digest("hex");
}
let stage = marker();
if (
  stage.state?.S !== "active" ||
  stage.savedRequestsSchemaVersion?.N !== "5" ||
  stage.sequentialWorkflowsSchemaVersion ||
  stage.localAgentExecutionsSchemaVersion?.N !== "1" ||
  stage.cloudExecutionsSchemaVersion?.N !== "1" ||
  stage.executionInputsSchemaVersion?.N !== "1" ||
  stage.protectedSecretsSchemaVersion?.N !== "1" ||
  stage.evidencePinningSchemaVersion?.N !== "1" ||
  !stage.recoveryGeneration?.S
)
  throw Error("Unexpected durable marker; no automatic resume/reopen.");
const originalAccounting = inventory();
function update(to, activate) {
  const before = stage;
  const values = {
    ":state": before.state,
    ":v": before.version,
    ":next": { N: String(Number(before.version.N) + 1) },
    ":to": { S: to },
    ":g": before.recoveryGeneration,
    ":five": { N: "5" },
    ":one": { N: "1" },
  };
  let expression = "SET #s = :to, #v = :next";
  if (activate) {
    expression +=
      ", savedRequestsSchemaVersion = :six, sequentialWorkflowsSchemaVersion = :one";
    values[":six"] = { N: "6" };
  }
  aws([
    "dynamodb",
    "update-item",
    "--table-name",
    o.controlTable,
    "--key",
    JSON.stringify(key),
    "--update-expression",
    expression,
    "--condition-expression",
    "#s = :state AND #v = :v AND recoveryGeneration = :g AND savedRequestsSchemaVersion = :five AND evidencePinningSchemaVersion = :one AND cloudExecutionsSchemaVersion = :one AND executionInputsSchemaVersion = :one AND protectedSecretsSchemaVersion = :one AND localAgentExecutionsSchemaVersion = :one AND attribute_not_exists(sequentialWorkflowsSchemaVersion)",
    "--expression-attribute-names",
    JSON.stringify({ "#s": "state", "#v": "version" }),
    "--expression-attribute-values",
    JSON.stringify(values),
  ]);
  stage = marker();
  const expected = {
    ...before,
    state: values[":to"],
    version: values[":next"],
    ...(activate
      ? {
          savedRequestsSchemaVersion: values[":six"],
          sequentialWorkflowsSchemaVersion: values[":one"],
        }
      : {}),
  };
  if (
    JSON.stringify(Object.entries(stage).sort()) !==
    JSON.stringify(Object.entries(expected).sort())
  )
    throw Error("Durable marker mismatch; inspect before resuming.");
}
try {
  update("workflow-migration", false);
  const accounting = inventory();
  if (accounting !== originalAccounting || inventory() !== accounting)
    throw Error("Closed accounting changed; stage stays closed.");
  update("active", true);
  if (inventory() !== accounting)
    throw Error("Accounting changed during activation; inspect durable state.");
  console.log(
    JSON.stringify({
      state: stage.state.S,
      version: Number(stage.version.N),
      savedRequests: 6,
      sequentialWorkflows: 1,
      localAgentExecutions: 1,
      accountingHash: accounting,
      generation: stage.recoveryGeneration.S,
      accountingPreserved: true,
    }),
  );
} catch (e) {
  const durable = marker();
  console.error(
    JSON.stringify({
      durableState: durable.state?.S,
      version: durable.version?.N,
      generation: durable.recoveryGeneration?.S,
      savedRequests: durable.savedRequestsSchemaVersion?.N,
      sequentialWorkflows: durable.sequentialWorkflowsSchemaVersion?.N ?? null,
      automaticReopen: false,
    }),
  );
  throw e;
}
