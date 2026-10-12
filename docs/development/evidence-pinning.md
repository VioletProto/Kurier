# Evidence pinning implementation and deployment review

The [accepted contract](../architecture/evidence-pinning-contract.md) at
`03a36c8` is implemented with the accepted wording: **Protected from automatic
expiry while pinned.** Workflows, local-agent execution, MCP and frontend hosting
remain outside this slice. The preview at `e6b964f` was approved on 2026-10-12.
Compatible consumers are deployed and capability activation passed. The
[deployment validation record](../architecture/evidence-pinning-validation.md)
separates actual AWS results, genuine Cognito reports and remaining checks.
The preview and local verification below are preserved as the reviewed baseline.

## Implementation

Owner-authorized, empty-body PUT/DELETE
`/projects/{projectId}/executions/{executionId}/pin` return the current retention
DTO and server time. Repeated operations preserve versions, timestamps and
counters. A new pin verifies a terminal job, settled publication ticket, snapshot
length and SHA-256 against the published sanitized S3 object. It never changes
that object or snapshot. Expired, deleting or unpublished evidence cannot be
resurrected. New pins require more than five seconds before normal expiry.

Pin transitions atomically coordinate project, stage quota and RET versions,
with authorization and capability guards. Unknown transaction outcomes reuse
the exact request token before guarded reconciliation; conditional conflicts
retry with fresh state/time. Project and stage quotas are respectively 100 pins /
256 MiB and 200 pins / 512 MiB, counting each pinned capture once. Ordinary
storage reservation accounting remains unchanged. Missing or inconsistent
counters fail closed; unpin and deletion remain available when over quota.

Pinned RET records leave the existing due index. Last unpin restores the original
expiry; an already expired capture is immediately unavailable and eligible for
cleanup, without a new grace period. Retention cleanup fences active project and
unpinned deleting RET throughout its existing resumable drain. Project deletion
releases pin accounting and drains pinned evidence too. Unknown cleanup records
retain the existing durable review obligation rather than claiming completion.

History and detail display effective retention, normal expiry and pin controls.
The UI handles loading, conflicts, quotas, unavailable evidence and uncertain
writes without automatic replay. Expired unpin requires confirmation and clears
open evidence. Server-time elapsed tracking disables pinning near expiry.
No evidence or credentials are written to browser persistence.

## Local verification

Final verification passed on 2026-10-12: repository formatting/lint, 44 frontend
tests, 10 infrastructure/migration tests, frontend production build, all five Go
modules test/vet, the full API integration race suite and three Chromium
scenarios. SST preview also built the API and worker Linux binaries.

Run DynamoDB Local with `docker compose -f compose.dynamodb.yml up -d`, then:

```sh
npm run verify
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration
```

The integration suite exercises owner/non-owner HTTP access, repetition,
concurrent same/opposite operations, project/stage count and byte limits,
cross-project contention for the last stage slot, missing accounting,
unpublished/unavailable captures, exact near-expiry boundaries, delayed
verification/commit, lost acknowledgements, recovery-generation changes,
cleanup races and multi-chunk deletion of pinned captures. It verifies unchanged
snapshot/object bytes and checksums and zero accounting after deletion.

React tests cover loading, late-unpin confirmation, quota/conflict refresh,
uncertain writes, unavailable evidence and project switching during a request.
Chromium exercises the actual local Go routes and DynamoDB transactions through
pin, reopen, expiry, unpin, refusal to resurrect, foreign-owner access and project
deletion. Cognito verification and S3 are controlled fixtures in these tests;
the test-only clock/cleanup routes exist only in the browser fixture.

## AWS preview for approval

Read-only preview on 2026-10-12 used account `747336059622`, non-root identity
`davian-admin`, region `us-east-2`, stage `dev-api`, and the existing SST bootstrap.
The generated SST diff proposes:

- Two API Gateway routes and integrations/permissions targeting the existing API
  Lambda: PUT and DELETE for the pin endpoint.
- Code/environment updates for the existing API, execution worker and cleanup
  functions, declaring `KURIER_EVIDENCE_PINNING_SCHEMA_VERSION=1`.
- A code asset update for the existing controlled endpoint because its package
  shares the worker build directory; its behavior and configuration are unchanged.
