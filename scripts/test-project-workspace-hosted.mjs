// Deployed static assets; synthetic session/project responses for workspace UI.
// No real sign-in, user data, backend mutations, traces or screenshots.
import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { chromium, expect } from "@playwright/test";

const outputs = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
const { siteUrl, publicArtifact: manifest } = outputs.frontendHosting;
assert.equal(siteUrl, "https://d2kpq905qyh9zg.cloudfront.net");
const response = await fetch(siteUrl + "/artifact.json");
assert.equal(response.status, 200);
assert.deepEqual(await response.json(), manifest);
for (const file of manifest.files) {
  const response = await fetch(siteUrl + "/" + file.path);
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("cache-control"), file.cacheControl);
  const bytes = Buffer.from(await response.arrayBuffer());
  assert.equal(bytes.length, file.bytes);
  assert.equal(createHash("sha256").update(bytes).digest("hex"), file.sha256);
}
const proxy = process.env.HTTPS_PROXY ?? process.env.https_proxy;
const browser = await chromium.launch({
  headless: true,
  args: ["--disable-quic", "--no-proxy-server"],
  ...(proxy ? { proxy: { server: proxy } } : {}),
});
const results = [];
try {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", () => errors.push("page error"));
  await page.goto(siteUrl, { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { name: "Sign in", exact: true }),
  ).toBeVisible();
  await page.reload({ waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { name: "Sign in", exact: true }),
  ).toBeVisible();
  assert.deepEqual(errors, []);
  await context.close();
  for (const width of [375, 1366, 1920]) {
    const context = await browser.newContext({
      viewport: { width, height: 1000 },
    });
    const page = await context.newPage();
    const errors = [];
    page.on("pageerror", () => errors.push("page error"));
    const mutations = [];
    const project = {
      projectId: "hosted-ui-fixture",
      name: "Hosted UI fixture",
      version: 0,
      createdAt: "2026-11-13T00:00:00Z",
      updatedAt: "2026-11-13T00:00:00Z",
    };
    await page.route("**/api/**", async (route) => {
      const pathname = new URL(route.request().url()).pathname;
      const session = pathname.startsWith("/api/v1/browser-session");
      if (!session && route.request().method() !== "GET")
        mutations.push(pathname);
      const expires = new Date(Date.now() + 3600000).toISOString();
      const data = session
        ? {
            sessionBinding: "ui-fixture",
            csrfToken: "synthetic-ui-only",
            sessionExpiresAt: expires,
            accessToken: "synthetic-ui-only",
            accessExpiresAt: expires,
          }
        : pathname.endsWith("/users/me")
          ? { user: { userId: "fixture", displayName: "UI fixture" } }
          : pathname.endsWith("/projects/hosted-ui-fixture")
            ? { project }
            : {
                items: pathname.endsWith("/projects") ? [project] : [],
                nextCursor: null,
              };
      await route.fulfill({
        json: { data },
        headers: { ETag: '\"0\"', "Access-Control-Expose-Headers": "ETag" },
      });
    });
    await page.goto(siteUrl, { waitUntil: "domcontentloaded" });
    await page.getByRole("button", { name: project.name, exact: true }).click();
    const tabs = page.getByRole("tab");
    await expect(tabs).toHaveCount(7);
    const requests = page.getByRole("tab", { name: "Requests & evidence" });
    await expect(requests).toHaveAttribute("aria-selected", "true");
    await page
      .getByRole("button", { name: "New request", exact: true })
      .click();
    await page
      .getByLabel("Request name", { exact: true })
      .fill("Unsaved hosted draft");
    await page
      .getByLabel("Public request URL")
      .fill("https://example.com/" + "x".repeat(400));
    await requests.focus();
    await page.keyboard.press("ArrowRight");
    await expect(
      page.getByRole("tab", { name: "Workflows", exact: true }),
    ).toBeFocused();
    await page.keyboard.press("End");
    const settings = page.getByRole("tab", { name: "Project settings" });
    await expect(settings).toBeFocused();
    await page
      .getByLabel("Project name", { exact: true })
      .fill("Unsaved rename");
    await settings.focus();
    await page.keyboard.press("ArrowRight");
    await expect(requests).toBeFocused();
    await expect(page.getByLabel("Request name", { exact: true })).toHaveValue(
      "Unsaved hosted draft",
    );
    await page.keyboard.press("ArrowLeft");
    await expect(settings).toBeFocused();
    await expect(page.getByLabel("Project name", { exact: true })).toHaveValue(
      "Unsaved rename",
    );
    await page.keyboard.press("Home");
    await expect(requests).toBeFocused();
    for (const tab of await tabs.all()) {
      await tab.click();
      await expect(page.getByRole("tabpanel")).toHaveCount(1);
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
        true,
      );
    }
    await page.getByRole("button", { name: "Dark theme" }).click();
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    assert.deepEqual(mutations, []);
    assert.deepEqual(errors, []);
    assert.equal(
      await page.evaluate(() => localStorage.length + sessionStorage.length),
      0,
    );
    results.push({
      width,
      keyboard: true,
      draftPreservation: true,
      noPageOverflow: true,
    });
    await context.close();
  }
} finally {
  await browser.close();
}
const report = {
  checkedAt: new Date().toISOString(),
  siteUrl,
  releaseId: manifest.releaseId,
  publicAssetHashesPassed: true,
  genuineSignedOutNavigationAndReload: true,
  workspaceApiAndSession: "synthetic responses; deployed JS/CSS/HTML",
  results,
  realAuthenticatedProductValidation: false,
};
writeFileSync(
  ".sst/workspace-tabs-hosted.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log(JSON.stringify(report));
