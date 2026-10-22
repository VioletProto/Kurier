// Read-only review tooling; no deploy/refresh/activation mode.
// See docs/development/workflow-assertions-deployment-review.md before use.
import { spawnSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";

const args = process.argv.slice(2);
const preview = args[0] === "--diff";
if (preview) args.shift();
let source = "55cd061c0f6607037ebd61e200fe54ab296237c4";
const pinned =
  args.length === 2 && args[0] === "--source" && /^[a-f0-9]{40}$/.test(args[1]);
if (pinned) source = args[1];
else if (args.length)
  throw Error("Choose inventory/--diff, optionally --source FULL_COMMIT.");
if (process.env.KURIER_FRONTEND_ROLLBACK_RELEASE)
  throw Error("Rollback overrides are outside this review.");
const root = new URL("../", import.meta.url);
function run(command, args) {
  const r = spawnSync(command, args, {
    cwd: root,
    encoding: "utf8",
    maxBuffer: 32 * 1024 * 1024,
  });
  if (r.status !== 0)
    throw Error(`Review command failed: ${command} ${args[0]}`);
  return r.stdout;
}
const hash = (value) =>
  createHash("sha256").update(JSON.stringify(value)).digest("hex");
function aws(args) {
  return JSON.parse(
    run("aws", [...args, "--region", "us-east-2", "--output", "json"]),
  );
}
if (
  pinned &&
  (run("git", ["rev-parse", "HEAD"]).trim() !== source ||
    run("git", ["status", "--porcelain", "--untracked-files=all"]).trim())
)
  throw Error("Explicit release source requires its exact clean checkout.");
const diff = run("git", [
  "diff",
  source,
  "--",
  "services",
  "apps",
  "infra",
  "sst.config.ts",
  "sst.config.d.ts",
  "package.json",
  "package-lock.json",
  "scripts/preview-frontend-hosting.mjs",
]);
if (diff)
  throw Error(
    "Application/infrastructure source differs from reviewed commit.",
  );
if (
  run("git", [
    "ls-files",
    "--others",
    "--exclude-standard",
    "services",
    "apps",
    "infra",
  ]).trim()
)
  throw Error("Untracked application/infrastructure source requires review.");
const identity = aws(["sts", "get-caller-identity"]);
if (identity.Account !== "747336059622" || identity.Arn.endsWith(":root"))
  throw Error("Intended non-root account required.");
function checkpoint() {
  const passphrase = aws([
    "ssm",
    "describe-parameters",
    "--parameter-filters",
    "Key=Name,Option=Equals,Values=/sst/passphrase/kurier/dev-api",
  ]);
  if (passphrase.Parameters?.length !== 1 || passphrase.NextToken)
    throw Error("Existing passphrase metadata required.");
  const state = aws([
    "s3api",
    "list-objects-v2",
    "--bucket",
    "sst-state-moshrdrrbeba",
    "--prefix",
    "",
    "--max-items",
    "10000",
  ]);
  if (state.NextToken || state.IsTruncated)
    throw Error("Incomplete state inventory.");
  if (!state.Contents?.some((x) => x.Key === "app/kurier/dev-api.json"))
    throw Error("Existing exact stage state required.");
  // Publish hashes only; state contents and passphrase values are never exported.
  return {
    passphraseMetadataSha256: hash(passphrase.Parameters),
    stateBucketMetadataSha256: hash(state.Contents),
    stateObjects: state.Contents.length,
  };
}
const before = checkpoint();
// This pinned, reviewed helper checks bootstrap v5, account/plan, active v13
// savedRequests7 marker/generation, IAM, consumer packages and accounting.
const inventory = JSON.parse(
  run(process.execPath, [
    "scripts/preview-frontend-hosting.mjs",
    ...(preview ? ["--diff"] : []),
  ]),
);
const after = checkpoint();
if (JSON.stringify(before) !== JSON.stringify(after))
  throw Error("SST state/passphrase metadata changed; stop and investigate.");
if (preview)
  writeFileSync(
    new URL(".sst/workflow-assertions-diff.txt", root),
    readFileSync(new URL(".sst/frontend-hosting-diff.txt", root)),
  );
const report = {
  sourceCommit: source,
  ...inventory,
  toolingMetadata: before,
  toolingMetadataUnchanged: true,
  deploymentPerformed: false,
  activationPerformed: false,
  targetRequestsPerformed: false,
};
writeFileSync(
  new URL(".sst/workflow-assertions-inventory.json", root),
  JSON.stringify(report, null, 2) + "\n",
);
console.log(JSON.stringify(report, null, 2));
