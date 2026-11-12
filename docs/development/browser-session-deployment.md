# Browser session broker deployment

Davian authorized the scoped development deployment on **2026-11-12**, after the
implementation and preview review. Deployed implementation source:
`3695874c8e0aacdd5157671468b12e090134397c`, branch `feat/browser-session-broker`.
This records development/demo hosting, not a general-public signup launch.

The hosted UI is **https://d2kpq905qyh9zg.cloudfront.net**. Its public artifact is
`bc4a44a609f5f1d7bcac1137bf00377c367197c355e761fc78c7cf97eea0a004`, with the
broker capability enabled. The [contract](../architecture/browser-session-contract.md)
and [implementation guide](browser-session.md) define the behavior and local tests.

## Scope and preservation

Used the existing supported `npm run sst:diff:api` and `npm run sst:deploy:api`
commands, SST 4.17.1, region `us-east-2`, account `747336059622`, verified non-root
identity. Existing bootstrap v5, Ohio assets/state/ECR and existing passphrase
metadata were verified before preview; no bootstrap migration was required.

The Sessions table, broker Lambda/role, five HTTP API routes, minute cleanup and
CloudFront policies were created. The existing distribution/CSP and frontend were
updated; global error body/status replacement was removed. The reviewed existing
Go ZIP replacements were applied. Worker/controlled-fixture executable sections
matched their deployed predecessors; read-only data matched after VCS metadata
normalization. API/cleanup share the task's exact HTTPS-loopback CORS amendment.
No session dashboard, new auth provider or deployment framework was added.

Post-deployment readback confirmed:

- Control stage marker unchanged by full canonical hash comparison: active,
  version 19, savedRequests10/OpenAPI1 and retained recovery generation.
- Free/ACTIVE plan and total/unreserved Lambda concurrency 10 preserved. The
  broker has no reserved concurrency.
- Broker Active with successful update and ARM64 architecture; code SHA-256
  `oe4AJEBVXy4RsWakCFwmIC13IvijE6oYfZ0pV4boSkM=`.
- Sessions table ACTIVE, on-demand, TTL `ttl` ENABLED, PITR DISABLED.
- CloudFront Deployed; both broker behaviors have zero cache TTL and no static
  viewer function. Custom-error entries have no response body/status replacement.
- Cleanup rule enabled with `{maintenance:true}` targeting the broker. Multiple
  actual scheduled sweeps advanced the durable maintenance cursor.
- Every public frontend file matched its deployed manifest checksum.

The normal SST update publishes resources within one deployment; there was no
zero-downtime claim. Readiness was established only after distribution propagation,
broker routes and frontend had all completed and the following live gates passed.

## Actual runtime verification

`scripts/check-browser-session-hosting.mjs` passed through CloudFront:

| Request                   | Actual result      |
| ------------------------- | ------------------ |
| Missing session           | JSON 401, no-store |
| Foreign Origin            | JSON 403, no-store |
| Missing custom header     | JSON 403, no-store |
| Navigation Fetch Metadata | JSON 403, no-store |

No response became `404.html` or a CloudFront cache hit. Direct API-origin broker
access returned 403. The static product API path and missing hashed asset returned 404. These are live CDN checks, not a configuration-fixture substitute.

Bounded disposable-proof probes through CloudFront also passed: cancellation
before bootstrap, identical cancellation retry, unknown valid-format cookie
denial and empty-session sign-out. Actual DynamoDB readback confirmed the
cancellation tombstone contains proof hashes, not the disposable proof values.
Sign-out returned a scoped HttpOnly/Secure/SameSite=Strict cookie clear with
Max-Age=0. Probe proofs stayed in process memory, were not printed or written to
test artifacts, and the cancellation tombstone expires through normal TTL.

These probes exercise real broker Lambda, DynamoDB/IAM, Control generation guards,
cookie response forwarding and cleanup. They do not establish successful Cognito
session bootstrap or KMS encryption/decryption of a refresh credential.

A bounded read of 46 messages from the broker's configured SST log group found
no runtime errors or credential-field logging; message contents were not printed
or stored. The paginated window was not exhausted, so this is not a full log audit
or evidence about real account credential handling.

Credential-free Chromium navigation and reload against the actual hosted site
passed: sign-in form and one-day-session wording visible, no private workspace,
empty Web Storage and no page errors. The first attempt timed out; configuring
the runner's proxy resolved connectivity. No browser account session, traces,
screenshots or browser-state files were created by this check.

Final post-deployment `npm run verify` passed: formatting, ESLint, 92 frontend
tests, 16 infrastructure tests, tooling suites, production build and configured
Go tests/vet. Generated SST resource types now include the broker and Sessions
table. Documentation-only closeout changes were formatted separately.

## Remaining validation

Real account sign-in, reload/reopening and two-tab behavior need an operator browser
check. Natural 15-minute refresh/rotation, fixed one-day expiry and real Cognito
revocation are still unperformed. Positive live KMS/context/IAM enforcement during
authenticated handoff and Firefox/WebKit/mobile Safari also remain unverified.
The existing local race, real DynamoDB Local and simulated HTTPS Chromium results
are recorded separately in the implementation guide. The API still has the accepted
offline access-token revocation window; deployment does not add immediate denial.

The delivery card remains Doing until those required browser/Cognito gates are
completed or explicitly accepted with their limits. No token-bearing HAR,
screenshots, browser storage exports or request logs should be collected.
