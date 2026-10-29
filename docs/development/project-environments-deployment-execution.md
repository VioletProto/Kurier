# Project environments: adopted artifacts and execution review

This packet adopts the exact review-8 ZIP candidates in
[the adopted manifest](project-environments-adopted-release.json), supported by
[binding attestation cb74c992](project-environments-key-observation.md).
The original prepared manifest and historical candidate manifest remain intact.
Release source is **44f1fb79d943501c95e06fcc1a6114111a7c1ea4**. Adoption accepts
artifacts only. Deployment, closure, migration, activation, identities/projects,
and target/runtime execution require further authorization.

Accounting remains **299/500**, zero new AWS calls for this implementation.
The remaining 201 calls are arithmetic headroom, not an accepted deployment
estimate. The same cumulative **$1 procedural allowance** remains; billing is
unmeasured. No automatic continuation is authorized by this packet.

## Artifacts and byte enforcement

The new manifest links the prior candidate manifest SHA and binding-attestation
SHA/commit. API/cleanup ZIP SHA256 is
`58f2bbf364042ac209d4f9b3a4d6ca583da18914fb5851f0c166d34b9c757373`;
worker/controlled endpoint ZIP SHA256 is
`82af7beeda4b619afa6831b342c693bb1fb3e8d08e5e6c0416ce253526f59098`.
No ZIP was regenerated or modified during this task. Historical rollback bytes
and evidence are checked by the existing preparation proof verifier.

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
held before forwarding. The prior read-only preview policy is unchanged.

Fresh collection compares the deployed configuration/code/IAM/generation and
routing to the accepted inventory. It scans only projected metadata, checks
queue/residual quiescence and frontend metadata, and reads the marker before
and after collection. A fresh SST diff must have the same reviewed projection
as review-8 (60 entries, 44 unique resources) and the same four generated ZIPs.
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

## Proposed future execution and allowances

Proposal only: one **55-minute** shared monotonic window, starting before
refresh/STS, at most **1,500 additional physical attempts**, proposed cumulative
ceiling **1,799**, retaining prior 299. This requires explicit approval to expand
the current 500 ceiling. The closed-stage subwindow is at most **30 minutes**,
bounded by the original deadline. The $1 cumulative procedural allowance is
unchanged; the call cap is a stop bound rather than a measured prediction of
cost or sufficient deployment capacity. Zero runtime requests, new identities,
new projects or runtime fixtures are included.

Use clean tooling commit **c5d794b4e3c3afacab7c478fd4fdeab84ff4a886** from this PR and the clean detached
release worktree at the source above. Required caches are SST 4.17.1, pinned
Pulumi AWS provider 7.40.0, reviewed SDK/models and existing dependencies.
`driver.py check` verifies tool, source, attestation and retained artifact pins.
No install/download/repair is part of execution. The same existing non-root
session must cover the deadline after at most one approved refresh. STS identity
verification is mandatory. Authentication and providers share one durable ledger
and deadline; credential expiry or cache replacement stops later phases.

Use the original workspace so retained cache links keep their reviewed paths.
Require a clean working tree before selecting the exact tooling commit; preserve
the review branch and do not reset or remove any evidence:

```sh
cd /home/king/Neumont/Kurier
git switch --detach c5d794b4e3c3afacab7c478fd4fdeab84ff4a886
```

Run `check` there with the existing `.build`, `.sst` and `node_modules` caches.
Do not rebuild adopted archives or install/download dependencies. Keep the clean
release source worktree at its pinned `/tmp` location. Any cache/source/hash
mismatch stops for review.

Exact commands below are proposals. Do not run `start` or `phase` until the
corresponding authorization has been granted. Select a fresh campaign directory
once; never reuse a failed claim or remove the window claim to restart.

```sh
python3 -B scripts/environment-deploy/driver.py check
mkdir -p .build/project-environments-execution
chmod 700 .build/project-environments-execution
python3 -B scripts/environment-deploy/driver.py template > .build/project-environments-execution/authorization.json
```

The template is deliberately `authorized: false`. Review every field and approve
it only after explicit expanded-budget execution authorization. It binds the
current exact clean commit and manifest/scope hashes. Then the sole start command:

```sh
python3 -B scripts/environment-deploy/driver.py start --campaign .build/project-environments-execution/release-from-299 --authorization .build/project-environments-execution/authorization.json
```

Review the fresh collection, complete diff, generated hashes and ledger before
closure. For each phase, substitute exactly one of `close`, `deploy`, `verify`,
`activate`, `final` for PHASE below. Run prepare, review the bound proof and
obtain separate authorization before changing only `authorized` to true.
Do not script a phase loop or approve all phases in advance.

```sh
python3 -B scripts/environment-deploy/driver.py prepare --campaign .build/project-environments-execution/release-from-299 --phase PHASE > .build/project-environments-execution/PHASE-approval.json
python3 -B scripts/environment-deploy/driver.py phase --campaign .build/project-environments-execution/release-from-299 --phase PHASE --approval .build/project-environments-execution/PHASE-approval.json
python3 -B scripts/environment-deploy/driver.py report --campaign .build/project-environments-execution/release-from-299
```

Deploy first warms hashed frontend assets and their KVS entries, then runs the
pinned SST deployment under the byte gate while the stage remains closed.
Verification independently checks deployed code/platform, exact new routing and
permissions, frontend and closed marker. Activation is a separate guarded CAS;
final checks the active tuple, platform and accounting projection. Neither
verification nor final sends API target requests.

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
shared attempts reconcile with fixture forwards. Seven cases cover shared
accounting, call exhaustion, continuous deadline, mutations, endpoint denial,
exact upload success and altered upload rejection; descendant pulse files stop.
17 offline policy/accounting tests pass. `npm run verify` passed (format, lint, application/infrastructure/tooling tests, frontend build and Go tests/vet); dependency fetching was disabled. Python Ruff lint and format checks also passed. The artifact/adoption/source verifier
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
Fresh baseline, expanded call authorization, phase-specific approvals, the closed
window and separate runtime/rollback allowances remain decisions for review.
The prior readiness and runtime validation plans remain applicable; OpenAPI
production scope remains separate.
