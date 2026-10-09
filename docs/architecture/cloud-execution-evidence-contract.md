# Single saved-request cloud execution: review packet

Status: **Accepted by Davian Hernandez**, 2026-10-08, including the source-bound
idempotency and evidence-delivery clarifications at commit `05840e8`. Davian
explicitly authorized implementation, scoped development AWS deployment and
validation. Accepted design is not evidence of implemented or tested behavior.
Unperformed runtime, fault, AWS and genuine browser checks remain pending.
Branch: `feat/cloud-execution-evidence`, from merged main `b7aaac8`.

Authority: [accepted architecture](README.md), [dispatch/publication design](w1-system-design.md),
[data model](w1-data-model.md), [defaults](w1-review-decisions.md), and
[protected saved requests](protected-request-secrets-contract.md).

## Accepted routes and submission identity

All routes use existing Cognito ownership, error envelopes, project gates,
active-stage/recovery guards and no-store responses. Foreign resources are 404.
No secrets are accepted in execution route payloads.
Success envelopes preserve the live contract: submission/rerun/detail return
`{data:{execution:...}}`, status `{data:{status:...}}`, history
`{data:{items:[],nextCursor:null}}`, evidence `{data:{evidence:...}}`.
Admission includes Location and Retry-After. The complete evidence API envelope,
not just its inner object, must fit the accepted 4 MiB cap; the fully serialized
Lambda proxy response must independently fit 6 MiB. See delivery encoding below.

| Route under `/api/v1/projects/{projectId}` | Contract                                                                                                                                                                                                        |
| ------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /requests/{requestId}/executions`    | Required quoted request-revision `If-Match` and `Idempotency-Key`; strict body below; 202 after durable acceptance. Missing revision 428, stale 412, idempotency mismatch 409.                                  |
| `GET /executions`                          | Newest-first history; optional frozen `requestId` and `status`; limit defaults to 25, maximum 100; existing signed, owner/project/filter/generation-bound, 24-hour cursors. A surviving request is unnecessary. |
| `GET /executions/{executionId}`            | Frozen safe request identity/configuration, source revision, current status, version, retention, nullable safe summary and separate live-request link.                                                          |
| `GET /executions/{executionId}/status`     | Immediate-return status DTO below, with server time. No response body or protected inputs.                                                                                                                      |
| `GET /executions/{executionId}/evidence`   | 200 with the immutable sanitized evidence object, including upstream failures; read only through published SNAP after current authorization, retention, size/checksum verification.                             |
| `POST /executions/{executionId}/rerun`     | Deliberate new execution from frozen public configuration/policy and current exact stable secret references. Required fresh `Idempotency-Key`; strict body `{allowInsecureSecrets:false}` by default.           |

Batch status remains a compatible future route; this slice polls one selected
execution. There is no evidence PATCH, replay-job route or pinning endpoint.

Submission body (defaults shown):

```json
{
  "timeoutSeconds": 30,
  "allowInsecureSecrets": false,
  "responseRedaction": {
    "headers": [],
    "jsonPointers": [],
    "omitBody": false
  }
}
```

`timeoutSeconds` is an integer 1–60. Response header names are case-insensitive;
at most 16 additional names. At most 16 RFC 6901 pointers, no overlapping paths,
maximum 1 KiB per pointer; the empty pointer masks the whole body. Policy is
immutable for this execution, counts toward the complete 64 KiB frozen plan,
and defaults identically on submission retry. Unknown/duplicate JSON fields,
invalid union types and unsupported policy forms fail closed with fixed errors.
The body contains policies only; it cannot contain a literal secret.

Idempotency keys are random UUIDs. Persist only private HMAC-derived key/input
identities. Use one execution-admission receipt namespace per owner/project for
both submissions and reruns: `P / IDEM#execution-admission#keyDigest`. The private
key digest binds app/stage, owner/project and the caller's key, **not the source
ID**. Thus using the same key for another source finds the existing receipt and
conflicts instead of creating a second receipt/job. The separately domain-bound
normalized-input HMAC includes:

- For submission: operation `submit`, owner/project, **source requestId from the
  route**, expected request revision from If-Match, normalized timeout, consent
  and response-redaction policy.
