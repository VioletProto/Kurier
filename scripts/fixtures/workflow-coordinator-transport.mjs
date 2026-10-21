// Runs only against the isolated meter's controlled local origin; no AWS calls.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  createCoordinatorTransport,
  coordinatorBudget,
  persistCoordinatorLedger,
} from "../lib/workflow-coordinator-transport.mjs";
const directory = process.env.KURIER_LOCAL_COORDINATOR_TEST;
assert(directory);
const events = () =>
  readFileSync(directory + "/ledger.jsonl", "utf8")
    .trim()
    .split("\n")
    .map(JSON.parse);
const first = events().find((e) => e.event === "attempt");
const ledger = {
  startedAt: new Date(first.at * 1000).toISOString(),
  apiCalls: 0,
  polls: 0,
  admittedSteps: 4,
  expectedSends: 4,
};
const budget = coordinatorBudget({
  ledger,
  meterPath: directory + "/ledger.jsonl",
});
const persist = () =>
  persistCoordinatorLedger(directory + "/coordinator.json", ledger);
const transport = createCoordinatorTransport({
  origin: "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com",
  ledger,
  token: async () => "local-private-canary",
  budget,
  persist,
});
const project = "11111111-1111-4111-8111-111111111111",
  run = "22222222-2222-4222-8222-222222222222";
const before = events().filter((e) => e.event === "attempt").length;
const result = await transport.pollSubmitted(project, {
  status: 202,
  data: { run: { workflowRunId: run, state: "queued" } },
  headers: new Headers(),
});
assert.equal(result.state, "completed");
assert.equal(ledger.apiCalls, 2);
assert.equal(ledger.polls, 2);
assert.equal(events().filter((e) => e.event === "attempt").length - before, 2);
assert.equal(ledger.admittedSteps, 4);
assert.equal(ledger.expectedSends, 4);
assert.equal(
  JSON.parse(readFileSync(directory + "/coordinator.json")).polls,
  2,
);
await assert.rejects(transport.call("GET", "/api/v1/redirect"), /uncertain/);
assert.equal(ledger.apiCalls, 3);
assert.equal(ledger.polls, 2);
console.log(
  JSON.stringify({
    realCoordinator503Then200: true,
    redirectRejected: true,
    admissions: 0,
    targetSends: 0,
    getAttempts: 3,
  }),
);
