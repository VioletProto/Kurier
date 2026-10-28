# Project environments implementation and local verification

## Authentication endpoint alignment — offline verified, live work held

The [amendment](project-environments-auth-endpoint-alignment.md) lets the pinned
SDK resolve CreateOAuth2Token and accepts only the prepared token URL at
https://us-east-2.signin.aws.amazon.com/v1/token before admission/forwarding.
Offline tests confirm DPoP URI alignment and rejection of the old/wrong hosts.
The current authentication/preview baseline verifies **168/500**; historical
ledgers/claims, release/rollback bytes and the byte-identical preview policy
remain preserved. **63 unit tests and 15 process/transport scenarios**, local
preflight and repository verification passed without external networking.
Implementation `4a0faafc673e97d9faae3f26906a327a8bd06e71` and an exact proposed
execution command are published for review. No refresh, probe or preview ran;
recovery and historical causation remain unverified. All live-work holds remain.

## Offline operation-endpoint comparison — no live work

The [comparison](project-environments-auth-path-comparison.md) confirms that
installed default SDK/CLI CreateOAuth2Token selects
us-east-2.signin.aws.amazon.com, while the unchanged helper forces
signin.us-east-2.amazonaws.com. Identical client base hostnames had masked this
operation-level divergence. Region/current proxy/TLS settings match; timeout and
retry configuration differ. This is not proof of historical causation. No
endpoints, cache, policy or networking changed; no network requests/probes/refresh ran.
Usage 168/500 and every historical proof/claim and release/rollback byte remain
preserved. The smallest next action is review of an authentication-only
operation-endpoint alignment proposal; no live continuation is authorized.

## Authorized DNS/TCP diagnostic stopped — cumulative168

The [once-only diagnostic result](project-environments-network-probe-result.md)
records two configured-resolver queries, one returned public IPv4 address, no
AAAA address, and one IPv4 TCP connect timeout. TLS0, HTTP/API/authentication0;
no refresh or preview. Actual context was measured; underlying cause and both
prior remote refresh outcomes remain uncertain. One charged connection brings
cumulative usage to168/500. New/old claims and every ledger remain preserved.
Offline59 unit/15 transport checks passed before execution. No repair, settings
change, retry or continuation; release/rollback bytes and all holds remain.

## Local network-context comparison and167 accounting — review only

The [context comparison and probe proposal](project-environments-network-context.md)
compare retained successful/failed invocation history with local namespaces,
routes, resolver/proxy, firewall/VPN configuration and journal evidence. Earlier
Wi-Fi/DHCP events do not establish the connect-timeout cause; historical actual
network snapshots and live firewall visibility are incomplete. Both refresh
outcomes remain uncertain. Accounting verifies167/500 and preserves all original
proofs/claims. No authentication, preview, external probe or repair ran. The
bounded DNS/TCP/TLS plan is proposed only; release bytes and all holds remain.

## Authorized review-4 stopped — typed connect-timeout

The [review-4 result](project-environments-refresh-preview-review-4.md) records
one execution of7e6e504 with a fresh166-based window. One refresh attempt at167
recorded connect-timeout/connect, no numeric code or HTTP status. No identity
verification or provider work followed; no diff or new packaging. Cumulative
167/500, authentication1/provider0. Underlying cause and remote refresh outcome
remain unverified; prior uncertainty is preserved. Both failed runs/claims, all
ledgers, tooling/policy, release and rollback bytes remain intact. No repeat,
repair, widening or restart. Same cumulative $1 allowance,333 arithmetic calls
remaining; all live-work holds remain.

## Local authentication diagnostics and166 accounting — review only

The [local investigation](project-environments-auth-transport-investigation.md)
finds no retained subtype establishing the historical transport cause; remote
refresh outcome remains uncertain. Typed, bounded category/phase/numeric-code
diagnostics and offline regressions are added without messages or request data.
Accounting now verifies166/500 against all frozen proofs, preserving the failed
165 campaign claim. No network probe, AWS request, live refresh/preview or repair
ran. Source, policy and release/rollback bytes remain unchanged. Proposed recovery
requires separate review and authorization; all live-work holds remain.

## Authorized refresh-preview stopped — 2026-10-28 MDT

