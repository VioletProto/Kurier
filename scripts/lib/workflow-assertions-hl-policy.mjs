export const actions = Object.freeze([
  "preflight.mjs",
  "init.mjs",
  "helpers.mjs",
  "campaign-decimal.mjs",
  "cleanup.mjs",
  "stopped-cleanup.mjs",
]);
export function checkAdmission(ledger, key, workflow, options = {}) {
  const cases = [
    ["H", "completed", 2],
    ["I", "completed", 2],
    ["J", "failed", 1],
    ["K", "failed", 2],
    ["L", "failed", 1],
  ];
  const index = ledger.runs.length,
    expected = cases[index];
  if (
    !ledger.preflightVerified ||
    ledger.users.length !== 1 ||
    ledger.projects.length !== 1 ||
    !expected ||
    key !== expected[0] ||
    (options.state ?? "completed") !== expected[1] ||
    (options.sends ?? 2) !== expected[2] ||
    workflow.steps.length !== 2 ||
    ledger.admittedSteps !== 18 + 2 * index ||
    ledger.admittedSteps + 2 > 28 ||
    ledger.expectedSends + (options.sends ?? 2) > 24 ||
    Boolean(options.source) !== (key === "I")
  )
    throw Error("H-L admission guard; no replay or reserve");
  return true;
}
