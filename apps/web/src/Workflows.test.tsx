import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
const mocks = vi.hoisted(() => ({
  workflows: vi.fn(),
  workflowRuns: vi.fn(),
  listRequests: vi.fn(),
  saveWorkflow: vi.fn(),
  deleteWorkflow: vi.fn(),
  submitWorkflow: vi.fn(),
  workflowRun: vi.fn(),
  executionEvidence: vi.fn(),
  pinExecution: vi.fn(),
}));
vi.mock("./api", async () => ({
  ...(await vi.importActual<typeof import("./api")>("./api")),
  api: mocks,
}));
import { Workflows } from "./Workflows";
import { AssertionResults } from "./Assertions";
import type { Evidence } from "./api";
import { UncertainWrite } from "./api";
const request = {
  requestId: "r",
  revision: 0,
  name: "Login",
  headers: [
    { name: "Authorization", value: "", enabled: true, sensitive: false },
  ],
  queryParameters: [],
};
const first = {
  stepId: "s1",
  requestId: "r",
  requestRevision: 0,
  extract: [
    { name: "token", path: "$.token", type: "string", sensitive: false },
  ],
  bindings: [],
};
const second = {
  stepId: "s2",
  requestId: "r",
  requestRevision: 0,
  extract: [],
  bindings: [
    {
      from: { stepId: "s1", name: "token" },
      to: { kind: "header", index: 0, prefix: "Bearer ", suffix: "" },
    },
  ],
};
const workflow = {
  workflowId: "w",
  revision: 2,
  name: "Login flow",
  steps: [first, second],
};
beforeEach(() => {
  vi.resetAllMocks();
  mocks.workflows.mockResolvedValue({ items: [workflow], nextCursor: null });
  mocks.workflowRuns.mockResolvedValue({ items: [], nextCursor: null });
  mocks.listRequests.mockResolvedValue({ items: [request], nextCursor: null });
});
afterEach(cleanup);
async function open() {
  render(
    <Workflows
      projectId="p"
      onExpired={vi.fn()}
      onGateChanged={vi.fn()}
      disabled={false}
    />,
  );
  fireEvent.click(screen.getByText("Load workflows"));
  await screen.findByText("Login flow (revision 2)");
  fireEvent.click(screen.getByText("Login flow (revision 2)"));
}
it("edits literal header rendering and prevents a forward reference after reorder", async () => {
  await open();
  expect(screen.getByLabelText("Public literal prefix")).toHaveValue("Bearer ");
  fireEvent.change(screen.getByLabelText("Public literal suffix"), {
    target: { value: "literal" },
  });
  mocks.saveWorkflow.mockResolvedValue(workflow);
  fireEvent.click(screen.getByText("Save workflow"));
  await waitFor(() => expect(mocks.saveWorkflow).toHaveBeenCalled());
  const draft = mocks.saveWorkflow.mock.calls[0][1];
  expect(draft.steps[1].bindings[0].to.suffix).toBe("literal");
  await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  const step = screen.getByRole("group", { name: "Step 2" });
  fireEvent.click(within(step).getByText("Move up"));
  expect(screen.getByText("Save workflow")).toBeDisabled();
});
it("reconciles an uncertain submission with the same key and frozen consent", async () => {
  await open();
  mocks.submitWorkflow.mockRejectedValueOnce(new UncertainWrite());
  fireEvent.click(screen.getByText("Submit saved workflow"));
  await screen.findByRole("alert");
  const firstCall = mocks.submitWorkflow.mock.calls[0];
  mocks.submitWorkflow.mockResolvedValue({
    workflowRunId: "run",
    state: "completed",
    steps: [],
  });
  fireEvent.click(screen.getByText("Reconcile original submission"));
  await waitFor(() => expect(mocks.submitWorkflow).toHaveBeenCalledTimes(2));
  expect(mocks.submitWorkflow.mock.calls[1].slice(0, 5)).toEqual(
    firstCall.slice(0, 5),
  );
});
it("shows unknown-outcome warning and requires deliberate entire-run rerun", async () => {
  const run = {
    workflowRunId: "run",
    state: "failed",
    steps: [
      {
        ...first,
        position: 0,
        state: "failed",
        executionId: "e",
        summary: { outcome: { certainty: "unknown" } },
        retention: null,
        evidenceExpired: false,
      },
    ],
  };
  mocks.workflowRuns.mockResolvedValue({ items: [run], nextCursor: null });
  mocks.workflowRun.mockResolvedValue(run);
  await open();
  fireEvent.click(screen.getByText("run: failed"));
  await screen.findByText(/Outcome unknown/);
  expect(screen.queryByText("disposable-runtime-value")).toBeNull();
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  fireEvent.click(screen.getByText("Rerun entire workflow"));
  expect(confirm).toHaveBeenCalled();
  expect(mocks.submitWorkflow).not.toHaveBeenCalled();
  confirm.mockRestore();
});

