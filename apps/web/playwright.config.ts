import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  // The broker fixture has its own HTTPS server and capability configuration.
  testIgnore: "browser-session.spec.ts",
  workers: 1,
  timeout: 60000,
  retries: 0,
  use: {
    baseURL: "http://localhost:5175",
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  reporter: "line",
  webServer: {
    command: "npm run dev -- --host localhost --port 5175 --strictPort",
    url: "http://localhost:5175",
    reuseExistingServer: false,
  },
});
