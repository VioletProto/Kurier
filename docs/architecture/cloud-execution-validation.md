# Cloud execution development rollout and validation

Contract: [Accepted execution/evidence contract](cloud-execution-evidence-contract.md).
Stage: `dev-api`, account `747336059622`, non-root `davian-admin`, `us-east-2`.
Branch: `feat/cloud-execution-evidence`. This record separates implementation,
simulated faults, AWS runtime checks and genuine browser checks.

## Predeployment preview

SST 4.17.1 preview generated on 2026-10-08. Reuses existing Control and Protected
tables, three keys-only indexes, HTTP API, API Lambda, maintenance Lambda/schedule,
KMS key and private signing parameter. No table replacement, Cognito change,
frontend hosting, VPC/NAT, additional customer key or unrelated stage change.
Existing SST bootstrap asset storage is reused; changed Lambda packages replace
old code artifacts in that storage. No new bootstrap deployment appeared.

Adds ARM64 worker (256 MiB, 90 seconds, no reserved concurrency), SQS queue
(four days, 540-second visibility, batch one, concurrency two, five receives)
and fourteen-day DLQ, a private SSE-S3 bucket with public-access blocking,
bucket-owner enforcement and TLS-only policy, and six execution routes.
Adds one development-only owned endpoint Lambda (128 MiB, no data-resource
permissions) and route for controlled echo/error/redirect/size/gzip/HTML/deadline
tests. Access logs retain static route template/status/latency only. Payloads
and secrets are never logged by these handlers. Log retention is one week.

API permissions add saved-secret KMS Decrypt and job-bindings GenerateDataKey,
queue send, private evidence read and protected-binding cleanup. Worker decrypt
is restricted to job-bindings context. Maintenance has no KMS permissions;
its S3 access supports publication recovery and exact-key/residual cleanup.
S3 object permissions are limited to `dev-api/projects/*`; maintenance list
permission requires that prefix. No presigned delivery or public bucket access.

Rollout will deploy all consumers before atomically upgrading the existing
stage capability from saved requests two to three, with cloud execution one,
execution inputs one and unchanged protected-secret one/recovery generation.
The old binaries reject capability three. Deployment and capability activation
were verified after the approved development concurrency clarification below.

## Deployment blocker and account-plan verification

The initial authorized deployment created the private evidence storage, queues,
controlled endpoint and execution routes, and updated API/maintenance code and
scoped IAM. Worker creation reached its reserved-concurrency configuration,
which AWS rejected: this account has ten concurrent executions and requires
ten to remain unreserved. No reserved concurrency was applied.

A minimal quota request of twelve was rejected by Service Quotas because its
request value must exceed the default of 1,000. No request for 1,001 was submitted.
Read-only `freetier get-account-plan-state` confirmed account `747336059622`
has an active **FREE** plan. Eligibility for that larger quota on this plan or
under any applicable student-program restrictions has not been established.
Per Davian's instruction, quota submission was stopped. No billing upgrade or
credit-arrangement change was performed.
The [AWS plan documentation](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/free-tier-plans.html)
describes restricted service access on the Free plan; it does not establish
eligibility for this specific Lambda quota request. This is an eligibility
blocker, not a claim that an upgrade has been conclusively shown necessary.

Davian subsequently approved standard-mode SQS event-source maximum concurrency
**two**, batch size **one**, without worker reserved concurrency. This caps
queue-driven concurrent invocations without reserving account capacity. It
does not guarantee shared capacity or cap authorized direct invocations. No
provisioned Lambda concurrency or provisioned pollers are enabled. Account quota
and Free plan must remain unchanged. AWS throttling may delay delivery; retry
delivery is fenced, expired queued work fails without dispatch after ten minutes,
and post-intent recovery never replays outbound HTTP.

SST refresh reconciled the partially created worker's missing ARN in state. The
subsequent scoped preview reused storage, queues, IAM and routes; it updated
the four development Lambda code packages and created only the missing SQS
mapping. No new bootstrap resources or additional fixed capacity cost appeared.
Deployment completed. Read-only verification confirmed one enabled SQS event
source with maximum concurrency two, batch size one and partial batch failure
reporting; no reserved/provisioned Lambda concurrency, provisioned pollers,
function URL, API worker integration, invocation resource policy, EventBridge
target or Scheduler target. Authorized IAM direct invocation remains possible.
Account concurrency and unreserved capacity both remain ten, with an active
Free plan. `scripts/verify-cloud-execution-config.mjs` repeats these checks.

