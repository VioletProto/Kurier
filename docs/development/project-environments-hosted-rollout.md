# Project environments hosted rollout — 2026-11-04

Environments are deployed and active in the development/demo application at
<https://d2kpq905qyh9zg.cloudfront.net>. The stage advanced from
`workflow-migration`/16 to `active`/17, with saved requests 9, environments 1,
protected secrets 2 and local-agent executions 2. Other capabilities, recovery
generation and existing accounting were preserved.

## Deployment and activation

Davian's 2026-11-04 request authorized completing deployment and moving beyond
the stalled release process. This rollout used the existing supported
`scripts/sst-auth.mjs` AWS-session bridge and SST 4.17.1, rather than restarting
an expired custom deployment campaign. Historical ledgers, deadlines, claims,
retained artifacts and recovery proposals remain untouched. Earlier separate
phase holds describe those historical operations, not the current authorization.

The intended account `747336059622`, non-root `davian-admin` identity and
`us-east-2` were verified. Fresh read-only inspection found the unchanged old
release closed at version 16, 107 projected metadata rows, zero active projects,
seven accounting rows and an empty queue. Old package/configuration/IAM,
gateway and frontend checks matched their retained baseline. Inventory reads
projected metadata only; full Lambda configuration was hashed in memory rather
than logged or persisted.

SST's diff showed only four function code updates, eight routes with their
integrations/invoke permissions, and the existing frontend's assets/KV entries
and invalidation. Existing bootstrap resources were reused; there were no
table, key, bucket, role-policy, plan, quota or concurrency changes. The supported
SST deployment completed successfully before activation.

Deployment source was `fa361335dc562b6f7daa5303e1a05251f68464b2`,
on `feat/deploy-project-environments`. Production application and infrastructure
source match implementation `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`; subsequent
changes in those directories add one integration test. SST rebuilt its normal
archives from this checkout, so these are newly observed build hashes, not a
claim that the older adopted ZIPs were deployed:

| Functions                                          | Deployed ZIP SHA-256                                               |
| -------------------------------------------------- | ------------------------------------------------------------------ |
| UsersProjectsApi / EmptyProjectCleanup             | `e7fba5f40952e666747b4b752aa099946ecdbc1ca0ecf8c016d146a5594582b0` |
| CloudExecutionWorker / ExecutionControlledEndpoint | `43d719f26e19c4c9ac2acf417ae685993d05318912c919924f84d05f335dee03` |

AWS-reported code digests matched all four local SST archives. Existing
configuration/IAM/platform controls matched the pre-deployment baseline; old
routes and integrations were unchanged, and all eight added routes and invoke
permissions matched the intended scope. The historical verifier initially
rejected new permissions because it pinned the preview's generated physical-ID
suffixes. The actual logical names and complete permission shapes matched;
the one-off verifier normalized only those suffixes and retained scope checks.
That first verification sent no activation write and required no cloud repair.

One conditional DynamoDB transaction activated the environment tuple, guarded
by every prior marker field, absence of the environment capability and all seven
accounting preimages. Fresh readback verified the complete new tuple, unchanged
generation and unchanged accounting. No data migration or old-release reopening
was needed. All five frontend files matched the prepared release over actual
CloudFront HTTPS; the entrypoint and security headers also passed. Frontend
release ID is `38325083b003b3f320752dbfa0357c377d57d2b66c1175e06c88ba3638210236`.

## Verification and remaining scope

`npm run verify` passed formatting, lint, application/infrastructure/tooling
tests, frontend production build and all Go tests/vet. Sandbox restrictions on
subprocess/network access required running verification and deployment commands
with normal host access; these were environment restrictions, not application
failures.

The hosted smoke passed from **18:38:51 to 18:42:02 UTC** using one disposable
no-email Cognito user, genuine SRP authentication and one disposable project:

- Environment create/list/detail/update/delete and stale-revision rejection.
- Exact decimal `0.10000000000000001` substitution and masked protected values.
- One cloud execution against the existing controlled echo endpoint, returning
  HTTP 200 and owner evidence schema 3 with the selected environment context.
- A second execution through rerun after changing the environment's public value
  to `2`; it retained the original decimal and environment revision. The original
  evidence response remained byte-identical.
- The protected canary was absent from all checked API replies and evidence.
- Normal project deletion completed: only its deleted project tombstone remains,
  protected children are absent and its evidence prefix is empty. The disposable
  Cognito user was deleted. No active smoke project remains.

These checks exercised real Cognito, API Gateway/Lambda, DynamoDB, KMS, SQS and
S3 behavior. Hosted browser interaction, local-agent protocol interoperability
and the full workflow/MCP environment matrix were not repeated; their existing
local/CI evidence remains distinct. This rollout does not claim a CloudTrail
audit, exact AWS request metering or measured billing.

Private operational snapshots, activation request/readback, logs and sanitized
receipts are under ignored mode-0700 `.build/environments-rollout`, with private
receipts mode 0600. Credentials and protected test values stay in process memory;
only sanitized findings are published here. The next proposed milestone is
[OpenAPI import/export and response validation](openapi-next-milestone.md).
