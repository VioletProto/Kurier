import {
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
  return { ...real, api: { ...real.api, me: mocks.me, list: mocks.list } };
});
import { App } from "./App";
beforeEach(() => {
  vi.clearAllMocks();
  mocks.broker = false;
  mocks.restore.mockResolvedValue(false);
  mocks.me.mockResolvedValue({
    userId: "user",
    displayName: "",
    createdAt: "2026-10-07T00:00:00Z",
  });
  mocks.list.mockResolvedValue({ items: [], nextCursor: null });
  mocks.logout.mockResolvedValue(undefined);
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
it("still revalidates a signed-in workspace when focus returns", async () => {
  mocks.broker = true;
  mocks.restore.mockResolvedValue(true);
  render(<App />);
  await screen.findByRole("button", { name: "Sign out" });
  fireEvent(window, new Event("focus"));
  await waitFor(() => expect(mocks.restore).toHaveBeenCalledTimes(2));
  await screen.findByRole("button", { name: "Sign out" });
});
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
