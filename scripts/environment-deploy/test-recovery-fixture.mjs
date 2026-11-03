import { readFileSync } from "node:fs";
import { hash, inspectRows } from "../lib/project-environments-activation.mjs";
const inv = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-inventory.json",
      import.meta.url,
    ),
  ),
);
const scope = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-execution-scope.json",
      import.meta.url,
    ),
  ),
);
export function fixture() {
  const marker = {
    ...inv.stageAfter,
    state: { S: "workflow-migration" },
    version: { N: "16" },
  };
  const functions = inv.functions.map((f) => ({
    resource: f.logical,
    iamHash: hash({
      policies: [...f.inlinePolicies].sort((a, b) =>
        a.PolicyName.localeCompare(b.PolicyName),
      ),
      managed: [...f.managedPolicies]
        .sort((a, b) => a.policy.Arn.localeCompare(b.policy.Arn))
        .map((x) => ({
          arn: x.policy.Arn,
          version: x.policy.DefaultVersionId,
          document: x.version.Document,
        })),
      trust: f.role.AssumeRolePolicyDocument,
    }),
    configurationHash: f.environmentSha256,
    codeSha256: f.configuration.CodeSha256,
    runtime: f.configuration.Runtime,
    architecture: f.configuration.Architectures,
    memory: f.configuration.MemorySize,
    timeout: f.configuration.Timeout,
    state: "Active",
    update: "Successful",
  }));
  const rows = [
    {
      table: scope.controlTable,
      item: {
        PK: { S: "STAGE#dev-api" },
        SK: { S: "QUOTA" },
        kind: { S: "executionQuota" },
        schemaVersion: { N: "1" },
        version: { N: "75" },
        count: { N: "32" },
        month: { S: "2026-10" },
      },
    },
  ];
  const bundle = {
    marker,
    platform: { functions },
    routing: {
      routes: inv.gateway.routes,
      integrations: inv.gateway.integrations,
      policies: Object.fromEntries(
        inv.functions.map((f) => [
          f.logical,
          f.invokePolicy?.Policy
            ? JSON.parse(f.invokePolicy.Policy)
            : { Version: "2012-10-17", Statement: [] },
        ]),
      ),
    },
    rows,
    depth: { visible: "0", inflight: "0", delayed: "0" },
    frontendReady: true,
    frontendExpectation: "baseline",
    frontendExpectedReleaseId: inv.frontendRollback.manifest.releaseId,
    stateInspection: {
      noCurrentDeployLock: true,
      noObservedStateChangesSinceOriginalCollection: true,
    },
  };
  return {
    bundle,
    context: {
      expectedMarker: structuredClone(marker),
      accountingSha256: inspectRows(rows).accountingSha256,
      clientRequestToken: "12345678-1234-5678-1234-123456789abc",
    },
  };
}
