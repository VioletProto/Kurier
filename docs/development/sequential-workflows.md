# Sequential cloud workflows: implementation and deployment review

Davian accepted the [revised contract](../architecture/sequential-workflows-contract.md)
at `25158b7` and authorized this implementation, external API contract
synchronization, meaningful local validation and scoped deployment/activation
preview. **AWS deployment and capability activation remain held for review.**
No workflow runtime success on AWS or genuine Cognito/user-operated browser
validation is claimed by this implementation.

## Implemented behavior

Nine project-owned definition/run routes, revisioned whole-step-array editing,
submission-time frozen plans and encrypted initial inputs, typed required JSON
extraction and explicit earlier-step bindings. Header string bindings support
literal prefix/suffix only (256 UTF-8 bytes each, 2 KiB rendered value). Query
and JSON bindings preserve the accepted typed behavior. An empty public
Authorization header is the saved placeholder; nonempty credential values still
require protected writes. No general template evaluator, scripts or local
workflow protocol was added.

All runtime values are encrypted under the existing stage key with opaque
project/run/producer/purpose contexts. Extraction happens before raw disposal;
source paths and both raw/rendered reflections are redacted, including a
sensitive:false value consumed by Authorization. Runtime ciphertext is removed
at terminalization regardless of pins. There is no variable reveal API. Collection limit inputs allow up to 100,
with a 25-record server page budget so maximum-size rules remain deliverable;
signed cursors continue the collection.

Creation commits run/steps/inputs/receipt and only the first job/outbox.
Finalization atomically publishes evidence/variables/current step/run and a
single deterministic successor/outbox, or fails the run/skips successors/removes
runtime material. Lost acknowledgments retry only exact workflow transactions;
standalone protocol retry behavior is preserved. After intent, a crash losing
outputs produces unknown failure with no automatic HTTP replay. An older recovery
generation is terminalized as recovery_interrupted without sending HTTP.

Run admission now persists server-derived `workflowSubmission: definition` or
`frozen_rerun`; the latter also records `rerunSourceRunId`. Only deliberate
whole-run rerun admission can set frozen provenance, after owned retained-context
authorization and current request/secret checks. Its creation transaction guards
the source run version against concurrent cleanup. The new run owns its copied
context; it does not depend on the original remaining retained after admission.
Missing provenance means an ordinary definition-backed run; malformed or unknown
provenance fails closed.

Preparation and first intent require an active definition only for an ordinary
run that has not started. The definition CAS shares the durable first-intent
transaction, so deletion winning first blocks HTTP; intent winning first lets
the frozen sequence continue. A frozen-context rerun can start after definition
deletion, and still records its first intent. All required requests, including
later-step requests, are checked before preparation and fenced at intent;
existing user/project/secret/recovery guards remain in place. A lost
pre-intent race can leave a claim until lease expiry; subsequent delivery
revalidates authorization without replaying committed HTTP.

Each step's original evidence expiry and individual pins are independent of
run context. RET claims update the step's evidenceExpired and run counters in
one transaction. Evidence cleanup preserves step records until run-context
cleanup; pins cannot resurrect a claimed step. Expired sibling bodies are
removed while pinned/available siblings protect nonsecret context. Project
deletion overrides pins and drains workflow records through existing two-store/
ticket/S3 fences; unknown records and uncertain PUTs keep cleanup pending.

React adds an opt-in workflow panel, ordered saved-request editor, extraction/
binding selectors, public header literals, submission/whole-run rerun, polling,
safe step evidence and individual pin controls. It shows source revisions,
unknown-outcome warnings and skipped/expired steps without variable values.
Submission uncertainty retains the original key/consent for explicit
reconciliation. No browser persistence was introduced.

## Local validation and how to run it

From the repository root, with Docker, Node dependencies, Go and Playwright
Chromium available:

```sh
npm install
# Loopback-only, in-memory DynamoDB; no AWS credentials required for these tests.
docker compose -f compose.dynamodb.yml up -d
npm run verify
cd services/api
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 go test -race -tags=integration -count=1 ./...
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 go test -race -tags=integration,browser -run TestIntegrationBrowserProjects -count=1 ./internal/ownership
cd ../request-runtime
go test -race ./...
```

The Go browser harness starts a temporary API and Vite on localhost:5175, uses
Chromium with tracing/screenshots/video disabled, provisions disposable Local
records and cleans up its fixture tables. Keep that port free. It includes
existing project/pinning/local-agent browser regressions and the new workflow
editor/ordering/Bearer-prefix/save/submission/reload-signout flow. Its workflow
submission stays queued: it does **not** pretend to execute cloud HTTP. The
separate worker integration fixtures execute controlled loopback HTTP through
an explicit test-only transport; production public-destination restrictions are
unchanged. The ordinary `dev:api` users/projects launcher is not a workflow/KMS/
S3 emulator. Do not point a manual browser at the deployed API expecting
workflows before approved deployment/activation.

Verified locally during this task:

