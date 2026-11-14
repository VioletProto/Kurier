import test from "node:test";
import assert from "node:assert/strict";
import {
  checkTuple,
  transitionInput,
  planSummary,
  checkPlan,
} from "./lib/mcp-scopes-release-guards.mjs";
const marker = {
  PK: { S: "STAGE#dev-api" },
  SK: { S: "META" },
  state: { S: "active" },
  version: { N: "19" },
  savedRequestsSchemaVersion: { N: "10" },
  mcpReadOnlySchemaVersion: { N: "1" },
  recoveryGeneration: { S: "dev-api-initial-v1" },
};
for (const [key, n] of Object.entries({
  openapiSchemaVersion: 1,
  projectEnvironmentsSchemaVersion: 1,
  protectedSecretsSchemaVersion: 2,
  localAgentExecutionsSchemaVersion: 2,
  workflowAssertionsSchemaVersion: 1,
  cloudExecutionsSchemaVersion: 1,
  executionInputsSchemaVersion: 1,
  evidencePinningSchemaVersion: 1,
  sequentialWorkflowsSchemaVersion: 1,
}))
  marker[key] = { N: String(n) };
test("close and activation condition every observed field and preserve generation", () => {
  const close = transitionInput(marker, false, "control");
  assert.equal(close.TableName, "control");
  assert.equal(
    close.ExpressionAttributeValues[":nextState"].S,
    "mcp-scopes-migration",
  );
  assert.equal(close.ExpressionAttributeValues[":nextVersion"].N, "20");
  assert.equal(
    Object.keys(close.ExpressionAttributeNames).length,
    Object.keys(marker).length + 2,
  );
  assert.ok(!close.UpdateExpression.includes("recoveryGeneration"));
  assert.ok(!close.UpdateExpression.includes("savedRequests"));
  const activate = transitionInput(
    { ...marker, state: { S: "mcp-scopes-migration" }, version: { N: "20" } },
    true,
    "control",
  );
  assert.equal(activate.ExpressionAttributeValues[":nextVersion"].N, "21");
  assert.equal(activate.ExpressionAttributeValues[":schema"].N, "11");
  assert.equal(activate.ExpressionAttributeValues[":mcp"].N, "2");
  assert.ok(!activate.UpdateExpression.includes("recoveryGeneration"));
});
test("wrong tuple/state/generation rejects before a mutation can be formed", () => {
  for (const k of [
    "state",
    "savedRequestsSchemaVersion",
    "mcpReadOnlySchemaVersion",
    "recoveryGeneration",
    "openapiSchemaVersion",
    "localAgentExecutionsSchemaVersion",
  ]) {
    assert.throws(() =>
      transitionInput({ ...marker, [k]: { S: "wrong", N: "99" } }),
    );
  }
  assert.throws(() => transitionInput(marker, true));
  assert.throws(() =>
    checkTuple(
      { ...marker, mcpReadOnlySchemaVersion: { N: "2" } },
      "active",
      10,
      1,
    ),
  );
});
test("preview requires complete JSON and retains exact operation and property names", () => {
  const raw =
    JSON.stringify(
      [
        { op: "read", urn: "x::ignored" },
        {
          op: "update",
          type: "aws:lambda/function:Function",
          urn: "x::Api",
          detailedDiff: { code: { kind: "update" } },
        },
      ],
      null,
      2,
    ) + "\npublic output";
  assert.deepEqual(planSummary(raw), [
    {
      op: "update",
      type: "aws:lambda/function:Function",
      name: "Api",
      properties: ["code"],
    },
  ]);
  assert.throws(() => planSummary("[incomplete"));
});

test("resource review rejects new IAM, tables and unrelated mutations", () => {
  for (const type of [
    "aws:iam/rolePolicy:RolePolicy",
    "aws:dynamodb/table:Table",
    "aws:lambda/function:Function",
  ]) {
    assert.throws(() =>
      checkPlan([{ op: "update", type, name: "Unrelated", properties: [] }]),
    );
  }
});
