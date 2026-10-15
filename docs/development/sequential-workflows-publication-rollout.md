# Publication retry fix: development rollout

Davian approved the code-only development deployment of `115a7a2`. Deployment
and validation completed in `dev-api`, account `747336059622`, `us-east-2`, using
non-root `davian-admin`. **No capability activation was performed.**
The [safe readback artifact](sequential-workflows-publication-rollout-readback.json)
records hashes, counters, diagnostic counts and cleanup proofs.
This follows the [publication investigation](sequential-workflows-publication.md);
prior successful browser acceptance and cleanup remain separate evidence.

## Deployment and preservation

The refreshed SST diff contained only the four existing function code assets
and package updates: API, cleanup, worker and the shared controlled endpoint.
Deployment updated those four functions, uploaded their replacement assets and
removed superseded assets. No IAM, routes, configuration, standing resources,
bootstrap resources, Cognito, quota or concurrency changes were planned/applied.
Postdeployment SST diff reported **No changes**.

All four deployed archives matched their final local deployment ZIP SHA-256,
with Active/Successful state, ARM64 and provided.al2023:

| Functions                    | Deployed CodeSha256, base64                    |
| ---------------------------- | ---------------------------------------------- |
| API / cleanup                | `4YWNTr5y/i6zhhF5apPwZb4WYOWPRH3z8ZSDTZ1USUQ=` |
| Worker / controlled endpoint | `4PdjuCXGE/NxN1UcEG8RlAdadFcbemcak/fscHodRoM=` |

SST rebuilt the ZIPs between preview and deployment, so the final archive hashes
above differ from the first preview's hashes. This was inspected before runtime
validation: both deployed entrypoint build records name revision
`115a7a2c9ff43f96f282066901e66f9803718612` with `vcs.modified=true`; the only tracked
working-tree differences at that build were generated SST declaration formatting.
Runtime/API/infrastructure source was unchanged. The final archive/readback
checks and subsequent no-change diff establish the deployed bytes; a separate
byte-level comparison against the overwritten first-preview ZIPs was not made.

Hash-only snapshots taken before deployment and after it matched exactly:

| Scope                                                              | Preserved SHA-256                                                  |
| ------------------------------------------------------------------ | ------------------------------------------------------------------ |
| Complete stage marker                                              | `325ad85a1650a25adcf6b1dee89ef3df7d07a051add93eb514b2ed650a1e256f` |
| Existing project/quota accounting records                          | `39b0877fc3d5701c066bd46428366e48e5166799372386911e121abafeb1f9a4` |
| Four roles' inline policies and attachments                        | `128b578169bd717266ad2561890cc27007d8560ca16019dddb9c3e2c577ae3b6` |
| Existing function configurations excluding package/update metadata | `2bc9d8ac0b39d865b3943493b1792faa639cf8013923c676763271d5a002a884` |

Routes also matched exactly. The deployment verifier rechecked all nine workflow
route integrations, purpose-scoped KMS permissions, protected/output scheduling
permissions and the existing minute maintenance schedule. Existing capability
markers remain active/v9, savedRequests6/workflows1, local-agent1/pinning1 and
recovery generation `dev-api-initial-v1`. Free ACTIVE, account/unreserved
concurrency10, SQS max2/batch1, standard pollers and no reserved/provisioned
concurrency remain unchanged.

## Genuine AWS and browser observations

Setup used the SDK only to prepare a disposable project under Davian's existing
Cognito owner. Davian submitted each workflow once from the normal signed-in
browser and kept normal run-detail polling active. He confirmed completed/failed
as expected and explicitly confirmed **no direct fixture-URL calls**. No fake
identity/session, broad AWS fault or automatic HTTP retry was used.

Project: `d4969c76-a65a-4338-90e8-f0a2f542dd2e`.

| Run                                    | Result                                                                                                                                                                      |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `fee8923f-e4b9-4982-88c1-804c5ac54fb8` | Two-step raw-token extraction → rendered Bearer binding; completed.                                                                                                         |
| `f2f0005c-4075-42eb-9eff-948bfbd0c254` | Four steps: producer/consumer completed, controlled twelve-second delay exceeded the existing response-header timeout and finalized failed/unknown, final consumer skipped. |

Strong readback and real S3 verification established:

- Five finalized jobs, each attempts=1/fence=1; five settled/published tickets
  and five immutable manifests. Evidence rereads were byte-for-byte unchanged.
