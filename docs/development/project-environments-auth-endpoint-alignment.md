# Authentication-only operation-endpoint alignment — local verification

The helper now lets the pinned SDK resolve CreateOAuth2Token normally and accepts
only the prepared token request destination
**https://us-east-2.signin.aws.amazon.com/v1/token**. Tooling implementation:
`4a0faafc673e97d9faae3f26906a327a8bd06e71`. Release source remains
`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

This implements the narrow amendment authorized after the
[offline comparison at 1853502](project-environments-auth-path-comparison.md).
No live refresh, probe or preview was executed. It corrects the current endpoint
selection; **historical causation, live connectivity and successful recovery
remain unverified**. Both failed remote refresh outcomes remain uncertain.

## Exact transport change

[Authentication](../../scripts/preview-meter/authentication.py) supplies no
Sign-In endpoint override. The isolated SDK session still fixes us-east-2,
uses static/unsigned credentials for Sign-In, disables proxies and default
credential-provider access, requires TLS verification, and allows one SDK
attempt. The pinned model resolves the token operation after the client is
created; its base hostname still differs from the operation hostname.

| Boundary                           | Before                                                             | Amended behavior                                                                                 |
| ---------------------------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| Client Sign-In endpoint override   | signin.us-east-2.amazonaws.com                                     | None; pinned operation resolver                                                                  |
| Accepted token request destination | https://signin.us-east-2.amazonaws.com/v1/token                    | **https://us-east-2.signin.aws.amazon.com/v1/token only**                                        |
| Enforcement point                  | Prepared request at Transport.forward                              | Same; exact method/URL/grant check before admission and forwarding                               |
| DPoP URI                           | SDK receives the forced endpoint URI                               | SDK receives the normally resolved operation URI; synthetic signed proof htu equals prepared URL |
| STS                                | Existing regional GetCallerIdentity and same identity/account gate | Unchanged                                                                                        |
| SST/provider read-only policy      | Reviewed policy.py                                                 | Byte-identical                                                                                   |

The allowlist is an exact full URL comparison, not a suffix, client-base check or
redirect rule. The old host, other regions/hosts, HTTP, other paths, query
strings and even an explicitly written :443 alternate URL are denied. Exact
POST and the existing client's refresh-token grant are still required. This
change does not authorize additional Sign-In operations or other identities.

The [SDK pin manifest](../../scripts/preview-meter/authentication-sdk.json) keeps
botocore 1.43.108 and urllib3 2.8.0, adding hashes of the installed operation
endpoint rule set, endpoint metadata and resolver source. No SDK upgrade or
installed SDK edit occurred. TLS verification, CA/cipher settings, no-proxy
configuration, ten/thirty-second connect/read timeout caps and the original
shared deadline remain. No retry, redirect, second refresh or extra identity
verification is permitted. Transport errors retain the reviewed sanitized
category/phase/numeric-code diagnostics, without raw request or exception data.

## Accounting and immutable evidence

[Current accounting](../../scripts/preview-meter/accounting.py) verifies all
historical 167-based proofs and the terminal diagnostic's additional connection
before exposing **168** as the authentication/preview baseline. It verifies 21
frozen files, including private diagnostic receipts and the window-from-167
claim. The historical baseline and diagnostic executor remain intact; their
167-based receipts are not rewritten or reinterpreted. No ledger or claim was
removed, resumed or replaced.

The launcher, authentication and handoff use this current baseline. A future
campaign must exclusively claim window-from-168, start one new 3,600-second
window before its first live call, count refresh/STS attempts durably before
forwarding, and pass the consumed count and original deadline to the provider
meter. Refresh is skipped if the existing cache meets the SDK's validity
threshold; identity verification remains mandatory. Failed or uncertain auth,
expiry, limits, policy denial or persistence failure cannot produce a provider
handoff. A new output directory cannot reset or replay the campaign baseline.

**Actual usage: 168/500, zero added attempts, 332 arithmetic headroom.** No
window-from-168 claim or proposed review-5 output exists. Same cumulative **$1
procedural allowance**; billing and unused dollar balance are unmeasured.

Local preflight verified the clean detached release source, prepared packages,
bootstraps, frontend/client artifacts and retained rollback bytes against their
original manifests. No packaging occurred; release artifact-hash differences
are empty. Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`;
read-only preview policy SHA-256 remains
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.

