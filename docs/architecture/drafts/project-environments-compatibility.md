# Environment protocol compatibility amendment

Status: **Proposed, unaccepted**, 2026-10-22. Companion to the
[environment review packet](project-environments-review.md). This amendment
replaces that packet's deferred protocol/version choices, without changing its
resolution, selection, freezing, rerun, sensitivity, lifecycle or limit semantics.
It allocates numbers within this proposal; it does not authorize activation.
All named wire objects are closed: duplicate/unknown keys, explicit null where
not allowed, unsafe integers, unknown versions and inconsistent combinations reject.
IDs use the existing UUID rules; revisions are integers 0..9007199254740991.
Errors use fixed codes, never input fragments, variable names, values or parser text.

## Version allocation and reader gates

| Domain                            | Legacy                             | Proposed allocation                                                                                                                                    |
| --------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Owner execution evidence          | 1 capture; 2 cloud assertions      | **3 environment/template context**, with optional existing cloud assertions                                                                            |
| Request configuration             | missing template marker is literal | `templateSemanticsVersion:1`                                                                                                                           |
| Internal frozen template snapshot | absent                             | `schemaVersion:1`                                                                                                                                      |
| Local executable Work / Result    | 1 / 1                              | **2 / 2** for template-enabled requests                                                                                                                |
| Local negotiation                 | absent, legacy only                | `schemaVersion:1` capability declaration                                                                                                               |
| Protected saved-secret row        | 1, no scalarType                   | 2 with trusted scalarType                                                                                                                              |
| Stage gates                       | accepted existing tuples           | savedRequestsSchemaVersion **9**, projectEnvironmentsSchemaVersion **1**, protectedSecretsSchemaVersion **2**, localAgentExecutionsSchemaVersion **2** |

The proposed stage tuple retains cloudExecutionsSchemaVersion:1,
executionInputsSchemaVersion:1, evidencePinningSchemaVersion:1,
sequentialWorkflowsSchemaVersion:1, mcpReadOnlySchemaVersion:1 and
workflowAssertionsSchemaVersion:1. These are an explicit future tuple, not a claim
about deployed state. Every strict stage predicate, writer, reader, cleanup and
recovery tool must recognize the complete tuple before a separately reviewed
closed-stage activation. Unknown schemas remain preserved and unavailable, never
guessed or deleted. Inventory, migration mechanics and activation remain review gates.
Saved request rows with the marker use row schemaVersion:3; unmarked rows remain
1/2. Frozen plan additions are gated by the environment capability, not an
in-place change to existing version-1 Work or immutable execution context.

**Evidence 3 is reserved here exclusively for environments.** The OpenAPI packet's
earlier proposed use of 3 is superseded by an **unallocated future version**;
4 is not allocated or accepted either. Library acceptance does not approve any
OpenAPI production contract. Never ship two meanings of evidence version 3.

## Owner evidence: exact additions

Evidence 3 has all existing version-1 fields with unchanged types:
`schemaVersion, projectId, executionId, source, target, submittedAt, startedAt,
completedAt, status, outcome, request, response, timing, redaction`.
`source` remains exactly `{requestId,revision}`. Add exactly these two required
fields: `templateSemanticsVersion:1` and `environmentContext`.
The latter is either null (explicit no selection) or exactly
`{environmentId,revision}` from admission. No variable names/IDs/maps, environment
name, values, secret IDs, template text, current revision or execution inputs.
The context is the effective step selection for workflow evidence, not its parent
default. Deletion and later edits never rewrite it.

Optional `assertions` is legal only for cloud workflow evidence whose frozen
assertion semantics is 1; then it is required, including `not_configured`, with
the existing schema-2 evaluation validation and frozen rule correspondence.
It must be absent (not null) for local/standalone or legacy assertion semantics.
Evidence 3 requires request template semantics 1 even when there are zero tokens.
Legacy requests continue to produce evidence 1 or 2, with neither new key; a
selector for an unmarked request rejects even if no variable would be used.
The owner detail/download serves stored version 3 unchanged; owner summaries add the
same two context fields only for version 3 using a dedicated owner DTO; shared
legacy/MCP Summary types must not grow these fields. Never silently downgrade an owner read.

Concrete context examples (fragments; other existing evidence fields required):

