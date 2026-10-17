import { createHash } from "node:crypto";
import {
  buildFrontendArtifact,
  assetCache,
  htmlCache,
} from "./frontend-artifact.ts";

export function hostingHeaders(apiOrigin: string) {
  return {
    contentSecurityPolicy: {
      override: true,
      contentSecurityPolicy: `default-src 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src ${apiOrigin} https://cognito-idp.us-east-2.amazonaws.com; form-action 'self'; manifest-src 'self'; worker-src 'none'; upgrade-insecure-requests`,
    },
    contentTypeOptions: { override: true },
    frameOptions: { override: true, frameOption: "DENY" },
    referrerPolicy: { override: true, referrerPolicy: "no-referrer" },
    strictTransportSecurity: {
      override: true,
      accessControlMaxAgeSec: 31536000,
      includeSubdomains: false,
      preload: false,
    },
  };
}

// No API proxy and no secret links. Only this existing API's public URL is needed.
export function createDevelopmentWeb(
  apiUrl: $util.Input<string>,
  pool: string,
  client: string,
) {
  return $output(apiUrl).apply((apiOrigin) => {
    // Match the pinned StaticSite namespace; no additional KV store/resource.
    const kvNamespace = createHash("md5")
      .update(`${$app.name}-${$app.stage}-FrontendWeb`)
      .digest("hex")
      .slice(0, 4);
    const artifact = buildFrontendArtifact({
      VITE_API_URL: apiOrigin,
      VITE_COGNITO_USER_POOL_ID: pool,
      VITE_COGNITO_CLIENT_ID: client,
      VITE_KURIER_STAGE: "dev-api",
    });
    const bucket = new aws.s3.BucketV2(
      "FrontendWebAssets",
      { bucketPrefix: "kurier-dev-api-web-", forceDestroy: false },
      { retainOnDelete: true, protect: true },
    );
    const block = new aws.s3.BucketPublicAccessBlock(
      "FrontendWebPublicAccess",
      {
        bucket: bucket.id,
        blockPublicAcls: true,
        blockPublicPolicy: true,
        ignorePublicAcls: true,
        restrictPublicBuckets: true,
      },
    );
    const ownership = new aws.s3.BucketOwnershipControls(
      "FrontendWebOwnership",
      { bucket: bucket.id, rule: { objectOwnership: "BucketOwnerEnforced" } },
    );
    const encryption = new aws.s3.BucketServerSideEncryptionConfigurationV2(
      "FrontendWebEncryption",
      {
        bucket: bucket.id,
        rules: [
          { applyServerSideEncryptionByDefault: { sseAlgorithm: "AES256" } },
        ],
      },
    );
    const cache = new aws.cloudfront.CachePolicy("FrontendWebCache", {
      minTtl: 0,
      defaultTtl: 0,
      maxTtl: 31536000,
      parametersInCacheKeyAndForwardedToOrigin: {
        enableAcceptEncodingBrotli: true,
        enableAcceptEncodingGzip: true,
        cookiesConfig: { cookieBehavior: "none" },
        headersConfig: { headerBehavior: "none" },
        queryStringsConfig: { queryStringBehavior: "none" },
      },
    });
    const headers = new aws.cloudfront.ResponseHeadersPolicy(
      "FrontendWebHeaders",
      {
        securityHeadersConfig: hostingHeaders(apiOrigin),
        customHeadersConfig: {
          items: [
            {
              header: "Permissions-Policy",
              value: "camera=(), microphone=(), geolocation=()",
              override: true,
            },
          ],
        },
      },
    );
    const site = new sst.aws.StaticSite(
      "FrontendWeb",
      {
        path: artifact.directory,
        dev: false,
        assets: {
          bucket: bucket.bucket,
          purge: false,
          fileOptions: [
            { files: "**", cacheControl: htmlCache },
            { files: "assets/**", cacheControl: assetCache },
          ],
        },
        invalidation: { paths: "all", wait: true },
        edge: {
          viewerRequest: {
            injection: `
// No protected API, archives, credentials or arbitrary files at this origin.
var missingStatic = { statusCode: 404, statusDescription: "Not Found", headers: { "content-security-policy": { value: "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'" }, "permissions-policy": { value: "camera=(), microphone=(), geolocation=()" }, "cache-control": { value: "no-store" }, "content-type": { value: "text/plain; charset=utf-8" }, "x-content-type-options": { value: "nosniff" }, "x-frame-options": { value: "DENY" }, "referrer-policy": { value: "no-referrer" }, "strict-transport-security": { value: "max-age=31536000" } }, body: "Not found." };
var uri;
try { uri = decodeURIComponent(event.request.uri); } catch (e) { uri = ""; }
if (!uri || uri.indexOf("..") !== -1 || uri.indexOf("\\\\") !== -1 || uri.indexOf("//") !== -1 ||
    uri === "/api" || uri.indexOf("/api/") === 0 || uri.indexOf("/_releases") === 0 ||
    (uri.split("/").pop().indexOf(".") !== -1 && uri !== "/404.html" && uri !== "/index.html" && uri !== "/startup.js" && uri !== "/artifact.json" && !/^\\/assets\\/[A-Za-z0-9_-]+-[A-Za-z0-9_-]{8,}\\.(js|css)$/.test(uri))) {
  return missingStatic;
}
// SST uses a placeholder default origin, which cannot serve custom error pages.
// Admit hashed requests only when a retained per-file KV entry exists.
if (uri.indexOf("/assets/") === 0) {
  try { await cf.kvs().get("${kvNamespace}:" + uri); }
  catch (e) { return missingStatic; }
}
delete event.request.headers.authorization;
delete event.request.headers.cookie;
event.request.cookies = {};
`,
          },
        },
        transform: {
          cdn(args) {
            // StaticSite 4.17.1 supplies a plain object here. Keep known fields
            // visible during preview even while generated policy IDs are unknown.
            args.defaultCacheBehavior = {
              ...(args.defaultCacheBehavior as aws.types.input.cloudfront.DistributionDefaultCacheBehavior),
              allowedMethods: ["GET", "HEAD"],
              cachedMethods: ["GET", "HEAD"],
              cachePolicyId: cache.id,
              responseHeadersPolicyId: headers.id,
            };
            args.customErrorResponses = [
              {
                errorCode: 403,
                responseCode: 404,
                responsePagePath: "/404.html",
                errorCachingMinTtl: 0,
              },
              { errorCode: 404, errorCachingMinTtl: 0 },
            ];
            args.transform = {
              distribution(distribution) {
                distribution.priceClass = "PriceClass_100";
                distribution.waitForDeployment = true;
              },
            };
          },
        },
      },
      {
        dependsOn: [block, ownership, encryption],
        transformations: [
          (resource) => {
            // SST's uploader deletes its objects on Delete even with purge:false.
            if (
              resource.type === "pulumi-nodejs:dynamic:Resource" &&
              resource.name === "FrontendWebAssetFiles.sst.aws.BucketFiles"
            )
              return {
                props: resource.props,
                opts: { ...resource.opts, retainOnDelete: true },
              };
            if (
              resource.type === "pulumi-nodejs:dynamic:Resource" &&
              resource.name === "FrontendWebKvKeys.sst.aws.KvKeys"
            )
              return {
                props: {
                  ...resource.props,
                  entries: $output(resource.props.entries).apply((entries) => ({
                    ...entries,
                    ...Object.fromEntries(
                      artifact.manifest.files
                        .filter((f) => f.path.startsWith("assets/"))
                        .map((f) => ["/" + f.path, "s3"]),
                    ),
                  })),
                },
                opts: resource.opts,
              };
            return undefined;
          },
        ],
      },
    );
    const distribution = site.nodes.cdn!.nodes.distribution;
    new aws.s3.BucketPolicy(
      "FrontendWebPolicy",
      {
        bucket: bucket.id,
        policy: $jsonStringify({
          Version: "2012-10-17",
          Statement: [
            {
              Effect: "Allow",
              Principal: { Service: "cloudfront.amazonaws.com" },
              Action: "s3:GetObject",
              Resource: $interpolate`${bucket.arn}/*`,
              Condition: {
                StringEquals: { "AWS:SourceArn": distribution.arn },
              },
            },
            {
              Effect: "Deny",
              Principal: "*",
              Action: "s3:*",
              Resource: [bucket.arn, $interpolate`${bucket.arn}/*`],
              Condition: { Bool: { "aws:SecureTransport": "false" } },
            },
          ],
        }),
      },
      { dependsOn: [block] },
    );
    return {
      siteUrl: site.url!,
      webBucket: bucket.bucket,
      distributionId: distribution.id,
      releaseId: artifact.manifest.releaseId,
      publicArtifact: artifact.manifest,
    };
  });
}
