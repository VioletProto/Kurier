import { Requests } from "./Requests";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { auth, authConfigured, authMessage, SessionExpired } from "./auth";
import {
  api,
  ApiError,
  UncertainWrite,
  type Deletion,
  type Project,
  type ProjectDetail,
  type User,
} from "./api";

type Mode = "signin" | "signup" | "verify" | "reset" | "confirmReset";
const titles: Record<Mode, string> = {
  signin: "Sign in",
  signup: "Create account",
  verify: "Verify email",
  reset: "Reset password",
  confirmReset: "Choose a new password",
};

export function App() {
  const [signedIn, setSignedIn] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [notice, setNotice] = useState("");
  const [dark, setDark] = useState(
    () => window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false,
  );
  useEffect(() => {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
  }, [dark]);
  async function logout(expired = false) {
    setSignedIn(false);
    setLeaving(true);
    setNotice(
      expired ? "Your session ended. Please sign in again." : "Signed out.",
    );
    try {
      await auth.logout();
    } catch {
      setNotice(
        "Signed out locally. Cognito could not confirm token revocation.",
      );
    } finally {
      setLeaving(false);
    }
  }
  return (
    <main>
      <a className="skip" href="#workspace">
        Skip to workspace
      </a>
      <header>
        <div>
          <p className="eyebrow">Agent-ready API evidence</p>
          <h1>Kurier</h1>
        </div>
        <button
          type="button"
          aria-pressed={dark}
          onClick={() => setDark(!dark)}
        >
          Dark theme
        </button>
      </header>
      <p className="stage">
        Environment: {import.meta.env.VITE_KURIER_STAGE ?? "local"}
      </p>
      <div role="status">{notice}</div>
      <section id="workspace" aria-label="Workspace">
        {signedIn ? (
          <>
            <button className="signout" onClick={() => void logout()}>
              Sign out
            </button>
            <Projects onExpired={() => void logout(true)} />
          </>
        ) : leaving ? (
          <p role="status">Signing out…</p>
        ) : (
          <AuthForm
            onSignedIn={() => {
              setNotice("");
              setSignedIn(true);
            }}
          />
        )}
      </section>
      <footer>
        Credentials stay with Cognito. This browser session ends when you
        reload.
      </footer>
    </main>
  );
}

