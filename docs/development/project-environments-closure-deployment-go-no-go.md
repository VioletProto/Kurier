# Stage closure and deployment: no-go pending timing amendment

The successful [collection at dd2d540](https://github.com/VioletProto/Kurier/blob/dd2d54059206398ff96307a74b2d3003e38d0200/docs/development/project-environments-collection-499-f1.md)
establishes artifact and baseline readiness for review. **Do not close or deploy
with the present four-minute deployment lease.** Approve and verify the narrow
timing/credential amendment below first, then review a new campaign authorization.
[Machine review](project-environments-closure-deployment-go-no-go.json).

This review made **zero AWS calls**. No real-cache inspection/capture, renewal,
collection replay, stage write, deployment, activation or runtime request ran.
Usage remains **608/1,799**, with at most **1,191** further physical attempts and
the unchanged cumulative **$1 procedural allowance**; billing is unmeasured.
Reviewed tooling is **bf1effb2835ae6be00bdbd753a802dc0b9bf6741** and release source
remains **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**. This task changes documents,
not execution policy or tooling.

## What is established

The one live collection completed in **90.598714 seconds**, using 109 metered
attempts. Its active/version-15 baseline, unchanged marker, empty queue, zero
active projects, empty residual prefixes, old frontend and deployed
configuration/IAM/routing checks passed. Its complete change list has
**60 entries / 44 resources**, matching review 8. All four generated whole-ZIP
hashes and frontend metadata match adoption exactly. No infrastructure/state
write or later phase ran. All seven campaigns, consumed claims, checkpoints,
private evidence and adopted/rollback bytes remain intact.

This measures **collection plus preview**, not deployment, closure, Lambda update
propagation or CloudFront invalidation. Offline production-policy and transport
fixtures establish gating, accounting, bytes and descendant termination; they
establish **no AWS completion-time distribution or deployment capacity**.
Pre-deployment checks require the accepted old frontend. Post-deployment readiness
requires the adopted release. No new data migration/backfill, runtime requests or
automatic rollback is implied.

## Actual waits and competing limits

Inspection used local pinned source, SDK source and installed binary symbols,
call sites and duration constants. Source hashes appear in the machine review;
private annotated disassemblies remain under `.build/project-environments-go-no-go-review`.
Neither SST nor the provider was invoked to deploy.

| Layer                                   | Inspected behavior                                                                                                                                                                      | Consequence                                                                                                                        |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `common.py`                             | Every phase gets `min(now + 240, campaign deadline, closed deadline if set)`; authentication, planning/setup and child work share it                                                    | Four minutes is the entire phase, not just propagation                                                                             |
| Release `infra/frontend-hosting.ts:112` | `invalidation: { paths: "all", wait: true }`                                                                                                                                            | Deploy must wait for invalidation completion; skipping it changes release behavior                                                 |
| SST static-site source                  | Invalidation depends on uploaded files and updated KVS; local docs say waiting can exceed five minutes                                                                                  | The wait is downstream of prerequisite deployment work                                                                             |
| Pinned SST binary                       | `DistributionInvalidation.Update` calls `handle`; `handle` creates invalidation and calls `InvalidationCompletedWaiter.WaitForOutput` with `0x8bb2c97000` nanoseconds = **600 seconds** | The existing host deadline can kill an accepted invalidation before the built-in waiter completes                                  |
| Pinned Lambda provider                  | Resource schema supplies **600-second** create/update/delete defaults; code update calls `UpdateFunctionCode`, then `waitFunctionUpdated` using the update timeout                      | An accepted update can still be propagating when the host's 240 seconds expire                                                     |
| `phase.mjs`                             | Warm hashed frontend assets/KVS, then synchronous SST deploy; no separate deploy subprocess timeout                                                                                     | Host/proxy lease is the effective outer limit; compilation, uploads, resource graph, waits and state completion consume it         |
| `isolation.py` / proxy                  | Host supervision about every 50 ms plus proxy deadline checks; child termination then up to five seconds before kill, with outcome reconciliation/cleanup                               | A deadline stops forwarding; it does not undo an accepted write or promise instantaneous process exit                              |
| Auth boundary                           | Sign-In/STS connect/read caps 10/30 seconds, one physical attempt each; outer alarm shares phase deadline                                                                               | Authentication consumes phase time; a transport failure is terminal                                                                |
| Pinned credential SDK                   | Reuses cached credentials above **300 seconds**; renewal model allows at most **900 seconds**, with no duration input                                                                   | Raising only the phase timer can fail the validity guard even though the cache is still reusable under the SDK's default threshold |
| Children                                | Frozen environment credentials, no host cache and no background/mid-phase renewal                                                                                                       | A running deploy cannot acquire a fresh lease silently                                                                             |

The Lambda update default was decoded from the installed provider's schema
`DefaultTimeout` argument (`00 70 c9 b2 8b 00 00 00`, little endian), and its update
call path passes that timeout to the propagation wait. These are **configured
bounds**, not observations that Lambda takes ten minutes. Provider waits may run
concurrently across resources, but dependency order, throttling, repeated reads
and state checkpointing prevent simply treating four functions as a single fixed
latency. No live deployment timing was measured.

A deployment may finish within 240 seconds, but that is **not a defensible
capacity assumption before deliberately closing the stage**. CloudFront alone
can consume longer. A timeout after any accepted write leaves a closed, possibly
partially deployed stage. Terminating the local child cannot cancel already
accepted AWS work. No uncertain write may be replayed, repaired or compensated
automatically.

## Smallest proposed amendment — not implemented or approved

Keep collection, close, verify, activate and final at **240 seconds**. Give only
deploy a reviewed **840-second** maximum, always clipped to the original campaign
and closed-stage deadlines. This provides a 600-second CloudFront wait plus a
240-second engineering allowance for other work. That allowance is an **estimate**,
not measured AWS performance. A Lambda wait followed by the CloudFront wait can
exceed 840 seconds; the proposal deliberately remains bounded and can fail closed.
Acceptance of that partial-deployment risk is a separate decision.

At the existing phase boundary, derive required validity from the **actual remaining
approved phase deadline plus the existing five-second guard**. For that one pinned
SDK fetcher instance, use a reuse threshold of `max(300, required validity)` so a
token with 301–844 seconds left does not prevent the one permitted startup renewal
for an 840-second phase. Keep real wall-clock expiration calculation, the existing
refresh grant/DPoP binding, strict locked cache continuity, metered save guard and
fresh same-identity/account STS. Do not edit cached expiry, spoof SDK time, patch
the installed SDK globally or change session configuration. Recheck actual
validity after STS; a short response stops before child work. A 900-second issued
token has only **55 seconds** beyond an initial 840+5 requirement, before boundary
overhead; no longer-lived credential is established by this review.

Retain at most **one renewal per phase / six per campaign**, no background or
mid-phase renewal, fixed identity/account, every physical attempt in the same
ledger, 1,799 cumulative ceiling and $1 allowance. Polling reads each consume an
attempt. No finite provider completion/call bound was demonstrated; 1,191 calls
are headroom, not proof of sufficient capacity or spend.

Keep the original **3,300-second campaign** and set the closed deadline once,
**before close authentication**, to `min(original campaign deadline, close entry + 1800)`.
Never reset either during renewal, deployment or review. Proposed close/deploy/
verify/activate ceilings total **1,560 seconds**, leaving at most **240 seconds**
for reviews and intervening overhead within the 30-minute closed window. All six
phase ceilings total **2,040 seconds**. These arithmetic sums are not completion
proofs. Final runs only after separately approved activation and remains bounded
by the original campaign deadline. Activation failure or any uncertainty leaves
the stage closed for separately approved inspection/recovery, with no automatic
reopen or rollback.

Required offline verification before a new tooling pin: actual driver approval and
phase selection; credentials just above 300 but below the required long lease;
adequate cached credentials; one successful renewal and STS; short/denied/uncertain
renewal; expired lease, external cache/identity drift and consumed phase replay;
read polling through 240 seconds within an 840-second deploy; process termination
at the clipped phase/closed/campaign deadline; no second forward of an uncertain
mutation; unchanged clocks/accounting across renewal. Use synthetic caches,
actual entrypoints and disabled external networking. Fixtures cannot prove AWS
timing. No API/resource-policy, release-source or adopted-hash change is proposed.

## Original campaign and need for a new authorization

A read-only immutable-ledger check at **2026-11-02 00:59:05.434950 UTC** found the
same boot, **2,102.970801 seconds elapsed**, and **1,197.029199 seconds remaining**
in `release-499-f1`. **It was not expired at that check.** The closed-stage deadline
had not started; next phase was close, usage 608. Nothing was resumed, reset or
written to its ledger. Remaining time is a decaying historical observation, not
permission or a promise of time left when this document is read. Recheck locally
immediately before any future decision; expired/boot-mismatched windows cannot
resume.

A second local check at **2026-11-02T01:03:10.897597+00:00** found **951.566535 seconds remaining**, with the same boot and no closed window. It was still unexpired; no action was resumed. Even the first check's 1,197-second remainder was below the proposed 1,560 seconds of closed-phase ceilings, before reviews. That does not prove faster completion impossible; it provides no full planned allowance.

The present campaign binds the exact old tooling commit and credential policy.
Continuing it with amended code fails the exact-tooling gate. Do not rewrite its
ledger, authorization or claim to migrate it. A new **`release-608-t1`** campaign
is therefore the smaller auditable route after amendment review, even if the old
window has not yet expired. Preserve its completed proof and consumed claim.
A new window requires **new explicit approval**; it is not an extension/reset of
the old campaign or authority to reuse its stale baseline.

The new tool must verify the completed f1 parent read-only (all 500–608 attempts,
608 cumulative, proof/authorization/claim/initial capture bindings, source/manifest/
scope hashes, nextPhase 1, no active phase or closed window), not pretend it was
a zero-call stopped parent. Carry **608 prior / 1,799 maximum / 1,191 additional**,
first possible ordinal **609**, same cumulative $1 allowance, source and exact
adopted hashes. Bind phase-specific limits and renewal policy into the new
authorization. Use unique `window-from-608-timing-1.json` and
`authorization-from-608-timing-1.json`. Neither exists or was created by this review.

A new initial collection requires separate approval of explicit fresh-at-start
capture for the exact existing identity/account, again acknowledging unproved
historical grant continuity. Capture only at that startup; strict continuity
then applies. Do not reuse the old authorization, checkpoint as an automatic
substitute, claim, deadlines or collection result. A fresh collection and artifact
agreement must precede a separate close decision.

## Exact proposed command sequence — blocked pending implementation and approval

**No new runnable tooling pin exists yet.** Current `bf1effb` hardcodes prior 499,
the consumed f1 campaign and 240-second phases. Do not run it for the proposed
608-based campaign. The interface below is exact, but requires the separately
implemented, verified and reviewed timing-tooling commit; no hash is invented.
The variable is deliberately mandatory. Commands are proposals, not permission.

After the amendment and a separate new initial-collection authorization:

```sh
cd /home/king/Neumont/Kurier
: "${REVIEWED_TIMING_TOOLING_COMMIT:?set only to the separately reviewed clean implementation pin}"
git switch --detach "$REVIEWED_TIMING_TOOLING_COMMIT"
python3 -B scripts/environment-deploy/driver.py check
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py template --fresh-checkpoint > .build/project-environments-execution/authorization-from-608-timing-1.json
```

Review exact identity/account, fresh-capture policy, phase limits, required-validity
renewal policy, 608/1,799, 3,300 seconds, fixed 1,800-second closed maximum and $1.
Set only `authorized` true after that separate approval, then execute once:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-608-t1 --authorization .build/project-environments-execution/authorization-from-608-timing-1.json --fresh-checkpoint
```

Stop after collection and publish/review it. Only if that campaign succeeds, is
unexpired, has sufficient unchanged original time/calls/credentials, and receives
**separate close approval**, prepare its bound false template:

```sh
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py prepare --campaign .build/project-environments-execution/release-608-t1 --phase close > .build/project-environments-execution/release-608-t1/close-approval.json
```

Review the full template including `engineApproval`; change only `authorized`
after separate approval. The proposed one-shot command is:

```sh
python3 -B scripts/environment-deploy/driver.py phase --campaign .build/project-environments-execution/release-608-t1 --phase close --approval .build/project-environments-execution/release-608-t1/close-approval.json
```

Stop and review the durable closure receipt and remaining original clocks. Only
after **separate deployment approval**, use the same clean pin:

```sh
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py prepare --campaign .build/project-environments-execution/release-608-t1 --phase deploy > .build/project-environments-execution/release-608-t1/deploy-approval.json
```

Review/change only `authorized` after approval, then once:

```sh
python3 -B scripts/environment-deploy/driver.py phase --campaign .build/project-environments-execution/release-608-t1 --phase deploy --approval .build/project-environments-execution/release-608-t1/deploy-approval.json
```

Stop after deploy or any failure; verify/activate/final and recovery remain separate
decisions. Any expired limit, mismatch, unknown operation or uncertain write stops
without restart, deadline extension, policy widening or write replay. Closure is
**no-go now**, notwithstanding the successful historical collection.

## Local verification

Repository `npm run verify` passed with external networking and dependency
downloads disabled: formatting, lint, tests, builds and Go tests/vet. Targeted
publication assertions checked pinned source/binary evidence, phase/credential
arithmetic, unchanged execution source, collection ledger state, all retained
checkpoint/claim/ledger hashes and absence of a new campaign/claim/authorization.
Disclosure and Prettier checks passed. No live timing or credential-capacity claim
is derived from those offline checks.
