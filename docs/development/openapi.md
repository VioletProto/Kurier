# OpenAPI import, export and response validation

OpenAPI is deployed and active in the [hosted development/demo
application](https://d2kpq905qyh9zg.cloudfront.net) as of 2026-11-05. Stage version
19 uses savedRequests 10 and openapi 1, preserving the environments capabilities,
recovery generation and pre-deployment accounting.

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
runtime behavior. The separate hosted checks below establish the observed cloud
behavior without treating those local fixtures as AWS validation.

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

The corrected supported SST deployment completed from `17decbc`, after the
implementation commit `13186fc`. Fresh verification compared AWS package digests
with all four built archives, retained the existing IAM/configuration/plan/quota/
concurrency controls, preserved all 53 prior routes and added eleven routes using
the existing users/me integration. The final API invoke policy matches the prior
52 statements, without new invoke grants. All five frontend asset hashes and the
served manifest matched over actual CloudFront HTTPS. Frontend release:
`85d3b6d1bfa024c9ee1345a13c5377bc208a300e1b248db94a02f99cc8083747`.

| Functions                                          | Deployed ZIP SHA-256                                               |
| -------------------------------------------------- | ------------------------------------------------------------------ |
| UsersProjectsApi / EmptyProjectCleanup             | `9ecbd30c51212c968de8e7d3ffce0b7b957382ac94d65ca67cd70b6816e018cd` |
| CloudExecutionWorker / ExecutionControlledEndpoint | `0b645dbb6261b42f91de6980cbe8278ea99bcaedb979ab977046126239b05c9f` |

The stage stayed closed at version 18 throughout the failed attempt and its
correction. A single guarded transaction then activated version 19, binding every
closed marker field and all eight existing accounting preimages. No data reset,
migration, old-release reopening, plan upgrade or concurrency change occurred.
Historical environment campaign evidence was not reset or reused as authority.

The final deployed code is `a8b4127`. The Lambda HTTP adapter's older 128 KiB
ordinary-request bound initially prevented the documented larger import uploads.
Its correction permits a 5 MiB decoded JSON envelope only for canonical import
POSTs, retaining the 2 MiB source, metadata and complete Lambda transport bounds.
Real adapter regressions cover exact 2 MiB sources, plain/base64 encodings,
overruns and other-route rejection. Package checks, full repository verification
and a compatible code-only deployment passed; the active tuple, frontend,
routing, invoke policy and platform remained unchanged. The table above records
this final deployment's package hashes.

The hosted test used one no-email disposable Cognito user/project and genuine
SRP authentication. Invalid JSON and unsupported dialects rejected. JSON and YAML
exports reimported successfully with the exact decimal
`0.10000000000000001`; the protected sample was omitted from checked API replies
and exported source. Preparing requests stayed hidden, stale completion rejected,
and all five generated requests became visible only after ready publication.
Six actual controlled executions verified a passed contract, all four failure
classes, evidence 4 and a frozen rerun after changing/detaching the saved request.
The original evidence remained byte-identical. The original five-minute cleanup
observation timed out: the scheduled bounded drain removes one upload ticket per
invocation, so six executions need additional ticks. Metadata subsequently
progressed from 39 to 18 rows and then its deleted tombstone. No cleanup code
change, direct data deletion or target replay was needed; the original timeout
receipt remains unchanged.

A separate continuation reused the same temporary provider identity, submitted
one exact **2,097,152-byte** OpenAPI source in one additional disposable project,
verified successful preparation/export and canceled the unpublished import. It
submitted zero target executions. Both projects were subsequently proved
tombstone-only, with zero Protected children and empty evidence prefixes. The
same Cognito user was deleted and a subsequent exact lookup confirmed
UserNotFoundException. All functional checks, the hosted size-boundary check and
normal cleanup are complete; retained synthetic user/audit/quota/tombstone
metadata follows existing account/retention behavior.

Private operational snapshots and receipts are in ignored mode-0700
`.build/openapi-rollout`, with mode-0600 private files. Runtime credentials and
protected test values stay in RAM. This is not a comprehensive cloud audit or
measured billing report; the full hosted browser/workflow/local-agent/MCP matrix
was not repeated. Local Chromium and backend integration coverage remain distinct.
