# One-shot recovery executor: local review only

The accepted recovery transaction now has a bounded executor. **No AWS call,
credential capture, renewal, stage write or live recovery occurred in this task.**
The last observed stage remains `workflow-migration`/16. Cumulative usage remains
**955/1,799**, leaving **844** attempts; the cumulative **$1 procedural allowance**
is unchanged and billing remains unmeasured. Current readiness must be checked
again by the eventual executor; retained inspection evidence is not a new live
readiness assertion.

Exact tooling: **8c8419b25b30ef15937f6da2a9b8276ea3916fa4**.
Unchanged release source: **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
The [successful inspection and accepted proposal](project-environments-recovery-inspect-868-r1.md)
remain immutable. The consumed inspection window, its claim/checkpoint, and the
original expired campaign/closed-stage deadlines are preserved. This executor
cannot resume those windows or enter deployment/activation phases.

## Fixed request and authorization boundary

The executor binds the original private proposal and pure transaction files in
`.build/project-environments-execution/recovery-inspect-868-r1/`. It does not
regenerate the request, token, conditions or expected values from fresh reads.
The original proposal's `executionAuthorized:false` remains unchanged; a separate
future authorization controls execution.

| Binding           | SHA256                                                             |
| ----------------- | ------------------------------------------------------------------ |
| Proposal wrapper  | `307a987a15b4e372b61bda5df4fbe12bba6b270f4a4c207470461d4286e01251` |
| Exact transaction | `4bb06443f6c2d3eb578a2c15f7ced1ea1b2b7bd8ea003864d47e97610dec3f1f` |
| Before marker     | `353cd6632e9351399bf4e62d853f500ec02e4fa2b5ba1fc7148b37965818a708` |
| After marker      | `c89cdb1316d80dc2d497ff13a13080e3a55ad0fa85cc4f6b861781aad5489e95` |

The only permitted stage mutation is one `TransactWriteItems`: the exact marker
Update from `workflow-migration`/16 to `active`/17 plus seven accounting
ConditionChecks. Only marker state/version change. Capabilities, recovery
generation, absent environment capability and all accounting remain unchanged.
No generic mutation allowance or additional read permission was added.

Proposed separate authorization: **recovery-reopen-955-r1**, **600 seconds**,
**at most 250 additional physical attempts**, starting at 955 and ending at a hard
local maximum of **1,205**, within the unchanged global 1,799. The bound covers all
renewal, STS, child reads, the single transaction and readback. Unused inspection
capacity is not reused. This proposed recovery execution window requires separate
explicit approval; the current instruction authorizes implementation/tests only.

The authorization binds tooling/source, manifest/scope, both historical ledgers,
inspection proof, fixed proposal/transaction/marker hashes and the exact approved
identity/account. A false review template exists privately at
`.build/project-environments-reopen-review/authorization-review-8c8419b.json`.
No actual recovery authorization, campaign directory or claim was created.

## Credential, readiness and one-shot controls

The writer strictly carries the consumed inspection's final credential checkpoint.
It does **not** capture the current real cache or assert it still matches. Cache
drift stops before authentication; a changed checkpoint needs separate explicit
review. At most one metered existing-session renewal may advance this checkpoint
at the recovery boundary. Credentials must cover the actual remaining 600-second
window plus five seconds; short credentials or failed renewal stop. Fresh host
STS and fresh STS in each isolated child must verify the exact inventoried non-root
identity/account. There is no background or mid-phase renewal. Existing exclusive
locking, continuity, signing, framing and physical-attempt accounting remain in
force.

Before arming, a read-only isolated child collects current marker, seven accounting
rows, incompatible-data inventory, all four old package hashes/configuration/IAM,
old frontend agreement, initial routing, queue quiescence and scoped SST state/lock
observations. The host independently runs the actual offline readiness assessor
on this freshly collected bundle. This first child has no stage transaction in its
policy context.

