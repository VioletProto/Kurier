# OpenAPI import, export and response contracts

Implementation direction: Davian requested proceeding on 2026-11-04 after the
[milestone review](../development/openapi-next-milestone.md). This contract uses
that review's recommended defaults. It describes application behavior; deployment
and genuine hosted validation are recorded separately. The
[library-selection research](../spikes/openapi-library-selection.md) supplies the
detailed supported grammar and security rationale. Libraries remain pinned to
kin-openapi 0.152.0, jsonschema/v6 6.0.3 and yaml3 0.0.14.

## Import and publication

Accept OpenAPI 3.0.0–3.0.4 as one strict JSON or JSON-compatible YAML document,
at most 2 MiB and 100 supported operations. JSON does not accept YAML syntax.
Reject duplicate keys, aliases, custom tags, multiple documents, invalid Unicode,
unsupported keywords/styles/dialects, cyclic or external references, and bounded
depth/work overruns. References resolve only within the uploaded document;
neither the parser nor schema compiler fetches network or filesystem references.
Diagnostics are fixed codes, without parser messages, snippets, values or paths.

Support GET/POST/PUT/PATCH/DELETE; inherited path/query/header parameters with
operation overrides; primitive path/header values; primitive and repeated-array
query values; optional/required JSON bodies; and bounded documented examples.
The schema subset supports types, required properties, nullable, array items and
boolean additionalProperties. Unsupported constraints reject explicitly rather
than being silently dropped. Numeric examples and owner inputs retain their
original bounded decimal lexemes.

Imports prepare candidates before creating executable saved requests. Owners
choose a declared server, or provide an absolute public server when none is
usable, then complete required inputs without examples. Multiple servers require
an explicit selection. Concrete URL generation encodes path values and leaves
query descriptors separate. Safe examples provide defaults; unsafe selected
examples become missing input rather than silently selecting a different sample.
Existing protected secret references can complete credential parameters/body;
samples never automatically create credentials or bind same-name secrets.

Lifecycle: `creating` → `preparing` → `publishing` → `ready`. Preparing choices
use revision compare-and-swap. Publication writes hidden requests in bounded
transactions, then one ready-marker transition exposes the complete import.
Every request reader and admission checks that marker. A failed or uncertain
publication remains hidden and can be canceled; it is never automatically
replayed. Cancellation is available for nonready states, including partially
created/published imports. Ready imports remain immutable while referenced;
project deletion removes them through normal cleanup.

The source and bundles are sanitized before persistence. Descriptions, unsafe
examples and unused annotations are omitted; structural constraints are preserved
or rejected when unsafe. Additional user-designated values/pointers apply to the
upload in memory and are not retained. Owner-authored completion inputs must be
public values or existing protected references. Only private, sanitized source
chunks, bounded operation rows and public choices are stored in the existing
Control table. This uses the accepted DynamoDB topology without adding an upload
worker, bucket or IAM permission. Source chunks are 48 KiB before base64, and
operations remain at most 32 KiB.

## Owner HTTP interfaces

All paths below are under `/api/v1/projects/{projectId}/openapi-imports` and
require an active owned project and the new stage capability.

| Method and suffix                          | Request and response                                                                |
| ------------------------------------------ | ----------------------------------------------------------------------------------- |
| POST collection                            | `{format:"json"                                                                     | "yaml",source,publicDataConfirmed:true,sensitiveValues?:string[],sensitiveJSONPointers?:string[]}`; 201 `{data:{import}}`, Location and revision ETag |
| GET collection                             | Signed owner/project/stage-bound cursor; `{data:{items,nextCursor}}`                |
| GET `/{importId}`                          | Optional `operationOffset`; `{data:{import}}`, revision ETag                        |
| PATCH `/{importId}`                        | If-Match; `{operationId,choices,publicDataConfirmed:true}`; updated detail and ETag |
| POST `/{importId}/publish`                 | If-Match, `{}`; ready detail on success                                             |
| GET `/{importId}/export?format=json\|yaml` | Private sanitized document download with fixed attachment filename and no-store     |
| DELETE `/{importId}`                       | If-Match, empty body; 204 after bounded cancellation; ready import returns 409      |

Detail includes import/project IDs, revision, lifecycle state, operation count,
timestamps, safe diagnostics, `readyOperationCount`, paged `operations` and
`nextOperationOffset`. Each candidate contains its sanitized operation, choices,
exact `choicesJSON`, generated request ID and readiness. HTTP previews return at
most 20 candidates within a measured 2 MiB response envelope. Publication still
covers all operations, independent of the viewed page.

Choices are `{server?,parameters?:{[inputId]:JSON value},body?:JSON value,
secretRefs?:{[inputId]:secretId},bodySecretRef?:secretId}`. The browser edits raw
JSON text and sends exact numeric tokens; it never roundtrips them through
JavaScript Number. Raw uploads and sensitive designations are cleared before
submission and never stored in browser storage. Unknown write outcomes require
readback, without automatic retry.

Export provides two encodings of the retained sanitized OpenAPI document. It
does not reconstruct omitted values, comments, original formatting or secrets.
It does not import arbitrary standalone JSON Schema or convert general dialects.

## Admission, evidence and execution

Saved requests use nullable `operationRef:{importId,operationId}`. Associations
are resolved from owned ready records at admission, guarded in the admission
transaction, and frozen with the operation bundle and import revision. Existing
request edits preserve the association unless explicitly detached. Referenced
request records use schema 4. A new stage tuple uses savedRequests 10 and
openapi 1, preserving the complete environments tuple and all other capabilities.
Startup never activates it. Earlier supported tuples remain readable.

Cloud standalone requests and cloud workflow steps validate raw responses before
sanitization. Checks cover undocumented status, required response headers, missing
required JSON properties and primitive type mismatch, including array/object
constraints within the subset. Status selection is explicit, including redirects
and no-body responses, and supports documented defaults.

Owner evidence 4 adds:

```json
{
  "contract": {
    "importId": "…",
    "operationId": "…",
    "revision": 3,
    "result": {
      "semanticsVersion": 1,
      "status": "passed",
      "checks": [
        { "kind": "status", "status": "passed" },
        { "kind": "headers", "status": "passed" },
        { "kind": "body", "status": "passed" }
      ]
    }
  }
}
```

Results contain fixed codes only, without values, header names, JSON paths or
validator diagnostics. Existing environment context and workflow assertions can
coexist. Evidence 1–3 remain unchanged. Frozen reruns retain the bundle, and
original evidence stays immutable through retention and pinning.

Protected inputs, recognized or designated sensitive response content and
workflow extraction conservatively suppress validation as `not_evaluated`.
Suppression does not expose secret-dependent presence or match bits. A failed or
unavailable configured contract fails an otherwise successful execution and
prevents workflow successors; transport, safety and existing HTTP failures keep
their earlier precedence. The UI explains reduced coverage.

Local admission with a contract association rejects before dispatch. Owners may
detach the association to use the generated ordinary request locally; sanitized
local results are never presented as raw-response contract validation. The seven
existing MCP tools retain their explicit legacy capture projection with contract
fields omitted. MCP contract-result exposure remains a separate milestone.
