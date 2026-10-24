# Project environments and variable resolution — proposed review packet

## Accepted continuation — 2026-10-24

Davian accepted this packet and its compatibility amendment at
`d8be596a2306f2e888ef34e81dfa836bc4b84447`, including evidence 3,
Work/Result 2, typed-secret rules and the corrected MCP complete-view projection.
This acceptance supersedes the historical Proposed/unaccepted wording below
within the environment scope. Scoped implementation and local/CI validation are
authorized on `feat/project-environments`; publication is for implementation review.
AWS inventory/preview, deployment, migration and activation remain held for
separate review. OpenAPI's production import/validation contract remains unaccepted
and has no allocated production evidence version. Acceptance is not deployment
or genuine runtime evidence.

Status: **Proposed, unaccepted.** Prepared 2026-10-22 on
`docs/project-environments-review` from merged main `b24fb17680bcd5802950394bd2811a597d693014`.
This task delivers a contract proposal and isolated local experiments only.
No production implementation, deployment, activation, AWS reads or AWS mutations.

## Authority and source reconciliation

Read the current native [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
and [API Contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit).
The approved MVP includes project key/value environments, `{{variableName}}`
resolution before execution, sensitive variables and environment identity in
sanitized execution evidence. The API document proposes a revisioned Environment,
32 KiB bound, collection POST/GET and item PATCH/DELETE; concrete resolution and
selection rules remain open. This packet adds item GET and specifies reviewable
choices without treating the earlier wire examples as accepted implementation.

Read the [environment card](https://trello.com/c/K2e3Nv2f), its implementation
checklist, [contract-review card](https://trello.com/c/b5LdvuTo), dependencies and
board state. Doing was empty under the configured limit of three; the environment
card was moved to Doing for this preparation. Acceptance/implementation items
remain incomplete. Davian is the sole developer; no assignment change is needed.

Follow [accepted architecture](../README.md), [ADR 0002](../../decisions/0002-serverless-persistence-topology.md),
[requests](../saved-requests-contract.md), [protected inputs](../protected-request-secrets-contract.md),
[cloud execution](../cloud-execution-evidence-contract.md),
[workflows](../sequential-workflows-contract.md),
[assertions](../workflow-assertions-contract.md),
[local agent](../local-agent-execution-contract.md) and
[MCP](../read-only-mcp-contract.md). Later accepted continuations override
historical Proposed labels only within their scope. Environments are currently
excluded from those delivered execution protocols. OpenAPI library acceptance
does not accept the independent import/validation contract or this packet.

## Proposed environment CRUD and exact value DTOs

All routes are relative to `/api/v1/projects/{projectId}`. Cognito owner,
enabled-user, active stage/generation and active project checks precede resource,
body and precondition validation. Foreign/missing/deleting resources give the
same fixed 404. No-store applies throughout; agent and MCP tokens cannot manage
environments. No caller-supplied owner/project IDs in bodies.

| Route                                  | Proposed behavior                                                                                                                                                                                                                                  |
| -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| POST `/environments`                   | `{name,variables}`; 201 `{data:{environment}}`, Location and revision ETag. No automatic creation replay on uncertain outcome.                                                                                                                     |
| GET `/environments`                    | 200 `{data:{items,nextCursor}}`; metadata summaries only (ID/project/revision/name/timestamps), no variable values. Default 25/max 100; existing 24-hour owner/project/filter/stage-generation cursor, five candidate pages and 2 MiB item budget. |
| GET `/environments/{environmentId}`    | 200 `{data:{environment}}`, quoted decimal revision ETag; public values and masked protected descriptors only.                                                                                                                                     |
| PATCH `/environments/{environmentId}`  | Name and/or variables; omission preserves, supplied variables replaces the complete collection. Empty PATCH, null name/variables and unknown fields reject. `variables:[]` clears the collection.                                                  |
| DELETE `/environments/{environmentId}` | Empty body; 204 logical tombstone with revision+1. Removes live public data/list keys, permanently denies selection, preserves historical frozen context.                                                                                          |

Revision starts at zero. PATCH/DELETE require one quoted nonnegative decimal
If-Match for the **environment revision**: 428 missing, 400 malformed, 412 stale.
PATCH increments once even when supplied values are unchanged. Each mutation
also CAS-increments the project gate; a separate gate contention gives 409.
Uncertain writes give fixed 503 and require explicit refresh/reconciliation, not
an invented success or blind new POST. Repeated DELETE with its initiating
revision can succeed against the retained minimal tombstone; other deleted reads
remain 404. No automatic environment-name uniqueness or lookup by display name.

Environment adds `{environmentId,projectId,revision,createdAt,updatedAt}` to
`{name,variables}`. Display name follows existing trimmed 1–100-code-point,
control-free rules. Variables are ordered; names are unique **including disabled
entries**, case-sensitive ASCII `[A-Za-z_][A-Za-z0-9_]{0,63}`. Names/types are
public metadata, never a place to hide credentials. No objects, arrays, computed
values, nested environments or environment-variable references inside values.

Public variable write/read:

```json
{
  "variableId": "opaque-id-returned-for-existing-row",
  "name": "ratio",
  "enabled": true,
  "sensitive": false,
  "value": { "type": "number", "decimal": "0.10000000000000001" }
}
```

`value` is a strict union: `{type:"string",value:string}`,
`{type:"boolean",value:boolean}`, `{type:"null"}` or
`{type:"number",decimal:string}`. Null is a present typed value, not missing,
disabled, preserve or removal. No bare JSON number DTO and no implicit string
coercion. Preserve decimal characters exactly through browser text state, JSON,
storage, freezes and reruns; never Number/parseFloat/float64. Reuse the accepted
bounded exact decimal domain: one JSON number token <=128 ASCII bytes, absolute
exponent <=308, zero or magnitude 1e-308 through 9007199254740991 inclusive.
Authored `1`, `1.0`, `1E+00` and `-0.0` spellings remain distinct stored text.

POST/new PATCH rows omit variableId and receive fresh server IDs. Existing PATCH
rows must carry an ID present in the current environment. Name, value type and
sensitivity are immutable for that ID; replace/remove-and-add to change them.
Removed IDs cannot be supplied again as new rows. Reordering preserves identity;
changing an ordinary value or enabled flag preserves the ID. This avoids
same-name rebinding and an unbounded per-environment variable tombstone list.
Literal empty public string is allowed; it never means preserve or missing.
Duplicate IDs in a collection reject. Store one bounded Control environment row
under the project's environment key family; the observed row revision guards all
used variable membership checks together, not one transaction action per public
variable. New row kind/schema and safe deletion tombstones must join cleanup.

Protected variable write:

```json
{
  "variableId": "existing-variable-id",
  "name": "accessToken",
  "enabled": true,
  "sensitive": true,
  "valueType": "string",
  "secretWrite": {
    "action": "preserve",
    "secretRef": { "secretId": "opaque-id" }
  }
}
```

Set uses `{action:"set",value:<typed value union>}` in write-only memory.
Preserve requires the exact existing `(environmentId,variableId,name,valueType)`
and existing secret ID. Explicit `{action:"secretRef",secretRef:{secretId}}`
may bind a compatible live project-owned reusable **saved-secret** record.
Remove the variable by omitting it from a replacing collection; there is no
implicit conversion from a protected mask to a public value. Protected reads
replace secretWrite with `{masked:true,secretRef}`, retain valueType and never
return value/decimal, length, suffix, hash or plaintext-derived diagnostics.

Propose sharing the accepted project-owned reusable saved-secret lifecycle:
environment string values use string secrets; number/boolean/null use jsonScalar
secrets plus server-validated scalar-type capability metadata. Set values are
encrypted before a guarded cross-table commit; source IDs and capabilities,
never ciphertext or plaintext, live in Control. Reuse is not allowed for
environment/run/job runtime bundles or foreign/missing/revoked/unknown sources.
Legacy jsonScalar refs without trustworthy type metadata cannot be attached as
typed environment values until a separately reviewed migration validates them;
do not trust caller-supplied valueType as proof. Header compatibility must be
recorded/checked at set and render; body-only newline strings are not silently
made header-safe. Keep accepted nonempty protected-string sets; typed null has
the nonempty serialized scalar `null`. No secret reveal route or automatic
promotion of public values into secret storage.

Project-owned secrets remain manageable through existing `/secrets` routes.
Removing/deleting an environment detaches references; it does not delete or
revoke a shared secret used elsewhere. Explicit secret DELETE revokes every
consumer. Explicit secret PATCH changes future admissions/reruns, not accepted
encrypted inputs. A new set on one variable creates a fresh secret ID rather
than replacing a shared secret implicitly. This reuse/type-capability extension
and typed secret PATCH contract require review; existing saved-request secret
DTOs/eligibility are not silently broadened by this packet.

Keep the existing `/secrets/{secretId}` PATCH transport (`value:string`): string
secret values use literal text; jsonScalar uses the complete JSON scalar text.
For newly typed reusable secrets, scalarType is immutable and verified on every
PATCH. Replacing a numeric secret with a boolean cannot invalidate the environment's
frozen type promise silently. Legacy untyped jsonScalar semantics remain unchanged
for old consumers and are ineligible for this new typed-ref attachment.

## Syntax, locations and type-preserving resolution

Propose explicit request `templateSemanticsVersion:1` opt-in. Absence retains
legacy literal behavior forever, including template-looking body strings. New
writes of actual templates require the marker and the compatible capability.
Adding the marker deliberately interprets only the supported locations below;
old runs/reruns without it never gain interpolation after activation.

Syntax is exactly `{{variableName}}` after ordinary JSON string decoding.
No whitespace, dots, indexing, filters, defaults, expressions, functions,
environment/OS access, escaping language or recursion. Reject unmatched/nested
double delimiters and invalid names at save/submit, including disabled template
rows. Single braces remain literals. No backslash escape for a marker. A value
containing `{{other}}` is inserted once as inert data; literal marker text can
therefore be supplied as a string variable. Never evaluate returned values or
upstream evidence as templates. Do not use Go text/template or JavaScript eval.

| Location             | Proposed behavior                                                                                                                                                                                                                                                                                                                                                                     |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| URL base             | One leading whole public string variable, e.g. `{{baseUrl}}/v1/pets`, supplies a validated absolute http(s) base with optional static path. No userinfo, query, fragment, controls or markers in its value. No partial scheme/host/port substitution.                                                                                                                                 |
| URL path             | A whole path-segment public string placeholder, e.g. `/pets/{{petId}}`; percent-encode once as component data. Reject `.`/`..`, slash, backslash, percent, query/fragment delimiters and markers in dynamic segment values. No string/numeric coercion.                                                                                                                               |
| URL query            | No interpolation in inline URL queries. Templated URLs must use separate query rows; legacy literal URLs retain their accepted behavior. Sensitive values are forbidden in base/path or any public URL.                                                                                                                                                                               |
| Ordered query values | Resolve enabled row values, encode through the existing query encoder once; duplicate rows/order preserved. Sole placeholder accepts string/number/boolean, retaining numeric token spelling; null rejects. Mixed text accepts strings only. Names and enabled flags cannot be interpolated.                                                                                          |
| Header values        | Same scalar/mixed-string rules as query, then control/header-name/forbidden-transport validation. Complete rendered value <=2 KiB; CR/LF/NUL/DEL reject, HTAB follows accepted policy. No header-name interpolation or sanitizing invalid outbound bytes.                                                                                                                             |
| JSON body            | Parse strict JSON first. Only string **values** are templates. A whole string value exactly `"{{x}}"` is replaced by its typed primitive, including null and exact json.Number; root and array-element slots are supported. Mixed strings require string inputs and JSON escaping. No raw unquoted `{{x}}`, property-name interpolation, array resizing or containers from variables. |
| Text body            | String variables only, single-pass concatenation. Protected whole-body source text stays opaque; it is never parsed as a second template program.                                                                                                                                                                                                                                     |

Method, names, operationRef, environment/variable metadata, JSON keys, assertion
expectations/paths, extraction/binding definitions, redaction selectors and
existing protected literal slots cannot be templated. Reject overlapping direct
saved-secret JSON pointers/whole-body sources and template targets. The new
public template syntax is stored without resolved secrets; it cannot become an
alternative credential-entry field. Recognized credential destinations accept
only explicit whole-variable placeholders, or the bounded public `Bearer `
header prefix plus a string placeholder; no literal credential content.
Stored `sensitive:false` on public template text does not declare the resolved
output public. The UI must show protected source requirements without revealing
rendered values; producer/runtime sensitivity rules remain authoritative.
This is a deliberate extension of current credential-name validation. Ordinary
noncredential template literals remain public configuration and must satisfy
existing recognized-name/obvious-secret/designated-secret checks.

If a JSON body has no template replacements, preserve exact source bytes. If it
does, deterministic compact reserialization may change whitespace/key order;
preserve every untouched numeric token and JSON logical type, use non-HTML
escaping, and retain the original frozen template separately. Never substitute
strings into raw JSON text, trim values, percent-decode inserted text a second
time or normalize numeric spellings. Typed variable insertion does not extend
the accepted workflow-extraction numeric domain or coerce its outputs.

## Selection, precedence and missing inputs

Standalone submission extends its policy body with
`environment:null|{environmentId,revision}` (default null); request If-Match
continues to select request revision. No secret values or per-call value overrides
are accepted on execution routes. The browser selection is per operation in
memory, not a server global/default environment or persisted credentials.

Workflow run body uses the same default selector. Saved steps add:
`environment:{mode:"inherit"}` (omission normalizes here), `{mode:"none"}` or
`{mode:"select",environmentId,revision}`. A selected step environment overrides
the entire run selection; it does not merge variables from two environments.
Saving validates explicit selected revisions; admission revalidates all selected
environments. The run default is checked even if all steps override it; UI should
submit null when unused. Inconsistent expected revisions for one shared source
reject rather than choosing whichever was read last.

| Precedence question       | Proposed answer                                                                                                                                                                                                                                         |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Environment choice        | Explicit step select/none, otherwise run selection; standalone uses its own selector. No saved-request environment default, global variables or implicit last-used lookup.                                                                              |
| Variable value            | One enabled unique-name row in that selected environment. Disabled is missing; present null is a value. No fallback to another environment, secret label, OS variable, request field or same-named workflow output.                                     |
| Workflow extracted output | Remains an explicit producer-step/name binding. It is a separate namespace, never an implicit `{{name}}` source.                                                                                                                                        |
| Target conflict           | Reject a template destination overlapping a workflow binding or saved protected target, in either direction for JSON pointers. There is no hidden override or taint reset. Independent accepted bindings execute normally after environment resolution. |

Resolve only enabled execution destinations. Disabled request rows are parsed
for valid syntax/security at save, but do not load secret values or require live
variable values. Enabled variable references absent/disabled in the selected
environment, or selecting no environment for a used token, give fixed
variable_missing before admission. Names are public but errors still do not echo
them. No empty-string/default/partial request substitution. An empty public
string is present and valid where its destination permits it. Failed preflight
performs no HTTP, quota reservation or evidence fabrication. Loss of source
eligibility after admission yields existing source_unavailable/not_dispatched
and atomic workflow failure/skipped successors, never a replacement environment.

## Public/protected containment and freezing

Public means deliberately nonsecret public configuration. Reject recognized
credential variable names/obvious credential literals in public variables, and
require protected sources at recognized/designated sensitive destinations.
A public `sensitive:false` source cannot bypass that requirement by having an
innocent name. At admission, known protected-value/reflection matches in public
structural inputs fail closed as sensitive_input; this is not arbitrary-secret
detection or a retroactive erasure guarantee for misclassified public data.

Protected source sensitivity, destination designation and known reflected-input
provenance are ORed and cannot be lowered by wrappers or flags. Never embed a
protected source in URL base/path or persist resolved protected query values in
the public `url`. Accepted separately protected query rows still require an
ephemeral **outbound wire URI** containing the encoded value; this execution-only
transport exception is not permission for public URL previews, logs or evidence.
Require existing explicit fresh allowInsecureSecrets consent for sensitive HTTP
delivery; HTTPS remains the default. Validate the final resolved destination
against cloud DNS/IP/port/redirect rules or the local credential's exact origin
and address class. An environment does not widen local pairing consent.

Before 202: strongly read request/workflow and selected environments, collect
only used variable IDs, validate exact value kinds/target graph/known limits,
read active compatible reusable secret records, and prepare fresh encrypted
job/run inputs in authorized API memory. Freeze semantics version, selected
environment IDs/revisions, variable ID/name/kind mappings, original templates,
public used typed values, exact protected source IDs and safe policies. Unused
values are not copied. Neither future resolution nor history depends on a
mutable environment name or a newly fetched same-name row.

Protected values go only into the existing purpose-bound encrypted job/run input
envelope; never Control, SQS, idempotency/status fields or a public frozen plan.
Commit frozen plans/encrypted inputs, expected environment/request/source
revision checks, parent/stage/user/generation/quota gates and receipt/outbox
atomically within existing transaction ceilings. KMS/size/CAS failure publishes
nothing. Resolve an identical seven-day receipt **before** checking newer live
environment revisions: a retry returns the same accepted execution/run and never
re-freezes values or dispatches twice. Extend versioned private input HMACs with
the explicit selection/expected revisions and semantics marker; do not alter old
receipt identity domains or put raw keys/value hashes in public responses.

Ordinary environment name/value edits and shared secret replacements do not
change accepted values. Before cloud intent/local grant and workflow advancement,
strongly recheck selected environment active, used variable IDs still present
with the same name/kind/sensitivity and enabled, plus exact frozen secret IDs
active/compatible, request eligibility and all parent/recovery fences. Condition
the **fresh observed** revisions in the intent transaction; requiring equality
to the admission-time environment revision would incorrectly cancel jobs after
an ordinary edit. Do not substitute newer values after a CAS retry.
Changing a variable to a fresh secret ID does not replace the old frozen ID;
the old ID remains usable only while that exact source is active.

Removing an ID or deleting an environment permanently blocks its future use;
recreating the same name cannot resurrect it. Enabled=false blocks while false;
reenabling can make an otherwise pending frozen job eligible, but never restarts
a terminal failure or replays HTTP. Use variable removal/environment or secret
deletion for irreversible revocation. Revocation after committed dispatch/grant
cannot recall an already authorized operation or device-held plaintext.

Workers/agents resolve once into transient buffers. Add protected raw, scalar
JSON, rendered and supported encoded forms to the existing bounded redaction
corpus before any request/response/error evidence publication. Mask complete
credential headers and sensitive query/JSON/text locations; the API defensively
sanitizes local results independently. Unsafe encoding/path/reflection handling
omits the affected capture, never publishes an unsafe best effort. Dispose of
encrypted runtime inputs on terminal publication/settled cleanup; pinning never
retains runtime secrets. Go/JS memory cleanup is best effort, not physical erasure.

MCP keeps seven read-only tools and no environment-management/value-resolution
endpoint. Catalog/status/audit allowlists exclude environment variable maps,
bindings and secrets; no decrypt/re-resolve on reads. New owner evidence includes
only immutable environment ID/revision/semantics metadata, not protected values
or full variable maps. Any projection update must keep explicit unsupported-field
omissions and current size/auth/audit/retention gates. Logs and diagnostics never
contain names/values, templates, resolved URLs, ciphertext, fingerprints or KMS/
storage/network exception text. Use fixed codes with opaque correlation IDs.

## Reruns, target coverage and lifecycle

Deliberate standalone/whole-run reruns create fresh IDs/keys/admissions and fresh
HTTP-secret consent. Keep frozen public used values, templates, selected IDs and
semantics; fetch current eligible values at the **exact frozen secret IDs**.
Never read a newer public environment snapshot, select another environment or
reuse historical runtime ciphertext/producer outputs. Missing/deleted selected
environment/variable/request/secret or missing retained frozen context blocks
rerun. A different environment is a deliberate fresh submission, not rerun.
Legacy executions/runs lacking the marker keep literal behavior and unavailable
environment context, including after upgrade. No automatic replay of unknown
HTTP outcomes and no retry-from-workflow-step.

Propose cloud standalone, local standalone and existing **cloud-only** workflows
in the first environment slice. Local workflows remain unsupported. All three
supported paths must use the same frozen semantics and golden vectors; do not
ship one target with different coercion/precedence rules under the same marker.
The local server sends environment-specific frozen inputs only in the authorized
execution grant, not ordinary polling/status/browser DTOs. Agent cannot fetch
environments or secrets. Extend strict grant/result version/capability negotiation;
older agents must be rejected before admission/grant, not silently execute literal
templates. Public target origin is resolved/validated before grant authorization;
server and agent recheck exact-origin policy. The agent resolves remaining
protected bindings only in memory and reports the same safe environment identity.
Untrusted paired devices retain the accepted trust limitation.

Environment deletion returns 204 after authoritative tombstone/denial and removes
live public variables; frozen public history remains under the existing execution/
run context lifetime and pinning rules. Live links become null; frozen IDs/revisions
do not change. Do not edit immutable evidence or retained rerun plans. Project
deletion overrides pins, denies every environment/input/grant immediately and
resumably drains environment rows/tombstones, project-owned Protected records,
job/run inputs, receipts, evidence/tickets and all known child types. Preserve
unknown schemas and uncertain/late upload obligations; no authoritative TTL or
unbounded refcount/reverse scan. Keep the accepted backup-restore exception and
closed-stage recovery generation: no restored nonterminal execution replay.

## Bounds and compatibility changes requiring review

| Boundary             | Proposed/preserved ceiling                                                                                                                                                                                                                                                                                                                                                        |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Environment          | <=32 variables, <=16 protected slots, <=2 KiB complete scalar each; complete normalized owner Environment DTO including IDs/defaults/masks/refs <=32 KiB. Ciphertexts counted separately in existing <=8 KiB plaintext/16 KiB envelope caps. Decoded write transport <=128 KiB.                                                                                                   |
| Template work        | <=32 references/location, <=128 references/complete request, <=32 distinct used variables, JSON depth <=32 and <=16,384 visited JSON nodes. No recursive expansion or container-valued inputs. Bound allocation before concatenation and total serialized output afterward.                                                                                                       |
| Request/runtime      | Complete saved and frozen configuration independently <=64 KiB, including marker, template graph, selection/context and public frozen values. Resolved body and complete header block independently <=64 KiB; each rendered header <=2 KiB. Existing 100 rows/query/header and policy/source bounds remain.                                                                       |
| Inputs/workflows     | Existing <=8 saved-secret inputs/step, <=16 distinct live inputs/run, <=18 total deduplicated workflow/request/environment/secret checks; environment sources consume this budget. <=4 distinct environments/run as a proposed extra cap. <=32 KiB complete encrypted input envelope and <=256 KiB aggregate frozen plans+initial bindings; <=10 steps. Some combinations reject. |
| Storage/transactions | Existing <=128 KiB serialized item, <=80 actions/2 MiB, <=60 workflow planned actions. Include both-table source checks, encryption framing, terminal/skipped cleanup and existing quotas. Do not split atomic admission to fit.                                                                                                                                                  |
| Capture/delivery     | Existing <=2 MiB wire/decoded response, <=4 MiB evidence/API, <=6 MiB Lambda delivery and <=128 KiB local executable grant. Add fields only after actual serialized size tests; no limit increase or secret-bearing truncation.                                                                                                                                                   |

Fixed proposed validation codes within existing envelopes: template_invalid,
variable_missing, variable_type, variable_limit, environment_revision_conflict,
sensitive_input, sensitive_url, template_target_overlap. Use existing
invalid_request/validation_failed, 413 payload_too_large and 404/409/412/428/503
mappings as applicable; no field/value/name excerpts. A malformed selector pair
is 400, stale selected environment revision is 409 with fixed conflict reason
(the request/workflow If-Match still uses 412). Post-admission unavailable source
or resolved overflow retains existing source_unavailable/input_limit_exceeded
and certainty rules. Successful assertions do not override failed resolution.

Compatibility is **not** additive to current strict readers. The
[implementation-ready compatibility amendment](project-environments-compatibility.md)
defines exact saved/frozen fields, owner evidence **3**, Work/Result **2**,
capability negotiation, trusted reusable-secret scalar types/PATCH and explicit
MCP projections. These allocations are proposed and unaccepted; schema 1/2 stay
unchanged. OpenAPI's earlier proposed evidence 3 is superseded by an unallocated
future-version placeholder, without accepting its production contract.
All strict readers/cleanup and the complete proposed stage tuple require a
separately reviewed closed-stage activation. No production changes in this packet.

## Focused local prototypes and remaining evidence

Local checks passed for this packet: full `npm run verify` (Prettier/gofmt,
lint, web/infra/tooling tests, web build and every Go module's tests/vet), plus
isolated Go build, vet, uncached race tests and compiled test-binary build.
The isolated module is included in root formatting/verification and CI.

```sh
npm run verify
cd spikes/environment-resolution
go build ./...
go vet ./...
go test -race -count=1 ./...
go test -c -o /tmp/kurier-environment-resolution.test
```

[Isolated Go tests](../../../spikes/environment-resolution/resolution_test.go)
import existing strict JSON, decimal, destination, header and redaction helpers
read-only. They are policy experiments, not production resolver/encryption/CAS
code. They demonstrate:

- exact near-equal numeric lexemes, booleans, present null and strings survive
  typed whole-value JSON substitution; strings containing markers are inert;
  mixed nonstring interpolation rejects; quotes/newlines remain valid JSON;
- over-limit scalar/expansion/token counts, unsupported containers/numbers,
  malformed markers, disabled/missing variables and header CR/LF fail safely;
- Go EscapedPath hides delimiter braces: source-component tokenization must
  precede URL normalization. Path escaping, separate query encoding, protected
  base/path denial and userinfo/query/fragment/path-structure rejection are
  exercised without DNS or HTTP;
- existing destination helpers continue to deny a private cloud target and
  permit an exact local-origin fixture; raw/rendered synthetic canaries are
  scrubbed by the shared sanitizer, with sensitivity retained;
- the [compatibility amendment](project-environments-compatibility.md) and
  [wire experiments](../../../spikes/environment-resolution/compatibility_test.go)
  distinguish absent/null context, reject incompatible declarations before spy
  side effects, exercise RFC 8785 identity changes and typed secret PATCH, and
  use the existing MCP capture envelope without changing stored bytes;
- frozen RawMessage buffers need independent copies; existing submit/workflow/
  plan readers reject new environment/marker fields, proving coordinated
  compatibility work is required.

No production secret-storage, full sanitizer, browser numeric round-trip, database
transaction/race, agent negotiation, worker scheduling, network/effect, AWS/KMS/
Cognito/IAM, retention/deletion, deployment or metered-cost result is claimed.
The experiments do not prove all redaction paths or simulated eligibility checks
safe in deployed protocols. Those are implementation/validation obligations.

## Acceptance criteria for a later implementation

1. Davian accepts the explicit decisions below and the reconciled wire/schema
   amendments; the native API document is synchronized/read back within that
   separately authorized implementation. Preparation alone checks no acceptance box.
2. Owner CRUD, metadata/detail projections, revision ETags/CAS, replacement arrays,
   stable variable identities, compatible secret reuse and uncertain-write
   reconciliation pass positive/foreign/stale/deleting/disabled/generation tests.
3. Browser author/reopen/unrelated edit/freeze/rerun preserves tagged decimal
   characters and types; root/null/empty/string escaping/malformed/disabled and
   maximum-work vectors pass identically on cloud and supported local agents.
4. Selector/override and target-conflict tests cover every supported location;
   missing/revoked sources never cause empty fallback, same-name rebinding,
   partial dispatch or another HTTP attempt. Local workflow denial remains.
5. Atomic admission/source-revocation/intent/advancement/receipt races, public edit
   isolation and exact current-secret reruns pass repository/race/integration
   tests. Real AWS/runtime evidence is distinguished from fixtures.
6. Synthetic secret canaries and supported encodings/JSON escaping/reflections
   are absent from Control, public URLs/previews, evidence, logs/errors, queues,
   browser persistence and every MCP result; independent local-result sanitization
   and failed-resolution/partial-error paths are exercised. Authorized transient
   outbound/grant buffers are the only intended plaintext delivery exceptions.
7. Actual serializers prove all combined bounds, transaction actions/bytes and
   worst-case escaping before dispatch; overflows fail without truncation or
   split atomic publication. No max-size Lambda claim without measurement.
8. Environment/variable/secret/project deletion, pin/context cleanup, late uploads,
   unknown schemas and restore-generation suppression retain no-resurrection/no-
   replay behavior. All readers fail closed under incompatible capabilities.
9. Relevant format/lint/build/unit/race/browser checks pass; deployment inventory,
   costs, IAM and genuine runtime campaign require a later scoped review.

## Decisions requiring Davian's review

- Approve primitive typed values/tagged decimals, immutable variable identity and
  the proposed project-owned reusable secret lifecycle/type-capability extension.
- Approve explicit request opt-in, no recursion/escaping language, component-only
  public URL interpolation, typed whole-JSON slots and the credential-template
  exception; arrays/objects and dynamic names remain unsupported.
- Approve selector precedence, no ad-hoc overrides, workflow namespace separation
  and overlapping-target rejection; accept the enabled-versus-revoked distinction.
- Approve frozen public/current-exact-secret reruns, environment/variable deletion
  blocking, receipt identity additions and the source/transaction budget tradeoffs.
- Approve cloud standalone + local standalone + cloud workflow coverage together,
  required agent negotiation and continued rejection of local workflows.
- Approve additional limits, CRUD/selection conflict mapping, staged rollout,
  coordinated evidence/MCP compatibility and later deployment/runtime review.

All recommendations remain **unaccepted**. No production implementation or AWS
work is authorized by publishing this packet or its prototype tests.
