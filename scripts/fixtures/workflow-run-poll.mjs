// Local Go regression bridge only. No public endpoint/AWS/credential support.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createCoordinatorRunReader } from "../lib/workflow-run-poll.mjs";

const input = JSON.parse(readFileSync(0, "utf8"));
const origin = new URL(input.origin);
assert.equal(origin.protocol, "http:");
assert.equal(origin.hostname, "127.0.0.1");
assert.ok(origin.port);
assert.equal(origin.pathname, "/");
const ledger = {
  startedAt: new Date().toISOString(),
  apiCalls: 0,
  polls: 0,
  admittedSteps: 2,
  expectedSends: 0,
};
let writes = 0;
const reader = createCoordinatorRunReader({
  ledger,
  budget: () => {}, // Fixture has no external I/O; live wiring requires meter.
  persist: () => {
    writes++;
  },
  exchangeOnce: async ({ method, route, signal }) => {
    assert.equal(method, "GET");
    assert.equal(writes, ledger.apiCalls);
    const response = await fetch(new URL(route, origin), {
      method,
      signal,
      redirect: "error",
    });
    const data = await response.json();
    return {
      status: response.status,
      data: data.data,
      error: data.error,
      headers: response.headers,
    };
  },
});
const run = await reader.pollSubmitted(input.projectId, {
  status: 202, // Go already admitted it once, before starting this read client.
  data: { run: { workflowRunId: input.runId, state: "queued" } },
  headers: new Headers(),
});
assert.equal(run.state, "completed");
assert.equal(ledger.apiCalls, 2);
assert.equal(ledger.polls, 2);
assert.equal(ledger.admittedSteps, 2);
assert.equal(ledger.expectedSends, 0);
console.log(
  JSON.stringify({ getAttempts: 2, admittedByReader: 0, sendsByReader: 0 }),
);
