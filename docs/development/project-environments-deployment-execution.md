# Project environments: adopted artifacts and execution review

**Latest: [read-only recovery inspection supports a guarded old-release reopen proposal](project-environments-recovery-inspect-868-r1.md).**
Stage remains closed at `workflow-migration`/16. The separately authorized inspection
used 87 attempts in 71.168 seconds; usage is **955/1,799**, with **844** headroom and
the unchanged cumulative $1 allowance. Old packages/configuration/IAM/frontend,
capabilities, generation/accounting and scoped state checks passed. The exact
private transaction proposes only `active`/17 with seven accounting guards.
**No stage write or reopen occurred.** Original campaign/deadlines are expired and
unchanged; any reopen needs separate reviewed execution approval.

**Latest deployment request: [held by the local minimum-time guard](project-environments-deploy-608-t1-time-hold.md).**
At 2026-11-03 19:08:17 UTC, the fixed closed-stage window had **1,167.041 seconds**
remaining, below the required **1,320 seconds**. No approval, authentication, AWS
call or deploy invocation occurred. Usage remains **868/1,799**. The stage remains
closed at its last verified version16; neither original deadline was reset.
No reopen, rollback, repair or later phase is authorized by this hold.

**Current stage: [closed and verified, version16](project-environments-close-608-t1.md).**
The separately authorized close ran once; one exact guarded transaction changed
only state/version to `workflow-migration`/16. Capabilities, generation and accounting
are preserved. Usage is **868/1,799**, with **931** arithmetic attempt headroom and
the unchanged cumulative $1 allowance. The original campaign and fixed closed-stage
deadlines continue without reset; deployment and all later phases remain held.
See the receipt for timestamped remaining-time observations.

**Latest collection: [release-608-t1 completed initial collection](project-environments-collection-608-t1.md).**
The missing source worktree was restored using Git's supported scoped mechanism;
clean source and pinned local caches/preflight passed with no downloads. The
approved start ran once from clean tooling **4de31ab9a9c791a1fc224bbce8a7fe79220519ba**.
Fresh capture, STS, readers and guarded preview passed without renewal. All four
ZIPs and frontend match adoption; the complete diff matches review 8.
Usage is **716/1,799**, leaving **1,083** arithmetic attempt headroom; cumulative
$1 allowance unchanged. Collection completed in **68.575541 seconds**. No later
phase ran. All eight campaign claims and historical evidence are preserved.
The preceding zero-call [preflight hold](project-environments-collection-608-t1-preflight.md)
remains intact; that failed local check had consumed no campaign or claim.

**Timing amendment implemented locally:** [verified workflow and commands](project-environments-timing-amendment.md),
clean tooling **4de31ab9a9c791a1fc224bbce8a7fe79220519ba**. Deploy is bounded to
840 seconds; other phases remain 240. Renewal uses the actual clipped remaining
phase plus five seconds on one pinned fetcher instance. Original 55-minute
campaign and fixed 30-minute closed maximum remain unchanged. Usage **716/1,799**.
No live work is approved or performed by this amendment. The `release-608-t1` start claim is now consumed; do not rerun it. Closure/deployment remain held pending review of the completed fresh evidence and separate phase approvals; 840 seconds is not proven AWS completion capacity.

**Historical successful collection:** [release-499-f1 completed initial collection](project-environments-collection-499-f1.md)
from clean tooling **bf1effb2835ae6be00bdbd753a802dc0b9bf6741** and unchanged
release source. Explicit fresh-at-start capture, one metered renewal, fresh STS,
read-only collection and guarded preview passed. Usage is **608/1,799**.
All four generated ZIPs match adoption exactly and frontend agreement passed.
Collection ended after **90.598714 seconds**; no later phase ran. All seven
campaigns/claims, old checkpoints and artifacts remain preserved. Historical
grant continuity remains unproved. The previous zero-call
[release-499-p1 drift stop](project-environments-collection-499-p1.md) remains intact.

The [fresh-start policy](project-environments-fresh-start-policy.md) is now exercised
by this separately authorized campaign. Its `release-499-f1` claim is consumed;
**do not rerun its historical start command**. Strict continuity and metered renewal
apply within this campaign. Any close or later phase needs separate explicit
approval under the same original deadline; neither call headroom nor this packet
authorizes continuation, fresh recapture or restart.

