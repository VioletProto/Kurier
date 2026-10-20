# Workflow assertions execution preparation

Davian accepted the deployment approach, temporary maintenance closure and
rollback limitations in packet `0f38a19998c5e50f064b7fb588d0a099a8ba9ce3`.
This continuation prepares clean artifacts, an equivalent read-only preview,
a locally tested activator and a bounded genuine-runtime plan. **Deployment,
stage-state writes, activation and target requests remain held until this
execution package is reviewed.** No application or fixture implementation is
included. The [accepted deployment packet](workflow-assertions-deployment-review.md)
continues to define the deployment order, compatibility, security and rollback
constraints. This package does not authorize executing the commands below.

## Pinned release and fresh preview

The release source is clean detached commit
`0f38a19998c5e50f064b7fb588d0a099a8ba9ce3`; its services/apps/infra trees match
application commit `55cd061c0f6607037ebd61e200fe54ab296237c4`. New activator and
review documents are deliberately outside that release checkout.
[Release manifest](workflow-assertions-release.json) records source tree IDs,
toolchain/build metadata, bootstrap and ZIP hashes, current baseline, frontend
manifest and the fresh sanitized preview. Every Go binary embeds that release
revision and **vcs.modified=false**. No build-VCS metadata was disabled or faked.

| ZIP                                                      |     Bytes | SHA-256                                                            |
| -------------------------------------------------------- | --------: | ------------------------------------------------------------------ |
| UsersProjectsApi + EmptyProjectCleanup, each             | 9,250,902 | `da71939d375ea757380f33444552917984823a94fa87847bcd02e19be324b4fb` |
| CloudExecutionWorker + ExecutionControlledEndpoint, each | 8,684,843 | `9ab72e89f5366904c9e9516aa1dec05e04ddff020ac08cc3431d4db1ef57d436` |

Go 1.27.1-X:nodwarf5, Linux/ARM64 v8.0, CGO disabled, `-s -w`; SST/platform
4.17.1, Pulumi AWS 7.40.0 and repository lockfiles remain pinned. Existing
installed dependencies were reused; there was no dependency upgrade/install.
Frontend release is unchanged from the accepted preview:
`bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde`.
JS SHA-256 is `726345aab32ea3f11a8a3075942223794cf33c1c8ad99095dd2fc652bfb97fee`;
all five app-file sizes/hashes and public config are in the manifest.

Exact bytes are retained locally under the repository's ignored
`.build/workflow-assertions-release/0f38a19998c5e50f064b7fb588d0a099a8ba9ce3/`:
four `<logical-name>/code.zip` files and `frontend/{app,site,artifact.json}`.
Raw preview is retained there privately as well; never upload/paste raw state.
ZIPs are not committed or uploaded to AWS/GitHub Releases by this preparation.
The published record pins their hashes; the operator must retain these bytes
or reproduce and re-review any changed hashes. Do not rebuild the Go ZIPs from
the execution-tooling commit and assume they are this release.
Release-manifest file SHA-256:
`c01401ce0839694ffe1da7ab8fd3fc7eb3de16a8aa3ea4a25f780ab5b11ed012`.

Preparation used a clean detached checkout under `/tmp`, with ignored dependency
and platform links and a local copy of public SST outputs. Before building,
`git status --porcelain --untracked-files=all` was empty. From that checkout,
`node scripts/preview-workflow-assertions.mjs --diff` ran after the same pinned
bootstrap/passphrase/RPC audit and guards as the accepted packet. The safe
exporter verified local ZIPs against preview S3 keys. Generated SST type-file
formatting was restored after builds, as in the prior packet. A later exporter
write only generated local review output; it did not alter the built binaries.

