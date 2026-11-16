# Browser teardown and transport stop correction

The [tooling card](https://trello.com/c/F47iWJY8) concerns the operator's
Playwright browser driver, Python request-meter proxy and Node test coordinator.
These tools automate assertion editing, charge outbound attempts and bound a
validation campaign. They are separate from the deployed application.

During the original live assertions campaign, nine read-only API OPTIONS
requests failed around context closure. The old meter recorded them without
stopping later work. OPTIONS is the browser's automatic cross-origin permission
request; this issue was discovered in live testing, not a deployment readiness
check. Cancellation remains a plausible explanation, not proven historical
attribution. The original functional acceptance remains intact.

## Corrected behavior

- A transport error or missing HTTP status on a read now records the outcome
  and latches the meter's stop. OPTIONS has no exemption. Uncertain writes retain
  their stronger no-replay stop. Known HTTP read responses, including the
  existing bounded 503 polling policy, remain distinct from transport failure.
- Requests admitted before a stop may finish and record their outcomes. Their
  charges remain. The meter refuses all later admissions, including requests
  permitted by the selected write phase. It never automatically retries a write.
- Both assertion coordinators check the meter before and after each action.
  The shared budget rejects failed, duplicate, missing-status and unfinished
  outcomes even if a historical meter omitted a stop event. Application transport
  rechecks after authentication and immediately before dispatch. The existing
  action-loop failure handler records the stop and prevents ordinary continuation.
- The browser guard tracks API and Cognito requests across the context, through
  completed response bodies. The driver drains post-save reads, checks the guard
  and meter, closes the context and checks again. A teardown-time
  `net::ERR_ABORTED` is counted separately in safe diagnostics but remains a
  failure. An abort outside teardown is not attributed to shutdown. No URLs,
  credentials, request bodies or raw browser/SDK errors are added to diagnostics.

## Cleanup and failure handoff

Healthy campaigns still reserve the final 20 minutes, 60 API attempts and 200
AWS attempts inside the existing absolute ceilings for normal cleanup. Entering
cleanup does not excuse a failed or unfinished request. A latched meter remains
closed even for cleanup and cannot be unlocked by changing phases or asking the
coordinator for `stopped-cleanup.mjs`.

On failure, stop ordinary actions, retain the ledger and safe browser counters,
and let already admitted requests settle. Missing outcomes, particularly writes,
remain unresolved; do not manufacture completion or issue replacement keys.
Review known project/identity IDs and outcomes before separately authorizing
bounded normal cleanup. The existing cleanup-only launcher still accepts only
its narrowly defined blocked-write parent proof and exact resource scope; this
change does not broaden it to read failures or unknown writes. A new cleanup
scope must preserve original charges, deadline and uncertainty guards. No
automatic restart, campaign replay or fresh target sends are introduced.

## Validation

Local deterministic checks cover completed-body draining, genuine read failure,
pre-close versus teardown abort diagnostics, requests racing with shutdown,
in-flight outcomes after a stop, stop propagation across authentication, and
zero subsequent writes or replay. These run in `npm run verify`.

The following integration checks use the existing network-isolated harness,
real Chromium/mitmproxy and controlled localhost origins with fake credentials:

```sh
python3 scripts/attempt-meter/test_browser_authoring.py
python3 scripts/attempt-meter/test_browser_authoring.py old-close
python3 scripts/attempt-meter/test_read_failure_transport.py
python3 scripts/attempt-meter/test_transport.py
```

Observed results:

- Corrected author/reopen/edit: three local PATCHes, all twelve decimal strings
  preserved, zero transport errors.
- Retained old driver: first edit followed by read transport failures and a
  latched stop; one local PATCH total and no admitted attempts after the stop.
- Dropped upstream read: one charged attempt; proxy and actual shared coordinator
  block subsequent reads/writes; zero forwarded writes or replays.
- Existing transport suite: nine SDK retry/pagination attempts, HTTP/2/browser
  metering, bounded 503→200 coordinator polling, redirect rejection and lost-write
  acknowledgment protection passed.

All are local/fixture results, with zero live AWS calls or target sends. The old
campaign was not rerun. No deployment or AWS changes are needed to install this
operator-script correction. `npm run verify` passed: repository formatting,
ESLint, 98 frontend tests, 16 infrastructure tests, tooling suites, production
build and all nine Go modules' tests/vet. The optional DynamoDB activation cases
remain skipped without their integration endpoint. Changed Python files also
passed Ruff format/core lint. The first sandbox run could not spawn Git for a
historical-release test; the permitted full rerun passed.

The frozen original ledger remains SHA256
`66050c4e8d224556dd65b5a96401039a2d4b8af1f79a59778b1ffbb5be97652f`.
The retained original live driver remains SHA256
`14c6e1e402ec1b7e45d005f6c00502b35c02018f3fc59c2fd6120981b8193017`.
Neither its failures nor their unknown historical causes are reclassified.

## Authorized hosted continuation

On 2026-11-15 Davian requested deployment, testing, merge and task-branch cleanup.
The operator scripts themselves need no application deployment. A signed-out
hosted check then exposed a frontend session-client response-body leak: the
startup session check returned 401, the sign-in form appeared, but Chromium kept
the unread response pending and the new guard correctly refused teardown.

Three pre-deployment smoke attempts are retained separately. The first two
timed out awaiting navigation network idle; the third used DOM readiness and
identified the pending `/api/v1/browser-session` body after a successful explicit
API read. These were not transport errors or uncertain writes. All attempts
were signed out, with no identity/project creation or target execution.

A single hosted diagnostic cycle applied the proposed error-body drain inside
the test browser, without changing the deployed application. It passed with
zero pending requests, transport failures or close cancellations. This is a
test-browser intervention, not deployed-fix evidence. The matching regression
failed on the old session client and passed after the fix: consume non-success
response bodies before throwing, within the existing request deadline. The
409 path already consumes its JSON body and remains unchanged. No error body
is logged or persisted. Full repository verification passed again with 99
frontend tests; both local HTTPS Chromium session scenarios passed. This new
session-client issue does not attribute the historical October OPTIONS failures.

### Deployment and hosted results

Source `8707e0a3fc586d17b67f6eb33e97c8c8659199a1` was deployed to the existing
`dev-api` frontend in `us-east-2`, after verifying account `747336059622` and
non-root `davian-admin`. The normal full-stage preview included unrelated Go
rebuilds, so SST's supported `--target FrontendWeb` was used for both preview and
deployment. The [targeted plan](browser-meter-frontend-preview.json) contains
only existing routing keys, uploaded frontend assets and invalidation updates.
No Lambda, IAM, table, bootstrap or schema mutation was needed. The initial
preview lacked the existing public Cognito identifiers and failed before any
deployment; the corrected targeted preview succeeded.

Deployed release:
`27f4c3361da468373356e15c8e8f8cda1dcd0b98993ab1beaf8192fb72d3da36`.
The [public manifest](browser-meter-frontend-artifact.json) and
[genuine HTTP validation](browser-meter-frontend-http.json) verify all five app
files, cache/security headers, CORS, private-bucket denial, SPA behavior and
missing-file/API errors. The [preservation record](browser-meter-frontend-preservation.json)
confirms all five Lambda code/configuration snapshots, active marker version 21,
stage quota, recovery/capability marker hash, bootstrap v5, Free/ACTIVE plan and
concurrency 10 remain unchanged.

Three fresh, signed-out hosted Chromium cycles then passed **without the
diagnostic fetch intervention**: real startup session 401, cross-origin OPTIONS
and unauthenticated API GET 401, completed response bodies, empty Web Storage,
zero pending requests, zero transport failures and zero close cancellations.
The [safe result](browser-meter-hosted-smoke.json),
[meter ledger](browser-meter-hosted-attempts.jsonl) and
[retained driver](browser-meter-hosted-driver.py.txt) record nine API attempts
(including three OPTIONS) and twelve static requests. The adapter is read-only,
permits only the fixed public frontend/session/project-read routes, rejects
credentials/bodies and isolates the browser network through the actual corrected
meter. It uses a fresh three-minute verification window, a 20-API limit and a
30-static runtime limit (the generic meter-created entry records its initial
200-static default before the adapter lowers it).

Earlier failed attempts remain in
[initial](browser-meter-hosted-initial-attempts.jsonl),
[second](browser-meter-hosted-second-attempts.jsonl) and
[third](browser-meter-hosted-third-attempts.jsonl) ledgers. The browser-intervention
diagnostic is separately recorded in its
[result](browser-meter-hosted-drain-proof.json) and
[ledger](browser-meter-hosted-drain-proof-attempts.jsonl). Across all five smoke
windows, the cumulative observed total is 17 API and 28 static attempts. Their
fresh clocks do not reset or reopen the old assertions campaign. No accounts,
projects, target sends, authenticated sessions or AWS resource writes were
created by these browser smoke tests. Post-deployment inventory and the separate
HTTP artifact/security probes are additional genuine read-only checks outside
those smoke ledgers. The deployment itself performs the three reviewed frontend
updates. Authenticated workflow authoring and genuine live fault injection were
not rerun; negative failure/no-replay behavior is established by the local
isolated tests.