- Prettier/gofmt, ESLint, 52 frontend tests, ten infrastructure checks,
  TypeScript/Vite production build and six Go modules' tests/vet.
- Race-enabled complete API/DynamoDB Local integration suite, including existing
  standalone cloud/local/ownership regressions. Simulated KMS/S3/SQS faults are
  separate from genuine AWS behavior.
- Restricted grammar (quoted/escaped/empty property names, root/null, rejected
  operators), typed query/JSON bindings, exact/over scalar/header/literal bounds,
  CR/LF/control rejection, HTTP-secret consent and raw/rendered containment.
- Duplicate admission/delivery, concurrent workers, missing/revoked sources,
  freeze-limit rejection before accounting, all-N admission/no skipped refund,
  ten-step terminal action/byte budget and inflight release.
- Simulated crash before atomic finalization, lost final ACK, durable successor
  outbox recovery, unknown/no-replay, deletion during S3 publication and restored
  generation interruption. Pin/expiry contention, pinned context with expired
  sibling cleanup, expired last unpin and subsequent context/project draining.
- Deleted-definition completed-run rerun executes the frozen sequence despite
  subsequent live request edits; ordinary queued and legacy unmarked runs send
  no HTTP after definition deletion. Required future-request deletion and secret
  revocation block rerun admission or dispatch, including at first intent.
  Both definition/frozen origins cover deletion-before/after-intent with
  concurrent duplicate delivery, exact send counts and quota/inflight accounting.
  Source-context deletion between retained read and rerun admission rejects the
  entire admission without consuming quota.
- Actual Chromium-to-Go/DynamoDB Local browser suite (five scenarios); Cognito,
  KMS/S3 and target-execution trust remain fixture boundaries.

A sandbox run hit read-only Go build-cache/listening-socket restrictions;
approved outside-sandbox Go runs passed. Fixture tests are not AWS/IAM proofs.

## External synchronization

