# Shared authentication and preview budget — local implementation

The handoff proposed at `671eea4664cf17ab54ccd4a307d3cb4fe91dac95` is implemented
for review. This step made **zero AWS calls** and ran no live refresh or preview.
The user's authorization to amend launcher/accounting tooling supersedes the
old identical0d1defa restriction only for this handoff. Execution remains held.
Release source remains `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

## Implemented behavior

The new `refresh-preview` action replaces the old `preview` action. It requires
an exact clean reviewed HEAD, clean pinned release source, pinned SST4.17.1 and
AWS provider7.40.0, pinned authentication SDK files, verified original ledgers,
prepared artifacts and rollback bytes, and a fresh private output directory.
The `check` action performs only local checks and does not start a window.
Before beginning, the live action exclusively claims the cumulative165 campaign
in `.build/project-environments-preview/window-from-165.json`. A second
invocation cannot evade the single-window limit by choosing another output
path. The claim is never deleted or repaired automatically; later authorization
requires reviewed accounting for any attempts consumed.

Before any authentication network work, `handoff.begin()` exclusively creates
and fsyncs a host-only window receipt with cumulative prior165, ceiling500,
3,600seconds, one monotonic start/deadline, wall start timestamp, boot ID,
source, directory binding and both original ledger hashes. All local preparation
after this point consumes the same time allowance. No resume/reset mode exists.

Authentication runs synchronously in the host launcher using the installed,
source-pinned **botocore1.43.108 LoginCredentialFetcher**, not a CLI subprocess.
It reads the existing default CLI login session only. No default credential
chain, IMDS, credential process, interactive login or identity creation runs.
SDK config/credential files and custom data paths are disabled; endpoints,
region, TLS verification, static/unsigned credentials and empty proxy config
are explicit. SDK retries are disabled. Logging is suppressed for this phase.
The process uses an original-deadline alarm to interrupt DNS/connect/read or SDK
processing; authentication has no child processes.

The dedicated authentication transport permits only:

| Endpoint                                          | Operation and constraint                                                                                                     |
| ------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `https://signin.us-east-2.amazonaws.com/v1/token` | POST `CreateOAuth2Token`, exact existing client ID and refresh token, `grantType=refresh_token`; no authorization-code grant |
| `https://sts.us-east-2.amazonaws.com/`            | POST `GetCallerIdentity`, exact action/version only                                                                          |

The window directory and new ledger directory entries are fsynced as well as
their file contents. Every physical send is admitted and fsynced before invoking the pinned SDK's
`URLLib3Session.send`. That pinned method disables urllib3 retries; the SDK also
has total_max_attempts1. Transport errors, non2xx responses, redirects and repeated
operations latch a stop. Pagination/extra parameters are denied in authentication.
No failed authentication is retried. The independent provider gate continues to
meter provider retries/pages/redirect responses using its existing policy.

The existing cache's account must be747336059622 and its configured ARN must be
an IAM user in that account. The original cache file is opened without creation
or symlink following, locked, and is the **only credential file written**; the
refreshed token is persisted to that same inode at mode0600 and fsynced. No
credential copies are kept in run artifacts. A partial cache write is terminal;
the tooling does not repair it. Cache metadata is the pre-refresh identity check;
fresh live identity cannot be established using expired credentials. After
refresh, STS must return the exact same account and configured non-root ARN
before any provider starts. Identity response material stays in memory.

If the existing cached session has more than300seconds validity, the SDK skips
the token exchange; the identity read is still metered. Otherwise there is at
most one refresh and one identity read. Credentials must have at least120seconds
remaining at handoff. Expiry during preview causes a stop; there is no further
refresh. Whether the real session can refresh remains unverified live.

A successful authentication ledger must have contiguous ordinals starting166,
matched successful outcomes, allowed phase order, a final verified-identity
marker, and monotonic timestamps inside the original window. Malformed,
truncated, stopped, failed, pending, duplicated or unexpected records cannot
produce a handoff. The sealed receipt hashes the window and authentication
ledger and derives prior usage from the validated attempts, not operator input.

The launcher validates the receipt before starting the proxy. The host proxy
independently revalidates it and atomically creates a single-use claim before
starting its SharedBudget. A claim survives failure; reuse, concurrent claims,
copying the window to a new path, changed boot, changed limits/counts/hashes,
expired windows and direct production budget overrides are rejected. Host-side
receipts and claims are outside all child mounts. The trusted operator's host
filesystem is the trust boundary; this is not protection against an operator
rewriting all host proof files. No provider can reset or edit its host budget.

