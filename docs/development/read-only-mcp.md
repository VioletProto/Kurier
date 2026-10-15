# Read-only MCP: implementation, setup and deployment review

Davian accepted [contract `2d8d43c`](../architecture/read-only-mcp-contract.md),
including evidence-size and private-file tradeoffs, and authorized this scoped
implementation, contract synchronization, tests and AWS preview. **Deployment
and capability activation have not been performed and remain held for review.**
The current deployed API does not yet provide these MCP routes. There is no new
hosted frontend or remote HTTP MCP server in this slice.

## Implemented behavior

`services/mcp` is a foreground Go stdio executable using the official
`github.com/modelcontextprotocol/go-sdk` **v1.8.0**, pinned with checksums. It
supports MCP **2025-11-25** only. Both the official SDK client and the actual
installed **codex-cli 0.162.1** negotiated working connections. Supporting newer
protocol revisions requires another explicit compatibility test; the current
specification's modern transport is not implicitly enabled.

Seven static read tools: `list_projects`, `list_saved_requests`,
`list_executions`, `get_execution`, `list_workflow_runs`, `get_workflow_run`,
`get_execution_evidence`. Their strict input schemas and per-tool output schemas
are returned through discovery; runtime decoding also rejects unknown fields.
Catalog/status projections omit request configuration and secret descriptors.
There are no execution/rerun, workflow-edit, pin/unpin, project mutation,
credential-management or local-agent-control tools.

The hosted bridge checks the separate opaque capability and its current owner,
project scope, expiry/revocation and recovery generation. It reuses the existing
published evidence, run and retention readers. A read collector records the
strongly observed versions/state, then a bounded conditioned audit transaction
checks those observations before data release. A racing deletion, disabled
owner, revoked credential, unpin/expiry, recovery or audit failure denies data.
Read-time availability bookkeeping does not confer execution or pin authority.

Pagination uses authenticated encrypted continuation containing public IDs/time
positions, not exposed DynamoDB keys. Pages default/max 25, projects max 10 and
runs max 5, with earlier stops for the release-transaction budget. At most five
query pages/125 candidates are evaluated. Cursors bind the credential/version,
query and generation and expire after 24 hours. They confer no authority.

Credentials have one separate MCP slot/account, fixed scope of 1–10 owned
projects, thirty-day expiry and no refresh. Browser management initially issues
for the selected project; the API supports the bounded multi-project scope.
Project deletion removes scope through credential CAS, revokes the last scope
and drains project audits before deletion completion. Account credential,
lookup, issuance, audit and rate cleanup uses the existing minute maintenance
schedule; old cleanup cannot clear a replacement slot. Unknown records/fields
remain pending. No native TTL grants access or completes deletion.

`get_execution_evidence` defaults to an explicitly derived summary. Complete
capture requires **<=98,304 bytes (96 KiB)** and actual complete serialized MCP
response **<=262,144 bytes (256 KiB)**, counting both structured/text data,
escaping, framing and bounded request ID. Oversize produces
`evidence_too_large`; it never silently truncates or changes the immutable
capture. Summary identifies omitted fields and preserves capture omission
reasons, redaction flags, status/outcome/HTTP status/timing and live retention.
Pins do not restore expired siblings. No data or positive-authorization cache,
raw download link, body chunking, protected input or runtime-variable retrieval.

Limits include 16-KiB input, 8-KiB cursor, 60 invocations/minute/credential,
fixed ten-second burst bucket 10, 120/minute/account, 10 complete evidence
reads/minute/credential and 16-MiB/minute/account conservative returned-byte
accounting. One hosted request/process and at most four queued; authenticated
rate denials also receive an account audit. Pending stdout release tickets are
capped at 256; a client that abandons enough replies must reconnect/restart
the foreground process. These controls do not establish a
hard monetary ceiling for rejected traffic. Client status polling should be at
least ten seconds apart; the server does not run idle polls or automatically retry 401/404.

Release deadlines are bounded by current authority/retention and at most five
seconds. The client uses monotonic request elapsed time with a conservative
margin and enforces the deadline at actual stdout writes. Inherited stdout is
opened as a nonblocking duplicate so Go's poller can enforce backpressure
limits. SDK protocol errors are reduced to fixed messages without argument or
dependency-error echoes. JSON-RPC framing/control escaping, fixed descriptions
and closed tool/route allowlists keep upstream strings inert. Already delivered
client bytes/transcripts cannot be recalled by revocation.

## Pairing and Codex TUI setup

