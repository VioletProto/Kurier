# W1 review decisions, costs and contract gaps

Status: serverless revision direction authorized; unresolved settings **proposed
for Davian Hernandez's review**, 2026-10-05. See [system design](w1-system-design.md),
[DynamoDB model](w1-data-model.md) and [proposed ADR 0002](../decisions/0002-serverless-persistence-topology.md).
Architecture remains pending review; API-contract card remains open. No Google
Docs/Trello synchronization, deployment or product implementation is authorized.

## Decision sheet

| Choice            | Recommendation/status                                                                 | Reason                                              | Material tradeoff                                                           |
| ----------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------- | --------------------------------------------------------------------------- |
| Infrastructure    | SST 4 **accepted**, ADR 0001 unchanged                                                | Historical spikes support framework selection       | Bootstrap/state costs and reviewed deployment IAM remain                    |
| Revised direction | Go ARM Lambda + HTTP API + SQS + DynamoDB + private S3; direction approved for draft  | Removes always-on compute/network/database baseline | New persistence and invocation behavior untested; no stable outbound IP     |
| Tables/indexes    | Proposed Control/Protected, Standard on-demand, three sparse KEYS_ONLY Control GSIs   | Isolates crypto store; explicit access patterns     | No SQL FKs/joins; index lag/read amplification; gate contention             |
| Integrity         | Proposed gate CAS + bounded cross-table transactions + immutable S3 manifests/tickets | Preserves reviewed atomic state boundaries          | Objects cannot join a DynamoDB transaction; orphan/deletion reconciliation  |
| Status delivery   | Proposed immediate polling, ten-second light default/two-second active option         | Avoids Lambda billing for held connections          | Notifications delayed; replaces draft SSE contract                          |
| Local dispatch    | Proposed intent before poll response, 30-second start/120-second lease                | Durable knowledge of possible HTTP dispatch         | Lost poll can produce conservative unknown; no replay                       |
| Local receipt     | Proposed retained canonical private HMAC/lease identity                               | ACK retries after successful commit/expired lease   | Key retention; credential revocation still denies ACK                       |
| Workflow          | Proposed sequential 2–10 steps, one atomic finalization/successor transaction         | Bounds action/byte limits                           | No branching/loops; oversized input rejected                                |
| Deletion          | Proposed immediate access denial + 202 resumable operation                            | Honest cross-store cleanup and late-PUT handling    | Changes DELETE 204 contract; physical completion may await uncertain writes |
| Secrets           | Proposed per-stage KMS AES-GCM envelopes, no reveal                                   | Authorized reuse without evidence disclosure        | KMS outages/cost/rotation; local device receives required runtime values    |
| Retention         | Agreed 30-day unpinned; pins until project deletion. Conditional protocol proposed    | Metadata independent of immutable evidence          | No native TTL for authoritative history; pinned growth requires quotas      |
| Auth              | Cognito and shared-theme screens **agreed**; Lite/15-minute tokens proposed           | Keep login credentials outside app store            | Offline JWT revocation delay and email-flow checks                          |

## Monthly development cost estimate

Official prices checked **2026-10-05**, USD, **us-east-2**, US CloudFront viewers.
No AWS credentials/account billing queries or mutations. Ohio Lambda/request/
DynamoDB/HTTP API rates were read from official regional catalogs during the
preceding investigation; official service pages/technical constraints were
rechecked for this revision. No credits, account free-tier allowances, savings
plans, reservations or discounts subtracted. Built-in no-charge service features
are distinguished from eligibility-based free tiers. No deployments exist to
measure this product workload: all usage below is an explicit forecast.

Retain the earlier **approximately $3.10/month** light screening estimate.
Detailed accounting including always-on scheduled maintenance and bootstrap
allowances refines it to **$3.17/month**. This is neither actual billed usage nor
a $5 hard cap. Heavy use is **$14.20/month**, including one KMS rotation.

### Workload and storage assumptions

