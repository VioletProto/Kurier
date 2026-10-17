// Read-only state inspection: raw state stays ignored; only explicit public fields escape.
import { writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { aws, identity } from "./verify-read-only-mcp-deployment.mjs";
identity();
const c = aws(["configure", "export-credentials", "--format", "process"]);
const r = spawnSync(
  "npm",
  ["exec", "--", "sst", "state", "export", "--stage", "dev-api"],
  {
    encoding: "utf8",
    maxBuffer: 32 * 1024 * 1024,
    env: {
      ...process.env,
      AWS_REGION: "us-east-2",
      AWS_ACCESS_KEY_ID: c.AccessKeyId,
      AWS_SECRET_ACCESS_KEY: c.SecretAccessKey,
      AWS_SESSION_TOKEN: c.SessionToken,
      SST_TELEMETRY_DISABLED: "1",
    },
  },
);
if (r.status !== 0) throw Error("Read-only state export failed.");
const state = JSON.parse(r.stdout);
writeFileSync(".sst/frontend-hosting-state.json", JSON.stringify(state));
const resources =
  state.latest?.resources ?? state.deployment?.resources ?? state.resources;
if (!Array.isArray(resources)) throw Error("Unexpected state export shape.");
const selected = resources
  .filter((r) => r.urn?.split("::").at(-1).startsWith("FrontendWeb"))
  .map((r) => ({
    name: r.urn.split("::").at(-1),
    type: r.type,
    id: r.id,
    protect: r.protect ?? false,
    retainOnDelete: r.retainOnDelete ?? false,
    dependencies: (r.dependencies ?? []).map((d) => d.split("::").at(-1)),
    inputKeys: Object.keys(r.inputs ?? {}).filter((k) => !k.startsWith("__")),
    outputKeys: Object.keys(r.outputs ?? {}).filter((k) => !k.startsWith("__")),
  }));
writeFileSync(
  ".sst/frontend-hosting-state-summary.json",
  JSON.stringify(selected, null, 2) + "\n",
);
console.log(JSON.stringify(selected));
