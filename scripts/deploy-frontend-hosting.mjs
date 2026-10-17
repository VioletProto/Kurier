// Requires explicit scoped deployment authorization; never activates schema markers.
// Run the guarded preview and inspect its plan immediately before this command.
import { readFileSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { aws, identity, outputs } from "./verify-read-only-mcp-deployment.mjs";
if (
  process.argv.length !== 3 ||
  !["--deploy", "--recheck-and-deploy"].includes(process.argv[2])
)
  throw Error(
    "Explicit approved --deploy required after inspecting the guarded preview.",
  );
identity();
if (process.argv[2] === "--recheck-and-deploy") {
  const check = spawnSync(
    process.execPath,
    ["scripts/preview-frontend-hosting.mjs", "--diff"],
    { encoding: "utf8", maxBuffer: 32 * 1024 * 1024 },
  );
  if (check.status !== 0)
    throw Error("Guarded release preview failed; no deployment attempted.");
  const raw = readFileSync(".sst/frontend-hosting-diff.txt", "utf8");
  const end = raw.lastIndexOf("\n]") + 2;
  if (end < 2) throw Error("Incomplete release preview.");
  const events = JSON.parse(raw.slice(0, end));
  const allowed = new Set([
    "FrontendWebCloudfrontFunctionRequest",
    "FrontendWebKvKeys.sst.aws.KvKeys",
    "FrontendWebAssetFiles.sst.aws.BucketFiles",
    "FrontendWebInvalidation.sst.aws.DistributionInvalidation",
  ]);
  const changes = events.filter((e) => e.op !== "read");
  if (
    !changes.length ||
    changes.some(
      (e) => e.op !== "update" || !allowed.has(e.urn.split("::").at(-1)),
    )
  )
    throw Error(
      "Release plan is empty or exceeds existing static hosting updates; inspect before deploying.",
    );
  console.log(
    JSON.stringify({
      reviewedStaticUpdates: changes.map((e) => ({
        name: e.urn.split("::").at(-1),
        properties: Object.keys(e.detailedDiff ?? {}),
      })),
      newResources: 0,
      runtimeIamChanges: false,
    }),
  );
}
const inventory = JSON.parse(
  readFileSync(".sst/frontend-hosting-inventory.json", "utf8"),
);
if (
  !inventory.previewCompleted ||
  !inventory.bootstrapUnchanged ||
  !inventory.consumersUnchanged ||
  !inventory.accountingUnchanged ||
  Date.now() - Date.parse(inventory.checkedAt) > 15 * 60000
)
  throw Error("Fresh guarded preview required.");
const parameter = aws([
  "ssm",
  "get-parameter",
  "--name",
  "/sst/bootstrap",
]).Parameter;
const bootstrap = JSON.parse(parameter.Value);
if (
  bootstrap.version !== 5 ||
  bootstrap.state !== inventory.bootstrap.state ||
  bootstrap.asset !== inventory.bootstrap.asset ||
  parameter.Version !== inventory.bootstrap.parameterVersion
)
  throw Error("Existing reviewed bootstrap required.");
const credentials = aws([
  "configure",
  "export-credentials",
  "--format",
  "process",
]);
const result = spawnSync(
  "npm",
  ["exec", "--", "sst", "deploy", "--stage", "dev-api"],
  {
    cwd: new URL("../", import.meta.url),
    encoding: "utf8",
    maxBuffer: 32 * 1024 * 1024,
    env: {
      ...process.env,
      AWS_REGION: "us-east-2",
      AWS_ACCESS_KEY_ID: credentials.AccessKeyId,
      AWS_SECRET_ACCESS_KEY: credentials.SecretAccessKey,
      AWS_SESSION_TOKEN: credentials.SessionToken,
      KURIER_AWS_ACCOUNT_ID: inventory.account,
      KURIER_COGNITO_POOL_ID: outputs.cognitoUserPoolId,
      KURIER_COGNITO_CLIENT_ID: outputs.cognitoClientId,
      KURIER_FRONTEND_ORIGIN: outputs.frontendOrigin,
      SST_TELEMETRY_DISABLED: "1",
    },
  },
);
writeFileSync(
  ".sst/frontend-hosting-deploy.log",
  result.stdout + result.stderr,
);
if (result.status !== 0)
  throw Error(
    "Scoped deployment failed; inspect ignored deployment log before any retry.",
  );
console.log(
  JSON.stringify({
    stage: "dev-api",
    deploymentCompleted: true,
    schemaActivationPerformed: false,
  }),
);
