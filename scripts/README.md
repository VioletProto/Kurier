# Repository scripts

Add cross-project development scripts here when a repeated workflow cannot be
expressed clearly through the root package scripts.

`lib/workflow-run-poll.mjs` supplies the operator coordinator's bounded status
GET policy and acknowledged-submission reader adapter. It retries only the
allowlisted `503 service_unavailable` response, honors Retry-After and shares
the original API/poll/time budgets. It cannot issue submission or stage writes.
`npm run test:workflow-poll` and the local Go HTTP bridge verify this policy.
See the [diagnosis and integration contract](../docs/development/workflow-run-get-503-diagnostic.md).
This helper does not authorize live execution or resume a stopped rollout.

`local-dev.mjs` consumes public `dev-auth` SST outputs and starts the local Go API
(`npm run dev:api`) or React (`npm run dev:auth`). It generates the private cursor
key only in process memory and refuses historical spike outputs. See the
[setup guide](../docs/development/cognito-local-projects.md).

`npm run dev:cloud` starts the local frontend against the authorized `dev-api`
HTTPS endpoint from public SST outputs. `sst-auth.mjs` also bridges the CLI session
for `sst:diff:api`/`sst:deploy:api`. `ensure-cursor-key.py` supplies the stable cloud
cursor key through a Linux memory descriptor to SSM without printing or writing
the key. See [cloud setup](../docs/development/aws-users-projects.md).

`enable-saved-requests.py` previews and explicitly activates the known-request
stage capability without resetting recovery generation. It is scoped to the
existing dev-api table/account or the loopback Local table; AWS application
requires both handlers' successful deployment markers. `aws-integration.mjs`
bridges CLI credentials only into the scoped Go test process memory. Run it via
`npm run test:aws:integration`. See [saved requests](../docs/development/saved-requests.md).

