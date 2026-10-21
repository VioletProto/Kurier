# Legacy-A status GET 503: local diagnosis and polling correction

This task investigates the stopped [replacement attempt](workflow-assertions-replacement-rollout.md)
using retained artifacts and local fixtures only. It makes **zero new live AWS,
application API or controlled-target calls**, creates no real identity/project,
and performs no stage write, activation or deployment. It does not resume the
campaign. Historical receipts and counters remain unchanged.

## Finding and attribution limit

A transition between RunDetail's individual strongly consistent reads and its
final transactional validation can produce `503 service_unavailable`. Strong
consistency of each read does not make a group of reads a snapshot. The final
condition checks correctly prevent returning a mixture of old run state and new
step/project state. A fresh GET builds a new guarded read and can succeed.
**This is a reproduced possible path, not the established historical cause.**

Retained attempt-ledger sequences **776/777** independently record the status
GET and its HTTP 503 response, without a transport error. The coordinator's
receipt records error code `service_unavailable`. Subsequent durable readbacks
and UUID audits prove A completed two steps with two sends. They cannot recover
which row versions the failed read observed, its DynamoDB cancellation reasons,
request-specific exception, or actual response headers. Raw Lambda messages
were not retained. Later success does not identify why an earlier read failed.
The stage's unchanged before/after marker likewise is not a transaction trace.

The retained old API ZIP matches the independently verified live SHA-256
`cf65f8badc4145962fee4cdc79ab4d4cb1f1829d302f41ce0f30833ae19a326e`.
Offline `go version -m` reports VCS revision
`21c24be9189f067deb5cf75df25b9e2df9e5190c` and **`vcs.modified=true`**.
The named ancestor, current tree and clean pinned source `68d9f218` have the same
RunDetail function text (SHA-256
`448b61f0e120474e86c66e7d931f0ab7d426386cb1252bf52da712ea91b69268`).
That comparison does not prove an exact source correspondence for the dirty
historical binary. No historical binary/source reconstruction or new cloud run
was attempted for attribution. The historical cause remains **unestablished**.

## Possible `service_unavailable` paths

The code trace covers the HTTP wrapper and service, not just the final CAS.
The relevant files are [workflow HTTP](../../services/api/internal/ownership/workflow_http.go),
[ownership store](../../services/api/internal/ownership/store.go),
[request context](../../services/api/internal/ownership/request_store.go),
[RunDetail](../../services/api/execution/workflows.go),
[execution store](../../services/api/execution/store.go) and
[row guard](../../services/api/execution/pinning.go).

| Boundary                            | Paths that can yield 503                                                                                                                                                  | Important distinction                                                                                                                                                                                          |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Authentication/ownership middleware | ResolveUser's stage/read/schema/mapping checks; failed identity-resolution transaction; unavailable/malformed user metadata                                               | JWT rejection itself is 401. An acknowledged submission already establishes an owner; status reads do not authorize creating another Cognito identity.                                                         |
| HTTP workflow handler               | Missing execution service; initial requestContext stage/user/project read failures or invalid capability configuration                                                    | Missing/foreign/deleting projects normally return 404; these are not all transient 503s.                                                                                                                       |
| workflowContext                     | Unavailable/invalid/inactive stage, incompatible capability shape, absent sequential-workflow capability                                                                  | Execution project lookup masks its errors to 404; execution userGuard masks unavailable/disabled user to 403. The outer ownership context can instead expose its own 503.                                      |
| runSteps / configuration            | Invalid 2–10 step count; failed/missing/unknown-schema step read; wrong step identity/position; absent plan; invalid frozen JSON or mismatched step count                 | These failures also use fixed 503 and can be persistent. They are not proof of contention. The run-row lookup itself maps failed/missing/wrong-owner results to 404.                                           |
| retention hydration                 | Failed/missing/invalid-schema RET read for a terminal, non-expired step                                                                                                   | A context that is no longer retained returns 404. Optional live workflow/request link read failures omit those links rather than produce this 503.                                                             |
| final validation                    | Any failure of stage capability/generation/state, enabled-user/version, project owner/version/state/deletion epoch, run/step kind/version, or RET kind/version conditions | A worker advance/finalization changes run/steps and project accounting versions. Pin/unpin, project edits/deletion or maintenance can invalidate other guards. Returning stale data would weaken those checks. |
| transaction infrastructure          | Unresolved DynamoDB/transport/IAM/throttle/cancellation error, exhausted conflict retry, or transaction size/action rejection                                             | All normalize to the same fixed 503. The normal <=10-step read has at most 24 small condition checks, below the existing 80-action/2-MiB bound.                                                                |

