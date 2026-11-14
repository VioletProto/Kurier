# MCP project scope delivery

Davian accepted [the design packet](../spikes/mcp-connection-project-scopes.md)
and requested implementation, deployment, testing and merge on 2026-11-14.
Delivery card: <https://trello.com/c/RaD7xqgS>.

## Behavior and compatibility

One stdio registration retains seven tools and the existing per-call schema.
The account's separate thirty-day credential can select 1–10 owned active
projects or explicitly expose all current and future owned projects. Automatic
mode permits zero projects; there are no exclusions or additional credential
slots. Every read checks current ownership, credential/version/expiry/recovery,
retention and conditioned pre-data audit. Local-agent authority is separate.

Open **MCP access** in a project or use **MCP connection** before selecting a
project. Refresh status, choose the mode, review access and confirm consent.
Automatic mode requires a second unchecked future-project confirmation. Create
a credential, transfer its one-time token through `kurier-mcp pair`'s hidden
prompt and register the executable once as described in [the setup guide](read-only-mcp.md).
The UI also displays the token-free pairing/registration commands.

Use **Save project access** to change scope for every process sharing the
credential. Token and original expiry remain unchanged; existing cursors fail
with `invalid_request`, requiring discovery from the beginning. Refresh after
an uncertain response or stale version; never blindly overwrite another tab.
Revocation remains independent from browser sign-out. Replacement/expiry/lost
tokens require reauthorization, pairing and process restart, with the same
registration.

New owner-only `/api/v1/mcp/connections` management routes implement the exact
accepted DTOs. Connection metadata adds `editable:boolean` for unambiguous legacy
upgrade handling. Legacy token routes and schema-1 credentials remain fixed
scope. Legacy GET-current returns safe409 for a schema-2 credential, while
legacy DELETE can revoke it. Old stdio bridges work with both modes because
pairing uses ordinary project discovery, whose default/max page remains ten.

Automatic discovery uses the existing owner GSI and strongly hydrates each
candidate. The encrypted cursor binds owner, stage, generation, token/version,
scope digest, tool/filters and public high-water/continuation positions. Index
lag and a fixed high-water mean a fresh traversal discovers newly created
projects after propagation; no snapshot, push or automatic all-project crawl
is promised. Query, hydration, transaction, rate, byte and release deadlines
remain bounded. Explicit last-project deletion revokes access; automatic
deletion denies the project immediately and preserves future-project consent.

## Verification and release

Local verification covers both strict unions, 0/10/11/duplicate selections,
schema isolation, one-time issuance, same-token narrowing, full pagination,
stale cursors/versions, foreign projects, original expiry, no-op updates,
last-explicit deletion, automatic deletion and credential revocation.
DynamoDB Local/race tests inject narrowing, deletion, revocation, owner disable,
generation changes and audit failure at the final release transaction.
HTTP tests verify strict bodies, ETags and 428/400/412 compatibility responses.
Browser tests cover separate future consent, empty accounts, conflict recovery,
one-time display and clearing late responses. The full Chromium→Go→DynamoDB
fixture suite and existing installed-Codex TLS-fixture compatibility passed.

`npm run verify` covers format/gofmt, ESLint, 98 frontend and 16 infrastructure
tests, tooling suites, TypeScript/Vite build and Go tests/vet. The new release
guard tests are included via `npm run test:mcp-scopes-release`.

The scoped release uses `scripts/mcp-scopes-release.mjs` with explicit
`preview`, `close`, `deploy`, `activate` operations. It verifies account/non-root
identity, existing bootstrap5, current schema10/MCP1, idle work and unchanged
accounting; reviews only three route integrations/permissions, existing Lambda
code/configuration and static assets. It closes the stage, uses supported SST,
compares exact packaged hashes/IAM/runtime/quota/concurrency and route presence,
then activates schema11/MCP2 while preserving all other capabilities and recovery.
No uncertain write is retried and no automatic reopen occurs.

The historical frontend-only preview rejects the newer OpenAPI tuple, so this
release uses its own exact current-tuple guards. Initial inventory incorrectly
treated retained reserved storage as unsettled; it was corrected before any
mutation to preserve and compare that accounting while blocking inflight work.
Shared Go dependency rebuilding also refreshes broker and controlled-endpoint
code packages; their interfaces, authority and configuration remain unchanged.

Hosted results and final merge are recorded here after execution. New scope
behavior has not yet been claimed as hosted verification in this preparation
record. Real elapsed thirty-day expiry, PITR, broad fault injection and every
browser engine remain outside this bounded campaign.