All three consumers were Active with successful updates and schema capabilities
one before conditional stage activation. Read-back verified saved-request
capability three, cloud execution one, execution inputs one and unchanged
protected capability/recovery generation.

## Incremental monthly cost preview

Planning estimate: **about $1/month; allow $1–$3/month** for light development,
above existing resources, without subtracting free tiers. This is a forecast,
not a hard billing cap. Assumptions: 1,000 executions/month, two seconds average
worker time, three protected references, ten status/history/detail reads and
two evidence reads per execution, 1 MiB average evidence retained thirty days,
approximately 0.2 GiB additional DynamoDB storage including indexes/bindings,
1 GiB incremental log ingestion, and up to 2 GiB internet delivery. The existing
minute maintenance invocation adds roughly half a second per tick for bounded
execution discovery; idle SQS polling assumes up to five twenty-second pollers.
No provisioned concurrency or enhanced SQS poller mode is configured.

| Incremental item                                                  | Approximate monthly USD |
| ----------------------------------------------------------------- | ----------------------: |
| Worker/API/maintenance Lambda compute and requests                |                    0.09 |
| SQS sends/receives/deletes including idle polling                 |                    0.26 |
| DynamoDB transaction/read units, storage and existing PITR growth |                    0.30 |
| S3 storage/PUT/GET/list                                           |                    0.03 |
| KMS calls at three protected sources per execution                |                    0.02 |
| HTTP API requests                                                 |                    0.02 |
| Logs and internet transfer allowance                              |                    0.70 |
| Estimated total                                                   |                    1.42 |

Official us-east-2 price catalogs retrieved 2026-10-08: ARM Lambda
$0.0000133334/GB-second and $0.20/million requests; DynamoDB standard
$0.625/million WRUs and $0.125/million RRUs (transaction multipliers apply),
storage $0.25/GiB-month and PITR $0.20/GiB-month; S3 Standard $0.023/GiB-month,
$0.005/1,000 PUT/list and $0.0004/1,000 GET; KMS $0.03/10,000 symmetric calls;
HTTP API $1/million requests; CloudWatch standard logs $0.50/GiB ingest and
$0.03/GiB-month stored. SQS Standard uses $0.40/million requests. Existing
key, API, scheduler, tables and bootstrap fixed costs are not new costs.
Larger captures, slow endpoints, persistent polling, unresolved cleanup,
or additional log/transfer use can exceed this estimate.

