// Read-only observation of the deletion already requested by the instructor owner.
import { readFileSync } from "node:fs";
import {
  awsProcess,
  outputs as o,
  hash,
} from "./verify-read-only-mcp-deployment.mjs";
import {
  accountingQuery,
  accountingSnapshot,
} from "./lib/workflow-assertions-collectors.mjs";
import {
  coordinatorBudget,
  persistCoordinatorLedger,
} from "./lib/workflow-coordinator-transport.mjs";
const dir = process.env.KURIER_ATTEMPT_METER_DIRECTORY;
if (process.argv[2] !== "--observe" || !dir)
  throw Error("Explicit metered observation required");
const opening = JSON.parse(
  readFileSync(dir + "/attempts.jsonl", "utf8").split("\n")[0],
);
if (
  opening.continuationParent?.sha256 !==
  "50318328c26aecc98f776e41768931d3d693c23a3b103c8bbf2f7b5282fdece1"
)
  throw Error("Reviewed continuation parent required");
const report = {
  kind: "kurier-existing-instructor-deletion-observation-v1",
  startedAt: new Date().toISOString(),
  startedCampaignAt: new Date(
    opening.continuationParent.firstAttemptAt * 1000,
  ).toISOString(),
  readOnly: true,
  checks: {},
  observations: [],
  userReportedAttribution:
    "User states extra two charged steps and3302 reserved bytes are disposable instructor-review data; normal project deletion already requested",
  historicalMissingReadbacksReconstructed: false,
};
const path = ".build/workflow-assertions-hl-resumed/instructor-cleanup.json";
const save = () => persistCoordinatorLedger(path, report);
const ledger = { startedAt: report.startedCampaignAt, apiCalls: 0 };
const budget = coordinatorBudget({
  ledger,
  meterPath: dir + "/attempts.jsonl",
});
function check(value, key) {
  report.checks[key] = Boolean(value);
  save();
  if (!value) throw Error("Existing cleanup observation failed: " + key);
}
function aws(args, input) {
  budget();
  const r = awsProcess(
    [...args, "--cli-connect-timeout", "4", "--cli-read-timeout", "8"],
    input,
    {
      deadlineMs: Date.parse(ledger.startedAt) + 100 * 60000,
      reserveMs: 30000,
    },
  );
  if (r.status !== 0 || r.error)
    throw Error("Read-only observation failed; no replay");
  return JSON.parse(r.stdout);
}
const fields = [
  "PK",
  "SK",
  "kind",
  "schemaVersion",
  "version",
  "state",
  "projectId",
  "ownerId",
  "deletionOperation",
  "deletionEpoch",
  "initiatingVersion",
  "bytes",
  "storageReservedBytes",
  "count",
  "month",
  "pinnedCount",
  "pinnedBytes",
  "inflightCount",
  "jobId",
  "executionId",
  "objectKey",
  "checksum",
  "writeSettled",
];
function query(pk, table = o.controlTable, prefix = "") {
  let key;
  const rows = [];
  for (let page = 0; page < 10; page++) {
    const r = aws(["dynamodb", "query", "--no-paginate"], {
      TableName: table,
      ConsistentRead: true,
      KeyConditionExpression:
        "PK = :p" + (prefix ? " AND begins_with(SK,:s)" : ""),
      ExpressionAttributeValues: {
        ":p": { S: pk },
        ...(prefix ? { ":s": { S: prefix } } : {}),
      },
      ProjectionExpression: fields.map((_, i) => "#f" + i).join(","),
      ExpressionAttributeNames: Object.fromEntries(
        fields.map((f, i) => ["#f" + i, f]),
      ),
      ...(key ? { ExclusiveStartKey: key } : {}),
    });
    rows.push(...(r.Items ?? []));
    key = r.LastEvaluatedKey;
    if (!key) return rows;
  }
  throw Error("Incomplete bounded query");
}
function accounting(owners) {
  const rows = [];
  for (const [pk, prefix] of [
    ["STAGE#dev-api", "QUOTA"],
    ...owners.map((x) => ["U#" + x, "EXECUTION_DAY#"]),
  ]) {
    const r = aws(
      ["dynamodb", "query", "--no-paginate"],
      accountingQuery(o.controlTable, pk, prefix),
    );
    check(!r.LastEvaluatedKey, "accountingComplete");
    rows.push(...(r.Items ?? []));
  }
  return accountingSnapshot(rows);
}
try {
  const who = aws(["sts", "get-caller-identity"]);
  check(
    who.Account === "747336059622" &&
      /^arn:aws:(iam|sts)::747336059622:(user\/|assumed-role\/)/.test(who.Arn),
    "intendedNonRootIdentity",
  );
  const retained = JSON.parse(
    readFileSync(
      "docs/development/workflow-assertions-cleanup-verification.json",
    ),
  );
  const matches = [];
  let key;
  for (let page = 0; page < 10; page++) {
    const r = aws(["dynamodb", "scan", "--no-paginate"], {
      TableName: o.controlTable,
      ConsistentRead: true,
      FilterExpression: "#k IN (:p,:t)",
      ExpressionAttributeNames: {
        "#k": "kind",
        ...Object.fromEntries(fields.map((f, i) => ["#f" + i, f])),
      },
      ExpressionAttributeValues: {
        ":p": { S: "project" },
        ":t": { S: "projectTombstone" },
      },
      ProjectionExpression: fields.map((_, i) => "#f" + i).join(","),
      ...(key ? { ExclusiveStartKey: key } : {}),
    });
    for (const item of r.Items ?? []) {
      if (
        item.deletionOperation?.M?.operationId?.S &&
        Date.parse(item.deletionOperation.M.startedAt?.S) >
          Date.parse(retained.finishedAt)
      )
        matches.push(item);
    }
    key = r.LastEvaluatedKey;
    if (!key) break;
  }
  check(!key, "discoveryComplete");
  check(matches.length === 1, "exactlyOneNewExistingDeletion");
  const project = matches[0].projectId.S,
    owner = matches[0].ownerId.S,
    operation = matches[0].deletionOperation.M.operationId.S;
  report.projectId = project;
  report.ownerId = owner;
  report.deletionOperationId = operation;
  report.initialProject = matches[0];
  save();
  const known = [
    "12931237-a352-4734-99fa-41a2f4c89f26",
    "c31af90f-008f-4184-b7f1-5f6da328117a",
    "b8b36faf-2c6c-4618-9351-9cacf513a4df",
    "9c919ed7-f835-42ce-82ad-753fce1ed39e",
  ];
  const owners = [...known, owner];
  report.accountingBefore = accounting(owners);
  const quota = report.accountingBefore.rows.find(
    (r) => r.PK.S === "STAGE#dev-api",
  );
  check(
    quota.count.N === "22" && quota.month.S === "2026-10",
    "chargedStageUsage22Preserved",
  );
  const day = report.accountingBefore.rows.filter(
    (r) => r.PK.S === "U#" + owner && r.SK.S === "EXECUTION_DAY#2026-10-21",
  );
  check(
    day.length === 1 && day[0].count.N === "2",
    "instructorCurrentDayChargedTwo",
  );
  check(
    report.accountingBefore.rows
      .filter((r) => r.PK.S.startsWith("U#"))
      .reduce((sum, r) => sum + BigInt(r.count.N), 0n) === 22n,
    "monthlyUsageReconcilesAcrossBothDaysAndKnownOwners",
  );
  const until = Math.min(
    Date.now() + 20 * 60000,
    Date.parse(ledger.startedAt) + 100 * 60000 - 30000,
  );
  let control;
  while (Date.now() < until) {
    control = query("P#" + project);
    const meta = control.find((r) => r.SK.S === "META");
    check(
      meta?.deletionOperation?.M?.operationId?.S === operation,
      "existingDeletionIdentityStable",
    );
    const obs = {
      at: new Date().toISOString(),
      controlRows: control.length,
      projectState: meta.state.S,
      deletionState: meta.deletionOperation.M.state.S,
      uploadTickets: control
        .filter((r) => r.kind?.S === "executionUpload")
        .map((r) => ({
          jobId: r.jobId?.S,
          bytes: r.bytes?.N,
          state: r.state?.S,
          writeSettled: r.writeSettled?.BOOL,
        })),
      snapshotBytes: control
        .filter((r) => r.kind?.S === "executionSnapshot")
        .map((r) => r.bytes?.N),
    };
    report.observations.push(obs);
    save();
    console.log(
      JSON.stringify({
        projectId: project,
        deletionOperationId: operation,
        ...obs,
      }),
    );
    if (
      control.length === 1 &&
      meta.kind.S === "projectTombstone" &&
      meta.state.S === "deleted" &&
      meta.deletionOperation.M.state.S === "completed"
    )
      break;
    await new Promise((resolve) => setTimeout(resolve, 20000));
  }
  check(
    control?.length === 1 &&
      control[0].kind.S === "projectTombstone" &&
      control[0].state.S === "deleted" &&
      control[0].deletionOperation.M.state.S === "completed",
    "matchingCompletedTombstoneOnly",
  );
  report.finalTombstone = control[0];
  check(query("P#" + project, o.protectedTable).length === 0, "protectedEmpty");
  check(
    query("P#" + project, o.controlTable, "UPLOAD#").length === 0,
    "noUploadTickets",
  );
  const s3 = aws(["s3api", "list-objects-v2"], {
    Bucket: o.evidenceBucket,
    Prefix: "dev-api/projects/" + project + "/",
    MaxKeys: 1,
  });
  check(!s3.IsTruncated && !(s3.Contents ?? []).length, "s3PrefixEmpty");
  const residual = query(
    "MAINTENANCE#dev-api",
    o.controlTable,
    "RESIDUAL#" + project,
  );
  check(
    residual.length === 1 && residual[0].kind.S === "executionResidual",
    "residualRetained",
  );
  report.residual = residual[0];
  report.accountingAfter = accounting(owners);
  const final = report.accountingAfter.rows.find(
    (r) => r.PK.S === "STAGE#dev-api",
  );
  check(
    final.count.N === "22" && final.month.S === "2026-10",
    "chargedAdmissionsNotReset",
  );
  check(
    (final.storageReservedBytes?.N ?? "0") === "0" &&
      final.pinnedCount.N === "0" &&
      final.pinnedBytes.N === "0" &&
      (final.inflightCount?.N ?? "0") === "0",
    "storagePinsInflightZero",
  );
  check(
    hash(
      report.accountingBefore.rows.filter((r) => r.PK.S !== "STAGE#dev-api"),
    ) ===
      hash(
        report.accountingAfter.rows.filter((r) => r.PK.S !== "STAGE#dev-api"),
      ),
    "allOwnerUsagePreserved",
  );
  check(final.version.N === "54", "quotaVersionMatchesTwoTicketReleasesFrom52");
  report.stage = aws(["dynamodb", "get-item"], {
    TableName: o.controlTable,
    Key: { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } },
    ConsistentRead: true,
  }).Item;
  check(
    hash(report.stage) === hash(retained.observations.stageAfter),
    "stageGenerationCapabilitiesPreserved",
  );
  report.verified = true;
  report.finishedAt = new Date().toISOString();
  report.accountingInterpretation = {
    retainedBeforeInstructor: { count: "20", version: "48", reserved: "0" },
    retainedStop: { count: "22", version: "52", reserved: "3302" },
    current: {
      count: final.count.N,
      version: final.version.N,
      reserved: final.storageReservedBytes?.N ?? "0",
    },
    instructorOwnerChargedSteps: "2",
    attributionBasis:
      "User report plus matching existing deletion/owner daily2 and fresh empty physical partitions/prefix. Missing historical admissions and upload transitions are not reconstructed.",
  };
  save();
  console.log(
    JSON.stringify({
      instructorCleanupVerified: true,
      projectId: project,
      existingOperationObserved: operation,
      stageMonthlyCount: 22,
      quotaVersion: 54,
      reservedBytes: 0,
      noDeletionReplay: true,
    }),
  );
} catch {
  report.stopped = true;
  report.finishedAt = new Date().toISOString();
  save();
  console.log(
    JSON.stringify({
      STOP: true,
      reason:
        "Existing cleanup verification guard failed; inspect safe receipt",
      noWrites: true,
    }),
  );
  process.exitCode = 1;
}
