// Fixed recovery executor. Host and wire gates independently bind the same request.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { main as inspectRecovery, assess } from "./recovery.mjs";
import {
  aws,
  marker,
  inventory,
  hash,
  queueDepth,
  frontendReady,
} from "./collector.mjs";
import { inspectRows } from "../lib/project-environments-activation.mjs";
export const FIXED = {
  proposal: "307a987a15b4e372b61bda5df4fbe12bba6b270f4a4c207470461d4286e01251",
  transaction:
    "4bb06443f6c2d3eb578a2c15f7ced1ea1b2b7bd8ea003864d47e97610dec3f1f",
  before: "353cd6632e9351399bf4e62d853f500ec02e4fa2b5ba1fc7148b37965818a708",
  after: "c89cdb1316d80dc2d497ff13a13080e3a55ad0fa85cc4f6b861781aad5489e95",
};
export function validateFixed(context) {
  const p = context.fixedProposal;
  if (
    hash(p) !== FIXED.proposal ||
    hash(p.transaction) !== FIXED.transaction ||
    hash(p.before) !== FIXED.before ||
    hash(p.after) !== FIXED.after ||
    hash(context.expectedMarker) !== FIXED.before ||
    context.accountingSha256 !== p.accountingSha256
  )
    throw Error("Fixed recovery binding drift.");
  return p;
}
export function verifySnapshot(bundle, context) {
  validateFixed(context);
  assess(bundle, context);
  return {
    markerSha256: hash(bundle.marker),
    accountingSha256: inspectRows(bundle.rows).accountingSha256,
    oldReleaseVerified: true,
  };
}
const live = {
  inspect: (file) => inspectRecovery("inspect", file),
  marker,
  rows: inventory,
  depth: queueDepth,
  frontend: () => frontendReady("collect"),
  transact: (p) => aws("dynamodb", "transact-write-items", p),
};
export function execute(file, context, adapter = live) {
  const fixed = validateFixed(context);
  const fresh = adapter.inspect(file);
  verifySnapshot(fresh, context);
  const lastMarker = adapter.marker(),
    lastRows = inspectRows(adapter.rows()),
    depth = adapter.depth();
  if (
    hash(lastMarker) !== FIXED.before ||
    lastRows.accountingSha256 !== fixed.accountingSha256 ||
    Object.keys(depth).length !== 3 ||
    Object.values(depth).some((v) => v !== "0") ||
    !adapter.frontend()
  )
    throw Error("Immediate prewrite drift.");
  // Exactly the accepted request; no plan regeneration or catch/retry path.
  adapter.transact(fixed.transaction);
  const after = adapter.marker(),
    accounting = inspectRows(adapter.rows());
  if (
    hash(after) !== FIXED.after ||
    accounting.accountingSha256 !== fixed.accountingSha256
  )
    throw Error("Recovery readback uncertain/mismatch; no retry.");
  return {
    state: after.state.S,
    version: after.version.N,
    marker: after,
    accountingSha256: accounting.accountingSha256,
    transactionSha256: FIXED.transaction,
    stageWrites: 1,
    oldReleaseReopened: true,
    capabilitiesAndGenerationPreserved: true,
    accountingPreserved: true,
  };
}
export function main(action, file) {
  const c = JSON.parse(readFileSync(file));
  validateFixed(c);
  if (action === "before") return inspectRecovery("inspect", file);
  if (action === "assess") return verifySnapshot(c.bundle, c);
  if (action === "execute") return execute(file, c);
  throw Error("Unknown recovery write action.");
}
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    console.log(JSON.stringify(main(...process.argv.slice(2))));
  } catch {
    console.error(
      "Recovery execution held/uncertain. No retry, substitution, repair or reopen replay.",
    );
    process.exitCode = 1;
  }
}
