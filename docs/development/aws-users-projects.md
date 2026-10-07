# Development AWS users/projects slice

The authorized `kurier/dev-api` SST stage runs the existing seven-route Go
users/projects API on ARM64 Lambda behind API Gateway HTTP API in `us-east-2`.
`/api/v1/users/me` remains unchanged. The React frontend runs locally. The existing
`dev-auth` Cognito Essentials pool/public SRP client is reused and managed only
by its original stage. There are no execution workers/queues, Protected table,
evidence buckets, hosted frontend or full cross-store deletion in this slice.

## Deployment and startup

Require Node 22+, Go 1.26+, AWS CLI with the intended non-root session, and the
existing SST bootstrap. Infrastructure is retained/protected; Control enables
on-demand capacity, encryption, PITR, deletion protection, and three sparse
KEYS_ONLY indexes (GSI1 LPK/LSK, GSI2 DPK/DSK, GSI3 HPK/HSK).

```sh
npm ci
aws sts get-caller-identity
export KURIER_AWS_ACCOUNT_ID=747336059622
export KURIER_COGNITO_POOL_ID=us-east-2_qTDZQT1FE
export KURIER_COGNITO_CLIENT_ID=5jg9sb38i6ae9c0adm5rtdfkno
export KURIER_FRONTEND_ORIGIN=http://localhost:5173
npm run sst:diff:api
# Inspect the scoped plan before applying.
python3 scripts/ensure-cursor-key.py
npm run sst:deploy:api
npm run dev:cloud
```

The secure-key helper is Linux-specific: it generates 32 random bytes only on
first provisioning and supplies base64 through an inherited memory-backed file
descriptor to AWS CLI. It never prints, writes a disk file, or overwrites the
key. SSM `/kurier/dev-api/cursor-key` is a Standard SecureString under the AWS
managed SSM KMS key. Only the API role can read this parameter; cleanup cannot.
The key is decrypted once per cold start. Failed configuration/decryption refuses
startup. Retain it across deployments; rotation invalidates outstanding cursors
and requires recycling warm API environments. Do not export it into VITE values,
SST outputs, state, source, shell history, evidence, or MCP.

`dev:cloud` reads the latest public `.sst/outputs.json`, validates the stage and
API Gateway HTTPS origin, and starts Vite at the deployed configured origin.
Keep the page at that exact origin (default `http://localhost:5173`). CORS is
implemented in the existing Go wrapper, including unauthenticated preflight;
API Gateway CORS is omitted entirely to preserve strict origin/header checks.
SST `cors:false` alone produced an empty configuration that caused AWS to strip
Lambda CORS headers; the API transform removes that configuration. ETag,
Location and Retry-After are exposed; cookies are omitted. Application JWT
verification remains RS256/configured issuer/client_id/token_use=access/expiry/
subject with bounded HTTPS JWKS fetches. There is no auth bypass or Gateway-only
trust. API Gateway routes exactly the seven application routes plus preflight.
Local `/healthz` remains process liveness and is not a hosted readiness claim.
Gateway-native missing-route/throttling errors can differ from application error
envelopes; clients safely handle non-JSON errors and never replay writes.

API Lambda uses 256 MiB/15 seconds, cleanup 256 MiB/60 seconds, without VPC or
provisioned concurrency. Reserved concurrency is omitted because the account quota cannot reserve one
while leaving AWS’s minimum ten unreserved executions. Checkpoint/gate CAS safely
handles overlapping maintenance invocations. Gateway throttles at ten
requests/second with burst twenty. Logs retain seven days. Gateway logs contain
request ID, static route template, status and latency only; no request path,
query, headers, body, identity IP/user-agent or integration error text. Lambda
code never logs authentication payloads or raw database errors. SDK mutations
have no automatic retries; existing uncertain-write reconciliation is retained.

## Empty-project deletion

DELETE still commits active ->deleting and durable WORK in one guarded
transaction, removes listing keys, and returns 202. Scheduled cleanup runs every
minute and queries GSI2 `delete#0` through `delete#7` (SHA-256 project ID shard),
DSK `startedAt#operationId`, one page of at most 25 candidates/shard/tick. Durable
`MAINTENANCE#dev-api / DELETE#shard` page cursors advance even past blocked work
and reset at the end, revisiting late-visible/earlier entries on subsequent
cycles. Empty idle passes do not rewrite checkpoints. Index absence never
establishes deletion completeness.

Every candidate is strongly re-read and the existing cleanup routine verifies
stage/generation, durable work, project state/version/epoch and a strong base
partition query before conditional completion/work removal. Only META and the
exact WORK may exist; unknown children/additional pages/missing work keep deletion
pending. Restart after interruption is safe. No public cleanup endpoint or local
operator command is necessary in cloud. Scheduler/index lag adds latency, usually
up to a minute for an idle development stage, not a guarantee. The historical
`localEmptyProjectsOnly` marker now explicitly means this isolated empty-project
slice in both local and dev-api stages; no child/object writers may be enabled
under it. Arbitrary administrator writes outside gate CAS cannot be fenced.
Full Control/Protected/S3/late-upload draining remains later work.

