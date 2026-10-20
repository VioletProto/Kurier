// Read-only generated-resource and deferred-provider readback.
import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import {
  aws,
  identity,
  outputs as out,
} from "./verify-read-only-mcp-deployment.mjs";
import { sha256 } from "../infra/frontend-artifact.ts";
import { hostingHeaders } from "../infra/frontend-hosting.ts";
identity();
const web = out.frontendHosting;
if (!web?.distributionId) throw Error("Deployed web outputs required.");
const state = JSON.parse(
  readFileSync(".sst/frontend-hosting-state.json", "utf8"),
);
const resources = state.latest?.resources ?? state.deployment?.resources;
const resource = (name) => {
  const r = resources.find((r) => r.urn.split("::").at(-1) === name);
  assert.ok(r, name);
  return r;
};
const upload = resource("FrontendWebAssetFiles.sst.aws.BucketFiles");
assert.equal(upload.inputs.purge, false);
assert.equal(upload.retainOnDelete, true);
assert.equal(upload.inputs.files.length, 10);
const bucket = resources.find(
  (r) => r.type === "aws:s3/bucketV2:BucketV2" && r.id === web.webBucket,
);
assert.equal(bucket.protect, true);
assert.equal(bucket.retainOnDelete, true);
const invalidation = resource(
  "FrontendWebInvalidation.sst.aws.DistributionInvalidation",
);
assert.deepEqual(invalidation.inputs.paths, ["/*"]);
assert.equal(invalidation.inputs.wait, true);
for (const dependency of [
  "FrontendWebAssetFiles.sst.aws.BucketFiles",
  "FrontendWebKvKeys.sst.aws.KvKeys",
])
  assert.ok(
    invalidation.dependencies.some((d) => d.endsWith("::" + dependency)),
  );
assert.equal(
  resource("FrontendWebCdnWaiter.sst.aws.DistributionDeploymentWaiter").inputs
    .wait,
  true,
);
const d = aws([
  "cloudfront",
  "get-distribution",
  "--id",
  web.distributionId,
]).Distribution;
assert.equal(d.Status, "Deployed");
assert.equal(d.InProgressInvalidationBatches, 0);
assert.equal("https://" + d.DomainName, web.siteUrl);
const c = d.DistributionConfig;
assert.equal(c.PriceClass, "PriceClass_100");
assert.equal(c.ViewerCertificate.CloudFrontDefaultCertificate, true);
assert.equal(c.CacheBehaviors.Quantity, 0);
assert.equal(c.Aliases.Quantity, 0);
assert.equal(c.Logging.Enabled, false);
const b = c.DefaultCacheBehavior;
assert.deepEqual([...b.AllowedMethods.Items].sort(), ["GET", "HEAD"]);
assert.equal(b.ViewerProtocolPolicy, "redirect-to-https");
assert.equal(b.Compress, true);
const cache = aws(["cloudfront", "get-cache-policy", "--id", b.CachePolicyId])
  .CachePolicy.CachePolicyConfig;
