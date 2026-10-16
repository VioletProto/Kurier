// Explicit guarded activation. No automatic reopen or retry of uncertain writes.
import {
  aws,
  outputs as o,
  marker,
  hash,
  verify,
} from "./verify-read-only-mcp-deployment.mjs";
const [mode, baseline, ...rest] = process.argv.slice(2);
if (mode !== "--activate" || !baseline || rest.length)
  throw Error(
    "Use --activate BASELINE only after explicit deployment approval.",
  );
verify(baseline);
function inventory() {
  const rows = [];
  for (const table of [o.controlTable, o.protectedTable]) {
    let cursor;
    for (let page = 0; page < 1000; page++) {
      // Ciphertext, protected values, runtime inputs and request bodies are never read.
      const fields = [
        "PK",
        "SK",
        "kind",
        "state",
        "version",
        "ownerId",
        "projectId",
        "pinnedCount",
        "pinnedBytes",
        "inflightCount",
        "writeSettled",
        "jobId",
        "executionId",
        "objectKey",
        "checksum",
        "bytes",
        "used",
        "limit",
        "normalExpiresAt",
      ];
      const names = Object.fromEntries(fields.map((f, i) => ["#p" + i, f]));
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
        Object.keys(names).join(","),
        "--expression-attribute-names",
        JSON.stringify(names),
      ];
      if (cursor) args.push("--exclusive-start-key", JSON.stringify(cursor));
      const r = aws(args);
      rows.push(...(r.Items ?? []));
      cursor = r.LastEvaluatedKey;
      if (!cursor) break;
      if (page === 999)
        throw Error("Incomplete bounded inventory; stage remains closed");
    }
  }
  const byKey = new Map(rows.map((r) => [r.PK.S + "/" + r.SK.S, r]));
  for (const r of rows) {
    const k = r.kind?.S ?? "",
      state = r.state?.S;
    if (
      (k === "project" && state !== "active") ||
      (k === "executionJob" &&
        !["completed", "failed", "deleted"].includes(state)) ||
      (k === "executionWorkflowRun" &&
        !["completed", "failed", "deleted"].includes(state)) ||
      k === "executionLocalWait" ||
      (k === "executionOutbox" && state === "pending") ||
      k === "executionWorkflowInputs" ||
      k === "executionWorkflowOutput"
    )
      throw Error("Unsettled execution/deletion/workflow requires inspection");
    if (k === "executionUpload") {
      const job = byKey.get(r.PK.S + "/JOB#" + r.jobId?.S),
        snap = byKey.get(r.PK.S + "/EXEC#" + r.executionId?.S + "#SNAP");
      if (
        state !== "published" ||
        r.writeSettled?.BOOL !== true ||
        !["completed", "failed"].includes(job?.state?.S) ||
        r.objectKey?.S !== snap?.objectKey?.S ||
        r.checksum?.S !== snap?.checksum?.S ||
        r.bytes?.N !== snap?.bytes?.N
      )
        throw Error("Writer settlement proof incomplete");
    }
    if (
      k === "project" &&
      (!r.pinnedCount?.N ||
        !r.pinnedBytes?.N ||
        (r.inflightCount && r.inflightCount.N !== "0"))
    )
      throw Error("Missing pin accounting or active inflight slot");
  }
  // Full quota rows contain only coordination/counters. Preserve all their fields,
  // including future accounting fields, rather than a selected subset.
  const accounting = [];
  for (const r of rows.filter(
    (r) => r.kind?.S === "project" || r.kind?.S === "executionQuota",
  )) {
    const full = aws([
      "dynamodb",
      "get-item",
      "--table-name",
      o.controlTable,
      "--key",
      JSON.stringify({ PK: r.PK, SK: r.SK }),
      "--consistent-read",
    ]).Item;
    if (!full || full.kind?.S !== r.kind.S)
      throw Error("Accounting inventory changed");
    accounting.push(full);
  }
  accounting.sort((a, b) =>
    (a.PK.S + "/" + a.SK.S).localeCompare(b.PK.S + "/" + b.SK.S),
  );
  return {
    accountingHash: hash(accounting),
    projects: accounting.filter((r) => r.kind.S === "project").length,
  };
}
let stage = marker();
if (
  stage.state?.S !== "active" ||
  stage.savedRequestsSchemaVersion?.N !== "6" ||
  stage.mcpReadOnlySchemaVersion ||
  !stage.recoveryGeneration?.S ||
  [
    "cloudExecutionsSchemaVersion",
    "executionInputsSchemaVersion",
    "protectedSecretsSchemaVersion",
    "evidencePinningSchemaVersion",
    "localAgentExecutionsSchemaVersion",
    "sequentialWorkflowsSchemaVersion",
  ].some((k) => stage[k]?.N !== "1")
)
  throw Error("Unexpected durable marker; inspect before any resume");
