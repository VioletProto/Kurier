# Local review-5 CONNECT investigation and private-capture amendment

**The exact denied destination remains unattributed.** Local inspection of the
retained provider logs, generated configuration and installed source did not
recover a target linked to the rejection. No hostname is guessed or proposed
for authorization. No AWS call, external probe, refresh or preview ran.
Actual usage remains **200/500**; release source remains
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

## Confirmed evidence and limits

The [review-5 result](project-environments-refresh-preview-review-5.md) and its
original private receipts were verified unchanged before inspection. The public
record remains endpoint-held at count 200, CONNECT, HTTP/1.1, port 443, connect
phase, hostname `<redacted-host>`. Its hostname disclosure catalogue was not
expanded. The record does not contain a rejected-request correlation ID or raw
hostname.

| Local evidence inspected                    | Confirmed observation                                                                                                      | Attribution limit                                                                |
| ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| Private stdout, proxy stderr, Pulumi stderr | All empty                                                                                                                  | No destination can be recovered there                                            |
| Pulumi process log                          | 3,433 bytes; no CONNECT/proxyconnect/endpoint-held string                                                                  | Not a wire trace                                                                 |
| Private Pulumi event stream                 | 907,184 bytes, 1,101 events, 816 diagnostic events; no CONNECT/proxyconnect/endpoint-held string                           | Contains resource/configuration data, not a linked rejected CONNECT              |
| Scoped hostname scan                        | 11 distinct candidates matching the inspected AWS/provider-domain patterns; all already in the static disclosure catalogue | The scan is not an exhaustive network trace, and none is tied to the denial      |
| Event timing                                | Last event timestamp 22:37:16 UTC; denied receipt 22:37:23.202198 UTC                                                      | Event timestamps have second precision; ordering does not identify a destination |
| Generated host meter configuration          | Only mode, ledger, handoff and accounting keys; no private-capture option                                                  | No hidden raw-host receipt was enabled in the historical run                     |
| Retained AWS provider declarations          | Region us-east-2; no explicit endpoints override; skipCredentialsValidation=false, skipRegionValidation=true               | Region/configuration alone cannot identify a service's resolved operation host   |

Candidate names and potentially sensitive contexts were inspected locally and
kept in a private ignored directory. They are **not published, selected as the
denied host, or hashed into the public findings packet**. Resource URLs and IAM
policy principals do not establish outbound connections. No absence-of-keyword
or regular-expression scan is treated as proof that a destination cannot appear
elsewhere in retained data; the affirmative attribution link is missing.

Installed SST 4.17.1 provider-helper TypeScript and the generated Pulumi AWS
7.40.0 JavaScript provider wrapper were inspected. The wrapper passes optional
endpoint settings through; the retained declaration has none. The installed
provider binary matches the reviewed SHA-256
`741bfe58a4f6100dfa839beb98d53e0f27c9b63998d53a293cbbfb7a6b23a797`.
Read-only Go build metadata identifies the provider implementation and its SDK
dependencies. The apparent Terraform dependency is replaced by `../upstream`
marked devel; it must not be treated as the unmodified published dependency.
The exact matching Go implementation tree was not located in the installed
plugin or inspected module caches. No source download was attempted. Generated
bindings, installed SST helpers and binary build metadata have different
strength from an observed transport destination.

Source review confirms that the historical CONNECT hook rejected the host before
forwarding. The static public sanitizer then discarded unrecognized host text.
The relay is a byte splice and does not retain connection authorities. Nothing
in these inspected paths reconstructs the missing parsed hostname. **No exact
endpoint-policy amendment is supported: none is proposed.**

## Minimal opt-in private receipt — implemented for review only

[private_diagnostics.py](../../scripts/preview-meter/private_diagnostics.py)
adds a separate, default-off receipt. The launcher exposes
`--private-denied-connect`; its host configuration sets
`privateDeniedConnectCapture:true` only when explicitly selected. This is a
proposed capture control, **not authorization or a command to run preview**.

The [gate](../../scripts/preview-meter/proxy.py) retains the existing public
sanitizer and ledger fields. Only the first connect-phase endpoint-held denial
can additionally write the fixed file `denied-connect.private.json` beside its
private provider ledger. That directory is ignored by Git, outside every child
mount and not included in publication manifests. No arbitrary output-path option
or hostname publication function was added.

The private receipt contains exactly version, phase, cumulative count, port,
hostnameStatus and hostname:

- Hostname input is **only the parsed host handed to the authorization check**,
  capped at 253 ASCII characters. Valid DNS spelling, case and trailing dot are
  preserved exactly; valid unscoped IPv6 literals are also accepted. This is not
  a claim to preserve the raw CONNECT authority's original wire spelling.
