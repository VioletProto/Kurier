# Local-agent execution: startup, validation and deployment review

Davian accepted [the contract](../architecture/local-agent-execution-contract.md)
at `585d5b3` on 2026-10-13 and authorized implementation, API-contract
synchronization, meaningful local tests and an AWS preview. Davian subsequently
approved the scoped development deployment and activation, including correction
`14323d8`. Both are now performed in `dev-api`/us-east-2; the live stage is
active/version 7, savedRequests 5 and localAgentExecutions 1. Recovery generation
remains `dev-api-initial-v1`. Code and validation live on
`feat/local-agent-execution`.

## Foreground startup

The [accepted frontend-hosting continuation](frontend-hosting.md) is implemented
and deployed for development/demo at https://d2kpq905qyh9zg.cloudfront.net.
Use that HTTPS UI, or the local UI below, to issue/manage the same local-agent
credential. After initially deferring this check, Davian reported successful
hosted local-agent execution against a local echo endpoint, redacted response
fields and working revocation. This completes the deferred hosting check; see
[operator acceptance](frontend-hosting-validation.md#operator-acceptance--2026-10-19).
A separate private-LAN target and exact target-send count were not individually
reported. Hidden-terminal pairing, outbound HTTPS polling and explicit destination
consent remain unchanged; the browser never contacts a localhost/private-network
control server.

Requires Go 1.26. Build from the checked-out repository root:

```sh
go -C services/local-agent build -o /tmp/kurier-agent .
```

1. Open https://d2kpq905qyh9zg.cloudfront.net (or start the local React UI
   with `npm run dev:cloud`) and sign in through Cognito. Open an owned project's execution panel.
2. Refresh pairing status. Choose the device name, loopback/private address class
   and exact origins for this project, and explicitly approve their scope.
   Create the credential and enter its one-time token into the CLI hidden prompt.
   The browser token display closes without adding it to browser storage.
3. Run against the public development API origin, confirmed from SST outputs:

```sh
/tmp/kurier-agent run --api https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com
```

4. Review the returned scope/expiry and type `yes` to approve those services on
   this machine. The process stays in the foreground and opens no listener.
5. Select Local in React, review the device/origins and approve local execution.
   Sending protected values over HTTP requires its separate unchecked consent.
   Execute the saved revision; inspect history, evidence and pin controls normally.
   Each new execution/rerun requires fresh consent. Unknown outcomes cannot replay.

Prefer an explicit `http://127.0.0.1:3000` origin when a service listens only on
IPv4. `localhost` can resolve IPv6 first; there is deliberately no fallback to a
second address after connection failure. Exact origins are distinct. HTTPS uses
ordinary certificate verification and operator-installed trust; no insecure TLS
flag or automatic certificate installation exists.

Memory-only startup requires entering the token again after process exit. An
optional, deliberate POSIX credential file can be created with:

```sh
/tmp/kurier-agent pair --api https://8dnymkcoa0.execute-api.us-east-2.amazonaws.com --persist
/tmp/kurier-agent run
/tmp/kurier-agent status
/tmp/kurier-agent unpair
```

The file is the private user configuration directory's
`kurier-agent/credential.json`, with directory 0700/file 0600 and owner/symlink/
shared-directory checks. It contains the capability credential and fixed API
origin, never protected request inputs, executable grants or result jobs. The
live scope is fetched and confirmed on every run. Memory-only mode supports
Windows; persistent Windows credentials are intentionally unavailable. Linux,
Windows amd64 and macOS arm64 compilation passed; Linux is the tested runtime.

`unpair` attempts authenticated self-revocation and removes the local file. If
the server is offline or the token no longer authenticates, it reports revocation
unconfirmed and directs the owner to browser Revoke. Never put tokens in arguments,
environment files or shell pipelines. `--stdin` is explicit controlled-test input,
not a suggested shell command containing a token. Ctrl-C cancels best-effort;
it cannot establish that an upstream operation had no effect.

These startup steps now target the deployed development capability. Browser
pairing and operator private-LAN validation remain separate from the SDK-created
disposable identities used in the genuine AWS protocol check.