- Replacement of corresponding existing bootstrap code assets; no new bootstrap
  infrastructure, tables, indexes, buckets, queues, keys or frontend hosting.

No IAM policy expansion, Cognito change, schedule change, concurrency change or
account plan/quota change appears in the scoped infrastructure changes. The
read-only cloud configuration verifier confirms the active Free plan, account
concurrency quota 10, worker SQS maximum concurrency 2 / batch size 1, standard
pollers, and no reserved or provisioned concurrency.

### Compatibility activation and rollback

Deployment alone leaves pinning disabled. Legacy capability 3 remains usable by
new consumers; pin controls are disabled until activation. Old binaries reject
saved-request capability 4 and cannot rewrite pin accounting.

`node scripts/activate-evidence-pinning.mjs` is read-only. Its current inventory
reports five projects, an existing stage quota, and consumers not yet ready
(expected before deployment). After approval and deployment of **all three**
consumers, `node scripts/activate-evidence-pinning.mjs --activate` separately
requires their successful pinning environment markers and rechecks Free plan /
concurrency settings. It conditionally closes the stage, preserves the recovery
generation, inventories metadata, rejects pins/unknown RET metadata/unresolved
work, initializes zero counters using guarded updates, verifies them, and then
atomically activates saved-request capability 4 / pinning capability 1.

Activation is resumable while closed; failures never guess an automatic reopen.
Review durable metadata before resuming. Once any pin exists, do not downgrade
to capability 3 or restore an old binary: retain capability 4 and compatible
consumers while reviewing recovery. Existing restore/generation procedures still
apply. No migration, zero initialization or stage closure was executed during preview
preparation. The separately approved activation and preservation readback are
recorded in the deployment validation record.

## Refreshed incremental estimate

Using the existing development baseline of approximately $3.19/month, allow
approximately **$0.70/month incremental**, or **$3.89/month total**, before Free
plan credits/free allowances. This is a planning estimate, not a billed amount or
hard spend ceiling. The storage/pin quotas are application limits and do not
change AWS account quotas.

Ohio public pricing was refreshed on 2026-10-12 using AWS price-list catalogs:
S3 Standard $0.023/GB-month and GET $0.40/million; DynamoDB writes $0.625/million,
reads $0.125/million, storage $0.25/GB-month, PITR $0.20/GB-month and on-demand
backup $0.10/GB-month; HTTP API $1/million; Lambda requests $0.20/million and
ARM compute $0.0000133334/GB-second; paid internet transfer $0.09/GB. These rates
exclude taxes and credits; the estimate deliberately does not subtract free
allowances. Sources: [S3](https://aws.amazon.com/s3/pricing/),
[DynamoDB](https://aws.amazon.com/dynamodb/pricing/on-demand/),
[Lambda](https://aws.amazon.com/lambda/pricing/),
[API Gateway](https://aws.amazon.com/api-gateway/pricing/), and regional catalogs
under `pricing.us-east-1.amazonaws.com/offers/v1.0/aws/`.

| Incremental workload allowance                                      | Monthly estimate |
| ------------------------------------------------------------------- | ---------------: |
| 512 MiB retained S3 evidence and 1,000 extra GETs                   |           $0.013 |
| 30,000 DynamoDB write units and 100,000 read units                  |           $0.032 |
| 256 MiB additional metadata/index storage, PITR and one backup copy |           $0.148 |
| 2,000 HTTP calls and 2,000 ARM invocations at 256 MiB / 0.5 seconds |           $0.006 |
| 4 GiB paid transfer plus logs/cleanup margin                        |           $0.500 |
| Rounded incremental allowance                                       |        **$0.70** |

This assumes about 1,000 pin/unpin operations and 1,000 additional evidence views
per month within existing execution limits. The DynamoDB allowance includes
transactional charging; large records, contention/retries, heavy delivery or
additional backup copies can exceed it. Pin storage is part of existing physical
storage accounting, not a second copy. There are no new fixed monthly resources.
Existing budget alerts and review remain appropriate.

## Remaining validation after approved deployment

See the deployment validation record for completed AWS and genuine Cognito
checks. Real clock skew, PITR/restore with pinned captures, interrupted recovery
and metered incremental cost remain follow-up validation. Local Chromium
results use controlled auth/object fixtures and are separate from genuine
Cognito reports. Frontend hosting remains outside this slice.