The initial stage item is SST-owned with ignored subsequent item changes; runtime
never creates/reactivates it or resets its recovery generation. Recovery requires
reviewed state/generation changes before reopening. PITR is enabled; restore and
resurrection reconciliation are not validated by this task.

## Preserve local development

`npm run dev:api` and `npm run dev:auth` continue to use loopback Go/DynamoDB Local
with Cognito configuration from either development stage's public outputs.
`-init-local` and `-cleanup-project` remain loopback-only and cannot target AWS.
Fresh local tables include GSI2 for scheduled-cleanup integration tests. Existing
in-memory tables need no GSI2 for ordinary routes/operator cleanup; restart the
disposable container to initialize new tables for scheduler tests.
See [local setup](local-ownership.md) and [Cognito setup](cognito-local-projects.md).

## Checks and evidence boundaries

```sh
npm run verify
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration
# Actual AWS test; table must be the scoped deployed Control table:
export KURIER_AWS_TEST_TABLE=DEPLOYED_CONTROL_TABLE
cd services/api
go test -race -tags=integration,aws -run TestAWSControl -count=1 -v ./internal/ownership
```

The AWS test checks account/non-root identity and table/index readback before
mutations. It creates signed fixture users through the production ownership
store, observes GSI1 create/removal and GSI2 WORK propagation, tests owner isolation,
pagination, concurrent CAS, actual AWS failed-condition transaction cancellation
and rollback, unknown-child preservation, resumed cleanup and minimal tombstones.
It removes its fixture identity/user rows; name-free project tombstones remain.
Fixture JWTs never reach the public cloud endpoint. This establishes actual
DynamoDB behavior, not Cognito sign-in, browser CORS or runtime IAM.

Genuine browser validation requires passwords/codes only in the browser/inbox.
Never paste tokens, passwords or codes into chat, scripts, logs or screenshots.
Record only route/status/version and safe opaque IDs. Test sign-in/current-user,
project create/detail/rename/delete/poll-to-completion, pagination, stale writes
and isolation with a second independently signed-in user. A script imported in
the live local browser can use the existing memory-only session without printing
or persisting its tokens; do not install fixture credentials in runtime.

## Incremental monthly forecast

Before credits/free tier, budget $0.20–$1.00/month for 10,000 API calls, small
records and mostly idle minute cleanup; reserve $2/month during validation. Not
a hard cap or measured spend. Approximate rates: HTTP API $1/million calls;
Lambda $0.20/million requests and ARM compute about $0.0000133334/GB-second;
DynamoDB Standard $0.625/million write units, $0.125/million read units, storage
$0.25/GB-month and PITR $0.20/GB-month. Transactions, three index writes and
maintenance reads/checkpoints count. Idle cleanup makes about 43,200 monthly
invocations and roughly 0.56 million read units; assume 0.5-second average runtime
at 256 MiB ($0.072 compute plus $0.009 requests). Storage/PITR and logs depend on
actual volume. Standard SSM parameter storage/standard throughput and EventBridge
scheduled-rule invocation have no separately modeled fixed fee. Allow for KMS
requests, log ingestion/storage, HTTPS egress and shared SST assets/state; no
new bootstrap bucket/ECR repository or customer KMS key is planned. Existing
Cognito costs are unchanged and excluded from this increment.

