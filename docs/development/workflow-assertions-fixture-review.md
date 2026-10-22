# Workflow assertions fixture-inclusive final review

Davian accepted execution preparation `c9f55d4980e24c1ec8d57e96a722fd06f7f0271f`
and authorized the proposed controlled-fixture extension. This packet implements
and locally tests that extension, refreshes the clean release and equivalent
preview, and replaces the activator's release pins. It preserves contract
`31de3929aa1423c25b264d9e459aec48728e1c90`, including exact numeric expectations
and MCP legacy projection. **Deployment, stage closure/state writes, activation,
identity creation and live target requests remain held for final review.**

## Implemented fixed fixtures

Source commit: `68d9f218225c7092a1e4fca0b66a21421d862a22`. Only the development
controlled endpoint's implementation/tests and clean-source preview tooling
changed in that source commit. No assertion evaluator/DTO, authoring, API,
accounting, quota, concurrency, cleanup or deployment configuration changed.

Both modes use the existing
`/execution-fixture/{mode}?fixture=<lowercase UUID>` route and require the
existing bounded UUID grammar. Missing, malformed, newline-bearing and oversized
UUIDs return fixed 400 `{"invalidFixture":true}` without an audit record. A
valid invocation emits exactly one existing send-count audit envelope:
`kurierWorkflowFixture`, public `fixtureId`, fixed `mode`, `authorized:false`
and gateway-owned `requestId`. These modes do not inspect authorization headers
or derive a synthetic token. They never log request headers, body, other query
parameters or response values. Existing workflow fixture behavior is preserved.

`assertions-values` returns 200 and these **150 literal UTF-8 bytes**:

```text
{"precise":0.10000000000000001,"short":0.1,"exponent":1E-01,"min":1e-308,"max":9007199254740991,"zero":-0.0,"exists":null,"flag":true,"text":"public"}
```

Response-body SHA-256:
`be88eeacbd302fd531b18b44713578f9d2b3420b628f3595c2201baa5df4ddc8`.
The compact literal has no trailing newline and is never encoded through a
floating-point map or JavaScript Number. The exponent spelling and signed zero
are intentional.

`assertions-not-found` returns 404 and these **20 literal UTF-8 bytes**:

```text
{"unavailable":true}
```

Response-body SHA-256:
`1bbc5eb53033fa10f526c0245b6032760000674c4a28b7df6dd4c2989bb39c52`.
Both responses use the existing base64 Lambda transport, Content-Type
`application/json` and Cache-Control `no-store`. Changing valid UUIDs, headers,
query values or even malformed base64 request bodies cannot change their status,
headers or response bytes. Request input is never reflected in the response.

Local handler tests verify exact bytes/headers/status across distinct UUIDs,
malformed-UUID rejection, one allowlisted audit record per valid call, and no
auth/cookie/body/query reflection. A decoder using `json.Number` verifies numeric
tokens before the real shared evaluator checks both directions of the
0.10000000000000001/0.1 mismatch, equivalent exponent forms, signed zero, minimum
and maximum values, primitive equality and missing versus null. These direct
local invocations make no live target request or AWS call.

## Refreshed clean release

[Release manifest](workflow-assertions-release.json) now pins source/application
commit `68d9f218225c7092a1e4fca0b66a21421d862a22`. The original assertion
implementation `55cd061…` is recorded separately as its accepted baseline.
All four Linux/ARM64 ZIPs embed source revision `68d9f218…` and
**vcs.modified=false**. This release supersedes the earlier preparation's ZIPs;
do not substitute those earlier binaries for the fixture-inclusive release.

| Package, each                                      |     Bytes | ZIP SHA-256                                                        |
| -------------------------------------------------- | --------: | ------------------------------------------------------------------ |
| UsersProjectsApi / EmptyProjectCleanup             | 9,251,049 | `f4be7af179049c70f479a45dd26e4ae24a19c3a9e8016a865318fa7a96965c32` |
| CloudExecutionWorker / ExecutionControlledEndpoint | 8,687,530 | `15e7e6db438de3c19fe962c95be0d3c9d52d17f81b21a0b58cd4603974021379` |

API/cleanup logic is unchanged; their rebuilt package identities include the new
clean build provenance. Worker and controlled endpoint share the worker ZIP, so
both code resources change even though fixture handling is selected only by the
existing controlled-endpoint environment flag. No environment value changes.

