// Read-only verification. Print only scoped configuration, never environments.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
const outputs = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args, absentAllowed = false) {
  const result = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (result.status !== 0) {
    if (absentAllowed && result.stderr.includes("ResourceNotFoundException"))
      return {};
    throw new Error(
      `Configuration inspection failed: ${args.slice(0, 2).join(" ")}`,
    );
  }
  return result.stdout.trim() ? JSON.parse(result.stdout) : {};
}
function requireCheck(ok, name) {
  if (!ok) throw new Error(`Configuration check failed: ${name}`);
}
const identity = aws(["sts", "get-caller-identity"]);
requireCheck(
  identity.Account === "747336059622" &&
    !identity.Arn.endsWith(":root") &&
    outputs.stage === "dev-api",
  "intended non-root development account",
);
const name = outputs.workerFunction;
requireCheck(
  name?.startsWith("kurier-dev-api-CloudExecutionWorkerFunction-"),
  "scoped worker",
);
const fn = aws([
  "lambda",
  "get-function-configuration",
  "--function-name",
  name,
]);
requireCheck(
  fn.State === "Active" && fn.LastUpdateStatus === "Successful",
  "worker readiness",
);
const reserved = aws([
  "lambda",
  "get-function-concurrency",
  "--function-name",
  name,
]);
requireCheck(
  reserved.ReservedConcurrentExecutions === undefined,
  "no reserved concurrency",
);
requireCheck(
  (
    aws([
      "lambda",
      "list-provisioned-concurrency-configs",
      "--function-name",
      name,
    ]).ProvisionedConcurrencyConfigs ?? []
  ).length === 0,
  "no provisioned concurrency",
);
const mappings = aws([
  "lambda",
  "list-event-source-mappings",
  "--function-name",
  name,
]);
requireCheck(
  !mappings.NextMarker && mappings.EventSourceMappings?.length === 1,
  "exactly one event source",
);
const mapping = mappings.EventSourceMappings[0];
requireCheck(
  mapping.EventSourceArn === outputs.executionQueueArn &&
    mapping.State === "Enabled" &&
    mapping.BatchSize === 1 &&
    mapping.ScalingConfig?.MaximumConcurrency === 2 &&
    !mapping.ProvisionedPollerConfig &&
    mapping.FunctionResponseTypes?.includes("ReportBatchItemFailures"),
  "standard SQS maximum concurrency two, batch one and partial failures",
);
requireCheck(
  (
    aws(["lambda", "list-function-url-configs", "--function-name", name])
      .FunctionUrlConfigs ?? []
  ).length === 0,
  "no function URL",
);
requireCheck(
  !aws(["lambda", "get-policy", "--function-name", name], true).Policy,
  "no service invocation resource policy",
);
const apiId = new URL(outputs.apiUrl).hostname.split(".")[0];
const integrations = aws([
  "apigatewayv2",
  "get-integrations",
  "--api-id",
  apiId,
]);
requireCheck(
  !integrations.NextToken &&
    !(integrations.Items ?? []).some((i) => i.IntegrationUri?.includes(name)),
  "no API worker integration",
);
const rules = aws([
  "events",
  "list-rule-names-by-target",
  "--target-arn",
  fn.FunctionArn,
]);
requireCheck(
  !rules.NextToken && (rules.RuleNames ?? []).length === 0,
  "no EventBridge worker target",
);
const schedules = aws(["scheduler", "list-schedules"]);
requireCheck(
  !schedules.NextToken &&
    !(schedules.Schedules ?? []).some((s) => s.Target?.Arn === fn.FunctionArn),
  "no Scheduler worker target",
);
const settings = aws(["lambda", "get-account-settings"]);
requireCheck(
  settings.AccountLimit.ConcurrentExecutions === 10 &&
    settings.AccountLimit.UnreservedConcurrentExecutions === 10,
  "unchanged account concurrency ten",
);
const plan = aws(["freetier", "get-account-plan-state"]);
requireCheck(
  plan.accountPlanType === "FREE" && plan.accountPlanStatus === "ACTIVE",
  "unchanged active Free plan",
);
console.log(
  JSON.stringify(
    {
      worker: name,
      queueOnlyConfiguredTrigger: true,
      queueMaximumConcurrency: 2,
      batchSize: 1,
      standardPollers: true,
      reservedConcurrency: false,
      provisionedConcurrency: false,
      accountConcurrency: 10,
      activeFreePlan: true,
    },
    null,
    2,
  ),
);
