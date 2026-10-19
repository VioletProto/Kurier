# Development/demo frontend deployment and validation

This is the historical deployment validation. Davian subsequently authorized the
[2026-10-19 live account/data reset](development-data-reset.md); prior six-project
accounting fingerprints and version 11 describe the deployment baseline, not
the reset state. The reset leaves the host/configuration intact and advances
the maintenance-fenced stage to version 13 without schema activation.

Davian approved the deployment documented in [the hosting guide](frontend-hosting.md),
including `21c24be`, the refreshed preview and a harmless public-marker rollback
exercise. The development/demo URL is **https://d2kpq905qyh9zg.cloudfront.net**.
This approval does not cover a general-public signup launch, custom DNS/domain,
SES sender setup, account-plan upgrades or schema activation.

## Scope and evidence

The predeployment plan matched the reviewed 16 creates, four deployment ZIP
replacements and four existing Lambda updates. Only the API environment adds
`KURIER_HOSTED_FRONTEND_ORIGIN`; the local origin remains
`http://localhost:5173`. The existing Ohio bootstrap version 5 was inspected and
reused. No bootstrap infrastructure was created/replaced. The live stage remains
version 11/schema 7/MCP 1 with generation `dev-api-initial-v1`.

Generated distribution: `EMNTWD8Q14ZLM`; private web bucket:
`kurier-dev-api-web-fc2927a49a0b246240e5a17774`. The generated cache/header policy,
Function and KV identifiers, deferred providers, completed invalidations,
public file hashes, before/after preservation fingerprints and recovery result
are recorded in `frontend-hosting-deployment.json` alongside this guide.
Raw state, complete function environments, credentials and provider serialization
remain excluded from committed evidence.

