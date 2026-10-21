# Final assertions attempt: coordinator integration

Davian accepted `f3a698d565fa29e99cd7f89517252994c32e44ef` and authorized
one final attempt after actual coordinator integration and local transport,
metering, redirect and cleanup-reserve verification. The historical GET503 cause
remains unestablished. This changes operator tooling only; the application source,
ZIPs and immutable release manifest remain unchanged.

The live coordinator now uses `createCoordinatorTransport` for its actual native
fetch exchanges and the accepted `createCoordinatorRunReader` for acknowledged
workflow-run polling. Only explicitly retryable GET503 responses receive bounded
backoff. Submission, stage writes and uncertain outcomes never use this retry
path. Redirects fail closed. Shared logical counters are fsynced before requests;
the isolated proxy independently meters SDK attempts, retries and pagination.
Actual HTTP integration exercises503→200 and a redirect, with zero admissions,
target sends or AWS calls. CLI/Go/Node retries/pagination, native fetch, Chromium,
network-bypass denial and lost-write acknowledgment/no-replay also pass locally.
Cleanup reserves stop work at100minutes,440API or1300AWS attempts. Cleanup alone
can use the remaining20minutes,60API and200AWS attempts, within absolute limits.
A latched transport stop remains fatal. After a coordinator stop, only an explicit
cleanup action is dispatchable; campaign execution cannot resume automatically.

The final allowance carries forward four consumed steps/four sends and four
already cleaned identities/projects. Exactly two additional identities/projects
are allowed (six cumulative). A–L admits24new steps:28cumulative, with **zero
reserve submissions**. Expected new target sends20, cumulative24; unexplained
extras stop execution. New window120minutes including20forcleanup;500API calls
including240polls;1500AWS attempts; additional$1procedural allowance. Every AWS
attempt is charged conservatively. Prior known48API/4polls and second-attempt574AWS
charges remain historical; first-attempt read total and all actual billing totals
remain unknown. These limits supersede only the historical campaign allowance
in the unchanged release manifest. They do not reset accounting.

Execution requires fresh preflight/equivalent eleven-resource preview, genuine
legacyA seed, settlement proof and separate fresh close/activation approvals.
The accepted e702 settlement guard remains unchanged. Deployment uses clean
source68d9f218225c7092a1e4fca0b66a21421d862a22 and pinned artifacts; no IAM,
generation, quota or concurrency change is authorized. No automatic reopening,
HTTP replay, broad fault injection or PITR exercise is allowed.

Local verification: full `npm run verify`;10meter budget tests;18Node polling and
coordinator tests; isolated actual CLI/Go/Node/browser transport regression;
Python formatting and core lint. Trello synchronization currently fails with
“MCP authentication required.” Runtime results will be published separately.
