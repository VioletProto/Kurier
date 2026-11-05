# Project environments — final deployment-readiness review

**Not ready to authorize deployment.** Review 8 completed successfully and its
four exact SST ZIPs are now retained in a separate proposed manifest. Archive
structure, bootstrap identity, handler security and the environment transition
core are locally verified. Authenticated `resource.enc` inspection and a reviewed
metered live deployment/activation adapter remain unresolved. The command-line
entrypoint deliberately supports local checks only; it rejects live modes.

This preparation made **zero AWS calls**. Actual usage remains **296/500**, with
204 arithmetic headroom and the same cumulative **$1 procedural allowance**.
Neither headroom nor this packet authorizes live work; billing remains unmeasured.

Release source: `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.
Review tooling: `fe7304cbc9d68fe85f3d1eac04799e14ab6878b4`.
The accepted contracts are recorded in [the architecture index](../architecture/README.md).
[Review 8](project-environments-refresh-preview-review-8.md) remains the completed
preview evidence, not deployment/runtime evidence.

## Exact proposed packages and preservation

The [new SST release manifest](project-environments-sst-release.json) proposes:

| Functions                                          | ZIP SHA-256                                                        | ZIP bytes each |
| -------------------------------------------------- | ------------------------------------------------------------------ | -------------: |
| UsersProjectsApi / EmptyProjectCleanup             | `58f2bbf364042ac209d4f9b3a4d6ca583da18914fb5851f0c166d34b9c757373` |      9,363,094 |
| CloudExecutionWorker / ExecutionControlledEndpoint | `82af7beeda4b619afa6831b342c693bb1fb3e8d08e5e6c0416ce253526f59098` |      8,738,299 |

These are **proposed for adoption, with adoption held**, not a replacement for the
original prepared manifest. Exact copies are in the ignored, private
`.build/project-environments-sst-release/44f1fb79d943501c95e06fcc1a6114111a7c1ea4/<function>/code.zip`,
with read-only file permissions. No ZIP was rebuilt, normalized or modified.
The original bootstrap-only manifest remains byte-identical at
[project-environments-release.json](project-environments-release.json), SHA-256
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
Original prepared packages, retained rollback packages/frontend/function, clients,
all 74 historical frozen proof files, and review-8 receipts/claims verify unchanged.
No original ledger or failed claim was reset. Historical uncertain refresh
outcomes remain uncertain. Frontend metadata matches the prepared frontend.

Every new archive contains exactly `./`, `bootstrap`, and `resource.enc`, with
no duplicate entries, extra files, links, credential/config files or path aliases.
The bootstrap hashes match the clean pinned source builds:

- API/cleanup: `6c85373de3c464b12cd502e21496f799da6a229a72db4ca3d34f7fb1f124b9f4`.
- Worker/controlled endpoint: `903309b98c8639062a24685b639e54f43968b59b0cb8331cf12865e387f7db4f`.

That proves no added executable or credential **file**. It does not authenticate
the encrypted member's contents; the next section preserves that distinction.

## SST bindings and actual runtime loading

The pinned SST 4.17.1 platform source retained by review 8 is byte-identical to
the installed platform. `aws/function.ts` builds a links map from explicit
`args.link`, supplies it and a stage `LambdaEncryptionKey` to the CLI build RPC,
and configures `SST_KEY` / `SST_KEY_FILE=resource.enc` for SDK consumers.
`infra/development-api.ts` declares no `link` for any of these four Go functions;
resource access is through explicit `KURIER_*` environment configuration and
explicit IAM permissions. The intended link map is exactly `{}` for every ZIP.

The pinned SST executable's Go function table/disassembly confirms its
`runtime.(*Collection).Build` binding-encryption branch: base64 key decode,
JSON marshal of the links field, AES construction, GCM construction, a 12-byte
zero nonce and authenticated encryption before writing the build artifact.
The function spans addresses 25289056–25292672; resolved calls include
`encoding/base64.(*Encoding).DecodeString` (5559968), `encoding/json.Marshal`
(6299104), `crypto/aes.NewCipher` (7686112), `crypto/cipher.NewGCM` (7086016),
and nonce allocation (4782144). No SST/provider executable was run for this trace.

The exact SST Go source revision embedded in the binary is
`v3.19.4-0.20260712200334-a0bd20f76288`; the available adjacent cached source is
v3.19.3. It is supporting context, not claimed to be identical pinned source.
Its Go SDK loader reads SST_KEY_FILE and decrypts AES-GCM with SST_KEY, then
merges SST_RESOURCE_* values when that SDK is imported. **These application Go
binaries do not depend on or import the SST Go SDK**, confirmed by source/module
inspection and each retained bootstrap's Go build information. Their entrypoints
use the explicit KURIER configuration path; no shell/Node wrapper or SDK loader
was added. Thus these functions do not load `resource.enc` at runtime.

All four encrypted members are 18 bytes: consistent with a two-byte `{}` plus
GCM tag. This supports the intended empty map, but **size is not authenticated
content validation**. The retained preview's Lambda environment maps are Pulumi
ciphertext. No plaintext SST key or Pulumi passphrase was located in the retained
local materials or current passphrase environment. The historical SSM value was
not retained by the sanitized collector; no AWS retrieval is authorized here.
No binding plaintext, key, raw environment or diagnostic receipt/hash is published.

The new offline checker can accept a mode-0600 private base64 key file, verify GCM
authentication and require plaintext **exactly `{}`**, without printing the key,
plaintext, private filename or exception text. It rejects wrong keys, tampering,
nonempty maps, other JSON, extra members and altered bootstrap/archive bytes.
Until that check succeeds on the real archives, the packet **cannot confirm the
encrypted member contains no credentials or unrelated bindings**. The manifest
records `bindingValidation.authenticated=false`; the transition core blocks
both closure and activation on that condition.

Smallest evidence-supported next observation, proposed only: retrieve the
existing UsersProjectsApi Lambda configuration once through a newly reviewed
read-only meter; extract only its existing SST_KEY into private memory and try
all four retained blobs offline. This avoids fetching unrelated parameters/state.
Require intended non-root STS identity first; any session refresh needs its own
explicit authorization. At most three physical attempts would cover refresh if
needed, STS and that one exact GetFunctionConfiguration. Stop on any uncertainty;
no retry, config repair or alternate key search through AWS. A preexisting local
private key supplied by the operator would instead require zero AWS calls.

## Gateway NONE, ownership and agent security

Gateway NONE matches the accepted architecture's existing application-auth pattern.
Owner environment CRUD routes are registered through `Server.protected`:
exactly one Bearer header, RS256 Cognito access JWT with issuer/client/token-use,
expiry and signature checks, then resolved active user and owned active project.
`environmentContext` checks the environment-capable stage and ownership; writes
carry stage, user and project guards in their transactions. Foreign/missing
resources return fixed unavailable/404 responses, not resource data.

The capability route intentionally uses the separate local-agent handler. It
requires an opaque agent credential, validates active credential/owner/stage,
expiry and recovery generation, then strict bounded capability JSON. A Cognito
JWT cannot substitute for an agent credential. Capability declarations do not
create a job, reserve quota or widen target scope. Admission requires compatible
negotiation before marked work; old agents remain limited to legacy work.

The new integration regression exercises every environment CRUD method with
missing, malformed, duplicated or wrong-token-use Authorization, plus foreign
ownership. It also checks the capability endpoint with missing/malformed/duplicate
headers and an owner JWT. The existing environment/local-agent tests cover
credential separation, frozen identity/context, revocation and incompatible-agent
no-send behavior. They passed against actual DynamoDB Local in an isolated network
namespace, with synthetic identities only. Live Cognito/API Gateway behavior and
browser preflight are still runtime-validation gates. OPTIONS is intentionally
unauthenticated CORS handling; it grants no protected operation.

The proposed invoke statements match the pinned SST route component and existing
inventoried accepted statements: API Gateway principal, lambda:InvokeFunction,
existing account/region/API execution ARN plus `/*`, existing API function target.
This permits invocation by that API across its routes; it is not a substitute for
handler authentication. Review 8 adds eight statements using the same pattern,
not a broader principal or another API/account. **No invoke-scope or gateway-auth
change is needed for this release on the accepted pattern.** Tightening to
method/path-specific invoke ARNs would be a separate SST infrastructure change,
with its own preview/compatibility review. No IAM change was performed here.

## Environment-specific guarded transition core

[scripts/lib/project-environments-activation.mjs](../../scripts/lib/project-environments-activation.mjs)
is separate from the historical schema-7 assertions activator. It implements
inspection, one full-tuple conditional close and one conditional activation via
an injected adapter; it imports no AWS SDK/transport. The CLI
[scripts/activate-project-environments.mjs](../../scripts/activate-project-environments.mjs)
exposes **local archive checking only**. It rejects `--close`, `--activate` and
other live modes before any AWS call. Do not run the old assertions activator.

The exact expected pre-transition tuple has savedRequests8, protectedSecrets1,
localAgentExecutions1, workflowAssertions1, cloudExecutions1, executionInputs1,
evidencePinning1, sequentialWorkflows1, mcpReadOnly1 and absent projectEnvironments.
This is a guard, not an assertion of current live state. Unknown keys, partial
shapes, wrong versions/state, missing generation or existing environment gate stop.
Closure preserves every capability/generation/accounting field, sets
`workflow-migration` and increments version once. Activation from that exact
closed baseline sets savedRequests9, protectedSecrets2, localAgentExecutions2,
projectEnvironments1, state active and version+1, retaining all other capabilities.

Both actions require explicit action/tooling-commit/manifest/marker/accounting
approval and an environment approval token. Activation additionally binds the
**original closure receipt hash** in the approval, requires its same marker and
30-minute deadline, and requires all four deployed code hashes, Active/Successful
states, configuration/IAM/concurrency/controls and frontend proof. The closure
receipt is emitted after verified closure; extending/resetting it invalidates
the existing approval. A future live adapter must provide the original metered
clock/boot/window continuity across processes, not reset clocks between commands.

Metadata inventory rejects unknown kinds/schemas and new environment/request3/
typed-secret shapes before activation. Existing assertion semantics 1 is legal.
It requires settled jobs/runs/steps, terminal published outbox/job/evidence/upload
proof, empty IN/OUT/bindings/local waits and deletion work, valid residual/tombstone/
empty-prefix proof, empty observed queue and exact accounting counters. Queue
emptiness alone is insufficient. It inventories twice, rejects marker/platform/
accounting drift and includes accounting ConditionChecks in the same stage
transaction (at most 80 actions / 2 MiB). Conditional failure, uncertain write,
readback mismatch or postwrite drift stops; no retry, rollback or reopen is built in.

The core and adapter contract are reviewable, but **there is no enabled live
collector/write adapter or metered stage/deployment driver in this packet**.
Implementing/reviewing that connection, current reader-baseline capture and
approval receipt handling is still required. The read-only preview meter must
not be widened or reused as an implicit deployment meter.

## Deployment and verification sequence after blockers are resolved

All steps are proposals requiring later explicit authorization:

1. Finish authenticated binding validation, approve the new manifest and review
   the complete classified review-8 diff: eight route components plus 24 physical
   route/integration/invoke creates, four Lambda updates/four code-object
   replacements and three frontend updates; existing bucket is read only.
2. Review a metered deployment driver with exact operation/resource scopes,
   cumulative accounting, original deadline, before-forward receipts and no
   uncertain-write retries. Refresh account/identity, stage tuple/version/
   generation, accounting/quota/month, IAM/configuration/controls, queue/DLQ,
   maintenance and artifact baseline through authorized reads. Stop on drift;
   older inventory is not current live proof.
3. Resolve all inflight/uncertain work under compatible old code. Produce exact
   marker/accounting approval. CAS closed once, retain the receipt, then inventory
   again for admission/maintenance races. The closure decision deadline is
   30 minutes from the original close window, never automatic reopen authority.
4. With traffic fenced, deploy all four Lambda readers/cleaners, hosted MCP within
   the API, and frontend. SST resources may update concurrently; closure is the
   barrier. Keep old frontend assets (`purge=false`), publish hashed assets and
   matching KV entries before public entrypoints, then await invalidation/deployed
   distribution proof. Do not activate capability rows from infrastructure code.
5. Verify exact Lambda CodeSha256 against the **ZIP** hashes, configurations/IAM,
   API route/integration/invoke policies, all frontend bytes/public marker/KV/
   invalidation and local-agent distribution/negotiation readiness. Prove legacy
   data readable and accounting/month/generation unchanged. Refresh the exact
   closed marker and original-deadline approval; activate once only if every
   reader/cleaner and artifact/binding proof is ready.
6. Strongly read back the exact intended tuple/version and unchanged generation/
   quotas/counters before any newly authorized admission. Run only the bounded,
   separately approved runtime/cleanup campaign below. Stop after its result.

**Using the reviewed bytes:** the retained candidate ZIPs are the adoption
source of truth. SST normally rebuilds and rearchives during deployment. The local
checker compares all regenerated ZIP bytes against the fixed manifest, not just
bootstrap or an object-key suffix. Before a future driver forwards **any upload
or infrastructure mutation**, all generated packages/frontend and the plan must
match their reviewed hashes; use immutable archive mounts/pinned paths and verify
actual upload payloads or staged source bytes to prevent a check/use race. Code
object keys have the reviewed `assets/<function>-code-<ZIP SHA>.zip` form.

The necessary provider/upload-boundary integration is **not implemented** by the
local checker. A pre-run hash followed by unrestricted `sst deploy` would not
meet this guarantee. If packaging regenerates different bytes, stop before any
upload; do not normalize/rewrite a ZIP, silently update hashes, substitute a
bootstrap-only package or continue after partial deployment. Retain the changed
bytes privately and submit a new manifest/diff for review. No executable live
deployment command is supplied until this driver is reviewed.

## Compatibility, migration and rollback limits

No mandatory data backfill, ciphertext re-encryption or evidence rewrite is needed
for the accepted representation. Unmarked legacy requests remain literal (even
braces), old secret schema1 strings remain strings and legacy untyped scalars are
not silently typed. New template requests use row3; new typed saved secrets use
row2; evidence3 freezes only the allowed context additions. Plans/reruns retain
frozen public values/identities and current values at the exact frozen secret IDs.
Deletion does not rebind by name or rewrite old evidence. Legacy agents retain
Work/Result1; marked work requires negotiated Work/Result2. MCP retains its seven
wire tools and projects evidence3 into the accepted legacy DTO with explicit
context omissions, preserving stored bytes/digests/pins. Unknown shapes stop for
separate migration review, not conversion or deletion. OpenAPI production remains
separate; evidence version 3 has only the accepted environment meaning.

Before activation/new-format writes, preserved old packages/frontend may be
restored while closed only after no-new-data proof and separately reviewed
marker transition. S3 code-object replacement can delete old keys; retained
rollback bytes, not old keys or rebuilds, are the proof. Verify all restored
readers/static assets and do not reopen automatically after any failure.

After activation, old code may reject the tuple. Once environment records,
request3, typed-secret2, frozen template/agent2 receipts or evidence3 exist,
lowering markers or restoring old readers/cleaners is unsafe. Keep the stage
fenced under a fresh exact guard; restore compatible readers or fix forward,
preserving new data/evidence/counters/generation. There is no automatic downgrade
or no-data rollback mode in this activator. An uncertain mutation, deployment or
submission is reconciled read-only by original identity/receipt, never by retrying
a new write to discover its outcome. Closure cannot undo an outbound request or
retract public values/downloaded evidence. PITR has separate authorization and
cross-table/S3 consistency, lost-recent-data, resurrected-deletion and replay risks.

## Bounded runtime plan and proposed allowances

The detailed [A–F campaign](project-environments-deployment-review.md#bounded-genuine-runtime-campaign-proposal)
remains the proposed validation: A legacy saved-request/rerun/local/workflow
compatibility; B typed/protected cloud/local resolution; C public-edit/exact-secret
frozen reruns; D workflow inherit/none/select precedence, assertions and bindings;
E assertion-stop/skipped successor and local variable revocation before dispatch;
F explicit null context, owner evidence/pin/download and MCP projection. Add
negative environment CRUD/auth/ownership, stale revision, typed-secret mismatch
and incompatible-agent checks before admission. Use only owned controlled cloud
and loopback fixtures, synthetic canaries and explicit local-origin consent.
No replay/queue/PITR/fault injection is implied.

| Proposed future boundary           | Limit / decision                                                                                                                                                                                                                                                                                    |
| ---------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Current preparation                | **296/500**, 204 headroom; zero new calls in this task                                                                                                                                                                                                                                              |
| Minimal missing-key observation    | At most 3 additional physical attempts / one 10-minute window if separately authorized; no targets/writes except explicitly approved session refresh                                                                                                                                                |
| Closed stage                       | One close/activate sequence; 30-minute decision deadline; no automatic reopen                                                                                                                                                                                                                       |
| Deployment + runtime campaign time | Proposed 120 minutes total, stop new admissions at minute 75; final 30 minutes reserved for cleanup                                                                                                                                                                                                 |
| Identities/projects                | At most two disposable identities and two projects cumulative; no replacements without review                                                                                                                                                                                                       |
| Admissions/target sends            | 20 planned admitted steps / 18 planned sends; maximum 24 / 24; at most 40 submission/rerun POST attempts                                                                                                                                                                                            |
| API / polling                      | At most 600 physical application attempts including OPTIONS, local polling/uploads/MCP; 240 status polls included, at least two seconds apart                                                                                                                                                       |
| Cognito/static                     | At most 80 Cognito calls / 150 static requests                                                                                                                                                                                                                                                      |
| Future AWS execution budget        | Original proposal: up to 1,500 operator/provider/control/data calls across deployment/runtime/cleanup. **Not authorized and not within the existing 204 headroom**; a separately reviewed execution ceiling and complete meter are prerequisites. Carry all 296 prior calls rather than reset them. |
| Cost                               | Same cumulative **$1**, no new dollar allowance; preserve account quotas/concurrency and existing $5 light-development target; actual spend/rates/shared charges remain unmeasured                                                                                                                  |

A full deployment/readback/activation campaign has not been demonstrated to fit
204 remaining preparation calls. Do not start one under that assumption or treat
the proposed execution budget as an increased current ceiling. Retries/pages,
failed transports, credential operations and writes must consume any future
approved physical-attempt budget. Application-service background SDK traffic is
additional cost exposure, not falsely claimed as fully metered operator traffic.
Unknown prior/shared spend is uncertainty, not a measured remaining dollar balance.

Cleanup is part of the bounded campaign: delete a disposable environment first,
prove shared secret retention/deleted-context rerun denial, then explicit secret
revocation and normal project deletion. Prove tombstones/operations, immediate
owner/MCP denial, Control/Protected child drainage, no buffers/jobs/leases/uploads,
scoped evidence S3 absence, settled residuals and pin/storage/inflight decrements.
Revoke disposable agent/MCP grants, sign out/delete disposable identities and
independently verify absence and final generation/accounting. Preserve unrelated
rows, quota history and monthly test charges. Unknown outcomes/cleanup gaps stop
with retained evidence; no unmetered continuation or replacements are implied.

## Local verification and exact review commands

- **88 transition tests passed**, including real DynamoDB Local atomic full-marker
  CAS/accounting conditions, unchanged generation/counters, capability/schema
  rejection, quiescence, reader/frontend/IAM drift, conditional failure, uncertain
  readback, approval mismatch, bound closure receipt/deadline and missing binding proof.
- **10 archive tests passed**: authenticated empty synthetic map, wrong key/tag,
  unverified ciphertext status, other bindings/JSON, extra/duplicate/path/link
  files, archive/bootstrap changes, private diagnostics and regenerated mismatch.
- Ownership and execution environment integration suites passed with `-race`;
  new gateway-NONE negatives and existing agent/owner checks used real DynamoDB
  Local, synthetic JWTs/KMS/S3/queue/targets and no external network.
- `npm run verify` passed offline: formatting, lint, application/tooling tests,
  builds, Go test/vet. Ruff checks passed. Original artifacts/proofs and source
  cleanliness were reverified after testing. No AWS credential/configuration was
  used by the isolated integration runner; no image pull or settings repair occurred.

From the clean review tooling checkout, these commands are local only:

```sh
node scripts/activate-project-environments.mjs --check-local
npm run test:environment-release
python3 -B scripts/test-environment-readiness-offline.py \
  --dynamodb-directory /tmp/kurier-environment-readiness-dynamodb
.build/attempt-meter-venv/bin/python -B scripts/environment-release-check.py \
  --regenerated-directory .build/project-environments-preview/review-8/provider/work/sst/artifacts
```

The last command compared retained review-8 archives with their exact proposed
copies; it did not regenerate or run SST. The isolated integration command requires
the cached local JAR; it never downloads it. The normal unit command skips only
the optional real-DynamoDB transaction case unless that fixture is supplied.
The actual integration run included it: **88 passed, zero skipped**.

## Decisions and unresolved gates

1. Supply a preexisting private SST key locally, or separately authorize the
   minimal metered key observation; authenticate all four blobs and confirm empty
   bindings/no credentials. Until then, keep candidate adoption and closure held.
2. Review/adopt the exact new ZIP manifest only after that proof. The original
   prepared manifest and rollback bytes remain authoritative historical evidence.
3. Implement/review the live metered collector/stage adapter and provider upload
   gate, including fresh expected reader baseline, approval receipts, clock/boot
   continuity, artifact check/use protection and actual deployment operation scope.
4. Approve a viable future call/time/cost budget explicitly; inspect fresh stage,
   accounting, IAM/config and current spend through authorized reads. Neither live
   state nor remaining dollars is established by these local checks.
5. Separately authorize closure/deployment/activation and, later, bounded runtime
   identities/projects/targets/cleanup. No live command is enabled by this packet.

All deployment, closure, migration, activation, identity/project creation and
live target requests remain held. This packet completes the local review and
makes the remaining blockers concrete; it does not label a blocked release ready.
