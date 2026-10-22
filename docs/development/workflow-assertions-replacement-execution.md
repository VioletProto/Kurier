# Workflow assertions correction and replacement execution review

Status: **prepared for review; execution held**. Davian authorized correcting the
activator, local verification and this replacement allowance proposal. This does
not authorize deployment, maintenance closure, activation, identity/project
creation or target requests. No live AWS inventory or mutation was performed by
this correction task. The [first attempt](workflow-assertions-rollout.md) and its
[receipt](workflow-assertions-rollout.json) remain historical evidence.

## Corrected settlement proof

The application `publishOutbox` writes `published`, including when a job is
still running. The activator now accepts that state **only with all of these
metadata proofs in the same table and project partition**:

- A schema 1 outbox with valid project/job/execution IDs and the exact `OUT#job`
  key; the matching `JOB#job` is an `executionJob`, schema 1, completed or failed.
- Job and outbox agree on project, owner, job, execution, optional workflow run/
  position and recovery generation. The matching execution anchor has the same
  identity and terminal state. Optional run/position values obey the application's
  UUID and zero-based position bounds; position 0 is legitimately omitted.
- `EXEC#execution#SNAP` is the schema 1 immutable publication manifest for that
  same project/execution. Its object key is within that execution's evidence
  prefix; checksum is a SHA-256 digest and bytes are positive and at most 4 MiB.
- Exactly one upload for that job exists in that partition. It is schema 1,
  `published`, `writeSettled:true`, and agrees on writer identity/generation
  and manifest object key/checksum/byte count. All other inventory/upload guards
  continue to apply independently.

Pending, running, queued, deleted or unknown job states, `pending`/`sent`/unknown
outbox states, missing or inconsistent identity/publication records, duplicate
tickets and unknown kinds/schemas fail closed. Expired/missing publication or
unsettled uploads are not exemptions: reconcile separately, never rewrite rows
to satisfy inspection. Even a terminal pre-dispatch failure normally publishes
immutable evidence through the same application path and needs these proofs.
The inspector checks durable metadata coherence; it does not claim a separate
S3 byte download/checksum verification. The runtime campaign must still verify
stored bytes independently.

Only the metadata projection adds `ownerId` and `position`. Consequently fresh
accounting/approval digests must be generated with this reviewed tooling; old
approval records cannot be reused. The default is still read-only. Full marker
CAS, counter conditions, all other inventory checks, reader/frontend pins,
IAM/configuration/concurrency checks, queue emptiness, transaction bounds,
one-attempt writes and no automatic reopen/retry behavior remain in force.

## Actual local regression

`TestIntegrationLegacyMaintenanceClosureInspection` uses DynamoDB Local and the
real service's two-step submission, worker publication and `Maintain` methods.
Controlled loopback HTTP/KMS/queue/object fixtures supply dependencies; no real
AWS, Cognito or live target is involved. It does not replace outbox rows with
test-authored `published` metadata.

The test completes a legacy schema 7 two-step workflow with exactly 2 fixture HTTP
sends. Before maintenance, actual pending outboxes fail inspection with zero
writes. Actual local maintenance settles both to `published`; the production
pure closure inspector then passes with zero writes, while both schema 1 evidence
objects remain available and byte-identical. Actual stored object lengths and
SHA-256 values still match their manifests. The stage, accounting metadata and
two-send count remain unchanged by inspection. The bridge imports only the pure
guard and supplies no AWS or write adapter.

The JavaScript suite covers completed and failed retained publications and
negative mutations/removals for pending/running/unknown states, wrong kind,
owner/project/job/execution/run/position/generation, anchor mismatch, missing
job/anchor/manifest/ticket, manifest scope/checksum/size errors, unacknowledged or
writing uploads, mismatched uploads and duplicate tickets. Rejections verify
zero writes and use guard errors, not incidental JavaScript exceptions. Existing
CAS/race/ACK-loss/reader/frontend/accounting/queue regressions remain required.

## Exact application release preserved

