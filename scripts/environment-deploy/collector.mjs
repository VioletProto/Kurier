// All network work occurs in AWS CLI children inside the metered net namespace.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import {
  hash,
  projectionFields,
  inspectRows,
  checkMarker,
} from "../lib/project-environments-activation.mjs";
export { hash };
import {
  frontendManifest,
  frontendMatches,
  frontendTarget,
} from "./frontend.mjs";
export const scope = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-execution-scope.json",
      import.meta.url,
    ),
  ),
);
export const release = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-adopted-release.json",
      import.meta.url,
    ),
  ),
);
const o = scope.outputs;
export function aws(service, operation, input = {}) {
  const program = String.raw`import json,os,subprocess,sys,re
v=json.load(sys.stdin)
f=os.memfd_create("kurier-private-input",os.MFD_CLOEXEC)
os.fchmod(f,0o600);os.write(f,json.dumps(v["input"]).encode());os.lseek(f,0,0)
r=subprocess.run(["aws",v["service"],v["operation"],"--region","us-east-2","--output","json","--no-paginate","--cli-input-json","file:///proc/self/fd/"+str(f)],capture_output=True,pass_fds=(f,))
os.ftruncate(f,0);os.close(f)
if r.returncode:
 code=re.search(rb"An error occurred \(([A-Za-z0-9]+)\)",r.stderr)
 if v["service"]=="lambda" and v["operation"]=="get-policy" and code and code.group(1)==b"ResourceNotFoundException":
  print(json.dumps({"Policy":json.dumps({"Version":"2012-10-17","Statement":[]})}));sys.exit(0)
 sys.exit(1)
sys.stdout.buffer.write(r.stdout)
`;
  const r = spawnSync("python3", ["-c", program], {
    input: JSON.stringify({ service, operation, input }),
    maxBuffer: 32 * 1024 * 1024,
    env: { ...process.env, AWS_MAX_ATTEMPTS: "1" },
  });
  if (r.status !== 0)
    throw Error("Metered AWS call failed/uncertain; no retry or repair.");
  return r.stdout.toString().trim() ? JSON.parse(r.stdout) : {};
}
export function uploadFrontend(f) {
  const file = new URL(
    "../../.build/project-environments-release/" +
      release.releaseSourceCommit +
      "/frontend/app/" +
      f.path,
    import.meta.url,
  ).pathname;
  const r = spawnSync(
    "aws",
    [
      "s3api",
      "put-object",
      "--region",
      "us-east-2",
      "--output",
      "json",
      "--bucket",
      scope.webBucket,
      "--key",
      f.path,
      "--body",
      file,
      "--content-type",
      f.path.endsWith(".css")
        ? "text/css;charset=UTF-8"
        : "text/javascript;charset=UTF-8",
      "--cache-control",
      f.cacheControl,
    ],
    { encoding: "utf8", env: { ...process.env, AWS_MAX_ATTEMPTS: "1" } },
  );
  if (r.status !== 0) throw Error("Reviewed asset upload failed/uncertain.");
}
export function verifyHistorical(p, m) {
  const i = JSON.parse(
    readFileSync(
      new URL(
        "../../docs/development/project-environments-inventory.json",
        import.meta.url,
      ),
    ),
  );
  if (m.recoveryGeneration.S !== i.stageAfter.recoveryGeneration.S)
    throw Error("Historical generation drift needs review.");
  for (const f of p.functions) {
    const old = i.functions.find((v) => v.logical === f.resource);
    const expectedIam = {
      policies: [...old.inlinePolicies].sort((a, b) =>
        a.PolicyName.localeCompare(b.PolicyName),
      ),
      managed: [...old.managedPolicies]
        .sort((a, b) => a.policy.Arn.localeCompare(b.policy.Arn))
        .map((v) => ({
          arn: v.policy.Arn,
          version: v.policy.DefaultVersionId,
          document: v.version.Document,
        })),
      trust: old.role.AssumeRolePolicyDocument,
    };
    const c = old.configuration;
    if (
      f.iamHash !== hash(expectedIam) ||
      f.configurationHash !== old.environmentSha256 ||
      f.codeSha256 !== c.CodeSha256 ||
      f.runtime !== c.Runtime ||
      hash(f.architecture) !== hash(c.Architectures) ||
      f.memory !== c.MemorySize ||
      f.timeout !== c.Timeout ||
      f.state !== "Active" ||
      f.update !== "Successful"
    )
      throw Error("Unreviewed initial deployed reader drift.");
  }
}
export function verifyInitialRouting(current) {
  const i = JSON.parse(
    readFileSync(
      new URL(
        "../../docs/development/project-environments-inventory.json",
        import.meta.url,
      ),
    ),
  );
  const sorted = (rows, k) =>
    [...rows].sort((a, b) => a[k].localeCompare(b[k]));
  if (
    hash(sorted(current.routes, "RouteId")) !==
      hash(sorted(i.gateway.routes, "RouteId")) ||
    hash(sorted(current.integrations, "IntegrationId")) !==
      hash(sorted(i.gateway.integrations, "IntegrationId"))
  )
    throw Error("Unreviewed existing gateway drift.");
  for (const f of i.functions) {
    const expected = f.invokePolicy?.Policy
      ? JSON.parse(f.invokePolicy.Policy)
      : { Version: "2012-10-17", Statement: [] };
    const actual = current.policies[f.logical];
    if (
      hash({ ...actual, Statement: sorted(actual.Statement, "Sid") }) !==
      hash({ ...expected, Statement: sorted(expected.Statement, "Sid") })
    )
      throw Error("Unreviewed existing invoke policy drift.");
  }
}
export function marker() {
  return aws("dynamodb", "get-item", {
    TableName: o.controlTable,
    Key: { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } },
    ConsistentRead: true,
  }).Item;
}
export function queueDepth() {
  return aws("sqs", "get-queue-attributes", {
    QueueUrl: o.executionQueueUrl,
    AttributeNames: [
      "ApproximateNumberOfMessages",
      "ApproximateNumberOfMessagesNotVisible",
      "ApproximateNumberOfMessagesDelayed",
    ],
  }).Attributes;
}
export function inventory() {
  const rows = [];
  const names = Object.fromEntries(
    projectionFields.map((x, i) => ["#p" + i, x]),
  );
  for (const table of [o.controlTable, o.protectedTable]) {
    let cursor;
    for (let page = 0; page < 1000; page++) {
      const r = aws("dynamodb", "scan", {
        TableName: table,
        ConsistentRead: true,
        Limit: 100,
        ProjectionExpression: Object.keys(names).join(","),
        ExpressionAttributeNames: names,
        ...(cursor ? { ExclusiveStartKey: cursor } : {}),
      });
      rows.push(...(r.Items ?? []).map((item) => ({ table, item })));
      cursor = r.LastEvaluatedKey;
      if (!cursor) break;
      if (page === 999) throw Error("Metadata pagination bound exhausted.");
    }
  }
  for (const r of rows.filter((x) => x.item.kind?.S === "executionResidual")) {
    const p = r.item.projectId?.S;
    if (!/^[a-f0-9-]{36}$/.test(p ?? ""))
      throw Error("Residual scope invalid.");
    const found = aws("s3api", "list-objects-v2", {
      Bucket: o.evidenceBucket,
      Prefix: "dev-api/projects/" + p + "/",
      MaxKeys: 1,
    });
    r.residualPrefixEmpty = !(found.Contents?.length || found.IsTruncated);
  }
  inspectRows(rows);
  return rows;
}
export function platform() {
  const functions = [];
  for (const [resource, id] of Object.entries(scope.functions)) {
    const f = aws("lambda", "get-function-configuration", {
      FunctionName: id.name,
    });
    if (f.FunctionArn !== id.arn || f.Role !== id.role)
      throw Error("Function identity drift.");
    const role = id.role.split("/").at(-1);
    const names = aws("iam", "list-role-policies", { RoleName: role });
    const attached = aws("iam", "list-attached-role-policies", {
      RoleName: role,
    });
    if (
      names.IsTruncated ||
      attached.IsTruncated ||
      hash([...names.PolicyNames].sort()) !==
        hash([...id.inlinePolicyNames].sort()) ||
      hash(attached.AttachedPolicies.map((x) => x.PolicyArn).sort()) !==
        hash([...id.managedPolicyArns].sort())
    )
      throw Error("IAM scope/list drift.");
    const policies = names.PolicyNames.sort().map((PolicyName) =>
      aws("iam", "get-role-policy", { RoleName: role, PolicyName }),
    );
    const managed = attached.AttachedPolicies.sort((a, b) =>
      a.PolicyArn.localeCompare(b.PolicyArn),
    ).map((x) => {
      const meta = aws("iam", "get-policy", { PolicyArn: x.PolicyArn }).Policy;
      return {
        arn: x.PolicyArn,
        version: meta.DefaultVersionId,
        document: aws("iam", "get-policy-version", {
          PolicyArn: x.PolicyArn,
          VersionId: meta.DefaultVersionId,
        }).PolicyVersion.Document,
      };
    });
    const trust = aws("iam", "get-role", { RoleName: role }).Role
      .AssumeRolePolicyDocument;
    const reserved =
      aws("lambda", "get-function-concurrency", { FunctionName: id.name })
        .ReservedConcurrentExecutions ?? null;
    const provisioned = aws("lambda", "list-provisioned-concurrency-configs", {
      FunctionName: id.name,
    });
    if (
      reserved !== null ||
      provisioned.NextMarker ||
      provisioned.ProvisionedConcurrencyConfigs?.length
    )
      throw Error("Concurrency drift.");
    const variables = f.Environment?.Variables;
    if (!variables || f.Environment.Error)
      throw Error("Configuration unavailable.");
    const configurationHash = hash(variables);
    Object.keys(variables).forEach((k) => delete variables[k]);
    functions.push({
      resource,
      name: id.name,
      role: id.role,
      iamHash: hash({ policies, managed, trust }),
      configurationHash,
      codeSha256: f.CodeSha256,
      state: f.State,
      update: f.LastUpdateStatus,
      runtime: f.Runtime,
      architecture: f.Architectures,
      memory: f.MemorySize,
      timeout: f.Timeout,
      reserved,
    });
  }
  const account = aws("lambda", "get-account-settings").AccountLimit;
  const plan = aws("freetier", "get-account-plan-state");
  const maps = aws("lambda", "list-event-source-mappings", {
    FunctionName: o.workerFunction,
  });
  if (maps.NextMarker || maps.EventSourceMappings?.length !== 1)
    throw Error("Mapping inventory incomplete.");
  const m = maps.EventSourceMappings[0];
  const queue = {
    source: m.EventSourceArn,
    state: m.State,
    batch: m.BatchSize,
    max: m.ScalingConfig?.MaximumConcurrency,
    pollers: m.ProvisionedPollerConfig ?? null,
    responses: m.FunctionResponseTypes,
  };
  const rule = aws("events", "describe-rule", { Name: o.cleanupSchedule });
  const targets = aws("events", "list-targets-by-rule", {
    Rule: o.cleanupSchedule,
  });
  const stage = aws("apigatewayv2", "get-stage", {
    ApiId: scope.apiId,
    StageName: "$default",
  });
  if (
    plan.accountPlanType !== "FREE" ||
    plan.accountPlanStatus !== "ACTIVE" ||
    account.ConcurrentExecutions !== 10 ||
    account.UnreservedConcurrentExecutions !== 10 ||
    queue.source !== o.executionQueueArn ||
    queue.state !== "Enabled" ||
    queue.batch !== 1 ||
    queue.max !== 2 ||
    queue.pollers ||
    rule.State !== "ENABLED" ||
    rule.ScheduleExpression !== "rate(1 minute)" ||
    targets.NextToken ||
    targets.Targets?.length !== 1 ||
    !targets.Targets[0].Arn.endsWith(":function:" + o.cleanupFunction) ||
    stage.DefaultRouteSettings?.ThrottlingRateLimit !== 10 ||
    stage.DefaultRouteSettings?.ThrottlingBurstLimit !== 20
  )
    throw Error("Plan/quota/schedule/API controls drift.");
  return {
    account: scope.account,
    region: scope.region,
    stage: scope.stage,
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
export function frontendReady(phase) {
  const expected = frontendManifest(phase);
  // Body bytes arrive via a RAM descriptor; no private env/config response files.
  const python = `import base64,boto3,json,sys
from botocore.config import Config
v=json.load(sys.stdin)
c=boto3.client("s3",region_name="us-east-2",config=Config(retries={"total_max_attempts":1}))
r=c.get_object(Bucket=v["bucket"],Key=v["key"])
b=r["Body"].read(2097153)
if len(b)>2097152:sys.exit(1)
print(base64.b64encode(b).decode())`;
  return frontendMatches(expected, {
    readObject(key) {
      const r = spawnSync("python3", ["-c", python], {
        input: JSON.stringify({ bucket: scope.webBucket, key }),
        maxBuffer: 4 * 1024 * 1024,
      });
      return r.status === 0
        ? Buffer.from(r.stdout.toString().trim(), "base64")
        : null;
    },
    keyValue(path) {
      return aws("cloudfront-keyvaluestore", "get-key", {
        KvsARN: scope.kvsArn,
        Key: scope.namespace + ":/" + path,
      }).Value;
    },
    distributionDeployed() {
      return (
        aws("cloudfront", "get-distribution", { Id: scope.distribution })
          .Distribution.Status === "Deployed"
      );
    },
  });
}
export function collect(phase = "collect") {
  const identity = aws("sts", "get-caller-identity");
  if (identity.Account !== scope.account || identity.Arn !== scope.identityArn)
    throw Error("Inventoried identity required.");
  const before = marker();
  checkMarker(before);
  const p = platform();
  const rows = inventory();
  const depth = queueDepth();
  if (
    Object.values(depth).some((v) => v !== "0") ||
    hash(marker()) !== hash(before)
  )
    throw Error("Quiescence/marker drift.");
  return {
    identity,
    marker: before,
    platform: p,
    rows,
    depth,
    frontendReady: frontendReady(phase),
    frontendExpectation: frontendTarget(phase),
    frontendExpectedReleaseId: frontendManifest(phase).releaseId,
    routing: routing(),
  };
}
export function routing() {
  const page = (op) => {
    const all = [];
    let token;
    for (let i = 0; i < 100; i++) {
      const r = aws("apigatewayv2", op, {
        ApiId: scope.apiId,
        ...(token ? { NextToken: token } : {}),
      });
      all.push(...(r.Items ?? []));
      token = r.NextToken;
      if (!token) return all;
    }
    throw Error("Route pagination exhausted.");
  };
  const policies = {};
  for (const [logical, f] of Object.entries(scope.functions)) {
    const r = aws("lambda", "get-policy", { FunctionName: f.name });
    policies[logical] = JSON.parse(r.Policy);
  }
  return {
    routes: page("get-routes"),
    integrations: page("get-integrations"),
    policies,
  };
}
export function verifyRouting(current, baseline) {
  const idMap = (rows, key) => new Map(rows.map((v) => [v[key], v]));
  const oldRoutes = idMap(baseline.routes, "RouteId"),
    routes = idMap(current.routes, "RouteId");
  for (const [id, row] of oldRoutes)
    if (hash(routes.get(id)) !== hash(row))
      throw Error("Existing route drift.");
  const added = current.routes.filter((r) => !oldRoutes.has(r.RouteId));
  if (
    hash(added.map((r) => r.RouteKey).sort()) !==
      hash([...scope.routes].sort()) ||
    added.some((r) => r.AuthorizationType !== "NONE")
  )
    throw Error("New route scope drift.");
  const oldIntegrations = idMap(baseline.integrations, "IntegrationId"),
    integrations = idMap(current.integrations, "IntegrationId");
  for (const [id, row] of oldIntegrations)
    if (hash(integrations.get(id)) !== hash(row))
      throw Error("Existing integration drift.");
  const newIds = current.integrations.filter(
    (r) => !oldIntegrations.has(r.IntegrationId),
  );
  if (
    newIds.length !== 8 ||
    newIds.some(
      (r) =>
        r.IntegrationUri !== scope.functions.UsersProjectsApi.arn ||
        r.IntegrationType !== "AWS_PROXY" ||
        r.PayloadFormatVersion !== "2.0",
    ) ||
    added.some(
      (r) =>
        !newIds.some((i) => r.Target === "integrations/" + i.IntegrationId),
    )
  )
    throw Error("Integration scope drift.");
  for (const [logical, policy] of Object.entries(current.policies)) {
    const before = new Map(
      baseline.policies[logical].Statement.map((v) => [v.Sid, v]),
    );
    for (const [sid, row] of before)
      if (hash(policy.Statement.find((v) => v.Sid === sid)) !== hash(row))
        throw Error("Existing invoke permission drift.");
    const extra = policy.Statement.filter((v) => !before.has(v.Sid));
    if (logical !== "UsersProjectsApi" && extra.length)
      throw Error("Unreviewed function permission.");
    if (logical === "UsersProjectsApi") {
      if (extra.length !== 8) throw Error("Invoke permission count mismatch.");
      for (const p of extra) {
        const wanted = scope.permissions.find((v) => v.statementId === p.Sid);
        if (
          !wanted ||
          p.Action !== "lambda:InvokeFunction" ||
          p.Effect !== "Allow" ||
          hash(p.Principal) !== hash({ Service: "apigateway.amazonaws.com" }) ||
          p.Resource !== scope.functions.UsersProjectsApi.arn ||
          hash(p.Condition) !==
            hash({ ArnLike: { "AWS:SourceArn": wanted.sourceArn } })
        )
          throw Error("Invoke permission shape mismatch.");
      }
    }
  }
  return true;
}
