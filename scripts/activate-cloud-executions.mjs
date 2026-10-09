// Capability migration only; no credentials or protected data are printed.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import "./verify-cloud-execution-config.mjs";
const activate = process.argv[2] === "--activate";
if (process.argv.length > 3 || (process.argv[2] && !activate))
  throw new Error(
    "Use no argument for preview or --activate for the authorized migration.",
  );
const outputs = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args) {
  const result = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (result.status !== 0)
    throw new Error(
      "AWS capability operation unavailable; no completion claimed.",
    );
  return result.stdout.trim() ? JSON.parse(result.stdout) : {};
}
const identity = aws(["sts", "get-caller-identity"]);
if (
  identity.Account !== "747336059622" ||
  identity.Arn.endsWith(":root") ||
  outputs.stage !== "dev-api" ||
  outputs.region !== "us-east-2"
)
  throw new Error("Intended development stage and non-root account required.");
if (
  !outputs.workerFunction ||
  !outputs.evidenceBucket ||
  !outputs.executionQueueUrl
)
  throw new Error("Deploy execution consumers before capability activation.");
for (const name of [
  outputs.apiFunction,
  outputs.cleanupFunction,
  outputs.workerFunction,
]) {
  const fn = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    name,
  ]);
  if (
    fn.State !== "Active" ||
    fn.LastUpdateStatus !== "Successful" ||
    fn.Environment?.Variables?.KURIER_CLOUD_EXECUTIONS_SCHEMA_VERSION !== "1" ||
    fn.Environment?.Variables?.KURIER_EXECUTION_INPUTS_SCHEMA_VERSION !== "1"
  )
    throw new Error("Execution consumer deployment not ready.");
}
const key = { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } };
const marker = aws([
  "dynamodb",
  "get-item",
  "--table-name",
  outputs.controlTable,
  "--key",
  JSON.stringify(key),
  "--consistent-read",
]).Item;
if (
  marker?.state?.S !== "active" ||
  !marker.recoveryGeneration?.S ||
  marker.protectedSecretsSchemaVersion?.N !== "1"
)
  throw new Error("Stage is not available for this migration.");
if (
  marker.savedRequestsSchemaVersion?.N === "3" &&
  marker.cloudExecutionsSchemaVersion?.N === "1" &&
  marker.executionInputsSchemaVersion?.N === "1"
) {
  console.log(
    "Execution capabilities already active; recovery generation preserved.",
  );
  process.exit(0);
}
if (
  marker.savedRequestsSchemaVersion?.N !== "2" ||
  marker.cloudExecutionsSchemaVersion ||
  marker.executionInputsSchemaVersion
)
  throw new Error("Unexpected capability version; preserve it for review.");
console.log(
  "Preview: savedRequestsSchemaVersion 2 → 3; cloudExecutionsSchemaVersion 1; executionInputsSchemaVersion 1; protected capability and recovery generation preserved.",
);
if (!activate) process.exit(0);
aws([
  "dynamodb",
  "update-item",
  "--table-name",
  outputs.controlTable,
  "--key",
  JSON.stringify(key),
  "--update-expression",
  "SET savedRequestsSchemaVersion = :three, cloudExecutionsSchemaVersion = :one, executionInputsSchemaVersion = :one, #v = :next",
  "--condition-expression",
  "#s = :active AND recoveryGeneration = :generation AND #v = :version AND savedRequestsSchemaVersion = :two AND protectedSecretsSchemaVersion = :one AND attribute_not_exists(cloudExecutionsSchemaVersion) AND attribute_not_exists(executionInputsSchemaVersion)",
  "--expression-attribute-names",
  JSON.stringify({ "#s": "state", "#v": "version" }),
  "--expression-attribute-values",
  JSON.stringify({
    ":three": { N: "3" },
    ":one": { N: "1" },
    ":two": { N: "2" },
    ":active": { S: "active" },
    ":generation": marker.recoveryGeneration,
    ":version": marker.version,
    ":next": { N: String(Number(marker.version.N) + 1) },
  }),
]);
const readback = aws([
  "dynamodb",
  "get-item",
  "--table-name",
  outputs.controlTable,
  "--key",
  JSON.stringify(key),
  "--consistent-read",
]).Item;
if (
  readback.savedRequestsSchemaVersion?.N !== "3" ||
  readback.cloudExecutionsSchemaVersion?.N !== "1" ||
  readback.executionInputsSchemaVersion?.N !== "1" ||
  readback.recoveryGeneration?.S !== marker.recoveryGeneration.S
)
  throw new Error("Capability readback failed; review durable state.");
console.log("Capability migration verified; recovery generation preserved.");