The initial unknown hashed-file probe exposed SST's placeholder-origin custom
error-page failure (502). A narrowly previewed correction updated only the
existing routing Function and existing KV entries. Unknown hashed files now
return a no-store 404 before origin access; current/historical hashed files have
retained per-file keys. No resources, runtime IAM or API configuration were
added by that correction. See the [implementation explanation](frontend-hosting.md#live-missing-asset-correction).

## Genuine checks and their limits

The live HTTP suite checks anonymous S3 object/list denial, successful signed
CDN asset reads, exact app byte counts/SHA-256/content types/cache metadata,
HTTP→HTTPS redirect, CSP/security headers, root/deep-link shell delivery, and
missing file/hashed asset/API/archive 404s. It probes both exact permitted API
origins, all five methods and four required request headers, exposed
ETag/Location/Retry-After, readable no-store 401s and foreign/null/suffix origin
rejection. No JWT or cookie credentials are supplied in these probes.

Genuine hosted Chromium checks signed-out rendering/deep links, enforced CSP,
a real cross-origin readable API 401, keyboard/mobile smoke behavior and empty
localStorage/sessionStorage/IndexedDB/Cache Storage. These are not authenticated
product or email results. Actual ETag/Location values on authenticated create/read
responses remain participant checks; exposure policy alone is already tested.

The rollback exercise uses public comment markers in the shell/startup/404 and
a harmless comment in a separately hashed JS file. B has the same public API,
pool/client/stage values and resource scope. Every manifest hash is checked at
A, B and restored A. A's entire selectable local archive is moved away before
recovery; the restore helper reads only private-origin release objects plus
shared hashed assets and verifies every size/hash before publishing a local
rollback candidate. Rollback selects those bytes without rebuilding.

A browser `fetch()` probe for a static script was correctly blocked by the
reviewed `connect-src` policy. The exercise resumed from the confirmed B release
using a script preload allowed by `script-src 'self'`; CSP was not relaxed.
Retained A assets are checked during B and retained B assets after rollback.
An uninterrupted old tab across all deployment attempts is not claimed.
The final distribution readback is `Deployed`; all three wildcard invalidations
are `Completed`. The restored A release ID is
`acb951896da16807815751c0509b1f25647656d8a0963932c7fc1f3a8ccac67e`; B is
`734b353c1b67b348389e53e1e9e9ac2b0a99ed3cb58545b870c49352f47b6963`.
Before/after verification passed exact built Lambda packages, unchanged full
runtime IAM policies/trust, unchanged configuration except the API hosted origin,
and unchanged capabilities, generation, quota and the combined fingerprint of
all six project accounting projections. Free/ACTIVE and account/unreserved concurrency ten, worker max two/
batch one/standard pollers and minute maintenance are preserved. No activation
was issued.

No disposable Cognito accounts, projects, credentials, requests, executions,
workflow runs, pins or evidence were created by these checks. The temporary local
public-archive backup is removed after successful rollback. The public B archive
and hashed chunk are intentionally retained under the accepted 30-day/current/
previous policy, not treated as disposable product data. No cleanup worker,
schedule, accounting counter, capability or generation is activated/reset.
Participant data created below still needs its own cleanup/readback. Final web
inventory is 15 public-only objects / 816,817 bytes, including both complete
private release archives; the retention planner returned no deletion or KV
removal candidates. Final `npm run verify` passed formatting/gofmt, lint,
58 frontend tests, 15 infrastructure tests, TypeScript/Vite build and tests/vet
for seven Go modules. These local checks remain distinct from genuine hosted
HTTP and signed-out Chromium checks.

## Participant browser checks

**All numbered steps require Davian's participation**: a controlled inbox and
real account, local/private service consent, or a Codex session on his machine.
Use disposable nonproduction values. Enter passwords/codes/tokens only in the
UI or hidden CLI prompt; do not paste them into chat, screenshots, HAR files,
logs, command arguments or environment variables. Report statuses and safe IDs.

1. **Signup/email/sign-in:** open the HTTPS host in a clean browser profile.
   Create a development test account in an inbox you control. Exercise verification,
   incorrect/expired-code rejection and deliberate resend, then sign in with
   genuine Cognito SRP. Confirm the owned workspace loads. Default Cognito mail
   is the accepted development-only sender exception; delivery/quotas are not
   evidence of launch readiness.
2. **Reset, refresh and sign-out:** reset the test password through the inbox and
   sign in with the new password. Keep the same page open past the 15-minute
   access-token lifetime; trigger an authenticated read and confirm automatic
   rotating refresh succeeds. Repeat later to exercise the replacement refresh
   token. Do not export token-bearing network records. Sign out, confirm later
   reads require sign-in, and test Back/Forward: previous workspace/secrets must
   stay cleared. Reload intentionally ends the memory-only session. One-day
   refresh expiry and elapsed code expiry need separate elapsed-time observation.
3. **Existing product features:** in a disposable project, verify user/project
   list/detail/rename, saved-request create/edit/revision/history, protected-value
   editing/clear behavior, cloud execution/evidence/redaction, rerun consent,
   pin/unpin and a small sequential workflow. Use the existing controlled HTTPS
   fixture or an explicitly controlled target. Check readable errors/conflicts,
   real ETag/Location headers, pagination, loading/empty/error states, cancellation
   and uncertain outcomes without replay. Confirm secrets never reappear in
   storage/history/evidence. Repeat keyboard/focus/mobile checks while signed in.
4. **Local agent:** follow [the setup and target checks](local-agent-execution.md)
   using the hosted UI. Build/run the foreground agent, issue a project-scoped
   credential, enter it at the hidden prompt and approve the exact loopback origin.
   Submit once with fresh execution consent; verify sanitized evidence and target
   count 1, including after history/receipt refresh. Repeat on a controlled private
   address if available. Revoke in the hosted UI and confirm later use is denied.
   The browser never contacts a localhost control endpoint.
5. **Codex MCP:** follow [the stdio setup](read-only-mcp.md#pairing-and-codex-tui-setup)
   from the hosted UI's **Read-only MCP access** panel. Pair the separate token
   at the hidden prompt, then let a fresh Codex TUI launch `serve`; do not start
   another standalone server. Confirm seven read tools, authorized project/
   execution/workflow evidence, preserved redaction and immediate denial after
   hosted-UI revocation. No hosted MCP endpoint is introduced.
6. **Cleanup and reconciliation:** revoke disposable agent/MCP credentials,
   unpair local files, unpin disposable captures, delete the disposable project
   and wait for deletion completion/list disappearance and any scheduled drains.
   Stop local targets/agents. Keep safe project IDs for a read-only server cleanup
   check; request test-account cleanup only for an explicitly disposable account.
   Do not delete an existing personal account. Report any pending cleanup.
7. **Additional browsers/accessibility:** repeat critical sign-in, session/history,
   errors, keyboard/focus and product smoke paths in the intended Vivaldi/Firefox/
   WebKit environments. Chromium signed-out smoke is not full accessibility or
   cross-engine certification.

## Costs and remaining gaps

Accepted allowances remain $0.25–$2/month hosting and $1–$3 validation, not caps.
The artifact remains about 0.4 MB; marker B adds about one more JS chunk and its
small private archive. The correction adds one KV read per hashed-file request.
There is no new standing compute, paid networking, sender, domain or customer
key. AWS Free plan, quotas and concurrency are preserved. Actual account-wide
allowances, metered charges and billing ceilings have not been verified.

The participant checks above, broader abuse/load testing, arbitrary negative
runtime IAM probes and elapsed email/refresh expiry remain open. This record
separates readback of unchanged runtime IAM from performing every possible
negative permission test. Native proposal/API hosting paragraphs were updated in place under revision
guards and read back, including the hosted URL, complete archive/recovery,
performed checks and participant gaps. Repository guides and Trello were updated
as part of this deployment task.
