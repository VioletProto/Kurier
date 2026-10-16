# Read-only MCP development deployment and validation

Davian explicitly approved the scoped deployment and guarded activation in
[the review packet](../development/read-only-mcp.md), including `916ea23`.
On 2026-10-16 (America/Denver), Davian reported successful deployed Codex
connection, information/execution retrieval, preserved redaction and immediate
access denial after revocation. He authorized final review, merge after CI and
deletion of the merged branch. Design acceptance, fixture tests, deployed AWS
validation and operator-reported results remain separate evidence.

## Deployment and readiness

On 2026-10-15, non-root `davian-admin` in account `747336059622`, us-east-2,
rechecked the existing-bootstrap inventory and SST diff. The preview matched the
approved four new routes and existing-function code/configuration/assets.
Deployment completed; API, cleanup, worker and shared controlled endpoint were
Active/Successful and matched local deployment archive hashes. The final SST
diff reported **No changes**. [Machine-readable proof](read-only-mcp-deployment.json)
records package/IAM hashes and activation readback.

| Consumer                              | Deployed SHA-256 (base64)                      |
| ------------------------------------- | ---------------------------------------------- |
| API and cleanup                       | `5R/t9UuF/UhpoTxtk4u3JuLCauMlR71t8bREoZT9oho=` |
| Worker and shared controlled endpoint | `IOwGRkqoKIxzjXsPmsg5LxhfBVWeURZ7Q4sjoR3rgnY=` |

Before activation, verification checked:

- All four exact MCP route keys, existing API integration, payload version 2.0
  and application middleware authentication. API Gateway authorization is NONE;
  the Go handlers enforce Cognito management versus separate MCP capabilities.
- All four new Lambda invoke statements, scoped to the existing API ARN using
  the existing SST API-wide source restriction, and no new runtime role policy.
- Exact before/after hashes of inline IAM policies, attached policy versions and
  role trust documents for all four functions. Those hashes were unchanged.
- Unchanged environment configuration except the reviewed MCP capability and
  saved-request readiness values. Memory/timeouts, ARM64/AL2023 runtime,
  protected resources and existing capability markers remained compatible.
- Existing enabled minute cleanup schedule and target; Free/ACTIVE, account and
  unreserved concurrency 10, no reserved/provisioned function concurrency,
  SQS maximum 2/batch 1/standard pollers remained unchanged.

No new table, index, bucket, key, queue, function, standing host, frontend host,
Cognito pool/client setting, plan or concurrency/quota change was introduced.
Deployment assets were replaced only in the existing bootstrap bucket.

## Guarded activation and inspected failure

The first attempt failed locally: AWS CLI could not parse JSON through Node's
socket-backed `/dev/stdin`. The error occurred before a DynamoDB mutation.
The failure handler and an independent strong read both showed
active/version9, savedRequests6, MCP absent, same recovery generation. There was
no automatic retry, reopen or mutation to reconcile.

A read-only reproduction identified the parser issue. The corrected helper
passes JSON through an inherited seekable anonymous RAM descriptor, verified
with another strong stage read. Secret values never enter arguments, logs,
environment variables or persistent files. This helper requires Linux/Python
`memfd_create`; it fails rather than falling back to disk storage.

After that inspection, the approved guarded activation was resumed from the
verified pre-MCP marker. It reverified consumers/routes/IAM/configuration,
inventoried settled writers, then used two conditioned DynamoDB transactions:

1. active/version9/schema6 → mcp-migration/version10/schema6.
2. After two stable closed-stage accounting inventories,
   mcp-migration/version10 → active/version11/schema7/MCP1.

Activation readback:

```json
{
  "state": "active",
  "version": 11,
  "savedRequestsSchemaVersion": 7,
  "mcpReadOnlySchemaVersion": 1,
  "recoveryGeneration": "dev-api-initial-v1",
  "existingCapabilitiesPreserved": true,
  "accountingPreserved": true,
  "counterResets": false
}
```

