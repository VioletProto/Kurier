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

## Hosted deployment and live checks

The [sanitized deployment proof](mcp-project-scopes-deployment.json) records
package hashes, preserved controls, live checks and independent cleanup readback.

Implementation `494ee58db0bae72fb9ffdb6cd2bf3ede8b996da4` is deployed on
`dev-api`, account `747336059622`, us-east-2, after verifying non-root
`davian-admin`. Marker version19/schema10/MCP1 was conditionally closed at
version20, then reopened at **version21/schema11/MCP2**, retaining
`dev-api-initial-v1`, every other capability and all accounting. Existing
bootstrap5, role IAM hashes, runtime configuration, Free/ACTIVE plan, account
and unreserved concurrency10, worker maximum2/batch1 and minute schedule were
verified unchanged. API/cleanup/worker, broker and controlled-endpoint package
hashes matched the local deployment ZIPs.

The first deployment stopped after code/static updates when a third duplicate
Lambda permission would have exceeded the 20-KiB resource-policy limit. No
activation occurred. The corrected source reuses the existing API integration
and API-scoped invoke grant, following the established OpenAPI route pattern.
An explicit `preview-closed` reconciled the partial state; supported SST then
removed only this attempt's two permissions/integrations and three components,
and created the three raw API routes. The invoke policy's original source ARN
was preserved; no policy-size/quota increase or broader runtime grant was used.
Marker/accounting/readiness checks preceded activation.

The bounded live campaign uses two disposable real Cognito SRP owners and three
empty projects, with no target HTTP execution or model turn. Completed gates:

- Real hosted Chromium sign-in, automatic access from an empty account,
  separate future-project consent, one-time display, no token in Web Storage
  and browser sign-out before closing the context.
- Installed Codex 0.169.0 app-server launching the compiled foreground bridge,
  discovering seven tools and successfully reading an empty automatic account.
- The same live connection discovering a newly created project, paging,
  narrowing to manual scope without restarting, rejecting an old cursor and
  denying another project and a foreign owner's project.
- Returning to automatic scope without extending expiry, immediate denial of
  a deleting project and subsequent denial after credential revocation.

The initial browser navigation waited for network idleness and timed out before
any credential issuance. A signed-out DOM probe succeeded; the harness resumed
the same identified fixtures using DOM/control readiness. No credential issuance
or project creation was blindly replayed. Passwords/JWTs remain memory-only;
the disposable MCP token uses a private temporary pairing file that is removed
when the client exits. No screenshots, traces, token-bearing state or protocol
transcripts are retained. The old fixture harness's fixed Codex0.162.1 success
label remains historical; the installed CLI version was independently checked.

All three projects completed scheduled deletion, with deleted tombstones and
empty Protected partitions. The revoked MCP lookup/credential/slot, fixture
account audits/rates/receipts, user/identity mappings and both Cognito users
were removed; any quota row was preserved without resetting usage. The teardown
helper initially conditioned rate rows on `ownerId`, which those partition-scoped
rows intentionally lack. It stopped safely, then resumed only fixture cleanup
with exact partition/kind/version conditions; no product change or test replay
was needed. Identifier-only receipts remain in ignored `.sst` output.

All 12 GitHub CI jobs passed for deployed source `494ee58`; final integration
is tracked in [PR #23](https://github.com/VioletProto/Kurier/pull/23).
Real elapsed thirty-day expiry, PITR, broad fault injection, cross-engine browser
coverage and maximum-size cloud load remain unperformed. More-than-ten-project
discovery and injected races were verified with DynamoDB Local, not represented
as independently exercised hosted faults or load.
