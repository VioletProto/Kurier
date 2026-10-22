# Workflow assertions deployment review

Prepared 2026-10-20 for application commit
`55cd061c0f6607037ebd61e200fe54ab296237c4`, implementing accepted contract
`31de3929aa1423c25b264d9e459aec48728e1c90`.
[Implementation and local evidence](workflow-assertions.md),
[contract](../architecture/workflow-assertions-contract.md), and
[Trello gate card](https://trello.com/c/FkUYPLYV) remain separate records.

**For review only.** Davian authorized read-only AWS inventory and conditional
SST preview for this packet. Both completed. No deployment, activation, target
request, Cognito sign-in, resource mutation or runtime fault injection occurred.
The original implementation's preview hold is historical; deployment,
capability activation and genuine runtime validation remain held. Publishing
this packet does not lift those holds. No application code changed in this task.

## Preview safety and observed baseline

Before running SST, inspect its pinned 4.17.1 source, not just the command name:

- [Run path](https://github.com/anomalyco/sst/blob/v4.17.1/pkg/project/run.go):
  diff skips remote locking, state/history pushes and pruning; invokes Pulumi
  preview with a local file backend. It still calls GetOrCreatePassphrase.
- [AWS bootstrap](https://github.com/anomalyco/sst/blob/v4.17.1/pkg/project/provider/aws.go)
  and [passphrase creation](https://github.com/anomalyco/sst/blob/v4.17.1/pkg/project/provider/provider.go):
  absent/outdated prerequisites can cause writes before preview. Require existing
  bootstrap v5, expected Ohio buckets/ECR and exact stage state, plus existing
  passphrase metadata. Stop on mismatch; never bootstrap or repair in this task.
- Local `infra/frontend-artifact.ts` runs the hosted Vite build and writes local
  archives. SST builds ARM64 Go ZIPs locally. These hooks contain no target
  calls. S3 uploads are resource declarations; pinned RPC providers put/delete
  files and KV entries or invalidate distributions only in Create/Update/Delete,
  not preview Read/Diff. Their source is under
  [pkg/server/resource](https://github.com/anomalyco/sst/tree/v4.17.1/pkg/server/resource).
  No dev/live mode, deployment, refresh, unlock, state repair or remove is allowed.

`node scripts/preview-workflow-assertions.mjs --diff` checks application/infra
source equality with the specified commit, rejects a frontend rollback override,
and reuses the reviewed inventory helper. It verifies non-root account
`747336059622`, region `us-east-2`, stage `dev-api`, SST/platform 4.17.1,
bootstrap, Free plan, packages, environment hashes, IAM, stage and accounting.
Before/after metadata hashes cover the existing stage passphrase and all 88
current state-bucket objects, including locks/history. Bootstrap, consumer code,
environment/IAM, stage and accounting checks also passed unchanged. This is
observational evidence, not an IAM-enforced read-only credential boundary.
Rerun only while these audited prerequisites and source remain valid.
SST also rewrote local generated type-file formatting; those generated changes
were inspected and restored, with no application change included in the packet.

Raw preview/state/logs stay ignored under `.sst`; they may contain sensitive
configuration and must not be pasted or committed. Only selected metadata is
exported by `node scripts/review-workflow-assertions-preview.mjs` to the
[sanitized preview record](workflow-assertions-preview.json). The exporter
rejects unexpected resources, operations and changed property roots.

Observed during the preview (2026-10-20 12:30 MDT):

| Invariant           | Observed value; preserve at rollout                                                    |
| ------------------- | -------------------------------------------------------------------------------------- |
| Stage               | active, version 13, savedRequests7; assertions marker absent                           |
| Other capabilities  | workflows1, MCP1, cloud1, inputs1, protected1, pinning1, local1                        |
| Recovery generation | `dev-api-initial-v1`                                                                   |
| Stage quota         | October 2026 count 2, version 5, pinnedCount 0, pinnedBytes 0; unchanged hash in JSON  |
| Projects            | 0 accounting rows; empty-set hash recorded, no fabricated project counters             |
| Account             | FREE / ACTIVE; expiration 2026-12-29; concurrency 10 / unreserved 10                   |
| Worker              | enabled existing queue mapping, batch 1, maximum concurrency 2, standard pollers       |
| Functions           | no reserved or provisioned concurrency on all four consumers                           |
| Maintenance         | existing enabled `rate(1 minute)` schedule                                             |
| API/Cognito         | rate 10 / burst 20; handler CORS; Essentials/default sender/rotating refresh preserved |
| Bootstrap           | v5, SSM parameter version 1; existing state/asset buckets and ECR unchanged            |

Quota inspection was metadata-only, with no reset. Runtime limits remain
1,000 admitted steps/stage/month and 100/owner/day; workflows charge every
planned step. Preserve project inflight slots, 1 GiB project / 2 GiB stage
storage admission and existing pin limits. Account Free plan and application
quotas are distinct controls. The inventory is a point-in-time observation;
read everything again before any separately authorized mutation.

## Exact proposed deployment delta

The plan has **11 distinct changed resources**: four existing code-object
replacements, four in-place Lambda code updates and three existing frontend
dynamic-resource updates. The JSON includes each logical name, type, operation,
changed properties, old/new package keys, SHA-256 and bytes. Repeated
create-replacement/replace/delete-replaced events describe the same code object.
There are no new application resources, table/index changes, replacements of
functions, IAM/trust/attached-policy changes, routes, environment changes,
memory/timeout changes, concurrency changes or stage marker updates.

| Logical Lambda                      | Existing physical function                                    | Memory / timeout | Package    |
| ----------------------------------- | ------------------------------------------------------------- | ---------------- | ---------- |
| UsersProjectsApiFunction            | `kurier-dev-api-UsersProjectsApiFunction-werhekow`            | 256 MiB / 15 s   | API ZIP    |
| EmptyProjectCleanupFunction         | `kurier-dev-api-EmptyProjectCleanupFunction-bcmuzwku`         | 256 MiB / 60 s   | API ZIP    |
| CloudExecutionWorkerFunction        | `kurier-dev-api-CloudExecutionWorkerFunction-wrzufkkr`        | 256 MiB / 90 s   | Worker ZIP |
| ExecutionControlledEndpointFunction | `kurier-dev-api-ExecutionControlledEndpointFunction-mvdmchok` | 128 MiB / 65 s   | Worker ZIP |

Preview API ZIP: 9,251,620 bytes,
`6f44c8013fe0215202006b7d9edca7b824fee84a28ae3ac7c182c42ced28388a`.
Preview worker ZIP: 8,685,938 bytes,
`4ca9f47d80965218f914b8830a4afdbf5990664cd6e9da81be5bd476e98d655d`.
Each ZIP contains Linux/ARM64 `bootstrap` and SST `resource.enc`. API/cleanup
share one compiled program; worker/controlled fixture share another. The fixture
update is a shared-package consequence, not authorization to invoke it.
Current live package identities and all four existing role policy/trust hashes
are recorded in JSON. No new dependency or lockfile change is proposed.

These are **preview artifacts**, built with Go 1.27.1, CGO disabled, ARM64 v8,
`-s -w`, VCS revision `55cd061…` and `vcs.modified=true` because review tooling
was present. Do not label them clean release artifacts. A clean, pinned release
checkout/build and fresh diff must record final ZIP hashes and compare only the
reviewed package/static changes before deployment. Documentation commits can
change embedded VCS revision and therefore ZIP identities without changing Go
source. Preserve verified previous binaries before code-object replacement:
Pulumi plans deletion of old code objects after replacement, so their old S3
keys are not a durable rollback archive.

Frontend changes are exactly:

- `FrontendWebAssetFiles.sst.aws.BucketFiles`: publish release
  `bd511e3f9979d2ef7dd6b73d183dcf7620a423b62659ac89c64e4a1a08981cde` into
  existing private bucket `kurier-dev-api-web-fc2927a49a0b246240e5a17774`.
  Current roots, private `_releases/<id>/` archive and manifest are the ten
  declared objects. The five app-file hashes/sizes and public config are in JSON.
- `FrontendWebKvKeys.sst.aws.KvKeys`: add `/assets/index-CD_8s4hz.js` in existing
  namespace `76ef`. CSS stays `index-CAjex_6u.css`; old JS is
  `index-TwEBpFzH.js`. The desired map drops the old JS entry, but the pinned
  provider's Update only purges absent entries when `purge=true`. Both KV and
  bucket upload retain `purge=false`, so historical keys/objects remain.
- `FrontendWebInvalidation.sst.aws.DistributionInvalidation`: new content
  version, existing `/*` path, wait true, existing distribution `EMNTWD8Q14ZLM`.

No CloudFront distribution/function/policy, bucket policy, OAC, origin, Cognito,
TLS or public access change is planned. Keep archives viewer-inaccessible and
retain prior frontend releases under the existing retention procedure. See
[hosting rollback/retention](frontend-hosting.md). No retention cleanup ran here.

The initial `DevelopmentStage` TableItem retains `ignoreChanges:["item"]`;
deployment must not reset its initial values over the durable row. Existing
`KURIER_SAVED_REQUESTS_SCHEMA_VERSION=7` environment stays unchanged: admission
uses the durable stage row and its transaction conditions. Activation is a
separate conditional data operation, absent from this preview.

## Refreshed incremental costs

[Captured Ohio prices](workflow-assertions-prices.json) were fetched 2026-10-20
from public AWS catalogs: Lambda published October 16, DynamoDB September 11,
S3 October 15. Publication dates, SKUs, full-catalog hashes and tier boundaries
are retained. USD estimates below exclude credits, tax and allowances; they are
not measured spend or a hard cap. No new fixed-price resource is proposed.

| Driver                     | Current Ohio rate                          | Example increment                                                                                                                                                                           |
| -------------------------- | ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ARM64 Lambda duration      | $0.0000133334 / GB-second                  | 1,000 evaluated steps × 50 ms extra × 0.25 GiB = $0.000167                                                                                                                                  |
| Lambda invocations         | $0.20 / million                            | no extra invocation per assertion; existing maintenance/polling/retries still bill                                                                                                          |
| DDB standard on-demand     | $0.625 / million WRU; $0.125 / million RRU | illustrative 1,000 steps × 20 additional 4-KiB transactional item writes = 160,000 WRU = $0.10; 10 extra rounded strong RRUs/step = $0.00125                                                |
| DDB storage                | $0.25 / GB-month above allowance           | illustrative 16 KiB extra retained metadata per step × 1,000 for full month = about $0.0041                                                                                                 |
| S3 Standard storage        | $0.023 / GB-month                          | 2 KiB result growth × 1,000 full-month retained evidence = about $0.000047                                                                                                                  |
| S3 PUT/LIST and GET        | $0.005 / 1,000; $0.0004 / 1,000            | results share the existing evidence PUT; unchanged count, larger immutable objects                                                                                                          |
| One package/static release | same S3 rates                              | four ZIPs total 35,875,116 bytes plus under 1 MiB static/archive objects: under $0.001/month storage; ten static plus four code PUTs under $0.0001, excluding tooling/state retries/history |

The write/storage counts and 50 ms are sensitivity assumptions, not observed
profiling or upper bounds. Apply actual rounded item sizes, transactional
doubling, conditional conflicts, frozen definitions, buffers and retention;
never multiply a literal's size as though every combination fits the 64 KiB
definition cap. DDB backup growth, polling bytes and network egress remain
usage-dependent. Measure evaluation duration and memory within the existing
timeouts; do not increase timeouts to accommodate unreviewed work.

This illustrative 1,000-step increment is about **$0.11/month plus release,
backup and read/transfer overhead**, subject to actual writes/duration/retention.
It does not refresh the entire historical $3.19 architecture forecast into an
actual account bill. The historical 10,000-execution scenario exceeds today's
1,000-step stage quota and is not an authorized budget increase. Existing KMS,
Cognito, HTTP API, SQS, maintenance, CloudFront/function/KV and retained
storage costs continue; assertions add no standing worker, queue or log stream.
Keep the existing $5 light-development target/alerts and review actual metering
after authorized runtime tests. Do not increase quotas or concurrency to meet it.

A `/*` invalidation counts as one path; the first 1,000 account-wide paths/month
are free, then $0.005/path ([AWS invalidation billing](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/PayingForInvalidation.html),
[rate](https://aws.amazon.com/about-aws/whats-new/2015/05/amazon-cloudfront-makes-it-easier-to-invalidate-multiple-objects/)).
Existing CloudFront request/transfer/function/KV charges apply to the new
408,338-byte JS file. This task sent no site or target traffic and performed no
invalidation. Read-only inventory/preview can incur ordinary AWS read fees.

## Deployment order for compatible readers

The following is a proposed runbook, **unperformed and requiring separate
authorization**. Stage closure and availability interruption require acceptance.

1. Recheck account/non-root identity, region/stage, bootstrap, exact source,
   clean release hashes, live code/env/IAM, Free plan, quotas, concurrency and
   stage/accounting snapshots. Reject any broader diff. Retain previous package
   bytes with hashes and the verified frontend archive before replacement.
2. Quiesce submissions and inspect identifier/state-only metadata across Control
   and Protected, including jobs, runs/steps, OUT, upload tickets, pending
   publication, runtime IN/OUT buffers and leases. Do not expose encrypted values
   or request contents. Allow known inflight sends to settle under compatible
   baseline code; then CAS the **fresh** active savedRequests7 marker into the
   existing `workflow-migration` maintenance state, incrementing version only.
   Preserve generation and every capability/counter. Reinventory after closure
   for races. A running outbound send cannot be cancelled by closing the stage.
   Any unsettled/ambiguous work requires reviewed fencing/reconciliation before
   continuing; never replay HTTP to resolve uncertainty.
3. While closed, deploy compatible API/hosted MCP readers, ownership and execution
   cleanup, worker/runtime/reconciler and frontend. A single SST apply may update
   these concurrently; closure prevents new writers until **all** are verified.
   Do not rely on Pulumi scheduling to provide an API-before-worker barrier.
   Cleanup and API share the API package; controlled fixture shares the worker
   package. Distribute the compatible local MCP client if needed and test its
   frozen legacy tool schema. Local-agent/result protocols remain schema 1.
4. Verify deployed code hashes/configuration and all reader/cleaner identities,
   current static manifest/KV/retained assets and unchanged IAM/invariants before
   opening any new writer. Compatible code must read old saved/frozen rows,
   evidence1 and marker-absent reruns. Preactivation writes cannot contain
   nonempty assertions. An old browser can replace a whole steps array and clear
   checks: require users to reload the compatible editor before activation.
5. Record the compatible-reader gate. Deployment does not authorize activation.
   If activation authorization is absent, keep the new capability off; reopening
   savedRequests7 requires its own reviewed CAS and accounting readback. Never
   automatically reopen on an error.

## Guarded capability activation proposal

No activation script or AWS write was added/run for this packet. Existing older
activation scripts target other schemas and must not be repurposed unreviewed.
A future reviewed activator must implement these checks and explicit readbacks:

- Require separately approved deployment verification and activation, stage
  maintenance closure, no unsettled jobs/runs/OUT/publications/runtime buffers,
  intact generation and healthy pin/inflight/storage/admission counters. Empty
  rows today are not permission to initialize/reset them at activation time.
- Strongly read the complete marker; require savedRequests7, assertions marker
  absent, workflows1/MCP1/cloud1/inputs1/protected1/pinning1/local1 and the exact
  expected state/version/generation. Guard every transition with that complete
  capability combination and original version. Stop on conditional failure;
  do not retry using newly observed values automatically.
- After closed-state inventories agree, one conditional update changes only
  savedRequests7→8, absent assertions marker→`workflowAssertionsSchemaVersion:1`,
  state→active and version→version+1. Do not modify recoveryGeneration, quotas,
  month/count, pin/storage reservations or project counters. A close+activate
  sequence would advance today's version 13 twice, but **never hardcode 13/15**.
- Strongly read back and compare the entire marker against exactly those
  intended changes; compare accounting separately. Confirm Free plan, API
  throttles, worker batch/max2 and account concurrency10 remain unchanged.
  Re-read before any explicitly authorized runtime submission. Unknown shape,
  mismatch or missing proof leaves the stage closed for review.

## Rollback limits and containment

Before activation/new data, a closed-stage code/frontend rollback to preserved,
verified bytes may be possible after demonstrating no new stored shapes.
Do not assume old S3 code keys survive the planned replacement. Rebuilds are
not byte-identical rollback proof. Frontend rollback requires verified private
archive contents, matching public configuration, retained hashed assets/KV keys,
reviewed diff and waited invalidation; reloading tabs is still necessary.

After savedRequests8, old binaries reject the stage. After assertion definitions,
marker-1 runs or schema-2 evidence exist, lowering the marker is not a rollback:
old readers/cleanup can reject or lose meaning. Close the stage with a fresh
version/generation guard and restore compatible readers/cleaners or deploy a
compatible fix forward. Keep new stored data and immutable evidence intact.
Do not strip rules/results, rewrite evidence1, reset generation, replace tables,
restore counters from the preview, automatically rerun jobs or reuse old uploads.
Closure does not undo already sent HTTP, retract data deliberately authored as
public expected values, or erase cached/downloaded evidence. Missing buffers
must yield evaluation unavailable/unknown outcome, not a second target send.
Backup restore remains the accepted ADR 0002 exception: later deletions can be
resurrected and recent data lost; any restore needs separate authorization,
reconciliation and recovery fencing, not this rollout's rollback shortcut.

## Genuine runtime validation and cleanup gates

All items below are **planned, not performed**. Local Go/DynamoDB Local race,
Chromium and mocked installed-Codex tests at the implementation commit do not
prove AWS IAM/Cognito, Lambda memory/time, live send counts or elapsed retention.
Future approval must name a disposable owner/project, controlled target,
maximum submissions/cost and cleanup identifiers. No arbitrary user endpoint or
implicit duplicate target sends. Prove counts at the target, not only job rows.

| Gate                          | Required real evidence after separate authorization                                                                                                                                                                                                                                                                                                                                                                                                                      |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Authentication/ownership      | Genuine Cognito owner sign-in; anonymous/other-owner denial on definition, run and evidence; revoked MCP credential denial; no assertion fields in legacy MCP tools/status/SQS. Keep tokens in memory.                                                                                                                                                                                                                                                                   |
| Numeric round trips           | Browser authoring→API decode/encode→saved rows→frozen run→editor reopening→name-only edit→whole-run rerun preserves exact tokens `0.10000000000000001`, `0.1`, `1e-1`, `1E-01`, `-0.0`, `1e-308`, `9007199254740991`. First pair differs, exponent equivalents compare equal. Boundary negatives: `1e-309`, `9007199254740992`, >128-byte token, absolute exponent >308 and bare numeric expected DTO fail before target dispatch. Do not inspect via JavaScript Number. |
| Evaluation/failure            | Correct/mismatched status, exists, string/bool/null/numeric checks; present-null vs missing/type mismatch; zero checks, eight checks, malformed JSON and oversized/omitted body. Matching expected 404 still fails HTTP policy; stronger capture/network/extraction/storage errors retain precedence.                                                                                                                                                                    |
| Atomic containment/send count | A failed producer creates no successor OUT/job or runtime output and skips successors. Compare target send count, terminal execution/step/run and one accounting release. Separately approved duplicate SQS/OUT, lost final ACK, post-PUT adoption/conflict, timeout and missing-buffer faults must never produce a second HTTP send. Fault injection itself requires review; do not kill arbitrary live work.                                                           |
| Security/immutable evidence   | Synthetic noncredential canaries cover sensitive names, configured redaction, extraction overlaps and reflected runtime taint. Unsafe equality is not evaluated. Inspect sanitized evidence/result shape without publishing raw response/secrets; no actual/expected/path/hash in results/logs/status/SQS/MCP. Check S3 checksum over stored bytes, owner schema2 read, legacy schema1 read and MCP complete/summary omission metadata without rewriting the object.     |
| Compatibility/retention       | Legacy run rerun preserves missing semantics marker and evidence1; new rerun preserves frozen rules/marker despite definition edits. Test local-agent/standalone schema1, pin/unpin, logical expiry, context reads and shared evidence accounting through genuine deployed readers/cleanup. Keep unknown shapes fail closed.                                                                                                                                             |
| Deletion/cleanup              | Record every disposable resource ID without credentials. Delete the test project via its normal 202 drain; verify immediate owner/MCP denial, then terminal deletion, no jobs/OUT/runtime IN/OUT, settled uploads/orphans and expected counter decrements. Verify evidence S3 objects physically absent after cleanup and no late PUT resurrection. Never delete unrelated rows/buckets or reset the monthly quota to erase test usage.                                  |
| Operational evidence          | Observe actual billed duration/max memory, timeout/error/conditional conflicts, maintenance passes, IAM denies, send counts and metering. Wait through actual expiry/late-upload retention windows and record timestamps; clock manipulation/local fixtures are not elapsed cloud proof. Remove only expressly approved test identities/fixtures; no new cleanup resources.                                                                                              |

Read back post-test stage/generation, monthly admission count (test usage remains),
pin/storage/inflight counters, quotas/concurrency, pending work and S3 cleanup.
Inventory has zero projects now; future zero projects does not imply a zero
monthly count. Any cleanup gap stays open on the card. Preserve separate
[workflow operations](https://trello.com/c/1QofXXKi) and
[incident/recovery](https://trello.com/c/8XqbFCJ0) follow-ups; this packet does not
close PITR, IAM/fault, elapsed-retention, browser or metered-cost obligations.

## Decisions requiring acceptance

Packet verification passed `npm run verify`: Prettier/gofmt checks, ESLint,
64 browser-component/unit tests, 15 infrastructure fixture tests, TypeScript/Vite
build, and Go tests/vet across all seven modules. Both new review scripts passed
Node syntax checks; the exporter ran successfully against the actual preview,
checked all 11 allowed resource changes and matched four local ZIP hashes.
`git diff --check` passed. No AWS integration test or target request was run.
The gated AWS preview and inventory above are real AWS observations; the local
checks do not upgrade them into deployed assertion-runtime evidence.

1. Accept this exact packet/source and the code-only four-function plus static
   delta; require clean final artifacts/fresh diff before deployment. Accept the
   closed-stage availability interruption and compatible-reader gate.
2. Separately authorize a scoped dev-api deployment, including package storage,
   existing KV updates and waited `/*` invalidation; no new IAM/resources or
   generation/accounting/quotas/concurrency changes.
3. Separately authorize the reviewed closed-stage guarded savedRequests8 /
   assertions1 activation procedure and its post-data rollback limitations.
4. Separately authorize a bounded real Cognito/controlled-target runtime test and
   cleanup plan, including any fault injection and retained test admission usage.

None of decisions 1–4 is inferred from packet publication. Trello remains open
for review; only inventory/preview/cost preparation can be marked complete.
