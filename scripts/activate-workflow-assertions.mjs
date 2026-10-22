// Default is read-only. --close/--activate remain held pending package review.
import { readFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { verifyReleasePins } from "./lib/workflow-assertions-release.mjs";
import {
  execute,
  projectionFields,
  ActivationGuardError,
  hash,
} from "./lib/workflow-assertions-activation.mjs";
const args = process.argv.slice(2);
const mode = args[0] ?? "--inspect";
if (
  !["--inspect", "--close", "--activate"].includes(mode) ||
  args.length !== (mode === "--inspect" ? (args.length ? 1 : 0) : 2)
)
  throw Error(
    "Use no arguments/--inspect, or explicitly reviewed --close/--activate APPROVAL.json.",
  );
const root = new URL("../", import.meta.url);
const bytesHash = (b) => createHash("sha256").update(b).digest("hex");
const raw = readFileSync(
  new URL("docs/development/workflow-assertions-release.json", root),
);
const baselineRaw = readFileSync(
  new URL("docs/development/workflow-assertions-reader-baseline.json", root),
);
const release = verifyReleasePins(raw, baselineRaw, (relative) =>
  readFileSync(new URL(relative, root)),
);
const git = (args) =>
  execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();
const action = mode.slice(2);
if (
  action !== "inspect" &&
  git(["status", "--porcelain", "--untracked-files=all"])
)
  throw Error(
    "Write mode requires a clean reviewed execution-package checkout.",
  );
// AWS CLI must not retry an uncertain transaction after a lost response.
process.env.AWS_MAX_ATTEMPTS = "1";
const {
  aws,
  outputs: o,
  marker,
  snapshot,
} = await import("./verify-read-only-mcp-deployment.mjs");
if (
  o.stage !== "dev-api" ||
  o.region !== "us-east-2" ||
  o.controlTable !== "kurier-dev-api-ControlTable-bdxbaoxb" ||
  o.protectedTable !== "kurier-dev-api-ProtectedTable-szbvcchc"
)
  throw Error("Unexpected stage/table scope.");
const adapter = {
  controlTable: o.controlTable,
  identity: () => aws(["sts", "get-caller-identity"]),
  marker,
  frontendReady: () => {
    const directory = mkdtempSync(
      path.join(tmpdir(), "kurier-public-artifact-"),
    );
    const file = path.join(directory, "public-file");
    const bucket = "kurier-dev-api-web-fc2927a49a0b246240e5a17774";
    try {
      aws([
        "s3api",
        "get-object",
        "--bucket",
        bucket,
        "--key",
        "artifact.json",
        file,
      ]);
      if (hash(JSON.parse(readFileSync(file))) !== hash(release.frontend))
        return false;
      if (
        aws(["cloudfront", "get-distribution", "--id", "EMNTWD8Q14ZLM"])
          .Distribution.Status !== "Deployed"
      )
        return false;
      for (const entry of release.frontend.files) {
        if (
          !/^(404\.html|index\.html|startup\.js|assets\/[A-Za-z0-9_-]+\.(js|css))$/.test(
            entry.path,
          )
        )
          throw Error("Unreviewed static path.");
        aws([
          "s3api",
          "get-object",
          "--bucket",
          bucket,
          "--key",
          entry.path,
          file,
        ]);
        const bytes = readFileSync(file);
        if (bytes.length !== entry.bytes || bytesHash(bytes) !== entry.sha256)
          return false;
        if (
          entry.path.startsWith("assets/") &&
          aws([
            "cloudfront-keyvaluestore",
            "get-key",
            "--kvs-arn",
            "arn:aws:cloudfront::747336059622:key-value-store/0e7ac6e9-b4b6-4a9e-b673-4d3838436ecd",
            "--key",
            "76ef:/" + entry.path,
          ]).Value !== "s3"
        )
          return false;
      }
      return true;
    } finally {
      rmSync(directory, { recursive: true });
    }
  },
  platform: () => {
    const api = new URL(o.apiUrl).hostname.split(".")[0];
    const stage = aws([
      "apigatewayv2",
      "get-stage",
      "--api-id",
      api,
      "--stage-name",
      "$default",
    ]);
    if (
      stage.DefaultRouteSettings?.ThrottlingRateLimit !== 10 ||
      stage.DefaultRouteSettings?.ThrottlingBurstLimit !== 20
    )
      throw Error("API throttle drift.");
    return snapshot();
  },
  queueDepth: () =>
    aws([
      "sqs",
      "get-queue-attributes",
      "--queue-url",
      o.executionQueueUrl,
      "--attribute-names",
      "ApproximateNumberOfMessages",
      "ApproximateNumberOfMessagesNotVisible",
      "ApproximateNumberOfMessagesDelayed",
    ]).Attributes,
  rows: () => {
    const rows = [];
    const names = Object.fromEntries(
      projectionFields.map((x, i) => ["#p" + i, x]),
    );
    for (const table of [o.controlTable, o.protectedTable]) {
      let cursor;
      for (let page = 0; page < 1000; page++) {
        const request = [
          "dynamodb",
          "scan",
          "--table-name",
          table,
          "--consistent-read",
          "--limit",
          "100",
          "--no-paginate",
          "--projection-expression",
          Object.keys(names).join(","),
          "--expression-attribute-names",
          JSON.stringify(names),
        ];
        if (cursor)
          request.push("--exclusive-start-key", JSON.stringify(cursor));
        const result = aws(request);
        rows.push(...(result.Items ?? []).map((item) => ({ table, item })));
        cursor = result.LastEvaluatedKey;
        if (!cursor) break;
        if (page === 999) throw Error("Incomplete bounded metadata inventory.");
      }
    }
    // Permanent residual sweep registrations intentionally survive project deletion.
    // Prove emptiness read-only; do not delete registrations or invoke the sweeper.
    for (const r of rows.filter(
      (x) => x.item.kind?.S === "executionResidual",
    )) {
      const project = r.item.projectId?.S;
      if (!/^[a-f0-9-]{36}$/.test(project ?? ""))
        throw Error("Invalid residual project.");
      const objects = aws([
        "s3api",
        "list-objects-v2",
        "--bucket",
        o.evidenceBucket,
        "--prefix",
        "dev-api/projects/" + project + "/",
        "--max-keys",
        "1",
        "--no-paginate",
      ]);
      r.residualPrefixEmpty = !(
        objects.Contents?.length || objects.IsTruncated
      );
    }
    return rows;
  },
  transact: (TransactItems) => {
    if (action === "inspect") throw Error("Read-only mode cannot write.");
    return aws(["dynamodb", "transact-write-items"], { TransactItems });
  },
};
try {
  console.log(
    JSON.stringify(
      await execute(adapter, {
        action,
        release,
        baseline: JSON.parse(baselineRaw),
        reviewCommit: git(["rev-parse", "HEAD"]),
        manifestSha256: bytesHash(raw),
        approval:
          action === "inspect"
            ? undefined
            : JSON.parse(readFileSync(args[1], "utf8")),
        writeApproval: process.env.KURIER_ASSERTIONS_WRITE_APPROVAL,
      }),
      null,
      2,
    ),
  );
} catch (error) {
  // Do not print AWS input/response, request bodies, tokens or arbitrary errors.
  console.error(
    "Workflow assertion preflight/write failed or is uncertain. No automatic retry/reopen. Inspect the durable marker read-only before any separately approved recovery.",
  );
  if (error instanceof ActivationGuardError) console.error(error.message);
  process.exitCode = 1;
}
