# Authentication transport investigation and bounded diagnostics

This investigation and implementation are **local only**. No AWS request,
refresh, live preview, DNS lookup or external diagnostic call ran. Local routing
and resolver files were read without probing reachability or changing network
configuration. Only the explicitly requested GitHub publication uses external
network access; no other connected service is needed for this investigation.
The failed refresh's remote outcome remains **uncertain**.

## Confirmed historical evidence

The [review-3 result](project-environments-refresh-preview-result.md) and original
private receipts are unchanged. The private run directory contains only
window.json, authentication.jsonl and cumulative-usage.json. Its five-row ledger
has one `signin.CreateOAuth2Token` attempt at166, one transport-error outcome
with no HTTP status, then a latched stop. No raw SDK exception, subtype, trace or
socket phase was retained. Logging was disabled during authentication. No
identity verification, handoff, provider process, resource diff or packaging
output exists from that attempt.

The SDK transport interval was10.032seconds and the full window-to-stop interval
10.248seconds. The attempt was below both the500-call and3,600-second ceilings;
the stop was recorded as transport failure rather than budget exhaustion.
The reviewed client configured connect timeout up to10seconds and read timeout
up to30seconds, but elapsed time alone cannot establish which path failed.
These new diagnostics are **not** backfilled into the old ledger.

Local inspection of pinned botocore1.43.108 source confirms that
`httpsession.URLLib3Session.send()` maps DNS/new-connection errors to
EndpointConnectionError, SSL errors to SSLError, and typed connection/read
timeouts to their corresponding SDK exceptions. It can also raise HTTPClientError
or ConnectionClosedError. Its synchronous transport consumes the response body
before returning; absence of HTTP status in the outer receipt therefore does
not prove that the request never reached AWS. The SDK refresh path saves the
cache only after a parsed token response, but the historical exception cannot
establish any remote credential issuance/rotation result. No real token/cache
contents or token hashes were inspected in this investigation.

The original authentication SDK source pins pass unchanged. The new pin manifest
adds the SDK exception definitions and installed urllib3 exception/connection/
pool/retry sources used to classify typed errors and verify physical-send retry
behavior. This updates local dependency validation, not AWS endpoint permissions.

## Current local configuration — not historical reachability proof

Read-only file inspection found a configured loopback resolver, a resolver/search
configuration, and an NSS hosts chain including systemd resolve and DNS. No
resolver query or health check ran. No proxy environment variable was set in the
inspected sandbox or host context. The reviewed authentication client explicitly
sets proxies={} and TLS verification=true; it does not inherit an environment
proxy. No evidence establishes that a proxy was required.

The sandbox's `/proc/net/route` view contained no IPv4 routes. A separate local
host-context read, without making network calls, showed six IPv4 routes with a
default route and27 IPv6 entries. The historical run used host execution
permission. Current host and sandbox views differ, and the historical receipt
did not record its network namespace or route table. Matching boot metadata
alone cannot establish an unchanged route, resolver, firewall or upstream state.
No interface addresses, resolver addresses, search suffixes, proxy values,
private URLs or raw configuration files are published.

| Hypothesis                               | What supports considering it                                                | Why it is unconfirmed                                                                     |
| ---------------------------------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Connect timeout or blocked egress        | About10seconds elapsed; configured connect bound is10seconds                | No typed exception or socket phase retained; no historical route/firewall/packet evidence |
| DNS failure or resolver delay            | DNS is part of the transport path; a local stub resolver is configured now  | No resolver error code or historical resolver result retained; no lookup performed        |
| TLS/certificate or response-read failure | The pinned SDK can wrap these errors before returning a status to the meter | No TLS/read error type, peer certificate, response or phase retained                      |
| Execution-context/network difference     | Current sandbox and host routing views differ                               | Current observations are not a snapshot of the failed execution                           |

None of these is selected as the cause. No network repair, alternate endpoint,
proxy bypass or policy expansion is justified by the retained evidence.

## Narrow diagnostic change

When a future admitted authentication physical send raises, the meter records
one `authentication-transport-error` event before the unchanged outcome/stop
path. It includes the attempt number, existing timestamps and exactly these
new bounded fields:

| Field             | Allowed content                                                                                                                                    |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| exceptionCategory | Static tags: dns, connection, tls, tls-certificate, connect-timeout, read-timeout, timeout, proxy, connection-closed, transport, deadline, unknown |
| failurePhase      | dns, connect, tls, read, unknown, selected from known exception types; no phase inferred from a generic timeout's duration                         |
| errorCode         | Only an exact integer from explicit platform EAI/errno/SSL or selected OpenSSL X509 verification code allowlists; otherwise null                   |
| errorCodeDomain   | gai, errno, ssl, x509, or null; identifies the code namespace                                                                                      |

The classifier examines at most16 exception objects, tracks cycles, and inspects
only exception links, SDK kwargs['error'], known urllib3 nested exceptions and
a bounded number of exception-valued arguments. It never formats an exception,
records a class name/message, or reads request/response URL/header/body fields.
Strings and arbitrary dictionaries are ignored. Numeric strings, booleans,
unknown codes and large integers are rejected. Unknown types/phases remain
unknown. A typed phase describes the known error path, not remote AWS effects.

