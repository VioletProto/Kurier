# Evidence pinning and unpinning: accepted contract

Status: **Accepted by Davian**, 2026-10-12, at `03a36c8`, with UI wording
changed to “Protected from automatic expiry while pinned.” Scoped API,
accounting, cleanup, React implementation, API contract synchronization and
local/race/browser tests are authorized. AWS deployment preview and refreshed
costs are authorized for review; the scoped deployment and guarded activation
were subsequently approved at `e6b964f`. Keep the
Free plan, account quota and existing concurrency settings unchanged.
Acceptance is not evidence of implemented behavior or completed validation;
actual results and remaining gaps are recorded in
[implementation and deployment review](../development/evidence-pinning.md).

Branch: `feat/evidence-pinning`, created from refreshed `origin/main`
`c6ecc991115441328984f519d155852b03db3932`. That main commit merges
[deployment-validation PR #7](https://github.com/VioletProto/Kurier/pull/7);
its head `0d147fa` was verified as an ancestor before branch creation.
The working tree was clean.

Authority and context read:

- [Accepted architecture](README.md), [ADR 0001](../decisions/0001-sst-infrastructure-candidate.md)
  and [ADR 0002](../decisions/0002-serverless-persistence-topology.md).
- [Conditional retention](w1-data-model.md#conditional-retention-and-workflow-context),
  [publication/deletion design](w1-system-design.md),
  [budget and quotas](w1-review-decisions.md#budgets-and-application-quotas),
  and accepted users/projects, saved-request, protected-secret and
  [cloud-execution contracts](cloud-execution-evidence-contract.md).
- Current [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
  and [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit),
  fetched read-only on 2026-10-12; no synchronization performed.
- [Retention/pinning card](https://trello.com/c/gH2vGh9p) and current
  [board](https://trello.com/b/dRYhnjS6/kurier-capstone). The card had no checklists at preparation; a review/acceptance checklist
  was subsequently added. Ownership, redaction and immutable publication dependencies have
  repository implementation/validation evidence. Broader contract review remains
  Doing; this packet resolves only this slice's remaining wire choices. No
  configured WIP limit was found. Preparation completed; acceptance was recorded and implementation is Doing.
  Product/runtime validation remains separately tracked.

## Scope and existing behavior

Extend the single saved-request cloud execution slice with owner pin/unpin and
React history/detail controls. Reuse Control `EXEC#id#RET`, immutable `SNAP`,
upload tickets, existing project gates, stage recovery generation, GSI2 discovery
and resumable cleanup. Workflows/run context, local-agent execution, MCP and
frontend hosting are outside scope. Do not add run counters or a future context
abstraction for standalone executions. Saved definitions survive history cleanup.

The accepted baseline already specifies project-scoped PUT/DELETE, idempotent
200 retention responses, 30-day normal retention, a five-second new-pin cutoff,
original expiry after unpin, conditional cleanup and project deletion overriding
pins. DTO additions, detailed errors, pin quotas and UI below are accepted.

Current main, inspected rather than inferred from older planning documents:

- `services/api/execution/worker.go` publishes terminal EXEC/JOB, insert-only
  SNAP and live RET together. RET has `pinned`, original `normalExpiresAt`,
  availability and version; no pin routes, timestamps or pin counters exist.
  Evidence GET bounds bytes and verifies SNAP length/SHA-256. Availability uses
  the existing values `available` and `unavailable`.
- `store.go`/`history.go` hide expired unpinned or deleting executions on strong
  hydration, even before physical removal. History uses bounded signed cursors;
  its execution version is distinct from RET version.
- `maintenance.go` skips pinned RET candidates, CAS-marks RET deleting, settles
  upload tickets before removing manifests/anchors and releases reserved storage
  only after S3 deletion/absence verification. Current publication reserves exact
  encoded bytes before PUT, within 1 GiB/project and 2 GiB/stage. Do not introduce
  a duplicate storage charge at pin time.
- `DrainProject` ignores retention protection, drains both stores, empties the
  project S3 prefix and registers durable residual sweeping. Unknown records and
  uncertain PUTs preserve pending obligations. There is no blanket evidence S3
  age lifecycle or authoritative live-record TTL.
- Important adaptation: current `expire` relies on its caller's pin/expiry
  checks and `drainExecution` later guards the current project state. Require
  explicit pin/expiry/state predicates at cleanup claim and a retention-specific
  RET fence throughout draining; never silently change a retention drain into
  a project-deletion drain when the project changes state.

## Accepted routes and DTOs

All routes require the existing verified Cognito access token, enabled application
user, active owned project and active compatible stage/recovery generation.
Authorize before resource/state checks; foreign, missing or deleted project or
execution returns 404 without existence disclosure. Preserve the shared safe
error envelope and `Cache-Control: no-store`. No credential input, pin label,
note, evidence patch or secret value is accepted.

| Route under `/api/v1/projects/{projectId}` | Successful behavior                                                               |
| ------------------------------------------ | --------------------------------------------------------------------------------- |
| `PUT /executions/{executionId}/pin`        | Set the single owner pin; 200 retention metadata.                                 |
| `DELETE /executions/{executionId}/pin`     | Remove that pin; 200 retention metadata, including an immediately expired result. |

Both requests have an empty body; reject nonempty bodies with 400
`invalid_request`. No submission `Idempotency-Key` or client `If-Match` is
required: these are desired-state operations with internal conditional versions.
Concurrent opposite requests use the last valid committed transition; no browser
may automatically replay an opposite intention after refreshing a conflict.
This concurrency policy was accepted for pin operations independently of request
revision ETags.

Accepted response (timestamps shown illustratively):

```json
{
  "data": {
    "retention": {
      "normalExpiresAt": "2026-11-11T12:00:00.000000000Z",
      "pinned": true,
      "evidenceAvailability": "available",
      "version": 1,
      "pinnedAt": "2026-10-12T12:00:00.000000000Z",
      "state": "live",
      "effectiveStatus": "pinned",
      "pinEligible": true
    },
    "serverTime": "2026-10-12T12:00:00.000000000Z"
  }
}
```

Extend the existing history/detail/status `retention` object with the same
fields. Its existing fields retain their meanings. Preterminal `retention` stays
null. `version` is RET's nonnegative integer CAS version, not EXEC version;
`pinnedAt` is UTC or null, cleared on a real unpin. `state` is `live|deleting`.
`effectiveStatus` is `pinned|retained|expired|deleting`: deletion wins, then pin,
then unexpired retention, otherwise expired. Availability remains independently
`available|unavailable` and never claims that an expired body is accessible.
`pinEligible` describes current state/availability/time eligibility, not a quota
reservation or a guarantee of success. UTC `serverTime` is the validated decision
time used for the response. Return no object key, checksum, job input or capture.
Expired/deleting retention metadata may be returned by DELETE while its anchor
survives; ordinary history/detail/evidence access retains existing expiry denial.

| Condition after ownership checks                                                           | Accepted response                                         |
| ------------------------------------------------------------------------------------------ | --------------------------------------------------------- |
| Missing/foreign anchor or inactive project                                                 | 404 `not_found`                                           |
| Preterminal, unpublished or deleting evidence on PUT                                       | 409 `conflict`                                            |
| New pin at or inside five-second cutoff, including expired-but-physically-present evidence | 409 `pin_window_closed`                                   |
| Known unavailable, missing or checksum-invalid published evidence on a new pin             | 503 `evidence_unavailable`; no pin/quota transition       |
| New pin exceeds count or pinned-byte limits                                                | 409 `pin_quota_exceeded`                                  |
| Conditional conflict after bounded fresh attempts                                          | 409 `conflict`; refresh required                          |
| Unresolved transaction acknowledgement or incompatible/recovering stage                    | 503 `service_unavailable`; reconcile before retry         |
| Existing authentication/throttling/validation failures                                     | Existing 400/401/403/429 mappings; 429 honors Retry-After |

Repeated PUT on an already pinned live anchor returns 200 without changing RET,
`pinnedAt`, quota or expiry, even beyond normal expiry. If availability has since
become unavailable, the repeat returns its truthful unavailable metadata; it
does not create a pin or restore data. A new false-to-true transition always
requires available published evidence. Repeated DELETE on an already unpinned
anchor returns 200 without counter/version changes, including after logical
expiry or a cleanup claim while the anchor survives. After anchor removal it is 404. Preterminal DELETE is 409 because no published retention record exists.
These repeats describe current state, not a permanently replayed response.

## Retention invariants and atomic protocol

One standalone capture has one Boolean owner pin: no reference-counted multiple
pins per capture. "Last unpin" means removing that one pin. Pinned evidence keeps
its original sanitized S3 bytes, SNAP length/checksum, summary and frozen source
identity. Never rerun HTTP, republish, extend expiry, copy bodies, keep terminal
protected inputs or rewrite captured redaction policy to pin it. All logical
changes belong to RET and coordination/quota metadata.

1. Strongly read stage/user/project, anchor, RET and published SNAP. Reject
   unknown kinds/schemas/attributes. A new pin requires terminal `completed` or
   `failed`, matching published manifest/ticket identity, RET live/available and
   original expiry strictly greater than `decisionTime + 5 seconds`.
2. For a new pin, reuse a bounded internal evidence verification path before
   admission: fetch only the published object, verify exact length/SHA-256 using
   existing evidence limits and discard bytes without logging or persisting
   them. A recent frontend GET or stale availability flag is insufficient.
   Refresh decisionTime immediately before each new CAS attempt. An already
   pinned repeat and unpin need no body read. S3 cannot join a DDB transaction:
   availability is verified before the decision; application deletion is fenced
   below. External object loss can still make a pin unavailable, never fabricated.
3. Commit a transition in one bounded DDB transaction, with consistent condition
   order: stage/generation, enabled user, active project owner/version/epoch,
   anchor terminal/version, SNAP identity/version, RET expected version/live
   state and pin/time predicates, stage quota expected version and limits.
   Update project gate/counters, RET and stage quota atomically. Combine conditions
   on a written item with its update; no duplicate actions for one item. Proposed
   standalone transition budget is at most seven actions, within accepted
   eight-action pin budget and global 80-action/2 MiB/128 KiB item ceilings.
4. A real pin increments RET version, sets `pinnedAt`/decisionTime, sets pinned
   and removes RET's due-index keys. A real unpin increments RET version, clears
   pinnedAt, restores the original expiry due-index key and decrements pin
   counters. It never changes `normalExpiresAt = completedAt + 30 days`.
   Repeats use a condition-checked coherent state read, not an unguarded early
   success assembled from mixed versions. Every read/mutation resolves ownership,
   deletion and expiry at its server decision point.
5. If expiry has passed at unpin, the same commit makes the body inaccessible
   and immediately due for cleanup. No new grace, normal-expiry extension or
   requirement to wait for the scheduled cleaner before denying reads. Return
   200 with `effectiveStatus:expired`; subsequent normal detail/evidence is 404.
   Leaving RET live until the cleaner claims deleting does not grant access.
6. Retention cleanup claims live -> deleting with exact RET version,
   `pinned=false`, original expiry <= fresh decisionTime and active project
   version/epoch/generation. Strengthen the shared helper rather than relying
   solely on a GSI candidate. Pin first invalidates the claim; cleanup first
   irrevocably prevents a new pin. Recheck strong state/time after losing CAS.
   Every retention drain chunk/ticket transition is fenced by that deleting,
   unpinned RET and active project. Once a ticket is durably deleting, pin cannot
   reopen RET; S3 removal remains resumable. Preserve RET until all obligations
   settle. Delayed GSI entries affect latency only; pinned due entries are skipped.
7. Project DELETE uses the existing 202 operation, immediately advances gate/epoch
   and denies all normal access. It overrides every pin and removes pinned RET,
   SNAP, anchor and S3 objects as well as other known children. A retention drain
   that observes project deletion stops; `DrainProject` takes over deliberately.
   Drain pinned accounting exactly once with a guarded RET transition/deletion
   before the final tombstone discards project counters. No physical completion
   until ticket accounting and strong whole-partition/prefix proofs pass;
   uncertain/late PUTs and durable residual sweeping retain existing semantics.

Use at most three new conditional attempts per call. Each attempt re-reads state,
refreshes decisionTime and re-evaluates expiry/quota; never reuse an old
eligibility decision. Distinguish known conditional cancellation from an unknown
acknowledgement. Use a DDB transaction client token for the exact prepared
transition and reuse only that identical transaction during acknowledgement
resolution. Strong readback may confirm the intended RET version/state and
atomic accounting; a later opposite transition can obscure the outcome, so
return 503 rather than blindly applying the intention again. Browser refreshes
current state and asks for a deliberate retry. No persistent pin receipt system.

DDB has no transaction NOW(): the accepted five-second margin is evaluated at
server decisionTime, not a claim of wall-clock precision at remote commit.
Slow commit/clock-skew tests must demonstrate this documented boundary. Do not
relax the margin or silently change time while resolving an uncertain commit.

Keep strong authorization/RET checks before and after evidence fetching, and
read history/detail retention coherently with the gate rather than mixing
versions. A response already authorized before an unpin/deletion cannot be
recalled from a browser; later reads must deny access. Native TTL cannot delete
authoritative live RET/SNAP/anchors/tickets, and age-only S3 lifecycle cannot
delete pinned objects. Missing/corrupt evidence updates availability only; a pin
does not override restore reconciliation or the accepted backup exception.

## Accepted quotas, accounting and development cost

| Limit                            | Accepted light development default                                         |
| -------------------------------- | -------------------------------------------------------------------------- |
| Pinned capture count             | 100/project, 200/stage                                                     |
| Pinned encoded bytes             | 256 MiB/project, 512 MiB/stage                                             |
| Total retained/reserved evidence | Existing 1 GiB/project, 2 GiB/stage, including pins and upload obligations |

All limits apply together. A capture is charged once regardless of age; bodies
with omission metadata still count. `pinnedCount` and `pinnedBytes` live on project
META and existing stage QUOTA. Byte charge is immutable SNAP encoded length,
not response wire size, API envelope size, frontend estimate or a second body
copy. Check both limits conditionally on false-to-true; repeated PUT is free of
new accounting even at a limit. True-to-false releases pin classification once,
including unavailable evidence, but retains total storage charge until existing
ticket/S3 deletion settlement. No execution-admission refund. Never evict a pin
to free quota or respond to budget alerts; allow unpin/deletion even when over
quota. Lowering limits blocks new pins, preserving existing ones.

Pin count/bytes transitions contend on project/stage versions across all
projects, preventing concurrent overfill. Project deletion decrements pin
classification exactly once even if retries resume midway. Storage charges
remain ticket-owned until object absence settles. Unknown records, underflow,
missing counters or inconsistent totals fail closed and keep cleanup pending.
Reconcile counters from authoritative retained records during recovery before
reopening; never infer a healthy zero from absent data on an active pin stage.

Accepted compatibility activation: advance `savedRequestsSchemaVersion` from 3
to 4 and add `evidencePinningSchemaVersion=1` only
after API, publication, ownership record readers/writers and both cleanup paths
understand the new counters/RET fields. Preserve counters during ordinary
project edits. Old binaries must fail closed before this stage opens. Existing
schema-1 RETs can read with null pinnedAt/default version semantics; no SNAP/S3
migration. Initialize missing pin counters only under a closed/gated stage and
verified no-pin inventory (or reconcile actual pins), with conditional writes;
preserve recovery generation. The scoped rollout was subsequently approved and completed; see the
[deployment validation record](evidence-pinning-validation.md).

Cost forecast: reuse existing development resources/schedule, no new S3 copy,
KMS key, queue, index, hosting or standing compute. Reference
[existing regional calculations](w1-review-decisions.md#monthly-development-cost-estimate)
and official [S3 pricing](https://aws.amazon.com/s3/pricing/),
[DDB pricing](https://aws.amazon.com/dynamodb/pricing/),
[Lambda pricing](https://aws.amazon.com/lambda/pricing/) and
[API Gateway pricing](https://aws.amazon.com/api-gateway/pricing/).
S3 GET and DDB request rates were checked against official pages on 2026-10-12;
The implementation [deployment review](../development/evidence-pinning.md)
refreshes all regional rates on 2026-10-12 and preserves the $0.70 incremental
allowance. Recheck rates for any later deployment preview.

Assume 1,000 pin/unpin calls/month, 512 MiB accumulated beyond normal retention,
1,000 extra evidence views, 256 MiB conservatively allowed extra metadata/index/
backup storage, and bounded transaction retries. Reserve about 30,000 WRU and
100,000 RRU for mutation/strong hydration, including transaction multipliers,
item rounding and related UI refreshes; measure before widening usage.

| Increment                                                                                   | Monthly forecast before credits/free tiers |
| ------------------------------------------------------------------------------------------- | ------------------------------------------ |
| Extra S3 storage, conservatively 0.537 billed GB × $0.023                                   | $0.013                                     |
| <=2,000 verification/view GETs × $0.0004/1k                                                 | $0.001                                     |
| DDB 30k WRU × $0.625/m + 100k RRU × $0.125/m                                                | $0.032                                     |
| Conservative extra 0.268 GB × ($0.25 storage + $0.20 PITR + $0.10 backup)                   | $0.148                                     |
| 2k API calls at $1/m; 2k 256-MiB/0.5-second ARM invocations including request charges       | $0.006                                     |
| <=4 GiB evidence delivery at assumed $0.09/billed GB plus small safe logs/cleanup allowance | $0.50                                      |
| **Rounded planning allowance**                                                              | **$0.70/month**                            |

This deliberately conservative allowance added to the existing $3.19 light
model yields about **$3.89/month**, below the $5 development target under these
assumptions. It is a slice estimate, not observed billing or a spending cap; do
not add earlier slice forecasts again to that all-service baseline. Rotation,
shared bootstrap, unrelated stages, more reads/egress, rejected calls and abuse
can exceed the target. At the maximum pin size, 1,000 extra views could deliver
4 GiB; storage quotas alone do not bound delivery costs. Keep accepted budget
alerts/weekly review and existing rate limits; no automatic destructive budget
action. Verify generated resource/IAM/cost differences only after acceptance
and separate deployment authorization.

## Accepted React behavior

- Add Pin/Unpin to history rows and selected execution detail, synchronized from
  returned RET metadata and refreshed history. Display normal expiry and
  "Protected from automatic expiry while pinned."; availability remains visible separately.
  No filter/index or bulk pin feature is needed for this slice.
- Disable Pin for preterminal/unpublished/unavailable evidence and the server
  cutoff; show the reason. Server remains authoritative for time and quotas.
  Continue to allow Unpin on unavailable pinned evidence. When unpinning past
  expiry, require explicit confirmation that access ends immediately and
  cleanup may remove the evidence; no additional grace period.
- One mutation in flight per execution, disable both controls while pending,
  show Pinning/Unpinning and accessible status/error messages. Abort on project
  switch/unmount/session expiry; late responses cannot update another row.
  Preserve the selection without falsely declaring an optimistic pin success.
- On 409 refresh current retention/history and display window/quota/conflict
  feedback; never silently retry the user's opposite intention. On lost ACK/503
  show unresolved state and offer refresh, then deliberate retry. A 200 late
  unpin immediately clears selected evidence from memory and removes the expired
  row after refresh. A subsequent 404 clears evidence/selection as unavailable.
- Existing failed/unknown-outcome warnings and escaped-text response rendering
  remain. Pinning failed but available evidence is valid. Missing/corrupt evidence
  shows unavailable, no fabricated response or action that restores it. Refresh
  project detail after a real gate change before project rename/delete CAS.
  No evidence or tokens added to browser persistence.

## Required future validation

All tests in this section are **planned**, not results of packet preparation.
Use deterministic clocks/barriers and fault injection for precise race coverage,
then real DDB/S3/scheduled-runtime checks under separately authorized deployment.

| Area                     | Required evidence                                                                                                                                                                                                                                                                                    |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Ownership and security   | Owner PUT/DELETE succeeds; second user, cross-project locator and deleted project get 404; disabled user, closed stage and recovery generation fence mutations. No secrets/body/checksums in DTO/errors/logs.                                                                                        |
| Publication/availability | Preterminal or unpublished cannot pin; missing/corrupt object/manifest or unavailable RET cannot create a pin or charge quota; failed published evidence may pin. Bounded verification and no secret persistence.                                                                                    |
| Repeats/immutability     | Repeated PUT/DELETE preserves RET version/timestamps/counts; repeated pinned PUT beyond expiry; repeats at quota cap; deleted anchor 404. Byte-for-byte S3, SNAP length/SHA-256 and frozen capture unchanged after pin/unpin.                                                                        |
| Concurrent intentions    | Two PUTs charge once; two DELETEs refund once; opposing PUT/DELETE has one serial valid outcome per commit. Project/stage quota contention across projects; lost ACK readback, obscured outcome and exact transaction-token retry.                                                                   |
| Timing                   | Strictly outside cutoff succeeds; exact five seconds/inside cutoff fails; before/at/after expiry; fresh time on each conflict; slow commit/clock skew follows decisionTime boundary. Last unpin after expiry denies reads immediately with original expiry.                                          |
| Cleanup races            | Pin wins then cleaner skips; cleanup wins then pin cannot resurrect. Stale GSI candidate, stale RET, interrupted ticket/body/anchor drain, GET before/after unpin and RET fence through every drain phase. Eventual physical removal with no grace/TTL bypass.                                       |
| Quotas/accounting        | Exact count/byte limit, one over, oversized capture pin, simultaneous last slots, unavailable unpin, limit reduction, ticket-owned storage release, no eviction/underflow/double release, counter reconciliation.                                                                                    |
| Project deletion         | Deletion overrides pins before/after competing commit; immediate read/PUT/DELETE denial; RET classification refunded once; both stores, pinned objects and counters drain over multiple chunks. Unknown records/uncertain or late PUTs remain pending; no completion until proofs and residual work. |
| Compatibility and budget | Old binary fails closed; preserved counters on unrelated project edit; existing RET defaults; conditional migration does not reset generation; transaction action/size bounds; actual resource diff and metered workload distinguish estimates from spend.                                           |
| React                    | History/detail sync, disabled reasons, loading/accessibility, quota/window/conflict, lost ACK refresh, unavailable pinned unpin, late-unpin confirmation, memory clearing, project switch/unmount and safe existing failed/unknown evidence rendering.                                               |

After accepted implementation: run repository `npm run verify`, relevant Go race
and Local integration tests, and browser fixture coverage. Report those separately
from actual AWS ownership/CAS/S3/scheduled cleanup and genuine Cognito/browser
checks. Restore/PITR reconciliation and external S3 loss remain distinct
operational validation, never inferred from fixture passes.

## Accepted review decisions

1. Accept empty-body PUT/DELETE with internal CAS, no client If-Match/key and
   last valid committed intention; truthful idempotent repeats while the anchor
   exists, including unavailable already-pinned metadata.
2. Accept RET DTO additions and proposed 409 window/quota errors, existing 503
   availability/unresolved-write behavior, and bounded verification for new pins.
3. Accept 100/200 project/stage pin counts and 256/512 MiB pinned-byte limits
   within the existing 1/2 GiB total storage limits, guarded classification
   accounting and the $0.70/month incremental planning allowance.
4. Accept the shared RET/project cleanup fences, capability-gated compatibility
   plan and UI late-unpin confirmation. Keep workflow/run context out of this
   standalone slice; its accepted future protection requirements are unchanged.

Davian accepted these choices with the UI wording amendment above. Dependent
implementation and API contract synchronization are authorized. The scoped
development deployment and guarded activation were separately approved at
`e6b964f` on 2026-10-12; broader AWS changes still require their own authorization.
See [actual results and remaining checks](evidence-pinning-validation.md).
