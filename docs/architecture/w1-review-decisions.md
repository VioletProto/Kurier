# W1 review decisions and contract gaps

Status: proposed for Davian Hernandez review, 2026-10-05. These recommendations
are concrete review inputs, not changes to the accepted ADR or source contracts.
See [system architecture and execution sequence](w1-system-design.md) and
[ERD/schema](w1-data-model.md). The architecture task stays Doing and the
API-contract review card stays open until review and synchronized documentation.

## Decision sheet

Every recommendation in this table is **proposed** unless marked agreed.
Costs are material cost drivers, not quotes or deployed billing measurements.

| Decision                            | Recommended choice                                                                                            | Reason                                                             | Material cost/tradeoff                                                                                           |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| Infrastructure framework (agreed)   | SST 4, accepted ADR 0001                                                                                      | Both spike phases support it                                       | SST/Pulumi state/bootstrap remain operational dependencies; proposal CDK wording needs reconciliation.           |
| Product networking                  | Private Fargate tasks, HTTPS ALB, isolated private RDS, one NAT in development                                | Keeps task/DB ingress constrained while worker reaches public APIs | NAT and ALB fixed charges plus egress; one NAT is an AZ dependency. Public-IP alternatives need explicit review. |
| Initial persistence                 | PostgreSQL 17; bounded sanitized evidence in DB; static frontend in S3/CloudFront                             | Atomic finalization/deletion avoids premature DB/S3 coordination   | Database storage/backup growth; S3 artifact extension adds deletion reconciliation.                              |
| Authentication (provider/UI agreed) | Cognito Lite explicitly selected; public browser app client; custom shared-theme screens; short access tokens | Basic email/password fits Lite; no login credentials in Kurier DB  | Cognito MAU/email usage; Lite excludes newer features; Essentials is the current default when tier omitted.      |
| Execution limits                    | 30-second HTTP deadline, maximum 60; 1 MiB request, 2 MiB response, 64 KiB headers                            | Bounds worker occupancy and evidence growth                        | Large APIs/uploads/streams unsupported; omitted bodies require clear UI metadata.                                |
| Cloud destinations                  | Public HTTP/HTTPS, ports 80/443 only, redirects off, validate and pin resolved public address                 | Prevents private-network and credential-endpoint requests          | Nonstandard public ports need local execution or later reviewed expansion.                                       |
| Reusable secrets                    | Per-stage symmetric KMS key, AES-256-GCM envelope encryption in protected PostgreSQL rows                     | Authenticated encryption plus explicit authorized runtime access   | KMS key/API cost and outage dependency; more crypto metadata and rotation work.                                  |
| Queue and retries                   | Standard SQS + DB outbox/lease/fence; no automatic HTTP replay after dispatch intent                          | Durable acceptance and one final snapshot despite duplicates       | Extra DB state; uncertain crash outcomes require user rerun.                                                     |
| Pagination                          | Keyset cursor, 25 default / 100 maximum, HMAC-authenticated scoped cursor                                     | Stable ordering and bounded work without large offsets             | Cursor key management; no random page-number jumps.                                                              |
| Status stream                       | Fetch-based bearer-authenticated SSE, monotonic event IDs, replay then GET reconciliation                     | Works with current bearer contract and recovers missed events      | Client parser/retry logic; persistent small events and open connections.                                         |
| Local agent                         | One active credential per user, 30-day token, 20-second long poll, 120-second job lease                       | Fits MVP scope and avoids busy polling                             | Device receives execution secrets; revocation cannot erase secrets already delivered.                            |
| Pin/retention (behavior agreed)     | Proposed idempotent PUT/DELETE pin routes; separate metadata; hourly cleanup                                  | Maintains immutable evidence and 30-day policy                     | Pinned storage is unbounded until deletion; serialize cleanup against pin.                                       |
| MCP transport/access                | Local Go stdio server with distinct read-only credential; audit before response                               | Keeps initial integration small and removes secret execution scope | Manual token pairing/renewal; remote OAuth MCP is separate work.                                                 |

## Execution limits and failure semantics

