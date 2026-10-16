import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { planFrontendRetention } from "../infra/frontend-artifact.ts";

const [current, previous] = process.argv.slice(2);
if (process.argv.length !== 4)
  throw Error(
    "Provide current and previous operator-verified known-good release IDs. This command only plans.",
  );
const out = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
const bucket = out.frontendHosting?.webBucket;
if (
  out.stage !== "dev-api" ||
  out.region !== "us-east-2" ||
  !/^kurier-dev-api-web-[a-z0-9-]+$/.test(bucket ?? "")
)
  throw Error("Deployed scoped web outputs required.");
function aws(args, json = true) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", ...(json ? ["--output", "json"] : [])],
    { encoding: "utf8", maxBuffer: 8 * 1024 * 1024 },
  );
  if (r.status !== 0) throw Error("Read-only web release inventory failed.");
  return json ? JSON.parse(r.stdout) : r.stdout;
}
const identity = aws(["sts", "get-caller-identity"]);
if (identity.Account !== "747336059622" || identity.Arn.endsWith(":root"))
  throw Error("Intended non-root account required.");
const objects = aws([
  "s3api",
  "list-objects-v2",
  "--bucket",
  bucket,
  "--prefix",
  "_releases/",
  "--max-keys",
  "1000",
  "--no-paginate",
]);
if (objects.IsTruncated)
  throw Error("Release inventory is incomplete; preserve everything.");
const releases = (objects.Contents ?? [])
  .filter((o) => /^_releases\/[a-f0-9]{64}\/artifact.json$/.test(o.Key))
  .map((o) => {
    const manifest = JSON.parse(
      aws(["s3", "cp", `s3://${bucket}/${o.Key}`, "-", "--no-progress"], false),
    );
    if (o.Key !== `_releases/${manifest.releaseId}/artifact.json`)
      throw Error("Archive identity mismatch.");
    return { manifest, uploadedAt: o.LastModified };
  });
console.log(
  JSON.stringify(
    {
      bucket,
      current,
      previous,
      deleteCandidates: planFrontendRetention(releases, current, previous),
      deletionPerformed: false,
    },
    null,
    2,
  ),
);
