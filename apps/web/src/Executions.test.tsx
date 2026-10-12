import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  ApiError,
  UncertainWrite,
  type Execution,
  type RequestDetail,
} from "./api";
const mocks = vi.hoisted(() => ({
  executionHistory: vi.fn(),
  submitExecution: vi.fn(),
  executionDetail: vi.fn(),
  executionStatus: vi.fn(),
  executionEvidence: vi.fn(),
  rerunExecution: vi.fn(),
  pinExecution: vi.fn(),
}));
vi.mock("./api", async () => ({
  ...(await vi.importActual<typeof import("./api")>("./api")),
  api: mocks,
}));
import { Executions } from "./Executions";
const request = {
  request: {
    requestId: "request-a",
    projectId: "project",
    revision: 3,
    name: "Fixture",
    method: "GET",
    url: "https://fixture.example/",
    headers: [],
    queryParameters: [],
    body: null,
    operationRef: null,
    createdAt: "date",
    updatedAt: "date",
  },
  etag: '"3"',
} as RequestDetail;
const execution: Execution = {
  executionId: "execution-a",
  projectId: "project",
  status: "queued",
  version: 0,
  submittedAt: "date",
  startedAt: null,
  completedAt: null,
  source: { requestId: "request-a", revision: 3 },
  liveRequestId: "request-a",
  summary: null,
  retention: null,
  serverTime: "date",
};
beforeEach(() => {
  vi.resetAllMocks();
  mocks.executionHistory.mockResolvedValue({ items: [], nextCursor: null });
  mocks.submitExecution.mockResolvedValue(execution);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

function retained(pinned = false, expired = false): Execution {
  const now = new Date().toISOString();
  return {
    ...execution,
    status: "completed",
    serverTime: now,
    completedAt: now,
    retention: {
      normalExpiresAt: new Date(
        Date.now() + (expired ? -10000 : 86400000),
      ).toISOString(),
      pinned,
      evidenceAvailability: "available",
      version: 0,
      pinnedAt: pinned ? now : null,
      state: "live",
      effectiveStatus: pinned ? "pinned" : "retained",
      pinEligible: true,
    },
  };
}
it("pins row and detail together and keeps controls pending until acknowledgement", async () => {
  const current = retained();
  mocks.executionHistory.mockResolvedValue({
    items: [current],
    nextCursor: null,
  });
  mocks.executionDetail.mockResolvedValue(current);
  let finish!: (v: unknown) => void;
  mocks.pinExecution.mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  mount();
  await screen.findByRole("button", { name: "Pin evidence execution-a" });
  fireEvent.click(
    screen.getByRole("button", { name: /completed.*revision 3/ }),
  );
  await screen.findByRole("heading", { name: "Execution execution-a" });
  fireEvent.click(
    screen.getAllByRole("button", { name: "Pin evidence execution-a" })[0],
  );
  await screen.findAllByText("Pinning…");
  expect(
    screen.getAllByRole("button", { name: "Pin evidence execution-a" })[1],
  ).toBeDisabled();
  const next = retained(true);
  mocks.executionHistory.mockResolvedValue({ items: [next], nextCursor: null });
  finish({ retention: next.retention, serverTime: next.serverTime });
  await screen.findAllByText(/Protected from automatic expiry while pinned\./);
  expect(
    screen.getAllByRole("button", { name: "Unpin evidence execution-a" }),
  ).toHaveLength(2);
  expect(mocks.pinExecution.mock.calls[0].slice(0, 3)).toEqual([
    "project",
    "execution-a",
    true,
  ]);
});
it("confirms late unpin and clears expired selection immediately", async () => {
  const current = retained(true, true);
  mocks.executionHistory.mockResolvedValue({
    items: [current],
    nextCursor: null,
  });
  mocks.executionDetail.mockResolvedValue(current);
  mount();
  await screen.findByRole("button", { name: "Unpin evidence execution-a" });
  fireEvent.click(
    screen.getByRole("button", { name: /completed.*revision 3/ }),
  );
  await screen.findByRole("heading", { name: "Execution execution-a" });
  fireEvent.click(
    screen.getAllByRole("button", { name: "Unpin evidence execution-a" })[1],
  );
  await screen.findByRole("dialog");
  expect(mocks.pinExecution).not.toHaveBeenCalled();
  const next = retained(false, true);
  next.retention!.effectiveStatus = "expired";
  mocks.pinExecution.mockResolvedValue({
    retention: next.retention,
    serverTime: next.serverTime,
  });
  mocks.executionHistory.mockResolvedValue({ items: [], nextCursor: null });
  fireEvent.click(
    screen.getByRole("button", { name: "Confirm unpin expired evidence" }),
  );
  await screen.findByText(/Automatic expiry applies immediately/);
  expect(
    screen.queryByRole("heading", { name: "Execution execution-a" }),
  ).not.toBeInTheDocument();
});
it("refreshes a quota conflict without replaying pin and reconciles uncertain writes deliberately", async () => {
  const current = retained();
  mocks.executionHistory.mockResolvedValue({
    items: [current],
    nextCursor: null,
  });
  mocks.executionDetail.mockResolvedValue(current);
  mocks.pinExecution
    .mockRejectedValueOnce(new ApiError(409, "pin_quota_exceeded"))
    .mockRejectedValueOnce(new UncertainWrite());
  mount();
  await screen.findByRole("button", { name: "Pin evidence execution-a" });
  fireEvent.click(
    screen.getByRole("button", { name: "Pin evidence execution-a" }),
  );
  await screen.findByText(/Pin quota reached/);
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Pin evidence execution-a" }),
    ).toBeEnabled(),
  );
  expect(mocks.pinExecution).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole("button", { name: "Pin evidence execution-a" }),
  );
  await screen.findByText(/Pin operation outcome is unresolved/);
  expect(
    screen.getByRole("button", { name: "Pin evidence execution-a" }),
  ).toBeDisabled();
  fireEvent.click(
    screen.getByRole("button", { name: "Refresh execution history" }),
  );
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Pin evidence execution-a" }),
    ).toBeEnabled(),
  );
  expect(mocks.pinExecution).toHaveBeenCalledTimes(2);
});
it("allows unavailable pinned evidence to be unpinned but blocks a new pin", async () => {
  const current = retained();
  current.retention!.evidenceAvailability = "unavailable";
  current.retention!.pinEligible = false;
  mocks.executionHistory.mockResolvedValue({
    items: [current],
    nextCursor: null,
  });
  const view = mount();
  expect(
    await screen.findByRole("button", { name: "Pin evidence execution-a" }),
  ).toBeDisabled();
  view.unmount();
  current.retention!.pinned = true;
  mocks.executionHistory.mockResolvedValue({
    items: [current],
    nextCursor: null,
  });
  mount();
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Unpin evidence execution-a" }),
    ).toBeEnabled(),
  );
});
it("ignores a delayed pin response after switching project", async () => {
  const current = retained();
  mocks.executionHistory.mockResolvedValue({
    items: [current],
    nextCursor: null,
  });
  let finish!: (v: unknown) => void;
  mocks.pinExecution.mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const view = mount();
  await screen.findByRole("button", { name: "Pin evidence execution-a" });
  fireEvent.click(
    screen.getByRole("button", { name: "Pin evidence execution-a" }),
  );
  await screen.findByText("Pinning…");
  mocks.executionHistory.mockResolvedValue({ items: [], nextCursor: null });
  view.rerender(
    <Executions
      projectId="another"
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
  finish({
    retention: retained(true).retention,
    serverTime: new Date().toISOString(),
  });
  await waitFor(() =>
    expect(
      screen.queryByText("Protected from automatic expiry while pinned."),
    ).not.toBeInTheDocument(),
  );
});
function mount() {
  return render(
    <Executions
      projectId="project"
      request={request}
      disabled={false}
      onSessionExpired={vi.fn()}
    />,
  );
}
it("keeps source, revision, key and policy on unresolved submission reconciliation", async () => {
  mocks.submitExecution
    .mockRejectedValueOnce(new UncertainWrite())
    .mockResolvedValueOnce(execution);
  mount();
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Execute saved revision 3" }),
    ).toBeEnabled(),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Execute saved revision 3" }),
  );
  await screen.findByText(/Submission outcome is unresolved/);
  expect(mocks.submitExecution).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole("button", { name: "Reconcile original submission" }),
  );
  await screen.findByText(/Execution durably accepted/);
  expect(mocks.submitExecution.mock.calls[0].slice(0, 5)).toEqual(
    mocks.submitExecution.mock.calls[1].slice(0, 5),
  );
  expect(mocks.submitExecution.mock.calls[0][1]).toBe("request-a");
  expect(mocks.submitExecution.mock.calls[0][2]).toBe('"3"');
});
it("renders unknown outcome and response text safely, and confirms a separate deliberate rerun", async () => {
  const failed: Execution = {
    ...execution,
    status: "failed",
    summary: {
      httpStatus: 200,
      durationMs: 12,
      outcome: { code: "execution_outcome_unknown", certainty: "unknown" },
    },
  };
  mocks.executionHistory.mockResolvedValue({
    items: [failed],
    nextCursor: null,
  });
  mocks.executionDetail.mockResolvedValue(failed);
  mocks.rerunExecution.mockResolvedValue({
    ...execution,
    executionId: "execution-b",
  });
  mocks.executionEvidence.mockResolvedValue({
    request: {
      method: "GET",
      url: "https://fixture.example/",
      headers: [],
      body: { kind: "none", text: null, omissionReason: null },
    },
    response: {
      httpStatus: 200,
      headers: [],
      body: {
        kind: "text",
        text: "<script>window.fixtureExecuted=true</script>",
        omissionReason: null,
      },
      wireBytesRead: 50,
      decodedBytesRead: 50,
    },
    timing: { durationMs: 12, timeToFirstByteMs: 10 },
  });
  const view = mount();
  fireEvent.click(await screen.findByRole("button", { name: /date · failed/ }));
  await screen.findByText(/Outcome unknown — do not assume no effect/);
  fireEvent.click(
    screen.getByRole("button", { name: "Inspect sanitized evidence" }),
  );
  await screen.findByText("<script>window.fixtureExecuted=true</script>");
  expect(view.container.querySelector("script")).toBeNull();
  fireEvent.click(
    screen.getByRole("button", { name: "Review deliberate rerun" }),
  );
  expect(mocks.rerunExecution).not.toHaveBeenCalled();
  fireEvent.click(
    screen.getByRole("button", { name: "Confirm deliberate rerun" }),
  );
  await waitFor(() => expect(mocks.rerunExecution).toHaveBeenCalledTimes(1));
  expect(mocks.rerunExecution.mock.calls[0].slice(0, 2)).toEqual([
    "project",
    "execution-a",
  ]);
});
