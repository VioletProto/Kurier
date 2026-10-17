import { mkdtempSync, mkdirSync, writeFileSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import vm from "node:vm";
import { afterEach, expect, it, vi } from "vitest";
import {
  inspectArtifact,
  planFrontendRetention,
  sha256,
} from "./frontend-artifact";
import { hostedPublicConfig } from "../apps/web/public-config";
import { createDevelopmentWeb } from "./frontend-hosting";

const config = {
  VITE_API_URL: "https://abc.execute-api.us-east-2.amazonaws.com",
  VITE_COGNITO_USER_POOL_ID: "us-east-2_Example",
  VITE_COGNITO_CLIENT_ID: "publicclient",
  VITE_KURIER_STAGE: "dev-api",
};
const build = vi.hoisted(() => vi.fn());
vi.mock("./frontend-artifact", async () => ({
  ...(await vi.importActual("./frontend-artifact")),
  buildFrontendArtifact: build,
}));
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

it("rejects missing, foreign-region, credential-bearing and extra public build values", () => {
  expect(hostedPublicConfig(config)).toEqual(config);
  for (const extra of [
    { VITE_API_URL: "http://localhost:8080" },
    {
      VITE_API_URL:
        "https://user:password@abc.execute-api.us-east-2.amazonaws.com",
    },
    { VITE_COGNITO_USER_POOL_ID: "us-east-1_wrong" },
    { VITE_PASSWORD: "never-public" },
    { VITE_API_URL: undefined },
    { VITE_KURIER_STAGE: "local" },
  ])
    expect(() => hostedPublicConfig({ ...config, ...extra })).toThrow(
      "configuration",
    );
});

it("content-addresses only bounded public files and rejects maps, bypasses and symlinks", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "kurier-public-artifact-"));
  mkdirSync(path.join(dir, "assets"));
  writeFileSync(
    path.join(dir, "index.html"),
    '<script src="/startup.js"></script><script src="/assets/app-abcdefgh.js"></script>',
  );
  writeFileSync(path.join(dir, "startup.js"), "// public startup");
  writeFileSync(path.join(dir, "404.html"), "Missing");
  writeFileSync(path.join(dir, "assets/app-abcdefgh.js"), "// application");
  writeFileSync(path.join(dir, "assets/app-abcdefgh.css"), "body{}");
  const a = inspectArtifact(dir, config);
  expect(a.files.find((f) => f.path === "index.html")!.cacheControl).toContain(
    "no-store",
  );
  expect(a.files.find((f) => f.path.endsWith(".css"))!.cacheControl).toContain(
    "immutable",
  );
  writeFileSync(path.join(dir, "assets/app-abcdefgh.js"), "// changed");
  expect(inspectArtifact(dir, config).releaseId).not.toBe(a.releaseId);
  writeFileSync(
    path.join(dir, "assets/app-abcdefgh.js"),
    "KURIER_BROWSER_FIXTURE_TOKEN",
  );
  expect(() => inspectArtifact(dir, config)).toThrow("Unsafe");
  writeFileSync(path.join(dir, "assets/app-abcdefgh.js"), "// restored");
  symlinkSync("app-abcdefgh.js", path.join(dir, "assets/linked-abcdefgh.js"));
  expect(() => inspectArtifact(dir, config)).toThrow("symlinks");
});

