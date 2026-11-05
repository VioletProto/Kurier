# Initial fresh collection — stopped at credential lifetime guard

The authorized one-shot start stopped before reader collection or guarded preview.
The refreshed existing-session credentials had approximately 15 minutes of
lifetime and could not cover the original 55-minute shared deadline plus guard.
The campaign is durably stopped; no retry, repair, second refresh or deadline
reset occurred. Historical command below must not be rerun.

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-299 --authorization .build/project-environments-execution/authorization.json
```

The user explicitly approved the cumulative ceiling of 1,799, carrying forward
299 prior attempts, at most 1,500 additional attempts, one original 55-minute
window and the unchanged cumulative $1 procedural allowance. Authorization was
limited to initial collection, at most one existing-session refresh, mandatory
STS verification and guarded read-only preview. Billing remains unmeasured.

The original workspace was clean when selecting exact tooling commit
`c5d794b4e3c3afacab7c478fd4fdeab84ff4a886`. The documented local `driver.py check`
passed with zero AWS calls. The generated authorization template was checked
against the approved commit and budgets; only `authorized` was changed to true.
It binds the adopted manifest and execution scope hashes. Release source remains
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4` in its clean detached worktree.

| Physical attempt | Operation                | Outcome  |
| ---------------- | ------------------------ | -------- |
| 300              | signin/CreateOAuth2Token | HTTP 200 |
| 301              | sts/GetCallerIdentity    | HTTP 200 |

Usage is **301/1,799**, with two new physical attempts and zero unmatched outcomes.
The authentication checkpoint was recorded after the mandatory account/non-root
identity check. Local cache metadata matched that checkpoint and the approved
identity/account. The session could not cover the original deadline, so the
credential-window guard stopped before launching any collector child.

The ledger records **0.549080 seconds elapsed** and **3,299.450920 seconds remaining
at the stop**, approximately 54 minutes 59 seconds. The 1,498 remaining attempts
and recorded time do not authorize continuation of this stopped campaign. No
closed-stage window began.

Fresh baseline and quiescence findings, configuration/IAM comparisons, the
complete resource diff, generated ZIP agreement and frontend agreement are all
**unobserved**. No claim of unchanged cloud state or current artifact agreement
is made. The previously completed review-8 and local fixture results remain
historical evidence; they were not rerun or substituted for fresh observations.

No close, deploy, verify, activate or final phase ran. No infrastructure/state
writes, repairs, runtime requests, identities/projects or rollback occurred. The
only authentication mutation was the explicitly permitted session refresh.

The private authorization, original campaign SQLite ledger and consumed
`window-from-299.json` claim remain under ignored
`.build/project-environments-execution`. All adopted ZIPs, rollback bytes,
historical ledgers/claims and private evidence remain untouched. The review branch
was restored for this publication. No raw credentials, configuration responses,
cache contents or private diagnostics are published.

[The sanitized machine record](project-environments-collection-stop.json)
contains the authorization hash, pinned source/tooling/manifest/scope, attempt
projection and stop accounting. Publication validation checks formatting and
record consistency only; application builds/tests already recorded in the
[execution packet](project-environments-deployment-execution.md) were not repeated
because no application or tooling code changed.

The next decision is a separate review of credential lifetime and an execution
window that the same session can cover, before authorizing any new campaign.
Carry forward **301** attempts. Preserve the consumed claim; do not shorten/reset
this campaign's deadline, reuse its start command or automatically create a new
campaign. All later phases remain unapproved.
