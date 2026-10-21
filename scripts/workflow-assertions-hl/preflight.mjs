import { snapshot, hash } from "../verify-read-only-mcp-deployment.mjs";
import { checkPlatform } from "../lib/workflow-assertions-activation.mjs";
import {
  accountingQuery,
  accountingSnapshot,
  compareAccounting,
} from "../lib/workflow-assertions-collectors.mjs";
export async function run(c) {
  const {
    ledger: l,
    privateState: p,
    check,
    aws,
    o,
    readFileSync,
    createHash,
    call,
    save,
    note,
  } = c;
  check(
    !l.preflightVerified && !l.users.length && !l.projects.length,
    "Preflight cannot replay",
  );
  const who = aws(["sts", "get-caller-identity"]);
  check(
    who.Account === "747336059622" &&
      /^arn:aws:(iam|sts)::747336059622:(user\/|assumed-role\/)/.test(who.Arn),
    "Intended nonroot account required",
  );
  const releaseBytes = readFileSync(
    "docs/development/workflow-assertions-release.json",
  );
  check(
    createHash("sha256").update(releaseBytes).digest("hex") ===
      "920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6",
    "Manifest drift",
  );
  const release = JSON.parse(releaseBytes),
    baseline = JSON.parse(
      readFileSync("docs/development/workflow-assertions-reader-baseline.json"),
    );
  c.budget();
  const platform = snapshot();
  c.budget();
  check(
    checkPlatform(platform, baseline, release),
    "Deployed pinned reader/IAM/platform drift",
  );
  const retained = JSON.parse(
    readFileSync(
      "docs/development/workflow-assertions-cleanup-verification.json",
    ),
  );
  c.marker = () =>
    aws(["dynamodb", "get-item"], {
      TableName: o.controlTable,
      Key: { PK: { S: "STAGE#dev-api" }, SK: { S: "META" } },
      ConsistentRead: true,
    }).Item;
  const stage = c.marker();
  check(
    hash(stage) === hash(retained.observations.stageAfter),
    "Active stage/generation/capability drift",
  );
  c.accounting = () => {
    const rows = [];
    for (const [pk, prefix] of [
      ["STAGE#dev-api", "QUOTA"],
      ...retained.observations.accountingAfter.rows
        .filter((r) => r.PK.S.startsWith("U#"))
        .map((r) => [r.PK.S, "EXECUTION_DAY#"]),
      ["U#9c919ed7-f835-42ce-82ad-753fce1ed39e", "EXECUTION_DAY#"],
    ]) {
      const r = aws(
        ["dynamodb", "query", "--no-paginate"],
        accountingQuery(o.controlTable, pk, prefix),
      );
      check(!r.LastEvaluatedKey, "Incomplete accounting read");
      rows.push(...(r.Items ?? []));
    }
    return accountingSnapshot(rows);
  };
  const accounting = c.accounting();
  check(
    compareAccounting(retained.observations.accountingAfter, accounting),
    "Retained accounting drift before campaign",
  );
  const hosted = [];
  for (const f of release.frontend.files) {
    c.budget();
    const r = await fetch(o.hostedFrontendOrigin + "/" + f.path, {
      redirect: "error",
      signal: AbortSignal.timeout(15000),
    });
    check(r.status === 200, "Hosted reader status drift");
    const b = Buffer.from(await r.arrayBuffer());
    check(
      b.length === f.bytes &&
        createHash("sha256").update(b).digest("hex") === f.sha256,
      "Hosted pinned byte drift",
    );
    hosted.push({ path: f.path, sha256: f.sha256, bytes: b.length });
  }
  const q = aws(["sqs", "get-queue-attributes"], {
    QueueUrl: o.executionQueueUrl,
    AttributeNames: [
      "ApproximateNumberOfMessages",
      "ApproximateNumberOfMessagesNotVisible",
      "ApproximateNumberOfMessagesDelayed",
    ],
  });
  check(
    Object.values(q.Attributes).every((v) => v === "0"),
    "Unexpected queued work before continuation",
  );
  await call("GET", "/api/v1/projects", undefined, 401, {}, null);
  l.baselineMarker = stage;
  l.baselineStageQuota = accounting.rows.filter(
    (r) => r.PK.S === "STAGE#dev-api",
  );
  l.baselineAccounting = accounting;
  l.preflight = {
    at: new Date().toISOString(),
    identity: { account: who.Account, arn: who.Arn },
    platform,
    hosted,
    queue: q.Attributes,
    anonymousDenied: true,
  };
  l.preflightVerified = true;
  p.projectIndex = 0;
  save();
  note("H-L-preflight-passed", {
    pinnedReaders: 4,
    hostedFiles: hosted.length,
    stageVersion: 15,
    historicalSteps: 18,
    historicalSends: 16,
  });
}