The [live refresh-preview result](project-environments-refresh-preview-result.md)
records the one authorized9cf8d74 invocation. One refresh attempt ended in
transport-failure with no HTTP status; remote outcome and cause are uncertain.
No identity verification or provider startup followed. New authentication1,
provider0; cumulative166/500. No diff or regenerated packaging. Original ledgers,
new private receipts, reviewed policy/tooling, release and rollback bytes remain
intact. No repeated refresh, repair, widening or restart. The same cumulative $1
allowance and all live-work holds remain;334 calls are arithmetic headroom only.

## Shared-budget handoff implemented — local review only

The [amended tooling packet](project-environments-shared-budget.md) implements
a single-use authentication ledger and original-deadline handoff to the provider
meter. The read-only policy is byte-identical. Offline ledger, exhaustion,
deadline, failed-authentication and reuse/reset tests passed; no AWS calls,
refresh or live preview occurred. Prior165/ceiling500 and the cumulative $1
allowance remain. Source, original ledgers, release and rollback bytes are
preserved. Execution awaits review of the amended commit and exact PR #18
command; all live-work holds remain.

## Refresh authorization preflight — shared-budget gap

The [local handoff preflight](project-environments-refresh-handoff-preflight.md)
confirmed unchanged reviewed0d1defa tooling cannot carry refresh calls and the
original deadline into the provider meter. Its non-fixture launcher hardcodes165
prior calls and a new3,600-second clock. The offline probe made zero AWS calls;
no refresh or preview started. Cumulative165/500, original ledgers, policy,
release and rollback bytes remain unchanged. A concrete handoff amendment is
proposed for review, not implemented; executing it requires resolving the
identical-tooling/no-repairs constraint. The cumulative $1 allowance and all
live-work holds remain.

## Authorized preview invocation refused — 2026-10-27 MDT

The [credential-stop result](project-environments-preview-credential-stop.md)
records one invocation of reviewed tooling0d1defa with a fresh review-2 output
path. Expired cached credentials caused the reviewed local guard to refuse
before proxy/SST startup. Zero new AWS attempts; cumulative165/500, original
ledgers and artifact/rollback bytes unchanged. No diff or new packaging exists.
No refresh, repair, policy change or restart. The same cumulative $1 allowance
and all deployment/closure/migration/activation/identity/project/runtime holds
remain; unused335 calls do not authorize another invocation.

## Local endpoint diagnostics follow-up — 2026-10-27 MDT

The [local investigation and diagnostic packet](project-environments-endpoint-investigation.md)
keeps historical stop attribution unestablished, distinguishes source deductions
from evidence, and adds bounded private hostname/protocol/port receipts. Eight
unit tests and14 offline transport scenarios verify denial, redaction and
parent/child/grandchild termination. The endpoint policy bytes are unchanged;
no policy amendment is proposed. Launcher accounting now verifies both frozen
ledgers and carries165/500 usage forward. Original ledgers, release and rollback
bytes are preserved. No AWS calls, preview rerun or live work; all holds remain.

## Authorized preview stopped — 2026-10-27 MDT

The [actual preview result](project-environments-preview-result.md) records the
single invocation authorized at tooling `843428a3875e6d5ac3544420c392731d6a3991ee`:
the unchanged meter stopped on `endpoint-held` after30 new AWS reads, retaining
135 prior attempts for165 cumulative. The rejected hostname/operation was not
retained, so attribution is unestablished. The partial event log has eight new
route-component registrations but no completed AWS diff/summary. Frontend hashes
match; no new Lambda ZIPs were produced; prepared and rollback bytes remain
intact. No policy widening, refresh, repair or restart. The window is stopped;
335 unused calls are arithmetic, not authority to continue. Deployment, closure,
migration, activation, identity/project creation and runtime holds remain.

## Provider-process meter preparation — 2026-10-27

The [preview meter packet](project-environments-preview-meter.md) implements and
locally verifies shared counting/time limits across isolated SST/Pulumi/Go/Node
processes. Four unit tests and ten transport scenarios cover retries/pages,
HTTP/2, concurrent limits, bypass denial, unexpected writes, redirects, failed
transports, persistence failure and descendant shutdown. The proposed new window
uses only the remaining365 calls, retaining135 prior calls and the cumulative
$1 allowance. No AWS calls or live preview ran. Release source, package/static
hashes and rollback bytes are unchanged. Preview and all deployment/runtime
holds remain; this is tooling preparation for review.

