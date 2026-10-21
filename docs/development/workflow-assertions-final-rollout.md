# Final workflow-assertions rollout

The pinned release was deployed and assertions activated. Genuine cases A–G
passed their reviewed expectations. The campaign stopped before H admission on
a fail-closed transport denial during hosted-browser sign-in; H–L were not run.
There was no campaign restart, target replay, automatic reopening or downgrade.
The [machine-readable receipt](workflow-assertions-final-rollout.json) contains
exact readbacks, package hashes, immutable-evidence checksums and cleanup proof.

Davian accepted `f3a698d565fa29e99cd7f89517252994c32e44ef` and authorized the final
attempt after coordinator integration. Integration commits `2ebeea5` and
`1faf542` use the actual native transport, accepted GET-only reader, durable
logical counters, isolated metering and cleanup reserves. Initial local tests
covered 503→200, Retry-After/backoff, redirects, real CLI/Go/Node retries and
pagination, native fetch, Chromium HTTP/2 GETs, network-bypass denial and lost
write acknowledgment. The initial browser transport test did **not** exercise
Cognito authentication preflights. Coordinator startup exposed a local clock
initialization-order error after cloud preflight, before fixture creation; it
was corrected and its zero-call startup verified without resetting the window.

## Release, closure and activation

Application source remains `68d9f218225c7092a1e4fca0b66a21421d862a22`. The unchanged
[release manifest](workflow-assertions-release.json) SHA-256 is
`920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6`.
API/cleanup ZIPs each remain
`f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32`;
worker/fixture ZIPs each remain
`15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379`.
Frontend release remains
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`.

Fresh non-root account `747336059622`, region `us-east-2`, inventory and SST preview matched
all eleven reviewed resource changes, four packages and five frontend files,
with no IAM/infrastructure changes and unchanged preview bootstrap/state
metadata. Legacy A completed before closure; evidence remained available while
the unchanged e702 guard accepted matching terminal jobs, published outboxes and
immutable/settled-upload proofs. Separate fresh approval records authorized the
single close transaction, `active` version 13 → `workflow-migration` version 14, preserving accounting.
Clean source and build guards reproduced every pinned ZIP during deployment.
All five hosted file bodies matched their expected hashes before activation.

The first activation inspection timed out on a metered IAM GetPolicyVersion
read **before any activation transaction**. The stage remained closed version 14. One
fresh read-only inspection passed, the unused approval was retained, and a new
approval was generated from that readback. Complete guards passed again before
the single activation transaction. Its readback confirmed active version 15,
savedRequests 8 / assertions 1, unchanged generation `dev-api-initial-v1` and unchanged
accounting. Post-runtime platform inspection verified all four deployed code hashes
and preserved IAM/configuration, Free plan, quotas, concurrency and schedule.
No application fix or changed release artifact was required. The accepted rollback
restrictions still apply: old readers cannot be restored blindly after
schema8/assertions1 data. Reviewed closure and reconciliation are required;
no rollback was attempted.

## Genuine runtime findings

| Case | Verified behavior                                                                                         | New admitted steps | Target sends |
| ---- | --------------------------------------------------------------------------------------------------------- | -----------------: | -----------: |
| A    | Legacy seed completed before closure; schema 1, semantics marker absent                                   |                  2 |            2 |
| B    | Legacy whole-run rerun after activation retained schema 1 and frozen semantics; source evidence unchanged |                  2 |            2 |
| C    | Configured status/existence/equality checks passed with schema 2 evidence                                 |                  2 |            2 |
| D    | Status mismatch failed first step and skipped successor                                                   |                  2 |            1 |
| E    | Empty checks returned not_configured without changing execution                                           |                  2 |            2 |
| F    | Matching status 500 assertion passed while HTTP failure still failed the run                              |                  2 |            1 |
| G    | Sensitive reflected input was not_evaluated/sensitive_input; run failed                                   |                  2 |            2 |
| H–L  | Not admitted; decimal/browser/rerun/boundary/404/missing-vs-null runtime campaign unperformed             |                  0 |            0 |

Owner/foreign/anonymous 200/404/401 boundaries, pin/unpin accounting, immutable
source checksums, MCP legacy complete/summary projection and revoked-token 401
were verified. Five invalid numeric authoring DTOs received 400 before H's empty
workflow definition was created. H was never submitted. Seven genuine workflow
status GETs succeeded; the 503 backoff was verified locally, not needed live.
UUID-only target audits proved twelve new sends, including authorized consumers.
Bounded scans of four actual configured Lambda log groups found no known
synthetic binding values, coordinator-known access tokens, fixture passwords or
MCP token. Raw messages/captures/credentials were not published. This does not
prove absence of every credential or of later log delivery.

## Browser stop and proposed correction

The browser reached the pinned hosted files, then the meter latched write-held
before forwarding the rejected request. Its retained stop lacks category and
operation, so the exact live blocked operation cannot be established. A local
real-browser regression against a controlled origin reproduces a Cognito CORS
OPTIONS preflight being classified as an unapproved AWS write. That establishes
a transport compatibility gap, not definitive attribution of the retained live
stop. The rejected preflight is not counted as a forwarded AWS request.

A future correction should review narrowly scoped Cognito preflight handling,
charge each forwarded OPTIONS request, and test the actual browser authentication
flow through the same isolated transport. Also retain fixed held-request
category/operation/method metadata without headers, bodies or credential values.
No such compatibility change was used to resume this campaign. The historical
legacy-A503 cause also remains unestablished. Further target execution requires
separate review; unused admissions do not authorize another attempt.

## Bounded cleanup

The original transport ledger remains latched and unchanged. An explicit,
locally verified cleanup-only continuation inherited its first-attempt clock,
all counters and frozen ledger hash. It permits only GET/DELETE on these two
project roots, sign-out/deletion of the two existing provider users, reviewed
authentication and read-only AWS requests. It denies submission, target routes,
identity/project creation, deployment, stage writes and static browsing. It
cannot continue a parent with outstanding or uncertain writes. Its tighter
limits are inherited totals plus at most 60 API / 200 AWS attempts, within the
original 500/1500 caps, at most 20 minutes and the original 120-minute deadline.
The local transport regression proves scoped deletion, blocked campaign POSTs
and an unchanged parent ledger; unit tests cover scope, uncertainty, time and
absolute-cap enforcement.

Both normal project deletions were acknowledged once and immediate owner 404
verified. The larger project drains one upload ticket per maintenance pass, then
bounded metadata batches. The read-only observer was handed off to prioritize
provider deletion within the same deadline; no provider deletion had previously
been sent, and neither project DELETE was replayed. Both provider users were
signed out, deleted and independently confirmed UserNotFound. No manual Lambda
invocation, schedule/concurrency change or durable-row repair accelerated cleanup.
The second project has a tombstone-only Control partition, zero Protected children
and an empty evidence prefix. The first project
`2d9723c2-facf-40e1-8a2b-130853d3c0f8` was still pending at the last bounded
observation. Its DELETE and immediate owner 404 were acknowledged, but physical
completion is **not proved**. All further calls stopped when the cleanup window
ended. Both local proxies and their ephemeral CAs were removed.
Tombstones, immutable admission usage, user metadata and residual sweep
registrations are intentional retained metadata, not an empty-database claim.

## Limits and remaining review

Four prior steps/four sends were carried forward. This attempt consumed 14 steps
and 12 sends: **18 cumulative admissions and 16 cumulative sends**. Six identities and
six projects have now been created cumulatively; no further pair is authorized.
The 24-step final plan had zero reserve submissions. H–L were not admitted; the
mathematical ten unused steps do not authorize a new campaign.

The final authoritative counts are **68 API calls, including 7 polls, and
917 AWS attempts** (all operator AWS writes/auth also charged conservatively),
plus 9 static requests. First-to-last cloud outcome was 2619.274 seconds; cleanup
transport creation to last cloud outcome was 1151.646649 seconds, within 20 minutes.
All forwarded write attempts received known HTTP outcomes; expected 400 rejections
were not uncertain writes. The parent ledger's write-held stop remains intact. The
additional $1 allowance is procedural, not an AWS-enforced cap or verified bill.
Prior 48 API / 4 polls and second-attempt 574 AWS charges remain known history. Historical
first-attempt AWS-read totals and all actual billing totals remain unknown; no
combined actual AWS-read/billing total or unused dollar balance is inferred.

Full repository format/lint/test/build/Go verification passed. Local tests include
nineteen polling/coordinator cases, thirteen meter/cleanup unit cases, retained
activation/release suites and controlled transport regressions. The combined
guard/release/collector run passed 89 cases with one optional actual-maintenance
fixture test skipped; its accepted earlier integration evidence is unchanged. Genuine runtime
findings above come from API/AWS/object readbacks and fixture audits; no new
Davian-reported observations are substituted for those checks. Trello read/write
synchronization is unavailable because its connector requires authentication.
Broad fault injection, elapsed-retention and PITR exercises remain excluded.

## Collector limitations requiring review

The final accounting collector compared a selected pre-cleanup projection with
a full post-cleanup row. A valid unchanged row with additional fields fails that
comparison, so its generic failure does not establish AWS drift. The full final
readback was not retained; post-cleanup accounting preservation remains unverified.
The last proved pre-cleanup monthly count was 20, preserving the original 6 plus 14
new admissions. No row was changed to satisfy a collector. Future collectors must
capture the same complete projection on both sides, including versions and all
accounting fields; dropping extra fields after the fact would not prove history.

The final proof launcher also passed a fractional remaining-millisecond timeout
to Node spawnSync, which rejected it locally. It sent no final-proof AWS request.
A local regression reproduces rejection and verifies integer milliseconds work.
The [actual accounting observer](workflow-assertions-final-collectors/cleanup-observer.mjs.txt)
and [actual proof launcher](workflow-assertions-final-collectors/finish-proofs.mjs.txt)
are retained as inert review text. Their failures were not hidden or replayed.

Next review must address the browser preflight compatibility test, consistent
accounting projections, integer timeouts, and separately authorized read-only
verification of the pending cascade/accounting. H–L, additional identities and
any new target execution remain unperformed and require review. No application
release change, fault injection or PITR exercise is proposed here.
