# W1 review decisions and contract gaps

Status: proposed for Davian Hernandez review, 2026-10-05. These recommendations
are concrete review inputs, not changes to the accepted ADR or source contracts.
See [system architecture and execution sequence](w1-system-design.md) and
[ERD/schema](w1-data-model.md). The architecture task stays Doing and the
API-contract review card stays open until review and synchronized documentation.

This revision starts from `c311e74`. The four corrected protocols, costed network
alternatives and synchronization checklist are **proposed for Davian Hernandez's
review**. No API contracts, Trello state, capstone proposal or accepted ADR
are modified; the architecture task remains pending review.

## Decision sheet

Every recommendation in this table is **proposed** unless marked agreed.
The estimate below uses checked official rates and explicit usage assumptions;
it is not a quote or deployed billing measurement.

| Decision                            | Recommended choice                                                                                            | Reason                                                             | Material cost/tradeoff                                                                                           |
| ----------------------------------- | ------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------- |
| Infrastructure framework (agreed)   | SST 4, accepted ADR 0001                                                                                      | Both spike phases support it                                       | SST/Pulumi state/bootstrap remain operational dependencies; proposal CDK wording needs reconciliation.           |
| Product networking                  | Private Fargate tasks, HTTPS ALB, isolated private RDS, one NAT in development                                | Keeps task/DB ingress constrained while worker reaches public APIs | NAT and ALB fixed charges plus egress; one NAT is an AZ dependency. Public-IP alternatives need explicit review. |
| Initial persistence                 | PostgreSQL 17; bounded sanitized evidence in DB; static frontend in S3/CloudFront                             | Atomic finalization/deletion avoids premature DB/S3 coordination   | Database storage/backup growth; S3 artifact extension adds deletion reconciliation.                              |
| Authentication (provider/UI agreed) | Cognito Lite explicitly selected; public browser app client; custom shared-theme screens; short access tokens | Basic email/password fits Lite; no login credentials in Kurier DB  | Cognito MAU/email usage; Lite excludes newer features; Essentials is the current default when tier omitted.      |
| Execution limits                    | 30-second HTTP deadline, maximum 60; 1 MiB request, 2 MiB response, 64 KiB headers                            | Bounds worker occupancy and evidence growth                        | Large APIs/uploads/streams unsupported; omitted bodies require clear UI metadata.                                |
| Cloud destinations                  | Public HTTP/HTTPS, ports 80/443 only, redirects off, validate and pin resolved public address                 | Prevents private-network and credential-endpoint requests          | Nonstandard public ports need local execution or later reviewed expansion.                                       |
| Reusable secrets                    | Per-stage symmetric KMS key, AES-256-GCM envelope encryption in protected PostgreSQL rows                     | Authenticated encryption plus explicit authorized runtime access   | KMS key/API cost and outage dependency; more crypto metadata and rotation work.                                  |
| Queue and retries                   | Standard SQS + DB outbox/lease/fence; no automatic HTTP replay after dispatch intent                          | Durable acceptance and one final snapshot despite duplicates       | Extra DB state; uncertain crash outcomes require user rerun.                                                     |
| Pagination                          | Keyset cursor, 25 default / 100 maximum, HMAC-authenticated scoped cursor                                     | Stable ordering and bounded work without large offsets             | Cursor key management; no random page-number jumps.                                                              |
| Status stream                       | Fetch-based bearer-authenticated SSE, monotonic event IDs, replay then GET reconciliation                     | Works with current bearer contract and recovers missed events      | Client parser/retry logic; persistent small events and open connections.                                         |
| Local agent                         | One active credential per user, 30-day token, 20-second long poll, 120-second job lease                       | Fits MVP scope and avoids busy polling                             | Device receives execution secrets; revocation cannot erase secrets already delivered.                            |
| Pin/retention (behavior agreed)     | Proposed idempotent PUT/DELETE pin routes; separate metadata; hourly cleanup                                  | Maintains immutable evidence and 30-day policy                     | Pinned storage is unbounded until deletion; serialize cleanup against pin.                                       |
| MCP transport/access                | Local Go stdio server with distinct read-only credential; audit before response                               | Keeps initial integration small and removes secret execution scope | Manual token pairing/renewal; remote OAuth MCP is separate work.                                                 |

## Monthly development cost estimate

**Proposed for Davian Hernandez's review.** USD, `us-east-2` (Ohio), checked
**2026-10-05** using public official AWS pricing pages and regional Price List
Bulk API files. No AWS credentials, resource queries or resource mutations were
used for cost research. Internet pricing fetches are not deployments.
Use on-demand Linux/ARM64, 730 operating hours/month, no Spot/reservations,
credits, Savings Plans or assumed free-tier eligibility. For services advertising
free allowances, charge all modeled usage at their first paid tier; actual bills
could be lower. CloudFront is pay-as-you-go with US viewers, not a bundled plan.

### Capacity and usage assumptions

- One API task and one worker task, each 0.25 vCPU / 0.5 GiB memory, 730 hours,
  with included 20 GiB ephemeral disk. This is a development sizing proposal,
  not a load-tested guarantee; four concurrent jobs and large OpenAPI parsing
  may require more memory. No autoscaling or deployment-overlap hours modeled.
- One public HTTPS ALB across two AZs, one ordinary zonal NAT Gateway for the
  private option. Normal steady-state API/worker and RDS placement is in the
  NAT AZ to avoid cross-AZ task/DB/NAT traffic; subnets exist in a second AZ.
  Single-AZ RDS PostgreSQL 17, `db.t4g.micro` (2 vCPU / 1 GiB), 20 GiB gp3,
  default included IOPS/throughput, seven-day backups. Assume 30 GiB average
  combined backup/snapshot storage: 20 GiB active-instance included allocation
  plus **10 GiB billable overage**. That allocation is an RDS service rule,
  not account free-tier eligibility. No excess CPU credits, Extended Support,
  Multi-AZ, RDS Proxy or snapshot export in this model.
