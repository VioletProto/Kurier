# Read-only MCP slice: proposed review packet

Status: **Accepted by Davian** at `2d8d43c`, including evidence-size limits
and credential-storage tradeoffs. Scoped implementation, API synchronization,
local/client tests and AWS deployment/activation preview with refreshed costs
are authorized. Deployment and activation remain on hold for explicit review.
The Proposed/preparation text below preserves the accepted packet; acceptance
does not claim implementation or runtime verification. Prepared 2026-10-15.
This packet authorizes no dependent implementation, external contract
synchronization, AWS preview, deployment or capability activation. New transport,
authentication, tool/schema and limit choices below remain **Proposed** even
where they refine accepted principles. Publishing this document is not acceptance.

Branch: `feat/read-only-mcp`, created from freshly fetched `origin/main`
`1672913c6a5d20a83d5f38277052c4c038d58615`. GitHub confirmed
[workflow PR #10](https://github.com/VioletProto/Kurier/pull/10) is merged at
that commit. The initial working tree was clean.
Tracking: [MCP review card](https://trello.com/c/9UBvPhBB).

## Authority, dependencies and scope

Read [accepted architecture](README.md), [ADR 0001](../decisions/0001-sst-infrastructure-candidate.md),
[ADR 0002](../decisions/0002-serverless-persistence-topology.md),
[W1 system design](w1-system-design.md), [data model](w1-data-model.md),
[defaults](w1-review-decisions.md), [remaining proposals](w1-acceptance-sync.md),
and the accepted [ownership](users-projects-contract.md),
[saved-request](saved-requests-contract.md),
[protected-input](protected-request-secrets-contract.md),
[cloud-evidence](cloud-execution-evidence-contract.md),
[pinning](evidence-pinning-contract.md),
[local-agent](local-agent-execution-contract.md) and
[sequential-workflow](sequential-workflows-contract.md) contracts.
The live [capstone proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
and [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
were fetched and read without edits. Their separate read-only credential/audit
principles agree with W1; their exact MCP routes/tools remain proposals.
Historical deployment-pending text does not supersede merged validation records.

Accepted baseline: local Go stdio MCP calls an audited hosted API bridge,
with separate hashed thirty-day read-only credentials, current ownership and
retention checks, no direct AWS/Protected/KMS access and no remote OAuth implied.
The proposal's broader contract-violation tools require future schema-validation
work; this slice cannot invent violations or report an empty list as validation
success. Listing failed executions and inspecting failed workflow steps fit
existing history/status/evidence. OpenAPI import, environments and assertions
remain outside this work.

Expose only scoped project/request discovery, execution/run inspection and
published sanitized evidence. There is no execution submit/rerun, workflow
definition editing, pin/unpin, project mutation, local-agent pairing/control,
arbitrary URL fetch, shell/file access or credential reveal through MCP.
Credential issuance/revocation is an owner-operated browser management function,
not a model tool. Internal rate/audit/availability bookkeeping does not confer
product mutation authority.

## Proposed transport, process and hosting

Prefer a separate foreground Go executable, provisionally `kurier-mcp`, in a
future `services/mcp` module. Codex starts it as a child; stdin/stdout carry MCP,
and its only network destination is an operator-configured Kurier HTTPS API
origin. No listening socket, background daemon, automatic updater or service
installer. Closing stdin, cancellation, SIGINT or SIGTERM stops bounded work,
closes connections and discards transient credential/evidence buffers.

Use the [official Go SDK](https://github.com/modelcontextprotocol/go-sdk)
as the first library candidate; exact version and supported protocol revisions
remain **Proposed**, to be pinned after compatibility checks. Do not implement
an ad hoc JSON-RPC parser or assume the newest specification is supported by
the installed Codex binary. Current official
[MCP versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning)
distinguishes per-request metadata/discovery in revision 2026-07-28 from legacy
initialization-based revisions. Propose supporting the mutually tested revision
first, recording the binary/SDK/client matrix. A legacy connection follows its
[initialize/initialized lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle);
a modern connection follows discovery/per-request version rules. Unsupported
versions fail explicitly; neither era can bypass authentication.

Advertise only tools plus the minimal discovery/liveness required by the
selected revision. No resources, prompts, sampling, elicitation, subscriptions,
background tasks or client roots are needed. Tool definitions/descriptions are
static trusted application text. Read-only annotations are hints, not security
enforcement. The [MCP tools specification](https://modelcontextprotocol.io/specification/2026-07-28/server/tools)
provides input/output schemas and structured results; validate both and use
the revision's result envelope. Proposed tool annotations are readOnlyHint=true,
destructiveHint=false, idempotentHint=true and openWorldHint=false: these tools
read existing Kurier records and never contact the captured target.

The bridge should call shared Go ownership/history/run/evidence services inside
the API after distinct MCP authentication. It must not exchange a capability
for a Cognito browser token, impersonate a browser request or forward arbitrary
HTTP methods/routes. The local binary needs no AWS credentials, SDK, bucket
access, KMS or Protected permission. Keep hosted API and target-execution
transports separate; disable redirects and ambient proxies for credentialed
API requests, verify TLS and reject URLs with credentials/query/fragment.
An explicitly selected loopback HTTP fixture mode may exist in tests only;
production pairing must not downgrade HTTPS or accept an evidence-supplied host.
The official [MCP authorization specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization)
addresses HTTP OAuth separately from stdio credential provisioning. This
application's HTTPS bridge is not a remote MCP/OAuth endpoint; private-file
provisioning is the Proposed local credential choice, not an OAuth implementation.

| Alternative                                          | Evaluation and incremental implications                                                                                                                                                                                                                                              |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Proposed first: foreground stdio + hosted bridge** | Reuses owner, published-manifest and cleanup logic. No standing MCP compute/listener; adds API calls, audit/rate metadata and scoped routes/permissions. Requires local installation and credential provisioning.                                                                    |
| Hosted Streamable HTTP MCP on existing API/Lambda    | Potential future web/remote clients; requires separate MCP OAuth/resource-server design, audience validation, HTTP transport/session review and client tests. Could retain low idle cost with immediate responses, but adds operational/auth work. No long-lived SSE Lambda assumed. |
| Local loopback HTTP MCP                              | Adds port/origin/DNS-rebinding/auth risks without removing the hosted bridge. No benefit for the initial TUI demonstration over stdio.                                                                                                                                               |
| Always-on Fargate/VM MCP                             | Adds recurring compute/ingress costs and patching; conflicts with the accepted low-idle-cost direction for this small read slice.                                                                                                                                                    |
| Direct local AWS reads or shared local-agent token   | Rejected: bypasses hosted policy/audit or joins read authority to execution/protected-input authority.                                                                                                                                                                               |

Proposed cost model, USD/month before credits/free tiers: 10,000 user-triggered
reads, including 1,000 evidence views averaging 100 KiB; no idle polling by the
server. Allow 10k HTTP API calls ($1/m), 10k ARM 256-MiB/100-ms Lambda invocations
($0.20/m requests + $0.0000133334/GB-s), 500k DDB RRU ($0.125/m) and 100k WRU
($0.625/m, including transactional audit/rate guards), 0.1 GB metadata/PITR/backup
($0.55/GB-month combined), 1k S3 GETs ($0.0004/1k), and <=1 GB delivery ($0.09/GB).
Arithmetic is approximately $0.01 API + $0.005 Lambda + $0.125 DDB requests +
$0.055 metadata + $0.001 S3 + $0.09 delivery = **$0.29**, with a **Proposed
$0.50–$1 incremental allowance** for logs, retries and cleanup. Local compute
runs on Davian's machine. No additional KMS operation/key or S3 evidence copy.

These rates are planning inputs from existing regional estimates, not newly
queried Ohio price-list entries or a measured bill. Official
[API Gateway](https://aws.amazon.com/api-gateway/pricing/),
[Lambda](https://aws.amazon.com/lambda/pricing/) and
[DynamoDB](https://aws.amazon.com/dynamodb/pricing/) pricing pages were checked
2026-10-15. Refresh region-specific S3/delivery/storage prices and actual IAM/
resource/bootstrap diff before any separately authorized deployment. More
views, pagination, rejected traffic, maximum-size captures and retries increase
cost; the allowance is not a spending cap or a current whole-system bill.

## Proposed credentials and browser authorization

Reuse Cognito verification only for owner management: verified issuer/sub,
access-token use/client/expiry, enabled application user and active stage.
Browser displays the requested read-only capability, exact project names/IDs,
expiry and local-machine disclosure before deliberate issuance. Never authorize
by email, caller ownerId or projectId alone.

Propose one active MCP credential/account, independent of the existing local
agent slot; immutable scope of 1–10 distinct owned active projects, no wildcard
or automatic inclusion of future projects. Thirty-day expiry/no refresh follows
the accepted baseline. Scope expansion/replacement requires owner revoke and
fresh issuance. Project deletion removes that project from usable scope;
removing the last scope revokes the credential. Deleting/recreating a same-name
project never transfers authority. Separate browser controls show safe current
scope/expiry/revocation metadata; no token redisplay.

Proposed management routes, not implemented:

| Route                                 | Contract                                                                                                                                                                                                                                                                                                           |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `POST /api/v1/mcp/tokens`             | Cognito owner; strict `{projectIds:[...]}` and UUID Idempotency-Key. 201 `{data:{credential,token}}` returns one opaque 256-bit random secret once. Identical issuance retry returns 200 safe metadata with token:null; lost token requires explicit revoke/reissue. Different input reuse/live slot conflict 409. |
| `GET /api/v1/mcp/tokens/current`      | Cognito owner; safe current credential or null, plus serverTime. No verifier or token.                                                                                                                                                                                                                             |
| `DELETE /api/v1/mcp/tokens/{tokenId}` | Cognito owner; empty body, idempotent 204 for own known credential, foreign 404. Revokes the capability independent of the local-agent credential.                                                                                                                                                                 |
| `POST /api/v1/mcp/invocations`        | Distinct MCP bearer middleware; strict `{tool,input}` dispatched by a fixed read-only allowlist. No arbitrary route/method/owner/header arguments. Returns the application result/error envelope described below.                                                                                                  |

Credential metadata: `{tokenId,projectIds,createdAt,expiresAt,revokedAt}` with
explicit nullable revokedAt. No credential fingerprint, ciphertext or internal
recovery/storage coordinates in client responses. Bind the server record to
purpose `mcp-read-only`, owner, stage and current recovery generation; use a
separate token lookup namespace/verifier from local-agent authentication. Store
only a verifier of the random proof, constant-time compare where applicable,
and never use the raw token as a key. Account slot/issuance receipt, credential
and project-scope links commit atomically with stage/user/project guards.
Seven-day issuance receipts and five issuance attempts/account/hour are
**Proposed**. Retry never recovers the original secret from persistence.

Each invocation strongly checks current credential purpose/proof, expiry,
revocation/version, enabled owner, stage state/generation and scope. No positive
auth cache or connection-lifetime authorization. Use fresh server time and a
five-second expiry margin before release; local time can stop early but cannot
grant access. Revocation invalidates subsequent decisions immediately through
the shared credential guard. A response already authorized before revocation
cannot be recalled. A reopened restored stage invalidates all old MCP tokens
through its new generation, requiring owner reauthorization; historical retained
evidence need not have been produced in that new generation.

Proposed CLI pairing is manual, browser-authorized token transfer to a hidden
terminal prompt. No browser credential copied to the CLI; no callback listener,
OAuth claim or token passed through model-visible MCP elicitation. Use a
controlling terminal for pairing, never protocol stdin. Display the token only
once in browser memory, clear on close/sign-out/unmount, no Web Storage, clipboard
automation or URL token. If issuance outcome/token transfer is lost, browser
metadata plus explicit revoke/reissue is the recovery path.

For this POSIX TUI slice, propose explicit `pair --persist` into a dedicated
per-user MCP credential file outside repositories: owner-only directory 0700,
regular file 0600, bounded strict parsing, verified ownership, no symlink or
shared-writable ancestor traversal, atomic same-directory write and secure
open/fstat checks against replacement races. Persist only the capability, its
safe metadata and bound trusted API origin; never browser sessions, evidence,
job inputs or variables. The existing local-agent file code is a reference,
not proof that an MCP store is secure; audit its check/open races before reuse.
No default plaintext environment/argument/config.toml token. OS keyring support
and Windows ACL persistence remain **Proposed future alternatives**; fail closed
on unsupported platforms instead of silently storing with weaker permissions.
An optional memory-only foreground mode must prompt via a real controlling
terminal before protocol service; without one it fails safely. Restart requires
reauthorization/transfer when nothing was persisted.

Local removal stops local use but does not prove server revocation. Lost files
or a compromised machine require browser Revoke. Revocation of either MCP or
local-agent credentials cannot affect the other slot or grant the other's routes.
A same-user coding process can read a user-readable private file; POSIX modes
protect other users, not a malicious process running as Davian. Stronger local
isolation/keyring-mediated access would require a separate reviewed design.

## Proposed minimal tools and schemas

All tool inputs are strict JSON objects with additionalProperties=false at
every defined object, bounded strings/enums/integers, no duplicate keys,
invalid UTF-8, trailing JSON, unknown fields or null substitutes for required
fields. IDs are opaque validated locators. Every execution/run request includes
projectId. No URL, filesystem path, token, secretRef or arbitrary filter DSL.
Expose seven static tools; all names and exact wire fields are **Proposed**:

| Tool                     | Input (optional fields marked `?`)                                    | Output data allowlist                                                                                                                                                                      |
| ------------------------ | --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `list_projects`          | `limit?,cursor?`                                                      | Only live authorized scoped `{projectId,name,createdAt,updatedAt}` items. Never list all owned projects then filter locally.                                                               |
| `list_saved_requests`    | `projectId,limit?,cursor?`                                            | `{requestId,projectId,name,method,revision,createdAt,updatedAt}` only. No URL, headers, query, body, bindings, secret descriptors or input configuration.                                  |
| `list_executions`        | `projectId,requestId?,status?,limit?,cursor?`                         | Safe execution summaries below; supports status=failed for recent-failure discovery and deleted-definition history.                                                                        |
| `get_execution`          | `projectId,executionId`                                               | One safe status/retention summary. No frozen request plan or executable inputs.                                                                                                            |
| `list_workflow_runs`     | `projectId,workflowId?,state?,limit?,cursor?`                         | Run summaries without steps/rules; deleted-definition runs may remain discoverable.                                                                                                        |
| `get_workflow_run`       | `projectId,workflowRunId`                                             | Run summary and <=10 ordered safe steps: `{stepId,position,requestId,requestRevision,state,executionId,summary,retention,evidenceExpired}`. No extraction/binding rules or runtime values. |
| `get_execution_evidence` | `projectId,executionId,view?` where view=summary(default) or complete | Current retention/availability plus a separately labeled sanitized evidence summary, or the complete published sanitized schema-1 capture when it fits the delivery bound.                 |

Execution summary: `{projectId,executionId,source:{requestId,revision},runId,
position,target,status,version,submittedAt,startedAt,completedAt,summary,
retention}`. Nullable fields stay explicit; target is cloud/local. Summary is
null or `{httpStatus,durationMs,outcome:{code,certainty}}`. Preserve known,
unknown and not_dispatched, exact nullable HTTP status and safe fixed outcome
codes, including workflow extraction/storage failures. Unknown effects remain
visible; no tool suggests automatic rerun. Do not return local-agent IDs/origins
or mutable source configuration merely because a browser DTO contains them.

Run summary: `{projectId,workflowRunId,workflowId,workflowRevision,target,state,
version,submittedAt,startedAt,completedAt,normalExpiresAt,currentPosition,
failedPosition}`. Steps link execution locators for separate evidence calls.
Expired sibling steps return evidenceExpired=true and no retained body; a pinned
sibling does not authorize their evidence. Retention projection is
`{normalExpiresAt,pinned,pinnedAt,evidenceAvailability,state,effectiveStatus}`,
without browser action hints, storage sizes/checksums or pin mutation authority.

Successful application envelope:
`{data:{schemaVersion:1,serverTime,items,nextCursor}}` for lists and
`{data:{schemaVersion:1,serverTime,execution|run|evidence}}` for singular reads.
MCP structuredContent contains that envelope and content contains its serialized
JSON text for clients needing text compatibility. Both carry exactly the same
safe data; count both plus JSON-RPC framing against the response limit. Keep
the selected protocol revision's required outer fields separate from these
application fields. Concrete JSON Schemas/golden vectors are implementation
deliverables after acceptance, not an executable OpenAPI specification here.

## Every-read authorization, evidence lifetime and audit

Each data call follows this **Proposed** release protocol, extending accepted
owner/RET fencing to MCP credentials:

1. Authenticate the separate capability and validate active stage/current
   generation, enabled user and scope before resource detail/errors. Foreign,
   out-of-scope, deleted and missing resource locators all return fixed404.
2. Strongly hydrate project owner/state/version/deletionEpoch and authoritative
   child records. Indexes only discover candidates. Cross-project IDs must
   agree with their parent. Saved requests must still be active; definition
   deletion does not erase otherwise retained execution history/run context.
3. For terminal execution reads check current live RET and
   `(pinned || normalExpiresAt > decisionTime)` plus publication/availability.
   Expired/deleting evidence is inaccessible even if SNAP/S3 physically remain.
   For run context apply the existing run lifetime and <=10 strong step-RET
   checks. Retained context cannot restore an expired sibling body.
4. Evidence complete/summary views both read only a published SNAP/object;
   reuse bounded object read, exact stored byte count/checksum validation and
   supported schema validation. No UPLOAD ticket, orphan, prepared candidate,
   Protected lookup/decrypt or reconstruction from runtime variables. Missing,
   corrupt or unsupported capture fails closed; availability bookkeeping may
   change, immutable evidence cannot.
5. After fetching/projecting/size checks, freshly revalidate authority, time and
   availability. Commit the bounded sanitized invocation audit with conditional
   stage, enabled-user, credential and relevant project/resource/RET/run guards
   before returning any data. Never rely only on a pre-fetch credential check.
   Conflicts require bounded fresh reads, not reuse of a stale authorization.
   Failure to persist audit denies delivery. Expiry is checked again immediately
   before local serialization; discard pending output on auth failure.

Return an internal, non-model-visible release deadline bounded by credential
expiry and the earliest relevant unpinned RET/run-context expiry, with at most
five seconds from hosted decisionTime. The local bridge rejects late results
using elapsed monotonic request time and a conservative server-time bound,
including clock-uncertainty margin; it must not serialize an expired capture
because a network response or stdout writer stalled. Abort a blocked output at
the deadline. This narrows delayed delivery but cannot retract an already
released frame. Credential revocation/deletion is ordered at the conditioned
hosted decision, not claimed to recall data during transit.

The hosted decision is the release boundary; it cannot recall bytes already in
transit or copied into Codex context/history. No promise of revocation purging
the client's transcript. All future calls reauthorize; process memory buffers
are discarded when complete/cancelled. No persistent local evidence or positive
authorization cache, CDN cache, presigned URL, browser-token fallback or offline
mode. Hosted responses are Cache-Control:no-store. Do not return a previous
successful payload after 401/404/503 or a failed refresh. Static tool schema
caching is allowed; it conveys no data authority.

Audit records contain only schema/version, server-generated correlation ID,
credential ID, tool enum, authorized opaque locator IDs, time and fixed outcome
code. No bodies, names, URLs, raw input JSON/cursors/tokens, credential hashes,
runtime values or ciphertext. Scoped successful reads audit within the project;
account-only list/denial audits remain account-scoped with unauthorized locators
omitted. Project audit expiry follows project deletion and at most 90 days;
account audit retention 90 days is the accepted baseline. Logical expiry is
authoritative; bounded scheduled cleanup removes records physically.
Unauthenticated traffic with no verified owner receives no project audit or
identity lookup based on caller claims; retain only bounded safe rejection
counts. All authenticated data invocations require their guarded audit.

List pages have one conditioned release decision per bounded unit. Propose
max 25 execution/request items, max 5 run summaries when per-run RET guards are
needed, and <=10 projects. Stop early with a cursor whenever the actual audit
transaction approaches 80 actions/2 MiB/128 KiB per item or response-size limit;
deduplicate shared guards, never split one audit decision across unguarded
data. A single <=10-step run detail must fit one release decision. Limit resource
churn conflicts to three fresh attempts within the hosted deadline, else 503.
This must be measured against actual transaction builders, not assumed from
the requested page count.

No pin expiry extension, pin creation, cleanup cancellation, native TTL authority
or expired-evidence resurrection. Project deletion overrides pins and scope;
its gate conditions prevent new project audits/scope writes after acceptance.
Cleanup must recognize MCP audit/scope links, revoke/remove scopes through
credential CAS, preserve other project scopes and drain all known project
children before the existing strong completion proof. Race-safe account
credential/lookup/receipt/rate cleanup cannot remove a replacement slot.
Unknown schemas/fields or uncertain writes keep cleanup pending.

After acceptance, design a coordinated MCP capability/version and record-schema
rollout so API, ownership/execution readers and both cleanup paths recognize
these records before activation. Exact marker/migration remains **Proposed**;
do not casually bump savedRequests 6 or reset generation/counters. No new
execution, storage-reservation or pin charges are created by reading. Preserve
the accepted backup-restore exception: reconciliation and new generation precede
reopening; earlier restores may recover deleted records. Do not claim deletion
survives every backup restore or silently reenable restored MCP credentials.

## Proposed bounds, evidence handling and errors

| Boundary          | Proposed initial value/behavior                                                                                                                                                                                                                                                                                                                                                                            |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Collection page   | Default25/max 25; projects capped 10 and run summaries default/max 5; smaller pages valid. No total/offset. At most5 underlying query pages and125 hydrated candidates/call, further reduced by guard budget.                                                                                                                                                                                              |
| Cursor            | <=8 KiB, 24-hour expiry; authenticated opaque continuation binds credential ID/version, scope, owner, stage/generation, tool, project, filters, high-water and position. No internal table/index/object locations in decoded client content; encrypt/authenticate a public-ID continuation or use a bounded server-side handle. Existing base64 signed database-key cursors cannot be forwarded unchanged. |
| Input             | <=16 KiB complete MCP request/hosted invocation body; strict schema and cursor included. No compressed input.                                                                                                                                                                                                                                                                                              |
| Output            | <=256 KiB complete UTF-8 JSON-RPC response, including structured/text duplication, escaping and framing. Hosted envelope and fully encoded Lambda response independently measured within existing 4/6 MiB limits.                                                                                                                                                                                          |
| Complete evidence | <=96 KiB published capture as an initial precheck AND actual complete serialized MCP response <=256 KiB. Larger or heavily escaped captures return evidence_too_large; use explicit summary view. No silent byte truncation or partial JSON.                                                                                                                                                               |
| Metadata strings  | Names <=100 code points inherited from saved definitions; cap serialized metadata/arrays before release. URLs/headers/body occur only in a complete sanitized published capture, never catalog/status DTOs.                                                                                                                                                                                                |
| Time/concurrency  | One in-flight hosted data request/process, queue at most4 then rate_limited; hosted deadline8s within existing API timeout, local deadline10s, credential expiry margin5s. No internal retry loop beyond3 fresh conditional attempts.                                                                                                                                                                      |
| Rate              | Hosted60 invocations/minute/credential, burst 10; account 120/minute and 5 issuance attempts/hour; evidence complete 10/minute/credential plus16 MiB/minute/account returned bytes. Distributed atomic enforcement; limits apply across processes.                                                                                                                                                         |
| Polling/retry     | No server background polling. Client status requests >=10s apart/object, one in flight; fixed Retry-After on 429/503, exponential backoff up to 60s. No automatic 401/404 retries or auto-reauthorization.                                                                                                                                                                                                 |

Lists may be empty with a non-null cursor; only null completes pagination.
Post-filter/hydration and guard/byte budgets advance only through evaluated
candidates without skipping undisclosed eligible items. Cursors are continuation,
never authority; every page checks current scope/generation/retention, expires
normally and is invalid after revoke/reissue. Index lag means eventual discovery,
not snapshot isolation. No automatic traversal of every page by the server.

Evidence `view=complete` returns the complete verified immutable sanitized
capture, unchanged, plus current retention and delivery metadata. Its original
body kind/text/omissionReason and redaction policyVersion/applied survive exactly.
If it cannot fit, return a small typed error instead of modifying the capture at
GET time. This preserves the accepted immutable-evidence contract.

Explicit `view=summary` returns a **derived summary**, labeled view=summary,
complete=false and omissionReason=mcp_summary_view, with status/outcome,
timestamps/timings, exact nullable HTTP status, original body kind and fixed
capture omission reasons, redaction flags and current retention. No URL, header
values or body text. It never masquerades as the full capture. Complete responses
declare complete=true, truncated=false; summary responses declare truncated=false
and explicit omittedFields for request URL/headers/body and response headers/body.
Never describe unsupported/binary/omitted content as an empty successful body.
No chunks, previews, range offsets, download links or raw artifact export in the
first slice; bounded excerpts can be a later reviewed extension if required.
This deliberately limits full MCP inspection of large captures; existing owner
browser evidence remains subject to its accepted delivery limits.

Application errors use the existing stable shape with an optional bounded
retry field: `{error:{code,message,details:[],requestId,retryAfterSeconds}}`.
Messages are fixed server text, requestId is generated and contains no caller
payload. Proposed error table:

| HTTP bridge status / code | Meaning                                                                                                              |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| 400 invalid_request       | Invalid tool input, query/cursor or unknown field; no raw argument echo.                                             |
| 401 unauthenticated       | Missing, wrong-purpose, revoked, expired or old-generation MCP credential; no reason distinguishing token existence. |
| 404 not_found             | Foreign/out-of-scope/deleted/missing resource or logically expired evidence/context.                                 |
| 409 conflict              | Authorized evidence requested before terminal publication.                                                           |
| 413 payload_too_large     | Request exceeds transport/input cap.                                                                                 |
| 413 evidence_too_large    | Complete view cannot fit; fixed message directs explicit summary view, no partial payload or internal size/location. |
| 429 rate_limited          | Aggregate/process/byte budget reached; bounded Retry-After mirrored as retryAfterSeconds.                            |
| 503 evidence_unavailable  | Published artifact missing/corrupt/unknown schema; no storage exception detail.                                      |
| 503 service_unavailable   | Recovery/closed stage, incompatible capabilities, audit failure, dependency timeout or unresolved guarded read.      |
| 500 internal_error        | Unexpected sanitized failure.                                                                                        |

Tool-domain failures return isError=true and the same small error in structured
and text content, using a schema union for success/error. JSON-RPC parse,
invalid-method and invalid-params errors follow the selected protocol revision
(including its version error) and contain no submitted values. Unknown tools
are rejected by both local and hosted allowlists. Gateway errors/timeouts that
do not use the Go envelope become fixed local service_unavailable; never forward
HTML, AWS messages, raw HTTP headers or URL-bearing network exceptions.

## Secret containment and untrusted evidence

Catalog/status projection is an explicit allowlist, not serialization of the
internal row or browser DTO. No protected input, runtime variable, secret
descriptor/reference/value, encrypted envelope, nonce/wrapped key, private
digest/checksum, bucket/object key, presigned URL, table key, queue identifier,
internal storage path or AWS error appears in MCP data or diagnostics. Only the
owner pairing UI/hidden CLI prompt handles the read capability itself. Never
print it on stdout/stderr, in command arguments, configuration or audit.

The complete evidence path reads only the existing sanitized publication format;
it never decrypts data or consults raw execution buffers. Preserve recognized
Authorization/Proxy-Authorization/Cookie/Set-Cookie/credential-name and
user-designated redaction, reflected-input sanitization, workflow source/rendered
taint and unsupported-content omissions. Reject unexpected schema fields or
unsafe capture structure rather than widening the response automatically.
MCP cannot recover historical plaintext to rerun redaction; producer sanitization
is the primary boundary. No claim of arbitrary embedded-secret/transformation
detection. If the immutable capture cannot safely be disclosed, deny/mark
unavailable through existing policy, never reconstruct or quietly rewrite it.

Evidence, names and upstream error text are untrusted data even after redaction.
Strings resembling system prompts, shell commands, MCP methods, credential
requests or links remain inert string values. Do not evaluate templates/scripts,
follow URLs, resolve remote JSON references, fetch embedded resources, execute
returned commands or merge response text into tool descriptions/server
instructions. No upstream-provided tool definitions, callbacks or authority
claims. Static descriptions identify evidence as untrusted and tell clients to
interpret it as captured data; this is defense in depth, not proof a model will
ignore every prompt injection. Server allowlists and hosted credentials enforce
authority regardless of model behavior. JSON escaping/control handling must
prevent protocol-line and terminal-control injection.

Diagnostics use fixed codes, latency/counts and generated correlation IDs only;
no request/response dumps, headers, URLs, token prefixes, bodies, credential-file
contents or stack traces. stderr can be captured by Codex, so it is equally
subject to containment. Error handling must not attach a raw wrapped Go error.
Secret tests inspect memory-held canaries and report only booleans/counts; failed
assertions must not print secret-bearing diffs. Best-effort buffer clearing in Go
does not guarantee garbage-collector erasure or client transcript deletion.

## Proposed foreground startup and Codex TUI connection

These are future commands/configuration for acceptance review; no MCP binary,
pairing page or bridge exists in this preparation commit. Official
[Codex MCP documentation](https://developers.openai.com/codex/mcp/), fetched
2026-10-15 (currently redirects to ChatGPT Learn), documents local command
servers, `codex mcp add`, `codex mcp list`, `/mcp` and command/args configuration.
The [stdio specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio)
requires newline-delimited UTF-8 JSON-RPC on standard streams, stdout containing
only protocol messages, and prompt exit on stdin EOF.
Local read-only inspection found `codex-cli 0.162.1`; `codex mcp add --help`
confirms the command-after-`--` registration shape. This is not a client
connection or proof of protocol-version compatibility.

After accepted implementation, build/install a reviewed binary at an absolute
path, recording its commit/checksum. Proposed example assumes Davian's binary
is `/home/king/.local/bin/kurier-mcp` and private credential file is
`/home/king/.config/kurier-mcp/credential.json`; no secret is in either path.
Pair interactively in a separate terminal after explicit browser issuance:

```sh
/home/king/.local/bin/kurier-mcp pair --api-origin https://YOUR-TRUSTED-KURIER-API --persist
```

The program reads the capability through a hidden terminal prompt, verifies
scope with the hosted bridge and stores only after local consent. Never paste
a token into a shell command, environment assignment, chat, AGENTS.md or TOML.
The placeholder origin is operator input, never copied from evidence. Proposed
foreground entry point is `kurier-mcp serve --credential-file <private-file>`;
manual execution waits for protocol input and is not a health page or daemon.

Register the executable so Codex launches its own foreground child:

```sh
codex mcp add kurier -- /home/king/.local/bin/kurier-mcp serve --credential-file /home/king/.config/kurier-mcp/credential.json
codex mcp list
codex
```

Equivalent user-level `~/.codex/config.toml`:

```toml
[mcp_servers.kurier]
command = "/home/king/.local/bin/kurier-mcp"
args = ["serve", "--credential-file", "/home/king/.config/kurier-mcp/credential.json"]
startup_timeout_sec = 10
tool_timeout_sec = 15
enabled_tools = ["list_projects", "list_saved_requests", "list_executions", "get_execution", "list_workflow_runs", "get_workflow_run", "get_execution_evidence"]
```

Use a new TUI session, inspect `/mcp`, then ask to list scoped projects and
inspect one known failed execution/run. The tool allowlist is defense in depth;
the server/API still reject writes. `codex mcp login` is for an OAuth-capable
server; it is not this proposed manual stdio pairing flow. A process manually
started in another terminal does not attach to Codex's stdio pipes.

`serve` has no banner, prompts, build/download output or diagnostic on stdout.
Pair/help commands run separately. Never combine stderr with stdout (`2>&1`),
tee protocol data into a log or run a verbose shell wrapper. Only bounded fixed
diagnostics go to stderr. On missing/unsafe/expired credentials, emit a safe
failure and require owner action; do not block protocol stdin awaiting a token
or silently reuse local-agent credentials. On EOF/signal cancel reads and exit;
restart performs fresh hosted checks, not cached authorization. Installed-client
command/config support and credential-safe behavior require the real integration
test below; this document does not claim a successful TUI connection.

## Planned tests and evidence required after acceptance

All MCP-specific checks below are **planned**, not passed by preparing this
document. Use deterministic clocks/barriers and disposable in-memory secrets;
separate unit/fault/Local/fixture results from real client and deployed evidence.

| Area                                       | Required scenarios                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Protocol and process                       | Real child-process pipes with the pinned Go SDK; selected revision discovery/initialization, list/call/schema validation, unsupported versions/methods, malformed/duplicate/deep JSON, request IDs, line injection, cancellation, deadlines, oversized input/output, bounded queue, EOF/signals/restart. Parse every stdout line as MCP; inspect stderr for canaries. Never treat an Inspector-only test as Codex integration.                                                                                                                                                         |
| Ownership/scope                            | Two Cognito owners, two projects, every tool, forged nested IDs, deleted definitions versus retained history, deleted project/tombstone, disabled owner, tampered/reused/expired/wrong-scope cursor; no unscoped list or internal locations in cursor plaintext. Index lag/candidate hydration and empty pages must not leak or skip eligible items.                                                                                                                                                                                                                                   |
| Credentials/revocation                     | Browser issuance/one-time display/no browser persistence, duplicate/lost issuance response, input-key conflict, simultaneous last slot, expiry boundaries and five-second margin, scope replacement, missing/wrong-purpose token, agent↔MCP↔Cognito confusion. Revoke/disable/generation change before fetch, during S3, before audit commit and before local output. Subsequent calls deny; old process restart cannot revive credentials.                                                                                                                                            |
| Containment and prompt injection           | Protected header/query/body/JSON inputs, workflow source/rendered values and encoded reflections absent from every tool/error/stdout/stderr/audit/config/file/transcript artifact under test. Credential-named containers and unsupported/binary captures follow existing omissions. Malicious prompt/tool/URL/terminal strings cannot trigger network, shell, file, sampling or tool-registration effects. Unknown evidence schema fails closed.                                                                                                                                      |
| Retention/publication                      | Queued/unpublished/orphan denied; published success/failure/unknown retained accurately; missing/corrupt/oversize objects and mismatched manifests; before/at/after expiry; pinned beyond expiry then unpin, deleting RET, expired sibling with pinned run context, definition deletion. No body fabrication, pin mutation or immutable capture modification.                                                                                                                                                                                                                          |
| Cleanup and races                          | Revocation versus read/audit; pin/unpin/expiry/project delete versus S3 fetch and final audit; run context cleanup versus RET guards; generation recovery; audit write versus deletion completion; scope cleanup versus replacement token. Interrupted multi-chunk drain, expired account audit/rate/lookup/receipt cleanup, unknown records and lost commit ACK must preserve completion fences and never resurrect access.                                                                                                                                                           |
| Limits/errors                              | Exact/one-over complete request, cursor, guard/action/item/transaction and serialized response caps; Unicode/quotes/backslashes/control/HTML escaping plus structured/text duplication. Distinguish summary omissions from capture omissions and evidence_too_large. Aggregate multi-process rate/byte enforcement, Retry-After, no leaked dependency errors or partial successful data on audit failure.                                                                                                                                                                              |
| Storage                                    | File/directory modes and ownership, hostile symlink/replacement/open races, insecure ancestors, truncated/oversized file, atomic replacement, no secret argv/env/output, missing controlling terminal, unsupported platform fail-closed, lost file/browser revoke and offline local deletion with revocation unconfirmed.                                                                                                                                                                                                                                                              |
| Genuine Codex TUI                          | Launch the compiled binary through Davian's installed Codex using the documented command/config and record both versions/commit. Browser-authorize disposable scoped MCP credential; `/mcp` discovery, seven-tool list, actual list/status/run/complete+summary evidence reads, hostile data, oversized capture, foreign-scope denial, revoke while process remains open, subsequent denial and restart denial. Inspect safe protocol/client output and ensure zero execution/pin/project/control mutations. Remove only the disposable config/credential and revoke server authority. |
| Deployed validation, separately authorized | Real Cognito management, HTTPS distinct auth routing, DDB atomic audit/revoke/rate guards and index lag, S3 integrity, IAM boundaries, minute cleanup and genuine browser/TUI flow. Actual restore/PITR, elapsed 30-day expiry, clock skew, negative IAM and metered costs remain separate operational checks.                                                                                                                                                                                                                                                                         |

After implementation authorization, run repository verification plus focused
Go race/DynamoDB Local and client-process tests. The genuine TUI test can first
use an authorized local fixture backend, labeled accordingly; real hosted
credentials/evidence require a separately approved deployment and distinct
record. Protocol simulators, fixture JWTs and fake clocks do not establish
actual Codex/AWS/Cognito/elapsed-time behavior.

## Preparation record and decisions requested

Read relevant Trello setup, contract-review/checklist, architecture, saved/
protected, redaction, pinning, local-agent and workflow dependency records.
Board Doing limit is 3; Doing was empty before creating the MCP review card.
Workflow implementation is Done; publication-incident attribution and operational
follow-ups remain To Do and are not claimed resolved. Broader API-contract
review stays open. No pre-existing MCP slice card was returned.

Inspected current Cognito verification, ownership gates/pagination, request
secret descriptors, local-agent token/storage, execution history/detail/evidence,
workflow run/RET and maintenance code. In particular, `execution/worker.go`
Evidence verifies manifest length/checksum and repeats Detail after fetch;
`execution/store.go` Detail/view applies RET/project guards; `workflows.go`
RunDetail checks run/step lifetime. These are reusable foundations, not MCP
credential/audit enforcement. Existing signed cursors encode internal database
keys and must not be exposed unchanged. Existing cleanup validators do not
yet recognize MCP records. No source/infrastructure/external contract changed.

Preparation checks passed: repository Prettier/gofmt formatting, ESLint,
52 frontend and 10 infrastructure tests, TypeScript/Vite production build,
tests/vet and builds across all six existing Go modules. `npm run verify`
passed through the frontend build, then its Go phase hit the sandbox's read-only
Go cache; the approved outside-sandbox `npm run verify:go` rerun and all-module
builds passed. These are baseline checks, not proposed MCP behavior, Local race,
browser, genuine Codex integration or AWS validation.
Document checks also passed: balanced code fences, parsed TOML configuration,
seven-tool table, existing local link targets and clean diff whitespace.
The Trello preparation item completes only after verified commit/push/readback;
acceptance, dependent implementation/synchronization, MCP tests and deployment
items stay open. Preparation alone is not an MCP runtime test.

Review decisions (all **Proposed**):

1. Foreground Go stdio + hosted audited bridge; official Go SDK candidate,
   client-tested protocol revision(s), no remote HTTP/OAuth or standing service.
2. Separate browser-issued thirty-day capability, one MCP slot/account,
   <=10 immutable project scopes, explicit revoke/reissue and private POSIX
   file pairing for TUI; accept the same-user process storage limitation.
3. Seven named tools, catalog/status allowlists, scoped bridge/management wire
   shapes, per-read guarded audit and cleanup-compatible credential/scope records.
4. Proposed page/transaction/rate/byte/deadline limits and stable errors;
   complete capture only when <=96 KiB and actual MCP response<=256 KiB,
   explicit derived summary otherwise, no excerpt/chunk subsystem initially.
5. Current owner/generation/RET/run/pin/deletion checks at release, no data/auth
   cache, untrusted-data containment, coordinated later capability rollout,
   planned race/client tests and $0.50–$1 monthly incremental planning allowance.

Record explicit acceptance and any amendments before dependent work. External
API-contract synchronization and AWS deployment remain on hold; a later request
must state their authorized scope. No AWS mutation or SST command was run to
prepare this packet.

## Accepted implementation continuation

Davian accepted this packet at `2d8d43c`, including initial evidence limits and
private-file tradeoffs, and authorized the scoped implementation, native API
contract synchronization, planned local/client tests and AWS preview/cost
preparation. The earlier preparation holds above are historical. Deployment
and capability activation remain held for explicit review.

Implementation pins official Go SDK 1.8.0 and MCP 2025-11-25, verified with the
installed Codex 0.162.1 MCP client. Concrete code uses savedRequests7 plus
mcpReadOnly1 as the later compatibility gate, retaining old-stage6 compatibility
until deliberate activation. Internal release deadlines, strict per-tool DTOs,
conditioned audit reads, encrypted public-ID cursors and dedicated credential
cleanup preserve this contract's boundaries. Browser authoring initially scopes
the selected project; the API supports1–10 projects. See the
[implementation/setup/validation and deployment review](../development/read-only-mcp.md)
for exact commands, native contract synchronization, actual evidence, refreshed
costs and unperformed operator/deployed checks. Acceptance is not proof of those
unperformed checks; no AWS deployment/activation occurred in this continuation.
