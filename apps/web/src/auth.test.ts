import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
const sdk = vi.hoisted(() => ({
  signIn: vi.fn(),
  fetchAuthSession: vi.fn(),
  signOut: vi.fn(),
  storage: { clear: vi.fn() },
  setStorage: vi.fn(),
  configure: vi.fn(),
}));
vi.mock("aws-amplify", () => ({ Amplify: { configure: sdk.configure } }));
vi.mock("aws-amplify/auth", () => ({
  signIn: sdk.signIn,
  fetchAuthSession: sdk.fetchAuthSession,
  signOut: sdk.signOut,
  signUp: vi.fn(),
  confirmSignUp: vi.fn(),
  resendSignUpCode: vi.fn(),
  resetPassword: vi.fn(),
  confirmResetPassword: vi.fn(),
}));
vi.mock("aws-amplify/auth/cognito", () => ({
  cognitoUserPoolsTokenProvider: { setKeyValueStorage: sdk.setStorage },
}));
vi.mock("aws-amplify/utils", () => ({ sharedInMemoryStorage: sdk.storage }));
beforeEach(() => {
  vi.resetModules();
  vi.clearAllMocks();
  vi.stubEnv("VITE_COGNITO_USER_POOL_ID", "us-east-2_fixture");
  vi.stubEnv("VITE_COGNITO_CLIENT_ID", "fixture-client");
  vi.stubEnv("VITE_KURIER_SESSION_BROKER", "false");
  sdk.signIn.mockResolvedValue({ isSignedIn: true });
  sdk.signOut.mockResolvedValue(undefined);
});
afterEach(() => vi.unstubAllGlobals());
describe("broker Cognito handoff", () => {
  it("does not adopt a Cognito sign-in that completes after page suspension", async () => {
    vi.stubEnv("VITE_KURIER_SESSION_BROKER", "true");
    vi.stubGlobal("BroadcastChannel", undefined);
    const fetcher = vi.fn();
    vi.stubGlobal("fetch", fetcher);
    let finish!: (result: { isSignedIn: boolean }) => void;
    sdk.signIn.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const { auth, SessionExpired } = await import("./auth");
    const pending = auth.login("person@example.com", "fixture-password");
    const rejection = expect(pending).rejects.toBeInstanceOf(SessionExpired);
    await Promise.resolve();
    auth.suspend();
    finish({ isSignedIn: true });
    await rejection;
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("captures the supported storage adapter's refresh credential and clears it without provider sign-out", async () => {
    vi.stubEnv("VITE_KURIER_SESSION_BROKER", "true");
    vi.stubGlobal("BroadcastChannel", undefined);
    const expiry = new Date(Date.now() + 3600000).toISOString();
    const fetcher = vi.fn().mockImplementation(
      async () =>
        new Response(
          JSON.stringify({
            data: {
              accessToken: "broker-access",
              accessExpiresAt: expiry,
              csrfToken: "fixture-csrf",
              sessionBinding: "fixture-binding",
              sessionExpiresAt: expiry,
            },
          }),
          { status: 201 },
        ),
    );
    vi.stubGlobal("fetch", fetcher);
    sdk.signIn.mockImplementationOnce(async () => {
      const storage = sdk.setStorage.mock.calls.at(-1)![0];
      await storage.setItem(
        "CognitoIdentityServiceProvider.fixture-client.person.refreshToken",
        "fixture-refresh",
      );
      return { isSignedIn: true };
    });
    sdk.fetchAuthSession.mockResolvedValue({
      tokens: { accessToken: { toString: () => "initial-access" } },
    });
    const { auth } = await import("./auth");
    expect(await auth.login("person@example.com", "fixture-password")).toBe(
      "done",
    );
    expect(JSON.parse(fetcher.mock.calls[0][1].body).refreshToken).toBe(
      "fixture-refresh",
    );
    expect(await auth.token()).toBe("broker-access");
    expect(
      await sdk.setStorage.mock.calls
        .at(-1)![0]
        .getItem(
          "CognitoIdentityServiceProvider.fixture-client.person.refreshToken",
        ),
    ).toBeNull();
    expect(sdk.signOut).not.toHaveBeenCalled();
  });
});
describe("memory-only Cognito session", () => {
  it("uses SRP and configures memory storage, with no session after reload", async () => {
    const { auth, SessionExpired } = await import("./auth");
    expect(sdk.setStorage).toHaveBeenCalledWith(sdk.storage);
    await expect(auth.token()).rejects.toBeInstanceOf(SessionExpired);
    await auth.login("person@example.com", "fixture-password");
    expect(sdk.signIn).toHaveBeenCalledWith(
      expect.objectContaining({ options: { authFlowType: "USER_SRP_AUTH" } }),
    );
  });
  it("coalesces refreshes and returns access tokens only", async () => {
    const { auth } = await import("./auth");
    await auth.login("person@example.com", "fixture-password");
    sdk.fetchAuthSession.mockResolvedValue({
      tokens: {
        accessToken: { toString: () => "access-fixture" },
        idToken: { toString: () => "id-fixture" },
      },
    });
    expect(await Promise.all([auth.token(true), auth.token(true)])).toEqual([
      "access-fixture",
      "access-fixture",
    ]);
    expect(sdk.fetchAuthSession).toHaveBeenCalledTimes(1);
    expect(sdk.fetchAuthSession).toHaveBeenCalledWith({ forceRefresh: true });
  });
  it("sign-out fences an in-flight refresh and clears memory after it settles", async () => {
    const { auth, SessionExpired } = await import("./auth");
    await auth.login("person@example.com", "fixture-password");
    let resolve!: (value: unknown) => void;
    sdk.fetchAuthSession.mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );
    const pending = auth.token(true);
    const rejected = expect(pending).rejects.toBeInstanceOf(SessionExpired);
    const leaving = auth.logout();
    resolve({ tokens: { accessToken: { toString: () => "late-access" } } });
    await rejected;
    await leaving;
    expect(sdk.storage.clear).toHaveBeenCalled();
    await expect(auth.token()).rejects.toBeInstanceOf(SessionExpired);
  });
});
