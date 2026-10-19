// Only inventory and SST diff. There is deliberately no deploy/refresh mode.
import { readFileSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";

const preview = process.argv[2] === "--diff";
if (process.argv.length !== (preview ? 3 : 2))
  throw Error("Choose read-only inventory or --diff.");
const root = new URL("../", import.meta.url);
const out = JSON.parse(
  readFileSync(new URL(".sst/outputs.json", root), "utf8"),
);
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8", maxBuffer: 32 * 1024 * 1024 },
  );
  if (r.status !== 0)
    throw Error(`Read-only lookup failed: ${args.slice(0, 2).join(" ")}`);
  return r.stdout.trim() ? JSON.parse(r.stdout) : {};
}
const canonical = (value) =>
  Array.isArray(value)
    ? value.map(canonical)
    : value && typeof value === "object"
      ? Object.fromEntries(
          Object.keys(value)
            .sort()
            .map((key) => [key, canonical(value[key])]),
        )
      : value;
const hash = (value) =>
  createHash("sha256")
    .update(JSON.stringify(canonical(value)))
    .digest("hex");
const identity = aws(["sts", "get-caller-identity"]);
if (
  identity.Account !== "747336059622" ||
  identity.Arn.endsWith(":root") ||
  out.stage !== "dev-api" ||
  out.region !== "us-east-2"
)
  throw Error("Intended non-root account/stage required.");
if (
  JSON.parse(readFileSync(new URL("node_modules/sst/package.json", root)))
    .version !== "4.17.1" ||
  readFileSync(new URL(".sst/platform/version", root), "utf8").trim() !==
    "4.17.1"
)
  throw Error("Only reviewed SST 4.17.1 is permitted.");
const bootstrapParameter = aws([
  "ssm",
  "get-parameter",
  "--name",
  "/sst/bootstrap",
]).Parameter;
const bootstrap = JSON.parse(bootstrapParameter.Value);
if (
  bootstrap.version !== 5 ||
  bootstrap.state !== "sst-state-moshrdrrbeba" ||
  bootstrap.asset !== "sst-asset-moshrdrrbeba" ||
  bootstrap.assetEcrRegistryId !== identity.Account ||
  bootstrap.assetEcrUrl !==
    "747336059622.dkr.ecr.us-east-2.amazonaws.com/sst-asset" ||
  bootstrap.appsyncHttp ||
  bootstrap.appsyncRealtime
)
  throw Error("Unexpected bootstrap; refuse migration or creation.");
for (const bucket of [bootstrap.state, bootstrap.asset]) {
  aws(["s3api", "head-bucket", "--bucket", bucket]);
  if (
    aws(["s3api", "get-bucket-location", "--bucket", bucket])
      .LocationConstraint !== "us-east-2"
  )
    throw Error("Wrong bootstrap region.");
}
aws(["ecr", "describe-repositories", "--repository-names", "sst-asset"]);
// Confirm passphrase exists by metadata only. Never print/decrypt its value.
const passphrase = aws([
  "ssm",
  "describe-parameters",
  "--parameter-filters",
  "Key=Name,Option=Equals,Values=/sst/passphrase/kurier/dev-api",
]);
if (passphrase.Parameters?.length !== 1 || passphrase.NextToken)
  throw Error("Existing stage passphrase required; do not create one.");
const state = aws([
  "s3api",
  "list-objects-v2",
  "--bucket",
  bootstrap.state,
  "--prefix",
  "app/kurier/dev-api",
  "--max-keys",
  "10",
]);
if (!state.Contents?.length || state.IsTruncated)
  throw Error("Existing stage state required.");
const plan = aws(["freetier", "get-account-plan-state"]);
const account = aws(["lambda", "get-account-settings"]).AccountLimit;
if (
  plan.accountPlanType !== "FREE" ||
  plan.accountPlanStatus !== "ACTIVE" ||
  account.ConcurrentExecutions !== 10 ||
  account.UnreservedConcurrentExecutions !== 10
)
  throw Error("Free plan/quota/concurrency changed.");
