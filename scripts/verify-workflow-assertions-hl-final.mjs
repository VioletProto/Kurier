// Post-cleanup read-only check. No write adapter, identity creation or target requests.
import { readFileSync } from "node:fs";
import {
  snapshot,
  marker,
  awsProcess,
  outputs as o,
  hash,
} from "./verify-read-only-mcp-deployment.mjs";
import { checkPlatform } from "./lib/workflow-assertions-activation.mjs";
import {
  accountingQuery,
  accountingSnapshot,
  compareAccounting,
} from "./lib/workflow-assertions-collectors.mjs";
import {
  coordinatorBudget,
  persistCoordinatorLedger,
} from "./lib/workflow-coordinator-transport.mjs";
if (
  process.argv[2] !== "--verify" ||
  !process.env.KURIER_ATTEMPT_METER_DIRECTORY
)
  throw Error("Explicit metered post-cleanup verification required");
const l = JSON.parse(
  readFileSync(".build/workflow-assertions-hl-replacement/ledger.json"),
);
if (
  !l.cleanupComplete ||
  l.admittedSteps !== 28 ||
  l.expectedSends !== 24 ||
  l.targetAudits.length !== 8 ||
  !l.users[0].providerAbsent ||
  !l.projects[0].cleaned
)
  throw Error("Completed campaign and normal cleanup required");
const budget = coordinatorBudget({
  ledger: l,
  meterPath: process.env.KURIER_ATTEMPT_METER_DIRECTORY + "/attempts.jsonl",
});
budget({ cleanup: true });
const release = JSON.parse(
    readFileSync("docs/development/workflow-assertions-release.json"),
  ),
  baseline = JSON.parse(
    readFileSync("docs/development/workflow-assertions-reader-baseline.json"),
  );
const platform = snapshot();
budget({ cleanup: true });
if (
  !checkPlatform(platform, baseline, release) ||
  hash(platform) !== hash(l.preflight.platform)
)
  throw Error("Final pinned readers/IAM/platform drift");
const before = marker();
if (hash(before) !== hash(l.baselineMarker))
  throw Error("Final stage/generation/capability drift");
const owner = l.users[0].owner,
  rows = [];
const owners = [
  ...new Set([
    ...l.baselineAccounting.rows
      .filter((r) => r.PK.S.startsWith("U#"))
      .map((r) => r.PK.S),
    "U#9c919ed7-f835-42ce-82ad-753fce1ed39e",
    "U#1eab4ab5-4dbe-4eda-b7d0-b1d7db56ce63",
    "U#" + owner,
  ]),
];
for (const [pk, prefix] of [
  ["STAGE#dev-api", "QUOTA"],
  ...owners.map((pk) => [pk, "EXECUTION_DAY#"]),
]) {
  budget({ cleanup: true });
  const r = awsProcess(
    [
      "dynamodb",
      "query",
      "--no-paginate",
      "--cli-connect-timeout",
      "4",
      "--cli-read-timeout",
      "8",
    ],
    accountingQuery(o.controlTable, pk, prefix),
    { deadlineMs: Date.parse(l.startedAt) + 120 * 60000, reserveMs: 30000 },
  );
  if (r.status !== 0 || r.error) throw Error("Final accounting read failed");
  const q = JSON.parse(r.stdout);
  if (q.LastEvaluatedKey) throw Error("Final accounting incomplete");
  rows.push(...(q.Items ?? []));
}
const accounting = accountingSnapshot(rows);
if (
  !compareAccounting(
    l.finalAccounting,
    accountingSnapshot(rows.filter((r) => r.PK.S !== "U#" + owner)),
  ) ||
  !compareAccounting(
    l.ownerDailyBefore,
    accountingSnapshot(rows.filter((r) => r.PK.S === "U#" + owner)),
  )
)
  throw Error("Final old/new charged usage drift");
const quota = rows.find((r) => r.PK.S === "STAGE#dev-api");
if (
  quota.count.N !== "32" ||
  quota.version.N !== "75" ||
  (quota.storageReservedBytes?.N ?? "0") !== "0" ||
  quota.pinnedCount.N !== "0" ||
  quota.pinnedBytes.N !== "0"
)
  throw Error("Final expected accounting mismatch");
const sum = rows
  .filter((r) => r.PK.S.startsWith("U#") && r.month?.S === quota.month.S)
  .reduce((s, r) => s + BigInt(r.count.N), 0n);
if (sum !== 32n) throw Error("Monthly usage does not reconcile");
const after = marker();
if (hash(before) !== hash(after))
  throw Error("Stage changed across final reads");
const result = {
  kind: "kurier-H-L-final-read-only-verification-v1",
  at: new Date().toISOString(),
  platform,
  stageBefore: before,
  stageAfter: after,
  accounting,
  checks: {
    pinnedReadersUnchanged: true,
    iamAndPlatformControlsUnchanged: true,
    stageGenerationCapabilitiesUnchanged: true,
    oldAndNewChargedUsagePreserved: true,
    monthlyUsage32Reconciles: true,
    reservedStoragePinsZero: true,
    quotaVersion75MatchesEightReleases: true,
  },
  resourceWrites: 0,
  stageWrites: 0,
  identityCreation: 0,
  targetRequests: 0,
};
persistCoordinatorLedger(
  ".build/workflow-assertions-hl-replacement/final-verification.json",
  result,
);
console.log(
  JSON.stringify({
    finalReadOnlyVerification: true,
    pinnedReaders: 4,
    iamAndControlsPreserved: true,
    stageVersion: 15,
    monthlyUsage: 32,
    quotaVersion: 75,
    reservedBytes: 0,
    newOwnerDayCount: 10,
  }),
);