The SharedBudget inherits the reviewed ledger/check/admit/outcome implementation
but uses the **original** monotonic start. The supervisor also uses that original
deadline. It never starts another60-minute clock. For one refresh and one STS
read, provider prior is167, its first ordinal is168, and333 provider attempts
remain. With only STS, prior is166 and334 remain. Total new attempts across both
phases can never exceed335; cumulative usage cannot exceed500.

The preview `policy.py` is byte-identical at SHA-256
`ed237fbf3fe24ccd06e5ed08a73df641499e5c19c9342c56f14e93198764a560`.
Authentication grants are not added to it. SST metadata writes, application and
infrastructure mutations and live target endpoints remain denied. The same
SST/Pulumi/Go/Node child isolation, relay and host proxy are retained.

## Local evidence

The new22 offline tests cover valid/invalid/truncated ledgers, sequence and hash
checks, outcome completeness, changed window/source/boot/limits, reset and reused
handoffs, exclusive/racing claims, cross-phase500-call exhaustion, original
deadline continuity, alarm interruption, persistence failure before forwarding,
unknown endpoints/grants/pagination, retries, redirects and uncertain transports.
They exercise the actual pinned SDK's serialization, DPoP/signing, parsing and
cache-save path with fake raw sends and sockets explicitly disabled. Failed
HTTP, malformed responses, expired credentials, mismatched identity and cache
write failure produce no handoff. A fake home/cache exercises the real local
cache loader/writer without touching the actual AWS cache.

The transport suite retains its14 existing offline SST/Pulumi/Go/Node cases and
adds a production-proxy-branch handoff case: a synthetic two-attempt auth ledger
is consumed, the proxy starts at167 with the original deadline, a denied endpoint
forwards no request, and parent/child/grandchild pulse files stop growing. The
entire transport suite runs inside an outer namespace with no external route.
Synthetic auth receipts are fixture evidence, not live usage or identity proof.

Verification commands (local only):

```sh
bash scripts/preview-meter/test.sh
.build/attempt-meter-venv/bin/ruff check scripts/preview-meter
.build/attempt-meter-venv/bin/ruff format --check scripts/preview-meter
python3 -B scripts/preview-meter/run.py check
npm run verify
```

[Test receipt](project-environments-shared-budget-tests.json) and
[source hashes](project-environments-shared-budget.sha256) are published with this
packet. All verification commands above passed. Historical
checksum manifests remain unchanged: expected historical run/proxy/test files
now differ because the handoff was explicitly amended. The new shared-budget
manifest records their current bytes. Original policy, ledger, source, release,
frontend, native client and rollback byte verification remains required and
passed locally. No release packaging changed; no new deployment artifact hashes
exist. Release-manifest SHA-256 remains
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.

## Proposed execution and required next review

The exact command with this newly published commit's literal review SHA is
recorded in [PR #18](https://github.com/VioletProto/Kurier/pull/18). It selects
`refresh-preview`, the unchanged release worktree, and fresh output
`/home/king/Neumont/Kurier/.build/project-environments-preview/review-3`.
That command has **not** run; local check/tests do not authorize it.

Its private output will contain window.json, authentication.jsonl, handoff.json,
provider-claim.json, provider/attempts.jsonl and cumulative-usage.json, plus the
existing private provider output and packaging comparison. Failed auth/launch
also writes conservative terminal usage; malformed/truncated accounting reports
an unknown cumulative count requiring inspection rather than claiming zero.
No restart or reusable handoff is allowed after success or failure. Credentials,
headers, bodies and raw diagnostics remain private; publication needs sanitized
review. No complete live resource diff exists from this local-only step.

Actual cumulative usage is unchanged at **135+30=165/500**. No live window was
started;335 calls are arithmetic headroom. The cumulative **$1 procedural
allowance** remains unchanged; billing and unused dollar balance are unmeasured.
Review this tooling and its command before separately authorizing any live
refresh/preview. Existing AWS configuration, session freshness and refresh
success require live evidence and are not asserted here. All deployment,
stage-closure, migration, activation, identity/project creation and live target
holds remain. OpenAPI production scope remains separate.
