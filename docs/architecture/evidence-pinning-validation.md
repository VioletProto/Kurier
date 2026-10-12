# Evidence pinning development deployment validation

Davian approved the scoped preview and activation in
[the development review](../development/evidence-pinning.md) at `e6b964f`.
Account `747336059622`, non-root `davian-admin`, region `us-east-2`, stage
`dev-api` were checked before mutation. The 2026-10-12 re-preview matched the
approved scope and the deployment completed successfully.

## Deployed changes and hash verification

Two PUT/DELETE pin routes/integrations/invocation permissions were added to the
existing API Gateway/API Lambda. Existing API, execution worker and cleanup
packages/environment gained pinning capability 1. The existing controlled
endpoint package changed with its shared worker build. Existing code assets
were replaced; no bootstrap infrastructure or standing resources were added.
Cognito, IAM policies, tables/indexes, bucket, queues, schedules, keys and
frontend hosting were unchanged.

`node scripts/verify-evidence-pinning-deployment.mjs` compared deployed
`CodeSha256` with SHA-256 of the exact locally built zip archives before
activation. All four functions were Active / update Successful, ARM64,
`provided.al2023`; the three participating consumers declared pinning 1 and
matched scoped table/bucket configuration.

| Functions                      | Verified CodeSha256 (base64)                   |
| ------------------------------ | ---------------------------------------------- |
| API and cleanup                | `uuDMu2QVfs6TRDXdobO6wchZPxaXJrbz1RSsONfCqJ8=` |
| Worker and controlled endpoint | `sN0XujM6DBXHs/B1dQUJ/pA9TWjhJDhWniudVnltie4=` |

The read-only configuration verifier passed before deployment and after code
verification: active Free plan, account concurrency limit/unreserved concurrency
10, SQS maximum concurrency 2 and batch size 1, standard pollers, no reserved
or provisioned worker concurrency. No account upgrade or quota request occurred.

## Guarded activation readback

`activate-evidence-pinning.mjs --activate` passed on its first execution after
consumer verification. It closed the stage, inventoried existing metadata,
conditionally initialized zero classification counters, verified all five
projects and existing stage quota, then reopened with:

- `state=active`, stage `version=5`.
- `savedRequestsSchemaVersion=4`, `evidencePinningSchemaVersion=1`.
- `recoveryGeneration=dev-api-initial-v1`, unchanged.
- Every initialized project and quota: `pinnedCount=0`, `pinnedBytes=0`.

Independent in-memory before/after hashes of each existing project and stage
quota, excluding only version/new pin counters, matched. Existing storage,
inflight and admission accounting and all other metadata were preserved.
No guard bypass, counter reset, activation failure or recovery resume occurred.

## Actual AWS service and schedule results

The scoped command is `node scripts/aws-integration.mjs TestAWSEvidencePinning`.
It uses disposable SDK identities and real Control/Protected/S3/SQS, the deployed
worker and scheduled cleanup. Direct SDK ownership checks do not establish
Cognito authentication. Only the explicit unauthenticated route check uses HTTPS
in this suite.

Expiry testing moves only one disposable RET's normal expiry into the past,
under stage/project/RET conditional guards, preserving the stored evidence.
This tests actual cleanup and unpin behavior with a fixture timestamp; it does
not establish that thirty wall-clock days elapsed or real clock skew behavior.
No manual cleanup invocation counts as scheduled validation.

First disposable project: `d1d10c28-4276-4976-a38e-9211a2b34ac3`.
The first run passed real pin/repeat/foreign-owner denial, S3 immutability and
classification, pinned protection across a scheduled interval, immediate expired
unpin denial and scheduled expiry cleanup. Its project-deletion observer timed
out after three minutes while the bounded cascade was pending. Guarded fixture
cleanup then verified physical deletion; that operator-driven fallback does not
count as scheduled completion. The observer was extended to eight minutes and
global accounting verification now reconciles against a stable quota version so
concurrent authorized browser fixtures do not create a false baseline failure.
The final run **passed in 445.80 seconds**. It verified real pin/repeat/foreign
owner denial, unchanged S3 bytes/SHA-256/manifest version, correct project/stage
count and byte classification, protection across a scheduled interval, expired
unpin returning expired with the same expiry, immediate access denial and
refusal to resurrect, scheduled expiry drain, and scheduled project deletion
overriding the remaining pin. It reconciled all current pin classification
against a stable quota version without resetting counters. The deployed HTTPS
PUT and DELETE routes returned 401 without Cognito tokens. No operator cleanup
invocation was used to achieve the successful run’s scheduled results.

## Genuine Cognito browser checks

Davian's owner-session report passed the opt-in `validateEvidencePinning()` probe
through real Cognito and HTTPS: pin/unpin, repeated operations and unchanged
sanitized evidence delivery. Safe fixture identifiers:

- Owner `ca677adc-9be3-43e2-8578-46b0464be718`.
- Project `50bc1481-5b1f-4318-bb78-1a4b13726e8b`.
- Execution `9ec9ae9c-4509-4586-a00d-ecb5ac183117`.