All five existing projects and all quota/accounting rows had identical hashes
before closure, on both closed inventories and after reopening:
`e543a81c133ae49027d129a04626adaf0e4d8fea92d1cadf4c79195a00d1d600`.
The full marker comparison allowed only state/version/schema7/MCP1 changes.
No generation, admission usage, inflight, storage, pin accounting or existing
capability was reset. Later test execution usage is legitimate additional
usage and is not refunded by fixture cleanup.

## Genuine AWS validation

Live credential/read validation passed on the resumed published fixture.
Scheduled physical cleanup subsequently completed through a resumed observer;
no direct drain was used. The initial four-minute observer windows were too
short for the bounded minute-scheduled cascade. The driver now allows a nominal
fifteen-minute cleanup window and preserves receipts on incomplete observation.
The opt-in driver uses temporary suppressed-email Cognito users with real SRP,
HTTPS/Lambda middleware, DDB/S3/KMS, deployed workers and scheduled cleanup.
It does not simulate the hosted API and does not claim operator browser/TUI
acceptance. Passwords, JWTs, capabilities and evidence remain in memory; the
private local receipt contains only disposable fixture identifiers/status.

The initial driver run correctly received HTTP400 because its large-body
fixture omitted the required `sensitive:false`. One small request existed;
no execution was admitted. Its two projects were removed by scheduled cleanup,
Protected/S3 emptiness verified and temporary identities deleted. A second
observer had a JavaScript variable-name error after early successful scoped
reads; that fixture's IDs were retained for scheduled cleanup. A later run's
transient workflow hydration GET503 was inspected and resumed through bounded
read retries using its already-published executions. Admission/mutation calls
are never automatically replayed. These unsuccessful driver attempts are not
counted as completed validation passes.

Live checks completed through genuine Cognito SRP and deployed HTTPS:

- Two distinct temporary Cognito users; one-time credential POST201, same-key
  receipt POST200 with `token:null`, current metadata without the token.
- MCP capabilities rejected by Cognito management and local-agent session routes;
  browser JWT and a valid separate local-agent capability rejected by the MCP
  bridge. Revoking that local-agent credential did not revoke MCP access.
- Foreign-user project/revocation404; same-owner excluded-project404; discovery
  limited to the authorized project, bounded pages and nonrepeating continuation.
- Successful calls to all seven read tools, including actual completed workflow
  and execution records from the deployed workers.
- Complete MCP capture equal to ordinary owner evidence; immutable manifest
  unchanged; independent S3 bytes match the stored SHA-256. A protected bearer
  fixture value was absent from MCP responses and audit records.
- A real capture over 96 KiB rejected with HTTP413/`evidence_too_large`; summary
  explicitly incomplete with reason and omitted fields. Delivered hosted response
  bytes remained below 256 KiB; exact serialized MCP boundary is fixture evidence.
- Eleven persisted success/denial audit records on this fixture, including safe
  tool/code/correlation metadata for scope and oversize denials.
- Owner revocation DELETE204 followed by old-token401; fresh scoped capability
  denied evidence404 immediately after project deletion was accepted.

Temporary users were created with suppressed email; no pool/client settings or
existing user credentials were changed. Test evidence used three controlled
executions (a two-step workflow and one large request). The resumed observer
read those publications; it did not rerun the HTTP targets.

Final cleanup verification used the ordinary deployed schedule, not operator
invocation/direct draining: all six disposable projects across the failed and
successful observers reached tombstone-only Control, empty Protected and no S3
objects under their evidence prefixes. All six temporary Cognito users and
application credential records were removed. Original application users/projects
were not used as fixtures. Usage accounting was retained, not refunded/reset.
The full live read and cleanup evidence is consolidated in the machine-readable
record; the earlier observer failures remain disclosed rather than relabeled
as uninterrupted passes.

