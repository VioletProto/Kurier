# Bounded deployment timing and credential reuse amendment

Tooling pin: **4de31ab9a9c791a1fc224bbce8a7fe79220519ba**.
Release source remains **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
This implements the local amendment approved from [the timing review](project-environments-closure-deployment-go-no-go.md)
at **9f6933148d8fc743aebb36c4bdaf3e1b2579571c**.
[Machine verification](project-environments-timing-amendment.json).

**Zero AWS calls, real-cache captures or renewals ran.** No collection, closure,
deployment or activation ran. Accounting remains **608/1,799**, at most **1,191**
additional physical attempts, first possible ordinal **609**, unchanged cumulative
**$1 procedural allowance**; billing is unmeasured. No real new authorization,
campaign, claim or checkpoint was created. The clean pinned false template was
reviewed in memory only.

## Implemented behavior

| Phase    | Maximum seconds |
| -------- | --------------: |
| Collect  |             240 |
| Close    |             240 |
| Deploy   |             840 |
| Verify   |             240 |
| Activate |             240 |
| Final    |             240 |

Every phase is clipped to the **original campaign deadline** and the existing
closed-stage deadline if present. Authentication, planning/setup, child work and
polling share the phase timer. The campaign remains **3,300 seconds** starting
before initial capture/authentication. Close starts its one **1,800-second**
closed-stage deadline before authentication, bounded by the original campaign.
Renewal and later phases never reset either clock. Existing deadline supervision,
stop-before-forward accounting, uncertain-write holds and descendant termination
remain in force. Expired campaigns cannot resume.

Immediately before invoking the pinned fetcher, required validity is the actual
remaining clipped phase time plus the **five-second** margin. Only that fetcher
instance receives `_REFRESH_THRESHOLD = max(class default 300, required validity)`.
The SDK class, installed source, real wall clock and cached expiry are unchanged.
A token with 600 seconds left now triggers the one startup renewal for an unclipped
840-second deploy, while an adequate token is reused. A phase clipped to 200 seconds
does not demand 845 seconds of validity. The post-STS check again uses actual
remaining time plus five seconds and rejects short credentials before child work.

At most **one metered existing-session renewal per phase / six per campaign** is
retained. Mandatory fresh same-identity/account STS precedes collector/provider
work. Cache loading/saving/send continuity guards, DPoP/session binding, save-only-
after-recorded-success, no background/mid-phase renewal and frozen child credentials
are unchanged. External identity/cache drift, failed/uncertain renewal, short
credentials, budget exhaustion or any expired limit stops without replay/reset.
No new operation, resource, signing/framing allowance or SDK/configuration change
was introduced.

The 840-second maximum is a **bounded engineering allowance, not measured AWS
completion time**. Pinned CloudFront and Lambda waits each reach 600 seconds;
sequential waits and other work can exceed 840. No fixture proves live completion
or spend. Timeout after accepted writes can leave the stage closed and partially
deployed. No uncertain write is retried, no automatic reopen/repair/rollback runs,
and later inspection/recovery needs separate approval. Close/deploy/verify/activate
ceilings total 1,560 seconds, leaving at most 240 seconds for reviews/overhead in
the fixed closed window; this arithmetic is not a capacity guarantee.

## Completed parent and new campaign binding

The new verifier reads the successful [f1 collection](project-environments-collection-499-f1.md)
at **dd2d54059206398ff96307a74b2d3003e38d0200** without opening a writable campaign.
It pins the sanitized record and projections; opens SQLite read-only/immutable;
rejects a nonempty WAL, aliased ledger or changed accounting; verifies all ordinals
**500–608**, durable outcomes, 499+109=608, the original deadline, exact tooling,
manifest/scope/source, nextPhase 1 and no active/closed phase. It reconciles private
authorization/claim/initial-checkpoint/acceptance bindings, continuity metadata,
collection proof and original reader output. This is historical verification,
not revalidation of current cloud state and not permission to resume that campaign.

The original f1 deadline was checked locally at **2026-11-02T01:20:09.661831+00:00**. It was **expired**, with -67.197705 seconds of signed remainder on the same boot. Its ledger was not changed, resumed or reset. The proposed 608 campaign requires separate approval of a new window; it does not extend or migrate f1.

All six stopped campaigns are still checked separately. Their checkpoints and the
completed f1 record/ledger/claim are preserved. The new false template explicitly
binds `completedParentSha256`, tooling, artifact/scope pins, phase-limit map,
required-validity reuse policy, cumulative budget and new **`release-608-t1`** identity.
The new consumed claim would be **`window-from-608-timing-1.json`**; old claims are
never reused or migrated.

