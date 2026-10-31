# Project environments provider-process preview meter

Prepared for [PR #18](https://github.com/VioletProto/Kurier/pull/18), continuing
inventory packet `cf99f52ab168c38c72ad7f42a2d3589458223634`. **Local implementation
and fixtures only. No AWS calls or live preview in this task.** Release source
remains `44f1fb79d943501c95e06fcc1a6114111a7c1ea4`.

The [inventory/artifact packet](project-environments-inventory.md) and
[original deployment review](project-environments-deployment-review.md) retain
the live findings, source/artifact pins, compatibility and rollback limits.
This closes the local provider-metering preparation gap, not the preview hold.
Deployment, closure, migration, activation, identity/project creation and runtime
requests remain held. OpenAPI production scope remains separate.

## Implementation and isolation boundary

The [launcher](../../scripts/preview-meter/run.py) has only `check` and `preview`
actions. Its preview command is fixed to the pinned SST binary and
`diff --stage dev-api --json`; it has no deploy, refresh, unlock, repair, activation
or arbitrary-command interface. It verifies the frozen parent ledger, clean
release checkout, clean reviewed tooling head, SST/platform/provider pins and
retained release/rollback bytes before starting.

Every SST descendant, including Pulumi, Go providers, Node SDKs and RPC workers,
inherits a bubblewrap sandbox with private network, PID, user, IPC, UTS and
cgroup namespaces, dropped capabilities and further user namespaces disabled.
Its root exposes selected read-only OS libraries, pinned source, Git metadata,
cached dependencies/tool binaries and meter adapters. It exposes neither host
`/run`/Docker/SSH sockets, host `/proc`, credential caches, nor the host meter's
ledger/configuration/private CA. It inherits no network file descriptor.
Writable tooling/build/dist/cache paths are private overlays; original source,
release artifacts and rollback bytes remain outside writable mounts.

The only network escape is a read-only mounted Unix relay socket to the
host-side HTTPS proxy. The child sees its own loopback proxy; its internal
Pulumi RPC remains local and does not consume AWS calls. Clearing proxy variables,
using direct IPv4/IPv6/UDP sockets or spawning another runtime cannot add an
external route. Nested namespace creation is denied. Host-service Unix socket
paths and the live ledger are absent. Node's SDK/custom HTTP agents use the
existing [forced proxy adapter](../../scripts/attempt-meter/node-proxy.mjs);
the network boundary still prevents a bypass if a provider ignores that adapter.

The host proxy has lazy upstream connections, upstream TLS verification and
HTTP/1.1 plus HTTP/2 interception. Its
[closed policy and budget](../../scripts/preview-meter/policy.py) and
[proxy gate](../../scripts/preview-meter/proxy.py) classify **each HTTP request**,
not each process, command, connection or SDK operation. Before forwarding, one
host-owned counter appends and fsyncs an attempt receipt. Concurrent clients,
HTTP/2 streams, SDK retries and pagination share that counter and one monotonic
clock. The child cannot modify or reset it.

The ledger records only timestamps, count, fixed service/operation, HTTP version,
status, transport failure and fixed stop reason. No request URLs, query strings,
headers, bodies, tokens, passphrases or decrypted state are logged there. Raw SST
output and generated preview/state remain private local tooling output, never
an automatically published report. The temporary interception CA is removed at
exit; stage credentials remain in process environment, not a copied cache file.

## Read-only policy and failure behavior

Only enumerated Ohio AWS control endpoints, known stage S3 buckets and necessary
global IAM/STS/CloudFront endpoints are recognized. JSON `X-Amz-Target` and Query
`Action` operations use closed read allowlists; duplicate/conflicting protocol
dispatch rejects. REST APIs permit GET/HEAD only. SSM value reads are restricted
to the existing bootstrap, dev-api passphrase and cursor-key parameter names.
Unknown endpoints/protocols/operations stop, rather than becoming broad
"Get/List/Describe" permissions. Application API, hosted frontend, fixture and
Lambda Function URL hosts are not permitted targets.

All metadata/application/infrastructure writes remain denied, including:

- SSM PutParameter for `/sst/bootstrap` or `/sst/passphrase/kurier/dev-api`,
  including missing-prerequisite repair or passphrase creation;
- S3 PUT/POST/DELETE for stage state, lock/update/snapshot/eventlog/history,
  bootstrap policies or cleanup, package uploads, static release/archive files;
- stage/quota/application DynamoDB writes or transactions, table/index changes;
- IAM/STS session or role creation/assumption, credential refresh,
  Cognito identity/project lifecycle, Lambda invocation/code/config changes;
- CloudFront KV changes, invalidations and any other non-read resource operation.

There is no write phase switch. Missing bootstrap/cache/provider prerequisites
must stop; dependency download endpoints are denied and npm/Go/plugin acquisition
is disabled. The exact earlier
[conditional metadata-write scopes](project-environments-inventory.md#sst-preview-preflight-and-exact-metadata-write-boundary)
are not authorized by preparing this meter.

A denied write or unknown endpoint latches the shared stop **before** forwarding.
A 3xx response is retained as the original outcome, replaced for the child with
a local 599, and latches stop before any redirect follow. An upstream failed
transport is charged once and latches stop; there is no replay to establish its
outcome. HTTP 503/429 SDK retries can proceed as separately charged attempts
within the same bounds. Authorization 401/403 stops, with no refresh fallback.

The proxy clock also runs while clients are idle. At deadline or exhaustion the
supervisor terminates the sandbox and its descendants. Proxy/relay death,
ledger/parser failure or missing outcomes cannot produce a successful result;
persistence failure kills the proxy before forwarding. Already charged reads
are reconciled by their outcomes, never repeated automatically. A stopped ledger
or failed setup has no resume/retry mode. A different output directory is not
permission to restart the campaign or reset its usage.

## Local evidence

Run from the repository root:

```sh
bash scripts/preview-meter/test.sh
python3 -B scripts/preview-meter/run.py check
```

The transport suite wraps **the fixture supervisor, origin and proxy themselves**
in an outer namespace without an external network route. Its children then use
the production sandbox. Only fake credentials and synthetic AWS replies are
supplied. The actual installed SST binary runs an empty fixture project, backed
by local fake SSM/S3/STS, and starts the actual Pulumi AWS provider. This is a
local fixture diff, not a preview of live dev-api or the production resource graph.

[Machine test receipt](project-environments-preview-meter-tests.json) records
binary/tool versions, tested code hashes and these passing cases:

| Case                         | Fixture result                                                                                                                                                                                                                                             |
| ---------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Real SST/Pulumi/Go/Node tree | 14 upstream requests share cumulative counts136–149: Go ListTables uses503 retry plus two pages (3); Node S3 uses503 retry plus two pages (3); SST/Pulumi make the remaining8 reads. Both HTTP/1.1 and HTTP/2 observed; origin count equals charged count. |
| Bypass attempts              | Direct IPv4/IPv6, nested namespaces, host Unix socket/cache/ledger access fail; legitimate local Pulumi RPC succeeds.                                                                                                                                      |
| Mutation boundary            | Actual SSM passphrase PutParameter, stage-state S3 PUT and DynamoDB transaction each forward0 requests and latch stop. Pure policy vectors also cover IAM, token issuance, Lambda invocation, KV/invalidation and application routes.                      |
| Exact remaining allowance    | Of366 requested reads, exactly365 forward; final cumulative count500, with the next request held and sandbox stopped.                                                                                                                                      |
| Concurrent descendants       | Two Node children race reads against a four-call fixture allowance; exactly4 forward across both processes.                                                                                                                                                |
| Idle deadline/teardown       | One-second fixture window stops without a request; a previously running grandchild heartbeat stops and stays stopped after return.                                                                                                                         |
| Redirect                     | One302 is charged; redirected destination receives0 calls; no follow.                                                                                                                                                                                      |
| Failed transport             | Origin receives one request then drops the response; one attempt/outcome failure, latched stop, no proxy replay.                                                                                                                                           |
| Persistence failure          | Closed ledger descriptor causes proxy termination, zero upstream calls, and overall failure even if a child catches its network error.                                                                                                                     |

Four unit tests and ten transport scenarios passed. `npm run verify` passed
format/gofmt, lint, web/infra/tooling/metering tests, web build and Go tests/vet/
spike test builds; ordinary Go tests may reuse cache. Python lint/format checks
passed. The new provider fixture suite is explicitly invoked above; it is not
silently claimed as an existing CI job.

This demonstrates the installed runtime transport boundary locally. It does not
establish current AWS state, effective IAM, a successful full dev-api preview,
preview call demand, or that every future provider read is in the allowlist.
A new endpoint/read failure must stop for review; do not widen policy live.

## Preserved release and rollback artifacts

No production packaging was run or changed in this task. Before/after checks
verified all prepared ZIPs/bootstraps, foreground clients, frontend files,
retrieved rollback ZIPs/bootstraps and retained CloudFront function bytes against
the existing manifests. Original source checkout remains clean at the selected
release; tooling changes are outside that checkout.

The unchanged release-manifest SHA-256 is
`94d658f4cdae67103c755e3cf5578a50252f8f780c2df82c46c2c2cf18f8c6c5`.
The earlier [preparation checksums](project-environments-preparation.sha256)
still pass, including the frozen135-call ledger. No new artifact hashes need
acceptance now. A later approved preview writes its packages into private
scratch, compares ZIP/frontend hashes to the prepared release, and emits
`packaging-review.json`. Any changed packaging hash requires review; matching
source never authorizes substituting new bytes. The launcher rechecks retained
release and rollback bytes after preview too.

## Proposed next window and exact command

**Proposal only:** one new60-minute window, starting when the shared proxy budget
is created, with at most **365 additional physical AWS attempts**. Prior usage135
is immutable; cumulative ceiling stays500. All provider reads, SDK retries,
pages, redirects and failed transports count. Locally held requests consume no
AWS attempt because nothing is forwarded. Fixture counts above are synthetic
and do not consume the live allowance. The stopped inventory window is not
reopened and its clock is not reused.

Retain the **same cumulative $1 procedural allowance**, not an additional dollar.
Earlier spend and billing delay remain unknown; no measured cost or unused
balance is asserted. No plan/quota/concurrency increase. Existing valid cached
credentials are read locally into memory; no CLI/provider refresh chain is run.
They must have at least120seconds validity at launch. Expiry/authorization denial
may end the campaign early; refresh would need separate authorization.

After explicit approval of the clean PR #18 head, the proposed command from
`/home/king/Neumont/Kurier` is:

```sh
python3 -B scripts/preview-meter/run.py preview \
  --review-commit "$(git rev-parse HEAD)" \
  --source /tmp/kurier-environments-release-source \
  --output /home/king/Neumont/Kurier/.build/project-environments-preview/review-1
```

The PR description pins the concrete reviewed-tooling commit for that command.
The source directory must be clean at44f1fb7; the output directory must be new
and outside child mounts. The source/dependency caches must already exist.
`--review-commit` checks integrity; it is not proof of user authorization.
This command was **not executed** here. Only local fixtures and the `check`
action ran.

After success or failure, stop, freeze the new ledger/outcomes, report cumulative
135+actual new calls and any packaging differences, and publish the sanitized
preview result for review. No follow-up inventory beyond that approved window,
automatic retry, metadata repair, closure, deployment, activation or runtime
campaign follows a successful preview.

Next decision: review this implementation/evidence, then explicitly authorize
that new bounded read-only preview window. All existing deployment and runtime
holds remain.