- For rerun: operation `rerun`, owner/project, **source executionId from the
  route**, and normalized rerun consent. The source execution identifies its
  immutable replay plan/policy; current secret values/revisions are resolved for
  first admission only and do not change a retry's input identity.

Consequently, equal revisions/policies on two different requests never make
their submissions identical; two executions of the same request never make
their reruns identical. Within this owner/project namespace, cross-source key
reuse returns **409**, as does reusing a submission key for a rerun (or vice
versa), before any new admission, KMS preparation or queue notification. Never
return another source's execution as an idempotent success. Source IDs belong in
the input identity even though they must not partition the unique-key lookup.
Reuse the stage's existing private signing-key material with separate HMAC
domains and unambiguous structured field encoding; no public credential
fingerprints. Look up an existing receipt after ownership/stage checks and before
current source revision validation:
same input returns the original execution even after a later request edit or
secret replacement. A mismatched input returns 409. Receipts have the accepted
seven-day retry window; known expired receipts return 409 with a fixed instruction
to submit deliberately with a new key. The client never retries outside that
window or silently changes a key. Receipt cleanup preserves active obligations.
On uncertain admission, read back by receipt; unresolved outcome returns 503
and the UI retains the original key/input in memory for explicit reconciliation.

Rerun is distinct from a retry. It creates new execution/job/receipt/OUT and uses
current active values at the frozen stable secret IDs, never the old job bundle
or a similarly named replacement. Missing/deleted request or missing/revoked
reference fails explicitly before admission. Changes to the live public request
do not modify the frozen replay configuration. A rerun retry uses its own receipt
and must return that same new execution. UI labels warn that a rerun can repeat
external effects, particularly after an unknown outcome.

## Accepted protected-input freezing and runtime boundary

At submission, strongly read source request and each unique active saved-secret
revision, decrypt only in authorized API memory, then freshly encrypt the job
bundle with context/AAD binding app/stage/project/job/purpose/version. Purpose
is `job-bindings`; saved-secret envelopes keep their existing context. Commit
source revision/state checks, request revision, project/user/stage/quota guards,
frozen EXEC, JOB, Protected BINDINGS, private receipt and OUT in one bounded
transaction. Shared references are deduplicated without losing binding identity.

The accepted complete job-binding envelope cap remains **32 KiB**, independent
of the 64 KiB frozen plan and resolved body limits. A valid saved request with
larger aggregate protected values can therefore receive 413 at submission.
No silent cap increase, splitting abstraction or historical-secret reuse.
Explain this distinction in the UI. Submission adds scoped API KMS Decrypt for
saved-secret context and GenerateDataKey for job-bindings context; worker only
decrypts its job-bindings context. No plaintext persistence or key cache.

Local/shared replacement affects later submissions/reruns, not accepted jobs.
Revocation/deletion of an exact frozen source secret or source request blocks
dispatch. Validate live states and condition-check observed source versions in
the intent transaction, then discard raw buffers when done. A concurrent ordinary
replacement may require another pre-intent validation, but does not substitute
new values into the frozen bundle. Revocation after committed dispatch intent
cannot recall an authorized outbound operation; it blocks future dispatches.
Delete protected execution inputs after terminal publication and on project
deletion. Preserve them during unresolved preterminal/publication obligations.

Explicit `allowInsecureSecrets:true` is required when any enabled protected
input will be sent over HTTP. The browser presents a deliberate consent control;
default false rejects before admission. Retries preserve the original consent;
reruns require their own explicit consent. HTTPS is the normal browser path.

## Accepted evidence and status schemas

Schema-1 evidence is typed JSON, with nullable scalars explicit and collections
always arrays. It includes no current live links, retention flags or mutable
availability. Those belong to Control/read DTOs. Request capture is sanitized,
never a recovered raw frozen bundle.

