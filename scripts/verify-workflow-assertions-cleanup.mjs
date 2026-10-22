// Read-only, fixed project scope. Explicit invocation required; no write adapter.
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import {
  awsProcess,
  outputs as o,
} from "./verify-read-only-mcp-deployment.mjs";
import {
  accountingQuery,
  accountingSnapshot,
  compareAccounting,
} from "./lib/workflow-assertions-collectors.mjs";
import { persistCoordinatorLedger } from "./lib/workflow-coordinator-transport.mjs";
import { hash } from "./lib/workflow-assertions-activation.mjs";
if (
  process.argv.slice(2).join(" ") !== "--collect" ||
  !process.env.KURIER_ATTEMPT_METER_DIRECTORY
)
  throw Error("Explicit metered --collect required");
const meter = process.env.KURIER_ATTEMPT_METER_DIRECTORY;
const opening = JSON.parse(
  readFileSync(meter + "/attempts.jsonl", "utf8").split("\n")[0],
);
if (
  opening.purpose !== "focused-read-only-cleanup-verification" ||
  opening.limits.aws !== 100 ||
  opening.seconds !== 600
)
  throw Error("100-attempt/10-minute read-only meter required");
const source = readFileSync(
  "docs/development/workflow-assertions-final-rollout.json",
);
if (
  createHash("sha256").update(source).digest("hex") !==
  "f8b20b3c63a0dff4e772ce6bec200415639770f94cefaa1d63e451ba9de829df"
)
  throw Error("Historical packet changed");
const history = JSON.parse(source);
const project = "2d9723c2-facf-40e1-8a2b-130853d3c0f8";
if (
  o.stage !== "dev-api" ||
  o.region !== "us-east-2" ||
  o.controlTable !== "kurier-dev-api-ControlTable-bdxbaoxb" ||
  o.protectedTable !== "kurier-dev-api-ProtectedTable-szbvcchc"
)
  throw Error("Scope drift");
const deadlineMs = Date.now() + 600000;
const destination =
  "docs/development/workflow-assertions-cleanup-verification.json";