- ALB IPv4: two addresses (one per enabled AZ), subject to growth as ALB scales.
  Private option adds one NAT Elastic IP; public option adds one IP per task.
  Thus three versus four charged addresses, not counting private RDS addresses.
- 100 monthly active users, 10,000 executions including workflow steps, average
  10 KiB outbound request / 100 KiB response, and 100,000 application calls.
  Forecast average ALB consumption 0.1 LCU, conservatively allowing bursts/SSE/
  local polls; assumed <=300 active connections, <=2.5 new connections/sec and
  <=0.1 GB/hour processed on average, <10 routing rules. Bill the **maximum**
  LCU dimension, not their sum. No reserved LCUs or mutual TLS trust store.
- 20 GB/month **total bidirectional NAT processing**, including public API
  traffic and task AWS control/log/image traffic; generous development/redeploy
  allowance rather than measured bytes. 5 GB combined internet egress from
  API/worker (including ALB responses), separately from 10 GB static CloudFront
  delivery. Inbound internet transfer is not charged as outbound. NAT processing
  and internet egress are distinct fees on overlapping bytes, not duplicate
  internet-transfer line items. Use an S3 gateway endpoint (no hourly endpoint
  fee); no paid interface endpoints. ECR same-region transfer is not charged
  as internet egress; AWS control HTTPS bytes still count toward NAT if routed
  through it. Routing actual image-layer bytes through S3 can reduce NAT usage.
- 200,000 billable standard SQS requests (send, receive including empty long
  polls, deletes, lease changes/retries/DLQ) at <=64 KiB per message; one idle
  20-second poll alone is about 131,400 receives/month. No payload/body queue.
- S3 Standard average 2 GB total for versioned static assets and SST state,
  5,000 PUT/LIST and 10,000 GET requests; bounded evidence stays in PostgreSQL,
  so it is **not** also priced as artifact S3. CloudFront: 100,000 HTTPS
  requests, 10 GB US edge egress, 100,000 SPA function invocations; no paid
  KeyValueStore, Origin Shield, Lambda@Edge, invalidations or optional metrics.
  Origin S3 GETs are included in the S3 count, not all viewer requests again.
- CloudWatch: 1 GB logs ingested, 0.5 GB-month stored with 14-day retention,
  2 GB queried, ten custom metrics, seven standard single-metric alarms,
  10,000 paid ordinary metric API requests. No paid dashboard, Container Insights,
  external managed Prometheus or tracing; avoid high-cardinality metric labels.
- One fresh customer-managed symmetric KMS key and 100,000 symmetric operations;
  four Secrets Manager secrets (DB master/migrator, API DB role, worker DB role,
  shared purpose-derived cursor/receipt HMAC root), 10,000 secret reads. Reusable
  user secrets remain encrypted DB rows, not individually billed Secrets Manager
  entries. RDS uses its default AWS-managed encryption key; no second paid key.
- Cognito Lite: 100 direct/email MAUs, no advanced protection/SMS/M2M charges.
  SES: 1,000 verification/reset emails, 0.02 GB outbound message data, no dedicated
  IP or email add-ons. One private ECR GB, one public Route 53 zone with only
  charge-exempt alias queries to ALB/CloudFront; standard non-exportable public
  ACM certificates. Domain registration/renewal and GitHub runner charges are
  outside the AWS runtime estimate, not asserted to be zero.

### Rate and monthly calculation

F = allocated capacity/resource charge at the stated 730-hour operation;
U = forecast usage/storage charge. Totals use unrounded amounts; row display
rounding can differ by a cent. Regional feeds were read in memory; no pricing
files or credentials were committed.

