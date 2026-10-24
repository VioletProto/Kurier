import { EnvironmentSelector } from "./Environments";
import { AssertionEditor, AssertionResults } from "./Assertions";
import { validStepAssertions } from "./assertions";
import { useEffect, useRef, useState } from "react";
import {
  api,
  ApiError,
  UncertainWrite,
  type EnvironmentSelection,
  type Workflow,
  type WorkflowDefinition,
  type WorkflowStep,
  type WorkflowRun,
  type SavedRequest,
  type Evidence,
  type WorkflowBinding,
} from "./api";
import { SessionExpired } from "./auth";
const empty = (): WorkflowDefinition => ({ name: "", steps: [] });
const terminal = (run: WorkflowRun) =>
  ["completed", "failed"].includes(run.state);
export function Workflows({
  projectId,
  onExpired,
  onGateChanged,
  disabled,
}: {
  projectId: string;
  onExpired: () => void;
  onGateChanged: () => void;
  disabled: boolean;
}) {
  const [environment, setEnvironment] = useState<EnvironmentSelection | null>(
    null,
  );
  const [opened, setOpened] = useState(false),
    [items, setItems] = useState<Workflow[]>([]),
    [requests, setRequests] = useState<SavedRequest[]>([]),
    [runs, setRuns] = useState<WorkflowRun[]>([]),
    [selected, setSelected] = useState<Workflow>(),
    [draft, setDraft] = useState(empty),
    [run, setRun] = useState<WorkflowRun>(),
    [evidence, setEvidence] = useState<Evidence>(),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [allow, setAllow] = useState(false),
    [cursor, setCursor] = useState<string | null>(null),
    [runCursor, setRunCursor] = useState<string | null>(null);
  const [requestCursor, setRequestCursor] = useState<string | null>(null);
  const lifetime = useRef(new AbortController()),
    lock = useRef(false),
    runRef = useRef(run),
    uncertain = useRef<{
      key: string;
      source: string;
      workflow?: Workflow;
      allow: boolean;
      created: number;
      environment: EnvironmentSelection | null;
    } | null>(null);
  useEffect(() => {
    runRef.current = run;
  }, [run]);
  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    return () => {
      controller.abort();
      uncertain.current = null;
    };
  }, [projectId]);
  const action = async (fn: () => Promise<void>) => {
    if (lock.current || disabled) return;
    lock.current = true;
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      if (lifetime.current.signal.aborted) return;
      if (e instanceof SessionExpired) onExpired();
      else {
        setError(
          e instanceof UncertainWrite
            ? "Result uncertain. Refresh before a deliberate retry; the original submission key is retained."
            : e instanceof ApiError
              ? e.message
              : "Workflow operation failed. Refresh and review before retrying.",
        );
        if (e instanceof ApiError && e.status === 404) {
          setEvidence(undefined);
          setRun(undefined);
        }
      }
    } finally {
      lock.current = false;
      if (!lifetime.current.signal.aborted) setBusy(false);
    }
  };
  const refresh = async () => {
    const signal = lifetime.current.signal;
    const [w, r, q] = await Promise.all([
      api.workflows(projectId, undefined, signal),
      api.workflowRuns(projectId, undefined, signal),
      api.listRequests(projectId, undefined, signal),
    ]);
    if (signal.aborted) return;
    setItems(w.items);
    setCursor(w.nextCursor);
    setRuns(r.items);
    setRunCursor(r.nextCursor);
    setRequests(q.items);
    setRequestCursor(q.nextCursor);
    if (runRef.current) {
      const latest = await api.workflowRun(
        projectId,
        runRef.current.workflowRunId,
        signal,
      );
      if (!signal.aborted) setRun(latest);
    }
  };
  useEffect(() => {
    if (!run || terminal(run) || !opened) return;
    let timer: ReturnType<typeof setTimeout>;
    let cancelled = false;
    let wait = 10000;
    const tick = async () => {
      if (cancelled) return;
      if (document.hidden || lock.current) {
        timer = setTimeout(tick, 10000);
        return;
      }
      lock.current = true;
      try {
        const v = await api.workflowRun(
          projectId,
          run.workflowRunId,
          lifetime.current.signal,
        );
        if (!cancelled) {
          setRun(v);
          wait = 10000;
        }
      } catch (e) {
        if (e instanceof SessionExpired) onExpired();
        wait = Math.min(wait * 2, 60000);
      } finally {
        lock.current = false;
        if (!cancelled) timer = setTimeout(tick, wait + Math.random() * 1000);
      }
    };
    timer = setTimeout(tick, 10000);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [run, opened, projectId, onExpired]);
  const change = (pos: number, fn: (s: WorkflowStep) => WorkflowStep) =>
    setDraft((d) => ({
      ...d,
      steps: d.steps.map((s, i) => (i === pos ? fn(structuredClone(s)) : s)),
    }));
  const submit = async (source: string, workflow?: Workflow) => {
    if (uncertain.current) throw new Error("Resolve original admission first");
    const admission = {
      source,
      workflow,
      key: crypto.randomUUID(),
      allow,
      environment,
      created: Date.now(),
    };
    uncertain.current = admission;
    try {
      const v = await api.submitWorkflow(
        projectId,
        source,
        admission.key,
        allow,
        workflow,
        lifetime.current.signal,
        workflow ? admission.environment : undefined,
      );
      if (lifetime.current.signal.aborted) return;
      uncertain.current = null;
      setRun(v);
      setEvidence(undefined);
      setAllow(false);
      onGateChanged();
      await refresh();
    } catch (e) {
      if (!(e instanceof UncertainWrite)) uncertain.current = null;
      throw e;
    }
  };
  const edit = (w: Workflow) => {
    setSelected(w);
    setDraft(structuredClone({ name: w.name, steps: w.steps }));
    setAllow(false);
  };
  return (
    <section aria-label="Workflows">
      <h3>Sequential cloud workflows</h3>
      <p>
        2–10 ordered saved requests. Runtime values stay encrypted and are never
        displayed.
      </p>
      {!opened ? (
        <button
          disabled={disabled || busy}
          onClick={() =>
            void action(async () => {
              await refresh();
              setOpened(true);
            })
          }
        >
          Load workflows
        </button>
      ) : (
        <>
          <button
            disabled={busy || disabled}
            onClick={() => void action(refresh)}
          >
            Refresh workflows and runs
          </button>
          <button
            disabled={busy}
            onClick={() => {
              setSelected(undefined);
              setDraft(empty());
            }}
          >
            New workflow
          </button>
          <ul>
            {items.map((w) => (
              <li key={w.workflowId}>
                <button disabled={busy} onClick={() => edit(w)}>
                  {w.name} (revision {w.revision})
                </button>
              </li>
            ))}
          </ul>
          {cursor && (
            <button
              disabled={busy}
              onClick={() =>
                void action(async () => {
                  const p = await api.workflows(
                    projectId,
                    cursor,
                    lifetime.current.signal,
                  );
                  if (!lifetime.current.signal.aborted) {
                    setItems((v) => [...v, ...p.items]);
                    setCursor(p.nextCursor);
                  }
                })
              }
            >
              More workflows
            </button>
          )}
          {requestCursor && (
            <button
              disabled={busy}
              onClick={() =>
                void action(async () => {
                  const p = await api.listRequests(
                    projectId,
                    requestCursor,
                    lifetime.current.signal,
                  );
                  if (!lifetime.current.signal.aborted) {
                    setRequests((v) => [...v, ...p.items]);
                    setRequestCursor(p.nextCursor);
                  }
                })
              }
            >
              More saved requests
            </button>
          )}
          <label>
            Workflow name
            <input
              value={draft.name}
              disabled={busy}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            />
          </label>
          {draft.steps.map((step, pos) => {
            const request = requests.find(
              (r) => r.requestId === step.requestId,
            );
            return (
              <fieldset key={step.stepId} disabled={busy}>
                <legend>Step {pos + 1}</legend>
                <label>
                  Saved request
                  <select
                    value={step.requestId}
                    onChange={(e) => {
                      const r = requests.find(
                        (r) => r.requestId === e.target.value,
                      );
                      if (r)
                        change(pos, (s) => ({
                          ...s,
                          requestId: r.requestId,
                          requestRevision: r.revision,
                          bindings: [],
                        }));
                    }}
                  >
                    {requests.map((r) => (
                      <option key={r.requestId} value={r.requestId}>
                        {r.name} (revision {r.revision})
                      </option>
                    ))}
                  </select>
                </label>
                <p>Frozen source revision: {step.requestRevision}</p>
                <label>
                  Step environment mode {pos + 1}
                  <select
                    value={step.environment?.mode ?? "inherit"}
                    onChange={(e) =>
                      change(pos, (s) => ({
                        ...s,
                        environment:
                          e.target.value === "select"
                            ? { mode: "select", environmentId: "", revision: 0 }
                            : { mode: e.target.value as "inherit" | "none" },
                      }))
                    }
                  >
                    <option value="inherit">Inherit run environment</option>
                    <option value="none">No environment</option>
                    <option value="select">Select environment</option>
                  </select>
                </label>
                {step.environment?.mode === "select" && (
                  <EnvironmentSelector
                    projectId={projectId}
                    label={`Step environment ${pos + 1}`}
                    value={
                      step.environment.environmentId ? step.environment : null
                    }
                    change={(v) =>
                      change(pos, (s) => ({
                        ...s,
                        environment: v
                          ? { mode: "select", ...v }
                          : { mode: "none" },
                      }))
                    }
                    disabled={busy || disabled}
                    onExpired={onExpired}
                  />
                )}

                <button
                  disabled={pos === 0}
                  onClick={() =>
                    setDraft((d) => {
                      const steps = [...d.steps];
                      [steps[pos - 1], steps[pos]] = [
                        steps[pos],
                        steps[pos - 1],
                      ];
                      return { ...d, steps };
                    })
                  }
                >
                  Move up
                </button>
                <button
                  disabled={pos === draft.steps.length - 1}
                  onClick={() =>
                    setDraft((d) => {
                      const steps = [...d.steps];
                      [steps[pos + 1], steps[pos]] = [
                        steps[pos],
                        steps[pos + 1],
                      ];
                      return { ...d, steps };
                    })
                  }
                >
                  Move down
                </button>
                <button
                  onClick={() =>
                    setDraft((d) => ({
                      ...d,
                      steps: d.steps.filter((_, i) => i !== pos),
                    }))
                  }
                >
                  Remove step
                </button>
                <AssertionEditor
                  checks={step.assertions ?? []}
                  onChange={(assertions) =>
                    change(pos, (s) => ({ ...s, assertions }))
                  }
                />
                {step.extract.map((x, i) => (
                  <fieldset key={i}>
                    <legend>Output {i + 1}</legend>
                    <label>
                      Output name
                      <input
                        value={x.name}
                        onChange={(e) =>
                          change(pos, (s) => {
                            s.extract[i].name = e.target.value;
                            return s;
                          })
                        }
                      />
                    </label>
                    <label>
                      JSON path
                      <input
                        value={x.path}
                        onChange={(e) =>
                          change(pos, (s) => {
                            s.extract[i].path = e.target.value;
                            return s;
                          })
                        }
                      />
                    </label>
                    <label>
                      Output type
                      <select
                        value={x.type}
                        onChange={(e) =>
                          change(pos, (s) => {
                            s.extract[i].type = e.target.value as typeof x.type;
                            return s;
                          })
                        }
                      >
                        {["string", "number", "boolean", "null"].map((t) => (
                          <option key={t}>{t}</option>
                        ))}
                      </select>
                    </label>
                    <label>
                      <input
                        type="checkbox"
                        checked={x.sensitive}
                        onChange={(e) =>
                          change(pos, (s) => {
                            s.extract[i].sensitive = e.target.checked;
                            return s;
                          })
                        }
                      />
                      Sensitive output
                    </label>
                    <button
                      onClick={() =>
                        change(pos, (s) => ({
                          ...s,
                          extract: s.extract.filter((_, j) => j !== i),
                        }))
                      }
                    >
                      Remove output
                    </button>
                  </fieldset>
                ))}
                <button
                  disabled={step.extract.length >= 8}
                  onClick={() =>
                    change(pos, (s) => ({
                      ...s,
                      extract: [
                        ...s.extract,
                        {
                          name: "value",
                          path: "$.value",
                          type: "string",
                          sensitive: true,
                        },
                      ],
                    }))
                  }
                >
                  Add output
                </button>
                {step.bindings.map((b, i) => {
                  const producer = draft.steps
                    .find((s) => s.stepId === b.from.stepId)
                    ?.extract.find((x) => x.name === b.from.name);
                  return (
                    <fieldset key={i}>
                      <legend>Binding {i + 1}</legend>
                      <label>
                        Producer output
                        <select
                          value={`${b.from.stepId}/${b.from.name}`}
                          onChange={(e) =>
                            change(pos, (s) => {
                              const [stepId, name] = e.target.value.split("/");
                              s.bindings[i] = {
                                from: { stepId, name },
                                to: { kind: "header", index: 0 },
                              };
                              return s;
                            })
                          }
                        >
                          {draft.steps.slice(0, pos).flatMap((s, j) =>
                            s.extract.map((x) => (
                              <option
                                key={s.stepId + "/" + x.name}
                                value={s.stepId + "/" + x.name}
                              >
                                Step {j + 1}: {x.name}
                              </option>
                            )),
                          )}
                        </select>
                      </label>
                      <label>
                        Binding target
                        <select
                          value={b.to.kind}
                          onChange={(e) =>
                            change(pos, (s) => {
                              const kind = e.target
                                .value as WorkflowBinding["to"]["kind"];
                              s.bindings[i].to =
                                kind === "jsonBody"
                                  ? { kind, pointer: "" }
                                  : kind === "textBody"
                                    ? { kind }
                                    : { kind, index: 0 };
                              return s;
                            })
                          }
                        >
                          {["header", "query", "jsonBody", "textBody"].map(
                            (t) => (
                              <option key={t}>{t}</option>
                            ),
                          )}
                        </select>
                      </label>
                      {(b.to.kind === "header" || b.to.kind === "query") && (
                        <label>
                          Destination field
                          <select
                            value={b.to.index}
                            onChange={(e) =>
                              change(pos, (s) => {
                                s.bindings[i].to.index = Number(e.target.value);
                                return s;
                              })
                            }
                          >
                            {(b.to.kind === "header"
                              ? request?.headers
                              : request?.queryParameters
                            )?.map((f, n) => (
                              <option
                                disabled={!f.enabled || f.sensitive}
                                key={n}
                                value={n}
                              >
                                {n}: {f.name}
                              </option>
                            ))}
                          </select>
                        </label>
                      )}
                      {b.to.kind === "jsonBody" && (
                        <label>
                          JSON body pointer
                          <input
                            value={b.to.pointer ?? ""}
                            onChange={(e) =>
                              change(pos, (s) => {
                                s.bindings[i].to.pointer = e.target.value;
                                return s;
                              })
                            }
                          />
                        </label>
                      )}
                      {b.to.kind === "header" &&
                        producer?.type === "string" && (
                          <>
                            <label>
                              Public literal prefix
                              <input
                                value={b.to.prefix ?? ""}
                                onChange={(e) =>
                                  change(pos, (s) => {
                                    s.bindings[i].to.prefix = e.target.value;
                                    return s;
                                  })
                                }
                              />
                            </label>
                            <label>
                              Public literal suffix
                              <input
                                value={b.to.suffix ?? ""}
                                onChange={(e) =>
                                  change(pos, (s) => {
                                    s.bindings[i].to.suffix = e.target.value;
                                    return s;
                                  })
                                }
                              />
                            </label>
                            <p>
                              256 UTF-8 bytes per literal; 2 KiB rendered
                              header. Literals must contain no secrets. Output:
                              prefix + [{producer.name}] + suffix.
                            </p>
                          </>
                        )}
                      <button
                        onClick={() =>
                          change(pos, (s) => ({
                            ...s,
                            bindings: s.bindings.filter((_, j) => j !== i),
                          }))
                        }
                      >
                        Remove binding
                      </button>
                    </fieldset>
                  );
                })}
                <button
                  disabled={
                    step.bindings.length >= 8 ||
                    !draft.steps.slice(0, pos).some((s) => s.extract.length)
                  }
                  onClick={() =>
                    change(pos, (s) => {
                      const previous = draft.steps
                        .slice(0, pos)
                        .find((s) => s.extract.length)!;
                      return {
                        ...s,
                        bindings: [
                          ...s.bindings,
                          {
                            from: {
                              stepId: previous.stepId,
                              name: previous.extract[0].name,
                            },
                            to: { kind: "header", index: 0 },
                          },
                        ],
                      };
                    })
                  }
                >
                  Add binding
                </button>
              </fieldset>
            );
          })}
          <button
            disabled={busy || draft.steps.length >= 10 || !requests.length}
            onClick={() =>
              setDraft((d) => ({
                ...d,
                steps: [
                  ...d.steps,
                  {
                    stepId: crypto.randomUUID(),
                    requestId: requests[0].requestId,
                    requestRevision: requests[0].revision,
                    extract: [],
                    bindings: [],
                  },
                ],
              }))
            }
          >
            Add step
          </button>
          <button
            disabled={
              busy ||
              draft.steps.length < 2 ||
              !draft.name.trim() ||
              !draft.steps.every(validStepAssertions) ||
              draft.steps.some((s, pos) =>
                s.bindings.some(
                  (b) =>
                    !draft.steps
                      .slice(0, pos)
                      .some(
                        (p) =>
                          p.stepId === b.from.stepId &&
                          p.extract.some((x) => x.name === b.from.name),
                      ),
                ),
              )
            }
            onClick={() =>
              void action(async () => {
                const w = await api.saveWorkflow(
                  projectId,
                  draft,
                  selected,
                  lifetime.current.signal,
                );
                if (lifetime.current.signal.aborted) return;
                edit(w);
                onGateChanged();
                await refresh();
              })
            }
          >
            Save workflow
          </button>
          {selected && (
            <>
              <button
                disabled={busy}
                onClick={() =>
                  void action(async () => {
                    await api.deleteWorkflow(
                      projectId,
                      selected,
                      lifetime.current.signal,
                    );
                    if (lifetime.current.signal.aborted) return;
                    setSelected(undefined);
                    setDraft(empty());
                    onGateChanged();
                    await refresh();
                  })
                }
              >
                Delete workflow definition
              </button>
              <p>
                Cloud submission reserves {selected.steps.length} steps against
                execution quotas, including skipped steps.
              </p>
              <button
                disabled={busy || !!uncertain.current}
                onClick={() =>
                  void action(() => submit(selected.workflowId, selected))
                }
              >
                Submit saved workflow
              </button>
            </>
          )}
          <label>
            <input
              type="checkbox"
              checked={allow}
              disabled={busy}
              onChange={(e) => setAllow(e.target.checked)}
            />
            Allow sensitive values over public HTTP for this submission or rerun
          </label>
          <EnvironmentSelector
            projectId={projectId}
            label="Workflow run environment"
            value={environment}
            change={setEnvironment}
            disabled={busy || disabled || !!uncertain.current}
            onExpired={onExpired}
          />
          {uncertain.current && (
            <button
              disabled={busy}
              onClick={() =>
                void action(async () => {
                  const a = uncertain.current!;
                  if (Date.now() - a.created > 7 * 86400000)
                    throw Error("Retry window expired");
                  const v = await api.submitWorkflow(
                    projectId,
                    a.source,
                    a.key,
                    a.allow,
                    a.workflow,
                    lifetime.current.signal,
                    a.workflow ? a.environment : undefined,
                  );
                  if (lifetime.current.signal.aborted) return;
                  uncertain.current = null;
                  setRun(v);
                  setAllow(false);
                  await refresh();
                })
              }
            >
              Reconcile original submission
            </button>
          )}
          <h4>Workflow runs</h4>
          <ul>
            {runs.map((v) => (
              <li key={v.workflowRunId}>
                <button
                  disabled={busy}
                  onClick={() =>
                    void action(async () => {
                      const current = await api.workflowRun(
                        projectId,
                        v.workflowRunId,
                        lifetime.current.signal,
                      );
                      if (!lifetime.current.signal.aborted) {
                        setRun(current);
                        setEvidence(undefined);
                        setAllow(false);
                      }
                    })
                  }
                >
                  {v.workflowRunId}: {v.state}
                </button>
              </li>
            ))}
          </ul>
          {runCursor && (
            <button
              disabled={busy}
              onClick={() =>
                void action(async () => {
                  const p = await api.workflowRuns(
                    projectId,
                    runCursor,
                    lifetime.current.signal,
                  );
                  if (!lifetime.current.signal.aborted) {
                    setRuns((v) => [...v, ...p.items]);
                    setRunCursor(p.nextCursor);
                  }
                })
              }
            >
              More workflow runs
            </button>
          )}
          {run && (
            <section aria-label="Workflow run">
              <p>
                Run {run.workflowRunId}: {run.state}
              </p>
              <ol>
                {run.steps.map((s) => (
                  <li key={s.stepId}>
                    Step {s.position + 1}: {s.state}; request revision{" "}
                    {s.requestRevision}{" "}
                    {s.summary?.outcome.certainty === "unknown" && (
                      <strong>
                        Outcome unknown — do not assume no effect. No automatic
                        HTTP replay.
                      </strong>
                    )}
                    {s.state === "skipped" ? (
                      <span> Skipped; no assertion evaluation.</span>
                    ) : (
                      (s.assertions ?? []).map((a) => (
                        <span key={a.assertionId}>
                          {" "}
                          Check {a.assertionId}: {a.kind}
                          {a.kind !== "statusEquals" ? ` at ${a.path}` : ""}
                          {a.kind === "jsonEquals"
                            ? `; public expected ${JSON.stringify(a.expected)}`
                            : a.kind === "statusEquals"
                              ? `; expected ${a.expected}`
                              : ""}
                          .
                        </span>
                      ))
                    )}
                    {s.extract.map((x) => (
                      <span key={x.name}>
                        {" "}
                        Output {x.name} ({x.type}); value never displayed.
                      </span>
                    ))}
                    {s.executionId && (
                      <>
                        <button
                          disabled={busy || s.evidenceExpired || !s.retention}
                          onClick={() =>
                            void action(async () => {
                              const data = await api.executionEvidence(
                                projectId,
                                s.executionId!,
                                lifetime.current.signal,
                              );
                              if (!lifetime.current.signal.aborted)
                                setEvidence(data);
                            })
                          }
                        >
                          Inspect step {s.position + 1} evidence
                        </button>
                        <button
                          disabled={
                            busy ||
                            !s.retention ||
                            (!s.retention.pinned && !s.retention.pinEligible)
                          }
                          onClick={() =>
                            void action(async () => {
                              if (
                                s.retention?.pinned &&
                                Date.parse(s.retention.normalExpiresAt) <=
                                  Date.now() &&
                                !window.confirm(
                                  "Unpinning ends access immediately. Continue?",
                                )
                              )
                                return;
                              await api.pinExecution(
                                projectId,
                                s.executionId!,
                                !s.retention!.pinned,
                                lifetime.current.signal,
                              );
                              if (!lifetime.current.signal.aborted) {
                                setEvidence(undefined);
                                onGateChanged();
                                await refresh();
                              }
                            })
                          }
                        >
                          {s.retention?.pinned ? "Unpin" : "Pin"} step{" "}
                          {s.position + 1}
                        </button>
                      </>
                    )}
                    {s.evidenceExpired && (
                      <span>
                        Evidence expired; retained run context does not protect
                        this body.
                      </span>
                    )}
                  </li>
                ))}
              </ol>
              <p>
                Pins protect individual evidence from automatic expiry while
                pinned.
              </p>
              <button
                disabled={busy || !!uncertain.current}
                onClick={() => {
                  if (
                    window.confirm(
                      "Rerun every step? This may repeat side effects, including after an unknown outcome.",
                    )
                  )
                    void action(() => submit(run.workflowRunId));
                }}
              >
                Rerun entire workflow
              </button>
            </section>
          )}
          {evidence && <AssertionResults evidence={evidence} />}
          {evidence && (
            <pre aria-label="Workflow evidence">
              {JSON.stringify(evidence, null, 2)}
            </pre>
          )}
        </>
      )}
      {busy && <p role="status">Working…</p>}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
