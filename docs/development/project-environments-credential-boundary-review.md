# Local credential boundary: diagnostics, correction and checkpoint hold

Tooling: **fb6be7701289464ef1a7946986ceb26085d4c133**. Release source remains
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
[Machine findings](project-environments-credential-boundary-review.json).
The consumed [zero-call campaign](project-environments-collection-from-390.md)
was not replayed. Accounting remains **390/1,799**, **1,409** arithmetic attempt
headroom, unchanged cumulative **$1** procedural allowance, billing unmeasured.
No AWS calls, renewal, live collection or later phases occurred.

## Retained metadata and pinned source

The preceding successful credential checkpoint in `release-from-303` and the
checkpoint copied into stopped `release-from-390` agree exactly. Comparison with
the current existing cache gives:

| Local comparison                                                       | Result                                       |
| ---------------------------------------------------------------------- | -------------------------------------------- |
| Configured identity equals inventoried identity                        | Yes                                          |
| Cache exists, regular, owned by current user; not a symlink            | Yes                                          |
| Cache mode / parent directory mode                                     | 0600 / 0755                                  |
| Cache readable                                                         | Yes                                          |
| Read/write open and nonblocking exclusive lock in live process context | Succeeded; descriptor closed without writing |
| Read-only descriptor exclusive lock                                    | Succeeded and released                       |
| Inode equals preceding checkpoint                                      | Yes                                          |
| Size equals preceding checkpoint                                       | **No**                                       |
| Modification time equals preceding checkpoint                          | **No**                                       |
| Cached account equals approved account                                 | Yes                                          |
| Required existing renewal fields present                               | Yes                                          |
| Access credential expired at inspection                                | Yes                                          |
| SDK and HTTP-library source pins                                       | Pass                                         |

The ordinary filesystem sandbox reports write access unavailable; the separately
approved access/lock-only diagnostic in the live process context succeeds. This
is not evidence that cache permissions caused the earlier stop. Inspection did
not write cache data, change permissions, renew credentials, call an authentication
provider or change a checkpoint. No credential values, token hashes, raw stat
values, hashed session filenames or grant material are published.

The original catch retained no underlying error. The current size/mtime mismatch
establishes a **present checkpoint hold**, not the historical zero-call failure's
exact cause or when/who changed the file. Same configured identity/account and
inode do not establish uninterrupted grant/session continuity. Expired access
credentials alone do not establish an expired refresh grant.

Pinned `LoginCredentialFetcher.refresh_credentials()` reloads the token and its
DPoP private key, reuses credentials only above the 300-second threshold, and
otherwise invokes the existing-session renewal before saving the same cache.
No requested lifetime field exists; issued lifetime remains at most 900 seconds.
The local boundary checks configured identity and the original inode/size/mtime
checkpoint **before** constructing that SDK fetcher. That guard remains intact.
ExistingCache still opens the original cache with O_RDWR/O_NOFOLLOW, takes a
nonblocking exclusive lock, validates account/bindings and closes it. No SDK,
identity, authentication-setting or provider-read scope pin changed.

## Demonstrated correction

The old boundary collapsed all local failures to
`phase-authentication-held-no-retry`. It now records one closed-vocabulary
`authenticationFailure` containing only **code** and **stage**, plus a matching
safe event. `driver.py report` projects those two fields only. Examples include
`cache-missing`, `cache-access-denied`, `cache-locked`, `cache-json-invalid`,
`inventoried-session-required`, `session-changed-no-unmetered-refresh`,
`authentication-identity-held` and `credential-phase-window-too-short`.
SDK pin failure uses the fixed `authentication-sdk-pins-held` guard/CLI code.
Unknown exceptions produce a fixed fallback; exception messages, traceback,
credential/request data and SDK response fields are never recorded by this path.
Deadline interruption during SDK processing also records a safe code and latches
the original campaign stopped.

Offline tests exposed a separate implementation defect in the budget adapter:
the shared authentication transport expects `policy.Stop`, while the campaign
raises `common.Held`. In particular, a failed synthetic renewal's recorded
outcome stopped with `write-uncertain-or-http-transport-failure`; the mismatched
exception type made the transport stop again with a generic transport/policy
reason. The adapter now translates campaign guards to the expected exception
with an allowlisted code, preserving the first stop. Tests verify the original
reason, one metered synthetic attempt, recorded outcome and no collector work.
This defect is not attributed as the historical zero-call cause.

No failure is converted to success. All same-session/account/identity checks,
checkpoint rejection, admission-before-send, retry/renewal caps, credential
validity requirements, original campaign/closed-stage/phase deadlines and stopped
claim behavior remain. No changed cache is silently adopted.

## Offline reproduction and verification

Nineteen new regressions enter the **actual `driver.main` → `start` →
`credentials` → real ExistingCache → pinned SDK** path. Only the external
approval/checkpoint inputs are synthetic; the credential function and cache
implementation are not replaced. All physical sends use synthetic responses;
sockets/DNS are forbidden, and the full suite runs in a network-isolated namespace.
The collector boundary is a deliberate synthetic stop, preventing collection.

The current mismatch is reproduced by changing synthetic cache size/mtime: it
produces `session-changed-no-unmetered-refresh` at `checkpoint-check`, **zero
attempts**, unchanged stored checkpoint and a retained consumed synthetic claim.
Other cases cover inode replacement, missing/permission-denied/locked/symlinked
or malformed cache, missing configuration, wrong configured session/account,
matching checkpoint reaching mandatory STS, bounded synthetic renewal, renewal
failure, STS identity mismatch, SDK-local failure, deadline interruption and
redaction of private canaries. Successful fixture renewals update only the
synthetic original file after metering; original deadlines are not reset.

**99 execution/frontend tests passed**: 17 policy/accounting, 22 credential,
19 driver-startup, nine path, 18 policy/SDK/diagnostic and 14 frontend checks.
All **12 isolated transport/preview/context cases**, repository
`npm run verify` (format, lint, tests, build and Go tests/vet), Ruff lint/format and
artifact/evidence/source checks pass. External downloads were disabled. Synthetic
attempts do not change live accounting. Historical investigations were not repeated;
new source/metadata inspection was limited to this credential-boundary failure.

## Checkpoint proposal and next command

**Proposed decision, not implemented:** explicitly review whether the currently
observed cache can be accepted as a new checkpoint for the same approved
identity/account. Its size/mtime differ, and the original approved grant's
continuity cannot be established from the retained metadata. Approval would need
a separately bound checkpoint-review record, preservation of both original
checkpoints and all consumed claims, and explicit treatment of any session
continuity limitation. Do not infer adoption from matching identity/account.

No current checkpoint or session is adopted by this correction. In addition,
`window-from-390.json` is already consumed: the historical start command must not
be replayed or its claim removed. A future campaign/claim design requires review
before any new live authorization. No live start command is proposed now.

Exact proposed next **local-only** command, using this clean tooling pin:

```sh
python3 -B scripts/environment-deploy/driver.py check
```

That command checks pinned artifacts/evidence/tooling; it does not authorize or
adopt the changed live cache. Existing stopped ledgers, private evidence, adopted
ZIPs, rollback bytes and worktree registrations remain intact. No infrastructure
writes, repair, policy widening, deadline reset, closure, deployment, verification,
activation, final phase or runtime work is authorized.
