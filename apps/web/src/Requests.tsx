import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  api,
  ApiError,
  UncertainWrite,
  type RequestConfiguration,
  type RequestDetail,
  type RequestField,
  type SavedRequest,
  type SecretMetadata,
  type SecretField,
} from "./api";
import { SessionExpired } from "./auth";
import { Executions } from "./Executions";

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
    queryParameters: r.queryParameters.map(editField),
    headers: r.headers.map(editField),
    body: r.body ? editBody(r.body) : null,
    operationRef: null,
  };
}
function editField(f: RequestField): RequestField {
  if (!f.sensitive) return { ...f };
  return {
    name: f.name,
    enabled: f.enabled,
    sensitive: true,
    bindingId: f.bindingId,
    secretWrite: f.secretWrite ?? {
      action: "preserve",
      secretRef: f.secretRef,
    },
  };
}
function editBody(
  b: NonNullable<RequestConfiguration["body"]>,
): NonNullable<RequestConfiguration["body"]> {
  if (b.sensitive)
    return {
      type: b.type,
      sensitive: true,
      bindingId: b.bindingId,
      secretWrite: b.secretWrite ?? {
        action: "preserve",
        secretRef: b.secretRef,
      },
    };
  return {
    ...b,
    secretFields: b.secretFields?.map((f) => ({
      pointer: f.pointer,
      bindingId: f.bindingId,
      secretWrite: f.secretWrite ?? {
        action: "preserve",
        secretRef: f.secretRef,
      },
    })),
  };
}
function cleared(c: RequestConfiguration): RequestConfiguration {
  const next = configuration(c);
  for (const f of [
    ...next.headers,
    ...next.queryParameters,
    ...(next.body ? [next.body, ...(next.body.secretFields ?? [])] : []),
  ]) {
    if (f.secretWrite?.action === "set")
      f.secretWrite = { action: "set", value: "" };
  }
  return next;
}
function sized(c: RequestConfiguration): RequestConfiguration {
  const next = configuration(c);
  for (const f of [
    ...next.headers,
    ...next.queryParameters,
    ...(next.body ? [next.body, ...(next.body.secretFields ?? [])] : []),
  ]) {
    if (f.secretWrite) {
      const secretRef = f.secretWrite.secretRef ?? {
        secretId: "00000000-0000-4000-8000-000000000000",
      };
      delete f.secretWrite;
      f.secretRef = secretRef;
      f.masked = true;
    }
  }
  return next;
}
// Go's normalized JSON encoder additionally escapes these two Unicode separators.
function configurationBytes(c: RequestConfiguration) {
  return new TextEncoder().encode(
    JSON.stringify(sized(c))
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
  for (const f of c.headers)
    if (!/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(f.name))
      return "Header names must be HTTP tokens without spaces. Use the exact name required by your API, such as Authorization or X-Api-Key.";
  for (const f of [...c.headers, ...c.queryParameters])
    if (
      !f.name ||
      (!f.sensitive && credentialName(f.name)) ||
      /[\r\n]/.test(f.name + (f.value ?? ""))
    )
      return "Use named public headers/query fields or protected credential inputs. Line breaks cannot be saved.";
  if (c.body?.type === "json" && !c.body.sensitive)
    try {
      JSON.parse(c.body.text ?? "");
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
  secrets,
}: {
  secrets: SecretMetadata[];
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
              type={field.sensitive ? "password" : "text"}
              spellCheck={false}
              placeholder={
                field.sensitive
                  ? field.secretWrite?.action === "preserve"
                    ? "Stored secret preserved; enter to replace"
                    : "Write-only value"
                  : ""
              }
              value={
                field.sensitive
                  ? (field.secretWrite?.value ?? "")
                  : (field.value ?? "")
              }
              onChange={(e) =>
                change(
                  values.map((v, n) =>
                    n === i
                      ? v.sensitive
                        ? {
                            ...v,
                            secretWrite: {
                              action: "set",
                              value: e.target.value,
                            },
                          }
                        : { ...v, value: e.target.value }
                      : v,
                  ),
                )
              }
            />
          </label>
          <label>
            <input
              type="checkbox"
              checked={field.sensitive}
              onChange={(e) =>
                change(
                  values.map((v, n) =>
                    n === i
                      ? e.target.checked
                        ? {
                            name: v.name,
                            enabled: v.enabled,
                            sensitive: true,
                            bindingId: crypto.randomUUID(),
                            secretWrite: { action: "set", value: "" },
                          }
                        : {
                            name: v.name,
                            enabled: v.enabled,
                            sensitive: false,
                            value: "",
                          }
                      : v,
                  ),
                )
              }
            />
            Protected
          </label>
          {field.sensitive && (
            <label>
              Reuse project secret
              <select
                aria-label={`${title} secret ${i + 1}`}
                value={field.secretWrite?.secretRef?.secretId ?? ""}
                onChange={(e) =>
                  change(
                    values.map((v, n) =>
                      n === i
                        ? {
                            ...v,
                            secretWrite: e.target.value
                              ? {
                                  action: "secretRef",
                                  secretRef: { secretId: e.target.value },
                                }
                              : { action: "set", value: "" },
                          }
                        : v,
                    ),
                  )
                }
              >
                <option value="">New value</option>
                {secrets
                  .filter(
                    (s) =>
                      s.state === "active" &&
                      s.valueKind === "string" &&
                      (title === "Header" ? s.headerSafe : s.querySafe),
                  )
                  .map((s) => (
                    <option key={s.secretId} value={s.secretId}>
                      {s.secretId} (masked)
                    </option>
                  ))}
              </select>
            </label>
          )}
          <button
            type="button"
            disabled={i === 0}
            onClick={() => {
              const next = [...values];
              [next[i - 1], next[i]] = [next[i], next[i - 1]];
              change(next);
            }}
          >
            Move {title.toLowerCase()} {i + 1} up
          </button>
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
  const [secrets, setSecrets] = useState<SecretMetadata[]>([]);
  const [secretCursor, setSecretCursor] = useState<string | null>(null);
  const [sharedValues, setSharedValues] = useState<Record<string, string>>({});
  const [sharedBlocked, setSharedBlocked] = useState(false);
  const [sharedConfirm, setSharedConfirm] = useState<string>();
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
    setDraft(cleared(draft));
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
      <Executions
        projectId={projectId}
        request={detail}
        disabled={disabled}
        onSessionExpired={onExpired}
        onGateChanged={onGateChanged}
      />
      <h3>Saved requests</h3>
      <p>
        Save public configuration with encrypted bearer tokens, API keys and
        designated sensitive fields. Mark credential rows Protected before
        entering values. For bearer tokens, use Authorization and enter the full
        Bearer value. Header names must match the API and cannot contain spaces.
        API keys use protected header/query rows. Protected inputs are
        write-only and clear after submission. Execute the saved revision below.
      </p>
      <p>
        URLs must be public. Checks reject recognized credential patterns,
        userinfo, templates and signed credential queries; they cannot detect
        arbitrary unmarked secrets. Sensitive URL paths and inline credential
        queries are unsupported: use separate protected query rows.
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
      <button
        disabled={busy || disabled}
        onClick={() =>
          void run(async () => {
            const p = await api.listSecrets(projectId);
            setSecrets(p.items);
            setSecretCursor(p.nextCursor);
            setSharedValues({});
            setSharedConfirm(undefined);
            setSharedBlocked(false);
          })
        }
      >
        Refresh project secrets
      </button>
      {secretCursor && (
        <button
          disabled={busy || disabled}
          onClick={() =>
            void run(async () => {
              const p = await api.listSecrets(projectId, secretCursor);
              setSecrets((old) => [...old, ...p.items]);
              setSecretCursor(p.nextCursor);
            })
          }
        >
          Load more secrets
        </button>
      )}
      {secrets.length > 0 && (
        <fieldset disabled={busy || disabled || sharedBlocked}>
          <legend>Project-owned secrets</legend>
          <p>
            Removing a request binding detaches it. Request-local replacement
            creates a new secret. Shared replacement or revocation affects every
            request referencing this ID. Secrets survive request deletion.
          </p>
          {secrets.map((s) => (
            <div key={s.secretId}>
              <p>
                {s.secretId} — {s.mask} — {s.state} — revision {s.revision}
              </p>
              {s.state === "active" && (
                <>
                  <label>
                    Shared replacement value {s.secretId}
                    <input
                      type="password"
                      autoComplete="off"
                      spellCheck={false}
                      value={sharedValues[s.secretId] ?? ""}
                      onChange={(e) =>
                        setSharedValues({
                          ...sharedValues,
                          [s.secretId]: e.target.value,
                        })
                      }
                    />
                  </label>
                  <button onClick={() => setSharedConfirm(s.secretId)}>
                    Review shared change
                  </button>
                  {sharedConfirm === s.secretId && (
                    <>
                      <p>
                        This affects every consumer. Replace updates this ID;
                        revoke removes its active ciphertext and makes every
                        reference unavailable.
                      </p>
                      <button
                        disabled={!sharedValues[s.secretId]}
                        onClick={() => {
                          const value = sharedValues[s.secretId];
                          setSharedValues({});
                          setSharedConfirm(undefined);
                          void run(async () => {
                            const next = await api.replaceSecret(
                              projectId,
                              s,
                              value,
                            );
                            setSecrets((old) =>
                              old.map((v) =>
                                v.secretId === s.secretId ? next : v,
                              ),
                            );
                            onGateChanged();
                          });
                        }}
                      >
                        Replace shared secret
                      </button>
                      <button
                        onClick={() => {
                          setSharedValues({});
                          setSharedConfirm(undefined);
                          void run(async () => {
                            try {
                              await api.revokeSecret(projectId, s);
                            } catch (e) {
                              setSharedBlocked(true);
                              throw e;
                            }
                            setSecrets((old) =>
                              old.map((v) =>
                                v.secretId === s.secretId
                                  ? {
                                      ...v,
                                      state: "revoked",
                                      revision: v.revision + 1,
                                    }
                                  : v,
                              ),
                            );
                            onGateChanged();
                          });
                        }}
                      >
                        Revoke shared secret
                      </button>
                    </>
                  )}
                </>
              )}
            </div>
          ))}
        </fieldset>
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
          <form
            onSubmit={save}
            onInvalidCapture={() => setDraft(cleared(draft))}
          >
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
                secrets={secrets}
                title="Header"
                values={draft.headers}
                disabled={locked}
                change={(headers) => setDraft({ ...draft, headers })}
              />
              <Fields
                secrets={secrets}
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
              {draft.body?.sensitive && (
                <label>
                  Reuse whole-body secret
                  <select
                    value={draft.body.secretWrite?.secretRef?.secretId ?? ""}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        body: {
                          ...draft.body!,
                          secretWrite: e.target.value
                            ? {
                                action: "secretRef",
                                secretRef: { secretId: e.target.value },
                              }
                            : { action: "set", value: "" },
                        },
                      })
                    }
                  >
                    <option value="">New body value</option>
                    {secrets
                      .filter(
                        (s) =>
                          s.state === "active" &&
                          s.valueKind ===
                            (draft.body!.type === "json"
                              ? "jsonBody"
                              : "string"),
                      )
                      .map((s) => (
                        <option key={s.secretId} value={s.secretId}>
                          {s.secretId} (masked)
                        </option>
                      ))}
                  </select>
                </label>
              )}
              {draft.body && (
                <label>
                  <input
                    type="checkbox"
                    checked={draft.body.sensitive}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        body: e.target.checked
                          ? {
                              type: draft.body!.type,
                              sensitive: true,
                              bindingId: crypto.randomUUID(),
                              secretWrite: { action: "set", value: "" },
                            }
                          : {
                              type: draft.body!.type,
                              sensitive: false,
                              text: "",
                            },
                      })
                    }
                  />
                  Protect whole body
                </label>
              )}
              {draft.body && (
                <>
                  <label htmlFor="request-body">
                    {draft.body.sensitive
                      ? "Write-only protected body"
                      : "Public request body"}
                  </label>
                  <textarea
                    placeholder={
                      draft.body.sensitive
                        ? "Stored body is never returned; enter a replacement"
                        : ""
                    }
                    id="request-body"
                    rows={8}
                    autoComplete="off"
                    spellCheck={false}
                    value={
                      draft.body.sensitive
                        ? (draft.body.secretWrite?.value ?? "")
                        : (draft.body.text ?? "")
                    }
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        body: draft.body!.sensitive
                          ? {
                              ...draft.body!,
                              secretWrite: {
                                action: "set",
                                value: e.target.value,
                              },
                            }
                          : { ...draft.body!, text: e.target.value },
                      })
                    }
                  />
                </>
              )}
              {draft.body?.type === "json" && !draft.body.sensitive && (
                <fieldset>
                  <legend>Protected JSON scalar fields</legend>
                  <p>
                    Use null placeholders in the public JSON. Enter canonical
                    RFC 6901 pointers; values are JSON-encoded scalars, e.g. a
                    quoted string. Remove credential keys from the template when
                    removing their protection.
                  </p>
                  {(draft.body.secretFields ?? []).map((f, i) => (
                    <div key={f.bindingId}>
                      <label>
                        JSON pointer {i + 1}
                        <input
                          value={f.pointer}
                          onChange={(e) =>
                            setDraft({
                              ...draft,
                              body: {
                                ...draft.body!,
                                secretFields: draft.body!.secretFields!.map(
                                  (v, n) =>
                                    n === i
                                      ? { ...v, pointer: e.target.value }
                                      : v,
                                ),
                              },
                            })
                          }
                        />
                      </label>
                      <label>
                        Protected JSON value {i + 1}
                        <input
                          type="password"
                          autoComplete="off"
                          spellCheck={false}
                          value={f.secretWrite?.value ?? ""}
                          placeholder="Write-only JSON scalar"
                          onChange={(e) =>
                            setDraft({
                              ...draft,
                              body: {
                                ...draft.body!,
                                secretFields: draft.body!.secretFields!.map(
                                  (v, n) =>
                                    n === i
                                      ? {
                                          ...v,
                                          secretWrite: {
                                            action: "set",
                                            value: e.target.value,
                                          },
                                        }
                                      : v,
                                ),
                              },
                            })
                          }
                        />
                      </label>
                      <label>
                        Reuse JSON scalar secret {i + 1}
                        <select
                          value={f.secretWrite?.secretRef?.secretId ?? ""}
                          onChange={(e) =>
                            setDraft({
                              ...draft,
                              body: {
                                ...draft.body!,
                                secretFields: draft.body!.secretFields!.map(
                                  (v, n) =>
                                    n === i
                                      ? {
                                          ...v,
                                          secretWrite: e.target.value
                                            ? {
                                                action: "secretRef",
                                                secretRef: {
                                                  secretId: e.target.value,
                                                },
                                              }
                                            : { action: "set", value: "" },
                                        }
                                      : v,
                                ),
                              },
                            })
                          }
                        >
                          <option value="">New scalar value</option>
                          {secrets
                            .filter(
                              (s) =>
                                s.state === "active" &&
                                s.valueKind === "jsonScalar",
                            )
                            .map((s) => (
                              <option key={s.secretId} value={s.secretId}>
                                {s.secretId} (masked)
                              </option>
                            ))}
                        </select>
                      </label>
                      <button
                        type="button"
                        onClick={() =>
                          setDraft({
                            ...draft,
                            body: {
                              ...draft.body!,
                              secretFields: draft.body!.secretFields!.filter(
                                (_, n) => n !== i,
                              ),
                            },
                          })
                        }
                      >
                        Remove JSON binding {i + 1}
                      </button>
                    </div>
                  ))}
                  <button
                    type="button"
                    onClick={() =>
                      setDraft({
                        ...draft,
                        body: {
                          ...draft.body!,
                          secretFields: [
                            ...(draft.body!.secretFields ?? []),
                            {
                              pointer: "",
                              bindingId: crypto.randomUUID(),
                              secretWrite: { action: "set", value: "" },
                            } as SecretField,
                          ],
                        },
                      })
                    }
                  >
                    Add protected JSON field
                  </button>
                </fieldset>
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
                All fields contain public data unless explicitly protected; the
                URL contains no credentials or secrets.
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
