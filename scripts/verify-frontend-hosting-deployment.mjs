// Read-only deployment proof. Persist hashes/public IDs only, never environments.
import { readFileSync, writeFileSync, existsSync, unlinkSync } from "node:fs";
import { createHash } from "node:crypto";
import {
  aws,
  identity,
  outputs,
  capture,
  hash,
} from "./verify-read-only-mcp-deployment.mjs";
const [mode, file, ...rest] = process.argv.slice(2);
if (
  !file ||
  rest.length ||
  !["--capture-baseline", "--verify-baseline"].includes(mode)
)
  throw Error("Choose --capture-baseline PATH or --verify-baseline PATH.");
identity();
if (mode === "--capture-baseline") {
  capture(file);
  const baseline = JSON.parse(readFileSync(file, "utf8"));
  const fn = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    outputs.apiFunction,
  ]);
  if (fn.Environment?.Variables?.KURIER_HOSTED_FRONTEND_ORIGIN)
    throw Error("Initial deployment baseline must predate hosting.");
  baseline.apiEnvironmentWithoutHostedHash = hash(
    fn.Environment?.Variables ?? {},
  );
  baseline.hostingInventory = JSON.parse(
    readFileSync(".sst/frontend-hosting-inventory.json", "utf8"),
  );
  writeFileSync(file, JSON.stringify(baseline, null, 2) + "\n");
} else {
  const baseline = JSON.parse(readFileSync(file, "utf8"));
  const temporary = ".sst/frontend-hosting-post-functions.json";
  if (existsSync(temporary)) unlinkSync(temporary);
  capture(temporary);
  const after = JSON.parse(readFileSync(temporary, "utf8"));
  if (hash(after.controls) !== hash(baseline.controls))
    throw Error("Runtime controls changed.");
  for (const f of after.functions) {
    const before = baseline.functions.find((b) => b.name === f.name);
    for (const key of [
      "name",
      "role",
      "iamHash",
      "runtime",
      "architecture",
      "memory",
      "timeout",
      "reserved",
      "mcp",
      "savedRequests",
    ])
      if (hash(f[key]) !== hash(before?.[key]))
        throw Error(`Runtime ${key} changed: ${f.resource}`);
    if (
      f.name !== outputs.apiFunction &&
      f.configurationHash !== before.configurationHash
    )
      throw Error("Unreviewed function environment change.");
    const wanted = createHash("sha256")
      .update(readFileSync(`.sst/artifacts/${f.resource}/code.zip`))
      .digest("base64");
    if (
      f.codeSha256 !== wanted ||
      f.state !== "Active" ||
      f.update !== "Successful"
    )
      throw Error("Function package/readiness mismatch.");
  }
  const env =
    aws([
      "lambda",
      "get-function-configuration",
      "--function-name",
      outputs.apiFunction,
    ]).Environment?.Variables ?? {};
  if (env.KURIER_HOSTED_FRONTEND_ORIGIN !== outputs.frontendHosting?.siteUrl)
    throw Error("Exact hosted origin missing.");
  delete env.KURIER_HOSTED_FRONTEND_ORIGIN;
  if (hash(env) !== baseline.apiEnvironmentWithoutHostedHash)
    throw Error("Unreviewed API environment change.");
  const inventory = JSON.parse(
    readFileSync(".sst/frontend-hosting-inventory.json", "utf8"),
  );
  for (const key of [
    "account",
    "region",
    "bootstrap",
    "plan",
    "accountConcurrency",
    "unreserved",
    "stageSha256",
    "quotaSha256",
    "projectAccounting",
    "schedule",
    "api",
    "generation",
    "stageVersion",
    "worker",
    "cognito",
  ])
    if (hash(inventory[key]) !== hash(baseline.hostingInventory[key]))
      throw Error(`Preserved ${key} changed.`);
  const report = {
    checkedAt: new Date().toISOString(),
    siteUrl: outputs.frontendHosting.siteUrl,
    runtimeIamUnchanged: true,
    onlyApiHostedOriginAdded: true,
    exactBuiltPackages: true,
    capabilitiesGenerationAccountingUnchanged: true,
    freePlanQuotasConcurrencyBootstrapUnchanged: true,
    functions: after.functions.map((f) => ({
      name: f.name,
      codeSha256: f.codeSha256,
      iamHash: f.iamHash,
    })),
    controls: after.controls,
  };
  writeFileSync(
    ".sst/frontend-hosting-preserved.json",
    JSON.stringify(report, null, 2) + "\n",
  );
  console.log(JSON.stringify(report));
}
