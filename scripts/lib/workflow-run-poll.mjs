// Operator-only polling. No AWS, credentials, submission or stage-write adapter.
export const workflowPollLimits = Object.freeze({
  attemptsPerRun: 35,
  consecutive503Retries: 3,
  minimumDelayMs: 2_000,
  maximumDelayMs: 30_000,
  runWindowMs: 120_000,
  requestTimeoutMs: 15_000,
  apiCalls: 500,
  polls: 240,
  workWindowMs: 100 * 60_000, // The reviewed 120 minutes includes 20 for cleanup.
});

export class WorkflowPollStop extends Error {
  constructor(code) {
    super(`Workflow status polling stopped: ${code}`);
    this.code = code;
  }
}
const stop = (code) => {
  throw new WorkflowPollStop(code);
};
const uuid = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/;
const terminal = (run) => ["completed", "failed"].includes(run.state);
function validateRun(run, runId) {
  if (
    !run ||
    run.workflowRunId !== runId ||
    !["queued", "running", "completed", "failed"].includes(run.state)
  )
    stop("invalid-run-response");
}

// RFC 9110 10.2.3: seconds or HTTP-date. Recognize all three HTTP-date shapes,
// rejecting other Date.parse inputs (e.g. a fractional delay or relative date).
function retryAfterMs(value, wallNow) {
  if (value === null || value === undefined) return 0;
  if (typeof value !== "string" || value.length > 128)
    stop("invalid-retry-after");
  const text = value.trim();
  let delay;
  if (/^\d+$/.test(text)) delay = Number(text) * 1_000;
  else {
    const day = "(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)";
    const month = "(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)";
    const time = "\\d{2}:\\d{2}:\\d{2}";
    const shapes = [
      `${day}, \\d{2} ${month} \\d{4} ${time} GMT`,
      `(?:Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday), \\d{2}-${month}-\\d{2} ${time} GMT`,
      `${day} ${month} (?: \\d|\\d{2}) ${time} \\d{4}`,
    ];
    if (!shapes.some((s) => new RegExp(`^${s}$`).test(text)))
      stop("invalid-retry-after");
    // asctime omits the timezone but HTTP-date is always UTC. Date.parse would
    // otherwise interpret that form in the operator machine's local timezone.
    delay = Math.max(
      0,
      Date.parse(text.endsWith("GMT") ? text : `${text} GMT`) - wallNow,
    );
  }
  if (!Number.isFinite(delay)) stop("invalid-retry-after");
  if (delay > workflowPollLimits.maximumDelayMs)
    stop("retry-after-exceeds-bound"); // Stop; never shorten the server's delay.
  return delay;
}