The [machine packet](project-environments-auth-endpoint-alignment.json) records
all eleven changed tooling files' before/after hashes relative to 1853502,
unchanged release/policy hashes, frozen proofs and verification results. The
[manifest](project-environments-auth-endpoint-alignment.sha256) binds this packet
and amended tooling. Earlier manifests remain historical evidence at their
original commits; hashes of intentionally amended tooling differ now.

## Offline verification

**63 unit regressions and 15 process/transport scenarios passed**, together with
full repository format/lint/test/build/Go verification and Python lint/format.
All tests ran with external networking disabled; repository dependency downloads
were disabled. Authentication used generated local keys, synthetic cache values
and mocked physical sends. No real credential provider or refresh ran.

The coverage includes:

- Actual pinned SDK serialization, operation resolution and DPoP generation:
  the synthetic signed proof's htu is the exact prepared token URL and htm is
  POST. The fake physical boundary sees the durable attempt first. Normal SDK,
  CLI-vendored SDK and custom-factory before-call comparisons now agree on the
  operation hostname, while their base hostname still differs from it.
- Prepared URL tampering after resolution/DPoP: the old host is rejected before
  a charge or fake send despite an unchanged client base. No handoff results.
  Old/wrong hosts, another region, wrong path, explicit port, insecure scheme,
  query strings, altered grants and pagination are held.
- TLS verification, region, no-proxy and one-attempt client configuration;
  existing-session/cache restrictions and the STS identity mismatch gate.
- Redirect/error/failed transports charge once with no repeat or handoff;
  sanitized DNS, connection, TLS and timeout diagnostics; uncertain/cache-write/
  expiry outcomes. No credential, DPoP header or body is published.
- Refresh and STS carry synthetic ordinals 169/170 into the provider budget;
  the next provider ordinal is 171, and all phases stop at cumulative 500.
  The provider keeps the original deadline. Expired windows, missing/failed
  outcomes, corrupt proof bytes, reset/reused handoffs and campaign reclaims
  are rejected. Persistence failure prevents forwarding.
- Go/Node descendants share the provider budget; endpoint/mutation denials,
  retries, pagination, redirects and failed transports remain metered, and
  terminal holds stop descendants. The exhaustion fixture now correctly expects
  332 remaining calls from the 168 baseline.

The local `run.py check` result reports priorCalls=168, remainingCalls=332,
awsCalls=0. This proves local pins/retention checks, not live identity, endpoint
reachability, current deployed state or a completed resource diff.

Offline reproduction uses installed dependencies only:

```sh
bwrap --unshare-net --ro-bind / / --bind /home/king /home/king --bind /tmp /tmp --proc /proc --dev /dev env PYTHONDONTWRITEBYTECODE=1 bash scripts/preview-meter/test.sh
```

## Proposed execution — separate review and authorization required

**Do not run now.** The proposal is one new 60-minute refresh/identity/preview
window with at most **332 additional physical AWS attempts**, cumulative ceiling
500 including the preserved 168, and the same cumulative $1 procedural
allowance. It would permit at most one existing-session refresh if needed and
require successful same-identity STS verification before the read-only preview.
Both previous uncertain outcomes must remain acknowledged. No probe, repair,
retry/restart, new identity, endpoint widening or automatic continuation is part
of this proposal.

The packet is a later documentation commit. The exact implementation commit
below must be checked out clean in the original tooling workspace, where the
private historical proofs and retained artifacts already reside. The detached
checkout is proposed only, and has not been performed. The existing feature
branch retains both implementation and packet commits.

```sh
cd /home/king/Neumont/Kurier
git switch --detach 4a0faafc673e97d9faae3f26906a327a8bd06e71
python3 -B scripts/preview-meter/run.py refresh-preview \
  --review-commit 4a0faafc673e97d9faae3f26906a327a8bd06e71 \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-5
```

The pinned child command remains SST 4.17.1 `diff --stage dev-api --json`.
The launcher rejects a dirty/mismatched tooling commit, changed release source
or pins, occupied output or reused campaign claim. After any success or failure,
stop and publish sanitized authentication/provider accounting, denials, any
completed diff and packaging comparisons. No repairs or automatic restarts.

**Stopped for review before all live work.** The next decision is whether to
accept the amended tooling and separately authorize this exact campaign scope.
Deployment, stage closure, migration, activation, identity/project creation and
live target requests remain held. OpenAPI production scope remains separate.
