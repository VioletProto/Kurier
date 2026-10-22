import type { Assertion, AssertionEvaluation, WorkflowStep } from "./api";

// Count the server's compact encoding, including encoding/json HTML/line escapes.
function serializedBytes(value: unknown): number {
  const text = JSON.stringify(value) ?? "";
  return new TextEncoder().encode(
    text.replace(
      /[<>&\u2028\u2029]/g,
      (c) => `\\u${c.charCodeAt(0).toString(16).padStart(4, "0")}`,
    ),
  ).length;
}

// Exact decimal validation uses strings/BigInt; a browser number never carries
// the expectation, even temporarily. Stored tokens are never normalized.
export function validAssertionDecimal(token: string): boolean {
  if (
    token.length === 0 ||
    token.length > 128 ||
    !/^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$/.test(token)
  )
    return false;
  const [mantissa, exponentToken = "0"] = token.split(/[eE]/);
  const exponent = BigInt(exponentToken);
  if (exponent < -308n || exponent > 308n) return false;
  const [whole, fraction = ""] = mantissa.replace(/^-/, "").split(".");
  const coefficient = BigInt(whole + fraction);
  const scale = exponent - BigInt(fraction.length);
  const numerator = scale >= 0n ? coefficient * 10n ** scale : coefficient;
  const denominator = scale < 0n ? 10n ** -scale : 1n;
  return (
    numerator <= 9007199254740991n * denominator &&
    (numerator === 0n || numerator * 10n ** 308n >= denominator)
  );
}

export function validAssertion(a: Assertion): boolean {
  if (!a || typeof a.assertionId !== "string") return false;
  if (a.kind === "statusEquals")
    return (
      Number.isInteger(a.expected) && a.expected >= 100 && a.expected <= 599
    );
  if (a.kind !== "jsonExists" && a.kind !== "jsonEquals") return false;
  if (
    typeof a.path !== "string" ||
    !a.path.startsWith("$") ||
    new TextEncoder().encode(a.path).length > 512
  )
    return false;
  if (a.kind === "jsonExists") return !("expected" in a);
  const e = a.expected;
  if (serializedBytes(e) > 2048) return false;
  return (
    e === null ||
    typeof e === "boolean" ||
    typeof e === "string" ||
    (typeof e === "object" &&
      e !== null &&
      e.type === "number" &&
      typeof e.decimal === "string" &&
      Object.keys(e).length === 2 &&
      validAssertionDecimal(e.decimal))
  );
}

export function validStepAssertions(step: WorkflowStep): boolean {
  const checks = step.assertions ?? [];
  return (
    Array.isArray(checks) &&
    checks.length <= 8 &&
    checks.filter((a) => a.kind === "statusEquals").length <= 1 &&
    new Set(checks.map((a) => a.assertionId)).size === checks.length &&
    checks.every(validAssertion) &&
    serializedBytes(checks) <= 8192
  );
}

export function validAssertionEvaluation(a: AssertionEvaluation): boolean {
  return (
    !!a &&
    a.schemaVersion === 1 &&
    Array.isArray(a.results) &&
    a.results.length <= 8 &&
    (a.state === "not_configured"
      ? a.results.length === 0
      : a.state === "evaluated" && a.results.length > 0) &&
    new Set(a.results.map((r) => r.assertionId)).size === a.results.length &&
    a.results.every(
      (r) =>
        typeof r.assertionId === "string" &&
        ["statusEquals", "jsonEquals", "jsonExists"].includes(r.kind) &&
        ["passed", "failed", "not_evaluated"].includes(r.outcome) &&
        [
          "matched",
          "exists",
          "status_mismatch",
          "path_missing",
          "type_mismatch",
          "value_mismatch",
          "not_dispatched",
          "status_unavailable",
          "response_unavailable",
          "body_absent",
          "body_unsupported",
          "body_limit_exceeded",
          "json_invalid",
          "json_number_unsupported",
          "sensitive_input",
          "evaluation_unavailable",
        ].includes(r.reason),
    )
  );
}
