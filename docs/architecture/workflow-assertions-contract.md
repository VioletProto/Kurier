# Workflow assertions review contract

Status: **Proposed**, prepared 2026-10-20 for Davian's review. Publication is
not acceptance, implementation authorization, or deployment authorization.
This packet changes documentation only. Assertion behavior below is required
future behavior, not a description of deployed functionality.

Review baseline: freshly fetched `origin/main`
`f4d1f3b84d3bbf04c451ba927ad9b4853468fed5`; initially clean working tree.
Branch: `docs/workflow-assertions-contract`.
Tracking: [assertions card](https://trello.com/c/8RUxoJyt), with
[API-contract coordination](https://trello.com/c/b5LdvuTo).
The supplied [review draft](drafts/workflow-assertions-review-draft.md) is
preserved as input; this packet resolves its open choices and takes precedence
for this review. Neither document is accepted.

## Authority and scope

Read the current [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
and [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit),
[architecture index](README.md), accepted ADRs
[0001](../decisions/0001-sst-infrastructure-candidate.md) and
[0002](../decisions/0002-serverless-persistence-topology.md), and accepted
[workflows](sequential-workflows-contract.md),
[execution/evidence](cloud-execution-evidence-contract.md),
[protected inputs](protected-request-secrets-contract.md),
[pinning](evidence-pinning-contract.md),
[local agent](local-agent-execution-contract.md), and
[read-only MCP](read-only-mcp-contract.md) contracts. Accepted slice contracts
supersede older generic Proposed passages in the live contracts. No Google Doc
edits are part of preparation; synchronize after explicit acceptance and scoped
follow-up authorization.

The demonstration is HTTP 200 with incorrect response data: the execution and
run fail with immutable, safe check results; the successor endpoint receives
zero sends. Add assertions only to cloud workflow steps, with revisioned
authoring, frozen rules, bounded evaluation and browser inspection. Standalone
cloud requests, local requests and local-agent protocols keep their existing
behavior. Broader target support remains backlog, as do environments, OpenAPI,
scripts, regex, coercion, expressions and assertions over runtime operands.

The target card's overall Done condition includes acceptance, implementation,
UI/contracts and verification. Preparing this packet completes only preparation.
The card and its implementation checklist must remain incomplete.

## Code review and required corrections

All links below refer to code at the review baseline. They identify integration
points, not changes made by this task.

| Current code                                                                                                                                                                    | Finding and contract consequence                                                                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Workflow grammar/capture](../../services/request-runtime/workflow.go)                                                                                                          | `WorkflowStep` has no assertions. Reuse `ParseExtractionPath`; do not reuse extraction's requirement that the selected value be a typed primitive for existence checks. Capture currently runs only when extraction is configured; assertion-only steps need a capture hook too.                                                                                                                                                        |
| [Strict JSON](../../services/request-runtime/json.go)                                                                                                                           | Rejects duplicate keys, invalid UTF-8/unpaired surrogate escapes, trailing documents, unknown typed fields and depth above 32. It uses `json.Number`, but does not itself validate every number's range. `NonNullObject` rejects null recursively: workflow HTTP parsing must replace that blanket check with field-specific null validation to permit `jsonEquals.expected:null`. Null elsewhere remains invalid under existing rules. |
| [Extraction scalar checks](../../services/request-runtime/workflow.go)                                                                                                          | `scalar` converts to float64 for finite/safe-integer checks. It is not exact decimal comparison and can round fractional inputs. Assertion numbers need the bounded exact algorithm below; changing extraction's accepted domain is outside this slice.                                                                                                                                                                                 |
| [Runtime](../../services/request-runtime/runtime.go)                                                                                                                            | HTTP success is final status 200–399; all other final statuses fail. Extraction, sanitization and runtime-storage errors precede the final HTTP classification. Preserve that ordering; assertions add the last failure tier. Capture occurs before raw-buffer disposal and sanitization.                                                                                                                                               |
| [Admission](../../services/api/execution/workflows.go)                                                                                                                          | Definitions are JSON strings in workflow/run rows; plans are separate step rows. Combined Plan+Step is capped at 64 KiB, total plans/input envelope at 256 KiB. Freeze rules in run configuration; do not consult live definitions during evaluation or rerun.                                                                                                                                                                          |
| [Finalization](../../services/api/execution/workflow_execution.go), [publication/recovery](../../services/api/execution/worker.go)                                              | Only `completed` evidence advances. Failure already atomically skips successors and deletes runtime inputs/outputs. Recovery strictly decodes schema-1 evidence; add a version-dispatched decoder before schema-2 publication.                                                                                                                                                                                                          |
| [Encoding](../../services/api/execution/model.go), [shared DTO](../../services/request-runtime/model.go)                                                                        | Shared Evidence is schema 1. Encoding may omit bodies to meet delivery limits; retain results through that omission. Distinguish evidence schema from DynamoDB row schema.                                                                                                                                                                                                                                                              |
| [Store gates](../../services/api/execution/store.go), [ownership gates](../../services/api/internal/ownership/store.go), [cleanup](../../services/api/execution/maintenance.go) | Stage currently recognizes savedRequests7/workflows1/MCP1. Row allowlists and round-trip checks must recognize any new fields before publication. Unknown records must continue to block destructive cleanup.                                                                                                                                                                                                                           |
| [Browser DTOs](../../apps/web/src/api.ts), [editor](../../apps/web/src/Workflows.tsx), [evidence UI](../../apps/web/src/Executions.tsx)                                         | Add typed authoring and version-aware result rendering. Existing `schemaVersion:number` typing does not establish compatibility or validate result completeness.                                                                                                                                                                                                                                                                        |
| [Hosted MCP](../../services/api/execution/mcp_read.go), [wire decoder](../../services/request-runtime/mcpwire/wire.go), [tool schemas](../../services/mcp/schema.go)            | Hosted read rejects capture schema other than 1; wire decoder rejects truncated or incomplete complete-view captures. Shared struct expansion would also silently expand generated MCP schemas. Use a separate legacy projection DTO, not the new shared evidence struct.                                                                                                                                                               |

## Authoring, revision and admission DTO

Extend each workflow step with optional `assertions`; omission normalizes to
`[]`, explicit null is invalid. Existing workflow definitions/retained runs
without the field are valid legacy records. PATCH omission of `steps` preserves
rules; replacing `steps` replaces each assertion array, with omission clearing
that step's rules. No independent assertion routes or revision counter.
Keep existing quoted revision ETag/If-Match behavior, including 428 missing,
400 malformed and 412 stale. Reordering steps/checks preserves their identities.

Each rule is one of these exact objects:

```json
[
  {
    "assertionId": "76d4dfb3-1b1a-47e6-84ee-8d5b4c77e235",
    "kind": "statusEquals",
    "expected": 200
  },
  {
    "assertionId": "a792a719-e5ba-41b1-97d8-b36948f0a641",
    "kind": "jsonExists",
    "path": "$.user.id"
  },
  {
    "assertionId": "41aeec9e-e093-4533-b0d9-f8dc4dde7584",
    "kind": "jsonEquals",
    "path": "$.user.active",
    "expected": true
  }
]
```

`assertionId` uses the existing lowercase UUID-v4 grammar, unique within its
step, stable on reorder; it is an identifier, never authority. All shown fields
are required; reject unknown, duplicate, mis-cased fields and invalid kind
combinations. `statusEquals` has no path; expected is a JSON integer token
100–599 (no decimal or exponent notation). `jsonExists` has no expected field.
`jsonEquals.expected` is a required JSON primitive including explicit null;
omission is distinct from null. Reject arrays/objects. No names, free-text
diagnostics, secret refs, variable operands or expression fields.

Bounds apply together: eight assertions/step, at most one status assertion,
2 KiB serialized expected primitive, 512 UTF-8 bytes/32 selectors per path,
and **8 KiB complete compact serialized assertion array per step**. All fields,
escaping and commas count. The extra array cap resolves the draft's independent
maxima problem; not every combination of scalar maxima is admissible. The
existing 64 KiB complete workflow and combined Plan+Step caps remain, as do
256 KiB aggregate frozen plans/initial envelope, 128 KiB serialized attribute
item, 60 workflow transaction actions and global 80 actions/2 MiB bounds.
Reject 413 before acceptance/dispatch if any bound fails; never silently drop a
rule. Invalid shape/type/path/security policy returns a fixed 400 code without
echoing inputs. Preserve existing ownership/not-found/conflict errors.

Freeze a rule-semantics version of 1 with every newly admitted assertion-capable
run, including runs with empty arrays. Preserve this version through whole-run
reruns, alongside frozen rules/plans and fresh identities. Definition changes
cannot affect accepted runs. Legacy reruns retain legacy absence and behavior;
do not invent evaluations for them. Existing source-bound seven-day admission
receipts and consent/current eligible stable secret input rules remain.

## Evaluation semantics and bounded numbers

Evaluate the configured set in frozen order; all rules must pass to satisfy
assertions. Do not stop after the first mismatch. Evaluate only trustworthy
inputs; status can remain usable when body checks cannot run. No configured
rules means `not_configured`, never pass. No historical result means
`unavailable`, never pass.

| Input or rule    | Required behavior                                                                                                                                                                                                                                                                                                                 |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Final status     | Compare exact observed final status, never an interim response. Missing status is `not_evaluated/status_unavailable`. A matching 404 assertion does not make HTTP 404 successful.                                                                                                                                                 |
| Path syntax      | Root `$`; dot ASCII identifiers; bracket JSON-string properties (including empty, dotted, escaped and Unicode names); zero-based canonical nonnegative integer indexes. No wildcards, filters, recursion, scripts, negative/leading-zero indexes or implicit container conversions. Use existing platform integer/index bounds.   |
| `jsonExists`     | Located value exists even if null, false, zero, empty string, object or array. `$` exists for every valid supported JSON document, including root null/scalar. Missing property, wrong intermediate container or out-of-range index fails `path_missing`.                                                                         |
| `jsonEquals`     | Present primitive with the same JSON type and value passes. Missing fails `path_missing`; present container or different primitive type fails `type_mismatch`; different same-type value fails `value_mismatch`. Null matches only present null. Strings compare decoded code points exactly, without normalization/case folding. |
| JSON eligibility | Require application/json or application/*+json with absent/UTF-8 charset, complete bounded decoded body, strict JSON/Unicode/depth checks. Empty body, unsupported media/charset/encoding, malformed JSON and unsafe numbers cannot pass JSON rules. Do not parse HTML/text merely because it looks like JSON.                    |
| Later omission   | Policy body omission or encoded-evidence size omission does not undo an evaluation completed on valid bounded raw data. Genuine capture/sanitization/extraction failures still fail under precedence below. Record omission on the body independently.                                                                            |

Number semantics are an explicit assertion-only refinement of the draft, rather
than claiming exactness from extraction's float64 predicate:

- JSON number token <=128 ASCII bytes; exponent absolute value <=308; valid JSON
  syntax only. Expected numbers also fit the 2 KiB primitive cap.
- Normalize sign, decimal coefficient and exponent using bounded integer/decimal
  arithmetic. Compare exact mathematical decimal values: `1`, `1.0`, `1e0`
  agree, `-0` equals `0`, and distinct fractional values never round together.
- Permit zero and nonzero magnitudes from `1e-308` through `9007199254740991`,
  inclusive. Reject all larger magnitudes, including unsafe integers, and
  nonzero smaller values. No infinity, NaN, overflow, underflow-to-zero or
  float64 conversion. Exponent leading signs/zeros obey JSON syntax and token
  limits; token length is checked before exponent/coefficient conversion.
- Validate every numeric token in the JSON document for JSON assertions, not
  just selected numbers; unsupported numbers make JSON checks
  `not_evaluated/json_number_unsupported`. A status check can still execute.
  This stricter assertion domain leaves extraction and standalone behavior
  unchanged. It is a review decision, not an existing parser guarantee.

A bounded coefficient and at most 308 exponent plus 128 fractional places can
be compared as decimal coefficient/exponent pairs or `math/big` integers/rationals
with bounded power-of-ten construction. Arbitrary exponent allocation is
prohibited. Parse the <=2 MiB body once, use the existing depth32 check, validate
number tokens once, then traverse at most 8×32 selectors. Reuse the parse tree
with extraction where practical, without broadening legacy extraction semantics.
The worker deadline still bounds work; deadline exhaustion is not an assertion
mismatch. No user code, remote lookup or extra request occurs during evaluation.

## Sensitive configuration and result disclosure

Expected literals are **public configuration**. The editor must state:
“Use public expected values only. Assertions cannot store or compare secrets.”
No protected expected values, reveal controls, refs or extracted operands.
This does not claim arbitrary secret detection; public user-entered text has the
same deliberate-public limitation as other public configuration.

Before saving/admission, resolve selector sequences into canonical JSON-pointer
segments for comparison, using decoded property names. Reject equality if any
property segment matches `CredentialName`, if its pointer overlaps configured
response redaction in either direction, if the whole response is policy-omitted,
or if it overlaps any extraction source path. The last rule is deliberately
conservative: **every extraction path is equality-ineligible**, even
`sensitive:false`, because extraction capture registers all output values for
redaction and later credential bindings can make them sensitive. Root equality
overlaps every descendant sensitive/extracted path. Compare segment boundaries,
not raw path string prefixes. Absent runtime containers do not bypass this check.

Current workflow admission creates empty response policies; future configurable
policies and retained frozen policies must still be checked. At runtime, finish
extraction discovery/taint classification before deciding equality eligibility.
Inspect the bounded raw tree in memory for recognized sensitive property paths,
configured omissions and existing saved/runtime raw/rendered/encoded reflection
matches, including newly extracted values. If a selected path or expected literal
is newly classified sensitive, do not perform/retain its comparison; use
`not_evaluated/sensitive_input`, fail the step, and publish no match bit. Never
rewrite an already stored public literal as a purported safe secret value.
Known unsafe public configuration must be rejected before its next persistence;
runtime discovery cannot retroactively undo deliberate public authoring.

Existence at sensitive paths is allowed for the owner browser and stores only
presence/missing through its outcome; it never exposes the value, length, type,
hash or prefix. This is a small explicit owner-facing presence disclosure.
Current MCP has no assertion permission/reporting policy, so neither existence
bits nor equality outcomes/definitions are exposed through MCP in this slice.
Later MCP assertion reporting requires separate acceptance. Generic execution
failed status/fixed `assertion_failed` remains visible as existing outcome data.

Results contain only opaque assertion ID, fixed kind, outcome and fixed reason.
No actuals, expected literals, paths, body excerpts, hashes, dependency errors
or user messages in results, status polling, logs/traces, SQS or MCP. Rules/paths
remain available only through authorized workflow definition/frozen run context;
keep browser sessions/editor/context in memory, clear on sign-out/project change,
and discard late responses. No Web Storage/cache persistence of editor state.
Do not log request DTOs or validation/library exceptions. Redact known values
before evidence publication as before; never use stored `[REDACTED]` as input.

## Immutable evidence and read contract

Newly admitted assertion-capable cloud-workflow executions publish **evidence
schemaVersion 2**, including empty-rule runs. Standalone cloud/local executions
and preactivation runs retain schema 1. Schema 2 has exactly the schema-1 fields
plus required `assertions`; it is not a generic optional-field expansion of v1.

```json
{
  "schemaVersion": 1,
  "state": "evaluated",
  "results": [
    {
      "assertionId": "41aeec9e-e093-4533-b0d9-f8dc4dde7584",
      "kind": "jsonEquals",
      "outcome": "failed",
      "reason": "value_mismatch"
    }
  ]
}
```

This is the `assertions` object, whose own schemaVersion is 1. Its state is
`not_configured` with `results:[]` for zero rules, otherwise `evaluated` with
exactly one ordered result for every frozen rule, even when all are
`not_evaluated`. “Evaluated” means the evaluation attempt is recorded, not that
all checks ran/passed. Results must match frozen IDs/kinds/order without extras.
Any failed/not-evaluated check prevents assertion success. A skipped successor
has no execution/evidence or fabricated result; browser shows “Skipped.”

| Outcome         | Allowed fixed reasons                                                                                                                                                                                            |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `passed`        | `matched`, `exists` (existence only)                                                                                                                                                                             |
| `failed`        | `status_mismatch`, `path_missing`, `type_mismatch`, `value_mismatch`                                                                                                                                             |
| `not_evaluated` | `not_dispatched`, `status_unavailable`, `response_unavailable`, `body_absent`, `body_unsupported`, `body_limit_exceeded`, `json_invalid`, `json_number_unsupported`, `sensitive_input`, `evaluation_unavailable` |

Validate kind/reason combinations (for example status uses only matched,
status_mismatch or applicable unavailable reasons). Per-rule reason precedence:
not dispatched; missing trustworthy input; incomplete/over-limit body; absent
body; unsupported encoding/media/charset; invalid JSON; unsupported number;
sensitive input; path/type/value comparison. A trustworthy final status can be
checked independently after body failure. A post-intent lost buffer uses
evaluation_unavailable, not a fabricated mismatch or fresh HTTP request.

Preserve `GET .../executions/{id}/evidence`'s existing envelope
`{data:{evidence:...}}`, delivering the original v1/v2 document without migration.
The owner browser decodes by evidence version. Definition and run detail return
normalized arrays for authoring/context. Results are loaded from evidence only,
not copied into mutable summaries, list pages or light status polling. With no
retained evidence, show expired/unavailable honestly, even if run context exists.
For schema-1 evidence show “Assertion results unavailable”; do not reconstruct
from bodies or a current definition. Unknown versions fail closed.

`assertions` compact serialization must fit **2 KiB**, never omit results to
make room for bodies. The complete evidence, API envelope and base64 Lambda
envelope retain 4 MiB/4 MiB/6 MiB caps and metadata allowance. Existing immutable
objects, checksums, manifests and stored lengths stay byte-identical. Results
have no independent storage/history/retention route.

## Deterministic failure precedence and advancement

Assertions cannot change HTTP status, dispatch certainty or a stronger failure.
Choose the first applicable tier, preserving current baseline branch order:

1. Pre-dispatch source/validation/revocation/transport failures: existing code
   and `not_dispatched`; all checks not_evaluated/not_dispatched.
2. Network/read/timeout failure after intent: existing code and `unknown`;
   evaluate only an independently trustworthy final status. Body checks cannot
   evaluate incomplete data. Existing response-limit/capture/encoding branches
   retain their code/certainty and branch precedence when multiple conditions
   occur (the current limit check precedes read-error handling).
3. Existing sanitization/header/capture failures, `extraction_failed`, and
   `runtime_storage_failed`: preserve their existing runtime order and known
   certainty. An extraction failure outranks an assertion mismatch. Complete
   safe assertion results may still be recorded; no unsafe extraction values.
4. Baseline HTTP classification: final status outside 200–399 yields
   `upstream_http_error/known`, regardless of matching assertions.
5. Otherwise, any failed/not-evaluated configured assertion yields
   `assertion_failed/known`; otherwise preserve completed/null-code/known.

This adds no new primary code for invalid JSON: absent/unsupported/invalid JSON
with configured JSON rules is assertion_failed unless a stronger baseline error
already exists. With no JSON rules, legacy body-omission behavior remains.
Later delivery-size omission is not a capture error. Existing extraction plus
omitted-body failure behavior remains; do not accidentally broaden extraction
success while adding assertions. If the complete result cannot be published,
publication remains unavailable/uncertain under existing reconciliation, never
a synthetic success or successor release.

Set the final evidence status before `workflowFinalActions`. Publish immutable
evidence through existing registered ticket/S3/manifest flow. Atomic transaction
commits execution/step/run terminal state, summary, retention/accounting,
skipped successors and runtime ciphertext deletes. On failure retain no producer
output envelope and create no successor job/OUT. On success keep existing source
guards and atomic output/successor scheduling. Assertion evaluation must finish
before sealing outputs for advancement; transient buffers are discarded on
failure. Preserve dispatch fences, identical transaction-token retries/readback,
source revocation, recovery-generation suppression and unknown-stop/no replay.
Recovery may adopt already uploaded valid versioned evidence after identity,
checksum, frozen-rule/result and publication checks; it must not reevaluate
sanitized evidence or send again to repair missing results.

Pinning protects the same evidence bytes. Existing expiry/run-context rules,
last-expired-unpin, project deletion overriding pins, pending late uploads,
unknown-record preservation and cross-store completion proofs apply. Run context
can retain public rules while a sibling is available; it cannot resurrect expired
results/bodies or retain terminal runtime values.

## Compatibility and activation proposal

No schema activation occurs during preparation. Proposed next capability is
`savedRequestsSchemaVersion:8`, keeping sequentialWorkflows1, MCP1, cloud1,
inputs1, protected1, pinning1 and local1, plus
`workflowAssertionsSchemaVersion:1`. Preserve recovery generation, counters,
quotas, account plan and concurrency. Add the semantics marker to new run rows;
legacy missing marker means legacy behavior. Row schema versions stay at their
existing kind-specific values; evidence schema 2 is a separate version domain.
New marker/shape validators must be explicit, not relaxed unknown-field reads.

| Consumer                            | Required coordinated compatibility work before activation                                                                                                                                      |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| API workflow HTTP/service           | Field-specific null checks, strict rule union, legacy normalization, frozen semantics marker, whole-run rerun preservation, revision/security/serialized-budget validation.                    |
| Worker/runtime/reconciler           | Version-aware capture/evidence creation, evaluation and finalization; recover v1/v2 with frozen-result validation and existing identity/checksum checks.                                       |
| Store/cleanup/ownership             | Both stage gates/guards; new row field allowlists and exact round trips; ownership project drain plus execution expiry/pin/context/restore paths. Unknown fields/versions still block cleanup. |
| Browser                             | Versioned evidence decode and frozen-rule context, honest legacy/expired/skipped displays, editor limits/null/stale revisions and in-memory state.                                             |
| Local agent/result canonicalization | Keep grants/capture/result/receipts and standalone evidence at current versions. Do not infer schema-2 support from shared Go struct changes. Explicit negative/regression tests.              |
| Hosted MCP/stdio schemas            | Decode stored evidence 1/2, then project to a dedicated frozen legacy capture DTO. Do not let a new shared field expand tools or leak assertions.                                              |

MCP compatibility needs a small **explicit amendment** to complete-view meaning:
for stored schema 2, emit only the full legacy capture projection with its
projection `schemaVersion:1`, `complete:true`, `truncated:false`,
`omissionReason:"assertions_not_exposed"`, `omittedFields:["assertions"]`.
“Complete” means every supported legacy capture field is present, not that the
wire projection is the stored object. The omission metadata makes that distinction
visible without exposing rules/results. Existing strict wire shape remains
unchanged; the projection is never persisted or substituted for checksum bytes.
Summary view remains summary with its current omission metadata plus
`"assertions"` in omittedFields. Stored v1 reads remain unchanged. Preserve
96 KiB complete-capture and 256 KiB duplicated text/structured RPC bounds,
retention rechecks, audit/credential/ownership and untrusted-data policy.
No fabricated empty assertion report. Existing stdio clients must be tested
against this projection; no tool/result-reporting extension is implied.

Follow-up rollout must deploy and verify compatible readers/cleaners before
new writers activate; close the stage through the existing maintenance gate,
settle/fence old inflight work, inspect exact packages and stage/accounting,
then guarded CAS activates the capability. Old binaries reject version 8.
Stage version is read fresh at rollout, not assumed from this packet. Schema-2
publication prevents simply rolling back old binaries: keep the stage closed
until compatible code is restored. No downgrading/reencoding old evidence.
Separate review must authorize AWS preview, account/non-root checks, deployment
and activation; preserve us-east-2 and existing resources/bootstrap boundaries.

## Size feasibility and implementation proof obligations

The draft's eight independent 2 KiB literals plus maximally escaped paths cannot
be multiplied across ten steps and assumed to fit a 64 KiB definition. The new
8 KiB per-array cap and existing cumulative caps resolve that conflict. Empty
arrays add 16 bytes per step as the `,"assertions":[]` member; the marker adds
small bounded row framing, all counted before admission.

At eight results, each object has a 36-byte ID, kind <=12 bytes, outcome <=13,
reason <=23 and fixed JSON keys/framing <=65: <=149 bytes/result, plus <128
bytes wrapper/comma overhead gives <1,320 bytes, below the 2 KiB result cap.
Use actual serializers in follow-up boundary tests, not this estimate alone.
No rule/results text is added to Summary or request Plan, no extra result row,
transaction action or SQS payload. Results live in evidence; frozen arrays live
in existing definition/run JSON. Thus admission/terminal/advancement action
counts are unchanged, including ten-step skips and output deletes.

An admissible maximal-count example has ten steps×eight checks with short
paths/public small expected values: assertion bytes well below 64 KiB, and
80 results across ten independent evidence documents. An individual 2 KiB
expected string also fits the 8 KiB array limit with short surrounding rules.
Combining every independent maximum is deliberately rejected. Each combined
Plan+Step <=64 KiB and aggregate <=256 KiB are still necessary; when stored JSON
is nested in AttributeValue JSON, escaping/field overhead can cross 128 KiB.
Admission must measure actual resulting items/transactions, not assume 64 KiB
source JSON always fits that item budget. Reject oversized inputs before 202;
reserve enough bounded finalization/evidence framing before the first send.

Preparation used Go `encoding/json`, existing runtime Plan/Evidence types and
the installed AWS `attributevalue.MarshalMap` for an isolated shape calculation.
No evaluator, storage writer or application implementation was added. Measured
compact UTF-8 sizes were:

| Review shape                                                                                  |         Bytes |
| --------------------------------------------------------------------------------------------- | ------------: |
| Eight short `jsonEquals` rules (`$.ok`, expected true)                                        |           841 |
| Ten-step definition with those eight rules per step                                           |        10,065 |
| Existing Plan plus one such augmented Step                                                    |         1,366 |
| Ten combined Plan+Step documents, before initial input envelope                               |        13,660 |
| Representative run JSON-string attribute item containing that definition (not a full run row) |        11,539 |
| Single short-path rule with exactly 2,048-byte serialized expected string                     |         2,150 |
| Eight result objects using maximum field widths, with assertion wrapper                       |         1,155 |
| Bodyless schema-2 evidence with that result-width fixture / owner API envelope                | 1,926 / 1,948 |

The width fixture deliberately combines longest enum strings to overestimate
legal combinations; it is not a valid rule evaluation. IDs are synthetic UUIDs;
the plan uses HTTPS example.com with empty field collections. These measurements
prove maximum check count and the maximum individual expected primitive are
feasible, alongside the algebraic result ceiling. They do not prove arbitrary
combinations of request/definition maxima fit: actual full row, encrypted input,
transaction and final transport measurement remains mandatory at admission.
An isolated standard-library `math/big.Rat` comparison also confirmed exact
equality for 1/1.0/1e0 and signed zero, distinction for
0.10000000000000001/0.1, and correct ordering at
9007199254740991.1/9007199254740991 and 1e-308/zero. That is feasibility evidence,
not an implemented bounded parser or a validation test for the proposed domain.

Follow-up must prove exact/one-over with production serializers for arrays,
escaped Unicode/controls, 2 KiB primitives, 64 KiB workflow/combined plan,
256 KiB freeze, 128 KiB attribute items, 60-action/2 MiB workflow transactions,
2 MiB response, 2 KiB result, 4 MiB evidence/API and 6 MiB Lambda transport.
Count repeated run configuration, conditions, envelopes, summaries and framing.
If a legal admitted configuration cannot finalize under those caps, lower
admission bounds before implementation acceptance; no post-send truncation of
rules/results or extra history store. No extra AWS resources are proposed;
CPU/storage costs remain bounded by existing admission and response caps.
Refresh cost/runtime measurements before any deployment request.

## Verification plan and review gates

Future tests must cover the following; these are not claimed by packet checks.

| Area              | Required evidence                                                                                                                                                                                                                                                                                                                  |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Authoring         | Create/edit/remove/reorder, stable IDs, omission/clear, explicit null equality, duplicate/unknown fields, eight/one-status/array/primitive limits, stale If-Match and editing during admission.                                                                                                                                    |
| Semantics         | HTTP 200 correct/incorrect data; 404 match still fails; missing status; root scalars/null/object/array; nested arrays; missing versus present-null; false/zero/empty string; wrong types; Unicode escapes/no normalization; denied path grammar.                                                                                   |
| Numeric           | 1/1.0/1e0, signed zero, exact distinct fractions, boundary safe integer, fractional value above bound, overflow/underflow, exponent/token limits, unsafe number outside selected path; extraction regressions unchanged.                                                                                                           |
| Body/precedence   | Empty/invalid/non-JSON/charset/encoding/oversized/incomplete bodies, timeout/unknown and HTTP failure combined with extraction/assertions, policy omission versus encoded-size omission, missing buffers, storage and sanitization failures.                                                                                       |
| Security          | Recognized/configured/ancestor/descendant/root/equivalent escaped paths; every extraction path including sensitive:false and downstream Authorization; dynamic raw/rendered/encoded reflections; sensitive existence only owner presence; no values/path/literals in error/log/queue/polling/MCP; browser clearing/late responses. |
| Atomic/no replay  | Exactly zero successor sends on failed producer, duplicate admission/SQS/OUT, two finalizers, lost ACK/transaction conflict, after PUT adoption, missing result/buffer, revoke/edit/delete races; one terminal result, no double accounting/advance/send.                                                                          |
| History/lifecycle | Frozen edits/whole-run rerun with fresh IDs/current eligible stable secrets; legacy absence, no-rules, skipped versus unavailable; pin/expiry/last unpin/context, project deletion/late uploads/unknown rows and restore-generation suppression.                                                                                   |
| Compatibility     | Mixed old/new workflows/runs/evidence through strict owner API, recovery, both cleanup paths, pinning, browser, local result canonicalization and installed stdio MCP. v1 object bytes/checksums unchanged; v2 projection reads safe with no assertion fields/bits.                                                                |

After implementation authorization, run relevant formatting/lint/unit/build/Go
vet, DynamoDB Local race and Chromium integration checks. Real AWS/Cognito,
email/IAM/runtime, controlled target send counts, process crash/ACK loss,
retention/PITR and costs require separate evidence/authorization as applicable;
Local fixtures do not establish them. Historical workflow operational and
publication-incident follow-ups remain open on their own cards.

Preparation read the full open board, target/dependency/coordination details and
checklists; Doing is empty, with configured WIP limit three. Only preparation
evidence is to be checked complete after verification and remote publication.
Keep acceptance, implementation, contract synchronization and actual runtime
validation items open. The current proposal/API contracts were read, not edited.
Preparation verification passed repository Prettier/gofmt, frontend ESLint,
58 frontend tests, 15 infrastructure tests, TypeScript/Vite production build,
and tests/vet across all seven Go modules (five product modules plus two preserved
spikes). The initial Go verification stopped because its build cache was outside
the filesystem sandbox; the approved outside-sandbox rerun passed. Markdown
fences, JSON examples and local links were checked. These are baseline/document
checks and isolated serializer/decimal feasibility calculations, not assertion
implementation tests, DynamoDB Local race/browser integration, or genuine
AWS/Cognito/email/IAM/runtime validation. No deployment or activation occurred.

## Decisions requiring explicit acceptance

All remain **Proposed**:

1. Cloud-workflow-only first slice; standalone/local targets and MCP assertion
   reporting remain backlog; browser inspection uses frozen context plus evidence.
2. Existing HTTP failure policy and precedence: expected 404 can match a check
   while execution/run still fail; stronger extraction/capture/runtime failures
   outrank assertion_failed.
3. Public-only expectations, conservative rejection of all extraction-path
   overlaps, runtime sensitive_input fail-closed behavior, and owner-only sensitive
   existence presence disclosure without new MCP bits.
4. Existing path grammar with assertion-specific presence/type/null/string rules,
   plus the bounded exact decimal domain (a new assertion-only restriction).
5. Evidence v2/assertions v1 and frozen semantics v1; eight checks, 2 KiB expected,
   8 KiB arrays, 2 KiB results and cumulative measured budgets; byte-identical
   legacy evidence and no independent assertion history.
6. Coordinated savedRequests8/assertions1 capability and all-reader cleanup
   rollout, including the explicit MCP complete legacy projection amendment.

Acceptance must identify the exact review commit and authorized follow-up scope.
Implementation, external contract synchronization, AWS preview, deployment and
activation follow separately; preparation/push does not authorize them.
