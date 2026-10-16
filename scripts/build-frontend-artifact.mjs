import { readFileSync } from "node:fs";
import { buildFrontendArtifact } from "../infra/frontend-artifact.ts";
const out = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
if (out.stage !== "dev-api" || out.region !== "us-east-2")
  throw Error("Expected public dev-api outputs.");
const artifact = buildFrontendArtifact({
  VITE_API_URL: out.apiUrl,
  VITE_COGNITO_USER_POOL_ID: out.cognitoUserPoolId,
  VITE_COGNITO_CLIENT_ID: out.cognitoClientId,
  VITE_KURIER_STAGE: "dev-api",
});
console.log(
  JSON.stringify({
    releaseId: artifact.manifest.releaseId,
    publicFiles: artifact.manifest.files.length,
    deploymentPerformed: false,
  }),
);
