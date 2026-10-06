# ADR 0002: Serverless execution and persistence topology

- Status: **Proposed for Davian Hernandez's review**
- Date: 2026-10-05
- Scope: W1 design direction, not deployment or implementation authorization

## Context

The W1 draft proposed Fargate/ALB/NAT and PostgreSQL, with estimated monthly
development baselines of $101.15 private/NAT or $71.05 public tasks/no NAT.
Always-on charges dominate light use. Davian authorized revising the draft
toward Go Lambda, HTTP API Gateway, SQS, DynamoDB and private S3, managed by SST,
with a proposed $5 light-development target. Detailed protocols remain review
inputs; approving direction is not acceptance of every implementation choice.

[ADR 0001](0001-sst-infrastructure-candidate.md) remains **Accepted**: SST 4 is
the selected framework. Its Fargate/RDS context and
[spike reports](../spikes/sst-viability.md) are historical evidence, not proof
that serverless product behavior was tested. Do not rewrite that evidence.
The prior application-user-in-PostgreSQL baseline is explicitly replaced in
this proposed design by DynamoDB users linked to Cognito issuer/sub. Cognito
provider, immutable sanitized evidence, ownership and retention decisions stay.

## Proposed decision

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
all data/pins/late uploads through a proposed 202 operation. Use immediate-return
polling rather than holding Lambda SSE/long-poll invocations. Protocols, limits
and contract changes are detailed in the [system design](../architecture/w1-system-design.md)
and [data model](../architecture/w1-data-model.md).

## Consequences and alternatives

- Refined light forecast $3.17/month (approximately $3.10 screening estimate),
  heavy 10k/30-second execution forecast $14.20 including one KMS rotation.
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

This ADR is Proposed, not a replacement for accepted SST ADR 0001. Davian must
review table/index/protocol limits, polling policy, asynchronous project deletion,
backup residual/tombstone policy, budgets/quotas and crypto/receipt details.
After agreement synchronize proposal's stale CDK and Fargate/PostgreSQL wording,
API contracts and Trello as enumerated in the decision sheet; do not update them
now. Begin only a separately authorized local users/projects ownership slice.
No full schema migration, executor implementation, deployment or AWS mutation
is part of this documentation revision.
