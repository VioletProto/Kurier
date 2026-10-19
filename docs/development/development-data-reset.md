# Development account and application-data reset

On 2026-10-19, Davian explicitly authorized deleting every Kurier development
account and all live application data so existing test email addresses could be
used again for genuine signup/verification. This supersedes preserving old test
projects and their accounting; it does not authorize deleting infrastructure,
changing the AWS account plan, or changing service limits.

Scope is account `747336059622`, `dev-api`, Ohio, and its existing Cognito pool
`us-east-2_qTDZQT1FE`, Control/Protected tables, evidence bucket, execution queue/
DLQ and the four application Lambda log groups. Initial inventory found two
Cognito users, 487 Control records including the stage marker, one protected
secret and eight evidence objects; both queues were empty. No user profiles,
emails, secret values, request payloads, evidence bodies or credential tokens
were copied into the operation receipt or committed evidence.

The destructive helper is deliberately limited to the verified resources and
initial version-11/schema-7/MCP-1 stage. `scripts/reset-development-data.mjs
--clear` requires explicit authorization. It checks the non-root identity,
resource configuration, token lifetime, plan and concurrency, refuses unsettled
jobs, closes the stage through a conditioned monotonic version change, deletes
Cognito users and waits for admitted Lambdas to drain. It then purges both queues,
deletes all Protected records and every Control record except `STAGE#dev-api /
META`, deletes evidence objects/aborts multipart uploads, and deletes the actual
application log streams while retaining their log groups and retention settings.
Application log groups are taken from each function's `LoggingConfig`, since
SST's log group name differs from the function name.

User/identity mappings, projects, requests/revisions/tombstones, saved secrets,
execution/workflow evidence and retention records, pins, agent/MCP credentials/
lookups/audits/issuances/rate counters, maintenance cursors and usage accounting
are removed. The stage-wide usage row is also removed; existing admission logic
creates fresh zero-based usage when needed, retaining its compiled per-user/
stage limits. No AWS quota or spending allowance is increased.

The API validates Cognito JWT signatures locally, so deleting a user does not
alone prevent a still-valid old token from reprovisioning an application user.
The stage therefore remains closed until the last user deletion plus the
verified 15-minute access-token lifetime, 60-second verifier skew and five seconds
of margin. Queue purge must also have had at least 60 seconds to settle.
`--finish` refuses to reopen early or after any unexpected data/configuration
change. It checks zero users, empty Protected/evidence/queues, only the Control
stage marker, and unchanged resource fingerprints. Only then does it reopen via
another conditioned version change. Errors leave the stage closed for inspection;
there is no automatic retry/reopen after uncertain operations. A genuine early
`--finish` attempt was rejected and the receipt remained byte-identical.

Infrastructure, public UI/release archives, SST/bootstrap state, Cognito pool/
client settings, keys/secrets, IAM, worker concurrency, schedules and account plan
are preserved. Capabilities remain schema 7/MCP 1 and the existing enabled execution,
pinning, agent and workflow capabilities; recovery generation remains
`dev-api-initial-v1`. The close/reopen transitions advance the stage version
11→12→13; this is a maintenance fence, not schema activation. The frontend preview
guard's expected active version changes to 13 accordingly. Earlier deployment
receipts retain their historical version-11/accounting fingerprints and must not
be mistaken for a current post-reset baseline.

Both DynamoDB tables have 35-day point-in-time recovery enabled and no on-demand
backups were listed. That existing recovery history is retained; this is a live
application reset, not a promise of immediate erasure from recovery history or
AWS audit records. Historical sanitized repository validation receipts are also
retained. No new data backup or export was created.

After completion, reload/close old tabs and choose **Create account** at
https://d2kpq905qyh9zg.cloudfront.net. The same email addresses can be registered
again. Passwords were deleted with their Cognito accounts; a chosen password is
entered anew during signup and still must satisfy the unchanged password policy.
Complete the emailed verification before signing in. Old agent and MCP credentials
are invalid because their authorization records no longer exist; unpair/reissue
through the hosted UI if those checks are repeated.

Completed readback: zero Cognito users, zero Protected records, zero evidence
objects and zero queued/in-flight/delayed messages; Control contains only the
required stage marker. The service is active at version 13, and all resource
configuration fingerprints matched before reopening. The reset passed the full
repository verification (format/lint, 58 frontend and 15 infrastructure tests,
production build and tests/vet for seven Go modules).

The reset's public result is recorded in `development-data-reset.json`. The
working receipt under `.sst/` contains configuration fingerprints and phase
checkpoints, not deleted user data. This one-time operation must not be rerun,
rebaseline itself, or delete a receipt without fresh reset authorization.

Trello synchronization could not be performed because its connector was unavailable
during this reset. Existing hosted participant acceptance remains pending.
