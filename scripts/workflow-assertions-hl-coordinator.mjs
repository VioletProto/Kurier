// Private execution coordinator. Credentials remain solely in this process.
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { randomUUID, randomBytes, createHash } from "node:crypto";
import {
  actions,
  checkAdmission,
} from "./lib/workflow-assertions-hl-policy.mjs";
import { createInterface } from "node:readline";
import { Amplify } from "aws-amplify";
import { signIn, signOut, fetchAuthSession } from "aws-amplify/auth";
import { cognitoUserPoolsTokenProvider } from "aws-amplify/auth/cognito";
import { sharedInMemoryStorage } from "aws-amplify/utils";
import {
  awsProcess as rawAWSProcess,
  outputs as o,
  identity,
  marker,
  hash,
} from "./verify-read-only-mcp-deployment.mjs";
process.env.AWS_MAX_ATTEMPTS = "1";
import {
  createCoordinatorTransport,
  coordinatorBudget,
  persistCoordinatorLedger,
} from "./lib/workflow-coordinator-transport.mjs";
const dir = new URL("./workflow-assertions-hl/", import.meta.url);
const receiptPath = new URL(
  "../.build/workflow-assertions-hl-replacement/ledger.json",
  import.meta.url,
).pathname;
if (!process.env.KURIER_ATTEMPT_METER_ACTIVE)
  throw Error("Isolated attempt meter required");
const ledger = {
  kind: "kurier-assertions-H-L-v1",
  authorization:
    "Explicit H-L replacement accepted4803bd9d1c108dd8655574af55aff8aa1fc0e74b; new120min clock,173AWS/31API/9static retained",
  tag: randomUUID(),
  users: [],
  projects: [],
  runs: [],
  events: [],
  apiCalls: 31,
  polls: 0,
  awsReads: 0,
  admittedSteps: 18,
  expectedSends: 16,
  prior: {
    admittedSteps: 18,
    targetSends: 16,
    identitiesCreatedAndCleaned: 7,
    projectsCreatedAndCleaned: 7,
    knownApiCalls: 147,
    knownPolls: 11,
    knownSecondAttemptAwsCharges: 574,
    knownFinalAttemptAwsCharges: 917,
    cleanupVerificationAwsCharges: 20,
    awsReadTotal: null,
    billedIncrementalUsd: null,
  },
};
const privateState = {
  users: [],
  requests: {},
  workflows: {},
  runs: {},
  evidence: {},
  fixtureCounts: {},
  stopped: false,
};
const check = (v, s) => {
  if (!v) throw Error(s);
};
const save = () => persistCoordinatorLedger(receiptPath, ledger);
const note = (event, fields = {}) => {
  ledger.events.push({ at: new Date().toISOString(), event, ...fields });
  save();
  console.log(JSON.stringify({ event, ...fields }));
};
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
function aws(args, input) {
  budget(privateState.cleanup);
  check(ledger.awsReads < 1500, "AWS read limit");
  ledger.awsReads++;
  save();
  const r = awsProcessNoCount(args, input);
  check(r.status === 0 && !r.error, "Scoped AWS read/write failed; no replay");
  const raw = r.stdout.toString("utf8");
  return raw.trim() ? JSON.parse(raw) : {};
}
function awsProcess(args, input) {
  budget(privateState.cleanup);
  ledger.awsReads++;
  save();
  return awsProcessNoCount(args, input);
}
function awsProcessNoCount(args, input) {
  const start = Date.parse(ledger.startedAt);
  return rawAWSProcess(
    [...args, "--cli-connect-timeout", "4", "--cli-read-timeout", "8"],
    input,
    {
      deadlineMs: start + (privateState.cleanup ? 120 : 100) * 60_000,
      reserveMs: 5000,
    },
  );
}
const sharedBudget = coordinatorBudget({
  ledger,
  meterPath: process.env.KURIER_ATTEMPT_METER_DIRECTORY + "/attempts.jsonl",
});
function budget(cleanup = false) {
  sharedBudget({ cleanup });
}
async function token(i = 0) {
  const u = privateState.users[i];
  check(u, "RAM identity missing");
  if (u.expires - Date.now() > 120000) return u.token;
  await signOut();
  const s = await signIn({
    username: u.name,
    password: u.password,
    options: { authFlowType: "USER_SRP_AUTH" },
  });
  check(s.isSignedIn, "Genuine Cognito SRP failed");
  const a = await fetchAuthSession();
  check(a.tokens?.accessToken, "SRP access token missing");
  u.token = a.tokens.accessToken.toString();
  u.expires = Number(a.tokens.accessToken.payload.exp) * 1000;
  return u.token;
}
const openingEvents = readFileSync(
  process.env.KURIER_ATTEMPT_METER_DIRECTORY + "/attempts.jsonl",
  "utf8",
)
  .trim()
  .split("\n")
  .map(JSON.parse);
