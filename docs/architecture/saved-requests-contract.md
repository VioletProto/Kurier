# Saved requests slice contract

Status: **Accepted by Davian, 2026-10-07**, with explicit PATCH clearing and collection replacement clarification.
Scope: public, non-sensitive saved definitions only; no execution, outbound HTTP,
evidence, environments, imports or encrypted secret storage.

## Routes and versions

Under `/api/v1/projects/{projectId}/requests`:

| Method | Path           | Input                         | Result                                 |
| ------ | -------------- | ----------------------------- | -------------------------------------- |
| POST   | collection     | Complete configuration        | 201 `{data:{request}}`, Location, ETag |
| GET    | collection     | limit/cursor                  | 200 `{data:{items,nextCursor}}`        |
| GET    | `/{requestId}` | none                          | 200 `{data:{request}}`, ETag           |
| PATCH  | `/{requestId}` | Supplied configuration fields | 200 `{data:{request}}`, ETag           |
| DELETE | `/{requestId}` | No body                       | 204                                    |

`request` adds server-controlled requestId/projectId/revision/createdAt/updatedAt
to the configuration. Revision starts at zero; PATCH increments it once, even
for an unchanged supplied value. PATCH/DELETE require the existing quoted decimal
If-Match grammar, applied to **request revision**, with 428 missing and 412 stale.
Every request mutation also CAS-increments the project gate version, so clients
must refresh project details before rename/deletion. Gate contention returns 409;
never automatically replay a write. Transport uncertainty returns 503, never
invented success or a stale-version error. Refresh detail/list to reconcile.
Authorization precedes validation/preconditions; foreign/missing/deleting IDs
return the same 404. Stage/generation/enabled-user guards remain transactional.

## Configuration and validation

POST requires name, method, url. Defaults queryParameters/headers to `[]`, body
and operationRef to `null`. PATCH changes only supplied fields, replaces supplied
arrays/body as a whole, and rejects an empty object. Omitted fields remain unchanged.
`body:null` clears the optional body; `headers:[]` and `queryParameters:[]`
clear those collections. Every supplied headers/queryParameters array replaces
the entire collection, never merges entries. `operationRef:null` clears it
(the only supported value here). Null arrays/name/method/url are invalid. Unknown and duplicate JSON
keys at every nesting level, invalid UTF-8, trailing JSON, null required fields
and unsupported union variants are rejected without echoing values.

Name follows project names: trim, 1–100 Unicode code points, no controls.
Methods: GET, POST, PUT, PATCH, DELETE (the current RequestDefinition enum).
URL: absolute http/https, nonempty host, no userinfo, fragments or controls.
Environment/template URL resolution remains deferred to its own slice.
Ordered query/header entries preserve duplicates and disabled rows:
`{name,value,enabled,sensitive:false}`. Header names are HTTP tokens, values
reject CR/LF/controls except horizontal tab; query names are nonempty without
controls. Arrays have at most 100 entries each. Header names are case insensitive
for credential rejection. Normalize lowercase names by removing -, _, dot and
spaces; reject names containing authorization, cookie, apikey, token, secret,
password, passwd, credential, privatekey, signature or sessionid, and exact key,
auth or pwd. JSON body keys follow the same rule; obvious Bearer/Basic/private-key
text is rejected. These checks do not identify arbitrary undisclosed secrets. Authorization, Proxy-Authorization, Cookie, Set-Cookie,
API-key/token/secret/password credential names and equivalent query names are
rejected even when disabled or sensitive=false. URL query credentials are rejected
likewise. Do not use credentials, signed URLs or personal/private body data.

Body is null, `{type:"text",text:"...",sensitive:false}` or
`{type:"json",text:"...",sensitive:false}`. JSON text must contain exactly one
valid JSON value with unique object keys and bounded nesting (32); text bytes
and formatting are preserved. User-designated sensitive fields/bodies,
secretWrite and secretRef inputs fail closed until encrypted storage and
same-project reference eligibility checks exist. No placeholder masks or fake
references are persisted. operationRef accepts null only in this slice.

The **complete normalized serialized configuration**, including name, method,
URL, queryParameters, headers, body and operationRef, must be <=65,536 UTF-8
bytes. Serialize compact JSON in the documented field order, use ordinary JSON
string escaping (no HTML escaping), count all descriptors/defaults/escaping.
U+2028/U+2029 serialize as six-byte \u2028/\u2029 escapes.
Apply after merging PATCH with stored configuration; never truncate. IDs,
revision and server timestamps are record metadata, outside configuration.
Transport JSON is separately limited to 128 KiB to permit formatting/escaping.
Stored item must stay <=128 KiB. Oversize configuration/transport returns 413.
Other validation/error envelopes follow the users/projects contract.

## Pagination and deletion

Reuse default 25/max 100, 24-hour signed owner/project/stage/generation-bound
cursors, high-water creation ordering, five bounded candidate pages and strong
hydration/active gate checks. GSI1 uses `LPK=P#id#REQUEST`, `LSK=createdAt#id`.
Results are eventual, not snapshots. Cursor reuse across projects is rejected.
A conservative 2 MiB serialized-item response budget may shorten a page before
the requested limit, with a nextCursor; continuation never skips the unreturned item.

Individual DELETE commits a minimal request tombstone with revision+1 and no
configuration/index keys. Missing/deleted requests return 404 thereafter; no
execution/history data exists in this slice. Project cleanup strongly queries
REQ records in bounded pages, removes only recognized schema-1 request/tombstone
records with exact project/key identity, under stage and deleting gate
version/epoch/operation CAS. Each chunk advances the gate; a restart can safely
resume a durable WORK continuation cursor; a complete cycle restarts from the
beginning to revisit unknown/uncertain records, without offsets.
Unknown children and unknown REQ schemas/kinds remain pending. Completion retains
the existing strong whole-partition emptiness proof and name-free project
tombstone. No Protected/S3 cleanup or unsupported children are assumed absent.

## Review boundary

The exact body union, request revision ETags, normalization/byte-count definition,
credential-name rejection and minimal request tombstone above are new concrete
wire choices. Davian accepted these choices before dependent CRUD/UI implementation.
Existing secret reuse/set/preserve/remove semantics remain accepted architecture;
their exact locator DTO/encrypted implementation are deferred, not replaced by
plaintext. Existing project cleanup can be extended independently using the agreed
request record identity; unknown entities must remain preserved.
