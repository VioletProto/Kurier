# Authentication network-context comparison and proposed probes

This step performed **local read-only inspection only**. No authentication,
preview, DNS lookup, TCP/TLS probe, AWS call, network/cache repair or endpoint
change ran. Both failed refresh outcomes remain uncertain. Accounting now
recognizes **135+30+1+1=167/500**, with333 calls arithmetic headroom; the same
cumulative $1 procedural allowance remains unmeasured, not replenished.
Only the requested GitHub publication uses external network access.

## Execution-context comparison

The retained local tool history was inspected without publishing command output
or credential material. The actual invocation entries establish these facts:

| Attribute                                             | Successful inventory/session refresh                                                                                   | Review-3 failure                                                                              | Review-4 failure                                                 |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Date/time UTC                                         | 2026-10-27 19:20; credential read completed19:20:24.455                                                                | 2026-10-28 19:53:34–19:53:44                                                                  | 2026-10-28 20:17:20–20:17:30                                     |
| Tool execution permission requested                   | require_escalated / host                                                                                               | require_escalated / host                                                                      | require_escalated / host                                         |
| Authentication code                                   | boto3.Session(region_name=us-east-2).get_credentials().get_frozen_credentials(), metered with allow_refresh=True       | Explicit existing-session SDK fetcher                                                         | Same fetcher with typed diagnostics                              |
| Provider namespace before authentication              | None                                                                                                                   | None                                                                                          | None                                                             |
| Credential/config behavior                            | Default credential chain and SDK configuration; historical effective proxy/endpoint snapshot absent                    | Existing cache only, SDK configuration files disabled, proxies={}, explicit reviewed endpoint | Same                                                             |
| Observed result                                       | One metered token attempt and successful credential read; original HTTP status not retained; tool reported0.373seconds | One uncertain transport failure, subtype not retained                                         | One typed connect-timeout, phase connect, no numeric code/status |
| Historical process namespace/routes/firewall snapshot | Not retained                                                                                                           | Not retained                                                                                  | Not retained                                                     |

The successful refresh invocation was submitted19:20:20.631 UTC; the failed
invocations were submitted19:53:29.258 and20:17:17.153 on the following day.
Their host-execution request is confirmed from the exact invocation entries,
not a search match in later documentation or inspection commands. None invoked
bwrap/unshare before authentication. Launcher source calls authentication
directly before creating the provider proxy/sandbox; the failed runs have no
provider directory or handoff. **The SST provider sandbox was not the point
where authentication timed out.**

The authentication processes have exited. Their actual namespace identifiers,
routing-policy selections, effective firewall rules and historical resolver/
proxy environment cannot be read retrospectively from /proc. Source and tool
metadata support host-context authentication in all three, but do not establish
that the actual network state was identical. Successful earlier inventory reads
prove earlier access, not current reachability. The actual successful token
hostname/address was not retained; current SDK defaults cannot supply a missing
historical destination. Current default AWS configuration has only login_session
and region, no service/endpoint override. No credential/cache contents were read.

## Current local inspection — observed facts and gaps

| Surface                         | Observation                                                                                                                                                                                                                                                               | Limit                                                                                                                                                                                              |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Network namespace               | Current sandbox net:[4026533510]; separate host inspection net:[4026531833], UID1000, no CAP_NET_ADMIN                                                                                                                                                                    | These are inspection-process IDs, not IDs captured from the historical authentication processes. Host PID-1 namespace access was denied.                                                           |
| Host routes                     | 46 IPv4 routes across main/local/table52; one main-table default through Wi-Fi;11 routes use a VPN device                                                                                                                                                                 | Table52 has no default route. This does not demonstrate that AWS traffic used a VPN exit or that a route caused the timeout. All-table counts differ in scope from earlier main-table/proc counts. |
| Policy routing/VPN              | Seven policy rules, three fwmark rules; Tailscale interface and table52 present                                                                                                                                                                                           | Current overlay policy is not historical flow evidence. No VPN client/control-plane query or settings change ran.                                                                                  |
| Resolver                        | 127.0.0.53 loopback stub, search configuration, NSS resolve/DNS sources                                                                                                                                                                                                   | No DNS query, resolver health check or upstream contact was made by this investigation.                                                                                                            |
| Proxy/SDK config                | No inspected proxy/endpoint/CA override environment variables; current default AWS profile has no endpoint override; authentication explicitly disables proxies and verifies TLS                                                                                          | Historical environment was not captured. No evidence establishes a required proxy.                                                                                                                 |
| Live firewall                   | nft list ruleset and iptables-save reads were denied; host process lacks CAP_NET_ADMIN                                                                                                                                                                                    | Active rules were not obtained. No sudo, capability changes, protection disabling or rule modifications were attempted.                                                                            |
| Readable firewall configuration | Static nftables has input/forward drop policies, established/related accept and no output chain. UFW is configured enabled with default output ACCEPT; both readable user-rule files contain no direct user-output DROP/REJECT rules. Static iptables policies are ACCEPT | Configuration text is not proof it was loaded or that current kernel rules match. It does not identify an outbound AWS block.                                                                      |
| Other VPN files                 | No .conf files found in inspected WireGuard/OpenVPN configuration directories                                                                                                                                                                                             | This is not proof that no VPN or higher-level policy exists.                                                                                                                                       |

