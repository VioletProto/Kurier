# Project environments deployment review

## Final readiness review — local prerequisites verified, deployment still blocked

The [readiness packet](project-environments-final-readiness.md) proposes the exact
review-8 SST ZIPs in a [new manifest](project-environments-sst-release.json), while
preserving the original manifest and rollback bytes. Structure/bootstrap hashes,
application JWT/ownership/agent guards and the accepted API-wide invoke pattern
are locally verified. **88 transition tests with real DynamoDB Local and 10 archive
tests passed**, plus security integration/race and repository verification.
Real resource.enc content remains unauthenticated because no local key was found;
the packet does not certify encrypted credential/binding absence. Candidate
adoption and closure remain held. The guarded transition core is implemented;
the CLI is local-only. A metered live adapter and upload-boundary byte gate still
need review/implementation. **296/500 cumulative; zero new AWS calls**, same $1
procedural allowance; no deployment/runtime work or automatic continuation.

## Authorized review-8 completed — cumulative 296, ZIP differences for review

The [review-8 result](project-environments-refresh-preview-review-8.md) records
one successful clean `1fd9812` execution: refresh + mandatory STS + 31 provider
reads, including the exact bucket ListTagsForResource (HTTP 200), with no denial.
**296/500 cumulative**; no restart or continuation. The completed SST change list
covers eight routes/integrations/invoke permissions, four Lambda code changes,
frontend changes and an existing bucket read. Four new ZIP hashes differ because
SST adds `resource.enc` and a directory entry; all bootstrap bytes and frontend
metadata match prepared bytes. New ZIPs/bindings require review and are not adopted
as release artifacts. All 74 original proof files, claims and release/rollback
bytes remain intact. All deployment/runtime holds remain; no further live work
is authorized. Historical packets below retain their original status/accounting.

## Resource-scoped ListTagsForResource amendment — local review only, cumulative 263

The [new packet](project-environments-listtags-resource-review.md) establishes
one ARN from the diagnosed FrontendWebAssets bucket, retained inventory/state
and the pinned provider's bucket-ARN construction. The dedicated gate permits
only its canonical empty-body GET on the exact account/region endpoint, with
strict query/header/signing constraints. Other resources, mutations and generic
S3 Control allowances remain held. **90 unit tests and 26 offline transport/
process scenarios passed**, as did repository verification. No AWS calls ran;
**263/500**, 74 frozen proof files and all release/rollback bytes are preserved.
Proposed exact tooling `1fd9812f8b974d527ba77e71ac78f7db63db276e` and a fresh
60-minute/237-attempt command are published for review, not execution. All live
work and deployment/runtime holds remain; historical results below are unchanged.

## Authorized review-7 stopped — GET ListTagsForResource, cumulative 263

The [review-7 result](project-environments-refresh-preview-review-7.md) records
one clean `987b894` execution. Refresh and mandatory STS verification returned
HTTP 200, followed by 30 provider reads. **263/500 cumulative**. Private review
identified a unique GET ListTagsForResource match; it was blocked before any
S3 Control AWS request. Raw receipt/hash and resource values remain private.
Resource ARN scope is unestablished by the name-only receipt; any read allowance
needs separate review. An error-time preview summary reports 8 create/97 same,
with three error diagnostics, not a completed diff. Frontend matches; no new ZIP
completed; original evidence and release/rollback bytes remain intact. No policy
change/restart/repair or deployment/runtime continuation occurred.

## S3 Control exact-host amendment — all HTTP operations held, cumulative 231

The [implemented amendment](project-environments-s3control-operation-review.md)
recognizes only the accepted account/region hostname and blocks every S3 Control
HTTP request before admission/forwarding. Local source inspection still cannot
establish the historical API. Default-off private diagnostics emit only bounded,
model-derived operation names/status; no paths, query keys/values, headers,
credentials or bodies are retained. **80 unit tests and 22 offline process/
transport scenarios** passed, including zero upstream requests and descendant
shutdown. Accounting verifies **231/500** and 56 protected original files; release/
rollback bytes and all prior claims remain intact. Exact proposed execution at
`987b89422c4da2d06a75ce9bd49988de0790cd86` is for review only. No live execution,
S3 Control API operation, deployment or runtime continuation is authorized.

## Authorized review-6 stopped — reviewed S3 Control host, cumulative 231

The [review-6 result](project-environments-refresh-preview-review-6.md) records
one clean `3109170` execution with private CONNECT capture. Cached credentials
were valid: no refresh; required STS verification succeeded, followed by 30
provider attempts. Actual usage is **231/500**. Private review approved disclosure
of the same-account regional S3 Control hostname; raw receipt/hash remain private
and excluded from public evidence. CONNECT establishes no HTTP operation.
The [separate proposal](project-environments-s3control-connect-proposal.md) is
exact-host recognition with all S3 Control HTTP requests still held, not an
applied read allowance. No policy change/restart/repair occurred. Diff remains
partial; frontend matches, no new ZIP completed, release/rollback bytes and all
original proofs/claims remain intact. All deployment/runtime holds remain.

