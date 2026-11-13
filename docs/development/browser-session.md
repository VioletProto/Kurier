# Browser session broker implementation

Davian authorized scoped implementation after the
[clarified session contract](../architecture/browser-session-contract.md).
This record separates code/local checks from deployment and actual Cognito/
CloudFront/browser results. Davian subsequently authorized deployment; the
[deployment record](browser-session-deployment.md) documents the completed
2026-11-12 SST rollout and live CloudFront/storage checks. Historical preview
boundaries below precede that authorization.

## Implemented scope

- Go refresh broker in `services/api/internal/browsersession`, with a dedicated
  runtime in `services/api/cmd/browser-session`. Cognito SRP remains in the browser;
  refresh/cached access credentials are envelope-encrypted server-side.
- Dedicated TTL Sessions table, strong Control stage/recovery-generation checks,
  conditional bootstrap receipts and cancellation, durable refresh leases, bounded
  rate limits and scheduled revocation cleanup. No five-session cap/dashboard.
- Authenticated bounded bootstrap replay with the original browser-generated
  sessionProof reissues the same HttpOnly cookie. The receipt binds request and
  proof hashes; it does not need recoverable plaintext cookie storage.
- Opt-in hosted frontend build flag `VITE_KURIER_SESSION_BROKER=true`, transient
  Amplify storage adapter, startup restore/retry, explicit sign-out and offline
  intent, account-change binding checks and BFCache fencing. Existing public
  release descriptors without the flag remain valid for historical releases.
- Exact base/child CloudFront broker behaviors with no caching/static function,
  purpose-separated private origin proof, cookie/header forwarding and self CSP
  connection permission. Global error body/status replacement is removed. Static
  viewer 404 handling remains isolated.

Source credential material, cookies, bodies and raw provider errors are never
logged. Runtime errors are fixed public messages. Fixture credentials are synthetic.
The separate MCP/local-agent credentials and product evidence schemas are unchanged.

## Local commands

Normal repository verification is `npm run verify`. It includes frontend tests,
infrastructure tests/build, Go tests/vet and existing tooling suites. The new broker
has no added third-party dependencies. Run storage integration against the existing
repository DynamoDB Local service:

```sh
docker compose -f compose.dynamodb.yml up -d
cd services/api
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 go test -race -tags=integration -count=1 ./internal/browsersession
```

The test creates/removes uniquely named tables; it does not access AWS. A generic
`go test ./...` does not run this integration-tagged case.

The browser fixture uses a local HTTPS certificate/key generated outside the
repository (for example, disposable files under `/tmp`). Its configuration disables
screenshots, video and traces, and never writes browser credential state to disk:

```sh
cd apps/web
KURIER_LOCAL_HTTPS_CERT=/tmp/kurier-session-browser.crt \
KURIER_LOCAL_HTTPS_KEY=/tmp/kurier-session-browser.key \
npx playwright test --config playwright.session.config.ts
```

This exercises Chromium against simulated broker/product routes; it is not a
CloudFront/Cognito integration result. For manual development with real Cognito,
run the local broker with `KURIER_SESSION_LOCAL=true`, `KURIER_STAGE=local`, an
explicit loopback DynamoDB endpoint, session/control table names, a securely
supplied 32-byte base64 `KURIER_SESSION_ENCRYPTION_KEY_BASE64`, configured Cognito
issuer/client and `KURIER_SESSION_FRONTEND_ORIGIN=https://localhost:5173`.
The existing local Control stage must be initialized/active first. The broker
creates its local session table only; it never creates AWS resources. Start with
`go run ./cmd/browser-session` from `services/api`.

Start Vite with `VITE_KURIER_SESSION_BROKER=true`, the same HTTPS certificate/key
variables and optional `KURIER_SESSION_PROXY_TARGET=http://127.0.0.1:8081`.
The proxy accepts only explicit loopback HTTP ports and preserves production
Secure-cookie flags. Existing `dev:auth`/`dev:cloud` memory-only modes remain useful;
they do not proxy hosted HttpOnly cookies into localhost. Hosted Cognito tests
must use disposable nonproduction accounts and keep credentials out of artifacts.
For a local product API, explicitly configure its frontend origin as the matching
HTTPS loopback origin. Local CORS now permits that exact HTTP or HTTPS loopback
origin, with a port, while retaining Bearer authentication and no cookie credentials.

## Deployment and review

