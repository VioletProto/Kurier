# Local-agent pairing and single saved-request execution: review packet

Status: **Accepted by Davian**, 2026-10-13, at `585d5b3`. Scoped implementation,
API-contract synchronization and local/race/protocol/browser validation are
authorized. Davian subsequently approved scoped development deployment and
capability activation including correction `14323d8`; both are performed.
See [actual deployment, validation and operator checks](../development/local-agent-execution.md).
Acceptance is not evidence that implementation or runtime checks passed.
The preparation/review sections below preserve the original packet history.

Branch: `feat/local-agent-execution`, created from refreshed `origin/main`
`95f3a39` with a clean working tree. That commit merges
[evidence-pinning PR #8](https://github.com/VioletProto/Kurier/pull/8), including
implementation `e6b964f` and validation `6d9c26d`. Pinning is merged, not merely
available on its former feature branch.

Authority: [accepted architecture](README.md), [ADR 0002](../decisions/0002-serverless-persistence-topology.md),
[dispatch/recovery design](w1-system-design.md), [data model](w1-data-model.md),
[saved requests](saved-requests-contract.md), [protected inputs](protected-request-secrets-contract.md),
[cloud execution](cloud-execution-evidence-contract.md) and
[pinning](evidence-pinning-contract.md). The live
[proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
and [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
were read. Their local-agent behavior is accepted at the W1 level, but exact
pairing, scope and result wire details remain Proposed. The proposal's exclusion
of secrets from ordinary polling means execution-only protected delivery must
be an explicit exception, never browser status polling or MCP delivery.

Scope is one saved request, one active agent per account and one unresolved job
per credential. No workflows, extraction/runtime outputs, environments, MCP,
frontend hosting, background-service installers or remote local-control API.
Use existing history/evidence and retention rather than a second evidence store.

## Browser-authorized pairing and identity

Propose manual browser-to-CLI pairing: a signed-in owner selects 1–10 owned active
projects and up to 16 exact destinations per project, names the device, reviews
the capability warning, then explicitly creates a credential. Return a random
256-bit opaque bearer token once over the authenticated HTTPS response. No
device-code OAuth flow or browser call into localhost is needed for this slice.
The Go CLI reads the token through a hidden terminal prompt (or explicit stdin
for testing), validates it against the fixed Kurier API origin, displays safe
scope and requires the operator to confirm the destination list before polling.
Names are display labels, not hardware identity or attestation.

The credential ID is `localAgentId`; replacement gets a new ID. Store only a
SHA-256 token verifier server-side, expiry, owner, approved scopes, version,
recovery generation and revocation state. A random high-entropy token does not
need a password KDF. Never expose verifier/fingerprint. Browser auth cannot use
agent routes and agent credentials cannot use browser CRUD, secret reads,
arbitrary execution submission or evidence reads. The hosted API must verify
the bearer token itself on agent integrations, rather than applying the existing
Cognito JWT authorizer to those routes. Unknown token is fixed 401.

Credential lifetime is 30 days from issuance; no refresh or automatic renewal.
Creation is a conditional account-slot transaction with owned-project gates and
active stage/generation; another live credential returns 409. No silent
replacement. Explicit revocation precedes re-pairing, including when the old
device is offline. Scope is immutable: adding projects/destinations requires
revocation and a new credential. Expiration behaves like revocation for all
poll, upload and ACK authorization. Credential tombstones remain until active
obligations settle; losing the slot does not erase jobs or receipts.

Proposed issuance uses a random UUID `Idempotency-Key`. Persist a private
issuance receipt for seven days, but never the raw token. Identical retry returns
the original safe metadata with `token:null`, never a new credential; lost token
requires explicit revoke/re-pair. Cross-input key reuse is 409. A 503 asks the
browser to refresh credential status before retrying/revoking; it never silently
creates a second credential. Rate limit issuance to five attempts/account/hour.

The minimal CLI defaults to memory-only token storage. Optional `pair --persist`
requires explicit local consent and writes only the agent credential plus safe
API/scope metadata into a private user configuration directory (0700), file
0600, atomic replacement, no symlinks, no shared directory; refuse unsupported
permission enforcement. This deliberately stores a capability credential on
the device, not request inputs. Never put the token in argv, environment examples,
shell history, stdout, telemetry, crash reports or job files. An OS keyring can
be a later enhancement; Windows persistent storage is excluded until an ACL or
keyring implementation is verified. `unpair` attempts server revocation then
removes local storage; offline removal reports server revocation still pending
and directs the owner to the browser. No universal memory-erasure guarantee.

Revocation is an owner-authorized conditional credential-version update and
account-slot clear. It immediately denies subsequent grants, first uploads and
identical ACKs. The version is conditioned in every grant/publication transaction.
Durable maintenance discovers jobs and clears only matching active-job slots;
new pairing cannot acquire the old credential's lease or receipt authority.
Already delivered secrets or HTTP cannot be recalled; show this limitation before
pairing/revocation. Browser sign-out does not revoke the deliberately independent
agent credential; show its expiry and a separate Revoke action.

## Destination permission and target selection

Browser pairing consent grants exact `(scheme, normalized host, port)` origins
and an allowed address class for each project. No wildcard hosts, entire private
CIDRs, port ranges, discovery, VPN scanning or implicit permission to all of
localhost. For example, permission for `http://localhost:3000` is one origin;
`http://127.0.0.1:3000` is a separate entry. Normalize omitted HTTP/S ports to
80/443; explicit local ports may be 1–65535. Reject userinfo, fragments, zone
IDs, malformed/noncanonical numeric IP forms, trailing-dot hosts, controls,
unsupported schemes and Host overrides. Exact origin consent covers every path
on that service; display that fact. Destinations are non-secret metadata, not
URLs with credentials or secret paths/query values.

Allowed classes are `loopback` (127.0.0.0/8 and ::1) or `private` (RFC 1918 IPv4
and fc00::/7 IPv6). Restrict `localhost` to loopback answers. All DNS answers must
belong to the consented class; reject the entire set if any answer is disallowed.
Reject public addresses for local execution in this initial slice, and always
deny link-local/metadata, unspecified, multicast, CGNAT, documentation/benchmark,
transition/tunnel/translation and IPv6 scoped addresses. IPv4-mapped IPv6 must
be unmapped and classified, never bypass checks. Private literal IPv6 ULA may
be allowed; a TLS certificate still must verify the original hostname/address.
No insecure TLS toggle, client-certificate loading, mDNS discovery or Unix sockets.

Before each dial, resolve with a two-second bound, validate all answers and pin
one validated address with ordinary hostname TLS verification. One new connection,
no alternate-address retry, redirects, ambient proxies, cookie jar, netrc or
automatic HTTP replay. The operator's locally confirmed list must intersect the
browser-issued list; a hosted job cannot expand it. HTTP to private or loopback
does not imply consent to send secrets: preserve separate
`allowInsecureSecrets:true` consent for every submission/rerun. Pairing warns
that the agent receives runtime secrets and can affect services on this machine
or network. Submission names the selected device and destination and asks for
explicit local-network consent; no preselected checkbox.

Existing request CRUD already validates HTTP/S URL syntax rather than resolving
public DNS. Preserve its protected-field rules and size/revision semantics;
adapt public-only guidance to explain local eligibility. Keep the cloud runtime's
public-only 80/443 validation unchanged. A syntactically saved private URL must
still fail cloud execution. No target migration or DNS trust decision at save.

Extend submission with optional `target`, default `cloud`, and optional `local`.
For cloud, `local` must be absent; its old body, behavior, response envelopes,
SQS/OUT dispatch and idempotency identities remain unchanged. For local require
`local:{localAgentId,allowPrivateNetwork:true}` and the current credential's exact
project/origin permission. Freeze target, agent ID, credential/scope version and
origin policy with the source revision and existing execution options. No
cloud fallback, agent reassignment or target change after acceptance.

Keep the same owner/project admission-key namespace. Local input HMAC adds the
target, agent and normalized consent/policy identity; a reused cloud key for a
local request must conflict. Preserve legacy cloud digest calculation for old
retries. New local receipts use a versioned identity domain without creating a
second unique-key namespace. Deliberate local rerun preserves the frozen public
request/policy but requires explicit `target:"local"`, currently active
`localAgentId` and fresh local/HTTP-secret consent; it may select a re-paired
credential only with the owner's new submission. A cloud-source rerun remains
cloud, a local-source rerun cannot silently become cloud. Cross-target execution
uses a new saved-request submission. Missing/revoked source refs fail as today.

## Immediate polling and durable intent

Local queued jobs use `DPK=AGENT#credentialId#PROJECT#projectId` and
`DSK=submittedAt#jobId` on existing GSI2, a proposed project-specific refinement
of the accepted agent discovery key. Admission also inserts a recognized
`LOCALWAIT#jobId` due-obligation row at submittedAt+10m under existing maintenance
due shards: one GSI slot on JOB cannot serve both agent discovery and offline
expiry. Grant removes LOCALWAIT and replaces JOB discovery keys with lease-due
keys atomically. Expiry/revocation cleanup conditions queued state and removes
the matching obligation; stale index entries never authorize transitions.
Cloud jobs continue through OUT/SQS. No local SQS notification, and the
worker must explicitly reject local jobs even if an identifier is sent to it.
Strong base reads and conditional transactions confer authority; GSI lag only
affects discovery latency. Queued local work expires ten minutes after admission.
Offline admission is allowed with explicit browser warning; never claim online
status guarantees dispatch.

Agent polls one project at a time, fairly rotating its <=10 scopes. Immediate
return, one poll in flight, nominal ten seconds per project, jitter and bounded
backoff to sixty seconds on network/503/429; honor Retry-After. Typical opportunity
is within one ten-second interval plus API/DNS latency, with no guarantee under
cold starts, throttling or index lag. At most one candidate grant per call and
five bounded discovery pages; empty discovery is safe. Agent presence is
`lastSeenAt` updated at most once/minute; browser displays online if seen within
60 seconds, otherwise offline/unknown, with server time. Presence does not fence
execution. Browser status retains ten-second polling and existing stop rules.

Grant transaction checks active stage/generation, enabled owner, active owned
project/version/deletion epoch, original live credential/version/empty slot,
queued local job/target/credential, current source request and exact frozen secret
eligibility. Set job/anchor to running, increment fence, store random lease ID,
256-bit nonce hash, dispatch intent timestamp, executeNotAfter = intent+30 seconds
and lease deadline = intent+120 seconds; occupy credential activeJob. There is
no client start route. Decrypt the frozen bundle only for this authorized grant,
hold it in memory and return executable work only after an acknowledged durable
commit and a fresh credential/project/generation check.

If commit is uncertain, return fixed 503 without executable work. Readback may
establish the intent for reconciliation, but must never regrant it. Lost poll
responses, truncated responses, failed decryption/disclosure, repeat polls or
process restart cannot clear running intent or return its executable packet.
Busy polls return no work. Server-side pre-intent preparation failure may retry
while queued; any possibly committed intent permanently removes automatic HTTP
eligibility. This sacrifices execution on lost grant to prevent duplicate effects.

Agent validates a complete strict response, credential/job/generation/policy and
deadlines before execution. Use serverTime and monotonic elapsed time measured
from poll send, conservatively subtract the entire round trip and a five-second
margin; if the start window is not provably open, do not send. Limit overall
execution to the smaller of frozen timeout and remaining lease minus ten seconds
for upload. Never persist grants or resume HTTP after restart. Duplicate packets
in the running process are suppressed by job/lease/fence; restart never reloads
an executable packet. No HTTP retry on timeout, crash or ambiguous outcome.

After intent, missing result at lease expiry is fenced and published as
`failed/execution_outcome_unknown`, certainty `unknown`, through the existing safe
capture/ticket protocol. A late result cannot replace it. Pre-intent expiry is
`queued_expired/not_dispatched`. UI retains “Outcome unknown — do not assume no
effect” and offers only a deliberate new submission/rerun with a fresh key.

## Protected delivery and trust boundary

Reuse admission-time strong source reads and fresh KMS job-binding envelopes:
Control contains descriptors/stable refs only, Protected contains ciphertext,
and replacement affects future submissions rather than accepted frozen values.
Exact source revocation/request deletion blocks pre-intent dispatch. Conditions
on observed source revisions prevent a race from substituting a newer value.
Secret revocation after intent cannot recall delivered values or outbound effects.

The execution-only HTTPS grant includes the frozen public plan and a deduplicated
map of enabled frozen secret IDs to values. Bind authorization to owner/project,
job, credential, fence, nonce and generation. No general secret read route or
arbitrary binding lookup. Never return plaintext in browser status/history,
poll-empty responses, errors, Control, SQS, local persistent job files or evidence.
Set no-store; disable request/response body and authorization logging/tracing at
API Gateway, Lambda, agent and test harness. Agent bearer credentials never enter
target headers; target/runtime transport and hosted API transport are separate.
Production hosted API origin is explicit HTTPS, certificate verified, without
redirects or arbitrary job-provided API URLs. Test-only HTTP fixture endpoints
are clearly separate and cannot activate production mode.

Server/agent discard raw buffers and references promptly, with best-effort byte
clearing; Go strings/GC cannot promise physical erasure. No decrypted key cache.
Keep encrypted job bindings until terminal publication or settled deletion
obligations, then remove them. Device compromise is within the paired-device
trust boundary; encryption at rest on the server does not hide executed inputs
from the agent. The server cannot prove honest HTTP execution by an untrusted agent.

## Result upload, publication and receipts

Agent sanitizes with the shared credential-name/whole-body/reflection/policy
rules before constructing a result; no raw target diagnostics. Preserve existing
supported UTF-8 JSON/text, gzip, duplicate-key/depth validation, body omissions,
wire/decoded/header bounds and fixed outcome vocabulary. No runtimeOutputs field
in this single-request DTO. The server validates IDs/source/target/policy,
numeric bounds, status/outcome consistency and complete encoded limits, decrypts
the frozen bundle in memory for independent defensive sanitization, and validates
that the final capture fits existing evidence/API/Lambda prepublication checks.
It never logs/reject-echoes input values. Invalid uploads publish nothing.
Sanitization can mask known reflections but cannot prove removal of arbitrary
agent-transformed secrets; configurable omitBody/path policies remain necessary.

First-upload authorization requires the original currently valid credential,
active owned project, active current generation and running job with matching
lease/fence/nonce and unexpired deadline. Authenticate and strongly revalidate
before KMS/ticket creation and condition these same versions in finalization.
Use server-authoritative IDs/submitted/intent/completion times. Agent timings and
observed status are claims from the paired device; timestamps never extend leases.

Canonicalize the validated normalized client packet with RFC 8785 and private
HMAC-SHA-256, separate domain including version/stage/project/job/credential/
lease/fence/nonce identity. Reject duplicate/unknown fields, invalid UTF-8,
nonfinite/unsafe numbers and trailing data; normalize collections to arrays and
nullable scalars to null using a versioned schema. Preserve array order and body
text exactly. Exclude server completion/acceptance timestamps and encryption
randomness. Pin a tested canonicalization implementation during implementation;
do not substitute Go JSON serialization for RFC 8785. Receipt identity uses the
validated client packet before server defensive masking, so an identical retry
does not require now-deleted job bindings. Final evidence checksum describes the
final sanitized bytes, separately from the private receipt HMAC.

Register one candidate identity and exact upload ticket/byte reservation under
live credential/job/project/stage fences. The API performs a bounded conditional
S3 PUT, not a presigned upload by the agent. Identical pending retries may resume
the same candidate/ticket while the lease and credential remain live; a different
candidate is 409. Persist only sanitized final bytes in S3, private HMAC metadata
in Control, and no raw upload body or plaintext staging file. The pending identity
is not an accepted receipt and confers no ACK-after-expiry authorization.

One final transaction inserts SNAP and retained `JOB#id#RECEIPT`, creates RET,
sets terminal job/anchor, settles ticket/accounting, clears only the matching
credential slot and deletes protected bindings. It conditions live first-upload
authorization again. Publication ACK may be 503 if uncertain; strongly read the
accepted receipt to resolve. No terminal overwrite or second evidence object.
Use an API handler bounded below the existing 15-second invocation limit; reserve
time for fixed-error handling. Publication failures preserve ticket obligations.

After authenticating the valid original credential and active project/generation,
read the accepted receipt coherently **before live lease checks**. Matching
canonical HMAC plus original lease/fence/nonce returns original 200 ACK even
after lease expiry, without KMS, PUT, counter changes or evidence writes.
Conflicting packet/lease is 409. Revoked/expired token is 401 even for identical
ACK; replacement credentials cannot acknowledge old work. Deleted project or
cleaned receipt is 404. Without an accepted receipt, expired/fenced/terminal lease
is 409. Keep receipt and HMAC key/version as long as evidence exists, including
pins; do not apply the seven-day admission-receipt lifetime to result receipts.

An upload prepared before expiry but unpublished at expiry is not a first-upload
success. Reconciler fences it, settles its possible S3 write and publishes a safe
unknown capture with its own recovery ticket; never accepts new late client data.
Deletion/revocation can similarly invalidate pending publication. Orphans remain
inaccessible and accounted until cleanup proves settlement. In-process agent
holds only the sanitized result and lease identity in memory; retries identical
uploads with backoff, never HTTP. Stop new-write retries at deadline; one
receipt-reconciliation retry after expiry may discover a lost successful ACK.
409/401/404 ends retry; process crash loses the packet and cannot recreate HTTP.

## Shared lifecycle, recovery and deletion

Local evidence uses schema 1 with `target:"local"`; expose target and nullable
localAgentId in history/detail/status metadata, without token, nonce, verifier,
IPs or protected values. Existing cloud rows default to cloud/null. Preserve
immutable captures and existing ownership, signed pagination, unavailable-body
handling, escaped React text and idempotent admission behavior. No evidence PATCH.
Keep normal expiry at completedAt+30 days; use the same RET/SNAP pin routes,
pin count/byte quotas, project/stage storage reservation and no-resurrection
conditions. Unknown/failed published local evidence can be pinned. Pinning keeps
the receipt/key, not protected runtime inputs. Unpin does not renew expiry.

Revocation before intent yields safe source-unavailable/not-dispatched cleanup.
After intent it denies uploads/ACK immediately and requests fenced unknown
reconciliation; agent cancels a running request best-effort when it observes 401
or local stop, but cancellation cannot prove absence of upstream effects.
An offline agent may finish HTTP and later be denied upload. Result publication
versus revocation is serialized by the credential version: publication first may
commit evidence; revocation first blocks it. Revoke does not erase existing evidence.

Project deletion immediately denies grants/uploads/ACK/evidence and advances the
epoch. Cleanup drains local jobs, pending candidate metadata, receipts, bindings,
RET/SNAP and ticket objects; pinning never blocks deletion. Remove that project's
credential scope through bounded versioned cleanup, preserving other scopes;
empty scope revokes/clears the account slot. Deleting-project gates deny access
before scope cleanup finishes. Clear activeJob only for its exact old identity,
so later jobs/pairing are not accidentally cleared. User-partition credential
metadata never contains request payloads and must be drained/redacted of deleted
project references without deleting unrelated project permissions.

Preserve existing registered-write deadlines, writer fencing, unknown-record
preservation, resumable WORK, exact-key removal and repeated residual prefix
sweeps. A timeout, absent HEAD or empty LIST is not proof that a PUT cannot land.
Do not delete tickets/reservations until possible writes settle. Physical project
completion requires strong Control/Protected partition proofs, empty prefix and
settled tickets plus completion of its credential-scope cleanup obligation.

Closed-stage recovery increments generation and fences all grants/uploads/ACKs.
Old credentials require re-pairing; old activeJob slots and restored queued/running
work are reconciled as recovery_interrupted, never made executable even if a
restored intent appears absent. Clear matching slots, reconcile counters,
publication/tickets and missing evidence before reopening. Existing terminal
evidence may remain readable after reconciliation, but old-generation credentials
cannot regain receipt authority. Preserve the accepted backup exception: actual
restore timestamps/warnings, possible lost changes and restored deleted items;
no independent anti-resurrection journal is added.

Compatibility proposal: add `localAgentExecutionsSchemaVersion=1` and advance
savedRequests capability 4→5 only after API, worker, ownership readers, both cleanup
paths and agent protocol understand target/credential/pending/receipt records.
Retain cloudExecutions=1, pinning=1, counters and recovery generation. Version
expanded job/plan rows explicitly; never let today's `knownExecution` silently
delete unsupported local records. Old binaries must fail closed at the stage
guard. No active-stage marker edit or deployment is part of preparation.

## Proposed routes and DTOs

All new application routes use `/api/v1`, existing success/error envelopes and
no-store. Browser routes use Cognito owner auth; agent routes use only agent
bearer auth and current project scope. Foreign/deleted project IDs are 404 after
authentication. Strict schemas reject unknown/duplicate keys. UUIDs are opaque.

| Route                                                              | Input and response                                                                                                                                                       |
| ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Browser `POST /local-agents`                                       | Required issuance Idempotency-Key; `{name,scopes:[{projectId,destinations:[{origin,addressClass}]}]}` → 201 `{data:{agent,token}}`; identical retry 200 with token null. |
| Browser `GET /local-agents/current`                                | 200 `{data:{agent:null\|Agent}}`; safe current/revoked metadata for reconciliation, no bearer token.                                                                     |
| Browser `DELETE /local-agents/{localAgentId}`                      | Empty body; repeated owned revocation 204 while metadata retained; foreign ID 404.                                                                                       |
| Agent `GET /local-agent/session`                                   | 200 `{data:{agent,serverTime}}`, current approved scopes and generation; no runtime inputs.                                                                              |
| Agent `POST /projects/{projectId}/local-agent/poll`                | Empty body → 200 `{data:{work:null\|Work,busy:boolean,serverTime}}`; busy never includes executable work.                                                                |
| Agent `POST /projects/{projectId}/local-agent/jobs/{jobId}/result` | Result below → 200 `{data:{executionId,acceptedAt}}` only after publication or matching accepted receipt.                                                                |
| Existing browser submission/rerun                                  | Local union below; same 202, Location, Retry-After, revision/key/error behavior as cloud.                                                                                |

`Agent={localAgentId,name,scopes,createdAt,expiresAt,revokedAt,lastSeenAt,
recoveryGeneration}`; null timestamps are explicit. Names follow bounded safe
project-name validation (1–100 characters), never used in logs. No token hash.
Status/metadata access returns only fixed safe reasons, not device diagnostics.

Example local submission (all old execution options retain their defaults):

```json
{
  "target": "local",
  "local": {
    "localAgentId": "opaque-id",
    "allowPrivateNetwork": true
  },
  "timeoutSeconds": 30,
  "allowInsecureSecrets": false,
  "responseRedaction": { "headers": [], "jsonPointers": [], "omitBody": false }
}
```

`Work={schemaVersion:1,projectId,jobId,executionId,localAgentId,
recoveryGeneration,leaseId,fence,nonce,dispatchIntentAt,executeNotAfter,deadline,
plan,protectedInputs:{values:{secretId:value}},destination:{origin,addressClass}}`.
The plan is the frozen execution configuration/options/source revision; source
refs are execution-only stable IDs, never current reusable secret API authority.
All identity/deadline fields are required; nonce is base64url without padding.

```json
{
  "schemaVersion": 1,
  "leaseId": "opaque-id",
  "fence": 1,
  "nonce": "base64url-256-bit-value",
  "capture": {
    "status": "completed",
    "outcome": { "code": null, "certainty": "known" },
    "request": {
      "method": "GET",
      "url": "http://localhost:3000/health",
      "headers": [],
      "body": { "kind": "none", "text": null, "omissionReason": null }
    },
    "response": {
      "httpStatus": 200,
      "headers": [],
      "body": { "kind": "none", "text": null, "omissionReason": null },
      "wireBytesRead": 0,
      "decodedBytesRead": 0
    },
    "timing": { "durationMs": 10, "timeToFirstByteMs": 8 },
    "redaction": { "policyVersion": 1, "applied": true }
  }
}
```

Server constructs the full shared Evidence DTO with authoritative source, target
and timestamps. Capture fields/enums/nulls match existing evidence; validate method
and sanitized request against the frozen plan, forbid target-controlled metadata
and unsupported outcome codes. No assertion/schema/output fields. Timing is
nonnegative integer milliseconds <=60,000, first byte <=duration, or null when
unavailable; fence is a positive safe integer. HTTP status null or 100–599,
byte counts bounded by capture limits. Fixed API errors: 400 invalid shape/policy,
401 invalid/revoked/expired credential, 404 inaccessible resource, 409 pairing,
candidate/lease conflict or missing local permission, 413 size, 429 rate/quota,
503 closed stage/uncertain write/unavailable dependency. Never forward raw errors.

## Limits, minimal delivery and costs

| Boundary                    | Proposed default or preserved accepted limit                                                                                                                 |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Credential/scope            | 30 days; one active/account; 1–10 projects, <=16 exact origins/project; <=16 KiB credential scope metadata                                                   |
| Pairing transport           | <=32 KiB request; five issuance attempts/account/hour                                                                                                        |
| Saved/frozen/resolved input | 64 KiB saved configuration; independent complete frozen plan 64 KiB and resolved body 64 KiB; protected encrypted bundle 32 KiB                              |
| Executable poll response    | Complete serialized response <=128 KiB, including escaping/plaintext values/envelope; reject admission if frozen grant cannot fit; recheck before disclosure |
| HTTP execution              | 30 seconds default, 1–60; DNS 2s, connect/TLS 5s, response headers 10s within total; headers 64 KiB                                                          |
| Dispatch                    | Start window 30s, lease 120s, queued expiry 10m; no heartbeat extension                                                                                      |
| Polling                     | Ten seconds/project; <=10 scopes gives <=1 aggregate poll/s; one in flight; sixty-second offline backoff; per-credential limit 2 polls/s burst 2             |
| Upload                      | Complete encoded request <=4 MiB; no compression; one upload in flight, <=10 attempts/job/minute; deadline bounds first acceptance                           |
| Evidence                    | Independent 2 MiB wire/decoded body; object/API envelope 4 MiB each; fully encoded Lambda response 6 MiB and metadata <=16 KiB                               |
| Transactions                | Existing <=80 actions, <=2 MiB total, <=128 KiB/item; local grant/admission/finalization each <=40 actions including <=16 distinct source guards             |
| Receipts/history            | Admission and issuance receipts 7d; accepted result receipt/key through evidence lifetime/pins; normal evidence 30d                                          |
| Quotas                      | Preserve 1,000 executions/month, 100/user/day, 1 GiB/project and 2 GiB/stage; pin quotas 100/200 captures and 256/512 MiB project/stage                      |

Complete limits include wrappers, policies, references and JSON escaping; never
count only body text. Poll-size preflight counts secret values only in authorized
memory, with actual transport serializer. If result-wrapper overhead exceeds
4 MiB, agent omits bodies with existing fixed limit reason before freezing its
candidate; server repeats every final evidence delivery-size check. No chunked
upload, large-body store or resumable raw capture. Explicitly build/test worst
escaping because the encrypted-bundle cap does not prove plaintext grant fits.
Also bound the complete API Gateway-to-Lambda request event below 6 MiB,
including any base64 expansion and fixed headers/context; reject excess headers
and never reflect them. Test the deployed payload-v2 encoding for near-cap
uploads rather than assuming that a 4 MiB HTTP body always fits Lambda ingress.
Deadline authorization uses fresh server decision time with a five-second
margin, matching the existing clock-boundary approach; DynamoDB conditions are
not a remote wall-clock NOW(). Slow commits/skew remain explicit test cases.

CLI: `kurier-agent pair`, `run`, `status`, `unpair`; run stays foreground and opens
no browser-facing socket. Build from the repository's Go module with a documented
version/commit, or provide checksummed platform binaries after implementation.
No shell download-and-execute installer, auto-update, launchd/systemd/Windows
service integration. Existing loopback health server is optional liveness only,
disabled by default in executor mode; it must never expose token, scope or jobs.
Signal handling stops polling, cancels best-effort, optionally finishes an
already prepared upload within live authorization, then discards memory.

React adds account pairing/revoke/expiry/status controls, project scope/origin
review and Cloud/Local target selection defaulting to Cloud. Clear token display
on close/sign-out/unmount; no localStorage/sessionStorage or automatic clipboard
copy. One issuance/submission in flight; uncertain issuance shows refresh then
explicit revoke/re-pair. Target selection names device/scope/offline state and
does not pretend the browser's localhost is the agent's machine. Retain existing
request-revision refresh, keys in memory, history/detail/evidence/pin controls,
unknown warning and project-switch/late-response fences. Rerun asks for fresh
consent; loss of pairing disables Local and preserves cloud behavior.

No new table, index, queue, bucket, KMS key, scheduled service or standing compute
is proposed. Reuse existing API/maintenance/resources; add routes and scoped
record permissions/authorizer wiring. Local HTTP time is borne by the user's
machine, not the cloud worker. Review generated SST/bootstrap/IAM differences
only after acceptance and deployment authorization; preserve Free plan, account
quota ten and standard SQS max concurrency two without reserved capacity.

Incremental development planning model, USD/us-east-2, no free-tier subtraction:
one additional agent, one project, 4h/day ×22 days at 10s gives 31,680 polls/month;
1,000 local executions at 100 KiB evidence and 1,000 additional evidence views.
Allowance: <=40k hosted calls (including 512-KiB request billing units for uploads),
40k 256-MiB ARM invocations averaging 100ms =1,000 GB-s, 500k RRU/100k WRU
(transactions/item rounding and presence included), 0.1 GB extra metadata/PITR/
backup, 0.1 GB retained S3 and <=1 GB delivery. Reference rates from the
[existing regional estimate](../development/evidence-pinning.md): HTTP API
$1/m calls, Lambda $0.20/m +$0.0000133334/GB-s, DDB $0.125/m RRU and $0.625/m
WRU, metadata $0.55/GB-month combined storage/PITR/backup, KMS $0.03/10k
operations, S3 $0.023/GB-month plus PUT/GET, delivery $0.09/GB.
This yields roughly $0.04 API +$0.022 Lambda +$0.125 DDB requests +$0.055
metadata +$0.01 crypto +$0.01 S3 operations/storage +$0.09 delivery; reserve
**$0.50–$1/month additional** for safe logs, retries and cleanup, not a hard cap.

Official [API Gateway](https://aws.amazon.com/api-gateway/pricing/),
[Lambda](https://aws.amazon.com/lambda/pricing/), [DDB](https://aws.amazon.com/dynamodb/pricing/),
[KMS](https://aws.amazon.com/kms/pricing/) and [S3](https://aws.amazon.com/s3/pricing/)
pricing pages were read on 2026-10-13. The numerical regional rates above remain
planning inputs from the prior estimate, not a freshly queried Ohio price-list
or measured bill. Refresh regional catalog and generated diff before deployment.
Ten projects multiply base agent polling by ten; all-day one-project polling is
259,200/month. More views, pins, max-size uploads, rejected calls, rotations and
bootstrap/shared stages increase spend. Do not double-count existing resources
or claim the earlier all-service forecast represents current total billing.

## Reuse and cloud-specific adaptations

| Existing component                                            | Reuse / adaptation required after acceptance                                                                                                                                                                                                                                         |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `services/api/internal/ownership`                             | Reuse Cognito ownership, project/source CAS, secret descriptors and deletion gate. Add separate agent bearer middleware; browser verifier cannot authenticate agents.                                                                                                                |
| `execution/store.go`, `crypto.go`                             | Reuse admission quotas, source-bound receipt namespace and fresh job bundles. Add target/scope freeze, local digest variant and no local OUT/SQS; preserve old cloud identity.                                                                                                       |
| `execution/runtime.go`, `redaction.go`, `json.go`, `model.go` | Reuse bounded resolution/capture/sanitization/evidence schemas. Extract a small AWS-independent Go runtime package usable by both worker and agent; current API module owns runtime and AWS dependencies, local-agent module has no dependency on it. Keep golden cloud regressions. |
| `execution/destination.go`                                    | Keep cloud public-IP/80/443 policy exactly; add separate explicit local policy at prepare and dial. Test hooks bypassing DNS are fixtures, never production permission.                                                                                                              |
| `execution/worker.go`                                         | Reuse ticket/S3/terminal/accounting logic through narrow publication helpers; cloud Process is not a local poll handler. It currently lacks a local-target rejection fence and local credential/result receipt conditions.                                                           |
| `execution/history.go`, `pinning.go`, React `Executions.tsx`  | Same ownership/RET/SNAP retrieval, immutable rendering and pin accounting; add safe target/agent metadata and retain local receipts through pinned retention.                                                                                                                        |
| `execution/maintenance.go`, ownership maintenance             | Add local queued/lease discovery, credential-slot/scope cleanup and result-receipt recognition. Existing reflective known-record checks and closed-stage capabilities require coordinated rollout; unknown schemas remain pending.                                                   |
| `services/local-agent/main.go`                                | Health-only loopback scaffold; replace default startup with explicit pairing/poll/run/upload lifecycle and separate hosted/target transports. Existing tests establish liveness only.                                                                                                |
| `apps/web/src/Requests.tsx`, `api.ts`, auth                   | Extend strict unions and consent/status controls; old cloud DTO defaults remain valid. No browser-protected-input grant or token persistence.                                                                                                                                        |
| Existing infrastructure/config tests                          | Reuse resources/schedule, but agent routes need their own auth integration and bounded payload/handler verification; no claim of zero IAM change before preview.                                                                                                                     |

## Planned tests and validation boundaries

All following local-agent tests are **planned**, not passed by this packet.
Use deterministic clocks/barriers and injected faults, then DynamoDB Local/race
and browser fixtures. Count HTTP at a controlled harmless destination; assert
no automatic second send, rather than only checking terminal status.

| Area                   | Required scenarios                                                                                                                                                                                                                                                                               |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Ownership/pairing      | Foreign/deleted/disabled project; excessive scope; concurrent last account slot; lost token/issuance ACK; key reuse; agent/browser/MCP auth confusion; foreign revoke; stored token absent from logs and browser persistence.                                                                    |
| Destination misuse     | Unconsented host/port/project, changed hosted/local list, private DNS with mixed/public answer, rebinding before dial, mapped IP, metadata/link-local/tunnels, proxy/redirect/Host override, TLS failure; secret-over-HTTP consent separate from local consent.                                  |
| Durable dispatch       | Rollback before intent; committed/lost/truncated poll response; ambiguous commit; two pollers; duplicate envelope; crash before/after send, missed start, lease expiry; zero regrant/replay/fallback, correct unknown warning and new-key deliberate rerun.                                      |
| Revocation             | Before grant, after grant, during HTTP, during KMS/PUT/finalization and after publication; expiry; replacement credential; old slot cleanup cannot clear new slot; identical revoked ACK denied; already dispatched side effects never claimed cancelled.                                        |
| Result receipts        | First-upload live lease; lost final ACK and identical retry after expiry; pending candidate is not accepted receipt; changed body/timing/order/lease/nonce conflicts; normalized key-order equality; original key-version retention through pins; canonical RFC 8785 vectors and unsafe numbers. |
| Secret containment     | Memory-only disposable header/query/body/JSON secrets and supported encoded reflections; server defensive sanitizer; no plaintext in grant errors, logs, Control, local files, browser status or evidence; no raw upload staging, untrusted-agent transformations explicitly outside proof.      |
| Bounds                 | Exact/one-over plan/bundle/grant/result/object/API/Lambda caps, escaping/Unicode/gzip/header growth; bounded timings/pages/rates/transactions; no admission when frozen grant cannot fit, no publication before final size checks.                                                               |
| Offline/recovery       | Offline queued expiry, presence lag, repeated failures/backoff, interrupted upload and process restart losing buffers; closed stage/generation change, restored missing intent/old receipt/old token; deliberate re-pair only, no restored HTTP.                                                 |
| Publication/deletion   | Result versus reconciler, expiry, revocation, pin/unpin and delete; late/uncertain PUT, missing/corrupt evidence, multichunk cleanup, retained pinned receipt, deleted scope/activeJob cleanup, unknown records and residual sweep/accounting proofs.                                            |
| Cloud/React regression | Existing cloud body/key/OUT/SQS behavior and public policy unchanged; all prior evidence/pinning tests; target/consent/offline/device wording, memory clearing, ambiguous pairing and project-switch responses, accessible errors and unknown-outcome UI.                                        |

Future genuine validation requires running the compiled foreground agent against
controlled localhost and private-network services on Davian's machine, using
disposable memory-only secrets; real browser pairing/consent/revoke/re-pair and
online/offline transitions; process termination and observed target send counts.
Actual AWS checks separately cover deployed agent auth, IAM/KMS contexts,
DDB transactions/index propagation, S3 immutable publication, scheduled lease/
deletion cleanup and generation gates. Real restore, elapsed credential expiry,
clock skew and metered cost require their own evidence. Fixture clocks, fake AWS,
Local DDB and simulated crashes cannot establish those outcomes. No genuine
agent/AWS checks or AWS mutations are performed during preparation.

## Preparation record and review decisions

- Accepted contracts, ADRs, live proposal/API contracts, execution/runtime/
  evidence/pinning/maintenance and local-agent scaffold were read. Trello board,
  contract-review, cloud/prototype, saved/protected and pinning dependency cards
  and relevant checklists were inspected. No configured WIP limit or pre-existing
  local-agent task card was returned; a scoped
  [local-agent card](https://trello.com/c/sule204p) was created in Doing.
- Evidence pinning merge confirmed and requested branch created from current
  main. Only this Proposed packet is intended for the task commit.
- Preparation checks passed: repository Prettier/gofmt checks, ESLint, 44
  frontend tests, 10 infrastructure tests, TypeScript/Vite production build,
  tests/vet and builds for all five Go modules. The initial sandbox Go run was
  blocked by listening-socket restrictions; the authorized outside-sandbox
  rerun passed. Balanced Markdown fences, JSON examples and local file links
  were checked. These verify the existing baseline and document structure,
  not proposed local-agent behavior or its future fault tests.
- Trello tracks preparation separately from acceptance, implementation/external
  synchronization and deployment/genuine checks. The preparation publication
  item is completed only after commit/push and readback; its card links the
  resulting GitHub commit. All dependent milestones remain open. No AWS API
  mutation, SST preview/deployment or live contract edit was performed.

Decisions requested for acceptance:

1. Manual browser-issued token/CLI pairing, immutable <=10-project exact-origin
   scope, 30-day expiry, one agent/account, memory-first credential storage with
   explicit private-file persistence and revoke/re-pair for scope changes.
2. Local-only loopback/RFC1918/ULA origin policy, explicit device/network and
   separate HTTP-secret consent, cloud defaults/digests unchanged, no fallback.
3. Intent-before-response, 30s start/120s lease, no regrant after any possible
   intent, ten-second/project polling and ten-minute offline queued expiry.
4. Execution-only frozen protected delivery, agent plus server sanitization,
   proposed strict capture/result schemas, RFC 8785 private receipt identity,
   live first-publication authorization and identical ACK-only retry after expiry.
5. Shared immutable evidence/pins/retention, revocation/generation/delete fences,
   pending-ticket settlement, receipt/key retention and coordinated capability
   rollout; foreground Go CLI/React controls and $0.50–$1 incremental allowance.

Acceptance of choices must be recorded explicitly. Wait before dependent code,
external contract synchronization or AWS deployment; each later authorization
must state its scope. Preparation publication alone is not acceptance.

## Accepted implementation continuation

Davian accepted `585d5b3` and authorized the scoped foreground Go agent,
pairing/authentication, local execution protocol, shared evidence/cleanup and
React controls. API contracts synchronization, meaningful local/race/protocol/
browser checks and AWS preview/cost preparation are authorized. AWS deployment
and capability activation are held for his review. Preserve Free plan, account
quota and existing concurrency.

Concrete wire supplements implement the accepted startup/status behavior:
`DELETE /api/v1/local-agent/session` permits only the authenticated credential
to revoke itself for CLI unpair; it grants no browser CRUD or other-agent
revocation authority. Browser current metadata displays credential expiry and
last seen; initial React pairing selects the current project, while the API
supports the bounded multi-project scope. Full validation/startup and deployment
preview are recorded in the development guide after performed checks.
