# Proposal operational records

Archived from the [native capstone proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
on 2026-10-26, before its documentation-only cleanup. These excerpts preserve
historical deployment, usage, cost and verification statements verbatim. They
are dated source records, not a fresh AWS inspection or current feature status.
Later accepted contracts and slice records take precedence over stale future-work
or hold language. No credentials, response bodies or new runtime evidence are
included.

## Documentation cleanup and verification — 2026-10-26

The proposal retains its summary, motivation, MVP commitments, research context,
image, native milestone table and faculty fields. Operational paragraphs below
were replaced with concise technology/research descriptions; the cloud-execution
appendix after Faculty Review was relocated here. Accepted library selection and
the accepted workflow/local-agent contracts replace obsolete pending-selection
wording without expanding product scope.

The original 11-row/four-column table, ten phases, 20 hours per week and 200-hour
total are preserved. A separate paragraph identifies the earlier W2/W9
architecture revisions and Davian's new W4-to-W9 Friday finish target. There is
no institutional submission deadline. The [remaining-work plan](../planning/remaining-work.md)
now records an inferred, unconfirmed December 4 finish, six weekly slots and
**116 hours of remaining required effort under current assumptions**: 16 hours
from Tuesday, October 27 plus five 20-hour weeks. The weekly workload prioritizes
required MVP work, then useful refinement, then optional scope if the earlier
priorities finish early. Davian's history-based duration estimates remain
provisional: 26–50 hours for the four named work areas. Keep the 5–10-hour
troubleshooting reserve separate; it contributes worked time only when actually
used and logged once, displacing lower-priority work within the 116 hours.
The prior 31–60-hour combined work-plus-reserve estimate is not the required
workload or a record of hours worked. No acceleration requirement is inferred.
PRs #15 and #16 are merged; environment deployment/activation and runtime
validation remain held for separate review. The original October 26 assumption
of 120 hours was corrected to 116 hours on October 27; December 4 remains
inferred, not confirmed.

Native revision-guarded edits and readback verified the relocated content,
unchanged table cell text, image and faculty fields, preserved narrative/MVP
text outside the two obsolete contract references, and bold technology labels
with ordinary body text. During merge closeout, the proposal links to this record and the work plan are
synchronized to `main` so deleting the documentation branch preserves navigation.
Native structure/readback was checked; rendered PDF
page layout was not assessed.

`npm run verify` passed formatting/gofmt, lint, web/infra/tooling/Python tests,
web build and tests/vet across all nine Go modules, including spike test-binary
builds. The optional activator DynamoDB Local case was skipped; no separate
integration or genuine runtime campaign was run. The initial sandbox run hit
read-only Go-cache and child-process restrictions; the authorized unrestricted
rerun passed. Documentation formatting, relative links and diff whitespace were
checked separately after the calendar update. No application, infrastructure,
AWS or Trello state changes were made.

## Where to maintain detailed records

| Information                                                       | Maintained source                                                                             |
| ----------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Cost model, workload/rate assumptions, alerts and quotas          | [W1 decision sheet](../architecture/w1-review-decisions.md#monthly-development-cost-estimate) |
| Auth deployment, email and browser verification                   | [Cognito guide](cognito-local-projects.md)                                                    |
| Users/projects deployment and incremental forecast                | [AWS guide](aws-users-projects.md)                                                            |
| Encrypted credentials, audits and cleanup                         | [Protected inputs guide](protected-request-secrets.md)                                        |
| Cloud execution deployment, browser checks and simulations        | [Cloud execution validation](../architecture/cloud-execution-validation.md)                   |
| Static releases, rollback, hosting allowance and reported results | [Hosting validation](frontend-hosting-validation.md)                                          |
| Workflow admissions, target sends, cleanup and evidence limits    | [Assertions closeout](workflow-assertions-pr-review.md) and its retained ledgers              |
| Current delivery estimates and weekly outcomes                    | [Remaining-work plan](../planning/remaining-work.md)                                          |

## Budgets and observed usage

All monetary figures in the archived proposal were planning forecasts or
validation allowances, not observed AWS bills or enforced financial caps:

| Historical allowance or forecast  | Amount                                          | Context                                                                                                                         |
| --------------------------------- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Full modeled baseline, Essentials | $3.19 light / $14.19 heavy per month            | Heavy: 10,000 executions, 30 seconds, 32 KiB request, 1 MiB response; includes one KMS rotation. Original Lite: $3.17 / $14.14. |
| Users/projects incremental        | $0.20–$1.00/month; $2 validation                | 10,000 API calls and mostly idle maintenance, before free tier.                                                                 |
| Protected-request incremental     | $1.10–$2.00/month                               | Includes $1/month KMS key; rotations add version charges.                                                                       |
| Cloud execution incremental       | Approximately $1.42/month; $1–$3 planning range | Light-use forecast before credits.                                                                                              |
| Hosting incremental               | $0.25–$2/month; $1–$3 validation                | Allowances, not observed charges.                                                                                               |

Do not add these forecasts together: the full baseline already models shared
services and later slices can overlap. Preserve original rate dates/assumptions
in the linked decision sheet; this task does not refresh pricing.

Observed quantities in the source below include scanned records/log events,
probe outcomes and cleanup counts. They do not measure monthly AWS spend. Later
[assertions closeout](workflow-assertions-pr-review.md#deployment-and-accepted-genuine-evidence)
records 28 campaign admissions/24 target sends and final monthly application usage
32 including four instructor charges. Those are retained campaign observations,
not a current inventory or billing statement. Actual billing remains unmeasured;
[hosting-cost follow-up](https://trello.com/c/Uu64lmoe) stays open.

## Archived technology deployment and verification entries

### Amazon Cognito

Amazon Cognito – Selected authentication provider beyond MVP; manages login credentials, verification and password-reset emails. Custom React authentication screens share a light/dark theme. Cognito Essentials, 15-minute verified access tokens, one-day rotating refresh and memory-only browser sessions are accepted; reload requires sign-in. Development uses Cognito's default email sender with Kurier branding; hosted signup/login/sign-out and password reset were later reported successful by Davian. Custom verified SES public-signup readiness is deferred. Davian accepted a development-only default-sender exception for the publicly reachable frontend demo at contract 6b026a7; custom SES readiness remains separately required for a general-public launch. Davian accepted hosted development/demo authentication on 2026-10-19; verification/resend, elapsed rotating refresh and expiry cases were not individually reported.

### Amazon CloudFront

Amazon CloudFront – Davian accepted frontend-hosting contract 6b026a7 and approved scoped development/demo deployment including correction 21c24be and the refreshed preview. SST StaticSite now serves https://d2kpq905qyh9zg.cloudfront.net in existing dev-api through dedicated private S3 and signed CloudFront access. Default AWS-hostname TLS limitations and development-only Cognito sender exception remain accepted; custom domain/DNS and general-public signup/SES readiness are separate. Only the four intended public values enter assets. Exact hosted plus deliberate localhost API CORS, ETag/Location/Retry-After exposure, strict CSP/security/cache headers and memory-only sessions preserve static-only CDN and evidence privacy. Manual local-agent pairing and Codex-launched stdio MCP remain unchanged. Complete private-origin release archives include shell, startup.js, 404.html and manifest; shared hashed assets and per-file KV entries are retained for 30 days plus current/previous known-good. Genuine HTTP checks passed private S3 denial, signed CDN/hash/content-type/cache/header/CORS checks, SPA and missing/API/archive 404s. A live missing-asset 502 was corrected within the existing routing Function/KV resources. Real harmless marker A→B→A, remote-only recovery of all A bytes and rollback without rebuild passed; final A release acb951896da16807815751c0509b1f25647656d8a0963932c7fc1f3a8ccac67e. Hosted Chromium signed-out/CSP/readable cross-origin errors and empty persistence passed. On 2026-10-19 Davian accepted the development/demo hosting slice and reported successful signup/login/sign-out, password reset, request execution with redacted values, project deletion and MCP. Davian subsequently completed the deferred hosted local-agent check: a local-agent request to a local echo endpoint succeeded, response fields were redacted and revocation worked. These are operator-reported results; a separate private-LAN target and exact target-send count were not individually reported. Remaining planned cases were accepted without individual verification; elapsed refresh/expiry, detailed tool/revocation, authenticated ETag/Location, full product/cross-engine/accessibility coverage and independent post-test physical cleanup are not claimed. No schema activation, standing compute or paid networking; preserve runtime IAM, capabilities, generation, accounting, AWS Free plan, quotas and concurrency. Allowances remain $0.25–$2/month hosting plus $1–$3 validation, not caps or observed charges. Results and original numbered regression plan: https://github.com/VioletProto/Kurier/blob/main/docs/development/frontend-hosting-validation.md

### SST 4 with TypeScript

SST 4 with TypeScript – Accepted infrastructure framework (ADR 0001). Accepted ADR 0002/baseline 1fba942 defines the Lambda/HTTP API/SQS/DynamoDB/private S3 topology. Historical compute/database spikes remain unchanged. Davian separately authorized and deployed the development users/projects subset: Control with three KEYS_ONLY indexes, ARM64 Go Lambda/HTTP API, required IAM/logs, a stable SSM SecureString cursor key, and minute-scheduled resumable empty-project cleanup. It reuses the existing development Cognito pool and keeps React local. Actual AWS index propagation and conditional transaction checks passed; genuine browser/Cognito-to-cloud validation is recorded separately. Unknown children remain pending. The separately authorized encrypted saved-request development slice now provisions a separate Protected table and annual-rotation stage KMS key, with scoped updates to existing API and cleanup Lambdas. Actual KMS encryption/context, ownership, concurrent edits and scheduled cross-table cleanup passed. Genuine browser creation succeeded after correcting an invalid header name containing spaces. The UI now validates HTTP header-name syntax before submission and explains that API header names must match exactly. Final format/lint/test/build and AWS preview passed. A memory-only active-value audit found no disposable plaintext in Control, Protected persistence or 1,741 cloud log events. Feature commit f94e04edd9e68606f7f1f3a2389baeb7724d2f25 and browser-audit fix 65c546cfa828e93596c72d36c26794986431aa9c are pushed on feat/protected-request-secrets. The first full genuine browser probe reported a combined safety error after expected rejection checks. The validator's empty-storage requirement was corrected to a bounded read-only secret-presence audit, with safe diagnostic counts/flags. Actual Chromium checks accepted unrelated storage and detected disposable leaks across Web Storage, IndexedDB and Cache Storage. Davian reported the corrected genuine Cognito/browser probe PASS: preserve, local/shared replacement, expected 412 stale replacement and 400 revoked/cross-project reuse, responsesSafe=true, storageSafe=true, auditComplete=true and safetyPassed=true. One unrelated local and one session entry were present; no IndexedDB/cache entries. Follow-up active-value AWS audit found no disposable plaintext in 119 Control records, Protected persistence or 2,952 cloud log events. Genuine second-user isolation also PASSED: distinctUser=true and all three request reads plus secret listing returned 404. Original disposable-probe deletion is verified by operator AWS readback: operation completed, Protected partition empty and Control retains only the safe project tombstone. Recovering that deleted probe correctly returns 404; browser deletion-helper output was not reported. Final genuine UI API-key query and whole Text body authoring, input clearing after save/refresh and unrelated rename/preserve PASSED as reported by Davian. Owner browser cleanup returned immediateDenial=404 and scheduledCleanup=completed for project ee5697d1-9677-4b0d-9681-473d1ddbb589 / operation 7c6e756e-2e89-4b69-a071-56cd6a4023e3. Independent AWS readback confirmed that exact operation completed, Protected count=0 and Control only its safe project tombstone. All requested slice implementation/validation is complete; Trello checklist is complete and card is Done. Final pushed branch commit is 7b3879a36a87310be5fef32d2a9ddd897eabdc5f. Restore/PITR recovery and elapsed annual rotation remain untested; execution/outbound HTTP/evidence/workflows/local delivery remain outside scope. Execution workers/queues, evidence buckets and broader cross-store deletion remain later work. Protected-slice incremental forecast is $1.10–$2.00/month before free tiers, including the $1/month KMS key; future rotations add key-version charges. Incremental development forecast $0.20–$1.00/month for 10,000 API calls and mostly idle maintenance before free tier, with $2 validation allowance; not a hard cap or observed spend.

### AWS CloudWatch and budget monitoring

AWS CloudWatch and budget monitoring – Structured sanitized logs, built-in metrics, queue age/DLQ, outbox lag, unknown outcomes, deletion/orphan backlogs and three alarms; logs retained fourteen days. With Essentials at $0.015/MAU for the modeled 2/5 users, forecast light $3.19/month, heavy 10k executions/30 s/32 KiB request/1 MiB response $14.19 with one KMS rotation. This adds about $0.02/$0.05 per month to the original Lite forecasts before free-tier allowances. These are assumptions, not observed bills or a $5 spending cap; billing/alerts lag and pins/rotation/abuse can exceed them.

## Archived cloud execution appendix

The following appendix originally appeared after Faculty Review. Its final
sentence describes scope at that earlier milestone; workflows, pinning,
local-agent, MCP and hosting have later accepted records in the
[architecture index](../architecture/README.md).

Accepted cloud execution development milestone

Davian accepted the single saved-request cloud execution/evidence contract. Its development implementation connects React submission, immediate-return status polling, execution history and safe evidence inspection to the Go Lambda worker and SQS. Request revisions and protected inputs freeze at admission; runtime-only secret resolution, durable outbox, fenced dispatch, no automatic replay after ambiguity and atomic evidence publication preserve the accepted reliability model. Source-bound idempotency separates submission retries from deliberate reruns. Authority: Accepted cloud execution contract. Results: Cloud execution validation record.

The scoped development AWS deployment is verified with a private evidence bucket, SQS/DLQ and worker, API and maintenance updates. Standard-mode queue maximum concurrency two and batch size one replace reserved concurrency; SQS is the worker's only configured execution trigger. The Free plan and quota ten remain unchanged, with no provisioned concurrency or pollers. The incremental light-use forecast is approximately $1.42/month before credits, using a $1–3 planning range.

Local fault simulations and actual AWS verification are distinguished in the validation record. Davian's genuine browser probe passed execution/evidence, secret containment, retry/rerun identities, history, immutable reopening and browser storage checks; independent server evidence/log audits passed. Browser project deletion denied access immediately and scheduled cleanup completed with only a Control tombstone, no Protected records or S3 evidence and a registered residual sweep. Davian confirmed response wrapping and all three genuine browser failure checks: failed HTTP 500, completed HTML with safe body omission and no popup, and failed timeout with HTTP unavailable and the explicit unknown-outcome warning. Fresh read-only AWS inspection of all three immutable captures passed after session renewal: terminal state, HTTP status and outcome matched Control execution/snapshot metadata, and stored lengths and SHA-256 checksums matched S3. No response bodies or sensitive values were printed or written to inspection files. All requested browser checks for this slice are complete; account throttling and dispatch/crash fault injection remain simulations, and negative IAM checks remain policy simulations. Workflows, extraction, pinning UI, local-agent execution, MCP and frontend hosting remain future work.