```json
{
  "schemaVersion": 1,
  "source": {
    "requestId": "11111111-1111-4111-8111-111111111111",
    "revision": 0
  },
  "target": "local"
}
```

```json
{
  "schemaVersion": 3,
  "source": {
    "requestId": "11111111-1111-4111-8111-111111111111",
    "revision": 4
  },
  "target": "local",
  "templateSemanticsVersion": 1,
  "environmentContext": {
    "environmentId": "22222222-2222-4222-8222-222222222222",
    "revision": 7
  }
}
```

Reject version 1/2 carrying either new key (even null), version 3 missing either
key, template version 0/2, context `{}`/extra keys/negative revision, local
assertions, or contractValidation on version 3. A failed execution still retains
its admitted context; no admission means no fabricated execution/evidence.

## Saved configuration, selection and frozen plan

Saved Configuration adds only optional `templateSemanticsVersion:1` to the
existing `name,method,url,queryParameters,headers,body,operationRef` fields.
Absence is distinct from null/0 and keeps literal semantics forever. No environment
ID/default or expanded value is stored in Configuration. PATCH preserves an
omitted marker; explicit 1 opts in; null/0/removal is invalid. To return to literal
semantics, create a new unmarked request. Existing descriptor bindings, secret
refs and options retain their accepted formats. New rows/edits retain <=64 KiB.

Legacy example: `{"name":"literal","method":"GET","url":"https://example.test/{{id}}","queryParameters":[],"headers":[],"body":null,"operationRef":null}`.
Adding `"templateSemanticsVersion":1` to that exact configuration enables the
path token; its missing id now rejects rather than becoming a literal or empty
string. Selection belongs to the top-level submission policy body `environment:null|{environmentId,
revision}`, default null, or workflow step `environment:{mode:"inherit"}` /
`{mode:"none"}` / `{mode:"select",environmentId,revision}` as already proposed.
Selectors enter idempotency identity; omission normalizes to the stated default.

For marked requests the internal frozen Plan retains all existing fields and
adds required `templateSnapshot`, exactly:

```json
{
  "schemaVersion": 1,
  "environmentContext": {
    "environmentId": "22222222-2222-4222-8222-222222222222",
    "revision": 7
  },
  "variables": [
    {
      "variableId": "33333333-3333-4333-8333-333333333333",
      "name": "count",
      "valueType": "number",
      "sensitive": false,
      "value": { "type": "number", "decimal": "1.0000000000000001" }
    },
    {
      "variableId": "44444444-4444-4444-8444-444444444444",
      "name": "token",
      "valueType": "string",
      "sensitive": true,
      "secretRef": { "secretId": "55555555-5555-4555-8555-555555555555" }
    }
  ]
}
```

Plan.configuration is the original normalized template configuration with its
marker, never a partially expanded mixture. Variables are exactly the enabled
used set, ordered by ASCII name, unique IDs/names; public rows have only `value`,
protected rows only `secretRef`. `valueType` matches the tagged primitive union.
Context may be null only with an empty variable array; selected environments
with no used variables still retain their context. No enabled flag, secret
revision, live name lookup or redundant template graph: compile bounded locations
from the frozen configuration using semantics 1. Existing direct protected bindings
remain separate and overlapping targets reject. The snapshot is retained only
in owner-authorized rerun context, never evidence/MCP or public job summaries.
It is counted together with Configuration against the 64 KiB frozen limit.

Snapshot protected values are absent from Control. Existing encrypted job/run
input values use exact secretId keys. For number/boolean/null inputs their strings
contain complete JSON scalar text; string inputs contain literal text. The
snapshot's immutable valueType governs decoding, with the same exact decimal
parser and bounds. Freeze copies independent buffers. Failed lookup/shape/type
never defaults. Reruns copy this snapshot/public values and exact refs, recheck
current eligibility/type and encrypt current secret values as already proposed.
No secret PATCH changes the frozen valueType or selection revision.

## Local negotiation, grant and verification

Add agent-token-only `POST /api/v1/local-agent/capabilities`, no-store, bounded
1 KiB request. The complete supported request/200 data object is:

```json
{
  "schemaVersion": 1,
  "workVersions": [1, 2],
  "resultVersions": [1, 2],
  "templateSemanticsVersions": [1],
  "environmentContextVersions": [1]
}
```