const originalStage = stage;
const before = inventory();
function update(to, activate) {
  const previous = stage;
  const values = {
    ":state": stage.state,
    ":version": stage.version,
    ":next": { N: String(Number(stage.version.N) + 1) },
    ":to": { S: to },
    ":generation": stage.recoveryGeneration,
    ":six": { N: "6" },
    ":one": { N: "1" },
  };
  let expression = "SET #state = :to, #version = :next";
  if (activate) {
    expression +=
      ", savedRequestsSchemaVersion = :seven, mcpReadOnlySchemaVersion = :one";
    values[":seven"] = { N: "7" };
  }
  aws(["dynamodb", "transact-write-items"], {
    TransactItems: [
      {
        Update: {
          TableName: o.controlTable,
          Key: { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } },
          UpdateExpression: expression,
          ConditionExpression:
            "#state = :state AND #version = :version AND recoveryGeneration = :generation AND savedRequestsSchemaVersion = :six AND cloudExecutionsSchemaVersion = :one AND executionInputsSchemaVersion = :one AND protectedSecretsSchemaVersion = :one AND evidencePinningSchemaVersion = :one AND localAgentExecutionsSchemaVersion = :one AND sequentialWorkflowsSchemaVersion = :one AND attribute_not_exists(mcpReadOnlySchemaVersion)",
          ExpressionAttributeNames: {
            "#state": "state",
            "#version": "version",
          },
          ExpressionAttributeValues: values,
        },
      },
    ],
  });
  stage = marker();
  const expected = {
    ...previous,
    state: values[":to"],
    version: values[":next"],
    ...(activate
      ? {
          savedRequestsSchemaVersion: { N: "7" },
          mcpReadOnlySchemaVersion: { N: "1" },
        }
      : {}),
  };
  if (hash(stage) !== hash(expected))
    throw Error("Activation readback mismatch; inspect durable state");
}
try {
  update("mcp-migration", false);
  if (hash(inventory()) !== hash(before) || hash(inventory()) !== hash(before))
    throw Error("Closed accounting changed; inspect stage before resuming");
  update("active", true);
  if (hash(inventory()) !== hash(before))
    throw Error(
      "Accounting changed during activation; inspect before resuming",
    );
  console.log(
    JSON.stringify({
      activated: true,
      state: stage.state.S,
      version: stage.version.N,
      savedRequests: stage.savedRequestsSchemaVersion.N,
      mcpReadOnly: stage.mcpReadOnlySchemaVersion.N,
      generation: stage.recoveryGeneration.S,
      originalVersion: originalStage.version.N,
      accountingPreserved: true,
      ...before,
      existingCapabilitiesPreserved: true,
      counterResets: false,
    }),
  );
} catch (e) {
  const durable = marker();
  console.error(
    JSON.stringify({
      activationFailedOrUncertain: true,
      durableState: durable.state?.S,
      version: durable.version?.N,
      generation: durable.recoveryGeneration?.S,
      savedRequests: durable.savedRequestsSchemaVersion?.N,
      mcpReadOnly: durable.mcpReadOnlySchemaVersion?.N ?? null,
      automaticReopen: false,
    }),
  );
  throw e;
}
