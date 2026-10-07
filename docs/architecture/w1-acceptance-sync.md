# W1 acceptance and source synchronization

Davian Hernandez accepted baseline commit `1fba942` and its documented defaults
on 2026-10-06. [ADR 0002](../decisions/0002-serverless-persistence-topology.md)
is **Accepted**; SST ADR 0001 and historical spike evidence remain unchanged.
The original architecture acceptance authorized documentation synchronization
only, not product code, deployment, AWS mutations or full schema migration.
The subsequently authorized local users/projects implementation is recorded below.

## Sources read and synchronized in place

- [API Contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit):
  ownership/auth, resource revisions, secret writes, submission/outbox, evidence,
  retention/pins, polling, import/workflow limits, agent receipts, MCP, health,
  error behavior, open choices and implementation sequence.
- [Capstone Proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit):
  technology bullets, affected MVP requirements and W2/W9 timeline cells now
  use SST/Lambda/HTTP API/DynamoDB/private S3, not product CDK/Fargate/RDS/SSE.
  Template instructions, unrelated narrative, image, stretch scope, faculty
  fields and the 11-row/four-column timeline are preserved.
- [Architecture card](https://trello.com/c/jbm6BuxP/2-w1-finalize-architecture-and-data-model):
  acceptance, updated design/data model and validation boundaries recorded;
  architecture documentation task complete, not implementation.
- [Contract review card](https://trello.com/c/b5LdvuTo/3-w1-finalize-kurier-api-contracts-v01):
  accepted behavior synchronized; keep Doing until the genuinely new wire/
  library recommendations below are agreed and final affected schemas checked.
- [Cognito card](https://trello.com/c/WCKm9Ec1/12-w1-w2-configure-cognito-authentication-and-verify-email-delivery)
  and [pinning card](https://trello.com/c/gH2vGh9p/11-w4-implement-execution-retention-and-pinning):
  accepted DynamoDB/auth/retention protocols and future test criteria recorded;
  both remain To Do, no implementation claimed.

Trello returned no assigned members on these cards. Davian explicitly authorized
treating `Owner: Davian Hernandez` description lines as assignment, and separately
authorized the contract-review card. Other unassigned cards and historical
completed spike cards are unchanged. This leaves stale PostgreSQL wording in
the unassigned foundation/backlog cards: obtain assignment/authorization before
synchronizing them. No Trello membership was changed.

## Accepted synchronization details

### Subsequent focused users/projects acceptance

Davian accepted the [users/projects packet](users-projects-contract.md) and
authorized a dedicated local implementation branch from this documentation
baseline. The live API contracts now record project version/ETag, quoted
If-Match with **428 missing / 412 stale**, validation/error mapping, lazy verified
identity provisioning, GSI1 pagination, no automatic creation retries, safe
deletionOperation DTO/minimal tombstone fields and initiating-version repeat
DELETE. Full both-table/S3 cleanup remains planned; local empty-project cleanup
only is authorized and implemented. Setup/tests and unvalidated cloud behaviors
are [explicit](../development/local-ownership.md).

The overall contract-review card remains **Doing**. Its existing description is
preserved; an acceptance checklist supersedes only the users/projects Proposed
labels. The connector rejected a long description update and its separate
comment server required reconnection, so the supported checklist surface was
used without replacing unrelated card content. Workflow extraction, OpenAPI
libraries, local-result schemas and other later interfaces remain proposed,
not prerequisites. The capstone proposal, other cards, accepted SST decision
and historical spikes are unchanged by this task.

Project-scoped execution/rerun/agent-result/workflow-run locators replace global
opaque-ID lookups. Collection limits/cursors, seven-day Idempotency-Key receipts,
frozen configuration/revision checks, encrypted current-secret reruns and the
64 KiB saved/frozen/resolved-body caps are explicit. Mutable status/RET and live
links are separate from immutable evidence/frozen IDs; failed upstream 404/500
evidence GET remains 200. Pin PUT/DELETE returns 200 RET, not a new capture.

SSE is replaced by immediate polling with versions/serverTime and stop/backoff
rules. Local intent commits before executable poll response; no start route or
ambiguous regrant. First result requires live lease; retained original-credential
ACK can succeed after lease expiry without a write, never after revocation.
Cloud claim/fencing and fast/scheduled identifier-only sends never replay
ambiguous external HTTP. Workflows freeze bounded plans and atomically commit
outputs/snapshot/step/run/receipt/successor intent. Imports stage bounded writes
before ready instead of pretending 100 operations fit one transaction.

Project DELETE is 202 with immediate denial and durable cross-store cleanup,
including uncertain/late PUTs. Normal-operation tombstones do not promise backup
anti-resurrection. Actual restore timestamps/warnings, reconciliation, unavailable
missing evidence and suppression of all restored nonterminal work are explicit.
Historical `/healthz` is documented with its actual foundation response
`{"service":"api","status":"ok"}`, outside `/api/v1` envelopes; spike routes
are not product routes. No executable repository OpenAPI exists yet.

## New wire recommendations — proposed for Davian Hernandez's review

Except for the subsequently accepted users/projects entries explicitly marked
below, these resolve previously unspecified spellings/shapes, not accepted architecture
guarantees. Do not implement them until reviewed; the contracts carry the same
proposed recommendations. Library prototypes remain separate authorized work.

| Gap                | Concrete proposed contract                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| History            | `GET /api/v1/projects/{projectId}/executions?requestId=<frozen-id>&status=failed&limit=25&cursor=...`; frozen filter does not require a surviving live request.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| Status             | `GET /api/v1/projects/{projectId}/executions/{executionId}/status`; `POST /api/v1/projects/{projectId}/execution-status` with `{executionIds: [...]}` (<=25). Return `{items:[{executionId,status,version,retention}],serverTime}`; deleted/expired IDs have per-item unavailable markers without existence disclosure across owners.                                                                                                                                                                                                                                                                                                                                                                                  |
| Revision writes    | Projects now accepted: `If-Match: "<version>"`, 428 missing, 412 stale; see [slice contract](users-projects-contract.md). Other definition revisions/submission fields remain proposed; submission idempotency conflicts remain 409.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Sensitive fields   | Read `{name,enabled,sensitive:true,masked:true,secretRef}`; write uses `secretWrite:{action:"set",value}` / `{action:"preserve",secretRef}` / `{action:"remove"}` / `{action:"secretRef",secretRef}`. Non-sensitive writes retain `value`. No raw `value` on sensitive reads. URL/query/header locators and JSON body RFC 6901 pointers identify sensitive paths; text body is wholly sensitive or non-sensitive, not arbitrary string offsets.                                                                                                                                                                                                                                                                        |
| Deletion           | Operation GET/DTO, minimal tombstone metadata and initiating-version repeat DELETE are now accepted in [the slice contract](users-projects-contract.md). Full cross-store cleanup remains planned; local implementation completes empty projects only.                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Workflow CRUD      | GET collection/detail, PATCH and DELETE under `/api/v1/projects/{projectId}/workflows[/{workflowId}]`; revision preconditions; run body `{workflowRevision,target,environmentId,environmentRevision,localAgentId}` with per-step overrides frozen. Each step `{requestId,requestRevision,environmentId,environmentRevision,target,extract:[{name,path,sensitive}],assertions:[]}`.                                                                                                                                                                                                                                                                                                                                     |
| Extraction grammar | JSON-path subset: root `$`, dot properties and nonnegative array indexes only; no wildcards, recursive descent, filters or evaluation. Exactly one primitive value/output, missing/type mismatch fails step. Select and test the library separately; escaping/property-name rules must be finalized with examples before acceptance.                                                                                                                                                                                                                                                                                                                                                                                   |
| Import             | POST returns 202 `{importId,state:"preparing",detectedVersion,warnings:[]}`; GET reports preparing/ready/failed and unsupported features; bounded readiness publication. Propose initial OpenAPI 3.0.x subset with local refs and explicit rejection of unsupported 3.1 features, subject to parser/JSON Schema prototype review; no library accepted yet.                                                                                                                                                                                                                                                                                                                                                             |
| Pairing            | Agent token POST `{projectIds:[...]}` returns `{tokenId,token,expiresAt}` once; one active credential/account. MCP uses distinct POST `/api/v1/mcp/tokens`, DELETE `/api/v1/mcp/tokens/{tokenId}`, and POST `/api/v1/mcp/invocations` `{tool,input}` with credential scope and pre-response audit. All tool inputs carrying an execution/run require projectId.                                                                                                                                                                                                                                                                                                                                                        |
| Local result       | Envelope `{schemaVersion:1,leaseId,fence,nonce,evidence,runtimeOutputs:[{name,value,sensitive}]}`; route supplies project/job IDs. Runtime values travel only in the authorized TLS channel and become encrypted bundles, not evidence/logs. Normalize optional collections to `[]`, optional scalar fields to explicit `null`; reject unknown/duplicate keys, unsafe numbers and unsupported encodings. RFC 8785 canonical HMAC includes domain/schema/project/job/lease/fence/nonce identity plus evidence timings/body text and outputs; excludes server timestamps/encryption randomness. ACK `{executionId,acceptedAt}` reveals no digest/values. Exact evidence schema must still be agreed before runtime work. |
| Queue              | Fixed `{projectId,jobId,outboxId}` for both fast and scheduled sends; all authority/configuration/generation comes from the strongly checked database. No advisory authority/userId.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Readiness/errors   | Keep public `/healthz` foundation separate; propose private `/ready` with bounded safe readiness and 200/503. Define capture codes `upstream_http_error`, `execution_timeout`, `upstream_network_error`, `response_limit_exceeded`, `assertion_failed`, `schema_failed`, `extraction_failed`, `dispatch_unknown`, `recovery_interrupted`; they are stored outcomes, not API upstream status forwarding.                                                                                                                                                                                                                                                                                                                |

Beyond the separately accepted users/projects packet above, no new product
feature, expanded request storage, remote MCP transport,
immediate universal JWT revocation or stronger backup deletion policy is accepted
here. The remaining shape recommendations stay proposed even where behavior is accepted.

## Validation and handoff

Documentation validation and external readback are separate from future runtime
tests. The accepted decision sheet's fault/race/restore/security tests remain
unperformed; historical spike tests do not cover Lambda/DynamoDB/S3 protocols.
At baseline synchronization, forecasts were $3.17 light / $14.14 heavy with the original usage/rate assumptions,
rotation and bootstrap caveats, not a hard cap or measured usage.

Checks actually run for acceptance synchronization:

- Prettier on all eight changed Markdown files; repository `GOCACHE=/tmp/kurier-w1-go-build
npm run verify`: format/gofmt, ESLint, Vitest (one existing frontend test),
  TypeScript/Vite build and Go tests/vet for all five existing modules passed.
- `docker compose config --quiet`, Mermaid parser (three diagrams), balanced
  fences, 43 internal file/heading links and `git diff --check` passed. Compose
  still describes the historical local PostgreSQL foundation, not DynamoDB.
- All 35 public AWS/SST/IANA links returned HTTP 200 after network-authorized
  retry. The six authenticated source links were checked through current Docs/
  Trello connector reads instead, not anonymous HTTP checks.
- Both Docs were fully read before edits and read back afterward. Original
  paragraph styles/list identities, proposal image and 11-by-four timeline
  topology were preserved; changed technology labels retain bold labels rather
  than accidentally bolding whole descriptions. The new Proposed wire section
  has native heading/body structure, not a Markdown table pasted into Docs.
- Both PDF exports were rasterized and all 18 pages of each were inspected.
  Existing template page breaks and narrow timeline columns remain; no live
  browser-canvas or pixel-perfect layout claim. Native text/structure readback,
  not PDF rendering, establishes semantic synchronization.
- Four authorized Trello card descriptions were read back byte-for-byte;
  architecture is Done/complete, contract review Doing/incomplete, Cognito and
  pinning To Do/incomplete. The connector's 2,048-character description ceiling
  required concise task summaries linked to this full record; no unrelated
  cards, member assignments or historical spike evidence were changed.

No product/runtime tests, deployed IAM validation, real Cognito/email delivery,
backup restore, DynamoDB/S3 races, AWS cost measurement or infrastructure preview
was performed. Acceptance is not evidence that those future checks pass.

After remaining interface agreement, the smallest separately authorized task is
the local users/projects DynamoDB ownership slice, not executor/deployment work.

## Browser/local authentication continuation

Davian authorized development Cognito through SST and browser users/projects
integration on 2026-10-07. The API and DynamoDB remain local. After checking
AWS documentation, he accepted Essentials instead of Lite, preserving one-day
rotating refresh and memory-only browser sessions, and authorized Cognito
default email for development. Public custom SES sender readiness is deferred.
See the [development record](../development/cognito-local-projects.md) for
resource/readback, validation, email participation and incremental costs.
The full contract-review card remains Doing; this changes auth configuration,
not the accepted seven users/projects routes or later-slice interfaces.

SST preview/deployment/readback and final no-change diff passed. Davian completed
genuine signup/verified sign-in, project create/list/detail/rename/delete and
password-reset email/code/new-password sign-in in Vivaldi. Independent local
inspection verified user provisioning, rename version 1, deletion acceptance
and name-free completed tombstone. All repository checks and race-enabled
DynamoDB Local/Chromium fixture integration passed. The project card is Done;
the Cognito card retains genuinely expired-code and one-day session-expiry checks.
Davian also confirmed a genuine forced Cognito SDK refresh succeeded in Vivaldi.
Subsequent live checks confirmed resend delivery with a disabled 60-second
cooldown, clear wrong verification/reset code errors with successful correction,
and automatic refresh at 901 seconds followed by successful project creation
and detail loading. Sign-out and reload while signed in each returned to sign-in
and removed project list/details. Tokens were not printed and lifetimes were
not shortened. Invalid-code and fifteen-minute refresh results do not establish
genuinely expired-code or one-day session-expiry behavior; those remain pending.
