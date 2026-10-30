import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import {
  execute,
  ActivationGuardError,
  inspectRows,
  transition,
  checkMarker,
  checkPlatform,
  hash as awaitableHash,
} from "./lib/project-environments-activation.mjs";
const S = (S) => ({ S }),
  N = (N) => ({ N: String(N) });
const names = [
  "UsersProjectsApi",
  "EmptyProjectCleanup",
  "CloudExecutionWorker",
  "ExecutionControlledEndpoint",
];
const commit = "a".repeat(40),
  manifestSha256 = "b".repeat(64);
function fixture({ closed = false, deployed = false } = {}) {
  let m = {
    PK: S("STAGE#dev-api"),
    SK: S("META"),
    kind: S("stage"),
    schemaVersion: N(1),
    version: N(13),
    state: S(closed ? "workflow-migration" : "active"),
    recoveryGeneration: S("preserved-generation"),
    savedRequestsSchemaVersion: N(8),
  };
  for (const k of [
    "workflowAssertions",
    "mcpReadOnly",
    "sequentialWorkflows",
    "cloudExecutions",
    "executionInputs",
    "protectedSecrets",
    "evidencePinning",
    "localAgentExecutions",
  ])
    m[k + "SchemaVersion"] = N(1);
  const rows = [
    {
      table: "control",
      item: {
        PK: S("STAGE#dev-api"),
        SK: S("QUOTA"),
        kind: S("executionQuota"),
        schemaVersion: N(1),
        version: N(5),
        month: S("2026-10"),
        count: N(2),
        pinnedCount: N(0),
        pinnedBytes: N(0),
      },
    },
  ];
  const baseline = {
    account: "747336059622",
    region: "us-east-2",
    stage: "dev-api",
    controls: { quota: 10, unreserved: 10, workerMax: 2 },
    functions: names.map((resource) => ({
      resource,
      codeSha256: "old",
      iamHash: "unchanged",
      state: "Active",
      update: "Successful",
    })),
  };
  const release = {
    bindingValidation: { authenticated: true },
    packages: names.map((name) => ({ name, zipSha256: "c".repeat(64) })),
  };
  const platform = structuredClone(baseline);
  if (deployed)
    for (const f of platform.functions)
      f.codeSha256 = Buffer.from("c".repeat(64), "hex").toString("base64");
  const calls = { writes: [], markerReads: 0 };
  const adapter = {
    controlTable: "control",
    identity: async () => ({
      Account: "747336059622",
      Arn: "arn:aws:iam::747336059622:user/operator",
    }),
    marker: async () => {
      calls.markerReads++;
      return structuredClone(m);
    },
    platform: async () => structuredClone(platform),
    frontendReady: async () => true,
    rows: async () => structuredClone(rows),
    queueDepth: async () => ({ visible: "0", inflight: "0", delayed: "0" }),
    transact: async (actions) => {
      calls.writes.push(actions);
      m = transition(m, closed ? "activate" : "close").next;
    },
  };
  const options = {
    release,
    baseline,
    reviewCommit: commit,
    manifestSha256,
    now: () => 1000000,
    closure: closed
      ? { closedAt: 999999, deadline: 999999 + 1800000, markerSha256: "" }
      : undefined,
  };
  if (closed) options.closure.markerSha256 = awaitableHash(m);
  const approve = async (action) => {
    const before = await execute(adapter, options);
    return {
      ...options,
      action,
      writeApproval: commit + ":" + action,
      approval: {
        action,
        reviewCommit: commit,
        manifestSha256,
        markerSha256: before.markerSha256,
        accountingSha256: before.accountingSha256,
        closureSha256:
          action === "activate" ? awaitableHash(options.closure ?? null) : null,
      },
    };
  };
  return {
    adapter,
    options,
    approve,
    rows,
    platform,
    baseline,
    release,
    calls,
    marker: () => m,
  };
}
test("default inspection makes zero writes and distinguishes undeployed readers", async () => {
  const f = fixture();
  const result = await execute(f.adapter, f.options);
  assert.equal(result.writes, 0);
  assert.equal(result.readersReady, false);
  assert.equal(f.calls.writes.length, 0);
});
test("CLI rejects unknown or extra modes before any AWS operation", () => {
  for (const args of [
    ["--deploy"],
    ["--inspect", "extra"],
    ["--close"],
    ["--activate", "a", "b"],
  ]) {
    assert.throws(
      () =>
        execFileSync(
          process.execPath,
          ["scripts/activate-project-environments.mjs", ...args],
          { cwd: new URL("../", import.meta.url), stdio: "pipe" },
        ),
      (e) =>
        e.status === 1 &&
        e.stderr.toString().includes("Use no arguments/--check-local"),
    );
  }
});
test("closure writes one guarded transaction, retains generation/counters and does not activate", async () => {
  const f = fixture();
  const o = await f.approve("close");
  const old = structuredClone(f.marker());
  const result = await execute(f.adapter, o);
  assert.equal(result.state, "workflow-migration");
  assert.equal(result.version, "14");
  assert.equal(result.activated, false);
  assert.equal(f.calls.writes.length, 1);
  assert.deepEqual(f.marker(), {
    ...old,
    state: S("workflow-migration"),
    version: N(14),
  });
  assert.equal(f.calls.writes[0].length, 2);
  assert.equal(f.calls.writes[0][1].ConditionCheck.Key.SK.S, "QUOTA");
  assert.ok(
    f.calls.writes[0][0].Update.ConditionExpression.includes(
      "attribute_not_exists(#environments)",
    ),
  );
});
test("activation requires closed stage and deployed readers, writes only intended fields", async () => {
  const f = fixture({ closed: true, deployed: true });
  const o = await f.approve("activate");
  const old = structuredClone(f.marker());
  const result = await execute(f.adapter, o);
  assert.equal(result.activated, true);
  assert.deepEqual(f.marker(), {
    ...old,
    state: S("active"),
    version: N(14),
    savedRequestsSchemaVersion: N(9),
    projectEnvironmentsSchemaVersion: N(1),
    protectedSecretsSchemaVersion: N(2),
    localAgentExecutionsSchemaVersion: N(2),
  });
  assert.equal(f.calls.writes.length, 1);
});
test("undeployed compatible readers block activation", async () => {
  const f = fixture({ closed: true });
  const o = await f.approve("activate");
  await assert.rejects(execute(f.adapter, o), /Compatible readers/);
  assert.equal(f.calls.writes.length, 0);
});
test("backend code alone cannot activate before compatible frontend is ready", async () => {
  const f = fixture({ closed: true, deployed: true });
  f.adapter.frontendReady = async () => false;
  const options = await f.approve("activate");
  await assert.rejects(execute(f.adapter, options), /Compatible readers/);
  assert.equal(f.calls.writes.length, 0);
});
for (const field of [
  "action",
  "reviewCommit",
  "manifestSha256",
  "markerSha256",
  "accountingSha256",
])
  test(`approval ${field} mismatch blocks all writes`, async () => {
    const f = fixture();
    const o = await f.approve("close");
    o.approval[field] = "wrong";
    await assert.rejects(execute(f.adapter, o), /approval/);
    assert.equal(f.calls.writes.length, 0);
  });