Arrays must equal these sorted sets exactly for this release; no optimistic
highest-version negotiation or arbitrary extension keys. The declaration is
persisted against credential ID/version and recovery generation. First write or
identical retry succeeds; conflicting declaration gives 409 agent_incompatible.
A new/recovered credential starts without it and must declare again. Missing
declaration means only Work/Result1, no template capability. This is a claimed
capability by an already trusted device, not attestation or a new permission.
Dedicated **owner** agent DTOs add `capabilities` (null or that object).
The agent-token GET `/api/v1/local-agent/session` keeps its exact legacy Agent
DTO with no new keys: older agents decode it strictly. The new POST response
alone acknowledges capabilities as `{data:<declaration-object>}`. Neither response
discloses protected inputs. Legacy agents may continue legacy work.

For any marked local request, admission requires the declaration above in the
same guarded credential-version/generation checks as authorization, **before**
creating a job, receipt, quota reservation, encrypted input or 202. Otherwise 409
`agent_incompatible`, no value/name details and no HTTP attempt. Recheck at lease
and executable grant; incompatibility yields existing source-unavailable terminal
handling, never downgrade or replay. Replacing/revoking a credential cannot inherit
its declaration. The agent strictly validates Work2 before recording dispatch
intent or issuing HTTP. Local workflows remain unsupported.

Work2 has **all Work1 keys and types**, schemaVersion:2, plus required
`templateSemanticsVersion:1` and `environmentContext` (same null/object as above).
Plan must contain the marked configuration and templateSnapshot described above;
its snapshot context must exactly equal the top-level context. No new endpoint
fetches environments/secrets. protectedInputs.values retains the existing
secretId-to-string format and transient-only exposure; primitive decoding uses
snapshot metadata. Both ends resolve identically and check resolved destination
against Work.destination, Plan.destination, consent scope and exact-origin rules.
The 128 KiB grant limit includes every new field and protected value.

Result2 has exactly existing `schemaVersion,leaseId,fence,nonce,capture` plus
required `templateSemanticsVersion:1,environmentContext`; capture keeps its
existing exact shape and contains no environment fields. Result1 remains exact
legacy format. A Work2 requires Result2 even for failures and null context.
Unknown/missing/mismatched versions, marker, IDs or revision give fixed 400
invalid_request and no accepted result/evidence. Different valid content for an
already accepted lease follows existing 409 receipt-conflict rules.

Version-2 canonical result uses the existing RFC 8785 procedure over the **entire
Result2 object**, including explicit null context, nonce/fence and both new fields,
before defensive sanitization. Do not use the current version-1 canonicalizer,
which rejects 2. Server compares identity against the frozen grant/lease before
acceptance, independently reconstructs/sanitizes request capture from the frozen
template plan and encrypted inputs, and builds owner Evidence3 with server-held
context. It never copies identity from untrusted capture or a live environment.
The accepted receipt retains version, marker, context and the canonical identity
HMAC with existing credential/fence/nonce binding; retries after input cleanup
verify that retained identity without decrypting deleted inputs. Raw results and
protected input values are not retained for this purpose. Context mismatch is
not repaired by silently substituting server IDs. Failed/unknown outcomes and
intent/deadline/no-replay semantics remain unchanged.

Example Result2 identity fragment: `{"schemaVersion":2,"leaseId":"66666666-6666-4666-8666-666666666666","fence":1,"nonce":"<existing 32-byte base64url nonce>","templateSemanticsVersion":1,"environmentContext":null}`;
`capture` is additionally required. Omitting context is not equivalent to null.
Changing revision 7 to 8 must change canonical identity and fail grant comparison.

## Reusable secret type metadata and PATCH

Protected saved-secret row schema 2 adds required `scalarType`, a closed enum
`string|number|boolean|null|untyped`. Existing keys/envelope rules remain;
this is not runtime-output or job-input metadata. Public owner SecretMetadata
adds `scalarType` with the same enum and never values. Schema-1 reads project
string as `string`, jsonScalar/jsonBody as `untyped`, without rewriting storage.
Schema-2 combinations: valueKind string requires scalarType string; jsonScalar
requires number/boolean/null; jsonBody requires untyped. Never allow new untyped
jsonScalar rows. Legacy jsonScalar (even currently a string scalar) stays untyped
and cannot attach to a typed environment. Legacy strings remain attachable;
jsonBody is never an environment scalar. Typed protected strings retain the accepted secret nonempty requirement within
2 KiB; literal empty **public** strings remain allowed as in the environment
packet. No envelope/ciphertext validation relaxation is implied.

