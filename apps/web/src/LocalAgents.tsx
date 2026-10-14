import { useEffect, useRef, useState } from "react";
import { api, ApiError, UncertainWrite, type LocalAgent } from "./api";
import { SessionExpired } from "./auth";

export function LocalAgents({
  projectId,
  disabled,
  onAgent,
  onSessionExpired,
}: {
  projectId: string;
  disabled: boolean;
  onAgent: (agent: LocalAgent | null) => void;
  onSessionExpired: () => void;
}) {
  const [agent, setAgent] = useState<LocalAgent | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [name, setName] = useState("My local agent");
  const [origins, setOrigins] = useState("http://localhost:3000");
  const [addressClass, setAddressClass] = useState<"loopback" | "private">(
    "loopback",
  );
  const [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [unresolved, setUnresolved] = useState(false);
  const active = useRef<AbortController | null>(null);
  const flight = useRef(false);
  const update = (value: LocalAgent | null) => {
    setAgent(value);
    onAgent(value);
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
        if (e instanceof SessionExpired) onSessionExpired();
        else {
          setError(
            e instanceof UncertainWrite
              ? "Pairing outcome is unresolved. Refresh status, then explicitly revoke and re-pair if the token was lost."
              : e instanceof ApiError
                ? `Pairing unavailable (${e.code}).`
                : "Pairing API unavailable. Refresh status.",
          );
          if (e instanceof UncertainWrite) setUnresolved(true);
        }
      }
    } finally {
      if (!signal.aborted) {
        flight.current = false;
        setBusy(false);
      }
    }
  };
  useEffect(() => {
    active.current = new AbortController();
    flight.current = false;
    setAgent(null);
    setToken(null);
    setConsent(false);
    setUnresolved(false);
    setError("");
    return () => {
      active.current?.abort();
      active.current = null;
    };
  }, [projectId, disabled]);
  const serverNow = agent?.serverTime
    ? Date.parse(agent.serverTime)
    : Date.now();
  const activeCredential =
    !!agent && !agent.revokedAt && Date.parse(agent.expiresAt) > serverNow;
  useEffect(() => {
    if (!agent || disabled || token || !activeCredential) return;
    const timer = setInterval(() => {
      void run(async (signal) => {
        const current = await api.currentAgent(signal);
        if (!signal.aborted) update(current);
      });
    }, 10000);
    return () => clearInterval(timer);
    // Component lifetime fences responses; the refresh callback reads current props.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agent?.localAgentId, disabled, token, activeCredential]);
  const presence = agent?.revokedAt
    ? "Revoked"
    : agent && Date.parse(agent.expiresAt) <= serverNow
      ? "Expired"
      : agent?.lastSeenAt &&
          serverNow - Date.parse(agent.lastSeenAt) >= 0 &&
          serverNow - Date.parse(agent.lastSeenAt) <= 60000
        ? "Seen within the last minute"
        : "Offline / unknown";
  return (
    <section className="panel" aria-label="Local agent pairing">
      <h4>Local agent pairing</h4>
      <p>
        One active agent per account, valid for 30 days. This device receives
        runtime secrets and may affect every path on the exact services you
        approve. Revocation denies further work and uploads; already delivered
        secrets or HTTP cannot be recalled. Signing out does not revoke the
        agent.
      </p>
      <button
        disabled={busy || disabled}
        onClick={() =>
          void run(async (signal) => {
            const current = await api.currentAgent(signal);
            if (!signal.aborted) {
              update(current);
              setUnresolved(false);
              setToken(null);
            }
          })
        }
      >
        Refresh pairing status
      </button>
      {agent && (
        <div>
          <p>
            Agent: {agent.name}. Expires: {agent.expiresAt}. {presence}. Last
            seen: {agent.lastSeenAt ?? "Offline / not yet seen"}.
          </p>
          {agent.scopes.map((scope) => (
            <p key={scope.projectId}>
              Project {scope.projectId}:{" "}
              {scope.destinations
                .map((d) => `${d.origin} (${d.addressClass})`)
                .join(", ")}
            </p>
          ))}
          <button
            disabled={busy || disabled || !!agent.revokedAt}
            onClick={() =>
              void run(async (signal) => {
                await api.revokeAgent(agent.localAgentId, signal);
                if (!signal.aborted) {
                  setToken(null);
                  const current = await api.currentAgent(signal);
                  if (!signal.aborted) update(current);
                }
              })
            }
          >
            Revoke local agent
          </button>
        </div>
      )}
      <fieldset disabled={busy || disabled || unresolved || activeCredential}>
        <legend>Pair for this project</legend>
        <label>
          Agent name
          <input
            value={name}
            maxLength={100}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label>
          Address permission
          <select
            value={addressClass}
            onChange={(e) => {
              setAddressClass(e.target.value as "loopback" | "private");
              setConsent(false);
            }}
          >
            <option value="loopback">Loopback only</option>
            <option value="private">Private network (RFC1918 / ULA)</option>
          </select>
        </label>
        <label>
          Exact HTTP(S) origins, one per line (up to 16)
          <textarea
            value={origins}
            onChange={(e) => {
              setOrigins(e.target.value);
              setConsent(false);
            }}
          />
        </label>
        <p>
          No wildcards, port ranges, metadata addresses or implicit access to
          the whole network. This scope is immutable; revoke and re-pair to
          change it. The browser's localhost may differ from the agent's
          machine.
        </p>
        <label>
          <input
            type="checkbox"
            checked={consent}
            onChange={(e) => setConsent(e.target.checked)}
          />
          I authorize this project and these exact destinations for the named
          device.
        </label>
        <button
          disabled={!consent}
          onClick={() =>
            void run(async (signal) => {
              setToken(null);
              const response = await api.pairAgent(
                crypto.randomUUID(),
                {
                  name,
                  scopes: [
                    {
                      projectId,
                      destinations: origins
                        .split("\n")
                        .map((s) => s.trim())
                        .filter(Boolean)
                        .map((origin) => ({ origin, addressClass })),
                    },
                  ],
                },
                signal,
              );
              if (!signal.aborted) {
                update(response.agent);
                setToken(response.token);
                setConsent(false);
              }
            })
          }
        >
          Create agent credential
        </button>
      </fieldset>
      {token && (
        <div role="status">
          <p>
            This token is shown once. Enter it into the agent's hidden prompt.
            It is held only in this view.
          </p>
          <pre aria-label="One-time agent token">{token}</pre>
          <button onClick={() => setToken(null)}>Close token display</button>
        </div>
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
