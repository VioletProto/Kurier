import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  coordinatorBudget,
  createCoordinatorTransport,
} from "../lib/workflow-coordinator-transport.mjs";

const path = process.env.KURIER_LOCAL_COORDINATOR_TEST + "/ledger.jsonl";
const origin = "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com";
// One dropped LOCAL upstream connection. No client retry.
try {
  const response = await fetch(origin + "/api/v1/fail", {
    headers: { Authorization: "Bearer local-private-canary" },
  });
  assert(response.status >= 500);
} catch (error) {
  assert(error instanceof TypeError);
}
const rows = readFileSync(path, "utf8").trim().split("\n").map(JSON.parse);
assert(rows.some((r) => r.event === "stop"));
const first = rows.find((r) => r.event === "attempt");
const ledger = {
  startedAt: new Date(Math.floor(first.at * 1000)).toISOString(),
  apiCalls: 0,
};
const budget = coordinatorBudget({ ledger, meterPath: path });
let auth = 0;
const transport = createCoordinatorTransport({
  origin,
  ledger,
  budget,
  persist() {},
  token() {
    auth++;
    throw Error("must not authenticate");
  },
});
for (const method of ["GET", "POST", "DELETE"])
  await assert.rejects(transport.call(method, "/api/v1/projects"));
assert.equal(auth, 0);
assert.equal(ledger.apiCalls, 0);
assert.throws(() => budget({ cleanup: true }));
// Bypassing the coordinator still cannot bypass the proxy's stop.
for (const method of ["GET", "POST", "DELETE"])
  assert.equal(
    (await fetch(origin + "/api/v1/projects", { method })).status,
    599,
  );
