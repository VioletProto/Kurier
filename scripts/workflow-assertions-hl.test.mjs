import test from "node:test";
import assert from "node:assert/strict";
import {
  actions,
  checkAdmission,
} from "./lib/workflow-assertions-hl-policy.mjs";
test("H-L-only sequential admissions preserve prior usage and exhaust zero reserve", () => {
  const ledger = {
      preflightVerified: true,
      users: [{}],
      projects: [{}],
      runs: [],
      admittedSteps: 18,
      expectedSends: 16,
    },
    workflow = { steps: [{}, {}] };
  for (const [key, state, sends] of [
    ["H", "completed", 2],
    ["I", "completed", 2],
    ["J", "failed", 1],
    ["K", "failed", 2],
    ["L", "failed", 1],
  ]) {
    const options = {
      state,
      sends,
      ...(key === "I" ? { source: "frozen-H" } : {}),
    };
    for (const bad of ["A", "B", "C", "D", "E", "F", "G", "M"])
      assert.throws(() => checkAdmission(ledger, bad, workflow, options));
    assert.throws(() =>
      checkAdmission({ ...ledger, projects: [{}, {}] }, key, workflow, options),
    );
    assert.throws(() => checkAdmission(ledger, key, { steps: [{}] }, options));
    assert.throws(() =>
      checkAdmission(
        { ...ledger, preflightVerified: false },
        key,
        workflow,
        options,
      ),
    );
    assert.equal(checkAdmission(ledger, key, workflow, options), true);
    ledger.runs.push({ key });
    ledger.admittedSteps += 2;
    ledger.expectedSends += sends;
    assert.throws(() => checkAdmission(ledger, key, workflow, options));
  }
  assert.equal(ledger.admittedSteps, 28);
  assert.equal(ledger.expectedSends, 24);
  assert.throws(() =>
    checkAdmission(ledger, "L", workflow, { state: "failed", sends: 1 }),
  );
  assert(!actions.some((x) => /deploy|activate|close|campaign-first/.test(x)));
});

test("refreshed baseline requires completed matched cleanup and deduplicates multi-day owner", async () => {
  const { refreshedBaseline } =
    await import("./lib/workflow-assertions-hl-policy.mjs");
  const report = {
    kind: "kurier-existing-instructor-deletion-observation-v1",
    verified: true,
    checks: { physical: true },
    projectId: "local-project",
    deletionOperationId: "local-operation",
    stage: {},
    finalTombstone: {
      projectId: { S: "local-project" },
      deletionOperation: {
        M: { operationId: { S: "local-operation" }, state: { S: "completed" } },
      },
    },
    accountingAfter: {
      rows: [
        {
          PK: { S: "STAGE#dev-api" },
          count: { N: "22" },
          version: { N: "54" },
          pinnedCount: { N: "0" },
          pinnedBytes: { N: "0" },
        },
        { PK: { S: "U#owner" }, SK: { S: "day20" } },
        { PK: { S: "U#owner" }, SK: { S: "day21" } },
      ],
    },
  };
  assert.deepEqual(refreshedBaseline(report).owners, ["U#owner"]);
  for (const bad of [
    { ...report, verified: false },
    { ...report, stopped: true },
    { ...report, checks: { physical: false } },
    { ...report, deletionOperationId: "other" },
  ])
    assert.throws(() => refreshedBaseline(bad));
});
