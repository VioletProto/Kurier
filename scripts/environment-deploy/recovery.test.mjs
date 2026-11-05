import { scope } from "./collector.mjs";
import { fixture } from "./test-recovery-fixture.mjs";
import test from "node:test";
import assert from "node:assert/strict";
import { assess, proposal } from "./recovery.mjs";
import { hash } from "../lib/project-environments-activation.mjs";
import { spawnSync } from "node:child_process";
test("reopen proposal preserves the old release and increments version", () => {
  const { bundle, context } = fixture();
  const p = proposal(bundle, context);
  assert.equal(p.before.state.S, "workflow-migration");
  assert.equal(p.after.state.S, "active");
  assert.equal(p.after.version.N, "17");
  assert.equal(p.after.recoveryGeneration.S, p.before.recoveryGeneration.S);
  assert.equal(p.after.projectEnvironmentsSchemaVersion, undefined);
  assert.deepEqual(
    Object.keys(p.before)
      .filter((k) => hash(p.before[k]) !== hash(p.after[k]))
      .sort(),
    ["state", "version"],
  );
  assert.equal(p.transaction.TransactItems.length, 2);
  assert.equal(
    p.transaction.TransactItems[0].Update.UpdateExpression,
    "SET #s = :active, #v = :next",
  );
  assert.equal(p.executionAuthorized, false);
  assert.equal(
    p.transaction.TransactItems[1].ConditionCheck.TableName,
    scope.controlTable,
  );
});
for (const [name, mutate] of [
  ["marker", (b) => (b.marker.version.N = "17")],
  ["generation", (b) => (b.marker.recoveryGeneration.S += "-drift")],
  ["code", (b) => (b.platform.functions[0].codeSha256 = "drift")],
  ["IAM", (b) => (b.platform.functions[0].iamHash = "drift")],
  ["frontend", (b) => (b.frontendReady = false)],
  ["accounting", (b) => (b.rows[0].item.count.N = "33")],
  [
    "new environment data",
    (b) => (b.rows[0].item.environmentId = { S: "new" }),
  ],
  ["new format", (b) => (b.rows[0].item.scalarType = { S: "string" })],
  ["queue", (b) => (b.depth.visible = "1")],
  [
    "state mutations",
    (b) =>
      (b.stateInspection.noObservedStateChangesSinceOriginalCollection = false),
  ],
  ["new gateway route", (b) => b.routing.routes.push({ RouteId: "new" })],
])
  test(name + " drift blocks reopening proposal", () => {
    const { bundle, context } = fixture();
    mutate(bundle);
    assert.throws(() => assess(bundle, context));
  });
test("there is no stage-write CLI action", () => {
  const r = spawnSync(process.execPath, [
    "scripts/environment-deploy/recovery.mjs",
    "reopen",
    "/dev/null",
  ]);
  assert.notEqual(r.status, 0);
});
