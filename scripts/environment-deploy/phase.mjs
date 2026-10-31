// Pinned child commands. No arbitrary command or AWS operation forwarding API.
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import {
  aws,
  marker,
  platform,
  inventory,
  queueDepth,
  frontendReady,
  collect,
  hash,
  release,
  scope,
  uploadFrontend,
  verifyHistorical,
  verifyInitialRouting,
  routing,
  verifyRouting,
} from "./collector.mjs";
import {
  execute,
  transition,
  inspectRows,
} from "../lib/project-environments-activation.mjs";
import { frontendTarget, frontendManifest } from "./frontend.mjs";
const [action, contextFile] = process.argv.slice(2);
const c = JSON.parse(readFileSync(contextFile));
const now = () => Number(process.hrtime.bigint() / 1000000n);
const adapter = {
  controlTable: scope.controlTable,
  identity: () => aws("sts", "get-caller-identity"),
  marker,
  platform: () => {
    const p = platform();
    if (action === "activate") verifyRouting(routing(), c.initialRouting);
    return p;
  },
  rows: inventory,
  queueDepth,
  frontendReady: () => frontendReady(action),
  transact: (TransactItems) =>
    aws("dynamodb", "transact-write-items", {
      TransactItems,
      ClientRequestToken: c.clientRequestToken,
    }),
};
try {
  if (action === "collect" || action === "verify") {
    const bundle = collect(action);
    if (!bundle.frontendReady)
      throw Error("Expected phase frontend baseline not ready.");
    if (action === "collect") {
      if (bundle.marker.state.S !== "active")
        throw Error("Existing closed stage requires separate recovery review.");
      verifyHistorical(bundle.platform, bundle.marker);
      verifyInitialRouting(bundle.routing);
    }
    if (action === "verify") {
      if (hash(bundle.marker) !== hash(c.closedMarker))
        throw Error("Closed tuple drift.");
      verifyRouting(bundle.routing, c.initialRouting);
    }
    console.log(JSON.stringify(bundle));
  } else if (action === "plan") {
    let m = structuredClone(c.bundle.marker);
    const b = c.bundle;
    if (
      b.frontendExpectation !== frontendTarget(c.action) ||
      b.frontendExpectedReleaseId !== frontendManifest(c.action).releaseId
    )
      throw Error("Wrong phase frontend proof.");
    const fake = {
      controlTable: scope.controlTable,
      identity: () => b.identity,
      marker: () => m,
      platform: () => b.platform,
      rows: () => b.rows,
      queueDepth: () => b.depth,
      frontendReady: () => b.frontendReady,
      transact: (items) => {
        c.transaction = {
          TransactItems: items,
          ClientRequestToken: c.clientRequestToken,
        };
        m = transition(m, c.action).next;
      },
    };
    const result = await execute(fake, {
      action: c.action,
      release,
      baseline: c.baseline,
      reviewCommit: c.reviewCommit,
      manifestSha256: c.manifestSha256,
      approval: c.approval,
      writeApproval: c.reviewCommit + ":" + c.action,
      closure: c.closure,
      now,
    });
    console.log(
      JSON.stringify({ transaction: c.transaction, next: m, result }),
    );
  } else if (action === "close" || action === "activate") {
    const result = await execute(adapter, {
      action,
      release,
      baseline: c.baseline,
      reviewCommit: c.reviewCommit,
      manifestSha256: c.manifestSha256,
      approval: c.approval,
      writeApproval: c.reviewCommit + ":" + action,
      closure: c.closure,
      now,
    });
    console.log(JSON.stringify(result));
  } else if (action === "deploy") {
    const identity = aws("sts", "get-caller-identity");
    if (
      identity.Arn !== scope.identityArn ||
      identity.Account !== scope.account ||
      hash(marker()) !== hash(c.closedMarker)
    )
      throw Error("Deployment identity/closure drift.");
    // Warm the approved hashed assets/KV entries before SST can publish entrypoints.
    const store = aws("cloudfront-keyvaluestore", "describe-key-value-store", {
      KvsARN: scope.kvsArn,
    });
    const puts = [];
    for (const f of release.frontend.files.filter((f) =>
      f.path.startsWith("assets/"),
    )) {
      uploadFrontend(f);
      puts.push({ Key: scope.namespace + ":/" + f.path, Value: "s3" });
    }
    // Uploads are performed by the Python helper, preserving actual bytes rather
    // than embedding them as JSON or letting CLI Body become an arbitrary string.
    if (puts.length)
      aws("cloudfront-keyvaluestore", "update-keys", {
        KvsARN: scope.kvsArn,
        IfMatch: store.ETag,
        Puts: puts,
      });
    const r = spawnSync(c.sst, ["deploy", "--stage", "dev-api", "--yes"], {
      stdio: "inherit",
    });
    if (r.status !== 0)
      throw Error("SST deployment failed; no retry or reopen.");
  } else if (action === "diff") {
    const r = spawnSync(c.sst, ["diff", "--stage", "dev-api", "--json"], {
      stdio: "inherit",
    });
    if (r.status !== 0) throw Error("Fresh SST diff failed.");
  } else if (action === "final") {
    const identity = aws("sts", "get-caller-identity");
    if (
      identity.Arn !== scope.identityArn ||
      hash(marker()) !== hash(c.expectedActiveMarker)
    )
      throw Error("Final tuple drift.");
    const p = platform();
    verifyRouting(routing(), c.initialRouting);
    if (
      hash(p) !== hash(c.verifiedPlatform) ||
      !frontendReady("final") ||
      inspectRows(inventory()).accountingSha256 !== c.accountingSha256
    )
      throw Error("Final deployed readers drift.");
    console.log(
      JSON.stringify({
        finalReadbackVerified: true,
        stageVersion: c.expectedActiveMarker.version.N,
        accountingUnchanged: true,
      }),
    );
  } else throw Error("Unknown phase.");
} catch {
  console.error(
    "Execution phase held/uncertain. No retry, compensation, reopen or automatic continuation.",
  );
  process.exitCode = 1;
}