The fresh preview is equivalent **by logical resource name/type/operation and
changed properties**, rather than nondeterministic event order. It has the same
11 changes: four code-object replacements, four Lambda code updates, three
existing frontend upload/KV/invalidation updates. Frontend bytes are identical;
only the clean ZIP hashes differ. No IAM, standing infrastructure, routes,
environment, memory, timeout, concurrency or stage marker changes are planned.
Before/after bootstrap, passphrase/state metadata, live code/env/IAM, generation
and accounting checks passed unchanged. The stage remains savedRequests7,
assertions absent, active version 13; Free plan/concurrency10/worker max2 and
existing quota limits are preserved. Recheck before an eventual deployment.

The additional [reader baseline](workflow-assertions-reader-baseline.json)
records full managed-policy-version/document hashes as well as inline/trust
configuration, current package identities, concurrency, queue mapping and minute
schedule. It contains selected metadata/hashes, not environments, tokens or
request contents. Its file SHA-256 is bound into the release manifest.

## Guarded activator

`node scripts/activate-workflow-assertions.mjs` (or `--inspect`) is read-only.
It verifies local pinned ZIP bytes and baseline hash, the intended non-root
account, region/tables, stage, complete compatible-reader configuration/IAM,
Free plan, API throttles, concurrency, queue and payload-free coordination
inventory. It reports marker/accounting hashes suitable for a later reviewed
approval record. It does not create missing markers/counters, deploy, send
HTTP, settle jobs, clean up data or open the stage.

`--close APPROVAL.json` and `--activate APPROVAL.json` are **separate explicit
write modes**, implemented for later approval and tested only against local
fixtures. They require a clean checkout of the reviewed execution-package
commit and `KURIER_ASSERTIONS_WRITE_APPROVAL=<that-commit>:close` or `:activate`.
The local approval JSON has exactly these fields:

```json
{
  "action": "close",
  "reviewCommit": "<reviewed execution-package Git commit>",
  "manifestSha256": "<SHA-256 of workflow-assertions-release.json bytes>",
  "markerSha256": "<fresh --inspect markerSha256>",
  "accountingSha256": "<fresh --inspect accountingSha256>"
}
```

This is an operator guard, not proof of human authorization or a substitute for
least-privilege IAM. Filling it in does not lift the user holds. Keep approval
files outside tracked source; never add credentials to them. Closure approval
does not authorize activation. After closure/deployment, inspect again and obtain
a separate activation approval bound to the new closed marker. Version numbers
are incremented exactly with BigInt; no stage version or generation is reset.

Both operations require savedRequests7, assertions marker absent, all seven
existing capability versions exactly 1, expected schema/kind/key and a fresh
complete marker hash. Unknown fields/versions fail closed. Closure changes only
active→`workflow-migration` and version+1. Activation requires that closed state
and verified new API/cleanup/worker/controlled-fixture ZIP identities, unchanged
configuration/IAM, deployed frontend manifest/file hashes and KV asset entries;
then changes only savedRequests7→8, assertions absent→1, state→active, version+1.
CloudFront distribution must be Deployed. Static reads are AWS S3/KV inventory,
not browser/runtime proof. The frontend publish/invalidation completion and
required user reload remain operator deployment gates from the accepted packet.

The activator scans both tables with explicit metadata projections: no plans,
configurationJSON, envelopes, auth/cookie values, credentials or bodies. It
rejects nonterminal jobs/runs/steps, pending OUT, buffers/bindings/local waits,
unsettled uploads, deleting work, unhealthy accounting and unknown kinds/schema.
Published uploads require matching terminal job, settled writer and immutable
manifest metadata. Queue visible/inflight/delayed counts must each be observed
zero; approximate SQS counts are an additional check, not an atomic drain proof.

Code inspection and the real inventory showed **permanent residual sweep
registrations** for two already deleted projects. They are intentional under
`execution/maintenance.go`, not pending jobs to delete. They are accepted only
with the matching deleted tombstone, no other Control/Protected project rows,
and an observed empty S3 project prefix; otherwise preparation stops. Preserve
the registrations and future hourly sweeps. Known cleanup/local-agent metadata
kinds remain readable, while unknown kinds still stop the operation. No
residuals, credentials, cursors or accounting rows were modified in this task.

