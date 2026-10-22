# Cloud workflow assertions implementation

Current status: **scoped implementation, deployment, functional validation and
normal cleanup are complete and accepted**. Davian accepted the retained A–G
and H–L evidence at `4ffdaaf4cfb569a349ebd16ca739d8deb15aa973`, explicitly including
the nine read-only OPTIONS failures. The [PR review record](workflow-assertions-pr-review.md)
tracks the full slice against current main. The [tooling follow-up](https://trello.com/c/F47iWJY8)
remains open for teardown cancellation and stop enforcement; the existing
correction is locally tested only. Historical packets below retain their original
holds/failures; they do not reopen functional validation. This closeout authorizes
no additional live validation, admissions, identities, deployment or AWS changes.

## Implementation and rollout chronology

Davian accepted [contract `31de3929`](../architecture/workflow-assertions-contract.md)
on 2026-10-20, explicitly including the lossless numeric DTO and MCP legacy
projection. Scoped implementation, local verification, API-contract
synchronization and Trello updates are authorized. **AWS preview, deployment and
capability activation are on hold for separate review.** No AWS command, SST
preview or capability mutation was performed by this implementation task.

The implementation branch is `feat/workflow-assertions`, based on that accepted
review commit. Deployment state is not inferred from merged/published code.

The later authorized [final rollout](workflow-assertions-final-rollout.md)
deployed the unchanged pinned release and activated assertions at version 15.
Genuine A–G passed, then the browser transport guard stopped H–L before admission.
Its bounded cleanup and remaining physical/accounting verification gaps are
recorded there; the original implementation-only holds above are historical.
The subsequent [focused verification](workflow-assertions-cleanup-verification.md)
closes current physical cleanup verification and corrects operator collection
and browser metering locally. Missing historical observations remain missing,
and H–L remain held.
The later authorized [H–L-only preflight](workflow-assertions-hl-rollout.md)
stopped on changed stage accounting before creating resources or admitting work.
It adds no campaign steps/sends; the changed global quota usage is unattributed.
The subsequent [authorized continuation](workflow-assertions-hl-continuation.md)
verified instructor deletion and reconciled the charged usage without resetting
it. Full preflight passed, then hosted authoring stopped before H admission.
The new fixture was normally cleaned; two driver defects were corrected and
tested locally. H–L runtime evidence remains open and application pins unchanged.

## Implemented behavior

Workflow steps accept up to eight revisioned assertions: one optional final
status equality, JSON existence, and primitive JSON equality. Strict unions
reject unknown/duplicate/mis-cased fields, disallowed nulls and invalid
combinations. Omission normalizes to an empty array. Whole-steps replacement
replaces assertions, while name-only edits preserve them under existing
revision/If-Match semantics. The editor can create/edit/remove checks, select
primitive type, and inspect frozen rules alongside immutable evidence results.

Numeric equality uses only `{"type":"number","decimal":"..."}` as its expected
wire value. Browser text state, native JSON transport, API RawMessage/string
decode, saved/frozen JSON and whole-run reruns preserve decoded ASCII tokens
exactly, including exponent spellings and signed zero. The browser validates
with bounded BigInt operations; the worker uses bounded `math/big.Rat`, parsing
raw response `json.Number` tokens without float conversion. Both enforce the
128-byte token, absolute exponent <=308, zero or nonzero magnitude
1e-308 through 9007199254740991. Distinct fractions remain distinct; equivalent
decimal/exponent forms compare equal without rewriting authored tokens.

The parser reuses the restricted extraction path grammar and strict JSON/depth/
Unicode rules. JSON existence accepts present null, containers and root scalars.
Equality requires a primitive of the same logical JSON type, with missing
distinct from null. All configured checks are recorded in frozen order; zero
rules means not_configured, legacy schema-1 records mean results unavailable.
The bounded response is evaluated in worker memory before sanitization/disposal.
Encoded-size body omission preserves completed results. Standalone/local-agent
execution and upload protocols keep schema 1 and existing semantics.

Public expected values are not a secret-entry mechanism. Static equality checks
reject recognized sensitive names, configured redaction overlap, whole-body
omission and all extraction-path overlaps. Runtime checks discover recognized
and reflected saved/runtime/raw/rendered/encoded taint before comparing; unsafe
checks become not_evaluated/sensitive_input. Owner existence checks disclose
only presence. Results contain fixed IDs/kinds/outcomes/reasons, with no actuals,
expected values, paths, hashes or arbitrary messages. No assertion rules/results
are added to SQS, status summaries or MCP. The browser keeps authoring/context in
memory and uses the existing project-keyed component/unmount/abort fences.

Assertions do not override HTTP failure policy or network/capture/extraction/
runtime-storage errors. A matching expected 404 still fails HTTP execution.
Otherwise mismatched/unavailable configured checks yield assertion_failed.
Failure reuses existing atomic terminal execution/step/run publication, skipped
successors and runtime ciphertext deletion; no successor job/OUT is created.
Results have no independent history or retention store. Missing worker buffers
stop with unknown outcome/evaluation_unavailable and never resend HTTP.

## Version and storage compatibility

New admission is enabled only with savedRequests8 plus
workflowAssertionsSchemaVersion1, retaining workflows1/MCP1/local1/pinning1 and
all existing capability dependencies. Both ownership and execution stage gates
and CAS guards recognize the exact combination. Existing capabilities remain
supported; nonempty assertion writes fail closed before activation. No activation
routine or deployment was run. Marker/generation/counters/quotas in AWS were not
read or modified as part of this task.

New run rows freeze assertionSemanticsVersion1. Whole-run reruns preserve that
marker and the source's exact frozen rules; a legacy run rerun retains marker
absence/schema-1 behavior even after future activation. Preactivation and legacy
stored definitions omit assertion members to remain readable by older binaries;
owner read DTOs normalize them to arrays. A dedicated RunStep serializer avoids
embedded custom authoring serialization hiding run state/position/retention.

Cloud-workflow evidence for marker-1 runs is schema 2 with assertions schema 1.
Pre-dispatch and recovery failures receive complete unavailable result sets.
Publication, finalization and owner reads validate versions and frozen result
identities; unknown versions fail closed. Old schema-1 serialization is
byte-identical. Cleanup recognizes the bounded marker/shape and preserves unknown
records. Pinning and expiry still govern the original complete immutable object.

MCP has a separate frozen LegacyEvidence DTO; generating stdio tool schemas cannot
accidentally expose added owner fields. The hosted bridge decodes schema 1/2 and
projects schema-2 complete reads to legacy schema 1 with complete:true,
truncated:false, omissionReason:assertions_not_exposed and
omittedFields:[assertions]. Summary reads also mark assertions omitted. It never
rewrites stored evidence or uses the projection for checksum verification.
Existing 96 KiB capture/256 KiB RPC bounds and authorization/retention/audit apply.

Existing complete configuration/freeze/item/transaction/transport bounds remain.
Expected values include tag framing in their 2 KiB cap; arrays are <=8 KiB and
results <=2 KiB. Admission additionally reserves 4 KiB of the existing 128 KiB
serialized-item ceiling for future terminal metadata on run, step and execution
items. This is conservative admission headroom, not a raised storage limit.
No new result rows, transaction actions, cloud resources or permissions.

## Verification performed

Local checks on 2026-10-20:

- Repository Prettier/gofmt, frontend ESLint, 64 frontend and 15 infrastructure unit tests,
  TypeScript/Vite production build, and Go tests/vet across all seven modules.
  The sandbox denied writes to the Go cache; the approved outside-sandbox Go
  rerun passed. API/worker/local-agent/MCP binaries also built successfully.
- Runtime unit tests cover exact decimal/type/presence/Unicode semantics,
  numeric domain/length boundaries, strict DTOs, unsupported numeric tokens
  outside selected paths, malformed/non-JSON/empty/oversized bodies, policy
  omission, HTTP/extraction precedence, sensitive static/dynamic paths,
  reflection suppression, exact/one-over expected and array sizes, frozen result
  validation, legacy byte identity and bounded result size.
- API serializer tests cover retained assertion results after encoded-size body
  omission with API/Lambda bounds, exact/one-over terminal metadata headroom,
  and complete flattened run DTO metadata.
- Race-enabled full DynamoDB Local API suite passed. New fixtures prove HTTP
  200 incorrect data fails, HTTP 404 remains failed despite a matching status
  check, successors skipped with zero sends, duplicate delivery sends no second
  request, failed runtime inputs/outputs removed, immutable pin/evidence reads,
  frozen exact numeric reruns after live edits, expiry, legacy reruns, gated
  writes, owner HTTP null/numeric/revision validation and completed project
  deletion. MCP complete/summary reads use the accepted projection safely.
- Additional race-enabled publication fault tests passed for lost final ACK,
  lost result buffer and lost PUT ACK: one target send, no successor, terminal
  results preserved or explicit unknown/evaluation_unavailable, never replay.
- Chromium browser-to-Go/DynamoDB Local suite passed. Workflow fixtures type,
  save, reopen, rename and whole-run rerun tagged expectations for
  0.10000000000000001 versus 0.1, 1E+00, signed zero, 1e-308 and the maximum
  integer. Unit fixtures also cover incomplete/invalid tokens and logical types.
- The actual installed Codex MCP client accepted the projection omission
  metadata against an isolated HTTPS API fixture. No model turn, AWS request or
  persistent Codex configuration/credential change was used.

Representative commands:

```sh
npm run verify
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration
KURIER_CODEX_INTEGRATION=1 go -C services/mcp test -tags=codex -run TestInstalledCodexMCPIntegration -count=1 .
```

Fixtures use actual local conditional DynamoDB transactions and controlled
loopback HTTP, with simulated KMS/S3/SQS/Cognito/HTTPS bridge dependencies.
They do not prove real AWS IAM/KMS/S3/SQS propagation, deployed runtime, actual
process crash/ACK loss, Cognito/email, real clock skew/PITR/elapsed retention,
cloud maximum-size performance or metered costs. Those require separate review
and authorization. Existing workflow operational follow-up cards remain open.
API-contract synchronization records accepted behavior, not deployment evidence.

The native [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
now contain “Accepted cloud-workflow assertions continuation (31de3929,
2026-10-20)”. A revision-guarded append preserved prior sections; readback
confirmed the exact inserted content and native heading, including numeric
round trips, result/compatibility DTOs and unchanged deployment holds. The
proposal was read without edits. Full API Contracts v0.1 acceptance and unrelated
environment/OpenAPI/MCP-result work remain separate.

## H–L replacement runtime and cleanup

The [authorized replacement packet](workflow-assertions-hl-replacement.md) records
H–L against the unchanged deployed pinned application: ten new admissions/eight
audited sends, cumulative 28/24, zero reserve submissions. Normal deletion and
provider absence are verified; charged monthly usage32 includes four separate
instructor charges, with reserved bytes/pins/inflight zero. Final read-only
reader/IAM/platform/stage/generation checks match preflight.

Nine charged read-only API OPTIONS transport errors were discovered in the
frozen audit after live processes stopped. No mutation/submission/target outcome
is uncertain. The old browser teardown can reproduce this locally; the corrected
driver drains background reads and asserts zero transport failures, locally only.
This remains a guard-review gap and does not prove exact historical causation.
No redeployment, A–G replay or further live request follows from these results.
Historical missing readbacks/billing, broad fault injection and PITR remain open.
