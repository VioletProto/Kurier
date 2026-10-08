# Protected saved-request secrets: accepted contract

Status: **Accepted by Davian, 2026-10-07**, including project-owned lifecycle,
shared replacement/revocation, encryption, limits and cross-table cleanup.
The following binding/URL clarifications were requested before implementation.
Branch
`feat/protected-request-secrets` starts at merged main `d8a534e`.

This extends the [accepted saved-request contract](saved-requests-contract.md)
and accepted W1 crypto/data boundaries. The proposal and live API contracts
were read alongside ADR 0002 and the contract-review/saved-request Trello cards.
Those sources accept encrypted reuse and set/preserve/remove semantics, but
still label concrete secret DTOs/locators as proposed. This contract records
only the additional choices accepted for this slice.

## Routes and project-owned lifecycle

Retain the existing five request routes, request revision ETags and project
gate CAS. Introduce these metadata/management routes:

| Method | Project-relative path | Input                                      | Result                                               |
| ------ | --------------------- | ------------------------------------------ | ---------------------------------------------------- |
| GET    | `/secrets`            | signed limit/cursor                        | `{data:{items,nextCursor}}`, safe metadata only      |
| PATCH  | `/secrets/{secretId}` | `{value:string}`, secret-revision If-Match | `{data:{secret}}`, ETag; explicit shared replacement |
| DELETE | `/secrets/{secretId}` | secret-revision If-Match                   | 204; revoke and remove active ciphertext             |

There is no plaintext GET or reveal route. New values are created atomically
with request writes using `set`; no separate secret POST is necessary.
Metadata contains only secretId/projectId/revision/createdAt/updatedAt,
state, `masked:true` and a fixed mask. No length, suffix, hash, fingerprint,
user label or submitted locator/name is derived from plaintext.

Secrets are **project-owned**, independently reusable after creation. Deleting
a request removes its bindings, not project-owned secrets used by other
requests. Unreferenced secrets remain manageable until explicit secret deletion
or project cleanup. This avoids speculative refcounts/orphan collectors and
unbounded scans in request writes. Original request/locator may be retained as
opaque provenance, but source-request survival is not an eligibility condition.
This lifecycle is accepted for this slice.

## Sensitive field writes and safe reads

Protected slots carry a client-generated opaque UUID `bindingId`, unique across
the request. A row's preserve identity is `(bindingId, collection, exact name)`;
order and enabled state are not identity. Duplicate names have distinct IDs;
reordering carries IDs with rows. A body slot's identity is
`(bindingId, body type, whole-body or canonical RFC 6901 pointer)`.
Pointers decode `~0`/`~1` strictly and re-encode canonically; array indexes use
canonical nonnegative decimal without leading zeros. Changing a row name,
moving between headers/query, changing body type or changing a pointer creates
a new binding and requires explicit `secretRef` or `set`, not `preserve`.
Preserve requires the exact identity and secretId from the previous revision;
it cannot copy another binding's ID/reference, resurrect a removed binding,
duplicate a binding, or introduce a binding on POST. IDs must remain unique
even for repeated refs. Explicit set/reference may use a fresh ID or retain the
slot ID when replacing its value. Public rows need no bindingId.

URL credential rejection covers the defined credential-name normalization,
obvious Bearer/Basic/private-key patterns, userinfo, and unsupported authoring
modes (sensitive path segments, embedded credential queries, templates).
It does **not** guarantee detection of arbitrary unmarked secrets embedded in
otherwise valid URLs. The browser explains this boundary and requires URLs
to be public; designated sensitive query values belong in separate rows.

Public fields retain `{name,value,enabled,sensitive:false}`. Protected headers
and query rows use `{name,enabled,sensitive:true,secretWrite}` on input and
`{name,enabled,sensitive:true,masked:true,secretRef}` on output. There is no
`value` property on protected reads or Control descriptors. A secretRef is
`{secretId:string}`; revisions are current safe metadata, not pinned values.

| Write                            | Meaning                                                                                                            |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `{action:"set",value:string}`    | Create a fresh encrypted project secret and bind it; on an existing slot, replace only that binding with a new ID. |
| `{action:"preserve",secretRef}`  | Keep a reference already present in this request's prior configuration; cannot introduce a new binding.            |
| `{action:"secretRef",secretRef}` | Explicitly bind an eligible existing secret from this same project.                                                |
| `{action:"remove"}`              | Remove this row/binding; never turn a mask or empty value into plaintext.                                          |

Omitted PATCH fields preserve current definitions. Supplied arrays still replace
whole collections; omitted rows detach their bindings. A removed whole body is
`body:null`. Empty strings are not implicit preserve/remove commands; set values
must be nonempty. Public reads are not directly accepted as write DTOs; the
editor turns protected reads into explicit preserve writes.