Go 1.27.1-X:nodwarf5, Linux/ARM64 v8.0, CGO disabled, `-s -w`, Node 26.11.1,
npm 12.2.1, SST/platform 4.17.1, Pulumi AWS 7.40.0 and existing lockfiles remain
the same. The manifest records source-tree IDs, dependency-file hashes,
bootstrap hashes/sizes and ZIP metadata. No dependency or lockfile changed.

Retained exact bytes live in the repository's ignored directory
`.build/workflow-assertions-release/68d9f218225c7092a1e4fca0b66a21421d862a22/`:
four `<logical-name>/code.zip` files and `frontend/{app,site,artifact.json}`.
No ZIP was uploaded to AWS or GitHub Releases. The published manifest pins the
bytes; preserve them locally or reproduce and re-review any changed hashes.

Frontend content is identical to the accepted preparation: release
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`;
JS SHA-256 `726345aab32ea3f11a8a3075942223794cf33c1c8ad99095dd2fc652bfb97fee`.
Its original five app files and public configuration remain in the manifest.

Release-manifest file SHA-256:
`920667777eb69cabfb7800910451062a18e6ff06ce22ad4fb543f96c78321dd6`.
The manifest explicitly includes both fixture bodies/hashes, UUID/audit policy
and the unchanged campaign limits, with decimal response JSON carried as string
data rather than parsed JavaScript numbers.

## Equivalent guarded preview and activator pins

The preview was built from a separate clean detached checkout of `68d9f218…`.
Installed dependencies/platform were reused through ignored local links; status
was empty before building. The reviewed preview tool now accepts
`--source FULL_COMMIT`, requiring that exact HEAD and a completely clean checkout
before AWS access. No deployment/refresh/activation mode was added. From that
checkout the command was:

```sh
node scripts/preview-workflow-assertions.mjs --diff --source 68d9f218225c7092a1e4fca0b66a21421d862a22
```

Existing SST bootstrap/passphrase/state, local build hooks and RPC side effects
were checked under the accepted 4.17.1 audit before preview. No bootstrap,
passphrase creation, state repair, deployment or fixture invocation ran. After
build/export, only task-generated type formatting/review output was restored to
leave the release checkout clean. Raw preview stays ignored, never published.

The fresh safe export matched all **11 distinct logical changes** from the
accepted preview by name/type/operation/changed properties, independent of event
order: four code-object replacements, four Lambda code updates and three existing
frontend publishing updates. Only package identities changed; frontend bytes
remain identical. There are **no IAM or standing-resource changes**, new routes,
environment/memory/timeout changes, capability writes or concurrency changes.

Observed at 2026-10-20 20:22 MDT: intended non-root account
747336059622/us-east-2/dev-api; bootstrap v5; active version13/savedRequests7,
assertions absent; generation `dev-api-initial-v1`; unchanged quota/project
accounting hashes and zero project accounting rows. Free plan, account10 /
unreserved10, worker batch1/max2/standard pollers, API10/burst20 and enabled
minute maintenance remain unchanged. Before/after bootstrap, all 88 current
state-object metadata entries, passphrase metadata, live code/env/IAM, generation
and accounting checks passed unchanged. These are observations, not mutations.

The refreshed [reader baseline](workflow-assertions-reader-baseline.json) covers
managed policy versions/documents, inline/trust hashes, existing package/config
and controls. Its digest is bound into the release manifest. The activator now
uses a pure release-pin verifier that checks the **exact manifest digest**, clean
source/application revision, baseline digest, four unique expected package
names, fixed ZIP hashes, clean ARM64 build metadata, preview code keys and local
archive bytes before AWS access. Old preparation manifests, changed fixture
metadata, different baselines and stale/substituted ZIPs are rejected locally.

The activator still defaults to read-only inspection. Its full-marker CAS,
compatible backend/frontend gates, payload-free inventories, residual-prefix
proofs, atomic counter conditions, explicit separate write approval, bounded
transaction, exact readback and one-attempt/no-reopen behavior are unchanged.
No `--close`/`--activate` AWS invocation was run. Refer to the accepted
[activation procedure](workflow-assertions-execution-package.md#guarded-activator)
and [deployment/rollback gates](workflow-assertions-deployment-review.md).

## Same bounded runtime campaign, still unperformed

H–L now have implemented, locally verified controlled responses; their separate
fixture-design/implementation prerequisite is satisfied. **That does not make
them live runtime evidence or authorize any send.** The full A–L campaign can be
reviewed against this one fixture-inclusive release before initial activation.
Do not deploy the older preparation first and then silently replace the worker.

Preserve every limit from the accepted preparation:

- 24 planned + 4 reserved = **28 admitted steps maximum**, at most 14 two-step
  submissions and 28 worker target sends; zero direct fixture probes.
- **Two disposable identities and two disposable projects**, at most six saved
  requests/twelve workflow definitions and one project-scoped MCP credential.
- **$1.00 incremental allowance**, two-hour window including 20-minute cleanup;
  at most 500 API calls including 240 status polls (two-second minimum interval),
  and 1,500 scoped AWS reads. This is a procedural allowance, not a guaranteed
  AWS billing cap; preserve existing quotas/concurrency and budget resources.

The existing exact controlled origin remains
`https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/`.
The target allowlist is the prior `workflow-token`, `workflow-consume`,
`workflow-failure` plus `assertions-values` and `assertions-not-found`. Each
future send uses its unique public fixture UUID; correlate the target's audit
records with run/execution IDs, not only job rows. No arbitrary endpoint or
external fixture service is introduced. Expected A–L total stays **20 sends**
across 24 charged steps because four first-step failures skip their successor.

The accepted A–G cases stay unchanged. A seeds legacy evidence before maintenance
closure, only if separately authorized and fully settled. H uses assertions-values
for exact numbers plus presence/status within eight checks per step; I reruns
that frozen run after an unrelated definition edit, preserving authored tokens;
J expects precise decimal against short actual and fails before its successor;
K checks primitive/null/presence values then matches expected 404 from
assertions-not-found while HTTP policy still fails the run; L distinguishes
missing from present null and skips the successor. These add the same ten
admitted steps/eight expected sends previously reserved for H–L.

Use the same real Cognito SRP, owner/anonymous/MCP denial, legacy/schema2/MCP
projection, immutable-checksum, pin/unpin and exact browser/API/save/freeze/reopen/
unrelated-edit/rerun checks, within those limits. Unsupported numeric DTOs and
domain/length violations fail before admission with zero target sends. Stop on
an unknown outcome or unexpected send; reserve steps never authorize ambiguous
HTTP replay. Normal 202 project drain, physical S3/Control/Protected checks,
healthy accounting and preserved admission usage remain mandatory. Revoke/delete
only the two disposable provider identities after project cleanup; retain
expected tombstones/residual/audit/identity metadata under current policies.
Do not claim immediate offline-JWT invalidation from deleting a Cognito user.

Broad live fault injection, elapsed-retention/late-upload windows and PITR/restore
stay separate. Cleanup must fit the same bounds or remain an explicit open gap;
never reset counters, erase quota usage, alter clocks or delete unrelated rows.
No standing-resource or fixed-price increment is introduced by these 170 fixed
response bytes. The prior planning allowance/rate assumptions remain sufficient;
actual live cost, IAM execution and runtime behavior are still unmeasured.

## Completed verification and final hold

Formatting/gofmt, ESLint, 64 frontend tests, 15 infrastructure tests and the
TypeScript/Vite build passed. Go tests/vet across all seven modules passed after
rerunning the Go portion with build-cache access when the sandbox denied it.
The changed worker suite also passed `go test -race ./...`, including the new
literal/audit/UUID/evaluator cases and existing workflow fixture tests.
All affected review/activation JavaScript passed recommended Node ESLint and
syntax checks. Invalid preview arguments and mismatched source pins were
rejected before AWS access.

`node scripts/workflow-assertions-release.test.mjs` passed all six checks,
including the actual four retained ZIPs. The 43 non-DynamoDB activator regression
checks passed; its optional DynamoDB Local transaction case was not rerun because
the coordination core is unchanged from the accepted preparation's 44-test run.
No local Docker resources were created by this continuation. All five retained
frontend app files and both literal response hashes were independently checked.

The refreshed [actual read-only preflight](workflow-assertions-activation-preflight.json)
is bound to this release manifest: active version13 and unchanged generation/
accounting, writes:0, backendReadersReady:false and frontendReady:false. Those
false readiness values correctly reflect that nothing has been deployed.

Publish this packet for acceptance with exact clean artifacts and pins. Before
eventual deployment, recheck live invariants and preserve prior rollback bytes;
do not rebuild from the later tooling/documentation commit and assume matching
ZIP identities. The accepted closed-stage deployment order, guarded activation,
no automatic reopen/replay and post-schema2 rollback limits remain in force.
No AWS deployment, stage-state write/closure, activation, identity creation,
live target request, broad fault injection or elapsed-retention/PITR exercise
was performed. Those actions still need separate authorization after this review.
