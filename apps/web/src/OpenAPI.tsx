import { useEffect, useRef, useState, type FormEvent } from "react";
import { api, ApiError, UncertainWrite } from "./api";
import { SessionExpired } from "./auth";
import {
  choicesWithServer,
  importErrorMessage,
  type OpenAPIImport,
  type OpenAPIImportSummary,
  type OpenAPICandidate,
} from "./openapi";

function Inputs({
  candidate,
  disabled,
  save,
}: {
  candidate: OpenAPICandidate;
  disabled: boolean;
  save: (id: string, choices: string) => Promise<void>;
}) {
  const [inputs, setInputs] = useState(candidate.choicesJSON || "{}");
  const [server, setServer] = useState(() => {
    try {
      return (
        JSON.parse(candidate.choicesJSON || "{}").server ||
        (candidate.operation.servers.length === 1
          ? candidate.operation.servers[0]
          : "")
      );
    } catch {
      return "";
    }
  });
  const [error, setError] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const operation = candidate.operation;
  async function submit(e: FormEvent) {
    e.preventDefault();
    let source: string;
    try {
      source = choicesWithServer(inputs, server);
    } catch {
      setError("Enter a JSON object for the operation inputs.");
      return;
    }
    setError("");
    await save(operation.operationId, source);
  }
  return (
    <form onSubmit={(e) => void submit(e)}>
      <fieldset disabled={disabled}>
        <legend>
          {operation.method} {operation.path} ·{" "}
          {candidate.ready ? "Ready" : "Needs input"}
        </legend>
        <label>
          Server
          {operation.servers.length === 0 ? (
            <input
              aria-label={`Server for ${operation.method} ${operation.path}`}
              type="url"
              value={server}
              required
              placeholder="https://api.example.com"
              onChange={(e) => setServer(e.target.value)}
            />
          ) : (
            <select
              aria-label={`Server for ${operation.method} ${operation.path}`}
              value={server}
              required
              onChange={(e) => setServer(e.target.value)}
            >
              <option value="">Choose a server</option>
              {operation.servers.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          )}
        </label>
        <ul>
          {operation.parameters.map((p) => (
            <li key={p.inputId}>
              {p.inputId}: {p.in} {p.name} · {p.schema.type || "value"}
              {p.required ? " · required" : " · optional"}
              {p.sensitive ? " · protected secret reference required" : ""}
            </li>
          ))}
        </ul>
        {operation.requestBody && (
          <p>
            JSON body:{" "}
            {operation.requestBody.required ? "required" : "optional"}.
          </p>
        )}
        <label>
          Operation inputs (JSON)
          <textarea
            value={inputs}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) => setInputs(e.target.value)}
          />
        </label>
        <p>
          Safe documented examples are used when no override is supplied. For
          example: {`{"parameters":{"p0":"item-id"},"body":{"count":1}}`}. Use
          the input IDs above. Keep exact decimal text. For protected parameters
          use {`"secretRefs":{"p0":"existing-secret-id"}`} or{" "}
          {`"bodySecretRef":"existing-secret-id"`}; never enter credentials
          here.
        </p>
        <label>
          <input
            type="checkbox"
            checked={confirmed}
            onChange={(e) => setConfirmed(e.target.checked)}
          />
          These operation inputs contain public values or existing protected
          references.
        </label>
        <button type="submit" disabled={!confirmed}>
          Save operation inputs
        </button>
        {error && <p role="alert">{error}</p>}
      </fieldset>
    </form>
  );
}

