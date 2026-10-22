import {
  readFileSync,
  openSync,
  writeFileSync,
  fsyncSync,
  closeSync,
  renameSync,
} from "node:fs";
import { createCoordinatorRunReader } from "./workflow-run-poll.mjs";

// No credentials, headers or response bodies are written to the operator ledger.
export function persistCoordinatorLedger(path, ledger) {
  const temporary = path + ".tmp";
  const fd = openSync(temporary, "wx", 0o600);
  try {
    writeFileSync(fd, JSON.stringify(ledger, null, 2) + "\n");
    fsyncSync(fd);
  } finally {
    closeSync(fd);
  }
  renameSync(temporary, path);
}

export function coordinatorBudget({ ledger, meterPath, now = Date.now }) {
  return ({ cleanup = false, waitMs = 0 } = {}) => {
    const events = readFileSync(meterPath, "utf8")
      .trim()
      .split("\n")
      .map(JSON.parse);
    if (
      !events.length ||
      events[0].event !== "meter-created" ||
      events.some((e) => e.event === "stop")
    )
      throw Error("Invalid or stopped transport meter");
    const attempts = events.filter((e) => e.event === "attempt");
    const counts = attempts.at(-1)?.counts ??
      events[0].cleanupParent?.counts ??
      events[0].continuationParent?.counts ?? { api: 0, aws: 0, poll: 0 };
    if (events[0].cleanupParent && !cleanup)
      throw Error("Cleanup-only continuation");
    const start = Math.floor(
      (events[0].cleanupParent?.firstAttemptAt ??
        events[0].continuationParent?.firstAttemptAt ??
        attempts[0]?.at) * 1000,
    );
    if (
      !Number.isFinite(start) ||
      !Number.isFinite(Date.parse(ledger.startedAt)) ||
      Date.parse(ledger.startedAt) !== start
    )
      throw Error("Campaign clock must match the first metered attempt");
    if (now() + waitMs - start >= (cleanup ? 120 : 100) * 60_000)
      throw Error("Campaign time reserve exhausted");
    if (
      !Object.values(counts).every((n) => Number.isSafeInteger(n) && n >= 0) ||
      counts.api >= (cleanup ? 500 : 440) ||
      counts.aws >= (cleanup ? 1500 : 1300) ||
      (!cleanup && counts.poll >= 240) ||
      ledger.apiCalls >= 500
    )
      throw Error("Campaign transport reserve exhausted");
    ledger.meterCounts = counts;
  };
}

// This is the actual coordinator transport. One native fetch, no status retry,
// no redirect following, no alternate origin and no secret-bearing exceptions.
// The reader owns GET counters; call() owns all other one-attempt API counters.
export function createCoordinatorTransport({
  origin,
  ledger,
  token,
  budget,
  persist,
  owner = () => 0,
}) {
  if (origin !== "https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com")
    throw Error("Unreviewed application origin");
  async function exchangeOnce({
    method,
    route,
    signal,
    body,
    headers = {},
    identity = owner(),
  }) {
    if (!/^\/api\/v1\/[A-Za-z0-9/_-]+$/.test(route))
      throw Error("Unscoped application route");
    try {
      const bearer =
        identity === null
          ? null
          : typeof identity === "string"
            ? identity
            : await token(identity);
      const response = await fetch(origin + route, {
        method,
        redirect: "error",
        signal: signal ?? AbortSignal.timeout(15_000),
        headers: {
          ...(bearer ? { Authorization: "Bearer " + bearer } : {}),
          ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
          ...headers,
        },
        ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
      });
      const raw = await response.text();
      const envelope = raw ? JSON.parse(raw) : null;
      return {
        status: response.status,
        data: envelope?.data,
        error: envelope?.error,
        headers: response.headers,
        raw,
      };
    } catch {
      throw Error("HTTP outcome uncertain; no replay");
    }
  }
  async function call(
    method,
    route,
    body,
    expected = 200,
    headers = {},
    identity = owner(),
  ) {
    await budget({ cleanup: ledger.cleanup === true });
    ledger.apiCalls++;
    await persist();
    const response = await exchangeOnce({
      method,
      route,
      body,
      headers,
      identity,
    });
    if (![expected].flat().includes(response.status))
      throw Error(`Unexpected application HTTP status ${response.status}`);
    return response;
  }
  const reader = createCoordinatorRunReader({
    ledger,
    exchangeOnce,
    budget,
    persist,
  });
  return { exchangeOnce, call, pollSubmitted: reader.pollSubmitted };
}