This packet adopts the exact review-8 ZIP candidates in
[the adopted manifest](project-environments-adopted-release.json), supported by
[binding attestation cb74c992](project-environments-key-observation.md).
The original prepared manifest and historical candidate manifest remain intact.
Release source is **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**. Adoption accepts
artifacts only. Deployment, closure, migration, activation, identities/projects,
and target/runtime execution require further authorization.

Accounting is now **955/1,799**, with **844** arithmetic attempt headroom within the
approved ceiling. The cumulative **$1 procedural allowance** is unchanged;
billing remains unmeasured. All six historical stopped campaigns and both completed collection campaigns, with their consumed claims, are preserved. The cp1 collection added 109 metered attempts, including one permitted renewal.
The fresh f1 collection added another 109 attempts. No later phase or infrastructure/state write occurred. Historical records and adopted/rollback bytes are unchanged.

## Artifacts and byte enforcement

The new manifest links the prior candidate manifest SHA and binding-attestation
SHA/commit. API/cleanup ZIP SHA256 is
`58f2bbf364042ac209d4f9b3a4d6ca583da18914fb5851f0c166d34b9c757373`;
worker/controlled endpoint ZIP SHA256 is
`82af7beeda4b619afa6831b342c693bb1fb3e8d08e5e6c0416ce253526f59098`.
No adopted or rollback ZIP was modified. The cp1 preview generated four separate
ZIPs whose hashes differ only because of external-attribute bytes; they remain
private evidence and are not adopted. Historical rollback bytes and evidence
are checked by the existing preparation proof verifier.

The host loads these exact bytes independently of the child. PutObject and
multipart bodies must match the reviewed payload, including complete ordered
part coverage and confirmed ETags. AWS chunked bodies are decoded and checksums
checked; unexpected framing/checksum variants stop. Generated SST ZIPs must
match all four adopted archive hashes and exact members before infrastructure
writes proceed. The frontend marker and file hashes must match the manifest.
The actual forwarded upload bytes are checked separately, so a generated-file
check cannot substitute for the upload boundary. Any regenerated difference
stops deployment for review; do not adopt new hashes automatically.

## Collector, scope and transitions

`scripts/environment-deploy/driver.py` is the phase controller;
`collector.mjs` and `phase.mjs` are fixed commands, not a generic AWS executor.
The exact account/region/resources, eight environment routes, four functions,
S3 keys, frontend namespace, IAM read scope and expected invocation permissions
are recorded in `project-environments-execution-scope.json` and pinned by
`pins.json`. IAM mutations, unrelated resources and unknown wire operations are
held before forwarding. Only the fourteen explicitly approved additional reads extend the preview policy.

Fresh collection compares the deployed configuration/code/IAM/generation and
routing to the accepted inventory. It scans only projected metadata, checks
queue/residual quiescence and frontend metadata, and reads the marker before
and after collection. A fresh SST diff must have the same reviewed projection
as review-8 (60 entries, 44 unique resources) and the same four generated ZIPs.
Collection and closure now require the accepted old frontend manifest and bytes;
verification, activation and final require the adopted release. Both live checks
and offline stage planning bind the expected frontend target/release ID. Closure
rechecks the old frontend before its transaction. The exact old-JS object/KVS reads are now granted separately from upload targets;
other old assets, versions, entries and mutations remain outside that grant.
Fresh outputs remain private. Stale or drifting baselines cannot be blessed by
simply recording them as a new baseline.

The permitted changes are the reviewed eight routes/integrations/Lambda invoke
permissions, four Lambda code updates, frontend files/KVS/invalidation and
exact code assets. SST bootstrap state writes are separately gated to the
owned application/stage: one new deploy lock, its run/update ID and its own
checkpoint/history objects, then its own lock release. No force unlock,
passphrase/bootstrap writes, unrelated history deletion, IAM role/policy or
database schema mutations are granted. Unknown provider requests stop.

