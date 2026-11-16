import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { test } from "node:test";
import { browserRequestGuard } from "./lib/browser-request-guard.mjs";

const request = (errorText = "net::ERR_CONNECTION_RESET") => ({
  url: () => "https://fixture.invalid/api/v1/projects?secret=never-retain",
  failure: () => ({ errorText }),
});

test("browser close requires finished response bodies, not just response headers", () => {
  const context = new EventEmitter();
  const guard = browserRequestGuard(context);
  const r = request();
  context.emit("request", r);
  context.emit("response", { request: () => r, status: () => 200 });
  assert.throws(() => guard.beginClose(), /in flight/);
  context.emit("requestfinished", r);
  guard.beginClose();
  guard.assertSettled();
  assert.deepEqual(guard.summary(), {
    pendingRequests: 0,
    transportFailures: 0,
    closeCancellations: 0,
  });
});

test("normal read failures and pre-close aborts remain failures", () => {
  for (const error of ["net::ERR_CONNECTION_RESET", "net::ERR_ABORTED"]) {
    const context = new EventEmitter();
    const guard = browserRequestGuard(context);
    const r = request(error);
    context.emit("request", r);
    context.emit("requestfailed", r);
    assert.throws(() => guard.beginClose(), /transport failed/);
    assert.equal(guard.summary().closeCancellations, 0);
    assert.equal(guard.summary().transportFailures, 1);
  }
});

test("a request racing with teardown is diagnosed but never excused", () => {
  const context = new EventEmitter();
  const guard = browserRequestGuard(context);
  guard.beginClose();
  const r = request("net::ERR_ABORTED");
  context.emit("request", r);
  context.emit("requestfailed", r);
  assert.throws(() => guard.assertSettled(), /transport failed/);
  assert.equal(guard.summary().closeCancellations, 1);
  assert(!JSON.stringify(guard.summary()).includes("never-retain"));
});
