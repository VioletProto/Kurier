# Project environments implementation and local verification

## Release preparation — 2026-10-27

PRs #15 and #16 are merged. The proposed
[deployment review packet](project-environments-deployment-review.md) pins release
source `44f1fb79d943501c95e06fcc1a6114111a7c1ea4` (PR #16 merge, including
implementation head `82fa7d760a485934bde3259f7652ab1c434ea85f`). It records
source-derived deployment/IAM expectations, compatibility, closed-stage activation,
no required backfill, post-data rollback limits and a bounded runtime/cleanup plan.
Fresh repository verification, uncached runtime/agent race tests and clean ARM64
builds passed locally. This preparation did not repeat browser/database integration
or establish live AWS state. AWS inventory/preview, deployment, migration,
activation, identity creation and live target requests remain held. The packet is
for review; the environment tracking card remains open. OpenAPI production scope
remains separate.

## Implementation record

Davian accepted the [review packet](../architecture/drafts/project-environments-review.md)
and [compatibility amendment](../architecture/drafts/project-environments-compatibility.md)
at `d8be596a2306f2e888ef34e81dfa836bc4b84447` on 2026-10-24. The implementation
below was verified on the feature branch and subsequently merged in PR #16.
Merge does not activate a stage or establish deployed runtime evidence.

AWS inventory/preview, deployment, migration and activation remain held for
separate review. No AWS calls, accounts, identities or resources were used for
this implementation campaign. OpenAPI production import/validation remains
unaccepted. Evidence **3** belongs to environments; OpenAPI has no allocated
production evidence version. Library acceptance is unchanged.

## Delivered behavior

Owner environment POST/list/detail/PATCH/DELETE routes use existing Cognito
owner/project/user/stage fences, no-store, revision ETags and quoted If-Match.
Variables replace as a collection, retain stable server IDs and immutable
name/type/protection, and have exact primitive/tagged-decimal DTOs. Protected
writes are encrypted before guarded cross-table publication; reads return fixed
masks and reusable saved-secret refs. Typed saved-secret metadata is trusted,
immutable and included in source conditions; PATCH keeps its value:string
transport and rejects cross-type values. Deleting an environment detaches refs;
explicit secret revocation and project deletion retain their existing meaning.

The browser editor keeps numbers in text state, clears protected drafts before
writes and after failed validation, supports typed reuse/preserve/set, and
requires reconciliation of uncertain writes. Environment summaries and selectors
contain metadata only. Requests opt in explicitly; legacy definitions remain
literal. Saved opt-in cannot be removed through PATCH. Standalone selection,
workflow run defaults and step inherit/none/select follow the accepted precedence.
No process environment, global defaults, expression evaluation or implicit output
bindings are introduced.

The shared resolver supports public URL base/whole path components, ordered
query/header values and primitive JSON string-value slots or text bodies. It
bounds references, distinct variables, JSON traversal and rendered output;
rejects missing/disabled values, unsafe URL/header structures, nonstring mixed
interpolation, credential declassification and overlapping direct/workflow
bindings. It resolves once into transient copies and adds protected rendered
variants to the redaction corpus. Workflow output bindings apply afterward and
cannot become another template program.

Admission freezes original templates, only used public typed values, exact
protected refs, variable IDs/type mappings and effective environment identity.
Guarded source conditions join the existing receipt/quota/input/outbox transaction;
private versioned HMAC domains distinguish environment admissions. Receipt lookup
precedes newer environment revision checks. Ordinary edits do not change admitted
values; dispatch/advancement check current membership/enablement/type and exact
source eligibility using current observed revisions. Reruns retain frozen public
values and select current values at the exact frozen secret IDs. Removed IDs,
deleted environments and revoked secrets cannot be rebound by name.

Cloud and local standalone requests and cloud-only workflows share this resolver.
Local Work/Result **2** require an exact credential-bound capability declaration
before admission. Legacy session DTOs and Work/Result **1** remain unchanged for
legacy work. Grants and results carry matching safe identity, included in RFC 8785
canonicalization and retained result receipts; verification independently rebuilds
and sanitizes capture. Retried accepted uploads work after input cleanup.

Owner evidence **3** contains template semantics and environment ID/revision/null,
with optional existing cloud-workflow assertions. Existing evidence 1/2 bytes
and owner read behavior are preserved. MCP retains seven tools and the existing
`view:"complete"`/`capture` envelope, projects through the dedicated legacy DTO,
and explicitly lists omitted environment/assertion fields. Projection never
rewrites stored evidence, checksums or pins. Cleanup recognizes the new exact
row shapes and typed secret records, uses bounded resumable cursors and preserves
unknown records rather than deleting them.

## Capability boundary

The new complete tuple is savedRequests9, projectEnvironments1, protectedSecrets2,
localAgentExecutions2, with cloud1/inputs1/pinning1/workflows1/MCP1/assertions1.
Readers and transaction conditions recognize it; application startup does not
write it. Only isolated fixtures set the tuple during these checks. Gateway route declarations include the six new API operations and explicit
environment collection/item preflights for a later reviewed deployment. No IAM
expansion, migration, activation command, deployed-state inventory or permissive partial tuple was
introduced. Reviewing deployment and an actual closed-stage transition is separate.

## Verification and practical limits

The decoded-JSON correction (2026-10-26) removes a raw-byte delimiter shortcut:
JSON string values are inspected after escape decoding. Bodies with no
replacements retain their exact original bytes; replaced bodies retain untouched
numeric tokens and insert variable values once. Regressions cover Unicode and
mixed delimiters, root/array values, inert inserted markers, protected-value
redaction, malformed input and traversal/reference/output bounds. A controlled
HTTP execution checks the actual outgoing bytes; API freeze/rerun fixtures also
use an escaped template. Review of discovery, authoring validation, workflow
overlap checks and cloud/local execution found those paths already use decoded
string values through the shared resolver; no protocol allocation changes are
needed.

The verification campaign covers:

- full repository formatting, lint, unit/tooling/infra tests, web build and Go
  tests/vet/test-binary builds through `npm run verify`;
- uncached race tests for the shared runtime and compiled local-agent client;
- real DynamoDB Local transactions with simulated KMS, memory-only object/queue
  adapters and controlled HTTP fixtures: owner CRUD/foreign access, exact values,
  strict unions, preconditions, current-secret PATCH, frozen admissions/receipts,
  revocation, local negotiation/result identity/receipt retry, workflow assertions,
  MCP projection and normal deletion/cleanup;
- Chromium against the Go API and DynamoDB Local, using fixture JWT/JWKS and
  Cognito responses, plus a compiled foreground CLI against controlled local TLS
  and HTTP: author/reopen exact decimals, masked protected values, Work 2
  negotiation before admission, evidence 3 and frozen-public/current-secret rerun;
- the existing ownership/publication/race/retention/MCP and browser regression
  suites, including legacy execution and assertion behavior.

The browser fixture clock now advances between deliberate retention jumps so
agent rate-limit windows behave normally; the fixture permits bounded CLI backoff.
New browser resources are cleaned after failures as well as successful runs.
Historical research remains under spikes/environment-resolution; legacy wire
experiments now use explicit frozen pre-environment field sets, rather than
mistakenly treating upgraded production readers as old readers.

These are local/CI results, not genuine AWS KMS/S3/SQS/Cognito/IAM, deployed DNS/
network, email, migration, activation, elapsed retention/PITR, performance, billing
or public signup evidence. Review of this implementation does not authorize those
campaigns. Exact published head and CI outcome belong in the implementation PR
and Trello, avoiding a self-referential commit hash in this file.