const key = (sk) =>
  JSON.stringify({ PK: { S: "STAGE#dev-api" }, SK: { S: sk } });
const stage = aws([
  "dynamodb",
  "get-item",
  "--table-name",
  out.controlTable,
  "--key",
  key("META"),
  "--consistent-read",
]).Item;
if (
  stage.state?.S !== "active" ||
  stage.version?.N !== "13" ||
  stage.savedRequestsSchemaVersion?.N !== "7" ||
  stage.mcpReadOnlySchemaVersion?.N !== "1" ||
  stage.recoveryGeneration?.S !== "dev-api-initial-v1"
)
  throw Error("Unexpected active capability/generation baseline.");
const quota =
  aws([
    "dynamodb",
    "get-item",
    "--table-name",
    out.controlTable,
    "--key",
    key("QUOTA"),
    "--consistent-read",
  ]).Item ?? {};
function projectAccounting() {
  const projects = [];
  let cursor;
  for (let page = 0; page < 1000; page++) {
    const args = [
      "dynamodb",
      "scan",
      "--table-name",
      out.controlTable,
      "--consistent-read",
      "--limit",
      "100",
      "--no-paginate",
      "--filter-expression",
      "#kind = :project",
      "--expression-attribute-names",
      JSON.stringify({ "#kind": "kind" }),
      "--expression-attribute-values",
      JSON.stringify({ ":project": { S: "project" } }),
      "--projection-expression",
      "PK, pinnedCount, pinnedBytes, inflightCount",
    ];
    if (cursor) args.push("--exclusive-start-key", JSON.stringify(cursor));
    const response = aws(args);
    projects.push(...(response.Items ?? []));
    cursor = response.LastEvaluatedKey;
    if (!cursor)
      return {
        count: projects.length,
        sha256: hash(projects.sort((a, b) => a.PK.S.localeCompare(b.PK.S))),
      };
  }
  throw Error("Incomplete project accounting inventory.");
}
const accounting = projectAccounting();
const schedule = aws([
  "events",
  "describe-rule",
  "--name",
  out.cleanupSchedule,
]);
if (
  schedule.State !== "ENABLED" ||
  schedule.ScheduleExpression !== "rate(1 minute)"
)
  throw Error("Maintenance schedule changed.");
const gatewayId = new URL(out.apiUrl).hostname.split(".")[0];
const gateway = aws(["apigatewayv2", "get-api", "--api-id", gatewayId]);
const gatewayStage = aws([
  "apigatewayv2",
  "get-stage",
  "--api-id",
  gatewayId,
  "--stage-name",
  "$default",
]);
if (
  gateway.CorsConfiguration ||
  gatewayStage.DefaultRouteSettings?.ThrottlingRateLimit !== 10 ||
  gatewayStage.DefaultRouteSettings?.ThrottlingBurstLimit !== 20
)
  throw Error("Existing API CORS/throttles changed.");
const consumers = [];
for (const name of [
  out.apiFunction,
  out.cleanupFunction,
  out.workerFunction,
  out.controlledEndpointFunction,
]) {
  const fn = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    name,
  ]);
  if (
    fn.State !== "Active" ||
    fn.LastUpdateStatus !== "Successful" ||
    aws(["lambda", "get-function-concurrency", "--function-name", name])
      .ReservedConcurrentExecutions !== undefined ||
    (
      aws([
        "lambda",
        "list-provisioned-concurrency-configs",
        "--function-name",
        name,
      ]).ProvisionedConcurrencyConfigs ?? []
    ).length
  )
    throw Error("Consumer readiness/concurrency changed.");
  const role = fn.Role.split("/").pop();
  const policies = aws(["iam", "list-role-policies", "--role-name", role]);
  const attached = aws([
    "iam",
    "list-attached-role-policies",
    "--role-name",
    role,
  ]);
  if (policies.IsTruncated || attached.IsTruncated)
    throw Error("Incomplete IAM inventory.");
  const inline = (policies.PolicyNames ?? [])
    .sort()
    .map(
      (policy) =>
        aws([
          "iam",
          "get-role-policy",
          "--role-name",
          role,
          "--policy-name",
          policy,
        ]).PolicyDocument,
    );
  consumers.push({
    name,
    codeSha256: fn.CodeSha256,
    environmentSha256: hash(fn.Environment?.Variables ?? {}),
    role,
    inlineIamSha256: hash(inline),
    attachedIamSha256: hash(attached.AttachedPolicies ?? []),
    trustSha256: hash(
      aws(["iam", "get-role", "--role-name", role]).Role
        .AssumeRolePolicyDocument,
    ),
    memory: fn.MemorySize,
    timeout: fn.Timeout,
    reserved: false,
    provisioned: false,
  });
}
const mapping = aws([
  "lambda",
  "list-event-source-mappings",
  "--function-name",
  out.workerFunction,
]);
if (mapping.NextMarker || mapping.EventSourceMappings?.length !== 1)
  throw Error("Unexpected worker triggers.");
