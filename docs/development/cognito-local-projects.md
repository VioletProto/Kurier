# Cognito browser / local users and projects

This slice connects the shared React light/dark auth screens to Cognito and the
existing seven [users/projects routes](../architecture/users-projects-contract.md)
to the loopback Go API and DynamoDB Local. `/api/v1/users/me` is unchanged.
Only development authentication is eligible for AWS deployment. There is no
hosted web/API/database, execution worker, protected table or evidence writer.

## Session choice and development email

The accepted Lite + rotating refresh combination is unsupported by AWS's
[current feature table](https://aws.amazon.com/cognito/pricing/): rotation
requires Essentials or Plus. Davian accepted **Essentials with one-day rotating refresh** for this slice.
Set `KURIER_AUTH_TIER=ESSENTIALS`; Lite is no longer the accepted auth tier.
The intended account is required; root and a different account fail closed.

Davian authorized Cognito's default email sender for development, with Kurier
in the subject/message. Custom verified SES sender configuration is deferred;
the accepted SES-backed public-signup recommendation remains future work.
The inspected Ohio SES account is sandboxed (200/day, 1/second), with no verified
identity. Cognito default email uses AWS's own sender configuration and service
limits instead of this account's custom sender. The [default quota](https://docs.aws.amazon.com/cognito/latest/developerguide/quotas.html)
is 50 email messages/day per AWS account. Do not claim SES sender/domain,
DKIM/SPF/DMARC, production-access or independent recipient delivery verified.
The developer supplies signup/reset codes in the browser; passwords and tokens
must not be pasted into chat, CLI arguments, logs or committed fixtures.

The public app client uses SRP, no secret, no hosted OAuth callback/implicit flow,
15-minute access/ID tokens, one-day refresh, token revocation and enumeration
protection. Recovery is verified email only; passwords require 12 characters,
uppercase/lowercase/numbers/symbols. No remembered devices, SMS/MFA, triggers,
domain, custom sender or hosted product resources are added.

`aws-amplify` 6.22.1 / Auth 6.21.1 supports SRP and
`GetTokensFromRefreshToken`. AWS's [refresh API](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-the-refresh-token.html)
works with rotation enabled or disabled. A rotating token keeps the original
session's remaining lifetime. The ten-second rotation grace period allows SDK
retries when a refresh response is lost, at the cost of accepting the previous
refresh token during that brief window; it does not extend the one-day session.
See AWS's [retry grace configuration](https://docs.aws.amazon.com/cognito-user-identity-pools/latest/APIReference/API_RefreshTokenRotationType.html).
`ALLOW_REFRESH_TOKEN_AUTH` is deliberately absent.
All SDK session storage uses shared memory before any authentication call;
reload requires sign-in. Concurrent refreshes share one promise. An in-flight
refresh settles before sign-out clears storage; generation checks deny its late
result. Idle sessions check tokens every minute. Reads refresh once on 401;
writes are not automatically resubmitted. Sign-out clears the UI and tokens;
remote revocation is best effort. Offline API JWT verification still has the
accepted access-token lifetime/clock-tolerance revocation lag.

## AWS development setup

Use Node 22+, Go 1.26+ and Docker Compose. SST is pinned to 4.17.1 and the
installed AWS provider 7.40.0 supports `userPoolTier` and refresh rotation.

Before any `sst diff` or deploy, verify the intended non-root identity and
inspect `/sst/bootstrap`: SST preview can itself create bootstrap resources.
This session observed account `747336059622`, non-root `davian-admin`, an existing
version-5 Ohio bootstrap with state/asset buckets and ECR registry. No auth
pool existed before this task. Deployment created pool `us-east-2_qTDZQT1FE` and
public client `5jg9sb38i6ae9c0adm5rtdfkno`; readback confirmed Essentials, SRP only,
no client secret, 15-minute access/ID tokens, one-day rotating refresh,
revocation/enumeration protection and the default branded email sender. The
initial zero-second rotation grace was subsequently increased to ten seconds
for refresh retry reliability; SST deployment and AWS client readback confirmed
the ten-second setting. A lost refresh response was not fault-injected against
live Cognito.
The existing version-5 bootstrap was reused. SST added
`/sst/passphrase/kurier/dev-auth` (SecureString) and app/stage state/link objects
in its existing state bucket. No new IAM/custom SES resources were planned. Do not modify historical spike stages or unrelated resources.

After the session decision is accepted:

```sh
npm ci
aws sts get-caller-identity
export KURIER_AWS_ACCOUNT_ID=747336059622
# Set only the tier explicitly accepted for this development slice:
export KURIER_AUTH_TIER=ESSENTIALS
npm run sst:diff:auth
# Inspect: development pool/client and necessary bootstrap only.
npm run sst:deploy:auth
```

The auth scripts verify AWS CLI identity and export the current CLI session only
into SST process memory, allowing the CLI login cache to work with SST's SDK.
No AWS credentials are printed or saved.

The stage is `dev-auth`, app `kurier`, region `us-east-2`. Pool deletion protection
and SST retain/protect prevent accidental account loss. The default sender needs
no custom SES identity. Public pool/client/issuer outputs go to ignored
`.sst/outputs.json`. Local helpers reject outputs from the historical spikes.
Never export SST state/secrets into the repository.

## Local startup

After auth deployment has produced `.sst/outputs.json`:

```sh
docker compose -f compose.dynamodb.yml up -d
npm run dev:api -- -init-local
```

Run in separate terminals:

```sh
npm run dev:api
```

```sh
npm run dev:auth
```

Open `http://localhost:5173`. The helpers read public Cognito outputs, point the
API at `http://127.0.0.1:8000` and React at `http://127.0.0.1:8080`, and generate a
private random cursor key only in the API process environment. On API restart,
old cursors expire operationally; refresh the first list page. To retain cursor
validity, provide a private `KURIER_CURSOR_KEY_BASE64` environment value securely.
Neither helper writes credentials or tokens to a file.

`KURIER_FRONTEND_ORIGIN` can select another explicit HTTP loopback origin/port;
use that exact URL in the browser. `PORT` controls the local API port.
CORS allows only that origin, GET/POST/PATCH/DELETE, and Authorization,
Content-Type/If-Match; it exposes ETag, Location and Retry-After, uses no cookie
credentials and permits unauthenticated preflight only. Actual requests still
pass Cognito JWT and ownership checks. CLI requests without Origin still work.

For manual startup using an existing pool, follow [API setup](local-ownership.md)
and supply `VITE_COGNITO_USER_POOL_ID`, `VITE_COGNITO_CLIENT_ID` and
`VITE_API_URL=http://127.0.0.1:8080` through the frontend environment. Config values
are public; do not put passwords/tokens in Vite variables.

## Browser behavior

Signup, verification/resend, sign-in, reset request/confirmation and sign-out
share accessible labels, status/alert regions, disabled loading controls,
keyboard focus and a system-default light/dark theme. Successful code requests
have a 60-second local resend cooldown. Errors are safe and enumeration-resistant;
Cognito still enforces its own attempt and delivery limits.

Project names follow server limits. Create/detail/rename use exposed ETags;
PATCH/DELETE carry the exact quoted If-Match. A 409/412 disables further writes
until explicit detail refresh; the user reviews and submits again. Lists use
opaque nextCursor, including empty pages, deduplicate appended IDs and refresh
from the first page. Recently created details remain immediate even with GSI lag.

A lost/5xx/incomplete creation acknowledgment disables Create until the list is
refreshed and checked. There is no automatic creation retry or idempotency claim.
Lists are eventual: absence is not proof of noncommit. Uncertain deletion offers
an explicit initiating-version check; the accepted repeat DELETE returns the
same operation. Operation polling starts at two seconds, backs off to ten, and
stops at completion, authentication failure or 404. Transient failures remain
visible and back off. Sign-out/unmount aborts reads/polling and clears UI state.

Physical cleanup remains the existing **operator-only empty-project command**.
The browser can accept deletion and poll, but never silently report completion:

```sh
npm run dev:api -- -cleanup-project PROJECT_ID
```

Get the opaque ID from the project details before deleting or the API response.
Unexpected children keep cleanup pending. Full cross-store deletion remains
outside this slice. DynamoDB Local is in memory; stopping its container loses data.

## Verification

```sh
npm run verify
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
npx playwright install chromium
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration
docker compose -f compose.dynamodb.yml config --quiet
git diff --check
```

The browser integration harness generates signed JWTs only in test memory,
launches the actual React app, intercepts Cognito alone, and sends all product
requests through actual Go JWT verification/CORS and DynamoDB Local. It checks
pagination, create/detail/rename, concurrent stale writes/manual refresh,
delete/operator cleanup/polling, committed-but-lost POST without replay,
light/dark switching and empty persistent browser storage after reload.
Its cleanup HTTP helper and fixture trust exist only in `_test.go`, never runtime.
Browser traces, screenshots and video are disabled; no authentication payloads
are persisted. CI runs this harness as well as the existing race-enabled suite.

Unit tests check auth form loading/code/reset/error behavior, memory storage,
refresh coalescing/sign-out fencing, access-token-only API calls, no POST retry,
ETag/If-Match, pagination, strict CORS, account/root guards and the accepted
Essentials session configuration. These tests do not prove AWS/Cognito/email delivery.

## Current validation and gaps

Frontend unit tests (16), infrastructure fixture tests (6), frontend build/lint,
Go API tests/vet/build, actual race-enabled DynamoDB Local tests, and the Chromium
fixture browser-to-backend flow passed during development. `npm run verify` passed all formatting, lint, unit tests, frontend build and Go
tests/vet in all five modules. Both final race/browser integration commands,
Compose validation, JS syntax and `git diff --check` also passed.

SST preview/deployment and AWS readback passed for the Essentials pool/public
client. Davian confirmed genuine sign-in in Vivaldi; AWS readback confirmed the test
account is enabled/CONFIRMED with verified email. Davian also confirmed project listing, creation, detail and rename in Vivaldi;
local user provisioning and saved project revisions are independently inspected.
The Vivaldi test project was renamed at version 1, deletion accepted at version 2,
and operator cleanup completed. Independent inspection confirmed a deleted
minimal tombstone at version 4 with the name removed. Davian confirmed deletion in Vivaldi and password-reset email/code confirmation
followed by successful new-password sign-in. Davian then forced an SDK refresh inside Vivaldi and confirmed success without
printing tokens. Live resend/invalid-or-expired-code tests and natural
15-minute/one-day expiry boundaries remain unverified. The Essentials decision is accepted. Delivery/code entry requires
Davian's inbox participation. Custom SES/domain verification and public email
production readiness are explicitly deferred by the development sender choice.

A standalone `tsc` over SST imports fails inside generated SST platform types
and the historical spike's root Pulumi module resolution; it is not a successful
SST resource preview. The auth configuration fixture tests pass; real `sst diff`, deployment and AWS
resource readback also passed. Runtime Cognito/email checks are tracked separately.

Incremental model before credits/free tier: Lite $0.0055/MAU, Essentials
$0.015/MAU at the first pricing band (see [AWS pricing](https://aws.amazon.com/cognito/pricing/)).
At 2–5 users this is roughly $0.01–$0.03 Lite or $0.03–$0.08 Essentials/month.
The default development sender adds no separately configured SES sender resource.
Existing SST bootstrap storage/request costs are shared and usage dependent;
there is no new always-on API/database/frontend infrastructure in this stage.
This is a forecast, not measured spend or a claim of free-tier eligibility.

Real-browser validation uses the developer's Vivaldi session. Passwords and codes
are entered directly into the app; there is no agent-accessible Vivaldi debugging
connection. Confirm signup/verification, project operations, sign-out and reset
manually; operator cleanup remains the documented local command. Headless
Chromium is used only for the automated fixture browser integration tests.

The Google Docs proposal and API contracts were edited in place and read back:
Essentials replaces Lite, default development email is distinguished from future
custom SES readiness, and forecasts are $3.19/$14.19. Trello authentication and
project cards/checklists record verified progress and retain incomplete live
resend/code checks. The overall contract-review card remains Doing.
