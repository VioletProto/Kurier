import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
const mocks = vi.hoisted(() => ({
  issueMCP: vi.fn(),
  currentMCP: vi.fn(),
  revokeMCP: vi.fn(),
}));
vi.mock("./api", async () => ({
  ...(await vi.importActual<typeof import("./api")>("./api")),
  api: mocks,
}));
import { MCPCredentials } from "./MCPCredentials";
import { ApiError, UncertainWrite } from "./api";
const credential = {
  tokenId: "mcp-id",
  projectIds: ["project"],
  createdAt: new Date().toISOString(),
  expiresAt: new Date(Date.now() + 86400000).toISOString(),
  revokedAt: null,
};
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});
it("requires deliberate scope consent, clears one-time token and revokes separately", async () => {
  mocks.issueMCP.mockResolvedValue({
    credential,
    token: "disposable-mcp-capability",
  });
  mocks.currentMCP.mockResolvedValue({
    ...credential,
    revokedAt: new Date().toISOString(),
  });
  mocks.revokeMCP.mockResolvedValue(undefined);
  render(
    <MCPCredentials
      projectId="project"
      projectName="Owned project"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  const issue = screen.getByRole("button", {
    name: "Create MCP credential",
  }) as HTMLButtonElement;
  expect(issue.disabled).toBe(true);
  fireEvent.click(screen.getByText("Refresh MCP status"));
  await screen.findByText(/Credential mcp-id/);
  fireEvent.click(screen.getByRole("checkbox"));
  fireEvent.click(issue);
  await screen.findByLabelText("One-time MCP token");
  expect(mocks.issueMCP).toHaveBeenCalledWith(
    expect.any(String),
    ["project"],
    expect.any(AbortSignal),
  );
  fireEvent.click(screen.getByText("Close MCP token display"));
  expect(screen.queryByLabelText("One-time MCP token")).toBeNull();
  fireEvent.click(screen.getByText("Revoke MCP access"));
  await waitFor(() =>
    expect(mocks.revokeMCP).toHaveBeenCalledWith(
      "mcp-id",
      expect.any(AbortSignal),
    ),
  );
});
it("discards a late issuance after switching projects", async () => {
  mocks.currentMCP.mockResolvedValue(null);
  let finish!: (v: unknown) => void;
  mocks.issueMCP.mockImplementation(
    () =>
      new Promise((r) => {
        finish = r;
      }),
  );
  const view = render(
    <MCPCredentials
      projectId="project"
      projectName="Owned"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  fireEvent.click(screen.getByText("Refresh MCP status"));
  await waitFor(() =>
    expect(screen.getByRole("checkbox").closest("fieldset")?.disabled).toBe(
      false,
    ),
  );
  fireEvent.click(screen.getByRole("checkbox"));
  fireEvent.click(screen.getByText("Create MCP credential"));
  view.rerender(
    <MCPCredentials
      projectId="other"
      projectName="Other"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  finish({ credential, token: "late-canary" });
  await waitFor(() =>
    expect(screen.queryByLabelText("One-time MCP token")).toBeNull(),
  );
});
it("blocks issuance when the API has not enabled MCP without claiming a lost token", async () => {
  mocks.currentMCP.mockRejectedValue(new ApiError(503, "service_unavailable"));
  render(
    <MCPCredentials
      projectId="project"
      projectName="Owned"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  expect(screen.getByRole("checkbox").closest("fieldset")?.disabled).toBe(true);
  fireEvent.click(screen.getByText("Refresh MCP status"));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "Deployment and activation may still be pending",
  );
  expect(screen.getByRole("alert").textContent).not.toContain(
    "issuance may have succeeded",
  );
  expect(mocks.issueMCP).not.toHaveBeenCalled();
});
it("keeps uncertain issuance recovery after a successful availability check", async () => {
  mocks.currentMCP.mockResolvedValue(null);
  mocks.issueMCP.mockRejectedValue(new UncertainWrite());
  render(
    <MCPCredentials
      projectId="project"
      projectName="Owned"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  fireEvent.click(screen.getByText("Refresh MCP status"));
  await waitFor(() =>
    expect(screen.getByRole("checkbox").closest("fieldset")?.disabled).toBe(
      false,
    ),
  );
  fireEvent.click(screen.getByRole("checkbox"));
  fireEvent.click(screen.getByText("Create MCP credential"));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "issuance may have succeeded",
  );
  expect(mocks.issueMCP).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("checkbox").closest("fieldset")?.disabled).toBe(true);
});
