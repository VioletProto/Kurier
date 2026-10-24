import { afterEach, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { Environments } from "./Environments";
import { api, UncertainWrite, type ProjectEnvironment } from "./api";
const stored: ProjectEnvironment = {
  environmentId: "environment",
  projectId: "project",
  revision: 0,
  name: "Development",
  createdAt: "now",
  updatedAt: "now",
  variables: [
    {
      variableId: "number",
      name: "ratio",
      enabled: true,
      sensitive: false,
      value: { type: "number", decimal: "0.10000000000000001" },
    },
    {
      variableId: "protected",
      name: "token",
      enabled: true,
      sensitive: true,
      valueType: "string",
      masked: true,
      secretRef: { secretId: "saved" },
    },
  ],
};
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
async function open() {
  vi.spyOn(api, "listEnvironments").mockResolvedValue({
    items: [stored],
    nextCursor: null,
  });
  vi.spyOn(api, "getEnvironment").mockResolvedValue(structuredClone(stored));
  render(
    <Environments
      projectId="project"
      disabled={false}
      onExpired={() => {}}
      onGateChanged={() => {}}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Open environments" }));
  await screen.findByRole("button", { name: "Development · revision 0" });
  fireEvent.click(
    screen.getByRole("button", { name: "Development · revision 0" }),
  );
  await screen.findByDisplayValue("0.10000000000000001");
}
it("preserves exact decimals and protected references during unrelated edits", async () => {
  await open();
  const save = vi
    .spyOn(api, "saveEnvironment")
    .mockResolvedValue({ ...stored, revision: 1, name: "Renamed" });
  expect(screen.queryByLabelText("Variable value 2")).not.toBeInTheDocument();
  expect(screen.getByLabelText("Variable name 2")).toBeDisabled();
  fireEvent.change(screen.getByLabelText("Environment name"), {
    target: { value: "Renamed" },
  });
  fireEvent.click(
    screen.getByRole("button", { name: "Save environment changes" }),
  );
  await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
  const draft = save.mock.calls[0][1];
  expect(draft.variables[0].value).toEqual({
    type: "number",
    decimal: "0.10000000000000001",
  });
  expect(draft.variables[1].secretWrite).toEqual({
    action: "preserve",
    secretRef: { secretId: "saved" },
  });
  expect(draft.variables[1]).not.toHaveProperty("value");
  await screen.findByDisplayValue("0.10000000000000001");
});
it("clears write-only protected drafts before an uncertain write and requires reconciliation", async () => {
  const persist = vi.fn();
  vi.stubGlobal("localStorage", { length: 0, setItem: persist });
  vi.stubGlobal("sessionStorage", { length: 0, setItem: persist });
  await open();
  let reject!: (reason: unknown) => void;
  vi.spyOn(api, "saveEnvironment").mockImplementation(
    () =>
      new Promise((_, r) => {
        reject = r;
      }),
  );
  fireEvent.change(screen.getByLabelText("Protected action 2"), {
    target: { value: "set" },
  });
  const input = screen.getByLabelText("Variable value 2");
  expect(input).toHaveAttribute("type", "password");
  fireEvent.change(input, {
    target: { value: "synthetic-browser-environment-canary" },
  });
  fireEvent.click(
    screen.getByRole("button", { name: "Save environment changes" }),
  );
  await waitFor(() =>
    expect(screen.queryByLabelText("Variable value 2")).not.toBeInTheDocument(),
  );
  reject(new UncertainWrite());
  await screen.findByText(/The write result is uncertain/);
  expect(
    screen.getByRole("button", { name: "Save environment changes" }),
  ).toBeDisabled();
  expect(document.body.textContent).not.toContain(
    "synthetic-browser-environment-canary",
  );
  expect(localStorage.length).toBe(0);
  expect(sessionStorage.length).toBe(0);
  expect(persist).not.toHaveBeenCalled();
});
