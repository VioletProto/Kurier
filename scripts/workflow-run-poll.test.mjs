import assert from "node:assert/strict";
import test from "node:test";
import {
  createWorkflowRunPoller,
  createCoordinatorRunReader,
  WorkflowPollStop,
} from "./lib/workflow-run-poll.mjs";

const projectId = "00000000-0000-4000-8000-000000000001";
const runId = "00000000-0000-4000-8000-000000000002";
const run = (state) => ({ workflowRunId: runId, state });
const retryable = (retryAfter) => ({
  status: 503,
  error: { code: "service_unavailable" },
  headers: new Headers(
    retryAfter === undefined ? {} : { "Retry-After": retryAfter },
  ),
});
const success = (state = "completed") => ({
  status: 200,
  data: { run: run(state) },
  headers: new Headers(),
});

function fixture(responses, options = {}) {
  let time = 0;
  const epoch = options.epoch ?? Date.UTC(2026, 9, 21);
  const ledger = {
    startedAt: new Date(epoch).toISOString(),
    apiCalls: 0,
    polls: 0,
    admittedSteps: 4,
    expectedSends: 4,
    ...options.ledger,
  };
  const calls = [],
    waits = [],
    persisted = [];
  const clock = { now: () => time, wallNow: () => epoch + time };
  const poll = createWorkflowRunPoller({
    ledger,
    clock,
    wait: async (ms) => {
      waits.push(ms);
      time += options.oversleep ?? ms;
      options.afterWait?.();
    },
    checkBudget: async (plan) => options.checkBudget?.(plan),
    persist: async () => {
      persisted.push({ ...ledger });
      await options.persist?.();
    },
    requestOnce: async (request) => {
      // Counts must be durably recorded before every exchange, including503.
      assert.equal(persisted.length, calls.length + 1);
      assert.equal(
        ledger.apiCalls,
        (options.ledger?.apiCalls ?? 0) + calls.length + 1,
      );
      assert.equal(
        ledger.polls,
        (options.ledger?.polls ?? 0) + calls.length + 1,
      );
      assert.equal(request.method, "GET");
      assert.equal(
        request.route,
        `/api/v1/projects/${projectId}/workflow-runs/${runId}`,
      );
      assert.deepEqual(Object.keys(request).sort(), [
        "method",
        "route",
        "signal",
      ]);
      assert.ok(request.signal instanceof AbortSignal);
      calls.push({ at: time });
      const response = responses.shift();
      return typeof response === "function" ? response(clock) : response;
    },
  });
  return {
    ledger,
    calls,
    waits,
    persisted,
    clock,
    poll: (options = {}) =>
      poll({ projectId, runId, initial: run("queued"), ...options }),
  };
}

const rejects = async (promise, code) => {
  await assert.rejects(promise, (e) => {
    assert.ok(e instanceof WorkflowPollStop);
    assert.equal(e.code, code);
    return true;
  });
};

test("503 retries honor initial202 hint, response hint and exponential backoff without admission", async () => {
  const f = fixture([retryable("7"), retryable(), success()]);
  assert.equal((await f.poll({ retryAfter: "10" })).state, "completed");
  assert.deepEqual(f.waits, [10_000, 7_000, 4_000]);
  assert.deepEqual(
    f.calls.map((r) => r.at),
    [10_000, 17_000, 21_000],
  );
  assert.equal(f.ledger.apiCalls, 3);
  assert.equal(f.ledger.polls, 3);
  assert.equal(f.ledger.admittedSteps, 4);
  assert.equal(f.ledger.expectedSends, 4);
});

test("all supported HTTP-date forms and past dates retain minimum poll spacing", async () => {
  for (const format of [
    (d) => d.toUTCString(),
    (d) => d.toUTCString().replace("Wed, 21 Oct 2026", "Wednesday, 21-Oct-26"),
    (d) => `Wed Oct 21 ${d.toUTCString().slice(17, 25)} 2026`,
  ]) {
    const f = fixture([
      (clock) => retryable(format(new Date(clock.wallNow() + 9_000))),
      success(),
    ]);
    await f.poll();
    assert.deepEqual(f.waits, [2_000, 9_000]);
  }
  for (const hint of ["0", "1", "Wed, 21 Oct 2020 00:00:00 GMT"]) {
    const f = fixture([retryable(hint), success()]);
    await f.poll();
    assert.deepEqual(f.waits, [2_000, 2_000]);
  }
});

