# Next milestone: OpenAPI import, export and response validation

**Proposed implementation scope, not accepted contract or completed work.**
This note follows the project-status review on 2026-11-04. It makes no claim
about environment deployment or runtime validation; record those independently.

The current [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
requires JSON/YAML OpenAPI import, generated requests with path/query/header
parameters and optional JSON bodies/examples, operation associations and immutable
response-validation results. The [implementation card](https://trello.com/c/zUsI4e6l)
is To Do with its five checklist items open. The
[library card](https://trello.com/c/y53kkj4i) is complete: kin-openapi v0.152.0,
santhosh-tekuri/jsonschema/v6 v6.0.3 and oasdiff/yaml3 v0.0.14 are accepted.
[The existing spike](../spikes/openapi-library-selection.md) supplies the technical
starting point; its production contract remains proposed. Another library spike
is unnecessary unless implementation reveals a concrete incompatibility.

## Minimum useful delivery

Deliver one complete flow: upload a supported OpenAPI document, inspect safe
diagnostics and generated candidates, complete missing inputs, publish saved
requests, execute a selected operation and inspect its stored contract results.
Provide YAML and JSON download of the sanitized supported document as requested
in the latest project direction. Export adds scope beyond the current card and
proposal; update that scope when the contract is agreed.

Use the existing proposed OpenAPI 3.0.0–3.0.4 subset, 2 MiB source and 100-operation
bounds, same-document references and explicit rejection of unsupported features.
Never fetch specification URLs or references. Preserve strict parsing, exact
numeric examples, secret omission, existing request limits and project cleanup.
Publish generated requests only when the import is ready; never expose partial
executable requests. Include the proposal's four checks: undocumented status,
missing required JSON properties, primitive type mismatch and missing required
response headers. Persist safe results with the immutable execution.

YAML and JSON are two encodings of an OpenAPI document. This milestone does not
imply a standalone arbitrary JSON Schema importer, general dialect conversion,
Postman/Bruno collections or execution-evidence export. Export only retained
sanitized structural content and safe examples, without decrypting credentials
or reconstructing omitted source. Clearly label exports as sanitized; do not
promise original formatting, comments or byte-for-byte source roundtrips.

## Recommended defaults for the remaining decisions

1. **Inputs and publication:** use one owner preview/completion screen while the
   import is preparing. Require explicit server choice when multiple targets
   exist and owner input for required values without safe examples. Reuse the
   saved-request editor's validation and protected-field authoring. Materialize
   concrete URLs only after completion; cancel uses normal project-scoped
   cleanup. Never fill missing path values with executable placeholders.
2. **Execution coverage:** implement raw-response validation in cloud standalone
   requests and cloud workflows first. Keep generated ordinary requests usable
   locally through existing request execution, with explicit UI wording that
   contract validation is unavailable there. Reject a configured validation
   association on local submission until a versioned local validation protocol
   exists; never present sanitized-response validation as equivalent.
3. **Sensitive responses:** use the spike's conservative whole-body suppression
   whenever validation could depend on recognized, designated or reflected
   sensitive data. Report `not_evaluated` with a fixed sensitive-input reason,
   without values, paths or match bits. Apply equivalent protection to sensitive
   headers. Show reduced coverage in the owner UI; no secrets enter diagnostics,
   stored source, examples, logs, evidence or MCP.
4. **Evidence and failures:** freeze the operation revision, bounded sanitized
   bundle and validation semantics at admission. Allocate a new evidence version
   explicitly; schema 3 already belongs to environments. Preserve historical
   evidence bytes and existing failure precedence. A configured failed or
   unavailable check fails an otherwise successful cloud execution and prevents
   workflow successors. Keep MCP's existing explicit projection until the
   [separate MCP card](https://trello.com/c/jQR3w1Zt) extends its contract.

These defaults require a concise contract decision before dependent production
implementation. Reuse existing persistence, publication, cleanup and SST
deployment patterns; introduce new machinery only for demonstrated requirements.

## Completion evidence

Verify JSON/YAML import and export, supported request generation, missing-input
completion, all four violation classes, invalid/unsupported/oversize documents,
ownership, partial-publication failure, secret omission, frozen reruns and
retention/deletion. Run relevant repository format/lint/build/tests and distinguish
local fixtures from hosted execution. Synchronize the card and API contracts,
then deploy and demonstrate the complete flow with normal cleanup.