## Prepared operator localhost and private-LAN checks

The compiled Linux agent is `/tmp/kurier-agent`; rebuild with the command above
on another machine. A disposable foreground target logs no request inputs:

```sh
go -C services/local-agent build -o /tmp/kurier-check-target ./cmd/check-target
/tmp/kurier-check-target --listen 127.0.0.1:3000
```

Use genuine Cognito sign-in in `npm run dev:cloud`, create a disposable project
and GET saved request at `http://127.0.0.1:3000/echo`. Optionally set an enabled
Authorization header as a protected value using a disposable random value.
Pair the browser to exact origin `http://127.0.0.1:3000`, class Loopback, then
start the agent with the hidden one-time token prompt and explicitly confirm its
scope. Submit Local once, including separate insecure-secret consent if needed.
Inspect evidence: successful JSON, authorization/cookie redacted. Read
`http://127.0.0.1:3000/counts`: `targetSends` must be 1. Receipt retries and
history/evidence refreshes must leave it at 1.

For the private-LAN check, run the target on a second controlled machine with an
explicit private address, e.g. `--listen 192.168.1.42:3000`; allow only the agent
machine through that machine's existing firewall. Use its actual address as the
saved URL and pairing origin, choose Private class, and run the compiled agent
on the first machine. Revoke/re-pair if the immutable credential lacks this
origin. Repeat the single submission/count/evidence check. Do not expose this
disposable target publicly; it refuses wildcard/public binds. Stop it with
Ctrl-C afterward.

Optional ambiguity check: save `/ambiguous` on the same origin. This target
records a send and closes the connection before returning a response. Submit
once; inspect the failed/unknown outcome and `/counts` without rerunning. The
count increases once and stays stable through subsequent polls/refreshes. This
is an induced controlled-target fault, not a genuine AWS failure. A new HTTP
send requires an explicit new submission and fresh consent.

Finally Revoke in React, stop the agent, confirm status rejects the old token,
delete the disposable project, and wait for the deletion operation to complete.
Do not capture token displays in screenshots/traces or put tokens in shell
arguments/files. The localhost execution and repeated revocation/re-pairing are
now operator-reported as tested, along with same-machine private-interface
execution. The operator ambiguity/no-replay check and its cleanup are also
completed as recorded below; separate-device LAN remains prepared/unconfirmed.

### Operator-reported results — 2026-10-13

Davian supplied the result from his localhost fixture test:

```text
Content-Length: 47
Content-Type: application/json
Date: Tue, 13 Oct 2026 22:51:25 GMT
Set-Cookie: [REDACTED]
```

```json
{ "authorization": "[REDACTED]", "ok": true, "targetSends": 1 }
```

This records a successful operator localhost execution with one counted target
send and redacted Authorization body field/Set-Cookie header. Content-Length
describes the upstream response; the sanitized body may have a different encoded
length. Davian also reports revoking and re-pairing multiple agents successfully.
These are user-reported runtime results, distinct from automated SDK identities
and controlled browser fixtures. They do not establish private-LAN behavior,
old-token denial on each protocol route, ambiguity handling or final deletion
of that initial fixture; subsequent private-interface evidence and cleanup
readback are recorded below.