it("keeps static origins separate, honors no-store, signs scoped S3 reads, retains uploads and rejects API/file navigation", async () => {
  const resources: any[] = [];
  const output = (value: any): any => ({ apply: (f: any) => f(value) });
  class Resource {
    id: string;
    arn: string;
    bucket: string;
    constructor(name: string, args: any, opts?: any) {
      this.id = name;
      this.arn = `arn:${name}`;
      this.bucket = name;
      resources.push({ name, args, opts });
    }
  }
  let staticArgs: any;
  let staticOpts: any;
  let cdn: any;
  class Site {
    url = "https://dexample.cloudfront.net";
    nodes = {
      cdn: {
        nodes: {
          distribution: { id: "distribution", arn: "arn:distribution" },
        },
      },
    };
    constructor(_name: string, args: any, opts: any) {
      staticArgs = args;
      staticOpts = opts;
      cdn = { defaultCacheBehavior: { targetOriginId: "default" } };
      args.transform.cdn(cdn);
    }
  }
  vi.stubGlobal("$app", { name: "kurier", stage: "dev-api" });
  vi.stubGlobal("$output", output);
  vi.stubGlobal("$jsonStringify", JSON.stringify);
  vi.stubGlobal(
    "$interpolate",
    (strings: TemplateStringsArray, ...values: unknown[]) =>
      strings.reduce((s, v, i) => s + v + (values[i] ?? ""), ""),
  );
  vi.stubGlobal("aws", {
    s3: {
      BucketV2: Resource,
      BucketPublicAccessBlock: Resource,
      BucketOwnershipControls: Resource,
      BucketServerSideEncryptionConfigurationV2: Resource,
      BucketPolicy: Resource,
    },
    cloudfront: { CachePolicy: Resource, ResponseHeadersPolicy: Resource },
  });
  vi.stubGlobal("sst", { aws: { StaticSite: Site } });
  build.mockReturnValue({
    directory: "/public-artifact",
    manifest: {
      releaseId: "a".repeat(64),
      publicConfig: config,
      files: [{ path: "assets/app-abcdefgh.js" }, { path: "index.html" }],
    },
  });
  const result = createDevelopmentWeb(
    config.VITE_API_URL,
    config.VITE_COGNITO_USER_POOL_ID,
    config.VITE_COGNITO_CLIENT_ID,
  );
  expect(result).toMatchObject({ siteUrl: "https://dexample.cloudfront.net" });
  expect(staticArgs.assets.purge).toBe(false);
  expect(staticArgs.invalidation).toEqual({ paths: "all", wait: true });
  expect(cdn.defaultCacheBehavior).toMatchObject({
    allowedMethods: ["GET", "HEAD"],
    responseHeadersPolicyId: "FrontendWebHeaders",
    cachePolicyId: "FrontendWebCache",
  });
  expect(
    resources.find((r) => r.name === "FrontendWebCache").args,
  ).toMatchObject({ minTtl: 0, defaultTtl: 0 });
  const policy = JSON.parse(
    resources.find((r) => r.name === "FrontendWebPolicy").args.policy,
  );
  expect(policy.Statement[0].Condition.StringEquals["AWS:SourceArn"]).toBe(
    "arn:distribution",
  );
  expect(
    resources.find((r) => r.name === "FrontendWebAssets").opts,
  ).toMatchObject({ retainOnDelete: true, protect: true });
  const retained = staticOpts.transformations[0]({
    type: "pulumi-nodejs:dynamic:Resource",
    name: "FrontendWebAssetFiles.sst.aws.BucketFiles",
    props: {},
    opts: {},
  });
  expect(retained.opts.retainOnDelete).toBe(true);
  const keys = staticOpts.transformations[0]({
    type: "pulumi-nodejs:dynamic:Resource",
    name: "FrontendWebKvKeys.sst.aws.KvKeys",
    props: {
      entries: { metadata: "existing", "/assets/old-abcdefgh.js": "s3" },
      purge: false,
    },
    opts: {},
  });
  expect(keys.props.entries).toEqual({
    metadata: "existing",
    "/assets/old-abcdefgh.js": "s3",
    "/assets/app-abcdefgh.js": "s3",
  });
  expect(keys.props.purge).toBe(false);
  const run = async (uri: string) => {
    const event = {
      request: {
        uri,
        headers: {
          authorization: { value: "sensitive" },
          cookie: { value: "sensitive" },
        },
        cookies: {},
      },
    };
    const response = await vm.runInNewContext(
      `(async function(){${staticArgs.edge.viewerRequest.injection}})()`,
      {
        event,
        cf: {
          kvs: () => ({
            get: async (key: string) => {
              if (key.endsWith(":/assets/app-abcdefgh.js")) return "s3";
              throw Error("Missing");
            },
          }),
        },
      },
    );
    return { response, event };
  };
  for (const uri of [
    "/api/v1/projects",
    "/_releases/release/index.html",
    "/_releases/release/startup.js",
    "/_releases/release/404.html",
    "/_releases/release/artifact.json",
    "/%5freleases/release/startup.js",
    "/missing.js",
    "/assets/missing-abcdefgh.js",
    "/%2e%2e/index.html",
    "/bad%",
    "/foo//bar",
  ])
    expect((await run(uri)).response.statusCode).toBe(404);
  for (const uri of [
    "/",
    "/project/opaque",
    "/startup.js",
    "/assets/app-abcdefgh.js",
  ]) {
    expect((await run(uri)).response).toBeUndefined();
    expect((await run(uri)).event.request.headers).toEqual({});
  }
});

it("retains the 30-day window and both known-good releases including shared chunks", () => {
  const now = Date.parse("2026-10-16T00:00:00Z");
  const release = (asset: string, uploadedAt: string) => {
    const payload = {
      schemaVersion: 1,
      publicConfig: config,
      files: [
        {
          path: asset,
          bytes: 1,
          sha256: "a".repeat(64),
          cacheControl: "immutable",
        },
      ],
    };
    return {
      manifest: { ...payload, releaseId: sha256(JSON.stringify(payload)) },
      uploadedAt,
    };
  };
  const current = release("assets/current-abcdefgh.js", "2026-08-01T00:00:00Z");
  const previous = release(
    "assets/previous-abcdefgh.js",
    "2026-08-02T00:00:00Z",
  );
  const recent = release("assets/recent-abcdefgh.js", "2026-10-01T00:00:00Z");
  const expired = release("assets/expired-abcdefgh.js", "2026-08-03T00:00:00Z");
  const plan = planFrontendRetention(
    [current, previous, recent, expired],
    current.manifest.releaseId,
    previous.manifest.releaseId,
    now,
  );
  expect(plan).toContain("assets/expired-abcdefgh.js");
  expect(plan).not.toContain("assets/current-abcdefgh.js");
  expect(plan).not.toContain("assets/previous-abcdefgh.js");
  expect(plan).not.toContain("assets/recent-abcdefgh.js");
  expect(() =>
    planFrontendRetention(
      [expired],
      current.manifest.releaseId,
      previous.manifest.releaseId,
      now,
    ),
  ).toThrow("known-good");
});
