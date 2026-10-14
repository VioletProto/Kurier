// Explicit guarded activation only. Failure never automatically reopens a stage.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
if (process.argv.length !== 3 || process.argv[2] !== "--activate")
  throw Error("Use --activate only after approval and deployed verification.");
await import("./verify-local-agent-deployment.mjs");
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
  let cursor;
  const rows = [];
  for (let page = 0; page < 1000; page++) {
    const args = [
      "dynamodb",
      "scan",
      "--table-name",
      o.controlTable,
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
    if (page === 999) throw Error("Inventory bound exceeded.");
  }
  const indexed = new Map(rows.map((r) => [`${r.PK.S}/${r.SK.S}`, r]));
  for (const r of rows) {
    const kind = r.kind?.S ?? "";
    if (
      kind.startsWith("localAgent") ||
      kind.startsWith("executionLocal") ||
      kind === "executionAgentScope" ||
      r.target?.S === "local" ||
      r.localAgentId ||
      (kind === "executionUpload" && r.state?.S !== "published") ||
      (kind === "executionJob" &&
        !["completed", "failed", "deleted"].includes(r.state?.S))
    )
      throw Error(
        "Unsettled writer/local work requires reviewed reconciliation.",
      );
    if (kind === "executionUpload") {
      const job = indexed.get(`${r.PK.S}/JOB#${r.jobId?.S}`);
      const snap = indexed.get(`${r.PK.S}/EXEC#${r.executionId?.S}#SNAP`);
      if (
        r.writeSettled?.BOOL !== true ||
        !["completed", "failed"].includes(job?.state?.S) ||
        snap?.kind?.S !== "executionSnapshot" ||
        snap.objectKey?.S !== r.objectKey?.S ||
        snap.checksum?.S !== r.checksum?.S ||
        snap.bytes?.N !== r.bytes?.N
      )
        throw Error("Published writer proof mismatch; preserve for review.");
    }
    if (kind === "project" && (!r.pinnedCount?.N || !r.pinnedBytes?.N))
      throw Error("Missing pin accounting.");
  }
  const accounts = rows.filter(
    (r) => r.kind?.S === "project" || r.kind?.S === "executionQuota",
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
  accounts.sort((a, b) =>
    `${a.PK.S}/${a.SK.S}`.localeCompare(`${b.PK.S}/${b.SK.S}`),
  );
  return createHash("sha256")
    .update(JSON.stringify(stable(accounts)))
    .digest("hex");
}
let stage = marker();
if (
  stage.state?.S !== "active" ||
  stage.savedRequestsSchemaVersion?.N !== "4" ||
  stage.localAgentExecutionsSchemaVersion ||
  stage.evidencePinningSchemaVersion?.N !== "1" ||
  !stage.recoveryGeneration?.S
)
  throw Error("Unexpected durable marker; no automatic resume/reopen.");
inventory();
function update(to, activate) {
  const before = stage;
  const values = {
    ":state": before.state,
    ":v": before.version,
    ":next": { N: String(Number(before.version.N) + 1) },
    ":to": { S: to },
    ":g": before.recoveryGeneration,
    ":four": { N: "4" },
    ":one": { N: "1" },
  };
  let expression = "SET #s = :to, #v = :next";
  if (activate) {
    expression +=
      ", savedRequestsSchemaVersion = :five, localAgentExecutionsSchemaVersion = :one";
    values[":five"] = { N: "5" };
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
    "#s = :state AND #v = :v AND recoveryGeneration = :g AND savedRequestsSchemaVersion = :four AND evidencePinningSchemaVersion = :one AND cloudExecutionsSchemaVersion = :one AND executionInputsSchemaVersion = :one AND protectedSecretsSchemaVersion = :one AND attribute_not_exists(localAgentExecutionsSchemaVersion)",
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
          savedRequestsSchemaVersion: values[":five"],
          localAgentExecutionsSchemaVersion: values[":one"],
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
  update("local-agent-migration", false);
  const accounting = inventory();
  if (inventory() !== accounting)
    throw Error("Closed accounting changed; stage stays closed.");
  update("active", true);
  if (inventory() !== accounting)
    throw Error("Accounting changed during activation; inspect durable state.");
  console.log(
    JSON.stringify({
      state: stage.state.S,
      version: Number(stage.version.N),
      savedRequests: 5,
      localAgentExecutions: 1,
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
      localAgentExecutions:
        durable.localAgentExecutionsSchemaVersion?.N ?? null,
      automaticReopen: false,
    }),
  );
  throw e;
}
