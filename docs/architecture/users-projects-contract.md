# Accepted users/projects contract

Davian Hernandez accepted the focused review packet and authorized local
implementation and the focused development AWS continuation. This resolves only users/projects interface choices; the overall
contract-review card remains **Doing**. Workflow extraction, OpenAPI libraries,
local-result schemas, other resource revisions, MCP and pairing remain proposed
in [the synchronization record](w1-acceptance-sync.md), not prerequisites.

## Interface

Require a verified Cognito access token on all seven routes. `/healthz` remains
public and unchanged. Use `/api/v1`, application/json, lower camelCase,
opaque random UUIDs and server-generated UTC RFC3339 timestamps. Return
`Cache-Control: no-store`. No caller-controlled owner/user ID.

| Route                                                              | Request                          | Success                                                  |
| ------------------------------------------------------------------ | -------------------------------- | -------------------------------------------------------- |
| GET /api/v1/users/me                                               | None                             | 200 `{data:{user:User}}`                                 |
| POST /api/v1/projects                                              | `{name:"My API"}`                | 201 `{data:{project:Project}}`, Location, ETag           |
| GET /api/v1/projects?limit=25&cursor=…                             | Optional pagination              | 200 `{data:{items:Project[],nextCursor:null \| string}}` |
| GET /api/v1/projects/{projectId}                                   | None                             | 200 project envelope, ETag                               |
| PATCH /api/v1/projects/{projectId}                                 | `{name:"Renamed API"}`, If-Match | 200 project envelope, new ETag                           |
| DELETE /api/v1/projects/{projectId}                                | No body, If-Match                | 202 deletionOperation envelope, Location                 |
| GET /api/v1/projects/{projectId}/deletion-operations/{operationId} | None                             | 200 deletionOperation envelope                           |

User example:

```json
{
  "data": {
    "user": {
      "userId": "c4fddf97-9bc7-42b1-9d65-f960b5a34977",
      "displayName": "",
      "createdAt": "2026-10-06T18:00:00.000000000Z"
    }
  }
}
```

Project POST body is `{"name":"My API"}`. Creation returns `ETag: "0"`,
Location pointing to the detail route, and:

```json
{
  "data": {
    "project": {
      "projectId": "1d0d5a30-6308-4560-9dcc-99e2d936ae66",
      "name": "My API",
      "version": 0,
      "createdAt": "2026-10-06T18:00:00.000000000Z",
      "updatedAt": "2026-10-06T18:00:00.000000000Z"
    }
  }
}
```

PATCH `{"name":"Renamed API"}` with `If-Match: "0"` returns version 1,
`ETag: "1"` and a new updatedAt. Clients treat IDs as opaque, not parsed UUIDs.

Names trim surrounding whitespace, contain 1–100 Unicode code points, reject
control characters and may be duplicated. POST/PATCH bodies are <=8 KiB,
including encoding. Reject missing/null/wrong-type names, malformed/non-UTF8 JSON,
unknown fields, duplicate JSON keys and trailing content. Client writes cannot
set IDs, ownership, version, timestamps or state. DELETE has no body.
No automatic project-creation retry and no creation idempotency receipt: an
uncertain response may correspond to a committed project. The SDK also disables
automatic retries. Reconcile by listing/detail, not blindly submitting again.

## Identity and persistence

Verified issuer/sub maps through
`IDENTITY#SHA256(length-prefixed issuer/sub)/META` to `U#userId/META`.
Provision both with conditional puts in one transaction guarded by the active
stage/generation. Concurrent first accesses converge on one user. Verify the
stored exact issuer/sub; email is not identity. Empty displayName avoids a profile
lookup or assumption that access tokens contain profile fields. Profile editing
is outside this slice.

Project gate is `P#projectId/META`, with ownerId, active/deleting/deleted,
version and deletionEpoch. GSI1 is KEYS_ONLY with `LPK=U#userId#PROJECT` and
`LSK=fixed-width UTC createdAt#projectId`. Creation conditionally inserts the
gate; mutations condition owner/state/version and increment the shared gate.
Stage/generation and current enabled user/version guards join write transactions.
There are at most four small actions in this slice; no full model migration.

Strong base reads enforce ownership/state. Cross-user and inaccessible project
IDs return the same 404. The shared gate version will also change on later child
mutations; a rename can require refresh even when the displayed name is unchanged.

## Preconditions and errors

Project GET/POST/PATCH expose quoted decimal ETags from `version`.
PATCH/DELETE require exactly one quoted decimal If-Match: no weak tag, wildcard,
multiple tags or leading zeroes. Missing ->428; stale ->412; authorize first.
This **replaces the earlier proposed 409 for project preconditions only**.
Never refresh/retry a stale PATCH against a newer version automatically.

| HTTP | Code                  | Cause                                         |
| ---- | --------------------- | --------------------------------------------- |
| 400  | invalid_request       | Malformed JSON, If-Match, query or cursor     |
| 400  | validation_failed     | Field validation/unknown fields               |
| 401  | unauthenticated       | Missing/invalid/expired access token          |
| 403  | forbidden             | Disabled application user                     |
| 404  | not_found             | Missing/foreign/inaccessible resource         |
| 409  | conflict              | Other conditional/concurrent state conflict   |
| 412  | precondition_failed   | Stale project version                         |
| 413  | payload_too_large     | Request body exceeds 8 KiB                    |
| 415  | invalid_request       | POST/PATCH media type is not application/json |
| 428  | precondition_required | Missing If-Match                              |
| 429  | rate_limited          | Throttling, Retry-After required              |
| 503  | service_unavailable   | Recovering stage/persistence/JWKS unavailable |
| 500  | internal_error        | Unexpected failure                            |