Before a write, repeat inventory/platform/marker checks. One bounded DynamoDB
transaction contains the complete marker CAS plus conditional checks for every
observed project/quota row: kind/schema/version and all existing admission,
pin, inflight, storage, month/count fields, including absence conditions.
It changes no accounting fields. Refuse >80 actions or >2 MiB serialized input.
Re-read the exact expected whole marker and unchanged accounting after success.
The stage guard fences normal admissions across closure; strong scans alone are
not snapshots and do not fence administrators bypassing the version protocol.

AWS CLI retries are disabled (`AWS_MAX_ATTEMPTS=1`); the coordinator issues one
transaction attempt. On conflict, lost response, bad readback or later drift,
there is **no retry, compensating write, automatic reopen or downgrade**.
Read durable state and seek separate recovery authorization. A lost activation
ACK can mean the stage is already active8; do not claim every failure leaves it
closed. Closing does not cancel an HTTP request already in flight. Keep the
accepted quiescence/settlement gate before closure. Rollback after new shapes
still requires compatible readers or a fix forward, preserving immutable bytes.

## Bounded genuine-runtime plan — unperformed

Future approval must identify a start time, unique random UUID run tag and the
operator, and explicitly authorize identity/project creation/deletion, activation
and these target sends. Use only account 747336059622/us-east-2/dev-api.
Create **two disposable Cognito identities** in the existing pool
`us-east-2_qTDZQT1FE`, using `AdminCreateUser` with MessageAction SUPPRESS and
permanent random passwords held in RAM. Use unique `example.invalid` fixture
emails, not an existing user's address; do not modify pool/client/sender/IAM.
Perform genuine Cognito SRP sign-in as owner A and negative owner B. This tests
provider auth, not normal signup, verification-email delivery or the default
sender's production readiness. Delete exactly those identities afterward.

Create **two projects**, one per disposable owner, named with the run tag.
Owner A holds at most six saved requests, twelve workflow definitions and one
MCP credential scoped only to its project. Owner B's empty project is for
ownership denial. No access grants to unrelated projects, real credentials or
user endpoints. Record generated IDs in a private local cleanup ledger; keep
passwords/refresh/access/MCP tokens solely in memory, never argv/logs/files.

Hard procedural limits for the whole reviewed campaign, including repeats:

- **24 planned admitted steps**, twelve two-step submissions, plus at most
  **4 reserved steps**: **28 admitted steps / 14 two-step submissions maximum**.
  Quota charges all configured steps, including skipped successors. Owner A's
  day-count totals and the stage month-count delta must reflect actual usage;
  preserve the existing 100/day and 1,000/month limits and all other accounting.
- At most **28 worker target HTTP sends**, zero direct fixture probes and zero
  extra external targets. Expected planned total is 20 sends because four
  failed first steps skip their successor. Reserve is available only for a
  separately recorded deterministic correction with known prior outcome;
  an ambiguous send never authorizes resubmission or automatic HTTP replay.
- At most **500 application API calls including 240 status polls**, polls at
  least two seconds apart, immediate responses only; no SSE/long polling.
  At most 1,500 scoped AWS inventory/log reads. Abort on budget exhaustion,
  unexpected target send, accounting mismatch or unavailable send-count proof.
- **Two-hour test window**, including a 20-minute normal cleanup observation
  window. **$1.00 incremental allowance**, including two Cognito MAUs and cleanup.
  The same captured Ohio rates give a conservative sub-$0.10 planning estimate
  for this tiny fixed-body campaign (28 × existing 90-second worker timeout at
  256 MiB is about $0.0084 compute; two Essentials MAUs about $0.03). Metering is
  delayed, so the allowance is not an AWS-enforced account spend cap. Do not
  increase account/concurrency/quotas or alter unrelated budget resources.

Use only the existing controlled endpoint under
`https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com/execution-fixture/`:
`workflow-token`, `workflow-consume`, `workflow-failure`. Each receives a unique
public fixture UUID in `?fixture=...`; existing audit records log only that ID,
mode, authorization boolean and request ID. They supply target-side send-count
proof independently of job rows. Synthetic token material is extracted/bound in
memory, never logged. The controlled response is not a real credential source.

