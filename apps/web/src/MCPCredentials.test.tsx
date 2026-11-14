import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
const mocks = vi.hoisted(() => ({
  issueMCPConnection: vi.fn(),
  currentMCPConnection: vi.fn(),
  revokeMCP: vi.fn(),
  list: vi.fn().mockResolvedValue({ items: [], nextCursor: null }),
  updateMCPScope: vi.fn(),
}));
vi.mock("./api", async () => ({
  ...(await vi.importActual<typeof import("./api")>("./api")),
  api: mocks,
}));
import { MCPCredentials } from "./MCPCredentials";
import { ApiError, UncertainWrite } from "./api";
const credential = {
  tokenId: "mcp-id",
  scope: { mode: "selected_projects", projectIds: ["project"] },
  version: 0,
  editable: true,
  createdAt: new Date().toISOString(),
  expiresAt: new Date(Date.now() + 86400000).toISOString(),
  revokedAt: null,
};
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  mocks.list.mockResolvedValue({ items: [], nextCursor: null });
});
it("requires deliberate scope consent, clears one-time token and revokes separately", async () => {
  mocks.issueMCPConnection.mockResolvedValue({
    connection: credential,
    token: "disposable-mcp-capability",
  });
  mocks.currentMCPConnection.mockResolvedValue({
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
  fireEvent.click(screen.getByLabelText(/^I authorize/));
  fireEvent.click(issue);
  await screen.findByLabelText("One-time MCP token");
  expect(mocks.issueMCPConnection).toHaveBeenCalledWith(
    expect.any(String),
    { mode: "selected_projects", projectIds: ["project"] },
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
  mocks.currentMCPConnection.mockResolvedValue(null);
  let finish!: (v: unknown) => void;
  mocks.issueMCPConnection.mockImplementation(
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
    expect(
      screen.getByLabelText(/^I authorize/).closest("fieldset")?.disabled,
    ).toBe(false),
  );
  fireEvent.click(screen.getByLabelText(/^I authorize/));
  fireEvent.click(screen.getByText("Create MCP credential"));
  view.rerender(
    <MCPCredentials
      projectId="other"
      projectName="Other"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  finish({ connection: credential, token: "late-canary" });
  await waitFor(() =>
    expect(screen.queryByLabelText("One-time MCP token")).toBeNull(),
  );
});
it("blocks issuance when the API has not enabled MCP without claiming a lost token", async () => {
  mocks.currentMCPConnection.mockRejectedValue(
    new ApiError(503, "service_unavailable"),
  );
  render(
    <MCPCredentials
      projectId="project"
      projectName="Owned"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  expect(
    screen.getByLabelText(/^I authorize/).closest("fieldset")?.disabled,
  ).toBe(true);
  fireEvent.click(screen.getByText("Refresh MCP status"));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "Deployment and activation may still be pending",
  );
  expect(screen.getByRole("alert").textContent).not.toContain(
    "issuance may have succeeded",
  );
  expect(mocks.issueMCPConnection).not.toHaveBeenCalled();
});
it("keeps uncertain issuance recovery after a successful availability check", async () => {
  mocks.currentMCPConnection.mockResolvedValue(null);
  mocks.issueMCPConnection.mockRejectedValue(new UncertainWrite());
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
    expect(
      screen.getByLabelText(/^I authorize/).closest("fieldset")?.disabled,
    ).toBe(false),
  );
  fireEvent.click(screen.getByLabelText(/^I authorize/));
  fireEvent.click(screen.getByText("Create MCP credential"));
  expect((await screen.findByRole("alert")).textContent).toContain(
    "issuance may have succeeded",
  );
  expect(mocks.issueMCPConnection).toHaveBeenCalledTimes(1);
  expect(
    screen.getByLabelText(/^I authorize/).closest("fieldset")?.disabled,
  ).toBe(true);
});

it("requires future-project consent with no projects and clears consent on mode changes", async () => {
  mocks.currentMCPConnection.mockResolvedValue(null);
  mocks.issueMCPConnection.mockResolvedValue({
    connection: { ...credential, scope: { mode: "all_owned_projects" } },
    token: "once",
  });
  render(<MCPCredentials disabled={false} onSessionExpired={vi.fn()} />);
  fireEvent.click(screen.getByText("Refresh MCP status"));
  await waitFor(() =>
    expect(
      screen
        .getByLabelText("All my projects, including future projects")
        .closest("fieldset")?.disabled,
    ).toBe(false),
  );
  fireEvent.click(
    screen.getByLabelText("All my projects, including future projects"),
  );
  fireEvent.click(screen.getByLabelText(/^I authorize/));
  expect(
    (screen.getByText("Create MCP credential") as HTMLButtonElement).disabled,
  ).toBe(true);
  fireEvent.click(screen.getByLabelText(/^I understand/));
  fireEvent.click(screen.getByLabelText("Choose projects"));
  expect(
    (screen.getByLabelText(/^I authorize/) as HTMLInputElement).checked,
  ).toBe(false);
  fireEvent.click(
    screen.getByLabelText("All my projects, including future projects"),
  );
  expect(
    (screen.getByLabelText(/^I understand/) as HTMLInputElement).checked,
  ).toBe(false);
  fireEvent.click(screen.getByLabelText(/^I authorize/));
  fireEvent.click(screen.getByLabelText(/^I understand/));
  fireEvent.click(screen.getByText("Create MCP credential"));
  await screen.findByLabelText("One-time MCP token");
  expect(mocks.issueMCPConnection).toHaveBeenCalledWith(
    expect.any(String),
    { mode: "all_owned_projects" },
    expect.any(AbortSignal),
  );
});
it("edits shared scope without issuing a token and requires review after conflicts", async () => {
  const automatic = {
    ...credential,
    scope: { mode: "all_owned_projects" },
    version: 4,
  };
  mocks.currentMCPConnection.mockResolvedValue(automatic);
  mocks.updateMCPScope.mockRejectedValue(
    new ApiError(412, "precondition_failed"),
  );
  render(
    <MCPCredentials
      projectId="project"
      projectName="Owned"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  fireEvent.click(screen.getByText("Refresh MCP status"));
  await screen.findByText(/Credential mcp-id/);
  fireEvent.click(screen.getByLabelText("Choose projects"));
  fireEvent.click(screen.getByLabelText(/^I authorize/));
  fireEvent.click(screen.getByText("Save project access"));
  await screen.findByText(/Project access changed in another session/);
  expect(mocks.updateMCPScope).toHaveBeenCalledWith(
    "mcp-id",
    4,
    { mode: "selected_projects", projectIds: ["project"] },
    expect.any(AbortSignal),
  );
  expect(
    screen.getByLabelText(/^I authorize/).closest("fieldset")?.disabled,
  ).toBe(true);
  expect(mocks.issueMCPConnection).not.toHaveBeenCalled();
});
