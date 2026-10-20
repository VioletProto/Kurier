// Explicitly authorized real A→B→A exercise. No account/product-data writes.
import assert from "node:assert/strict";
import {
  readFileSync,
  writeFileSync,
  mkdirSync,
  renameSync,
  existsSync,
  rmSync,
  rmdirSync,
} from "node:fs";
import { spawn } from "node:child_process";
import { chromium } from "@playwright/test";
import { inspectArtifact, sha256 } from "../infra/frontend-artifact.ts";
if (
  process.argv.length !== 3 ||
  !["--deploy", "--resume-b"].includes(process.argv[2])
)
  throw Error("Explicit authorized --deploy exercise required.");
const initial = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
const exercise = JSON.parse(
  readFileSync(".sst/frontend-hosting-marker-exercise.json", "utf8"),
);
const resumeB = process.argv[2] === "--resume-b";
assert.equal(
  initial.frontendHosting?.releaseId,
  resumeB ? exercise.b : exercise.a,
);
const a = resumeB
  ? JSON.parse(
      readFileSync(
        `.build/frontend-releases/${exercise.a}/artifact.json`,
        "utf8",
      ),
    )
  : initial.frontendHosting.publicArtifact;
assert.equal(initial.stage, "dev-api");
assert.equal(initial.region, "us-east-2");
const b = JSON.parse(
  readFileSync(`.build/frontend-releases/${exercise.b}/artifact.json`, "utf8"),
);
const site = initial.frontendHosting.siteUrl;
const phases = [];
if (resumeB) {
  const prior = JSON.parse(
    readFileSync(".sst/frontend-hosting-http-validation.json", "utf8"),
  );
  assert.equal(prior.releaseId, a.releaseId);
  assert.equal(prior.genuineAwsHttp, true);
  phases.push({
    phase: "A-before",
    releaseId: a.releaseId,
    verifiedFiles: prior.appFiles.length,
    checkedAt: prior.checkedAt,
    source: "prior genuine HTTP validation",
  });
}
async function command(script, args = [], release) {
  console.log(JSON.stringify({ starting: script, release: release ?? null }));
  await new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [script, ...args], {
      env: {
        ...process.env,
        ...(release ? { KURIER_FRONTEND_ROLLBACK_RELEASE: release } : {}),
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let output = "";
    child.stdout.on("data", (chunk) => {
      output += chunk;
      process.stdout.write(chunk);
    });
    child.stderr.on("data", (chunk) => {
      output += chunk;
    });
    child.on("error", reject);
    child.on("exit", (code) =>
      code === 0
        ? resolve()
        : reject(
            Error(
              `Exercise step failed: ${script}; exit ${code}. Inspect ignored exercise log before resuming.`,
            ),
          ),
    );
    child.on("close", () =>
      writeFileSync(".sst/frontend-hosting-exercise-step.log", output),
    );
  });
}
async function verify(manifest, phase) {
  const response = await fetch(site + "/artifact.json", {
    cache: "no-store",
    signal: AbortSignal.timeout(30000),
  });
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), manifest);
  for (const file of manifest.files) {
    const r = await fetch(site + "/" + file.path, {
      signal: AbortSignal.timeout(30000),
    });
    assert.equal(r.status, 200);
    const bytes = Buffer.from(await r.arrayBuffer());
    assert.equal(bytes.length, file.bytes);
    assert.equal(sha256(bytes), file.sha256);
  }
  phases.push({
    phase,
    releaseId: manifest.releaseId,
    verifiedFiles: manifest.files.length,
    checkedAt: new Date().toISOString(),
  });
  writeFileSync(
    ".sst/frontend-hosting-rollback-progress.json",
    JSON.stringify(phases, null, 2) + "\n",
  );
  console.log(JSON.stringify(phases.at(-1)));
}
async function browserAsset(page, file) {
  const url = new URL("/" + file.path, site);
  url.searchParams.set("kurier-hosting-probe", file.sha256.slice(0, 12));
  const pending = page.waitForResponse((r) => r.url() === url.href);
  await page.evaluate((url) => {
    const link = document.createElement("link");
    link.rel = "preload";
    link.as = "script";
    link.href = url;
    document.head.append(link);
  }, url.href);
  const response = await pending;
  assert.equal(response.status(), 200);
  assert.equal(sha256(await response.body()), file.sha256);
}
async function hasMarker(page) {
  return page.evaluate(
    (marker) =>
      Array.from(document.childNodes).some(
        (node) =>
          node.nodeType === Node.COMMENT_NODE &&
          node.nodeValue?.includes(marker),
      ),
    exercise.marker,
  );
}
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.goto(site, { waitUntil: "networkidle" });
  await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
  if (!resumeB) {
    await verify(a, "A-before");
    assert.ok(!(await hasMarker(page)));
    await command(
      "scripts/deploy-frontend-hosting.mjs",
      ["--recheck-and-deploy"],
      b.releaseId,
    );
  }
  await verify(b, "B");
  await browserAsset(
    page,
    a.files.find((f) => f.path === exercise.retainedOldHashedJs),
  );
  await page.reload({ waitUntil: "networkidle" });
  await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
  assert.ok(await hasMarker(page));
  // Remove A's entire selectable local archive. Recovery has only remote origin bytes.
  const localA = `.build/frontend-releases/${a.releaseId}`;
  const backup = `.build/frontend-rollback-local-backup/${a.releaseId}`;
  assert.ok(existsSync(localA));
  assert.ok(!existsSync(backup));
  mkdirSync(".build/frontend-rollback-local-backup", { recursive: true });
  renameSync(localA, backup);
  assert.ok(!existsSync(localA));
  await command("scripts/restore-frontend-artifact.mjs", [a.releaseId]);
  assert.deepEqual(inspectArtifact(localA + "/app", a.publicConfig), a);
  for (const file of a.files) {
    const bytes = readFileSync(localA + "/app/" + file.path);
    assert.equal(bytes.length, file.bytes);
    assert.equal(sha256(bytes), file.sha256);
  }
  console.log(
    JSON.stringify({
      remoteOnlyARecovery: true,
      verifiedEveryHash: true,
      localArchiveAbsentBeforeRestore: true,
    }),
  );
  await command("scripts/build-frontend-artifact.mjs", [], a.releaseId);
  await command(
    "scripts/deploy-frontend-hosting.mjs",
    ["--recheck-and-deploy"],
    a.releaseId,
  );
  await verify(a, "A-restored");
  await browserAsset(
    page,
    b.files.find((f) => f.path === exercise.markerHashedJs),
  );
  await page.reload({ waitUntil: "networkidle" });
  await page.getByRole("heading", { name: "Sign in", exact: true }).waitFor();
  assert.ok(!(await hasMarker(page)));
  rmSync(backup, { recursive: true });
  try {
    rmdirSync(".build/frontend-rollback-local-backup");
  } catch (error) {
    if (error.code !== "ENOTEMPTY") throw error;
  }
  const report = {
    ...exercise,
    completedAt: new Date().toISOString(),
    siteUrl: site,
    genuineAwsAndChromium: true,
    deploymentPerformed: true,
    deploymentsPerformed: ["B", "A-restored"],
    resumedFromVerifiedB: resumeB,
    retainedAssetProbe:
      "script preload permitted by script-src self; no connect-src change",
    phases,
    retainedAAssetAvailableDuringB: true,
    oldBAssetAvailableAfterRollback: true,
    remoteOnlyARecovery: true,
    verifiedEveryRecoveredHash: true,
    rollbackRebuilt: false,
    finalRelease: a.releaseId,
    temporaryLocalBackupRemoved: true,
    disposableAccountsProjectsExecutionsCreated: 0,
    retainedBArchiveIntentional: true,
  };
  writeFileSync(
    ".sst/frontend-hosting-rollback-validation.json",
    JSON.stringify(report, null, 2) + "\n",
  );
  console.log(JSON.stringify(report));
} finally {
  await browser.close();
}
