const path = "/api/v1/browser-session";
const logoutKey = "kurier.browser-session.logout-pending";
export const sessionChangedEvent = "kurier:browser-session-changed";

export class SessionExpired extends Error {
  constructor() {
    super("Your session ended. Please sign in again.");
  }
}
export class SessionUnavailable extends Error {
  constructor(
    message = "Your session could not be reached. Check your connection and try again.",
    readonly retryable = true,
  ) {
    super(message);
  }
}

type Metadata = {
  sessionBinding: string;
  csrfToken: string;
  sessionExpiresAt: string;
};
type Tokens = {
  accessToken: string;
  accessExpiresAt: string;
  sessionExpiresAt: string;
  sessionBinding: string;
};
type Attempt = {
  email: string;
  accessToken: string;
  refreshToken: string;
  attemptId: string;
  cancellationProof: string;
  sessionProof: string;
  createdAt: number;
};

function proof() {
  return btoa(
    String.fromCharCode(...crypto.getRandomValues(new Uint8Array(32))),
  )
    .replaceAll("+", "-")
    .replaceAll("/", "_")
    .replaceAll("=", "");
}
function record(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new SessionUnavailable();
  return value as Record<string, unknown>;
}
function text(value: unknown, max = 256): string {
  if (typeof value !== "string" || !value || value.length > max)
    throw new SessionUnavailable();
  return value;
}
function deadline(value: unknown): string {
  const result = text(value, 40);
  if (!Number.isFinite(Date.parse(result))) throw new SessionUnavailable();
  return result;
}
function metadata(value: unknown): Metadata {
  const data = record(value);
  return {
    sessionBinding: text(data.sessionBinding),
    csrfToken: text(data.csrfToken),
    sessionExpiresAt: deadline(data.sessionExpiresAt),
  };
}
function tokens(value: unknown): Tokens {
  const data = record(value);
  return {
    accessToken: text(data.accessToken, 16384),
    accessExpiresAt: deadline(data.accessExpiresAt),
    sessionExpiresAt: deadline(data.sessionExpiresAt),
    sessionBinding: text(data.sessionBinding),
  };
}

// Every credential in this object is transient. Only the server can read the
// persistent HttpOnly cookie; no credential is written to browser storage.
export class SessionBroker {
  private meta?: Metadata;
  private access?: Tokens;
  private attempt?: Attempt;
  private generation = 0;
  private refreshing?: Promise<string>;
  private pendingLogout = false;
  private retryAfter = 0;
  private channel?: BroadcastChannel;

  constructor() {
    if (typeof BroadcastChannel !== "undefined") {
      this.channel = new BroadcastChannel("kurier-browser-session");
      this.channel.onmessage = (event: MessageEvent<unknown>) => {
        if (event.data !== "signed-out" && event.data !== "replaced") return;
        this.suspend();
        if (event.data === "signed-out") this.pendingLogout = true;
        window.dispatchEvent(new Event(sessionChangedEvent));
      };
    }
  }

