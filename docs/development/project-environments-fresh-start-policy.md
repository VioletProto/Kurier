# Explicit fresh checkpoint at new campaign startup

Tooling: **bf1effb2835ae6be00bdbd753a802dc0b9bf6741**. Release source remains
**44f1fb79d943501c95e06fcc1a6114111a7c1ea4**.
[Machine verification](project-environments-fresh-start-policy.json).
This task implements and tests the policy locally. **No real-cache capture,
renewal, STS, AWS call or live campaign ran.** Usage remains **499/1,799**, at most
**1,300** additional attempts, unchanged cumulative **$1** procedural allowance;
billing is unmeasured.

## Two distinct rules

**New campaign initialization:** an execution authorization may explicitly permit
fresh capture of the current existing cache for the exact inventoried non-root
identity/account. Its `checkpointPolicy` must contain `mode: fresh-at-start`,
`allowFreshCapture: true`, the exact scoped identity/account,
`historicalGrantContinuityEstablished: false`, and fixed record name
`initial-checkpoint.json`. Earlier campaign metadata may differ. This permission
does not prove continuity with the historical grant or allow another identity.
The global execution authorization must also be true; a flag/template alone
cannot authorize capture.

**Active campaign:** once the initial checkpoint is accepted, strict continuity
applies. The boundary matches full file metadata before any request. Only that
campaign's metered SDK renewal, followed by mandatory fresh same-identity/account
STS and sufficient credential validity, may advance its checkpoint. No external
cache change is adopted, no background/mid-phase renewal occurs, and no later
phase can invoke fresh capture. The carried-checkpoint mode remains available
and never falls back automatically to fresh capture on drift.

## Startup order and bindings

1. Validate clean tooling/source/model/artifact/accounting pins and the exact
   mode-0600, owned execution authorization. Read it once with metadata/race checks;
   retain its original byte hash, avoiding later authorization-file substitution.
2. Exclusively consume `window-from-499-fresh-1.json` and create the single fixed
   campaign **`release-499-f1`**, starting at **499**. The first physical attempt
   would be **500**. Existing campaign names/claims, repeats, budget resets and
   mixing fresh and bound checkpoint arguments are rejected.
3. Start the original 55-minute campaign and 240-second collection-phase clocks
   before capture. Capture time counts against both; no later action resets them.
4. Under an exclusive nonblocking lock, open the existing cache with O_NOFOLLOW;
   validate regular-file status, ownership, mode 0600, configured identity/account,
   required structure, timezone-aware expiry and P-256 key structure. No SDK fetcher
   or network is invoked during capture. Recheck descriptor/named-file identity,
   full metadata, configuration identity/metadata and in-memory bytes through capture.
5. Write the private mode-0600 `initial-checkpoint.json` and acceptance marker in
   the mode-0700 campaign directory. Any capture change/replacement or interruption
   rejects the operation; partial evidence is retained and cannot be overwritten.
   The capture latch forbids replay even if no physical request was admitted.
6. Release the capture lock, then immediately enter the existing authentication
   boundary, which locks/rechecks that exact snapshot before credential fetching.
   A change in this gap stops before sending; it is not recaptured. At most one
   reviewed existing-session renewal, if required, and mandatory fresh STS must
   succeed before any collector/provider child starts.

The authorization approves the capture **policy**, since the future metadata is
unknown at review time. The immutable claim binds that authorization/policy and
exact campaign directory; its pre-capture checkpoint hash is null. The resulting
checkpoint embeds the authorization hash, tooling/source scope and capture policy,
and the durable ledger binds the resulting checkpoint hash. The claim is not
rewritten after capture. No cache token contents, credential values, private keys
or token hashes are copied into these records or published. Snapshot hashes are
hashes of metadata records, not token hashes.

## Continuity during work and failure behavior

