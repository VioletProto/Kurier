# ADR 0002: Serverless execution and persistence topology

- Status: **Accepted**
- Date: 2026-10-05
- Accepted: 2026-10-06 by Davian Hernandez, baseline commit `1fba942`
- Scope: W1 design direction, not deployment or implementation authorization

## Context

The W1 draft proposed Fargate/ALB/NAT and PostgreSQL, with estimated monthly
development baselines of $101.15 private/NAT or $71.05 public tasks/no NAT.
Always-on charges dominate light use. Davian authorized revising the draft
toward Go Lambda, HTTP API Gateway, SQS, DynamoDB and private S3, managed by SST,
with a $5 light-development target. Davian subsequently accepted commit
`1fba942`, including its documented defaults. This accepts the design, not
deployment or implementation, and does not turn forecasts into measured bills.

[ADR 0001](0001-sst-infrastructure-candidate.md) remains **Accepted**: SST 4 is
the selected framework. Its Fargate/RDS context and
[spike reports](../spikes/sst-viability.md) are historical evidence, not proof
that serverless product behavior was tested. Do not rewrite that evidence.
The prior application-user-in-PostgreSQL baseline is explicitly replaced in
this accepted design by DynamoDB users linked to Cognito issuer/sub. Cognito
provider, immutable sanitized evidence, ownership and retention decisions stay.

## Accepted decision

Use Go ARM64 zip Lambda handlers behind HTTP API Gateway, standard SQS wakeups,
two on-demand DynamoDB tables (Control/Protected), private S3 sanitized evidence,
CloudFront/static S3, Cognito, KMS, one service secret and scheduled maintenance.
No customer-VPC attachment, ALB, NAT, RDS or provisioned concurrency initially.
Keep SST selected; review its installed-version components/generated IAM later.

Map PostgreSQL serialization to shared project version conditions and bounded
DynamoDB transactions. Preserve intent-before-dispatch/no ambiguous replay,
atomic workflow variables/state/successor scheduling, retained idempotent local
upload receipts, and conditional pin/context/cleanup protection. S3 publication
uses registered upload tickets and transactionally published manifests, not
cross-service atomicity. Project deletion immediately denies access, then drains
all data/pins/late uploads through a 202 operation. Use immediate-return
polling rather than holding Lambda SSE/long-poll invocations. Protocols, limits
and contract changes are detailed in the [system design](../architecture/w1-system-design.md)
and [data model](../architecture/w1-data-model.md).

Davian selected the MVP restore exception: earlier restoration may restore
later-deleted projects/items and lose later changes. Communicate actual
Control/Protected restore timestamps and that warning; deletion does not survive
every backup restore. No independently preserved deletion journal solely for
anti-resurrection. Normal active-store denial, upload fencing, tombstones and
orphan cleanup remain. Reconcile Control/Protected/S3 before reopening: missing
evidence is unavailable, never fabricated. The recovery generation and
closed-stage procedure suppress restored queued/running work/OUT; require
deliberate new submissions/reruns, never automatic external HTTP replay.

Use complete saved request and per-execution frozen configuration caps
64 KiB each (all fields/serialization), independent resolved outbound body cap
64 KiB, response wire-read/decompressed caps 2 MiB each, and existing complete
encoded evidence/API/local-upload cap 4 MiB. Reject invalid saved configuration
and oversized resolved input before HTTP; oversized response is Failed with
omission metadata. No S3 large-request-body subsystem in MVP. Future increases
require item/transaction/transport/memory review.

Use one bounded immediate best-effort identifier-only SQS notification after
submission/successor commit, with durable OUT/scheduled delivery as recovery.
Send failure/uncertainty does not undo acceptance or create another job. Identical
scheduled/fast duplicates obey existing claim/fencing; latency targets are not
guarantees. Documented baseline defaults are accepted; runtime validation remains required.

## Consequences and alternatives

- Refined light forecast $3.17/month (approximately $3.10 screening estimate),
  heavy 10k/30-second execution forecast $14.14 including one KMS rotation.
  [Assumptions/rates](../architecture/w1-review-decisions.md#monthly-development-cost-estimate)
  are not measured usage or hard caps. Pins, rotation and abuse can exceed $5.
- DynamoDB requires explicit indexes, owner/reference conditions, item/action
  ceilings and versioned data changes instead of SQL FKs/migrations/joins.
- S3 cannot join database transactions: inaccessible orphans and uncertain
  late PUTs require durable cleanup; physical deletion is eventual.
- Polling adds notification latency; cold starts/Go memory and real costs need
  measurement. No fixed cloud egress IP/security-group egress boundary.
- Fargate/PostgreSQL remains a historical alternative with straightforward SQL
  transactions and sustained streaming but a higher idle-cost floor. Keeping
  RDS with Lambda would not achieve the target and reintroduces connection/
  networking costs. Function URLs would avoid HTTP API request charges but need
  different ingress/auth choices for negligible savings here.

## Review and rollout boundary

ADR 0001 remains Accepted and its historical evidence is unchanged. Acceptance
covers the baseline's table/index/protocol limits, polling, deletion, retention,
backup defaults, recovery, sizes, fast notification, budgets/quotas and crypto/
receipt rules. It does not choose a previously unspecified library, extraction
grammar or wire DTO. Such new choices remain proposed for Davian's review in
the [synchronization record](../architecture/w1-acceptance-sync.md).

Forecast limitations and the future fault/race/restore/security tests in the
decision sheet remain requirements, not completed tests. Synchronization is
documentation-only. Begin a local users/projects ownership slice only after
separate implementation authorization. No full schema migration, executor,
deployment or AWS mutation is authorized by acceptance.
