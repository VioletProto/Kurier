import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
  frontendManifest,
  frontendMatches,
  frontendTarget,
} from "./frontend.mjs";

const prepared = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-release.json",
      import.meta.url,
    ),
  ),
);
function fixture(phase) {
  const expected = frontendManifest(phase);
  const folder =
    phase === "collect" || phase === "close" ? "rollback/frontend" : "frontend";
  const objects = new Map([
    ["artifact.json", Buffer.from(JSON.stringify(expected))],
    ...expected.files.map((f) => [
      f.path,
      readFileSync(
        new URL(
          "../../" +
            prepared.artifactDirectory +
            "/" +
            folder +
            "/app/" +
            f.path,
          import.meta.url,
        ),
      ),
    ]),
  ]);
  const io = {
    readObject: (key) => objects.get(key),
    keyValue: () => "s3",
    distributionDeployed: () => true,
  };
  return { expected, objects, io };
}
for (const phase of ["collect", "close", "verify", "activate", "final"]) {
  test(`${phase} compares exact bytes against its fixed frontend target`, () => {
    const f = fixture(phase);
    assert.equal(frontendMatches(frontendManifest(phase), f.io), true);
    assert.equal(
      frontendTarget(phase),
      ["collect", "close"].includes(phase) ? "baseline" : "adopted",
    );
  });
}
test("old frontend is accepted before deployment and rejected after deployment", () => {
  const f = fixture("collect");
  assert.equal(frontendMatches(frontendManifest("close"), f.io), true);
  for (const phase of ["verify", "activate", "final"])
    assert.equal(frontendMatches(frontendManifest(phase), f.io), false);
});
test("new frontend cannot silently replace the accepted predeployment baseline", () => {
  const f = fixture("verify");
  for (const phase of ["collect", "close"])
    assert.equal(frontendMatches(frontendManifest(phase), f.io), false);
});
for (const mode of [
  "missing",
  "corrupt",
  "wrong-kv",
  "not-deployed",
  "denied",
  "invalid-marker",
]) {
  test(`baseline ${mode} fails closed`, () => {
    const f = fixture("collect");
    const js = f.expected.files.find(
      (x) => x.path.endsWith(".js") && x.path.startsWith("assets/"),
    ).path;
    if (mode === "missing") f.objects.delete(js);
    if (mode === "corrupt")
      f.objects.set(js, Buffer.alloc(f.objects.get(js).length));
    if (mode === "wrong-kv") f.io.keyValue = () => "other";
    if (mode === "not-deployed") f.io.distributionDeployed = () => false;
    if (mode === "denied")
      f.io.readObject = () => {
        throw Error("synthetic private error");
      };
    if (mode === "invalid-marker")
      f.objects.set("artifact.json", Buffer.from("not-json"));
    assert.equal(frontendMatches(f.expected, f.io), false);
  });
}
test("unknown phase cannot choose a frontend manifest", () =>
  assert.throws(() => frontendManifest("deploy")));