export function OpenAPI({
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
  const [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false),
    [uncertain, setUncertain] = useState(false);
  const [items, setItems] = useState<OpenAPIImportSummary[]>([]),
    [cursor, setCursor] = useState<string | null>(null);
  const [detail, setDetail] = useState<OpenAPIImport>();
  const [format, setFormat] = useState<"json" | "yaml">("yaml"),
    [source, setSource] = useState("");
  const [publicData, setPublicData] = useState(false),
    [sensitiveValues, setSensitiveValues] = useState([""]),
    [pointers, setPointers] = useState("");
  const [error, setError] = useState(""),
    [message, setMessage] = useState("");
  const mounted = useRef(true),
    file = useRef<HTMLInputElement>(null);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  async function run(work: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      await work();
    } catch (e) {
      if (!mounted.current) return;
      if (e instanceof SessionExpired) onExpired();
      else if (e instanceof UncertainWrite) {
        setUncertain(true);
        setError(
          "The write outcome is unknown. Refresh imports and inspect the current state before another write.",
        );
      } else
        setError(
          e instanceof ApiError
            ? e.status === 413
              ? "The OpenAPI source or operation inputs exceed the supported size limit. Source is limited to 2 MiB."
              : (importErrorMessage(e.code) ?? e.message)
            : "OpenAPI operation unavailable. Refresh before trying again.",
        );
      if (e instanceof ApiError && e.status === 412) onGateChanged();
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  async function refresh(more = false) {
    const page = await api.listOpenAPIImports(projectId, more ? cursor : null);
    if (!mounted.current) return;
    setItems((old) =>
      more
        ? [
            ...old,
            ...page.items.filter(
              (x) => !old.some((y) => y.importId === x.importId),
            ),
          ]
        : page.items,
    );
    setCursor(page.nextCursor);
    if (detail) {
      const current = await api.openAPIImport(projectId, detail.importId);
      if (mounted.current) setDetail(current);
    }
    setUncertain(false);
  }
  async function upload(e: FormEvent) {
    e.preventDefault();
    if (!publicData || !source.trim()) return;
    if (new TextEncoder().encode(source).length > 2 * 1024 * 1024) {
      setError("The source exceeds the 2 MiB limit.");
      return;
    }
    const payload = {
      format,
      source,
      publicDataConfirmed: true as const,
      sensitiveValues: sensitiveValues.filter(Boolean),
      sensitiveJSONPointers: pointers
        .split("\n")
        .map((p) => p.trim())
        .filter(Boolean),
    };
    // Raw uploads and secret designations are write-only drafts, cleared before transport.
    setSource("");
    setSensitiveValues([""]);
    setPointers("");
    setPublicData(false);
    if (file.current) file.current.value = "";
    await run(async () => {
      const result = await api.createOpenAPIImport(projectId, payload);
      if (mounted.current) {
        setDetail(result);
        setItems((old) => [result, ...old]);
        setMessage(
          "Import prepared. Complete required inputs, then publish the generated requests.",
        );
      }
      onGateChanged();
    });
  }
  async function download(encoding: "json" | "yaml") {
    if (!detail) return;
    const text = await api.exportOpenAPIImport(
      projectId,
      detail.importId,
      encoding,
    );
    if (!mounted.current) return;
    const url = URL.createObjectURL(
      new Blob([text], {
        type: encoding === "json" ? "application/json" : "application/yaml",
      }),
    );
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `openapi-sanitized.${encoding}`;
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
  return (
    <section aria-label="OpenAPI imports">
      <h3>OpenAPI import and export</h3>
      <button
        disabled={disabled || busy}
        onClick={() => {
          setOpen(true);
          void run(() => refresh());
        }}
      >
        {open ? "Refresh imports" : "Open OpenAPI imports"}
      </button>
      {open && (
        <>
          <p>
            Import an OpenAPI 3.0 JSON or YAML file, up to 2 MiB and 100
            operations. Unsupported features produce safe diagnostics.
            References stay within the document. No requests run during import.
          </p>
          <form onSubmit={(e) => void upload(e)}>
            <fieldset disabled={disabled || busy || uncertain}>
              <legend>New OpenAPI import</legend>
              <label>
                Source format
                <select
                  value={format}
                  onChange={(e) => setFormat(e.target.value as "json" | "yaml")}
                >
                  <option value="yaml">YAML</option>
                  <option value="json">JSON</option>
                </select>
              </label>
              <label>
                OpenAPI file
                <input
                  ref={file}
                  type="file"
                  accept=".yaml,.yml,.json,application/json,application/yaml"
                  onChange={(e) => {
                    const chosen = e.target.files?.[0];
                    if (!chosen) return;
                    e.target.value = "";
                    if (chosen.size > 2 * 1024 * 1024) {
                      setError("The source exceeds the 2 MiB limit.");
                      return;
                    }
                    setFormat(
                      chosen.name.toLowerCase().endsWith(".json")
                        ? "json"
                        : "yaml",
                    );
                    void chosen
                      .text()
                      .then((text) => {
                        if (mounted.current) setSource(text);
                      })
                      .catch(() => {
                        if (mounted.current)
                          setError("Cannot read the selected file.");
                      });
                  }}
                />
              </label>
              <label>
                OpenAPI source
                <textarea
                  value={source}
                  autoComplete="off"
                  spellCheck={false}
                  onChange={(e) => setSource(e.target.value)}
                />
              </label>
              <p>
                Source and designations stay in this page's memory until
                submission. Only sanitized structure and safe examples are
                retained. Exports omit removed content and original formatting.
              </p>
              {sensitiveValues.map((value, i) => (
                <label key={i}>
                  Additional sensitive value {i + 1}
                  <input
                    type="password"
                    value={value}
                    autoComplete="new-password"
                    onChange={(e) =>
                      setSensitiveValues((old) =>
                        old.map((v, n) => (n === i ? e.target.value : v)),
                      )
                    }
                  />
                </label>
              ))}
              <button
                type="button"
                onClick={() => setSensitiveValues((old) => [...old, ""])}
                disabled={sensitiveValues.length >= 32}
              >
                Add sensitive value
              </button>
              <label>
                Sensitive JSON pointers (one per line)
                <textarea
                  value={pointers}
                  onChange={(e) => setPointers(e.target.value)}
                />
              </label>
              <label>
                <input
                  type="checkbox"
                  checked={publicData}
                  onChange={(e) => setPublicData(e.target.checked)}
                />
                I confirm the remaining structure and examples are public data
                and have designated any additional secrets.
              </label>
              <button type="submit" disabled={!publicData || !source.trim()}>
                Prepare import
              </button>
            </fieldset>
          </form>
          <ul>
            {items.map((item) => (
              <li key={item.importId}>
                <button
                  disabled={disabled || busy}
                  onClick={() =>
                    void run(async () => {
                      const result = await api.openAPIImport(
                        projectId,
                        item.importId,
                      );
                      if (mounted.current) setDetail(result);
                    })
                  }
                >
                  {item.operationCount} operations · {item.state} ·{" "}
                  {item.importId}
                </button>
              </li>
            ))}
          </ul>
          {cursor && (
            <button
              disabled={disabled || busy}
              onClick={() => void run(() => refresh(true))}
            >
              More imports
            </button>
          )}
          {detail && (
            <div>
              <h4>
                Import: {detail.state} · revision {detail.revision}
              </h4>
              {detail.diagnostics.length > 0 && (
                <ul aria-label="Import diagnostics">
                  {detail.diagnostics.map((d, i) => (
                    <li key={i}>{d.code.replaceAll("_", " ")}</li>
                  ))}
                </ul>
              )}
              <p>
                Contract validation is available for cloud requests and
                workflows. Generated requests can be used locally after
                detaching their contract.
              </p>
              <button
                disabled={disabled || busy}
                onClick={() => void run(() => download("json"))}
              >
                Export sanitized JSON
              </button>
              <button
                disabled={disabled || busy}
                onClick={() => void run(() => download("yaml"))}
              >
                Export sanitized YAML
              </button>
              {detail.state === "preparing" && (
                <>
                  {detail.operations.map((candidate) => (
                    <Inputs
                      key={
                        candidate.operation.operationId + ":" + detail.revision
                      }
                      candidate={candidate}
                      disabled={disabled || busy || uncertain}
                      save={async (id, choices) => {
                        await run(async () => {
                          const result = await api.saveOpenAPIChoices(
                            projectId,
                            detail,
                            id,
                            choices,
                          );
                          if (mounted.current) setDetail(result);
                          onGateChanged();
                        });
                      }}
                    />
                  ))}
                  {detail.nextOperationOffset != null && (
                    <button
                      disabled={disabled || busy}
                      onClick={() =>
                        void run(async () => {
                          const next = await api.openAPIImport(
                            projectId,
                            detail.importId,
                            detail.nextOperationOffset!,
                          );
                          if (!mounted.current) return;
                          if (next.revision !== detail.revision) {
                            setDetail(next);
                            setMessage(
                              "The import changed. Review its current inputs before publishing.",
                            );
                            return;
                          }
                          setDetail({
                            ...next,
                            operations: [
                              ...detail.operations,
                              ...next.operations,
                            ],
                          });
                        })
                      }
                    >
                      More operation previews
                    </button>
                  )}
                  <button
                    disabled={
                      disabled ||
                      busy ||
                      uncertain ||
                      !detail.operations.length ||
                      (detail.readyOperationCount !== undefined
                        ? detail.readyOperationCount !== detail.operationCount
                        : detail.operations.length !== detail.operationCount ||
                          !detail.operations.every((c) => c.ready))
                    }
                    onClick={() =>
                      void run(async () => {
                        const result = await api.publishOpenAPIImport(
                          projectId,
                          detail,
                        );
                        if (mounted.current) {
                          setDetail(result);
                          setMessage(
                            "Requests published. Refresh saved requests to open and execute them.",
                          );
                        }
                        onGateChanged();
                      })
                    }
                  >
                    Publish generated requests
                  </button>
                </>
              )}
              {detail.state !== "ready" && (
                <button
                  disabled={disabled || busy || uncertain}
                  onClick={() =>
                    void run(async () => {
                      await api.deleteOpenAPIImport(projectId, detail);
                      if (mounted.current) {
                        setItems((old) =>
                          old.filter((x) => x.importId !== detail.importId),
                        );
                        setDetail(undefined);
                        setMessage("Import canceled.");
                      }
                      onGateChanged();
                    })
                  }
                >
                  Cancel import
                </button>
              )}
            </div>
          )}
        </>
      )}
      {message && <p role="status">{message}</p>}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