Propose request body <= 1 MiB, serialized definition/execute-write <= 2 MiB,
OpenAPI upload <= 5 MiB, URL <= 8 KiB, combined request/response headers <= 64 KiB
each and <= 100 fields. Response body <= 2 MiB **after decompression**, with a
separate 2 MiB wire-read cap. Bound JSON nesting at 64 levels and reject
unsupported body encodings; propose JSON/text UTF-8 only in MVP.
Use a 30-second overall DNS/connect/TLS/body deadline, user-selectable 1–60
seconds; connect five seconds, TLS handshake five seconds, response-header wait
ten seconds, all capped by remaining overall deadline. Worker finalization
gets a separate bounded 15-second internal budget and the 120-second lease.
Start worker concurrency at four, per-user in-flight limit five, queued limit 100. Rate/concurrency enforcement must not expose another user's activity.

Go exposes
[Client.Timeout, CheckRedirect, Transport limits, and MaxBytesReader](https://pkg.go.dev/net/http).
Apply caps while streaming, independent of Content-Length. If cap+1 bytes are
observed, stop reading; produce failed `response_too_large`, preserve status
and sanitized headers if received, omit body and set `bodyOmittedReason`.
Do not store an unsafe truncated prefix or label it a full response. A 4xx/5xx
remains failed with exact status even if body omitted. Unsafe decoding/redaction
also omits body. A pre-response timeout has null httpStatus; a body timeout
after response headers preserves the received status. Proposed failures add
structured codes without inventing an upstream HTTP status.

OpenAPI parser/library selection remains on its existing card. Propose only
internal document `$ref` resolution initially with explicit unsupported-feature
warnings, bounded references and validation work; no arbitrary remote fetch.

## Cloud destination policy

Accept only absolute http/https URLs; reject userinfo, malformed hosts, zone
identifiers, alternate/ambiguous IP encodings, unsupported ports, and explicit
Host overrides. Do not use ambient HTTP_PROXY/HTTPS_PROXY. Disable redirects
initially; show sanitized Location and 3xx response. A later redirect option
would allow at most three hops, reapply the complete policy on every hop, and
drop credentials on origin change. HTTP is allowed by MVP scope, but warn when
sending protected configuration without TLS and propose explicit confirmation.

Resolve A/AAAA records with a controlled resolver on each new dial. Reject
the destination if any candidate is nonpublic, including loopback, RFC1918,
ULA, link-local, CGNAT, multicast, unspecified, reserved/documentation, metadata,
and VPC-local ranges. Normalize IPv4-mapped IPv6 before checking; deny
translation/tunnel address forms initially. Use the current
[IANA IPv4](https://www.iana.org/assignments/iana-ipv4-special-registry) and
[IPv6](https://www.iana.org/assignments/iana-ipv6-special-registry) registries
to maintain/test the policy; Go's global-unicast predicate alone is insufficient.
Pin the validated IP in the dialer; retain original hostname for TLS SNI and
certificate validation. Never validate DNS and then let another resolver pick
the connection address. Apply checks on reconnect as well; cap DNS time and
reject private answers even from a previously allowed hostname. Destination
restrictions apply only to user-request HTTP transport, separate from narrowly
configured PostgreSQL/AWS SDK transports that require private/runtime endpoints.

The local agent permits explicitly approved localhost/private hosts and ports
on that device. Default allowlist is loopback only; private network expansion
requires local operator configuration. Block cloud metadata/credential endpoints
there too. Hosted clients cannot turn the agent into an unrestricted network
proxy. Cloud public-destination restriction does not eliminate abuse against
public services; quotas and per-user limits are needed from the first executor.

## Secret encryption and rotation

Use a symmetric customer-managed KMS key per stage and one fresh data key per
protected value/binding; AES-256-GCM with fresh nonce. Store ciphertext, nonce,
tag, wrapped key, key ARN, crypto version, and non-secret context identifiers.
Generate and unwrap keys using narrowly scoped task roles; never put decrypted
keys in SST links, env vars, logs, queues, disk, or evidence. Use a reviewed Go
crypto implementation; avoid designing a new cryptographic format without
known-answer/tamper tests. Local tests use an isolated test key provider, never
a plaintext production fallback.

[KMS envelope encryption](https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html)
and [encryption context](https://docs.aws.amazon.com/kms/latest/developerguide/encrypt_context.html)
support this approach. Context is visible in CloudTrail; include only stage,
opaque project/binding IDs, app name and purpose. IAM/KMS checks constrain
stage/purpose; application authorization still proves user/job ownership.
For local polling, only the leased execution path can decrypt and deliver
runtime configuration over TLS to its paired device. MCP never has that path.

Propose automatic annual KMS key-material rotation. Existing wrapped keys
remain decryptable; key rotation does not rotate users' API credentials or
rewrite historical evidence.
[AWS documents this distinction](https://docs.aws.amazon.com/kms/latest/developerguide/rotate-keys.html).
Rewrapping for a replacement key is a reviewed background migration; retain
the old key until live bindings/backups requiring it have expired or been
migrated. Secret-value rotation updates the stable saved-secret slot and
revision; future submissions/reruns use the new value. Queued jobs use their
frozen encrypted bindings unless explicitly revoked. Retain no reusable
historic plaintext/ciphertext in execution snapshots. DB credentials use
Secrets Manager independently; propose rotation plus pool refresh testing,
not a static copy of the spike master credential.

## Pagination and SSE

All collection routes propose `limit` (default 25, maximum 100) and `cursor`.
Use descending `(createdAt,id)` for definitions/projects and
`(submittedAt,executionId)` for history; failed terminal lists use
`(completedAt,executionId)`. Cursor is versioned base64url JSON + HMAC,
binding owner, resource/filter, last sort tuple, and initial high-water tuple.
Reject invalid/mismatched cursors with invalid_request. Include key ID for
rotation; expire cursors after 24 hours. New items wait for a fresh first page;
deletions may shorten pages. No total count by default. MCP uses the same scope
and ordering policy. API limits apply regardless of provided token scope.

Use browser fetch with Authorization bearer and incremental SSE parsing;
native EventSource cannot supply the current bearer header directly. Never
put access tokens in query strings. `id` is the per-execution monotonic sequence;
send Last-Event-ID on reconnect, persist only the cursor client-side, deduplicate
events, replay events with greater sequence. Keep the four existing contract
event names and use heartbeat comments every 15 seconds. Replay all events
until execution retention removes them; the MVP generates only bounded state
transitions, not progress/body streams. Preserve retry attempts in job metadata.
CloudFront must not cache authenticated API/evidence responses; SSE goes to ALB
with proposed idle timeout 120 seconds and proxy buffering disabled.

[The HTML SSE standard](https://html.spec.whatwg.org/multipage/server-sent-events.html)
defines event IDs and Last-Event-ID. Fetch requires implementing those client
behaviors explicitly. Reconnect with jittered 1/2/4-second delays capped at
30 seconds. Refresh token through the auth SDK on 401, then retry once; close
streams at token expiry. Recheck ownership/project state periodically and on
each event read. Invalid/ahead cursor returns invalid_request before streaming;
missing/expired execution returns not_found. On reconnect fetch GET execution
and its proposed lastEventId to reconcile status. If database state is terminal
and already delivered, close; do not add an unreviewed reset event type.

## Local agent and MCP credentials

Local tokens: 256 random bits, raw token returned once, store SHA-256 digest,
30-day expiry. High entropy permits a fast digest; this is not password hashing.
Create/revoke under a user row lock, enforce one unrevoked credential per user,
and atomically revoke expired predecessor before replacement. Poll long-waits
20 seconds, empty response proposed 204; after empty wait jitter 1–3 seconds,
after network errors exponential backoff to 30 seconds. One poll/job at a time
for MVP. No inbound device networking or AWS credentials required.

Return a job ID, execution ID, frozen runtime configuration and per-lease nonce;
nonce hash/fence/deadline stored in job metadata. Lease 120 seconds is adequate
for maximum 60-second HTTP plus sanitization/upload. Proposed result endpoint
requires that exact live lease and token. Same final sanitized payload digest
returns existing result; different duplicate returns conflict. Result upload
must meet size/schema caps and undergo server-side sanitization too. Client
timing is identified as agent-reported; API receipt/completion timestamps are
server-controlled. A local agent is trusted to execute on its device, not to
make arbitrary writes to evidence tables.

Revocation is checked from DB on **every** poll/result call without positive
credential caching; reject revoked/expired credentials as unauthenticated.
Revoke also fences pending leases and destroys their bindings. A request already
dispatched cannot be undone, and a device can retain a secret already delivered.
Lost local lease after dispatch finalizes unknown outcome instead of assigning
the HTTP request again. No forced cloud fallback. Pair/rotation UI should state
these limits clearly. Agent token storage uses the OS credential store or an
owner-readable local file; runtime secrets stay memory-only.

Propose separate 30-day read-only MCP credentials, created/revoked by an
authenticated owner, exposed once and hashed. Stdio server receives its own
credential from local secure configuration, verifies tool inputs, and calls
an API endpoint that audits invocation before returning sanitized data. Fail
closed if mandatory audit persistence fails. Unauthenticated attempts log only
safe security categories since no user is known. Cross-user denied attempts
must not copy the foreign project's content into an audit row. Project-specific
audit rows delete with that project; account-only records expire after 90 days.
Remote MCP must follow the current
[MCP security guidance](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices),
including intended-recipient tokens; raw Cognito token passthrough is not an
approved remote MCP authorization design.

## Pinning and cleanup API proposal

Propose `PUT /api/v1/executions/{executionId}/pin` and
`DELETE /api/v1/executions/{executionId}/pin`, owner-authorized, terminal-only,
idempotent, returning 200 with `data.retention`:

```json
{
  "data": {
    "retention": {
      "executionId": "opaque-id",
      "pinned": true,
      "pinnedAt": "2026-10-05T23:00:00Z",
      "expiresAt": null
    }
  }
}
```

Unpin resets expiry to original completedAt + 30 days. If past due, the unpin
response reports the past expiresAt and the execution becomes unavailable to
normal reads immediately; cleanup removes it on the next pass. Repeating an
operation on an already removed result returns not_found. Pinning an expired
unpinned result is rejected even if physical cleanup has not run. Pinned
results cannot be removed by unpin cleanup before that transaction commits.
Retention is a sibling of the immutable record in read responses, never
embedded in the captured evidence hash. Hourly batches and project deletion
share locking/fencing procedures. No blanket S3 30-day lifecycle may override
pinning if object storage is introduced.

## Cognito verification and email checks

Verify RS256 signature, allowed algorithm/key type, `kid`, exact configured
issuer, `token_use=access`, allowed `client_id`, `exp`, and sensible issuance
time with 60-second clock tolerance. Require scopes for any separately defined
API resource-server scopes. Cache only the configured pool's JWKS, refresh on
unknown kid with rate limiting, never trust token-supplied key URLs. Reject ID
tokens. User identity is `(iss,sub)`. Use a maintained Go JWT/JWKS library chosen
and tested in the auth task; AWS's Node-specific verifier is not a Go dependency.
These checks follow
[Cognito JWT verification guidance](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-tokens-verifying-a-jwt.html).

Propose 15-minute access-token lifetime, one-day refresh session with rotation,
in-memory browser storage and reauthentication after reload. Do not persist
bearer/refresh tokens in localStorage. Cognito revocation is not automatically
observed by offline signature checks; document API access may continue until
token expiry. An immediate sign-out guarantee requires a reviewed application
session/revocation check. Application user disabled state should be enforced
on each request. See
[Cognito revocation behavior](https://docs.aws.amazon.com/cognito/latest/developerguide/token-revocation.html).

Select Lite explicitly after checking SDK flow compatibility; no email MFA or
Plus risk protection is assumed. The current
[feature-plan guide](https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-sign-in-feature-plans.html)
distinguishes those plans. Propose SES-backed Cognito transactional email for
presentation/public signup, with verified sender identity, regional compatibility,
DKIM/SPF/DMARC and bounce/complaint visibility. Cognito's default email capacity
is limited and SES sandbox restricts recipients; independently verify quotas,
production access, sender/reply-to, and delivery for Kurier.
[Cognito email documentation](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-email.html)
describes both options. No cost or free-tier assumption is made.

Before acceptance of the auth implementation: read Dollhouse's email configuration
as a reference under a separate scoped task; do not copy its credentials or pool.
For Kurier verify signup, verification/resend and cooldown, reset request/code/
confirmation, expired/incorrect codes, real delivery to independent recipients,
rate limits, generic enumeration-resistant errors, and sign-out/session limits.
Record actual pool tier, SES region/identity/quotas and pricing reviewed at that
time. No email delivery or AWS configuration was tested by this design task.

## Contract gaps and source conflicts requiring review

The following are findings against the read v0.1 document, not silently accepted
routes. Pre-release adoption could remain v1 after coordinated review; there
is no implemented product data to migrate yet. Later revisions need versioned
SQL and clients updated alongside the contract.

| Source/route or schema                                      | Gap/conflict                                                                                                                                                                 | Concrete proposal for review                                                                                                                                                                                                                                |
| ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Proposal technology, deployment requirement, W2/W9 timeline | Still names CDK; accepted ADR selects SST. `infra/README.md` incorrectly says no framework adopted.                                                                          | Keep SST; reconcile proposal on its existing card. Leave accepted ADR unchanged; flag stale infra text for synchronized documentation.                                                                                                                      |
| Proposal history versus contracts                           | Durable preserved executions can read as indefinite retention; contracts select 30-day unpinned MVP and indefinite history post-MVP.                                         | Explicitly describe bounded MVP plus pinning; retain agreed policy.                                                                                                                                                                                         |
| KeyValueField and request/environment writes                | `value` plus sensitive boolean lacks secretRef, set/preserve/remove semantics, body JSON paths, and masked reads. Returning RequestDefinition cannot echo saved credentials. | Sensitive writes accept value only for replacement, secretRef for preserving known binding, explicit remove operation; reject literal mask as credential. Reads return null value, masked=true, secretRef. Define nested body/query/URL sensitive paths.    |
| POST executions / Queue ExecutionJob                        | No frozen revisions, idempotency, limits, lease fields, schema capture; queue attempt can be stale.                                                                          | Add reviewed Idempotency-Key/revision preconditions and DB frozen plan; keep queue identifier-only; attempt advisory. Public revision fields need schema updates.                                                                                           |
| ExecutionSummary / ExecutionRecord                          | queued summary mutable but record called immutable; lacks projectId, nullable deleted request link, omission metadata, evidence version and replay references.               | Separate status resource from terminal captured record; captured requestId remains historical string; expose liveRequestId separately if needed. Add typed sanitized body/error/assertion/violation shapes.                                                 |
| GET request history after request deletion                  | No project-wide history route; nested deleted request cannot be read normally.                                                                                               | Add GET /api/v1/projects/{projectId}/executions with filters; direct GET execution remains available by owner. Historical request filter uses frozen ID.                                                                                                    |
| POST execution reruns                                       | Secret rebinding and deleted-source behavior unspecified.                                                                                                                    | Replay frozen non-secret config, current stable secret references, fail clearly if missing; capture new bindings and execution. No fallback by variable name.                                                                                               |
| Failed/completed semantics                                  | HTTP 4xx/5xx agreed; assertions/validation failures and 3xx policy unspecified.                                                                                              | Failed for assertions; propose failed for supported contract violations, skipped/not-evaluated for unsupported validation. 3xx completed with redirects off unless assertions fail. Add outcome-unknown and omission error codes.                           |
| Pinning                                                     | Routes/metadata explicitly deferred to pinning card.                                                                                                                         | Review PUT/DELETE pin routes above; extend read responses with separate retention object and terminal-only rules.                                                                                                                                           |
| Environment GET/PATCH                                       | No GET-by-ID, concrete collection envelope, conflict/version behavior or deletion-history semantics.                                                                         | Add GET by ID; consistent items/nextCursor; revision precondition; null live history link and fail dependent pending jobs on deletion.                                                                                                                      |
| OpenAPI import/operation                                    | Library/dialect unresolved; no association field/write route on request, partial import behavior, reference policy or source sanitization.                                   | Optional operationId refers to Kurier operation UUID; expose method/path/documentOperationId; atomic generated requests, internal refs only; warnings/unsupported typed. Library card determines dialect support.                                           |
| Workflows / workflowRun                                     | No list/get/edit/delete definitions, step schemas, extraction grammar, run environment/target selection, retention or step evidence expiration.                              | Add definition CRUD and revision; steps have ordered request IDs and bounded extraction/assertion rules; freeze plans on run; add run-retention/expired-step metadata as described in model. Choose extraction grammar in workflow implementation proposal. |
| Agent token and poll/result                                 | localAgentId versus tokenId not mapped; expiry/one-agent/poll response/lease/result shapes absent.                                                                           | MVP localAgentId equals credential ID; define 20-second poll and 204 empty, 120-second lease nonce, expiry/revocation, bounded sanitized result and duplicate conflict behavior.                                                                            |
| SSE                                                         | Last-Event-ID, authentication implementation, replay, completion/expiry absent.                                                                                              | Fetch-based bearer SSE; sequence IDs and proposed lastEventId in status reads; replay then reconcile, no secret/body events.                                                                                                                                |
| MCP tools                                                   | No transport/auth pairing routes, audit schema, limit behavior, or API bridge for invocation-level auditing.                                                                 | Local stdio and separate read-only MCP credentials; propose POST/DELETE /api/v1/mcp/tokens and POST /api/v1/mcp/invocations with allowlisted tool/input; audit transaction before response. Remote transport separate.                                      |
| Project DELETE 204                                          | Active-data deletion specified; backup erasure and future DB/S3 coordination unspecified.                                                                                    | Complete active-store deletion before 204; seven-day backup residual policy plus restore deletion ledger; async 202 proposal only if artifact deletion requires it.                                                                                         |
| Health contracts versus code                                | Contracts /health and /ready envelopes differ from product /healthz and spike operational routes.                                                                            | Keep /healthz for current foundations; auth/project implementation proposes contract-shaped /health and /ready; do not treat spike HTTP endpoints as product contracts.                                                                                     |
| Error envelope versus upstream response                     | Application error and captured upstream status could be conflated.                                                                                                           | GET execution returns 200 sanitized failed record for upstream 404/500; do not make its API response status equal the target's status.                                                                                                                      |

## Approval and smallest next implementation task

Two additional policy questions remain explicit: whether protected HTTP
execution requires confirmation for plaintext transport, and whether a minimal
opaque-ID deletion ledger with seven-day backup residual lifetime satisfies
the permanent-deletion requirement. The ledger would contain only project ID
and deletion timestamp, never project configuration or evidence; its operational
location and retention require approval before backup/restore implementation.

Davian should review the topology/egress cost, protected runtime envelopes,
frozen-secret versus revocation behavior, unknown-outcome rule, schema deletion
rules, and proposed contract additions. After acceptance, synchronize the
affected contract sections and planning documentation; architecture acceptance
does not imply permission to deploy infrastructure.

The smallest implementation task is **local users/projects persistence and
ownership boundary**: add only users/projects migrations and a serialized
migration runner, a verified-identity boundary with test JWT fixtures,
GET users/me and project CRUD in the agreed envelopes, revision handling if
approved, and PostgreSQL integration tests proving cross-user denial and project
deletion. Production Cognito token verification/library integration must be
completed before exposing these routes; fixture identity is test-only, never
a production bypass. Use local Compose, no AWS deployment. The next task then
connects the approved Cognito configuration and React auth/project flow into
the presentation-ready vertical slice. Queue/worker/OpenAPI/workflow/MCP tables
are deliberately deferred until their slices need them.

## Documentation validation scope

Completed for this draft:

- Formatted the four changed Markdown files with Prettier; repository
  formatting checks passed.
- `GOCACHE=/tmp/kurier-w1-go-build npm run verify` passed formatting, ESLint,
  Vitest, TypeScript/Vite build, and Go tests/vet for all five existing modules.
  The first attempt used the sandbox's read-only default Go cache; rerunning
  with the temporary cache resolved that environment issue.
- `docker compose config --quiet` and `git diff --check` passed.
- Internal file links and balanced Markdown fences passed read-only checks.
- Mermaid's parser validated the architecture flowchart, sequence diagram,
  and ERD using temporary dependencies outside the repository. Syntax was
  checked; rendered layout was not visually reviewed.

These checks validate the draft/repository, not proposed infrastructure, SQL
migrations, encryption, authentication, or execution behavior. No SST diff,
deployment, database migration, AWS mutation, or repeated cloud spike was run.
