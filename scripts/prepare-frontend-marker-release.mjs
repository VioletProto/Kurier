import assert from "node:assert/strict";
// Local-only harmless B artifact for an explicitly authorized A→B→A exercise.
import {
  cpSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
  mkdtempSync,
  renameSync,
  rmSync,
  existsSync,
  unlinkSync,
} from "node:fs";
import path from "node:path";
import {
  inspectArtifact,
  buildFrontendArtifact,
  sha256,
} from "../infra/frontend-artifact.ts";
const out = JSON.parse(readFileSync(".sst/outputs.json", "utf8"));
const a = out.frontendHosting?.publicArtifact;
if (
  out.stage !== "dev-api" ||
  out.region !== "us-east-2" ||
  !/^[a-f0-9]{64}$/.test(a?.releaseId ?? "")
)
  throw Error("Verified deployed A required.");
const root = path.resolve(".build/frontend-releases");
const source = path.join(root, a.releaseId, "app");
assert.deepEqual(
  inspectArtifact(source, a.publicConfig),
  a,
  "Local A differs.",
);
const temp = mkdtempSync(path.join(root, ".marker-"));
try {
  const app = path.join(temp, "app");
  cpSync(source, app, { recursive: true });
  const marker = "Kurier harmless hosting rollback probe B";
  for (const file of ["index.html", "startup.js", "404.html"]) {
    const original = readFileSync(path.join(app, file), "utf8");
    writeFileSync(
      path.join(app, file),
      (file.endsWith(".js") ? `// ${marker}\n` : `<!-- ${marker} -->\n`) +
        original,
    );
  }
  const oldJs = a.files.find(
    (f) => f.path.startsWith("assets/") && f.path.endsWith(".js"),
  );
  const nextJs = Buffer.concat([
    readFileSync(path.join(app, oldJs.path)),
    Buffer.from(`\n// ${marker}\n`),
  ]);
  const nextPath = "assets/index-" + sha256(nextJs).slice(0, 12) + ".js";
  writeFileSync(path.join(app, nextPath), nextJs);
  unlinkSync(path.join(app, oldJs.path));
  const shell = path.join(app, "index.html");
  writeFileSync(
    shell,
    readFileSync(shell, "utf8").replace(oldJs.path, nextPath),
  );
  const b = inspectArtifact(app, a.publicConfig);
  if (b.releaseId === a.releaseId)
    throw Error("Marker did not change release.");
  const target = path.join(root, b.releaseId);
  if (existsSync(target))
    throw Error("Marker archive already exists; inspect separately.");
  renameSync(temp, target);
  const old = process.env.KURIER_FRONTEND_ROLLBACK_RELEASE;
  try {
    process.env.KURIER_FRONTEND_ROLLBACK_RELEASE = b.releaseId;
    buildFrontendArtifact(a.publicConfig);
  } finally {
    if (old === undefined) delete process.env.KURIER_FRONTEND_ROLLBACK_RELEASE;
    else process.env.KURIER_FRONTEND_ROLLBACK_RELEASE = old;
  }
  const report = {
    a: a.releaseId,
    b: b.releaseId,
    marker,
    changedFiles: ["index.html", "startup.js", "404.html", nextPath],
    retainedOldHashedJs: oldJs.path,
    markerHashedJs: nextPath,
    publicConfigurationUnchanged: true,
    retainedHashedAssetsPreserved: true,
    deploymentPerformed: false,
  };
  writeFileSync(
    ".sst/frontend-hosting-marker-exercise.json",
    JSON.stringify(report, null, 2) + "\n",
  );
  console.log(JSON.stringify(report));
} finally {
  rmSync(temp, { recursive: true, force: true });
}
