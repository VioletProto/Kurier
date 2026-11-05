# Local endpoint-stop investigation and diagnostics

Continuation of [PR #18](https://github.com/VioletProto/Kurier/pull/18) after
[result98e16fa](project-environments-preview-result.md). **Local work only: zero
AWS calls, no live preview, policy widening, credential refresh or repair.**
The original135-call inventory and30-call preview ledgers remain immutable;
real cumulative usage stays **165/500**. Release source remains
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

## Attribution: evidence and limits

The original private event log, SST log, empty stdout/stderr and frozen ledger
were inspected locally. Raw logs/state remain private. A private projection of
26 diagnostic contexts contained10 distinct AWS hostnames; it was reviewed before
selecting the facts below. No raw messages, resource inputs/outputs, credentials,
URLs, paths, queries, headers or bodies are published here.

**Recorded facts:** `endpoint-held` latched at count165; all30 forwarded read
attempts have outcomes. The rejected hostname, protocol/port and hook phase were
not recorded by the old gate. There is no rejected-request identifier linking a
provider message to that stop. The log has no final preview summary.

**Deduction from the exact reviewed guard source:** both paths that can produce
`endpoint-held` first require port443. The CONNECT hook checks port443 before
calling `service(host)`; the request hook requires HTTPS/443 before `classify()`
calls that same function. Therefore443 follows from the reviewed control flow,
not a retained packet measurement. The hook phase, method, HTTP version and
hostname remain unknown. A CONNECT-phase denial does not establish a completed
TLS handshake.

**Source observations, not attribution:**

- The installed SST4.17.1 `aws/function.ts` uses `lambda.amazonaws.com` as an
  IAM role service principal (lines2281/2290). Its appearance in serialization
  diagnostics does not establish an outbound request to that hostname.
- The application API hostname appears in public frontend/resource configuration.
  The local frontend build helper builds files; that configuration value is not
  evidence that it sent a live application request.
- Installed platform helpers can construct us-east-1 providers for conditional
  certificate/WAF/edge paths. The retained provider declaration/state examined
  here contains only `default_7_40_0`, regionus-east-2. These facts do not establish
  that a regional STS endpoint caused the stop.
- The cached botocore CloudFront KV endpoint model uses an account-prefixed
  `cloudfront-kvs.global.api.aws` template, which the unchanged policy already
  recognizes. The installed provider binary contains that suffix and identifies
  cloudfront SDKv1.67.0 / KV SDKv1.15.0. This does not support adding a guessed
  `cloudfront-keyvaluestore.global.api.aws` alias as a historical fix.

The exact Pulumi AWS7.40.0 Go source was not located in the inspected caches.
Its binary SHA/build metadata were inspected without executing provider requests.
The adjacent cached SST3.19.3 Go KV source was also inspected, but is explicitly
not asserted identical to the reviewed4.17.1 binary. Installed4.17.1 TypeScript
source, adjacent Go source, binary constants and a different SDK's endpoint model
have different evidentiary strength. Their hashes/version limits are retained in
[the machine investigation](project-environments-endpoint-investigation.json).

**Inferred causes remain unconfirmed.** A regional/global alias mismatch or SDK/
provider default is possible; configuration hostnames alone cannot identify it.
No particular rejected endpoint, safe read or mutation is established. The new
synthetic STS denial tests below are chosen fixtures, not a replay or attribution
of the historical stop.

## Bounded diagnostic implementation

[diagnostics.py](../../scripts/preview-meter/diagnostics.py) accepts only parsed
host, port, scheme, HTTP version, method and hook phase. It cannot receive a URL,
path, query, header collection or body. A first denial records one private
`denied-endpoint` receipt **before** the stop, with count, fixed reason and:

- a lowercase hostname from a reviewed static display catalogue, or the fixed
  `<redacted-host>` marker; raw host input is limited to253 ASCII characters;
- known/redacted status and a trailing-dot flag, so normalization does not hide
  a possible reason a literal hostname failed the existing policy;
- an integer port1–65535, or null;
- fixed enums for scheme, HTTP version, method and `connect|request` phase.

The display catalogue is independent of authorization. It includes conventional
provider endpoint spellings useful for explaining a denial, while every such
request still passes through the **unchanged** `service()`/`classify()` policy.
Unknown names, arbitrary credential-like DNS labels, userinfo, malformed URLs,
Unicode, injected control characters and overlong input are redacted rather than
truncated or hashed. Even syntactically valid DNS labels can encode secrets;
LDH validation alone is not treated as safe disclosure. Unsupported scalar/
protocol values become fixed markers/null, never their original text.

The proxy adds metadata only for the first denial; subsequent latched requests
cannot flood the ledger. Persistence failure still kills the proxy before
forwarding. No raw diagnostic dump or new flow recording was added. Ledgers stay
private under the existing protected run directory and require review before
publication. Raw retained historical diagnostics were not copied into the packet.
The four published synthetic receipts were checked against an exact key/enum/
hostname set and scanned for the canaries used in userinfo, paths, queries,
headers, bodies and credential-like host labels.

## Preserve the cumulative baseline

The old launcher described only its135-call inventory parent. The new
[accounting helper](../../scripts/preview-meter/accounting.py) verifies **both**
immutable ledger hashes and exact sequences1–135 /136–165 before reporting the
baseline or reading any credentials. Production proxy configuration explicitly
supplies prior165, maximum500 and the stopped-preview parent hash; a new ledger
would start at166. The ledger also records the two-parent accounting chain.
`check` now reports165 prior /335 arithmetic headroom, with zero AWS calls.

This changes accounting provenance, not endpoint permissions. The low-level
policy file is byte-for-byte unchanged:
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
The helper never appends to or rewrites either original ledger. Mutated copies
are rejected in tests. No new window, restart or permission to consume335 calls
is created by this update. The same cumulative $1 procedural allowance remains;
spend and unused balance remain unmeasured.

## Offline verification

```sh
bash scripts/preview-meter/test.sh
python3 -B scripts/preview-meter/run.py check
```

**Eight unit tests and14 transport scenarios passed.** The entire fixture
supervisor/proxy/origin runs in an outer namespace without an external network
route, with fake credentials. Children then use the production sandbox. No live
credential cache was loaded for these tests.

The existing actual SST/Pulumi/Go/Node fixture still shares14 read requests,
including503 retries, pagination and HTTP/2. Its synthetic counters now start
at166. The exhaustion test permits exactly335 fixture requests, reaches500,
and blocks the next. Existing write, concurrent-limit, idle deadline, redirect,
failed-transport and persistence-failure regressions remain passing.

Four new denial scenarios each start a parent, child and grandchild heartbeat,
then provoke a blocked request:

| Fixture                              | Published safe metadata                                             | Enforcement                         |
| ------------------------------------ | ------------------------------------------------------------------- | ----------------------------------- |
| Unsupported regional STS             | `sts.us-east-1.amazonaws.com`,443, CONNECT, HTTP/1.1, connect phase | endpoint-held; zero forwarded calls |
| Credential-like arbitrary host label | `<redacted-host>`,443, CONNECT, HTTP/1.1                            | endpoint-held; secret label absent  |
| Unsupported port on a known host     | `sts.us-east-2.amazonaws.com`,8443, CONNECT                         | port-held; zero forwarded calls     |
| Cleartext HTTP on known host         | `sts.us-east-2.amazonaws.com`,80, POST, HTTP/1.1, request phase     | tls-required; zero forwarded calls  |

Every case leaves count165 unchanged, emits one bounded sanitized diagnostic and
latched stop, and terminates all three running processes. Their heartbeat files
remain stable after launcher return. Each fixture ledger is checked for absence
of URL/path/query/header/body/credential canaries. Unit vectors additionally
exercise malformed/overlong hosts, embedded userinfo, arbitrary valid DNS labels,
invalid port types/ranges and bounded protocol enums. Policy integrity is checked
by its original hash, not merely by selected examples.

[Reviewed machine test receipt](project-environments-endpoint-diagnostics-tests.json)
binds tested source/tool hashes and the safe synthetic receipts. Python format/
lint, repository `npm run verify`, local links, original ledger sequences and
artifact/checksum preservation checks passed. These are local fixture results,
not a new AWS trace or completed deployment preview.

## Preserved bytes and review decision

The original private event log remains
`659f0582ac75156867c3f1acd65e33fa3ea78e3b4b5645c1fed1c76a35ff9258`;
the original private and published preview ledger both remain
`e65635f9b3e21f406cb70dcd0ba7ae71f952e203e54d30f6474da591abe525eb`.
The inventory ledger remains
`1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171`.
Existing prepared packages/bootstraps, clients, frontend and retrieved rollback
bytes were verified unchanged before/after fixtures. No production packaging was
run. Release-manifest SHA remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
Historical machine receipts/checksum files are preserved; the new diagnostics
receipt/checksums describe this new tooling version.

**Exact proposed endpoint-policy amendment: none.** There is insufficient
historical attribution to justify a new host or operation. The static display
catalogue is solely a privacy control and is not a proposed allowlist amendment.
Any future endpoint amendment must identify the exact host/protocol/port and
read operation with evidence and retain all write denials; it requires separate
review and is not implemented here.

Review this investigation and diagnostic/accounting changes. No preview rerun,
credential refresh, policy widening, repair or new live window is authorized by
publication. Deployment, stage closure, migration, activation, identity/project
creation and live target requests remain held. OpenAPI production scope remains
separate.
