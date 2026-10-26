import { EnvironmentSelector } from "./Environments";
import { AssertionResults } from "./Assertions";
import { LocalAgents } from "./LocalAgents";
import { useEffect, useRef, useState } from "react";
import {
  api,
  ApiError,
  UncertainWrite,
  type EnvironmentSelection,
  type Execution,
  type ExecutionOptions,
  type Evidence,
  type EvidenceBody,
  type LocalAgent,
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
  const [environment, setEnvironment] = useState<EnvironmentSelection | null>(
    null,
  );
  const [agent, setAgent] = useState<LocalAgent | null>(null);
  const [target, setTarget] = useState<"cloud" | "local">("cloud");
  const [localConsent, setLocalConsent] = useState(false);
  const [rerunConsent, setRerunConsent] = useState(false);
  const [rerunHTTPConsent, setRerunHTTPConsent] = useState(false);
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
  const [confirmUnpin, setConfirmUnpin] = useState<Execution>();
  const [pinBusy, setPinBusy] = useState<string>();
  const [pinUnresolved, setPinUnresolved] = useState(false);
  const mounted = useRef(false);
  const inFlight = useRef(false);
  const lifecycle = useRef<AbortController | null>(null);
  const received = useRef(new Map<string, number>());
  const [clock, setClock] = useState(Date.now());
  useEffect(() => {
    setLocalConsent(false);
    setAllow(false);
    setRerunConsent(false);
  }, [
    agent?.localAgentId,
    request?.request.requestId,
    request?.request.revision,
  ]);
  useEffect(() => {
    const timer = setInterval(() => setClock(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
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
    const signal = lifecycle.current?.signal;
    const page = await api.executionHistory(projectId, next, signal);
    if (!mounted.current || signal?.aborted) return;
    for (const item of page.items)
      received.current.set(item.executionId, Date.now());
    setItems((old) => (next ? [...old, ...page.items] : page.items));
    setCursor(page.nextCursor);
  };
  const run = async (work: () => Promise<void>) => {
    if (inFlight.current || disabled || !mounted.current) return;
    const signal = lifecycle.current?.signal;
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      await work();
    } catch (e) {
      if (!signal?.aborted) report(e);
    } finally {
      if (!signal?.aborted) {
        inFlight.current = false;
        if (mounted.current) setBusy(false);
      }
    }
  };
  useEffect(() => {
    mounted.current = true;
    setAgent(null);
    setTarget("cloud");
    setLocalConsent(false);
    setSelected(undefined);
    setEvidence(undefined);
    setConfirmUnpin(undefined);
    setPinUnresolved(false);
    setPinBusy(undefined);
    inFlight.current = false;
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
            admission.options.local,
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
      setLocalConsent(false);
      setAllow(false);
      setRerunConsent(false);
      setRerunHTTPConsent(false);
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
  const changePin = async (item: Execution, pinned: boolean) => {
    const signal = lifecycle.current?.signal;
    setPinBusy(item.executionId);
    setPinUnresolved(false);
    setConfirmUnpin(undefined);
    try {
      const result = await api.pinExecution(
        projectId,
        item.executionId,
        pinned,
        signal,
      );
      if (!mounted.current || signal?.aborted) return;
      onGateChanged?.();
      received.current.set(item.executionId, Date.now());
      const updated = {
        ...item,
        retention: result.retention,
        serverTime: result.serverTime,
      };
      const expired =
        result.retention.effectiveStatus === "expired" ||
        result.retention.effectiveStatus === "deleting";
      setItems((old) =>
        expired
          ? old.filter((e) => e.executionId !== item.executionId)
          : old.map((e) =>
              e.executionId === item.executionId
                ? {
                    ...e,
                    retention: result.retention,
                    serverTime: result.serverTime,
                  }
                : e,
            ),
      );
      setSelected((old) =>
        old?.executionId === item.executionId
          ? expired
            ? undefined
            : {
                ...old,
                retention: result.retention,
                serverTime: result.serverTime,
              }
          : old,
      );
      if (expired && selected?.executionId === item.executionId)
        setEvidence(undefined);
      setMessage(
        pinned
          ? "Evidence pinned."
          : expired
            ? "Evidence unpinned. Automatic expiry applies immediately."
            : "Evidence unpinned. Original expiry applies.",
      );
      await history();
      return updated;
    } catch (e) {
      if (!mounted.current || signal?.aborted) return;
      onGateChanged?.();
      if (e instanceof SessionExpired) {
        onSessionExpired();
        return;
      }
      if (e instanceof ApiError && e.status === 404) {
        setItems((old) =>
          old.filter((v) => v.executionId !== item.executionId),
        );
        if (selected?.executionId === item.executionId) {
          setSelected(undefined);
          setEvidence(undefined);
        }
        setError("Evidence is no longer available.");
        return;
      }
      if (e instanceof ApiError && e.status === 409) {
        setError(
          e.code === "pin_quota_exceeded"
            ? "Pin quota reached. Unpin other evidence before deliberately retrying."
            : e.code === "pin_window_closed"
              ? "Too close to expiry to pin this evidence."
              : "Retention changed concurrently. Review refreshed state before deliberately retrying.",
        );
        try {
          const current = await api.executionDetail(
            projectId,
            item.executionId,
            signal,
          );
          if (!signal?.aborted && mounted.current) {
            setItems((old) =>
              old.map((v) =>
                v.executionId === item.executionId ? current : v,
              ),
            );
            setSelected((old) =>
              old?.executionId === item.executionId ? current : old,
            );
          }
        } catch (refreshError) {
          if (
            !signal?.aborted &&
            refreshError instanceof ApiError &&
            refreshError.status === 404 &&
            selected?.executionId === item.executionId
          ) {
            setSelected(undefined);
            setEvidence(undefined);
          }
        }
        await history();
        return;
      }
      setPinUnresolved(true);
      setError(
        "Pin operation outcome is unresolved. Refresh retention before a deliberate retry.",
      );
    } finally {
      if (!signal?.aborted && mounted.current) setPinBusy(undefined);
    }
  };
  const pinControl = (item: Execution) => {
    const retention = item.retention;
    const pinned = retention?.pinned === true;
    const now =
      Date.parse(item.serverTime) +
      Math.max(0, clock - (received.current.get(item.executionId) ?? clock));
    const eligible =
      !!retention?.pinEligible &&
      Date.parse(retention.normalExpiresAt) > now + 5000;
    return (
      <span>
        <span>
          {pinned
            ? "Protected from automatic expiry while pinned."
            : retention
              ? `Normal expiry: ${retention.normalExpiresAt}`
              : "Evidence is not published yet."}{" "}
          {pinned && retention
            ? ` Normal expiry: ${retention.normalExpiresAt}.`
            : ""}
          {retention?.evidenceAvailability === "unavailable"
            ? "Evidence unavailable."
            : ""}
        </span>
        <button
          aria-label={`${pinned ? "Unpin" : "Pin"} evidence ${item.executionId}`}
          disabled={busy || disabled || pinUnresolved || (!pinned && !eligible)}
          onClick={() => {
            if (
              pinned &&
              retention &&
              Date.parse(retention.normalExpiresAt) <= now
            ) {
              setConfirmUnpin(item);
              return;
            }
            void run(() => changePin(item, !pinned).then(() => undefined));
          }}
        >
          {pinBusy === item.executionId
            ? pinned
              ? "Unpinning…"
              : "Pinning…"
            : pinned
              ? "Unpin"
              : "Pin"}
        </button>
        {!pinned && retention && !eligible && (
          <span>
            Pin unavailable: evidence is unavailable, being removed, or too
            close to expiry.
          </span>
        )}
      </span>
    );
  };
  const options = (): ExecutionOptions => ({
    ...(request?.request.templateSemanticsVersion === 1 && environment
      ? { environment }
      : {}),
    ...(target === "local" && agent
      ? {
          target: "local" as const,
          local: {
            localAgentId: agent.localAgentId,
            allowPrivateNetwork: true as const,
          },
        }
      : {}),
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
    <section aria-label="Executions" className="panel">
      <h3>Executions and evidence</h3>
      <LocalAgents
        projectId={projectId}
        disabled={disabled}
        onAgent={setAgent}
        onSessionExpired={onSessionExpired}
      />
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
          Execution target
          <select
            value={target}
            onChange={(e) => {
              setTarget(e.target.value as "cloud" | "local");
              setLocalConsent(false);
              setAllow(false);
            }}
          >
            <option value="cloud">Cloud</option>
            <option
              value="local"
              disabled={
                !agent ||
                !!agent.revokedAt ||
                Date.parse(agent.expiresAt) <=
                  (agent.serverTime
                    ? Date.parse(agent.serverTime)
                    : Date.now()) ||
                !agent.scopes.some((s) => s.projectId === projectId)
              }
            >
              Local agent
            </option>
          </select>
        </label>
        {target === "local" && agent && (
          <div>
            <p>
              Device: {agent.name}. Approved services:{" "}
              {agent.scopes
                .find((s) => s.projectId === projectId)
                ?.destinations.map((d) => d.origin)
                .join(", ")}
              . Last seen: {agent.lastSeenAt ?? "Offline / unknown"}. Offline
              submissions may wait up to ten minutes; there is no cloud
              fallback.
            </p>
            <label>
              <input
                type="checkbox"
                checked={localConsent}
                onChange={(e) => setLocalConsent(e.target.checked)}
              />
              I allow this execution on the selected device and its approved
              local/private services.
            </label>
          </div>
        )}
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
          disabled={
            !request ||
            (target === "local" &&
              (!localConsent ||
                !agent ||
                !!agent.revokedAt ||
                Date.parse(agent.expiresAt) <=
                  (agent.serverTime
                    ? Date.parse(agent.serverTime)
                    : Date.now()) ||
                !agent.scopes.some((s) => s.projectId === projectId)))
          }
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
      {request?.request.templateSemanticsVersion === 1 && (
        <EnvironmentSelector
          projectId={projectId}
          label="Execution environment"
          value={environment}
          change={setEnvironment}
          disabled={disabled || busy || !!pending}
          onExpired={onSessionExpired}
        />
      )}
      {evidence?.schemaVersion === 3 && (
        <p>
          Frozen environment:{" "}
          {evidence.environmentContext
            ? `${evidence.environmentContext.environmentId} · revision ${evidence.environmentContext.revision}`
            : "none"}{" "}
          · template semantics 1
        </p>
      )}
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
        onClick={() =>
          void run(async () => {
            const signal = lifecycle.current?.signal;
            await history();
            if (selected) {
              try {
                const current = await api.executionDetail(
                  projectId,
                  selected.executionId,
                  signal,
                );
                if (!signal?.aborted) setSelected(current);
              } catch (e) {
                if (e instanceof ApiError && e.status === 404) {
                  setSelected(undefined);
                  setEvidence(undefined);
                } else throw e;
              }
            }
            if (!signal?.aborted) {
              setPinUnresolved(false);
              setError("");
            }
          })
        }
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
                    received.current.set(detail.executionId, Date.now());
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
            {pinControl(item)}
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
            Target: {selected.target ?? "cloud"}
            {selected.localAgentId ? ` · Agent ${selected.localAgentId}` : ""}
          </p>
          {pinControl(selected)}
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
              Evidence {selected.retention.evidenceAvailability}; normal expiry{" "}
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
            disabled={
              busy ||
              disabled ||
              !!pending ||
              !selected.liveRequestId ||
              !!selected.runId
            }
            onClick={() => {
              setRerunConsent(false);
              setRerunHTTPConsent(false);
              setConfirmRerun(true);
            }}
          >
            Review deliberate rerun
          </button>
          {selected.runId && (
            <p>
              This step belongs to workflow run {selected.runId}. Use the
              workflow panel to deliberately rerun the entire sequence.
            </p>
          )}
          {confirmRerun && (
            <div>
              <p>
                A rerun creates a new execution from this frozen public request
                and its redaction policy, using current active values at the
                same secret IDs. It may repeat external effects, especially
                after an unknown outcome. This is separate from reconciling a
                submission.
              </p>
              {selected.target === "local" && (
                <label>
                  <input
                    type="checkbox"
                    checked={rerunConsent}
                    onChange={(e) => setRerunConsent(e.target.checked)}
                  />
                  I authorize a deliberate local rerun on{" "}
                  {agent?.name ?? "an unavailable agent"} and its approved
                  services.
                </label>
              )}
              <label>
                <input
                  type="checkbox"
                  checked={rerunHTTPConsent}
                  onChange={(e) => setRerunHTTPConsent(e.target.checked)}
                />
                I explicitly allow protected inputs over HTTP for this new
                rerun.
              </label>
              <button
                disabled={
                  busy ||
                  disabled ||
                  !!pending ||
                  (selected.target === "local" &&
                    (!rerunConsent ||
                      !agent ||
                      !!agent.revokedAt ||
                      Date.parse(agent.expiresAt) <=
                        (agent.serverTime
                          ? Date.parse(agent.serverTime)
                          : Date.now()) ||
                      !agent.scopes.some((s) => s.projectId === projectId)))
                }
                onClick={() =>
                  void run(() =>
                    admit({
                      key: crypto.randomUUID(),
                      source: selected.executionId,
                      options: {
                        ...options(),
                        target:
                          selected.target === "local" ? "local" : undefined,
                        local:
                          selected.target === "local" && agent
                            ? {
                                localAgentId: agent.localAgentId,
                                allowPrivateNetwork: true,
                              }
                            : undefined,
                        allowInsecureSecrets: rerunHTTPConsent,
                      },
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
      {confirmUnpin && (
        <div role="dialog" aria-label="Confirm expired evidence unpin">
          <p>
            Unpinning ends access immediately because normal retention has
            expired. Automatic cleanup may remove this evidence, with no new
            grace period.
          </p>
          <button
            disabled={busy || disabled}
            onClick={() =>
              void run(() =>
                changePin(confirmUnpin, false).then(() => undefined),
              )
            }
          >
            Confirm unpin expired evidence
          </button>
          <button onClick={() => setConfirmUnpin(undefined)}>
            Cancel unpin
          </button>
        </div>
      )}
      {evidence && <AssertionResults evidence={evidence} />}
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
