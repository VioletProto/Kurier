# Workflow publication investigation

This investigation is separate from Davian's successful nine browser acceptance
checks and the independently completed original project deletion. It adds one
genuine browser polling run, a confirmed retry defect and regression coverage.
The earlier SDK publication interruption is **not conclusively attributed**.

## Confirmed defect and fix

`execution.Service.transact` previously excluded every
`TransactionCanceledException` from its workflow retry. That includes transient
`TransactionConflict` cancellations. Ticket-only acknowledged-PUT settlement has
no `runId`, so it also missed the separate historical workflow retry entirely.
Run-detail hydration uses condition-check transactions on stage, user, project,
run, step and retention records that live workers also access. A conflict can
interrupt registration, writing intent, durable PUT settlement or finalization.
[AWS documents transaction conflicts and the need for application retries](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/transaction-apis.html).

The fix retries only known transaction conflicts: a direct conflict exception,
or cancellation reasons consisting entirely of `None`/`TransactionConflict`
with at least one conflict. Six total attempts reuse the identical actions,
guards, time, ciphertext and client request token. Backoff starts at 10 ms,
doubles between attempts and adds 0–10 ms jitter; context cancellation stops it.
Missing/unknown reasons and mixed condition/validation failures do not qualify.
The prior exact-token ambiguous workflow retry/readback behavior is preserved.
No enclosing request execution, HTTP dispatch or S3 PUT is replayed by this loop.

This removes a needless failure on brief contention; it does not promise success
under sustained faults. After lease expiry, workflow recovery deliberately
publishes a failed unknown result if successful evidence/outputs cannot be
reconstructed. Successors are skipped and HTTP is never replayed. A truly
unsettled PUT remains protected by the existing cleanup guard. Deletion,
revocation, recovery generation, ownership and revision gates remain mandatory.

Future worker diagnostics emit only fixed phase/error codes plus validated
project/run/job/execution/lease/ticket UUIDs and numeric fence. They trace claim,
intent, ticket registration/writing, PUT acknowledgment, durable settlement,
finalization and encountered conflicts. They exclude request/response data,
URLs, credentials, envelopes, SDK messages and cancellation item values.
`ok` identifies an acknowledged boundary, not a promise that a later boundary
committed. These diagnostics are implemented but **not deployed**.

## Genuine deployed browser polling

Prepared a disposable six-step workflow under the same genuine Cognito owner
as the accepted browser fixture. Each step uses the existing controlled
five-second `/execution-fixture/delay` endpoint; no endpoint change or fake
identity was introduced. Setup used the SDK; **Davian submitted once in the
normal signed-in browser**, kept its detail visible as instructed, and reported:
`Run c59a8ba2-8342-4522-9bbd-fcad1996290b: completed`.

- Project: `ace3b7a2-6499-4a5e-a5dd-281c46a3dbac`.
- Workflow: `c818f137-c85e-4c3a-ad62-21757d365996`.
- Run: `c59a8ba2-8342-4522-9bbd-fcad1996290b`.
- Unmodified frontend polling: ten-second interval, visibility/busy gates,
  jitter and failure backoff. Browser completion is operator-reported;
  we did not capture authenticated browser traffic or tokens.
- Independent strong readback: all six jobs/steps completed, each job
  attempts=1/fence=1/version=5; six settled/published tickets and six immutable
  manifests whose S3 evidence verified. Project inflight=0; evidence storage
  6,870 bytes; Protected partition empty after terminalization.

The [safe sampled trace](sequential-workflows-publication-trace.json) records
observed job/lease/run advancement and ticket publication. Ordinary GetItem/
Query inspection did not participate in the worker's transactions. Short
pending/writing/uploaded transitions can fall between samples; their absence
from this artifact is not proof they did not occur. This run did not reproduce
the earlier interruption. The table-level `TransactionConflict` metric returned
no datapoints for the inspected window, which is not an affirmative zero count
or proof of absence of contention.

The baseline deployed hashes remained unchanged throughout this run. Therefore
this proves existing deployed execution under normal browser polling, **not AWS
validation of the new retry fix or new diagnostic logs**.

## Regression validation

DynamoDB Local race regressions inject AWS conflict cancellation codes while
performing real `RunDetail` hydration and real Local transactions. The HTTP,
KMS, S3 and conflict injection in those tests are fixtures, not genuine AWS
faults. With two injected cancellations at each ticket boundary (pending,
writing, uploaded, published), both steps finish, exactly two HTTP sends and
two S3 PUTs occur, outputs are removed, admission remains two slots and inflight
returns to zero. The same regression was run against the historical transaction
helper and failed at all four expected boundaries, then passed with the fix.