it("authors exact decimal expectations, reopens them and preserves unrelated edits", async () => {
  await open();
  const firstStep = screen.getByRole("group", { name: "Step 1" });
  fireEvent.click(within(firstStep).getByText("Add assertion"));
  fireEvent.change(within(firstStep).getByLabelText("Assertion kind"), {
    target: { value: "jsonEquals" },
  });
  fireEvent.change(within(firstStep).getByLabelText("Expected type"), {
    target: { value: "number" },
  });
  expect(screen.getByText("Save workflow")).toBeDisabled();
  fireEvent.change(within(firstStep).getByLabelText("Assertion JSON path"), {
    target: { value: "$.ratio" },
  });
  fireEvent.change(within(firstStep).getByLabelText("Expected decimal"), {
    target: { value: "0.10000000000000001" },
  });
  mocks.saveWorkflow.mockImplementation(async (_project, draft) => ({
    ...workflow,
    ...JSON.parse(JSON.stringify(draft)),
  }));
  fireEvent.click(screen.getByText("Save workflow"));
  await waitFor(() => expect(mocks.saveWorkflow).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  expect(screen.getByLabelText("Expected decimal")).toHaveValue(
    "0.10000000000000001",
  );
  fireEvent.change(screen.getByLabelText("Workflow name"), {
    target: { value: "Unrelated rename" },
  });
  fireEvent.click(screen.getByText("Save workflow"));
  await waitFor(() => expect(mocks.saveWorkflow).toHaveBeenCalledTimes(2));
  const oldRule = mocks.saveWorkflow.mock.calls[0][1].steps[0].assertions[0];
  expect(mocks.saveWorkflow.mock.calls[1][1].steps[0].assertions[0]).toEqual(
    oldRule,
  );
  expect(oldRule.expected).toEqual({
    type: "number",
    decimal: "0.10000000000000001",
  });
  await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  fireEvent.click(screen.getByText("Remove assertion"));
  expect(screen.queryByLabelText("Expected decimal")).toBeNull();
});

it("reorders numeric checks without changing IDs or authored tokens", async () => {
  const assertions = ["0.10000000000000001", "1E+00"].map((decimal, i) => ({
    assertionId: `check-${i}`,
    kind: "jsonEquals",
    path: `$.ratio${i}`,
    expected: { type: "number", decimal },
  }));
  mocks.workflows.mockResolvedValue({
    items: [{ ...workflow, steps: [{ ...first, assertions }, second] }],
    nextCursor: null,
  });
  await open();
  const check = screen.getByRole("group", { name: "Assertion 1" });
  fireEvent.click(within(check).getByText("Move assertion down"));
  mocks.saveWorkflow.mockResolvedValue(workflow);
  fireEvent.click(screen.getByText("Save workflow"));
  await waitFor(() => expect(mocks.saveWorkflow).toHaveBeenCalled());
  expect(mocks.saveWorkflow.mock.calls[0][1].steps[0].assertions).toEqual([
    assertions[1],
    assertions[0],
  ]);
});

it("distinguishes legacy unavailable results, empty configuration and recorded failure", () => {
  const { rerender } = render(
    <AssertionResults evidence={{ schemaVersion: 1 } as Evidence} />,
  );
  expect(screen.getByText("Assertion results unavailable.")).toBeVisible();
  rerender(
    <AssertionResults
      evidence={
        {
          schemaVersion: 2,
          assertions: {
            schemaVersion: 1,
            state: "not_configured",
            results: [],
          },
        } as unknown as Evidence
      }
    />,
  );
  expect(screen.getByText("No assertions configured.")).toBeVisible();
  rerender(
    <AssertionResults
      evidence={
        {
          schemaVersion: 2,
          assertions: {
            schemaVersion: 1,
            state: "evaluated",
            results: [
              {
                assertionId: "check",
                kind: "jsonEquals",
                outcome: "failed",
                reason: "value_mismatch",
              },
            ],
          },
        } as unknown as Evidence
      }
    />,
  );
  expect(
    screen.getByRole("region", { name: "Assertion results" }),
  ).toHaveTextContent("failed (value_mismatch)");
});