Fresh capture at startup still requires **new explicit approval** for the exact
inventoried identity/account, acknowledging unproved historical grant continuity.
The reviewed fresh mode captures under the exclusive lock immediately before
authentication and binds the resulting private record to the new authorization.
Once active, only that campaign's metered renewal may advance continuity. No drift
is recaptured automatically. The carried-checkpoint mode has no fresh fallback.

## Offline verification and preservation

**151 execution/frontend tests pass**, including **nine timing regressions through
the actual driver phase entrypoint** with the pinned SDK, synthetic cache and
network sends replaced by fixtures, plus **four completed-parent check-entrypoint
regressions** with synthetic records. Variants cover all phase ceilings, campaign/
closed clipping, adequate/insufficient cached validity, short renewal responses,
failed renewal, identity/cache drift, no global SDK threshold change, unchanged
clocks/accounting, polling after 300 virtual seconds, hold at 840, and no second
attempt after an uncertain mutation. Completed-parent tests cover record/pending
WAL, active/closed/count/deadline/tooling drift, claim/checkpoint/proof/attempt
changes and byte-preserving read-only verification even after the old deadline.

All **17 isolated scenarios** pass, including actual-driver cache-drift descendant
termination, shared transport/accounting/deadline/uncertain-write controls, amended
production preview/read denials, documented relative child-context isolation and
four parent-umask packaging cases. Every packaging case reproduces all four adopted
whole-ZIP hashes, members/attributes and executable bootstraps exactly. Fixture
attempts are synthetic and do not change 608/1,799.

Repository `npm run verify` passed with external networking and dependency fetching
disabled: format/lint/tests/build/Go tests and vet. Ruff lint/format for all 30
Python files and local source/model/adopted/rollback verification pass. Retained
checkpoint/claim/authorization/ledger hashes match the preservation snapshot.
No stale worktree registration was pruned, `.build` deleted, old evidence rebuilt
or adopted/rollback bytes altered. No credential values, token hashes or raw cache
metadata are published.

## Exact workflow — separate live authorization required

The next decision is **one initial collection using this pin**, including explicit
fresh-at-start capture for the same scoped identity/account and historical
continuity limitation. The local implementation approval grants no such live
permission. Do not execute the commands below under this task.

After that separate approval, select the exact clean tooling pin:

```sh
cd /home/king/Neumont/Kurier
git switch --detach 4de31ab9a9c791a1fc224bbce8a7fe79220519ba
python3 -B scripts/environment-deploy/driver.py check
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py template --fresh-checkpoint > .build/project-environments-execution/authorization-from-608-timing-1.json
```

Review exact identity/account and fresh policy, completed parent, source/artifacts,
phase limits (deploy 840, others 240), reuse policy, **608 prior / 1,799 ceiling**,
**1,191 maximum additional attempts**, **3,300 seconds** and cumulative **$1**.
The fixed 1,800-second closed maximum remains in the pinned controller.
The template is `authorized: false`; change only that flag after separate approval.
No existing authorization may be overwritten. Execute exactly once:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-608-t1 --authorization .build/project-environments-execution/authorization-from-608-timing-1.json --fresh-checkpoint
```

Stop after collection or any failure. Publish fresh baseline/quiescence, old
frontend, configuration/IAM comparisons, complete diff, exact four-ZIP/frontend
agreement, elapsed time and cumulative attempts. The old collection is historical;
it cannot replace this new campaign's fresh proof. Any guard failure, unknown
operation, artifact mismatch or expired limit stops without recapture or restart.

Only after that successful fresh collection and **separate close approval**, with
enough unchanged original time/calls and accepted bounded timeout risk, prepare:

```sh
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py prepare --campaign .build/project-environments-execution/release-608-t1 --phase close > .build/project-environments-execution/release-608-t1/close-approval.json
```

Review its complete bound template including `engineApproval`; change only
`authorized` after separate approval. Proposed single-use close command:

```sh
python3 -B scripts/environment-deploy/driver.py phase --campaign .build/project-environments-execution/release-608-t1 --phase close --approval .build/project-environments-execution/release-608-t1/close-approval.json
```

Stop and review the durable receipt and original deadlines. Only with **separate
deployment approval**, prepare/review its false template:

```sh
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py prepare --campaign .build/project-environments-execution/release-608-t1 --phase deploy > .build/project-environments-execution/release-608-t1/deploy-approval.json
```

Change only `authorized` after approval, then once:

```sh
python3 -B scripts/environment-deploy/driver.py phase --campaign .build/project-environments-execution/release-608-t1 --phase deploy --approval .build/project-environments-execution/release-608-t1/deploy-approval.json
```

Stop after deployment or failure. Verify, activate, final and any inspection or
recovery remain separate approvals. Never reset the campaign/closed deadline,
replay an uncertain write, widen policy, switch identity or automatically reopen.