  private logoutIntent() {
    if (this.pendingLogout) return true;
    try {
      return localStorage.getItem(logoutKey) === "1";
    } catch {
      return false;
    }
  }
  private markLogout(pending: boolean) {
    this.pendingLogout = pending;
    try {
      if (pending) localStorage.setItem(logoutKey, "1");
      else localStorage.removeItem(logoutKey);
    } catch {
      /* The in-process fence still blocks restoration in this document. */
    }
  }
  suspend() {
    this.generation++;
    this.meta = undefined;
    this.access = undefined;
    this.refreshing = undefined;
  }
  private async request(
    method: string,
    suffix = "",
    body?: unknown,
    headers: Record<string, string> = {},
    busyRetries = 0,
    requestDeadline = Date.now() + 10000,
  ): Promise<unknown> {
    if (Date.now() < this.retryAfter)
      throw new SessionUnavailable(
        "Too many session attempts. Wait before trying again.",
        false,
      );
    const controller = new AbortController();
    if (Date.now() >= requestDeadline) throw new SessionUnavailable();
    const timer = setTimeout(
      () => controller.abort(),
      requestDeadline - Date.now(),
    );
    try {
      const response = await fetch(path + suffix, {
        method,
        credentials: "same-origin",
        cache: "no-store",
        redirect: "error",
        signal: controller.signal,
        headers: {
          "X-Kurier-Session": "1",
          ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
          ...headers,
        },
        ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
      });
      if (response.status === 401) throw new SessionExpired();
      if (!response.ok) {
        if (response.status === 409) {
          const failure = record(await response.json());
          const code = record(failure.error).code;
          if (code === "session_changed") {
            this.suspend();
            window.dispatchEvent(new Event(sessionChangedEvent));
            throw new SessionUnavailable(
              "Your browser session changed. Restoring its current sign-in.",
              false,
            );
          }
          if (code === "session_busy" && busyRetries < 4) {
            const delay = response.headers.get("Retry-After");
            if (delay !== "1") throw new SessionUnavailable(undefined, false);
            await new Promise((resolve) => setTimeout(resolve, 1000));
            return this.request(
              method,
              suffix,
              body,
              headers,
              busyRetries + 1,
              requestDeadline,
            );
          }
          throw new SessionUnavailable(undefined, false);
        }
        if (response.status === 429) {
          const retry = response.headers.get("Retry-After");
          const seconds = retry && /^\d+$/.test(retry) ? Number(retry) : 60;
          this.retryAfter =
            Date.now() + Math.min(Math.max(seconds, 1), 86400) * 1000;
        }
        throw new SessionUnavailable(
          response.status === 429
            ? "Too many session attempts. Wait before trying again."
            : response.status === 403
              ? "This browser could not authorize the session request. Reload and try again."
              : undefined,
          response.status >= 500,
        );
      }
      return record(await response.json()).data;
    } catch (error) {
      if (
        error instanceof SessionExpired ||
        error instanceof SessionUnavailable
      )
        throw error;
      throw new SessionUnavailable();
    } finally {
      clearTimeout(timer);
    }
  }
  private headers(meta: Metadata) {
    return {
      "X-Kurier-CSRF": meta.csrfToken,
      "X-Kurier-Session-Binding": meta.sessionBinding,
    };
  }
  private async readMetadata() {
    return metadata(await this.request("GET"));
  }

  async restore(): Promise<boolean> {
    if (this.logoutIntent()) {
      await this.logout();
      return false;
    }
    const current = ++this.generation;
    this.meta = undefined;
    this.access = undefined;
    this.refreshing = undefined;
    try {
      const meta = await this.readMetadata();
      if (current !== this.generation) throw new SessionExpired();
      if (Date.parse(meta.sessionExpiresAt) <= Date.now())
        throw new SessionExpired();
      this.meta = meta;
      await this.token();
      return true;
    } catch (error) {
      if (current === this.generation) this.suspend();
      if (error instanceof SessionExpired) return false;
      throw error;
    }
  }

