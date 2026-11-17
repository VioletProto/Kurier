import { useEffect, useRef, useState } from "react";
import {
  api,
  ApiError,
  UncertainWrite,
  type MCPConnection,
  type MCPScope,
  type Project,
} from "./api";
import { SessionExpired } from "./auth";

export function MCPCredentials({
  projectId,
  projectName,
  disabled,
  onSessionExpired,
}: {
  projectId?: string;
  projectName?: string;
  disabled: boolean;
  onSessionExpired: () => void;
}) {
  const [connection, setConnection] = useState<MCPConnection | null>(null);
  const [mode, setMode] = useState<MCPScope["mode"]>("selected_projects");
  const [selected, setSelected] = useState<string[]>(
    projectId ? [projectId] : [],
  );
  const [projects, setProjects] = useState<Project[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [consent, setConsent] = useState(false);
  const [future, setFuture] = useState(false);
  const [checked, setChecked] = useState(false);
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const active = useRef<AbortController | null>(null);
  const flight = useRef(false);
  useEffect(() => {
    active.current = new AbortController();
    setConnection(null);
    setChecked(false);
    setToken(null);
    setError("");
    setConsent(false);
    setFuture(false);
    setMode("selected_projects");
    setSelected(projectId ? [projectId] : []);
    setProjects([]);
    setCursor(null);
    setBusy(false);
    flight.current = false;
    return () => {
      active.current?.abort();
      active.current = null;
    };
  }, [projectId, disabled]);
  const clearConsent = () => {
    setConsent(false);
    setFuture(false);
    setToken(null);
  };
  const apply = (v: MCPConnection | null) => {
    setConnection(v);
    setMode(v?.scope.mode ?? "selected_projects");
    setSelected(
      v?.scope.mode === "selected_projects"
        ? v.scope.projectIds
        : projectId
          ? [projectId]
          : [],
    );
    clearConsent();
  };
  const run = async (work: (signal: AbortSignal) => Promise<void>) => {
    if (disabled || flight.current || !active.current) return;
    const signal = active.current.signal;
    flight.current = true;
    setBusy(true);
    setError("");
    try {
      await work(signal);
    } catch (e) {
      if (!signal.aborted) {
        setChecked(false);
        setToken(null);
        clearConsent();
        if (e instanceof SessionExpired) onSessionExpired();
        else
          setError(
            e instanceof UncertainWrite
              ? "The credential operation could not be confirmed; issuance may have succeeded. Refresh status and compare project access before retrying. A lost token requires revoke and reissue."
              : e instanceof ApiError && e.status === 412
                ? "Project access changed in another session. Refresh status and review it before saving again."
                : "MCP access is unavailable. Deployment and activation may still be pending. Refresh status before a deliberate retry.",
          );
      }
    } finally {
      if (!signal.aborted) {
        flight.current = false;
        setBusy(false);
      }
    }
  };
  const live =
    connection &&
    !connection.revokedAt &&
    Date.parse(connection.expiresAt) > Date.now();
  const scope: MCPScope =
    mode === "all_owned_projects"
      ? { mode }
      : { mode, projectIds: [...selected].sort() };
  const summary = (s: MCPScope) =>
    s.mode === "all_owned_projects"
      ? "All current and future owned projects"
      : s.projectIds.length
        ? s.projectIds.join(", ")
        : "No projects selected";
  const canEdit = !live || connection.editable;
  const options = new Map(projects.map((p) => [p.projectId, p.name]));
  for (const id of selected)
    if (!options.has(id))
      options.set(id, id === projectId ? (projectName ?? id) : id);
  return (
    <section className="panel" aria-label="Read-only MCP access">
      <h3>MCP connection</h3>
      <p>
        One separate read credential per account, valid for 30 days. Your coding
        clients can inspect sanitized project and execution information. Signing
        out does not revoke access. Information already delivered cannot be
        recalled.
      </p>
      <p>
        This access cannot execute requests, change workflows or pins, or
        control the local agent. All clients sharing this credential use the
        same project access.
      </p>
      <button
        disabled={busy || disabled}
        onClick={() =>
          void run(async (signal) => {
            setToken(null);
            const v = await api.currentMCPConnection(signal);
            const page = await api.list(undefined, signal);
            if (!signal.aborted) {
              apply(v);
              setProjects(page.items);
              setCursor(page.nextCursor);
              setChecked(true);
            }
          })
        }
      >
        Refresh MCP status
      </button>
      {!checked && (
        <p>
          Refresh MCP status to check availability before creating or changing
          access.
        </p>
      )}
      {connection && (
        <div>
          <p>
            Credential {connection.tokenId}. Expires {connection.expiresAt}.{" "}
            {connection.revokedAt ? "Revoked" : live ? "Active" : "Expired"}.
          </p>
          <p>Current access: {summary(connection.scope)}</p>
          {live && !connection.editable && (
            <p>
              This credential has fixed legacy scope. To upgrade, revoke it,
              create a new credential and pair again using the same client
              registration.
            </p>
          )}
          <button
            disabled={busy || disabled || !!connection.revokedAt}
            onClick={() =>
              void run(async (signal) => {
                setToken(null);
                await api.revokeMCP(connection.tokenId, signal);
                const v = await api.currentMCPConnection(signal);
                if (!signal.aborted) apply(v);
              })
            }
          >
            Revoke MCP access
          </button>
        </div>
      )}
      <fieldset disabled={busy || disabled || !checked || !canEdit}>
        <legend>Project access</legend>
        <label>
          <input
            type="radio"
            name={"mcp-mode-" + (projectId ?? "account")}
            checked={mode === "selected_projects"}
            onChange={() => {
              setMode("selected_projects");
              clearConsent();
            }}
          />
          Choose projects
        </label>
        <p>
          Allow only the projects selected below (up to 10). Projects you create
          later stay private until you select them.
        </p>
        <label>
          <input
            type="radio"
            name={"mcp-mode-" + (projectId ?? "account")}
            checked={mode === "all_owned_projects"}
            onChange={() => {
              setMode("all_owned_projects");
              clearConsent();
            }}
          />
          All my projects, including future projects
        </label>
        <p>
          Allow read-only access to every active project you own now and every
          project you create while this credential remains valid. There are no
          project exclusions in this mode.
        </p>
        {mode === "selected_projects" && (
          <div>
            <p>{selected.length} of 10 projects selected</p>
            {[...options].map(([id, name]) => (
              <label key={id} style={{ display: "block" }}>
                <input
                  type="checkbox"
                  checked={selected.includes(id)}
                  disabled={!selected.includes(id) && selected.length >= 10}
                  onChange={(e) => {
                    setSelected((old) =>
                      e.target.checked
                        ? [...old, id]
                        : old.filter((p) => p !== id),
                    );
                    clearConsent();
                  }}
                />
                {name} ({id})
              </label>
            ))}
            {!options.size && (
              <p>
                Create a project to use manual selection, or choose automatic
                access.
              </p>
            )}
            {cursor && (
              <button
                onClick={() =>
                  void run(async (signal) => {
                    const page = await api.list(cursor, signal);
                    if (!signal.aborted) {
                      setProjects((old) => [
                        ...new Map(
                          [...old, ...page.items].map((p) => [p.projectId, p]),
                        ).values(),
                      ]);
                      setCursor(page.nextCursor);
                    }
                  })
                }
              >
                Load more projects
              </button>
            )}
          </div>
        )}
        {live && (
          <p>
            Before: {summary(connection.scope)}. After: {summary(scope)}. Saving
            changes every connected client and keeps the original expiry.
          </p>
        )}
        <label>
          <input
            type="checkbox"
            checked={consent}
            onChange={(e) => setConsent(e.target.checked)}
          />
          I authorize my coding clients to read sanitized project and execution
          information for this scope until{" "}
          {live ? connection.expiresAt : "30 days after issuance"}. Signing out
          does not revoke this access. Information already delivered cannot be
          recalled.
        </label>
        {mode === "all_owned_projects" && (
          <label>
            <input
              type="checkbox"
              checked={future}
              onChange={(e) => setFuture(e.target.checked)}
            />
            I understand that newly created projects will become available
            automatically.
          </label>
        )}
        <button
          disabled={
            !consent ||
            (mode === "all_owned_projects" && !future) ||
            (mode === "selected_projects" &&
              (selected.length < 1 || selected.length > 10))
          }
          onClick={() =>
            void run(async (signal) => {
              setToken(null);
              if (live) {
                const v = await api.updateMCPScope(
                  connection.tokenId,
                  connection.version,
                  scope,
                  signal,
                );
                if (!signal.aborted) apply(v);
              } else {
                const v = await api.issueMCPConnection(
                  crypto.randomUUID(),
                  scope,
                  signal,
                );
                if (!signal.aborted) {
                  apply(v.connection);
                  setToken(v.token);
                }
              }
            })
          }
        >
          {live ? "Save project access" : "Create MCP credential"}
        </button>
      </fieldset>
      {token && (
        <div role="status">
          <p>
            Shown once. Enter this token into kurier-mcp's hidden terminal
            pairing prompt. Closing this view clears it; a lost token requires
            revoke and reissue. The private credential file can be read by other
            processes running as you.
          </p>
          <pre aria-label="One-time MCP token">{token}</pre>
          <button onClick={() => setToken(null)}>
            Close MCP token display
          </button>
        </div>
      )}
      <details>
        <summary>Connect your coding client once</summary>
        <p>
          Use your installed kurier-mcp executable and the trusted API origin
          shown in your setup guide. Pair using its hidden terminal prompt:
        </p>
        <pre>
          kurier-mcp pair --api-origin &lt;trusted HTTPS API origin&gt;
          --persist
        </pre>
        <p>
          Inspect an existing registration with{" "}
          <code>codex mcp get kurier</code>. If none exists, register the
          executable once with your absolute private credential-file path:
        </p>
        <pre>
          codex mcp add kurier -- /absolute/path/kurier-mcp serve
          --credential-file /absolute/path/credential.json
        </pre>
        <p>
          Start a fresh client session and call list_projects; follow its cursor
          until null. Refresh discovery for new projects after index
          propagation. Scope changes need no restart. Expiry, revocation or
          token replacement requires reauthorization, pairing and a client
          restart using the same registration.
        </p>
      </details>
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
