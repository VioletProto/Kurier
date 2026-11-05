# OpenAPI import, export and response validation

The implementation follows the [OpenAPI contract](../architecture/openapi-contract.md).
It adds a project OpenAPI panel for strict JSON/YAML upload, safe diagnostics,
missing-input completion, explicit server selection, ready-only generated requests
and sanitized JSON/YAML download. Linked requests validate cloud responses and
display immutable contract results. Existing request edits preserve associations;
owners can explicitly detach them for ordinary local execution.

Implementation lives in `services/request-runtime/openapi`, ownership import
handlers/storage/cleanup, execution admission/runtime and `apps/web/src/OpenAPI.tsx`.
It reuses existing DynamoDB tables and request protection, publication, retention
and cleanup patterns. No additional infrastructure service or custom release
harness is introduced.

## Using the flow

1. Open a project, then **Open OpenAPI imports**. Choose JSON or YAML and select
   a file or paste a supported document. Designate additional sensitive values
   or JSON pointers, confirm the remaining data is public, then prepare.
2. Review safe diagnostics and operation previews. Select a server and complete
   missing public inputs using the displayed input IDs. Exact decimal literals
   are preserved. Protected inputs use existing secret references. Candidate
   pages can be loaded without publishing anything.
3. Publish when every operation is ready, then refresh saved requests. Select
   an imported request and execute it in the cloud. The evidence panel shows
   fixed status/header/body checks without response values or validator paths.
4. Export either sanitized JSON or YAML. Downloads reflect retained structural
   content and safe examples, not original comments, formatting or credentials.

Only the documented OpenAPI 3.0 subset is supported. Arbitrary JSON Schema,
3.1 dialects, remote references, security-flow generation, scripts and advanced
schema/style keywords reject explicitly. Raw-response validation is cloud-only.
Protected inputs, response redaction or workflow extraction can suppress checks;
an unavailable configured contract fails an otherwise successful execution.

Preparing or partially published imports can be canceled. Unknown write outcomes
require refreshing status before another action. Ready imports remain immutable
while linked requests exist and are removed with their project. Publication does
not execute requests.

## Verification and release

Verification and release results are recorded here when the implementation's
format, lint, build, unit, integration and Chromium checks complete. Local tests
use actual DynamoDB Local with synthetic authentication, KMS, queue/object
adapters and bounded local HTTP fixtures; they do not establish live AWS,
Cognito, IAM, email or hosted-runtime behavior.

The capability tuple adds savedRequests 10 and openapi 1 to the existing complete
environments tuple. Startup never upgrades live capabilities. Source records use
private sanitized chunks, and linked saved requests use record schema 4. Owner
evidence 4 preserves earlier versions and supports existing pinning/retention.
MCP continues its explicit legacy projection; contract-result MCP exposure is a
separate card.
