import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { UncertainWrite, type Execution, type RequestDetail } from "./api";
const mocks = vi.hoisted(() => ({
  executionHistory: vi.fn(),
  submitExecution: vi.fn(),
  executionDetail: vi.fn(),
  executionStatus: vi.fn(),
  executionEvidence: vi.fn(),
  rerunExecution: vi.fn(),
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
