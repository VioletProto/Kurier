import { expect, it } from "vitest";
import { containsSecretText } from "./secret-persistence-audit";

it("detects disposable secrets in escaped JSON and scheme-stripped responses", () => {
  const token = crypto.randomUUID();
  const scalar = crypto.randomUUID();
  const values = ["Bearer " + token, JSON.stringify(scalar)];
  for (const response of [
    { credential: values[0] },
    { credential: token },
    { credential: values[1] },
    { credential: scalar },
  ])
    expect(containsSecretText(JSON.stringify(response), values)).toBe(true);
  expect(
    containsSecretText(
      JSON.stringify({ secretId: crypto.randomUUID(), masked: true }),
      values,
    ),
  ).toBe(false);
});
