// Fixed phase expectations. No transport, credential lookup or caller manifests.
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { hash } from "../lib/project-environments-activation.mjs";

const inventory = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-inventory.json",
      import.meta.url,
    ),
  ),
);
const adopted = JSON.parse(
  readFileSync(
    new URL(
      "../../docs/development/project-environments-adopted-release.json",
      import.meta.url,
    ),
  ),
);
export function frontendTarget(phase) {
  if (["collect", "close"].includes(phase)) return "baseline";
  if (["verify", "activate", "final"].includes(phase)) return "adopted";
  throw Error("Unknown frontend phase.");
}
export function frontendManifest(phase) {
  return frontendTarget(phase) === "baseline"
    ? inventory.frontendRollback.manifest
    : adopted.frontend;
}
export function frontendMatches(expected, io) {
  try {
    const marker = io.readObject("artifact.json");
    if (!marker || hash(JSON.parse(marker)) !== hash(expected)) return false;
    for (const f of expected.files) {
      const bytes = io.readObject(f.path);
      if (
        !bytes ||
        bytes.length !== f.bytes ||
        createHash("sha256").update(bytes).digest("hex") !== f.sha256
      )
        return false;
      if (f.path.startsWith("assets/") && io.keyValue(f.path) !== "s3")
        return false;
    }
    return io.distributionDeployed();
  } catch {
    return false;
  }
}