test("missing explicit environment approval blocks writes", async () => {
  const f = fixture();
  const o = await f.approve("close");
  delete o.writeApproval;
  await assert.rejects(execute(f.adapter, o), /approval/);
  assert.equal(f.calls.writes.length, 0);
});
test("version increments use BigInt beyond Number precision", () => {
  const f = fixture();
  f.marker().version = N("9007199254740993");
  assert.equal(
    transition(f.marker(), "close").next.version.N,
    "9007199254740994",
  );
});
for (const [key, value] of [
  ["savedRequestsSchemaVersion", N(9)],
  ["workflowAssertionsSchemaVersion", N(0)],
  ["mcpReadOnlySchemaVersion", N(2)],
  ["recoveryGeneration", S("")],
  ["version", N(-1)],
  ["unexpected", S("x")],
])
  test(`stage shape rejects ${key}`, () => {
    const f = fixture();
    f.marker()[key] = value;
    assert.throws(() => checkMarker(f.marker()), /Unexpected stage/);
  });
test("every existing capability is included in stage CAS", () => {
  const f = fixture();
  const p = transition(f.marker(), "close");
  for (const [alias, k] of Object.entries(p.Update.ExpressionAttributeNames))
    if (k.endsWith("SchemaVersion") && k !== "projectEnvironmentsSchemaVersion")
      assert.ok(p.Update.ConditionExpression.includes(alias + " = "));
});
test("root/wrong account identity blocks writes", async () => {
  for (const who of [
    { Account: "747336059622", Arn: "arn:aws:iam::747336059622:root" },
    { Account: "123456789012", Arn: "arn:aws:iam::123456789012:user/operator" },
  ]) {
    const f = fixture();
    f.adapter.identity = async () => who;
    await assert.rejects(execute(f.adapter, f.options), /non-root/);
    assert.equal(f.calls.writes.length, 0);
  }
});
for (const change of [
  "iam",
  "concurrency",
  "unreviewed-code",
  "duplicate-consumer",
])
  test(`platform ${change} drift fails closed`, () => {
    const f = fixture();
    if (change === "iam")
      f.platform.functions[0].iamHash = "changed-policy-version";
    if (change === "concurrency") f.platform.controls.workerMax = 3;
    if (change === "unreviewed-code")
      f.platform.functions[0].codeSha256 = "unknown";
    if (change === "duplicate-consumer")
      f.platform.functions[0] = f.platform.functions[1];
    assert.throws(() => checkPlatform(f.platform, f.baseline, f.release));
  });
