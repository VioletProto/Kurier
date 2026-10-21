// Execution-tooling pins, independent of the source checkout used for Go builds.
import { createHash } from "node:crypto";
export const releaseSource = "68d9f218225c7092a1e4fca0b66a21421d862a22";
export const releaseManifestSha256 =
  "920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6";
const sha256 = (b) => createHash("sha256").update(b).digest("hex");
const zipHashes = {
  UsersProjectsApi:
    "f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32",
  EmptyProjectCleanup:
    "f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32",
  CloudExecutionWorker:
    "15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379",
  ExecutionControlledEndpoint:
    "15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379",
};
export function verifyReleasePins(manifestBytes, baselineBytes, readArchive) {
  if (sha256(manifestBytes) !== releaseManifestSha256)
    throw Error("Pinned fixture-inclusive manifest required.");
  const r = JSON.parse(manifestBytes);
  if (
    r.releaseSourceCommit !== releaseSource ||
    r.applicationCommit !== releaseSource ||
    r.inventory.sourceCommit !== releaseSource ||
    sha256(baselineBytes) !== r.readerBaselineSha256 ||
    r.artifactDirectory !==
      ".build/workflow-assertions-release/" + releaseSource ||
    r.preview.packages.length !== 4 ||
    r.packageBuilds.length !== 4
  )
    throw Error("Pinned release/baseline mismatch.");
  const seen = new Set();
  for (const p of r.preview.packages) {
    const build = r.packageBuilds.find((b) => b.name === p.name);
    if (
      seen.has(p.name) ||
      zipHashes[p.name] !== p.sha256 ||
      !build ||
      build.vcsRevision !== releaseSource ||
      build.vcsModified !== false ||
      build.zipSha256 !== p.sha256 ||
      build.goos !== "linux" ||
      build.goarch !== "arm64" ||
      p.newKey !== `assets/${p.name}-code-${p.sha256}.zip`
    )
      throw Error("Clean pinned fixture-inclusive package required.");
    seen.add(p.name);
    const bytes = readArchive(`${r.artifactDirectory}/${p.name}/code.zip`);
    if (bytes.length !== p.bytes || sha256(bytes) !== p.sha256)
      throw Error("Local archive does not match the reviewed fixture release.");
  }
  return r;
}
