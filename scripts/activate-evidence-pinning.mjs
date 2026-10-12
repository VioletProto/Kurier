// No argument is read-only preview. --activate requires separate authorization.
// A failed activation leaves the stage closed; do not guess an automatic reopen.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { pinningInventory } from "./evidence-pinning-inventory.mjs";
const activate = process.argv[2] === "--activate";
if (process.argv.length > 3 || (process.argv[2] && !activate))
  throw Error(
    "Use no argument for preview or --activate only after authorization.",
  );
const out = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args) {
  const result = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (result.status !== 0)
    throw Error(
      "Pinning capability operation failed; inspect durable state before retry.",
    );
  return result.stdout.trim() ? JSON.parse(result.stdout) : {};
}
const identity = aws(["sts", "get-caller-identity"]);
if (
  identity.Account !== "747336059622" ||
  identity.Arn.endsWith(":root") ||
  out.stage !== "dev-api" ||
  out.region !== "us-east-2" ||
  !out.controlTable?.startsWith("kurier-dev-api-ControlTable-")
)
  throw Error("Intended non-root development account/stage required.");
const table = out.controlTable;
const key = { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } };
function marker() {
  return aws([
    "dynamodb",
    "get-item",
    "--table-name",
    table,
    "--key",
    JSON.stringify(key),
    "--consistent-read",
  ]).Item;
}
function inventory() {
  const items = [];
  let cursor;
  do {
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
    const page = aws(args);
    items.push(...(page.Items ?? []));
    cursor = page.LastEvaluatedKey;
  } while (cursor);
  return pinningInventory(items);
}
let stage = marker();
if (
  stage?.savedRequestsSchemaVersion?.N === "4" &&
  stage.evidencePinningSchemaVersion?.N === "1"
) {
  if (stage.state?.S !== "active")
    throw Error("Pinning stage is closed; preserve it for review.");
  console.log("Pinning capabilities already active; no reset or mutation.");
  process.exit(0);
}
if (
  stage?.savedRequestsSchemaVersion?.N !== "3" ||
  stage.cloudExecutionsSchemaVersion?.N !== "1" ||
  stage.executionInputsSchemaVersion?.N !== "1" ||
  stage.protectedSecretsSchemaVersion?.N !== "1" ||
  stage.evidencePinningSchemaVersion ||
  !stage.recoveryGeneration?.S ||
  !["active", "pinning-migration"].includes(stage.state?.S)
)
  throw Error("Unexpected capability/state; preserve for review.");
const consumers = [
  out.apiFunction,
  out.cleanupFunction,
  out.workerFunction,
].map((name) =>
  aws(["lambda", "get-function-configuration", "--function-name", name]),
);
const ready = consumers.every(
  (fn) =>
    fn.State === "Active" &&
    fn.LastUpdateStatus === "Successful" &&
    fn.Environment?.Variables?.KURIER_EVIDENCE_PINNING_SCHEMA_VERSION === "1",
);
let planned = inventory();
console.log(
  JSON.stringify({
    preview: true,
    consumersReady: ready,
    projectsToCheck: planned.projects.length,
    stageQuotaExists: !!planned.quota,
    savedRequestsCapability: "3 -> 4",
    evidencePinningCapability: 1,
    closeStageForInventory: true,
    generationPreserved: true,
    freePlanAndConcurrencyChanges: false,
  }),
);
if (!activate) process.exit(0);
if (!ready)
  throw Error("Deploy all pinning consumers before authorized activation.");