for (const kind of [
  "executionWorkflowInputs",
  "executionWorkflowOutput",
  "executionBindings",
  "executionLocalWait",
  "executionResidual",
  "deletionWork",
  "unknownKind",
])
  test(`inventory blocks ${kind}`, () => {
    const f = fixture();
    f.rows.push({
      table: "protected",
      item: {
        PK: S("P#test"),
        SK: S("buffer"),
        kind: S(kind),
        schemaVersion: N(1),
      },
    });
    assert.throws(() => inspectRows(f.rows));
  });
test("running jobs, uncommitted uploads and bad accounting block writes", () => {
  for (const extra of [
    { kind: S("executionJob"), state: S("running") },
    { kind: S("executionUpload"), state: S("uploaded") },
    {
      kind: S("project"),
      state: S("active"),
      version: N(1),
      pinnedCount: N(-1),
      pinnedBytes: N(0),
    },
  ]) {
    const f = fixture();
    f.rows.push({
      table: "control",
      item: { PK: S("P#test"), SK: S("row"), schemaVersion: N(1), ...extra },
    });
    assert.throws(() => inspectRows(f.rows));
  }
});
test("published upload requires matching terminal job and immutable manifest", () => {
  const f = fixture();
  const base = { PK: S("P#test"), schemaVersion: N(1) };
  const content = {
    objectKey: S("public-fixture-key"),
    checksum: S("f".repeat(64)),
    bytes: N(100),
  };
  for (const item of [
    { ...base, SK: S("JOB#j"), kind: S("executionJob"), state: S("completed") },
    { ...base, SK: S("EXEC#e#SNAP"), kind: S("executionSnapshot"), ...content },
    {
      ...base,
      SK: S("UPLOAD#u"),
      kind: S("executionUpload"),
      state: S("published"),
      writeSettled: { BOOL: true },
      jobId: S("j"),
      executionId: S("e"),
      ...content,
    },
  ])
    f.rows.push({ table: "control", item });
  inspectRows(f.rows);
  f.rows.at(-1).item.bytes = N(101);
  assert.throws(() => inspectRows(f.rows), /proof/);
});
function publishedOutboxFixture(state = "completed") {
  const f = fixture();
  const project = "11111111-1111-4111-8111-111111111111";
  const job = "22222222-2222-4222-8222-222222222222";
  const execution = "33333333-3333-4333-8333-333333333333";
  const base = {
    PK: S("P#" + project),
    schemaVersion: N(1),
    projectId: S(project),
    executionId: S(execution),
  };
  const writer = {
    ...base,
    ownerId: S("owner"),
    jobId: S(job),
    recoveryGeneration: S("preserved-generation"),
  };
  const content = {
    objectKey: S(
      `dev-api/projects/${project}/evidence/${execution}/ticket.json`,
    ),
    checksum: S("f".repeat(64)),
    bytes: N(100),
  };
  const items = {
    job: {
      ...writer,
      SK: S("JOB#" + job),
      kind: S("executionJob"),
      state: S(state),
    },
    anchor: {
      ...writer,
      SK: S("EXEC#" + execution),
      kind: S("execution"),
      state: S(state),
    },
    outbox: {
      ...writer,
      SK: S("OUT#" + job),
      kind: S("executionOutbox"),
      state: S("published"),
    },
    manifest: {
      ...base,
      SK: S("EXEC#" + execution + "#SNAP"),
      kind: S("executionSnapshot"),
      ...content,
    },
    upload: {
      ...writer,
      SK: S("UPLOAD#ticket"),
      kind: S("executionUpload"),
      state: S("published"),
      writeSettled: { BOOL: true },
      ...content,
    },
  };
  for (const item of Object.values(items))
    f.rows.push({ table: "control", item });
  return { ...f, items };
}
for (const state of ["completed", "failed"])
  test(`published outbox with matching ${state} writer and retained publication passes zero-write inspection`, async () => {
    const f = publishedOutboxFixture(state);
    const before = structuredClone(f.rows);
    const result = await execute(f.adapter, f.options);
    assert.equal(result.writes, 0);
    assert.equal(f.calls.writes.length, 0);
    assert.deepEqual(f.rows, before);
    assert.equal(f.marker().state.S, "active");
  });