- The whole file is capped at **512 bytes**, one exclusive creation, mode 0600,
  inside an owner-matched mode-0700 directory. Symlinks for the receipt directory itself and final file
  are refused; short writes are completed and file/directory fsyncs are required.
  Existing or partial receipts are never overwritten, appended or retried.
- The interface cannot receive URLs, paths, queries, headers or bodies. Userinfo,
  URL-like delimiters, control characters, Unicode, scoped literals and malformed
  input yield only a fixed status and null hostname. Overlong input is marked
  overlong, never truncated. Protocol phase is fixed; port/count are bounded
  integers.
- **Valid DNS labels can contain secrets.** A captured hostname is sensitive even
  if it resembles an AWS endpoint. The receipt must remain private and be
  inspected locally. Require a separate disclosure review before publishing any
  captured hostname or file/hash; do not attach, cat, auto-export or include it
  in a public ledger/PR. This task publishes no denied hostname.

The public denial is recorded with its existing redaction. Capture does not
admit or forward the CONNECT or increment AWS attempts. The ordinary terminal
hold still stops descendants. A capture persistence failure or unexpected
exception kills the flow and exits the meter; the supervisor stops descendants.
Such failure may leave a partial private file and a denial without a terminal
stop receipt, requiring inspection, not a retry or permission to forward.
Disabled capture creates no receipt. Port-held and request-phase denials do not
produce this host-only endpoint-held receipt.

## Carry-forward accounting and deadline validation

[Accounting](../../scripts/preview-meter/accounting.py) now verifies the immutable
review-5 originals and public projections/report before reporting **200 prior,
500 maximum, 300 arithmetic headroom**. The historical 167 and 168 proof chains
remain available and unmodified; 38 frozen files are checked. The consumed
window-from-168 and provider claims remain required. A different output path or
old baseline cannot reset usage. No window-from-200 claim or live window was
created. The same cumulative **$1 procedural allowance** remains; billing and
unused dollar balance are unmeasured.

Offline testing also reproduced a pre-existing floating-point edge in
[handoff.py](../../scripts/preview-meter/handoff.py): subtracting a rounded
deadline from a fractional start can differ from 3,600 by a few trillionths of a
second at a binary boundary, incorrectly rejecting a window created by begin.
The one-line amendment compares `deadline == started + SECONDS`, the exact
creation expression. It introduces no tolerance, new start, time extension or
reset. The regression accepts the original persisted deadline at that boundary
and rejects even a one-microsecond alteration. No historical deadline or receipt
was rewritten; no live failure was repaired or replayed.

Authentication endpoint, TLS verification, region, no-proxy/one-attempt controls,
existing-session restrictions, STS gate, SDK pins and read-only preview policy
remain unchanged. The public sanitizer is byte-identical. Deployment/runtime
holds remain in force.

## Offline verification and next decision

The [machine packet](project-environments-denied-connect-review-5.json) and
[checksum manifest](project-environments-denied-connect-review-5.sha256) bind
reviewed findings, source hashes, test evidence and preservation checks. Synthetic
fixtures exercise private-only exact-host capture, unchanged public redaction,
no forwarding/charge and parent/child/grandchild termination. Exclusive-file
collision and unexpected capture exceptions fail closed. Unit coverage includes
size bounds, invalid URL material, modes/symlinks, short writes, fsync failure,
reuse rejection, frozen accounting and original-deadline continuity. Fixture
hostnames are invented tests, never historical attribution.

All verification uses a namespace with external networking disabled; repository
downloads are disabled. No live authentication or external endpoint probe ran. Comparison checks read
existing local configuration/cache metadata without writing or refreshing it. The final verified totals are **72 unit tests and 16 process/transport
scenarios**; local-only preflight reports 200 prior, 300 remaining and zero AWS
calls. Full repository format/lint/test/build/Go verification and Python
lint/format passed. Results are recorded in the machine packet. Prepared/rollback bytes and every original
ledger/claim remain intact. No release packaging changed and no new artifact
hash requires review.

**Stopped for review.** The next decision is whether to accept this default-off
private capture and 200-based accounting amendment. Any later execution needs a
separately reviewed exact tooling commit, fresh bounded window and explicit
scope; previous consumed commands must not be rerun. A captured hostname must
then undergo private disclosure review before publication or any policy proposal.
No new live command/window or endpoint allowance is authorized here. No AWS
calls, refresh, probes, preview rerun, deployment, closure, migration, activation,
identity/project creation, live targets, network/cache repairs or automatic
continuation occurred. OpenAPI production scope remains separate.
