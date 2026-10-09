import { useEffect, useRef, useState } from "react";
import {
  api,
  ApiError,
  UncertainWrite,
  type Execution,
  type ExecutionOptions,
  type Evidence,
  type EvidenceBody,
  type RequestDetail,
} from "./api";
import { SessionExpired } from "./auth";

const terminal = (e: Execution) =>
  e.status === "completed" || e.status === "failed";
type Admission = {
  key: string;
  source: string;
  etag?: string;
  options: ExecutionOptions;
  rerun: boolean;
  createdAt: number;
};
function Body({ body }: { body: EvidenceBody }) {
  return body.text === null ? (
    <p>
      {body.kind === "omitted"
        ? `Body omitted: ${body.omissionReason}`
        : "No body"}
    </p>
  ) : (
    <pre>{body.text}</pre>
  );
}
export function Executions({
  projectId,
  request,
  disabled,
  onSessionExpired,
  onGateChanged,
}: {
  projectId: string;
  request?: RequestDetail;
  disabled: boolean;
  onSessionExpired: () => void;
  onGateChanged?: () => void;
}) {
  const [items, setItems] = useState<Execution[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [selected, setSelected] = useState<Execution>();
  const [evidence, setEvidence] = useState<Evidence>();
  const [pending, setPending] = useState<Admission>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [timeout, setTimeoutSeconds] = useState(30);
  const [allow, setAllow] = useState(false);
  const [omitBody, setOmitBody] = useState(false);
  const [headers, setHeaders] = useState("");
  const [pointers, setPointers] = useState("");
  const [confirmRerun, setConfirmRerun] = useState(false);
  const mounted = useRef(false);
  const inFlight = useRef(false);
  const lifecycle = useRef<AbortController | null>(null);
  const report = (e: unknown) => {
    if (!mounted.current || lifecycle.current?.signal.aborted) return;
    if (e instanceof SessionExpired) {
      onSessionExpired();
      return;
    }
    setError(
      e instanceof UncertainWrite
        ? "Submission outcome is unresolved. Reconcile using the same key and frozen options; do not start another execution."
        : e instanceof ApiError
          ? `Execution operation failed (${e.code}).`
          : "Execution API unavailable. Refresh to reconcile status.",
    );
  };
  const history = async (next?: string) => {
    const page = await api.executionHistory(
      projectId,
      next,
      lifecycle.current?.signal,
    );
    if (!mounted.current) return;
    setItems((old) => (next ? [...old, ...page.items] : page.items));
    setCursor(page.nextCursor);
  };
  const run = async (work: () => Promise<void>) => {
    if (inFlight.current || disabled || !mounted.current) return;
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      await work();
    } catch (e) {
      report(e);
    } finally {
      inFlight.current = false;
      if (mounted.current) setBusy(false);
    }
  };
  useEffect(() => {
    mounted.current = true;
    const controller = new AbortController();
    lifecycle.current = controller;
    if (!disabled) void run(() => history());
    return () => {
      mounted.current = false;
      controller.abort();
    };
    // Ownership/session changes remount this project panel. A disabled gate cancels reads.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, disabled]);
  useEffect(() => {
    if (!selected || terminal(selected) || disabled) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    let failures = 0;
    const controller = new AbortController();
    const poll = async () => {
      if (stopped) return;
      if (inFlight.current) {
        timer = setTimeout(poll, 1000);
        return;
      }
      inFlight.current = true;
      try {
        const status = await api.executionStatus(
          projectId,
          selected.executionId,
          controller.signal,
        );
        if (stopped) return;
        failures = 0;
        setSelected((old) =>
          old?.executionId === status.executionId ? { ...old, ...status } : old,
        );
        setMessage(`Status refreshed: ${status.status}`);
        if (terminal(status)) return;
      } catch (e) {
        if (!stopped) {
          failures++;
          report(e);
        }
      } finally {
        inFlight.current = false;
      }
      if (!stopped)
        timer = setTimeout(
          poll,
          Math.min(60000, 10000 * 2 ** failures) + Math.random() * 1000,
        );
    };
    timer = setTimeout(poll, 10000 + Math.random() * 1000);
    return () => {
      stopped = true;
      controller.abort();
      clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, selected?.executionId, selected?.status, disabled]);
  const admit = async (admission: Admission) => {
    if (Date.now() - admission.createdAt >= 7 * 24 * 60 * 60 * 1000) {
      setError(
        "The seven-day reconciliation window has expired. Review history before deliberately starting again.",
      );
      return;
    }
    setPending(admission);
    try {
      const execution = admission.rerun
        ? await api.rerunExecution(
            projectId,
            admission.source,
            admission.key,
            admission.options.allowInsecureSecrets,
            lifecycle.current?.signal,
          )
        : await api.submitExecution(
            projectId,
            admission.source,
            admission.etag!,
            admission.key,
            admission.options,
            lifecycle.current?.signal,
          );
      if (!mounted.current) return;
      onGateChanged?.();
      setPending(undefined);
      setSelected(execution);
      setEvidence(undefined);
      setConfirmRerun(false);
      setMessage(
        "Execution durably accepted. Status polling returns immediately and refreshes about every ten seconds.",
      );
      await history();
    } catch (e) {
      onGateChanged?.();
      if (e instanceof ApiError && e.status < 500) setPending(undefined);
      throw e;
    }
  };
  const options = (): ExecutionOptions => ({
    timeoutSeconds: timeout,
    allowInsecureSecrets: allow,
    responseRedaction: {
      headers: headers
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean),
      jsonPointers: pointers.split("\n").filter((s) => s !== ""),
      omitBody,
    },
  });
  return (
    <section aria-label="Cloud executions" className="panel">
      <h3>Cloud executions and evidence</h3>
      <p>
        Execute the saved revision. Protected inputs are frozen in an encrypted
        bundle limited to 32 KiB; the complete request and resolved body each
        have a separate 64 KiB limit. Evidence is automatically saved for 30
        days.
      </p>
      <p>
        Known protected reflections and credential fields are masked. Arbitrary
        transformed secrets cannot be detected: configure response JSON paths or
        omit the body. HTML and unsupported content are omitted. Responses are
        displayed as escaped text.
      </p>
      <fieldset disabled={busy || disabled || !!pending}>
        <label>
          Execution deadline (seconds)
          <input
            type="number"
            min={1}
            max={60}
            value={timeout}
            onChange={(e) => setTimeoutSeconds(Number(e.target.value))}
          />
        </label>
        <label>
          <input
            type="checkbox"
            checked={allow}
            onChange={(e) => setAllow(e.target.checked)}
          />
          I explicitly allow protected inputs over unencrypted HTTP for this
          execution or rerun.
        </label>
        <label>
          Additional response headers to redact (one per line)
          <textarea
            value={headers}
            onChange={(e) => setHeaders(e.target.value)}
          />
        </label>
        <label>
          Response JSON pointers to redact (one per line)
          <textarea
            value={pointers}
            onChange={(e) => setPointers(e.target.value)}
          />
        </label>
        <label>
          <input
            type="checkbox"
            checked={omitBody}
            onChange={(e) => setOmitBody(e.target.checked)}
          />
          Omit the response body
        </label>
        <button
          disabled={!request}
          onClick={() => {
            if (request)
              void run(() =>
                admit({
                  key: crypto.randomUUID(),
                  source: request.request.requestId,
                  etag: request.etag,
                  options: options(),
                  rerun: false,
                  createdAt: Date.now(),
                }),
              );
          }}
        >
          Execute saved revision {request?.request.revision}
        </button>
      </fieldset>
      {pending && (
        <p role="status">
          An admission is unresolved. Its identity and options are held only in
          this tab.{" "}
          <button
            disabled={busy || disabled}
            onClick={() => void run(() => admit(pending))}
          >
            Reconcile original submission
          </button>
        </p>
      )}
      {error && <p role="alert">{error}</p>}
      <p role="status">{busy ? "Refreshing executions…" : message}</p>
      <button
        disabled={busy || disabled}
        onClick={() => void run(() => history())}
      >
        Refresh execution history
      </button>
      <ul>
        {items.map((item) => (
          <li key={item.executionId}>
            <button
              disabled={busy || disabled}
              onClick={() =>
                void run(async () => {
                  const detail = await api.executionDetail(
                    projectId,
                    item.executionId,
                    lifecycle.current?.signal,
                  );
                  if (mounted.current) {
                    setSelected(detail);
                    setEvidence(undefined);
                    setConfirmRerun(false);
                  }
                })
              }
            >
              {item.submittedAt} · {item.status} · revision{" "}
              {item.source.revision}
            </button>
          </li>
        ))}
      </ul>
      {cursor && (
        <button
          disabled={busy || disabled}
          onClick={() => void run(() => history(cursor))}
        >
          More executions
        </button>
      )}
      {selected && (
        <div>
          <h4>Execution {selected.executionId}</h4>
          <p>
            {selected.status} · saved revision {selected.source.revision}
          </p>
          {selected.configuration && (
            <p>
              {selected.configuration.method} {selected.configuration.url}
            </p>
          )}
          {selected.summary?.outcome.certainty === "unknown" && (
            <p role="alert">
              <strong>Outcome unknown — do not assume no effect.</strong> An
              external operation may have completed. Automatic replay is
              disabled.
            </p>
          )}
          {selected.summary && (
            <p>
              HTTP {selected.summary.httpStatus ?? "unavailable"} ·{" "}
              {selected.summary.durationMs === null
                ? "Timing unavailable"
                : `${selected.summary.durationMs} ms`}{" "}
              · {selected.summary.outcome.code ?? "response complete"} ·{" "}
              {selected.summary.outcome.certainty}
            </p>
          )}
          {selected.retention && (
            <p>
              Evidence {selected.retention.evidenceAvailability}; expires{" "}
              {selected.retention.normalExpiresAt}
            </p>
          )}
          <button
            disabled={busy || disabled}
            onClick={() =>
              void run(async () => {
                const status = await api.executionStatus(
                  projectId,
                  selected.executionId,
                  lifecycle.current?.signal,
                );
                if (mounted.current) setSelected({ ...selected, ...status });
              })
            }
          >
            Refresh execution status
          </button>
          {terminal(selected) && (
            <button
              disabled={busy || disabled}
              onClick={() =>
                void run(async () => {
                  const capture = await api.executionEvidence(
                    projectId,
                    selected.executionId,
                    lifecycle.current?.signal,
                  );
                  if (mounted.current) setEvidence(capture);
                })
              }
            >
              Inspect sanitized evidence
            </button>
          )}
          <button
            disabled={busy || disabled || !!pending || !selected.liveRequestId}
            onClick={() => setConfirmRerun(true)}
          >
            Review deliberate rerun
          </button>
          {confirmRerun && (
            <div>
              <p>
                A rerun creates a new execution from this frozen public request
                and its redaction policy, using current active values at the
                same secret IDs. It may repeat external effects, especially
                after an unknown outcome. This is separate from reconciling a
                submission.
              </p>
              <button
                disabled={busy || disabled || !!pending}
                onClick={() =>
                  void run(() =>
                    admit({
                      key: crypto.randomUUID(),
                      source: selected.executionId,
                      options: options(),
                      rerun: true,
                      createdAt: Date.now(),
                    }),
                  )
                }
              >
                Confirm deliberate rerun
              </button>
              <button onClick={() => setConfirmRerun(false)}>
                Cancel rerun
              </button>
            </div>
          )}
        </div>
      )}
      {evidence && (
        <div className="execution-evidence">
          <h4>Immutable sanitized capture</h4>
          <p>
            HTTP {evidence.response.httpStatus ?? "unavailable"}; duration{" "}
            {evidence.timing.durationMs ?? "unavailable"} ms; first byte{" "}
            {evidence.timing.timeToFirstByteMs ?? "unavailable"} ms
          </p>
          <h5>Request</h5>
          <pre>
            {evidence.request.method} {evidence.request.url}
          </pre>
          <pre>
            {evidence.request.headers
              .map((h) => `${h.name}: ${h.value}`)
              .join("\n")}
          </pre>
          <Body body={evidence.request.body} />
          <h5>Response headers</h5>
          <pre>
            {evidence.response.headers
              .map((h) => `${h.name}: ${h.value}`)
              .join("\n")}
          </pre>
          <h5>Response body</h5>
          <Body body={evidence.response.body} />
          <p>
            Read {evidence.response.wireBytesRead} wire bytes and{" "}
            {evidence.response.decodedBytesRead} decoded bytes.
          </p>
        </div>
      )}
    </section>
  );
}