Davian subsequently tested the prepared private origin
`http://192.168.1.253:3000` (the machine's `wlan0` address), with the fixture
bound to `192.168.1.253:3000` and the request at `/echo`. He reported:

```json
{ "authorization": "[REDACTED]", "ok": true, "targetSends": 1 }
```

The reported stored response had Content-Type `application/json`, upstream
Content-Length 51, Date `Tue, 13 Oct 2026 23:21:09 GMT`, and Set-Cookie
`[REDACTED]`. His separate `/counts` read also returned `targetSends:1`.
Project: `896767dd-d499-4226-ae5a-45b0df6efa36`; execution:
`86ff6250-6835-472d-a2a0-28bf4537a145`. This establishes an operator-reported
private-address execution/count/redaction check on the same machine, not traffic
to a separate LAN device. The fixture was already stopped during inspection,
so its count was not independently re-read. By then AWS execution/SNAP records
and evidence objects were already removed by deletion; original evidence bytes
could not be independently re-read or checksum-verified after removal.

Deletion `b12b85aa-6dc1-459f-accb-14de2ab67574` belongs to project
`c5585b9a-63be-466a-b59d-9357e90dbb24`, distinct from the private-interface
project. Its durable operation was confirmed completed. Deletion
`e15968fc-8d26-445f-bee3-ae53c0079ac6` belongs to the private-interface
project: initial inspection showed only META/work records and one encrypted
protected source remaining, with no S3 evidence objects. Subsequent read-only
observation confirmed **both deletion operations completed** through scheduled
cleanup, without target HTTP replay or manual deletion. Strong Control queries
returned only each project's deleted tombstone, strong Protected queries returned
zero records, and bounded S3 prefix listings returned zero objects without
truncation. Operator fixture cleanup is verified for both supplied projects.

### Completed operator ambiguity/no-replay check

Davian reports the prepared `/ambiguous` check completed: baseline counter 0,
failed execution with the displayed certainty warning “Outcome unknown — do not
assume no effect. An external operation may have completed. Automatic replay is
disabled.”, and final counter 1 after **4 minutes 20 seconds**. The UI-reported
error code was `unavailable`. Project:
`95b55739-aaa6-419c-b96d-da79a07447ae`; execution:
`a6e69199-3f21-48b6-ae78-6d1d754e641c`. This records one counted target send and
no automatic replay during the observed interval. The report labels the counter
reads `/checks`; the supplied fixture's implemented counter endpoint is `/counts`.

Deletion `aa168dd2-5981-49f1-bdc9-7655d4cdb31d` was independently confirmed
completed in AWS. Strong Control readback returned only the deleted project
tombstone, strong Protected readback returned zero records, and the bounded S3
prefix listing returned zero objects without truncation. Execution/evidence were
already deleted, so the original stored outcome code/bytes could not be
independently verified; `unavailable` remains attributed to the operator's UI
report. No target request was sent during inspection.

Mark this controlled-target connection-loss/no-replay check and fixture cleanup
**completed**, distinct from an induced AWS fault, agent-process crash, offline
recovery or separate-device LAN test. Those unperformed checks remain open.

## Implementation and compatibility

- `services/request-runtime` owns the AWS-independent HTTP preparation, separate
  cloud/local destination checks, capture, redaction, strict JSON and local DTOs.
  API aliases preserve cloud types and the existing Lambda publication-size proof.
  Valid cloud options retain their original serialized admission digest, OUT/SQS
  behavior and public-only 80/443 policy. Local jobs use schema 2; cloud rows stay
  schema 1. Optional local fields omit from legacy DynamoDB maps.
- Pairing stores a random 256-bit token verifier, an account slot and immutable
  scoped credential, private issuance receipt and project cleanup links. Agent
  middleware cannot authenticate browser CRUD. Current metadata includes server
  time for presence; self-revocation is `DELETE /api/v1/local-agent/session`.
  Scope removal on deletion preserves unrelated projects and only matching slots.
- Local admission freezes origin/credential and protected values, with shared
  quotas and source-bound idempotency. GSI2 discovery plus LOCALWAIT expiry
  obligations avoid local SQS dispatch. Only one executable lease occupies an
  agent slot; separately deliberate queued admissions retain their own identities.
- Poll commits intent/lease/fence/nonce before disclosure, then rechecks authority.
  Uncertain commit returns no executable packet, including after readback.
  Crashes, lost grants, duplicate delivery and expiry never restore HTTP eligibility.
- First result publication rechecks credential, user, project, generation and live
  lease at each fenced boundary. One durable candidate/ticket can resume before
  expiry; conflicting candidates cannot replace it. SNAP/RET/accepted receipt,
  terminal state, slot clear and encrypted-input deletion commit together.
  Accepted identical ACK retries consult the receipt before lease checks and do
  no KMS/PUT/counter writes, even after expiry. Revoked/expired/replacement tokens
  cannot acknowledge the old credential's work.
- RFC 8785 canonicalization is pinned to
  [cyberphone/json-canonicalization](https://github.com/cyberphone/json-canonicalization/tree/32a48195b9b2/go)
  `v0.0.0-20261011152517-32a48195b9b2`, with number/UTF-16 ordering vectors.
  Shared strict JSON now rejects malformed UTF-8/unpaired surrogate escapes
  instead of silently replacing them. Private receipt HMAC identifies the
  normalized client packet; immutable evidence checksum identifies final
  sanitized bytes. The existing SSM signing root is preserved, with no rotation
  in this task. A future rotation must retain prior key material while receipts
  and pinned evidence survive; the service supports explicitly retained keys.
- Body omission is frozen before candidate identity/publication if upload, API
  evidence or worst-case plain/base64 Lambda ingress/egress would exceed bounds.
  Defensive server sanitization compares request capture to the frozen plan and
  sanitizes response claims. Device honesty and arbitrary transformed-secret
  detection are not proven by sanitization.

## Local validation evidence

These are simulations/controlled local checks, **not genuine AWS/Cognito/private
LAN or Davian-operated agent validation**. Required checks are run before commit.

| Check                              | Result and boundary                                                                                                                                                                                                                                                                                                                                                                                                            |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Repository verification            | Prettier/gofmt, ESLint, 48 frontend tests, 10 infrastructure tests, TypeScript/Vite build and tests/vet across all six Go modules. API/worker/agent binaries build; the agent also cross-compiles for Windows/macOS.                                                                                                                                                                                                           |
| Shared runtime and CLI race tests  | Cloud regression, local origin/class/IP/rebinding/metadata denials, canonical vectors/unsafe JSON, worst-case encoded ingress, scope/deadline/incomplete grant rejection before HTTP, identical upload retry with exactly one target send, TLS transport and private-file permission/symlink checks.                                                                                                                           |
| DynamoDB Local full API race suite | Real Local transactions; simulated KMS/S3/SQS and injected lost commit/PUT/ACK responses. Cloud/protected/pinning regressions remain included.                                                                                                                                                                                                                                                                                 |
| Local protocol faults              | Ownership, single active credential, issuance retry without token redisclosure, source/target isolation, concurrent grants, lost/truncated grant semantics, crashes before/after HTTP, offline queued expiry, credential revocation during PUT/publication, generation fencing, pending candidate resume/conflicts, post-expiry accepted ACK, secret containment and deletion races. Assertions count target HTTP and S3 PUTs. |
| HTTP/ingress boundaries            | Agent bearer cannot use Cognito CRUD; browser JWT cannot poll/session; foreign revoke denied; unknown credential fields preserved. Plain/base64 4 MiB ingress accepted at adapter boundary, one-over/escaping event overflow and ordinary-route uploads denied before handler.                                                                                                                                                 |
| Chromium suite                     | Four scenarios through real Go/DynamoDB Local: project/secret flow, secret audit, pinning/expiry/deletion and local pairing/execution. New scenario builds/runs the foreground CLI against a verified TLS API fixture using a test CA, executes a controlled localhost service, checks redacted evidence/storage, revocation and completed project deletion.                                                                   |
| External contract synchronization  | Existing native API document's local section and obsolete pairing/result proposals updated and read back; neighboring queue/MCP text and structure preserved. Acceptance is explicit, runtime evidence is separate.                                                                                                                                                                                                            |

Commands (DynamoDB Local must be running):

Sanitized response text uses a separate 4 MiB defensive parsing ceiling: JSON
escaping can expand a valid response beyond the unchanged 2 MiB wire and decoded
read limits. Complete result, evidence object and browser envelopes remain
independently capped at 4 MiB, with the existing 6 MiB Lambda transport checks.
The expansion regression runs agent capture/result construction, strict server
validation and immutable publication through DynamoDB Local and simulated S3/KMS;
it covers retained expanded JSON, overflow omission and identical receipt retries
with exactly one target HTTP send and one object PUT per execution.

```sh
docker compose -f compose.dynamodb.yml up -d
npm run verify
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:browser:integration
go -C services/request-runtime test -race ./...
go -C services/local-agent test -race ./...
```

Fixture JWTs/KMS keys and S3 objects are test doubles; the agent subprocess's
certificate trust file contains only the public fixture CA. Tokens pass through
process stdin, never argv/files. Playwright trace, screenshot and video are off;
failure page snapshots are disabled to avoid persisting token displays.
Containment checks report booleans/counts, never actual credential values. Sandbox
socket restrictions require an authorized outside-sandbox test run; that is an
execution-environment constraint, not an AWS validation result.

## Actual deployment and activation — 2026-10-13

Approved scope was re-previewed and deployed to account `747336059622` as
non-root `davian-admin`, us-east-2, stage `dev-api`, including `14323d8`.
Only seven existing-API routes/integrations/permissions, scoped API permissions
and four existing code assets/consumer environments changed. No new standing
resources, bootstrap, frontend hosting, Cognito or account settings changed.

`node scripts/verify-local-agent-deployment.mjs` checks archive equality,
readiness/runtime/ARM64, local support on API/maintenance/worker, capability 5
support on API/maintenance, seven payload-v2 route targets, scoped API S3 PUT
and stage/purpose-bound job-binding KMS decrypt, maintenance without decrypt,
and the enabled minute cleanup target. The S3 grant reuses the existing
`dev-api/projects/*` object prefix; evidence keys are restricted by the protocol.

| Deployed asset                      | SHA-256, base64                                |
| ----------------------------------- | ---------------------------------------------- |
| API and cleanup                     | `tkaKRqHYPYBV1yl97Qfy45n6pfTcbq2h5eGLet2tkXk=` |
| Worker and controlled cloud fixture | `7ocqkj9O5JwcZvmxzNCgVi2mQYlHdfnScpAX3zMoz7A=` |

Guarded activation used `node scripts/activate-local-agent-executions.mjs
--activate`. Initial inventory rejected six retained **published** upload
tickets before any marker mutation. Strong readback confirmed active/version 5
and capability 4. Each published ticket was then checked against a settled
write, terminal job and matching immutable SNAP metadata; no record was reset,
removed or replayed. Activation conditionally closed the stage at version 6,
re-inventoried while closed, and activated at version 7. Strong readback:
active, savedRequests 5, localAgentExecutions 1, pinning/cloud/inputs/protected 1,
same `dev-api-initial-v1` generation. Existing project/quota accounting was
fingerprinted while closed and preserved. Only marker state/version/capability
attributes changed. The script rejects unexpected or already-closed markers;
failed/uncertain operations read durable state and never automatically reopen.

Free plan remains ACTIVE, account and unreserved concurrency 10; SQS remains the
only worker trigger, standard pollers, max concurrency 2/batch 1, with no reserved
or provisioned capacity. The existing incremental cost forecast below is
unchanged; measured costs are not claimed.

Genuine AWS protocol checks use `node scripts/aws-integration.mjs TestAWSLocalAgent`
and `TestAWSCloudExecution`. Disposable identities, pairing and admissions are
created through the SDK, **not browser/Cognito pairing**. Agent session/poll/result
and self-revocation traverse the real public HTTPS API and deployed Lambda IAM.
They verify invalid credentials and browser-CRUD isolation, actual KMS frozen
input delivery, one controlled loopback HTTP send, sanitized JSON expansion
beyond 2 MiB, actual S3 immutable publication, identical ACK retry, and revoked
session/poll/upload denial. Cloud regression covers actual KMS/DynamoDB/SQS/
worker/HTTPS/S3 and duplicate handling, credential-name sanitization and
disposable-fixture cleanup. Crashes/lost responses remain simulation evidence;
Separate-device LAN and elapsed expiry checks remain operator work. Davian's
subsequent localhost/private-interface execution, revocation/re-pairing and
verified operator fixture cleanup results are recorded above.

Both AWS suites passed. The local fixture's project deletion completed through
the actual enabled minute schedule in about four minutes, without manually
invoking maintenance or its drain. Strong Control/Protected partition reads
proved only the project tombstone remained; S3 prefix listing was empty. Local
job, retained receipt, frozen binding and scope obligations were removed.
The test then removed only its own revoked disposable credential/account lookup
and issuance/rate records after drain completion, with strong absence readback.
These credential records have no automatic TTL; project deletion removes scope
authority, while revoked metadata can otherwise remain retained. Cloud fixtures
also verified physical cleanup. Public project IDs for the completed checks:
`880d82de-1b8f-4fb6-8714-bf40b54f7f4f` (local),
`1cae4b48-4f5b-47eb-8d65-99b7f396aca5` (cloud),
`101f8e67-9ef4-49c6-9cde-564a3ff3c364` (credential names).

No genuine first-upload retry after lease expiry, thirty-day expiry, LAN/offline,
PITR restore or induced AWS lost-response/crash behavior is claimed. The local
AWS result ran the shared agent capture/result runtime with an HTTPS protocol
client; Davian subsequently reported successful localhost execution and repeated
revocation/re-pairing through the prepared operator workflow. The cloud suite audited safe
logs/evidence for its own disposable values; local AWS assertions cover frozen
delivery and published evidence, not a separate exhaustive log audit.

## Historical approved deployment and activation preview

Read-only STS confirmed account `747336059622`, non-root `davian-admin`, region
us-east-2. `scripts/verify-cloud-execution-config.mjs` confirmed active Free plan,
account/unreserved concurrency ten, SQS as the only configured worker trigger,
standard pollers, batch one/max concurrency two, no reserved/provisioned capacity.
No plan, quota or concurrency settings are modified in the implementation/diff.

The authorized SST diff reuses the existing bootstrap/resources and plans:

- Seven routes with their existing-API Lambda integrations/invocation permissions:
  browser POST/current GET/revoke DELETE, agent session GET/self DELETE, scoped
  poll POST and scoped result POST. Authentication remains enforced by the Go
  middleware; agent routes do not acquire browser CRUD authority.
- Existing API role adds evidence-prefix S3 PutObject and purpose-restricted
  job-bindings KMS Decrypt. Existing saved-secret scope remains. Maintenance still
  has no KMS decrypt, and worker policy/concurrency/queue triggers are unchanged.
- Code and capability-support environment updates on existing API, cleanup and
  worker; shared worker source also refreshes the existing controlled fixture
  code asset. API/cleanup saved-request support environment becomes 5, local
  support marker 1 is added. This **does not activate** the stored stage marker.
- No new table/index/queue/bucket/key/standing compute, Cognito/email changes,
  hosting or bootstrap changes. Versioned code assets incur ordinary shared
  bootstrap storage; the actual resource diff is checked before deployment.

Read-only `node scripts/preview-local-agent-activation.mjs` has no mutation mode.
The pre-deployment inventory returned five projects, zero queued/running cloud
jobs and zero local records. The original marker was active/version 5,
savedRequests 4/pinning 1 with generation `dev-api-initial-v1`. The reviewed
activation sequence was:

1. Deploy only the approved dev-api changes; verify all consumer package hashes,
   environment markers/readiness, routes and scoped permissions against the
   built artifacts. Re-read Free plan/quota/concurrency.
2. Conditionally close active stage to `local-agent-migration` using exact
   version/generation and capability 4/pinning 1/local absent. Do not reset data,
   counters or generation. Closed guards fence submissions, workers and cleanup.
3. Re-inventory while closed, verify compatibility/no unexpected local records
   and settled writer obligations, then conditionally activate savedRequests 5
   and localAgentExecutions 1 with the same generation and unchanged counters.
   Marker state/version changes are the only data mutation in activation.
4. Read back and perform separately authorized runtime validation. Any failed
   or uncertain activation leaves the stage closed for reviewed reconciliation;
   no automatic reopen, generation reset or replay of restored HTTP.

## Refreshed development cost forecast

Official Ohio regional catalogs were downloaded on 2026-10-13:
[Lambda](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSLambda/current/us-east-2/index.json),
[API Gateway](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonApiGateway/current/us-east-2/index.json),
[DynamoDB](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonDynamoDB/current/us-east-2/index.json),
[S3](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/us-east-2/index.json),
[KMS](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/awskms/current/us-east-2/index.json).
Catalog publication dates are October 12–13 for compute/API/S3 and September 11
for DDB/KMS. No credits or eligibility-based free tiers are subtracted.

Assume one agent/one visible browser pairing panel, 4h/day ×22 days, ten-second
polling: 31,680 agent polls plus up to 31,680 pairing-status reads. Allow 70k
hosted billing units/month including uploads, views and retries (HTTP API uploads
bill in 512 KiB units). 1,000 local executions fit **within** the existing shared
monthly 1,000 quota, replacing some cloud work, with 100 KiB average evidence and
1,000 extra views. Conservatively do not subtract cloud worker/SQS savings.

| Increment                                                                                   | Monthly forecast   |
| ------------------------------------------------------------------------------------------- | ------------------ |
| 70k HTTP API units at $1/million                                                            | $0.070             |
| 70k ARM 256-MiB/100ms calls: 1,750 GB-s at $0.0000133334 plus $0.20/million requests        | $0.037             |
| 800k RRU at $0.125/million +100k WRU at $0.625/million, including transaction/item rounding | $0.163             |
| 0.1 GB extra metadata ×($0.25 storage+$0.20 PITR+$0.10 on-demand backup)/GB-month           | $0.055             |
| <=3,000 incremental KMS operations at $0.03/10k                                             | $0.009             |
| 0.1 GB S3 at $0.023/GB-month, bounded PUT $0.005/1k and GET $0.0004/1k                      | $0.010             |
| <=1 billed GB extra delivery at assumed $0.09/GB                                            | $0.090             |
| Safe logs, code assets and extra reconciliation allowance                                   | $0.100             |
| **Rounded additional planning allowance**                                                   | **$0.75–$1/month** |

Request-unit multipliers and delivery assumptions follow the official
[API Gateway examples](https://aws.amazon.com/api-gateway/pricing/). This is a
forecast, not measured spend or a hard cap. Ten scoped projects multiply agent
base polling by ten; all-day one-project polling is 259,200/month. Max-size views,
retries, rejected calls, accumulated pins and key rotations can exceed this light
allowance. Existing key/bootstrap/maintenance fixed costs are not charged again.
No account plan change or capacity reservation is part of the estimate.

## Remaining validation and operational gaps

- Davian's localhost execution and repeated revocation/re-pairing are recorded
  as operator-tested, including same-machine private-interface execution.
  Separate-device LAN, real device offline recovery, thirty-day elapsed
  credential expiry and clock skew remain unconfirmed. Automated CLI/TLS/browser
  fixtures remain controlled local evidence.
- Crashes and lost responses are injected simulations, not induced AWS faults.
  An uncertain S3 PUT keeps its ticket/reservation inaccessible and can keep
  physical deletion pending indefinitely until reviewed settlement; absence
  alone does not release it. Normal acknowledged PUT/deletion is tested.
- Actual PITR/closed-stage restore/counter reconciliation, negative IAM runtime
  checks, metered costs and root-signing-key rotation remain unperformed.
  Generation-fencing simulations do not prove an operational restore procedure.
- Windows/macOS runtime/private credential storage behavior was not exercised;
  only cross-compilation is claimed. No installer/keyring/autoupdate/service is
  supplied. Arbitrary secret transformations and device honesty remain explicit
  trust limitations, not sanitization guarantees.

Trello keeps implementation/synchronization separate from deployment/genuine
validation; the full slice card remains Doing while those later obligations are
open. No additional workflows, MCP or hosting behavior is implemented.

Known metadata endpoints remain denied even inside ULA: the AWS reserved
`fd00:ec2::/32` range and GCP `fd20:ce::254`, alongside IPv4 link-local metadata.
Primary references: [EC2 IMDS IPv6](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html)
and [Compute metadata endpoints](https://cloud.google.com/compute/docs/metadata/querying-metadata).
This supplements the private-class checks, including at DNS resolution/dial.