Raw route tables, policy rules, firewall-read errors and selected system journal
records are retained privately under `.build/project-environments-context-inspection/`.
No private interface addresses, gateway/upstream addresses, SSIDs, MACs, VPN peer
identifiers, proxy values, journal messages or credentials are published.

The selected NetworkManager/tailscaled/systemd-resolved journal contains865
records spanning **2026-10-28 03:47:47–20:15:05 UTC**. It does not cover the
successful inventory window. At19:35:39 it reports NetworkManager reaching
CONNECTED_GLOBAL, followed by a VPN link-change event. At20:15:05, roughly two
minutes before review-4, Wi-Fi supplicant transitions completed→authenticating→
associating→completed, DHCP transactions restart, and a new lease is recorded.
These events show preceding network reconfiguration and completion; they do
**not** establish a fault during20:17 or prove causation. The DHCP messages refer
to configured transaction timeouts, not a recorded DHCP timeout failure. No
selected-unit event was recorded during either exact failed-send interval.
Absence of such a log is not proof of network health or packet delivery.

The current boot timestamp predates the successful inventory, and both failed
window boot IDs match the current host. Successful inventory did not retain a
boot ID. Neither a reboot between attempts nor uninterrupted identical network
state can be inferred from the available journal coverage.

## Finding and recovery position

The confirmed review-4 failure is typed **connect-timeout/connect**, no numeric
code or HTTP status. The review-3 cause is unclassified. Current host/sandbox
separation, VPN policy and earlier Wi-Fi/DHCP events are relevant context; none
establishes the endpoint-specific cause. Live firewall visibility and historical
flow/namespace evidence are incomplete. Both remote token issuance/rotation
outcomes remain **uncertain**; no new diagnostic is retrofitted into either
ledger and no token is assumed valid, invalid or safe to repeat.

**No settings repair, VPN/protection disabling, proxy change, CA change, timeout
increase, alternate AWS endpoint or cache repair is evidence-supported now.**
The next supported step is review of the bounded, unauthenticated probe proposal
below, or a separately authorized privileged read of live firewall rules. Neither
step includes refresh, preview or deployment, and neither was executed here.

## Minimal DNS/TCP/TLS proposal — not executed

Purpose: separate name-resolution, TCP establishment and TLS failures for the
**exact current authentication hostname** without sending HTTP or credentials.
A small reviewed executor would be required before any authorization to run;
this packet does not implement or execute a probe command.

| Stage           | Exact destination/scope                                                                                              | Maximum controlled work                                                                                                | Expected evidence                                                                                                                                |
| --------------- | -------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| Local preflight | Same host execution context as the failed launcher; snapshot own namespace, resolver, routes/rules and proxy state   | No external request; stop if context/destination differs                                                               | Measured context ID rather than inference from tool permissions                                                                                  |
| DNS             | Configured127.0.0.53 UDP port53; absolute signin.us-east-2.amazonaws.com name, one A and one AAAA query              | Two client DNS sends total,2seconds each, no retry, search suffix, alternate resolver, TCP fallback or alias follow-up | Response code/timeout, address-family availability and bounded public addresses retained privately; stop on malformed/truncated/no usable answer |
| Route selection | Local route/policy lookup for at most one returned public IPv4 and one public IPv6 address                           | No packet sent; reject private/loopback/link-local answers and stop for review                                         | Selected route/interface class for these exact current addresses                                                                                 |
| TCP             | At most one selected IPv4 address, and only after its connection fails, one selected IPv6 address, each port443      | Two socket connect attempts maximum,10seconds each; no Happy Eyeballs, alternative IPs, reconnect or retry             | Success/latency or bounded errno/category; identifies family-specific failure without changing routes                                            |
| TLS             | First successful TCP socket only, server_name=signin.us-east-2.amazonaws.com, same verified CA anchors as pinned SDK | One TLS handshake maximum on that same connection,5seconds; stop after its success/failure                             | TLS success/hostname verification or allowlisted SSL/X509 error; no raw certificate or exception text                                            |

