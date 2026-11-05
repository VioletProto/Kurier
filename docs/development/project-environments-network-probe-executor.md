# Minimal network diagnostic executor

The user accepted the scope at `cb4bfc922c149e5c8a814e583851f89d95608908`,
including the DNS-recursion counting limitation, and authorized implementation,
offline verification and one conditional execution. This implementation adds no
HTTP, authentication, credential-chain, refresh or preview capability.

[Executor](../../scripts/preview-meter/network_probe.py) uses one45-second
monotonic deadline including preparation, a host-namespace/UID check against the
accepted context, the configured127.0.0.53 resolver only, exact Sign-In A/AAAA
wire queries, public-address validation and a strict family order. At most two
UDP DNS sends, two TCP connect attempts and one verified TLS handshake are
admitted. IPv6 is tried only if IPv4 is absent/fails; after the first TCP success
there is one TLS attempt on that socket and then an unconditional stop. No HTTP
request or application bytes are sent. Failed DNS, unexpected scope, persistence
failure or exhausted limits stop without retries or an alternate resolver.

Each operation counter is fsynced before backend work. Each TCP attempt advances
cumulative accounting from167 toward at most169, even on transport failure.
DNS and TLS counters remain separate. TLS uses the pinned SDK's CA anchors with
CERT_REQUIRED and hostname checking; no credentials/cache/client certificate is
loaded. Private context and route/selected-address files stay outside published
diagnostics. The ledger contains only counters, phase/category/code and bounded
TLS/DNS outcomes, not raw exception text, headers, bodies or URLs.

A durable single-use window-from-167 claim prevents replay under another output
path or old167-based launcher. Existing window-from-165/166 claims and every
historical ledger remain unchanged. The claim is a diagnostic claim, not a
refresh/preview activation. The same cumulative $1 procedural allowance remains;
billing is unmeasured. The existing resolver's upstream recursion/background
traffic is outside the controlled two-client-query counter, as explicitly
accepted by the user. Kernel packet retransmissions are not new socket attempts.

[Offline regressions](../../scripts/preview-meter/test_network_probe.py) cover
DNS framing/parsing and in-packet aliases, malformed/truncated/private/wrong-name
answers, strict family order, DNS/TCP/TLS failures, deadline continuity and signal
interruption, all counters, global ceiling, pre-send persistence, sanitized
outputs, absence of HTTP writes/DNS fallback, verification-gate hashes and
exclusive claims. **22 new executor tests,59 total unit tests and15 offline
transport scenarios passed.** Full repository format/lint/tests/build/Go checks
passed with external routes removed and dependency downloads disabled. The
[offline receipt](project-environments-network-probe-offline.json) binds the gate
to exact executor/test/accounting/policy/diagnostic hashes. No functional offline
test failed. A local orchestration receipt was rechecked to retain the repository
exit status; this did not run external work.

The CLI requires a clean exact implementation commit, this matching offline gate,
unchanged source44f1fb7 and retained release/rollback bytes, the accepted host
namespace/UID/resolver, and a fresh output under
`.build/project-environments-network-diagnostics/`. If any requirement differs,
it stops before probing. Only the explicitly authorized once-only invocation is
eligible. Its result and exact pinned command will be published separately after
execution; offline success does not authorize any refresh or preview.

Release source remains `44f1fb79d943501c95e06fcc1a6114111a7c1ea4` and preview
policy remains SHA-256 `ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
Both uncertain remote refresh outcomes are preserved. All deployment/runtime,
identity/project creation, refresh, preview and settings/cache repair holds
remain. No automatic continuation follows connectivity success or failure.
