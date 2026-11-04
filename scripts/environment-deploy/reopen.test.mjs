// Retained private evidence is read only; failed assertions never print its contents.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { execute, validateFixed, verifySnapshot } from "./reopen.mjs";
import { hash } from "./collector.mjs";
const base = new URL(
  "../../.build/project-environments-execution/recovery-inspect-868-r1/",
  import.meta.url,
);
const read = (name) => JSON.parse(readFileSync(new URL(name, base)));
function fixture() {
  const p = read("proposed-reopen.json"),
    proof = read("inspection-proof.json");
  const context = { ...proof.originalParent, fixedProposal: p };
  const bundle = proof.bundle;
  let writes = 0,
    markers = 0;
  const adapter = {
    inspect: () => bundle,
    marker: () => structuredClone(markers++ ? p.after : p.before),
    rows: () => bundle.rows,
    depth: () => bundle.depth,
    frontend: () => true,
    transact: (request) => {
      assert.equal(hash(request), hash(p.transaction));
      writes++;
    },
  };
  return { p, context, bundle, adapter, writes: () => writes };
}
test("actual executor sends the exact accepted transaction once and verifies active/17", () => {
  const f = fixture(),
    result = execute("synthetic", f.context, f.adapter);
  assert.equal(result.version, "17");
  assert.equal(result.state, "active");
  assert.equal(f.writes(), 1);
  assert.equal(result.accountingPreserved, true);
  assert.equal(f.p.transaction.TransactItems.length, 8);
});
for (const [name, change] of [
  ["marker", (b) => (b.marker.version.N = "18")],
  ["code", (b) => (b.platform.functions[0].codeSha256 = "drift")],
  ["IAM", (b) => (b.platform.functions[0].iamHash = "drift")],
  ["frontend", (b) => (b.frontendReady = false)],
  ["queue", (b) => (b.depth.visible = "1")],
  [
    "state changes",
    (b) =>
      (b.stateInspection.noObservedStateChangesSinceOriginalCollection = false),
  ],
  ["deploy lock", (b) => (b.stateInspection.noCurrentDeployLock = false)],
  ["incompatible data", (b) => (b.rows[0].item.environmentId = { S: "new" })],
])
  test(name + " blocks before write", () => {
    const f = fixture();
    change(f.bundle);
    assert.throws(() => execute("synthetic", f.context, f.adapter));
    assert.equal(f.writes(), 0);
  });
for (const [name, change] of [
  ["token", (p) => (p.transaction.ClientRequestToken += "x")],
  [
    "condition",
    (p) =>
      (p.transaction.TransactItems[0].Update.ConditionExpression =
        "attribute_exists(PK)"),
  ],
  [
    "extra action",
    (p) => p.transaction.TransactItems.push(p.transaction.TransactItems[1]),
  ],
  ["after", (p) => (p.after.version.N = "18")],
])
  test("changed " + name + " cannot replace accepted expectations", () => {
    const f = fixture();
    change(f.context.fixedProposal);
    assert.throws(() => validateFixed(f.context));
    assert.equal(f.writes(), 0);
  });
test("transport uncertainty never retries", () => {
  const f = fixture();
  let sends = 0;
  f.adapter.transact = () => {
    sends++;
    throw Error("synthetic uncertain");
  };
  assert.throws(() => execute("synthetic", f.context, f.adapter));
  assert.equal(sends, 1);
});
test("marker readback mismatch stops after one write", () => {
  const f = fixture();
  f.adapter.marker = () => f.p.before;
  assert.throws(() => execute("synthetic", f.context, f.adapter));
  assert.equal(f.writes(), 1);
});
test("accounting readback mismatch stops after one write", () => {
  const f = fixture();
  const rows = structuredClone(f.bundle.rows);
  const counter = rows.find((r) => r.item.count?.N !== undefined);
  counter.item.count.N = "999";
  let reads = 0;
  f.adapter.rows = () => (reads++ ? rows : f.bundle.rows);
  assert.throws(() => execute("synthetic", f.context, f.adapter));
  assert.equal(f.writes(), 1);
});
test("immediate marker drift after full inspection blocks write", () => {
  const f = fixture();
  f.adapter.marker = () => f.p.after;
  assert.throws(() => execute("synthetic", f.context, f.adapter));
  assert.equal(f.writes(), 0);
});
test("retained snapshot passes the actual offline assessor", () => {
  const f = fixture();
  assert.equal(verifySnapshot(f.bundle, f.context).oldReleaseVerified, true);
});
