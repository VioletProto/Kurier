// Read-only package, protocol, IAM and consumer verification before activation.
import "./verify-evidence-pinning-deployment.mjs";
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
const o = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (r.status !== 0) throw Error("Local deployment inspection failed.");
  return JSON.parse(r.stdout);
}
const apiId = new URL(o.apiUrl).hostname.split(".")[0];
const routes = aws(["apigatewayv2", "get-routes", "--api-id", apiId]);
const integrations = aws([
  "apigatewayv2",
  "get-integrations",
  "--api-id",
  apiId,
]);
if (routes.NextToken || integrations.NextToken)
  throw Error("Incomplete route inventory.");
for (const key of [
  "POST /api/v1/local-agents",
  "GET /api/v1/local-agents/current",
  "DELETE /api/v1/local-agents/{localAgentId}",
  "GET /api/v1/local-agent/session",
  "DELETE /api/v1/local-agent/session",
  "POST /api/v1/projects/{projectId}/local-agent/poll",
  "POST /api/v1/projects/{projectId}/local-agent/jobs/{jobId}/result",
]) {
  const route = routes.Items.find((r) => r.RouteKey === key);
  const integration = integrations.Items.find(
    (i) => route?.Target === `integrations/${i.IntegrationId}`,
  );
  if (
    !integration?.IntegrationUri?.includes(o.apiFunction) ||
    integration.PayloadFormatVersion !== "2.0" ||
    route.AuthorizationType !== "NONE"
  )
    throw Error("Agent/browser middleware route mismatch.");
}
for (const name of [o.apiFunction, o.cleanupFunction, o.workerFunction]) {
  const fn = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    name,
  ]);
  const env = fn.Environment.Variables;
  if (
    env.KURIER_LOCAL_AGENT_EXECUTIONS_SCHEMA_VERSION !== "1" ||
    (name !== o.workerFunction &&
      env.KURIER_SAVED_REQUESTS_SCHEMA_VERSION !== "5")
  )
    throw Error("Consumer capability support mismatch.");
  const role = fn.Role.split("/").at(-1);
  const listed = aws(["iam", "list-role-policies", "--role-name", role]);
  if (listed.IsTruncated) throw Error("Incomplete role inventory.");
  const statements = listed.PolicyNames.flatMap(
    (policy) =>
      aws([
        "iam",
        "get-role-policy",
        "--role-name",
        role,
        "--policy-name",
        policy,
      ]).PolicyDocument.Statement,
  );
  const has = (s, a) => [s.Action].flat().includes(a);
  const decrypt = statements.filter((s) => has(s, "kms:Decrypt"));
  if (name === o.cleanupFunction && decrypt.length)
    throw Error("Maintenance acquired KMS decrypt.");
  if (name === o.apiFunction) {
    if (
      !statements.some(
        (s) =>
          has(s, "s3:PutObject") &&
          [s.Resource].flat().join() ===
            `arn:aws:s3:::${o.evidenceBucket}/dev-api/projects/*`,
      )
    )
      throw Error("API publication prefix mismatch.");
    if (
      !decrypt.some(
        (s) =>
          [s.Resource].flat().includes(o.protectedKeyArn) &&
          s.Condition?.StringEquals?.["kms:EncryptionContext:purpose"] ===
            "job-bindings" &&
          s.Condition?.StringEquals?.["kms:EncryptionContext:stage"] ===
            "dev-api",
      )
    )
      throw Error("API job-binding decrypt scope mismatch.");
  }
}
const rule = aws(["events", "describe-rule", "--name", o.cleanupSchedule]);
const targets = aws([
  "events",
  "list-targets-by-rule",
  "--rule",
  o.cleanupSchedule,
]);
if (
  rule.State !== "ENABLED" ||
  rule.ScheduleExpression !== "rate(1 minute)" ||
  targets.NextToken ||
  targets.Targets.length !== 1 ||
  !targets.Targets[0].Arn.endsWith(`:function:${o.cleanupFunction}`)
)
  throw Error("Scheduled cleanup mismatch.");
console.log(
  JSON.stringify({
    localConsumers: 3,
    routes: 7,
    scopedPublicationIAM: true,
    cleanupSchedule: "rate(1 minute)",
    verified: true,
  }),
);