Stage close and activate use the existing environment-specific guarded CAS
engine. The proxy permits only the exact planned TransactWriteItems body and
client token, then checks an independent response marker hash. Phase approval
binds source/tooling, manifest, input proof and authorization hashes; close and
activate also bind engine approval hashes. Sequence is collect → close →
deploy → verify → activate → final. Every phase stops for review. There is no
loop that automatically advances phases.

## Historical execution proposal and allowances

The initial command below was authorized and ran once, then stopped before the
first physical request. It is retained as execution history, not a rerunnable
proposal. All four stopped campaigns and consumed claims must remain intact.
No future live continuation is proposed; review the credential boundary locally
before considering separately authorized work. The later fresh-capture review is
now complete locally; only the linked new campaign is proposed for separate
initial-collection authorization.

Proposal only: a separately authorized new campaign starts from **390**, retaining
all prior usage and the approved **1,799** cumulative ceiling, with at most
**1,409 additional physical attempts**. Keep one original **55-minute** monotonic
window starting before authentication and at most a **30-minute** closed-stage
subwindow bounded by that original deadline. The $1 cumulative procedural
allowance is unchanged; call headroom does not establish cost or capacity. Zero
runtime requests, new identities, new projects or runtime fixtures are included.

The historical run used clean tooling commit **9eafa58928b5a2486f30bd7531f18c324d4fe534** from this PR
and the clean detached release worktree at the unchanged source above. Historical f1
fresh-start tooling was **bf1effb2835ae6be00bdbd753a802dc0b9bf6741**; current timing tooling is **4de31ab9a9c791a1fc224bbce8a7fe79220519ba**. The linked fresh campaign completed collection; its start command is consumed. Required
caches remain SST 4.17.1, Pulumi AWS provider 7.40.0, reviewed SDK/models and
existing dependencies. `driver.py check` also validates the stopped parent's
sanitized records, all three original authorizations/claims, 300–390 attempt outcomes
and latest recorded same-session cache metadata. No install/download/repair is included.
`check`/`template` additionally report `pendingScopedReads`; a nonempty list
hard-stops `start` before credentials, claim creation or any network activity.
The prior four reads and the ten configuration reads approved at `69f31ef` are
implemented and pinned. The [production-policy fixture results](project-environments-bucket-read-correction.md)
show no remaining observed permission gap in the minimal bucket read. Unknown
operations remain held. A new initial collection requires separate authorization
of the exact tooling/source and a fresh bound authorization; the local correction
provides no live execution authority.

The pinned Sign-In model caps credential lifetime at **900 seconds**, with no
requested-duration input. The pinned SDK defaults to renewal at **300 seconds or less**;
the current amendment raises only the individual fetcher threshold to cover actual remaining phase time plus five seconds. Renewal rotates the same locked cache. Credentials need to cover the bounded
phase, rather than the entire campaign. The historical policy gave every phase a **240-second** ceiling; the current timing amendment gives deploy 840 seconds and others 240,
including authentication, bounded by both original campaign and closed-stage
deadlines. Before its work, require credential validity through that phase's
remaining time plus **five seconds**. Collection's reader inventory and SST diff
share one four-minute phase and one credential boundary.

Each separately approved phase permits at most one metered existing-session
renewal if the SDK requires it, followed by mandatory STS account/non-root
identity verification. Six phases permit at most **six renewals and six host STS
checks** in total; child/provider STS calls also consume the same shared budget.
Approval is checked before authentication. No background or mid-phase renewal,
second renewal within a phase, write retry, clock extension or session replacement
is permitted. External cache changes, expired refresh grants, renewal/STS failure,
insufficient validity or any deadline exhaustion latch a stop. The first boundary
must match the cache metadata retained by the latest stopped parent; later boundaries
must match the preceding successful boundary. New session selection needs review.

Four minutes is a strict stop bound, not demonstrated live deployment capacity.
A phase timeout while closed leaves the stage closed for separately authorized
inspection/recovery. See the linked credential review for source evidence,
limitations and offline regressions. Longer-lived credentials cannot be requested
through this existing login mechanism; no authentication configuration changed.

Use the original workspace so retained cache links keep their reviewed paths.
Require a clean working tree before selecting the exact tooling commit; preserve
the review branch and do not reset or remove any evidence:

