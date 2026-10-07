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
  token: vi.fn(),
  me: vi.fn(),
  list: vi.fn(),
}));
vi.mock("./auth", async () => {
  const real = await vi.importActual<typeof import("./auth")>("./auth");
  return {
    ...real,
    authConfigured: true,
    auth: {
      login: mocks.login,
      signup: mocks.signup,
      verify: mocks.verify,
      resend: mocks.resend,
      reset: mocks.reset,
      confirmReset: mocks.confirmReset,
      logout: mocks.logout,
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
  mocks.me.mockResolvedValue({
    userId: "user",
    displayName: "",
    createdAt: "2026-10-07T00:00:00Z",
  });
  mocks.list.mockResolvedValue({ items: [], nextCursor: null });
  mocks.logout.mockResolvedValue(undefined);
});
afterEach(cleanup);
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