```json
{
  "schemaVersion": 1,
  "projectId": "opaque-id",
  "executionId": "opaque-id",
  "source": { "requestId": "opaque-id", "revision": 3 },
  "target": "cloud",
  "submittedAt": "UTC timestamp",
  "startedAt": "UTC timestamp or null",
  "completedAt": "UTC timestamp",
  "status": "completed or failed",
  "outcome": { "code": null, "certainty": "known" },
  "request": {
    "method": "GET",
    "url": "sanitized absolute URL",
    "headers": [],
    "body": { "kind": "none", "text": null, "omissionReason": null }
  },
  "response": {
    "httpStatus": 200,
    "headers": [{ "name": "content-type", "value": "application/json" }],
    "body": { "kind": "json", "text": "{\"ok\":true}", "omissionReason": null },
    "wireBytesRead": 11,
    "decodedBytesRead": 11
  },
  "timing": { "durationMs": 24, "timeToFirstByteMs": 20 },
  "redaction": { "policyVersion": 1, "applied": true }
}
```

Actual DTOs use enums/numbers/nulls, not the descriptive placeholder strings
above. `startedAt` is the committed intent timestamp, not proof of HTTP delivery.
Duration uses a monotonic clock and covers destination resolution through body
read; timings unavailable to reconciliation are null. Byte counts are bounded
counts observed while reading, not guessed complete sizes. Header arrays preserve
duplicate values; Go cannot promise original wire ordering/casing across names.
Body kinds are `none`, `json`, `text`, `omitted`; text is UTF-8 or null.
No raw HTTP reason phrase, TLS/DNS/socket error, IP trace, URL-bearing diagnostic,
stack trace, secret digest or redaction matched-value list.

Execution states: `queued`, `claimed`, `running`, `completed`, `failed`.
Unknown outcome is `failed` with code `execution_outcome_unknown` and certainty
`unknown`, prominently rendered as **Outcome unknown — do not assume no effect**.
Pre-intent failures have certainty `not_dispatched`. Once intent is durable,
transport failures/timeouts without a complete response are conservatively
unknown. Observed HTTP status is still retained when available. Completed
responses, including 4xx/5xx and safe body omissions, have certainty `known`.
Unsupported content omission alone does not turn a complete 2xx/3xx into failed.
Response overruns do fail the capture, with observed status if available and
fixed omission reason; they do not imply the upstream operation had no effect.

Fixed outcome codes: `upstream_http_error`, `execution_timeout`,
`upstream_network_error`, `execution_outcome_unknown`, `response_limit_exceeded`,
`destination_blocked`, `source_unavailable`, `input_limit_exceeded`,
`queued_expired`, `sanitization_failed`, `recovery_interrupted`.
Assertions, schemas and extraction are outside this slice.

Status DTO: `{executionId,status,version,submittedAt,startedAt,completedAt,
summary,retention,serverTime}`. Summary is null before publication, otherwise
`{httpStatus,durationMs,outcome}`. Retention is null before publication,
otherwise `{normalExpiresAt,pinned:false,evidenceAvailability}`. Terminal status,
insert-only SNAP, RET, ticket publication, counters and protected-input cleanup
commit atomically. Uploaded but unpublished evidence is never returned.
Missing/corrupt published objects yield fixed 503 `evidence_unavailable` and
update mutable availability only; expiry yields 404. No fabricated evidence.

## Accepted evidence delivery encoding and publication size checks

Store sanitized evidence as deterministic UTF-8 JSON bytes in S3. Browser
delivery is `application/json; charset=utf-8`, with the complete UTF-8 JSON
success envelope `{data:{evidence:...}}`; the evidence is a JSON object, not a
JSON-encoded string or a browser-visible base64 field. Use the same defined
JSON serializer/escaping rules for publication preflight and API delivery,
including HTML escaping; count bytes after serialization, not characters or
decoded upstream-body length. No content compression or streaming in this slice.

For the evidence route, the Lambda HTTP API payload-v2 response **always** uses
standard base64 in its `body` field and `isBase64Encoded:true`, even for small
captures. API Gateway decodes the transport body; React receives the ordinary
JSON success envelope. This avoids a second JSON-string escaping pass over
the full evidence envelope. The existing general adapter's 64 KiB threshold
is not the evidence-route contract. This is a contract requirement for later
implementation, not a claim that that adapter has already been changed.
[HTTP API Lambda payload-v2 encoding](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html).

Before registering an upload ticket or attempting PUT, freeze the sanitized
capture, all omission decisions and outcome, then check all three serialized
representations:

1. Complete S3 evidence bytes are <=4,194,304 bytes (4 MiB).
2. Complete API-envelope bytes, including `data`/`evidence` wrappers, JSON
   escaping and serialization overhead, are <=4,194,304 bytes independently.
