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

This change has not been deployed. Deployment must include the API's new poll
route, API/cleanup code, frontend, and updated local-agent executable; a static-only
frontend release is insufficient. No AWS mutations or existing credential
migrations were performed during implementation.