These commands are ready for use **after the reviewed API deployment and
activation**. Until then the live API will reject/miss MCP routes; do not use a
local-agent token or browser JWT as a substitute. React remains local and uses
the existing approved Cognito/development API configuration.

From `/home/king/Neumont/Kurier`, build a stable ignored local binary:

```sh
mkdir -p bin
go -C services/mcp build -o /home/king/Neumont/Kurier/bin/kurier-mcp .
```

Run the current local React application against the approved development API
(`npm run dev:cloud` with the repository's existing local configuration). Sign
in, select the owned project and open **Read-only MCP access**. Refresh MCP
status first to confirm that this API supports access and load any existing credential.
Creation stays disabled until this read succeeds.
If access is unavailable, token creation cannot proceed. On 2026-10-15 a
read-only inspection confirmed the hosted consumers were not MCP-capable and
there were zero MCP records. The earlier generic “Issuance may have succeeded”
message did not establish that a token existed; deployment and activation
remain required before cloud pairing. Review project name/ID and the
thirty-day read-only disclosure, check consent and choose **Create MCP
credential**. The capability is shown once in browser memory. Scope changes or
a lost token require **Revoke MCP access**, then explicit reissuance.

In a separate terminal, run:

```sh
/home/king/Neumont/Kurier/bin/kurier-mcp pair --api-origin https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com --persist
```

Enter the browser-issued token only at the hidden controlling-terminal prompt.
Pairing verifies scoped access before writing the file. Default private file:
`/home/king/.config/kurier-mcp/credential.json`, in a dedicated 0700 directory,
regular 0600 file. Storage checks descriptor-relative ancestors, ownership,
permissions, symlinks/hardlinks and bounded content; writes are atomic. Only
token and bound HTTPS API origin are stored. Same-user processes can read the
file; this accepted limitation is not an OS keyring guarantee. Windows
persistence/output remains unsupported and fails closed. There is no background
installer or auto-update.

Register the local command; no token goes into Codex configuration:

```sh
codex mcp add kurier -- /home/king/Neumont/Kurier/bin/kurier-mcp serve --credential-file /home/king/.config/kurier-mcp/credential.json
codex mcp list
codex
```

In the new TUI use `/mcp` to inspect the connection and its seven tools. Ask to
list scoped projects, find recent failed executions, inspect a run or retrieve
summary/complete evidence. Data in names/body text is untrusted, including
text that resembles instructions. A manually launched process in another
terminal does not attach to Codex's pipes.

Equivalent user-level `~/.codex/config.toml` entry:

```toml
[mcp_servers.kurier]
command = "/home/king/Neumont/Kurier/bin/kurier-mcp"
args = ["serve", "--credential-file", "/home/king/.config/kurier-mcp/credential.json"]
startup_timeout_sec = 10
tool_timeout_sec = 15
enabled_tools = ["list_projects", "list_saved_requests", "list_executions", "get_execution", "list_workflow_runs", "get_workflow_run", "get_execution_evidence"]
```

`serve --api-origin <trusted HTTPS origin> --prompt` is an optional memory-only
foreground mode requiring a real controlling terminal. Codex subprocesses
without a terminal should use explicitly persisted pairing. `pair` requires
`--persist`; it never reads credentials from protocol stdin. `codex mcp login`
is for OAuth-capable servers and does not implement this manual pairing flow.

stdout is protocol only. Do not use `2>&1`, log/tee protocol output, put tokens
in argv/environment/TOML/chat, or wrap startup in a verbose build command.
stderr contains only fixed safe diagnostics and can also be captured by Codex.
EOF/signals cancel foreground work. Expired/revoked/missing credentials require
browser reauthorization; restarts never enable offline/cached access. Browser
Revoke is authoritative; deleting the local file alone does not revoke a lost
copy. Close the token display after pairing and revoke disposable test access.

Official sources checked during implementation:
[Codex MCP](https://developers.openai.com/codex/mcp/),
[official Go SDK](https://github.com/modelcontextprotocol/go-sdk),
[MCP lifecycle 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle).
The current specification's
[version distinction](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning)
is why the supported revision is explicit. Local installed CLI help confirmed
`codex mcp add NAME -- COMMAND...`; configuration was not changed persistently
by automated integration tests.

## Validation and its boundaries

Completed checks, separated by evidence source:

| Evidence                          | Result and boundary                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Repository checks                 | Formatting/gofmt, ESLint, 56 frontend tests, 10 infrastructure tests and TypeScript/Vite production build passed. Go modules are covered by tests/vet/build, including the new seventh MCP module.                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| Official SDK/stdio tests          | Real OS pipes and controlled HTTPS fixture; initialization/version pin, seven-tool schemas/calls, invalid-input containment, revocation, EOF, actual-write expiry/oversize/error scrubbing, private-file permissions/symlinks/hardlinks/ancestor/size/atomic replacement. This is protocol simulation using the official SDK, not Codex or AWS.                                                                                                                                                                                                                                                                                            |
| Shared wire tests                 | Strict required/unknown/duplicate/null/trailing inputs and page enums/bounds; output allowlists and fully encoded structured/text escaping/size checks.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| DynamoDB Local/race               | Full existing API/execution/ownership race suite passed. Focused MCP checks cover one-time issuance/receipt/slot contention, credential separation, scope, generation/revocation/disable/deletion/audit races, run projection/expired sibling/pins, unavailable evidence with a surviving object, immutable evidence, large-capture summary/denial, cursor scope/continuation, aggregate rate and account/project cleanup. S3/KMS/network failures and clocks are controlled doubles.                                                                                                                                                      |
| Chromium integration              | Real Chromium→Go→DynamoDB Local, including MCP consent, one-time display/no Web Storage, scope denial, close/revoke, later 401 and project cleanup; prior browser fixtures also pass. Cognito is a signing/authentication fixture and there is no deployed IAM/S3/KMS validation.                                                                                                                                                                                                                                                                                                                                                          |
| **Actual installed Codex client** | `codex-cli 0.162.1` app-server launched the compiled foreground binary with SDK 1.8.0/MCP 2025-11-25, discovered and called all seven tools, handled summary/complete/oversize/scope denial and revocation with the connection open. A separate fresh compiled process, connected by the official SDK, also denied the persisted revoked credential. A malicious instruction-like project name stayed captured data. HTTPS API/auth/evidence are simulated with a private test CA. No model turn or AWS invocation, no persistent Codex config edit. This is genuine installed-client integration, not a hand-written protocol transcript. |
| Read-only AWS preview             | Actual identity/inventory/price-catalog reads and generated SST preview, described below. This proves current settings and planned diff only, not deployed MCP behavior.                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |

Reproducible commands:

```sh
npm run verify
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration
go -C services/mcp test -race ./...
KURIER_CODEX_INTEGRATION=1 go -C services/mcp test -tags=codex -run TestInstalledCodexMCPIntegration -v -count=1 .
```

Go tests/builds require a writable Go cache and permission for controlled local
listeners/processes. Sandbox-denied runs were rerun with approved permissions;
none of those denials is a product runtime failure. Browser recording is off,
credential assertions report booleans/counts and fixture credentials are
memory-only except deliberately private disposable CLI pairing files. Test
configuration uses a public fixture CA path, never a token literal in Codex TOML.

Unperformed: interactive operator browser pairing/hidden terminal prompt plus
manual `/mcp` inspection in the TUI; deployed Cognito/MCP HTTPS routing, IAM,
actual DDB/S3 integrity/revocation/audit/rate/cleanup behavior; elapsed thirty-day
expiry, real clock skew/PITR/recovery and metered billing. Fixtures and app-server
client integration do not establish those outcomes. Deployed checks must follow
separate deployment/activation approval and use disposable owner-scoped data.

## Scoped AWS deployment and activation review

Read-only inventory on 2026-10-15 verified non-root `davian-admin`, account
`747336059622`, us-east-2, existing SST bootstrap. Stage active/version 9,
`savedRequestsSchemaVersion=6`, workflow/local/pinning capabilities 1 and
`recoveryGeneration=dev-api-initial-v1`. Five projects; zero queued/running jobs,
unsettled uploads or MCP records at inspection. Current API/cleanup/worker
functions are Active/Successful and do not yet advertise MCP capability.

**Free/ACTIVE plan, account/unreserved concurrency 10, no function reservations,
standard SQS maximum 2/batch 1 and no provisioned pollers are unchanged.**
No Cognito/account-plan/quota/concurrency mutation was performed.

The guarded SST diff proposes only:

- Four routes on the existing HTTP API: owner POST/GET-current/DELETE MCP
  credential management and distinct bearer POST invocations; their
  integrations and Lambda invoke permissions target the existing API function.
- API and cleanup code/configuration updates, `KURIER_MCP_READ_ONLY_SCHEMA_VERSION=1`
  and saved-request readiness 7. Worker code/configuration is updated for shared
  record validators/capability compatibility. The shared controlled-endpoint
  binary asset is rebuilt through the existing worker packaging.
- Replacement deployment assets in the existing SST bootstrap asset bucket.
  No new table/index/bucket/key/queue/function/standing hosting, role-policy
  expansion, Cognito changes, frontend hosting or schedule/concurrency change.

The SST output's “Created/Updated/Deleted” lines describe **previewed** resource
operations, not deployed changes. The existing bootstrap was checked before
running diff, so this was not an implicit first bootstrap. Run the guarded
read-only previews again before a later approved deployment:

```sh
node scripts/preview-read-only-mcp.mjs
node scripts/preview-read-only-mcp-deployment.mjs
```

These scripts have no deploy/activate mode. The latter runs only SST diff after
identity/bootstrap/inventory guards. Do not run any existing activation script
as a substitute for an MCP activation procedure.

Proposed later activation is two conditioned marker transactions:

1. After approved deployment, verify exact hashes/runtime/configuration for all
   compatible API/cleanup/worker consumers, four route targets and retained
   minute schedule, scope of IAM and preserved plan/quota/concurrency. Strongly
   inventory the current marker and unresolved work; close active→mcp-migration
   with exact version/generation and existing capability conditions.
2. Recheck the closed-stage inventory and compatibility, then conditionally
   reopen with `savedRequestsSchemaVersion=7` and
   `mcpReadOnlySchemaVersion=1`, incrementing coordination version only.
   Preserve recovery generation, all admission/storage/pin/inflight accounting
   and all existing capabilities. No zero initialization or counter reset.

Old binaries reject schema 7; new code remains compatible with the pre-MCP
schema 6 stage but MCP issuance/reads fail closed until capability 1 is active.
Unknown/uncertain results leave the stage closed for deliberate reconciliation;
there is no automatic reopen/recovery or HTTP replay. Re-inventory rather than
assuming version 9 or idle work is still current. This review does not execute
those transactions.

## Refreshed incremental cost

[Public Ohio rates](read-only-mcp-prices.json) were read from AWS Price List
GetProducts on 2026-10-15, without account billing data. Lambda catalog published
2026-10-14, HTTP API 2026-10-13, DDB 2026-09-11, selected S3 rates 2026-10-15.
The record selects ordinary Standard/on-demand/ARM first-tier prices, not IA,
vector, replicated, managed-instance or S3 Table alternatives. No free-tier or
credit subtraction is applied; preserve the account's existing Free plan.

Assume 10,000 user-triggered reads/month, 1,000 evidence fetches, 250 GB-s
(256 MiB × 100 ms × 10k), 500k RRU/100k WRU including audit/rate guards, 0.1 GB additional
metadata/PITR/backup, 1,000 S3 GETs and a conservative 1 GB delivery allowance:

| Increment                         | Rate/assumption                                | Estimate/month             |
| --------------------------------- | ---------------------------------------------- | -------------------------- |
| HTTP API                          | $1/million                                     | $0.010                     |
| ARM Lambda requests+duration      | $0.20/million +$0.0000133334/GB-s              | $0.0053                    |
| DDB requests                      | $0.125/million RRU +$0.625/million WRU         | $0.125                     |
| Metadata/PITR/on-demand backup    | $0.25+$0.20+$0.10 per GB-month                 | $0.055                     |
| Existing evidence S3 GETs         | $0.0004/1,000                                  | $0.0004                    |
| Delivery                          | $0.09/GB planning assumption                   | $0.090                     |
| **Subtotal / planning allowance** | Safe logs, retries, cleanup rounded separately | **about $0.29 / $0.50–$1** |

Delivery uses the existing regional planning assumption and official
[API Gateway pricing example](https://aws.amazon.com/api-gateway/pricing/), not
an independently retrieved transfer SKU. No additional KMS operation/key or
retained evidence copy/standing compute is introduced. There is no idle server
polling bill. Larger pages, rare large metadata, rejects, retries and sustained
reads can exceed the allowance; local tests do not measure AWS duration/request
rounding/traffic. This is an incremental forecast, not a measured bill, new
whole-system estimate or spending cap.

## Contract and coordination record

The native API Contracts document was updated in place under a revision guard
with **Accepted read-only MCP continuation (2d8d43c)**. Readback confirmed the
new authority/routes/DTOs/limits/SDK pin/credential/cleanup section and preserved
all original text. Earlier unrelated Proposed interfaces and the capstone
proposal remain unchanged. [Trello](https://trello.com/c/9UBvPhBB) distinguishes
acceptance, implementation/publication, client/local tests and the still-open
AWS deployment/runtime checks. Original workflow operational follow-ups and the
broader contract-review card remain open.
