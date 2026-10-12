import { expect, it } from "vitest";
import { pinningInventory } from "../scripts/evidence-pinning-inventory.mjs";
const retention = {
  PK: { S: "P#fixture" },
  SK: { S: "EXEC#fixture#RET" },
  kind: { S: "executionRetention" },
  schemaVersion: { N: "1" },
  version: { N: "0" },
  state: { S: "live" },
  normalExpiresAt: { S: "2026-11-01T00:00:00Z" },
};
it("refuses pins, unknown retention, unresolved jobs and missing storage accounting before zero initialization", () => {
  expect(() =>
    pinningInventory([{ ...retention, pinned: { BOOL: true } }]),
  ).toThrow("Existing pins");
  expect(() =>
    pinningInventory([{ ...retention, future: { S: "unknown" } }]),
  ).toThrow("Unknown retention");
  expect(() =>
    pinningInventory([
      { kind: { S: "executionJob" }, state: { S: "running" } },
    ]),
  ).toThrow("Unfinished jobs");
  expect(() =>
    pinningInventory([
      { kind: { S: "executionUpload" }, state: { S: "published" } },
    ]),
  ).toThrow("Missing storage quota");
  expect(() =>
    pinningInventory([{ ...retention, pinnedBytes: { N: "99" } }]),
  ).toThrow("Unknown retention");
});
it("retains the exact project metadata for conditional initialization and never supplies payloads to summaries", () => {
  expect(() =>
    pinningInventory([{ ...retention, version: { N: "NaN" } }]),
  ).toThrow("Invalid metadata version");
  expect(() =>
    pinningInventory([{ ...retention, pinned: { S: "true" } }]),
  ).toThrow("Unknown pin metadata");
  const project = {
    PK: { S: "P#fixture" },
    SK: { S: "META" },
    kind: { S: "project" },
    schemaVersion: { N: "1" },
    version: { N: "4" },
    state: { S: "active" },
    pinnedCount: { N: "0" },
    pinnedBytes: { N: "0" },
  };
  const inventory = pinningInventory([retention, project]);
  expect(inventory.projects).toEqual([project]);
  expect(inventory.quota).toBeUndefined();
  expect(() =>
    pinningInventory([{ ...project, pinnedCount: { N: "1" } }]),
  ).toThrow("Nonzero");
});
