# Kurier Settings implementation

Davian requested MCP and local-agent management outside project tabs and chose
one local-agent pairing for all current and future projects. The
[contract continuation](../architecture/connection-settings-contract.md) records
the account scope and compatibility behavior.

Open **Settings** in Kurier's main navigation to manage **MCP connection** and
**Local agent pairing**. Projects now contain five product tabs. Switching to
Settings preserves project drafts; leaving Settings clears one-time token
views. Agent status is shared across projects and loaded after sign-in, so
opening another project does not clear the selected device.

New agent credentials approve the same exact services for all current and future
owned projects, even when no project exists yet. Use an updated foreground-agent
executable. Existing fixed-project credentials continue working; revoke and
re-pair once to opt into all-project access. Destination changes and thirty-day
expiry still require reauthorization. Manual installation/hidden token transfer
and optional private credential persistence remain the existing flow; this does
not implement the separate guided-installer work.

Implementation adds one account poll route on the existing API integration,
schema2 automatic credentials and an account queue key on the existing GSI2.
Every job retains its project identity and original ownership, destination,
secret-delivery, no-replay and publication guards. No new database or queue.

Validation includes frontend unit tests, keyboard/layout browser fixtures,
DynamoDB Local/race tests and Chromium through the local Go API plus compiled
foreground agent. Local fixtures are distinct from real AWS, Cognito, devices,
network failures or operating-system support. The initial integration-browser
run found the environment scenario still using the old pairing-tab navigation;
its navigation and exact-credential cleanup were updated before rerunning.

Verified: `npm run verify` (format/gofmt, lint, frontend/infra/tooling tests,
production build and Go tests/vet); full API `go test -race -tags=integration`
against DynamoDB Local; targeted account-agent future-project, result/ACK,
revocation and final-grant deletion/revocation tests; nine keyboard/layout browser
tests; and the full Go/Chromium/compiled-agent integration suite (17 scenarios).

## Hosted release — 2026-11-16

Davian authorized deployment, hosted testing, merge and repository cleanup.
Source `924df0757cbc5b7b179fc22b7de4e34591a5402f` is deployed at
**https://d2kpq905qyh9zg.cloudfront.net** on `dev-api` in `us-east-2`.
Frontend release:
`62262dd1b88c874339a06904a45a6e37076b4b10bc7af2e53e77aa400c3d2924`.

Verified account `747336059622`, non-root `davian-admin`, pinned SST4.17.1,
existing bootstrap5 and active version21/savedRequests11/MCP2 before deployment.
The full preview included unrelated broker/worker/controlled-endpoint package
replacements; these were excluded. Separate supported SST previews/deployments
used `--stage dev-api --target UsersProjectsApi`, then `EmptyProjectCleanup`,
then `FrontendWeb`. The API phase created only the new account polling route
on the existing integration and updated the API package. Cleanup updated only
its package. Frontend updated its assets, lookup keys and invalidation after
both backend phases completed. No IAM, database/index/queue, broker settings,
bootstrap migration, schema activation or existing-credential rewrite.

Post-deployment readback verified exact API/cleanup package hashes and polling
route target/auth, unchanged runtime configuration/IAM/account controls,
unchanged broker/worker/controlled-endpoint code, active stage/recovery marker
and bootstrap, and completed distribution/invalidation with unchanged
CloudFront configuration. Every public frontend file matched the manifest;
CSP/security headers, cache/content types, private S3 denial, CORS, foreign
origins, deep links, missing assets/API/archive404 and redirect checks passed.

Genuine hosted Chromium checks used two temporary Cognito identities (mail
suppressed), three disposable projects and the freshly built Linux amd64
foreground agent. Pairing happened before the owner's first project existed.
Both projects created afterward executed through that original pairing, with
**exactly two controlled-loopback target sends total** and sanitized immutable
evidence. Protected input and agent credentials were absent from evidence and
captured agent output. Unapproved destinations were rejected before dispatch.
A real foreign-owned project was denied to the agent and excluded from MCP.

The UI showed one Settings surface outside the five project tabs, preserved an
unsaved project draft, exposed the shared device in the project execution
selector and retained account-level MCP access. Leaving Settings cleared its
one-time token. Agent/MCP revocation through Settings denied subsequent use;
sign-out/reload cleared private surfaces with empty Web Storage/IndexedDB/Cache
Storage and no page errors. Passwords/tokens/cookies were not included in logs,
traces, screenshots, browser-state exports or deployment receipts.

The [sanitized release receipt](connection-settings-deployment.json) records the
plans, package hashes, HTTP/browser results and cleanup. Scheduled product
cleanup left all three projects tombstone-only, with zero Protected children and
empty evidence prefixes. Conditional application identity/credential cleanup,
Cognito deletion and identity-absence readback completed for both disposable
users. Numeric admission usage is intentionally retained; quotas/counters are
not reset or refunded. Broker
session records follow the existing logout/retention policy; physical elapsed
expiry was not exercised. Existing accounts, projects and agent/MCP credentials
were not modified.

The built agent is `.build/local-agent/kurier-agent` in this workspace, from
clean source `924df07`. Existing fixed-project pairings retain their scope;
explicitly revoke/re-pair with the updated executable to adopt all-project
access. This is a Linux runtime result, not a Windows/macOS, separate-device LAN,
elapsed-expiry, fault-injection or metered-cost result. Distribution/automatic
installation remain the separate guided-setup work.

[PR #28](https://github.com/VioletProto/Kurier/pull/28) tracks the release and
integration. The implementation's CI passed all twelve jobs in
[run 43986735334](https://github.com/VioletProto/Kurier/actions/runs/43986735334).
