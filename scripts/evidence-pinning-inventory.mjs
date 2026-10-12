// Pure migration preflight. Inspect metadata in memory; never return payloads.
export function pinningInventory(items) {
  const projects = [];
  let quota;
  const retentionFields = new Set([
    "PK",
    "SK",
    "kind",
    "schemaVersion",
    "version",
    "projectId",
    "executionId",
    "state",
    "normalExpiresAt",
    "pinned",
    "pinnedAt",
    "decisionTime",
    "evidenceAvailability",
    "DPK",
    "DSK",
  ]);
  for (const item of items) {
    const kind = item.kind?.S;
    if (
      ["executionRetention", "project", "executionQuota"].includes(kind) &&
      (!/^\d+$/.test(item.version?.N ?? "") ||
        !Number.isSafeInteger(Number(item.version.N)))
    )
      throw new Error("Invalid metadata version; preserve for review.");
    if (item.pinned && typeof item.pinned.BOOL !== "boolean")
      throw new Error("Unknown pin metadata; preserve for review.");
    if (item.pinned?.BOOL === true)
      throw new Error(
        "Existing pins require reviewed reconciliation; no zero initialization.",
      );
    if (kind === "executionRetention") {
      if (
        item.schemaVersion?.N !== "1" ||
        !["live", "deleting"].includes(item.state?.S) ||
        !Number.isFinite(Date.parse(item.normalExpiresAt?.S)) ||
        Object.keys(item).some((k) => !retentionFields.has(k))
      )
        throw new Error("Unknown retention metadata; preserve for review.");
    }
    if (
      kind === "executionJob" &&
      !["completed", "failed"].includes(item.state?.S)
    )
      throw new Error(
        "Unfinished jobs must settle before capability migration.",
      );
    if (
      kind === "executionUpload" &&
      item.state?.S === "writing" &&
      item.writeSettled?.BOOL !== true
    )
      throw new Error(
        "Uncertain upload must settle before capability migration.",
      );
    if (kind === "project" && item.SK?.S === "META") {
      if (
        item.schemaVersion?.N !== "1" ||
        !["active", "deleting"].includes(item.state?.S) ||
        Number(item.inflightCount?.N ?? "0") !== 0
      )
        throw new Error("Project is not safe for pin counter initialization.");
      projects.push(item);
    }
    if (item.PK?.S === "STAGE#dev-api" && item.SK?.S === "QUOTA") {
      if (kind !== "executionQuota" || item.schemaVersion?.N !== "1")
        throw new Error("Unknown stage quota; preserve for review.");
      quota = item;
    }
    if (
      (item.pinnedCount && item.pinnedCount.N !== "0") ||
      (item.pinnedBytes && item.pinnedBytes.N !== "0")
    )
      throw new Error(
        "Nonzero or invalid pin counters require reconciliation.",
      );
  }
  if (!quota && items.some((i) => i.kind?.S === "executionUpload"))
    throw new Error(
      "Missing storage quota with upload obligations; preserve for review.",
    );
  return { projects, quota };
}
