import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createDevelopmentApi } from "./development-api";
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
    this.nodes = { rule: { name } };
    const table: any = {};
    const stage: any = { accessLogSettings: { destinationArn: "logs" } };
    const fn: any = {};
    const api: any = { corsConfiguration: {} };
    args.transform?.api?.(api);
    args.transform?.table?.(table);
    args.transform?.stage?.(stage);
    args.transform?.function?.(fn);
    resources.push({ name, args, opts, table, stage, fn, api });
  }
  route(route: string, arn: string) {
    resources.push({ route, arn });
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
    cognito: {
      getUserPool: async () => ({ id: "us-east-2_Example" }),
      getUserPoolClient: async () => ({
        clientSecret: "",
        explicitAuthFlows: ["ALLOW_USER_SRP_AUTH"],
      }),
    },
    dynamodb: { TableItem: Resource },
  });
  vi.stubGlobal("sst", {
    aws: {
      Dynamo: Resource,
      Function: Resource,
      ApiGatewayV2: Resource,
      Cron: Resource,
    },
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
    expect(resources.filter((r) => r.route)).toHaveLength(8);
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
    expect(out.stage).toBe("dev-api");
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
