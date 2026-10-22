// Export only allowlisted review metadata, never raw Pulumi inputs/state.
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
const raw = readFileSync(".sst/workflow-assertions-diff.txt", "utf8");
let depth = 0,
  quoted = false,
  escaped = false,
  end = -1;
for (let i = 0; i < raw.length; i++) {
  const c = raw[i];
  if (quoted) {
    if (escaped) escaped = false;
    else if (c === "\\") escaped = true;
    else if (c === '"') quoted = false;
    continue;
  }
  if (c === '"') quoted = true;
  if (c === "[") depth++;
  if (c === "]" && --depth === 0) {
    end = i + 1;
    break;
  }
}
if (end < 0) throw Error("Incomplete preview.");
const events = JSON.parse(raw.slice(0, end));
const names = [
  "UsersProjectsApi",
  "EmptyProjectCleanup",
  "CloudExecutionWorker",
  "ExecutionControlledEndpoint",
];
const allowed = new Map(
  names.flatMap((n) => [
    [
      n + "Code",
      {
        type: "aws:s3/bucketObjectv2:BucketObjectv2",
        op: "replace",
        fields: ["key", "source", "versionId"],
      },
    ],
    [
      n + "Function",
      {
        type: "aws:lambda/function:Function",
        op: "update",
        fields: ["lastModified", "s3Key"],
      },
    ],
  ]),
);
for (const [name, fields] of [
  ["FrontendWebKvKeys.sst.aws.KvKeys", ["entries"]],
  ["FrontendWebAssetFiles.sst.aws.BucketFiles", ["files"]],
  ["FrontendWebInvalidation.sst.aws.DistributionInvalidation", ["version"]],
])
  allowed.set(name, {
    type: "pulumi-nodejs:dynamic:Resource",
    op: "update",
    fields,
  });
const resources = new Map();
for (const e of events) {
  const name = e.urn.split("::").pop();
  if (e.op === "read" && name === "FrontendWebAssets") continue;
  const rule = allowed.get(name);
  const op = ["create-replacement", "delete-replaced"].includes(e.op)
    ? "replace"
    : e.op;
  const fields = Object.keys(e.detailedDiff ?? {}).sort();
  if (
    !rule ||
    e.type !== rule.type ||
    op !== rule.op ||
    fields.some(
      (f) =>
        !rule.fields.some(
          (p) => f === p || f.startsWith(p + "[") || f.startsWith(p + "."),
        ),
    )
  )
    throw Error(
      "Unexpected resource/operation/property; manual review required.",
    );
  if (e.op !== "delete-replaced")
    resources.set(name, {
      name,
      type: e.type,
      operation: op,
      changedProperties: fields,
    });
}
if (resources.size !== allowed.size)
  throw Error("Incomplete expected changes.");
const packages = names.map((name) => {
  const e = events.find(
    (e) => e.op === "replace" && e.urn.endsWith("::" + name + "Code"),
  );
  const bytes = readFileSync(`.sst/artifacts/${name}/code.zip`);
  const sha256 = createHash("sha256").update(bytes).digest("hex");
  const oldKey = e.old.inputs.key,
    newKey = e.new.inputs.key;
  if (
    !newKey.endsWith(`-code-${sha256}.zip`) ||
    !/^assets\/[A-Za-z]+-code-[a-f0-9]{64}\.zip$/.test(oldKey)
  )
    throw Error("Package identity mismatch.");
  return { name, bytes: bytes.length, sha256, oldKey, newKey };
});
const kv = events.find((e) =>
  e.urn.endsWith("::FrontendWebKvKeys.sst.aws.KvKeys"),
);
const upload = events.find((e) =>
  e.urn.endsWith("::FrontendWebAssetFiles.sst.aws.BucketFiles"),
);
if (
  kv.new.inputs.purge !== false ||
  upload.new.inputs.purge !== false ||
  kv.new.inputs.namespace !== "76ef"
)
  throw Error("Retention baseline changed.");
const inventory = JSON.parse(
  readFileSync(".sst/workflow-assertions-inventory.json", "utf8"),
);
if (
  !inventory.previewCompleted ||
  !inventory.toolingMetadataUnchanged ||
  inventory.deploymentPerformed ||
  inventory.activationPerformed ||
  inventory.targetRequestsPerformed
)
  throw Error("Completed nonmutating preview evidence required.");
const report = {
  inventory,
  preview: {
    resourceChanges: resources.size,
    resources: [...resources.values()],
    iamChanges: [],
    infrastructureChanges: [],
    packages,
  },
  frontend: JSON.parse(
    readFileSync(".sst/frontend-hosting-artifact.json", "utf8"),
  ),
};
writeFileSync(
  "docs/development/workflow-assertions-preview.json",
  JSON.stringify(report, null, 2) + "\n",
);
console.log(
  JSON.stringify({
    resources: resources.size,
    packages: packages.length,
    iamChanges: 0,
    deploymentPerformed: false,
  }),
);