## Local CONNECT investigation — private capture proposed, cumulative 200

The [review-5 investigation](project-environments-denied-connect-review-5.md)
cannot attribute the exact denied destination from retained private logs,
generated configuration or installed source. No hostname is guessed or allowed.
A default-off private host-only receipt is implemented for review: 253-character
host/512-byte file cap, exclusive 0600 file in the ignored private ledger
directory, no paths/queries/headers/bodies, unchanged public redaction and
stop-before-forward. Any captured hostname requires private review before
publication. Accounting verifies the preserved terminal campaign at **200/500**;
no new claim/window or live call occurred. All deployment/runtime holds remain.

## Authorized review-5 stopped — authentication succeeded, cumulative 200

The [review-5 result](project-environments-refresh-preview-review-5.md) records
one execution of clean `4a0faaf` against unchanged release source. Existing-session
refresh and same-identity STS verification returned HTTP 200; 30 read-only provider
attempts followed. The meter then held an unattributed CONNECT endpoint on port
443; hostname is redacted by the reviewed sanitizer. **200/500 cumulative**:
168 prior + 2 authentication/verification + 30 provider attempts. No restart,
repair or widening followed. Partial component evidence has no final summary;
frontend bytes match, no new ZIP completed, release/rollback bytes and all prior
proofs remain unchanged. New campaign/provider claims and original ledgers are
preserved. All deployment/runtime holds remain; future work needs reviewed
200-based accounting and explicit authorization.

## Authentication endpoint alignment — offline verified, live work held

The [amendment](project-environments-auth-endpoint-alignment.md) lets the pinned
SDK resolve CreateOAuth2Token and accepts only the prepared token URL at
https://us-east-2.signin.aws.amazon.com/v1/token before admission/forwarding.
Offline tests confirm DPoP URI alignment and rejection of the old/wrong hosts.
The current authentication/preview baseline verifies **168/500**; historical
ledgers/claims, release/rollback bytes and the byte-identical preview policy
remain preserved. **63 unit tests and 15 process/transport scenarios**, local
preflight and repository verification passed without external networking.
Implementation `4a0faafc673e97d9faae3f26906a327a8bd06e71` and an exact proposed
execution command are published for review. No refresh, probe or preview ran;
recovery and historical causation remain unverified. All live-work holds remain.

## Offline operation-endpoint comparison — no live work

The [comparison](project-environments-auth-path-comparison.md) confirms that
installed default SDK/CLI CreateOAuth2Token selects
us-east-2.signin.aws.amazon.com, while the unchanged helper forces
signin.us-east-2.amazonaws.com. Identical client base hostnames had masked this
operation-level divergence. Region/current proxy/TLS settings match; timeout and
retry configuration differ. This is not proof of historical causation. No
endpoints, cache, policy or networking changed; no network requests/probes/refresh ran.
Usage 168/500 and every historical proof/claim and release/rollback byte remain
preserved. The smallest next action is review of an authentication-only
operation-endpoint alignment proposal; no live continuation is authorized.

## Authorized DNS/TCP diagnostic stopped — cumulative168

The [once-only diagnostic result](project-environments-network-probe-result.md)
records two configured-resolver queries, one returned public IPv4 address, no
AAAA address, and one IPv4 TCP connect timeout. TLS0, HTTP/API/authentication0;
no refresh or preview. Actual context was measured; underlying cause and both
prior remote refresh outcomes remain uncertain. One charged connection brings
cumulative usage to168/500. New/old claims and every ledger remain preserved.
Offline59 unit/15 transport checks passed before execution. No repair, settings
change, retry or continuation; release/rollback bytes and all holds remain.

## Local network-context comparison and167 accounting — review only

The [context comparison and probe proposal](project-environments-network-context.md)
compare retained successful/failed invocation history with local namespaces,
routes, resolver/proxy, firewall/VPN configuration and journal evidence. Earlier
Wi-Fi/DHCP events do not establish the connect-timeout cause; historical actual
network snapshots and live firewall visibility are incomplete. Both refresh
outcomes remain uncertain. Accounting verifies167/500 and preserves all original
proofs/claims. No authentication, preview, external probe or repair ran. The
bounded DNS/TCP/TLS plan is proposed only; release bytes and all holds remain.

## Authorized review-4 stopped — typed connect-timeout

The [review-4 result](project-environments-refresh-preview-review-4.md) records
one execution of7e6e504 with a fresh166-based window. One refresh attempt at167
recorded connect-timeout/connect, no numeric code or HTTP status. No identity
verification or provider work followed; no diff or new packaging. Cumulative
167/500, authentication1/provider0. Underlying cause and remote refresh outcome
remain unverified; prior uncertainty is preserved. Both failed runs/claims, all
ledgers, tooling/policy, release and rollback bytes remain intact. No repeat,
repair, widening or restart. Same cumulative $1 allowance,333 arithmetic calls
remaining; all live-work holds remain.

