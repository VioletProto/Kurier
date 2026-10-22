import { run as browser } from "./browser.mjs";
export async function run(c) {
  const {
    privateState: p,
    ledger: l,
    check,
    call,
    root,
    definition,
    note,
    hash,
    query,
    o,
  } = c;
  check(
    p.projectIndex === 0 && l.users.length === 1 && l.projects.length === 1,
    "Single continuation owner/project required",
  );
  const values = await c.request("values", "assertions-values");
  const notfound = await c.request("notfound", "assertions-not-found");
  // These expected rejections precede admission and cannot dispatch a target.
  for (const expected of [
    0.1,
    { type: "number", decimal: "1e-309" },
    { type: "number", decimal: "9007199254740992" },
    { type: "number", decimal: "0." + "0".repeat(127) },
    { type: "number", decimal: "1e309" },
  ]) {
    const steps = [c.step(values), c.step(values)];
    steps[0].assertions = [c.equals("$.short", expected)];
    await call(
      "POST",
      root() + "/workflows",
      { name: "Rejected numeric DTO", steps },
      400,
    );
  }
  let w = await definition("H", values, values, { assertions: [[], []] });
  const project = (await call("GET", root())).data.project;
  for (const mode of ["author", "rename-before"]) {
    p.browserAction = { p: project, w: p.workflows.H, mode };
    await browser(c);
  }
  w = p.workflows.H;
  const rules = hash(w.steps.map((s) => s.assertions));
  let e = await c.runCase("H", w);
  e.forEach(c.expectPassed);
  check(hash(c.rules(p.runs.H)) === rules, "Freeze changed exact expectations");
  // Sanitized body remains JSON text: do not parse its numeric tokens into JS numbers.
  for (const item of e) {
    const text = item.response.body.text;
    check(
      typeof text === "string" &&
        text.includes("0.10000000000000001") &&
        text.includes("1E-01") &&
        text.includes("1e-308") &&
        text.includes("9007199254740991"),
      "Literal response number tokens lost",
    );
  }
  p.browserAction = { p: project, w: p.workflows.H, mode: "rename-after" };
  await browser(c);
  check(
    hash(p.workflows.H.steps.map((s) => s.assertions)) === rules,
    "Unrelated browser edit changed expectations",
  );
  e = await c.runCase("I", p.workflows.H, { source: p.runs.H.workflowRunId });
  e.forEach(c.expectPassed);
  check(
    p.runs.I.workflowRevision === p.runs.H.workflowRevision &&
      hash(c.rules(p.runs.I)) === rules,
    "Whole-run rerun changed frozen expectations",
  );
  await c.unchangedEvidence("H");
  w = await definition("J", values, values, {
    assertions: [[c.numeric("$.short", "0.10000000000000001")], []],
  });
  e = await c.runCase("J", w, { state: "failed", sends: 1 });
  check(
    e[0].assertions.results.some((r) => r.reason === "value_mismatch") &&
      p.runs.J.steps[1].state === "skipped",
    "Distinct decimal expectations collapsed",
  );
  w = await definition("K", values, notfound, {
    assertions: [
      [
        c.exists("$.exists"),
        c.equals("$.exists", null),
        c.equals("$.flag", true),
        c.equals("$.text", "public"),
      ],
      [c.status(404), c.equals("$.unavailable", true)],
    ],
  });
  e = await c.runCase("K", w, { state: "failed", sends: 2 });
  e.forEach(c.expectPassed);
  check(
    p.runs.K.steps[0].state === "completed" &&
      p.runs.K.steps[1].state === "failed",
    "404 became transport success",
  );
  w = await definition("L", values, values, {
    assertions: [[c.equals("$.missing", null)], []],
  });
  e = await c.runCase("L", w, { state: "failed", sends: 1 });
  check(
    e[0].assertions.results.some((r) => r.reason === "path_missing") &&
      p.runs.L.steps[1].state === "skipped",
    "Missing collapsed into null",
  );
  await c.settled();
  await c.reconcile();
  check(
    l.admittedSteps === 28 && l.expectedSends === 24,
    "A-L cumulative counters drift",
  );
  note("H-L-complete", {
    cumulativeAdmitted: 28,
    cumulativeAuditedSends: 24,
    continuationSteps: 10,
    continuationSends: 8,
    reserveAdmittedSteps: 0,
    maximumCumulativeSteps: 28,
  });
}
