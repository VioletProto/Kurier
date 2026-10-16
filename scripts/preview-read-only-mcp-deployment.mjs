// Authorized preview only. Existing bootstrap must be proven before SST diff.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
if (process.argv.length !== 2) throw Error("Preview only; no arguments.");
await import("./preview-read-only-mcp.mjs");
const out = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
const r = spawnSync("node", ["scripts/sst-auth.mjs", "diff", "dev-api"], {
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
if (r.status !== 0)
  throw Error("Scoped diff failed; no deployment or activation attempted.");