If only IPv6 is available, try its single selected address once. If the first
IPv4 TCP connection succeeds, do not open an IPv6 connection. After any TLS
attempt, stop; no other family/address is tried. No HTTP request, OAuth path,
SDK credential chain, session cache, client certificate or token is used. TLS
success would prove only current reachability/trust for one address/context,
not authentication success, earlier delivery or readiness to resume preview.

Use one **45-second total deadline**, beginning before the first DNS send and
including local preparation; enforce it across stages without reset. Stop on
unexpected destination, malformed response, persistence failure or exhausted
allowance. Emit only sanitized phase/category/code/timing/verification flags;
keep DNS answers and routing details private. Do not disable existing kernel TCP
behavior or protections; packet retransmissions are not new socket attempts.

Counting must be explicit before execution: durably record each of at most two
DNS client sends and each of at most two socket connect attempts before sending.
TLS shares the successful TCP connection and has its own one-handshake counter.
There are **zero AWS HTTP/API/authentication requests**. Conservatively charge
each AWS-destination TCP connect attempt, including failure, against the existing
500 ceiling: ordinals168 and at most169, consuming at most2 of333 headroom.
Actual cumulative usage stays167 until probes are separately authorized and run.
Keep these contacts in a separate diagnostic ledger, never append to or resume
either stopped authentication ledger/claim. Same cumulative $1 allowance; no
billing measurement or extra dollar balance is implied.

DNS bounds above apply to the controlled client's two datagrams. The existing
system resolver can perform upstream recursion/retries or unrelated background
work; these are not counted as client sends and **complete host-wide DNS packet
metering is not claimed**. Authorization must explicitly accept that distinction;
if complete host-wide DNS emission limits are required, stop before querying
and first review a resolver-metered alternative that preserves existing DNS/VPN
protections. No unreviewed public resolver or forced transport bypass is allowed.

A positive result must not trigger authentication/preview automatically. A failed
probe would support a later exact-scope recovery proposal, not permission to
change settings. Destination/address selection and DNS recursion limitations
are review decisions; historical addresses were not retained and a current
address cannot recreate the old flow exactly.

## Accounting, verification and holds

The167 baseline now freezes both original authentication ledgers/windows/terminal
usage receipts, both failed claims and both public projections, in addition to
the inventory135 and provider30 ledgers. Tampering with any frozen proof fails
closed. Old window-from-165 and window-from-166 claims remain intact. No live
window-from-167 claim exists. The old stopped ledgers are not resumed and their
uncertainty is not converted into success. A later authorized authentication
window would begin at168 and use its own reviewed claim; none is authorized here.

Offline accounting, cumulative exhaustion, handoff and process tests passed:
37 unit tests and15 transport scenarios, including rejection of every changed
frozen proof. Tests run
against fake transports/local origins inside network namespaces with no external
route. Repository format/lint/tests/build/Go checks passed via `npm run verify`
(exit0), with dependency downloads disabled. [Machine evidence and the proposed
plan](project-environments-network-context.json) and the
[new checksum manifest](project-environments-network-context.sha256) record final
results. Historical
result/ledger manifests remain unchanged; only newly reviewed accounting/tests
and this packet have new tooling hashes.

Release source remains clean at
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`. Release packages, frontend/native
clients, rollback bytes and policy remain unchanged; no packaging or resource
diff was generated. Policy SHA-256 remains
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`;
release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
Deployment, stage closure, migration, activation, identity/project creation,
live target requests, refresh, preview, external probes, network/cache repair
and policy changes remain held. OpenAPI production scope remains separate.
