// Publish only explicitly selected public resource/configuration evidence.
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
const raw = readFileSync(".sst/frontend-hosting-diff.txt", "utf8");
// SST emits an array followed by npm diagnostics. Never export raw state/inputs.
let depth = 0,
  quoted = false,
  escaped = false,
  end = -1;
for (let i = 0; i < raw.length; i++) {
  const c = raw[i];
  if (quoted) {
    if (escaped) escaped = false;
    else if (c === "\\") escaped = true;
    else if (c === '"') quoted = false;
    continue;
  }
  if (c === '"') quoted = true;
  if (c === "[") depth++;
  if (c === "]" && --depth === 0) {
    end = i + 1;
    break;
  }
}
if (end < 0) throw Error("Expected complete SST JSON diff.");
const events = JSON.parse(raw.slice(0, end));
const changes = new Map();
for (const e of events) {
  if (e.op === "read" || e.op === "delete-replaced") continue;
  const op = ["replace", "create-replacement"].includes(e.op)
    ? "replace"
    : e.op;
  if (!["create", "replace", "update"].includes(op))
    throw Error("Unexpected preview operation; inspect before publishing.");
  changes.set(e.urn, {
    name: e.urn.split("::").pop(),
    type: e.type,
    operation: op,
    changedProperties: Object.keys(e.detailedDiff ?? {}).sort(),
  });
}
if ([...changes.values()].some((c) => c.type.startsWith("aws:iam/")))
  throw Error("Unexpected runtime IAM change.");
const find = (name) =>
  events.find((e) => e.op === "create" && e.urn.split("::").pop() === name)?.new
    ?.inputs;
const edge = find("FrontendWebCloudfrontFunctionRequest");
const distribution = find("FrontendWebCdnDistribution");
const policies = {
  cache: find("FrontendWebCache"),
  headers: find("FrontendWebHeaders"),
};
for (const value of Object.values(policies)) delete value.__defaults;
const report = {
  inventory: JSON.parse(
    readFileSync(".sst/frontend-hosting-inventory.json", "utf8"),
  ),
  resources: [...changes.values()],
  runtimeIamChanges: false,
  newBucketPolicy: {
    cloudFrontGetObject:
      "web-bucket/* only, AWS:SourceArn == new distribution ARN",
    insecureTransportDenied: true,
    previewPolicyString: "unknown until distribution ARN resolves",
  },
  policies,
  distribution: {
    defaultCacheBehavior: distribution.defaultCacheBehavior,
    customErrorResponses: distribution.customErrorResponses,
    priceClass: distribution.priceClass,
    viewerCertificate: distribution.viewerCertificate,
    waitForDeployment: distribution.waitForDeployment,
    originSelection:
      "SST viewer-request function uses signed S3 REST origin; no API behavior",
    edgeCodeBytes: Buffer.byteLength(edge.code),
    edgeCodeSha256: createHash("sha256").update(edge.code).digest("hex"),
  },
  previewUnknowns: [
    "Generated bucket/distribution hostname and IDs; exact new bucket policy ARN",
    "StaticSite AssetFiles and Invalidation providers are registered inside applies after the new bucket read resolves; absent from the first-create preview",
    "Exact API environment is secret/unknown in preview; source adds only KURIER_HOSTED_FRONTEND_ORIGIN",
  ],
  deferredProviders: {
    AssetFiles: {
      purge: false,
      retainOnDelete: true,
      ordering: "SST 4.17.1 uploads non-HTML then HTML",
    },
    Invalidation: {
      paths: ["/*"],
      wait: true,
      dependencies: ["AssetFiles", "KvKeys"],
    },
  },
  deploymentAuthorized: false,
};
function publicValue(value) {
  if (value === "04da6b54-80e4-46f7-96ec-b56ff0331ba9") return "[unknown]";
  if (Array.isArray(value)) return value.map(publicValue);
  if (value && typeof value === "object")
    return Object.fromEntries(
      Object.entries(value)
        .filter(([key]) => key !== "__defaults")
        .map(([key, v]) => [key, publicValue(v)]),
    );
  return value;
}
writeFileSync(
  "docs/development/frontend-hosting-preview.json",
  JSON.stringify(publicValue(report), null, 2) + "\n",
);
writeFileSync(
  "docs/development/frontend-hosting-artifact.json",
  readFileSync(".sst/frontend-hosting-artifact.json"),
);
console.log(
  JSON.stringify({
    inspectedResourceChanges: report.resources.length,
    runtimeIamChanges: false,
    publishedOnlyPublicEvidence: true,
    deploymentPerformed: false,
  }),
);
