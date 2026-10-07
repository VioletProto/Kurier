// Bridge AWS CLI login sessions to SST's AWS SDK without writing credentials.
import { spawn, spawnSync } from "node:child_process";
const operation = process.argv[2];
if (!["diff", "deploy"].includes(operation))
  throw new Error("Choose diff or deploy.");
const account = process.env.KURIER_AWS_ACCOUNT_ID;
if (!/^\d{12}$/.test(account ?? ""))
  throw new Error("Set the intended KURIER_AWS_ACCOUNT_ID.");
function awsJson(args) {
  const result = spawnSync("aws", args, { encoding: "utf8" });
  if (result.status !== 0)
    throw new Error(
      "AWS CLI account/session lookup failed. Sign in using a non-root identity.",
    );
  try {
    return JSON.parse(result.stdout);
  } catch {
    throw new Error("AWS CLI did not return the expected session metadata.");
  }
}
const identity = awsJson(["sts", "get-caller-identity", "--output", "json"]);
if (identity.Account !== account || identity.Arn.endsWith(":root"))
  throw new Error("Wrong account or root identity; refusing SST.");
const credentials = awsJson([
  "configure",
  "export-credentials",
  "--format",
  "process",
]);
const env = {
  ...process.env,
  KURIER_AUTH_TIER: "ESSENTIALS",
  AWS_REGION: "us-east-2",
};
for (const [source, target] of [
  ["AccessKeyId", "AWS_ACCESS_KEY_ID"],
  ["SecretAccessKey", "AWS_SECRET_ACCESS_KEY"],
  ["SessionToken", "AWS_SESSION_TOKEN"],
]) {
  if (credentials[source]) env[target] = credentials[source];
}
if (!env.AWS_ACCESS_KEY_ID || !env.AWS_SECRET_ACCESS_KEY)
  throw new Error("No usable AWS CLI session; refusing SST.");
const child = spawn(
  "npm",
  ["exec", "--", "sst", operation, "--stage", "dev-auth"],
  { cwd: new URL("../", import.meta.url), env, stdio: "inherit" },
);
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => child.kill(signal));
child.on("error", () => {
  console.error("SST could not start.");
  process.exitCode = 1;
});
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