## Release preparation — 2026-10-27

Subsequent [authorized inventory and artifact preparation](project-environments-inventory.md)
completed with 135/500 physical attempts, preserved source pins, current
capability/accounting/IAM/resource readbacks and verified rollback retrieval.
Prepared release ZIPs/static manifest are retained locally. Stopped before SST
preview because provider-process metering is not established. Only Davian's
separately authorized existing-session refresh wrote authentication state;
no application/infrastructure changes or live target requests occurred.
All deployment/closure/migration/activation/identity/project/runtime holds remain.

PRs #15 and #16 are merged. The proposed
[deployment review packet](project-environments-deployment-review.md) pins release
source `44f1fb79d943501c95e06fcc1a6114111a7c1ea4` (PR #16 merge, including
implementation head `82fa7d760a485934bde3259f7652ab1c434ea85f`). It records
source-derived deployment/IAM expectations, compatibility, closed-stage activation,
no required backfill, post-data rollback limits and a bounded runtime/cleanup plan.
Fresh repository verification, uncached runtime/agent race tests and clean ARM64
builds passed locally. This preparation did not repeat browser/database integration
or establish live AWS state. AWS inventory/preview, deployment, migration,
activation, identity creation and live target requests remain held. The packet is
for review; the environment tracking card remains open. OpenAPI production scope
remains separate.

## Implementation record

Davian accepted the [review packet](../architecture/drafts/project-environments-review.md)
and [compatibility amendment](../architecture/drafts/project-environments-compatibility.md)
at `d8be596a2306f2e888ef34e81dfa836bc4b84447` on 2026-10-24. The implementation
below was verified on the feature branch and subsequently merged in PR #16.
Merge does not activate a stage or establish deployed runtime evidence.

AWS inventory/preview, deployment, migration and activation remain held for
separate review. No AWS calls, accounts, identities or resources were used for
this implementation campaign. OpenAPI production import/validation remains
unaccepted. Evidence **3** belongs to environments; OpenAPI has no allocated
production evidence version. Library acceptance is unchanged.

## Delivered behavior

Owner environment POST/list/detail/PATCH/DELETE routes use existing Cognito
owner/project/user/stage fences, no-store, revision ETags and quoted If-Match.
Variables replace as a collection, retain stable server IDs and immutable
name/type/protection, and have exact primitive/tagged-decimal DTOs. Protected
writes are encrypted before guarded cross-table publication; reads return fixed
masks and reusable saved-secret refs. Typed saved-secret metadata is trusted,
immutable and included in source conditions; PATCH keeps its value:string
transport and rejects cross-type values. Deleting an environment detaches refs;
explicit secret revocation and project deletion retain their existing meaning.

The browser editor keeps numbers in text state, clears protected drafts before
writes and after failed validation, supports typed reuse/preserve/set, and
requires reconciliation of uncertain writes. Environment summaries and selectors
contain metadata only. Requests opt in explicitly; legacy definitions remain
literal. Saved opt-in cannot be removed through PATCH. Standalone selection,
workflow run defaults and step inherit/none/select follow the accepted precedence.
No process environment, global defaults, expression evaluation or implicit output
bindings are introduced.

The shared resolver supports public URL base/whole path components, ordered
query/header values and primitive JSON string-value slots or text bodies. It
bounds references, distinct variables, JSON traversal and rendered output;
rejects missing/disabled values, unsafe URL/header structures, nonstring mixed
interpolation, credential declassification and overlapping direct/workflow
bindings. It resolves once into transient copies and adds protected rendered
variants to the redaction corpus. Workflow output bindings apply afterward and
cannot become another template program.

Admission freezes original templates, only used public typed values, exact
protected refs, variable IDs/type mappings and effective environment identity.
Guarded source conditions join the existing receipt/quota/input/outbox transaction;
private versioned HMAC domains distinguish environment admissions. Receipt lookup
precedes newer environment revision checks. Ordinary edits do not change admitted
values; dispatch/advancement check current membership/enablement/type and exact
source eligibility using current observed revisions. Reruns retain frozen public
values and select current values at the exact frozen secret IDs. Removed IDs,
deleted environments and revoked secrets cannot be rebound by name.

