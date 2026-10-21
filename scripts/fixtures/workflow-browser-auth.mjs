// Pinned application, real Amplify/browser network. Only the TLS proxy maps origins.
import assert from "node:assert/strict";
import { chromium } from "@playwright/test";
const browser = await chromium.launch({
  headless: true,
  proxy: { server: "http://127.0.0.1:18080" },
  args: ["--disable-background-networking", "--disable-component-update"],
});
try {
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    serviceWorkers: "block",
  });
  const page = await context.newPage();
  page.setDefaultTimeout(15000);
  await page.goto("https://d2kpq905qyh9zg.cloudfront.net", {
    waitUntil: "domcontentloaded",
  });
  await page
    .getByLabel("Email", { exact: true })
    .fill("fixture@example.invalid");
  await page
    .getByLabel("Password", { exact: true })
    .fill("Local-only-password1!");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page
    .getByRole("heading", { name: "Your projects", exact: true })
    .waitFor();
  await page
    .getByRole("button", { name: "Refresh projects", exact: true })
    .waitFor();
  await page.waitForTimeout(3500); // Local fixture access token lasts only two seconds.
  const response = page.waitForResponse(
    (r) =>
      r.url().includes("/api/v1/projects") &&
      r.request().method() === "GET" &&
      r.status() === 200,
  );
  await page
    .getByRole("button", { name: "Refresh projects", exact: true })
    .click();
  await response;
  assert.deepEqual(
    await page.evaluate(async () => ({
      local: localStorage.length,
      session: sessionStorage.length,
      indexed: (await indexedDB.databases()).length,
      caches: (await caches.keys()).length,
    })),
    { local: 0, session: 0, indexed: 0, caches: 0 },
  );
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await page.getByLabel("Email", { exact: true }).waitFor();
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.getByLabel("Email", { exact: true }).waitFor();
  assert.equal(
    await page
      .getByRole("heading", { name: "Your projects", exact: true })
      .count(),
    0,
  );
  await context.close();
} finally {
  await browser.close();
}
