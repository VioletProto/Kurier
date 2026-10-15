// Authorised diff preparation only. Never deploys or changes the capability marker.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
if (process.argv.length !== 2) throw Error("Preview only.");
const out = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
await import("./preview-sequential-workflow-activation.mjs");
await import("./verify-cloud-execution-config.mjs");
// Refuse a first bootstrap. Do not print the parameter's value.
const bootstrap = spawnSync(
  "aws",
  [
    "ssm",
    "get-parameter",
    "--name",
    "/sst/bootstrap",
    "--region",
    "us-east-2",
    "--output",
    "json",
  ],
  { encoding: "utf8" },
);
if (bootstrap.status !== 0)
  throw Error(
    "Existing SST bootstrap could not be verified; do not bootstrap implicitly.",
  );
const metadata = JSON.parse(bootstrap.stdout);
if (!metadata.Parameter?.Value) throw Error("Bootstrap metadata missing");
const result = spawnSync("node", ["scripts/sst-auth.mjs", "diff", "dev-api"], {
  cwd: new URL("../", import.meta.url),
  env: {
    ...process.env,
    KURIER_AWS_ACCOUNT_ID: "747336059622",
    KURIER_COGNITO_POOL_ID: out.cognitoUserPoolId,
    KURIER_COGNITO_CLIENT_ID: out.cognitoClientId,
    KURIER_FRONTEND_ORIGIN: out.frontendOrigin,
  },
  stdio: "inherit",
});
if (result.status !== 0)
  throw Error("Scoped SST diff failed; no deployment or activation attempted.");