Application source remains `68d9f218225c7092a1e4fca0b66a21421d862a22`.
The [release manifest](workflow-assertions-release.json) is byte-for-byte unchanged:
SHA-256 `920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6`.
API/cleanup ZIP SHA-256 is
`f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32`;
worker/fixture ZIP SHA-256 is
`15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379`.
All four retained archives retain their clean source provenance and exact bytes.
Frontend release stays
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`.
No application production code, package, dependency, IAM, infrastructure or
durable cloud row is changed. The Go addition is integration-test-only. Do not
build deployment ZIPs from this later tooling commit.

The last [equivalent preview](workflow-assertions-rollout-preview.json) remains
historical, not a fresh preview from this correction. A future approved attempt
must recheck live guards/bootstrap effects and obtain an equivalent preview for
the same pinned bytes before mutation. Preserve old rollback bytes, compatible
reader/frontend order, temporary closure and accepted rollback limitations.

## One replacement allowance proposed

The [machine-readable allowance](workflow-assertions-replacement-allowance.json)
records these explicit counters; it is not an execution approval.

| Quantity              |  Prior attempt, consumed |            Replacement proposal | Cumulative ceiling/result                                            |
| --------------------- | -----------------------: | ------------------------------: | -------------------------------------------------------------------- |
| Disposable identities |               2, cleaned |                    2 additional | 4 total created maximum                                              |
| Disposable projects   |               2, cleaned |     2 additional, one per owner | 4 total created maximum                                              |
| Admitted steps        |                        2 |     A–L: 24 planned + 2 reserve | 26 planned cumulative; 28 maximum                                    |
| Two-step submissions  |                        1 |           12 planned +1 reserve | 14 maximum cumulative                                                |
| Worker target sends   | 2, independently audited | 20 expected + at most 2 reserve | 22 expected cumulative; at most 24 with reserve, absolute 28 ceiling |

The historical 2 steps/sends are never reset or refunded. Only 2 admitted reserve
steps remain after repeating A–L, not the original 4. The mathematical remaining
send ceiling is 26, but the reviewed replacement sequence permits at most 22
sends because only 2 extra admitted steps are reserved. Unused send capacity
does not authorize extra admissions, unexpected sends or HTTP replay. Reserve
is one deterministic two-step correction with a known prior outcome, explicitly
recorded before submission; an uncertain outcome stops execution.

The prior identities/projects and A evidence were normally removed. Create the
replacement pair only after execution authorization; do not silently reuse
deleted identity names. Seed a new legacy A under active7, settle/prove its
publication, run corrected inspection and keep its evidence retained through
closure, deployment, activation and frozen rerun B. Repeat the accepted A–L
definitions, exact decimal DTO/round-trip cases, ownership/MCP/pin checks and
cleanup with at most 6 saved requests / 12 workflow definitions / 1 scoped MCP token.
Allow only the five reviewed controlled modes; direct fixture probes remain 0.
Frozen reruns preserve URLs: audit expected fixture-UUID reuse using distinct
gateway request IDs. No fixture change or arbitrary target is proposed.

This companion allowance explicitly supersedes the original packet's
two-identities/two-projects cumulative limit for **this one replacement pair**;
the pinned manifest is not edited to disguise the exception. Step/send ceilings
and all application quotas/accounting/concurrency/IAM remain unchanged.

## Replacement time, read and cost limits for review

- **120 minutes**, beginning before the first replacement live preflight read,
  includes preview/deployment/verification and a reserved **20-minute normal
  cleanup window**. Admit no new runs after minute 100. Stop earlier if remaining
  time cannot cover the next phase and cleanup. This is a new attempt clock;
  preserve the prior measured 501.638s campaign / 238.034s cleanup separately.
- **500 application API calls**, including at most **240 status polls**, at least
  **2 seconds apart**. Count browser, owner, anonymous, MCP and cleanup calls,
  including failed attempts; do not hide SDK retries or browser polling.
- **1,500 scoped AWS read API attempts** for the replacement attempt, including
  failed reads, pagination, before/after guards, logs, object reads, IAM, SST
  preview/deployment SDK reads and cleanup verification. A response with a
  continuation token is not a complete audit; bounded pagination must consume
  the same read/time budget. Resolve the retained fixture log-group name from
  Lambda configuration; retain only allowlisted UUID audit metadata.
- Start a durable non-secret ledger at zero for this attempt's API/read counters,
  with **2 historical steps and 2 historical sends** carried into cumulative
  admission/send counters. Record each attempt before issuing it. Account for
  automatic SDK retries as attempts. Reserve enough reads/API calls for cleanup;
  stop discretionary verification when the remaining allowance only covers it.
  No phase with unmetered AWS SDK calls may be treated as covered by a CLI-only
  counter. **End-to-end SDK read-meter coverage is an execution prerequisite**;
  this correction does not implement or validate that future execution runner.
  If coverage is unavailable, keep execution held rather than claim the cap.
- **$1.00 additional incremental allowance** for this replacement attempt,
  including its two Cognito MAUs and normal cleanup. The accepted Ohio planning
  rates suggest under $0.10 for the bounded campaign; actual billing is delayed,
  so this is procedural, not an AWS-enforced spend cap or verified total cost.
  No standing-resource, quota, concurrency, IAM or budget-resource change is
  permitted. An observed allowance overrun stops discretionary work.

The first attempt's **total AWS reads and billed incremental cost remain
unknown**. Its 89 coordinator operations include mutations and exclude external
preflights/SDK reads; they are not an 89-read total. No estimate is substituted
for the missing total, no unused read/billing allowance is inferred, and no
combined actual read/cost total or remaining original dollar balance is claimed.
The prior 25 API calls, 2 polls, 2 admissions and 2 target sends remain known history.

## Review and execution holds

Review this correction, the single replacement pair and its 26-new-step maximum,
the limits above and read-meter prerequisite. Execution remains held. If later
authorized, use a clean checkout of the newly accepted tooling commit with the
same pinned artifacts. Generate separate close and activation records only from
fresh passing readbacks under that execution authorization; include this exact
tooling commit, unchanged manifest digest and fresh marker/accounting hashes.
Neither old approval records nor synthetic approval examples authorize writes.

Stop on drift, conditional failure, uncertain writes, unexpected sends, missing
send-count proof or exhausted budgets. Never automatically reopen or replay HTTP.
Normal cleanup must prove owner/MCP denial, completed project deletion, retained
tombstones/residual registration, empty S3/Protected prefixes and healthy shared
accounting without resetting usage. Then sign out/delete only the replacement
identities and verify provider absence; retain the offline-JWT expiry caveat.
If a closed/uncertain stage prevents normal cleanup, preserve identifiers and
report the obligation; do not reopen it or manually drain rows/objects.
Broad live fault injection and elapsed-retention/PITR remain separate. Previous
operator-reported observations stay separate from independently verified results.

## Completed local checks

`npm run verify` passed formatting/gofmt, lint, 64 web and 15 infrastructure
checks, TypeScript/Vite build, and tests/vet in all seven Go modules. Recommended
Node ESLint passed for the changed guard, regression suite and test bridge.
The full execution integration race suite passed against DynamoDB Local,
including the actual-maintenance regression. All 82 JavaScript activator tests
passed, including the local DynamoDB CAS/atomic accounting case (no skips).
All six release-pin checks passed against the four retained archives and the
unchanged manifest. The test-created local container/network are removed after
verification; unrelated containers are preserved. These are local fixture
results, not fresh deployed runtime or AWS/IAM evidence.
