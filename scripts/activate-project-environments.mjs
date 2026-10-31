// Local-only entrypoint. The transition engine is transport-injected and tested.
// A metered live adapter must be separately reviewed before any stage action.
import { spawnSync } from "node:child_process";
const args = process.argv.slice(2);
if (args.length > 1 || (args.length && args[0] !== "--check-local"))
  throw Error(
    "Use no arguments/--check-local. Live stage modes require a separately reviewed metered adapter; no AWS transport is enabled.",
  );
const result = spawnSync(
  new URL("../.build/attempt-meter-venv/bin/python", import.meta.url).pathname,
  ["-B", new URL("environment-release-check.py", import.meta.url).pathname],
  { encoding: "utf8" },
);
if (result.status !== 0) {
  console.error(
    "Local environment archive check held; no AWS operation was attempted.",
  );
  process.exitCode = 1;
} else process.stdout.write(result.stdout);
