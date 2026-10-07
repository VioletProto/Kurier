// Consume public SST outputs; passwords/tokens/cursor keys are never written.
import { readFileSync } from "node:fs";
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";

const [mode, ...args] = process.argv.slice(2);
if (!["api", "web", "cloud-web"].includes(mode))
  throw new Error("Choose api, web or cloud-web.");
let outputs;
try {
  outputs = JSON.parse(
    readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
  );
} catch {
  throw new Error(
    "Deploy the authorized dev-auth SST stage first; .sst/outputs.json is unavailable.",
  );
}
if (
  !["dev-auth", "dev-api"].includes(outputs.stage) ||
  outputs.region !== "us-east-2" ||
  !/^us-east-2_[A-Za-z0-9]+$/.test(outputs.cognitoUserPoolId ?? "") ||
  !/^[a-z0-9]+$/.test(outputs.cognitoClientId ?? "") ||
  outputs.cognitoIssuer !==
    `https://cognito-idp.us-east-2.amazonaws.com/${outputs.cognitoUserPoolId}`
)
  throw new Error(
    "Expected public outputs from a development auth/API stage, not a historical spike.",
  );
if (
  mode === "cloud-web" &&
  (outputs.stage !== "dev-api" ||
    !/^https:\/\/[a-z0-9]+\.execute-api\.us-east-2\.amazonaws\.com$/.test(
      outputs.apiUrl ?? "",
    ))
)
  throw new Error("Deploy dev-api first; valid cloud API outputs required.");
const origin = new URL(
  process.env.KURIER_FRONTEND_ORIGIN ??
    (mode === "cloud-web" ? outputs.frontendOrigin : "http://localhost:5173"),
);
if (
  origin.protocol !== "http:" ||
  !["localhost", "127.0.0.1", "[::1]"].includes(origin.hostname) ||
  !origin.port ||
  origin.username ||
  origin.password ||
  origin.pathname !== "/" ||
  origin.search ||
  origin.hash
)
  throw new Error("Use a loopback frontend origin with an explicit port.");
if (mode === "cloud-web" && origin.origin !== outputs.frontendOrigin)
  throw new Error(
    "The frontend origin must match the deployed API CORS origin; preview/deploy to change it.",
  );
const env = {
  ...process.env,
  KURIER_DYNAMODB_ENDPOINT:
    process.env.KURIER_DYNAMODB_ENDPOINT ?? "http://127.0.0.1:8000",
  KURIER_CONTROL_TABLE:
    process.env.KURIER_CONTROL_TABLE ?? "kurier-local-control",
  KURIER_STAGE: "local",
  KURIER_COGNITO_ISSUER: outputs.cognitoIssuer,
  KURIER_COGNITO_CLIENT_ID: outputs.cognitoClientId,
  KURIER_FRONTEND_ORIGIN: origin.origin,
  KURIER_CURSOR_KEY_BASE64:
    process.env.KURIER_CURSOR_KEY_BASE64 ?? randomBytes(32).toString("base64"),
  VITE_COGNITO_USER_POOL_ID: outputs.cognitoUserPoolId,
  VITE_COGNITO_CLIENT_ID: outputs.cognitoClientId,
  VITE_API_URL:
    mode === "cloud-web"
      ? outputs.apiUrl
      : `http://127.0.0.1:${process.env.PORT ?? "8080"}`,
  VITE_KURIER_STAGE: mode === "cloud-web" ? "dev-api" : "local",
};
const root = new URL("../", import.meta.url);
const child =
  mode === "api"
    ? spawn("go", ["run", ".", ...args], {
        cwd: new URL("../services/api/", import.meta.url),
        env,
        stdio: "inherit",
      })
    : spawn(
        "npm",
        [
          "run",
          "dev",
          "--workspace",
          "@kurier/web",
          "--",
          "--host",
          origin.hostname,
          "--port",
          origin.port,
          "--strictPort",
        ],
        { cwd: root, env, stdio: "inherit" },
      );
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => child.kill(signal));
child.on("error", () => {
  console.error("Local development process could not start.");
  process.exitCode = 1;
});
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