No diagnostic record creates an extra physical attempt. Durable admission still
precedes forwarding; one failed physical attempt remains charged even when its
outcome is uncertain. Diagnostic write failure latches a stop and cannot produce
retry or handoff. Repeated authentication operations remain forbidden. Provider
read-only policy bytes remain at SHA-256
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
Authentication endpoint/grant rules and provider endpoint rules are unchanged.

## Accounting and retained bytes

Cumulative accounting is now **135+30+1=166/500**, with334 calls arithmetic
headroom. Baseline validation hashes and checks the original inventory/preview
ledgers, the existing sanitized auth projection, private failed authentication
ledger/window/terminal usage receipt and failed campaign claim. The failed
attempt is counted without converting its uncertain outcome into success.

The old `window-from-165.json` claim is retained and validated at SHA-256
`87e39ae4b58c5942d951a7c12b5f5534e23c6f8fc64a8fc16db591921f5bd108`;
failed authentication ledger SHA-256 remains
`5316b2ec89fd7ff682f175a6cd14008a840fe2438814c368ab15a05023b997d7`.
No old claim/ledger is deleted, edited, replayed or resumed. The old165-based
code remains blocked by that claim. A future authorized166-based window would
use a distinct `window-from-166.json` claim and inherit the frozen166 baseline.
That new claim and a review-4 output do **not** exist after this local work.
New authentication starts at167; one refresh plus identity read would leave
provider prior168, next ordinal169, and332 provider attempts under the same500
ceiling. Identity-only would leave333 provider attempts. One original60-minute
deadline still covers both phases; no cross-phase clock reset is introduced.

Release source remains clean at
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. Retained release, native clients,
frontend and rollback bytes passed local integrity checks. No packaging changed
and no release artifact hashes changed. Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
Historical publication/checksum manifests remain untouched; new tooling hashes
are published in this investigation's own manifest. Same cumulative **$1
procedural allowance**; billing and unused dollar balance remain unmeasured.

## Offline verification

Seven new regression tests exercise ten representative failures through the
actual pinned SDK URLlib3Session.send method with fake connection pools and
socket/DNS creation disabled. They cover temporary DNS failure, refused and
unreachable connections, generic TLS and certificate verification errors,
connect/read/generic timeouts, unknown transport errors and absolute-deadline
interrupts. Each asserts a durable attempt before the fake send, sanitized tags/
codes, one charged attempt, no retry, no authenticated marker and no handoff.
Additional checks cover cycles, non-stringifiable errors, invalid numeric codes,
missing TLS codes, diagnostic persistence failure and tampering with every frozen
proof. Only synthetic secret canaries appear in test inputs; none enter receipts.

The complete suite has37 unit tests and15 offline transport scenarios, including
SST/Pulumi/Go/Node sharing, remaining334-call exhaustion, original deadlines,
mutation/endpoint denials and descendant shutdown. The concurrent fixture now
derives its four-call ceiling from PRIOR+4 instead of its obsolete fixed169.
The complete suite and repository format/lint/tests/build/Go verification run
inside outer network namespaces without external routes; dependency downloads
are disabled for repository verification. [Machine findings and test evidence](project-environments-auth-transport-investigation.json)
and [current hashes](project-environments-auth-transport-investigation.sha256)
record all checks passed. The seven diagnostic tests reran after strengthening
exact-field/URL-exclusion assertions; transport/runtime source hashes are
unchanged from the complete transport-suite snapshot.

## Proposed recovery procedure — not executed or authorized

1. Review the diagnostic/accounting commit and its offline evidence. Keep all
   existing receipts, the old failed campaign claim, release source and rollback
   bytes. Review that unknown historical cause and remote refresh outcome remain
   unknown, rather than approving a presumed network fix.
2. If a later live attempt is wanted, obtain separate explicit authorization for
   one new60-minute window and at most334 additional physical attempts, including
   all refresh/identity requests, with cumulative ceiling500 and the same $1
   allowance. Pin that reviewed commit and a fresh output such as review-4. Do
   not reuse/delete either the failed run or its claim. The production166 claim is only
   created by an authorized live invocation, never by local check/tests.
3. Use the same existing identity/account and unchanged endpoints. Do not assume
   the old refresh token is valid, invalid, or safe to repeat based on the missing
   response. The reviewed existing-session path may stop if that session cannot
   refresh; no interactive login, replacement identity, cache repair or repeated
   refresh is part of this procedure. Fresh identity verification must complete
   before provider work. Any failure stops the entire attempt.
4. Review only sanitized new category/phase/code evidence and updated cumulative
   usage after that attempt. If it establishes a repair need, propose its exact
   scope for separate review; do not change routes, resolver, proxy, CA trust,
   firewall or endpoint policy automatically. No automatic continuation follows
   success or failure, and no further live command is run in this task.

All deployment, stage closure, migration, activation, identity/project creation,
live target requests, live refresh/preview, policy widening and network repairs
remain held. OpenAPI production scope remains separate. Stop for review.
