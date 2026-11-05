# Fresh private checkpoint and single-use campaign identity

Tooling: **42915e66301e272d28f5d9a3aac253239ca51e9e**. Release source remains
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
[Sanitized machine findings](project-environments-checkpoint-adoption.json).

The authorized **local capture completed once**, under an exclusive nonblocking
cache lock. No renewal, STS, collection, preview or other AWS call occurred.
Accounting remains **390/1,799**, at most **1,409** additional physical attempts,
unchanged cumulative **$1** procedural allowance; billing is unmeasured.

## Historical limitation and fresh approval

Retained inspection outputs contain comparison booleans, not the exact tuple from
the inspection published at `35e0e87`. The search found only the prior verification
log/result and the published comparison record for that review. **Unchanged cache
metadata since that historical inspection cannot be established.** Historical
grant/session continuity and the original zero-call failure's cause remain unknown.

The user's subsequent instruction explicitly superseded the unavailable-tuple
match with authorization to capture the current cache freshly. The private approval
records that mode, the approved identity/account, exact tooling/source, manifest,
scope, zero-call parent record, prior usage/ceiling, exact checkpoint output path
and one fixed future campaign identity. It does not assert historical continuity.
No old checkpoint was edited or replaced.

Private, mode-0600 records in a mode-0700 directory:

- `.build/project-environments-execution/checkpoints/capture-approval-current-1.json`
- `.build/project-environments-execution/checkpoints/adopted-current-1.json`
- The checkpoint's separate `.accepted` integrity marker.

Only approved identity/account and file metadata are retained there; cache token
contents, private keys and token hashes are not copied. Public records omit
credential values, token hashes, raw metadata tuples and private-record digests.

## Locked capture and future continuity

`capture-checkpoint` validates the explicit approval and exact output path, then
opens the original cache read-only with O_NOFOLLOW and takes LOCK_EX/LOCK_NB.
Under that lock it rechecks configuration identity, account, regular-file status,
owner, mode 0600, required token structure, timezone-aware expiry and P-256 key
structure. Parsing does not invoke a credential provider or SDK renewal.

It compares the opened descriptor with the named file and rechecks device, inode,
size, mtime, ctime, owner/mode, configuration metadata and in-memory cache bytes
through capture. Replacement or change rejects the operation. Partial records
are preserved with a rejection marker and cannot load; missing acceptance is also
held. Existing records cannot be overwritten, and an approval for one output
cannot be reused for a different output. Capture never writes the cache.

The accepted checkpoint binds the exact approval hash privately. Future false
execution templates and explicit authorizations bind its exact hash, tooling,
manifest/scope, campaign identity and cumulative budgets. On the first credential
boundary, the locked cache must match both the original inode/size/mtime check
and the adopted full metadata. Drift stops **before any physical attempt**.

Within the campaign, each separately approved boundary must match the preceding
checkpoint. Only metered SDK renewal followed by mandatory fresh same-identity/
account STS and sufficient validity may advance it. Cached credential fields must
also match those used for that STS check, using in-memory comparisons only. No
background/mid-phase renewal, external cache adoption or validity bypass is added.
Original campaign, phase and closed-stage deadlines remain enforced.

## Fresh identity and cumulative accounting

The only new campaign identity is **`release-390-cp1`**, with exclusive claim
**`window-from-390-checkpoint-1.json`**. Both are distinct from the consumed
`release-from-390` campaign and `window-from-390.json`. Alternative campaign names,
old authorizations, a changed checkpoint hash, budget resets and repeat starts
are rejected. The new claim binds authorization, checkpoint and exact directory.
No automatic alternate identity or restart is selected after a stop.

The driver now also verifies the pinned zero-call 390 parent, its original
checkpoint, authorization and consumed claim, alongside all earlier parents.
The next campaign starts at **390**, not zero; the first physical attempt would
be 391. Its cumulative maximum remains **1,799**. No campaign, new claim or live
execution authorization was created during this task.

The context fixture caught that a longer candidate name exceeded Linux's Unix
socket pathname bound. The selected short name yields a **106-byte** longest
relay path in the documented workspace and passes the real child entrypoint.
No relay, network-isolation or mount policy was widened. The child still receives
only its required context file; checkpoints, approval files, claims, ledgers and
campaign directories remain outside its mounts.

## Verification

**118 execution/frontend tests pass**, including 19 actual driver-startup and
19 capture/claim regressions. Cases cover capture replacement/change races,
configuration drift, competing locks, wrong identity/account, malformed structure,
permissions/ownership, output and approval tampering, absent acceptance, old-claim
preservation, fresh-claim reuse, checkpoint drift and cumulative-budget reset
rejection. Socket/DNS calls are forbidden in authentication fixtures; the full
suite ran with external networking disabled.

All **12 isolated cases** pass, including the updated documented relative campaign
argument, actual child entrypoint/working directory, read-only context and hidden
checkpoint/claim files. Repository `npm run verify` (format/lint/tests/build/Go
tests/vet), Ruff lint/format and retained artifact/evidence/source checks pass.
Downloads were disabled. The long-name fixture failure remains retained and was
resolved by selecting the short identity before capture or any live attempt.

## Proposed initial collection: separate authorization required

Use the original workspace, unchanged `.build`/dependencies and clean tooling pin
above. Do not rerun capture or edit its records to accept later cache drift.
The checkpoint is adopted locally; **live initial collection is not authorized**.

Local commands for future bound-authorization review:

```sh
python3 -B scripts/environment-deploy/driver.py check
python3 -B scripts/environment-deploy/driver.py template --checkpoint .build/project-environments-execution/checkpoints/adopted-current-1.json
```

The template is `authorized: false`. No actual
`authorization-from-390-checkpoint-1.json` was created. After separate initial
collection authorization, prepare it exclusively at mode 0600 from that template,
review all fields and change only `authorized` to true. The exact proposed start
command is:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-390-cp1 --authorization .build/project-environments-execution/authorization-from-390-checkpoint-1.json --checkpoint .build/project-environments-execution/checkpoints/adopted-current-1.json
```

Proposed bounds remain one original **55-minute** campaign window starting before
authentication, a shared **240-second** collection phase, at most one reviewed
existing-session renewal if needed, mandatory fresh STS, and stop after collection.
A later closed-stage window remains at most 30 minutes inside the campaign's
original deadline, with separate approvals. This new identity does not resume or
reset any stopped campaign's deadline. Call headroom is not continuation authority.

Preserve all four stopped campaigns, consumed claims, original checkpoints,
private evidence and adopted/rollback bytes. No infrastructure/state writes,
repairs, policy widening, later phases, runtime requests or rollback are authorized.
