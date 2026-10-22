export const actions = Object.freeze([
  "preflight.mjs",
  "init.mjs",
  "helpers.mjs",
  "campaign-decimal.mjs",
  "cleanup.mjs",
  "stopped-cleanup.mjs",
]);
export function checkAdmission(ledger, key, workflow, options = {}) {
  const cases = [
    ["H", "completed", 2],
    ["I", "completed", 2],
    ["J", "failed", 1],
    ["K", "failed", 2],
    ["L", "failed", 1],
  ];
  const index = ledger.runs.length,
    expected = cases[index];
  if (
    !ledger.preflightVerified ||
    ledger.users.length !== 1 ||
    ledger.projects.length !== 1 ||
    !expected ||
    key !== expected[0] ||
    (options.state ?? "completed") !== expected[1] ||
    (options.sends ?? 2) !== expected[2] ||
    workflow.steps.length !== 2 ||
    ledger.admittedSteps !== 18 + 2 * index ||
    ledger.admittedSteps + 2 > 28 ||
    ledger.expectedSends + (options.sends ?? 2) > 24 ||
    Boolean(options.source) !== (key === "I")
  )
    throw Error("H-L admission guard; no replay or reserve");
  return true;
}

export function refreshedBaseline(report) {
  if (
    report?.kind !== "kurier-existing-instructor-deletion-observation-v1" ||
    report.verified !== true ||
    report.stopped ||
    !Object.values(report.checks ?? {}).length ||
    !Object.values(report.checks).every((v) => v === true)
  )
    throw Error("Verified existing instructor cleanup required");
  const quota = report.accountingAfter?.rows?.find(
    (r) => r.PK.S === "STAGE#dev-api",
  );
  if (
    quota?.count?.N !== "22" ||
    quota.version?.N !== "54" ||
    (quota.storageReservedBytes?.N ?? "0") !== "0" ||
    quota.pinnedCount?.N !== "0" ||
    quota.pinnedBytes?.N !== "0" ||
    report.finalTombstone?.deletionOperation?.M?.operationId?.S !==
      report.deletionOperationId ||
    report.finalTombstone?.deletionOperation?.M?.state?.S !== "completed" ||
    report.finalTombstone?.projectId?.S !== report.projectId
  )
    throw Error("Refreshed accounting/deletion identity invalid");
  return {
    stage: report.stage,
    accounting: report.accountingAfter,
    owners: [
      ...new Set(
        report.accountingAfter.rows
          .filter((r) => r.PK.S.startsWith("U#"))
          .map((r) => r.PK.S),
      ),
    ],
  };
}