ledger.startedAt = new Date(
  (openingEvents[0].continuationParent?.firstAttemptAt ??
    openingEvents.find((e) => e.event === "attempt")?.at) * 1000,
).toISOString();
ledger.continuationParent = openingEvents[0].continuationParent ?? null;
ledger.replacementParent = openingEvents[0].replacementParent ?? null;
check(
  ledger.replacementParent?.counts.aws === 173 &&
    ledger.replacementParent?.counts.api === 31 &&
    ledger.replacementParent?.counts.static === 9,
  "Explicit replacement parent required",
);
const transport = createCoordinatorTransport({
  origin: o.apiUrl,
  ledger,
  token,
  budget: sharedBudget,
  persist: save,
  owner: () => privateState.projectIndex ?? 0,
});
const call = (...args) => {
  ledger.cleanup = privateState.cleanup === true;
  return transport.call(...args);
};
const root = () =>
  "/api/v1/projects/" + ledger.projects[privateState.projectIndex ?? 0].id;
function query(
  table,
  pk,
  prefix = "",
  fields = [
    "PK",
    "SK",
    "kind",
    "state",
    "version",
    "ownerId",
    "projectId",
    "executionId",
    "workflowRunId",
    "position",
    "inflightCount",
    "storageReservedBytes",
    "pinnedCount",
    "pinnedBytes",
    "count",
    "month",
    "day",
    "normalExpiresAt",
    "assertionsSchemaVersion",
  ],
) {
  const names = Object.fromEntries(fields.map((x, i) => ["#f" + i, x]));
  const r = aws(["dynamodb", "query"], {
    TableName: table,
    KeyConditionExpression:
      "PK = :pk" + (prefix ? " AND begins_with(SK,:prefix)" : ""),
    ExpressionAttributeValues: {
      ":pk": { S: pk },
      ...(prefix ? { ":prefix": { S: prefix } } : {}),
    },
    ProjectionExpression: Object.keys(names).join(","),
    ExpressionAttributeNames: names,
    ConsistentRead: true,
  });
  check(!r.LastEvaluatedKey, "Bounded query incomplete");
  return r.Items ?? [];
}
async function request(key, mode, auth = false) {
  check(Object.keys(privateState.requests).length < 6, "Saved request limit");
  const fixture = randomUUID();
  const b = {
    name: "Assertions " + key,
    method: "GET",
    url: o.apiUrl + "/execution-fixture/" + mode + "?fixture=" + fixture,
    queryParameters: [],
    headers: auth
      ? [{ name: "Authorization", value: "", enabled: true, sensitive: false }]
      : [],
    body: null,
    operationRef: null,
  };
  const r = await call("POST", root() + "/requests", b, 201);
  const v = r.data.request;
  check(v?.requestId, "Request write outcome invalid");
  privateState.requests[key] = { ...v, fixture, mode };
  note("request-created", { key, id: v.requestId, fixture, mode });
  return privateState.requests[key];
}
function step(r) {
  return {
    stepId: randomUUID(),
    requestId: r.requestId,
    requestRevision: r.revision,
    extract: [],
    bindings: [],
  };
}
async function definition(key, a, b, { bind = false, assertions } = {}) {
  check(
    Object.keys(privateState.workflows).length < 12,
    "Workflow definition limit",
  );
  const steps = [step(a), step(b)];
  if (bind) {
    steps[0].extract = [
      { name: "token", path: "$.token", type: "string", sensitive: false },
    ];
    steps[1].bindings = [
      {
        from: { stepId: steps[0].stepId, name: "token" },
        to: { kind: "header", index: 0, prefix: "Bearer " },
      },
    ];
  }
  if (assertions) steps.forEach((s, i) => (s.assertions = assertions[i]));
  const r = await call(
    "POST",
    root() + "/workflows",
    { name: "Assertions " + key, steps },
    201,
  );
  check(r.data.workflow?.workflowId, "Workflow write outcome invalid");
  privateState.workflows[key] = r.data.workflow;
  note("workflow-created", {
    key,
    id: r.data.workflow.workflowId,
    revision: r.data.workflow.revision,
  });
  return r.data.workflow;
}
async function submit(key, w, { source, state = "completed", sends = 2 } = {}) {
  budget();
  checkAdmission(ledger, key, w, { source, state, sends });
  check(
    !privateState.runs[key] &&
      /^[H-L]$/.test(key) &&
      ledger.preflightVerified &&
      ledger.users.length === 1 &&
      ledger.projects.length === 1,
    "No replay or reserve submission",
  );
  check(
    ledger.admittedSteps + 2 <= 28 && ledger.expectedSends + sends <= 24,
    "Admission/send limit",
  );
  const idempotency = randomUUID();
  note("submission-intent", {
    key,
    idempotency,
    source: source ?? w.workflowId,
    steps: 2,
    expectedSends: sends,
  });
  ledger.admittedSteps += 2;
  ledger.expectedSends += sends;
  save();
  const route = source
    ? "/workflow-runs/" + source + "/rerun"
    : "/workflows/" + w.workflowId + "/runs";
  const r = await call(
    "POST",
    root() + route,
    { target: "cloud", allowInsecureSecrets: false },
    202,
    {
      "Idempotency-Key": idempotency,
      ...(!source ? { "If-Match": '"' + w.revision + '"' } : {}),
    },
  );
  let run = r.data.run;
  check(run?.workflowRunId, "Admission response uncertain");
  privateState.runs[key] = run;
  const receipt = {
    key,
    id: run.workflowRunId,
    idempotency,
    expectedState: state,
    expectedSends: sends,
    projectIndex: privateState.projectIndex ?? 0,
    steps: [],
  };
  ledger.runs.push(receipt);
  save();
  run = await transport.pollSubmitted(
    ledger.projects[privateState.projectIndex ?? 0].id,
    r,
  );
  check(
    run.state === state,
    "Unexpected terminal run state " + key + ": " + run.state,
  );
  privateState.runs[key] = run;
  receipt.state = run.state;
  receipt.steps = run.steps.map((s) => ({
    position: s.position,
    state: s.state,
    executionId: s.executionId,
  }));
  note("run-terminal", {
    key,
    id: run.workflowRunId,
    state: run.state,
    steps: receipt.steps,
  });
  return run;
}
async function audits() {
  const result = aws(["logs", "filter-log-events"], {
    logGroupName: "/aws/lambda/" + o.controlledEndpointFunction,
    startTime: Date.parse(ledger.startedAt),
    filterPattern: '"kurierWorkflowFixture"',
    limit: 1000,
  });
  check(!result.nextToken, "Audit result incomplete");
  const records = [];
  const ids = new Set(
    Object.values(privateState.requests).map((r) => r.fixture),
  );
  for (const e of result.events ?? []) {
    let v;
    try {
      v = JSON.parse(e.message.slice(e.message.indexOf("{")));
    } catch {
      continue;
    }
    if (ids.has(v.fixtureId))
      records.push({
        fixtureId: v.fixtureId,
        mode: v.mode,
        authorized: v.authorized,
        requestId: v.requestId,
      });
  }
  const uniq = new Set(records.map((r) => r.requestId));
  check(uniq.size === records.length, "Duplicate audit request ID");
  check(records.length <= ledger.expectedSends - 16, "Unexpected target send");
  ledger.targetAudits = records;
  save();
  return records;
}
async function reconcile() {
  let a;
  for (let n = 0; n < 12; n++) {
    a = await audits();
    if (a.length === ledger.expectedSends - 16) break;
    await sleep(5000);
  }
  check(
    a.length === ledger.expectedSends - 16,
    "Target send-count proof unavailable",
  );
  note("target-sends-verified", {
    actual: a.length,
    expected: ledger.expectedSends,
  });
}
Amplify.configure({
  Auth: {
    Cognito: {
      userPoolId: o.cognitoUserPoolId,
      userPoolClientId: o.cognitoClientId,
    },
  },
});
cognitoUserPoolsTokenProvider.setKeyValueStorage(sharedInMemoryStorage);
const c = {
  ledger,
  privateState,
  check,
  save,
  note,
  sleep,
  aws,
  awsProcess,
  o,
  identity,
  marker,
  hash,
  budget,
  token,
  call,
  root,
  query,
  request,
  step,
  definition,
  submit,
  audits,
  reconcile,
  dir,
  readFileSync,
  writeFileSync,
  mkdirSync,
  randomUUID,
  randomBytes,
  createHash,
};
console.log(
  JSON.stringify({ ready: true, credentials: "RAM only", tag: ledger.tag }),
);
for await (const line of createInterface({ input: process.stdin })) {
  try {
    const a = JSON.parse(line);
    check(
      actions.includes(a.module),
      "H-L-only fixed action allowlist required",
    );
    check(
      !privateState.stopped || a.module === "stopped-cleanup.mjs",
      "Stopped: cleanup only; no automatic continuation",
    );
    const mod = await import(new URL(a.module, dir) + "?" + Date.now());
    await mod.run(c);
    console.log(
      JSON.stringify({
        actionComplete: a.module,
        apiCalls: ledger.apiCalls,
        awsReads: ledger.awsReads,
        admitted: ledger.admittedSteps,
        sends: ledger.expectedSends,
      }),
    );
  } catch (e) {
    privateState.stopped = true;
    ledger.stoppedAt = new Date().toISOString();
    if (
      /^(launch|navigate|sign-in|select-project|load-workflows|select-workflow|author-step-[12](-assertion-[1-8]-add)?|author-(kind|path|type|decimal)|reopen-decimals|save-workflow|drain-browser-reads|close-browser)$/.test(
        privateState.browserProgress ?? "",
      )
    )
      ledger.browserStopContext = {
        stage: privateState.browserProgress,
        errorClass: ["TimeoutError", "TypeError", "Error"].includes(
          privateState.browserFailureClass,
        )
          ? privateState.browserFailureClass
          : "unknown",
      };
    ledger.stopReason = /^[A-Za-z0-9 /:.,_#?=+<>"@()-]{1,300}$/.test(e.message)
      ? e.message
      : "Sanitized execution failure; inspect recorded state";
    save();
    console.log(
      JSON.stringify({
        STOP: true,
        reason: ledger.stopReason,
        noAutomaticReplayOrReopen: true,
      }),
    );
  }
}
