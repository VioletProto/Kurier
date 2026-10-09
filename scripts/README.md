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
