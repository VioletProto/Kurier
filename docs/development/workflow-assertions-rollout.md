# Workflow assertions rollout stopped before closure

Davian accepted final review `7b5e847b6929e1b0006aede823fcb43b1926b9f8`
and authorized its scoped dev-api rollout, separate fresh close/activation
approvals, disposable identities/projects, legacy seed A, bounded A–L campaign
and normal cleanup. Execution stopped at the **reviewed closure preflight**.
No close approval or activation approval was issued, no write mode was invoked,
and no deployment, capability activation, reopen or HTTP replay occurred.

The [sanitized execution receipt](workflow-assertions-rollout.json) records
independently observed identifiers, hashes, counters, target audits and cleanup.
The [fresh preview](workflow-assertions-rollout-preview.json) is equivalent to
the accepted release by resource name/type/operation/properties, ZIP hashes and
frontend content; nondeterministic resource event ordering is ignored.
Historical review packets remain records of their original holds. This record
supersedes those holds only for the attempted authorization and its actual result.

## Blocking reviewed guard

After genuine legacy A completed and scheduled maintenance settled its outboxes,
the unmodified reviewed CLI was invoked with **`--inspect` only**. It exited 1:

```text
Unsettled work or new assertion data; reconcile separately.
```

The two observed `executionOutbox` rows were `published`, with matching completed
jobs. `publishOutbox` in `services/api/execution/maintenance.go` uses `published`
for normal settled delivery; it does not produce `sent`. The reviewed
`inspectRows` in `scripts/lib/workflow-assertions-activation.mjs` rejects every
outbox whose state is not `sent`. Waiting for normal settlement therefore cannot
satisfy this guard. This is an execution-tooling incompatibility exposed by the
legacy seed, not evidence that HTTP should be replayed or durable rows rewritten.

The added local regression reproduces that rejection and verifies zero writes
and an unchanged active13 marker. The guard and application are **unchanged**.
No approval was manufactured from an incomplete/failed inventory, and the passing
empty-project preflight before A was not reused after A changed accounting.
Any later empty-project preflight is cleanup evidence, not permission to resume
the deployment after discarding its legacy compatibility seed.

Two audit-reader issues were corrected read-only before accepting send-count
proof: the fixture's actual `LoggingConfig.LogGroup` retains an older function
suffix, and CloudWatch results require bounded pagination, including empty pages.
Logs were read in memory and only UUID/mode/authorization/request-ID audit fields
were retained. These corrections caused no write, admission or target request.
Frozen reruns must preserve their URLs; future audit ledgers must reconcile
expected repeated fixture UUIDs using distinct gateway request IDs.

## Release and actual live hashes

Clean accepted source: `68d9f218225c7092a1e4fca0b66a21421d862a22`.
Manifest SHA-256:
`920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6`.

| Packages                    | Accepted ZIP SHA-256, **not deployed**                             |
| --------------------------- | ------------------------------------------------------------------ |
| API / cleanup               | `f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32` |
| Worker / controlled fixture | `15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379` |

