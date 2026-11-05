# Authorized collection from 301 — stopped before reader requests

The authorized start command ran once and stopped during reader-child startup.
Host renewal, STS identity verification and the bounded credential-validity guard
all passed. The child then failed to open its relative context path with
`ENOENT`, before any collector request or guarded preview. The campaign is
durably stopped; no repair, restart, second renewal or deadline reset occurred.

Actual usage is **303/1,799**: 301 carried forward plus exactly two physical
attempts. There are **1,496** remaining attempts, which do not authorize further
work. The cumulative **$1 procedural allowance** is unchanged; billing remains
unmeasured. No infrastructure/state writes or runtime requests occurred. The
only authentication mutation was the explicitly authorized session renewal.

## Exact once-only execution and accounting

The clean review branch was preserved while selecting tooling commit
`3e380db5f1ef7dba6587abb003d953bc79436871` in the original workspace. Release
source remained the clean detached worktree at
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. The documented local check passed
with zero AWS calls, including artifact/source, SDK/model and stopped-parent
proofs. No adopted ZIP or dependency was rebuilt.

The private authorization template bound the exact clean tooling commit,
adopted manifest, scope, 301 baseline, 1,799 ceiling, original 3,300-second window,
240-second phase ceiling, five-second credential margin and bounded renewal
policy. Its values were checked against the accepted packet and only
`authorized` was changed to true. The file was created exclusively with mode
0600; the new campaign and claim were absent before execution.

Historical command — **do not rerun**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-301 --authorization .build/project-environments-execution/authorization-from-301.json
```

| Physical attempt | Operation                | Outcome  |
| ---------------- | ------------------------ | -------- |
| 302              | signin/CreateOAuth2Token | HTTP 200 |
| 303              | sts/GetCallerIdentity    | HTTP 200 |

Both attempts were durably admitted before forwarding and have recorded
outcomes. There are zero unmatched attempts, one host renewal, one mandatory
host STS verification and **zero collector/provider forwards**. The
`phase-authenticated` event confirms the inventoried account/non-root identity
and phase-validity check passed. No background or mid-phase renewal occurred.

At the recorded stop, campaign elapsed time was **1.377449 seconds**, original
campaign time remaining was **3,298.622551 seconds**, and collection-phase time
remaining was **238.652698 seconds**. Neither deadline expired or restarted.
No closed-stage window began. Remaining time is historical accounting, not
authority to resume the stopped campaign.

## Findings, limits and retained evidence

The private reader output reports `ENOENT` at `readFileSync` in `phase.mjs`, with
the missing path exactly matching the relative `collect-readers-context.json`
argument. That file exists in the original workspace. Local source inspection
shows the driver constructs it relative to the campaign argument, while the
sandbox changes directory to `/tmp/kurier-environments-release-source` and
mounts that worktree's `.build` separately. This supports a context-path
resolution failure before collection. No correction or reproduction was run.

The reader proxy started, recorded no forwarded requests and no child identity
observation, and confirmed zero uploads. Its error log is empty. Host identity
verification succeeded separately; it must not be confused with the absent
child observation. No `collect-diff` directory or collection proof exists.

| Requested fresh finding              | Result       |
| ------------------------------------ | ------------ |
| Baseline and quiescence              | Not observed |
| Configuration and IAM comparisons    | Not observed |
| Complete resource diff               | Not observed |
| Generated ZIP and frontend agreement | Not observed |
| Live propagation or runtime behavior | Not observed |

Historical preview/artifact/local-fixture results remain historical. They do
not substitute for the fresh findings above. No close, deploy, verify, activate
or final phase ran; no stage transition, infrastructure/state write, repair,
runtime request, identity/project creation or rollback occurred.

The authorization, campaign ledger, consumed `window-from-301.json` claim,
private child output, proxy summary and child work directory remain retained
under ignored `.build/project-environments-execution`. The stopped
`release-from-299`, original authorization, consumed `window-from-299` claim,
adopted ZIPs, rollback bytes and prior private evidence remain intact. No
cleanup, worktree pruning, `.build` removal or evidence replacement occurred.
The review branch was restored for publication.

[The sanitized machine record](project-environments-collection-from-301.json)
publishes only bound hashes, projected attempts, timing, boolean conclusions and
the reviewed failure category. Raw diagnostics and their hashes, credentials,
configuration responses and cache contents are not published. Formatting and
record/ledger/claim consistency checks validate this publication; unchanged
application and tooling builds/tests were not repeated.

The next decision is whether to authorize a local context-path correction and
offline regression before considering another live campaign. Carry forward
**303** attempts. Both consumed claims must remain intact. No new campaign,
repair, refresh or later phase is authorized by this result.