The write child repeats the full live inspection, then immediately rereads the
marker, full inventory/accounting, queue depth and old frontend. Observed drift or
missing evidence stops without substituting newly observed expectations. The host
proxy independently requires the bound proof, unmodified snapshot and protected
historical evidence, clean exact tooling/source, an unexpired **240-second arm
lease**, at least **60 seconds** remaining and at least **33 attempt slots** before (32 reserved after admission)
admitting the transaction. Nonempty protected-ledger WAL files also hold execution.
The wire gate requires the fixed transaction digest and exact closed marker after
fresh identity verification. No other stage write can pass this profile.

Admission durably records the transaction attempt before forwarding. Any previous
transaction row, including one without a recorded outcome, blocks a second attempt
across proxy instances. The consumed exclusive claim prevents a second start.
SDK retries are disabled; there is no uncertain-write replay, repair, automatic
reopen or continuation. A denied/failed/uncertain transaction consumes the window.
After HTTP200, the child rereads the marker and inventory/accounting, requiring the
exact `active`/17 marker and unchanged accounting hash. The host reconciles the one
successful durable transaction and proxy marker evidence before writing a receipt.
A failed or mismatched readback stops with an uncertain outcome, never another
write.

The two full inspections and reserves are conservative bounds, not measured AWS
completion guarantees. Cross-service observations are not an atomic cloud audit;
the transaction atomically guards the exact reviewed marker fields and seven
counters. Scoped SST observations cannot prove absence of all historical changes
or deletions. Offline fixtures prove control behavior, not live readiness or
service completion time. Timeout can leave a successful write with unverified
readback; manual evidence review is then required.

## Offline verification and preservation

**208 execution/frontend/recovery tests pass**, including 12 new Python tests
through actual recovery `main`/start/authentication with synthetic locked cache,
pinned SDK sends and actual offline readiness assessment, and 18 Node executor
tests using retained evidence and synthetic collector/transaction responses.
Actual SDK serialization exercises the exact transaction and rejects changed
conditions, token, action count and table. Tests cover identity rejection, failed/
short renewal, pre-/mid-campaign cache drift, stale/mutated evidence, missing
source/evidence, insufficient reserves, one-shot admission, pending outcomes,
readback mismatch and consumed-claim replay. No synthetic failure retries a write.

All **17** shared isolated transport/context/preview/packaging scenarios pass.
Repository format/lint/tests/build/Go verification passed with networking disabled;
Ruff checks and formatting pass for 37 Python files. Verification log SHA256:
`da7d1228c2df8132508730bb3fca425ad4bac252744a016ce33fa09631cd9ada`.
A local clean-pin preflight and false-template generation passed without AWS calls.

Preservation comparison checked **11,987 retained files**, including historical
campaigns, claims/checkpoints/proofs, adopted ZIP/frontend bytes and rollback bytes:
zero changes. Worktree registrations are unchanged. No `.build` removal, artifact
regeneration/replacement or new recovery claim occurred.

## Exact proposed workflow — held pending separate live approval

Use the exact tooling pin in a clean checkout; do not change or reset historical
windows. These are proposed commands, not commands executed by this task:

```sh
git switch --detach 8c8419b25b30ef15937f6da2a9b8276ea3916fa4
python3 -B scripts/environment-deploy/reopen.py check
(umask 077; set -C; python3 -B scripts/environment-deploy/reopen.py template > /home/king/Neumont/Kurier/.build/project-environments-execution/reopen-authorization-from-955-r1.json)
```

Review the private false authorization against the accepted transaction, exact
source/tooling, strict carried checkpoint and proposed 600-second/250-attempt
bound. Only after separate explicit live approval, change **only** `authorized`
from `false` to `true`, preserving the reviewed bindings and private 0600 mode.
The start command is exactly:

```sh
python3 -B scripts/environment-deploy/reopen.py start --authorization /home/king/Neumont/Kurier/.build/project-environments-execution/reopen-authorization-from-955-r1.json
```

Execute once only. Cache drift, stale evidence, insufficient limits or any guard
failure means stop and preserve the claim/evidence; no recapture, restart or
substitution. Neither this packet nor accepting the transaction's scope authorizes
that live command. Deployment, environment activation, runtime requests, rollback
and repairs remain held.