Use the existing supported `npm run sst:diff:api` / `npm run sst:deploy:api`
commands. No new deployment framework is introduced. Before preview, verify the
intended account/non-root identity, Ohio bootstrap v5 and existing state/passphrase,
Free plan and unchanged concurrency. Supply the existing public Cognito identifiers
and exact loopback origin from current outputs. Never emit/decrypt secret values
in a review artifact. The broker derives its CloudFront origin proof from the
existing cursor SecureString using a purpose-separated HMAC; it is marked secret
in infrastructure state and never exposed as a frontend value/output.

Review the Sessions table, broker Lambda/role/policy/log group, broker routes,
minute cleanup rule/target/invoke permission, two CloudFront policies and scoped
distribution/CSP/frontend changes. Product Control/Protected/evidence, stage
capabilities/recovery generation, Free plan and concurrency must remain unchanged.
The broker adds metered Lambda/API, DynamoDB, KMS and dynamic CloudFront usage;
there is no standing compute or additional KMS key. Actual usage is not measured
by local tests.

Current read-only AWS inventory confirmed intended non-root account, existing
Ohio bootstrap/assets/state, existing stage passphrase metadata, Free/ACTIVE and
10 total/unreserved Lambda concurrency. Control is active at version 19,
savedRequests10/OpenAPI1 with the retained recovery generation. The first SST diff
failed on the nested Go handler import path; no product deployment occurred.
The corrected supported SST preview succeeded. It built the Linux ARM64 broker
using the pinned SST runtime's full-import-path handler convention, without a new
module or build framework. It proposes the scoped broker resources and hosting
changes above, plus four existing Go function ZIP replacements (API, cleanup,
worker and controlled fixture). Their runtime configuration is unchanged in the
preview. After reauthentication, read-only in-memory comparison confirmed identical
worker/controlled-fixture executable `.text`; their read-only data also matches
after normalizing VCS metadata. Their other changed sections are build identifiers
and Go build information. API and cleanup share the API main package and include
the task's exact-loopback HTTPS CORS support; their executable sections therefore
change. No unrelated worker/fixture runtime-code change is implied by those ZIP
replacements. Refresh identity/inventory and preview against the final source
commit before deployment. No deployment was performed.

Deploy only the reviewed source/preview. Infrastructure and frontend publishing
must be coordinated; do not enable the browser flag against missing broker routes.
Then perform the read-only error-preservation gate through the distribution:

```sh
node scripts/check-browser-session-hosting.mjs https://d2kpq905qyh9zg.cloudfront.net
```

It sends no credentials and prints only fixed scenario names/pass status. Missing
session must remain JSON 401; foreign origin, missing custom header and navigation
must remain JSON 403, with no-store and no cache hits. This specifically detects
the old distribution-wide 403 → 404.html replacement. Also verify direct-origin
denial, static blocked API/missing assets, cookie retention and no cross-user cache
reuse. Fault fixtures cover 429/503 preservation without inducing production outages.

## Validation boundaries

The backend fixture/race suite and real DynamoDB Local integration passed, including
bootstrap replay, CAS atomicity, rotate-vs-revoke, cancellation before creation and
stage/recovery denial. Frontend tests/build/lint and infrastructure tests passed.
The HTTPS Chromium fixture passed reload, new tab, context reopening, HttpOnly
unreadability, empty Web Storage, direct bearer API/no-cookie behavior and sign-out.
Final `npm run verify` passed after resuming on 2026-11-12: formatting, ESLint,
92 frontend tests, 16 infrastructure tests, existing tooling suites, production
build, and all configured Go tests/vet and spike test-binary builds. The broker
adds 16 Go unit/security tests and two Lambda-adapter tests; targeted race checks
and its actual DynamoDB Local integration passed before the usage-limit pause.
Optional endpoint-dependent cases in existing tooling suites remain skipped by
the generic verification command and do not represent live AWS tests.

The source-address safety throttle may be shared by viewers behind a CloudFront
edge. It is not presented as an accurate per-browser identity. Separate authenticated
identity/session limits remain; no untrusted forwarded-address header is used.

The [deployment record](browser-session-deployment.md) now verifies actual
CloudFront JSON error preservation, direct-origin denial, public assets, scoped
storage/cancellation/sign-out probes and scheduled cleanup. Remaining checks: real Cognito
SRP handoff/refresh/revocation; elapsed 15-minute rotation and one-day expiry;
actual reopen/privacy restrictions; actual KMS/IAM/context enforcement; sanitized
log inspection; Firefox/WebKit/mobile Safari. Synthetic clocks, local AEAD and
mocked provider/API routes do not establish these results. No live deployment or
general-public signup launch is authorized by a passing local fixture.