Cloud and local standalone requests and cloud-only workflows share this resolver.
Local Work/Result **2** require an exact credential-bound capability declaration
before admission. Legacy session DTOs and Work/Result **1** remain unchanged for
legacy work. Grants and results carry matching safe identity, included in RFC 8785
canonicalization and retained result receipts; verification independently rebuilds
and sanitizes capture. Retried accepted uploads work after input cleanup.

Owner evidence **3** contains template semantics and environment ID/revision/null,
with optional existing cloud-workflow assertions. Existing evidence 1/2 bytes
and owner read behavior are preserved. MCP retains seven tools and the existing
`view:"complete"`/`capture` envelope, projects through the dedicated legacy DTO,
and explicitly lists omitted environment/assertion fields. Projection never
rewrites stored evidence, checksums or pins. Cleanup recognizes the new exact
row shapes and typed secret records, uses bounded resumable cursors and preserves
unknown records rather than deleting them.

## Capability boundary

The new complete tuple is savedRequests9, projectEnvironments1, protectedSecrets2,
localAgentExecutions2, with cloud1/inputs1/pinning1/workflows1/MCP1/assertions1.
Readers and transaction conditions recognize it; application startup does not
write it. Only isolated fixtures set the tuple during these checks. Gateway route declarations include the six new API operations and explicit
environment collection/item preflights for a later reviewed deployment. No IAM
expansion, migration, activation command, deployed-state inventory or permissive partial tuple was
introduced. Reviewing deployment and an actual closed-stage transition is separate.

## Verification and practical limits

The decoded-JSON correction (2026-10-26) removes a raw-byte delimiter shortcut:
JSON string values are inspected after escape decoding. Bodies with no
replacements retain their exact original bytes; replaced bodies retain untouched
numeric tokens and insert variable values once. Regressions cover Unicode and
mixed delimiters, root/array values, inert inserted markers, protected-value
redaction, malformed input and traversal/reference/output bounds. A controlled
HTTP execution checks the actual outgoing bytes; API freeze/rerun fixtures also
use an escaped template. Review of discovery, authoring validation, workflow
overlap checks and cloud/local execution found those paths already use decoded
string values through the shared resolver; no protocol allocation changes are
needed.

The verification campaign covers:

- full repository formatting, lint, unit/tooling/infra tests, web build and Go
  tests/vet/test-binary builds through `npm run verify`;
- uncached race tests for the shared runtime and compiled local-agent client;
- real DynamoDB Local transactions with simulated KMS, memory-only object/queue
  adapters and controlled HTTP fixtures: owner CRUD/foreign access, exact values,
  strict unions, preconditions, current-secret PATCH, frozen admissions/receipts,
  revocation, local negotiation/result identity/receipt retry, workflow assertions,
  MCP projection and normal deletion/cleanup;
- Chromium against the Go API and DynamoDB Local, using fixture JWT/JWKS and
  Cognito responses, plus a compiled foreground CLI against controlled local TLS
  and HTTP: author/reopen exact decimals, masked protected values, Work 2
  negotiation before admission, evidence 3 and frozen-public/current-secret rerun;
- the existing ownership/publication/race/retention/MCP and browser regression
  suites, including legacy execution and assertion behavior.

The browser fixture clock now advances between deliberate retention jumps so
agent rate-limit windows behave normally; the fixture permits bounded CLI backoff.
New browser resources are cleaned after failures as well as successful runs.
Historical research remains under spikes/environment-resolution; legacy wire
experiments now use explicit frozen pre-environment field sets, rather than
mistakenly treating upgraded production readers as old readers.

These are local/CI results, not genuine AWS KMS/S3/SQS/Cognito/IAM, deployed DNS/
network, email, migration, activation, elapsed retention/PITR, performance, billing
or public signup evidence. Review of this implementation does not authorize those
campaigns. Exact published head and CI outcome belong in the implementation PR
and Trello, avoiding a self-referential commit hash in this file.