const report = {
  kind: "kurier-focused-cleanup-verification-v1",
  startedAt: new Date().toISOString(),
  project,
  readOnly: true,
  limits: { awsAttempts: 100, seconds: 600 },
  historicalPacketSha256: createHash("sha256").update(source).digest("hex"),
  historicalCollectorFailuresPreserved: true,
  historicalMissingReadbacksReconstructed: false,
  checks: {},
  observations: {},
  gaps: [],
};
const save = () => persistCoordinatorLedger(destination, report);
function check(v, name) {
  report.checks[name] = Boolean(v);
  save();
}
function aws(args, input) {
  const r = awsProcess(
    [...args, "--cli-connect-timeout", "4", "--cli-read-timeout", "8"],
    input,
    { deadlineMs, reserveMs: 30000 },
  );
  if (r.status !== 0 || r.error)
    throw Error("Scoped read failed; no automatic retry");
  const raw = r.stdout.toString("utf8");
  return raw.trim() ? JSON.parse(raw) : {};
}
function query(
  TableName,
  pk,
  prefix = "",
  fields = [
    "PK",
    "SK",
    "kind",
    "schemaVersion",
    "version",
    "state",
    "ownerId",
    "projectId",
    "deletionOperation",
    "writeSettled",
    "executionId",
    "jobId",
    "bytes",
  ],
) {
  const rows = [];
  let key;
  for (let page = 0; page < 10; page++) {
    const r = aws(["dynamodb", "query"], {
      TableName,
      ConsistentRead: true,
      KeyConditionExpression:
        "PK = :pk" + (prefix ? " AND begins_with(SK,:prefix)" : ""),
      ExpressionAttributeValues: {
        ":pk": { S: pk },
        ...(prefix ? { ":prefix": { S: prefix } } : {}),
      },
      ProjectionExpression: fields.map((_, i) => "#p" + i).join(","),
      ExpressionAttributeNames: Object.fromEntries(
        fields.map((f, i) => ["#p" + i, f]),
      ),
      ...(key ? { ExclusiveStartKey: key } : {}),
    });
    rows.push(...(r.Items ?? []));
    key = r.LastEvaluatedKey;
    if (!key) return rows;
  }
  throw Error("Bounded query incomplete");
}
const owners = [
  "12931237-a352-4734-99fa-41a2f4c89f26",
  "c31af90f-008f-4184-b7f1-5f6da328117a",
  "b8b36faf-2c6c-4618-9351-9cacf513a4df",
  "9c919ed7-f835-42ce-82ad-753fce1ed39e",
];
function accounting() {
  const rows = [];
  for (const [pk, prefix] of [
    ["STAGE#dev-api", "QUOTA"],
    ...owners.map((id) => ["U#" + id, "EXECUTION_DAY#"]),
  ]) {
    const r = aws(
      ["dynamodb", "query"],
      accountingQuery(o.controlTable, pk, prefix),
    );
    if (r.LastEvaluatedKey)
      throw Error("Accounting pagination requires review");
    rows.push(...(r.Items ?? []));
  }
  return accountingSnapshot(rows);
}
function stage() {
  return aws(["dynamodb", "get-item"], {
    TableName: o.controlTable,
    Key: { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } },
    ConsistentRead: true,
  }).Item;
}
try {
  const who = aws(["sts", "get-caller-identity"]);
  check(
    who.Account === "747336059622" &&
      /^arn:aws:(iam|sts)::747336059622:(user\/|assumed-role\/)/.test(who.Arn),
    "intendedNonRootIdentity",
  );
  if (!report.checks.intendedNonRootIdentity) throw Error("Identity drift");
  report.identity = { account: who.Account, arn: who.Arn };
  const startStage = stage();
  report.observations.stageBefore = startStage;
  check(
    hash(startStage) === hash(history.activation.finalVerifiedMarker),
    "stageVersionCapabilitiesGenerationMatchRetained",
  );
  const before = accounting();
  report.observations.accountingBefore = before;
  save();
  const control = query(o.controlTable, "P#" + project);
  report.observations.control = control;
  check(
    control.length === 1 &&
      control[0].kind?.S === "projectTombstone" &&
      control[0].state?.S === "deleted",
    "controlTombstoneOnly",
  );
  const tomb = control.find((r) => r.kind?.S === "projectTombstone");
  const operation = tomb?.deletionOperation?.M;
  check(
    operation?.state?.S === "completed" &&
      operation?.operationId?.S === "4527b106-f0f9-4cc8-8749-bb19fe34a934" &&
      Boolean(operation?.completedAt?.S),
    "deletionOperationCompletedAndIdentityMatched",
  );
  const fullTombstone = aws(["dynamodb", "get-item"], {
    TableName: o.controlTable,
    Key: { PK: { S: "P#" + project }, SK: { S: "META" } },
    ConsistentRead: true,
  }).Item;
  const tombstoneFields = [
    "PK",
    "SK",
    "kind",
    "schemaVersion",
    "projectId",
    "ownerId",
    "version",
    "state",
    "deletionEpoch",
    "initiatingVersion",
    "deletionOperation",
  ];
  check(
    fullTombstone?.PK?.S === "P#" + project &&
      fullTombstone?.SK?.S === "META" &&
      fullTombstone?.kind?.S === "projectTombstone" &&
      fullTombstone?.schemaVersion?.N === "1" &&
      fullTombstone?.projectId?.S === project &&
      fullTombstone?.ownerId?.S === owners[2] &&
      Object.keys(fullTombstone).every((k) => tombstoneFields.includes(k)),
    "completeTombstoneShapeRecognized",
  );
  report.observations.tombstone = Object.fromEntries(
    tombstoneFields
      .filter((k) => fullTombstone?.[k])
      .map((k) => [k, fullTombstone[k]]),
  );
  const protectedRows = query(o.protectedTable, "P#" + project);
  report.observations.protectedRows = protectedRows;
  check(protectedRows.length === 0, "protectedPartitionEmpty");
  const uploads = query(o.controlTable, "P#" + project, "UPLOAD#");
  report.observations.uploads = uploads;
  check(uploads.length === 0, "noOutstandingUploadTickets");
  let token;
  let objects = 0;
  for (let page = 0; page < 10; page++) {
    const s = aws(["s3api", "list-objects-v2"], {
      Bucket: o.evidenceBucket,
      Prefix: "dev-api/projects/" + project + "/",
      MaxKeys: 1000,
      ...(token ? { ContinuationToken: token } : {}),
    });
    objects += (s.Contents ?? []).length;
    token = s.IsTruncated ? s.NextContinuationToken : undefined;
    if (s.IsTruncated && !token) throw Error("S3 pagination inconsistent");
    if (!token) break;
  }
  if (token) throw Error("Bounded S3 inventory incomplete");
  report.observations.s3 = {
    bucket: o.evidenceBucket,
    prefix: "dev-api/projects/" + project + "/",
    objects,
    complete: true,
  };
  check(objects === 0, "evidencePrefixEmpty");
  const residual = aws(["dynamodb", "get-item"], {
    TableName: o.controlTable,
    Key: { PK: { S: "MAINTENANCE#dev-api" }, SK: { S: "RESIDUAL#" + project } },
    ConsistentRead: true,
    ProjectionExpression: "PK,SK,#k,projectId",
    ExpressionAttributeNames: { "#k": "kind" },
  }).Item;
  report.observations.residualRegistration = residual;
  check(
    residual?.kind?.S === "executionResidual" &&
      residual.projectId?.S === project,
    "residualSweepRegistrationRetained",
  );
  const after = accounting();
  report.observations.accountingAfter = after;
  check(
    compareAccounting(before, after),
    "currentAccountingStableAcrossFreshReads",
  );
  const quota = after.rows.find(
    (r) => r.PK.S === "STAGE#dev-api" && r.SK.S === "QUOTA",
  );
  const old = history.cleanup.lastCompletePreCleanupStageQuota[0];
  const captures = history.runtime.cases.flatMap((c) => c.evidence);
  const bytes = captures.reduce((sum, e) => sum + BigInt(e.storedBytes), 0n);
  const n = (r, k) => BigInt(r?.[k]?.N ?? "0");
  report.observations.accountingInterpretation = {
    retainedMonthlyCount: old.count.N,
    retainedMonthlyVersion: old.version.N,
    retainedReservedBytes: old.storageReservedBytes.N,
    retainedPublishedCaptures: captures.length,
    retainedEvidenceBytes: String(bytes),
    currentMonthlyCount: quota?.count?.N,
    currentMonthlyVersion: quota?.version?.N,
    currentStorageReservedBytes: quota?.storageReservedBytes?.N ?? "0",
    expectedCleanupVersion: String(
      BigInt(old.version.N) + BigInt(captures.length),
    ),
    historicalPreservationProved: false,
  };
  check(
    quota?.count?.N === old.count.N && quota.month?.S === old.month.S,
    "admissionUsageMatchesRetainedEvidence",
  );
  check(
    n(quota, "pinnedCount") === 0n &&
      n(quota, "pinnedBytes") === 0n &&
      n(quota, "storageReservedBytes") === 0n &&
      n(quota, "inflightCount") === 0n,
    "currentStoragePinsAndInflightZero",
  );
  check(
    bytes === BigInt(old.storageReservedBytes.N) &&
      n(quota, "version") === BigInt(old.version.N) + BigInt(captures.length),
    "versionAndStorageConsistentWithNormalTicketCleanup",
  );
  const expected = ["2", "2", "14"];
  for (let i = 0; i < 4; i++) {
    const day = after.rows.filter((r) => r.PK.S === "U#" + owners[i]);
    check(
      i === 3
        ? day.length === 0
        : day.length === 1 && day[0].count?.N === expected[i],
      "ownerDailyUsageConsistent" + i,
    );
  }
  const finalStage = stage();
  report.observations.stageAfter = finalStage;
  check(
    hash(finalStage) === hash(startStage),
    "stageUnchangedAcrossVerification",
  );
  report.currentCleanupVerified = [
    "controlTombstoneOnly",
    "completeTombstoneShapeRecognized",
    "deletionOperationCompletedAndIdentityMatched",
    "protectedPartitionEmpty",
    "noOutstandingUploadTickets",
    "evidencePrefixEmpty",
  ].every((k) => report.checks[k]);
  report.currentConsistencyVerified = Object.values(report.checks).every(
    Boolean,
  );
  report.uploadInterpretation =
    "No upload tickets remain now; retained publication/settlement evidence and completed deletion are consistent with normal draining. Missing historical upload transitions are not reconstructed.";
  report.gaps.push(
    "Missing historical final accounting readbacks remain unproved; current consistency cannot establish every historical intermediate state",
    "Historical legacy-A503 cause and exact prior browser-blocked operation remain unestablished",
    "Historical first-attempt AWS-read total and all actual billing totals remain unknown",
    "H-L remain held; no new target execution or identity allowance is inferred",
  );
  if (!report.currentConsistencyVerified)
    report.gaps.push(
      "One or more current checks did not pass; inspect retained read-only observations without repairs",
    );
} catch {
  report.error =
    "Focused read-only collection stopped; no automatic replay or writes";
  report.gaps.push(
    "Collection incomplete; only retained observations are established",
  );
  process.exitCode = 1;
}
report.finishedAt = new Date().toISOString();
save();
console.log(
  JSON.stringify({
    receipt: destination,
    currentCleanupVerified: report.currentCleanupVerified ?? false,
    currentConsistencyVerified: report.currentConsistencyVerified ?? false,
    failedChecks: Object.entries(report.checks)
      .filter(([, v]) => !v)
      .map(([k]) => k),
    collectionError: Boolean(report.error),
  }),
);
