import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  verifyReleasePins,
  releaseSource,
  releaseManifestSha256,
} from "./lib/workflow-assertions-release.mjs";
const root = new URL("../", import.meta.url);
const manifest = readFileSync(
  new URL("docs/development/workflow-assertions-release.json", root),
);
const baseline = readFileSync(
  new URL("docs/development/workflow-assertions-reader-baseline.json", root),
);
const sha = (b) => createHash("sha256").update(b).digest("hex");
const release = JSON.parse(manifest);
const archive = (relative) => readFileSync(new URL(relative, root));
test("manifest binds clean fixture source, literal tokens and unchanged campaign limits", () => {
  assert.equal(sha(manifest), releaseManifestSha256);
  assert.equal(release.releaseSourceCommit, releaseSource);
  assert.equal(release.inventory.sourceCommit, releaseSource);
  assert.equal(release.equivalentToAcceptedPreview, true);
  assert.equal(release.preview.resourceChanges, 11);
  assert.deepEqual(release.preview.iamChanges, []);
  assert.deepEqual(release.preview.infrastructureChanges, []);
  const responses = release.controlledFixtures.responses;
  assert.deepEqual(
    responses.map((r) => [r.mode, r.status]),
    [
      ["assertions-values", 200],
      ["assertions-not-found", 404],
    ],
  );
  for (const r of responses) {
    assert.equal(sha(Buffer.from(r.literalUtf8)), r.bodySha256);
    assert.equal(Buffer.byteLength(r.literalUtf8), r.bodyBytes);
    assert.equal(r.requiresFixtureUUID, true);
    assert.equal(r.reflectsCallerInput, false);
    assert.equal(r.auditAuthorizedAlwaysFalse, true);
  }
  // Never parse numeric response literals with JavaScript JSON.parse/Number.
  assert.ok(
    responses[0].literalUtf8.includes(
      '"precise":0.10000000000000001,"short":0.1,"exponent":1E-01',
    ),
  );
  assert.ok(
    responses[0].literalUtf8.includes(
      '"min":1e-308,"max":9007199254740991,"zero":-0.0',
    ),
  );
  assert.equal(release.campaignLimits.maximumAdmittedSteps, 28);
  assert.equal(release.campaignLimits.disposableIdentities, 2);
  assert.equal(release.campaignLimits.disposableProjects, 2);
  assert.equal(release.campaignLimits.incrementalAllowanceUsd, "1.00");
});
const available = release.preview.packages.every((p) =>
  existsSync(new URL(`${release.artifactDirectory}/${p.name}/code.zip`, root)),
);
test(
  "all four retained clean archives pass the activator's exact pins",
  { skip: !available },
  () => {
    assert.equal(
      verifyReleasePins(manifest, baseline, archive).releaseSourceCommit,
      releaseSource,
    );
  },
);
test("previous accepted preparation manifest cannot substitute for fixture release", () => {
  const previous = execFileSync(
    "git",
    [
      "show",
      "c9f55d4980e24c1ec8d57e96a722fd06f7f0271f:docs/development/workflow-assertions-release.json",
    ],
    { cwd: root },
  );
  assert.throws(
    () =>
      verifyReleasePins(previous, baseline, () =>
        assert.fail("must reject before reading archives"),
      ),
    /fixture-inclusive manifest/,
  );
});
test("fixture metadata tampering fails before any archive or AWS access", () => {
  const altered = structuredClone(release);
  altered.controlledFixtures.responses[0].bodySha256 = "0".repeat(64);
  assert.throws(
    () =>
      verifyReleasePins(Buffer.from(JSON.stringify(altered)), baseline, () =>
        assert.fail("must reject before reading archives"),
      ),
    /fixture-inclusive manifest/,
  );
});
test("unreviewed reader baseline fails before archive access", () => {
  assert.throws(
    () =>
      verifyReleasePins(manifest, Buffer.from("{}"), () =>
        assert.fail("must reject baseline first"),
      ),
    /baseline mismatch/,
  );
});
test("stale or replaced ZIP bytes cannot activate under new source metadata", () => {
  assert.throws(
    () =>
      verifyReleasePins(manifest, baseline, () =>
        Buffer.from("old-release-archive"),
      ),
    /Local archive/,
  );
});
