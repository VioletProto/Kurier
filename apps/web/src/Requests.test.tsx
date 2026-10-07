import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiError, UncertainWrite, type SavedRequest } from "./api";
const mocks = vi.hoisted(() => ({
  listRequests: vi.fn(),
  requestDetail: vi.fn(),
  createRequest: vi.fn(),
  patchRequest: vi.fn(),
  deleteRequest: vi.fn(),
}));
vi.mock("./api", async () => ({
  ...(await vi.importActual<typeof import("./api")>("./api")),
  api: mocks,
}));
import { Requests } from "./Requests";
const saved: SavedRequest = {
  requestId: "request",
  projectId: "project",
  revision: 0,
  name: "Public",
  method: "GET",
  url: "https://example.com",
  queryParameters: [],
  headers: [],
  body: null,
  operationRef: null,
  createdAt: "2026-10-07T00:00:00Z",
  updatedAt: "2026-10-07T00:00:00Z",
};
beforeEach(() => {
  vi.clearAllMocks();
  mocks.listRequests.mockResolvedValue({ items: [saved], nextCursor: null });
  mocks.requestDetail.mockResolvedValue({ request: saved, etag: '"0"' });
});
afterEach(cleanup);
const gate = vi.fn();
function mount() {
  render(
    <Requests
      projectId="project"
      disabled={false}
      onExpired={vi.fn()}
      onGateChanged={gate}
    />,
  );
}
async function edit() {
  await screen.findByRole("button", { name: "GET Public" });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "GET Public" })).toBeEnabled(),
  );
  fireEvent.click(screen.getByRole("button", { name: "GET Public" }));
  await screen.findByLabelText("Request name");
}
function confirm() {
  fireEvent.click(
    screen.getByRole("checkbox", { name: /All fields contain public data/ }),
  );
}
it("explains whole configuration limit and rejects credential headers before saving", async () => {
  mount();
  await edit();
  expect(
    screen.getByText(/64 KiB applies to the complete saved configuration/),
  ).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Add header" }));
  fireEvent.change(screen.getByLabelText("Header name 1"), {
    target: { value: "Authorization" },
  });
  fireEvent.change(screen.getByLabelText("Header value 1"), {
    target: { value: "fixture" },
  });
  confirm();
  fireEvent.click(screen.getByRole("button", { name: "Save request changes" }));
  await screen.findByText(/Use named public headers/);
  expect(mocks.patchRequest).not.toHaveBeenCalled();
});
it("keeps stale drafts until explicit refresh and does not retry", async () => {
  mocks.patchRequest.mockRejectedValue(
    new ApiError(412, "precondition_failed"),
  );
  mount();
  await edit();
  fireEvent.change(screen.getByLabelText("Request name"), {
    target: { value: "My draft" },
  });
  confirm();
  fireEvent.click(screen.getByRole("button", { name: "Save request changes" }));
  await screen.findByText(/saved version or write outcome changed/);
  expect(screen.getByLabelText("Request name")).toHaveValue("My draft");
  expect(
    screen.getByRole("button", { name: "Save request changes" }),
  ).toBeDisabled();
  expect(mocks.patchRequest).toHaveBeenCalledTimes(1);
  fireEvent.click(
    screen.getByRole("button", { name: "Refresh request details" }),
  );
  await waitFor(() =>
    expect(screen.getByLabelText("Request name")).toHaveValue("Public"),
  );
  expect(mocks.patchRequest).toHaveBeenCalledTimes(1);
});
it("holds uncertain creation until deliberate list review", async () => {
  mocks.createRequest.mockRejectedValue(new UncertainWrite());
  mount();
  await screen.findByRole("button", { name: "GET Public" });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "New request" })).toBeEnabled(),
  );
  fireEvent.click(screen.getByRole("button", { name: "New request" }));
  fireEvent.change(screen.getByLabelText("Request name"), {
    target: { value: "New" },
  });
  fireEvent.change(screen.getByLabelText("Public request URL"), {
    target: { value: "https://example.com/public" },
  });
  confirm();
  fireEvent.click(screen.getByRole("button", { name: "Create request" }));
  await screen.findByText(/Creation may have succeeded/);
  expect(screen.getByRole("button", { name: "Create request" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Refresh requests" }));
  await waitFor(() => expect(mocks.listRequests).toHaveBeenCalledTimes(2));
  expect(screen.getByRole("button", { name: "Create request" })).toBeDisabled();
  expect(mocks.createRequest).toHaveBeenCalledTimes(1);
});
it("clears body and headers in the saved configuration", async () => {
  mocks.requestDetail.mockResolvedValue({
    request: {
      ...saved,
      headers: [
        {
          name: "Accept",
          value: "application/json",
          enabled: true,
          sensitive: false,
        },
      ],
      body: { type: "text", text: "public", sensitive: false },
    },
    etag: '"0"',
  });
  mocks.patchRequest.mockResolvedValue({
    request: { ...saved, revision: 1 },
    etag: '"1"',
  });
  mount();
  await edit();
  fireEvent.click(screen.getByRole("button", { name: "Remove header 1" }));
  fireEvent.change(screen.getByLabelText("Body type"), {
    target: { value: "none" },
  });
  confirm();
  fireEvent.click(screen.getByRole("button", { name: "Save request changes" }));
  await waitFor(() =>
    expect(mocks.patchRequest).toHaveBeenCalledWith(
      "project",
      "request",
      expect.objectContaining({ headers: [], body: null }),
      '"0"',
    ),
  );
  expect(gate).toHaveBeenCalled();
});
