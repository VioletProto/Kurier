import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  SessionBroker,
  SessionExpired,
  SessionUnavailable,
  sessionChangedEvent,
} from "./session-broker";

const fetcher = vi.fn();
const expiry = () => new Date(Date.now() + 3600000).toISOString();
const meta = () => ({
  sessionBinding: "fixture-binding",
  csrfToken: "fixture-csrf",
  sessionExpiresAt: expiry(),
});
const token = () => ({
  ...meta(),
  accessToken: "fixture-access",
  accessExpiresAt: expiry(),
});
const response = (data: unknown, status = 200, headers?: HeadersInit) =>
  new Response(JSON.stringify({ data }), { status, headers });
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("fetch", fetcher);
  vi.stubGlobal("BroadcastChannel", undefined);
  const entries = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => entries.get(key) ?? null,
    setItem: (key: string, value: string) => entries.set(key, value),
    removeItem: (key: string) => entries.delete(key),
    clear: () => entries.clear(),
    get length() {
      return entries.size;
    },
  });
  vi.stubGlobal("sessionStorage", { length: 0 });
  localStorage.clear();
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it("waits for the shared rotation lease instead of ending a concurrent tab's session", async () => {
  vi.useFakeTimers();
  const broker = new SessionBroker();
  fetcher.mockResolvedValueOnce(response(token(), 201));
  await broker.bootstrap(
    "fixture@example.com",
    "initial-access",
    "initial-refresh",
  );
  fetcher
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ error: { code: "session_busy" } }), {
        status: 409,
        headers: { "Retry-After": "1" },
      }),
    )
    .mockResolvedValueOnce(response(token()));
  const pending = broker.token(true);
  await vi.advanceTimersByTimeAsync(1000);
  expect(await pending).toBe("fixture-access");
  expect(fetcher.mock.calls[1][1].body).toBe(fetcher.mock.calls[2][1].body);
  expect(fetcher.mock.calls[1][1].headers).toEqual(
    fetcher.mock.calls[2][1].headers,
  );
  expect(await broker.token()).toBe("fixture-access");
});

it("revalidates an account switch without revoking the replacement session or replaying a product write", async () => {
  const broker = new SessionBroker();
  fetcher.mockResolvedValueOnce(response(token(), 201));
  await broker.bootstrap(
    "fixture@example.com",
    "initial-access",
    "initial-refresh",
  );
  const changed = vi.fn();
  window.addEventListener(sessionChangedEvent, changed);
  fetcher.mockResolvedValueOnce(
    new Response(JSON.stringify({ error: { code: "session_changed" } }), {
      status: 409,
    }),
  );
  await expect(broker.token(true)).rejects.toThrow("browser session changed");
  expect(changed).toHaveBeenCalledTimes(1);
  expect(fetcher.mock.calls.some(([, init]) => init.method === "DELETE")).toBe(
    false,
  );
  expect(localStorage.length).toBe(0);
  window.removeEventListener(sessionChangedEvent, changed);
});

it("restores using same-origin cookies, CSRF and session binding with no credential persistence", async () => {
  const broker = new SessionBroker();
  fetcher
    .mockResolvedValueOnce(response(meta()))
    .mockResolvedValueOnce(response(token()));
  expect(await broker.restore()).toBe(true);
  expect(await broker.token()).toBe("fixture-access");
  expect(fetcher).toHaveBeenCalledTimes(2);
  const [url, init] = fetcher.mock.calls[1];
  expect(url).toBe("/api/v1/browser-session/token");
  expect(init).toMatchObject({
    method: "POST",
    credentials: "same-origin",
    cache: "no-store",
    redirect: "error",
    headers: {
      "X-Kurier-Session": "1",
      "X-Kurier-CSRF": "fixture-csrf",
      "X-Kurier-Session-Binding": "fixture-binding",
    },
    body: "{}",
  });
  expect(localStorage.length).toBe(0);
  expect(sessionStorage.length).toBe(0);
});

it("restores an online cookie session when Web Storage is denied", async () => {
  vi.stubGlobal("localStorage", {
    getItem() {
      throw new DOMException("storage denied", "SecurityError");
    },
    setItem() {
      throw new DOMException("storage denied", "SecurityError");
    },
    removeItem() {
      throw new DOMException("storage denied", "SecurityError");
    },
  });
  const broker = new SessionBroker();
  fetcher
    .mockResolvedValueOnce(response(meta()))
    .mockResolvedValueOnce(response(token()));
  expect(await broker.restore()).toBe(true);
  expect(await broker.token()).toBe("fixture-access");
  fetcher.mockResolvedValueOnce(
    response({ signedOut: true, providerRevocation: "confirmed" }),
  );
  await broker.logout();
  await expect(broker.token()).rejects.toBeInstanceOf(SessionExpired);
});

it("explains cookie blocking before a completed handoff is exposed as persistent sign-in", async () => {
  const broker = new SessionBroker();
  fetcher
    .mockResolvedValueOnce(response(token(), 201))
    .mockResolvedValueOnce(response({}, 401));
  await broker.bootstrap(
    "fixture@example.com",
    "initial-access",
    "initial-refresh",
  );
  await expect(broker.confirmCookie()).rejects.toThrow(
    "Allow first-party cookies",
  );
});