3. The **fully serialized Lambda response JSON** is <=6,291,456 bytes (6 MiB).
   Include `statusCode`, all headers/cookies/other emitted fields, the
   base64-encoded envelope and `isBase64Encoded`, plus their JSON escaping and
   serializer overhead. Base64 length is `4 * ceil(envelopeBytes / 3)`; that
   calculation alone is not the complete response-size check. AWS documents
   the synchronous request/response quota as 6 MB and explicitly uses binary
   units for MB here. [Lambda quotas](https://docs.aws.amazon.com/lambda/latest/dg/gettingstarted-limits.html).

Prepublication Lambda measurement must use the deployed serializer and a
bounded worst-case set of allowed API response headers/metadata, including
configured CORS headers. The complete serialized proxy response with `body:""`
must be <=16 KiB, counting all framing, headers, cookies and other fields; use
that maximum metadata allowance for prepublication measurement, then measure
the actual response at delivery. A maximum-size 4 MiB API envelope needs
5,592,408 base64 bytes; adding the 16 KiB allowance remains below 6 MiB.
This allowance supplements, not replaces, the fully serialized response check.
Do not reflect arbitrary upstream/client headers into that envelope. API reads
use the same bounds and measure the actual fully
serialized proxy response again before returning it. Any serializer, encoding
or header-bound change must preserve this prepublication proof and its tests.
Private S3 keys/checksums/byte reservations describe the final evidence bytes,
not a larger candidate later changed for delivery.

If any check fails, omit the unsafe/oversized body before publication with
`encoded_evidence_limit`, retain the bounded sanitized status/timing/header
summary, and rerun **all three checks** on the final capture. If necessary omit
both request and response body text. If even the minimal safe capture cannot
fit, do not publish it: preserve the pending obligation for safe reconciliation
without HTTP replay. Never silently truncate, omit/rewrite evidence at GET time,
or publish a capture that only fits S3 but cannot be delivered. A later violated
delivery bound returns a fixed small 503 `evidence_unavailable`, not a modified
capture. Atomic immutable publication occurs only after these final checks;
evidence contents and checksum remain unchanged across reads.

Required implementation tests (pending review/implementation): exact-limit and
one-byte-over cases at each representation; 2 MiB bodies dominated by quotes,
backslashes, control characters and HTML-escaped characters; multi-byte Unicode
and already escaped JSON strings; worst-case permitted headers/metadata;
base64 lengths for all three input-length remainders; and API Gateway decode
round-trip equality with the published evidence. Include a candidate fitting
the evidence-object limit but exceeding the API-envelope limit, and a raw
proxy-body double-escaping case that exceeds Lambda's limit while base64 fits.
Assert every omission happens before ticket/PUT/SNAP publication, manifest
checksum/size match the final object, repeated GETs preserve the same capture,
and the actual serialized proxy response satisfies the Lambda bound. These
requirements are not claimed tested by the documentation-only task.

## Accepted safe redaction and content policy

- Always mask credential/cookie headers and credential-named URL query/JSON
  fields, plus configured response headers/paths. Credential-named JSON
  containers are masked as whole subtrees. Match names case-insensitively using
  the existing saved-request recognizer, extended to Set-Cookie responses.
- Collect the enabled protected input values in runtime memory. Include Bearer
  token and cookie value components; include scalar leaves of whole protected
  JSON. Remove exact reflections everywhere in request/response URLs, headers,
  body and text metadata, including supported JSON escape, URL percent/form
  encoding and standard/base64url forms. Decode supported content before
  matching; never persist raw buffers as intermediate evidence.
- Sensitive JSON scalars are masked as an unambiguous JSON string `[REDACTED]`,
  even when originally numeric/null. Whole sensitive request bodies are omitted
  with `sensitive_request_body`; never reconstruct them for evidence display.
- Support strict UTF-8 `application/json`, `application/*+json`, and `text/plain`;
  charset absent or UTF-8 only. JSON rejects duplicate keys, trailing data and
  depth >32. Nonempty content without a supported media type is omitted.
  Support identity or one gzip encoding with independent wire/decoded caps;
  reject stacked/other encodings. Disable implicit Go decompression.
- Binary, HTML, XML, multipart, unsupported charset/encoding, invalid UTF-8/JSON,
  or unapplicable configured paths omit the entire response body with a fixed
  reason. Missing configured JSON paths are treated as unsafe policy application,
  rather than silently ignoring a spelling error. Sanitizer failure also omits
  unsafe headers/body and marks capture failed. Empty bodies can use `none`.
- Extremely short protected values can cause extensive masking/omission;
  secrecy takes precedence over readability. No arbitrary secret transformation
  detector is promised. For unsupported/custom encodings or transformed echoes,
  users must configure paths or `omitBody:true`. UI states this limitation.
- Bound sanitized headers under the accepted 64 KiB header cap. Enforce the
  evidence-object, complete API-envelope and fully serialized Lambda-response
  limits above. Any size omission, including `encoded_evidence_limit`, is frozen
  before immutable publication, retaining the safe status/timing/header summary.
- Render only escaped React text/`pre`. No HTML injection, iframe preview,
  executable download, auto-followed response links or browser persistence.
  Stop polling on terminal state/sign-out/unmount/deletion, allow only one request
  in flight, use ten-second light polling with jitter/backoff and visible refresh.

## Accepted lifecycle preserved by implementation

Durable OUT plus one bounded two-second fast SQS notification; no unawaited
sends. Fenced dispatch intent before HTTP; uncertainty readback; no running-to-
queued transition. Fresh nonreused target connection, no automatic HTTP retry,
redirect or ambient proxy. Identifier-only SQS messages; one-record batches,
two worker concurrency, 90-second invocation, 120-second lease, 540-second
visibility, five-receive redrive, four-day queue/fourteen-day DLQ. Claims before
intent can recover with new fence up to five; queued jobs expire after ten minutes.

Accepted development configuration clarification: standard-mode SQS event-source
maximum concurrency is two with batch size one. The worker has no reserved or
provisioned concurrency and no provisioned pollers. SQS is its only configured
execution trigger. This caps queue-driven concurrency without reserving account
capacity; it does not guarantee available capacity or limit authorized direct
invocations. Account throttling can delay queue delivery. Visibility/redrive,
durable outbox, bounded queued expiry, fenced claims and intent recovery remain
in force: retries before intent may dispatch once; recovery after intent never
replays outbound HTTP.

Apply all accepted destination protections at every dial: public A/AAAA only,
reject any unsafe answer and mapped/tunnel/translation bypasses; pin validated
IP with hostname TLS verification. HTTP/S ports 80/443 only, no Host override;
separate AWS SDK transport. DNS two seconds, connect/TLS five, response headers
ten within total 30-second default/60-second maximum; 64 KiB headers/resolved
body, independent 2 MiB wire/decompressed limits, complete 4 MiB evidence cap.

Register exact immutable S3 keys and byte reservations before single conditional
PUT. Finalization can retry prepared evidence only, never outbound HTTP.
Reconciliation after expired running lease fences original writer and publishes
a safe unknown capture through the same ticket/terminal protocol. Missing result
does not invent HTTP status or body. Failed publication preserves pending state
and inaccessible orphan for bounded reconciliation.

Retention is completedAt+30 days, separate mutable RET, no authoritative native
TTL and no blanket S3 expiry. Preserve future pinned records. Existing light
admission/storage quotas apply (1,000/month, 100/user/day, 1 GiB/project,
2 GiB/stage). Cleanup must release counters once under the same gate/RET/ticket
conditions, never erase unknown records or incomplete obligations.

Project deletion immediately denies access; resumably drains recognized EXEC,
JOB, OUT, receipts, RET/SNAP/events, Protected bundles and evidence. It preserves
unknown schemas/fields, upload tickets and WORK until settled. Fence before
deleting exact keys; repeat prefix sweeps after writer quiescence. Uncertain PUT
outcomes keep deletion pending, even after one absent HEAD/empty LIST. Maintain
minimal project tombstones and low-frequency residual-prefix checks after
completion. Physical completion requires both strong table partition proofs,
settled tickets and empty S3 prefix; backups keep the accepted restore exception.

Add `cloudExecutionsSchemaVersion=1` capability only after all consumers and
cleanup are deployed; update saved-request capability from 2 to 3 as the existing
old-binary rejection fence. Protected saved-secret capability remains 1; new
execution-input capability is independently checked. Preserve recovery generation.
Existing protected slice binaries reject saved-request capability 3 and cannot
incorrectly complete Control/Protected-only deletion.

## Deployment preview and verification preparation

Reuse dev-api Control/Protected indexes, stage KMS key, HTTP API, API Lambda,
maintenance schedule and private cursor-key parameter. Add one worker Lambda,
standard queue/DLQ and one private SSE-S3 evidence bucket with TLS-only policy,
Block Public Access, no versioning/public/presigned access. Extend scoped API,
maintenance and worker permissions by purpose; maintenance has no secret decrypt.
Inspect generated SST diff, bootstrap effects and current non-root account
before mutation. Frontend hosting, Cognito changes and unrelated stages excluded.

Incremental costs will be refreshed from official AWS prices and the generated
diff before deployment. Existing API/scheduler/key fixed costs are not new costs.
Forecast must state polling, compute, DDB transaction/storage/PITR, SQS/S3,
KMS, logs/bootstrap assumptions; no free-tier subtraction or hard cap claim.

Meaningful tests will cover duplicate delivery; pre/post-intent crashes and lost
commit/queue/S3 acknowledgments; source replacement/revocation races; reflected
redaction/unsupported content; DNS/IP/rebinding/redirect protections; independent
wire/gzip/envelope/Lambda limits and worst-case escaping; source-bound submission
and rerun retries, cross-source/cross-operation key reuse returning 409 without
new admission or disclosure; terminal publication races; retention and late-PUT
deletion/unknown records. Fault injection is simulated evidence, explicitly
separate from actual AWS runtime/IAM/queue/S3 verification and genuine browser
checks. Controlled public endpoints and disposable memory-only secrets are
required for browser validation; do not send credentials to third-party echo
services. Secret containment checks return booleans/counts only, never values.

## Preparation status

- Read merged repository contracts/ADRs and relevant Trello board/prototype card;
  no configured WIP limit or prototype checklist was returned.
- Feature branch created from merged main; no unrelated working-tree edits.
- Following initial session expiry and Davian's reauthentication, STS verified
  account `747336059622`, non-root `davian-admin`. Read-only us-east-2 inventory
  found the existing dev-api Control/Protected tables and API/cleanup Lambdas;
  no dev-api-prefix queues or Kurier-prefix S3 buckets were returned. Existing
  API is ARM64/256 MiB/15 seconds; cleanup ARM64/256 MiB/60 seconds. Recheck
  scoped policies, stage state and shared SST bootstrap during generated preview.
  No AWS mutation has been performed.
- Preparation checks passed: repository formatting, frontend lint, 36 frontend
  tests, eight infrastructure tests, frontend TypeScript/Vite build, Go tests/vet
  across all five modules and API/worker builds. Go fixture tests required the
  approved unsandboxed run because listening sockets are denied in the sandbox.
  These are existing-baseline checks, not execution/fault/AWS/browser tests.
- Trello prototype card has a read-back preparation/verification checklist;
  preparation is complete and review/implementation/deployment/browser work
  remains open. Live proposal/contracts were read; acceptance is not recorded.
- Davian requested committing and pushing this Proposed packet and preparation
  record on `feat/cloud-execution-evidence` for external review. Publication
  does not record acceptance or authorize dependent implementation/deployment.
- Davian subsequently found the design acceptable subject to the two source-
  identity and delivery-size clarifications above, and requested committing and
  pushing those clarifications for review before dependent work. This revision
  changes documentation only; the new runtime/fault-test requirements remain
  pending and no AWS mutation is authorized by publishing the clarification.
- Review choices above are not accepted merely by writing this packet. Product
  implementation, genuine browser checks and final submission of the verified
  implementation remain outstanding.

## Accepted implementation continuation

Davian accepted this clarified packet and authorized the previously scoped
implementation/deployment/validation on 2026-10-08. Earlier preparation entries
are historical; their review holds are superseded by this explicit acceptance.
Implementation, generated infrastructure/cost preview, deployment, actual AWS
verification and genuine browser validation are pending until individually
performed and documented. No acceptance alone marks those checks passed.