**Shared-reference example:** A and B both reference S. Setting a new value in
A creates T: A uses T, B still uses S. Removing A's binding leaves B/S intact.
Explicit PATCH of S changes the current value for every S consumer, increments
S's revision and the project gate, and does not change A/B request revisions.
Explicit DELETE of S revokes it for every consumer and clears ciphertext/wrapped
key/nonce/tag. Consumer descriptors remain safe refs and display unavailable;
they do not silently fall back to another value. The browser distinguishes
"replace for this request" from "replace shared secret" and warns that shared
replacement/revocation affects other requests without claiming an exact count.

Request reads remain available with revoked/missing refs so owners can repair
them. Unrelated PATCH can preserve an already-bound unavailable ref, but cannot
attach it elsewhere. `secretRef` eligibility requires a recognized live reusable
saved-secret record in the same project; environment/runtime/job/input/output
bundles, unknown schemas, revoked records and cross-project IDs are ineligible.
Eligibility also requires a compatible stored value kind: `string`, `jsonBody`
or `jsonScalar`. Header reuse requires a string validated as HTTP header-safe
at set time; JSON scalar secrets cannot be attached as raw headers. Store safe
validation capabilities rather than decrypting existing secrets in the API.
Secret PATCH retains its kind and repeats its validation. Metadata may expose
kind/eligible target types, but no property revealing the value's content.
Ownership is checked before validation. Ineligible references produce a fixed
400 with no existence disclosure; foreign project/resource routes remain 404.

## Credentials, body paths and URL boundary

Bearer authoring creates a sensitive Authorization header whose encrypted value
includes the `Bearer ` prefix. API-key authoring creates a sensitive header or
query row; cookie and credential-name recognition remains mandatory even if
the caller submits sensitive=false. Require the protected union for those
fields; reject plaintext union variants instead of silently persisting them.
Ordinary user-designated rows use the same protected union.

Whole sensitive text/JSON bodies use
`{type:"text"|"json",sensitive:true,secretWrite}` and read with masked/ref
metadata only. JSON plaintext is validated in memory, without echoed errors.
Partially sensitive JSON uses
`{type:"json",text:string,sensitive:false,secretFields:[{bindingId,pointer,secretWrite}]}`;
reads substitute `secretRef`/`masked:true` for each write.
`pointer` is an RFC 6901 pointer identifying an existing scalar/null leaf in
the JSON text. Public JSON at protected paths must contain literal null,
never secret plaintext or a mask. Set values are JSON-encoded scalar values
(string, number, boolean or null), encrypted verbatim after strict parsing.
Reject duplicate/overlapping pointers, container values, missing paths, invalid
escapes, duplicate object keys and depth >32. Credential-named JSON leaves
must have protected pointers; container subtrees require a whole sensitive body.
Public body text/formatting remains exact, with null placeholders supplied by
the client. The browser offers write-only per-path values and a public JSON
template; it never inserts entered secrets into the public template. Removing
an individual binding leaves the public template exact; remove credential-named
keys from that template as well, or retain their protected pointer.

URLs remain public absolute HTTP(S): no userinfo/fragments/templates, embedded
credentials or signed credential query values. Sensitive query parameters use
separate ordered rows rather than an inline URL. Sensitive URL path segments
and arbitrary text offsets remain unsupported and fail closed; this restriction
must be visible in authoring. No execution/resolution is implemented here.

## Encryption, limits and transactions

Protected uses `PK=P#projectId`, `SK=SECRET#secretId`, no indexes/stream/TTL,
Standard on-demand, PITR and deletion protection. A schema-1 saved-secret record
has project scope, reusable purpose, revision/state, safe timestamps, opaque
provenance and a versioned envelope. Each set/shared replacement obtains a fresh
AES-256 data key with KMS GenerateDataKey and encrypts with AES-256-GCM, fresh
96-bit nonce and 128-bit tag. KMS context and AEAD associated data bind
app/stage/project/secretId/purpose/version using only opaque IDs/constants.
Envelope contains algorithm/version/key ARN/wrapped key/nonce/ciphertext/tag.
No data-key cache or plaintext fallback. Clear plaintext/key buffers as far as
Go/browser runtimes permit; no guarantee of complete garbage-collector erasure.

Retain <=8 KiB plaintext and <=16 KiB complete envelope per secret. Use
**at most 16 protected slots per saved request** (including body pointers),
with complete post-transformation configuration <=64 KiB. References,
descriptors, body pointers, defaults and JSON escaping all count. Plaintext is
bounded separately, never persisted as part of the configuration-size count.
Keep the existing 128 KiB decoded transport cap; therefore 16 maximum-sized
secrets may require multiple deliberate edits. Do not inflate transport limits.
Enforce existing <=128 KiB Control item and accepted <=2 MiB application
transaction budget on fully encoded actions, plus DynamoDB item/action limits.

