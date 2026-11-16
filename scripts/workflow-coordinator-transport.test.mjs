import assert from "node:assert/strict";
import { test } from "node:test";
import { mkdtempSync, writeFileSync, rmSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import {
  coordinatorBudget,
  persistCoordinatorLedger,
  createCoordinatorTransport,
} from "./lib/workflow-coordinator-transport.mjs";

test("meter failures and unfinished requests block coordinator writes before authentication", async () => {
  const dir = mkdtempSync(tmpdir() + "/kurier-stop-handoff-");
  try {
    const meterPath = dir + "/attempts.jsonl";
    const start = Date.now();
    for (const outcomes of [
      [], // Request is still in flight, including a potentially accepted write.
      [{ event: "outcome", attempt: 2, status: null, transportError: true }],
      [{ event: "outcome", attempt: 2, status: null }],
      [{ event: "outcome", attempt: 2, status: 503 }],
      [
        { event: "outcome", attempt: 2, status: 200 },
        { event: "outcome", attempt: 2, status: 200 },
      ],
      [{ event: "outcome", attempt: 2, status: 200 }, { event: "stop" }],
    ]) {
      const ledger = { startedAt: new Date(start).toISOString(), apiCalls: 0 };
      writeFileSync(
        meterPath,
        [
          { event: "meter-created" },
          {
            event: "attempt",
            sequence: 2,
            at: start / 1000,
            write: true,
            counts: { api: 1, aws: 0, poll: 0 },
          },
          ...outcomes,
        ]
          .map(JSON.stringify)
          .join("\n") + "\n",
      );
      let auth = 0;
      const budget = coordinatorBudget({ ledger, meterPath });
      const transport = createCoordinatorTransport({
        origin: "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com",
        ledger,
        budget,
        persist() {},
        token() {
          auth++;
          throw Error("must not authenticate");
        },
      });
      await assert.rejects(transport.call("POST", "/api/v1/projects"));
      assert.equal(auth, 0);
      assert.equal(ledger.apiCalls, 0);
      assert.throws(() => budget({ cleanup: true }));
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("legacy failed read without a stop event still blocks later coordinator work", () => {
  const dir = mkdtempSync(tmpdir() + "/kurier-legacy-read-");
  try {
    const meterPath = dir + "/attempts.jsonl";
    const start = Date.now();
    const ledger = { startedAt: new Date(start).toISOString(), apiCalls: 0 };
    writeFileSync(
      meterPath,
      [
        { event: "meter-created" },
        {
          event: "attempt",
          sequence: 2,
          at: start / 1000,
          write: false,
          operation: "OPTIONS",
          counts: { api: 1, aws: 0, poll: 0 },
        },
        { event: "outcome", attempt: 2, status: null, transportError: true },
      ]
        .map(JSON.stringify)
        .join("\n") + "\n",
    );
    const budget = coordinatorBudget({ ledger, meterPath });
    assert.throws(() => budget(), /Failed or uncertain/);
    assert.throws(() => budget({ cleanup: true }), /Failed or uncertain/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("a stop during authentication blocks the pending write with no replay", async () => {
  const dir = mkdtempSync(tmpdir() + "/kurier-auth-stop-");
  try {
    const meterPath = dir + "/attempts.jsonl";
    const start = Date.now();
    const ledger = { startedAt: new Date(start).toISOString(), apiCalls: 0 };
    writeFileSync(
      meterPath,
      [
        { event: "meter-created" },
        {
          event: "attempt",
          sequence: 2,
          at: start / 1000,
          counts: { api: 0, aws: 1, poll: 0 },
        },
        { event: "outcome", attempt: 2, status: 200 },
      ]
        .map(JSON.stringify)
        .join("\n") + "\n",
    );
    let auth = 0;
    const transport = createCoordinatorTransport({
      origin: "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com",
      ledger,
      budget: coordinatorBudget({ ledger, meterPath }),
      persist() {},
      token() {
        auth++;
        writeFileSync(
          meterPath,
          readFileSync(meterPath) + '{"event":"stop"}\n',
        );
        return "fixture";
      },
    });
    const originalFetch = globalThis.fetch;
    let writes = 0;
    globalThis.fetch = async () => {
      writes++;
      throw Error("must not dispatch");
    };
    try {
      await assert.rejects(
        transport.call("POST", "/api/v1/projects"),
        /no replay/,
      );
      await assert.rejects(transport.call("POST", "/api/v1/projects"));
      assert.equal(auth, 1);
      assert.equal(writes, 0);
    } finally {
      globalThis.fetch = originalFetch;
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
test("actual coordinator reserve wiring blocks GET before transport and preserves cleanup", async () => {
  const dir = mkdtempSync(tmpdir() + "/kurier-coordinator-");
  try {
    const start = Date.now();
    const ledger = {
      startedAt: new Date(start).toISOString(),
      apiCalls: 0,
      polls: 0,
    };
    const meter = dir + "/attempts.jsonl";
    let counts = { api: 0, aws: 0, poll: 0 };
    const save = () =>
      writeFileSync(
        meter,
        [
          { event: "meter-created" },
          { event: "attempt", sequence: 2, at: start / 1000, counts },
          { event: "outcome", attempt: 2, status: 200 },
        ]
          .map(JSON.stringify)
          .join("\n") + "\n",
      );
    save();
    let now = start;
    const budget = coordinatorBudget({
      ledger,
      meterPath: meter,
      now: () => now,
    });
    for (const key of ["api", "aws", "time"]) {
      counts = {
        api: key === "api" ? 440 : 0,
        aws: key === "aws" ? 1300 : 0,
        poll: 0,
      };
      now = start + (key === "time" ? 100 * 60000 : 0);
      save();
      budget({ cleanup: true });
      assert.throws(() => budget(), /reserve/);
      const transport = createCoordinatorTransport({
        origin: "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com",
        ledger,
        token: () => {
          throw Error("MUST NOT CALL AUTH OR NETWORK");
        },
        budget,
        persist: () =>
          persistCoordinatorLedger(dir + "/coordinator.json", ledger),
      });
      await assert.rejects(
        transport.pollSubmitted("11111111-1111-4111-8111-111111111111", {
          status: 202,
          data: {
            run: {
              workflowRunId: "22222222-2222-4222-8222-222222222222",
              state: "queued",
            },
          },
          headers: new Headers(),
        }),
      );
      assert.equal(ledger.apiCalls, 0);
      assert.equal(ledger.polls, 0);
    }
    now = start;
    counts = { api: 500, aws: 0, poll: 0 };
    save();
    assert.throws(() => budget({ cleanup: true }));
    counts = { api: 0, aws: 1500, poll: 0 };
    save();
    assert.throws(() => budget({ cleanup: true }));
    counts = { api: 0, aws: 0, poll: 240 };
    save();
    budget({ cleanup: true });
    assert.throws(() => budget());
    counts = { api: 0, aws: 0, poll: 0 };
    now = start + 120 * 60000;
    save();
    assert.throws(() => budget({ cleanup: true }));
    now = start;
    save();
    writeFileSync(meter, readFileSync(meter) + '{"event":"stop"}\n');
    assert.throws(() => budget({ cleanup: true }));
    persistCoordinatorLedger(dir + "/coordinator.json", ledger);
    assert.equal(
      JSON.parse(readFileSync(dir + "/coordinator.json")).apiCalls,
      0,
    );
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("cleanup budget uses frozen parent clock and counters before its first exchange", () => {
  const directory = mkdtempSync(tmpdir() + "/kurier-cleanup-budget-");
  try {
    const ledger = {
      startedAt: new Date(100000).toISOString(),
      apiCalls: 62,
      polls: 7,
    };
    const path = directory + "/attempts.jsonl";
    writeFileSync(
      path,
      JSON.stringify({
        event: "meter-created",
        cleanupParent: {
          firstAttemptAt: 100,
          counts: { api: 62, aws: 766, poll: 7, static: 9 },
        },
      }) + "\n",
    );
    const budget = coordinatorBudget({
      ledger,
      meterPath: path,
      now: () => 200000,
    });
    assert.throws(() => budget(), /Cleanup-only/);
    budget({ cleanup: true });
    assert.equal(ledger.meterCounts.aws, 766);
    writeFileSync(path, readFileSync(path) + '{"event":"stop"}\n');
    assert.throws(() => budget({ cleanup: true }));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("explicit H-L continuation retains original clock/counters and cleanup reserve", () => {
  const directory = mkdtempSync(tmpdir() + "/kurier-hl-budget-");
  try {
    const path = directory + "/attempts.jsonl";
    const ledger = {
      startedAt: new Date(100000).toISOString(),
      apiCalls: 0,
      polls: 0,
    };
    writeFileSync(
      path,
      JSON.stringify({
        event: "meter-created",
        continuationParent: {
          firstAttemptAt: 100,
          counts: { api: 0, aws: 54, poll: 0, static: 0 },
        },
      }) + "\n",
    );
    let now = 100000 + 99 * 60000;
    const budget = coordinatorBudget({
      ledger,
      meterPath: path,
      now: () => now,
    });
    budget();
    assert.equal(ledger.meterCounts.aws, 54);
    now = 100000 + 100 * 60000;
    assert.throws(() => budget(), /reserve/);
    budget({ cleanup: true });
    now = 100000 + 120 * 60000;
    assert.throws(() => budget({ cleanup: true }), /reserve/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("authorized replacement uses first NEW attempt clock while preserving total counters", () => {
  const directory = mkdtempSync(tmpdir() + "/kurier-hl-replacement-");
  try {
    const path = directory + "/attempts.jsonl";
    const start = 500000;
    const ledger = {
      startedAt: new Date(start).toISOString(),
      apiCalls: 31,
      polls: 0,
    };
    writeFileSync(
      path,
      [
        {
          event: "meter-created",
          replacementParent: {
            historicalFirstAttemptAt: 100,
            counts: { api: 31, aws: 173, poll: 0, static: 9 },
          },
        },
        {
          event: "attempt",
          sequence: 2,
          at: start / 1000,
          counts: { api: 31, aws: 174, poll: 0, static: 9 },
        },
        { event: "outcome", attempt: 2, status: 200 },
      ]
        .map(JSON.stringify)
        .join("\n") + "\n",
    );
    let now = start + 99 * 60000;
    const budget = coordinatorBudget({
      ledger,
      meterPath: path,
      now: () => now,
    });
    budget();
    assert.equal(ledger.meterCounts.aws, 174);
    assert.equal(ledger.meterCounts.api, 31);
    assert.equal(ledger.meterCounts.static, 9);
    now = start + 100 * 60000;
    assert.throws(() => budget(), /reserve/);
    budget({ cleanup: true });
    now = start + 120 * 60000;
    assert.throws(() => budget({ cleanup: true }), /reserve/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
