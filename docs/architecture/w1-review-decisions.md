# W1 review decisions, costs and contract gaps

Status: **Accepted by Davian Hernandez**, 2026-10-06, baseline `1fba942`,
including documented defaults. See [system design](w1-system-design.md),
[DynamoDB model](w1-data-model.md) and [accepted ADR 0002](../decisions/0002-serverless-persistence-topology.md).
Acceptance/source synchronization is recorded in [the handoff](w1-acceptance-sync.md).
Genuinely new wire/library choices remain proposed; contract-review stays open.
Deployment, product implementation and AWS mutations are not authorized.

## Decision sheet

| Choice            | Recommendation/status                                                                 | Reason                                              | Material tradeoff                                                           |
| ----------------- | ------------------------------------------------------------------------------------- | --------------------------------------------------- | --------------------------------------------------------------------------- |
| Infrastructure    | SST 4 **accepted**, ADR 0001 unchanged                                                | Historical spikes support framework selection       | Bootstrap/state costs and reviewed deployment IAM remain                    |
| Revised direction | Go ARM Lambda + HTTP API + SQS + DynamoDB + private S3; accepted baseline 1fba942     | Removes always-on compute/network/database baseline | New persistence and invocation behavior untested; no stable outbound IP     |
| Tables/indexes    | Accepted Control/Protected, Standard on-demand, three sparse KEYS_ONLY Control GSIs   | Isolates crypto store; explicit access patterns     | No SQL FKs/joins; index lag/read amplification; gate contention             |
| Integrity         | Accepted gate CAS + bounded cross-table transactions + immutable S3 manifests/tickets | Preserves reviewed atomic state boundaries          | Objects cannot join a DynamoDB transaction; orphan/deletion reconciliation  |
| Status delivery   | Accepted immediate polling, ten-second light default/two-second active option         | Avoids Lambda billing for held connections          | Notifications delayed; replaces draft SSE contract                          |
| Local dispatch    | Accepted intent before poll response, 30-second start/120-second lease                | Durable knowledge of possible HTTP dispatch         | Lost poll can produce conservative unknown; no replay                       |
| Local receipt     | Accepted retained canonical private HMAC/lease identity                               | ACK retries after successful commit/expired lease   | Key retention; credential revocation still denies ACK                       |
| Workflow          | Accepted sequential 2–10 steps, one atomic finalization/successor transaction         | Bounds action/byte limits                           | No branching/loops; oversized input rejected                                |
| Deletion          | Accepted immediate access denial + 202 resumable operation                            | Honest cross-store cleanup and late-PUT handling    | Changes DELETE 204 contract; physical completion may await uncertain writes |
| Secrets           | Accepted per-stage KMS AES-GCM envelopes, no reveal                                   | Authorized reuse without evidence disclosure        | KMS outages/cost/rotation; local device receives required runtime values    |
| Retention         | Agreed 30-day unpinned; pins until project deletion. Conditional protocol accepted    | Metadata independent of immutable evidence          | No native TTL for authoritative history; pinned growth requires quotas      |
| Auth              | Cognito and shared-theme screens **agreed**; Lite/15-minute tokens accepted           | Keep login credentials outside app store            | Offline JWT revocation delay and email-flow checks                          |

## Monthly development cost estimate

Official prices checked **2026-10-06**, USD, **us-east-2**, US CloudFront viewers.
No AWS credentials/account billing queries or mutations. Ohio Lambda/request/
DynamoDB/HTTP API rates were read from official regional catalogs during the
preceding investigation and rechecked for this revision; public reference links
and restore/transaction constraints were also checked. No credits, account free-tier allowances, savings
plans, reservations or discounts subtracted. Built-in no-charge service features
are distinguished from eligibility-based free tiers. No deployments exist to
measure this product workload: all usage below is an explicit forecast.

