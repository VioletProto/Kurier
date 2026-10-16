# Frontend hosting: proposed review packet

Status: **Proposed — awaiting Davian's acceptance.** Prepared 2026-10-16.
This packet authorizes nothing beyond its own preparation. Implementation,
external proposal/API-contract synchronization, SST preview/bootstrap and AWS
deployment require subsequent authorization. No hosted browser, Cognito email,
cache, IAM or deployment result is claimed here.

Dependency: [MCP PR #11](https://github.com/VioletProto/Kurier/pull/11) is merged;
GitHub returned `merged=true`, merge commit
`9f42fc476f4bc9ad31aba5cb304825a54f39740a`. After fetching, `origin/main`
matched that commit. `feat/frontend-hosting` was created from it with a clean
working tree. [Preparation card](https://trello.com/c/piZAWsGf) belongs to the
[Kurier Capstone board](https://trello.com/b/dRYhnjS6/kurier-capstone).
The configured Doing WIP limit is three; Doing was empty before preparation.

## Scope and evidence reviewed

Deliver the existing React product through HTTPS, retaining the current API,
identity, execution/evidence, local-agent and read-only MCP boundaries. This is
a development/demo deployment, not a general-public signup launch. No new
features, custom domain, hosted MCP transport, browser-to-localhost control,
SSR, standing compute or paid networking are proposed.

Reviewed [accepted architecture](README.md), [ADR 0001](../decisions/0001-sst-infrastructure-candidate.md),
[ADR 0002](../decisions/0002-serverless-persistence-topology.md), W1
[system design](w1-system-design.md), [data model](w1-data-model.md),
[decisions/costs](w1-review-decisions.md), and the users/projects, saved/protected
requests, cloud evidence, pinning, local-agent, sequential-workflow and
read-only MCP contracts and validation records. Later slice records supersede
historical descriptions of unimplemented features; hosting does not reset them.

Read the current native [proposal](https://docs.google.com/document/d/1tatrrhqytTxAZxlrOnbSQRymmqjL2MmGq7Tdj0-1T60/edit)
and [API contracts](https://docs.google.com/document/d/1KSfRYOb2UxmrIl8VoFjc3YP38-PMu7k_fM9wrVkEenM/edit)
without editing either. They already select SST, static CloudFront/S3,
memory-only Cognito sessions and separate local execution/stdio MCP. Some
implementation-status paragraphs are historical. In particular the auth
contract says to verify SES sender/domain and regional readiness before public
hosting. Keeping the default sender for this publicly reachable development
demo is an explicit proposed exception below, not an implicit contract change.

Repository inspection establishes:

- SST **4.17.1**, AWS provider **7.40.0**, region `us-east-2`. `sst.config.ts`
  permits only historical `viability`/`db-viability`, plus protected/retained
  `dev-auth` and `dev-api`. The spike's StaticSite proves historical viability;
  it is not a current product host and must not be redeployed.
- `infra/development-api.ts` owns the current API and reads the separate
  `dev-auth` Cognito pool/client. Its origin guard and Go `LocalCORS` both
  accept a single explicit HTTP loopback origin only. Gateway CORS is omitted
  through a transform, because an empty configuration previously stripped
  Lambda CORS headers. Preserve that fix.
- `apps/web` uses React 19, TypeScript, Vite 7 and Amplify 6.22.1-compatible
  dependencies. Build is `tsc -b && vite build`, output `dist`, default base
  `/`. The UI uses React state rather than a pathname router; hosting fallback
  cannot invent project/detail deep links.
- `auth.ts` sets Amplify token storage to shared memory before auth calls;
  `api.ts` uses `fetch` with `cache: "no-store"`. Sign-out disposes the workspace
  and clears tokens; protected inputs and one-time credentials have existing
  containment rules. There is no product service worker/offline store.
- The last verified deployment record reports Free/ACTIVE, account/unreserved
  Lambda concurrency **10**, no function reserved/provisioned concurrency,
  SQS standard pollers, worker batch **1**/maximum concurrency **2**, minute
  cleanup, active stage version **11**, savedRequests **7**/MCP **1** and
  recovery generation `dev-api-initial-v1`. These are prior recorded facts,
  not fresh AWS inventory performed during this preparation.

## Proposed topology and hostname

Add one `sst.aws.StaticSite` for `apps/web` to existing app `kurier`, stage
`dev-api`, rather than adding another stage, API, auth pool or bootstrap stack.
Use the existing HTTP API HTTPS URL and existing `dev-auth` public client.
Reuse shared SST state/assets infrastructure where present. Create a separate
web-assets S3 bucket: never reuse the evidence bucket, Protected storage or a
bootstrap bucket as the public site's origin.

Initial site URL is the distribution's generated
`https://<distribution>.cloudfront.net`, exported as `siteUrl`/`hostedFrontendOrigin`.
There is no allocated hostname yet. No domain purchase, DNS hosted zone, ACM
custom certificate or redirect domain is necessary. CloudFront is global;
the web bucket, API, Cognito and existing application resources stay in Ohio.
CloudFront cache copies of public assets are global, not region-confined.

[SST StaticSite](https://sst.dev/docs/component/aws/static-site/) supports Vite
build output, asset metadata, invalidations and CDN transforms. Verify its
generated resources against the installed version, not just examples from the
latest docs. Installed 4.17.1 uses a viewer-request CloudFront Function and
KeyValueStore for routing, including dynamic signed S3-origin selection. Those
are metered edge operations, not Lambda or standing servers. Its default
bucket policy grants the CloudFront service access without a distribution
condition: tighten it for this bucket.

Require private S3 REST origin, all public-access blocks, bucket-owner-enforced
ownership, SSE-S3, HTTPS-only access and signed origin requests. Bucket policy
permits CloudFront `s3:GetObject` only on web assets with this distribution's
`AWS:SourceArn`; deny insecure transport. No public ACL, S3 website endpoint,
browser S3 credentials or evidence-origin route. Verify dynamic OAC signing
and the policy condition together in the generated plan and real requests;
[AWS recommends distribution-scoped OAC policies](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-restricting-access-to-s3.html).

CloudFront serves static GET/HEAD only; reject other viewer methods. Redirect
HTTP to HTTPS. With the default `*.cloudfront.net` certificate, AWS fixes the
viewer security policy to `TLSv1`: do not claim a configurable TLS-1.2-only
minimum. Modern browsers still negotiate TLS 1.2/1.3. Acceptance includes this
initial hostname tradeoff. A stricter minimum requires a custom-domain SNI
certificate and separate review. See [AWS distribution settings](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/DownloadDistValuesGeneral.html).
Any later CloudFront ACM viewer certificate must be in **us-east-1**; that does
not move the application. [Certificate regional requirements](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/cnames-and-https-requirements.html).

## Public build configuration and trust

The complete application-specific allowlist for the hosted build is:

| Value                       | Intended public meaning                                                                              |
| --------------------------- | ---------------------------------------------------------------------------------------------------- |
| `VITE_API_URL`              | Exact existing `https://<api-id>.execute-api.us-east-2.amazonaws.com`, without a path/query/fragment |
| `VITE_COGNITO_USER_POOL_ID` | Existing Ohio pool identifier                                                                        |
| `VITE_COGNITO_CLIENT_ID`    | Existing public SRP app-client identifier, never a client secret                                     |
| `VITE_KURIER_STAGE`         | `dev-api` display label                                                                              |

Issuer/region can be derived from the pool; they are public if needed. A public
release identifier may be displayed for cache diagnosis. Treat every byte in
HTML/JS/CSS/maps, `public/`, build metadata and downloadable archives as public.
No AWS credentials, cursor/service signing keys, KMS material, SST secrets,
passwords, JWT/refresh tokens, local-agent/MCP capabilities, user data or
protected request values may enter the build, resource links or output files.
Do not link protected resources to the StaticSite or spread `.sst/outputs.json`
or the deployment environment into browser configuration. Disable public
source maps and omit fixture/browser-auth bypasses from production entrypoints.

Hosted builds fail closed on missing or malformed configuration, unexpected
stage/region or a non-HTTPS API endpoint; never silently use `api.ts`'s local
fallback in a hosted build. Explicit local helpers retain their development
defaults. Validate pool/client correspondence against read-only AWS metadata
before a future deployment. Scan output using disposable canary values without
printing secrets; a string search alone is not proof of complete containment.
Public IDs authorize nothing: API JWT, ownership, stage and scoped-capability
checks remain authoritative.

## Exact API CORS and localhost development

Propose an explicit origin allowlist with **two entries** initially: the actual
generated hosted HTTPS origin and `http://localhost:5173`. A deliberately
configured replacement loopback origin with an explicit port remains possible
for local development/tests; `127.0.0.1`, IPv6 and other ports are not implicitly
equivalent or wildcarded. Keep the legacy `frontendOrigin` local output/helper
meaning stable and export the hosted origin separately. Do not remove
`npm run dev:cloud` or redirect that helper to a remote host.

Update infrastructure and runtime origin validation together. Accept normalized
exact origins only: no userinfo, path, trailing slash, query, fragment, `null`,
wildcard, suffix match or arbitrary HTTPS site. An Origin header must contain
one exact allowlisted value; reflect only that value. Reject duplicate or
unapproved origins. Origin-less CLI/agent/MCP traffic continues through normal
authentication and ownership checks. CORS itself never grants authority.

| Behavior                    | Required contract                                                                                                     |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Preflight                   | Unauthenticated `OPTIONS /api/v1/...`; validate requested method/header names; success `204`                          |
| Allowed application methods | `GET, POST, PUT, PATCH, DELETE` (PUT remains necessary for pinning)                                                   |
| Allowed request headers     | `Authorization, Content-Type, If-Match, Idempotency-Key`; case-insensitive validation, reject extra requested headers |
| Allowed origin              | One exact matched origin per response; no `*`                                                                         |
| Exposed response headers    | `ETag, Location, Retry-After` on successful and application error responses                                           |
| Credentials                 | No `Access-Control-Allow-Credentials`; bearer tokens, no API auth cookies; browser credentials omitted                |
| Variation                   | `Vary: Origin`; preflight additionally varies on `Access-Control-Request-Method` and `Access-Control-Request-Headers` |
| Cache                       | `Cache-Control: no-store`; initially omit `Access-Control-Max-Age` to avoid durable stale preflight authorization     |

Preserve handler-owned CORS with Gateway `corsConfiguration` absent.
[Gateway-managed CORS overrides backend headers](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-cors.html).
Keep OPTIONS routing and genuine denied preflight tests. Gateway-native unknown
route/throttle errors may lack application CORS; show a safe network/service
message without assuming noncommit or retrying writes. Application 401/403/404,
412/428, 429 and 5xx must expose the expected headers for approved origins.

Avoid an infrastructure dependency cycle: construct the HTTP API independently
before site build configuration, inject only `api.url` into the site, then use
`site.url` in the API Lambda allowlist and attach routes afterward. API resource
identity need not depend on its integrations. Review the actual graph; do not
hardcode a guessed CloudFront hostname or permit `https://*.cloudfront.net` to
get a first deployment working. No evidence/API proxy behavior on this CDN.

## Genuine Cognito authentication from HTTPS

Reuse the current pool/client and custom React auth screens. Preserve Essentials,
email verification/recovery, public client without secret, SRP-only initial
auth, 15-minute access/ID tokens, one-day refresh lifetime, ten-second rotation
grace, token revocation, enumeration protection, existing password policy,
MFA/device settings and deletion protection.

No Cognito configuration change is required solely because the frontend origin
changes: this app uses direct user-pool API calls, not managed-login redirects.
Keep OAuth flows, callbacks, logout URLs and user-pool domain unset. Do not add
hosted login, identity pools, browser AWS IAM or secret-bearing clients. If the
real deployed configuration differs from these assumptions, stop and review
the required change in `dev-auth`; `dev-api` must not take over pool ownership.

From the real hosted browser, prove signup, delivered verification/resend,
confirmation, SRP sign-in, reset request/delivered code/new-password sign-in,
automatic access-token refresh and sign-out. The SDK must rotate via
`GetTokensFromRefreshToken`; `ALLOW_REFRESH_TOKEN_AUTH` stays disabled.
Rotation retains the original session deadline. Preserve memory-only storage,
single-flight refresh, generation guards and clearing after in-flight refresh
settles. Reload/new tab requires sign-in; reads may refresh once, writes never
automatically replay. [AWS refresh-token behavior](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-the-refresh-token.html).

Default `COGNITO_DEFAULT` email remains proposed **for this development demo
only**, with explicit acceptance of the API-contract public-hosting exception.
The default sender's documented cap is 50 emails/day/account; actual remaining
quota/delivery must be checked before the small test cohort. Keep the existing
60-second resend UI cooldown and safe throttle/errors; no quota increase,
SES setup, paid sender or general-public launch is implied.
[Cognito quotas](https://docs.aws.amazon.com/cognito/latest/developerguide/quotas.html).
If Davian requires general-public signup readiness instead, custom SES identity,
regional sandbox/production access and costs need a separate accepted scope.

Sign-out must clear workspace, editor values, one-time credential display and
pending callbacks/polls even if Cognito revocation fails. Report remote failure
safely. Offline JWT verification retains the accepted access-token expiry and
clock-tolerance revocation lag; hosting does not provide immediate universal
JWT revocation. Scoped MCP/local-agent revocation remains server checked.

## SPA paths, caching and rollback

Serve at `/` with root-relative hashed Vite asset paths. Navigation to `/` or a
non-file application path must return the shell with status 200, including
refresh/direct navigation. Current state-only UI shows its normal signed-out
screen after reload; it does not promise restoration of the selected project
or a new `/projects/:id` route. Unknown paths may load that same shell initially.
New semantic route selection is outside this slice.

Keep missing `/assets/*`, explicit file paths and accidental `/api/*` requests
from becoming successful HTML module/API responses. Return real 404s and correct
content types. Use the installed StaticSite routing defaults only where they
meet this contract; use a bounded viewer-request adjustment for exclusions.
Do not set `errorPage: "index.html"` blindly: installed code treats an explicit
error page as a 404 response, unlike its default SPA fallback. Test trailing
slashes, queries, encoded paths and missing chunks.

| Object                                 | Deployment/browser policy                                                                                                     |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| HTML, nonhashed public metadata/config | `Cache-Control: no-store, max-age=0, must-revalidate, stale-if-error=0`; CloudFront min/default TTL zero                      |
| Content-hashed `/assets/*`             | `Cache-Control: public, max-age=31536000, immutable`; never overwrite a hash with different bytes                             |
| Missing assets/errors                  | No long-lived negative caching; error TTL zero where supported, accounting for S3's documented minimum error caching behavior |
| Authenticated API/evidence             | Direct existing API HTTPS only, response `no-store`, fetch `no-store`; absent from CDN origins/behaviors and static bucket    |

Replace StaticSite's managed CachingOptimized policy (positive minimum TTL)
with a reviewed policy whose minimum/default TTL are zero, maximum one year,
and compression variants are correct. Honor origin asset metadata, exclude
cookies/auth headers/query strings from static cache keys and do not forward
browser credentials to S3. A positive minimum TTL can override `no-store`:
[AWS expiration rules](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/Expiration.html).
No service worker, Cache Storage, IndexedDB, Web Storage or persisted query cache
for authenticated responses, evidence, editor secrets or credentials. Keep
evidence as inert text; upstream HTML/scripts never become active DOM. Browser
Back/Forward restoration must not reveal the previous signed-out workspace;
test lifecycle restoration and revalidate/clear state when necessary.

Set `assets.purge: false` and explicitly override SST's delete behavior for the
retained static bucket. Retain old hashed assets for a proposed **30-day**
compatibility window, plus the current and previous known-good release even
when older. Keep a bounded public-assets-only release archive/manifest (commit,
public config, file hashes), and garbage-collect obsolete releases deliberately;
do not use a blanket object-age expiration rule that deletes the current build.
This prevents tabs open during deployment from losing their referenced chunks.

Deploy assets before publishing the shell/routing metadata, invalidate `/*`
(one wildcard path) and wait for completion before declaring release ready.
Uploads/KV propagation/invalidation are not globally atomic. Verify SST's
ordering and use a reviewed staged upload/shell publication if its provider
does not establish the necessary ordering. Preserve the prior shell/manifest
until postdeploy checks pass. Long-open tabs may request a deliberate reload;
never auto-reload a form containing unsaved secret input or replay a write.

Rollback republishes the previous verified public artifact/config and matching
route metadata to the same distribution/bucket, then invalidates and waits.
Retain origin identity and API compatibility. It does not remove resources,
revert DynamoDB data/capabilities, reset generation/accounting, undo executions,
or downgrade the API to a pre-MCP binary. Keep backend CORS changes narrowly
separable so a UI rollback does not silently remove an origin still in use.

## Security headers, compatibility and user experience

Attach a response-headers policy through reviewed SST/CDN transforms. Initial
production CSP proposal, replacing the placeholder with the exact API origin:

```text
default-src 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none';
script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self';
connect-src https://<api-id>.execute-api.us-east-2.amazonaws.com https://cognito-idp.us-east-2.amazonaws.com;
form-action 'self'; manifest-src 'self'; worker-src 'none'; upgrade-insecure-requests
```

No `unsafe-inline`, `unsafe-eval`, remote script CDN or wildcard connection
host. Current production CSS/JS are external; verify Amplify SRP/refresh and
React under enforced CSP. Add only an evidenced narrow exception through
review, never weaken CSP just to hide a violation. Local Vite/HMR policy is
separate and does not add localhost to hosted `connect-src`. URLs entered as
execution destinations are data sent to the API/agent, not browser fetch hosts.
Do not introduce a CSP reporting endpoint that receives secret-bearing URLs.

Also require `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy: no-referrer`, `Permissions-Policy: camera=(), microphone=(), geolocation=()`,
and HSTS `max-age=31536000` for this HTTPS host, without preload or a broad
subdomain commitment. Do not assume the AWS managed SecurityHeadersPolicy
contains this CSP or all desired values; use a custom policy and inspect headers
on shell/assets/error responses. [AWS response-header policies](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-response-headers-policies.html).
Keep CDN access/real-time logs and edge console logging disabled initially:
raw paths, query strings and user-supplied identifiers must not become logs.

Preserve labelled inputs, skip navigation, focus visibility, keyboard operation,
status/alert regions, light/dark contrast, responsive wrapping and 200% zoom.
Provide bounded loading/disabled submit states and safe actionable errors for
bad public config, offline/API/Cognito outage, expired session/code, throttling
and failed chunk loading. Preserve user edits where safe; uncertain writes
require explicit reconciliation, not a retry button that silently resubmits.
No raw AWS errors, tokens, protected input or evidence in telemetry/errors.

## Hosted UI, local agent and stdio MCP

The hosted UI continues browser-authenticated scope/consent, one-time issuance,
status and revocation through the existing API. It never contacts loopback,
private LAN addresses or a local control port. No browser extension, CORS bypass,
mixed-content permission or hosted MCP server is introduced.

Local execution setup remains a separate manual foreground-terminal flow:
build/run the existing Go agent, issue the scoped local-agent credential from
the hosted UI, enter it at the hidden terminal prompt and let the agent poll
the trusted API over outbound HTTPS. Destination consent applies only to the
agent. Preserve one active agent, expiry/revocation and uncertain-result/no
automatic target replay. See [local-agent setup](../development/local-agent-execution.md).

MCP setup follows [the existing numbered guide](../development/read-only-mcp.md):
build `kurier-mcp`; use the hosted UI instead of starting local React to issue
a distinct read-only MCP credential; run `kurier-mcp pair --api-origin <trusted-api-origin> --persist`
and enter it only at the hidden prompt; register `serve --credential-file ...`
with Codex. **Codex launches the stdio server.** Pairing, registration and serving
are different steps. No token in URLs, arguments, environment, downloadable
config, shell history or Codex config. Preserve private 0600/0700 file storage
and its accepted same-user limitation, seven tools, scopes/limits, audit and
immediate subsequent-read denial after revocation. Issued local-agent and MCP
credentials remain distinct from each other and browser Cognito tokens.

## Resources, IAM, bootstrap and account constraints

Expected additions, subject to a later reviewed plan: one private static bucket
with policy/public-access/ownership/encryption controls; one distribution;
the installed StaticSite routing Function/KeyValueStore and route entries;
custom cache/response-header policies; asset uploads, release artifacts and
invalidations. Dynamic OAC may not create a standalone OAC resource in this
SST version. Do not promise a resource count based on older SST examples.
Expected changes: existing API Lambda origin environment/Go middleware,
public outputs/local helper compatibility, hosting build validation, and
relevant tests. No API route/DTO/schema/capability activation is needed.

Deployment principal needs reviewed resource-scoped S3 configuration/upload/read
and release-cleanup permissions, CloudFront distribution/functions/KV/policy
management and invalidation, plus existing SST state/bootstrap and scoped API
update permissions. List/create actions that cannot be resource-scoped must be
identified explicitly in the later IAM review; no AdministratorAccess or broad
IAM administration. Browser has no AWS role. CloudFront has only signed public
asset reads. API/worker/maintenance runtime IAM stays unchanged, including no
web-bucket read/write, new KMS decrypt or Protected permissions.

Before any later AWS mutation, verify intended account **747336059622** and
non-root identity, active Free plan/credit expiry and service availability,
quota/concurrency, current resource inventory and `/sst/bootstrap`. Previous
records show version-5 Ohio shared bootstrap/state/asset buckets/ECR; re-read,
reuse and do not recreate or remove it. SST diff can bootstrap and is itself
held until authorized. Inspect passphrase/state/link writes, provider upgrades,
global CloudFront changes and any new regional provider/bootstrap effects.
No new us-east-1 bootstrap should be required for the default certificate.

Preserve account Free plan; no paid-plan conversion, quota increase, concurrency
reservation, provisioned concurrency/pollers or new worker trigger. Preserve
SQS max2/batch1/standard, API throttles 10 RPS/burst20, minute schedule, all
resource names/retention/protection, current capabilities/generation and existing
project/quota accounting. No VPC, NAT, ALB, public IPv4, RDS, ECS, Lambda@Edge,
Origin Shield, paid WAF/Shield plan or standing build server. If a needed service
is unavailable under the account's current Free plan, stop for review rather
than upgrade. [AWS account plan constraints](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/free-tier-plans.html).

## Incremental cost model

USD planning estimates checked against official prices 2026-10-16; Ohio S3,
US/Canada/Europe delivery assumptions. These are additional hosting costs,
not a re-estimate of the entire deployed system or measured bills. Existing
W1 forecasts already contain some static/bootstrap allowances; do not add
this table to those totals without removing overlap.

Propose ordinary CloudFront **pay-as-you-go**, no subscription/paid networking.
That distribution billing option is different from the account's **AWS Free
plan**, which must remain unchanged. Current AWS pricing also advertises a
separate CloudFront flat-rate Free offering ($0, 1M requests/100 GB with bundled
features); do not silently enroll, switch plans, add bundled resources or count
both offers. Its feature/plan eligibility and SST support require separate
review if selected. [CloudFront pricing choices](https://aws.amazon.com/cloudfront/pricing/).

Light month: 1,000 visits, 20,000 HTTPS object requests, 1 GB transfer, 10 releases,
0.1 GB retained web artifacts plus 0.1 GB incremental shared SST state/assets,
2,000 PUT/LIST, 5,000 S3 GET, 20,000 viewer-function invocations, up to 100,000
KV reads and 100 non-read KV API calls. Larger demo: 10,000 visits, 200,000 HTTPS
requests, 10 GB transfer, 30 releases, 1 GB total incremental storage,
6,000 PUT/LIST, 50,000 GET, 200,000 function invocations, 1M KV reads and 300 KV
API calls. Actual KV reads include routing probes/misses, not one per request.

| Incremental item and unit rate                                                         |       Light | Larger demo |
| -------------------------------------------------------------------------------------- | ----------: | ----------: |
| CloudFront transfer, $0.085/GB screening rate                                          |      $0.085 |      $0.850 |
| HTTPS requests, $0.01/10,000                                                           |      $0.020 |      $0.200 |
| CloudFront Function, $0.10/million                                                     |      $0.002 |      $0.020 |
| KV reads in functions, $0.03/million                                                   |      $0.003 |      $0.030 |
| Other KV API requests, $1/1,000                                                        |      $0.100 |      $0.300 |
| S3 Standard storage, $0.023/GB-month                                                   |     $0.0046 |     $0.0230 |
| S3 PUT/LIST, $0.005/1,000                                                              |      $0.010 |      $0.030 |
| S3 GET, $0.0004/1,000                                                                  |      $0.002 |      $0.020 |
| Invalidation, 10/30 wildcard paths within first 1,000/month shared allowance           |          $0 |          $0 |
| **Gross subtotal before transfer/request/function/KV-read free allowances or credits** | **$0.2266** | **$1.4730** |

Within otherwise unused account-wide pay-as-you-go allowances (1 TB transfer,
10M HTTP(S) requests, 2M function invocations, 2M KV reads/month), modeled
subtotal is about **$0.12/$0.37** before S3 allowances or promotional credits.
Invalidations beyond the shared 1,000-path allowance cost $0.005/path. No charge
for S3-to-CloudFront transfer, default hostname/certificate, response/cache
policies or standard S3 DELETE. Rates/allowances: [CloudFront pay-as-you-go](https://aws.amazon.com/cloudfront/pricing/pay-as-you-go/)
and [S3 pricing](https://aws.amazon.com/s3/pricing/).

No incremental MAUs if the same users already use this pool; an additional
Cognito Essentials MAU screens at $0.015 before allowances, so 2–5 new demo users
add $0.03–$0.075. [Cognito pricing](https://aws.amazon.com/cognito/pricing/).
Existing KMS, secret, API, worker, DynamoDB, evidence S3, logs/schedule and ECR
costs continue; no new key/secret/compute/ECR image is needed for static hosting.
Optional DNS registration/zone/queries and SES are excluded and separately
scoped. Browser sessions can increase existing API/poll/execution traffic;
static hosting alone does not model or cap those costs.

Use **$0.25–$2/month** as a hosting planning allowance and **$1–$3 one-time**
validation allowance for asset uploads, invalidations/KV changes, small auth
cohort and existing API checks. These are not spend caps. Free allowances are
shared, credits expire, retained releases grow and public traffic/abuse can
exceed assumptions. Fresh plan/usage reads and the actual generated KV update
count are deployment-review requirements, not reasons to change the plan.

## Planned verification and release gates

All hosted checks below are **planned, not performed**. Existing localhost,
fixture and historical AWS proofs do not establish hosted-origin success.
Use disposable accounts/projects/canaries, memory-only credential handling and
sanitized pass/fail/count evidence. Do not record HARs, traces, screenshots of
one-time credentials, console secrets or unredacted auth/evidence payloads.

| Layer                 | Required checks and evidence                                                                                                                                                                                                                                                                                               |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Repository/local      | `npm run verify` (format/gofmt, lint, frontend/infra tests, production build, seven Go modules test/vet); targeted strict allowlist/CORS tests; real DynamoDB Local race and Chromium harness where relevant; distinguish fixtures from AWS                                                                                |
| Artifact/security     | Correct four public config values and hashed paths; reject missing/unsafe build config; no secret canaries, fixture login entrypoints or source maps; enforced CSP/headers and safe body rendering                                                                                                                         |
| Browser/HTTPS         | Genuine hosted Chromium plus operator Vivaldi; Firefox/WebKit when available or explicitly pending; HTTP redirect, certificate/modern TLS negotiation, no mixed content, console CSP errors or localhost requests; keyboard, focus, announcements, contrast, narrow viewport and zoom                                      |
| Cross-origin          | Exact hosted and deliberate localhost pass; hostile/sibling/null/malformed/duplicate origins fail; all five methods/header permutations, denied extras, unauthenticated OPTIONS, origin-less CLI; readable ETag/Location/Retry-After, 401/404/412/428/429/error behavior; Gateway-native network error fallback            |
| Genuine auth/email    | New signup, real delivered verify/resend and reset codes, wrong/expired-code UX, sign-in/new password, automatic refresh after real 15-minute expiry without shortened settings, rotation/revocation metadata, sign-out during refresh/offline, reload/new-tab sign-in; one-day elapsed expiry only when actually observed |
| Deep links/assets     | Direct nested navigation/reload/trailing slash/query, normal signed-out shell; no implied route restoration; root asset paths, correct MIME, real missing asset/API 404 and no HTML-as-JS response                                                                                                                         |
| Stale cache/rollback  | Build A open during B; old chunk requests remain valid, new navigation gets B, CSP/header changes propagate, invalidation waited; missing chunk safe recovery; republish A with matching metadata/config and confirm current API compatibility; retained archive/cleanup proof                                             |
| Persistence/lifecycle | No API/evidence in CloudFront/S3 web bucket or browser stores/cache/service worker; no prior-account workspace after logout/reload/history restoration; late responses cannot restore tokens/data; disconnected loading/error states are safe                                                                              |
| Existing features     | Owned/foreign projects and versioned edits/deletion; public/protected request create/preserve/replace/revoke and stale edits; cloud execution/rerun/history/immutable sanitized evidence; pin/unpin/expiry; sequential runs and no replay; scheduled deletion and accounting preservation                                  |
| Local agent/MCP       | Hosted consent/status/one-time display; real manual hidden-prompt agent pairing and controlled localhost/private target through agent only; genuine Codex-launched stdio MCP, seven scoped reads/redaction/limits and revoke denial; no browser-localhost or remote MCP transport                                          |
| AWS/IAM               | Intended non-root account, private-bucket anonymous denial and signed CDN success, scoped policy/IAM review and negative checks, distribution/config/header readback, asset hashes, stage/counters/Free plan/quota/concurrency unchanged; cleanup disposable data through actual existing schedule                         |

Implementation acceptance and deployment approval are separate. Before asking
for deployment, finish reviewable code/tests, refresh read-only AWS inventory,
and obtain authorization for any bootstrap-capable preview. Present the scoped
resource/IAM diff, public artifact manifest, cost update and rollback steps.
Then wait for explicit deployment permission. No schema activation or stage
closure is expected for this compatible hosting/CORS change; unexpected need
for either requires a new review. After authorized deployment, report local,
fixture, AWS and genuine browser outcomes separately, leaving unperformed
checks open. Do not launch general-public signup on development email claims.

## Preparation validation

Documentation-only change. `npm run verify` passed formatting/gofmt, lint,
all **56 frontend** and **10 infrastructure** tests, and the TypeScript/Vite
production build. Its Go step initially failed because the sandbox could not
write the normal Go cache. The approved outside-sandbox `npm run verify:go`
rerun passed tests and vet for all seven modules. No implementation tests were
added for this proposed document. These checks do not validate the proposed
hosted configuration; no AWS inventory/mutation, SST diff, hosted browser test
or external-document synchronization was performed during preparation.

## Decisions requiring acceptance

1. **Topology and scope:** one SST StaticSite in existing `dev-api`, a dedicated
   private web bucket, current API/Cognito reuse, AWS-generated HTTPS hostname;
   custom domain/DNS/ACM and general-public launch deferred. Accept the default
   certificate's fixed TLS-policy limitation for this initial demo.
2. **Origin/config/security:** only the four intended public build values;
   exact hosted plus explicit localhost CORS with the methods/headers above;
   static-only CDN, memory-only auth/data and the proposed strict CSP/headers.
3. **Authentication/email exception:** preserve current Essentials/SRP/rotation
   settings with no origin-driven Cognito change; explicitly extend the default
   Cognito sender permission to this public-URL development/demo host. Custom
   SES readiness remains required for a separately scoped public launch.
4. **Release behavior:** root SPA shell without new semantic routes, zero-TTL
   HTML policy, immutable hashed assets, 30-day/current/previous retention,
   waited invalidation and artifact rollback without backend/data reversal.
5. **Operations/costs:** pay-as-you-go CloudFront while preserving the AWS Free
   account plan and existing quotas/concurrency; metered routing Function/KV,
   scoped IAM/bootstrap review, $0.25–$2 monthly hosting planning range and
   $1–$3 validation allowance, with no hard-cap claim.
6. **Next authorization:** decide separately whether to authorize implementation
   and local verification, external proposal/API synchronization, and a scoped
   bootstrap-capable AWS preview. Deployment remains held for its concrete
   resource/IAM/cost review even if the design is accepted.
