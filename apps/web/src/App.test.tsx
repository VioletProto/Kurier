import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
const mocks = vi.hoisted(() => ({
  login: vi.fn(),
  signup: vi.fn(),
  verify: vi.fn(),
  resend: vi.fn(),
  reset: vi.fn(),
  confirmReset: vi.fn(),
  logout: vi.fn(),
  restore: vi.fn(),
  suspend: vi.fn(),
  broker: false,
  token: vi.fn(),
  me: vi.fn(),
  list: vi.fn(),
  currentAgent: vi.fn(),
  pairAgent: vi.fn(),
}));
vi.mock("./auth", async () => {
  const real = await vi.importActual<typeof import("./auth")>("./auth");
  return {
    ...real,
    authConfigured: true,
    get sessionBrokerEnabled() {
      return mocks.broker;
    },
    auth: {
      login: mocks.login,
      signup: mocks.signup,
      verify: mocks.verify,
      resend: mocks.resend,
      reset: mocks.reset,
      confirmReset: mocks.confirmReset,
      logout: mocks.logout,
      restore: mocks.restore,
      suspend: mocks.suspend,
      token: mocks.token,
    },
  };
});
vi.mock("./api", async () => {
  const real = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...real,
    api: {
      ...real.api,
      me: mocks.me,
      list: mocks.list,
      currentAgent: mocks.currentAgent,
      pairAgent: mocks.pairAgent,
    },
  };
});
import { App } from "./App";
import {
  SessionExpired,
  SessionUnavailable,
  sessionChangedEvent,
} from "./auth";
beforeEach(() => {
  vi.resetAllMocks();
  mocks.broker = false;
  mocks.restore.mockResolvedValue(false);
  mocks.me.mockResolvedValue({
    userId: "user",
    displayName: "",
    createdAt: "2026-10-07T00:00:00Z",
  });
  mocks.list.mockResolvedValue({ items: [], nextCursor: null });
  mocks.currentAgent.mockResolvedValue(null);
  mocks.logout.mockResolvedValue(undefined);
  mocks.token.mockResolvedValue("fixture-access");
});
afterEach(cleanup);
it("holds both private content and sign-in while restoring a broker session", async () => {
  mocks.broker = true;
  let finish!: (restored: boolean) => void;
  mocks.restore.mockReturnValue(
    new Promise<boolean>((resolve) => {
      finish = resolve;
    }),
  );
  render(<App />);
  expect(screen.getByText("Restoring your session…")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Sign in" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Sign out" })).toBeNull();
  finish(true);
  await screen.findByRole("button", { name: "Sign out" });
  expect(mocks.me).toHaveBeenCalled();
});
it("keeps transient broker failures retryable without rendering sign-in", async () => {
  mocks.broker = true;
  mocks.restore.mockRejectedValueOnce(new Error("temporary"));
  render(<App />);
  await screen.findByRole("button", { name: "Retry session" });
  expect(screen.queryByRole("button", { name: "Sign in" })).toBeNull();
  mocks.restore.mockResolvedValueOnce(false);
  fireEvent.click(screen.getByRole("button", { name: "Retry session" }));
  await screen.findByRole("button", { name: "Sign in" });
});
it("preserves password-manager-filled fields when focus returns to sign-in", async () => {
  mocks.broker = true;
  render(<App />);
  await screen.findByRole("button", { name: /^Sign in$/ });
  const originalEmail = screen.getByLabelText("Email");
  const originalPassword = screen.getByLabelText("Password");
  email();
  password();
  fireEvent(window, new Event("blur"));
  fireEvent(window, new Event("focus"));
  expect(screen.getByLabelText("Email")).toBe(originalEmail);
  expect(screen.getByLabelText("Password")).toBe(originalPassword);
  expect(originalEmail).toHaveValue("fixture@example.com");
  expect(originalPassword).toHaveValue("Test-only-password1!");
  expect(mocks.restore).toHaveBeenCalledTimes(1);
});
it("checks focus in the background, coalesces events and preserves the mounted draft", async () => {
  mocks.broker = true;
  mocks.restore.mockResolvedValue(true);
  render(<App />);
  const draft = await screen.findByLabelText("New project name");
  await waitFor(() => expect(draft).toBeEnabled());
  fireEvent.change(draft, { target: { value: "Unsaved project" } });
  draft.focus();
  let finish!: (token: string) => void;
  mocks.token.mockReturnValueOnce(
    new Promise<string>((resolve) => {
      finish = resolve;
    }),
  );
  fireEvent(window, new Event("focus"));
  fireEvent(window, new Event("focus"));
  expect(mocks.token).toHaveBeenCalledExactlyOnceWith(true);
  expect(screen.getByLabelText("New project name")).toBe(draft);
  expect(draft).toHaveFocus();
  expect(screen.queryByText("Restoring your session…")).toBeNull();
  await act(async () => finish("fixture-access"));
  expect(screen.getByLabelText("New project name")).toBe(draft);
  expect(draft).toHaveValue("Unsaved project");
  expect(mocks.restore).toHaveBeenCalledTimes(1);
  expect(mocks.list).toHaveBeenCalledTimes(1);
});
it("preserves drafts on a temporary focus-check failure and retries on focus", async () => {
  mocks.broker = true;
  mocks.restore.mockResolvedValue(true);
  render(<App />);
  const draft = await screen.findByLabelText("New project name");
  await waitFor(() => expect(draft).toBeEnabled());
  fireEvent.change(draft, { target: { value: "Keep this draft" } });
  mocks.token.mockRejectedValueOnce(new SessionUnavailable());
  fireEvent(window, new Event("focus"));
  await screen.findByText(new SessionUnavailable().message);
  expect(screen.getByLabelText("New project name")).toBe(draft);
  expect(draft).toHaveValue("Keep this draft");
  fireEvent(window, new Event("focus"));
  await waitFor(() =>
    expect(screen.queryByText(new SessionUnavailable().message)).toBeNull(),
  );
  expect(draft).toHaveValue("Keep this draft");
});
it("clears the workspace on confirmed expiry during a focus check", async () => {
  mocks.broker = true;
  mocks.restore.mockResolvedValue(true);
  render(<App />);
  await screen.findByRole("button", { name: "Sign out" });
  mocks.token.mockRejectedValueOnce(new SessionExpired());
  fireEvent(window, new Event("focus"));
  await screen.findByRole("button", { name: /^Sign in$/ });
  expect(screen.queryByLabelText("New project name")).toBeNull();
  expect(mocks.logout).toHaveBeenCalledTimes(1);
});
it.each(["signout", "replacement", "pagehide"])(
  "ignores late focus-check expiry after %s",
  async (transition) => {
    mocks.broker = true;
    mocks.restore.mockResolvedValue(true);
    render(<App />);
    await waitFor(() =>
      expect(screen.getByLabelText("New project name")).toBeEnabled(),
    );
    let fail!: (error: Error) => void;
    mocks.token.mockImplementationOnce(
      () =>
        new Promise<string>((_, reject) => {
          fail = reject;
        }),
    );
    fireEvent(window, new Event("focus"));
    expect(mocks.token).toHaveBeenCalledExactlyOnceWith(true);
    if (transition === "signout")
      fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    else if (transition === "replacement") {
      fireEvent(window, new Event(sessionChangedEvent));
      await waitFor(() => expect(mocks.restore).toHaveBeenCalledTimes(2));
      await screen.findByRole("button", { name: "Sign out" });
    } else
      fireEvent(
        window,
        new PageTransitionEvent("pagehide", { persisted: true }),
      );
    await act(async () => fail(new SessionExpired()));
    expect(mocks.logout).toHaveBeenCalledTimes(
      transition === "signout" ? 1 : 0,
    );
    if (transition === "replacement")
      expect(
        screen.getByRole("button", { name: "Sign out" }),
      ).toBeInTheDocument();
  },
);
it.each([false, true])(
  "allows signing in again after sign-out (broker=%s)",
  async (broker) => {
    mocks.broker = broker;
    mocks.login.mockResolvedValue("done");
    render(<App />);
    await screen.findByRole("button", { name: /^Sign in$/ });
    email();
    password();
    fireEvent.click(screen.getByRole("button", { name: /^Sign in$/ }));
    await screen.findByRole("button", { name: "Sign out" });
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    await screen.findByRole("button", { name: /^Sign in$/ });
    email();
    password();
    fireEvent.click(screen.getByRole("button", { name: /^Sign in$/ }));
    await screen.findByRole("button", { name: "Sign out" });
    expect(mocks.login).toHaveBeenCalledTimes(2);
    expect(mocks.logout).toHaveBeenCalledTimes(1);
  },
);
it("suspends broker state on pagehide without revocation and restores BFCache", async () => {
  mocks.broker = true;
  mocks.restore.mockResolvedValue(true);
  render(<App />);
  await screen.findByRole("button", { name: "Sign out" });
  fireEvent(window, new PageTransitionEvent("pagehide", { persisted: true }));
  expect(mocks.suspend).toHaveBeenCalled();
  expect(mocks.logout).not.toHaveBeenCalled();
  expect(screen.queryByRole("button", { name: "Sign out" })).toBeNull();
  fireEvent(window, new PageTransitionEvent("pageshow", { persisted: true }));
  await screen.findByRole("button", { name: "Sign out" });
  expect(mocks.restore).toHaveBeenCalledTimes(2);
});
it("clears signed-in state and auth inputs across browser history restoration", async () => {
  mocks.login.mockResolvedValue("done");
  render(<App />);
  email();
  password();
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
  await screen.findByRole("button", { name: "Sign out" });
  fireEvent(window, new PageTransitionEvent("pagehide", { persisted: true }));
  await screen.findByRole("button", { name: "Sign in" });
  expect(screen.queryByRole("button", { name: "Sign out" })).toBeNull();
  expect(screen.getByLabelText("Email")).toHaveValue("");
  expect(screen.getByLabelText("Password")).toHaveValue("");
  expect(mocks.logout).toHaveBeenCalled();
  fireEvent(window, new PageTransitionEvent("pageshow", { persisted: true }));
  await screen.findByRole("button", { name: "Sign in" });
  expect(
    screen.getByText("Your session ended. Please sign in again."),
  ).toBeInTheDocument();
});
it("does not restore a workspace when sign-in completes after pagehide", async () => {
  let finish!: (result: string) => void;
  mocks.login.mockReturnValue(
    new Promise<string>((resolve) => {
      finish = resolve;
    }),
  );
  render(<App />);
  email();
  password();
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
  fireEvent(window, new PageTransitionEvent("pagehide", { persisted: true }));
  await screen.findByRole("button", { name: "Sign in" });
  finish("done");
  await waitFor(() => expect(mocks.logout).toHaveBeenCalledTimes(2));
  expect(screen.queryByRole("button", { name: "Sign out" })).toBeNull();
  expect(screen.getByLabelText("Password")).toHaveValue("");
});
function email() {
  fireEvent.change(screen.getByLabelText("Email"), {
    target: { value: "fixture@example.com" },
  });
}
function password() {
  fireEvent.change(screen.getByLabelText("Password"), {
    target: { value: "Test-only-password1!" },
  });
}
describe("accessible authentication screens", () => {
  it("identifies the workspace and switches the shared theme", () => {
    render(<App />);
    expect(screen.getByRole("heading", { name: "Kurier" })).toBeInTheDocument();
    expect(screen.getByText("Environment: local")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Dark theme" }));
    expect(document.documentElement.dataset.theme).toBe("dark");
  });
  it("shows signup loading, moves to verification and enforces resend cooldown", async () => {
    let resolve!: () => void;
    mocks.signup.mockReturnValue(
      new Promise<void>((r) => {
        resolve = r;
      }),
    );
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    email();
    password();
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(screen.getByLabelText("Email")).toBeDisabled();
    resolve();
    await screen.findByLabelText("Verification code");
    expect(
      screen.getByRole("button", { name: "Resend code in 60s" }),
    ).toBeDisabled();
    expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
    mocks.verify.mockResolvedValue({});
    fireEvent.change(screen.getByLabelText("Verification code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify email" }));
    await screen.findByRole("heading", { name: "Sign in" });
  });
  it("handles invalid/expired codes without displaying Cognito internals", async () => {
    mocks.verify.mockRejectedValue(
      Object.assign(new Error("raw upstream detail"), {
        name: "ExpiredCodeException",
      }),
    );
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: "Verify email" }));
    email();
    fireEvent.change(screen.getByLabelText("Verification code"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Verify email" }));
    await screen.findByText(
      "The code is invalid or expired. Request a new code and try again.",
    );
    expect(screen.queryByText("raw upstream detail")).not.toBeInTheDocument();
  });
  it("supports reset request and confirmation", async () => {
    mocks.reset.mockResolvedValue({});
    mocks.confirmReset.mockResolvedValue({});
    render(<App />);
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    email();
    fireEvent.click(screen.getByRole("button", { name: "Reset password" }));
    await screen.findByLabelText("New password");
    fireEvent.change(screen.getByLabelText("Verification code"), {
      target: { value: "123456" },
    });
    fireEvent.change(screen.getByLabelText("New password"), {
      target: { value: "Test-only-password2!" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Choose a new password" }),
    );
    await screen.findByText(
      "Password updated. Sign in with your new password.",
    );
    expect(mocks.confirmReset).toHaveBeenCalled();
  });
  it("loads current user/projects after sign-in and removes them on sign-out", async () => {
    mocks.login.mockResolvedValue("done");
    render(<App />);
    email();
    password();
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    await screen.findByRole("heading", { name: "Your projects" });
    await waitFor(() => expect(mocks.me).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    await screen.findByRole("heading", { name: "Sign in" });
    expect(
      screen.queryByRole("heading", { name: "Your projects" }),
    ).not.toBeInTheDocument();
    expect(mocks.logout).toHaveBeenCalled();
  });
});

it("keeps one account settings surface and ignores a stale initial agent lookup", async () => {
  mocks.login.mockResolvedValue("done");
  let initial!: (value: null) => void;
  mocks.currentAgent.mockReturnValueOnce(
    new Promise<null>((resolve) => {
      initial = resolve;
    }),
  );
  const agent = {
    localAgentId: "shared",
    name: "Account device",
    projectScope: "all_owned_projects",
    scopes: [],
    destinations: [
      { origin: "http://localhost:3000", addressClass: "loopback" },
    ],
    createdAt: new Date().toISOString(),
    expiresAt: new Date(Date.now() + 86400000).toISOString(),
    revokedAt: null,
    lastSeenAt: null,
    recoveryGeneration: "fixture",
  };
  mocks.pairAgent.mockResolvedValue({ agent, token: "fixture-once" });
  render(<App />);
  email();
  password();
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
  await screen.findByRole("heading", { name: "Your projects" });
  expect(
    screen.queryByRole("region", { name: "Read-only MCP access" }),
  ).toBeNull();
  const draft = screen.getByLabelText("New project name");
  fireEvent.change(draft, { target: { value: "Unsaved project" } });
  fireEvent.click(screen.getByRole("button", { name: "Settings" }));
  expect(
    screen.getAllByRole("region", { name: "Read-only MCP access" }),
  ).toHaveLength(1);
  fireEvent.click(
    screen.getByLabelText(
      "I authorize all my current and future projects to use these exact destinations on the named device.",
    ),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Create agent credential" }),
  );
  await screen.findByText(/Agent: Account device/);
  await act(async () => initial(null));
  expect(screen.getByText(/Agent: Account device/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Projects" }));
  expect(screen.getByLabelText("New project name")).toBe(draft);
  expect(draft).toHaveValue("Unsaved project");
  expect(screen.queryByLabelText("One-time agent token")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Settings" }));
  expect(screen.getByText(/Agent: Account device/)).toBeInTheDocument();
  expect(screen.queryByLabelText("One-time agent token")).toBeNull();
});