Retain the earlier **approximately $3.10/month** light screening estimate.
Detailed accounting including always-on scheduled maintenance and bootstrap
allowances refines it to **$3.17/month**, including the fast-notification allowance. This is neither actual billed usage nor
a $5 hard cap. Heavy use is **$14.14/month**, including one KMS rotation.

### Workload and storage assumptions

| Assumption                                 | Light development                                                                                       | Heavier development                                                                                                                                                    |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Active users                               | 2 Cognito Lite MAUs                                                                                     | 5 MAUs                                                                                                                                                                 |
| Cloud executions, including workflow steps | 1,000/month; average billed worker 5 s at 512 MiB                                                       | 10,000/month; average billed worker 30 s at 512 MiB                                                                                                                    |
| API calls                                  | 80,000 at 50 ms/256 MiB                                                                                 | 600,000 at 100 ms/256 MiB                                                                                                                                              |
| Polling                                    | One browser + one agent, four h/day × 22 days, ten-second base; <=63,360 base polls, other calls in 80k | Two browsers + one agent, eight h/day × 22 days: two-second active browser/ten-second agent, <=696,960 if continuously active; backoff/stops modeled to fit 600k total |
| Maintenance/outbox/cleanup                 | 50,000 invocations at 20 ms/256 MiB, including empty schedules                                          | 50,000 at 100 ms/256 MiB; batched retries/cleanup work                                                                                                                 |
| Replicas/operating hours                   | No allocated replicas/provisioned concurrency; stage available 730 h; worker max concurrency 2          | Same; enough total capacity but not a benchmark                                                                                                                        |
| Worker body                                | Average 10 KiB request/100 KiB sanitized response                                                       | Average 32 KiB resolved request body/1 MiB sanitized response; cap 64 KiB request and 2 MiB decompressed response                                                      |
| Average DDB base/index storage             | 0.07 GB Control+Protected / 0.03 GB all three indexes                                                   | 0.6 GB base / 0.4 GB indexes                                                                                                                                           |
| S3 evidence retained                       | Approximately 0.1 GB 30-day body population, minimal pins                                               | 20 GB total: about 10 GB rolling bodies + 10 GB accumulated pins/orphans/import allowance                                                                              |
| Other S3 storage                           | 0.2 GB static assets + versioned SST state/assets                                                       | 1 GB static/bootstrap versions/assets                                                                                                                                  |
| Backups                                    | Seven-day PITR on base tables + one on-demand backup totaling 0.07 GB-month, expiring after seven days  | Same with 0.6 GB-month backup, seven-day on-demand backup expiry; historical unlimited snapshots excluded                                                              |
| Logs                                       | 0.1 GB ingest, 0.05 GB-month storage, 0.1 GB query                                                      | 2 GB ingest, 0.8 GB-month storage, 2 GB query; 14-day retention                                                                                                        |
| Delivery/egress                            | Static 1 GB/20k HTTPS; separate API/worker 1 GB                                                         | Static 5 GB/100k HTTPS; separate API 20 GB + worker target-request egress 0.4 GB                                                                                       |
| Secrets/crypto                             | One fresh KMS key, 10k operations; one service HMAC-root secret, 1k reads                               | One key with first rotation, 100k operations; one root secret, 10k reads                                                                                               |
| Bootstrap ECR residual                     | Allowance 0.1 GB existing shared images                                                                 | Allowance 0.5 GB; zip Lambda needs no new product ECR                                                                                                                  |

Billed durations include initialization/processing; values are assumptions,
not measured warm/cold latency. Lambda total GB-seconds: light
`1000×5×0.5 + 80000×0.05×0.25 + 50000×0.02×0.25 = 3750`;
heavy `10000×30×0.5 + 600000×0.1×0.25 + 50000×0.1×0.25 = 166250`.
Invocations: 131k / 660k. Add one fast-send opportunity per cloud job, 50 ms
average billed duration (unmeasured), 80% in 256 MiB API submission handlers and
20% in 512 MiB successor handlers: `N×0.05×(0.8×0.25+0.2×0.5)`
= 15 / 150 additional GB-seconds, total **3765 / 166400**. Incremental to baseline
durations, not another Lambda invocation. At the accepted two-second send
deadline that opportunity mix costs about $0.008/$0.080 compute instead of
$0.0002/$0.002, excluding unrelated execution time. Skip sends lacking time.
SQS allowances already include 1k/10k additional fast notifications and scheduled
duplicates; no extra DDB transaction marks OUT published on the fast path.
No double-counted notification charge or provisioned-poller premium.