Sources: [AWS regional price catalogs](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/using-the-aws-price-list-bulk-api-fetching-price-list-files-manually.html),
[Lambda](https://aws.amazon.com/lambda/pricing/),
[DynamoDB](https://aws.amazon.com/dynamodb/pricing/),
[S3](https://aws.amazon.com/s3/pricing/),
[SQS](https://aws.amazon.com/sqs/pricing/),
[KMS](https://aws.amazon.com/kms/pricing/),
[HTTP API](https://aws.amazon.com/api-gateway/pricing/),
[CloudWatch](https://aws.amazon.com/cloudwatch/pricing/).

## Verification ledger

- Passed: repository formatting, lint, 39 frontend tests, eight infrastructure
  tests, frontend build, Go tests/vet across all five modules. Repeated after
  the latest frontend/runtime edits and account-plan blocker documentation.
- Passed: actual DynamoDB Local transactions with **simulated** KMS/S3/SQS
  and controlled loopback HTTP. Duplicate delivery, source/cross-operation
  idempotency, lost admission/claim/intent/queue/PUT acknowledgments, pre-intent
  interruption, post-intent publication interruption, known-upload publication
  recovery, frozen secret replacement and revocation races, expired retention,
  pinned preservation, upload finishing after deletion begins and uncertain PUT
  preservation. Pure runtime tests cover public/mapped/special-use IPs, mixed
  DNS and rebinding checks, redirect refusal, wire/gzip limits, redaction,
  unsupported content, worst-case escaping, API-envelope overflow and base64
  proxy delivery/metadata limits. Unsupported response encoding also retains
  the wire cap and marks interrupted reads unknown; those tests passed.
- Passed: real AWS KMS source encryption and job-bound decryption, DynamoDB
  admission/publication, SQS/deployed worker/HTTPS/S3, immutable evidence after
  duplicate queue delivery, controlled echo/500/redirect/wire limit/gzip limit/
  HTML omission/deadline modes, and production execution cleanup. Disposable
  secrets were absent from evidence and fetched logs from all four configured
  Lambda log groups (8 API, 27 worker, 3 maintenance, 18 endpoint events in the
  passing run). Values were never printed. The initial test stopped because
  inferred log names differed from SST's configured groups; the corrected test
  passed, and its interrupted disposable fixture cleanup was separately resumed
  and verified. Admission in these tests uses the service/SDK, not Cognito HTTP.
- Passed: actual bucket public-access blocking, SSE-S3 encryption and TLS-only
  policy, queue visibility/retention/redrive/encryption read-back. IAM policy
  **simulation** denied worker saved-secret decryption and evidence read/write
  outside the stage prefix; deployed worker's successful job-bound decryption
  exercises its real role. These simulations are not actual denied API calls.
- Passed: local delayed-delivery **simulation** (two minutes dispatches once;
  eleven minutes expires without HTTP). Actual shared-account throttling was
  not induced; fencing and post-intent fault tests remain simulations.
- Passed: real AWS ownership/protected regressions after adapting historical
  fixtures and restart/race stores to configure the accepted capability-three
  consumer. Control conditional rollback/CAS, pagination, isolation, unknown-child
  preservation, resumed cleanup and actual scheduled cleanup passed. Actual KMS
  context rejection and scheduled 25-request cross-table cleanup passed (about
  three minutes for the latter drain). Initial old fixtures correctly failed the
  capability fence; they were updated without weakening the production guard.
- Reported by Davian: genuine browser saved GET submission to the owned echo
  endpoint, Completed status and evidence inspection. His first capture contained
  no protected fields, so it does not establish browser secret containment.
  Response wrapping was requested and fixed with escaped preformatted text,
  wrapping long tokens and bounded vertical scrolling. Davian subsequently
  confirmed the response text wraps in the genuine browser UI.
- Subsequently reported by Davian: protected Authorization/custom fields and
  their reflected response values are masked, with two distinct execution IDs.
  Actual private-S3 read-back verified distinct immutable captures: first
  duration 372 ms, second 71 ms, different upstream request IDs and completion
  times. Davian confirmed the repeated pasted capture was accidental; no stale
  UI capture defect was established.
- Passed: read-only containment audit of those genuine browser executions,
  comparing two protected sources and 236 application log events against raw
  and supported encoded variants. Source values were resolved only in authorized
  test-process memory; neither values nor log contents were printed or persisted.
  This audit establishes server evidence/log containment.
- Reported by Davian: the genuine signed-in browser containment probe passed
  protected evidence, immutable reopen, history, stable submission-retry identity
  and separate deliberate-rerun identity. Browser persistence audit was complete
  and safe: one local-storage entry, one session-storage entry, no IndexedDB
  databases/records or cache entries. Entries were inspected without exposing
  their contents and unrelated browser data was preserved.
- Passed: separate read-only server containment audit for that browser probe:
  two executions, two protected sources and 99 application log events; no raw
  or supported encoded values found. No source values/log contents printed or
  persisted.
- Reported by Davian: deleting the disposable probe project through the UI
  removed access immediately and initially showed cleanup pending. Independent
  read-only AWS checks then verified scheduled physical cleanup completed:
  `projectTombstone` with completed deletion operation, one Control record,
  zero Protected records and zero S3 evidence objects, with complete query/list
  pagination and a durable execution residual sweep registered. No unrelated
  project was deleted and no records were manually removed to force completion.
- Passed: shared [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
  and [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
  now include accepted cloud-execution continuations, source-bound idempotency,
  evidence limits, approved Free-plan concurrency configuration and verified
  results. Native read-back confirmed prior content, tabs, inline objects and
  source-link typography were preserved. Other proposed routes remain proposals.
- Reported by Davian: all three genuine browser controlled failure checks
  passed. Execution `1387546e-52a0-4ae1-b1b2-960c3052ece5` showed failed/HTTP 500;
  `b90d13c9-fc12-4de1-b7ed-df1b1dd27318` showed completed/HTTP 200 with HTML
  safely omitted and no popup; `9753ac46-db43-466e-8062-25504009bf13` showed
  failed/HTTP unavailable with the unknown-outcome timeout warning. The owner
  left that disposable project available for inspection. These are genuine
  user-reported browser checks, distinct from the earlier AWS fixture suite.
- Passed: fresh independent read-only AWS inspection of those three executions
  after session renewal. All belong to disposable project
  `f800e37d-b1b8-443d-872c-453f52551035`. Strong Control reads and private S3
  captures matched execution IDs, terminal state, HTTP status and outcome in
  both execution and immutable snapshot metadata; all three stored lengths and
  SHA-256 checksums matched. HTTP 500 was `upstream_http_error`/known with JSON;
  HTTP 200 HTML was completed/known with an omitted body and omission reason;
  timeout was `execution_timeout`/unknown, HTTP unavailable and no response body.
  No values or response bodies were printed or written to inspection files.
  The first inspection helper assumed JSON-style names for the stored summary;
  it was corrected to the actual Go DynamoDB field names and rerun successfully.
  The final fixture project was left intact; the earlier probe already verified
  owner-driven deletion and physical cleanup. This slice's requested browser
  checks are complete. Account throttling and dispatch/crash fault injection
  remain simulations, and negative IAM checks remain policy simulations.
  The browser probe is opt-in, uses the genuine memory-only session and reports
  only safe identifiers/booleans/counts.

## Genuine browser checklist

Use `npm run dev:cloud` after deployment, then open `http://localhost:5173`
and sign in with the existing development Cognito account. Create a disposable
project and saved request targeting the **owned** URL from SST's
`controlledEndpointUrl`. Create random disposable secret values directly in
the browser, mark their fields Protected before entering them, and never paste
the values into chat, logs or screenshots. Keep values only in test memory.

1. Save a request with an Authorization header, a custom protected header,
   a protected query value and/or a protected JSON scalar. Execute the saved
   revision, observe queued/running/terminal state and inspect evidence.
2. Confirm status/timing and the sanitized request/response capture. Known
   reflected values must be masked; no raw protected value should appear in
   API read responses, evidence, browser storage or application logs. A
   containment helper must report only booleans/counts, never values.
3. Refresh history and reopen the same immutable evidence. Edit the live
   request: the old execution continues to show its frozen revision.
4. Review and confirm a deliberate rerun; confirm a different execution ID.
   A retry of an unresolved admission keeps the original key/options in this
   tab. Do not manufacture a new key to reconcile an uncertain submission.
5. Use owned endpoint modes `http-error`, `redirect`, `oversize`, `gzip-limit`,
   `html`, and `delay` (one-second deadline for delay). Confirm clear known
   HTTP failure, redirect capture, size omission, HTML omission and unknown
   timeout states. No redirect is followed. Do not assume an unknown outcome
   means the external operation had no effect.
6. Delete the disposable project and verify immediate access denial followed
   by physical cleanup completion. Unknown records/PUT obligations must keep
   cleanup pending; do not remove them to make the checklist appear complete.

Browser and AWS checks remain pending until actually performed. Automated
browser fixtures and direct AWS SDK tests do not count as Davian's genuine
browser validation or independent Cognito/email validation.

Optional DevTools Console probe while signed in:

```js
await (
  await import("/src/cloud-validation.ts")
).validateCloudExecutionEvidence();
```

This creates a disposable project and two protected random values in memory,
checks immutable evidence, retry identity, a deliberate rerun of a known-successful
owned GET, history and browser storage. It returns safe IDs and audit results.
Keep its project until evidence inspection and log audit are finished. Never
paste values or tokens into the console, chat or screenshots.
