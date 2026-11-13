import { defineConfig } from "@playwright/test";

// Local HTTPS browser fixture only. Cognito, CloudFront and the broker server
// are tested separately; these routes exercise React's real document lifecycle.
export default defineConfig({
  testDir: "./e2e",
  testMatch: "browser-session.spec.ts",
  workers: 1,
  timeout: 30000,
  use: {
    baseURL: "https://localhost:5176",
    ignoreHTTPSErrors: true,
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
  reporter: "line",
  webServer: {
    command: "npm run dev -- --host localhost --port 5176 --strictPort",
    url: "https://localhost:5176",
    ignoreHTTPSErrors: true,
    reuseExistingServer: false,
    env: {
      VITE_KURIER_SESSION_BROKER: "true",
      VITE_COGNITO_USER_POOL_ID: "us-east-2_fixture",
      VITE_COGNITO_CLIENT_ID: "fixtureclient",
      VITE_API_URL: "https://fixture.execute-api.us-east-2.amazonaws.com",
    },
  },
});
