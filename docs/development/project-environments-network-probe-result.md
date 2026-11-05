# Authorized network diagnostic — DNS answered, IPv4 TCP timed out

After the offline gate passed, implementation commit
`b3e27a4be6f429e334be554d10897ee7e93b6981` executed **once** in the authorized
host context under the accepted scope atcb4bfc9. It stopped with exit1 and
reason tcp-failed. No retry, second execution, authentication or preview followed.

The created receipt is dated **2026-10-28 21:42:02.546 UTC** and the stop receipt
**21:42:12.916 UTC**. One absolute45-second deadline covered preparation and
operations; total elapsed time was **10.381seconds**, below that deadline.

| Operation                   | Allowed | Actual result                                                                                        |
| --------------------------- | ------: | ---------------------------------------------------------------------------------------------------- |
| DNS client query            |       2 | **2**: one A and one AAAA, exact absolute Sign-In name via configured127.0.0.53:53/UDP; both NOERROR |
| Public-address TCP connect  |       2 | **1 IPv4 attempt**, port443; timeout after10.011seconds                                              |
| TLS handshake               |       1 | **0**, because no TCP socket connected                                                               |
| AWS HTTP/API/authentication |       0 | **0**                                                                                                |

A returned one public IPv4 address. AAAA returned no address, so there was no
permitted IPv6 target and no second connection attempt. No alternate IPv4,
alternate resolver, DNS search/alias follow-up, TCP retry or TLS fallback ran.
The resolver may have answered from cache; this does not prove upstream DNS
reachability. Its upstream recursion/retry/background traffic was outside the
controlled client count, as explicitly accepted by the user.

The measured execution context was **net:[4026531833], UID1000**. Local route
lookup succeeded for the selected public IPv4 address and selected a Wi-Fi
interface. Private context, selected address and route details are retained;
no interface/source/gateway/address values are published. The credential-free
executor loaded only public CA anchors for a possible verified TLS handshake;
it did not load the AWS credential chain or session cache. TLS was not reached,
so certificate validity and TLS reachability remain untested.

The sanitized exception helper recorded **timeout**, phase unknown and no numeric
code. The known failing operation is specifically the raw TCP connect: the
outcome is attached to kind=tcp and no TLS attempt occurred. This establishes a
**current IPv4 TCP connection timeout for one returned address** in a measured
host context. It does not establish which routing, firewall, VPN, upstream or
endpoint condition caused it, or prove a general AWS outage. It cannot assign
a cause or remote token outcome to either previous refresh. **Both uncertain
refresh outcomes remain uncertain.** No repair or protection change is supported
or performed by this diagnostic alone.

## Accounting and retained proof

| Counter                                     |       Value |
| ------------------------------------------- | ----------: |
| Prior cumulative usage                      |         167 |
| Charged AWS-destination connection attempts |       **1** |
| New cumulative usage                        | **168/500** |
| Remaining arithmetic headroom               |         332 |
| Separate DNS client counter                 |         2/2 |
| Separate TLS counter                        |         0/1 |

Each counter was durably written before its operation. The TCP contact was
charged conservatively even though connection setup failed; DNS/TLS counters
are separate and are not added again to the AWS total. Kernel packet
retransmissions are not extra socket attempts. The source binds all operations
to the single deadline and the exact endpoint/destination rules.

The new diagnostic window-from-167 claim is preserved and blocks another167-based
action or replay under a different output directory. The existing165/166 claims
and all historical inventory/preview/authentication ledgers remain unchanged.
The reviewed parent baseline/check still describes167; the diagnostic ledger
and terminal receipt establish actual168 usage. No parent accounting proof was
rewritten. Any future live work requires review of168-based accounting and its
new proof chain; no authorization follows from332 unused calls or connectivity
observations. Same cumulative **$1 procedural allowance**, billing and unused
dollar balance unmeasured.

The [sanitized result](project-environments-network-probe-result.json),
[public ledger projection](project-environments-network-probe-ledger.jsonl) and
[new manifest](project-environments-network-probe-result.sha256) record the
operation sequence, counters, bounded diagnostics and private receipt hashes.
The projection omits private context/address/route data; original files stay
intact in `.build/project-environments-network-diagnostics/review-1/`.
Raw exception messages, DNS packets, credentials, headers, bodies and URLs are
not recorded or published. No HTTP or application bytes were sent to the endpoint.

## Verification and execution history

The [implementation packet](project-environments-network-probe-executor.md) and
[offline gate receipt](project-environments-network-probe-offline.json) show
**22 new/59 total unit tests,15 offline process/transport scenarios, Python
lint/format and full repository verification passed before execution**. Test and
repository checks had no external routes; dependency downloads were disabled.
No functional offline test failed. The gate pins executor/test/accounting/policy/
diagnostic source hashes, and live execution required a clean exact commit and
fresh output. Scope preflight passed; no broader permission or destination was
required. Source/accounting/endpoint policy were not modified after that gate.

Exact once-only command, retained as history rather than instructions to rerun:

```sh
python3 -B scripts/preview-meter/network_probe.py \
  --implementation-commit b3e27a4be6f429e334be554d10897ee7e93b6981 \
  --gate /home/king/Neumont/Kurier/.build/project-environments-network-probe-offline/gate.json \
  --output /home/king/Neumont/Kurier/.build/project-environments-network-diagnostics/review-1
```

Release source remains clean at
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. Local integrity checks before/after
verified prepared ZIPs/bootstraps, frontend/native clients and retained rollback
ZIPs/bootstraps/frontend/CloudFront function bytes. No release packaging, resource
diff, activation or deployment was generated. Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`;
preview-policy SHA-256 remains
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
Publication checks passed for sanitized fields/counters, retained private and
historical hashes and formatting; they made no further endpoint requests.

Stopped for review. No HTTP, credentials, refresh, preview, alternate resolver,
settings/cache repair, endpoint-policy widening or automatic retry/continuation.
All deployment, stage closure, migration, activation, identity/project creation
and live target holds remain. OpenAPI production scope remains separate.