Authentication checks the locked checkpoint before and after SDK cache loads,
before each physical send, after renewal/STS responses, and before accepting the
authenticated checkpoint. A cache save requires exactly one successful metered
renewal in the active phase and cannot repeat. External drift during either
renewal or STS stops without adopting it or launching work.

A host-only metadata check runs immediately before child launch, on the existing
child-supervision loop (about every 50 ms), and after child exit. It checks the
configured identity and the last authenticated full cache metadata. It does not
load tokens, refresh credentials or issue network requests. Detection latches a
stop and terminates descendants. In-flight attempts retain their original durable
accounting; the monitor does not retry them or extend deadlines.

Children still receive only their required context file and frozen credentials;
`.aws`, checkpoint/authorization files, claims and ledgers are not mounted into
their namespace. Restrictive child umask **0077**, whole-archive byte checks,
source/artifact pins, account/signing/framing controls and provider read scope
remain unchanged. Separate close/deploy/verify/activate/final approvals remain
required; fresh capture is valid only before initial authentication and requests.

## Offline verification

**138 execution/frontend tests pass**, including **18 actual-driver fresh-start
regressions**. They cover changed pre-campaign cache under explicit approval,
unauthorized/mismatched scope, wrong local identity/account/ownership/permissions,
cache mutation/replacement races, capture-to-auth drift, rejected STS identity,
bounded metered renewal, SDK-load drift, in-flight renewal/STS drift, unmetered
cache-save rejection, active-campaign drift, deadline failure, old-mode
non-fallback and claim/capture replay rejection. Private canaries do not appear
in events or reports. Ordinary templates and checks do not capture the cache.

All **17 isolated scenarios** pass: the existing 16 transport, packaging, preview
and context scenarios plus an actual driver → fresh capture → mocked STS → real
namespace launcher case. That case starts a local child, changes only a synthetic
cache during work, observes `active-cache-checkpoint-changed`, and confirms the
child's pulse stops without another capture or request. Host loopback is used only
for the existing local proxy; external networking is disabled and synthetic cache
paths are used throughout. No real cache is read by that test.

Repository `npm run verify` (format/lint/tests/build/Go tests/vet), Ruff lint/format,
exact adopted ZIP reproduction and retained source/artifact/evidence checks pass.
Dependencies were not downloaded. The new pinned parent record additionally
verifies the stopped zero-call `release-499-p1` ledger and claim. All six old
campaigns, consumed claims, original checkpoints and adopted/rollback bytes remain
intact. No new real capture record, campaign, claim or execution authorization was
created during implementation/testing.

## Exact revised workflow for separate authorization

The next decision is explicit approval of **one initial collection including
fresh-at-start capture** using this clean tooling pin, unchanged source and exact
scoped identity/account, acknowledging unproved historical grant continuity.
This packet proposes the workflow; it does not grant that live permission.

After that approval, select the exact tooling commit clean in the original workspace:

```sh
cd /home/king/Neumont/Kurier
git switch --detach bf1effb2835ae6be00bdbd753a802dc0b9bf6741
python3 -B scripts/environment-deploy/driver.py check
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py template --fresh-checkpoint > .build/project-environments-execution/authorization-from-499-fresh-1.json
```

The template is deliberately `authorized: false`, with the explicit fresh-capture
policy and `checkpointSha256: null`. Review the exact identity/account, source,
policy, prior 499, ceiling 1,799, 3,300 seconds, 240-second phase and $1 allowance;
change only `authorized` to true after the separate approval. No existing checkpoint
is supplied, and no real capture occurs while preparing the template.

Execute once:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-499-f1 --authorization .build/project-environments-execution/authorization-from-499-fresh-1.json --fresh-checkpoint
```

Stop after collection or any guard failure and preserve the initial snapshot,
acceptance/rejection evidence, original authorization, consumed claim and ledger.
No recapture, automatic alternate identity, restart, hash substitution, deadline
reset, infrastructure/state writes, repairs, runtime requests, later phase or
rollback is implied. The later closed-stage window remains bounded by its original
campaign deadline and separate phase approvals.
