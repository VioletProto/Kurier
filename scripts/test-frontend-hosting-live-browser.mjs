// Genuine hosted signed-out Chromium/CORS check. No fixture auth or account writes.
import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { chromium } from "@playwright/test";
const out = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
const site = out.frontendHosting?.siteUrl;
if (!/^https:\/\/d[a-z0-9]+\.cloudfront\.net$/.test(site ?? ""))
  throw Error("Scoped host required.");
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext();
  const page = await context.newPage();
  const violations = [];
  page.on("console", (m) => {
    if (
      m.type() === "error" &&
      /Content Security Policy|Refused to/.test(m.text())
    )
      violations.push("CSP violation");
  });
  await page.goto(site + "/project/hosting-check/deep-link", {
    waitUntil: "networkidle",
  });
  await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
  assert.equal(await page.title(), "Kurier");
  const proof = await page.evaluate(async (api) => {
    const r = await fetch(api + "/api/v1/projects", {
      headers: { "Content-Type": "application/json" },
      cache: "no-store",
    });
    return {
      origin: location.origin,
      status: r.status,
      exposed: r.headers.get("cache-control"),
      bodyReadable: (await r.text()).length > 0,
      localStorage: localStorage.length,
      sessionStorage: sessionStorage.length,
      indexedDb: (await indexedDB.databases()).length,
      caches: (await caches.keys()).length,
    };
  }, out.apiUrl);
  assert.equal(proof.origin, site);
  assert.equal(proof.status, 401);
  assert.equal(proof.bodyReadable, true);
  for (const k of ["localStorage", "sessionStorage", "indexedDb", "caches"])
    assert.equal(proof[k], 0);
  assert.deepEqual(violations, []);
  await page.getByRole("link", { name: "Skip to workspace" }).focus();
  assert.equal(
    await page
      .getByRole("link", { name: "Skip to workspace" })
      .evaluate((el) => el === document.activeElement),
    true,
  );
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
    true,
  );
  const report = {
    checkedAt: new Date().toISOString(),
    siteUrl: site,
    genuineHostedChromium: true,
    signedOutDeepLink: true,
    realCrossOriginReadable401: true,
    enforcedCspNoViolations: true,
    emptyBrowserPersistence: true,
    keyboardAndMobileSmoke: true,
    authEmailRefreshValidated: false,
    proof,
  };
  writeFileSync(
    ".sst/frontend-hosting-live-browser.json",
    JSON.stringify(report, null, 2) + "\n",
  );
  console.log(JSON.stringify(report));
  await context.close();
} finally {
  await browser.close();
}