await import("./verify-cloud-execution-config.mjs");
function updateStage(from, to, capability = false) {
  const values = {
    ":state": { S: from },
    ":nextState": { S: to },
    ":v": stage.version,
    ":next": { N: String(Number(stage.version.N) + 1) },
    ":g": stage.recoveryGeneration,
    ":three": { N: "3" },
    ":one": { N: "1" },
  };
  let expression = "SET #s = :nextState, #v = :next";
  if (capability) {
    expression +=
      ", savedRequestsSchemaVersion = :four, evidencePinningSchemaVersion = :one";
    values[":four"] = { N: "4" };
  }
  aws([
    "dynamodb",
    "update-item",
    "--table-name",
    table,
    "--key",
    JSON.stringify(key),
    "--update-expression",
    expression,
    "--condition-expression",
    "#s = :state AND #v = :v AND recoveryGeneration = :g AND savedRequestsSchemaVersion = :three AND cloudExecutionsSchemaVersion = :one AND executionInputsSchemaVersion = :one AND protectedSecretsSchemaVersion = :one AND attribute_not_exists(evidencePinningSchemaVersion)",
    "--expression-attribute-names",
    JSON.stringify({ "#s": "state", "#v": "version" }),
    "--expression-attribute-values",
    JSON.stringify(values),
  ]);
  const before = stage;
  stage = marker();
  if (
    stage.state?.S !== to ||
    stage.recoveryGeneration?.S !== before.recoveryGeneration.S ||
    stage.version.N !== values[":next"].N
  )
    throw Error("Stage readback mismatch; do not reopen automatically.");
}
if (stage.state.S === "active") updateStage("active", "pinning-migration");
planned = inventory(); // closed stage fences all participating writers/cleanup
const gate = () => ({
  ConditionCheck: {
    TableName: table,
    Key: key,
    ConditionExpression: "#s = :closed AND #v = :v AND recoveryGeneration = :g",
    ExpressionAttributeNames: { "#s": "state", "#v": "version" },
    ExpressionAttributeValues: {
      ":closed": { S: "pinning-migration" },
      ":v": stage.version,
      ":g": stage.recoveryGeneration,
    },
  },
});
for (const item of [
  ...planned.projects,
  ...(planned.quota ? [planned.quota] : []),
]) {
  if (item.pinnedCount?.N === "0" && item.pinnedBytes?.N === "0") continue;
  const pk = { PK: item.PK, SK: item.SK };
  aws([
    "dynamodb",
    "transact-write-items",
    "--client-request-token",
    crypto.randomUUID(),
    "--transact-items",
    JSON.stringify([
      gate(),
      {
        Update: {
          TableName: table,
          Key: pk,
          UpdateExpression:
            "SET pinnedCount = :zero, pinnedBytes = :zero, #v = :next",
          ConditionExpression:
            "#v = :v AND (attribute_not_exists(pinnedCount) OR pinnedCount = :zero) AND (attribute_not_exists(pinnedBytes) OR pinnedBytes = :zero)",
          ExpressionAttributeNames: { "#v": "version" },
          ExpressionAttributeValues: {
            ":v": item.version,
            ":zero": { N: "0" },
            ":next": { N: String(Number(item.version.N) + 1) },
          },
        },
      },
    ]),
  ]);
}
if (!planned.quota)
  aws([
    "dynamodb",
    "transact-write-items",
    "--transact-items",
    JSON.stringify([
      gate(),
      {
        Put: {
          TableName: table,
          Item: {
            PK: { S: "STAGE#dev-api" },
            SK: { S: "QUOTA" },
            kind: { S: "executionQuota" },
            schemaVersion: { N: "1" },
            version: { N: "0" },
            pinnedCount: { N: "0" },
            pinnedBytes: { N: "0" },
          },
          ConditionExpression: "attribute_not_exists(PK)",
        },
      },
    ]),
  ]);
const verified = inventory();
if (
  [...verified.projects, verified.quota].some(
    (i) => !i || i.pinnedCount?.N !== "0" || i.pinnedBytes?.N !== "0",
  )
)
  throw Error("Counter verification incomplete; stage remains closed.");
updateStage("pinning-migration", "active", true);
if (
  stage.savedRequestsSchemaVersion.N !== "4" ||
  stage.evidencePinningSchemaVersion.N !== "1"
)
  throw Error("Capability verification incomplete.");
console.log(
  "Capability four/pinning one verified; generation preserved. No evidence bytes or checksums changed.",
);