  hasAttempt(email: string) {
    return this.attempt?.email === email;
  }
  async confirmCookie() {
    const current = this.generation;
    const expected = this.meta?.sessionBinding;
    try {
      const retained = await this.readMetadata();
      if (
        current !== this.generation ||
        !expected ||
        retained.sessionBinding !== expected
      )
        throw new SessionUnavailable(
          "This browser's sign-in changed. Reload before continuing.",
          false,
        );
    } catch (error) {
      if (error instanceof SessionExpired)
        throw new SessionUnavailable(
          "This browser did not retain the sign-in cookie. Allow first-party cookies to keep your session.",
          false,
        );
      throw error;
    }
  }
  async prepareLogin(email: string) {
    if (this.logoutIntent()) await this.logout();
    if (this.attempt && this.attempt.email !== email)
      await this.cancelPending();
  }
  async bootstrap(email: string, accessToken?: string, refreshToken?: string) {
    if (this.logoutIntent()) await this.logout();
    if (this.attempt && this.attempt.email !== email)
      await this.cancelPending();
    if (!this.attempt) {
      if (!accessToken || !refreshToken) throw new SessionExpired();
      this.attempt = {
        email,
        accessToken,
        refreshToken,
        attemptId: proof(),
        cancellationProof: proof(),
        sessionProof: proof(),
        createdAt: Date.now(),
      };
    }
    const attempt = this.attempt;
    if (Date.now() - attempt.createdAt >= 5 * 60 * 1000) {
      await this.cancelPending();
      throw new SessionExpired();
    }
    const current = ++this.generation;
    const send = () =>
      this.request(
        "POST",
        "",
        {
          refreshToken: attempt.refreshToken,
          attemptId: attempt.attemptId,
          cancellationProof: attempt.cancellationProof,
          sessionProof: attempt.sessionProof,
        },
        { Authorization: `Bearer ${attempt.accessToken}` },
      );
    let result: unknown;
    try {
      result = await send();
    } catch (error) {
      // Exactly the same authenticated attempt reissues its original cookie.
      // Never retry a definitively rejected or canceled handoff.
      if (
        error instanceof SessionExpired ||
        (error instanceof SessionUnavailable && !error.retryable) ||
        current !== this.generation
      )
        throw error;
      result = await send();
    }
    if (current !== this.generation || this.attempt !== attempt)
      throw new SessionExpired();
    const meta = metadata(result);
    const access = tokens(result);
    if (
      Date.parse(meta.sessionExpiresAt) <= Date.now() ||
      Date.parse(access.accessExpiresAt) <= Date.now()
    )
      throw new SessionExpired();
    this.meta = meta;
    this.access = access;
    this.attempt = undefined;
    this.channel?.postMessage("replaced");
  }

  async token(force = false): Promise<string> {
    if (this.logoutIntent() || !this.meta) throw new SessionExpired();
    if (Date.parse(this.meta.sessionExpiresAt) <= Date.now())
      throw new SessionExpired();
    if (
      !force &&
      this.access &&
      Date.parse(this.access.accessExpiresAt) > Date.now() + 30000
    )
      return this.access.accessToken;
    if (this.refreshing) return this.refreshing;
    const current = this.generation;
    const meta = this.meta;
    const task = (async () => {
      const access = tokens(
        await this.request("POST", "/token", {}, this.headers(meta)),
      );
      if (
        current !== this.generation ||
        access.sessionBinding !== meta.sessionBinding ||
        Date.parse(access.accessExpiresAt) <= Date.now() ||
        Date.parse(access.sessionExpiresAt) <= Date.now()
      )
        throw new SessionExpired();
      this.access = access;
      return access.accessToken;
    })();
    this.refreshing = task;
    try {
      return await task;
    } finally {
      if (this.refreshing === task) this.refreshing = undefined;
    }
  }

  private async cancelPending() {
    const attempt = this.attempt;
    if (!attempt) return;
    await this.request("POST", "/bootstrap-cancellations", {
      attemptId: attempt.attemptId,
      cancellationProof: attempt.cancellationProof,
    });
    if (this.attempt === attempt) this.attempt = undefined;
  }
  async logout() {
    const known = this.meta;
    this.suspend();
    this.markLogout(true);
    this.channel?.postMessage("signed-out");
    try {
      await this.cancelPending();
      let meta = known;
      if (!meta) {
        try {
          meta = await this.readMetadata();
        } catch (error) {
          if (!(error instanceof SessionExpired)) throw error;
        }
      }
      const data = record(
        await this.request(
          "DELETE",
          "",
          undefined,
          meta ? this.headers(meta) : {},
        ),
      );
      if (data.signedOut !== true) throw new SessionUnavailable();
      this.markLogout(false);
      return data.providerRevocation === "pending"
        ? "Server session ended; provider revocation is pending."
        : undefined;
    } catch {
      throw new SessionUnavailable(
        "Signed out here; server sign-out awaits a connection. Retry before signing in again. If browser storage is blocked, reopening may restore the session; clear site data to remove it.",
      );
    }
  }
}
