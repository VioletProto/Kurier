# Refresh and preview shared-budget preflight

The newly authorized refresh and preview were **not started**. Local inspection
and an offline probe establish that unchanged reviewed tooling
`0d1defa8460d0e12c592be7b6d79ce1d7ab4758e` cannot carry refresh usage and the
original deadline into the provider meter. This conflicts with the simultaneous
requirements to keep tooling identical, make no repairs, and meter refresh plus
preview in one window. No refresh call was spent before discovering this gap.

Release source remains `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.
The feature branch was clean at `27410356389f976c320b475cbb0eeb1d297a334c`
before this documentation-only follow-up. Reviewed live tooling is unchanged;
no launcher invocation, new preview directory, ledger or live window was created.

## Confirmed locally

`git diff --exit-code 0d1defa8460d0e12c592be7b6d79ce1d7ab4758e -- scripts/preview-meter scripts/attempt-meter scripts/environment-inventory-meter.py`
passed. Reviewed checksums and `python3 -B scripts/preview-meter/run.py check`
passed, including original ledgers, retained packages and rollback bytes.

In `scripts/preview-meter/run.py`, `cached_credentials()` only reads the existing
cache; it cannot refresh. In non-fixture mode, `launch()` always writes budget
`prior=165, maximum=500, seconds=3600`. Its optional `budget` argument is used
only in fixture mode. The CLI has no refresh-receipt/deadline input. The proxy
constructs a new Budget with a new monotonic start time. The earlier inventory
meter can meter a refresh, but does not hand off its ledger or clock to this
launcher. The preview endpoint policy denies authentication writes.

The [offline probe source](project-environments-refresh-handoff-probe.py.txt)
imports the unchanged launcher, mocks platform copying and **all subprocess
startup**, and passes a simulated one-call/ten-second refresh handoff. It reads
the actual generated config, then tests the unchanged Budget with a fake clock.
It makes zero AWS calls and starts zero subprocesses. An ephemeral loopback
port is allocated locally before mocked subprocess startup; the sandbox refused
that socket, so the same offline probe ran with local execution permission.
The temporary output is discarded; this is not a preview invocation.

The [machine receipt](project-environments-refresh-handoff-preflight.json)
records the assertions and reviewed source hashes. Observed configuration remains
165/500 and 3,600 seconds despite the requested 166/500 and 3,590 seconds.
Its first simulated provider call is numbered166; after one refresh it should
be167. At the original window deadline the fresh provider clock still admits
checks. These are fixture results, not claims about a live refresh or AWS.

## Exact proposed handoff for review — not implemented

A narrow tooling amendment is needed before execution under the combined budget:

1. Establish one monotonic deadline before the first authentication attempt,
   with immutable parent usage165 and cumulative ceiling500. Persist the window
   receipt before sending any request. Never reset its deadline between phases.
2. Meter only the existing CLI session's `signin.CreateOAuth2Token` refresh,
   with the same identity/account restriction and no identity creation. Record
   each physical attempt durably before forwarding, including retries, redirects
   and failed transports. Stop on ambiguous outcome, expiry, or any unapproved
   operation; no automatic refresh retry/restart. Keep credentials private.
3. If live `sts.GetCallerIdentity` verification is needed before this authorized
   authentication write, meter that read within the same allowance as well.
   Require the intended account and same non-root identity; stop on mismatch.
   Cached identity metadata alone is not a fresh live verification.
4. Validate the parent ledgers, refresh ledger, outcomes and shared deadline
   before starting any provider. Provider prior usage must be165 plus **all**
   actual calls in the authentication/identity phase. For example one identity
   read plus one refresh leaves333 calls and starts the next ordinal at168.
   Pass the original deadline, rather than granting another3,600 seconds.
5. Keep authentication outside the preview's read-only endpoint policy. Keep
   that policy byte-identical. Offline tests must prove cross-phase exhaustion,
   durable receipt validation, retry/transport accounting and deadline continuity
   before this amendment is eligible for live review.

This proposal is not a runnable command or an applied repair. The existing exact
pinned command cannot consume this handoff. Review must resolve the requirement
for identical0d1defa tooling versus a separately reviewed handoff amendment before
any refresh or preview. No unreviewed wrapper or runtime monkeypatch was used.

## Accounting, artifacts and holds

New physical AWS calls: **0**. Original inventory135 plus preview30 remains
**165/500**, with335 calls arithmetic headroom. No synthetic zero-call ledger
replaces either frozen ledger. The new authorization's live window was never
started. The cumulative **$1 procedural allowance** remains unchanged; actual
billing and unused dollar balance are unmeasured. No AWS inventory, resource
diff, live identity confirmation or current stage inspection was performed.

All retained release and rollback bytes passed local verification. No packaging
changed and no new artifact hashes exist. Release manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
Endpoint policy SHA-256 remains
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
Original inventory ledger SHA-256 remains
`1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171`;
original preview ledger SHA-256 remains
`e65635f9b3e21f406cb70dcd0ba7ae71f952e203e54d30f6474da591abe525eb`.

Repository format, lint, tests, build and Go verification passed via
`npm run verify` (exit0). The offline proof, reviewed tooling comparison, source
cleanliness and frozen checksum/artifact checks passed. These results concern
local code only. All deployment, closure,
migration, activation, identity/project creation and live target holds remain.
No policy widening, repairs, refresh, preview, restart or live continuation occurred.
OpenAPI production scope remains separate.
