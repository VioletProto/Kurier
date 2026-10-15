# Repository scripts

Add cross-project development scripts here when a repeated workflow cannot be
expressed clearly through the root package scripts.

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
