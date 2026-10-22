# Instructor cleanup and H–L continuation

The instructor's existing normal deletion is physically verified. Charged
admissions were preserved and the refreshed baseline passed full preflight.
The authorized H–L continuation then stopped in hosted browser authoring before
H admission. Its one disposable project/provider identity was normally cleaned
and independently verified. **H–L remains unperformed; no new steps or target
sends occurred.** The driver corrections described below passed local tests.
The pinned application release was unchanged, with no redeployment or stage
write and no A–G replay.

The [receipt](workflow-assertions-hl-continuation.json) retains the instructor
cleanup proof, initial collector failure, full successful preflight, stopped
coordinator, normal cleanup and final accounting. The
[continuation meter ledger](workflow-assertions-hl-continuation-attempts.jsonl)
links the exact [54-read parent ledger](workflow-assertions-hl-attempts.jsonl).
The [failed browser driver source](workflow-assertions-hl-failed-browser.mjs.txt)
is preserved unchanged from the code used for the live attempt. These are new
records; prior published receipts and failures have not been rewritten.

## Existing instructor deletion and refreshed accounting

The user reported that the extra activity was disposable instructor-review data
and that normal deletion was already requested. Metadata-only discovery found
one new existing deletion:

- Project `530b7e83-1c4b-4ec5-a878-1510afa709ed`, owner
  `ec04b675-818d-4f1b-a294-5a6330acb432`.
- Existing operation `999e2cff-987b-4d49-9eb0-ae6b54ee100c`, with currently read
  durable completion timestamp `2026-10-21T23:40:56.192360661Z`.
- Matching completed tombstone only in Control; no Protected children or upload
  tickets; complete empty S3 prefix; permanent residual registration retained.

No project deletion was resubmitted, no rows or objects were manually drained,
and no instructor identity was changed. Completion was observed after it
occurred; missing intermediate upload/deletion transitions are not reconstructed.

The first observer stopped because it incorrectly expected a single daily quota
row for the instructor owner. Its complete failure receipt is retained. The
owner has two charged steps on **October20** and two on **October21**. The observer
was corrected to select the current date while retaining both days. Fresh rows
sum to the unchanged monthly charge: instructor2+2, earlier campaign owners2+2,
and final campaign owner14 = **22**. The other retained campaign owner has no
daily quota row. Attribution to instructor-review activity comes from the user;
the matching deletion identity, daily rows, monthly sum and physical emptiness
are independently retained observations.

| Observation                        | Monthly count | Quota version | Reserved bytes |
| ---------------------------------- | ------------: | ------------: | -------------: |
| Earlier cleanup verification       |            20 |            48 |              0 |
| Prior stopped H–L preflight        |            22 |            52 |          3,302 |
| Instructor cleanup, fresh baseline |        **22** |        **54** |          **0** |
| Final H–L disposable cleanup       |        **22** |        **54** |          **0** |

The version/storage change is consistent with normal release of two upload
tickets. Individual historical ticket readbacks are absent and are not claimed.
Before/after owner snapshots are identical, including both instructor dates.
The refreshed/final accounting snapshot hash is
`d3794504a5a0af7566816f95509737272a71fdc1235a676799a0acba30a52485`.
Pins and inflight are zero. No charged count was reset or subtracted.

## Full preflight and genuine hosted stop

Fresh preflight verified intended account `747336059622`/non-root identity,
the exact pinned four Lambda packages/configurations and IAM/platform controls,
all five frontend files, an empty queue, anonymous401 and the refreshed accounting
baseline. Stage remains active **15**, savedRequests **8**, assertions **1**, other
capabilities **1**, generation `dev-api-initial-v1`. Successful platform details
are now persisted before later preflight guards, unlike the earlier failed run.

One additional disposable owner `1eab4ab5-4dbe-4eda-b7d0-b1d7db56ce63` and
project `2dc402b9-1c56-4906-a5ab-d34985400d58` were created after preflight.
Genuine Cognito SRP succeeded in the Node coordinator and hosted Chromium. The
corrected meter forwarded and charged both browser Cognito OPTIONS attempts and
both challenge POSTs; application reads succeeded. Two saved requests and an
empty-assertion H definition were prepared. The five reviewed invalid numeric
DTO/domain/length definitions returned400, with no admission or dispatch.

Hosted authoring stopped before any browser PATCH or workflow-run submission.
The live exception was sanitized before retention and does not identify a
specific failed locator. No live reproduction, automatic continuation or target
replay followed the stop. The reviewed coordinator dispatches only cleanup after
a recorded stop; this guard was preserved.

Separate local reproduction using the **actual pinned browser bundle**, real
isolated TLS meter and simulated API/provider responses found two driver defects:

1. Exact label matching did not locate the assertion-kind select. Select labels
   include option text in this context. Matching the label within its already
   scoped assertion group works without requiring exact text.
