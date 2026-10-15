# Cloud-only sequential workflows: review packet

Status: **Proposed**. Prepared for Davian on 2026-10-14. Publication is not
acceptance. Wait for explicit acceptance before dependent implementation or
external contract synchronization, and for explicit authorization before AWS
deployment. No workflow behavior or future fault test is claimed implemented.

Branch `feat/sequential-workflows` starts at refreshed `origin/main` `42fe9e5`,
which merges [local-agent PR #9](https://github.com/VioletProto/Kurier/pull/9).
The initial working tree was clean. Local-agent implementation and operator
validation are merged; deferred checks on its card are not prerequisites for
cloud workflows and are not retroactively declared passed.

Authority: [accepted architecture](README.md), [ADR 0001](../decisions/0001-sst-infrastructure-candidate.md),
[ADR 0002](../decisions/0002-serverless-persistence-topology.md),
[atomic workflow design](w1-system-design.md#atomic-workflow-advancement),
[workflow limits/context](w1-data-model.md#bounded-transactions-and-workflow-limits),
[saved requests](saved-requests-contract.md), [protected inputs](protected-request-secrets-contract.md),
[cloud execution](cloud-execution-evidence-contract.md), [pinning](evidence-pinning-contract.md)
and [local-agent contract](local-agent-execution-contract.md).
The live [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
and [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
were fetched and read. Their workflow grammar/CRUD details remain Proposed.
Their historical local-agent deployment-pending wording is superseded by the
merged repository validation record; no external text was changed here.

## Scope and acceptance boundary

Preserve accepted 2–10 ordered saved-request steps, immutable submission-time
freezing, encrypted runtime outputs, atomic advancement, no ambiguous HTTP
replay and conditional workflow context retention. Exact routes, binding/path
syntax, scalar semantics, admission accounting and compatibility markers below
are new **Proposed** choices, even when they refine accepted behavior.

First slice: cloud execution only, sequential, stop on failure, request and
protected-source reuse, explicit extraction/bindings and minimal React editing,
submission and inspection. Reject local targets/agent IDs, environments,
assertions/schema options and unsupported fields rather than silently ignoring
them. Local workflows, branching, loops, parallel steps, arbitrary scripts and
MCP are deferred. The broader proposal's status/JSON assertions, environments,
OpenAPI validation and scheduled runs remain backlog; this first slice does not
claim the complete workflow MVP or introduce those features implicitly.

## Definition, ownership and revisions

Use existing verified Cognito owner identity, active stage/recovery generation,
enabled user and project gate/version/deletion epoch on every read/write.
Every workflow, request, secret and run locator is project-scoped; foreign,
cross-project, deleted or inaccessible resources return fixed 404. No caller
owner ID and no agent-token authorization. All responses are no-store.

Proposed routes below are relative to `/api/v1/projects/{projectId}`:

| Route                                       | Request and result                                                                                                                            |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| POST `/workflows`                           | Strict definition body; 201 `{data:{workflow}}`, Location, ETag. No automatic creation retry.                                                 |
| GET `/workflows`                            | Existing signed bounded owner pagination, default 25/max 100; 200 `{data:{items,nextCursor}}`.                                                |
| GET `/workflows/{workflowId}`               | 200 workflow envelope and revision ETag.                                                                                                      |
| PATCH `/workflows/{workflowId}`             | Required quoted revision If-Match; partial name/steps replacement; omission preserves, steps replaces entire array. 200/new ETag.             |
| DELETE `/workflows/{workflowId}`            | Required If-Match; 204 logical tombstone, preserve frozen runs. Retained initiating-version repeat succeeds; later missing definition is 404. |
| POST `/workflows/{workflowId}/runs`         | Required workflow If-Match and Idempotency-Key; `{target:"cloud",allowInsecureSecrets:false}`; 202 run envelope, Location.                    |
| GET `/workflow-runs`                        | Same bounded pagination; optional workflowId filter validated under project; retained deleted-definition runs remain inspectable.             |
| GET `/workflow-runs/{workflowRunId}`        | 200 `{data:{run,serverTime}}` with all <=10 ordered step descriptors; safe frozen context. Poll this route.                                   |
| POST `/workflow-runs/{workflowRunId}/rerun` | Fresh Idempotency-Key and explicit cloud/HTTP-secret consent; 202 new run from frozen public plan/current exact stable secret refs.           |

Definition DTO adds workflowId, revision (starts 0), createdAt and updatedAt to
`{name,steps}`. Names follow existing 1–100-code-point rules. Each step has
`{stepId,requestId,requestRevision,extract,bindings}`. stepId is a client-generated
UUID unique within the definition, stable across reorder; never an authority
token. requestRevision is a nonnegative integer. Repeated use of one request is
allowed with distinct step IDs; inconsistent revisions for one request reject.
Save validates same-project current request revisions and graph/grammar; submit
revalidates. Saved workflows contain references/rules, not copied credentials.

PATCH cannot clear steps to null/empty or assign server fields. Missing If-Match
428, malformed 400, stale 412; missing admission key 400; mismatched reuse 409;
quota/rate 429 with existing retry guidance; bounded input overflow 413;
unresolved service/write outcome 503. Syntax errors are fixed 400
invalid_request; invalid references/rules are fixed 400 validation_failed.
Do not echo submitted values, paths, URLs or upstream errors in diagnostics.
Strict UTF-8 JSON rejects duplicate/unknown fields and trailing data.

Workflow deletion before first durable dispatch intent fails an unstarted run
with source_unavailable, skips pending steps and performs terminal cleanup.
After the first intent, a run continues from its frozen definition, subject to
request/secret revocation and project/recovery fences. Workflow edits never
change accepted runs. Request deletion blocks future dispatch/advancement;
request edits and shared-secret replacement do not change accepted values.
Definition links become null when deleted; frozen IDs/revisions remain.

## Submission freezing and idempotency

Strongly read workflow and deduplicated request/source revisions. Freeze every
step's complete public request plan, extraction/binding rules, source IDs and
revisions, redaction policy, timeout and consent at submission, not when its
job eventually starts. Decrypt authorized saved inputs in API memory and
freshly encrypt immutable run inputs; do not resolve replacements later.
Later extracted inputs refer to exact producer step/version, never a mutable
latest-value map. Check current source eligibility at dispatch and successful
advancement with conditional observed versions; revocation wins by the shared
project/source gate. Already sent HTTP cannot be recalled.

One bounded creation transaction puts run/all step identities, frozen plans,
encrypted initial inputs, receipt and only the first execution/job/OUT;
reserves admission and inflight accounting. Non-first steps are pending with
null executionId. Never create ten runnable jobs in advance. Step/job/anchor
identity is deterministic from run ID/position; conditional puts prevent repeats.
Freeze separate <=64 KiB plans per step; do not squeeze all plans into one item.
The <=256 KiB aggregate plus framing must satisfy creation budgets before 202.

Propose one shared existing owner/project admission namespace across standalone
submission/rerun and workflow submission/rerun. Keep old standalone digest
calculation intact; versioned workflow HMAC identities bind operation, source
workflowId + revision or source runId, target and normalized consent. Key digest
excludes source ID so cross-source/operation reuse finds a receipt and conflicts.
Store only private HMACs, no raw keys or credential digests in DTOs/logs.

Seven-day receipts: authorize active owner/project first, then resolve an
identical receipt before checking current definition revisions/secret values.
Return the same accepted run on retry, without re-freezing, admission or HTTP.
Known expired keys return 409; UI uses a fresh UUID for a deliberate action.
Uncertain commits retry only the identical prepared transaction token/time or
read back the receipt; no new IDs/key or blind second admission.

Proposed accounting reserves all N planned steps against existing 1,000-stage/
month and 100-user/day execution budgets on run acceptance, once. Skipped steps
receive no refund; reruns reserve N again. One inflight slot per active run,
transferred across steps and released once at terminalization. Storage remains
ticket-owned actual reservations within 1 GiB/project and 2 GiB/stage; no
upfront N-times-maximum evidence reservation. Reject successor publication
cleanly if storage unavailable; never overfill or replay the preceding request.
This conservative admission policy is Proposed, not an existing measured limit.

## Explicit extraction and binding grammar

Propose a small bounded parser, no expression engine or third-party JSONPath
evaluation. Paths: `$`, dot identifier `[A-Za-z_][A-Za-z0-9_]*`, bracket canonical
nonnegative index (`[0]`, `[12]`, no leading zeros), or bracket JSON string
property (`["a.b"]`, `["quote\"key"]`, `[""]`). JSON string escapes decode
exactly once. Maximum 512 UTF-8 bytes/path and 32 selectors. No negative index,
wildcard, slice, filter, recursive descent, function, coercion or script.
Dot segments always address objects; numeric brackets always address arrays;
quoted brackets always address objects. Prototype/golden vectors are required
after acceptance; no library choice is accepted by this packet.

Extraction DTO: `{name,path,type,sensitive}`, where type is string, number,
boolean or null. Names match `[A-Za-z_][A-Za-z0-9_]{0,63}`, are case-sensitive,
unique per producer step and never globally overwritten. All extractions are
required; no optional/default behavior. Exactly one primitive of the declared
type must exist. Missing, wrong type, object/array, invalid/unsupported/oversized
JSON or over-limit scalar fails extraction, never advances. `$` permits a scalar
root. Parse JSON strictly once with existing UTF-8/duplicate/depth rules and
number handling that preserves precision: reject nonfinite values and integers
outside JavaScript's safe integer range; serialize scalar tokens deterministically.
No conversion of strings into numbers or of null into a missing value.

Bindings DTO: `{from:{stepId,name},to:{kind,...}}`. Producer must precede consumer
in the submitted order and declare that name. Reordering that violates this is
rejected. No self/forward references, same-target duplicates or implicit name
lookup. At most eight bindings/step. Proposed target kinds:

| Kind     | Locator and rendering                                                                                                                                                               |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| query    | `index` into frozen ordered queryParameters; enabled entry required. Replace value; percent-encode through existing query serializer.                                               |
| header   | `index` into frozen headers; enabled entry required. Replace value; reject CR/LF and invalid header value. Host/transport-controlled headers remain forbidden.                      |
| jsonBody | `pointer` canonical RFC 6901 pointer to an existing JSON scalar; replace with typed primitive, preserving string/number/boolean/null. No container creation or overlapping targets. |
| textBody | Replace the complete text body, string source only.                                                                                                                                 |

Query/header accept string/number/boolean (deterministic scalar spelling), reject
null. No URL scheme/authority/method interpolation, raw template substitution,
header names, query names, array resizing or protected-locator overwrite. Reject
binding to a saved protected field; credentials can instead be bound to a public
placeholder which runtime marks sensitive. Existing JSON sensitive-field paths
cannot overlap the target. Bindings are workflow rules, not modifications to
the saved request. A login step can extract `$.token` as string and bind it to
the next request's Authorization header; the header placeholder must already
contain the desired complete value in the response (e.g. Bearer prefix), since
this slice has no concatenation/transforms. Query IDs and typed JSON scalars are
the other presentation examples.

## Runtime variables and containment

Encrypt **all** extracted values, including sensitive:false, using existing
stage KMS/AES-256-GCM envelopes in Protected, fresh nonce/data key and opaque
stage/project/run/producer/purpose context. Never names/values in KMS context.
Immutable OUTPUT#position contains producer ID/version, declared type and
effective sensitivity/provenance inside the authenticated envelope. Browser
and Control see rule names/types and availability only, never runtime values,
ciphertext, fingerprints, checksums, S3 keys or secret previews. No reveal API.

Treat every runtime value as redaction input, even sensitive:false; that flag
cannot declassify credentials or designated paths. Effective sensitivity is the
OR of explicit flag, recognized credential path, input reflection and inherited
provenance. Encrypted producer provenance follows every binding. Secret over
HTTP requires fresh explicit allowInsecureSecrets consent at submission/rerun;
default rejects before dispatch. Cloud public-IP/80/443, DNS pinning, no redirect,
proxy or automatic retries remain mandatory after resolution.

While bounded raw response is in worker memory: validate supported strict JSON,
extract all required values, build redaction corpus from saved and accumulated
runtime values, mask extraction source paths and supported reflections across
request/response, encrypt producer outputs, prepare immutable sanitized evidence
and successor exact references, then clear/discard raw buffers. Apply redaction
also to failed partial extraction candidates; if containment is uncertain omit
the body and fail closed. Never extract from already-redacted evidence. No raw
S3 staging, raw structured logs, debug dumps or queue/API values. Memory clearing
is best effort in Go, not a guarantee of erasure from every runtime allocation.
Unsupported transformations require configured omissions; preserve the existing
sanitizer's explicitly limited supported-encoding guarantee.

KMS/encryption failure or publication failure cannot produce a successful step
without its outputs. If prepared finalization can retry within its fence/lease,
retry preparation/publication only. Losing raw/output buffers after intent
forces safe failed unknown reconciliation, no fabricated successful output and
no HTTP replay. At terminal commit delete initial inputs, all <=10 output
bundles and current job bindings; pins never retain runtime ciphertext. Retain
prior output bundles while the run is active so later bindings and redaction
have exact provenance; final-step outputs validate but need not be stored.

## Atomic finalization and durable advancement

KMS and S3 cannot participate in a DynamoDB transaction. Verify final
object/API/Lambda size preflights before registering the bounded exact sanitized
upload ticket/reservation, conditionally PUT immutable evidence, then one
TransactWriteItems:

1. Guard active owner/stage/generation/project epoch, current run/position,
   intent/job fence/live lease, source eligibility and uploaded ticket identity.
2. Insert SNAP/RET/event; terminalize current job/anchor/step, settle publication
   accounting and increment run availableCount together.
3. On success with successor: insert encrypted immutable output, queue next step,
   insert deterministic execution/job/exact protected binding refs/OUT and
   advance run/version. Remove only current job bindings; no inflight decrement.
4. On final success/failure: terminalize run, record failedPosition when failed,
   mark all untouched successors skipped, assign normal run expiry, delete
   runtime/input bundles/current bindings and release inflight once. No successor.

Use combined conditions on updated items (no duplicate action per key), <=60
actions per creation/finalization and accepted global 80 actions/2 MiB/128 KiB
item ceilings. Builder must measure every actual serialized item/envelope and
deduplicated source check. Reject before admission if any possible transition
cannot fit its bounded budget; never split workflow advancement into independent
step/output/successor commits. Creation's per-step frozen plan records count
as additional actions; include them in the <=60 proof rather than hiding them.

No commit: unpublished sanitized object stays inaccessible; ticket/orphan cleanup
owns it. Commit with lost ACK: read exact terminal run/step/output/OUT state and
retry only identical transaction; one next job survives. Postcommit bounded
best-effort identifier-only SQS notification supplements durable OUT and
scheduled delivery. Lost/duplicated queue sends do not create another job or
advance a run. No SQS send inside the transaction, no volatile callback as the
only successor scheduler. Existing standard SQS max2/batch1 and concurrency/
lease/visibility/redrive settings remain. Never keep a worker Lambda alive for
all ten HTTP steps; each job independently claims and fences its one request.

## Failure, unknown outcomes and deliberate reruns

Run states queued/running/completed/failed; step states pending/queued/running/
completed/failed/skipped. Run becomes running at first intent, terminal only
after atomic finalization. Failure stops the run and skips untouched successors.
Only successfully extracted/encrypted/published complete responses can advance.
HTTP 4xx/5xx, extraction failure, bounds, source loss and sanitization failure
stop. Complete 2xx/3xx without extraction may keep existing safe omission
semantics; required extraction from omitted/unsupported content fails.

Use existing outcome codes/certainty plus Proposed extraction_failed and
runtime_storage_failed. Known complete HTTP plus extraction failure is known;
pre-intent input/source failure is not_dispatched. After durable intent, no
complete trustworthy result is failed execution_outcome_unknown/unknown.
Display **Outcome unknown — do not assume no effect**. Infrastructure retries
before intent may claim safely; after intent never return the job to queued,
re-send HTTP, resume the failed step or automatically continue the run. Restore
generation closes admission/delivery and fails restored nonterminal runs with
recovery_interrupted, skipping successors and draining ciphertext; no replay.

Whole-run deliberate rerun only; no retry-from-step or standalone rerun of a
workflow step in this slice (409 on the existing execution rerun route for a
run-owned step). Warn that all steps can repeat side effects. Fresh consent/key,
new run/step/job/evidence identities; replay frozen public plan/rules using
current values at exact stable saved-secret refs. Do not reuse historical output
values or recover deleted ciphertext. No same-name fallback. Missing/deleted
request/secret or unavailable frozen context blocks rerun. Deleted workflow
definition does not block rerun if the retained frozen replay plan and current
required request/secret refs remain authorized; this refines the started-run
definition-deletion rule and is Proposed.

## Limits, evidence retention and deletion

| Limit              | Accepted baseline / Proposed supplement                                                                                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Steps / definition | Accepted 2–10 sequential / 64 KiB complete definition including all rules.                                                                                                                             |
| Frozen input       | Accepted <=64 KiB/complete step config; <=256 KiB total frozen plans + initial bindings. Count encryption/serialization expansion in transaction/item bounds.                                          |
| Outputs            | Accepted eight/step, 2 KiB serialized typed scalar/output, <=32 KiB complete encrypted bundle, <=10 bundle versions. Proposed <=80 named outputs/run, eight bindings/consumer.                         |
| Sources            | Accepted eight saved-secret inputs/step, 16 distinct live inputs/run, <=18 total deduplicated workflow/request/source checks. Some ten-step combinations therefore reject.                             |
| Resolved/runtime   | Accepted independent 64 KiB outbound body/headers, 2 MiB wire and decoded response, 4 MiB evidence/API, 6 MiB fully serialized Lambda response. Existing depth32; Proposed path512 bytes/32 selectors. |
| Transactions       | Accepted <=80 actions/2 MiB/128 KiB item; <=60 creation/finalization. Measure worst-case terminal skips/deletes and successor refs before HTTP.                                                        |
| Admission/pins     | Existing light execution/storage/pin quotas apply together; proposed N-step reservation and single inflight run slot described above.                                                                  |

Each step evidence keeps its own completedAt+30d RET expiry and existing
individual pin endpoints/quotas: 100/200 project/stage pins, 256/512 MiB pin bytes.
No run-wide pin or evidence copies. Run normal expiry is terminal completedAt+30d.
Add runId/position as safe mutable execution-detail/history metadata; retain
existing schema-1 evidence unchanged. Run read DTO includes workflowRunId,
workflowId/revision, target, state/version, submitted/started/completed timestamps,
normalExpiresAt, currentPosition/failedPosition and ordered steps. Each step
contains stepId/position, frozen requestId/revision, nullable executionId, state,
safe summary, existing retention DTO and evidenceExpired. Include separate nullable
live definition links; no value/digest or raw diagnostic in extraction summaries.

Keep nonsecret frozen order/plans/rules/source IDs/safe outcomes while any step
is pinned or logically available, even after run expiry. Expired sibling bodies
are not retained by that protection. Runtime ciphertext is always removed at
terminalization. availableCount/pinnedCount change atomically with publication,
pin/unpin and RET cleanup, but counts alone cannot prove logical availability:
strongly inspect <=10 RETs and condition versions/run/project on context reads
and summary cleanup. Last expired unpin hides that body and otherwise unneeded
context immediately; original expiry never extends. A new pin must verify
required run context exists and contend with summary cleanup under the same
run/RET/project versions. Never mutate immutable evidence for pinning.

Project DELETE retains existing immediate denial + 202 operation and overrides
pins. Fence every queued/claimed/running successor and upload by project epoch;
resumably drain known WF/RUN/STEP/frozen-plan/input/output/IDEM/JOB/OUT/RET/SNAP/
events/bindings plus tickets and exact S3 keys. Run cleanup releases counters
once, and never starts a successor. Preserve unknown records, WORK and uncertain
uploads; complete only after strong both-table partition proofs, settled tickets
and empty quiescent prefix, with durable residual sweeps/minimal tombstone.
No authoritative TTL or blanket S3 expiry. Backups retain the accepted restore
exception and explicit timestamps/warnings; no new anti-resurrection guarantee.

Proposed compatibility: savedRequestsSchemaVersion 5→6 and
sequentialWorkflowsSchemaVersion=1, retain localAgent1/cloud1/pinning1 and recovery
generation. All API/worker/maintenance, record validators and pin readers must
support run ownership/counters/cleanup before guarded closed-stage activation.
Inventory/reconcile existing state; no counter resets. Old consumers fail closed;
local jobs remain single-request and cannot claim cloud workflow jobs.
Exact infrastructure/IAM/bootstrap preview and activation procedure await
acceptance and separately authorized deployment preparation.

## Minimal React behavior

Project workflow list/editor: name, ordered saved-request selectors with explicit
revision, move up/down/remove/add, extraction name/path/type/sensitive and
binding producer/target selectors. Validate backward refs after reorder; stale
save/submit asks to refresh/review, never silently upgrades request revisions.
No raw JSON scripting or workflow canvas is needed. Submit defaults cloud,
shows order/N-step quota reservation and HTTP-secret consent when needed.

Run list/detail shows ordered status and failure/skipped step, frozen revisions,
safe extraction names/types only, and links existing execution evidence/pin UI.
Inspect failed/unknown evidence with existing escaped text rendering. Pin protects
that step's evidence; context retention explains expired siblings. Rerun warns
about repeating the entire sequence and requires deliberate action/new key.

One write/poll in flight; ten-second immediate polling with jitter/backoff,
refresh button, stop on terminal/sign-out/unmount/project deletion/hidden tab.
Late responses cannot replace another project's selection. Keys stay in memory
for uncertain submission reconciliation; timeout does not auto-create a new run.
No evidence, runtime values, credential or draft secrets in browser persistence.
Accessible loading/conflict/error states; refresh uncertain writes before a
deliberate retry. Definition-creation uncertainty uses listing, no blind POST.

## Incremental cost proposal

Reuse current dev-api Control/Protected/indexes, KMS key, API/worker/maintenance,
SQS/DLQ and evidence bucket; no new standing resource, hosting or polling service.
Preserve Free plan, account quota ten and standard queue max2/batch1 with no
reserved/provisioned capacity. Generated resource/IAM diff has not been run.

Model 100 runs/month ×3 steps =300 total executions **within** existing 1,000-step
light budget, average 5s/512 MiB worker, 100 KiB evidence/step. Do not add their
ordinary worker/SQS/S3 workload again to a baseline already counting those steps.
Workflow overhead allowance: 20k extra API polls/edits at 100ms/256 MiB,
100k extra transactional WRU and 250k RRU after item rounding, <=0.1 billed GB
metadata including indexes plus PITR/backup, <=2k additional KMS operations,
<=100 worker GB-s for extraction/encryption, <=1 GB additional evidence delivery.

Planning inputs from the [prior regional model](../development/evidence-pinning.md):
HTTP API $1/m; ARM Lambda $0.20/m requests and $0.0000133334/GB-s;
DDB $0.625/m WRU/$0.125/m RRU; metadata combined $0.55/GB-month;
KMS $0.03/10k; delivery $0.09/GB. Estimated overhead $0.02 API +$0.012 Lambda +$0.094 DDB requests +$0.055 metadata +$0.006 KMS +$0.09 delivery ≈$0.28.
Reserve **$0.50–$1/month incremental** for safe logs, cleanup and contention;
no free-tier/credit subtraction, hard cap or measured-cost claim.

Official [DynamoDB](https://aws.amazon.com/dynamodb/pricing/),
[Lambda](https://aws.amazon.com/lambda/pricing/) and
[KMS](https://aws.amazon.com/kms/pricing/) pages were read during preparation.
Transactional units double standard reads/writes and round by item size; this
is included in the allowance. Numerical Ohio rates are historical planning
inputs, not a newly queried regional catalog. Refresh catalog/diff before any
deployment. An additional 300 steps rather than reallocated baseline steps adds
at least 750 worker GB-s (about $0.01 compute), plus ordinary evidence/storage/
SQS/crypto/egress. Max-size captures, pins, ten-step runs, long HTTP, more polling,
rejected calls, shared bootstrap and rotations can exceed the allowance/target.
Do not sum all historical slice estimates into a claimed current total bill.

## Planned tests and validation boundary

Implementation review must adapt `services/request-runtime/runtime.go` to expose
a narrow memory-only extraction phase before sanitization/disposal;
`services/api/execution/store.go` and `worker.go` to build bounded run admission
and atomic advancement; `maintenance.go`, `history.go` and `pinning.go` to share
run/context fences; ownership validators/cleanup and React API types/editor to
recognize the new records. Current `recoverPrepared` can finalize standalone
sanitized evidence after a lost worker: it must **not** recover a successful
workflow successor from evidence alone when encrypted outputs were never
committed. The safe unknown/no-successor rule above is required in that path.
Existing local-agent results remain output-free; do not extend their protocol.

All workflow tests below are **planned**. Baseline verification of this packet
cannot establish workflow correctness. Use deterministic barriers/clocks,
fault injection, DynamoDB Local race tests and browser fixtures after acceptance;
count sends at a controlled endpoint, with disposable memory-only secrets.

| Area                      | Required scenarios                                                                                                                                                                                                                                                                                                                                                       |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Ownership/revisions       | Foreign and cross-project workflow/request/output/run; disabled user/deleted project; stale workflow/request at submit; edit during freeze; source replace/revoke/delete before claim and advancement; started versus unstarted workflow deletion.                                                                                                                       |
| Grammar/types             | Escaped/dotted/empty/Unicode property names; root scalar, array boundaries, missing/null/type mismatch; unsafe numbers, duplicate keys/depth; denied filters/scripts/negative indexes; forward refs/reorder/duplicate targets; injection in header, query encoding and typed JSON replacement.                                                                           |
| Limits/accounting         | Exact/one-over every scalar/path/envelope/config/body/aggregate/action cap; maximal ten-step sources/terminal deletes; N-step admission single charge/no skipped refund; successor storage saturation; zero underflow/double release.                                                                                                                                    |
| Admission crash/duplicate | Crash before/after creation transaction and lost ACK; identical key after edits; cross-source/standalone/rerun conflicts; expired key; no second first-job/quota admission.                                                                                                                                                                                              |
| Dispatch crash/duplicate  | Duplicate SQS/OUT fast and scheduled notifications, two workers, before intent, lost intent ACK, crash immediately before/after HTTP send, during read; zero automatic second send after intent and no successor on unknown.                                                                                                                                             |
| Finalization crash        | Crash after extraction, KMS encryption, ticket registration, PUT, before transaction, after transaction/lost ACK and before fast notification; exact output/version/job/OUT survives only with atomic state; missing buffers cause safe unknown/no replay.                                                                                                               |
| Finalizer races           | Two successful finalizers; worker versus lease reconciler; stale producer version; conditional source revoke; output/step/manifest/ticket/counters never partially commit; exact transaction-token retry and readback do not advance twice.                                                                                                                              |
| Secret containment        | Saved/runtime credentials including sensitive:false, recognized names, source paths, partial failed extraction and supported encoded reflections across later requests/responses; no values in Control, logs/traces/errors, SQS, browser status/storage, evidence, temp files or KMS context; terminal ciphertext absent even with pins. Audits print counts/flags only. |
| Failure/rerun/recovery    | HTTP 500, timeout/unknown, invalid/oversized JSON, encryption/publication failure; all successors skipped; whole-run new identities/current stable secrets; missing refs; workflow-step standalone rerun denied; generation restore suppresses all OUT/jobs and drains variables.                                                                                        |
| Cleanup races             | Pin versus RET claim/summary deletion, available sibling versus expired body, last expired unpin; context read/version races; delete versus creation/claim/finalization/successor/KMS/late PUT; uncertain upload stays pending, interrupted multichunk drains/unknown records, strong completion proofs and residual sweep.                                              |
| React/regression          | Editor order/ref validation, stale refresh, uncertain key reuse, safe values-free run inspection, unknown warning, deliberate whole rerun, pin/context/expired UI, memory clearing/late responses; existing cloud/local standalone auth, digests, public destination and evidence bounds preserved.                                                                      |

Actual AWS checks later must separately prove scoped IAM/KMS contexts, atomic
cross-table transactions/index discovery, SQS duplicate delivery, immutable S3,
scheduled advancement/expiry/project drain and compatibility activation. Genuine
Cognito/browser workflow editing/inspection and controlled-target send counts
remain separate from fixtures. Induced AWS crashes/lost responses, real PITR,
elapsed expiry/clock skew, negative IAM and metered cost are not inferred from
local passes. No AWS call, SST preview or deployment was performed for this packet.

## Preparation and decisions requiring acceptance

[Trello preparation card](https://trello.com/c/woHPDkI3) was created after the full
open board returned no workflow card. Relevant architecture/contract, saved/
protected request, cloud prototype, pinning and local-agent cards/dependency
checklists were read. Doing's configured WIP limit is three; zero cards preceded
this task. Preparation and publication are tracked separately from acceptance,
implementation/synchronization and deployment/runtime checks; unfinished work
stays open. Proposal and contracts were read without edits.

Preparation verification passed: repository Prettier/gofmt, frontend ESLint,
48 frontend tests, 10 infrastructure tests and TypeScript/Vite production build;
tests/vet for all six Go modules and builds for API/worker/agent/runtime. The
initial sandbox Go verification stopped at a read-only build-cache write;
the approved outside-sandbox Go rerun passed. Markdown fences and local links
were checked. These are baseline/document checks, not new workflow tests,
DynamoDB Local integration, actual AWS or genuine browser workflow validation.
Only this packet and its architecture-index link belong to the task commit.
Trello publication is checked complete only after commit/push/readback; the card
returns to To Do while awaiting acceptance, with later milestones open.

Decisions requested (all **Proposed**):

1. Cloud-only first slice with explicit reference bindings, whole-run reruns,
   minimal editor/inspection and deferred assertions/environments/schema/MCP.
2. Revisioned CRUD/run DTOs and shared seven-day source/operation-bound admission
   receipts, N-step quota reservation/no skipped refund and one inflight run slot.
3. Restricted path grammar/typed primitive extraction and target mapping rules,
   eight outputs/bindings per step and the accepted transaction/config caps.
4. Encrypt every runtime value/no reveal, mandatory extraction-source/reflection
   redaction, exact immutable producer provenance and terminal ciphertext deletion.
5. Atomic publication/output/advancement/OUT, unknown-stop/no replay, conditional
   pin/run-context cleanup and capability6/workflows1 compatibility plan.
6. $0.50–$1 incremental planning allowance and the planned crash/duplicate/
   containment/cleanup-race validation boundary; refresh costs before deployment.

Acceptance must be explicit and state the authorized follow-up scope. Commit/
push and Trello preparation completion do not authorize dependent code, external
contract edits or AWS deployment.
