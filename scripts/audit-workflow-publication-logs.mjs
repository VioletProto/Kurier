// Read-only controlled validation audit. Raw log messages and derived synthetic
// tokens stay in child/parent memory; stdout contains fixed codes and counts.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
const fixture = JSON.parse(
  readFileSync(
    process.argv[2] ?? "/tmp/kurier-publication-rollout-fixture.json",
    "utf8",
  ),
);
const uuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
for (const key of ["ProjectID", "SuccessFixtureID", "StopFixtureID"])
  if (!uuid.test(fixture[key] ?? "")) throw Error("invalid_fixture_identifier");
const start = Date.parse(fixture.CreatedAt);
if (!Number.isFinite(start)) throw Error("invalid_fixture_time");
const outputs = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8", maxBuffer: 16 * 1024 * 1024 },
  );
  if (r.status !== 0) throw Error("log_audit_unavailable");
  return JSON.parse(r.stdout);
}
const identity = aws(["sts", "get-caller-identity"]);
if (
  identity.Account !== "747336059622" ||
  identity.Arn.endsWith(":root") ||
  outputs.stage !== "dev-api"
)
  throw Error("intended_identity_required");
const tokens = [fixture.SuccessFixtureID, fixture.StopFixtureID].map((id) =>
  createHash("sha256")
    .update("kurier-workflow-fixture-v1/" + id)
    .digest("hex"),
);
const expected = {
  [fixture.SuccessFixtureID + "/workflow-token"]: 1,
  [fixture.SuccessFixtureID + "/workflow-consume"]: 1,
  [fixture.StopFixtureID + "/workflow-token"]: 1,
  [fixture.StopFixtureID + "/workflow-consume"]: 1,
  [fixture.StopFixtureID + "/workflow-delay"]: 1,
};
const counts = {},
  targetRequests = new Set(),
  phases = {},
  jobs = new Map();
let inspected = 0,
  diagnostics = 0,
  conflicts = 0;
const phaseNames = new Set([
  "claimed",
  "intent",
  "register",
  "registered",
  "writing",
  "put_ack",
  "settle",
  "settled",
  "finalize",
  "transaction_conflict",
]);
const codes = new Set([
  "ok",
  "publication_unavailable",
  "transaction_conflict",
  "transaction_canceled",
  "condition_failed",
]);
for (const [key, kind] of [
  ["apiFunction", "api"],
  ["workerFunction", "worker"],
  ["cleanupFunction", "cleanup"],
  ["controlledEndpointFunction", "target"],
]) {
  const config = aws([
    "lambda",
    "get-function-configuration",
    "--function-name",
    outputs[key],
  ]);
  const group = config.LoggingConfig?.LogGroup;
  if (!group?.startsWith("/aws/lambda/kurier-dev-api-"))
    throw Error("unexpected_log_scope");
  // AWS CLI paginates in memory; refuse an incomplete service/CLI page. Bound
  // both message count and bytes, and never print unexpected message content.
  const response = aws([
    "logs",
    "filter-log-events",
    "--log-group-name",
    group,
    "--start-time",
    String(start - 60000),
  ]);
  if (
    response.nextToken ||
    response.NextToken ||
    response.events.length > 10000
  )
    throw Error("log_audit_incomplete");
  for (const event of response.events) {
    const message = event.message;
    inspected++;
    if (tokens.some((token) => message.includes(token)))
      throw Error("runtime_value_containment_failed");
    const at = message.indexOf("{");
    if (at < 0) continue;
    let row;
    try {
      row = JSON.parse(message.slice(at));
    } catch {
      continue;
    }
    if (
      kind === "target" &&
      [fixture.SuccessFixtureID, fixture.StopFixtureID].includes(row.fixtureId)
    ) {
      const key = row.fixtureId + "/" + row.mode;
      if (!(key in expected)) throw Error("unexpected_target_mode");
      if (row.mode === "workflow-consume" && row.authorized !== true)
        throw Error("bearer_rejected");
      if (
        typeof row.requestId !== "string" ||
        targetRequests.has(row.requestId)
      )
        throw Error("duplicate_target_log_identifier");
      targetRequests.add(row.requestId);
      counts[key] = (counts[key] ?? 0) + 1;
    }
    if (kind === "worker" && row.projectId === fixture.ProjectID) {
      const allowed = new Set([
        "phase",
        "code",
        "projectId",
        "runId",
        "jobId",
        "executionId",
        "leaseId",
        "fence",
        "ticketId",
      ]);
      if (
        Object.keys(row).some((key) => !allowed.has(key)) ||
        !phaseNames.has(row.phase) ||
        !codes.has(row.code)
      )
        throw Error("unsafe_diagnostic_schema");
      for (const name of [
        "projectId",
        "runId",
        "jobId",
        "executionId",
        "leaseId",
        "ticketId",
      ])
        if (row[name] !== undefined && !uuid.test(row[name]))
          throw Error("unsafe_diagnostic_identifier");
      if (!Number.isSafeInteger(row.fence) || row.fence !== 1)
        throw Error("unexpected_lease_fence");
      if (row.code !== "ok" && row.code !== "transaction_conflict")
        throw Error("publication_boundary_failed");
      if (row.phase === "transaction_conflict") conflicts++;
      diagnostics++;
      phases[row.phase] = (phases[row.phase] ?? 0) + 1;
      const job = jobs.get(row.jobId) ?? new Set();
      if (row.code === "ok") job.add(row.phase);
      jobs.set(row.jobId, job);
    }
  }
}
if (
  Object.keys(counts).length !== Object.keys(expected).length ||
  Object.keys(expected).some((key) => counts[key] !== expected[key])
)
  throw Error("target_send_count_mismatch_or_logs_pending");
const required = [
  "claimed",
  "intent",
  "registered",
  "writing",
  "put_ack",
  "settled",
  "finalize",
];
if (
  jobs.size !== 5 ||
  [...jobs.values()].some((phases) => required.some((p) => !phases.has(p)))
)
  throw Error("publication_diagnostics_pending_or_incomplete");
console.log(
  JSON.stringify({
    projectId: fixture.ProjectID,
    targetCounts: counts,
    uniqueTargetRequests: targetRequests.size,
    jobs: jobs.size,
    phases,
    diagnostics,
    observedConflictEvents: conflicts,
    inspectedLogEvents: inspected,
    sourceAndRenderedValuesAbsent: true,
    safeDiagnosticSchema: true,
  }),
);