Heavy body traffic is `10000×32 KiB = 327680000 bytes`; assume another 4 KiB
request-line/headers per execution (40960000 bytes). Round combined traffic up
to a **0.4 priced-GB allowance**, replacing 1 GB. Unchanged 20 GB API result
delivery makes 20.4 GB/$1.836 rather than 21 GB/$1.89. Static delivery is separate.
Retained bodies remain 1 MiB average/20 GB population. DDB units/index/storage,
OUT retries, cleanup/logs/bootstrap and backups remain explicit conservative
budgets (including stage-generation checks), not automatically proportional to
request-body bytes. On-demand backups rotate weekly with seven-day expiry,
approximately one retained full base-table copy over the month, hence the
unchanged 0.07/0.6 GB-month backup-storage allowances. Recovery exercises and
transient restored tables remain outside steady-state totals, not zero-cost.

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
| API/worker egress U    | [Transfer pricing example](https://aws.amazon.com/vpc/pricing/): $0.09/GB × 1 / 20.4; static CloudFront separate                                                       |     $0.09 |      $1.84 |
| Route 53 F             | [Pricing](https://aws.amazon.com/route53/pricing/): one $0.50 zone, CloudFront/API Gateway alias queries have no query charge                                          |     $0.50 |      $0.50 |
| Shared bootstrap ECR U | [Pricing](https://aws.amazon.com/ecr/pricing/): $0.10/GB-month × 0.1 / 0.5; S3 bootstrap already included above                                                        |     $0.01 |      $0.05 |
| Scheduler U            | [EventBridge Scheduler](https://aws.amazon.com/eventbridge/pricing/): $1/m invocations × (43800 minute + 730 hourly)                                                   |     $0.04 |      $0.04 |
| **Monthly total**      | Light $3.169570251 / heavy $14.14310776 before rounding                                                                                                                | **$3.17** | **$14.14** |

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
becomes $4.17 then $5.17. Heavy second rotation becomes $15.14. Do not disable
needed rotation to preserve the target. Pins are indefinite until deletion but
capacity is finite: additional pinned bodies cost storage every month. At
10,000 executions averaging 60 billed seconds, worker compute adds $2/month
to heavy. One continuously occupied 512 MiB Lambda adds about $17.52/month;
held SSE/long polls are explicitly excluded. The previous $71.05/$101.15
Fargate baselines remain historical estimates in Git, not current alternatives
or directly comparable workloads. No hard cap/guaranteed savings claim.

## Budgets and application quotas

**Accepted design defaults; do not configure AWS.** Stage-scoped cost allocation plus
account-wide visibility for unattributed/shared bootstrap charges. Light alerts
at actual $3/$4/$5 and forecast >$5; heavy profile at $10/$15 and forecast >$20.
Owner email notification, weekly review, no automatic destructive budget actions.
Billing/alerts lag and do not stop spend. Admission quotas reduce expected use,
not unauthenticated API traffic, scheduler/poller charges or malicious request
costs. Public development access should be invite-only initially.

| Guard                               | Light default                                                                                                                                             | Reviewed heavier profile                                               |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Executions including workflow steps | 1k/stage/calendar month, 100/user/day                                                                                                                     | 10k/month, 1000/user/day                                               |
| HTTP time                           | 30 s default, 60 s max; worker 90 s                                                                                                                       | Same                                                                   |
| Concurrency                         | 2 cloud stage, 1 unresolved local/credential; API reserved 5; maintenance reserved 1                                                                      | Same until measured                                                    |
| Polling                             | >=10 s/client, one in flight; server <=6 polls/min/credential, idle backoff to 60 s                                                                       | Active browser >=2 s, <=30 polls/min; local remains ten-second minimum |
| Retained evidence quota             | 1 GiB/project, 2 GiB/stage, pins consume same quota                                                                                                       | 10 GiB/project, 25 GiB/stage                                           |
| Definition/workflow sizes           | Model's entity/80-action/2 MiB transaction caps                                                                                                           | Same, not silently raised                                              |
| Capture/upload                      | 64 KiB complete saved/frozen config each and resolved body; 2 MiB wire/decompressed response separately, 64 KiB headers, 4 MiB complete encoded envelopes | Same                                                                   |

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

**Accepted MVP size defaults:** complete serialized saved request configuration
<=64 KiB; complete serialized frozen configuration per execution <=64 KiB;
resolved outbound body <=64 KiB. Configuration totals include URL, headers,
query fields, body descriptors, references, policies/schema and serialization
overhead, not only body text. Reject invalid saved writes and oversize frozen
plans before accepting submission. Independently resolve/interpolate and bound
outbound body before HTTP on cloud/local execution: template size alone is not
sufficient. Large request-body storage/S3 body indirection is outside MVP.
Future increases require item/transaction/transport/memory review.

Response has **two independent limits**: wire bytes read <=2 MiB and body after
decompression <=2 MiB. Abort overrun, classify capture Failed and report explicit
omission/limit metadata with observed HTTP status where available. Never retain
a silently truncated success or extract workflow variables from oversize response.
Compression cannot evade the decompressed cap.

Request timeout covers connect/TLS/body; DNS 2 s, connect/TLS 5 s within total,
response header 10 s, response header cap 64 KiB. Disable streaming/infinite
responses, raw HTTP tracing and hidden Go retries. Bound decompression/JSON
depth/regex/assertion CPU and redact omissions/errors. HTTP API 30 s timeout
and Lambda synchronous 6 MiB event limit mean base64/envelope overhead counts;
local result 4 MiB wire cap is conservative, must be runtime-tested before exposure.
Also cap the complete serialized evidence object/API reply at 4 MiB, counting
JSON escaping/base64 and headers/envelope. Omit oversized body fields with
explicit metadata before PUT/publication; a 2 MiB raw-body cap alone cannot
guarantee fitting Lambda's 6 MiB reply. Reserve the encoded object size, not
only decoded body bytes.
[HTTP API quotas](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-quotas.html),
[Lambda quotas](https://docs.aws.amazon.com/lambda/latest/dg/gettingstarted-limits.html).

KMS symmetric stage key, fresh per-binding data key and AES-256-GCM nonce;
envelopes in Protected only, no plaintext fallback. Review known-answer/tamper/
context tests and narrowly scoped KMS IAM. Annual key-material rotation accepted;
saved credential-value changes affect future submissions/reruns, not frozen jobs
unless revoked. Replacement-key rewrap is a reviewed versioned background change;
retain old keys/root-HMAC versions while pins/receipts/backups need them. No DB
credentials remain; one purpose-derived cursor/receipt HMAC root in Secrets
Manager. [Envelope encryption](https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html),
[encryption context](https://docs.aws.amazon.com/kms/latest/developerguide/encrypt_context.html).

## Authentication and email

Use Cognito Lite explicitly, public SRP-capable app client, 15-minute access
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

SES-backed Cognito email accepted for public signup. Verify regional compatibility,
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
protected data/pins left. Recovery tests restore an earlier point, show a
later-deleted item reappearing with actual timestamp/warning, and verify no
universal deletion-survival promise. Missing/corrupt S3 evidence is unavailable,
not fabricated; missing Protected inputs quarantined. Restore queued/claimed/
running work/stale SQS: zero automatic target HTTP, stopped runs/suppressed OUT,
old agent results rejected; deliberate new-generation submission can execute.
Partial cross-table transaction records cannot open an unreconciled stage.

Fast-dispatch future tests: lose SendMessage ACK, fail/throttle/timeout send,
race scheduled OUT, crash after send. Original committed 202/identity remains;
one job, duplicate notifications, at most one target HTTP. Repeated idempotent
submission never creates a job. Successor commit near deadline skips fast send;
durable OUT recovers. Test 64 KiB saved/frozen boundaries including overhead;
valid template expanding to >64 KiB fails before HTTP. Test separate 2 MiB wire/
decompression overruns, Failed omission metadata and 4 MiB encoded caps.
Lost result DB acknowledgment is resolved by reading receipt, never HTTP replay.

## Contract synchronization and remaining review

Davian accepted baseline `1fba942`, including documented defaults; ADR 0002
is Accepted. The [acceptance/synchronization record](w1-acceptance-sync.md)
records fresh external reads, completed edits, Trello scope and concrete new
wire recommendations. Historical draft findings are now resolved as follows:

| Source/route/schema                  | Synchronization recorded                                                                                                                                                                                                                                                  |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Proposal technology/deployment/W2/W9 | SST/Lambda/HTTP API/DynamoDB/private S3 replace CDK/Fargate/RDS product plans. infra/README is corrected; historical spikes remain unchanged.                                                                                                                             |
| Architecture/contract cards          | Architecture acceptance and documentation completion; contract review remains Doing for genuinely new wire/library proposals. Only Davian-authorized cards changed.                                                                                                       |
| Ownership/collections                | Project-scoped execution/job/result/rerun/run routes; 25/100 cursors, bounded filters and eventual reconciliation; config/body limits and recovery conditions.                                                                                                            |
| Sensitive writes                     | Masked stable refs, set/preserve/remove/reference semantics, no ordinary raw reads; exact discriminated DTO/path shape is newly proposed.                                                                                                                                 |
| Submission/queue                     | Idempotency-Key/seven-day receipt, expected revisions, immutable frozen plan, identifier-only duplicate notification, bounded fast send; failure never undoes 202.                                                                                                        |
| Evidence/history/reruns              | Mutable status/RET separate from immutable capture; nullable live/frozen IDs, checksum/omission/schema/unknown/status; GET failed upstream evidence 200; current stable secrets only.                                                                                     |
| Polling                              | SSE removed; immediate status/batch, versions/serverTime, stop/backoff/2/10/30/60-second profiles; exact route spellings newly proposed.                                                                                                                                  |
| Local protocol                       | Credential identity, 204 empty/busy, intent-before-response, deadlines/fencing/no start route/regrant; retained duplicate ACK versus live first-upload authorization and revocation. Canonical DTO proposed separately.                                                   |
| Workflows                            | Bounded frozen sequential plans, revisions/source/target/provenance/skipped states, atomic output/snapshot/step/run/receipt/successor transaction. CRUD DTO and extraction grammar newly proposed.                                                                        |
| Pin/context                          | Accepted project-scoped PUT/DELETE, idempotent 200 RET; original expiry/no grace, five-second margin, shared CAS cleanup protocol/context without expired bodies.                                                                                                         |
| Deletion/recovery                    | Accepted 202 denial/drain, late-upload fencing and restore exception. The users/projects operation DTO and repeat-delete/tombstone fields are subsequently accepted in [the slice contract](users-projects-contract.md); only local empty-project cleanup is implemented. |
| Imports/MCP                          | Ready-only staged imports/operation refs, local refs/warnings; distinct read-only stdio MCP token/audit bridge, no Protected/KMS. Dialect/library and wire proposals remain review inputs.                                                                                |
| Health/errors                        | /healthz unchanged; users/projects API error mappings now accepted in the slice contract. Product /ready exposure and execution capture-code enumeration remain proposed; Retry-After and upstream-status distinctions are preserved.                                     |

All existing architecture defaults (including key retention, table/index/action
limits, quota profiles, PITR/backup/rotation, pin decisionTime, HTTP secret
confirmation, polling, complete size caps and fast notification) are accepted.
Acceptance does not establish AWS correctness or measured usage. Only previously
unspecified wire details/libraries remain proposed for Davian's review. See the
concrete proposal table in the acceptance record; contract review is not Done
until those affected schemas are agreed and checked.

## Davian-selected restore policy and backup residuals

**Selected 2026-10-06:** earlier restoration may restore later-deleted projects/
items and lose later changes. Report actual Control/Protected restore timestamp(s)
and warn that later changes may be lost and later deletions may reappear.
No independently preserved deletion journal solely to block resurrection.
Normal active-store denial, dispatch/upload fencing, tombstones and orphan
cleanup remain; backups are not project-selectively erased. Seven-day PITR/
on-demand retention is an accepted operational default.

Before reopening reconcile Control, Protected and S3 under a closed recovering
stage. Missing evidence is unavailable without rewriting immutable capture;
do not invent bodies or infer a complete cross-store snapshot. All restored
nonterminal jobs/OUT are suppressed/failed recovery_interrupted regardless of
restored intent; new generation and deliberate submissions/reruns are required
for external HTTP. Invalidate old agent/MCP credentials pending re-pairing.
Review the [recovery procedure](w1-system-design.md#backup-restore-and-recovery-exception);
no restore or runtime tests were run here.

## Smallest next implementation task

The focused users/projects packet and its local implementation have now been
separately authorized. [Accepted interfaces](users-projects-contract.md) and
[Local tests/limitations](../development/local-ownership.md) record that slice.
The paragraph below is the historical first-slice recommendation, not a claim
that the full protocol's future tests have run. Later extraction/import/result
choices remain proposed and are not users/projects prerequisites.

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

The list below records historical documentation validation of accepted baseline
`1fba942` (revision from `0fd9194`), not the external synchronization or runtime
recovery/dispatch implementation. Current synchronization checks are recorded
in [the acceptance handoff](w1-acceptance-sync.md#validation-and-handoff).

Completed documentation/repository checks for that baseline revision:

- Prettier formatted the five changed Markdown files. `GOCACHE=/tmp/kurier-w1-go-build
npm run verify` passed repository formatting/gofmt checks, ESLint, Vitest
  (one existing frontend test), TypeScript/Vite build, and Go test/vet for all
  five existing modules. No product behavior tests were added.
- `docker compose config --quiet` and `git diff --check` passed. Compose remains
  the existing local PostgreSQL foundation, not validation of DynamoDB.
- Mermaid parser passed all three diagrams (topology, sequence, logical ERD).
  Balanced fences and 27 internal file/heading links passed across five files.
  Parser validation is not visual layout review or runtime execution.
- All 35 public external AWS/SST/IANA links returned HTTP 200 (redirects allowed),
  including regional catalogs. Three authenticated Trello/Google source URLs
  were not revalidated; their contents were read for the original baseline,
  and none was edited in this revision.
- Re-read public Ohio price catalogs and checked Lambda ARM duration/requests,
  HTTP API calls and DynamoDB write/read/storage/PITR rates on 2026-10-06.
  This was anonymous pricing research, not an AWS account/resource operation.
- Independent arithmetic recomputed $3.169570251 light and $14.14310776 heavy
  from the stated unrounded inputs. Usage/cold-start/storage/bootstrap allowances
  remain unmeasured and price availability does not prove a deployed bill.

Future reliability, IAM, polling load, DynamoDB isolation/GSI/TTL, S3 publication/
deletion, sanitizer, crypto, Cognito/email and restore tests above are **not run**.
No SST diff/install/deploy, migration, AWS resource mutation or repeated spike.
Formatting/links/Mermaid and existing scaffold tests validate documentation and
repository consistency only, not proposed runtime behavior or measured cost.
