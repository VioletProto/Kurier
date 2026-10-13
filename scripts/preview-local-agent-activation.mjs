// Read-only inventory and activation plan. There is no mutation mode.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
const out = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
if (process.argv.length !== 2) throw Error("This command is preview-only.");
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (r.status !== 0)
    throw Error(
      `Read-only activation inspection failed: ${args.slice(0, 2).join(" ")}`,
    );
  return r.stdout.trim() ? JSON.parse(r.stdout) : {};
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
const key = { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } };
const stage = aws([
  "dynamodb",
  "get-item",
  "--table-name",
  out.controlTable,
  "--key",
  JSON.stringify(key),
  "--consistent-read",
]).Item;
if (
  stage?.state?.S !== "active" ||
  stage.savedRequestsSchemaVersion?.N !== "4" ||
  stage.evidencePinningSchemaVersion?.N !== "1" ||
  stage.cloudExecutionsSchemaVersion?.N !== "1" ||
  stage.executionInputsSchemaVersion?.N !== "1" ||
  stage.protectedSecretsSchemaVersion?.N !== "1" ||
  stage.localAgentExecutionsSchemaVersion ||
  !stage.recoveryGeneration?.S
)
  throw Error("Unexpected capability/state; preserve for review.");
const counts = {
  projects: 0,
  queuedCloud: 0,
  runningCloud: 0,
  localRecords: 0,
};
let cursor;
let pages = 0;
do {
  if (++pages > 1000)
    throw Error("Inventory bound exceeded; no complete preflight claimed.");
  const args = [
    "dynamodb",
    "scan",
    "--table-name",
    out.controlTable,
    "--consistent-read",
    "--projection-expression",
    "PK, SK, #k, #s, #t, localAgentId, pinnedCount, pinnedBytes",
    "--expression-attribute-names",
    JSON.stringify({ "#k": "kind", "#s": "state", "#t": "target" }),
    "--limit",
    "100",
    "--no-paginate",
  ];
  if (cursor) args.push("--exclusive-start-key", JSON.stringify(cursor));
  const page = aws(args);
  cursor = page.LastEvaluatedKey;
  for (const item of page.Items ?? []) {
    const kind = item.kind?.S ?? "";
    if (kind === "project") {
      counts.projects++;
      if (!item.pinnedCount?.N || !item.pinnedBytes?.N)
        throw Error("Missing pin counters; no reset/reconciliation inferred.");
    }
    if (kind === "executionJob" && item.state?.S === "queued")
      counts.queuedCloud++;
    if (
      kind === "executionJob" &&
      ["running", "claimed"].includes(item.state?.S)
    )
      counts.runningCloud++;
    if (
      kind.startsWith("localAgent") ||
      kind.startsWith("executionLocal") ||
      kind === "executionAgentScope" ||
      item.target?.S === "local" ||
      item.localAgentId
    )
      counts.localRecords++;
  }
} while (cursor);
if (counts.localRecords !== 0)
  throw Error("Existing local records require reviewed reconciliation.");
const consumers = [
  out.apiFunction,
  out.cleanupFunction,
  out.workerFunction,
].map((name) => {
  const fn = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    name,
  ]);
  return {
    name,
    ready:
      fn.State === "Active" &&
      fn.LastUpdateStatus === "Successful" &&
      fn.Environment?.Variables
        ?.KURIER_LOCAL_AGENT_EXECUTIONS_SCHEMA_VERSION === "1",
  };
});
console.log(
  JSON.stringify(
    {
      preview: true,
      deploymentRequired: consumers.some((c) => !c.ready),
      consumers,
      inventory: counts,
      currentStageVersion: Number(stage.version.N),
      generation: stage.recoveryGeneration.S,
      plannedMarkerWrites: [
        {
          from: "active",
          to: "local-agent-migration",
          changes: ["state", "version"],
          conditions:
            "expected state/version/generation and savedRequests=4,pinning=1,local capability absent",
        },
        {
          from: "local-agent-migration",
          to: "active",
          changes: [
            "state",
            "version",
            "savedRequestsSchemaVersion=5",
            "localAgentExecutionsSchemaVersion=1",
          ],
          conditions:
            "same generation, all deployed consumers verified, closed inventory rechecked",
        },
      ],
      counterResets: false,
      queueOrConcurrencyChanges: false,
      secretKeyRotation: false,
      automaticReopenOnFailure: false,
      deploymentOrActivationPerformed: false,
    },
    null,
    2,
  ),
);
