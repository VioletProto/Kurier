import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  api,
  ApiError,
  UncertainWrite,
  type RequestConfiguration,
  type RequestDetail,
  type RequestField,
  type SavedRequest,
} from "./api";
import { SessionExpired } from "./auth";

const empty = (): RequestConfiguration => ({
  name: "",
  method: "GET",
  url: "https://",
  headers: [],
  queryParameters: [],
  body: null,
  operationRef: null,
});
function configuration(r: RequestConfiguration): RequestConfiguration {
  return {
    name: r.name,
    method: r.method,
    url: r.url,
    queryParameters: r.queryParameters,
    headers: r.headers,
    body: r.body,
    operationRef: null,
  };
}
// Go's normalized JSON encoder additionally escapes these two Unicode separators.
function configurationBytes(c: RequestConfiguration) {
  return new TextEncoder().encode(
    JSON.stringify(configuration(c))
      .replaceAll("\u2028", "\\u2028")
      .replaceAll("\u2029", "\\u2029"),
  ).length;
}
function credentialName(name: string) {
  const n = name.toLowerCase().replace(/[-_. ]/g, "");
  return (
    /authorization|cookie|apikey|token|secret|password|passwd|credential|privatekey|signature|sessionid/.test(
      n,
    ) || ["key", "auth", "pwd"].includes(n)
  );
}
function validate(c: RequestConfiguration) {
  if (
    !c.name.trim() ||
    [...c.name.trim()].length > 100 ||
    /\p{Cc}/u.test(c.name)
  )
    return "Provide a request name of 1–100 characters without controls.";
  try {
    const u = new URL(c.url);
    if (
      !["http:", "https:"].includes(u.protocol) ||
      u.username ||
      u.password ||
      u.hash ||
      /[ {}\p{Cc}]/u.test(c.url)
    )
      return "Use an absolute public HTTP(S) URL without credentials, templates or fragments.";
    for (const [key] of u.searchParams)
      if (credentialName(key))
        return "Credential query parameters cannot be saved.";
  } catch {
    return "Provide an absolute public HTTP(S) URL.";
  }
  for (const f of [...c.headers, ...c.queryParameters])
    if (!f.name || credentialName(f.name) || /[\r\n]/.test(f.name + f.value))
      return "Use named public headers/query fields. Credentials and line breaks cannot be saved.";
  if (c.body?.type === "json")
    try {
      JSON.parse(c.body.text);
    } catch {
      return "The JSON body must contain valid JSON.";
    }
  if (configurationBytes(c) > 65536)
    return "The complete saved configuration exceeds 64 KiB. Reduce the URL, headers, query or body.";
  return "";
}
function Fields({
  title,
  values,
  disabled,
  change,
}: {
  title: string;
  values: RequestField[];
  disabled: boolean;
  change: (values: RequestField[]) => void;
}) {
  return (
    <fieldset disabled={disabled}>
      <legend>{title}</legend>
      {values.map((field, i) => (
        <div className="request-field" key={i}>
          <label>
            {title} name {i + 1}
            <input
              autoComplete="off"
              value={field.name}
              onChange={(e) =>
                change(
                  values.map((v, n) =>
                    n === i ? { ...v, name: e.target.value } : v,
                  ),
                )
              }
            />
          </label>
          <label>
            {title} value {i + 1}
            <input
              autoComplete="off"
              value={field.value}
              onChange={(e) =>
                change(
                  values.map((v, n) =>
                    n === i ? { ...v, value: e.target.value } : v,
                  ),
                )
              }
            />
          </label>
          <label>
            <input
              type="checkbox"
              checked={field.enabled}
              onChange={(e) =>
                change(
                  values.map((v, n) =>
                    n === i ? { ...v, enabled: e.target.checked } : v,
                  ),
                )
              }
            />
            Enabled
          </label>
          <button
            type="button"
            onClick={() => change(values.filter((_, n) => n !== i))}
          >
            Remove {title.toLowerCase()} {i + 1}
          </button>
        </div>
      ))}
      <button
        type="button"
        disabled={disabled || values.length >= 100}
        onClick={() =>
          change([
            ...values,
            { name: "", value: "", enabled: true, sensitive: false },
          ])
        }
      >
        Add {title.toLowerCase()}
      </button>
    </fieldset>
  );
}
export function Requests({
  projectId,
  disabled,
  onExpired,
  onGateChanged,
}: {
  projectId: string;
  disabled: boolean;
  onExpired: () => void;
  onGateChanged: () => void;
}) {
  const mounted = useRef(true);
  const [items, setItems] = useState<SavedRequest[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [detail, setDetail] = useState<RequestDetail>();
  const [draft, setDraft] = useState<RequestConfiguration>(empty);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [blocked, setBlocked] = useState(false);
  const [uncertainCreate, setUncertainCreate] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [publicConfirmed, setPublicConfirmed] = useState(false);
  function failure(e: unknown) {
    if (!mounted.current) return;
    if (e instanceof SessionExpired) {
      onExpired();
      return;
    }
    setError(e instanceof Error ? e.message : "Unable to complete this step.");
    if (
      e instanceof UncertainWrite ||
      (e instanceof ApiError && [409, 412].includes(e.status))
    ) {
      setBlocked(true);
      onGateChanged();
    }
    if (e instanceof ApiError && e.status === 404) {
      setDetail(undefined);
      setEditing(false);
      setBlocked(true);
      onGateChanged();
    }
  }
  useEffect(() => {
    mounted.current = true;
    const controller = new AbortController();
    api
      .listRequests(projectId, undefined, controller.signal)
      .then((page) => {
        if (!controller.signal.aborted) {
          setItems(page.items);
          setCursor(page.nextCursor);
        }
      })
      .catch((e) => {
        if (!controller.signal.aborted) failure(e);
      })
      .finally(() => {
        if (!controller.signal.aborted) setBusy(false);
      });
    return () => {
      mounted.current = false;
      controller.abort();
    };
    // This workspace is keyed by project ID; session callback belongs to its parent.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId]);
  async function run(task: () => Promise<void>) {
    if (busy || disabled) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      await task();
    } catch (e) {
      failure(e);
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  async function refresh() {
    const page = await api.listRequests(projectId);
    if (!mounted.current) return;
    setItems(page.items);
    setCursor(page.nextCursor);
    // Creation has no idempotency receipt. A list refresh alone does not prove
    // absence under eventual consistency, so explicit review is still required.
    if (uncertainCreate)
      setMessage(
        "Review all pages and refresh again if necessary. Creation may have succeeded; do not duplicate it.",
      );
  }
  async function open(id: string) {
    const next = await api.requestDetail(projectId, id);
    if (!mounted.current) return;
    setDetail(next);
    setDraft(configuration(next.request));
    setEditing(true);
    setBlocked(false);
    setConfirmDelete(false);
    setPublicConfirmed(false);
  }
  async function save(event: FormEvent) {
    event.preventDefault();
    if (blocked || uncertainCreate || !publicConfirmed) return;
    const c = { ...draft, name: draft.name.trim() };
    const invalid = validate(c);
    if (invalid) {
      setError(invalid);
      return;
    }
    await run(async () => {
      let next: RequestDetail;
      try {
        next = detail
          ? await api.patchRequest(
              projectId,
              detail.request.requestId,
              c,
              detail.etag,
            )
          : await api.createRequest(projectId, c);
      } catch (e) {
        if (!detail && e instanceof UncertainWrite) setUncertainCreate(true);
        throw e;
      }
      if (!mounted.current) return;
      setDetail(next);
      setDraft(configuration(next.request));
      setItems((old) => [
        next.request,
        ...old.filter((r) => r.requestId !== next.request.requestId),
      ]);
      setPublicConfirmed(false);
      setMessage(
        "Request saved. Refresh project details before renaming or deleting the project.",
      );
      onGateChanged();
    });
  }
  const locked = busy || disabled || blocked || uncertainCreate;
  return (
    <section
      className="panel requests"
      aria-label="Saved requests"
      aria-busy={busy}
    >
      <h3>Saved requests</h3>
      <p>
        Save public, non-sensitive requests. Credentials, cookies, API keys and
        user-designated secrets are unavailable until encrypted secret storage
        exists. Do not enter private data or signed URLs. Requests are saved
        here; execution is not available yet.
      </p>
      <p className="hint">
        64 KiB applies to the complete saved configuration: name, method, URL,
        headers, query, body and JSON overhead.
      </p>
      <p role="status">{busy ? "Loading requests…" : message}</p>
      <p role="alert">{error}</p>
      <button disabled={busy || disabled} onClick={() => void run(refresh)}>
        Refresh requests
      </button>
      <button
        disabled={busy || disabled || uncertainCreate}
        onClick={() => {
          setDetail(undefined);
          setDraft(empty());
          setEditing(true);
          setBlocked(false);
          setConfirmDelete(false);
          setPublicConfirmed(false);
          setError("");
        }}
      >
        New request
      </button>
      <ul className="projects">
        {items.map((r) => (
          <li key={r.requestId}>
            <button
              disabled={busy || disabled}
              onClick={() => void run(() => open(r.requestId))}
            >
              {r.method} {r.name}
            </button>
          </li>
        ))}
      </ul>
      {!busy && items.length === 0 && <p>No saved requests on this page.</p>}
      {cursor !== null && (
        <button
          disabled={busy || disabled}
          onClick={() =>
            void run(async () => {
              const page = await api.listRequests(projectId, cursor);
              if (!mounted.current) return;
              setItems((old) => [
                ...old,
                ...page.items.filter(
                  (r) => !old.some((v) => v.requestId === r.requestId),
                ),
              ]);
              setCursor(page.nextCursor);
            })
          }
        >
          Load more requests
        </button>
      )}
      {uncertainCreate && (
        <div role="status">
          <p>
            Creation may have succeeded. Review the list, including other pages,
            before starting another request.
          </p>
          <button
            disabled={busy || disabled}
            onClick={() => {
              setUncertainCreate(false);
              setBlocked(false);
              setDetail(undefined);
              setEditing(false);
              setDraft(empty());
            }}
          >
            I reviewed the list; start over
          </button>
        </div>
      )}
      {editing && (
        <>
          <h4>
            {detail
              ? `Edit request (revision ${detail.request.revision})`
              : "Create saved request"}
          </h4>
          {detail && (
            <button
              disabled={busy || disabled}
              onClick={() => void run(() => open(detail.request.requestId))}
            >
              Refresh request details
            </button>
          )}
          {blocked && (
            <p role="status">
              The saved version or write outcome changed. Refresh request
              details to review the server version before submitting again. Your
              draft is kept until you refresh.
            </p>
          )}
          <form onSubmit={save}>
            <fieldset disabled={locked}>
              <label htmlFor="request-name">Request name</label>
              <input
                id="request-name"
                autoComplete="off"
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                required
              />
              <label htmlFor="request-method">Method</label>
              <select
                id="request-method"
                value={draft.method}
                onChange={(e) =>
                  setDraft({
                    ...draft,
                    method: e.target.value as RequestConfiguration["method"],
                  })
                }
              >
                {["GET", "POST", "PUT", "PATCH", "DELETE"].map((m) => (
                  <option key={m}>{m}</option>
                ))}
              </select>
              <label htmlFor="request-url">Public request URL</label>
              <input
                id="request-url"
                autoComplete="off"
                value={draft.url}
                onChange={(e) => setDraft({ ...draft, url: e.target.value })}
                required
              />
              <Fields
                title="Header"
                values={draft.headers}
                disabled={locked}
                change={(headers) => setDraft({ ...draft, headers })}
              />
              <Fields
                title="Query parameter"
                values={draft.queryParameters}
                disabled={locked}
                change={(queryParameters) =>
                  setDraft({ ...draft, queryParameters })
                }
              />
              <label htmlFor="request-body-type">Body type</label>
              <select
                id="request-body-type"
                value={draft.body?.type ?? "none"}
                onChange={(e) =>
                  setDraft({
                    ...draft,
                    body:
                      e.target.value === "none"
                        ? null
                        : {
                            type: e.target.value as "text" | "json",
                            text: draft.body?.text ?? "",
                            sensitive: false,
                          },
                  })
                }
              >
                <option value="none">No body</option>
                <option value="text">Text</option>
                <option value="json">JSON</option>
              </select>
              {draft.body && (
                <>
                  <label htmlFor="request-body">Public request body</label>
                  <textarea
                    id="request-body"
                    rows={8}
                    autoComplete="off"
                    spellCheck={false}
                    value={draft.body.text}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        body: { ...draft.body!, text: e.target.value },
                      })
                    }
                  />
                </>
              )}
              <p>
                {configurationBytes({
                  ...draft,
                  name: draft.name.trim(),
                }).toLocaleString()}{" "}
                / 65,536 bytes for the complete configuration
              </p>
              <label>
                <input
                  type="checkbox"
                  checked={publicConfirmed}
                  onChange={(e) => setPublicConfirmed(e.target.checked)}
                />
                All fields contain public data, with no credentials or secrets.
              </label>
              <button
                className="primary"
                type="submit"
                disabled={locked || !publicConfirmed}
              >
                {detail ? "Save request changes" : "Create request"}
              </button>
            </fieldset>
          </form>
          {detail &&
            (confirmDelete ? (
              <div>
                <p>Delete this saved request?</p>
                <button
                  className="danger"
                  disabled={locked}
                  onClick={() =>
                    void run(async () => {
                      await api.deleteRequest(
                        projectId,
                        detail.request.requestId,
                        detail.etag,
                      );
                      if (!mounted.current) return;
                      setItems((old) =>
                        old.filter(
                          (r) => r.requestId !== detail.request.requestId,
                        ),
                      );
                      setDetail(undefined);
                      setEditing(false);
                      setConfirmDelete(false);
                      setMessage("Request deleted.");
                      onGateChanged();
                    })
                  }
                >
                  Confirm request deletion
                </button>
                <button
                  disabled={busy || disabled}
                  onClick={() => setConfirmDelete(false)}
                >
                  Cancel request deletion
                </button>
              </div>
            ) : (
              <button
                className="danger"
                disabled={locked}
                onClick={() => setConfirmDelete(true)}
              >
                Delete request
              </button>
            ))}
        </>
      )}
    </section>
  );
}
