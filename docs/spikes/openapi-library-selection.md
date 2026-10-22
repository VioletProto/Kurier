# OpenAPI library selection — proposed for review

Prepared 2026-10-22 for [the library card](https://trello.com/c/y53kkj4i).
This is a tested recommendation, not architecture acceptance or feature delivery.
The implementation handoff is [OpenAPI import/validation](https://trello.com/c/zUsI4e6l);
[MCP result exposure](https://trello.com/c/jQR3w1Zt) remains separate.

## Recommendation

Use **kin-openapi v0.152.0** for document loading, semantic validation and
local reference resolution; **santhosh-tekuri/jsonschema/v6 v6.0.3** for response
schema validation; and **oasdiff/yaml3 v0.0.14** for syntax-tree preflight.
Translate only the explicit allowlisted 3.0 schema subset below to standard
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
proposed. This packet refines those proposals without editing the live documents.

Read [accepted architecture](../architecture/README.md), both accepted
[ADRs](../decisions/), the W1 design/data model/review/synchronization records and
the relevant execution, protected-input, assertions and MCP boundaries. Preserve
2 MiB source, 100 operations, 32 KiB operation bundles, 64 KiB complete saved/frozen
configuration, 2 MiB wire and decoded response, 64 KiB response headers, 4 MiB
encoded evidence/API, transaction/chunk budgets, retention and cleanup fences.

Trello target, architecture, contract-review, implementation and MCP handoffs and
their returned checklists were read. The target had no checklist. Doing was empty;
the configured WIP limit is three. Only the library spike was moved to Doing.
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

## Exact first supported subset

Propose deliberately small **OpenAPI 3.0.0–3.0.4**, single-document JSON or YAML.
No Swagger 2, OpenAPI 3.1/3.2, external files, ZIP bundles or standalone JSON Schema
imports. The executable prototype grammar in `selection_test.go` is a review aid;
the following semantic restrictions also apply to a future implementation.

| Area        | Supported                                                                                                                                                                                                                                                                                                                                      |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Document    | `openapi`, `info` (title/version/description), `servers` (literal URL/description), `paths`, `components.schemas`, `components.responses`.                                                                                                                                                                                                     |
| Operations  | GET/POST/PUT/PATCH/DELETE on literal absolute paths. operationId, summary, description, responses. At least one operation and response; unique internal operation identity independent of authored operationId. No path variables or parameter/request-body generation in this initial subset.                                                 |
| References  | Same-document `#/components/schemas/...` and `#/components/responses/...`, matching target kind; escaped RFC 6901 names. Acyclic only, no sibling keywords. Missing/wrong-kind targets fail. No external URI, file, relative-file, anchor, dynamic ref or recursive schema.                                                                    |
| Responses   | Exact three-digit 100–599 and optional `default`, chosen exact first then default. No status ranges. Declared content-free responses check status/headers only. JSON responses require one `application/json` entry with a schema.                                                                                                             |
| Headers     | Inline Header Object: description, boolean required and schema of type string only. Case-insensitive lookup; missing differs from present empty. Reject duplicate names under case folding. Content-Type is checked as media metadata, not a custom required header. No header content/style/explode or header refs.                           |
| Schema      | Explicit single type object/array/string/integer/number/boolean; nullable boolean, object properties/required, array items, boolean additionalProperties; title/description annotations. Missing additionalProperties follows OAS default allowing extras. No coercion; integral JSON numeric values satisfy integer. Null only with nullable. |
| Annotations | Listed descriptive fields accepted but stripped from retained validation bundles. No automatic request values, example/default injection, auth setup or target calls.                                                                                                                                                                          |

Reject every other field, including allOf/anyOf/oneOf/not, discriminator, enum,
numeric/string/array bounds, pattern/format, readOnly/writeOnly, schema-valued
additionalProperties, boolean schemas, `$schema`/`$id`/`$defs`, security schemes,
security requirements, callbacks, links, webhooks, externalDocs, tags, examples,
defaults, extensions, parameters, requestBody and server variables. Reject the
whole import rather than publishing a silently weakened contract. This narrower
slice supports importing static endpoints and editing their saved requests using
existing authoring; ordinary parameterized/auth-described APIs will be rejected.
That compatibility cost is explicit and is a review decision, not an assertion
that the libraries lack these capabilities. Expand only with additional fixtures
and an accepted contract amendment.

Use an AST context allowlist: a property literally named `oneOf` remains a property,
not a composition keyword. Inspect unused components too. YAML permits only
string-key mappings, ordinary JSON-compatible scalar tags and sequences; reject
duplicates, anchors/aliases, merge keys, custom tags and multiple documents.
JSON ingress must additionally use the existing strict JSON rules, not accept
the YAML superset under a JSON content type.

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
- Native kin rounds a near-integer token; the selected engine rejects it exactly.
  Bounded translation preserves nullable, required, array and object semantics;
  integral exponent spelling passes. Unbounded numeric tokens are not evaluated.

The prototype is intentionally **not** a finished importer or safe production
adapter. Its YAML-to-JSON conversion is for synthetic fixtures, and its header
experiment checks presence only. Complete typed grammar enforcement, UTF-8/JSON
ingress, header schema/case-collision rules, operation-bundle serialization,
source sanitization, sensitive-response suppression, all diagnostic mappings and
publication/cleanup are implementation requirements, not claimed finished here.
No adversarial CPU/RSS benchmark, production Lambda measurement or runtime proof.

## Schema translation contract

After feature preflight, semantic validation and cycle/work checks, map one OAS
`type` to a 2020-12 type array; add `null` exactly when nullable is true. Recursively
map properties and items, copy required names and boolean additionalProperties,
and drop title/description. Resolve only allowed local refs first. No other
keywords enter the compiler. Require explicit type at each schema and items for
arrays; reject incompatible type-specific keywords rather than silently dropping
them. Preserve absent additionalProperties as the standard allowing default.
Compile with fixed Draft2020, in-memory registered resources and no URLLoader.
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
library error text. Codes include syntax_invalid, dialect_unsupported,
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
errors. Generated requests use opaque internal operation IDs, empty values/body,
validated public literal server URL/path and existing saved-request bounds. Do
not follow specification URLs or synthesize credentials. Multiple eligible server
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

Propose **cloud standalone and cloud workflow** validation first. Local agent
results are already sanitized before upload; the server cannot reliably validate
raw values from them. Local operation associations must be rejected until a
separately accepted local protocol supplies trustworthy versioned results.
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
conservative whole-body suppression needs implementation fixtures and explicit
owner UI wording. It preserves the accepted no-secret-evidence boundary while
making reduced coverage visible.

Propose the next immutable evidence schema (**3**, preserving all schema-2
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

## Verification and review boundary

Local verification passed: root `npm run verify` (format, lint, web/infra/tooling
tests, build and all Go test/vet checks), plus the isolated prototype's Go build,
race tests and compiled test-binary build. The initial sandbox run failed on an
existing release test's Git subprocess (`spawnSync git EPERM`); the unrestricted
local rerun passed. No test failure was bypassed or production test changed.

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
were not implemented or exercised. Review the subset's compatibility cost,
conservative sensitive-body suppression, cloud-only first target and evidence-3
proposal before starting the implementation card.