const m = mapping.EventSourceMappings[0];
if (
  m.EventSourceArn !== out.executionQueueArn ||
  m.State !== "Enabled" ||
  m.BatchSize !== 1 ||
  m.ScalingConfig?.MaximumConcurrency !== 2 ||
  m.ProvisionedPollerConfig
)
  throw Error("Worker concurrency/pollers changed.");
const pool = aws([
  "cognito-idp",
  "describe-user-pool",
  "--user-pool-id",
  out.cognitoUserPoolId,
]).UserPool;
const client = aws([
  "cognito-idp",
  "describe-user-pool-client",
  "--user-pool-id",
  out.cognitoUserPoolId,
  "--client-id",
  out.cognitoClientId,
]).UserPoolClient;
if (
  client.ClientSecret ||
  pool.UserPoolTier !== "ESSENTIALS" ||
  pool.EmailConfiguration?.EmailSendingAccount !== "COGNITO_DEFAULT" ||
  client.RefreshTokenRotation?.Feature !== "ENABLED" ||
  client.RefreshTokenRotation?.RetryGracePeriodSeconds !== 10 ||
  client.RefreshTokenValidity !== 1 ||
  client.AccessTokenValidity !== 15 ||
  client.AllowedOAuthFlowsUserPoolClient ||
  !client.ExplicitAuthFlows?.includes("ALLOW_USER_SRP_AUTH") ||
  client.ExplicitAuthFlows?.includes("ALLOW_REFRESH_TOKEN_AUTH")
)
  throw Error("Cognito baseline mismatch.");