function AuthForm({ onSignedIn }: { onSignedIn: () => void }) {
  const [mode, setMode] = useState<Mode>("signin");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [cooldown, setCooldown] = useState(0);
  useEffect(() => {
    if (!cooldown) return;
    const timer = setTimeout(() => setCooldown(cooldown - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);
  function change(next: Mode) {
    setMode(next);
    setError("");
    setMessage("");
    setPassword("");
    setCode("");
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    setMessage("");
    try {
      if (mode === "signin") {
        const result = await auth.login(email.trim(), password);
        if (result === "done") onSignedIn();
        else if (result === "verify") {
          setMode("verify");
          setMessage("Verify your email before signing in.");
        } else
          setError(
            "This account requires an authentication step unavailable here.",
          );
      } else if (mode === "signup") {
        try {
          await auth.signup(email.trim(), password);
        } catch (e) {
          if (!(e instanceof Error) || e.name !== "UsernameExistsException")
            throw e;
        }
        setMode("verify");
        setCooldown(60);
        setMessage(
          "If this address can be registered, a verification code has been sent. Existing users can sign in or reset their password.",
        );
      } else if (mode === "verify") {
        await auth.verify(email.trim(), code.trim());
        change("signin");
        setMessage("Email verified. Sign in to continue.");
      } else if (mode === "reset") {
        await auth.reset(email.trim());
        setMode("confirmReset");
        setCooldown(60);
        setMessage("If the account is eligible, a reset code has been sent.");
      } else {
        await auth.confirmReset(email.trim(), code.trim(), password);
        change("signin");
        setMessage("Password updated. Sign in with your new password.");
      }
    } catch (e) {
      setError(authMessage(e));
    } finally {
      setPassword("");
      setBusy(false);
    }
  }
  async function resend() {
    setBusy(true);
    setError("");
    try {
      if (mode === "verify") await auth.resend(email.trim());
      else await auth.reset(email.trim());
      setMessage("If eligible, a new code has been sent.");
      setCooldown(60);
    } catch (e) {
      setError(authMessage(e));
      setCooldown(60);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth panel">
      <h2>{titles[mode]}</h2>
      {!authConfigured && (
        <p role="alert">
          Configure the development Cognito pool and client before signing in.
          See the local setup guide.
        </p>
      )}
      <form onSubmit={submit} aria-busy={busy}>
        <fieldset disabled={busy || !authConfigured}>
          <label htmlFor="email">Email</label>
          <input
            id="email"
            name="email"
            type="email"
            autoComplete="username"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          {(mode === "verify" || mode === "confirmReset") && (
            <>
              <label htmlFor="code">Verification code</label>
              <input
                id="code"
                name="code"
                inputMode="numeric"
                autoComplete="one-time-code"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                required
              />
            </>
          )}
          {(mode === "signin" ||
            mode === "signup" ||
            mode === "confirmReset") && (
            <>
              <label htmlFor="password">
                {mode === "confirmReset" ? "New password" : "Password"}
              </label>
              <input
                id="password"
                name="password"
                type="password"
                autoComplete={
                  mode === "signin" ? "current-password" : "new-password"
                }
                minLength={mode === "signin" ? undefined : 12}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
              {mode !== "signin" && (
                <p className="hint">
                  At least 12 characters, with uppercase, lowercase, a number
                  and a symbol.
                </p>
              )}
            </>
          )}
          <button className="primary" type="submit">
            {busy ? "Please wait…" : titles[mode]}
          </button>
        </fieldset>
      </form>
      <p role="alert">{error}</p>
      <p role="status">{message}</p>
      <nav aria-label="Account options">
        {(["signin", "signup", "verify", "reset"] as Mode[])
          .filter((x) => x !== mode)
          .map((x) => (
            <button key={x} disabled={busy} onClick={() => change(x)}>
              {titles[x]}
            </button>
          ))}
      </nav>
      {(mode === "verify" || mode === "confirmReset") && (
        <button
          disabled={busy || cooldown > 0 || !authConfigured || !email.trim()}
          onClick={() => void resend()}
        >
          {cooldown ? `Resend code in ${cooldown}s` : "Resend code"}
        </button>
      )}
    </div>
  );
}

function validName(value: string) {
  const name = value.trim();
  return name.length > 0 && [...name].length <= 100 && !/\p{Cc}/u.test(value);
}
function Projects({ onExpired }: { onExpired: () => void }) {
  const mounted = useRef(true);
  const [user, setUser] = useState<User>();
  const [items, setItems] = useState<Project[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [detail, setDetail] = useState<ProjectDetail>();
  const [name, setName] = useState("");
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const [stale, setStale] = useState(false);
  const [uncertainCreate, setUncertainCreate] = useState(false);
  const [uncertainDeletion, setUncertainDeletion] = useState<ProjectDetail>();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [operations, setOperations] = useState<Deletion[]>([]);
  function failure(e: unknown) {
    if (!mounted.current) return;
    if (e instanceof SessionExpired) {
      onExpired();
      return;
    }
    setError(e instanceof Error ? e.message : "Unable to complete this step.");
    if (e instanceof ApiError && e.status === 404) {
      setDetail(undefined);
      setConfirmDelete(false);
    }
  }
  useEffect(() => {
    mounted.current = true;
    const controller = new AbortController();
    Promise.all([
      api.me(controller.signal),
      api.list(undefined, controller.signal),
    ])
      .then(([u, p]) => {
        if (!controller.signal.aborted) {
          setUser(u);
          setItems(p.items);
          setCursor(p.nextCursor);
        }
      })
      .catch((e) => {
        if (!controller.signal.aborted) failure(e);
      })
      .finally(() => {
        if (!controller.signal.aborted) setBusy(false);
      });
    const timer = setInterval(() => {
      void auth.token().catch((e) => {
        if (mounted.current && e instanceof SessionExpired) onExpired();
      });
    }, 60000);
    return () => {
      mounted.current = false;
      controller.abort();
      clearInterval(timer);
    };
    // The parent callback only changes when this workspace is removed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  async function run(task: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      await task();
    } catch (e) {
      failure(e);
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  async function refreshList() {
    const [u, p] = await Promise.all([api.me(), api.list()]);
    setUser(u);
    setItems(p.items);
    setCursor(p.nextCursor);
    setUncertainCreate(false);
  }
  async function open(id: string) {
    const d = await api.detail(id);
    setDetail(d);
    setDraft(d.project.name);
    setStale(false);
    setConfirmDelete(false);
  }
  async function create(event: FormEvent) {
    event.preventDefault();
    if (!validName(name)) {
      setError(
        "Provide a name of 1–100 characters without control characters.",
      );
      return;
    }
    await run(async () => {
      try {
        const d = await api.create(name.trim());
        setDetail(d);
        setDraft(d.project.name);
        setName("");
        setStale(false);
        setConfirmDelete(false);
        setItems((old) => [
          d.project,
          ...old.filter((p) => p.projectId !== d.project.projectId),
        ]);
      } catch (e) {
        if (e instanceof UncertainWrite) setUncertainCreate(true);
        throw e;
      }
    });
  }
  async function rename(event: FormEvent) {
    event.preventDefault();
    if (!detail) return;
    if (!validName(draft)) {
      setError(
        "Provide a name of 1–100 characters without control characters.",
      );
      return;
    }
    await run(async () => {
      try {
        const d = await api.rename(
          detail.project.projectId,
          draft.trim(),
          detail.etag,
        );
        setDetail(d);
        setDraft(d.project.name);
        setItems((old) =>
          old.map((p) => (p.projectId === d.project.projectId ? d.project : p)),
        );
      } catch (e) {
        if (
          e instanceof UncertainWrite ||
          (e instanceof ApiError && [409, 412].includes(e.status))
        )
          setStale(true);
        throw e;
      }
    });
  }
  return (
    <div aria-busy={busy}>
      <h2>Your projects</h2>
      {user && (
        <p className="hint">Account {user.displayName || user.userId}</p>
      )}
      <p role="status">{busy ? "Loading…" : ""}</p>
      <p role="alert">{error}</p>
      <div className="columns">
        <section className="panel" aria-label="Project list">
          <button disabled={busy} onClick={() => void run(refreshList)}>
            Refresh projects
          </button>
          <form onSubmit={create}>
            <label htmlFor="project-name">New project name</label>
            <input
              id="project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={busy || uncertainCreate}
              required
            />
            <button
              className="primary"
              disabled={busy || uncertainCreate}
              type="submit"
            >
              Create project
            </button>
          </form>
          {uncertainCreate && (
            <p role="status">
              Creation may have succeeded. Refresh and check the list before
              creating again. Recent projects can take time to appear.
            </p>
          )}
          {!busy && items.length === 0 && <p>No projects on this page.</p>}
          <ul className="projects">
            {items.map((p) => (
              <li key={p.projectId}>
                <button
                  disabled={busy}
                  aria-current={
                    detail?.project.projectId === p.projectId
                      ? "true"
                      : undefined
                  }
                  onClick={() => void run(() => open(p.projectId))}
                >
                  {p.name}
                </button>
              </li>
            ))}
          </ul>
          {cursor !== null && (
            <button
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  const p = await api.list(cursor);
                  setItems((old) => [
                    ...old,
                    ...p.items.filter(
                      (x) => !old.some((y) => y.projectId === x.projectId),
                    ),
                  ]);
                  setCursor(p.nextCursor);
                })
              }
            >
              Load more projects
            </button>
          )}
        </section>
        <section className="panel" aria-label="Project details">
          {detail ? (
            <>
              <h3>{detail.project.name}</h3>
              <Requests
                key={detail.project.projectId}
                projectId={detail.project.projectId}
                disabled={busy}
                onExpired={onExpired}
                onGateChanged={() => setStale(true)}
              />
              <dl>
                <dt>Project ID</dt>
                <dd>{detail.project.projectId}</dd>
                <dt>Version</dt>
                <dd>{detail.project.version}</dd>
                <dt>Created</dt>
                <dd>{new Date(detail.project.createdAt).toLocaleString()}</dd>
                <dt>Updated</dt>
                <dd>{new Date(detail.project.updatedAt).toLocaleString()}</dd>
              </dl>
              <button
                disabled={busy}
                onClick={() => void run(() => open(detail.project.projectId))}
              >
                Refresh details
              </button>
              {stale && (
                <p role="status">
                  Refresh details to review the current version, then submit
                  your change again.
                </p>
              )}
              <form onSubmit={rename}>
                <label htmlFor="rename">Project name</label>
                <input
                  id="rename"
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  disabled={busy || stale}
                  required
                />
                <button disabled={busy || stale} type="submit">
                  Rename project
                </button>
              </form>
              {confirmDelete ? (
                <div>
                  <p>
                    Delete “{detail.project.name}”? Access is removed
                    immediately.
                  </p>
                  <button
                    className="danger"
                    disabled={busy || stale}
                    onClick={() =>
                      void run(async () => {
                        try {
                          const op = await api.delete(
                            detail.project.projectId,
                            detail.etag,
                          );
                          setOperations((old) => [...old, op]);
                          setItems((old) =>
                            old.filter(
                              (p) => p.projectId !== detail.project.projectId,
                            ),
                          );
                          setDetail(undefined);
                          setConfirmDelete(false);
                        } catch (e) {
                          if (e instanceof UncertainWrite)
                            setUncertainDeletion(detail);
                          if (
                            e instanceof UncertainWrite ||
                            (e instanceof ApiError &&
                              [409, 412].includes(e.status))
                          )
                            setStale(true);
                          throw e;
                        }
                      })
                    }
                  >
                    Confirm deletion
                  </button>
                  <button
                    disabled={busy}
                    onClick={() => setConfirmDelete(false)}
                  >
                    Cancel
                  </button>
                </div>
              ) : (
                <button
                  className="danger"
                  disabled={busy || stale}
                  onClick={() => setConfirmDelete(true)}
                >
                  Delete project
                </button>
              )}
            </>
          ) : (
            <p>Select a project to inspect its details.</p>
          )}
        </section>
      </div>
      {uncertainDeletion && (
        <section className="panel" aria-label="Uncertain deletion">
          <p>
            The deletion response was lost. Check its outcome using the original
            version.
          </p>
          <button
            disabled={busy}
            onClick={() =>
              void run(async () => {
                try {
                  // Deliberate initiating-version repeat returns the same operation.
                  const op = await api.delete(
                    uncertainDeletion.project.projectId,
                    uncertainDeletion.etag,
                  );
                  setOperations((old) => [
                    ...old.filter((x) => x.operationId !== op.operationId),
                    op,
                  ]);
                  setItems((old) =>
                    old.filter((p) => p.projectId !== op.projectId),
                  );
                  setDetail(undefined);
                  setUncertainDeletion(undefined);
                  setConfirmDelete(false);
                } catch (e) {
                  if (e instanceof ApiError && [404, 412].includes(e.status))
                    setUncertainDeletion(undefined);
                  throw e;
                }
              })
            }
          >
            Check deletion outcome
          </button>
        </section>
      )}
      {operations.map((op) => (
        <DeletionStatus
          key={op.operationId}
          initial={op}
          onExpired={onExpired}
        />
      ))}
    </div>
  );
}

function DeletionStatus({
  initial,
  onExpired,
}: {
  initial: Deletion;
  onExpired: () => void;
}) {
  const [op, setOp] = useState(initial);
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let delay = 2000;
    async function poll() {
      try {
        const next = await api.operation(initial, controller.signal);
        if (controller.signal.aborted) return;
        setOp(next);
        setError("");
        if (next.state === "completed") return;
        delay = Math.min(
          10000,
          Math.max(delay * 1.5, next.retryAfterSeconds * 1000, 2000),
        );
      } catch (e) {
        if (controller.signal.aborted) return;
        if (e instanceof SessionExpired) {
          onExpired();
          return;
        }
        if (e instanceof ApiError && e.status === 404) {
          setError("Deletion status is no longer available.");
          return;
        }
        if (e instanceof ApiError && e.status === 403) {
          setError(
            "Your account cannot access deletion status. Polling stopped.",
          );
          return;
        }
        setError("Deletion status is temporarily unavailable. Checking again…");
        delay = Math.min(10000, delay * 2);
      }
      timer = setTimeout(() => void poll(), delay);
    }
    if (initial.state !== "completed")
      timer = setTimeout(() => void poll(), delay);
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [initial, onExpired]);
  return (
    <section className="panel" aria-label="Deletion status">
      <p role="status">
        Deletion {op.operationId}:{" "}
        {op.state === "completed"
          ? "completed"
          : `${op.state} — cleanup pending`}
      </p>
      <p role="alert">{error}</p>
    </section>
  );
}