test("persistent503 stops after three additional attempts and cannot resume", async () => {
  const f = fixture(Array.from({ length: 5 }, () => retryable()));
  await rejects(f.poll(), "retry-limit");
  assert.equal(f.calls.length, 4);
  assert.deepEqual(f.waits, [2_000, 2_000, 4_000, 8_000]);
  await rejects(f.poll(), "latched-stop");
  assert.equal(f.calls.length, 4);
});

test("RFC850 years cannot turn a future wait into an early retry, including century rollover", async () => {
  const future = fixture([
    retryable("Monday, 21-Oct-75 00:00:11 GMT"),
    success(),
  ]);
  await rejects(future.poll(), "retry-after-exceeds-bound");
  assert.equal(future.calls.length, 1);
  const past = fixture([
    retryable("Friday, 21-Oct-94 00:00:11 GMT"),
    success(),
  ]);
  await past.poll();
  assert.deepEqual(past.waits, [2_000, 2_000]);
  const rollover = fixture(
    [retryable("Friday, 01-Jan-00 00:00:04 GMT"), success()],
    { epoch: Date.UTC(2099, 11, 31, 23, 59, 57) },
  );
  await rollover.poll();
  assert.deepEqual(rollover.waits, [2_000, 5_000]);
});

test("successful nonterminal reads reset only consecutive backoff, never total attempts", async () => {
  const f = fixture([
    retryable(),
    retryable(),
    success("running"),
    retryable(),
    success("failed"),
  ]);
  assert.equal((await f.poll()).state, "failed");
  assert.deepEqual(f.waits, [2_000, 2_000, 4_000, 2_000, 2_000]);
  assert.equal(f.ledger.apiCalls, 5);
});

test("only allowlisted503 envelopes retry; auth, deletion, CAS, throttling and other failures stop", async () => {
  for (const status of [
    301, 400, 401, 403, 404, 409, 412, 428, 429, 500, 502, 504, 599,
  ]) {
    const f = fixture([{ ...retryable("2"), status }, success()]);
    await rejects(f.poll(), "non-retryable-response");
    assert.equal(f.calls.length, 1);
  }
  for (const error of [
    null,
    {},
    { code: "evidence_unavailable" },
    { code: "internal_error" },
  ]) {
    const f = fixture([{ ...retryable(), error }, success()]);
    await rejects(f.poll(), "non-retryable-response");
    assert.equal(f.calls.length, 1);
  }
});

test("transport, timeout and JSON-parse uncertainty is never retried or exposed", async () => {
  for (const error of [
    new TypeError("private-token"),
    new SyntaxError("private-token"),
    new DOMException("private-token", "TimeoutError"),
  ]) {
    const f = fixture([
      () => {
        throw error;
      },
      success(),
    ]);
    await rejects(f.poll(), "get-outcome-uncertain");
    assert.equal(f.calls.length, 1);
    await rejects(f.poll(), "latched-stop");
  }
});

test("invalid or excessive Retry-After stops without truncation or early retry", async () => {
  for (const hint of [
    "",
    "2.5",
    "-1",
    "tomorrow",
    "2, 3",
    "bogus",
    "31",
    "9".repeat(128),
    "Wed, 21 Oct 2026 00:01:00 GMT",
  ]) {
    const f = fixture([retryable(hint), success()]);
    await assert.rejects(f.poll(), WorkflowPollStop);
    assert.equal(f.calls.length, 1);
    assert.deepEqual(f.waits, [2_000]);
  }
  const f = fixture([success()]);
  await rejects(f.poll({ retryAfter: "31" }), "retry-after-exceeds-bound");
  assert.equal(f.calls.length, 0);
});

test("35 total GET ceiling includes all attempts and successes", async () => {
  const f = fixture(Array.from({ length: 36 }, () => success("running")));
  await rejects(f.poll(), "run-poll-limit");
  assert.equal(f.calls.length, 35);
  assert.equal(f.ledger.polls, 35);
});

test("shared500 API and240 poll limits are not reset by retry", async () => {
  for (const [ledger, code] of [
    [{ apiCalls: 499 }, "api-limit"],
    [{ polls: 239 }, "poll-limit"],
  ]) {
    const f = fixture([retryable(), success()], { ledger });
    await rejects(f.poll(), code);
    assert.equal(f.calls.length, 1);
    assert.deepEqual(f.waits, [2_000]);
  }
});