Accepted frontend release, also **not deployed**:
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`.
All four retained release archives passed exact pin verification. A fresh clean
source preview produced the same 11 logical changes and exact package/frontend
bytes. Bootstrap v5, existing passphrase metadata and all 88 current SST state
object metadata entries were unchanged before/after preview. No SST deploy ran.

| Packages                    | Independently verified current live ZIP SHA-256                    |
| --------------------------- | ------------------------------------------------------------------ |
| API / cleanup               | `cf65f8badc4145962fee4cdc79ab4d4cb1f1829d302f41ce0f30833ae19a326e` |
| Worker / controlled fixture | `2fb39ef077362119c0d35206e935f291f4085c8017032f5cc24800a43a170900` |

Prior live ZIPs were downloaded to private ignored local storage and verified
against Lambda `CodeSha256`; previous frontend app files were retained and
verified against their public manifest. Live frontend release was
`acb951896da16807815751c0509b1f25647656d8a0963932c7fc1f3a8ccac67e`.
No rollback was needed or attempted. The accepted post-schema2 rollback limits
still apply to any future rollout; these rollback bytes do not make old readers
safe after activation or permit rewriting immutable evidence.

## Genuine runtime result and limits

Account 747336059622, non-root identity, us-east-2/dev-api were verified before
mutations. Two uniquely tagged `example.invalid` Cognito users were created with
message suppression and permanent random passwords held only in RAM. Genuine
Cognito SRP sign-in resolved two distinct owners; one disposable project belonged
to each. This checks provider authentication, not signup or email delivery.

Only **A** ran: token extraction followed by a bound Bearer consumer. Both steps
and the run completed, and the target's two distinct gateway request-ID audit
records independently prove **two sends**, with the consumer authorized. Both
stored schema-1 evidence objects matched their manifest byte counts/SHA-256,
omitted assertions and redacted the synthetic binding value. The frozen run's
assertion semantics marker was absent. Foreign-owner read returned 404;
anonymous read returned 401. The receipt records exact evidence checksums.

**B–L were not run.** There is no genuine schema2/numeric round-trip, whole-run
rerun, MCP projection, pin/unpin, assertion-failure precedence, hosted reader or
frontend activation validation from this attempt. No MCP credential was issued.
No direct fixture probe, live fault injection, elapsed-retention or PITR exercise
was performed. No operator-reported runtime observations are treated as verified
results; the runtime facts here were independently read from AWS/API/target logs.

Two admitted steps were charged: stage month usage increased **2 → 4**, owner A
day usage is **2**, and skipped/reserved submissions were not fabricated. The
maximum28-step/28-send bounds were not approached. The receipt reports exact
coordinator API/poll counts and its conservative AWS-operation count. Other
preflight/preview SDK calls were not independently metered end to end; an exact
total AWS-read claim is therefore unavailable. No additional inventory loops
were launched to consume the remaining read allowance.

The $1 incremental allowance remained the procedural ceiling. The accepted Ohio
planning rates imply a small fraction of it for two Cognito MAUs and two bounded
worker sends; no new standing resource or package upload was created. **Actual
billed incremental cost is not independently verified** because metering is
delayed. The accepted rates/estimate are not a measured bill or an AWS spend cap.

## Cleanup and follow-up review

Normal project deletion was requested once per project; both returned 202 and
immediate owner reads returned 404. Scheduled cleanup alone performs physical
removal. Both projects were verified tombstone-only in Control, with zero
Protected children and empty S3 project prefixes. Both Cognito users were globally
signed out/deleted and independently confirmed absent; the credential-holding
process exited. Cleanup took **238.034 seconds** within the 20-minute window;
the whole campaign took **501.638 seconds** within two hours. No operator Lambda
invocation, manual row/object deletion, quota reset or clock change occurred.
Expected tombstones, residual sweep registrations and
application identity/user/admission metadata remain under existing policy.
Deleting Cognito identities does not establish immediate expiry of offline access
JWTs; immediate project denial is a separate verified property.

The final read-only inspection passed with zero active projects, empty observed
queue and healthy accounting, writes0. The full marker hash stayed
`dca180063fb02b0c7bf7b1a5e582ae4b5b19109cadaae8eea61983412a09d10d`:
**active13/savedRequests7, assertions absent**, generation `dev-api-initial-v1`.
All live package/configuration/IAM/control hashes still matched the reader
baseline. Backend/frontend readiness stayed false because deployment never ran.
Stage quota version advanced5 →10 through legitimate admission/storage cleanup;
month count4 and owner day count2 were preserved, storage returned to baseline
and both pin counters were zero. The final accounting digest is
`5cf132afc2815c2eb4471a4cbde8e7a4f3d7620d36e148f36c1f1cb406dd1453`.

Before any resumed rollout, review a correction to the activator's outbox
settlement proof against actual code, terminal jobs and immutable publication,
plus a full legacy-seed → closure preflight regression. A simple unreviewed
`sent` → `published` substitution is not an execution authorization. Preserve
the exact pinned release unless a new artifact review explicitly replaces it.
Recheck every guard and create separate fresh approval records only after it
passes. This stopped attempt's identities/projects consume its two-identity/
two-project allowance; replacement fixtures and a replacement legacy seed require
an explicit revised campaign allowance before another attempt. Preserve already
charged usage. Existing operations/incident follow-ups and broad fault/PITR holds
remain open.

Local verification: repository formatting/gofmt, lint, 64 web and 15 infrastructure
tests, TypeScript/Vite build, Go tests/vet in all seven modules passed. The
activator suite passed44 checks including the new stop regression; its optional
DynamoDB Local case was skipped. All six release-pin checks passed. Sandbox
subprocess permission failures were resolved by rerunning with the required
permissions; they were not application test failures.
