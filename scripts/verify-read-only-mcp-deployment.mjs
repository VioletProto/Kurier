// Deployed package/configuration proof and exact before/after runtime IAM comparison.
import { readFileSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { pathToFileURL } from "node:url";
export const outputs = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
export const stable = (v) =>
  Array.isArray(v)
    ? v.map(stable)
    : v && typeof v === "object"
      ? Object.fromEntries(
          Object.keys(v)
            .sort()
            .map((k) => [k, stable(v[k])]),
        )
      : v;
export const hash = (v) =>
  createHash("sha256")
    .update(JSON.stringify(stable(v)))
    .digest("hex");
// Node uses socketpairs for child stdio on Linux; AWS cannot reopen those
// through /dev/stdin. Use an inherited, seekable anonymous RAM descriptor;
// secret values never enter argv, persistent files, shell interpolation or logs.
export function awsProcess(args, input) {
  const program = `import json, os, subprocess, sys
v = json.load(sys.stdin)
a = ["aws", *v["args"], "--region", "us-east-2", "--output", "json"]
fds = ()
if "input" in v:
    fd = os.memfd_create("kurier-private-cli", os.MFD_CLOEXEC)
    os.fchmod(fd, 0o600)
    os.write(fd, json.dumps(v["input"]).encode())
    os.lseek(fd, 0, os.SEEK_SET)
    a += ["--cli-input-json", "file:///proc/self/fd/" + str(fd)]
    fds = (fd,)
r = subprocess.run(a, capture_output=True, pass_fds=fds)
for fd in fds:
    os.ftruncate(fd, 0)
    os.close(fd)
sys.stdout.buffer.write(r.stdout)
sys.stderr.buffer.write(r.stderr)
sys.exit(r.returncode)
`;
  return spawnSync("python3", ["-c", program], {
    input: JSON.stringify({ args, ...(input ? { input } : {}) }),
    maxBuffer: 16 * 1024 * 1024,
  });
}
export function aws(args, input) {
  const r = awsProcess(args, input);
  if (r.status !== 0) {
    const code =
      r.stderr?.toString().match(/An error occurred \(([A-Za-z0-9]+)\)/)?.[1] ??
      "CLIOrTransport";
    throw Error(
      `Scoped AWS operation failed: ${args.slice(0, 2).join(" ")} (${code}, exit${r.status}); inspect before retrying mutations.`,
    );
  }
  const raw = r.stdout.toString("utf8");
  return raw.trim() ? JSON.parse(raw) : {};
}
export function identity() {
  const who = aws(["sts", "get-caller-identity"]);
  if (
    who.Account !== "747336059622" ||
    who.Arn.endsWith(":root") ||
    outputs.stage !== "dev-api" ||
    outputs.region !== "us-east-2"
  )
    throw Error("Intended non-root development account required.");
}
export function marker() {
  return aws([
    "dynamodb",
    "get-item",
    "--table-name",
    outputs.controlTable,
    "--key",
    JSON.stringify({ PK: { S: "STAGE#dev-api" }, SK: { S: "META" } }),
    "--consistent-read",
  ]).Item;
}
export function snapshot() {
  const functions = [];
  for (const [resource, name] of [
    ["UsersProjectsApi", outputs.apiFunction],
    ["EmptyProjectCleanup", outputs.cleanupFunction],
    ["CloudExecutionWorker", outputs.workerFunction],
    ["ExecutionControlledEndpoint", outputs.controlledEndpointFunction],
  ]) {
    if (!name?.startsWith("kurier-dev-api-")) throw Error("Unscoped function");
    const f = aws([
      "lambda",
      "get-function-configuration",
      "--function-name",
      name,
    ]);
    const role = f.Role.split("/").at(-1);
    const names = aws(["iam", "list-role-policies", "--role-name", role]);
    const attached = aws([
      "iam",
      "list-attached-role-policies",
      "--role-name",
      role,
    ]);
    if (names.IsTruncated || attached.IsTruncated)
      throw Error("Incomplete IAM inventory");
    const policies = names.PolicyNames.sort().map((p) =>
      aws(["iam", "get-role-policy", "--role-name", role, "--policy-name", p]),
    );
    const managed = attached.AttachedPolicies.sort((a, b) =>
      a.PolicyArn.localeCompare(b.PolicyArn),
    ).map((p) => {
      const meta = aws([
        "iam",
        "get-policy",
        "--policy-arn",
        p.PolicyArn,
      ]).Policy;
      return {
        arn: p.PolicyArn,
        version: meta.DefaultVersionId,
        document: aws([
          "iam",
          "get-policy-version",
          "--policy-arn",
          p.PolicyArn,
          "--version-id",
          meta.DefaultVersionId,
        ]).PolicyVersion.Document,
      };
    });
    const trust = aws(["iam", "get-role", "--role-name", role]).Role
      .AssumeRolePolicyDocument;
    const reserved =
      aws(["lambda", "get-function-concurrency", "--function-name", name])
        .ReservedConcurrentExecutions ?? null;
    const provisioned = aws([
      "lambda",
      "list-provisioned-concurrency-configs",
      "--function-name",
      name,
    ]);
    if (
      provisioned.NextMarker ||
      provisioned.ProvisionedConcurrencyConfigs.length ||
      reserved !== null
    )
      throw Error("Unexpected function concurrency");
    // Keep only approved public markers; never persist entire environments.
    const e = f.Environment?.Variables ?? {};
    functions.push({
      resource,
      name,
      role: f.Role,
      iamHash: hash({ policies, managed, trust }),
      codeSha256: f.CodeSha256,
      state: f.State,
      update: f.LastUpdateStatus,
      runtime: f.Runtime,
      architecture: f.Architectures,
      memory: f.MemorySize,
      timeout: f.Timeout,
      reserved,
      configurationHash: hash(
        Object.fromEntries(
          Object.entries(e).filter(
            ([k]) =>
              ![
                "KURIER_MCP_READ_ONLY_SCHEMA_VERSION",
                "KURIER_SAVED_REQUESTS_SCHEMA_VERSION",
              ].includes(k),
          ),
        ),
      ),
      mcp: e.KURIER_MCP_READ_ONLY_SCHEMA_VERSION ?? null,
      savedRequests: e.KURIER_SAVED_REQUESTS_SCHEMA_VERSION ?? null,
    });
  }
  const account = aws(["lambda", "get-account-settings"]).AccountLimit;
  const plan = aws(["freetier", "get-account-plan-state"]);
  const maps = aws([
    "lambda",
    "list-event-source-mappings",
    "--function-name",
    outputs.workerFunction,
  ]);
  if (maps.NextMarker || maps.EventSourceMappings.length !== 1)
    throw Error("Incomplete event mapping inventory");
  const m = maps.EventSourceMappings[0];
  const queue = {
    source: m.EventSourceArn,
    state: m.State,
    batch: m.BatchSize,
    max: m.ScalingConfig?.MaximumConcurrency,
    pollers: m.ProvisionedPollerConfig ?? null,
    responses: m.FunctionResponseTypes,
  };
  const rule = aws([
    "events",
    "describe-rule",
    "--name",
    outputs.cleanupSchedule,
  ]);
  const targets = aws([
    "events",
    "list-targets-by-rule",
    "--rule",
    outputs.cleanupSchedule,
  ]);
  if (
    targets.NextToken ||
    rule.State !== "ENABLED" ||
    rule.ScheduleExpression !== "rate(1 minute)" ||
    targets.Targets.length !== 1 ||
    !targets.Targets[0].Arn.endsWith(":function:" + outputs.cleanupFunction)
  )
    throw Error("Minute cleanup schedule mismatch");
  if (
    plan.accountPlanType !== "FREE" ||
    plan.accountPlanStatus !== "ACTIVE" ||
    account.ConcurrentExecutions !== 10 ||
    account.UnreservedConcurrentExecutions !== 10 ||
    queue.source !== outputs.executionQueueArn ||
    queue.state !== "Enabled" ||
    queue.batch !== 1 ||
    queue.max !== 2 ||
    queue.pollers
  )
    throw Error("Plan/quota/concurrency mismatch");
  return {
    account: "747336059622",
    region: "us-east-2",
    stage: "dev-api",
    functions,
    controls: {
      plan: { type: plan.accountPlanType, status: plan.accountPlanStatus },
      quota: 10,
      unreserved: 10,
      queue,
      schedule: {
        state: rule.State,
        expression: rule.ScheduleExpression,
        target: targets.Targets[0].Arn,
      },
    },
  };
}
export function capture(path) {
  identity();
  const v = snapshot();
  writeFileSync(path, JSON.stringify(v, null, 2) + "\n", {
    mode: 0o600,
    flag: "wx",
  });
  console.log(
    JSON.stringify({
      baseline: path,
      functions: v.functions.length,
      controls: v.controls,
    }),
  );
}
export function verify(path) {
  identity();
  const before = JSON.parse(readFileSync(path, "utf8")),
    current = snapshot();
  if (hash(before.controls) !== hash(current.controls))
    throw Error("Plan/quota/concurrency/schedule changed");
  for (const f of current.functions) {
    const old = before.functions.find((v) => v.name === f.name);
    const wanted = createHash("sha256")
      .update(
        readFileSync(
          new URL(`../.sst/artifacts/${f.resource}/code.zip`, import.meta.url),
        ),
      )
      .digest("base64");
    if (
      !old ||
      f.role !== old.role ||
      f.iamHash !== old.iamHash ||
      f.configurationHash !== old.configurationHash ||
      f.memory !== old.memory ||
      f.timeout !== old.timeout ||
      f.runtime !== "provided.al2023" ||
      f.architecture.join() !== "arm64" ||
      f.state !== "Active" ||
      f.update !== "Successful" ||
      f.codeSha256 !== wanted ||
      (f.name !== outputs.controlledEndpointFunction &&
        (f.mcp !== "1" ||
          (f.name !== outputs.workerFunction && f.savedRequests !== "7")))
    )
      throw Error("Deployed hash/configuration/IAM mismatch: " + f.resource);
  }
  const api = new URL(outputs.apiUrl).hostname.split(".")[0];
  const routes = aws(["apigatewayv2", "get-routes", "--api-id", api]),
    integrations = aws(["apigatewayv2", "get-integrations", "--api-id", api]);
  if (routes.NextToken || integrations.NextToken)
    throw Error("Incomplete routes");
  const policy = JSON.parse(
    aws(["lambda", "get-policy", "--function-name", outputs.apiFunction])
      .Policy,
  );
  // SST emits one invoke statement per route, restricted to this API's ARN.
  for (const resource of ["Bbtvrd", "Ruxrfx", "Bbvrka", "Ehmvvt"]) {
    if (
      !policy.Statement.some(
        (s) =>
          s.Sid.startsWith(
            "DevelopmentHttpApiRoute" + resource + "Permissions-",
          ) &&
          s.Effect === "Allow" &&
          s.Principal?.Service === "apigateway.amazonaws.com" &&
          s.Action === "lambda:InvokeFunction" &&
          s.Resource ===
            `arn:aws:lambda:us-east-2:747336059622:function:${outputs.apiFunction}` &&
          s.Condition?.ArnLike?.["AWS:SourceArn"] ===
            `arn:aws:execute-api:us-east-2:747336059622:${api}/*`,
      )
    )
      throw Error("Missing scoped MCP invocation permission: " + resource);
  }
  for (const key of [
    "POST /api/v1/mcp/tokens",
    "GET /api/v1/mcp/tokens/current",
    "DELETE /api/v1/mcp/tokens/{tokenId}",
    "POST /api/v1/mcp/invocations",
  ]) {
    const route = routes.Items.find((r) => r.RouteKey === key),
      i = integrations.Items.find(
        (i) => route?.Target === "integrations/" + i.IntegrationId,
      );
    if (
      !i?.IntegrationUri?.endsWith(":function:" + outputs.apiFunction) ||
      i.PayloadFormatVersion !== "2.0" ||
      route.AuthorizationType !== "NONE" ||
      !policy.Statement.some(
        (s) =>
          s.Principal?.Service === "apigateway.amazonaws.com" &&
          s.Action === "lambda:InvokeFunction" &&
          s.Condition?.ArnLike?.["AWS:SourceArn"]?.includes(":" + api + "/"),
      )
    )
      throw Error("Route/invoke permission mismatch: " + key);
  }
  console.log(
    JSON.stringify({
      verified: true,
      functions: current.functions.map((f) => ({
        resource: f.resource,
        codeSha256: f.codeSha256,
        iamHash: f.iamHash,
        archiveMatch: true,
        iamUnchanged: true,
      })),
      routes: 4,
      controls: current.controls,
    }),
  );
  return current;
}
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  const [mode, path, ...rest] = process.argv.slice(2);
  if (
    !path ||
    rest.length ||
    !["--capture-baseline", "--verify-baseline"].includes(mode)
  )
    throw Error(
      "Choose --capture-baseline PATH before deploy or --verify-baseline PATH after deploy.",
    );
  if (mode === "--capture-baseline") capture(path);
  else verify(path);
}
