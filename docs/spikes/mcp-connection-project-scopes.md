# One-connection MCP onboarding and project scope modes

Prepared 2026-11-14 for [the design spike](https://trello.com/c/mgcmRSIa).
Status: **research complete; recommended design proposed for Davian's review**.
The [delivery card](https://trello.com/c/RaD7xqgS) remains implementation work.
This packet does not amend the accepted runtime contract until accepted.

Continuation — 2026-11-14: Davian requested implementation, deployment, testing
and merge of this recommendation, accepting the scope modes, in-place changes
and release fence described here. The [delivery record](../development/mcp-project-scopes.md)
tracks implementation and actual verification separately from this historical
research. Connection metadata adds an `editable:boolean` compatibility signal
so the browser can distinguish fixed schema-1 scope from editable schema 2.

## Recommendation and evidence

Keep the foreground Go stdio bridge, one client registration and one active
read-only credential per account. Add two browser-managed scope modes:
`selected_projects` (1–10 explicit IDs) and `all_owned_projects` (all eligible
current and future owned projects). Default to selected projects; require
separate explicit consent for future projects. Support in-place scope changes
under credential version CAS, keeping the same token and original expiry.
Do not add exclusions, independent per-client credentials, remote MCP/OAuth,
automatic installation or local-agent authority in this slice.

“One connection” means one registered server can discover all allowed projects.
It does not mean perpetual authorization: thirty-day expiry, revocation, recovery
generation changes and lost tokens still require deliberate reauthorization.
Scope changes need no client restart or pairing; token replacement requires
pairing and restarting each process that holds the old token, with the same
client registration. Previously returned data cannot be recalled.

Repository findings at base `718c5f9653fe254f805ba6ca34f1231d72db7e52`
come from the following source inspection, not hosted inventory:

| Source                                                      | Finding and consequence                                                                                                                                                                                                                                        |
| ----------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `services/mcp/main.go`, `client.go`, `go.mod`               | SDK 1.8.0, MCP 2025-11-25, static seven-tool catalog; every call reaches the hosted API. Pairing already uses `list_projects`, so an empty automatic list must count as successful pairing. No scope cache needs to be added.                                  |
| `services/api/execution/mcp_credentials.go`                 | One `MCP#CURRENT` slot; schema-1 credentials require 1–10 IDs, hashed proof, thirty-day expiry, seven-day issuance receipts and explicit project links. Automatic mode needs a distinct validated record shape.                                                |
| `services/api/execution/mcp_read.go`                        | Authorization uses explicit membership; project discovery walks the stored array. Other collections already use bounded candidate discovery, hydration and conditioned audit. Encrypted cursors bind token/version/generation but need explicit scope binding. |
| `services/api/internal/ownership/store.go`, `pagination.go` | Existing owner project GSI1 uses `U#<owner>#PROJECT`, immutable creation ordering and public project IDs. It can discover automatic-mode candidates, but its browser cursor exposes internal keys and must not be reused for MCP.                              |
| `services/api/execution/mcp_cleanup.go`                     | Project deletion removes explicit links and revokes the last remaining scope. Automatic access must survive deletion of all projects, with no per-project wildcard links to drain.                                                                             |
| `apps/web/src/MCPCredentials.tsx`                           | UI currently authorizes only the selected workspace project. It must become account-level connection management, accessible when the owner has zero projects.                                                                                                  |

The native proposal and API Contracts were read, including their 2026-11-11
planning continuations. Accepted [MCP](../architecture/read-only-mcp-contract.md),
[OpenAPI](../architecture/openapi-contract.md),
[local-agent](../architecture/local-agent-execution-contract.md) boundaries and
[ADR 0002](../decisions/0002-serverless-persistence-topology.md) remain authoritative.
OpenAPI evidence already exists, but exposing its stored results is a separate
[tool-extension card](https://trello.com/c/jQR3w1Zt), not a dependency for scopes.

Official [MCP stdio transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)
and [Codex configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli)
support retaining a command-based server. Installed `codex-cli 0.169.0` help
confirmed `codex mcp add NAME -- COMMAND...` on 2026-11-14. This is evidence for
configuration availability; a hosted MCP endpoint or OAuth provider is unnecessary
for project discovery. The existing SDK/protocol pin remains unchanged.

| Alternative                                          | Decision                                                                                                                                                                                   |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Keep immutable scopes; revoke/reissue for every edit | Simplest backend, but repeated token transfer and restarting all clients makes routine selection changes awkward. Retain this path for legacy credentials only.                            |
| Mutable scope on the existing credential             | Recommended. Version-guarded reads already provide the necessary fence; add exact scope validation and atomic link updates.                                                                |
| Materialize every owned project into the credential  | Reject: ten-project/transaction growth, project-creation races and repeated fan-out writes. Evaluate current ownership per read instead.                                                   |
| Remote Streamable HTTP with OAuth                    | Adds authorization-server, callback, client compatibility and deployment scope without solving a transport constraint. Reconsider separately for clients unable to launch local processes. |
| Per-client credentials or all-owned exclusions       | Useful future controls, but expand slots, lifecycle and cleanup. Preserve one shared authority and use manual selection for narrower exposure.                                             |

## Exact browser flow and consent

Add account-level **MCP connection** settings, reachable from project and empty
account views. Refresh safe status before enabling issuance or changes. Show
mode, selected names and IDs where applicable, expiry and shared-client effects.
Offer radio choices with these labels and explanations:

- **Choose projects**: “Allow only the projects selected below (up to 10).
  Projects you create later stay private until you select them.” Require 1–10
  distinct active owned IDs; show an explicit selection count.
- **All my projects, including future projects**: “Allow read-only access to
  every active project you own now and every project you create while this
  credential remains valid. There are no project exclusions in this mode.”
  Zero current projects is valid. Never present a checked list as its authority.

Both require an unchecked confirmation: “I authorize my coding clients to read
sanitized project and execution information for this scope until [expiry].
Signing out does not revoke this access. Information already delivered cannot
be recalled.” Automatic mode additionally requires: “I understand that newly
created projects will become available automatically.” Switching modes or
changing selected IDs clears consent. Changing to automatic mode always requires
fresh future-project consent, including after a prior automatic grant.

Use **Create MCP credential** for issuance and **Save project access** for a
scope update. Show a before/after scope summary before saving; explain that all
processes sharing the credential change together and that expiry is unchanged.
Offer **Revoke MCP access** independently. Revocation is the way to stop all
access; an empty manual set is invalid. Do not claim browser sign-out, deleting
a local file or removing a client registration revokes a copied token.

After issuance, show the token once in browser memory and guide the existing
hidden-prompt `kurier-mcp pair --api-origin <trusted-origin> --persist` flow.
Show the executable/credential-file registration command once, without a token.
Check an existing registration before suggesting replacement. The client starts
`serve --credential-file <private-file>`; do not start a second server manually.
Use `list_projects` to verify access, explaining pagination and propagation delay.
New projects appear on a fresh discovery call after owner-index propagation;
there is no push subscription or background discovery loop.

Retain private POSIX 0700 directory/0600 regular-file persistence, HTTPS origin
validation, no redirects/proxies, safe stderr and protocol-only stdout. No secret
in URLs, argv, environment, TOML, Web Storage, chat, logs or checked-in files.
Setup remains manual at first pairing. Command-free agent installation and
desktop packaging belong to their own spikes; Windows persistence remains
unsupported. No MCP scope grants execution, pairing or destination authority.

## Proposed management interfaces and storage

Introduce separate connection-management routes so old strict DTOs never
misrepresent automatic access as an empty explicit project set. Cognito owner
authentication, enabled-user/stage checks, existing cookie/CSRF policy when
applicable, strict duplicate/unknown/null rejection and 16-KiB bodies apply.
MCP bearers and local-agent credentials cannot call these routes.

Scope is a strict discriminated union:

```ts
type Scope =
  | { mode: "selected_projects"; projectIds: string[] }
  | { mode: "all_owned_projects" };
type Connection = {
  tokenId: string;
  version: number;
  scope: Scope;
  createdAt: string;
  expiresAt: string;
  revokedAt: string | null;
};
```

IDs are distinct canonical UUIDv4 values; sort before comparison/digest/storage.
Reject `projectIds` for automatic mode, unknown modes, exclusions and wildcard
strings. Version is a nonnegative safe integer, incremented on every actual
scope change, deletion-driven manual narrowing and revocation. Never expose
verifiers, recovery coordinates or internal storage keys.

| Route                                           | Exact behavior                                                                                                                                                                                                                                                                                                                                                                    |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /api/v1/mcp/connections`                  | Strict `{scope}` plus one UUID Idempotency-Key. Existing account slot and five issuance attempts/hour apply across old/new routes. 201 `{data:{connection,token}}`; same canonical input/key retry gives 200 metadata with `token:null`; changed input/key reuse or occupied live slot gives 409 `conflict`. Receipt namespace is distinct from legacy issuance.                  |
| `GET /api/v1/mcp/connections/current`           | 200 `{data:{connection:null\|Connection,serverTime}}`. Projects in `scope` are IDs only; names come from owner project discovery. Legacy records project as selected mode with their stored version. No token redisplay.                                                                                                                                                          |
| `PATCH /api/v1/mcp/connections/{tokenId}/scope` | Strict `{scope}` and required strong `If-Match: "<version>"`. 428 `precondition_required` if absent; malformed header/body 400 `invalid_request`; stale version 412 `precondition_failed`; foreign/missing token 404 `not_found`; expired/revoked/legacy credential 409 `conflict`. Success 200 `{data:{connection,serverTime}}` plus new ETag. No new token or expiry extension. |
| `DELETE /api/v1/mcp/tokens/{tokenId}`           | Existing owner-only idempotent revocation works for either record version. Keep its DTO/status unchanged.                                                                                                                                                                                                                                                                         |
| `POST /api/v1/mcp/invocations`                  | Existing distinct MCP bearer, seven tool names, schemas and safe response envelopes remain unchanged. The hosted service resolves scope per invocation.                                                                                                                                                                                                                           |

GET-current and issuance expose ETag for a non-null connection. PATCH compares
the observed version first, then no-ops identical canonical scope without a
version increment. For a lost PATCH response, refresh current status and compare
the whole scope; only deliberately retry with the freshly observed version.
Never automatically overwrite a concurrent tab's change. A PATCH cannot revive
an expired/revoked credential. Explicit legacy upgrade is revoke, new issuance,
pair and restart using the existing registration.

Add schema-2 `mcpCredential` records with `mcpScopeMode` and, only for selected
mode, `mcpProjectIds`. Keep purpose/proof lookup, owner, slot, generation and
original thirty-day expiry. The row's existing `version` is the authoritative
scope fence. Schema-1 records retain immutable explicit scope and existing
semantics. Add strict validators for each shape; unknown fields/schema fail
closed. Existing lookup/slot/audit/rate shapes remain at schema 1.

Selected mode creates existing per-project scope links. All-owned mode creates
none. In-place changes atomically condition the exact credential version,
active stage/generation, enabled owner and all newly selected project gates,
update the credential, and add/remove only the set difference of links.
Selected→all removes up to ten links; all→selected adds up to ten. Manual
replacement can remove ten/add ten/condition ten projects plus fixed guards,
comfortably below 80 actions, subject to measured encoded 2-MiB transaction and
128-KiB item limits. Any overrun rejects before writes. A concurrent delete,
revoke, cleanup or scope update loses CAS and cannot partially change scope.
Store a safe account audit of the management change in that transaction with
fixed action/outcome and token/version/mode only; no token, names or raw input.

Legacy POST `/mcp/tokens` remains explicit-only and issues schema 1. Legacy GET
`/mcp/tokens/current` retains its exact response for schema 1 or no record; when
a schema-2 record occupies the slot, return fixed 409 `conflict` directing the
human to updated connection settings. Never return a false narrow DTO. Old
browser UI safely disables issuance on error; legacy DELETE remains usable.
The updated UI uses new routes for both record types and labels legacy scope
as fixed with a deliberate upgrade action. An old CLI bridge can read a new
credential because pairing/invocation DTOs and the ten-item page limit stay
unchanged; it has no credential-metadata dependency.

## Per-read authority and project discovery

Every call strongly authenticates purpose/proof, owner enablement, active stage,
generation, expiry and credential state/version. Selected mode requires explicit
ID membership; all-owned mode requires current ownership and active project
state. Neither mode trusts a supplied owner ID, project name, GSI membership,
cursor or previous successful connection. Foreign/out-of-scope/deleting/missing
locators return fixed 404 without revealing which check failed.

Every row affecting returned data must participate in the existing final
conditioned audit transaction, including credential version and the project
owner/state/version/deletion epoch. Preserve expiry/retention checks and release
deadline at actual stdio output. If narrowing/deletion wins before audit commit,
discard and recompute within the existing three-attempt cap. Subsequent reads
deny removed projects. A result authorized before the change may still arrive
within its bounded release deadline; already delivered evidence cannot be erased.

Manual discovery continues over the canonical ID array. Automatic discovery
queries the existing owner-project GSI1 newest first with a fixed initial
`createdAt#projectId` high-water. Strongly hydrate each candidate and release only
active owned project summaries under the existing allowlist. A vanished or
deleting candidate is skipped; malformed data or infrastructure errors fail
closed. Do not call the browser list endpoint with an MCP credential or forward
its signed internal-key cursor.

Keep `list_projects` default/max **10 per page**, with no total or offset.
Automatic mode has no credential-wide project count limit and does not raise
project-creation quotas. Retain at most five underlying query pages/125 examined
candidates, stopping earlier for the read-set limit (72), audit transaction
(80 actions/2 MiB), 128-KiB item, 256-KiB encoded MCP response or eight-second
hosted deadline. Reserve fixed guard/audit overhead before hydrating another
candidate. Never hydrate unbounded skipped rows. A continuation points after
the last fully processed candidate, not the fetched page's last key; a full
response or guard stop must not skip unprocessed candidates. Empty pages may
have a cursor; only null ends a traversal.

Use a versioned authenticated-encrypted MCP cursor, <=8 KiB and <=24 hours,
bound to owner, stage, generation, credential ID/version, canonical scope digest,
tool, project, filters, high-water and last processed public creation-time/ID
position (or manual-array position). Reconstruct database keys server-side.
No internal locations in client-decodable content. Keep the existing cursor
decoder for legacy credentials during compatibility support; never accept an
old unbound cursor for a schema-2 credential. Any scope edit or deletion-driven
manual narrowing invalidates all prior cursors for that credential, including
child collections: fixed 400 `invalid_request`, restart without a cursor.

Newer projects are outside an in-progress high-water traversal; restart
`list_projects` without a cursor to discover them. GSI lag can omit a candidate
even below the high-water; this is eventual discovery, not a snapshot or complete
instantaneous inventory. Refresh from the beginning after propagation if needed.
Direct reads still require current authoritative project ownership. No polling
loop or automatic all-project evidence crawl is added.

Preserve existing aggregate 60/minute/credential, burst 10, 120/minute/account,
complete-evidence and byte budgets, 16-KiB inputs, 96-KiB complete capture,
256-KiB encoded responses, process queue/deadline limits and audit-before-data.
Additional discovery pages consume the same budget; the old incremental cost
forecast is not refreshed pricing or a measured bill for this feature.

## Deletion, revocation, recovery and rollout

Manual project deletion denies immediately through its project gate, then
bounded cleanup removes the link and increments credential version. Removing
the last explicit project revokes the credential. Same-name recreation never
restores explicit access. Automatic deletion uses the same immediate gate but
has no scope link to drain, does not revoke the credential or bump its version,
and permits a future new owned project under the original future-project grant.
Account cleanup still removes the single slot, lookup, credential and account
audit/rate/receipts. Recognize both credential schemas; preserve unknown rows.
Stale cleanup must condition the observed version and cannot remove replacement
links or a replacement slot.

Expiry, explicit revocation, disabled user and generation changes deny both
modes; no refresh/offline cache. Restored access requires owner reauthorization
in the new generation. Preserve reconciliation and no automatic HTTP replay.
No schema fetching, runtime validation or target request occurs on an MCP read.

Propose a coordinated `mcpReadOnlySchemaVersion=2` plus
`savedRequestsSchemaVersion=11` release fence from the current OpenAPI schema-10
baseline. Preserve every other active capability, recovery generation and all
accounting; re-inventory at implementation time rather than assuming a live
version. Update all API/cleanup/worker/local grant validators, known-row cleanup
and stage guards before activation. The schema-11 fence makes old consumers
reject the stage instead of misreading schema-2 credentials. This number is a
proposal to reserve in delivery, not a marker write authorized by this spike.

Local implementation/fixtures come first after design acceptance. Deployment,
marker migration and genuine AWS/Cognito/browser acceptance require the existing
scoped review workflow. Inspect exact code/assets, route additions, CORS
PATCH/If-Match/ETag exposure, IAM/bootstrap effects and current work before any
approved rollout. No new table/index/standing service is expected. Close the
stage under current guards, deploy compatible consumers, activate the tuple and
reopen only after verification. Unknown results leave it closed for reconciliation.

Rollback after schema-2 issuance cannot lower markers while old readers/cleanup
would encounter new records. Prefer compatible code rollback with capability 2;
otherwise deliberately revoke and completely drain schema-2 state under the
closed-stage procedure before a reviewed downgrade. Never reinterpret automatic
credentials as manual or silently broaden legacy tokens.

## Delivery and verification plan

| Gate                      | Required evidence                                                                                                                                                                                                                                                                                                                                    |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DTO/unit                  | Both strict unions; 0/1/10/11 selections; duplicates/foreign IDs; no automatic exclusions; one-time issuance and cross-route slot/rate conflicts; original expiry; lost issuance/PATCH response; stale/missing/malformed If-Match; no-op and concurrent edits.                                                                                       |
| DynamoDB Local/race       | Automatic zero/>10 projects; bounded GSI discovery/lag, tied timestamps, skipped/deleted candidates, empty continuation, no skipped unprocessed items; scope edit/delete/revoke/disable/generation races before fetch and final audit; audit failure releases no data; link/last-project/account cleanup and stale replacement fencing.              |
| Cursor/limits             | Tamper/expiry/wrong owner/token/generation/tool/filter/scope/version; legacy cursor only for legacy token; new project during traversal/fresh restart; exact/one-over action, read-set, item, input/output/escaping and byte/rate/deadline bounds across processes.                                                                                  |
| Browser fixture           | Account-level setup with no projects, default manual, distinct unchecked future consent, clearing consent on edits, mode changes and before/after review, original expiry, conflicts/recovery, no Web Storage/token logging, independent revoke and sign-out disclosure.                                                                             |
| SDK/installed client      | Existing seven tools and DTOs on old/new bridge binaries; automatic >10 pages, new project discovery, narrowing with connection open, stale cursor then restart, simultaneous processes share scope/rates, replacement/restart, foreign IDs, hostile names, safe summary/complete redaction and revoke denial. Record versions and fixture boundary. |
| Hosted, separately scoped | Real owner identities/projects, GSI propagation, IAM/CORS routes, durable audits, browser consent and live Codex narrowing/revocation. Record which checks are real versus fixture; no fault/PITR or elapsed-expiry claim without separate evidence.                                                                                                 |

Estimated delivery: **28–44 active hours**, excluding waiting for acceptance and
real elapsed expiry: backend/cleanup/compatibility 10–16; browser management/setup
5–8; race/cursor/client coverage 8–12; documentation/release preparation 3–5;
bounded hosted checks 2–3. These are estimates, not logged hours or a new deadline.
Prioritize the two modes, live narrowing and compatibility; defer exclusions,
multiple credential slots, automatic install and remote transport.

Accepting this recommendation should explicitly accept the union/limits,
in-place shared scope updates and unchanged expiry, future-project consent with
no exclusions, versioned management compatibility, eventual discovery and
release/rollback fence. The spike can finish with that decision proposed; the
delivery card's design-acceptance item remains open until Davian accepts it.
The stored-contract-results card owns new tools/DTOs and safe validation states;
it must reuse these per-read scope/audit gates if implemented afterward.

## Verification performed for this spike

This change adds research and planning links only. Source inspection and installed
CLI help are complete. `npm run verify` passed: Prettier/gofmt, ESLint, 96 frontend
tests, 16 infrastructure tests, workflow/tooling/environment-release suites,
TypeScript/Vite production build and all nine Go modules' tests/vet (plus the
two spike test-binary builds). The environment activator's optional DynamoDB
Local integration case was skipped; no new database integration is claimed.

`KURIER_CODEX_INTEGRATION=1 go -C services/mcp test -tags=codex
-run TestInstalledCodexMCPIntegration -v -count=1 .` passed against a controlled
TLS API with seven-tool discovery/calls, scope/size/redaction and revocation
behavior. `codex --version` independently confirmed **0.169.0** in the test
environment; the harness's fixed success label still says 0.162.1 and is not a
version measurement. SDK 1.8.0/protocol 2025-11-25 remain pinned. No model turn,
persistent client configuration change or AWS invocation occurred.

Initial sandbox runs could not write the Go cache or spawn the release test's
Git subprocess (`EPERM`). The required checks and installed-client test passed
when rerun with approved permissions. New modes, scope mutation, their race
matrix and hosted behavior remain unimplemented and untested.

The native proposal and API Contracts received revision-guarded proposed
continuations and were read back. Their prior accepted sections remain in place;
neither continuation labels the new design accepted. Repository contract/index
and planning links point here. The spike checklist tracks completed research;
delivery acceptance/implementation and stored-contract-result checks remain open.
