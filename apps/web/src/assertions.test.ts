import { expect, it } from "vitest";
import {
  validAssertion,
  validAssertionDecimal,
  validStepAssertions,
} from "./assertions";
import type { Assertion, WorkflowStep } from "./api";

it("preserves decimal tokens through native JSON and editor clones without a numeric intermediary", () => {
  for (const token of [
    "0.10000000000000001",
    "0.1",
    "1",
    "1.0",
    "1e0",
    "1E+00",
    "10e-1",
    "1e-000",
    "0",
    "-0",
    "-0.0",
    "0E+308",
    "9007199254740991",
    "-9007199254740991",
    "9007199254740990.999999999999999999",
    "1e-308",
    "-1E-308",
    "10e-308",
    `1.${"0".repeat(126)}`,
  ]) {
    const expected = { type: "number" as const, decimal: token };
    const rule: Assertion = {
      assertionId: crypto.randomUUID(),
      kind: "jsonEquals",
      path: "$.ratio",
      expected,
    };
    expect(validAssertion(rule)).toBe(true);
    const read = JSON.parse(JSON.stringify(rule));
    expect(structuredClone(read).expected).toEqual(expected);
  }
  expect(JSON.parse("0.10000000000000001")).toBe(JSON.parse("0.1"));
  expect(
    JSON.parse(JSON.stringify({ decimal: "0.10000000000000001" })).decimal,
  ).not.toBe("0.1");
});
it("rejects unsafe, malformed, localized, oversized or unsupported decimal expectations", () => {
  for (const token of [
    "",
    " 1",
    "1 ",
    "+1",
    "01",
    ".1",
    "1.",
    "NaN",
    "Infinity",
    "0,1",
    "١",
    "9007199254740992",
    "-9007199254740992",
    "9007199254740991.1",
    "1e-309",
    "1e309",
    "1e308",
    "0e309",
    "1e-999999999999999999",
    `1.${"0".repeat(127)}`,
  ]) {
    expect(validAssertionDecimal(token), token).toBe(false);
  }
  for (const expected of [
    0.1,
    1,
    { type: "number", decimal: 0.1 },
    { type: "number", decimal: null },
    { type: "number", decimal: "1", extra: true },
    [],
  ]) {
    expect(
      validAssertion({
        assertionId: "a",
        kind: "jsonEquals",
        path: "$",
        expected,
      } as Assertion),
    ).toBe(false);
  }
});
it("enforces aggregate array bounds, status uniqueness and public primitive types", () => {
  expect(
    validAssertion({
      assertionId: "html",
      kind: "jsonEquals",
      path: "$",
      expected: "<".repeat(341),
    }),
  ).toBe(true);
  expect(
    validAssertion({
      assertionId: "html",
      kind: "jsonEquals",
      path: "$",
      expected: "<".repeat(342),
    }),
  ).toBe(false);
  const step: WorkflowStep = {
    stepId: "s",
    requestId: "r",
    requestRevision: 0,
    extract: [],
    bindings: [],
    assertions: [],
  };
  for (const expected of [null, true, false, "", "0.1"])
    expect(
      validAssertion({
        assertionId: "a",
        kind: "jsonEquals",
        path: "$",
        expected,
      }),
    ).toBe(true);
  step.assertions = [0, 1].map((i) => ({
    assertionId: String(i),
    kind: "statusEquals",
    expected: 200,
  }));
  expect(validStepAssertions(step)).toBe(false);
  step.assertions = Array.from({ length: 8 }, (_, i) => ({
    assertionId: String(i),
    kind: "jsonEquals",
    path: "$",
    expected: "x".repeat(1000),
  }));
  expect(validStepAssertions(step)).toBe(false);
});