```sh
cd /home/king/Neumont/Kurier
git switch --detach 9eafa58928b5a2486f30bd7531f18c324d4fe534
```

Run `check` there with the existing `.build`, `.sst` and `node_modules` caches.
Do not rebuild adopted archives or install/download dependencies. Keep the clean
release source worktree at its pinned `/tmp` location. Any cache/source/hash
mismatch stops for review.

Campaign, approval and authorization inputs are resolved and validated before
launch. The child receives exactly one read-only current context file at
`/meter/phase-context.json`; campaign directories, ledgers, claims and sibling
contexts are not mounted. Both collection children and close/deploy/verify/
activate/final use this shared mapping. Offline close/activate planning uses a
validated absolute host input path. Missing paths or symlink aliases stop;
no broad campaign-directory mount or generic command override is introduced.

Exact commands below are proposals. Do not run `start` or `phase` until the
corresponding authorization has been granted. Preserve `release-from-299`,
`release-from-301`, `release-from-303`, all three original authorization files and
their consumed window claims. The `release-from-390` campaign,
`authorization-from-390.json` and consumed `window-from-390.json` now exist and
must be preserved. Never reuse or remove a consumed claim. The template/start
commands below are historical; do not rerun them or overwrite these files.

```sh
python3 -B scripts/environment-deploy/driver.py check
umask 077
set -C
python3 -B scripts/environment-deploy/driver.py template > .build/project-environments-execution/authorization-from-390.json
```