Independent AWS audit confirmed that capture remained pinned, its 1,499 S3 bytes
matched SNAP length/SHA-256, and project classification was one pin / 1,499
bytes. No token or evidence payload was printed, committed or persisted.
Davian subsequently reported distinct-user isolation passed: PUT and DELETE
each returned 404. Owner deletion’s strong server readback is completed with
operation `b1134df9-c099-45d0-98cd-39ceb7f46bb0`. Davian reported that the original
finish helper threw an error; its generic error message is unavailable, so that
helper is not recorded as passed and no cause is inferred. An earlier console
import/name issue was addressed with a complete import-and-call expression.
Davian's fresh authenticated `verifyCompletedEvidencePinningProbe` reconciliation
passed: deletion completed, evidence GET / pin PUT / pin DELETE each returned
404, and project deletion was not replayed. This establishes current completion
and access denial; it cannot retrospectively prove the original helper's
immediate-denial observation. The original error message is not remembered and
its cause remains unknown.

While signed in at `http://localhost:5173` using `npm run dev:cloud`, DevTools can
run `await (await import('/src/cloud-validation.ts')).validateEvidencePinning()`.
It returns safe IDs only and leaves its project pinned. Pass that report's
`ownerId`, `projectId`, `executionId` to `validateEvidencePinningIsolation(probe)`
under a distinct second Cognito user (both PUT/DELETE must return 404), then to
`finishEvidencePinningProbe(probe)` under the original owner. The final helper
requires a remaining pin, accepts project deletion, verifies immediate 404, and
waits for scheduled completion. Do not paste tokens, passwords or capture bytes.
If a probe fails, retain its safe IDs and inspect/reconcile rather than replaying
an uncertain write. History/detail UI wording and controls should also be
reviewed in the genuine signed-in session.

A separate prepared genuine expired-unpin fixture uses project
`c435c765-b28e-43fd-8ea0-8ee3a77f885c`, execution
`4b34ffd9-175d-4be1-ae13-aea2ad61fc7f`, under the same participating owner.
Its normal expiry was conditionally moved into the past, without changing
evidence or counters. Preparation passed. Davian reported genuine expired unpin passed: immediate
evidence denial 404, resurrection denial 409, and scheduled cleanup completed
with operation `944dda0e-8683-4c47-b610-f437ec8a6c19`. Independent strong queries
and S3 listings confirmed physical cleanup. The opt-in helper
`validateExpiredEvidenceUnpin({ownerId,projectId,executionId})` verifies an expired
unpin response, immediate 404, no resurrection, then project deletion and
scheduled completion. Davian’s successful response establishes actual HTTP behavior with fixture-controlled age,
not a thirty-day wall-clock test.

## Independent final cleanup

Strong queries and S3 prefix listings verified all four disposable projects:

| Project                                | Completion source                                                                | Control / Protected / S3 |
| -------------------------------------- | -------------------------------------------------------------------------------- | ------------------------ |
| `d1d10c28-4276-4976-a38e-9211a2b34ac3` | Guarded fallback after first observer timeout                                    | Tombstone only / 0 / 0   |
| `4eb46d8b-599f-4740-90c4-e81e6159e8d3` | Successful scheduled SDK suite; operation `265bc2db-f746-45ab-9e73-d32d03f20769` | Tombstone only / 0 / 0   |
| `50bc1481-5b1f-4318-bb78-1a4b13726e8b` | Genuine owner-driven deletion; scheduled completion                              | Tombstone only / 0 / 0   |
| `c435c765-b28e-43fd-8ea0-8ee3a77f885c` | Prepared expiry browser fixture; scheduled completion                            | Tombstone only / 0 / 0   |

Both SDK fixture user META rows were absent afterward; existing Cognito/application
users were preserved. Numeric daily/monthly admission usage remains deliberately
charged under the existing accounting policy; fixture cleanup does not refund
admissions or reset counters. Stage `pinnedCount=0` / `pinnedBytes=0` read back without
any reset. Minimal project tombstones and intentional durable maintenance /
residual-sweep obligations remain; they are not live evidence or secrets.

## Simulations and unperformed checks

The earlier full DynamoDB Local race suite and Chromium fixtures remain local
regression evidence. Injected lost acknowledgements, concurrent conflicts,
quota limit saturation, near-expiry boundaries and recovery-generation changes
are simulations, separate from real AWS results above. The cloud probe verifies
real pin accounting without fabricating stage counters or exhausting quotas.

PITR/restore with pins, interrupted activation/deployment recovery, real clock
skew, elapsed annual KMS rotation, comprehensive negative IAM tests and metered
incremental cost were not performed. There were no recovery actions or restores.
The $0.70/month incremental allowance remains an estimate, not measured billing.
Manual review of the signed-in history/detail controls has not been reported.
Workflows, local-agent execution, MCP and frontend hosting remain excluded.

Post-deployment SST diff passed with **No changes**. Repository verification
passed (44 frontend tests, 10 infrastructure checks, build/lint/format, all five
Go modules test/vet); AWS-tagged race compilation passed separately.