The existing [native API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
received an accepted workflow supplement with concrete routes/DTOs, grammar,
header literals, sensitivity, transaction/retention/accounting rules and rollout
hold. A revision-guarded native insertion preserved all prior document text and
was read back, including its heading style. Historical Proposed workflow text
is explicitly superseded for this slice; unrelated proposals remain unchanged.
The capstone proposal was read during preparation and was not rewritten.

## Live AWS inspection and proposed deployment

Read-only STS verified account `747336059622`, non-root `davian-admin`, us-east-2
after Davian renewed the expired CLI session. Inspection found:

- Free plan **ACTIVE**; account/unreserved Lambda concurrency **10**.
- Worker standard SQS only, batch **1**, max concurrency **2**, no reserved or
  provisioned concurrency/pollers.
- API/cleanup/worker active/successful ARM64, 256 MiB each; existing timeouts
  15/60/90 seconds. No compute/concurrency setting change is proposed.
- Stage marker active/version **7**, savedRequests **5**, local **1**, pinning
  **1**, recovery generation `dev-api-initial-v1` preserved.
- Six projects, zero queued/claimed/running jobs, zero workflow records and zero
  unsettled upload tickets in the bounded complete Control/Protected inventory.
  This observation expires; repeat it under the closed activation gate.

The authorized SST 4.17.1 diff was generated after confirming the existing
`/sst/bootstrap` metadata. No new bootstrap is permitted implicitly by the
preview helper. It proposes:

| Area               | Planned change                                                                                                                                                                                                                  |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| HTTP API           | Nine workflow routes, integrations and existing-API Lambda invocation permissions. Existing Go Cognito/ownership middleware applies.                                                                                            |
| API IAM            | Existing stage-key GenerateDataKey restricted to app/stage/purpose workflow-inputs. Existing saved-secret permissions retained.                                                                                                 |
| Worker IAM         | Protected-table PutItem for encrypted outputs, existing queue SendMessage for postcommit fast notification; stage-key Decrypt for workflow-inputs/output and GenerateDataKey for workflow-output, app/stage/purpose restricted. |
| Consumers          | Existing API/cleanup/worker code packages and workflow-capability environment; API/cleanup saved schema environment 5→6. Shared controlled-endpoint worker package asset also changes.                                          |
| Standing resources | No new tables/indexes/queues/buckets/KMS keys/schedules/hosting/Cognito or account/plan/quota/concurrency changes. Versioned code assets use existing bootstrap storage.                                                        |

The diff includes six updates (two IAM roles/four existing functions) and the
nine routes' component/integration/permission resources plus code assets. No
product deployment or marker write was performed. Rebuild/re-diff if runtime
code changes after preview; code hashes from an earlier preview are not release
verification.

The deleted-definition rerun fix was rebuilt and previewed against the same
live stage before deployment. Proposed code archive SHA-256 values are
`b3a3d8a870817280200b00d2cfdb810d896c68847c9feedf4b84bb024d4e286a`
for API/cleanup and
`f28f4eea35b1362d5276f4d4925b21e8200f48442baefeb127eac79ffb7e1a0c`
for worker/controlled endpoint. These identify preview ZIP artifacts, not
deployed Lambda CodeSha256 readback. Resource/IAM scope and live inventory/
plan/concurrency observations above were rechecked unchanged. No deployment or
activation occurred. This fidelity fix changes internal metadata and guards;
the accepted external API contract requires no change.

Repeat read-only previews with the existing public `.sst/outputs.json` and an
active CLI login:

```sh
node scripts/preview-sequential-workflow-activation.mjs
node scripts/preview-sequential-workflow-deployment.mjs
```

Both helpers are preview-only and have no deploy/activate argument. The latter
runs the scoped `sst diff` through the existing in-memory CLI credential bridge;
it never prints or persists AWS credentials.

Proposed activation after explicit authorization: verify deployed code hashes,
consumer readiness, generated purpose-scoped IAM and all nine route targets;
re-read Free plan/concurrency; require quiescent jobs and settled tickets. CAS
active/version7/generation/current capabilities to workflow-migration/version8.
Under the closed gate, repeat both-table inventories and accounting checks.
CAS the same generation to active/version9/savedRequests6/
sequentialWorkflows1, preserving every existing local/cloud/pinning capability,
project/stage counter and stored record. No resets or secret re-encryption. If
any proof fails, leave the stage closed and review; never reopen automatically.
The exact versions are an observed preview, not hard-coded future authority.
An activation mutation command is intentionally not provided before deployment
review and deployed artifact verification.

## Refreshed incremental development cost

Official Ohio public catalogs were fetched on 2026-10-14 for
[Lambda](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSLambda/current/us-east-2/index.json),
[DynamoDB](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonDynamoDB/current/us-east-2/index.json),
[KMS](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/awskms/current/us-east-2/index.json),
[HTTP API](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonApiGateway/current/us-east-2/index.json),
[S3](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/us-east-2/index.json)
and [SQS](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSQueueService/current/us-east-2/index.json).
Lambda/S3 catalogs were published 2026-10-14; DDB/KMS/SQS 2026-09-11.
Rates remain ARM $0.0000133334/GB-s +$0.20/m requests; DDB Standard
$0.625/m WRU, $0.125/m RRU, $0.25/GB-month storage +$0.20 PITR +$0.10 backup;
KMS $0.03/10k symmetric operations; HTTP API $1/m at this tier; S3 Standard
$0.023/GB-month, $0.005/1k PUT, $0.0004/1k GET; standard SQS $0.40/m.
No eligibility/free-tier/credit subtraction. Existing key/maintenance fixed
charges are not incremental. Egress $0.09/GB is a conservative planning input,
not newly measured workload billing.

100 three-step runs (300 HTTP executions) fit inside the accepted existing
1,000-step light budget; do not charge their ordinary worker/evidence costs a
second time. Refresh the overhead allowance to account for actual strong
run/step/RET hydration: 20k extra API calls at 100ms/256 MiB, up to 1m extra RRU
and 200k WRU after transaction/item rounding, 0.1 billed GB extra metadata/
PITR/backup, 2k KMS operations, 100 extra worker GB-s and <=1 GB additional views:

| Increment                                    | Monthly estimate                        |
| -------------------------------------------- | --------------------------------------- |
| HTTP API                                     | $0.020                                  |
| API/worker compute and request overhead      | $0.012                                  |
| DynamoDB requests                            | $0.250                                  |
| Metadata/PITR/backup                         | $0.055                                  |
| Extra KMS operations                         | $0.006                                  |
| Evidence delivery                            | $0.090                                  |
| Subtotal                                     | About $0.43                             |
| Logs/cleanup/retry/bootstrap-asset allowance | Round to **$0.50–$1/month incremental** |

The rerun fix adds a small provenance field/source-run condition check and
bounded reads/intent checks for at most ten distinct required requests; these
fit the conservative request allowance above. It adds no standing resources
or compute/concurrency settings. The planning range remains unchanged.

This remains the accepted planning range with a more conservative read budget,
not observed spend or a spending cap. Current worker is 256 MiB: adding 300 new
5-second HTTP steps beyond the baseline adds 375 GB-s (~$0.005 compute), plus
normal step storage/SQS/S3/crypto/egress. Larger plans/captures, more polling,
failed admission, long HTTP, accumulated pins, rotations/shared stages and abuse
can exceed the model. No billed-usage conclusion is inferred from tests.

## Remaining review and validation gaps

Deployment/activation and workflow execution against real AWS KMS/DDB/SQS/S3/
IAM/scheduled maintenance are unperformed and require the held approval.
No genuine Cognito/operator workflow browser report; the browser suite is an
actual browser against fixture auth and Local stores. Genuine cloud crash/lost-
response injection, negative IAM, PITR/restore, thirty-day elapsed retention,
real clock skew, maximum-size Lambda memory/latency and metered cost remain
unperformed. Existing uncertain-PUT settlement still requires reviewed operator
reconciliation, never an automatic replay or invented absence proof.

Final review should inspect the scoped diff and choose whether to authorize
development deployment and the guarded capability transition. Local fixtures
establish the implemented protocol behavior, not those operational guarantees.
