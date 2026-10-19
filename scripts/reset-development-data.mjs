// Explicitly destructive development-only test reset. Never deletes infrastructure.
// Read request/secret data only as keys; keep Cognito usernames in RAM, never logs.
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import {
  aws,
  identity,
  outputs as o,
  marker,
  hash,
} from "./verify-read-only-mcp-deployment.mjs";
const mode = process.argv[2];
if (process.argv.length !== 3 || !["--clear", "--finish"].includes(mode))
  throw Error("Explicit authorized --clear or inspected --finish required.");
identity();
if (
  o.controlTable !== "kurier-dev-api-ControlTable-bdxbaoxb" ||
  o.protectedTable !== "kurier-dev-api-ProtectedTable-szbvcchc" ||
  o.cognitoUserPoolId !== "us-east-2_qTDZQT1FE" ||
  o.evidenceBucket !== "executionevidence-71a0f7a"
)
  throw Error("Unreviewed resource scope.");
const receiptPath = ".sst/development-data-reset.json";
function save(receipt) {
  writeFileSync(receiptPath, JSON.stringify(receipt, null, 2) + "\n", {
    mode: 0o600,
  });
}
function scan(table) {
  const rows = [];
  let cursor;
  do {
    const r = aws(["dynamodb", "scan"], {
      TableName: table,
      ConsistentRead: true,
      ProjectionExpression: "PK,SK,#k,#s",
      ExpressionAttributeNames: { "#k": "kind", "#s": "state" },
      ...(cursor ? { ExclusiveStartKey: cursor } : {}),
    });
    rows.push(...(r.Items ?? []));
    cursor = r.LastEvaluatedKey;
    if (rows.length > 100000) throw Error("Reset inventory exceeds bounds.");
  } while (cursor);
  return rows;
}
function users() {
  const rows = [];
  let cursor;
  do {
    const r = aws(["cognito-idp", "list-users"], {
      UserPoolId: o.cognitoUserPoolId,
      ...(cursor ? { PaginationToken: cursor } : {}),
    });
    rows.push(...(r.Users ?? []));
    cursor = r.PaginationToken;
    if (rows.length > 1000) throw Error("User inventory exceeds reset scope.");
  } while (cursor);
  return rows;
}
function evidence() {
  const all = [];
  let cursor;
  do {
    const r = aws(["s3api", "list-objects-v2"], {
      Bucket: o.evidenceBucket,
      ...(cursor ? { ContinuationToken: cursor } : {}),
    });
    all.push(...(r.Contents ?? []));
    cursor = r.NextContinuationToken;
    if (r.IsTruncated && !cursor) throw Error("Incomplete evidence inventory.");
  } while (cursor);
  return all;
}
function switchStage(from, to) {
  const next = Number(from.version.N) + 1;
  const r = aws(["dynamodb", "update-item"], {
    TableName: o.controlTable,
    Key: { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } },
    UpdateExpression: "SET #s = :to, #v = :next",
    ConditionExpression:
      "#s = :from AND #v = :old AND recoveryGeneration = :generation",
    ExpressionAttributeNames: { "#s": "state", "#v": "version" },
    ExpressionAttributeValues: {
      ":to": { S: to },
      ":next": { N: String(next) },
      ":from": from.state,
      ":old": from.version,
      ":generation": from.recoveryGeneration,
    },
    ReturnValues: "ALL_NEW",
  }).Attributes;
  const expected = { ...from, state: { S: to }, version: { N: String(next) } };
  if (hash(r) !== hash(expected))
    throw Error("Stage transition readback differs.");
  return r;
}
function config() {
  const fns = [
    o.apiFunction,
    o.cleanupFunction,
    o.workerFunction,
    o.controlledEndpointFunction,
  ].map((name) => {
    const c = aws([
      "lambda",
      "get-function-configuration",
      "--function-name",
      name,
    ]);
    return {
      name,
      hash: hash(c),
      reserved:
        aws(["lambda", "get-function-concurrency", "--function-name", name])
          .ReservedConcurrentExecutions ?? null,
      provisioned:
        aws([
          "lambda",
          "list-provisioned-concurrency-configs",
          "--function-name",
          name,
        ]).ProvisionedConcurrencyConfigs ?? [],
      timeout: c.Timeout,
    };
  });
  const client = aws([
    "cognito-idp",
    "describe-user-pool-client",
    "--user-pool-id",
    o.cognitoUserPoolId,
    "--client-id",
    o.cognitoClientId,
  ]).UserPoolClient;
  if (
    client.ClientSecret ||
    client.AccessTokenValidity !== 15 ||
    client.TokenValidityUnits?.AccessToken !== "minutes"
  )
    throw Error("Unexpected JWT expiry configuration.");
  const plan = aws(["freetier", "get-account-plan-state"]);
  const account = aws(["lambda", "get-account-settings"]).AccountLimit;
  if (
    plan.accountPlanType !== "FREE" ||
    plan.accountPlanStatus !== "ACTIVE" ||
    account.ConcurrentExecutions !== 10 ||
    account.UnreservedConcurrentExecutions !== 10
  )
    throw Error("Unexpected plan/concurrency.");
  const mappings = aws([
    "lambda",
    "list-event-source-mappings",
    "--function-name",
    o.workerFunction,
  ]);
  const schedule = aws([
    "events",
    "describe-rule",
    "--name",
    o.cleanupSchedule,
  ]);
  const web = aws([
    "s3api",
    "list-objects-v2",
    "--bucket",
    o.frontendHosting.webBucket,
    "--no-paginate",
  ]);
  if (web.IsTruncated) throw Error("Incomplete frontend inventory.");
  return {
    functions: fns,
    cognitoClientHash: hash(client),
    plan: { type: plan.accountPlanType, status: plan.accountPlanStatus },
    account,
    workerHash: hash(mappings),
    scheduleHash: hash(schedule),
    bootstrapHash: hash(
      aws(["ssm", "get-parameter", "--name", "/sst/bootstrap"]).Parameter,
    ),
    webHash: hash(web.Contents ?? []),
    distributionHash: hash(
      aws([
        "cloudfront",
        "get-distribution",
        "--id",
        o.frontendHosting.distributionId,
      ]).Distribution.DistributionConfig,
    ),
  };
}
async function deleteRows(table, rows) {
  for (let i = 0; i < rows.length; i += 25) {
    let pending = rows
      .slice(i, i + 25)
      .map((r) => ({ DeleteRequest: { Key: { PK: r.PK, SK: r.SK } } }));
    for (let attempt = 0; pending.length; attempt++) {
      if (attempt >= 8)
        throw Error("Unprocessed deletes remain; stage stays closed.");
      const r = aws(["dynamodb", "batch-write-item"], {
        RequestItems: { [table]: pending },
      });
      pending = r.UnprocessedItems?.[table] ?? [];
      if (pending.length)
        await new Promise((resolve) =>
          setTimeout(resolve, Math.min(5000, 100 * 2 ** attempt)),
        );
    }
  }
}
function clearEvidence() {
  const versions = aws([
    "s3api",
    "get-bucket-versioning",
    "--bucket",
    o.evidenceBucket,
  ]);
  if (versions.Status)
    throw Error(
      "Versioned evidence requires explicit version inventory; stage stays closed.",
    );
  const uploads = aws([
    "s3api",
    "list-multipart-uploads",
    "--bucket",
    o.evidenceBucket,
  ]);
  if (uploads.IsTruncated) throw Error("Incomplete multipart inventory.");
  for (const u of uploads.Uploads ?? [])
    aws(["s3api", "abort-multipart-upload"], {
      Bucket: o.evidenceBucket,
      Key: u.Key,
      UploadId: u.UploadId,
    });
  const objects = evidence();
  for (let i = 0; i < objects.length; i += 1000) {
    const r = aws(["s3api", "delete-objects"], {
      Bucket: o.evidenceBucket,
      Delete: {
        Objects: objects.slice(i, i + 1000).map((x) => ({ Key: x.Key })),
        Quiet: true,
      },
    });
    if (r.Errors?.length)
      throw Error("Evidence deletion failed; stage stays closed.");
  }
  return objects.length;
}
function clearLogs() {
  let deleted = 0;
  for (const name of [
    o.apiFunction,
    o.cleanupFunction,
    o.workerFunction,
    o.controlledEndpointFunction,
  ]) {
    const group = aws([
      "lambda",
      "get-function-configuration",
      "--function-name",
      name,
    ]).LoggingConfig?.LogGroup;
    if (!group || !group.startsWith("/aws/lambda/kurier-dev-api-"))
      throw Error("Unreviewed application log group.");
    const streams = [];
    let cursor;
    do {
      const r = aws(["logs", "describe-log-streams"], {
        logGroupName: group,
        ...(cursor ? { nextToken: cursor } : {}),
      });
      streams.push(...(r.logStreams ?? []));
      cursor = r.nextToken;
    } while (cursor);
    for (const stream of streams) {
      aws([
        "logs",
        "delete-log-stream",
        "--log-group-name",
        group,
        "--log-stream-name",
        stream.logStreamName,
      ]);
      deleted++;
    }
  }
  return deleted;
}
if (mode === "--clear") {
  if (existsSync(receiptPath))
    throw Error("Existing reset receipt: inspect before any further mutation.");
  const stage = marker();
  if (
    stage.state.S !== "active" ||
    stage.version.N !== "11" ||
    stage.savedRequestsSchemaVersion.N !== "7" ||
    stage.mcpReadOnlySchemaVersion.N !== "1"
  )
    throw Error("Unreviewed durable stage baseline.");
  const beforeConfig = config();
  const accountUsers = users();
  const control = scan(o.controlTable);
  const protectedRows = scan(o.protectedTable);
  if (
    control.some(
      (r) =>
        ["executionJob", "executionWorkflowRun"].includes(r.kind?.S) &&
        !["completed", "failed", "deleted"].includes(r.state?.S),
    )
  )
    throw Error("Unsettled work requires inspection before clearing.");
  const receipt = {
    startedAt: new Date().toISOString(),
    scope: {
      account: "747336059622",
      stage: "dev-api",
      region: "us-east-2",
      cognitoPool: o.cognitoUserPoolId,
      controlTable: o.controlTable,
      protectedTable: o.protectedTable,
      evidenceBucket: o.evidenceBucket,
    },
    originalStage: stage,
    beforeConfig,
    before: {
      cognitoUsers: accountUsers.length,
      controlRows: control.length,
      protectedRows: protectedRows.length,
    },
    phase: "prepared",
  };
  save(receipt);
  receipt.closedStage = switchStage(stage, "closed");
  receipt.phase = "closed";
  save(receipt);
  for (const user of accountUsers) {
    aws(["cognito-idp", "admin-delete-user"], {
      UserPoolId: o.cognitoUserPoolId,
      Username: user.Username,
    });
  }
  receipt.usersDeletedAt = new Date().toISOString();
  receipt.notBeforeReopen = new Date(Date.now() + 965000).toISOString();
  receipt.phase = "identities-cleared";
  save(receipt);
  console.log(
    JSON.stringify({
      phase: receipt.phase,
      deletedUsers: accountUsers.length,
      stageClosed: true,
      notBeforeReopen: receipt.notBeforeReopen,
    }),
  );
  // Drain admitted Lambdas before clearing stored bytes; stage CAS prevents new work.
  const drainSeconds =
    Math.max(...beforeConfig.functions.map((f) => f.timeout)) + 5;
  const drainUntil = Date.now() + drainSeconds * 1000;
  while (Date.now() < drainUntil)
    await new Promise((resolve) =>
      setTimeout(resolve, Math.min(10000, drainUntil - Date.now())),
    );
  for (const url of [o.executionQueueUrl, o.executionDlqUrl])
    aws(["sqs", "purge-queue", "--queue-url", url]);
  receipt.queuePurgedAt = new Date().toISOString();
  save(receipt);
  receipt.deletedEvidence = clearEvidence();
  await deleteRows(o.protectedTable, scan(o.protectedTable));
  await deleteRows(
    o.controlTable,
    scan(o.controlTable).filter(
      (r) => !(r.PK?.S === "STAGE#dev-api" && r.SK?.S === "META"),
    ),
  );
  receipt.deletedLogStreams = clearLogs();
  receipt.phase = "cleared-awaiting-token-expiry";
  receipt.clearedAt = new Date().toISOString();
  save(receipt);
  console.log(
    JSON.stringify({
      phase: receipt.phase,
      deletedEvidence: receipt.deletedEvidence,
      deletedLogStreams: receipt.deletedLogStreams,
      notBeforeReopen: receipt.notBeforeReopen,
    }),
  );
} else {
  const receipt = JSON.parse(readFileSync(receiptPath, "utf8"));
  if (
    receipt.phase !== "cleared-awaiting-token-expiry" ||
    Date.now() < Date.parse(receipt.notBeforeReopen) ||
    Date.now() < Date.parse(receipt.queuePurgedAt) + 60000
  )
    throw Error("Reset not ready for inspected reopen.");
  const current = marker();
  if (hash(current) !== hash(receipt.closedStage))
    throw Error("Closed-stage baseline changed; refusing reopen.");
  if (users().length || evidence().length || scan(o.protectedTable).length)
    throw Error("Data reappeared; stage remains closed.");
  const control = scan(o.controlTable);
  if (
    control.length !== 1 ||
    control[0].PK.S !== "STAGE#dev-api" ||
    control[0].SK.S !== "META"
  )
    throw Error("Unexpected Control rows; stage remains closed.");
  for (const url of [o.executionQueueUrl, o.executionDlqUrl]) {
    const q = aws([
      "sqs",
      "get-queue-attributes",
      "--queue-url",
      url,
      "--attribute-names",
      "ApproximateNumberOfMessages",
      "ApproximateNumberOfMessagesNotVisible",
      "ApproximateNumberOfMessagesDelayed",
    ]).Attributes;
    if (Object.values(q).some((v) => v !== "0"))
      throw Error("Queue not empty; stage remains closed.");
  }
  const afterConfig = config();
  if (hash(afterConfig) !== hash(receipt.beforeConfig))
    throw Error("Resource settings changed; stage remains closed.");
  receipt.activeStage = switchStage(current, "active");
  receipt.phase = "complete";
  receipt.completedAt = new Date().toISOString();
  receipt.after = {
    cognitoUsers: 0,
    controlRows: 1,
    protectedRows: 0,
    evidenceObjects: 0,
    queueMessages: 0,
    configurationPreserved: true,
  };
  save(receipt);
  console.log(
    JSON.stringify({
      phase: receipt.phase,
      after: receipt.after,
      stageVersion: receipt.activeStage.version.N,
      generation: receipt.activeStage.recoveryGeneration.S,
    }),
  );
}
