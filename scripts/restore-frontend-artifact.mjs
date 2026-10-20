// Local recovery only: S3 GETs, no uploads, builds, deployment or invalidation.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { restoreFrontendArtifact } from "../infra/frontend-artifact.ts";
const [releaseId] = process.argv.slice(2);
if (process.argv.length !== 3 || !/^[a-f0-9]{64}$/.test(releaseId ?? ""))
  throw Error("Provide the operator-verified historical release ID.");
const out = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
const bucket = out.frontendHosting?.webBucket;
if (
  out.stage !== "dev-api" ||
  out.region !== "us-east-2" ||
  !/^kurier-dev-api-web-[a-z0-9-]+$/.test(bucket ?? "")
)
  throw Error("Deployed scoped web outputs required.");
function aws(args) {
  const r = spawnSync("aws", [...args, "--region", "us-east-2"], {
    maxBuffer: 8 * 1024 * 1024,
  });
  if (r.status !== 0)
    throw Error("Read-only historical object recovery failed.");
  return r.stdout;
}
const identity = JSON.parse(
  aws(["sts", "get-caller-identity", "--output", "json"]).toString(),
);
if (identity.Account !== "747336059622" || identity.Arn.endsWith(":root"))
  throw Error("Intended non-root account required.");
const manifest = restoreFrontendArtifact(
  releaseId,
  {
    VITE_API_URL: out.apiUrl,
    VITE_COGNITO_USER_POOL_ID: out.cognitoUserPoolId,
    VITE_COGNITO_CLIENT_ID: out.cognitoClientId,
    VITE_KURIER_STAGE: "dev-api",
  },
  (key) => aws(["s3", "cp", `s3://${bucket}/${key}`, "-", "--no-progress"]),
);
console.log(
  JSON.stringify({
    releaseId: manifest.releaseId,
    verifiedFiles: manifest.files.length,
    rebuilt: false,
    deploymentPerformed: false,
  }),
);
