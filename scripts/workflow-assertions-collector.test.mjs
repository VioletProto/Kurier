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

// Corrected production helpers, independent of live outputs or AWS adapters.
import {
  accountingQuery,
  accountingSnapshot,
  compareAccounting,
  runCollectorOnce,
  subprocessTimeout,
} from "./lib/workflow-assertions-collectors.mjs";
test("new accounting captures use one complete projection and reject incomplete history", () => {
  const row = {
    PK: { S: "STAGE#dev-api" },
    SK: { S: "QUOTA" },
    kind: { S: "executionQuota" },
    schemaVersion: { N: "1" },
    version: { N: "36" },
    count: { N: "20" },
    month: { S: "2026-10" },
    storageReservedBytes: { N: "16308" },
    pinnedCount: { N: "0" },
    pinnedBytes: { N: "0" },
  };
  const request = accountingQuery("Control", "STAGE#dev-api", "QUOTA");
  assert.equal(request.ConsistentRead, true);
  assert.deepEqual(
    request,
    accountingQuery("Control", "STAGE#dev-api", "QUOTA"),
  );
  const a = accountingSnapshot([row]),
    b = accountingSnapshot([{ ...row }]);
  assert(compareAccounting(a, b));
  for (const field of [
    "count",
    "version",
    "storageReservedBytes",
    "pinnedCount",
    "pinnedBytes",
  ])
    assert.equal(
      compareAccounting(
        a,
        accountingSnapshot([{ ...row, [field]: { N: "1" } }]),
      ),
      false,
    );
  const { schemaVersion, ...historical } = row;
  assert(schemaVersion);
  assert.throws(() => accountingSnapshot([historical]), /Incomplete/);
  assert.throws(
    () => compareAccounting({ ...a, projection: ["PK"] }, b),
    /projections differ/,
  );
  assert.throws(
    () => accountingSnapshot([{ ...row, unreviewedCounter: { N: "1" } }]),
    /inconsistent/,
  );
  assert.throws(() => accountingSnapshot([row, row]), /Duplicate/);
  for (const changed of [
    { ...row, schemaVersion: { N: "2" } },
    { ...row, kind: { S: "unknown" } },
    { ...row, version: { N: "-1" } },
  ])
    assert.throws(() => accountingSnapshot([changed]), /inconsistent/);
  assert.throws(
    () => compareAccounting(a, { ...b, sha256: "wrong" }),
    /checksum mismatch/,
  );
});
test("actual subprocess wrapper supplies bounded integer timeout and never executes at expiry", () => {
  let calls = 0;
  const run = (command, args, options) => {
    calls++;
    assert.equal(options.timeout, 14835);
    assert.equal(command, "local");
    return { status: 0 };
  };
  assert.equal(
    runCollectorOnce("local", [], {
      deadlineMs: 19835.81689453125,
      now: () => 0,
      run,
    }).status,
    0,
  );
  assert.equal(calls, 1);
  assert.throws(
    () =>
      runCollectorOnce("local", [], { deadlineMs: 5000, now: () => 0, run }),
    /exhausted/,
  );
  assert.equal(calls, 1);
  for (const value of [NaN, Infinity, -1])
    assert.throws(() => subprocessTimeout(value, 0));
  const result = runCollectorOnce(process.execPath, ["-e", ""], {
    deadlineMs: Date.now() + 15000.5,
    reserveMs: 1000,
  });
  assert.equal(result.status, 0);
});
