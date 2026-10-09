// Scoped users/projects slice; existing Cognito is read, never managed here.
export async function createDevelopmentApi() {
  const account = process.env.KURIER_AWS_ACCOUNT_ID;
  const poolId = process.env.KURIER_COGNITO_POOL_ID;
  const clientId = process.env.KURIER_COGNITO_CLIENT_ID;
  const origin = process.env.KURIER_FRONTEND_ORIGIN ?? "http://localhost:5173";
  const parameterName = "/kurier/dev-api/cursor-key";
  if (
    !/^\d{12}$/.test(account ?? "") ||
    !/^us-east-2_[A-Za-z0-9]+$/.test(poolId ?? "") ||
    !/^[a-z0-9]+$/.test(clientId ?? "")
  ) {
    throw new Error(
      "Supply the intended AWS account and existing development Cognito pool/client.",
    );
  }
  const frontend = new URL(origin);
  if (
    frontend.protocol !== "http:" ||
    !["localhost", "127.0.0.1", "[::1]"].includes(frontend.hostname) ||
    !frontend.port ||
    frontend.origin !== origin
  ) {
    throw new Error("Supply an explicit local frontend origin with port.");
  }
  const identity = await aws.getCallerIdentity({});
  if (identity.accountId !== account || identity.arn.endsWith(":root")) {
    throw new Error(
      "Development API requires the intended account and a non-root identity.",
    );
  }
  const pool = await aws.cognito.getUserPool({ userPoolId: poolId! });
  const client = await aws.cognito.getUserPoolClient({
    userPoolId: poolId!,
    clientId: clientId!,
  });
  if (
    pool.id !== poolId ||
    !!client.clientSecret ||
    !client.explicitAuthFlows.includes("ALLOW_USER_SRP_AUTH")
  ) {
    throw new Error(
      "Expected existing Essentials public SRP development client.",
    );
  }
  const control = new sst.aws.Dynamo("Control", {
    fields: {
      PK: "string",
      SK: "string",
      LPK: "string",
      LSK: "string",
      DPK: "string",
      DSK: "string",
      HPK: "string",
      HSK: "string",
    },
    primaryIndex: { hashKey: "PK", rangeKey: "SK" },
    globalIndexes: {
      GSI1: { hashKey: "LPK", rangeKey: "LSK", projection: "keys-only" },
      GSI2: { hashKey: "DPK", rangeKey: "DSK", projection: "keys-only" },
      GSI3: { hashKey: "HPK", rangeKey: "HSK", projection: "keys-only" },
    },
    transform: {
      table(args) {
        args.billingMode = "PAY_PER_REQUEST";
        args.pointInTimeRecovery = { enabled: true };
        args.deletionProtectionEnabled = true;
      },
    },
  });
  const protectedTable = new sst.aws.Dynamo("Protected", {
    fields: { PK: "string", SK: "string" },
    primaryIndex: { hashKey: "PK", rangeKey: "SK" },
    transform: {
      table(args) {
        args.billingMode = "PAY_PER_REQUEST";
        args.pointInTimeRecovery = { enabled: true };
        args.deletionProtectionEnabled = true;
      },
    },
  });
  // Inventory confirmed no existing customer stage key; retained annually rotated key.
  const stageKey = new aws.kms.Key(
    "ProtectedStageKey",
    {
      description: "Kurier dev-api saved-secret envelope key",
      enableKeyRotation: true,
      deletionWindowInDays: 30,
      tags: { "kurier:stage": "dev-api", "kurier:purpose": "saved-secret" },
    },
    { retainOnDelete: true, protect: true },
  );
  new aws.kms.Alias("ProtectedStageKeyAlias", {
    name: "alias/kurier/dev-api/protected",
    targetKeyId: stageKey.id,
  });
  // Owned initial marker; retained with the table. Never reset generation during
  // runtime startup. Recovery must update state/generation before reopening.
  new aws.dynamodb.TableItem(
    "DevelopmentStage",
    {
      tableName: control.name,
      hashKey: "PK",
      rangeKey: "SK",
      item: JSON.stringify({
        PK: { S: "STAGE#dev-api" },
        SK: { S: "META" },
        kind: { S: "stage" },
        schemaVersion: { N: "1" },
        version: { N: "0" },
        state: { S: "active" },
        recoveryGeneration: { S: "dev-api-initial-v1" },
        savedRequestsSchemaVersion: { N: "2" },
        protectedSecretsSchemaVersion: { N: "1" },
      }),
    },
    { ignoreChanges: ["item"] },
  );
  const tablePermissions = [
    {
      actions: [
        "dynamodb:GetItem",
        "dynamodb:Query",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:ConditionCheckItem",
      ],
      resources: [control.arn, $interpolate`${control.arn}/index/*`],
    },
  ];
  const protectedPermissions = [
    {
      actions: [
        "dynamodb:GetItem",
        "dynamodb:Query",
        "dynamodb:PutItem",
        "dynamodb:DeleteItem",
        "dynamodb:ConditionCheckItem",
      ],
      resources: [protectedTable.arn],
    },
  ];
  const evidence = new aws.s3.BucketV2("ExecutionEvidence", {
    forceDestroy: false,
  });
  new aws.s3.BucketPublicAccessBlock("ExecutionEvidencePublicAccess", {
    bucket: evidence.id,
    blockPublicAcls: true,
    blockPublicPolicy: true,
    ignorePublicAcls: true,
    restrictPublicBuckets: true,
  });
  new aws.s3.BucketServerSideEncryptionConfigurationV2(
    "ExecutionEvidenceEncryption",
    {
      bucket: evidence.id,
      rules: [
        { applyServerSideEncryptionByDefault: { sseAlgorithm: "AES256" } },
      ],
    },
  );
  new aws.s3.BucketOwnershipControls("ExecutionEvidenceOwnership", {
    bucket: evidence.id,
    rule: { objectOwnership: "BucketOwnerEnforced" },
  });
  new aws.s3.BucketPolicy("ExecutionEvidenceTls", {
    bucket: evidence.id,
    policy: $jsonStringify({
      Version: "2012-10-17",
      Statement: [
        {
          Effect: "Deny",
          Principal: "*",
          Action: "s3:*",
          Resource: [evidence.arn, $interpolate`${evidence.arn}/*`],
          Condition: { Bool: { "aws:SecureTransport": "false" } },
        },
      ],
    }),
  });
  const dlq = new aws.sqs.Queue("ExecutionDeadLetter", {
    messageRetentionSeconds: 1209600,
    sqsManagedSseEnabled: true,
  });
  const queue = new aws.sqs.Queue("ExecutionQueue", {
    visibilityTimeoutSeconds: 540,
    messageRetentionSeconds: 345600,
    receiveWaitTimeSeconds: 20,
    sqsManagedSseEnabled: true,
    redrivePolicy: $jsonStringify({
      deadLetterTargetArn: dlq.arn,
      maxReceiveCount: 5,
    }),
  });
  new aws.sqs.RedriveAllowPolicy("ExecutionDeadLetterAllow", {
    queueUrl: dlq.url,
    redriveAllowPolicy: $jsonStringify({
      redrivePermission: "byQueue",
      sourceQueueArns: [queue.arn],
    }),
  });
  const queueSend = { actions: ["sqs:SendMessage"], resources: [queue.arn] };
  const objectArn = $interpolate`${evidence.arn}/dev-api/projects/*`;
  const purposeConditions = (purpose: string) => [
    {
      test: "StringEquals",
      variable: "kms:EncryptionContext:app",
      values: ["kurier"],
    },
    {
      test: "StringEquals",
      variable: "kms:EncryptionContext:stage",
      values: ["dev-api"],
    },
    {
      test: "StringEquals",
      variable: "kms:EncryptionContext:purpose",
      values: [purpose],
    },
  ];
  const executionEnvironment = {
    KURIER_CLOUD_EXECUTIONS_SCHEMA_VERSION: "1",
    KURIER_EXECUTION_INPUTS_SCHEMA_VERSION: "1",
    KURIER_EVIDENCE_BUCKET: evidence.bucket,
    KURIER_EXECUTION_QUEUE_URL: queue.url,
  };
  const environment = {
    ...executionEnvironment,
    KURIER_STAGE: "dev-api",
    KURIER_SAVED_REQUESTS_SCHEMA_VERSION: "3",
    KURIER_PROTECTED_SECRETS_SCHEMA_VERSION: "1",
    KURIER_PROTECTED_TABLE: protectedTable.name,
    KURIER_CONTROL_TABLE: control.name,
    KURIER_KMS_KEY_ARN: stageKey.arn,
    KURIER_COGNITO_ISSUER: `https://cognito-idp.us-east-2.amazonaws.com/${poolId}`,
    KURIER_COGNITO_CLIENT_ID: clientId!,
    KURIER_FRONTEND_ORIGIN: origin,
    KURIER_CURSOR_KEY_PARAMETER: parameterName,
  };
  const fn = new sst.aws.Function("UsersProjectsApi", {
    runtime: "go",
    handler: "services/api",
    architecture: "arm64",
    memory: "256 MB",
    timeout: "15 seconds",
    dev: false,
    environment,
    logging: { retention: "1 week" },
    permissions: [
      ...tablePermissions,
      {
        actions: ["ssm:GetParameter"],
        resources: [
          `arn:aws:ssm:us-east-2:${account}:parameter${parameterName}`,
        ],
      },
      ...protectedPermissions,
      queueSend,
      { actions: ["s3:GetObject"], resources: [objectArn] },
      {
        actions: ["kms:Decrypt"],
        resources: [stageKey.arn],
        conditions: purposeConditions("saved-secret"),
      },
      {
        actions: ["kms:GenerateDataKey"],
        resources: [stageKey.arn],
        conditions: purposeConditions("job-bindings"),
      },
      {
        actions: ["kms:GenerateDataKey"],
        resources: [stageKey.arn],
        conditions: [
          {
            test: "StringEquals",
            variable: "kms:EncryptionContext:app",
            values: ["kurier"],
          },
          {
            test: "StringEquals",
            variable: "kms:EncryptionContext:stage",
            values: ["dev-api"],
          },
          {
            test: "StringEquals",
            variable: "kms:EncryptionContext:purpose",
            values: ["saved-secret"],
          },
        ],
      },
    ],
  });
  const api = new sst.aws.ApiGatewayV2("DevelopmentHttpApi", {
    cors: false,
    accessLog: { retention: "1 week" },
    transform: {
      api(args) {
        // SST cors:false normalizes to {}; AWS treats that as configured CORS
        // and strips Lambda CORS headers. Omit the configuration entirely.
        args.corsConfiguration = undefined;
      },
      stage(args) {
        args.defaultRouteSettings = {
          throttlingBurstLimit: 20,
          throttlingRateLimit: 10,
        };
        // Static route templates and status only; no paths, headers, query strings,
        // IP addresses, user agents, bodies or integration error text.
        args.accessLogSettings = {
          ...args.accessLogSettings,
          format: JSON.stringify({
            requestId: "$context.requestId",
            route: "$context.routeKey",
            status: "$context.status",
            latency: "$context.responseLatency",
          }),
        };
      },
    },
  });
  for (const route of [
    "GET /api/v1/users/me",
    "GET /api/v1/projects",
    "POST /api/v1/projects",
    "GET /api/v1/projects/{projectId}",
    "PATCH /api/v1/projects/{projectId}",
    "DELETE /api/v1/projects/{projectId}",
    "GET /api/v1/projects/{projectId}/deletion-operations/{operationId}",
    "POST /api/v1/projects/{projectId}/requests",
    "GET /api/v1/projects/{projectId}/requests",
    "GET /api/v1/projects/{projectId}/requests/{requestId}",
    "PATCH /api/v1/projects/{projectId}/requests/{requestId}",
    "DELETE /api/v1/projects/{projectId}/requests/{requestId}",
    "GET /api/v1/projects/{projectId}/secrets",
    "PATCH /api/v1/projects/{projectId}/secrets/{secretId}",
    "DELETE /api/v1/projects/{projectId}/secrets/{secretId}",
    "POST /api/v1/projects/{projectId}/requests/{requestId}/executions",
    "GET /api/v1/projects/{projectId}/executions",
    "GET /api/v1/projects/{projectId}/executions/{executionId}",
    "GET /api/v1/projects/{projectId}/executions/{executionId}/status",
    "GET /api/v1/projects/{projectId}/executions/{executionId}/evidence",
    "POST /api/v1/projects/{projectId}/executions/{executionId}/rerun",
    "OPTIONS /api/v1/{proxy+}",
  ]) {
    api.route(route, fn.arn);
  }
  const maintenance = new sst.aws.Function("EmptyProjectCleanup", {
    runtime: "go",
    handler: "services/api",
    architecture: "arm64",
    memory: "256 MB",
    timeout: "60 seconds",
    dev: false,
    environment: {
      ...executionEnvironment,
      KURIER_STAGE: "dev-api",
      KURIER_SAVED_REQUESTS_SCHEMA_VERSION: "3",
      KURIER_PROTECTED_SECRETS_SCHEMA_VERSION: "1",
      KURIER_PROTECTED_TABLE: protectedTable.name,
      KURIER_CONTROL_TABLE: control.name,
      KURIER_MAINTENANCE: "empty-projects",
    },
    permissions: [
      ...tablePermissions,
      queueSend,
      {
        actions: ["s3:PutObject", "s3:GetObject", "s3:DeleteObject"],
        resources: [objectArn],
      },
      {
        actions: ["s3:ListBucket"],
        resources: [evidence.arn],
        conditions: [
          {
            test: "StringLike",
            variable: "s3:prefix",
            values: ["dev-api/projects/*"],
          },
        ],
      },
      {
        actions: ["dynamodb:GetItem", "dynamodb:Query", "dynamodb:DeleteItem"],
        resources: [protectedTable.arn],
      },
    ],
    logging: { retention: "1 week" },
  });
  const worker = new sst.aws.Function("CloudExecutionWorker", {
    runtime: "go",
    handler: "services/worker",
    architecture: "arm64",
    memory: "256 MB",
    timeout: "90 seconds",
    dev: false,
    // Development Free plan: queue concurrency is capped on the standard ESM,
    // without reserving any of the account's shared Lambda capacity.
    logging: { retention: "1 week" },
    environment: {
      ...executionEnvironment,
      KURIER_STAGE: "dev-api",
      KURIER_CONTROL_TABLE: control.name,
      KURIER_PROTECTED_TABLE: protectedTable.name,
      KURIER_KMS_KEY_ARN: stageKey.arn,
    },
    permissions: [
      ...tablePermissions,
      {
        actions: [
          "dynamodb:GetItem",
          "dynamodb:DeleteItem",
          "dynamodb:ConditionCheckItem",
        ],
        resources: [protectedTable.arn],
      },
      {
        actions: [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes",
        ],
        resources: [queue.arn],
      },
      { actions: ["s3:PutObject", "s3:GetObject"], resources: [objectArn] },
      {
        actions: ["kms:Decrypt"],
        resources: [stageKey.arn],
        conditions: purposeConditions("job-bindings"),
      },
    ],
  });
  new aws.lambda.EventSourceMapping("ExecutionQueueWorker", {
    eventSourceArn: queue.arn,
    functionName: worker.arn,
    batchSize: 1,
    maximumBatchingWindowInSeconds: 0,
    functionResponseTypes: ["ReportBatchItemFailures"],
    scalingConfig: { maximumConcurrency: 2 },
  });
  // Owned development fixtures have no data-resource permissions and no payload logs.
  const fixture = new sst.aws.Function("ExecutionControlledEndpoint", {
    runtime: "go",
    handler: "services/worker",
    architecture: "arm64",
    memory: "128 MB",
    timeout: "65 seconds",
    dev: false,
    logging: { retention: "1 week" },
    environment: {
      KURIER_STAGE: "dev-api",
      KURIER_CONTROLLED_ENDPOINT: "true",
    },
  });
  api.route("ANY /execution-fixture/{mode}", fixture.arn);
  const schedule = new sst.aws.Cron("EmptyProjectSchedule", {
    schedule: "rate(1 minute)",
    function: maintenance,
  });
  return {
    stage: $app.stage,
    region: "us-east-2",
    apiUrl: api.url,
    controlTable: control.name,
    protectedTable: protectedTable.name,
    protectedKeyArn: stageKey.arn,
    apiFunction: fn.name,
    workerFunction: worker.name,
    evidenceBucket: evidence.bucket,
    executionQueueUrl: queue.url,
    executionQueueArn: queue.arn,
    executionDlqUrl: dlq.url,
    controlledEndpointFunction: fixture.name,
    controlledEndpointUrl: $interpolate`${api.url}/execution-fixture/echo`,
    cleanupFunction: maintenance.name,
    cleanupSchedule: schedule.nodes.rule.name,
    frontendOrigin: origin,
    cursorKeyParameter: parameterName,
    cognitoUserPoolId: poolId!,
    cognitoClientId: clientId!,
    cognitoIssuer: environment.KURIER_COGNITO_ISSUER,
  };
}