2. Reopening called nonexistent Playwright Locator `allInputValues()`. The
   correction uses supported `evaluateAll` to read each DOM `input.value` as a
   string, without numeric conversion.

The corrected local test authors sixteen checks, then closes/reopens the browser
and makes two unrelated name edits. All three local PATCHes preserve the twelve
decimal tokens exactly, including `0.10000000000000001` versus `0.1`, `1e-1`
versus `1E-01`, `1e-308`, `9007199254740991` and `-0.0`. Browser storage is empty.
Fixed stage/error-class diagnostics were added for future failures; raw errors,
headers, passwords and tokens are not added to live receipts. Local SRP responses
check proof shape, not the password cryptographically. These tests are not live
assertion evaluation. The local failures are consistent with the observed live
pre-PATCH stop; they do not conclusively establish its historical cause.

## Normal disposable cleanup

Only the new H–L project was deleted via the normal conditional API, once:
operation `9543afe4-a0a9-4768-89ff-ae1f0d83a14c`. Owner404 followed acceptance.
Its matching completed tombstone, empty Protected/S3/upload state and retained
residual registration were verified. Normal cleanup completed in **114.096s**.
The exact disposable provider identity was globally signed out, deleted and
confirmed absent; no additional identity was created. This does not establish
immediate offline access-JWT invalidation.

Fixture UUID audit reads found zero sends from the prepared requests. Target
audit collection starts with this continuation's runtime preparation, after
instructor cleanup; the **budget clock still starts with the original parent**.
The final accounting snapshot matches the refreshed baseline, new owner daily
usage is absent, and the stage comparison is unchanged. Coordinator/proxy/relay
exit and temporary CA removal were checked locally.

## Unchanged window and allowances

The explicitly authorized continuation accepts only the exact published
read-only parent ledger hash
`50318328c26aecc98f776e41768931d3d693c23a3b103c8bbf2f7b5282fdece1`.
It inherits its54AWS charges and first-attempt timestamp; it never clears an
uncertain write or opens a new window. Local tests reject substituted parents
and verify inherited retry/page charges, original deadlines and cleanup reserves.

The first parent attempt was at **2026-10-21 17:29:56 America/Denver**; the last
live outcome was at **17:51:21**, an original-window interval of **1285.463660s**.
The child ledger contains159attempts/outcomes:119newAWS,31API and9static. Including
the parent, totals are:

| Meter                                             |           Used |               Existing maximum |
| ------------------------------------------------- | -------------: | -----------------------------: |
| AWS attempts, including writes/auth/retries/pages |        **173** |                          1,500 |
| Application API, including browser OPTIONS        |         **31** |                            500 |
| Status polls                                      |          **0** |                            240 |
| Static requests                                   |          **9** |                            200 |
| Original window                                   | **21m25.464s** | 120min, including20min cleanup |

All159child attempts have known outcomes. Expected statuses include validation400,
anonymous401, post-delete404 and provider-absence400; there were no uncertain
outcomes or meter stops. Native CLI token refresh consumed one of the AWS charges.
The unchanged work fences reserve20minutes,60API and200AWS for cleanup.

The one authorized additional identity/project was used and cleaned, giving
**seven cumulative disposable identities/projects**. Known campaign consumption
remains **18 admitted steps /16 sends**. The instructor's separate charged usage
is retained in the global count22. There are ten unperformed H–L steps/eight
expected sends, but zero reserve submissions and no inferred identity allowance
or automatic restart. The existing $1 procedural allowance is not a measured
bill or AWS-enforced cap. First-attempt historical AWS totals and all actual
billing totals remain unknown.

## Verification and remaining review

Full repository formatting/lint/tests/build/Go test/vet and eighteen Python unit
cases passed. Additional isolated local transport tests verify inherited54charges
and retry/pagination accounting, while Node regressions verify original-clock
cleanup reserves, completed refreshed-baseline requirements and multi-day-owner
deduplication. The complete local pinned-browser author/reopen/edit regression
passes through the real meter with zero live AWS calls or target sends:

```sh
npm run verify
python3 -m unittest discover -s scripts/attempt-meter -p 'test_*.py'
.build/attempt-meter-venv/bin/python scripts/attempt-meter/test_hl_resume_transport.py
.build/attempt-meter-venv/bin/python scripts/attempt-meter/test_browser_authoring.py
```

H–L runtime evidence remains open. The one-identity allowance is consumed and the
reviewed stop is latched; any further live continuation needs review of these
driver corrections and a deliberate disposable-fixture allowance. No application
fix, release substitution, redeployment, A–G replay, manual draining, broad fault
injection or PITR is included. Existing missing historical observations remain
missing. Trello reflects the completed cleanup and locally tested corrections
while keeping H–L incomplete; updates are read back after publication.