| Run                  | Two admitted steps each; expected evidence                                                                                                                 |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A, before activation | Legacy token→consume workflow; retain marker-absent run/evidence1 for compatibility.                                                                       |
| B, after activation  | Rerun A's frozen run: legacy marker remains absent, evidence1 and old frozen rules preserved.                                                              |
| C                    | Token presence then bound consume `authorized=true`: successful schema2 checks, two sends, exact one terminal result per step.                             |
| D                    | First token response status expected 201 vs actual 200: assertion_failed, failed run, skipped successor, exactly one target send and no successor OUT/job. |
| E                    | Empty assertion arrays: not_configured on both steps; normal two-step completion.                                                                          |
| F                    | First workflow-failure expects matching 500: HTTP policy still fails execution/run; successor skipped, one send.                                           |
| G                    | Token extraction/binding then consume reflected-value equality: sensitive_input/not_evaluated, no raw/rendered token in results/MCP/logs, two sends.       |
| H–L, blocked         | Exact numeric/primitive cases below; ten admitted steps withheld until the separate fixture extension is reviewed.                                         |

Run A, if separately authorized, seeds legacy evidence under the existing active7
stage **before** the deployment's maintenance closure. Settle it fully before
closing. This avoids reopening an unactivated stage just to manufacture legacy
data. If A cannot be authorized or completed, record the missing genuine legacy
seed and do not fabricate it by direct database writes.

For A–G, assert owner B/anonymous denial without dispatching; read schema1/2
evidence and stored-byte checksum through the owner, compare frozen rules,
name-only edits, stale revision rejection and result precedence. MCP complete
and summary reads preserve frozen schemas, omit assertions with the accepted
metadata, and do not rewrite S3 bytes. Pin/unpin one schema2 execution and compare
shared pin/storage counters. Revoke the disposable MCP credential and verify
denial before deleting its project. These calls fit the API limit above.

**Explicit decimal target prerequisite:** Davian selected a separately reviewed
extension to the existing controlled fixture, with **no fixture implementation
now**. Existing echo returns body text, and workflow fixtures do not return JSON
numbers; they cannot establish genuine exact-decimal evaluation. Proposed future
`assertions-values` emits fixed literal UTF-8 JSON, without a float encoder:

```json
{
  "precise": 0.10000000000000001,
  "short": 0.1,
  "exponent": 1e-1,
  "min": 1e-308,
  "max": 9007199254740991,
  "zero": -0.0,
  "exists": null,
  "flag": true,
  "text": "public"
}
```

Proposed `assertions-not-found` emits fixed 404 JSON. Both must use the same
bounded UUID-only audit scheme and no reflected user input. Any implementation
changes the shared worker/controlled-fixture package: it needs a new reviewed
source, clean package hashes and equivalent preview; **do not silently replace
this package's pinned bytes**. H–L cannot be marked passed by local fixtures or
by comparing a decimal expectation with a string field.
Prefer resolving that fixture review before the first activation if the full
campaign is required. Approving only A–G leaves numeric cloud validation open;
a later fixture rollout on active8 needs its own reviewed maintenance procedure.
This initial activator intentionally does not close or downgrade active8.

H: numeric pass with at most eight checks/step (six numbers, presence and status);
author tagged decimal strings, reopen editor, name-only edit, then freeze.
I: whole-run rerun H after a definition edit; source numeric tokens/semantics
must remain exact. J: compare short actual 0.1 against expected
0.10000000000000001; first-step failure and zero successor sends. K: null,
boolean, string and presence checks, followed by matching expected 404 which
still fails HTTP policy. L: missing vs present-null first-step mismatch skips
successor. This adds eight target sends across ten admitted steps.

