// Offline-only: execute the pinned createCode function verbatim, with only its
// post-packaging AWS registration replaced. Archiver, glob and filesystem are real.
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { createRequire } from "node:module";
import net from "node:net";
import dns from "node:dns";
const forbidden = () => {
  throw Error("offline-packaging-network-held");
};
net.connect = net.createConnection = dns.lookup = forbidden;
const config = JSON.parse(fs.readFileSync(process.argv[2]));
const require = createRequire(path.join(config.platform, "package.json"));
const ts = require("typescript");
const source = fs.readFileSync(
  path.join(config.platform, "src/components/aws/function.ts"),
  "utf8",
);
if (
  crypto.createHash("sha256").update(source).digest("hex") !==
  config.sourceSha256
)
  throw Error("pinned-packager-source-held");
const parsed = ts.createSourceFile(
  "function.ts",
  source,
  ts.ScriptTarget.Latest,
  true,
);
const candidates = [];
function visit(node) {
  if (ts.isFunctionDeclaration(node) && node.name?.text === "createCode")
    candidates.push(node);
  ts.forEachChild(node, visit);
}
visit(parsed);
if (candidates.length !== 1) throw Error("pinned-packager-entrypoint-held");
const body = candidates[0].getText(parsed);
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;
const execute = new AsyncFunction(
  "fs",
  "path",
  "crypto",
  "archiver",
  "glob",
  "$cli",
  "bundle",
  "name",
  "dev",
  "regionName",
  "copyFiles",
  "sourcemaps",
  "wrapper",
  "logGroupArn",
  "region",
  "bootstrap",
  "s3",
  "asset",
  "interpolate",
  "parent",
  body + ";return await createCode();",
);
fs.mkdirSync(config.output, { recursive: true, mode: 0o700 });
const results = [];
for (const entry of config.packages) {
  const bundle = path.join(config.output, entry.name + "-src");
  fs.mkdirSync(bundle, { recursive: false, mode: 0o755 });
  fs.writeFileSync(
    path.join(bundle, "bootstrap"),
    fs.readFileSync(path.join(config.inputs, entry.name, "bootstrap")),
    { mode: 0o777 },
  );
  fs.writeFileSync(
    path.join(bundle, "resource.enc"),
    fs.readFileSync(path.join(config.inputs, entry.name, "resource.enc")),
    { mode: 0o644 },
  );
  class FileArchive {
    constructor(file) {
      this.path = file;
    }
  }
  class BucketObjectv2 {
    constructor(_name, inputs) {
      this.path = inputs.source.path;
    }
  }
  const result = await execute(
    fs,
    path,
    crypto,
    require("archiver"),
    require("glob").glob,
    { paths: { work: config.output } },
    bundle,
    entry.name,
    false,
    "us-east-2",
    [],
    [],
    undefined,
    undefined,
    { apply: (callback) => callback("us-east-2") },
    { forRegion: async () => ({ asset: "synthetic" }) },
    { BucketObjectv2 },
    { FileArchive },
    (strings, ...values) =>
      strings.reduce((r, s, i) => r + s + (values[i] ?? ""), ""),
    undefined,
  );
  results.push({
    name: entry.name,
    sha256: crypto
      .createHash("sha256")
      .update(fs.readFileSync(result.path))
      .digest("hex"),
    bootstrapExecutable: Boolean(
      fs.statSync(path.join(bundle, "bootstrap")).mode & 0o100,
    ),
    umask: process.umask(),
  });
}
console.log(JSON.stringify(results));
