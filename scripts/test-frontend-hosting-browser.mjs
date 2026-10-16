// Production assets + local HTTP/CSP fixture only. Never signs in or calls AWS.
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFileSync } from "node:fs";
import path from "node:path";
import { chromium } from "@playwright/test";
import { hostingHeaders } from "../infra/frontend-hosting.ts";
import { sha256 } from "../infra/frontend-artifact.ts";

const manifest = JSON.parse(
  readFileSync(".sst/frontend-hosting-artifact.json", "utf8"),
);
const dir = path.resolve(".build/frontend-releases", manifest.releaseId, "app");
const objects = new Map(
  manifest.files.map((f) => [
    "/" + f.path,
    { bytes: readFileSync(path.join(dir, f.path)), cache: f.cacheControl },
  ]),
);
const htmlA = objects.get("/index.html").bytes.toString();
const jsA = manifest.files.find(
  (f) => f.path.startsWith("assets/") && f.path.endsWith(".js"),
);
const jsBBytes = Buffer.concat([
  objects.get("/" + jsA.path).bytes,
  Buffer.from("\n// simulated next deployment\n"),
]);
const jsB = "/assets/index-" + sha256(jsBBytes).slice(0, 12) + ".js";
objects.set(jsB, {
  bytes: jsBBytes,
  cache: "public, max-age=31536000, immutable",
});
const htmlB = htmlA
  .replace(jsA.path, jsB.slice(1))
  .replace("<title>Kurier</title>", "<title>Kurier release B</title>");
let activeHtml = htmlA;
const csp = hostingHeaders(manifest.publicConfig.VITE_API_URL)
  .contentSecurityPolicy.contentSecurityPolicy;
const server = createServer((req, res) => {
  const url = new URL(req.url, "http://localhost");
  const file = objects.get(url.pathname);
  res.setHeader("Content-Security-Policy", csp);
  res.setHeader("X-Content-Type-Options", "nosniff");
  res.setHeader("Cache-Control", file?.cache ?? "no-store");
  const missing =
    !file &&
    (url.pathname.startsWith("/assets/") ||
      url.pathname.startsWith("/api/") ||
      url.pathname.includes("."));
  res.statusCode = missing ? 404 : 200;
  res.setHeader(
    "Content-Type",
    url.pathname.endsWith(".js")
      ? "application/javascript"
      : url.pathname.endsWith(".css")
        ? "text/css"
        : "text/html",
  );
  res.end(
    missing
      ? "Not found"
      : url.pathname === "/index.html" || !file
        ? activeHtml
        : file.bytes,
  );
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext();
  let unexpectedNetwork = 0;
  await context.route("**/*", (route) => {
    if (new URL(route.request().url()).origin !== origin) {
      unexpectedNetwork++;
      return route.abort();
    }
    return route.continue();
  });
  const page = await context.newPage();
  let cspViolations = 0;
  await page.exposeFunction("recordViolation", () => {
    cspViolations++;
  });
  await page.addInitScript(() =>
    document.addEventListener(
      "securitypolicyviolation",
      () => void window.recordViolation(),
    ),
  );
  for (const location of [
    "/",
    "/project/opaque",
    "/project/opaque/?view=history",
  ]) {
    const response = await page.goto(origin + location);
    assert.equal(response.status(), 200);
    assert.match(response.headers()["cache-control"], /no-store/);
    await page.getByRole("button", { name: "Sign in", exact: true }).waitFor();
  }
  await page.getByRole("link", { name: "Skip to workspace" }).focus();
  assert.equal(
    await page.evaluate(() => document.activeElement.textContent),
    "Skip to workspace",
  );
  await page.setViewportSize({ width: 375, height: 812 });
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
    true,
  );
  activeHtml = htmlB;
  await page.reload();
  await page.getByRole("button", { name: "Sign in", exact: true }).waitFor();
  assert.equal(await page.title(), "Kurier release B");
  assert.equal(
    (await context.request.get(origin + "/" + jsA.path)).status(),
    200,
  );
  assert.equal(
    (
      await context.request.get(origin + "/assets/missing-abcdefgh.js")
    ).status(),
    404,
  );
  assert.equal(
    (await context.request.get(origin + "/api/v1/projects")).status(),
    404,
  );
  activeHtml = htmlA;
  await page.reload();
  await page.getByRole("button", { name: "Sign in", exact: true }).waitFor();
  assert.equal(await page.title(), "Kurier");
  assert.equal(
    await page.evaluate(
      async () =>
        localStorage.length +
        sessionStorage.length +
        (await caches.keys()).length +
        (await indexedDB.databases()).length,
    ),
    0,
  );
  assert.equal(cspViolations, 0);
  assert.equal(unexpectedNetwork, 0);
  const broken = await context.newPage();
  await broken.route("**/assets/*.js", (route) => route.abort());
  await broken.goto(origin);
  await broken
    .getByRole("alert")
    .filter({ hasText: "could not finish loading" })
    .waitFor();
  await broken.getByRole("link", { name: "Reload Kurier" }).waitFor();
  console.log(
    JSON.stringify({
      productionChromiumLocalFixture: "PASS",
      deepLinks: true,
      strictCsp: true,
      keyboardAndMobile: true,
      simulatedTwoReleaseCacheAndRollback: true,
      missingAssetsAndApi404: true,
      emptyBrowserPersistence: true,
      startupFailure: true,
      awsRequests: unexpectedNetwork,
      hostedAwsValidated: false,
    }),
  );
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
