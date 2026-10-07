# Repository scripts

Add cross-project development scripts here when a repeated workflow cannot be
expressed clearly through the root package scripts.

`local-dev.mjs` consumes public `dev-auth` SST outputs and starts the local Go API
(`npm run dev:api`) or React (`npm run dev:auth`). It generates the private cursor
key only in process memory and refuses historical spike outputs. See the
[setup guide](../docs/development/cognito-local-projects.md).