Environment `set` derives metadata from the tagged union on the server; attaching
or preserving verifies same project, active SAVED-secret kind and exact valueType.
No client-writable scalarType on secret PATCH, no inference from a masked value,
no casting string to number and no implicit metadata upgrade of legacy rows.
Migration of old untyped refs means explicit environment set to a fresh secretId.
Existing direct secret slots continue to obey their existing valueKind checks.

`PATCH /projects/{projectId}/secrets/{secretId}` stays exactly `{"value":string}`
with existing owner gates, quoted If-Match and revision/CAS/error semantics.
For typed string the value is literal text, including `"null"`; for typed number,
boolean or null it is a single complete JSON primitive of that exact type.
Reuse StrictJSON/UseNumber and the exact decimal domain, never float64. Reject
extra/duplicate keys, missing/null value, arrays/objects, cross-type values,
unsupported decimal, overflow or >2 KiB complete typed scalar. Legacy schema-1
PATCH keeps its accepted 8 KiB/nonempty/valueKind behavior and remains untyped
where applicable. Type cannot change in place, even without an active environment
reference; no unbounded reverse-reference scan. Revocation retains scalarType and
removes envelope. Type and kind enter atomic source conditions, not just an
unguarded preflight. Existing environment bounds can still reject attaching a
legacy string over 2 KiB.

Examples: typed number PATCH `{"value":"1.0000000000000001"}` preserves its exact
decimal characters; `{"value":"\"1\""}` rejects as string. Typed boolean accepts
`{"value":"false"}`, rejects `{"value":"0"}`; typed null accepts
`{"value":"null"}` but rejects `{"value":null}`. Typed string accepts
`{"value":"false"}` as string; `{"value":"x","scalarType":"number"}` rejects.

## MCP projection without evidence mutation

The seven existing tools and their wire schemas remain unchanged. Recognized,
validated owner evidence3 projects using a dedicated version-1 evidence DTO,
selecting exactly the legacy fields listed above and schemaVersion:1; never embed
or spread the owner object. For the complete view, the existing Evidence envelope uses `view:"complete"` and
`capture` (the dedicated legacy DTO). The supported view values remain exactly
`summary|complete`; `capture` is a field name, never a view value. It retains `complete:true`,
`truncated:false`, with `omissionReason:"environment_context_not_exposed"` and
`omittedFields:["templateSemanticsVersion","environmentContext"]`. If assertions
are present, use reason `unsupported_context_not_exposed` and ordered fields
`["assertions","templateSemanticsVersion","environmentContext"]`. Version2
keeps its existing assertions_not_exposed behavior. These fixed strings fit the
existing string/list fields; they add no tool or evidence keys. For evidence summary view retain `complete:false`, `truncated:false` and
`omissionReason:"mcp_summary_view"`; append these unsupported fields in the same
order to the existing summary-view omittedFields list, without duplicating keys.
List-execution and run-step summaries have no omission envelope: use dedicated
legacy summary DTOs with only existing fields, not expanded owner DTOs.
Unknown versions or invalid
version3 evidence fail closed with existing fixed unavailable errors, not an
attempted projection. Pins/tickets/owner downloads still refer to the original
immutable version3 bytes/digest; projection neither overwrites nor rehashes them.
Redacted rendered capture remains visible under existing policy; unsupported
context omission is not truncation or redaction of a stored execution.

## Review and verification boundaries

[Compatibility experiments](../../../spikes/environment-resolution/compatibility_test.go)
exercise strict legacy/new decoding, version presence, independent environment
identity/canonical bytes, pre-admission capability refusal, typed PATCH and explicit
MCP omission without mutating source bytes. They are disposable test DTOs, not
production protocol handlers, signing, persistence or a complete validator.
The original resolution experiments and all bounds remain applicable.

Review must accept this allocation and exact DTOs together with the environment
semantics before implementation. Closed-stage migration/inventory, all strict
reader changes, true atomic admission/receipt races, whole sanitized capture,
agent/server parity and real end-to-end delivery remain implementation obligations.
OpenAPI cloud-only validation, sensitive-body suppression and its future evidence
schema remain **separately unaccepted**, with no implied approval here.