## Local authentication diagnostics and166 accounting — review only

The [local investigation](project-environments-auth-transport-investigation.md)
finds no retained subtype establishing the historical transport cause; remote
refresh outcome remains uncertain. Typed, bounded category/phase/numeric-code
diagnostics and offline regressions are added without messages or request data.
Accounting now verifies166/500 against all frozen proofs, preserving the failed
165 campaign claim. No network probe, AWS request, live refresh/preview or repair
ran. Source, policy and release/rollback bytes remain unchanged. Proposed recovery
requires separate review and authorization; all live-work holds remain.

## Authorized refresh-preview stopped — 2026-10-28 MDT

The [live refresh-preview result](project-environments-refresh-preview-result.md)
records the one authorized9cf8d74 invocation. One refresh attempt ended in
transport-failure with no HTTP status; remote outcome and cause are uncertain.
No identity verification or provider startup followed. New authentication1,
provider0; cumulative166/500. No diff or regenerated packaging. Original ledgers,
new private receipts, reviewed policy/tooling, release and rollback bytes remain
intact. No repeated refresh, repair, widening or restart. The same cumulative $1
allowance and all live-work holds remain;334 calls are arithmetic headroom only.

## Shared-budget handoff implemented — local review only

The [amended tooling packet](project-environments-shared-budget.md) implements
a single-use authentication ledger and original-deadline handoff to the provider
meter. The read-only policy is byte-identical. Offline ledger, exhaustion,
deadline, failed-authentication and reuse/reset tests passed; no AWS calls,
refresh or live preview occurred. Prior165/ceiling500 and the cumulative $1
allowance remain. Source, original ledgers, release and rollback bytes are
preserved. Execution awaits review of the amended commit and exact PR #18
command; all live-work holds remain.

## Refresh authorization preflight — shared-budget gap

The [local handoff preflight](project-environments-refresh-handoff-preflight.md)
confirmed unchanged reviewed0d1defa tooling cannot carry refresh calls and the
original deadline into the provider meter. Its non-fixture launcher hardcodes165
prior calls and a new3,600-second clock. The offline probe made zero AWS calls;
no refresh or preview started. Cumulative165/500, original ledgers, policy,
release and rollback bytes remain unchanged. A concrete handoff amendment is
proposed for review, not implemented; executing it requires resolving the
identical-tooling/no-repairs constraint. The cumulative $1 allowance and all
live-work holds remain.

## Latest authorized invocation — local credential refusal

The [review-2 result](project-environments-preview-credential-stop.md) records
the one authorized attempt with tooling0d1defa: expired cached credentials
stopped it before proxy/SST startup, with zero new AWS calls and165 cumulative.
No resource diff or new artifacts; original ledgers, policy, prepared release
and rollback bytes preserved. No refresh, repair or restart. All live-work
holds remain, and no continuation follows from the unused335-call headroom.

## Local attribution/diagnostics follow-up

The [endpoint investigation](project-environments-endpoint-investigation.md)
reports the limits of the retained private evidence and locally tested bounded
denial metadata. Historical hostname attribution is still unestablished; no
endpoint-policy change is proposed or applied. Both original ledgers are frozen,
and tooling carries cumulative165 forward without a new window. Release/rollback
bytes remain unchanged; no AWS calls or preview rerun occurred. All live holds
remain in effect.

## Actual authorized preview — stopped

The [preview result and classified partial diff](project-environments-preview-result.md)
record the one authorized invocation of reviewed tooling843428a: stopped on an
unrecognized endpoint at30 additional/165 cumulative physical AWS attempts.
The endpoint attribution and complete resource plan remain unavailable. The
frontend matches prepared bytes; no Lambda ZIPs completed. All original release
and rollback hashes are preserved. No policy change, refresh, repair, restart,
deployment, closure, activation or runtime work. The earlier proposed preview
window below is historical; unused335 calls do not authorize continuation.

## Local provider-metering continuation — 2026-10-27

The [preview meter implementation and fixture evidence](project-environments-preview-meter.md)
now address the provider-process metering preparation gap locally. No AWS calls
or live preview ran. The proposed separate60-minute/365-call preview window
retains135 prior calls, the cumulative500 ceiling and the same $1 allowance.
Review/authorization remains required before preview, with no automatic
deployment, closure, activation or runtime continuation. Release source and all
prepared/rollback hashes remain unchanged.

## Authorized inventory continuation — 2026-10-27

