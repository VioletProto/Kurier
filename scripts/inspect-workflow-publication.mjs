// Read-only sampling: identifiers/states only, never payloads or credentials.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
const project = process.argv[2];
const seconds = Number(process.argv[3] ?? 180);
const uuid =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
if (
  !uuid.test(project ?? "") ||
  !Number.isInteger(seconds) ||
  seconds < 1 ||
  seconds > 600
)
  throw Error("Provide a project UUID and 1–600 seconds.");
const outputs = JSON.parse(
  readFileSync(new URL("../.sst/outputs.json", import.meta.url), "utf8"),
);
if (
  outputs.stage !== "dev-api" ||
  outputs.controlTable !== "kurier-dev-api-ControlTable-bdxbaoxb"
)
  throw Error("Expected development resources required.");
function aws(args) {
  const r = spawnSync(
    "aws",
    [...args, "--region", "us-east-2", "--output", "json"],
    { encoding: "utf8" },
  );
  if (r.status !== 0) throw Error("inspection_unavailable");
  return JSON.parse(r.stdout);
}
const identity = aws(["sts", "get-caller-identity"]);
if (identity.Account !== "747336059622" || identity.Arn.endsWith(":root"))
  throw Error("intended_identity_required");
const fields = {
  kind: "S",
  state: "S",
  version: "N",
  fence: "N",
  leaseId: "S",
  deadline: "S",
  writeSettled: "BOOL",
  bytes: "N",
  inflightCount: "N",
  storageReservedBytes: "N",
  runId: "S",
  jobId: "S",
  executionId: "S",
  position: "N",
  currentPosition: "N",
  attempts: "N",
};
let previous = new Map();
const until = Date.now() + seconds * 1000;
do {
  const next = new Map();
  for (const table of [outputs.controlTable, outputs.protectedTable]) {
    const names = Object.fromEntries(
      Object.keys(fields).map((key, index) => ["#a" + index, key]),
    );
    const result = aws([
      "dynamodb",
      "query",
      "--table-name",
      table,
      "--consistent-read",
      "--key-condition-expression",
      "PK = :pk",
      "--expression-attribute-values",
      JSON.stringify({ ":pk": { S: "P#" + project } }),
      "--projection-expression",
      "SK," + Object.keys(names).join(","),
      "--expression-attribute-names",
      JSON.stringify(names),
    ]);
    if (result.LastEvaluatedKey) throw Error("inspection_incomplete");
    for (const item of result.Items) {
      const key = item.SK?.S;
      if (key !== "META" && !/^(RUN|JOB|UPLOAD|OUT|EXEC)#/.test(key ?? ""))
        continue;
      // SKs are server-created identifiers with fixed suffixes. Fail closed on a
      // future unexpected format rather than printing arbitrary persisted text.
      if (
        !/^META$|^(RUN|JOB|UPLOAD|OUT|EXEC)#[0-9a-f-]{36}(#(STEP#[0-9]{2}|SNAP|RET|BINDINGS|INPUTS|OUTPUT#[0-9]{2}))?$/.test(
          key,
        )
      )
        throw Error("unknown_identifier_shape");
      const row = {
        table: table === outputs.controlTable ? "Control" : "Protected",
        key,
      };
      for (const [name, type] of Object.entries(fields)) {
        const value = item[name]?.[type];
        if (value === undefined) continue;
        if (
          ["leaseId", "runId", "jobId", "executionId"].includes(name) &&
          !uuid.test(value)
        )
          throw Error("unknown_identifier_shape");
        if (
          name === "state" &&
          ![
            "active",
            "deleting",
            "deleted",
            "queued",
            "pending",
            "claimed",
            "running",
            "completed",
            "failed",
            "skipped",
            "writing",
            "uploaded",
            "published",
            "live",
            "expired",
          ].includes(value)
        )
          throw Error("unknown_state");
        if (
          name === "kind" &&
          ![
            "project",
            "projectTombstone",
            "execution",
            "executionJob",
            "executionUpload",
            "executionOutbox",
            "executionSnapshot",
            "executionRetention",
            "executionWorkflowRun",
            "executionWorkflowStep",
            "executionWorkflowInputs",
            "executionWorkflowOutput",
            "executionBindings",
          ].includes(value)
        )
          throw Error("unknown_kind");
        if (name === "deadline" && !/^\d{4}-\d{2}-\d{2}T[0-9:.]+Z$/.test(value))
          throw Error("unknown_time");
        row[name] = value;
      }
      const mapKey = row.table + ":" + key;
      next.set(mapKey, JSON.stringify(row));
      if (previous.get(mapKey) !== next.get(mapKey))
        console.log(
          JSON.stringify({
            observedAt: new Date().toISOString(),
            projectId: project,
            ...row,
          }),
        );
    }
  }
  for (const key of previous.keys())
    if (!next.has(key))
      console.log(
        JSON.stringify({
          observedAt: new Date().toISOString(),
          projectId: project,
          removed: key,
        }),
      );
  previous = next;
  if (Date.now() < until)
    await new Promise((resolve) => setTimeout(resolve, 250));
} while (Date.now() < until);
console.log(
  JSON.stringify({ code: "inspection_complete", projectId: project, seconds }),
);
