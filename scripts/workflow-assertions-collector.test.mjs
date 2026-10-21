// Local reproductions of collector errors. No cloud adapter or credentials.
import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";

test("mixed quota projections cannot establish drift or preservation", () => {
  const selected = {
    PK: { S: "STAGE#dev-api" },
    SK: { S: "QUOTA#EXECUTION" },
    kind: { S: "executionQuota" },
    version: { N: "20" },
    count: { N: "20" },
    month: { S: "2026-10" },
  };
  const full = {
    ...selected,
    schemaVersion: { N: "1" },
    pinnedCount: { N: "0" },
    pinnedBytes: { N: "0" },
  };
  assert.notDeepEqual(selected, full); // The live collector compared these shapes.
  // Future reads must both request this same complete projection from the outset.
  const fields = [
    "PK",
    "SK",
    "kind",
    "schemaVersion",
    "version",
    "count",
    "month",
    "pinnedCount",
    "pinnedBytes",
  ];
  const project = (row) =>
    Object.fromEntries(
      fields.filter((f) => Object.hasOwn(row, f)).map((f) => [f, row[f]]),
    );
  assert.deepEqual(project(full), project({ ...full }));
  assert.notDeepEqual(project(full), project({ ...full, count: { N: "21" } }));
  assert.notDeepEqual(
    project(full),
    project({ ...full, version: { N: "21" } }),
  );
  assert.notDeepEqual(project(selected), project(full)); // Never repair missing historical fields.
});
test("fractional remaining timeout fails locally; integer milliseconds work", () => {
  assert.throws(
    () =>
      spawnSync(process.execPath, ["-e", ""], { timeout: 14835.81689453125 }),
    { code: "ERR_OUT_OF_RANGE" },
  );
  const remaining = Math.floor(14835.81689453125);
  const result = spawnSync(process.execPath, ["-e", ""], {
    timeout: remaining,
  });
  assert.equal(result.status, 0);
  assert.equal(Math.max(0, Math.floor(-1.5)), 0);
});