it("retries a lost bootstrap response with exactly the same authenticated proof and credentials", async () => {
  const broker = new SessionBroker();
  fetcher
    .mockRejectedValueOnce(new TypeError("lost response"))
    .mockResolvedValueOnce(response(token(), 201));
  await broker.bootstrap(
    "fixture@example.com",
    "initial-access",
    "initial-refresh",
  );
  const first = fetcher.mock.calls[0][1];
  const second = fetcher.mock.calls[1][1];
  expect(first.body).toBe(second.body);
  expect(first.headers.Authorization).toBe("Bearer initial-access");
  const body = JSON.parse(first.body);
  for (const name of ["attemptId", "cancellationProof", "sessionProof"])
    expect(body[name]).toMatch(/^[A-Za-z0-9_-]{43}$/);
  expect(body.refreshToken).toBe("initial-refresh");
  expect(await broker.token()).toBe("fixture-access");
  expect(localStorage.length).toBe(0);
});

it("retains the bootstrap attempt for an explicit retry after two lost responses", async () => {
  const broker = new SessionBroker();
  fetcher
    .mockRejectedValueOnce(new TypeError())
    .mockRejectedValueOnce(new TypeError());
  await expect(
    broker.bootstrap(
      "fixture@example.com",
      "initial-access",
      "initial-refresh",
    ),
  ).rejects.toBeInstanceOf(SessionUnavailable);
  expect(broker.hasAttempt("fixture@example.com")).toBe(true);
  fetcher.mockResolvedValueOnce(response(token(), 201));
  await broker.bootstrap("fixture@example.com");
  expect(fetcher.mock.calls[2][1].body).toBe(fetcher.mock.calls[0][1].body);
});

it("does not retry rejected handoffs or violate provider Retry-After", async () => {
  const broker = new SessionBroker();
  fetcher.mockResolvedValueOnce(response({}, 429, { "Retry-After": "120" }));
  await expect(
    broker.bootstrap(
      "fixture@example.com",
      "initial-access",
      "initial-refresh",
    ),
  ).rejects.toThrow("Too many");
  await expect(broker.bootstrap("fixture@example.com")).rejects.toThrow(
    "Too many",
  );
  expect(fetcher).toHaveBeenCalledTimes(1);
});

it("cancels a pending bootstrap before logout and refuses a late success", async () => {
  const broker = new SessionBroker();
  let complete!: (value: Response) => void;
  fetcher.mockImplementationOnce(
    () =>
      new Promise<Response>((resolve) => {
        complete = resolve;
      }),
  );
  const pending = broker.bootstrap(
    "fixture@example.com",
    "initial-access",
    "initial-refresh",
  );
  const rejection = expect(pending).rejects.toBeInstanceOf(SessionExpired);
  fetcher
    .mockResolvedValueOnce(response({ canceled: true }))
    .mockResolvedValueOnce(response({}, 401))
    .mockResolvedValueOnce(
      response({ signedOut: true, providerRevocation: "confirmed" }),
    );
  await broker.logout();
  expect(fetcher.mock.calls[1][0]).toBe(
    "/api/v1/browser-session/bootstrap-cancellations",
  );
  const original = JSON.parse(fetcher.mock.calls[0][1].body);
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({
    attemptId: original.attemptId,
    cancellationProof: original.cancellationProof,
  });
  complete(response(token(), 201));
  await rejection;
  await expect(broker.token()).rejects.toBeInstanceOf(SessionExpired);
});

it("fences an in-flight refresh when signing out", async () => {
  const broker = new SessionBroker();
  fetcher.mockResolvedValueOnce(response(token(), 201));
  await broker.bootstrap(
    "fixture@example.com",
    "initial-access",
    "initial-refresh",
  );
  let complete!: (value: Response) => void;
  fetcher.mockImplementationOnce(
    () =>
      new Promise<Response>((resolve) => {
        complete = resolve;
      }),
  );
  const pending = broker.token(true);
  const rejection = expect(pending).rejects.toBeInstanceOf(SessionExpired);
  fetcher.mockResolvedValueOnce(
    response({ signedOut: true, providerRevocation: "pending" }),
  );
  expect(await broker.logout()).toContain("provider revocation is pending");
  complete(response(token()));
  await rejection;
  await expect(broker.token()).rejects.toBeInstanceOf(SessionExpired);
});

it("keeps a noncredential offline logout intent and settles it before restoration", async () => {
  const broker = new SessionBroker();
  fetcher.mockResolvedValueOnce(response(token(), 201));
  await broker.bootstrap(
    "fixture@example.com",
    "initial-access",
    "initial-refresh",
  );
  fetcher.mockRejectedValueOnce(new TypeError("offline"));
  await expect(broker.logout()).rejects.toThrow(
    "server sign-out awaits a connection",
  );
  expect(localStorage.getItem("kurier.browser-session.logout-pending")).toBe(
    "1",
  );
  await expect(broker.token()).rejects.toBeInstanceOf(SessionExpired);
  const reopened = new SessionBroker();
  fetcher
    .mockResolvedValueOnce(response(meta()))
    .mockResolvedValueOnce(response({ signedOut: true }));
  expect(await reopened.restore()).toBe(false);
  expect(localStorage.length).toBe(0);
});

it("treats temporary restoration failure differently from an ended session", async () => {
  fetcher.mockResolvedValueOnce(response({}, 503));
  await expect(new SessionBroker().restore()).rejects.toBeInstanceOf(
    SessionUnavailable,
  );
  fetcher.mockResolvedValueOnce(response({}, 401));
  expect(await new SessionBroker().restore()).toBe(false);
});

it("rejects an account-binding change and an expired fixed session", async () => {
  const broker = new SessionBroker();
  fetcher
    .mockResolvedValueOnce(response(meta()))
    .mockResolvedValueOnce(
      response({ ...token(), sessionBinding: "other-account" }),
    );
  expect(await broker.restore()).toBe(false);
  fetcher.mockResolvedValueOnce(
    response({ ...meta(), sessionExpiresAt: new Date(0).toISOString() }),
  );
  expect(await new SessionBroker().restore()).toBe(false);
});
