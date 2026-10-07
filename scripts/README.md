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