const invalidSettlement = {
  "pending outbox": (x) => (x.outbox.state = S("pending")),
  "unrecognized sent state": (x) => (x.outbox.state = S("sent")),
  "unknown outbox state": (x) => (x.outbox.state = S("unknown")),
  "running job": (x) => (x.job.state = S("running")),
  "queued job": (x) => (x.job.state = S("queued")),
  "deleted job": (x) => (x.job.state = S("deleted")),
  "unknown job state": (x) => (x.job.state = S("unknown")),
  "wrong job kind": (x) => (x.job.kind = S("request")),
  "wrong job identity": (x) => (x.job.jobId = S("wrong")),
  "wrong execution identity": (x) => (x.job.executionId = S("wrong")),
  "wrong project": (x) => (x.job.projectId = S("wrong")),
  "wrong owner": (x) => (x.job.ownerId = S("other")),
  "wrong generation": (x) => (x.job.recoveryGeneration = S("other")),
  "wrong run": (x) => (x.job.runId = S("other")),
  "wrong position": (x) => (x.job.position = N(1)),
  "inconsistent terminal anchor": (x) => (x.anchor.state = S("failed")),
  "wrong anchor identity": (x) => (x.anchor.jobId = S("wrong")),
  "wrong manifest kind": (x) => (x.manifest.kind = S("request")),
  "wrong manifest identity": (x) => (x.manifest.executionId = S("wrong")),
  "wrong manifest project": (x) => (x.manifest.projectId = S("wrong")),
  "wrong object scope": (x) => (x.manifest.objectKey = S("other/prefix")),
  "invalid checksum": (x) => (x.manifest.checksum = S("invalid")),
  "zero bytes": (x) => (x.manifest.bytes = N(0)),
  "excess bytes": (x) => (x.manifest.bytes = N(4194305)),
  "unsettled upload": (x) => (x.upload.writeSettled = { BOOL: false }),
  "writing upload": (x) => (x.upload.state = S("writing")),
  "mismatched upload checksum": (x) => (x.upload.checksum = S("a".repeat(64))),
  "mismatched upload job": (x) => (x.upload.jobId = S("wrong")),
  "mismatched upload owner": (x) => (x.upload.ownerId = S("other")),
  "mismatched upload generation": (x) =>
    (x.upload.recoveryGeneration = S("other")),
  "unknown record": (x) => (x.upload.kind = S("unknownKind")),
};
for (const [name, mutate] of Object.entries(invalidSettlement))
  test(`outbox settlement rejects ${name} without writing`, async () => {
    const f = publishedOutboxFixture();
    mutate(f.items);
    await assert.rejects(execute(f.adapter, f.options), ActivationGuardError);
    assert.equal(f.calls.writes.length, 0);
    assert.equal(f.marker().version.N, "13");
  });
for (const name of ["job", "anchor", "manifest", "upload"])
  test(`outbox settlement rejects missing ${name} without writing`, async () => {
    const f = publishedOutboxFixture();
    f.rows.splice(
      f.rows.findIndex((r) => r.item === f.items[name]),
      1,
    );
    await assert.rejects(execute(f.adapter, f.options), ActivationGuardError);
    assert.equal(f.calls.writes.length, 0);
  });
