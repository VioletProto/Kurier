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

On 2026-11-05, `npm run verify` passed formatting, lint, 72 frontend tests,
15 infrastructure tests, tooling tests, production frontend build and all Go
tests/vet. The full API race/integration suite and Chromium browser suite passed
against actual DynamoDB Local. Dedicated runtime/parser race checks also passed.
The browser flow exercises upload, missing-input completion, publication,
sanitized downloads and normal project cleanup. Backend tests include all four
violation classes, 100-operation publication, interruption/hidden-row containment,
unknown-row preservation, ownership, frozen reruns and workflow successor fences.

These tests use synthetic authentication, KMS, queue/object adapters and bounded
local HTTP fixtures; they do not establish genuine Cognito, IAM, email or hosted
runtime behavior. Hosted rollout and smoke results follow separately when observed.

The capability tuple adds savedRequests 10 and openapi 1 to the existing complete
environments tuple. Startup never upgrades live capabilities. Source records use
private sanitized chunks, and linked saved requests use record schema 4. Owner
evidence 4 preserves earlier versions and supports existing pinning/retention.
MCP continues its explicit legacy projection; contract-result MCP exposure is a
separate card.

Current completion limits: public body inputs use the same 8 KiB bound as
examples, inside the existing 64 KiB complete-request bound. Credential query
arrays are unsupported; scalar credential references and whole-body references
are supported. Named samples supply one safe deterministic default with manual
typed overrides, rather than a named-example picker. These are explicit limits
of this slice, not claims of general OpenAPI support.

The first hosted deployment hit Lambda's 20 KiB invoke-policy limit: SST added
an identical API-scoped permission for each new route. The stage remained closed
at version 18 without capability activation. The routing correction reuses the
existing users/me integration for all eleven OpenAPI routes, which invoke the
same Go handler under the same existing API-scoped grant. It removes only the
two duplicate permissions/integrations created by the partial attempt, preserves
all existing routes and permissions, and adds no IAM access. Infrastructure
tests explicitly assert this shared integration to prevent policy growth.