- Both consumers authorized successfully. Raw synthetic tokens and their rendered
  Bearer values were absent from sanitized evidence and Control/Protected
  plaintext. A raw-token substring check detects every rendered form as well.
- Terminal Protected partition empty; project inflight=0 and reserved evidence
  storage=6,013 bytes before deletion. Six admitted slots remain charged; the
  skipped step is not refunded.
- Independently logged target sends: success fixture token1/consumer1; stop
  fixture token1/consumer1/delay1. **Exactly five distinct target request IDs**;
  no skipped-successor send. Counts remained unchanged after the timeout job's
  original two-minute lease deadline, before project deletion.

The fixed worker emitted **35** genuine publication diagnostic records: five
apiece for claim, intent, registered, writing, PUT acknowledgment, settled and
finalize. Every boundary code was `ok`; every fence was 1. The schema contained
only the fixed phase/code, validated UUID identifiers and numeric fence. The
read-only audit inspected **295** scoped API/worker/cleanup/endpoint log events
and found neither raw nor rendered synthetic values. The audit is bounded to
this validation window and these disposable values, not a universal log audit.

**No genuine conflict event occurred** (`observedConflictEvents=0`). The run
validates deployed normal execution, unknown stopping, diagnostics and polling;
it does not claim that an AWS cancellation/retry was observed. The controlled
upstream timeout is an intentional target behavior, not a broad AWS outage.

## Cleanup and accounting

Deletion `56d57f4b-6679-451f-a340-670dba4c8ba7` completed through the existing
fenced project deletion and bounded maintenance paths. Cleanup was manually
assisted, not a schedule-only timing test. Strong Control tombstone-only /
Protected0 / complete S3-prefix-empty proofs passed. Real user META, capability/
generation marker and cumulative admission usage remained unchanged by cleanup.
External residual/drain coordination remains intentionally for late-write safety.

After fixture cleanup, a separate project deletion was accepted during final
readback: project `af4672de-5a97-4bcf-a60b-50ec5ae8101e`, operation
`fd787057-f232-42aa-a9e0-5450866e7a51`, started `2026-10-15T06:10:54.661088872Z`.
This task did not submit or manually advance that deletion. Its existing
scheduled drain released additional storage: readbacks moved from the earlier
10,390-byte baseline through 9,310 bytes to **6,627 bytes**, with stage, remaining
project counters and remaining upload tickets reconciling. The final existing
project retained six published tickets totaling 6,627 bytes; the independently
deleting project's storage reached zero. Its metadata cleanup is outside this
fixture's completion claim.

Pins=0 and unsettled jobs=0. Monthly admission count is **93**: prior87 plus the
six authorized validation slots. No refund/reset occurred. Strict accounting
hash equality was verified across deployment before fixtures; later accounting
changes comprise the intentional validation admissions/cleanup and concurrent
accepted project cleanup, rather than an invariant of unchanged storage. IAM,
configuration, routes, capabilities, Free plan and quota/concurrency settings
still match their predeployment baselines. Deployed hashes remain the values
above.

## Verification and remaining gaps

Passed for this task: repository format/gofmt/lint, 52 frontend tests, ten
infrastructure tests, production build, six Go modules' tests/vet; focused Local
race regressions for transaction conflict classification/token preservation,
publication boundaries, exhaustion/no replay, source/deletion/recovery guards
and diagnostic containment; AWS-tagged helper vet; genuine setup, immutable
readback, log/send-count/containment audits and physical fixture cleanup.

Injected conflicts, KMS/S3/HTTP fixtures and lease-clock advancement remain
**simulated regression coverage**. No broad AWS conflict/throttling/outage was
induced. A real AWS conflict retry, process-crash/ACK-loss fault, complete
credential-expiry-crossing runtime suite, negative IAM, PITR, elapsed retention/
skew, maximum-size memory/latency and metered-cost checks remain unperformed.
The earlier historical publication interruption still lacks original error
reasons and cannot be conclusively attributed. Its unknown cause does not
invalidate the independent successful browser, cleanup or rollout checks.

The opt-in `TestAWSWorkflowPublicationRollout` helper retains only public fixture
IDs in `/tmp/kurier-publication-rollout-fixture.json`, refuses duplicate setup,
never submits a workflow or replays HTTP, and can reconcile physical cleanup.
`node scripts/audit-workflow-publication-logs.mjs` performs the bounded safe
read-only log audit using that public receipt. Runtime source was not changed
beyond approved commit115a7a2; the rollout commit adds verification tooling and
documentation only.
