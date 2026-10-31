# Authorized review-4 refresh-preview — connect-timeout stop

The one authorized command ran at clean reviewed tooling commit
`7e6e5046407ae9bde96d2e9ac5ac19ce3a0767ad` with unchanged clean release source
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. It stopped during authentication
after one existing-session refresh attempt. Exit1: “Preview launcher failed
closed. No automatic retry or continuation.” No identity verification or provider
work followed, and no restart, repeated refresh, repairs or widening occurred.

```sh
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 7e6e5046407ae9bde96d2e9ac5ac19ce3a0767ad \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-4
```

This is execution history, not a replacement command or permission to rerun it.
The failed review-3 run and window-from-165 claim were preserved; review-4 used
a fresh directory and the reviewed166 baseline with a distinct window-from-166
claim. Both failed campaign claims remain intact and block replay.

## Sanitized evidence and limits

The window started **2026-10-28 20:17:20.310 UTC** (14:17:20 MDT), with one
original60-minute deadline nominally at21:17:20.310 UTC. Stop was recorded at
**20:17:30.428 UTC**,10.118seconds after window creation. The attempt-to-outcome
interval was10.033seconds. The call/time budgets were not exhausted.

The original six-row authentication ledger contains:

1. Created/shared-window receipts: prior166, ceiling500,3,600seconds.
2. One `signin.CreateOAuth2Token` physical attempt at ordinal167, HTTP/1.1.
3. One bounded diagnostic: **exceptionCategory=connect-timeout**, **failurePhase=connect**,
   errorCode=null, errorCodeDomain=null, attempt167.
4. An outcome with status=null and transportError=true.
5. A latched transport-failure stop at167.

This is a **confirmed typed connect-timeout diagnostic**, rather than an inference
from elapsed time. The bounded diagnostic does not establish why connection
setup timed out: network path, routing/firewall behavior, endpoint reachability
and other underlying conditions were not probed or verified. No raw exception
message or trace is retained/published. No follow-up network/AWS diagnostic call
ran. It was not an endpoint-policy denial.

There was no HTTP status, parsed successful refresh, authenticated marker or
fresh STS identity response. The remote refresh/credential outcome remains
unverified and is treated as uncertain; this diagnostic is not independent proof
of remote state. The [previous failed refresh](project-environments-refresh-preview-result.md)
remains unclassified and uncertain; this new observation does not retroactively
attribute it. No cache or network repair was attempted.

There is no handoff, provider claim/directory/ledger, SST/Pulumi output, resource
plan or packaging receipt for review-4. Successful identity verification was
required before provider work; the failed authentication prevented that work.

The [sanitized machine receipt](project-environments-refresh-preview-review-4.json)
and [ledger projection](project-environments-refresh-preview-review-4-auth-projection.jsonl)
include only reviewed accounting/timing/category/phase/code fields and receipt
hashes. Credentials, token hashes, raw messages, headers, bodies, URLs, paths and
queries from transport diagnostics are excluded. The projection is a derivative;
the original private ledger, window/terminal usage receipts and claim remain
at their original paths and byte hashes. No original ledger was replaced.

## Accounting and artifact comparison

| Phase                      | Physical AWS attempts |
| -------------------------- | --------------------: |
| Original inventory         |                   135 |
| Original stopped preview   |                    30 |
| Previous uncertain refresh |                     1 |
| This refresh               |                 **1** |
| This identity verification |                 **0** |
| This provider preview      |                 **0** |
| Cumulative                 |           **167/500** |

One of334 additional authorized attempts was consumed. The333 remaining calls
are arithmetic headroom, not authority to continue or reset the window. The
reviewed tooling's local `check` still describes its frozen166 parent baseline;
the new terminal usage receipt and this result establish actual167 usage.
Accounting/tooling were not modified during this execution. A later live attempt
requires separately reviewed167-based accounting and explicit authorization;
neither failed claim may be deleted or resumed to bypass that review.

The same cumulative **$1 procedural allowance** remains. Actual billing, shared
historical spend and unused dollar balance remain unmeasured. No billing query
or measured cost claim was made.

No completed or partial resource diff exists because SST/Pulumi/provider never
started. No ZIP/frontend build or new packaging receipt exists; **there are no
artifact-hash differences**. Retained prepared ZIPs/bootstraps, native clients,
frontend files and rollback ZIPs/bootstraps/frontend/CloudFront function bytes
passed local integrity checks before and after the attempt. No additional AWS
inventory or rollback retrieval ran. Release manifest remains SHA-256
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`;
preview policy remains byte-identical at
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
All reviewed tooling, historical results and ledgers, and both old claims are
preserved. The new private authentication ledger SHA-256 is
`0bb82bf9b14cd14a95861424bf3103ebb13bdf8218c08cf5444965837a72de69`.

## Publication checks and remaining decisions

Local preflight verified reviewed commit/cleanliness, pins, baseline166, old
failed proofs, fresh output/claim, release source and retained bytes. After the
stop, sanitized projection fields/order/counts were checked against the private
ledger; all private receipt hashes and historical checksum/integrity checks
were rechecked. Repository format/lint/tests/build/Go publication verification passed via
`npm run verify` (exit0), inside a network namespace without external routes and
with dependency downloads disabled. These local checks made no further AWS
requests. Sanitized receipt/projection consistency and private receipt hash
retention checks passed. Current AWS resources,
IAM, stage/capabilities, generation, runtime behavior and fresh identity were
not inspected or confirmed by this failed authentication attempt.

The remaining decision is whether to authorize a separately scoped investigation
of connection timeout and, only with evidence, propose an exact recovery scope.
No network/cache repair, alternate endpoint or policy amendment is proposed or
applied by this execution. No automatic continuation follows this failure.
Deployment, stage closure, migration, activation, identity/project creation and
live target requests remain held. OpenAPI production scope remains separate.