| Item                        | Official rate and modeled calculation                                                                                                                                                                           | Private + NAT | Public tasks, no NAT |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------: | -------------------: |
| Fargate F                   | [Ohio ECS catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonECS/current/us-east-2/index.json): ARM CPU $0.03238/vCPU-hour, memory $0.00356/GiB-hour; 2 × 730 × (0.25 × CPU + 0.5 × memory) |        $14.42 |               $14.42 |
| ALB F                       | [Ohio ELB catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSELB/current/us-east-2/index.json): 730 × $0.0225/hour                                                                             |        $16.43 |               $16.43 |
| ALB U                       | Same catalog: 730 × 0.1 LCU × $0.008/LCU-hour                                                                                                                                                                   |         $0.58 |                $0.58 |
| NAT F                       | [VPC pricing, Ohio example](https://aws.amazon.com/vpc/pricing/): 730 × $0.045/hour                                                                                                                             |        $32.85 |                $0.00 |
| NAT U                       | Same source: 20 GB × $0.045/GB processed                                                                                                                                                                        |         $0.90 |                $0.00 |
| Public IPv4 F               | Same source: $0.005/address-hour; 3 versus 4 × 730 hours                                                                                                                                                        |        $10.95 |               $14.60 |
| RDS F                       | [Ohio RDS catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonRDS/current/us-east-2/index.json): db.t4g.micro Single-AZ PostgreSQL, 730 × $0.016/hour                                        |        $11.68 |               $11.68 |
| RDS gp3 F                   | Same catalog: 20 × $0.115/GiB-month                                                                                                                                                                             |         $2.30 |                $2.30 |
| RDS backups U               | Same catalog: 10 GiB billable × $0.095/GiB-month                                                                                                                                                                |         $0.95 |                $0.95 |
| SQS U                       | [Ohio SQS catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AWSQueueService/current/us-east-2/index.json): 0.2 million × $0.40/million standard requests                                         |         $0.08 |                $0.08 |
| S3 U                        | [Ohio S3 catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonS3/current/us-east-2/index.json): 2 × $0.023/GB-month + 5 × $0.005/1,000 PUT/LIST + 10 × $0.0004/1,000 GET                      |         $0.08 |                $0.08 |
| CloudFront U                | [Pay-as-you-go US rates](https://aws.amazon.com/cloudfront/pricing/pay-as-you-go/): 10 × $0.085/GB + 10 × $0.01/10,000 HTTPS + 0.1 × $0.10/million function calls                                               |         $0.96 |                $0.96 |
| CloudWatch logs U           | [Ohio CloudWatch catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonCloudWatch/current/us-east-2/index.json): 1 × $0.50 ingest + 0.5 × $0.03 storage + 2 × $0.005 query                     |         $0.53 |                $0.53 |
| CloudWatch metrics/alarms F | Same catalog: 10 × $0.30/metric-month + 7 × $0.10/alarm metric-month                                                                                                                                            |         $3.70 |                $3.70 |
| CloudWatch API U            | [Ohio CW:Requests catalog](https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonCloudWatch/current/us-east-2/index.json): 10 × $0.01/1,000 requests                                                    |         $0.10 |                $0.10 |
| KMS F + U                   | [KMS pricing](https://aws.amazon.com/kms/pricing/): $1/key-month + 10 × $0.03/10,000 symmetric requests                                                                                                         |         $1.30 |                $1.30 |
| Secrets Manager F + U       | [Secrets Manager pricing](https://aws.amazon.com/secrets-manager/pricing/): 4 × $0.40/secret-month + $0.05/10,000 reads                                                                                         |         $1.65 |                $1.65 |
| Cognito U                   | [Lite pricing](https://aws.amazon.com/cognito/pricing/): 100 × $0.0055/paid MAU, no free allowance subtracted                                                                                                   |         $0.55 |                $0.55 |
| SES U                       | [SES pricing](https://aws.amazon.com/ses/pricing/): $0.10/1,000 emails + 0.02 × $0.12/GB outbound data                                                                                                          |         $0.10 |                $0.10 |
| Internet DTO U              | [Ohio VPC internet-transfer example](https://aws.amazon.com/vpc/pricing/): 5 × $0.09/GB; API/worker only, CloudFront priced separately                                                                          |         $0.45 |                $0.45 |
| ECR U                       | [ECR pricing](https://aws.amazon.com/ecr/pricing/): 1 × $0.10/GB-month private images                                                                                                                           |         $0.10 |                $0.10 |
| Route 53 F                  | [Route 53 pricing](https://aws.amazon.com/route53/pricing/): one zone × $0.50/month, alias queries only                                                                                                         |         $0.50 |                $0.50 |
| ACM                         | [ACM pricing](https://aws.amazon.com/certificate-manager/pricing/): standard non-exportable public certificates for integrated services                                                                         |         $0.00 |                $0.00 |
| **Fixed subtotal**          | Compute/resources, allocated gp3, IPs, metrics/alarms, key, secrets and zone                                                                                                                                    |    **$95.42** |           **$66.22** |
| **Usage subtotal**          | All modeled request/transfer/variable-storage charges                                                                                                                                                           |     **$5.73** |            **$4.83** |
| **Monthly total**           | No credits/free-tier discounts or taxes                                                                                                                                                                         |   **$101.15** |           **$71.05** |

Public-task alternative saves **$30.10/month (~29.8%)**:
`$32.85 NAT hours + $0.90 processing - $3.65 one extra public IP`.
It removes NAT's fixed charge/AZ dependency but exposes task addresses to the
internet and demands careful security-group/egress review. API ingress still
comes only from ALB; worker inbound remains closed; RDS remains private and
application destination restrictions remain identical. Keep private+NAT as the
current proposed default pending Davian's cost/security decision; do not silently
change topology because the alternative is cheaper.

### Evidence limits, sensitivities and exclusions

Ohio catalog publication dates observed: ECS/ELB/SQS 2026-09-11, RDS
2026-10-05, S3 2026-09-28, CloudWatch 2026-09-22. Selected SKUs include ARM CPU
`FH39MKJC497ESWU9`, ARM memory `B7ZYF6FKX8ECBKSF`, ALB `HCVPZW9WEWSJKJWJ`,
RDS instance `GGD6FE59WWUNDKDC`, gp3 `JQQGK6D789TWJF8B`, and backup
`HQRQUXJ6DAN3XS6X`. These links are mutable **current** catalogs, not a promise
that rates remain fixed. All prices used above were verifiable; **usage is
unverified forecasting**, not measured deployment consumption.

CloudWatch API row uses regional generic `CW:Requests` SKU
`EPEDMD8JEQSJS958` ($0.01/1,000). The current
[pricing page](https://aws.amazon.com/cloudwatch/pricing/) separately describes
PutMetricData publishing at $0.01/million, which is not a distinct publishing
SKU in the catalog read here. Treat $0.10 for the 10,000-call publishing/
management budget as a **conservative category allowance**, not proof that every
metric API has the same rate. Verify actual publishing/reading usage categories
before infrastructure implementation; GetMetricData is priced per metric, not
per generic API call, and is not assumed in this budget.

[RDS backup allocation](https://docs.aws.amazon.com/aws-backup/latest/devguide/rds-backup.html)
is shared across automated/manual snapshots; do not charge those backups again
as customer S3 objects. If total backup use is <=20 GiB, the $0.95 overage goes
away. Deleting/stopping instances changes that calculation. Excess T4g RDS
CPU credits cost
[$0.075/vCPU-hour](https://aws.amazon.com/rds/postgresql/pricing/), unlike EC2
credit rates; zero excess is only an assumption. gp3 storage growth, pins and
additional replicas increase cost. Doubling both tasks to 0.5 vCPU/1 GiB adds
about $14.42/month, before any overlap IP/network changes.

At 100 GB NAT processing rather than 20 GB, private option adds $3.60/month;
0.1 -> 1 average ALB LCU adds $5.26/month. Single-AZ placement avoids modeled
cross-AZ transfers; failover/rebalancing or placing API/worker in the other AZ
adds regional transfer charges (allowance not included; verify exact paths
before implementation). ALB scaling/additional deployments may add IPv4 hours.
A second zonal NAT adds at least $36.50/month (hourly + IPv4) before traffic.
KMS's first and second key-material rotations each add
[$1/key-month](https://aws.amazon.com/kms/pricing/); this estimate is for a new
unrotated key. Paid metrics APIs outside the specified ordinary API category,
CloudFront KeyValueStore use, and interface endpoints need separate line items
if SST's chosen components enable them. Disable/avoid unneeded Cloud Map,
Container Insights, paid dashboards, WAF and tracing in the priced dev baseline;
this is a proposal, not a verified SST resource plan. CloudWatch metric/receipt
retention-key lifecycle and SES quotas still need implementation review.

Excluded because quantities are unknown: taxes, domain renewal, CI/build runner
usage, one-shot migration/deployment task overlap, support plans, retained
bootstrap artifacts beyond the 2 GB S3/1 GB ECR allowance, restore drills and
commercial HA. Those exclusions are not claimed to be free. A **$120/month
private / $90/month public development budget** gives a modest planning margin,
not a billing cap or automatic approval. Stopping Fargate alone does not stop
ALB/NAT/storage charges; no shutdown workflow is authorized here.

## Execution limits and failure semantics

Propose request body <= 1 MiB, serialized definition/execute-write <= 2 MiB,
OpenAPI upload <= 5 MiB, URL <= 8 KiB, combined request/response headers <= 64 KiB
each and <= 100 fields. Response body <= 2 MiB **after decompression**, with a
separate 2 MiB wire-read cap. Bound JSON nesting at 64 levels and reject
unsupported body encodings; propose JSON/text UTF-8 only in MVP.
Use a 30-second overall DNS/connect/TLS/body deadline, user-selectable 1–60
seconds; connect five seconds, TLS handshake five seconds, response-header wait
ten seconds, all capped by remaining overall deadline. Worker finalization
gets a separate bounded 15-second internal budget and the 120-second lease.
Start worker concurrency at four, per-user in-flight limit five, queued limit 100. Rate/concurrency enforcement must not expose another user's activity.

Go exposes
[Client.Timeout, CheckRedirect, Transport limits, and MaxBytesReader](https://pkg.go.dev/net/http).
Apply caps while streaming, independent of Content-Length. If cap+1 bytes are
observed, stop reading; produce failed `response_too_large`, preserve status
and sanitized headers if received, omit body and set `bodyOmittedReason`.
Do not store an unsafe truncated prefix or label it a full response. A 4xx/5xx
remains failed with exact status even if body omitted. Unsafe decoding/redaction
also omits body. A pre-response timeout has null httpStatus; a body timeout
after response headers preserves the received status. Proposed failures add
structured codes without inventing an upstream HTTP status.

OpenAPI parser/library selection remains on its existing card. Propose only
internal document `$ref` resolution initially with explicit unsupported-feature
warnings, bounded references and validation work; no arbitrary remote fetch.

## Cloud destination policy

Accept only absolute http/https URLs; reject userinfo, malformed hosts, zone
identifiers, alternate/ambiguous IP encodings, unsupported ports, and explicit
Host overrides. Do not use ambient HTTP_PROXY/HTTPS_PROXY. Disable redirects
initially; show sanitized Location and 3xx response. A later redirect option
would allow at most three hops, reapply the complete policy on every hop, and
drop credentials on origin change. HTTP is allowed by MVP scope, but warn when
sending protected configuration without TLS and propose explicit confirmation.

Resolve A/AAAA records with a controlled resolver on each new dial. Reject
the destination if any candidate is nonpublic, including loopback, RFC1918,
ULA, link-local, CGNAT, multicast, unspecified, reserved/documentation, metadata,
and VPC-local ranges. Normalize IPv4-mapped IPv6 before checking; deny
translation/tunnel address forms initially. Use the current
[IANA IPv4](https://www.iana.org/assignments/iana-ipv4-special-registry) and
[IPv6](https://www.iana.org/assignments/iana-ipv6-special-registry) registries
to maintain/test the policy; Go's global-unicast predicate alone is insufficient.
Pin the validated IP in the dialer; retain original hostname for TLS SNI and
certificate validation. Never validate DNS and then let another resolver pick
the connection address. Apply checks on reconnect as well; cap DNS time and
reject private answers even from a previously allowed hostname. Destination
restrictions apply only to user-request HTTP transport, separate from narrowly
configured PostgreSQL/AWS SDK transports that require private/runtime endpoints.

The local agent permits explicitly approved localhost/private hosts and ports
on that device. Default allowlist is loopback only; private network expansion
requires local operator configuration. Block cloud metadata/credential endpoints
there too. Hosted clients cannot turn the agent into an unrestricted network
proxy. Cloud public-destination restriction does not eliminate abuse against
public services; quotas and per-user limits are needed from the first executor.

## Secret encryption and rotation

Use a symmetric customer-managed KMS key per stage and one fresh data key per
protected value/binding; AES-256-GCM with fresh nonce. Store ciphertext, nonce,
tag, wrapped key, key ARN, crypto version, and non-secret context identifiers.
Generate and unwrap keys using narrowly scoped task roles; never put decrypted
keys in SST links, env vars, logs, queues, disk, or evidence. Use a reviewed Go
crypto implementation; avoid designing a new cryptographic format without
known-answer/tamper tests. Local tests use an isolated test key provider, never
a plaintext production fallback.

[KMS envelope encryption](https://docs.aws.amazon.com/kms/latest/developerguide/concepts.html)
and [encryption context](https://docs.aws.amazon.com/kms/latest/developerguide/encrypt_context.html)
support this approach. Context is visible in CloudTrail; include only stage,
opaque project/binding IDs, app name and purpose. IAM/KMS checks constrain
stage/purpose; application authorization still proves user/job ownership.
For local polling, only the leased execution path can decrypt and deliver
runtime configuration over TLS to its paired device. MCP never has that path.

Propose automatic annual KMS key-material rotation. Existing wrapped keys
remain decryptable; key rotation does not rotate users' API credentials or
rewrite historical evidence.
[AWS documents this distinction](https://docs.aws.amazon.com/kms/latest/developerguide/rotate-keys.html).
Rewrapping for a replacement key is a reviewed background migration; retain
the old key until live bindings/backups requiring it have expired or been
migrated. Secret-value rotation updates the stable saved-secret slot and
revision; future submissions/reruns use the new value. Queued jobs use their
frozen encrypted bindings unless explicitly revoked. Retain no reusable
historic plaintext/ciphertext in execution snapshots. DB credentials use
Secrets Manager independently; propose rotation plus pool refresh testing,
not a static copy of the spike master credential.

## Pagination and SSE

All collection routes propose `limit` (default 25, maximum 100) and `cursor`.
Use descending `(createdAt,id)` for definitions/projects and
`(submittedAt,executionId)` for history; failed terminal lists use
`(completedAt,executionId)`. Cursor is versioned base64url JSON + HMAC,
binding owner, resource/filter, last sort tuple, and initial high-water tuple.
Reject invalid/mismatched cursors with invalid_request. Include key ID for
rotation; expire cursors after 24 hours. New items wait for a fresh first page;
deletions may shorten pages. No total count by default. MCP uses the same scope
and ordering policy. API limits apply regardless of provided token scope.

Use browser fetch with Authorization bearer and incremental SSE parsing;
native EventSource cannot supply the current bearer header directly. Never
put access tokens in query strings. `id` is the per-execution monotonic sequence;
send Last-Event-ID on reconnect, persist only the cursor client-side, deduplicate
events, replay events with greater sequence. Keep the four existing contract
event names and use heartbeat comments every 15 seconds. Replay all events
until execution retention removes them; the MVP generates only bounded state
transitions, not progress/body streams. Preserve retry attempts in job metadata.
CloudFront must not cache authenticated API/evidence responses; SSE goes to ALB
with proposed idle timeout 120 seconds and proxy buffering disabled.

[The HTML SSE standard](https://html.spec.whatwg.org/multipage/server-sent-events.html)
defines event IDs and Last-Event-ID. Fetch requires implementing those client
behaviors explicitly. Reconnect with jittered 1/2/4-second delays capped at
30 seconds. Refresh token through the auth SDK on 401, then retry once; close
streams at token expiry. Recheck ownership/project state periodically and on
each event read. Invalid/ahead cursor returns invalid_request before streaming;
missing/expired execution returns not_found. On reconnect fetch GET execution
and its proposed lastEventId to reconcile status. If database state is terminal
and already delivered, close; do not add an unreviewed reset event type.

## Local agent and MCP credentials

Local tokens: 256 random bits, raw token returned once, store SHA-256 digest,
30-day expiry. High entropy permits a fast digest; this is not password hashing.
Create/revoke under a user row lock, enforce one unrevoked credential per user,
and atomically revoke expired predecessor before replacement. Poll long-waits
20 seconds, empty response proposed 204; after empty wait jitter 1–3 seconds,
after network errors exponential backoff to 30 seconds. One poll/job at a time
for MVP. No inbound device networking or AWS credentials required.

The [local dispatch protocol](w1-system-design.md#local-poll-is-the-durable-dispatch-boundary)
commits running state, dispatch intent and random lease identity **before** poll
returns an executable envelope. The agent must start within 30 seconds and
finish/upload within a 120-second lease; no start acknowledgment route is needed.
A lost executable poll response or crash before/after HTTP is ambiguous and
terminalizes as unknown on expiry, without re-delivery or automatic replay.
Execution secrets/nonce are execution-only TLS data, never evidence or logs.

The [result receipt protocol](w1-system-design.md#local-result-creation-versus-duplicate-acknowledgment)
requires the exact live lease/fence/nonce and original valid credential to
**create** a result. A committed result atomically creates a retained receipt.
The same valid credential plus accepted lease identity can **acknowledge** an
identical canonical packet even after lease expiry; it cannot write evidence.
Different packet/lease is conflict; revoked/expired original credentials are
unauthenticated even for duplicates. No receipt plus expired lease cannot create
or replace a terminal record. Use private HMAC-SHA-256 over normalized versioned
DTO bytes with [RFC 8785 canonicalization](https://www.rfc-editor.org/rfc/rfc8785),
distinct from the sanitized evidence hash; do not store raw packet/extractions
for deduplication. Receipts cascade on execution/project cleanup. Server-side
sanitization and upload caps still apply. Timing reported by an agent is marked
as such; server times are assigned once. Local workflows return protected
extractions through a separate runtime envelope, encrypted before persistence.

Revocation is checked from DB on **every** poll/result call without positive
credential caching; reject revoked/expired credentials as unauthenticated.
Revoke also fences pending leases and destroys their bindings. A request already
dispatched cannot be undone, and a device can retain a secret already delivered.
Lost local lease after dispatch finalizes unknown outcome instead of assigning
the HTTP request again. No forced cloud fallback. Pair/rotation UI should state
these limits clearly. Agent token storage uses the OS credential store or an
owner-readable local file; runtime secrets stay memory-only.

Propose separate 30-day read-only MCP credentials, created/revoked by an
authenticated owner, exposed once and hashed. Stdio server receives its own
credential from local secure configuration, verifies tool inputs, and calls
an API endpoint that audits invocation before returning sanitized data. Fail
closed if mandatory audit persistence fails. Unauthenticated attempts log only
safe security categories since no user is known. Cross-user denied attempts
must not copy the foreign project's content into an audit row. Project-specific
audit rows delete with that project; account-only records expire after 90 days.
Remote MCP must follow the current
[MCP security guidance](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices),
including intended-recipient tokens; raw Cognito token passthrough is not an
approved remote MCP authorization design.

## Pinning and cleanup API proposal

Propose `PUT /api/v1/executions/{executionId}/pin` and
`DELETE /api/v1/executions/{executionId}/pin`, owner-authorized, terminal-only,
idempotent, returning 200 with `data.retention`:

```json
{
  "data": {
    "retention": {
      "executionId": "opaque-id",
      "pinned": true,
      "pinnedAt": "2026-10-05T23:00:00Z",
      "expiresAt": null
    }
  }
}
```

Unpin resets expiry to original completedAt + 30 days. If past due, the unpin
response reports the past expiresAt and the execution becomes unavailable to
normal reads immediately; cleanup removes it on the next pass. Repeating an
operation on an already removed result returns not_found. Pinning an expired
unpinned result is rejected even if physical cleanup has not run. Pinned
results cannot be removed by unpin cleanup before that transaction commits.
Retention is a sibling of the immutable record in read responses, never
embedded in the captured evidence hash. Hourly batches and project deletion
share locking/fencing procedures. No blanket S3 30-day lifecycle may override
pinning if object storage is introduced.

Use the [shared lock order](w1-data-model.md#shared-locking-and-retention-transactions):
credential when required, project gate, run, steps, execution, retention, job,
then child writes, with each set sorted. All retention mutations hold project
FOR UPDATE; cleanup discovers IDs before locking and uses SKIP LOCKED only
on project gates. Recheck expiry using fresh server time after acquiring locks.
Workflow context persists for pinned or still-available step evidence, without
response copies. Last unpin after normal run expiry creates no new grace period:
expired bodies and unneeded summary become logically unavailable at commit and
eligible for physical cleanup. Project deletion removes all of them regardless
of pins. Per-project serialization is a deliberate MVP throughput tradeoff.

## Future implementation acceptance tests for the four findings

These are required **future tests**, not tests implemented or run by this
documentation revision. Use controlled transport crashes, a real PostgreSQL
transaction boundary and barriers to force each race; mock-only happy paths
would not prove these invariants.

| Finding                 | Acceptance scenarios and observable outcomes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Local dispatch boundary | Crash poll server before commit: job can be claimed and target sees one request. Lose response after commit, crash agent before send, crash after send: repeated polls never return that executable lease; expiry produces exactly one unknown snapshot and at most one target call. Expired executeNotAfter blocks agent HTTP. Revocation before poll denies envelope; revoke after delivery denies uploads/fences job but makes no claim of undoing HTTP. Race lease reconciler versus live upload: exactly one terminal transition wins.                                                                                                                                                                                                                                                                    |
| Atomic advancement      | Inject rollback after each staged snapshot/output/state/job/outbox write: none becomes visible. Lose process after HTTP but before commit: unknown failure, no successor. Kill after commit before queue publish: next job and exact encrypted variable versions survive and dispatcher resumes. Concurrent finalizers/duplicate queue messages create one successor identity and one final snapshot per step. Verify marked token extracted from raw response reaches later HTTP, while evidence/logs/queue omit it. Extraction failure produces failed step and skipped successors with no next job.                                                                                                                                                                                                         |
| Idempotent uploads      | Commit result then drop response: retry same DTO after lease expiry returns original 200 receipt without changes to snapshot/event counts/run version/next job count. Reorder object keys/whitespace: same digest; reorder headers array, change body/timing/extraction: conflict. Reject duplicate keys and unsafe numbers. Wrong nonce/lease or replacement credential cannot acknowledge. Revoke original credential: identical retry returns 401. Reconciler unknown terminal or deleted receipt: late upload never overwrites/reopens it. Changing sanitizer version after acceptance does not break unchanged packet acknowledgment.                                                                                                                                                                     |
| Retention/pinning       | Commit pin while evidence is still available, then let cleanup reach expiry: body/context retained. Cleanup first or expiry during lock wait: pin cannot resurrect expired rows. Race pin versus run-summary cleanup: pin and safe context survive together or pin fails, never orphaned pinned history. While one step pinned, clean expired unpinned sibling body and preserve only safe context/evidenceExpired marker. Unpin last step after run expiry, then run cleaners in either order: context and expired bodies become unavailable immediately and are removed with no new grace period. Run unpin/cleanup/project-delete stress with lock barriers: no inconsistent protection, deadlocks from reversed lock order, or resurrection; project deletion removes pinned snapshots/receipts/variables. |

Additional protocol checks: a database timeout with uncertain commit must read
back receipt/job state before transaction retry; a retry never performs HTTP.
Validate accepted packet HMAC/key retention through pinning and key rotation.
Tests must prove project ownership and same-run variable provenance, not merely
uniqueness. Assertions and diagnostics must never echo an extracted secret.

## Cognito verification and email checks

Verify RS256 signature, allowed algorithm/key type, `kid`, exact configured
issuer, `token_use=access`, allowed `client_id`, `exp`, and sensible issuance
time with 60-second clock tolerance. Require scopes for any separately defined
API resource-server scopes. Cache only the configured pool's JWKS, refresh on
unknown kid with rate limiting, never trust token-supplied key URLs. Reject ID
tokens. User identity is `(iss,sub)`. Use a maintained Go JWT/JWKS library chosen
and tested in the auth task; AWS's Node-specific verifier is not a Go dependency.
These checks follow
[Cognito JWT verification guidance](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-tokens-verifying-a-jwt.html).

Propose 15-minute access-token lifetime, one-day refresh session with rotation,
in-memory browser storage and reauthentication after reload. Do not persist
bearer/refresh tokens in localStorage. Cognito revocation is not automatically
observed by offline signature checks; document API access may continue until
token expiry. An immediate sign-out guarantee requires a reviewed application
session/revocation check. Application user disabled state should be enforced
on each request. See
[Cognito revocation behavior](https://docs.aws.amazon.com/cognito/latest/developerguide/token-revocation.html).

Select Lite explicitly after checking SDK flow compatibility; no email MFA or
Plus risk protection is assumed. The current
[feature-plan guide](https://docs.aws.amazon.com/cognito/latest/developerguide/cognito-sign-in-feature-plans.html)
distinguishes those plans. Propose SES-backed Cognito transactional email for
presentation/public signup, with verified sender identity, regional compatibility,
DKIM/SPF/DMARC and bounce/complaint visibility. Cognito's default email capacity
is limited and SES sandbox restricts recipients; independently verify quotas,
production access, sender/reply-to, and delivery for Kurier.
[Cognito email documentation](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-email.html)
describes both options. No cost or free-tier assumption is made.

Before acceptance of the auth implementation: read Dollhouse's email configuration
as a reference under a separate scoped task; do not copy its credentials or pool.
For Kurier verify signup, verification/resend and cooldown, reset request/code/
confirmation, expired/incorrect codes, real delivery to independent recipients,
rate limits, generic enumeration-resistant errors, and sign-out/session limits.
Record actual pool tier, SES region/identity/quotas and pricing reviewed at that
time. No email delivery or AWS configuration was tested by this design task.

## Contract gaps and source conflicts requiring review

The following are findings against the read v0.1 document, not silently accepted
routes. Pre-release adoption could remain v1 after coordinated review; there
is no implemented product data to migrate yet. Later revisions need versioned
SQL and clients updated alongside the contract.

| Source/route or schema                                      | Gap/conflict                                                                                                                                                                 | Concrete proposal for review                                                                                                                                                                                                                                |
| ----------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Proposal technology, deployment requirement, W2/W9 timeline | Still names CDK; accepted ADR selects SST. `infra/README.md` incorrectly says no framework adopted.                                                                          | Keep SST; reconcile proposal on its existing card. Leave accepted ADR unchanged; flag stale infra text for synchronized documentation.                                                                                                                      |
| Proposal history versus contracts                           | Durable preserved executions can read as indefinite retention; contracts select 30-day unpinned MVP and indefinite history post-MVP.                                         | Explicitly describe bounded MVP plus pinning; retain agreed policy.                                                                                                                                                                                         |
| KeyValueField and request/environment writes                | `value` plus sensitive boolean lacks secretRef, set/preserve/remove semantics, body JSON paths, and masked reads. Returning RequestDefinition cannot echo saved credentials. | Sensitive writes accept value only for replacement, secretRef for preserving known binding, explicit remove operation; reject literal mask as credential. Reads return null value, masked=true, secretRef. Define nested body/query/URL sensitive paths.    |
| POST executions / Queue ExecutionJob                        | No frozen revisions, idempotency, limits, lease fields, schema capture; queue attempt can be stale.                                                                          | Add reviewed Idempotency-Key/revision preconditions and DB frozen plan; keep queue identifier-only; attempt advisory. Public revision fields need schema updates.                                                                                           |
| ExecutionSummary / ExecutionRecord                          | queued summary mutable but record called immutable; lacks projectId, nullable deleted request link, omission metadata, evidence version and replay references.               | Separate status resource from terminal captured record; captured requestId remains historical string; expose liveRequestId separately if needed. Add typed sanitized body/error/assertion/violation shapes.                                                 |
| GET request history after request deletion                  | No project-wide history route; nested deleted request cannot be read normally.                                                                                               | Add GET /api/v1/projects/{projectId}/executions with filters; direct GET execution remains available by owner. Historical request filter uses frozen ID.                                                                                                    |
| POST execution reruns                                       | Secret rebinding and deleted-source behavior unspecified.                                                                                                                    | Replay frozen non-secret config, current stable secret references, fail clearly if missing; capture new bindings and execution. No fallback by variable name.                                                                                               |
| Failed/completed semantics                                  | HTTP 4xx/5xx agreed; assertions/validation failures and 3xx policy unspecified.                                                                                              | Failed for assertions; propose failed for supported contract violations, skipped/not-evaluated for unsupported validation. 3xx completed with redirects off unless assertions fail. Add outcome-unknown and omission error codes.                           |
| Pinning                                                     | Routes/metadata explicitly deferred to pinning card.                                                                                                                         | Review PUT/DELETE pin routes above; extend read responses with separate retention object and terminal-only rules.                                                                                                                                           |
| Environment GET/PATCH                                       | No GET-by-ID, concrete collection envelope, conflict/version behavior or deletion-history semantics.                                                                         | Add GET by ID; consistent items/nextCursor; revision precondition; null live history link and fail dependent pending jobs on deletion.                                                                                                                      |
| OpenAPI import/operation                                    | Library/dialect unresolved; no association field/write route on request, partial import behavior, reference policy or source sanitization.                                   | Optional operationId refers to Kurier operation UUID; expose method/path/documentOperationId; atomic generated requests, internal refs only; warnings/unsupported typed. Library card determines dialect support.                                           |
| Workflows / workflowRun                                     | No list/get/edit/delete definitions, step schemas, extraction grammar, run environment/target selection, retention or step evidence expiration.                              | Add definition CRUD and revision; steps have ordered request IDs and bounded extraction/assertion rules; freeze plans on run; add run-retention/expired-step metadata as described in model. Choose extraction grammar in workflow implementation proposal. |
| Agent token and poll/result                                 | localAgentId versus tokenId not mapped; expiry/one-agent/poll response/lease/result shapes absent.                                                                           | MVP localAgentId equals credential ID; define 20-second poll and 204 empty, 120-second lease nonce, expiry/revocation, bounded sanitized result and duplicate conflict behavior.                                                                            |
| SSE                                                         | Last-Event-ID, authentication implementation, replay, completion/expiry absent.                                                                                              | Fetch-based bearer SSE; sequence IDs and proposed lastEventId in status reads; replay then reconcile, no secret/body events.                                                                                                                                |
| MCP tools                                                   | No transport/auth pairing routes, audit schema, limit behavior, or API bridge for invocation-level auditing.                                                                 | Local stdio and separate read-only MCP credentials; propose POST/DELETE /api/v1/mcp/tokens and POST /api/v1/mcp/invocations with allowlisted tool/input; audit transaction before response. Remote transport separate.                                      |
| Project DELETE 204                                          | Active-data deletion specified; backup erasure and future DB/S3 coordination unspecified.                                                                                    | Complete active-store deletion before 204; seven-day backup residual policy plus restore deletion ledger; async 202 proposal only if artifact deletion requires it.                                                                                         |
| Health contracts versus code                                | Contracts /health and /ready envelopes differ from product /healthz and spike operational routes.                                                                            | Keep /healthz for current foundations; auth/project implementation proposes contract-shaped /health and /ready; do not treat spike HTTP endpoints as product contracts.                                                                                     |
| Error envelope versus upstream response                     | Application error and captured upstream status could be conflated.                                                                                                           | GET execution returns 200 sanitized failed record for upstream 404/500; do not make its API response status equal the target's status.                                                                                                                      |

## Approval and smallest next implementation task

Two additional policy questions remain explicit: whether protected HTTP
execution requires confirmation for plaintext transport, and whether a minimal
opaque-ID deletion ledger with seven-day backup residual lifetime satisfies
the permanent-deletion requirement. The ledger would contain only project ID
and deletion timestamp, never project configuration or evidence; its operational
location and retention require approval before backup/restore implementation.

Davian should review private+NAT versus public-task cost/security, dispatch-intent
before poll, the 30/120-second execution windows, private canonical receipt
HMACs retained with evidence, atomic advancement/runtime envelopes, and the
project gate's serialization cost. These are proposals; architecture acceptance
does not itself authorize deployment.

### Exact synchronization work after agreement

Do **not** perform these changes during this revision. After Davian agrees:

1. Update API contracts' local poll response with leaseId/fence/nonce,
   executeNotAfter/serverTime/deadline and dispatch-intent semantics; document
   no start route, no executable re-delivery, unknown errors and revocation races.
2. Define local result DTO/canonicalization/null/default rules, protected runtime
   extraction envelope, size caps, first-create versus receipt-only retry checks,
   retained original credential/lease identity, 200 minimal receipt, 401/404/409
   outcomes, and behavior after expiry, cleanup or revocation.
3. Extend workflow step/run schemas with queued/skipped states, immutable input
   version references, extraction failure behavior and atomic next-step scheduling;
   distinguish mutable operational version from immutable captured evidence.
4. Specify pin/unpin routes and retention metadata plus normal run expiry,
   derived context protection, sibling evidenceExpired, last-unpin/no-grace
   behavior and project-deletion precedence. Align SSE terminal transitions and
   error labels with these procedures, without adding secret payloads/events.
5. Update the architecture/retention/local-agent/workflow Trello descriptions
   with these boundaries and future acceptance tests. Keep the architecture task
   pending review until accepted/synchronized; keep contract review open until
   affected sections are agreed. No card move or completion is authorized here.
6. Reconcile the proposal's stale CDK technology/deployment and W2/W9 milestone
   wording to accepted SST under its existing reconciliation task; fix stale
   infra/README.md wording. Preserve product MVP commitments. Record network and
   protocol decisions as reviewed follow-up ADR(s) if required, rather than
   changing accepted SST ADR 0001 to imply a newly approved topology.

Select the network option and recheck actual SST-generated resource costs before
a separately authorized infrastructure implementation. No full-schema migration
is implied by approving this logical model.

The smallest implementation task is **local users/projects persistence and
ownership boundary**: add only users/projects migrations and a serialized
migration runner, a verified-identity boundary with test JWT fixtures,
GET users/me and project CRUD in the agreed envelopes, revision handling if
approved, and PostgreSQL integration tests proving cross-user denial and project
deletion. Production Cognito token verification/library integration must be
completed before exposing these routes; fixture identity is test-only, never
a production bypass. Use local Compose, no AWS deployment. The next task then
connects the approved Cognito configuration and React auth/project flow into
the presentation-ready vertical slice. Queue/worker/OpenAPI/workflow/MCP tables
are deliberately deferred until their slices need them.

## Documentation validation scope

Completed for this revision (baseline `c311e74`):

- Formatted the three changed Markdown files with Prettier; repository
  formatting checks passed.
- `GOCACHE=/tmp/kurier-w1-go-build npm run verify` passed formatting, ESLint,
  Vitest, TypeScript/Vite build, and Go tests/vet for all five existing modules.
  Used the temporary Go build cache from the baseline task to respect the
  sandbox's read-only default cache. No executor/protocol tests were added.
- `docker compose config --quiet` and `git diff --check` passed.
- All 18 internal file links/heading anchors and balanced Markdown fences
  passed read-only checks. New official external pricing sources were fetched
  during research; current regional catalogs and public page links were verified.
- Mermaid's parser validated the architecture flowchart, sequence diagram,
  and ERD using temporary dependencies outside the repository. Syntax was
  checked; rendered layout was not visually reviewed.
- Independently recomputed the cost rows/subtotals using unrounded rates:
  $101.1489 private+NAT, $71.0489 public tasks, $30.10 savings. Usage estimates
  and future fault/race acceptance tests were not experimentally validated.

These checks validate the draft/repository, not proposed infrastructure, SQL
migrations, encryption, authentication, or execution behavior. No SST diff,
deployment, database migration, AWS mutation, or repeated cloud spike was run.
No API-contract, Trello or capstone-proposal synchronization was performed.
