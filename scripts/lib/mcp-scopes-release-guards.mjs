import assert from "node:assert/strict";
export function checkTuple(m, state, schema, mcp) {
  assert.equal(m.state?.S, state);
  assert.equal(m.savedRequestsSchemaVersion?.N, String(schema));
  assert.equal(m.mcpReadOnlySchemaVersion?.N, String(mcp));
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
    assert.equal(m[key]?.N, String(n));
  assert.equal(m.recoveryGeneration?.S, "dev-api-initial-v1");
}
export function transitionInput(before, activate = false, table = "fixture") {
  checkTuple(before, activate ? "mcp-scopes-migration" : "active", 10, 1);
  const names = {},
    values = {};
  const conditions = [];
  for (const [i, [k, v]] of Object.entries(before).entries()) {
    names["#f" + i] = k;
    values[":f" + i] = v;
    conditions.push("#f" + i + " = :f" + i);
  }
  names["#state"] = "state";
  names["#version"] = "version";
  values[":nextState"] = { S: activate ? "active" : "mcp-scopes-migration" };
  values[":nextVersion"] = { N: String(Number(before.version.N) + 1) };
  let update = "SET #state = :nextState, #version = :nextVersion";
  if (activate) {
    values[":schema"] = { N: "11" };
    values[":mcp"] = { N: "2" };
    update +=
      ", savedRequestsSchemaVersion = :schema, mcpReadOnlySchemaVersion = :mcp";
  }
  return {
    TableName: table,
    Key: { PK: before.PK, SK: before.SK },
    ConditionExpression: conditions.join(" AND "),
    UpdateExpression: update,
    ExpressionAttributeNames: names,
    ExpressionAttributeValues: values,
  };
}
export function planSummary(raw) {
  const end = raw.lastIndexOf("\n]") + 2;
  assert.ok(end > 1, "Complete JSON plan required");
  return JSON.parse(raw.slice(0, end))
    .filter((x) => x.op !== "read")
    .map((x) => ({
      op: x.op,
      type: x.type,
      name: x.urn.split("::").at(-1),
      properties: Object.keys(x.detailedDiff ?? {}),
    }));
}

export function checkPlan(changes) {
  const functions = [
    "UsersProjectsApi",
    "CloudExecutionWorker",
    "EmptyProjectCleanup",
    "BrowserSessionBroker",
    "ExecutionControlledEndpoint",
  ];
  for (const c of changes) {
    if (
      c.op === "update" &&
      c.type === "pulumi-nodejs:dynamic:Resource" &&
      [
        "FrontendWebKvKeys.sst.aws.KvKeys",
        "FrontendWebAssetFiles.sst.aws.BucketFiles",
        "FrontendWebInvalidation.sst.aws.DistributionInvalidation",
      ].includes(c.name)
    )
      continue;
    if (
      c.type === "aws:s3/bucketObjectv2:BucketObjectv2" &&
      functions.some((n) => c.name === n + "Code") &&
      ["create-replacement", "replace", "delete-replaced"].includes(c.op) &&
      c.properties.every((k) => ["key", "source", "versionId"].includes(k))
    )
      continue;
    if (
      c.op === "update" &&
      c.type === "aws:lambda/function:Function" &&
      functions.some((n) => c.name === n + "Function") &&
      c.properties.every((k) =>
        [
          "s3Key",
          "lastModified",
          "environment.variables.KURIER_MCP_READ_ONLY_SCHEMA_VERSION",
        ].includes(k),
      )
    )
      continue;
    if (
      c.op === "create" &&
      /^DevelopmentHttpApiRoute(Koanhm|Bakfkn|Ebmcab)(Permissions|Integration|Route)?$/.test(
        c.name,
      ) &&
      [
        "sst:aws:ApiGatewayV2LambdaRoute",
        "aws:lambda/permission:Permission",
        "aws:apigatewayv2/integration:Integration",
        "aws:apigatewayv2/route:Route",
      ].includes(c.type)
    )
      continue;
    throw Error("Unreviewed resource change: " + c.type + " " + c.name);
  }
  assert.equal(
    changes.filter(
      (c) => c.type === "aws:apigatewayv2/route:Route" && c.op === "create",
    ).length,
    3,
  );
}
