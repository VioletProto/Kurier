// Read-only: compare locally built deployment archives and scoped configuration.
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
const o = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (r.status !== 0) throw Error("Scoped deployment inspection failed.");
  return JSON.parse(r.stdout);
}
const identity = aws(["sts", "get-caller-identity"]);
if (
  identity.Account !== "747336059622" ||
  identity.Arn.endsWith(":root") ||
  o.stage !== "dev-api" ||
  o.region !== "us-east-2"
)
  throw Error("Wrong account/stage.");
for (const [resource, name, pinning] of [
  ["UsersProjectsApi", o.apiFunction, true],
  ["EmptyProjectCleanup", o.cleanupFunction, true],
  ["CloudExecutionWorker", o.workerFunction, true],
  ["ExecutionControlledEndpoint", o.controlledEndpointFunction, false],
]) {
  if (!name?.startsWith("kurier-dev-api-")) throw Error("Unscoped function.");
  const expected = createHash("sha256")
    .update(
      readFileSync(
        new URL(`../.sst/artifacts/${resource}/code.zip`, import.meta.url),
      ),
    )
    .digest("base64");
  const fn = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    name,
  ]);
  const e = fn.Environment?.Variables ?? {};
  if (
    fn.CodeSha256 !== expected ||
    fn.State !== "Active" ||
    fn.LastUpdateStatus !== "Successful" ||
    fn.Runtime !== "provided.al2023" ||
    fn.Architectures?.join() !== "arm64" ||
    e.KURIER_STAGE !== "dev-api" ||
    (pinning &&
      (e.KURIER_EVIDENCE_PINNING_SCHEMA_VERSION !== "1" ||
        e.KURIER_CONTROL_TABLE !== o.controlTable ||
        e.KURIER_EVIDENCE_BUCKET !== o.evidenceBucket))
  )
    throw Error(`Deployment mismatch: ${resource}`);
  console.log(
    JSON.stringify({
      resource,
      codeSha256: fn.CodeSha256,
      archiveMatch: true,
      active: true,
      pinningMarker: pinning ? e.KURIER_EVIDENCE_PINNING_SCHEMA_VERSION : null,
      architecture: fn.Architectures[0],
      runtime: fn.Runtime,
    }),
  );
}
await import("./verify-cloud-execution-config.mjs");
