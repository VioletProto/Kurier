# Accepted OpenAPI libraries — production contract still proposed

Prepared and revised 2026-10-22 for [the library card](https://trello.com/c/y53kkj4i).
Library selection is accepted as recorded below; production contract acceptance
and feature delivery remain separate.
The implementation handoff is [OpenAPI import/validation](https://trello.com/c/zUsI4e6l);
[MCP result exposure](https://trello.com/c/jQR3w1Zt) remains separate.

## Scoped library acceptance

On 2026-10-22 Davian accepted **kin-openapi v0.152.0**,
**santhosh-tekuri/jsonschema/v6 v6.0.3** and **oasdiff/yaml3 v0.0.14**, supported
by the revised research at `a759851a252db854c7f74ad98ec8114ba2a7b668`, and authorized
recording this acceptance and merging PR #14 after head/check verification.
This acceptance selects the three exact library pins only. It does **not** accept
the proposed production import/validation contract, dialect/feature and
serialization boundaries, candidate lifecycle/DTOs, example/secret-designation
policy, translator semantics, cloud-only validation, sensitive-body suppression,
evidence-schema/result/capability changes or operational decisions below.
Those remain explicitly unaccepted and need separate review before implementation.
No production implementation, deployment or AWS changes are authorized.

This acceptance record changes the PR head. The new head must pass the relevant
checks and fresh GitHub CI before merge; merge must use an expected-head guard.
The PR and Trello closeout will record the exact merged review head and CI run.

## Scope correction and review status

The current proposal's approved MVP requires generating requests from path, query
and header parameters, optional JSON request bodies, and documented examples when
available. The original static-endpoint experiment did not satisfy that complete
requirement. Its restrictions remain below as research evidence, **not a reduction
of the approved MVP**. The revised proposed boundary and additional compatibility
probes cover those generation requirements. Neither this revision nor library
selection constitutes acceptance of the full production import/validation contract.

## Recommendation

Use **kin-openapi v0.152.0** for document loading, semantic validation and
local reference resolution; **santhosh-tekuri/jsonschema/v6 v6.0.3** for response
schema validation; and **oasdiff/yaml3 v0.0.14** for syntax-tree preflight.
The remaining adapter/production recommendation is **proposed, not accepted**:
translate only the explicit allowlisted 3.0 schema subset below to standard
JSON Schema 2020-12, with bounded traversal and serialization. Compile at import,
then use the frozen compiled semantics with raw `json.Number` values. Apply
independent status/header checks. Do not forward arbitrary OAS Schema Objects
straight into a standard JSON Schema compiler or claim general 3.x support.

Reject kin's native 3.0 response-schema validator for this slice: it converts
`json.Number` to float64 and accepts `1.00000000000000001` as an integer after
rounding. The selected engine rejects that fractional integer without coercion.
Also reject `openapi3filter.ValidateResponse` as the complete contract verdict:
it skips several statuses even with undocumented-status checking enabled.

All dependencies are isolated in [the prototype module](../../spikes/openapi-library-selection/go.mod).
No application Go module, deployed capability or production handler changes.

## Sources and dependency state

Read the current native [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
and [API Contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit),
including later accepted continuations. They require bounded JSON/YAML imports,
local references, ready-only publication, frozen operation associations and safe
immutable validation evidence. Dialect/library and import DTO choices remain
proposed at source-reading time. The scoped library acceptance above supersedes
only the library-selection status; this packet does not accept or rewrite the
live production contract documents.

Read [accepted architecture](../architecture/README.md), both accepted
[ADRs](../decisions/), the W1 design/data model/review/synchronization records and
the relevant execution, protected-input, assertions and MCP boundaries. Preserve
2 MiB source, 100 operations, 32 KiB operation bundles, 64 KiB complete saved/frozen
configuration, 2 MiB wire and decoded response, 64 KiB response headers, 4 MiB
encoded evidence/API, transaction/chunk budgets, retention and cleanup fences.

The original spike read Trello target/dependency cards and checklists; Doing was
empty under the configured limit of three. The target initially had no checklist;
verified research criteria were later added and completed. This revision reread
the current proposal/contracts and board; library research completion is separate
from acceptance of the revised production contract.
The implementation checklist item combining spike completion **and contract
acceptance** must remain open until acceptance; no downstream feature is complete.

## Compatibility, maintenance and alternatives

Versions below are exact Go module pins, with tag commit timestamps observed via
`go list -m -json` on 2026-10-22. A tag timestamp is not a GitHub release-publication
date. Fresh tagged releases and upstream fixes are maintenance evidence, not a
promise of support or a security audit.

| Candidate                            | Compatibility and maintenance evidence                                                                                                                                                                                                                                          | License and decision                                                                                                                                                                                                |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| kin-openapi v0.152.0                 | Go >=1.25; tag 2026-10-22. Current upstream supports newer OpenAPI versions as well as 3.0. Prototype resolves JSON/YAML local schema/response refs, rejects malformed input and validates schema semantics. Structured schema errors expose keyword and instance pointer.      | MIT. Select loading/semantic validation, reject native response-schema validation because of numeric rounding. Broad upstream support does not expand Kurier's subset.                                              |
| oasdiff/yaml3 v0.0.14                | Same syntax-tree family used by kin's YAML dependency; inspect duplicate keys, aliases, tags and feature positions before conversion.                                                                                                                                           | Ported files MIT, remaining code Apache-2.0: preserve both notices. Select for preflight; it is not a security policy by itself.                                                                                    |
| libopenapi v0.41.6                   | Go >=1.26; tag 2026-10-10; active release stream. Both fixture encodings and nested local refs build correctly. Its high-level model also builds a fixture with an unknown info field that kin semantic validation rejects. Model construction is not full semantic validation. | MIT. Reject for this narrow slice: a separate semantic/HTTP validation path and larger model/index surface add work without a demonstrated requirement. This is not a claim that its parser is defective or slower. |
| santhosh-tekuri/jsonschema/v6 v6.0.3 | Go >=1.21; tag 2026-06-28, later GitHub release. Standard drafts 4/6/7/2019-09/2020-12; local refs, required and primitive-type checks pass. Default external loading is unconfigured in this pin.                                                                              | Apache-2.0. Select with explicit subset translation. Direct OAS 3.0 use rejected: nullable is ignored. Translated type unions and exact fractional-integer checks work.                                             |
| pb33f/libopenapi-validator           | Upstream provides HTTP/schema validation beyond the parser.                                                                                                                                                                                                                     | MIT according to upstream. Documentation reviewed only; not downloaded or response-benchmarked here. Reject adding a second HTTP-validation stack before the subset needs it.                                       |
| go-openapi/spec + validate           | Swagger/OpenAPI 2 oriented, unlike this 3.0 import boundary.                                                                                                                                                                                                                    | Apache-2.0. Documentation screen only; no prototype run.                                                                                                                                                            |
| xeipuuv/gojsonschema                 | Documents draft 4/6/7 support, not OAS Schema semantics or 2020-12.                                                                                                                                                                                                             | Apache-2.0 per repository license. Documentation screen only; no claim that it is abandoned. No benefit over the tested standalone candidate.                                                                       |

Primary sources: [kin repository](https://github.com/getkin/kin-openapi),
[kin releases](https://github.com/getkin/kin-openapi/releases),
[libopenapi releases](https://github.com/pb33f/libopenapi/releases),
[JSON Schema repository](https://github.com/santhosh-tekuri/jsonschema),
[JSON Schema releases](https://github.com/santhosh-tekuri/jsonschema/releases),
[libopenapi-validator](https://github.com/pb33f/libopenapi-validator),
[go-openapi/spec](https://github.com/go-openapi/spec),
[gojsonschema](https://github.com/xeipuuv/gojsonschema).
The three downloaded candidates' LICENSE files and go.mod requirements were also
inspected locally. MIT requires notice retention; Apache-2.0 adds license,
attribution/NOTICE and modification-notice requirements where applicable. These
are compatible permissive choices; retain dependency notices in distributed
artifacts and review the complete transitive inventory before production adoption.
No claim of zero vulnerabilities or completed legal/security audit.

## Restricted prototype — research evidence, not the import MVP

The original `selection_test.go` preflight exercises a small OpenAPI
3.0.0–3.0.4 grammar. Its table and rejections describe that experiment only.
It remains useful for bounded parsing, schema translation and response-validation
regressions. It is **not** the production allowlist and must not gate the complete
MVP. The separate `request_generation_test.go` probes the added generation surface;
neither test helper is a production importer.

| Area        | Supported                                                                                                                                                                                                                                                                                                                                      |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Document    | `openapi`, `info` (title/version/description), `servers` (literal URL/description), `paths`, `components.schemas`, `components.responses`.                                                                                                                                                                                                     |
| Operations  | GET/POST/PUT/PATCH/DELETE on literal absolute paths. operationId, summary, description, responses. At least one operation and response; unique internal operation identity independent of authored operationId. No path variables or parameter/request-body generation in this initial subset.                                                 |
| References  | Same-document `#/components/schemas/...` and `#/components/responses/...`, matching target kind; escaped RFC 6901 names. Acyclic only, no sibling keywords. Missing/wrong-kind targets fail. No external URI, file, relative-file, anchor, dynamic ref or recursive schema.                                                                    |
| Responses   | Exact three-digit 100–599 and optional `default`, chosen exact first then default. No status ranges. Declared content-free responses check status/headers only. JSON responses require one `application/json` entry with a schema.                                                                                                             |
| Headers     | Inline Header Object: description, boolean required and schema of type string only. Case-insensitive lookup; missing differs from present empty. Reject duplicate names under case folding. Content-Type is checked as media metadata, not a custom required header. No header content/style/explode or header refs.                           |
| Schema      | Explicit single type object/array/string/integer/number/boolean; nullable boolean, object properties/required, array items, boolean additionalProperties; title/description annotations. Missing additionalProperties follows OAS default allowing extras. No coercion; integral JSON numeric values satisfy integer. Null only with nullable. |
| Annotations | Listed descriptive fields accepted but stripped from retained validation bundles. No automatic request values, example/default injection, auth setup or target calls.                                                                                                                                                                          |

That original gate rejects all other fields, including parameters, requestBody,
examples, security declarations and advanced schema keywords. Its static-only
result was insufficient for MVP request generation; the revised proposal below
supersedes those generation exclusions. The tests intentionally retain the old
gate unchanged so its original research findings remain reproducible.

Use an AST context allowlist: a property literally named `oneOf` remains a property,
not a composition keyword. Inspect unused components too. YAML permits only
string-key mappings, ordinary JSON-compatible scalar tags and sequences; reject
duplicates, anchors/aliases, merge keys, custom tags and multiple documents.
JSON ingress must additionally use the existing strict JSON rules, not accept
the YAML superset under a JSON content type.

## Proposed MVP request-generation boundary

Keep OpenAPI **3.0.0–3.0.4**, single strict JSON/YAML document, <=2 MiB and
<=100 operations. Keep the response-schema type/required/nullable/items/boolean
additionalProperties subset and existing execution bounds. Extend the proposed
operation model to include the following generation features; this is a proposed
bounded implementation of approved product requirements, not a new approval.

| Feature                | Proposed supported behavior                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Paths and methods      | GET/POST/PUT/PATCH/DELETE; absolute path templates with named, nonnested `{parameter}` substitutions. Every template name has exactly one effective required path parameter; reject extra/unmatched/duplicate/ambiguous placeholders. Never leave braces in a saved executable URL.                                                                                                                                                                                                                                                                     |
| Parameter declarations | Inline or same-document `components.parameters` refs at Path Item and Operation level. Inherit path-level parameters; operation-level `(in,name)` overrides without adding a duplicate. Reject duplicates within a level; compare header names case-insensitively, other names exactly. Preserve deterministic effective order (inherited positions replaced in place, new operation parameters appended).                                                                                                                                              |
| Path parameters        | Nonnullable string/integer/number/boolean only, required:true, simple style, explode:false (including defaults). Substitute one percent-encoded scalar per placeholder; never treat example bytes as URI syntax. No sensitive path values.                                                                                                                                                                                                                                                                                                              |
| Query parameters       | Nonnullable primitive or array of nonnullable primitives; form style, explode:true (including defaults). Scalar creates one ordered row; array creates repeated rows with the same name in source order. Keep query values separate from the URL until the existing execution encoder runs.                                                                                                                                                                                                                                                             |
| Header parameters      | Nonnullable primitive only, simple style, explode:false (including defaults). Render a control-safe scalar; HTTP-token header name, no CR/LF or newline folding. Reject routing/framing headers controlled by the runtime, plus parameter declarations for reserved Accept/Content-Type/Authorization; body media and existing protected auth authoring govern those instead. Credential-like custom headers use existing protected authoring, never a public example value.                                                                            |
| Request bodies         | Inline or local `components.requestBodies` ref; one application/json media entry with the same bounded typed schema subset. Preserve requestBody.required (default false). Optional body is absent unless a safe selected example or explicit owner input supplies it; required body without one needs input. No fabricated `{}` or defaults to conceal missing required fields.                                                                                                                                                                        |
| Examples               | Parameter.example, Parameter.examples, media.example, media.examples and Schema.example, including inline Example Objects and local `components.examples` refs. Direct and named examples are mutually exclusive at one location; outer parameter/media examples take precedence over schema examples. Keep source presence separate from value, including explicit null in direct/schema examples. Named Example Objects with value:null are excluded pending an adapter decision; kin conflates nil with absence. See selection/security rules below. |
| Local refs             | In addition to schemas/responses, allow same-document parameters/requestBodies/examples of the correct kind; preserve acyclic/depth/work budgets and prohibit ref siblings. No external URI/file refs or externalValue fetches.                                                                                                                                                                                                                                                                                                                         |

Reject cookie parameters; object/nested-array parameter values; label, matrix,
deepObject, spaceDelimited and pipeDelimited styles; explode:false query arrays;
explode:true path/header parameters; parameter content; allowReserved:true;
allowEmptyValue:true; nullable parameters; multipart, form, binary, text and other
request-body media; encoding objects; server variables; external examples; and
all unlisted features. Explicit default style/explode values equal omission.
Request schemas still reject enum/composition/pattern/format/bounds/readOnly/
writeOnly/default and standard-JSON-Schema-only keywords. Security scheme import,
auth-flow generation, callbacks, links, webhooks and extensions remain unsupported
with diagnostics, not silently ignored. Library support is broader than this
proposed product boundary.

Proposed generation bounds, applied together: <=32 effective parameters/operation,
<=128 UTF-8 bytes/name, 1–16 elements/query-array example, <=2 KiB serialized
scalar example, <=8 named examples/location, <=8 KiB complete example value and
<=16 KiB aggregate retained examples/operation. Count every candidate, including
unselected or unsafe examples, against source/traversal work limits before
omitting it. The **entire** operation bundle including schemas, generation
metadata and sanitized examples remains <=32 KiB; complete generated/frozen
configuration and resolved request body each remain <=64 KiB, and existing
header/query descriptor limits still apply. Overflow rejects with a fixed limit
code; never truncate values, drop required fields or publish a weakened operation.
These additional per-feature numbers are proposed, not existing accepted quotas.

### Example choice, missing values and saved-request compatibility

Retain safe bounded examples for owner selection, using opaque candidate IDs;
never publish raw example names in diagnostics. A direct outer example is selected
first; for named examples propose the lexically first authored key as deterministic
initial selection, with explicit owner override before publication. Fall back to a
schema-level example only when no outer example exists. Do not merge examples
from unrelated named cases or synthesize values from descriptions/defaults.
All retained values must match the declared schema; invalid examples reject with
example_invalid instead of silently changing them. If a selected example is
unsafe, omit it with a warning and require owner selection/input where needed;
never silently choose a different target or example. Optional unprovided
parameters stay disabled; explicit enablement of empty string differs from no
example. Required query/header/path/body inputs without usable examples need
owner completion. Supported array examples are typed arrays of logical primitive
values; already serialized delimiter strings for numeric/array parameters are
unsupported. String values are literal data, not templates or executable text.

OAS 3.0 example precedence and mutually exclusive forms are defined in the
[Example Object specification](https://spec.openapis.org/oas/v3.0.4.html#example-object).
Library model construction and numeric example validation do not replace strict
source parsing. Kin's generic Example values round numeric literals through
float64. Preserve exact raw JSON lexemes (or validated YAML scalar lexemes) from
the source AST, parse with strict JSON/UseNumber and the bounded decimal domain,
then validate using the selected translated schema engine. YAML numeric examples
must use the supported JSON decimal grammar; reject hex/octal, digit separators,
NaN/infinity and other non-JSON scalar spellings without coercion. Never regenerate
example JSON from kin's generic Example maps. The new precision regression
preserves 0.10000000000000001 through fixture conversion and records kin's loss.

Existing saved requests require concrete absolute public URLs and permit no
unresolved path templates. Therefore propose owner-only **staged generation
candidates** while the import is preparing, separate from executable saved
requests. Each records safe parameter/body definitions, examples, chosen server
and unresolved-input markers. Missing path values are never replaced with empty
segments or fake executable placeholders. After owner completion/confirmation,
materialize concrete requests through existing saved-request/secret validation,
then publish the import ready with its final count/digest guards. No automatic
execution; no generated request visibility before ready. Optional bodies without
examples may remain absent and be authored later. A ready imported request can
use existing local/cloud execution only as permitted by the separately reviewed
validation-association policy; do not infer a new execution restriction from the
library choice. Exact candidate preview/complete/cancel HTTP DTOs and lifecycle
are still a production-contract decision, explicitly listed below.

### Imported examples and the accepted secret boundary

Treat the complete upload, every example (including unselected ones), parameter
values and descriptions as untrusted input. Before any durable source/bundle,
generated request, diagnostic, preview or MCP output, apply the existing
[protected-input contract](../architecture/protected-request-secrets-contract.md):
Authorization/cookies/credential headers and query names, recognized credential
JSON fields, obvious credential text, user-designated paths/values and their
reflections cannot become public examples. Designations must be supplied through
a reviewed import input policy; no free-text credential values in diagnostics.

Propose conservative omission for examples: discard the complete affected
parameter/body example and issue example_omitted_sensitive, with no offending
name/value/path/excerpt/hash. Safe unrelated examples may remain; unsafe selected
required inputs become needs-input markers. A JSON body containing a secret
field is withheld entirely at import, rather than storing its raw text or fake
`[REDACTED]` as a value. Subsequent owner authoring can use the existing whole-body
or per-pointer protected union with literal null placeholders and fresh bindings.
Do not create a secretRef/binding from a sample, automatically encrypt/store an
imported credential, reuse a same-name secret, or downgrade to plaintext when
protection is unavailable. Sensitive path/server/structural values reject as
source_unsafe. Public generated URLs contain only validated concrete nonsensitive
path values; credential queries stay separate protected rows. Validate percent
encoding and control characters before saving, not just before dispatch.

Strip descriptions/unused annotations and omitted examples from retained source;
persist only sanitized structural source/bundles after semantic and budget
checks. Preserve every execution-relevant constraint; reject if sanitizing a
structural field would change it. Apply existing public URL/SSRF rules to the
selected server, without contacting it. Arbitrary undisclosed secrets cannot be
detected reliably; require the existing public-data/explicit designation boundary.
New tests probe credential-name/designated-value omission and fixed diagnostics
using synthetic inputs; they do not implement a production sanitizer or encryption. Per-pointer designation
matching, encoding/reflection variants, full URL/header validation and all durable
secret-write paths remain future implementation tests.

## Prototype findings and preserved selection criteria

[Tests](../../spikes/openapi-library-selection/selection_test.go) contain synthetic
fixtures only. No target API is contacted, and no sensitive source is retained.

- Both parsers handle representative JSON/YAML and nested response/schema refs;
  malformed JSON/YAML is rejected. Kin explicitly runs `doc.Validate` after load.
- AST preflight rejects unknown features, unsupported dialects, external refs,
  duplicate/multiple documents, aliases, missing refs, cycles, excessive depth,
  excessive reference work and operation 101. Operation 100 passes. The byte
  overrun is rejected before parsing. A property named oneOf passes.
- Response cases cover undocumented status, required JSON properties, integer,
  number, string and boolean mismatches, fractional integer rejection, nullable,
  array items, additional properties and a missing required response header.
  Documented no-body response and default response behavior are tested.
- `IncludeResponseStatus` is false by default. Even true, the filter returns
  success early for 304/307 (source also skips 301/308 and HEAD). Explicit status
  selection avoids these holes; future header checks must apply to such responses
  too. Current production HTTP failure policy remains independent of schema match.
- Kin's `SchemaError` carries useful SchemaField/JSONPointer diagnostics, but
  `Error()` includes the synthetic offending value. Never serialize the error,
  Value, schema, input body, authored names or an unrestricted error chain.
- Shared production `StrictJSON` is reused read-only for body experiments:
  duplicate keys, invalid surrogate escapes, trailing JSON and over-limit bodies
  cannot pass. Standalone JSON Schema tests demonstrate the nullable dialect trap.
- Additional JSON/YAML generation fixtures resolve inherited/local parameters,
  operation overrides, local requestBody/example refs, serialization defaults,
  named/schema examples and optional JSON bodies in both parser candidates.
  Direct null-example presence is preserved from source even though the model
  cannot distinguish nil from absence; the translated nullable schema accepts it.
  Request-filter wire probes cover encoded path, scalar/repeated query, required
  header and valid/invalid optional body; this is compatibility evidence only.
- Native kin rounds a near-integer token; the selected engine rejects it exactly.
  Bounded translation preserves nullable, required, array and object semantics;
  integral exponent spelling passes. Unbounded numeric tokens are not evaluated.

The prototype is intentionally **not** a finished importer or safe production
adapter. Its YAML-to-JSON conversion is for synthetic fixtures, and its header
experiment checks presence only. Complete typed grammar enforcement, UTF-8/JSON
ingress, header schema/case-collision rules, complete MVP-generation preflight,
operation-bundle serialization,
source sanitization, sensitive-response suppression, all diagnostic mappings and
publication/cleanup are implementation requirements, not claimed finished here.
No adversarial CPU/RSS benchmark, production Lambda measurement or runtime proof.

## Schema translation contract

After feature preflight, semantic validation and cycle/work checks, map one OAS
`type` to a 2020-12 type array; add `null` exactly when nullable is true. Recursively
map properties and items, copy required names and boolean additionalProperties,
and drop title/description/example from validation schemas; retain only safe
bounded generation examples separately. Resolve only allowed local refs first. No other
keywords enter the compiler. Require explicit type at each schema and items for
arrays; reject incompatible type-specific keywords rather than silently dropping
them. Preserve absent additionalProperties as the standard allowing default.
Compile with fixed Draft2020, in-memory registered resources and no URLLoader.
Validate examples from exact source separately from the model's float64-based
example checks; the production adapter must not rely on native example verdicts.
The prototype bounds translated schema bytes to 32 KiB; production must fit the
**entire operation bundle**, not just this schema, within that same accepted cap.
Repeated ref traversal and translated allocation must remain bounded before
serialization, not merely check the final encoded length.

This is a small semantics adapter for the listed subset, not a general OpenAPI
to JSON Schema converter. Rejecting all other keywords is essential to its
correctness. Freeze both source-subset and translated-schema semantics version 1;
never rebuild historical bundles using a changed translator. Library upgrades
must rerun the complete compatibility suite before changing an execution pin.

## Proposed import and publication contract

Retain the existing project-scoped routes. POST accepts source bytes with
application/json or application/yaml; reject compression and URL-fetch inputs.
Authorize owner/project state before processing. Enforce the 2,097,152-byte body
bound while reading (including transport decoding); no source logging or request
body tracing. Strict syntax, dialect and feature preflight precede admission.

Proposed success: HTTP 202, Location pointing to the project-scoped status route,
and the usual data envelope containing:

```json
{
  "importId": "opaque-id",
  "state": "preparing",
  "detectedVersion": "3.0.3",
  "contractVersion": 1,
  "warnings": []
}
```

GET returns those fields plus `revision`, `unsupportedFeatures`, `diagnostics`
and `generatedRequestIds`. Lists are empty until ready. States are preparing,
ready, failed; project deletion immediately denies access and uses the existing
cleanup protocol. A failed import is never usable. No progress percentage or
partial-success claim. Syntax failures use 400 invalid_request; unsupported or
semantically invalid documents use 400 validation_failed; source overrun 413;
media mismatch 415, preserving existing error envelopes/authorization precedence.
Failures found after admission produce failed plus fixed diagnostic codes.

Bound diagnostics to 16 entries / 2 KiB serialized, with `diagnosticsTruncated`
when more were found. Entries contain fixed code and optional bounded numeric
line/column; never offending source, filenames, URLs, refs, property names or
library error text. Generation failures use the same fixed diagnostic envelope,
including parameter_unsupported, example_invalid, example_limit and input_required;
example_omitted_sensitive is a warning with no source locator. Codes also include
syntax_invalid, dialect_unsupported,
feature_unsupported, reference_unsupported, reference_invalid, reference_cycle,
structure_limit, reference_limit, operation_limit, bundle_limit and source_unsafe.
Owner editors can locate their own source using line/column; status/logs/MCP do
not receive excerpts. Unknown features reject rather than become warnings;
`annotations_removed` is a permissible fixed warning.

Proposed additional bounds: syntax and dereferenced depth <=32, <=20,000 syntax
nodes and <=20,000 reference-traversal visits including repeated edges. Do not
expand refs into unbounded copies. In production, bound parser CPU/memory with an
isolated worker deadline; context cancellation alone cannot stop arbitrary
library recursion. No remote fetching and no filesystem fallback under any
circumstances. Kin's custom ReadFromURIFunc bypasses its external-ref flag, so
the tested callback always denies; libopenapi alternatives must keep both file
and remote-reference flags false. Never set a base directory or HTTP loader.

Build private sanitized structural source and per-operation bundles <=32 KiB;
preserve validation semantics while stripping annotations. Reject secrets in
structural values that cannot be removed without changing semantics. Apply
existing credential/URL/designated-secret/reflection rules; these do not detect
arbitrary undisclosed secrets. Do not persist raw source, raw hashes or parse
errors. Generated requests use opaque internal operation IDs and the revised
parameter/body/example generation contract above, with concrete validated public
URLs and the existing saved-request bounds. Do not follow specification URLs or
synthesize credentials. Multiple eligible server
URLs require explicit selection; never silently choose a different target.

Stage operations/requests under preparing in guarded bounded chunks, preserving
the accepted <=50 mutations plus guards and transaction/item budgets. Publish
ready only after expected counts, revision and sanitized bundle digest agree and
the parent remains active. Interrupted staging resumes idempotently or drains;
no unguarded partial list visibility. Import source and bundles join ordinary
project/late-upload cleanup. No new infrastructure design is selected here.

## Proposed execution validation contract

Freeze `{importId, importRevision, operationId, operationRevision,
contractVersion:1, sanitizedBundleDigest}` with the admitted execution; include
all framing in existing saved/frozen caps. Freeze the bounded sanitized bundle
with the immutable context so later import/request edits cannot change an
accepted run or a deliberate whole-run rerun. Project deletion still wins;
missing legacy context is unavailable, never a passing empty result.

**Unaccepted target proposal:** cloud standalone and cloud workflow validation first. Local agent
results are already sanitized before upload; the server cannot reliably validate
raw values from them. Whether to reject local validation associations or allow imported local requests
with explicit not-evaluated/unsupported validation remains undecided. A future
local protocol would need trustworthy versioned results; library selection alone
does not choose that policy.
Do not revalidate stored evidence on owner GET or MCP reads.

Validate bounded complete raw response in worker memory before sanitization and
disposal. Check exact status/default independently, required non-sensitive header
presence case-insensitively, then supported complete UTF-8 JSON body. Reuse strict
duplicate-key/Unicode/trailing-data/depth guards. Gate every numeric token with
the existing bounded exact-decimal parser before the schema engine: <=128 ASCII
bytes, absolute exponent <=308, zero or magnitude 1e-308 through
9007199254740991 inclusive. Unsupported numbers anywhere make the body
not_evaluated/json_number_unsupported; do not round them or allocate arbitrarily
large rationals. Within this domain, distinguish fractional values from integers
exactly, including `1.00000000000000001` versus `1e0`. Preserve independent wire and
decoded body limits. No comparison against `[REDACTED]` evidence. Missing,
incomplete, malformed, unsupported media/charset, oversized or unsafe buffers
produce explicit failed/not-evaluated checks; never pass by omission. Declared
no-content responses have an explicitly not_applicable body check. Unexpected
content type is a contract failure; unavailable body is not_evaluated.

Avoid match/type oracles on secrets: suppress body validation if its evaluation
could depend on any recognized/designated/reflected sensitive field or value;
report not_evaluated/sensitive_input without a body match bit. Do the equivalent
for sensitive headers, including Authorization/cookies/credential headers. This
conservative whole-body suppression is an **unaccepted policy proposal** requiring
review, implementation fixtures and explicit owner UI wording. It preserves the accepted no-secret-evidence boundary while
making reduced coverage visible.

**Unaccepted versioning proposal:** a future, **unallocated** immutable evidence schema (preserving all schema-2
fields) with required `contractValidation`:

```json
{
  "schemaVersion": 1,
  "state": "evaluated",
  "checks": [
    { "kind": "status", "outcome": "passed", "reason": "matched" },
    { "kind": "headers", "outcome": "failed", "reason": "header_required" },
    { "kind": "body", "outcome": "failed", "reason": "property_required" }
  ]
}
```

State is not_configured (checks empty) or evaluated (exactly three ordered checks).
Outcomes: passed, failed, not_evaluated, not_applicable. Fixed reasons: matched,
status_undocumented, header_required, property_required, type_mismatch,
schema_mismatch, content_type_mismatch, no_body_contract, status_unavailable,
response_unavailable, body_absent, body_unsupported, body_limit_exceeded,
json_invalid, json_number_unsupported, sensitive_input, evaluation_unavailable. Only compatible kind/
outcome/reason triples are legal. Prefer a generic schema_mismatch when a safe
typed classification is unavailable. No arbitrary messages, actual values,
expected values, property/header names, paths, excerpts or response-derived hashes.
Cap the complete result at 2 KiB; aggregate per kind rather than enumerate an
unbounded violation list. not_applicable is allowed only for the body of a
documented response without content. Evaluated means attempted, not successful.

Retain existing dispatch/capture/extraction and HTTP/assertion failure precedence.
A matching documented 404 can pass the contract while the execution remains
upstream_http_error. After existing failures, any configured failed or
not_evaluated contract check makes an otherwise successful execution
contract_validation_failed and stops workflow successors through the existing
atomic terminal publication. No resend to obtain validation data. Capture and
retention/pinning results remain immutable; storage omission is independent from
an evaluation already completed safely in memory.

Evidence 3, capability/version activation, coordinated readers/cleanup, legacy
absence and the existing MCP legacy projection all require implementation review
before rollout. MCP must explicitly omit contract results until its separate
card accepts a tool/projection extension; no fourth-result inference from empty
violations. Do not reencode existing evidence or change accepted schema-1/2 bytes.

## Remaining acceptance decisions — not resolved by library selection

- **Generation handoff:** accept the proposed scalar/query-array styles, example
  precedence/selection and budgets; settle candidate preview/complete/cancel DTOs,
  explicit server selection, missing-required-input lifecycle and secret
  designation/omission UI. Staged candidate storage/cleanup needs schema review.
- **Target coverage:** cloud-only validation is a proposal. Decide standalone and
  workflow/local behavior explicitly; imported local requests and raw-response
  validation availability are separate concerns. No new local restriction is
  accepted by this packet.
- **Sensitive-response policy:** decide whole-body suppression versus a safely
  scoped alternative, allowed presence/type disclosure and not-evaluated wording.
  Request-example omission does not settle response-validation suppression.
- **Evidence compatibility:** an unallocated future evidence schema, result DTO/reason triples,
  capability/translator versioning, coordinated readers/cleanup and activation
  remain proposed. Preserve old immutable bytes and MCP's explicit omission;
  separate MCP contract acceptance is still required.
- **Runtime semantics and operations:** accept failure precedence, admission/freeze
  budgets, async publication/error DTOs, cleanup and worker CPU/memory isolation;
  implement and verify them separately. No deployment, AWS work or production
  feature is authorized by this research revision.

The three library pins are now accepted independently. The library
spike's completion/acceptance is not a checkmark for the combined library-plus-production-
contract acceptance items on the implementation or API-contract cards.

## Verification and review boundary

Local verification passed: root `npm run verify` (format, lint, web/infra/tooling
tests, build and all Go test/vet checks), plus the isolated prototype's Go build,
race tests and compiled test-binary build. The initial sandbox run failed on an
existing release test's Git subprocess (`spawnSync git EPERM`); the unrestricted
local rerun passed. No test failure was bypassed or production test changed. The revised generation
fixtures also passed full root verification and isolated Go build/vet/race and
test-binary compilation. Exact pushed review head and remote CI outcome are
reported on PR #14 after push; a local pass is not a remote CI result.

Run from the repository root:

```sh
npm run verify
cd spikes/openapi-library-selection
go build ./...
go vet ./...
go test -race -count=1 ./...
```

The module is included in root Go formatting/verification and CI, including a
test-binary build because it intentionally contains tests only. All evidence in
this packet is local/synthetic. Import API/UI, persistence, actual sanitization,
runtime error policy, partial publication, IAM, Cognito, email, AWS and deployment
were not implemented or exercised. Review the revised MVP generation boundary,
conservative sensitive-body suppression, cloud-only first target and future evidence-schema
proposal before starting the implementation card.

Environment compatibility coordination: the [environment amendment](../architecture/drafts/project-environments-compatibility.md)
now proposes reserving evidence **3** exclusively for environment/template context.
This supersedes this research packet's earlier numeric suggestion; OpenAPI has no
allocated production evidence version. Neither version 4 nor any import/validation
contract is accepted by the library selection.