RunDetail's validation transaction contains **ConditionCheck actions only**.
It does not submit work, prepare a target, decrypt inputs, publish an outbox,
write evidence, or dispatch HTTP. The shared `transact` helper may retry an
identical transaction/token for recognized pure `TransactionConflict` reasons
(six attempts maximum). `ConditionalCheckFailed`, mixed/unknown cancellation
reasons and other failures do not refresh the captured row versions. The
workflow-Put acknowledgment recovery branch does not apply to this read-only
transaction. RunDetail turns the remaining error into fixed 503.

The HTTP formatter sends `Retry-After: 2` for its 503/429 errors. That is a source
observation, not proof that the historical header was received or retained.
Neither the error code nor that header identifies the failed guard.

## Deterministic local evidence

[The new integration regression](../../services/api/execution/workflow_detail_race_integration_test.go)
uses DynamoDB Local with static local credentials and the existing loopback
HTTP, object, KMS and queue fixtures. No synthetic DynamoDB cancellation is
substituted for the stale-read result:

1. Admit one local two-step workflow. Suspend RunDetail after it reads the run,
   then use the actual worker to complete both steps before the step reads.
   DynamoDB Local rejects stale project/run versions; RunDetail returns fixed
   503 and no partially constructed DTO.
2. Repeat with the worker transition just before transactional validation,
   after all rows were read. The actual cancellation includes stale run, both
   step and project guards.
3. Separately model an active project-version change at validation. The first
   read rejects; a fresh read succeeds. Model deletion at the same boundary:
   the stale read rejects and fresh reads return 404, never bypassing the gate.
4. After each interleaving, make three fresh reads. Admission count remains 2;
   quota/project versions, target-send count, queue notifications, object puts
   and mutating-transaction counts do not change during those reads.

The worker-transition cases intentionally make **two local fixture sends** as
the concurrent actor; the project-only cases make zero. These are distinguished
from the **zero additional sends/admissions caused by fresh GETs**.

`TestIntegrationCoordinatorRunDetailGETBackoff` additionally runs the actual
Node coordinator-reader adapter against a loopback HTTP bridge to the real
RunDetail method. It observes **503 → bounded GET backoff → 200**, checks the
actual two-second interval after sending Retry-After, and verifies zero retry
deltas in admissions, target sends, queue notifications, object writes and row
mutations. The bridge is test-only, rejects non-GET requests and non-loopback
origins, and supplies no AWS or credential adapter. It does not claim production
Cognito/JWT/HTTP-adapter validation.

## Coordinator polling correction

The historical coordinator waited two seconds, called a generic one-attempt
HTTP function that required 200, and stopped on the first non-200 result. Its
35-iteration counter was separate from that function. It did not honor the
initial accepted submission's Retry-After hint (the current server sets 10)
and had no status-specific 503 policy. This explains the coordinator's stop
behavior; it does not explain the server response.

[The tracked polling helper and coordinator adapter](../../scripts/lib/workflow-run-poll.mjs)
replace that policy without changing the application. `createCoordinatorRunReader`
accepts an already acknowledged 202 response and polls its scoped run UUID. It
rejects uncertain submissions or inconsistent Location and latches a stop.
It stores no write callback and issues **only GET** to
`/api/v1/projects/{projectId}/workflow-runs/{runId}`. It rejects redirect responses;
the single-exchange transport must disable automatic redirect following.
There is no generic retry wrapper for submission, rerun, definition writes,
closure, activation or cleanup mutations.

- Retry only HTTP **503 with parsed error code `service_unavailable`** on this
  exact status route. This explicit operator allowlist is not a claim that all
  such errors are transient. Other 503 codes, malformed envelopes, 429,
  authorization/deletion/CAS errors and other statuses stop immediately.