The development backend is ready for pairing. Davian's normal account was
not used for the automated credential probes. [Numbered setup steps](../development/read-only-mcp.md#pairing-and-codex-tui-setup)
separate browser consent, one-time terminal pairing/registration and the server
command launched by Codex itself.

## Operator-reported deployed Codex acceptance

On 2026-10-16 (America/Denver), Davian confirmed in this task conversation that
he connected another Codex instance to the deployed Kurier backend and observed:

- The client could request information and retrieve executions.
- Redacted fields remained redacted in the information returned to Codex.
- Revoking MCP access immediately removed access; subsequent tool calls stopped
  working.

This is Davian's genuine deployed-client report, separate from the automated
installed-Codex test against simulated HTTPS services and the disposable AWS
backend checks above. No credential, evidence body or client transcript was
requested or recorded. The report does not identify a client version, enumerate
all seven tools, document exact browser/hidden-prompt steps, or establish
cancellation of in-flight responses or recall of already delivered data.
Those details are not inferred from his confirmation. The deployed connection,
execution inspection, redaction and revocation acceptance gate is complete.

## Separate client/local evidence and remaining checks

`npm run verify` passed formatting/gofmt, ESLint, 56 frontend tests, 10 infra
tests, TypeScript/Vite build and all seven Go modules' tests/vet. The new
operational scripts also receive syntax and undefined-variable lint checks.
Existing DynamoDB Local race and Chromium fixture results remain local evidence.

Installed `codex-cli 0.162.1` was retested with the compiled foreground server,
official Go SDK 1.8.0 and MCP 2025-11-25. Seven-tool discovery/calls,
summary/complete/size/scope denial and live-connection revocation passed against
a controlled HTTPS service. No model turn or deployed AWS call occurred in that
test. It is genuine installed-client integration against simulated services.

Still unperformed or not separately confirmed: detailed browser token-display/
hidden-prompt UX and `/mcp` inspection steps; real thirty-day elapsed expiry, clock-skew and
PITR/recovery drills, interrupted cloud activation after a committed close,
comprehensive negative IAM/quota saturation, exact live 256-KiB serialized MCP
boundary and metered costs. Local protocol tests cover response-size handling;
live checks must not be presented as an exhaustive boundary/race campaign.

The current incremental forecast remains about $0.29/month for the documented
10,000-read workload, with $0.50–$1 planning allowance before credits, not a
spending cap or a measured bill. Free plan/quota/concurrency remain preserved.

## Operational commands

The read-only pre-MCP preview intentionally refuses an already activated marker.
Do not rerun activation on the live stage or use an old activation/recovery
script as a substitute. For later approved rollouts, inspect the new diff and
retain a fresh baseline before mutation.

```sh
node scripts/verify-read-only-mcp-deployment.mjs --capture-baseline /tmp/kurier-mcp-baseline.json
node scripts/verify-read-only-mcp-deployment.mjs --verify-baseline /tmp/kurier-mcp-baseline.json
node scripts/activate-read-only-mcp.mjs --activate /tmp/kurier-mcp-baseline.json
```

Those are operator deployment steps, not Codex startup commands. The validation
driver is opt-in and mutates only its disposable fixture:

```sh
node scripts/validate-read-only-mcp-aws.mjs --validate /tmp/kurier-mcp-new-fixture.json
node scripts/validate-read-only-mcp-aws.mjs --resume /tmp/kurier-mcp-existing-fixture.json
node scripts/validate-read-only-mcp-aws.mjs --cleanup /tmp/kurier-mcp-existing-fixture.json
```

Resume reads existing definitions/publications, explicitly revokes any surviving
fixture MCP credential before issuance, and never repeats admission. Cleanup
uses project deletion plus the actual schedule, then verifies tombstone-only
Control, empty Protected/S3 and removes only the disposable identities/credential
lookups. Removing fixture account audits is explicit test teardown, not proof
that ninety-day elapsed cleanup ran. Preserve receipts on incomplete/uncertain
work and inspect before resuming.