`node scripts/aws-integration.mjs TestAWSCloudExecutionCredentialNames` runs the
scoped deployed-worker credential-name regression with zero protected inputs.
It creates only a disposable owned fixture, compares private S3 evidence with
the API service envelope and verifies fixture cleanup. Existing read-only audit
or resumed-cleanup modes skip this additional fixture. The opt-in signed-in
browser probe `validateCredentialNameRedaction()` checks actual authenticated
API delivery. See the [deployment record](../docs/architecture/cloud-execution-validation.md#credential-name-correction-deployment--2026-10-09).

`activate-evidence-pinning.mjs` defaults to a read-only, metadata-only `dev-api`
preflight. `--activate` closes the stage and conditionally initializes pin counters
before capability activation; it requires separate deployment approval and
compatible deployed consumers. It preserves recovery generation and never
reopens after an uncertain failure. See the
[evidence pinning review and runbook](../docs/development/evidence-pinning.md).

`verify-evidence-pinning-deployment.mjs` compares scoped deployed function hashes
with local deployment archives and verifies compatible configuration before
activation. `aws-integration.mjs TestAWSEvidencePinning` uses disposable AWS
fixtures to validate pin accounting, immutable S3 evidence and scheduled cleanup;
its expiry mutation is fixture-only and its identities are direct SDK identities.
See [deployment results](../docs/architecture/evidence-pinning-validation.md).
The explicit `KURIER_AWS_PIN_BROWSER_OWNER` preparation mode creates a new
disposable fixture for the participating owner and changes only its guarded RET
expiry. It leaves cleanup to the owner's browser check; it is preparation,
not authenticated route validation. Keep its safe IDs for reconciliation.

`verify-local-agent-deployment.mjs` verifies the approved local-agent consumers,
archive hashes, routes, scoped IAM and cleanup schedule, including unchanged
Free-plan/concurrency settings. `activate-local-agent-executions.mjs --activate`
requires explicit approval: it closes/re-inventories/activates with exact marker
guards, preserves generation/accounting, validates settled published writers,
and refuses automatic resume/reopen after failure. It is not a recovery command.
`aws-integration.mjs TestAWSLocalAgent` exercises actual deployed agent HTTPS,
protected delivery/publication/revocation and scheduled fixture cleanup with
SDK-created identities; genuine browser pairing remains separate. See the
[local-agent deployment and operator checks](../docs/development/local-agent-execution.md).

`preview-read-only-mcp.mjs` is a read-only dev-api inventory and two-phase
activation plan: savedRequests6→7/mcpReadOnly1, same recovery generation and
accounting. It verifies the non-root account, existing SST bootstrap, Free plan,
quota and consumer/queue concurrency, and reports unresolved work. It has no
mutation flag. `preview-read-only-mcp-deployment.mjs` performs that guarded
inventory before an authorized SST diff; it never deploys or activates. See
[read-only MCP setup and validation](../docs/development/read-only-mcp.md).

`verify-read-only-mcp-deployment.mjs --capture-baseline PATH` records only public
configuration/IAM hashes before an approved deployment. `--verify-baseline PATH`
checks exact built/deployed archives, four MCP routes/invoke permissions,
unchanged runtime IAM/configuration and Free-plan/quota/concurrency/schedule.
`activate-read-only-mcp.mjs --activate BASELINE` is an explicit approved two-phase
schema6→7/MCP1 activation, not a recovery command. It refuses unexpected state
and inspects failures without automatic retry/reopen. JSON mutation input uses
an inherited anonymous RAM descriptor (Linux/Python), never secret argv/disk.
`validate-read-only-mcp-aws.mjs --validate NEW_RECEIPT` uses temporary real Cognito
SRP users, deployed HTTPS/worker/storage and scheduled project cleanup. Its
`--resume RECEIPT` reads existing publications without replaying admission;
`--cleanup RECEIPT` reconciles only that disposable fixture. Values remain in
memory; receipts contain IDs/status only. See the
[deployment/validation record](../docs/architecture/read-only-mcp-validation.md).

## Frontend hosting

Use Node 22.18+ (native TypeScript stripping) for the frontend helper scripts.
`build-frontend-artifact.mjs` reads only the public `dev-api` output fields and
builds a fail-closed hosted artifact with a filtered build environment and no
dotenv loading. `test-frontend-hosting-browser.mjs` runs production assets against
a local Chromium/CSP/cache fixture with all AWS requests blocked.
`preview-frontend-hosting.mjs` is inventory-only by default; `--diff` additionally
runs only SST diff after verifying the existing version-5 Ohio bootstrap,
passphrase/state, account, plan, quotas, consumers, auth and stage/accounting.
`review-frontend-preview.mjs` publishes selected public evidence from that preview.
`frontend-retention.mjs <current-id> <previous-known-good-id>` is a read-only
30-day/current/previous retention planner; it never deletes objects.
`restore-frontend-artifact.mjs <verified-release-id>` reads the complete private
release archive and shared hashed assets, verifies every manifest hash, and
atomically restores a local rollback candidate without building or uploading.
See the [operating and rollback guide](../docs/development/frontend-hosting.md).
No helper grants deployment or general-public signup authorization.

The development/demo frontend is https://d2kpq905qyh9zg.cloudfront.net.
`deploy-frontend-hosting.mjs --deploy` requires an explicitly approved scoped
plan and a fresh guarded preview; `--recheck-and-deploy` combines a fresh preview
with a strict allowlist of updates to the four existing static hosting resources.
It refuses creations, deletions and backend/IAM changes. It never activates a
schema marker. Reusing either command requires authorization for that release.
`verify-frontend-hosting-deployment.mjs --capture-baseline PATH` captures public
fingerprints before initial hosting deployment; `--verify-baseline PATH` proves
exact built packages, unchanged runtime IAM/configuration/controls and only the
API's hosted-origin addition against that baseline and a fresh inventory.

`inspect-frontend-hosting-state.mjs` exports read-only ignored state and lists
public resource metadata. `verify-frontend-hosting-resources.mjs` checks the
exported deferred upload/invalidation dependencies and actual generated AWS
policies, live routing Function/KV entries and completed distribution/invalidations.
`validate-frontend-hosting-http.mjs` checks genuine public HTTP/CDN/S3/CORS
without account creation or product writes. `test-frontend-hosting-live-browser.mjs`
checks real signed-out Chromium rendering/CSP/CORS/persistence.

`prepare-frontend-marker-release.mjs` creates local public marker B bytes only.
`exercise-frontend-hosting-rollback.mjs --deploy` is an explicitly authorized real
A→B→A deployment exercise, using the strict static-update preview guard, a real
browser, remote-only A reconstruction and no rebuild. If inspection confirms B
is deployed and recovery has not begun, `--resume-b` can resume that state using
prior genuine A HTTP evidence; it never blindly retries an uncertain deployment.
An interrupted recovery requires inspecting local archive/backup state before
continuing. See [deployment results and participant checks](../docs/development/frontend-hosting-validation.md).

`reset-development-data.mjs --clear` is a destructive, explicitly authorized,
one-time `dev-api` test-data reset with exact resource/version guards. It closes
the stage, deletes Cognito users and all live application data/credentials,
drains Lambda writers, purges queues and clears application log streams. It
keeps the stage closed through the previous access-token expiry window.
`--finish` verifies empty stores and preserved configuration before a conditioned
reopen; failures stay closed for inspection. It preserves infrastructure,
frontend releases and existing PITR history. Never use this as schema activation
or automatically retry an uncertain reset. See the
[authorization, scope and reset result](../docs/development/development-data-reset.md).
