import { beforeEach, describe, expect, it, vi } from "vitest";
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
  sdk.signIn.mockResolvedValue({ isSignedIn: true });
  sdk.signOut.mockResolvedValue(undefined);
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