The template is deliberately `authorized: false`. Review every field and approve
it only after explicit authorization of a new initial collection using this
corrected tooling. The documented read permissions are implemented; fresh initial
collection authorization remains required. Do not edit `pendingScopedReads` as an
approval mechanism. The accepted credential policy remains bounded to separately
approved phases. It binds the current exact clean tooling commit, manifest/scope
hashes, 390 baseline, pinned read scope and bounded credential policy. The ceiling approval remains
in effect but does not authorize this new campaign. Then the sole start command:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-390 --authorization .build/project-environments-execution/authorization-from-390.json
```

Stop after initial collection. Review its baseline/quiescence findings,
configuration/IAM comparisons, complete diff, generated hashes, ledger and remaining
time before deciding on closure. The later commands below remain proposals. No
phase approval is granted by this local correction.

Review the fresh collection, complete diff, generated hashes and ledger before
closure. For each phase, substitute exactly one of `close`, `deploy`, `verify`,
`activate`, `final` for PHASE below. Run prepare, review the bound proof and
obtain separate authorization before changing only `authorized` to true.
Do not script a phase loop or approve all phases in advance.

```sh
python3 -B scripts/environment-deploy/driver.py prepare --campaign .build/project-environments-execution/release-from-390 --phase PHASE > .build/project-environments-execution/PHASE-approval.json
python3 -B scripts/environment-deploy/driver.py phase --campaign .build/project-environments-execution/release-from-390 --phase PHASE --approval .build/project-environments-execution/PHASE-approval.json
python3 -B scripts/environment-deploy/driver.py report --campaign .build/project-environments-execution/release-from-390
```

Deploy first warms hashed frontend assets and their KVS entries, then runs the
pinned SST deployment under the byte gate while the stage remains closed.
Verification independently checks deployed code/platform, exact new routing and
permissions, frontend and closed marker. Activation is a separate guarded CAS;
final checks the active tuple, platform and accounting projection. Neither
verification nor final sends API target requests.

## Approved reads and remaining hold

The three exact reads proposed in the denial review at
`8a14fad485a8869cda8262b121e2842f979a66b6` were approved for local implementation:
GetBucketPolicy for the owned frontend bucket, GetObject for
`assets/index-CD_8s4hz.js`, and GetKey for its exact existing KVS entry. They are
now recorded in `approvedAdditionalReads`, pinned to that review, and enforced
as read-only matches. No upload/deletion target, wildcard, additional operation,
IAM mutation, framing/signing relaxation or live phase authority was added.
The approved S3 Control helper remains unchanged.

GetBucketAcl and the ten bucket-configuration reads approved from review `69f31ef`
are now separately pinned grants. Each new configuration read is limited to the
exact frontend bucket, region/account, root GET with one canonical empty selector
and empty body, under existing signing/framing/identity/deadline/accounting checks.
See the [exact operation/query table and results](project-environments-bucket-read-correction.md).
The production-policy preview fixture exits zero with 22 reconciled synthetic
forwards and no denied reads. A separate unapproved bucket read remains blocked.
`pendingScopedReads` is empty. Full release preview capacity, complete diff and
archive agreement remain unestablished; new initial collection is a separate decision.

## Failure and rollback

Every physical forward is durably admitted before transport; responses and
uncertain outcomes are recorded. Retry headers/invocation IDs, single-use phase
claims and write tickets prevent automatic retries of uncertain writes. Transport
failure, denied request, failed outcome, expiry, exhaustion, ledger/pin failure
or child failure latches the campaign stopped and terminates descendants. No
restart, repair, reopen, force-unlock or widening is implicit. Preserve all
private receipts and the claim. Publish only reviewed, sanitized accounting.

A partially deployed closed stage remains closed for independent inspection.
Rollback is a separate authorization and must use retained verified bytes plus
a fresh deployed/state/ledger inspection; the deploy driver does not perform
rollback. Existing rollback limits in the readiness packet apply: after new
version records exist, old code alone cannot safely interpret or erase them.
Do not reopen old code over incompatible records, discard SST state, or infer
that restoring four ZIPs reverses routing/frontend/state/data changes. No data
migration is currently planned, and no migration is authorized here.

## Locally verified evidence and limits

The isolated transport fixture runs real pinned SST/Pulumi plus Go, Node and
Python children against loopback responses with synthetic credentials. All 11
shared attempts reconcile with fixture forwards. Eight cases cover shared
accounting, call exhaustion, continuous campaign and phase deadlines, mutations, endpoint denial,
exact upload success and altered upload rejection; descendant pulse files stop.
17 offline policy/accounting tests, 22 credential regressions, nine path tests,
18 policy/read/diagnostic regressions, 19 driver/authentication-startup regressions
21 checkpoint/claim regressions, 18 fresh-start regressions and 14 frontend checks pass (138 total).
The 12 isolated cases include an actual `phase.mjs diff` + pinned SST/Pulumi
successful bucket preview under the production policy (no observer), a separate
unapproved GetBucketTagging denial before forwarding, plus an
actual old-frontend SDK check that passes before rejecting an unexpected object.
Historical actual-DynamoDB transition evidence remains in the prior review; the
transition core was unchanged by this correction.
The actual-entrypoint context regression also passes with no credentials or
network requests: two synthetic collector replies, read-only context and hidden
campaign/claim/ledger assertions. Its invalid synthetic marker intentionally stops
collection before platform reads; it does not establish a live baseline. `npm run verify` passed (format, lint, application/infrastructure/tooling tests, frontend build and Go tests/vet); dependency fetching was disabled. Python Ruff lint and format checks also passed. The artifact/adoption/source verifier
passes with zero AWS calls.

The startup failure was an import error: mitmproxy's dedicated Python environment
could not see system-installed botocore. The launcher now exposes the existing
system package path; both interpreters are Python 3.14.8. The runner also supplies
private proc/dev mounts and permits the nested provider namespace while that
provider namespace itself blocks further namespaces. Private startup stdout and
stderr are captured with request/response flow logging disabled. Failure logs
survive teardown under mode-0700 `.build/environment-deploy-test-private` with
mode-0600 files. That directory is mounted into the outer fixture namespace and
is absent from the provider sandbox mounts. Logs are private and not published.

Local fixtures do not establish current cloud configuration, absence of live
traffic/drift, actual provider write variants, propagation, or runtime behavior.
Those remain future observations. Unknown variants are held rather than repaired.
Fresh baseline, a newly authorized
initial campaign, phase-specific approvals, the closed window and separate
runtime/rollback allowances remain decisions for review. The expanded ceiling
itself is approved; no current live continuation is authorized.
The prior readiness and runtime validation plans remain applicable; OpenAPI
production scope remains separate.
