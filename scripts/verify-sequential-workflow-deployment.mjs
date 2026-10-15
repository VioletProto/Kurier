// Read-only deployed package/configuration/routes/IAM proof before activation.
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
  if (r.status !== 0) throw Error("Workflow deployment inspection failed.");
  return r.stdout.trim() ? JSON.parse(r.stdout) : {};
}
const api = new URL(o.apiUrl).hostname.split(".")[0];
const routes = aws(["apigatewayv2", "get-routes", "--api-id", api]);
const integrations = aws(["apigatewayv2", "get-integrations", "--api-id", api]);
if (routes.NextToken || integrations.NextToken)
  throw Error("Incomplete route inventory");
for (const key of [
  "POST /api/v1/projects/{projectId}/workflows",
  "GET /api/v1/projects/{projectId}/workflows",
  "GET /api/v1/projects/{projectId}/workflows/{workflowId}",
  "PATCH /api/v1/projects/{projectId}/workflows/{workflowId}",
  "DELETE /api/v1/projects/{projectId}/workflows/{workflowId}",
  "POST /api/v1/projects/{projectId}/workflows/{workflowId}/runs",
  "GET /api/v1/projects/{projectId}/workflow-runs",
  "GET /api/v1/projects/{projectId}/workflow-runs/{workflowRunId}",
  "POST /api/v1/projects/{projectId}/workflow-runs/{workflowRunId}/rerun",
]) {
  const route = routes.Items.find((r) => r.RouteKey === key);
  const integration = integrations.Items.find(
    (i) => route?.Target === "integrations/" + i.IntegrationId,
  );
  if (
    !integration?.IntegrationUri?.endsWith(":function:" + o.apiFunction) ||
    integration.PayloadFormatVersion !== "2.0" ||
    route.AuthorizationType !== "NONE"
  )
    throw Error("Workflow middleware route mismatch: " + key);
}
for (const [name, timeout] of [
  [o.apiFunction, 15],
  [o.cleanupFunction, 60],
  [o.workerFunction, 90],
]) {
  const f = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    name,
  ]);
  const e = f.Environment?.Variables ?? {};
  if (
    f.MemorySize !== 256 ||
    f.Timeout !== timeout ||
    e.KURIER_SEQUENTIAL_WORKFLOWS_SCHEMA_VERSION !== "1" ||
    e.KURIER_LOCAL_AGENT_EXECUTIONS_SCHEMA_VERSION !== "1" ||
    e.KURIER_PROTECTED_TABLE !== o.protectedTable ||
    e.KURIER_RECOVERY_GENERATION ||
    (name !== o.workerFunction &&
      e.KURIER_SAVED_REQUESTS_SCHEMA_VERSION !== "6")
  )
    throw Error("Workflow consumer mismatch");
  const role = f.Role.split("/").at(-1);
  const listed = aws(["iam", "list-role-policies", "--role-name", role]);
  const attached = aws([
    "iam",
    "list-attached-role-policies",
    "--role-name",
    role,
  ]);
  if (listed.IsTruncated || attached.IsTruncated)
    throw Error("Incomplete IAM inventory");
  // Existing SST roles also carry AWS's basic logging policy. Inspect its
  // current version and reject any added data or crypto permissions.
  for (const policy of attached.AttachedPolicies) {
    if (
      policy.PolicyArn !==
      "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
    )
      throw Error("Unexpected attached role policy");
    const metadata = aws([
      "iam",
      "get-policy",
      "--policy-arn",
      policy.PolicyArn,
    ]);
    const version = aws([
      "iam",
      "get-policy-version",
      "--policy-arn",
      policy.PolicyArn,
      "--version-id",
      metadata.Policy.DefaultVersionId,
    ]);
    if (
      version.PolicyVersion.Document.Statement.some(
        (s) =>
          s.Effect !== "Allow" ||
          [s.Action]
            .flat()
            .some(
              (a) =>
                ![
                  "logs:CreateLogGroup",
                  "logs:CreateLogStream",
                  "logs:PutLogEvents",
                ].includes(a),
            ),
      )
    )
      throw Error("Managed logging policy acquired unreviewed permissions");
  }
  const statements = listed.PolicyNames.flatMap(
    (p) =>
      aws(["iam", "get-role-policy", "--role-name", role, "--policy-name", p])
        .PolicyDocument.Statement,
  );
  const has = (s, a) => [s.Action].flat().includes(a);
  const kms = statements.filter((s) =>
    [s.Action].flat().some((a) => a.startsWith("kms:")),
  );
  const allowed =
    name === o.apiFunction
      ? {
          "kms:Decrypt": ["saved-secret", "job-bindings"],
          "kms:GenerateDataKey": [
            "saved-secret",
            "job-bindings",
            "workflow-inputs",
          ],
        }
      : name === o.workerFunction
        ? {
            "kms:Decrypt": [
              "job-bindings",
              "workflow-inputs",
              "workflow-output",
            ],
            "kms:GenerateDataKey": ["workflow-output"],
          }
        : {};
  for (const s of kms) {
    const c = s.Condition?.StringEquals;
    if (
      s.Effect !== "Allow" ||
      [s.Resource].flat().join() !== o.protectedKeyArn ||
      c?.["kms:EncryptionContext:app"] !== "kurier" ||
      c?.["kms:EncryptionContext:stage"] !== "dev-api" ||
      [s.Action]
        .flat()
        .some(
          (a) => !allowed[a]?.includes(c?.["kms:EncryptionContext:purpose"]),
        )
    )
      throw Error("Unscoped KMS permission");
  }
  for (const [action, purposes] of Object.entries(allowed))
    for (const purpose of purposes)
      if (
        !kms.some(
          (s) =>
            has(s, action) &&
            s.Condition.StringEquals["kms:EncryptionContext:purpose"] ===
              purpose,
        )
      )
        throw Error("Missing purpose-scoped KMS permission");
  if (name === o.workerFunction) {
    if (
      !statements.some(
        (s) =>
          has(s, "dynamodb:PutItem") &&
          [s.Resource].flat().join() ===
            `arn:aws:dynamodb:us-east-2:747336059622:table/${o.protectedTable}`,
      ) ||
      !statements.some(
        (s) =>
          has(s, "sqs:SendMessage") &&
          [s.Resource].flat().join() === o.executionQueueArn,
      )
    )
      throw Error("Missing scoped worker output/scheduling permission");
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
  !targets.Targets[0].Arn.endsWith(":function:" + o.cleanupFunction)
)
  throw Error("Maintenance schedule mismatch");
console.log(
  JSON.stringify({
    workflowConsumers: 3,
    routes: 9,
    purposeScopedIAM: true,
    cleanupSchedule: "rate(1 minute)",
    verified: true,
  }),
);
