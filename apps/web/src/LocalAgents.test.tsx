import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { UncertainWrite, type LocalAgent } from "./api";
const mocks = vi.hoisted(() => ({
  currentAgent: vi.fn(),
  pairAgent: vi.fn(),
  revokeAgent: vi.fn(),
}));
vi.mock("./api", async () => ({
  ...(await vi.importActual<typeof import("./api")>("./api")),
  api: mocks,
}));
import { LocalAgents } from "./LocalAgents";
beforeEach(() => {
  vi.stubGlobal("localStorage", {
    length: 0,
    setItem: vi.fn(),
    clear: vi.fn(),
  });
  vi.stubGlobal("sessionStorage", {
    length: 0,
    setItem: vi.fn(),
    clear: vi.fn(),
  });
});
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
  vi.unstubAllGlobals();
});
const agent: LocalAgent = {
  localAgentId: "agent",
  name: "Device",
  scopes: [
    {
      projectId: "project",
      destinations: [
        { origin: "http://localhost:3000", addressClass: "loopback" },
      ],
    },
  ],
  createdAt: new Date().toISOString(),
  expiresAt: new Date(Date.now() + 86400000).toISOString(),
  revokedAt: null,
  lastSeenAt: null,
  recoveryGeneration: "fixture",
};
it("requires explicit project/destination consent and clears the one-time credential without persistence", async () => {
  mocks.pairAgent.mockResolvedValue({
    agent,
    token: "disposable-one-time-capability",
  });
  const changed = vi.fn();
  const view = render(
    <LocalAgents
      projectId="project"
      disabled={false}
      onAgent={changed}
      onSessionExpired={vi.fn()}
    />,
  );
  expect(
    (
      screen.getByRole("button", {
        name: "Create agent credential",
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  fireEvent.click(
    screen.getByLabelText(
      "I authorize this project and these exact destinations for the named device.",
    ),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Create agent credential" }),
  );
  await screen.findByLabelText("One-time agent token");
  expect(mocks.pairAgent.mock.calls[0][1].scopes[0].projectId).toBe("project");
  expect(localStorage.length).toBe(0);
  expect(sessionStorage.length).toBe(0);
  fireEvent.click(screen.getByRole("button", { name: "Close token display" }));
  expect(screen.queryByLabelText("One-time agent token")).toBeNull();
  view.unmount();
  expect(changed).toHaveBeenCalledWith(agent);
});
it("handles lost issuance response by refresh and deliberate revoke instead of another credential", async () => {
  mocks.pairAgent.mockRejectedValue(new UncertainWrite());
  mocks.currentAgent.mockResolvedValue(agent);
  render(
    <LocalAgents
      projectId="project"
      disabled={false}
      onAgent={vi.fn()}
      onSessionExpired={vi.fn()}
    />,
  );
  fireEvent.click(
    screen.getByLabelText(
      "I authorize this project and these exact destinations for the named device.",
    ),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Create agent credential" }),
  );
  await screen.findByText(/Pairing outcome is unresolved/);
  expect(mocks.pairAgent).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole("button", { name: "Refresh pairing status" }),
  );
  await waitFor(() => expect(screen.getByText(/Agent: Device/)).toBeTruthy());
  expect(
    (
      screen
        .getByRole("button", { name: "Create agent credential" })
        .closest("fieldset") as HTMLFieldSetElement
    ).disabled,
  ).toBe(true);
  mocks.revokeAgent.mockResolvedValue(undefined);
  mocks.currentAgent.mockResolvedValue({
    ...agent,
    revokedAt: new Date().toISOString(),
  });
  fireEvent.click(screen.getByRole("button", { name: "Revoke local agent" }));
  await waitFor(() =>
    expect(mocks.revokeAgent).toHaveBeenCalledWith(
      "agent",
      expect.any(AbortSignal),
    ),
  );
});
it("discards a late pairing response after project switch", async () => {
  let complete!: (value: unknown) => void;
  mocks.pairAgent.mockImplementation(
    () =>
      new Promise((resolve) => {
        complete = resolve;
      }),
  );
  const changed = vi.fn();
  const view = render(
    <LocalAgents
      projectId="project"
      disabled={false}
      onAgent={changed}
      onSessionExpired={vi.fn()}
    />,
  );
  fireEvent.click(
    screen.getByLabelText(
      "I authorize this project and these exact destinations for the named device.",
    ),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Create agent credential" }),
  );
  view.rerender(
    <LocalAgents
      projectId="other"
      disabled={false}
      onAgent={changed}
      onSessionExpired={vi.fn()}
    />,
  );
  complete({ agent, token: "late-disposable-capability" });
  await waitFor(() =>
    expect(screen.queryByLabelText("One-time agent token")).toBeNull(),
  );
  expect(changed).not.toHaveBeenCalled();
});