assert.equal(cache.MinTTL, 0);
assert.equal(cache.DefaultTTL, 0);
assert.equal(cache.MaxTTL, 31536000);
const parameters = cache.ParametersInCacheKeyAndForwardedToOrigin;
assert.equal(parameters.CookiesConfig.CookieBehavior, "none");
assert.equal(parameters.HeadersConfig.HeaderBehavior, "none");
assert.equal(parameters.QueryStringsConfig.QueryStringBehavior, "none");
assert.equal(parameters.EnableAcceptEncodingGzip, true);
assert.equal(parameters.EnableAcceptEncodingBrotli, true);
const headers = aws([
  "cloudfront",
  "get-response-headers-policy",
  "--id",
  b.ResponseHeadersPolicyId,
]).ResponseHeadersPolicy.ResponseHeadersPolicyConfig;
assert.equal(
  headers.SecurityHeadersConfig.ContentSecurityPolicy.ContentSecurityPolicy,
  hostingHeaders(out.apiUrl).contentSecurityPolicy.contentSecurityPolicy,
);
const publicBlock = aws([
  "s3api",
  "get-public-access-block",
  "--bucket",
  web.webBucket,
]).PublicAccessBlockConfiguration;
assert.ok(Object.values(publicBlock).every((v) => v === true));
assert.equal(Object.keys(publicBlock).length, 4);
assert.equal(
  aws(["s3api", "get-bucket-ownership-controls", "--bucket", web.webBucket])
    .OwnershipControls.Rules[0].ObjectOwnership,
  "BucketOwnerEnforced",
);
assert.equal(
  aws(["s3api", "get-bucket-encryption", "--bucket", web.webBucket])
    .ServerSideEncryptionConfiguration.Rules[0]
    .ApplyServerSideEncryptionByDefault.SSEAlgorithm,
  "AES256",
);
const policy = JSON.parse(
  aws(["s3api", "get-bucket-policy", "--bucket", web.webBucket]).Policy,
);
assert.equal(policy.Statement.length, 2);
const grant = policy.Statement.find((s) => s.Effect === "Allow");
assert.equal(grant.Principal.Service, "cloudfront.amazonaws.com");
assert.equal(grant.Action, "s3:GetObject");
assert.equal(grant.Resource, `arn:aws:s3:::${web.webBucket}/*`);
assert.equal(grant.Condition.StringEquals["AWS:SourceArn"], d.ARN);
const deny = policy.Statement.find((s) => s.Effect === "Deny");
assert.equal(deny.Principal, "*");
assert.equal(deny.Action, "s3:*");
assert.equal(deny.Condition.Bool["aws:SecureTransport"], "false");
const fn = resource("FrontendWebCloudfrontFunctionRequest");
assert.equal(fn.inputs.publish, true);
assert.match(
  fn.inputs.code,
  /signingBehavior:"always",signingProtocol:"sigv4",originType:"s3"/,
);
assert.ok(Buffer.byteLength(fn.inputs.code) <= 10240);
const live = aws([
  "cloudfront",
  "get-function",
  "--name",
  fn.id,
  "--stage",
  "LIVE",
  ".sst/frontend-hosting-function-live.js",
]);
assert.equal(
  readFileSync(".sst/frontend-hosting-function-live.js", "utf8"),
  fn.inputs.code,
);
const kv = resource("FrontendWebKvKeys.sst.aws.KvKeys");
assert.equal(kv.inputs.purge, false);
const meta = JSON.parse(
  aws([
    "cloudfront-keyvaluestore",
    "get-key",
    "--kvs-arn",
    kv.inputs.store,
    "--key",
    kv.inputs.namespace + ":metadata",
  ]).Value,
);
assert.equal(meta.s3.domain, `${web.webBucket}.s3.us-east-2.amazonaws.com`);
assert.equal(meta.servers, undefined);
for (const f of web.publicArtifact.files.filter((f) =>
  f.path.startsWith("assets/"),
))
  assert.equal(
    aws([
      "cloudfront-keyvaluestore",
      "get-key",
      "--kvs-arn",
      kv.inputs.store,
      "--key",
      kv.inputs.namespace + ":/" + f.path,
    ]).Value,
    "s3",
  );
const inv = aws([
  "cloudfront",
  "list-invalidations",
  "--distribution-id",
  web.distributionId,
]).InvalidationList;
assert.ok(!inv.IsTruncated);
assert.ok(inv.Items?.length);
assert.ok(inv.Items.every((i) => i.Status === "Completed"));
const report = {
  checkedAt: new Date().toISOString(),
  siteUrl: web.siteUrl,
  webBucket: web.webBucket,
  distributionId: d.Id,
  distributionArn: d.ARN,
  distributionStatus: d.Status,
  cachePolicyId: b.CachePolicyId,
  headersPolicyId: b.ResponseHeadersPolicyId,
  functionName: fn.id,
  functionArn: fn.outputs.arn,
  functionCodeBytes: Buffer.byteLength(fn.inputs.code),
  functionCodeSha256: sha256(fn.inputs.code),
  keyValueStoreArn: kv.inputs.store,
  kvNamespace: kv.inputs.namespace,
  viewerCertificate: c.ViewerCertificate,
  cachePolicy: cache,
  privateBucketPolicy: policy,
  deferred: {
    uploadFiles: upload.inputs.files.length,
    purge: false,
    retainOnDelete: true,
    invalidationPaths: invalidation.inputs.paths,
    invalidationWait: true,
    invalidationDependencies: invalidation.dependencies.map((d) =>
      d.split("::").at(-1),
    ),
  },
  invalidations: inv.Items.map((i) => ({ id: i.Id, status: i.Status })),
  signedS3Origin: true,
  noApiOrigin: true,
};
writeFileSync(
  ".sst/frontend-hosting-resources.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log(JSON.stringify(report));
