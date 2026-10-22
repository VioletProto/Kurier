import { spawnSync } from "node:child_process";
import { hash } from "./workflow-assertions-activation.mjs";

// One explicit projection for BOTH ends of every new accounting observation.
// Missing historical fields stay missing: callers must never backfill them.
export const accountingFields = Object.freeze([
  "PK",
  "SK",
  "kind",
  "schemaVersion",
  "version",
  "state",
  "ownerId",
  "projectId",
  "count",
  "month",
  "day",
  "pinnedCount",
  "pinnedBytes",
  "inflightCount",
  "storageReservedBytes",
]);
export function accountingQuery(TableName, pk, prefix = "") {
  return {
    TableName,
    ConsistentRead: true,
    KeyConditionExpression:
      "PK = :pk" + (prefix ? " AND begins_with(SK,:prefix)" : ""),
    ExpressionAttributeValues: {
      ":pk": { S: pk },
      ...(prefix ? { ":prefix": { S: prefix } } : {}),
    },
    ProjectionExpression: accountingFields.map((_, i) => "#a" + i).join(","),
    ExpressionAttributeNames: Object.fromEntries(
      accountingFields.map((f, i) => ["#a" + i, f]),
    ),
  };
}
export function accountingSnapshot(rows) {
  if (
    !Array.isArray(rows) ||
    rows.some(
      (r) =>
        !r.PK?.S ||
        !r.SK?.S ||
        !r.kind?.S ||
        r.schemaVersion?.N !== "1" ||
        !["executionQuota", "project", "projectTombstone", "user"].includes(
          r.kind.S,
        ) ||
        !/^(0|[1-9][0-9]*)$/.test(r.version?.N ?? "") ||
        Object.keys(r).some((k) => !accountingFields.includes(k)),
    )
  )
    throw Error("Incomplete or inconsistent accounting projection");
  const sorted = [...rows].sort((a, b) =>
    (a.PK.S + "\0" + a.SK.S).localeCompare(b.PK.S + "\0" + b.SK.S),
  );
  if (new Set(sorted.map((r) => r.PK.S + "\0" + r.SK.S)).size !== sorted.length)
    throw Error("Duplicate accounting identity");
  return {
    projection: [...accountingFields],
    rows: sorted,
    sha256: hash(sorted),
  };
}
export function compareAccounting(before, after) {
  if (
    hash(before.projection) !== hash(accountingFields) ||
    hash(after.projection) !== hash(accountingFields)
  )
    throw Error("Accounting projections differ");
  const a = accountingSnapshot(before.rows),
    b = accountingSnapshot(after.rows);
  if (a.sha256 !== before.sha256 || b.sha256 !== after.sha256)
    throw Error("Accounting snapshot checksum mismatch");
  return a.sha256 === b.sha256;
}

export function subprocessTimeout(
  deadlineMs,
  nowMs = Date.now(),
  reserveMs = 5_000,
) {
  if (![deadlineMs, nowMs, reserveMs].every(Number.isFinite) || reserveMs < 0)
    throw Error("Invalid subprocess deadline");
  const remaining = Math.floor(deadlineMs - nowMs - reserveMs);
  // spawnSync timeout=0 means unlimited. Never translate expiration into zero.
  if (!Number.isSafeInteger(remaining) || remaining < 1)
    throw Error("Subprocess deadline exhausted");
  return remaining;
}
export function runCollectorOnce(
  command,
  args,
  {
    deadlineMs,
    reserveMs = 5_000,
    now = Date.now,
    run = spawnSync,
    ...options
  },
) {
  return run(command, args, {
    ...options,
    timeout: subprocessTimeout(deadlineMs, now(), reserveMs),
  });
}