Additional regressions cover direct conflict exceptions, missing/mixed reasons,
conditional/validation rejection, exact token/action preservation, retry budget,
context cancellation, safe diagnostic containment, and sustained finalization
conflict. Exhaustion followed by lease recovery terminalizes failed/unknown,
skips the successor and retains one HTTP send. Changes to project deletion,
source authorization or recovery generation between attempts must still deny
publication and successor/output creation.

Verification passed: repository formatting/gofmt/lint, 52 frontend and ten
infrastructure tests, production build, six Go modules tests/vet; complete API
DynamoDB Local integration race suite; all five Chromium fixture scenarios;
AWS-tagged vet; genuine AWS fixture setup, immutable-evidence/readback and
guarded physical cleanup. No simulated failure is labeled a genuine AWS fault.

## Earlier extra browser target calls

Independent endpoint logs contain ten distinct gateway request IDs:
3 token / 3 consumer / 2 failure / 2 delay. Consumer authorization results were
false, true, true. Two producer/authorized-consumer pairs align with the original
and deliberate whole-run rerun. The first token call precedes both pairs; the
first consumer call is unauthorized. Delay calls are approximately two minutes
apart, but that timing alone cannot establish a lease replay.

Davian explicitly confirmed that he opened/called the fixture URLs directly.
That operator confirmation explains the extra calls alongside retained usage:
monthly stage count is now 87 = prior 73 + eight original browser workflow slots

- six investigation slots. This is consistent with direct endpoint calls that
  do not consume Kurier admissions. Original project metadata/evidence was already
  deleted and no surviving admission links to those fixture URLs remain; endpoint
  logs alone cannot assign each extra gateway request to a caller. Attribution is
  operator-confirmed, not a reconstructed per-request proof. No additional HTTP
  was sent to investigate historical calls, and those historical extras are not
  counted as evidence of automatic worker replay.

## Cleanup, preview and remaining uncertainty

Investigation deletion `3f2390eb-8676-402b-9e00-ec5c13f81983` completed through the
existing guarded deletion/maintenance paths. This was manually assisted cleanup,
not a schedule-only measurement. Strong Control tombstone-only / Protected0 /
complete S3-prefix-empty proofs passed. Real user META, capability/generation
marker and admitted usage were preserved. Final storage reconciles across stage,
projects and upload tickets to the preexisting **10,390 bytes**; pins=0,
unsettled jobs=0, stage active/v9 with savedRequests6/workflows1, generation
`dev-api-initial-v1`. Free ACTIVE, account/unreserved concurrency10 and worker
SQS max2/batch1 remain unchanged.

Rebuilt SST development preview: only code assets and package updates for the
existing API, cleanup, worker and shared controlled-endpoint binaries. No IAM,
route, standing-resource, bootstrap, capability or account configuration change.
The first preview invocation lacked existing Cognito identifiers and stopped
before producing a plan; the subsequent correctly configured diff succeeded.
No deployment or activation was performed.

| Pending archive          | ZIP SHA-256, base64                            |
| ------------------------ | ---------------------------------------------- |
| API / cleanup            | `JZC9O9M8DkFsh2i+xI6sFSwxY3U5ZI9h8nfD5MvZbO8=` |
| Worker / shared endpoint | `384gi0Hd4EYWELtikPhpjYyew3Q1U8ZD/ZZKmuHKUQM=` |

The earlier publication incident's records were already cleaned up and its
original error/cancellation reasons were not captured. Contention is now a
confirmed reproducible failure mechanism, but its role in that specific incident
remains unknown. No permanent deadlock was reproduced: transient conflicts now
succeed in fixtures, while sustained faults conservatively stop after recovery.
Genuine AWS validation of the fixed artifacts, actual process-crash/ACK-loss
faults and the complete expiry-crossing credential harness remain unperformed.
Earlier operational gaps (negative IAM, PITR, elapsed retention/skew, maximum-size
memory/latency and metered cost) remain unchanged. Successful prior browser and
cleanup checks are not relabeled as failures or as proof of these gaps.

Reproduction tools:

```sh
# Read only, safe identifiers/states, maximum 600 seconds; no HTTP submission.
node scripts/inspect-workflow-publication.mjs PROJECT_UUID 180

# Local fixture faults and no-replay/guard regressions.
cd services/api
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 \
  go test -race -tags=integration -run 'TestIntegrationWorkflow.*(Conflict|Publication)' -count=1 ./execution
```

The explicit AWS setup/readback helpers are opt-in and restricted to this
investigation's owner/project. Setup refuses to create a duplicate fixture;
readback cleanup is repeatable and never submits another run or removes users.
