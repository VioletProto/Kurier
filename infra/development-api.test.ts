import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  browserSessionOriginSecret,
  createDevelopmentApi,
} from "./development-api";
import { createHmac } from "node:crypto";
vi.mock("./frontend-hosting", () => ({
  createDevelopmentWeb: () => ({
    apply: (fn: any) => fn({ siteUrl: "https://dexample.cloudfront.net" }),
  }),
}));
const resources: Record<string, any>[] = [];
const identity = vi.fn();
class Resource {
  name: string;
  arn: string;
  id: string;
  url: string;
  nodes: any;
  constructor(name: string, args: any, opts?: any) {
    this.name = name;
    this.arn = `arn:test:${name}`;
    this.id = name;
    this.url = "https://abc.execute-api.us-east-2.amazonaws.com";
    this.nodes = { rule: { name }, api: { id: name } };
    const table: any = {};
    const stage: any = { accessLogSettings: { destinationArn: "logs" } };
    const fn: any = {};
    const api: any = { corsConfiguration: {} };
    args.transform?.api?.(api);
    args.transform?.table?.(table);
    args.transform?.stage?.(stage);
    args.transform?.function?.(fn);
    resources.push({
      name,
      args,
      opts,
      table,
      stage,
      fn,
      api,
      ...(args.routeKey ? { route: args.routeKey, arn: args.target } : {}),
    });
  }
  route(route: string, arn: string) {
    resources.push({ route, arn });
    return { nodes: { integration: { id: `existing-integration:${route}` } } };
  }
}
beforeEach(() => {
  resources.length = 0;
  vi.stubEnv("KURIER_AWS_ACCOUNT_ID", "123456789012");
  vi.stubEnv("KURIER_COGNITO_POOL_ID", "us-east-2_Example");
  vi.stubEnv("KURIER_COGNITO_CLIENT_ID", "publicclient");
  identity.mockResolvedValue({
    accountId: "123456789012",
    arn: "arn:aws:iam::123456789012:user/test",
  });
  vi.stubGlobal("aws", {
    getCallerIdentity: identity,
    ssm: {
      getParameterOutput: () => ({
        value: Buffer.alloc(32, 7).toString("base64"),
      }),
    },
    cognito: {
      getUserPool: async () => ({ id: "us-east-2_Example" }),
      getUserPoolClient: async () => ({
        clientSecret: "",
        explicitAuthFlows: ["ALLOW_USER_SRP_AUTH"],
      }),
    },
    dynamodb: { TableItem: Resource },
    kms: { Key: Resource, Alias: Resource },
    s3: {
      BucketV2: Resource,
      BucketPublicAccessBlock: Resource,
      BucketServerSideEncryptionConfigurationV2: Resource,
      BucketOwnershipControls: Resource,
      BucketPolicy: Resource,
    },
    sqs: { Queue: Resource, RedriveAllowPolicy: Resource },
    lambda: { EventSourceMapping: Resource },
    apigatewayv2: { Route: Resource },
  });
  vi.stubGlobal("sst", {
    aws: {
      Dynamo: Resource,
      Function: Resource,
      ApiGatewayV2: Resource,
      Cron: Resource,
    },
  });
  vi.stubGlobal("$jsonStringify", JSON.stringify);
  vi.stubGlobal("$util", {
    secret: (value: any) => ({ apply: (fn: any) => fn(value) }),
  });
  vi.stubGlobal("$app", { stage: "dev-api" });
  vi.stubGlobal(
    "$interpolate",
    (strings: TemplateStringsArray, ...values: unknown[]) =>
      strings.reduce((out, s, i) => out + s + (values[i] ?? ""), ""),
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
describe("scoped development API infrastructure", () => {
  it("defines keys-only indexes, guarded ARM64 handlers, private key access and safe logs", async () => {
    const out = await createDevelopmentApi();
    const table = resources.find((r) => r.name === "Control")!;
    expect(table.table).toMatchObject({
      billingMode: "PAY_PER_REQUEST",
      pointInTimeRecovery: { enabled: true },
      deletionProtectionEnabled: true,
    });
    expect(Object.keys(table.args.globalIndexes)).toEqual([
      "GSI1",
      "GSI2",
      "GSI3",
    ]);
    for (const index of Object.values(table.args.globalIndexes) as any[])
      expect(index.projection).toBe("keys-only");
    const api = resources.find((r) => r.name === "UsersProjectsApi")!;
    expect(api.args).toMatchObject({
      runtime: "go",
      architecture: "arm64",
      dev: false,
    });
    expect(api.args.environment.KURIER_CURSOR_KEY_BASE64).toBeUndefined();
    expect(api.args.permissions[1]).toMatchObject({
      actions: ["ssm:GetParameter"],
      resources: [
        "arn:aws:ssm:us-east-2:123456789012:parameter/kurier/dev-api/cursor-key",
      ],
    });
    expect(resources.filter((r) => r.route).map((r) => r.route)).toContain(
      "GET /api/v1/users/me",
    );
    expect(resources.filter((r) => r.route)).toHaveLength(69);
    const declaredRoutes = resources.filter((r) => r.route).map((r) => r.route);
    expect(declaredRoutes).toEqual(
      expect.arrayContaining([
        "POST /api/v1/projects/{projectId}/environments",
        "GET /api/v1/projects/{projectId}/environments",
        "GET /api/v1/projects/{projectId}/environments/{environmentId}",
        "PATCH /api/v1/projects/{projectId}/environments/{environmentId}",
        "DELETE /api/v1/projects/{projectId}/environments/{environmentId}",
        "OPTIONS /api/v1/projects/{projectId}/environments",
        "OPTIONS /api/v1/projects/{projectId}/environments/{environmentId}",
        "POST /api/v1/local-agent/capabilities",
      ]),
    );
    expect(
      declaredRoutes.filter((route) => route.includes("/openapi-imports")),
    ).toEqual([
      "POST /api/v1/projects/{projectId}/openapi-imports",
      "GET /api/v1/projects/{projectId}/openapi-imports",
      "GET /api/v1/projects/{projectId}/openapi-imports/{importId}",
      "PATCH /api/v1/projects/{projectId}/openapi-imports/{importId}",
      "DELETE /api/v1/projects/{projectId}/openapi-imports/{importId}",
      "POST /api/v1/projects/{projectId}/openapi-imports/{importId}/publish",
      "GET /api/v1/projects/{projectId}/openapi-imports/{importId}/export",
      "OPTIONS /api/v1/projects/{projectId}/openapi-imports",
      "OPTIONS /api/v1/projects/{projectId}/openapi-imports/{importId}",
      "OPTIONS /api/v1/projects/{projectId}/openapi-imports/{importId}/publish",
      "OPTIONS /api/v1/projects/{projectId}/openapi-imports/{importId}/export",
    ]);
    const importRoutes = resources.filter((r) =>
      r.name?.startsWith("OpenAPIImport"),
    );
    expect(importRoutes).toHaveLength(11);
    for (const route of importRoutes)
      expect(route.args).toMatchObject({
        apiId: "DevelopmentHttpApi",
        authorizationType: "NONE",
        target: "integrations/existing-integration:GET /api/v1/users/me",
      });
    expect(
      resources.filter((r) => r.route?.includes("/mcp/")).map((r) => r.route),
    ).toEqual([
      "POST /api/v1/mcp/tokens",
      "GET /api/v1/mcp/tokens/current",
      "DELETE /api/v1/mcp/tokens/{tokenId}",
      "POST /api/v1/mcp/invocations",
    ]);
    expect(api.args.environment.KURIER_MCP_READ_ONLY_SCHEMA_VERSION).toBe("1");
    expect(api.args.environment.KURIER_SAVED_REQUESTS_SCHEMA_VERSION).toBe("7");
    expect(resources.filter((r) => r.route?.includes("workflow"))).toHaveLength(
      9,
    );
    expect(
      api.args.environment.KURIER_SEQUENTIAL_WORKFLOWS_SCHEMA_VERSION,
    ).toBe("1");

    const http = resources.find((r) => r.name === "DevelopmentHttpApi")!;
    expect(http.args.cors).toBe(false);
    expect(http.api.corsConfiguration).toBeUndefined();
    expect(
      Object.keys(JSON.parse(http.stage.accessLogSettings.format)),
    ).toEqual(["requestId", "route", "status", "latency"]);
    expect(
      resources.find((r) => r.name === "EmptyProjectCleanup")!.fn
        .reservedConcurrentExecutions,
    ).toBeUndefined();
    const protectedTable = resources.find((r) => r.name === "Protected")!;
    expect(protectedTable.args.globalIndexes).toBeUndefined();
    expect(protectedTable.table).toMatchObject({
      deletionProtectionEnabled: true,
      pointInTimeRecovery: { enabled: true },
    });
    expect(api.args.permissions.flatMap((p: any) => p.actions)).toContain(
      "kms:GenerateDataKey",
    );
    expect(api.args.permissions.flatMap((p: any) => p.actions)).toContain(
      "kms:Decrypt",
    );
    expect(
      resources
        .find((r) => r.name === "EmptyProjectCleanup")!
        .args.permissions.flatMap((p: any) => p.actions)
        .some((a: string) => a.startsWith("kms:")),
    ).toBe(false);
    expect(
      resources.find((r) => r.name === "ExecutionQueue")!.args,
    ).toMatchObject({
      visibilityTimeoutSeconds: 540,
      messageRetentionSeconds: 345600,
      sqsManagedSseEnabled: true,
    });
    expect(
      resources.find((r) => r.name === "ExecutionQueueWorker")!.args,
    ).toMatchObject({
      batchSize: 1,
      functionResponseTypes: ["ReportBatchItemFailures"],
      scalingConfig: { maximumConcurrency: 2 },
    });
    expect(
      resources.find((r) => r.name === "CloudExecutionWorker")!.args,
    ).toMatchObject({ timeout: "90 seconds" });
    expect(
      resources.find((r) => r.name === "CloudExecutionWorker")!.args
        .concurrency,
    ).toBeUndefined();
    expect(
      resources.find((r) => r.name === "ExecutionQueueWorker")!.args
        .provisionedPollerConfig,
    ).toBeUndefined();
    expect(
      resources.find((r) => r.name === "ExecutionEvidencePublicAccess")!.args,
    ).toMatchObject({
      blockPublicAcls: true,
      blockPublicPolicy: true,
      ignorePublicAcls: true,
      restrictPublicBuckets: true,
    });
    const workerPermissions = resources.find(
      (r) => r.name === "CloudExecutionWorker",
    )!.args.permissions;
    expect(
      workerPermissions.find(
        (p: any) =>
          p.actions.includes("kms:Decrypt") &&
          p.conditions?.some(
            (c: any) =>
              c.variable === "kms:EncryptionContext:purpose" &&
              c.values.includes("job-bindings"),
          ),
      ).conditions,
    ).toContainEqual({
      test: "StringEquals",
      variable: "kms:EncryptionContext:purpose",
      values: ["job-bindings"],
    });
    expect(
      resources.find((r) => r.name === "ExecutionControlledEndpoint")!.args
        .permissions,
    ).toBeUndefined();
    expect(out.stage).toBe("dev-api");
  });
  it("isolates disposable sessions, broker crypto and bounded cleanup from product roles", async () => {
    const out = await createDevelopmentApi();
    const table = resources.find((r) => r.name === "BrowserSessions")!;
    expect(table.args).toMatchObject({
      fields: { PK: "string" },
      primaryIndex: { hashKey: "PK" },
      ttl: "ttl",
    });
    expect(table.table).toMatchObject({
      pointInTimeRecovery: { enabled: false },
      deletionProtectionEnabled: true,
      billingMode: "PAY_PER_REQUEST",
    });
    const broker = resources.find((r) => r.name === "BrowserSessionBroker")!;
    expect(broker.args).toMatchObject({
      handler:
        "services/api/github.com/VioletProto/Kurier/services/api/cmd/browser-session",
      runtime: "go",
      architecture: "arm64",
      timeout: "15 seconds",
      dev: false,
    });
    expect(broker.args.environment).toMatchObject({
      KURIER_SESSION_TABLE: "BrowserSessions",
      KURIER_SESSION_KEY_ARN: "arn:test:ProtectedStageKey",
      KURIER_SESSION_ORIGIN_SECRET_PARAMETER: "/kurier/dev-api/cursor-key",
      KURIER_SESSION_FRONTEND_ORIGIN: "https://dexample.cloudfront.net",
    });
    expect(
      broker.args.environment.KURIER_SESSION_ORIGIN_SECRET,
    ).toBeUndefined();
    const control = broker.args.permissions.find((p: any) =>
      p.resources.includes("arn:test:Control"),
    );
    expect(control).toMatchObject({
      actions: ["dynamodb:GetItem", "dynamodb:ConditionCheckItem"],
      conditions: [
        {
          test: "ForAllValues:StringEquals",
          variable: "dynamodb:LeadingKeys",
          values: ["STAGE#dev-api"],
        },
      ],
    });
    const crypto = broker.args.permissions.find((p: any) =>
      p.actions.includes("kms:Decrypt"),
    );
    expect(crypto.conditions).toContainEqual({
      test: "StringEquals",
      variable: "kms:EncryptionContext:purpose",
      values: ["browser-session"],
    });
    for (const role of [
      "UsersProjectsApi",
      "CloudExecutionWorker",
      "EmptyProjectCleanup",
    ]) {
      expect(
        JSON.stringify(
          resources.find((r) => r.name === role)!.args.permissions,
        ),
      ).not.toContain("BrowserSessions");
      expect(
        JSON.stringify(
          resources.find((r) => r.name === role)!.args.permissions,
        ),
      ).not.toContain('"browser-session"');
    }
    expect(
      resources.find((r) => r.name === "BrowserSessionCleanupSchedule")!.args,
    ).toEqual({
      schedule: "rate(1 minute)",
      function: expect.objectContaining({ name: "BrowserSessionBroker" }),
      event: { maintenance: true },
    });
    expect(JSON.stringify(out)).not.toContain("KURIER_SESSION_ORIGIN_SECRET");
    const encoded = Buffer.alloc(32, 7).toString("base64");
    expect(browserSessionOriginSecret(encoded)).toBe(
      createHmac("sha256", encoded)
        .update("kurier:dev-api:browser-session-origin:v1")
        .digest("base64url"),
    );
    expect(() => browserSessionOriginSecret("short")).toThrow("unavailable");
  });
  it("refuses root and wrong-account deployment", async () => {
    identity.mockResolvedValue({
      accountId: "123456789012",
      arn: "arn:aws:iam::123456789012:root",
    });
    await expect(createDevelopmentApi()).rejects.toThrow("non-root");
    expect(resources).toHaveLength(0);
  });
});