const before = {
  checkedAt: new Date().toISOString(),
  account: identity.Account,
  nonRoot: true,
  region: "us-east-2",
  bootstrap: {
    version: bootstrap.version,
    parameterVersion: bootstrapParameter.Version,
    valueSha256: hash(bootstrap),
    state: bootstrap.state,
    asset: bootstrap.asset,
  },
  plan: {
    type: plan.accountPlanType,
    status: plan.accountPlanStatus,
    expiration: plan.accountPlanExpirationDate ?? null,
  },
  accountConcurrency: account.ConcurrentExecutions,
  unreserved: account.UnreservedConcurrentExecutions,
  stageSha256: hash(stage),
  quotaSha256: hash(quota),
  projectAccounting: accounting,
  schedule: { enabled: true, expression: schedule.ScheduleExpression },
  api: { gatewayCorsAbsent: true, rate: 10, burst: 20 },
  generation: stage.recoveryGeneration.S,
  stageVersion: stage.version.N,
  consumers,
  worker: { batch: 1, max: 2, standardPollers: true },
  cognito: {
    pool: out.cognitoUserPoolId,
    client: out.cognitoClientId,
    essentials: true,
    defaultEmail: true,
    rotatingRefresh: true,
  },
  productDeploymentPerformed: false,
};
writeFileSync(
  new URL(".sst/frontend-hosting-inventory.json", root),
  JSON.stringify(before, null, 2) + "\n",
);
if (preview) {
  const credentials = aws([
    "configure",
    "export-credentials",
    "--format",
    "process",
  ]);
  const result = spawnSync(
    "npm",
    ["exec", "--", "sst", "diff", "--stage", "dev-api", "--json"],
    {
      cwd: root,
      encoding: "utf8",
      maxBuffer: 32 * 1024 * 1024,
      env: {
        ...process.env,
        AWS_REGION: "us-east-2",
        AWS_ACCESS_KEY_ID: credentials.AccessKeyId,
        AWS_SECRET_ACCESS_KEY: credentials.SecretAccessKey,
        AWS_SESSION_TOKEN: credentials.SessionToken,
        KURIER_AWS_ACCOUNT_ID: identity.Account,
        KURIER_COGNITO_POOL_ID: out.cognitoUserPoolId,
        KURIER_COGNITO_CLIENT_ID: out.cognitoClientId,
        KURIER_FRONTEND_ORIGIN: out.frontendOrigin,
        SST_TELEMETRY_DISABLED: "1",
      },
    },
  );
  // Store raw preview in ignored tooling output; publish only inspected summaries.
  writeFileSync(
    new URL(".sst/frontend-hosting-diff.txt", root),
    result.stdout + result.stderr,
  );
  if (result.status !== 0)
    throw Error(
      "SST diff failed; inspect ignored .sst/frontend-hosting-diff.txt before resuming.",
    );
}
for (const beforeConsumer of consumers) {
  const fn = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    beforeConsumer.name,
  ]);
  const inlineNames = aws([
    "iam",
    "list-role-policies",
    "--role-name",
    beforeConsumer.role,
  ]);
  const inline = (inlineNames.PolicyNames ?? [])
    .sort()
    .map(
      (policy) =>
        aws([
          "iam",
          "get-role-policy",
          "--role-name",
          beforeConsumer.role,
          "--policy-name",
          policy,
        ]).PolicyDocument,
    );
  const attached = aws([
    "iam",
    "list-attached-role-policies",
    "--role-name",
    beforeConsumer.role,
  ]);
  if (
    inlineNames.IsTruncated ||
    attached.IsTruncated ||
    fn.CodeSha256 !== beforeConsumer.codeSha256 ||
    hash(fn.Environment?.Variables ?? {}) !==
      beforeConsumer.environmentSha256 ||
    hash(inline) !== beforeConsumer.inlineIamSha256 ||
    hash(attached.AttachedPolicies ?? []) !==
      beforeConsumer.attachedIamSha256 ||
    hash(
      aws(["iam", "get-role", "--role-name", beforeConsumer.role]).Role
        .AssumeRolePolicyDocument,
    ) !== beforeConsumer.trustSha256
  )
    throw Error("Consumer code/configuration/IAM changed during preview.");
}
const after = aws([
  "ssm",
  "get-parameter",
  "--name",
  "/sst/bootstrap",
]).Parameter;
if (
  after.Version !== bootstrapParameter.Version ||
  after.Value !== bootstrapParameter.Value
)
  throw Error("Bootstrap changed; stop.");
if (
  hash(
    aws([
      "dynamodb",
      "get-item",
      "--table-name",
      out.controlTable,
      "--key",
      key("META"),
      "--consistent-read",
    ]).Item,
  ) !== before.stageSha256
)
  throw Error("Stage changed during preview; inspect before proceeding.");
if (
  hash(
    aws([
      "dynamodb",
      "get-item",
      "--table-name",
      out.controlTable,
      "--key",
      key("QUOTA"),
      "--consistent-read",
    ]).Item ?? {},
  ) !== before.quotaSha256 ||
  projectAccounting().sha256 !== accounting.sha256
)
  throw Error("Accounting changed during preview; inspect before proceeding.");
const completed = {
  ...before,
  previewCompleted: preview,
  bootstrapUnchanged: true,
  consumersUnchanged: true,
  accountingUnchanged: true,
};
writeFileSync(
  new URL(".sst/frontend-hosting-inventory.json", root),
  JSON.stringify(completed, null, 2) + "\n",
);
console.log(JSON.stringify(completed, null, 2));