- Permit **three additional consecutive GET attempts**, with default waits
  **2, 4 and 8 seconds** after successive 503s. A successful nonterminal 200
  resets consecutive backoff, never total counters. The fourth consecutive 503
  stops. Transport/parse/timeout uncertainty is never retried by this helper.
- Honor Retry-After on the initial 202 and follow-up responses, supporting
  nonnegative delay-seconds and HTTP-date forms in UTC. Wait at least two
  seconds and at least the server hint. Invalid hints or a required wait above
  **30 seconds** stop; never cap a server hint downward to send early. This
  follows [RFC 9110 §10.2.3](https://www.rfc-editor.org/rfc/rfc9110.html#section-10.2.3).
- Preserve **35 total GET attempts per run**, **240 shared polls** and **500
  shared API attempts**, including every failed GET. Increment and durably
  persist counters before calling the single-exchange transport. The existing
  authoritative network meter still counts retries/SDK/browser traffic and
  conservatively charges all AWS attempts to the 1,500-read allowance.
- Keep one poll in flight. Check shared stops, meter limits and cleanup reserves
  before waiting and again after waiting/before transmission. Use a monotonic
  per-run deadline of **120 seconds**, and a campaign deadline computed from the
  original first-call timestamp with the **100-minute work / 20-minute cleanup**
  split. Each request receives an abort signal bounded by the remaining time
  and the existing 15-second request timeout. Excessive delay, persistent error,
  exhausted budget or deadline latches a stop; there is no automatic resume.

The concrete integration boundary is:

```js
const reader = createCoordinatorRunReader({
  ledger, // Existing API/poll counters and original startedAt; never reset them.
  exchangeOnce, // Raw single GET exchange, parsed {status,data,error,headers}.
  budget: () => budget(false), // Same meter stop and 200-AWS/60-API cleanup reserve checks.
  persist: save, // Same durable ledger; GET accounting belongs to the reader.
});
const accepted = await callSubmissionOnce(); // Existing one-attempt write path.
const run = await reader.pollSubmitted(projectId, accepted);
```

The raw `exchangeOnce({method,route,signal})` must disable redirects, honor the signal, parse the
response without throwing away its 503 status, have no retry loop, and **not
increment API counters a second time**. All other coordinator operations retain
their existing single-attempt accounting and stop policy. Do not wrap the old
`call(...expected200)` inside the new reader: it would discard the retryable
response and double-charge logical counters. The local HTTP bridge exercises
this exact adapter contract. Integrating a future live campaign must retain
its reviewed meter/reserve checks; the no-op budget callback exists only in
the local fixture.

## Verification, release and review boundary

`npm run test:workflow-poll` runs 16 deterministic policy tests, including HTTP
date forms, minimum/Retry-After delays, persistent 503 bounds, other status
rejection, uncertain/malformed responses, shared limits, cleanup reserve/deadlines,
durable-accounting failure, stop latching, parallel polls and write-route denial.
It is included in `npm test`/`npm run verify` so CI exercises the correction.
The full API integration race suite passes against DynamoDB Local, including
the actual-transition/fresh-read cases and the Node-to-Go HTTP bridge.
Formatting/gofmt, recommended Node ESLint, web/infra tests, TypeScript/Vite build,
all seven Go module tests/vet and six release-pin checks pass.

**No application fix is proposed or needed for the reproduced behavior.**
The guarded read correctly rejects a stale snapshot. Production API/worker/web
source, IAM, schemas, generation, quotas and concurrency are unchanged. The Go
addition is integration-test-only; package.json changes only the test command.
Clean application source remains `68d9f218225c7092a1e4fca0b66a21421d862a22`;
manifest SHA-256 remains
`920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6`.
Pinned API/cleanup ZIPs remain
`f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32`,
worker/fixture ZIPs
`15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379`,
and frontend release
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`.
No pinned release artifact is rebuilt or repinned by this diagnostic task.

The new operator policy is implemented and locally verified for review; it has
not run against the live API. It does not reopen the stopped attempt, authorize
new identities/projects, change the four consumed steps/sends or four cleaned
identities/projects, infer historical AWS reads/billing, or resume B–L. A future
execution packet must use the reviewed reader integration and an explicitly
authorized campaign allowance. Any later application correction requires its
own source/release/package review. Broad live fault injection and PITR remain
separate. No cloud state was recreated merely to attribute the historical 503.