| Assumption                                 | Light development                                                                                       | Heavier development                                                                                                                                                    |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Active users                               | 2 Cognito Lite MAUs                                                                                     | 5 MAUs                                                                                                                                                                 |
| Cloud executions, including workflow steps | 1,000/month; average billed worker 5 s at 512 MiB                                                       | 10,000/month; average billed worker 30 s at 512 MiB                                                                                                                    |
| API calls                                  | 80,000 at 50 ms/256 MiB                                                                                 | 600,000 at 100 ms/256 MiB                                                                                                                                              |
| Polling                                    | One browser + one agent, four h/day × 22 days, ten-second base; <=63,360 base polls, other calls in 80k | Two browsers + one agent, eight h/day × 22 days: two-second active browser/ten-second agent, <=696,960 if continuously active; backoff/stops modeled to fit 600k total |
| Maintenance/outbox/cleanup                 | 50,000 invocations at 20 ms/256 MiB, including empty schedules                                          | 50,000 at 100 ms/256 MiB; batched retries/cleanup work                                                                                                                 |
| Replicas/operating hours                   | No allocated replicas/provisioned concurrency; stage available 730 h; worker max concurrency 2          | Same; enough total capacity but not a benchmark                                                                                                                        |
| Worker body                                | Average 10 KiB request/100 KiB sanitized response                                                       | Average 100 KiB request/1 MiB sanitized response; cap 2 MiB                                                                                                            |
| Average DDB base/index storage             | 0.07 GB Control+Protected / 0.03 GB all three indexes                                                   | 0.6 GB base / 0.4 GB indexes                                                                                                                                           |
| S3 evidence retained                       | Approximately 0.1 GB 30-day body population, minimal pins                                               | 20 GB total: about 10 GB rolling bodies + 10 GB accumulated pins/orphans/import allowance                                                                              |
| Other S3 storage                           | 0.2 GB static assets + versioned SST state/assets                                                       | 1 GB static/bootstrap versions/assets                                                                                                                                  |
| Backups                                    | Seven-day PITR on base tables + one short-lived on-demand backup totaling 0.07 GB-month                 | Same with 0.6 GB-month backup; historical unlimited snapshots excluded                                                                                                 |
| Logs                                       | 0.1 GB ingest, 0.05 GB-month storage, 0.1 GB query                                                      | 2 GB ingest, 0.8 GB-month storage, 2 GB query; 14-day retention                                                                                                        |
| Delivery/egress                            | Static 1 GB/20k HTTPS; separate API/worker 1 GB                                                         | Static 5 GB/100k HTTPS; separate API 20 GB + worker target-request egress 1 GB                                                                                         |
| Secrets/crypto                             | One fresh KMS key, 10k operations; one service HMAC-root secret, 1k reads                               | One key with first rotation, 100k operations; one root secret, 10k reads                                                                                               |
| Bootstrap ECR residual                     | Allowance 0.1 GB existing shared images                                                                 | Allowance 0.5 GB; zip Lambda needs no new product ECR                                                                                                                  |

Billed durations include initialization/processing; values are assumptions,
not measured warm/cold latency. Lambda total GB-seconds: light
`1000×5×0.5 + 80000×0.05×0.25 + 50000×0.02×0.25 = 3750`;
heavy `10000×30×0.5 + 600000×0.1×0.25 + 50000×0.1×0.25 = 166250`.
Invocations: 131k / 660k. No durable-function/provisioned-poller premium.

DDB **billable units**, not raw API-call counts: light 0.20m WRUs = 0.12m
transactional base units + 0.08m normal index units; heavy 3m = 2m transactional
base units + 1m index units. These budgets include frozen plans/protected
bundles, gate mutations, receipts, events, OUT/tickets, retries, cleanup/audits.
Base transactional writes consume two units per rounded KiB; reads round at
4 KiB and transactional reads double strong-read units. Read budgets 0.5m/5m
RRUs include GSI discovery, hydration, transaction reads, empty maintenance
queries and conflicts. Larger actual frozen plans, crypto bundles or workflow
mix can exceed these budgets. No double addition of transaction multiplier:
it is already in the stated units. GSI writes are budgeted separately at normal
index units. [AWS transaction capacity accounting](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/transaction-apis.html).

### Rates and calculations

Totals use unrounded values. F=fixed configured charge; U=usage/storage. The
fixed baseline is $2.20 light / $3.20 heavy (including first rotation).
Everything else is U, including retained index/body/bootstrap storage.

