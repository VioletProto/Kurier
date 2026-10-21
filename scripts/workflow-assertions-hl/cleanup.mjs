import {
  accountingSnapshot,
  compareAccounting,
} from "../lib/workflow-assertions-collectors.mjs";
export async function run(c) {
  const {
    ledger: l,
    privateState: p,
    check,
    call,
    aws,
    awsProcess,
    query,
    o,
    hash,
    note,
    save,
    sleep,
  } = c;
  check(
    l.preflightVerified && l.users.length === 1 && l.projects.length === 1,
    "Only current single-owner/project cleanup permitted",
  );
  check(!l.cleanupStartedAt, "Cleanup cannot replay");
  p.cleanup = true;
  check(
    hash(c.marker()) === hash(l.baselineMarker),
    "Stage drift before cleanup",
  );
  if (l.runs.length) await c.settled();
  await c.reconcile();
  const before = c.accounting(),
    quota = before.rows.find((r) => r.PK.S === "STAGE#dev-api");
  check(
    BigInt(quota.count.N) ===
      BigInt(l.baselineStageQuota[0].count.N) + BigInt(l.admittedSteps - 18),
    "Admission accounting mismatch",
  );
  const owner = l.users[0].owner;
  const day = query(
    o.controlTable,
    "U#" + owner,
    "EXECUTION_DAY#",
    before.projection,
  );
  check(
    day.length === (l.admittedSteps === 18 ? 0 : 1) &&
      (!day.length || day[0].count.N === String(l.admittedSteps - 18)),
    "Owner usage mismatch",
  );
  const project = l.projects[0],
    route = "/api/v1/projects/" + project.id;
  const meta = query(
    o.controlTable,
    "P#" + project.id,
    "META",
    before.projection,
  );
  check(
    meta.length === 1 && meta[0].kind.S === "project",
    "Current project accounting missing",
  );
  l.preCleanupAccounting = before;
  l.ownerDailyBefore = accountingSnapshot(day);
  l.projectAccountingBefore = accountingSnapshot(meta);
  l.cleanupStartedAt = new Date().toISOString();
  save();
  const g = await call("GET", route);
  const d = await call("DELETE", route, undefined, 202, {
    "If-Match": g.headers.get("etag"),
  });
  check(
    d.data.deletionOperation?.operationId,
    "Deletion acknowledgment missing",
  );
  project.deletionOperationId = d.data.deletionOperation.operationId;
  save();
  await call("GET", route, undefined, 404);
  project.immediateOwnerDenied = true;
  save();
  const until = Math.min(
    Date.now() + 20 * 60_000,
    Date.parse(l.startedAt) + 120 * 60_000 - 30000,
  );
  let tombstone;
  while (Date.now() < until) {
    c.budget(true);
    const rows = query(o.controlTable, "P#" + project.id, "", [
      "PK",
      "SK",
      "kind",
      "schemaVersion",
      "version",
      "state",
      "ownerId",
      "projectId",
      "deletionOperation",
    ]);
    if (
      rows.length === 1 &&
      rows[0].kind?.S === "projectTombstone" &&
      rows[0].state?.S === "deleted" &&
      rows[0].deletionOperation?.M?.state?.S === "completed"
    ) {
      tombstone = rows[0];
      break;
    }
    await sleep(20000);
  }
  check(tombstone, "Bounded normal cleanup observation exhausted");
  check(
    tombstone.deletionOperation.M.operationId.S ===
      project.deletionOperationId && tombstone.ownerId.S === owner,
    "Deletion identity mismatch",
  );
  check(
    query(o.protectedTable, "P#" + project.id).length === 0,
    "Protected children remain",
  );
  const s3 = aws(["s3api", "list-objects-v2"], {
    Bucket: o.evidenceBucket,
    Prefix: "dev-api/projects/" + project.id + "/",
    MaxKeys: 1,
  });
  check(!s3.IsTruncated && !(s3.Contents ?? []).length, "S3 prefix remains");
  check(
    query(o.controlTable, "P#" + project.id, "UPLOAD#").length === 0,
    "Upload tickets remain",
  );
  const residual = query(
    o.controlTable,
    "MAINTENANCE#dev-api",
    "RESIDUAL#" + project.id,
    ["PK", "SK", "kind", "projectId"],
  );
  check(
    residual.length === 1 && residual[0].kind.S === "executionResidual",
    "Permanent residual registration missing",
  );
  project.cleaned = true;
  project.cleanupProof = {
    tombstone,
    protectedChildren: 0,
    s3Objects: 0,
    uploadTickets: 0,
    residualRegistration: residual[0],
    currentOnly: true,
  };
  save();
  note("physical-project-cleanup-verified", {
    projectId: project.id,
    controlTombstoneOnly: true,
    protectedChildren: 0,
    s3Objects: 0,
    uploadTickets: 0,
  });
  const u = l.users[0];
  aws(["cognito-idp", "admin-user-global-sign-out"], {
    UserPoolId: o.cognitoUserPoolId,
    Username: u.name,
  });
  u.globalSignedOut = true;
  save();
  aws(["cognito-idp", "admin-delete-user"], {
    UserPoolId: o.cognitoUserPoolId,
    Username: u.name,
  });
  u.deleted = true;
  save();
  const absent = awsProcess(["cognito-idp", "admin-get-user"], {
    UserPoolId: o.cognitoUserPoolId,
    Username: u.name,
  });
  check(
    absent.status !== 0 &&
      !absent.error &&
      absent.stderr.toString().includes("(UserNotFoundException)"),
    "Provider absence unconfirmed",
  );
  u.providerAbsent = true;
  p.users[0] = {};
  save();
  const final = c.accounting(),
    q = final.rows.find((r) => r.PK.S === "STAGE#dev-api");
  check(
    q.count.N === quota.count.N && q.month.S === quota.month.S,
    "Cleanup changed admission usage",
  );
  check(
    (q.storageReservedBytes?.N ?? "0") === "0" &&
      q.pinnedCount.N === "0" &&
      q.pinnedBytes.N === "0" &&
      (q.inflightCount?.N ?? "0") === "0",
    "Final storage/pin/inflight mismatch",
  );
  const captureCount = l.results?.flatMap((r) => r.evidence).length ?? 0;
  check(
    BigInt(q.version.N) === BigInt(quota.version.N) + BigInt(captureCount),
    "Cleanup quota version does not match ticket releases",
  );
  check(
    compareAccounting(
      accountingSnapshot(before.rows.filter((r) => r.PK.S !== "STAGE#dev-api")),
      accountingSnapshot(final.rows.filter((r) => r.PK.S !== "STAGE#dev-api")),
    ),
    "Prior owner usage changed",
  );
  check(
    compareAccounting(
      l.ownerDailyBefore,
      accountingSnapshot(
        query(
          o.controlTable,
          "U#" + owner,
          "EXECUTION_DAY#",
          before.projection,
        ),
      ),
    ),
    "New owner usage changed",
  );
  check(
    hash(c.marker()) === hash(l.baselineMarker),
    "Stage changed during cleanup",
  );
  await c.reconcile();
  l.finalAccounting = final;
  l.cleanupComplete = true;
  l.finishedAt = new Date().toISOString();
  note("H-L-cleanup-complete", {
    newSteps: l.admittedSteps - 18,
    newSends: l.targetAudits.length,
    cumulativeSteps: l.admittedSteps,
    cumulativeSends: l.targetAudits.length + 16,
    physicalCleanup: true,
    providerAbsent: true,
    accountingVerified: true,
    cleanupSeconds: (Date.now() - Date.parse(l.cleanupStartedAt)) / 1000,
  });
}
