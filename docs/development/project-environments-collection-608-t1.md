# release-608-t1: restored source and completed initial collection

The source worktree was restored as authorized, full local preflight passed, and
the exact published start command ran **once** using clean tooling
**4de31ab9a9c791a1fc224bbce8a7fe79220519ba** and unchanged clean release source
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**. It exited **0** after collection.
**No later phase ran.** [Machine outcome](project-environments-collection-608-t1.json),
[reader and IAM findings](project-environments-collection-608-t1-readers.json),
[complete sanitized resource diff](project-environments-collection-608-t1-diff.json),
[all projected attempts](project-environments-collection-608-t1-attempts.jsonl).

## Scoped restoration and preflight

The existing registration named the missing `/tmp/kurier-environments-release-source`
worktree, detached at the exact approved source commit. Git's installed worktree
manual documents `add --force` for a registered path whose directory is missing.
The original target registration metadata was copied into a new private restoration
record before running this supported command:

```sh
git worktree add --force --detach /tmp/kurier-environments-release-source 44f1fb79d943501c95e06fcc1a6114111a7c1ea4
```

Only that worktree was restored. Its exact detached HEAD and clean status were
verified. Existing root `node_modules`, `.sst/platform` and provider-lock caches
were linked at the required source locations; empty ignored child mountpoints
were created without building assets. SST npm/platform **4.17.1**, the pinned
**7.40.0** provider binary and reviewed SDK/model/packager/tool hashes verified.
No dependency downloads, artifact regeneration or adopted-hash changes occurred
during restoration. The later authorized preview generated its own four ZIPs.

Unrelated registrations, including the three stale assertions registrations, were
not changed or pruned. Existing worktree/claim/checkpoint/ledger hashes were checked
against the private pre-restoration snapshot. Full `driver.py check` then passed,
including the seven historical campaign bindings, adopted/rollback bytes and
restored source. The [preceding preflight hold](project-environments-collection-608-t1-preflight.md)
remains unchanged; it had created no campaign/claim or start attempt. This run did
not restart a stopped campaign.

## Bound authorization and authentication

The private mode-0600 authorization was created exclusively from the approved
pin's `template --fresh-checkpoint`. Review checked the exact inventoried identity/
account, completed-parent binding, source/artifact/scope pins, phase-limit map,
renewal policy, 608/1,799, 3,300 seconds, 240-second collection and $1 allowance.
Only `authorized` changed to true. The new immutable claim, authorization, private
initial-checkpoint acceptance and campaign bindings were reconciled after execution.

Explicitly approved fresh-at-start capture passed under the exclusive cache lock.
Historical grant continuity remains **unproved**. Existing credentials covered the
actual collection window, so **zero renewals** were required. Mandatory fresh host
STS at ordinal **609** returned **200** before any collector/provider work; identity/
account and validity checks passed. Nine STS attempts total include that boundary
and child/provider identity reads. No recapture, alternate session, background or
mid-phase renewal occurred.

## Fresh findings and complete diff

The bounded stage snapshot is **active/version 15**, with the historical recovery
generation and schema-8 saved requests; environment capability remains absent.
The marker was unchanged across collection. All inventory guards passed:
**107 projected rows, zero active projects, seven accounting rows, eleven residual
rows with empty prefixes**, and zero visible/in-flight/delayed queue depth. Quota
remains version **75**, month **2026-10**, count **32**. Full sanitized kind counts
are in the reader record; row keys, identities and payloads are unpublished.

All four deployed functions match the accepted code/configuration/IAM baseline,
including role policies/trust, runtime, architecture, memory and timeout. They are
Active/Successful with no reserved concurrency. The existing **45 routes and 45
integrations**, invocation policies, active free plan, account/unreserved
concurrency **10**, queue batch **1** / max concurrency **2**, enabled minute cleanup
schedule and throttle guards passed.

**Old-frontend readiness passed against the accepted old baseline**, including file
bytes, KVS and deployed distribution. Generated release frontend metadata separately
matches the adopted manifest. These are bounded snapshot observations, not claims
about cloud state after the snapshot.

The complete successful SST change list contains **60 entries / 44 unique
resources**, with the same reviewed projection as review 8: **32 creates, seven
updates, four replacements and one existing-bucket read**. Every logical resource,
type, operation and changed field appears in the linked complete diff. Repeated
route-component creates and separate replacement steps explain duplicated entries;
unchanged resources are omitted. This is a complete change list, not a new full
AWS inventory.

Physical creates remain eight routes, eight proxy integrations and eight Lambda
invoke permissions. Gateway authorization is NONE; application authentication and
runtime behavior were not exercised. Invocation principal/action and existing
API-wide source scope match review. Target inputs remain encrypted in retained
preview data; no direct decrypted target validation is claimed. No IAM role/
managed-policy, identity, table, queue, KMS, Cognito, distribution or stage metadata
change appears. **No proposed infrastructure change was applied.**

All **four whole-ZIP hashes exactly match adoption**, independently comparing every
byte, member, external attribute and executable bootstrap. API/cleanup ZIP SHA256:
`58f2bbf364042ac209d4f9b3a4d6ca583da18914fb5851f0c166d34b9c757373`;
worker/controlled endpoint:
`82af7beeda4b619afa6831b342c693bb1fb3e8d08e5e6c0416ce253526f59098`.
The generated frontend artifact equals the adopted manifest. The collection proof
was produced and its ledger hash verified. No ZIP was patched, hash substituted,
new artifact adopted or upload performed.

## Accounting, clocks and stop

**608 + 108 = 716/1,799**, leaving **1,083 arithmetic attempt headroom**.
All 108 physical attempts have recorded outcomes: **100 HTTP 200 and eight HTTP
404**, zero failed transports or unmatched attempts. All were read-only;
**zero infrastructure/state writes, runtime requests or renewals**. The cumulative
**$1 procedural allowance** is unchanged; billing remains unmeasured.

Collection completed **68.575541 seconds** after the original campaign start,
with **3231.424459 seconds remaining at completion** in the new 55-minute window.
The phase-start event to completion interval was **68.558752 seconds**, within the
240-second ceiling shared by capture, authentication, readers and preview. The
collection timer cleared on successful completion. The original campaign deadline
was never reset and the closed-stage window never started. Completion-time capacity
is not a statement of current remaining time or permission to continue.

Execution stopped after successful collection. NextPhase is close, activePhase is
null and the failure latch is unset; no subsequent command was run to close,
deploy, verify, activate or finalize. All **eight campaigns/consumed claims**, old
checkpoints, new capture/authorization/proof/ledger and private evidence are
preserved, alongside adopted/rollback bytes and the original registration backup.
The PR branch was restored for publication.

Publication assertions independently reconciled ordinals 609–716, outcomes, private
bindings, original clocks, reader comparisons, complete diff and exact ZIP/frontend
agreement. Source cleanliness, versions, preservation, format and disclosure checks
passed. No tooling/application changes were needed; prior format/lint/test/build
proofs remain applicable. No completed investigation or fixture suite was replayed.

Historical once-only start command — **do not rerun**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-608-t1 --authorization .build/project-environments-execution/authorization-from-608-timing-1.json --fresh-checkpoint
```

The next decision is review of this collection. Closure and every later phase
remain held pending **separate explicit approval**, strict cache continuity,
cumulative accounting and the **same original campaign deadline**. No recapture,
restart, clock reset, policy widening, repair, uncertain-write replay or automatic
recovery is implied.
