import { inspectRows } from "../lib/workflow-assertions-activation.mjs";
// Campaign-only helpers; all credentials, captures and request bodies stay RAM.
export async function run(c) {
  const {
    ledger: l,
    privateState: p,
    check,
    aws,
    o,
    note,
    save,
    sleep,
    query,
    call,
    root,
    hash,
    awsProcess,
    createHash,
  } = c;
  c.rule = (kind, path, expected) => ({
    assertionId: c.randomUUID(),
    kind,
    ...(path ? { path } : {}),
    ...(expected !== undefined ? { expected } : {}),
  });
  c.equals = (path, expected) => c.rule("jsonEquals", path, expected);
  c.numeric = (path, decimal) => c.equals(path, { type: "number", decimal });
  c.status = (n) => c.rule("statusEquals", "", n);
  c.exists = (path) => c.rule("jsonExists", path);
  c.rules = (run) => run.steps.map((s) => s.assertions ?? []);
  c.evidence = async (key, schema) => {
    const run = p.runs[key];
    const proofs = [];
    p.evidence[key] = [];
    for (const step of run.steps.filter((s) => s.executionId)) {
      const id = step.executionId;
      const owner = await call(
        "GET",
        root() + "/executions/" + id + "/evidence",
      );
      const evidence = owner.data.evidence;
      check(
        evidence?.schemaVersion === schema,
        "Unexpected evidence schema " + key,
      );
      const snap = query(
        o.controlTable,
        "P#" + l.projects[p.projectIndex ?? 0].id,
        "EXEC#" + id + "#SNAP",
        ["PK", "SK", "objectKey", "checksum", "bytes"],
      );
      check(snap.length === 1, "Immutable manifest missing");
      const s = snap[0];
      check(
        s.objectKey.S.startsWith(
          "dev-api/projects/" +
            l.projects[p.projectIndex ?? 0].id +
            "/evidence/" +
            id +
            "/",
        ),
        "Unscoped immutable object",
      );
      const object = awsProcess([
        "s3",
        "cp",
        "s3://" + o.evidenceBucket + "/" + s.objectKey.S,
        "-",
        "--only-show-errors",
      ]);
      check(object.status === 0, "Immutable object unavailable");
      const raw = object.stdout;
      const sum = createHash("sha256").update(raw).digest("hex");
      check(
        raw.length === Number(s.bytes.N) && sum === s.checksum.S,
        "Stored-byte checksum mismatch",
      );
      const capture = JSON.parse(raw);
      check(
        capture?.schemaVersion === schema,
        "Stored evidence schema mismatch",
      );
      for (const r of Object.values(p.requests).filter(
        (x) => x.mode === "workflow-token",
      )) {
        const token = createHash("sha256")
          .update("kurier-workflow-fixture-v1/" + r.fixture)
          .digest("hex");
        check(
          !raw.includes(Buffer.from(token)) && !owner.raw.includes(token),
          "Synthetic binding escaped evidence",
        );
      }
      if (schema === 1)
        check(
          !Object.hasOwn(capture, "assertions"),
          "Legacy evidence expanded",
        );
      else {
        check(
          capture.assertions?.schemaVersion === 1,
          "Versioned assertion results absent",
        );
        check(
          Object.keys(capture.assertions).every((k) =>
            ["schemaVersion", "state", "results"].includes(k),
          ) &&
            capture.assertions.results.every(
              (r) =>
                Object.keys(r).every((k) =>
                  ["assertionId", "kind", "outcome", "reason"].includes(k),
                ) && /^[a-z_]{1,64}$/.test(r.reason),
            ),
          "Unsafe assertion result fields",
        );
      }
      p.evidence[key].push({
        id,
        objectKey: s.objectKey.S,
        raw,
        evidence: capture,
        sum,
      });
      proofs.push({
        executionId: id,
        position: step.position,
        state: step.state,
        schemaVersion: schema,
        storedBytes: raw.length,
        sha256: sum,
        assertions: capture.assertions ?? null,
      });
    }
    const row = query(
      o.controlTable,
      "P#" + l.projects[p.projectIndex ?? 0].id,
      "RUN#" + run.workflowRunId,
      ["PK", "SK", "kind", "assertionSemanticsVersion", "configurationJSON"],
    ).find((r) => r.SK.S === "RUN#" + run.workflowRunId);
    check(row, "Frozen run missing");
    check(
      schema === 1
        ? !row.assertionSemanticsVersion
        : row.assertionSemanticsVersion?.N === "1",
      "Frozen semantics marker mismatch",
    );
    const frozen = JSON.parse(row.configurationJSON.S);
    check(
      hash(frozen.steps.map((s) => s.assertions ?? [])) === hash(c.rules(run)),
      "Frozen/API assertion DTO mismatch",
    );
    const result = {
      key,
      runId: run.workflowRunId,
      state: run.state,
      semanticsMarker: row.assertionSemanticsVersion?.N ?? null,
      frozenAssertionSha256: hash(frozen.steps.map((s) => s.assertions ?? [])),
      evidence: proofs,
    };
    l.results ??= [];
    l.results.push(result);
    note("immutable-case-proof", result);
    return p.evidence[key].map((v) => v.evidence);
  };
  c.unchangedEvidence = async (key) => {
    for (const e of p.evidence[key]) {
      const raw = awsProcess([
        "s3",
        "cp",
        "s3://" + o.evidenceBucket + "/" + e.objectKey,
        "-",
        "--only-show-errors",
      ]);
      check(
        raw.status === 0 &&
          createHash("sha256").update(raw.stdout).digest("hex") === e.sum,
        "Source immutable evidence changed",
      );
    }
    note("source-evidence-unchanged", { key });
  };
  c.audits = async () => {
    if (!p.fixtureLogGroup) {
      const f = aws([
        "lambda",
        "get-function-configuration",
        "--function-name",
        o.controlledEndpointFunction,
      ]);
      p.fixtureLogGroup = f.LoggingConfig?.LogGroup;
      check(
        /^\/aws\/lambda\/kurier-dev-api-ExecutionControlledEndpointFunction-[a-z]+$/.test(
          p.fixtureLogGroup,
        ),
        "Unscoped target log group",
      );
    }
    const records = [];
    let nextToken;
    const ids = new Set(Object.values(p.requests).map((r) => r.fixture));
    for (let page = 0; page < 20; page++) {
      const r = aws(["logs", "filter-log-events", "--no-paginate"], {
        logGroupName: p.fixtureLogGroup,
        startTime: Date.parse(l.runtimeStartedAt ?? l.startedAt),
        filterPattern: '"kurierWorkflowFixture"',
        limit: 1000,
        ...(nextToken ? { nextToken } : {}),
      });
      for (const e of r.events ?? []) {
        let v;
        try {
          v = JSON.parse(e.message.slice(e.message.indexOf("{")));
        } catch {
          continue;
        }
        check(
          ids.has(v.fixtureId),
          "Unexpected unowned fixture send in campaign window",
        );
        if (ids.has(v.fixtureId))
          records.push({
            fixtureId: v.fixtureId,
            mode: v.mode,
            authorized: v.authorized,
            requestId: v.requestId,
          });
      }
      if (!r.nextToken || r.nextToken === nextToken) {
        nextToken = undefined;
        break;
      }
      nextToken = r.nextToken;
    }
    check(!nextToken, "Incomplete bounded audit pagination");
    check(
      new Set(records.map((r) => r.requestId)).size === records.length,
      "Duplicate target audit request ID",
    );
    check(records.length <= l.expectedSends - 16, "Unexpected target send");
    check(
      records.every(
        (r) => r.mode !== "workflow-consume" || r.authorized === true,
      ),
      "Target binding authorization mismatch",
    );
    l.targetAudits = records;
    save();
    return records;
  };
  c.reconcile = async () => {
    let records;
    for (let i = 0; i < 12; i++) {
      records = await c.audits();
      if (records.length === l.expectedSends - 16) break;
      await sleep(5000);
    }
    check(
      records.length === l.expectedSends - 16,
      "Send-count proof unavailable",
    );
    note("target-sends-verified", {
      replacement: records.length,
      cumulative: records.length + 16,
      expectedCumulative: l.expectedSends,
    });
  };
  c.settled = async () => {
    let rows;
    for (let i = 0; i < 30; i++) {
      rows = query(
        o.controlTable,
        "P#" + l.projects[p.projectIndex ?? 0].id,
        "",
        [
          "PK",
          "SK",
          "kind",
          "state",
          "writeSettled",
          "schemaVersion",
          "version",
          "projectId",
          "ownerId",
          "jobId",
          "executionId",
          "runId",
          "position",
          "recoveryGeneration",
          "objectKey",
          "checksum",
          "bytes",
          "inflightCount",
          "storageReservedBytes",
          "pinnedCount",
          "pinnedBytes",
        ],
      );
      if (
        rows
          .filter((r) => r.kind?.S === "executionOutbox")
          .every((r) => r.state?.S === "published") &&
        rows
          .filter((r) => r.kind?.S === "executionUpload")
          .every(
            (r) => r.state?.S === "published" && r.writeSettled?.BOOL === true,
          )
      )
        break;
      await sleep(5000);
    }
    check(
      rows
        .filter((r) => r.kind?.S === "executionOutbox")
        .every((r) => r.state?.S === "published"),
      "Outbox settlement incomplete",
    );
    check(
      rows
        .filter((r) => r.kind?.S === "executionUpload")
        .every(
          (r) => r.state?.S === "published" && r.writeSettled?.BOOL === true,
        ),
      "Upload settlement incomplete",
    );
    check(
      rows
        .filter((r) =>
          ["execution", "executionJob", "executionWorkflowRun"].includes(
            r.kind?.S,
          ),
        )
        .every((r) => ["completed", "failed"].includes(r.state?.S)),
      "Nonterminal work",
    );
    check(
      query(o.protectedTable, "P#" + l.projects[p.projectIndex ?? 0].id)
        .length === 0,
      "Transient Protected material retained",
    );
    const meta = rows.find((r) => r.SK.S === "META");
    check(meta?.inflightCount?.N === "0", "Inflight accounting mismatch");
    const publications = rows.filter((r) =>
      [
        "executionOutbox",
        "executionJob",
        "execution",
        "executionSnapshot",
        "executionUpload",
      ].includes(r.kind?.S),
    );
    const quota = c.accounting().rows.filter((r) => r.PK.S === "STAGE#dev-api");
    // Reuse the accepted published-outbox identity/terminal/immutable/ticket
    // guard unchanged, on publication records only. This is not stage closure.
    const proof = inspectRows(
      [...publications, ...quota].map((item) => ({
        table: o.controlTable,
        item,
      })),
    );
    l.publicationProof = {
      at: new Date().toISOString(),
      outboxes: rows.filter((r) => r.kind?.S === "executionOutbox").length,
      exactTerminalIdentityAndImmutableTicketProof: true,
    };
    save();
    note("normal-maintenance-settlement", {
      jobs: rows.filter((r) => r.kind?.S === "executionJob").length,
      outboxes: rows.filter((r) => r.kind?.S === "executionOutbox").length,
      protectedRows: 0,
      inflight: 0,
    });
  };
  c.expectPassed = (e) =>
    check(
      e.assertions?.results?.every((r) => r.outcome === "passed"),
      "Expected assertion did not pass",
    );
  c.runCase = async (key, w, options = {}) => {
    await c.submit(key, w, options);
    const result = await c.evidence(key, options.schema ?? 2);
    await c.reconcile();
    return result;
  };
  c.mcp = async (tool, input, status = 200) => {
    await sleep(1200);
    const r = await call(
      "POST",
      "/api/v1/mcp/invocations",
      { tool, input },
      status,
      {},
      p.mcp.token,
    );
    check(!r.raw.includes(p.mcp.token), "MCP token disclosure");
    return r.data;
  };
  note("replacement-helpers-ready", {
    historicalSteps: 18,
    historicalSends: 16,
    liveRequests: 0,
  });
}