// requestOnce must perform exactly one exchange, forbid redirects, honor signal, parse JSON and
// return {status, data, error, headers}. It must not check away the 503 first.
// checkBudget must consult the same authoritative meter/reserves as all other
// coordinator operations. persist must durably save the shared counters before
// requestOnce. It is intentionally impossible to specify a method/body/URL here.
export function createWorkflowRunPoller({
  ledger,
  requestOnce,
  checkBudget,
  persist,
  wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
  clock = { now: () => performance.now(), wallNow: () => Date.now() },
}) {
  if (
    !ledger ||
    [requestOnce, checkBudget, persist, wait, clock.now, clock.wallNow].some(
      (f) => typeof f !== "function",
    )
  )
    stop("missing-budget-or-transport");
  const started = Date.parse(ledger.startedAt);
  if (!Number.isFinite(started) || started > clock.wallNow())
    stop("invalid-campaign-start");
  const campaignDeadline =
    clock.now() + workflowPollLimits.workWindowMs - (clock.wallNow() - started);
  let busy = false;
  let stopped = false;

  return async function poll(options) {
    if (stopped || ledger.stoppedAt || ledger.rolloutRemainsStopped)
      stop("latched-stop");
    if (busy) stop("poll-in-flight");
    busy = true;
    try {
      if (
        !options ||
        Object.keys(options).some(
          (k) => !["projectId", "runId", "initial", "retryAfter"].includes(k),
        ) ||
        !uuid.test(options.projectId) ||
        !uuid.test(options.runId)
      )
        stop("invalid-poll-scope");
      const { projectId, runId, initial } = options;
      validateRun(initial, runId);
      if (terminal(initial)) return initial;
      const route = `/api/v1/projects/${projectId}/workflow-runs/${runId}`;
      const deadline = Math.min(
        campaignDeadline,
        clock.now() + workflowPollLimits.runWindowMs,
      );
      let consecutive503 = 0;
      let delay = Math.max(
        workflowPollLimits.minimumDelayMs,
        retryAfterMs(options.retryAfter, clock.wallNow()),
      );

      const guard = async (waitMs = 0) => {
        if (
          stopped ||
          ledger.stoppedAt ||
          ledger.rolloutRemainsStopped ||
          !Number.isSafeInteger(ledger.apiCalls) ||
          !Number.isSafeInteger(ledger.polls) ||
          ledger.apiCalls < 0 ||
          ledger.polls < 0
        )
          stop("invalid-or-stopped-budget");
        if (ledger.apiCalls >= workflowPollLimits.apiCalls) stop("api-limit");
        if (ledger.polls >= workflowPollLimits.polls) stop("poll-limit");
        if (clock.now() + waitMs >= deadline) stop("time-limit");
        await checkBudget({ waitMs, deadline });
      };

      for (let n = 0; n < workflowPollLimits.attemptsPerRun; n++) {
        await guard(delay);
        const due = clock.now() + delay;
        do {
          await wait(Math.ceil(due - clock.now()));
          await guard();
        } while (clock.now() < due);
        await guard();
        ledger.apiCalls++;
        ledger.polls++;
        await persist();
        // Counter persistence/budget inspection can consume time as well.
        if (clock.now() >= deadline) stop("time-limit");
        const signal = AbortSignal.timeout(
          Math.max(
            1,
            Math.floor(
              Math.min(
                workflowPollLimits.requestTimeoutMs,
                deadline - clock.now(),
              ),
            ),
          ),
        );
        let response;
        try {
          response = await requestOnce({ method: "GET", route, signal });
        } catch {
          stop("get-outcome-uncertain"); // No transport/parse/timeout retries.
        }
        if (clock.now() >= deadline) stop("time-limit");
        if (response?.status === 200) {
          const run = response.data?.run;
          validateRun(run, runId);
          if (terminal(run)) return run;
          consecutive503 = 0;
          delay = Math.max(
            workflowPollLimits.minimumDelayMs,
            retryAfterMs(response.headers?.get("retry-after"), clock.wallNow()),
          );
        } else {
          if (
            response?.status !== 503 ||
            response.error?.code !== "service_unavailable"
          )
            stop("non-retryable-response");
          if (++consecutive503 > workflowPollLimits.consecutive503Retries)
            stop("retry-limit");
          delay = Math.max(
            workflowPollLimits.minimumDelayMs * 2 ** (consecutive503 - 1),
            retryAfterMs(response.headers?.get("retry-after"), clock.wallNow()),
          );
        }
      }
      stop("run-poll-limit");
    } catch (error) {
      stopped = true;
      // Never expose a budget/transport implementation's secret-bearing error.
      if (error instanceof WorkflowPollStop) throw error;
      stop("budget-or-persistence-failure");
    } finally {
      busy = false;
    }
  };
}

// Concrete coordinator wiring: call the submission ONCE outside this adapter,
// then pass its acknowledged202 response. exchangeOnce is the uncounted,
// non-retrying transport; poll owns GET accounting, other operations keep their
// existing one-attempt accounting. No request body or write callback is stored.
export function createCoordinatorRunReader({
  ledger,
  exchangeOnce,
  budget,
  persist,
  wait,
  clock,
}) {
  const poll = createWorkflowRunPoller({
    ledger,
    requestOnce: exchangeOnce,
    checkBudget: budget,
    persist,
    ...(wait ? { wait } : {}),
    ...(clock ? { clock } : {}),
  });
  let stopped = false;
  return {
    async pollSubmitted(projectId, submission) {
      if (stopped) stop("latched-stop");
      try {
        if (submission?.status !== 202 || !submission.data?.run)
          stop("submission-not-acknowledged");
        const initial = submission.data.run;
        const location = submission.headers?.get("location");
        if (
          location &&
          location !==
            `/api/v1/projects/${projectId}/workflow-runs/${initial.workflowRunId}`
        )
          stop("submission-locator-mismatch");
        return await poll({
          projectId,
          runId: initial.workflowRunId,
          initial,
          retryAfter: submission.headers?.get("retry-after"),
        });
      } catch (error) {
        stopped = true;
        if (error instanceof WorkflowPollStop) throw error;
        stop("invalid-submission-response");
      }
    },
  };
}