test("duplicate publication tickets cannot prove outbox settlement", async () => {
  const f = publishedOutboxFixture();
  f.rows.push({
    table: "control",
    item: { ...f.items.upload, SK: S("UPLOAD#duplicate") },
  });
  await assert.rejects(execute(f.adapter, f.options), ActivationGuardError);
  assert.equal(f.calls.writes.length, 0);
});
test("permanent residual registration requires tombstone, no children and empty-prefix proof", () => {
  const f = fixture();
  const project = "11111111-1111-4111-8111-111111111111";
  f.rows.push({
    table: "control",
    item: {
      PK: S("P#" + project),
      SK: S("META"),
      kind: S("projectTombstone"),
      schemaVersion: N(1),
      state: S("deleted"),
    },
  });
  const residual = {
    table: "control",
    item: {
      PK: S("MAINTENANCE#dev-api"),
      SK: S("RESIDUAL#" + project),
      kind: S("executionResidual"),
      schemaVersion: N(1),
      projectId: S(project),
    },
    residualPrefixEmpty: true,
  };
  f.rows.push(residual);
  inspectRows(f.rows);
  residual.residualPrefixEmpty = false;
  assert.throws(() => inspectRows(f.rows), /residual/);
  residual.residualPrefixEmpty = true;
  f.rows.push({
    table: "protected",
    item: {
      PK: S("P#" + project),
      SK: S("SECRET#x"),
      kind: S("savedSecret"),
      schemaVersion: N(1),
    },
  });
  assert.throws(() => inspectRows(f.rows), /residual/);
});
test("approximate nonempty queue blocks writes", async () => {
  const f = fixture();
  f.adapter.queueDepth = async () => ({
    visible: "1",
    inflight: "0",
    delayed: "0",
  });
  await assert.rejects(execute(f.adapter, f.options), /Queue/);
  assert.equal(f.calls.writes.length, 0);
});
test("drift after approval and during prewrite inspection blocks transaction", async () => {
  for (const drift of ["marker", "accounting", "generation"]) {
    const f = fixture();
    const o = await f.approve("close");
    if (drift === "marker") f.marker().version = N(14);
    if (drift === "generation") f.marker().recoveryGeneration = S("new");
    if (drift === "accounting") f.rows[0].item.count = N(3);
    await assert.rejects(execute(f.adapter, o), /approval/);
    assert.equal(f.calls.writes.length, 0);
  }
});
test("prewrite counter race is detected by second inventory", async () => {
  const f = fixture();
  const o = await f.approve("close");
  let scans = 0;
  const rows = f.adapter.rows;
  f.adapter.rows = async () => {
    if (++scans === 2) f.rows[0].item.count = N(3);
    return rows();
  };
  await assert.rejects(execute(f.adapter, o), /drift/);
  assert.equal(f.calls.writes.length, 0);
});
for (const action of ["close", "activate"])
  test(`lost ${action} ACK never retries/reopens`, async () => {
    const f = fixture({ closed: action === "activate", deployed: true });
    const o = await f.approve(action);
    const transact = f.adapter.transact;
    f.adapter.transact = async (a) => {
      await transact(a);
      throw Error("lost ACK");
    };
    await assert.rejects(execute(f.adapter, o), /lost ACK/);
    assert.equal(f.calls.writes.length, 1);
    assert.equal(
      f.marker().state.S,
      action === "close" ? "workflow-migration" : "active",
    );
  });
test("transaction condition failure has one attempt and no reopen", async () => {
  const f = fixture();
  const o = await f.approve("close");
  f.adapter.transact = async (a) => {
    f.calls.writes.push(a);
    throw Error("TransactionCanceledException");
  };
  await assert.rejects(execute(f.adapter, o), /TransactionCanceled/);
  assert.equal(f.calls.writes.length, 1);
  assert.equal(f.marker().state.S, "active");
});
test("unexpected readback fails without compensating writes", async () => {
  const f = fixture();
  const o = await f.approve("close");
  const transact = f.adapter.transact;
  f.adapter.transact = async (a) => {
    await transact(a);
    f.marker().recoveryGeneration = S("unexpected");
  };
  await assert.rejects(execute(f.adapter, o), /readback/);
  assert.equal(f.calls.writes.length, 1);
});
test("accounting transaction stays within 80 actions", async () => {
  const f = fixture();
  for (let i = 0; i < 79; i++)
    f.rows.push({
      table: "control",
      item: {
        PK: S("P#" + i),
        SK: S("META"),
        kind: S("project"),
        schemaVersion: N(1),
        state: S("active"),
        version: N(1),
        pinnedCount: N(0),
        pinnedBytes: N(0),
      },
    });
  const o = await f.approve("close");
  await assert.rejects(execute(f.adapter, o), /80-action/);
  assert.equal(f.calls.writes.length, 0);
});

