# Browser refresh-session broker

Accepted direction: Davian selected backend persistence and one-day reopening on
2026-11-11. After reviewing the research, he authorized proceeding with the broker
subject to clarifying orphan-session recovery, bootstrap replay and CloudFront
error replacement. This contract incorporates those corrections. Scoped code,
infrastructure definitions, tests and documentation are authorized; live deployment
still follows the existing SST preview/review workflow. No session dashboard or
new deployment framework is included.

The [research](../spikes/browser-session-persistence.md) explains alternatives and
threats. This focused contract supersedes its unresolved details. Implementation
and actual validation are tracked [separately](../development/browser-session.md).

## Persistence and trust

Keep browser Cognito SRP and the existing public Essentials client. Transfer the
refresh credential once over TLS into an encrypted server-side session. Access
tokens remain in browser memory and authenticate direct API requests; product
API cookies are omitted. The UI-origin broker uses a host-only
`__Host-kurier-session` cookie, Secure, HttpOnly, SameSite=Strict, Path=/, with
Max-Age bounded by original Cognito `auth_time` + 86,400 seconds. Rotation and
retry do not extend that deadline. Browser reopen restores within it. No refresh
credential is persisted in browser storage, URLs, evidence or MCP.

The broker verifies bootstrap and rotated access tokens against the configured
issuer, RS256/JWKS, client_id, token_use, sub, iat, exp and matching auth_time.
Different same-user sign-in lineages cannot be combined. Normal Go API ownership
and application-user checks remain. Immediate access-token revocation is not
added: a previously issued access token can remain accepted for its 15-minute
lifetime plus existing 60-second tolerance. Active XSS can request short-lived
access tokens even though it cannot read the cookie.

## Orphan sessions and admission

**Defer the proposed five-session limit.** Fresh authentication is never denied
solely because cookies were lost, sign-out was unavailable or old bootstrap
responses disappeared. Keep bounded per-identity bootstrap admission, per-session
refresh throttling, API throttles, fixed expiry and TTL cleanup. This does not
introduce a list/revoke dashboard or global sign-out. Old inaccessible sessions
expire normally; fresh login may replace the current accessible cookie session.

## Routes and bootstrap replay

All routes live only under `/api/v1/browser-session` on the UI origin. Return
application JSON envelopes and `Cache-Control: no-store` on success and failure.
Require `X-Kurier-Session: 1`, reject query strings and foreign/null Origin,
cross-site metadata and navigation. Mutations require exact configured Origin.
Use strict bounded JSON. No permissive CORS, request/response logging or tokens
in diagnostics.

| Route                           | Request                                                                                                | Result                                                                                                                   |
| ------------------------------- | ------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ |
| POST base                       | Bearer original fresh access token; `{refreshToken,attemptId,cancellationProof,sessionProof}` <=16 KiB | Bootstrap or replay; 201 data with accessToken, accessExpiresAt, sessionExpiresAt, csrfToken, sessionBinding; Set-Cookie |
| GET base                        | Session cookie and custom header                                                                       | 200 data with sessionExpiresAt, csrfToken, sessionBinding; no rotation                                                   |
| POST `/token`                   | Cookie, `{}`, X-Kurier-CSRF and X-Kurier-Session-Binding                                               | Cached or serialized refreshed access token and expiry/binding; never refresh/ID tokens                                  |
| DELETE base                     | Cookie, CSRF and binding; no body                                                                      | Durably deny session, expire cookie; data signedOut and providerRevocation confirmed/pending                             |
| POST `/bootstrap-cancellations` | `{attemptId,cancellationProof}`                                                                        | Cancel attempt and any created session, including cancellation before creation                                           |

Before its first bootstrap request the browser generates independent 32-byte
cryptographically random base64url values for attemptId, cancellationProof and
sessionProof. They remain in memory during handoff/retry. The server stores hashes
of proofs; sessionProof is the future cookie value, never returned in JSON.

A receipt binds attempt ID, proof hashes, verified issuer/sub/client/auth_time,
original bootstrap request identity and one fixed session binding. Creation is
conditional. A completed receipt references the encrypted session credential
bundle. A retry presents the same original valid fresh Bearer token, body and
proofs. After checking all bindings and active session state, the broker reissues
**the supplied sessionProof as the same cookie** and returns the committed session
result, without another Cognito rotation or session creation. Knowing an attempt
ID alone is insufficient. Changed proofs/identity/request are rejected.