test("campaign clock preserves20-minute cleanup reserve; per-run deadline stops before waiting", async () => {
  const f = fixture([success()], {
    ledger: {
      startedAt: new Date(Date.UTC(2026, 9, 20, 22, 20)).toISOString(),
    },
  });
  await rejects(f.poll(), "time-limit");
  assert.equal(f.calls.length, 0);
  const g = fixture(
    Array.from({ length: 6 }, () => ({
      ...success("running"),
      headers: new Headers({ "Retry-After": "30" }),
    })),
  );
  await rejects(g.poll(), "time-limit");
  assert.equal(g.calls.length, 4);
  assert.deepEqual(g.waits, [2_000, 30_000, 30_000, 30_000]);
});

test("authoritative meter reserve/stop is checked again after waiting", async () => {
  let stop = false;
  const f = fixture([success()], {
    afterWait: () => {
      stop = true;
    },
    checkBudget: () => {
      if (stop) throw Error("AWS cleanup reserve reached");
    },
  });
  await rejects(f.poll(), "budget-or-persistence-failure");
  assert.equal(f.calls.length, 0);
  const g = fixture([success()], { oversleep: 121_000 });
  await rejects(g.poll(), "time-limit");
  assert.equal(g.calls.length, 0);
});

test("persistence failure stops before transmission with conservative count retained", async () => {
  const f = fixture([success()], {
    persist: () => {
      throw Error("private-disk-error");
    },
  });
  await rejects(f.poll(), "budget-or-persistence-failure");
  assert.equal(f.calls.length, 0);
  assert.equal(f.ledger.apiCalls, 1);
  assert.equal(f.ledger.polls, 1);
});

test("cannot route submission, rerun, closure, activation or a body through polling", async () => {
  for (const extra of [
    { method: "POST" },
    { route: `/api/v1/projects/${projectId}/workflows/${runId}/runs` },
    { action: "close" },
    { action: "activate" },
    { body: {} },
    { runId: runId + "/rerun" },
  ]) {
    const f = fixture([success()]);
    await rejects(f.poll(extra), "invalid-poll-scope");
    assert.equal(f.calls.length, 0);
  }
  const f = fixture([success()], { ledger: { stoppedAt: "retained-stop" } });
  await rejects(f.poll(), "latched-stop");
  assert.equal(f.calls.length, 0);
});

test("initial terminal snapshot needs no GET; malformed or mismatched read never retries", async () => {
  const f = fixture([]);
  await f.poll({ initial: run("completed") });
  assert.equal(f.calls.length, 0);
  for (const bad of [
    undefined,
    run("unknown"),
    { ...run("completed"), workflowRunId: projectId },
  ]) {
    const g = fixture([{ ...success(), data: { run: bad } }, success()]);
    await rejects(g.poll(), "invalid-run-response");
    assert.equal(g.calls.length, 1);
  }
});

test("coordinator adapter cannot poll an uncertain submission or a mismatched locator", async () => {
  let calls = 0;
  const makeReader = () =>
    createCoordinatorRunReader({
      ledger: { startedAt: new Date().toISOString(), apiCalls: 1, polls: 0 },
      exchangeOnce: async () => {
        calls++;
      },
      budget: () => {},
      persist: () => {},
    });
  for (const submission of [
    undefined,
    { status: 503, error: { code: "service_unavailable" } },
    { status: 200, data: { run: run("queued") } },
    { status: 202, data: {} },
  ]) {
    const reader = makeReader();
    await rejects(
      reader.pollSubmitted(projectId, submission),
      "submission-not-acknowledged",
    );
    await rejects(
      reader.pollSubmitted(projectId, {
        status: 202,
        data: { run: run("queued") },
      }),
      "latched-stop",
    );
  }
  await rejects(
    makeReader().pollSubmitted(projectId, {
      status: 202,
      data: { run: run("queued") },
      headers: new Headers({
        Location: `/api/v1/projects/${runId}/workflow-runs/${runId}`,
      }),
    }),
    "submission-locator-mismatch",
  );
  assert.equal(calls, 0);
});

test("only one poll is in flight and shared stop during a wait prohibits the GET", async () => {
  let release;
  const ready = new Promise((resolve) => {
    release = resolve;
  });
  const f = fixture([success()], { checkBudget: () => ready });
  const first = f.poll();
  await rejects(f.poll(), "poll-in-flight");
  release();
  await first;
  assert.equal(f.calls.length, 1);
  let ledger;
  const g = fixture([success()], {
    afterWait: () => {
      ledger.stoppedAt = "explicit-stop";
    },
  });
  ledger = g.ledger;
  await rejects(g.poll(), "invalid-or-stopped-budget");
  assert.equal(g.calls.length, 0);
});