Encrypt before persistence; atomically commit new Protected records, Control
configuration, stage/user guards and request/project CAS in a cross-table
transaction. Condition-check each unique reused active source revision; consolidate
actions per item. KMS failure writes nothing. Request conflict writes neither
configuration nor envelopes. Lost transaction acknowledgement returns 503;
never replay automatically. Reconcile request metadata/ref IDs after refresh.
Shared PATCH/DELETE uses secret revision CAS and project gate CAS; revoked
tombstones prevent identifier reuse and retain safe metadata only. Secret-list
pagination uses strong Protected partition queries with bounded pages and
signed owner/project/stage/generation-bound cursors; no new GSI.

## Project cleanup and capability rollout

Advance the existing `savedRequestsSchemaVersion` capability to **2** and add
`protectedSecretsSchemaVersion=1`, conditionally activated only after both
deployed handlers/resources are ready. The current binary's activeStage rejects
savedRequestsSchemaVersion other than 0/1, so older binaries cannot perform
Control-only cleanup after activation. Keep existing public request record
schema 1 readable; use request schema 2 for extended protected configurations.
Do not reset recovery
generation or relax active-stage/user/project gates. Older binaries must fail
closed against the new capability rather than complete Control-only deletion.

Extend WORK with durable bounded Protected continuation, versioned alongside
the deleting project gate. Drain only exact recognized saved-secret/live/revoked
records; validate all attributes/keys/scope/envelope shapes. No decryption is
needed for cleanup. Preserve unknown fields/kinds/schemas, foreign identity
records and runtime bundle keys. Restart complete cycles from the beginning;
unknown records or read/write uncertainty keep cleanup pending. Completion
requires strong whole-project partition emptiness proofs in **both** tables
and the existing final stage/deleting-gate/deletion-epoch/WORK CAS transaction.
All legitimate writers use that gate, so no protected record can commit after
deletion acceptance. Backups retain the accepted restore/resurrection exception.

## Resource preview and cost assumptions

Read-only inventory on 2026-10-07 verified account 747336059622, non-root
`davian-admin`, us-east-2. Control is the only table. All six KMS keys map to
AWS-managed service aliases; there is no existing customer stage key to reuse.
Recheck before deployment.

Expected additions: one Protected table and one symmetric dev-api KMS key
with annual rotation/retention, plus alias. Expected scoped updates: existing
API/cleanup Go packages/environments/IAM, three secret routes and invocation
permissions, conditional capability migration. Cleanup has no KMS decrypt
permission. API initially needs GenerateDataKey only; no runtime decrypt route.
Integration tests exercise decryption using the test operator identity only.
No Cognito, worker, queue, S3 evidence, frontend hosting, VPC or extra schedule.
Review actual SST diff/bootstrap effects before deploying; this is not yet a
generated infrastructure diff.

Preliminary incremental forecast: **$1.10–$2.00/month** before free tiers,
credits or discounts for 10,000 additional API operations, 1,000 set/replacement
calls, <=1,000 retained secrets averaging 4 KiB and modest cleanup/metadata
traffic. New unrotated KMS key contributes $1/month; 1,000 symmetric calls
contribute $0.003. Allow $0.10–$1 for table transactional reads/writes,
storage/PITR, Lambda duration, API calls and sanitized log/bootstrap growth.
First/second key rotations each add $1/month. Key reuse would remove new-key
storage cost, but inventory currently rules that out. This is a forecast,
not measured spend or a hard cap; heavy edits/large configurations cost more.
Pricing checked against [KMS pricing](https://aws.amazon.com/kms/pricing/)
and [DynamoDB pricing](https://aws.amazon.com/dynamodb/pricing/).

## Required implementation evidence after acceptance

- Repository formatting, lint, frontend/infra tests/build, Go tests/vet/build.
- Meaningful local encrypted-envelope and DynamoDB integration tests: two-owner
  isolation, cross-project/runtime/ref misuse, missing/revoked sources,
  plaintext rejection, size limits, wrong-context/tampered decrypt refusal,
  preserve/local replacement/shared replacement/revocation, CAS races,
  KMS failure, cancellation, lost acknowledgement and restart cleanup.
- Actual AWS KMS/encrypted persistence and cross-table transactions; deployed
  authenticated HTTPS API, scoped IAM and scheduled cleanup. Confirm unknown
  Protected records hold deletion pending until deliberately removed by tests.
- Disposable randomized secrets held only in test memory; inspect Control,
  API responses, bounded CloudWatch logs and browser storage by boolean
  containment checks. Never print input values, payloads, decoded ciphertext,
  tokens or a secret-containing assertion diff. Protected stores ciphertext
  only; operator decrypt check compares in memory and reports pass/fail.
- Chromium fixture authoring and storage checks, distinct from genuine Cognito
  browser validation guided with Davian. Verify blank/write-only inputs after
  every submission attempt, preserved unrelated edits, two-request sharing,
  local/shared replacement distinctions, revoked-state repair, concurrent tabs,
  uncertain-write refresh, second-user denial and scheduled project cleanup.
- Synchronize accepted contracts/proposal addendum and Trello by readback;
  preserve unrelated proposed contracts and incomplete checklist items.
  Commit/push the verified slice only after actual results/gaps are recorded.