Accept bootstrap/retry only within five minutes of the original verified access
token issuance and before its expiry and the fixed session deadline. Retain
receipts and cancellation tombstones for at least ten minutes from creation,
longer than the bootstrap window plus clock tolerance. TTL deletion does not
authorize an expired attempt. If a document unload loses its in-memory attempt
proofs, start a fresh SRP login instead of recovering an old cookie by identity.
There is no session-count lockout. Provider-success/storage-failure uncertainty
fails closed and requires fresh sign-in when the committed successor is unknown.

Cancellation authenticates with attempt ID and cancellation proof under the same
origin/custom-header checks. Atomically tombstone even an unknown attempt. A
later creation with that attempt must fail; a completed attempt's associated
session is durably denied. A late Set-Cookie may still arrive, but it references
an ended session. Retry never revives cancellation or expiry. Session-binding
headers stop a competing login's cookie from silently switching the account used
for an already-running operation. Missing/ended-session DELETE can clear cookies
using origin/custom-header checks without unavailable CSRF metadata; active
sessions always require CSRF and binding.

## Storage and concurrency

Use a dedicated on-demand TTL Sessions table without PITR. No authoritative
session-count index. Strong reads and conditional updates bind the current active
Control stage/recovery generation. A recovery-generation change invalidates old
sessions. Hash lookup proofs; envelope-encrypt refresh/cached access credentials
under the existing stage KMS key with purpose-separated context and narrowly
scoped broker permissions. No other runtime gains session decryption.

Serialize refresh with durable leases and version fencing. Re-read the latest
bundle after acquiring the lease. Commit successor credentials atomically; an
expired lease or revoked session cannot accept a late write. Never use a browser
lock as server coordination. Retain Cognito's ten-second retry grace. Fail closed
when a lost provider result cannot be recovered unambiguously. Sign-out denies
server session state before provider revocation; bounded scheduled retries retain
only encrypted material until provider confirmation or original expiry. Do not
claim provider success on a failed request.

## Hosting and error preservation

Only exact base and child broker paths get uncached CloudFront API behaviors,
without the static viewer-request function. Forward cookie, Authorization,
Origin, content type and session/CSRF/binding/Fetch Metadata headers. Use the API
origin Host. Authenticate origin forwarding with a private origin header; deny
direct-origin broker calls. Static paths retain cookie/authorization stripping.
Add `'self'` to CSP connect-src; keep direct product API CORS unchanged.

**Remove distribution-wide custom-error body/status replacements**, including the
existing 403 → 404.html mapping. Custom-error entries may only set zero error TTL,
without ResponseCode or ResponsePagePath. Static viewer routing handles missing
files in its own behavior. Broker 400/401/403/409/429/503 status and JSON body pass
through unchanged, including Set-Cookie and no-store headers. An uncached behavior
alone does not override distribution-level custom errors. See
[AWS custom-error behavior](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/GeneratingCustomErrorResponses.html).

Verification must include requests **through CloudFront** after reviewed
deployment: wrong Origin/CSRF, missing/ended cookie and throttled/unavailable
broker responses retain status, JSON envelope and no-store; no 404.html substitution
or cross-user caching. Configuration/unit fixtures are recorded separately and
cannot substitute for this live gate.

## Browser lifecycle and rollout

Gate the coordinated hosted build with the public broker capability flag. Old
builds/local fixtures retain the previous memory-only behavior when absent.
Restore before showing private workspace or sign-in form. Transient failures
show retry; definitive expiry returns sign-in. Pagehide clears private UI and
memory without revoking the cookie; BFCache return revalidates. Sign-out uses
generation fences and cancels known handoffs. Broadcast events carry no tokens.
Offline logout clears local UI but explicitly reports pending server sign-out;
persist only a noncredential intent marker when storage permits. Do not clear it
on fresh login until prior revocation/cancellation is settled.

Publish/validate broker infrastructure before its frontend. Keep Free plan,
quotas, concurrency and stage capabilities unchanged. Rollback invalidates broker
sessions before retiring routes; it never replays product mutations or migrates
evidence/MCP/agent authority. Validate real rotation, reopening and elapsed expiry
separately from synthetic clock/provider fixtures.
