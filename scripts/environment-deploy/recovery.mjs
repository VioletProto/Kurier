// Recovery inspection and offline proposal only. No stage-write entrypoint.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import {
  collect,
  marker,
  hash,
  verifyHistorical,
  verifyInitialRouting,
  scope,
} from "./collector.mjs";
import {
  checkMarker,
  inspectRows,
  counterGuard,
} from "../lib/project-environments-activation.mjs";
const inventory = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-inventory.json",
      import.meta.url,
    ),
  ),
);

export function assess(bundle, context) {
  checkMarker(bundle.marker);
  if (
    hash(bundle.marker) !== hash(context.expectedMarker) ||
    bundle.marker.state.S !== "workflow-migration"
  )
    throw Error("Closed marker drift.");
  verifyHistorical(bundle.platform, bundle.marker);
  if (
    bundle.platform.functions.length !== inventory.functions.length ||
    hash(bundle.platform.functions.map((f) => f.resource).sort()) !==
      hash(inventory.functions.map((f) => f.logical).sort())
  )
    throw Error("Function inventory incomplete.");
  verifyInitialRouting(bundle.routing);
  if (
    !bundle.frontendReady ||
    bundle.frontendExpectation !== "baseline" ||
    bundle.frontendExpectedReleaseId !==
      inventory.frontendRollback.manifest.releaseId
  )
    throw Error("Old frontend not ready.");
  const rows = inspectRows(bundle.rows);
  if (
    rows.accountingSha256 !== context.accountingSha256 ||
    Object.keys(bundle.depth).length !== 3 ||
    Object.values(bundle.depth).some((v) => v !== "0")
  )
    throw Error("Accounting/quiescence drift.");
  if (
    !bundle.stateInspection?.noCurrentDeployLock ||
    !bundle.stateInspection?.noObservedStateChangesSinceOriginalCollection
  )
    throw Error("State observation missing.");
  return rows;
}
export function proposal(bundle, context) {
  const rows = assess(bundle, context),
    before = bundle.marker;
  const after = {
    ...before,
    state: { S: "active" },
    version: { N: String(BigInt(before.version.N) + 1n) },
  };
  const names = {
    "#s": "state",
    "#v": "version",
    "#env": "projectEnvironmentsSchemaVersion",
  };
  const values = { ":active": { S: "active" }, ":next": after.version },
    conditions = [];
  Object.keys(before)
    .sort()
    .forEach((k, i) => {
      names["#b" + i] = k;
      values[":b" + i] = before[k];
      conditions.push("#b" + i + " = :b" + i);
    });
  conditions.push("attribute_not_exists(#env)");
  const actions = [
    {
      Update: {
        TableName: scope.controlTable,
        Key: { PK: before.PK, SK: before.SK },
        UpdateExpression: "SET #s = :active, #v = :next",
        ConditionExpression: conditions.join(" AND "),
        ExpressionAttributeNames: names,
        ExpressionAttributeValues: values,
      },
    },
    ...rows.accounting.map(counterGuard),
  ];
  if (
    actions.length > 80 ||
    rows.accounting.some((r) => r.table !== scope.controlTable)
  )
    throw Error("Recovery transaction bound.");
  return {
    action: "proposed-reopen-old-only",
    before,
    after,
    transaction: {
      TransactItems: actions,
      ClientRequestToken: context.clientRequestToken,
    },
    accountingSha256: rows.accountingSha256,
    onlyStateVersionChange: true,
    executionAuthorized: false,
  };
}
export function main(action, file) {
  const context = JSON.parse(readFileSync(file));
  if (action === "plan") return proposal(context.bundle, context);
  if (action !== "inspect") throw Error("Recovery has no write command.");
  const bundle = collect("collect");
  const r = spawnSync(
    "python3",
    [fileURLToPath(new URL("./recovery_state.py", import.meta.url)), file],
    { encoding: "utf8", maxBuffer: 1024 * 1024 },
  );
  if (r.status !== 0) throw Error("State inspection held.");
  bundle.stateInspection = JSON.parse(r.stdout);
  assess(bundle, context);
  if (hash(marker()) !== hash(bundle.marker))
    throw Error("Final marker drift.");
  return bundle;
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    console.log(JSON.stringify(main(...process.argv.slice(2))));
  } catch {
    console.error("Read-only recovery held; no retry or stage write.");
    process.exitCode = 1;
  }
}
