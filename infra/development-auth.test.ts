import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createDevelopmentAuth } from "./development-auth";
let poolArgs: Record<string, unknown>;
let clientArgs: Record<string, unknown>;
const identity = vi.fn();
beforeEach(() => {
  vi.stubEnv("KURIER_AWS_ACCOUNT_ID", "123456789012");
  vi.stubEnv("KURIER_AUTH_TIER", "ESSENTIALS");
  vi.stubGlobal("$app", { stage: "dev-auth" });
  vi.stubGlobal("aws", { getCallerIdentity: identity });
  identity.mockResolvedValue({
    accountId: "123456789012",
    arn: "arn:aws:iam::123456789012:user/test-deployer",
  });
  vi.stubGlobal("sst", {
    aws: {
      CognitoUserPool: class {
        id = { apply: (fn: (id: string) => string) => fn("us-east-2_fixture") };
        constructor(
          _name: string,
          args: {
            transform: { userPool: (args: Record<string, unknown>) => void };
          },
        ) {
          poolArgs = {};
          args.transform.userPool(poolArgs);
        }
        addClient(
          _name: string,
          args: {
            transform: { client: (args: Record<string, unknown>) => void };
          },
        ) {
          clientArgs = {};
          args.transform.client(clientArgs);
          return { id: "public-client" };
        }
      },
    },
  });
});
afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});
describe("development auth resource configuration", () => {
  it.each(["ESSENTIALS"])(
    "sets reviewed %s sessions without hosted product components",
    async (tier) => {
      vi.stubEnv("KURIER_AUTH_TIER", tier);
      const result = await createDevelopmentAuth();
      expect(result.stage).toBe("dev-auth");
      expect(poolArgs).toMatchObject({
        userPoolTier: tier,
        emailConfiguration: { emailSendingAccount: "COGNITO_DEFAULT" },
        deletionProtection: "ACTIVE",
        mfaConfiguration: "OFF",
      });
      expect(clientArgs).toMatchObject({
        generateSecret: false,
        explicitAuthFlows: ["ALLOW_USER_SRP_AUTH"],
        accessTokenValidity: 15,
        refreshTokenValidity: 1,
        enableTokenRevocation: true,
        preventUserExistenceErrors: "ENABLED",
        allowedOauthFlows: [],
        callbackUrls: [],
        refreshTokenRotation: {
          feature: "ENABLED",
        },
      });
    },
  );
  it.each(["", "LITE", "PLUS"])("rejects unaccepted tier %s", async (tier) => {
    vi.stubEnv("KURIER_AUTH_TIER", tier);
    await expect(createDevelopmentAuth()).rejects.toThrow("accepted");
    expect(identity).not.toHaveBeenCalled();
  });
  it.each([
    { accountId: "999999999999", arn: "arn:aws:iam::999999999999:user/test" },
    { accountId: "123456789012", arn: "arn:aws:iam::123456789012:root" },
  ])("rejects wrong account or root", async (caller) => {
    identity.mockResolvedValue(caller);
    await expect(createDevelopmentAuth()).rejects.toThrow("non-root");
  });
});
