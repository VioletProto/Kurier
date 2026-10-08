// Bridge the AWS CLI session to Go only in child memory; never print credentials.
import { readFileSync } from "node:fs";
import { spawn, spawnSync } from "node:child_process";
function awsJson(args) {
  const result = spawnSync("aws", args, { encoding: "utf8" });
  if (result.status !== 0) throw new Error("AWS session unavailable.");
  return JSON.parse(result.stdout);
}
const identity = awsJson(["sts", "get-caller-identity", "--output", "json"]);
if (identity.Account !== "747336059622" || identity.Arn.endsWith(":root"))
  throw new Error("Intended account/non-root identity required.");
const credentials = awsJson([
  "configure",
  "export-credentials",
  "--format",
  "process",
]);
const outputs = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
const env = {
  ...process.env,
  AWS_REGION: "us-east-2",
  KURIER_AWS_ACCOUNT_ID: "747336059622",
  KURIER_AWS_TEST_TABLE: outputs.controlTable,
  KURIER_AWS_TEST_PROTECTED: outputs.protectedTable,
  KURIER_AWS_TEST_KEY: outputs.protectedKeyArn,
};
for (const [source, target] of [
  ["AccessKeyId", "AWS_ACCESS_KEY_ID"],
  ["SecretAccessKey", "AWS_SECRET_ACCESS_KEY"],
  ["SessionToken", "AWS_SESSION_TOKEN"],
]) {
  if (credentials[source]) env[target] = credentials[source];
}
if (!env.AWS_ACCESS_KEY_ID || !env.AWS_SECRET_ACCESS_KEY)
  throw new Error("No usable AWS session.");
const filter = process.argv[2] ?? "TestAWS";
if (!["TestAWS", "TestAWSControl", "TestAWSProtected"].includes(filter))
  throw new Error("Choose a scoped AWS test filter.");
const child = spawn(
  "go",
  [
    "test",
    "-race",
    "-tags=integration,aws",
    "-run",
    filter,
    "-count=1",
    "-v",
    "./internal/ownership",
  ],
  { cwd: new URL("../services/api/", import.meta.url), env, stdio: "inherit" },
);
child.on("error", () => {
  console.error("AWS integration could not start.");
  process.exitCode = 1;
});
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
