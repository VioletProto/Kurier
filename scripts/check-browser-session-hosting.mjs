// Read-only post-deployment check through CloudFront. Never accepts credentials.
import assert from "node:assert/strict";
import { request } from "node:https";

const site = new URL(process.argv[2] ?? "https://invalid.invalid");
if (
  site.protocol !== "https:" ||
  !/^[a-z0-9]+\.cloudfront\.net$/.test(site.hostname) ||
  site.origin !== process.argv[2]
)
  throw new Error("Supply the exact HTTPS CloudFront origin without a path.");

const cases = [
  {
    name: "missing session",
    headers: { Origin: site.origin, "X-Kurier-Session": "1" },
    status: 401,
  },
  {
    name: "foreign origin",
    headers: {
      Origin: "https://foreign.invalid",
      "X-Kurier-Session": "1",
    },
    status: 403,
  },
  {
    name: "missing custom header",
    headers: { Origin: site.origin },
    status: 403,
  },
  {
    name: "navigation denied",
    headers: {
      Origin: site.origin,
      "X-Kurier-Session": "1",
      "Sec-Fetch-Mode": "navigate",
      "Sec-Fetch-Site": "same-origin",
    },
    status: 403,
  },
];
function readResponse(headers) {
  return new Promise((resolve, reject) => {
    // Raw HTTPS preserves the deliberately hostile Fetch Metadata headers;
    // fetch implementations can replace Sec-Fetch-Mode with their own value.
    const req = request(
      site.origin + "/api/v1/browser-session",
      { headers },
      (res) => {
        const chunks = [];
        let size = 0;
        res.on("data", (chunk) => {
          size += chunk.length;
          if (size > 32768) {
            res.destroy();
            reject(Error("Unexpected oversized session error response."));
            return;
          }
          chunks.push(chunk);
        });
        res.on("error", () =>
          reject(Error("Session check response unavailable.")),
        );
        res.on("end", () =>
          resolve({
            status: res.statusCode,
            headers: res.headers,
            body: Buffer.concat(chunks).toString("utf8"),
          }),
        );
      },
    );
    req.setTimeout(15000, () => req.destroy(Error("Session check timed out.")));
    req.on("error", () => reject(Error("Session check request unavailable.")));
    req.end();
  });
}
for (const scenario of cases) {
  const response = await readResponse(scenario.headers);
  assert.equal(response.status, scenario.status, scenario.name);
  assert.match(
    response.headers["content-type"] ?? "",
    /^application\/json\b/,
    scenario.name,
  );
  assert.match(response.headers["cache-control"] ?? "", /no-store/);
  assert.notEqual(response.headers["x-cache"], "Hit from cloudfront");
  const body = JSON.parse(response.body);
  assert.equal(typeof body.error?.code, "string", scenario.name);
  // Print only a fixed scenario name and pass status, never response contents.
  console.log(`${scenario.name}: PASS`);
}
