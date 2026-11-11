# Secure browser session persistence

Research date: **2026-11-11**. [Spike](https://trello.com/c/JR5tQXNA),
[delivery](https://trello.com/c/p4yuaybE).

Davian selected **backend sessions** and **survival across browser reopening until
sign-out or the existing one-day expiry** during this spike. The recommended
implementation is a same-origin refresh-session broker. The detailed interface
below is a **proposed contract amendment for review**, not a claim of runtime
implementation, deployment or acceptance of every wire detail. The memory-only
implementation remains active. No AWS calls or mutations were made for research.

## Result and decisions

Keep Cognito SRP sign-in in the browser, transfer its refresh credential once
over TLS to a same-origin session broker, and clear the browser's transient
Cognito store. Persist only an opaque HttpOnly cookie in the browser. The broker
encrypts the rotating refresh credential server-side and supplies short-lived
access tokens to browser memory. Existing product requests continue directly to
the bearer-authenticated Go API. This reduces persistent credential theft through
XSS without moving every application response through a new proxy.

| Decision      | Recommendation and effect on Kurier                                                                                                                                                                          |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Persistence   | Backend session selected by Davian; no refresh tokens in Web Storage, IndexedDB, Cache Storage or readable cookies. Login still transiently handles Cognito tokens in memory.                                |
| Reopening     | Selected by Davian: persistent cookie until sign-out or fixed one-day expiry; rotation/activity never extends that deadline.                                                                                 |
| Backend scope | Proposed narrow refresh broker, rather than full application proxy; access tokens remain accessible to active same-origin JavaScript.                                                                        |
| Revocation    | Preserve current offline JWT validation and its residual access window, up to 15 minutes plus 60-second verifier tolerance. Immediate denial of issued access tokens would need a separate API session gate. |
| Hosting       | Add only the auth route family to the existing CloudFront distribution with caching disabled. Static S3 and direct API/evidence delivery remain.                                                             |
| Delivery      | Contract review, local implementation/tests and a separately reviewed deployment preview are follow-on work. Existing Free plan, quotas, recovery and concurrency settings stay in scope as constraints.     |

## Evidence from the current implementation

- [auth.ts](../../apps/web/src/auth.ts) configures Amplify's shared in-memory
  storage, requires an in-process `active` flag and serializes refresh only
  within that module instance. It has no startup session restoration.
- [App.tsx](../../apps/web/src/App.tsx) starts signed out and calls logout on
  `pagehide` and restored history. Persisting credentials alone would still
  revoke a session on reload/navigation. Change that lifecycle deliberately.
- [api.ts](../../apps/web/src/api.ts) attaches access-token Bearer authorization
  to direct API calls. Preserve current uncertain-write/retry rules; refreshing
  authentication is not permission to replay an ambiguous product mutation.
- [development-auth.ts](../../infra/development-auth.ts) sets a public SRP-only
  client, Essentials, 15-minute access/ID tokens, one-day rotating refresh,
  ten-second rotation retry grace, token revocation, no OAuth URLs/client secret
  and no device remembering. The broker needs no password database or OAuth move.
- [frontend-hosting.ts](../../infra/frontend-hosting.ts) currently permits only
  GET/HEAD static delivery, strips authorization/cookies and blocks `/api`.
  Its CSP `connect-src` does not include `'self'`. New same-origin auth fetches
  require a CSP amendment and a separate behavior without static request rewriting.
- The recorded UI origin is `https://d2kpq905qyh9zg.cloudfront.net`; API configuration
  uses the existing `https://<api-id>.execute-api.us-east-2.amazonaws.com` origin.
  The exact current API hostname is deployment output, not freshly inventoried
  here. These hosts are different origins and different sites; a direct API cookie
  cannot be a first-party cookie for the UI. Loopback development is another origin.
- Installed Amplify sources reload the token store in `TokenOrchestrator`, write
  multiple keys in `TokenStore`, and catch remote revocation failures inside
  `apis/signOut.ts`. A resolved `signOut()` is not proof of provider revocation.
  These are source-inspection findings, not elapsed live Cognito observations.
- [Go verification](../../services/api/internal/ownership/auth.go) uses configured
  RS256/issuer/JWKS/client/access-token validation and 60-second leeway. Local-agent
  and MCP credentials are independent authorities and must not become cookie auth.

Reviewed the current native proposal and API Contracts authentication/planning
sections, accepted [architecture](../architecture/README.md),
[ADR 0002](../decisions/0002-serverless-persistence-topology.md),
[W1 auth policy](../architecture/w1-review-decisions.md#authentication-and-email),
[hosting contract](../architecture/frontend-hosting-contract.md) and
[onboarding plan](../planning/onboarding-and-desktop.md). Doing was empty before
starting the spike; the configured WIP limit is three.

## Alternatives and compatibility

| Approach                           | Benefit                                                                 | Cost or limitation                                                                                                                         | Disposition                                               |
| ---------------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------- |
| Existing memory only               | Small exposure lifetime, existing behavior                              | Reload loses sign-in                                                                                                                       | Superseded only after delivery                            |
| Amplify `sessionStorage`           | Small reload-persistence change                                         | Script-readable tokens; opener/duplicated tab can copy a refresh lineage into independent stores; ordinary fresh tabs do not share sign-in | Not selected                                              |
| Shared `localStorage` or IndexedDB | Reload/reopening and coherent shared state possible                     | XSS can steal a day-long refresh credential; requires cross-tab locks, atomic snapshots, logout fencing and corruption handling            | Lower-effort alternative, not selected                    |
| Amplify JavaScript CookieStorage   | Cookie persistence                                                      | JavaScript cannot create HttpOnly cookies; still exposes tokens to XSS                                                                     | Not a backend session                                     |
| Cookie directly on API hostname    | Avoids CloudFront auth behavior                                         | Requires SameSite=None, credentialed CORS and third-party-cookie support across sites                                                      | Not recommended                                           |
| Same-origin refresh broker         | Persistent refresh authority server-side; preserves SRP/direct data API | New sensitive store, CSRF boundary and distributed rotation protocol; active XSS can still obtain access tokens                            | Recommended backend scope                                 |
| Full backend for frontend          | Both refresh and access tokens remain server-side                       | Proxy all product/evidence traffic; wider auth, transport, size, caching and deployment changes                                            | Defer unless stronger browser-token isolation is required |

[Amplify session documentation](https://docs.amplify.aws/react/frontend/auth/manage-user-sessions/)
describes configurable token storage. Its defaults are not Kurier's policy.
[OWASP browser-storage guidance](https://cheatsheetseries.owasp.org/cheatsheets/HTML5_Security_Cheat_Sheet.html)
warns that XSS can read stored credentials. Browser-side encryption with a key
available to the same application does not remove that threat.
[MDN sessionStorage](https://developer.mozilla.org/en-US/docs/Web/API/Window/sessionStorage)
documents reload survival, tab partitioning, opener copies and storage denial.
[Web Locks](https://developer.mozilla.org/en-US/docs/Web/API/Web_Locks_API) could
coordinate shared browser storage, but cannot make independent copied stores
authoritative or coordinate Lambda instances. None is needed for correctness of
the selected server-side rotation design.

## Proposed flow and session contract

Reserve `/api/v1/browser-session` on the UI origin for the broker only. Use a new
Go handler behind HTTP API, with application envelopes and strict JSON parsing.
No passwords, tokens or cookies in URLs, query strings, logs, errors or evidence.
All session responses, including errors, have `Cache-Control: no-store`.

POST `/api/v1/browser-session` is the bootstrap: Bearer access token plus JSON
`{refreshToken,attemptId,cancellationProof}` <=16 KiB and `X-Kurier-Session: 1`.
Validate the access token, rotate the supplied refresh credential, and verify the
returned issuer/client/sub/auth_time before creating an opaque session. Return
201 `{data:{accessToken,accessExpiresAt,sessionExpiresAt,csrfToken,sessionBinding}}`
and the cookie. Never return refresh/ID tokens. The attempt protocol is detailed
below. Metadata and token responses also include `sessionBinding`; token and
active-session DELETE requests require `X-Kurier-Session-Binding` to match.

| Method and path                      | Input                            | Response and behavior                                                                                                                                                     |
| ------------------------------------ | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GET `/api/v1/browser-session`        | Cookie; `X-Kurier-Session: 1`    | Read active, unexpired session; 200 `{data:{sessionExpiresAt,csrfToken}}`. No rotation or access token here. 401 if absent/ended.                                         |
| POST `/api/v1/browser-session/token` | Cookie, `X-Kurier-CSRF`, `{}`    | 200 `{data:{accessToken,accessExpiresAt,sessionExpiresAt}}`; use an unexpired encrypted cached token or serialize a rotation.                                             |
| DELETE `/api/v1/browser-session`     | Cookie, `X-Kurier-CSRF`; no body | Durably deny session before provider revoke; expire cookie. 200 `{data:{signedOut:true,providerRevocation:"confirmed"\|"pending"}}`; missing/ended session is idempotent. |

Use a random 256-bit session proof; store only its hash as the lookup identity.
Issue `__Host-kurier-session=<proof>; Secure; HttpOnly; SameSite=Strict; Path=/`
with `Max-Age` equal to the remaining session lifetime and no Domain attribute.
Clear with identical scope and `Max-Age=0`. Cookie attributes and browser restore
semantics are documented by [MDN Set-Cookie](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie).
Missing cookies yield sign-in, never a browser-token-storage fallback.

On SRP success, use an explicit supported custom Amplify memory-storage adapter
to capture the refresh value for the one-time handoff: `fetchAuthSession` does not
publicly return it. Encapsulate and test the pinned SDK key integration, never
import private SDK token-provider APIs. Retain password/email/code handling in
existing Cognito screens. Clear transient SDK storage without invoking provider
sign-out after successful handoff, since sign-out could revoke the adopted session.
Never render the workspace until handoff and an authenticated user read succeed.
An aborted fetch or UI generation fence cannot prevent a late Set-Cookie response.
The handoff therefore also needs a durable bootstrap-attempt receipt: a random
attempt ID and cancellation proof generated before POST, with a short expiry,
conditional create and idempotent result. Retrying that attempt must not rotate
again or create another session. A same-origin cancellation POST authenticated by
that proof must atomically tombstone the attempt and revoke any session it created,
including when cancellation arrives before creation. A late cookie then identifies
a denied session. Never store the proof in URLs/logs or persist refresh material
in the receipt outside its encrypted session bundle.

This additional `/api/v1/browser-session/bootstrap-cancellations` route uses JSON,
the custom session header and the exact-Origin mutation checks; it does not require
an already established session. Bound attempt/proof lengths and receipt lifetime
(propose five minutes), rate-limit unknown attempts and retain cancellation
tombstones through the full bootstrap acceptance/retry window. Delivery contract
review must pin the receipt DTO, retry response and cancellation expiry rules
before implementation; the four primary routes alone are insufficient.

Return a nonsecret session binding ID with metadata/token/bootstrap responses;
require the expected binding on token/DELETE calls. Competing login responses can
replace the browser cookie in arrival order, so a binding mismatch must stop the
operation and revalidate identity, never silently act as the other account. Notify
sibling tabs of replacement without credentials. Sign-out cancels known in-flight
bootstrap attempts as well as the active session; test cancellation-before-create,
late Set-Cookie and competing cross-tab sign-ins. Offline cancellation has the same
pending-server limitation as offline logout below.

Derive the absolute deadline from the rotated token's verified sign-in `auth_time` plus
86,400 seconds, capped by provider validity. Require a freshly issued bootstrap
access token and a matching rotated identity and `auth_time`; reject a supplied
credential for another user/client or another same-user sign-in lineage. Reject
missing/invalid/future `auth_time`. Never accept a browser-provided deadline or reset it during
bootstrap retry/refresh. Cognito remains the authority if the refresh token expires
sooner. Discard returned ID tokens. The UI stops use at the session deadline even
if an access JWT expires later; the direct API's residual JWT window still applies.

[AWS rotation guidance](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-the-refresh-token.html)
confirms rotation retains the original expiry and that rotating clients use
`GetTokensFromRefreshToken`, not `REFRESH_TOKEN_AUTH`. Retain the existing
ten-second retry grace. This is retry tolerance, not a concurrency protocol.
The [operation contract](https://docs.aws.amazon.com/cognito-user-identity-pools/latest/APIReference/API_GetTokensFromRefreshToken.html)
defines returned identity/refresh credentials and client binding.

## Server storage, rotation and sign-out

Propose one dedicated on-demand Sessions table, isolating ephemeral browser
sessions from project evidence and its backup/restore lifecycle. Store hashed
lookup identity, issuer/sub, client, generation, absolute expiry, status, version,
CSRF proof and a bounded encrypted credential bundle (refresh plus cached access).
Use envelope encryption with the stage KMS key and a distinct session encryption
context; add only broker-scoped GenerateDataKey/Decrypt permissions. Existing
product API/worker/MCP roles receive no session decryption. Deliberate encrypted
persistence is an amendment to the browser-auth baseline, not permission to log
credentials or expose them through MCP. Table schema, IAM and capability rollout
must receive delivery review; no resources are created by this spike.

Perform strong reads and conditional version/status/expiry/generation checks.
TTL is physical cleanup only, never authorization. Sessions are disposable: no
PITR restore of login state; changing recovery generation invalidates every old
session. Bound creation per identity (propose five active browser sessions) and
rate-limit bootstrap/refresh, with bounded cleanup of expired/revoked entries.
Do not evict another session silently: return 409 `session_limit` with a sign-out
instruction. An online sign-in can replace the current browser's existing session.

Only one broker instance may rotate a given session at a time. Acquire a durable
lease with version/fencing value, re-read the latest encrypted bundle, refresh
only when necessary, then conditionally commit all new credentials together.
Waiters re-read committed state rather than rotate stale tokens. A module promise
or browser lock is insufficient. Do not hold a DynamoDB transaction over Cognito.

Provider calls have bounded timeouts/retries within the grace interval. A crash
after Cognito success but before commit cannot be atomically resolved across
systems. If no unambiguous committed successor can be recovered, end that session
and require sign-in; never loop on an old token after grace or allow a stale lease
holder to overwrite a new credential. Concurrent sign-out wins: fenced refresh
cannot reactivate a revoked record, and any received late credential is revoked
best effort. Lost bootstrap responses can leave inaccessible sessions: expire
them at the fixed deadline; do not blindly repeat an unknown handoff indefinitely.

DELETE first marks the session revoked durably and invalidates refresh leases.
For an absent/already-ended session, permit idempotent cookie clearance with the
exact-Origin and custom-header checks even if its CSRF metadata is unavailable;
an active session still requires its CSRF proof and expected binding ID.
Provider `RevokeToken` follows; retain encrypted credential only for a bounded
revocation retry until confirmation/absolute expiry, then erase it. A failed
durable revoke returns 503, not a success claim. A successful local cookie clear
alone does not revoke a server session. A provider outage after durable deny may
return `pending`; cleanup retries without resurrecting access.

[AWS revocation documentation](https://docs.aws.amazon.com/cognito/latest/developerguide/token-revocation.html)
explains that offline signature/expiry checks do not observe provider revocation.
Already issued access tokens therefore remain usable at the direct API within
the stated residual window. Sign-out ends this browser session, not every device,
MCP credential or local-agent credential. Global sign-out is separate scope.

## XSS, CSRF and transport boundaries

HttpOnly prevents scripts reading the session cookie, but active XSS can call the
broker, steal its access-token response and perform user-authorized operations.
Retain CSP, escaped text-only evidence rendering, no third-party scripts, no token
diagnostic dumps and dependency review. Add `'self'` to CSP `connect-src` for the
broker. Preserve current direct API/Cognito destinations. A full backend proxy
would be needed to keep even access tokens out of browser JavaScript.

Broker operations require the exact configured UI Origin when present, reject
cross-site Fetch Metadata, and reject missing/null/wrong Origin on all mutations.
Bootstrap requires JSON plus the custom header, including when no cookie exists,
to prevent login CSRF. Same-origin metadata GET uses the custom header and rejects
foreign Origin/Fetch Metadata, including navigations. Token POST and DELETE also
require the synchronizer CSRF token read through that protected metadata route.
CSRF tokens stay in memory; do not put them in URLs/logs. No permissive broker CORS,
no form/simple-content-type variants, no state-changing GET. SameSite is defense
in depth, not the sole defense. Follow [OWASP CSRF guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html).

Add an ordered CloudFront behavior for the exact broker path family before static
fallback. It needs POST/DELETE as well as GET, an API custom origin, zero min/default/
max cache TTL (CachingDisabled), zero error caching, no SPA fallback, and explicit
forwarding of the session cookie, Authorization, Origin, content type and custom
headers. Forward upstream Host correctly for API Gateway, not the viewer hostname.
Pass Set-Cookie and no-store responses unchanged. Never attach the static viewer
function that strips cookies/authorization to this behavior. Restrict the new
origin route to the configured CloudFront distribution using an origin verification
secret checked before session parsing, held outside source/logs. Direct broker
origin calls must fail; CORS alone does not enforce that boundary.

Static behaviors continue stripping cookies/authorization before S3, and all
other `/api` paths remain unavailable on the UI hostname. CloudFront standard/
realtime logs, API Gateway access logs, Lambda logs and observability must exclude
cookies, Authorization and bodies. A session proof must never reach web asset
logs or storage. [CloudFront cookie forwarding](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/Cookies.html)
and [cache policies](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/using-managed-cache-policies.html)
need explicit review: a positive minimum TTL can override no-store.

Product fetches remain `credentials: "omit"` with Bearer authorization to the
existing API origin; preserve its exact-origin CORS and exposed ETag/Location.
Broker fetches use relative UI-origin URLs and same-origin cookies. Do not enable
credentialed cross-site product CORS. Development needs a same-origin Vite proxy
to a local broker; validate forwarded development Origin against an explicit
loopback allowlist. Secure-cookie fidelity requires a local HTTPS fixture. Never
weaken production cookies or send hosted cookies to localhost for convenience.

## Browser lifecycle, errors and migration

Startup state is `restoring`, then authenticated, signed out or temporarily
unavailable. Suppress the sign-in form and private workspace during restoration.
Read session metadata, obtain an access token, then fetch the current user.
401 definitively ends restoration; 429 honors Retry-After; network/503 shows a
retry state without deleting a valid server session or claiming expiry. Broker
403 is an origin/CSRF/configuration failure, not a prompt to retry a product write.

Keep no product data in persistent browser storage. `pagehide` clears private
rendered data and in-memory access/CSRF state without revoking the cookie.
`pageshow`/BFCache restore revalidates before revealing private content. Remove
the current logout-on-navigation behavior and test both paths.

BroadcastChannel may signal sign-out/account replacement to sibling tabs, carrying
only an opaque event/version, never tokens or user data. Server state remains
authoritative if messaging is unavailable. On focus/pageshow validate the session;
before starting a new private operation recheck if validity is uncertain. Other
tabs may retain already-issued access tokens until the residual expiry window.
Use generation fences so a late token/metadata response cannot undo sign-out.

If offline at sign-out, clear UI/token memory and retain a noncredential logout
intent in local storage when available; broadcast it and block restoration until
the server revoke succeeds online. A new login must settle that intent and cancel
the prior known handoff before removing the marker. Say "Signed out here; server sign-out awaits a
connection." Do not promise server revocation. If storage is denied, explain that
an HttpOnly cookie cannot be removed offline by JavaScript and closing/reopening
may restore the session. Provide retry/clear-site-data guidance. This limitation
must be covered by browser tests and operator-facing copy.

Target current Chromium, Firefox and WebKit desktop engines; include mobile Safari
in hosted validation if claimed supported. First-party cookies are required;
third-party-cookie blocking should not break this design. Cookies blocked/cleared
means sign-in or an explicit persistence-unavailable message, without silently
storing refresh tokens elsewhere. Private browsing may erase state on exit.
Web Storage denial must not affect online cookie-session correctness.

Existing users sign in once after the frontend update: memory-only credentials
cannot be recovered from a previous document. Do not migrate credential material
through URLs or local storage. Publish broker/infrastructure first, verify the
no-cache/CSRF boundary, then publish the coordinated frontend. Pin release
compatibility; older frontend releases retain memory-only behavior. Rollback
disables broker creation and revokes/invalidates broker generation before removing
routes; old cookies must fail closed. No evidence-schema or MCP/agent migration.

## Verification and delivery implications

These are **future delivery gates**, not results of this research.

1. Unit/fixture tests: strict route parsing/limits, token/client/sub mismatch,
   deadline calculation, cookie flags, cache headers, CSRF/login CSRF, direct-origin
   denial, expiry checks independent of TTL, generation invalidation and sanitized
   errors/logs. Confirm refresh material never appears in public DTOs/MCP.
2. Storage/concurrency integration: concurrent Lambda-style callers converge on
   one rotation; sign-out during provider response wins; stale leases cannot commit;
   crash before/after provider success and before/after database commit; retry within
   grace, loss beyond grace, provider throttle/outage and cleanup failure. Expired
   rows still physically present must deny access. Test with actual DynamoDB Local;
   real KMS/IAM behavior remains a separate AWS check.
3. Browser fixtures in Chromium/Firefox/WebKit: reload, new/duplicated/concurrent
   tabs, reopen, BFCache, account switch, one-day cutoff using injected clocks,
   expired/revoked cookie, network recovery, storage denial, cookie blocking,
   messaging absence, offline logout and failed revoke. Audit browser stores with
   disposable marker values without printing tokens. No refresh tokens persist.
4. Hosted gates after scoped preview/deployment authorization: actual Cognito SRP
   handoff and subsequent natural 15-minute refresh/rotation; real one-day expiry;
   real browser reopening; real cookie/privacy behavior; cross-user isolation;
   provider revocation; CloudFront no-cache/no Set-Cookie cross-user reuse, direct
   origin denial, CSP, static header stripping and bounded sanitized log inspection.
   Report synthetic clocks separately from elapsed expiry and fixture IdP separately
   from Cognito. Preserve product uncertain-write behavior during auth retry.

Provisional engineering estimates, not measured durations: browser storage with
coordination/verification **8–16 active hours**; selected broker **24–40 hours**
including contracts, Go store/rotation, frontend, fixtures and preview preparation;
full backend proxy **40–64+ hours**. Hosted troubleshooting and elapsed one-day
observation are additional. The broker adds Lambda/API calls, DynamoDB reads/writes,
KMS operations and CloudFront dynamic requests; no standing compute/NAT is needed.
Prepare a regional rate/workload estimate and generated IAM/resource diff during
delivery review rather than presenting unmeasured costs as an existing bill.

Amend W1 authentication, frontend-hosting and native API Contracts with the
accepted detailed flow before delivery. Add a focused session contract/ADR if
accepted; do not rewrite historical verification claims. Keep delivery blocked on
that acceptance and its implementation authorization. The desktop spike should
reuse these trust/lifetime findings without assuming native webviews can inherit
the browser cookie or transferring it into local-agent/MCP credentials.

## Research validation

`npm run verify` passed: repository formatting, ESLint, frontend/infrastructure
and tooling tests, frontend production build, and all configured Go tests/vet and
spike test-binary builds. The first sandboxed run hit `spawnSync git EPERM` in an
existing release test; the rerun with required access passed. The environment
activation suite explicitly skips its optional DynamoDB Local case when no endpoint
is configured. Final documentation changes were formatted and checked separately.
The native API Contracts amendment was read back in its authentication section.

This spike adds documentation only. It contains no working broker, deployed
cookies, browser proof of persistence, live Cognito rotation result or AWS resource
preview. Research completion does not mark the dependent delivery checks complete.