The [inventory and artifact packet](project-environments-inventory.md) records
the separately authorized read-only preparation: 135/500 metered attempts,
current stage/IAM/resource/accounting readbacks, prepared release hashes and
verified retrieved rollback bytes. Release source is unchanged. The campaign
stopped **before SST preview** because complete provider-process metering is not
established. Only the separately authorized existing-session authentication
refresh was an AWS write. Deployment, closure, migration, activation,
identity/project creation and live targets remain held. The original preparation
and its then-current holds below are historical; the continuation does not
authorize a preview or further live work.

## Original preparation

Prepared 2026-10-27. **Proposed for review; no live work authorized or performed.**
AWS inventory/preview, deployment, migration, capability activation, identity
creation and live target requests remain held. OpenAPI production
import/validation remains separate and unaccepted; evidence 3 is exclusively
the accepted environment version. Publication does not lift any hold.

## Authority and exact source

Release **`44f1fb79d943501c95e06fcc1a6114111a7c1ea4`**, the merge of
[PR #16](https://github.com/VioletProto/Kurier/pull/16). It includes reviewed
implementation head `82fa7d760a485934bde3259f7652ab1c434ea85f`, including the
decoded-JSON delimiter correction. This is a fixed source choice, not a request
to deploy a moving main or this documentation branch.

The [accepted environment contract](../architecture/drafts/project-environments-review.md)
and [compatibility amendment](../architecture/drafts/project-environments-compatibility.md)
were accepted at `d8be596a2306f2e888ef34e81dfa836bc4b84447`.
Read together with the [implementation record](project-environments.md),
[accepted architecture](../architecture/README.md), repository AGENTS.md and
[ADR 0002](../decisions/0002-serverless-persistence-topology.md).
Precedents reviewed: [assertions deployment review](workflow-assertions-deployment-review.md),
[execution preparation](workflow-assertions-execution-package.md),
[actual H–L closeout](workflow-assertions-hl-replacement.md), and
[frontend retention/rollback](frontend-hosting.md#upload-order-retention-and-rollback).
Their historical authorizations and allowances do not carry into this campaign.

Local main and GitHub main were both
`9e181b238b50361284b0c4119a00833b48bf420c` at preparation. Only README/planning/
operational documents changed after the selected merge. Services, apps, infra,
scripts, package files and CI workflow compare identically. Tree pins:

| Tree     | Git object                                 |
| -------- | ------------------------------------------ |
| services | `4015676405b5946f499c7222fed59bb9c269fc64` |
| apps     | `530521097e84c1b397fa73d2cae070da3a16592d` |
| infra    | `fe48bd9ee357b2d5c74223a46b18a8a014048fe1` |

The existing [environment card](https://trello.com/c/K2e3Nv2f) is the only Doing
card in the board readback; configured WIP limit is three. Its accepted/local
implementation checklist remains complete, while deployment/runtime stays open.
The card's old statements that PRs #15/#16 were open require status correction.

## Expected application, frontend, infrastructure and IAM delta

These are **source-derived expectations**, not an AWS diff or current inventory.
The last retained assertions deployment source was
`68d9f218225c7092a1e4fca0b66a21421d862a22`; its artifacts, stage version 15 and
savedRequests8/assertions1 are historical evidence only. Live drift is unknown.

| Surface             | Expected change and review boundary                                                                                                                                                                                                                                                                                                                                                                          |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| API/hosted MCP      | Environment CRUD, strict typed reusable secrets, opt-in requests, guarded freeze/admission/rerun, agent negotiation, evidence3 owner reads and explicit legacy MCP projection. Existing API Lambda package also carries cleanup code.                                                                                                                                                                        |
| Maintenance         | Compatible environment/tombstone, typed-secret, frozen context and agent-row recognition; bounded deletion preserves unknown shapes. Replace EmptyProjectCleanup code with the same API source.                                                                                                                                                                                                              |
| Worker              | Shared bounded resolver, current eligibility checks, frozen workflow selections and evidence3 with optional assertions. Replace CloudExecutionWorker code.                                                                                                                                                                                                                                                   |
| Controlled endpoint | ExecutionControlledEndpoint shares services/worker; expect its code package to change too. No new fixture behavior is proposed.                                                                                                                                                                                                                                                                              |
| Foreground clients  | New local-agent binary negotiates Work/Result2 before marked admissions. Existing MCP stdio tool schema remains unchanged; verify its seven-tool compatibility. No automatic client installation.                                                                                                                                                                                                            |
| Web                 | Environment editor, metadata selectors, request opt-in, workflow inherit/none/select and evidence context. Exact decimal text, fixed protected masks, uncertain-write reconciliation. Build static release with verified public API/Cognito/stage configuration; publish archive/assets, KV entries and waited invalidation through the existing hosting procedure. Require tab reload before new authoring. |
| API Gateway         | Six application operations: collection POST/GET, item GET/PATCH/DELETE under `/api/v1/projects/{projectId}/environments`, plus POST `/api/v1/local-agent/capabilities`. Two explicit environment collection/item OPTIONS routes. Generated route/integration/permission resources must be classified in a later preview; do not reuse the assertions eleven-resource allowlist.                              |
| IAM/resources       | Environment merge edits no IAM policy declarations. Existing Control/Protected/KMS/S3/SQS permissions and encryption-purpose conditions are expected to suffice. Gateway invocation permissions may be generated for added routes. No planned new table/index/key/bucket/queue/pool, standing compute, IAM data-access expansion or public bucket access. This is unverified against deployed/generated IAM. |

Retain Free plan, quotas, account concurrency10, worker batch1/max2, existing
memory/timeouts, one-minute maintenance, logging retention, region us-east-2 and
stage dev-api. No bootstrap replacement, dependency upgrade, CORS broadening,
Cognito/email change, retention cleanup or unrelated infrastructure work.
Initial DevelopmentStage uses `ignoreChanges:["item"]`; application startup and
SST deployment must not activate or reset durable capability/accounting rows.
Existing Lambda schema environment defaults are not the durable activation gate.

## Compatibility and migration decision

| Domain                  | Required behavior                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Existing saved requests | Unmarked configurations/row1–2 remain literal, including braces. No bulk opt-in or rewrite. Marked requests use row3/`templateSemanticsVersion:1`; PATCH cannot remove opt-in. A selector on a legacy request rejects.                                                                                                                                                                                                                                                                                                                              |
| Protected secrets       | Schema1 strings project as string; legacy jsonScalar/jsonBody remain untyped. New saved-secret row2 has trusted immutable scalarType. Typed PATCH retains `{"value":string}` and rejects cross-type values. Legacy untyped scalars cannot attach to typed variables; explicit set creates a fresh secret ID. Never infer type from a mask or silently upgrade old ciphertext.                                                                                                                                                                       |
| Frozen data/evidence    | Existing plans, reruns and immutable evidence1/2 retain their semantics/bytes. New snapshots freeze templates, used public typed values, exact secret refs and effective environment identity. Evidence3 contains only semantics/context additions, never variable values/maps or secret IDs. Rerun uses frozen public values and current values at exact frozen secret IDs; removed IDs cannot rebind by name.                                                                                                                                     |
| Local agents            | Credential/generation-bound exact capability declaration is required before a marked request reserves quota or returns 202. Missing declaration gives agent_incompatible with no send. Legacy session DTO and Work/Result1 stay exact for legacy work. Work/Result2 identity/context must match, including null; accepted upload retries use retained canonical receipts after input cleanup. No local workflows. Old agents can execute legacy work but cannot execute marked work.                                                                |
| MCP                     | Same seven tools and wire schemas. Valid evidence3 projects through a dedicated legacy DTO; stored bytes/digest/pins stay unchanged. Complete uses `view:"complete"` and `capture`, complete=true/truncated=false, reason environment_context_not_exposed and omittedFields `[templateSemanticsVersion,environmentContext]`; with assertions use unsupported_context_not_exposed and `[assertions,templateSemanticsVersion,environmentContext]`. Summary retains mcp_summary_view and appends omissions once. Unknown/invalid versions fail closed. |

**No mandatory data backfill, ciphertext re-encryption or evidence migration is
required by the accepted representation.** A closed-stage capability transition
is required. Inventory must establish all retained shapes are supported; any
unknown shape or proposed legacy-type conversion stops for a separate migration
review. No scan/rewrite/reset is authorized here.

## Proposed deployment order and closed-stage window

All steps below are unperformed and require later explicit authorization.

1. In a clean detached checkout of the pinned source, create final SST-compatible
   ZIPs and a hosted frontend manifest using reviewed public configuration.
   Record hashes/sizes, toolchain/lockfiles and embedded VCS identity. Capture
   previous Lambda bytes and frontend archive/hashed assets/KV entries with
   verified hashes; previous code S3 keys may be deleted on replacement.
2. After inventory/preview authorization, verify intended account/non-root
   identity, Ohio/dev-api, bootstrap/provider behavior, deployed code/config/IAM,
   all managed-policy versions, routing/static assets, stage tuple/version/
   generation, quota/month/counters, queue/DLQ and maintenance. Preview the exact
   release; reject unrelated/replacement/data/IAM changes. Retain sanitized proof.
3. Coordinate a proposed **30-minute maximum closed-stage window**. First stop
   campaign submissions and let known inflight work settle under compatible
   baseline code. Prove terminal jobs/runs/steps, outbox publication against the
   matching terminal job and immutable evidence, settled uploads, no pending
   leases/local waits/runtime IN/OUT buffers or deletion work. `published` is an
   outbox state, not sufficient quiescence by itself; never expect historical
   `sent`. Queue emptiness alone is insufficient. Unknown outcome means stop,
   not replay. Closing cannot cancel an already issued outbound request.
4. CAS the freshly observed complete active tuple into `workflow-migration`,
   incrementing version only; preserve recoveryGeneration and every capability/
   accounting field. Reinventory after closure for races. With traffic fenced,
   deploy API/hosted MCP, maintenance, worker, shared fixture package and frontend.
   SST can update concurrently: closure is the barrier, not resource scheduling.
   Verify **all** readers/cleaners/code hashes/config/IAM and static manifests
   before allowing any new writer. Confirm old-data reads with compatible code.
5. Verify the complete reader gate and new local-agent distribution/negotiation
   readiness. Activation requires its own acceptance and a newly reviewed,
   locally tested activator. Existing assertions activator expects schema7 and
   is unsuitable; do not run even its inspect mode under current holds. This
   packet adds no activation command.
6. Conditional activation from the exact closed baseline changes savedRequests
   8→9, protectedSecrets1→2, localAgentExecutions1→2, absent projectEnvironments
   →1, state→active, version→version+1. Retain cloudExecutions1, executionInputs1,
   evidencePinning1, sequentialWorkflows1, mcpReadOnly1, workflowAssertions1.
   Require the full old tuple, closed state, version and generation in the
   condition; no partial tuple, initialization or retry with silently refreshed
   expectations. Strong readback must match only intended changes and unchanged
   month/count/storage/pins/inflight/quotas/generation. Recheck before admission.

The historical schema8 baseline must be freshly proved, never hardcoded as live
truth. Close plus activate increments the observed stage version twice. If the
window expires or any guard fails, stop and leave the stage closed; 30 minutes
is a decision deadline, not automatic reopen authority. If activation is deferred,
reopening the old tuple needs a separately reviewed guarded action and proof
that no new stored shapes exist. Browser caches and foreground agents require
explicit compatibility checks; a frontend rollback alone cannot revert data.

## Failure handling and rollback limits

Before activation/new-format writes, rollback to preserved prior application and
frontend bytes may be possible while closed, after proving no new stored shapes.
Rebuilds are not byte-identical rollback proof. Restore frontend archive with
matching public configuration and retained hashed assets/KV keys, reviewed diff,
waited invalidation and browser reload; keep old assets (`purge=false`).

After the new tuple, old binaries can reject the stage. Once row3 requests,
typed row2 secrets, environment/snapshot/capability records, Work/Result2 receipts
or evidence3 exist, lowering markers or rolling back old code is unsafe. Close
with a fresh version/generation guard and restore compatible readers/cleaners or
fix forward. Preserve new data and immutable evidence. Do not strip context,
downgrade evidence, reset counters/generation, reuse old grants or automatically
rerun requests. Environment deletion only detaches refs; secret revocation and
project deletion retain their separate lifecycle.

On uncertain mutation/submission/upload, reconcile through the original receipt/
status identity using bounded reads; never issue a fresh mutation to discover
whether the first succeeded. Missing buffers/source eligibility gives unavailable
or unknown outcome, never a second send. Stop on unexplained accounting drift,
unexpected IAM deny, secret exposure, transport failure (including background
OPTIONS), ambiguous publication or cleanup gap. Drain browser reads before
closing contexts; the assertions closeout's nine transport failures remain a
historical gap and its correction is locally verified only.

Closure cannot undo sent HTTP, revoke downloaded evidence or retract values
authored as public. PITR is separate authorization with ADR 0002's possible loss
of recent data and resurrection of later deletions; reconcile Control/Protected/
S3, advance recovery fencing appropriately and suppress restored job replay.
It is not a shortcut for this rollout's rollback.

## Bounded genuine runtime campaign proposal

Not performed. Start only after deployment/activation verification and explicit
campaign approval. Preassign case IDs, submission keys and synthetic canaries;
meter every physical attempt before forwarding, including SDK retry/page calls,
Cognito, browser OPTIONS/static, local polls/uploads and failed transports.
Freeze sanitized ledgers/hashes and timestamped readbacks; retain no credentials,
raw protected values, response bodies or plaintext-derived diagnostics in Git.

Use two disposable Cognito identities, one project per identity, and foreign-owner
checks between them. Use only the existing owned cloud fixture and one foreground
loopback controlled endpoint with explicit origin consent; zero direct live fixture
probes. First confirm the existing fixture can provide required safe observations.
If additional echo/audit behavior is needed, review it separately before any live
request; no fixture implementation is included here. Browser checks use genuine
hosted auth; local tests below do not establish that path.

| Case                    | Planned admitted steps / maximum sends | Required observations                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ----------------------- | -------------------------------------: | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A Legacy compatibility  |                                  5 / 5 | New unmarked cloud seed and its deliberate rerun, old-agent local standalone, plus two-step legacy cloud workflow. Literal braces, Work/Result1, evidence1/2 and frozen legacy rerun/read behavior. No retained historical context is required or assumed.                                                                                                                                                                                                                                                                                                      |
| B Typed cloud/local     |                                  2 / 2 | Same marked template on cloud and negotiated local: exact `0.10000000000000001`, exponent spelling, boolean, present null and string; decoded Unicode delimiters, inert inserted markers, query/header/path escaping. Public base only; protected auth/body synthetic canaries masked in UI and absent from evidence/logs/MCP/persistence. Compare sanitized capture and controlled endpoint observations; assert no secret in URL.                                                                                                                             |
| C Frozen reruns         |                                  2 / 2 | Rerun B after public edit and exact secret PATCH. Public value/environment revision remain frozen; current value at exact frozen secret ID is used. Same-key receipt retry returns the same execution with no admission/send increment. Original evidence bytes/digest unchanged.                                                                                                                                                                                                                                                                               |
| D Workflow precedence   |                                  6 / 6 | Three-step cloud workflow with default inherit, explicit none on zero-token marked request and explicit select. Assertions and supported output bindings coexist; bindings apply after single-pass resolution. Rerun after definition/environment edits preserves frozen public data/effective selections and current exact secrets.                                                                                                                                                                                                                            |
| E Failure containment   |                                  3 / 1 | Two admitted workflow steps with producer assertion failure and skipped successor: one send. One marked local admission while approved disposable agent is stopped before polling; disable/remove a used variable, then start agent and prove no dispatch. Do not interrupt a live send.                                                                                                                                                                                                                                                                        |
| F Explicit null context |                                  2 / 2 | Marked zero-token cloud/local with no selection produces evidence3/context null and Work/Result2 locally. Owner read/pin/download and MCP complete/summary omission flags preserve original checksum.                                                                                                                                                                                                                                                                                                                                                           |
| Negative/CRUD/cleanup   |                                  0 / 0 | CRUD list/detail/masked reopen, unrelated edit/exact decimals, immutable ID/type/protection, replacement arrays; missing/malformed/stale If-Match and foreign owner. Missing/disabled vars, mixed nonstring interpolation, malformed/oversize tokens, unsafe URL/CRLF, direct/workflow overlap, legacy selector, untyped scalar attach, cross-type secret PATCH, old-agent marked admission and local workflow reject before quota/send. Deleting environment/variable then rerunning rejects exact identity; revocation cannot rebind a same-name replacement. |
| Total planned           |                            **20 / 18** | Failed/skipped reserved workflow steps still count as admissions; prove against fresh quota and unique target audit IDs.                                                                                                                                                                                                                                                                                                                                                                                                                                        |

Case A's seed and legacy rerun are performed after activation, proving the new
readers preserve legacy semantics. Four reserve steps are available only for
diagnosed, explicitly recorded corrections within the approved ceilings.
No broad faults, deliberate duplicate queue delivery, mismatched raw uploads,
PITR or retention-clock manipulation in this campaign. Invalid Result2 identity,
canonical receipt retries and atomic races remain local evidence unless a later
separately bounded fault exercise is approved.

Delete a disposable environment first and prove reusable secret remains while
deleted-context reruns fail; explicitly revoke the test secret and prove denial.
Then delete both projects through normal 202 operations; record tombstone/operation
IDs and immediate owner/MCP denial. Verify physical Control/Protected drainage,
no jobs/OUT/IN/leases/pending uploads, scoped evidence S3 absence, settled orphans,
pin/storage/inflight decrements and no late resurrection. Revoke test agent/MCP
credentials; sign out/delete provider identities and independently prove absence.
Preserve unrelated users/projects, quota history and monthly test charges.
Read back stage/generation/counters and target audits before and after cleanup.
If physical cleanup exceeds the bounded window, retain an open gap and stop;
no unmetered cleanup continuation or replacement identity is implied. Elapsed
retention, PITR, general signup/email and maximum-load evidence remain separate.

## Proposed allowances and cost boundary

These are new **proposals**, not authority or historical unused balances.

| Allowance                             | Cumulative ceiling for a future approved campaign                                                                                                                                                                                                                                                                                                                             |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Preparation now                       | Local checks and GitHub/Trello publication only; zero AWS calls, identities, projects or live target requests                                                                                                                                                                                                                                                                 |
| Future inventory/preview preparation  | One 60-minute window; at most 500 physical AWS calls including identity, pagination, retries and preview/provider calls; zero target requests or writes except separately approved tooling effects. Unmeterable provider calls require a revised plan before preview.                                                                                                         |
| Deployment/closed stage               | One release, one close/activate sequence; closure decision deadline 30 minutes. No automatic retry/reopen; deployment calls count toward future execution allowance below.                                                                                                                                                                                                    |
| Future execution window               | 120 minutes from first preflight through final readback, including at least final 30 minutes reserved for cleanup; stop new admissions at minute 75                                                                                                                                                                                                                           |
| Admissions/sends                      | 20 planned admissions/18 planned sends, at most **24 admitted steps / 24 target sends** including up to four reserve steps; at most 40 submission/rerun POST attempts: 14 planned accepted submissions, up to 20 negative admissions and six correction/retry attempts, including uncertain attempts. Reserve covers diagnosed corrections only; no replay of ambiguous work. |
| Identities/projects                   | At most two disposable identities and two projects cumulative, not simultaneous; no replacements without review                                                                                                                                                                                                                                                               |
| API/provider/static                   | At most 600 physical application API attempts (including OPTIONS, local polling/uploads, MCP hosted calls), 240 status polls included, minimum two-second status interval; 80 Cognito calls and 150 static requests                                                                                                                                                           |
| AWS control/data reads and deployment | At most 1,500 operator/tool/provider AWS calls, including pages/retries/writes, across deployment/runtime/cleanup; adds to the separate 500-call preparation ceiling. Runtime SDK background service usage is observed separately and included in cost estimation, not falsely counted as operator calls.                                                                     |
| Incremental cost                      | **$1.00 total procedural stop allowance** across inventory/preview, deployment and campaign; preserve existing $5 light-development target, quotas and concurrency. This is not a billing-enforced cap.                                                                                                                                                                       |

No current price lookup or account bill was performed. Historical
[Ohio price capture](workflow-assertions-prices.json) is context only. Refresh
public rates and inspect actual billing after authorization before execution;
account-wide/shared charges and delayed billing remain uncertain. Source suggests
no additional fixed-price resource, but typed metadata/frozen snapshots add DDB
read/write/storage and KMS operations, evidence bytes add S3/transfer, and new
packages/static release add storage/PUTs/invalidation. Retries, polling, maintenance,
CloudFront/KV, backups and retained archives also cost money. Stop before a call
whose conservative estimate would exceed the allowance; unknown earlier spend
does not become zero. No quota, plan, timeout or concurrency increase to finish
tests. A fresh metered driver/allowance review is required before any execution.

## Local verification and remaining gates

Fresh checks on production-equivalent main passed `npm run verify`: format/gofmt,
ESLint, 66 web tests, 15 infra tests, workflow polling/assertions tooling and
Python metering tests, TypeScript/Vite build, tests/vet across all nine Go modules
and both spike test-binary builds. Some ordinary Go results reused the local
cache. Uncached `go test -race -count=1 ./...` passed in request-runtime and
local-agent. Initial sandbox attempts denied subprocess/loopback operations;
reruns with approved local process/socket permissions passed without code edits.

Clean detached selected source also cross-built API and worker bootstraps with
Go `go1.27.3-X:nodwarf5`, Linux/ARM64 v8.0, CGO disabled, `-trimpath -ldflags='-s -w'`.
Both embed the selected revision and `vcs.modified=false`. These are local build
checks, **not** final SST ZIPs, verified deployed hashes or a hosted frontend
release. The ordinary web dist build does not establish production public config.

| Local bootstrap                |      Bytes | SHA-256                                                            |
| ------------------------------ | ---------: | ------------------------------------------------------------------ |
| API (also used by maintenance) | 23,003,296 | `40c42862b302f407b3b3dce213ccaf722d1b37adbe95d281e576337e892b219d` |
| Worker (also used by fixture)  | 21,495,968 | `2d9530aca9e0939d3604ef2c6401be67777993992a2d8fc466a817ce0a1447d2` |

Local build files are under `/tmp/kurier-environments-release-build`; their
temporary retention is not a release archive or rollback guarantee.
Final artifact packaging, retained rollback bytes and manifest/hash review are
next-stage obligations. Repository formatting and `git diff --check` cover this
packet and its links; no application code or accepted DTO is changed.

This task did not rerun DynamoDB Local/Chromium integration. The implementation
record and exact-head CI retain those fixture results; they do not prove real
KMS, S3, SQS, Cognito, IAM, DNS/network, elapsed retention, billing or deployed
cleanup. No AWS inventory/preview/mutation, activation, identity creation or live
target request occurred. Current stage/accounting/drift and generated resource
counts remain unknown.

## Decisions for the next stage

1. Accept the exact release source and this expected scope; authorize a bounded
   read-only inventory and guarded preview separately, including any unavoidable
   SST tooling metadata effects. No deployment follows merely from preview approval.
2. Review fresh diff, final package/static hashes, rollback archive proof and
   compatible-reader matrix before authorizing deployment and its closure window.
3. Authorize preparation/review of an environment-specific guarded activator,
   then separately accept its full-tuple transition and post-data rollback limits.
4. Approve named disposable identities/projects,
   controlled endpoints/observation capability, cumulative call/time/cost bounds
   and normal cleanup before genuine runtime execution. Keep any additional
   fixture behavior/fault campaign and OpenAPI production work separately scoped.

The environment card remains open. Packet publication completes release
preparation only; none of these decisions is inferred from publication or merge.
