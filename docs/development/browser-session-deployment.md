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

## Password-manager focus fix

On 2026-11-12 Davian reported that returning from Bitwarden cleared filled sign-in
fields. The broker's unconditional window-focus handler remounted the auth form.
Fix `d65aef3` limits focus revalidation to displayed signed-in workspaces; startup,
explicit session-change events and BFCache revalidation remain. It was verified
and deployed through the existing SST workflow, with no infrastructure-setting
changes. Release:
`fc68412e0102987fc5ce9b4784de05aa5156c0a542126e042ee729d6464cee3f`.

Full repository verification passed (94 frontend tests, 16 infrastructure tests,
tooling, build and Go tests/vet), as did both HTTPS Chromium fixtures. A real
hosted Chromium probe filled synthetic fields and repeatedly returned window
focus: field values and form identity were preserved, no extra restore requests
occurred, and no page errors occurred. Live CloudFront security probes and public
asset hashes passed; stage marker, Free plan and concurrency were preserved.
The Bitwarden extension itself was not installed in the runner. No real vault
values were used, submitted or saved. This fix does not close the remaining
authenticated Cognito/expiry gates above.

## Workspace focus preservation

Davian authorized deployment, testing, merge and repository cleanup on 2026-11-15
after reporting lost workspace progress when returning from another window.
Implementation `fe051f20cfca3870e58d1b4864545eb4f0d23501` changes signed-in
focus checks to validate the current binding in the background. Release
`4ea92660947db4b0166123a818ace5e55503f383d5b800e63d7713ce6c266f25`
is deployed at **https://d2kpq905qyh9zg.cloudfront.net** on `dev-api` in
`us-east-2`, account `747336059622`, using verified non-root `davian-admin`.

The full SST preview proposed unrelated Go Lambda package replacements because
their embedded VCS metadata changes with the repository revision. That plan was
rejected. Supported SST 4.17.1 `diff` and `deploy --stage dev-api --target
FrontendWeb` limited the reviewed/applied plan to the existing static asset
uploader, asset lookup keys and cache invalidation. No resources were created and
no runtime, IAM, database, broker settings or bootstrap changes were applied.

Post-deployment readback confirmed unchanged API/cleanup/worker/controlled-endpoint
and broker code hashes, runtime configuration and IAM. The active version21,
savedRequests11/MCP2 marker and recovery generation remained unchanged, along with
bootstrap5, the Free/ACTIVE plan, account/unreserved concurrency10, worker
maximum2/batch1, minute maintenance and CloudFront distribution configuration.

Verification completed:

- Full `npm run verify`: formatting/gofmt, ESLint, 104 frontend tests,
  16 infrastructure tests, tooling suites, production build and Go tests/vet.
  PR CI also passed, including HTTPS Chromium session fixtures and the
  Go/DynamoDB/browser integration job.
- Actual CloudFront public files matched the release manifest hashes. Security
  headers/CSP, private S3 denial, deep links, static missing paths, CORS and the
  broker's JSON 401/403/no-store denial cases passed.
- Genuine hosted Chromium signed in through Cognito SRP and the broker with one
  disposable identity and created one empty disposable project. Repeated focus
  events preserved the selected **Project settings** tab, unsaved rename text,
  original mounted input and keyboard focus, with no product-data reload.
- Simulating a broker connection failure preserved the tab and draft while
  displaying the connection notice. The next focus successfully checked the real
  broker, removed the notice and preserved the same draft/input. No page errors
  or Web Storage, IndexedDB or Cache Storage entries were observed.

The test signed out, deleted the disposable project through the product API and
confirmed scheduled cleanup left only its deleted project tombstone and no
Protected children. Conditional removal of the disposable application-user and
identity records, Cognito deletion and independent readback completed. Passwords,
access tokens and cookies stayed in memory or the normal encrypted broker store;
no screenshots, traces, browser-state exports or credential transcripts were saved.

The connection failure is simulated; successful sign-in and checks use real
hosted services. This bounded test does not establish natural 15-minute refresh,
fixed one-day expiry or additional browser-engine coverage. Reload/history
privacy boundaries remain unchanged; drafts are preserved only while the page
stays mounted.