H/I preserve `0.10000000000000001` vs `0.1`, exponent spellings `1e-1`/`1E-01`,
`-0.0`, `1e-308` and `9007199254740991` through browser→API→saved→frozen→reopen→
unrelated edit→rerun; equivalent exponent values compare equal without rewriting
tokens. Bare JSON numeric expected values, `1e-309`, `9007199254740992`, >128-byte
tokens and exponent magnitude >308 must fail authoring/API admission with zero
admitted steps and zero target sends. No JavaScript Number exactness path.

## Cleanup, verification and remaining holds

Stop submissions; correlate fixture UUID/request IDs against worker evidence and
target audit records, check every run terminal and no pending jobs/OUT/runtime
IN/OUT/bindings/unsettled tickets. Record exact stage and owner admission usage;
**never reset it**. Unpin the test execution, revoke the test MCP token, delete
only the two test projects via normal 202 drain and poll within the same limits.
Confirm immediate owner/MCP denial, completed project deletion, no remaining
Control/Protected project children, and empty S3 project prefixes including late
uploads. Keep normal deleted tombstones and permanent residual registrations.
Read back healthy project/stage pin, inflight and storage accounting (compare
with baseline plus legitimate test usage); confirm generation, capabilities,
Free plan, account10/worker max2, quotas/throttles/schedule unchanged.

Only then revoke their refresh sessions/global-sign-out and delete the two
recorded disposable Cognito users; verify provider-side absence. The API checks
JWTs locally, so deleting a Cognito user does not prove immediate rejection of
an already issued access JWT before its expiry. Project deletion must still
deny all test project access immediately. Record that distinction; do not claim
an untested global revocation guarantee. Existing application user/identity and
issuance/audit/rate metadata may remain under their current retention policy;
inventory those expected leftovers without reading token hashes/values or
claiming full account-data erasure. Do not add a new deletion policy here. Remove
private local test credentials/ledger after retaining sanitized IDs/outcomes.
If cleanup fails or the 20-minute window expires, preserve its IDs and open the
gap on Trello; do not delete arbitrary AWS objects/rows or claim physical removal.

Broad fault injection (crashes, lost ACKs, duplicate queue delivery, IAM failures,
clock skew), elapsed expiry/late-upload windows, PITR/restore and metered cost
analysis remain separately scoped follow-ups. No live fault was injected; the
activator's local transaction/ACK tests do not constitute those cloud tests.
This preparation neither deploys nor closes/opens the stage. Acceptance of the
approach is recorded; execution authorization and the numeric fixture extension
are still separate reviews.

## Completed preparation checks

`npm run verify` passed formatting/gofmt, ESLint, 64 frontend tests, 15
infrastructure tests, TypeScript/Vite build and tests/vet across all seven Go
modules. Additional recommended Node ESLint checks passed on all four affected
activation/review tools; Node syntax/import checks passed. `go mod verify`
passed for API/worker/shared runtime dependencies; `npm ls --depth=0` found the
installed pinned versions. All four retained ZIPs and five frontend app files
were rehashed successfully against the published manifest.

The direct Node test runner command below passed **44 tests**, including the
real DynamoDB Local transaction case (not skipped):

```sh
KURIER_ACTIVATOR_DDB_ENDPOINT=http://127.0.0.1:8000 node scripts/workflow-assertions-activation.test.mjs
```

It proved full-marker CAS and accounting conditions against the repository's
pinned in-memory DynamoDB Local image, plus local ACK-loss, bad readback, drift,
approval, unknown-shape, stale-reader/frontend, residual and bounded-transaction
fixtures. Its table was deleted and the task-created container/network removed;
unrelated containers were preserved. These local tests did not mutate AWS.

The actual AWS [read-only default preflight](workflow-assertions-activation-preflight.json)
passed with active version13/generation unchanged, zero projects, **writes:0**,
backendReadersReady:false and frontendReady:false, correctly distinguishing
unperformed deployment from preparation. Fresh preview/bootstrap/state/IAM and
accounting observations are in the release manifest. No live identity/project
was created or deleted, no stage write/activation/target send occurred, and no
live fault or retention/PITR exercise ran. Keep those gates open on Trello.
