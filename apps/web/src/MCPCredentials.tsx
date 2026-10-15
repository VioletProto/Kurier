import { useEffect, useRef, useState } from "react";
import { api, ApiError, UncertainWrite, type MCPCredential } from "./api";
import { SessionExpired } from "./auth";

export function MCPCredentials({
  projectId,
  projectName,
  disabled,
  onSessionExpired,
}: {
  projectId: string;
  projectName: string;
  disabled: boolean;
  onSessionExpired: () => void;
}) {
  const [credential, setCredential] = useState<MCPCredential | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [unresolved, setUnresolved] = useState(false);
  const [statusChecked, setStatusChecked] = useState(false);
  const active = useRef<AbortController | null>(null);
  const flight = useRef(false);
  useEffect(() => {
    active.current = new AbortController();
    setCredential(null);
    setToken(null);
    setConsent(false);
    setError("");
    setUnresolved(false);
    setStatusChecked(false);
    setBusy(false);
    flight.current = false;
    return () => {
      active.current?.abort();
      active.current = null;
    };
  }, [projectId, disabled]);
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
        if (e instanceof SessionExpired) onSessionExpired();
        else {
          setUnresolved(true);
          setStatusChecked(false);
          setError(
            e instanceof UncertainWrite
              ? "The credential operation could not be confirmed. If you were creating a credential, issuance may have succeeded. Refresh status, then revoke and reissue if the token was lost."
              : e instanceof ApiError
                ? e.status === 404 || e.status === 503
                  ? "MCP access is unavailable on this API. Deployment and activation may still be pending. Refresh status when access is available."
                  : `MCP credential unavailable (${e.code}). Refresh status.`
                : "Cannot check MCP access. Check your connection and API configuration, then refresh status.",
          );
        }
      }
    } finally {
      if (!signal.aborted) {
        flight.current = false;
        setBusy(false);
      }
    }
  };
  const live =
    credential &&
    !credential.revokedAt &&
    Date.parse(credential.expiresAt) > Date.now();
  return (
    <section className="panel" aria-label="Read-only MCP access">
      <h4>Read-only MCP access</h4>
      <p>
        One separate read credential per account, valid for 30 days. A coding
        client can inspect sanitized evidence from the projects you authorize.
        It cannot execute requests, change workflows or pins, or control the
        local agent. Signing out does not revoke access. Already delivered
        evidence cannot be recalled.
      </p>
      <button
        disabled={busy || disabled}
        onClick={() =>
          void run(async (signal) => {
            setToken(null);
            const current = await api.currentMCP(signal);
            if (!signal.aborted) {
              setCredential(current);
              setStatusChecked(true);
              setUnresolved(false);
              setError("");
            }
          })
        }
      >
        Refresh MCP status
      </button>
      {!statusChecked && (
        <p>
          Refresh MCP status to check availability before creating a credential.
        </p>
      )}
      {credential && (
        <div>
          <p>
            Credential {credential.tokenId}. Expires {credential.expiresAt}.{" "}
            {credential.revokedAt ? "Revoked" : live ? "Active" : "Expired"}.
          </p>
          {credential.projectIds.map((id) => (
            <p key={id}>Project {id}</p>
          ))}
          <button
            disabled={busy || disabled || !!credential.revokedAt}
            onClick={() =>
              void run(async (signal) => {
                await api.revokeMCP(credential.tokenId, signal);
                if (!signal.aborted) {
                  setToken(null);
                  const current = await api.currentMCP(signal);
                  if (!signal.aborted) setCredential(current);
                }
              })
            }
          >
            Revoke MCP access
          </button>
        </div>
      )}
      <fieldset
        disabled={busy || disabled || !!live || unresolved || !statusChecked}
      >
        <legend>Authorize this project</legend>
        <p>
          {projectName} ({projectId}). Scope is fixed. Revoke and issue again to
          change it. The local TUI stores this capability only with explicit
          consent in a private file; other processes running as you can read
          that file.
        </p>
        <label>
          <input
            type="checkbox"
            checked={consent}
            onChange={(e) => setConsent(e.target.checked)}
          />
          I authorize read-only MCP access to this project for my coding client.
        </label>
        <button
          disabled={!consent}
          onClick={() =>
            void run(async (signal) => {
              setToken(null);
              const response = await api.issueMCP(
                crypto.randomUUID(),
                [projectId],
                signal,
              );
              if (!signal.aborted) {
                setCredential(response.credential);
                setToken(response.token);
                setConsent(false);
              }
            })
          }
        >
          Create MCP credential
        </button>
      </fieldset>
      {token && (
        <div role="status">
          <p>
            Shown once. Enter this token into kurier-mcp's hidden terminal
            pairing prompt. Closing this view clears it; a lost token requires
            revoke and reissue.
          </p>
          <pre aria-label="One-time MCP token">{token}</pre>
          <button onClick={() => setToken(null)}>
            Close MCP token display
          </button>
        </div>
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
