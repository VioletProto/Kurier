// Pure, dependency-injected coordination. The CLI defaults to inspection.
import { createHash } from "node:crypto";
export const stable = (v) =>
  Array.isArray(v)
    ? v.map(stable)
    : v && typeof v === "object"
      ? Object.fromEntries(
          Object.keys(v)
            .sort()
            .map((k) => [k, stable(v[k])]),
        )
      : v;
export const hash = (v) =>
  createHash("sha256")
    .update(JSON.stringify(stable(v)))
    .digest("hex");
export class ActivationGuardError extends Error {}
const fail = (s) => {
  throw new ActivationGuardError(s);
};
const caps = [
  "mcpReadOnlySchemaVersion",
  "sequentialWorkflowsSchemaVersion",
  "cloudExecutionsSchemaVersion",
  "executionInputsSchemaVersion",
  "protectedSecretsSchemaVersion",
  "evidencePinningSchemaVersion",
  "localAgentExecutionsSchemaVersion",
];
const markerFields = [
  "PK",
  "SK",
  "kind",
  "schemaVersion",
  "state",
  "version",
  "recoveryGeneration",
  "savedRequestsSchemaVersion",
  ...caps,
];
const terminal = (s) => ["completed", "failed", "deleted"].includes(s);
const nonnegative = (v) =>
  typeof v?.N === "string" &&
  /^(0|[1-9][0-9]*)$/.test(v.N) &&
  BigInt(v.N) <= 9223372036854775806n;
