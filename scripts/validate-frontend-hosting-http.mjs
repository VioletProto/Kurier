// Genuine public HTTP/CDN checks. No credentials, user creation or application writes.
import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { hostingHeaders } from "../infra/frontend-hosting.ts";
import { sha256 } from "../infra/frontend-artifact.ts";
const out = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
const web = out.frontendHosting;
if (
  out.stage !== "dev-api" ||
  out.region !== "us-east-2" ||
  !/^https:\/\/d[a-z0-9]+\.cloudfront\.net$/.test(web?.siteUrl ?? "") ||
  !/^kurier-dev-api-web-[a-z0-9-]+$/.test(web.webBucket)
)
  throw Error("Scoped deployed outputs required.");
const expected = JSON.parse(
  readFileSync(".sst/frontend-hosting-artifact.json", "utf8"),
);
const checks = [];
async function get(url, options = {}) {
  const r = await fetch(url, {
    ...options,
    redirect: "manual",
    signal: AbortSignal.timeout(30000),
  });
  const bytes = Buffer.from(await r.arrayBuffer());
  return { status: r.status, headers: r.headers, bytes };
}
const headerRules = {
  "content-security-policy": hostingHeaders(out.apiUrl).contentSecurityPolicy
    .contentSecurityPolicy,
  "x-content-type-options": "nosniff",
  "x-frame-options": "DENY",
  "referrer-policy": "no-referrer",
  "strict-transport-security": "max-age=31536000",
  "permissions-policy": "camera=(), microphone=(), geolocation=()",
};
function secure(r) {
  for (const [k, v] of Object.entries(headerRules))
    assert.equal(r.headers.get(k), v, k);
}
const live = await get(`${web.siteUrl}/artifact.json`);
assert.equal(live.status, 200);
assert.deepEqual(JSON.parse(live.bytes), expected);
secure(live);
assert.match(live.headers.get("cache-control"), /no-store/);
for (const file of expected.files) {
  const r = await get(`${web.siteUrl}/${file.path}`);
  assert.equal(r.status, 200, file.path);
  secure(r);
  assert.equal(r.bytes.length, file.bytes);
  assert.equal(sha256(r.bytes), file.sha256, file.path);
  assert.equal(r.headers.get("cache-control"), file.cacheControl);
  const type = file.path.endsWith(".js")
    ? /javascript/
    : file.path.endsWith(".css")
      ? /text\/css/
      : /text\/html/;
  assert.match(r.headers.get("content-type"), type);
  checks.push({
    path: file.path,
    status: r.status,
    sha256: file.sha256,
    contentType: r.headers.get("content-type"),
    cacheControl: r.headers.get("cache-control"),
  });
}
const shell = expected.files.find((f) => f.path === "index.html");
for (const route of ["/", "/project/hosting-check/deep-link"]) {
  const r = await get(web.siteUrl + route);
  assert.equal(r.status, 200);
  assert.equal(sha256(r.bytes), shell.sha256);
  secure(r);
  assert.match(r.headers.get("cache-control"), /no-store/);
}
for (const route of [
  "/missing.js",
  "/assets/missing-abcdefgh.js",
  "/api/v1/projects",
  `/_releases/${expected.releaseId}/startup.js`,
  `/_releases/${expected.releaseId}/404.html`,
  `/_releases/${expected.releaseId}/artifact.json`,
]) {
  const r = await get(web.siteUrl + route);
  assert.equal(r.status, 404, route);
  assert.match(r.headers.get("cache-control"), /no-store/);
  assert.equal(r.headers.get("x-content-type-options"), "nosniff");
  assert.notEqual(sha256(r.bytes), shell.sha256);
}
const credentialed = await get(web.siteUrl + "/index.html", {
  headers: {
    Authorization: "Bearer public-negative-probe",
    Cookie: "hosting_probe=public-negative-probe",
  },
});
assert.equal(credentialed.status, 200);
assert.equal(sha256(credentialed.bytes), shell.sha256);
for (const suffix of ["/index.html", "/"]) {
  const r = await get(
    `https://${web.webBucket}.s3.us-east-2.amazonaws.com${suffix}`,
  );
  assert.equal(r.status, 403);
}
const redirect = await get(web.siteUrl.replace("https:", "http:") + "/");
assert.equal(redirect.status, 301);
assert.equal(redirect.headers.get("location"), web.siteUrl + "/");
for (const origin of [web.siteUrl, out.frontendOrigin]) {
  for (const method of ["GET", "POST", "PUT", "PATCH", "DELETE"]) {
    const r = await get(out.apiUrl + "/api/v1/projects", {
      method: "OPTIONS",
      headers: {
        Origin: origin,
        "Access-Control-Request-Method": method,
        "Access-Control-Request-Headers":
          "authorization,content-type,if-match,idempotency-key",
      },
    });
    assert.equal(r.status, 204);
    assert.equal(r.headers.get("access-control-allow-origin"), origin);
    assert.equal(
      r.headers.get("access-control-allow-methods"),
      "GET, POST, PUT, PATCH, DELETE",
    );
    assert.equal(
      r.headers.get("access-control-allow-headers"),
      "Authorization, Content-Type, If-Match, Idempotency-Key",
    );
    assert.equal(
      r.headers.get("access-control-expose-headers"),
      "ETag, Location, Retry-After",
    );
    assert.equal(r.headers.get("access-control-allow-credentials"), null);
    assert.match(r.headers.get("cache-control"), /no-store/);
  }
  const r = await get(out.apiUrl + "/api/v1/projects", {
    headers: { Origin: origin },
  });
  assert.equal(r.status, 401);
  assert.equal(r.headers.get("access-control-allow-origin"), origin);
  assert.equal(
    r.headers.get("access-control-expose-headers"),
    "ETag, Location, Retry-After",
  );
  assert.match(r.headers.get("cache-control"), /no-store/);
  const denied = await get(out.apiUrl + "/api/v1/projects", {
    method: "OPTIONS",
    headers: {
      Origin: origin,
      "Access-Control-Request-Method": "GET",
      "Access-Control-Request-Headers": "x-not-allowed",
    },
  });
  assert.equal(denied.status, 403);
}
for (const origin of [
  "https://example.invalid",
  web.siteUrl + ".evil.invalid",
  "null",
  out.frontendOrigin.replace("localhost", "127.0.0.1"),
]) {
  const r = await get(out.apiUrl + "/api/v1/projects", {
    method: "OPTIONS",
    headers: { Origin: origin, "Access-Control-Request-Method": "GET" },
  });
  assert.equal(r.status, 403, origin);
  assert.equal(r.headers.get("access-control-allow-origin"), null);
}
const cli = await get(out.apiUrl + "/api/v1/projects");
assert.equal(cli.status, 401);
assert.match(cli.headers.get("cache-control"), /no-store/);
const report = {
  checkedAt: new Date().toISOString(),
  siteUrl: web.siteUrl,
  releaseId: expected.releaseId,
  genuineAwsHttp: true,
  privateS3Denied: true,
  signedCdnAssetAccess: true,
  exactHostedAndLocalCors: true,
  foreignOriginsDenied: true,
  securityHeadersAndCsp: true,
  spaDeepLinks: true,
  missingFilesAndApi404: true,
  httpRedirect: true,
  appFiles: checks,
  authenticatedBrowserValidated: false,
  disposableApplicationDataCreated: false,
};
writeFileSync(
  ".sst/frontend-hosting-http-validation.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log(JSON.stringify(report));
