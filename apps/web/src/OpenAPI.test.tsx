import { afterEach, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { OpenAPI } from "./OpenAPI";
import { Api, api, UncertainWrite } from "./api";
import { choicesWithServer, type OpenAPIImport } from "./openapi";

const prepared: OpenAPIImport = {
  importId: "import",
  projectId: "project",
  revision: 0,
  state: "preparing",
  operationCount: 1,
  createdAt: "now",
  updatedAt: "now",
  diagnostics: [],
  operations: [
    {
      operation: {
        operationId: "operation",
        method: "GET",
        path: "/items/{id}",
        servers: ["https://example.com", "https://example.org"],
        parameters: [
          {
            inputId: "p0",
            name: "id",
            in: "path",
            required: true,
            sensitive: false,
            schema: { type: "number" },
          },
        ],
      },
      choicesJSON: '{"parameters":{"p0":0.10000000000000001}}',
      requestId: "request",
      ready: false,
    },
  ],
};
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
async function open() {
  vi.spyOn(api, "listOpenAPIImports").mockResolvedValue({
    items: [prepared],
    nextCursor: null,
  });
  vi.spyOn(api, "openAPIImport").mockResolvedValue(prepared);
  render(
    <OpenAPI
      projectId="project"
      disabled={false}
      onExpired={vi.fn()}
      onGateChanged={vi.fn()}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Open OpenAPI imports" }));
  const item = await screen.findByRole("button", { name: /1 operations/ });
  fireEvent.click(item);
  await screen.findByLabelText("Operation inputs (JSON)");
}
it("preserves exact numeric lexemes when choosing servers and completing a candidate", async () => {
  const saved = vi.spyOn(api, "saveOpenAPIChoices").mockResolvedValue({
    ...prepared,
    revision: 1,
    operations: [{ ...prepared.operations[0], ready: true }],
  });
  await open();
  expect(
    screen.getByRole("button", { name: "Publish generated requests" }),
  ).toBeDisabled();
  fireEvent.change(screen.getByLabelText("Server for GET /items/{id}"), {
    target: { value: "https://example.org" },
  });
  fireEvent.click(
    screen.getByRole("checkbox", { name: /These operation inputs/ }),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Save operation inputs" }),
  );
  await waitFor(() => expect(saved).toHaveBeenCalledTimes(1));
  expect(saved.mock.calls[0][3]).toBe(
    '{"server":"https://example.org","parameters":{"p0":0.10000000000000001}}',
  );
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Publish generated requests" }),
    ).toBeEnabled(),
  );
});
it("clears raw source and sensitive designations before an uncertain import and requires readback", async () => {
  let reject!: (error: Error) => void;
  const create = vi.spyOn(api, "createOpenAPIImport").mockImplementation(
    () =>
      new Promise((_, r) => {
        reject = r;
      }),
  );
  await open();
  fireEvent.change(screen.getByLabelText("OpenAPI source"), {
    target: { value: "synthetic-private-upload-canary" },
  });
  fireEvent.change(screen.getByLabelText("Additional sensitive value 1"), {
    target: { value: "synthetic-private-upload-canary" },
  });
  fireEvent.click(
    screen.getByRole("checkbox", { name: /I confirm the remaining/ }),
  );
  fireEvent.click(screen.getByRole("button", { name: "Prepare import" }));
  await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
  expect(screen.getByLabelText("OpenAPI source")).toHaveValue("");
  expect(screen.getByLabelText("Additional sensitive value 1")).toHaveValue("");
  reject(new UncertainWrite());
  await screen.findByRole("alert");
  expect(
    screen.getByRole("button", { name: "Save operation inputs" }),
  ).toBeDisabled();
  expect(create).toHaveBeenCalledTimes(1);
});
it("does not reserialize raw numeric inputs in the API payload or retry publication", async () => {
  const transport = vi
    .fn()
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { import: prepared } })),
    )
    .mockRejectedValueOnce(new TypeError("lost ack"));
  const client = new Api(async () => "fixture-token", transport);
  await client.saveOpenAPIChoices(
    "project",
    prepared,
    "operation",
    '{"parameters":{"p0":9007199254740993}}',
  );
  expect(transport.mock.calls[0][1].body).toContain('"p0":9007199254740993');
  expect(transport.mock.calls[0][1].headers["If-Match"]).toBe('"0"');
  await expect(
    client.publishOpenAPIImport("project", prepared),
  ).rejects.toBeInstanceOf(UncertainWrite);
  expect(transport).toHaveBeenCalledTimes(2);
});
it("edits only the root server value and keeps nested strings and exact decimals unchanged", () => {
  const text = String.raw`{"body":{"server":"nested", "decimal":1.0000000000000001},"server":"old","parameters":{"p0":"a,\"b"}}`;
  expect(choicesWithServer(text, "https://example.com")).toBe(
    text.replace('"server":"old"', '"server":"https://example.com"'),
  );
  expect(() => choicesWithServer("[]", "https://example.com")).toThrow();
});
it("waits for all candidates and loads paged previews without losing prior operations", async () => {
  const first = {
    ...prepared,
    operationCount: 2,
    readyOperationCount: 1,
    nextOperationOffset: 1,
    operations: [{ ...prepared.operations[0], ready: true }],
  };
  vi.spyOn(api, "listOpenAPIImports").mockResolvedValue({
    items: [first],
    nextCursor: null,
  });
  const load = vi
    .spyOn(api, "openAPIImport")
    .mockResolvedValueOnce(first)
    .mockResolvedValueOnce({
      ...first,
      nextOperationOffset: null,
      operations: [
        {
          ...prepared.operations[0],
          operation: {
            ...prepared.operations[0].operation,
            operationId: "second",
            path: "/other",
          },
        },
      ],
    });
  render(
    <OpenAPI
      projectId="project"
      disabled={false}
      onExpired={vi.fn()}
      onGateChanged={vi.fn()}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Open OpenAPI imports" }));
  fireEvent.click(await screen.findByRole("button", { name: /2 operations/ }));
  expect(
    await screen.findByRole("button", { name: "Publish generated requests" }),
  ).toBeDisabled();
  fireEvent.click(
    screen.getByRole("button", { name: "More operation previews" }),
  );
  await screen.findByLabelText("Server for GET /other");
  expect(screen.getAllByLabelText("Operation inputs (JSON)")).toHaveLength(2);
  expect(load.mock.calls[1]).toEqual(["project", "import", 1]);
});