export function checkMarker(m) {
  if (
    !m ||
    Object.keys(m).some((k) => !markerFields.includes(k)) ||
    markerFields.some((k) => !m[k] || Object.keys(m[k]).length !== 1) ||
    m.PK.S !== "STAGE#dev-api" ||
    m.SK.S !== "META" ||
    m.kind.S !== "stage" ||
    m.schemaVersion.N !== "1" ||
    !["active", "workflow-migration"].includes(m.state.S) ||
    !nonnegative(m.version) ||
    !m.recoveryGeneration.S ||
    m.savedRequestsSchemaVersion.N !== "7" ||
    caps.some((k) => m[k].N !== "1")
  )
    fail("Unexpected stage shape/state/capabilities; manual review required.");
}
const counterFields = [
  "version",
  "pinnedCount",
  "pinnedBytes",
  "inflightCount",
  "storageReservedBytes",
  "count",
  "month",
];
export const projectionFields = [
  "PK",
  "SK",
  "kind",
  "schemaVersion",
  "state",
  ...counterFields,
  "projectId",
  "jobId",
  "executionId",
  "runId",
  "writeSettled",
  "objectKey",
  "checksum",
  "bytes",
  "recoveryGeneration",
  "assertionSemanticsVersion",
  "activeJob",
];
const knownKinds = new Set([
  "stage",
  "user",
  "identity",
  "project",
  "projectTombstone",
  "deletion",
  "deletionWork",
  "request",
  "requestTombstone",
  "savedSecret",
  "savedSecretTombstone",
  "executionQuota",
  "execution",
  "executionJob",
  "executionWorkflow",
  "executionWorkflowRun",
  "executionWorkflowStep",
  "executionSnapshot",
  "executionRetention",
  "executionOutbox",
  "executionUpload",
  "executionBindings",
  "executionWorkflowInputs",
  "executionWorkflowOutput",
  "executionLocalWait",
  "executionLocalReceipt",
  "executionReceipt",
  "executionAgentScope",
  "executionMCPScope",
  "executionMCPAudit",
  "executionCursor",
  "cleanupCursor",
  "executionResidual",
  "localAgent",
  "localAgentPairing",
  "localAgentSlot",
  "localAgentIssuance",
  "localAgentRate",
  "localAgentCredential",
  "localAgentLookup",
  "mcpCredential",
  "mcpSlot",
  "mcpLookup",
  "mcpIssuance",
  "mcpAudit",
]);
export function inspectRows(rows) {
  const index = new Map();
  for (const r of rows) {
    const key = r.table + "/" + r.item.PK?.S + "/" + r.item.SK?.S;
    if (!r.item.PK?.S || !r.item.SK?.S || index.has(key))
      fail("Incomplete/duplicate inventory key.");
    index.set(key, r.item);
  }
  const accounting = [];
  for (const { table, item: r, residualPrefixEmpty } of rows) {
    const kind = r.kind?.S,
      state = r.state?.S;
    if (!knownKinds.has(kind) || !["1", "2"].includes(r.schemaVersion?.N))
      fail("Unknown inventory kind/schema; inspect without reading payloads.");
    if (
      [
        "deletion",
        "deletionWork",
        "executionLocalWait",
        "executionBindings",
        "executionWorkflowInputs",
        "executionWorkflowOutput",
      ].includes(kind) ||
      r.activeJob?.S ||
      (["execution", "executionJob", "executionWorkflowRun"].includes(kind) &&
        !terminal(state)) ||
      (kind === "executionWorkflowStep" &&
        !["completed", "failed", "skipped", "deleted"].includes(state)) ||
      (kind === "executionOutbox" && state !== "sent") ||
      (kind === "executionRetention" && state !== "live") ||
      (kind === "project" && state !== "active") ||
      (r.assertionSemanticsVersion && r.assertionSemanticsVersion.N !== "0")
    )
      fail("Unsettled work or new assertion data; reconcile separately.");
    if (kind === "executionResidual") {
      const project = r.projectId?.S;
      const tombstone = index.get(table + "/P#" + project + "/META");
      if (
        !/^[a-f0-9-]{36}$/.test(project ?? "") ||
        r.PK.S !== "MAINTENANCE#dev-api" ||
        r.SK.S !== "RESIDUAL#" + project ||
        residualPrefixEmpty !== true ||
        tombstone?.kind?.S !== "projectTombstone" ||
        tombstone?.state?.S !== "deleted" ||
        rows.filter((x) => x.item.PK.S === "P#" + project).length !== 1
      )
        fail(
          "Permanent residual work requires a deleted tombstone, no children and an observed empty S3 prefix.",
        );
    }
    if (kind === "executionUpload") {
      const job = index.get(table + "/" + r.PK.S + "/JOB#" + r.jobId?.S);
      const snap = index.get(
        table + "/" + r.PK.S + "/EXEC#" + r.executionId?.S + "#SNAP",
      );
      if (
        state !== "published" ||
        r.writeSettled?.BOOL !== true ||
        !terminal(job?.state?.S) ||
        !snap ||
        !r.objectKey?.S ||
        !/^[a-f0-9]{64}$/.test(r.checksum?.S ?? "") ||
        !nonnegative(r.bytes) ||
        ["objectKey", "checksum", "bytes"].some(
          (k) => hash(r[k]) !== hash(snap[k]),
        )
      )
        fail("Publication settlement/manifest proof incomplete.");
    }
    if (["project", "executionQuota"].includes(kind)) {
      for (const k of counterFields.filter((k) => k !== "month"))
        if (r[k] && !nonnegative(r[k])) fail("Invalid accounting number.");
      if (
        !nonnegative(r.version) ||
        (kind === "executionQuota" &&
          (!nonnegative(r.count) ||
            !/^\d{4}-(0[1-9]|1[0-2])$/.test(r.month?.S ?? ""))) ||
        (kind === "project" &&
          (!nonnegative(r.pinnedCount) ||
            !nonnegative(r.pinnedBytes) ||
            (r.inflightCount?.N ?? "0") !== "0")) ||
        (r.pinnedCount?.N === "0") !== (r.pinnedBytes?.N === "0")
      )
        fail("Incomplete/inconsistent accounting.");
      accounting.push({ table, item: r });
    }
  }
  if (
    !accounting.some(
      ({ item: r }) =>
        r.PK.S === "STAGE#dev-api" &&
        r.SK.S === "QUOTA" &&
        r.kind.S === "executionQuota",
    )
  )
    fail("Existing stage quota required; never initialize it.");
  accounting.sort((a, b) =>
    (a.table + a.item.PK.S + a.item.SK.S).localeCompare(
      b.table + b.item.PK.S + b.item.SK.S,
    ),
  );
  return {
    accounting,
    accountingSha256: hash(accounting),
    projects: accounting.filter((r) => r.item.kind.S === "project").length,
  };
}
export function checkPlatform(current, baseline, release) {
  if (
    hash(current.controls) !== hash(baseline.controls) ||
    current.account !== "747336059622" ||
    current.region !== "us-east-2" ||
    current.stage !== "dev-api" ||
    current.functions.length !== 4
  )
    fail("Account/plan/quota/concurrency/schedule drift.");
  let readersReady = true;
  const seen = new Set();
  for (const f of current.functions) {
    const old = baseline.functions.find((x) => x.resource === f.resource);
    const pkg = release.preview.packages.find((x) => x.name === f.resource);
    if (
      !old ||
      !pkg ||
      seen.has(f.resource) ||
      f.state !== "Active" ||
      f.update !== "Successful" ||
      hash({ ...f, codeSha256: old.codeSha256 }) !== hash(old)
    )
      fail("Reader configuration/IAM/concurrency drift.");
    seen.add(f.resource);
    const expected = Buffer.from(pkg.sha256, "hex").toString("base64");
    if (f.codeSha256 !== expected) readersReady = false;
    if (![old.codeSha256, expected].includes(f.codeSha256))
      fail("Unreviewed deployed package.");
  }
  return readersReady;
}
export function transition(m, action) {
  checkMarker(m);
  if (
    !["close", "activate"].includes(action) ||
    m.state.S !== (action === "close" ? "active" : "workflow-migration")
  )
    fail("Wrong transition/state; no automatic resume.");
  const next = {
    ...m,
    state: { S: action === "close" ? "workflow-migration" : "active" },
    version: { N: String(BigInt(m.version.N) + 1n) },
  };
  if (action === "activate")
    Object.assign(next, {
      savedRequestsSchemaVersion: { N: "8" },
      workflowAssertionsSchemaVersion: { N: "1" },
    });
  const names = {},
    values = {},
    conditions = [];
  for (const [i, k] of markerFields.entries()) {
    names["#m" + i] = k;
    values[":m" + i] = m[k];
    conditions.push("#m" + i + " = :m" + i);
  }
  names["#assertions"] = "workflowAssertionsSchemaVersion";
  conditions.push("attribute_not_exists(#assertions)");
  names["#s"] = "state";
  names["#v"] = "version";
  values[":to"] = next.state;
  values[":next"] = next.version;
  let expression = "SET #s = :to, #v = :next";
  if (action === "activate") {
    names["#requests"] = "savedRequestsSchemaVersion";
    values[":eight"] = { N: "8" };
    values[":one"] = { N: "1" };
    expression += ", #requests = :eight, #assertions = :one";
  }
  return {
    next,
    Update: {
      Key: { PK: m.PK, SK: m.SK },
      UpdateExpression: expression,
      ConditionExpression: conditions.join(" AND "),
      ExpressionAttributeNames: names,
      ExpressionAttributeValues: values,
    },
  };
}
function counterGuard({ table, item: r }) {
  const names = {},
    values = {},
    conditions = [];
  for (const [i, k] of ["kind", "schemaVersion", ...counterFields].entries()) {
    const key = "#c" + i;
    names[key] = k;
    if (r[k]) {
      values[":c" + i] = r[k];
      conditions.push(key + " = :c" + i);
    } else conditions.push("attribute_not_exists(" + key + ")");
  }
  return {
    ConditionCheck: {
      TableName: table,
      Key: { PK: r.PK, SK: r.SK },
      ConditionExpression: conditions.join(" AND "),
      ExpressionAttributeNames: names,
      ExpressionAttributeValues: values,
    },
  };
}
export async function execute(
  adapter,
  {
    action = "inspect",
    approval,
    release,
    baseline,
    reviewCommit,
    manifestSha256,
    writeApproval,
  } = {},
) {
  if (!["inspect", "close", "activate"].includes(action)) fail("Unknown mode.");
  const identity = await adapter.identity();
  if (
    identity.Account !== "747336059622" ||
    !/^arn:aws:(iam|sts)::747336059622:(user\/|assumed-role\/)/.test(
      identity.Arn ?? "",
    )
  )
    fail("Intended non-root identity required.");
  const before = await adapter.marker();
  checkMarker(before);
  const platform = await adapter.platform();
  const backendReadersReady = checkPlatform(platform, baseline, release);
  const frontendReady = await adapter.frontendReady();
  const readersReady = backendReadersReady && frontendReady;
  const rows = inspectRows(await adapter.rows());
  const depth = await adapter.queueDepth();
  if (
    Object.keys(depth).length !== 3 ||
    Object.values(depth).some((x) => x !== "0")
  )
    fail("Queue is not observed empty; wait and review.");
  const second = await adapter.marker();
  if (hash(second) !== hash(before)) fail("Marker changed during inspection.");
  const report = {
    mode: action,
    markerSha256: hash(before),
    accountingSha256: rows.accountingSha256,
    projects: rows.projects,
    state: before.state.S,
    version: before.version.N,
    generation: before.recoveryGeneration.S,
    readersReady,
    backendReadersReady,
    frontendReady,
    writes: 0,
  };
  if (action === "inspect") return report;
  if (
    !/^[a-f0-9]{40}$/.test(reviewCommit ?? "") ||
    !/^[a-f0-9]{64}$/.test(manifestSha256 ?? "") ||
    !approval ||
    hash(Object.keys(approval).sort()) !==
      hash(
        [
          "action",
          "reviewCommit",
          "manifestSha256",
          "markerSha256",
          "accountingSha256",
        ].sort(),
      ) ||
    approval.action !== action ||
    approval.reviewCommit !== reviewCommit ||
    approval.manifestSha256 !== manifestSha256 ||
    approval.markerSha256 !== report.markerSha256 ||
    approval.accountingSha256 !== report.accountingSha256 ||
    writeApproval !== reviewCommit + ":" + action
  )
    fail(
      "Explicit reviewed action/artifact/marker/accounting approval required.",
    );
  if (action === "activate" && !readersReady)
    fail("Compatible readers not deployed/verified.");
  const plan = transition(before, action);
  const again = inspectRows(await adapter.rows());
  if (
    again.accountingSha256 !== rows.accountingSha256 ||
    hash(await adapter.marker()) !== hash(before) ||
    hash(await adapter.platform()) !== hash(platform) ||
    hash(await adapter.queueDepth()) !== hash(depth) ||
    (action === "activate" && !(await adapter.frontendReady()))
  )
    fail("Prewrite inventory/platform drift.");
  if (rows.accounting.length > 79)
    fail(
      "Accounting exceeds bounded 80-action transaction; separate review required.",
    );
  // Exactly one attempt. Never retry, compensate, reopen or downgrade on error.
  const actions = [
    { Update: { TableName: adapter.controlTable, ...plan.Update } },
    ...rows.accounting.map(counterGuard),
  ];
  if (
    rows.accounting.some((r) => r.table !== adapter.controlTable) ||
    Buffer.byteLength(JSON.stringify(actions)) > 2 * 1024 * 1024
  )
    fail("Accounting table/serialized transaction boundary exceeded.");
  await adapter.transact(actions);
  const durable = await adapter.marker();
  if (hash(durable) !== hash(plan.next))
    fail(
      "Write readback mismatch/uncertain; inspect durable state, never reopen automatically.",
    );
  if (
    inspectRows(await adapter.rows()).accountingSha256 !== rows.accountingSha256
  )
    fail("Postwrite accounting drift; inspect durable state.");
  return {
    ...report,
    state: durable.state.S,
    version: durable.version.N,
    writes: 1,
    activated: action === "activate",
    accountingPreserved: true,
  };
}