| Item                   | Official rate and calculation                                                                                                                                          |     Light |      Heavy |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------: | ---------: |
| Lambda U               | [Ohio catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSLambda/current/us-east-2/index.json): ARM $0.0000133334/GB-s + $0.20/m invocations           |     $0.08 |      $2.35 |
| HTTP API U             | [Ohio catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonApiGateway/current/us-east-2/index.json): $1/m calls                                      |     $0.08 |      $0.60 |
| DDB requests U         | [Ohio catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonDynamoDB/current/us-east-2/index.json): $0.625/m WRU + $0.125/m RRU; 0.2/0.5m versus 3/5m |     $0.19 |      $2.50 |
| DDB base + indexes U   | Same source: $0.25/GB-month × 0.10 / 1.0 GB                                                                                                                            |     $0.03 |      $0.25 |
| DDB backups U          | Same source/[pricing](https://aws.amazon.com/dynamodb/pricing/): PITR $0.20/base GB-month + on-demand $0.10/base GB-month, 0.07 / 0.6 GB; indexes rebuilt on restore   |     $0.02 |      $0.18 |
| SQS U                  | [Pricing](https://aws.amazon.com/sqs/pricing/): $0.40/m standard requests; 0.5m / 1m includes empty receives, OUT retries, deletes/DLQ and 64 KiB billing chunks       |     $0.20 |      $0.40 |
| S3 U                   | [Pricing](https://aws.amazon.com/s3/pricing/): $0.023/GB-month × 0.3 / 21; PUT/LIST $0.005/1k × 2k / 32k; GET $0.0004/1k × 10k / 100k                                  |     $0.02 |      $0.68 |
| CloudFront U           | [US pay-as-you-go](https://aws.amazon.com/cloudfront/pricing/pay-as-you-go/): $0.085/GB × 1 / 5; $0.01/10k HTTPS × 20k / 100k; $0.10/m functions × 10k / 100k          |     $0.11 |      $0.54 |
| Logs + alarms U/F      | [CloudWatch](https://aws.amazon.com/cloudwatch/pricing/): $0.50/GB ingest + $0.03/GB-month storage + $0.005/GB query; three $0.10 standard alarms                      |     $0.35 |      $1.33 |
| KMS F/U                | [Pricing](https://aws.amazon.com/kms/pricing/): $1/key + $1 first rotation heavy; $0.03/10k symmetric operations × 10k / 100k                                          |     $1.03 |      $2.30 |
| Secrets Manager F/U    | [Pricing](https://aws.amazon.com/secrets-manager/pricing/): one $0.40 secret + $0.05/10k reads × 1k / 10k                                                              |     $0.41 |      $0.45 |
| Cognito U              | [Lite](https://aws.amazon.com/cognito/pricing/): $0.0055/MAU × 2 / 5, charge all users                                                                                 |     $0.01 |      $0.03 |
| SES U                  | [Pricing](https://aws.amazon.com/ses/pricing/): $0.10/1k emails × 100 / 1000, plus $0.12/GB × 0.002 / 0.02                                                             |     $0.01 |      $0.10 |
| API/worker egress U    | [Transfer pricing example](https://aws.amazon.com/vpc/pricing/): $0.09/GB × 1 / 21; static CloudFront separate                                                         |     $0.09 |      $1.89 |
| Route 53 F             | [Pricing](https://aws.amazon.com/route53/pricing/): one $0.50 zone, CloudFront/API Gateway alias queries have no query charge                                          |     $0.50 |      $0.50 |
| Shared bootstrap ECR U | [Pricing](https://aws.amazon.com/ecr/pricing/): $0.10/GB-month × 0.1 / 0.5; S3 bootstrap already included above                                                        |     $0.01 |      $0.05 |
| Scheduler U            | [EventBridge Scheduler](https://aws.amazon.com/eventbridge/pricing/): $1/m invocations × (43800 minute + 730 hourly)                                                   |     $0.04 |      $0.04 |
| **Monthly total**      | Light $3.16937025 / heavy $14.19510775 before rounding                                                                                                                 | **$3.17** | **$14.20** |

S3 heavy PUT/LIST count includes 10k evidence uploads, retries/import/static
writes and 2k orphan/deletion listings. GET count includes authorized evidence
reads and verification; not all CloudFront viewer requests are origin GETs.
Standard S3 DELETE has no request charge. No duplicate body stored in DDB or
workflow context. PITR charges base data, index storage charged once in DDB.
SSE-S3 avoids a second customer-managed storage key; application envelopes use
the single KMS key above. Secrets Manager root uses its default managed key;
crypto-operation allowance includes any paid managed-key requests.

Shared SST bootstrap S3 state/asset versions and residual ECR allowances are
**unverified**, not an actual inventory/bill. Existing standard SSM bootstrap
parameter has no standard storage charge; advanced parameters are not assumed.
SST state does not contain application secret plaintext. Lambda zip assets use
bootstrap S3 already counted. Unknown old stages/snapshots/unrelated account
resources are excluded, not asserted absent. Optional custom metrics, WAF,
tracing, paid endpoints, domains/CI, restore requests/transient restored tables,
SMS/email add-ons, taxes and deployment overlaps are not modeled. Alerts-only
AWS Budgets are a no-charge feature, not a credit/free-tier assumption.
[AWS Budgets pricing](https://aws.amazon.com/aws-cost-management/aws-budgets/pricing/).

KMS first and second rotations each add $1/month; light after those rotations
becomes $4.17 then $5.17. Heavy second rotation becomes $15.20. Do not disable
needed rotation to preserve the target. Pins are indefinite until deletion but
capacity is finite: additional pinned bodies cost storage every month. At
10,000 executions averaging 60 billed seconds, worker compute adds $2/month
to heavy. One continuously occupied 512 MiB Lambda adds about $17.52/month;
held SSE/long polls are explicitly excluded. The previous $71.05/$101.15
Fargate baselines remain historical estimates in Git, not current alternatives
or directly comparable workloads. No hard cap/guaranteed savings claim.

## Budgets and application quotas

**Proposed only; do not configure AWS.** Stage-scoped cost allocation plus
account-wide visibility for unattributed/shared bootstrap charges. Light alerts
at actual $3/$4/$5 and forecast >$5; heavy profile at $10/$15 and forecast >$20.
Owner email notification, weekly review, no automatic destructive budget actions.
Billing/alerts lag and do not stop spend. Admission quotas reduce expected use,
not unauthenticated API traffic, scheduler/poller charges or malicious request
costs. Public development access should be invite-only initially.

| Guard                               | Light default                                                                        | Reviewed heavier profile                                               |
| ----------------------------------- | ------------------------------------------------------------------------------------ | ---------------------------------------------------------------------- |
| Executions including workflow steps | 1k/stage/calendar month, 100/user/day                                                | 10k/month, 1000/user/day                                               |
| HTTP time                           | 30 s default, 60 s max; worker 90 s                                                  | Same                                                                   |
| Concurrency                         | 2 cloud stage, 1 unresolved local/credential; API reserved 5; maintenance reserved 1 | Same until measured                                                    |
| Polling                             | >=10 s/client, one in flight; server <=6 polls/min/credential, idle backoff to 60 s  | Active browser >=2 s, <=30 polls/min; local remains ten-second minimum |
| Retained evidence quota             | 1 GiB/project, 2 GiB/stage, pins consume same quota                                  | 10 GiB/project, 25 GiB/stage                                           |
| Definition/workflow sizes           | Model's entity/80-action/2 MiB transaction caps                                      | Same, not silently raised                                              |
| Capture/upload                      | 1 MiB request, 2 MiB response, 64 KiB upstream headers, 4 MiB encoded local result   | Same                                                                   |

Stage quota item `PK=STAGE#id/SK=QUOTA` joins admission/storage transactions,
with version/conditional counters; project gate owns per-project counters.
Quota reservations occur **before** accepting each execution (workflow reserves
all planned steps' monthly admission budget), and before PUT, with conservative
max-body reservation so concurrent work cannot overfill. Extra body beyond quota
is omitted with explicit metadata or new execution rejected; never evict pins.
Tickets prevent double accounting/release during uncertain uploads. Stage-wide
counter contention is an MVP tradeoff. Cleanup/refunds are conditional and
idempotent; completed execution count is not refunded to enable unlimited use.
Rate limits may use separate coarse-minute conditional counters (not every poll
changing project gate). Rejected calls still incur API/Lambda/DDB cost. Worker
event mapping max two plus reserved concurrency two bounds occupancy, not a
monthly monetary maximum. Do not enable provisioned concurrency/pollers.

## Execution, destinations and secret defaults

Cloud HTTP/HTTPS ports 80/443 only, redirects off. Reject userinfo/Host overrides,
ambiguous IP forms, ambient proxy environment, loopback/private/link-local/ULA/
CGNAT/multicast/reserved/documentation/metadata destinations. Resolve A/AAAA at
each dial; reject any nonpublic answer, normalize IPv4-mapped IPv6, initially deny
tunnel/translation forms. Pin validated IP while preserving hostname TLS SNI;
no second unvalidated resolver. Go global-unicast alone is insufficient. Maintain
tests against [IANA IPv4](https://www.iana.org/assignments/iana-ipv4-special-registry)
and [IPv6 registries](https://www.iana.org/assignments/iana-ipv6-special-registry).
Later redirects require full checks each hop and stripped cross-origin secrets.
Local agent default allowlist loopback only; private ports/hosts need explicit
local configuration; metadata endpoints remain blocked. Public API abuse still
needs quotas. Protected values over plaintext HTTP need explicit confirmation.

Request timeout covers connect/TLS/body; DNS 2 s, connect/TLS 5 s within total,
response header 10 s, response header cap 64 KiB. Disable streaming/infinite
responses, raw HTTP tracing and hidden Go retries. Bound decompression/JSON
depth/regex/assertion CPU and redact omissions/errors. HTTP API 30 s timeout
and Lambda synchronous 6 MiB event limit mean base64/envelope overhead counts;
local result 4 MiB wire cap is conservative, must be tested before acceptance.
Also cap the complete serialized evidence object/API reply at 4 MiB, counting
JSON escaping/base64 and headers/envelope. Omit oversized body fields with
explicit metadata before PUT/publication; a 2 MiB raw-body cap alone cannot
guarantee fitting Lambda's 6 MiB reply. Reserve the encoded object size, not
only decoded body bytes.
[HTTP API quotas](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-quotas.html),
[Lambda quotas](https://docs.aws.amazon.com/lambda/latest/dg/gettingstarted-limits.html).

KMS symmetric stage key, fresh per-binding data key and AES-256-GCM nonce;
envelopes in Protected only, no plaintext fallback. Review known-answer/tamper/
context tests and narrowly scoped KMS IAM. Annual key-material rotation proposed;
saved credential-value changes affect future submissions/reruns, not frozen jobs
unless revoked. Replacement-key rewrap is a reviewed versioned background change;
retain old keys/root-HMAC versions while pins/receipts/backups need them. No DB
credentials remain; one purpose-derived cursor/receipt HMAC root in Secrets
Manager. [Envelope encryption](https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html),
[encryption context](https://docs.aws.amazon.com/kms/latest/developerguide/encrypt_context.html).

## Authentication and email

Propose Cognito Lite explicitly, public SRP-capable app client, 15-minute access
tokens, one-day rotating refresh session, browser memory storage (not
localStorage), reauthenticate after reload. Go verifies RS256/key type/kid,
configured issuer/client_id, token_use=access, exp/issuance with 60-second clock
tolerance; cache configured JWKS only, rate-limit unknown-kid refresh. Reject
ID tokens even if gateway accepted them; user disabled state checked each time.
Offline JWT validation does not immediately observe Cognito sign-out/revocation;
immediate session denial would require an additional reviewed session check.
[Cognito verification](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-tokens-verifying-a-jwt.html).

Local/MCP tokens separate, 30-day expiry, hashed 256-bit proofs; one active local
credential/user; revoked original token denies even accepted-upload ACKs. Pairing
requires Cognito-authenticated owner. MCP stdio transport/read-only audited
bridge initially; remote OAuth/intended-recipient behavior is not implied.

SES-backed Cognito email proposed for public signup. Verify regional compatibility,
sender identity, DKIM/SPF/DMARC, sandbox/production quotas, bounce/complaint
handling and delivery before presentation. Test signup/verify/resend cooldown,
reset/expired codes, generic enumeration-resistant errors and real independent
recipient delivery. No email delivery/configuration tested here.
[Cognito email guidance](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-email.html).

## Future acceptance tests for the four reliability guarantees

These are **future implementation tests**, not added/run by this documentation
task. Use controlled crash barriers, actual service semantics in a separately
authorized test environment, and target call counters; mocks alone are insufficient.

| Guarantee                   | Required tests and observable outcomes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Durable dispatch boundary   | Roll back local grant: still queued, no executable response. Lose committed poll response/crash before or after HTTP: no regrant/replay, exactly one unknown terminal result on expiry. Cloud uncertain intent must read back or abstain. Expired start window blocks agent HTTP. Revoke credential during grant/upload: one conditioned outcome, never falsely promise recalled secrets. Duplicate SQS delivery performs at most one target send after intent.                                                                |
| Atomic workflow advancement | Inject failure at every finalization action: no visible SNAP/output/successor. Crash after S3 PUT before commit: inaccessible orphan, unknown step and no successor. Kill after commit before OUT send: exact encrypted variable versions survive and one successor recovers. Extract marked token before raw-buffer disposal: later HTTP receives it, evidence/logs/queue/MCP never do. Missing output fails/skips. Concurrent finalizers create one snapshot/job; test 80-action/2 MiB limits and no duplicate item actions. |
| Idempotent uploads          | Drop result commit ACK then retry after lease expiry: original receipt 200, unchanged snapshot/events/run version/successor count. Reordered object keys/whitespace identical; body/array/timing/extraction changes 409. Wrong lease/nonce/replacement token rejected. Revoked original token 401; cleanup 404; unknown terminal never overwritten. Canonicalization/key rotation preserves accepted duplicates without recovering historical secrets.                                                                         |
| Pin/context/cleanup         | Force gate/RET conflicts: still-available pin first protects body/context; cleanup first/expiry means pin cannot resurrect. Clean expired sibling while retaining only required safe context. Unpin last step after run expiry: body/unneeded context disappear logically at commit, either cleaner order yields same physical result. TTL never deletes authoritative protected history. Project tombstone overrides pins and all later writes.                                                                               |

Additional mandatory tests: base/GSI lag cannot bypass owner/state conditions;
transaction read cannot mix pin/cleanup versions; quota reservations never
double-release. Pause PUT across project tombstone and finish afterward: manifest
commit rejected, no user access, ticket drain removes object. Lose PUT ACK:
deletion remains draining until uncertainty resolved, never completes from one
empty listing. Race orphan sweeper against publication: exactly one winner, no
published body erased as orphan. Late object after sweep is caught by ticket/
residual prefix sweeper. Drain >100 child items with restart/cursor loss; no
protected data/pins left. Restore deletion ledger before opening restored data.
Lost result DB acknowledgment is resolved by reading receipt, never HTTP replay.

## Contract gaps and synchronization after review

Findings use the original v0.1 source baseline. Do **not** update Google Docs,
Trello, proposal or accepted ADR during this revision. After agreement:

| Source/route/schema                  | Exact synchronization needed                                                                                                                                                                                                                                                                                  |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Proposal technology/deployment/W2/W9 | Replace stale CDK with accepted SST; replace Fargate/RDS/PostgreSQL topology with reviewed Lambda/DDB/S3 design, including current app users keyed to Cognito; preserve historical spikes. Fix stale infra/README framework wording in that later task.                                                       |
| Architecture/contract review cards   | Attach approved design/ADR/settings and new acceptance criteria; keep architecture pending until accepted, contract card open until affected schemas agreed. No Done/card move here.                                                                                                                          |
| Resource routes/ownership            | Add reviewed project scope to execution/job/rerun/pin/result paths or query parameters; no opaque-ID global locator. Define 25/100 cursor collections and bounded filters/eventual list reconciliation.                                                                                                       |
| KeyValueField/sensitive writes       | Add set/preserve/remove/secretRef semantics, masked reads and sensitive URL/body paths; literal mask is not a credential. Exclude raw secrets from ordinary returned definitions.                                                                                                                             |
| Submit/queue                         | Add Idempotency-Key/revision checks/frozen plans/limits; identifier-only queue, advisory attempt, no user authority in message. Document seven-day submission retry window.                                                                                                                                   |
| Execution summary/record             | Separate mutable status/version/RET from immutable manifest/body; nullable deleted live links versus frozen source IDs, body omission/checksum/schema version, unknown outcome, exact upstream status. GET returns 200 failed evidence for upstream 404/500.                                                  |
| Project history/reruns               | Add project history with frozen request filter; reruns capture current stable secret refs and fail if missing, no historical secret recovery.                                                                                                                                                                 |
| SSE/status                           | Replace SSE proposal with immediate-return status/batch polling, version/serverTime, 2/10/30/60 s policy, stop/retry rules and latency expectations. Do not silently keep an implemented SSE promise—none exists.                                                                                             |
| Agent token/poll/result              | Map localAgentId to credential ID, immediate 204 empty, intent-before-response, leaseId/fence/nonce/start deadline/serverTime, busy behavior and no start route/regrant. Define canonical DTO/crypto envelope/4 MiB cap and original-credential ACK after lease expiry; 200/401/404/409 and revocation races. |
| Workflow definitions/runs            | Add CRUD/revision, ordered 2–10 steps/extraction grammar, environment/target selection, exact output provenance, skipped states, limits and atomic successor scheduling; local sensitive extraction trust limitation.                                                                                         |
| Pin/unpin/context                    | Proposed PUT/DELETE project-scoped execution pin, idempotent 200 RET; original expiry/no grace, expired pin rejection, context protection and sibling evidenceExpired; no TTL race.                                                                                                                           |
| DELETE project 204                   | Propose 202 deletion-operation resource: immediate denial, durable drains, pending uncertain uploads, terminal physical completion, minimal tombstone/backup residual. Exact GET operation route and response states require review.                                                                          |
| Imports/operation association        | Specify supported dialect/library, local refs only, sanitization/warnings, request operation association, staged import readiness instead of claiming atomic 100-operation insertion.                                                                                                                         |
| MCP                                  | Define distinct token pairing/revocation and audited invocation bridge, read-only tools/cursors; no Protected/KMS access; remote transport separate.                                                                                                                                                          |
| Health/error envelopes               | Reconcile current foundation /healthz with /health and /ready contracts; spike routes are not product routes. Review assertion/schema/3xx/unknown/omission codes and polling Retry-After.                                                                                                                     |

Remaining review choices: server decisionTime/five-second near-expiry pin margin
(DynamoDB has no transaction NOW), async deletion/backup residual/minimal permanent ledger,
the three-index/two-table design and project/stage contention, ten-step/bundle/
import limits, receipt canonical DTO and HMAC key retention, supported extraction
grammar/OpenAPI dialect, HTTP secret confirmation, active polling latency,
budget/profile quotas, seven-day PITR plus short-lived backup policy and annual
KMS rotation. Direction approval does not accept all of these defaults or
authorize infrastructure implementation. ADR 0002 remains Proposed.

## Smallest next implementation task

After design acceptance and contract synchronization: **local users/projects
DynamoDB ownership slice**. Add only identity/user/project codecs and conditional
repository operations, verified-identity boundary, agreed users/me/project CRUD,
project-scoped version/owner checks and transaction-budget tests using a local
DynamoDB test environment. No AWS deployment/full model/executor yet. Verify
cross-user denial, concurrent revision conflict and tombstone refusal. Production
Cognito verification is mandatory before hosted exposure; test JWT identity is
never a production bypass. Separate later task tests actual DynamoDB/S3/Lambda
semantics under expressly authorized AWS test resources.

## Documentation validation scope

Completed documentation/repository checks for this revision:

- Prettier formatted the five changed Markdown files. `GOCACHE=/tmp/kurier-w1-go-build
npm run verify` passed repository formatting/gofmt checks, ESLint, Vitest
  (one existing frontend test), TypeScript/Vite build, and Go test/vet for all
  five existing modules. No product behavior tests were added.
- `docker compose config --quiet` and `git diff --check` passed. Compose remains
  the existing local PostgreSQL foundation, not validation of DynamoDB.
- Mermaid parser passed all three diagrams (topology, sequence, logical ERD).
  Balanced fences and 25 internal file/heading links passed across five files.
  Parser validation is not visual layout review or runtime execution.
- All 34 public external AWS/SST/IANA links returned HTTP 200 (redirects allowed),
  including regional catalogs. Three authenticated Trello/Google source URLs
  were not revalidated; their contents were read for the original baseline,
  and none was edited in this revision.
- Independent arithmetic recomputed $3.16937025 light and $14.19510775 heavy
  from the stated unrounded inputs. Usage/cold-start/storage/bootstrap allowances
  remain unmeasured and price availability does not prove a deployed bill.

Future reliability, IAM, polling load, DynamoDB isolation/GSI/TTL, S3 publication/
deletion, sanitizer, crypto, Cognito/email and restore tests above are **not run**.
No SST diff/install/deploy, migration, AWS resource mutation or repeated spike.
Formatting/links/Mermaid and existing scaffold tests validate documentation and
repository consistency only, not proposed runtime behavior or measured cost.
