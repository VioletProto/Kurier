import { describe, expect, it } from "vitest";
import { hostedPublicConfig } from "../public-config";
import { sessionDevelopmentConfig } from "../session-dev-config";

const publicConfig = {
  VITE_API_URL: "https://example.execute-api.us-east-2.amazonaws.com",
  VITE_COGNITO_USER_POOL_ID: "us-east-2_Example",
  VITE_COGNITO_CLIENT_ID: "example",
  VITE_KURIER_STAGE: "dev-api",
};

describe("broker public and local configuration boundaries", () => {
  it("preserves retained release configuration and admits only a boolean capability", () => {
    expect(hostedPublicConfig(publicConfig)).toEqual(publicConfig);
    expect(
      hostedPublicConfig({
        ...publicConfig,
        VITE_KURIER_SESSION_BROKER: "true",
      }),
    ).toEqual({ ...publicConfig, VITE_KURIER_SESSION_BROKER: "true" });
    expect(() =>
      hostedPublicConfig({
        ...publicConfig,
        VITE_KURIER_SESSION_BROKER: "secret",
      }),
    ).toThrow();
    expect(() =>
      hostedPublicConfig({
        ...publicConfig,
        VITE_SESSION_ORIGIN_SECRET: "secret",
      }),
    ).toThrow();
  });

  it("never proxies broker credentials to nonloopback development targets", () => {
    for (const target of [
      "https://api.example:8081",
      "http://127.0.0.1:8081/path",
      "http://secret@127.0.0.1:8081",
      "http://127.0.0.1:8081?secret=value",
    ])
      expect(() =>
        sessionDevelopmentConfig({
          VITE_KURIER_SESSION_BROKER: "true",
          KURIER_SESSION_PROXY_TARGET: target,
        }),
      ).toThrow("loopback");
    expect(() =>
      sessionDevelopmentConfig({ VITE_KURIER_SESSION_BROKER: "true" }),
    ).toThrow("HTTPS");
    expect(sessionDevelopmentConfig({})).toEqual({});
  });
});