test(
  "DynamoDB Local enforces full-marker CAS and atomic accounting conditions",
  { skip: !process.env.KURIER_ACTIVATOR_DDB_ENDPOINT },
  async () => {
    const endpoint = process.env.KURIER_ACTIVATOR_DDB_ENDPOINT;
    assert.match(endpoint, /^http:\/\/127\.0\.0\.1:\d+$/);
    const table = "kurier-activator-test-" + Date.now();
    const db = (op, input) =>
      JSON.parse(
        execFileSync(
          "aws",
          [
            "dynamodb",
            op,
            "--endpoint-url",
            endpoint,
            "--region",
            "us-east-2",
            "--output",
            "json",
            "--cli-input-json",
            JSON.stringify(input),
          ],
          {
            encoding: "utf8",
            stdio: ["pipe", "pipe", "pipe"],
            env: {
              ...process.env,
              AWS_ACCESS_KEY_ID: "localfixture",
              AWS_SECRET_ACCESS_KEY: "localfixture",
              AWS_SESSION_TOKEN: "",
              AWS_MAX_ATTEMPTS: "1",
              AWS_EC2_METADATA_DISABLED: "true",
            },
          },
        ).trim() || "{}",
      );
    db("create-table", {
      TableName: table,
      AttributeDefinitions: [
        { AttributeName: "PK", AttributeType: "S" },
        { AttributeName: "SK", AttributeType: "S" },
      ],
      KeySchema: [
        { AttributeName: "PK", KeyType: "HASH" },
        { AttributeName: "SK", KeyType: "RANGE" },
      ],
      BillingMode: "PAY_PER_REQUEST",
    });
    try {
      const f = fixture({ deployed: true });
      const initial = structuredClone(f.marker());
      const quota = f.rows[0].item;
      const put = (Item) => db("put-item", { TableName: table, Item });
      const read = (SK) =>
        db("get-item", {
          TableName: table,
          Key: { PK: S("STAGE#dev-api"), SK: S(SK) },
          ConsistentRead: true,
        }).Item;
      put(initial);
      put(quota);
      let attempts = 0;
      f.adapter.controlTable = table;
      f.adapter.marker = async () => read("META");
      f.adapter.rows = async () =>
        db("scan", { TableName: table, ConsistentRead: true }).Items.map(
          (item) => ({ table, item }),
        );
      f.adapter.transact = async (TransactItems) => {
        attempts++;
        return db("transact-write-items", { TransactItems });
      };
      const closed = await execute(f.adapter, await f.approve("close"));
      f.options.closure = closed.closure;
      assert.equal(read("META").state.S, "workflow-migration");
      await execute(f.adapter, await f.approve("activate"));
      assert.equal(read("META").savedRequestsSchemaVersion.N, "9");
      assert.deepEqual(read("QUOTA"), quota);
      assert.equal(attempts, 2);
      // A quota update between scan and commit cancels the entire stage transition.
      put(initial);
      attempts = 0;
      const approval = await f.approve("close");
      const transact = f.adapter.transact;
      f.adapter.transact = async (actions) => {
        put({ ...quota, count: N(3), version: N(6) });
        return transact(actions);
      };
      await assert.rejects(execute(f.adapter, approval));
      assert.equal(attempts, 1);
      assert.deepEqual(read("META"), initial);
      assert.equal(read("QUOTA").count.N, "3");
      // A concurrently advanced marker also cancels, without overwriting generation.
      put(quota);
      attempts = 0;
      f.adapter.transact = transact;
      const nextApproval = await f.approve("close");
      f.adapter.transact = async (actions) => {
        put({
          ...initial,
          version: N(14),
          recoveryGeneration: S("concurrent-generation"),
        });
        return transact(actions);
      };
      await assert.rejects(execute(f.adapter, nextApproval));
      assert.equal(attempts, 1);
      assert.equal(read("META").recoveryGeneration.S, "concurrent-generation");
    } finally {
      db("delete-table", { TableName: table });
    }
  },
);

