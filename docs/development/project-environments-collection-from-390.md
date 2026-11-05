# Initial collection from 390: authentication boundary held

The authorized start command ran **once**, using clean tooling
**9eafa58928b5a2486f30bd7531f18c324d4fe534** and unchanged clean release source
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**. It exited **1** at the credential
boundary, before any physical request was admitted. The campaign is latched
stopped. No retry, repair, renewal, live collection, preview or later phase followed.
[Machine accounting and stop record](project-environments-collection-from-390.json).

The documented local check passed before execution: all stopped-parent
records/authorizations/claims, adopted ZIPs and rollback/proof bytes, model/tool
pins and source cleanliness verified. `pendingScopedReads` was empty. A fresh
mode-0600 `authorization-from-390.json` was generated from the pinned template,
reviewed and changed only from `authorized: false` to `true`, carrying prior 390,
ceiling 1,799, original 3,300 seconds and unchanged $1 procedural allowance.
Its hash matches the campaign and exclusive consumed `window-from-390.json`.

Executed once, **historical command only — do not rerun**:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-390 --authorization .build/project-environments-execution/authorization-from-390.json
```

## Outcome and limits

The durable ledger contains **zero attempts**, zero outcomes and zero unmatched
attempts. Cumulative usage remains **390/1,799**, with **1,409 arithmetic attempt
headroom** and the unchanged cumulative **$1 procedural allowance**. Billing is
unmeasured. No renewal or mandatory STS request was sent; identity/credential
verification did not complete. The gate admits durably before every physical send.
No infrastructure/state write or runtime request occurred.

The retained events are campaign creation, collection phase start,
`phase-authentication-held-no-retry` and
`phase-failed-or-uncertain-no-restart`. Terminal elapsed time is **0.014134 seconds**
from the original monotonic campaign start, leaving **3299.985866 seconds**
of the original 55-minute window and **239.997215 seconds** of the original
240-second phase bound at that stop. The closed-stage window never began.
Neither clock nor campaign was restarted. Remaining time is not continuation authority.

The authentication catch preserves only the sanitized stop reason; it does not
retain the underlying SDK/cache/local exception. **The exact root cause is not
established.** No attribution to credential expiry, cache/identity drift, network
failure or remote rejection is supported by this record. The stopped credential
boundary was not replayed to obtain a different exception.

## Requested collection findings

| Finding                                                  | This campaign's evidence                          |
| -------------------------------------------------------- | ------------------------------------------------- |
| Active baseline and capabilities                         | Unavailable; reader child never started           |
| Quiescence, residual prefixes and queue depth            | Unavailable                                       |
| Accepted old-frontend readiness                          | Not checked                                       |
| Deployed configuration, code and IAM comparisons         | Unavailable                                       |
| Complete resource diff and review-8 projection agreement | Unavailable; preview never started                |
| Fresh generated artifact agreement                       | Unestablished; zero fresh ZIPs/metadata generated |
| Collection proof                                         | Absent                                            |

The prior [303 reader snapshot](project-environments-collection-from-303-readers.json)
and successful review-8 preview remain historical evidence. They do not supply
this campaign's missing fresh observations. Adopted and rollback bytes still
verify unchanged; this preservation check is separate from missing fresh generated
artifact agreement. No deployment readiness or stage-closure claim is made.

## Preservation, verification and next decision

The new private authorization, exclusive consumed claim, stopped ledger and all
three earlier stopped campaigns/claims remain under ignored `.build`. Existing
private evidence, adopted ZIPs, rollback bytes and worktree registrations are
preserved. No cache/session setting change, artifact rebuild, cleanup or worktree
pruning occurred. The PR branch was restored for publication. Only sanitized
counters, elapsed time, hashes of the bound authorization/manifest/scope and guard
conclusions are published; no credentials, identity values, raw requests or private
captures are included.

Publication checks independently read the terminal ledger without mutation,
reconciled its zero-attempt accounting, verified authorization/claim/pin agreement,
checked absence of reader/preview children and collection proof, and reran the
local artifact/evidence/source check. Formatting and disclosure checks passed.
No application or tooling implementation changed; the prior 80 execution tests,
12 isolated cases and repository build/lint/test verification remain applicable
and were not repeated for this evidence-only publication.

The next decision is a **local review of the credential boundary** using retained
metadata and pinned source, if desired. No further live command is proposed or
authorized. Do not replay this stopped campaign, reuse or delete its consumed
claim, replace the approved session, reset a deadline or infer execution authority
from remaining call/time headroom. Any further live attempt requires separately
reviewed authorization. Closure, deployment, verification, activation, final and
runtime/rollback work remain held.