```json
{
  "error": {
    "code": "precondition_failed",
    "message": "Project changed; refresh before retrying.",
    "details": [],
    "requestId": "opaque-correlation-id"
  }
}
```

No secrets, raw database errors or stack traces in messages. Local 503 responses
suggest Retry-After 2; rate limiting is a future hosted enforcement concern,
not claimed as implemented. Unregistered routes/methods return a safe 404.

## Pagination

Default limit 25; integer range 1–100. Only limit/cursor are allowed; duplicates,
empty cursor, malformed query or invalid/expired/tampered/wrong-owner cursor
return 400 invalid_request. No offsets/counts. Signed cursor binds owner, stage/
generation, projects type/index, first-page high-water, LastEvaluatedKey and a
24-hour expiry. The signing key is required runtime configuration, not embedded.

Query GSI1 descending, then strongly hydrate each project gate. At most five
candidate query pages per response, respecting DynamoDB's page boundary;
`{"data":{"items":[],"nextCursor":"opaque"}}` is valid. Finish only when
nextCursor is null. Lists are eventual, not snapshots; refresh the first page
for late-visible entries. Creation/detail responses provide immediate
confirmation. Cursor signing does not make index results authoritative.

## Deletion

One transaction changes active ->deleting, increments version/epoch, removes
listing keys and writes `P/WORK#delete#operationId`. Deny ordinary access at
commit; already delivered bytes cannot be recalled. The owner-only operation
route is a deliberate tombstone exception, never a project-data read.

```json
{
  "data": {
    "deletionOperation": {
      "operationId": "opaque-operation-id",
      "projectId": "opaque-project-id",
      "state": "accepted",
      "startedAt": "2026-10-06T18:05:00.000000000Z",
      "completedAt": null,
      "retryAfterSeconds": 2
    }
  }
}
```

The DTO supports accepted/deleting/waiting_uploads/completed/failed_retryable.
The local empty-project path uses accepted ->deleting ->completed; unknown
children/conflicts leave it deleting/pending. Completion sets completedAt and
retryAfterSeconds 0. Poll at two seconds, back off up to ten, stop on completion,
authentication failure or 404. No SSE/long polling.

Repeat DELETE with the initiating version returns the same operation, even
after completion. Missing/malformed preconditions still fail; a different stale
version yields 412. Lost acceptance acknowledgment is reconciled by strong read,
not a second operation. Completion acknowledgment loss is similarly read back.

The permanent minimal tombstone retains opaque project/owner ID, schema/type,
coordination version/epoch, initiating version, operation ID and safe state/
timestamps. No names, ordinary creation/update timestamps, configurations,
listing attributes or evidence remain. Never reuse project UUIDs. No TTL governs
authorization or deletion. User identity is account-level and remains.

**Empty-project scope:** the local CLI requires a loopback endpoint and active
`localEmptyProjectsOnly` stage. There is no Protected table or object writer in
this isolated subset. It first persists deleting status; on restart it rechecks
durable work and strongly queries the project partition. Only META and the exact
deletion WORK may exist. Any unknown child, additional page or missing work
prevents completion and is not erased. Completion conditionally replaces the
gate with a minimal tombstone and deletes WORK in one transaction. A writer
participating in gate CAS invalidates the completion version. Arbitrary direct
administrator writes outside that protocol cannot be fenced; do not enable child
writers under this marker. Removing the marker fails this local executable closed.

The eventual full implementation must drain both tables/S3, pins/secrets and
late/uncertain uploads before reporting physical completion. That is **not
implemented or validated here**. Backup residuals and Davian's restore exception
remain: earlier restores may lose later changes and resurrect later deletions.
Communicate actual restore times; reconcile stores before reopening; never
automatically replay restored external HTTP work.

See [local setup, tests and limitations](../development/local-ownership.md).

## Development cloud adapter

The authorized SST `dev-api` stage reuses this seven-route contract and the
existing development Cognito pool. ARM64 Go Lambda/HTTP API uses the same verifier,
ownership store, conditional transactions, signed pagination and uncertain-write
behavior. React remains local at the explicitly configured loopback origin. Only
local HTTP origins with a port or HTTPS API Gateway origins in us-east-2 are
accepted by frontend configuration; no credentials/path/query/fragment. CORS
exposes ETag/Location/Retry-After and permits only the configured frontend origin.

Control has all three accepted KEYS_ONLY indexes. Stable cursor signing uses an
SSM SecureString read by the API at cold start, with no key in frontend/SST outputs.
A minute scheduler discovers deletion WORK through eight GSI2 due shards and
durable rotating page cursors, then strongly rechecks the existing empty-project
cleanup protocol. Unknown children keep deletion pending and are never erased.
The historical `localEmptyProjectsOnly` marker is retained for both isolated
stages; child/object writers remain prohibited. Full cross-store deletion is
still later work. Gateway-native errors outside Go can differ from shared
application envelopes; application status/error contracts remain unchanged.

See [cloud setup and separately recorded validation](../development/aws-users-projects.md).