test("environment activation carries all four changes and keeps every retained capability", () => {
  const f = fixture({ closed: true });
  const { next, Update } = transition(f.marker(), "activate");
  assert.equal(next.savedRequestsSchemaVersion.N, "9");
  assert.equal(next.protectedSecretsSchemaVersion.N, "2");
  assert.equal(next.localAgentExecutionsSchemaVersion.N, "2");
  assert.equal(next.projectEnvironmentsSchemaVersion.N, "1");
  assert.equal(next.workflowAssertionsSchemaVersion.N, "1");
  assert.equal(next.recoveryGeneration.S, f.marker().recoveryGeneration.S);
  for (const k of Object.keys(f.marker()))
    assert.ok(Object.values(Update.ExpressionAttributeNames).includes(k));
  assert.ok(
    Update.ConditionExpression.includes("attribute_not_exists(#environments)"),
  );
});
test("existing environment gate or incompatible protected/local reader gate rejects", () => {
  for (const k of [
    "projectEnvironmentsSchemaVersion",
    "protectedSecretsSchemaVersion",
    "localAgentExecutionsSchemaVersion",
  ]) {
    const f = fixture();
    f.marker()[k] = N(2);
    assert.throws(() => checkMarker(f.marker()), ActivationGuardError);
  }
});
test("retained assertion semantics one is compatible, new environment rows/shapes are not", () => {
  const f = fixture();
  assert.doesNotThrow(() =>
    inspectRows([
      ...f.rows,
      {
        table: "control",
        item: {
          PK: S("P#fixture"),
          SK: S("RUN#fixture"),
          kind: S("executionWorkflowRun"),
          schemaVersion: N(2),
          state: S("completed"),
          assertionSemanticsVersion: N(1),
        },
      },
    ]),
  );
  for (const item of [
    { kind: S("environment"), schemaVersion: N(1) },
    { kind: S("savedSecret"), schemaVersion: N(2) },
    { kind: S("request"), schemaVersion: N(3) },
    { kind: S("request"), schemaVersion: N(2), templateSemanticsVersion: N(1) },
  ])
    assert.throws(
      () =>
        inspectRows([
          ...f.rows,
          {
            table: "control",
            item: { PK: S("P#fixture"), SK: S("NEW#fixture"), ...item },
          },
        ]),
      ActivationGuardError,
    );
});
test("unverified archive bindings prohibit both close and activate", async () => {
  for (const action of ["close", "activate"]) {
    const f = fixture({ closed: action === "activate", deployed: true });
    const o = await f.approve(action);
    f.release.bindingValidation.authenticated = false;
    await assert.rejects(execute(f.adapter, o), /Authenticated/);
    assert.equal(f.calls.writes.length, 0);
  }
});
test("closed-window proof cannot be omitted, reset, expired or rebound", async () => {
  for (const change of [
    (o) => delete o.closure,
    (o) => o.closure.deadline++,
    (o) => o.closure.closedAt++,
    (o) => (o.closure.markerSha256 = "wrong"),
    (o) => (o.now = () => o.closure.deadline),
  ]) {
    const f = fixture({ closed: true, deployed: true });
    const o = await f.approve("activate");
    change(o);
    await assert.rejects(execute(f.adapter, o), /closed-window|approval/);
    assert.equal(f.calls.writes.length, 0);
  }
});
test("deadline exhaustion at the transaction boundary makes zero writes", async () => {
  const f = fixture({ closed: true, deployed: true });
  const o = await f.approve("activate");
  let reads = 0;
  o.now = () => (++reads < 4 ? 1000000 : o.closure.deadline);
  await assert.rejects(execute(f.adapter, o), /deadline/);
  assert.equal(f.calls.writes.length, 0);
});

test("closure requires accepted baseline frontend before any write", async () => {
  const f = fixture();
  const o = await f.approve("close");
  f.adapter.frontendReady = async () => false;
  await assert.rejects(execute(f.adapter, o), /baseline frontend/);
  assert.equal(f.calls.writes.length, 0);
});
test("baseline frontend drift on close prewrite reread blocks the transaction", async () => {
  const f = fixture();
  const o = await f.approve("close");
  let calls = 0;
  f.adapter.frontendReady = async () => ++calls === 1;
  await assert.rejects(execute(f.adapter, o), /Prewrite.*drift/);
  assert.equal(f.calls.writes.length, 0);
});
