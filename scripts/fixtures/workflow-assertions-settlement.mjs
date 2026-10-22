// Test-only bridge from actual local service rows to the production pure guard.
// No AWS adapter, credentials, write mode or network access.
import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import {
  execute,
  projectionFields,
} from "../lib/workflow-assertions-activation.mjs";
if (process.argv[2] === "--fields") {
  console.log(JSON.stringify(projectionFields));
} else {
  assert.equal(process.argv.length, 2);
  const input = JSON.parse(readFileSync(0, "utf8"));
  const before = structuredClone(input);
  let writes = 0;
  const platform = {
    account: "747336059622",
    region: "us-east-2",
    stage: "dev-api",
    controls: {},
    functions: [],
  };
  const names = [
    "UsersProjectsApi",
    "EmptyProjectCleanup",
    "CloudExecutionWorker",
    "ExecutionControlledEndpoint",
  ];
  const release = {
    preview: {
      packages: names.map((name) => ({ name, sha256: "a".repeat(64) })),
    },
  };
  platform.functions = names.map((resource) => ({
    resource,
    state: "Active",
    update: "Successful",
    codeSha256: "old",
  }));
  const adapter = {
    identity: async () => ({
      Account: "747336059622",
      Arn: "arn:aws:iam::747336059622:user/local-fixture",
    }),
    marker: async () => input.marker,
    platform: async () => platform,
    frontendReady: async () => false,
    rows: async () => input.rows,
    queueDepth: async () => ({ visible: "0", inflight: "0", delayed: "0" }),
    transact: async () => {
      writes++;
      throw Error("Inspection must never write");
    },
  };
  try {
    const result = await execute(adapter, {
      action: "inspect",
      baseline: platform,
      release,
    });
    assert.equal(result.writes, 0);
    assert.deepEqual(input, before);
    console.log(JSON.stringify({ accepted: true, writes, result }));
  } catch (error) {
    assert.equal(writes, 0);
    assert.deepEqual(input, before);
    console.log(
      JSON.stringify({ accepted: false, writes, message: error.message }),
    );
    process.exitCode = 1;
  }
}