Pricing references: [API Gateway](https://aws.amazon.com/api-gateway/pricing/),
[Lambda](https://aws.amazon.com/lambda/pricing/),
[DynamoDB](https://aws.amazon.com/dynamodb/pricing/),
[Systems Manager](https://aws.amazon.com/systems-manager/pricing/).

## Deployed resource inventory

| Resource          | Development identifier                                               |
| ----------------- | -------------------------------------------------------------------- |
| HTTP API          | `8dnymkcoa0`, https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com |
| Control           | `kurier-dev-api-ControlTable-bdxbaoxb`                               |
| API Lambda        | `kurier-dev-api-UsersProjectsApiFunction-werhekow`                   |
| Cleanup Lambda    | `kurier-dev-api-EmptyProjectCleanupFunction-bcmuzwku`                |
| Minute rule       | `kurier-dev-api-EmptyProjectScheduleRule-drmfctzc`                   |
| API role          | `kurier-dev-api-UsersProjectsApiRole-obmwtcha`                       |
| Cleanup role      | `kurier-dev-api-EmptyProjectCleanupRole-bcbvfhbb`                    |
| Cursor parameter  | `/kurier/dev-api/cursor-key` (SecureString, value private)           |
| API log group     | `/aws/lambda/kurier-dev-api-UsersProjectsApiFunction-orrftkzh`       |
| Cleanup log group | `/aws/lambda/kurier-dev-api-EmptyProjectCleanupFunction-conactwz`    |
| Gateway log group | `/aws/vendedlogs/apis/kurier-dev-api-DevelopmentHttpApi-eorfmchr`    |

SST also adds `/sst/passphrase/kurier/dev-api` (Standard SecureString), protected
app/stage state/link objects and code archives in the existing bootstrap state/
asset buckets. Existing bootstrap `sst-state-moshrdrrbeba` /
`sst-asset-moshrdrrbeba` and ECR registry are reused, not new product evidence
storage. Code archive versions are deployment assets. No new ECR image/runtime
container is used by this zip Lambda slice. Re-running secure configuration
retained cursor-key version 1; parameter values were never read into tool output.

## Actual results

SST preview and deployment created the scoped resources above using the existing
bootstrap, with no Cognito change. A first deployment stopped at the account's
ten-execution concurrency quota; dev-api-only refresh recovered the partially
created cleanup Lambda, then deployment completed without a quota change. A
real HTTPS probe exposed the empty-CORS issue above; it was fixed in SST.

Actual AWS Control test passed: all three indexes ACTIVE/KEYS_ONLY and on-demand
billing; GSI1 create/removal observations about 48 ms/query and GSI2 work visibility
about 55 ms/query in this run. These are observations, not lag guarantees. Actual
conditional cancellation/rollback, concurrent one-winner CAS, owner isolation,
pagination, unknown-child preservation and fresh-store cleanup resumption passed.
Fixture identities used actual AWS persistence, not public Cognito authentication.
PITR readback ENABLED with 35-day recovery window; restore itself untested.
Enabled minute schedule and correct Lambda target read back. An actual scheduled
Lambda invocation completed a durable fixture deletion in 22.6 seconds without
manual invocation/operator cleanup, verifying its runtime and transaction IAM.
Real HTTPS checks passed: OPTIONS 204 with exact origin/method/header and exposed
ETag/Location/Retry-After, missing-token users/me 401, foreign origin 403.

Repository verification passed (26 frontend tests, eight infrastructure fixture
tests, formatting, lint, frontend build and all Go tests/vet); local race and
Chromium fixture browser-to-Go integrations passed. The scheduler local test
recreated its store and completed an empty project behind 26 blocked children,
leaving every unknown child pending/preserved. ARM64 Linux build passed.

Davian confirmed genuine Cognito sign-in and the cloud-backed Projects screen.
The opt-in check in his real browser passed current-user lookup, 26 creations,
25-item pagination, detail/rename, stale rename/delete 412, hidden-after-delete
404, all 25 scheduled deletions and same-operation repeat DELETE. It used genuine
Cognito access tokens only in the existing memory-only session; no tokens,
passwords or codes were shared. A second independently signed-in Cognito user
passed cross-user isolation: the owner differed, the probe was absent from their
list, and read/rename/delete/operation lookup all returned 404. The original owner
then verified the probe name/version remained unchanged and its final deletion
completed through scheduled cleanup. Actual AWS strong read confirmed a deleted,
name-free project tombstone. All 26 browser fixture projects are deleted.
The opt-in browser helper uses the actual memory-only signed-in session:

```js
var cloudCheck = await import("/src/cloud-validation.ts");
await cloudCheck.validateCloudProjects();
```

It creates 26 projects, verifies 25-item pagination, detail/rename, stale rename
and stale delete 412, scheduled cleanup of 25 projects, hidden project 404, and
same-operation repeat DELETE. It returns only safe result fields plus
`isolationProjectId`/`isolationOwnerId`. Never rerun after an uncertain failure;
refresh/reconcile created projects first. One probe intentionally remains for
isolation. Sign out and sign in with a second confirmed Cognito account (use
signup/inbox verification if needed), then run:

```js
await cloudCheck.validateForeignProject(
  "PROBE_PROJECT_ID",
  "ORIGINAL_OWNER_ID",
);
```

It requires a different current user and checks list/read/rename/delete/operation
isolation. Finally sign back in as the original owner:

```js
await cloudCheck.cleanupIsolationProbe("PROBE_PROJECT_ID", "ORIGINAL_OWNER_ID");
```

This verifies that foreign writes left version/name unchanged, then deletes and
waits for scheduled completion. Only safe IDs/results may be shared; no tokens,
passwords or inbox codes. The helper is not imported by the production UI/bundle.

Full cross-store deletion, production signup/SES, restore/recovery, load/throttling,
live response-loss fault injection and full-day Cognito session expiry remain
untested or out of scope. Existing wrong-code and natural-refresh email evidence
is in the Cognito guide, not a new validation claim here.
