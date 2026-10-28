# Authorized preview attempt — expired-credential refusal

Davian authorized one read-only preview invocation using tooling
`0d1defa8460d0e12c592be7b6d79ce1d7ab4758e` and unchanged release source
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`, with a new60-minute/335-call ceiling,
165 prior calls and the same cumulative $1 procedural allowance.

**The launcher refused expired cached credentials before starting the proxy or
SST. Zero new AWS attempts; cumulative usage remains165/500.** No refresh,
repair, policy change or restart followed. This is the result of the one
invocation, not a completed resource preview.

## Exact invocation and confirmed local refusal

From the clean reviewed tooling checkout, after checksum/source/artifact checks:

```sh
python3 -B scripts/preview-meter/run.py preview \
  --review-commit 0d1defa8460d0e12c592be7b6d79ce1d7ab4758e \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-2
```

The command ran once and exited1 with the fixed message:

> Preview launcher failed closed. No automatic retry or continuation.

A local-only inspection of cache expiry metadata and the exact reviewed helper
confirmed `credential-window-too-short-refresh-held`. Credentials expired at
**2026-10-28 03:02:26 UTC** (October27 21:02:26 MDT), before this invocation.
The reviewed helper requires at least120seconds of remaining validity and checks
it before calling `launch()`. The cache had the intended non-root profile/account
metadata; no live caller-identity verification was performed. No credential
values, cache contents, token hashes or provider-chain output are published.
The inspection timestamp in the machine receipt is a local diagnosis timestamp,
not a proxy/window start time.

The fresh `review-2` output directory does not exist. The proxy, sandbox and SST
were not started. No meter ledger or budget-creation timestamp exists for this
attempt. No synthetic zero-call ledger was substituted for an actual meter run.

[Sanitized machine receipt](project-environments-preview-credential-stop.json)
records the command, exit, local guard diagnosis, accounting and artifact
comparisons. Network denied-endpoint diagnostics are absent because no network
request reached a meter. This local expiry refusal does **not** attribute the
[earlier endpoint-held stop](project-environments-preview-result.md).

## Accounting, diff and artifacts

| Item                   | Result                                                                    |
| ---------------------- | ------------------------------------------------------------------------- |
| Retained inventory     | 135 physical AWS attempts, original ledger unchanged                      |
| Retained first preview | 30 physical AWS attempts, original stopped ledger unchanged               |
| This invocation        | **0/335** additional AWS attempts                                         |
| Cumulative             | **165/500**, unchanged                                                    |
| New window             | Proxy budget never created; no start time or elapsed meter window         |
| Continuation           | None: the one authorized invocation failed; no automatic retry or restart |
| Resource diff          | Unavailable: SST/Pulumi never ran; no resource events or completed plan   |
| Network denial receipt | None: refusal occurred before proxy startup                               |
| Candidate packaging    | No ZIPs/frontend or packaging receipt generated                           |

The335-call arithmetic headroom does not authorize a replacement invocation or
clock reset. The same cumulative **$1 procedural allowance** is retained, not
replenished. No billing/price call was made; actual historical/shared spend and
unused dollar balance remain unmeasured.

Original prepared packages/bootstraps, clients, frontend files and retrieved
rollback bytes passed local integrity checks before and after the refusal. There
are no regenerated artifact hashes to compare or accept. Release-manifest SHA-256
remains `94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
Original inventory ledger remains
`1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171`;
original stopped preview ledger remains
`e65635f9b3e21f406cb70dcd0ba7ae71f952e203e54d30f6474da591abe525eb`.
The endpoint policy remains byte-identical at
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.

## Publication verification and holds

Local pin/checksum and retained artifact checks passed. The receipt's zero-call
accounting, absence of output/meter artifacts, local links and repository
format/lint/tests/build/Go verification were checked for publication. These are
local checks; current AWS stage/capabilities, generation, accounting, IAM and
resources were not inspected or changed by this attempt.

No deployment, closure, migration, activation, identity/project creation or live
target requests occurred. No credential refresh or repair was attempted; no
policy amendment was applied. All live work stopped and existing holds remain.
OpenAPI production scope remains separate. Any further attempt requires separate
authorization and valid credentials; none is inferred from this result.
